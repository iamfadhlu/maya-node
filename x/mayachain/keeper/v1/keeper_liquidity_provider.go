package keeperv1

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper/types"
)

const DefaultTier = int64(0)

func (k KVStore) setLiquidityProvider(ctx cosmos.Context, key string, record LiquidityProvider) {
	store := ctx.KVStore(k.storeKey)
	buf := k.cdc.MustMarshal(&record)
	if buf == nil {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), buf)
	}
}

func (k KVStore) getLiquidityProvider(ctx cosmos.Context, key string, record *LiquidityProvider) (bool, error) {
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return false, nil
	}

	bz := store.Get([]byte(key))
	if err := k.cdc.Unmarshal(bz, record); err != nil {
		return true, dbError(ctx, fmt.Sprintf("Unmarshal kvstore: (%T) %s", record, key), err)
	}
	return true, nil
}

func (k KVStore) setLiquidityAuctionTier(ctx cosmos.Context, key string, record LiquidityAuctionTier) {
	store := ctx.KVStore(k.storeKey)
	buf := k.cdc.MustMarshal(&record)
	if buf == nil {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), buf)
	}
}

func (k KVStore) getLiquidityAuctionTier(ctx cosmos.Context, key string, record *LiquidityAuctionTier) (bool, error) {
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return false, nil
	}

	bz := store.Get([]byte(key))
	if err := k.cdc.Unmarshal(bz, record); err != nil {
		return true, dbError(ctx, fmt.Sprintf("Unmarshal kvstore: (%T) %s", record, key), err)
	}
	return true, nil
}

// GetLiquidityProviderIterator iterate liquidity providers
func (k KVStore) GetLiquidityProviderIterator(ctx cosmos.Context, asset common.Asset) cosmos.Iterator {
	key := k.GetKey(ctx, prefixLiquidityProvider, (&LiquidityProvider{Asset: asset}).Key())
	return k.getIterator(ctx, types.DbPrefix(key))
}

func (k KVStore) GetTotalSupply(ctx cosmos.Context, asset common.Asset) cosmos.Uint {
	if k.GetVersion().GTE(semver.MustParse("1.91.0")) {
		// when pool ragnarok started , synth unit become zero
		lay1Asset := asset.GetLayer1Asset()
		ragStart, _ := k.GetPoolRagnarokStart(ctx, lay1Asset)
		if ragStart > 0 {
			return cosmos.ZeroUint()
		}
	}
	coin := k.coinKeeper.GetSupply(ctx, asset.Native())
	return cosmos.NewUint(coin.Amount.Uint64())
}

// GetLiquidityProvider retrieve liquidity provider from the data store
func (k KVStore) GetLiquidityProvider(ctx cosmos.Context, asset common.Asset, addr common.Address) (LiquidityProvider, error) {
	record := LiquidityProvider{
		Asset:             asset,
		CacaoAddress:      addr,
		Units:             cosmos.ZeroUint(),
		PendingCacao:      cosmos.ZeroUint(),
		PendingAsset:      cosmos.ZeroUint(),
		CacaoDepositValue: cosmos.ZeroUint(),
		AssetDepositValue: cosmos.ZeroUint(),
		NodeBondAddress:   nil,
	}
	if !addr.IsChain(common.BaseAsset().Chain, k.GetVersion()) {
		record.AssetAddress = addr
		record.CacaoAddress = common.NoAddress
	}

	_, err := k.getLiquidityProvider(ctx, k.GetKey(ctx, prefixLiquidityProvider, record.Key()), &record)
	if err != nil {
		return record, err
	}

	return record, nil
}

// GetLiquidityProviderByAssets returns an LP in the provided assets
func (k KVStore) GetLiquidityProviderByAssets(ctx cosmos.Context, assets common.Assets, addr common.Address) (LiquidityProviders, error) {
	liquidityProviders := LiquidityProviders{}

	for _, asset := range assets {
		lp, err := k.GetLiquidityProvider(ctx, asset, addr)
		if err != nil {
			return liquidityProviders, err
		}
		if lp.Units.GT(cosmos.ZeroUint()) {
			liquidityProviders = append(liquidityProviders, lp)
		}
	}

	return liquidityProviders, nil
}

func (k KVStore) CalcLPLiquidityBond(ctx cosmos.Context, bondAddr common.Address, nodeAddr cosmos.AccAddress) (cosmos.Uint, error) {
	version := k.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.105.0")):
		return k.calcLPLiquidityBondV105(ctx, bondAddr, nodeAddr)
	default:
		return k.calcLPLiquidityBondV1(ctx, bondAddr)
	}
}

func (k KVStore) calcLPLiquidityBondV105(ctx cosmos.Context, bondAddr common.Address, nodeAddr cosmos.AccAddress) (cosmos.Uint, error) {
	liquidity := cosmos.ZeroUint()
	lps, err := k.GetLiquidityProviderByAssets(ctx, GetLiquidityPools(k.GetVersion()), bondAddr)
	if err != nil {
		return cosmos.ZeroUint(), err
	}

	for _, lp := range lps {
		units := cosmos.ZeroUint()
		// If the deprecated NodeBondAddress field is set, all the liquidity on this LP is bonded to the node
		if !lp.NodeBondAddress.Empty() && lp.NodeBondAddress.Equals(nodeAddr) {
			units = lp.Units
		} else {
			// Otherwise, find the node in the list of bonded nodes and calculate the share of liquidity bonded to this specific node
			for _, bond := range lp.BondedNodes {
				if bond.NodeAddress.Equals(nodeAddr) {
					units = bond.Units
					break
				}
			}
		}

		var pool Pool
		pool, err = k.GetPool(ctx, lp.Asset)
		if err != nil {
			return cosmos.ZeroUint(), err
		}
		liquidity = liquidity.Add(common.GetSafeShare(units, pool.LPUnits, pool.BalanceCacao))
		liquidity = liquidity.Add(pool.AssetValueInRune(common.GetSafeShare(units, pool.LPUnits, pool.BalanceAsset)))
	}
	return liquidity, nil
}

func (k KVStore) GetLPLiquidityBond(ctx cosmos.Context, bondAddr common.Address, nodeAddr cosmos.AccAddress) (cosmos.Uint, LiquidityProviders, error) {
	liquidity := cosmos.ZeroUint()
	lps, err := k.GetLiquidityProviderByAssets(ctx, GetLiquidityPools(k.GetVersion()), bondAddr)
	if err != nil {
		return cosmos.ZeroUint(), nil, err
	}

	bondedLPs := make(LiquidityProviders, 0)

	for _, lp := range lps {
		units := cosmos.ZeroUint()
		// If the deprecated NodeBondAddress field is set, all the liquidity on this LP is bonded to the node
		if !lp.NodeBondAddress.Empty() && lp.NodeBondAddress.Equals(nodeAddr) {
			units = lp.Units
		} else {
			// Otherwise, find the node in the list of bonded nodes and calculate the share of liquidity bonded to this specific node
			for _, bond := range lp.BondedNodes {
				if bond.NodeAddress.Equals(nodeAddr) {
					units = bond.Units
					bondedLPs = append(bondedLPs, lp)
					break
				}
			}
		}

		var pool Pool
		pool, err = k.GetPool(ctx, lp.Asset)
		if err != nil {
			return cosmos.ZeroUint(), nil, err
		}
		liquidity = liquidity.Add(common.GetSafeShare(units, pool.LPUnits, pool.BalanceCacao))
		liquidity = liquidity.Add(pool.AssetValueInRune(common.GetSafeShare(units, pool.LPUnits, pool.BalanceAsset)))
	}
	return liquidity, bondedLPs, nil
}

// CalcTotalBondableLiquidity returns the total liquidity of the LP in rune that is bonded or can be bonded to nodes.
func (k KVStore) CalcTotalBondableLiquidity(ctx cosmos.Context, addr common.Address) (cosmos.Uint, error) {
	totalLiquidity := cosmos.ZeroUint()
	liquidityPools := GetLiquidityPools(k.GetVersion())
	lps, err := k.GetLiquidityProviderByAssets(ctx, liquidityPools, addr)
	if err != nil {
		return cosmos.ZeroUint(), err
	}

	for _, lp := range lps {
		var pool Pool
		pool, err = k.GetPool(ctx, lp.Asset)
		if err != nil {
			return cosmos.ZeroUint(), err
		}

		totalLiquidity = totalLiquidity.Add(common.GetSafeShare(lp.Units, pool.LPUnits, pool.BalanceCacao))
		totalLiquidity = totalLiquidity.Add(pool.AssetValueInRune(common.GetSafeShare(lp.Units, pool.LPUnits, pool.BalanceAsset)))
	}

	return totalLiquidity, nil
}

// SetLiquidityProvider save the liquidity provider to kv store
func (k KVStore) SetLiquidityProvider(ctx cosmos.Context, lp LiquidityProvider) {
	k.setLiquidityProvider(ctx, k.GetKey(ctx, prefixLiquidityProvider, lp.Key()), lp)
}

// SetLiquidityProvider save the liquidity provider to kv store
func (k KVStore) SetLiquidityProviders(ctx cosmos.Context, lps LiquidityProviders) {
	for _, liquidityProvider := range lps {
		k.SetLiquidityProvider(ctx, liquidityProvider)
	}
}

// RemoveLiquidityProvider remove the liquidity provider to kv store
func (k KVStore) RemoveLiquidityProvider(ctx cosmos.Context, lp LiquidityProvider) {
	k.del(ctx, k.GetKey(ctx, prefixLiquidityProvider, lp.Key()))
}

// SetLiquidityAuctionTier save the liquidity auction tier to kv store
// if tier already set, it will only be updated if new tier it's equal or greater
func (k KVStore) SetLiquidityAuctionTier(ctx cosmos.Context, addr common.Address, newTier int64) error {
	newLATier := LiquidityAuctionTier{
		Address: addr,
		Tier:    newTier,
	}

	k.setLiquidityAuctionTier(ctx, k.GetKey(ctx, prefixLiquidityAuctionTier, newLATier.Key()), newLATier)
	return nil
}

// GetLiquidityAuctionTier retrieve liquidity auction tier from the data store
// if liquidity auction tier doesn't exist, return const LATier_Dont_Exist
func (k KVStore) GetLiquidityAuctionTier(ctx cosmos.Context, addr common.Address) (int64, error) {
	record := LiquidityAuctionTier{
		Address: addr,
		Tier:    DefaultTier,
	}
	if !addr.IsChain(common.BaseAsset().Chain, k.GetVersion()) {
		record.Address = common.NoAddress
	}

	has, err := k.getLiquidityAuctionTier(ctx, k.GetKey(ctx, prefixLiquidityAuctionTier, record.Key()), &record)
	if err != nil {
		return 0, err
	}

	if !has {
		return 0, nil
	}

	return record.Tier, nil
}
