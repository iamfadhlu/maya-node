package mayachain

import (
	"errors"

	"github.com/blang/semver"
	se "github.com/cosmos/cosmos-sdk/types/errors"

	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
)

type HandlerRefundSuite struct{}

var _ = Suite(&HandlerRefundSuite{})

type refundTxHandlerTestHelper struct {
	ctx           cosmos.Context
	pool          Pool
	version       semver.Version
	keeper        *refundTxHandlerKeeperTestHelper
	asgardVault   Vault
	yggVault      Vault
	constAccessor constants.ConstantValues
	nodeAccount   NodeAccount
	inboundTx     ObservedTx
	toi           TxOutItem
	mgr           Manager
}

type refundTxHandlerKeeperTestHelper struct {
	keeper.Keeper
	observeTxVoterErrHash common.TxID
	errGetTxOut           bool
	errGetNodeAccount     bool
	errGetPool            bool
	errSetPool            bool
	errSetNodeAccount     bool
	errGetNetwork         bool
	errSetNetwork         bool
	vault                 Vault
}

func newRefundTxHandlerKeeperTestHelper(keeper keeper.Keeper) *refundTxHandlerKeeperTestHelper {
	return &refundTxHandlerKeeperTestHelper{
		Keeper:                keeper,
		observeTxVoterErrHash: GetRandomTxHash(),
	}
}

func (k *refundTxHandlerKeeperTestHelper) GetObservedTxInVoter(ctx cosmos.Context, hash common.TxID) (ObservedTxVoter, error) {
	if hash.Equals(k.observeTxVoterErrHash) {
		return ObservedTxVoter{}, errKaboom
	}
	return k.Keeper.GetObservedTxOutVoter(ctx, hash)
}

func (k *refundTxHandlerKeeperTestHelper) GetTxOut(ctx cosmos.Context, height int64) (*TxOut, error) {
	if k.errGetTxOut {
		return nil, errKaboom
	}
	return k.Keeper.GetTxOut(ctx, height)
}

func (k *refundTxHandlerKeeperTestHelper) GetNodeAccountByPubKey(ctx cosmos.Context, pk common.PubKey) (NodeAccount, error) {
	if k.errGetNodeAccount {
		return NodeAccount{}, errKaboom
	}
	return k.Keeper.GetNodeAccountByPubKey(ctx, pk)
}

func (k *refundTxHandlerKeeperTestHelper) GetPool(ctx cosmos.Context, asset common.Asset) (Pool, error) {
	if k.errGetPool {
		return NewPool(), errKaboom
	}
	return k.Keeper.GetPool(ctx, asset)
}

func (k *refundTxHandlerKeeperTestHelper) SetPool(ctx cosmos.Context, pool Pool) error {
	if k.errSetPool {
		return errKaboom
	}
	return k.Keeper.SetPool(ctx, pool)
}

func (k *refundTxHandlerKeeperTestHelper) SetNodeAccount(ctx cosmos.Context, na NodeAccount) error {
	if k.errSetNodeAccount {
		return errKaboom
	}
	return k.Keeper.SetNodeAccount(ctx, na)
}

func (k *refundTxHandlerKeeperTestHelper) GetVault(_ cosmos.Context, _ common.PubKey) (Vault, error) {
	return k.vault, nil
}

func (k *refundTxHandlerKeeperTestHelper) SetVault(_ cosmos.Context, v Vault) error {
	k.vault = v
	return nil
}

func (k *refundTxHandlerKeeperTestHelper) GetNetwork(ctx cosmos.Context) (Network, error) {
	if k.errGetNetwork {
		return Network{}, errKaboom
	}
	return k.Keeper.GetNetwork(ctx)
}

func (k *refundTxHandlerKeeperTestHelper) SetNetwork(ctx cosmos.Context, data Network) error {
	if k.errSetNetwork {
		return errKaboom
	}
	return k.Keeper.SetNetwork(ctx, data)
}

// newRefundTxHandlerTestHelper setup all the basic condition to test OutboundTxHandler
func newRefundTxHandlerTestHelper(c *C) refundTxHandlerTestHelper {
	ctx, k := setupKeeperForTest(c)
	ctx = ctx.WithBlockHeight(1023)
	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.BalanceCacao = cosmos.NewUint(100 * common.One)
	pool.LPUnits = pool.BalanceCacao
	pool.Status = PoolAvailable

	version := GetCurrentVersion()
	asgardVault := GetRandomVault()
	asgardVault.Membership = []string{asgardVault.PubKey.String()}
	asgardVault.Coins = common.NewCoins(
		common.NewCoin(common.BNBAsset, cosmos.NewUint(200*common.One)),
		common.NewCoin(common.BTCAsset, cosmos.NewUint(200*common.One)),
	)
	addr, err := asgardVault.PubKey.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)

	yggVault := GetRandomVault()
	yggVault.Membership = []string{yggVault.PubKey.String()}
	yggVault.Coins = common.NewCoins(
		common.NewCoin(common.BNBAsset, cosmos.NewUint(200*common.One)),
		common.NewCoin(common.BTCAsset, cosmos.NewUint(200*common.One)),
	)

	tx := NewObservedTx(common.Tx{
		ID:          GetRandomTxHash(),
		Chain:       common.BNBChain,
		Coins:       common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(2*common.One))},
		Memo:        "swap:RUNE-67C",
		FromAddress: GetRandomBNBAddress(),
		ToAddress:   addr,
		Gas:         BNBGasFeeSingleton,
	}, 12, GetRandomPubKey(), 12)

	keeperTestHelper := newRefundTxHandlerKeeperTestHelper(k)
	keeperTestHelper.vault = yggVault

	mgr := NewDummyMgrWithKeeper(keeperTestHelper)
	mgr.slasher = newSlasherVCUR(keeperTestHelper, NewDummyEventMgr())
	c.Assert(keeperTestHelper.SetPool(ctx, pool), IsNil)

	nodeAccount := GetRandomValidatorNode(NodeActive)
	nodeAccount.NodeAddress, err = yggVault.PubKey.GetThorAddress()
	c.Assert(err, IsNil)
	SetupLiquidityBondForTest(c, ctx, k, common.BTCAsset, nodeAccount.BondAddress, nodeAccount, cosmos.NewUint(100*common.One))
	bp := NewBondProviders(nodeAccount.NodeAddress)
	acc, err := nodeAccount.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp.Providers = append(bp.Providers, NewBondProvider(acc))
	bp.Providers[0].Bonded = true
	nodeAccount.PubKeySet = common.NewPubKeySet(yggVault.PubKey, yggVault.PubKey)
	c.Assert(keeperTestHelper.Keeper.SetBondProviders(ctx, bp), IsNil)
	c.Assert(keeperTestHelper.SetNodeAccount(ctx, nodeAccount), IsNil)

	voter := NewObservedTxVoter(tx.Tx.ID, make(ObservedTxs, 0))
	voter.Add(tx, nodeAccount.NodeAddress)
	voter.Tx = voter.GetTx(NodeAccounts{nodeAccount})
	voter.FinalisedHeight = ctx.BlockHeight()
	keeperTestHelper.SetObservedTxOutVoter(ctx, voter)

	constAccessor := constants.GetConstantValues(version)
	gasMgr := newGasMgrVCUR(constAccessor, keeperTestHelper)
	gasMgr.BeginBlock(mgr)
	txOutStorage := newTxOutStorageVCUR(keeperTestHelper, constAccessor, NewDummyEventMgr(), gasMgr)
	toi := TxOutItem{
		Chain:       common.BNBChain,
		ToAddress:   tx.Tx.FromAddress,
		VaultPubKey: yggVault.PubKey,
		Coin:        common.NewCoin(common.BNBAsset, cosmos.NewUint(2*common.One)),
		Memo:        NewRefundMemo(tx.Tx.ID).String(),
		InHash:      tx.Tx.ID,
	}
	result, err := txOutStorage.TryAddTxOutItem(ctx, mgr, toi, cosmos.ZeroUint())
	c.Assert(err, IsNil)
	c.Check(result, Equals, true)

	// Add missing balance from TryAddTxOutItem()
	p, err := mgr.K.GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	p.BalanceCacao = p.BalanceCacao.Add(cosmos.NewUint(112499))
	p.BalanceAsset = p.BalanceAsset.Sub(cosmos.NewUint(112500))
	err = mgr.K.SetPool(ctx, p)
	c.Assert(err, IsNil)

	return refundTxHandlerTestHelper{
		ctx:           ctx,
		pool:          pool,
		version:       version,
		keeper:        keeperTestHelper,
		asgardVault:   asgardVault,
		yggVault:      yggVault,
		nodeAccount:   nodeAccount,
		inboundTx:     tx,
		toi:           toi,
		constAccessor: constAccessor,
		mgr:           mgr,
	}
}

func (s *HandlerRefundSuite) TestRefundTxHandlerShouldUpdateTxOut(c *C) {
	testCases := []struct {
		name           string
		messageCreator func(helper refundTxHandlerTestHelper, tx ObservedTx) cosmos.Msg
		runner         func(handler RefundHandler, helper refundTxHandlerTestHelper, msg cosmos.Msg) (*cosmos.Result, error)
		expectedResult error
	}{
		{
			name: "invalid message should return an error",
			messageCreator: func(helper refundTxHandlerTestHelper, tx ObservedTx) cosmos.Msg {
				return NewMsgNoOp(GetRandomObservedTx(), helper.nodeAccount.NodeAddress, "")
			},
			runner: func(handler RefundHandler, helper refundTxHandlerTestHelper, msg cosmos.Msg) (*cosmos.Result, error) {
				return handler.Run(helper.ctx, msg)
			},
			expectedResult: errInvalidMessage,
		},
		{
			name: "fail to get observed TxVoter should result in an error",
			messageCreator: func(helper refundTxHandlerTestHelper, tx ObservedTx) cosmos.Msg {
				return NewMsgRefundTx(tx, helper.keeper.observeTxVoterErrHash, helper.nodeAccount.NodeAddress)
			},
			runner: func(handler RefundHandler, helper refundTxHandlerTestHelper, msg cosmos.Msg) (*cosmos.Result, error) {
				return handler.Run(helper.ctx, msg)
			},
			expectedResult: errInternal,
		},
		{
			name: "fail to get txout should result in an error",
			messageCreator: func(helper refundTxHandlerTestHelper, tx ObservedTx) cosmos.Msg {
				return NewMsgRefundTx(tx, helper.inboundTx.Tx.ID, helper.nodeAccount.NodeAddress)
			},
			runner: func(handler RefundHandler, helper refundTxHandlerTestHelper, msg cosmos.Msg) (*cosmos.Result, error) {
				helper.keeper.errGetTxOut = true
				return handler.Run(helper.ctx, msg)
			},
			expectedResult: se.ErrUnknownRequest,
		},
		{
			name: "valid outbound message, no event, no txout",
			messageCreator: func(helper refundTxHandlerTestHelper, tx ObservedTx) cosmos.Msg {
				return NewMsgRefundTx(tx, helper.inboundTx.Tx.ID, helper.nodeAccount.NodeAddress)
			},
			runner: func(handler RefundHandler, helper refundTxHandlerTestHelper, msg cosmos.Msg) (*cosmos.Result, error) {
				return handler.Run(helper.ctx, msg)
			},
			expectedResult: nil,
		},
	}

	for _, tc := range testCases {
		helper := newRefundTxHandlerTestHelper(c)
		handler := NewRefundHandler(helper.mgr)
		fromAddr, err := helper.yggVault.PubKey.GetAddress(common.BNBChain)
		c.Assert(err, IsNil)
		tx := NewObservedTx(common.Tx{
			ID:    GetRandomTxHash(),
			Chain: common.BNBChain,
			Coins: common.Coins{
				common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One)),
			},
			Memo:        NewRefundMemo(helper.inboundTx.Tx.ID).String(),
			FromAddress: fromAddr,
			ToAddress:   helper.inboundTx.Tx.FromAddress,
			Gas:         BNBGasFeeSingleton,
		}, helper.ctx.BlockHeight(), helper.yggVault.PubKey, helper.ctx.BlockHeight())
		msg := tc.messageCreator(helper, tx)
		_, err = tc.runner(handler, helper, msg)
		if tc.expectedResult == nil {
			c.Check(err, IsNil)
		} else {
			c.Check(errors.Is(err, tc.expectedResult), Equals, true, Commentf("name:%s", tc.name))
		}
	}
}

func (s *HandlerRefundSuite) TestRefundTxNormalCase(c *C) {
	helper := newRefundTxHandlerTestHelper(c)
	handler := NewRefundHandler(helper.mgr)

	fromAddr, err := helper.yggVault.PubKey.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)
	tx := NewObservedTx(common.Tx{
		ID:    GetRandomTxHash(),
		Chain: common.BNBChain,
		Coins: common.Coins{
			common.NewCoin(common.BNBAsset, cosmos.NewUint(199925000)), // 199887500
		},
		Memo:        NewRefundMemo(helper.inboundTx.Tx.ID).String(),
		FromAddress: fromAddr,
		ToAddress:   helper.inboundTx.Tx.FromAddress,
		Gas:         BNBGasFeeSingleton,
	}, helper.ctx.BlockHeight(), helper.yggVault.PubKey, helper.ctx.BlockHeight())
	// valid outbound message, with event, with txout
	outMsg := NewMsgRefundTx(tx, helper.inboundTx.Tx.ID, helper.nodeAccount.NodeAddress)
	_, err = handler.Run(helper.ctx, outMsg)
	c.Assert(err, IsNil)

	// txout should had been complete
	txOut, err := helper.keeper.GetTxOut(helper.ctx, helper.ctx.BlockHeight())
	c.Assert(err, IsNil)
	c.Assert(txOut.TxArray[0].OutHash.IsEmpty(), Equals, false)
}

func (s *HandlerRefundSuite) TestRefundTxHandlerSendExtraFundShouldBeSlashed(c *C) {
	helper := newRefundTxHandlerTestHelper(c)
	handler := NewRefundHandler(helper.mgr)
	fromAddr, err := helper.asgardVault.PubKey.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)
	tx := NewObservedTx(common.Tx{
		ID:    GetRandomTxHash(),
		Chain: common.BNBChain,
		Coins: common.Coins{
			common.NewCoin(common.BNBAsset, cosmos.NewUint(2*common.One)),
		},
		Memo:        NewRefundMemo(helper.inboundTx.Tx.ID).String(),
		FromAddress: fromAddr,
		ToAddress:   helper.inboundTx.Tx.FromAddress,
		Gas:         BNBGasFeeSingleton,
	}, helper.ctx.BlockHeight(), helper.nodeAccount.PubKeySet.Secp256k1, helper.ctx.BlockHeight())
	expectedBond := cosmos.NewUint(19699941500)                      // zlyzol: previous version value: 19699943750
	expectedVaultTotalReserve := cosmos.NewUint(7766279631252277919) // zlyzol: previous version value: 7766279631452335668
	// valid refund message, with event, with txout
	outMsg := NewMsgRefundTx(tx, helper.inboundTx.Tx.ID, helper.nodeAccount.NodeAddress)
	_, err = handler.Run(helper.ctx, outMsg)
	c.Assert(err, IsNil)
	na, err := helper.keeper.GetNodeAccount(helper.ctx, helper.nodeAccount.NodeAddress)
	c.Assert(err, IsNil)
	naBond, err := helper.keeper.CalcNodeLiquidityBond(helper.ctx, na)
	c.Assert(err, IsNil)
	c.Assert(naBond, DeepEquals, expectedBond, Commentf("expect bond %s, got %s", expectedBond, naBond))
	newReserve := helper.keeper.GetRuneBalanceOfModule(helper.ctx, ReserveName)
	c.Log(newReserve.String())
	c.Assert(newReserve, DeepEquals, expectedVaultTotalReserve, Commentf("expect reserve %s, got %s", expectedBond, naBond))
}

func (s *HandlerRefundSuite) TestOutboundTxHandlerSendAdditionalCoinsShouldBeSlashed(c *C) {
	helper := newRefundTxHandlerTestHelper(c)
	handler := NewRefundHandler(helper.mgr)
	fromAddr, err := helper.asgardVault.PubKey.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)
	tx := NewObservedTx(common.Tx{
		ID:    GetRandomTxHash(),
		Chain: common.BNBChain,
		Coins: common.Coins{
			common.NewCoin(common.BNBAsset, cosmos.NewUint(2*common.One)),
		},
		Memo:        NewRefundMemo(helper.inboundTx.Tx.ID).String(),
		FromAddress: fromAddr,
		ToAddress:   helper.inboundTx.Tx.FromAddress,
		Gas:         BNBGasFeeSingleton,
	}, helper.ctx.BlockHeight(), helper.nodeAccount.PubKeySet.Secp256k1, helper.ctx.BlockHeight())
	expectedBond := cosmos.NewUint(19699941500) // zlyzol: previous version value: 19699943750
	// slash one BNB and one rune
	outMsg := NewMsgRefundTx(tx, helper.inboundTx.Tx.ID, helper.nodeAccount.NodeAddress)
	_, err = handler.Run(helper.ctx, outMsg)
	c.Assert(err, IsNil)
	naBond, err := helper.keeper.CalcNodeLiquidityBond(helper.ctx, helper.nodeAccount)
	c.Assert(err, IsNil)
	c.Assert(naBond, DeepEquals, expectedBond, Commentf("expect %s, got %s", expectedBond, naBond))
}

func (s *HandlerRefundSuite) TestOutboundTxHandlerInvalidObservedTxVoterShouldSlash(c *C) {
	helper := newRefundTxHandlerTestHelper(c)
	handler := NewRefundHandler(helper.mgr)
	fromAddr, err := helper.asgardVault.PubKey.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)
	tx := NewObservedTx(common.Tx{
		ID:    GetRandomTxHash(),
		Chain: common.BNBChain,
		Coins: common.Coins{
			common.NewCoin(common.BNBAsset, cosmos.NewUint(2*common.One)),
		},
		Memo:        NewRefundMemo(helper.inboundTx.Tx.ID).String(),
		FromAddress: fromAddr,
		ToAddress:   helper.inboundTx.Tx.FromAddress,
		Gas:         BNBGasFeeSingleton,
	}, helper.ctx.BlockHeight(), helper.nodeAccount.PubKeySet.Secp256k1, helper.ctx.BlockHeight())

	expectedBond := cosmos.NewUint(19699941500)                      // zlyzol: previous version value: 19699943750
	expectedVaultTotalReserve := cosmos.NewUint(7766279631252277919) // zlyzol: previous version value: 7766279631502334918

	// expected 0.5 slashed RUNE be added to reserve
	pool, err := helper.keeper.GetPool(helper.ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	poolBNB := common.SafeSub(pool.BalanceAsset, cosmos.NewUint(2*common.One).Add(BNBGasFeeSingleton[0].Amount))

	// given the outbound tx doesn't have relevant OservedTxVoter in system ,
	// thus it should be slashed with 1.5 * the full amount of assets
	outMsg := NewMsgRefundTx(tx, tx.Tx.ID, helper.nodeAccount.NodeAddress)
	_, err = handler.Run(helper.ctx, outMsg)
	c.Assert(err, IsNil)
	naBond, err := helper.keeper.CalcNodeLiquidityBond(helper.ctx, helper.nodeAccount)
	c.Assert(err, IsNil)
	c.Assert(naBond, DeepEquals, expectedBond, Commentf("expect %s, got %s", expectedBond, naBond))

	newReserve := helper.keeper.GetRuneBalanceOfModule(helper.ctx, ReserveName)
	c.Assert(newReserve, DeepEquals, expectedVaultTotalReserve)
	pool, err = helper.keeper.GetPool(helper.ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	newBalance := cosmos.NewUint(10200076500) // 10099961000
	c.Assert(pool.BalanceCacao, DeepEquals, newBalance)
	c.Assert(pool.BalanceAsset, DeepEquals, poolBNB)

	// Balance should change due to subsidizing
	// zlyzol: check
	// pool, err = helper.keeper.GetPool(helper.ctx, common.BNBAsset)
	// c.Assert(err, IsNil)
	// expectedRuneBalance := cosmos.NewUint(100 * common.One).Add(cosmos.NewUint(2 * common.One)).Add(BNBGasFeeSingleton[0].Amount)
	// expectedAssetBalance := cosmos.NewUint(100 * common.One).Sub(cosmos.NewUint(2 * common.One).Add(BNBGasFeeSingleton[0].Amount))
	// c.Assert(pool.BalanceCacao, DeepEquals, expectedRuneBalance, Commentf("expect %s, got %s", expectedRuneBalance.Uint64(), pool.BalanceCacao.Uint64()))
	// c.Assert(pool.BalanceAsset, DeepEquals, expectedAssetBalance, Commentf("expected %s, got %s", poolBNB, pool.BalanceAsset))
}
