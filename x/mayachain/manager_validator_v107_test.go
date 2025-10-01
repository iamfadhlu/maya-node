package mayachain

import (
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

type ValidatorMgrV107TestSuite struct{}

var _ = Suite(&ValidatorMgrV107TestSuite{})

func (vts *ValidatorMgrV107TestSuite) SetUpSuite(_ *C) {
	SetupConfigForTest()
}

func (vts *ValidatorMgrV107TestSuite) TestSetupValidatorNodes(c *C) {
	ctx, k := setupKeeperForTest(c)
	ctx = ctx.WithBlockHeight(1)
	mgr := NewDummyMgr()
	networkMgr := newValidatorMgrV107(k, mgr.NetworkMgr(), mgr.TxOutStore(), mgr.EventMgr())
	c.Assert(networkMgr, NotNil)
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)
	err := networkMgr.setupValidatorNodes(ctx, 0, constAccessor)
	c.Assert(err, IsNil)

	// no node accounts at all
	err = networkMgr.setupValidatorNodes(ctx, 1, constAccessor)
	c.Assert(err, NotNil)

	activeNode := GetRandomValidatorNode(NodeActive)
	c.Assert(k.SetNodeAccount(ctx, activeNode), IsNil)

	err = networkMgr.setupValidatorNodes(ctx, 1, constAccessor)
	c.Assert(err, IsNil)

	readyNode := GetRandomValidatorNode(NodeReady)
	c.Assert(k.SetNodeAccount(ctx, readyNode), IsNil)

	// one active node and one ready node on start up
	// it should take both of the node as active
	networkMgr1 := newValidatorMgrV107(k, mgr.NetworkMgr(), mgr.TxOutStore(), mgr.EventMgr())

	c.Assert(networkMgr1.BeginBlock(ctx, mgr, nil), IsNil)
	activeNodes, err := k.ListActiveValidators(ctx)
	c.Assert(err, IsNil)
	c.Assert(len(activeNodes) == 2, Equals, true)

	activeNode1 := GetRandomValidatorNode(NodeActive)
	activeNode2 := GetRandomValidatorNode(NodeActive)
	c.Assert(k.SetNodeAccount(ctx, activeNode1), IsNil)
	c.Assert(k.SetNodeAccount(ctx, activeNode2), IsNil)

	// three active nodes and 1 ready nodes, it should take them all
	networkMgr2 := newValidatorMgrV107(k, mgr.NetworkMgr(), mgr.TxOutStore(), mgr.EventMgr())
	c.Assert(networkMgr2.BeginBlock(ctx, mgr, nil), IsNil)

	activeNodes1, err := k.ListActiveValidators(ctx)
	c.Assert(err, IsNil)
	c.Assert(len(activeNodes1) == 4, Equals, true)
}

func (vts *ValidatorMgrV107TestSuite) TestRagnarokForChaosnet(c *C) {
	ctx, mgr := setupManagerForTest(c)
	networkMgr := newValidatorMgrV107(mgr.Keeper(), mgr.NetworkMgr(), mgr.TxOutStore(), mgr.EventMgr())

	mgr.constAccessor = constants.NewDummyConstants(map[constants.ConstantName]int64{
		constants.DesiredValidatorSet:           12,
		constants.ArtificialRagnarokBlockHeight: 1024,
		constants.MinimumNodesForBFT:            4,
		constants.ChurnInterval:                 256,
		constants.ChurnRetryInterval:            720,
		constants.AsgardSize:                    30,
	}, map[constants.ConstantName]bool{
		constants.StrictBondLiquidityRatio: false,
	}, map[constants.ConstantName]string{})
	for i := 0; i < 12; i++ {
		node := GetRandomValidatorNode(NodeReady)
		bp := NewBondProviders(node.NodeAddress)
		acc, err := node.BondAddress.AccAddress()
		c.Assert(err, IsNil)
		bp.Providers = append(bp.Providers, NewBondProvider(acc))
		bp.Providers[0].Bonded = true
		SetupLiquidityBondForTestV105(c, ctx, mgr.Keeper(), common.BNBAsset, node.BondAddress, node, cosmos.NewUint(uint64(i+1)*common.One))
		c.Assert(mgr.Keeper().SetBondProviders(ctx, bp), IsNil)
		c.Assert(mgr.Keeper().SetNodeAccount(ctx, node), IsNil)
	}
	c.Assert(networkMgr.setupValidatorNodes(ctx, 1, mgr.GetConstants()), IsNil)
	nodeAccounts, err := mgr.Keeper().ListValidatorsByStatus(ctx, NodeActive)
	c.Assert(err, IsNil)
	c.Assert(len(nodeAccounts), Equals, 12)

	// trigger ragnarok
	ctx = ctx.WithBlockHeight(1024)
	c.Assert(networkMgr.BeginBlock(ctx, mgr, nil), IsNil)
	vault := NewVault(ctx.BlockHeight(), ActiveVault, AsgardVault, GetRandomPubKey(), common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	for _, item := range nodeAccounts {
		vault.Membership = append(vault.Membership, item.PubKeySet.Secp256k1.String())
	}
	c.Assert(mgr.Keeper().SetVault(ctx, vault), IsNil)
	updates := networkMgr.EndBlock(ctx, mgr)
	// ragnarok , no one leaves
	c.Assert(updates, IsNil)
	ragnarokHeight, err := mgr.Keeper().GetRagnarokBlockHeight(ctx)
	c.Assert(err, IsNil)
	c.Assert(ragnarokHeight == 1024, Equals, true, Commentf("%d == %d", ragnarokHeight, 1024))
}

func (vts *ValidatorMgrV107TestSuite) TestLowerVersion(c *C) {
	ctx, mgr := setupManagerForTest(c)
	ctx = ctx.WithBlockHeight(1440)

	constAccessor := constants.NewDummyConstants(map[constants.ConstantName]int64{
		constants.DesiredValidatorSet:            12,
		constants.ArtificialRagnarokBlockHeight:  1024,
		constants.MinimumNodesForBFT:             4,
		constants.ChurnInterval:                  256,
		constants.ChurnRetryInterval:             720,
		constants.AsgardSize:                     30,
		constants.MaxNodeToChurnOutForLowVersion: 3,
	}, map[constants.ConstantName]bool{
		constants.StrictBondLiquidityRatio: false,
	}, map[constants.ConstantName]string{})

	networkMgr := newValidatorMgrV107(mgr.Keeper(), mgr.NetworkMgr(), mgr.TxOutStore(), mgr.EventMgr())
	c.Assert(networkMgr, NotNil)
	c.Assert(networkMgr.markLowVersionValidators(ctx, constAccessor), IsNil)

	for i := 0; i < 12; i++ {
		activeNode := GetRandomValidatorNode(NodeActive)
		activeNode.Version = "0.5.0"
		c.Assert(mgr.Keeper().SetNodeAccount(ctx, activeNode), IsNil)
	}

	// Add 5 low version nodes (1 being a genesis node which shouldn't be marked)
	activeNode1 := GetRandomValidatorNode(NodeActive)
	activeNode1.Version = "0.4.0"
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, activeNode1), IsNil)

	activeNode2 := GetRandomValidatorNode(NodeActive)
	activeNode2.Version = "0.4.0"
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, activeNode2), IsNil)

	activeNode3 := GetRandomValidatorNode(NodeActive)
	activeNode3.Version = "0.4.0"
	acc, err := cosmos.AccAddressFromBech32(GenesisNodes[0])
	c.Assert(err, IsNil)
	activeNode3.NodeAddress = acc
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, activeNode3), IsNil)

	activeNode4 := GetRandomValidatorNode(NodeActive)
	activeNode4.Version = "0.4.0"
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, activeNode4), IsNil)
	c.Assert(networkMgr.markLowVersionValidators(ctx, constAccessor), IsNil)

	activeNas, _ := networkMgr.k.ListActiveValidators(ctx)
	markedCount := 0
	lowVersionAddresses := []common.Address{activeNode1.BondAddress, activeNode2.BondAddress, activeNode3.BondAddress, activeNode4.BondAddress}

	// should have marked 3 of the correct validators as low version
	genesisAdd, err := common.NewAddress(GenesisNodes[0], mgr.GetVersion())
	c.Assert(err, IsNil)
	for _, na := range activeNas {

		isCorrectNode := false
		for _, addr := range lowVersionAddresses {
			if addr == na.BondAddress && !na.BondAddress.Equals(genesisAdd) {
				isCorrectNode = true
				break
			}
		}

		if na.LeaveScore == uint64(144000000000) && isCorrectNode {
			markedCount++
		}
	}

	c.Assert(markedCount, Equals, 3)
}

func (vts *ValidatorMgrV107TestSuite) TestBadActors(c *C) {
	ctx, mgr := setupManagerForTest(c)
	ctx = ctx.WithBlockHeight(1000)

	networkMgr := newValidatorMgrV107(mgr.Keeper(), mgr.NetworkMgr(), mgr.TxOutStore(), mgr.EventMgr())
	c.Assert(networkMgr, NotNil)

	// no bad actors with active node accounts
	nas, err := networkMgr.findBadActors(ctx, 0, 3)
	c.Assert(err, IsNil)
	c.Assert(nas, HasLen, 0)

	activeNode := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, activeNode), IsNil)

	// no bad actors with active node accounts with no slash points
	nas, err = networkMgr.findBadActors(ctx, 0, 3)
	c.Assert(err, IsNil)
	c.Assert(nas, HasLen, 0)

	activeNode = GetRandomValidatorNode(NodeActive)
	mgr.Keeper().SetNodeAccountSlashPoints(ctx, activeNode.NodeAddress, 250)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, activeNode), IsNil)
	activeNode = GetRandomValidatorNode(NodeActive)
	mgr.Keeper().SetNodeAccountSlashPoints(ctx, activeNode.NodeAddress, 500)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, activeNode), IsNil)

	// finds the worse actor
	nas, err = networkMgr.findBadActors(ctx, 0, 3)
	c.Assert(err, IsNil)
	c.Assert(nas, HasLen, 1)
	c.Check(nas[0].NodeAddress.Equals(activeNode.NodeAddress), Equals, true)

	// create really bad actors (crossing the redline)
	bad1 := GetRandomValidatorNode(NodeActive)
	mgr.Keeper().SetNodeAccountSlashPoints(ctx, bad1.NodeAddress, 5000)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, bad1), IsNil)
	bad2 := GetRandomValidatorNode(NodeActive)
	mgr.Keeper().SetNodeAccountSlashPoints(ctx, bad2.NodeAddress, 5000)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, bad2), IsNil)

	nas, err = networkMgr.findBadActors(ctx, 0, 3)
	c.Assert(err, IsNil)
	c.Assert(nas, HasLen, 2, Commentf("%d", len(nas)))

	// inconsistent order, workaround
	var count int
	for _, bad := range nas {
		if bad.Equals(bad1) || bad.Equals(bad2) {
			count++
		}
	}
	c.Check(count, Equals, 2)
}

func (vts *ValidatorMgrV107TestSuite) TestFindBadActors(c *C) {
	ctx, mgr := setupManagerForTest(c)
	ctx = ctx.WithBlockHeight(1000)

	networkMgr := newValidatorMgrV107(mgr.Keeper(), mgr.NetworkMgr(), mgr.TxOutStore(), mgr.EventMgr())
	c.Assert(networkMgr, NotNil)

	activeNode := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, activeNode), IsNil)
	mgr.Keeper().SetNodeAccountSlashPoints(ctx, activeNode.NodeAddress, 50)
	nodeAccounts, err := networkMgr.findBadActors(ctx, 100, 3)
	c.Assert(err, IsNil)
	c.Assert(nodeAccounts, HasLen, 0)

	activeNode1 := GetRandomValidatorNode(NodeActive)
	activeNode1.StatusSince = 900
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, activeNode1), IsNil)
	mgr.Keeper().SetNodeAccountSlashPoints(ctx, activeNode1.NodeAddress, 200)

	// findBadActor assumes it is being called during a churn now,
	// so this should now mark this node as bad.
	nodeAccounts, err = networkMgr.findBadActors(ctx, 100, 3)
	c.Assert(err, IsNil)
	c.Assert(nodeAccounts, HasLen, 1)
	c.Assert(nodeAccounts.Contains(activeNode1), Equals, true)

	activeNode2 := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, activeNode2), IsNil)
	mgr.Keeper().SetNodeAccountSlashPoints(ctx, activeNode2.NodeAddress, 2000)

	activeNode3 := GetRandomValidatorNode(NodeActive)
	activeNode3.StatusSince = 1000
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, activeNode3), IsNil)
	mgr.Keeper().SetNodeAccountSlashPoints(ctx, activeNode3.NodeAddress, 2000)
	ctx = ctx.WithBlockHeight(2000)
	// node 3 and node 2 should both be marked even though node 3 is newer
	// (this is because we're not favoring older nodes anymore)
	nodeAccounts, err = networkMgr.findBadActors(ctx, 100, 3)
	c.Assert(err, IsNil)
	c.Assert(nodeAccounts, HasLen, 2)
	c.Assert(nodeAccounts.Contains(activeNode2), Equals, true)
	c.Assert(nodeAccounts.Contains(activeNode3), Equals, true)
}

func (vts *ValidatorMgrV107TestSuite) TestFindLowBondActor(c *C) {
	ctx, mgr := setupManagerForTest(c)
	ctx = ctx.WithBlockHeight(1000)

	networkMgr := newValidatorMgrV107(mgr.Keeper(), mgr.NetworkMgr(), mgr.TxOutStore(), mgr.EventMgr())
	c.Assert(networkMgr, NotNil)

	na := GetRandomValidatorNode(NodeActive)
	bp := NewBondProviders(na.NodeAddress)
	acc, err := na.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp.Providers = append(bp.Providers, NewBondProvider(acc))
	bp.Providers[0].Bonded = true
	SetupLiquidityBondForTestV105(c, ctx, mgr.Keeper(), common.BNBAsset, na.BondAddress, na, cosmos.NewUint(10))
	c.Assert(mgr.Keeper().SetBondProviders(ctx, bp), IsNil)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, na), IsNil)

	na, err = networkMgr.findLowBondActor(ctx)
	c.Assert(err, IsNil)

	naBond, err := mgr.Keeper().CalcNodeLiquidityBond(ctx, na)
	c.Assert(err, IsNil)

	c.Assert(na.IsEmpty(), Equals, false)
	c.Assert(int64(naBond.Uint64()), Equals, int64(20))

	na2 := GetRandomValidatorNode(NodeActive)
	na2Bond := cosmos.NewUint(9)
	bp = NewBondProviders(na2.NodeAddress)
	acc, err = na2.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp.Providers = append(bp.Providers, NewBondProvider(acc))
	bp.Providers[0].Bonded = true
	SetupLiquidityBondForTestV105(c, ctx, mgr.Keeper(), common.BNBAsset, na2.BondAddress, na2, na2Bond)
	c.Assert(mgr.Keeper().SetBondProviders(ctx, bp), IsNil)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, na2), IsNil)

	na, err = networkMgr.findLowBondActor(ctx)
	c.Assert(err, IsNil)
	naBond, err = mgr.Keeper().CalcNodeLiquidityBond(ctx, na)
	c.Assert(err, IsNil)
	c.Assert(int64(naBond.Uint64()), Equals, int64(18))

	na3 := GetRandomValidatorNode(NodeActive)
	na3Bond := cosmos.ZeroUint()
	bp = NewBondProviders(na3.NodeAddress)
	acc, err = na3.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp.Providers = append(bp.Providers, NewBondProvider(acc))
	bp.Providers[0].Bonded = true
	SetupLiquidityBondForTestV105(c, ctx, mgr.Keeper(), common.BNBAsset, na3.BondAddress, na3, na3Bond)
	c.Assert(mgr.Keeper().SetBondProviders(ctx, bp), IsNil)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, na3), IsNil)

	na, err = networkMgr.findLowBondActor(ctx)
	c.Assert(err, IsNil)
	naBond, err = mgr.Keeper().CalcNodeLiquidityBond(ctx, na)
	c.Assert(err, IsNil)
	c.Assert(naBond.IsZero(), Equals, true)
}

func (vts *ValidatorMgrV107TestSuite) TestGetChangedNodes(c *C) {
	ctx, k := setupKeeperForTest(c)
	ctx = ctx.WithBlockHeight(1)
	ver := GetCurrentVersion()

	mgr := NewDummyMgrWithKeeper(k)
	networkMgr := newValidatorMgrV107(k, mgr.NetworkMgr(), mgr.TxOutStore(), mgr.EventMgr())
	c.Assert(networkMgr, NotNil)

	constAccessor := constants.GetConstantValues(ver)
	err := networkMgr.setupValidatorNodes(ctx, 0, constAccessor)
	c.Assert(err, IsNil)

	activeNode := GetRandomValidatorNode(NodeActive)
	activeNodeBond := cosmos.NewUint(100)
	bp := NewBondProviders(activeNode.NodeAddress)
	acc, err := activeNode.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp.Providers = append(bp.Providers, NewBondProvider(acc))
	bp.Providers[0].Bonded = true
	SetupLiquidityBondForTestV105(c, ctx, k, common.BNBAsset, activeNode.BondAddress, activeNode, activeNodeBond)
	c.Assert(k.SetBondProviders(ctx, bp), IsNil)
	activeNode.ForcedToLeave = true
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, activeNode), IsNil)

	// Zero bond
	disabledNode := GetRandomValidatorNode(NodeDisabled)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, disabledNode), IsNil)

	vault := NewVault(ctx.BlockHeight(), ActiveVault, AsgardVault, GetRandomPubKey(), common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	vault.Membership = append(vault.Membership, activeNode.PubKeySet.Secp256k1.String())
	c.Assert(mgr.Keeper().SetVault(ctx, vault), IsNil)

	newNodes, removedNodes, err := networkMgr.getChangedNodes(ctx, NodeAccounts{activeNode})
	c.Assert(err, IsNil)
	c.Assert(newNodes, HasLen, 0)
	c.Assert(removedNodes, HasLen, 1)
}

func (vts *ValidatorMgrV107TestSuite) TestSplitNext(c *C) {
	ctx, k := setupKeeperForTest(c)
	mgr := NewDummyMgr()
	networkMgr := newValidatorMgrV107(k, mgr.NetworkMgr(), mgr.TxOutStore(), mgr.EventMgr())
	c.Assert(networkMgr, NotNil)

	nas := make(NodeAccounts, 0)
	for i := 0; i < 90; i++ {
		na := GetRandomValidatorNode(NodeActive)
		naBond := cosmos.NewUint(uint64(i))
		bp := NewBondProviders(na.NodeAddress)
		acc, err := na.BondAddress.AccAddress()
		c.Assert(err, IsNil)
		bp.Providers = append(bp.Providers, NewBondProvider(acc))
		bp.Providers[0].Bonded = true
		SetupLiquidityBondForTestV105(c, ctx, k, common.BNBAsset, na.BondAddress, na, naBond)
		c.Assert(k.SetBondProviders(ctx, bp), IsNil)
		c.Assert(k.SetNodeAccount(ctx, na), IsNil)
		nas = append(nas, na)
	}
	sets := networkMgr.splitNext(ctx, nas, 30)
	c.Assert(sets, HasLen, 3)
	c.Assert(sets[0], HasLen, 30)
	c.Assert(sets[1], HasLen, 30)
	c.Assert(sets[2], HasLen, 30)

	nas = make(NodeAccounts, 0)
	for i := 0; i < 100; i++ {
		na := GetRandomValidatorNode(NodeActive)
		naBond := cosmos.NewUint(uint64(i))
		bp := NewBondProviders(na.NodeAddress)
		acc, err := na.BondAddress.AccAddress()
		c.Assert(err, IsNil)
		bp.Providers = append(bp.Providers, NewBondProvider(acc))
		bp.Providers[0].Bonded = true
		SetupLiquidityBondForTestV105(c, ctx, k, common.BNBAsset, na.BondAddress, na, naBond)
		c.Assert(k.SetBondProviders(ctx, bp), IsNil)
		c.Assert(k.SetNodeAccount(ctx, na), IsNil)
		nas = append(nas, na)
	}
	sets = networkMgr.splitNext(ctx, nas, 30)
	c.Assert(sets, HasLen, 4)
	c.Assert(sets[0], HasLen, 25)
	c.Assert(sets[1], HasLen, 25)
	c.Assert(sets[2], HasLen, 25)
	c.Assert(sets[3], HasLen, 25)

	nas = make(NodeAccounts, 0)
	for i := 0; i < 3; i++ {
		na := GetRandomValidatorNode(NodeActive)
		naBond := cosmos.NewUint(uint64(i))
		bp := NewBondProviders(na.NodeAddress)
		acc, err := na.BondAddress.AccAddress()
		c.Assert(err, IsNil)
		bp.Providers = append(bp.Providers, NewBondProvider(acc))
		bp.Providers[0].Bonded = true
		SetupLiquidityBondForTestV105(c, ctx, k, common.BNBAsset, na.BondAddress, na, naBond)
		c.Assert(k.SetBondProviders(ctx, bp), IsNil)
		c.Assert(k.SetNodeAccount(ctx, na), IsNil)
		nas = append(nas, na)
	}
	sets = networkMgr.splitNext(ctx, nas, 30)
	c.Assert(sets, HasLen, 1)
	c.Assert(sets[0], HasLen, 3)
}

func (vts *ValidatorMgrV107TestSuite) TestFindCounToRemove(c *C) {
	// remove one
	c.Check(findCountToRemove(NodeAccounts{
		NodeAccount{LeaveScore: 12},
		NodeAccount{},
		NodeAccount{},
		NodeAccount{},
		NodeAccount{},
	}), Equals, 1)

	// don't remove one
	c.Check(findCountToRemove(NodeAccounts{
		NodeAccount{LeaveScore: 12},
		NodeAccount{LeaveScore: 12},
		NodeAccount{},
		NodeAccount{},
	}), Equals, 0)

	// remove one because of request to leave
	c.Check(findCountToRemove(NodeAccounts{
		NodeAccount{LeaveScore: 12, RequestedToLeave: true},
		NodeAccount{},
		NodeAccount{},
		NodeAccount{},
	}), Equals, 1)

	// remove one because of banned
	c.Check(findCountToRemove(NodeAccounts{
		NodeAccount{LeaveScore: 12, ForcedToLeave: true},
		NodeAccount{},
		NodeAccount{},
		NodeAccount{},
	}), Equals, 1)

	// don't remove more than 1/3rd of node accounts
	c.Check(findCountToRemove(NodeAccounts{
		NodeAccount{LeaveScore: 12},
		NodeAccount{LeaveScore: 12},
		NodeAccount{LeaveScore: 12},
		NodeAccount{LeaveScore: 12},
		NodeAccount{LeaveScore: 12},
		NodeAccount{LeaveScore: 12},
		NodeAccount{LeaveScore: 12},
		NodeAccount{LeaveScore: 12},
		NodeAccount{LeaveScore: 12},
		NodeAccount{LeaveScore: 12},
		NodeAccount{LeaveScore: 12},
		NodeAccount{LeaveScore: 12},
	}), Equals, 3)
}

func (vts *ValidatorMgrV107TestSuite) TestFindMaxAbleToLeave(c *C) {
	c.Check(findMaxAbleToLeave(-1), Equals, 0)
	c.Check(findMaxAbleToLeave(0), Equals, 0)
	c.Check(findMaxAbleToLeave(1), Equals, 0)
	c.Check(findMaxAbleToLeave(2), Equals, 0)
	c.Check(findMaxAbleToLeave(3), Equals, 0)
	c.Check(findMaxAbleToLeave(4), Equals, 0)

	c.Check(findMaxAbleToLeave(5), Equals, 1)
	c.Check(findMaxAbleToLeave(6), Equals, 1)
	c.Check(findMaxAbleToLeave(7), Equals, 2)
	c.Check(findMaxAbleToLeave(8), Equals, 2)
	c.Check(findMaxAbleToLeave(9), Equals, 2)
	c.Check(findMaxAbleToLeave(10), Equals, 3)
	c.Check(findMaxAbleToLeave(11), Equals, 3)
	c.Check(findMaxAbleToLeave(12), Equals, 3)
}

func (vts *ValidatorMgrV107TestSuite) TestFindNextVaultNodeAccounts(c *C) {
	ctx, k := setupKeeperForTest(c)
	mgr := NewDummyMgrWithKeeper(k)
	networkMgr := newValidatorMgrV107(k, mgr.NetworkMgr(), mgr.TxOutStore(), mgr.EventMgr())
	c.Assert(networkMgr, NotNil)
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)
	nas := NodeAccounts{}
	for i := 0; i < 12; i++ {
		na := GetRandomValidatorNode(NodeActive)
		nas = append(nas, na)
	}
	nas[0].LeaveScore = 1024
	k.SetNodeAccountSlashPoints(ctx, nas[0].NodeAddress, 50)
	nas[1].LeaveScore = 1025
	k.SetNodeAccountSlashPoints(ctx, nas[1].NodeAddress, 200)
	nas[2].ForcedToLeave = true
	nas[3].RequestedToLeave = true
	for _, item := range nas {
		c.Assert(k.SetNodeAccount(ctx, item), IsNil)
	}
	nasAfter, rotate, err := networkMgr.nextVaultNodeAccounts(ctx, 12, constAccessor)
	c.Assert(err, IsNil)
	c.Assert(rotate, Equals, true)
	c.Assert(nasAfter, HasLen, 10)
}

func (vts *ValidatorMgrV107TestSuite) TestFindNextVaultNodeAccountsMax(c *C) {
	// test that we don't exceed the targetCount
	ctx, mgr := setupManagerForTest(c)
	networkMgr := newValidatorMgrV107(mgr.Keeper(), mgr.NetworkMgr(), mgr.TxOutStore(), mgr.EventMgr())
	c.Assert(networkMgr, NotNil)
	// create active nodes
	for i := 0; i < 12; i++ {
		na := GetRandomValidatorNode(NodeActive)
		bp := NewBondProviders(na.NodeAddress)
		acc, err := na.BondAddress.AccAddress()
		c.Assert(err, IsNil)
		bp.Providers = append(bp.Providers, NewBondProvider(acc))
		bp.Providers[0].Bonded = true
		SetupLiquidityBondForTestV105(c, ctx, mgr.Keeper(), common.BNBAsset, na.BondAddress, na, cosmos.NewUint(100*common.One))
		c.Assert(mgr.Keeper().SetBondProviders(ctx, bp), IsNil)
		if i < 3 {
			na.LeaveScore = 1024
		}
		c.Assert(mgr.Keeper().SetNodeAccount(ctx, na), IsNil)
	}
	// create standby nodes
	for i := 0; i < 12; i++ {
		na := GetRandomValidatorNode(NodeStandby)
		bp := NewBondProviders(na.NodeAddress)
		acc, err := na.BondAddress.AccAddress()
		c.Assert(err, IsNil)
		bp.Providers = append(bp.Providers, NewBondProvider(acc))
		bp.Providers[0].Bonded = true
		SetupLiquidityBondForTestV105(c, ctx, mgr.Keeper(), common.BNBAsset, na.BondAddress, na, cosmos.NewUint(100*common.One))
		c.Assert(mgr.Keeper().SetBondProviders(ctx, bp), IsNil)
		c.Assert(mgr.Keeper().SetNodeAccount(ctx, na), IsNil)
	}
	nasAfter, rotate, err := networkMgr.nextVaultNodeAccounts(ctx, 12, mgr.GetConstants())
	c.Assert(err, IsNil)
	c.Assert(rotate, Equals, true)
	c.Assert(nasAfter, HasLen, 12, Commentf("%d", len(nasAfter)))
}

func (vts *ValidatorMgrV107TestSuite) TestEquitableBondReward(c *C) {
	ctx, k := setupKeeperForTest(c)
	ctx = ctx.WithBlockHeight(20)

	mgr := NewDummyMgrWithKeeper(k)
	networkMgr := newValidatorMgrV107(k, mgr.NetworkMgr(), mgr.TxOutStore(), mgr.EventMgr())
	c.Assert(networkMgr, NotNil)
	FundModule(c, ctx, mgr.Keeper(), BondName, 100*common.One)
	mgr.Keeper().SetMimir(ctx, "MinimumBondInCacao", 100_000_00000000)

	na1 := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, na1), IsNil)

	na2 := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, na2), IsNil)

	na3 := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, na3), IsNil)

	network, _ := networkMgr.k.GetNetwork(ctx)
	network.BondRewardRune = cosmos.NewUint(9 * common.One)
	c.Assert(mgr.Keeper().SetNetwork(ctx, network), IsNil)

	nodes := NodeAccounts{na1, na2, na3}
	for i, node := range nodes {
		// 1/2 each bp
		// 1/3 and 2/3 each bp respectively
		// 1/4 and 3/4 each bp respectively
		SetupLiquidityBondForTestV105(c, ctx, k, common.BTCAsset, node.BondAddress, node, cosmos.NewUint(1*common.One))

		extraBP := GetRandomBaseAddress()
		extraBPAcc, _ := extraBP.AccAddress()
		bondAddressAcc, _ := node.BondAddress.AccAddress()
		SetupLiquidityBondForTestV105(c, ctx, k, common.BTCAsset, extraBP, node, cosmos.NewUint((uint64(i)+1)*common.One))
		bp := NewBondProviders(node.NodeAddress)
		bp.Providers = append(bp.Providers, NewBondProvider(bondAddressAcc), NewBondProvider(extraBPAcc))
		bp.Providers[0].Bonded = true
		bp.Providers[1].Bonded = true
		c.Assert(mgr.Keeper().SetBondProviders(ctx, bp), IsNil)
	}

	// pay out bond rewards
	c.Assert(networkMgr.ragnarokBondReward(ctx, mgr), IsNil)

	naRewards := make([]cosmos.Uint, 3)
	bondBalance := cosmos.ZeroUint()
	for i, node := range nodes {
		bp, err := k.GetBondProviders(ctx, node.NodeAddress)
		c.Assert(err, IsNil)
		naRewards[i] = cosmos.ZeroUint()
		for j, b := range bp.Providers {
			if j == 0 {
				// 1/2, 1/3, 1/4
				rewardCheck := common.GetUncappedShare(cosmos.NewUint(common.One), cosmos.NewUint((uint64(i)+2)*common.One), cosmos.NewUint(3*common.One))
				bondBalance = cosmos.NewUint(mgr.Keeper().GetBalance(ctx, b.BondAddress).AmountOf(common.BaseAsset().Native()).Uint64())
				c.Check(bondBalance.Equal(rewardCheck), Equals, true, Commentf("expected %d, got %d", rewardCheck, bondBalance))
			} else {
				// 1/2, 2/3, 3/4
				nominator := cosmos.NewUint((uint64(i) + 1) * common.One)
				denominator := cosmos.NewUint((uint64(i) + 2) * common.One)

				rewardCheck := common.GetUncappedShare(nominator, denominator, cosmos.NewUint(3*common.One))
				bondBalance = cosmos.NewUint(mgr.Keeper().GetBalance(ctx, b.BondAddress).AmountOf(common.BaseAsset().Native()).Uint64())
				c.Check(bondBalance.Equal(rewardCheck), Equals, true, Commentf("expected %d, got %d", rewardCheck, bondBalance))
			}

			naRewards[i] = naRewards[i].Add(bondBalance)
		}
	}

	// bond balance should not be zero
	c.Check(!bondBalance.IsZero(), Equals, true, Commentf("expected %d, got %d", 0, bondBalance))

	// There's no reward hard cap. na1, na2, na3 should have the same reward
	for _, reward := range naRewards {
		c.Check(reward.Uint64(), Equals, uint64(3*common.One), Commentf("expected %d, got %d", 3*common.One, reward.Uint64()))
	}
}

func (vts *ValidatorMgrV107TestSuite) TestActiveNodeRequestToLeaveShouldBeStandby(c *C) {
	var err error
	ctx, mgr := setupManagerForTest(c)
	ctx = ctx.WithBlockHeight(10)

	// create active asgard vault
	asgard := GetRandomVault()
	c.Assert(mgr.Keeper().SetVault(ctx, asgard), IsNil)

	// Add bonders/validators
	bonderCount := 4
	for i := 1; i <= bonderCount; i++ {
		na := GetRandomValidatorNode(NodeActive)
		na.ActiveBlockHeight = 5
		naBond := cosmos.NewUint(100 * uint64(i) * common.One)
		SetupLiquidityBondForTestV105(c, ctx, mgr.Keeper(), common.BNBAsset, na.BondAddress, na, naBond)
		c.Assert(mgr.Keeper().SetNodeAccount(ctx, na), IsNil)

		// Add bond to asgard
		asgard.AddFunds(common.Coins{
			common.NewCoin(common.BaseAsset(), naBond),
		})
		asgard.Membership = append(asgard.Membership, na.PubKeySet.Secp256k1.String())
		c.Assert(mgr.Keeper().SetVault(ctx, asgard), IsNil)
	}
	// set one node request to leave
	nodeKey := asgard.Membership[0]
	nodePubKey, err := common.NewPubKey(nodeKey)
	c.Assert(err, IsNil)
	na, err := mgr.Keeper().GetNodeAccountByPubKey(ctx, nodePubKey)
	c.Assert(err, IsNil)
	na.RequestedToLeave = true
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, na), IsNil)
	newAsgard := GetRandomVault()
	newAsgard.Type = AsgardVault
	newAsgard.Membership = asgard.Membership[1:]
	c.Assert(mgr.Keeper().SetVault(ctx, newAsgard), IsNil)
	c.Assert(mgr.NetworkMgr().RotateVault(ctx, newAsgard), IsNil)

	updates := mgr.ValidatorMgr().EndBlock(ctx, mgr)
	c.Assert(updates, NotNil)

	naAfter, err := mgr.Keeper().GetNodeAccount(ctx, na.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(naAfter.RequestedToLeave, Equals, false)
}
