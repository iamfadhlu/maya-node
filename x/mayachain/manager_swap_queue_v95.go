package mayachain

import (
	"strconv"
	"strings"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
)

// SwapQueueV95 is going to manage the swaps queue
type SwapQueueV95 struct {
	k keeper.Keeper
}

// newSwapQueueV95 create a new vault manager
func newSwapQueueV95(k keeper.Keeper) *SwapQueueV95 {
	return &SwapQueueV95{k: k}
}

// FetchQueue - grabs all swap queue items from the kvstore and returns them
func (vm *SwapQueueV95) FetchQueue(ctx cosmos.Context) (swapItems, error) { // nolint
	items := make(swapItems, 0)
	iterator := vm.k.GetSwapQueueIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var msg MsgSwap
		if err := vm.k.Cdc().Unmarshal(iterator.Value(), &msg); err != nil {
			ctx.Logger().Error("fail to fetch swap msg from queue", "error", err)
			continue
		}

		ss := strings.Split(string(iterator.Key()), "-")
		i, err := strconv.Atoi(ss[len(ss)-1])
		if err != nil {
			ctx.Logger().Error("fail to parse swap queue msg index", "key", iterator.Key(), "error", err)
			continue
		}

		items = append(items, swapItem{
			msg:   msg,
			index: i,
			fee:   cosmos.ZeroUint(),
			slip:  cosmos.ZeroUint(),
		})
	}

	return items, nil
}

// EndBlock trigger the real swap to be processed
func (vm *SwapQueueV95) EndBlock(ctx cosmos.Context, mgr Manager) error {
	handler := NewInternalHandler(mgr)

	minSwapsPerBlock, err := vm.k.GetMimir(ctx, constants.MinSwapsPerBlock.String())
	if minSwapsPerBlock < 0 || err != nil {
		minSwapsPerBlock = mgr.GetConstants().GetInt64Value(constants.MinSwapsPerBlock)
	}
	maxSwapsPerBlock, err := vm.k.GetMimir(ctx, constants.MaxSwapsPerBlock.String())
	if maxSwapsPerBlock < 0 || err != nil {
		maxSwapsPerBlock = mgr.GetConstants().GetInt64Value(constants.MaxSwapsPerBlock)
	}
	synthVirtualDepthMult, err := vm.k.GetMimir(ctx, constants.VirtualMultSynthsBasisPoints.String())
	if synthVirtualDepthMult < 1 || err != nil {
		synthVirtualDepthMult = mgr.GetConstants().GetInt64Value(constants.VirtualMultSynthsBasisPoints)
	}

	swaps, err := vm.FetchQueue(ctx)
	if err != nil {
		ctx.Logger().Error("fail to fetch swap queue from store", "error", err)
		return err
	}
	swaps, err = vm.scoreMsgs(ctx, swaps, synthVirtualDepthMult, mgr)
	if err != nil {
		ctx.Logger().Error("fail to fetch swap items", "error", err)
		// continue, don't exit, just do them out of order (instead of not at all)
	}
	swaps = swaps.Sort()

	for i := int64(0); i < vm.getTodoNum(int64(len(swaps)), minSwapsPerBlock, maxSwapsPerBlock); i++ {
		pick := swaps[i]
		_, err := handler(ctx, &pick.msg)
		if err != nil {
			ctx.Logger().Error("fail to swap", "msg", pick.msg.Tx.String(), "error", err)
			if newErr := refundTx(ctx, ObservedTx{Tx: pick.msg.Tx}, mgr, CodeSwapFail, err.Error(), ""); nil != newErr {
				ctx.Logger().Error("fail to refund swap", "error", err)
			}
		}
		vm.k.RemoveSwapQueueItem(ctx, pick.msg.Tx.ID, pick.index)
	}
	return nil
}

// getTodoNum - determine how many swaps to do.
func (vm *SwapQueueV95) getTodoNum(queueLen, minSwapsPerBlock, maxSwapsPerBlock int64) int64 {
	// Do half the length of the queue. Unless...
	//	1. The queue length is greater than maxSwapsPerBlock
	//  2. The queue length is less than minSwapsPerBlock
	todo := queueLen / 2
	if minSwapsPerBlock >= queueLen {
		todo = queueLen
	}
	if maxSwapsPerBlock < todo {
		todo = maxSwapsPerBlock
	}
	return todo
}

// scoreMsgs - this takes a list of MsgSwap, and converts them to a scored
// swapItem list
func (vm *SwapQueueV95) scoreMsgs(ctx cosmos.Context, items swapItems, synthVirtualDepthMult int64, mgr Manager) (swapItems, error) {
	pools := make(map[common.Asset]Pool)

	for i, item := range items {
		// the asset customer send
		sourceAsset := item.msg.Tx.Coins[0].Asset
		// the asset customer want
		targetAsset := item.msg.TargetAsset

		assets := common.Assets{sourceAsset, targetAsset}
		for _, a := range assets {
			if a.IsBase() {
				continue
			}

			if _, ok := pools[a]; !ok {
				var err error
				pools[a], err = vm.k.GetPool(ctx, a.GetLayer1Asset())
				if err != nil {
					ctx.Logger().Error("fail to get pool", "pool", a, "error", err)
					continue
				}
			}
		}

		nonBaseAsset := sourceAsset
		if nonBaseAsset.IsBase() {
			nonBaseAsset = targetAsset
		}
		pool := pools[nonBaseAsset]
		if pool.IsEmpty() || pool.BalanceCacao.IsZero() || pool.BalanceAsset.IsZero() {
			continue
		}
		// synths may be redeemed on unavailable pools, score them
		if !pool.IsAvailable() && !sourceAsset.IsSyntheticAsset() {
			continue
		}
		virtualDepthMult := int64(10_000)
		if nonBaseAsset.IsSyntheticAsset() {
			virtualDepthMult = synthVirtualDepthMult
		}
		vm.getLiquidityFeeAndSlip(ctx, pool, item.msg.Tx.Coins[0], &items[i], virtualDepthMult, mgr)

		if sourceAsset.IsBase() || targetAsset.IsBase() {
			// single swap , stop here
			continue
		}
		// double swap , thus need to convert source coin to RUNE and calculate fee and slip again
		runeCoin := common.NewCoin(common.BaseAsset(), pool.AssetValueInRune(item.msg.Tx.Coins[0].Amount))
		nonBaseAsset = targetAsset
		pool = pools[nonBaseAsset]
		if pool.IsEmpty() || !pool.IsAvailable() || pool.BalanceCacao.IsZero() || pool.BalanceAsset.IsZero() {
			continue
		}
		virtualDepthMult = int64(10_000)
		if targetAsset.IsSyntheticAsset() {
			virtualDepthMult = synthVirtualDepthMult
		}
		vm.getLiquidityFeeAndSlip(ctx, pool, runeCoin, &items[i], virtualDepthMult, mgr)
	}

	return items, nil
}

// getLiquidityFeeAndSlip calculate liquidity fee and slip, fee is in RUNE
func (vm *SwapQueueV95) getLiquidityFeeAndSlip(ctx cosmos.Context, pool Pool, sourceCoin common.Coin, item *swapItem, virtualDepthMult int64, mgr Manager) {
	// Get our X, x, Y values
	var X, x, Y cosmos.Uint
	x = sourceCoin.Amount
	if sourceCoin.Asset.IsBase() {
		X = pool.BalanceCacao
		Y = pool.BalanceAsset
	} else {
		Y = pool.BalanceCacao
		X = pool.BalanceAsset
	}

	X = common.GetUncappedShare(cosmos.NewUint(uint64(virtualDepthMult)), cosmos.NewUint(10_000), X)
	Y = common.GetUncappedShare(cosmos.NewUint(uint64(virtualDepthMult)), cosmos.NewUint(10_000), Y)

	swapper, err := GetSwapper(vm.k.GetVersion())
	if err != nil {
		ctx.Logger().Error("fail to fetch swapper", "error", err)
		swapper = newSwapperV95()
	}
	fee := swapper.CalcLiquidityFee(X, x, Y)
	if sourceCoin.Asset.IsBase() {
		fee = pool.AssetValueInRune(fee)
	}

	slipFeeAddedBasisPoints := getSlipFeeAddedBasisPoints(ctx, mgr)

	slip := swapper.CalcSwapSlip(X, x, cosmos.NewUint(slipFeeAddedBasisPoints))
	item.fee = item.fee.Add(fee)
	item.slip = item.slip.Add(slip)
}
