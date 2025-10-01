package mayachain

import (
	"errors"
	"fmt"
	"strings"

	se "github.com/cosmos/cosmos-sdk/types/errors"
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
)

type HandlerBondSuite struct{}

type TestBondKeeper struct {
	keeper.Keeper
	standbyNodeAccount  NodeAccount
	failGetNodeAccount  NodeAccount
	notEmptyNodeAccount NodeAccount
}

func (k *TestBondKeeper) GetNodeAccount(_ cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error) {
	if k.standbyNodeAccount.NodeAddress.Equals(addr) {
		return k.standbyNodeAccount, nil
	}
	if k.failGetNodeAccount.NodeAddress.Equals(addr) {
		return NodeAccount{}, fmt.Errorf("you asked for this error")
	}
	if k.notEmptyNodeAccount.NodeAddress.Equals(addr) {
		return k.notEmptyNodeAccount, nil
	}
	return NodeAccount{}, nil
}

var _ = Suite(&HandlerBondSuite{})

func (HandlerBondSuite) TestBondHandler_ValidateActive(c *C) {
	ctx, k := setupKeeperForTest(c)

	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	c.Assert(k.SetNodeAccount(ctx, activeNodeAccount), IsNil)

	vault := GetRandomVault()
	vault.Status = RetiringVault
	c.Assert(k.SetVault(ctx, vault), IsNil)

	handler := NewBondHandler(NewDummyMgrWithKeeper(k))

	txIn := common.NewTx(
		GetRandomTxHash(),
		GetRandomBNBAddress(),
		GetRandomBNBAddress(),
		common.Coins{
			common.NewCoin(common.BaseAsset(), cosmos.NewUint(10*common.One)),
		},
		BNBGasFeeSingleton,
		"bond",
	)
	lp, _ := SetupLiquidityBondForTest(c, ctx, k, common.BTCAsset, activeNodeAccount.BondAddress, activeNodeAccount, cosmos.NewUint(100*common.One))
	// Remove the bond from the node account
	lp.NodeBondAddress = nil
	k.SetLiquidityProvider(ctx, lp)
	msg := NewMsgBond(txIn, activeNodeAccount.NodeAddress, txIn.Coins[0].Amount, lp.CacaoAddress, nil, activeNodeAccount.NodeAddress, -1, common.BTCAsset, cosmos.NewUint(100))

	// happy path
	c.Assert(handler.validate(ctx, *msg), IsNil)

	vault.Status = ActiveVault
	c.Assert(k.SetVault(ctx, vault), IsNil)

	// node should be able to bond even it is active
	c.Assert(handler.validate(ctx, *msg), IsNil)
}

func (HandlerBondSuite) TestBondHandler_Run(c *C) {
	ctx, k1 := setupKeeperForTest(c)

	standbyNodeAccount := GetRandomValidatorNode(NodeStandby)
	k := &TestBondKeeper{
		Keeper:              k1,
		standbyNodeAccount:  standbyNodeAccount,
		failGetNodeAccount:  GetRandomValidatorNode(NodeStandby),
		notEmptyNodeAccount: GetRandomValidatorNode(NodeStandby),
	}
	c.Assert(k1.SetNodeAccount(ctx, standbyNodeAccount), IsNil)

	// happy path
	handler := NewBondHandler(NewDummyMgrWithKeeper(k1))
	txIn := common.NewTx(
		GetRandomTxHash(),
		GetRandomBaseAddress(),
		GetRandomBaseAddress(),
		common.Coins{
			common.NewCoin(common.BaseAsset(), cosmos.NewUint(10*common.One)),
		},
		common.Gas{
			common.NewCoin(common.BaseNative, cosmos.NewUint(200000)),
		},
		"bond",
	)
	amt := cosmos.NewUint(100 * common.One)

	na := GetRandomValidatorNode(NodeUnknown)
	bp := NewBondProviders(na.NodeAddress)
	acc, err := na.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp.Providers = append(bp.Providers, NewBondProvider(acc))
	bp.Providers[0].Bonded = true
	FundModule(c, ctx, k1, BondName, common.One)
	lp, _ := SetupLiquidityBondForTest(c, ctx, k1, common.BTCAsset, na.BondAddress, na, amt)
	// Remove the bond from the node account
	lp.NodeBondAddress = nil
	k.SetLiquidityProvider(ctx, lp)
	c.Assert(k.SetBondProviders(ctx, bp), IsNil)
	c.Assert(k.SetNodeAccount(ctx, na), IsNil)
	msg := NewMsgBond(txIn, na.NodeAddress, cosmos.NewUint(common.One), lp.CacaoAddress, nil, standbyNodeAccount.NodeAddress, -1, common.BTCAsset, amt)
	_, err = handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	coin := common.NewCoin(common.BaseNative, cosmos.NewUint(common.One))
	nativeRuneCoin, err := coin.Native()
	c.Assert(err, IsNil)
	c.Assert(k1.HasCoins(ctx, msg.NodeAddress, cosmos.NewCoins(nativeRuneCoin)), Equals, true)
	na, err = k1.GetNodeAccount(ctx, msg.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(na.Status.String(), Equals, NodeWhiteListed.String())
	nodeBond, err := k1.CalcNodeLiquidityBond(ctx, na)
	c.Assert(err, IsNil)
	c.Assert(nodeBond.Uint64(), Equals, amt.MulUint64(2).Uint64(), Commentf("%d\n", nodeBond.Uint64()))

	// simulate fail to get node account
	handler = NewBondHandler(NewDummyMgrWithKeeper(k))
	msg = NewMsgBond(txIn, k.failGetNodeAccount.NodeAddress, cosmos.NewUint(common.One), GetRandomBaseAddress(), nil, standbyNodeAccount.NodeAddress, -1, common.BTCAsset, cosmos.NewUint(1000))
	_, err = handler.Run(ctx, msg)
	c.Assert(errors.Is(err, errInternal), Equals, true)

	// When node account is standby , it is ok to bond
	lp, _ = SetupLiquidityBondForTest(c, ctx, k, common.BTCAsset, k.notEmptyNodeAccount.BondAddress, k.notEmptyNodeAccount, amt)
	// Remove the bond from the node account
	lp.NodeBondAddress = nil
	k.SetLiquidityProvider(ctx, lp)
	msg = NewMsgBond(txIn, k.notEmptyNodeAccount.NodeAddress, cosmos.NewUint(common.One), lp.CacaoAddress, nil, k.notEmptyNodeAccount.NodeAddress, -1, common.BTCAsset, cosmos.NewUint(1000))
	_, err = handler.Run(ctx, msg)
	c.Assert(err, IsNil)
}

func (HandlerBondSuite) TestBondHandlerFailValidation(c *C) {
	ctx, k := setupKeeperForTest(c)
	standbyNodeAccount := GetRandomValidatorNode(NodeStandby)
	c.Assert(k.SetNodeAccount(ctx, standbyNodeAccount), IsNil)
	handler := NewBondHandler(NewDummyMgrWithKeeper(k))
	txIn := common.NewTx(
		GetRandomTxHash(),
		GetRandomBaseAddress(),
		GetRandomBaseAddress(),
		common.Coins{
			common.NewCoin(common.BaseAsset(), cosmos.NewUint(10*common.One)),
		},
		BNBGasFeeSingleton,
		"apply",
	)
	txInNoTxID := txIn
	txInNoTxID.ID = ""
	testCases := []struct {
		name        string
		msg         *MsgBond
		expectedErr error
	}{
		{
			name:        "empty node address",
			msg:         NewMsgBond(txIn, cosmos.AccAddress{}, cosmos.NewUint(common.One), GetRandomBaseAddress(), nil, standbyNodeAccount.NodeAddress, -1, common.BTCAsset, cosmos.NewUint(1000)),
			expectedErr: se.ErrInvalidAddress,
		},
		{
			name:        "zero bond",
			msg:         NewMsgBond(txIn, GetRandomValidatorNode(NodeStandby).NodeAddress, cosmos.NewUint(common.One), GetRandomBaseAddress(), nil, standbyNodeAccount.NodeAddress, -1, common.BTCAsset, cosmos.ZeroUint()),
			expectedErr: se.ErrUnknownRequest,
		},
		{
			name:        "empty bond address",
			msg:         NewMsgBond(txIn, GetRandomValidatorNode(NodeStandby).NodeAddress, cosmos.NewUint(common.One), common.Address(""), nil, standbyNodeAccount.NodeAddress, -1, common.BTCAsset, cosmos.NewUint(1000)),
			expectedErr: se.ErrInvalidAddress,
		},
		{
			name:        "empty request hash",
			msg:         NewMsgBond(txInNoTxID, GetRandomValidatorNode(NodeStandby).NodeAddress, cosmos.NewUint(common.One), GetRandomBaseAddress(), nil, standbyNodeAccount.NodeAddress, -1, common.BTCAsset, cosmos.NewUint(1000)),
			expectedErr: se.ErrUnknownRequest,
		},
		{
			name:        "empty signer",
			msg:         NewMsgBond(txIn, GetRandomValidatorNode(NodeStandby).NodeAddress, cosmos.NewUint(common.One), GetRandomBaseAddress(), nil, cosmos.AccAddress{}, -1, common.BTCAsset, cosmos.NewUint(1000)),
			expectedErr: se.ErrInvalidAddress,
		},
		{
			name:        "active node",
			msg:         NewMsgBond(txIn, GetRandomValidatorNode(NodeActive).NodeAddress, cosmos.NewUint(common.One), GetRandomBNBAddress(), nil, cosmos.AccAddress{}, -1, common.BTCAsset, cosmos.NewUint(1000)),
			expectedErr: se.ErrInvalidAddress,
		},
	}
	for _, item := range testCases {
		c.Log(item.name)
		_, err := handler.Run(ctx, item.msg)
		c.Check(errors.Is(err, item.expectedErr), Equals, true, Commentf("name: %s, %s != %s", item.name, item.expectedErr, err))
	}
}

func (HandlerBondSuite) TestBondProvider_Validate(c *C) {
	ctx, k := setupKeeperForTest(c)
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	c.Assert(k.SetNodeAccount(ctx, activeNodeAccount), IsNil)
	standbyNodeAccount := GetRandomValidatorNode(NodeStandby)
	c.Assert(k.SetNodeAccount(ctx, standbyNodeAccount), IsNil)
	handler := NewBondHandler(NewDummyMgrWithKeeper(k))
	txIn := GetRandomTx()
	amt := cosmos.NewUint(100 * common.One)
	txIn.Coins = common.NewCoins(common.NewCoin(common.BaseAsset(), amt))
	activeNA := activeNodeAccount.NodeAddress
	activeNAAddress := common.Address(activeNA.String())
	standbyNA := standbyNodeAccount.NodeAddress
	standbyNAAddress := common.Address(standbyNA.String())
	additionalBondAddress := GetRandomBech32Addr()

	errCheck := func(c *C, err error, str string) {
		c.Assert(err, NotNil, Commentf("expected error \"%s\" but got nil", str))
		c.Check(strings.Contains(err.Error(), str), Equals, true, Commentf("%s != %w", str, err))
	}

	// TEST VALIDATION //

	// bond without liquidity
	msg := NewMsgBond(txIn, standbyNA, cosmos.NewUint(common.One), standbyNAAddress, nil, standbyNA, -1, common.BTCAsset, cosmos.NewUint(1000))
	err := handler.validate(ctx, *msg)
	errCheck(c, err, "no free liquidity units in pool to bond")

	// bond again after already bonded
	addr1 := GetRandomBaseAddress()
	na1 := GetRandomValidatorNode(NodeStandby)
	lp, _ := SetupLiquidityBondForTest(c, ctx, k, common.BTCAsset, addr1, na1, cosmos.NewUint(1000))
	// Set the deprecated "NodeBondAddress" to dedicate all of the LP to the node account
	lp.NodeBondAddress = standbyNodeAccount.NodeAddress
	k.SetLiquidityProvider(ctx, lp)
	msg = NewMsgBond(txIn, na1.NodeAddress, cosmos.NewUint(common.One), lp.CacaoAddress, nil, na1.NodeAddress, -1, common.BTCAsset, cosmos.NewUint(1000))
	err = handler.validate(ctx, *msg)
	errCheck(c, err, "no free liquidity units in pool to bond")

	// bond more than the remaining liquidity
	lp.NodeBondAddress = nil
	lp.Bond(na1.NodeAddress, cosmos.NewUint(100))
	k.SetLiquidityProvider(ctx, lp)
	msg = NewMsgBond(txIn, na1.NodeAddress, cosmos.NewUint(common.One), lp.CacaoAddress, nil, na1.NodeAddress, -1, common.BTCAsset, cosmos.NewUint(1000))
	err = handler.validate(ctx, *msg)
	errCheck(c, err, "insufficient free liquidity units in pool to bond")

	// happy path
	msg = NewMsgBond(txIn, standbyNA, cosmos.NewUint(common.One), standbyNodeAccount.BondAddress, additionalBondAddress, standbyNA, -1, common.EmptyAsset, cosmos.ZeroUint())
	err = handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	// try to bond while node account is active
	lp, _ = SetupLiquidityBondForTest(c, ctx, k, common.BTCAsset, activeNAAddress, activeNodeAccount, amt)
	// Remove the bond from the node account
	lp.NodeBondAddress = nil
	k.SetLiquidityProvider(ctx, lp)
	msg = NewMsgBond(txIn, activeNA, cosmos.NewUint(common.One), lp.CacaoAddress, nil, activeNA, -1, common.BTCAsset, cosmos.NewUint(1000))
	err = handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	// try to bond with a bnb address
	lp, _ = SetupLiquidityBondForTest(c, ctx, k, common.BTCAsset, GetRandomBaseAddress(), standbyNodeAccount, amt)
	// Remove the bond from the node account
	lp.NodeBondAddress = nil
	k.SetLiquidityProvider(ctx, lp)
	msg = NewMsgBond(txIn, standbyNA, cosmos.NewUint(common.One), lp.AssetAddress, nil, activeNA, -1, common.BTCAsset, cosmos.NewUint(1000))
	err = handler.validate(ctx, *msg)
	errCheck(c, err, "bonding address is NOT a BASEChain address")

	// try to bond with a valid additional bond provider
	bp := NewBondProviders(standbyNA)
	bp.Providers = []BondProvider{NewBondProvider(standbyNA), NewBondProvider(additionalBondAddress)}
	c.Assert(k.SetBondProviders(ctx, bp), IsNil)
	lp, _ = SetupLiquidityBondForTest(c, ctx, k, common.BTCAsset, common.Address(additionalBondAddress.String()), standbyNodeAccount, amt)
	// Remove the bond from the node account
	lp.NodeBondAddress = nil
	k.SetLiquidityProvider(ctx, lp)
	msg = NewMsgBond(txIn, standbyNA, cosmos.NewUint(common.One), lp.CacaoAddress, nil, activeNA, -1, common.BTCAsset, cosmos.NewUint(1000))
	err = handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	// try to bond with an invalid additional bond provider
	lp, _ = SetupLiquidityBondForTest(c, ctx, k, common.BTCAsset, GetRandomBaseAddress(), standbyNodeAccount, amt)
	// Remove the bond from the node account
	lp.NodeBondAddress = nil
	k.SetLiquidityProvider(ctx, lp)
	msg = NewMsgBond(txIn, standbyNA, cosmos.NewUint(common.One), lp.CacaoAddress, nil, activeNA, -1, common.BTCAsset, cosmos.NewUint(1000))
	err = handler.validate(ctx, *msg)
	errCheck(c, err, "address is not a valid bond provider for this node")

	// try to bond with MaximumBondInRune set
	bp.Providers[1].Bonded = false
	maxBond := int64(1_00000000)
	handler.mgr.Keeper().SetMimir(ctx, "MaximumBondInRune", maxBond)
	c.Assert(k.SetBondProviders(ctx, bp), IsNil)
	msg = NewMsgBond(txIn, standbyNA, cosmos.NewUint(common.One), common.Address(additionalBondAddress.String()), nil, activeNA, -1, common.BTCAsset, cosmos.NewUint(100*common.One))
	err = handler.validate(ctx, *msg)
	errCheck(c, err, fmt.Sprintf("too much bond, max validator bond (%s), bond(%s)", cosmos.NewUint(uint64(maxBond)).String(), amt.MulUint64(2).String()))

	// try to bond with MaximumLPBondedNodes set
	lp, _ = SetupLiquidityBondForTest(c, ctx, k, common.BTCAsset, common.Address(additionalBondAddress.String()), standbyNodeAccount, amt)
	// Remove the bond from the node account
	lp.NodeBondAddress = nil
	lp.Units = cosmos.NewUint(1000)
	lp.Bond(activeNA, cosmos.NewUint(100))
	k.SetLiquidityProvider(ctx, lp)
	handler.mgr.Keeper().SetMimir(ctx, "MaximumBondInRune", 0)
	handler.mgr.Keeper().SetMimir(ctx, "MaximumLPBondedNodes", 1)
	msg = NewMsgBond(txIn, standbyNA, cosmos.NewUint(common.One), common.Address(additionalBondAddress.String()), nil, activeNA, -1, common.BTCAsset, cosmos.NewUint(100))
	err = handler.validate(ctx, *msg)
	errCheck(c, err, "lp has reached maximum bonded nodes")
}

func (HandlerBondSuite) TestBondProvider_OperatorFee(c *C) {
	ctx, k := setupKeeperForTest(c)
	handler := NewBondHandler(NewDummyMgrWithKeeper(k))

	standbyNodeAccount := GetRandomValidatorNode(NodeStandby)
	operatorBondAddress := GetRandomBaseAddress()
	operatorAccAddress, _ := operatorBondAddress.AccAddress()
	providerBondAddress := GetRandomBaseAddress()
	providerAccAddr, _ := providerBondAddress.AccAddress()
	standbyNodeAccount.BondAddress = operatorBondAddress

	c.Assert(k.SetNodeAccount(ctx, standbyNodeAccount), IsNil)

	standbyNodeAddr := standbyNodeAccount.NodeAddress
	amt := cosmos.NewUint(100 * common.One)
	txIn := GetRandomTx()
	txIn.Coins = common.NewCoins(common.NewCoin(common.BaseAsset(), amt))

	/* Test Validation and Handling */

	// happy path should be able to set node operator fee
	msg := NewMsgBond(txIn, standbyNodeAddr, cosmos.NewUint(common.One), standbyNodeAccount.BondAddress, providerAccAddr, operatorAccAddress, 5000, common.EmptyAsset, cosmos.ZeroUint())
	err := handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	bp, _ := k.GetBondProviders(ctx, standbyNodeAccount.NodeAddress)
	c.Assert(bp.NodeOperatorFee.Uint64(), Equals, uint64(5000))

	// Check that a bond provider for the operator + new provider was added
	c.Assert(len(bp.Providers), Equals, 2)

	// try to increase operator fee after provider has bonded, should success , because bond providers should trust each other
	bp.Providers[1].Bonded = true
	bp.NodeOperatorFee = cosmos.NewUint(5000)
	c.Assert(k.SetBondProviders(ctx, bp), IsNil)
	msg = NewMsgBond(txIn, standbyNodeAddr, cosmos.NewUint(common.One), operatorBondAddress, providerAccAddr, operatorAccAddress, 6000, common.EmptyAsset, cosmos.ZeroUint())
	err = handler.validate(ctx, *msg)
	c.Assert(err, IsNil)
	err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	bp, _ = k.GetBondProviders(ctx, standbyNodeAccount.NodeAddress)
	c.Assert(bp.NodeOperatorFee.Uint64(), Equals, uint64(6000))

	// Should be able to decrease operator fee after provider has bonded
	msg = NewMsgBond(txIn, standbyNodeAddr, cosmos.NewUint(common.One), operatorBondAddress, providerAccAddr, operatorAccAddress, 4000, common.EmptyAsset, cosmos.ZeroUint())
	err = handler.validate(ctx, *msg)
	c.Assert(err, IsNil)
	err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	bp, _ = k.GetBondProviders(ctx, standbyNodeAccount.NodeAddress)
	c.Assert(bp.NodeOperatorFee.Uint64(), Equals, uint64(4000))

	// Only operator can set operator fee
	msg = NewMsgBond(txIn, standbyNodeAddr, amt, providerBondAddress, providerAccAddr, providerAccAddr, 0, common.EmptyAsset, cosmos.ZeroUint())
	err = handler.validate(ctx, *msg)
	c.Assert(err.Error(), Equals, "only node operator can set fee: unknown request")

	msg = NewMsgBond(txIn, standbyNodeAddr, amt, providerBondAddress, providerAccAddr, providerAccAddr, 4000, common.EmptyAsset, cosmos.ZeroUint())
	err = handler.validate(ctx, *msg)
	c.Assert(err.Error(), Equals, "only node operator can set fee: unknown request")

	// If nodeAcc.BondAddress is empty, any address should be able to set operator fee (and become bonder address)
	standbyNodeAccount.BondAddress = common.NoAddress
	c.Assert(k.SetNodeAccount(ctx, standbyNodeAccount), IsNil)
	bp.Providers = []BondProvider{}
	c.Assert(k.SetBondProviders(ctx, bp), IsNil)
	msg = NewMsgBond(txIn, standbyNodeAddr, cosmos.NewUint(common.One), providerBondAddress, providerAccAddr, providerAccAddr, 4000, common.EmptyAsset, cosmos.ZeroUint())
	err = handler.validate(ctx, *msg)
	c.Assert(err, IsNil)
}

func (HandlerBondSuite) TestBondProvider_Handler(c *C) {
	ctx, k := setupKeeperForTest(c)
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	c.Assert(k.SetNodeAccount(ctx, activeNodeAccount), IsNil)
	standbyNodeAccount := GetRandomValidatorNode(NodeStandby)
	c.Assert(k.SetNodeAccount(ctx, standbyNodeAccount), IsNil)
	handler := NewBondHandler(NewDummyMgrWithKeeper(k))
	txIn := GetRandomTx()
	amt := cosmos.NewUint(100 * common.One)
	txIn.Coins = common.NewCoins(common.NewCoin(common.BaseAsset(), amt))
	activeNA := activeNodeAccount.NodeAddress
	standbyNA := standbyNodeAccount.NodeAddress
	standbyNAAddress := common.Address(standbyNA.String())
	additionalBondAddress := GetRandomBech32Addr()
	FundAccount(c, ctx, k, standbyNA, amt.Uint64())
	FundAccount(c, ctx, k, activeNA, amt.Uint64())

	// TEST HANDLER //

	// happy path, and add a whitelisted address (Invite)
	standbyNALP, _ := SetupLiquidityBondForTest(c, ctx, handler.mgr.Keeper(), common.BTCAsset, standbyNAAddress, standbyNodeAccount, amt)
	// Remove the bond from the node account
	standbyNALP.NodeBondAddress = nil
	k.SetLiquidityProvider(ctx, standbyNALP)
	msg := NewMsgBond(txIn, standbyNA, cosmos.NewUint(common.One), standbyNAAddress, additionalBondAddress, standbyNA, 0, common.EmptyAsset, cosmos.ZeroUint())
	err := handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	// It should add both the node bond and the additional bond addresses to the bond providers
	bp, err := k.GetBondProviders(ctx, standbyNA)
	c.Assert(err, IsNil)
	c.Assert(bp.Providers, HasLen, 2)
	c.Assert(bp.Has(standbyNA), Equals, true)
	c.Assert(bp.Has(additionalBondAddress), Equals, true)
	// New BP should have no bond
	bpBond, err := handler.mgr.Keeper().CalcLPLiquidityBond(ctx, common.Address(additionalBondAddress.String()), standbyNA)
	c.Assert(err, IsNil)
	c.Assert(bpBond.Uint64(), Equals, uint64(0), Commentf("%d", bpBond.Uint64()))
	c.Assert(bp.Get(additionalBondAddress).Bonded, Equals, false)
	// This is only an invite transaction, so the node account should not be bonded automatically
	standbyNABond, err := handler.mgr.Keeper().CalcLPLiquidityBond(ctx, standbyNALP.CacaoAddress, standbyNA)
	c.Assert(err, IsNil)
	c.Assert(bp.Get(standbyNA).Bonded, Equals, false)
	c.Assert(standbyNABond.Uint64(), Equals, uint64(0), Commentf("%d", standbyNABond.Uint64()))

	// bond with additional bonder and test rewards
	additionalLP, _ := SetupLiquidityBondForTest(c, ctx, handler.mgr.Keeper(), common.BTCAsset, common.Address(additionalBondAddress.String()), standbyNodeAccount, amt)
	bp, err = k.GetBondProviders(ctx, standbyNA)
	c.Assert(err, IsNil)
	reward := cosmos.NewUint(10 * common.One)
	bp.Providers[1].Reward = &reward
	FundModule(c, ctx, k, BondName, 10)
	c.Assert(k.SetBondProviders(ctx, bp), IsNil)
	// Remove the bond from the node account
	additionalLP.NodeBondAddress = nil
	k.SetLiquidityProvider(ctx, additionalLP)
	additionalAcc, err := additionalLP.CacaoAddress.AccAddress()
	c.Assert(err, IsNil)
	msg = NewMsgBond(txIn, standbyNA, cosmos.NewUint(common.One), additionalLP.CacaoAddress, nil, additionalAcc, -1, common.BTCAsset, amt)
	err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	bp, err = k.GetBondProviders(ctx, standbyNA)
	c.Assert(err, IsNil)
	c.Assert(bp.Providers, HasLen, 2)
	c.Assert(bp.Providers[1].Reward, NotNil)
	c.Assert(bp.Providers[1].Reward.Uint64(), Equals, uint64(0), Commentf("expected %d, got %d", 0, bp.Providers[1].Reward.Uint64()))
	balance := k.GetBalance(ctx, bp.Providers[1].BondAddress)
	c.Assert(balance[0].Amount.Uint64(), Equals, uint64(10*common.One), Commentf("expected %d, got %d", 10*common.One, balance[0].Amount.Uint64()))
	c.Assert(bp.Has(additionalBondAddress), Equals, true)
	lpBond, err := k.CalcLPLiquidityBond(ctx, common.Address(additionalBondAddress.String()), standbyNA)
	c.Assert(err, IsNil)
	c.Assert(bp.Get(additionalBondAddress).Bonded, Equals, true)
	c.Assert(lpBond.Uint64(), Equals, amt.MulUint64(2).Uint64(), Commentf("%d", lpBond.Uint64()))

	// Random bonder
	msg = NewMsgBond(txIn, standbyNA, cosmos.NewUint(common.One), GetRandomBaseAddress(), nil, standbyNA, -1, common.BTCAsset, cosmos.NewUint(1000))
	err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	bp, _ = k.GetBondProviders(ctx, standbyNA)
	c.Assert(bp.Providers, HasLen, 2)
	naBond, err := handler.mgr.Keeper().CalcNodeLiquidityBond(ctx, standbyNodeAccount)
	c.Assert(err, IsNil)
	c.Assert(naBond.Uint64(), Equals, amt.MulUint64(2).Uint64(), Commentf("expected %d, got %d", amt.MulUint64(2).Uint64(), naBond.Uint64()))

	// Set the node operator fee to 5%
	bp, _ = k.GetBondProviders(ctx, standbyNA)
	bp.NodeOperatorFee = cosmos.NewUint(500)
	c.Assert(k.SetBondProviders(ctx, bp), IsNil)

	// Check BP bond - Node Operator should have 0, BP #2 should have 100
	bp, _ = k.GetBondProviders(ctx, standbyNA)
	naBond, err = k.CalcLPLiquidityBond(ctx, standbyNALP.CacaoAddress, standbyNA)
	c.Assert(err, IsNil)
	c.Assert(naBond.Uint64(), Equals, uint64(0), Commentf("%d", naBond.Uint64()))
	lpBond, err = k.CalcLPLiquidityBond(ctx, common.Address(additionalBondAddress.String()), standbyNA)
	c.Assert(err, IsNil)
	c.Assert(lpBond.Uint64(), Equals, amt.MulUint64(2).Uint64(), Commentf("%d", lpBond.Uint64()))
}
