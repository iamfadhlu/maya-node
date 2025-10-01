package thorchain

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	rpcclient "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/hashicorp/go-retryablehttp"

	cmttypes "github.com/cometbft/cometbft/types"
	ctypes "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/tendermint/tendermint/crypto/tmhash"

	mayachaintypes "gitlab.com/mayachain/mayanode/x/mayachain/types"

	"gitlab.com/mayachain/mayanode/bifrost/blockscanner"
	"gitlab.com/mayachain/mayanode/bifrost/mayaclient"
	"gitlab.com/mayachain/mayanode/bifrost/mayaclient/types"
	"gitlab.com/mayachain/mayanode/bifrost/metrics"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/kuji/wasm"
	"gitlab.com/mayachain/mayanode/config"
)

// SolvencyReporter is to report solvency info to THORNode
type SolvencyReporter func(int64) error

const (
	// FeeUpdatePeriodBlocks is the block interval at which we report gas fee changes.
	FeeUpdatePeriodBlocks = 10

	// Fee Endpoint
	FeeEndpoint = "/thorchain/network"
	DefaultFee  = 2000000

	// GasLimit is the default gas limit we will use for all outbound transactions.
	GasLimit = 4000000000
)

var (
	_                     ctypes.Msg = &mayachaintypes.MsgSend{}
	_                     ctypes.Msg = &banktypes.MsgSend{}
	_                     ctypes.Msg = &wasm.MsgExecuteContract{}
	ErrInvalidScanStorage            = errors.New("scan storage is empty or nil")
	ErrInvalidMetrics                = errors.New("metrics is empty or nil")
	ErrEmptyTx                       = errors.New("empty tx")
)

// CosmosBlockScanner is to scan the blocks
type CosmosBlockScanner struct {
	cfg              config.BifrostBlockScannerConfiguration
	logger           zerolog.Logger
	db               blockscanner.ScannerStorage
	cdc              *codec.ProtoCodec
	txConfig         client.TxConfig
	rpc              CometBFTRPC
	lastFee          ctypes.Uint
	httpClient       *retryablehttp.Client
	bridge           mayaclient.MayachainBridge
	solvencyReporter SolvencyReporter
}

// NewCosmosBlockScanner create a new instance of BlockScan
func NewCosmosBlockScanner(cfg config.BifrostBlockScannerConfiguration,
	scanStorage blockscanner.ScannerStorage,
	bridge mayaclient.MayachainBridge,
	m *metrics.Metrics,
	solvencyReporter SolvencyReporter,
) (*CosmosBlockScanner, error) {
	if scanStorage == nil {
		return nil, errors.New("scanStorage is nil")
	}
	if m == nil {
		return nil, errors.New("metrics is nil")
	}

	logger := log.Logger.With().Str("module", "blockscanner").Str("chain", cfg.ChainID.String()).Logger()

	// Bifrost only supports an "RPCHost" in its configuration.
	// We also need to access GRPC for Cosmos chains

	// Registry for decoding txs
	registry := bridge.GetContext().InterfaceRegistry

	// Thorchain's MsgSend can be decoded as a ctypes.Msg,
	// Necessary when using the TxDecoder to decode the transaction bytes from CometBFT
	banktypes.RegisterInterfaces(registry)
	registry.RegisterImplementations((*ctypes.Msg)(nil), &mayachaintypes.MsgSend{})
	registry.RegisterImplementations((*ctypes.Msg)(nil), &banktypes.MsgSend{})
	registry.RegisterImplementations((*ctypes.Msg)(nil), &wasm.MsgExecuteContract{})

	cdc := codec.NewProtoCodec(registry)

	// Registry for encoding txs
	txConfig := tx.NewTxConfig(cdc, []signingtypes.SignMode{signingtypes.SignMode_SIGN_MODE_DIRECT})
	rpcClient, err := rpcclient.New(cfg.RPCHost, "/websocket")
	if err != nil {
		logger.Fatal().Err(err).Msg("fail to create tendemrint rpcclient")
	}

	httpClient := retryablehttp.NewClient()
	httpClient.Logger = nil

	return &CosmosBlockScanner{
		cfg:              cfg,
		logger:           logger,
		db:               scanStorage,
		cdc:              cdc,
		txConfig:         txConfig,
		rpc:              rpcClient,
		lastFee:          ctypes.NewUint(0),
		bridge:           bridge,
		httpClient:       httpClient,
		solvencyReporter: solvencyReporter,
	}, nil
}

// GetHeight returns the height from the latest block minus 1
// NOTE: we must lag by one block due to a race condition fetching the block results
// Since the GetLatestBlockRequests tells what transactions will be in the block at T+1
func (c *CosmosBlockScanner) GetHeight() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	resultBlock, err := c.rpc.Block(ctx, nil)
	if err != nil {
		return 0, err
	}

	return resultBlock.Block.Header.Height - 1, nil
}

// FetchMemPool returns nothing since we are only concerned about finalized transactions in Cosmos
func (c *CosmosBlockScanner) FetchMemPool(height int64) (types.TxIn, error) {
	return types.TxIn{}, nil
}

// GetBlock returns a CometBFT block as a reference to a ResultBlock for a
// given height. As noted above, this is not necessarily the final state of transactions
// and must be checked again for success by getting the BlockResults in FetchTxs
func (c *CosmosBlockScanner) GetBlock(height int64) (*cmttypes.Block, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	resultBlock, err := c.rpc.Block(ctx, &height)
	if err != nil {
		c.logger.Error().Int64("height", height).Msgf("failed to get block: %v", err)
		return nil, fmt.Errorf("failed to get block: %w", err)
	}

	return resultBlock.Block, nil
}

func (c *CosmosBlockScanner) getFee() (ctypes.Uint, error) {
	uri := url.URL{
		Scheme: "http",
		Host:   c.cfg.ChainAPI,
		Path:   FeeEndpoint,
	}

	result, _, err := c.get(uri.String())
	if err != nil {
		c.logger.Error().Msgf("failed to get fee: %v", err)
		return ctypes.NewUint(DefaultFee), err
	}

	var networkResp NetworkResponse
	err = json.Unmarshal(result, &networkResp)
	if err != nil {
		c.logger.Error().Msgf("failed to unmarshal fee: %v", err)
		return ctypes.NewUint(DefaultFee), err
	}

	// string to uint64
	fee, err := ctypes.ParseUint(networkResp.NativeTxFeeRune)
	if err != nil {
		c.logger.Error().Msgf("failed to parse fee: %v", err)
		return ctypes.NewUint(DefaultFee), err
	}

	return fee, nil
}

func (c *CosmosBlockScanner) get(url string) ([]byte, int, error) {
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, http.StatusNotFound, fmt.Errorf("failed to GET from thorchain: %w", err)
	}
	defer func() {
		if err = resp.Body.Close(); err != nil {
			c.logger.Error().Err(err).Msg("failed to close response body")
		}
	}()

	buf, err := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return buf, resp.StatusCode, errors.New("Status code: " + resp.Status + " returned")
	}
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}
	return buf, resp.StatusCode, nil
}

func (c *CosmosBlockScanner) updateFee(height int64) error {
	fee, err := c.getFee()
	if err != nil {
		return err
	}

	// sanity check the fee is not zero
	if fee.IsZero() {
		return errors.New("suggested gas fee was zero")
	}

	// Check if UpdateThorFeeBlocks mimir value is present
	updateBlocks, err := c.bridge.GetMimir("UpdateThorFeeBlocks")
	if err != nil || updateBlocks <= 0 {
		updateBlocks = FeeUpdatePeriodBlocks
	}

	// post the gas fee over every cache period when we have a full gas cache or when gas fee is different from last one
	if height%updateBlocks == 0 || !fee.Equal(c.lastFee) {
		// NOTE: We post the fee to the network instead of the transaction rate, and set the
		// transaction size 1 to ensure the MaxGas in the generated TxOut contains the
		// correct fee.
		feeTx, err := c.bridge.PostNetworkFee(height, c.cfg.ChainID, 1, fee.Uint64())
		if err != nil {
			return err
		}
		c.lastFee = fee
		c.logger.Info().
			Str("tx", feeTx.String()).
			Uint64("fee", fee.Uint64()).
			Int64("height", height).
			Msg("sent network fee to MAYAChain")
	}

	return nil
}

func (c *CosmosBlockScanner) processTxs(height int64, rawTxs []cmttypes.Tx) ([]types.TxInItem, error) {
	// Proto types for Cosmos chains that we are transacting with may not be included in this repo.
	// Therefore, it is necessary to include them in the "proto" directory and register them in
	// the cdc (codec) that is passed below. Registry occurs in the NewCosmosBlockScanner function.
	decoder := tx.DefaultTxDecoder(c.cdc)

	// Fetch the block results so that we can ensure the transaction was successful before processing a TxInItem
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	blockResults, err := c.rpc.BlockResults(ctx, &height)
	if err != nil {
		return []types.TxInItem{}, fmt.Errorf("unable to get BlockResults: %w", err)
	}

	var txIn []types.TxInItem
	for i, rawTx := range rawTxs {
		hash := hex.EncodeToString(tmhash.Sum(rawTx))
		tx, err := decoder(rawTx)
		if err != nil {
			c.logger.Debug().Str("tx", string(rawTx)).Err(err).Msg("unable to decode msg")
			if strings.Contains(err.Error(), "unable to resolve type URL") {
				// One of the transaction message contains an unknown type
				// Though the transaction may contain valid MsgSend, we only support transactions
				// containing MsgSend.
				// Check for these in the error before discarding the transaction.
				if strings.Contains(err.Error(), "MsgSend") { // || strings.Contains(err.Error(), "MsgExecuteContract") {
					// double check to make sure MsgSend isn't mentioned
					c.logger.Error().Str("tx", string(rawTx)).Err(err).Msg("unable to decode msg")
				}
			}
			continue
		}

		feeTx, _ := tx.(ctypes.FeeTx)
		fees := feeTx.GetFee()
		mem, _ := tx.(ctypes.TxWithMemo)
		memo := mem.GetMemo()

		for _, msg := range tx.GetMsgs() {
			txData := NewTxData(hash, height, i, memo, fees, blockResults)
			txin, err := c.processMessage(msg, txData)
			if err != nil {
				c.logger.Debug().Err(err).Msg("unable to process message")
				continue
			}
			txIn = append(txIn, txin...)
		}

	}

	return txIn, nil
}

// ProcessMessage processes a message with the appropriate processor
func (c *CosmosBlockScanner) processMessage(msg ctypes.Msg, txData TxData) ([]types.TxInItem, error) {
	// Type assert the message and processor
	var txIns []types.TxInItem
	var txIn types.TxInItem
	var err error
	switch m := msg.(type) {
	case *banktypes.MsgSend:
		txIn, err = processBankMsgSend(m, txData, c.cfg.ChainID.GetGasAsset(), c.lastFee)
		if err != nil {
			return nil, err
		}
		txIns = append(txIns, txIn)
	case *mayachaintypes.MsgSend:
		txIn, err = processThorchainMsgSend(m, txData, c.cfg.ChainID.GetGasAsset(), c.lastFee)
		if err != nil {
			return nil, err
		}
		txIns = append(txIns, txIn)
	// case *wasm.MsgExecuteContract:
	// 	c.logger.Info().Msg("Processing wasm message")
	// 	txIns, err = processWasmMsgExecuteContract(m, txData)
	default:
		return nil, fmt.Errorf("invalid processor type for message type: %s", m.String())
	}

	return txIns, err
}

func (c *CosmosBlockScanner) FetchTxs(height, chainHeight int64) (types.TxIn, error) {
	block, err := c.GetBlock(height)
	if err != nil {
		return types.TxIn{}, err
	}

	txs, err := c.processTxs(height, block.Data.Txs)
	if err != nil {
		return types.TxIn{}, err
	}

	txIn := types.TxIn{
		Count:    strconv.Itoa(len(txs)),
		Chain:    c.cfg.ChainID,
		TxArray:  txs,
		Filtered: false,
		MemPool:  false,
	}

	// skip reporting network fee and solvency if block more than flexibility blocks from tip
	if chainHeight-height > c.cfg.ObservationFlexibilityBlocks {
		return txIn, nil
	}

	err = c.updateFee(height)
	if err != nil {
		c.logger.Err(err).Int64("height", height).Msg("unable to update network fee")
	}

	if err = c.solvencyReporter(height); err != nil {
		c.logger.Err(err).Msg("fail to send solvency to THORChain")
	}

	return txIn, nil
}
