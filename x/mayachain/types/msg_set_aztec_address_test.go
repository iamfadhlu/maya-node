package types

import (
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common/cosmos"
)

type MsgSetAztecAddressSuite struct{}

var _ = Suite(&MsgSetAztecAddressSuite{})

func (MsgSetAztecAddressSuite) TestMsgSetAztecAddress(c *C) {
	acc1 := GetRandomBech32Addr()
	aztecAddress := GetRandomAZTECAddress()
	c.Assert(acc1.Empty(), Equals, false)
	msgSetAztecAddress := NewMsgSetAztecAddress(aztecAddress, acc1)
	c.Assert(msgSetAztecAddress.Route(), Equals, RouterKey)
	c.Assert(msgSetAztecAddress.Type(), Equals, "set_aztec_address")
	c.Assert(msgSetAztecAddress.ValidateBasic(), IsNil)
	c.Assert(len(msgSetAztecAddress.GetSignBytes()) > 0, Equals, true)
	c.Assert(msgSetAztecAddress.GetSigners(), NotNil)
	c.Assert(msgSetAztecAddress.GetSigners()[0].String(), Equals, acc1.String())
	emptySigner := NewMsgSetAztecAddress(aztecAddress, cosmos.AccAddress{})
	c.Assert(emptySigner.ValidateBasic(), NotNil)
	emptyAztecAddress := NewMsgSetAztecAddress("", acc1)
	c.Assert(emptyAztecAddress.ValidateBasic(), NotNil)
}
