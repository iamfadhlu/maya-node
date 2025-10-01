package mayachain

import (
	"errors"
	"fmt"

	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

type WithdrawSuiteV102 struct{}

var _ = Suite(&WithdrawSuiteV102{})

type WithdrawTestKeeperV102 struct {
	keeper.KVStoreDummy
	store       map[string]interface{}
	networkFees map[common.Chain]NetworkFee
	keeper      keeper.Keeper
}

func NewWithdrawTestKeeperV102(keeper keeper.Keeper) *WithdrawTestKeeperV102 {
	return &WithdrawTestKeeperV102{
		keeper:      keeper,
		store:       make(map[string]interface{}),
		networkFees: make(map[common.Chain]NetworkFee),
		// mimir:       make(map[string]int64),
	}
}

func (k *WithdrawTestKeeperV102) PoolExist(ctx cosmos.Context, asset common.Asset) bool {
	return !asset.Equals(common.Asset{Chain: common.BNBChain, Symbol: "NOTEXIST", Ticker: "NOTEXIST"})
}

func (k *WithdrawTestKeeperV102) GetPool(ctx cosmos.Context, asset common.Asset) (types.Pool, error) {
	if asset.Equals(common.Asset{Chain: common.BNBChain, Symbol: "NOTEXIST", Ticker: "NOTEXIST"}) {
		return types.Pool{}, nil
	}
	if val, ok := k.store[asset.String()]; ok {
		p, _ := val.(types.Pool)
		return p, nil
	}
	return types.Pool{
		BalanceCacao: cosmos.NewUint(100).MulUint64(common.One),
		BalanceAsset: cosmos.NewUint(100).MulUint64(common.One),
		LPUnits:      cosmos.NewUint(200).MulUint64(common.One), // Total pool units is 200
		SynthUnits:   cosmos.ZeroUint(),
		Status:       PoolAvailable,
		Asset:        asset,
	}, nil
}

func (k *WithdrawTestKeeperV102) SetPool(ctx cosmos.Context, ps Pool) error {
	k.store[ps.Asset.String()] = ps
	return nil
}

func (k *WithdrawTestKeeperV102) GetGas(ctx cosmos.Context, asset common.Asset) ([]cosmos.Uint, error) {
	return []cosmos.Uint{cosmos.NewUint(37500), cosmos.NewUint(30000)}, nil
}

func (k *WithdrawTestKeeperV102) GetLiquidityProvider(ctx cosmos.Context, asset common.Asset, addr common.Address) (LiquidityProvider, error) {
	if asset.Equals(common.Asset{Chain: common.BNBChain, Symbol: "NOTEXISTSTICKER", Ticker: "NOTEXISTSTICKER"}) {
		return types.LiquidityProvider{}, errors.New("you asked for it")
	}
	if notExistLiquidityProviderAsset.Equals(asset) {
		return LiquidityProvider{}, errors.New("simulate error for test")
	}
	return k.keeper.GetLiquidityProvider(ctx, asset, addr)
}

func (k *WithdrawTestKeeperV102) GetNetworkFee(ctx cosmos.Context, chain common.Chain) (NetworkFee, error) {
	return k.networkFees[chain], nil
}

func (k *WithdrawTestKeeperV102) SaveNetworkFee(ctx cosmos.Context, chain common.Chain, networkFee NetworkFee) error {
	k.networkFees[chain] = networkFee
	return nil
}

func (k *WithdrawTestKeeperV102) SetLiquidityProvider(ctx cosmos.Context, lp LiquidityProvider) {
	k.keeper.SetLiquidityProvider(ctx, lp)
}

func (k *WithdrawTestKeeperV102) SetLiquidityAuctionTier(ctx cosmos.Context, addr common.Address, tier int64) error {
	return k.keeper.SetLiquidityAuctionTier(ctx, addr, tier)
}

func (k *WithdrawTestKeeperV102) GetLiquidityAuctionTier(ctx cosmos.Context, addr common.Address) (int64, error) {
	return k.keeper.GetLiquidityAuctionTier(ctx, addr)
}

func (k *WithdrawTestKeeperV102) GetMimir(ctx cosmos.Context, key string) (int64, error) {
	return k.keeper.GetMimir(ctx, key)
}

func (k *WithdrawTestKeeperV102) SetMimir(ctx cosmos.Context, key string, value int64) {
	k.keeper.SetMimir(ctx, key, value)
}

func (k *WithdrawTestKeeperV102) SendFromModuleToModule(ctx cosmos.Context, from, to string, coins common.Coins) error {
	// Just return nil to allow the test to proceed
	// This simulates successful module-to-module transfer
	return nil
}

func (k *WithdrawTestKeeperV102) GetTotalSupply(ctx cosmos.Context, asset common.Asset) cosmos.Uint {
	// Return zero for synthetic assets in tests
	return cosmos.ZeroUint()
}

func (k *WithdrawTestKeeperV102) RagnarokInProgress(ctx cosmos.Context) bool {
	// Return false for tests - no ragnarok in progress
	return false
}

func (s *WithdrawSuiteV102) SetUpSuite(c *C) {
	SetupConfigForTest()
}

// TestValidateWithdraw is to test validateWithdraw function
func (s WithdrawSuiteV102) TestValidateWithdraw(c *C) {
	accountAddr := GetRandomValidatorNode(NodeWhiteListed).NodeAddress
	runeAddress, err := common.NewAddress("bnb1g0xakzh03tpa54khxyvheeu92hwzypkdce77rm", GetCurrentVersion())
	if err != nil {
		c.Error("fail to create new BNB Address")
	}
	inputs := []struct {
		name          string
		msg           MsgWithdrawLiquidity
		expectedError error
	}{
		{
			name: "empty-rune-address",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: "",
				BasisPoints:     cosmos.NewUint(10000),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
			},
			expectedError: errors.New("empty withdraw address"),
		},
		{
			name: "empty-withdraw-basis-points",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: runeAddress,
				BasisPoints:     cosmos.ZeroUint(),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
			},
			expectedError: errors.New("withdraw basis points 0 is invalid"),
		},
		{
			name: "empty-request-txhash",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: runeAddress,
				BasisPoints:     cosmos.NewUint(10000),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{},
				Signer:          accountAddr,
			},
			expectedError: errors.New("request tx hash is empty"),
		},
		{
			name: "empty-asset",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: runeAddress,
				BasisPoints:     cosmos.NewUint(10000),
				Asset:           common.Asset{},
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
			},
			expectedError: errors.New("empty asset"),
		},
		{
			name: "invalid-basis-point",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: runeAddress,
				BasisPoints:     cosmos.NewUint(10001),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
			},
			expectedError: errors.New("withdraw basis points 10001 is invalid"),
		},
		{
			name: "invalid-pool-notexist",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: runeAddress,
				BasisPoints:     cosmos.NewUint(10000),
				Asset:           common.Asset{Chain: common.BNBChain, Ticker: "NOTEXIST", Symbol: "NOTEXIST"},
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
			},
			expectedError: errors.New("pool-BNB.NOTEXIST doesn't exist"),
		},
		{
			name: "all-good",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: runeAddress,
				BasisPoints:     cosmos.NewUint(10000),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
			},
			expectedError: nil,
		},
	}

	for _, item := range inputs {
		ctx, _ := setupKeeperForTest(c)
		ps := &WithdrawTestKeeperV102{}
		c.Logf("name:%s", item.name)
		err := validateWithdrawV1(ctx, ps, item.msg)
		if item.expectedError != nil {
			c.Assert(err, NotNil)
			c.Assert(err.Error(), Equals, item.expectedError.Error())
			continue
		}
		c.Assert(err, IsNil)
	}
}

func (s WithdrawSuiteV102) TestCalculateUnsake(c *C) {
	inputs := []struct {
		name                  string
		poolUnit              cosmos.Uint
		poolRune              cosmos.Uint
		poolAsset             cosmos.Uint
		lpUnit                cosmos.Uint
		percentage            cosmos.Uint
		expectedWithdrawRune  cosmos.Uint
		expectedWithdrawAsset cosmos.Uint
		expectedUnitLeft      cosmos.Uint
		expectedErr           error
	}{
		{
			name:                  "zero-poolunit",
			poolUnit:              cosmos.ZeroUint(),
			poolRune:              cosmos.ZeroUint(),
			poolAsset:             cosmos.ZeroUint(),
			lpUnit:                cosmos.ZeroUint(),
			percentage:            cosmos.ZeroUint(),
			expectedWithdrawRune:  cosmos.ZeroUint(),
			expectedWithdrawAsset: cosmos.ZeroUint(),
			expectedUnitLeft:      cosmos.ZeroUint(),
			expectedErr:           errors.New("poolUnits can't be zero"),
		},

		{
			name:                  "zero-poolrune",
			poolUnit:              cosmos.NewUint(500 * common.One),
			poolRune:              cosmos.ZeroUint(),
			poolAsset:             cosmos.ZeroUint(),
			lpUnit:                cosmos.ZeroUint(),
			percentage:            cosmos.ZeroUint(),
			expectedWithdrawRune:  cosmos.ZeroUint(),
			expectedWithdrawAsset: cosmos.ZeroUint(),
			expectedUnitLeft:      cosmos.ZeroUint(),
			expectedErr:           errors.New("pool rune balance can't be zero"),
		},

		{
			name:                  "zero-poolasset",
			poolUnit:              cosmos.NewUint(500 * common.One),
			poolRune:              cosmos.NewUint(500 * common.One),
			poolAsset:             cosmos.ZeroUint(),
			lpUnit:                cosmos.ZeroUint(),
			percentage:            cosmos.ZeroUint(),
			expectedWithdrawRune:  cosmos.ZeroUint(),
			expectedWithdrawAsset: cosmos.ZeroUint(),
			expectedUnitLeft:      cosmos.ZeroUint(),
			expectedErr:           errors.New("pool asset balance can't be zero"),
		},
		{
			name:                  "negative-liquidity-provider-unit",
			poolUnit:              cosmos.NewUint(500 * common.One),
			poolRune:              cosmos.NewUint(500 * common.One),
			poolAsset:             cosmos.NewUint(5100 * common.One),
			lpUnit:                cosmos.ZeroUint(),
			percentage:            cosmos.ZeroUint(),
			expectedWithdrawRune:  cosmos.ZeroUint(),
			expectedWithdrawAsset: cosmos.ZeroUint(),
			expectedUnitLeft:      cosmos.ZeroUint(),
			expectedErr:           errors.New("liquidity provider unit can't be zero"),
		},

		{
			name:                  "percentage-larger-than-100",
			poolUnit:              cosmos.NewUint(500 * common.One),
			poolRune:              cosmos.NewUint(500 * common.One),
			poolAsset:             cosmos.NewUint(500 * common.One),
			lpUnit:                cosmos.NewUint(100 * common.One),
			percentage:            cosmos.NewUint(12000),
			expectedWithdrawRune:  cosmos.ZeroUint(),
			expectedWithdrawAsset: cosmos.ZeroUint(),
			expectedUnitLeft:      cosmos.ZeroUint(),
			expectedErr:           fmt.Errorf("withdraw basis point %s is not valid", cosmos.NewUint(12000)),
		},
		{
			name:                  "withdraw-1",
			poolUnit:              cosmos.NewUint(700 * common.One),
			poolRune:              cosmos.NewUint(700 * common.One),
			poolAsset:             cosmos.NewUint(700 * common.One),
			lpUnit:                cosmos.NewUint(200 * common.One),
			percentage:            cosmos.NewUint(10000),
			expectedUnitLeft:      cosmos.ZeroUint(),
			expectedWithdrawAsset: cosmos.NewUint(200 * common.One),
			expectedWithdrawRune:  cosmos.NewUint(200 * common.One),
			expectedErr:           nil,
		},
		{
			name:                  "withdraw-2",
			poolUnit:              cosmos.NewUint(100),
			poolRune:              cosmos.NewUint(15 * common.One),
			poolAsset:             cosmos.NewUint(155 * common.One),
			lpUnit:                cosmos.NewUint(100),
			percentage:            cosmos.NewUint(1000),
			expectedUnitLeft:      cosmos.NewUint(90),
			expectedWithdrawAsset: cosmos.NewUint(1550000000),
			expectedWithdrawRune:  cosmos.NewUint(150000000),
			expectedErr:           nil,
		},
	}

	for _, item := range inputs {
		c.Logf("name:%s", item.name)
		withDrawRune, withDrawAsset, unitAfter, err := calculateWithdrawV91(item.poolUnit, item.poolRune, item.poolAsset, item.lpUnit, cosmos.ZeroUint(), item.percentage, common.EmptyAsset)
		if item.expectedErr == nil {
			c.Assert(err, IsNil)
		} else {
			c.Assert(err.Error(), Equals, item.expectedErr.Error())
		}
		c.Logf("expected rune:%s,rune:%s", item.expectedWithdrawRune, withDrawRune)
		c.Check(item.expectedWithdrawRune.Uint64(), Equals, withDrawRune.Uint64(), Commentf("Expected %d, got %d", item.expectedWithdrawRune.Uint64(), withDrawRune.Uint64()))
		c.Check(item.expectedWithdrawAsset.Uint64(), Equals, withDrawAsset.Uint64(), Commentf("Expected %d, got %d", item.expectedWithdrawAsset.Uint64(), withDrawAsset.Uint64()))
		c.Check(item.expectedUnitLeft.Uint64(), Equals, unitAfter.Uint64())
	}
}

func (WithdrawSuiteV102) TestWithdraw(c *C) {
	ctx, mgr := setupManagerForTest(c)
	accountAddr := GetRandomValidatorNode(NodeWhiteListed).NodeAddress
	runeAddress := GetRandomBaseAddress()
	ps := NewWithdrawTestKeeperV102(mgr.Keeper())
	ps2 := getWithdrawTestKeeper(c, ctx, mgr.Keeper(), runeAddress, 3, cosmos.ZeroUint())

	remainGas := uint64(37500)
	testCases := []struct {
		name          string
		msg           MsgWithdrawLiquidity
		ps            keeper.Keeper
		runeAmount    cosmos.Uint
		assetAmount   cosmos.Uint
		expectedError error
	}{
		{
			name: "empty-rune-address",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: "",
				BasisPoints:     cosmos.NewUint(10000),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
			},
			ps:            ps,
			runeAmount:    cosmos.ZeroUint(),
			assetAmount:   cosmos.ZeroUint(),
			expectedError: errors.New("empty withdraw address"),
		},
		{
			name: "empty-request-txhash",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: runeAddress,
				BasisPoints:     cosmos.NewUint(10000),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{},
				Signer:          accountAddr,
			},
			ps:            ps,
			runeAmount:    cosmos.ZeroUint(),
			assetAmount:   cosmos.ZeroUint(),
			expectedError: errors.New("request tx hash is empty"),
		},
		{
			name: "empty-asset",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: runeAddress,
				BasisPoints:     cosmos.NewUint(10000),
				Asset:           common.Asset{},
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
			},
			ps:            ps,
			runeAmount:    cosmos.ZeroUint(),
			assetAmount:   cosmos.ZeroUint(),
			expectedError: errors.New("empty asset"),
		},
		{
			name: "invalid-basis-point",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: runeAddress,
				BasisPoints:     cosmos.NewUint(10001),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
			},
			ps:            ps,
			runeAmount:    cosmos.ZeroUint(),
			assetAmount:   cosmos.ZeroUint(),
			expectedError: errors.New("withdraw basis points 10001 is invalid"),
		},
		{
			name: "invalid-pool-notexist",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: runeAddress,
				BasisPoints:     cosmos.NewUint(10000),
				Asset:           common.Asset{Chain: common.BNBChain, Ticker: "NOTEXIST", Symbol: "NOTEXIST"},
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
			},
			ps:            ps,
			runeAmount:    cosmos.ZeroUint(),
			assetAmount:   cosmos.ZeroUint(),
			expectedError: errors.New("pool-BNB.NOTEXIST doesn't exist"),
		},
		{
			name: "invalid-pool-liquidity-provider-notexist",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: runeAddress,
				BasisPoints:     cosmos.NewUint(10000),
				Asset:           common.Asset{Chain: common.BNBChain, Ticker: "NOTEXISTSTICKER", Symbol: "NOTEXISTSTICKER"},
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
			},
			ps:            ps,
			runeAmount:    cosmos.ZeroUint(),
			assetAmount:   cosmos.ZeroUint(),
			expectedError: errors.New("you asked for it"),
		},
		{
			name: "nothing-to-withdraw",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: runeAddress,
				BasisPoints:     cosmos.ZeroUint(),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
			},
			ps:            ps,
			runeAmount:    cosmos.ZeroUint(),
			assetAmount:   cosmos.ZeroUint(),
			expectedError: errors.New("withdraw basis points 0 is invalid"),
		},
		{
			name: "all-good-half",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: runeAddress,
				BasisPoints:     cosmos.NewUint(5000),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
			},
			ps:            ps2,
			runeAmount:    cosmos.NewUint(50 * common.One),
			assetAmount:   cosmos.NewUint(50 * common.One),
			expectedError: nil,
		},
		{
			name: "all-good",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: runeAddress,
				BasisPoints:     cosmos.NewUint(10000),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
			},
			ps:            ps2,
			runeAmount:    cosmos.NewUint(50 * common.One),
			assetAmount:   cosmos.NewUint(50 * common.One).Sub(cosmos.NewUint(remainGas)),
			expectedError: nil,
		},
	}
	for _, tc := range testCases {
		c.Logf("name:%s", tc.name)
		mgr.K = tc.ps
		c.Assert(tc.ps.SaveNetworkFee(ctx, common.BNBChain, NetworkFee{
			Chain:              common.BNBChain,
			TransactionSize:    1,
			TransactionFeeRate: bnbSingleTxFee.Uint64(),
		}), IsNil)
		r, asset, _, _, _, err := withdrawV102(ctx, tc.msg, mgr)
		if tc.expectedError != nil {
			c.Assert(err, NotNil)
			c.Check(err.Error(), Equals, tc.expectedError.Error())
			c.Check(r.Uint64(), Equals, tc.runeAmount.Uint64())
			c.Check(asset.Uint64(), Equals, tc.assetAmount.Uint64())
			continue
		}
		c.Assert(err, IsNil)
		c.Assert(r.Uint64(), Equals, tc.runeAmount.Uint64(), Commentf("%d != %d", r.Uint64(), tc.runeAmount.Uint64()))
		c.Assert(asset.Equal(tc.assetAmount), Equals, true, Commentf("expect:%s, however got:%s", tc.assetAmount.String(), asset.String()))
	}
}

func (WithdrawSuiteV102) TestWithdrawAsym(c *C) {
	accountAddr := GetRandomValidatorNode(NodeWhiteListed).NodeAddress
	runeAddress := GetRandomBaseAddress()

	testCases := []struct {
		name          string
		msg           MsgWithdrawLiquidity
		runeAmount    cosmos.Uint
		assetAmount   cosmos.Uint
		expectedError error
	}{
		{
			name: "all-good-asymmetric-rune",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: runeAddress,
				BasisPoints:     cosmos.NewUint(10000),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				WithdrawalAsset: common.BaseAsset(),
				Signer:          accountAddr,
			},
			runeAmount:    cosmos.NewUint(6250000000),
			assetAmount:   cosmos.ZeroUint(),
			expectedError: nil,
		},
		{
			name: "all-good-asymmetric-asset",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: runeAddress,
				BasisPoints:     cosmos.NewUint(10000),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				WithdrawalAsset: common.BNBAsset,
				Signer:          accountAddr,
			},
			runeAmount:    cosmos.ZeroUint(),
			assetAmount:   cosmos.NewUint(6250000000),
			expectedError: nil,
		},
	}
	for _, tc := range testCases {
		c.Logf("name:%s", tc.name)
		ctx, mgr := setupManagerForTest(c)
		ps := getWithdrawTestKeeper2(c, ctx, mgr.Keeper(), runeAddress, 0, cosmos.ZeroUint())
		mgr.K = ps
		c.Assert(ps.SaveNetworkFee(ctx, common.BNBChain, NetworkFee{
			Chain:              common.BNBChain,
			TransactionSize:    1,
			TransactionFeeRate: bnbSingleTxFee.Uint64(),
		}), IsNil)
		r, asset, _, _, _, err := withdrawV102(ctx, tc.msg, mgr)
		if tc.expectedError != nil {
			c.Assert(err, NotNil)
			c.Check(err.Error(), Equals, tc.expectedError.Error())
			c.Check(r.Uint64(), Equals, tc.runeAmount.Uint64())
			c.Check(asset.Uint64(), Equals, tc.assetAmount.Uint64())
			continue
		}
		c.Assert(err, IsNil)
		c.Assert(r.Uint64(), Equals, tc.runeAmount.Uint64(), Commentf("%d != %d", r.Uint64(), tc.runeAmount.Uint64()))
		c.Assert(asset.Equal(tc.assetAmount), Equals, true, Commentf("expect:%s, however got:%s", tc.assetAmount.String(), asset.String()))
	}
}

func (WithdrawSuiteV102) TestWithdrawPendingCacaoOrAsset(c *C) {
	accountAddr := GetRandomValidatorNode(NodeActive).NodeAddress
	ctx, mgr := setupManagerForTest(c)
	pool := Pool{
		BalanceCacao: cosmos.NewUint(100 * common.One),
		BalanceAsset: cosmos.NewUint(100 * common.One),
		Asset:        common.BNBAsset,
		LPUnits:      cosmos.NewUint(200 * common.One),
		Status:       PoolAvailable,
	}
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)
	lp := LiquidityProvider{
		Asset:              common.BNBAsset,
		CacaoAddress:       GetRandomBaseAddress(),
		AssetAddress:       GetRandomBNBAddress(),
		LastAddHeight:      1024,
		LastWithdrawHeight: 0,
		Units:              cosmos.ZeroUint(),
		PendingCacao:       cosmos.NewUint(1024),
		PendingAsset:       cosmos.ZeroUint(),
		PendingTxID:        GetRandomTxHash(),
	}
	mgr.Keeper().SetLiquidityProvider(ctx, lp)
	msg := MsgWithdrawLiquidity{
		WithdrawAddress: lp.CacaoAddress,
		BasisPoints:     cosmos.NewUint(10000),
		Asset:           common.BNBAsset,
		Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
		WithdrawalAsset: common.BNBAsset,
		Signer:          accountAddr,
	}
	runeAmt, assetAmt, _, unitsLeft, gas, err := withdrawV102(ctx, msg, mgr)
	c.Assert(err, IsNil)
	c.Assert(runeAmt.Equal(cosmos.NewUint(1024)), Equals, true)
	c.Assert(assetAmt.IsZero(), Equals, true)
	c.Assert(unitsLeft.IsZero(), Equals, true)
	c.Assert(gas.IsZero(), Equals, true)

	lp1 := LiquidityProvider{
		Asset:              common.BNBAsset,
		CacaoAddress:       GetRandomBaseAddress(),
		AssetAddress:       GetRandomBNBAddress(),
		LastAddHeight:      1024,
		LastWithdrawHeight: 0,
		Units:              cosmos.ZeroUint(),
		PendingCacao:       cosmos.ZeroUint(),
		PendingAsset:       cosmos.NewUint(1024),
		PendingTxID:        GetRandomTxHash(),
	}
	mgr.Keeper().SetLiquidityProvider(ctx, lp1)
	msg1 := MsgWithdrawLiquidity{
		WithdrawAddress: lp1.CacaoAddress,
		BasisPoints:     cosmos.NewUint(10000),
		Asset:           common.BNBAsset,
		Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
		WithdrawalAsset: common.BNBAsset,
		Signer:          accountAddr,
	}
	runeAmt, assetAmt, _, unitsLeft, gas, err = withdrawV102(ctx, msg1, mgr)
	c.Assert(err, IsNil)
	c.Assert(assetAmt.Equal(cosmos.NewUint(1024)), Equals, true)
	c.Assert(runeAmt.IsZero(), Equals, true)
	c.Assert(unitsLeft.IsZero(), Equals, true)
	c.Assert(gas.IsZero(), Equals, true)
}

func (s *WithdrawSuiteV102) TestWithdrawWithImpermanentLossProtection(c *C) {
	accountAddr := GetRandomValidatorNode(NodeActive).NodeAddress
	ctx, mgr := setupManagerForTest(c)
	pool := Pool{
		BalanceCacao: cosmos.NewUint(100 * common.One),
		BalanceAsset: cosmos.NewUint(100 * common.One),
		Asset:        common.BTCAsset,
		LPUnits:      cosmos.NewUint(200 * common.One),
		Status:       PoolAvailable,
	}
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)
	v := GetCurrentVersion()
	constantAccessor := constants.GetConstantValues(v)
	mgr.Keeper().SetMimir(ctx, constants.WithdrawDaysTier1.String(), 1)
	mgr.Keeper().SetMimir(ctx, constants.WithdrawDaysTier2.String(), 1)
	mgr.Keeper().SetMimir(ctx, constants.WithdrawDaysTier3.String(), 1)
	addHandler := NewAddLiquidityHandler(mgr)
	// add some liquidity
	// add some liquidity
	for i := 0; i <= 10; i++ {
		c.Assert(addHandler.addLiquidity(ctx,
			common.BTCAsset,
			cosmos.NewUint(common.One),
			cosmos.NewUint(common.One),
			GetRandomBaseAddress(),
			GetRandomBTCAddress(),
			GetRandomTx(),
			false,
			constantAccessor,
			0), IsNil)
	}
	lpAddr := GetRandomBaseAddress()
	c.Assert(addHandler.addLiquidity(ctx,
		common.BTCAsset,
		cosmos.NewUint(common.One),
		cosmos.NewUint(common.One),
		lpAddr,
		GetRandomBTCAddress(),
		GetRandomTx(),
		false,
		constantAccessor,
		0), IsNil)
	newctx := ctx.WithBlockHeight(ctx.BlockHeight() + 720_000*2)
	msg2 := MsgWithdrawLiquidity{
		WithdrawAddress: lpAddr,
		BasisPoints:     cosmos.NewUint(2500),
		Asset:           common.BTCAsset,
		Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
		WithdrawalAsset: common.BTCAsset,
		Signer:          accountAddr,
	}
	p, err := mgr.Keeper().GetPool(ctx, common.BTCAsset)
	c.Assert(err, IsNil)
	p.BalanceCacao = p.BalanceCacao.Sub(cosmos.NewUint(5 * common.One))
	p.BalanceAsset = p.BalanceAsset.Add(cosmos.NewUint(common.One))
	mgr.Keeper().SetMimir(ctx, fmt.Sprintf("ilp-%s", p.Asset), 1)
	c.Assert(mgr.Keeper().SetPool(ctx, p), IsNil)
	cacaoAmt, assetAmt, protectionCacaoAmt, unitsClaimed, gas, err := withdrawV102(newctx, msg2, mgr)
	c.Assert(err, IsNil)
	p, err = mgr.Keeper().GetPool(ctx, common.BTCAsset)
	c.Assert(err, IsNil)
	c.Check(assetAmt.Equal(cosmos.NewUint(50452579)), Equals, true, Commentf("%d", assetAmt.Uint64()))
	c.Check(cacaoAmt.IsZero(), Equals, true)
	c.Check(unitsClaimed.Equal(cosmos.NewUint(50000000)), Equals, true, Commentf("%d", unitsClaimed.Uint64()))
	c.Check(gas.IsZero(), Equals, true)
	c.Assert(protectionCacaoAmt.Equal(cosmos.NewUint(113088)), Equals, true, Commentf("%d", protectionCacaoAmt.Uint64()))
}

func (s *WithdrawSuiteV102) TestWithdrawPendingLiquidityShouldRoundToPoolDecimals(c *C) {
	accountAddr := GetRandomValidatorNode(NodeActive).NodeAddress
	ctx, mgr := setupManagerForTest(c)
	pool := Pool{
		BalanceCacao: cosmos.NewUint(100 * common.One),
		BalanceAsset: cosmos.NewUint(100 * common.One),
		Asset:        common.BNBAsset,
		LPUnits:      cosmos.NewUint(200 * common.One),
		Status:       PoolAvailable,
		Decimals:     int64(6),
	}
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)
	v := GetCurrentVersion()
	constantAccessor := constants.GetConstantValues(v)
	addHandler := NewAddLiquidityHandler(mgr)
	// create a LP record that has pending asset
	lpAddr := GetRandomBaseAddress()
	c.Assert(addHandler.addLiquidity(ctx,
		common.BNBAsset,
		cosmos.ZeroUint(),
		cosmos.NewUint(339448125567),
		lpAddr,
		GetRandomBTCAddress(),
		GetRandomTx(),
		true,
		constantAccessor,
		0), IsNil)

	newctx := ctx.WithBlockHeight(ctx.BlockHeight() + 17280*2)
	msg2 := MsgWithdrawLiquidity{
		WithdrawAddress: lpAddr,
		BasisPoints:     cosmos.NewUint(10000),
		Asset:           common.BNBAsset,
		Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
		WithdrawalAsset: common.BNBAsset,
		Signer:          accountAddr,
	}
	runeAmt, assetAmt, protectoinRuneAmt, unitsClaimed, _, err := withdrawV102(newctx, msg2, mgr)
	c.Assert(err, IsNil)
	c.Assert(assetAmt.Equal(cosmos.NewUint(339448125500)), Equals, true, Commentf("%d", assetAmt.Uint64()))
	c.Assert(runeAmt.IsZero(), Equals, true)
	c.Assert(protectoinRuneAmt.IsZero(), Equals, true)
	c.Assert(unitsClaimed.IsZero(), Equals, true)
}

func (WithdrawSuiteV102) TestCalcImpLossV102(c *C) {
	testCases := []struct {
		name                 string
		pool                 Pool
		lp                   types.LiquidityProvider
		withdrawBasisPoint   int64
		height               int64
		expectedILP          cosmos.Uint
		expectedDepositValue cosmos.Uint
		expectedRedeemValue  cosmos.Uint
	}{
		{
			name: "little-asset-large-rune",
			pool: Pool{
				Asset:        common.BTCAsset,
				BalanceAsset: cosmos.NewUint(100 * common.One),
				BalanceCacao: cosmos.NewUint(100 * common.One),
				LPUnits:      cosmos.NewUint(100 * common.One),
				SynthUnits:   cosmos.ZeroUint(),
			},
			lp: types.LiquidityProvider{
				Units:             cosmos.NewUint(12345678),
				AssetDepositValue: cosmos.NewUint(common.One),
				CacaoDepositValue: cosmos.NewUint(common.One * 50),
			},
			withdrawBasisPoint:   10000,
			height:               2_160_000 * 4,
			expectedILP:          cosmos.NewUint(5075308644),
			expectedDepositValue: cosmos.NewUint(5100000000),
			expectedRedeemValue:  cosmos.NewUint(24691356),
		},
		{
			name: "symmetrical-add",
			pool: Pool{
				Asset:        common.BTCAsset,
				BalanceAsset: cosmos.NewUint(100 * common.One),
				BalanceCacao: cosmos.NewUint(100 * common.One),
				LPUnits:      cosmos.NewUint(100 * common.One),
				SynthUnits:   cosmos.ZeroUint(),
			},
			lp: types.LiquidityProvider{
				Units:             cosmos.NewUint(common.One * 50),
				AssetDepositValue: cosmos.NewUint(common.One * 50),
				CacaoDepositValue: cosmos.NewUint(common.One * 50),
			},
			withdrawBasisPoint:   10000,
			height:               2_160_000 * 4,
			expectedILP:          cosmos.ZeroUint(),
			expectedDepositValue: cosmos.NewUint(10000000000),
			expectedRedeemValue:  cosmos.NewUint(10000000000),
		},
		{
			name: "ASSET-decreases-in-price-relative-to-CACAO",
			pool: Pool{
				Asset:        common.BTCAsset,
				BalanceAsset: cosmos.NewUint(250 * common.One),
				BalanceCacao: cosmos.NewUint(100 * common.One),
				LPUnits:      cosmos.NewUint(100 * common.One),
				SynthUnits:   cosmos.ZeroUint(),
			},
			lp: types.LiquidityProvider{
				Units:             cosmos.NewUint(common.One * 10),
				AssetDepositValue: cosmos.NewUint(common.One * 20),
				CacaoDepositValue: cosmos.NewUint(common.One * 10),
			},
			withdrawBasisPoint:   10000,
			height:               6_480_000,
			expectedILP:          cosmos.ZeroUint(),
			expectedDepositValue: cosmos.NewUint(1800000000),
			expectedRedeemValue:  cosmos.NewUint(2000000000),
		},
		{
			name: "CACAO-increases-in-price-relative-to-ASSET",
			pool: Pool{
				Asset:        common.BTCAsset,
				BalanceAsset: cosmos.NewUint(200 * common.One),
				BalanceCacao: cosmos.NewUint(50 * common.One),
				LPUnits:      cosmos.NewUint(100 * common.One),
				SynthUnits:   cosmos.ZeroUint(),
			},
			lp: types.LiquidityProvider{
				Units:             cosmos.NewUint(common.One * 10),
				AssetDepositValue: cosmos.NewUint(common.One * 20),
				CacaoDepositValue: cosmos.NewUint(common.One * 10),
			},
			withdrawBasisPoint:   10000,
			height:               6_480_000,
			expectedILP:          cosmos.NewUint(500000000),
			expectedDepositValue: cosmos.NewUint(1500000000),
			expectedRedeemValue:  cosmos.NewUint(1000000000),
		},
		{
			name: "ASSET-decreases-in-price-relative-to-CACAO",
			pool: Pool{
				Asset:        common.BTCAsset,
				BalanceAsset: cosmos.NewUint(150 * common.One),
				BalanceCacao: cosmos.NewUint(100 * common.One),
				LPUnits:      cosmos.NewUint(100 * common.One),
				SynthUnits:   cosmos.ZeroUint(),
			},
			lp: types.LiquidityProvider{
				Units:             cosmos.NewUint(common.One * 10),
				AssetDepositValue: cosmos.NewUint(common.One * 20),
				CacaoDepositValue: cosmos.NewUint(common.One * 10),
			},
			withdrawBasisPoint:   10000,
			height:               2_160_000,
			expectedILP:          cosmos.NewUint(333333333),
			expectedDepositValue: cosmos.NewUint(2333333333),
			expectedRedeemValue:  cosmos.NewUint(2000000000),
		},
		{
			name: "CACAO-decreases-in-price-relative-to-ASSET",
			pool: Pool{
				Asset:        common.BTCAsset,
				BalanceAsset: cosmos.NewUint(200 * common.One),
				BalanceCacao: cosmos.NewUint(150 * common.One),
				LPUnits:      cosmos.NewUint(100 * common.One),
				SynthUnits:   cosmos.ZeroUint(),
			},
			lp: types.LiquidityProvider{
				Units:             cosmos.NewUint(common.One * 10),
				AssetDepositValue: cosmos.NewUint(common.One * 20),
				CacaoDepositValue: cosmos.NewUint(common.One * 10),
			},
			withdrawBasisPoint:   10000,
			height:               2_160_000,
			expectedILP:          cosmos.ZeroUint(),
			expectedDepositValue: cosmos.NewUint(2500000000),
			expectedRedeemValue:  cosmos.NewUint(3000000000),
		},
		{
			name: "half-coverage",
			pool: Pool{
				Asset:        common.BTCAsset,
				BalanceAsset: cosmos.NewUint(100 * common.One),
				BalanceCacao: cosmos.NewUint(100 * common.One),
				LPUnits:      cosmos.NewUint(100 * common.One),
				SynthUnits:   cosmos.ZeroUint(),
			},
			lp: types.LiquidityProvider{
				Units:             cosmos.NewUint(12345678),
				AssetDepositValue: cosmos.NewUint(common.One),
				CacaoDepositValue: cosmos.NewUint(common.One * 50),
			},
			withdrawBasisPoint:   10000,
			height:               3_600_000,
			expectedILP:          cosmos.NewUint(2537654322),
			expectedDepositValue: cosmos.NewUint(5100000000),
			expectedRedeemValue:  cosmos.NewUint(24691356),
		},
		{
			name: "no-panic",
			pool: Pool{
				Asset:        common.BTCAsset,
				BalanceAsset: cosmos.NewUint(100 * common.One),
				BalanceCacao: cosmos.NewUint(100 * common.One),
				LPUnits:      cosmos.NewUintFromString("56641523781457101833101355"),
				SynthUnits:   cosmos.NewUintFromString("14364006570060942596417271288417281565500"),
			},
			lp: types.LiquidityProvider{
				Units:             cosmos.NewUintFromString("56641523781433101833101355"),
				AssetDepositValue: cosmos.NewUint(common.One),
				CacaoDepositValue: cosmos.NewUint(common.One * 50),
			},
			withdrawBasisPoint:   10000,
			height:               2_160_000,
			expectedILP:          cosmos.ZeroUint(),
			expectedDepositValue: cosmos.ZeroUint(),
			expectedRedeemValue:  cosmos.ZeroUint(),
		},
	}
	for _, tc := range testCases {
		c.Logf("name:%s", tc.name)
		ctx, mgr := setupManagerForTest(c)
		ctx = ctx.WithBlockHeight(tc.height)
		ilpCacao, depositValue, redeemValue := calcImpLossV102(ctx, mgr, 0, tc.lp, cosmos.NewUint(uint64(tc.withdrawBasisPoint)), 2_160_000, tc.pool)
		c.Check(tc.expectedILP.Equal(ilpCacao), Equals, true, Commentf("expected %s, got %s", tc.expectedILP.String(), ilpCacao.String()))
		c.Check(tc.expectedDepositValue.Equal(depositValue), Equals, true, Commentf("expected %s, got %s", tc.expectedDepositValue.String(), depositValue.String()))
		c.Check(tc.expectedRedeemValue.Equal(redeemValue), Equals, true, Commentf("expected %s, got %s", tc.expectedRedeemValue.String(), redeemValue.String()))
	}
}

func (s WithdrawSuiteV102) TestAssetToWithdrawV102(c *C) {
	msg := MsgWithdrawLiquidity{
		Asset:           common.BTCAsset,
		WithdrawalAsset: common.BTCAsset,
	}
	lp := LiquidityProvider{
		CacaoAddress: GetRandomBaseAddress(),
		AssetAddress: GetRandomBTCAddress(),
	}

	asset := assetToWithdrawV89(msg, lp, 0)
	c.Assert(asset.Equals(common.BTCAsset), Equals, true)
	asset = assetToWithdrawV89(msg, lp, 1)
	c.Assert(asset.Equals(common.EmptyAsset), Equals, true)

	lp.AssetAddress = common.NoAddress
	asset = assetToWithdrawV89(msg, lp, 0)
	c.Assert(asset.Equals(common.BaseAsset()), Equals, true)

	lp.AssetAddress = GetRandomBTCAddress()
	lp.CacaoAddress = common.NoAddress
	msg.WithdrawalAsset = common.EmptyAsset
	asset = assetToWithdrawV89(msg, lp, 0)
	c.Assert(asset.Equals(common.BTCAsset), Equals, true)
}

func (WithdrawSuiteV102) TestcheckWithdrawLimit(c *C) {
	ctx, mgr := setupManagerForTest(c)

	accountAddr := GetRandomValidatorNode(NodeWhiteListed).NodeAddress
	testCases := []struct {
		name                string
		msg                 MsgWithdrawLiquidity
		withdrawCounterIn   cosmos.Uint
		withdrawCounterOut  cosmos.Uint
		expectedToBeLimited bool
		expectedError       error
		lpTier              int64
		heightAfterDay      int64
	}{
		{
			name: "no-withdraw-limit-because-of-tier",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: GetRandomBaseAddress(),
				BasisPoints:     cosmos.NewUint(100),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
			},
			expectedToBeLimited: false,
			expectedError:       nil,
			lpTier:              0,
		},
		{
			name: "no-withdraw-limit-because-of-ragnarok",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: GetRandomBaseAddress(),
				BasisPoints:     cosmos.NewUint(100),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE", Memo: "Ragnarok"},
				Signer:          accountAddr,
			},
			expectedToBeLimited: false,
			expectedError:       nil,
			lpTier:              1,
		},
		{
			name: "max-withdraw-already-reach",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: GetRandomBaseAddress(),
				BasisPoints:     cosmos.NewUint(100),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
			},
			withdrawCounterIn:   cosmos.NewUint(50),
			expectedToBeLimited: true,
			expectedError:       errMaxWithdrawReach,
			lpTier:              1,
		},
		{
			name: "max-withdraw-will-be-reach",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: GetRandomBaseAddress(),
				BasisPoints:     cosmos.NewUint(100),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
			},
			withdrawCounterIn:   cosmos.NewUint(40),
			expectedToBeLimited: true,
			expectedError:       errMaxWithdrawWillBeReach,
			lpTier:              1,
		},
		{
			name: "withdraw-tier-1",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: GetRandomBaseAddress(),
				BasisPoints:     cosmos.NewUint(10),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
			},
			withdrawCounterIn:   cosmos.NewUint(40),
			withdrawCounterOut:  cosmos.NewUint(50),
			expectedToBeLimited: true,
			expectedError:       nil,
			lpTier:              1,
		},
		{
			name: "withdraw-tier-2",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: GetRandomBaseAddress(),
				BasisPoints:     cosmos.NewUint(10),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
			},
			withdrawCounterIn:   cosmos.NewUint(140),
			withdrawCounterOut:  cosmos.NewUint(150),
			expectedToBeLimited: true,
			expectedError:       nil,
			lpTier:              2,
		},
		{
			name: "withdraw-tier-3",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: GetRandomBaseAddress(),
				BasisPoints:     cosmos.NewUint(10),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
			},
			withdrawCounterIn:   cosmos.NewUint(440),
			withdrawCounterOut:  cosmos.NewUint(450),
			expectedToBeLimited: true,
			expectedError:       nil,
			lpTier:              3,
		},
		{
			name: "no-withdraw-limit-because-of-tier",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: GetRandomBaseAddress(),
				BasisPoints:     cosmos.NewUint(100),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
			},
			expectedToBeLimited: false,
			expectedError:       nil,
			lpTier:              0,
		},
		{
			name: "reset-counter-after-24-hrs",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: GetRandomBaseAddress(),
				BasisPoints:     cosmos.NewUint(40),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
			},
			withdrawCounterIn:   cosmos.NewUint(50),
			withdrawCounterOut:  cosmos.NewUint(40),
			expectedToBeLimited: true,
			expectedError:       nil,
			lpTier:              3,
			heightAfterDay:      14400,
		},
	}
	for _, tc := range testCases {
		c.Logf("name:%s", tc.name)
		mgr.K = getWithdrawTestKeeper(c, ctx, mgr.Keeper(), tc.msg.WithdrawAddress, tc.lpTier, tc.withdrawCounterIn)

		if tc.heightAfterDay == 14400 {
			ctx = ctx.WithBlockHeight(tc.heightAfterDay)
		}
		mgr.Keeper().SetMimir(ctx, constants.LiquidityAuction.String(), 17)

		lpBefore, err := mgr.K.GetLiquidityProvider(ctx, common.BNBAsset, tc.msg.WithdrawAddress)
		c.Assert(err, IsNil)
		lpAfter, err := checkWithdrawLimitV102(ctx, mgr, tc.msg, mgr.constAccessor, lpBefore)

		if tc.expectedError != nil {
			c.Assert(err, NotNil)
			c.Check(err.Error(), Equals, tc.expectedError.Error())
			areLpsEqualAssert(c, lpBefore, lpAfter)
			continue
		}
		if !tc.expectedToBeLimited {
			c.Assert(err, IsNil)
			areLpsEqualAssert(c, lpBefore, lpAfter)
			continue
		}
		// Normal withdraw limit for tier 1, 2 and 3
		c.Assert(err, IsNil)
		c.Assert(lpAfter.WithdrawCounter.Equal(tc.withdrawCounterOut), Equals, true)
	}
}

func (s WithdrawSuiteV102) TestAssetToWithdrawV89(c *C) {
	msg := MsgWithdrawLiquidity{
		Asset:           common.BTCAsset,
		WithdrawalAsset: common.BTCAsset,
	}
	lp := LiquidityProvider{
		CacaoAddress: GetRandomBaseAddress(),
		AssetAddress: GetRandomBTCAddress(),
	}

	asset := assetToWithdrawV89(msg, lp, 0)
	c.Assert(asset.Equals(common.BTCAsset), Equals, true)
	asset = assetToWithdrawV89(msg, lp, 1)
	c.Assert(asset.Equals(common.EmptyAsset), Equals, true)

	lp.AssetAddress = common.NoAddress
	asset = assetToWithdrawV89(msg, lp, 0)
	c.Assert(asset.Equals(common.BaseAsset()), Equals, true)

	lp.AssetAddress = GetRandomBTCAddress()
	lp.CacaoAddress = common.NoAddress
	msg.WithdrawalAsset = common.EmptyAsset
	asset = assetToWithdrawV89(msg, lp, 0)
	c.Assert(asset.Equals(common.BTCAsset), Equals, true)
}

func (WithdrawSuiteV102) TestWithdrawSynth(c *C) {
	accountAddr := GetRandomValidatorNode(NodeActive).NodeAddress
	ctx, mgr := setupManagerForTest(c)
	asset := common.BTCAsset.GetSyntheticAsset()

	coin := common.NewCoin(asset, cosmos.NewUint(100*common.One))
	c.Assert(mgr.Keeper().MintToModule(ctx, ModuleName, coin), IsNil)
	c.Assert(mgr.Keeper().SendFromModuleToModule(ctx, ModuleName, AsgardName, common.NewCoins(coin)), IsNil)

	pool := Pool{
		BalanceCacao: cosmos.ZeroUint(),
		BalanceAsset: coin.Amount,
		Asset:        asset,
		LPUnits:      cosmos.NewUint(200 * common.One),
		Status:       PoolAvailable,
	}
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)
	lp := LiquidityProvider{
		Asset:              asset,
		CacaoAddress:       common.NoAddress,
		AssetAddress:       GetRandomBaseAddress(),
		LastAddHeight:      0,
		LastWithdrawHeight: 0,
		Units:              cosmos.NewUint(100 * common.One),
		PendingCacao:       cosmos.ZeroUint(),
		PendingAsset:       cosmos.ZeroUint(),
		PendingTxID:        GetRandomTxHash(),
	}
	mgr.Keeper().SetLiquidityProvider(ctx, lp)
	msg := MsgWithdrawLiquidity{
		WithdrawAddress: lp.AssetAddress,
		BasisPoints:     cosmos.NewUint(MaxWithdrawBasisPoints / 2),
		Asset:           asset,
		Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
		WithdrawalAsset: common.EmptyAsset,
		Signer:          accountAddr,
	}
	runeAmt, assetAmt, _, unitsLeft, gas, err := withdrawV102(ctx, msg, mgr)
	c.Assert(err, IsNil)
	c.Check(assetAmt.Uint64(), Equals, uint64(25*common.One), Commentf("%d", assetAmt.Uint64()))
	c.Check(runeAmt.IsZero(), Equals, true)
	c.Check(unitsLeft.Uint64(), Equals, uint64(50*common.One), Commentf("%d", unitsLeft.Uint64()))
	c.Check(gas.IsZero(), Equals, true)

	pool, err = mgr.Keeper().GetPool(ctx, asset)
	c.Assert(err, IsNil)
	c.Check(pool.BalanceCacao.Uint64(), Equals, uint64(0), Commentf("%d", pool.BalanceCacao.Uint64()))
	c.Check(pool.BalanceAsset.Uint64(), Equals, uint64(75*common.One), Commentf("%d", pool.BalanceAsset.Uint64()))
	c.Check(pool.LPUnits.Uint64(), Equals, uint64(150*common.One), Commentf("%d", pool.LPUnits.Uint64())) // LP units did decreased
}

func (WithdrawSuiteV102) TestWithdrawSynthSingleLP(c *C) {
	accountAddr := GetRandomValidatorNode(NodeActive).NodeAddress
	ctx, mgr := setupManagerForTest(c)
	asset := common.BTCAsset.GetSyntheticAsset()

	coin := common.NewCoin(asset, cosmos.NewUint(30*common.One))
	c.Assert(mgr.Keeper().MintToModule(ctx, ModuleName, coin), IsNil)
	c.Assert(mgr.Keeper().SendFromModuleToModule(ctx, ModuleName, AsgardName, common.NewCoins(coin)), IsNil)

	pool := Pool{
		BalanceCacao: cosmos.ZeroUint(),
		BalanceAsset: coin.Amount,
		Asset:        asset,
		LPUnits:      cosmos.NewUint(200 * common.One),
		Status:       PoolAvailable,
	}
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)
	lp := LiquidityProvider{
		Asset:              asset,
		CacaoAddress:       common.NoAddress,
		AssetAddress:       GetRandomBaseAddress(),
		LastAddHeight:      0,
		LastWithdrawHeight: 0,
		Units:              cosmos.NewUint(200 * common.One),
		PendingCacao:       cosmos.ZeroUint(),
		PendingAsset:       cosmos.ZeroUint(),
		PendingTxID:        GetRandomTxHash(),
	}
	mgr.Keeper().SetLiquidityProvider(ctx, lp)
	msg := MsgWithdrawLiquidity{
		WithdrawAddress: lp.AssetAddress,
		BasisPoints:     cosmos.NewUint(MaxWithdrawBasisPoints),
		Asset:           asset,
		Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
		WithdrawalAsset: common.EmptyAsset,
		Signer:          accountAddr,
	}
	runeAmt, assetAmt, _, unitsLeft, gas, err := withdrawV102(ctx, msg, mgr)
	c.Assert(err, IsNil)
	c.Check(assetAmt.Uint64(), Equals, coin.Amount.Uint64(), Commentf("%d", assetAmt.Uint64()))
	c.Check(runeAmt.IsZero(), Equals, true)
	c.Check(unitsLeft.Uint64(), Equals, uint64(200*common.One), Commentf("%d", unitsLeft.Uint64()))
	c.Check(gas.IsZero(), Equals, true)

	pool, err = mgr.Keeper().GetPool(ctx, asset)
	c.Check(err, IsNil)
	c.Check(pool.BalanceCacao.Uint64(), Equals, uint64(0), Commentf("%d", pool.BalanceCacao.Uint64()))
	c.Check(pool.BalanceAsset.Uint64(), Equals, uint64(0), Commentf("%d", pool.BalanceAsset.Uint64()))
	c.Check(pool.LPUnits.Uint64(), Equals, uint64(0), Commentf("%d", pool.LPUnits.Uint64())) // LP units did decreased
}
