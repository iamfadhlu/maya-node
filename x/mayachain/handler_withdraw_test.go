package mayachain

import (
	"errors"
	"fmt"
	"strings"

	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
)

type HandlerWithdrawSuite struct{}

var _ = Suite(&HandlerWithdrawSuite{})

type MockWithdrawKeeper struct {
	keeper.KVStoreDummy
	activeNodeAccount     NodeAccount
	currentPool           Pool
	failPool              bool
	suspendedPool         bool
	failLiquidityProvider bool
	failAddEvents         bool
	lp                    LiquidityProvider
	keeper                keeper.Keeper
	pol                   ProtocolOwnedLiquidity
	polAddress            common.Address
}

func (mfp *MockWithdrawKeeper) PoolExist(_ cosmos.Context, asset common.Asset) bool {
	return mfp.currentPool.Asset.Equals(asset)
}

// GetPool return a pool
func (mfp *MockWithdrawKeeper) GetPool(_ cosmos.Context, _ common.Asset) (Pool, error) {
	if mfp.failPool {
		return Pool{}, errors.New("test error")
	}
	if mfp.suspendedPool {
		return Pool{
			BalanceCacao: cosmos.ZeroUint(),
			BalanceAsset: cosmos.ZeroUint(),
			Asset:        common.BNBAsset,
			LPUnits:      cosmos.ZeroUint(),
			Status:       PoolSuspended,
		}, nil
	}
	return mfp.currentPool, nil
}

func (mfp *MockWithdrawKeeper) SetPool(_ cosmos.Context, pool Pool) error {
	mfp.currentPool = pool
	return nil
}

func (mfp *MockWithdrawKeeper) GetModuleAddress(mod string) (common.Address, error) {
	return mfp.polAddress, nil
}

func (mfp *MockWithdrawKeeper) GetPOL(_ cosmos.Context) (ProtocolOwnedLiquidity, error) {
	return mfp.pol, nil
}

func (mfp *MockWithdrawKeeper) SetPOL(_ cosmos.Context, pol ProtocolOwnedLiquidity) error {
	mfp.pol = pol
	return nil
}

func (mfp *MockWithdrawKeeper) GetNodeAccount(_ cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error) {
	if mfp.activeNodeAccount.NodeAddress.Equals(addr) {
		return mfp.activeNodeAccount, nil
	}
	return NodeAccount{}, nil
}

func (mfp *MockWithdrawKeeper) GetLiquidityProviderIterator(ctx cosmos.Context, _ common.Asset) cosmos.Iterator {
	iter := keeper.NewDummyIterator()
	iter.AddItem([]byte("key"), mfp.Cdc().MustMarshal(&mfp.lp))
	return iter
}

func (mfp *MockWithdrawKeeper) GetLiquidityProvider(ctx cosmos.Context, asset common.Asset, addr common.Address) (LiquidityProvider, error) {
	if mfp.failLiquidityProvider {
		return LiquidityProvider{}, errors.New("fail to get liquidity provider")
	}
	return mfp.lp, nil
}

func (mfp *MockWithdrawKeeper) SetLiquidityProvider(_ cosmos.Context, lp LiquidityProvider) {
	mfp.lp = lp
}

func (mfp *MockWithdrawKeeper) GetGas(ctx cosmos.Context, asset common.Asset) ([]cosmos.Uint, error) {
	return []cosmos.Uint{cosmos.NewUint(37500), cosmos.NewUint(30000)}, nil
}

func (HandlerWithdrawSuite) TestWithdrawHandler(c *C) {
	// w := getHandlerTestWrapper(c, 1, true, true)
	SetupConfigForTest()
	ctx, keeper := setupKeeperForTest(c)
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	runeAddr := GetRandomBaseAddress()
	k := &MockWithdrawKeeper{
		keeper:            keeper,
		activeNodeAccount: activeNodeAccount,
		currentPool: Pool{
			BalanceCacao:        cosmos.ZeroUint(),
			BalanceAsset:        cosmos.ZeroUint(),
			Asset:               common.BNBAsset,
			LPUnits:             cosmos.ZeroUint(),
			SynthUnits:          cosmos.ZeroUint(),
			PendingInboundCacao: cosmos.ZeroUint(),
			PendingInboundAsset: cosmos.ZeroUint(),
			Status:              PoolAvailable,
		},
		lp: LiquidityProvider{
			Units:                     cosmos.ZeroUint(),
			PendingCacao:              cosmos.ZeroUint(),
			PendingAsset:              cosmos.ZeroUint(),
			CacaoDepositValue:         cosmos.ZeroUint(),
			AssetDepositValue:         cosmos.ZeroUint(),
			WithdrawCounter:           cosmos.ZeroUint(),
			LastWithdrawCounterHeight: 0,
		},
		pol:        NewProtocolOwnedLiquidity(),
		polAddress: runeAddr,
	}
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)
	// Happy path , this is a round trip , first we provide liquidity, then we withdraw
	addHandler := NewAddLiquidityHandler(NewDummyMgrWithKeeper(k))
	err := addHandler.addLiquidity(ctx,
		common.BNBAsset,
		cosmos.NewUint(common.One*100),
		cosmos.NewUint(common.One*100),
		runeAddr,
		GetRandomBNBAddress(),
		GetRandomTx(),
		false,
		constAccessor,
		0)
	c.Assert(err, IsNil)
	// let's just withdraw
	withdrawHandler := NewWithdrawLiquidityHandler(NewDummyMgrWithKeeper(k))

	tx := GetRandomTx()
	tx.Chain = common.BASEChain
	msgWithdraw := NewMsgWithdrawLiquidity(tx, runeAddr, cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), common.BNBAsset, common.EmptyAsset, activeNodeAccount.NodeAddress)
	_, err = withdrawHandler.Run(ctx, msgWithdraw)
	c.Assert(err, IsNil)

	pol, err := k.GetPOL(ctx)
	c.Assert(err, IsNil)
	c.Check(pol.CacaoWithdrawn.Uint64(), Equals, uint64(100*common.One))

	// Bad version should fail
	_, err = withdrawHandler.Run(ctx, msgWithdraw)
	c.Assert(err, NotNil)
}

func (HandlerWithdrawSuite) TestAsymmetricWithdraw(c *C) {
	SetupConfigForTest()
	ctx, keeper := setupKeeperForTest(c)
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)
	pool := NewPool()
	pool.Asset = common.BTCAsset
	pool.BalanceAsset = cosmos.ZeroUint()
	pool.BalanceCacao = cosmos.ZeroUint()
	pool.Status = PoolAvailable
	c.Assert(keeper.SetPool(ctx, pool), IsNil)
	// Happy path , this is a round trip , first we provide liquidity, then we withdraw
	// Let's stake some BTC first
	runeAddr := GetRandomBaseAddress()
	btcAddress := GetRandomBTCAddress()
	addHandler := NewAddLiquidityHandler(NewDummyMgrWithKeeper(keeper))
	// stake some RUNE first
	err := addHandler.addLiquidity(ctx,
		common.BTCAsset,
		cosmos.NewUint(common.One*100),
		cosmos.ZeroUint(),
		runeAddr,
		btcAddress,
		GetRandomTx(),
		true,
		constAccessor,
		0)
	c.Assert(err, IsNil)
	lp, err := keeper.GetLiquidityProvider(ctx, common.BTCAsset, runeAddr)
	c.Assert(err, IsNil)
	c.Assert(lp.Valid(), IsNil)
	c.Assert(lp.PendingCacao.Equal(cosmos.NewUint(common.One*100)), Equals, true)
	// Stake some BTC , make sure stake finished
	err = addHandler.addLiquidity(ctx, common.BTCAsset, cosmos.ZeroUint(), cosmos.NewUint(100*common.One), runeAddr, btcAddress, GetRandomTx(), false, constAccessor, 0)
	c.Assert(err, IsNil)
	lp, err = keeper.GetLiquidityProvider(ctx, common.BTCAsset, runeAddr)
	c.Assert(err, IsNil)
	c.Assert(lp.Valid(), IsNil)
	c.Assert(lp.PendingCacao.IsZero(), Equals, true)
	// symmetric stake, units is measured by liquidity tokens
	c.Assert(lp.Units.IsZero(), Equals, false)

	runeAddr1 := GetRandomBaseAddress()
	err = addHandler.addLiquidity(ctx, common.BTCAsset, cosmos.NewUint(50*common.One), cosmos.ZeroUint(), runeAddr1, common.NoAddress, GetRandomTx(), false, constAccessor, 0)
	c.Assert(err, IsNil)
	lp, err = keeper.GetLiquidityProvider(ctx, common.BTCAsset, runeAddr1)
	c.Assert(err, IsNil)
	c.Assert(lp.Valid(), IsNil)
	c.Assert(lp.PendingCacao.IsZero(), Equals, true)
	c.Assert(lp.PendingAsset.IsZero(), Equals, true)
	c.Assert(lp.Units.IsZero(), Equals, false)

	// let's withdraw the RUNE we just staked
	withdrawHandler := NewWithdrawLiquidityHandler(NewDummyMgrWithKeeper(keeper))
	tx := GetRandomTx()
	tx.Chain = common.BASEChain
	msgWithdraw := NewMsgWithdrawLiquidity(tx, runeAddr1, cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), common.BTCAsset, common.EmptyAsset, activeNodeAccount.NodeAddress)
	_, err = withdrawHandler.Run(ctx, msgWithdraw)
	c.Assert(err, IsNil)
	lp, err = keeper.GetLiquidityProvider(ctx, common.BTCAsset, runeAddr1)
	c.Assert(err, IsNil)
	c.Assert(lp.Valid(), NotNil)

	// stake some BTC only
	btcAddress1 := GetRandomBTCAddress()
	err = addHandler.addLiquidity(ctx, common.BTCAsset, cosmos.ZeroUint(), cosmos.NewUint(50*common.One),
		common.NoAddress, btcAddress1, GetRandomTx(), false, constAccessor, 0)
	c.Assert(err, IsNil)
	lp, err = keeper.GetLiquidityProvider(ctx, common.BTCAsset, btcAddress1)
	c.Assert(err, IsNil)
	c.Assert(lp.Valid(), IsNil)
	c.Assert(lp.PendingCacao.IsZero(), Equals, true)
	c.Assert(lp.PendingAsset.IsZero(), Equals, true)
	c.Assert(lp.Units.IsZero(), Equals, false)

	// let's withdraw the BTC we just staked
	msgWithdraw = NewMsgWithdrawLiquidity(GetRandomTx(), btcAddress1, cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), common.BTCAsset, common.EmptyAsset, activeNodeAccount.NodeAddress)
	_, err = withdrawHandler.Run(ctx, msgWithdraw)
	c.Assert(err, IsNil)
	lp, err = keeper.GetLiquidityProvider(ctx, common.BTCAsset, btcAddress1)
	c.Assert(err, IsNil)
	c.Assert(lp.Valid(), NotNil)

	// stake some BTC only as pending
	baseAddress := GetRandomBaseAddress()
	btcAddress1 = GetRandomBTCAddress()
	err = addHandler.addLiquidity(ctx, common.BTCAsset, cosmos.ZeroUint(), cosmos.NewUint(50*common.One),
		baseAddress, btcAddress1, GetRandomTx(), true, constAccessor, 0)
	c.Assert(err, IsNil)
	lp, err = keeper.GetLiquidityProvider(ctx, common.BTCAsset, baseAddress)
	c.Assert(err, IsNil)
	c.Assert(lp.PendingCacao.IsZero(), Equals, true)
	c.Assert(lp.PendingAsset.Uint64(), Equals, uint64(common.One*50))
	c.Assert(lp.Units.IsZero(), Equals, true)

	// sym lp withrawing asymmetrically
	tx = GetRandomTx()
	tx.FromAddress = btcAddress1
	tx.Chain = common.BTCChain
	msgWithdraw = NewMsgWithdrawLiquidity(tx, baseAddress, cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), common.BTCAsset, common.BTCAsset, activeNodeAccount.NodeAddress)
	_, err = withdrawHandler.Run(ctx, msgWithdraw)
	c.Assert(err, IsNil)
	lp, err = keeper.GetLiquidityProvider(ctx, common.BTCAsset, btcAddress1)
	c.Assert(err, IsNil)
	c.Assert(lp.Valid(), NotNil)

	// Bad version should fail
	_, err = withdrawHandler.Run(ctx, msgWithdraw)
	c.Assert(err, NotNil)
}

func (HandlerWithdrawSuite) TestWithdrawHandler_Validation(c *C) {
	ctx, k := setupKeeperForTest(c)
	testCases := []struct {
		name           string
		msg            *MsgWithdrawLiquidity
		expectedResult error
	}{
		{
			name:           "empty signer should fail",
			msg:            NewMsgWithdrawLiquidity(GetRandomTx(), GetRandomBaseAddress(), cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), common.BNBAsset, common.EmptyAsset, cosmos.AccAddress{}),
			expectedResult: errWithdrawFailValidation,
		},
		{
			name:           "empty asset should fail",
			msg:            NewMsgWithdrawLiquidity(GetRandomTx(), GetRandomBaseAddress(), cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), common.Asset{}, common.EmptyAsset, GetRandomValidatorNode(NodeActive).NodeAddress),
			expectedResult: errWithdrawFailValidation,
		},
		{
			name:           "empty RUNE address should fail",
			msg:            NewMsgWithdrawLiquidity(GetRandomTx(), common.NoAddress, cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), common.BNBAsset, common.EmptyAsset, GetRandomValidatorNode(NodeActive).NodeAddress),
			expectedResult: errWithdrawFailValidation,
		},
		{
			name:           "withdraw basis point is 0 should fail",
			msg:            NewMsgWithdrawLiquidity(GetRandomTx(), GetRandomBaseAddress(), cosmos.ZeroUint(), common.BNBAsset, common.EmptyAsset, GetRandomValidatorNode(NodeActive).NodeAddress),
			expectedResult: errWithdrawFailValidation,
		},
		{
			name:           "withdraw basis point is larger than 10000 should fail",
			msg:            NewMsgWithdrawLiquidity(GetRandomTx(), GetRandomBaseAddress(), cosmos.NewUint(uint64(MaxWithdrawBasisPoints+100)), common.BNBAsset, common.EmptyAsset, GetRandomValidatorNode(NodeActive).NodeAddress),
			expectedResult: errWithdrawFailValidation,
		},
		{
			name: "withdraw from external asset with maya address mismatch should fail",
			msg: func(ctx cosmos.Context, k keeper.Keeper) *MsgWithdrawLiquidity {
				lp := LiquidityProvider{
					Asset:        common.BNBAsset,
					CacaoAddress: GetRandomBaseAddress(),
					AssetAddress: GetRandomBNBAddress(),
				}

				tx := GetRandomTx()
				tx.Chain = common.BNBChain
				tx.FromAddress = lp.AssetAddress

				k.SetLiquidityProvider(ctx, lp)
				lp, _ = k.GetLiquidityProvider(ctx, common.BNBAsset, lp.CacaoAddress)
				return NewMsgWithdrawLiquidity(tx, GetRandomBaseAddress(), cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), common.BNBAsset, common.EmptyAsset, GetRandomValidatorNode(NodeActive).NodeAddress)
			}(ctx, k),
			expectedResult: errWithdrawLiquidityMismatchAddr,
		},
		{
			name: "withdraw from external asset with asset address mismatch should fail",
			msg: func(ctx cosmos.Context, k keeper.Keeper) *MsgWithdrawLiquidity {
				lp := LiquidityProvider{
					Asset:        common.BNBAsset,
					CacaoAddress: GetRandomBaseAddress(),
					AssetAddress: GetRandomBNBAddress(),
				}
				k.SetLiquidityProvider(ctx, lp)

				tx := GetRandomTx()
				tx.Chain = common.BNBChain
				tx.FromAddress = GetRandomBNBAddress()

				return NewMsgWithdrawLiquidity(tx, lp.CacaoAddress, cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), common.BNBAsset, common.EmptyAsset, GetRandomValidatorNode(NodeActive).NodeAddress)
			}(ctx, k),
			expectedResult: errWithdrawLiquidityMismatchAddr,
		},
	}
	for _, tc := range testCases {
		withdrawHandler := NewWithdrawLiquidityHandler(NewDummyMgrWithKeeper(k))
		_, err := withdrawHandler.Run(ctx, tc.msg)
		c.Assert(err.Error(), Equals, tc.expectedResult.Error(), Commentf(tc.name))
	}
}

func (HandlerWithdrawSuite) TestWithdrawHandler_mockFailScenarios(c *C) {
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	ctx, k := setupKeeperForTest(c)
	currentPool := Pool{
		BalanceCacao: cosmos.ZeroUint(),
		BalanceAsset: cosmos.ZeroUint(),
		Asset:        common.BNBAsset,
		LPUnits:      cosmos.ZeroUint(),
		Status:       PoolAvailable,
	}
	lp := LiquidityProvider{
		Units:        cosmos.ZeroUint(),
		PendingCacao: cosmos.ZeroUint(),
		PendingAsset: cosmos.ZeroUint(),
	}
	testCases := []struct {
		name           string
		k              keeper.Keeper
		expectedResult error
	}{
		{
			name: "fail to get pool withdraw should fail",
			k: &MockWithdrawKeeper{
				activeNodeAccount: activeNodeAccount,
				failPool:          true,
				lp:                lp,
				keeper:            k,
			},
			expectedResult: errInternal,
		},
		{
			name: "suspended pool withdraw should fail",
			k: &MockWithdrawKeeper{
				activeNodeAccount: activeNodeAccount,
				suspendedPool:     true,
				lp:                lp,
				keeper:            k,
			},
			expectedResult: errInvalidPoolStatus,
		},
		{
			name: "fail to get liquidity provider withdraw should fail",
			k: &MockWithdrawKeeper{
				activeNodeAccount:     activeNodeAccount,
				currentPool:           currentPool,
				failLiquidityProvider: true,
				lp:                    lp,
				keeper:                k,
			},
			expectedResult: errFailGetLiquidityProvider,
		},
		{
			name: "fail to add incomplete event withdraw should fail",
			k: &MockWithdrawKeeper{
				activeNodeAccount: activeNodeAccount,
				currentPool:       currentPool,
				failAddEvents:     true,
				lp:                lp,
				keeper:            k,
			},
			expectedResult: errInternal,
		},
	}

	for _, tc := range testCases {
		withdrawHandler := NewWithdrawLiquidityHandler(NewDummyMgrWithKeeper(tc.k))
		tx := GetRandomTx()
		tx.Chain = common.BASEChain
		msgWithdraw := NewMsgWithdrawLiquidity(tx, GetRandomBaseAddress(), cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), common.BNBAsset, common.EmptyAsset, activeNodeAccount.NodeAddress)
		_, err := withdrawHandler.Run(ctx, msgWithdraw)
		c.Assert(errors.Is(err, tc.expectedResult), Equals, true, Commentf(tc.name))
	}
}

type MockWithdrawTxOutStore struct {
	TxOutStore
	errAsset error
	errRune  error
}

func (store *MockWithdrawTxOutStore) TryAddTxOutItem(ctx cosmos.Context, mgr Manager, toi TxOutItem, _ cosmos.Uint) (bool, error) {
	if toi.Coin.Asset.IsNativeBase() && store.errRune != nil {
		return false, store.errRune
	}
	if !toi.Coin.Asset.IsNativeBase() && store.errAsset != nil {
		return false, store.errAsset
	}
	return true, nil
}

type MockWithdrawEventMgr struct {
	EventManager
	count int
}

func (m *MockWithdrawEventMgr) EmitEvent(ctx cosmos.Context, evt EmitEventItem) error {
	m.count++
	return nil
}

func (HandlerWithdrawSuite) TestWithdrawHandler_outboundFailures(c *C) {
	SetupConfigForTest()
	ctx, keeper := setupKeeperForTest(c)
	na := GetRandomValidatorNode(NodeActive)
	asset := common.BTCAsset

	pool := Pool{
		Asset:               asset,
		BalanceAsset:        cosmos.NewUint(10000),
		BalanceCacao:        cosmos.NewUint(10000),
		LPUnits:             cosmos.NewUint(1000),
		SynthUnits:          cosmos.ZeroUint(),
		PendingInboundCacao: cosmos.ZeroUint(),
		PendingInboundAsset: cosmos.ZeroUint(),
		Status:              PoolAvailable,
	}
	c.Assert(pool.Valid(), IsNil)
	_ = keeper.SetPool(ctx, pool)

	runeAddr := GetRandomBaseAddress()
	lp := LiquidityProvider{
		Asset:              asset,
		LastAddHeight:      ctx.BlockHeight(),
		CacaoAddress:       runeAddr,
		AssetAddress:       GetRandomBTCAddress(),
		Units:              cosmos.NewUint(100),
		LastWithdrawHeight: ctx.BlockHeight(),
	}
	c.Assert(lp.Valid(), IsNil)
	keeper.SetLiquidityProvider(ctx, lp)

	tx := GetRandomTx()
	tx.Chain = common.BASEChain
	msg := NewMsgWithdrawLiquidity(
		tx,
		lp.CacaoAddress,
		cosmos.NewUint(1000),
		asset,
		common.BaseAsset(),
		na.NodeAddress)

	c.Assert(msg.ValidateBasic(), IsNil)

	mgr := NewDummyMgrWithKeeper(keeper)

	// runs the handler and checks pool/lp state for changes
	handleCase := func(msg *MsgWithdrawLiquidity, errRune, errAsset error, name string, shouldFail bool) {
		_ = keeper.SetPool(ctx, pool)
		keeper.SetLiquidityProvider(ctx, lp)
		mgr.txOutStore = &MockWithdrawTxOutStore{
			TxOutStore: mgr.txOutStore,
			errRune:    errRune,
			errAsset:   errAsset,
		}
		eventMgr := &MockWithdrawEventMgr{
			EventManager: mgr.eventMgr,
			count:        0,
		}
		mgr.eventMgr = eventMgr
		handler := NewWithdrawLiquidityHandler(mgr)
		_, err := handler.Run(ctx, msg)
		lpAfter, _ := keeper.GetLiquidityProvider(ctx, asset, runeAddr)
		poolAfter, _ := keeper.GetPool(ctx, asset)
		if shouldFail {
			// should error
			c.Assert(err, NotNil, Commentf(name))
		} else {
			// should not error and pool/lp  should be modified
			c.Assert(err, IsNil, Commentf(name))
			c.Assert(lpAfter.String(), Not(Equals), lp.String(), Commentf(name))
			c.Assert(poolAfter.String(), Not(Equals), pool.String(), Commentf(name))
			c.Assert(eventMgr.count, Equals, 1, Commentf(name))
		}
	}

	msg.WithdrawalAsset = common.BaseAsset()
	handleCase(msg, errInternal, nil, "asym rune fail", true)

	msg.WithdrawalAsset = common.BTCAsset
	handleCase(msg, nil, errInternal, "asym asset fail", true)

	msg.WithdrawalAsset = common.EmptyAsset
	handleCase(msg, errInternal, nil, "sym rune fail/asset success", true)
	handleCase(msg, nil, errInternal, "sym rune success/asset fail", true)
	handleCase(msg, errInternal, errInternal, "sym rune/asset fail", true)
	handleCase(msg, nil, nil, "sym rune/asset success", false)
}

func (s *HandlerWithdrawSuite) TestWithdrawLiquidityHandlerWithSwap(c *C) {
	var err error
	ctx, mgr := setupManagerForTest(c)

	mgr.txOutStore = NewTxStoreDummy()
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	runeAddr := GetRandomBaseAddress()
	bnbAddr := GetRandomBNBAddress()
	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceCacao = cosmos.NewUint(219911755050746)
	pool.BalanceAsset = cosmos.NewUint(2189430478930)
	pool.LPUnits = cosmos.NewUint(104756821848147)
	pool.Status = PoolAvailable
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)
	pool.Asset = common.BTCAsset
	pool.BalanceCacao = cosmos.NewUint(929514035216049)
	pool.BalanceAsset = cosmos.NewUint(89025872745)
	pool.LPUnits = cosmos.NewUint(530237037742827)
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)

	// happy path
	addHandler := NewAddLiquidityHandler(mgr)
	btcAddr := GetRandomBTCAddress()
	tx := common.NewTx(
		GetRandomTxHash(),
		btcAddr,
		runeAddr,
		common.Coins{common.NewCoin(common.BTCAsset, cosmos.NewUint(common.One*5))},
		BNBGasFeeSingleton,
		"add:BNB.BNB",
	)
	msg := NewMsgAddLiquidity(
		tx,
		common.BNBAsset,
		cosmos.NewUint(100*common.One),
		cosmos.ZeroUint(),
		runeAddr,
		bnbAddr,
		common.NoAddress, cosmos.ZeroUint(),
		activeNodeAccount.NodeAddress, 1)
	err = addHandler.handle(ctx, *msg)
	c.Assert(err, IsNil)

	c.Assert(mgr.SwapQ().EndBlock(ctx, mgr), IsNil)

	pool, err = mgr.Keeper().GetPool(ctx, common.BTCAsset)
	c.Assert(err, IsNil)
	c.Check(pool.BalanceCacao.Uint64(), Equals, uint64(924351713466224), Commentf("%d", pool.BalanceCacao.Uint64()))
	c.Check(pool.BalanceAsset.Uint64(), Equals, uint64(89525872745), Commentf("%d", pool.BalanceAsset.Uint64()))

	pool, err = mgr.Keeper().GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Check(pool.BalanceCacao.Uint64(), Equals, uint64(225074076800571), Commentf("%d", pool.BalanceCacao.Uint64()))
	c.Check(pool.BalanceAsset.Uint64(), Equals, uint64(2189430478930), Commentf("%d", pool.BalanceAsset.Uint64()))

	lp, err := mgr.Keeper().GetLiquidityProvider(ctx, common.BNBAsset, btcAddr)
	c.Assert(err, IsNil)
	c.Check(lp.Units.IsZero(), Equals, false)
	c.Check(lp.Units.Uint64(), Equals, uint64(1173802086792), Commentf("%d", lp.Units.Uint64()))

	// nothing in the outbound queue
	outbound, err := mgr.txOutStore.GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(outbound, HasLen, 0)

	withdrawHandler := NewWithdrawLiquidityHandler(mgr)

	msgWithdraw := NewMsgWithdrawLiquidity(GetRandomTx(), btcAddr, cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), common.BNBAsset, common.EmptyAsset, activeNodeAccount.NodeAddress)
	_, err = withdrawHandler.Run(ctx, msgWithdraw)
	c.Assert(err, IsNil)

	c.Assert(mgr.SwapQ().EndBlock(ctx, mgr), IsNil)

	outbound, err = mgr.txOutStore.GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(outbound, HasLen, 1)

	expected := common.NewCoin(common.BTCAsset, cosmos.NewUint(462467571))
	c.Check(outbound[0].Coin.Equals(expected), Equals, true, Commentf("%s", outbound[0].Coin))
}

func (s *HandlerWithdrawSuite) TestWithdrawLiquidityHandlerFromGenesisNode(c *C) {
	var err error
	ctx, mgr := setupManagerForTest(c)
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	bp := NewBondProviders(activeNodeAccount.NodeAddress)
	bp.Providers = append(bp.Providers, NewBondProvider(activeNodeAccount.NodeAddress))
	bp.Providers[0].Bonded = true

	asgard := GetRandomVault()
	asgard.Membership = []string{activeNodeAccount.PubKeySet.Secp256k1.String()}
	c.Assert(mgr.Keeper().SetVault(ctx, asgard), IsNil)

	c.Assert(mgr.Keeper().SetBondProviders(ctx, bp), IsNil)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, activeNodeAccount), IsNil)

	pool := NewPool()
	pool.Asset = common.BTCAsset
	pool.BalanceCacao = cosmos.NewUint(929514035216049)
	pool.BalanceAsset = cosmos.NewUint(89025872745)
	pool.LPUnits = cosmos.NewUint(530237037742827)
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)

	add, err := common.NewAddress(GenesisNodes[0], GetCurrentVersion())
	c.Assert(err, IsNil)
	genesisNode := LiquidityProvider{
		CacaoAddress:      add,
		AssetAddress:      common.NoAddress,
		Asset:             common.BTCAsset,
		CacaoDepositValue: cosmos.NewUint(100000 * common.One),
		AssetDepositValue: cosmos.ZeroUint(),
		Units:             cosmos.NewUint(100000 * common.One),
	}
	mgr.Keeper().SetLiquidityProvider(ctx, genesisNode)

	withdrawHandler := NewWithdrawLiquidityHandler(mgr)
	reserveBalBefore := mgr.Keeper().GetRuneBalanceOfModule(ctx, ReserveName)

	tx := GetRandomTx()
	tx.FromAddress = genesisNode.CacaoAddress
	tx.Chain = common.BASEChain
	msgWithdraw := NewMsgWithdrawLiquidity(tx, genesisNode.CacaoAddress, cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), common.BTCAsset, common.BaseAsset(), activeNodeAccount.NodeAddress)
	_, err = withdrawHandler.Run(ctx, msgWithdraw)
	c.Assert(err, IsNil)

	reserveBalAfter := mgr.Keeper().GetRuneBalanceOfModule(ctx, ReserveName)

	expected := common.NewCoin(common.BaseAsset(), cosmos.NewUint(34405336284411))
	c.Check(reserveBalAfter.Sub(reserveBalBefore).Uint64(), Equals, expected.Amount.Uint64(), Commentf("%d", reserveBalAfter.Sub(reserveBalBefore).Uint64()))
}

func (HandlerObservedTxInSuite) TestWithdrawFromDisabledSavers(c *C) {
	ctx, mgr := setupManagerForTest(c)

	asset := common.BTCAsset.GetSyntheticAsset()

	addHandler := NewAddLiquidityHandler(mgr)
	withdrawHandler := NewWithdrawLiquidityHandler(mgr)

	addr := GetRandomBTCAddress()
	signer := GetRandomBech32Addr()
	addTxHash := GetRandomTxHash()

	tx := common.NewTx(
		addTxHash,
		addr,
		addr,
		common.Coins{common.NewCoin(common.BTCAsset, cosmos.NewUint(1000*common.One))},
		BNBGasFeeSingleton,
		fmt.Sprintf("add:%s", asset.String()),
	)

	// BTC.BTC pool
	c.Assert(mgr.Keeper().SetPool(ctx, Pool{
		BalanceCacao: cosmos.NewUint(100000 * common.One),
		BalanceAsset: cosmos.NewUint(10000 * common.One),
		Asset:        common.BTCAsset,
		LPUnits:      cosmos.ZeroUint(),
		SynthUnits:   cosmos.ZeroUint(),
		Status:       PoolAvailable,
	}), IsNil)

	// BTC/BTC pool
	pool := NewPool()
	pool.Asset = asset
	pool.Status = PoolAvailable
	pool.BalanceCacao = cosmos.NewUint(0)
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.LPUnits = cosmos.NewUint(100)
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)

	coin := common.NewCoin(asset, pool.BalanceAsset)
	c.Assert(mgr.Keeper().MintToModule(ctx, ModuleName, coin), IsNil)
	c.Assert(mgr.Keeper().SendFromModuleToModule(ctx, ModuleName, AsgardName, common.NewCoins(coin)), IsNil)

	//	constAccessor := constants.GetConstantValues(GetCurrentVersion())
	//	h := NewAddLiquidityHandler(NewDummyMgrWithKeeper(mgr.Keeper()))
	addCoin := common.NewCoin(asset, cosmos.NewUint(100*common.One))
	c.Assert(mgr.Keeper().MintToModule(ctx, ModuleName, addCoin), IsNil)
	c.Assert(mgr.Keeper().SendFromModuleToModule(ctx, ModuleName, AsgardName, common.NewCoins(addCoin)), IsNil)
	//	err := h.addLiquidity(ctx, asset, cosmos.ZeroUint(), addCoin.Amount, common.NoAddress, addr, tx, false, constAccessor, 0)
	//	c.Assert(err, IsNil)

	// happy path add
	msg := NewMsgAddLiquidity(tx, asset, cosmos.ZeroUint(), cosmos.NewUint(1000*common.One), common.NoAddress, addr, common.NoAddress, cosmos.ZeroUint(), signer, 1)
	_, err := addHandler.Run(ctx, msg)
	c.Assert(err, IsNil)

	// withdraw should be ok
	tx.ID = GetRandomTxHash()
	msgWithdraw := NewMsgWithdrawLiquidity(tx, addr, cosmos.NewUint(uint64(MaxWithdrawBasisPoints)/2), common.BTCAsset, common.BaseAsset(), signer)
	err = withdrawHandler.validate(ctx, *msgWithdraw)
	c.Assert(err, IsNil)

	mgr.Keeper().SetMimir(ctx, "SaverWithdrawalDisabled", 1)

	// withdraw the rest, but withdrawals are disabled
	tx.ID = GetRandomTxHash()
	msgWithdraw = NewMsgWithdrawLiquidity(tx, addr, cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), asset, common.BaseAsset(), signer)
	_, err = withdrawHandler.Run(ctx, msgWithdraw)
	c.Assert(err, NotNil)
	c.Assert(strings.Contains(err.Error(), "saver withdrawals are disabled"), Equals, true)

	// normal add liquidity works fine
	asset = common.BTCAsset
	msg = NewMsgAddLiquidity(tx, asset, cosmos.ZeroUint(), cosmos.NewUint(1000*common.One), common.NoAddress, addr, common.NoAddress, cosmos.ZeroUint(), signer, 1)
	err = addHandler.validate(ctx, *msg)
	c.Assert(err, IsNil)
	// normal withdraw liquidity works fine even when SaverWithdrawalDisabled is set
	asset = common.BTCAsset
	tx.Coins = common.Coins{common.NewCoin(asset, cosmos.NewUint(1000*common.One))}
	msgWithdraw = NewMsgWithdrawLiquidity(tx, addr, cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), common.BTCAsset, common.BaseAsset(), signer)
	err = withdrawHandler.validate(ctx, *msgWithdraw)
	c.Assert(err, IsNil)
}
