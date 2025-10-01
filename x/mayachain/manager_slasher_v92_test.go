package mayachain

import (
	"errors"

	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/x/mayachain/keeper/types"
	types2 "gitlab.com/mayachain/mayanode/x/mayachain/types"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

type SlashingV92Suite struct{}

var _ = Suite(&SlashingV92Suite{})

func (s *SlashingV92Suite) SetUpSuite(_ *C) {
	SetupConfigForTest()
}

func (s *SlashingV92Suite) TestObservingSlashing(c *C) {
	var err error
	ctx, k := setupKeeperForTest(c)
	naActiveAfterTx := GetRandomValidatorNode(NodeActive)
	naActiveAfterTx.ActiveBlockHeight = 1030
	nas := NodeAccounts{
		GetRandomValidatorNode(NodeActive),
		GetRandomValidatorNode(NodeActive),
		GetRandomValidatorNode(NodeStandby),
		naActiveAfterTx,
	}
	for _, item := range nas {
		c.Assert(k.SetNodeAccount(ctx, item), IsNil)
	}
	height := int64(1024)
	txOut := NewTxOut(height)
	txHash := GetRandomTxHash()
	observedTx := GetRandomObservedTx()
	txVoter := NewObservedTxVoter(txHash, []ObservedTx{
		observedTx,
	})
	txVoter.FinalisedHeight = 1024
	txVoter.Add(observedTx, nas[0].NodeAddress)
	txVoter.Tx = txVoter.Txs[0]
	k.SetObservedTxInVoter(ctx, txVoter)

	txOut.TxArray = append(txOut.TxArray, TxOutItem{
		Chain:       common.BNBChain,
		InHash:      txHash,
		ToAddress:   GetRandomBNBAddress(),
		VaultPubKey: GetRandomPubKey(),
		Coin:        common.NewCoin(common.BNBAsset, cosmos.NewUint(1024)),
		Memo:        "whatever",
	})

	c.Assert(k.SetTxOut(ctx, txOut), IsNil)

	ctx = ctx.WithBlockHeight(height + 300)
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)

	slasher := newSlasherV92(k, NewDummyEventMgr())
	// should slash na2 only
	lackOfObservationPenalty := constAccessor.GetInt64Value(constants.LackOfObservationPenalty)
	err = slasher.LackObserving(ctx, constAccessor)
	c.Assert(err, IsNil)
	slashPoint, err := k.GetNodeAccountSlashPoints(ctx, nas[0].NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(slashPoint, Equals, int64(0))

	slashPoint, err = k.GetNodeAccountSlashPoints(ctx, nas[1].NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(slashPoint, Equals, lackOfObservationPenalty)

	// standby node should not be slashed
	slashPoint, err = k.GetNodeAccountSlashPoints(ctx, nas[2].NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(slashPoint, Equals, int64(0))

	// if node is active after the tx get observed , it should not be slashed
	slashPoint, err = k.GetNodeAccountSlashPoints(ctx, nas[3].NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(slashPoint, Equals, int64(0))

	ctx = ctx.WithBlockHeight(height + 301)
	err = slasher.LackObserving(ctx, constAccessor)

	c.Assert(err, IsNil)
	slashPoint, err = k.GetNodeAccountSlashPoints(ctx, nas[0].NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(slashPoint, Equals, int64(0))

	slashPoint, err = k.GetNodeAccountSlashPoints(ctx, nas[1].NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(slashPoint, Equals, lackOfObservationPenalty)
}

func (s *SlashingV92Suite) TestLackObservingErrors(c *C) {
	ctx, _ := setupKeeperForTest(c)

	nas := NodeAccounts{
		GetRandomValidatorNode(NodeActive),
		GetRandomValidatorNode(NodeActive),
	}
	keeper := &TestSlashObservingKeeper{
		nas:      nas,
		addrs:    []cosmos.AccAddress{nas[0].NodeAddress},
		slashPts: make(map[string]int64),
	}
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)
	slasher := newSlasherV92(keeper, NewDummyEventMgr())
	err := slasher.LackObserving(ctx, constAccessor)
	c.Assert(err, IsNil)
}

func (s *SlashingV92Suite) TestNodeSignSlashErrors(c *C) {
	testCases := []struct {
		name        string
		condition   func(keeper *TestSlashingLackKeeper)
		shouldError bool
	}{
		{
			name: "fail to get tx out should return an error",
			condition: func(keeper *TestSlashingLackKeeper) {
				keeper.failGetTxOut = true
			},
			shouldError: true,
		},
		{
			name: "fail to get vault should return an error",
			condition: func(keeper *TestSlashingLackKeeper) {
				keeper.failGetVault = true
			},
			shouldError: false,
		},
		{
			name: "fail to get node account by pub key should return an error",
			condition: func(keeper *TestSlashingLackKeeper) {
				keeper.failGetNodeAccountByPubKey = true
			},
			shouldError: false,
		},
		{
			name: "fail to get asgard vault by status should return an error",
			condition: func(keeper *TestSlashingLackKeeper) {
				keeper.failGetAsgardByStatus = true
			},
			shouldError: true,
		},
		{
			name: "fail to get observed tx voter should return an error",
			condition: func(keeper *TestSlashingLackKeeper) {
				keeper.failGetObservedTxVoter = true
			},
			shouldError: true,
		},
		{
			name: "fail to set tx out should return an error",
			condition: func(keeper *TestSlashingLackKeeper) {
				keeper.failSetTxOut = true
			},
			shouldError: true,
		},
	}
	for _, item := range testCases {
		c.Logf("name:%s", item.name)
		ctx, _ := setupKeeperForTest(c)
		ctx = ctx.WithBlockHeight(201) // set blockheight
		ver := GetCurrentVersion()
		constAccessor := constants.GetConstantValues(ver)
		na := GetRandomValidatorNode(NodeActive)
		inTx := common.NewTx(
			GetRandomTxHash(),
			GetRandomBNBAddress(),
			GetRandomBNBAddress(),
			common.Coins{
				common.NewCoin(common.BNBAsset, cosmos.NewUint(320000000)),
				common.NewCoin(common.BaseAsset(), cosmos.NewUint(420000000)),
			},
			nil,
			"SWAP:BNB.BNB",
		)

		txOutItem := TxOutItem{
			Chain:       common.BNBChain,
			InHash:      inTx.ID,
			VaultPubKey: na.PubKeySet.Secp256k1,
			ToAddress:   GetRandomBNBAddress(),
			Coin: common.NewCoin(
				common.BNBAsset, cosmos.NewUint(3980500*common.One),
			),
		}
		txOut := NewTxOut(3)
		txOut.TxArray = append(txOut.TxArray, txOutItem)

		ygg := GetRandomVault()
		ygg.Type = YggdrasilVault
		keeper := &TestSlashingLackKeeper{
			txOut:  txOut,
			na:     na,
			vaults: Vaults{ygg},
			voter: ObservedTxVoter{
				Actions: []TxOutItem{txOutItem},
			},
			slashPts: make(map[string]int64),
		}
		signingTransactionPeriod := constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
		ctx = ctx.WithBlockHeight(3 + signingTransactionPeriod)
		slasher := newSlasherV92(keeper, NewDummyEventMgr())
		item.condition(keeper)
		if item.shouldError {
			c.Assert(slasher.LackSigning(ctx, NewDummyMgr()), NotNil)
		} else {
			c.Assert(slasher.LackSigning(ctx, NewDummyMgr()), IsNil)
		}
	}
}

func (s *SlashingV92Suite) TestNotSigningSlash(c *C) {
	ctx, _ := setupKeeperForTest(c)
	ctx = ctx.WithBlockHeight(201) // set blockheight
	txOutStore := NewTxStoreDummy()
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)
	na := GetRandomValidatorNode(NodeActive)
	inTx := common.NewTx(
		GetRandomTxHash(),
		GetRandomBNBAddress(),
		GetRandomBNBAddress(),
		common.Coins{
			common.NewCoin(common.BNBAsset, cosmos.NewUint(320000000)),
			common.NewCoin(common.BaseAsset(), cosmos.NewUint(420000000)),
		},
		nil,
		"SWAP:BNB.BNB",
	)

	txOutItem := TxOutItem{
		Chain:       common.BNBChain,
		InHash:      inTx.ID,
		VaultPubKey: na.PubKeySet.Secp256k1,
		ToAddress:   GetRandomBNBAddress(),
		Coin: common.NewCoin(
			common.BNBAsset, cosmos.NewUint(3980500*common.One),
		),
	}
	txOut := NewTxOut(3)
	txOut.TxArray = append(txOut.TxArray, txOutItem)

	ygg := GetRandomVault()
	ygg.Type = YggdrasilVault
	ygg.Coins = common.Coins{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(5000000*common.One)),
	}
	keeper := &TestSlashingLackKeeper{
		txOut:  txOut,
		na:     na,
		vaults: Vaults{ygg},
		voter: ObservedTxVoter{
			Actions: []TxOutItem{txOutItem},
		},
		slashPts: make(map[string]int64),
	}
	signingTransactionPeriod := constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
	ctx = ctx.WithBlockHeight(3 + signingTransactionPeriod)
	mgr := NewDummyMgr()
	mgr.txOutStore = txOutStore
	slasher := newSlasherV92(keeper, NewDummyEventMgr())
	c.Assert(slasher.LackSigning(ctx, mgr), IsNil)

	c.Check(keeper.slashPts[na.NodeAddress.String()], Equals, int64(600), Commentf("%+v\n", na))

	outItems, err := txOutStore.GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(outItems, HasLen, 1)
	c.Assert(outItems[0].VaultPubKey.Equals(keeper.vaults[0].PubKey), Equals, true)
	c.Assert(outItems[0].Memo, Equals, "")
	c.Assert(keeper.voter.Actions, HasLen, 1)
	// ensure we've updated our action item
	c.Assert(keeper.voter.Actions[0].VaultPubKey.Equals(outItems[0].VaultPubKey), Equals, true)
	c.Assert(keeper.txOut.TxArray[0].OutHash.IsEmpty(), Equals, false)
}

func (s *SlashingV92Suite) TestNewSlasher(c *C) {
	nas := NodeAccounts{
		GetRandomValidatorNode(NodeActive),
		GetRandomValidatorNode(NodeActive),
	}
	keeper := &TestSlashObservingKeeper{
		nas:      nas,
		addrs:    []cosmos.AccAddress{nas[0].NodeAddress},
		slashPts: make(map[string]int64),
	}
	slasher := newSlasherV92(keeper, NewDummyEventMgr())
	c.Assert(slasher, NotNil)
}

func (s *SlashingV92Suite) TestDoubleSign(c *C) {
	ctx, mgr := setupManagerForTest(c)
	constAccessor := constants.GetConstantValues(GetCurrentVersion())

	na := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, na), IsNil)
	naBond := cosmos.NewUint(1000000 * common.One)
	SetupLiquidityBondForTest(c, ctx, mgr.Keeper(), common.BTCAsset, na.BondAddress, na, naBond)
	acc, err := na.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp := NewBondProviders(na.NodeAddress)
	bp.Providers = append(bp.Providers, BondProvider{
		BondAddress: acc,
		Bonded:      true,
	})
	c.Assert(mgr.Keeper().SetBondProviders(ctx, bp), IsNil)
	prevNodeBond, err := mgr.Keeper().CalcNodeLiquidityBond(ctx, na)
	c.Assert(err, IsNil)
	c.Assert(prevNodeBond.Equal(naBond.MulUint64(2)), Equals, true, Commentf("%d", prevNodeBond))

	slasher := newSlasherV92(mgr.Keeper(), mgr.EventMgr())

	pk, err := cosmos.GetPubKeyFromBech32(cosmos.Bech32PubKeyTypeConsPub, na.ValidatorConsPubKey)
	c.Assert(err, IsNil)
	err = slasher.HandleDoubleSign(ctx, pk.Address(), 0, constAccessor)
	c.Assert(err, IsNil)

	updatedNode, err := mgr.Keeper().GetNodeAccountByPubKey(ctx, na.PubKeySet.Secp256k1)
	c.Assert(err, IsNil)
	calcNodeBond, err := mgr.Keeper().CalcNodeLiquidityBond(ctx, updatedNode)
	c.Assert(err, IsNil)
	c.Assert(calcNodeBond.LT(prevNodeBond), Equals, true, Commentf("%d", calcNodeBond))
}

func (s *SlashingV92Suite) TestIncreaseDecreaseSlashPoints(c *C) {
	ctx, _ := setupKeeperForTest(c)

	na := GetRandomValidatorNode(NodeActive)
	naBond := cosmos.NewUint(100 * common.One)
	bp := NewBondProviders(na.NodeAddress)
	acc, err := na.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp.Providers = append(bp.Providers, NewBondProvider(acc))
	bp.Providers[0].Bonded = true

	keeper := &TestDoubleSlashKeeper{
		na:     na,
		naBond: naBond,
		bp:     bp,
		lp: LiquidityProvider{
			Asset:        common.BNBAsset,
			Units:        naBond,
			CacaoAddress: common.Address(na.BondAddress.String()),
			AssetAddress: GetRandomBNBAddress(),
		},
		network:     NewNetwork(),
		slashPoints: make(map[string]int64),
	}
	slasher := newSlasherV92(keeper, NewDummyEventMgr())
	addr := GetRandomBech32Addr()
	slasher.IncSlashPoints(ctx, 1, addr)
	slasher.DecSlashPoints(ctx, 1, addr)
	c.Assert(keeper.slashPoints[addr.String()], Equals, int64(0))
}

func (s *SlashingV92Suite) TestSlashVault(c *C) {
	ctx, mgr := setupManagerForTest(c)
	slasher := newSlasherV92(mgr.Keeper(), mgr.EventMgr())
	// when coins are empty , it should return nil
	c.Assert(slasher.SlashVaultToLP(ctx, GetRandomPubKey(), common.NewCoins(), mgr, true), IsNil)

	// when vault is not available , it should return an error
	err := slasher.SlashVaultToLP(ctx, GetRandomPubKey(), common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(common.One))), mgr, true)
	c.Assert(err, NotNil)
	c.Assert(errors.Is(err, types.ErrVaultNotFound), Equals, true)

	// create a node
	node := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, node), IsNil)
	nodeBond := cosmos.NewUint(100_000 * common.One)
	SetupLiquidityBondForTest(c, ctx, mgr.Keeper(), common.BNBAsset, node.BondAddress, node, nodeBond)
	acc, err := node.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp := NewBondProviders(node.NodeAddress)
	bp.Providers = append(bp.Providers, BondProvider{
		BondAddress: acc,
		Bonded:      true,
	})
	c.Assert(mgr.Keeper().SetBondProviders(ctx, bp), IsNil)

	vault := GetRandomVault()
	vault.Type = YggdrasilVault
	vault.Status = types2.VaultStatus_ActiveVault
	vault.PubKey = node.PubKeySet.Secp256k1
	vault.Membership = []string{
		node.PubKeySet.Secp256k1.String(),
	}
	vault.Coins = common.NewCoins(
		common.NewCoin(common.BTCAsset, cosmos.NewUint(2000*common.One)),
	)
	c.Assert(mgr.Keeper().SetVault(ctx, vault), IsNil)

	// setup btc pool
	btcPool := NewPool()
	btcPool.Asset = common.BTCAsset
	btcPool.BalanceCacao = cosmos.NewUint(1000 * common.One)
	btcPool.BalanceAsset = cosmos.NewUint(1000 * common.One)
	btcPool.LPUnits = cosmos.NewUint(1000 * common.One)
	c.Assert(mgr.Keeper().SetPool(ctx, btcPool), IsNil)

	stolen := common.NewCoin(common.BTCAsset, cosmos.NewUint(1000*common.One))
	err = slasher.SlashVaultToLP(ctx, vault.PubKey, common.NewCoins(stolen), mgr, true)
	c.Assert(err, IsNil)
	calcNodeBond, err := mgr.Keeper().CalcNodeLiquidityBond(ctx, node)
	c.Assert(err, IsNil)

	slash := stolen.Amount.MulUint64(3).QuoUint64(2)
	expectedBond := nodeBond.MulUint64(2).Sub(slash)
	c.Assert(expectedBond.Uint64(), Equals, calcNodeBond.Uint64(), Commentf("expected %d, got %d", expectedBond.Uint64(), calcNodeBond.Uint64()))

	// Test without pol withdraw (asgard not setup so no toi)
	polAddress, err := mgr.Keeper().GetModuleAddress(ReserveName)
	c.Assert(err, IsNil)
	polLP, err := mgr.Keeper().CalcTotalBondableLiquidity(ctx, polAddress)
	c.Assert(err, IsNil)
	c.Assert(polLP.Uint64(), Equals, slash.Uint64(), Commentf("expected %d, got %d", slash.Sub(stolen.Amount).Uint64(), polLP.Uint64()))

	// add one more node , slash asgard
	node1 := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, node1), IsNil)
	node1Bond := cosmos.NewUint(100_000 * common.One)
	SetupLiquidityBondForTest(c, ctx, mgr.Keeper(), common.BNBAsset, node1.BondAddress, node1, node1Bond)
	acc, err = node1.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp = NewBondProviders(node1.NodeAddress)
	bp.Providers = append(bp.Providers, BondProvider{
		BondAddress: acc,
		Bonded:      true,
	})
	c.Assert(mgr.Keeper().SetBondProviders(ctx, bp), IsNil)

	// Reset btc pool
	btcPool.BalanceCacao = cosmos.NewUint(1000 * common.One)
	btcPool.BalanceAsset = cosmos.NewUint(1000 * common.One)
	btcPool.LPUnits = cosmos.NewUint(1000 * common.One)
	c.Assert(mgr.Keeper().SetPool(ctx, btcPool), IsNil)

	// Setup vault.
	vault1 := GetRandomVault()
	vault1.Type = AsgardVault
	vault1.Status = types2.VaultStatus_ActiveVault
	vault1.PubKey = GetRandomPubKey()
	vault1.Membership = []string{
		node.PubKeySet.Secp256k1.String(),
		node1.PubKeySet.Secp256k1.String(),
	}
	vault1.Coins = common.NewCoins(
		common.NewCoin(common.BTCAsset, cosmos.NewUint(2000*common.One)),
	)
	c.Assert(mgr.Keeper().SetVault(ctx, vault1), IsNil)

	mgr.Keeper().SetMimir(ctx, "PauseOnSlashThreshold", 1)

	// Slash action.
	err = slasher.SlashVaultToLP(ctx, vault1.PubKey, common.NewCoins(stolen), mgr, true)
	c.Assert(err, IsNil)

	nodeBondAfterSlash, err := mgr.Keeper().CalcNodeLiquidityBond(ctx, node)
	c.Assert(err, IsNil)
	node1BondAfterSlash, err := mgr.Keeper().CalcNodeLiquidityBond(ctx, node1)
	c.Assert(err, IsNil)

	// approx. 3000 * common.One from first and this second slash
	c.Assert(nodeBondAfterSlash.Uint64(), Equals, uint64(19775282308656), Commentf("expected %d, got %d", 19775282308656, nodeBondAfterSlash.Uint64()))
	c.Assert(node1BondAfterSlash.Uint64(), Equals, uint64(19924717691342), Commentf("expected %d, got %d", 19924717691342, node1BondAfterSlash.Uint64()))

	slashed := cosmos.NewUint(400_000 * common.One).Sub(nodeBondAfterSlash).Sub(node1BondAfterSlash)
	// Test without pol withdraw (asgard not setup so no toi)
	polLP, err = mgr.Keeper().CalcTotalBondableLiquidity(ctx, polAddress)
	c.Assert(err, IsNil)
	c.Assert(polLP.Uint64(), Equals, slashed.Uint64(), Commentf("expected %d, got %d", slashed.Uint64(), polLP.Uint64()))

	val, err := mgr.Keeper().GetMimir(ctx, mimirStopFundYggdrasil)
	c.Assert(err, IsNil)
	c.Assert(val, Equals, int64(18), Commentf("%d", val))

	val, err = mgr.Keeper().GetMimir(ctx, "HaltBTCChain")
	c.Assert(err, IsNil)
	c.Assert(val, Equals, int64(18), Commentf("%d", val))
}

func (s *SlashingV92Suite) TestSlashNodeAccountLP(c *C) {
	ctx, mgr := setupManagerForTest(c)
	keeper := &TestSlashNodeAccountLPKeeper{
		Keeper: mgr.Keeper(),
	}

	slasher := newSlasherV92(keeper, mgr.EventMgr())

	// error on calc bond should return error
	na := GetRandomValidatorNode(NodeActive)
	keeper.failCalcBond = true
	amt, poolAmts, err := slasher.SlashNodeAccountLP(ctx, na, cosmos.NewUint(1))
	c.Assert(err, NotNil)
	c.Assert(amt.IsZero(), Equals, true)
	c.Assert(poolAmts, IsNil)
	keeper.failCalcBond = false

	// error on get pol address should return error
	keeper.failGetPolAddr = true
	amt, poolAmts, err = slasher.SlashNodeAccountLP(ctx, na, cosmos.NewUint(1))
	c.Assert(err, NotNil)
	c.Assert(amt.IsZero(), Equals, true)
	c.Assert(poolAmts, IsNil)
	keeper.failGetPolAddr = false

	// node without bond should return an error
	keeper.zeroBond = true
	amt, poolAmts, err = slasher.SlashNodeAccountLP(ctx, na, cosmos.NewUint(1))
	c.Assert(err, IsNil)
	c.Assert(amt.IsZero(), Equals, true)
	c.Assert(poolAmts, IsNil)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, na), IsNil)
	c.Assert(amt.IsZero(), Equals, true)
	keeper.zeroBond = false

	// initialize btc pool
	btcPool := NewPool()
	btcPool.Asset = common.BTCAsset
	btcPool.LPUnits = cosmos.NewUint(90 * common.One)
	btcPool.BalanceCacao = cosmos.NewUint(90 * common.One)
	btcPool.BalanceAsset = cosmos.NewUint(90 * common.One)
	c.Assert(keeper.Keeper.SetPool(ctx, btcPool), IsNil)

	// slash is greater than bond should slash all bond
	nodeBond := cosmos.NewUint(10 * common.One)
	bp := NewBondProviders(na.NodeAddress)
	acc, err := na.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp.Providers = append(bp.Providers, BondProvider{
		BondAddress: acc,
		Bonded:      true,
	})
	c.Assert(keeper.Keeper.SetBondProviders(ctx, bp), IsNil)
	c.Assert(keeper.Keeper.SetNodeAccount(ctx, na), IsNil)
	SetupLiquidityBondForTest(c, ctx, keeper.Keeper, common.BTCAsset, na.BondAddress, na, nodeBond)
	amt, poolAmts, err = slasher.SlashNodeAccountLP(ctx, na, cosmos.NewUint(101*common.One))
	c.Assert(err, IsNil)
	c.Assert(amt.Uint64(), Equals, uint64(20*common.One))
	c.Assert(len(poolAmts), Equals, 1)
	c.Assert(poolAmts[0].Amount, Equals, int64(20*common.One))
	nodeBond, err = keeper.CalcNodeLiquidityBond(ctx, na)
	c.Log("node bond", nodeBond.Uint64())
	c.Assert(err, IsNil)
	c.Assert(nodeBond.IsZero(), Equals, true)

	// fail to get liquidity provider by assets should continue
	keeper.failGetLiquidityProviderByAssets = true
	btcPool.Asset = common.BTCAsset
	btcPool.LPUnits = cosmos.NewUint(90 * common.One)
	btcPool.BalanceCacao = cosmos.NewUint(90 * common.One)
	btcPool.BalanceAsset = cosmos.NewUint(90 * common.One)
	nodeBond = cosmos.NewUint(10 * common.One)
	c.Assert(keeper.Keeper.SetPool(ctx, btcPool), IsNil)
	SetupLiquidityBondForTest(c, ctx, keeper.Keeper, common.BTCAsset, na.BondAddress, na, nodeBond)
	amt, poolAmts, err = slasher.SlashNodeAccountLP(ctx, na, cosmos.NewUint(3*common.One))
	c.Assert(err, NotNil)
	c.Assert(amt.IsZero(), Equals, true)
	c.Assert(poolAmts, IsNil)
	nodeBond, err = keeper.CalcNodeLiquidityBond(ctx, na)
	c.Assert(err, IsNil)
	c.Assert(nodeBond.Uint64(), Equals, uint64(20*common.One), Commentf("expected %d, got %d", 10*common.One, nodeBond.Uint64()))
	keeper.failGetLiquidityProviderByAssets = false

	// fail to get pool should continue to next asset
	keeper.failGetPool = true
	amt, poolAmts, err = slasher.SlashNodeAccountLP(ctx, na, cosmos.NewUint(3*common.One))
	c.Assert(err, IsNil)
	c.Assert(amt.IsZero(), Equals, true)
	c.Assert(poolAmts, IsNil)
	nodeBond, err = keeper.CalcNodeLiquidityBond(ctx, na)
	c.Assert(err, IsNil)
	c.Assert(nodeBond.Uint64(), Equals, uint64(20*common.One))
	keeper.failGetPool = false

	// fail to get LP should skip that asset
	keeper.failGetLP = true
	amt, poolAmts, err = slasher.SlashNodeAccountLP(ctx, na, cosmos.NewUint(3*common.One))
	c.Assert(err, IsNil)
	c.Assert(amt.IsZero(), Equals, true)
	c.Assert(poolAmts, IsNil)
	nodeBond, err = keeper.CalcNodeLiquidityBond(ctx, na)
	c.Assert(err, IsNil)
	c.Assert(nodeBond.Uint64(), Equals, uint64(20*common.One))
	keeper.failGetLP = false

	// happy path
	amt, poolAmts, err = slasher.SlashNodeAccountLP(ctx, na, cosmos.NewUint(3*common.One))
	c.Assert(err, IsNil)
	c.Assert(amt.Uint64(), Equals, uint64(3*common.One))
	c.Assert(len(poolAmts), Equals, 1)
	c.Assert(poolAmts[0].Amount, Equals, int64(3*common.One))
	lp, err := keeper.Keeper.GetLiquidityProvider(ctx, common.BTCAsset, na.BondAddress)
	c.Assert(err, IsNil)
	c.Assert(lp.Units.Uint64(), Equals, uint64(8_50000000), Commentf("expected %d, got %d", 7*common.One, lp.Units.Uint64()))
}

func (s *SlashingV92Suite) TestNetworkShouldNotSlashMorethanVaultAmount(c *C) {
	ctx, mgr := setupManagerForTest(c)
	slasher := newSlasherV92(mgr.Keeper(), mgr.EventMgr())

	// create a node
	node := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, node), IsNil)
	nodeBond := cosmos.NewUint(1000000 * common.One)
	SetupLiquidityBondForTest(c, ctx, mgr.Keeper(), common.BNBAsset, node.BondAddress, node, nodeBond)
	acc, err := node.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp := NewBondProviders(node.NodeAddress)
	bp.Providers = append(bp.Providers, BondProvider{
		BondAddress: acc,
		Bonded:      true,
	})
	c.Assert(mgr.Keeper().SetBondProviders(ctx, bp), IsNil)

	vault := GetRandomVault()
	vault.Type = YggdrasilVault
	vault.Status = types2.VaultStatus_ActiveVault
	vault.PubKey = node.PubKeySet.Secp256k1
	vault.Membership = []string{
		node.PubKeySet.Secp256k1.String(),
	}
	vault.Coins = common.NewCoins(
		common.NewCoin(common.BTCAsset, cosmos.NewUint(1000*common.One/2)),
	)
	c.Assert(mgr.Keeper().SetVault(ctx, vault), IsNil)

	// setup btc pool
	btcPool := NewPool()
	btcPool.Asset = common.BTCAsset
	btcPool.BalanceCacao = cosmos.NewUint(1000 * common.One)
	btcPool.BalanceAsset = cosmos.NewUint(1000 * common.One)
	btcPool.LPUnits = cosmos.NewUint(1000 * common.One)
	c.Assert(mgr.Keeper().SetPool(ctx, btcPool), IsNil)

	// vault only has 0.5 BTC , however the outbound is 1 BTC , make sure we don't over slash the vault
	err = slasher.SlashVaultToLP(ctx, vault.PubKey, common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(1000*common.One))), mgr, true)
	c.Assert(err, IsNil)
	nodeTemp, err := mgr.Keeper().GetNodeAccountByPubKey(ctx, vault.PubKey)
	c.Assert(err, IsNil)
	calcNodeBond, err := mgr.Keeper().CalcNodeLiquidityBond(ctx, nodeTemp)
	c.Assert(err, IsNil)
	expectedBond := cosmos.NewUint(1999250 * common.One)
	c.Assert(calcNodeBond.Uint64(), Equals, expectedBond.Uint64(), Commentf("expected %d, got %d", expectedBond.Uint64(), calcNodeBond.Uint64()))

	// add one more node , slash asgard
	node1 := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, node1), IsNil)
	node1Bond := cosmos.NewUint(1_000_000 * common.One)
	SetupLiquidityBondForTest(c, ctx, mgr.Keeper(), common.BTCAsset, node1.BondAddress, node1, node1Bond)
	acc, err = node1.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp = NewBondProviders(node1.NodeAddress)
	bp.Providers = append(bp.Providers, BondProvider{
		BondAddress: acc,
		Bonded:      true,
	})
	c.Assert(mgr.Keeper().SetBondProviders(ctx, bp), IsNil)

	vault1 := GetRandomVault()
	vault1.Type = AsgardVault
	vault1.Status = types2.VaultStatus_ActiveVault
	vault1.PubKey = GetRandomPubKey()
	vault1.Membership = []string{
		node.PubKeySet.Secp256k1.String(),
		node1.PubKeySet.Secp256k1.String(),
	}
	vault1.Coins = common.NewCoins(
		common.NewCoin(common.BTCAsset, cosmos.NewUint(common.One/2)),
	)
	c.Assert(mgr.Keeper().SetVault(ctx, vault1), IsNil)

	nodeBeforeSlash, err := mgr.Keeper().GetNodeAccount(ctx, node.NodeAddress)
	c.Assert(err, IsNil)
	nodeBondBeforeSlash, err := mgr.Keeper().CalcNodeLiquidityBond(ctx, nodeBeforeSlash)
	c.Assert(err, IsNil)
	node1BondBeforeSlash, err := mgr.Keeper().CalcNodeLiquidityBond(ctx, node1)
	c.Assert(err, IsNil)
	mgr.Keeper().SetMimir(ctx, "PauseOnSlashThreshold", 1)

	// reset btc pool
	btcPool.Asset = common.BTCAsset
	btcPool.BalanceCacao = cosmos.NewUint(1000 * common.One)
	btcPool.BalanceAsset = cosmos.NewUint(1000 * common.One)
	btcPool.LPUnits = cosmos.NewUint(1000 * common.One)
	c.Assert(mgr.Keeper().SetPool(ctx, btcPool), IsNil)

	// Slash action.
	err = slasher.SlashVaultToLP(ctx, vault1.PubKey, common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(common.One))), mgr, true)
	c.Assert(err, IsNil)

	nodeBondAfterSlash, err := mgr.Keeper().CalcNodeLiquidityBond(ctx, node)
	c.Assert(err, IsNil)
	node1BondAfterSlash, err := mgr.Keeper().CalcNodeLiquidityBond(ctx, node1)
	c.Assert(err, IsNil)

	c.Check(nodeBondBeforeSlash.GT(nodeBondAfterSlash), Equals, true, Commentf("Difference of %d", nodeBondBeforeSlash.Sub(nodeBondAfterSlash).Uint64()))
	c.Check(node1BondBeforeSlash.GT(node1BondAfterSlash), Equals, true, Commentf("Difference of %d", node1BondBeforeSlash.Sub(node1BondAfterSlash).Uint64()))

	val, err := mgr.Keeper().GetMimir(ctx, mimirStopFundYggdrasil)
	c.Assert(err, IsNil)
	c.Assert(val, Equals, int64(18), Commentf("%d", val))

	val, err = mgr.Keeper().GetMimir(ctx, "HaltBTCChain")
	c.Assert(err, IsNil)
	c.Assert(val, Equals, int64(18), Commentf("%d", val))

	node2 := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, node2), IsNil)
	SetupLiquidityBondForTest(c, ctx, mgr.Keeper(), common.BTCAsset, node.BondAddress, node, cosmos.NewUint(1000*common.One))

	vault = GetRandomYggVault()
	vault.Status = types2.VaultStatus_ActiveVault
	vault.PubKey = node.PubKeySet.Secp256k1
	vault.Membership = []string{
		node2.PubKeySet.Secp256k1.String(),
	}
	vault.Coins = common.NewCoins(
		common.NewCoin(common.BTCAsset, cosmos.NewUint(4000*common.One)),
	)
	c.Assert(mgr.Keeper().SetVault(ctx, vault), IsNil)

	err = slasher.SlashVaultToLP(ctx, vault.PubKey, common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(2000*common.One))), mgr, true)
	c.Assert(err, IsNil)
}

func (s *SlashingV92Suite) TestNeedsNewVault(c *C) {
	ctx, mgr := setupManagerForTest(c)

	inhash := GetRandomTxHash()
	outhash := GetRandomTxHash()
	sig1 := GetRandomBech32Addr()
	sig2 := GetRandomBech32Addr()
	sig3 := GetRandomBech32Addr()
	pk := GetRandomPubKey()
	tx := GetRandomTx()
	tx.ID = outhash
	obs := NewObservedTx(tx, 0, pk, 0)
	obs.ObservedPubKey = pk
	obs.Signers = []string{sig1.String(), sig2.String(), sig3.String()}

	voter := NewObservedTxVoter(outhash, []ObservedTx{obs})
	mgr.Keeper().SetObservedTxOutVoter(ctx, voter)

	mgr.Keeper().SetObservedLink(ctx, inhash, outhash)
	slasher := newSlasherV92(mgr.Keeper(), mgr.EventMgr())

	c.Check(slasher.needsNewVault(ctx, mgr, 10, 300, 1, inhash, pk), Equals, false)
	ctx = ctx.WithBlockHeight(600)
	c.Check(slasher.needsNewVault(ctx, mgr, 10, 300, 1, inhash, pk), Equals, false)
	ctx = ctx.WithBlockHeight(900)
	c.Check(slasher.needsNewVault(ctx, mgr, 10, 300, 1, inhash, pk), Equals, false)
	ctx = ctx.WithBlockHeight(1600)
	c.Check(slasher.needsNewVault(ctx, mgr, 10, 300, 1, inhash, pk), Equals, true)

	// test that more than 1/3rd will always return false
	ctx = ctx.WithBlockHeight(999999999)
	c.Check(slasher.needsNewVault(ctx, mgr, 9, 300, 1, inhash, pk), Equals, false)
}
