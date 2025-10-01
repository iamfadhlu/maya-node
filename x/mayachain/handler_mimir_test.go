package mayachain

import (
	"strings"

	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

type HandlerMimirSuite struct{}

var _ = Suite(&HandlerMimirSuite{})

func (s *HandlerMimirSuite) SetUpSuite(c *C) {
	SetupConfigForTest()
}

func (s *HandlerMimirSuite) TestValidate(c *C) {
	ctx, keeper := setupKeeperForTest(c)

	addr, _ := cosmos.AccAddressFromBech32(ADMINS[0])
	handler := NewMimirHandler(NewDummyMgrWithKeeper(keeper))
	// happy path
	msg := NewMsgMimir("foo", 44, addr)
	err := handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	// invalid msg
	msg = &MsgMimir{}
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
}

func (s *HandlerMimirSuite) TestMimirHandle(c *C) {
	ctx, keeper := setupKeeperForTest(c)
	handler := NewMimirHandler(NewDummyMgrWithKeeper(keeper))
	addr, err := cosmos.AccAddressFromBech32(ADMINS[0])
	c.Assert(err, IsNil)
	msg := NewMsgMimir("foo", 55, addr)
	sdkErr := handler.handle(ctx, *msg)
	c.Assert(sdkErr, IsNil)
	val, err := keeper.GetMimir(ctx, "foo")
	c.Assert(err, IsNil)
	c.Check(val, Equals, int64(55))

	invalidMsg := NewMsgNetworkFee(ctx.BlockHeight(), common.BNBChain, 1, bnbSingleTxFee.Uint64(), GetRandomBech32Addr())
	result, err := handler.Run(ctx, invalidMsg)
	c.Check(err, NotNil)
	c.Check(result, IsNil)

	msg.Signer = GetRandomBech32Addr()
	result, err = handler.Run(ctx, msg)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)

	msg1 := NewMsgMimir("hello", 1, addr)
	result, err = handler.Run(ctx, msg1)
	c.Check(err, IsNil)
	c.Check(result, NotNil)

	val, err = keeper.GetMimir(ctx, "hello")
	c.Assert(err, IsNil)
	c.Assert(val, Equals, int64(1))

	// delete mimir
	msg1 = NewMsgMimir("hello", -3, addr)
	result, err = handler.Run(ctx, msg1)
	c.Check(err, IsNil)
	c.Check(result, NotNil)
	val, err = keeper.GetMimir(ctx, "hello")
	c.Assert(err, IsNil)
	c.Assert(val, Equals, int64(-1))

	// Test IBC mimir params
	IBCReceiveEnabled := strings.ToUpper("IBCReceiveEnabled")
	IBCSendEnabled := strings.ToUpper("IBCSendEnabled")

	msg = NewMsgMimir(IBCReceiveEnabled, 1, addr)
	sdkErr = handler.handle(ctx, *msg)
	val, err = keeper.GetMimir(ctx, IBCReceiveEnabled)
	c.Assert(sdkErr, IsNil)
	c.Assert(err, IsNil)
	c.Check(val, Equals, int64(1))

	msg = NewMsgMimir(IBCSendEnabled, 1, addr)
	sdkErr = handler.handle(ctx, *msg)
	val, err = keeper.GetMimir(ctx, IBCSendEnabled)
	c.Assert(sdkErr, IsNil)
	c.Assert(err, IsNil)
	c.Check(val, Equals, int64(1))

	params := keeper.GetIBCTransferParams(ctx)
	c.Check(params.ReceiveEnabled, Equals, true)
	c.Check(params.SendEnabled, Equals, true)

	msg = NewMsgMimir(IBCReceiveEnabled, 0, addr)
	sdkErr = handler.handle(ctx, *msg)
	val, err = keeper.GetMimir(ctx, IBCReceiveEnabled)
	c.Assert(sdkErr, IsNil)
	c.Assert(err, IsNil)
	c.Check(val, Equals, int64(0))

	msg = NewMsgMimir(IBCSendEnabled, 0, addr)
	sdkErr = handler.handle(ctx, *msg)
	val, err = keeper.GetMimir(ctx, IBCSendEnabled)
	c.Assert(sdkErr, IsNil)
	c.Assert(err, IsNil)
	c.Check(val, Equals, int64(0))

	params = keeper.GetIBCTransferParams(ctx)
	c.Check(params.ReceiveEnabled, Equals, false)
	c.Check(params.SendEnabled, Equals, false)

	// node set mimir
	FundModule(c, ctx, keeper, BondName, 100*common.One)
	ver := "1.92.0"
	na1 := GetRandomValidatorNode(NodeActive)
	na1.Version = ver
	na2 := GetRandomValidatorNode(NodeActive)
	na2.Version = ver
	na3 := GetRandomValidatorNode(NodeActive)
	na3.Version = ver
	c.Assert(keeper.SetNodeAccount(ctx, na1), IsNil)
	c.Assert(keeper.SetNodeAccount(ctx, na2), IsNil)
	c.Assert(keeper.SetNodeAccount(ctx, na3), IsNil)

	FundAccount(c, ctx, keeper, na1.NodeAddress, 5*common.One)
	FundAccount(c, ctx, keeper, na2.NodeAddress, 5*common.One)
	FundAccount(c, ctx, keeper, na3.NodeAddress, 5*common.One)

	// first node set mimir , no consensus
	result, err = handler.Run(ctx, NewMsgMimir("ACCEPT_TEST", 1, na1.NodeAddress))
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
	mvalue, err := keeper.GetMimir(ctx, "ACCEPT_TEST")
	c.Assert(err, IsNil)
	c.Assert(mvalue, Equals, int64(-1))

	// value different than SymbolicNodeMimirValue should give an error for nodes
	_, err = handler.Run(ctx, NewMsgMimir("MinimumNodesForYggdrasil", 1, na1.NodeAddress))
	c.Assert(err, Equals, errInvalidSymbolicNodeMimirValue)
	_, err = handler.Run(ctx, NewMsgMimir("MinimumNodesForYggdrasil", 1, na2.NodeAddress))
	c.Assert(err, Equals, errInvalidSymbolicNodeMimirValue)
	_, err = handler.Run(ctx, NewMsgMimir("MinimumNodesForYggdrasil", 1, na3.NodeAddress))
	c.Assert(err, Equals, errInvalidSymbolicNodeMimirValue)

	// second node set mimir, reach consensus
	result, err = handler.Run(ctx, NewMsgMimir("ACCEPT_TEST", 1, na2.NodeAddress))
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)

	mvalue, err = keeper.GetMimir(ctx, "ACCEPT_TEST")
	c.Assert(err, IsNil)
	c.Assert(mvalue, Equals, int64(1))

	// third node set mimir, reach consensus
	result, err = handler.Run(ctx, NewMsgMimir("ACCEPT_TEST", 1, na3.NodeAddress))
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)

	mvalue, err = keeper.GetMimir(ctx, "ACCEPT_TEST")
	c.Assert(err, IsNil)
	c.Assert(mvalue, Equals, int64(1))

	// third node vote mimir to a different value, it should not change the admin mimir value
	result, err = handler.Run(ctx, NewMsgMimir("ACCEPT_TEST", 0, na3.NodeAddress))
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)

	mvalue, err = keeper.GetMimir(ctx, "ACCEPT_TEST")
	c.Assert(err, IsNil)
	c.Assert(mvalue, Equals, int64(1))

	// second node vote mimir to a different value , it should update admin mimir
	result, err = handler.Run(ctx, NewMsgMimir("ACCEPT_TEST", 0, na2.NodeAddress))
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)

	mvalue, err = keeper.GetMimir(ctx, "ACCEPT_TEST")
	c.Assert(err, IsNil)
	c.Assert(mvalue, Equals, int64(0))

	result, err = handler.Run(ctx, NewMsgMimir("ACCEPT_TEST-1", 0, na2.NodeAddress))
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
}
