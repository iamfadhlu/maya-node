package mayachain

import (
	"gitlab.com/mayachain/mayanode/common"
	cosmos "gitlab.com/mayachain/mayanode/common/cosmos"

	. "gopkg.in/check.v1"
)

type HandlerSendSuiteV87 struct{}

var _ = Suite(&HandlerSendSuiteV87{})

func (s *HandlerSendSuiteV87) TestValidate(c *C) {
	ctx, k := setupKeeperForTest(c)

	addr1 := GetRandomBech32Addr()
	addr2 := GetRandomBech32Addr()

	msg := MsgSend{
		FromAddress: addr1,
		ToAddress:   addr2,
		Amount:      cosmos.NewCoins(cosmos.NewCoin("dummy", cosmos.NewInt(12))),
	}
	handler := NewSendHandler(NewDummyMgrWithKeeper(k))
	err := handler.validate(ctx, msg)
	c.Assert(err, IsNil)

	for _, moduleName := range []string{AsgardName, BondName, ReserveName, ModuleName} {
		msg.ToAddress = k.GetModuleAccAddress(moduleName)
		err = handler.validate(ctx, msg)
		c.Assert(err, NotNil, Commentf("cannot send to module: %s", moduleName))
	}

	founders, err := common.NewAddress(FOUNDERS, GetCurrentVersion())
	c.Assert(err, IsNil)
	foundersAcc, err := founders.AccAddress()
	c.Assert(err, IsNil)
	msg = MsgSend{
		FromAddress: foundersAcc,
		ToAddress:   addr2,
		Amount:      cosmos.NewCoins(cosmos.NewCoin("maya", cosmos.NewInt(12))),
	}
	err = handler.validate(ctx, msg)
	c.Assert(err.Error(), Equals, "cannot send maya from founders address")

	// invalid msg
	msg = MsgSend{}
	err = handler.validate(ctx, msg)
	c.Assert(err, NotNil)
}

func (s *HandlerSendSuiteV87) TestHandle(c *C) {
	ctx, k := setupKeeperForTest(c)

	addr1 := GetRandomBech32Addr()
	addr2 := GetRandomBech32Addr()

	funds, err := common.NewCoin(common.BaseNative, cosmos.NewUint(200*common.One)).Native()
	c.Assert(err, IsNil)
	err = k.AddCoins(ctx, addr1, cosmos.NewCoins(funds))
	c.Assert(err, IsNil)

	coin, err := common.NewCoin(common.BaseNative, cosmos.NewUint(12*common.One)).Native()
	c.Assert(err, IsNil)
	msg := MsgSend{
		FromAddress: addr1,
		ToAddress:   addr2,
		Amount:      cosmos.NewCoins(coin),
	}

	handler := NewSendHandler(NewDummyMgrWithKeeper(k))
	_, err = handler.handle(ctx, msg)
	c.Assert(err, IsNil)

	// invalid msg should result in a error
	result, err := handler.Run(ctx, NewMsgNetworkFee(ctx.BlockHeight(), common.BNBChain, 1, bnbSingleTxFee.Uint64(), GetRandomBech32Addr()))
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)
	// insufficient funds
	coin, err = common.NewCoin(common.BaseNative, cosmos.NewUint(3000*common.One)).Native()
	c.Assert(err, IsNil)
	msg = MsgSend{
		FromAddress: addr1,
		ToAddress:   addr2,
		Amount:      cosmos.NewCoins(coin),
	}
	_, err = handler.handle(ctx, msg)
	c.Assert(err, NotNil)
}
