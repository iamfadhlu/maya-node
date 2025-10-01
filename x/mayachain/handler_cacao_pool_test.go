package mayachain

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"

	. "gopkg.in/check.v1"

	"github.com/tendermint/tendermint/libs/log"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	openapi "gitlab.com/mayachain/mayanode/openapi/gen"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"

	"github.com/cosmos/cosmos-sdk/simapp"
	"github.com/cosmos/cosmos-sdk/store"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	capabilitykeeper "github.com/cosmos/cosmos-sdk/x/capability/keeper"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	upgradekeeper "github.com/cosmos/cosmos-sdk/x/upgrade/keeper"
	ibctransferkeeper "github.com/cosmos/ibc-go/v2/modules/apps/transfer/keeper"
	ibctransfertypes "github.com/cosmos/ibc-go/v2/modules/apps/transfer/types"
	ibccoreclienttypes "github.com/cosmos/ibc-go/v2/modules/core/02-client/types"
	ibcconnectiontypes "github.com/cosmos/ibc-go/v2/modules/core/03-connection/types"
	ibchost "github.com/cosmos/ibc-go/v2/modules/core/24-host"
	ibckeeper "github.com/cosmos/ibc-go/v2/modules/core/keeper"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	dbm "github.com/tendermint/tm-db"

	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
)

type CacaoPoolTestSuite struct {
	ctx             cosmos.Context
	mgr             *Mgrs
	queue           *SwapQueueVCUR
	keeper          *TestAffiliateKeeper
	handler         ObservedTxInHandler
	_depositHandler DepositHandler
	node            types.NodeAccount
	signer          cosmos.AccAddress
	mockTxOutStore  MockWithdrawTxOutStoreForMultiAff
	errLogs         *ErrorLogCollector
	tx              common.Tx
	addrMayaDog     common.Address
	accMayaDog      cosmos.AccAddress
	addrBtcDog      common.Address
	addrMayaCat     common.Address
	accMayaCat      cosmos.AccAddress
	addrEthCat      common.Address
	addrMayaFox     common.Address
	accMayaFox      cosmos.AccAddress
	addrEthFox      common.Address
	addrBtcFox      common.Address
}

var _ = Suite(&CacaoPoolTestSuite{})

func (s *CacaoPoolTestSuite) SetUpTest(c *C) {
	s.ctx, s.mgr = setupManagerForTestAsInRegtest(c)

	s.errLogs = NewErrorLogCollector(`"level":"error"`, "E", []string{"\033[43m", "\033[0m", "ignore me", "ignore this error", "error calculating rewards"})
	// Create a logger that uses the custom message collector
	logger := log.NewTMLogger(s.errLogs)
	s.ctx = s.ctx.WithLogger(logger)

	gasFee := s.mgr.gasMgr.GetFee(s.ctx, common.BASEChain, common.BaseAsset())
	gas := common.Gas{common.NewCoin(common.BaseNative, gasFee)}
	from := GetRandomBaseAddress()
	to := GetRandomBaseAddress()

	s.tx = common.NewTx(
		GetRandomTxHash(),
		from, to,
		common.Coins{common.NewCoin(common.BaseAsset(), cosmos.NewUint(10000*common.One))},
		gas, "",
	)
	s.node = GetRandomValidatorNode(NodeActive)
	vault := GetRandomVault()
	vault.PubKey = GetRandomPubKey()
	asgard := GetRandomVault()
	asgard.PubKey = GetRandomPubKey()

	asgard.Coins = common.Coins{
		// common.NewCoin(common.BaseNative, cosmos.NewUint(10000000*common.One)),
		common.NewCoin(common.BTCAsset, cosmos.NewUint(10000000*common.One)),
		// common.NewCoin(common.BNBAsset, cosmos.NewUint(10000000*common.One)),
		// common.NewCoin(common.ETHAsset, cosmos.NewUint(10000000*common.One)),
	}

	s.keeper = &TestAffiliateKeeper{
		nas:       NodeAccounts{s.node},
		voter:     NewObservedTxVoter(s.tx.ID, make(ObservedTxs, 0)),
		voterTxID: s.tx.ID,
		vault:     vault,
		asgard:    asgard,
	}

	var err error
	s.keeper.Keeper = s.mgr.K
	s.mgr.K = s.keeper
	s.mgr.networkMgr, err = GetNetworkManager(GetCurrentVersion(), s.mgr.K, s.mgr.txOutStore, s.mgr.eventMgr)
	c.Assert(err, IsNil)

	// c.Assert(s.mgr.Keeper().SaveNetworkFee(s.ctx, common.BASEChain, NewNetworkFee(common.BASEChain, 1, 2000000000)), IsNil)
	// c.Assert(s.mgr.Keeper().SaveNetworkFee(s.ctx, common.BTCChain, NewNetworkFee(common.BTCChain, 70, 500)), IsNil)
	// c.Assert(s.mgr.Keeper().SaveNetworkFee(s.ctx, common.ETHChain, NewNetworkFee(common.ETHChain, 80000, 300)), IsNil)

	txOutStore, err := GetTxOutStore(GetCurrentVersion(), s.mgr.K, s.mgr.eventMgr, s.mgr.gasMgr)
	c.Assert(err, IsNil)

	s.mockTxOutStore = MockWithdrawTxOutStoreForMultiAff{
		TxOutStore: txOutStore,
		asgard:     asgard,
	}

	s.mgr.txOutStore = &s.mockTxOutStore
	s.mockTxOutStore.full = true

	s.addrMayaDog, _ = common.NewAddress("tmaya1zf3gsk7edzwl9syyefvfhle37cjtql35hdgtzt", s.mgr.currentVersion)
	s.accMayaDog, _ = s.addrMayaDog.AccAddress()
	s.addrMayaCat, _ = common.NewAddress("tmaya1uuds8pd92qnnq0udw0rpg0szpgcslc9p8gps0z", s.mgr.currentVersion)
	s.accMayaCat, _ = s.addrMayaCat.AccAddress()
	s.addrMayaFox, _ = common.NewAddress("tmaya13wrmhnh2qe98rjse30pl7u6jxszjjwl4fd6gwn", s.mgr.currentVersion)
	s.accMayaFox, _ = s.addrMayaFox.AccAddress()
	s.addrEthCat, _ = common.NewAddress("0x1b03d088612a00df0049634e9cc8684d622cada2", s.mgr.currentVersion)
	s.addrEthFox, _ = common.NewAddress("0xe3c64974c78f5693bd2bc68b3221d58df5c6e877", s.mgr.currentVersion)
	s.addrBtcDog, _ = common.NewAddress("bcrt1qzf3gsk7edzwl9syyefvfhle37cjtql35tlzesk", s.mgr.currentVersion)
	s.addrBtcFox = GetRandomBTCAddress()
	s.signer = s.accMayaDog
	c.Assert(err, IsNil)

	assetAmount := cosmos.NewUint(10000000000)      // as in regtest template pol/pol.yaml
	cacaoAmount := cosmos.NewUint(1000000000000000) // as in regtest template default-state.yaml
	s.node.Bond = cacaoAmount
	// BTC pool
	pool := Pool{
		BalanceCacao: cacaoAmount,
		BalanceAsset: assetAmount,
		Asset:        common.BTCAsset,
		LPUnits:      cacaoAmount,
		SynthUnits:   cosmos.ZeroUint(),
		Status:       PoolAvailable,
	}
	lp := LiquidityProvider{
		Asset:        pool.Asset,
		CacaoAddress: s.node.BondAddress,
		AssetAddress: common.Address(s.node.NodeAddress),
		Units:        cacaoAmount,
		// NodeBondAddress: s.node.NodeAddress, // If this deprecated NodeBondAddress field is set, all the liquidity on this LP is bonded to the node
		LastAddHeight: 1,
		BondedNodes: []LPBondedNode{
			{
				NodeAddress: s.node.NodeAddress,
				Units:       cacaoAmount.QuoUint64(10), // Otherwise, this is used as the share of liquidity bonded to this specific node
			},
		},
	}
	bps := BondProviders{
		NodeAddress:     s.node.NodeAddress,
		NodeOperatorFee: cosmos.ZeroUint(),
		Providers: []BondProvider{{
			BondAddress: s.node.NodeAddress,
			Bonded:      true,
			Reward:      nil,
		}},
	}

	c.Assert(s.mgr.Keeper().SetPool(s.ctx, pool), IsNil)
	s.mgr.Keeper().SetLiquidityProvider(s.ctx, lp)
	c.Assert(s.mgr.Keeper().SetBondProviders(s.ctx, bps), IsNil)
	c.Assert(s.mgr.Keeper().SetNodeAccount(s.ctx, s.node), IsNil)

	s.queue = newSwapQueueVCUR(s.mgr.Keeper())
	s.handler = NewObservedTxInHandler(s.mgr)
	s._depositHandler = NewDepositHandler(s.mgr)

	// prepare funds
	addFunds := func(acc cosmos.AccAddress, amt uint64) {
		funds, err := common.NewCoin(common.BaseNative, cosmos.NewUint(amt)).Native()
		c.Assert(err, IsNil)
		err = s.mgr.Keeper().AddCoins(s.ctx, acc, cosmos.NewCoins(funds))
		c.Assert(err, IsNil)
	}
	addFunds(s.accMayaDog, 50_000_00*common.One)
	addFunds(s.accMayaCat, 250_000_00*common.One)
	addFunds(s.accMayaFox, 250_000_00*common.One)
	addFunds(s.keeper.nas[0].NodeAddress, 250_000_00*common.One)

	// s.mgr.Keeper().SetMimir(s.ctx, constants.MaxSynthPerPoolDepth.String(), 5000)
	// s.mgr.Keeper().SetMimir(s.ctx, constants.POLMaxNetworkDeposit.String(), 1000000000)
	// s.mgr.Keeper().SetMimir(s.ctx, constants.CACAOPoolMaxReserveBackstop.String(), 1000000000)
	// s.mgr.Keeper().SetMimir(s.ctx, constants.POLSynthUtilization.String(), 2500) // tc: POLTargetSynthPerPoolDepth
	// s.mgr.Keeper().SetMimir(s.ctx, constants.POLMaxPoolMovement.String(), 5000)  // 0.5%
	// s.mgr.Keeper().SetMimir(s.ctx, constants.POLBuffer.String(), 1000)
	// s.mgr.Keeper().SetMimir(s.ctx, "POL-BTC-BTC", 1)

	s.mgr.Keeper().SetMimir(s.ctx, constants.CACAOPoolEnabled.String(), 1)
	s.mgr.Keeper().SetMimir(s.ctx, constants.CACAOPoolRewardsEnabled.String(), 1)
}

func setupManagerForTestAsInRegtest(c *C) (cosmos.Context, *Mgrs) {
	SetupConfigForTest()
	keyAcc := cosmos.NewKVStoreKey(authtypes.StoreKey)
	keyBank := cosmos.NewKVStoreKey(banktypes.StoreKey)
	keyIBC := cosmos.NewKVStoreKey(ibctransfertypes.StoreKey)
	keyIBCHost := cosmos.NewKVStoreKey(ibchost.StoreKey)
	keyCap := cosmos.NewKVStoreKey(capabilitytypes.StoreKey)
	keyParams := cosmos.NewKVStoreKey(paramstypes.StoreKey)
	tkeyParams := cosmos.NewTransientStoreKey(paramstypes.TStoreKey)
	memKeys := sdk.NewMemoryStoreKeys(capabilitytypes.MemStoreKey)

	db := dbm.NewMemDB()
	ms := store.NewCommitMultiStore(db)
	ms.MountStoreWithDB(keyAcc, cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyParams, cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyThorchain, cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyBank, cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyCap, cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyIBCHost, cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyIBC, cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(tkeyParams, cosmos.StoreTypeTransient, db)
	err := ms.LoadLatestVersion()
	c.Assert(err, IsNil)

	ctx := cosmos.NewContext(ms, tmproto.Header{ChainID: "mayachain"}, false, log.NewNopLogger())
	ctx = ctx.WithBlockHeight(18)
	legacyCodec := makeTestCodec()
	marshaler := simapp.MakeTestEncodingConfig().Marshaler

	pk := paramskeeper.NewKeeper(marshaler, legacyCodec, keyParams, tkeyParams)
	pkt := ibctransfertypes.ParamKeyTable().RegisterParamSet(&ibccoreclienttypes.Params{}).RegisterParamSet(&ibcconnectiontypes.Params{})
	pk.Subspace(ibctransfertypes.ModuleName).WithKeyTable(pkt)
	sSIBC, _ := pk.GetSubspace(ibctransfertypes.ModuleName)
	ak := authkeeper.NewAccountKeeper(marshaler, keyAcc, pk.Subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, map[string][]string{
		ModuleName:                  {authtypes.Minter, authtypes.Burner},
		ibctransfertypes.ModuleName: {authtypes.Minter, authtypes.Burner},
		AsgardName:                  {},
		BondName:                    {},
		ReserveName:                 {},
		AffiliateCollectorName:      {},
		MayaFund:                    {},
		CACAOPoolName:               {},
	})

	bk := bankkeeper.NewBaseKeeper(marshaler, keyBank, ak, pk.Subspace(banktypes.ModuleName), nil)
	ck := capabilitykeeper.NewKeeper(marshaler, keyCap, memKeys[capabilitytypes.MemStoreKey])
	scopedIBCKeeper := ck.ScopeToModule(ibchost.ModuleName)
	scopedTransferKeeper := ck.ScopeToModule(ibctransfertypes.ModuleName)
	ck.Seal()
	IBCKeeper := ibckeeper.NewKeeper(marshaler, keyIBCHost, sSIBC, stakingkeeper.Keeper{}, upgradekeeper.Keeper{}, scopedIBCKeeper)
	ibck := ibctransferkeeper.NewKeeper(marshaler, keyIBC, sSIBC, IBCKeeper.ChannelKeeper, &IBCKeeper.PortKeeper, ak, bk, scopedTransferKeeper)
	ibck.SetParams(ctx, ibctransfertypes.Params{})
	c.Assert(bk.MintCoins(ctx, ModuleName, cosmos.Coins{
		cosmos.NewCoin(common.BaseAsset().Native(), cosmos.NewInt(200_000_000_00000000)),
	}), IsNil)
	k := keeper.NewKeeper(marshaler, bk, ak, ibck, keyThorchain)
	FundModule(c, ctx, k, ModuleName, 5000000)
	FundModule(c, ctx, k, AsgardName, 5000000)
	FundModule(c, ctx, k, ReserveName, 35000000)

	c.Assert(k.SaveNetworkFee(ctx, common.BASEChain, NewNetworkFee(common.BASEChain, 1, 2000000000)), IsNil)
	c.Assert(k.SaveNetworkFee(ctx, common.BTCChain, NewNetworkFee(common.BTCChain, 70, 500)), IsNil)
	c.Assert(k.SaveNetworkFee(ctx, common.BNBChain, NewNetworkFee(common.BNBChain, 1, 37500)), IsNil)
	c.Assert(k.SaveNetworkFee(ctx, common.ETHChain, NewNetworkFee(common.ETHChain, 80000, 300)), IsNil)

	os.Setenv("NET", "mocknet")
	mgr := NewManagers(k, marshaler, bk, ak, ibck, keyThorchain)
	constants.SWVersion = GetCurrentVersion()

	_, hasVerStored := k.GetVersionWithCtx(ctx)
	c.Assert(hasVerStored, Equals, false)

	c.Assert(mgr.BeginBlock(ctx), IsNil)
	mgr.gasMgr.BeginBlock(mgr)

	verStored, hasVerStored := k.GetVersionWithCtx(ctx)
	c.Assert(hasVerStored, Equals, true)
	verComputed := k.GetLowestActiveVersion(ctx)
	c.Assert(verStored.String(), Equals, verComputed.String())

	return ctx, mgr
}

func (s *CacaoPoolTestSuite) handleDeposit(msg *MsgDeposit) error {
	// pay gas fee for "broadcasting" to have the Reserve unified with the regtests
	coin := common.NewCoin(common.BaseNative, cosmos.NewUint(2000000000))
	if err := s.mgr.Keeper().SendFromModuleToModule(s.ctx, ModuleName, ReserveName, common.NewCoins(coin)); err != nil {
		return err
	}
	// handle the tx
	s.ctx = s.ctx.WithTxBytes([]byte(common.RandHexString(10)))
	_, err := s._depositHandler.handle(s.ctx, *msg)
	if err == nil {
		if str, cnt := s.errLogs.GetCollectedString(true); cnt > 0 {
			err = fmt.Errorf("%s", str)
		}
	}
	return err
}

func (s *CacaoPoolTestSuite) fetchQueue() (swapItems, error) {
	s.mockTxOutStore.tois = nil
	swaps, err := s.queue.FetchQueue(s.ctx, s.mgr)
	if err == nil {
		if str, cnt := s.errLogs.GetCollectedString(true); cnt > 0 {
			err = fmt.Errorf("%s", str)
		}
	}
	sort.SliceStable(swaps, func(i, j int) bool {
		return swaps[i].index < swaps[j].index
	})
	return swaps, err
}

func (s *CacaoPoolTestSuite) txDeposit(amount, memo string, signer cosmos.AccAddress, expTxOutLen int, c *C) {
	s.addTxDeposit(common.BaseNative, amount, memo, signer, c)
	s.endBlock(expTxOutLen, c)
}

func (s *CacaoPoolTestSuite) addTxDeposit(asset common.Asset, amount, memo string, signer cosmos.AccAddress, c *C) {
	coins := common.Coins{common.NewCoin(asset, cosmos.NewUintFromString(amount))}
	msg := NewMsgDeposit(coins, memo, signer)
	err := s.handleDeposit(msg)
	c.Assert(err, IsNil)
}

func (s *CacaoPoolTestSuite) endBlock(expTxOutLen int, c *C) {
	swaps, err := s.fetchQueue()
	c.Assert(err, IsNil)
	c.Assert(swaps, HasLen, expTxOutLen)
	err = s.queue.EndBlock(s.ctx, s.mgr)
	c.Assert(err, IsNil)
	c.Check(s.mockTxOutStore.tois, HasLen, expTxOutLen)
	err = s.mgr.NetworkMgr().UpdateNetwork(s.ctx, s.mgr.GetConstants(), s.mgr.GasMgr(), s.mgr.EventMgr())
	c.Assert(err, IsNil)
	err = s.mgr.NetworkMgr().EndBlock(s.ctx, s.mgr)
	c.Assert(err, IsNil)
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
}

func (s *CacaoPoolTestSuite) getCACAOPool(c *C) (respCACAOPool openapi.CACAOPoolResponse) {
	jsonData, err := queryCACAOPool(s.ctx, s.mgr)
	c.Assert(err, IsNil)
	err = json.Unmarshal(jsonData, &respCACAOPool)
	c.Assert(err, IsNil)
	return respCACAOPool
}

func (s *CacaoPoolTestSuite) getCACAOProvider(addr common.Address, c *C) (respCACAOProvider openapi.CACAOProvider) {
	jsonData, err := queryCACAOProvider(s.ctx, []string{addr.String()}, s.mgr)
	c.Assert(err, IsNil)
	err = json.Unmarshal(jsonData, &respCACAOProvider)
	c.Assert(err, IsNil)
	return respCACAOProvider
}

func (s *CacaoPoolTestSuite) getCACAOProviders(c *C) (respCACAOProviders []openapi.CACAOProvider) {
	jsonData, err := queryCACAOProviders(s.ctx, s.mgr)
	c.Assert(err, IsNil)
	err = json.Unmarshal(jsonData, &respCACAOProviders)
	c.Assert(err, IsNil)
	return respCACAOProviders
}

func (s *CacaoPoolTestSuite) TestCACAOPoolOneProvider(c *C) {
	// CACAOPool is empty
	cp := s.getCACAOPool(c)
	p := s.getCACAOProvider(s.addrMayaFox, c)
	ps := s.getCACAOProviders(c)
	c.Assert(cp.Pol.Value, Equals, "0")
	c.Assert(cp.Reserve.Value, Equals, "0")
	c.Assert(cp.Providers.Value, Equals, "0")
	c.Assert(cp.Providers.Units, Equals, "0")
	c.Assert(p.CacaoAddress, Equals, s.addrMayaFox.String())
	c.Assert(p.DepositAmount, Equals, "0")
	c.Assert(p.Units, Equals, "0")
	c.Assert(p.Value, Equals, "0")
	c.Assert(ps, HasLen, 0)

	// deposit to CACAOPool
	s.txDeposit("1000", "pool+", s.accMayaFox, 0, c)
	cp = s.getCACAOPool(c)
	p = s.getCACAOProvider(s.addrMayaFox, c)
	ps = s.getCACAOProviders(c)
	c.Assert(cp.Pol.Value, Equals, "0")
	c.Assert(cp.Reserve.Value, Equals, "0")
	c.Assert(cp.Providers.Value, Equals, "1000")
	c.Assert(cp.Providers.Units, Equals, "1000")
	c.Assert(p.CacaoAddress, Equals, s.addrMayaFox.String())
	c.Assert(p.DepositAmount, Equals, "1000")
	c.Assert(p.Units, Equals, "1000")
	c.Assert(p.Value, Equals, "1000")
	c.Assert(ps, HasLen, 1)

	// withdraw 50%
	s.txDeposit("0", "pool-:5000", s.accMayaFox, 0, c)
	cp = s.getCACAOPool(c)
	p = s.getCACAOProvider(s.addrMayaFox, c)
	ps = s.getCACAOProviders(c)
	c.Assert(cp.Pol.Value, Equals, "0")
	c.Assert(cp.Reserve.Value, Equals, "0")
	c.Assert(cp.Providers.Value, Equals, "500")
	c.Assert(cp.Providers.Units, Equals, "500")
	c.Assert(p.CacaoAddress, Equals, s.addrMayaFox.String())
	c.Assert(p.DepositAmount, Equals, "1000")
	c.Assert(p.WithdrawAmount, Equals, "500")
	c.Assert(p.Units, Equals, "500")
	c.Assert(p.Value, Equals, "500")
	c.Assert(ps, HasLen, 1)

	// withdraw all
	s.txDeposit("0", "pool-:10000", s.accMayaFox, 0, c)
	cp = s.getCACAOPool(c)
	p = s.getCACAOProvider(s.addrMayaFox, c)
	ps = s.getCACAOProviders(c)
	c.Assert(cp.Pol.Value, Equals, "0")
	c.Assert(cp.Reserve.Value, Equals, "0")
	c.Assert(cp.Providers.Value, Equals, "0")
	c.Assert(cp.Providers.Units, Equals, "0")
	c.Assert(p.CacaoAddress, Equals, s.addrMayaFox.String())
	c.Assert(p.DepositAmount, Equals, "1000")
	c.Assert(p.WithdrawAmount, Equals, "1000")
	c.Assert(p.Units, Equals, "0")
	c.Assert(p.Value, Equals, "0")
	c.Assert(ps, HasLen, 1)

	// deposit again
	s.txDeposit("1000", "pool+", s.accMayaFox, 0, c)
	cp = s.getCACAOPool(c)
	p = s.getCACAOProvider(s.addrMayaFox, c)
	ps = s.getCACAOProviders(c)
	c.Assert(cp.Pol.Value, Equals, "0")
	c.Assert(cp.Reserve.Value, Equals, "0")
	c.Assert(cp.Providers.Value, Equals, "1000")
	c.Assert(cp.Providers.Units, Equals, "1000")
	c.Assert(p.CacaoAddress, Equals, s.addrMayaFox.String())
	c.Assert(p.DepositAmount, Equals, "2000")
	c.Assert(p.WithdrawAmount, Equals, "1000")
	c.Assert(p.Units, Equals, "1000")
	c.Assert(p.Value, Equals, "1000")
	c.Assert(ps, HasLen, 1)

	// make swap to generate swap fee for LPs
	s.txDeposit("100000000000", fmt.Sprintf("=:BTC.BTC:%s", s.addrBtcFox), s.accMayaFox, 1, c)

	cp = s.getCACAOPool(c)
	p = s.getCACAOProvider(s.addrMayaFox, c)
	ps = s.getCACAOProviders(c)
	c.Assert(cp.Pol.Value, Equals, "0")
	c.Assert(cp.Reserve.Value, Equals, "0")
	c.Assert(cp.Providers.Value, Equals, "496000") // added fees
	c.Assert(cp.Providers.Units, Equals, "1000")   // units didn't change
	c.Assert(p.CacaoAddress, Equals, s.addrMayaFox.String())
	c.Assert(p.DepositAmount, Equals, "2000")
	c.Assert(p.WithdrawAmount, Equals, "1000")
	c.Assert(p.Units, Equals, "1000")   // units didn't change
	c.Assert(p.Value, Equals, "496000") // added fees
	c.Assert(p.Pnl, Equals, "495000")   // fees - deposited
	c.Assert(ps, HasLen, 1)

	// withdraw all
	prevBal := s.mgr.Keeper().GetBalance(s.ctx, s.accMayaFox).AmountOf(common.BaseNative.Native()).BigInt().Uint64()
	s.txDeposit("0", "pool-:10000", s.accMayaFox, 0, c)
	cp = s.getCACAOPool(c)
	p = s.getCACAOProvider(s.addrMayaFox, c)
	ps = s.getCACAOProviders(c)
	c.Assert(cp.Pol.Value, Equals, "0")
	c.Assert(cp.Reserve.Value, Equals, "0")
	c.Assert(cp.Providers.Value, Equals, "0")
	c.Assert(cp.Providers.Units, Equals, "0")
	c.Assert(p.CacaoAddress, Equals, s.addrMayaFox.String())
	c.Assert(p.DepositAmount, Equals, "2000")
	c.Assert(p.WithdrawAmount, Equals, "497000")
	c.Assert(p.Units, Equals, "0")
	c.Assert(p.Value, Equals, "0")
	c.Assert(ps, HasLen, 1)
	newBal := s.mgr.Keeper().GetBalance(s.ctx, s.accMayaFox).AmountOf(common.BaseNative.Native()).BigInt().Uint64()
	cacaoFee := uint64(2000000000)
	c.Assert(newBal, Equals, prevBal-cacaoFee+496000)
}

func (s *CacaoPoolTestSuite) TestCACAOPoolTwoProviders(c *C) {
	// CACAOPool is empty
	cp := s.getCACAOPool(c)
	pFox := s.getCACAOProvider(s.addrMayaFox, c)
	pCat := s.getCACAOProvider(s.addrMayaCat, c)
	ps := s.getCACAOProviders(c)
	c.Assert(cp.Pol.Value, Equals, "0")
	c.Assert(cp.Reserve.Value, Equals, "0")
	c.Assert(cp.Providers.Value, Equals, "0")
	c.Assert(cp.Providers.Units, Equals, "0")
	c.Assert(pFox.CacaoAddress, Equals, s.addrMayaFox.String())
	c.Assert(pFox.DepositAmount, Equals, "0")
	c.Assert(pFox.Units, Equals, "0")
	c.Assert(pFox.Value, Equals, "0")
	c.Assert(pCat.CacaoAddress, Equals, s.addrMayaCat.String())
	c.Assert(pCat.DepositAmount, Equals, "0")
	c.Assert(pCat.Units, Equals, "0")
	c.Assert(pCat.Value, Equals, "0")
	c.Assert(ps, HasLen, 0)

	// fox deposits to CACAOPool
	s.txDeposit("1000", "pool+", s.accMayaFox, 0, c)
	cp = s.getCACAOPool(c)
	pFox = s.getCACAOProvider(s.addrMayaFox, c)
	pCat = s.getCACAOProvider(s.addrMayaCat, c)
	ps = s.getCACAOProviders(c)
	c.Assert(cp.Pol.Value, Equals, "0")
	c.Assert(cp.Reserve.Value, Equals, "0")
	c.Assert(cp.Providers.Value, Equals, "1000")
	c.Assert(cp.Providers.Units, Equals, "1000")
	c.Assert(pFox.CacaoAddress, Equals, s.addrMayaFox.String())
	c.Assert(pFox.DepositAmount, Equals, "1000")
	c.Assert(pFox.Units, Equals, "1000")
	c.Assert(pFox.Value, Equals, "1000")
	c.Assert(pCat.CacaoAddress, Equals, s.addrMayaCat.String())
	c.Assert(pCat.DepositAmount, Equals, "0")
	c.Assert(pCat.Units, Equals, "0")
	c.Assert(pCat.Value, Equals, "0")
	c.Assert(ps, HasLen, 1)

	// cat deposits to CACAOPool
	s.txDeposit("1000", "pool+", s.accMayaCat, 0, c)
	cp = s.getCACAOPool(c)
	pFox = s.getCACAOProvider(s.addrMayaFox, c)
	pCat = s.getCACAOProvider(s.addrMayaCat, c)
	ps = s.getCACAOProviders(c)
	c.Assert(cp.Pol.Value, Equals, "0")
	c.Assert(cp.Reserve.Value, Equals, "0")
	c.Assert(cp.Providers.Value, Equals, "2000")
	c.Assert(cp.Providers.Units, Equals, "2000")
	c.Assert(pFox.CacaoAddress, Equals, s.addrMayaFox.String())
	c.Assert(pFox.DepositAmount, Equals, "1000")
	c.Assert(pFox.Units, Equals, "1000")
	c.Assert(pFox.Value, Equals, "1000")
	c.Assert(pCat.CacaoAddress, Equals, s.addrMayaCat.String())
	c.Assert(pCat.DepositAmount, Equals, "1000")
	c.Assert(pCat.Units, Equals, "1000")
	c.Assert(pCat.Value, Equals, "1000")
	c.Assert(ps, HasLen, 2)

	// fox withdraws 50%
	s.txDeposit("0", "pool-:5000", s.accMayaFox, 0, c)
	cp = s.getCACAOPool(c)
	pFox = s.getCACAOProvider(s.addrMayaFox, c)
	pCat = s.getCACAOProvider(s.addrMayaCat, c)
	ps = s.getCACAOProviders(c)
	c.Assert(cp.Pol.Value, Equals, "0")
	c.Assert(cp.Reserve.Value, Equals, "0")
	c.Assert(cp.Providers.Value, Equals, "1500")
	c.Assert(cp.Providers.Units, Equals, "1500")
	c.Assert(pFox.CacaoAddress, Equals, s.addrMayaFox.String())
	c.Assert(pFox.DepositAmount, Equals, "1000")
	c.Assert(pFox.WithdrawAmount, Equals, "500")
	c.Assert(pFox.Units, Equals, "500")
	c.Assert(pFox.Value, Equals, "500")
	c.Assert(pCat.CacaoAddress, Equals, s.addrMayaCat.String())
	c.Assert(pCat.DepositAmount, Equals, "1000")
	c.Assert(pCat.WithdrawAmount, Equals, "0")
	c.Assert(pCat.Units, Equals, "1000")
	c.Assert(pCat.Value, Equals, "1000")
	c.Assert(ps, HasLen, 2)

	// fox withdraws all
	s.txDeposit("0", "pool-:10000", s.accMayaFox, 0, c)
	cp = s.getCACAOPool(c)
	pFox = s.getCACAOProvider(s.addrMayaFox, c)
	pCat = s.getCACAOProvider(s.addrMayaCat, c)
	ps = s.getCACAOProviders(c)
	c.Assert(cp.Pol.Value, Equals, "0")
	c.Assert(cp.Reserve.Value, Equals, "0")
	c.Assert(cp.Providers.Value, Equals, "1000")
	c.Assert(cp.Providers.Units, Equals, "1000")
	c.Assert(pFox.CacaoAddress, Equals, s.addrMayaFox.String())
	c.Assert(pFox.DepositAmount, Equals, "1000")
	c.Assert(pFox.WithdrawAmount, Equals, "1000")
	c.Assert(pFox.Units, Equals, "0")
	c.Assert(pFox.Value, Equals, "0")
	c.Assert(pCat.CacaoAddress, Equals, s.addrMayaCat.String())
	c.Assert(pCat.DepositAmount, Equals, "1000")
	c.Assert(pCat.WithdrawAmount, Equals, "0")
	c.Assert(pCat.Units, Equals, "1000")
	c.Assert(pCat.Value, Equals, "1000")
	c.Assert(ps, HasLen, 2)

	// deposit again
	s.txDeposit("1000", "pool+", s.accMayaFox, 0, c)
	cp = s.getCACAOPool(c)
	pFox = s.getCACAOProvider(s.addrMayaFox, c)
	pCat = s.getCACAOProvider(s.addrMayaCat, c)
	ps = s.getCACAOProviders(c)
	c.Assert(cp.Pol.Value, Equals, "0")
	c.Assert(cp.Reserve.Value, Equals, "0")
	c.Assert(cp.Providers.Value, Equals, "2000")
	c.Assert(cp.Providers.Units, Equals, "2000")
	c.Assert(pFox.CacaoAddress, Equals, s.addrMayaFox.String())
	c.Assert(pFox.DepositAmount, Equals, "2000")
	c.Assert(pFox.WithdrawAmount, Equals, "1000")
	c.Assert(pFox.Units, Equals, "1000")
	c.Assert(pFox.Value, Equals, "1000")
	c.Assert(pCat.CacaoAddress, Equals, s.addrMayaCat.String())
	c.Assert(pCat.DepositAmount, Equals, "1000")
	c.Assert(pCat.WithdrawAmount, Equals, "0")
	c.Assert(pCat.Units, Equals, "1000")
	c.Assert(pCat.Value, Equals, "1000")
	c.Assert(ps, HasLen, 2)

	// make swap to generate swap fee for LPs
	s.txDeposit("100000000000", fmt.Sprintf("=:BTC.BTC:%s", s.addrBtcFox), s.accMayaFox, 1, c)

	cp = s.getCACAOPool(c)
	pFox = s.getCACAOProvider(s.addrMayaFox, c)
	pCat = s.getCACAOProvider(s.addrMayaCat, c)
	ps = s.getCACAOProviders(c)
	c.Assert(cp.Pol.Value, Equals, "0")
	c.Assert(cp.Reserve.Value, Equals, "0")
	c.Assert(cp.Providers.Value, Equals, "497000") // added fees (495000) for both fox & cat
	c.Assert(cp.Providers.Units, Equals, "2000")   // units didn't change
	c.Assert(pFox.CacaoAddress, Equals, s.addrMayaFox.String())
	c.Assert(pFox.DepositAmount, Equals, "2000")
	c.Assert(pFox.WithdrawAmount, Equals, "1000")
	c.Assert(pFox.Units, Equals, "1000")   // units didn't change
	c.Assert(pFox.Value, Equals, "248500") // added fees
	c.Assert(pFox.Pnl, Equals, "247500")   // fees - deposited
	c.Assert(pCat.CacaoAddress, Equals, s.addrMayaCat.String())
	c.Assert(pCat.DepositAmount, Equals, "1000")
	c.Assert(pCat.WithdrawAmount, Equals, "0")
	c.Assert(pCat.Units, Equals, "1000")
	c.Assert(pCat.Value, Equals, "248500")
	c.Assert(pCat.Pnl, Equals, "247500")
	c.Assert(ps, HasLen, 2)

	// fox withdraws all
	// cat withdraws 50%
	prevBalFox := s.mgr.Keeper().GetBalance(s.ctx, s.accMayaFox).AmountOf(common.BaseNative.Native()).BigInt().Uint64()
	prevBalCat := s.mgr.Keeper().GetBalance(s.ctx, s.accMayaCat).AmountOf(common.BaseNative.Native()).BigInt().Uint64()
	s.txDeposit("0", "pool-:10000", s.accMayaFox, 0, c)
	s.txDeposit("0", "pool-:5000", s.accMayaCat, 0, c)
	cp = s.getCACAOPool(c)
	pFox = s.getCACAOProvider(s.addrMayaFox, c)
	pCat = s.getCACAOProvider(s.addrMayaCat, c)
	ps = s.getCACAOProviders(c)
	c.Assert(cp.Pol.Value, Equals, "0")
	c.Assert(cp.Reserve.Value, Equals, "0")
	c.Assert(cp.Providers.Value, Equals, "124250")
	c.Assert(cp.Providers.Units, Equals, "500")
	c.Assert(pFox.CacaoAddress, Equals, s.addrMayaFox.String())
	c.Assert(pFox.DepositAmount, Equals, "2000")
	c.Assert(pFox.WithdrawAmount, Equals, "249500")
	c.Assert(pFox.Units, Equals, "0")
	c.Assert(pFox.Value, Equals, "0")
	c.Assert(pFox.Pnl, Equals, "247500")
	c.Assert(pCat.CacaoAddress, Equals, s.addrMayaCat.String())
	c.Assert(pCat.DepositAmount, Equals, "1000")
	c.Assert(pCat.WithdrawAmount, Equals, "124250")
	c.Assert(pCat.Units, Equals, "500")
	c.Assert(pCat.Value, Equals, "124250")
	c.Assert(pCat.Pnl, Equals, "247500")
	c.Assert(ps, HasLen, 2)
	newBalFox := s.mgr.Keeper().GetBalance(s.ctx, s.accMayaFox).AmountOf(common.BaseNative.Native()).BigInt().Uint64()
	newBalCat := s.mgr.Keeper().GetBalance(s.ctx, s.accMayaCat).AmountOf(common.BaseNative.Native()).BigInt().Uint64()
	cacaoFee := uint64(2000000000)
	c.Assert(newBalFox, Equals, prevBalFox-cacaoFee+248500)
	c.Assert(newBalCat, Equals, prevBalCat-cacaoFee+124250)
}

func (s *CacaoPoolTestSuite) TestCACAOPoolWithdrawAffiliates(c *C) {
	var err error
	var aAcc, bAcc, cAcc cosmos.AccAddress
	aAddr := GetRandomBaseAddress()
	bAddr := GetRandomBaseAddress()
	cAddr := GetRandomBaseAddress()
	aAcc, err = aAddr.AccAddress()
	c.Assert(err, IsNil)
	bAcc, err = bAddr.AccAddress()
	c.Assert(err, IsNil)
	cAcc, err = cAddr.AccAddress()
	c.Assert(err, IsNil)
	s.txDeposit("110000000000", fmt.Sprintf("~:a:MAYA:%s", aAddr), s.signer, 0, c)
	s.txDeposit("110000000000", fmt.Sprintf("~:c:MAYA:%s", cAddr), s.signer, 0, c)
	s.txDeposit("110000000000", fmt.Sprintf("~:b:MAYA:%s:::::c:4000", bAddr), s.signer, 0, c)

	// CACAOPool is empty
	cp := s.getCACAOPool(c)
	p := s.getCACAOProvider(s.addrMayaFox, c)
	ps := s.getCACAOProviders(c)
	c.Assert(cp.Pol.Value, Equals, "0")
	c.Assert(cp.Reserve.Value, Equals, "0")
	c.Assert(cp.Providers.Value, Equals, "0")
	c.Assert(cp.Providers.Units, Equals, "0")
	c.Assert(p.CacaoAddress, Equals, s.addrMayaFox.String())
	c.Assert(p.DepositAmount, Equals, "0")
	c.Assert(p.Units, Equals, "0")
	c.Assert(p.Value, Equals, "0")
	c.Assert(ps, HasLen, 0)

	// deposit to CACAOPool
	s.txDeposit("1000", "pool+", s.accMayaFox, 0, c)
	cp = s.getCACAOPool(c)
	p = s.getCACAOProvider(s.addrMayaFox, c)
	ps = s.getCACAOProviders(c)
	c.Assert(cp.Pol.Value, Equals, "0")
	c.Assert(cp.Reserve.Value, Equals, "0")
	c.Assert(cp.Providers.Value, Equals, "1000")
	c.Assert(cp.Providers.Units, Equals, "1000")
	c.Assert(p.CacaoAddress, Equals, s.addrMayaFox.String())
	c.Assert(p.DepositAmount, Equals, "1000")
	c.Assert(p.Units, Equals, "1000")
	c.Assert(p.Value, Equals, "1000")
	c.Assert(ps, HasLen, 1)

	// withdraw 50%
	s.txDeposit("0", "pool-:5000:a/b:3000/4000", s.accMayaFox, 0, c)
	cp = s.getCACAOPool(c)
	p = s.getCACAOProvider(s.addrMayaFox, c)
	ps = s.getCACAOProviders(c)
	c.Assert(cp.Pol.Value, Equals, "0")
	c.Assert(cp.Reserve.Value, Equals, "0")
	c.Assert(cp.Providers.Value, Equals, "500")
	c.Assert(cp.Providers.Units, Equals, "500")
	c.Assert(p.CacaoAddress, Equals, s.addrMayaFox.String())
	c.Assert(p.DepositAmount, Equals, "1000")
	c.Assert(p.WithdrawAmount, Equals, "500")
	c.Assert(p.Units, Equals, "500")
	c.Assert(p.Value, Equals, "500")
	c.Assert(ps, HasLen, 1)

	// no yield -> no affiliate yield
	aBal := s.mgr.Keeper().GetBalance(s.ctx, aAcc).AmountOf(common.BaseNative.Native()).BigInt().Uint64()
	bBal := s.mgr.Keeper().GetBalance(s.ctx, bAcc).AmountOf(common.BaseNative.Native()).BigInt().Uint64()
	cBal := s.mgr.Keeper().GetBalance(s.ctx, cAcc).AmountOf(common.BaseNative.Native()).BigInt().Uint64()
	c.Assert(aBal, Equals, uint64(0))
	c.Assert(bBal, Equals, uint64(0))
	c.Assert(cBal, Equals, uint64(0))

	// withdraw all
	s.txDeposit("0", "pool-:10000:a/b:3000/4000", s.accMayaFox, 0, c)
	cp = s.getCACAOPool(c)
	p = s.getCACAOProvider(s.addrMayaFox, c)
	ps = s.getCACAOProviders(c)
	c.Assert(cp.Pol.Value, Equals, "0")
	c.Assert(cp.Reserve.Value, Equals, "0")
	c.Assert(cp.Providers.Value, Equals, "0")
	c.Assert(cp.Providers.Units, Equals, "0")
	c.Assert(p.CacaoAddress, Equals, s.addrMayaFox.String())
	c.Assert(p.DepositAmount, Equals, "1000")
	c.Assert(p.WithdrawAmount, Equals, "1000")
	c.Assert(p.Units, Equals, "0")
	c.Assert(p.Value, Equals, "0")
	c.Assert(ps, HasLen, 1)
	// no yield -> no affiliate yield
	aBal = s.mgr.Keeper().GetBalance(s.ctx, aAcc).AmountOf(common.BaseNative.Native()).BigInt().Uint64()
	bBal = s.mgr.Keeper().GetBalance(s.ctx, bAcc).AmountOf(common.BaseNative.Native()).BigInt().Uint64()
	cBal = s.mgr.Keeper().GetBalance(s.ctx, cAcc).AmountOf(common.BaseNative.Native()).BigInt().Uint64()
	c.Assert(aBal, Equals, uint64(0))
	c.Assert(bBal, Equals, uint64(0))
	c.Assert(cBal, Equals, uint64(0))

	// deposit again
	s.txDeposit("1000", "pool+", s.accMayaFox, 0, c)
	cp = s.getCACAOPool(c)
	p = s.getCACAOProvider(s.addrMayaFox, c)
	ps = s.getCACAOProviders(c)
	c.Assert(cp.Pol.Value, Equals, "0")
	c.Assert(cp.Reserve.Value, Equals, "0")
	c.Assert(cp.Providers.Value, Equals, "1000")
	c.Assert(cp.Providers.Units, Equals, "1000")
	c.Assert(p.CacaoAddress, Equals, s.addrMayaFox.String())
	c.Assert(p.DepositAmount, Equals, "2000")
	c.Assert(p.WithdrawAmount, Equals, "1000")
	c.Assert(p.Units, Equals, "1000")
	c.Assert(p.Value, Equals, "1000")
	c.Assert(ps, HasLen, 1)

	// make swap to generate swap fee for LPs
	s.txDeposit("100000000000", fmt.Sprintf("=:BTC.BTC:%s", s.addrBtcFox), s.accMayaFox, 1, c)

	cp = s.getCACAOPool(c)
	p = s.getCACAOProvider(s.addrMayaFox, c)
	ps = s.getCACAOProviders(c)
	c.Assert(cp.Pol.Value, Equals, "0")
	c.Assert(cp.Reserve.Value, Equals, "0")
	c.Assert(cp.Providers.Value, Equals, "496000") // added fees
	c.Assert(cp.Providers.Units, Equals, "1000")   // units didn't change
	c.Assert(p.CacaoAddress, Equals, s.addrMayaFox.String())
	c.Assert(p.DepositAmount, Equals, "2000")
	c.Assert(p.WithdrawAmount, Equals, "1000")
	c.Assert(p.Units, Equals, "1000")   // units didn't change
	c.Assert(p.Value, Equals, "496000") // added fees
	c.Assert(p.Pnl, Equals, "495000")   // fees - deposited
	c.Assert(ps, HasLen, 1)

	var yieldInt int
	yieldInt, err = strconv.Atoi(p.Pnl)
	c.Assert(err, IsNil)
	yield := uint64(yieldInt)

	// withdraw 50%
	s.txDeposit("0", "pool-:5000:a/b:3000/4000", s.accMayaFox, 0, c)
	cp = s.getCACAOPool(c)
	p = s.getCACAOProvider(s.addrMayaFox, c)
	ps = s.getCACAOProviders(c)
	c.Assert(cp.Pol.Value, Equals, "0")
	c.Assert(cp.Reserve.Value, Equals, "0")
	c.Assert(cp.Providers.Value, Equals, "248000")
	c.Assert(cp.Providers.Units, Equals, "500")
	c.Assert(p.CacaoAddress, Equals, s.addrMayaFox.String())
	c.Assert(p.DepositAmount, Equals, "2000")
	c.Assert(p.WithdrawAmount, Equals, "249000")
	c.Assert(p.Units, Equals, "500")
	c.Assert(p.Value, Equals, "248000")
	c.Assert(p.Pnl, Equals, "495000")
	c.Assert(ps, HasLen, 1)

	aBal = s.mgr.Keeper().GetBalance(s.ctx, aAcc).AmountOf(common.BaseNative.Native()).BigInt().Uint64()
	bBal = s.mgr.Keeper().GetBalance(s.ctx, bAcc).AmountOf(common.BaseNative.Native()).BigInt().Uint64()
	cBal = s.mgr.Keeper().GetBalance(s.ctx, cAcc).AmountOf(common.BaseNative.Native()).BigInt().Uint64()

	// only 50% withdrawn
	yield /= 2

	c.Assert(aBal, Equals, yield*30/100) // 30% aff
	c.Assert(bBal, Equals, yield*24/100) // 24% aff = 60% of 40%
	c.Assert(cBal, Equals, yield*16/100) // 16% aff = 40% of 40%
	aPrevBal, bPrevBal, cPrevBal := aBal, bBal, cBal

	// withdraw all
	s.txDeposit("0", "pool-:10000:a/b:3000/4000", s.accMayaFox, 0, c)
	cp = s.getCACAOPool(c)
	p = s.getCACAOProvider(s.addrMayaFox, c)
	ps = s.getCACAOProviders(c)
	c.Assert(cp.Pol.Value, Equals, "0")
	c.Assert(cp.Reserve.Value, Equals, "0")
	c.Assert(cp.Providers.Value, Equals, "0")
	c.Assert(cp.Providers.Units, Equals, "0")
	c.Assert(p.CacaoAddress, Equals, s.addrMayaFox.String())
	c.Assert(p.DepositAmount, Equals, "2000")
	c.Assert(p.WithdrawAmount, Equals, "497000")
	c.Assert(p.Units, Equals, "0")
	c.Assert(p.Value, Equals, "0")
	c.Assert(ps, HasLen, 1)

	aBal = s.mgr.Keeper().GetBalance(s.ctx, aAcc).AmountOf(common.BaseNative.Native()).BigInt().Uint64()
	bBal = s.mgr.Keeper().GetBalance(s.ctx, bAcc).AmountOf(common.BaseNative.Native()).BigInt().Uint64()
	cBal = s.mgr.Keeper().GetBalance(s.ctx, cAcc).AmountOf(common.BaseNative.Native()).BigInt().Uint64()
	yield = 248000
	c.Assert(aBal-aPrevBal, Equals, yield*30/100) // 30% aff
	c.Assert(bBal-bPrevBal, Equals, yield*24/100) // 24% aff = 60% of 40%
	c.Assert(cBal-cPrevBal, Equals, yield*16/100) // 16% aff = 40% of 40%
}
