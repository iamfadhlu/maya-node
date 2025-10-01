package types

import (
	"gitlab.com/mayachain/mayanode/common"
	cosmos "gitlab.com/mayachain/mayanode/common/cosmos"
	. "gopkg.in/check.v1"
)

type MsgUnBondSuite struct{}

var _ = Suite(&MsgUnBondSuite{})

func (mas *MsgUnBondSuite) SetUpSuite(c *C) {
	SetupConfigForTest()
}

func (MsgUnBondSuite) TestMsgUnBond(c *C) {
	nodeAddr := GetRandomBech32Addr()
	txId := GetRandomTxHash()
	c.Check(txId.IsEmpty(), Equals, false)
	signerAddr := GetRandomBech32Addr()
	bondAddr := GetRandomBNBAddress()
	providerAddr := GetRandomBech32Addr()
	txin := GetRandomTx()
	txinNoID := txin
	txinNoID.ID = ""
	msgApply := NewMsgUnBond(txin, nodeAddr, bondAddr, nil, signerAddr, common.BNBAsset, cosmos.NewUint(1000))
	c.Assert(msgApply.ValidateBasic(), IsNil)
	c.Assert(msgApply.Route(), Equals, RouterKey)
	c.Assert(msgApply.Type(), Equals, "unbond")
	c.Assert(msgApply.GetSignBytes(), NotNil)
	c.Assert(len(msgApply.GetSigners()), Equals, 1)
	c.Assert(msgApply.GetSigners()[0].Equals(signerAddr), Equals, true)
	c.Assert(NewMsgUnBond(txin, cosmos.AccAddress{}, bondAddr, nil, signerAddr, common.EmptyAsset, cosmos.ZeroUint()).ValidateBasic(), NotNil)
	c.Assert(NewMsgUnBond(txin, nodeAddr, bondAddr, nil, signerAddr, common.EmptyAsset, cosmos.ZeroUint()).ValidateBasic(), IsNil)
	c.Assert(NewMsgUnBond(txinNoID, nodeAddr, bondAddr, nil, signerAddr, common.EmptyAsset, cosmos.ZeroUint()).ValidateBasic(), NotNil)
	c.Assert(NewMsgUnBond(txin, nodeAddr, "", nil, signerAddr, common.EmptyAsset, cosmos.ZeroUint()).ValidateBasic(), NotNil)
	c.Assert(NewMsgUnBond(txin, nodeAddr, bondAddr, nil, cosmos.AccAddress{}, common.EmptyAsset, cosmos.ZeroUint()).ValidateBasic(), NotNil)
	c.Assert(NewMsgUnBond(txin, nodeAddr, bondAddr, providerAddr, signerAddr, common.BNBAsset, cosmos.ZeroUint()).ValidateBasic(), NotNil)
	c.Assert(NewMsgUnBond(txin, nodeAddr, bondAddr, nil, signerAddr, common.BNBAsset, cosmos.ZeroUint()).ValidateBasic(), IsNil)
	c.Assert(NewMsgUnBond(txin, nodeAddr, bondAddr, nil, signerAddr, common.EmptyAsset, cosmos.NewUint(1000)).ValidateBasic(), NotNil)
}
