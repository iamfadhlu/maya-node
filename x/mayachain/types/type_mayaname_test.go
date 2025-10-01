package types

import (
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

type MAYANameSuite struct{}

var _ = Suite(&MAYANameSuite{})

func (MAYANameSuite) TestMAYAName(c *C) {
	// happy path
	n := NewMAYAName("iamthewalrus", 0, []MAYANameAlias{{Chain: common.BASEChain, Address: GetRandomBaseAddress()}}, common.EmptyAsset, nil, cosmos.ZeroUint(), nil)
	c.Check(n.Valid(), IsNil)

	// unhappy path
	n1 := NewMAYAName("", 0, []MAYANameAlias{{Chain: common.BNBChain, Address: GetRandomBaseAddress()}}, common.EmptyAsset, nil, cosmos.ZeroUint(), nil)
	c.Check(n1.Valid(), NotNil)
	n2 := NewMAYAName("hello", 0, []MAYANameAlias{{Chain: common.EmptyChain, Address: GetRandomBaseAddress()}}, common.EmptyAsset, nil, cosmos.ZeroUint(), nil)
	c.Check(n2.Valid(), NotNil)
	n3 := NewMAYAName("hello", 0, []MAYANameAlias{{Chain: common.BASEChain, Address: common.Address("")}}, common.EmptyAsset, nil, cosmos.ZeroUint(), nil)
	c.Check(n3.Valid(), NotNil)
	n4 := NewMAYAName("hello", 0, []MAYANameAlias{{Chain: common.BASEChain, Address: common.Address("")}}, common.EmptyAsset, nil, cosmos.NewUint(20000000), []MAYANameSubaffiliate{{Name: "subaff1", Bps: cosmos.NewUint(1000)}})
	c.Check(n4.Valid(), NotNil)
	n5 := NewMAYAName("hello", 0, []MAYANameAlias{{Chain: common.BASEChain, Address: common.Address("")}}, common.EmptyAsset, nil, cosmos.NewUint(2000), []MAYANameSubaffiliate{{Name: "", Bps: cosmos.NewUint(1000)}})
	c.Check(n5.Valid(), NotNil)
	n6 := NewMAYAName("hello", 0, []MAYANameAlias{{Chain: common.BASEChain, Address: common.Address("")}}, common.EmptyAsset, nil, cosmos.NewUint(2000), []MAYANameSubaffiliate{{Name: "subaff1", Bps: cosmos.NewUint(10000000)}})
	c.Check(n6.Valid(), NotNil)

	// set/get alias
	eth1 := GetRandomETHAddress()
	n1.SetAlias(common.ETHChain, eth1)
	c.Check(n1.GetAlias(common.ETHChain), Equals, eth1)

	// set/get subaffiliate
	_ = n1.SetSubaffiliate("subalfa", cosmos.NewUint(2000))
	_ = n1.SetSubaffiliate("subbeta", cosmos.NewUint(3000))
	c.Check(n1.GetSubaffiliates(), NotNil)
	c.Check(n1.GetSubaffiliates(), HasLen, 2)
	c.Check(n1.GetSubaffiliates()[0].GetName(), Equals, "subalfa")
	c.Check(n1.GetSubaffiliates()[0].Bps.Equal(cosmos.NewUint(2000)), Equals, true)
	c.Check(n1.GetSubaffiliates()[1].GetName(), Equals, "subbeta")
	c.Check(n1.GetSubaffiliates()[1].Bps.Equal(cosmos.NewUint(3000)), Equals, true)
}
