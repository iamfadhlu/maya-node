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

type HandlerAddLiquiditySuite struct{}

var _ = Suite(&HandlerAddLiquiditySuite{})

type MockAddLiquidityKeeper struct {
	keeper.KVStoreDummy
	currentPool           Pool
	activeNodeAccount     NodeAccount
	activeNodeAccountBond cosmos.Uint
	failGetPool           bool
	lp                    LiquidityProvider
	pol                   ProtocolOwnedLiquidity
	polAddress            common.Address
	tier                  int64
	mimir                 map[string]int64
}

func (m *MockAddLiquidityKeeper) PoolExist(_ cosmos.Context, asset common.Asset) bool {
	return m.currentPool.Asset.Equals(asset)
}

func (m *MockAddLiquidityKeeper) GetPools(_ cosmos.Context) (Pools, error) {
	return Pools{m.currentPool}, nil
}

func (m *MockAddLiquidityKeeper) GetPool(_ cosmos.Context, _ common.Asset) (Pool, error) {
	if m.failGetPool {
		return Pool{}, errors.New("fail to get pool")
	}
	return m.currentPool, nil
}

func (m *MockAddLiquidityKeeper) SetPool(_ cosmos.Context, pool Pool) error {
	m.currentPool = pool
	return nil
}

func (m *MockAddLiquidityKeeper) GetModuleAddress(mod string) (common.Address, error) {
	return m.polAddress, nil
}

func (m *MockAddLiquidityKeeper) GetPOL(_ cosmos.Context) (ProtocolOwnedLiquidity, error) {
	return m.pol, nil
}

func (m *MockAddLiquidityKeeper) SetPOL(_ cosmos.Context, pol ProtocolOwnedLiquidity) error {
	m.pol = pol
	return nil
}

func (m *MockAddLiquidityKeeper) ListValidatorsWithBond(_ cosmos.Context) (NodeAccounts, error) {
	return NodeAccounts{m.activeNodeAccount}, nil
}

func (m *MockAddLiquidityKeeper) ListActiveValidators(_ cosmos.Context) (NodeAccounts, error) {
	return NodeAccounts{m.activeNodeAccount}, nil
}

func (m *MockAddLiquidityKeeper) GetNodeAccount(_ cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error) {
	if m.activeNodeAccount.NodeAddress.Equals(addr) {
		return m.activeNodeAccount, nil
	}
	return NodeAccount{}, errors.New("not exist")
}

func (m *MockAddLiquidityKeeper) GetLiquidityProvider(_ cosmos.Context, asset common.Asset, addr common.Address) (LiquidityProvider, error) {
	return m.lp, nil
}

func (m *MockAddLiquidityKeeper) GetLiquidityProviderByAssets(_ cosmos.Context, assets common.Assets, addr common.Address) (LiquidityProviders, error) {
	return []LiquidityProvider{m.lp}, nil
}

func (m *MockAddLiquidityKeeper) GetLiquidityAuctionTier(_ cosmos.Context, _ common.Address) (int64, error) {
	return m.tier, nil
}

func (m *MockAddLiquidityKeeper) SetLiquidityAuctionTier(_ cosmos.Context, _ common.Address, tier int64) error {
	m.tier = tier
	return nil
}

func (m *MockAddLiquidityKeeper) SetLiquidityProvider(ctx cosmos.Context, lp LiquidityProvider) {
	m.lp = lp
}

func (m *MockAddLiquidityKeeper) AddOwnership(ctx cosmos.Context, coin common.Coin, _ cosmos.AccAddress) error {
	m.lp.Units = m.lp.Units.Add(coin.Amount)
	return nil
}

func (m *MockAddLiquidityKeeper) CalcNodeLiquidityBond(ctx cosmos.Context, na NodeAccount) (cosmos.Uint, error) {
	return m.activeNodeAccountBond, nil
}

func (m *MockAddLiquidityKeeper) GetLiquidityProviderIterator(ctx cosmos.Context, _ common.Asset) cosmos.Iterator {
	iter := keeper.NewDummyIterator()
	iter.AddItem([]byte("key"), m.Cdc().MustMarshal(&m.lp))
	return iter
}

func (m *MockAddLiquidityKeeper) SetMimir(ctx cosmos.Context, key string, value int64) {
	m.mimir[key] = value
}

func (m *MockAddLiquidityKeeper) GetMimir(ctx cosmos.Context, key string) (int64, error) {
	v, ok := m.mimir[key]
	if !ok {
		return -1, nil
	}
	return v, nil
}

type MockConstant struct {
	constants.DummyConstants
}

func (s *HandlerAddLiquiditySuite) SetUpSuite(c *C) {
	SetupConfigForTest()
}

func (s *HandlerAddLiquiditySuite) TestAddLiquidityHandler(c *C) {
	var err error
	ctx, mgr := setupManagerForTest(c)
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	runeAddr := GetRandomBaseAddress()
	bnbAddr := GetRandomBNBAddress()
	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.Status = PoolAvailable
	k := &MockAddLiquidityKeeper{
		activeNodeAccount:     activeNodeAccount,
		activeNodeAccountBond: cosmos.NewUint(100 * common.One),
		currentPool:           pool,
		lp: LiquidityProvider{
			Asset:             common.BNBAsset,
			CacaoAddress:      runeAddr,
			AssetAddress:      bnbAddr,
			Units:             cosmos.ZeroUint(),
			PendingCacao:      cosmos.ZeroUint(),
			PendingAsset:      cosmos.ZeroUint(),
			CacaoDepositValue: cosmos.ZeroUint(),
			AssetDepositValue: cosmos.ZeroUint(),
		},
		pol:        NewProtocolOwnedLiquidity(),
		polAddress: runeAddr,
		mimir: map[string]int64{
			constants.LiquidityAuction.String(): 21 * 14400,
		},
		tier: 3,
	}
	mgr.K = k
	// happy path
	addHandler := NewAddLiquidityHandler(mgr)
	addTx := GetRandomTx()
	tx := common.NewTx(
		addTx.ID,
		runeAddr,
		GetRandomBNBAddress(),
		common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One*5))},
		BNBGasFeeSingleton,
		"add:BNB",
	)
	msg := NewMsgAddLiquidity(
		tx,
		common.BNBAsset,
		cosmos.ZeroUint(),
		cosmos.NewUint(100*common.One),
		runeAddr,
		bnbAddr,
		common.NoAddress, cosmos.ZeroUint(),
		activeNodeAccount.NodeAddress, 2)
	_, err = addHandler.Run(ctx, msg)
	c.Assert(err, IsNil)

	midLiquidityPool, err := mgr.Keeper().GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Assert(midLiquidityPool.PendingInboundAsset.String(), Equals, "10000000000")
	c.Assert(k.tier, Equals, int64(2))

	msg.AssetAmount = cosmos.ZeroUint()
	msg.CacaoAmount = cosmos.NewUint(100 * common.One)
	msg.LiquidityAuctionTier = 1
	_, err = addHandler.Run(ctx, msg)
	c.Assert(err, IsNil)

	postLiquidityPool, err := mgr.Keeper().GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Assert(postLiquidityPool.BalanceAsset.String(), Equals, "10000000000")
	c.Assert(postLiquidityPool.BalanceCacao.String(), Equals, "10000000000")
	c.Assert(postLiquidityPool.PendingInboundAsset.String(), Equals, "0")
	c.Assert(postLiquidityPool.PendingInboundCacao.String(), Equals, "0")
	c.Assert(k.tier, Equals, int64(2))

	// test liquidity auction tier should be changed
	k.tier = 2
	k.lp.Units = cosmos.ZeroUint()
	k.lp.CacaoDepositValue = cosmos.ZeroUint()
	k.lp.AssetDepositValue = cosmos.ZeroUint()
	msg.LiquidityAuctionTier = 1
	msg.CacaoAmount = cosmos.ZeroUint()
	msg.AssetAmount = cosmos.NewUint(100 * common.One)

	_, err = addHandler.Run(ctx, msg)
	c.Assert(err, IsNil)
	pol, err := mgr.Keeper().GetPOL(ctx)
	c.Assert(err, IsNil)
	midLiquidityPool, err = mgr.Keeper().GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Assert(midLiquidityPool.PendingInboundAsset.String(), Equals, "10000000000")
	c.Assert(k.tier, Equals, int64(1))

	// test liquidity auction tier not set when not in la
	k.mimir[constants.LiquidityAuction.String()] = 0
	k.tier = 0
	k.lp.Units = cosmos.ZeroUint()
	k.lp.CacaoDepositValue = cosmos.ZeroUint()
	k.lp.AssetDepositValue = cosmos.ZeroUint()
	msg.LiquidityAuctionTier = 0
	msg.CacaoAmount = cosmos.ZeroUint()
	msg.AssetAmount = cosmos.NewUint(100 * common.One)

	_, err = addHandler.Run(ctx, msg)
	c.Assert(err, IsNil)
	midLiquidityPool, err = mgr.Keeper().GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Check(pol.CacaoDeposited.Uint64(), Equals, uint64(100*common.One))
	c.Assert(midLiquidityPool.PendingInboundAsset.String(), Equals, "20000000000")
	c.Assert(k.tier, Equals, int64(0))
}

func (s *HandlerAddLiquiditySuite) TestAddLiquidityHandler_NoPool_ShouldCreateNewPool(c *C) {
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	runeAddr := GetRandomBaseAddress()
	bnbAddr := GetRandomBNBAddress()
	pool := NewPool()
	pool.Status = PoolAvailable
	k := &MockAddLiquidityKeeper{
		activeNodeAccount:     activeNodeAccount,
		activeNodeAccountBond: cosmos.NewUint(1000000 * common.One),
		currentPool:           pool,
		lp: LiquidityProvider{
			Asset:             common.BNBAsset,
			CacaoAddress:      runeAddr,
			AssetAddress:      bnbAddr,
			Units:             cosmos.ZeroUint(),
			PendingCacao:      cosmos.ZeroUint(),
			PendingAsset:      cosmos.ZeroUint(),
			CacaoDepositValue: cosmos.ZeroUint(),
			AssetDepositValue: cosmos.ZeroUint(),
		},
	}
	// happy path
	ctx, mgr := setupManagerForTest(c)
	mgr.K = k
	addHandler := NewAddLiquidityHandler(mgr)
	preLiquidityPool, err := k.GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Assert(preLiquidityPool.IsEmpty(), Equals, true)
	addTx := GetRandomTx()
	tx := common.NewTx(
		addTx.ID,
		runeAddr,
		GetRandomBNBAddress(),
		common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One*5))},
		BNBGasFeeSingleton,
		"add:BNB",
	)
	mgr.constAccessor = constants.NewDummyConstants(map[constants.ConstantName]int64{
		constants.MaximumLiquidityCacao: 600_000_00000000,
	}, map[constants.ConstantName]bool{
		constants.StrictBondLiquidityRatio: false,
	}, map[constants.ConstantName]string{})

	msg := NewMsgAddLiquidity(
		tx,
		common.BNBAsset,
		cosmos.NewUint(100*common.One),
		cosmos.NewUint(100*common.One),
		runeAddr,
		bnbAddr,
		common.NoAddress, cosmos.ZeroUint(),
		activeNodeAccount.NodeAddress, 1)
	_, err = addHandler.Run(ctx, msg)
	c.Assert(err, IsNil)
	postLiquidityPool, err := k.GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Assert(postLiquidityPool.BalanceAsset.String(), Equals, preLiquidityPool.BalanceAsset.Add(msg.AssetAmount).String())
	c.Assert(postLiquidityPool.BalanceCacao.String(), Equals, preLiquidityPool.BalanceCacao.Add(msg.CacaoAmount).String())
}

func (s *HandlerAddLiquiditySuite) TestAddLiquidityHandlerValidation(c *C) {
	ctx, _ := setupKeeperForTest(c)
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	runeAddr := GetRandomBaseAddress()
	bnbAddr := GetRandomBNBAddress()
	bnbSynthAsset, _ := common.NewAsset("BNB/BNB")
	tx := common.NewTx(
		GetRandomTxHash(),
		GetRandomBaseAddress(),
		GetRandomBaseAddress(),
		common.Coins{common.NewCoin(bnbSynthAsset, cosmos.NewUint(common.One*5))},
		common.Gas{
			{Asset: common.BaseNative, Amount: cosmos.NewUint(1 * common.One)},
		},
		"add:BNB.BNB",
	)

	k := &MockAddLiquidityKeeper{
		activeNodeAccount:     activeNodeAccount,
		activeNodeAccountBond: cosmos.NewUint(1000 * common.One),
		currentPool: Pool{
			BalanceCacao: cosmos.ZeroUint(),
			BalanceAsset: cosmos.ZeroUint(),
			Asset:        common.BNBAsset,
			LPUnits:      cosmos.ZeroUint(),
			SynthUnits:   cosmos.ZeroUint(),
			Status:       PoolAvailable,
		},
		lp: LiquidityProvider{
			Asset:             common.BNBAsset,
			CacaoAddress:      runeAddr,
			AssetAddress:      bnbAddr,
			Units:             cosmos.ZeroUint(),
			PendingCacao:      cosmos.ZeroUint(),
			PendingAsset:      cosmos.ZeroUint(),
			CacaoDepositValue: cosmos.NewUint(common.One * 1001),
			AssetDepositValue: cosmos.ZeroUint(),
		},
		mimir: make(map[string]int64),
	}
	testCases := []struct {
		name           string
		msg            *MsgAddLiquidity
		expectedResult error
	}{
		{
			name:           "empty signer should fail",
			msg:            NewMsgAddLiquidity(GetRandomTx(), common.BNBAsset, cosmos.NewUint(common.One*5), cosmos.NewUint(common.One*5), GetRandomBNBAddress(), GetRandomBNBAddress(), common.NoAddress, cosmos.ZeroUint(), cosmos.AccAddress{}, 1),
			expectedResult: errAddLiquidityFailValidation,
		},
		{
			name:           "empty asset should fail",
			msg:            NewMsgAddLiquidity(GetRandomTx(), common.Asset{}, cosmos.NewUint(common.One*5), cosmos.NewUint(common.One*5), GetRandomBNBAddress(), GetRandomBNBAddress(), common.NoAddress, cosmos.ZeroUint(), GetRandomValidatorNode(NodeActive).NodeAddress, 1),
			expectedResult: errAddLiquidityFailValidation,
		},
		{
			name:           "synth asset from coins should fail",
			msg:            NewMsgAddLiquidity(tx, common.BNBAsset, cosmos.NewUint(common.One*5), cosmos.NewUint(common.One*5), GetRandomBNBAddress(), GetRandomBNBAddress(), common.NoAddress, cosmos.ZeroUint(), GetRandomValidatorNode(NodeActive).NodeAddress, 1),
			expectedResult: errAddLiquidityFailValidation,
		},
		{
			name:           "empty addresses should fail",
			msg:            NewMsgAddLiquidity(GetRandomTx(), common.BTCAsset, cosmos.NewUint(common.One*5), cosmos.NewUint(common.One*5), common.NoAddress, common.NoAddress, common.NoAddress, cosmos.ZeroUint(), GetRandomValidatorNode(NodeActive).NodeAddress, 1),
			expectedResult: errAddLiquidityFailValidation,
		},
		{
			name:           "total liquidity provider is more than total bond should fail",
			msg:            NewMsgAddLiquidity(GetRandomTx(), common.BNBAsset, cosmos.NewUint(common.One*5000), cosmos.NewUint(common.One*5000), GetRandomBaseAddress(), GetRandomBNBAddress(), common.NoAddress, cosmos.ZeroUint(), activeNodeAccount.NodeAddress, 1),
			expectedResult: errAddLiquidityCACAOMoreThanBond,
		},
		{
			name:           "total liquidity more than bond should not fail when ensureLiquidityNoLargerThanBond active",
			msg:            NewMsgAddLiquidity(GetRandomTx(), common.BNBAsset, cosmos.NewUint(common.One*5000), cosmos.NewUint(common.One*5000), GetRandomBaseAddress(), GetRandomBNBAddress(), common.NoAddress, cosmos.ZeroUint(), activeNodeAccount.NodeAddress, 1),
			expectedResult: errAddLiquidityMismatchAddr,
		},
		{
			name:           "rune address with wrong chain should fail",
			msg:            NewMsgAddLiquidity(GetRandomTx(), common.BNBAsset, cosmos.NewUint(common.One*5), cosmos.NewUint(common.One*5), GetRandomBNBAddress(), GetRandomBaseAddress(), common.NoAddress, cosmos.ZeroUint(), GetRandomValidatorNode(NodeActive).NodeAddress, 1),
			expectedResult: errAddLiquidityFailValidation,
		},
	}
	constAccessor := constants.NewDummyConstants(map[constants.ConstantName]int64{
		constants.MaximumLiquidityCacao: 600_000_00000000,
	}, map[constants.ConstantName]bool{
		constants.StrictBondLiquidityRatio: true,
	}, map[constants.ConstantName]string{})

	for _, item := range testCases {
		mgr := NewDummyMgrWithKeeper(k)
		mgr.constAccessor = constAccessor
		if item.name == "total liquidity more than bond should not fail when ensureLiquidityNoLargerThanBond active" {
			mgr.K.SetMimir(ctx, "EnsureLiquidityNoLargerThanBond", 0)
		}
		addHandler := NewAddLiquidityHandler(mgr)
		_, err := addHandler.Run(ctx, item.msg)
		c.Assert(errors.Is(err, item.expectedResult), Equals, true, Commentf("name:%s, %w", item.name, err))
	}
}

func (s *HandlerAddLiquiditySuite) TestHandlerAddLiquidityFailScenario(c *C) {
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	emptyPool := Pool{
		BalanceCacao: cosmos.ZeroUint(),
		BalanceAsset: cosmos.ZeroUint(),
		Asset:        common.BNBAsset,
		LPUnits:      cosmos.ZeroUint(),
		Status:       PoolAvailable,
	}

	testCases := []struct {
		name           string
		k              keeper.Keeper
		expectedResult error
	}{
		{
			name: "fail to get pool should fail add liquidity",
			k: &MockAddLiquidityKeeper{
				activeNodeAccount:     activeNodeAccount,
				activeNodeAccountBond: cosmos.NewUint(1000000 * common.One),
				currentPool:           emptyPool,
				failGetPool:           true,
			},
			expectedResult: errInternal,
		},
		{
			name: "suspended pool should fail add liquidity",
			k: &MockAddLiquidityKeeper{
				activeNodeAccount:     activeNodeAccount,
				activeNodeAccountBond: cosmos.NewUint(1000000 * common.One),
				currentPool: Pool{
					BalanceCacao: cosmos.ZeroUint(),
					BalanceAsset: cosmos.ZeroUint(),
					Asset:        common.BNBAsset,
					LPUnits:      cosmos.ZeroUint(),
					Status:       PoolSuspended,
				},
			},
			expectedResult: errInvalidPoolStatus,
		},
	}
	for _, tc := range testCases {
		runeAddr := GetRandomBaseAddress()
		bnbAddr := GetRandomBNBAddress()
		addTx := GetRandomTx()
		tx := common.NewTx(
			addTx.ID,
			runeAddr,
			GetRandomBNBAddress(),
			common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One*5))},
			BNBGasFeeSingleton,
			"add:BNB",
		)
		msg := NewMsgAddLiquidity(
			tx,
			common.BNBAsset,
			cosmos.NewUint(100*common.One),
			cosmos.NewUint(100*common.One),
			runeAddr,
			bnbAddr,
			common.NoAddress, cosmos.ZeroUint(),
			activeNodeAccount.NodeAddress,
			1)
		ctx, mgr := setupManagerForTest(c)
		mgr.K = tc.k
		addHandler := NewAddLiquidityHandler(mgr)
		_, err := addHandler.Run(ctx, msg)
		c.Assert(errors.Is(err, tc.expectedResult), Equals, true, Commentf(tc.name))
	}
}

func (s *HandlerAddLiquiditySuite) TestAddLiquidityHandlerWithSwap(c *C) {
	var err error
	ctx, mgr := setupManagerForTest(c)

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
		tx, common.BNBAsset, cosmos.NewUint(100*common.One), cosmos.ZeroUint(), runeAddr, bnbAddr, common.NoAddress, cosmos.ZeroUint(), activeNodeAccount.NodeAddress, 1)
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
}

type AddLiquidityTestKeeper struct {
	keeper.KVStoreDummy
	store          map[string]interface{}
	liquidityUnits cosmos.Uint
}

// NewAddLiquidityTestKeeper
func NewAddLiquidityTestKeeper() *AddLiquidityTestKeeper {
	return &AddLiquidityTestKeeper{
		store:          make(map[string]interface{}),
		liquidityUnits: cosmos.ZeroUint(),
	}
}

func (p *AddLiquidityTestKeeper) PoolExist(ctx cosmos.Context, asset common.Asset) bool {
	_, ok := p.store[asset.String()]
	return ok
}

var notExistLiquidityProviderAsset, _ = common.NewAsset("BNB.NotExistLiquidityProviderAsset")

func (p *AddLiquidityTestKeeper) GetPool(ctx cosmos.Context, asset common.Asset) (Pool, error) {
	if p, ok := p.store[asset.String()]; ok {
		pool, ok := p.(Pool)
		if !ok {
			return pool, fmt.Errorf("dev error: failed to cast pool")
		}
		return pool, nil
	}
	return NewPool(), nil
}

func (p *AddLiquidityTestKeeper) SetPool(ctx cosmos.Context, ps Pool) error {
	p.store[ps.Asset.String()] = ps
	return nil
}

func (p *AddLiquidityTestKeeper) GetModuleAddress(_ string) (common.Address, error) {
	return common.NoAddress, nil
}

func (p *AddLiquidityTestKeeper) GetPOL(_ cosmos.Context) (ProtocolOwnedLiquidity, error) {
	return NewProtocolOwnedLiquidity(), nil
}

func (p *AddLiquidityTestKeeper) SetPOL(_ cosmos.Context, pol ProtocolOwnedLiquidity) error {
	return nil
}

func (p *AddLiquidityTestKeeper) GetLiquidityProvider(ctx cosmos.Context, asset common.Asset, addr common.Address) (LiquidityProvider, error) {
	if notExistLiquidityProviderAsset.Equals(asset) {
		return LiquidityProvider{}, errors.New("simulate error for test")
	}
	lp := LiquidityProvider{
		Asset:             asset,
		CacaoAddress:      addr,
		Units:             cosmos.ZeroUint(),
		PendingCacao:      cosmos.ZeroUint(),
		PendingAsset:      cosmos.ZeroUint(),
		CacaoDepositValue: cosmos.ZeroUint(),
		AssetDepositValue: cosmos.ZeroUint(),
	}
	key := p.GetKey(ctx, "lp/", lp.Key())
	if res, ok := p.store[key]; ok {
		lp, ok = res.(LiquidityProvider)
		if !ok {
			return lp, fmt.Errorf("dev error: failed to cast liquidity provider")
		}
		return lp, nil
	}
	lp.Units = p.liquidityUnits
	return lp, nil
}

func (p *AddLiquidityTestKeeper) SetLiquidityProvider(ctx cosmos.Context, lp LiquidityProvider) {
	key := p.GetKey(ctx, "lp/", lp.Key())
	p.store[key] = lp
}

func (p *AddLiquidityTestKeeper) AddOwnership(ctx cosmos.Context, coin common.Coin, addr cosmos.AccAddress) error {
	p.liquidityUnits = p.liquidityUnits.Add(coin.Amount)
	return nil
}

func (s *HandlerAddLiquiditySuite) TestCalculateLPUnitsV1(c *C) {
	inputs := []struct {
		name           string
		oldLPUnits     cosmos.Uint
		poolRune       cosmos.Uint
		poolAsset      cosmos.Uint
		addRune        cosmos.Uint
		addAsset       cosmos.Uint
		poolUnits      cosmos.Uint
		liquidityUnits cosmos.Uint
		expectedErr    error
	}{
		{
			name:           "first-add-zero-rune",
			oldLPUnits:     cosmos.ZeroUint(),
			poolRune:       cosmos.ZeroUint(),
			poolAsset:      cosmos.ZeroUint(),
			addRune:        cosmos.ZeroUint(),
			addAsset:       cosmos.NewUint(100 * common.One),
			poolUnits:      cosmos.ZeroUint(),
			liquidityUnits: cosmos.ZeroUint(),
			expectedErr:    errors.New("total RUNE in the pool is zero"),
		},
		{
			name:           "first-add-zero-asset",
			oldLPUnits:     cosmos.ZeroUint(),
			poolRune:       cosmos.ZeroUint(),
			poolAsset:      cosmos.ZeroUint(),
			addRune:        cosmos.NewUint(100 * common.One),
			addAsset:       cosmos.ZeroUint(),
			poolUnits:      cosmos.ZeroUint(),
			liquidityUnits: cosmos.ZeroUint(),
			expectedErr:    errors.New("total asset in the pool is zero"),
		},
		{
			name:           "first-add",
			oldLPUnits:     cosmos.ZeroUint(),
			poolRune:       cosmos.ZeroUint(),
			poolAsset:      cosmos.ZeroUint(),
			addRune:        cosmos.NewUint(100 * common.One),
			addAsset:       cosmos.NewUint(100 * common.One),
			poolUnits:      cosmos.NewUint(100 * common.One),
			liquidityUnits: cosmos.NewUint(100 * common.One),
			expectedErr:    nil,
		},
		{
			name:           "second-add",
			oldLPUnits:     cosmos.NewUint(500 * common.One),
			poolRune:       cosmos.NewUint(500 * common.One),
			poolAsset:      cosmos.NewUint(500 * common.One),
			addRune:        cosmos.NewUint(345 * common.One),
			addAsset:       cosmos.NewUint(234 * common.One),
			poolUnits:      cosmos.NewUint(76359469067),
			liquidityUnits: cosmos.NewUint(26359469067),
			expectedErr:    nil,
		},
	}

	for _, item := range inputs {
		c.Logf("Name: %s", item.name)
		poolUnits, liquidityUnits, err := calculatePoolUnitsV1(item.oldLPUnits, item.poolRune, item.poolAsset, item.addRune, item.addAsset)
		if item.expectedErr == nil {
			c.Assert(err, IsNil)
		} else {
			c.Assert(err.Error(), Equals, item.expectedErr.Error())
		}

		c.Check(item.poolUnits.Uint64(), Equals, poolUnits.Uint64(), Commentf("%d / %d", item.poolUnits.Uint64(), poolUnits.Uint64()))
		c.Check(item.liquidityUnits.Uint64(), Equals, liquidityUnits.Uint64(), Commentf("%d / %d", item.liquidityUnits.Uint64(), liquidityUnits.Uint64()))
	}
}

func (s *HandlerAddLiquiditySuite) TestValidateAddLiquidityMessage(c *C) {
	ps := NewAddLiquidityTestKeeper()
	ctx, mgr := setupManagerForTest(c)
	mgr.K = ps
	tx := GetRandomTx()
	bnbAddress := GetRandomBNBAddress()
	assetAddress := GetRandomBNBAddress()
	h := NewAddLiquidityHandler(mgr)
	c.Assert(h.validateAddLiquidityMessage(ctx, ps, common.Asset{}, tx, bnbAddress, assetAddress), NotNil)
	c.Assert(h.validateAddLiquidityMessage(ctx, ps, common.BNBAsset, tx, bnbAddress, assetAddress), NotNil)
	c.Assert(h.validateAddLiquidityMessage(ctx, ps, common.BNBAsset, tx, bnbAddress, assetAddress), NotNil)
	c.Assert(h.validateAddLiquidityMessage(ctx, ps, common.BNBAsset, common.Tx{}, bnbAddress, assetAddress), NotNil)
	c.Assert(h.validateAddLiquidityMessage(ctx, ps, common.BNBAsset, tx, common.NoAddress, common.NoAddress), NotNil)
	c.Assert(h.validateAddLiquidityMessage(ctx, ps, common.BNBAsset, tx, bnbAddress, assetAddress), NotNil)
	c.Assert(h.validateAddLiquidityMessage(ctx, ps, common.BNBAsset, tx, common.NoAddress, assetAddress), NotNil)
	c.Assert(h.validateAddLiquidityMessage(ctx, ps, common.BTCAsset, tx, bnbAddress, common.NoAddress), NotNil)
	c.Assert(ps.SetPool(ctx, Pool{
		BalanceCacao: cosmos.NewUint(100 * common.One),
		BalanceAsset: cosmos.NewUint(100 * common.One),
		Asset:        common.BNBAsset,
		LPUnits:      cosmos.NewUint(100 * common.One),
		Status:       PoolAvailable,
	}), IsNil)
	c.Assert(h.validateAddLiquidityMessage(ctx, ps, common.BNBAsset, tx, bnbAddress, assetAddress), Equals, nil)
}

func (s *HandlerAddLiquiditySuite) TestAddLiquidityV1(c *C) {
	ps := NewAddLiquidityTestKeeper()
	ctx, _ := setupKeeperForTest(c)
	tx := GetRandomTx()

	runeAddress := GetRandomBaseAddress()
	assetAddress := GetRandomBNBAddress()
	btcAddress, err := common.NewAddress("bc1qwqdg6squsna38e46795at95yu9atm8azzmyvckulcc7kytlcckxswvvzej", GetCurrentVersion())
	c.Assert(err, IsNil)
	constAccessor := constants.GetConstantValues(GetCurrentVersion())
	h := NewAddLiquidityHandler(NewDummyMgrWithKeeper(ps))
	err = h.addLiquidity(ctx, common.Asset{}, cosmos.NewUint(100*common.One), cosmos.NewUint(100*common.One), runeAddress, assetAddress, tx, false, constAccessor, 0)
	c.Assert(err, NotNil)
	c.Assert(ps.SetPool(ctx, Pool{
		BalanceCacao:        cosmos.ZeroUint(),
		BalanceAsset:        cosmos.NewUint(100 * common.One),
		Asset:               common.BNBAsset,
		LPUnits:             cosmos.NewUint(100 * common.One),
		SynthUnits:          cosmos.ZeroUint(),
		PendingInboundAsset: cosmos.ZeroUint(),
		PendingInboundCacao: cosmos.ZeroUint(),
		Status:              PoolAvailable,
	}), IsNil)
	err = h.addLiquidity(ctx, common.BNBAsset, cosmos.NewUint(100*common.One), cosmos.NewUint(100*common.One), runeAddress, assetAddress, tx, false, constAccessor, 0)
	c.Assert(err, IsNil)
	su, err := ps.GetLiquidityProvider(ctx, common.BNBAsset, runeAddress)
	c.Assert(err, IsNil)
	// c.Assert(su.Units.Equal(cosmos.NewUint(11250000000)), Equals, true, Commentf("%d", su.Units.Uint64()))

	c.Assert(ps.SetPool(ctx, Pool{
		BalanceCacao:        cosmos.NewUint(100 * common.One),
		BalanceAsset:        cosmos.NewUint(100 * common.One),
		Asset:               notExistLiquidityProviderAsset,
		LPUnits:             cosmos.NewUint(100 * common.One),
		SynthUnits:          cosmos.ZeroUint(),
		PendingInboundAsset: cosmos.ZeroUint(),
		PendingInboundCacao: cosmos.ZeroUint(),
		Status:              PoolAvailable,
	}), IsNil)
	// add asymmetically
	err = h.addLiquidity(ctx, common.BNBAsset, cosmos.NewUint(100*common.One), cosmos.ZeroUint(), runeAddress, assetAddress, tx, false, constAccessor, 0)
	c.Assert(err, IsNil)
	err = h.addLiquidity(ctx, common.BNBAsset, cosmos.ZeroUint(), cosmos.NewUint(100*common.One), runeAddress, assetAddress, tx, false, constAccessor, 0)
	c.Assert(err, IsNil)

	err = h.addLiquidity(ctx, notExistLiquidityProviderAsset, cosmos.NewUint(100*common.One), cosmos.NewUint(100*common.One), runeAddress, assetAddress, tx, false, constAccessor, 0)
	c.Assert(err, NotNil)
	c.Assert(ps.SetPool(ctx, Pool{
		BalanceCacao:        cosmos.NewUint(100 * common.One),
		BalanceAsset:        cosmos.NewUint(100 * common.One),
		Asset:               common.BNBAsset,
		LPUnits:             cosmos.NewUint(100 * common.One),
		SynthUnits:          cosmos.ZeroUint(),
		PendingInboundAsset: cosmos.ZeroUint(),
		PendingInboundCacao: cosmos.ZeroUint(),
		Status:              PoolAvailable,
	}), IsNil)

	for i := 1; i <= 150; i++ {
		lp := LiquidityProvider{Units: cosmos.NewUint(common.One / 5000)}
		ps.SetLiquidityProvider(ctx, lp)
	}
	err = h.addLiquidity(ctx, common.BNBAsset, cosmos.NewUint(common.One), cosmos.NewUint(common.One), runeAddress, assetAddress, tx, false, constAccessor, 0)
	c.Assert(err, IsNil)

	err = h.addLiquidity(ctx, common.BNBAsset, cosmos.NewUint(100*common.One), cosmos.NewUint(100*common.One), runeAddress, assetAddress, tx, false, constAccessor, 0)
	c.Assert(err, IsNil)
	p, err := ps.GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Check(p.LPUnits.Equal(cosmos.NewUint(201*common.One)), Equals, true, Commentf("%d", p.LPUnits.Uint64()))

	// Test atomic cross chain liquidity provision
	// create BTC pool
	c.Assert(ps.SetPool(ctx, Pool{
		BalanceCacao:        cosmos.ZeroUint(),
		BalanceAsset:        cosmos.ZeroUint(),
		Asset:               common.BTCAsset,
		LPUnits:             cosmos.ZeroUint(),
		SynthUnits:          cosmos.ZeroUint(),
		PendingInboundAsset: cosmos.ZeroUint(),
		PendingInboundCacao: cosmos.ZeroUint(),
		Status:              PoolAvailable,
	}), IsNil)

	// add rune
	err = h.addLiquidity(ctx, common.BTCAsset, cosmos.NewUint(100*common.One), cosmos.ZeroUint(), runeAddress, btcAddress, tx, true, constAccessor, 0)
	c.Assert(err, IsNil)
	_, err = ps.GetLiquidityProvider(ctx, common.BTCAsset, runeAddress)
	c.Assert(err, IsNil)
	// c.Check(su.Units.IsZero(), Equals, true)
	// add btc
	err = h.addLiquidity(ctx, common.BTCAsset, cosmos.ZeroUint(), cosmos.NewUint(100*common.One), runeAddress, btcAddress, tx, false, constAccessor, 0)
	c.Assert(err, IsNil)
	su, err = ps.GetLiquidityProvider(ctx, common.BTCAsset, runeAddress)
	c.Assert(err, IsNil)
	c.Check(su.Units.IsZero(), Equals, false)
	p, err = ps.GetPool(ctx, common.BTCAsset)
	c.Assert(err, IsNil)
	c.Check(p.BalanceAsset.Equal(cosmos.NewUint(100*common.One)), Equals, true, Commentf("%d", p.BalanceAsset.Uint64()))
	c.Check(p.BalanceCacao.Equal(cosmos.NewUint(100*common.One)), Equals, true, Commentf("%d", p.BalanceCacao.Uint64()))
	c.Check(p.LPUnits.Equal(cosmos.NewUint(100*common.One)), Equals, true, Commentf("%d", p.LPUnits.Uint64()))
}

func (HandlerAddLiquiditySuite) TestRuneOnlyLiquidity(c *C) {
	ctx, k := setupKeeperForTest(c)
	tx := GetRandomTx()

	c.Assert(k.SetPool(ctx, Pool{
		BalanceCacao: cosmos.NewUint(100 * common.One),
		BalanceAsset: cosmos.NewUint(100 * common.One),
		Asset:        common.BTCAsset,
		LPUnits:      cosmos.NewUint(100 * common.One),
		SynthUnits:   cosmos.ZeroUint(),
		Status:       PoolAvailable,
	}), IsNil)

	runeAddr := GetRandomBaseAddress()
	constAccessor := constants.GetConstantValues(GetCurrentVersion())
	h := NewAddLiquidityHandler(NewDummyMgrWithKeeper(k))
	err := h.addLiquidity(ctx, common.BTCAsset, cosmos.NewUint(100*common.One), cosmos.ZeroUint(), runeAddr, common.NoAddress, tx, false, constAccessor, 0)
	c.Assert(err, IsNil)

	su, err := k.GetLiquidityProvider(ctx, common.BTCAsset, runeAddr)
	c.Assert(err, IsNil)
	c.Assert(su.Units.Uint64(), Equals, uint64(2500000000), Commentf("%d", su.Units.Uint64()))

	pool, err := k.GetPool(ctx, common.BTCAsset)
	c.Assert(err, IsNil)
	c.Assert(pool.LPUnits.Uint64(), Equals, uint64(12500000000), Commentf("%d", pool.LPUnits.Uint64()))
}

func (HandlerAddLiquiditySuite) TestAssetOnlyProvidedLiquidity(c *C) {
	ctx, k := setupKeeperForTest(c)
	tx := GetRandomTx()

	c.Assert(k.SetPool(ctx, Pool{
		BalanceCacao: cosmos.NewUint(100 * common.One),
		BalanceAsset: cosmos.NewUint(100 * common.One),
		Asset:        common.BTCAsset,
		LPUnits:      cosmos.NewUint(100 * common.One),
		SynthUnits:   cosmos.ZeroUint(),
		Status:       PoolAvailable,
	}), IsNil)

	assetAddr := GetRandomBTCAddress()
	constAccessor := constants.GetConstantValues(GetCurrentVersion())
	h := NewAddLiquidityHandler(NewDummyMgrWithKeeper(k))
	err := h.addLiquidity(ctx, common.BTCAsset, cosmos.ZeroUint(), cosmos.NewUint(100*common.One), common.NoAddress, assetAddr, tx, false, constAccessor, 0)
	c.Assert(err, IsNil)

	su, err := k.GetLiquidityProvider(ctx, common.BTCAsset, assetAddr)
	c.Assert(err, IsNil)
	c.Assert(su.Units.Uint64(), Equals, uint64(2500000000), Commentf("%d", su.Units.Uint64()))

	pool, err := k.GetPool(ctx, common.BTCAsset)
	c.Assert(err, IsNil)
	c.Assert(pool.LPUnits.Uint64(), Equals, uint64(12500000000), Commentf("%d", pool.LPUnits.Uint64()))
}

func (HandlerAddLiquiditySuite) TestSynthValidate(c *C) {
	ctx, mgr := setupManagerForTest(c)

	asset := common.BTCAsset.GetSyntheticAsset()

	c.Assert(mgr.Keeper().SetPool(ctx, Pool{
		BalanceCacao: cosmos.NewUint(100 * common.One),
		BalanceAsset: cosmos.NewUint(10 * common.One),
		Asset:        asset,
		LPUnits:      cosmos.ZeroUint(),
		SynthUnits:   cosmos.ZeroUint(),
		Status:       PoolAvailable,
	}), IsNil)

	handler := NewAddLiquidityHandler(mgr)

	addr := GetRandomBTCAddress()
	signer := GetRandomBech32Addr()
	addTxHash := GetRandomTxHash()

	tx := common.NewTx(
		addTxHash,
		addr,
		addr,
		common.Coins{common.NewCoin(asset, cosmos.NewUint(1000*common.One))},
		BNBGasFeeSingleton,
		fmt.Sprintf("add:%s", asset.String()),
	)

	// happy path
	msg := NewMsgAddLiquidity(tx, asset, cosmos.ZeroUint(), cosmos.NewUint(1000*common.One), common.NoAddress, addr, common.NoAddress, cosmos.ZeroUint(), signer, 1)
	err := handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	// don't allow non-gas assets
	busd, err := common.NewAsset("BNB.BUSD-BD1")
	c.Assert(err, IsNil)
	msg = NewMsgAddLiquidity(tx, busd.GetSyntheticAsset(), cosmos.ZeroUint(), cosmos.NewUint(1000*common.One), addr, common.NoAddress, common.NoAddress, cosmos.ZeroUint(), signer, 1)
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)

	// address mismatch
	msg = NewMsgAddLiquidity(tx, asset, cosmos.ZeroUint(), cosmos.NewUint(1000*common.One), addr, common.NoAddress, common.NoAddress, cosmos.ZeroUint(), signer, 1)
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
	msg = NewMsgAddLiquidity(tx, asset, cosmos.ZeroUint(), cosmos.NewUint(1000*common.One), common.NoAddress, common.NoAddress, common.NoAddress, cosmos.ZeroUint(), signer, 1)
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)

	// don't allow rune
	msg = NewMsgAddLiquidity(tx, asset, cosmos.NewUint(1000*common.One), cosmos.ZeroUint(), common.NoAddress, addr, common.NoAddress, cosmos.ZeroUint(), signer, 1)
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
	msg = NewMsgAddLiquidity(tx, asset, cosmos.NewUint(1000*common.One), cosmos.NewUint(1000*common.One), common.NoAddress, addr, common.NoAddress, cosmos.ZeroUint(), signer, 1)
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
}

func (HandlerObservedTxInSuite) TestAddToDisabledSavers(c *C) {
	ctx, mgr := setupManagerForTest(c)

	asset := common.BTCAsset.GetSyntheticAsset()

	c.Assert(mgr.Keeper().SetPool(ctx, Pool{
		BalanceCacao: cosmos.NewUint(100 * common.One),
		BalanceAsset: cosmos.NewUint(10 * common.One),
		Asset:        asset,
		LPUnits:      cosmos.ZeroUint(),
		SynthUnits:   cosmos.ZeroUint(),
		Status:       PoolAvailable,
	}), IsNil)

	handler := NewAddLiquidityHandler(mgr)

	addr := GetRandomBTCAddress()
	signer := GetRandomBech32Addr()
	addTxHash := GetRandomTxHash()

	tx := common.NewTx(
		addTxHash,
		addr,
		addr,
		common.Coins{common.NewCoin(asset, cosmos.NewUint(1000*common.One))},
		BNBGasFeeSingleton,
		fmt.Sprintf("add:%s", asset.String()),
	)

	// happy path
	msg := NewMsgAddLiquidity(tx, asset, cosmos.ZeroUint(), cosmos.NewUint(1000*common.One), common.NoAddress, addr, common.NoAddress, cosmos.ZeroUint(), signer, 1)
	err := handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	mgr.Keeper().SetMimir(ctx, "SaverDepositDisabled", 1)

	// unhappy path as Savers are disabled
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
	c.Assert(strings.Contains(err.Error(), "saver deposits are disabled"), Equals, true)

	// normal add liquidity works fine
	asset = common.BTCAsset
	tx.Coins = common.Coins{common.NewCoin(asset, cosmos.NewUint(1000*common.One))}
	// happy path
	msg = NewMsgAddLiquidity(tx, asset, cosmos.ZeroUint(), cosmos.NewUint(1000*common.One), common.NoAddress, addr, common.NoAddress, cosmos.ZeroUint(), signer, 1)
	err = handler.validate(ctx, *msg)
	c.Assert(err, IsNil)
}

func (HandlerAddLiquiditySuite) TestLiquidityNodeValidate(c *C) {
	ctx, mgr := setupManagerForTest(c)

	node := GetRandomValidatorNode(NodeActive)
	bp := NewBondProviders(node.NodeAddress)
	acc, err := node.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp.Providers = append(bp.Providers, NewBondProvider(acc))
	bp.Providers[0].Bonded = true
	c.Assert(mgr.Keeper().SetBondProviders(ctx, bp), IsNil)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, node), IsNil)

	asset := common.BTCAsset

	// already bonded node
	lp := LiquidityProvider{
		Asset:        asset,
		CacaoAddress: node.BondAddress,
		AssetAddress: GetRandomBTCAddress(),
		Units:        cosmos.NewUint(10 * common.One),
	}
	mgr.Keeper().SetLiquidityProvider(ctx, lp)

	c.Assert(mgr.Keeper().SetPool(ctx, Pool{
		BalanceCacao: cosmos.NewUint(100 * common.One),
		BalanceAsset: cosmos.NewUint(10 * common.One),
		Asset:        asset,
		LPUnits:      cosmos.NewUint(100 * common.One),
		SynthUnits:   cosmos.ZeroUint(),
		Status:       PoolAvailable,
	}), IsNil)

	handler := NewAddLiquidityHandler(mgr)

	addr := GetRandomBTCAddress()
	signer := GetRandomBech32Addr()
	addTxHash := GetRandomTxHash()

	tx := common.NewTx(
		addTxHash,
		addr,
		addr,
		common.Coins{common.NewCoin(asset, cosmos.NewUint(10*common.One))},
		BTCGasFeeSingleton,
		fmt.Sprintf("add:%s", asset.String()),
	)

	msg := NewMsgAddLiquidity(tx, asset, cosmos.ZeroUint(), cosmos.NewUint(10*common.One), node.BondAddress, addr, common.NoAddress, cosmos.ZeroUint(), signer, 0)
	// Set gas pool's Asset to represent existence for IsEmpty
	gasPool := NewPool()
	gasPool.Asset = common.BTCAsset
	c.Assert(mgr.Keeper().SetPool(ctx, gasPool), IsNil)

	// happy path
	err = handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	// genesis node shouldn't be able to add
	genesis, err := cosmos.AccAddressFromBech32(GenesisNodes[0])
	c.Assert(err, IsNil)
	lp.NodeBondAddress = genesis
	mgr.Keeper().SetLiquidityProvider(ctx, lp)

	err = handler.validate(ctx, *msg)
	c.Assert(err.Error(), Equals, fmt.Sprintf("cannot add liquidity to genesis node: %s", lp.NodeBondAddress.String()))

	// cannot add liquidity bond to ready status node
	lp.NodeBondAddress = node.NodeAddress
	mgr.Keeper().SetLiquidityProvider(ctx, lp)
	node.Status = NodeReady
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, node), IsNil)

	err = handler.validate(ctx, *msg)
	c.Assert(err.Error(), Equals, fmt.Sprintf("cannot add bond while node is ready status: %s", lp.NodeBondAddress.String()))

	// check if bonding is paused
	node.Status = NodeActive
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, node), IsNil)
	mgr.Keeper().SetMimir(ctx, constants.PauseBond.String(), 1)

	err = handler.validate(ctx, *msg)
	c.Assert(err.Error(), Equals, "bonding has been paused")

	mgr.Keeper().SetMimir(ctx, "MaximumBondInRune", 1)
	mgr.Keeper().SetMimir(ctx, constants.PauseBond.String(), -1)
}

func (HandlerAddLiquiditySuite) TestAddSynthNoLPs(c *C) {
	// there is an odd case where its possible in a synth vault to have a
	// balance asset of non-zero BUT have no LPs yet. Testing this edge case.
	ctx, k := setupKeeperForTest(c)
	tx := GetRandomTx()

	asset := common.BTCAsset.GetSyntheticAsset()

	pool := NewPool()
	pool.Asset = asset
	pool.Status = PoolAvailable
	pool.BalanceCacao = cosmos.NewUint(0)
	pool.BalanceAsset = cosmos.NewUint(10 * common.One)
	c.Assert(k.SetPool(ctx, pool), IsNil)

	coin := common.NewCoin(asset, pool.BalanceAsset)
	c.Assert(k.MintToModule(ctx, ModuleName, coin), IsNil)
	c.Assert(k.SendFromModuleToModule(ctx, ModuleName, AsgardName, common.NewCoins(coin)), IsNil)

	addr := GetRandomBTCAddress()
	constAccessor := constants.GetConstantValues(GetCurrentVersion())
	h := NewAddLiquidityHandler(NewDummyMgrWithKeeper(k))
	addCoin := common.NewCoin(asset, cosmos.NewUint(10*common.One))
	c.Assert(k.MintToModule(ctx, ModuleName, addCoin), IsNil)
	c.Assert(k.SendFromModuleToModule(ctx, ModuleName, AsgardName, common.NewCoins(addCoin)), IsNil)
	err := h.addLiquidity(ctx, asset, cosmos.ZeroUint(), addCoin.Amount, common.NoAddress, addr, tx, false, constAccessor, 0)
	c.Assert(err, IsNil)

	su, err := k.GetLiquidityProvider(ctx, asset, addr)
	c.Assert(err, IsNil)
	c.Check(su.Units.Uint64(), Equals, uint64(10*common.One), Commentf("%d", su.Units.Uint64()))

	pool, err = k.GetPool(ctx, asset)
	c.Assert(err, IsNil)
	c.Check(pool.BalanceCacao.Uint64(), Equals, uint64(0), Commentf("%d", pool.BalanceCacao.Uint64()))
	c.Check(pool.BalanceAsset.Uint64(), Equals, uint64(20*common.One), Commentf("%d", pool.BalanceAsset.Uint64()))
	c.Check(pool.LPUnits.Uint64(), Equals, uint64(10*common.One), Commentf("%d", pool.LPUnits.Uint64()))
}

func (HandlerAddLiquiditySuite) TestAddSynth(c *C) {
	ctx, k := setupKeeperForTest(c)
	tx := GetRandomTx()

	asset := common.BTCAsset.GetSyntheticAsset()

	pool := NewPool()
	pool.Asset = asset
	pool.Status = PoolAvailable
	pool.BalanceCacao = cosmos.NewUint(0)
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.LPUnits = cosmos.NewUint(100)
	c.Assert(k.SetPool(ctx, pool), IsNil)

	coin := common.NewCoin(asset, pool.BalanceAsset)
	c.Assert(k.MintToModule(ctx, ModuleName, coin), IsNil)
	c.Assert(k.SendFromModuleToModule(ctx, ModuleName, AsgardName, common.NewCoins(coin)), IsNil)

	addr := GetRandomBTCAddress()
	constAccessor := constants.GetConstantValues(GetCurrentVersion())
	h := NewAddLiquidityHandler(NewDummyMgrWithKeeper(k))
	addCoin := common.NewCoin(asset, cosmos.NewUint(100*common.One))
	c.Assert(k.MintToModule(ctx, ModuleName, addCoin), IsNil)
	c.Assert(k.SendFromModuleToModule(ctx, ModuleName, AsgardName, common.NewCoins(addCoin)), IsNil)
	err := h.addLiquidity(ctx, asset, cosmos.ZeroUint(), addCoin.Amount, common.NoAddress, addr, tx, false, constAccessor, 0)
	c.Assert(err, IsNil)

	su, err := k.GetLiquidityProvider(ctx, asset, addr)
	c.Assert(err, IsNil)
	c.Check(su.Units.Uint64(), Equals, uint64(100), Commentf("%d", su.Units.Uint64()))

	pool, err = k.GetPool(ctx, asset)
	c.Assert(err, IsNil)
	c.Check(pool.BalanceCacao.Uint64(), Equals, uint64(0), Commentf("%d", pool.BalanceCacao.Uint64()))
	c.Check(pool.BalanceAsset.Uint64(), Equals, uint64(200*common.One), Commentf("%d", pool.BalanceAsset.Uint64()))
	c.Check(pool.LPUnits.Uint64(), Equals, uint64(200), Commentf("%d", pool.LPUnits.Uint64()))
}
