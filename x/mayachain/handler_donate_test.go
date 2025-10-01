package mayachain

import (
	"errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	se "github.com/cosmos/cosmos-sdk/types/errors"
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
)

type HandlerDonateSuite struct{}

var _ = Suite(&HandlerDonateSuite{})

type HandlerDonateTestHelper struct {
	keeper.Keeper
	failToGetPool  bool
	failToSavePool bool
	mimir          map[string]int64
}

func NewHandlerDonateTestHelper(k keeper.Keeper) *HandlerDonateTestHelper {
	return &HandlerDonateTestHelper{
		Keeper: k,
		mimir:  make(map[string]int64),
	}
}

func (h *HandlerDonateTestHelper) GetPool(ctx cosmos.Context, asset common.Asset) (Pool, error) {
	if h.failToGetPool {
		return NewPool(), errKaboom
	}
	return h.Keeper.GetPool(ctx, asset)
}

func (h *HandlerDonateTestHelper) SetPool(ctx cosmos.Context, p Pool) error {
	if h.failToSavePool {
		return errKaboom
	}
	return h.Keeper.SetPool(ctx, p)
}

func (h *HandlerDonateTestHelper) GetMimir(_ sdk.Context, key string) (int64, error) {
	v, ok := h.mimir[key]
	if !ok {
		return -1, nil
	}
	return v, nil
}

func getLPs(c *C, k keeper.Keeper, ctx cosmos.Context) LiquidityProviders {
	var lps LiquidityProviders
	iterator := k.GetLiquidityProviderIterator(ctx, common.BNBAsset)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var lp LiquidityProvider
		k.Cdc().MustUnmarshal(iterator.Value(), &lp)
		lps = append(lps, lp)
	}
	return lps
}

func (HandlerDonateSuite) TestDonate(c *C) {
	w := getHandlerTestWrapper(c, 1, true, true)
	// happy path
	prePool, err := w.keeper.GetPool(w.ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	mgr := NewDummyMgrWithKeeper(w.keeper)
	donateHandler := NewDonateHandler(mgr)
	msg := NewMsgDonate(GetRandomTx(), common.BNBAsset, cosmos.NewUint(common.One*5), cosmos.NewUint(common.One*5), w.activeNodeAccount.NodeAddress)
	_, err = donateHandler.Run(w.ctx, msg)
	c.Assert(err, IsNil)
	afterPool, err := w.keeper.GetPool(w.ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Assert(afterPool.BalanceCacao.String(), Equals, prePool.BalanceCacao.Add(msg.CacaoAmount).String())
	c.Assert(afterPool.BalanceAsset.String(), Equals, prePool.BalanceAsset.Add(msg.AssetAmount).String())

	msgBan := NewMsgBan(GetRandomBech32Addr(), w.activeNodeAccount.NodeAddress)
	result, err := donateHandler.Run(w.ctx, msgBan)
	c.Check(err, NotNil)
	c.Check(errors.Is(err, errInvalidMessage), Equals, true)
	c.Check(result, IsNil)

	testKeeper := NewHandlerDonateTestHelper(w.keeper)
	testKeeper.failToGetPool = true
	donateHandler1 := NewDonateHandler(NewDummyMgrWithKeeper(testKeeper))
	result, err = donateHandler1.Run(w.ctx, msg)
	c.Check(err, NotNil)
	c.Check(errors.Is(err, errInternal), Equals, true)
	c.Check(result, IsNil)

	testKeeper = NewHandlerDonateTestHelper(w.keeper)
	testKeeper.failToSavePool = true
	donateHandler2 := NewDonateHandler(NewDummyMgrWithKeeper(testKeeper))
	result, err = donateHandler2.Run(w.ctx, msg)
	c.Check(err, NotNil)
	c.Check(errors.Is(err, errInternal), Equals, true)
	c.Check(result, IsNil)
}

func (HandlerDonateSuite) TestHandleMsgDonateValidation(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	testCases := []struct {
		name        string
		msg         *MsgDonate
		expectedErr error
	}{
		{
			name:        "invalid signer address should fail",
			msg:         NewMsgDonate(GetRandomTx(), common.BNBAsset, cosmos.NewUint(common.One*5), cosmos.NewUint(common.One*5), cosmos.AccAddress{}),
			expectedErr: se.ErrInvalidAddress,
		},
		{
			name:        "empty asset should fail",
			msg:         NewMsgDonate(GetRandomTx(), common.Asset{}, cosmos.NewUint(common.One*5), cosmos.NewUint(common.One*5), w.activeNodeAccount.NodeAddress),
			expectedErr: se.ErrUnknownRequest,
		},
		{
			name:        "pool doesn't exist should fail",
			msg:         NewMsgDonate(GetRandomTx(), common.BNBAsset, cosmos.NewUint(common.One*5), cosmos.NewUint(common.One*5), w.activeNodeAccount.NodeAddress),
			expectedErr: se.ErrUnknownRequest,
		},
		{
			name:        "synth asset should fail",
			msg:         NewMsgDonate(GetRandomTx(), common.BNBAsset.GetSyntheticAsset(), cosmos.NewUint(common.One*5), cosmos.NewUint(common.One*5), w.activeNodeAccount.NodeAddress),
			expectedErr: errInvalidMessage,
		},
	}

	donateHandler := NewDonateHandler(NewDummyMgrWithKeeper(w.keeper))
	for _, item := range testCases {
		_, err := donateHandler.Run(w.ctx, item.msg)
		c.Check(errors.Is(err, item.expectedErr), Equals, true, Commentf("name:%s", item.name))
	}
}

func (HandlerDonateSuite) TestDonateLiquidityAuction(c *C) {
	w := getHandlerTestWrapper(c, 10, false, false)
	testKeeper := NewHandlerDonateTestHelper(w.keeper)
	testKeeper.mimir[constants.LiquidityAuction.String()] = 10 // current height
	lpassets := make(map[common.Address]uint64)

	// Set pool
	p, err := testKeeper.GetPool(w.ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	p.Asset = common.BNBAsset
	p.Status = PoolAvailable
	p.BalanceCacao = cosmos.ZeroUint()
	p.BalanceAsset = cosmos.ZeroUint()
	p.LPUnits = cosmos.ZeroUint()
	p.PendingInboundAsset = cosmos.NewUint(150 * common.One)
	c.Assert(testKeeper.SetPool(w.ctx, p), IsNil)

	// Set LP's
	lpsIdx := make(map[common.Address]int)
	for i := 1; i <= 5; i++ {
		na := GetRandomValidatorNode(NodeActive)
		c.Assert(testKeeper.SetNodeAccount(w.ctx, na), IsNil)

		btcProvider := NewBondProvider(GetRandomBech32Addr())
		btcProvider.Bonded = true
		bnbProvider := NewBondProvider(GetRandomBech32Addr())
		bp := NewBondProviders(na.NodeAddress)
		bp.Providers = []BondProvider{
			btcProvider,
			bnbProvider,
		}
		c.Assert(testKeeper.SetBondProviders(w.ctx, bp), IsNil)

		asset := cosmos.NewUint(uint64(i) * 10 * common.One)
		bnbLP := LiquidityProvider{
			Asset:        common.BNBAsset,
			CacaoAddress: GetRandomBaseAddress(),
			AssetAddress: GetRandomBNBAddress(),
			PendingAsset: asset,
			PendingCacao: cosmos.ZeroUint(),
			Units:        cosmos.ZeroUint(),
		}
		lpassets[bnbLP.CacaoAddress] = asset.Uint64()
		testKeeper.SetLiquidityProvider(w.ctx, bnbLP)
		lpsIdx[bnbLP.CacaoAddress] = i
		switch i {
		case 1, 2:
			err = testKeeper.SetLiquidityAuctionTier(w.ctx, bnbLP.CacaoAddress, 1)
		case 3, 4:
			err = testKeeper.SetLiquidityAuctionTier(w.ctx, bnbLP.CacaoAddress, 2)
		case 5:
			err = testKeeper.SetLiquidityAuctionTier(w.ctx, bnbLP.CacaoAddress, 3)
		}
	}

	if err != nil {
		c.Error(err)
	}

	lps := getLPs(c, testKeeper, w.ctx)

	// At the beginning we should have PendingCacao, CacaoDepositValue, AssetDepositValue and Units equal to zero.
	for _, lp := range lps {
		c.Assert(lp.PendingCacao.IsZero(), Equals, true)
		c.Assert(lp.PendingAsset.Uint64(), Equals, lpassets[lp.CacaoAddress], Commentf("expected %d got %d", lpassets[lp.CacaoAddress], lp.PendingAsset.Uint64()))
		c.Assert(lp.CacaoDepositValue.IsZero(), Equals, true)
		c.Assert(lp.AssetDepositValue.IsZero(), Equals, true)
		c.Assert(lp.Units.IsZero(), Equals, true)
	}

	mgr := NewDummyMgrWithKeeper(testKeeper)
	donateHandler := NewDonateHandler(mgr)
	msg := NewMsgDonate(GetRandomTx(), common.BNBAsset, cosmos.NewUint(100*common.One), cosmos.ZeroUint(), w.activeNodeAccount.NodeAddress)

	c.Assert(testKeeper.SetPool(w.ctx, p), IsNil)
	_, err = donateHandler.Run(w.ctx, msg)
	c.Assert(err.Error(), Equals, "only admin can donate to liquidity auction: unauthorized")

	acc, err := cosmos.AccAddressFromBech32(ADMINS[0])
	c.Assert(err, IsNil)
	msg.Signer = acc
	_, err = donateHandler.Run(w.ctx, msg)
	c.Assert(err, IsNil)

	lps = getLPs(c, testKeeper, w.ctx)
	// After we donate we should have something on CacaoDepositValue, AssetDepositValue and Units.
	// PendingCacao and PendingAsset should now be zero.
	// 10, 20, 30, 40 and 50 asset and we're donating 100 cacao.
	totalAssetClaim := cosmos.ZeroUint()
	totalCacaoClaim := cosmos.ZeroUint()
	for _, lp := range lps {
		c.Log("lp ", lpsIdx[lp.CacaoAddress])
		var lpClaim cosmos.Uint
		var lpAssetClaim cosmos.Uint
		switch lpsIdx[lp.CacaoAddress] {
		case 1:
			// lp 1 (tier 1) = 6.66% + 33.33%*(33%*33% + 26%*10 + 20%*10) = 6.66% + 33.33%*(10.89% + 2.6% + 2%) = 6.66% + 33.33%*(15.49%) = 6.66% + 5.1% = 11.76%
			lpClaim = cosmos.NewUint(11_88888890)
			// lp 1 (tier 1) = 11_88888890 / 100_00000000 * 150_00000000 = 17_83333385
			lpAssetClaim = cosmos.NewUint(17_83333335)
		case 2:
			// lp 2 (tier 1) = 13.33% + 66.66%*(33%*33% + 26%*10 + 20%*10) = 13.33% + 66.66%*(10.89% + 2.6% + 2%) = 13.33% + 66.66%*(15.49%) = 13.33% + 10.2% = 23.53%
			lpClaim = cosmos.NewUint(23_77777777)
			// lp 2 (tier 1) = 23_77777777 / 100_00000000 * 150_00000000 = 35_66666765
			lpAssetClaim = cosmos.NewUint(35_66666666)
		case 3:
			// lp 3 (tier 2) = 20% - 20%*10 = 20% - 2% = 18%
			lpClaim = cosmos.NewUint(18 * common.One)
			// lp 3 (tier 2) = 18_00000000 / 100_00000000 * 150_00000000 = 27_00000000
			lpAssetClaim = cosmos.NewUint(27 * common.One)
		case 4:
			// lp 4 (tier 2) = 26.66% - 26.66%*10 = 26.66% - 2.66% = 24%
			lpClaim = cosmos.NewUint(24 * common.One)
			// lp 4 (tier 2) = 24_00000000 / 100_00000000 * 150_00000000 = 36_00000000
			lpAssetClaim = cosmos.NewUint(36 * common.One)
		case 5:
			// lp 4 (tier 3) = 33.33% - 33.33%*33% = 33.33% - 10.89% = 22.44%
			lpClaim = cosmos.NewUint(22_33333333)
			// lp 4 (tier 3) = 22_33333333 / 100_00000000 * 150_00000000 = 33_50000000
			lpAssetClaim = cosmos.NewUint(33_50000000)
		}
		// sum of all lpAssetClaim should be 100_00000000
		totalAssetClaim = totalAssetClaim.Add(lpAssetClaim)
		// sum of all lpClaim should be 150_00000000
		totalCacaoClaim = totalCacaoClaim.Add(lpClaim)

		c.Assert(lp.PendingCacao.IsZero(), Equals, true)
		c.Assert(lp.PendingAsset.IsZero(), Equals, true)
		c.Assert(lp.CacaoDepositValue.Uint64(), Equals, lp.Units.Uint64())
		c.Check(lp.Units.Uint64(), Equals, lpClaim.Uint64(), Commentf("expected %d got %d", lpClaim.Uint64(), lp.Units.Uint64()))
		c.Check(lp.AssetDepositValue.Uint64(), Equals, lpAssetClaim.Uint64(), Commentf("expected %d got %d", lpAssetClaim.Uint64(), lp.AssetDepositValue.Uint64()))
	}
	c.Assert(totalAssetClaim.Uint64(), Equals, uint64(150*common.One+1), Commentf("expected %d got %d", 150*common.One, totalAssetClaim.Uint64()))
	c.Assert(totalCacaoClaim.Uint64(), Equals, uint64(100*common.One), Commentf("expected %d got %d", 100*common.One, totalCacaoClaim.Uint64()))

	// Check that the pool is updated with the 10*common.One pending asset from lp
	p, err = testKeeper.GetPool(w.ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Assert(p.PendingInboundAsset.IsZero(), Equals, true)
	c.Assert(p.BalanceCacao.Uint64(), Equals, p.LPUnits.Uint64())
	c.Assert(p.BalanceCacao.Uint64(), Equals, uint64(100*common.One))
	c.Assert(p.BalanceAsset.Uint64(), Equals, uint64(150*common.One))
}

func (HandlerDonateSuite) TestDonateLiquidityAuctionBonded(c *C) {
	w := getHandlerTestWrapper(c, 10, false, false)
	testKeeper := NewHandlerDonateTestHelper(w.keeper)
	testKeeper.mimir[constants.LiquidityAuction.String()] = 19

	// Set pool
	p, err := testKeeper.GetPool(w.ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	p.Asset = common.BNBAsset
	p.BalanceCacao = cosmos.NewUint(common.One)
	p.BalanceAsset = cosmos.NewUint(common.One)
	p.LPUnits = cosmos.NewUint(common.One)
	p.PendingInboundAsset = cosmos.NewUint(40 * common.One)
	p.Status = PoolAvailable
	c.Assert(testKeeper.SetPool(w.ctx, p), IsNil)

	// Set LP's
	for i := 0; i < 4; i++ {
		na := GetRandomValidatorNode(NodeActive)
		c.Assert(testKeeper.SetNodeAccount(w.ctx, na), IsNil)

		btcProvider := NewBondProvider(GetRandomBech32Addr())
		btcProvider.Bonded = true
		bnbProvider := NewBondProvider(GetRandomBech32Addr())
		bp := NewBondProviders(na.NodeAddress)
		bp.Providers = []BondProvider{
			btcProvider,
			bnbProvider,
		}
		c.Assert(testKeeper.SetBondProviders(w.ctx, bp), IsNil)

		btcLP := LiquidityProvider{
			Asset:           common.BTCAsset,
			NodeBondAddress: na.NodeAddress,
			CacaoAddress:    common.Address(btcProvider.BondAddress.String()),
			AssetAddress:    GetRandomBTCAddress(),
			PendingAsset:    cosmos.NewUint(10 * common.One),
			PendingCacao:    cosmos.ZeroUint(),
			Units:           cosmos.ZeroUint(),
		}
		testKeeper.SetLiquidityProvider(w.ctx, btcLP)
	}

	lps := getLPs(c, testKeeper, w.ctx)

	// At the beginning we should have PendingCacao, CacaoDepositValue, AssetDepositValue and Units equal to zero.
	for _, lp := range lps {
		c.Assert(lp.PendingCacao.IsZero(), Equals, true)
		c.Assert(lp.PendingAsset.Equal(cosmos.NewUint(10*common.One)), Equals, true)
		c.Assert(lp.CacaoDepositValue.IsZero(), Equals, true)
		c.Assert(lp.AssetDepositValue.IsZero(), Equals, true)
		c.Assert(lp.Units.IsZero(), Equals, true)
	}

	mgr := NewDummyMgrWithKeeper(testKeeper)
	donateHandler := NewDonateHandler(mgr)
	acc, err := cosmos.AccAddressFromBech32(ADMINS[0])
	c.Assert(err, IsNil)

	msg := NewMsgDonate(GetRandomTx(), common.BNBAsset, cosmos.NewUint(10*common.One), cosmos.ZeroUint(), acc)
	_, err = donateHandler.Run(w.ctx, msg)
	c.Assert(err, IsNil)

	lps = getLPs(c, testKeeper, w.ctx)

	// After we donate we should have the same values as before due the LP's already been bonded.
	for _, lp := range lps {
		c.Assert(lp.PendingCacao.IsZero(), Equals, true)
		c.Assert(lp.PendingAsset.Equal(cosmos.NewUint(10*common.One)), Equals, true)
		c.Assert(lp.CacaoDepositValue.IsZero(), Equals, true)
		c.Assert(lp.AssetDepositValue.IsZero(), Equals, true)
		c.Assert(lp.Units.IsZero(), Equals, true)
	}
}
