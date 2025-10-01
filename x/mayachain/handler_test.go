package mayachain

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/simapp"
	"github.com/cosmos/cosmos-sdk/store"
	sdk "github.com/cosmos/cosmos-sdk/types"
	se "github.com/cosmos/cosmos-sdk/types/errors"
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
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	dbm "github.com/tendermint/tm-db"
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"

	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
	kv1 "gitlab.com/mayachain/mayanode/x/mayachain/keeper/v1"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

var errKaboom = errors.New("kaboom")

type HandlerSuite struct{}

var _ = Suite(&HandlerSuite{})

func (s *HandlerSuite) SetUpSuite(*C) {
	SetupConfigForTest()
}

func FundModule(c *C, ctx cosmos.Context, k keeper.Keeper, name string, amt uint64) {
	coin := common.NewCoin(common.BaseNative, cosmos.NewUint(amt*common.One))
	err := k.MintToModule(ctx, ModuleName, coin)
	c.Assert(err, IsNil)
	err = k.SendFromModuleToModule(ctx, ModuleName, name, common.NewCoins(coin))
	c.Assert(err, IsNil)
}

func FundAccount(c *C, ctx cosmos.Context, k keeper.Keeper, addr cosmos.AccAddress, amt uint64) {
	coin := common.NewCoin(common.BaseNative, cosmos.NewUint(amt*common.One))
	err := k.MintToModule(ctx, ModuleName, coin)
	c.Assert(err, IsNil)
	err = k.SendFromModuleToAccount(ctx, ModuleName, addr, common.NewCoins(coin))
	c.Assert(err, IsNil)
}

// nolint: deadcode unused
// create a codec used only for testing
func makeTestCodec() *codec.LegacyAmino {
	return types.MakeTestCodec()
}

var keyThorchain = cosmos.NewKVStoreKey(StoreKey)

func setupManagerForTest(c *C) (cosmos.Context, *Mgrs) {
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
	FundModule(c, ctx, k, ModuleName, 100_000_000*common.One)
	FundModule(c, ctx, k, AsgardName, 100*common.One)
	FundModule(c, ctx, k, ReserveName, 1_000_000*common.One)
	c.Assert(k.SaveNetworkFee(ctx, common.BNBChain, NetworkFee{
		Chain:              common.BNBChain,
		TransactionSize:    1,
		TransactionFeeRate: 37500,
	}), IsNil)
	c.Assert(k.SaveNetworkFee(ctx, common.THORChain, NetworkFee{
		Chain:              common.THORChain,
		TransactionSize:    1,
		TransactionFeeRate: 2_000_000,
	}), IsNil)

	os.Setenv("NET", "mocknet")
	mgr := NewManagers(k, marshaler, bk, ak, ibck, keyThorchain)
	constants.SWVersion = GetCurrentVersion()

	_, hasVerStored := k.GetVersionWithCtx(ctx)
	c.Assert(hasVerStored, Equals, false,
		Commentf("version should not be stored until BeginBlock"))

	c.Assert(mgr.BeginBlock(ctx), IsNil)
	mgr.gasMgr.BeginBlock(mgr)

	verStored, hasVerStored := k.GetVersionWithCtx(ctx)
	c.Assert(hasVerStored, Equals, true,
		Commentf("version should be stored"))
	verComputed := k.GetLowestActiveVersion(ctx)
	c.Assert(verStored.String(), Equals, verComputed.String(),
		Commentf("stored version should match computed version"))

	return ctx, mgr
}

func setupKeeperForTest(c *C) (cosmos.Context, keeper.Keeper) {
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
	k := kv1.NewKVStore(marshaler, bk, ak, ibck, keyThorchain, GetCurrentVersion())
	FundModule(c, ctx, k, ModuleName, 1000000*common.One)
	FundModule(c, ctx, k, AsgardName, common.One)
	FundModule(c, ctx, k, ReserveName, 10000*common.One)
	err = k.SaveNetworkFee(ctx, common.BNBChain, NetworkFee{
		Chain:              common.BNBChain,
		TransactionSize:    1,
		TransactionFeeRate: 37500,
	})
	c.Assert(err, IsNil)
	os.Setenv("NET", "mocknet")
	return ctx, k
}

func SetupLiquidityBondForTest(c *C, ctx cosmos.Context, k keeper.Keeper, asset common.Asset, addr common.Address, na NodeAccount, bond cosmos.Uint) (LiquidityProvider, cosmos.Uint) {
	pk := na.PubKeySet.Secp256k1
	assetAddr, err := pk.GetAddress(asset.GetChain())
	c.Assert(err, IsNil)
	lp := LiquidityProvider{
		Asset:           asset,
		CacaoAddress:    addr,
		AssetAddress:    assetAddr,
		Units:           bond,
		NodeBondAddress: na.NodeAddress,
		LastAddHeight:   1,
	}
	k.SetLiquidityProvider(ctx, lp)
	pool, err := k.GetPool(ctx, asset)
	c.Assert(err, IsNil)
	if pool.IsEmpty() {
		pool = Pool{
			BalanceCacao: bond,
			BalanceAsset: bond,
			Asset:        asset,
			LPUnits:      bond,
			Status:       PoolAvailable,
		}
	} else {
		pool.BalanceAsset = pool.BalanceAsset.Add(bond)
		pool.BalanceCacao = pool.BalanceCacao.Add(bond)
		pool.LPUnits = pool.LPUnits.Add(bond)
		pool.Status = PoolAvailable
	}
	c.Assert(k.SetPool(ctx, pool), IsNil)
	k.SetLiquidityProvider(ctx, lp)

	calcBond := common.GetSafeShare(lp.Units, pool.LPUnits, pool.BalanceCacao)
	return lp, calcBond
}

func SetupLiquidityBondForTestV105(c *C, ctx cosmos.Context, k keeper.Keeper, asset common.Asset, addr common.Address, na NodeAccount, bond cosmos.Uint) (LiquidityProvider, cosmos.Uint) {
	pk := na.PubKeySet.Secp256k1
	assetAddr, err := pk.GetAddress(asset.GetChain())
	c.Assert(err, IsNil)
	lp := LiquidityProvider{
		Asset:        asset,
		CacaoAddress: addr,
		AssetAddress: assetAddr,
		Units:        bond,
		BondedNodes: []types.LPBondedNode{
			{
				NodeAddress: na.NodeAddress,
				Units:       bond,
			},
		},
		LastAddHeight: 1,
	}
	k.SetLiquidityProvider(ctx, lp)
	pool, err := k.GetPool(ctx, asset)
	c.Assert(err, IsNil)
	if pool.IsEmpty() {
		pool = Pool{
			BalanceCacao: bond,
			BalanceAsset: bond,
			Asset:        asset,
			LPUnits:      bond,
			Status:       PoolAvailable,
		}
	} else {
		pool.BalanceAsset = pool.BalanceAsset.Add(bond)
		pool.BalanceCacao = pool.BalanceCacao.Add(bond)
		pool.LPUnits = pool.LPUnits.Add(bond)
		pool.Status = PoolAvailable
	}
	c.Assert(k.SetPool(ctx, pool), IsNil)
	k.SetLiquidityProvider(ctx, lp)

	calcBond := common.GetSafeShare(lp.Units, pool.LPUnits, pool.BalanceCacao)
	return lp, calcBond
}

type handlerTestWrapper struct {
	ctx                  cosmos.Context
	keeper               keeper.Keeper
	mgr                  Manager
	activeNodeAccount    NodeAccount
	notActiveNodeAccount NodeAccount
}

func getHandlerTestWrapper(c *C, height int64, withActiveNode, withActieBNBPool bool) handlerTestWrapper {
	ctx, mgr := setupManagerForTest(c)
	ctx = ctx.WithBlockHeight(height)
	acc1 := GetRandomValidatorNode(NodeActive)
	acc1.Version = mgr.GetVersion().String()
	if withActiveNode {
		c.Assert(mgr.Keeper().SetNodeAccount(ctx, acc1), IsNil)
	}
	if withActieBNBPool {
		p, err := mgr.Keeper().GetPool(ctx, common.BNBAsset)
		c.Assert(err, IsNil)
		p.Asset = common.BNBAsset
		p.Status = PoolAvailable
		p.BalanceCacao = cosmos.NewUint(100 * common.One)
		p.BalanceAsset = cosmos.NewUint(100 * common.One)
		p.LPUnits = cosmos.NewUint(100 * common.One)
		c.Assert(mgr.Keeper().SetPool(ctx, p), IsNil)
	}

	FundModule(c, ctx, mgr.Keeper(), AsgardName, 100000000)

	c.Assert(mgr.ValidatorMgr().BeginBlock(ctx, mgr, nil), IsNil)

	return handlerTestWrapper{
		ctx:                  ctx,
		keeper:               mgr.Keeper(),
		mgr:                  mgr,
		activeNodeAccount:    acc1,
		notActiveNodeAccount: GetRandomValidatorNode(NodeDisabled),
	}
}

func (HandlerSuite) TestHandleTxInWithdrawLiquidityMemo(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)

	vault := GetRandomVault()
	vault.Coins = common.Coins{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(100*common.One)),
		common.NewCoin(common.BaseAsset(), cosmos.NewUint(100*common.One)),
	}
	c.Assert(w.keeper.SetVault(w.ctx, vault), IsNil)
	vaultAddr, err := vault.PubKey.GetAddress(common.BNBChain)

	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.BalanceCacao = cosmos.NewUint(100 * common.One)
	pool.LPUnits = cosmos.NewUint(100)
	c.Assert(w.keeper.SetPool(w.ctx, pool), IsNil)

	runeAddr := GetRandomBaseAddress()
	lp := LiquidityProvider{
		Asset:        common.BNBAsset,
		CacaoAddress: runeAddr,
		AssetAddress: GetRandomBNBAddress(),
		PendingCacao: cosmos.ZeroUint(),
		Units:        cosmos.NewUint(100),
	}
	w.keeper.SetLiquidityProvider(w.ctx, lp)

	// sym withdrawal from external chain
	tx := common.Tx{
		ID:    GetRandomTxHash(),
		Chain: common.BNBChain,
		Coins: common.Coins{
			common.NewCoin(common.BaseAsset(), cosmos.NewUint(1*common.One)),
		},
		Memo:        "withdraw:BNB.BNB",
		FromAddress: lp.AssetAddress,
		ToAddress:   vaultAddr,
		Gas:         BNBGasFeeSingleton,
	}

	msg := NewMsgWithdrawLiquidity(tx, lp.CacaoAddress, cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), common.BNBAsset, common.EmptyAsset, w.activeNodeAccount.NodeAddress)
	c.Assert(err, IsNil)

	handler := NewInternalHandler(w.mgr)

	FundModule(c, w.ctx, w.keeper, AsgardName, 500)
	c.Assert(w.keeper.SaveNetworkFee(w.ctx, common.BNBChain, NetworkFee{
		Chain:              common.BNBChain,
		TransactionSize:    1,
		TransactionFeeRate: bnbSingleTxFee.Uint64(),
	}), IsNil)

	_, err = handler(w.ctx, msg)
	c.Assert(err, IsNil)
	pool, err = w.keeper.GetPool(w.ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Assert(pool.IsEmpty(), Equals, false)
	c.Check(pool.Status, Equals, PoolStaged)
	c.Check(pool.LPUnits.Uint64(), Equals, uint64(0), Commentf("%d", pool.LPUnits.Uint64()))
	c.Check(pool.BalanceCacao.Uint64(), Equals, uint64(0), Commentf("%d", pool.BalanceCacao.Uint64()))
	remainGas := uint64(37500)
	c.Check(pool.BalanceAsset.Uint64(), Equals, remainGas, Commentf("%d", pool.BalanceAsset.Uint64())) // leave a little behind for gas
}

func (HandlerSuite) TestRefund(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)

	pool := Pool{
		Asset:        common.BNBAsset,
		BalanceCacao: cosmos.NewUint(100 * common.One),
		BalanceAsset: cosmos.NewUint(100 * common.One),
	}
	c.Assert(w.keeper.SetPool(w.ctx, pool), IsNil)

	vault := GetRandomVault()
	c.Assert(w.keeper.SetVault(w.ctx, vault), IsNil)

	txin := NewObservedTx(
		common.Tx{
			ID:    GetRandomTxHash(),
			Chain: common.BNBChain,
			Coins: common.Coins{
				common.NewCoin(common.BNBAsset, cosmos.NewUint(100*common.One)),
			},
			Memo:        "withdraw:BNB.BNB",
			FromAddress: GetRandomBNBAddress(),
			ToAddress:   GetRandomBNBAddress(),
			Gas:         BNBGasFeeSingleton,
		},
		1024,
		vault.PubKey, 1024,
	)
	txOutStore := w.mgr.TxOutStore()
	c.Assert(refundTx(w.ctx, txin, w.mgr, 0, "refund", ""), IsNil)
	items, err := txOutStore.GetOutboundItems(w.ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 1)

	// check THORNode DONT create a refund transaction when THORNode don't have a pool for
	// the asset sent.
	lokiAsset, _ := common.NewAsset("BNB.LOKI")
	txin.Tx.Coins = common.Coins{
		common.NewCoin(lokiAsset, cosmos.NewUint(100*common.One)),
	}

	c.Assert(refundTx(w.ctx, txin, w.mgr, 0, "refund", ""), IsNil)
	items, err = txOutStore.GetOutboundItems(w.ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 1)

	pool, err = w.keeper.GetPool(w.ctx, lokiAsset)
	c.Assert(err, IsNil)
	// pool should be zero since we drop coins we don't recognize on the floor
	c.Assert(pool.BalanceAsset.Equal(cosmos.ZeroUint()), Equals, true, Commentf("%d", pool.BalanceAsset.Uint64()))

	// doing it a second time should keep it at zero
	c.Assert(refundTx(w.ctx, txin, w.mgr, 0, "refund", ""), IsNil)
	items, err = txOutStore.GetOutboundItems(w.ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 1)
	pool, err = w.keeper.GetPool(w.ctx, lokiAsset)
	c.Assert(err, IsNil)
	c.Assert(pool.BalanceAsset.Equal(cosmos.ZeroUint()), Equals, true)
}

func (HandlerSuite) TestGetMsgSwapFromMemo(c *C) {
	m, err := ParseMemo(GetCurrentVersion(), "swap:BNB.BNB")
	swapMemo, ok := m.(SwapMemo)
	c.Assert(ok, Equals, true)
	c.Assert(err, IsNil)

	txin := types.NewObservedTx(
		common.Tx{
			ID:    GetRandomTxHash(),
			Chain: common.BNBChain,
			Coins: common.Coins{
				common.NewCoin(
					common.BaseAsset(),
					cosmos.NewUint(100*common.One),
				),
			},
			Memo:        m.String(),
			FromAddress: GetRandomBNBAddress(),
			ToAddress:   GetRandomBNBAddress(),
			Gas:         BNBGasFeeSingleton,
		},
		1024,
		common.EmptyPubKey, 1024,
	)

	resultMsg1, err := getMsgSwapFromMemo(swapMemo, txin, GetRandomBech32Addr())
	c.Assert(resultMsg1, NotNil)
	c.Assert(err, IsNil)
}

func (HandlerSuite) TestGetMsgWithdrawFromMemo(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	tx := GetRandomTx()
	tx.Memo = "withdraw:BTC.BTC:10000"
	if common.BaseAsset().Equals(common.BaseNative) {
		tx.FromAddress = GetRandomBaseAddress()
	}
	obTx := NewObservedTx(tx, w.ctx.BlockHeight(), GetRandomPubKey(), w.ctx.BlockHeight())
	msg, err := processOneTxIn(w.ctx, GetCurrentVersion(), w.keeper, obTx, w.activeNodeAccount.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(msg, NotNil)
	_, isWithdraw := msg.(*MsgWithdrawLiquidity)
	c.Assert(isWithdraw, Equals, true)

	// symmetric lp withdrawing asymmetrically from external chain
	tx.Memo = "withdraw:BTC.BTC:10000:BTC.BTC:tmaya16xxn0cadruuw6a2qwpv35av0mehryvdzz9uate"
	tx.FromAddress = GetRandomBTCAddress()
	tx.Chain = common.BTCChain

	obTx = NewObservedTx(tx, w.ctx.BlockHeight(), GetRandomPubKey(), w.ctx.BlockHeight())
	msg, err = processOneTxIn(w.ctx, GetCurrentVersion(), w.keeper, obTx, w.activeNodeAccount.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(msg, NotNil)
	wmsg, isWithdraw := msg.(*MsgWithdrawLiquidity)
	c.Assert(isWithdraw, Equals, true)
	c.Assert(wmsg.WithdrawAddress.String(), Equals, "tmaya16xxn0cadruuw6a2qwpv35av0mehryvdzz9uate")

	// wrong pairaddress
	tx.Memo = "withdraw:BTC.BTC:10000:BTC.BTC:bc1qwqdg6squsna38e46795at95yu9atm8azzmyvckulcc7kytlcckxswvvzej"
	tx.FromAddress = GetRandomBTCAddress()

	obTx = NewObservedTx(tx, w.ctx.BlockHeight(), GetRandomPubKey(), w.ctx.BlockHeight())
	msg, err = processOneTxIn(w.ctx, GetCurrentVersion(), w.keeper, obTx, w.activeNodeAccount.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(msg, NotNil)
	wmsg, isWithdraw = msg.(*MsgWithdrawLiquidity)
	c.Assert(isWithdraw, Equals, true)
	c.Assert(wmsg.WithdrawAddress.String(), Equals, tx.FromAddress.String())

	// pairaddress ignores origin is BASEChain
	tx.Memo = "withdraw:BTC.BTC:10000:BTC.BTC:tmaya16xxn0cadruuw6a2qwpv35av0mehryvdzz9uate"
	tx.FromAddress = GetRandomBaseAddress()

	obTx = NewObservedTx(tx, w.ctx.BlockHeight(), GetRandomPubKey(), w.ctx.BlockHeight())
	msg, err = processOneTxIn(w.ctx, GetCurrentVersion(), w.keeper, obTx, w.activeNodeAccount.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(msg, NotNil)
	wmsg, isWithdraw = msg.(*MsgWithdrawLiquidity)
	c.Assert(isWithdraw, Equals, true)
	c.Assert(wmsg.WithdrawAddress.String(), Equals, tx.FromAddress.String())
}

func (HandlerSuite) TestGetMsgMigrationFromMemo(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	tx := GetRandomTx()
	tx.Memo = "migrate:10"
	obTx := NewObservedTx(tx, w.ctx.BlockHeight(), GetRandomPubKey(), w.ctx.BlockHeight())
	msg, err := processOneTxIn(w.ctx, GetCurrentVersion(), w.keeper, obTx, w.activeNodeAccount.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(msg, NotNil)
	_, isMigrate := msg.(*MsgMigrate)
	c.Assert(isMigrate, Equals, true)
}

func (HandlerSuite) TestGetMsgBondFromMemo(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	tx := GetRandomTx()
	tx.Coins = common.Coins{
		common.NewCoin(common.BaseAsset(), cosmos.NewUint(100*common.One)),
	}
	tx.Memo = fmt.Sprintf("bond:%s:%d:%s", common.BNBAsset, 1000, GetRandomBech32Addr().String())
	obTx := NewObservedTx(tx, w.ctx.BlockHeight(), GetRandomPubKey(), w.ctx.BlockHeight())
	msg, err := processOneTxIn(w.ctx, GetCurrentVersion(), w.keeper, obTx, w.activeNodeAccount.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(msg, NotNil)
	_, isBond := msg.(*MsgBond)
	c.Assert(isBond, Equals, true)
}

func (HandlerSuite) TestGetMsgUnBondFromMemo(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	tx := GetRandomTx()
	tx.Coins = common.Coins{
		common.NewCoin(common.BaseAsset(), cosmos.NewUint(100*common.One)),
	}
	tx.Memo = "unbond:::" + GetRandomBaseAddress().String()
	obTx := NewObservedTx(tx, w.ctx.BlockHeight(), GetRandomPubKey(), w.ctx.BlockHeight())
	msg, err := processOneTxIn(w.ctx, GetCurrentVersion(), w.keeper, obTx, w.activeNodeAccount.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(msg, NotNil)
	_, isUnBond := msg.(*MsgUnBond)
	c.Assert(isUnBond, Equals, true)
}

func (HandlerSuite) TestGetMsgLiquidityFromMemo(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	// provide BNB, however THORNode send T-CAN as coin , which is incorrect, should result in an error
	m, err := ParseMemo(GetCurrentVersion(), fmt.Sprintf("add:BNB.BNB:%s", GetRandomBaseAddress()))
	c.Assert(err, IsNil)
	lpMemo, ok := m.(AddLiquidityMemo)
	c.Assert(ok, Equals, true)
	tcanAsset, err := common.NewAsset("BNB.TCAN-014")
	c.Assert(err, IsNil)
	baseAsset := common.BaseAsset()
	c.Assert(err, IsNil)

	txin := types.NewObservedTx(
		common.Tx{
			ID:    GetRandomTxHash(),
			Chain: common.BNBChain,
			Coins: common.Coins{
				common.NewCoin(tcanAsset,
					cosmos.NewUint(100*common.One)),
				common.NewCoin(baseAsset,
					cosmos.NewUint(100*common.One)),
			},
			Memo:        m.String(),
			FromAddress: GetRandomBNBAddress(),
			ToAddress:   GetRandomBNBAddress(),
			Gas:         BNBGasFeeSingleton,
		},
		1024,
		common.EmptyPubKey, 1024,
	)

	msg, err := getMsgAddLiquidityFromMemo(w.ctx, lpMemo, txin, GetRandomBech32Addr(), 0)
	c.Assert(msg, NotNil)
	c.Assert(err, IsNil)

	// Asymentic liquidity provision should works fine, only RUNE
	txin.Tx.Coins = common.Coins{
		common.NewCoin(baseAsset,
			cosmos.NewUint(100*common.One)),
	}

	// provide only rune should be fine
	msg1, err1 := getMsgAddLiquidityFromMemo(w.ctx, lpMemo, txin, GetRandomBech32Addr(), 0)
	c.Assert(msg1, NotNil)
	c.Assert(err1, IsNil)

	bnbAsset, err := common.NewAsset("BNB.BNB")
	c.Assert(err, IsNil)
	txin.Tx.Coins = common.Coins{
		common.NewCoin(bnbAsset,
			cosmos.NewUint(100*common.One)),
	}

	// provide only token(BNB) should be fine
	msg2, err2 := getMsgAddLiquidityFromMemo(w.ctx, lpMemo, txin, GetRandomBech32Addr(), 0)
	c.Assert(msg2, NotNil)
	c.Assert(err2, IsNil)

	lokiAsset, _ := common.NewAsset("BNB.LOKI")
	// Make sure the RUNE Address and Asset Address set correctly
	txin.Tx.Coins = common.Coins{
		common.NewCoin(baseAsset,
			cosmos.NewUint(100*common.One)),
		common.NewCoin(lokiAsset,
			cosmos.NewUint(100*common.One)),
	}

	runeAddr := GetRandomBaseAddress()
	lokiAddLiquidityMemo, err := ParseMemo(GetCurrentVersion(), fmt.Sprintf("add:BNB.LOKI:%s", runeAddr))
	c.Assert(err, IsNil)
	msg4, err4 := getMsgAddLiquidityFromMemo(w.ctx, lokiAddLiquidityMemo.(AddLiquidityMemo), txin, GetRandomBech32Addr(), 0)
	c.Assert(err4, IsNil)
	c.Assert(msg4, NotNil)
	msgAddLiquidity, ok := msg4.(*MsgAddLiquidity)
	c.Assert(ok, Equals, true)
	c.Assert(msgAddLiquidity, NotNil)
	c.Assert(msgAddLiquidity.CacaoAddress, Equals, runeAddr)
	c.Assert(msgAddLiquidity.AssetAddress, Equals, txin.Tx.FromAddress)
}

func (HandlerSuite) TestMsgLeaveFromMemo(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	addr := types.GetRandomBech32Addr()
	txin := types.NewObservedTx(
		common.Tx{
			ID:          GetRandomTxHash(),
			Chain:       common.BNBChain,
			Coins:       common.Coins{common.NewCoin(common.BaseAsset(), cosmos.NewUint(1))},
			Memo:        fmt.Sprintf("LEAVE:%s", addr.String()),
			FromAddress: GetRandomBNBAddress(),
			ToAddress:   GetRandomBNBAddress(),
			Gas:         BNBGasFeeSingleton,
		},
		1024,
		common.EmptyPubKey, 1024,
	)

	msg, err := processOneTxIn(w.ctx, GetCurrentVersion(), w.keeper, txin, addr)
	c.Assert(err, IsNil)
	c.Check(msg.ValidateBasic(), IsNil)
}

func (HandlerSuite) TestYggdrasilMemo(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	addr := types.GetRandomBech32Addr()
	txin := types.NewObservedTx(
		common.Tx{
			ID:          GetRandomTxHash(),
			Chain:       common.BNBChain,
			Coins:       common.Coins{common.NewCoin(common.BaseAsset(), cosmos.NewUint(1))},
			Memo:        "yggdrasil+:1024",
			FromAddress: GetRandomBNBAddress(),
			ToAddress:   GetRandomBNBAddress(),
			Gas:         BNBGasFeeSingleton,
		},
		1024,
		GetRandomPubKey(), 1024,
	)

	msg, err := processOneTxIn(w.ctx, GetCurrentVersion(), w.keeper, txin, addr)
	c.Assert(err, IsNil)
	c.Check(msg.ValidateBasic(), IsNil)

	txin.Tx.Memo = "yggdrasil-:1024"
	msg, err = processOneTxIn(w.ctx, GetCurrentVersion(), w.keeper, txin, addr)
	c.Assert(err, IsNil)
	c.Check(msg.ValidateBasic(), IsNil)
}

func (s *HandlerSuite) TestReserveContributor(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	addr := types.GetRandomBech32Addr()
	txin := types.NewObservedTx(
		common.Tx{
			ID:          GetRandomTxHash(),
			Chain:       common.BNBChain,
			Coins:       common.Coins{common.NewCoin(common.BaseAsset(), cosmos.NewUint(1))},
			Memo:        "reserve",
			FromAddress: GetRandomBNBAddress(),
			ToAddress:   GetRandomBNBAddress(),
			Gas:         BNBGasFeeSingleton,
		},
		1024,
		GetRandomPubKey(), 1024,
	)

	msg, err := processOneTxIn(w.ctx, GetCurrentVersion(), w.keeper, txin, addr)
	c.Assert(err, IsNil)
	c.Check(msg.ValidateBasic(), IsNil)
	_, isReserve := msg.(*MsgReserveContributor)
	c.Assert(isReserve, Equals, true)
}

func (s *HandlerSuite) TestExternalHandler(c *C) {
	ctx, mgr := setupManagerForTest(c)
	handler := NewExternalHandler(mgr)
	ctx = ctx.WithBlockHeight(1024)
	msg := NewMsgNetworkFee(1024, common.BNBChain, 1, bnbSingleTxFee.Uint64(), GetRandomBech32Addr())
	result, err := handler(ctx, msg)
	c.Check(err, NotNil)
	c.Check(errors.Is(err, se.ErrUnauthorized), Equals, true)
	c.Check(result, IsNil)
	na := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, na), IsNil)
	FundAccount(c, ctx, mgr.Keeper(), na.NodeAddress, 10*common.One)
	result, err = handler(ctx, NewMsgSetVersion("0.1.0", na.NodeAddress))
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
}

func (s *HandlerSuite) TestFuzzyMatching(c *C) {
	ctx, mgr := setupManagerForTest(c)
	k := mgr.Keeper()
	p1 := NewPool()
	p1.Asset = common.BNBAsset
	p1.BalanceCacao = cosmos.NewUint(10 * common.One)
	c.Assert(k.SetPool(ctx, p1), IsNil)

	// real USDT
	p2 := NewPool()
	p2.Asset, _ = common.NewAsset("ETH.USDT-0XDAC17F958D2EE523A2206206994597C13D831EC7")
	p2.BalanceCacao = cosmos.NewUint(80 * common.One)
	c.Assert(k.SetPool(ctx, p2), IsNil)

	// fake USDT, attempt to clone end of contract address
	p3 := NewPool()
	p3.Asset, _ = common.NewAsset("ETH.USDT-0XD084B83C305DAFD76AE3E1B4E1F1FE213D831EC7")
	p3.BalanceCacao = cosmos.NewUint(20 * common.One)
	c.Assert(k.SetPool(ctx, p3), IsNil)

	// fake USDT, bad contract address
	p4 := NewPool()
	p4.Asset, _ = common.NewAsset("ETH.USDT-0XD084B83C305DAFD76AE3E1B4E1F1FE2ECCCB3988")
	p4.BalanceCacao = cosmos.NewUint(20 * common.One)
	c.Assert(k.SetPool(ctx, p4), IsNil)

	// fake USDT, right contract address, wrong ticker
	p6 := NewPool()
	p6.Asset, _ = common.NewAsset("ETH.UST-0XDAC17F958D2EE523A2206206994597C13D831EC7")
	p6.BalanceCacao = cosmos.NewUint(90 * common.One)
	c.Assert(k.SetPool(ctx, p6), IsNil)

	result := fuzzyAssetMatch(ctx, k, p1.Asset)
	c.Check(result.Equals(p1.Asset), Equals, true)
	result = fuzzyAssetMatch(ctx, k, p6.Asset)
	c.Check(result.Equals(p6.Asset), Equals, true)

	check, _ := common.NewAsset("ETH.USDT")
	result = fuzzyAssetMatch(ctx, k, check)
	c.Check(result.Equals(p2.Asset), Equals, true)
	check, _ = common.NewAsset("ETH.USDT-")
	result = fuzzyAssetMatch(ctx, k, check)
	c.Check(result.Equals(p2.Asset), Equals, true)
	check, _ = common.NewAsset("ETH.USDT-1EC7")
	result = fuzzyAssetMatch(ctx, k, check)
	c.Check(result.Equals(p2.Asset), Equals, true)

	check, _ = common.NewAsset("ETH/USDT-1EC7")
	result = fuzzyAssetMatch(ctx, k, check)
	c.Check(result.Synth, Equals, true)
	c.Check(result.Equals(p2.Asset.GetSyntheticAsset()), Equals, true)

	check, _ = common.NewAsset("ETH~USDT")
	result = fuzzyAssetMatch(ctx, k, check)
	c.Check(result.Synth, Equals, false)
	c.Check(result.Trade, Equals, true)
	c.Check(result.Equals(p2.Asset.GetTradeAsset()), Equals, true)
}

func (s *HandlerSuite) TestMemoFetchAddress(c *C) {
	ctx, k := setupKeeperForTest(c)

	baseAddr := GetRandomBaseAddress()
	name := NewMAYAName("hello", 50, []MAYANameAlias{{Chain: common.BASEChain, Address: baseAddr}}, common.EmptyAsset, nil, cosmos.ZeroUint(), nil)
	k.SetMAYAName(ctx, name)

	bnbAddr := GetRandomBNBAddress()
	addr, err := FetchAddress(ctx, k, bnbAddr.String(), common.BNBChain)
	c.Assert(err, IsNil)
	c.Check(addr.Equals(bnbAddr), Equals, true)

	addr, err = FetchAddress(ctx, k, "hello", common.BASEChain)
	c.Assert(err, IsNil)
	c.Check(addr.Equals(baseAddr), Equals, true)

	addr, err = FetchAddress(ctx, k, "hello.maya", common.BASEChain)
	c.Assert(err, IsNil)
	c.Check(addr.Equals(baseAddr), Equals, true)
}

func (s *HandlerSuite) TestExternalAssetMatch(c *C) {
	v := GetCurrentVersion()

	c.Check(externalAssetMatch(v, common.ETHChain, "7a0"), Equals, "0xd601c6A3a36721320573885A8d8420746dA3d7A0")
	c.Check(externalAssetMatch(v, common.ETHChain, "foobar"), Equals, "foobar")
	c.Check(externalAssetMatch(v, common.ETHChain, "3"), Equals, "3")
	c.Check(externalAssetMatch(v, common.ETHChain, ""), Equals, "")
	c.Check(externalAssetMatch(v, common.BTCChain, "foo"), Equals, "foo")
}

func makeFakeSignature(c *C) []byte {
	signature := make([]byte, 1024)
	_, err := rand.Read(signature)
	c.Assert(err, IsNil)
	return signature
}
