package mayachain

import (
	"errors"
	"fmt"
	"strings"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

func (h SwapHandler) validateV108(ctx cosmos.Context, msg MsgSwap) error {
	if err := msg.ValidateBasicV63(h.mgr.GetVersion()); err != nil {
		return err
	}

	target := msg.TargetAsset
	if isTradingHalt(ctx, &msg, h.mgr) {
		return errors.New("trading is halted, can't process swap")
	}

	if isLiquidityAuction(ctx, h.mgr.Keeper()) {
		return errors.New("liquidity auction is in progress, can't process swap")
	}

	if target.IsSyntheticAsset() {
		if target.GetLayer1Asset().IsNative() {
			return errors.New("minting a synthetic of a native coin is not allowed")
		}

		// the following  only applicable for chaosnet
		totalLiquidityRUNE, err := h.getTotalLiquidityRUNE(ctx)
		if err != nil {
			return ErrInternal(err, "fail to get total liquidity RUNE")
		}

		var sourceAsset common.Asset
		// total liquidity RUNE after current add liquidity
		if len(msg.Tx.Coins) > 0 {
			// calculate rune value on incoming swap, and add to total liquidity.
			coin := msg.Tx.Coins[0]
			sourceAsset = coin.Asset
			runeVal := coin.Amount
			if !coin.Asset.IsBase() {
				var pool Pool
				pool, err = h.mgr.Keeper().GetPool(ctx, coin.Asset.GetLayer1Asset())
				if err != nil {
					return ErrInternal(err, "fail to get pool")
				}
				runeVal = pool.AssetValueInRune(coin.Amount)
			}
			totalLiquidityRUNE = totalLiquidityRUNE.Add(runeVal)
		}
		maximumLiquidityRune, err := h.mgr.Keeper().GetMimir(ctx, constants.MaximumLiquidityCacao.String())
		if maximumLiquidityRune < 0 || err != nil {
			maximumLiquidityRune = h.mgr.GetConstants().GetInt64Value(constants.MaximumLiquidityCacao)
		}
		if maximumLiquidityRune > 0 {
			if totalLiquidityRUNE.GT(cosmos.NewUint(uint64(maximumLiquidityRune))) {
				return errAddLiquidityRUNEOverLimit
			}
		}

		// fail validation if synth supply is already too high, relative to pool depth
		err = isSynthMintPaused(ctx, h.mgr, target, cosmos.ZeroUint())
		if err != nil {
			return err
		}

		ensureLiquidityNoLargerThanBond := h.mgr.GetConstants().GetBoolValue(constants.StrictBondLiquidityRatio)
		if !ensureLiquidityNoLargerThanBond {
			return nil
		}
		securityBond, err := h.getEffectiveSecurityBond(ctx, h.mgr)
		if err != nil {
			return ErrInternal(err, "fail to get security bond RUNE")
		}
		// If source and target are synthetic assets there is no net liquidity gain (RUNE is just moved from pool A to pool B),
		// so skip this check
		if totalLiquidityRUNE.GT(securityBond) && !sourceAsset.IsSyntheticAsset() {
			ctx.Logger().Info("total liquidity RUNE is more than effective security bond", "liquidity rune", totalLiquidityRUNE, "effective security bond", securityBond)
			return errAddLiquidityCACAOMoreThanBond
		}
	}

	if len(msg.Aggregator) > 0 {
		swapOutDisabled := fetchConfigInt64(ctx, h.mgr, constants.SwapOutDexAggregationDisabled)
		if swapOutDisabled > 0 {
			return errors.New("swap out dex integration disabled")
		}
		if !msg.TargetAsset.Equals(msg.TargetAsset.Chain.GetGasAsset()) {
			return fmt.Errorf("target asset (%s) is not gas asset , can't use dex feature", msg.TargetAsset)
		}
		// validate that a referenced dex aggregator is legit
		addr, err := FetchDexAggregator(h.mgr.GetVersion(), target.Chain, msg.Aggregator)
		if err != nil {
			return err
		}
		if addr == "" {
			return fmt.Errorf("aggregator address is empty")
		}
		if len(msg.AggregatorTargetAddress) == 0 {
			return fmt.Errorf("aggregator target address is empty")
		}
	}

	return nil
}

func (h SwapHandler) validateV95(ctx cosmos.Context, msg MsgSwap) error {
	if err := msg.ValidateBasicV63(h.mgr.GetVersion()); err != nil {
		return err
	}

	target := msg.TargetAsset
	if isTradingHalt(ctx, &msg, h.mgr) {
		return errors.New("trading is halted, can't process swap")
	}

	if target.IsSyntheticAsset() {
		// the following  only applicable for chaosnet
		totalLiquidityRUNE, err := h.getTotalLiquidityRUNE(ctx)
		if err != nil {
			return ErrInternal(err, "fail to get total liquidity RUNE")
		}

		// total liquidity RUNE after current add liquidity
		if len(msg.Tx.Coins) > 0 {
			// calculate rune value on incoming swap, and add to total liquidity.
			coin := msg.Tx.Coins[0]
			runeVal := coin.Amount
			if !coin.Asset.IsBase() {
				var pool Pool
				pool, err = h.mgr.Keeper().GetPool(ctx, coin.Asset.GetLayer1Asset())
				if err != nil {
					return ErrInternal(err, "fail to get pool")
				}
				runeVal = pool.AssetValueInRune(coin.Amount)
			}
			totalLiquidityRUNE = totalLiquidityRUNE.Add(runeVal)
		}
		maximumLiquidityRune, err := h.mgr.Keeper().GetMimir(ctx, constants.MaximumLiquidityCacao.String())
		if maximumLiquidityRune < 0 || err != nil {
			maximumLiquidityRune = h.mgr.GetConstants().GetInt64Value(constants.MaximumLiquidityCacao)
		}
		if maximumLiquidityRune > 0 {
			if totalLiquidityRUNE.GT(cosmos.NewUint(uint64(maximumLiquidityRune))) {
				return errAddLiquidityRUNEOverLimit
			}
		}

		// fail validation if synth supply is already too high, relative to pool depth
		maxSynths, err := h.mgr.Keeper().GetMimir(ctx, constants.MaxSynthPerAssetDepth.String())
		if maxSynths < 0 || err != nil {
			maxSynths = h.mgr.GetConstants().GetInt64Value(constants.MaxSynthPerAssetDepth)
		}
		synthSupply := h.mgr.Keeper().GetTotalSupply(ctx, target.GetSyntheticAsset())
		pool, err := h.mgr.Keeper().GetPool(ctx, target.GetLayer1Asset())
		if err != nil {
			return ErrInternal(err, "fail to get pool")
		}
		if pool.BalanceAsset.IsZero() {
			return fmt.Errorf("pool(%s) has zero asset balance", pool.Asset.String())
		}
		coverage := synthSupply.MulUint64(MaxWithdrawBasisPoints).Quo(pool.BalanceAsset).Uint64()
		if coverage > uint64(maxSynths) {
			return fmt.Errorf("synth quantity is too high relative to asset depth of related pool (%d/%d)", coverage, maxSynths)
		}

		ensureLiquidityNoLargerThanBond := h.mgr.GetConstants().GetBoolValue(constants.StrictBondLiquidityRatio)
		if !ensureLiquidityNoLargerThanBond {
			return nil
		}
		securityBond, err := h.getEffectiveSecurityBond(ctx, h.mgr)
		if err != nil {
			return ErrInternal(err, "fail to get security bond RUNE")
		}
		if totalLiquidityRUNE.GT(securityBond) {
			ctx.Logger().Info("total liquidity RUNE is more than effective security bond", "liquidity rune", totalLiquidityRUNE, "effective security bond", securityBond)
			return errAddLiquidityCACAOMoreThanBond
		}
	}

	if len(msg.Aggregator) > 0 {
		swapOutDisabled := fetchConfigInt64(ctx, h.mgr, constants.SwapOutDexAggregationDisabled)
		if swapOutDisabled > 0 {
			return errors.New("swap out dex integration disabled")
		}
		if !msg.TargetAsset.Equals(msg.TargetAsset.Chain.GetGasAsset()) {
			return fmt.Errorf("target asset (%s) is not gas asset , can't use dex feature", msg.TargetAsset)
		}
		// validate that a referenced dex aggregator is legit
		addr, err := FetchDexAggregator(h.mgr.GetVersion(), target.Chain, msg.Aggregator)
		if err != nil {
			return err
		}
		if addr == "" {
			return fmt.Errorf("aggregator address is empty")
		}
		if len(msg.AggregatorTargetAddress) == 0 {
			return fmt.Errorf("aggregator target address is empty")
		}
	}

	return nil
}

func (h SwapHandler) validateV101(ctx cosmos.Context, msg MsgSwap) error {
	if err := msg.ValidateBasicV63(h.mgr.GetVersion()); err != nil {
		return err
	}

	target := msg.TargetAsset
	if isTradingHalt(ctx, &msg, h.mgr) {
		return errors.New("trading is halted, can't process swap")
	}

	if isLiquidityAuction(ctx, h.mgr.Keeper()) {
		return errors.New("liquidity auction is in progress, can't process swap")
	}

	if target.IsSyntheticAsset() {
		// the following  only applicable for chaosnet
		totalLiquidityRUNE, err := h.getTotalLiquidityRUNE(ctx)
		if err != nil {
			return ErrInternal(err, "fail to get total liquidity RUNE")
		}

		// total liquidity RUNE after current add liquidity
		if len(msg.Tx.Coins) > 0 {
			// calculate rune value on incoming swap, and add to total liquidity.
			coin := msg.Tx.Coins[0]
			runeVal := coin.Amount
			if !coin.Asset.IsBase() {
				var pool Pool
				pool, err = h.mgr.Keeper().GetPool(ctx, coin.Asset.GetLayer1Asset())
				if err != nil {
					return ErrInternal(err, "fail to get pool")
				}
				runeVal = pool.AssetValueInRune(coin.Amount)
			}
			totalLiquidityRUNE = totalLiquidityRUNE.Add(runeVal)
		}
		maximumLiquidityRune, err := h.mgr.Keeper().GetMimir(ctx, constants.MaximumLiquidityCacao.String())
		if maximumLiquidityRune < 0 || err != nil {
			maximumLiquidityRune = h.mgr.GetConstants().GetInt64Value(constants.MaximumLiquidityCacao)
		}
		if maximumLiquidityRune > 0 {
			if totalLiquidityRUNE.GT(cosmos.NewUint(uint64(maximumLiquidityRune))) {
				return errAddLiquidityRUNEOverLimit
			}
		}

		// fail validation if synth supply is already too high, relative to pool depth
		maxSynths, err := h.mgr.Keeper().GetMimir(ctx, constants.MaxSynthPerAssetDepth.String())
		if maxSynths < 0 || err != nil {
			maxSynths = h.mgr.GetConstants().GetInt64Value(constants.MaxSynthPerAssetDepth)
		}
		synthSupply := h.mgr.Keeper().GetTotalSupply(ctx, target.GetSyntheticAsset())
		pool, err := h.mgr.Keeper().GetPool(ctx, target.GetLayer1Asset())
		if err != nil {
			return ErrInternal(err, "fail to get pool")
		}
		if pool.BalanceAsset.IsZero() {
			return fmt.Errorf("pool(%s) has zero asset balance", pool.Asset.String())
		}
		coverage := synthSupply.MulUint64(MaxWithdrawBasisPoints).Quo(pool.BalanceAsset).Uint64()
		if coverage > uint64(maxSynths) {
			return fmt.Errorf("synth quantity is too high relative to asset depth of related pool (%d/%d)", coverage, maxSynths)
		}

		ensureLiquidityNoLargerThanBond := h.mgr.GetConstants().GetBoolValue(constants.StrictBondLiquidityRatio)
		if !ensureLiquidityNoLargerThanBond {
			return nil
		}
		securityBond, err := h.getEffectiveSecurityBond(ctx, h.mgr)
		if err != nil {
			return ErrInternal(err, "fail to get security bond RUNE")
		}
		if totalLiquidityRUNE.GT(securityBond) {
			ctx.Logger().Info("total liquidity RUNE is more than effective security bond", "liquidity rune", totalLiquidityRUNE, "effective security bond", securityBond)
			return errAddLiquidityCACAOMoreThanBond
		}
	}

	if len(msg.Aggregator) > 0 {
		swapOutDisabled := fetchConfigInt64(ctx, h.mgr, constants.SwapOutDexAggregationDisabled)
		if swapOutDisabled > 0 {
			return errors.New("swap out dex integration disabled")
		}
		if !msg.TargetAsset.Equals(msg.TargetAsset.Chain.GetGasAsset()) {
			return fmt.Errorf("target asset (%s) is not gas asset , can't use dex feature", msg.TargetAsset)
		}
		// validate that a referenced dex aggregator is legit
		addr, err := FetchDexAggregator(h.mgr.GetVersion(), target.Chain, msg.Aggregator)
		if err != nil {
			return err
		}
		if addr == "" {
			return fmt.Errorf("aggregator address is empty")
		}
		if len(msg.AggregatorTargetAddress) == 0 {
			return fmt.Errorf("aggregator target address is empty")
		}
	}

	return nil
}

func (h SwapHandler) handleV110(ctx cosmos.Context, msg MsgSwap) (*cosmos.Result, error) {
	// We use TargetAsset instead of Destination since Address.GetChain() iterates over all chains bumping ETH
	// before ARB
	destinationChain := msg.TargetAsset.GetChain()
	// test that the network we are running matches the destination network
	ctx.Logger().Info("destination chain", "destinationChain", destinationChain)
	ctx.Logger().Info("current chain network", "currentChainNetwork", common.CurrentChainNetwork)
	ctx.Logger().Info("msg destination network", "msgDestinationNetwork", msg.Destination.GetNetwork(h.mgr.GetVersion(), destinationChain))
	if !common.CurrentChainNetwork.SoftEquals(msg.Destination.GetNetwork(h.mgr.GetVersion(), destinationChain)) {
		return nil, fmt.Errorf("address(%s) is not same network", msg.Destination)
	}
	synthVirtualDepthMult, err := h.mgr.Keeper().GetMimir(ctx, constants.VirtualMultSynthsBasisPoints.String())
	if synthVirtualDepthMult < 1 || err != nil {
		synthVirtualDepthMult = h.mgr.GetConstants().GetInt64Value(constants.VirtualMultSynthsBasisPoints)
	}

	if msg.TargetAsset.IsBase() && !msg.TargetAsset.IsNativeBase() {
		return nil, fmt.Errorf("target asset can't be %s", msg.TargetAsset.String())
	}

	dexAgg := ""
	dexAggTargetAsset := ""
	if len(msg.Aggregator) > 0 {
		dexAgg, err = FetchDexAggregator(h.mgr.GetVersion(), msg.TargetAsset.Chain, msg.Aggregator)
		if err != nil {
			return nil, err
		}
	}
	dexAggTargetAsset = msg.AggregatorTargetAddress

	swapper, err := GetSwapper(h.mgr.Keeper().GetVersion())
	if err != nil {
		return nil, err
	}

	swp := msg.GetStreamingSwap()
	if msg.IsStreaming() {
		if h.mgr.Keeper().StreamingSwapExists(ctx, msg.Tx.ID) {
			swp, err = h.mgr.Keeper().GetStreamingSwap(ctx, msg.Tx.ID)
			if err != nil {
				ctx.Logger().Error("fail to fetch streaming swap", "error", err)
				return nil, err
			}
		}

		// for first swap only, override interval and quantity (if needed)
		if swp.Count == 0 {
			// ensure interval is never larger than max length, override if so
			maxLength := fetchConfigInt64(ctx, h.mgr, constants.StreamingSwapMaxLength)
			if uint64(maxLength) < swp.Interval {
				swp.Interval = uint64(maxLength)
			}

			sourceAsset := msg.Tx.Coins[0].Asset
			targetAsset := msg.TargetAsset
			var maxSwapQuantity uint64
			maxSwapQuantity, err = getMaxSwapQuantity(ctx, h.mgr, sourceAsset, targetAsset, swp)
			if err != nil {
				return nil, err
			}
			if swp.Quantity == 0 || swp.Quantity > maxSwapQuantity {
				swp.Quantity = maxSwapQuantity
			}
		}
		h.mgr.Keeper().SetStreamingSwap(ctx, swp)
		// hijack the inbound amount
		// NOTE: its okay if the amount is zero. The swap will fail as it
		// should, which will cause the swap queue manager later to send out
		// the In/Out amounts accordingly
		msg.Tx.Coins[0].Amount, msg.TradeTarget = swp.NextSize(h.mgr.GetVersion())
	}

	emit, _, swapErr := swapper.Swap(
		ctx,
		h.mgr.Keeper(),
		msg.Tx,
		msg.TargetAsset,
		msg.Destination,
		msg.TradeTarget,
		dexAgg,
		dexAggTargetAsset,
		msg.AggregatorTargetLimit,
		swp,
		cosmos.ZeroUint(),
		synthVirtualDepthMult,
		h.mgr)
	if swapErr != nil {
		return nil, swapErr
	}

	// Check if swap to a synth would cause synth supply to exceed MaxSynthPerPoolDepth cap
	if msg.TargetAsset.IsSyntheticAsset() && !msg.IsStreaming() {
		err = isSynthMintPaused(ctx, h.mgr, msg.TargetAsset, emit)
		if err != nil {
			return nil, err
		}
	}

	if msg.IsStreaming() {
		// only increment In/Out if we have a successful swap
		swp.In = swp.In.Add(msg.Tx.Coins[0].Amount)
		swp.Out = swp.Out.Add(emit)
		h.mgr.Keeper().SetStreamingSwap(ctx, swp)
		if !swp.IsLastSwap() {
			// exit early so we don't execute follow-on handlers mid streaming swap. if this
			// is the last swap execute the follow-on handlers as swap count is incremented in
			// the swap queue manager
			return &cosmos.Result{}, nil
		}
		emit = swp.Out
	}

	mem, err := ParseMemoWithMAYANames(ctx, h.mgr.Keeper(), msg.Tx.Memo)
	if err != nil {
		ctx.Logger().Error("swap handler failed to parse memo", "memo", msg.Tx.Memo, "error", err)
		return nil, err
	}
	if mem.IsType(TxAdd) {
		m, ok := mem.(AddLiquidityMemo)
		if !ok {
			return nil, fmt.Errorf("fail to cast add liquidity memo")
		}
		m.Asset = fuzzyAssetMatch(ctx, h.mgr.Keeper(), m.Asset)
		msg.Tx.Coins = common.NewCoins(common.NewCoin(m.Asset, emit))
		obTx := ObservedTx{Tx: msg.Tx}
		msg, err := getMsgAddLiquidityFromMemo(ctx, m, obTx, msg.Signer, 0)
		if err != nil {
			return nil, err
		}
		handler := NewAddLiquidityHandler(h.mgr)
		_, err = handler.Run(ctx, msg)
		if err != nil {
			ctx.Logger().Error("swap handler failed to add liquidity", "error", err)
			return nil, err
		}
	}

	return &cosmos.Result{}, nil
}

func (h SwapHandler) handleV95(ctx cosmos.Context, msg MsgSwap) (*cosmos.Result, error) {
	destinationChain := msg.Destination.GetChain(h.mgr.GetVersion())
	// test that the network we are running matches the destination network
	if !common.CurrentChainNetwork.SoftEquals(msg.Destination.GetNetwork(h.mgr.GetVersion(), destinationChain)) {
		return nil, fmt.Errorf("address(%s) is not same network", msg.Destination)
	}
	transactionFee := h.mgr.GasMgr().GetFee(ctx, destinationChain, common.BaseAsset())
	synthVirtualDepthMult, err := h.mgr.Keeper().GetMimir(ctx, constants.VirtualMultSynthsBasisPoints.String())
	if synthVirtualDepthMult < 1 || err != nil {
		synthVirtualDepthMult = h.mgr.GetConstants().GetInt64Value(constants.VirtualMultSynthsBasisPoints)
	}

	if msg.TargetAsset.IsBase() && !msg.TargetAsset.IsNativeBase() {
		return nil, fmt.Errorf("target asset can't be %s", msg.TargetAsset.String())
	}

	dexAgg := ""
	dexAggTargetAsset := ""
	if len(msg.Aggregator) > 0 {
		dexAgg, err = FetchDexAggregator(h.mgr.GetVersion(), msg.TargetAsset.Chain, msg.Aggregator)
		if err != nil {
			return nil, err
		}
	}
	dexAggTargetAsset = msg.AggregatorTargetAddress

	swapper, err := GetSwapper(h.mgr.Keeper().GetVersion())
	if err != nil {
		return nil, err
	}

	emit, _, swapErr := swapper.Swap(
		ctx,
		h.mgr.Keeper(),
		msg.Tx,
		msg.TargetAsset,
		msg.Destination,
		msg.TradeTarget,
		dexAgg,
		dexAggTargetAsset,
		msg.AggregatorTargetLimit,
		StreamingSwap{},
		transactionFee,
		synthVirtualDepthMult,
		h.mgr)
	if swapErr != nil {
		return nil, swapErr
	}

	mem, err := ParseMemoWithMAYANames(ctx, h.mgr.Keeper(), msg.Tx.Memo)
	if err != nil {
		ctx.Logger().Error("swap handler failed to parse memo", "memo", msg.Tx.Memo, "error", err)
		return nil, err
	}
	if mem.IsType(TxAdd) {
		m, ok := mem.(AddLiquidityMemo)
		if !ok {
			return nil, fmt.Errorf("fail to cast add liquidity memo")
		}
		m.Asset = fuzzyAssetMatch(ctx, h.mgr.Keeper(), m.Asset)
		msg.Tx.Coins = common.NewCoins(common.NewCoin(m.Asset, emit))
		obTx := ObservedTx{Tx: msg.Tx}
		msg, err := getMsgAddLiquidityFromMemo(ctx, m, obTx, msg.Signer, 0)
		if err != nil {
			return nil, err
		}
		handler := NewAddLiquidityHandler(h.mgr)
		_, err = handler.Run(ctx, msg)
		if err != nil {
			ctx.Logger().Error("swap handler failed to add liquidity", "error", err)
			return nil, err
		}
	}

	return &cosmos.Result{}, nil
}

func (h SwapHandler) handleV108(ctx cosmos.Context, msg MsgSwap) (*cosmos.Result, error) {
	destinationChain := msg.Destination.GetChain(h.mgr.GetVersion())
	// test that the network we are running matches the destination network
	if !common.CurrentChainNetwork.SoftEquals(msg.Destination.GetNetwork(h.mgr.GetVersion(), destinationChain)) {
		return nil, fmt.Errorf("address(%s) is not same network", msg.Destination)
	}
	transactionFee := h.mgr.GasMgr().GetFee(ctx, destinationChain, common.BaseAsset())
	synthVirtualDepthMult, err := h.mgr.Keeper().GetMimir(ctx, constants.VirtualMultSynthsBasisPoints.String())
	if synthVirtualDepthMult < 1 || err != nil {
		synthVirtualDepthMult = h.mgr.GetConstants().GetInt64Value(constants.VirtualMultSynthsBasisPoints)
	}

	if msg.TargetAsset.IsBase() && !msg.TargetAsset.IsNativeBase() {
		return nil, fmt.Errorf("target asset can't be %s", msg.TargetAsset.String())
	}

	dexAgg := ""
	dexAggTargetAsset := ""
	if len(msg.Aggregator) > 0 {
		dexAgg, err = FetchDexAggregator(h.mgr.GetVersion(), msg.TargetAsset.Chain, msg.Aggregator)
		if err != nil {
			return nil, err
		}
	}
	dexAggTargetAsset = msg.AggregatorTargetAddress

	swapper, err := GetSwapper(h.mgr.Keeper().GetVersion())
	if err != nil {
		return nil, err
	}

	emit, _, swapErr := swapper.Swap(
		ctx,
		h.mgr.Keeper(),
		msg.Tx,
		msg.TargetAsset,
		msg.Destination,
		msg.TradeTarget,
		dexAgg,
		dexAggTargetAsset,
		msg.AggregatorTargetLimit,
		StreamingSwap{},
		transactionFee,
		synthVirtualDepthMult,
		h.mgr)
	if swapErr != nil {
		return nil, swapErr
	}

	// Check if swap to a synth would cause synth supply to exceed MaxSynthPerPoolDepth cap
	if msg.TargetAsset.IsSyntheticAsset() {
		err = isSynthMintPaused(ctx, h.mgr, msg.TargetAsset, emit)
		if err != nil {
			return nil, err
		}
	}

	mem, err := ParseMemoWithMAYANames(ctx, h.mgr.Keeper(), msg.Tx.Memo)
	if err != nil {
		ctx.Logger().Error("swap handler failed to parse memo", "memo", msg.Tx.Memo, "error", err)
		return nil, err
	}
	if mem.IsType(TxAdd) {
		m, ok := mem.(AddLiquidityMemo)
		if !ok {
			return nil, fmt.Errorf("fail to cast add liquidity memo")
		}
		m.Asset = fuzzyAssetMatch(ctx, h.mgr.Keeper(), m.Asset)
		msg.Tx.Coins = common.NewCoins(common.NewCoin(m.Asset, emit))
		obTx := ObservedTx{Tx: msg.Tx}
		msg, err := getMsgAddLiquidityFromMemo(ctx, m, obTx, msg.Signer, 0)
		if err != nil {
			return nil, err
		}
		handler := NewAddLiquidityHandler(h.mgr)
		_, err = handler.Run(ctx, msg)
		if err != nil {
			ctx.Logger().Error("swap handler failed to add liquidity", "error", err)
			return nil, err
		}
	}

	return &cosmos.Result{}, nil
}

func (h SwapHandler) validateV110(ctx cosmos.Context, msg MsgSwap) error {
	if err := msg.ValidateBasicV63(h.mgr.GetVersion()); err != nil {
		return err
	}

	target := msg.TargetAsset
	if isTradingHalt(ctx, &msg, h.mgr) {
		return errors.New("trading is halted, can't process swap")
	}

	// For external-origin (here valid) memos, do not allow a network module as the final destination.
	// If unable to parse the memo, here assume it to be internal.
	memo, _ := ParseMemoWithMAYANames(ctx, h.mgr.Keeper(), msg.Tx.Memo)
	mem, isSwapMemo := memo.(SwapMemo)
	if isSwapMemo {
		destAccAddr, err := mem.Destination.AccAddress()
		// A network module address would be resolvable,
		// so if not resolvable it should not be a network module address.
		if err == nil && IsModuleAccAddress(h.mgr.Keeper(), destAccAddr) {
			return fmt.Errorf("a network module cannot be the final destination of a swap memo")
		}
	}

	if isLiquidityAuction(ctx, h.mgr.Keeper()) {
		return errors.New("liquidity auction is in progress, can't process swap")
	}

	if len(msg.Aggregator) > 0 {
		swapOutDisabled := fetchConfigInt64(ctx, h.mgr, constants.SwapOutDexAggregationDisabled)
		if swapOutDisabled > 0 {
			return errors.New("swap out dex integration disabled")
		}
		if !msg.TargetAsset.Equals(msg.TargetAsset.Chain.GetGasAsset()) {
			return fmt.Errorf("target asset (%s) is not gas asset , can't use dex feature", msg.TargetAsset)
		}
		// validate that a referenced dex aggregator is legit
		addr, err := FetchDexAggregator(h.mgr.GetVersion(), target.Chain, msg.Aggregator)
		if err != nil {
			return err
		}
		if addr == "" {
			return fmt.Errorf("aggregator address is empty")
		}
		if len(msg.AggregatorTargetAddress) == 0 {
			return fmt.Errorf("aggregator target address is empty")
		}
	}

	if target.IsSyntheticAsset() && target.GetLayer1Asset().IsNative() {
		return errors.New("minting a synthetic of a native coin is not allowed")
	}

	var sourceCoin common.Coin
	if len(msg.Tx.Coins) > 0 {
		sourceCoin = msg.Tx.Coins[0]
	}

	if msg.IsStreaming() {
		pausedStreaming := fetchConfigInt64(ctx, h.mgr, constants.StreamingSwapPause)
		if pausedStreaming > 0 {
			return fmt.Errorf("streaming swaps are paused")
		}

		// if either source or target in ragnarok, streaming is not allowed
		for _, asset := range []common.Asset{sourceCoin.Asset, target} {
			key := "RAGNAROK-" + asset.MimirString()
			ragnarok, err := h.mgr.Keeper().GetMimir(ctx, key)
			if err == nil && ragnarok > 0 {
				return fmt.Errorf("streaming swaps disabled on ragnarok asset %s", asset)
			}
		}

		swp := msg.GetStreamingSwap()
		if h.mgr.Keeper().StreamingSwapExists(ctx, msg.Tx.ID) {
			var err error
			swp, err = h.mgr.Keeper().GetStreamingSwap(ctx, msg.Tx.ID)
			if err != nil {
				ctx.Logger().Error("fail to fetch streaming swap", "error", err)
				return err
			}
		}

		if (swp.Quantity > 0 && swp.IsDone()) || swp.In.GTE(swp.Deposit) {
			// check both swap count and swap in vs deposit to cover all basis
			return fmt.Errorf("streaming swap is completed, cannot continue to swap again")
		}

		if swp.Count > 0 {
			// end validation early, as synth TVL caps are not applied to streaming
			// swaps. This is to ensure that streaming swaps don't get interrupted
			// and cause a partial fulfillment, which would cause issues for
			// internal streaming swaps for savers and loans.
			return nil
		} else {
			// first swap we check the entire swap amount (not just the
			// sub-swap amount) to ensure the value of the entire has TVL/synth
			// room
			sourceCoin.Amount = swp.Deposit
		}
	}

	if target.IsSyntheticAsset() {
		// the following is only applicable for mainnet
		totalLiquidityCACAO, err := h.getTotalLiquidityRUNE(ctx)
		if err != nil {
			return ErrInternal(err, "fail to get total liquidity RUNE")
		}

		// total liquidity RUNE after current add liquidity
		if len(msg.Tx.Coins) > 0 {
			// calculate rune value on incoming swap, and add to total liquidity.
			runeVal := sourceCoin.Amount
			if !sourceCoin.Asset.IsBase() {
				var pool Pool
				pool, err = h.mgr.Keeper().GetPool(ctx, sourceCoin.Asset.GetLayer1Asset())
				if err != nil {
					return ErrInternal(err, "fail to get pool")
				}
				runeVal = pool.AssetValueInRune(sourceCoin.Amount)
			}
			totalLiquidityCACAO = totalLiquidityCACAO.Add(runeVal)
		}
		maximumLiquidityRune, err := h.mgr.Keeper().GetMimir(ctx, constants.MaximumLiquidityCacao.String())
		if maximumLiquidityRune < 0 || err != nil {
			maximumLiquidityRune = h.mgr.GetConstants().GetInt64Value(constants.MaximumLiquidityCacao)
		}
		if maximumLiquidityRune > 0 {
			if totalLiquidityCACAO.GT(cosmos.NewUint(uint64(maximumLiquidityRune))) {
				return errAddLiquidityRUNEOverLimit
			}
		}

		// fail validation if synth supply is already too high, relative to pool depth
		// do a simulated swap to see how much of the target synth the network
		// will need to mint and check if that amount exceeds limits
		targetAmount, cacaoAmount := cosmos.ZeroUint(), cosmos.ZeroUint()
		swapper, err := GetSwapper(h.mgr.GetVersion())
		if err == nil {
			if sourceCoin.Asset.IsBase() {
				cacaoAmount = sourceCoin.Amount
			} else {
				// asset --> rune swap
				sourceAssetPool := sourceCoin.Asset
				if sourceAssetPool.IsSyntheticAsset() {
					sourceAssetPool = sourceAssetPool.GetLayer1Asset()
				}
				var sourcePool Pool
				sourcePool, err = h.mgr.Keeper().GetPool(ctx, sourceAssetPool)
				if err != nil {
					ctx.Logger().Error("fail to fetch pool for swap simulation", "error", err)
				} else {
					cacaoAmount = swapper.CalcAssetEmission(sourcePool.BalanceAsset, sourceCoin.Amount, sourcePool.BalanceCacao)
				}
			}
			// rune --> synth swap
			var targetPool Pool
			targetPool, err = h.mgr.Keeper().GetPool(ctx, target.GetLayer1Asset())
			if err != nil {
				ctx.Logger().Error("fail to fetch pool for swap simulation", "error", err)
			} else {
				targetAmount = swapper.CalcAssetEmission(targetPool.BalanceCacao, cacaoAmount, targetPool.BalanceAsset)
			}
		}

		err = isSynthMintPaused(ctx, h.mgr, target, targetAmount)
		if err != nil {
			return err
		}

		ensureLiquidityNoLargerThanBondInt64, err := h.mgr.Keeper().GetMimir(ctx, "EnsureLiquidityNoLargerThanBond")
		if err != nil {
			ctx.Logger().Error("fail to get mimir", "error", err)
		}
		// 0 not active, 1 active, -1 use default
		var ensureLiquidityNoLargerThanBond bool
		if ensureLiquidityNoLargerThanBondInt64 < 0 {
			ensureLiquidityNoLargerThanBond = h.mgr.GetConstants().GetBoolValue(constants.StrictBondLiquidityRatio)
		} else {
			ensureLiquidityNoLargerThanBond = ensureLiquidityNoLargerThanBondInt64 == 1
		}

		// If source and target are synthetic assets there is no net liquidity gain (RUNE is just moved from pool A to pool B),
		// so skip this check
		if ensureLiquidityNoLargerThanBond {
			if !sourceCoin.Asset.IsSyntheticAsset() && atTVLCap(ctx, common.NewCoins(sourceCoin), h.mgr) {
				return errAddLiquidityCACAOMoreThanBond
			}
		}
	}

	return nil
}

func (h SwapHandler) handleV112(ctx cosmos.Context, msg MsgSwap) (*cosmos.Result, error) {
	// We use TargetAsset instead of Destination since Address.GetChain() iterates over all chains bumping ETH
	// before ARB
	destinationChain := msg.TargetAsset.GetChain()
	// test that the network we are running matches the destination network
	// Don't change msg.Destination here; this line was introduced to avoid people from swapping mainnet asset,
	// but using mocknet address.
	ctx.Logger().Info("destination chain", "destinationChain", destinationChain)
	ctx.Logger().Info("current chain network", "currentChainNetwork", common.CurrentChainNetwork)
	ctx.Logger().Info("msg destination network", "msgDestinationNetwork", msg.Destination.GetNetwork(h.mgr.GetVersion(), destinationChain))
	if !common.CurrentChainNetwork.SoftEquals(msg.Destination.GetNetwork(h.mgr.GetVersion(), destinationChain)) {
		return nil, fmt.Errorf("address(%s) is not same network", msg.Destination)
	}
	synthVirtualDepthMult, err := h.mgr.Keeper().GetMimir(ctx, constants.VirtualMultSynthsBasisPoints.String())
	if synthVirtualDepthMult < 1 || err != nil {
		synthVirtualDepthMult = h.mgr.GetConstants().GetInt64Value(constants.VirtualMultSynthsBasisPoints)
	}

	if msg.TargetAsset.IsBase() && !msg.TargetAsset.IsNativeBase() {
		return nil, fmt.Errorf("target asset can't be %s", msg.TargetAsset.String())
	}

	dexAgg := ""
	dexAggTargetAsset := ""
	if len(msg.Aggregator) > 0 {
		dexAgg, err = FetchDexAggregator(h.mgr.GetVersion(), msg.TargetAsset.Chain, msg.Aggregator)
		if err != nil {
			return nil, err
		}
	}
	dexAggTargetAsset = msg.AggregatorTargetAddress

	swapper, err := GetSwapper(h.mgr.Keeper().GetVersion())
	if err != nil {
		return nil, err
	}

	swp := msg.GetStreamingSwap()
	if msg.IsStreaming() {
		if h.mgr.Keeper().StreamingSwapExists(ctx, msg.Tx.ID) {
			swp, err = h.mgr.Keeper().GetStreamingSwap(ctx, msg.Tx.ID)
			if err != nil {
				ctx.Logger().Error("fail to fetch streaming swap", "error", err)
				return nil, err
			}
		}

		// for first swap only, override interval and quantity (if needed)
		if swp.Count == 0 {
			// ensure interval is never larger than max length, override if so
			maxLength := fetchConfigInt64(ctx, h.mgr, constants.StreamingSwapMaxLength)
			if uint64(maxLength) < swp.Interval {
				swp.Interval = uint64(maxLength)
			}

			sourceAsset := msg.Tx.Coins[0].Asset
			targetAsset := msg.TargetAsset
			var maxSwapQuantity uint64
			maxSwapQuantity, err = getMaxSwapQuantity(ctx, h.mgr, sourceAsset, targetAsset, swp)
			if err != nil {
				return nil, err
			}
			if swp.Quantity == 0 || swp.Quantity > maxSwapQuantity {
				swp.Quantity = maxSwapQuantity
			}
		}
		h.mgr.Keeper().SetStreamingSwap(ctx, swp)
		// hijack the inbound amount
		// NOTE: its okay if the amount is zero. The swap will fail as it
		// should, which will cause the swap queue manager later to send out
		// the In/Out amounts accordingly
		msg.Tx.Coins[0].Amount, msg.TradeTarget = swp.NextSize(h.mgr.GetVersion())
	}

	emit, _, swapErr := swapper.Swap(
		ctx,
		h.mgr.Keeper(),
		msg.Tx,
		msg.TargetAsset,
		msg.Destination,
		msg.TradeTarget,
		dexAgg,
		dexAggTargetAsset,
		msg.AggregatorTargetLimit,
		swp,
		cosmos.ZeroUint(),
		synthVirtualDepthMult,
		h.mgr)
	if swapErr != nil {
		return nil, swapErr
	}

	// Check if swap is to AffiliateCollector Module, if so, add the accrued RUNE for the affiliate
	affColAddress, err := h.mgr.Keeper().GetModuleAddress(AffiliateCollectorName)
	if err != nil {
		ctx.Logger().Error("failed to retrieve AffiliateCollector module address", "error", err)
	}

	if msg.Destination.Equals(affColAddress) {
		// Add accrued CACAO for this affiliate
		owner, err2 := msg.AffiliateAddress.AccAddress()
		if err2 != nil {
			ctx.Logger().Error("failed to retrieve AccAddress for mayaname owner", "address", msg.AffiliateAddress, "error", err2)
			return &cosmos.Result{}, err2
		}
		affCol, err2 := h.mgr.Keeper().GetAffiliateCollector(ctx, owner)
		if err2 != nil {
			ctx.Logger().Error("failed to retrieve AffiliateCollector for mayaname owner", "address", msg.AffiliateAddress, "error", err2)
			return &cosmos.Result{}, err2
		}
		// The TargetAsset has already been established to be CACAO.
		transactionFee := h.mgr.GasMgr().GetFee(ctx, common.BASEChain, common.BaseAsset())
		addCacaoAmt := common.SafeSub(emit, transactionFee)
		affCol.CacaoAmount = affCol.CacaoAmount.Add(addCacaoAmt)
		h.mgr.Keeper().SetAffiliateCollector(ctx, affCol)

		return &cosmos.Result{}, nil
	}

	// Check if swap to a synth would cause synth supply to exceed MaxSynthPerPoolDepth cap
	if msg.TargetAsset.IsSyntheticAsset() && !msg.IsStreaming() {
		err = isSynthMintPaused(ctx, h.mgr, msg.TargetAsset, emit)
		if err != nil {
			return nil, err
		}
	}

	if msg.IsStreaming() {
		// only increment In/Out if we have a successful swap
		swp.In = swp.In.Add(msg.Tx.Coins[0].Amount)
		swp.Out = swp.Out.Add(emit)
		h.mgr.Keeper().SetStreamingSwap(ctx, swp)
		if !swp.IsLastSwap() {
			// exit early so we don't execute follow-on handlers mid streaming swap. if this
			// is the last swap execute the follow-on handlers as swap count is incremented in
			// the swap queue manager
			return &cosmos.Result{}, nil
		}
		emit = swp.Out
	}

	// This is a preferred asset swap, so subtract the affiliate's CACAO from the
	// AffiliateCollector module, and send CACAO from the module to Asgard. Then return
	// early since there is no need to call any downstream handlers.
	if strings.HasPrefix(msg.Tx.Memo, "MAYA-PREFERRED-ASSET") && msg.Tx.FromAddress.Equals(affColAddress) {
		err = h.processPreferredAssetSwap(ctx, msg)
		// Failed to update the AffiliateCollector / return err to revert preferred asset swap
		if err != nil {
			ctx.Logger().Error("failed to update affiliate collector", "error", err)
			return &cosmos.Result{}, err
		}
		return &cosmos.Result{}, nil
	}

	mem, parseMemoErr := ParseMemoWithMAYANames(ctx, h.mgr.Keeper(), msg.Tx.Memo)
	if parseMemoErr != nil {
		ctx.Logger().Error("swap handler failed to parse memo", "memo", msg.Tx.Memo, "error", err)
		return nil, err
	}
	if mem.IsType(TxAdd) {
		m, ok := mem.(AddLiquidityMemo)
		if !ok {
			return nil, fmt.Errorf("fail to cast add liquidity memo")
		}
		m.Asset = fuzzyAssetMatch(ctx, h.mgr.Keeper(), m.Asset)
		msg.Tx.Coins = common.NewCoins(common.NewCoin(m.Asset, emit))
		obTx := ObservedTx{Tx: msg.Tx}
		msg, err := getMsgAddLiquidityFromMemo(ctx, m, obTx, msg.Signer, 0)
		if err != nil {
			return nil, err
		}
		handler := NewAddLiquidityHandler(h.mgr)
		_, err = handler.Run(ctx, msg)
		if err != nil {
			ctx.Logger().Error("swap handler failed to add liquidity", "error", err)
			return nil, err
		}
	}

	return &cosmos.Result{}, nil
}

// processPreferredAssetSwap - after a preferred asset swap, deduct the input CACAO
// amount from AffiliateCollector module accounting and send appropriate amount of CACAO
// from AffiliateCollector module to Asgard
func (h SwapHandler) processPreferredAssetSwap(ctx cosmos.Context, msg MsgSwap) error {
	if msg.Tx.Coins.IsEmpty() || !msg.Tx.Coins[0].Asset.IsNativeBase() {
		return fmt.Errorf("native CACAO not in coins: %s", msg.Tx.Coins)
	}
	// For preferred asset swaps, the signer of the Msg is the MAYAName owner
	affCol, err := h.mgr.Keeper().GetAffiliateCollector(ctx, msg.Signer)
	if err != nil {
		return err
	}

	cacaoCoin := msg.Tx.Coins[0]
	cacaoAmt := cacaoCoin.Amount

	if affCol.CacaoAmount.LT(cacaoAmt) {
		return fmt.Errorf("not enough affiliate collector balance for preferred asset swap, balance: %s, needed: %s", affCol.CacaoAmount.String(), cacaoAmt.String())
	}

	// 1. Send CACAO from the AffiliateCollector Module to Asgard for the swap
	err = h.mgr.Keeper().SendFromModuleToModule(ctx, AffiliateCollectorName, AsgardName, common.NewCoins(cacaoCoin))
	if err != nil {
		return err
	}
	// 2. Subtract input CACAO amt from AffiliateCollector accounting
	affCol.CacaoAmount = affCol.CacaoAmount.Sub(cacaoAmt)
	h.mgr.Keeper().SetAffiliateCollector(ctx, affCol)

	return nil
}

func (h SwapHandler) validateV112(ctx cosmos.Context, msg MsgSwap) error {
	if err := msg.ValidateBasicV112(h.mgr.GetVersion()); err != nil {
		return err
	}

	target := msg.TargetAsset
	if isTradingHalt(ctx, &msg, h.mgr) {
		return errors.New("trading is halted, can't process swap")
	}
	maxAffiliateFeeBasisPoints := uint64(h.mgr.Keeper().GetConfigInt64(ctx, constants.MaxAffiliateFeeBasisPoints))
	// if AffiliateBasisPoints provided, it must not be greater than MaxAffiliateFeeBasisPoints
	if !msg.AffiliateBasisPoints.Equal(EmptyBps) && msg.AffiliateBasisPoints.GT(cosmos.NewUint(maxAffiliateFeeBasisPoints)) {
		return fmt.Errorf("affiliate fee basis points must not exceed %d", maxAffiliateFeeBasisPoints)
	}

	// For external-origin (here valid) memos, do not allow a network module as the final destination.
	// If unable to parse the memo, here assume it to be internal.
	memo, _ := ParseMemoWithMAYANames(ctx, h.mgr.Keeper(), msg.Tx.Memo)
	mem, isSwapMemo := memo.(SwapMemo)
	if isSwapMemo {
		destAccAddr, err := mem.Destination.AccAddress()
		// A network module address would be resolvable,
		// so if not resolvable it should not be a network module address.
		if err == nil && IsModuleAccAddress(h.mgr.Keeper(), destAccAddr) {
			return fmt.Errorf("a network module cannot be the final destination of a swap memo")
		}
	}

	if isLiquidityAuction(ctx, h.mgr.Keeper()) {
		return errors.New("liquidity auction is in progress, can't process swap")
	}

	if len(msg.Aggregator) > 0 {
		swapOutDisabled := h.mgr.Keeper().GetConfigInt64(ctx, constants.SwapOutDexAggregationDisabled)
		if swapOutDisabled > 0 {
			return errors.New("swap out dex integration disabled")
		}
		if !msg.TargetAsset.Equals(msg.TargetAsset.Chain.GetGasAsset()) {
			return fmt.Errorf("target asset (%s) is not gas asset , can't use dex feature", msg.TargetAsset)
		}
		// validate that a referenced dex aggregator is legit
		addr, err := FetchDexAggregator(h.mgr.GetVersion(), target.Chain, msg.Aggregator)
		if err != nil {
			return err
		}
		if addr == "" {
			return fmt.Errorf("aggregator address is empty")
		}
		if len(msg.AggregatorTargetAddress) == 0 {
			return fmt.Errorf("aggregator target address is empty")
		}
	}

	if target.IsSyntheticAsset() && target.GetLayer1Asset().IsNative() {
		return errors.New("minting a synthetic of a native coin is not allowed")
	}

	var sourceCoin common.Coin
	if len(msg.Tx.Coins) > 0 {
		sourceCoin = msg.Tx.Coins[0]
	}

	if msg.IsStreaming() {
		pausedStreaming := h.mgr.Keeper().GetConfigInt64(ctx, constants.StreamingSwapPause)
		if pausedStreaming > 0 {
			return fmt.Errorf("streaming swaps are paused")
		}

		// if either source or target in ragnarok, streaming is not allowed
		for _, asset := range []common.Asset{sourceCoin.Asset, target} {
			key := "RAGNAROK-" + asset.MimirString()
			ragnarok, err := h.mgr.Keeper().GetMimir(ctx, key)
			if err == nil && ragnarok > 0 {
				return fmt.Errorf("streaming swaps disabled on ragnarok asset %s", asset)
			}
		}

		swp := msg.GetStreamingSwap()
		if h.mgr.Keeper().StreamingSwapExists(ctx, msg.Tx.ID) {
			var err error
			swp, err = h.mgr.Keeper().GetStreamingSwap(ctx, msg.Tx.ID)
			if err != nil {
				ctx.Logger().Error("fail to fetch streaming swap", "error", err)
				return err
			}
		}

		if (swp.Quantity > 0 && swp.IsDone()) || swp.In.GTE(swp.Deposit) {
			// check both swap count and swap in vs deposit to cover all basis
			return fmt.Errorf("streaming swap is completed, cannot continue to swap again")
		}

		if swp.Count > 0 {
			// end validation early, as synth TVL caps are not applied to streaming
			// swaps. This is to ensure that streaming swaps don't get interrupted
			// and cause a partial fulfillment, which would cause issues for
			// internal streaming swaps for savers and loans.
			return nil
		} else {
			// first swap we check the entire swap amount (not just the
			// sub-swap amount) to ensure the value of the entire has TVL/synth
			// room
			sourceCoin.Amount = swp.Deposit
		}
	}

	if target.IsSyntheticAsset() {
		// the following is only applicable for mainnet
		totalLiquidityCACAO, err := h.getTotalLiquidityRUNE(ctx)
		if err != nil {
			return ErrInternal(err, "fail to get total liquidity RUNE")
		}

		// total liquidity RUNE after current add liquidity
		if len(msg.Tx.Coins) > 0 {
			// calculate rune value on incoming swap, and add to total liquidity.
			runeVal := sourceCoin.Amount
			if !sourceCoin.Asset.IsBase() {
				var pool Pool
				pool, err = h.mgr.Keeper().GetPool(ctx, sourceCoin.Asset.GetLayer1Asset())
				if err != nil {
					return ErrInternal(err, "fail to get pool")
				}
				runeVal = pool.AssetValueInRune(sourceCoin.Amount)
			}
			totalLiquidityCACAO = totalLiquidityCACAO.Add(runeVal)
		}
		maximumLiquidityRune, err := h.mgr.Keeper().GetMimir(ctx, constants.MaximumLiquidityCacao.String())
		if maximumLiquidityRune < 0 || err != nil {
			maximumLiquidityRune = h.mgr.GetConstants().GetInt64Value(constants.MaximumLiquidityCacao)
		}
		if maximumLiquidityRune > 0 {
			if totalLiquidityCACAO.GT(cosmos.NewUint(uint64(maximumLiquidityRune))) {
				return errAddLiquidityRUNEOverLimit
			}
		}

		// fail validation if synth supply is already too high, relative to pool depth
		// do a simulated swap to see how much of the target synth the network
		// will need to mint and check if that amount exceeds limits
		targetAmount, cacaoAmount := cosmos.ZeroUint(), cosmos.ZeroUint()
		swapper, err := GetSwapper(h.mgr.GetVersion())
		if err == nil {
			if sourceCoin.Asset.IsBase() {
				cacaoAmount = sourceCoin.Amount
			} else {
				// asset --> rune swap
				sourceAssetPool := sourceCoin.Asset
				if sourceAssetPool.IsSyntheticAsset() {
					sourceAssetPool = sourceAssetPool.GetLayer1Asset()
				}
				var sourcePool Pool
				sourcePool, err = h.mgr.Keeper().GetPool(ctx, sourceAssetPool)
				if err != nil {
					ctx.Logger().Error("fail to fetch pool for swap simulation", "error", err)
				} else {
					cacaoAmount = swapper.CalcAssetEmission(sourcePool.BalanceAsset, sourceCoin.Amount, sourcePool.BalanceCacao)
				}
			}
			// rune --> synth swap
			var targetPool Pool
			targetPool, err = h.mgr.Keeper().GetPool(ctx, target.GetLayer1Asset())
			if err != nil {
				ctx.Logger().Error("fail to fetch pool for swap simulation", "error", err)
			} else {
				targetAmount = swapper.CalcAssetEmission(targetPool.BalanceCacao, cacaoAmount, targetPool.BalanceAsset)
			}
		}

		err = isSynthMintPaused(ctx, h.mgr, target, targetAmount)
		if err != nil {
			return err
		}

		ensureLiquidityNoLargerThanBondInt64, err := h.mgr.Keeper().GetMimir(ctx, "EnsureLiquidityNoLargerThanBond")
		if err != nil {
			ctx.Logger().Error("fail to get mimir", "error", err)
		}
		// 0 not active, 1 active, -1 use default
		var ensureLiquidityNoLargerThanBond bool
		if ensureLiquidityNoLargerThanBondInt64 < 0 {
			ensureLiquidityNoLargerThanBond = h.mgr.GetConstants().GetBoolValue(constants.StrictBondLiquidityRatio)
		} else {
			ensureLiquidityNoLargerThanBond = ensureLiquidityNoLargerThanBondInt64 == 1
		}

		// If source and target are synthetic assets there is no net liquidity gain (RUNE is just moved from pool A to pool B),
		// so skip this check
		if ensureLiquidityNoLargerThanBond {
			if !sourceCoin.Asset.IsSyntheticAsset() && atTVLCap(ctx, common.NewCoins(sourceCoin), h.mgr) {
				return errAddLiquidityCACAOMoreThanBond
			}
		}
	}

	return nil
}
