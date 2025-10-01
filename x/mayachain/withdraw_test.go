package mayachain

import (
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
)

// getWithdrawTestKeeper creates a test keeper with a pool setup for withdrawal tests
// This is used by multiple withdrawal version tests (v102, v105, etc.)
func getWithdrawTestKeeper(c *C, ctx cosmos.Context, k keeper.Keeper, runeAddress common.Address, tier int64, withdrawCounter cosmos.Uint) keeper.Keeper {
	// Create the appropriate test keeper based on the keeper type
	// For now, we'll use a generic approach that works with embedded keepers

	store := NewWithdrawTestKeeperV105(k)
	pool := Pool{
		BalanceCacao: cosmos.NewUint(100 * common.One),
		BalanceAsset: cosmos.NewUint(100 * common.One),
		Asset:        common.BNBAsset,
		LPUnits:      cosmos.NewUint(100 * common.One), // Total pool units is 100
		SynthUnits:   cosmos.ZeroUint(),
		Status:       PoolAvailable,
	}
	c.Assert(store.SetPool(ctx, pool), IsNil)

	lp := LiquidityProvider{
		Asset:              pool.Asset,
		CacaoAddress:       runeAddress,
		AssetAddress:       runeAddress,
		LastAddHeight:      0,
		LastWithdrawHeight: 0,
		Units:              cosmos.NewUint(100 * common.One), // LP owns 100 units (100% of pool)
		PendingCacao:       cosmos.ZeroUint(),
		PendingAsset:       cosmos.ZeroUint(),
		PendingTxID:        "",
		CacaoDepositValue:  cosmos.NewUint(100 * common.One),
		AssetDepositValue:  cosmos.NewUint(100 * common.One),
		WithdrawCounter:    withdrawCounter,
	}
	store.SetLiquidityProvider(ctx, lp)

	c.Assert(k.SetLiquidityAuctionTier(ctx, runeAddress, tier), IsNil)
	getTier, err := k.GetLiquidityAuctionTier(ctx, runeAddress)
	c.Assert(err, IsNil)
	c.Assert(tier, Equals, getTier)

	return store
}

// getWithdrawTestKeeper2 creates a test keeper with a different pool setup for asymmetric withdrawal tests
// This matches the expected behavior in the asymmetric withdrawal tests
func getWithdrawTestKeeper2(c *C, ctx cosmos.Context, k keeper.Keeper, runeAddress common.Address, tier int64, withdrawCounter cosmos.Uint) keeper.Keeper {
	store := NewWithdrawTestKeeperV105(k)
	pool := Pool{
		BalanceCacao: cosmos.NewUint(100 * common.One),
		BalanceAsset: cosmos.NewUint(100 * common.One),
		Asset:        common.BNBAsset,
		LPUnits:      cosmos.NewUint(200 * common.One), // Total pool units is 200
		SynthUnits:   cosmos.ZeroUint(),
		Status:       PoolAvailable,
	}
	c.Assert(store.SetPool(ctx, pool), IsNil)

	lp := LiquidityProvider{
		Asset:              pool.Asset,
		CacaoAddress:       runeAddress,
		AssetAddress:       runeAddress,
		LastAddHeight:      0,
		LastWithdrawHeight: 0,
		Units:              cosmos.NewUint(100 * common.One), // LP owns 100 units (50% of pool)
		PendingCacao:       cosmos.ZeroUint(),
		PendingAsset:       cosmos.ZeroUint(),
		PendingTxID:        "",
		CacaoDepositValue:  cosmos.NewUint(100 * common.One),
		AssetDepositValue:  cosmos.NewUint(100 * common.One),
		WithdrawCounter:    withdrawCounter,
	}
	store.SetLiquidityProvider(ctx, lp)

	c.Assert(k.SetLiquidityAuctionTier(ctx, runeAddress, tier), IsNil)
	getTier, err := k.GetLiquidityAuctionTier(ctx, runeAddress)
	c.Assert(err, IsNil)
	c.Assert(tier, Equals, getTier)

	return store
}
