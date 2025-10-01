package mayachain

import (
	"errors"
	"fmt"
	"strconv"

	se "github.com/cosmos/cosmos-sdk/types/errors"
	tmtypes "github.com/tendermint/tendermint/types"
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

type HandlerDepositSuite struct{}

var _ = Suite(&HandlerDepositSuite{})

func (s *HandlerDepositSuite) TestValidate(c *C) {
	ctx, k := setupKeeperForTest(c)

	addr := GetRandomBech32Addr()

	coins := common.Coins{
		common.NewCoin(common.BaseNative, cosmos.NewUint(200*common.One)),
	}
	msg := NewMsgDeposit(coins, fmt.Sprintf("ADD:BNB.BNB:%s", GetRandomBaseAddress()), addr)

	handler := NewDepositHandler(NewDummyMgrWithKeeper(k))
	err := handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	// invalid msg
	msg = &MsgDeposit{}
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
}

func (s *HandlerDepositSuite) TestHandle(c *C) {
	ctx, k := setupKeeperForTest(c)
	activeNode := GetRandomValidatorNode(NodeActive)
	c.Assert(k.SetNodeAccount(ctx, activeNode), IsNil)
	dummyMgr := NewDummyMgrWithKeeper(k)
	handler := NewDepositHandler(dummyMgr)

	addr := GetRandomBech32Addr()

	coins := common.Coins{
		common.NewCoin(common.BaseNative, cosmos.NewUint(200*common.One)),
	}

	funds, err := common.NewCoin(common.BaseNative, cosmos.NewUint(300*common.One)).Native()
	c.Assert(err, IsNil)
	err = k.AddCoins(ctx, addr, cosmos.NewCoins(funds))
	c.Assert(err, IsNil)
	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.BalanceCacao = cosmos.NewUint(100 * common.One)
	pool.Status = PoolAvailable
	c.Assert(k.SetPool(ctx, pool), IsNil)
	msg := NewMsgDeposit(coins, "ADD:BNB.BNB", addr)

	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	// ensure observe tx had been saved
	hash := tmtypes.Tx(ctx.TxBytes()).Hash()
	txID, err := common.NewTxID(fmt.Sprintf("%X", hash))
	c.Assert(err, IsNil)
	voter, err := k.GetObservedTxInVoter(ctx, txID)
	c.Assert(err, IsNil)
	c.Assert(voter.Tx.IsEmpty(), Equals, false)
	c.Assert(voter.Tx.Status, Equals, types.Status_done)
}

type HandlerDepositTestHelper struct {
	keeper.Keeper
}

func NewHandlerDepositTestHelper(k keeper.Keeper) *HandlerDepositTestHelper {
	return &HandlerDepositTestHelper{
		Keeper: k,
	}
}

func (s *HandlerDepositSuite) TestDifferentValidation(c *C) {
	acctAddr := GetRandomBech32Addr()
	testCases := []struct {
		name            string
		messageProvider func(c *C, ctx cosmos.Context, helper *HandlerDepositTestHelper) cosmos.Msg
		validator       func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerDepositTestHelper, name string)
	}{
		{
			name: "invalid message should result an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *HandlerDepositTestHelper) cosmos.Msg {
				return NewMsgNetworkFee(ctx.BlockHeight(), common.BNBChain, 1, bnbSingleTxFee.Uint64(), GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerDepositTestHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
				c.Check(errors.Is(err, errInvalidMessage), Equals, true, Commentf(name))
			},
		},
		{
			name: "coin is not on BASEChain should result in an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *HandlerDepositTestHelper) cosmos.Msg {
				return NewMsgDeposit(common.Coins{
					common.NewCoin(common.BNBAsset, cosmos.NewUint(100)),
				}, "hello", GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerDepositTestHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
		},
		{
			name: "Insufficient funds should result in an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *HandlerDepositTestHelper) cosmos.Msg {
				return NewMsgDeposit(common.Coins{
					common.NewCoin(common.BaseNative, cosmos.NewUint(100)),
				}, "hello", GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerDepositTestHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
				c.Check(errors.Is(err, se.ErrInsufficientFunds), Equals, true, Commentf(name))
			},
		},
		{
			name: "invalid memo should refund",
			messageProvider: func(c *C, ctx cosmos.Context, helper *HandlerDepositTestHelper) cosmos.Msg {
				FundAccount(c, ctx, helper.Keeper, acctAddr, 100)
				vault := NewVault(ctx.BlockHeight(), ActiveVault, AsgardVault, GetRandomPubKey(), common.Chains{common.BNBChain, common.BASEChain}.Strings(), []ChainContract{})
				c.Check(helper.Keeper.SetVault(ctx, vault), IsNil)
				return NewMsgDeposit(common.Coins{
					common.NewCoin(common.BaseNative, cosmos.NewUint(2*common.One)),
				}, "hello", acctAddr)
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerDepositTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
				coins := common.NewCoin(common.BaseNative, cosmos.NewUint(68*common.One))
				coin, err := coins.Native()
				c.Check(err, IsNil)
				hasCoin := helper.Keeper.HasCoins(ctx, acctAddr, cosmos.NewCoins().Add(coin))
				c.Check(hasCoin, Equals, true)
			},
		},
	}
	for _, tc := range testCases {
		ctx, mgr := setupManagerForTest(c)
		helper := NewHandlerDepositTestHelper(mgr.Keeper())
		mgr.K = helper
		handler := NewDepositHandler(mgr)
		msg := tc.messageProvider(c, ctx, helper)
		result, err := handler.Run(ctx, msg)
		tc.validator(c, ctx, result, err, helper, tc.name)
	}
}

func (s *HandlerDepositSuite) TestAddSwap(c *C) {
	SetupConfigForTest()
	ctx, mgr := setupManagerForTest(c)
	handler := NewDepositHandler(mgr)
	affAddr := GetRandomBaseAddress()
	tx := common.NewTx(
		GetRandomTxHash(),
		GetRandomBaseAddress(),
		GetRandomBaseAddress(),
		common.Coins{common.NewCoin(common.BaseNative, cosmos.NewUint(common.One))},
		common.Gas{
			{Asset: common.BNBAsset, Amount: cosmos.NewUint(37500)},
		},
		fmt.Sprintf("=:BTC.BTC:%s", GetRandomBTCAddress().String()),
	)
	// no affiliate fee
	msg := NewMsgSwap(tx, common.BTCAsset, GetRandomBTCAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(), "", "", nil, MarketOrder, 0, 0, GetRandomBech32Addr())

	handler.addSwap(ctx, *msg)
	swap, err := mgr.Keeper().GetSwapQueueItem(ctx, tx.ID, 0)
	c.Assert(err, IsNil)
	c.Assert(swap.String(), Equals, msg.String())

	tx.Memo = fmt.Sprintf("=:BTC.BTC:%s::%s:20000", GetRandomBTCAddress().String(), affAddr.String())

	// affiliate fee, with more than 10K as basis points
	msg1 := NewMsgSwap(tx, common.BTCAsset, GetRandomBTCAddress(), cosmos.ZeroUint(), GetRandomBaseAddress(), cosmos.NewUint(20000), "", "", nil, MarketOrder, 0, 0, GetRandomBech32Addr())

	// Check balance before swap
	affiliateFeeAddr, err := msg1.GetAffiliateAddress().AccAddress()
	c.Assert(err, IsNil)
	acct := mgr.Keeper().GetBalance(ctx, affiliateFeeAddr)
	c.Assert(acct.AmountOf(common.BaseNative.Native()).String(), Equals, "0")

	handler.addSwap(ctx, *msg1)
	swap, err = mgr.Keeper().GetSwapQueueItem(ctx, tx.ID, 0)
	c.Assert(err, IsNil)
	c.Assert(swap.Tx.Coins[0].Amount.IsZero(), Equals, false)
	// Check balance after swap, should be the same
	c.Assert(acct.AmountOf(common.BaseNative.Native()).String(), Equals, "0")

	// affiliate fee not taken on deposit
	tx.Memo = fmt.Sprintf("=:BTC.BTC:%s::%s:1000", GetRandomBTCAddress().String(), affAddr.String())
	tx.Coins[0].Amount = cosmos.NewUint(common.One)
	msg2 := NewMsgSwap(tx, common.BTCAsset, GetRandomBTCAddress(), cosmos.ZeroUint(), GetRandomBaseAddress(), cosmos.NewUint(1000), "", "", nil, MarketOrder, 0, 0, GetRandomBech32Addr())
	handler.addSwap(ctx, *msg2)
	swap, err = mgr.Keeper().GetSwapQueueItem(ctx, tx.ID, 0)
	c.Assert(err, IsNil)
	c.Assert(swap.Tx.Coins[0].Amount.IsZero(), Equals, false)
	c.Assert(swap.Tx.Coins[0].Amount.String(), Equals, cosmos.NewUint(common.One).String())

	affiliateFeeAddr2, err := msg2.GetAffiliateAddress().AccAddress()
	c.Assert(err, IsNil)
	acct2 := mgr.Keeper().GetBalance(ctx, affiliateFeeAddr2)
	c.Assert(acct2.AmountOf(common.BaseNative.Native()).String(), Equals, strconv.FormatInt(0, 10))

	// NONE CACAO , synth asset should be handled correctly

	synthAsset, err := common.NewAsset("BTC/BTC")
	c.Assert(err, IsNil)
	tx1 := common.NewTx(
		GetRandomTxHash(),
		GetRandomBaseAddress(),
		GetRandomBaseAddress(),
		common.Coins{common.NewCoin(synthAsset, cosmos.NewUint(common.One))},
		common.Gas{
			{Asset: common.BaseNative, Amount: cosmos.NewUint(200000)},
		},
		tx.Memo,
	)

	c.Assert(mgr.Keeper().MintToModule(ctx, ModuleName, tx1.Coins[0]), IsNil)
	c.Assert(mgr.Keeper().SendFromModuleToModule(ctx, ModuleName, AsgardName, tx1.Coins), IsNil)
	msg3 := NewMsgSwap(tx1, common.BTCAsset, GetRandomBTCAddress(), cosmos.ZeroUint(), GetRandomBaseAddress(), cosmos.NewUint(1000), "", "", nil, MarketOrder, 0, 0, GetRandomBech32Addr())
	handler.addSwap(ctx, *msg3)
	swap, err = mgr.Keeper().GetSwapQueueItem(ctx, tx1.ID, 0)
	c.Assert(err, IsNil)
	c.Assert(swap.Tx.Coins[0].Amount.IsZero(), Equals, false)
	c.Assert(swap.Tx.Coins[0].Amount.String(), Equals, cosmos.NewUint(common.One).String())

	// affiliate fee not taken on deposit
	affiliateFeeAddr3, err := msg3.GetAffiliateAddress().AccAddress()
	c.Assert(err, IsNil)
	acct3 := mgr.Keeper().GetBalance(ctx, affiliateFeeAddr3)
	c.Assert(acct3.AmountOf(synthAsset.Native()).String(), Equals, strconv.FormatInt(0, 10))
}

func (s *HandlerDepositSuite) TestTargetModule(c *C) {
	fee := common.NewCoin(common.BaseAsset(), cosmos.NewUint(20_00000000))
	gasFee := common.NewCoin(common.BaseAsset(), cosmos.NewUint(18_00000000))
	mayaFee := common.NewCoin(common.BaseAsset(), cosmos.NewUint(2_00000000))
	acctAddr := GetRandomBech32Addr()
	mayaAcctAddr := GetRandomBech32Addr()
	testCases := []struct {
		name            string
		moduleName      string
		messageProvider func(c *C, ctx cosmos.Context) *MsgDeposit
		validator       func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, name string, balDelta cosmos.Uint)
	}{
		{
			name:       "90 percent of gas coins should go to reserve",
			moduleName: ReserveName,
			messageProvider: func(c *C, ctx cosmos.Context) *MsgDeposit {
				addr := GetRandomBaseAddress()
				coin := common.NewCoin(common.BaseAsset(), cosmos.NewUint(2000_00000000))
				return NewMsgDeposit(common.Coins{coin}, "name:test:MAYA:"+addr.String(), acctAddr)
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, name string, balDelta cosmos.Uint) {
				c.Check(err, IsNil, Commentf(name))
				c.Assert(cosmos.NewUint(2000_00000000).Add(gasFee.Amount).String(), Equals, balDelta.String(), Commentf(name))
			},
		},
		{
			name:       "10 percent of gas coins should go to maya fund",
			moduleName: MayaFund,
			messageProvider: func(c *C, ctx cosmos.Context) *MsgDeposit {
				addr := GetRandomBaseAddress()
				coin := common.NewCoin(common.BaseAsset(), cosmos.NewUint(2000_00000000))
				return NewMsgDeposit(common.Coins{coin}, "name:test:MAYA:"+addr.String(), mayaAcctAddr)
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, name string, balDelta cosmos.Uint) {
				c.Check(err, IsNil, Commentf(name))
				c.Assert(mayaFee.Amount.String(), Equals, balDelta.String(), Commentf(name))
			},
		},
	}
	for _, tc := range testCases {
		ctx, mgr := setupManagerForTest(c)
		handler := NewDepositHandler(mgr)
		msg := tc.messageProvider(c, ctx)
		totalCoins := common.NewCoins(msg.Coins[0])
		totalCoins.Add(fee)
		c.Assert(mgr.Keeper().MintToModule(ctx, ModuleName, totalCoins[0]), IsNil)
		c.Assert(mgr.Keeper().SendFromModuleToAccount(ctx, ModuleName, msg.Signer, totalCoins), IsNil)
		balBefore := mgr.Keeper().GetRuneBalanceOfModule(ctx, tc.moduleName)
		result, err := handler.Run(ctx, msg)
		balAfter := mgr.Keeper().GetRuneBalanceOfModule(ctx, tc.moduleName)
		balDelta := balAfter.Sub(balBefore)
		tc.validator(c, ctx, result, err, tc.name, balDelta)
	}
}

func (s *MultipleAffiliatesSuite) TestMultipleAffiliatesNoMayaAliasOrOwner(c *C) {
	s.errLogs.showLogs = true
	// investigating this:
	// =:e:0xf8856124ea157D6f26472E38224ee6744aD13AF3:720740882/5/40:_/ts:5/50

	addr := GetRandomBaseAddress()
	owner, _ := addr.AccAddress()
	mn := NewMAYAName("_", 50,
		[]MAYANameAlias{
			// no maya alias
			{
				Chain:   common.ETHChain,
				Address: "0x2d4f72825c5908b6fca5a624f1b412b6e1d79bb4",
			},
		},
		common.ETHAsset,
		owner,
		EmptyBps,
		nil,
	)
	s.mgr.Keeper().SetMAYAName(s.ctx, mn)

	addr = GetRandomBaseAddress()
	owner, _ = addr.AccAddress()
	mn = NewMAYAName("ts", 50,
		[]MAYANameAlias{
			{
				Chain:   common.BASEChain,
				Address: addr,
			},
			{
				Chain:   common.ETHChain,
				Address: "0x546e7b1f4b4df6cdb19fbddff325133ebfe04ba7",
			},
		},
		common.USDCAsset,
		owner,
		EmptyBps,
		nil,
	)
	s.mgr.Keeper().SetMAYAName(s.ctx, mn)

	memo := "=:e:0xf8856124ea157D6f26472E38224ee6744aD13AF3:720740882/5/40:_/ts:5/50"
	var err error
	var swaps swapItems

	swapAmt := cosmos.NewUint(100000 * common.One) // swap 10 cacao
	coins := common.Coins{common.NewCoin(common.BaseNative, swapAmt)}
	msg := NewMsgDeposit(coins, memo, s.signer)
	c.Assert(s.handleDeposit(msg), IsNil)
	// verify swap queue
	swaps, err = s.fetchQueue()
	c.Assert(err, IsNil)
	c.Assert(swaps, HasLen, 1) // main swap
	c.Assert(s.queue.EndBlock(s.ctx, s.mgr), IsNil)
	swaps, err = s.fetchQueue()
	c.Assert(err, IsNil)
	c.Assert(swaps, HasLen, 2) // two affiliate swaps
	c.Assert(s.queue.EndBlock(s.ctx, s.mgr), IsNil)
	s.clear()

	gasFee := s.mgr.gasMgr.GetFee(s.ctx, common.BTCChain, common.BTCAsset)
	s.tx.Gas = common.Gas{common.NewCoin(common.BTCAsset, gasFee)}
	s.tx.FromAddress = GetRandomBTCAddress()
	s.tx.Coins = common.Coins{common.NewCoin(common.BTCAsset, cosmos.NewUint(21000000))}
	s.tx.Memo = memo
	// process the tx
	c.Assert(s.processTx(), IsNil)
	// verify swap queue
	swaps, err = s.fetchQueue()
	c.Assert(err, IsNil)
	c.Assert(swaps, HasLen, 1) // main swap
	c.Assert(s.queue.EndBlock(s.ctx, s.mgr), IsNil)
	swaps, err = s.fetchQueue()
	c.Assert(err, IsNil)
	c.Assert(swaps, HasLen, 2) // two affiliate swaps
	c.Assert(s.queue.EndBlock(s.ctx, s.mgr), IsNil)
	s.clear()

	mn = NewMAYAName("_2", 50,
		[]MAYANameAlias{
			// no maya alias
			{
				Chain:   common.ETHChain,
				Address: "0x2d4f72825c5908b6fca5a624f1b412b6e1d79bb4",
			},
		},
		common.ETHAsset,
		// no owner
		nil,
		EmptyBps,
		nil,
	)
	s.mgr.Keeper().SetMAYAName(s.ctx, mn)

	memo = "=:e:0xf8856124ea157D6f26472E38224ee6744aD13AF3:720740882/5/40:_2/ts:5/50"

	msg = NewMsgDeposit(coins, memo, s.signer)
	c.Assert(s.handleDeposit(msg), IsNil)
	// verify swap queue
	swaps, err = s.fetchQueue()
	c.Assert(err, IsNil)
	c.Assert(swaps, HasLen, 1) // main swap
	c.Assert(s.queue.EndBlock(s.ctx, s.mgr), IsNil)
	swaps, err = s.fetchQueue()
	c.Assert(err, IsNil)
	c.Assert(swaps, HasLen, 2) // two affiliate swaps
	c.Assert(s.queue.EndBlock(s.ctx, s.mgr), IsNil)
	s.clear()

	s.tx.Memo = memo
	// process the tx
	c.Assert(s.processTx(), IsNil)
	// verify swap queue
	swaps, err = s.fetchQueue()
	c.Assert(err, IsNil)
	c.Assert(swaps, HasLen, 1) // main swap
	c.Assert(s.queue.EndBlock(s.ctx, s.mgr), IsNil)
	swaps, err = s.fetchQueue()
	c.Assert(err, IsNil)
	c.Assert(swaps, HasLen, 2) // two affiliate swaps
	s.clear()
}
