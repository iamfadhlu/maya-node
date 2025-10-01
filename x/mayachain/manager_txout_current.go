package mayachain

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/armon/go-metrics"
	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/telemetry"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
)

// TxOutStorageVCUR is going to manage all the outgoing tx
type TxOutStorageVCUR struct {
	keeper        keeper.Keeper
	constAccessor constants.ConstantValues
	eventMgr      EventManager
	gasManager    GasManager
}

// newTxOutStorageVCUR will create a new instance of TxOutStore.
func newTxOutStorageVCUR(keeper keeper.Keeper, constAccessor constants.ConstantValues, eventMgr EventManager, gasManager GasManager) *TxOutStorageVCUR {
	return &TxOutStorageVCUR{
		keeper:        keeper,
		eventMgr:      eventMgr,
		constAccessor: constAccessor,
		gasManager:    gasManager,
	}
}

func (tos *TxOutStorageVCUR) EndBlock(ctx cosmos.Context, mgr Manager) error {
	// update the max gas for all outbounds in this block. This can be useful
	// if an outbound transaction was scheduled into the future, and the gas
	// for that blockchain changes in that time span. This avoids the need to
	// reschedule the transaction to Asgard, as well as avoids slash point
	// accural on ygg nodes.
	txOut, err := tos.GetBlockOut(ctx)
	if err != nil {
		return err
	}

	maxGasCache := make(map[common.Chain]common.Coin)
	gasRateCache := make(map[common.Chain]int64)

	for i, tx := range txOut.TxArray {
		voter, err := tos.keeper.GetObservedTxInVoter(ctx, tx.InHash)
		if err != nil {
			ctx.Logger().Error("fail to get observe tx in voter", "error", err)
			continue
		}

		// if the outbound height exists and is in the past, then no need to calculate new max gas
		if voter.OutboundHeight > 0 && voter.OutboundHeight < ctx.BlockHeight() {
			continue
		}

		// update max gas, take the larger of the current gas, or the last gas used

		// update cache if needed
		if _, ok := maxGasCache[tx.Chain]; !ok {
			maxGasCache[tx.Chain], _ = mgr.GasMgr().GetMaxGas(ctx, tx.Chain)
		}
		if _, ok := gasRateCache[tx.Chain]; !ok {
			gasRateCache[tx.Chain] = int64(mgr.GasMgr().GetGasRate(ctx, tx.Chain).Uint64())
		}

		maxGas := maxGasCache[tx.Chain]
		gasRate := gasRateCache[tx.Chain]
		if len(tx.MaxGas) == 0 || maxGas.Amount.GT(tx.MaxGas[0].Amount) {
			txOut.TxArray[i].MaxGas = common.Gas{maxGas}
			// Update MaxGas in ObservedTxVoter action as well
			err := updateTxOutGas(ctx, tos.keeper, tx, common.Gas{maxGas})
			if err != nil {
				ctx.Logger().Error("Failed to update MaxGas of action in ObservedTxVoter", "hash", tx.InHash, "error", err)
			}
		}
		// Equals checks GasRate so update actions GasRate too (before updating in the queue item)
		// for future updates of MaxGas, which must match for matchActionItem in AddOutTx.
		if err := updateTxOutGasRate(ctx, tos.keeper, tx, gasRate); err != nil {
			ctx.Logger().Error("Failed to update GasRate of action in ObservedTxVoter", "hash", tx.InHash, "error", err)
		}
		txOut.TxArray[i].GasRate = gasRate
	}

	if err := tos.keeper.SetTxOut(ctx, txOut); err != nil {
		return fmt.Errorf("fail to save tx out : %w", err)
	}
	return nil
}

// GetBlockOut read the TxOut from kv store
func (tos *TxOutStorageVCUR) GetBlockOut(ctx cosmos.Context) (*TxOut, error) {
	return tos.keeper.GetTxOut(ctx, ctx.BlockHeight())
}

// GetOutboundItems read all the outbound item from kv store
func (tos *TxOutStorageVCUR) GetOutboundItems(ctx cosmos.Context) ([]TxOutItem, error) {
	block, err := tos.keeper.GetTxOut(ctx, ctx.BlockHeight())
	if block == nil {
		return nil, nil
	}
	return block.TxArray, err
}

// GetOutboundItemByToAddress read all the outbound items filter by the given to address
func (tos *TxOutStorageVCUR) GetOutboundItemByToAddress(ctx cosmos.Context, to common.Address) []TxOutItem {
	filterItems := make([]TxOutItem, 0)
	items, _ := tos.GetOutboundItems(ctx)
	for _, item := range items {
		if item.ToAddress.Equals(to) {
			filterItems = append(filterItems, item)
		}
	}
	return filterItems
}

// ClearOutboundItems remove all the tx out items , mostly used for test
func (tos *TxOutStorageVCUR) ClearOutboundItems(ctx cosmos.Context) {
	_ = tos.keeper.ClearTxOut(ctx, ctx.BlockHeight())
}

// When TryAddTxOutItem returns an error, there should be no state changes from it,
// including funds movements or fee events from prepareTxOutItem.
// So, use CacheContext to only commit state changes when cachedTryAddTxOutItem doesn't return an error.
func (tos *TxOutStorageVCUR) TryAddTxOutItem(ctx cosmos.Context, mgr Manager, toi TxOutItem, minOut cosmos.Uint) (bool, error) {
	if toi.ToAddress.IsNoop() {
		return true, nil
	}
	// EVM outbounds to the null address should be dropped and a security event emitted
	if toi.Chain.IsEVM() && toi.ToAddress.Equals(common.EVMNullAddress) {
		ctx.Logger().Error("evm outbound to null address", "txout", toi)
		etx := common.Tx{
			ID:        toi.InHash,
			Chain:     toi.Chain,
			ToAddress: toi.ToAddress,
			Coins:     common.Coins{toi.Coin},
			Gas:       toi.MaxGas,
			Memo:      toi.Memo,
		}
		event := NewEventSecurity(etx, "evm outbound to null address")
		if err := tos.eventMgr.EmitEvent(ctx, event); err != nil {
			ctx.Logger().Error("failed to emit security event", "error", err)
		}
		return true, nil
	}

	cacheCtx, commit := ctx.CacheContext()

	// Deduct affiliate fee from outbound amount
	amount, err := tos.takeAffiliateFee(cacheCtx, mgr, toi)
	if err != nil {
		ctx.Logger().Error("fail to take affiliate fee", "error", err)
	} else if !toi.Coin.Asset.IsTradeAsset() {
		// For Trade Assets do not decrement the affiliate fee here,
		// as the affiliate fee swap will take it from the user's balance after the outbound.
		// (Since Trade Asset Withdraw is done in the MsgSwap internal handler,
		//  not the MsgDeposit external handler.)
		toi.Coin.Amount = amount
	}

	success, err := tos.cachedTryAddTxOutItem(cacheCtx, mgr, toi, minOut)
	if err == nil {
		commit()
		ctx.EventManager().EmitEvents(cacheCtx.EventManager().Events())
	}
	return success, err
}

// takeAffiliateFee - take affiliate fee from outbound amount using the inbound memo.
// should not skim fees for refunds. returns the outbound amount less the affiliate fee(s)
func (tos *TxOutStorageVCUR) takeAffiliateFee(ctx cosmos.Context, mgr Manager, toi TxOutItem) (cosmos.Uint, error) {
	// no affiliate fee for refunds or migrate txs
	if strings.Split(toi.Memo, ":")[0] == constants.MemoPrefixRefund || strings.Split(toi.Memo, ":")[0] == constants.MemoPrefixMigrate {
		return toi.Coin.Amount, nil
	}

	// Get inbound tx
	inboundVoter, err := tos.keeper.GetObservedTxInVoter(ctx, toi.InHash)
	if err != nil || inboundVoter.Tx.Tx.Memo == "" {
		return toi.Coin.Amount, fmt.Errorf("fail to get observe tx in voter: %w", err)
	}

	// if it is a preferred asset swap, no affliat fees should be taken
	if strings.HasPrefix(inboundVoter.Tx.Tx.Memo, PreferredAssetSwapMemoPrefix) {
		return toi.Coin.Amount, nil
	}

	memo, err := ParseMemoWithMAYANames(ctx, tos.keeper, inboundVoter.Tx.Tx.Memo)
	if err != nil {
		return toi.Coin.Amount, fmt.Errorf("fail to parse memo: %w", err)
	}

	// If the current outbound asset is CACAO and the original target asset is NOT CACAO, we
	// know this is the affiliate fee outbound. In this case we should skip taking an
	// additional fee. For swaps to CACAO the affiliate fee will be paid out as a direct
	// CACAO transfer with no txout manager outbound, so it won't get back to this check.
	if toi.Coin.Asset.IsNativeBase() && !memo.GetAsset().IsNativeBase() {
		return toi.Coin.Amount, nil
	}

	// Only allow outbound affiliate fees for swaps that have an affiliate fee
	if memo.IsType(TxSwap) && len(memo.GetAffiliatesBasisPoints()) > 0 {
		tx := common.Tx{
			ID:          toi.InHash,
			Chain:       toi.Chain,
			FromAddress: inboundVoter.Tx.Tx.FromAddress,
			ToAddress:   toi.ToAddress,
			Coins:       common.Coins{toi.Coin},
			Gas:         common.Gas{common.NewCoin(toi.Chain.GetGasAsset(), cosmos.NewUint(1))},
			Memo:        inboundVoter.Tx.Tx.Memo,
		}

		nodeAccounts, err := mgr.Keeper().ListActiveValidators(ctx)
		if err != nil {
			return toi.Coin.Amount, err
		}
		if len(nodeAccounts) == 0 {
			return toi.Coin.Amount, fmt.Errorf("dev err: no active node accounts")
		}
		signer := nodeAccounts[0].NodeAddress

		totalAffiliateFee, err := skimAffiliateFees(ctx, mgr, tx, signer, inboundVoter.Tx.Tx.Memo)
		if err != nil {
			ctx.Logger().Error("fail to skim affiliate fees", "error", err)
		}
		// Deduct affiliate fee from outbound amount
		toi.Coin.Amount = common.SafeSub(toi.Coin.Amount, totalAffiliateFee)
	}

	return toi.Coin.Amount, nil
}

// TryAddTxOutItem add an outbound tx to block
// return bool indicate whether the transaction had been added successful or not
// return error indicate error
func (tos *TxOutStorageVCUR) cachedTryAddTxOutItem(ctx cosmos.Context, mgr Manager, toi TxOutItem, minOut cosmos.Uint) (bool, error) {
	outputs, totalOutboundFeeCacao, err := tos.prepareTxOutItem(ctx, toi)
	if err != nil {
		return false, fmt.Errorf("fail to prepare outbound tx: %w", err)
	}
	if len(outputs) == 0 {
		return false, ErrNotEnoughToPayFee
	}

	sumOut := cosmos.ZeroUint()
	for _, o := range outputs {
		sumOut = sumOut.Add(o.Coin.Amount)
	}
	if sumOut.LT(minOut) {
		// **NOTE** this error string is utilized by the order book manager to
		// catch the error. DO NOT change this error string without updating
		// the order book manager as well
		return false, fmt.Errorf("outbound amount does not meet requirements (%d/%d)", sumOut.Uint64(), minOut.Uint64())
	}

	// blacklist binance exchange as an outbound destination. This is because
	// the format of BASEChain memos are NOT compatible with the memo
	// requirements of binance inbound transactions.
	blacklist := []string{
		"bnb136ns6lfw4zs5hg4n85vdthaad7hq5m4gtkgf23", // binance CEX address
	}
	for _, b := range blacklist {
		if toi.ToAddress.Equals(common.Address(b)) {
			return false, fmt.Errorf("non-supported outbound address")
		}
	}

	// calculate the single block height to send all of these txout items,
	// using the summed amount
	outboundHeight := ctx.BlockHeight()
	if !toi.Chain.IsBASEChain() && !toi.InHash.IsEmpty() && !toi.InHash.Equals(common.BlankTxID) {
		toi.Memo = outputs[0].Memo
		voter, err := tos.keeper.GetObservedTxInVoter(ctx, toi.InHash)
		if err != nil {
			ctx.Logger().Error("fail to get observe tx in voter", "error", err)
			return false, fmt.Errorf("fail to get observe tx in voter,err:%w", err)
		}

		targetHeight, err := tos.CalcTxOutHeight(ctx, mgr.GetVersion(), toi)
		if err != nil {
			ctx.Logger().Error("failed to calc target block height for txout item", "error", err)
		}

		// adjust delay to include streaming swap time since inbound consensus
		if voter.Height > 0 {
			targetHeight = (targetHeight - ctx.BlockHeight()) + voter.Height
		}

		if targetHeight > outboundHeight {
			outboundHeight = targetHeight
		}

		// When the inbound transaction already has an outbound , the make sure the outbound will be scheduled on the same block
		if voter.OutboundHeight > 0 {
			outboundHeight = voter.OutboundHeight
		} else {
			voter.OutboundHeight = outboundHeight
			tos.keeper.SetObservedTxInVoter(ctx, voter)
		}
	}

	// add tx to block out
	for _, output := range outputs {
		if err := tos.addToBlockOut(ctx, mgr, output, outboundHeight); err != nil {
			return false, err
		}
	}

	// Add total outbound fee to the OutboundGasWithheldRune. totalOutboundFeeCacao will be 0 if these are Migration outbounds
	// Don't count outbounds on MAYAChain ($CACAO and Synths)
	if !totalOutboundFeeCacao.IsZero() && !toi.Chain.IsBASEChain() {
		network, err := tos.keeper.GetNetwork(ctx)
		if err != nil {
			ctx.Logger().Error("fail to get network data", "error", err)
		} else {
			network.OutboundGasWithheldCacao += totalOutboundFeeCacao.Uint64()
			if err := tos.keeper.SetNetwork(ctx, network); err != nil {
				ctx.Logger().Error("fail to set network data", "error", err)
			}
		}
	}

	return true, nil
}

// UnSafeAddTxOutItem - blindly adds a tx out, skipping vault selection, transaction
// fee deduction, etc
func (tos *TxOutStorageVCUR) UnSafeAddTxOutItem(ctx cosmos.Context, mgr Manager, toi TxOutItem, height int64) error {
	if toi.ToAddress.IsNoop() {
		return nil
	}
	return tos.addToBlockOut(ctx, mgr, toi, height)
}

func (tos *TxOutStorageVCUR) discoverOutbounds(ctx cosmos.Context, transactionFeeAsset cosmos.Uint, maxGasAsset common.Coin, toi TxOutItem, vaults Vaults) ([]TxOutItem, cosmos.Uint) {
	var outputs []TxOutItem

	// When there is more than one vault, sort the vaults by
	// (as an integer) how many vaults of that size
	// would be necessary to fulfill the outbound (smallest number first).
	// Having already been sorted by security, for a given vaults-necessary
	// the lowest security ones will still be ordered first.
	// The greater a vault's vaults-necessary, the less its security would be
	// decreased by taking part in the outbound;
	// also, outbounds from negligible-amount vaults (other than wasting gas) risk creating
	// duplicate txout items of which all but one would be stuck in the outbound queue.
	// Note that for vaults of equal (integer) vaults-necessary, any previous sort order remains.
	if len(vaults) > 1 {
		type VaultsNecessary struct {
			Vault    Vault
			Estimate uint64
		}

		vaultsNecessary := make([]VaultsNecessary, 0)

		for _, vault := range vaults {
			// Avoid a divide-by-zero by ignoring vaults with zero of the asset.
			if vault.GetCoin(toi.Coin.Asset).Amount.IsZero() {
				continue
			}

			// if vault is frozen, don't send more txns to sign, as they may be
			// delayed. Once a txn is skipped here, it will not be rescheduled again.
			if len(vault.Frozen) > 0 {
				chains, err := common.NewChains(vault.Frozen)
				if err != nil {
					ctx.Logger().Error("failed to convert chains", "error", err)
				}
				if chains.Has(maxGasAsset.Asset.GetChain()) {
					continue
				}
			}

			vaultsNecessary = append(vaultsNecessary, VaultsNecessary{
				Vault:    vault,
				Estimate: toi.Coin.Amount.Quo(vault.GetCoin(toi.Coin.Asset).Amount).Uint64(),
			})
		}

		// If more than one vault remains, sort by vaults-necessary ascending.
		if len(vaultsNecessary) > 1 {
			sort.SliceStable(vaultsNecessary, func(i, j int) bool {
				return vaultsNecessary[i].Estimate < vaultsNecessary[j].Estimate
			})
		}

		// Set 'vaults' to the sorted order.
		vaults = make(Vaults, len(vaultsNecessary))
		for i, v := range vaultsNecessary {
			vaults[i] = v.Vault
		}
	}

	for _, vault := range vaults {
		// Ensure THORNode are not sending from and to the same address
		fromAddr, err := vault.GetAddress(toi.Chain)
		if err != nil || fromAddr.IsEmpty() || toi.ToAddress.Equals(fromAddr) {
			continue
		}
		// if the asset in the vault is not enough to pay for the fee , then skip it
		if vault.GetCoin(toi.Coin.Asset).Amount.LTE(transactionFeeAsset) {
			continue
		}
		// if the vault doesn't have gas asset in it , or it doesn't have enough to pay for gas
		gasAsset := vault.GetCoin(toi.Chain.GetGasAsset())
		if gasAsset.IsEmpty() || gasAsset.Amount.LT(maxGasAsset.Amount) {
			continue
		}

		toi.VaultPubKey = vault.PubKey
		if toi.Coin.Amount.LTE(vault.GetCoin(toi.Coin.Asset).Amount) {
			outputs = append(outputs, toi)
			toi.Coin.Amount = cosmos.ZeroUint()
			break
		} else {
			remainingAmount := common.SafeSub(toi.Coin.Amount, vault.GetCoin(toi.Coin.Asset).Amount)
			toi.Coin.Amount = common.SafeSub(toi.Coin.Amount, remainingAmount)
			outputs = append(outputs, toi)
			toi.Coin.Amount = remainingAmount
		}
	}
	return outputs, toi.Coin.Amount
}

// prepareTxOutItem will do some data validation which include the following
// 1. Make sure it has a legitimate memo
// 2. choose an appropriate vault(s) to send from (ygg first, active asgard, then retiring asgard)
// 3. deduct transaction fee, keep in mind, only take transaction fee when active nodes are  more then minimumBFT
// return list of outbound transactions
func (tos *TxOutStorageVCUR) prepareTxOutItem(ctx cosmos.Context, toi TxOutItem) ([]TxOutItem, cosmos.Uint, error) {
	var outputs []TxOutItem
	var remaining cosmos.Uint

	// Default the memo to the standard outbound memo
	if toi.Memo == "" {
		toi.Memo = NewOutboundMemo(toi.InHash).String()
	}

	// Ensure the InHash is set
	if toi.InHash.IsEmpty() {
		toi.InHash = common.BlankTxID
	} else {
		// fetch inbound txn memo, and append arbitrary data (if applicable)
		inboundVoter, err := tos.keeper.GetObservedTxInVoter(ctx, toi.InHash)
		if err == nil {
			parts := strings.SplitN(inboundVoter.Tx.Tx.Memo, "|", 2)
			if len(parts) == 2 {
				toi.Memo = fmt.Sprintf("%s|%s", toi.Memo, parts[1])
				if len(toi.Memo) > constants.MaxMemoSize {
					toi.Memo = toi.Memo[:constants.MaxMemoSize]
				}
			}
		}
	}
	if toi.ToAddress.IsEmpty() {
		return outputs, cosmos.ZeroUint(), fmt.Errorf("empty to address, can't send out")
	}
	if !toi.ToAddress.IsChain(toi.Chain, tos.keeper.GetVersion()) {
		return outputs, cosmos.ZeroUint(), fmt.Errorf("to address(%s), is not of chain(%s)", toi.ToAddress, toi.Chain)
	}

	// ensure amount is rounded to appropriate decimals
	toiPool, err := tos.keeper.GetPool(ctx, toi.Coin.Asset.GetLayer1Asset())
	if err != nil {
		return nil, cosmos.ZeroUint(), fmt.Errorf("fail to get pool for txout manager: %w", err)
	}

	signingTransactionPeriod := tos.constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
	transactionFeeRune := tos.gasManager.GetFee(ctx, toi.Chain, common.BaseAsset())
	transactionFeeAsset := tos.gasManager.GetFee(ctx, toi.Chain, toi.Coin.Asset)
	maxGasAsset, err := tos.gasManager.GetMaxGas(ctx, toi.Chain)
	if err != nil {
		ctx.Logger().Error("fail to get max gas asset", "error", err)
	}
	if toi.Chain.IsBASEChain() {
		outputs = append(outputs, toi)
	} else {
		if !toi.VaultPubKey.IsEmpty() {
			// a vault is already manually selected, blindly go forth with that
			outputs = append(outputs, toi)
		} else {
			// MAYANode don't have a vault already selected to send from, discover one.
			// List all pending outbounds for the asset, this will be used
			// to deduct balances of vaults that have outstanding txs assigned
			pendingOutbounds := tos.keeper.GetPendingOutbounds(ctx, toi.Coin.Asset)
			// ///////////// COLLECT YGGDRASIL VAULTS ///////////////////////////
			// When deciding which Yggdrasil pool will send out our tx out, we
			// should consider which ones observed the inbound request tx, as
			// yggdrasil pools can go offline. Here THORNode get the voter record and
			// only consider Yggdrasils where their observed saw the "correct"
			// tx.

			activeNodeAccounts, err := tos.keeper.ListActiveValidators(ctx)
			if err != nil {
				ctx.Logger().Error("fail to get all active node accounts", "error", err)
			}
			ygg := make(Vaults, 0)
			if len(activeNodeAccounts) > 0 {
				var voter ObservedTxVoter
				voter, err = tos.keeper.GetObservedTxInVoter(ctx, toi.InHash)
				if err != nil {
					return nil, cosmos.ZeroUint(), fmt.Errorf("fail to get observed tx voter: %w", err)
				}
				tx := voter.GetTx(activeNodeAccounts)

				// collect yggdrasil pools is going to get a list of yggdrasil
				// vault that BASEChain can used to send out fund
				ygg, err = tos.collectYggdrasilPools(ctx, tx, toi.Chain.GetGasAsset())
				if err != nil {
					return nil, cosmos.ZeroUint(), fmt.Errorf("fail to collect yggdrasil pool: %w", err)
				}
				for i := range ygg {
					// deduct the value of any assigned pending outbounds
					ygg[i].DeductVaultPendingOutbounds(pendingOutbounds)
				}
			}
			// All else being equal, prefer lower-security vaults for outbounds.
			yggs := tos.keeper.SortBySecurity(ctx, ygg, signingTransactionPeriod)
			// //////////////////////////////////////////////////////////////

			// ///////////// COLLECT ACTIVE ASGARD VAULTS ///////////////////
			activeAsgards, err := tos.keeper.GetAsgardVaultsByStatus(ctx, ActiveVault)
			if err != nil {
				ctx.Logger().Error("fail to get active vaults", "error", err)
			}

			// All else being equal, prefer lower-security vaults for outbounds.
			activeAsgards = tos.keeper.SortBySecurity(ctx, activeAsgards, signingTransactionPeriod)

			for i := range activeAsgards {
				// deduct the value of any assigned pending outbounds
				activeAsgards[i].DeductVaultPendingOutbounds(pendingOutbounds)
			}
			// //////////////////////////////////////////////////////////////

			// ///////////// COLLECT RETIRING ASGARD VAULTS /////////////////
			retiringAsgards, err := tos.keeper.GetAsgardVaultsByStatus(ctx, RetiringVault)
			if err != nil {
				ctx.Logger().Error("fail to get retiring vaults", "error", err)
			}

			// All else being equal, prefer lower-security vaults for outbounds.
			retiringAsgards = tos.keeper.SortBySecurity(ctx, retiringAsgards, signingTransactionPeriod)

			for i := range retiringAsgards {
				// Having, sorted by security, deduct the value of any assigned pending outbounds
				retiringAsgards[i].DeductVaultPendingOutbounds(pendingOutbounds)
			}

			// //////////////////////////////////////////////////////////////

			// iterate over discovered vaults and find vaults to send funds from

			// All else being equal, choose active Asgards over retiring Asgards.
			outputs, remaining = tos.discoverOutbounds(ctx, transactionFeeAsset, maxGasAsset, toi, append(append(yggs, activeAsgards...), retiringAsgards...))

			// Check we found enough funds to satisfy the request, error if we didn't
			if !remaining.IsZero() {
				return nil, cosmos.ZeroUint(), fmt.Errorf("insufficient funds for outbound request: %s %s remaining", toi.ToAddress.String(), remaining.String())
			}
		}
	}
	var finalOutput []TxOutItem
	var pool Pool
	var feeEvents []*EventFee
	finalRuneFee := cosmos.ZeroUint()
	for i := range outputs {
		if outputs[i].MaxGas.IsEmpty() {
			maxGasCoin, err := tos.gasManager.GetMaxGas(ctx, outputs[i].Chain)
			if err != nil {
				return nil, cosmos.ZeroUint(), fmt.Errorf("fail to get max gas coin: %w", err)
			}
			outputs[i].MaxGas = common.Gas{
				maxGasCoin,
			}
			// THOR/MAYA Chain doesn't need to have max gas
			if outputs[i].MaxGas.IsEmpty() && !outputs[i].Chain.Equals(common.BASEChain) && !outputs[i].Chain.Equals(common.THORChain) {
				return nil, cosmos.ZeroUint(), fmt.Errorf("max gas cannot be empty: %s", outputs[i].MaxGas)
			}
			outputs[i].GasRate = int64(tos.gasManager.GetGasRate(ctx, outputs[i].Chain).Uint64())
		}

		runeFee := transactionFeeRune // Fee is the prescribed fee

		// Deduct OutboundTransactionFee from TOI and add to Reserve
		memo, err := ParseMemoWithMAYANames(ctx, tos.keeper, outputs[i].Memo)
		if err == nil && !memo.IsType(TxYggdrasilFund) && !memo.IsType(TxYggdrasilReturn) && !memo.IsType(TxMigrate) && !memo.IsType(TxRagnarok) {
			if outputs[i].Coin.Asset.IsBase() {
				if outputs[i].Coin.Amount.LTE(transactionFeeRune) {
					runeFee = outputs[i].Coin.Amount // Fee is the full amount
				}
				finalRuneFee = finalRuneFee.Add(runeFee)
				outputs[i].Coin.Amount = common.SafeSub(outputs[i].Coin.Amount, runeFee)
				fee := common.NewFee(common.Coins{common.NewCoin(outputs[i].Coin.Asset, runeFee)}, cosmos.ZeroUint())
				feeEvents = append(feeEvents, NewEventFee(outputs[i].InHash, fee, cosmos.ZeroUint()))
			} else {
				if pool.IsEmpty() {
					pool, err = tos.keeper.GetPool(ctx, toi.Coin.Asset.GetLayer1Asset()) // Get pool
					if err != nil {
						// the error is already logged within kvstore
						return nil, cosmos.ZeroUint(), fmt.Errorf("fail to get pool: %w", err)
					}
				}

				// if pool units is zero, no asset fee is taken
				if !pool.GetPoolUnits().IsZero() {
					assetFee := transactionFeeAsset
					if outputs[i].Coin.Amount.LTE(assetFee) {
						assetFee = outputs[i].Coin.Amount // Fee is the full amount
					}

					outputs[i].Coin.Amount = common.SafeSub(outputs[i].Coin.Amount, assetFee) // Deduct Asset fee
					if outputs[i].Coin.Asset.IsSyntheticAsset() {
						// burn the native asset which used to pay for fee, that's only required when sending Synthetic/Derived assets from asgard
						// (not for instance applicable for Trade Assets which are not (1-to-1) Cosmos-SDK coins transferred from the Pool Module)
						if outputs[i].ModuleName == "" || outputs[i].ModuleName == AsgardName {
							if err = tos.keeper.SendFromModuleToModule(ctx,
								AsgardName,
								ModuleName,
								common.NewCoins(common.NewCoin(outputs[i].Coin.Asset, assetFee))); err != nil {
								ctx.Logger().Error("fail to move synth asset fee from asgard to Module", "error", err)
							} else if err = tos.keeper.BurnFromModule(ctx, ModuleName, common.NewCoin(outputs[i].Coin.Asset, assetFee)); err != nil {
								ctx.Logger().Error("fail to burn synth asset", "error", err)
							}
						}
					}
					if !isLiquidityAuction(ctx, tos.keeper) {
						var poolDeduct cosmos.Uint
						runeFee = pool.RuneDisbursementForAssetAdd(assetFee)
						if runeFee.GT(pool.BalanceCacao) {
							poolDeduct = pool.BalanceCacao
						} else {
							poolDeduct = runeFee
						}
						finalRuneFee = finalRuneFee.Add(poolDeduct)
						if !outputs[i].Coin.Asset.IsSyntheticAsset() {
							pool.BalanceAsset = pool.BalanceAsset.Add(assetFee) // Add Asset fee to Pool
						}
						pool.BalanceCacao = common.SafeSub(pool.BalanceCacao, poolDeduct) // Deduct Rune from Pool
						fee := common.NewFee(common.Coins{common.NewCoin(outputs[i].Coin.Asset, assetFee)}, poolDeduct)
						feeEvents = append(feeEvents, NewEventFee(outputs[i].InHash, fee, cosmos.ZeroUint()))
					}
				}
			}
		}

		vault, err := tos.keeper.GetVault(ctx, outputs[i].VaultPubKey)
		if err != nil && !outputs[i].Chain.IsBASEChain() {
			// For THORChain outputs (since having an empty VaultPubKey)
			// GetVault is expected to fail, so do not log the error.
			ctx.Logger().Error("fail to get vault", "error", err)
		}
		// when it is ragnarok , the network doesn't charge fee , however if the output asset is gas asset,
		// then the amount of max gas need to be taken away from the customer , otherwise the vault will be insolvent and doesn't
		// have enough to fulfill outbound
		// Also the MaxGas has not put back to pool ,so there is no need to subside pool when ragnarok is in progress
		// OR, if the vault is inactive, subtract maxgas from amount so we have gas to spend to refund the txn
		if (memo.IsType(TxRagnarok) || vault.Status == InactiveVault) && outputs[i].Coin.Asset.IsGasAsset() {
			gasAmt := outputs[i].MaxGas.ToCoins().GetCoin(outputs[i].Coin.Asset).Amount
			outputs[i].Coin.Amount = common.SafeSub(outputs[i].Coin.Amount, gasAmt)
		}
		// When we request Yggdrasil pool to return the fund, the coin field is actually empty
		// Signer when it sees an tx out item with memo "yggdrasil-" it will query the account on relevant chain
		// and coin field will be filled there, thus we have to let this one go
		if outputs[i].Coin.IsEmpty() && !memo.IsType(TxYggdrasilReturn) {
			ctx.Logger().Info("tx out item has zero coin", "tx_out", outputs[i].String())

			// Need to determinate whether the outbound is triggered by a withdrawal request
			// if the outbound is trigger by withdrawal request, and emit asset is not enough to pay for the fee
			// this need to return with an error , thus handler_withdraw can restore LP's LPUnits
			// and also the fee event will not be emitted
			if !outputs[i].InHash.IsEmpty() && !outputs[i].InHash.Equals(common.BlankTxID) {
				inboundVoter, err := tos.keeper.GetObservedTxInVoter(ctx, outputs[i].InHash)
				if err != nil {
					ctx.Logger().Error("fail to get observed txin voter", "error", err)
					continue
				}
				if inboundVoter.Tx.IsEmpty() {
					continue
				}
				inboundMemo, err := ParseMemoWithMAYANames(ctx, tos.keeper, inboundVoter.Tx.Tx.Memo)
				if err != nil {
					ctx.Logger().Error("fail to parse inbound transaction memo", "error", err)
					continue
				}
				if inboundMemo.IsType(TxWithdraw) {
					return nil, cosmos.ZeroUint(), errors.New("tx out item has zero coin")
				}
			}
			continue
		}

		// sanity check: ensure outbound amount respect asset decimals
		outputs[i].Coin.Amount = cosmos.RoundToDecimal(outputs[i].Coin.Amount, toiPool.Decimals)

		if !outputs[i].InHash.Equals(common.BlankTxID) {
			// increment out number of out tx for this in tx
			voter, err := tos.keeper.GetObservedTxInVoter(ctx, outputs[i].InHash)
			if err != nil {
				return nil, cosmos.ZeroUint(), fmt.Errorf("fail to get observed tx voter: %w", err)
			}
			voter.FinalisedHeight = ctx.BlockHeight()
			voter.Actions = append(voter.Actions, outputs[i])
			tos.keeper.SetObservedTxInVoter(ctx, voter)
		}

		finalOutput = append(finalOutput, outputs[i])
	}

	if !pool.IsEmpty() {
		if err := tos.keeper.SetPool(ctx, pool); err != nil { // Set Pool
			return nil, cosmos.ZeroUint(), fmt.Errorf("fail to save pool: %w", err)
		}
	}
	for _, feeEvent := range feeEvents {
		if err := tos.eventMgr.EmitFeeEvent(ctx, feeEvent); err != nil {
			ctx.Logger().Error("fail to emit fee event", "error", err)
		}
	}
	if !finalRuneFee.IsZero() {
		if toi.ModuleName == BondName {
			if err := tos.keeper.AddBondFeeToReserve(ctx, finalRuneFee); err != nil {
				ctx.Logger().Error("fail to add bond fee to reserve", "error", err)
			}
		} else {
			if err := tos.keeper.AddPoolFeeToReserve(ctx, finalRuneFee); err != nil {
				ctx.Logger().Error("fail to add pool fee to reserve", "error", err)
			}
		}
	}

	return finalOutput, finalRuneFee, nil
}

func (tos *TxOutStorageVCUR) addToBlockOut(ctx cosmos.Context, mgr Manager, item TxOutItem, outboundHeight int64) error {
	// if we're sending native assets, transfer them now and return
	if item.Chain.IsBASEChain() {
		return tos.nativeTxOut(ctx, mgr, item)
	}

	vault, err := tos.keeper.GetVault(ctx, item.VaultPubKey)
	if err != nil {
		ctx.Logger().Error("fail to get vault", "error", err)
	}
	memo, _ := ParseMemo(mgr.GetVersion(), item.Memo) // ignore err
	labels := []metrics.Label{
		telemetry.NewLabel("vault_type", vault.Type.String()),
		telemetry.NewLabel("pubkey", item.VaultPubKey.String()),
		telemetry.NewLabel("memo_type", memo.GetType().String()),
	}
	telemetry.SetGaugeWithLabels([]string{"mayanode", "vault", "out_txn"}, float32(1), labels)

	if err := tos.eventMgr.EmitEvent(ctx, NewEventScheduledOutbound(item)); err != nil {
		ctx.Logger().Error("fail to emit scheduled outbound event", "error", err)
	}

	return tos.keeper.AppendTxOut(ctx, outboundHeight, item)
}

func (tos *TxOutStorageVCUR) CalcTxOutHeight(ctx cosmos.Context, version semver.Version, toi TxOutItem) (int64, error) {
	// non-outbound transactions are skipped. This is so this code does not
	// affect internal transactions (ie consolidation and migrate txs)
	memo, _ := ParseMemo(version, toi.Memo) // ignore err
	if !memo.IsType(TxRefund) && !memo.IsType(TxOutbound) {
		return ctx.BlockHeight(), nil
	}

	minTxOutVolumeThreshold, err := tos.keeper.GetMimir(ctx, constants.MinTxOutVolumeThreshold.String())
	if minTxOutVolumeThreshold <= 0 || err != nil {
		minTxOutVolumeThreshold = tos.constAccessor.GetInt64Value(constants.MinTxOutVolumeThreshold)
	}
	minVolumeThreshold := cosmos.NewUint(uint64(minTxOutVolumeThreshold))
	txOutDelayRate, err := tos.keeper.GetMimir(ctx, constants.TxOutDelayRate.String())
	if txOutDelayRate <= 0 || err != nil {
		txOutDelayRate = tos.constAccessor.GetInt64Value(constants.TxOutDelayRate)
	}
	txOutDelayMax, err := tos.keeper.GetMimir(ctx, constants.TxOutDelayMax.String())
	if txOutDelayMax <= 0 || err != nil {
		txOutDelayMax = tos.constAccessor.GetInt64Value(constants.TxOutDelayMax)
	}
	maxTxOutOffset, err := tos.keeper.GetMimir(ctx, constants.MaxTxOutOffset.String())
	if maxTxOutOffset <= 0 || err != nil {
		maxTxOutOffset = tos.constAccessor.GetInt64Value(constants.MaxTxOutOffset)
	}

	// if volume threshold is zero
	if minVolumeThreshold.IsZero() || txOutDelayRate == 0 {
		return ctx.BlockHeight(), nil
	}

	// get txout item value in rune
	runeValue := toi.Coin.Amount
	if !toi.Coin.Asset.IsBase() {
		pool, err := tos.keeper.GetPool(ctx, toi.Coin.Asset.GetLayer1Asset())
		if err != nil {
			ctx.Logger().Error("fail to get pool for appending txout item", "error", err)
			return ctx.BlockHeight() + maxTxOutOffset, err
		}
		runeValue = pool.AssetValueInRune(toi.Coin.Amount)
	}

	// sum value of scheduled txns (including this one)
	sumValue := runeValue
	for height := ctx.BlockHeight() + 1; height <= ctx.BlockHeight()+txOutDelayMax; height++ {
		value, err := tos.keeper.GetTxOutValue(ctx, height)
		if err != nil {
			ctx.Logger().Error("fail to get tx out array from key value store", "error", err)
			continue
		}
		if height > ctx.BlockHeight()+maxTxOutOffset && value.IsZero() {
			// we've hit our max offset, and an empty block, we can assume the
			// rest will be empty as well
			break
		}
		sumValue = sumValue.Add(value)
	}
	// reduce delay rate relative to the total scheduled value. In high volume
	// scenarios, this causes the network to send outbound transactions slower,
	// giving the community & NOs time to analyze and react. In an attack
	// scenario, the attacker is likely going to move as much value as possible
	// (as we've seen in the past). The act of doing this will slow down their
	// own transaction(s), reducing the attack's effectiveness.
	txOutDelayRate -= int64(sumValue.Uint64()) / minTxOutVolumeThreshold
	if txOutDelayRate < 1 {
		txOutDelayRate = 1
	}

	// calculate the minimum number of blocks in the future the txn has to be
	minBlocks := int64(runeValue.Uint64()) / txOutDelayRate
	// min shouldn't be anything longer than the max txout offset
	if minBlocks > maxTxOutOffset {
		minBlocks = maxTxOutOffset
	}
	targetBlock := ctx.BlockHeight() + minBlocks

	// find targetBlock that has space for new txout item.
	count := int64(0)
	for count < txOutDelayMax { // max set 1 day into the future
		txOutValue, err := tos.keeper.GetTxOutValue(ctx, targetBlock)
		if err != nil {
			ctx.Logger().Error("fail to get txOutValue for block height", "error", err)
			break
		}
		if txOutValue.IsZero() {
			// the txout has no outbound txns, let's use this one
			break
		}
		if txOutValue.Add(runeValue).LTE(minVolumeThreshold) {
			// the txout + this txout item has enough space to fit, lets use this one
			break
		}
		targetBlock++
		count++
	}

	return targetBlock, nil
}

func (tos *TxOutStorageVCUR) nativeTxOut(ctx cosmos.Context, mgr Manager, toi TxOutItem) error {
	addr, err := toi.ToAddress.AccAddress()
	if err != nil {
		return err
	}

	if toi.ModuleName == "" {
		toi.ModuleName = AsgardName
	}

	// mint if we're sending from BASEChain module
	if toi.ModuleName == ModuleName {
		if err = tos.keeper.MintToModule(ctx, toi.ModuleName, toi.Coin); err != nil {
			return fmt.Errorf("fail to mint coins during txout: %w", err)
		}
	}

	polAddress, err := tos.keeper.GetModuleAddress(ReserveName)
	if err != nil {
		ctx.Logger().Error("fail to get from address", "err", err)
		return err
	}

	affColAddress, err := tos.keeper.GetModuleAddress(AffiliateCollectorName)
	if err != nil {
		ctx.Logger().Error("fail to get from address", "err", err)
		return err
	}

	// send funds to/from modules
	var sdkErr error
	switch {
	case toi.Coin.Asset.IsTradeAsset():
		// Even if trade accounts are not enabled, outbounds (as for streaming swap refunds) should complete.
		_, err = mgr.TradeAccountManager().Deposit(ctx, toi.Coin.Asset, toi.Coin.Amount, addr, common.NoAddress, toi.InHash)
		if err != nil {
			return ErrInternal(err, "fail to deposit to trade account")
		}
	case polAddress.Equals(toi.ToAddress):
		sdkErr = tos.keeper.SendFromModuleToModule(ctx, toi.ModuleName, ReserveName, common.NewCoins(toi.Coin))
	case affColAddress.Equals(toi.ToAddress):
		sdkErr = tos.keeper.SendFromModuleToModule(ctx, toi.ModuleName, AffiliateCollectorName, common.NewCoins(toi.Coin))
	default:
		sdkErr = tos.keeper.SendFromModuleToAccount(ctx, toi.ModuleName, addr, common.NewCoins(toi.Coin))
	}

	if sdkErr != nil {
		return errors.New(sdkErr.Error())
	}

	from, err := tos.keeper.GetModuleAddress(toi.ModuleName)
	if err != nil {
		ctx.Logger().Error("fail to get from address", "err", err)
		return err
	}
	outboundTxFee, err := tos.keeper.GetMimir(ctx, constants.OutboundTransactionFee.String())
	if outboundTxFee < 0 || err != nil {
		outboundTxFee = tos.constAccessor.GetInt64Value(constants.OutboundTransactionFee)
	}

	tx := common.NewTx(
		common.BlankTxID,
		from,
		toi.ToAddress,
		common.Coins{toi.Coin},
		common.Gas{common.NewCoin(common.BaseAsset(), cosmos.NewUint(uint64(outboundTxFee)))},
		toi.Memo,
	)

	active, err := tos.keeper.GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		ctx.Logger().Error("fail to get active vaults", "err", err)
		return err
	}

	if len(active) == 0 {
		return fmt.Errorf("dev error: no pubkey for native txn")
	}

	observedTx := ObservedTx{
		ObservedPubKey: active[0].PubKey,
		BlockHeight:    ctx.BlockHeight(),
		Tx:             tx,
		FinaliseHeight: ctx.BlockHeight(),
	}
	m, err := processOneTxIn(ctx, mgr.GetVersion(), tos.keeper, observedTx, tos.keeper.GetModuleAccAddress(AsgardName))
	if err != nil {
		ctx.Logger().Error("fail to process txOut", "error", err, "tx", tx.String())
		return err
	}

	handler := NewInternalHandler(mgr)

	_, err = handler(ctx, m)
	if err != nil {
		ctx.Logger().Error("TxOut Handler failed:", "error", err)
		return err
	}

	return nil
}

// collectYggdrasilPools is to get all the yggdrasil vaults , that THORChain can used to send out fund
func (tos *TxOutStorageVCUR) collectYggdrasilPools(ctx cosmos.Context, tx ObservedTx, gasAsset common.Asset) (Vaults, error) {
	// collect yggdrasil pools
	var vaults Vaults
	iterator := tos.keeper.GetVaultIterator(ctx)
	defer func() {
		if err := iterator.Close(); err != nil {
			ctx.Logger().Error("fail to close vault iterator", "error", err)
		}
	}()
	for ; iterator.Valid(); iterator.Next() {
		var vault Vault
		if err := tos.keeper.Cdc().Unmarshal(iterator.Value(), &vault); err != nil {
			return nil, fmt.Errorf("fail to unmarshal vault: %w", err)
		}
		if !vault.IsYggdrasil() {
			continue
		}
		// When trying to choose a ygg pool candidate to send out fund , let's
		// make sure the ygg pool has gasAsset , for example, if it is
		// on Binance chain , make sure ygg pool has BNB asset in it ,
		// otherwise it won't be able to pay the transaction fee
		if !vault.HasAsset(gasAsset) {
			continue
		}

		// if THORNode are already sending assets from this ygg pool, deduct them.
		addr, err := vault.PubKey.GetThorAddress()
		if err != nil {
			return nil, fmt.Errorf("fail to get thor address from pub key(%s):%w", vault.PubKey, err)
		}

		// if the ygg pool didn't observe the TxIn, and didn't sign the TxIn,
		// THORNode is not going to choose them to send out fund , because they
		// might offline
		if !tx.HasSigned(addr) {
			continue
		}

		jail, err := tos.keeper.GetNodeAccountJail(ctx, addr)
		if err != nil {
			return nil, fmt.Errorf("fail to get ygg jail:%w", err)
		}
		if jail.IsJailed(ctx) {
			continue
		}

		vaults = append(vaults, vault)
	}

	return vaults, nil
}
