package zcash

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/btcsuite/btcd/btcec"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/hashicorp/go-multierror"

	stypes "gitlab.com/mayachain/mayanode/bifrost/mayaclient/types"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/shared/utxo"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/zcash/rpc"
	"gitlab.com/mayachain/mayanode/bifrost/tss"
	"gitlab.com/mayachain/mayanode/chain/zec/go/zec"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	mem "gitlab.com/mayachain/mayanode/x/mayachain/memo"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

type (
	Sighash   []byte
	Sighashes [][]byte
)

// The constants are defined in client.go

func getZECPrivateKey(key cryptotypes.PrivKey) (*btcec.PrivateKey, error) {
	privateKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), key.Bytes())
	return privateKey, nil
}

func (c *Client) getChainCfg() *zec.NetworkParams {
	cn := common.CurrentChainNetwork
	switch cn {
	case common.MockNet, common.TestNet:
		return &zec.RegtestNetParams
	case common.MainNet, common.StageNet:
		return &zec.MainNetParams
	}
	return nil
}

// isYggdrasil - when the pubkey and node pubkey is the same that means it is signing from yggdrasil
func (c *Client) isYggdrasil(key common.PubKey) bool {
	return key.Equals(c.nodePubKey)
}

func (c *Client) getMaximumUtxosToSpend() int64 {
	const mimirMaxUTXOsToSpend = `MaxUTXOsToSpend`
	utxosToSpend, err := c.bridge.GetMimir(mimirMaxUTXOsToSpend)
	if err != nil {
		c.logger.Err(err).Msg("fail to get MaxUTXOsToSpend")
	}
	if utxosToSpend <= 0 {
		utxosToSpend = maxUTXOsToSpend
	}
	return utxosToSpend
}

// getUtxoToSpend selects UTXOs for a transaction without calculating fees
// This follows the same pattern as the Bitcoin client
func (c *Client) getUtxoToSpend(pubKey common.PubKey, amount cosmos.Uint) ([]rpc.Utxo, error) {
	maxUtxosToSpend := c.getMaximumUtxosToSpend()
	isYggdrasil := c.isYggdrasil(pubKey)
	addr, err := pubKey.GetAddress(common.ZECChain)
	if err != nil {
		return nil, fmt.Errorf("fail to get address from pubkey: %w", err)
	}
	utxos, err := c.client.GetAddressUTXOs(addr.String())
	if err != nil {
		return nil, fmt.Errorf("fail to get UTXOs: %w", err)
	}

	// sort & spend UTXO older to younger
	// The number of confirmations is (LastHeight - UTXO.Height) + 1
	lastHeight, err := c.getBlockHeight()
	if err != nil {
		return nil, fmt.Errorf("fail to get zec current block height: %w", err)
	}
	sort.SliceStable(utxos, func(i, j int) bool {
		iConfirmations := lastHeight - utxos[i].Height + 1
		jConfirmations := lastHeight - utxos[j].Height + 1
		if iConfirmations > jConfirmations {
			return true
		} else if iConfirmations < jConfirmations {
			return false
		}
		return utxos[i].Txid < utxos[j].Txid
	})

	var selectedUtxos []rpc.Utxo
	selectedUtxos = make([]rpc.Utxo, 0, len(utxos)) // pre-allocate slice capacity
	inputAmount := cosmos.ZeroUint()
	minUTXOAmt := c.chain.DustThreshold()

	for _, utxo := range utxos {
		isValidUTXO, itemAddress := c.getValidUTXOAddress(utxo.Script)
		if !isValidUTXO {
			c.logger.Info().Msgf("invalid UTXO, can't spend it")
			continue
		}

		isSelfTx, isSpentUtxo := c.isSelfTransactionAndSpentUtxo(utxo.Txid, utxo.Vout)
		if isSpentUtxo {
			continue
		}

		itemConfirmations := lastHeight - utxo.Height + 1
		// Skip unconfirmed UTXOs unless they're self-transactions or from Asgard
		if itemConfirmations == 0 {
			if !isSelfTx && !c.isAsgardAddress(itemAddress) {
				continue
			}
		}

		// Skip UTXOs below dust threshold unless self/yggdrasil
		if cosmos.NewUint(uint64(utxo.Value)).LT(minUTXOAmt) && !isSelfTx && !isYggdrasil {
			continue
		}

		// Require minimum confirmations if not self/yggdrasil
		if !isYggdrasil && itemConfirmations < MinUTXOConfirmation && !isSelfTx {
			continue
		}

		// Add the current UTXO
		selectedUtxos = append(selectedUtxos, utxo)
		inputAmount = inputAmount.AddUint64(uint64(utxo.Value))

		// in the scenario that there are too many unspent utxos available, make sure it doesn't spend too much
		// as too much UTXO will cause huge pressure on TSS, also make sure it will spend at least maxUTXOsToSpend
		// so the UTXOs will be consolidated
		if int64(len(selectedUtxos)) >= maxUtxosToSpend && inputAmount.GTE(amount) {
			break
		}
	}

	return selectedUtxos, nil
}

func (c *Client) isSelfTransactionAndSpentUtxo(txID string, voutIndex uint32) (bool, bool) {
	bms, err := c.temporalStorage.GetBlockMetas()
	if err != nil {
		c.logger.Err(err).Msg("fail to get block metas")
		return false, false
	}
	isSelf := false
	isSpent := false
	utxoKey := fmt.Sprintf("%s:%d", txID, voutIndex)
	for _, bm := range bms {
		for _, tx := range bm.SelfTransactions {
			if strings.EqualFold(tx, txID) {
				c.logger.Debug().Msgf("%s is self transaction", txID)
				isSelf = true
				break
			}
		}
		for _, spentUtxo := range bm.SpentUtxos {
			if strings.EqualFold(utxoKey, spentUtxo) {
				c.logger.Debug().Msgf("%s is already spent utxo", utxoKey)
				isSpent = true
				break
			}
		}
		if isSelf && isSpent {
			break
		}
	}
	return isSelf, isSpent
}

func (c *Client) getBlockHeight() (int64, error) {
	height, err := c.client.GetLatestHeight()
	if err != nil {
		return 0, fmt.Errorf("fail to get latest block height: %w", err)
	}
	return height, nil
}

func (c *Client) getZECPaymentAmount(tx stypes.TxOutItem) cosmos.Uint {
	amtToPay := tx.Coins.GetCoin(common.ZECAsset).Amount

	// If MaxGas is specified, add it to the amount to make sure we have enough inputs
	if !tx.MaxGas.IsEmpty() {
		gasAmt := tx.MaxGas.ToCoins().GetCoin(common.ZECAsset).Amount
		amtToPay = amtToPay.Add(gasAmt)
	}

	return amtToPay
}

// SignTx builds and signs the outbound transaction. Returns the signed transaction, a
// serialized checkpoint on error, and an error.
func (c *Client) SignTx(tx stypes.TxOutItem, mayachainHeight int64) ([]byte, []byte, *stypes.TxInItem, error) {
	if !tx.Chain.Equals(common.ZECChain) {
		return nil, nil, nil, errors.New("not ZEC chain")
	}

	// skip outbounds without coins
	if tx.Coins.IsEmpty() {
		return nil, nil, nil, nil
	}

	// skip outbounds that have been signed
	if c.signerCacheManager.HasSigned(tx.CacheHash()) {
		return nil, nil, nil, fmt.Errorf("transaction(%+v), signed before , ignore", tx)
	}

	// only one keysign per chain at a time
	vaultSignerLock := c.getVaultSignerLock(tx.VaultPubKey.String())
	if vaultSignerLock == nil {
		c.logger.Error().Msgf("fail to get signer lock for vault pub key: %s", tx.VaultPubKey.String())
		return nil, nil, nil, fmt.Errorf("fail to get signer lock")
	}
	vaultSignerLock.Lock()
	defer vaultSignerLock.Unlock()

	var ptx zec.PartialTx
	var err error
	var checkpoint []byte
	if len(tx.Checkpoint) > 0 {
		c.logger.Info().Msg("loading ZEC transaction from checkpoint")
		var initialPtx zec.PartialTx
		err = json.Unmarshal(tx.Checkpoint, &initialPtx)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to unmarshal ZEC checkpoint: %w", err)
		}
		var vaultPubKeyBytes []byte
		vaultPubKeyBytes, err = tx.VaultPubKey.Bytes()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("fail to get vault pubkey bytes: %w", err)
		}
		// re-run Rust build step to get sighashes from the checkpointed state
		ptx, err = zec.BuildPtx(vaultPubKeyBytes, initialPtx, c.getChainCfg().Net)
		if err != nil {
			return nil, tx.Checkpoint, nil, fmt.Errorf("rust build_ptx failed on checkpoint data: %w", err)
		}
		checkpoint = tx.Checkpoint // Keep the existing checkpoint data for potential further retries

	} else {
		ptx, checkpoint, err = c.buildPtx(tx)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("fail to pay from ZEC vault '%s' to address '%s', coins: %s, memo: %s, error: %w", tx.VaultPubKey, tx.ToAddress, tx.Coins, tx.Memo, err)
		}
	}

	{ // debug logs
		c.logger.Debug().Msgf("ptx to sign: Height: %d, Fee: %d", ptx.Height, ptx.Fee)
		for i, input := range ptx.Inputs {
			c.logger.Debug().Msgf("ptx Input[%d]: %v", i, input)
		}
		for i, output := range ptx.Outputs {
			c.logger.Debug().Msgf("ptx Output[%d]: %v", i, output)
		}
	}
	if len(ptx.Outputs) < 1 || len(ptx.Outputs) > 2 {
		return nil, nil, nil, fmt.Errorf("invalid count of ptx outputs (1 or 2): %d", len(ptx.Outputs))
	}

	// buildPtx returns the calculated gas
	// this check is to ensure the calculated gas doesn't exceed TxOutItem.MaxGas
	gasCoins := tx.MaxGas.ToCoins()
	if len(gasCoins) > 0 && gasCoins[0].Asset.Equals(common.ZECAsset) && gasCoins[0].Amount.BigInt().Uint64() < ptx.Fee {
		return nil, checkpoint, nil, fmt.Errorf("gas fee must not exceed TxOutItem.MaxGas: %d", ptx.Fee)
	}

	var txBytes []byte
	txBytes, err = c.signPtx(ptx, tx, mayachainHeight)
	if err != nil {
		return nil, checkpoint, nil, fmt.Errorf("fail to sign partial txs: %w", err)
	}

	{ // debug logs
		c.logger.Debug().Msgf("ptx txid: %v", hex.EncodeToString(ptx.Txid))
		for i, sigHash := range ptx.Sighashes {
			c.logger.Debug().Msgf("ptx Sighashes[%d]: %s", i, hex.EncodeToString(sigHash))
		}
		c.logger.Debug().Msgf("ptx pyload: %v", hex.EncodeToString(txBytes))
	}

	// create the observation to be sent by the signer before broadcast
	chainHeight, err := c.getBlockHeight()
	if err != nil { // fall back to the scanner height, thornode voter does not use height
		chainHeight = c.currentBlockHeight.Load()
	}
	amt := ptx.Outputs[0].Amount // the first output is the outbound amount
	gas := ptx.Fee
	var txIn *stypes.TxInItem
	sender, err := tx.VaultPubKey.GetAddress(tx.Chain)
	if err != nil {
		return txBytes, checkpoint, txIn, err
	}
	txID := hex.EncodeToString(ptx.Txid)
	txIn = stypes.NewTxInItem(
		chainHeight+1,
		txID,
		tx.Memo,
		sender.String(),
		tx.ToAddress.String(),
		common.NewCoins(
			common.NewCoin(c.chain.GetGasAsset(), cosmos.NewUint(amt)),
		),
		common.Gas(common.NewCoins(
			common.NewCoin(c.chain.GetGasAsset(), cosmos.NewUint(gas)),
		)),
		tx.VaultPubKey,
		"",
		"",
		nil,
	)

	return txBytes, checkpoint, txIn, err
}

// BroadcastTx will broadcast the given payload to ZEC chain
func (c *Client) BroadcastTx(txOut stypes.TxOutItem, payload []byte) (string, error) {
	height, err := c.getBlockHeight()
	if err != nil {
		return "", fmt.Errorf("fail to get block height: %w", err)
	}
	bm, err := c.temporalStorage.GetBlockMeta(height)
	if err != nil {
		c.logger.Err(err).Msgf("fail to get blockmeta for height: %d", height)
	}
	if bm == nil {
		bm = utxo.NewBlockMeta("", height, "")
	}
	defer func() {
		if err = c.temporalStorage.SaveBlockMeta(height, bm); err != nil {
			c.logger.Err(err).Msg("fail to save block metadata")
		}
	}()
	// broadcast tx
	txId, err := c.client.BroadcastRawTx(payload)
	if txId != "" {
		bm.AddSelfTransaction(txId)
	}
	if err != nil {
		// HHH: This should be checked. The result code for the server
		// is not guaranteed to be stable afaik
		// if rpcErr, ok := err.(*btcjson.RPCError); ok && rpcErr.Code == btcjson.ErrRPCTxAlreadyInChain {
		// 	// this means the tx had been broadcast to chain, it must be another signer finished quicker then us
		// 	// save tx id to block meta in case we need to errata later
		// 	c.logger.Info().Str("hash", redeemTx.TxHash().String()).Msg("broadcast on ZEC chain by another node")
		// 	if err = c.signerCacheManager.SetSigned(txOut.CacheHash(), txOut.CacheVault(c.GetChain()), redeemTx.TxHash().String()); err != nil {
		// 		c.logger.Err(err).Msgf("fail to mark tx out item (%+v) as signed", txOut)
		// 	}
		// 	return redeemTx.TxHash().String(), nil
		// }

		// Zlyzol: TODO: changed to TC code, ok?
		if strings.Contains(err.Error(), "already in block chain") {
			c.logger.Info().Str("hash", txId).Msg("broadcast on ZEC chain by another node")
			txId, err = c.client.GetRawTxID(payload)
			return txId, err
		}
		return "", fmt.Errorf("fail to broadcast transaction to chain: %w", err)
	}

	// commit pending spent utxos after successful broadcast
	bm.CommitPendingUtxoSpent(txId)

	// save tx id to block meta in case we need to errata later
	c.logger.Info().Str("hash", txId).Msg("broadcast to ZEC chain successfully")
	if err := c.signerCacheManager.SetSigned(txOut.CacheHash(), txOut.CacheVault(c.GetChain()), txId); err != nil {
		c.logger.Err(err).Msgf("fail to mark tx out item (%+v) as signed", txOut)
	}
	return txId, nil
}

// consolidateUTXOs only required when there is a new block
func (c *Client) consolidateUTXOs() {
	defer func() {
		c.wg.Done()
		c.consolidateInProgress.Store(false)
	}()
	var err error
	var nodeStatus types.NodeStatus
	nodeStatus, err = c.bridge.FetchNodeStatus()
	if err != nil {
		c.logger.Err(err).Msg("fail to get node status")
		return
	}
	if nodeStatus != types.NodeStatus_Active {
		c.logger.Info().Msgf("node is not active , doesn't need to consolidate utxos")
		return
	}
	memo := mem.NewConsolidateMemo().String()
	var vaults types.Vaults
	vaults, err = c.bridge.GetAsgards()
	if err != nil {
		c.logger.Err(err).Msg("fail to get current asgards")
		return
	}
	maxUtxosToSpend := c.getMaximumUtxosToSpend()
	for _, vault := range vaults {
		if !vault.Contains(c.nodePubKey) {
			// Not part of this vault , don't need to consolidate UTXOs for this Vault
			continue
		}

		// Get filtered UTXOs directly
		utxos, err := c.getUtxoToSpend(vault.PubKey, cosmos.ZeroUint())
		if err != nil {
			c.logger.Err(err).Msg("fail to get filtered utxos")
			continue
		}

		// doesn't have enough UTXOs, don't need to consolidate
		if int64(len(utxos)) < maxUtxosToSpend {
			c.logger.Debug().Msgf("no need to consolidate, utxo count: %d", len(utxos))
			continue
		}
		c.logger.Debug().Msgf("starting consolidation, utxo count: %d", len(utxos))

		// Calculate total amount from all UTXOs
		totalAmount := cosmos.ZeroUint()
		for _, item := range utxos {
			totalAmount = totalAmount.AddUint64(uint64(item.Value))
		}

		var vaultAddr common.Address
		vaultAddr, err = vault.PubKey.GetAddress(common.ZECChain)
		if err != nil {
			c.logger.Err(err).Msgf("fail to get ZEC address for pubkey:%s", vault.PubKey)
			continue
		}

		maxFeeRate := zec.CalculateFeeWithMemo(uint64(maxUtxosToSpend), 1, memo) // only 1 output in consolidate tx
		txOutItem := stypes.TxOutItem{
			Chain:       common.ZECChain,
			ToAddress:   vaultAddr,
			VaultPubKey: vault.PubKey,
			Coins: common.Coins{
				common.NewCoin(common.ZECAsset, totalAmount),
			},
			Memo:    memo,
			MaxGas:  nil,
			GasRate: int64(maxFeeRate),
		}

		height, err := c.bridge.GetBlockHeight()
		if err != nil {
			c.logger.Err(err).Msg("fail to get BASEChain block height")
			continue
		}

		var rawTx []byte
		rawTx, _, _, err = c.SignTx(txOutItem, height)
		if err != nil {
			c.logger.Err(err).Msg("fail to sign consolidate txout item")
			continue
		}

		txID, err := c.BroadcastTx(txOutItem, rawTx)
		if err != nil {
			c.logger.Err(err).Msg("fail to broadcast consolidate tx")
			continue
		}
		c.logger.Info().Msgf("broadcast consolidate tx successful, hash: %s", txID)
	}
}

func (c *Client) signPtx(ptx zec.PartialTx, tx stypes.TxOutItem, mayachainHeight int64) ([]byte, error) {
	var err error
	if len(ptx.Sighashes) == 0 {
		return nil, errors.New("no sighashes")
	}

	// sign the sighashes
	var signatures [][]byte
	signatures, err = c.signPtxParallel(ptx, tx, mayachainHeight, true) // true = parallel
	if err != nil {
		return nil, errors.New("fail to sign ZEC sighash with TSS remote signer")
	}

	// Check if signatures is nil or empty
	if signatures == nil {
		return nil, errors.New("no signatures produced")
	}

	var pubKeyByts []byte
	pubKeyByts, err = tx.VaultPubKey.Bytes()
	if err != nil {
		return nil, fmt.Errorf("fail to get vault pubkey bytes from tx pub key:%s, err: %w", tx.VaultPubKey, err)
	}
	var txBytes []byte
	txBytes, err = zec.ApplySignatures(pubKeyByts, ptx, signatures, c.getChainCfg().Net)
	if err != nil {
		return nil, fmt.Errorf("fail to apply ZEC sighash signatures: %w", err)
	}
	return txBytes, nil
}

func (c *Client) signPtxParallel(ptx zec.PartialTx, tx stypes.TxOutItem, mayachainHeight int64, parallel bool) ([][]byte, error) {
	wg := &sync.WaitGroup{}
	var utxoErr error
	// pre-allocate the signature slice with the correct size
	signatures := make([][]byte, len(ptx.Sighashes))

	// Use a mutex to protect concurrent access to signatures and utxoErr
	mu := &sync.Mutex{}

	sign := func(idx int, sigHash []byte) {
		if parallel {
			defer wg.Done()
		}
		signature, err := c.signSigHash(sigHash, tx, mayachainHeight)
		if err != nil {
			mu.Lock()
			if nil == utxoErr {
				utxoErr = err
			} else {
				utxoErr = multierror.Append(utxoErr, err)
			}
			mu.Unlock()
			// do not assign to signatures[idx] if there's an error, it will remain nil
			return
		}

		// only assign valid non-nil signatures
		if signature != nil {
			// write signature to a pre-allocated slot
			signatures[idx] = signature
		}
	}

	for idx, sigHash := range ptx.Sighashes {
		if parallel {
			wg.Add(1)
			go sign(idx, sigHash)
		} else {
			sign(idx, sigHash)
		}
	}
	if parallel {
		wg.Wait()
	}

	// if any signing goroutine reported an error
	// return the (possibly partially nil) signatures and the error
	if utxoErr != nil {
		return signatures, utxoErr
	}

	// sanity check to ensure all signatures were produced
	for i, sig := range signatures {
		if sig == nil {
			return signatures, fmt.Errorf("signature for input %d was not generated (nil), last error: %w", i, utxoErr)
		}
	}

	return signatures, nil
}

func (c *Client) signSigHash(sigHash Sighash, tx stypes.TxOutItem, mayachainHeight int64) ([]byte, error) {
	signable := c.ksWrapper.GetSignable(tx.VaultPubKey)
	if signable == nil {
		return nil, fmt.Errorf("fail to get signable for vault pubkey: %s", tx.VaultPubKey.String())
	}
	signature, err := signable.Sign(sigHash)
	if err != nil {
		var keysignError tss.KeysignError
		if errors.As(err, &keysignError) {
			if len(keysignError.Blame.BlameNodes) == 0 {
				// TSS doesn't know which node to blame
				return nil, fmt.Errorf("fail to sign sigHash: %w", err)
			}

			// key sign error forward the keysign blame to mayachain
			var txID common.TxID
			txID, err = c.bridge.PostKeysignFailure(keysignError.Blame, mayachainHeight, tx.Memo, tx.Coins, tx.VaultPubKey)
			if err != nil {
				c.logger.Error().Err(err).Msg("fail to post keysign failure to mayachain")
				return nil, fmt.Errorf("fail to post keysign failure to MAYA Chain: %w", err)
			}
			c.logger.Info().Str("tx_id", txID.String()).Msgf("post keysign failure to mayachain")
		}
		return nil, fmt.Errorf("fail to sign tx input: %w", err)
	}

	// Remove redundant error check
	if signature == nil {
		return nil, errors.New("signature is nil after signing")
	}
	return signature.Serialize(), nil
}

// builds a partial Zcash transaction for signing
func (c *Client) buildPtx(tx stypes.TxOutItem) (zec.PartialTx, []byte, error) {
	// verify output address
	err := zec.ValidateAddress(tx.ToAddress.String(), c.getChainCfg().Net)
	if err != nil {
		return zec.PartialTx{}, nil, fmt.Errorf("tx toAddress is invalid: %s, err: %w", tx.ToAddress, err)
	}

	// get vault pubk bytes
	vaultPubKeyBytes, err := tx.VaultPubKey.Bytes()
	if err != nil {
		return zec.PartialTx{}, nil, fmt.Errorf("fail to get vault pubkey bytes: %w", err)
	}

	// get expiry height
	zecHeight, err := c.getBlockHeight()
	if err != nil {
		return zec.PartialTx{}, nil, fmt.Errorf("fail to get signer ZEC last height: %w", err)
	}

	// get from address
	from, err := tx.VaultPubKey.GetAddress(common.ZECChain)
	if err != nil {
		return zec.PartialTx{}, nil, fmt.Errorf("fail to get zec address from tx vault pub key: %s, err: %w", tx.VaultPubKey, err)
	}

	getZecUtxos := func(rpcUtxos []rpc.Utxo) []zec.Utxo {
		utxos := make([]zec.Utxo, len(rpcUtxos))
		for i, utxo := range rpcUtxos {
			utxos[i] = zec.Utxo{
				Txid:   utxo.Txid,
				Height: uint32(utxo.Height),
				Value:  uint64(utxo.Value),
				Vout:   utxo.Vout,
				Script: utxo.Script,
			}
		}
		return utxos
	}

	// Get UTXOs to spend
	utxos, err := c.getUtxoToSpend(tx.VaultPubKey, c.getZECPaymentAmount(tx))
	if err != nil {
		return zec.PartialTx{}, nil, fmt.Errorf("fail to get utxos to spend: %w", err)
	}
	if len(utxos) == 0 {
		return zec.PartialTx{}, nil, errors.New("no utxo to spend found")
	}

	// Calculate total input amount
	totalInputAmount := cosmos.ZeroUint()
	for _, utxo := range utxos {
		totalInputAmount = totalInputAmount.AddUint64(uint64(utxo.Value))
	}

	// Calculate fee based on input/output count following ZIP-317
	fee := zec.CalculateFeeWithMemo(uint64(len(utxos)), 2, tx.Memo)

	// customer payment amount
	amountToCustomer := tx.Coins.GetCoin(common.ZECAsset).Amount

	// If maximum gas is specified, ensure we don't exceed it
	if !tx.MaxGas.IsEmpty() {
		maxGasCoin := tx.MaxGas.ToCoins().GetCoin(common.ZECAsset)
		// If our calculated fee exceeds max gas, cap it
		if fee > maxGasCoin.Amount.Uint64() {
			c.logger.Info().Msgf("max gas: %s, however estimated gas need %d", tx.MaxGas, fee)
			fee = maxGasCoin.Amount.Uint64()
		} else if fee < maxGasCoin.Amount.Uint64() {
			// If we use less gas than allocated, add the difference to the customer's payment
			gap := maxGasCoin.Amount.Uint64() - fee
			c.logger.Info().Msgf("max gas is: %s, however only: %d is required, gap: %d goes to customer", tx.MaxGas, fee, gap)
			amountToCustomer = amountToCustomer.Add(cosmos.NewUint(gap))
		}
	} else {
		// Handle special memos like TxYggdrasilReturn or TxConsolidate
		var memo mem.Memo
		memo, err = mem.ParseMemo(common.LatestVersion, tx.Memo)
		if err == nil && (memo.GetType() == mem.TxYggdrasilReturn || memo.GetType() == mem.TxConsolidate) {
			c.logger.Info().Msgf("yggdrasil return asset or consolidate tx, need gas: %d", fee)
			amountToCustomer = common.SafeSub(amountToCustomer, cosmos.NewUint(fee))
		}
	}

	if totalInputAmount.LT(amountToCustomer.AddUint64(fee)) {
		return zec.PartialTx{}, nil, fmt.Errorf("total utxo amount (%s) is less than out amount (%s) + gas (%d) = (%s)", totalInputAmount, amountToCustomer, fee, amountToCustomer.AddUint64(fee))
	}

	// Calculate change
	change := uint64(0)
	if totalInputAmount.GT(amountToCustomer.AddUint64(fee)) {
		change = totalInputAmount.Sub(amountToCustomer).SubUint64(fee).BigInt().Uint64()
	}

	c.logger.Info().Msgf("total inputs: %d, to customer: %d, gas: %d, change: %d",
		totalInputAmount.Uint64(), amountToCustomer, fee, change)

	// Create partial transaction
	ptx := zec.PartialTx{
		Height:       uint32(zecHeight),
		ExpiryHeight: 0, // never expires
		Inputs:       getZecUtxos(utxos),
		Fee:          fee,
		Outputs: []zec.Output{
			{
				Address: tx.ToAddress.String(),
				Amount:  amountToCustomer.BigInt().Uint64(),
				Memo:    tx.Memo,
			},
		},
	}

	// Add change output if needed
	if change > 0 {
		c.logger.Info().Msgf("send %d back to self", change)
		ptx.Outputs = append(ptx.Outputs, zec.Output{
			Address: from.String(),
			Amount:  change,
			Memo:    "",
		})
	}

	c.logger.Debug().Msgf("ZEC gas fee in ptx: %d for (inputs: %d, outputs: %d, memo len: %d)", ptx.Fee, len(ptx.Inputs), len(ptx.Outputs), len(ptx.Outputs[0].Memo))

	// serialize the initial ptx for the checkpoint
	checkpoint, err := json.Marshal(ptx)
	if err != nil {
		return zec.PartialTx{}, nil, fmt.Errorf("failed to marshal initial PartialTx for checkpoint: %w", err)
	}

	// call the Rust function to build the partial transaction
	ptx, err = zec.BuildPtx(vaultPubKeyBytes, ptx, c.getChainCfg().Net)
	if err != nil {
		return zec.PartialTx{}, checkpoint, fmt.Errorf("rust build_ptx failed: %w", err)
	}

	ptxID := hex.EncodeToString(ptx.Txid)
	c.logger.Info().Str("txid", ptxID).Int("num_sighashes", len(ptx.Sighashes)).Msg("Rust build_ptx successful")

	// sanity check sighashes were populated
	if len(ptx.Sighashes) != len(ptx.Inputs) {
		return zec.PartialTx{}, checkpoint, fmt.Errorf("number of sighashes (%d) does not match number of inputs (%d)", len(ptx.Sighashes), len(ptx.Inputs))
	}

	// save spent utxos in temporalStorage.BlockMeta
	bm, err := c.temporalStorage.GetBlockMeta(zecHeight)
	if err != nil {
		c.logger.Err(err).Msgf("fail to get blockmeta for height: %d", zecHeight)
	}
	if bm == nil {
		bm = utxo.NewBlockMeta("", zecHeight, "")
	}
	for _, utxo := range utxos {
		bm.AddPendingSpentUtxo(ptxID, utxo.Txid, utxo.Vout)
	}
	if err = c.temporalStorage.SaveBlockMeta(zecHeight, bm); err != nil {
		c.logger.Err(err).Msg("fail to save block metadata")
	}

	return ptx, checkpoint, nil
}
