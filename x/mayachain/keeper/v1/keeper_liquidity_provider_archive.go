package keeperv1

import (
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

func (k KVStore) calcLPLiquidityBondV1(ctx cosmos.Context, bondAddr common.Address) (cosmos.Uint, error) {
	liquidity := cosmos.ZeroUint()
	lps, err := k.GetLiquidityProviderByAssets(ctx, GetLiquidityPools(k.GetVersion()), bondAddr)
	if err != nil {
		return cosmos.ZeroUint(), err
	}

	for _, lp := range lps {
		var pool Pool
		pool, err = k.GetPool(ctx, lp.Asset)
		if err != nil {
			return cosmos.ZeroUint(), err
		}

		liquidity = liquidity.Add(common.GetSafeShare(lp.Units, pool.LPUnits, pool.BalanceCacao))
		liquidity = liquidity.Add(pool.AssetValueInRune(common.GetSafeShare(lp.Units, pool.LPUnits, pool.BalanceAsset)))
	}
	return liquidity, nil
}
