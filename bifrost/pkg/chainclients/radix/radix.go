package radix

import (
	_ "embed"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/common/tokenlist"

	ecrypto "github.com/ethereum/go-ethereum/crypto"

	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/radix/coreapi"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/radix/decimal"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/radix/router"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/radix/types"
	mem "gitlab.com/mayachain/mayanode/x/mayachain/memo"

	ret "github.com/radixdlt/radix-engine-toolkit-go/v2/radix_engine_toolkit_uniffi"

	"github.com/cosmos/cosmos-sdk/crypto/codec"
	auth "github.com/microsoft/kiota-abstractions-go/authentication"
	http "github.com/microsoft/kiota-http-go"
	apiclient "github.com/radixdlt/maya/radix_core_api_client"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	tssp "gitlab.com/mayachain/mayanode/bifrost/tss/go-tss/tss"

	"gitlab.com/mayachain/mayanode/bifrost/blockscanner"
	"gitlab.com/mayachain/mayanode/bifrost/mayaclient"
	stypes "gitlab.com/mayachain/mayanode/bifrost/mayaclient/types"
	"gitlab.com/mayachain/mayanode/bifrost/metrics"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/shared/runners"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/shared/signercache"
	"gitlab.com/mayachain/mayanode/bifrost/pubkeymanager"
	"gitlab.com/mayachain/mayanode/bifrost/tss"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/config"
	"gitlab.com/mayachain/mayanode/constants"
)

type Client struct {
	logger                  zerolog.Logger
	cfg                     config.BifrostChainConfiguration
	network                 types.Network
	coreApiWrapper          *coreapi.CoreApiWrapper
	mayaBridge              mayaclient.MayachainBridge
	radixScanner            *RadixScanner
	blockScanner            *blockscanner.BlockScanner
	localPubKey             common.PubKey
	tssKeySign              *tss.KeySign
	wg                      *sync.WaitGroup
	stopchan                chan struct{}
	globalSolvencyQueue     chan stypes.Solvency
	signerCacheManager      *signercache.CacheManager
	pubKeyValidator         pubkeymanager.PubKeyValidator
	signerHelper            *SignerHelper
	lastSolvencyCheckHeight int64
	tokensBySymbol          map[string]tokenlist.RadixToken
}

func CreateRadixCoreApiClient(baseUrl string) (*apiclient.RadixCoreApiClient, error) {
	adapter, err := http.
		NewNetHttpRequestAdapterWithParseNodeFactoryAndSerializationWriterFactoryAndHttpClient(
			&auth.AnonymousAuthenticationProvider{},
			nil,
			nil,
			http.GetDefaultClient(
				http.NewRetryHandler(),
				http.NewRedirectHandler(),
				http.NewParametersNameDecodingHandler(),
				http.NewUserAgentHandler(),
				http.NewHeadersInspectionHandler(),
			),
		)
	if err != nil {
		return nil, err
	}
	adapter.SetBaseUrl(baseUrl)
	return apiclient.NewRadixCoreApiClient(adapter), nil
}

func NewClient(
	mayaKeys *mayaclient.Keys,
	cfg config.BifrostChainConfiguration,
	tssServer *tssp.TssServer,
	mayaBridge mayaclient.MayachainBridge,
	metrics *metrics.Metrics,
	pubKeyValidator pubkeymanager.PubKeyValidator,
) (*Client, error) {
	if mayaBridge == nil {
		return nil, errors.New("mayaBridge is nil")
	}

	if mayaKeys == nil {
		return nil, fmt.Errorf("mayaKeys is nil")
	}

	tssKeySign, err := tss.NewKeySign(tssServer, mayaBridge)
	if err != nil {
		return nil, fmt.Errorf("fail to create tss key sign: %w", err)
	}

	priv, err := mayaKeys.GetPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("fail to get private key: %w", err)
	}

	tmPubKey, err := codec.ToTmPubKeyInterface(priv.PubKey())
	if err != nil {
		return nil, fmt.Errorf("fail to get tm pub key: %w", err)
	}
	localPubKey, err := common.NewPubKeyFromCrypto(tmPubKey)
	if err != nil {
		return nil, fmt.Errorf("fail to get pub key: %w", err)
	}

	network := types.NetworkFromChainNetwork(common.CurrentChainNetwork)

	radixApiClient, err := CreateRadixCoreApiClient(cfg.RPCHost)
	if err != nil {
		return nil, fmt.Errorf("failed to create Radix API client: %w", err)
	}

	coreApiWrapper := coreapi.NewCoreApiWrapper(radixApiClient, network, cfg.BlockScanner.HTTPRequestTimeout)

	// This can be empty, in which case `NewBlockScannerStorage` will use an in-memory blockScannerStorage
	var blockScannerDbPath string
	if len(cfg.BlockScanner.DBPath) > 0 {
		blockScannerDbPath = fmt.Sprintf("%s/%s", cfg.BlockScanner.DBPath, cfg.BlockScanner.ChainID)
	}

	blockScannerStorage, err := blockscanner.NewBlockScannerStorage(blockScannerDbPath, cfg.ScannerLevelDB)
	if err != nil {
		return nil, fmt.Errorf("fail to create block scanner storage: %w", err)
	}

	signerCacheManager, err := signercache.NewSignerCacheManager(blockScannerStorage.GetInternalDb())
	if err != nil {
		return nil, fmt.Errorf("fail to create signer cache manager")
	}

	tokensBySymbol := make(map[string]tokenlist.RadixToken)
	tokensByAddress := make(map[string]tokenlist.RadixToken)
	for _, token := range tokenlist.GetRadixTokenList(common.LatestVersion) {
		if token.Symbol == "XRD" {
			tokensBySymbol[token.Symbol] = token
		} else {
			tokensBySymbol[strings.ToUpper(fmt.Sprintf("%s-%s", token.Symbol, token.Address))] = token
		}
		tokensByAddress[strings.ToUpper(token.Address)] = token
	}

	radixScanner, err := NewRadixScanner(cfg.BlockScanner, blockScannerStorage, &coreApiWrapper, mayaBridge, metrics, pubKeyValidator, network, tokensByAddress)
	if err != nil {
		return nil, fmt.Errorf("fail to create radix block scanner: %w", err)
	}

	blockScanner, err := blockscanner.NewBlockScanner(cfg.BlockScanner, blockScannerStorage, metrics, mayaBridge, radixScanner)
	if err != nil {
		return nil, fmt.Errorf("fail to create block scanner: %w", err)
	}

	logger := log.With().Str("module", "radix").Logger()

	retBuildInfo := ret.GetBuildInformation()
	logger.Info().Msgf("Radix Engine Toolkit version: %s", retBuildInfo.Version)

	ecdsaPrivateKey, err := ecrypto.ToECDSA(priv.Bytes())
	if err != nil {
		return nil, err
	}
	signerHelper := NewSignerHelper(ecdsaPrivateKey, localPubKey, tssKeySign, mayaBridge)

	client := &Client{
		logger:             logger,
		cfg:                cfg,
		network:            network,
		coreApiWrapper:     &coreApiWrapper,
		mayaBridge:         mayaBridge,
		radixScanner:       radixScanner,
		blockScanner:       blockScanner,
		localPubKey:        localPubKey,
		tssKeySign:         tssKeySign,
		wg:                 &sync.WaitGroup{},
		stopchan:           make(chan struct{}),
		signerCacheManager: signerCacheManager,
		pubKeyValidator:    pubKeyValidator,
		signerHelper:       signerHelper,
		tokensBySymbol:     tokensBySymbol,
	}

	radixScanner.solvencyReporter = client.ReportSolvency

	return client, nil
}

func (c *Client) SignTx(tx stypes.TxOutItem, height int64) ([]byte, []byte, *stypes.TxInItem, error) {
	c.logger.Info().Msgf("signTx %+v at height %d", tx, height)

	if c.signerCacheManager.HasSigned(tx.CacheHash()) {
		c.logger.Info().Msgf("transaction(%+v), signed before, ignore", tx)
		return nil, nil, nil, nil
	}

	if tx.ToAddress.IsEmpty() {
		return nil, nil, nil, fmt.Errorf("to address is empty")
	}
	if tx.VaultPubKey.IsEmpty() {
		return nil, nil, nil, fmt.Errorf("vault public key is empty")
	}
	if len(tx.Memo) == 0 {
		return nil, nil, nil, fmt.Errorf("can't sign tx when it doesn't have memo")
	}

	memo, err := mem.ParseMemo(common.LatestVersion, tx.Memo)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fail to parse memo(%s):%w", tx.Memo, err)
	}

	if memo.GetType() == mem.TxMigrate && (tx.Aggregator != "" || tx.AggregatorTargetAsset != "") {
		return nil, nil, nil, fmt.Errorf("migration can't use aggregators")
	}

	if memo.IsInbound() {
		return nil, nil, nil, fmt.Errorf("inbound memo should not be used for outbound tx")
	}

	var coin common.Coin
	if len(tx.Coins) == 1 {
		coin = tx.Coins[0]
	} else {
		return nil, nil, nil, fmt.Errorf("radix txn out must contain exactly one coin")
	}

	token, found := c.tokensBySymbol[strings.ToUpper(coin.Asset.Symbol.String())]
	if !found {
		return nil, nil, nil, fmt.Errorf("unknown token: %s", coin.Asset.Symbol.String())
	}

	amountInDecimalSubunits := MayaSubunitsToRadixDecimalSubunits(coin.Amount)
	// round down to match token decimals/divisibility
	decimalsDiff := cosmos.NewUint(uint64(RadixDecimals - token.Decimals))
	if !decimalsDiff.IsZero() {
		roundingFactor := new(big.Int).Exp(big.NewInt(10), decimalsDiff.BigInt(), nil).Uint64()
		amountInDecimalSubunits = amountInDecimalSubunits.QuoUint64(roundingFactor).MulUint64(roundingFactor)
	}

	decimalAmount, err := decimal.UintSubunitsToDecimal(amountInDecimalSubunits)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create decimal amount: %w", err)
	}

	c.logger.Info().Msgf("converted %s subunits of %s to %s Decimal subunits (%s)",
		coin.Amount, coin.Asset, amountInDecimalSubunits, decimalAmount.AsStr())

	if len(tx.MaxGas) != 1 || tx.MaxGas[0].Asset != common.XRDAsset {
		return nil, nil, nil, fmt.Errorf("maxGas must contain exactly one XRD asset amount")
	}
	maxGasAmountInXrdSubunits := MayaSubunitsToRadixDecimalSubunits(tx.MaxGas[0].Amount)
	maxFeeDecimal, err := decimal.UintSubunitsToDecimal(maxGasAmountInXrdSubunits)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to convert max gas amount to decimal: %w", err)
	}

	currentEpoch, err := c.coreApiWrapper.GetCurrentEpoch()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get current epoch: %w", err)
	}

	// An address of the router used for funds withdrawals. This is the router component ("contract"), that's
	// assigned to the transaction initiator/signer vault (i.e. `tx.VaultPubKey`).
	routerForWithdrawals := c.pubKeyValidator.GetContract(common.XRDChain, tx.VaultPubKey)
	if routerForWithdrawals.IsEmpty() {
		return nil, nil, nil, fmt.Errorf("can't sign tx, failed to get router address")
	}

	// When a router component is updated to a new instance, during churn funds will be transferred out
	// from `routerForWithdrawals` (i.e. the old router) from the account of `tx.VaultPubKey`
	// and deposited to a (new) router component corresponding to `tx.ToAddress`.
	// If the router doesn't update then `routerForWithdrawals` and `routerForDeposits` point to the same component.
	var routerForDeposits common.Address
	switch memo.GetType() {
	case mem.TxMigrate:
		routerForDeposits, err = c.getRouterAddressByVaultAddress(tx.ToAddress)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to get new router address for TxMigrate: %w", err)
		}
	default:
		routerForDeposits = routerForWithdrawals
	}

	// `height` can overflow uint32, but this is fine for the purpose of Radix nonces,
	// which don't need to be unique or increasing.
	// They're just an arbitrary transaction intent discriminator values.
	nonce := uint32(height)

	signedIntent, err := router.BuildTransferOutTransaction(
		routerForWithdrawals.String(),
		routerForDeposits.String(),
		uint64(currentEpoch),
		nonce,
		token.Address,
		decimalAmount,
		memo,
		maxFeeDecimal,
		c.network,
		tx,
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to build transfer out transaction: %w", err)
	}
	intentHash, err := signedIntent.IntentHash()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to compute intent hash: %w", err)
	}
	signedIntentHash, err := signedIntent.SignedIntentHash()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to compute signed intent hash: %w", err)
	}

	c.logger.Info().Msgf("radix txn for signing: intent_hash=%s, signed_intent_hash=%s, currentEpoch=%d, nonce=%d, memo=%s, maxFeeDecimal=%s",
		intentHash.AsStr(), signedIntentHash.AsStr(), currentEpoch, nonce, memo, maxFeeDecimal.AsStr())

	notarizedTransaction, err := c.signerHelper.Sign(tx, signedIntent, tx.VaultPubKey, height)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to sign: %w", err)
	}

	notarizedTransactionBytes, err := notarizedTransaction.Compile()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to compile notarized transaction: %w", err)
	}
	notarizedTransactionHash, err := notarizedTransaction.NotarizedTransactionHash()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to compute notarized transaction hash: %w", err)
	}

	c.logger.Info().Msgf("radix signed txn: intent_hash=%s, notarized_transaction_hash=%s",
		intentHash.AsStr(), notarizedTransactionHash.AsStr())

	return notarizedTransactionBytes, nil, nil, nil
}

func (c *Client) getRouterAddressByVaultAddress(addr common.Address) (common.Address, error) {
	for _, pk := range c.pubKeyValidator.GetPubKeys() {
		radixAddr, err := pk.GetAddress(common.XRDChain)
		if err != nil {
			return common.NoAddress, err
		}
		if radixAddr.Equals(addr) {
			return c.pubKeyValidator.GetContract(common.XRDChain, pk), nil
		}
	}
	return common.NoAddress, fmt.Errorf("could not find contract address")
}

func (c *Client) BroadcastTx(txOutItem stypes.TxOutItem, notarizedTransactionBytes []byte) (string, error) {
	// 1. Parse the transaction
	notarizedTransaction, err := ret.NotarizedTransactionDecompile(notarizedTransactionBytes)
	if err != nil {
		c.logger.Error().Err(err).Msg("failed to decompile notarized transaction to broadcast")
		return "", err
	}

	txId, err := notarizedTransaction.IntentHash()
	if err != nil {
		c.logger.Error().Err(err).Msg("failed to retrieve intent hash")
		return "", err
	}

	notarizedTxnHash, err := notarizedTransaction.Hash()
	if err != nil {
		c.logger.Error().Err(err).Msg("failed to retrieve notarized transaction hash")
		return "", err
	}

	// 2. Submit it
	err = c.coreApiWrapper.SubmitTransaction(notarizedTransactionBytes)
	if err != nil {
		c.logger.Error().Str("txid", txId.AsStr()).Err(err).Msg("notarized transaction submit failed")
		return "", err
	}

	// 3. Update the cache manager
	if err := c.signerCacheManager.SetSigned(txOutItem.CacheHash(), txOutItem.CacheVault(c.GetChain()), txId.AsStr()); err != nil {
		c.logger.Err(err).Msgf("fail to mark tx out item (%+v) as signed", txOutItem)
	}

	c.logger.Info().Msgf("radix tx broadcasted txId: %s notarizedTxHash: %s", txId.AsStr(), notarizedTxnHash.AsStr())

	return txId.AsStr(), nil
}

func (c *Client) GetHeight() (int64, error) {
	return c.radixScanner.GetHeight()
}

func (c *Client) GetBlockScannerHeight() (int64, error) {
	return c.blockScanner.PreviousHeight(), nil
}

func (c *Client) GetLatestTxForVault(vault string) (string, string, error) {
	lastObserved, err := c.signerCacheManager.GetLatestRecordedTx(stypes.InboundCacheKey(vault, c.GetChain().String()))
	if err != nil {
		return "", "", err
	}
	lastBroadCasted, err := c.signerCacheManager.GetLatestRecordedTx(stypes.BroadcastCacheKey(vault, c.GetChain().String()))
	return lastObserved, lastBroadCasted, err
}

func (c *Client) GetAddress(poolPubKey common.PubKey) string {
	addr, err := poolPubKey.GetAddress(common.XRDChain)
	if err != nil {
		c.logger.Error().Err(err).Str("pool_pub_key", poolPubKey.String()).Msg("fail to get pool address")
		return ""
	}
	return addr.String()
}

func (c *Client) GetAccount(pkey common.PubKey, height *big.Int) (common.Account, error) {
	xrdCoins, err := c.getOnChainBalances(pkey, []common.Asset{common.XRDAsset})
	if err != nil {
		return common.Account{}, err
	}
	return common.Account{
		Sequence:      0,
		AccountNumber: 0,
		Coins:         xrdCoins,
		HasMemoFlag:   false,
	}, nil
}

func (c *Client) getOnChainBalances(pkey common.PubKey, assets []common.Asset) (common.Coins, error) {
	var coins []common.Coin

	routerAddresses := c.pubKeyValidator.GetContracts(common.XRDChain)
	if len(routerAddresses) < 1 {
		return common.Coins{}, fmt.Errorf("radix router address is missing")
	}
	routerAddress := routerAddresses[0].String()

	for _, asset := range assets {
		token, found := c.tokensBySymbol[strings.ToUpper(asset.Symbol.String())]
		if !found {
			c.logger.Error().Str("asset", asset.Symbol.String()).Msg("unknown token")
			continue
		}
		tokenBalanceDec, err := router.GetVaultBalanceInRouter(c.coreApiWrapper, routerAddress, pkey, token.Address, c.network.Id)
		if err != nil {
			c.logger.Error().Str("asset", asset.Symbol.String()).Err(err).Msg("failed to get vault token balance")
			continue
		}
		tokenBalanceUintSubunits, err := decimal.NonNegativeDecimalToUintSubunits(tokenBalanceDec)
		if err != nil {
			c.logger.Error().Str("asset", asset.Symbol.String()).Err(err).Msg("failed to convert token balance to uint")
			continue
		}
		xrdBalanceMayaSubunits := DecimalSubunitsToMayaRoundingDown(tokenBalanceUintSubunits)

		coins = append(coins, common.NewCoin(asset, xrdBalanceMayaSubunits))
	}

	return coins, nil
}

func (c *Client) GetAccountByAddress(address string, height *big.Int) (common.Account, error) {
	return common.Account{}, nil
}

func (c *Client) GetChain() common.Chain {
	return common.XRDChain
}

func (c *Client) OnObservedTxIn(txIn stypes.TxInItem, blockHeight int64) {
	m, err := mem.ParseMemo(common.LatestVersion, txIn.Memo)
	if err != nil {
		c.logger.Err(err).Str("memo", txIn.Memo).Msg("fail to parse memo")
		return
	}
	if !m.IsOutbound() {
		return
	}
	if m.GetTxID().IsEmpty() {
		return
	}
	if err = c.signerCacheManager.SetSigned(txIn.CacheHash(c.GetChain(), m.GetTxID().String()), txIn.CacheVault(c.GetChain()), txIn.Tx); err != nil {
		c.logger.Err(err).Msg("fail to update signer cache")
	}
}

func (c *Client) Start(globalTxsQueue chan stypes.TxIn, globalErrataQueue chan stypes.ErrataBlock, globalSolvencyQueue chan stypes.Solvency) {
	c.globalSolvencyQueue = globalSolvencyQueue
	c.tssKeySign.Start()
	c.blockScanner.Start(globalTxsQueue)
	c.wg.Add(1) // for SolvencyCheckRunner
	go runners.SolvencyCheckRunner(c.GetChain(), c, c.mayaBridge, c.stopchan, c.wg, constants.MayachainBlockTime)
}

func (c *Client) GetConfig() config.BifrostChainConfiguration {
	return c.cfg
}

func (c *Client) GetConfirmationCount(txIn stypes.TxIn) int64 {
	// Radix has instant finality, so return 0 (similarly to other chains with instant finality, like Avalanche)
	return 0
}

func (c *Client) ConfirmationCountReady(txIn stypes.TxIn) bool {
	// Observed committed Radix transactions don't require any more confirmations.
	// It is also expected that this method returns `true` for empty TxIns (so no need to check if
	// txIn.TxArray is non-empty) and mempool transactions (no need to inspect `txIn.MemPool`).
	return true
}

func (c *Client) IsBlockScannerHealthy() bool {
	return c.blockScanner.IsHealthy()
}

func (c *Client) Stop() {
	c.tssKeySign.Stop()
	c.blockScanner.Stop()

	// Send a close signal to the channel.
	// This will cause the goroutines to stop.
	close(c.stopchan)
	// Wait for the routines to stop.
	c.wg.Wait()
}

func (c *Client) ShouldReportSolvency(xrdHeight int64) bool {
	return xrdHeight-c.lastSolvencyCheckHeight > 1000 // Report every 1000 "blocks"
}

func (c *Client) ReportSolvency(xrdHeight int64) error {
	if !c.ShouldReportSolvency(xrdHeight) {
		return nil
	}

	asgardVaults, err := c.mayaBridge.GetAsgards()
	if err != nil {
		return fmt.Errorf("fail to get asgards: %w", err)
	}

	for _, asgard := range asgardVaults {
		var asgardAssets []common.Asset
		for _, coin := range asgard.Coins {
			if coin.Asset.Chain == common.XRDChain {
				asgardAssets = append(asgardAssets, coin.Asset)
			}
		}

		onChainCoins, err := c.getOnChainBalances(asgard.PubKey, asgardAssets)
		if err != nil {
			c.logger.Err(err).Msgf("fail to get account balances")
			continue
		}
		if runners.IsVaultSolvent(common.Account{Coins: onChainCoins}, asgard, XrdFeeEstimateInMayaSubunits) && c.IsBlockScannerHealthy() {
			// We don't need to report solvency when the vault is solvent.
			// When block scanner is not healthy, usually that means the chain is halted, in that scenario, we continue to report solvency.
			continue
		}
		select {
		case c.globalSolvencyQueue <- stypes.Solvency{
			Height: xrdHeight,
			Chain:  common.XRDChain,
			PubKey: asgard.PubKey,
			Coins:  onChainCoins,
		}:
		case <-time.After(constants.MayachainBlockTime):
			c.logger.Info().Msgf("fail to send solvency info to BASEChain, timeout")
		}
	}
	c.lastSolvencyCheckHeight = xrdHeight
	return nil
}
