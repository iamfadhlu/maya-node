package keeperv1

import (
	"fmt"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

// InvariantRoutes return the keeper's invariant routes
func (k KVStore) InvariantRoutes() []common.InvariantRoute {
	return []common.InvariantRoute{
		// common.NewInvariantRoute("asgard", AsgardInvariant(k)),
		// common.NewInvariantRoute("node_rewards", NodeRewardsInvariant(k)),
		common.NewInvariantRoute("mayachain", MAYAChainInvariant(k)),
		common.NewInvariantRoute("affiliate_collector", AffiliateCollectorInvariant(k)),
		common.NewInvariantRoute("pools", PoolsInvariant(k)),
		common.NewInvariantRoute("streaming_swaps", StreamingSwapsInvariant(k)),
	}
}

// AsgardInvariant the asgard module backs pool rune, savers synths, and native
// coins in queued swaps
func AsgardInvariant(k KVStore) common.Invariant {
	return func(ctx cosmos.Context) (msg []string, broken bool) {
		// sum all rune liquidity on pools, including pending
		var poolCoins common.Coins
		pools, _ := k.GetPools(ctx)
		for _, pool := range pools {
			switch {
			case pool.Asset.IsSyntheticAsset():
				coin := common.NewCoin(
					pool.Asset,
					pool.BalanceAsset,
				)
				poolCoins = poolCoins.Add(coin)
				// case !pool.Asset.IsDerivedAsset():
			default:
				coin := common.NewCoin(
					common.BaseAsset(),
					pool.BalanceCacao.Add(pool.PendingInboundCacao),
				)
				poolCoins = poolCoins.Add(coin)
			}
		}

		// sum all rune in pending swaps
		var swapCoins common.Coins
		swapIter := k.GetSwapQueueIterator(ctx)
		defer swapIter.Close()
		for ; swapIter.Valid(); swapIter.Next() {
			var swap MsgSwap
			k.Cdc().MustUnmarshal(swapIter.Value(), &swap)

			if len(swap.Tx.Coins) != 1 {
				broken = true
				msg = append(msg, fmt.Sprintf("wrong number of coins for swap: %d, %s", len(swap.Tx.Coins), swap.Tx.ID))
				continue
			}

			coin := swap.Tx.Coins[0]
			if !coin.IsNative() && !swap.TargetAsset.IsNative() {
				continue // only verifying native coins in this invariant
			}

			// adjust for streaming swaps
			ss := swap.GetStreamingSwap() // GetStreamingSwap() rather than var so In.IsZero() doesn't panic
			// A non-streaming affiliate swap and streaming main swap could have the same TxID,
			// so explicitly check IsStreaming to not double-count the main swap's In and Out amounts.
			if swap.IsStreaming() {
				var err error
				ss, err = k.GetStreamingSwap(ctx, swap.Tx.ID)
				if err != nil {
					ctx.Logger().Error("error getting streaming swap", "error", err)
					continue // should never happen
				}
			}

			// Trade Assets do not correspond to Module balance coins and panic on .Native(),
			// so do not include them in swapCoins.
			if coin.IsNative() && !coin.Asset.IsTradeAsset() {
				if !ss.In.IsZero() {
					// adjust for stream swap amount, the amount In has been added
					// to the pool but not deducted from the tx or module, so deduct
					// that In amount from the tx coin
					coin.Amount = coin.Amount.Sub(ss.In)
				}
				swapCoins = swapCoins.Add(coin)
			}

			if swap.TargetAsset.IsNative() && !swap.TargetAsset.IsTradeAsset() && !ss.Out.IsZero() {
				swapCoins = swapCoins.Add(common.NewCoin(swap.TargetAsset, ss.Out))
			}
		}

		// get asgard module balance
		asgardAddr := k.GetModuleAccAddress(AsgardName)
		asgardCoins := k.GetBalance(ctx, asgardAddr)

		// asgard balance is expected to equal sum of pool and swap coins
		expNative, _ := poolCoins.Adds_deprecated(swapCoins).Native()

		// note: coins must be sorted for SafeSub
		diffCoins, _ := asgardCoins.SafeSub(expNative.Sort())
		if !diffCoins.IsZero() {
			broken = true
			for _, coin := range diffCoins {
				if coin.IsPositive() {
					msg = append(msg, fmt.Sprintf("oversolvent: %s", coin))
				} else {
					coin.Amount = coin.Amount.Neg()
					msg = append(msg, fmt.Sprintf("insolvent: %s", coin))
				}
			}
		}

		return msg, broken
	}
}

// NodeRewardsInvariant the bond module backs node bond and pending reward bond
func NodeRewardsInvariant(k KVStore) common.Invariant {
	return func(ctx cosmos.Context) (msg []string, broken bool) {
		// get pending bond reward cacao
		network, _ := k.GetNetwork(ctx)
		expectedCacao := network.BondRewardRune

		// get rune balance of bond module
		bondModuleCacao := k.GetBalanceOfModule(ctx, BondName, common.BaseAsset().Native())

		// bond module is expected to equal pending rewards
		if expectedCacao.GT(bondModuleCacao) {
			broken = true
			diff := expectedCacao.Sub(bondModuleCacao)
			coin, _ := common.NewCoin(common.BaseAsset(), diff).Native()
			msg = append(msg, fmt.Sprintf("insolvent: %s", coin))

		} else if expectedCacao.LT(bondModuleCacao) {
			broken = true
			diff := bondModuleCacao.Sub(expectedCacao)
			coin, _ := common.NewCoin(common.BaseAsset(), diff).Native()
			msg = append(msg, fmt.Sprintf("oversolvent: %s", coin))
		}

		return msg, broken
	}
}

// MAYAChainInvariant the thorchain module should never hold a balance
func MAYAChainInvariant(k KVStore) common.Invariant {
	return func(ctx cosmos.Context) (msg []string, broken bool) {
		// module balance of theorchain
		tcAddr := k.GetModuleAccAddress(ModuleName)
		tcCoins := k.GetBalance(ctx, tcAddr)

		// thorchain module should never carry a balance
		if !tcCoins.Empty() {
			broken = true
			for _, coin := range tcCoins {
				msg = append(msg, fmt.Sprintf("oversolvent: %s", coin))
			}
		}

		return msg, broken
	}
}

// PoolsInvariant pool units and pending rune/asset should match the sum
// of units and pending rune/asset for all lps
func PoolsInvariant(k KVStore) common.Invariant {
	return func(ctx cosmos.Context) (msg []string, broken bool) {
		pools, _ := k.GetPools(ctx)
		for _, pool := range pools {
			if pool.Asset.IsNative() {
				continue // only looking at layer-one pools
			}

			lpUnits := cosmos.ZeroUint()
			lpPendingCacao := cosmos.ZeroUint()
			lpPendingAsset := cosmos.ZeroUint()

			lpIter := k.GetLiquidityProviderIterator(ctx, pool.Asset)
			defer lpIter.Close()
			for ; lpIter.Valid(); lpIter.Next() {
				var lp LiquidityProvider
				k.Cdc().MustUnmarshal(lpIter.Value(), &lp)
				lpUnits = lpUnits.Add(lp.Units)
				lpPendingCacao = lpPendingCacao.Add(lp.PendingCacao)
				lpPendingAsset = lpPendingAsset.Add(lp.PendingAsset)
			}

			check := func(poolValue, lpValue cosmos.Uint, valueType string) {
				if poolValue.GT(lpValue) {
					diff := poolValue.Sub(lpValue)
					msg = append(msg, fmt.Sprintf("%s oversolvent: %s %s", pool.Asset, diff.String(), valueType))
					broken = true
				} else if poolValue.LT(lpValue) {
					diff := lpValue.Sub(poolValue)
					msg = append(msg, fmt.Sprintf("%s insolvent: %s %s", pool.Asset, diff.String(), valueType))
					broken = true
				}
			}

			check(pool.LPUnits, lpUnits, "units")
			check(pool.PendingInboundCacao, lpPendingCacao, "pending cacao")
			check(pool.PendingInboundAsset, lpPendingAsset, "pending asset")
		}

		return msg, broken
	}
}

// StreamingSwapsInvariant every streaming swap should have a corresponding
// queued swap, stream deposit should equal the queued swap's source coin,
// and the stream should be internally consistent
func StreamingSwapsInvariant(k KVStore) common.Invariant {
	return func(ctx cosmos.Context) (msg []string, broken bool) {
		// fetch all streaming swaps from the swap queue
		var swaps []MsgSwap
		swapIter := k.GetSwapQueueIterator(ctx)
		defer swapIter.Close()
		for ; swapIter.Valid(); swapIter.Next() {
			var swap MsgSwap
			k.Cdc().MustUnmarshal(swapIter.Value(), &swap)
			if swap.IsStreaming() {
				swaps = append(swaps, swap)
			}
		}

		// fetch all stream swap records
		var streams []StreamingSwap
		ssIter := k.GetStreamingSwapIterator(ctx)
		defer ssIter.Close()
		for ; ssIter.Valid(); ssIter.Next() {
			var stream StreamingSwap
			k.Cdc().MustUnmarshal(ssIter.Value(), &stream)
			streams = append(streams, stream)
		}

		for _, stream := range streams {
			found := false
			for _, swap := range swaps {
				if !swap.Tx.ID.Equals(stream.TxID) {
					continue
				}
				found = true
				if !swap.Tx.Coins[0].Amount.Equal(stream.Deposit) {
					broken = true
					msg = append(msg, fmt.Sprintf(
						"%s: swap.coin %s != stream.deposit %s",
						stream.TxID.String(),
						swap.Tx.Coins[0].Amount,
						stream.Deposit.String()))
				}
				if stream.Count > stream.Quantity {
					broken = true
					msg = append(msg, fmt.Sprintf(
						"%s: stream.count %d > stream.quantity %d",
						stream.TxID.String(),
						stream.Count,
						stream.Quantity))
				}
				if stream.In.GT(stream.Deposit) {
					broken = true
					msg = append(msg, fmt.Sprintf(
						"%s: stream.in %s > stream.deposit %s",
						stream.TxID.String(),
						stream.In.String(),
						stream.Deposit.String()))
				}
			}
			if !found {
				broken = true
				msg = append(msg, fmt.Sprintf("swap not found for stream: %s", stream.TxID.String()))
			}
		}

		return msg, broken
	}
}

// AffiliateCollectorInvariant the affiliate_collector module backs accrued affiliate
// rewards
func AffiliateCollectorInvariant(k KVStore) common.Invariant {
	return func(ctx cosmos.Context) (msg []string, broken bool) {
		affColModuleCacao := k.GetBalanceOfModule(ctx, AffiliateCollectorName, common.BaseAsset().Native())
		affCols, err := k.GetAffiliateCollectors(ctx)
		if err != nil {
			if affColModuleCacao.IsZero() {
				return nil, false
			}
			msg = append(msg, err.Error())
			return msg, true
		}

		totalAffCacao := cosmos.ZeroUint()
		for _, ac := range affCols {
			totalAffCacao = totalAffCacao.Add(ac.CacaoAmount)
		}

		if totalAffCacao.GT(affColModuleCacao) {
			broken = true
			diff := totalAffCacao.Sub(affColModuleCacao)
			coin, _ := common.NewCoin(common.BaseAsset(), diff).Native()
			msg = append(msg, fmt.Sprintf("insolvent: %s", coin))
		} else if totalAffCacao.LT(affColModuleCacao) {
			broken = true
			diff := affColModuleCacao.Sub(totalAffCacao)
			coin, _ := common.NewCoin(common.BaseAsset(), diff).Native()
			msg = append(msg, fmt.Sprintf("oversolvent: %s", coin))
		}

		return msg, broken
	}
}

// CACAOPoolInvariant asserts that the CACAOPool units and provider units are consistent.
func CACAOPoolInvariant(k KVStore) common.Invariant {
	return func(ctx cosmos.Context) (msg []string, broken bool) {
		cacaoPool, err := k.GetCACAOPool(ctx)
		if err != nil {
			ctx.Logger().Error("error getting cacao pool", "error", err)
			return []string{err.Error()}, true
		}

		providerUnits := cosmos.ZeroUint()
		providerDeposited := cosmos.ZeroUint()
		providerWithdrawn := cosmos.ZeroUint()
		iterator := k.GetCACAOProviderIterator(ctx)
		defer iterator.Close()
		for ; iterator.Valid(); iterator.Next() {
			var rp CACAOProvider
			k.Cdc().MustUnmarshal(iterator.Value(), &rp)
			if rp.CacaoAddress.Empty() {
				continue
			}
			providerUnits = providerUnits.Add(rp.Units)
			providerDeposited = providerDeposited.Add(rp.DepositAmount)
			providerWithdrawn = providerWithdrawn.Add(rp.WithdrawAmount)
		}

		if !providerUnits.Equal(cacaoPool.PoolUnits) {
			m := fmt.Sprintf(
				"pool units %s != provider units %s",
				cacaoPool.PoolUnits, providerUnits,
			)
			msg = append(msg, m)
			broken = true
		}

		if !providerDeposited.Equal(cacaoPool.CacaoDeposited) {
			m := fmt.Sprintf(
				"cacao deposited %s != provider cacao deposited %s",
				cacaoPool.CacaoDeposited, providerDeposited,
			)
			msg = append(msg, m)
			broken = true
		}

		if !providerWithdrawn.Equal(cacaoPool.CacaoWithdrawn) {
			m := fmt.Sprintf(
				"cacao withdrawn %s != provider cacao withdrawn %s",
				cacaoPool.CacaoWithdrawn, providerWithdrawn,
			)
			msg = append(msg, m)
			broken = true
		}

		return msg, broken
	}
}
