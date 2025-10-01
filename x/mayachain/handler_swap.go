package mayachain

import (
	"errors"
	"fmt"
	"strings"

	"github.com/blang/semver"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

// SwapHandler is the handler to process swap request
type SwapHandler struct {
	mgr Manager
}

// NewSwapHandler create a new instance of swap handler
func NewSwapHandler(mgr Manager) SwapHandler {
	return SwapHandler{
		mgr: mgr,
	}
}

// Run is the main entry point of swap message
func (h SwapHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgSwap)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("MsgSwap failed validation", "error", err)
		return nil, err
	}
	result, err := h.handle(ctx, *msg)
	if err != nil {
		ctx.Logger().Error("fail to handle MsgSwap", "error", err)
		return nil, err
	}
	return result, err
}

func (h SwapHandler) validate(ctx cosmos.Context, msg MsgSwap) error {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.123.0")): // trade-accounts
		return h.validateV123(ctx, msg)
	case version.GTE(semver.MustParse("1.112.0")):
		return h.validateV112(ctx, msg)
	case version.GTE(semver.MustParse("1.110.0")):
		return h.validateV110(ctx, msg)
	case version.GTE(semver.MustParse("1.108.0")):
		return h.validateV108(ctx, msg)
	case version.GTE(semver.MustParse("1.101.0")):
		return h.validateV101(ctx, msg)
	case version.GTE(semver.MustParse("1.95.0")):
		return h.validateV95(ctx, msg)
	default:
		return errInvalidVersion
	}
}

func (h SwapHandler) validateV123(ctx cosmos.Context, msg MsgSwap) error {
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

		if target.IsSyntheticAsset() && h.mgr.Keeper().GetConfigInt64(ctx, constants.ManualSwapsToSynthDisabled) > 0 {
			// Reject manual swap attempts for minting synths (encouraging Trade Assets for manual swaps),
			// allowing synth minting only in other contexts like with add liquidity memos (Savers) or internal memos.
			return fmt.Errorf("manual swaps to synths not supported, use trade assets instead")
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

	if target.IsTradeAsset() && target.GetLayer1Asset().IsNative() {
		return errors.New("swapping to a trade asset of a native coin is not allowed")
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

func (h SwapHandler) handle(ctx cosmos.Context, msg MsgSwap) (*cosmos.Result, error) {
	ctx.Logger().Info("receive MsgSwap", "request tx hash", msg.Tx.ID, "source asset", msg.Tx.Coins[0].Asset, "target asset", msg.TargetAsset, "signer", msg.Signer.String())
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.118.0")):
		return h.handleV118(ctx, msg)
	case version.GTE(semver.MustParse("1.112.0")):
		return h.handleV112(ctx, msg)
	case version.GTE(semver.MustParse("1.110.0")):
		return h.handleV110(ctx, msg)
	case version.GTE(semver.MustParse("1.108.0")):
		return h.handleV108(ctx, msg)
	case version.GTE(semver.MustParse("1.95.0")):
		return h.handleV95(ctx, msg)
	default:
		return nil, errBadVersion
	}
}

func (h SwapHandler) handleV118(ctx cosmos.Context, msg MsgSwap) (*cosmos.Result, error) {
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
			maxLength := h.mgr.Keeper().GetConfigInt64(ctx, constants.StreamingSwapMaxLength)
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

	mem, parseMemoErr := ParseMemoWithMAYANames(ctx, h.mgr.Keeper(), msg.Tx.Memo)

	// process the affiliate swap to affiliate collector
	if msg.Destination.Equals(affColAddress) {
		var mayaname MAYAName
		if parseMemoErr == nil {
			affs := mem.GetAffiliates()
			if len(affs) > 0 {
				mayaname, err = h.mgr.Keeper().GetMAYAName(ctx, affs[0])
			} else {
				err = fmt.Errorf("failed to process swap to affiliate collector, affiliate MAYAName not provided in memo")
			}
		}
		if err == nil && !msg.TargetAsset.IsNativeBase() {
			err = fmt.Errorf("failed to process swap to affiliate collector, swap target asset is %s, expected CACAO", msg.TargetAsset)
		}
		if err != nil {
			return nil, err
		}
		transactionFee := h.mgr.GasMgr().GetFee(ctx, common.BASEChain, common.BaseAsset())
		addCacaoAmt := common.SafeSub(emit, transactionFee)
		swapIndex := 0
		err = updateAffiliateCollector(ctx, h.mgr, addCacaoAmt, mayaname, &swapIndex)
		if err != nil {
			return &cosmos.Result{}, fmt.Errorf("failed to update affiliate collector, err: %w", err)
		}

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

	// this is a preferred asset swap, so return early since there is no need to call any
	// downstream handlers
	if strings.HasPrefix(msg.Tx.Memo, PreferredAssetSwapMemoPrefix) && msg.Tx.FromAddress.Equals(affColAddress) {
		return &cosmos.Result{}, nil
	}

	if parseMemoErr != nil {
		ctx.Logger().Error("swap handler failed to parse memo", "memo", msg.Tx.Memo, "error", parseMemoErr)
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

// get the total bond of the bottom 2/3rds active validators
func (h SwapHandler) getEffectiveSecurityBond(ctx cosmos.Context, mgr Manager) (cosmos.Uint, error) {
	nodeAccounts, err := h.mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return cosmos.ZeroUint(), err
	}
	return getEffectiveSecurityBond(ctx, mgr, nodeAccounts), nil
}

// getTotalLiquidityRUNE we have in all pools
func (h SwapHandler) getTotalLiquidityRUNE(ctx cosmos.Context) (cosmos.Uint, error) {
	pools, err := h.mgr.Keeper().GetPools(ctx)
	if err != nil {
		return cosmos.ZeroUint(), fmt.Errorf("fail to get pools from data store: %w", err)
	}
	total := cosmos.ZeroUint()
	for _, p := range pools {
		// ignore suspended pools
		if p.Status == PoolSuspended {
			continue
		}
		if p.Asset.IsNative() {
			continue
		}
		total = total.Add(p.BalanceCacao)
	}
	return total, nil
}
