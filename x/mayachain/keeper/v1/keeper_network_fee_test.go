package keeperv1

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

type KeeperNetworkFeeSuite struct{}

var _ = Suite(&KeeperNetworkFeeSuite{})

func (KeeperNetworkFeeSuite) TestNetworkFee(c *C) {
	ctx, k := setupKeeperForTest(c)
	networkFee := NewNetworkFee(common.BNBChain, 1, 37500)
	c.Check(k.SaveNetworkFee(ctx, common.BNBChain, networkFee), IsNil)

	networkFee1 := NewNetworkFee(common.BNBChain, 0, 37500)
	c.Check(k.SaveNetworkFee(ctx, common.BNBChain, networkFee1), NotNil)

	networkFee2, err := k.GetNetworkFee(ctx, common.ETHChain)
	c.Check(err, IsNil)
	c.Check(networkFee2.Valid(), NotNil)
	c.Check(k.GetNetworkFeeIterator(ctx), NotNil)
	networkFee3, err := k.GetNetworkFee(ctx, common.BNBChain)
	c.Check(err, IsNil)
	c.Check(networkFee3.Valid(), IsNil)
}

func (KeeperNetworkFeeSuite) TestDistributeMayaFund(c *C) {
	ctx, k := setupKeeperForTest(c)

	// Mint MayaFund
	FundModule(c, ctx, k, MayaFund, 100)

	// Set Account for TestDistributeMayaFund
	addr1 := GetRandomBech32Addr()
	addr2 := GetRandomBech32Addr()
	acc1 := k.accountKeeper.NewAccountWithAddress(ctx, addr1)
	acc2 := k.accountKeeper.NewAccountWithAddress(ctx, addr2)

	// In/Out values
	amtAcc1 := (uint64)(7500000000)
	amtAcc2 := (uint64)(2500000000)

	// Add maya token to accounts -> acc1 = 75%, acc2 = 25%
	FundAccountMayaToken(c, ctx, k, acc1.GetAddress(), amtAcc1)
	FundAccountMayaToken(c, ctx, k, acc2.GetAddress(), amtAcc2)

	v := GetCurrentVersion()
	constantAccessor := constants.GetConstantValues(v)
	k.DistributeMayaFund(ctx, constantAccessor)

	// Get balances
	balAcc1 := k.GetBalance(ctx, acc1.GetAddress())
	balAcc2 := k.GetBalance(ctx, acc2.GetAddress())
	balMaya := k.GetBalance(ctx, k.GetModuleAccAddress(MayaFund))

	// Distribute MayaFund -> acc1 with 75% and acc2 with 25%
	for _, coin := range balAcc1 {
		if coin.GetDenom() == common.BaseNative.Native() {
			c.Assert(coin.Amount.Equal(cosmos.NewInt((int64)(amtAcc1))), Equals, true)
		}
	}
	for _, coin := range balAcc2 {
		if coin.GetDenom() == common.BaseNative.Native() {
			c.Assert(coin.Amount.Equal(cosmos.NewInt((int64)(amtAcc2))), Equals, true)
		}
	}

	// At the end the MayaFund should be left with 0 rune
	for _, coin := range balMaya {
		if coin.GetDenom() == common.BaseNative.Native() {
			c.Assert(coin.Amount.IsZero(), Equals, true)
		}
	}

	// In/Out values
	inAmt2 := (uint64)(98765430000)
	amt2Acc1 := (uint64)(67500000000)
	amt2Acc2 := (uint64)(22500000000)
	outAmt2 := inAmt2 - (amt2Acc1 + amt2Acc2)

	// Mint MayaFund
	coin := common.NewCoin(common.BaseNative, cosmos.NewUint(inAmt2))
	err := k.MintToModule(ctx, ModuleName, coin)
	c.Assert(err, IsNil)
	err = k.SendFromModuleToModule(ctx, ModuleName, MayaFund, common.NewCoins(coin))
	c.Assert(err, IsNil)

	k.DistributeMayaFund(ctx, constantAccessor)

	// Get balances
	balAcc1 = k.GetBalance(ctx, acc1.GetAddress())
	balAcc2 = k.GetBalance(ctx, acc2.GetAddress())
	balMaya = k.GetBalance(ctx, k.GetModuleAccAddress(MayaFund))

	// Distribute MayaFund -> acc1 with 75% and acc2 with 25%
	for _, coin := range balAcc1 {
		if coin.GetDenom() == common.BaseNative.Native() {
			c.Assert(coin.Amount.Equal(cosmos.NewInt((int64)(amtAcc1+amt2Acc1))), Equals, true)
		}
	}
	for _, coin := range balAcc2 {
		if coin.GetDenom() == common.BaseNative.Native() {
			c.Assert(coin.Amount.Equal(cosmos.NewInt((int64)(amtAcc2+amt2Acc2))), Equals, true)
		}
	}

	// At the end the MayaFund should be left with 876,543 rune
	for _, coin := range balMaya {
		if coin.GetDenom() == common.BaseNative.Native() {
			c.Assert(coin.Amount.Equal(cosmos.NewInt((int64)(outAmt2))), Equals, true)
		}
	}
}

func (KeeperNetworkFeeSuite) TestDynamicInflation(c *C) {
	v := GetCurrentVersion()
	constantAccessor := constants.GetConstantValues(v)

	// Inflation = 100% -> Should not mint cacao
	ctx, k := SetupForDynamicInflationTest(c, 100_000_000)
	amtCacaoBefore := k.GetTotalSupply(ctx, common.BaseNative)
	c.Assert(k.DynamicInflation(ctx, constantAccessor), IsNil)
	amtCacaoAter := k.GetTotalSupply(ctx, common.BaseNative)
	c.Assert(amtCacaoAter.Equal(amtCacaoBefore), Equals, true)

	// Inflation = 90% -> Should not mint cacao
	ctx, k = SetupForDynamicInflationTest(c, 90_000_000)
	amtCacaoBefore = k.GetTotalSupply(ctx, common.BaseNative)
	c.Assert(k.DynamicInflation(ctx, constantAccessor), IsNil)
	amtCacaoAter = k.GetTotalSupply(ctx, common.BaseNative)
	c.Assert(amtCacaoAter.Equal(amtCacaoBefore), Equals, true)

	// Inflation ~ 0% -> Should not mint
	ctx, k = SetupForDynamicInflationTest(c, 1)
	amtCacaoBefore = k.GetTotalSupply(ctx, common.BaseNative)
	c.Assert(k.DynamicInflation(ctx, constantAccessor), IsNil)
	amtCacaoAter = k.GetTotalSupply(ctx, common.BaseNative)
	c.Assert(amtCacaoAter.Equal(amtCacaoBefore), Equals, true)

	// Inflation = 89% -> Should mint: TotalCACAOonChain - CACAOonReserve * Inflation / BlockPerYear
	// Minted = (10,000,000,000,000,000 - 0) * 0.054 / 5,256,000 = 102,739,726
	ctx, k = SetupForDynamicInflationTest(c, 89_000_000)
	amtCacaoBefore = k.GetTotalSupply(ctx, common.BaseNative)
	c.Assert(k.DynamicInflation(ctx, constantAccessor), IsNil)
	amtCacaoAter = k.GetTotalSupply(ctx, common.BaseNative)
	c.Assert(amtCacaoAter.Equal(amtCacaoBefore.Add(cosmos.NewUint(102_739_726))), Equals, true)

	// Inflation = 80% -> Should mint: TotalCACAOonChain - CACAOonReserve * Inflation / BlockPerYear
	// Minted = (10,000,000,000,000,000 - 0) * 0.09 / 5,256,000 = 171,232,876
	ctx, k = SetupForDynamicInflationTest(c, 80_000_000)
	amtCacaoBefore = k.GetTotalSupply(ctx, common.BaseNative)
	c.Assert(k.DynamicInflation(ctx, constantAccessor), IsNil)
	amtCacaoAter = k.GetTotalSupply(ctx, common.BaseNative)
	c.Assert(amtCacaoAter.Equal(amtCacaoBefore.Add(cosmos.NewUint(171_232_876))), Equals, true)

	// Inflation = 70% -> Should mint: TotalCACAOonChain - CACAOonReserve * Inflation / BlockPerYear
	// Minted = (10,000,000,000,000,000 - 0) * 0.13 / 5,256,000 = 247,336,377
	ctx, k = SetupForDynamicInflationTest(c, 70_000_000)
	amtCacaoBefore = k.GetTotalSupply(ctx, common.BaseNative)
	c.Assert(k.DynamicInflation(ctx, constantAccessor), IsNil)
	amtCacaoAter = k.GetTotalSupply(ctx, common.BaseNative)
	c.Assert(amtCacaoAter.Equal(amtCacaoBefore.Add(cosmos.NewUint(247_336_377))), Equals, true)

	// Inflation = 60% -> Should mint: TotalCACAOonChain - CACAOonReserve * Inflation / BlockPerYear
	// Minted = (10,000,000,000,000,000 - 0) * 0.17 / 5,256,000 = 323,439,878
	ctx, k = SetupForDynamicInflationTest(c, 60_000_000)
	amtCacaoBefore = k.GetTotalSupply(ctx, common.BaseNative)
	c.Assert(k.DynamicInflation(ctx, constantAccessor), IsNil)
	amtCacaoAter = k.GetTotalSupply(ctx, common.BaseNative)
	c.Assert(amtCacaoAter.Equal(amtCacaoBefore.Add(cosmos.NewUint(323_439_878))), Equals, true)

	// Inflation = 50% -> Should mint: TotalCACAOonChain - CACAOonReserve * Inflation / BlockPerYear
	// Minted = (10,000,000,000,000,000 - 0) * 0.21 / 5,256,000 = 399,543,378
	ctx, k = SetupForDynamicInflationTest(c, 50_000_000)
	amtCacaoBefore = k.GetTotalSupply(ctx, common.BaseNative)
	c.Assert(k.DynamicInflation(ctx, constantAccessor), IsNil)
	amtCacaoAter = k.GetTotalSupply(ctx, common.BaseNative)
	c.Assert(amtCacaoAter.Equal(amtCacaoBefore.Add(cosmos.NewUint(399_543_378))), Equals, true)

	// Inflation = 10% -> Should mint: TotalCACAOonChain - CACAOonReserve * Inflation / BlockPerYear
	// Minted = (10,000,000,000,000,000 - 0) * 0.37 / 5,256,000 = 703,957,382
	ctx, k = SetupForDynamicInflationTest(c, 10_000_000)
	amtCacaoBefore = k.GetTotalSupply(ctx, common.BaseNative)
	c.Assert(k.DynamicInflation(ctx, constantAccessor), IsNil)
	amtCacaoAter = k.GetTotalSupply(ctx, common.BaseNative)
	c.Assert(amtCacaoAter.Equal(amtCacaoBefore.Add(cosmos.NewUint(703_957_382))), Equals, true)

	// Inflation = 1% -> Should mint: TotalCACAOonChain - CACAOonReserve * Inflation / BlockPerYear
	// Minted = (10,000,000,000,000,000 - 0) * 0.406 / 5,256,000 = 772,450,532
	ctx, k = SetupForDynamicInflationTest(c, 1_000_000)
	amtCacaoBefore = k.GetTotalSupply(ctx, common.BaseNative)
	c.Assert(k.DynamicInflation(ctx, constantAccessor), IsNil)
	amtCacaoAter = k.GetTotalSupply(ctx, common.BaseNative)
	c.Assert(amtCacaoAter.Equal(amtCacaoBefore.Add(cosmos.NewUint(772_450_532))), Equals, true)

	// Test with CACAO on reserve
	// Inflation = 80% -> Should mint: TotalCACAOonChain - CACAOonReserve * Inflation / BlockPerYear
	// Minted = (10,010,000,000,000,000 - 10,000,000,000,000) * 0.09 / 5,256,000 = 171,232,876
	ctx, k = SetupForDynamicInflationTest(c, 80_000_000)
	FundModule(c, ctx, k, ReserveName, 10_000_000_000_000)
	amtCacaoBefore = k.GetTotalSupply(ctx, common.BaseNative)
	c.Assert(k.DynamicInflation(ctx, constantAccessor), IsNil)
	amtCacaoAter = k.GetTotalSupply(ctx, common.BaseNative)
	c.Assert(amtCacaoAter.Equal(amtCacaoBefore.Add(cosmos.NewUint(171_232_876))), Equals, true)
}

func (KeeperNetworkFeeSuite) TestdistributeDynamicInflation(c *C) {
	ctx, k := SetupForDynamicInflationTest(c, 100_000_000)
	v := GetCurrentVersion()
	constantAccessor := constants.GetConstantValues(v)

	// Zero cacao to mint
	c.Assert(k.distributeDynamicInflation(ctx, constantAccessor, sdk.ZeroUint()).Error(), Equals, "nothing to mint on distributeDynamicInflation")

	// Distribute 100,000,000
	// 50% Pools = 50,000,000
	// 50% System Income = 50,000,000 -> 90% to Reserve = 45,000,000, 10% to MayaFund = 5,000,000
	pool, err := k.GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	poolBefore := pool.BalanceCacao
	reserveAmtBefore := k.GetBalance(ctx, k.GetModuleAccAddress(ReserveName)).AmountOf(common.BaseNative.Native())
	mayaAmtBefore := k.GetBalance(ctx, k.GetModuleAccAddress(MayaFund)).AmountOf(common.BaseNative.Native())

	c.Assert(k.distributeDynamicInflation(ctx, constantAccessor, sdk.NewUint(100_000_000)), IsNil)

	pool, err = k.GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Assert(pool.BalanceCacao.Equal(poolBefore.Add(sdk.NewUint(50_000_000))), Equals, true)
	reserveAmtAfter := k.GetBalance(ctx, k.GetModuleAccAddress(ReserveName)).AmountOf(common.BaseNative.Native())
	c.Assert(reserveAmtAfter.Equal(reserveAmtBefore.Add(sdk.NewIntFromUint64(45_000_000))), Equals, true)
	mayaAmtAfter := k.GetBalance(ctx, k.GetModuleAccAddress(MayaFund)).AmountOf(common.BaseNative.Native())
	c.Assert(mayaAmtAfter.Equal(mayaAmtBefore.Add(sdk.NewIntFromUint64(5_000_000))), Equals, true)

	// Distribute 123,456,790
	// 50% Pools = 61,728,395
	// 50% System Income = 61,728,395 -> 90% to Reserve = 55,555,556, 10% to MayaFund = 6,172,839
	pool, err = k.GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	poolBefore = pool.BalanceCacao
	reserveAmtBefore = k.GetBalance(ctx, k.GetModuleAccAddress(ReserveName)).AmountOf(common.BaseNative.Native())
	mayaAmtBefore = k.GetBalance(ctx, k.GetModuleAccAddress(MayaFund)).AmountOf(common.BaseNative.Native())

	c.Assert(k.distributeDynamicInflation(ctx, constantAccessor, sdk.NewUint(123_456_790)), IsNil)

	pool, err = k.GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Assert(pool.BalanceCacao.Equal(poolBefore.Add(sdk.NewUint(61_728_395))), Equals, true)
	reserveAmtAfter = k.GetBalance(ctx, k.GetModuleAccAddress(ReserveName)).AmountOf(common.BaseNative.Native())
	c.Assert(reserveAmtAfter.Equal(reserveAmtBefore.Add(sdk.NewIntFromUint64(55_555_556))), Equals, true)
	mayaAmtAfter = k.GetBalance(ctx, k.GetModuleAccAddress(MayaFund)).AmountOf(common.BaseNative.Native())
	c.Assert(mayaAmtAfter.Equal(mayaAmtBefore.Add(sdk.NewIntFromUint64(6_172_839))), Equals, true)
}

func (KeeperNetworkFeeSuite) TestGetCACAOOnPools(c *C) {
	ctx, k := setupKeeperForTest(c)

	// Set BNB pool
	pBNB, err := k.GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	pBNB.Asset = common.BNBAsset
	pBNB.BalanceCacao = cosmos.NewUint(common.One)
	pBNB.BalanceAsset = cosmos.NewUint(common.One)
	pBNB.LPUnits = cosmos.NewUint(common.One)
	pBNB.Status = PoolAvailable
	c.Assert(k.SetPool(ctx, pBNB), IsNil)

	// Set BTC pool
	pBTC, err := k.GetPool(ctx, common.BTCAsset)
	c.Assert(err, IsNil)
	pBTC.Asset = common.BTCAsset
	pBTC.BalanceCacao = cosmos.NewUint(common.One)
	pBTC.BalanceAsset = cosmos.NewUint(common.One)
	pBTC.LPUnits = cosmos.NewUint(common.One)
	pBTC.Status = PoolAvailable
	c.Assert(k.SetPool(ctx, pBTC), IsNil)

	// Set ETH pool
	pETH, err := k.GetPool(ctx, common.ETHAsset)
	c.Assert(err, IsNil)
	pETH.Asset = common.ETHAsset
	pETH.BalanceCacao = cosmos.NewUint(common.One)
	pETH.BalanceAsset = cosmos.NewUint(common.One)
	pETH.LPUnits = cosmos.NewUint(common.One)
	pETH.Status = PoolStaged
	c.Assert(k.SetPool(ctx, pETH), IsNil)

	// The sum of the pools BalanceRune should be the same as GetCACAOOnPools
	poolSum := pBNB.BalanceCacao.Add(pBTC.BalanceCacao).Add(pETH.BalanceCacao)
	cacao, err := k.GetCacaoOnPools(ctx, AllPoolStatus)
	c.Assert(err, IsNil)
	c.Assert(cacao.Equal(poolSum), Equals, true)
}

func (KeeperNetworkFeeSuite) TestGetCACAOOnAvailablePools(c *C) {
	ctx, k := setupKeeperForTest(c)

	// Set BNB pool
	pBNB, err := k.GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	pBNB.Asset = common.BNBAsset
	pBNB.BalanceCacao = cosmos.NewUint(common.One)
	pBNB.BalanceAsset = cosmos.NewUint(common.One)
	pBNB.LPUnits = cosmos.NewUint(common.One)
	pBNB.Status = PoolAvailable
	c.Assert(k.SetPool(ctx, pBNB), IsNil)

	// Set BTC pool
	pBTC, err := k.GetPool(ctx, common.BTCAsset)
	c.Assert(err, IsNil)
	pBTC.Asset = common.BTCAsset
	pBTC.BalanceCacao = cosmos.NewUint(common.One)
	pBTC.BalanceAsset = cosmos.NewUint(common.One)
	pBTC.LPUnits = cosmos.NewUint(common.One)
	pBTC.Status = PoolAvailable
	c.Assert(k.SetPool(ctx, pBTC), IsNil)

	// Set ETH pool
	pETH, err := k.GetPool(ctx, common.ETHAsset)
	c.Assert(err, IsNil)
	pETH.Asset = common.ETHAsset
	pETH.BalanceCacao = cosmos.NewUint(common.One)
	pETH.BalanceAsset = cosmos.NewUint(common.One)
	pETH.LPUnits = cosmos.NewUint(common.One)
	pETH.Status = PoolStaged
	c.Assert(k.SetPool(ctx, pETH), IsNil)

	// The sum of the pools BalanceRune should be the same as GetCACAOOnAvailablePools only for the Available pools
	poolSum := pBNB.BalanceCacao.Add(pBTC.BalanceCacao)
	cacao, err := k.GetCacaoOnPools(ctx, PoolAvailable)
	c.Assert(err, IsNil)
	c.Assert(cacao.Equal(poolSum), Equals, true)
}
