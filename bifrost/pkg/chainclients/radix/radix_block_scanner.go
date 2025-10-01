package radix

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"gitlab.com/mayachain/mayanode/common/tokenlist"

	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/radix/coreapi"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/radix/router"

	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/radix/types"

	"github.com/radixdlt/maya/radix_core_api_client/models"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/mayachain/mayanode/bifrost/blockscanner"
	btypes "gitlab.com/mayachain/mayanode/bifrost/blockscanner/types"
	"gitlab.com/mayachain/mayanode/bifrost/mayaclient"
	stypes "gitlab.com/mayachain/mayanode/bifrost/mayaclient/types"
	"gitlab.com/mayachain/mayanode/bifrost/metrics"
	"gitlab.com/mayachain/mayanode/bifrost/pubkeymanager"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/config"
)

type SolvencyReporter func(int64) error

type RadixScanner struct {
	cfg              config.BifrostBlockScannerConfiguration
	logger           zerolog.Logger
	scannerStorage   blockscanner.ScannerStorage
	metrics          *metrics.Metrics
	coreApiWrapper   *coreapi.CoreApiWrapper
	network          types.Network
	mayaBridge       mayaclient.MayachainBridge
	pubKeyValidator  pubkeymanager.PubKeyValidator
	solvencyReporter SolvencyReporter
	tokensByAddress  map[string]tokenlist.RadixToken
}

func NewRadixScanner(
	cfg config.BifrostBlockScannerConfiguration,
	scannerStorage blockscanner.ScannerStorage,
	coreApiWrapper *coreapi.CoreApiWrapper,
	mayaBridge mayaclient.MayachainBridge,
	metrics *metrics.Metrics,
	pubKeyValidator pubkeymanager.PubKeyValidator,
	network types.Network,
	tokensByAddress map[string]tokenlist.RadixToken,
) (*RadixScanner, error) {
	if scannerStorage == nil {
		return nil, errors.New("scannerStorage is nil")
	}

	if metrics == nil {
		return nil, errors.New("metrics is nil")
	}

	if coreApiWrapper == nil {
		return nil, errors.New("coreApiWrapper is nil")
	}

	logger := log.Logger.With().
		Str("module", "block_scanner").
		Str("chain", common.XRDChain.String()).Logger()

	return &RadixScanner{
		cfg:             cfg,
		logger:          logger,
		scannerStorage:  scannerStorage,
		metrics:         metrics,
		coreApiWrapper:  coreApiWrapper,
		network:         network,
		mayaBridge:      mayaBridge,
		pubKeyValidator: pubKeyValidator,
		tokensByAddress: tokensByAddress,
	}, nil
}

func (e *RadixScanner) FetchMemPool(_ int64) (stypes.TxIn, error) {
	return stypes.TxIn{}, nil
}

func (e *RadixScanner) FetchTxs(fetchHeight, chainHeight int64) (stypes.TxIn, error) {
	committedTxn, err := e.coreApiWrapper.GetSingleTransactionAtStateVersion(fetchHeight)
	if err != nil {
		return stypes.TxIn{}, fmt.Errorf("failed to get txn at state version %d: %w", fetchHeight, err)
	}

	if committedTxn == nil {
		return stypes.TxIn{}, btypes.ErrUnavailableBlock
	}

	txInItems, err := e.extractTxInItemsFromCommittedTxn(*committedTxn)
	if err != nil {
		return stypes.TxIn{}, fmt.Errorf("failed to process committed transaction: %w", err)
	}
	resultantTxIn := stypes.TxIn{
		Count:   strconv.Itoa(len(txInItems)),
		Chain:   common.XRDChain,
		TxArray: txInItems,
	}

	// Supplementary "FetchTxs" action #1: post network fee information to Maya
	err = e.postNetworkFeeToMayaIfNeeded(*committedTxn)
	if err != nil {
		// Non-fatal; log an error and continue
		e.logger.Err(err).Msg("failed to post Radix transaction fee to Maya")
	}

	// Supplementary "FetchTxs" action #2: trigger the solvency reporter
	e.triggerSolvencyReporter(fetchHeight)

	return resultantTxIn, nil
}

func (e *RadixScanner) postNetworkFeeToMayaIfNeeded(committedTxn models.CommittedTransactionable) error {
	shouldPost := *committedTxn.GetResultantStateIdentifiers().GetStateVersion()%1000 == 0
	if shouldPost {
		// Similarly to e.g. BNB, we post the total fee amount as a single value,
		// accompanied by a transaction size of 1.
		// In Radix the sender of the transaction doesn't specify the "gas price".
		// Txn prioritization is done via a tipping mechanism instead.
		// We're just using a fixed fee here (and a fixed tip percentage when signing).
		_, err := e.mayaBridge.PostNetworkFee(
			*committedTxn.GetResultantStateIdentifiers().GetStateVersion(),
			common.XRDChain,
			1,
			XrdFeeEstimateInMayaSubunits.Uint64())
		return err
	}
	return nil
}

func (e *RadixScanner) triggerSolvencyReporter(height int64) {
	if e.solvencyReporter != nil {
		if err := e.solvencyReporter(height); err != nil {
			e.logger.Err(err).Msg("fail to report Solvency info to MAYANode")
		}
	}
}

func (e *RadixScanner) convertResourceAddressToAsset(resourceAddress string) (common.Asset, error) {
	token, found := e.tokensByAddress[strings.ToUpper(resourceAddress)]
	if !found {
		e.logger.Warn().Msgf("ignoring deposit event because the token address isn't listed")
		return common.EmptyAsset, nil
	}

	if token.Symbol == common.XRDAsset.Symbol.String() {
		return common.XRDAsset, nil
	}

	asset, err := common.NewAsset(fmt.Sprintf("XRD.%s-%s", token.Symbol, token.Address))
	if err != nil {
		return common.EmptyAsset, fmt.Errorf("failed to create asset: %w", err)
	}

	return asset, nil
}

func (e *RadixScanner) extractTxInItemsFromCommittedTxn(committedTxn models.CommittedTransactionable) ([]stypes.TxInItem, error) {
	stateVersion := *committedTxn.GetResultantStateIdentifiers().GetStateVersion()
	receipt := committedTxn.GetReceipt()

	var txnId string
	switch ledgerTransaction := committedTxn.GetLedgerTransaction().(type) {
	case models.UserLedgerTransactionable:
		userTransaction := ledgerTransaction
		txnId = *userTransaction.
			GetNotarizedTransaction().
			GetSignedIntent().
			GetIntent().
			GetHash()
	default:
		// Not a user transaction, ignore
		return []stypes.TxInItem{}, nil
	}

	version, err := e.mayaBridge.GetMayachainVersion()
	if err != nil {
		e.logger.Error().Err(err).Msgf("fail to get version: err:%s", err)
	}

	routersAddresses := e.pubKeyValidator.GetContracts(common.XRDChain)

	var resultantTxInItems []stypes.TxInItem

	for _, event := range receipt.GetEvents() {
		emitter := event.GetTypeEscaped().GetEmitter()
		emitterIdentifierable, ok := emitter.(models.MethodEventEmitterIdentifierable)
		if ok {
			emitterEntity, err := common.NewAddress(*emitterIdentifierable.GetEntity().GetEntityAddress(), version)
			if err != nil {
				// Ignore `resultantTxInItems` collected so far and return an error
				return []stypes.TxInItem{}, fmt.Errorf("invalid event emitter address: %w", err)
			}
			eventName := *event.GetTypeEscaped().GetName()
			if slices.Contains(routersAddresses, emitterEntity) {
				switch eventName {
				case router.DepositEventName:
					depositEvent, err := router.DecodeDepositEventFromApiEvent(event, e.network)
					if err != nil {
						// Ignore `resultantTxInItems` collected so far and return an error
						return []stypes.TxInItem{}, fmt.Errorf("could not decode router deposit event: %w", err)
					}
					txInItem := stypes.TxInItem{}
					txInItem.BlockHeight = stateVersion
					txInItem.Tx = txnId
					txInItem.Memo = depositEvent.Memo
					txInItem.Sender = depositEvent.Sender
					txInItem.To = depositEvent.VaultAddress

					asset, err := e.convertResourceAddressToAsset(depositEvent.ResourceAddress)
					if err != nil {
						return []stypes.TxInItem{}, fmt.Errorf("could not convert resource address at state version %d: %w", stateVersion, err)
					}
					// We use this to indicate a non-fatal error, which results in the event being ignored
					if asset == common.EmptyAsset {
						e.logger.Warn().Msgf("ignoring deposit event at state version %d", stateVersion)
						continue
					}

					amountInMayaSubunits := DecimalSubunitsToMayaRoundingDown(depositEvent.AmountInDecimalSubunits)
					txInItem.Coins = []common.Coin{common.NewCoin(asset, amountInMayaSubunits)}

					totalFee, err := GetTotalTxnFee(receipt)
					if err != nil {
						// Ignore `resultantTxInItems` collected so far and return an error
						return []stypes.TxInItem{}, fmt.Errorf("could not calculate transaction fee: %w", err)
					}
					txInItem.Gas = totalFee
					e.logger.Info().Msgf("extracted deposit txIn item at height %d memo %s sender %s to %s coins %s gas %s xrd amount %s",
						stateVersion, depositEvent.Memo, txInItem.Sender, txInItem.To, txInItem.Coins, txInItem.Gas, depositEvent.AmountInDecimalSubunits)
					resultantTxInItems = append(resultantTxInItems, txInItem)
				case router.WithdrawEventName:
					withdrawEvent, err := router.DecodeWithdrawEventFromApiEvent(event, e.network)
					if err != nil {
						// Ignore `resultantTxInItems` collected so far and return an error
						return []stypes.TxInItem{}, fmt.Errorf("could not decode router withdraw event: %w", err)
					}
					txInItem := stypes.TxInItem{}
					txInItem.BlockHeight = stateVersion
					txInItem.Tx = txnId
					txInItem.Memo = withdrawEvent.Memo
					txInItem.Sender = withdrawEvent.VaultAddress
					txInItem.To = withdrawEvent.IntendedRecipient

					asset, err := e.convertResourceAddressToAsset(withdrawEvent.ResourceAddress)
					if err != nil {
						return []stypes.TxInItem{}, fmt.Errorf("could not convert resource address at state version %d: %w", stateVersion, err)
					}
					// We use this to indicate a non-fatal error, which results in the event being ignored
					if asset == common.EmptyAsset {
						e.logger.Warn().Msgf("ignoring withdraw event at state version %d", stateVersion)
						continue
					}

					amountInMayaSubunits := DecimalSubunitsToMayaRoundingUp(withdrawEvent.AmountInDecimalSubunits)
					txInItem.Coins = []common.Coin{common.NewCoin(asset, amountInMayaSubunits)}

					totalFee, err := GetTotalTxnFee(receipt)
					if err != nil {
						// Ignore `resultantTxInItems` collected so far and return an error
						return []stypes.TxInItem{}, fmt.Errorf("could not calculate transaction fee: %w", err)
					}
					txInItem.Gas = totalFee
					txInItem.Aggregator = withdrawEvent.AggregatorAddress
					txInItem.AggregatorTarget = withdrawEvent.AggregatorTargetAddress
					txInItem.AggregatorTargetLimit = withdrawEvent.AggregatorMinAmount
					e.logger.Info().Msgf("extracted withdrawal txIn item at height %d memo %s sender %s to %s aggregator (%s %s %s) coins %s gas %s xrd amount %s",
						stateVersion, withdrawEvent.Memo, txInItem.Sender, txInItem.To, txInItem.Aggregator, txInItem.AggregatorTarget, txInItem.AggregatorTargetLimit, txInItem.Coins, txInItem.Gas, withdrawEvent.AmountInDecimalSubunits)
					resultantTxInItems = append(resultantTxInItems, txInItem)
				}
			}
		}
	}

	return resultantTxInItems, nil
}

func (e *RadixScanner) GetHeight() (int64, error) {
	return e.coreApiWrapper.GetCurrentStateVersion()
}

// func (e *RadixScanner) getContextForApiCalls() (context.Context, context.CancelFunc) {
// 	return context.WithTimeout(context.Background(), e.cfg.HTTPRequestTimeout)
// }
