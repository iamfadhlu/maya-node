package zcash

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcutil"

	"github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.uber.org/atomic"

	tssp "gitlab.com/mayachain/mayanode/bifrost/tss/go-tss/tss"

	"github.com/btcsuite/btcd/btcec"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	tmcrypto "github.com/tendermint/tendermint/crypto"

	"gitlab.com/mayachain/mayanode/bifrost/blockscanner"
	btypes "gitlab.com/mayachain/mayanode/bifrost/blockscanner/types"
	"gitlab.com/mayachain/mayanode/bifrost/mayaclient"
	"gitlab.com/mayachain/mayanode/bifrost/mayaclient/types"
	"gitlab.com/mayachain/mayanode/bifrost/metrics"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/shared/runners"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/shared/signercache"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/shared/utxo"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/zcash/rpc"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/zcash/txscript"
	"gitlab.com/mayachain/mayanode/bifrost/tss"
	"gitlab.com/mayachain/mayanode/chain/zec/go/zec"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/config"
	"gitlab.com/mayachain/mayanode/constants"
	mem "gitlab.com/mayachain/mayanode/x/mayachain/memo"
)

// BlockCacheSize the number of block meta that get store in storage.
const (
	BlockCacheSize      = 100
	MaximumConfirmation = 99999999
	MaxAsgardAddresses  = 100
	maxUTXOsToSpend     = 10
	MinUTXOConfirmation = 1  // // MinUTXOConfirmation UTXO that has less confirmation then this will not be spent , unless it is yggdrasil
	expiryHeightDelta   = 35 // Should be equal to SigningTransactionPeriod's time in Zcash blocks
)

var ErrTxHassShieldedOuts = fmt.Errorf("transaction contains shielded outputs; UTXO logic not applicable")

// Client observes zcash chain and allows to sign and broadcast tx
type Client struct {
	logger                  zerolog.Logger
	cfg                     config.BifrostChainConfiguration
	m                       *metrics.Metrics
	chain                   common.Chain
	blockScanner            *blockscanner.BlockScanner
	temporalStorage         *utxo.TemporalStorage
	bridge                  mayaclient.MayachainBridge
	globalErrataQueue       chan<- types.ErrataBlock
	globalSolvencyQueue     chan<- types.Solvency
	nodePubKey              common.PubKey
	nodeAddress             common.Address
	currentBlockHeight      *atomic.Int64
	asgardAddresses         []common.Address
	lastAsgard              time.Time
	tssKeySigner            *tss.KeySign
	wg                      *sync.WaitGroup
	lastFeeRate             uint64
	signerLock              *sync.Mutex
	vaultSignerLocks        map[string]*sync.Mutex
	consolidateInProgress   *atomic.Bool
	signerCacheManager      *signercache.CacheManager
	stopchan                chan struct{}
	lastSolvencyCheckHeight int64
	ksWrapper               *KeySignWrapper
	client                  *rpc.ZcashClient
}

// NewClient generates a new Client
func NewClient(
	thorKeys *mayaclient.Keys,
	cfg config.BifrostChainConfiguration,
	server *tssp.TssServer,
	bridge mayaclient.MayachainBridge,
	m *metrics.Metrics,
) (*Client, error) {
	logger := log.Logger.With().Str("module", "zcash").Logger()

	rpcClient, err := rpc.NewZcashClient(cfg.RPCHost, cfg.UserName, cfg.Password, 10, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create Zcash rpc client")
	}

	tssKm, err := tss.NewKeySign(server, bridge)
	if err != nil {
		return nil, fmt.Errorf("fail to create tss signer: %w", err)
	}
	var thorPrivateKey cryptotypes.PrivKey
	thorPrivateKey, err = thorKeys.GetPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("fail to get MAYAChain private key: %w", err)
	}

	var ksWrapper *KeySignWrapper
	var zecPrivateKey *btcec.PrivateKey
	zecPrivateKey, err = getZECPrivateKey(thorPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("fail to convert private key for ZEC: %w", err)
	}
	ksWrapper, err = NewKeySignWrapper(zecPrivateKey, tssKm)
	if err != nil {
		return nil, fmt.Errorf("fail to create keysign wrapper: %w", err)
	}

	var temp tmcrypto.PubKey
	temp, err = codec.ToTmPubKeyInterface(thorPrivateKey.PubKey())
	if err != nil {
		return nil, fmt.Errorf("fail to get tm pub key: %w", err)
	}

	var nodePubKey common.PubKey
	nodePubKey, err = common.NewPubKeyFromCrypto(temp)
	if err != nil {
		return nil, fmt.Errorf("fail to get the node pubkey: %w", err)
	}

	var nodeAddress common.Address
	nodeAddress, err = nodePubKey.GetAddress(common.ZECChain)
	if err != nil {
		return nil, fmt.Errorf("fail to get vault address: %w", err)
	}

	c := &Client{
		logger:                logger,
		cfg:                   cfg,
		m:                     m,
		chain:                 cfg.ChainID,
		client:                rpcClient,
		ksWrapper:             ksWrapper,
		bridge:                bridge,
		nodePubKey:            nodePubKey,
		nodeAddress:           nodeAddress,
		tssKeySigner:          tssKm,
		wg:                    &sync.WaitGroup{},
		signerLock:            &sync.Mutex{},
		vaultSignerLocks:      make(map[string]*sync.Mutex),
		stopchan:              make(chan struct{}),
		consolidateInProgress: atomic.NewBool(false),
		currentBlockHeight:    atomic.NewInt64(0),
	}

	var path string // if not set later, will in memory storage
	if len(c.cfg.BlockScanner.DBPath) > 0 {
		path = fmt.Sprintf("%s/%s", c.cfg.BlockScanner.DBPath, c.cfg.BlockScanner.ChainID)
	}
	storage, err := blockscanner.NewBlockScannerStorage(path, c.cfg.ScannerLevelDB)
	if err != nil {
		return c, fmt.Errorf("fail to create blockscanner storage: %w", err)
	}

	c.blockScanner, err = blockscanner.NewBlockScanner(c.cfg.BlockScanner, storage, m, bridge, c)
	if err != nil {
		return c, fmt.Errorf("fail to create block scanner: %w", err)
	}

	c.temporalStorage, err = utxo.NewTemporalStorage(storage.GetInternalDb(), c.cfg.MempoolTxIDCacheSize)
	if err != nil {
		return c, fmt.Errorf("fail to create temporal storage: %w", err)
	}

	signerCacheManager, err := signercache.NewSignerCacheManager(storage.GetInternalDb())
	if err != nil {
		return nil, fmt.Errorf("fail to create signer cache manager,err: %w", err)
	}
	c.signerCacheManager = signerCacheManager

	return c, nil
}

// Start starts the block scanner
func (c *Client) Start(globalTxsQueue chan types.TxIn, globalErrataQueue chan types.ErrataBlock, globalSolvencyQueue chan types.Solvency) {
	c.globalErrataQueue = globalErrataQueue
	c.globalSolvencyQueue = globalSolvencyQueue
	c.tssKeySigner.Start()
	c.logger.Info().Msgf("Starting ZCASH blockscanner")
	c.blockScanner.Start(globalTxsQueue)
	c.wg.Add(1)
}

// Stop stops the block scanner
func (c *Client) Stop() {
	c.tssKeySigner.Stop()
	c.blockScanner.Stop()
	close(c.stopchan)
	// wait for consolidate utxo to exit
	c.wg.Wait()
}

// GetConfig - get the chain configuration
func (c *Client) GetConfig() config.BifrostChainConfiguration {
	return c.cfg
}

// GetChain returns ZEC Chain
func (c *Client) GetChain() common.Chain {
	return common.ZECChain
}

// GetHeight returns current block height
func (c *Client) GetHeight() (int64, error) {
	height, err := c.client.GetLatestHeight()
	if err != nil {
		return 0, err
	}
	c.logger.Info().Msgf("height %d", height)
	return height, nil
}

// GetBlockScannerHeight returns blockscanner height
func (c *Client) GetBlockScannerHeight() (int64, error) {
	return c.blockScanner.PreviousHeight(), nil
}

func (c *Client) GetLatestTxForVault(vault string) (string, string, error) {
	lastObserved, err := c.signerCacheManager.GetLatestRecordedTx(types.InboundCacheKey(vault, c.GetChain().String()))
	if err != nil {
		return "", "", err
	}
	lastBroadCasted, err := c.signerCacheManager.GetLatestRecordedTx(types.BroadcastCacheKey(vault, c.GetChain().String()))
	return lastObserved, lastBroadCasted, err
}

func (c *Client) IsBlockScannerHealthy() bool {
	return c.blockScanner.IsHealthy()
}

// GetAddress returns address from pubkey
func (c *Client) GetAddress(pubKey common.PubKey) string {
	address, err := pubKey.GetAddress(common.ZECChain)
	if err != nil {
		c.logger.Error().Err(err).Msgf("fail to get pubkey address")
		return ""
	}
	return address.String()
}

// GetAccount returns account with balance for an address
func (c *Client) GetAccount(pkey common.PubKey, height *big.Int) (common.Account, error) {
	if height != nil {
		c.logger.Error().Msg("height was provided but will be ignored")
	}
	address := c.GetAddress(pkey)
	return c.GetAccountByAddress(address, height)
}

func (c *Client) GetAccountByAddress(address string, _ *big.Int) (common.Account, error) {
	account := common.Account{}
	balance, err := c.client.GetAddressBalance(address)
	if err != nil {
		return account, err
	}

	coins := common.Coins{
		common.NewCoin(common.ZECAsset, cosmos.NewUint(balance)),
	}
	account = common.Account{
		Coins: coins,
	}

	return account, nil
}

func (c *Client) getAsgardAddresses() ([]common.Address, error) {
	if time.Since(c.lastAsgard) < constants.MayachainBlockTime && c.asgardAddresses != nil {
		return c.asgardAddresses, nil
	}
	newAddresses, err := utxo.GetAsgardAddresses(c.cfg.ChainID, c.bridge)
	if err != nil {
		return nil, fmt.Errorf("fail to get asgards: %w", err)
	}
	if len(newAddresses) > 0 { // ensure we don't overwrite with empty list
		c.asgardAddresses = newAddresses
	}
	c.lastAsgard = time.Now()
	return c.asgardAddresses, nil
}

func (c *Client) isAsgardAddress(addressToCheck string) bool {
	addresses, err := c.getAsgardAddresses()
	if err != nil {
		c.logger.Err(err).Msgf("fail to get asgard addresses")
		return false
	}
	for _, addr := range addresses {
		if strings.EqualFold(addr.String(), addressToCheck) {
			return true
		}
	}
	return false
}

// OnObservedTxIn gets called from observer when we have a valid observation
// For zcash chain client, the observer is on the Rust side and records
// the UTXO/Shielded notes in its database
func (c *Client) OnObservedTxIn(txIn types.TxInItem, blockHeight int64) {
	hash, err := chainhash.NewHashFromStr(txIn.Tx)
	if err != nil {
		c.logger.Error().Err(err).Str("txID", txIn.Tx).Msg("fail to add spendable utxo to storage")
		return
	}
	blockMeta, err := c.temporalStorage.GetBlockMeta(blockHeight)
	if err != nil {
		c.logger.Err(err).Msgf("fail to get block meta on block height(%d)", blockHeight)
		return
	}
	if blockMeta == nil {
		blockMeta = utxo.NewBlockMeta("", blockHeight, "")
	}
	if _, err = c.temporalStorage.TrackObservedTx(txIn.Tx); err != nil {
		c.logger.Err(err).Msgf("fail to add hash (%s) to observed tx cache", txIn.Tx)
	}
	if c.isAsgardAddress(txIn.Sender) {
		c.logger.Debug().Msgf("add hash %s as self transaction,block height:%d", hash.String(), blockHeight)
		blockMeta.AddSelfTransaction(hash.String())
	} else {
		// add the transaction to block meta
		blockMeta.AddCustomerTransaction(hash.String())
	}
	if err = c.temporalStorage.SaveBlockMeta(blockHeight, blockMeta); err != nil {
		c.logger.Err(err).Msgf("fail to save block meta to storage,block height(%d)", blockHeight)
	}
	// update the signer cache
	m, err := mem.ParseMemo(common.LatestVersion, txIn.Memo)
	if err != nil {
		c.logger.Err(err).Msgf("fail to parse memo: %s", txIn.Memo)
		return
	}
	if !m.IsOutbound() {
		return
	}
	if m.GetTxID().IsEmpty() {
		return
	}
	if err := c.signerCacheManager.SetSigned(txIn.CacheHash(c.GetChain(), m.GetTxID().String()), txIn.CacheVault(c.GetChain()), txIn.Tx); err != nil {
		c.logger.Err(err).Msg("fail to update signer cache")
	}
}

func (c *Client) processReorg(block *rpc.BlockWithTx) ([]types.TxIn, error) {
	previousHeight := block.Height - 1
	prevBlockMeta, err := c.temporalStorage.GetBlockMeta(previousHeight)
	if err != nil {
		return nil, fmt.Errorf("fail to get block meta of height(%d) : %w", previousHeight, err)
	}
	if prevBlockMeta == nil {
		return nil, nil
	}
	// the block's previous hash need to be the same as the block hash chain client recorded in block meta
	// blockMetas[PreviousHeight].BlockHash == Block.PreviousHash
	if strings.EqualFold(prevBlockMeta.BlockHash, block.PreviousHash) {
		return nil, nil
	}

	c.logger.Info().Msgf("re-org detected, current block height: %d, previous block hash is : %s, however block meta at height: %d, block hash is %s", block.Height, block.PreviousHash, prevBlockMeta.Height, prevBlockMeta.BlockHash)
	blockHeights, err := c.reConfirmTx()
	if err != nil {
		c.logger.Err(err).Msgf("fail to reprocess re-orged zcash blocks")
	}
	var txIns []types.TxIn
	for _, item := range blockHeights {
		c.logger.Info().Msgf("rescan block height: %d", item)
		b, err := c.getBlock(item)
		if err != nil {
			c.logger.Err(err).Msgf("fail to get block from RPC for height:%d", item)
			continue
		}
		txIn, err := c.extractTxs(b)
		if err != nil {
			c.logger.Err(err).Msgf("fail to extract txIn from block")
			continue
		}

		if len(txIn.TxArray) == 0 {
			continue
		}
		txIns = append(txIns, txIn)
	}
	return txIns, nil
}

// reConfirmTx is triggered only when the chain client detects a re-org on the Zcash chain.
// It reads all block metadata from local storage and checks all TXs.
// If a TX still exists, then all is good. However, if a previously detected TX no longer exists,
// it means the transaction was removed from the chain, and the chain client should report this to mayachain.
func (c *Client) reConfirmTx() ([]int64, error) {
	blockMetas, err := c.temporalStorage.GetBlockMetas()
	if err != nil {
		return nil, fmt.Errorf("fail to get block metas from local storage: %w", err)
	}
	var rescanBlockHeights []int64
	for _, blockMeta := range blockMetas {
		var errataTxs []types.ErrataTx
		for _, tx := range blockMeta.CustomerTransactions {
			h, err := chainhash.NewHashFromStr(tx)
			if err != nil {
				c.logger.Info().Msgf("%s invalid transaction hash", tx)
				continue
			}
			if c.confirmTx(h) {
				c.logger.Info().Msgf("block height: %d, tx: %s still exist", blockMeta.Height, tx)
				continue
			}
			// this means the tx doesn't exist in chain ,thus should errata it
			errataTxs = append(errataTxs, types.ErrataTx{
				TxID:  common.TxID(tx),
				Chain: common.BTCChain,
			})
			blockMeta.RemoveCustomerTransaction(tx)
		}
		if len(errataTxs) > 0 {
			c.globalErrataQueue <- types.ErrataBlock{
				Height: blockMeta.Height,
				Txs:    errataTxs,
			}
		}
		// Let's get the block again to fix the block hash
		r, err := c.getBlock(blockMeta.Height)
		if err != nil {
			c.logger.Err(err).Msgf("fail to get block verbose tx result: %d", blockMeta.Height)
		}
		if !strings.EqualFold(blockMeta.BlockHash, r.Hash) {
			rescanBlockHeights = append(rescanBlockHeights, blockMeta.Height)
		}
		blockMeta.PreviousHash = r.PreviousHash
		blockMeta.BlockHash = r.Hash
		if err := c.temporalStorage.SaveBlockMeta(blockMeta.Height, blockMeta); err != nil {
			c.logger.Err(err).Msgf("fail to save block meta of height: %d ", blockMeta.Height)
		}
	}
	return rescanBlockHeights, nil
}

// confirmTx checks if a tx is found in the reorged blocks txs
func (c *Client) confirmTx(txHash *chainhash.Hash) bool {
	// GetRawTransaction, it should check transaction in mempool as well
	_, err := c.client.GetRawTransactionVerbose(txHash.String())
	if err == nil {
		// exist , all good
		return true
	}
	c.logger.Err(err).Msgf("fail to get tx (%s) from chain", txHash)
	// double check mempool
	_, err = c.client.GetMempoolEntry(txHash.String())
	if err != nil {
		c.logger.Err(err).Msgf("fail to get tx(%s) from mempool", txHash)
		return false
	}
	return true
}

// FetchMemPool retrieves txs from mempool
func (c *Client) FetchMemPool(height int64) (types.TxIn, error) {
	return types.TxIn{}, nil
}

// FetchTxs retrieves txs for a block height
func (c *Client) FetchTxs(height, chainHeight int64) (types.TxIn, error) {
	txIn := types.TxIn{
		Chain:   c.GetChain(),
		TxArray: nil,
	}
	c.logger.Debug().Msgf("fetch txs for block height: %d", height)
	block, err := c.getBlock(height)
	if err != nil {
		// if rpcErr, ok := err.(*btcjson.RPCError); ok && rpcErr.Code == btcjson.ErrRPCInvalidParameter {
		// 	// this means the tx had been broadcast to chain, it must be another signer finished quicker then us
		// 	return txIn, btypes.ErrUnavailableBlock
		// }
		return txIn, fmt.Errorf("fail to get block: %w", err)
	}
	blockHeight := block.Height

	// if somehow the block is not valid
	if block.Hash == "" && block.PreviousHash == "" {
		return txIn, fmt.Errorf("fail to get block: %w", err)
	}
	c.logger.Debug().Msgf("stored block height: %d", height)
	c.currentBlockHeight.Store(height)
	reScannedTxs, err := c.processReorg(block)
	if err != nil {
		c.logger.Err(err).Msg("fail to process bitcoin re-org")
	}
	if len(reScannedTxs) > 0 {
		for _, item := range reScannedTxs {
			if len(item.TxArray) == 0 {
				continue
			}
			txIn.TxArray = append(txIn.TxArray, item.TxArray...)
		}
	}

	blockMeta, err := c.temporalStorage.GetBlockMeta(blockHeight)
	if err != nil {
		return txIn, fmt.Errorf("fail to get block meta from storage: %w", err)
	}
	if blockMeta == nil {
		blockMeta = utxo.NewBlockMeta(block.PreviousHash, blockHeight, block.Hash)
	} else {
		blockMeta.PreviousHash = block.PreviousHash
		blockMeta.BlockHash = block.Hash
	}

	if err = c.temporalStorage.SaveBlockMeta(blockHeight, blockMeta); err != nil {
		return txIn, fmt.Errorf("fail to save block meta into storage: %w", err)
	}

	pruneHeight := height - c.cfg.BlockScanner.MaxReorgRescanBlocks
	if pruneHeight > 0 {
		defer func() {
			if err = c.temporalStorage.PruneBlockMeta(pruneHeight, c.canDeleteBlock); err != nil {
				c.logger.Err(err).Msgf("fail to prune block meta, height(%d)", pruneHeight)
			}
		}()
	}

	txInBlock, err := c.extractTxs(block)
	if err != nil {
		return types.TxIn{}, fmt.Errorf("fail to get txs from blocks: %w", err)
	}
	if len(txInBlock.TxArray) > 0 {
		txIn.TxArray = append(txIn.TxArray, txInBlock.TxArray...)
	}

	// report network fee and solvency if within flexibility blocks of tip
	if chainHeight-height <= c.cfg.BlockScanner.ObservationFlexibilityBlocks {
		if err := c.sendNetworkFee(height); err != nil {
			c.logger.Err(err).Msg("fail to send network fee")
		}
		if c.IsBlockScannerHealthy() {
			if err := c.ReportSolvency(height); err != nil {
				c.logger.Err(err).Msgf("fail to send solvency info to MAYAChain")
			}
		}
	}

	txIn.Count = strconv.Itoa(len(txIn.TxArray))
	if !c.consolidateInProgress.Load() {
		// try to consolidate UTXOs
		c.wg.Add(1)
		c.consolidateInProgress.Store(true)
		go c.consolidateUTXOs()
	}

	// debug log:
	for _, txInItem := range txIn.TxArray {
		c.logger.Debug().
			Int64("Height", txInItem.BlockHeight).
			Str("Memo", txInItem.Memo).
			Str("Sender", txInItem.Sender).
			Str("To", txInItem.To).
			Stringer("Coins", txInItem.Coins).
			Stringer("Gas", txInItem.Gas.ToCoins()).
			Msg("Fetched ZEC TxInItem")
	}

	return txIn, nil
}

func (c *Client) getBlock(height int64) (block *rpc.BlockWithTx, err error) {
	hash, err := c.client.GetBlockHashAsHash(height)
	if err != nil {
		return
	}
	return c.client.GetBlockVerboseTxHash(hash)
}

func (c *Client) ReportSolvency(height int64) error {
	if !c.ShouldReportSolvency(height) {
		return nil
	}
	asgardVaults, err := c.bridge.GetAsgards()
	if err != nil {
		return fmt.Errorf("fail to get asgards,err: %w", err)
	}
	for _, asgard := range asgardVaults {
		acct, err := c.GetAccount(asgard.PubKey, nil)
		if err != nil {
			c.logger.Err(err).Msgf("fail to get account balance")
			continue
		}

		if runners.IsVaultSolvent(acct, asgard, cosmos.NewUint(3*c.lastFeeRate)) && c.IsBlockScannerHealthy() {
			// when vault is solvent , don't need to report solvency
			continue
		}

		select {
		case c.globalSolvencyQueue <- types.Solvency{
			Height: height,
			Chain:  common.ZECChain,
			PubKey: asgard.PubKey,
			Coins:  acct.Coins,
		}:
			c.logger.Debug().
				Int64("height", height).
				Stringer("chain", common.ZECChain).
				Stringer("wallet amount", acct.Coins[0].Amount).
				Msg("insolvency report sent to global solvency queue")
		case <-time.After(constants.MayachainBlockTime):
			c.logger.Info().Msgf("fail to send solvency info to BASEChain, timeout")
		}
	}
	c.lastSolvencyCheckHeight = height
	return nil
}

// ShouldReportSolvency based on the given block height , should the client report solvency to THORNode
func (c *Client) ShouldReportSolvency(height int64) bool {
	return height-c.lastSolvencyCheckHeight > 10
}

func (c *Client) canDeleteBlock(blockMeta *utxo.BlockMeta) bool {
	if blockMeta == nil {
		return true
	}
	for _, tx := range blockMeta.SelfTransactions {
		if result, err := c.client.GetMempoolEntry(tx); err == nil && result != nil {
			c.logger.Info().Msgf("tx: %s still in mempool , block can't be deleted", tx)
			return false
		}
	}
	return true
}

func getUsualGas() uint64 {
	usualInCountInBlock := uint64(1)
	usualOutCountInBlock := uint64(2)
	usualMemoOutputSlots := uint64(3) // standard OUT:xxx memo (68 chars) needs 3 slots
	return zec.CalculateFee(usualInCountInBlock, usualOutCountInBlock+usualMemoOutputSlots)
}

func (c *Client) getMaxGas() uint64 {
	usualOutCountInBlock := uint64(2)
	usualMemoOutputSlots := uint64(3) // standard OUT:xxx memo (68 chars) needs 3 slots
	return zec.CalculateFee(uint64(c.getMaximumUtxosToSpend()), usualOutCountInBlock+usualMemoOutputSlots)
}

// as the feeRate is every time the same sendNetworkFee will call PostNetworkFee only once
func (c *Client) sendNetworkFee(height int64) error {
	feeRate := c.getMaxGas()
	// send network fee if changed and every 100-th block (~= every 2 hours) just to be safe
	if c.lastFeeRate != feeRate || height%100 == 0 {
		c.m.GetCounter(metrics.GasPriceChange(common.ZECChain)).Inc()
		txid, err := c.bridge.PostNetworkFee(height, common.ZECChain, 1, feeRate)
		if err != nil {
			return fmt.Errorf("fail to post network fee to thornode: %w", err)
		}
		c.lastFeeRate = feeRate
		c.logger.Debug().Str("txid", txid.String()).Msg("send network fee to THORNode successfully")
	}
	return nil
}

// isValidUTXO checks if the UTXO is valid for Zcash, considering transparent outputs only.
// It decodes the scriptPubKey hex and uses ExtractPkScriptAddrs to validate the script.
// Note:  This implementation does NOT handle shielded outputs (Sapling, Orchard).
func (c *Client) isValidUTXO(hexPubKey string) bool {
	buf, err := hex.DecodeString(hexPubKey)
	if err != nil {
		c.logger.Error().Err(err).Msgf("failed to decode hex string: %s", hexPubKey)
		return false
	}

	addresses, err := txscript.ExtractPkScriptAddrs(buf, c.getChainCfg())
	if err != nil {
		c.logger.Err(err).Msg("failed to extract pub key script")
		return false
	}

	// Valid transparent UTXOs should have exactly one address associated with them.
	return len(addresses) == 1
}

func (c *Client) getValidUTXOAddress(hexPubKey string) (bool, string) {
	buf, err := hex.DecodeString(hexPubKey)
	if err != nil {
		c.logger.Error().Err(err).Msgf("failed to decode hex string: %s", hexPubKey)
		return false, ""
	}

	addresses, err := txscript.ExtractPkScriptAddrs(buf, c.getChainCfg())
	if err != nil {
		c.logger.Err(err).Msg("failed to extract pub key script")
		return false, ""
	}

	// Valid transparent UTXOs should have exactly one address associated with them.
	if len(addresses) != 1 {
		return false, ""
	}
	return true, addresses[0]
}

// getTxIn converts VaultTx to TxInItem
func (c *Client) getTxIn(tx *rpc.TxVerbose, height int64) (txInItem types.TxInItem, err error) {
	if shouldIgnore, ignoreReason := c.ignoreTx(tx, height); shouldIgnore {
		c.logger.Debug().Int64("height", height).Str("tx", tx.Txid).Msg("ignore tx not matching format, " + ignoreReason)
		return
	}
	sender, err := c.getSender(tx)
	if err != nil {
		err = fmt.Errorf("fail to get sender from tx '%s': %w", tx.Txid, err)
		return
	}
	memo, err := c.getMemo(tx)
	if err != nil {
		err = fmt.Errorf("fail to get memo from tx: %w", err)
		return
	}
	if len([]byte(memo)) > constants.MaxMemoSize {
		err = fmt.Errorf("memo (%s) longer than max allow length(%d)", memo, constants.MaxMemoSize)
		return
	}
	m, err := mem.ParseMemo(common.LatestVersion, memo)
	if err != nil {
		c.logger.Debug().Msgf("fail to parse memo: '%s', err : %s", memo, err)
	}
	output, err := c.getOutput(sender, tx, m.IsType(mem.TxConsolidate))
	if err != nil {
		if errors.Is(err, btypes.ErrFailOutputMatchCriteria) {
			c.logger.Debug().Int64("height", height).Str("tx", tx.Hash).Msg("ignore tx not matching format")
			return types.TxInItem{}, nil
		}
		return types.TxInItem{}, fmt.Errorf("fail to get output from tx: %w", err)
	}
	addresses := c.getAddressesFromScriptPubKey(output.ScriptPubKey)
	if len(addresses) == 0 {
		return types.TxInItem{}, fmt.Errorf("fail to get addresses from script pub key")
	}
	toAddr := addresses[0]
	// If a UTXO is outbound , there is no need to validate the UTXO against mutisig
	if c.isAsgardAddress(toAddr) {
		if !c.isValidUTXO(output.ScriptPubKey.Hex) {
			return types.TxInItem{}, fmt.Errorf("invalid utxo")
		}
	}

	amount, err := zec.ZecToUint(output.Value)
	if err != nil {
		err = fmt.Errorf("fail to parse float64: %w", err)
		return
	}

	gas, err := c.getGas(tx)
	if err != nil {
		err = fmt.Errorf("fail to get gas for tx '%s': %w", tx.Txid, err)
		return
	}
	txInItem = types.TxInItem{
		BlockHeight: height,
		Tx:          tx.Txid,
		Sender:      sender,
		To:          toAddr,
		Coins: common.Coins{
			common.NewCoin(common.ZECAsset, amount),
		},
		Memo: memo,
		Gas:  gas,
	}
	return
}

// extractTxs extracts txs from a block to type TxIn
func (c *Client) extractTxs(txBlock *rpc.BlockWithTx) (types.TxIn, error) {
	txIn := types.TxIn{
		Chain:   c.GetChain(),
		MemPool: false,
	}
	var txItems []types.TxInItem
	for i := range txBlock.Tx {
		txInItem, err := c.getTxIn(&txBlock.Tx[i], txBlock.Height)
		if err != nil {
			c.logger.Err(err).Msg("fail to get TxInItem")
			continue
		}
		if txInItem.IsEmpty() {
			continue
		}
		if txInItem.Coins.IsEmpty() {
			continue
		}
		if txInItem.Coins[0].Amount.LT(cosmos.NewUint(c.chain.DustThreshold().Uint64())) {
			continue
		}
		exist, err := c.temporalStorage.TrackObservedTx(txInItem.Tx)
		if err != nil {
			c.logger.Err(err).Msgf("fail to determinate whether hash(%s) had been observed before", txInItem.Tx)
		}
		if !exist {
			c.logger.Info().Msgf("tx: %s had been report before, ignore", txInItem.Tx)
			// TODO: Zlyzol: I don't get why this untrack should be here
			// if err := c.temporalStorage.UntrackObservedTx(txInItem.Tx); err != nil {
			// 	c.logger.Err(err).Msgf("fail to remove observed tx from cache: %s", txInItem.Tx)
			// }
			continue
		}
		txItems = append(txItems, txInItem)
	}
	txIn.TxArray = txItems
	txIn.Count = strconv.Itoa(len(txItems))
	return txIn, nil
}

// ignoreTx checks if we should ignore a Zcash transaction
func (zc *Client) ignoreTx(tx *rpc.TxVerbose, height int64) (bool, string) {
	if txscript.IsTransactionShielded(tx) {
		return false, "transaction contains shielded outputs"
	}
	// Basic validation
	if len(tx.Vin) == 0 {
		return true, "0 vins"
	}
	if len(tx.Vout) == 0 {
		return true, "0 vouts"
	}

	// // Special handling for shielded transactions
	// if tx.Version >= 4 { // Sapling+
	// 	// We can't properly validate shielded components without full context
	// 	return false, ""
	// }

	// Standard transparent transaction checks
	if len(tx.Vout) > 4 {
		return true, "more than 4 vouts"
	}

	if tx.LockTime > uint32(height) {
		return true, "locktime has been set"
	}

	if tx.Vin[0].Txid == "" {
		return true, "missing txid - coinbase"
	}

	countWithOutput := 0
	for idx, vout := range tx.Vout {
		if vout.Value > 0 {
			countWithOutput++
		}

		// Standard output validation
		if idx < 2 && vout.ScriptPubKey.Type != "nulldata" && len(vout.ScriptPubKey.Addresses) != 1 {
			return true, "invalid script pub key"
		}
	}

	if countWithOutput == 0 {
		return true, "vout total is 0"
	}
	if countWithOutput > 2 {
		return true, "more than 2 vouts with value"
	}

	return false, ""
}

func (c *Client) getAddressesFromScriptPubKey(scriptPubKey rpc.ScriptPubKey) []string {
	addresses := scriptPubKey.Addresses
	if len(addresses) > 0 {
		return addresses
	}

	if len(scriptPubKey.Hex) == 0 {
		return nil
	}
	buf, err := hex.DecodeString(scriptPubKey.Hex)
	if err != nil {
		c.logger.Err(err).Msg("fail to hex decode script pub key")
		return nil
	}
	extractedAddresses, err := txscript.ExtractPkScriptAddrs(buf, c.getChainCfg())
	if err != nil {
		c.logger.Err(err).Msg("fail to extract addresses from script pub key")
		return nil

	}
	return extractedAddresses
}

// getOutput retrieves the correct output from a Zcash transaction for inbound/outbound scenarios.
//
// logic is if sender is a vault then prefer the first Vout with value,
// else prefer the first Vout with value that's to a vault
func (c *Client) getOutput(sender string, tx *rpc.TxVerbose, consolidate bool) (rpc.Vout, error) {
	if txscript.IsTransactionShielded(tx) {
		return rpc.Vout{}, ErrTxHassShieldedOuts
	}

	isSenderAsgard := c.isAsgardAddress(sender)
	for _, vout := range tx.Vout {
		if strings.EqualFold(vout.ScriptPubKey.Type, "nulldata") {
			continue
		}
		if vout.Value <= 0 {
			continue
		}
		addresses := c.getAddressesFromScriptPubKey(vout.ScriptPubKey)
		if len(addresses) != 1 {
			// If more than one address, ignore this Vout.
			// TODO check what we do if get multiple addresses
			continue
		}
		receiver := addresses[0]
		// To be observed, either the sender or receiver must be an observed THORChain vault;
		// if the sender is a vault then assume the first Vout is the output (and a later Vout could be change).
		// If the sender isn't a vault, then do do not for instance
		// return a change address Vout as the output if before the vault-inbound Vout.
		if !isSenderAsgard && !c.isAsgardAddress(receiver) {
			continue
		}

		if consolidate && receiver == sender {
			return vout, nil
		}
		if !consolidate && receiver != sender {
			return vout, nil
		}
	}
	return rpc.Vout{}, btypes.ErrFailOutputMatchCriteria
}

// getSender returns sender address for a tx, using vin:0
func (c *Client) getSender(tx *rpc.TxVerbose) (string, error) {
	if txscript.IsTransactionShielded(tx) {
		return "", ErrTxHassShieldedOuts
	}
	if len(tx.Vin) == 0 {
		return "", fmt.Errorf("no vin available in tx")
	}
	txHash, err := chainhash.NewHashFromStr(tx.Vin[0].Txid)
	if err != nil {
		return "", fmt.Errorf("fail to get tx hash from tx id string, err: %w", err)
	}
	vinTx, err := c.client.GetRawTransactionVerbose(txHash.String())
	if err != nil {
		return "", fmt.Errorf("fail to query raw tx from zcash, err: %w", err)
	}
	vout := vinTx.Vout[tx.Vin[0].Vout]
	// Convert btcjson.ScriptPubKeyResult to rpc.ScriptPubKey
	scriptPubKey := rpc.ScriptPubKey{
		Asm:       vout.ScriptPubKey.Asm,
		Hex:       vout.ScriptPubKey.Hex,
		ReqSigs:   vout.ScriptPubKey.ReqSigs,
		Type:      vout.ScriptPubKey.Type,
		Addresses: vout.ScriptPubKey.Addresses,
	}
	addresses := c.getAddressesFromScriptPubKey(scriptPubKey)
	if len(addresses) == 0 {
		return "", fmt.Errorf("no address available in vout")
	}
	return addresses[0], nil
}

// getMemo returns the memo/OP_RETURN data from a Zcash transaction.
// This implementation iterates through the Vout array, searching for
// OP_RETURN scriptPubKey types and concatenating the associated data.
func (c *Client) getMemo(tx *rpc.TxVerbose) (string, error) {
	var opreturns string
	for _, vout := range tx.Vout {
		if strings.EqualFold(vout.ScriptPubKey.Type, "nulldata") {
			opreturn := strings.Fields(vout.ScriptPubKey.Asm)
			if len(opreturn) == 2 {
				opreturns += opreturn[1]
			}
		}
	}
	decoded, err := hex.DecodeString(opreturns)
	if err != nil {
		return "", fmt.Errorf("fail to decode OP_RETURN string: %s", opreturns)
	}
	return string(decoded), nil
}

// getGas calculates the transaction fee for a Zcash transaction based on the number of inputs, outputs, and memo size.
func (c *Client) getGas(tx *rpc.TxVerbose) (common.Gas, error) {
	sumVin := cosmos.ZeroUint()
	for _, vin := range tx.Vin {
		txHash, err := chainhash.NewHashFromStr(vin.Txid)
		if err != nil {
			return common.Gas{}, fmt.Errorf("fail to get tx hash from tx id string")
		}
		vinTx, err := c.client.GetRawTransactionVerbose(txHash.String())
		if err != nil {
			return common.Gas{}, fmt.Errorf("fail to query raw tx from zcash node")
		}

		amount, err := zec.ZecToUint(vinTx.Vout[vin.Vout].Value)
		if err != nil {
			return nil, err
		}
		sumVin = sumVin.Add(amount)
	}
	sumVout := cosmos.ZeroUint()
	for _, vout := range tx.Vout {
		amount, err := zec.ZecToUint(vout.Value)
		if err != nil {
			return nil, err
		}
		sumVout = sumVout.Add(amount)
	}
	totalGas := common.SafeSub(sumVin, sumVout)
	return common.Gas{
		common.NewCoin(common.ZECAsset, totalGas),
	}, nil
}

// registerAddressInWalletAsWatch make a RPC call to import the address relevant to the given pubkey
// in wallet as watch only , so as when bifrost call ListUnspent , it will return appropriate result
func (c *Client) registerAddressInWalletAsWatch(pkey common.PubKey) error {
	return nil
}

// RegisterPublicKey register the given pubkey to zcash wallet
func (c *Client) RegisterPublicKey(pkey common.PubKey) error {
	return c.registerAddressInWalletAsWatch(pkey)
}

func (c *Client) getCoinbaseValue(blockHeight int64) (int64, error) {
	hash, err := c.client.GetBlockHashAsHash(blockHeight)
	if err != nil {
		return 0, fmt.Errorf("fail to get block hash:%w", err)
	}
	result, err := c.client.GetBlockVerboseTxHash(hash)
	if err != nil {
		return 0, fmt.Errorf("fail to get block verbose tx: %w", err)
	}
	for _, tx := range result.Tx {
		if len(tx.Vin) == 1 && tx.Vin[0].IsCoinBase() {
			total := float64(0)
			for _, opt := range tx.Vout {
				total += opt.Value
			}
			amt, err := btcutil.NewAmount(total)
			if err != nil {
				return 0, fmt.Errorf("fail to parse amount: %w", err)
			}
			return int64(amt), nil
		}
	}
	return 0, fmt.Errorf("fail to get coinbase value")
}

// getBlockRequiredConfirmation find out how many confirmation the given txIn need to have before it can be send to BASEChain
func (c *Client) getBlockRequiredConfirmation(txIn types.TxIn, height int64) (int64, error) {
	totalTxValue := txIn.GetTotalTransactionValue(common.ZECAsset, c.asgardAddresses)
	totalFeeAndSubsidy, err := c.getCoinbaseValue(height)
	if err != nil {
		c.logger.Err(err).Msg("fail to get coinbase value")
	}
	confMul, err := utxo.GetConfMulBasisPoint(c.GetChain().String(), c.bridge)
	if err != nil {
		c.logger.Err(err).Msgf("fail to get conf multiplier mimir value for %s", c.GetChain().String())
	}
	if totalFeeAndSubsidy == 0 {
		var cbValue btcutil.Amount
		cbValue, err = btcutil.NewAmount(c.cfg.ChainID.DefaultCoinbase())
		if err != nil {
			return 0, fmt.Errorf("fail to get default coinbase value: %w", err)
		}
		totalFeeAndSubsidy = int64(cbValue)
	}
	confValue := common.GetUncappedShare(confMul, cosmos.NewUint(constants.MaxBasisPts), cosmos.SafeUintFromInt64(totalFeeAndSubsidy))
	confirm := totalTxValue.Quo(confValue).Uint64()
	confirm, err = utxo.MaxConfAdjustment(confirm, c.GetChain().String(), c.bridge)
	if err != nil {
		c.logger.Err(err).Msgf("fail to get max conf value adjustment for %s", c.GetChain().String())
	}
	c.logger.Info().Msgf("totalTxValue: %s, total fee and Subsidy: %d, confirmation: %d", totalTxValue, totalFeeAndSubsidy, confirm)
	return int64(confirm), nil
}

// GetConfirmationCount return the number of blocks the tx need to wait before processing in BASEChain
func (c *Client) GetConfirmationCount(txIn types.TxIn) int64 {
	// It is hardcoded to 15 to align with major CEX
	// The current tx volume is too low to use it as a
	// measure of confirmation required
	// In the future, this may need to be revisited
	// return 15
	if len(txIn.TxArray) == 0 {
		return 0
	}
	// MemPool items doesn't need confirmation
	if txIn.MemPool {
		return 0
	}
	blockHeight := txIn.TxArray[0].BlockHeight
	confirm, err := c.getBlockRequiredConfirmation(txIn, blockHeight)
	c.logger.Info().Msgf("confirmation required: %d", confirm)
	if err != nil {
		c.logger.Err(err).Msg("fail to get block confirmation ")
		return 0
	}
	return confirm
}

// ConfirmationCountReady will be called by observer before send the txIn to mayachain
// confirmation counting is on block level , refer to https://medium.com/coinmonks/1confvalue-a-simple-pow-confirmation-rule-of-thumb-a8d9c6c483dd for detail
func (c *Client) ConfirmationCountReady(txIn types.TxIn) bool {
	if txIn.MemPool {
		c.logger.Warn().Msgf("Calling ConfirmationCountReady on Mempool tx SHOULD be avoided")
		return true
	}

	currentHeight := int(c.currentBlockHeight.Load())
	confirmed := true
	confirmationsNeeded := int(c.GetConfirmationCount(txIn))
	for _, tx := range txIn.TxArray {
		height := int(tx.BlockHeight)
		confirmations := currentHeight - height + 1
		c.logger.Info().Msgf("confirmations %d %d", confirmations, confirmationsNeeded)
		if confirmations < confirmationsNeeded {
			confirmed = false
			break
		}
	}
	return confirmed
}

// getVaultSignerLock , with consolidate UTXO process add into bifrost , there are two entry points for SignTx , one is from signer , signing the outbound tx
// from state machine, the other one will be consolidate utxo process
// this keep a lock per vault pubkey , the goal is each vault we only have one key sign in flight at a time, however different vault can do key sign in parallel
// assume there are multiple asgards(A,B) , and local yggdrasil vault , when A is signing , B and local yggdrasil vault should be able to sign as well
// however if A already has a key sign in flight , bifrost should not kick off another key sign in parallel, otherwise we might double spend some UTXOs
func (c *Client) getVaultSignerLock(vaultPubKey string) *sync.Mutex {
	c.signerLock.Lock()
	defer c.signerLock.Unlock()
	l, ok := c.vaultSignerLocks[vaultPubKey]
	if !ok {
		newLock := &sync.Mutex{}
		c.vaultSignerLocks[vaultPubKey] = newLock
		return newLock
	}
	return l
}
