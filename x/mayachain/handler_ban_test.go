package mayachain

import (
	"errors"

	"github.com/blang/semver"
	se "github.com/cosmos/cosmos-sdk/types/errors"
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
)

var _ = Suite(&HandlerBanSuite{})

type HandlerBanSuite struct{}

type TestBanKeeper struct {
	keeper.KVStoreDummy
	bond      cosmos.Uint
	lp1       LiquidityProvider
	lp2       LiquidityProvider
	lp3       LiquidityProvider
	pool      Pool
	ban       BanVoter
	toBan     NodeAccount
	toBanBP   BondProviders
	banner1   NodeAccount
	banner1BP BondProviders
	banner2   NodeAccount
	banner2BP BondProviders
	network   Network
	err       error
	modules   map[string]int64
}

func (k *TestBanKeeper) SendFromModuleToModule(_ cosmos.Context, from, to string, coins common.Coins) error {
	k.modules[from] -= int64(coins[0].Amount.Uint64())
	k.modules[to] += int64(coins[0].Amount.Uint64())
	return nil
}

func (k *TestBanKeeper) ListActiveValidators(_ cosmos.Context) (NodeAccounts, error) {
	return NodeAccounts{k.toBan, k.banner1, k.banner2}, k.err
}

func (k *TestBanKeeper) GetNodeAccount(_ cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error) {
	if addr.Equals(k.toBan.NodeAddress) {
		return k.toBan, k.err
	}
	if addr.Equals(k.banner1.NodeAddress) {
		return k.banner1, k.err
	}
	if addr.Equals(k.banner2.NodeAddress) {
		return k.banner2, k.err
	}
	return NodeAccount{}, errors.New("could not find node account, oops")
}

func (k *TestBanKeeper) SetNodeAccount(_ cosmos.Context, na NodeAccount) error {
	if na.NodeAddress.Equals(k.toBan.NodeAddress) {
		k.toBan = na
		return k.err
	}
	if na.NodeAddress.Equals(k.banner1.NodeAddress) {
		k.banner1 = na
		return k.err
	}
	if na.NodeAddress.Equals(k.banner2.NodeAddress) {
		k.banner2 = na
		return k.err
	}
	return k.err
}

func (k *TestBanKeeper) GetNetwork(ctx cosmos.Context) (Network, error) {
	return k.network, nil
}

func (k *TestBanKeeper) SetNetwork(ctx cosmos.Context, data Network) error {
	k.network = data
	return nil
}

func (k *TestBanKeeper) GetBanVoter(_ cosmos.Context, addr cosmos.AccAddress) (BanVoter, error) {
	return k.ban, k.err
}

func (k *TestBanKeeper) SetBanVoter(_ cosmos.Context, ban BanVoter) {
	k.ban = ban
}

func (k *TestBanKeeper) CalcNodeLiquidityBond(_ cosmos.Context, _ NodeAccount) (cosmos.Uint, error) {
	return k.bond, nil
}

func (k *TestBanKeeper) SetBondProviders(ctx cosmos.Context, bp BondProviders) error {
	if k.toBan.NodeAddress.Equals(bp.NodeAddress) {
		k.toBanBP = bp
	}
	if k.banner1.NodeAddress.Equals(bp.NodeAddress) {
		k.banner1BP = bp
	}
	if k.banner2.NodeAddress.Equals(bp.NodeAddress) {
		k.banner2BP = bp
	}

	return nil
}

func (k *TestBanKeeper) GetBondProviders(ctx cosmos.Context, add cosmos.AccAddress) (BondProviders, error) {
	if k.toBan.NodeAddress.Equals(add) {
		return k.toBanBP, nil
	}
	if k.banner1.NodeAddress.Equals(add) {
		return k.banner1BP, nil
	}
	if k.banner2.NodeAddress.Equals(add) {
		return k.banner2BP, nil
	}
	return NewBondProviders(add), nil
}

func (k *TestBanKeeper) SetLiquidityProvider(_ cosmos.Context, lp LiquidityProvider) {
	if lp.CacaoAddress.Equals(k.lp1.CacaoAddress) {
		k.lp1 = lp
	}
	if lp.CacaoAddress.Equals(k.lp2.CacaoAddress) {
		k.lp2 = lp
	}
	if lp.CacaoAddress.Equals(k.lp3.CacaoAddress) {
		k.lp3 = lp
	}
}

func (k *TestBanKeeper) GetLiquidityProvider(_ cosmos.Context, asset common.Asset, addr common.Address) (LiquidityProvider, error) {
	// change to switch
	switch addr {
	case k.lp1.CacaoAddress:
		return k.lp1, nil
	case k.lp2.CacaoAddress:
		return k.lp2, nil
	case k.lp3.CacaoAddress:
		return k.lp3, nil
	default:
		return LiquidityProvider{
			CacaoAddress: addr,
			Asset:        asset,
			Units:        cosmos.ZeroUint(),
		}, nil
	}
}

func (k *TestBanKeeper) GetLiquidityProviderByAssets(_ cosmos.Context, assets common.Assets, addr common.Address) (LiquidityProviders, error) {
	if k.lp1.CacaoAddress.Equals(addr) {
		return LiquidityProviders{k.lp1}, nil
	}
	if k.lp2.CacaoAddress.Equals(addr) {
		return LiquidityProviders{k.lp2}, nil
	}
	if k.lp3.CacaoAddress.Equals(addr) {
		return LiquidityProviders{k.lp3}, nil
	}
	return LiquidityProviders{}, nil
}

func (k *TestBanKeeper) SetLiquidityProviders(ctx cosmos.Context, lps LiquidityProviders) {
	for _, lp := range lps {
		switch lp.CacaoAddress {
		case k.lp1.CacaoAddress:
			k.lp1 = lp
		case k.lp2.CacaoAddress:
			k.lp2 = lp
		case k.lp3.CacaoAddress:
			k.lp3 = lp
		}
	}
}

func (k *TestBanKeeper) GetModuleAddress(module string) (common.Address, error) {
	return GetRandomBaseAddress(), nil
}

func (k *TestBanKeeper) GetPool(_ cosmos.Context, _ common.Asset) (Pool, error) {
	return k.pool, nil
}

func (k *TestBanKeeper) SetPool(_ cosmos.Context, pool Pool) error {
	k.pool = pool
	return nil
}

func (s *HandlerBanSuite) TestValidate(c *C) {
	ctx, _ := setupKeeperForTest(c)

	toBan := GetRandomValidatorNode(NodeActive)
	bp := NewBondProviders(toBan.NodeAddress)
	acc, err := toBan.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp.Providers = append(bp.Providers, NewBondProvider(acc))
	bp.Providers[0].Bonded = true

	banner1 := GetRandomValidatorNode(NodeActive)
	banner2 := GetRandomValidatorNode(NodeActive)

	keeper := &TestBanKeeper{
		toBan:   toBan,
		banner1: banner1,
		banner2: banner2,
		bond:    cosmos.NewUint(100 * common.One),
	}

	handler := NewBanHandler(NewDummyMgrWithKeeper(keeper))
	// happy path
	msg := NewMsgBan(toBan.NodeAddress, banner1.NodeAddress)
	err = handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	// invalid msg
	msg = &MsgBan{}
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
}

func (s *HandlerBanSuite) TestHandle(c *C) {
	ctx, _ := setupKeeperForTest(c)
	constAccessor := constants.GetConstantValues(GetCurrentVersion())
	minBond := constAccessor.GetInt64Value(constants.MinimumBondInCacao)

	bond := cosmos.NewUint(uint64(minBond))
	toBan := GetRandomValidatorNode(NodeActive)
	toBanBP := NewBondProviders(toBan.NodeAddress)
	acc, err := toBan.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	toBanBP.Providers = append(toBanBP.Providers, NewBondProvider(acc))
	toBanBP.Providers[0].Bonded = true

	banner1 := GetRandomValidatorNode(NodeActive)
	banner1BP := NewBondProviders(banner1.NodeAddress)
	acc, err = banner1.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	banner1BP.Providers = append(banner1BP.Providers, NewBondProvider(acc))
	banner1BP.Providers[0].Bonded = true

	banner2 := GetRandomValidatorNode(NodeActive)
	banner2BP := NewBondProviders(banner2.NodeAddress)
	acc, err = banner2.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	banner2BP.Providers = append(banner2BP.Providers, NewBondProvider(acc))
	banner2BP.Providers[0].Bonded = true

	keeper := &TestBanKeeper{
		ban:       NewBanVoter(toBan.NodeAddress),
		toBan:     toBan,
		toBanBP:   toBanBP,
		banner1:   banner1,
		banner1BP: banner1BP,
		banner2:   banner2,
		banner2BP: banner2BP,
		lp1: LiquidityProvider{
			Asset:        common.BNBAsset,
			Units:        bond,
			CacaoAddress: common.Address(toBan.BondAddress.String()),
			AssetAddress: GetRandomBNBAddress(),
		},
		lp2: LiquidityProvider{
			Asset:        common.BNBAsset,
			Units:        bond,
			CacaoAddress: common.Address(banner1.BondAddress.String()),
			AssetAddress: GetRandomBNBAddress(),
		},
		lp3: LiquidityProvider{
			Asset:        common.BNBAsset,
			Units:        bond,
			CacaoAddress: common.Address(banner2.BondAddress.String()),
			AssetAddress: GetRandomBNBAddress(),
		},
		pool: Pool{
			Asset:        common.BNBAsset,
			LPUnits:      bond.MulUint64(3),
			Status:       PoolAvailable,
			BalanceCacao: bond.MulUint64(3),
			BalanceAsset: bond.MulUint64(3),
		},
		bond:    bond,
		network: NewNetwork(),
		modules: make(map[string]int64),
	}
	keeper.lp2.Bond(keeper.banner1.NodeAddress, keeper.lp2.Units)
	keeper.lp3.Bond(keeper.banner2.NodeAddress, keeper.lp3.Units)

	mgr := NewDummyMgrWithKeeper(keeper)
	mgr.slasher = newSlasherVCUR(keeper, NewDummyEventMgr())
	handler := NewBanHandler(mgr)

	// ban with banner 1
	msg := NewMsgBan(toBan.NodeAddress, banner1.NodeAddress)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	c.Check(int64(keeper.lp2.Units.Uint64()), Equals, int64(99900000))
	c.Check(keeper.toBan.ForcedToLeave, Equals, false)
	c.Check(keeper.ban.Signers, HasLen, 1)
	// ensure banner 1 can't ban twice
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	c.Check(int64(keeper.lp2.Units.Uint64()), Equals, int64(99900000))
	c.Check(keeper.toBan.ForcedToLeave, Equals, false)
	c.Check(keeper.ban.Signers, HasLen, 1)

	// ban with banner 2, which should actually ban the node account
	msg = NewMsgBan(toBan.NodeAddress, banner2.NodeAddress)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	c.Check(int64(keeper.lp3.Units.Uint64()), Equals, int64(99900000))
	c.Check(keeper.toBan.ForcedToLeave, Equals, true)
	c.Check(keeper.toBan.LeaveScore, Equals, uint64(1))
	c.Check(keeper.ban.Signers, HasLen, 2)
	c.Check(keeper.ban.BlockHeight, Equals, int64(18))
}

type TestBanKeeperHelper struct {
	keeper.Keeper
	banner                     NodeAccount
	toBanNodeAddr              cosmos.AccAddress
	bannerNodeAddr             cosmos.AccAddress
	failToGetToBanAddr         bool
	failToGetBannerNodeAccount bool
	failToListActiveValidators bool
	failToGetBanVoter          bool
	failToGetNetwork           bool
	failToSetNetwork           bool
	failToSaveBanner           bool
	failToSaveToBan            bool
}

func NewTestBanKeeperHelper(k keeper.Keeper) *TestBanKeeperHelper {
	return &TestBanKeeperHelper{
		Keeper: k,
	}
}

func (k *TestBanKeeperHelper) GetNodeAccount(ctx cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error) {
	if addr.Equals(k.toBanNodeAddr) && k.failToGetToBanAddr {
		return NodeAccount{}, errKaboom
	}
	if addr.Equals(k.bannerNodeAddr) && k.failToGetBannerNodeAccount {
		return NodeAccount{}, errKaboom
	}
	return k.Keeper.GetNodeAccount(ctx, addr)
}

func (k *TestBanKeeperHelper) ListActiveValidators(ctx cosmos.Context) (NodeAccounts, error) {
	if k.failToListActiveValidators {
		return NodeAccounts{}, errKaboom
	}
	return k.Keeper.ListActiveValidators(ctx)
}

func (k *TestBanKeeperHelper) GetBanVoter(ctx cosmos.Context, addr cosmos.AccAddress) (BanVoter, error) {
	if k.failToGetBanVoter {
		return BanVoter{}, errKaboom
	}
	return k.Keeper.GetBanVoter(ctx, addr)
}

func (k *TestBanKeeperHelper) GetNetwork(ctx cosmos.Context) (Network, error) {
	if k.failToGetNetwork {
		return Network{}, errKaboom
	}
	return k.Keeper.GetNetwork(ctx)
}

func (k *TestBanKeeperHelper) SetNetwork(ctx cosmos.Context, network Network) error {
	if k.failToSetNetwork {
		return errKaboom
	}
	return k.Keeper.SetNetwork(ctx, network)
}

func (k *TestBanKeeperHelper) SetNodeAccount(ctx cosmos.Context, na NodeAccount) error {
	if k.failToSaveBanner && na.NodeAddress.Equals(k.bannerNodeAddr) {
		return errKaboom
	}
	if k.failToSaveToBan && na.NodeAddress.Equals(k.toBanNodeAddr) {
		return errKaboom
	}
	return k.Keeper.SetNodeAccount(ctx, na)
}

func (s *HandlerBanSuite) TestBanHandlerValidation(c *C) {
	testCases := []struct {
		name              string
		messageProvider   func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg
		validator         func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string)
		skipForNativeRUNE bool
	}{
		{
			name: "invalid msg should return an error",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				return NewMsgNetworkFee(1024, common.BNBChain, 1, bnbSingleTxFee.Uint64(), GetRandomBech32Addr())
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
				c.Check(errors.Is(err, errInvalidMessage), Equals, true, Commentf(name))
			},
		},
		{
			name: "MsgBan failed validation should return error",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				return NewMsgBan(cosmos.AccAddress{}, GetRandomBech32Addr())
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
				c.Check(errors.Is(err, se.ErrInvalidAddress), Equals, true, Commentf(name))
			},
		},
		{
			name: "MsgBan not signed by an active account should return error",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				return NewMsgBan(GetRandomBech32Addr(), GetRandomBech32Addr())
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
				c.Check(errors.Is(err, se.ErrUnauthorized), Equals, true, Commentf(name))
			},
		},
		{
			name: "fail to get to ban node account should return an error",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				helper.failToGetToBanAddr = true
				return NewMsgBan(helper.toBanNodeAddr, helper.bannerNodeAddr)
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
				c.Check(errors.Is(err, errInternal), Equals, true, Commentf(name))
			},
		},
		{
			name: "to ban node account is not valid should return an error",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				return NewMsgBan(helper.toBanNodeAddr, helper.bannerNodeAddr)
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
		},
		{
			name: "to ban node account has been banned already should not do any thing",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				na := GetRandomValidatorNode(NodeActive)
				na.ForcedToLeave = true
				c.Assert(helper.SetNodeAccount(ctx, na), IsNil)
				return NewMsgBan(na.NodeAddress, helper.bannerNodeAddr)
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
		},
		{
			name: "ban an not active account should return an error",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				na := GetRandomValidatorNode(NodeReady)
				c.Assert(helper.SetNodeAccount(ctx, na), IsNil)
				return NewMsgBan(GetRandomValidatorNode(NodeActive).NodeAddress, na.NodeAddress)
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
		},
		{
			name: "banner is invalid return an error",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				toBanAcct := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, toBanAcct), IsNil)
				newBanner := helper.banner
				newBanner.BondAddress = common.NoAddress
				c.Assert(helper.SetNodeAccount(ctx, newBanner), IsNil)
				return NewMsgBan(toBanAcct.NodeAddress, helper.bannerNodeAddr)
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
		},
		{
			name: "fail to list active node account should return an error",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				toBanAcct := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, toBanAcct), IsNil)
				helper.failToListActiveValidators = true
				return NewMsgBan(toBanAcct.NodeAddress, helper.bannerNodeAddr)
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
		},
		{
			name: "fail to get ban voter should return an error",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				toBanAcct := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, toBanAcct), IsNil)
				helper.failToGetBanVoter = true
				return NewMsgBan(toBanAcct.NodeAddress, helper.bannerNodeAddr)
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
		},
		{
			name: "fail to get network data should return an error",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				toBanAcct := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, toBanAcct), IsNil)
				helper.failToGetNetwork = true
				return NewMsgBan(toBanAcct.NodeAddress, helper.bannerNodeAddr)
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
			skipForNativeRUNE: true,
		},
		{
			name: "fail to set network data should return an error",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				toBanAcct := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, toBanAcct), IsNil)
				helper.failToSetNetwork = true
				return NewMsgBan(toBanAcct.NodeAddress, helper.bannerNodeAddr)
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
			skipForNativeRUNE: true,
		},
		{
			name: "fail to save banner should return an error",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				toBanAcct := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, toBanAcct), IsNil)
				helper.failToSaveBanner = true
				return NewMsgBan(toBanAcct.NodeAddress, helper.bannerNodeAddr)
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
			skipForNativeRUNE: true,
		},
		{
			name: "when voter had been processed , it should not error",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				toBanAcct := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, toBanAcct), IsNil)
				voter, _ := helper.GetBanVoter(ctx, toBanAcct.NodeAddress)
				activeNode := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, activeNode), IsNil)
				voter.Sign(activeNode.NodeAddress)
				voter.BlockHeight = ctx.BlockHeight()
				helper.SetBanVoter(ctx, voter)
				return NewMsgBan(toBanAcct.NodeAddress, helper.bannerNodeAddr)
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
		},
		{
			name: "fail to save to ban account, it should return an error",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				toBanAcct := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, toBanAcct), IsNil)
				voter, _ := helper.GetBanVoter(ctx, toBanAcct.NodeAddress)
				activeNode := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, activeNode), IsNil)
				voter.Sign(activeNode.NodeAddress)
				helper.SetBanVoter(ctx, voter)
				helper.failToSaveToBan = true
				helper.toBanNodeAddr = toBanAcct.NodeAddress
				return NewMsgBan(toBanAcct.NodeAddress, helper.bannerNodeAddr)
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
		},
	}
	versions := []semver.Version{
		GetCurrentVersion(),
	}
	for _, tc := range testCases {
		if tc.skipForNativeRUNE {
			continue
		}
		for _, ver := range versions {
			ctx, mgr := setupManagerForTest(c)
			toBanAddr := GetRandomBech32Addr()
			banner := GetRandomValidatorNode(NodeActive)
			bannerNodeAddr := banner.NodeAddress
			c.Assert(mgr.Keeper().SetNodeAccount(ctx, banner), IsNil)
			helper := NewTestBanKeeperHelper(mgr.Keeper())
			helper.banner = banner
			helper.toBanNodeAddr = toBanAddr
			helper.bannerNodeAddr = bannerNodeAddr
			mgr.K = helper
			mgr.currentVersion = ver
			mgr.constAccessor = constants.GetConstantValues(ver)
			handler := NewBanHandler(mgr)
			result, err := handler.Run(ctx, tc.messageProvider(ctx, helper))
			tc.validator(c, result, err, helper, tc.name)
		}
	}
}
