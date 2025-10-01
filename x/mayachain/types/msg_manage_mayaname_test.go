package types

import (
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	. "gopkg.in/check.v1"
)

type MsgManageMAYANameSuite struct{}

var _ = Suite(&MsgManageMAYANameSuite{})

func (MsgManageMAYANameSuite) TestMsgManageMAYANameSuite(c *C) {
	ver := GetCurrentVersion()
	owner := GetRandomBech32Addr()
	signer := GetRandomBech32Addr()
	coin := common.NewCoin(common.BaseAsset(), cosmos.NewUint(10*common.One))
	msg := NewMsgManageMAYAName("myname", common.BNBChain, GetRandomBNBAddress(), coin, 0, common.BNBAsset, cosmos.ZeroUint(), []cosmos.Uint{}, []string{}, owner, signer)
	c.Assert(msg.Route(), Equals, RouterKey)
	c.Assert(msg.Type(), Equals, "manage_mayaname")
	c.Assert(msg.ValidateBasicV112(ver), IsNil)
	c.Assert(len(msg.GetSignBytes()) > 0, Equals, true)
	c.Assert(msg.GetSigners(), NotNil)
	c.Assert(msg.GetSigners()[0].String(), Equals, signer.String())
	// unhappy paths
	msg = NewMsgManageMAYAName("myname", common.BNBChain, GetRandomBNBAddress(), coin, 0, common.BNBAsset, cosmos.ZeroUint(), []cosmos.Uint{}, []string{}, owner, cosmos.AccAddress{})
	c.Assert(msg.ValidateBasicV112(ver), NotNil)
	msg = NewMsgManageMAYAName("myname", common.EmptyChain, GetRandomBNBAddress(), coin, 0, common.BNBAsset, cosmos.ZeroUint(), []cosmos.Uint{}, []string{}, owner, signer)
	c.Assert(msg.ValidateBasicV112(ver), NotNil)
	msg = NewMsgManageMAYAName("myname", common.BNBChain, common.NoAddress, coin, 0, common.BNBAsset, cosmos.ZeroUint(), []cosmos.Uint{}, []string{}, owner, signer)
	c.Assert(msg.ValidateBasicV112(ver), NotNil)
	msg = NewMsgManageMAYAName("myname", common.BNBChain, GetRandomBTCAddress(), coin, 0, common.BNBAsset, cosmos.ZeroUint(), []cosmos.Uint{}, []string{}, owner, signer)
	c.Assert(msg.ValidateBasicV112(ver), NotNil)
	msg = NewMsgManageMAYAName("myname", common.BNBChain, GetRandomBNBAddress(), common.NewCoin(common.BNBAsset, cosmos.NewUint(10*common.One)), 0, common.BNBAsset, cosmos.ZeroUint(), []cosmos.Uint{}, []string{}, owner, signer)
	c.Assert(msg.ValidateBasicV112(ver), NotNil)
	// V112 - affiliate shate tests
	msg = NewMsgManageMAYAName("myname", common.BNBChain, GetRandomBNBAddress(), coin, 0, common.BNBAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(1000)}, []string{"myname"}, owner, cosmos.AccAddress{})
	c.Assert(msg.ValidateBasicV112(ver), NotNil)
	msg = NewMsgManageMAYAName("myname", common.BNBChain, GetRandomBNBAddress(), coin, 0, common.BNBAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(10000)}, []string{"othername"}, owner, cosmos.AccAddress{})
	c.Assert(msg.ValidateBasicV112(ver), NotNil)
	msg = NewMsgManageMAYAName("myname", common.BNBChain, GetRandomBNBAddress(), coin, 0, common.BNBAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(1000)}, []string{"myname"}, owner, cosmos.AccAddress{})
	c.Assert(msg.ValidateBasicV112(ver), NotNil)
	msg = NewMsgManageMAYAName("myname", common.BNBChain, GetRandomBNBAddress(), coin, 0, common.BNBAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(10000)}, []string{"othername"}, owner, cosmos.AccAddress{})
	c.Assert(msg.ValidateBasicV112(ver), NotNil)

	msg = NewMsgManageMAYAName("myname", common.BNBChain, GetRandomBNBAddress(), coin, 0, common.BNBAsset, EmptyBps, []cosmos.Uint{cosmos.ZeroUint()}, []string{"othername"}, owner, cosmos.AccAddress{})
	c.Assert(msg.ValidateBasicV112(ver), NotNil)
	msg = NewMsgManageMAYAName("myname", common.BNBChain, GetRandomBNBAddress(), coin, 0, common.BNBAsset, cosmos.NewUint(120000), []cosmos.Uint{cosmos.NewUint(1000)}, []string{"othername"}, owner, cosmos.AccAddress{})
	c.Assert(msg.ValidateBasicV112(ver), NotNil)
	msg = NewMsgManageMAYAName("myname", common.BNBChain, GetRandomBNBAddress(), coin, 0, common.BNBAsset, cosmos.NewUint(120000), []cosmos.Uint{cosmos.NewUint(1000)}, []string{"othername"}, owner, cosmos.AccAddress{})
	c.Assert(msg.ValidateBasicV112(ver), NotNil)
	// optional chain and chain alias
	msg = NewMsgManageMAYAName("myname", common.BNBChain, common.NoAddress, coin, 0, common.BNBAsset, cosmos.NewUint(12000), []cosmos.Uint{cosmos.NewUint(1000)}, []string{"othername"}, owner, cosmos.AccAddress{})
	c.Assert(msg.ValidateBasicV112(ver), NotNil)
	msg = NewMsgManageMAYAName("myname", common.EmptyChain, GetRandomBNBAddress(), coin, 0, common.BNBAsset, cosmos.NewUint(12000), []cosmos.Uint{cosmos.NewUint(1000)}, []string{"othername"}, owner, cosmos.AccAddress{})
	c.Assert(msg.ValidateBasicV112(ver), NotNil)

	// Test MAYAName starting with "TIER"
	msg = NewMsgManageMAYAName("TIER1", common.BNBChain, GetRandomBNBAddress(), coin, 0, common.BNBAsset, cosmos.ZeroUint(), []cosmos.Uint{}, []string{}, owner, signer)
	c.Assert(msg.ValidateBasicV112(ver), NotNil)

	// Test subaffiliate name and bps count validation
	// Valid case: equal lengths
	msg = NewMsgManageMAYAName("myname", common.BNBChain, GetRandomBNBAddress(), coin, 0, common.BNBAsset,
		cosmos.ZeroUint(),
		[]cosmos.Uint{cosmos.NewUint(100), cosmos.NewUint(200)},
		[]string{"name1", "name2"},
		owner, signer)
	c.Assert(msg.ValidateBasicV112(ver), IsNil)

	// Valid case: single bps value
	msg = NewMsgManageMAYAName("myname", common.BNBChain, GetRandomBNBAddress(), coin, 0, common.BNBAsset,
		cosmos.ZeroUint(),
		[]cosmos.Uint{cosmos.NewUint(100)},
		[]string{"name1", "name2"},
		owner, signer)
	c.Assert(msg.ValidateBasicV112(ver), IsNil)

	// Invalid case: mismatched lengths and not single bps
	msg = NewMsgManageMAYAName("myname", common.BNBChain, GetRandomBNBAddress(), coin, 0, common.BNBAsset,
		cosmos.ZeroUint(),
		[]cosmos.Uint{cosmos.NewUint(100), cosmos.NewUint(200)},
		[]string{"name1"},
		owner, signer)
	c.Assert(msg.ValidateBasicV112(ver), NotNil)
}
