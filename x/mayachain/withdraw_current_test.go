package mayachain

import (
	"errors"
	"testing"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
	. "gopkg.in/check.v1"
)

func TestPackageWithdrawV124(t *testing.T) { TestingT(t) }

type WithdrawV124TestSuite struct{}

var _ = Suite(&WithdrawV124TestSuite{})

type TestWithdrawKeeperV124 struct {
	keeper.Keeper
	store   map[string]interface{}
	modules map[string]cosmos.Uint
	pools   map[string]Pool
	lps     map[string]LiquidityProvider
}

func NewTestWithdrawKeeperV124(keeper keeper.Keeper) *TestWithdrawKeeperV124 {
	return &TestWithdrawKeeperV124{
		Keeper:  keeper,
		store:   make(map[string]interface{}),
		modules: make(map[string]cosmos.Uint),
		pools:   make(map[string]Pool),
		lps:     make(map[string]LiquidityProvider),
	}
}

func (k *TestWithdrawKeeperV124) GetPool(ctx cosmos.Context, asset common.Asset) (Pool, error) {
	if pool, ok := k.pools[asset.String()]; ok {
		return pool, nil
	}
	return NewPool(), nil
}

func (k *TestWithdrawKeeperV124) SetPool(ctx cosmos.Context, pool Pool) error {
	k.pools[pool.Asset.String()] = pool
	return nil
}

func (k *TestWithdrawKeeperV124) GetLiquidityProvider(ctx cosmos.Context, asset common.Asset, addr common.Address) (LiquidityProvider, error) {
	key := asset.String() + addr.String()
	if lp, ok := k.lps[key]; ok {
		return lp, nil
	}
	return LiquidityProvider{}, nil
}

func (k *TestWithdrawKeeperV124) SetLiquidityProvider(ctx cosmos.Context, lp LiquidityProvider) {
	key := lp.Asset.String() + lp.CacaoAddress.String()
	k.lps[key] = lp
}

func (k *TestWithdrawKeeperV124) RemoveLiquidityProvider(ctx cosmos.Context, lp LiquidityProvider) {
	key := lp.Asset.String() + lp.CacaoAddress.String()
	delete(k.lps, key)
}

func (k *TestWithdrawKeeperV124) GetRuneBalanceOfModule(ctx cosmos.Context, moduleName string) cosmos.Uint {
	if balance, ok := k.modules[moduleName]; ok {
		return balance
	}
	return cosmos.ZeroUint()
}

func (k *TestWithdrawKeeperV124) SendFromModuleToModule(ctx cosmos.Context, from, to string, coins common.Coins) error {
	for _, coin := range coins {
		if coin.Asset.Equals(common.BaseAsset()) {
			fromBalance := k.GetRuneBalanceOfModule(ctx, from)
			if fromBalance.LT(coin.Amount) {
				return errors.New("insufficient funds")
			}
			k.modules[from] = fromBalance.Sub(coin.Amount)
			toBalance := k.GetRuneBalanceOfModule(ctx, to)
			k.modules[to] = toBalance.Add(coin.Amount)
		}
	}
	return nil
}

func (k *TestWithdrawKeeperV124) GetMimir(ctx cosmos.Context, key string) (int64, error) {
	switch key {
	case constants.FullImpLossProtectionBlocks.String():
		return 144000, nil // 100 days
	default:
		return 0, nil
	}
}

func (k *TestWithdrawKeeperV124) GetTotalSupply(ctx cosmos.Context, asset common.Asset) cosmos.Uint {
	return cosmos.ZeroUint()
}

func (k *TestWithdrawKeeperV124) RagnarokInProgress(ctx cosmos.Context) bool {
	return false
}

func (k *TestWithdrawKeeperV124) GetLiquidityAuctionTier(ctx cosmos.Context, addr common.Address) (int64, error) {
	return 0, nil
}

func (k *TestWithdrawKeeperV124) PoolExist(ctx cosmos.Context, asset common.Asset) bool {
	_, ok := k.pools[asset.String()]
	return ok
}

func (s *WithdrawV124TestSuite) TestWithdrawV124_LowReserveBalance(c *C) {
	ctx, mgr := setupManagerForTest(c)

	keeper := NewTestWithdrawKeeperV124(mgr.Keeper())
	keeper.modules[ReserveName] = cosmos.NewUint(1 * common.One) // Low reserve balance
	mgr.K = keeper

	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.BalanceCacao = cosmos.NewUint(100 * common.One)
	pool.LPUnits = cosmos.NewUint(100)
	pool.Status = PoolAvailable
	pool.StatusSince = 1 // Set to match LP's LastAddHeight for ILP calculation
	c.Assert(keeper.SetPool(ctx, pool), IsNil)

	// Create LP with units that would qualify for ILP
	lp := LiquidityProvider{
		Asset:             pool.Asset,
		CacaoAddress:      GetRandomBaseAddress(),
		AssetAddress:      GetRandomBNBAddress(),
		Units:             cosmos.NewUint(50),
		PendingCacao:      cosmos.ZeroUint(),
		PendingAsset:      cosmos.ZeroUint(),
		CacaoDepositValue: cosmos.NewUint(60 * common.One), // deposited more than current value
		AssetDepositValue: cosmos.NewUint(40 * common.One), // to trigger ILP
		LastAddHeight:     1,
	}
	keeper.SetLiquidityProvider(ctx, lp)

	// Set block height to qualify for full ILP (after 100 days)
	ctx = ctx.WithBlockHeight(144000) // 100 days worth of blocks

	// Create withdraw message for 100% withdrawal
	tx := GetRandomTx()
	tx.Chain = common.BNBChain
	tx.FromAddress = lp.AssetAddress
	msg := NewMsgWithdrawLiquidity(
		tx,
		lp.CacaoAddress,
		cosmos.NewUint(10000), // 100% withdrawal
		pool.Asset,
		common.EmptyAsset,
		GetRandomBech32Addr(),
	)

	// Test withdrawal with insufficient reserve
	runeAmt, assetAmt, impLossProtection, units, gasAsset, err := withdrawV124(ctx, *msg, mgr)

	// Should succeed but without ILP protection
	c.Assert(err, IsNil)
	c.Assert(runeAmt.GT(cosmos.ZeroUint()), Equals, true)
	c.Assert(assetAmt.GT(cosmos.ZeroUint()), Equals, true)
	c.Assert(impLossProtection.IsZero(), Equals, true) // No ILP due to insufficient reserve
	c.Assert(units.GT(cosmos.ZeroUint()), Equals, true)
	c.Assert(gasAsset.IsZero(), Equals, true)
}

func (s *WithdrawV124TestSuite) TestWithdrawV124_SufficientReserveBalance(c *C) {
	ctx, mgr := setupManagerForTest(c)

	keeper := NewTestWithdrawKeeperV124(mgr.Keeper())
	keeper.modules[ReserveName] = cosmos.NewUint(1000 * common.One) // Sufficient reserve balance
	mgr.K = keeper

	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceAsset = cosmos.NewUint(200 * common.One) // Asset price dropped (more assets in pool)
	pool.BalanceCacao = cosmos.NewUint(50 * common.One)  // Rune increased in value
	pool.LPUnits = cosmos.NewUint(100)
	pool.Status = PoolAvailable
	pool.StatusSince = 1 // Set to match LP's LastAddHeight for ILP calculation
	c.Assert(keeper.SetPool(ctx, pool), IsNil)

	// Create LP with units that would qualify for ILP
	// LP deposited when ratio was 1:1, now it's 1:4 (impermanent loss scenario)
	lp := LiquidityProvider{
		Asset:             pool.Asset,
		CacaoAddress:      GetRandomBaseAddress(),
		AssetAddress:      GetRandomBNBAddress(),
		Units:             cosmos.NewUint(50),
		PendingCacao:      cosmos.ZeroUint(),
		PendingAsset:      cosmos.ZeroUint(),
		CacaoDepositValue: cosmos.NewUint(50 * common.One), // deposited 50 CACAO
		AssetDepositValue: cosmos.NewUint(50 * common.One), // deposited 50 Asset (1:1 ratio originally)
		LastAddHeight:     1,
	}
	keeper.SetLiquidityProvider(ctx, lp)

	// Set block height to qualify for full ILP (after 100 days)
	ctx = ctx.WithBlockHeight(144000) // 100 days worth of blocks

	// Create withdraw message for 100% withdrawal
	tx := GetRandomTx()
	tx.Chain = common.BNBChain
	tx.FromAddress = lp.AssetAddress
	msg := NewMsgWithdrawLiquidity(
		tx,
		lp.CacaoAddress,
		cosmos.NewUint(10000), // 100% withdrawal
		pool.Asset,
		common.EmptyAsset,
		GetRandomBech32Addr(),
	)

	// Test withdrawal with sufficient reserve
	runeAmt, assetAmt, impLossProtection, units, gasAsset, err := withdrawV124(ctx, *msg, mgr)

	// Should succeed
	c.Assert(err, IsNil)
	c.Assert(runeAmt.GT(cosmos.ZeroUint()), Equals, true)
	c.Assert(assetAmt.GT(cosmos.ZeroUint()), Equals, true)
	c.Assert(units.GT(cosmos.ZeroUint()), Equals, true)
	// ILP should be applied since reserve has sufficient balance
	c.Assert(impLossProtection.GT(cosmos.ZeroUint()), Equals, true)
	c.Assert(gasAsset.IsZero(), Equals, true)
}
