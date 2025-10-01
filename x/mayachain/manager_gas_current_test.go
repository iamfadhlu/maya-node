package mayachain

import (
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
)

type GasManagerTestSuiteV106 struct{}

var _ = Suite(&GasManagerTestSuiteV106{})

type gasManagerTestHelper struct {
	keeper.Keeper
	failGetNetwork bool
	failGetPool    bool
	failSetPool    bool
}

func newGasManagerTestHelper(k keeper.Keeper) *gasManagerTestHelper {
	return &gasManagerTestHelper{
		Keeper: k,
	}
}

func (GasManagerTestSuiteV106) TestGetFeeV106(c *C) {
	ctx, mgr := setupManagerForTest(c)
	k := mgr.Keeper()
	constAccessor := constants.GetConstantValues(GetCurrentVersion())
	gasMgr := newGasMgrVCUR(constAccessor, k)
	gasMgr.BeginBlock(mgr)
	fee := gasMgr.GetFee(ctx, common.BNBChain, common.BaseAsset())
	defaultBaseTxFee := uint64(constAccessor.GetInt64Value(constants.OutboundTransactionFee))
	defaultTxFee := uint64(2_000000)
	// when there is no network fee available, it should just get from the constants
	c.Assert(fee.Uint64(), Equals, defaultTxFee)
	fee = gasMgr.GetFee(ctx, common.BASEChain, common.BaseAsset())
	c.Assert(fee.Uint64(), Equals, defaultBaseTxFee)

	// when chain is thorchain it should calc based on fixed 0.02 rune
	fee = gasMgr.GetFee(ctx, common.THORChain, common.RUNEAsset)
	c.Assert(fee.Uint64(), Equals, uint64(4_000_000), Commentf("%d vs %d", fee.Uint64(), uint64(4_000_000)))
	c.Assert(k.SetPool(ctx, Pool{
		BalanceCacao: cosmos.NewUint(10000 * common.One),
		BalanceAsset: cosmos.NewUint(100 * common.One),
		Asset:        common.RUNEAsset,
		Status:       PoolAvailable,
	}), IsNil)
	fee = gasMgr.GetFee(ctx, common.THORChain, common.BaseAsset())
	c.Assert(fee.Uint64(), Equals, uint64(400_000_000), Commentf("%d vs %d", fee.Uint64(), uint64(400_000_000)))
	// bnb
	networkFee := NewNetworkFee(common.BNBChain, 1, bnbSingleTxFee.Uint64())
	c.Assert(k.SaveNetworkFee(ctx, common.BNBChain, networkFee), IsNil)
	fee = gasMgr.GetFee(ctx, common.BNBChain, common.BaseAsset())
	c.Assert(fee.Uint64(), Equals, defaultTxFee)
	c.Assert(k.SetPool(ctx, Pool{
		BalanceCacao: cosmos.NewUint(100 * common.One),
		BalanceAsset: cosmos.NewUint(100 * common.One),
		Asset:        common.BNBAsset,
		Status:       PoolAvailable,
	}), IsNil)
	fee = gasMgr.GetFee(ctx, common.BNBChain, common.BaseAsset())
	c.Assert(fee.Uint64(), Equals, bnbSingleTxFee.Uint64()*2, Commentf("%d vs %d", fee.Uint64(), bnbSingleTxFee.Uint64()*2))

	// BTC chain
	networkFee = NewNetworkFee(common.BTCChain, 70, 50)
	c.Assert(k.SaveNetworkFee(ctx, common.BTCChain, networkFee), IsNil)
	fee = gasMgr.GetFee(ctx, common.BTCChain, common.BaseAsset())
	c.Assert(fee.Uint64(), Equals, defaultTxFee)
	c.Assert(k.SetPool(ctx, Pool{
		BalanceCacao: cosmos.NewUint(100 * common.One),
		BalanceAsset: cosmos.NewUint(100 * common.One),
		Asset:        common.BTCAsset,
		Status:       PoolAvailable,
	}), IsNil)
	fee = gasMgr.GetFee(ctx, common.BTCChain, common.BaseAsset())
	c.Assert(fee.Uint64(), Equals, uint64(70*50*2))

	// Synth asset (BTC/BTC)
	sBTC, err := common.NewAsset("BTC/BTC")
	c.Assert(err, IsNil)

	// change the pool balance
	c.Assert(k.SetPool(ctx, Pool{
		BalanceCacao: cosmos.NewUint(500 * common.One),
		BalanceAsset: cosmos.NewUint(100 * common.One),
		Asset:        common.BTCAsset,
		Status:       PoolAvailable,
	}), IsNil)
	synthAssetFee := gasMgr.GetFee(ctx, common.BASEChain, sBTC)
	c.Assert(synthAssetFee.Uint64(), Equals, uint64(400000000), Commentf("expected %d, got %d", 400000000, synthAssetFee.Uint64()))

	// when MinimumL1OutboundFeeUSD set to something higher, it should override the network fee
	busdAsset, err := common.NewAsset("BNB.BUSD-BD1")
	c.Assert(err, IsNil)
	c.Assert(k.SetPool(ctx, Pool{
		BalanceCacao: cosmos.NewUint(500 * common.One),
		BalanceAsset: cosmos.NewUint(500 * common.One),
		Asset:        busdAsset,
		Status:       PoolAvailable,
	}), IsNil)
	k.SetMimir(ctx, constants.MinimumL1OutboundFeeUSD.String(), 1_0000_0000)

	fee = gasMgr.GetFee(ctx, common.BTCChain, common.BTCAsset)
	c.Assert(fee.Uint64(), Equals, uint64(20000000))

	// when network fee is higher than MinimumL1OutboundFeeUSD , then choose network fee
	networkFee = NewNetworkFee(common.BTCChain, 1000, 50000)
	c.Assert(k.SaveNetworkFee(ctx, common.BTCChain, networkFee), IsNil)
	fee = gasMgr.GetFee(ctx, common.BTCChain, common.BTCAsset)
	c.Assert(fee.Uint64(), Equals, uint64(100000000))
}

func (GasManagerTestSuiteV106) TestOutboundFeeMultiplier(c *C) {
	ctx, k := setupKeeperForTest(c)
	constAccessor := constants.GetConstantValues(GetCurrentVersion())
	gasMgr := newGasMgrVCUR(constAccessor, k)

	targetSurplus := cosmos.NewUint(100_00000000) // 100 $RUNE
	minMultiplier := cosmos.NewUint(15_000)
	maxMultiplier := cosmos.NewUint(20_000)
	gasSpent := cosmos.ZeroUint()
	gasWithheld := cosmos.ZeroUint()

	// No surplus to start, should return maxMultiplier
	m := gasMgr.CalcOutboundFeeMultiplier(ctx, targetSurplus, gasSpent, gasWithheld, maxMultiplier, minMultiplier)
	c.Assert(m.Uint64(), Equals, maxMultiplier.Uint64())

	// More gas spent than withheld, use maxMultiplier
	gasSpent = cosmos.NewUint(1000)
	m = gasMgr.CalcOutboundFeeMultiplier(ctx, targetSurplus, gasSpent, gasWithheld, maxMultiplier, minMultiplier)
	c.Assert(m.Uint64(), Equals, maxMultiplier.Uint64())

	gasSpent = cosmos.NewUint(100_00000000)
	gasWithheld = cosmos.NewUint(110_00000000)
	m = gasMgr.CalcOutboundFeeMultiplier(ctx, targetSurplus, gasSpent, gasWithheld, maxMultiplier, minMultiplier)
	c.Assert(m.Uint64(), Equals, uint64(19_500), Commentf("%d", m.Uint64()))

	// 50% surplus vs target, reduce multiplier by 50%
	gasSpent = cosmos.NewUint(100_00000000)
	gasWithheld = cosmos.NewUint(150_00000000)
	m = gasMgr.CalcOutboundFeeMultiplier(ctx, targetSurplus, gasSpent, gasWithheld, maxMultiplier, minMultiplier)
	c.Assert(m.Uint64(), Equals, uint64(17_500), Commentf("%d", m.Uint64()))

	// 75% surplus vs target, reduce multiplier by 75%
	gasSpent = cosmos.NewUint(100_00000000)
	gasWithheld = cosmos.NewUint(175_00000000)
	m = gasMgr.CalcOutboundFeeMultiplier(ctx, targetSurplus, gasSpent, gasWithheld, maxMultiplier, minMultiplier)
	c.Assert(m.Uint64(), Equals, uint64(16_250), Commentf("%d", m.Uint64()))

	// 99% surplus vs target, reduce multiplier by 99%
	gasSpent = cosmos.NewUint(100_00000000)
	gasWithheld = cosmos.NewUint(199_00000000)
	m = gasMgr.CalcOutboundFeeMultiplier(ctx, targetSurplus, gasSpent, gasWithheld, maxMultiplier, minMultiplier)
	c.Assert(m.Uint64(), Equals, uint64(15_050), Commentf("%d", m.Uint64()))

	// 100% surplus vs target, reduce multiplier by 100%
	gasSpent = cosmos.NewUint(100_00000000)
	gasWithheld = cosmos.NewUint(200_00000000)
	m = gasMgr.CalcOutboundFeeMultiplier(ctx, targetSurplus, gasSpent, gasWithheld, maxMultiplier, minMultiplier)
	c.Assert(m.Uint64(), Equals, uint64(15_000), Commentf("%d", m.Uint64()))

	// 110% surplus vs target, still reduce multiplier by 100%
	gasSpent = cosmos.NewUint(100_00000000)
	gasWithheld = cosmos.NewUint(210_00000000)
	m = gasMgr.CalcOutboundFeeMultiplier(ctx, targetSurplus, gasSpent, gasWithheld, maxMultiplier, minMultiplier)
	c.Assert(m.Uint64(), Equals, uint64(15_000))

	// If min multiplier somehow gets set above max multiplier, multiplier should return old default (3x)
	maxMultiplier = cosmos.NewUint(10_000)
	m = gasMgr.CalcOutboundFeeMultiplier(ctx, targetSurplus, gasSpent, gasWithheld, maxMultiplier, minMultiplier)
	c.Assert(m.Uint64(), Equals, uint64(30_000), Commentf("%d", m.Uint64()))
}
