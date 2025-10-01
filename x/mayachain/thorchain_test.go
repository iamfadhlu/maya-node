package mayachain

import (
	"sort"
	"testing"

	"github.com/cosmos/cosmos-sdk/types"
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

func TestPackage(t *testing.T) { TestingT(t) }

var (
	bnbSingleTxFee = cosmos.NewUint(37500)
	btcSingleTxFee = cosmos.NewUint(50)
)

// Gas Fees
var BNBGasFeeSingleton = common.Gas{
	{Asset: common.BNBAsset, Amount: bnbSingleTxFee},
}

var BTCGasFeeSingleton = common.Gas{
	{Asset: common.BTCAsset, Amount: btcSingleTxFee},
}

type ThorchainSuite struct{}

var _ = Suite(&ThorchainSuite{})

func (s *ThorchainSuite) TestLiquidityProvision(c *C) {
	var err error
	ctx, keeper := setupKeeperForTest(c)
	user1rune := GetRandomBaseAddress()
	user1asset := GetRandomBNBAddress()
	user2rune := GetRandomBaseAddress()
	user2asset := GetRandomBNBAddress()
	tx := GetRandomTx()
	constAccessor := constants.GetConstantValues(GetCurrentVersion())
	c.Assert(err, IsNil)

	// create bnb pool
	pool := NewPool()
	pool.Asset = common.BNBAsset
	c.Assert(keeper.SetPool(ctx, pool), IsNil)
	addHandler := NewAddLiquidityHandler(NewDummyMgrWithKeeper(keeper))
	// liquidity provider for user1
	err = addHandler.addLiquidity(ctx, common.BNBAsset, cosmos.NewUint(100*common.One), cosmos.NewUint(100*common.One), user1rune, user1asset, tx, false, constAccessor, 0)
	c.Assert(err, IsNil)
	err = addHandler.addLiquidity(ctx, common.BNBAsset, cosmos.NewUint(100*common.One), cosmos.NewUint(100*common.One), user1rune, user1asset, tx, false, constAccessor, 0)
	c.Assert(err, IsNil)
	lp1, err := keeper.GetLiquidityProvider(ctx, common.BNBAsset, user1rune)
	c.Assert(err, IsNil)
	c.Check(lp1.Units.IsZero(), Equals, false)

	// liquidity provider for user2
	err = addHandler.addLiquidity(ctx, common.BNBAsset, cosmos.NewUint(75*common.One), cosmos.NewUint(75*common.One), user2rune, user2asset, tx, false, constAccessor, 0)
	c.Assert(err, IsNil)
	err = addHandler.addLiquidity(ctx, common.BNBAsset, cosmos.NewUint(75*common.One), cosmos.NewUint(75*common.One), user2rune, user2asset, tx, false, constAccessor, 0)
	c.Assert(err, IsNil)
	lp2, err := keeper.GetLiquidityProvider(ctx, common.BNBAsset, user2rune)
	c.Assert(err, IsNil)
	c.Check(lp2.Units.IsZero(), Equals, false)

	// withdraw for user1
	msg := NewMsgWithdrawLiquidity(GetRandomTx(), user1rune, cosmos.NewUint(10000), common.BNBAsset, common.EmptyAsset, GetRandomBech32Addr())
	_, _, _, _, _, err = withdraw(ctx, *msg, NewDummyMgrWithKeeper(keeper))
	c.Assert(err, IsNil)
	lp1, err = keeper.GetLiquidityProvider(ctx, common.BNBAsset, user1rune)
	c.Assert(err, IsNil)
	c.Check(lp1.PendingAsset.IsZero(), Equals, true)

	// withdraw for user2
	msg = NewMsgWithdrawLiquidity(GetRandomTx(), user2rune, cosmos.NewUint(10000), common.BNBAsset, common.EmptyAsset, GetRandomBech32Addr())
	_, _, _, _, _, err = withdraw(ctx, *msg, NewDummyMgrWithKeeper(keeper))
	c.Assert(err, IsNil)
	lp2, err = keeper.GetLiquidityProvider(ctx, common.BNBAsset, user2rune)
	c.Assert(err, IsNil)
	c.Check(lp2.PendingAsset.IsZero(), Equals, true)

	// check pool is now empty
	pool, err = keeper.GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Check(pool.BalanceCacao.IsZero(), Equals, true)
	remainGas := uint64(37500)
	c.Check(pool.BalanceAsset.Uint64(), Equals, remainGas) // leave a little behind for gas
	c.Check(pool.PendingInboundAsset.IsZero(), Equals, true)

	// liquidity provider for user1, again
	err = addHandler.addLiquidity(ctx, common.BNBAsset, cosmos.NewUint(100*common.One), cosmos.NewUint(100*common.One), user1rune, user1asset, tx, false, constAccessor, 0)
	c.Assert(err, IsNil)
	err = addHandler.addLiquidity(ctx, common.BNBAsset, cosmos.NewUint(100*common.One), cosmos.NewUint(100*common.One), user1rune, user1asset, tx, false, constAccessor, 0)
	c.Assert(err, IsNil)
	lp1, err = keeper.GetLiquidityProvider(ctx, common.BNBAsset, user1rune)
	c.Assert(err, IsNil)
	c.Check(lp1.Units.IsZero(), Equals, false)

	// check pool is NOT empty
	pool, err = keeper.GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Check(pool.BalanceCacao.Equal(cosmos.NewUint(200*common.One)), Equals, true)
	c.Check(pool.BalanceAsset.Equal(cosmos.NewUint(20000000000+remainGas)), Equals, true, Commentf("%d", pool.BalanceAsset.Uint64()))
	c.Check(pool.LPUnits.IsZero(), Equals, false)
}

func (s *ThorchainSuite) TestLiquidityAuction(c *C) {
	var err error
	ctx, keeper := setupKeeperForTest(c)
	user1rune := GetRandomBaseAddress()
	user1asset := GetRandomBTCAddress()
	user2rune := GetRandomBaseAddress()
	user2asset := GetRandomBTCAddress()
	tx := GetRandomTx()
	constAccessor := constants.GetConstantValues(GetCurrentVersion())
	c.Assert(err, IsNil)
	// minimumBasePoolDepth := constAccessor.GetInt64Value(constants.MinRunePoolDepth)

	// create btc pool
	pool := NewPool()
	pool.Asset = common.BTCAsset
	c.Assert(keeper.SetPool(ctx, pool), IsNil)
	addHandler := NewAddLiquidityHandler(NewDummyMgrWithKeeper(keeper))
	donateHandler := NewDonateHandler(NewDummyMgrWithKeeper(keeper))
	// setup liquidity auction until block 20
	keeper.SetMimir(ctx, constants.LiquidityAuction.String(), 20)

	// liquidity provider for user1
	err = addHandler.addLiquidity(ctx, common.BTCAsset, cosmos.ZeroUint(), cosmos.NewUint(3*common.One), user1rune, user1asset, tx, true, constAccessor, 1)
	c.Assert(err, IsNil)
	lp1, err := keeper.GetLiquidityProvider(ctx, common.BTCAsset, user1rune)
	c.Assert(err, IsNil)
	c.Check(lp1.Units.IsZero(), Equals, true)
	c.Check(lp1.PendingAsset.Equal(cosmos.NewUint(3*common.One)), Equals, true, Commentf("expected %d, got %d", 100*common.One, lp1.PendingAsset.Uint64()))
	lp1LATier, err := keeper.GetLiquidityAuctionTier(ctx, lp1.CacaoAddress)
	c.Assert(err, IsNil)
	c.Assert(lp1LATier, Equals, int64(1))

	// liquidity provider for user2
	err = addHandler.addLiquidity(ctx, common.BTCAsset, cosmos.ZeroUint(), cosmos.NewUint(1*common.One), user2rune, user2asset, tx, true, constAccessor, 0)
	c.Assert(err, IsNil)
	lp2, err := keeper.GetLiquidityProvider(ctx, common.BTCAsset, user2rune)
	c.Assert(err, IsNil)
	c.Check(lp2.PendingAsset.Equal(cosmos.NewUint(1*common.One)), Equals, true)
	c.Check(lp2.Units.IsZero(), Equals, true)
	lp2LATier, err := keeper.GetLiquidityAuctionTier(ctx, lp2.CacaoAddress)
	c.Assert(err, IsNil)
	c.Assert(lp2LATier, Equals, int64(3))

	// withdraw for user1 won't work since he is tier1
	msg := NewMsgWithdrawLiquidity(GetRandomTx(), user1rune, cosmos.NewUint(10000), common.BTCAsset, common.EmptyAsset, GetRandomBech32Addr())
	_, _, _, _, _, err = withdraw(ctx, *msg, NewDummyMgrWithKeeper(keeper))
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "tier1 cannot withdraw during liquidity auction")
	lp1, err = keeper.GetLiquidityProvider(ctx, common.BTCAsset, user1rune)
	c.Assert(err, IsNil)
	c.Assert(lp1.PendingAsset.Uint64(), Equals, uint64(3*common.One))

	// withdraw for user2 should work and withdraw 100%
	msg = NewMsgWithdrawLiquidity(GetRandomTx(), user2rune, cosmos.NewUint(10000), common.BTCAsset, common.EmptyAsset, GetRandomBech32Addr())
	_, _, _, _, _, err = withdraw(ctx, *msg, NewDummyMgrWithKeeper(keeper))
	c.Assert(err, IsNil)
	lp2, err = keeper.GetLiquidityProvider(ctx, common.BTCAsset, user2rune)
	c.Assert(err, IsNil)
	c.Check(lp2.PendingAsset.IsZero(), Equals, true)

	// liquidity provider for user2, again and change tier
	err = addHandler.addLiquidity(ctx, common.BTCAsset, cosmos.ZeroUint(), cosmos.NewUint(7*common.One), user2rune, user2asset, tx, true, constAccessor, 2)
	c.Assert(err, IsNil)
	lp2, err = keeper.GetLiquidityProvider(ctx, common.BTCAsset, user2rune)
	c.Assert(err, IsNil)
	c.Check(lp2.PendingAsset.Uint64(), Equals, uint64(7*common.One))
	c.Check(lp2.Units.IsZero(), Equals, true)
	lp2LATier, err = keeper.GetLiquidityAuctionTier(ctx, lp2.CacaoAddress)
	c.Assert(err, IsNil)
	c.Assert(lp2LATier, Equals, int64(2))

	pool, err = keeper.GetPool(ctx, common.BTCAsset)
	c.Assert(err, IsNil)
	c.Check(pool.BalanceCacao.IsZero(), Equals, true)
	c.Check(pool.PendingInboundAsset.Uint64(), Equals, uint64(10*common.One)) // leave a little behind for gas
	c.Assert(pool.LPUnits.IsZero(), Equals, true)

	mayaAcc, err := cosmos.AccAddressFromBech32(ADMINS[0])
	c.Assert(err, IsNil)
	msgDonate := NewMsgDonate(GetRandomTx(), common.BTCAsset, cosmos.NewUint(1000000*common.One), cosmos.ZeroUint(), mayaAcc)
	_, err = donateHandler.Run(ctx, msgDonate)
	c.Assert(err, IsNil)

	pool, err = keeper.GetPool(ctx, common.BTCAsset)
	c.Assert(err, IsNil)
	c.Check(pool.BalanceCacao.Uint64(), Equals, uint64(1000000*common.One))
	c.Check(pool.LPUnits.Uint64(), Equals, uint64(1000000*common.One))

	lp1, err = keeper.GetLiquidityProvider(ctx, common.BTCAsset, user1rune)
	c.Assert(err, IsNil)
	c.Check(lp1.Units.Uint64(), Equals, uint64(370000*common.One), Commentf("expected %s got %s", cosmos.NewUint(370000*common.One), lp1.Units))
	lp2, err = keeper.GetLiquidityProvider(ctx, common.BTCAsset, user2rune)
	c.Assert(err, IsNil)
	c.Check(lp2.Units.IsZero(), Equals, false)
	c.Assert(lp2.Units.Uint64(), Equals, uint64(630000*common.One), Commentf("expected %s got %s", cosmos.NewUint(630000*common.One), lp2.Units))
}

func (s *ThorchainSuite) TestChurn(c *C) {
	ctx, mgr := setupManagerForTest(c)
	ver := GetCurrentVersion()
	consts := constants.GetConstantValues(ver)
	// create starting point, vault and four node active node accounts
	vault := GetRandomVault()
	vault.AddFunds(common.Coins{
		common.NewCoin(common.BaseAsset(), cosmos.NewUint(100*common.One)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(79*common.One)),
	})
	c.Assert(mgr.Keeper().SaveNetworkFee(ctx, common.BNBChain, NetworkFee{
		Chain:              common.BNBChain,
		TransactionSize:    1,
		TransactionFeeRate: 37500,
	}), IsNil)
	c.Assert(mgr.Keeper().SetPool(ctx, Pool{
		BalanceCacao: cosmos.NewUint(common.One),
		BalanceAsset: cosmos.NewUint(common.One),
		Asset:        common.BNBAsset,
		LPUnits:      cosmos.NewUint(common.One),
		Status:       PoolAvailable,
	}), IsNil)
	addresses := make([]cosmos.AccAddress, 4)
	var existingValidators []string
	for i := 0; i <= 3; i++ {
		na := GetRandomValidatorNode(NodeActive)
		bp := NewBondProviders(na.NodeAddress)
		acc, err := na.BondAddress.AccAddress()
		c.Assert(err, IsNil)
		bp.Providers = append(bp.Providers, NewBondProvider(acc))
		bp.Providers[0].Bonded = true
		SetupLiquidityBondForTest(c, ctx, mgr.Keeper(), common.BNBAsset, na.BondAddress, na, cosmos.NewUint(100*common.One))
		c.Assert(mgr.Keeper().SetBondProviders(ctx, bp), IsNil)
		addresses[i] = na.NodeAddress
		na.SignerMembership = common.PubKeys{vault.PubKey}.Strings()
		if i == 0 { // give the first node account slash points
			na.RequestedToLeave = true
		}
		pk, err := cosmos.GetPubKeyFromBech32(cosmos.Bech32PubKeyTypeConsPub, na.ValidatorConsPubKey)
		if err != nil {
			ctx.Logger().Error("fail to parse consensus public key", "key", na.ValidatorConsPubKey, "error", err)
			continue
		}
		caddr := types.ValAddress(pk.Address()).String()
		existingValidators = append(existingValidators, caddr)
		vault.Membership = append(vault.Membership, na.PubKeySet.Secp256k1.String())
		c.Assert(mgr.Keeper().SetNodeAccount(ctx, na), IsNil)
	}
	c.Assert(mgr.Keeper().SetVault(ctx, vault), IsNil)

	// create new node account to rotate in
	na := GetRandomValidatorNode(NodeReady)
	bp := NewBondProviders(na.NodeAddress)
	acc, err := na.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp.Providers = append(bp.Providers, NewBondProvider(acc))
	bp.Providers[0].Bonded = true
	SetupLiquidityBondForTest(c, ctx, mgr.Keeper(), common.BNBAsset, na.BondAddress, na, cosmos.NewUint(100*common.One))
	c.Assert(mgr.Keeper().SetBondProviders(ctx, bp), IsNil)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, na), IsNil)

	// trigger marking bad actors as well as a keygen
	rotateHeight := consts.GetInt64Value(constants.ChurnInterval) + vault.BlockHeight
	ctx = ctx.WithBlockHeight(rotateHeight)
	valMgr := newValidatorMgrVCUR(mgr.Keeper(), mgr.NetworkMgr(), mgr.TxOutStore(), mgr.EventMgr())
	c.Assert(valMgr.BeginBlock(ctx, mgr, existingValidators), IsNil)

	// check we've created a keygen, with the correct members
	keygenBlock, err := mgr.Keeper().GetKeygenBlock(ctx, ctx.BlockHeight())
	c.Assert(err, IsNil)
	c.Assert(keygenBlock.IsEmpty(), Equals, false)
	expected := append(vault.Membership[1:], na.PubKeySet.Secp256k1.String()) // nolint
	c.Assert(keygenBlock.Keygens, HasLen, 1)
	keygen := keygenBlock.Keygens[0]
	// sort our slices so they are in the same order
	sort.Slice(expected, func(i, j int) bool { return expected[i] < expected[j] })
	sort.Slice(keygen.Members, func(i, j int) bool { return keygen.Members[i] < keygen.Members[j] })
	c.Assert(expected, HasLen, len(keygen.Members))
	for i := range expected {
		c.Assert(expected[i], Equals, keygen.Members[i], Commentf("%d: %s <==> %s", i, expected[i], keygen.Members[i]))
	}

	// generate a tss keygen handler event
	newVaultPk := GetRandomPubKey()
	signer, err := common.PubKey(keygen.Members[0]).GetThorAddress()
	c.Assert(err, IsNil)
	keygenTime := int64(1024)
	msg, err := NewMsgTssPoolV2(keygen.Members, newVaultPk, makeFakeSignature(c), AsgardKeygen, ctx.BlockHeight(), nil, common.Chains{common.BaseAsset().Chain}.Strings(), signer, keygenTime, GetRandomPubKey(), []byte("backup"))
	c.Assert(err, IsNil)
	tssHandler := NewTssHandler(mgr)

	voter := NewTssVoter(msg.ID, msg.PubKeys, msg.PoolPubKey, msg.PoolPubKeyEddsa)
	signers := make([]string, len(msg.PubKeys)-1)
	for i, pk := range msg.PubKeys {
		if i == 0 {
			continue
		}
		var sig cosmos.AccAddress
		sig, err = common.PubKey(pk).GetThorAddress()
		c.Assert(err, IsNil)
		signers[i-1] = sig.String()
	}
	voter.Signers = signers // ensure we have consensus, so handler is properly executed
	mgr.Keeper().SetTssVoter(ctx, voter)

	_, err = tssHandler.Run(ctx, msg)
	c.Assert(err, IsNil)

	// check that we've rotated our vaults
	vault1, err := mgr.Keeper().GetVault(ctx, vault.PubKey)
	c.Assert(err, IsNil)
	c.Assert(vault1.Status, Equals, RetiringVault) // first vault should now be retiring
	vault2, err := mgr.Keeper().GetVault(ctx, newVaultPk)
	c.Assert(err, IsNil)
	c.Assert(vault2.Status, Equals, ActiveVault) // new vault should now be active
	c.Assert(vault2.Membership, HasLen, 4)

	// check our validators get rotated appropriately
	validators := valMgr.EndBlock(ctx, mgr)
	nas, err := mgr.Keeper().ListActiveValidators(ctx)
	c.Assert(err, IsNil)
	c.Assert(nas, HasLen, 4)
	c.Assert(validators, HasLen, 2)
	// ensure that the first one is rotated out and the new one is rotated in
	standby, err := mgr.Keeper().GetNodeAccount(ctx, addresses[0])
	c.Assert(err, IsNil)
	c.Check(standby.Status == NodeStandby, Equals, true)
	na, err = mgr.Keeper().GetNodeAccount(ctx, na.NodeAddress)
	c.Assert(err, IsNil)
	c.Check(na.Status == NodeActive, Equals, true)

	// check that the funds can be migrated from the retiring vault to the new
	// vault
	ctx = ctx.WithBlockHeight(vault1.StatusSince)
	err = mgr.NetworkMgr().EndBlock(ctx, mgr) // should attempt to send 20% of the coin values
	c.Assert(err, IsNil)
	vault, err = mgr.Keeper().GetVault(ctx, vault1.PubKey)
	c.Assert(err, IsNil)
	items, err := mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 1)
	item := items[0]
	c.Check(item.Coin.Amount.Uint64(), Equals, uint64(1579962500), Commentf("%d", item.Coin.Amount.Uint64()))
	// check we empty the rest at the last migration event
	migrateInterval := consts.GetInt64Value(constants.FundMigrationInterval)
	ctx = ctx.WithBlockHeight(vault.StatusSince + (migrateInterval * 7))
	vault, err = mgr.Keeper().GetVault(ctx, vault.PubKey)
	c.Assert(err, IsNil)
	vault.PendingTxBlockHeights = nil
	c.Assert(mgr.Keeper().SetVault(ctx, vault), IsNil)
	c.Check(mgr.NetworkMgr().EndBlock(ctx, mgr), IsNil) // should attempt to send 100% of the coin values
	items, err = mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 1, Commentf("%d", len(items)))
	item = items[0]
	c.Check(item.Coin.Amount.Uint64(), Equals, uint64(7899962500), Commentf("%d", item.Coin.Amount.Uint64()))
}

func (s *ThorchainSuite) TestRagnarok(c *C) {
	SetupConfigForTest()
	var err error
	ctx, mgr := setupManagerForTest(c)
	ctx = ctx.WithBlockHeight(10)
	ver := GetCurrentVersion()
	consts := constants.GetConstantValues(ver)
	FundModule(c, ctx, mgr.Keeper(), BondName, 100*common.One)
	c.Assert(mgr.Keeper().SaveNetworkFee(ctx, common.BNBChain, NetworkFee{
		Chain:              common.BNBChain,
		TransactionSize:    1,
		TransactionFeeRate: bnbSingleTxFee.Uint64(),
	}), IsNil)

	// create active asgard vault
	asgard := GetRandomVault()
	c.Assert(mgr.Keeper().SetVault(ctx, asgard), IsNil)

	// create pools
	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.Status = PoolAvailable
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)
	boltAsset, err := common.NewAsset("BNB.BOLT-123")
	c.Assert(err, IsNil)
	pool.Asset = boltAsset
	pool.Status = PoolAvailable
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)
	pool = NewPool()
	pool.Asset = common.BTCAsset
	pool.Status = PoolAvailable
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)
	addHandler := NewAddLiquidityHandler(mgr)
	// add liquidity providers
	lp1 := GetRandomBaseAddress() // LiquidityProvider1
	lp1asset := GetRandomBNBAddress()
	err = addHandler.addLiquidity(ctx, common.BNBAsset, cosmos.NewUint(100*common.One), cosmos.NewUint(10*common.One), lp1, lp1asset, GetRandomTx(), false, consts, 1)
	c.Assert(err, IsNil)
	err = addHandler.addLiquidity(ctx, boltAsset, cosmos.NewUint(50*common.One), cosmos.NewUint(110*common.One), lp1, lp1asset, GetRandomTx(), false, consts, 1)
	c.Assert(err, IsNil)

	lp2 := GetRandomBaseAddress() // liquidity provider 2
	lp2asset := GetRandomBNBAddress()
	err = addHandler.addLiquidity(ctx, common.BNBAsset, cosmos.NewUint(155*common.One), cosmos.NewUint(15*common.One), lp2, lp2asset, GetRandomTx(), false, consts, 1)
	c.Assert(err, IsNil)
	err = addHandler.addLiquidity(ctx, boltAsset, cosmos.NewUint(20*common.One), cosmos.NewUint(40*common.One), lp2, lp2asset, GetRandomTx(), false, consts, 1)
	c.Assert(err, IsNil)

	lp3 := GetRandomBaseAddress() // liquidity provider 3
	lp3asset := GetRandomBNBAddress()
	err = addHandler.addLiquidity(ctx, common.BNBAsset, cosmos.NewUint(155*common.One), cosmos.NewUint(15*common.One), lp3, lp3asset, GetRandomTx(), false, consts, 1)
	c.Assert(err, IsNil)

	lp4 := GetRandomBaseAddress() // liquidity provider 4 , BTC
	lp4Asset := GetRandomBTCAddress()
	err = addHandler.addLiquidity(ctx, common.BTCAsset, cosmos.NewUint(100*common.One), cosmos.NewUint(100*common.One), lp4, lp4Asset, GetRandomTx(), false, consts, 0)
	c.Assert(err, IsNil)

	lp5 := GetRandomBaseAddress() // Rune only
	err = addHandler.addLiquidity(ctx, common.BTCAsset, cosmos.NewUint(100*common.One), cosmos.ZeroUint(), lp5, common.NoAddress, GetRandomTx(), false, consts, 0)
	c.Assert(err, IsNil)

	lp6Asset := GetRandomBTCAddress() // BTC only
	err = addHandler.addLiquidity(ctx, common.BTCAsset, cosmos.ZeroUint(), cosmos.NewUint(100*common.One), common.NoAddress, lp6Asset, GetRandomTx(), false, consts, 0)
	c.Assert(err, IsNil)

	asgard.AddFunds(common.Coins{
		common.NewCoin(common.BTCAsset, cosmos.NewUint(101*common.One)),
	})

	lps := []common.Address{
		lp1, lp2, lp3,
	}
	lpsAssets := []common.Address{
		lp1asset, lp2asset, lp3asset,
	}

	// get new pool data
	bnbPool, err := mgr.Keeper().GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	boltPool, err := mgr.Keeper().GetPool(ctx, boltAsset)
	c.Assert(err, IsNil)

	// Add bonders/validators
	bonderCount := 3
	bonders := make(NodeAccounts, bonderCount)
	for i := 1; i <= bonderCount; i++ {
		na := GetRandomValidatorNode(NodeActive)
		naBond := cosmos.NewUint(1_000_000 * uint64(i) * common.One)
		bp := NewBondProviders(na.NodeAddress)
		lp, _ := SetupLiquidityBondForTest(c, ctx, mgr.Keeper(), common.ETHAsset, na.BondAddress, na, naBond)
		lp.NodeBondAddress = na.NodeAddress
		mgr.Keeper().SetLiquidityProvider(ctx, lp)
		c.Assert(err, IsNil)
		var acc cosmos.AccAddress
		acc, err = na.BondAddress.AccAddress()
		c.Assert(err, IsNil)
		bp.Providers = append(bp.Providers, NewBondProvider(acc))
		bp.Providers[0].Bonded = true
		c.Assert(mgr.Keeper().SetBondProviders(ctx, bp), IsNil)
		c.Assert(mgr.Keeper().SetNodeAccount(ctx, na), IsNil)
		bonders[i-1] = na

		// Add bond to asgard
		asgard.AddFunds(common.Coins{
			common.NewCoin(common.BaseAsset(), naBond),
		})
		c.Assert(mgr.Keeper().SetVault(ctx, asgard), IsNil)

		var ethPool Pool
		ethPool, err = mgr.Keeper().GetPool(ctx, common.ETHAsset)
		c.Assert(err, IsNil)

		// create yggdrasil vault, with 1/3 of the liquidity provider funds
		ygg := GetRandomVault()
		ygg.PubKey = na.PubKeySet.Secp256k1
		ygg.Type = YggdrasilVault
		ygg.AddFunds(common.Coins{
			common.NewCoin(common.BaseAsset(), bnbPool.BalanceCacao.QuoUint64(uint64(bonderCount))),
			common.NewCoin(common.BNBAsset, bnbPool.BalanceAsset.QuoUint64(uint64(bonderCount))),
			common.NewCoin(common.BaseAsset(), boltPool.BalanceCacao.QuoUint64(uint64(bonderCount))),
			common.NewCoin(boltAsset, boltPool.BalanceAsset.QuoUint64(uint64(bonderCount))),
			common.NewCoin(common.ETHAsset, ethPool.BalanceAsset.QuoUint64(uint64(bonderCount))),
		})
		c.Assert(mgr.Keeper().SetVault(ctx, ygg), IsNil)
	}

	// ////////////////////////////////////////////////////////
	// ////////////// Start Ragnarok Protocol /////////////////
	// ////////////////////////////////////////////////////////
	network := Network{
		BondRewardRune: cosmos.NewUint(1000_000 * common.One),
		TotalBondUnits: cosmos.NewUint(3 * 1014), // block height * node count
	}
	FundModule(c, ctx, mgr.Keeper(), ReserveName, cosmos.NewUint(40_010_000_000*common.One).Uint64())
	c.Assert(mgr.Keeper().SetNetwork(ctx, network), IsNil)
	ctx = ctx.WithBlockHeight(1024)

	active, err := mgr.Keeper().ListActiveValidators(ctx)
	c.Assert(err, IsNil)
	// this should trigger stage 1 of the ragnarok protocol. We should see a tx
	// out per node account
	c.Assert(mgr.ValidatorMgr().processRagnarok(ctx, mgr), IsNil)
	// after ragnarok get triggered , we pay bond reward immediately
	for idx, bonder := range bonders {
		var na NodeAccount
		na, err = mgr.Keeper().GetNodeAccount(ctx, bonder.NodeAddress)
		c.Assert(err, IsNil)
		bonders[idx].Reward = na.Reward
	}
	// make sure all yggdrasil vault get recalled
	items, err := mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, bonderCount)
	for _, item := range items {
		c.Assert(item.Coin.Equals(common.NewCoin(common.BaseAsset(), cosmos.ZeroUint())), Equals, true)
	}

	// we'll assume the signer does it's job and sends the yggdrasil funds back
	// to asgard, and do it ourselves here manually
	for _, na := range active {
		ygg, err := mgr.Keeper().GetVault(ctx, na.PubKeySet.Secp256k1)
		c.Assert(err, IsNil)
		asgard.AddFunds(ygg.Coins)
		c.Assert(mgr.Keeper().SetVault(ctx, asgard), IsNil)
		ygg.SubFunds(ygg.Coins)
		c.Assert(mgr.Keeper().SetVault(ctx, ygg), IsNil)
	}
	mgr.TxOutStore().ClearOutboundItems(ctx) // clear out txs

	for i := 1; i <= 11; i++ { // simulate each round of ragnarok (max of ten)
		c.Assert(mgr.ValidatorMgr().processRagnarok(ctx, mgr), IsNil)
		_, err := mgr.TxOutStore().GetOutboundItems(ctx)
		c.Assert(err, IsNil)
		// validate liquidity providers get their returns
		for j, lp := range lpsAssets {
			items = mgr.TxOutStore().GetOutboundItemByToAddress(ctx, lp)
			if i == 1 { // nolint
				if j >= len(lps)-1 {
					c.Assert(items, HasLen, 0, Commentf("%d", len(items)))
				} else {
					c.Assert(items, HasLen, 1, Commentf("%d", len(items)))
				}
			} else if i > 10 {
				c.Assert(items, HasLen, 1, Commentf("%d", len(items)))
			} else {
				c.Assert(items, HasLen, 0)
			}
		}
		mgr.TxOutStore().ClearOutboundItems(ctx) // clear out txs
		mgr.Keeper().SetRagnarokPending(ctx, 0)
		items, err = mgr.TxOutStore().GetOutboundItems(ctx)
		c.Assert(items, HasLen, 0)
		c.Assert(err, IsNil)
	}
}

func (s *ThorchainSuite) TestRagnarokNoOneLeave(c *C) {
	var err error
	ctx, mgr := setupManagerForTest(c)
	ctx = ctx.WithBlockHeight(10)
	ver := GetCurrentVersion()
	consts := constants.GetConstantValues(ver)

	// create active asgard vault
	asgard := GetRandomVault()
	c.Assert(mgr.Keeper().SetVault(ctx, asgard), IsNil)

	// create pools
	pool := NewPool()
	pool.Asset = common.BNBAsset
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)
	boltAsset, err := common.NewAsset("BNB.BOLT-123")
	c.Assert(err, IsNil)
	pool.Asset = boltAsset
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)
	addHandler := NewAddLiquidityHandler(NewDummyMgrWithKeeper(mgr.Keeper()))
	// add liquidity providers
	lp1 := GetRandomBaseAddress() // LiquidityProvider1
	err = addHandler.addLiquidity(ctx, common.BNBAsset, cosmos.NewUint(100*common.One), cosmos.NewUint(10*common.One), lp1, lp1, GetRandomTx(), false, consts, 0)
	c.Assert(err, IsNil)
	err = addHandler.addLiquidity(ctx, boltAsset, cosmos.NewUint(500*common.One), cosmos.NewUint(11*common.One), lp1, lp1, GetRandomTx(), false, consts, 0)
	c.Assert(err, IsNil)
	lp2 := GetRandomBaseAddress() // liquidity provider 2
	err = addHandler.addLiquidity(ctx, common.BNBAsset, cosmos.NewUint(155*common.One), cosmos.NewUint(15*common.One), lp2, lp2, GetRandomTx(), false, consts, 0)
	c.Assert(err, IsNil)
	err = addHandler.addLiquidity(ctx, boltAsset, cosmos.NewUint(200*common.One), cosmos.NewUint(4*common.One), lp2, lp2, GetRandomTx(), false, consts, 0)
	c.Assert(err, IsNil)
	lp3 := GetRandomBaseAddress() // liquidity provider 3
	err = addHandler.addLiquidity(ctx, common.BNBAsset, cosmos.NewUint(155*common.One), cosmos.NewUint(15*common.One), lp3, lp3, GetRandomTx(), false, consts, 0)
	c.Assert(err, IsNil)
	lps := []common.Address{
		lp1, lp2, lp3,
	}
	_ = lps

	// get new pool data
	bnbPool, err := mgr.Keeper().GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	boltPool, err := mgr.Keeper().GetPool(ctx, boltAsset)
	c.Assert(err, IsNil)

	// Add bonders/validators
	bonderCount := 4
	bonders := make(NodeAccounts, bonderCount)
	for i := 1; i <= bonderCount; i++ {
		na := GetRandomValidatorNode(NodeActive)
		na.ActiveBlockHeight = 5
		naBond := cosmos.NewUint(1_000_000 * uint64(i) * common.One)
		bp := NewBondProviders(na.NodeAddress)
		var acc cosmos.AccAddress
		acc, err = na.BondAddress.AccAddress()
		c.Assert(err, IsNil)
		bp.Providers = append(bp.Providers, NewBondProvider(acc))
		bp.Providers[0].Bonded = true
		SetupLiquidityBondForTest(c, ctx, mgr.Keeper(), common.BNBAsset, na.BondAddress, na, naBond)
		c.Assert(mgr.Keeper().SetBondProviders(ctx, bp), IsNil)
		c.Assert(mgr.Keeper().SetNodeAccount(ctx, na), IsNil)
		bonders[i-1] = na

		// Add bond to asgard
		asgard.AddFunds(common.Coins{
			common.NewCoin(common.BaseAsset(), naBond),
		})
		asgard.Membership = append(asgard.Membership, na.PubKeySet.Secp256k1.String())
		c.Assert(mgr.Keeper().SetVault(ctx, asgard), IsNil)

		// create yggdrasil vault, with 1/3 of the liquidity provider funds
		ygg := GetRandomVault()
		ygg.PubKey = na.PubKeySet.Secp256k1
		ygg.Type = YggdrasilVault
		ygg.AddFunds(common.Coins{
			common.NewCoin(common.BaseAsset(), bnbPool.BalanceCacao.QuoUint64(uint64(bonderCount))),
			common.NewCoin(common.BNBAsset, bnbPool.BalanceAsset.QuoUint64(uint64(bonderCount))),
			common.NewCoin(common.BaseAsset(), boltPool.BalanceCacao.QuoUint64(uint64(bonderCount))),
			common.NewCoin(boltAsset, boltPool.BalanceAsset.QuoUint64(uint64(bonderCount))),
		})
		c.Assert(mgr.Keeper().SetVault(ctx, ygg), IsNil)

	}

	// Add reserve contributors
	contrib1 := GetRandomBNBAddress()
	contrib2 := GetRandomBNBAddress()
	reserves := ReserveContributors{
		NewReserveContributor(contrib1, cosmos.NewUint(400_000_000*common.One)),
		NewReserveContributor(contrib2, cosmos.NewUint(100_000*common.One)),
	}
	resHandler := NewReserveContributorHandler(mgr)
	for _, res := range reserves {
		asgard.AddFunds(common.Coins{
			common.NewCoin(common.BaseAsset(), res.Amount),
		})
		msg := NewMsgReserveContributor(GetRandomTx(), res, bonders[0].NodeAddress)
		err = resHandler.handle(ctx, *msg)
		_ = err
		// c.Assert(err, IsNil)
	}
	c.Assert(mgr.Keeper().SetVault(ctx, asgard), IsNil)
	asgard.Membership = asgard.Membership[:len(asgard.Membership)-1]
	c.Assert(mgr.Keeper().SetVault(ctx, asgard), IsNil)
	// no validator should leave, because it trigger ragnarok
	updates := mgr.ValidatorMgr().EndBlock(ctx, mgr)
	c.Assert(updates, IsNil)
	ragnarokHeight, err := mgr.Keeper().GetRagnarokBlockHeight(ctx)
	c.Assert(err, IsNil)
	c.Assert(ragnarokHeight, Equals, ctx.BlockHeight())
	currentHeight := ctx.BlockHeight()
	migrateInterval := consts.GetInt64Value(constants.FundMigrationInterval)
	ctx = ctx.WithBlockHeight(currentHeight + migrateInterval)
	c.Assert(mgr.ValidatorMgr().BeginBlock(ctx, mgr, nil), IsNil)
	mgr.TxOutStore().ClearOutboundItems(ctx)
	c.Assert(mgr.ValidatorMgr().EndBlock(ctx, mgr), IsNil)
}
