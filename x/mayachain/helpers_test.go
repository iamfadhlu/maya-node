package mayachain

import (
	"fmt"
	"strings"

	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

type HelperSuite struct{}

var _ = Suite(&HelperSuite{})

type TestRefundBondKeeper struct {
	keeper.KVStoreDummy
	ygg      Vault
	pool     Pool
	failPool bool
	na       NodeAccount
	naBond   cosmos.Uint
	bp       BondProviders
	lp       LiquidityProvider
	polLP    LiquidityProvider
	vaults   Vaults
	modules  map[string]int64
	consts   constants.ConstantValues
}

type TestHelpersKeeper struct {
	keeper.KVStoreDummy
	nas types.NodeAccounts
}

func (k *TestRefundBondKeeper) GetConfigInt64(ctx cosmos.Context, key constants.ConstantName) int64 {
	return k.consts.GetInt64Value(key)
}

func (k *TestRefundBondKeeper) GetAsgardVaultsByStatus(_ cosmos.Context, _ VaultStatus) (Vaults, error) {
	return k.vaults, nil
}

func (k *TestRefundBondKeeper) VaultExists(_ cosmos.Context, pk common.PubKey) bool {
	return true
}

func (k *TestRefundBondKeeper) GetVault(_ cosmos.Context, pk common.PubKey) (Vault, error) {
	if k.ygg.PubKey.Equals(pk) {
		return k.ygg, nil
	}
	return Vault{}, errKaboom
}

func (k *TestRefundBondKeeper) GetLeastSecure(ctx cosmos.Context, vaults Vaults, signingTransPeriod int64) Vault {
	return vaults[0]
}

func (k *TestRefundBondKeeper) GetPool(_ cosmos.Context, asset common.Asset) (Pool, error) {
	if k.pool.Asset.Equals(asset) && !k.failPool {
		return k.pool, nil
	}
	return NewPool(), errKaboom
}

func (k *TestRefundBondKeeper) SetNodeAccount(_ cosmos.Context, na NodeAccount) error {
	k.na = na
	return nil
}

func (k *TestRefundBondKeeper) SetPool(_ cosmos.Context, p Pool) error {
	if k.pool.Asset.Equals(p.Asset) {
		k.pool = p
		return nil
	}
	return errKaboom
}

func (k *TestRefundBondKeeper) DeleteVault(_ cosmos.Context, key common.PubKey) error {
	if k.ygg.PubKey.Equals(key) {
		k.ygg = NewVault(1, InactiveVault, AsgardVault, GetRandomPubKey(), common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	}
	return nil
}

func (k *TestRefundBondKeeper) SetVault(ctx cosmos.Context, vault Vault) error {
	if k.ygg.PubKey.Equals(vault.PubKey) {
		k.ygg = vault
	}
	return nil
}

func (k *TestRefundBondKeeper) SetBondProviders(ctx cosmos.Context, bp BondProviders) error {
	k.bp = bp
	return nil
}

func (k *TestRefundBondKeeper) GetBondProviders(ctx cosmos.Context, add cosmos.AccAddress) (BondProviders, error) {
	return k.bp, nil
}

func (k *TestRefundBondKeeper) SendFromModuleToModule(_ cosmos.Context, from, to string, coins common.Coins) error {
	k.modules[from] -= int64(coins[0].Amount.Uint64())
	k.modules[to] += int64(coins[0].Amount.Uint64())
	return nil
}

func (k *TestRefundBondKeeper) CalcNodeLiquidityBond(_ cosmos.Context, na NodeAccount) (cosmos.Uint, error) {
	return k.naBond, nil
}

func (k *TestHelpersKeeper) CalcNodeLiquidityBond(_ cosmos.Context, na NodeAccount) (cosmos.Uint, error) {
	for _, nodeAccount := range k.nas {
		if nodeAccount.BondAddress.Equals(na.BondAddress) {
			return nodeAccount.Bond, nil
		}
	}
	return cosmos.ZeroUint(), nil
}

func (k *TestRefundBondKeeper) CalcLPLiquidityBond(_ cosmos.Context, bondAddr common.Address, nodeAddr cosmos.AccAddress) (cosmos.Uint, error) {
	var lp LiquidityProvider
	switch bondAddr.String() {
	case k.lp.CacaoAddress.String():
		lp = k.lp
	case bondAddr.String():
		lp = k.polLP
	default:
		lp = k.lp
	}
	baseValue := common.GetSafeShare(lp.Units, k.pool.LPUnits, k.pool.BalanceCacao)
	baseValue = baseValue.Add(k.pool.AssetValueInRune(common.GetSafeShare(lp.Units, k.pool.LPUnits, k.pool.BalanceAsset)))
	return baseValue, nil
}

func (k *TestRefundBondKeeper) SetLiquidityProvider(_ cosmos.Context, lp LiquidityProvider) {
	k.lp = lp
}

func (k *TestRefundBondKeeper) SetLiquidityProviders(_ cosmos.Context, lps LiquidityProviders) {
	for _, lp := range lps {
		switch lp.CacaoAddress.String() {
		case k.lp.CacaoAddress.String():
			k.lp = lp
		case k.polLP.CacaoAddress.String():
			k.polLP = lp
		default:
			k.lp = lp
		}
	}
}

func (k *TestRefundBondKeeper) GetLiquidityProvider(_ cosmos.Context, asset common.Asset, addr common.Address) (LiquidityProvider, error) {
	switch addr.String() {
	case k.lp.CacaoAddress.String():
		return k.lp, nil
	case k.polLP.CacaoAddress.String():
		return k.polLP, nil
	default:
		return k.lp, nil
	}
}

func (k *TestRefundBondKeeper) GetLiquidityProviderByAssets(_ cosmos.Context, assets common.Assets, addr common.Address) (LiquidityProviders, error) {
	return LiquidityProviders{k.lp}, nil
}

func (k *TestRefundBondKeeper) GetModuleAddress(module string) (common.Address, error) {
	return k.polLP.CacaoAddress, nil
}

func (s *HelperSuite) TestSubsidizePoolWithSlashBond(c *C) {
	ctx, mgr := setupManagerForTest(c)
	na := GetRandomValidatorNode(NodeActive)
	bp := NewBondProviders(na.NodeAddress)
	acc, err := na.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp.Providers = append(bp.Providers, NewBondProvider(acc))
	bp.Providers[0].Bonded = true
	c.Assert(mgr.Keeper().SetBondProviders(ctx, bp), IsNil)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, na), IsNil)

	ygg := GetRandomVault()
	c.Assert(subsidizePoolsWithSlashBond(ctx, ygg.Coins, ygg, cosmos.NewUint(100*common.One), cosmos.ZeroUint(), mgr), IsNil)
	poolBNB := NewPool()
	poolBNB.Asset = common.BNBAsset
	poolBNB.BalanceCacao = cosmos.NewUint(100 * common.One)
	poolBNB.BalanceAsset = cosmos.NewUint(100 * common.One)
	poolBNB.LPUnits = cosmos.NewUint(100 * common.One)
	poolBNB.Status = PoolAvailable
	c.Assert(mgr.Keeper().SetPool(ctx, poolBNB), IsNil)

	poolTCAN := NewPool()
	tCanAsset, err := common.NewAsset("BNB.TCAN-014")
	c.Assert(err, IsNil)
	poolTCAN.Asset = tCanAsset
	poolTCAN.BalanceCacao = cosmos.NewUint(200 * common.One)
	poolTCAN.BalanceAsset = cosmos.NewUint(200 * common.One)
	poolTCAN.Status = PoolAvailable
	c.Assert(mgr.Keeper().SetPool(ctx, poolTCAN), IsNil)

	poolBTC := NewPool()
	poolBTC.Asset = common.BTCAsset
	poolBTC.BalanceAsset = cosmos.NewUint(300 * common.One)
	poolBTC.BalanceCacao = cosmos.NewUint(300 * common.One)
	poolBTC.Status = PoolAvailable
	c.Assert(mgr.Keeper().SetPool(ctx, poolBTC), IsNil)
	ygg.Type = YggdrasilVault
	ygg.Coins = common.Coins{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(1*common.One)),            // 1
		common.NewCoin(tCanAsset, cosmos.NewUint(common.One).QuoUint64(2)),       // 0.5 TCAN
		common.NewCoin(common.BTCAsset, cosmos.NewUint(common.One).QuoUint64(4)), // 0.25 BTC
	}
	totalRuneStolen, err := getTotalYggValueInRune(ctx, mgr.Keeper(), ygg)
	c.Assert(err, IsNil)

	slashAmt := totalRuneStolen.MulUint64(3).QuoUint64(2)
	fmt.Println(slashAmt.Uint64())
	// add slash to pol
	polAddress, err := mgr.Keeper().GetModuleAddress(ReserveName)
	c.Assert(err, IsNil)
	polLP, err := mgr.Keeper().GetLiquidityProvider(ctx, common.BNBAsset, polAddress)
	c.Assert(err, IsNil)
	polLP.Units = polLP.Units.Add(slashAmt)
	mgr.Keeper().SetLiquidityProvider(ctx, polLP)
	mgr.Keeper().SetMimir(ctx, "POL-BNB-BNB", 1)

	asgard := GetRandomVault()
	asgard.Membership = append(asgard.Membership, na.PubKeySet.Secp256k1.String())
	c.Assert(mgr.Keeper().SetVault(ctx, asgard), IsNil)

	asgardBeforeSlash := mgr.Keeper().GetRuneBalanceOfModule(ctx, AsgardName)
	reserveBeforeSlash := mgr.Keeper().GetRuneBalanceOfModule(ctx, ReserveName)
	poolsBeforeSlash := poolBNB.BalanceCacao.Add(poolTCAN.BalanceCacao).Add(poolBTC.BalanceCacao)

	c.Assert(subsidizePoolsWithSlashBond(ctx, ygg.Coins, ygg, totalRuneStolen, slashAmt, mgr), IsNil)

	bnbStolen := ygg.GetCoin(common.BNBAsset).Amount
	// we're really only adding 1/2 of the stolen BNB to the BNB pool, the other half is withdrawn
	amountBNBForBNBPool := poolBNB.AssetValueInRune(bnbStolen).QuoUint64(2)
	runeBNB := poolBNB.BalanceCacao.Add(amountBNBForBNBPool).SubUint64(353808)
	bnbPoolAsset := poolBNB.BalanceAsset.Sub(cosmos.NewUint(common.One))
	poolBNB, err = mgr.Keeper().GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Assert(poolBNB.BalanceCacao.Equal(runeBNB), Equals, true, Commentf("expected %d, got %d", runeBNB.Uint64(), poolBNB.BalanceCacao.Uint64()))
	c.Assert(poolBNB.BalanceAsset.Equal(bnbPoolAsset), Equals, true)

	amountRuneForTCANPool := poolTCAN.AssetValueInRune(ygg.GetCoin(tCanAsset).Amount)
	runeTCAN := poolTCAN.BalanceCacao.Add(amountRuneForTCANPool)
	tcanPoolAsset := poolTCAN.BalanceAsset.Sub(cosmos.NewUint(common.One).QuoUint64(2))
	poolTCAN, err = mgr.Keeper().GetPool(ctx, tCanAsset)
	c.Assert(err, IsNil)
	c.Assert(poolTCAN.BalanceCacao.Equal(runeTCAN), Equals, true)
	c.Assert(poolTCAN.BalanceAsset.Equal(tcanPoolAsset), Equals, true)

	amountRuneForBTCPool := poolBTC.AssetValueInRune(ygg.GetCoin(common.BTCAsset).Amount)
	runeBTC := poolBTC.BalanceCacao.Add(amountRuneForBTCPool)
	btcPoolAsset := poolBTC.BalanceAsset.Sub(ygg.GetCoin(common.BTCAsset).Amount)
	poolBTC, err = mgr.Keeper().GetPool(ctx, common.BTCAsset)
	c.Assert(err, IsNil)
	c.Assert(poolBTC.BalanceCacao.Equal(runeBTC), Equals, true, Commentf("expected %d, got %d", runeBTC.Uint64(), poolBTC.BalanceCacao.Uint64()))
	c.Assert(poolBTC.BalanceAsset.Equal(btcPoolAsset), Equals, true, Commentf("expected %d, got %d", btcPoolAsset.Uint64(), poolBTC.BalanceAsset.Uint64()))

	asgardAfterSlash := mgr.Keeper().GetRuneBalanceOfModule(ctx, AsgardName)
	reserveAfterSlash := mgr.Keeper().GetRuneBalanceOfModule(ctx, ReserveName)
	poolsAfterSlash := runeBNB.Add(runeTCAN).Add(runeBTC)

	// subsidized BASE should move from reserve to asgard
	// FIXME
	c.Assert(poolsAfterSlash.Sub(poolsBeforeSlash).SubUint64(1).Uint64(), Equals, asgardAfterSlash.Sub(asgardBeforeSlash).Uint64(), Commentf("expected %d, got %d", poolsAfterSlash.Sub(poolsBeforeSlash).Uint64(), asgardAfterSlash.Sub(asgardBeforeSlash).Uint64()))
	c.Assert(asgardAfterSlash.Sub(asgardBeforeSlash).Uint64(), Equals, reserveBeforeSlash.Sub(reserveAfterSlash).Uint64(), Commentf("expected %d, got %d", asgardAfterSlash.Sub(asgardBeforeSlash).Uint64(), reserveBeforeSlash.Sub(reserveAfterSlash).Uint64()))

	ygg1 := GetRandomVault()
	ygg1.Type = YggdrasilVault
	ygg1.Coins = common.Coins{
		common.NewCoin(tCanAsset, cosmos.NewUint(common.One*2)),       // 2 TCAN
		common.NewCoin(common.BTCAsset, cosmos.NewUint(common.One*4)), // 4 BTC
	}
	totalRuneStolen, err = getTotalYggValueInRune(ctx, mgr.Keeper(), ygg1)
	c.Assert(err, IsNil)
	slashAmt = cosmos.NewUint(100 * common.One)
	c.Assert(subsidizePoolsWithSlashBond(ctx, ygg1.Coins, ygg1, totalRuneStolen, slashAmt, mgr), IsNil)

	amountRuneForTCANPool = poolTCAN.AssetValueInRune(ygg1.GetCoin(tCanAsset).Amount)
	runeTCAN = poolTCAN.BalanceCacao.Add(amountRuneForTCANPool)
	poolTCAN, err = mgr.Keeper().GetPool(ctx, tCanAsset)
	c.Assert(err, IsNil)
	c.Assert(poolTCAN.BalanceCacao.Equal(runeTCAN), Equals, true)

	amountRuneForBTCPool = poolBTC.AssetValueInRune(ygg1.GetCoin(common.BTCAsset).Amount)
	runeBTC = poolBTC.BalanceCacao.Add(amountRuneForBTCPool)
	poolBTC, err = mgr.Keeper().GetPool(ctx, common.BTCAsset)
	c.Assert(err, IsNil)
	c.Assert(poolBTC.BalanceCacao.Equal(runeBTC), Equals, true)

	ygg2 := GetRandomVault()
	ygg2.Type = YggdrasilVault
	ygg2.Coins = common.Coins{
		common.NewCoin(tCanAsset, cosmos.NewUint(0)),
	}
	totalRuneStolen, err = getTotalYggValueInRune(ctx, mgr.Keeper(), ygg2)
	c.Assert(err, IsNil)
	c.Assert(subsidizePoolsWithSlashBond(ctx, ygg2.Coins, ygg2, totalRuneStolen, slashAmt, mgr), IsNil)

	// Skip subsidy if rune value of coin is 0 - can happen with an old, empty pool
	poolETH := NewPool()
	poolETH.Asset = common.ETHAsset
	poolETH.BalanceAsset = cosmos.ZeroUint()
	poolETH.BalanceCacao = cosmos.NewUint(300 * common.One)
	poolETH.Status = PoolAvailable
	c.Assert(mgr.Keeper().SetPool(ctx, poolETH), IsNil)

	ygg3 := GetRandomVault()
	ygg3.Type = YggdrasilVault
	ygg3.Coins = common.Coins{
		common.NewCoin(common.ETHAsset, cosmos.NewUint(100)),
	}

	c.Assert(subsidizePoolsWithSlashBond(ctx, ygg3.Coins, ygg3, totalRuneStolen, slashAmt, mgr), IsNil)
	poolETH, err = mgr.Keeper().GetPool(ctx, common.ETHAsset)
	c.Assert(err, IsNil)
	c.Assert(poolETH.BalanceCacao.Equal(cosmos.NewUint(300*common.One)), Equals, true)
}

func (s *HelperSuite) TestPausedLP(c *C) {
	ctx, mgr := setupManagerForTest(c)

	c.Check(isLPPaused(ctx, common.BNBChain, mgr), Equals, false)
	c.Check(isLPPaused(ctx, common.BTCChain, mgr), Equals, false)

	mgr.Keeper().SetMimir(ctx, "PauseLPBTC", 1)
	c.Check(isLPPaused(ctx, common.BTCChain, mgr), Equals, true)

	mgr.Keeper().SetMimir(ctx, "PauseLP", 1)
	c.Check(isLPPaused(ctx, common.BNBChain, mgr), Equals, true)
}

func (s *HelperSuite) TestRefundBondError(c *C) {
	ctx, _ := setupKeeperForTest(c)
	// active node should not refund bond
	pk := GetRandomPubKey()
	na := GetRandomValidatorNode(NodeActive)
	na.PubKeySet.Secp256k1 = pk
	na.Reward = cosmos.NewUint(100 * common.One)
	acc, err := na.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp := NewBondProviders(na.NodeAddress)
	bp.Providers = append(bp.Providers, NewBondProvider(acc))
	bp.Providers[0].Bonded = true
	tx := GetRandomTx()
	tx.FromAddress = GetRandomBaseAddress()
	keeper1 := &TestRefundBondKeeper{
		modules: make(map[string]int64),
		consts:  constants.GetConstantValues(GetCurrentVersion()),
		naBond:  cosmos.NewUint(100 * common.One),
		na:      na,
		bp:      bp,
		lp: LiquidityProvider{
			Asset:        common.BNBAsset,
			CacaoAddress: na.BondAddress,
			AssetAddress: GetRandomBNBAddress(),
			Units:        cosmos.NewUint(100 * common.One),
		},
		pool: types.Pool{
			Asset:        common.BNBAsset,
			BalanceAsset: cosmos.NewUint(100 * common.One),
			BalanceCacao: cosmos.NewUint(100 * common.One),
			LPUnits:      cosmos.NewUint(100 * common.One),
			Status:       PoolAvailable,
		},
	}
	mgr := NewDummyMgrWithKeeper(keeper1)
	c.Assert(refundBond(ctx, tx, na.NodeAddress, common.BNBAsset, cosmos.NewUint(100*common.One), &na, mgr), IsNil)
	c.Assert(refundBond(ctx, tx, na.NodeAddress, common.BNBAsset, cosmos.ZeroUint(), &na, mgr), IsNil)
	c.Assert(refundBond(ctx, tx, na.NodeAddress, common.EmptyAsset, cosmos.ZeroUint(), &na, mgr), IsNil)

	// fail to get vault should return an error
	na.UpdateStatus(NodeStandby, ctx.BlockHeight())
	keeper1.na = na
	mgr.K = keeper1
	c.Assert(refundBond(ctx, tx, na.NodeAddress, common.EmptyAsset, cosmos.ZeroUint(), &na, mgr), NotNil)

	// if the vault is not a yggdrasil pool , it should return an error
	ygg := NewVault(ctx.BlockHeight(), ActiveVault, AsgardVault, pk, common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	ygg.Coins = common.Coins{}
	keeper1.ygg = ygg
	mgr.K = keeper1
	c.Assert(refundBond(ctx, tx, na.NodeAddress, common.EmptyAsset, cosmos.ZeroUint(), &na, mgr), NotNil)

	// fail to get pool should fail
	ygg = NewVault(ctx.BlockHeight(), ActiveVault, YggdrasilVault, pk, common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	ygg.Coins = common.Coins{
		common.NewCoin(common.BaseAsset(), cosmos.NewUint(27*common.One)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(27*common.One)),
	}
	keeper1.ygg = ygg
	mgr.K = keeper1
	mgr.slasher = BlankSlasherManager{}
	keeper1.failPool = true
	c.Assert(refundBond(ctx, tx, na.NodeAddress, common.BNBAsset, cosmos.NewUint(100*common.One), &na, mgr), NotNil)

	// when ygg asset in RUNE is more then bond , thorchain should slash the node account with all their bond
	keeper1.failPool = false
	keeper1.pool.BalanceCacao = cosmos.NewUint(10 * common.One)
	keeper1.pool.BalanceAsset = cosmos.NewUint(10 * common.One)
	mgr.K = keeper1
	c.Assert(refundBond(ctx, tx, na.NodeAddress, common.BNBAsset, cosmos.NewUint(100*common.One), &na, mgr), IsNil)
	// make sure no tx has been generated for refund
	items, err := mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Check(items, HasLen, 0)

	// return error when asset is empty but units is not zero
	c.Assert(refundBond(ctx, tx, na.NodeAddress, common.EmptyAsset, cosmos.NewUint(1), &na, mgr), NotNil)
	// make sure no tx has been generated for refund
	items, err = mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Check(items, HasLen, 0)
}

func (s *HelperSuite) TestRefundBondHappyPath(c *C) {
	ctx, mgr := setupManagerForTest(c)
	na := GetRandomValidatorNode(NodeActive)
	na.Reward = cosmos.NewUint(12098 * common.One)
	naBond := cosmos.NewUint(23376 * common.One)
	// setup active node
	na = GetRandomValidatorNode(NodeActive)
	acc, err := na.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp := NewBondProviders(na.NodeAddress)
	bp.Providers = append(bp.Providers, NewBondProvider(acc))
	bp.Providers[0].Bonded = true
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, na), IsNil)
	c.Assert(mgr.Keeper().SetBondProviders(ctx, bp), IsNil)
	SetupLiquidityBondForTest(c, ctx, mgr.Keeper(), common.ETHAsset, na.BondAddress, na, naBond)

	// setup asgard vault needed for nativeTxOut
	asgard := NewVault(ctx.BlockHeight(), ActiveVault, AsgardVault, na.PubKeySet.Secp256k1, common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	c.Assert(mgr.Keeper().SetVault(ctx, asgard), IsNil)

	// setup to slash standby node
	reward := cosmos.NewUint(12098 * common.One)
	standByNode := GetRandomValidatorNode(NodeStandby)
	standByNode.Reward = reward
	FundModule(c, ctx, mgr.Keeper(), BondName, reward.Uint64())
	acc, err = standByNode.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp = NewBondProviders(standByNode.NodeAddress)
	bp.Providers = append(bp.Providers, NewBondProvider(acc))
	bp.Providers[0].Bonded = true
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, standByNode), IsNil)
	c.Assert(mgr.Keeper().SetBondProviders(ctx, bp), IsNil)
	SetupLiquidityBondForTest(c, ctx, mgr.Keeper(), common.BTCAsset, standByNode.BondAddress, standByNode, naBond)

	ygg := NewVault(ctx.BlockHeight(), ActiveVault, YggdrasilVault, standByNode.PubKeySet.Secp256k1, common.Chains{common.BNBChain}.Strings(), []ChainContract{})

	ygg.Coins = common.Coins{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(55*common.One)),
	}
	c.Assert(mgr.Keeper().SetVault(ctx, ygg), IsNil)

	// setup bnb pool
	poolUnits := cosmos.NewUint(23789 * common.One)
	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceCacao = poolUnits
	pool.BalanceAsset = cosmos.NewUint(167 * common.One)
	pool.LPUnits = poolUnits
	pool.Status = PoolAvailable
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)

	mgr.slasher, err = GetSlasher(mgr.currentVersion, mgr.Keeper(), mgr.eventMgr)
	c.Assert(err, IsNil)
	tx := GetRandomTx()
	tx.FromAddress, _ = common.NewAddress(standByNode.BondAddress.String(), GetCurrentVersion())
	c.Assert(err, IsNil)
	err = refundBond(ctx, tx, standByNode.NodeAddress, common.BNBAsset, poolUnits, &standByNode, mgr)
	c.Assert(err, IsNil)
	items, err := mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 0)

	slashedLPUnits := cosmos.NewUint(587602544910)
	polAddress, err := mgr.Keeper().GetModuleAddress(ReserveName)
	c.Assert(err, IsNil)
	polLP, err := mgr.Keeper().GetLiquidityProvider(ctx, common.BTCAsset, polAddress)
	c.Assert(err, IsNil)
	c.Assert(slashedLPUnits.Uint64(), Equals, polLP.Units.Uint64(), Commentf("expected %d, got %d", slashedLPUnits.Uint64(), polLP.Units.Uint64()))
	// we have pool units equal rune balance we can avoid calculating slash to units
	lp, err := mgr.Keeper().GetLiquidityProvider(ctx, common.BTCAsset, standByNode.BondAddress)
	c.Assert(err, IsNil)
	expectedUnits := naBond.Sub(slashedLPUnits)
	c.Assert(lp.Units.Uint64(), Equals, expectedUnits.Uint64(), Commentf("expected %d, got %d", expectedUnits, lp.Units.Uint64()))
}

func (s *HelperSuite) TestRefundBondDisableRequestToLeaveNode(c *C) {
	ctx, mgr := setupManagerForTest(c)
	naBond := cosmos.NewUint(23376 * common.One)
	// setup active node
	na := GetRandomValidatorNode(NodeActive)
	acc, err := na.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp := NewBondProviders(na.NodeAddress)
	bp.Providers = append(bp.Providers, NewBondProvider(acc))
	bp.Providers[0].Bonded = true
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, na), IsNil)
	c.Assert(mgr.Keeper().SetBondProviders(ctx, bp), IsNil)
	SetupLiquidityBondForTest(c, ctx, mgr.Keeper(), common.ETHAsset, na.BondAddress, na, naBond)

	// setup asgard vault needed for nativeTxOut
	asgard := NewVault(ctx.BlockHeight(), ActiveVault, AsgardVault, na.PubKeySet.Secp256k1, common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	c.Assert(mgr.Keeper().SetVault(ctx, asgard), IsNil)

	// setup to slash standby node
	reward := cosmos.NewUint(12098 * common.One)
	standByNode := GetRandomValidatorNode(NodeStandby)
	standByNode.Reward = reward
	standByNode.RequestedToLeave = true
	FundModule(c, ctx, mgr.Keeper(), BondName, reward.Uint64())
	acc, err = standByNode.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp = NewBondProviders(standByNode.NodeAddress)
	bp.Providers = append(bp.Providers, NewBondProvider(acc))
	bp.Providers[0].Bonded = true
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, standByNode), IsNil)
	c.Assert(mgr.Keeper().SetBondProviders(ctx, bp), IsNil)
	SetupLiquidityBondForTest(c, ctx, mgr.Keeper(), common.BTCAsset, standByNode.BondAddress, standByNode, naBond)

	ygg := NewVault(ctx.BlockHeight(), ActiveVault, YggdrasilVault, standByNode.PubKeySet.Secp256k1, common.Chains{common.BNBChain}.Strings(), []ChainContract{})

	ygg.Coins = common.Coins{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(55*common.One)),
	}
	c.Assert(mgr.Keeper().SetVault(ctx, ygg), IsNil)

	// setup bnb pool
	poolUnits := cosmos.NewUint(23789 * common.One)
	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceCacao = poolUnits
	pool.BalanceAsset = cosmos.NewUint(167 * common.One)
	pool.LPUnits = poolUnits
	pool.Status = PoolAvailable
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)

	mgr.slasher, err = GetSlasher(mgr.currentVersion, mgr.Keeper(), mgr.eventMgr)
	c.Assert(err, IsNil)
	tx := GetRandomTx()
	tx.FromAddress, _ = common.NewAddress(standByNode.BondAddress.String(), GetCurrentVersion())
	c.Assert(err, IsNil)
	err = refundBond(ctx, tx, standByNode.NodeAddress, common.BTCAsset, naBond, &standByNode, mgr)
	c.Assert(err, IsNil)
	items, err := mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 0)

	slashedLPUnits := cosmos.NewUint(587602544910)
	polAddress, err := mgr.Keeper().GetModuleAddress(ReserveName)
	c.Assert(err, IsNil)
	polLP, err := mgr.Keeper().GetLiquidityProvider(ctx, common.BTCAsset, polAddress)
	c.Assert(err, IsNil)
	c.Assert(polLP.Units.Uint64(), Equals, slashedLPUnits.Uint64(), Commentf("expected %d, got %d", slashedLPUnits.Uint64(), polLP.Units.Uint64()))
	// we have pool units equal rune balance we can avoid calculating slash to units
	lp, err := mgr.Keeper().GetLiquidityProvider(ctx, common.BTCAsset, standByNode.BondAddress)
	c.Assert(err, IsNil)
	expectedUnits := naBond.Sub(slashedLPUnits)
	c.Assert(lp.Units.Uint64(), Equals, expectedUnits.Uint64(), Commentf("expected %d, got %d", expectedUnits, lp.Units.Uint64()))

	// check node account is disabled
	standByNode, err = mgr.Keeper().GetNodeAccount(ctx, standByNode.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(standByNode.Status, Equals, NodeDisabled)
}

func (s *HelperSuite) TestDollarInRune(c *C) {
	ctx, k := setupKeeperForTest(c)
	mgr := NewDummyMgrWithKeeper(k)
	busd, err := common.NewAsset("BNB.BUSD-BD1")
	c.Assert(err, IsNil)
	pool := NewPool()
	pool.Asset = busd
	pool.Status = PoolAvailable
	pool.BalanceCacao = cosmos.NewUint(85515078103667)
	pool.BalanceAsset = cosmos.NewUint(709802235538353)
	c.Assert(k.SetPool(ctx, pool), IsNil)

	runeUSDPrice := telem(DollarInRune(ctx, mgr))
	c.Assert(runeUSDPrice, Equals, float32(0.12047733))
}

func (s *HelperSuite) TestTelem(c *C) {
	value := cosmos.NewUint(12047733)
	c.Assert(value.Uint64(), Equals, uint64(12047733))
	c.Assert(telem(value), Equals, float32(0.12047733))
}

type addGasFeesKeeperHelper struct {
	keeper.Keeper
	errGetNetwork bool
	errSetNetwork bool
	errGetPool    bool
	errSetPool    bool
}

func newAddGasFeesKeeperHelper(keeper keeper.Keeper) *addGasFeesKeeperHelper {
	return &addGasFeesKeeperHelper{
		Keeper: keeper,
	}
}

func (h *addGasFeesKeeperHelper) GetNetwork(ctx cosmos.Context) (Network, error) {
	if h.errGetNetwork {
		return Network{}, errKaboom
	}
	return h.Keeper.GetNetwork(ctx)
}

func (h *addGasFeesKeeperHelper) SetNetwork(ctx cosmos.Context, data Network) error {
	if h.errSetNetwork {
		return errKaboom
	}
	return h.Keeper.SetNetwork(ctx, data)
}

func (h *addGasFeesKeeperHelper) SetPool(ctx cosmos.Context, pool Pool) error {
	if h.errSetPool {
		return errKaboom
	}
	return h.Keeper.SetPool(ctx, pool)
}

func (h *addGasFeesKeeperHelper) GetPool(ctx cosmos.Context, asset common.Asset) (Pool, error) {
	if h.errGetPool {
		return Pool{}, errKaboom
	}
	return h.Keeper.GetPool(ctx, asset)
}

type addGasFeeTestHelper struct {
	ctx cosmos.Context
	na  NodeAccount
	mgr Manager
}

func newAddGasFeeTestHelper(c *C) addGasFeeTestHelper {
	ctx, mgr := setupManagerForTest(c)
	keeper := newAddGasFeesKeeperHelper(mgr.Keeper())
	mgr.K = keeper
	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.BalanceCacao = cosmos.NewUint(100 * common.One)
	pool.Status = PoolAvailable
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)

	poolBTC := NewPool()
	poolBTC.Asset = common.BTCAsset
	poolBTC.BalanceAsset = cosmos.NewUint(100 * common.One)
	poolBTC.BalanceCacao = cosmos.NewUint(100 * common.One)
	poolBTC.Status = PoolAvailable
	c.Assert(mgr.Keeper().SetPool(ctx, poolBTC), IsNil)

	na := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, na), IsNil)
	yggVault := NewVault(ctx.BlockHeight(), ActiveVault, YggdrasilVault, na.PubKeySet.Secp256k1, common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	c.Assert(mgr.Keeper().SetVault(ctx, yggVault), IsNil)
	version := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(version)
	mgr.gasMgr = newGasMgrVCUR(constAccessor, keeper)
	return addGasFeeTestHelper{
		ctx: ctx,
		mgr: mgr,
		na:  na,
	}
}

func (s *HelperSuite) TestAddGasFees(c *C) {
	testCases := []struct {
		name        string
		txCreator   func(helper addGasFeeTestHelper) ObservedTx
		runner      func(helper addGasFeeTestHelper, tx ObservedTx) error
		expectError bool
		validator   func(helper addGasFeeTestHelper, c *C)
	}{
		{
			name: "empty Gas should just return nil",
			txCreator: func(helper addGasFeeTestHelper) ObservedTx {
				return GetRandomObservedTx()
			},

			expectError: false,
		},
		{
			name: "normal BNB gas",
			txCreator: func(helper addGasFeeTestHelper) ObservedTx {
				tx := ObservedTx{
					Tx: common.Tx{
						ID:          GetRandomTxHash(),
						Chain:       common.BNBChain,
						FromAddress: GetRandomBNBAddress(),
						ToAddress:   GetRandomBNBAddress(),
						Coins: common.Coins{
							common.NewCoin(common.BNBAsset, cosmos.NewUint(5*common.One)),
							common.NewCoin(common.BaseAsset(), cosmos.NewUint(8*common.One)),
						},
						Gas: common.Gas{
							common.NewCoin(common.BNBAsset, BNBGasFeeSingleton[0].Amount),
						},
						Memo: "",
					},
					Status:         types.Status_done,
					OutHashes:      nil,
					BlockHeight:    helper.ctx.BlockHeight(),
					Signers:        []string{helper.na.NodeAddress.String()},
					ObservedPubKey: helper.na.PubKeySet.Secp256k1,
				}
				return tx
			},
			runner: func(helper addGasFeeTestHelper, tx ObservedTx) error {
				return addGasFees(helper.ctx, helper.mgr, tx)
			},
			expectError: false,
			validator: func(helper addGasFeeTestHelper, c *C) {
				expected := common.NewCoin(common.BNBAsset, BNBGasFeeSingleton[0].Amount)
				c.Assert(helper.mgr.GasMgr().GetGas(), HasLen, 1)
				c.Assert(helper.mgr.GasMgr().GetGas()[0].Equals(expected), Equals, true)
			},
		},
		{
			name: "normal BTC gas",
			txCreator: func(helper addGasFeeTestHelper) ObservedTx {
				tx := ObservedTx{
					Tx: common.Tx{
						ID:          GetRandomTxHash(),
						Chain:       common.BTCChain,
						FromAddress: GetRandomBTCAddress(),
						ToAddress:   GetRandomBTCAddress(),
						Coins: common.Coins{
							common.NewCoin(common.BTCAsset, cosmos.NewUint(5*common.One)),
						},
						Gas: common.Gas{
							common.NewCoin(common.BTCAsset, cosmos.NewUint(2000)),
						},
						Memo: "",
					},
					Status:         types.Status_done,
					OutHashes:      nil,
					BlockHeight:    helper.ctx.BlockHeight(),
					Signers:        []string{helper.na.NodeAddress.String()},
					ObservedPubKey: helper.na.PubKeySet.Secp256k1,
				}
				return tx
			},
			runner: func(helper addGasFeeTestHelper, tx ObservedTx) error {
				return addGasFees(helper.ctx, helper.mgr, tx)
			},
			expectError: false,
			validator: func(helper addGasFeeTestHelper, c *C) {
				expected := common.NewCoin(common.BTCAsset, cosmos.NewUint(2000))
				c.Assert(helper.mgr.GasMgr().GetGas(), HasLen, 1)
				c.Assert(helper.mgr.GasMgr().GetGas()[0].Equals(expected), Equals, true)
			},
		},
	}
	for _, tc := range testCases {
		helper := newAddGasFeeTestHelper(c)
		tx := tc.txCreator(helper)
		var err error
		if tc.runner == nil {
			err = addGasFees(helper.ctx, helper.mgr, tx)
		} else {
			err = tc.runner(helper, tx)
		}

		if err != nil && !tc.expectError {
			c.Errorf("test case: %s,didn't expect error however it got : %s", tc.name, err)
			c.FailNow()
		}
		if err == nil && tc.expectError {
			c.Errorf("test case: %s, expect error however it didn't", tc.name)
			c.FailNow()
		}
		if !tc.expectError && tc.validator != nil {
			tc.validator(helper, c)
			continue
		}
	}
}

func (s *HelperSuite) TestEmitPoolStageCostEvent(c *C) {
	ctx, mgr := setupManagerForTest(c)
	emitPoolBalanceChangedEvent(ctx,
		NewPoolMod(common.BTCAsset, cosmos.NewUint(1000), false, cosmos.ZeroUint(), false), "test", mgr)
	found := false
	for _, e := range ctx.EventManager().Events() {
		if strings.EqualFold(e.Type, types.PoolBalanceChangeEventType) {
			found = true
			break
		}
	}
	c.Assert(found, Equals, true)
}

func (s *HelperSuite) TestIsSynthMintPause(c *C) {
	ctx, mgr := setupManagerForTest(c)

	mgr.Keeper().SetMimir(ctx, constants.MaxSynthPerPoolDepth.String(), 1500)

	pool := types.Pool{
		Asset:        common.BTCAsset,
		BalanceAsset: cosmos.NewUint(100 * common.One),
		BalanceCacao: cosmos.NewUint(100 * common.One),
	}
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)

	coins := cosmos.NewCoins(cosmos.NewCoin("btc/btc", cosmos.NewInt(29*common.One))) // 29% utilization
	c.Assert(mgr.coinKeeper.MintCoins(ctx, ModuleName, coins), IsNil)

	c.Assert(isSynthMintPaused(ctx, mgr, common.BTCAsset, cosmos.ZeroUint()), IsNil)

	// A swap that outputs 0.5 synth BTC would not surpass the synth utilization cap (29% -> 29.5%)
	c.Assert(isSynthMintPaused(ctx, mgr, common.BTCAsset, cosmos.NewUint(0.5*common.One)), IsNil)
	// A swap that outputs 1 synth BTC would not surpass the synth utilization cap (29% -> 30%)
	c.Assert(isSynthMintPaused(ctx, mgr, common.BTCAsset, cosmos.NewUint(1*common.One)), IsNil)
	// A swap that outputs 1.1 synth BTC would surpass the synth utilization cap (29% -> 30.1%)
	c.Assert(isSynthMintPaused(ctx, mgr, common.BTCAsset, cosmos.NewUint(1.1*common.One)), NotNil)

	coins = cosmos.NewCoins(cosmos.NewCoin("btc/btc", cosmos.NewInt(1*common.One))) // 30% utilization
	c.Assert(mgr.coinKeeper.MintCoins(ctx, ModuleName, coins), IsNil)

	c.Assert(isSynthMintPaused(ctx, mgr, common.BTCAsset, cosmos.ZeroUint()), IsNil)

	coins = cosmos.NewCoins(cosmos.NewCoin("btc/btc", cosmos.NewInt(1*common.One))) // 31% utilization
	c.Assert(mgr.coinKeeper.MintCoins(ctx, ModuleName, coins), IsNil)

	c.Assert(isSynthMintPaused(ctx, mgr, common.BTCAsset, cosmos.ZeroUint()), NotNil)
}

func (s *HelperSuite) TestIsTradingHalt(c *C) {
	ctx, mgr := setupManagerForTest(c)
	txID := GetRandomTxHash()
	tx := common.NewTx(txID, GetRandomBTCAddress(), GetRandomBTCAddress(), common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(100))), common.Gas{
		common.NewCoin(common.BTCAsset, cosmos.NewUint(100)),
	}, "swap:BNB.BNB:"+GetRandomBNBAddress().String())
	memo, err := ParseMemoWithMAYANames(ctx, mgr.Keeper(), tx.Memo)
	c.Assert(err, IsNil)
	m, err := getMsgSwapFromMemo(memo.(SwapMemo), NewObservedTx(tx, ctx.BlockHeight(), GetRandomPubKey(), ctx.BlockHeight()), GetRandomBech32Addr())
	c.Assert(err, IsNil)

	txAddLiquidity := common.NewTx(txID, GetRandomBTCAddress(), GetRandomBTCAddress(), common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(100))), common.Gas{
		common.NewCoin(common.BTCAsset, cosmos.NewUint(100)),
	}, "add:BTC.BTC:"+GetRandomBaseAddress().String())
	memoAddExternal, err := ParseMemoWithMAYANames(ctx, mgr.Keeper(), txAddLiquidity.Memo)
	c.Assert(err, IsNil)
	mAddExternal, err := getMsgAddLiquidityFromMemo(ctx,
		memoAddExternal.(AddLiquidityMemo),
		NewObservedTx(txAddLiquidity, ctx.BlockHeight(), GetRandomPubKey(), ctx.BlockHeight()),
		GetRandomBech32Addr(), 0)

	c.Assert(err, IsNil)
	txAddRUNE := common.NewTx(txID, GetRandomBaseAddress(), GetRandomBaseAddress(), common.NewCoins(common.NewCoin(common.BaseNative, cosmos.NewUint(100))), common.Gas{
		common.NewCoin(common.BaseNative, cosmos.NewUint(100)),
	}, "add:BTC.BTC:"+GetRandomBTCAddress().String())
	memoAddRUNE, err := ParseMemoWithMAYANames(ctx, mgr.Keeper(), txAddRUNE.Memo)
	c.Assert(err, IsNil)
	mAddRUNE, err := getMsgAddLiquidityFromMemo(ctx,
		memoAddRUNE.(AddLiquidityMemo),
		NewObservedTx(txAddRUNE, ctx.BlockHeight(), GetRandomPubKey(), ctx.BlockHeight()),
		GetRandomBech32Addr(), 0)
	c.Assert(err, IsNil)

	mgr.Keeper().SetMAYAName(ctx, MAYAName{
		Name:              "testtest",
		ExpireBlockHeight: ctx.BlockHeight() + 1024,
		Owner:             GetRandomBech32Addr(),
		PreferredAsset:    common.BNBAsset,
		Aliases: []MAYANameAlias{
			{
				Chain:   common.BNBChain,
				Address: GetRandomBNBAddress(),
			},
		},
	})
	txWithThorname := common.NewTx(txID, GetRandomBTCAddress(), GetRandomBTCAddress(), common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(100))), common.Gas{
		common.NewCoin(common.BTCAsset, cosmos.NewUint(100)),
	}, "swap:BNB.BNB:testtest")
	memoWithThorname, err := ParseMemoWithMAYANames(ctx, mgr.Keeper(), txWithThorname.Memo)
	c.Assert(err, IsNil)
	mWithThorname, err := getMsgSwapFromMemo(memoWithThorname.(SwapMemo), NewObservedTx(txWithThorname, ctx.BlockHeight(), GetRandomPubKey(), ctx.BlockHeight()), GetRandomBech32Addr())
	c.Assert(err, IsNil)

	txSynth := common.NewTx(txID, GetRandomBaseAddress(), GetRandomBaseAddress(),
		common.NewCoins(common.NewCoin(common.BNBAsset.GetSyntheticAsset(), cosmos.NewUint(100))),
		common.Gas{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))},
		"swap:ETH.ETH:"+GetRandomBaseAddress().String())
	memoRedeemSynth, err := ParseMemoWithMAYANames(ctx, mgr.Keeper(), txSynth.Memo)
	c.Assert(err, IsNil)
	mRedeemSynth, err := getMsgSwapFromMemo(memoRedeemSynth.(SwapMemo), NewObservedTx(txSynth, ctx.BlockHeight(), GetRandomPubKey(), ctx.BlockHeight()), GetRandomBech32Addr())
	c.Assert(err, IsNil)

	c.Assert(isTradingHalt(ctx, m, mgr), Equals, false)
	c.Assert(isTradingHalt(ctx, mAddExternal, mgr), Equals, false)
	c.Assert(isTradingHalt(ctx, mAddRUNE, mgr), Equals, false)
	c.Assert(isTradingHalt(ctx, mWithThorname, mgr), Equals, false)
	c.Assert(isTradingHalt(ctx, mRedeemSynth, mgr), Equals, false)

	mgr.Keeper().SetMimir(ctx, "HaltTrading", 1)
	c.Assert(isTradingHalt(ctx, m, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mAddExternal, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mAddRUNE, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mWithThorname, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mRedeemSynth, mgr), Equals, true)
	c.Assert(mgr.Keeper().DeleteMimir(ctx, "HaltTrading"), IsNil)

	mgr.Keeper().SetMimir(ctx, "HaltBNBTrading", 1)
	c.Assert(isTradingHalt(ctx, m, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mAddExternal, mgr), Equals, false)
	c.Assert(isTradingHalt(ctx, mAddRUNE, mgr), Equals, false)
	c.Assert(isTradingHalt(ctx, mWithThorname, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mRedeemSynth, mgr), Equals, true)
	c.Assert(mgr.Keeper().DeleteMimir(ctx, "HaltBNBTrading"), IsNil)

	mgr.Keeper().SetMimir(ctx, "HaltBTCTrading", 1)
	c.Assert(isTradingHalt(ctx, m, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mAddExternal, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mAddRUNE, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mWithThorname, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mRedeemSynth, mgr), Equals, false)
	c.Assert(mgr.Keeper().DeleteMimir(ctx, "HaltBTCTrading"), IsNil)

	mgr.Keeper().SetMimir(ctx, "SolvencyHaltBTCChain", 1)
	c.Assert(isTradingHalt(ctx, m, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mAddExternal, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mAddRUNE, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mWithThorname, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mRedeemSynth, mgr), Equals, false)
	c.Assert(mgr.Keeper().DeleteMimir(ctx, "SolvencyHaltBTCChain"), IsNil)

	// Test per-pool halt trading
	mgr.Keeper().SetMimir(ctx, "HaltTrading-BTC-BTC", 1)
	c.Assert(isTradingHalt(ctx, m, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mAddExternal, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mAddRUNE, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mWithThorname, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mRedeemSynth, mgr), Equals, false)
	c.Assert(mgr.Keeper().DeleteMimir(ctx, "HaltTrading-BTC-BTC"), IsNil)
}

func (s *HelperSuite) TestUpdateTxOutGas(c *C) {
	ctx, mgr := setupManagerForTest(c)

	// Create ObservedVoter and add a TxOut
	txVoter := GetRandomObservedTxVoter()
	txOut := GetRandomTxOutItem()
	txVoter.Actions = append(txVoter.Actions, txOut)
	mgr.Keeper().SetObservedTxInVoter(ctx, txVoter)

	// Try to set new gas, should return error as TxOut InHash doesn't match
	newGas := common.Gas{common.NewCoin(common.RUNEAsset, cosmos.NewUint(2000000))}
	err := updateTxOutGas(ctx, mgr.K, txOut, newGas)
	c.Assert(err.Error(), Equals, fmt.Sprintf("fail to find tx out in ObservedTxVoter %s", txOut.InHash))

	// Update TxOut InHash to match, should update gas
	txOut.InHash = txVoter.TxID
	txVoter.Actions[1] = txOut
	mgr.Keeper().SetObservedTxInVoter(ctx, txVoter)

	// Err should be Nil
	err = updateTxOutGas(ctx, mgr.K, txOut, newGas)
	c.Assert(err, IsNil)

	// Keeper should have updated gas of TxOut in Actions
	txVoter, err = mgr.Keeper().GetObservedTxInVoter(ctx, txVoter.TxID)
	c.Assert(err, IsNil)

	didUpdate := false
	for _, item := range txVoter.Actions {
		if item.Equals(txOut) && item.MaxGas.Equals(newGas) {
			didUpdate = true
			break
		}
	}

	c.Assert(didUpdate, Equals, true)
}

func (s *HelperSuite) TestIsPeriodLastBlock(c *C) {
	ctx, _ := setupManagerForTest(c)
	var blockVar uint64

	blockVar = 10
	ctx = ctx.WithBlockHeight(10)
	result := IsPeriodLastBlock(ctx, blockVar)
	c.Assert(result, Equals, true)

	blockVar = 100
	ctx = ctx.WithBlockHeight(100)
	result = IsPeriodLastBlock(ctx, blockVar)
	c.Assert(result, Equals, true)

	blockVar = 90
	ctx = ctx.WithBlockHeight(89)
	result = IsPeriodLastBlock(ctx, blockVar)
	c.Assert(result, Equals, false)

	blockVar = 100
	ctx = ctx.WithBlockHeight(101)
	result = IsPeriodLastBlock(ctx, blockVar)
	c.Assert(result, Equals, false)
}

func (s *HelperSuite) TestCalculateMayaFundPercentage(c *C) {
	_, mgr := setupManagerForTest(c)
	inGas := common.NewCoin(common.BaseNative, cosmos.NewUint(100))

	outGas, mayaGas := CalculateMayaFundPercentage(inGas, mgr)
	c.Assert(outGas.Amount.Equal(cosmos.NewUint(90)), Equals, true)
	c.Assert(mayaGas.Amount.Equal(cosmos.NewUint(10)), Equals, true)
}

func (s *HelperSuite) TestRemoveBondAddress(c *C) {
	ctx, mgr := setupManagerForTest(c)
	addr := GetRandomBaseAddress()

	btcPool := NewPool()
	btcPool.Asset = common.BTCAsset
	btcPool.Status = PoolAvailable
	btcPool.BalanceCacao = cosmos.NewUint(10000 * common.One)
	btcPool.BalanceAsset = cosmos.NewUint(10000 * common.One)
	btcPool.LPUnits = cosmos.NewUint(10000 * common.One)
	c.Assert(mgr.Keeper().SetPool(ctx, btcPool), IsNil)

	btcLP := LiquidityProvider{
		Asset:        common.BTCAsset,
		CacaoAddress: addr,
		AssetAddress: GetRandomBTCAddress(),
		PendingCacao: cosmos.ZeroUint(),
		Units:        cosmos.NewUint(2000 * common.One),
	}
	mgr.Keeper().SetLiquidityProvider(ctx, btcLP)

	c.Assert(removeBondAddress(ctx, mgr, addr), IsNil)

	liquidityPools := GetLiquidityPools(mgr.GetVersion())
	liquidityProviders, err := mgr.Keeper().GetLiquidityProviderByAssets(ctx, liquidityPools, addr)
	c.Assert(err, IsNil)

	for _, liquidityProvider := range liquidityProviders {
		c.Assert(liquidityProvider.NodeBondAddress, IsNil)
	}
}

func (s *HelperSuite) TestisWithDrawDaysLimit(c *C) {
	ctx, mgr := setupManagerForTest(c)
	ctx = ctx.WithBlockHeight(50)
	cv := mgr.GetConstants()

	btcPool := NewPool()
	btcPool.Asset = common.BTCAsset
	btcPool.Status = PoolAvailable
	btcPool.BalanceCacao = cosmos.NewUint(10000 * common.One)
	btcPool.BalanceAsset = cosmos.NewUint(10000 * common.One)
	btcPool.LPUnits = cosmos.NewUint(10000 * common.One)
	c.Assert(mgr.Keeper().SetPool(ctx, btcPool), IsNil)

	btcLPDefaultTier := LiquidityProvider{
		Asset:        common.BTCAsset,
		CacaoAddress: GetRandomBaseAddress(),
		AssetAddress: GetRandomBTCAddress(),
		PendingCacao: cosmos.ZeroUint(),
		Units:        cosmos.NewUint(2000 * common.One),
	}
	mgr.Keeper().SetLiquidityProvider(ctx, btcLPDefaultTier)

	// test with default tier and no liquidity auction
	c.Assert(isWithinWithdrawDaysLimit(ctx, mgr, cv, btcLPDefaultTier.CacaoAddress), Equals, false)

	mgr.Keeper().SetMimir(ctx, constants.LiquidityAuction.String(), 100)

	btcLPTier1 := LiquidityProvider{
		Asset:        common.BTCAsset,
		CacaoAddress: GetRandomBaseAddress(),
		AssetAddress: GetRandomBTCAddress(),
		PendingCacao: cosmos.ZeroUint(),
		Units:        cosmos.NewUint(2000 * common.One),
	}
	mgr.Keeper().SetLiquidityProvider(ctx, btcLPTier1)
	c.Assert(mgr.Keeper().SetLiquidityAuctionTier(ctx, btcLPTier1.CacaoAddress, 1), IsNil)

	// test with tier 1 and ctx block = 50, LA = 100
	c.Assert(isWithinWithdrawDaysLimit(ctx, mgr, cv, btcLPTier1.CacaoAddress), Equals, false)
	ctx = ctx.WithBlockHeight(101)

	btcLPTier2 := LiquidityProvider{
		Asset:        common.BTCAsset,
		CacaoAddress: GetRandomBaseAddress(),
		AssetAddress: GetRandomBTCAddress(),
		PendingCacao: cosmos.ZeroUint(),
		Units:        cosmos.NewUint(2000 * common.One),
	}
	mgr.Keeper().SetLiquidityProvider(ctx, btcLPTier2)
	c.Assert(mgr.Keeper().SetLiquidityAuctionTier(ctx, btcLPTier2.CacaoAddress, 2), IsNil)

	// test with tier 2 and ctx block = 100
	c.Assert(isWithinWithdrawDaysLimit(ctx, mgr, cv, btcLPTier2.CacaoAddress), Equals, true)

	btcLPTier3 := LiquidityProvider{
		Asset:        common.BTCAsset,
		CacaoAddress: GetRandomBaseAddress(),
		AssetAddress: GetRandomBTCAddress(),
		PendingCacao: cosmos.ZeroUint(),
		Units:        cosmos.NewUint(2000 * common.One),
	}
	mgr.Keeper().SetLiquidityProvider(ctx, btcLPTier3)
	c.Assert(mgr.Keeper().SetLiquidityAuctionTier(ctx, btcLPTier3.CacaoAddress, 3), IsNil)

	// Test with tier 3 and ctx block = 100
	c.Assert(isWithinWithdrawDaysLimit(ctx, mgr, cv, btcLPTier3.CacaoAddress), Equals, true)

	withdrawDaysTier1 := mgr.Keeper().GetConfigInt64(ctx, constants.WithdrawDaysTier1)
	withdrawDaysTier2 := mgr.Keeper().GetConfigInt64(ctx, constants.WithdrawDaysTier2)
	withdrawDaysTier3 := mgr.Keeper().GetConfigInt64(ctx, constants.WithdrawDaysTier3)
	blocksPerDay := cv.GetInt64Value(constants.BlocksPerDay)
	liquidityAuction, err := mgr.Keeper().GetMimir(ctx, constants.LiquidityAuction.String())
	c.Assert(err, IsNil)

	// Tier 1 days off
	ctx = ctx.WithBlockHeight(liquidityAuction + withdrawDaysTier1*blocksPerDay + 1)
	c.Assert(isWithinWithdrawDaysLimit(ctx, mgr, cv, btcLPTier1.CacaoAddress), Equals, false)
	// Tier 2 days off
	ctx = ctx.WithBlockHeight(liquidityAuction + withdrawDaysTier2*blocksPerDay + 1)
	c.Assert(isWithinWithdrawDaysLimit(ctx, mgr, cv, btcLPTier2.CacaoAddress), Equals, false)
	// Tier 3 days off
	ctx = ctx.WithBlockHeight(liquidityAuction + withdrawDaysTier3*blocksPerDay + 1)
	c.Assert(isWithinWithdrawDaysLimit(ctx, mgr, cv, btcLPTier3.CacaoAddress), Equals, false)
}

func (s *HelperSuite) TestGetWithdrawLimit(c *C) {
	ctx, mgr := setupManagerForTest(c)
	cv := mgr.GetConstants()

	btcPool := NewPool()
	btcPool.Asset = common.BTCAsset
	btcPool.Status = PoolAvailable
	btcPool.BalanceCacao = cosmos.NewUint(10000 * common.One)
	btcPool.BalanceAsset = cosmos.NewUint(10000 * common.One)
	btcPool.LPUnits = cosmos.NewUint(10000 * common.One)
	c.Assert(mgr.Keeper().SetPool(ctx, btcPool), IsNil)

	btcLPNoTier := LiquidityProvider{
		Asset:        common.BTCAsset,
		CacaoAddress: GetRandomBaseAddress(),
		AssetAddress: GetRandomBTCAddress(),
		PendingCacao: cosmos.ZeroUint(),
		Units:        cosmos.NewUint(2000 * common.One),
	}
	mgr.Keeper().SetLiquidityProvider(ctx, btcLPNoTier)
	// No tier, can withdraw without limit
	withdrawLimit, err := getWithdrawLimit(ctx, mgr, cv, btcLPNoTier.CacaoAddress)
	c.Assert(err, IsNil)
	c.Assert(withdrawLimit, Equals, int64(10000))

	btcLPTier1 := LiquidityProvider{
		Asset:        common.BTCAsset,
		CacaoAddress: GetRandomBaseAddress(),
		AssetAddress: GetRandomBTCAddress(),
		PendingCacao: cosmos.ZeroUint(),
		Units:        cosmos.NewUint(2000 * common.One),
	}
	mgr.Keeper().SetLiquidityProvider(ctx, btcLPTier1)
	c.Assert(mgr.Keeper().SetLiquidityAuctionTier(ctx, btcLPTier1.CacaoAddress, 1), IsNil)
	// Test tier 1 with value of WithdrawLimitTier1
	withdrawLimit, err = getWithdrawLimit(ctx, mgr, cv, btcLPTier1.CacaoAddress)
	c.Assert(err, IsNil)
	c.Assert(withdrawLimit, Equals, int64(50))

	btcLPTier2 := LiquidityProvider{
		Asset:        common.BTCAsset,
		CacaoAddress: GetRandomBaseAddress(),
		AssetAddress: GetRandomBTCAddress(),
		PendingCacao: cosmos.ZeroUint(),
		Units:        cosmos.NewUint(2000 * common.One),
	}
	mgr.Keeper().SetLiquidityProvider(ctx, btcLPTier2)
	c.Assert(mgr.Keeper().SetLiquidityAuctionTier(ctx, btcLPTier2.CacaoAddress, 2), IsNil)
	// Test tier 2 withvalue of WithdrawLimitTier2
	withdrawLimit, err = getWithdrawLimit(ctx, mgr, cv, btcLPTier2.CacaoAddress)
	c.Assert(err, IsNil)
	c.Assert(withdrawLimit, Equals, int64(150))

	btcLPTier3 := LiquidityProvider{
		Asset:        common.BTCAsset,
		CacaoAddress: GetRandomBaseAddress(),
		AssetAddress: GetRandomBTCAddress(),
		PendingCacao: cosmos.ZeroUint(),
		Units:        cosmos.NewUint(2000 * common.One),
	}
	mgr.Keeper().SetLiquidityProvider(ctx, btcLPTier3)
	c.Assert(mgr.Keeper().SetLiquidityAuctionTier(ctx, btcLPTier3.CacaoAddress, 3), IsNil)
	// Test tier 3 with value of WithdrawLimitTier3
	withdrawLimit, err = getWithdrawLimit(ctx, mgr, cv, btcLPTier3.CacaoAddress)
	c.Assert(err, IsNil)
	c.Assert(withdrawLimit, Equals, int64(450))
}

func (s *HelperSuite) TestUpdateTxOutGasRate(c *C) {
	ctx, mgr := setupManagerForTest(c)

	// Create ObservedVoter and add a TxOut
	txVoter := GetRandomObservedTxVoter()
	txOut := GetRandomTxOutItem()
	txVoter.Actions = append(txVoter.Actions, txOut)
	mgr.Keeper().SetObservedTxInVoter(ctx, txVoter)

	// Try to set new gas rate, should return error as TxOut InHash doesn't match
	newGasRate := int64(25)
	err := updateTxOutGasRate(ctx, mgr.K, txOut, newGasRate)
	c.Assert(err.Error(), Equals, fmt.Sprintf("fail to find tx out in ObservedTxVoter %s", txOut.InHash))

	// Update TxOut InHash to match, should update gas
	txOut.InHash = txVoter.TxID
	txVoter.Actions[1] = txOut
	mgr.Keeper().SetObservedTxInVoter(ctx, txVoter)

	// Err should be Nil
	err = updateTxOutGasRate(ctx, mgr.K, txOut, newGasRate)
	c.Assert(err, IsNil)

	// Now that the actions have been updated (dependent on Equals which checks GasRate),
	// update the GasRate in the outbound queue item.
	txOut.GasRate = newGasRate

	// Keeper should have updated gas of TxOut in Actions
	txVoter, err = mgr.Keeper().GetObservedTxInVoter(ctx, txVoter.TxID)
	c.Assert(err, IsNil)

	didUpdate := false
	for _, item := range txVoter.Actions {
		if item.Equals(txOut) && item.GasRate == newGasRate {
			didUpdate = true
			break
		}
	}

	c.Assert(didUpdate, Equals, true)
}

func (s *HelperSuite) TestPOLPoolValue(c *C) {
	ctx, mgr := setupManagerForTest(c)

	polAddress, err := mgr.Keeper().GetModuleAddress(ReserveName)
	c.Assert(err, IsNil)

	btcPool := NewPool()
	btcPool.Asset = common.BTCAsset
	btcPool.BalanceCacao = cosmos.NewUint(2000 * common.One)
	btcPool.BalanceAsset = cosmos.NewUint(20 * common.One)
	btcPool.LPUnits = cosmos.NewUint(1600)
	c.Assert(mgr.Keeper().SetPool(ctx, btcPool), IsNil)

	coin := common.NewCoin(common.BTCAsset.GetSyntheticAsset(), cosmos.NewUint(10*common.One))
	c.Assert(mgr.Keeper().MintToModule(ctx, ModuleName, coin), IsNil)

	lps := LiquidityProviders{
		{
			Asset:             btcPool.Asset,
			CacaoAddress:      GetRandomBNBAddress(),
			AssetAddress:      GetRandomBTCAddress(),
			LastAddHeight:     5,
			Units:             btcPool.LPUnits.QuoUint64(2),
			PendingCacao:      cosmos.ZeroUint(),
			PendingAsset:      cosmos.ZeroUint(),
			AssetDepositValue: cosmos.ZeroUint(),
			CacaoDepositValue: cosmos.ZeroUint(),
		},
		{
			Asset:             btcPool.Asset,
			CacaoAddress:      polAddress,
			AssetAddress:      common.NoAddress,
			LastAddHeight:     10,
			Units:             btcPool.LPUnits.QuoUint64(2),
			PendingCacao:      cosmos.ZeroUint(),
			PendingAsset:      cosmos.ZeroUint(),
			AssetDepositValue: cosmos.ZeroUint(),
			CacaoDepositValue: cosmos.ZeroUint(),
		},
	}
	for _, lp := range lps {
		mgr.Keeper().SetLiquidityProvider(ctx, lp)
	}

	value, err := polPoolValue(ctx, mgr)
	c.Assert(err, IsNil)
	c.Check(value.Uint64(), Equals, uint64(150023441162), Commentf("%d", value.Uint64()))
}

func (s *HelperSuite) TestSecurityBond(c *C) {
	ctx, _ := setupManagerForTest(c)
	nas := make(NodeAccounts, 0)
	keeper := &TestHelpersKeeper{
		nas: nas,
	}
	mgr := NewDummyMgrWithKeeper(keeper)
	c.Assert(getEffectiveSecurityBond(ctx, mgr, nas).Uint64(), Equals, uint64(0), Commentf("%d", getEffectiveSecurityBond(ctx, mgr, nas).Uint64()))

	nas = NodeAccounts{
		NodeAccount{Bond: cosmos.NewUint(10), BondAddress: GetRandomBaseAddress()},
	}
	keeper = &TestHelpersKeeper{
		nas: nas,
	}
	mgr = NewDummyMgrWithKeeper(keeper)
	c.Assert(getEffectiveSecurityBond(ctx, mgr, nas).Uint64(), Equals, uint64(10), Commentf("%d", getEffectiveSecurityBond(ctx, mgr, nas).Uint64()))

	nas = NodeAccounts{
		NodeAccount{Bond: cosmos.NewUint(10), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(20), BondAddress: GetRandomBaseAddress()},
	}
	keeper = &TestHelpersKeeper{
		nas: nas,
	}
	mgr = NewDummyMgrWithKeeper(keeper)
	c.Assert(getEffectiveSecurityBond(ctx, mgr, nas).Uint64(), Equals, uint64(30), Commentf("%d", getEffectiveSecurityBond(ctx, mgr, nas).Uint64()))

	nas = NodeAccounts{
		NodeAccount{Bond: cosmos.NewUint(10), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(20), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(30), BondAddress: GetRandomBaseAddress()},
	}
	keeper = &TestHelpersKeeper{
		nas: nas,
	}
	mgr = NewDummyMgrWithKeeper(keeper)
	c.Assert(getEffectiveSecurityBond(ctx, mgr, nas).Uint64(), Equals, uint64(30), Commentf("%d", getEffectiveSecurityBond(ctx, mgr, nas).Uint64()))

	nas = NodeAccounts{
		NodeAccount{Bond: cosmos.NewUint(10), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(20), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(30), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(40), BondAddress: GetRandomBaseAddress()},
	}
	keeper = &TestHelpersKeeper{
		nas: nas,
	}
	mgr = NewDummyMgrWithKeeper(keeper)
	c.Assert(getEffectiveSecurityBond(ctx, mgr, nas).Uint64(), Equals, uint64(60), Commentf("%d", getEffectiveSecurityBond(ctx, mgr, nas).Uint64()))

	nas = NodeAccounts{
		NodeAccount{Bond: cosmos.NewUint(10), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(20), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(30), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(40), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(50), BondAddress: GetRandomBaseAddress()},
	}
	keeper = &TestHelpersKeeper{
		nas: nas,
	}
	mgr = NewDummyMgrWithKeeper(keeper)
	c.Assert(getEffectiveSecurityBond(ctx, mgr, nas).Uint64(), Equals, uint64(100), Commentf("%d", getEffectiveSecurityBond(ctx, mgr, nas).Uint64()))

	nas = NodeAccounts{
		NodeAccount{Bond: cosmos.NewUint(10), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(20), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(30), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(40), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(50), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(60), BondAddress: GetRandomBaseAddress()},
	}
	keeper = &TestHelpersKeeper{
		nas: nas,
	}
	mgr = NewDummyMgrWithKeeper(keeper)
	c.Assert(getEffectiveSecurityBond(ctx, mgr, nas).Uint64(), Equals, uint64(100), Commentf("%d", getEffectiveSecurityBond(ctx, mgr, nas).Uint64()))
}

func (s *HelperSuite) TestGetHardBondCap(c *C) {
	ctx, _ := setupManagerForTest(c)
	nas := make(NodeAccounts, 0)
	keeper := &TestHelpersKeeper{
		nas: nas,
	}
	mgr := NewDummyMgrWithKeeper(keeper)
	c.Assert(getHardBondCap(ctx, mgr, nas).Uint64(), Equals, uint64(0), Commentf("%d", getHardBondCap(ctx, mgr, nas).Uint64()))

	nas = NodeAccounts{
		NodeAccount{Bond: cosmos.NewUint(10), BondAddress: GetRandomBaseAddress()},
	}
	keeper = &TestHelpersKeeper{
		nas: nas,
	}
	mgr = NewDummyMgrWithKeeper(keeper)
	c.Assert(getHardBondCap(ctx, mgr, nas).Uint64(), Equals, uint64(10), Commentf("%d", getHardBondCap(ctx, mgr, nas).Uint64()))

	nas = NodeAccounts{
		NodeAccount{Bond: cosmos.NewUint(10), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(20), BondAddress: GetRandomBaseAddress()},
	}
	keeper = &TestHelpersKeeper{
		nas: nas,
	}
	mgr = NewDummyMgrWithKeeper(keeper)
	c.Assert(getHardBondCap(ctx, mgr, nas).Uint64(), Equals, uint64(20), Commentf("%d", getHardBondCap(ctx, mgr, nas).Uint64()))

	nas = NodeAccounts{
		NodeAccount{Bond: cosmos.NewUint(10), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(20), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(30), BondAddress: GetRandomBaseAddress()},
	}
	keeper = &TestHelpersKeeper{
		nas: nas,
	}
	mgr = NewDummyMgrWithKeeper(keeper)
	c.Assert(getHardBondCap(ctx, mgr, nas).Uint64(), Equals, uint64(20), Commentf("%d", getHardBondCap(ctx, mgr, nas).Uint64()))

	nas = NodeAccounts{
		NodeAccount{Bond: cosmos.NewUint(10), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(20), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(30), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(40), BondAddress: GetRandomBaseAddress()},
	}
	keeper = &TestHelpersKeeper{
		nas: nas,
	}
	mgr = NewDummyMgrWithKeeper(keeper)
	c.Assert(getHardBondCap(ctx, mgr, nas).Uint64(), Equals, uint64(30), Commentf("%d", getHardBondCap(ctx, mgr, nas).Uint64()))

	nas = NodeAccounts{
		NodeAccount{Bond: cosmos.NewUint(10), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(20), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(30), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(40), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(50), BondAddress: GetRandomBaseAddress()},
	}
	keeper = &TestHelpersKeeper{
		nas: nas,
	}
	mgr = NewDummyMgrWithKeeper(keeper)
	c.Assert(getHardBondCap(ctx, mgr, nas).Uint64(), Equals, uint64(40), Commentf("%d", getHardBondCap(ctx, mgr, nas).Uint64()))

	nas = NodeAccounts{
		NodeAccount{Bond: cosmos.NewUint(10), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(20), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(30), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(40), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(50), BondAddress: GetRandomBaseAddress()},
		NodeAccount{Bond: cosmos.NewUint(60), BondAddress: GetRandomBaseAddress()},
	}
	keeper = &TestHelpersKeeper{
		nas: nas,
	}
	mgr = NewDummyMgrWithKeeper(keeper)
	c.Assert(getHardBondCap(ctx, mgr, nas).Uint64(), Equals, uint64(40), Commentf("%d", getHardBondCap(ctx, mgr, nas).Uint64()))
}

func (HandlerSuite) TestIsSignedByActiveNodeAccounts(c *C) {
	ctx, mgr := setupManagerForTest(c)

	r := isSignedByActiveNodeAccounts(ctx, mgr.Keeper(), []cosmos.AccAddress{})
	c.Check(r, Equals, false,
		Commentf("empty signers should return false"))

	nodeAddr := GetRandomBech32Addr()
	r = isSignedByActiveNodeAccounts(ctx, mgr.Keeper(), []cosmos.AccAddress{nodeAddr})
	c.Check(r, Equals, false,
		Commentf("empty node account should return false"))

	nodeAccount1 := GetRandomValidatorNode(NodeWhiteListed)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, nodeAccount1), IsNil)
	r = isSignedByActiveNodeAccounts(ctx, mgr.Keeper(), []cosmos.AccAddress{nodeAccount1.NodeAddress})
	c.Check(r, Equals, false,
		Commentf("non-active node account should return false"))

	nodeAccount1.Status = NodeActive
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, nodeAccount1), IsNil)
	r = isSignedByActiveNodeAccounts(ctx, mgr.Keeper(), []cosmos.AccAddress{nodeAccount1.NodeAddress})
	c.Check(r, Equals, true,
		Commentf("active node account should return true"))

	r = isSignedByActiveNodeAccounts(ctx, mgr.Keeper(), []cosmos.AccAddress{nodeAccount1.NodeAddress, nodeAddr})
	c.Check(r, Equals, false,
		Commentf("should return false if any signer is not an active validator"))

	nodeAccount1.Type = NodeTypeVault
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, nodeAccount1), IsNil)
	r = isSignedByActiveNodeAccounts(ctx, mgr.Keeper(), []cosmos.AccAddress{nodeAccount1.NodeAddress})
	c.Check(r, Equals, false,
		Commentf("non-validator node should return false"))

	asgardAddr := mgr.Keeper().GetModuleAccAddress(AsgardName)
	r = isSignedByActiveNodeAccounts(ctx, mgr.Keeper(), []cosmos.AccAddress{asgardAddr})
	c.Check(r, Equals, true,
		Commentf("asgard module address should return true"))
}

func (s *HelperSuite) TestIsLiquidityAuction(c *C) {
	ctx, mgr := setupManagerForTest(c)

	// default value is -1
	c.Assert(isLiquidityAuction(ctx, mgr.Keeper()), Equals, false)

	mgr.Keeper().SetMimir(ctx, constants.LiquidityAuction.String(), 50)
	c.Assert(isLiquidityAuction(ctx, mgr.Keeper()), Equals, true)

	ctx = ctx.WithBlockHeight(51)
	c.Assert(isLiquidityAuction(ctx, mgr.Keeper()), Equals, false)
}

func (s *HelperSuite) TestAffiliateShareCalculating(c *C) {
	ctx, mgr := setupManagerForTest(c)

	// happy path
	mgr.Keeper().SetMAYAName(ctx, NewMAYAName("subaffOfAff1", 50, nil, common.EmptyAsset, nil, cosmos.ZeroUint(), nil))
	mgr.Keeper().SetMAYAName(ctx, NewMAYAName("aff1", 50, nil, common.EmptyAsset, nil, cosmos.ZeroUint(), []types.MAYANameSubaffiliate{
		{Name: "subaffOfAff1", Bps: cosmos.NewUint(4000)},
	}))
	mgr.Keeper().SetMAYAName(ctx, NewMAYAName("aff2", 50, nil, common.EmptyAsset, nil, cosmos.ZeroUint(), nil))
	mgr.Keeper().SetMAYAName(ctx, NewMAYAName("main", 50, nil, common.EmptyAsset, nil, cosmos.ZeroUint(), []types.MAYANameSubaffiliate{
		{Name: "aff1", Bps: cosmos.NewUint(3000)},
		{Name: "aff2", Bps: cosmos.NewUint(2000)},
	}))
	shares := calculateAffiliateShares(ctx, mgr, cosmos.NewUint(10000), []string{"main"}, []cosmos.Uint{cosmos.NewUint(100)}, cosmos.ZeroUint())
	c.Assert(shares, HasLen, 4)
	// 40% of 30% (of aff1) goes to subaffOfAff1 (=12%)
	c.Assert(shares[0].mayaname.Name, Equals, "subaffOfAff1")
	c.Assert(shares[0].amount.Uint64(), Equals, uint64(12))
	// remaining 60% of 30% goes to aff1 (=18%)
	c.Assert(shares[1].mayaname.Name, Equals, "aff1")
	c.Assert(shares[1].amount.Uint64(), Equals, uint64(18))
	// 20% goes to aff2
	c.Assert(shares[2].mayaname.Name, Equals, "aff2")
	c.Assert(shares[2].amount.Uint64(), Equals, uint64(20))
	// remaining 50% to main
	c.Assert(shares[3].mayaname.Name, Equals, "main")
	c.Assert(shares[3].amount.Uint64(), Equals, uint64(50))

	// sub affiliates together are greater than 100%, ignore the second sub affiliate
	mgr.Keeper().SetMAYAName(ctx, NewMAYAName("aff1", 50, nil, common.EmptyAsset, nil, cosmos.ZeroUint(), nil))
	mgr.Keeper().SetMAYAName(ctx, NewMAYAName("aff2", 50, nil, common.EmptyAsset, nil, cosmos.ZeroUint(), nil))
	mgr.Keeper().SetMAYAName(ctx, NewMAYAName("aff3", 50, nil, common.EmptyAsset, nil, cosmos.ZeroUint(), nil))
	mgr.Keeper().SetMAYAName(ctx, NewMAYAName("main", 50, nil, common.EmptyAsset, nil, cosmos.ZeroUint(), []types.MAYANameSubaffiliate{
		{Name: "aff1", Bps: cosmos.NewUint(3000)},
		{Name: "aff2", Bps: cosmos.NewUint(2000)},
		{Name: "aff3", Bps: cosmos.NewUint(9000)},
	}))
	shares = calculateAffiliateShares(ctx, mgr, cosmos.NewUint(10000), []string{"main"}, []cosmos.Uint{cosmos.NewUint(100)}, cosmos.ZeroUint())
	c.Assert(shares, HasLen, 3)
	// 30% goes to aff2
	c.Assert(shares[0].mayaname.Name, Equals, "aff1")
	c.Assert(shares[0].amount.Uint64(), Equals, uint64(30))
	// 20% goes to aff2
	c.Assert(shares[1].mayaname.Name, Equals, "aff2")
	c.Assert(shares[1].amount.Uint64(), Equals, uint64(20))
	// remaining 50% to main
	c.Assert(shares[2].mayaname.Name, Equals, "main")
	c.Assert(shares[2].amount.Uint64(), Equals, uint64(50))
}

func (s *HelperSuite) TestAffiliateShareCalculating2(c *C) {
	ctx, mgr := setupManagerForTest(c)

	mgr.Keeper().SetMAYAName(ctx, NewMAYAName("frog", 999, nil, common.EmptyAsset, nil, cosmos.ZeroUint(), nil))
	mgr.Keeper().SetMAYAName(ctx, NewMAYAName("fox", 999, nil, common.EmptyAsset, nil, cosmos.ZeroUint(), []types.MAYANameSubaffiliate{
		{Name: "frog", Bps: cosmos.NewUint(4000)},
	}))
	mgr.Keeper().SetMAYAName(ctx, NewMAYAName("pig", 999, nil, common.EmptyAsset, nil, cosmos.ZeroUint(), nil))
	mgr.Keeper().SetMAYAName(ctx, NewMAYAName("cat", 999, nil, common.EmptyAsset, nil, cosmos.NewUint(150), []types.MAYANameSubaffiliate{
		{Name: "fox", Bps: cosmos.NewUint(3000)},
		{Name: "pig", Bps: cosmos.NewUint(3000)},
	}))
	amt := uint64(5000000000)
	mainBps := uint64(75)
	shares := calculateAffiliateShares(ctx, mgr, cosmos.NewUint(amt), []string{"cat"}, []cosmos.Uint{cosmos.NewUint(mainBps)}, cosmos.ZeroUint())
	c.Assert(shares, HasLen, 4)
	grossFee := amt * mainBps / 100_00

	// 40% of 30% of the fee
	expected := grossFee * 40_00 * 30_00 / 100_00 / 100_00
	c.Assert(shares[0].mayaname.Name, Equals, "frog")
	c.Assert(shares[0].amount.Uint64(), Equals, expected) // 4500000
	// 60% of 30% of the fee
	expected = grossFee * 60_00 * 30_00 / 100_00 / 100_00
	c.Assert(shares[1].mayaname.Name, Equals, "fox")
	c.Assert(shares[1].amount.Uint64(), Equals, expected) // 6750000
	// 30% goes to pig
	expected = grossFee * 30_00 / 100_00
	c.Assert(shares[2].mayaname.Name, Equals, "pig")
	c.Assert(shares[2].amount.Uint64(), Equals, expected) // 11250000
	// remaining 40% to cat
	expected = grossFee * 40_00 / 100_00
	c.Assert(shares[3].mayaname.Name, Equals, "cat")
	c.Assert(shares[3].amount.Uint64(), Equals, expected) // 15000000
}

func (s *HelperSuite) TestIsModuleAccAddress(c *C) {
	_, k := setupKeeperForTest(c)

	nonModuleAdd, err := GetRandomBaseAddress().AccAddress()
	c.Assert(err, IsNil)
	c.Assert(IsModuleAccAddress(k, nonModuleAdd), Equals, false)

	modules := []string{
		ModuleName,
		DefaultCodespace,
		ReserveName,
		AsgardName,
		BondName,
		AffiliateCollectorName,
		MayaFund,
		CACAOPoolName,
	}

	for _, module := range modules {
		moduleAccAdd := k.GetModuleAccAddress(module)
		c.Assert(IsModuleAccAddress(k, moduleAccAdd), Equals, true)
	}
}
