package mayachain

import (
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

type GasManagerTestSuiteV104 struct{}

var _ = Suite(&GasManagerTestSuiteV104{})

func (GasManagerTestSuiteV104) TestGetFeeV104(c *C) {
	ctx, mgr := setupManagerForTest(c)
	k := mgr.Keeper()
	constAccessor := constants.GetConstantValues(GetCurrentVersion())
	gasMgr := newGasMgrV104(constAccessor, k)
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
	c.Assert(fee.Uint64(), Equals, uint64(6_000_000), Commentf("%d vs %d", fee.Uint64(), uint64(6_000_000)))
	c.Assert(k.SetPool(ctx, Pool{
		BalanceCacao: cosmos.NewUint(10000 * common.One),
		BalanceAsset: cosmos.NewUint(100 * common.One),
		Asset:        common.RUNEAsset,
		Status:       PoolAvailable,
	}), IsNil)
	fee = gasMgr.GetFee(ctx, common.THORChain, common.BaseAsset())
	c.Assert(fee.Uint64(), Equals, uint64(6_00000000), Commentf("%d vs %d", fee.Uint64(), uint64(6_00000000)))
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
	c.Assert(fee.Uint64(), Equals, bnbSingleTxFee.Uint64()*3, Commentf("%d vs %d", fee.Uint64(), bnbSingleTxFee.Uint64()*3))

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
	c.Assert(fee.Uint64(), Equals, uint64(70*50*3))

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
	c.Assert(fee.Uint64(), Equals, uint64(150000000))
}
