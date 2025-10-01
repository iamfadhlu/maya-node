package types

import (
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	. "gopkg.in/check.v1"
)

type MsgTradeAccountSuite struct{}

var _ = Suite(&MsgTradeAccountSuite{})

func (MsgTradeAccountSuite) TestDeposit(c *C) {
	asset := common.ETHAsset
	amt := cosmos.NewUint(100)
	signer := GetRandomBech32Addr()
	dummyTx := common.Tx{ID: "test"}

	m := NewMsgTradeAccountDeposit(asset, amt, signer, signer, dummyTx)
	EnsureMsgBasicCorrect(m, c)

	m = NewMsgTradeAccountDeposit(common.EmptyAsset, amt, signer, signer, dummyTx)
	c.Check(m.ValidateBasic(), NotNil)

	m = NewMsgTradeAccountDeposit(common.BaseNative, amt, signer, signer, dummyTx)
	c.Check(m.ValidateBasic(), NotNil)

	m = NewMsgTradeAccountDeposit(asset, cosmos.ZeroUint(), signer, signer, dummyTx)
	c.Check(m.ValidateBasic(), NotNil)
}

func (MsgTradeAccountSuite) TestWithdrawal(c *C) {
	version := GetCurrentVersion()
	asset := common.ETHAsset.GetTradeAsset()
	amt := cosmos.NewUint(100)
	ethAddr := GetRandomETHAddress()
	signer := GetRandomBech32Addr()
	dummyTx := common.Tx{ID: "test"}

	m := NewMsgTradeAccountWithdrawal(asset, amt, ethAddr, signer, dummyTx)
	EnsureMsgBasicCorrect(m, c)

	m = NewMsgTradeAccountWithdrawal(common.EmptyAsset, amt, ethAddr, signer, dummyTx)
	c.Check(m.ValidateBasicVersioned(version), NotNil)

	m = NewMsgTradeAccountWithdrawal(common.BaseNative, amt, ethAddr, signer, dummyTx)
	c.Check(m.ValidateBasicVersioned(version), NotNil)

	m = NewMsgTradeAccountWithdrawal(asset, cosmos.ZeroUint(), ethAddr, signer, dummyTx)
	c.Check(m.ValidateBasicVersioned(version), NotNil)

	m = NewMsgTradeAccountWithdrawal(asset, cosmos.ZeroUint(), GetRandomBaseAddress(), signer, dummyTx)
	c.Check(m.ValidateBasicVersioned(version), NotNil)
}
