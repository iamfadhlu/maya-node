package mayachain

import (
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
)

type HandlerManageMAYANameSuite struct{}

var _ = Suite(&HandlerManageMAYANameSuite{})

type KeeperManageMAYANameTest struct {
	keeper.Keeper
}

func NewKeeperManageMAYANameTest(k keeper.Keeper) KeeperManageMAYANameTest {
	return KeeperManageMAYANameTest{Keeper: k}
}

func (s *HandlerManageMAYANameSuite) TestValidator(c *C) {
	ctx, mgr := setupManagerForTest(c)

	handler := NewManageMAYANameHandler(mgr)
	coin := common.NewCoin(common.BaseAsset(), cosmos.NewUint(1001*common.One))
	addr := GetRandomBaseAddress()
	acc, _ := addr.AccAddress()
	name := NewMAYAName("hello", 50, []MAYANameAlias{{Chain: common.BASEChain, Address: addr}}, common.EmptyAsset, nil, cosmos.ZeroUint(), nil)
	mgr.Keeper().SetMAYAName(ctx, name)

	// happy path
	msg := NewMsgManageMAYAName("I-am_the_99th_walrus+", common.BASEChain, addr, coin, 0, common.EmptyAsset, cosmos.ZeroUint(), []cosmos.Uint{}, []string{}, acc, acc)
	c.Assert(handler.validate(ctx, *msg), IsNil)

	// fail: BNB.BNB pool doesn't exist
	msg = NewMsgManageMAYAName("I-am_the_99th_walrus+", common.BASEChain, addr, coin, 0, common.BNBAsset, cosmos.ZeroUint(), []cosmos.Uint{}, []string{}, acc, acc)
	c.Assert(handler.validate(ctx, *msg), NotNil)

	// fail: name is too long
	msg.Name = "this_name_is_way_too_long_to_be_a_valid_name"
	c.Assert(handler.validate(ctx, *msg), NotNil)

	// fail: bad characters
	msg.Name = "i am the walrus"
	c.Assert(handler.validate(ctx, *msg), NotNil)

	// fail: bad attempt to inflate expire block height
	msg.Name = "hello"
	msg.ExpireBlockHeight = 100
	c.Assert(handler.validate(ctx, *msg), NotNil)

	// fail: bad auth
	msg.ExpireBlockHeight = 0
	msg.Signer = GetRandomBech32Addr()
	c.Assert(handler.validate(ctx, *msg), NotNil)

	// fail: not enough funds for new MAYAName
	msg.Name = "bang"
	msg.Coin.Amount = cosmos.ZeroUint()
	c.Assert(handler.validate(ctx, *msg), NotNil)
}

func (s *HandlerManageMAYANameSuite) TestHandler(c *C) {
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)
	feePerBlock := constAccessor.GetInt64Value(constants.TNSFeePerBlock)
	registrationFee := constAccessor.GetInt64Value(constants.TNSRegisterFee)
	ctx, mgr := setupManagerForTest(c)

	blocksPerYear := mgr.GetConstants().GetInt64Value(constants.BlocksPerYear)
	handler := NewManageMAYANameHandler(mgr)
	coin := common.NewCoin(common.BaseAsset(), cosmos.NewUint(10000*common.One))
	addr := GetRandomBaseAddress()
	acc, _ := addr.AccAddress()
	mnName := "hello"

	// add rune to addr for gas
	FundAccount(c, ctx, mgr.Keeper(), acc, 10*common.One)

	preferredAsset := common.BNBAsset

	// happy path, register new name
	msg := NewMsgManageMAYAName(mnName, common.BASEChain, addr, coin, 0, preferredAsset, cosmos.ZeroUint(), []cosmos.Uint{}, []string{}, acc, acc)
	_, err := handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	name, err := mgr.Keeper().GetMAYAName(ctx, mnName)
	c.Assert(err, IsNil)
	c.Check(name.Owner.Empty(), Equals, false)
	c.Check(name.ExpireBlockHeight, Equals, ctx.BlockHeight()+blocksPerYear+(int64(coin.Amount.Uint64())-registrationFee)/feePerBlock)

	// happy path, set alt chain address
	bnbAddr := GetRandomBNBAddress()
	msg = NewMsgManageMAYAName(mnName, common.BNBChain, bnbAddr, coin, 0, preferredAsset, cosmos.ZeroUint(), []cosmos.Uint{}, []string{}, acc, acc)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	name, err = mgr.Keeper().GetMAYAName(ctx, mnName)
	c.Assert(err, IsNil)
	c.Check(name.GetAlias(common.BNBChain).Equals(bnbAddr), Equals, true)

	msg = NewMsgManageMAYAName(mnName, common.BNBChain, bnbAddr, coin, 0, preferredAsset, cosmos.ZeroUint(), []cosmos.Uint{}, []string{}, acc, acc)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	name, err = mgr.Keeper().GetMAYAName(ctx, mnName)
	c.Assert(err, IsNil)
	c.Check(name.GetAlias(common.BNBChain).Equals(bnbAddr), Equals, true)

	// update preferred asset
	msg = NewMsgManageMAYAName(mnName, common.BNBChain, bnbAddr, coin, 0, common.RUNEAsset, cosmos.ZeroUint(), []cosmos.Uint{}, []string{}, acc, acc)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	name, err = mgr.Keeper().GetMAYAName(ctx, mnName)
	c.Assert(err, IsNil)
	c.Check(name.GetPreferredAsset(), Equals, common.RUNEAsset)

	// remove preferred asset
	msg = NewMsgManageMAYAName(mnName, common.BNBChain, bnbAddr, coin, 0, common.BaseNative, cosmos.ZeroUint(), []cosmos.Uint{}, []string{}, acc, acc)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	name, err = mgr.Keeper().GetMAYAName(ctx, mnName)
	c.Assert(err, IsNil)
	c.Check(name.GetPreferredAsset(), Equals, common.EmptyAsset)

	// transfer mayaname to new owner, should reset preferred asset/external aliases
	addr2 := GetRandomBaseAddress()
	acc2, _ := addr2.AccAddress()
	msg = NewMsgManageMAYAName(mnName, common.THORChain, addr, coin, 0, preferredAsset, cosmos.ZeroUint(), []cosmos.Uint{}, []string{}, acc2, acc)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	name, err = mgr.Keeper().GetMAYAName(ctx, mnName)
	c.Assert(err, IsNil)
	c.Check(len(name.GetAliases()), Equals, 0)
	c.Check(name.GetPreferredAsset().IsEmpty(), Equals, true)
	c.Check(name.GetOwner().Equals(acc2), Equals, true)

	// happy path, release mayaname back into the wild
	msg = NewMsgManageMAYAName(mnName, common.BASEChain, addr, common.NewCoin(common.BaseAsset(), cosmos.ZeroUint()), 1, preferredAsset, EmptyBps, []cosmos.Uint{}, []string{}, acc, acc)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	name, err = mgr.Keeper().GetMAYAName(ctx, mnName)
	c.Assert(err, IsNil)
	c.Check(name.Owner.Empty(), Equals, true)
	c.Check(name.ExpireBlockHeight, Equals, int64(0))

	// *** test subaffiliates - original syntax ***
	// register the main mayaname
	msg = NewMsgManageMAYAName(mnName, common.BASEChain, addr, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{}, []string{}, acc, acc)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	// register aff1 with BNB and THOR alias
	msg = NewMsgManageMAYAName("aff1", common.BASEChain, addr, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{}, []string{}, acc2, acc2)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	msg = NewMsgManageMAYAName("aff1", common.BNBChain, bnbAddr, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{}, []string{}, acc2, acc2)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	thorAddr := GetRandomTHORAddress()
	msg = NewMsgManageMAYAName("aff2", common.THORChain, thorAddr, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{}, []string{}, acc2, acc2)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	// set 1% default affiliate fee bps
	msg = NewMsgManageMAYAName(mnName, common.BASEChain, addr, coin, 0, preferredAsset, cosmos.NewUint(100), []cosmos.Uint{}, []string{}, acc, acc)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	name, err = mgr.Keeper().GetMAYAName(ctx, mnName)
	c.Assert(err, IsNil)
	c.Check(name.GetAffiliateBps().Equal(cosmos.NewUint(100)), Equals, true)
	// register aff1 as a subaffiliate with 10% fee cut
	msg = NewMsgManageMAYAName(mnName, common.BASEChain, addr, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(1000)}, []string{"aff1"}, acc, acc)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	name, err = mgr.Keeper().GetMAYAName(ctx, mnName)
	c.Assert(err, IsNil)
	c.Check(name.Subaffiliates, HasLen, 1)
	c.Check(name.Subaffiliates[0].Name, Equals, "aff1")
	c.Check(name.Subaffiliates[0].Bps.Equal(cosmos.NewUint(1000)), Equals, true)
	// register aff2 as a second subaffiliate with 20% fee cut
	msg = NewMsgManageMAYAName(mnName, common.BASEChain, addr, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(2000)}, []string{"aff2"}, acc, acc)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	name, err = mgr.Keeper().GetMAYAName(ctx, mnName)
	c.Assert(err, IsNil)
	c.Check(name.Subaffiliates, HasLen, 2)
	c.Check(name.Subaffiliates[0].Name, Equals, "aff1")
	c.Check(name.Subaffiliates[0].Bps.Equal(cosmos.NewUint(1000)), Equals, true)
	c.Check(name.Subaffiliates[1].Name, Equals, "aff2")
	c.Check(name.Subaffiliates[1].Bps.Equal(cosmos.NewUint(2000)), Equals, true)
	// change aff1 fee cut to 30%
	msg = NewMsgManageMAYAName(mnName, common.BASEChain, addr, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(3000)}, []string{"aff1"}, acc, acc)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	name, err = mgr.Keeper().GetMAYAName(ctx, mnName)
	c.Assert(err, IsNil)
	c.Check(name.Subaffiliates, HasLen, 2)
	c.Check(name.Subaffiliates[0].Name, Equals, "aff1")
	c.Check(name.Subaffiliates[0].Bps.Equal(cosmos.NewUint(3000)), Equals, true)
	c.Check(name.Subaffiliates[1].Name, Equals, "aff2")
	c.Check(name.Subaffiliates[1].Bps.Equal(cosmos.NewUint(2000)), Equals, true)
	// remove aff1 as subaffiliate
	msg = NewMsgManageMAYAName(mnName, common.BASEChain, addr, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{cosmos.ZeroUint()}, []string{"aff1"}, acc, acc)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	name, err = mgr.Keeper().GetMAYAName(ctx, mnName)
	c.Assert(err, IsNil)
	c.Check(name.Subaffiliates, HasLen, 1)
	c.Check(name.Subaffiliates[0].Name, Equals, "aff2")
	c.Check(name.Subaffiliates[0].Bps.Equal(cosmos.NewUint(2000)), Equals, true)
	// remove aff2 as subaffiliate as well
	msg = NewMsgManageMAYAName(mnName, common.BASEChain, addr, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{cosmos.ZeroUint()}, []string{"aff2"}, acc, acc)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	name, err = mgr.Keeper().GetMAYAName(ctx, mnName)
	c.Assert(err, IsNil)
	c.Check(name.Subaffiliates, HasLen, 0)

	// register the main mayaname
	msg = NewMsgManageMAYAName(mnName, common.BASEChain, addr, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{}, []string{}, acc, acc)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	// register aff1 with BNB and THOR alias
	msg = NewMsgManageMAYAName("aff1", common.BASEChain, addr, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{}, []string{}, acc2, acc2)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	msg = NewMsgManageMAYAName("aff1", common.BNBChain, bnbAddr, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{}, []string{}, acc2, acc2)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	msg = NewMsgManageMAYAName("aff2", common.THORChain, thorAddr, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{}, []string{}, acc2, acc2)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	// set 1% default affiliate fee bps
	msg = NewMsgManageMAYAName(mnName, common.EmptyChain, common.NoAddress, coin, 0, preferredAsset, cosmos.NewUint(100), []cosmos.Uint{}, []string{}, acc, acc)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	name, err = mgr.Keeper().GetMAYAName(ctx, mnName)
	c.Assert(err, IsNil)
	c.Check(name.GetAffiliateBps().Equal(cosmos.NewUint(100)), Equals, true)
	// register aff1 as a subaffiliate with 10% fee cut
	msg = NewMsgManageMAYAName(mnName, common.EmptyChain, common.NoAddress, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(1000)}, []string{"aff1"}, acc, acc)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	name, err = mgr.Keeper().GetMAYAName(ctx, mnName)
	c.Assert(err, IsNil)
	c.Check(name.Subaffiliates, HasLen, 1)
	c.Check(name.Subaffiliates[0].Name, Equals, "aff1")
	c.Check(name.Subaffiliates[0].Bps.Equal(cosmos.NewUint(1000)), Equals, true)
	// register aff2 as a second subaffiliate with 20% fee cut
	msg = NewMsgManageMAYAName(mnName, common.EmptyChain, common.NoAddress, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(2000)}, []string{"aff2"}, acc, acc)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	name, err = mgr.Keeper().GetMAYAName(ctx, mnName)
	c.Assert(err, IsNil)
	c.Check(name.Subaffiliates, HasLen, 2)
	c.Check(name.Subaffiliates[0].Name, Equals, "aff1")
	c.Check(name.Subaffiliates[0].Bps.Equal(cosmos.NewUint(1000)), Equals, true)
	c.Check(name.Subaffiliates[1].Name, Equals, "aff2")
	c.Check(name.Subaffiliates[1].Bps.Equal(cosmos.NewUint(2000)), Equals, true)
	// change aff1 fee cut to 30%
	msg = NewMsgManageMAYAName(mnName, common.EmptyChain, common.NoAddress, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(3000)}, []string{"aff1"}, acc, acc)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	name, err = mgr.Keeper().GetMAYAName(ctx, mnName)
	c.Assert(err, IsNil)
	c.Check(name.Subaffiliates, HasLen, 2)
	c.Check(name.Subaffiliates[0].Name, Equals, "aff1")
	c.Check(name.Subaffiliates[0].Bps.Equal(cosmos.NewUint(3000)), Equals, true)
	c.Check(name.Subaffiliates[1].Name, Equals, "aff2")
	c.Check(name.Subaffiliates[1].Bps.Equal(cosmos.NewUint(2000)), Equals, true)
	// remove aff1 as subaffiliate
	msg = NewMsgManageMAYAName(mnName, common.EmptyChain, common.NoAddress, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{cosmos.ZeroUint()}, []string{"aff1"}, acc, acc)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	name, err = mgr.Keeper().GetMAYAName(ctx, mnName)
	c.Assert(err, IsNil)
	c.Check(name.Subaffiliates, HasLen, 1)
	c.Check(name.Subaffiliates[0].Name, Equals, "aff2")
	c.Check(name.Subaffiliates[0].Bps.Equal(cosmos.NewUint(2000)), Equals, true)
	// remove aff2 as subaffiliate as well
	msg = NewMsgManageMAYAName(mnName, common.EmptyChain, common.NoAddress, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{cosmos.ZeroUint()}, []string{"aff2"}, acc, acc)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	name, err = mgr.Keeper().GetMAYAName(ctx, mnName)
	c.Assert(err, IsNil)
	c.Check(name.Subaffiliates, HasLen, 0)

	// test multiple sub-affiliates on single tx
	// register the main mayaname
	msg = NewMsgManageMAYAName(mnName, common.BASEChain, addr, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{}, []string{}, acc, acc)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	// register aff1 with BNB and THOR alias
	msg = NewMsgManageMAYAName("aff1", common.BASEChain, addr, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{}, []string{}, acc2, acc2)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	msg = NewMsgManageMAYAName("aff1", common.BNBChain, bnbAddr, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{}, []string{}, acc2, acc2)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	thorAddr = GetRandomTHORAddress()
	msg = NewMsgManageMAYAName("aff2", common.THORChain, thorAddr, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{}, []string{}, acc2, acc2)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	// set 1% default affiliate fee bps
	msg = NewMsgManageMAYAName(mnName, common.BASEChain, addr, coin, 0, preferredAsset, cosmos.NewUint(100), []cosmos.Uint{}, []string{}, acc, acc)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	name, err = mgr.Keeper().GetMAYAName(ctx, mnName)
	c.Assert(err, IsNil)
	c.Check(name.GetAffiliateBps().Equal(cosmos.NewUint(100)), Equals, true)
	// register aff1 as a subaffiliate with 10% fee cut and aff2 as a second subaffiliate with 20% fee cut
	msg = NewMsgManageMAYAName(mnName, common.BASEChain, addr, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(1000), cosmos.NewUint(2000)}, []string{"aff1", "aff2"}, acc, acc)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	name, err = mgr.Keeper().GetMAYAName(ctx, mnName)
	c.Assert(err, IsNil)
	c.Check(name.Subaffiliates, HasLen, 2)
	c.Check(name.Subaffiliates[0].Name, Equals, "aff1")
	c.Check(name.Subaffiliates[0].Bps.Equal(cosmos.NewUint(1000)), Equals, true)
	c.Check(name.Subaffiliates[1].Name, Equals, "aff2")
	c.Check(name.Subaffiliates[1].Bps.Equal(cosmos.NewUint(2000)), Equals, true)
	// change aff1 fee cut to 30%
	msg = NewMsgManageMAYAName(mnName, common.BASEChain, addr, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(3000)}, []string{"aff1"}, acc, acc)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	name, err = mgr.Keeper().GetMAYAName(ctx, mnName)
	c.Assert(err, IsNil)
	c.Check(name.Subaffiliates, HasLen, 2)
	c.Check(name.Subaffiliates[0].Name, Equals, "aff1")
	c.Check(name.Subaffiliates[0].Bps.Equal(cosmos.NewUint(3000)), Equals, true)
	c.Check(name.Subaffiliates[1].Name, Equals, "aff2")
	c.Check(name.Subaffiliates[1].Bps.Equal(cosmos.NewUint(2000)), Equals, true)
	// remove aff1 and aff2 as subaffiliate
	msg = NewMsgManageMAYAName(mnName, common.BASEChain, addr, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{cosmos.ZeroUint(), cosmos.ZeroUint()}, []string{"aff1", "aff2"}, acc, acc)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	name, err = mgr.Keeper().GetMAYAName(ctx, mnName)
	c.Assert(err, IsNil)
	c.Check(name.Subaffiliates, HasLen, 0)

	// **** validator checks
	// set self as subaffiliate - should fail
	msg = NewMsgManageMAYAName(mnName, common.BASEChain, addr, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(1000)}, []string{mnName}, acc, acc)
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
	// set non-existent mayaname as subaffiliate - should fail
	msg = NewMsgManageMAYAName(mnName, common.BASEChain, addr, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(1000)}, []string{"does_not_exist"}, acc, acc)
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
	// set out of range affiliate bps - should fail
	msg = NewMsgManageMAYAName(mnName, common.BASEChain, addr, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(10_001)}, []string{"aff1"}, acc, acc)
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
	// set out of range affiliate bps - should fail
	msg = NewMsgManageMAYAName(mnName, common.BASEChain, addr, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{}, []string{"aff1"}, acc, acc)
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
	// handler checks
	// set self as subaffiliate - should fail
	msg = NewMsgManageMAYAName(mnName, common.BASEChain, addr, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(1000)}, []string{mnName}, acc, acc)
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
	// set non-existent mayaname as subaffiliate - should fail
	msg = NewMsgManageMAYAName(mnName, common.BASEChain, addr, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(1000)}, []string{"does_not_exist"}, acc, acc)
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
	// set out of range affiliate bps - should fail
	msg = NewMsgManageMAYAName(mnName, common.BASEChain, addr, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(10_001)}, []string{"aff1"}, acc, acc)
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
	// EmptyBps aff bps and chain not provided - should fail
	msg = NewMsgManageMAYAName(mnName, common.EmptyChain, common.NoAddress, coin, 0, preferredAsset, EmptyBps, []cosmos.Uint{}, []string{"aff1"}, acc, acc)
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
	// register the main mayaname without providing an alias - should fail
	msg = NewMsgManageMAYAName("newone", common.EmptyChain, common.NoAddress, coin, 0, preferredAsset, cosmos.ZeroUint(), []cosmos.Uint{}, []string{}, acc, acc)
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
	// preferred asset doesn't have alias
	msg = NewMsgManageMAYAName(mnName, common.BASEChain, addr, coin, 0, preferredAsset, cosmos.ZeroUint(), []cosmos.Uint{}, []string{}, acc, acc)
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
	// cacao can be always set as preferred asset
	msg = NewMsgManageMAYAName(mnName, common.BASEChain, addr, coin, 0, common.BaseNative, cosmos.ZeroUint(), []cosmos.Uint{}, []string{}, acc, acc)
	err = handler.validate(ctx, *msg)
	c.Assert(err, IsNil)
	// either both the chain and alias are provided together or neither
	msg = NewMsgManageMAYAName(mnName, common.BASEChain, common.NoAddress, coin, 0, preferredAsset, cosmos.NewUint(100), []cosmos.Uint{}, []string{}, acc, acc)
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
	msg = NewMsgManageMAYAName(mnName, common.EmptyChain, addr, coin, 0, preferredAsset, cosmos.NewUint(100), []cosmos.Uint{}, []string{}, acc, acc)
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)

	// ***********************************************
	// check circular references in mayaname hierarchy
	msg = NewMsgManageMAYAName("foo", common.BASEChain, addr, coin, 0, common.EmptyAsset, EmptyBps, []cosmos.Uint{}, []string{}, acc, acc)
	_, err = handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	msg = NewMsgManageMAYAName("b", common.BASEChain, addr, coin, 0, common.EmptyAsset, EmptyBps, []cosmos.Uint{}, []string{}, acc, acc)
	_, err = handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	// create a -> b (a has b as sub-affiliate)
	msg = NewMsgManageMAYAName("a", common.BASEChain, addr, coin, 0, common.EmptyAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(1000), cosmos.NewUint(1000)}, []string{"b", "foo"}, acc, acc)
	_, err = handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	// create c -> b
	msg = NewMsgManageMAYAName("c", common.BASEChain, addr, coin, 0, common.EmptyAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(1000)}, []string{"b"}, acc, acc)
	_, err = handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	// create d -> a
	msg = NewMsgManageMAYAName("d", common.BASEChain, addr, coin, 0, common.EmptyAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(1000)}, []string{"a"}, acc, acc)
	_, err = handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	// try to add 'a' as a sub-affiliate to 'b', where 'b' already has 'a' as its parent (which would create a cycle: a -> b -> a).
	msg = NewMsgManageMAYAName("b", common.BASEChain, addr, coin, 0, common.EmptyAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(1000), cosmos.NewUint(1000)}, []string{"a", "foo"}, acc, acc)
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
	// try to add 'c' as a sub-affiliate to 'b', where 'c' already has 'b' as its sub-affiliate (which would create a cycle: a -> b -> c -> b).
	msg = NewMsgManageMAYAName("b", common.BASEChain, addr, coin, 0, common.EmptyAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(1000), cosmos.NewUint(1000)}, []string{"c", "foo"}, acc, acc)
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
	// try to add 'd' as a sub-affiliate to 'b', where 'd' already has 'a' as its sub-affiliate (which would create a cycle: a -> b -> d -> a).
	msg = NewMsgManageMAYAName("b", common.BASEChain, addr, coin, 0, common.EmptyAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(1000), cosmos.NewUint(1000)}, []string{"d", "foo"}, acc, acc)
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)

	// check circular references in mayaname hierarchy
	// create a -> b (a has b as sub-affiliate)
	msg = NewMsgManageMAYAName("bb", common.BASEChain, addr, coin, 0, common.EmptyAsset, EmptyBps, []cosmos.Uint{}, []string{}, acc, acc)
	_, err = handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	msg = NewMsgManageMAYAName("z1", common.BASEChain, addr, coin, 0, common.EmptyAsset, EmptyBps, []cosmos.Uint{}, []string{}, acc, acc)
	_, err = handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	msg = NewMsgManageMAYAName("y1", common.BASEChain, addr, coin, 0, common.EmptyAsset, EmptyBps, []cosmos.Uint{}, []string{}, acc, acc)
	_, err = handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	msg = NewMsgManageMAYAName("aa", common.BASEChain, addr, coin, 0, common.EmptyAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(1000)}, []string{"bb"}, acc, acc)
	_, err = handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	// create cc -> x1 -> bb
	//                 -> z1
	//           -> y1
	msg = NewMsgManageMAYAName("x1", common.BASEChain, addr, coin, 0, common.EmptyAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(1000)}, []string{"bb"}, acc, acc)
	_, err = handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	msg = NewMsgManageMAYAName("x1", common.BASEChain, addr, coin, 0, common.EmptyAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(1000)}, []string{"z1"}, acc, acc)
	_, err = handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	msg = NewMsgManageMAYAName("cc", common.BASEChain, addr, coin, 0, common.EmptyAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(1000)}, []string{"x1"}, acc, acc)
	_, err = handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	msg = NewMsgManageMAYAName("cc", common.BASEChain, addr, coin, 0, common.EmptyAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(1000)}, []string{"y1"}, acc, acc)
	_, err = handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	// create dd -> x2 -> aa
	//                 -> z2
	//           -> y2
	msg = NewMsgManageMAYAName("aa", common.BASEChain, addr, coin, 0, common.EmptyAsset, EmptyBps, []cosmos.Uint{}, []string{}, acc, acc)
	_, err = handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	msg = NewMsgManageMAYAName("z2", common.BASEChain, addr, coin, 0, common.EmptyAsset, EmptyBps, []cosmos.Uint{}, []string{}, acc, acc)
	_, err = handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	msg = NewMsgManageMAYAName("y2", common.BASEChain, addr, coin, 0, common.EmptyAsset, EmptyBps, []cosmos.Uint{}, []string{}, acc, acc)
	_, err = handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	msg = NewMsgManageMAYAName("x2", common.BASEChain, addr, coin, 0, common.EmptyAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(1000)}, []string{"aa"}, acc, acc)
	_, err = handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	msg = NewMsgManageMAYAName("x2", common.BASEChain, addr, coin, 0, common.EmptyAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(1000)}, []string{"z2"}, acc, acc)
	_, err = handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	msg = NewMsgManageMAYAName("dd", common.BASEChain, addr, coin, 0, common.EmptyAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(1000)}, []string{"x2"}, acc, acc)
	_, err = handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	msg = NewMsgManageMAYAName("dd", common.BASEChain, addr, coin, 0, common.EmptyAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(1000)}, []string{"y2"}, acc, acc)
	_, err = handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	// try to add 'cc' as a sub-affiliate to 'bb', where 'cc' already has 'bb' as its sub-sub-affiliate (which would create a cycle: aa -> bb -> cc -> x1 -> bb).
	msg = NewMsgManageMAYAName("bb", common.BASEChain, addr, coin, 0, common.EmptyAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(1000), cosmos.NewUint(1000)}, []string{"foo", "cc"}, acc, acc)
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
	// try to add 'dd' as a sub-affiliate to 'bb', where 'dd' already has 'aa' (a parent of 'bb') as its sub-sub-affiliate (which would create a cycle: aa -> bb -> dd -> x2 -> aa).
	msg = NewMsgManageMAYAName("bb", common.BASEChain, addr, coin, 0, common.EmptyAsset, EmptyBps, []cosmos.Uint{cosmos.NewUint(1000), cosmos.NewUint(1000)}, []string{"foo", "dd"}, acc, acc)
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
}
