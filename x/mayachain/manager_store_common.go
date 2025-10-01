// trunk-ignore-all(golangci-lint/unused): some functions may be unused currently but may be used in future
package mayachain

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

// removeTransactions is a method used to remove a tx out item in the queue
func removeTransactions(ctx cosmos.Context, mgr Manager, hashes ...string) {
	for _, txID := range hashes {
		inTxID, err := common.NewTxID(txID)
		if err != nil {
			ctx.Logger().Error("fail to parse tx id", "error", err, "tx_id", inTxID)
			continue
		}
		voter, err := mgr.Keeper().GetObservedTxInVoter(ctx, inTxID)
		if err != nil {
			ctx.Logger().Error("fail to get observed tx voter", "error", err)
			continue
		}
		// all outbound action get removed
		voter.Actions = []TxOutItem{}
		if voter.Tx.IsEmpty() {
			continue
		}
		voter.Tx.SetDone(common.BlankTxID, 0)
		// set the tx outbound with a blank txid will mark it as down , and will be skipped in the reschedule logic
		for idx := range voter.Txs {
			voter.Txs[idx].SetDone(common.BlankTxID, 0)
		}
		mgr.Keeper().SetObservedTxInVoter(ctx, voter)
	}
}

// nolint
type adhocRefundTx struct {
	inboundHash string
	toAddr      string
	amount      uint64
	asset       string
}

// refundTransactions is design to use store migration to refund adhoc transactions
// nolint
func refundTransactions(ctx cosmos.Context, mgr *Mgrs, pubKey string, adhocRefundTxes ...adhocRefundTx) {
	asgardPubKey, err := common.NewPubKey(pubKey)
	if err != nil {
		ctx.Logger().Error("fail to parse pub key", "error", err, "pubkey", pubKey)
		return
	}
	for _, item := range adhocRefundTxes {
		hash, err := common.NewTxID(item.inboundHash)
		if err != nil {
			ctx.Logger().Error("fail to parse hash", "hash", item.inboundHash, "error", err)
			continue
		}
		addr, err := common.NewAddress(item.toAddr, mgr.Keeper().GetVersion())
		if err != nil {
			ctx.Logger().Error("fail to parse address", "address", item.toAddr, "error", err)
			continue
		}
		asset, err := common.NewAsset(item.asset)
		if err != nil {
			ctx.Logger().Error("fail to parse asset", "asset", item.asset, "error", err)
			continue
		}
		coin := common.NewCoin(asset, cosmos.NewUint(item.amount))
		maxGas, err := mgr.GasMgr().GetMaxGas(ctx, coin.Asset.GetChain())
		if err != nil {
			ctx.Logger().Error("fail to get max gas", "error", err)
			continue
		}
		toi := TxOutItem{
			Chain:       coin.Asset.GetChain(),
			InHash:      hash,
			ToAddress:   addr,
			Coin:        coin,
			Memo:        NewRefundMemo(hash).String(),
			MaxGas:      common.Gas{maxGas},
			GasRate:     int64(mgr.GasMgr().GetGasRate(ctx, coin.Asset.GetChain()).Uint64()),
			VaultPubKey: asgardPubKey,
		}

		voter, err := mgr.Keeper().GetObservedTxInVoter(ctx, toi.InHash)
		if err != nil {
			ctx.Logger().Error("fail to get observe tx in voter", "error", err)
			continue
		}
		voter.OutboundHeight = ctx.BlockHeight()
		voter.Actions = append(voter.Actions, toi)
		mgr.Keeper().SetObservedTxInVoter(ctx, voter)

		if err := mgr.TxOutStore().UnSafeAddTxOutItem(ctx, mgr, toi, ctx.BlockHeight()); err != nil {
			ctx.Logger().Error("fail to send manual refund", "address", item.toAddr, "error", err)
		}
	}
}

// nolint
type adhocRefundTxV118 struct {
	inboundHash common.TxID
	toAddr      common.Address
	amount      cosmos.Uint
	asset       common.Asset
}

// refundTransactionsV118 is design to use store migration to refund adhoc transactions
// nolint
func refundTransactionsV118(ctx cosmos.Context, mgr *Mgrs, pubKey string, adhocRefundTxes ...adhocRefundTxV118) {
	asgardPubKey, err := common.NewPubKey(pubKey)
	if err != nil {
		ctx.Logger().Error("fail to parse pub key", "error", err, "pubkey", pubKey)
		return
	}
	for _, item := range adhocRefundTxes {
		coin := common.NewCoin(item.asset, item.amount)
		maxGas, err := mgr.GasMgr().GetMaxGas(ctx, coin.Asset.GetChain())
		if err != nil {
			ctx.Logger().Error("fail to get max gas", "error", err)
			continue
		}
		toi := TxOutItem{
			Chain:       coin.Asset.GetChain(),
			InHash:      item.inboundHash,
			ToAddress:   item.toAddr,
			Coin:        coin,
			Memo:        NewOutboundMemo(item.inboundHash).String(),
			MaxGas:      common.Gas{maxGas},
			GasRate:     int64(mgr.GasMgr().GetGasRate(ctx, coin.Asset.GetChain()).Uint64()),
			VaultPubKey: asgardPubKey,
		}

		voter, err := mgr.Keeper().GetObservedTxInVoter(ctx, toi.InHash)
		if err != nil {
			ctx.Logger().Error("fail to get observe tx in voter", "error", err)
			continue
		}
		voter.OutboundHeight = ctx.BlockHeight()
		voter.Actions = append(voter.Actions, toi)
		mgr.Keeper().SetObservedTxInVoter(ctx, voter)

		if err := mgr.TxOutStore().UnSafeAddTxOutItem(ctx, mgr, toi, ctx.BlockHeight()); err != nil {
			ctx.Logger().Error("fail to send manual refund", "address", item.toAddr, "error", err)
		}
	}
}

// When an ObservedTxInVoter has dangling Actions items swallowed by the vaults, requeue them.
func requeueDanglingActions(ctx cosmos.Context, mgr *Mgrs, txIDs []common.TxID) {
	// Select the least secure ActiveVault Asgard for all outbounds.
	// Even if it fails (as in if the version changed upon the keygens-complete block of a churn),
	// updating the voter's FinalisedHeight allows another MaxOutboundAttempts for LackSigning vault selection.
	activeAsgards, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil || len(activeAsgards) == 0 {
		ctx.Logger().Error("fail to get active asgard vaults", "error", err)
		return
	}
	if len(activeAsgards) > 1 {
		signingTransactionPeriod := mgr.GetConstants().GetInt64Value(constants.SigningTransactionPeriod)
		activeAsgards = mgr.Keeper().SortBySecurity(ctx, activeAsgards, signingTransactionPeriod)
	}
	vaultPubKey := activeAsgards[0].PubKey

	for _, txID := range txIDs {
		voter, err := mgr.Keeper().GetObservedTxInVoter(ctx, txID)
		if err != nil {
			ctx.Logger().Error("fail to get observed tx voter", "error", err)
			continue
		}

		if len(voter.OutTxs) >= len(voter.Actions) {
			log := fmt.Sprintf("(%d) OutTxs present for (%s), despite expecting fewer than the (%d) Actions.", len(voter.OutTxs), txID.String(), len(voter.Actions))
			ctx.Logger().Info(log)
			continue
		}

		var indices []int
		for i := range voter.Actions {
			if isActionsItemDangling(voter, i) {
				indices = append(indices, i)
			}
		}
		if len(indices) == 0 {
			log := fmt.Sprintf("No dangling Actions item found for (%s).", txID.String())
			ctx.Logger().Info(log)
			continue
		}

		if len(voter.Actions)-len(voter.OutTxs) != len(indices) {
			log := fmt.Sprintf("(%d) Actions and (%d) OutTxs present for (%s), yet there appeared to be (%d) dangling Actions.", len(voter.Actions), len(voter.OutTxs), txID.String(), len(indices))
			ctx.Logger().Debug(log)
			continue
		}

		// Update the voter's FinalisedHeight to give another MaxOutboundAttempts.
		voter.FinalisedHeight = ctx.BlockHeight()
		voter.OutboundHeight = ctx.BlockHeight()

		for _, index := range indices {
			// Use a pointer to update the voter as well.
			actionItem := &voter.Actions[index]

			// Update the vault pubkey.
			actionItem.VaultPubKey = vaultPubKey

			// Update the Actions item's MaxGas and GasRate.
			// Note that nothing in this function should require a GasManager BeginBlock.
			gasCoin, err := mgr.GasMgr().GetMaxGas(ctx, actionItem.Chain)
			if err != nil {
				ctx.Logger().Error("fail to get max gas", "chain", actionItem.Chain, "error", err)
				continue
			}
			actionItem.MaxGas = common.Gas{gasCoin}
			actionItem.GasRate = int64(mgr.GasMgr().GetGasRate(ctx, actionItem.Chain).Uint64())

			// UnSafeAddTxOutItem is used to queue the txout item directly, without for instance deducting another fee.
			err = mgr.TxOutStore().UnSafeAddTxOutItem(ctx, mgr, *actionItem, ctx.BlockHeight())
			if err != nil {
				ctx.Logger().Error("fail to add outbound tx", "error", err)
				continue
			}
		}

		// Having requeued all dangling Actions items, set the updated voter.
		mgr.Keeper().SetObservedTxInVoter(ctx, voter)
	}
}

func isActionsItemDangling(voter ObservedTxVoter, i int) bool {
	if i < 0 || i > len(voter.Actions)-1 {
		// No such Actions item exists in the voter.
		return false
	}

	toi := voter.Actions[i]

	// If any OutTxs item matches an Actions item, deem it to be not dangling.
	for _, outboundTx := range voter.OutTxs {
		// The comparison code is based on matchActionItem, as matchActionItem is unimportable.
		// note: Coins.Contains will match amount as well
		matchCoin := outboundTx.Coins.Contains(toi.Coin)
		if !matchCoin && toi.Coin.Asset.Equals(toi.Chain.GetGasAsset()) {
			asset := toi.Chain.GetGasAsset()
			intendToSpend := toi.Coin.Amount.Add(toi.MaxGas.ToCoins().GetCoin(asset).Amount)
			actualSpend := outboundTx.Coins.GetCoin(asset).Amount.Add(outboundTx.Gas.ToCoins().GetCoin(asset).Amount)
			if intendToSpend.Equal(actualSpend) {
				matchCoin = true
			}
		}
		if strings.EqualFold(toi.Memo, outboundTx.Memo) &&
			toi.ToAddress.Equals(outboundTx.ToAddress) &&
			toi.Chain.Equals(outboundTx.Chain) &&
			matchCoin {
			return false
		}
	}
	return true
}

// nolint
type unbondBondProvider struct {
	bondProviderAddress string
	nodeAccountAddress  string
}

func unbondBondProviders(ctx cosmos.Context, mgr *Mgrs, unbondBPAddresses []unbondBondProvider) {
	for _, unbondAddress := range unbondBPAddresses {
		nodeAcc, err := cosmos.AccAddressFromBech32(unbondAddress.nodeAccountAddress)
		if err != nil {
			ctx.Logger().Error("fail to parse address: %s", unbondAddress.nodeAccountAddress, "error", err)
		}

		bps, err := mgr.Keeper().GetBondProviders(ctx, nodeAcc)
		if err != nil {
			ctx.Logger().Error("fail to get bond providers(%s)", nodeAcc)
		}

		bpAcc, err := cosmos.AccAddressFromBech32(unbondAddress.bondProviderAddress)
		if err != nil {
			ctx.Logger().Error("fail to parse address: %s", unbondAddress.bondProviderAddress, "error", err)
		}

		provider := bps.Get(bpAcc)
		providerBond, err := mgr.Keeper().CalcLPLiquidityBond(ctx, common.Address(bpAcc.String()), nodeAcc)
		if err != nil {
			ctx.Logger().Error("fail to get bond provider liquidity: %s", err)
		}
		if !provider.IsEmpty() && providerBond.IsZero() {
			bps.Unbond(bpAcc)
			if ok := bps.Remove(bpAcc); ok {
				if err := mgr.Keeper().SetBondProviders(ctx, bps); err != nil {
					ctx.Logger().Error("fail to save bond providers(%s)", bpAcc, "error", err)
				}
			}
		}

	}
}

type RefundTxCACAO struct {
	sendAddress string
	amount      cosmos.Uint
}

func refundTxsCACAO(ctx cosmos.Context, mgr *Mgrs, refunds []RefundTxCACAO) {
	for _, refund := range refunds {
		if refund.amount.IsZero() {
			continue
		}

		acc, err := cosmos.AccAddressFromBech32(refund.sendAddress)
		if err != nil {
			ctx.Logger().Error("fail to parse address: %s", refund.sendAddress, "error", err)
			continue
		}

		if err := mgr.Keeper().SendFromModuleToAccount(ctx, ReserveName, acc, common.NewCoins(common.NewCoin(common.BaseNative, refund.amount))); err != nil {
			ctx.Logger().Error("fail to send provider reward: %s", refund.sendAddress, "error", err)
		}
	}
}

type LiquidityProvidersSlash struct {
	Asset             common.Asset
	BondAddressString string
	LPAddressString   string
	LPUnits           cosmos.Uint
}

type ReserveSlash struct {
	Asset common.Asset
	Units cosmos.Uint
}

func refundLPSlashes(ctx cosmos.Context, mgr *Mgrs, lpSlashes []LiquidityProvidersSlash, reserveAddressString string) error {
	var reserveSlash []ReserveSlash
	for _, lpSlash := range lpSlashes {
		bondAddress, err := cosmos.AccAddressFromBech32(lpSlash.BondAddressString)
		if err != nil {
			ctx.Logger().Error("failed to get address", "error", err)
			return err
		}

		lpAddress, err := common.NewAddress(lpSlash.LPAddressString, GetCurrentVersion())
		if err != nil {
			ctx.Logger().Error("failed to get address", "error", err)
			return err
		}

		lp, err := mgr.Keeper().GetLiquidityProvider(ctx, lpSlash.Asset, lpAddress)
		if err != nil {
			ctx.Logger().Error("failed to get liquidity provider", "error", err)
			return err
		}

		// Move units and bonded nodes back to LP
		lp.Units = lpSlash.LPUnits
		lp.BondedNodes = []types.LPBondedNode{
			{
				NodeAddress: bondAddress,
				Units:       lpSlash.LPUnits,
			},
		}

		mgr.Keeper().SetLiquidityProvider(ctx, lp)
		hasAsset := false
		for i, reserve := range reserveSlash {
			if reserve.Asset == lpSlash.Asset {
				reserveSlash[i].Units = reserveSlash[i].Units.Add(lpSlash.LPUnits)
				hasAsset = true
				break
			}
		}

		if !hasAsset {
			reserveSlash = append(reserveSlash, ReserveSlash{
				Asset: lpSlash.Asset,
				Units: lpSlash.LPUnits,
			})
		}
	}

	// LP units went to Reserve, after moving units back, remove from there
	for _, lpReserve := range reserveSlash {
		reserveAddress, err := common.NewAddress(reserveAddressString, GetCurrentVersion())
		if err != nil {
			ctx.Logger().Error("failed to get address", "error", err)
			return err
		}
		reserveLP, err := mgr.Keeper().GetLiquidityProvider(ctx, lpReserve.Asset, reserveAddress)
		if err != nil {
			ctx.Logger().Error("failed to get liquidity provider", "error", err)
			return err
		}

		reserveLP.Units = reserveLP.Units.Sub(lpReserve.Units)
		mgr.Keeper().SetLiquidityProvider(ctx, reserveLP)
	}

	return nil
}

// makeFakeTxInObservation - accepts an array of unobserved inbounds, queries for active node accounts, and makes
// a fake observation for each validator and unobserved TxIn. Once enough nodes have "observed" each inbound the tx will be
// processed as normal.
func makeFakeTxInObservation(ctx cosmos.Context, mgr *Mgrs, txs ObservedTxs) error {
	activeNodes, err := mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		ctx.Logger().Error("Failed to get active nodes", "err", err)
		return err
	}

	handler := NewObservedTxInHandler(mgr)

	for _, na := range activeNodes {
		txInMsg := NewMsgObservedTxIn(txs, na.NodeAddress)
		_, err := handler.handle(ctx, *txInMsg)
		if err != nil {
			ctx.Logger().Error("failed ObservedTxIn handler", "error", err)
			continue
		}
	}

	return nil
}

type DroppedSwapOutTx struct {
	inboundHash string
	gasAsset    common.Asset
}

// refundDroppedSwapOutFromCACAO refunds a dropped swap out TX that originated from $CACAO

// These txs completed the swap to the EVM gas asset, but bifrost dropped the final swap out outbound
// To refund:
// 1. Credit the gas asset pool the amount of gas asset that never left
// 2. Deduct the corresponding amount of CACAO from the pool, as that will be refunded
// 3. Send the user their CACAO back
//
//lint:ignore ST1005 We intentionally capitalize this error message
func refundDroppedSwapOutFromCACAO(ctx cosmos.Context, mgr *Mgrs, droppedTx DroppedSwapOutTx) error {
	txId, err := common.NewTxID(droppedTx.inboundHash)
	if err != nil {
		return err
	}

	txVoter, err := mgr.Keeper().GetObservedTxInVoter(ctx, txId)
	if err != nil {
		return err
	}

	if txVoter.OutTxs != nil {
		return fmt.Errorf("For a dropped swap out there should be no out_txs")
	}

	// Get the original inbound, if it's not for CACAO, skip
	inboundTx := txVoter.Tx.Tx
	if !inboundTx.Chain.IsBASEChain() {
		return fmt.Errorf("Inbound tx isn't from mayachain")
	}

	inboundCoins := inboundTx.Coins
	if len(inboundCoins) != 1 || !inboundCoins[0].Asset.IsNativeBase() {
		return fmt.Errorf("Inbound coin is not native CACAO")
	}

	inboundCACAO := inboundCoins[0]
	swapperCACAOAddr := inboundTx.FromAddress

	if len(txVoter.Actions) == 0 {
		return fmt.Errorf("Tx Voter has empty Actions")
	}

	// gasAssetCoin is the gas asset that was swapped to for the swap out
	// Since the swap out was dropped, this amount of the gas asset never left the pool.
	// This amount should be credited back to the pool since it was originally deducted when mayanode sent the swap out
	gasAssetCoin := txVoter.Actions[0].Coin
	if !gasAssetCoin.Asset.Equals(droppedTx.gasAsset) {
		return fmt.Errorf("Tx Voter action coin isn't swap out gas asset")
	}

	gasPool, err := mgr.Keeper().GetPool(ctx, droppedTx.gasAsset)
	if err != nil {
		return err
	}

	totalGasAssetAmt := cosmos.NewUint(0)

	// If the outbound was split between multiple Asgards, add up the full amount here
	for _, action := range txVoter.Actions {
		totalGasAssetAmt = totalGasAssetAmt.Add(action.Coin.Amount)
	}

	// Credit Gas Pool the Gas Asset balance, deduct the CACAO balance
	gasPool.BalanceAsset = gasPool.BalanceAsset.Add(totalGasAssetAmt)
	gasPool.BalanceCacao = gasPool.BalanceCacao.Sub(inboundCACAO.Amount)

	// Update the pool
	if err = mgr.Keeper().SetPool(ctx, gasPool); err != nil {
		return err
	}

	addrAcct, err := swapperCACAOAddr.AccAddress()
	if err != nil {
		ctx.Logger().Error("fail to create acct in migrate store to v98", "error", err)
	}

	cacaoCoins := common.NewCoins(inboundCACAO)

	// Send user their funds
	err = mgr.Keeper().SendFromModuleToAccount(ctx, AsgardName, addrAcct, cacaoCoins)
	if err != nil {
		return err
	}

	memo := fmt.Sprintf("REFUND:%s", inboundTx.ID)

	// Generate a fake TxID from the refund memo for Midgard to record.
	// Since the inbound hash is expected to be unique, the sha256 hash is expected to be unique.
	hash := fmt.Sprintf("%X", sha256.Sum256([]byte(memo)))
	fakeTxID, err := common.NewTxID(hash)
	if err != nil {
		return err
	}

	// create and emit a fake tx and swap event to keep pools balanced in Midgard
	fakeSwapTx := common.Tx{
		ID:          fakeTxID,
		Chain:       common.ETHChain,
		FromAddress: txVoter.Actions[0].ToAddress,
		ToAddress:   common.Address(txVoter.Actions[0].Aggregator),
		Coins:       common.NewCoins(gasAssetCoin),
		Memo:        memo,
	}

	swapEvt := NewEventSwap(
		droppedTx.gasAsset,
		cosmos.ZeroUint(),
		cosmos.ZeroUint(),
		cosmos.ZeroUint(),
		cosmos.ZeroUint(),
		fakeSwapTx,
		inboundCACAO,
		cosmos.ZeroUint(),
	)

	if err := mgr.EventMgr().EmitEvent(ctx, swapEvt); err != nil {
		ctx.Logger().Error("fail to emit fake swap event", "error", err)
	}

	return nil
}

func deductFromAsgardModule(ctx cosmos.Context, mgr *Mgrs, sAsset common.Asset) error {
	asgardModuleBalance := mgr.Keeper().GetBalanceOfModule(ctx, AsgardName, sAsset.Native())
	if asgardModuleBalance.IsZero() {
		ctx.Logger().Info("no asgard module balance for asset", "asset", sAsset.String())
		return nil
	}

	err := mgr.Keeper().SendFromModuleToModule(ctx, AsgardName, ModuleName, common.NewCoins(common.NewCoin(sAsset, asgardModuleBalance)))
	if err != nil {
		ctx.Logger().Error("fail to send from module to asgard", "error", err)
		return err
	}

	if err := mgr.Keeper().BurnFromModule(ctx, ModuleName, common.NewCoin(sAsset, asgardModuleBalance)); err != nil {
		ctx.Logger().Error("fail to burn synth asset", "error", err)
		return err
	}

	return nil
}

type BondedNodeRemovalParams struct {
	LPAddress         common.Address
	BondedNodeAddress common.Address
	Asset             common.Asset
}

func removeBondedNodeFromLP(ctx cosmos.Context, mgr *Mgrs, params BondedNodeRemovalParams) (bool, error) {
	removed := false
	bondedNodeToRemove, err := params.BondedNodeAddress.AccAddress()
	if err != nil {
		return removed, err
	}

	lp, err := mgr.K.GetLiquidityProvider(ctx, params.Asset, params.LPAddress)
	if err != nil {
		return removed, err
	}

	if len(lp.BondedNodes) == 0 {
		ctx.Logger().Error("failed to remove bonded node", "bonded_node", params.BondedNodeAddress.String(), "liquidity_provider", params.LPAddress.String())
		return removed, nil
	}

	var filteredBondedNodes []types.LPBondedNode
	for _, bondedNode := range lp.BondedNodes {
		if bondedNode.NodeAddress.Equals(bondedNodeToRemove) {
			ctx.Logger().Info("removing bonded node from LP", "bonded_node", bondedNode.NodeAddress.String(), "liquidity_provider", lp.CacaoAddress.String())
			removed = true
			continue
		}
		filteredBondedNodes = append(filteredBondedNodes, bondedNode)
	}

	if removed {
		lp.BondedNodes = filteredBondedNodes
		mgr.K.SetLiquidityProvider(ctx, lp)
	}

	return removed, nil
}

func makeTxID(fromAddress common.Address, toAddress common.Address, coins common.Coins, memo string) string {
	str := fmt.Sprintf("%s|%s|%s%s", fromAddress, toAddress, coins, memo)
	return fmt.Sprintf("%X", sha256.Sum256([]byte(str)))
}

// resetObservationHeights will force reset the last chain and last observed heights for
// all active nodes.
func resetObservationHeights(ctx cosmos.Context, mgr *Mgrs, version int, chain common.Chain, height int64) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error(fmt.Sprintf("fail to migrate store to v%d", version), "error", err)
		}
	}()

	// get active nodes
	activeNodes, err := mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		ctx.Logger().Error("failed to get active nodes", "err", err)
		return
	}

	// force set last observed height on all nodes
	for _, node := range activeNodes {
		if err := mgr.Keeper().ForceSetLastObserveHeight(ctx, chain, node.NodeAddress, height); err != nil {
			ctx.Logger().Error("failed to force set last observe height", "err", err, "node", node.NodeAddress, "chain", chain, "height", height)
		}
	}

	// force set chain height
	if err := mgr.Keeper().ForceSetLastChainHeight(ctx, chain, height); err != nil {
		ctx.Logger().Error("failed to force set last chain height", "err", err, "chain", chain, "height", height)
	}
}

// Fixes insolvency for a given asset and amount by subtracting funds from the vault.
func fixInsolvency(ctx cosmos.Context, mgr *Mgrs, vault *Vault, asset common.Asset, amount cosmos.Uint) {
	coin := common.NewCoin(asset, amount)
	ctx.Logger().Info("Fixing insolvency",
		"vault_pubkey", vault.PubKey.String(),
		"amount", amount.String(),
		"asset", asset.String())
	vault.SubFunds(common.NewCoins(coin))

	if err := mgr.Keeper().SetVault(ctx, *vault); err != nil {
		ctx.Logger().Error("fail to save vault after insolvency fix", "error", err)
		return
	}
}
