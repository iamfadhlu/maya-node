package mayachain

import (
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
)

type SwapQueueVCURSuite struct{}

var _ = Suite(&SwapQueueVCURSuite{})

func (s SwapQueueVCURSuite) TestGetTodoNum(c *C) {
	queue := newSwapQueueVCUR(keeper.KVStoreDummy{})

	c.Check(queue.getTodoNum(50, 10, 100), Equals, int64(25))     // halves it
	c.Check(queue.getTodoNum(11, 10, 100), Equals, int64(5))      // halves it
	c.Check(queue.getTodoNum(10, 10, 100), Equals, int64(10))     // does all of them
	c.Check(queue.getTodoNum(1, 10, 100), Equals, int64(1))       // does all of them
	c.Check(queue.getTodoNum(0, 10, 100), Equals, int64(0))       // does none
	c.Check(queue.getTodoNum(10000, 10, 100), Equals, int64(100)) // does max 100
	c.Check(queue.getTodoNum(200, 10, 100), Equals, int64(100))   // does max 100
}

func (s SwapQueueVCURSuite) TestScoreMsgs(c *C) {
	ctx, k := setupKeeperForTest(c)
	_, mgr := setupManagerForTest(c)

	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceCacao = cosmos.NewUint(143166 * common.One)
	pool.BalanceAsset = cosmos.NewUint(1000 * common.One)
	c.Assert(k.SetPool(ctx, pool), IsNil)
	pool = NewPool()
	pool.Asset = common.BTCAsset
	pool.BalanceCacao = cosmos.NewUint(73708333 * common.One)
	pool.BalanceAsset = cosmos.NewUint(1000 * common.One)
	c.Assert(k.SetPool(ctx, pool), IsNil)
	pool = NewPool()
	pool.Asset = common.ETHAsset
	pool.BalanceCacao = cosmos.NewUint(1000 * common.One)
	pool.BalanceAsset = cosmos.NewUint(1000 * common.One)
	pool.Status = PoolStaged
	c.Assert(k.SetPool(ctx, pool), IsNil)

	queue := newSwapQueueVCUR(k)

	// check that we sort by liquidity ok
	msgs := []*MsgSwap{
		NewMsgSwap(common.Tx{
			ID:    common.TxID("5E1DF027321F1FE37CA19B9ECB11C2B4ABEC0D8322199D335D9CE4C39F85F115"),
			Coins: common.Coins{common.NewCoin(common.BaseAsset(), cosmos.NewUint(2*common.One))},
		}, common.BNBAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			0, 0,
			GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    common.TxID("53C1A22436B385133BDD9157BB365DB7AAC885910D2FA7C9DC3578A04FFD4ADC"),
			Coins: common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(50*common.One))},
		}, common.BaseAsset(), GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			0, 0,
			GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    common.TxID("6A470EB9AFE82981979A5EEEED3296E1E325597794BD5BFB3543A372CAF435E5"),
			Coins: common.Coins{common.NewCoin(common.BaseAsset(), cosmos.NewUint(1*common.One))},
		}, common.BNBAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			0, 0,
			GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    common.TxID("5EE9A7CCC55A3EBAFA0E542388CA1B909B1E3CE96929ED34427B96B7CCE9F8E8"),
			Coins: common.Coins{common.NewCoin(common.BaseAsset(), cosmos.NewUint(100*common.One))},
		}, common.BNBAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			0, 0,
			GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    common.TxID("0FF2A521FB11FFEA4DFE3B7AD4066FF0A33202E652D846F8397EFC447C97A91B"),
			Coins: common.Coins{common.NewCoin(common.BaseAsset(), cosmos.NewUint(10*common.One))},
		}, common.BNBAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			0, 0,
			GetRandomBech32Addr()),

		NewMsgSwap(common.Tx{
			ID:    GetRandomTxHash(),
			Coins: common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(150*common.One))},
		}, common.BaseAsset(), GetRandomBaseAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			0, 0,
			GetRandomBech32Addr()),

		NewMsgSwap(common.Tx{
			ID:    GetRandomTxHash(),
			Coins: common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(151*common.One))},
		}, common.BaseAsset(), GetRandomBaseAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			0, 0,
			GetRandomBech32Addr()),

		// synthetics can be redeemed on unavailable pools, should score
		NewMsgSwap(common.Tx{
			ID:    GetRandomTxHash(),
			Coins: common.Coins{common.NewCoin(common.ETHAsset.GetSyntheticAsset(), cosmos.NewUint(3*common.One))},
		}, common.BaseAsset(), GetRandomBaseAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			0, 0,
			GetRandomBech32Addr()),
	}

	swaps := make(swapItems, len(msgs))
	for i, msg := range msgs {
		swaps[i] = swapItem{
			msg:  *msg,
			fee:  cosmos.ZeroUint(),
			slip: cosmos.ZeroUint(),
		}
	}
	swaps, err := queue.scoreMsgs(ctx, swaps, 10_000, mgr)
	c.Assert(err, IsNil)
	swaps = swaps.Sort()
	c.Check(swaps, HasLen, 8)
	c.Check(swaps[0].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(151*common.One)), Equals, true, Commentf("%d", swaps[0].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[1].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(150*common.One)), Equals, true, Commentf("%d", swaps[1].msg.Tx.Coins[0].Amount.Uint64()))
	// 50 BNB is worth more than 100 RUNE
	c.Check(swaps[2].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(50*common.One)), Equals, true, Commentf("%d", swaps[2].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[3].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(3*common.One)), Equals, true, Commentf("%d", swaps[3].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[4].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(100*common.One)), Equals, true, Commentf("%d", swaps[4].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[5].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(10*common.One)), Equals, true, Commentf("%d", swaps[5].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[6].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(2*common.One)), Equals, true, Commentf("%d", swaps[6].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[7].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(1*common.One)), Equals, true, Commentf("%d", swaps[7].msg.Tx.Coins[0].Amount.Uint64()))

	// check that slip is taken into account
	msgs = []*MsgSwap{
		NewMsgSwap(common.Tx{
			ID:    GetRandomTxHash(),
			Coins: common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(2*common.One))},
		}, common.BaseAsset(), GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			0, 0,
			GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    GetRandomTxHash(),
			Coins: common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(50*common.One))},
		}, common.BaseAsset(), GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			0, 0,
			GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    GetRandomTxHash(),
			Coins: common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(1*common.One))},
		}, common.BaseAsset(), GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			0, 0,
			GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    GetRandomTxHash(),
			Coins: common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100*common.One))},
		}, common.BaseAsset(), GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			0, 0,
			GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    GetRandomTxHash(),
			Coins: common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(10*common.One))},
		}, common.BaseAsset(), GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			0, 0,
			GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    GetRandomTxHash(),
			Coins: common.Coins{common.NewCoin(common.BTCAsset, cosmos.NewUint(2*common.One))},
		}, common.BaseAsset(), GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			0, 0,
			GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    GetRandomTxHash(),
			Coins: common.Coins{common.NewCoin(common.BTCAsset, cosmos.NewUint(50*common.One))},
		}, common.BaseAsset(), GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			0, 0,
			GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    GetRandomTxHash(),
			Coins: common.Coins{common.NewCoin(common.BTCAsset, cosmos.NewUint(1*common.One))},
		}, common.BaseAsset(), GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			0, 0,
			GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    GetRandomTxHash(),
			Coins: common.Coins{common.NewCoin(common.BTCAsset, cosmos.NewUint(100*common.One))},
		}, common.BaseAsset(), GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			0, 0,
			GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    GetRandomTxHash(),
			Coins: common.Coins{common.NewCoin(common.BTCAsset, cosmos.NewUint(10*common.One))},
		}, common.BaseAsset(), GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			0, 0,
			GetRandomBech32Addr()),

		NewMsgSwap(common.Tx{
			ID:    GetRandomTxHash(),
			Coins: common.Coins{common.NewCoin(common.BTCAsset, cosmos.NewUint(10*common.One))},
		}, common.BNBAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			0, 0,
			GetRandomBech32Addr()),
	}

	swaps = make(swapItems, len(msgs))
	for i, msg := range msgs {
		swaps[i] = swapItem{
			msg:  *msg,
			fee:  cosmos.ZeroUint(),
			slip: cosmos.ZeroUint(),
		}
	}
	swaps, err = queue.scoreMsgs(ctx, swaps, 10_000, mgr)
	c.Assert(err, IsNil)
	swaps = swaps.Sort()
	c.Assert(swaps, HasLen, 11)

	c.Check(swaps[0].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(10*common.One)), Equals, true, Commentf("%d", swaps[0].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[0].msg.Tx.Coins[0].Asset.Equals(common.BTCAsset), Equals, true)

	c.Check(swaps[1].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(100*common.One)), Equals, true, Commentf("%d", swaps[1].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[1].msg.Tx.Coins[0].Asset.Equals(common.BTCAsset), Equals, true)

	c.Check(swaps[2].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(100*common.One)), Equals, true, Commentf("%d", swaps[2].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[2].msg.Tx.Coins[0].Asset.Equals(common.BNBAsset), Equals, true)

	c.Check(swaps[3].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(50*common.One)), Equals, true, Commentf("%d", swaps[3].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[3].msg.Tx.Coins[0].Asset.Equals(common.BTCAsset), Equals, true)

	c.Check(swaps[4].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(50*common.One)), Equals, true, Commentf("%d", swaps[4].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[4].msg.Tx.Coins[0].Asset.Equals(common.BNBAsset), Equals, true)

	c.Check(swaps[5].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(10*common.One)), Equals, true, Commentf("%d", swaps[5].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[5].msg.Tx.Coins[0].Asset.Equals(common.BTCAsset), Equals, true)

	c.Check(swaps[6].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(10*common.One)), Equals, true, Commentf("%d", swaps[6].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[6].msg.Tx.Coins[0].Asset.Equals(common.BNBAsset), Equals, true)

	c.Check(swaps[7].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(2*common.One)), Equals, true, Commentf("%d", swaps[7].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[7].msg.Tx.Coins[0].Asset.Equals(common.BTCAsset), Equals, true)

	c.Check(swaps[8].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(2*common.One)), Equals, true, Commentf("%d", swaps[8].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[8].msg.Tx.Coins[0].Asset.Equals(common.BNBAsset), Equals, true)

	c.Check(swaps[9].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(1*common.One)), Equals, true, Commentf("%d", swaps[9].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[9].msg.Tx.Coins[0].Asset.Equals(common.BTCAsset), Equals, true)

	c.Check(swaps[10].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(1*common.One)), Equals, true, Commentf("%d", swaps[10].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[10].msg.Tx.Coins[0].Asset.Equals(common.BNBAsset), Equals, true)
}

func (s SwapQueueVCURSuite) TestStreamingSwapSelection(c *C) {
	ctx, mgr := setupManagerForTest(c)
	k := mgr.Keeper()
	queue := newSwapQueueVCUR(k)

	bnbAddr := GetRandomBNBAddress()
	txID := GetRandomTxHash()
	tx := common.NewTx(
		txID,
		bnbAddr,
		bnbAddr,
		common.NewCoins(common.NewCoin(common.BaseAsset(), cosmos.NewUint(common.One*100))),
		BNBGasFeeSingleton,
		"",
	)

	// happy path
	msg := NewMsgSwap(tx, common.BNBAsset.GetSyntheticAsset(), GetRandomBaseAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(), "", "", nil, MarketOrder, 10, 20, GetRandomBech32Addr())
	c.Assert(k.SetSwapQueueItem(ctx, *msg, 0), IsNil)

	// no saved streaming swap, should swap now
	items, err := queue.FetchQueue(ctx, mgr)
	c.Assert(err, IsNil)
	c.Check(items, HasLen, 1)

	// save streaming swap data, should have same result
	swp := msg.GetStreamingSwap()
	k.SetStreamingSwap(ctx, swp)
	items, err = queue.FetchQueue(ctx, mgr)
	c.Assert(err, IsNil)
	c.Check(items, HasLen, 1)

	// last height is this block, no result
	swp.LastHeight = ctx.BlockHeight()
	k.SetStreamingSwap(ctx, swp)
	items, err = queue.FetchQueue(ctx, mgr)
	c.Assert(err, IsNil)
	c.Check(items, HasLen, 0)

	// last height is halfway there
	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + (int64(swp.Interval) / 2))
	items, err = queue.FetchQueue(ctx, mgr)
	c.Assert(err, IsNil)
	c.Check(items, HasLen, 0)

	// last height is interval blocks ago
	ctx = ctx.WithBlockHeight(swp.LastHeight + int64(swp.Interval))
	items, err = queue.FetchQueue(ctx, mgr)
	c.Assert(err, IsNil)
	c.Check(items, HasLen, 1)
}
