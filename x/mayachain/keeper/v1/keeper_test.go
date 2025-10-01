package keeperv1

import (
	"testing"

	. "gopkg.in/check.v1"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/simapp"
	"github.com/cosmos/cosmos-sdk/store"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	ibctransferkeeper "github.com/cosmos/ibc-go/v2/modules/apps/transfer/keeper"
	ibctransfertypes "github.com/cosmos/ibc-go/v2/modules/apps/transfer/types"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	dbm "github.com/tendermint/tm-db"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

func TestPackage(t *testing.T) { TestingT(t) }

func FundModule(c *C, ctx cosmos.Context, k KVStore, name string, amt uint64) {
	coin := common.NewCoin(common.BaseNative, cosmos.NewUint(amt*common.One))
	err := k.MintToModule(ctx, ModuleName, coin)
	c.Assert(err, IsNil)
	err = k.SendFromModuleToModule(ctx, ModuleName, name, common.NewCoins(coin))
	c.Assert(err, IsNil)
}

func FundModuleMayaToken(c *C, ctx cosmos.Context, k KVStore, name string, amt uint64) {
	coin := common.NewCoin(common.MayaNative, cosmos.NewUint(amt))
	err := k.MintToModule(ctx, ModuleName, coin)
	c.Assert(err, IsNil)
	err = k.SendFromModuleToModule(ctx, ModuleName, name, common.NewCoins(coin))
	c.Assert(err, IsNil)
}

func FundAccount(c *C, ctx cosmos.Context, k KVStore, addr cosmos.AccAddress, amt uint64) {
	coin := common.NewCoin(common.BaseNative, cosmos.NewUint(amt*common.One))
	c.Assert(k.MintAndSendToAccount(ctx, addr, coin), IsNil)
}

func FundAccountMayaToken(c *C, ctx cosmos.Context, k KVStore, addr cosmos.AccAddress, amt uint64) {
	coin := common.NewCoin(common.MayaNative, cosmos.NewUint(amt))
	c.Assert(k.MintAndSendToAccount(ctx, addr, coin), IsNil)
}

func SetupLiquidityBondForTest(c *C, ctx cosmos.Context, k KVStore, asset common.Asset, addr common.Address, bond cosmos.Uint) (LiquidityProvider, cosmos.Uint) {
	// Coins sent to BondName are sent to NodeAccount
	FundModule(c, ctx, k, BondName, 100*common.One)
	pk := GetRandomPubKey()
	assetAddr, err := pk.GetAddress(asset.GetChain())
	c.Assert(err, IsNil)
	lp := LiquidityProvider{
		Asset:        asset,
		CacaoAddress: addr,
		AssetAddress: assetAddr,
		Units:        bond,
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
	}
	c.Assert(k.SetPool(ctx, pool), IsNil)
	k.SetLiquidityProvider(ctx, lp)

	calcBond := common.GetSafeShare(lp.Units, pool.LPUnits, pool.BalanceCacao)
	return lp, calcBond
}

func SetupForDynamicInflationTest(c *C, poolBalance uint64) (cosmos.Context, KVStore) {
	ctx, k := setupKeeperForTest(c)

	// Set pool
	p, err := k.GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	p.Asset = common.BNBAsset
	p.BalanceCacao = cosmos.NewUint(common.One * poolBalance)
	p.BalanceAsset = cosmos.NewUint(common.One * common.One)
	p.LPUnits = cosmos.NewUint(common.One * common.One)
	p.Status = PoolAvailable
	c.Assert(k.SetPool(ctx, p), IsNil)

	return ctx, k
}

// nolint: deadcode unused
// create a codec used only for testing
func makeTestCodec() *codec.LegacyAmino {
	cdc := codec.NewLegacyAmino()
	banktypes.RegisterLegacyAminoCodec(cdc)
	authtypes.RegisterLegacyAminoCodec(cdc)
	RegisterCodec(cdc)
	cosmos.RegisterCodec(cdc)
	// codec.RegisterLegacyAminoCodec(cdc)
	return cdc
}

var keyThorchain = cosmos.NewKVStoreKey(StoreKey)

func setupKeeperForTest(c *C) (cosmos.Context, KVStore) {
	SetupConfigForTest()
	keys := cosmos.NewKVStoreKeys(
		authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey, paramstypes.StoreKey,
	)
	tkeyParams := cosmos.NewTransientStoreKey(paramstypes.TStoreKey)

	db := dbm.NewMemDB()
	ms := store.NewCommitMultiStore(db)
	ms.MountStoreWithDB(keys[authtypes.StoreKey], cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keys[paramstypes.StoreKey], cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keys[banktypes.StoreKey], cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyThorchain, cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(tkeyParams, cosmos.StoreTypeTransient, db)
	err := ms.LoadLatestVersion()
	c.Assert(err, IsNil)

	ctx := cosmos.NewContext(ms, tmproto.Header{ChainID: "thorchain"}, false, log.NewNopLogger())
	ctx = ctx.WithBlockHeight(18)
	legacyCodec := makeTestCodec()
	marshaler := simapp.MakeTestEncodingConfig().Marshaler

	maccPerms := map[string][]string{
		authtypes.FeeCollectorName:     nil,
		distrtypes.ModuleName:          nil,
		minttypes.ModuleName:           {authtypes.Minter},
		stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
		govtypes.ModuleName:            {authtypes.Burner},
		ibctransfertypes.ModuleName:    {authtypes.Minter, authtypes.Burner},
		ModuleName:                     {authtypes.Minter},
		ReserveName:                    {},
		AsgardName:                     {},
		BondName:                       {authtypes.Staking},
		MayaFund:                       {},
		AffiliateCollectorName:         {},
		CACAOPoolName:                  {},
	}

	pk := paramskeeper.NewKeeper(marshaler, legacyCodec, keys[paramstypes.StoreKey], tkeyParams)
	pk.Subspace(ibctransfertypes.ModuleName)
	ak := authkeeper.NewAccountKeeper(marshaler, keys[authtypes.StoreKey], pk.Subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, maccPerms)
	bk := bankkeeper.NewBaseKeeper(marshaler, keys[banktypes.StoreKey], ak, pk.Subspace(banktypes.ModuleName), nil)

	k := NewKVStore(marshaler, bk, ak, ibctransferkeeper.Keeper{}, keyThorchain, GetCurrentVersion())

	FundModule(c, ctx, k, AsgardName, common.One)

	return ctx, k
}

type KeeperTestSuite struct{}

var _ = Suite(&KeeperTestSuite{})

func (KeeperTestSuite) TestKeeperVersion(c *C) {
	ctx, k := setupKeeperForTest(c)
	c.Check(k.GetStoreVersion(ctx), Equals, int64(38))

	k.SetStoreVersion(ctx, 2)
	c.Check(k.GetStoreVersion(ctx), Equals, int64(2))

	c.Check(k.GetRuneBalanceOfModule(ctx, AsgardName).Equal(cosmos.NewUint(100000000*common.One)), Equals, true)
	coinsToSend := common.NewCoins(common.NewCoin(common.BaseNative, cosmos.NewUint(1*common.One)))
	c.Check(k.SendFromModuleToModule(ctx, AsgardName, BondName, coinsToSend), IsNil)

	acct := GetRandomBech32Addr()
	c.Check(k.SendFromModuleToAccount(ctx, AsgardName, acct, coinsToSend), IsNil)

	// check get account balance
	coins := k.GetBalance(ctx, acct)
	c.Check(coins, HasLen, 1)

	c.Check(k.SendFromAccountToModule(ctx, acct, AsgardName, coinsToSend), IsNil)

	// check no account balance
	coins = k.GetBalance(ctx, GetRandomBech32Addr())
	c.Check(coins, HasLen, 0)
}

func (KeeperTestSuite) TestMaxMint(c *C) {
	ctx, k := setupKeeperForTest(c)

	max := int64(200000000_00000000)
	k.SetMimir(ctx, "MaxRuneSupply", max)

	// check value is zero first
	val, err := k.GetMimir(ctx, "HaltTrading")
	c.Assert(err, IsNil)
	c.Check(val, Equals, int64(-1))

	// max NOT hit
	err = k.MintToModule(ctx, ModuleName, common.NewCoin(common.BaseAsset(), cosmos.NewUint(5_00000000)))
	c.Assert(err, IsNil)
	val, err = k.GetMimir(ctx, "HaltTrading")
	c.Assert(err, IsNil)
	c.Check(val, Equals, int64(-1))

	// max hit
	err = k.MintToModule(ctx, ModuleName, common.NewCoin(common.BaseAsset(), cosmos.NewUint(uint64(max*2))))
	c.Assert(err, IsNil)
	val, err = k.GetMimir(ctx, "HaltTrading")
	c.Assert(err, IsNil)
	c.Check(val, Equals, int64(1))

	val, err = k.GetMimir(ctx, "HaltChainGlobal")
	c.Assert(err, IsNil)
	c.Check(val, Equals, int64(1))

	val, err = k.GetMimir(ctx, "PauseLP")
	c.Assert(err, IsNil)
	c.Check(val, Equals, int64(1))

	val, err = k.GetMimir(ctx, "HaltTHORChain")
	c.Assert(err, IsNil)
	c.Check(val, Equals, int64(1))
}
