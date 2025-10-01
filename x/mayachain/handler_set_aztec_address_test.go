package mayachain

import (
	"errors"
	"fmt"

	"github.com/cosmos/cosmos-sdk/types"
	se "github.com/cosmos/cosmos-sdk/types/errors"
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
)

type HandlerSetAztecAddressSuite struct{}

type TestSetAztecAddressKeeper struct {
	keeper.KVStoreDummy
	na         NodeAccount
	hasBalance bool
	ensure     error
}

// Check balance
func (k *TestSetAztecAddressKeeper) HasCoins(_ cosmos.Context, _ cosmos.AccAddress, _ types.Coins) bool {
	return k.hasBalance
}

func (k *TestSetAztecAddressKeeper) SendFromAccountToModule(ctx cosmos.Context, from cosmos.AccAddress, to string, coins common.Coins) error {
	return nil
}

func (k *TestSetAztecAddressKeeper) GetNodeAccount(ctx cosmos.Context, signer cosmos.AccAddress) (NodeAccount, error) {
	return k.na, nil
}

func (k *TestSetAztecAddressKeeper) EnsureAztecAddressUnique(_ cosmos.Context, _ common.Address) error {
	return k.ensure
}

func (k *TestSetAztecAddressKeeper) GetNetwork(ctx cosmos.Context) (Network, error) {
	return NewNetwork(), nil
}

func (k *TestSetAztecAddressKeeper) SetNetwork(ctx cosmos.Context, data Network) error {
	return nil
}

func (k *TestSetAztecAddressKeeper) SetNodeAccount(ctx cosmos.Context, na NodeAccount) error {
	return nil
}

func (k *TestSetAztecAddressKeeper) SendFromModuleToModule(ctx cosmos.Context, from, to string, coins common.Coins) error {
	return nil
}

var _ = Suite(&HandlerSetAztecAddressSuite{})

func (s *HandlerSetAztecAddressSuite) TestValidate(c *C) {
	ctx, _ := setupKeeperForTest(c)

	keeper := &TestSetAztecAddressKeeper{
		na:         GetRandomValidatorNode(NodeStandby),
		hasBalance: true,
		ensure:     nil,
	}
	keeper.na.PubKeySet = common.PubKeySet{}

	handler := NewSetAztecAddressHandler(NewDummyMgrWithKeeper(keeper))

	// happy path
	signer := GetRandomBech32Addr()
	c.Assert(signer.Empty(), Equals, false)
	// consensPubKey := GetRandomBech32ConsensusPubKey()
	// pubKeys := GetRandomPubKeySet()
	aztecAddress := GetRandomAZTECAddress()

	msg := NewMsgSetAztecAddress(aztecAddress, signer)
	err := handler.validate(ctx, *msg)
	c.Assert(err, IsNil)
	result, err := handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)

	// cannot set Aztec Address again
	keeper.na.AztecAddress = aztecAddress
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)

	// cannot set Aztec Address for disabled account
	keeper.na.Status = NodeDisabled
	msg = NewMsgSetAztecAddress(aztecAddress, signer)
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)

	// cannot set Aztec Address when duplicate
	keeper.na.Status = NodeStandby
	keeper.ensure = fmt.Errorf("duplicate keys")
	msg = NewMsgSetAztecAddress(aztecAddress, signer)
	err = handler.validate(ctx, *msg)
	c.Assert(err, ErrorMatches, "node already has aztec address set.*")
	keeper.ensure = nil

	// invalid msg
	msg = &MsgSetAztecAddress{}
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
	result, err = handler.Run(ctx, msg)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)

	result, err = handler.Run(ctx, NewMsgMimir("what", 1, GetRandomBech32Addr()))
	c.Check(err, NotNil)
	c.Check(result, IsNil)
}

type TestSetAztecAddressHandleKeeper struct {
	keeper.Keeper
	failGetNodeAccount bool
	failSetNodeAccount bool
	failGetNetwork     bool
	failSetNetwork     bool
}

func NewTestSetAztecAddressHandleKeeper(k keeper.Keeper) *TestSetAztecAddressHandleKeeper {
	return &TestSetAztecAddressHandleKeeper{
		Keeper: k,
	}
}

func (k *TestSetAztecAddressHandleKeeper) SendFromAccountToModule(ctx cosmos.Context, from cosmos.AccAddress, to string, coins common.Coins) error {
	return nil
}

func (k *TestSetAztecAddressHandleKeeper) GetNodeAccount(ctx cosmos.Context, signer cosmos.AccAddress) (NodeAccount, error) {
	if k.failGetNodeAccount {
		return NodeAccount{}, errKaboom
	}
	return k.Keeper.GetNodeAccount(ctx, signer)
}

func (k *TestSetAztecAddressHandleKeeper) SetNodeAccount(ctx cosmos.Context, na NodeAccount) error {
	if k.failSetNodeAccount {
		return errKaboom
	}
	return k.Keeper.SetNodeAccount(ctx, na)
}

func (k *TestSetAztecAddressHandleKeeper) GetNetwork(ctx cosmos.Context) (Network, error) {
	if k.failGetNetwork {
		return Network{}, errKaboom
	}
	return k.Keeper.GetNetwork(ctx)
}

func (k *TestSetAztecAddressHandleKeeper) SetNetwork(ctx cosmos.Context, data Network) error {
	if k.failSetNetwork {
		return errKaboom
	}
	return k.Keeper.SetNetwork(ctx, data)
}

func (k *TestSetAztecAddressHandleKeeper) EnsureAztecAddressUnique(_ cosmos.Context, _ common.Address) error {
	return nil
}

func (s *HandlerSetAztecAddressSuite) TestHandle(c *C) {
	ctx, mgr := setupManagerForTest(c)
	helper := NewTestSetAztecAddressHandleKeeper(mgr.Keeper())
	mgr.K = helper
	handler := NewSetAztecAddressHandler(mgr)
	ctx = ctx.WithBlockHeight(1)
	signer := GetRandomBech32Addr()

	// add observer
	bondAddr := GetRandomBNBAddress()
	emptyPubKeySet := common.PubKeySet{}
	aztecAddress := GetRandomAZTECAddress()

	msgAztecAddress := NewMsgSetAztecAddress(aztecAddress, signer)

	bond := cosmos.NewUint(common.One * 100)
	nodeAccount := NewNodeAccount(signer, NodeActive, emptyPubKeySet, "", "", bond, bondAddr, ctx.BlockHeight())
	c.Assert(helper.Keeper.SetNodeAccount(ctx, nodeAccount), IsNil)

	nodeAccount = NewNodeAccount(signer, NodeWhiteListed, emptyPubKeySet, "", "", bond, bondAddr, ctx.BlockHeight())
	c.Assert(helper.Keeper.SetNodeAccount(ctx, nodeAccount), IsNil)
	FundModule(c, ctx, helper, BondName, common.One*100)
	// happy path
	_, err := handler.handle(ctx, *msgAztecAddress)
	c.Assert(err, IsNil)
	na, err := helper.Keeper.GetNodeAccount(ctx, msgAztecAddress.Signer)
	c.Assert(err, IsNil)
	c.Assert(na.AztecAddress, Equals, aztecAddress)

	testCases := []struct {
		name              string
		messageProvider   func(c *C, ctx cosmos.Context, helper *TestSetAztecAddressHandleKeeper) cosmos.Msg
		validator         func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *TestSetAztecAddressHandleKeeper, name string)
		skipForNativeRune bool
	}{
		{
			name: "fail to get node account should return an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *TestSetAztecAddressHandleKeeper) cosmos.Msg {
				helper.failGetNodeAccount = true
				return NewMsgSetAztecAddress(GetRandomAZTECAddress(), nodeAccount.NodeAddress)
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *TestSetAztecAddressHandleKeeper, name string) {
				c.Check(result, IsNil, Commentf(name))
				c.Check(err, NotNil, Commentf(name))
				c.Check(errors.Is(err, se.ErrUnauthorized), Equals, true)
			},
		},
		{
			name: "node account is empty should return an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *TestSetAztecAddressHandleKeeper) cosmos.Msg {
				nodeAcct := NewNodeAccount(signer, NodeWhiteListed, emptyPubKeySet, "", "", bond, bondAddr, ctx.BlockHeight())
				return NewMsgSetAztecAddress(aztecAddress, nodeAcct.NodeAddress)
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *TestSetAztecAddressHandleKeeper, name string) {
				c.Check(result, IsNil, Commentf(name))
				c.Check(err, NotNil, Commentf(name))
				c.Check(errors.Is(err, se.ErrUnauthorized), Equals, true)
			},
		},
		{
			name: "fail to save node account should return an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *TestSetAztecAddressHandleKeeper) cosmos.Msg {
				nodeAcct := GetRandomValidatorNode(NodeWhiteListed)
				c.Assert(helper.Keeper.SetNodeAccount(ctx, nodeAcct), IsNil)
				helper.failSetNodeAccount = true
				return NewMsgSetAztecAddress(GetRandomAZTECAddress(), nodeAcct.NodeAddress)
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *TestSetAztecAddressHandleKeeper, name string) {
				c.Check(result, IsNil, Commentf(name))
				c.Check(err, NotNil, Commentf(name))
			},
		},
		{
			name: "fail to get network data should return an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *TestSetAztecAddressHandleKeeper) cosmos.Msg {
				nodeAcct := GetRandomValidatorNode(NodeWhiteListed)
				c.Assert(helper.Keeper.SetNodeAccount(ctx, nodeAcct), IsNil)
				helper.failGetNetwork = true
				return NewMsgSetAztecAddress(GetRandomAZTECAddress(), nodeAcct.NodeAddress)
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *TestSetAztecAddressHandleKeeper, name string) {
				c.Check(result, IsNil, Commentf(name))
				c.Check(err, NotNil, Commentf(name))
			},
			skipForNativeRune: true,
		},
		{
			name: "fail to set network data should return an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *TestSetAztecAddressHandleKeeper) cosmos.Msg {
				nodeAcct := GetRandomValidatorNode(NodeWhiteListed)
				c.Assert(helper.Keeper.SetNodeAccount(ctx, nodeAcct), IsNil)
				helper.failSetNetwork = true
				return NewMsgSetAztecAddress(GetRandomAZTECAddress(), nodeAcct.NodeAddress)
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *TestSetAztecAddressHandleKeeper, name string) {
				c.Check(result, IsNil, Commentf(name))
				c.Check(err, NotNil, Commentf(name))
			},
			skipForNativeRune: true,
		},
	}
	for _, tc := range testCases {
		if common.BaseAsset().Native() != "" && tc.skipForNativeRune {
			continue
		}
		ctx, mgr := setupManagerForTest(c)
		helper := NewTestSetAztecAddressHandleKeeper(mgr.Keeper())
		mgr.K = helper
		handler := NewSetAztecAddressHandler(mgr)
		msg := tc.messageProvider(c, ctx, helper)
		result, err := handler.Run(ctx, msg)
		tc.validator(c, ctx, result, err, helper, tc.name)
	}
}
