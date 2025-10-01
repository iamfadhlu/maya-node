package keeperv1

import (
	"github.com/blang/semver"
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	cosmos "gitlab.com/mayachain/mayanode/common/cosmos"
)

type KeeperLiquidityProviderSuite struct{}

var _ = Suite(&KeeperLiquidityProviderSuite{})

func (mas *KeeperLiquidityProviderSuite) SetUpSuite(c *C) {
	SetupConfigForTest()
}

func (s *KeeperLiquidityProviderSuite) TestLiquidityProvider(c *C) {
	ctx, k := setupKeeperForTest(c)
	asset := common.BNBAsset

	lp, err := k.GetLiquidityProvider(ctx, asset, GetRandomBaseAddress())
	c.Assert(err, IsNil)
	c.Check(lp.PendingCacao, NotNil)
	c.Check(lp.Units, NotNil)

	lp = LiquidityProvider{
		Asset:        asset,
		Units:        cosmos.NewUint(12),
		CacaoAddress: GetRandomBaseAddress(),
		AssetAddress: GetRandomBTCAddress(),
	}
	k.SetLiquidityProvider(ctx, lp)
	lp, err = k.GetLiquidityProvider(ctx, asset, lp.CacaoAddress)
	c.Assert(err, IsNil)
	c.Check(lp.Asset.Equals(asset), Equals, true)
	c.Check(lp.Units.Equal(cosmos.NewUint(12)), Equals, true)
	iter := k.GetLiquidityProviderIterator(ctx, common.BNBAsset)
	c.Check(iter, NotNil)
	iter.Close()
	k.RemoveLiquidityProvider(ctx, lp)
}

func (s *KeeperLiquidityProviderSuite) TestLiquidityAuctionTier(c *C) {
	ctx, k := setupKeeperForTest(c)
	newLATier := LiquidityAuctionTier{
		Address: GetRandomBaseAddress(),
	}

	// Tier not previously set
	laTierValue, err := k.GetLiquidityAuctionTier(ctx, newLATier.Address)
	c.Assert(err, IsNil)
	c.Assert(laTierValue, Equals, int64(0))

	// It should set the tier if it's equals or lower
	err = k.SetLiquidityAuctionTier(ctx, newLATier.Address, 3)
	c.Assert(err, IsNil)
	laTierValue, err = k.GetLiquidityAuctionTier(ctx, newLATier.Address)
	c.Assert(err, IsNil)
	c.Assert(laTierValue, Equals, int64(3))

	err = k.SetLiquidityAuctionTier(ctx, newLATier.Address, 2)
	c.Assert(err, IsNil)
	laTierValue, err = k.GetLiquidityAuctionTier(ctx, newLATier.Address)
	c.Assert(err, IsNil)
	c.Assert(laTierValue, Equals, int64(2))

	err = k.SetLiquidityAuctionTier(ctx, newLATier.Address, 1)
	c.Assert(err, IsNil)
	laTierValue, err = k.GetLiquidityAuctionTier(ctx, newLATier.Address)
	c.Assert(err, IsNil)
	c.Assert(laTierValue, Equals, int64(1))
}

func (s *KeeperLiquidityProviderSuite) TestCalcLPLiquidityBond(c *C) {
	ctx, k := setupKeeperForTest(c)

	fakeNa := GetRandomValidatorNode(NodeActive)

	addr1 := GetRandomBaseAddress()
	na1 := GetRandomValidatorNode(NodeActive)
	lp1, _ := SetupLiquidityBondForTest(c, ctx, k, common.BTCAsset, addr1, cosmos.NewUint(100))

	// Check v1 calculation and independency of the node address
	k.SetVersion(semver.MustParse("1.104.0"))

	bond, err := k.CalcLPLiquidityBond(ctx, addr1, na1.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(bond.Uint64(), Equals, uint64(200))

	bond, err = k.CalcLPLiquidityBond(ctx, addr1, fakeNa.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(bond.Uint64(), Equals, uint64(200))

	// Check backward compatibility with the deprecated "NodeBondAddress" field
	k.SetVersion(semver.MustParse("1.105.0"))
	lp1.NodeBondAddress = na1.NodeAddress
	k.SetLiquidityProvider(ctx, lp1)

	tl, err := k.CalcTotalBondableLiquidity(ctx, addr1)
	c.Assert(err, IsNil)
	c.Assert(tl.Uint64(), Equals, uint64(200))

	bond, err = k.CalcLPLiquidityBond(ctx, addr1, na1.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(bond.Uint64(), Equals, uint64(200))

	bond, err = k.CalcLPLiquidityBond(ctx, addr1, fakeNa.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(bond.IsZero(), Equals, true)

	// Check BondedNodes field
	addr2 := GetRandomBaseAddress()
	na2 := GetRandomValidatorNode(NodeActive)
	lp2, _ := SetupLiquidityBondForTest(c, ctx, k, common.BNBAsset, addr2, cosmos.NewUint(300))
	lp2.Bond(na1.NodeAddress, cosmos.NewUint(100))
	lp2.Bond(na2.NodeAddress, cosmos.NewUint(150))
	k.SetLiquidityProvider(ctx, lp2)

	bond, err = k.CalcLPLiquidityBond(ctx, addr2, na1.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(bond.Uint64(), Equals, uint64(200))

	bond, err = k.CalcLPLiquidityBond(ctx, addr2, na2.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(bond.Uint64(), Equals, uint64(300))

	bond, err = k.CalcLPLiquidityBond(ctx, addr2, fakeNa.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(bond.IsZero(), Equals, true)

	// Check that the bond is calculated correctly when the LP has more than pools bonded to the same node
	lp3, _ := SetupLiquidityBondForTest(c, ctx, k, common.ETHAsset, addr1, cosmos.NewUint(150))
	lp3.Bond(na1.NodeAddress, cosmos.NewUint(50))
	k.SetLiquidityProvider(ctx, lp3)

	tl, err = k.CalcTotalBondableLiquidity(ctx, addr1)
	c.Assert(err, IsNil)
	c.Assert(tl.Uint64(), Equals, uint64(500))

	bond, err = k.CalcLPLiquidityBond(ctx, addr1, na1.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(bond.Uint64(), Equals, uint64(300))
}

func (s *KeeperLiquidityProviderSuite) TestGetLPLiquidityBond(c *C) {
	ctx, k := setupKeeperForTest(c)

	fakeNa := GetRandomValidatorNode(NodeActive)

	addr1 := GetRandomBaseAddress()
	lp1, _ := SetupLiquidityBondForTest(c, ctx, k, common.BTCAsset, addr1, cosmos.NewUint(100))
	na1 := GetRandomValidatorNode(NodeActive)
	lp1.NodeBondAddress = na1.NodeAddress
	k.SetLiquidityProvider(ctx, lp1)

	tl, err := k.CalcTotalBondableLiquidity(ctx, addr1)
	c.Assert(err, IsNil)
	c.Assert(tl.Uint64(), Equals, uint64(200))

	bond, lps, err := k.GetLPLiquidityBond(ctx, addr1, na1.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(len(lps), Equals, 0)
	c.Assert(bond.Uint64(), Equals, uint64(200))

	bond, lps, err = k.GetLPLiquidityBond(ctx, addr1, fakeNa.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(len(lps), Equals, 0)
	c.Assert(bond.IsZero(), Equals, true)

	// Check BondedNodes field
	addr2 := GetRandomBaseAddress()
	lp2, _ := SetupLiquidityBondForTest(c, ctx, k, common.BNBAsset, addr2, cosmos.NewUint(300))
	na2 := GetRandomValidatorNode(NodeActive)
	lp2.Bond(na1.NodeAddress, cosmos.NewUint(100))
	lp2.Bond(na2.NodeAddress, cosmos.NewUint(150))
	k.SetLiquidityProvider(ctx, lp2)

	bond, lps, err = k.GetLPLiquidityBond(ctx, addr2, na1.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(len(lps), Equals, 1)
	c.Assert(bond.Uint64(), Equals, uint64(200))

	bond, lps, err = k.GetLPLiquidityBond(ctx, addr2, na2.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(len(lps), Equals, 1)
	c.Assert(bond.Uint64(), Equals, uint64(300))

	bond, lps, err = k.GetLPLiquidityBond(ctx, addr2, fakeNa.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(len(lps), Equals, 0)
	c.Assert(bond.IsZero(), Equals, true)

	// Check that the bond is calculated correctly when the LP has more than pools bonded to the same node
	lp3, _ := SetupLiquidityBondForTest(c, ctx, k, common.ETHAsset, addr1, cosmos.NewUint(150))
	lp3.Bond(na1.NodeAddress, cosmos.NewUint(50))
	k.SetLiquidityProvider(ctx, lp3)

	tl, err = k.CalcTotalBondableLiquidity(ctx, addr1)
	c.Assert(err, IsNil)
	c.Assert(tl.Uint64(), Equals, uint64(500))

	bond, lps, err = k.GetLPLiquidityBond(ctx, addr1, na1.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(len(lps), Equals, 1)
	c.Assert(bond.Uint64(), Equals, uint64(300))
}
