package keeperv1

import (
	"github.com/blang/semver"
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

type KeeperNodeAccountSuite struct{}

var _ = Suite(&KeeperNodeAccountSuite{})

func (s *KeeperNodeAccountSuite) TestNodeAccount(c *C) {
	ctx, k := setupKeeperForTest(c)
	ctx = ctx.WithBlockHeight(10)

	na1 := GetRandomValidatorNode(NodeActive)
	acc1, err := na1.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp1 := NewBondProviders(na1.NodeAddress)
	bp1.Providers = append(bp1.Providers, NewBondProvider(acc1))
	bp1.Providers[0].Bonded = true
	lp, _ := SetupLiquidityBondForTest(c, ctx, k, common.BNBAsset, na1.BondAddress, cosmos.NewUint(100*common.One))
	lp.Bond(na1.NodeAddress, cosmos.NewUint(100*common.One))
	k.SetLiquidityProvider(ctx, lp)

	na2 := GetRandomValidatorNode(NodeStandby)
	acc2, err := na1.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp2 := NewBondProviders(na2.NodeAddress)
	bp2.Providers = append(bp2.Providers, NewBondProvider(acc2))
	bp2.Providers[0].Bonded = true
	lp, _ = SetupLiquidityBondForTest(c, ctx, k, common.BNBAsset, na2.BondAddress, cosmos.NewUint(100*common.One))
	lp.Bond(na2.NodeAddress, cosmos.NewUint(100*common.One))
	k.SetLiquidityProvider(ctx, lp)

	na3 := GetRandomVaultNode(NodeActive)
	acc3, err := na1.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp3 := NewBondProviders(na3.NodeAddress)
	bp3.Providers = append(bp3.Providers, NewBondProvider(acc3))
	bp3.Providers[0].Bonded = true
	lp, _ = SetupLiquidityBondForTest(c, ctx, k, common.BNBAsset, na3.BondAddress, cosmos.NewUint(100*common.One))
	lp.Bond(na2.NodeAddress, cosmos.NewUint(100*common.One))
	k.SetLiquidityProvider(ctx, lp)

	c.Assert(k.SetNodeAccount(ctx, na1), IsNil)
	c.Assert(k.SetBondProviders(ctx, bp1), IsNil)
	c.Assert(k.SetNodeAccount(ctx, na2), IsNil)
	c.Assert(k.SetBondProviders(ctx, bp2), IsNil)
	c.Assert(k.SetNodeAccount(ctx, na3), IsNil)
	c.Assert(k.SetBondProviders(ctx, bp3), IsNil)
	c.Check(na1.ActiveBlockHeight, Equals, int64(10))
	c.Check(na2.ActiveBlockHeight, Equals, int64(0))

	count, err := k.TotalActiveValidators(ctx)
	c.Assert(err, IsNil)
	c.Check(count, Equals, 1)

	na, err := k.GetNodeAccount(ctx, na1.NodeAddress)
	c.Assert(err, IsNil)
	c.Check(na.Equals(na1), Equals, true)

	na, err = k.GetNodeAccountByPubKey(ctx, na1.PubKeySet.Secp256k1)
	c.Assert(err, IsNil)
	c.Check(na.Equals(na1), Equals, true)

	valCon := "im unique!"
	pubkeys := GetRandomPubKeySet()
	err = k.EnsureNodeKeysUnique(ctx, na1.ValidatorConsPubKey, common.EmptyPubKeySet)
	c.Assert(err, NotNil)
	err = k.EnsureNodeKeysUnique(ctx, "", pubkeys)
	c.Assert(err, NotNil)
	err = k.EnsureNodeKeysUnique(ctx, na1.ValidatorConsPubKey, pubkeys)
	c.Assert(err, NotNil)
	err = k.EnsureNodeKeysUnique(ctx, valCon, na1.PubKeySet)
	c.Assert(err, NotNil)
	err = k.EnsureNodeKeysUnique(ctx, valCon, pubkeys)
	c.Assert(err, IsNil)
	addr := GetRandomBech32Addr()
	na, err = k.GetNodeAccount(ctx, addr)
	c.Assert(err, IsNil)
	c.Assert(na.Status, Equals, NodeUnknown)
	c.Assert(na.ValidatorConsPubKey, Equals, "")
	nodeAccounts, err := k.ListValidatorsWithBond(ctx)
	c.Check(err, IsNil)
	c.Check(nodeAccounts.Len() > 0 && nodeAccounts.Len() < 3, Equals, true)
}

func (s *KeeperNodeAccountSuite) TestGetMinJoinVersion(c *C) {
	type nodeInfo struct {
		status  NodeStatus
		version semver.Version
	}
	inputs := []struct {
		nodeInfoes            []nodeInfo
		expectedVersion       semver.Version
		expectedActiveVersion semver.Version
	}{
		{
			nodeInfoes: []nodeInfo{
				{
					status:  NodeActive,
					version: semver.MustParse("0.2.0"),
				},
				{
					status:  NodeActive,
					version: semver.MustParse("0.3.0"),
				},
				{
					status:  NodeActive,
					version: semver.MustParse("0.3.0"),
				},
				{
					status:  NodeStandby,
					version: semver.MustParse("0.2.0"),
				},
				{
					status:  NodeStandby,
					version: semver.MustParse("0.2.0"),
				},
			},
			expectedVersion:       semver.MustParse("0.3.0"),
			expectedActiveVersion: semver.MustParse("0.2.0"),
		},
		{
			nodeInfoes: []nodeInfo{
				{
					status:  NodeActive,
					version: semver.MustParse("0.2.0"),
				},
				{
					status:  NodeActive,
					version: semver.MustParse("1.3.0"),
				},
				{
					status:  NodeActive,
					version: semver.MustParse("0.3.0"),
				},
				{
					status:  NodeStandby,
					version: semver.MustParse("0.2.0"),
				},
				{
					status:  NodeStandby,
					version: semver.MustParse("0.2.0"),
				},
			},
			expectedVersion:       semver.MustParse("0.3.0"),
			expectedActiveVersion: semver.MustParse("0.2.0"),
		},
		{
			nodeInfoes: []nodeInfo{
				{
					status:  NodeActive,
					version: semver.MustParse("0.2.0"),
				},
				{
					status:  NodeActive,
					version: semver.MustParse("1.3.0"),
				},
				{
					status:  NodeActive,
					version: semver.MustParse("0.3.0"),
				},
				{
					status:  NodeActive,
					version: semver.MustParse("0.2.0"),
				},
				{
					status:  NodeActive,
					version: semver.MustParse("0.2.0"),
				},
			},
			expectedVersion:       semver.MustParse("0.2.0"),
			expectedActiveVersion: semver.MustParse("0.2.0"),
		},
		{
			nodeInfoes: []nodeInfo{
				{
					status:  NodeActive,
					version: semver.MustParse("0.79.0"),
				},
				{
					status:  NodeActive,
					version: semver.MustParse("0.79.0"),
				},
				{
					status:  NodeActive,
					version: semver.MustParse("0.79.0"),
				},
				{
					status:  NodeActive,
					version: semver.MustParse("0.79.0+a"),
				},
				{
					status:  NodeActive,
					version: semver.MustParse("0.79.0+b"),
				},
			},
			expectedVersion:       semver.MustParse("0.79.0"),
			expectedActiveVersion: semver.MustParse("0.79.0"),
		},
		{
			nodeInfoes: []nodeInfo{
				{
					status:  NodeActive,
					version: semver.MustParse("0.79.0"),
				},
				{
					status:  NodeActive,
					version: semver.MustParse("0.79.0"),
				},
				{
					status:  NodeActive,
					version: semver.MustParse("0.79.0-c"),
				},
				{
					status:  NodeActive,
					version: semver.MustParse("0.79.0-a"),
				},
				{
					status:  NodeActive,
					version: semver.MustParse("0.79.0-b"),
				},
			},
			expectedVersion:       semver.MustParse("0.79.0-b"),
			expectedActiveVersion: semver.MustParse("0.79.0-a"),
		},
	}

	for _, item := range inputs {
		ctx, k := setupKeeperForTest(c)
		for _, ni := range item.nodeInfoes {
			na1 := GetRandomValidatorNode(ni.status)
			na1.Version = ni.version.String()
			c.Assert(k.SetNodeAccount(ctx, na1), IsNil)
		}
		c.Check(k.GetMinJoinVersion(ctx).Equals(item.expectedVersion), Equals, true, Commentf("%+v", k.GetMinJoinVersion(ctx)))
		c.Check(k.GetLowestActiveVersion(ctx).Equals(item.expectedActiveVersion), Equals, true)
	}
}

func (s *KeeperNodeAccountSuite) TestNodeAccountSlashPoints(c *C) {
	ctx, k := setupKeeperForTest(c)
	addr := GetRandomBech32Addr()

	pts, err := k.GetNodeAccountSlashPoints(ctx, addr)
	c.Assert(err, IsNil)
	c.Check(pts, Equals, int64(0))

	pts = 5
	k.SetNodeAccountSlashPoints(ctx, addr, pts)
	pts, err = k.GetNodeAccountSlashPoints(ctx, addr)
	c.Assert(err, IsNil)
	c.Check(pts, Equals, int64(5))

	c.Assert(k.IncNodeAccountSlashPoints(ctx, addr, 12), IsNil)
	pts, err = k.GetNodeAccountSlashPoints(ctx, addr)
	c.Assert(err, IsNil)
	c.Check(pts, Equals, int64(17))

	c.Assert(k.DecNodeAccountSlashPoints(ctx, addr, 7), IsNil)
	pts, err = k.GetNodeAccountSlashPoints(ctx, addr)
	c.Assert(err, IsNil)
	c.Check(pts, Equals, int64(10))
	k.ResetNodeAccountSlashPoints(ctx, GetRandomBech32Addr())
}

func (s *KeeperNodeAccountSuite) TestJail(c *C) {
	ctx, k := setupKeeperForTest(c)
	addr := GetRandomBech32Addr()

	jail, err := k.GetNodeAccountJail(ctx, addr)
	c.Assert(err, IsNil)
	c.Check(jail.NodeAddress.Equals(addr), Equals, true)
	c.Check(jail.ReleaseHeight, Equals, int64(0))
	c.Check(jail.Reason, Equals, "")

	// ensure setting it works
	err = k.SetNodeAccountJail(ctx, addr, 50, "foo")
	c.Assert(err, IsNil)
	jail, err = k.GetNodeAccountJail(ctx, addr)
	c.Assert(err, IsNil)
	c.Check(jail.NodeAddress.Equals(addr), Equals, true)
	c.Check(jail.ReleaseHeight, Equals, int64(50))
	c.Check(jail.Reason, Equals, "foo")

	// ensure we won't reduce sentence
	err = k.SetNodeAccountJail(ctx, addr, 20, "bar")
	c.Assert(err, IsNil)
	jail, err = k.GetNodeAccountJail(ctx, addr)
	c.Assert(err, IsNil)
	c.Check(jail.NodeAddress.Equals(addr), Equals, true)
	c.Check(jail.ReleaseHeight, Equals, int64(50))
	c.Check(jail.Reason, Equals, "foo")

	// ensure we can update
	err = k.SetNodeAccountJail(ctx, addr, 70, "bar")
	c.Assert(err, IsNil)
	jail, err = k.GetNodeAccountJail(ctx, addr)
	c.Assert(err, IsNil)
	c.Check(jail.NodeAddress.Equals(addr), Equals, true)
	c.Check(jail.ReleaseHeight, Equals, int64(70))
	c.Check(jail.Reason, Equals, "bar")
}

func (s *KeeperNodeAccountSuite) TestBondProviders(c *C) {
	acc := GetRandomBech32Addr()
	bp := NewBondProviders(acc)
	bp.NodeOperatorFee = cosmos.NewUint(2000)
	p := NewBondProvider(acc)
	p.Bonded = true
	bp.Providers = append(bp.Providers, p)
	c.Assert(bp.Providers, HasLen, 1)

	ctx, k := setupKeeperForTest(c)
	c.Assert(k.SetBondProviders(ctx, bp), IsNil)

	providers, err := k.GetBondProviders(ctx, acc)
	c.Assert(err, IsNil)
	c.Assert(providers.Providers, HasLen, 1)
}

func (s *KeeperNodeAccountSuite) TestCalcNodeLiquidityBond(c *C) {
	ctx, k := setupKeeperForTest(c)
	standByNodeAccount := GetRandomValidatorNode(NodeStandby)
	c.Assert(k.SetNodeAccount(ctx, standByNodeAccount), IsNil)

	btcProvider := NewBondProvider(GetRandomBech32Addr())
	btcProvider.Bonded = true
	bnbProvider := NewBondProvider(GetRandomBech32Addr())
	bp := NewBondProviders(standByNodeAccount.NodeAddress)
	bp.Providers = []BondProvider{
		btcProvider,
		bnbProvider,
	}
	c.Assert(k.SetBondProviders(ctx, bp), IsNil)

	liquidityBond, err := k.CalcNodeLiquidityBond(ctx, standByNodeAccount)
	c.Assert(err, IsNil)
	c.Assert(liquidityBond.Uint64(), Equals, uint64(0))

	bp.Providers[0].Bonded = true
	c.Assert(k.SetBondProviders(ctx, bp), IsNil)
	btcPool := NewPool()
	btcPool.Asset = common.BTCAsset
	btcPool.Status = PoolAvailable
	btcPool.BalanceCacao = cosmos.NewUint(10000 * common.One)
	btcPool.BalanceAsset = cosmos.NewUint(10000 * common.One)
	btcPool.LPUnits = cosmos.NewUint(10000 * common.One)
	c.Assert(k.SetPool(ctx, btcPool), IsNil)

	btcLP := LiquidityProvider{
		Asset:           common.BTCAsset,
		NodeBondAddress: standByNodeAccount.NodeAddress,
		CacaoAddress:    common.Address(btcProvider.BondAddress.String()),
		AssetAddress:    GetRandomBTCAddress(),
		PendingCacao:    cosmos.ZeroUint(),
		Units:           cosmos.NewUint(2000 * common.One),
	}
	k.SetLiquidityProvider(ctx, btcLP)

	liquidityBond, err = k.CalcNodeLiquidityBond(ctx, standByNodeAccount)
	c.Assert(err, IsNil)
	c.Assert(liquidityBond.Uint64(), Equals, uint64(4000*common.One), Commentf("%d\n", liquidityBond.Uint64()))

	bnbPool := NewPool()
	bnbPool.Asset = common.BNBAsset
	bnbPool.Status = PoolAvailable
	bnbPool.BalanceCacao = cosmos.NewUint(10000 * common.One)
	bnbPool.BalanceAsset = cosmos.NewUint(10000 * common.One)
	bnbPool.LPUnits = cosmos.NewUint(10000 * common.One)
	c.Assert(k.SetPool(ctx, bnbPool), IsNil)

	bnbLP := LiquidityProvider{
		Asset:        common.BNBAsset,
		CacaoAddress: common.Address(bnbProvider.BondAddress.String()),
		AssetAddress: GetRandomBTCAddress(),
		PendingCacao: cosmos.ZeroUint(),
		Units:        cosmos.NewUint(2000 * common.One),
	}
	k.SetLiquidityProvider(ctx, bnbLP)

	liquidityBond, err = k.CalcNodeLiquidityBond(ctx, standByNodeAccount)
	// Check it doesn't take into account if bond provider hasn't bonded
	c.Assert(err, IsNil)
	c.Assert(liquidityBond.Uint64(), Equals, uint64(4000*common.One), Commentf("%d\n", liquidityBond.Uint64()))

	// should increase bond after bp has bonded
	bnbLP.NodeBondAddress = standByNodeAccount.NodeAddress
	k.SetLiquidityProvider(ctx, bnbLP)
	bp.Providers[1].Bonded = true
	c.Assert(k.SetBondProviders(ctx, bp), IsNil)
	liquidityBond, err = k.CalcNodeLiquidityBond(ctx, standByNodeAccount)
	c.Assert(err, IsNil)
	c.Assert(liquidityBond.Uint64(), Equals, uint64(8000*common.One), Commentf("%d\n", liquidityBond.Uint64()))
}

func (s *KeeperNodeAccountSuite) TestListValidatorsByStatuses(c *C) {
	ctx, k := setupKeeperForTest(c)
	ctx = ctx.WithBlockHeight(10)

	na1 := GetRandomValidatorNode(NodeActive)
	na2 := GetRandomValidatorNode(NodeStandby)
	na3 := GetRandomValidatorNode(NodeReady)
	na4 := GetRandomValidatorNode(NodeDisabled)

	c.Assert(k.SetNodeAccount(ctx, na1), IsNil)
	c.Assert(k.SetNodeAccount(ctx, na2), IsNil)
	c.Assert(k.SetNodeAccount(ctx, na3), IsNil)
	c.Assert(k.SetNodeAccount(ctx, na4), IsNil)
	c.Check(na1.ActiveBlockHeight, Equals, int64(10))
	c.Check(na2.ActiveBlockHeight, Equals, int64(0))

	nodeAccounts, err := k.ListValidatorsByStatuses(ctx, NodeActive, NodeStandby, NodeReady, NodeDisabled)
	c.Assert(err, IsNil)

	c.Assert(nodeAccounts.Len(), Equals, 4)
	c.Assert(nodeAccounts.Contains(na1), Equals, true)
	c.Assert(nodeAccounts.Contains(na2), Equals, true)
	c.Assert(nodeAccounts.Contains(na3), Equals, true)
	c.Assert(nodeAccounts.Contains(na4), Equals, true)
}
