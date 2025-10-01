package dash

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	dashec "gitlab.com/mayachain/dashd-go/btcec"
	"gitlab.com/mayachain/dashd-go/btcjson"
	dashutil "gitlab.com/mayachain/dashd-go/btcutil"
	"gitlab.com/mayachain/dashd-go/chaincfg/chainhash"
	"gitlab.com/mayachain/dashd-go/rpcclient"
	"gitlab.com/mayachain/dashd-go/txscript"
	"gitlab.com/mayachain/mayanode/bifrost/blockscanner"
	btypes "gitlab.com/mayachain/mayanode/bifrost/blockscanner/types"
	"gitlab.com/mayachain/mayanode/bifrost/mayaclient"
	"gitlab.com/mayachain/mayanode/bifrost/mayaclient/types"
	"gitlab.com/mayachain/mayanode/bifrost/metrics"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/shared/runners"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/shared/signercache"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/shared/utxo"
	"gitlab.com/mayachain/mayanode/bifrost/tss"
	tssp "gitlab.com/mayachain/mayanode/bifrost/tss/go-tss/tss"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/config"
	"gitlab.com/mayachain/mayanode/constants"
	mem "gitlab.com/mayachain/mayanode/x/mayachain/memo"
	"go.uber.org/atomic"
)

const (
	MaximumConfirmation   = 99999999
	MaxAsgardAddresses    = 100
	EstimateAverageTxSize = 451 // assuming 2 vins and 3 vouts (2 standard, 1 memo)
)

// Client implements the bifrost ChainClient for Dash.
//
// Dash is based on Bitcoin and is compatible with many key components of the
// Bitcoin ecosystem. The difference with Dash is the second layer of network
// participants, called masternodes, which form long-lived quorums (LLMQs)
// which facilitate instant transaction confirmation, double spend protection
// and are the foundation on which the decentralised autonomous organisation
// (DAO) self-governs.
//
// Significant differences:
//   - Transactions can be considered instant (i.e. 0 confirmations required) if
//     they are part of a block which has the Chainlock flag set to true.
//   - Block reorganisations are not possible.
//   - Replace By Fee has not been implemented.
type Client struct {
	logger                  zerolog.Logger
	cfg                     config.BifrostChainConfiguration
	m                       *metrics.Metrics
	client                  *rpcclient.Client
	chain                   common.Chain
	privateKey              *btcec.PrivateKey
	blockScanner            *blockscanner.BlockScanner
	temporalStorage         *utxo.TemporalStorage
	keySignWrapper          *KeySignWrapper
	bridge                  mayaclient.MayachainBridge
	globalErrataQueue       chan<- types.ErrataBlock
	globalSolvencyQueue     chan<- types.Solvency
	nodePubKey              common.PubKey
	currentBlockHeight      *atomic.Int64
	asgardAddresses         []common.Address
	lastAsgard              time.Time
	minRelayFeeSats         uint64
	tssKeySigner            *tss.KeySign
	lastFeeRate             uint64
	wg                      *sync.WaitGroup
	signerLock              *sync.Mutex
	vaultSignerLocks        map[string]*sync.Mutex
	consolidateInProgress   *atomic.Bool
	signerCacheManager      *signercache.CacheManager
	stopchan                chan struct{}
	lastSolvencyCheckHeight int64
}

func NewClient(
	thorKeys *mayaclient.Keys,
	cfg config.BifrostChainConfiguration,
	server *tssp.TssServer,
	bridge mayaclient.MayachainBridge,
	m *metrics.Metrics,
) (*Client, error) {
	client, err := rpcclient.New(&rpcclient.ConnConfig{
		Host:         cfg.RPCHost,
		User:         cfg.UserName,
		Pass:         cfg.Password,
		DisableTLS:   cfg.DisableTLS,
		HTTPPostMode: cfg.HTTPostMode,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("fail to create dash rpc client: %w", err)
	}

	tssKeySigner, err := tss.NewKeySign(server, bridge)
	if err != nil {
		return nil, fmt.Errorf("fail to create tss signer: %w", err)
	}

	thorPrivateKey, err := thorKeys.GetPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("fail to get THORChain private key: %w", err)
	}

	dashPrivateKey, err := getDASHPrivateKey(thorPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("fail to convert private key for DASH: %w", err)
	}

	dashPrivateKeyV2, _ := dashec.PrivKeyFromBytes(dashPrivateKey.Serialize())

	keySignWrapper, err := NewKeySignWrapper(dashPrivateKeyV2, tssKeySigner)
	if err != nil {
		return nil, fmt.Errorf("fail to create keysign wrapper: %w", err)
	}

	tendermintPubKey, err := codec.ToTmPubKeyInterface(thorPrivateKey.PubKey())
	if err != nil {
		return nil, fmt.Errorf("fail to get tm pub key: %w", err)
	}

	nodePubKey, err := common.NewPubKeyFromCrypto(tendermintPubKey)
	if err != nil {
		return nil, fmt.Errorf("fail to get the node pubkey: %w", err)
	}

	c := &Client{
		logger:                log.Logger.With().Str("module", "dash").Logger(),
		cfg:                   cfg,
		m:                     m,
		chain:                 cfg.ChainID,
		client:                client,
		privateKey:            dashPrivateKey,
		keySignWrapper:        keySignWrapper,
		bridge:                bridge,
		nodePubKey:            nodePubKey,
		minRelayFeeSats:       1000, // 1000 sats is the default minimal relay fee
		tssKeySigner:          tssKeySigner,
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
		return c, fmt.Errorf("fail to create utxo accessor: %w", err)
	}

	if err = c.registerAddressInWalletAsWatch(c.nodePubKey); err != nil {
		return nil, fmt.Errorf("fail to register (%s): %w", c.nodePubKey, err)
	}

	signerCacheManager, err := signercache.NewSignerCacheManager(storage.GetInternalDb())
	if err != nil {
		return nil, fmt.Errorf("fail to create signer cache manager,err: %w", err)
	}
	c.signerCacheManager = signerCacheManager
	c.updateNetworkInfo()

	return c, nil
}

func (c *Client) Start(globalTxsQueue chan types.TxIn, globalErrataQueue chan types.ErrataBlock, globalSolvencyQueue chan types.Solvency) {
	c.globalErrataQueue = globalErrataQueue
	c.globalSolvencyQueue = globalSolvencyQueue
	c.tssKeySigner.Start()
	c.blockScanner.Start(globalTxsQueue)
	c.wg.Add(1)
	go runners.SolvencyCheckRunner(c.GetChain(), c, c.bridge, c.stopchan, c.wg, constants.MayachainBlockTime)
}

func (c *Client) Stop() {
	c.blockScanner.Stop()
	c.tssKeySigner.Stop()
	close(c.stopchan)
	c.wg.Wait()
}

func (c *Client) GetConfig() config.BifrostChainConfiguration {
	return c.cfg
}

func (c *Client) GetChain() common.Chain {
	return common.DASHChain
}

// GetHeight returns the height of the most permanent block the node has reached so far.
// When Dash blocks are chainlocked, they are immutable and therefore completely reliable.
// We only want to recognise transactions from chainlocked blocks.
func (c *Client) GetHeight() (height int64, err error) {
	bestChainlock, err := c.client.GetBestChainlock()
	if err != nil {
		return
	}

	height = bestChainlock.Height

	return
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

func (c *Client) GetAddress(poolPubKey common.PubKey) string {
	addr, err := poolPubKey.GetAddress(common.DASHChain)
	if err != nil {
		c.logger.Error().Err(err).Str("pool_pub_key", poolPubKey.String()).Msg("fail to get pool address")
		return ""
	}
	return addr.String()
}

func (c *Client) getUTXOs(minConfirm, maximumConfirm int, pkey common.PubKey) ([]btcjson.ListUnspentResult, error) {
	dashAddress, err := pkey.GetAddress(common.DASHChain)
	if err != nil {
		return nil, fmt.Errorf("fail to get DASH Address for pubkey(%s): %w", pkey, err)
	}
	addr, err := dashutil.DecodeAddress(dashAddress.String(), c.getChainCfg())
	if err != nil {
		return nil, fmt.Errorf("fail to decode DASH address(%s): %w", dashAddress.String(), err)
	}
	return c.client.ListUnspentMinMaxAddresses(minConfirm, maximumConfirm, []dashutil.Address{addr})
}

func (c *Client) GetAccount(pkey common.PubKey, height *big.Int) (common.Account, error) {
	if height != nil {
		c.logger.Error().Msg("height was provided but will be ignored")
	}

	acct := common.Account{}
	if pkey.IsEmpty() {
		return acct, errors.New("pubkey can't be empty")
	}
	utxos, err := c.getUTXOs(0, MaximumConfirmation, pkey)
	if err != nil {
		return acct, fmt.Errorf("fail to get UTXOs: %w", err)
	}
	total := 0.0
	for _, item := range utxos {
		if !c.isValidUTXO(item.ScriptPubKey) {
			continue
		}
		if item.Confirmations == 0 {
			// pending tx that is still  in mempool, only count yggdrasil send to itself or from asgard
			if !c.isSelfTransaction(item.TxID) && !c.isAsgardAddress(item.Address) {
				continue
			}
		}
		total += item.Amount
	}
	totalAmt, err := dashutil.NewAmount(total)
	if err != nil {
		return acct, fmt.Errorf("fail to convert total amount: %w", err)
	}
	return common.NewAccount(0, 0,
		common.Coins{
			common.NewCoin(common.DASHAsset, cosmos.NewUint(uint64(totalAmt))),
		}, false), nil
}

func (c *Client) GetAccountByAddress(string, *big.Int) (common.Account, error) {
	return common.Account{}, nil
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

// FetchMemPool retrieves txs from mempool
func (c *Client) FetchMemPool(_ int64) (types.TxIn, error) {
	return types.TxIn{}, nil
}

// FetchTxs retrieves txs for a block height
func (c *Client) FetchTxs(height, chainHeight int64) (types.TxIn, error) {
	txIn := types.TxIn{
		Chain:   common.DASHChain,
		TxArray: nil,
	}
	c.logger.Debug().Msgf("fetch txs for block height: %d", height)
	block, err := c.getBlock(height)
	if err != nil {
		if rpcErr, ok := err.(*btcjson.RPCError); ok && rpcErr.Code == btcjson.ErrRPCInvalidParameter {
			// this means the tx had been broadcast to chain, it must be another signer finished quicker then us
			return txIn, btypes.ErrUnavailableBlock
		}
		return txIn, fmt.Errorf("fail to get block: %w", err)
	}

	// if somehow the block is not valid
	if block.Hash == "" && block.PreviousHash == "" {
		return txIn, fmt.Errorf("fail to get block: %w", err)
	}
	c.currentBlockHeight.Store(height)
	c.logger.Debug().Msgf("stored block height: %d", height)

	blockMeta, err := c.temporalStorage.GetBlockMeta(block.Height)
	if err != nil {
		return txIn, fmt.Errorf("fail to get block meta from storage: %w", err)
	}
	if blockMeta == nil {
		blockMeta = utxo.NewBlockMeta(block.PreviousHash, block.Height, block.Hash)
	} else {
		blockMeta.PreviousHash = block.PreviousHash
		blockMeta.BlockHash = block.Hash
	}

	if err = c.temporalStorage.SaveBlockMeta(block.Height, blockMeta); err != nil {
		return txIn, fmt.Errorf("fail to save block meta into storage: %w", err)
	}

	txInBlock, err := c.extractTxs(block)
	if err != nil {
		return types.TxIn{}, fmt.Errorf("fail to extract txIn from block: %w", err)
	}
	if len(txInBlock.TxArray) > 0 {
		txIn.TxArray = append(txIn.TxArray, txInBlock.TxArray...)
	}
	c.logger.Debug().Msgf("txIn.TxArray: %v", txIn.TxArray)
	c.updateNetworkInfo()

	// report network fee and solvency if within flexibility blocks of tip
	if chainHeight-height <= c.cfg.BlockScanner.ObservationFlexibilityBlocks {
		if err := c.sendNetworkFee(height); err != nil {
			c.logger.Err(err).Msg("fail to send network fee")
		}
		if c.IsBlockScannerHealthy() {
			if err := c.ReportSolvency(height); err != nil {
				c.logger.Err(err).Msg("fail to send solvency to MAYAChain")
			}
		}
	}

	txIn.Count = strconv.Itoa(len(txIn.TxArray))
	if !c.consolidateInProgress.Load() {
		c.wg.Add(1)
		c.consolidateInProgress.Store(true)
		go c.consolidateUTXOs()
	}
	return txIn, nil
}

func (c *Client) updateNetworkInfo() {
	networkInfo, err := c.client.GetNetworkInfo()
	if err != nil {
		c.logger.Err(err).Msg("fail to get network info")
		return
	}
	amt, err := dashutil.NewAmount(networkInfo.RelayFee)
	if err != nil {
		c.logger.Err(err).Msg("fail to get minimum relay fee")
		return
	}
	c.minRelayFeeSats = uint64(amt.ToUnit(dashutil.AmountSatoshi))
}

func (c *Client) sendNetworkFee(height int64) error {
	feeRate := EstimatedDashGasRate
	c.lastFeeRate = uint64(feeRate)
	c.m.GetGauge(metrics.GasPrice(common.DASHChain)).Set(float64(feeRate))

	txid, err := c.bridge.PostNetworkFee(height, common.DASHChain, uint64(EstimateAverageTxSize), uint64(feeRate))
	if err != nil {
		return fmt.Errorf("fail to post network fee to thornode: %w", err)
	}
	c.logger.Debug().Str("txid", txid.String()).Msg("send network fee to THORNode successfully")
	return nil
}

func (c *Client) getBlock(height int64) (block *btcjson.GetBlockVerboseTxResult, err error) {
	hash, err := c.client.GetBlockHash(height)
	if err != nil {
		return
	}
	return c.client.GetBlockVerboseTx(hash)
}

func (c *Client) isValidUTXO(hexPubKey string) bool {
	buf, err := hex.DecodeString(hexPubKey)
	if err != nil {
		c.logger.Err(err).Msgf("fail to decode hex string,%s", hexPubKey)
		return false
	}
	scriptType, addresses, requireSigs, err := txscript.ExtractPkScriptAddrs(buf, c.getChainCfg())
	if err != nil {
		c.logger.Err(err).Msg("fail to extract pub key script")
		return false
	}
	switch scriptType {
	case txscript.MultiSigTy:
		return false
	default:
		return len(addresses) == 1 && requireSigs == 1
	}
}

func (c *Client) getTxIn(tx *btcjson.TxRawResult, height int64) (txInItem types.TxInItem, err error) {
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
		c.logger.Debug().Msgf("fail to parse memo: %s,err : %s", memo, err)
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

	amount, err := dashutil.NewAmount(output.Value)
	if err != nil {
		err = fmt.Errorf("fail to parse float64: %w", err)
		return
	}
	amt := uint64(amount.ToUnit(dashutil.AmountSatoshi))

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
			common.NewCoin(common.DASHAsset, cosmos.NewUint(amt)),
		},
		Memo: memo,
		Gas:  gas,
	}
	return
}

// extractTxs extracts txs from a block to type TxIn
func (c *Client) extractTxs(block *btcjson.GetBlockVerboseTxResult) (types.TxIn, error) {
	txIn := types.TxIn{
		Chain:   c.GetChain(),
		MemPool: false,
	}
	var txItems []types.TxInItem
	for _, tx := range block.Tx {
		tx := tx
		txInItem, err := c.getTxIn(&tx, block.Height)
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
			if err := c.temporalStorage.UntrackObservedTx(txInItem.Tx); err != nil {
				c.logger.Err(err).Msgf("fail to remove observed tx from cache: %s", txInItem.Tx)
			}
			continue
		}
		txItems = append(txItems, txInItem)
	}
	txIn.TxArray = txItems
	txIn.Count = strconv.Itoa(len(txItems))
	return txIn, nil
}

// ignoreTx checks if we can already ignore a tx according to preset rules
//
// we expect array of "vout" for a DASH to have this format
// OP_RETURN is mandatory only on inbound tx
// vout:0 is our vault
// vout:1 is any any change back to themselves
// vout:2 is OP_RETURN (first 80 bytes)
// vout:3 is OP_RETURN (next 80 bytes)
//
// Rules to ignore a tx are:
// - count vouts > 4
// - count vouts with coins (value) > 2
func (c *Client) ignoreTx(tx *btcjson.TxRawResult, height int64) (shouldIngore bool, ignoreReason string) {
	if len(tx.Vin) == 0 {
		return true, "0 vins"
	}
	if len(tx.Vout) == 0 {
		return true, "0 vouts"
	}
	if len(tx.Vout) > 4 {
		return true, "more than 4 vouts"
	}

	// LockTime <= current height doesn't affect spendability,
	// and most wallets for users doing Memoless Savers deposits automatically set LockTime to the current height.
	if tx.LockTime > uint32(height) {
		return true, "locktime has been set"
	}

	if tx.Vin[0].Txid == "" {
		return true, "missing txid"
	}

	countWithOutput := 0
	for idx, vout := range tx.Vout {
		if vout.Value > 0 {
			countWithOutput++
		}
		// OPRETURN/nulldata is only valid on vout 3 and 4.
		addresses := c.getAddressesFromScriptPubKey(vout.ScriptPubKey)

		if idx < 2 && vout.ScriptPubKey.Type != "nulldata" && len(addresses) != 1 {
			return true, "null script pub key"
		}
	}
	if countWithOutput == 0 {
		return true, "vout total is 0"
	}
	if countWithOutput > 2 {
		return true, "more than 2 vouts with value"
	}
	return
}

func (c *Client) getAddressesFromScriptPubKey(scriptPubKey btcjson.ScriptPubKeyResult) []string {
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
	_, extractedAddresses, _, err := txscript.ExtractPkScriptAddrs(buf, c.getChainCfg())
	if err != nil {
		c.logger.Err(err).Msg("fail to extract addresses from script pub key")
		return nil
	}
	for _, item := range extractedAddresses {
		addresses = append(addresses, item.String())
	}
	return addresses
}

// getOutput retrieve the correct output for both inbound
// outbound tx.
// logic is if sender is a vault then prefer the first Vout with value,
// else prefer the first Vout with value that's to a vault
func (c *Client) getOutput(sender string, tx *btcjson.TxRawResult, consolidate bool) (btcjson.Vout, error) {
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
	return btcjson.Vout{}, btypes.ErrFailOutputMatchCriteria
}

// getSender returns sender address for a tx, using vin:0
func (c *Client) getSender(tx *btcjson.TxRawResult) (string, error) {
	if len(tx.Vin) == 0 {
		return "", fmt.Errorf("no vin available in tx")
	}
	txHash, err := chainhash.NewHashFromStr(tx.Vin[0].Txid)
	if err != nil {
		return "", fmt.Errorf("fail to get tx hash from tx id string,err: %w", err)
	}
	vinTx, err := c.client.GetRawTransactionVerbose(txHash)
	if err != nil {
		return "", fmt.Errorf("fail to query raw tx from dashd,err: %w", err)
	}
	vout := vinTx.Vout[tx.Vin[0].Vout]
	addresses := c.getAddressesFromScriptPubKey(vout.ScriptPubKey)
	if len(addresses) == 0 {
		return "", fmt.Errorf("no address available in vout")
	}
	return addresses[0], nil
}

// getMemo returns the memo/OP_RETURN data
func (c *Client) getMemo(tx *btcjson.TxRawResult) (string, error) {
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

// getGas returns "gas" AKA transaction fee for a dash tx (sum vin - sum vout)
func (c *Client) getGas(tx *btcjson.TxRawResult) (common.Gas, error) {
	var sumVin uint64 = 0
	for _, vin := range tx.Vin {
		txHash, err := chainhash.NewHashFromStr(vin.Txid)
		if err != nil {
			return common.Gas{}, fmt.Errorf("fail to get tx hash from tx id string")
		}
		vinTx, err := c.client.GetRawTransactionVerbose(txHash)
		if err != nil {
			return common.Gas{}, fmt.Errorf("fail to query raw tx from dash node")
		}

		amount, err := dashutil.NewAmount(vinTx.Vout[vin.Vout].Value)
		if err != nil {
			return nil, err
		}
		sumVin += uint64(amount.ToUnit(dashutil.AmountSatoshi))
	}
	var sumVout uint64 = 0
	for _, vout := range tx.Vout {
		amount, err := dashutil.NewAmount(vout.Value)
		if err != nil {
			return nil, err
		}
		sumVout += uint64(amount.ToUnit(dashutil.AmountSatoshi))
	}
	totalGas := sumVin - sumVout
	return common.Gas{
		common.NewCoin(common.DASHAsset, cosmos.NewUint(totalGas)),
	}, nil
}

// registerAddressInWalletAsWatch make a RPC call to import the address relevant to the given pubkey
// in wallet as watch only , so as when bifrost call ListUnspent , it will return appropriate result
func (c *Client) registerAddressInWalletAsWatch(pkey common.PubKey) error {
	addr, err := pkey.GetAddress(common.DASHChain)
	if err != nil {
		return fmt.Errorf("fail to get DASH address from pubkey(%s): %w", pkey, err)
	}
	err = c.createWallet("")
	if err != nil {
		return err
	}
	c.logger.Info().Msgf("import address: %s", addr.String())
	return c.client.ImportAddressRescan(addr.String(), "", false)
}

func (c *Client) createWallet(name string) error {
	walletNameJSON, err := json.Marshal(name)
	if err != nil {
		return err
	}
	falseJSON, err := json.Marshal(false)
	if err != nil {
		return err
	}

	_, err = c.client.RawRequest("createwallet", []json.RawMessage{
		walletNameJSON,
		falseJSON,
		falseJSON,
		json.RawMessage([]byte("\"\"")),
		falseJSON,
		falseJSON,
		falseJSON,
	})
	if err != nil {
		// ignore code -4 which means wallet already exists
		if strings.HasPrefix(err.Error(), "-4") {
			return nil
		}
		return err
	}
	return nil
}

// RegisterPublicKey register the given pubkey to dash wallet
func (c *Client) RegisterPublicKey(pkey common.PubKey) error {
	return c.registerAddressInWalletAsWatch(pkey)
}

func (c *Client) getCoinbaseValue(blockHeight int64) (int64, error) {
	result, err := c.getBlock(blockHeight)
	if err != nil {
		return 0, fmt.Errorf("fail to get block verbose tx: %w", err)
	}
	for _, tx := range result.Tx {
		if len(tx.Vin) == 1 && tx.Vin[0].IsCoinBase() {
			total := float64(0)
			for _, opt := range tx.Vout {
				total += opt.Value
			}
			amt, err := dashutil.NewAmount(total)
			if err != nil {
				return 0, fmt.Errorf("fail to parse amount: %w", err)
			}
			return int64(amt), nil
		}
	}
	return 0, fmt.Errorf("fail to get coinbase value")
}

// getBlockRequiredConfirmation find out how many confirmation the given txIn need to have before it can be send to THORChain
func (c *Client) getBlockRequiredConfirmation(txIn types.TxIn, height int64) (int64, error) {
	totalTxValue := txIn.GetTotalTransactionValue(common.DASHAsset, c.asgardAddresses)
	totalFeeAndSubsidy, err := c.getCoinbaseValue(height)
	if err != nil {
		c.logger.Err(err).Msgf("fail to get coinbase value")
	}
	if totalFeeAndSubsidy == 0 {
		cbValue, err := dashutil.NewAmount(c.chain.DefaultCoinbase())
		if err != nil {
			return 0, fmt.Errorf("fail to get default coinbase value: %w", err)
		}
		totalFeeAndSubsidy = int64(cbValue)
	}
	confirm := totalTxValue.QuoUint64(uint64(totalFeeAndSubsidy)).Uint64()
	c.logger.Info().Msgf("totalTxValue:%s,total fee and Subsidy:%d,confirmation:%d", totalTxValue, totalFeeAndSubsidy, confirm)
	return int64(confirm), nil
}

// GetConfirmationCount Dash blocks which have been chainlocked can be
// considered instantly confirmed, all other blocks are ignored. Therefore,
// this check doesn't apply to Dash.
func (c *Client) GetConfirmationCount(txIn types.TxIn) int64 {
	if len(txIn.TxArray) == 0 {
		return 0
	}
	blockHeight := txIn.TxArray[0].BlockHeight
	rawBlock, err := c.getBlock(blockHeight)
	if err != nil {
		c.logger.Err(err).Msg("fail to get block confirmation, dashd rpc failed")
		return 0
	}
	if rawBlock.Chainlock {
		return 0
	}
	confirm, err := c.getBlockRequiredConfirmation(txIn, blockHeight)
	c.logger.Info().Msgf("confirmation required: %d", confirm)
	if err != nil {
		c.logger.Err(err).Msg("fail to get block confirmation ")
		return 0
	}
	return confirm
}

func (c *Client) ConfirmationCountReady(_ types.TxIn) bool {
	// By this point all the txIn transactions should have been chainlocked already.
	return true
}

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

// ShouldReportSolvency based on the given block height , should the client report solvency to THORNode
func (c *Client) ShouldReportSolvency(height int64) bool {
	return height-c.lastSolvencyCheckHeight > 5
}

func (c *Client) ReportSolvency(dashBlockHeight int64) error {
	if !c.ShouldReportSolvency(dashBlockHeight) {
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
		if runners.IsVaultSolvent(acct, asgard, cosmos.NewUint(3*EstimateAverageTxSize*c.lastFeeRate)) && c.IsBlockScannerHealthy() {
			// when vault is solvent , don't need to report solvency
			continue
		}
		select {
		case c.globalSolvencyQueue <- types.Solvency{
			Height: dashBlockHeight,
			Chain:  common.DASHChain,
			PubKey: asgard.PubKey,
			Coins:  acct.Coins,
		}:
		case <-time.After(constants.MayachainBlockTime):
			c.logger.Info().Msgf("fail to send solvency info to MAYAChain, timeout")
		}
	}
	c.lastSolvencyCheckHeight = dashBlockHeight
	return nil
}
