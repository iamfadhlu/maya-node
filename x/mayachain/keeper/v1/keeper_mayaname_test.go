package keeperv1

import (
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	. "gopkg.in/check.v1"
)

type KeeperMAYANameSuite struct{}

var _ = Suite(&KeeperMAYANameSuite{})

func (s *KeeperMAYANameSuite) TestMAYAName(c *C) {
	ctx, k := setupKeeperForTest(c)
	var err error
	ref := "helloworld"

	ok := k.MAYANameExists(ctx, ref)
	c.Assert(ok, Equals, false)

	thorAddr := GetRandomBaseAddress()
	bnbAddr := GetRandomBNBAddress()
	name := NewMAYAName(ref, 50, []MAYANameAlias{{Chain: common.BASEChain, Address: thorAddr}, {Chain: common.BNBChain, Address: bnbAddr}}, common.EmptyAsset, nil, cosmos.NewUint(2000), []MAYANameSubaffiliate{{Name: "alfa", Bps: cosmos.NewUint(100)}, {Name: "beta", Bps: cosmos.NewUint(200)}})
	k.SetMAYAName(ctx, name)

	ok = k.MAYANameExists(ctx, ref)
	c.Assert(ok, Equals, true)
	ok = k.MAYANameExists(ctx, "bogus")
	c.Assert(ok, Equals, false)

	name, err = k.GetMAYAName(ctx, ref)
	c.Assert(err, IsNil)
	c.Assert(name.GetAlias(common.BASEChain).Equals(thorAddr), Equals, true)
	c.Assert(name.GetAlias(common.BNBChain).Equals(bnbAddr), Equals, true)

	c.Assert(name.GetAffiliateBps().Equal(cosmos.NewUint(2000)), Equals, true)
	c.Assert(name.GetSubaffiliates(), NotNil)
	c.Assert(name.GetSubaffiliates(), HasLen, 2)
	c.Assert(name.GetSubaffiliates()[0].GetName(), Equals, "alfa")
	c.Assert(name.GetSubaffiliates()[1].GetName(), Equals, "beta")
}
