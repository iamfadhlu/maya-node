package mayachain

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/blang/semver"
	sdk "github.com/cosmos/cosmos-sdk/types"
	se "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/log"
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

type HandlerObservedTxInSuite struct{}

type TestObservedTxInValidateKeeper struct {
	keeper.KVStoreDummy
	activeNodeAccount NodeAccount
	standbyAccount    NodeAccount
}

func (k *TestObservedTxInValidateKeeper) GetNodeAccount(_ cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error) {
	if addr.Equals(k.standbyAccount.NodeAddress) {
		return k.standbyAccount, nil
	}
	if addr.Equals(k.activeNodeAccount.NodeAddress) {
		return k.activeNodeAccount, nil
	}
	return NodeAccount{}, errKaboom
}

func (k *TestObservedTxInValidateKeeper) SetNodeAccount(_ cosmos.Context, na NodeAccount) error {
	if na.NodeAddress.Equals(k.standbyAccount.NodeAddress) {
		k.standbyAccount = na
		return nil
	}
	return errKaboom
}

var _ = Suite(&HandlerObservedTxInSuite{})

func (s *HandlerObservedTxInSuite) TestValidate(c *C) {
	var err error
	ctx, _ := setupKeeperForTest(c)
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	standbyAccount := GetRandomValidatorNode(NodeStandby)
	keeper := &TestObservedTxInValidateKeeper{
		activeNodeAccount: activeNodeAccount,
		standbyAccount:    standbyAccount,
	}

	handler := NewObservedTxInHandler(NewDummyMgrWithKeeper(keeper))

	// happy path
	pk := GetRandomPubKey()
	txs := ObservedTxs{NewObservedTx(GetRandomTx(), 12, pk, 12)}
	txs[0].Tx.ToAddress, err = pk.GetAddress(txs[0].Tx.Coins[0].Asset.Chain)
	c.Assert(err, IsNil)
	msg := NewMsgObservedTxIn(txs, activeNodeAccount.NodeAddress)
	err = handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	// inactive node account
	msg = NewMsgObservedTxIn(txs, GetRandomBech32Addr())
	err = handler.validate(ctx, *msg)
	c.Assert(errors.Is(err, se.ErrUnauthorized), Equals, true)

	// invalid msg
	msg = &MsgObservedTxIn{}
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
}

type TestObservedTxInFailureKeeper struct {
	keeper.KVStoreDummy
	pool Pool
}

func (k *TestObservedTxInFailureKeeper) GetPool(_ cosmos.Context, _ common.Asset) (Pool, error) {
	return k.pool, nil
}

func (k *TestObservedTxInFailureKeeper) GetVault(_ cosmos.Context, pubKey common.PubKey) (Vault, error) {
	return Vault{
		PubKey:      pubKey,
		PubKeyEddsa: pubKey,
	}, nil
}

func (s *HandlerObservedTxInSuite) TestFailure(c *C) {
	ctx, _ := setupKeeperForTest(c)
	// w := getHandlerTestWrapper(c, 1, true, false)

	keeper := &TestObservedTxInFailureKeeper{
		pool: Pool{
			Asset:        common.BNBAsset,
			BalanceCacao: cosmos.NewUint(200),
			BalanceAsset: cosmos.NewUint(300),
		},
	}
	mgr := NewDummyMgrWithKeeper(keeper)

	tx := NewObservedTx(GetRandomTx(), 12, GetRandomPubKey(), 12)
	err := refundTx(ctx, tx, mgr, CodeInvalidMemo, "Invalid memo", "")
	c.Assert(err, IsNil)
	items, err := mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Check(items, HasLen, 1)
}

type TestObservedTxInHandleKeeper struct {
	keeper.KVStoreDummy
	nas                  NodeAccounts
	voter                ObservedTxVoter
	yggExists            bool
	height               int64
	msg                  MsgSwap
	pool                 Pool
	observing            []cosmos.AccAddress
	vault                Vault
	yggVault             Vault
	txOut                *TxOut
	setLastObserveHeight bool
}

func (k *TestObservedTxInHandleKeeper) SetSwapQueueItem(_ cosmos.Context, msg MsgSwap, _ int) error {
	k.msg = msg
	return nil
}

func (k *TestObservedTxInHandleKeeper) ListActiveValidators(_ cosmos.Context) (NodeAccounts, error) {
	return k.nas, nil
}

func (k *TestObservedTxInHandleKeeper) GetObservedTxInVoter(_ cosmos.Context, _ common.TxID) (ObservedTxVoter, error) {
	return k.voter, nil
}

func (k *TestObservedTxInHandleKeeper) SetObservedTxInVoter(_ cosmos.Context, voter ObservedTxVoter) {
	k.voter = voter
}

func (k *TestObservedTxInHandleKeeper) VaultExists(_ cosmos.Context, _ common.PubKey) bool {
	return k.yggExists
}

func (k *TestObservedTxInHandleKeeper) SetLastChainHeight(_ cosmos.Context, _ common.Chain, height int64) error {
	k.height = height
	return nil
}

func (k *TestObservedTxInHandleKeeper) AddObservingAddresses(_ cosmos.Context, addrs []cosmos.AccAddress) error {
	k.observing = addrs
	return nil
}

func (k *TestObservedTxInHandleKeeper) GetVault(_ cosmos.Context, key common.PubKey) (Vault, error) {
	if k.vault.PubKey.Equals(key) {
		return k.vault, nil
	} else if k.yggVault.PubKey.Equals(key) {
		return k.yggVault, nil
	}
	return GetRandomVault(), errKaboom
}

func (k *TestObservedTxInHandleKeeper) GetAsgardVaults(_ cosmos.Context) (Vaults, error) {
	return Vaults{k.vault}, nil
}

func (k *TestObservedTxInHandleKeeper) SetVault(_ cosmos.Context, vault Vault) error {
	if k.vault.PubKey.Equals(vault.PubKey) {
		k.vault = vault
		return nil
	} else if k.yggVault.PubKey.Equals(vault.PubKey) {
		k.yggVault = vault
	}
	return errKaboom
}

func (k *TestObservedTxInHandleKeeper) GetLowestActiveVersion(_ cosmos.Context) semver.Version {
	return GetCurrentVersion()
}

func (k *TestObservedTxInHandleKeeper) IsActiveObserver(_ cosmos.Context, addr cosmos.AccAddress) bool {
	return addr.Equals(k.nas[0].NodeAddress)
}

func (k *TestObservedTxInHandleKeeper) GetTxOut(ctx cosmos.Context, blockHeight int64) (*TxOut, error) {
	if k.txOut != nil && k.txOut.Height == blockHeight {
		return k.txOut, nil
	}
	return nil, errKaboom
}

func (k *TestObservedTxInHandleKeeper) SetTxOut(ctx cosmos.Context, blockOut *TxOut) error {
	if k.txOut.Height == blockOut.Height {
		k.txOut = blockOut
		return nil
	}
	return errKaboom
}

func (k *TestObservedTxInHandleKeeper) SetLastObserveHeight(ctx cosmos.Context, chain common.Chain, address cosmos.AccAddress, height int64) error {
	k.setLastObserveHeight = true
	return nil
}

func (s *HandlerObservedTxInSuite) TestHandle(c *C) {
	s.testHandleWithVersion(c)
	s.testHandleWithConfirmation(c)
}

func (s *HandlerObservedTxInSuite) testHandleWithConfirmation(c *C) {
	var err error
	ctx, mgr := setupManagerForTest(c)
	tx := GetRandomTx()
	tx.Memo = "SWAP:BTC.BTC:" + GetRandomBTCAddress().String()
	obTx := NewObservedTx(tx, 12, GetRandomPubKey(), 15)
	txs := ObservedTxs{obTx}
	pk := GetRandomPubKey()
	txs[0].Tx.ToAddress, err = pk.GetAddress(txs[0].Tx.Coins[0].Asset.Chain)
	c.Assert(err, IsNil)
	vault := GetRandomVault()
	vault.PubKey = obTx.ObservedPubKey

	keeper := &TestObservedTxInHandleKeeper{
		nas: NodeAccounts{
			GetRandomValidatorNode(NodeActive),
			GetRandomValidatorNode(NodeActive),
			GetRandomValidatorNode(NodeActive),
			GetRandomValidatorNode(NodeActive),
		},
		vault: vault,
		pool: Pool{
			Asset:        common.BNBAsset,
			BalanceCacao: cosmos.NewUint(200),
			BalanceAsset: cosmos.NewUint(300),
		},
		yggExists: true,
	}
	mgr.K = keeper
	handler := NewObservedTxInHandler(mgr)

	// first not confirmed message
	msg := NewMsgObservedTxIn(txs, keeper.nas[0].NodeAddress)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	voter, err := keeper.GetObservedTxInVoter(ctx, tx.ID)
	c.Assert(err, IsNil)
	c.Assert(voter.Txs, HasLen, 1)
	// tx has not reach consensus yet, thus fund should not be credit to vault
	c.Assert(keeper.vault.HasFunds(), Equals, false)
	c.Assert(voter.UpdatedVault, Equals, false)
	c.Assert(voter.FinalisedHeight, Equals, int64(0))
	c.Assert(voter.Height, Equals, int64(0))
	mgr.ObMgr().EndBlock(ctx, keeper)

	// second not confirmed message
	msg1 := NewMsgObservedTxIn(txs, keeper.nas[1].NodeAddress)
	_, err = handler.handle(ctx, *msg1)
	c.Assert(err, IsNil)
	voter, err = keeper.GetObservedTxInVoter(ctx, tx.ID)
	c.Assert(err, IsNil)
	c.Assert(voter.Txs, HasLen, 1)
	c.Assert(voter.UpdatedVault, Equals, false)
	c.Assert(voter.FinalisedHeight, Equals, int64(0))
	c.Assert(voter.Height, Equals, int64(0))
	c.Assert(keeper.vault.HasFunds(), Equals, false)

	// third not confirmed message
	msg2 := NewMsgObservedTxIn(txs, keeper.nas[2].NodeAddress)
	_, err = handler.handle(ctx, *msg2)
	c.Assert(err, IsNil)
	voter, err = keeper.GetObservedTxInVoter(ctx, tx.ID)
	c.Assert(err, IsNil)
	c.Assert(voter.Txs, HasLen, 1)
	c.Assert(voter.UpdatedVault, Equals, false)
	c.Assert(voter.FinalisedHeight, Equals, int64(0))
	c.Check(keeper.height, Equals, int64(12))
	// make sure fund has been credit to vault correctly
	bnbCoin := keeper.vault.Coins.GetCoin(common.BNBAsset)
	c.Assert(bnbCoin.Amount.Equal(cosmos.ZeroUint()), Equals, true)
	// make sure the logic has not been processed , as tx has not been finalised , still waiting for confirmation
	c.Check(keeper.msg.Tx.ID.Equals(tx.ID), Equals, false)

	// fourth not confirmed message
	msg3 := NewMsgObservedTxIn(txs, keeper.nas[3].NodeAddress)
	_, err = handler.handle(ctx, *msg3)
	c.Assert(err, IsNil)
	voter, err = keeper.GetObservedTxInVoter(ctx, tx.ID)
	c.Assert(err, IsNil)
	c.Assert(voter.Txs, HasLen, 1)
	c.Assert(voter.UpdatedVault, Equals, false)
	c.Assert(voter.FinalisedHeight, Equals, int64(0))
	c.Check(keeper.height, Equals, int64(12))
	// make sure fund has not been doubled
	bnbCoin = keeper.vault.Coins.GetCoin(common.BNBAsset)
	c.Assert(bnbCoin.Amount.Equal(cosmos.ZeroUint()), Equals, true)
	c.Check(keeper.msg.Tx.ID.Equals(tx.ID), Equals, false)

	//  first finalised message
	txs[0].BlockHeight = 15
	fMsg := NewMsgObservedTxIn(txs, keeper.nas[0].NodeAddress)
	_, err = handler.handle(ctx, *fMsg)
	c.Assert(err, IsNil)
	voter, err = keeper.GetObservedTxInVoter(ctx, tx.ID)
	c.Assert(err, IsNil)
	c.Assert(voter.UpdatedVault, Equals, false)
	c.Assert(voter.FinalisedHeight, Equals, int64(0))
	c.Assert(voter.Height, Equals, int64(18))
	// make sure fund has not been doubled
	bnbCoin = keeper.vault.Coins.GetCoin(common.BNBAsset)
	c.Assert(bnbCoin.Amount.Equal(cosmos.ZeroUint()), Equals, true)
	c.Check(keeper.msg.Tx.ID.Equals(tx.ID), Equals, false)

	// second finalised message
	fMsg1 := NewMsgObservedTxIn(txs, keeper.nas[1].NodeAddress)
	_, err = handler.handle(ctx, *fMsg1)
	c.Assert(err, IsNil)
	voter, err = keeper.GetObservedTxInVoter(ctx, tx.ID)
	c.Assert(err, IsNil)
	c.Assert(voter.UpdatedVault, Equals, false)
	c.Assert(voter.FinalisedHeight, Equals, int64(0))
	c.Assert(voter.Height, Equals, int64(18))
	bnbCoin = keeper.vault.Coins.GetCoin(common.BNBAsset)
	c.Assert(bnbCoin.Amount.Equal(cosmos.ZeroUint()), Equals, true)
	c.Check(keeper.msg.Tx.ID.Equals(tx.ID), Equals, false)

	// third finalised message
	fMsg2 := NewMsgObservedTxIn(txs, keeper.nas[2].NodeAddress)
	_, err = handler.handle(ctx, *fMsg2)
	c.Assert(err, IsNil)
	voter, err = keeper.GetObservedTxInVoter(ctx, tx.ID)
	c.Assert(err, IsNil)
	c.Assert(voter.UpdatedVault, Equals, true)
	c.Assert(voter.FinalisedHeight, Equals, int64(18))
	c.Assert(voter.Height, Equals, int64(18))
	// make sure fund has not been doubled
	bnbCoin = keeper.vault.Coins.GetCoin(common.BNBAsset)
	c.Assert(bnbCoin.Amount.Equal(cosmos.OneUint()), Equals, true)
	c.Check(keeper.msg.Tx.ID.String(), Equals, tx.ID.String())

	// third finalised message
	fMsg3 := NewMsgObservedTxIn(txs, keeper.nas[3].NodeAddress)
	_, err = handler.handle(ctx, *fMsg3)
	c.Assert(err, IsNil)
	voter, err = keeper.GetObservedTxInVoter(ctx, tx.ID)
	c.Assert(err, IsNil)
	c.Assert(voter.UpdatedVault, Equals, true)
	c.Assert(voter.FinalisedHeight, Equals, int64(18))
	c.Assert(voter.Height, Equals, int64(18))
	// make sure fund has not been doubled
	bnbCoin = keeper.vault.Coins.GetCoin(common.BNBAsset)
	c.Assert(bnbCoin.Amount.Equal(cosmos.OneUint()), Equals, true)
	c.Check(keeper.msg.Tx.ID.String(), Equals, tx.ID.String())
}

func (s *HandlerObservedTxInSuite) testHandleWithVersion(c *C) {
	var err error
	ctx, mgr := setupManagerForTest(c)

	tx := GetRandomTx()
	tx.Memo = "SWAP:BTC.BTC:" + GetRandomBTCAddress().String()
	obTx := NewObservedTx(tx, 12, GetRandomPubKey(), 12)
	txs := ObservedTxs{obTx}
	pk := GetRandomPubKey()
	txs[0].Tx.ToAddress, err = pk.GetAddress(txs[0].Tx.Coins[0].Asset.Chain)

	vault := GetRandomVault()
	vault.PubKey = obTx.ObservedPubKey

	keeper := &TestObservedTxInHandleKeeper{
		nas:   NodeAccounts{GetRandomValidatorNode(NodeActive)},
		voter: NewObservedTxVoter(tx.ID, make(ObservedTxs, 0)),
		vault: vault,
		pool: Pool{
			Asset:        common.BNBAsset,
			BalanceCacao: cosmos.NewUint(200),
			BalanceAsset: cosmos.NewUint(300),
		},
		yggExists: true,
	}
	mgr.K = keeper
	handler := NewObservedTxInHandler(mgr)

	c.Assert(err, IsNil)
	msg := NewMsgObservedTxIn(txs, keeper.nas[0].NodeAddress)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	mgr.ObMgr().EndBlock(ctx, keeper)
	c.Check(keeper.msg.Tx.ID.Equals(tx.ID), Equals, true)
	c.Check(keeper.observing, HasLen, 1)
	c.Check(keeper.height, Equals, int64(12))
	bnbCoin := keeper.vault.Coins.GetCoin(common.BNBAsset)
	c.Assert(bnbCoin.Amount.Equal(cosmos.OneUint()), Equals, true)
}

// Test migrate memo
func (s *HandlerObservedTxInSuite) TestMigrateMemo(c *C) {
	var err error
	ctx, _ := setupKeeperForTest(c)

	vault := GetRandomVault()
	addr, err := vault.PubKey.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)
	newVault := GetRandomVault()
	txout := NewTxOut(12)
	newVaultAddr, err := newVault.PubKey.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)

	txout.TxArray = append(txout.TxArray, TxOutItem{
		Chain:       common.BNBChain,
		InHash:      common.BlankTxID,
		ToAddress:   newVaultAddr,
		VaultPubKey: vault.PubKey,
		Coin:        common.NewCoin(common.BNBAsset, cosmos.NewUint(1024)),
		Memo:        NewMigrateMemo(1).String(),
	})
	tx := NewObservedTx(common.Tx{
		ID:    GetRandomTxHash(),
		Chain: common.BNBChain,
		Coins: common.Coins{
			common.NewCoin(common.BNBAsset, cosmos.NewUint(1024)),
		},
		Memo:        NewMigrateMemo(12).String(),
		FromAddress: addr,
		ToAddress:   newVaultAddr,
		Gas:         BNBGasFeeSingleton,
	}, 13, vault.PubKey, 13)

	txs := ObservedTxs{tx}
	keeper := &TestObservedTxInHandleKeeper{
		nas:   NodeAccounts{GetRandomValidatorNode(NodeActive)},
		voter: NewObservedTxVoter(tx.Tx.ID, make(ObservedTxs, 0)),
		vault: vault,
		pool: Pool{
			Asset:        common.BNBAsset,
			BalanceCacao: cosmos.NewUint(200),
			BalanceAsset: cosmos.NewUint(300),
		},
		yggExists: true,
		txOut:     txout,
	}

	handler := NewObservedTxInHandler(NewDummyMgrWithKeeper(keeper))

	c.Assert(err, IsNil)
	msg := NewMsgObservedTxIn(txs, keeper.nas[0].NodeAddress)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
}

type ObservedTxInHandlerTestHelper struct {
	keeper.Keeper
	failListActiveValidators bool
	failVaultExist           bool
	failGetObservedTxInVote  bool
	failGetVault             bool
	failSetVault             bool
}

func NewObservedTxInHandlerTestHelper(k keeper.Keeper) *ObservedTxInHandlerTestHelper {
	return &ObservedTxInHandlerTestHelper{
		Keeper: k,
	}
}

func (h *ObservedTxInHandlerTestHelper) ListActiveValidators(ctx cosmos.Context) (NodeAccounts, error) {
	if h.failListActiveValidators {
		return NodeAccounts{}, errKaboom
	}
	return h.Keeper.ListActiveValidators(ctx)
}

func (h *ObservedTxInHandlerTestHelper) VaultExists(ctx cosmos.Context, pk common.PubKey) bool {
	if h.failVaultExist {
		return false
	}
	return h.Keeper.VaultExists(ctx, pk)
}

func (h *ObservedTxInHandlerTestHelper) GetObservedTxInVoter(ctx cosmos.Context, hash common.TxID) (ObservedTxVoter, error) {
	if h.failGetObservedTxInVote {
		return ObservedTxVoter{}, errKaboom
	}
	return h.Keeper.GetObservedTxInVoter(ctx, hash)
}

func (h *ObservedTxInHandlerTestHelper) GetVault(ctx cosmos.Context, pk common.PubKey) (Vault, error) {
	if h.failGetVault {
		return Vault{}, errKaboom
	}
	return h.Keeper.GetVault(ctx, pk)
}

func (h *ObservedTxInHandlerTestHelper) SetVault(ctx cosmos.Context, vault Vault) error {
	if h.failSetVault {
		return errKaboom
	}
	return h.Keeper.SetVault(ctx, vault)
}

func setupAnLegitObservedTx(ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper, c *C) *MsgObservedTxIn {
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	pk := GetRandomPubKey()
	tx := GetRandomTx()
	tx.Coins = common.Coins{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One*3)),
	}
	tx.Memo = "SWAP:RUNE"
	addr, err := pk.GetAddress(tx.Coins[0].Asset.Chain)
	c.Assert(err, IsNil)
	tx.ToAddress = addr
	obTx := NewObservedTx(tx, ctx.BlockHeight(), pk, ctx.BlockHeight())
	txs := ObservedTxs{obTx}
	txs[0].Tx.ToAddress, err = pk.GetAddress(txs[0].Tx.Coins[0].Asset.Chain)
	c.Assert(err, IsNil)
	vault := GetRandomVault()
	vault.PubKey = obTx.ObservedPubKey
	c.Assert(helper.Keeper.SetNodeAccount(ctx, activeNodeAccount), IsNil)
	c.Assert(helper.SetVault(ctx, vault), IsNil)
	p := NewPool()
	p.Asset = common.BNBAsset
	p.BalanceCacao = cosmos.NewUint(100 * common.One)
	p.BalanceAsset = cosmos.NewUint(100 * common.One)
	p.Status = PoolAvailable
	c.Assert(helper.Keeper.SetPool(ctx, p), IsNil)
	return NewMsgObservedTxIn(ObservedTxs{
		obTx,
	}, activeNodeAccount.NodeAddress)
}

func (HandlerObservedTxInSuite) TestObservedTxHandler_validations(c *C) {
	testCases := []struct {
		name            string
		messageProvider func(c *C, ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper) cosmos.Msg
		validator       func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ObservedTxInHandlerTestHelper, name string)
	}{
		{
			name: "invalid message should return an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper) cosmos.Msg {
				return NewMsgNetworkFee(ctx.BlockHeight(), common.BNBChain, 1, bnbSingleTxFee.Uint64(), GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ObservedTxInHandlerTestHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil)
				c.Check(errors.Is(err, errInvalidMessage), Equals, true)
			},
		},
		{
			name: "message fail validation should return an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper) cosmos.Msg {
				return NewMsgObservedTxIn(ObservedTxs{
					NewObservedTx(GetRandomTx(), ctx.BlockHeight(), GetRandomPubKey(), ctx.BlockHeight()),
				}, GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ObservedTxInHandlerTestHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil)
			},
		},
		{
			name: "signer vote for the same tx should be slashed , and not doing anything else",
			messageProvider: func(c *C, ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper) cosmos.Msg {
				m := setupAnLegitObservedTx(ctx, helper, c)
				voter, err := helper.Keeper.GetObservedTxInVoter(ctx, m.Txs[0].Tx.ID)
				c.Assert(err, IsNil)
				voter.Add(m.Txs[0], m.Signer)
				helper.Keeper.SetObservedTxInVoter(ctx, voter)
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ObservedTxInHandlerTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
		},
		{
			name: "fail to list active node accounts should result in an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper) cosmos.Msg {
				m := setupAnLegitObservedTx(ctx, helper, c)
				helper.failListActiveValidators = true
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ObservedTxInHandlerTestHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
		},
		{
			name: "vault not exist should not result in an error, it should continue",
			messageProvider: func(c *C, ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper) cosmos.Msg {
				m := setupAnLegitObservedTx(ctx, helper, c)
				helper.failVaultExist = true
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ObservedTxInHandlerTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
		},
		{
			name: "fail to get observedTxInVoter should not result in an error, it should continue",
			messageProvider: func(c *C, ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper) cosmos.Msg {
				m := setupAnLegitObservedTx(ctx, helper, c)
				helper.failGetObservedTxInVote = true
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ObservedTxInHandlerTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
		},
		{
			name: "empty memo should not result in an error, it should continue",
			messageProvider: func(c *C, ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper) cosmos.Msg {
				m := setupAnLegitObservedTx(ctx, helper, c)
				m.Txs[0].Tx.Memo = ""
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ObservedTxInHandlerTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
				txOut, err := helper.GetTxOut(ctx, ctx.BlockHeight())
				c.Assert(err, IsNil, Commentf(name))
				c.Assert(txOut.IsEmpty(), Equals, false)
			},
		},
		{
			name: "fail to get vault, it should continue",
			messageProvider: func(c *C, ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper) cosmos.Msg {
				m := setupAnLegitObservedTx(ctx, helper, c)
				helper.failGetVault = true
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ObservedTxInHandlerTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
		},
		{
			name: "fail to set vault, it should continue",
			messageProvider: func(c *C, ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper) cosmos.Msg {
				m := setupAnLegitObservedTx(ctx, helper, c)
				helper.failSetVault = true
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ObservedTxInHandlerTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
		},
		{
			name: "if the vault is not asgard, it should continue",
			messageProvider: func(c *C, ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper) cosmos.Msg {
				m := setupAnLegitObservedTx(ctx, helper, c)
				vault, err := helper.Keeper.GetVault(ctx, m.Txs[0].ObservedPubKey)
				c.Assert(err, IsNil)
				vault.Type = YggdrasilVault
				c.Assert(helper.Keeper.SetVault(ctx, vault), IsNil)
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ObservedTxInHandlerTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
		},
		{
			name: "inactive vault, it should refund",
			messageProvider: func(c *C, ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper) cosmos.Msg {
				m := setupAnLegitObservedTx(ctx, helper, c)
				vault, err := helper.Keeper.GetVault(ctx, m.Txs[0].ObservedPubKey)
				c.Assert(err, IsNil)
				vault.Status = InactiveVault
				c.Assert(helper.Keeper.SetVault(ctx, vault), IsNil)
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ObservedTxInHandlerTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
				txOut, err := helper.GetTxOut(ctx, ctx.BlockHeight())
				c.Assert(err, IsNil, Commentf(name))
				c.Assert(txOut.IsEmpty(), Equals, false)
			},
		},
		{
			name: "chain halt, it should refund",
			messageProvider: func(c *C, ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper) cosmos.Msg {
				m := setupAnLegitObservedTx(ctx, helper, c)
				helper.Keeper.SetMimir(ctx, "HaltTrading", 1)
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ObservedTxInHandlerTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
				txOut, err := helper.GetTxOut(ctx, ctx.BlockHeight())
				c.Assert(err, IsNil, Commentf(name))
				c.Assert(txOut.IsEmpty(), Equals, false)
			},
		},
		{
			name: "normal provision, it should success",
			messageProvider: func(c *C, ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper) cosmos.Msg {
				m := setupAnLegitObservedTx(ctx, helper, c)
				m.Txs[0].Tx.Memo = "add:BNB.BNB"
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ObservedTxInHandlerTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
		},
	}
	versions := []semver.Version{
		GetCurrentVersion(),
	}
	for _, tc := range testCases {
		for _, ver := range versions {
			ctx, mgr := setupManagerForTest(c)
			helper := NewObservedTxInHandlerTestHelper(mgr.Keeper())
			mgr.K = helper
			mgr.currentVersion = ver
			handler := NewObservedTxInHandler(mgr)
			msg := tc.messageProvider(c, ctx, helper)
			result, err := handler.Run(ctx, msg)
			tc.validator(c, ctx, result, err, helper, tc.name)
		}
	}
}

func (s HandlerObservedTxInSuite) TestSwapWithAffiliate(c *C) {
	ctx, mgr := setupManagerForTest(c)

	queue := newSwapQueueVCUR(mgr.Keeper())
	handler := NewObservedTxInHandler(mgr)

	affAddr := GetRandomBaseAddress()
	msg := NewMsgSwap(common.Tx{
		ID:          common.TxID("5E1DF027321F1FE37CA19B9ECB11C2B4ABEC0D8322199D335D9CE4C39F85F115"),
		FromAddress: GetRandomBNBAddress(),
		ToAddress:   GetRandomBNBAddress(),
		Gas:         BNBGasFeeSingleton,
		Chain:       common.BNBChain,
		Coins:       common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(2*common.One))},
		Memo:        "=:ETH.ETH:" + GetRandomETHAddress().String() + "::" + affAddr.String() + ":100",
	}, common.BNBAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), GetRandomBaseAddress(), cosmos.NewUint(100),
		"", "", nil,
		MarketOrder,
		0, 0,
		GetRandomBech32Addr(),
	)
	handler.addSwap(ctx, *msg)
	swaps, err := queue.FetchQueue(ctx, mgr)
	c.Assert(err, IsNil)
	c.Assert(swaps, HasLen, 1)
	c.Check(swaps[0].msg.Tx.Coins[0].Amount.Uint64(), Equals, uint64(200000000))
}

func (s *HandlerObservedTxInSuite) TestVaultStatus(c *C) {
	testCases := []struct {
		name                 string
		statusAtConsensus    VaultStatus
		statusAtFinalisation VaultStatus
	}{
		{
			name:                 "should observe if active on consensus and finalisation",
			statusAtConsensus:    ActiveVault,
			statusAtFinalisation: ActiveVault,
		}, {
			name:                 "should observe if active on consensus, inactive on finalisation",
			statusAtConsensus:    ActiveVault,
			statusAtFinalisation: InactiveVault,
		}, {
			name:                 "should not observe if inactive on consensus",
			statusAtConsensus:    InactiveVault,
			statusAtFinalisation: InactiveVault,
		},
	}
	for _, tc := range testCases {
		var err error
		ctx, mgr := setupManagerForTest(c)
		tx := GetRandomTx()
		tx.Memo = "SWAP:BTC.BTC:" + GetRandomBTCAddress().String()
		obTx := NewObservedTx(tx, 12, GetRandomPubKey(), 15)
		txs := ObservedTxs{obTx}
		vault := GetRandomVault()
		vault.PubKey = obTx.ObservedPubKey
		keeper := &TestObservedTxInHandleKeeper{
			nas:   NodeAccounts{GetRandomValidatorNode(NodeActive)},
			voter: NewObservedTxVoter(tx.ID, make(ObservedTxs, 0)),
			vault: vault,
			pool: Pool{
				Asset:        common.BNBAsset,
				BalanceCacao: cosmos.NewUint(200),
				BalanceAsset: cosmos.NewUint(300),
			},
			yggExists: true,
		}
		mgr.K = keeper
		handler := NewObservedTxInHandler(mgr)

		keeper.vault.Status = tc.statusAtConsensus
		msg := NewMsgObservedTxIn(txs, keeper.nas[0].NodeAddress)
		_, err = handler.handle(ctx, *msg)
		c.Assert(err, IsNil, Commentf(tc.name))
		c.Check(keeper.voter.Height, Equals, int64(18), Commentf(tc.name))

		c.Check(keeper.voter.UpdatedVault, Equals, false, Commentf(tc.name))
		c.Check(keeper.vault.InboundTxCount, Equals, int64(0), Commentf(tc.name))

		keeper.vault.Status = tc.statusAtFinalisation
		txs[0].BlockHeight = 15
		msg = NewMsgObservedTxIn(txs, keeper.nas[0].NodeAddress)
		ctx = ctx.WithBlockHeight(30)
		_, err = handler.handle(ctx, *msg)
		c.Assert(err, IsNil, Commentf(tc.name))
		c.Check(keeper.voter.FinalisedHeight, Equals, int64(30), Commentf(tc.name))

		c.Check(keeper.voter.UpdatedVault, Equals, true, Commentf(tc.name))
		c.Check(keeper.vault.InboundTxCount, Equals, int64(1), Commentf(tc.name))
	}
}

func (s *HandlerObservedTxInSuite) TestYggFundedOnlyFromAsgard(c *C) {
	ctx, mgr := setupManagerForTest(c)

	tx := GetRandomTx()
	vault := GetRandomVault()
	obTx := NewObservedTx(tx, 12, GetRandomPubKey(), 15)

	vault.PubKey = obTx.ObservedPubKey
	vault.Type = AsgardVault
	asgardFromAddr, err := vault.PubKey.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)
	obTx.Tx.FromAddress = asgardFromAddr

	keeper := &TestObservedTxInHandleKeeper{
		nas:       NodeAccounts{GetRandomValidatorNode(NodeActive)},
		voter:     NewObservedTxVoter(tx.ID, make(ObservedTxs, 0)),
		vault:     vault,
		yggExists: true,
	}
	mgr.K = keeper
	handler := NewObservedTxInHandler(mgr)

	// Test isFromAsgard func
	isFromAsgard, err := handler.isFromAsgard(ctx, obTx)
	c.Assert(err, IsNil)
	c.Assert(isFromAsgard, Equals, true)

	obTx.Tx.FromAddress = GetRandomBNBAddress()
	isFromAsgard, err = handler.isFromAsgard(ctx, obTx)
	c.Assert(err, IsNil)
	c.Assert(isFromAsgard, Equals, false)

	// TX is not from Asgard, shouldn't fund Ygg
	fundTx := GetRandomTx()
	fundTx.Memo = "yggdrasil+:15"
	obTx = NewObservedTx(fundTx, 12, GetRandomPubKey(), 15)
	ygg := GetRandomVault()
	ygg.Type = YggdrasilVault
	ygg.PubKey = obTx.ObservedPubKey

	txValue := tx.Coins[0].Amount
	yggBnbBalanceBefore := ygg.GetCoin(common.BNBAsset).Amount

	keeper.yggVault = ygg
	keeper.voter = NewObservedTxVoter(tx.ID, make(ObservedTxs, 0))
	mgr.K = keeper

	handler = NewObservedTxInHandler(mgr)
	txs := ObservedTxs{obTx}
	txs[0].BlockHeight = 15
	msg := NewMsgObservedTxIn(txs, keeper.nas[0].NodeAddress)

	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	c.Assert(keeper.yggVault.GetCoin(common.BNBAsset).Amount.Sub(yggBnbBalanceBefore).Uint64(), Equals, cosmos.ZeroUint().Uint64())

	// TX is from asgard, should fund Ygg
	keeper.voter = NewObservedTxVoter(tx.ID, make(ObservedTxs, 0))
	mgr.K = keeper

	handler = NewObservedTxInHandler(mgr)
	txs[0].Tx.FromAddress = asgardFromAddr
	msg = NewMsgObservedTxIn(txs, keeper.nas[0].NodeAddress)

	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	c.Assert(keeper.yggVault.GetCoin(common.BNBAsset).Amount.Sub(yggBnbBalanceBefore).Uint64(), Equals, txValue.Uint64())
}

// *************************************************************************************************
// *************************************************************************************************
// *************************************************************************************************
// *********
// *********              AFFILIATE TESTS
// *********
// *************************************************************************************************
// *************************************************************************************************
// *************************************************************************************************

var (
	cacaoFee       = cosmos.NewUint(2000000000)
	simpleShareBps = cosmos.NewUint(1_50) // 1.5% share for mayaname 'simple'
	affShareBps    = cosmos.NewUint(1_50) // 1.5% share for mayaname 'aff'
)

type MultipleAffiliatesSuite struct {
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
	affMns          []affMn
	affMnMap        map[common.Address]int
	subafAddress    common.Address
	affColAddress   common.Address
	accAffCol       cosmos.AccAddress
	addrSimple      common.Address
	accSimple       cosmos.AccAddress
	perc100         cosmos.Uint
	version         semver.Version
}

var _ = Suite(&MultipleAffiliatesSuite{})

type affMn struct {
	name   string
	parent string
	bps    cosmos.Uint
	resbps cosmos.Uint
	pref   string
}

type MockWithdrawTxOutStoreForMultiAff struct {
	TxOutStore
	tois   []TxOutItem
	full   bool
	asgard Vault
}

func (store *MockWithdrawTxOutStoreForMultiAff) TryAddTxOutItem(ctx cosmos.Context, mgr Manager, toi TxOutItem, minOut cosmos.Uint) (bool, error) {
	if store.full {
		toi.VaultPubKey = store.asgard.PubKey
		success, err := store.TxOutStore.TryAddTxOutItem(ctx, mgr, toi, minOut)
		if success && err == nil {
			store.tois = append(store.tois, toi)
		}
		return success, err
	} else {
		store.tois = append(store.tois, toi)
		return true, nil
	}
}

type TestAffiliateKeeper struct {
	keeper.Keeper
	nas       NodeAccounts
	voter     ObservedTxVoter
	voterTxID common.TxID
	height    int64
	vault     Vault
	asgard    Vault
}

func (k *TestAffiliateKeeper) ListActiveValidators(_ cosmos.Context) (NodeAccounts, error) {
	return k.nas, nil
}

func (k *TestAffiliateKeeper) GetObservedTxInVoter(ctx cosmos.Context, hash common.TxID) (ObservedTxVoter, error) {
	return k.Keeper.GetObservedTxInVoter(ctx, hash)
	/*
		if hash.Equals(k.voter.TxID) {
			return k.voter, nil
		}
		return ObservedTxVoter{TxID: hash}, nil
	*/
}

func (k *TestAffiliateKeeper) SetObservedTxInVoter(ctx cosmos.Context, voter ObservedTxVoter) {
	k.Keeper.SetObservedTxInVoter(ctx, voter)
	// k.voter = voter
}

func (k *TestAffiliateKeeper) VaultExists(_ cosmos.Context, key common.PubKey) bool {
	return k.vault.PubKey.Equals(key)
}

func (k *TestAffiliateKeeper) SetLastChainHeight(_ cosmos.Context, _ common.Chain, height int64) error {
	k.height = height
	return nil
}

func (k *TestAffiliateKeeper) GetVault(_ cosmos.Context, key common.PubKey) (Vault, error) {
	if k.vault.PubKey.Equals(key) {
		return k.vault, nil
	}
	if k.asgard.PubKey.Equals(key) {
		return k.asgard, nil
	}
	return GetRandomVault(), errKaboom
}

func (k *TestAffiliateKeeper) SetVault(_ cosmos.Context, vault Vault) error {
	if k.vault.PubKey.Equals(vault.PubKey) {
		k.vault = vault
		return nil
	}
	if k.asgard.PubKey.Equals(vault.PubKey) {
		k.asgard = vault
		return nil
	}
	return errKaboom
}

func (k *TestAffiliateKeeper) GetLowestActiveVersion(_ cosmos.Context) semver.Version {
	return GetCurrentVersion()
}

func (k *TestAffiliateKeeper) IsActiveObserver(_ cosmos.Context, addr cosmos.AccAddress) bool {
	return addr.Equals(k.nas[0].NodeAddress)
}

func (k *TestAffiliateKeeper) GetAsgardVaults(_ cosmos.Context) (Vaults, error) {
	return Vaults{k.asgard}, nil
}

func (k *TestAffiliateKeeper) GetAsgardVaultsByStatus(ctx cosmos.Context, status VaultStatus) (Vaults, error) {
	return Vaults{k.asgard}, nil
}

// ErrorLogCollector is a custom logger writer that filters and collects specific error log messages with the defined substring
// to verify fully ObservedTxInHandler handler tests
type ErrorLogCollector struct {
	mu          sync.Mutex
	substring   string
	checkPrefix bool
	prefix      string
	ignore      []string
	showLogs    bool
	collected   []string
}

// Write processes the log entry and collects error messages ("E") that contain the specified substring
func (mc *ErrorLogCollector) Write(p []byte) (n int, err error) {
	message := string(p)
	if mc.showLogs {
		fmt.Print(message)
	}
	if mc.shouldCollect(message) {
		mc.mu.Lock()
		mc.collected = append(mc.collected, message)
		mc.mu.Unlock()
	}
	return len(p), nil
}

// shouldCollect checks if the message should be collected based on the substring position.
func (mc *ErrorLogCollector) shouldCollect(msg string) bool {
	hasPrefix := mc.checkPrefix && strings.HasPrefix(msg, mc.prefix)
	hasSubstring := len(mc.substring) != 0 && strings.Contains(msg, mc.substring)
	hasIgnore := false
	for _, ignore := range mc.ignore {
		if hasIgnore = strings.Contains(msg, ignore); hasIgnore {
			break
		}
	}
	return (hasPrefix || hasSubstring) && !hasIgnore
}

// GetCollected returns the collected log messages
func (mc *ErrorLogCollector) GetCollected(clear bool) []string {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	c := make([]string, 0, len(mc.collected))
	c = append(c, mc.collected...)
	if clear {
		mc.Clear()
	}
	return c
}

// GetCollectedString returns the collected log messages as one string
func (mc *ErrorLogCollector) GetCollectedString(clear bool) (string, int) {
	s := ""
	logs := mc.GetCollected(clear)
	if len(logs) > 0 {
		s = logs[0]
		for i := 1; i < len(logs); i++ {
			s = s + "\n" + logs[i]
		}
		s = fmt.Sprintf("%d error log occurred: %s", len(logs), s)
	}
	return s, len(logs)
}

func (mc *ErrorLogCollector) Clear() {
	mc.collected = make([]string, 0)
}

// Info implements the log.Info interface, filtering and collecting log messages
func (mc *ErrorLogCollector) Info(msg string, keyvals ...interface{}) {
	if strings.Contains(msg, mc.substring) {
		mc.mu.Lock()
		mc.collected = append(mc.collected, msg)
		mc.mu.Unlock()
	}
}

// Debug implements the log.Debug interface
func (mc *ErrorLogCollector) Debug(msg string, keyvals ...interface{}) {}

// Error implements the log.Error interface
func (mc *ErrorLogCollector) Error(msg string, keyvals ...interface{}) {
	if strings.Contains(msg, mc.substring) {
		mc.mu.Lock()
		mc.collected = append(mc.collected, msg)
		mc.mu.Unlock()
	}
}

// With implements the log.With interface
func (mc *ErrorLogCollector) With(keyvals ...interface{}) log.Logger {
	return mc
}

// NewMessageCollector creates a new MessageCollector.
func NewErrorLogCollector(substring, prefix string, ignore []string) *ErrorLogCollector {
	zly := os.Getenv("ZLY")
	showLogs := strings.Contains(zly, "showlogs")
	e := ErrorLogCollector{
		substring: substring,
		collected: make([]string, 0),
		ignore:    ignore,
		showLogs:  showLogs,
	}
	if prefix != "" {
		e.checkPrefix = true
		e.prefix = prefix
	}
	return &e
}

// This block provides utilities for checking approximate equality between numeric values,
// supporting `cosmos.Uint` and standard integer types (`int`, `uint`, `int64`, `uint64`).
// It includes:
//
// 1. `AlmostEqualBps`: A helper function to determine if two numeric values (of supported types)
//    are within a given basis points (bps) difference. Basis points represent hundredths
//    of a percent, enabling high precision comparisons.
//
// 2. `FixedToleranceChecker`: A custom `Checker` implementation for use with the `check`
//    testing framework. It validates whether two numeric values are within a fixed basis
//    points tolerance, e.g., 1% (100 bps) or 10 bps.
//
// 3. Predefined Checkers:
//    - `AlmostEqualTo1PercentChecker`: Checks if values differ by no more than 1% (100 bps).
//    - `AlmostEqualTo10BpsChecker`: Checks if values differ by no more than 0.1% (10 bps).
//
// These utilities enable precise validation of numeric values in tests, particularly
// useful for financial or blockchain-related computations involving small tolerances.

// ConvertToUint converts various numeric types to cosmos.Uint.
func ConvertToUint(value interface{}) (cosmos.Uint, error) {
	switch v := value.(type) {
	case cosmos.Uint:
		return v, nil
	case int:
		return cosmos.NewUint(uint64(v)), nil
	case uint:
		return cosmos.NewUint(uint64(v)), nil
	case int64:
		return cosmos.NewUint(uint64(v)), nil
	case uint64:
		return cosmos.NewUint(v), nil
	case string:
		return cosmos.NewUintFromString(v), nil
	default:
		return cosmos.Uint{}, fmt.Errorf("unsupported type: %T", value)
	}
}

// AlmostEqualBps determines if two values are approximately equal within a given basis points tolerance.
func AlmostEqualBps(a, b interface{}, bps uint64) (bool, uint64, cosmos.Uint, error) {
	valA, errA := ConvertToUint(a)
	valB, errB := ConvertToUint(b)

	if errA != nil || errB != nil {
		return false, 0, cosmos.ZeroUint(), fmt.Errorf("invalid input types: %v, %v", errA, errB)
	}

	var valBig, diff cosmos.Uint
	if valB.GT(valA) {
		diff = valB.Sub(valA)
		valBig = valB
	} else {
		diff = valA.Sub(valB)
		valBig = valA
	}
	tolerance := valA.MulUint64(bps).QuoUint64(10000) // bps tolerance in Uint
	obtainedBps := uint64(0)
	if !valBig.IsZero() {
		obtainedBps = diff.MulUint64(10000).Quo(valBig).BigInt().Uint64()
	}
	return diff.LTE(tolerance), obtainedBps, diff, nil
}

// FixedToleranceChecker defines a custom checker for approximate equality with fixed bps tolerance
type FixedToleranceChecker struct {
	ExpectedBps uint64
	logger      log.Logger
}

//nolint:typecheck
func (c *FixedToleranceChecker) Info() *CheckerInfo {
	return &CheckerInfo{
		Name:   "AlmostEqualBpsChecker",
		Params: []string{"obtained", "expected"},
	}
}

// Check performs the approximate equality check
func (c *FixedToleranceChecker) Check(params []interface{}, names []string) (result bool, err string) {
	if len(params) != 2 {
		return false, "requires exactly two parameters"
	}
	obtained, expected := params[0], params[1]

	equal, obtainedBps, diff, e := AlmostEqualBps(obtained, expected, c.ExpectedBps)
	if e != nil {
		c.logger.Error("FixedToleranceChecker", "error", e.Error())
		return false, e.Error()
	}
	if !equal {
		c.logger.Error("\033[30;FixedToleranceChecker - not equal", "expected", expected, "obtained", obtained, "expected bps", c.ExpectedBps, "obtained bps", obtainedBps, "diff", diff)
	}
	return equal, ""
}

func (c *FixedToleranceChecker) SetLogger(logger log.Logger) {
	c.logger = logger
}

type AddressChecker struct {
	logger log.Logger
}

//nolint:typecheck
func (c *AddressChecker) Info() *CheckerInfo {
	return &CheckerInfo{
		Name:   "AddressChecker",
		Params: []string{"obtained", "expected"},
	}
}

// Check performs the approximate equality check.
func (c *AddressChecker) Check(params []interface{}, names []string) (result bool, err string) {
	if len(params) != 2 {
		return false, "requires exactly two parameters"
	}
	valO, valE := params[0], params[1]
	var obtained, expected common.Address
	var ok bool
	if expected, ok = valE.(common.Address); !ok {
		c.logger.Error("AddressChecker", "error", fmt.Sprintf("unsupported type: %T", valE))
		return false, fmt.Sprintf("unsupported type: %T", valE)
	} else if obtained, ok = valO.(common.Address); !ok {
		c.logger.Error("AddressChecker", "error", fmt.Sprintf("unsupported type: %T", valO))
		return false, fmt.Sprintf("unsupported type: %T", valO)
	}
	equal := obtained.Equals(expected)
	if !equal {
		c.logger.Error("\033[43mAddressChecker - not equal", "expected", expected, "obtained", obtained)
	}
	return equal, ""
}

func (c *AddressChecker) SetLogger(logger log.Logger) {
	c.logger = logger
}

// Predefined Checkers for common tolerances.
var (
	EqualTo1Percent = &FixedToleranceChecker{ExpectedBps: 100} // 1%
	EqualTo10Bps    = &FixedToleranceChecker{ExpectedBps: 10}  // 0.1%
	EqualUint       = &FixedToleranceChecker{ExpectedBps: 0}   // no tolerance
	EqualAddress    = &AddressChecker{}                        // compare common.Address-es
)

func (s *MultipleAffiliatesSuite) SetUpTest(c *C) {
	s.ctx, s.mgr = setupManagerForTest(c)

	// Message collector to filter and store error log messages, such as "fail to process inbound tx", eg:
	//     E[2024-08-14|13:47:40.562] fail to process inbound tx                   error="swap destination address is not the same chain as the target asset: unknown request" txhash=79CDAC3BCC22DCFEBCAB5CD50DEF60D83788D3AD0F07BD5ABFAF9B1FF7D350EE
	// if you want show logs in stdout change the last parameter to true
	s.errLogs = NewErrorLogCollector("", "E", []string{"\033[43m", "\033[0m", "ignore me", "ignore this error"})
	// Create a logger that uses the custom message collector
	logger := log.NewTMLogger(s.errLogs)
	s.ctx = s.ctx.WithLogger(logger)

	EqualTo1Percent.SetLogger(logger)
	EqualTo10Bps.SetLogger(logger)
	EqualUint.SetLogger(logger)
	EqualAddress.SetLogger(logger)

	gasFee := s.mgr.gasMgr.GetFee(s.ctx, common.BASEChain, common.BaseAsset())
	gas := common.Gas{common.NewCoin(common.BaseNative, gasFee)}
	from := GetRandomBaseAddress()
	to := GetRandomTHORAddress()

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
		common.NewCoin(common.BaseNative, cosmos.NewUint(10000000*common.One)),
		common.NewCoin(common.BTCAsset, cosmos.NewUint(10000000*common.One)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(10000000*common.One)),
		common.NewCoin(common.ETHAsset, cosmos.NewUint(10000000*common.One)),
	}

	s.keeper = &TestAffiliateKeeper{
		nas:       NodeAccounts{s.node},
		voter:     NewObservedTxVoter(s.tx.ID, make(ObservedTxs, 0)),
		voterTxID: s.tx.ID,
		vault:     vault,
		asgard:    asgard,
	}
	s.keeper.Keeper = s.mgr.K
	s.mgr.K = s.keeper
	s.version = GetCurrentVersion()

	// mint synths
	/*
		coin := common.NewCoin(common.BTCAsset.GetSyntheticAsset(), cosmos.NewUint(1000000*common.One))
		c.Assert(s.mgr.Keeper().MintToModule(s.ctx, ModuleName, coin), IsNil)
		coin = common.NewCoin(common.BTCAsset.GetSyntheticAsset(), cosmos.NewUint(15000*common.One))
		c.Assert(s.mgr.Keeper().SendFromModuleToModule(s.ctx, ModuleName, AsgardName, common.NewCoins(coin)), IsNil)
		coin = common.NewCoin(common.BNBAsset.GetSyntheticAsset(), cosmos.NewUint(1000000*common.One))
		c.Assert(s.mgr.Keeper().MintToModule(s.ctx, ModuleName, coin), IsNil)
		coin = common.NewCoin(common.BNBAsset.GetSyntheticAsset(), cosmos.NewUint(15000*common.One))
		c.Assert(s.mgr.Keeper().SendFromModuleToModule(s.ctx, ModuleName, AsgardName, common.NewCoins(coin)), IsNil)
	*/

	networkFee := NewNetworkFee(common.THORChain, 1, 2000000)
	c.Assert(s.mgr.Keeper().SaveNetworkFee(s.ctx, common.THORChain, networkFee), IsNil)
	networkFee = NewNetworkFee(common.BTCChain, 70, 500)
	c.Assert(s.mgr.Keeper().SaveNetworkFee(s.ctx, common.BTCChain, networkFee), IsNil)
	networkFee = NewNetworkFee(common.ETHChain, 80000, 300)
	c.Assert(s.mgr.Keeper().SaveNetworkFee(s.ctx, common.ETHChain, networkFee), IsNil)

	txOutStore, err := GetTxOutStore(s.version, s.mgr.K, s.mgr.eventMgr, s.mgr.gasMgr)
	c.Assert(err, IsNil)
	s.mockTxOutStore = MockWithdrawTxOutStoreForMultiAff{
		TxOutStore: txOutStore,
		asgard:     asgard,
	}
	s.mgr.txOutStore = &s.mockTxOutStore
	s.mockTxOutStore.full = true

	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceCacao = cosmos.NewUint(10000000000 * common.One)
	pool.BalanceAsset = cosmos.NewUint(10000000 * common.One)
	pool.LPUnits = cosmos.NewUint(10000000000 * common.One)
	pool.Status = PoolAvailable
	c.Assert(s.mgr.Keeper().SetPool(s.ctx, pool), IsNil)
	pool.Asset = common.BTCAsset
	pool.BalanceCacao = cosmos.NewUint(2000000000 * common.One)
	pool.BalanceAsset = cosmos.NewUint(10000 * common.One)
	pool.LPUnits = cosmos.NewUint(2000000000 * common.One)
	c.Assert(s.mgr.Keeper().SetPool(s.ctx, pool), IsNil)
	pool.Asset = common.RUNEAsset
	pool.BalanceCacao = cosmos.NewUint(100000000 * common.One)
	pool.BalanceAsset = cosmos.NewUint(10000000 * common.One)
	pool.LPUnits = cosmos.NewUint(100000000 * common.One)
	c.Assert(s.mgr.Keeper().SetPool(s.ctx, pool), IsNil)
	pool.Asset = common.ETHAsset
	pool.BalanceCacao = cosmos.NewUint(5000000000 * common.One)
	pool.BalanceAsset = cosmos.NewUint(10000000 * common.One)
	pool.LPUnits = cosmos.NewUint(5000000000 * common.One)
	c.Assert(s.mgr.Keeper().SetPool(s.ctx, pool), IsNil)

	s.queue = newSwapQueueVCUR(s.mgr.Keeper())
	s.handler = NewObservedTxInHandler(s.mgr)
	s._depositHandler = NewDepositHandler(s.mgr)

	// prepare & fund deposit signer
	s.signer = GetRandomBech32Addr()
	funds, err := common.NewCoin(common.BaseNative, cosmos.NewUint(3000_000*common.One)).Native()
	c.Assert(err, IsNil)
	err = s.mgr.Keeper().AddCoins(s.ctx, s.signer, cosmos.NewCoins(funds))
	c.Assert(err, IsNil)
	/*
		funds, err = common.NewCoin(common.BTCAsset.GetSyntheticAsset(), cosmos.NewUint(10000*common.One)).Native()
		c.Assert(err, IsNil)
		err = s.mgr.Keeper().AddCoins(s.ctx, s.signer, cosmos.NewCoins(funds))
		c.Assert(err, IsNil)
		funds, err = common.NewCoin(common.BNBAsset.GetSyntheticAsset(), cosmos.NewUint(100000*common.One)).Native()
		c.Assert(err, IsNil)
		err = s.mgr.Keeper().AddCoins(s.ctx, s.signer, cosmos.NewCoins(funds))
		c.Assert(err, IsNil)
	*/

	s.affColAddress, err = s.mgr.Keeper().GetModuleAddress(AffiliateCollectorName)
	c.Assert(err, IsNil)
	s.accAffCol, err = s.affColAddress.AccAddress()
	c.Assert(err, IsNil)

	// register affiliate mayaname for simple aff fee testing
	s.addrSimple, err = s.setMayaname("simple", simpleShareBps, "", EmptyBps, "THOR.RUNE")
	c.Assert(err, IsNil)
	s.accSimple, err = s.addrSimple.AccAddress()
	c.Assert(err, IsNil)

	s.subafAddress = common.Address("tmaya13wrmhnh2qe98rjse30pl7u6jxszjjwl4fd6gwn")
	s.perc100 = cosmos.NewUint(10000)

	// Map of affiliates with the affiliate Bps and aff fee result Bps of the whole swap amount
	// The 'aff' affiliate operates with a 1.5% fee. From this, subaff_1 receives 30% and subaff_2 gets 20%.
	// Additionally, subaff_1 passes 40% of its share to its sub-sub-affiliate, subsubaff_1.
	// Resulting distribution: aff: 0.75%, subaff_1: 0.27% (60% of 30% of 1.5%), subaff_2: 0.3% (20% of 1.5%), subsubaff_1: 0.18% (40% of 30% of 1.5%).
	s.affMns = []affMn{
		{"aff", "", affShareBps, cosmos.NewUint(75), "THOR.RUNE"},
		{"subaff_1", "aff", cosmos.NewUint(30_00), cosmos.NewUint(27), "BTC.BTC"},
		{"subaff_2", "aff", cosmos.NewUint(10_00), cosmos.NewUint(15), ""},
		{s.subafAddress.String(), "aff", cosmos.NewUint(10_00), cosmos.NewUint(15), ""},
		{"subsubaff_1", "subaff_1", cosmos.NewUint(40_00), cosmos.NewUint(18), ""},
	}
	s.affMnMap = make(map[common.Address]int)

	for i, d := range s.affMns {
		bps := EmptyBps
		if d.parent == "" {
			bps = d.bps
		}
		var addr common.Address
		if d.name != s.subafAddress.String() {
			addr, err = s.setMayaname(d.name, bps, "", EmptyBps, d.pref)
		} else {
			addr = common.Address(d.name)
		}
		c.Assert(err, IsNil)
		s.affMnMap[addr] = i
		if d.parent != "" {
			parent, err := s.mgr.Keeper().GetMAYAName(s.ctx, d.parent)
			c.Assert(err, IsNil)
			c.Assert(parent.Name == "", Equals, false)
			prevSubAffCnt := len(parent.GetSubaffiliates())
			_, err = s.setMayaname(d.parent, EmptyBps, d.name, d.bps, "")
			c.Assert(err, IsNil)
			parent, err = s.mgr.Keeper().GetMAYAName(s.ctx, d.parent)
			c.Assert(err, IsNil)
			subAffs := parent.GetSubaffiliates()
			c.Assert(subAffs, HasLen, prevSubAffCnt+1)
			c.Assert(subAffs[prevSubAffCnt].Name, Equals, d.name)
			c.Assert(subAffs[prevSubAffCnt].Bps, EqualUint, d.bps)
		}
	}
}

func (s *MultipleAffiliatesSuite) handleDeposit(msg *MsgDeposit) error {
	_, err := s._depositHandler.handle(s.ctx, *msg)
	if err == nil {
		if str, cnt := s.errLogs.GetCollectedString(true); cnt > 0 {
			err = fmt.Errorf("%s", str)
		}
	} else {
		fmt.Print(err)
	}
	return err
}

func (s *MultipleAffiliatesSuite) processTx() error {
	s.keeper.voter.FinalisedHeight = 0
	height := int64(12)
	s.tx.ID = GetRandomTxHash()
	obTx := NewObservedTx(s.tx, height, s.keeper.vault.PubKey, 15)
	txs := ObservedTxs{obTx}
	txs[0].BlockHeight = 15
	msg := NewMsgObservedTxIn(txs, s.node.NodeAddress)
	s.ctx = s.ctx.WithBlockHeight(16)
	_, err := s.handler.handle(s.ctx, *msg)
	if err == nil {
		if str, cnt := s.errLogs.GetCollectedString(true); cnt > 0 {
			err = fmt.Errorf("%s", str)
		}
	}
	return err
}

func (s *MultipleAffiliatesSuite) fetchQueue() (swapItems, error) {
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

func (s *MultipleAffiliatesSuite) queueEndBlock() error {
	err := s.queue.EndBlock(s.ctx, s.mgr)
	if err == nil {
		if str, cnt := s.errLogs.GetCollectedString(true); cnt > 0 {
			err = fmt.Errorf("%s", str)
		}
	} else {
		fmt.Print(err)
	}
	return err
}

func (s *MultipleAffiliatesSuite) setMayanameSimple(name string, affBps cosmos.Uint, subaff string, subaffBps cosmos.Uint, pref string) error {
	_, err := s.setMayaname(name, affBps, subaff, subaffBps, pref)
	return err
}

func (s *MultipleAffiliatesSuite) setMayaname(name string, affBps cosmos.Uint, subaff string, subaffBps cosmos.Uint, pref string) (common.Address, error) {
	// MAYAName memo format 1:  ~:name:chain:address:?owner:?preferredAsset:?expiry:?affbps:?subaff:?subaffbps
	exists := s.mgr.Keeper().MAYANameExists(s.ctx, name)
	// prepare the mayaname owner
	var owner cosmos.AccAddress
	var alias common.Address
	var mn types.MAYAName
	var err error

	if exists {
		// get the existing owner
		mn, err = s.mgr.Keeper().GetMAYAName(s.ctx, name)
		if err != nil {
			return alias, err
		}
		owner = mn.Owner
	} else {
		// get new owner
		owner = GetRandomBech32Addr()
		funds, err := common.NewCoin(common.BaseNative, cosmos.NewUint(3000_000*common.One)).Native()
		if err != nil {
			return alias, err
		}
		if err := s.mgr.Keeper().AddCoins(s.ctx, owner, cosmos.NewCoins(funds)); err != nil {
			return alias, err
		}
	}
	affBpsStr := ""
	if !affBps.Equal(EmptyBps) {
		affBpsStr = affBps.String()
	}
	subaffBpsStr := ""
	if !subaffBps.Equal(EmptyBps) {
		subaffBpsStr = subaffBps.String()
	}

	var msg *types.MsgDeposit
	regAmt := cosmos.ZeroUint()
	if !exists {
		regAmt = cosmos.NewUint(3_000 * common.One)
	}
	coins := common.Coins{common.NewCoin(common.BaseNative, regAmt)}
	if !exists {
		alias = GetRandomBaseAddress()
		// register mayaname
		msg = NewMsgDeposit(coins, fmt.Sprintf("~:%s:MAYA:%s::::%s:%s:%s", name, alias, affBpsStr, subaff, subaffBpsStr), owner)
	} else if subaff != "" || affBpsStr != "" || subaffBpsStr != "" {
		alias = mn.GetAlias(common.BASEChain)
		// update mayaname
		msg = NewMsgDeposit(coins, fmt.Sprintf("~:%s:MAYA:%s::::%s:%s:%s", name, alias, affBpsStr, subaff, subaffBpsStr), owner)
	}
	if msg != nil {
		err := s.handleDeposit(msg)
		if err != nil {
			return alias, err
		}
	}
	prefWithoutAddress := strings.HasPrefix(pref, "-")
	if prefWithoutAddress {
		pref = pref[1:]
	}
	if len(pref) != 0 {
		prefChain := strings.Split(pref, ".")[0]
		var prefAddr common.Address
		if prefWithoutAddress {
			// we don't want to set alias for preferred asset, use base chain
			prefChain = "MAYA"
			prefAddr = GetRandomBaseAddress()
		} else {
			// set the preferred asset and the corresponding alias
			switch pref {
			case "THOR.RUNE":
				prefAddr = s.getRandomTTHORAddress()
			case "MAYA.CACAO":
				prefAddr = GetRandomBaseAddress()
			case "BNB.BNB":
				prefAddr = GetRandomBNBAddress()
			case "BTC.BTC":
				prefAddr = GetRandomBTCAddress()
			default:
				return alias, fmt.Errorf("can't use %s as preferred asset in this test", pref)
			}
		}
		// for setting preferred asset we have to use long format
		memo := fmt.Sprintf("~:%s:%s:%s::%s", name, prefChain, prefAddr, pref)
		msg = NewMsgDeposit(coins, memo, owner)
		err := s.handleDeposit(msg)
		if err != nil {
			return alias, err
		}
	}
	errLogs := s.errLogs.GetCollected(true)
	if len(errLogs) > 0 {
		return alias, fmt.Errorf("%d error log occurred: %s", len(errLogs), errLogs[0])
	}
	return alias, nil
}

func (s *MultipleAffiliatesSuite) getRandomTTHORAddress() common.Address {
	name := common.RandHexString(10)
	str, _ := common.ConvertAndEncode("tthor", crypto.AddressHash([]byte(name)))
	base, _ := common.NewAddress(str, s.version)
	return base
}

func (s *MultipleAffiliatesSuite) getRandomTMAYAAddress() common.Address {
	name := common.RandHexString(10)
	str, _ := common.ConvertAndEncode("tmaya", crypto.AddressHash([]byte(name)))
	base, _ := common.NewAddress(str, s.version)
	return base
}

func (s *MultipleAffiliatesSuite) getRandomChainAddress(chain common.Chain) common.Address {
	switch chain {
	case common.BNBChain:
		return GetRandomBNBAddress()
	case common.ETHChain:
		return GetRandomETHAddress()
	case common.BTCChain:
		return GetRandomBTCAddress()
	case common.DASHChain:
		return GetRandomDASHAddress()
	case common.BASEChain:
		return s.getRandomTMAYAAddress()
	case common.THORChain:
		return s.getRandomTTHORAddress()
	default:
		return common.NoAddress
	}
}

func (s *MultipleAffiliatesSuite) clearAffiliateCollector(name string) {
	if !s.mgr.Keeper().MAYANameExists(s.ctx, name) {
		return
	}
	mn, err := s.mgr.Keeper().GetMAYAName(s.ctx, name)
	if err != nil {
		return
	}
	affCol, err := s.mgr.Keeper().GetAffiliateCollector(s.ctx, mn.Owner)
	if err != nil {
		return
	}
	affCol.CacaoAmount = cosmos.ZeroUint()
	s.mgr.Keeper().SetAffiliateCollector(s.ctx, affCol)
}

func (s *MultipleAffiliatesSuite) clearAllAffiliateCollectors() {
	for _, mn := range s.affMns {
		s.clearAffiliateCollector(mn.name)
	}
	s.clearAffiliateCollector("simple")
}

func (s *MultipleAffiliatesSuite) assetValueInCacao(c *C, amount cosmos.Uint, asset common.Asset) cosmos.Uint {
	if asset.IsNativeBase() {
		return amount
	}
	p, err := s.mgr.Keeper().GetPool(s.ctx, asset.GetLayer1Asset())
	c.Assert(err, IsNil)
	return p.AssetValueInRune(amount)
}

func (s *MultipleAffiliatesSuite) cacaoValueInAsset(c *C, amount cosmos.Uint, asset common.Asset) cosmos.Uint {
	if asset.IsNativeBase() {
		return amount
	}
	p, err := s.mgr.Keeper().GetPool(s.ctx, asset.GetLayer1Asset())
	c.Assert(err, IsNil)
	return p.RuneValueInAsset(amount)
}

func (s *MultipleAffiliatesSuite) testSwap(c *C, swapAmtUint64 uint64, fromAsset common.Asset, fromAddress common.Address, toAsset common.Asset, toAddress common.Address, affBps cosmos.Uint) {
	swapAmt := cosmos.NewUint(swapAmtUint64 * common.One)
	// save the previous native balance for the case when preferred asset is not set
	simple, err := s.mgr.Keeper().GetMAYAName(s.ctx, "simple")
	c.Assert(err, IsNil)
	acc := s.accSimple
	// if no preferred asset, fee goes to the maya alias
	if simple.PreferredAsset.IsEmpty() && simple.GetAlias(common.BASEChain).IsEmpty() {
		acc = simple.Owner
	}
	prevBalance := cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, acc).AmountOf("cacao").Uint64())

	// sanity check that affiliate collector is empty
	affCol, err := s.mgr.Keeper().GetAffiliateCollector(s.ctx, simple.Owner)
	c.Assert(err, IsNil)
	c.Assert(affCol.CacaoAmount, EqualUint, cosmos.ZeroUint())

	// construct the swap memo with 'simple' affiliate
	affBpsStr := ""
	if affBps.Equal(EmptyBps) {
		affBps = simple.GetAffiliateBps()
	} else {
		affBpsStr = fmt.Sprintf(":%s", affBps)
	}
	memo := fmt.Sprintf("=:%s:%s::simple%s", toAsset, toAddress, affBpsStr)
	coins := common.Coins{common.NewCoin(fromAsset, swapAmt)}
	// do the swap
	if fromAsset.IsNative() {
		err = s.handleDeposit(NewMsgDeposit(coins, memo, s.signer))
	} else {
		gasFee := s.mgr.gasMgr.GetFee(s.ctx, fromAsset.Chain, fromAsset)
		s.tx.Gas = common.Gas{common.NewCoin(fromAsset, gasFee)}
		s.tx.FromAddress = fromAddress
		s.tx.Coins = coins
		s.tx.Memo = memo

		err = s.processTx()
	}
	c.Assert(err, IsNil)
	// verify the swap queue for the main swap
	swaps, err := s.fetchQueue()
	c.Assert(err, IsNil)
	c.Assert(swaps, HasLen, 1) // main swap

	// affiliate fees in main swap target asset
	affFeeInSrc := common.GetSafeShare(affBps, s.perc100, swapAmt)
	var affFeeInCacao cosmos.Uint
	if fromAsset.IsNativeBase() {
		affFeeInCacao = affFeeInSrc
	} else {
		var p Pool
		p, err = s.mgr.Keeper().GetPool(s.ctx, fromAsset.GetLayer1Asset())
		c.Assert(err, IsNil)
		affFeeInCacao = p.AssetValueInRune(affFeeInSrc)
	}

	c.Assert(swaps[0].msg.Tx.Coins[0].Amount, EqualUint, swapAmt)
	err = s.queueEndBlock()
	c.Assert(err, IsNil)
	// verify the main swap tx out
	c.Assert(s.mockTxOutStore.tois, HasLen, 1) // main swap
	c.Assert(s.mockTxOutStore.tois[0].ToAddress, EqualAddress, toAddress)
	// if target is cacao, both main swap and fee (not swapped as it was deducted from the swap result in cacao) are already in target wallets
	// otherwise the fee swap has to be processed
	if !toAsset.IsNativeBase() {
		// verify swap queue for the affiliate fee swap
		swaps, err = s.fetchQueue()
		c.Assert(err, IsNil)
		c.Assert(swaps, HasLen, 1) // affiliate fee swap
		err = s.queueEndBlock()
		c.Assert(err, IsNil)
		// affiliate fee swaps out tx
		c.Assert(s.mockTxOutStore.tois, HasLen, 1)
	}
	if simple.PreferredAsset.IsEmpty() { // no preferred asset -> fee goes to maya alias (or owner)
		// if target is not native, verify the fee swap tx out
		if !toAsset.IsNative() {
			dest := common.Address(acc.String())
			c.Assert(s.mockTxOutStore.tois[0].ToAddress, EqualAddress, dest)
		}
		// preferred asset not set, fee goes to the native address
		curBalance := cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, acc).AmountOf("cacao").Uint64())
		expectedBalance := prevBalance.Add(affFeeInCacao)
		c.Assert(curBalance, EqualTo10Bps, expectedBalance.Sub(cacaoFee)) // prevBalance + affiliate fee - outbound tx fee
	} else { // preferred asset set
		if !toAsset.IsNative() {
			// verify fee goes to affiliate collector
			c.Assert(s.mockTxOutStore.tois[0].ToAddress, EqualAddress, s.affColAddress)
		}
		// preferred asset set, fee goes to collector or is swapped to preferred asset if threshold is met
		if affFeeInCacao.LT(getPreferredAssetSwapThreshold(s.ctx, s.mgr, simple.PreferredAsset)) {
			// if small amount swapped
			// verify affiliate fee amount in affiliate collector
			var affCol types.AffiliateFeeCollector
			affCol, err = s.mgr.Keeper().GetAffiliateCollector(s.ctx, simple.Owner)
			c.Assert(err, IsNil)
			expectedInAffCol := affFeeInCacao
			if !toAsset.Equals(common.BaseNative) { // if main swap target asset is cacao, the affiliate fee is deducted from the swap outbound
				expectedInAffCol = expectedInAffCol.Sub(cacaoFee)
			}
			c.Assert(affCol.CacaoAmount, EqualTo10Bps, expectedInAffCol) // affiliate fee - the outbound tx fee
		} else {
			// if big amount swapped, preferred asset swap expected (from main swap target asset to preferred asset)
			// this is the swap of the affiliate collector cacao content to the preferred asset
			swaps, err = s.fetchQueue()
			c.Assert(err, IsNil)
			c.Assert(swaps, HasLen, 1)
			c.Assert(swaps[0].msg.Tx.Memo, Equals, "MAYA-PREFERRED-ASSET-simple")
			ofCacao := cosmos.ZeroUint()
			// if the main swap target is not cacao, we need to deduct the cacao fee for the affiliate fee swap from the main swap target asset into the affiliate collector
			if !toAsset.Equals(common.BaseNative) {
				ofCacao = cacaoFee
			}
			c.Assert(swaps[0].msg.Tx.Coins[0].Amount, EqualTo10Bps, affFeeInCacao.Sub(ofCacao)) // affiliate fee - swap is always in cacao from the AffCol
			// process preferred asset swap in queue
			c.Assert(s.queueEndBlock(), IsNil)
			// affiliate fee swap in tx out
			c.Assert(s.mockTxOutStore.tois, HasLen, 1)
			c.Assert(s.mockTxOutStore.tois[0].ToAddress, EqualAddress, simple.GetAlias(simple.PreferredAsset.Chain))
			s.mockTxOutStore.tois = nil
			err = s.queueEndBlock()
			c.Assert(err, IsNil)
		}
	}
	s.clear()
}

func (s *MultipleAffiliatesSuite) clear() {
	s.errLogs.Clear()
	s.clearAllAffiliateCollectors()
	s.mockTxOutStore.tois = nil
	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
}

func FundAccountWith(c *C, ctx cosmos.Context, k keeper.Keeper, addr cosmos.AccAddress, coin common.Coin) {
	err := k.MintToModule(ctx, ModuleName, coin)
	c.Assert(err, IsNil)
	err = k.SendFromModuleToAccount(ctx, ModuleName, addr, common.NewCoins(coin))
	c.Assert(err, IsNil)
}

func (s *MultipleAffiliatesSuite) TestSimpleAffiliatesViaMsgDeposit(c *C) {
	thorDest := s.getRandomTTHORAddress()
	// normal affiliate test, small amount, fee goes to collector
	s.testSwap(c, 1500, common.BaseNative, common.NoAddress, common.RUNEAsset, thorDest, EmptyBps)
	// affiliate test with overwritten affiliate bps, small amount, fee goes to collector
	s.testSwap(c, 1500, common.BaseNative, common.NoAddress, common.RUNEAsset, thorDest, cosmos.NewUint(170))
	// preferred asset payment test, bigger amount, should trigger preferred asset payment
	s.testSwap(c, 300000, common.BaseNative, common.NoAddress, common.RUNEAsset, thorDest, EmptyBps)
	// to synth swap with affiliate, no preferred asset - payment to maya alias
	s.testSwap(c, 1000000, common.BaseNative, common.NoAddress, common.BTCAsset.GetSyntheticAsset(), s.getRandomTMAYAAddress(), EmptyBps)
	// from synth swap with affiliate
	FundAccountWith(c, s.ctx, s.mgr.Keeper(), s.signer, common.NewCoin(common.BTCAsset.GetSyntheticAsset(), cosmos.NewUint(2*common.One)))
	s.testSwap(c, 1, common.BTCAsset.GetSyntheticAsset(), common.NoAddress, common.RUNEAsset, thorDest, EmptyBps)
	// remove preferred asset
	simple, err := s.mgr.Keeper().GetMAYAName(s.ctx, "simple")
	c.Assert(err, IsNil)
	simple.PreferredAsset = common.EmptyAsset
	s.mgr.Keeper().SetMAYAName(s.ctx, simple)
	// from synth swap with affiliate, no preferred asset - payment to maya alias
	s.testSwap(c, 1, common.BTCAsset.GetSyntheticAsset(), common.NoAddress, common.RUNEAsset, thorDest, EmptyBps)
	// to synth swap with affiliate, no preferred asset - payment to maya alias
	s.testSwap(c, 1000000, common.BaseNative, common.NoAddress, common.BTCAsset.GetSyntheticAsset(), s.getRandomTMAYAAddress(), EmptyBps)
}

func (s *MultipleAffiliatesSuite) TestSimpleAffiliatesViaSwap(c *C) {
	bnbSource := GetRandomBNBAddress()

	// *** target is MAYA ***
	mayaDest := s.getRandomTMAYAAddress()
	// normal affiliate test, small amount, fee goes to collector
	s.testSwap(c, 1, common.BNBAsset, bnbSource, common.BaseNative, mayaDest, EmptyBps)
	// affiliate test with overwritten affiliate bps, small amount, fee goes to collector
	s.testSwap(c, 1, common.BNBAsset, bnbSource, common.BaseNative, mayaDest, cosmos.NewUint(170))
	// preferred asset payment test, bigger amount, should trigger preferred asset payment
	s.testSwap(c, 10, common.BNBAsset, bnbSource, common.BaseNative, mayaDest, EmptyBps)
	// to synth swap with affiliate
	s.testSwap(c, 10, common.BNBAsset, bnbSource, common.BTCAsset.GetSyntheticAsset(), mayaDest, EmptyBps)
	// remove preferred asset
	simple, err := s.mgr.Keeper().GetMAYAName(s.ctx, "simple")
	c.Assert(err, IsNil)
	simple.PreferredAsset = common.EmptyAsset
	s.mgr.Keeper().SetMAYAName(s.ctx, simple)
	// to synth swap with affiliate without preferred asset - payment to maya alias
	s.testSwap(c, 3, common.BNBAsset, bnbSource, common.BTCAsset.GetSyntheticAsset(), mayaDest, EmptyBps)
	// set preferred asset back to THOR.RUNE
	s.addrSimple, err = s.setMayaname("simple", EmptyBps, "", EmptyBps, "THOR.RUNE")
	c.Assert(err, IsNil)

	// *** target is THOR ***
	thorDest := s.getRandomTTHORAddress()
	// normal affiliate test, small amount, fee goes to collector
	s.testSwap(c, 2, common.BNBAsset, bnbSource, common.RUNEAsset, thorDest, EmptyBps)
	// affiliate test with overwritten affiliate bps, small amount, fee goes to collector
	s.testSwap(c, 2, common.BNBAsset, bnbSource, common.RUNEAsset, thorDest, cosmos.NewUint(170))
	// preferred asset payment test, bigger amount, should trigger preferred asset payment
	s.testSwap(c, 10, common.BNBAsset, bnbSource, common.RUNEAsset, thorDest, EmptyBps)
	// remove preferred asset
	simple, err = s.mgr.Keeper().GetMAYAName(s.ctx, "simple")
	c.Assert(err, IsNil)
	simple.PreferredAsset = common.EmptyAsset
	s.mgr.Keeper().SetMAYAName(s.ctx, simple)
	// preferred asset not set, fee goes directly to maya alias/owner
	s.testSwap(c, 10, common.BNBAsset, bnbSource, common.RUNEAsset, thorDest, EmptyBps)
}

type expectedResult struct {
	name  string // for debug / log purposes - the name of the mayaname
	ticks uint64 // in hundredths of a basis points (ten-thousandths of a percent, 1 tick = 0.0001%)
	addr  common.Address
}

func (s *MultipleAffiliatesSuite) tryMultiSwap(c *C, swapAmt cosmos.Uint, fromAssetString, memo string, expextedRes []expectedResult) {
	// search for the bee && count expected non-zero fees
	bee_ := common.NoAddress
	feeTxCount := -1 // don't count the main swap
	withZeroBps := make(map[string]bool)
	for _, res := range expextedRes {
		if res.name == "bee" {
			bee_ = res.addr
		}
		if res.ticks != 0 {
			feeTxCount++
		} else {
			withZeroBps[res.name] = true
		}
	}

	// save the current balances of maya addresses
	prevBalances := make([]cosmos.Uint, len(expextedRes))
	for i, res := range expextedRes {
		if res.addr.IsChain(common.BASEChain, s.version) && !res.addr.Equals(s.affColAddress) {
			acc, _ := res.addr.AccAddress()
			prevBalances[i] = cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, acc).AmountOf(common.BaseAsset().Native()).Uint64())
		}
	}
	prevAffColBalance := cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, s.accAffCol).AmountOf(common.BaseAsset().Native()).Uint64())

	// get the fromAsset
	fromAsset, err := common.NewAsset(fromAssetString)
	c.Assert(err, IsNil)
	swapAmtInCacao := s.assetValueInCacao(c, swapAmt, fromAsset)

	if !fromAsset.IsNativeBase() {
		// swap from asset
		gasFee := s.mgr.gasMgr.GetFee(s.ctx, fromAsset.Chain, fromAsset)
		s.tx.Gas = common.Gas{common.NewCoin(fromAsset, gasFee)}
		s.tx.FromAddress = s.getRandomChainAddress(fromAsset.Chain)
		s.tx.Coins = common.Coins{common.NewCoin(fromAsset, swapAmt)}
		s.tx.Memo = memo
		c.Assert(s.processTx(), IsNil)
	} else {
		// swap from CACAO (MsgDeposit)
		swapCoins := common.Coins{common.NewCoin(common.BaseNative, swapAmt)}
		msg := NewMsgDeposit(swapCoins, memo, s.signer)
		c.Assert(s.handleDeposit(msg), IsNil)
	}

	// verify swap queue - only the main swap should be queued
	swaps, err := s.fetchQueue()
	c.Assert(err, IsNil)
	c.Assert(swaps, HasLen, 1)
	// verify the main swap destination and amount
	c.Assert(swaps[0].msg.Destination, EqualAddress, expextedRes[0].addr)
	c.Assert(swaps[0].msg.Tx.Coins[0].Amount, EqualUint, swapAmt)
	// process the main swap; it should txout the main swap and queue the affiliate swaps
	c.Assert(s.queueEndBlock(), IsNil)
	c.Assert(s.mockTxOutStore.tois, HasLen, 1)
	s.mockTxOutStore.tois = nil
	// check tx queue - only the affiliate swaps should be queued
	swaps, err = s.fetchQueue()
	c.Assert(err, IsNil)
	c.Assert(swaps, HasLen, feeTxCount)
	// process the affiliate swaps
	c.Assert(s.queueEndBlock(), IsNil)
	c.Assert(s.mockTxOutStore.tois, HasLen, feeTxCount)
	s.mockTxOutStore.tois = nil
	// verify fees' destinations and amounts
	feesAddedToffCol := cosmos.ZeroUint()
	for i, res := range expextedRes {
		if res.ticks == 0 {
			continue
		}
		if res.addr.IsChain(common.BASEChain, s.version) {
			feeInCacao := swapAmtInCacao.MulUint64(res.ticks).QuoUint64(1000000)
			if res.addr.Equals(s.affColAddress) {
				s.ctx.Logger().Info(fmt.Sprintf("affiliate %s goes to collector", res.name))
				feesAddedToffCol = feesAddedToffCol.Add(feeInCacao)
			} else {
				acc, _ := res.addr.AccAddress()
				curBal := cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, acc).AmountOf(common.BaseAsset().Native()).Uint64())
				expBal := prevBalances[i].Add(feeInCacao).Sub(cacaoFee)
				s.ctx.Logger().Info(fmt.Sprintf("affiliate %s goes to maya alias, fee %s <-should be similar to-> diff %s, prevBal: %s, curBal: %s", res.name, feeInCacao.Sub(cacaoFee), curBal.Sub(prevBalances[i]), prevBalances[i], curBal))
				c.Assert(curBal, EqualTo10Bps, expBal)
			}
		}
	}
	// verify the affiliate collector amount
	curAffColBalance := cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, s.accAffCol).AmountOf(common.BaseAsset().Native()).Uint64())
	// 2x Sub(cacaoFee) as two swaps went to affiliate collector
	c.Assert(curAffColBalance, EqualTo10Bps, prevAffColBalance.Add(feesAddedToffCol).Sub(cacaoFee).Sub(cacaoFee))

	// verify events
	receivedEventFor := map[string]bool{"fox": false, "bat": false, "frog": false, "cat": false, bee_.String(): false, "owl": false, "pig": false, "ant": false}
	for i, e := range s.ctx.EventManager().Events() {
		if strings.EqualFold(e.Type, types.AffiliateFeeEventType) {
			s.ctx.Logger().Debug("affilaiet fee event", "i", i)
			var ev types.EventAffiliateFee
			for j, a := range e.Attributes {
				s.ctx.Logger().Debug("affilaiet fee event attribute", "i", i, "j", j, "key", string(a.Key), "value", string(a.Value))
				switch string(a.Key) {
				case "tx_id":
					ev.TxID, err = common.NewTxID(string(a.Value))
					c.Assert(err, IsNil)
				case "memo":
					ev.Memo = string(a.Value)
				case "mayaname":
					ev.Mayaname = string(a.Value)
				case "cacao_address":
					ev.CacaoAddress = common.Address(a.Value)
				case "asset":
					ev.Asset, err = common.NewAsset(string(a.Value))
					c.Assert(err, IsNil)
				case "gross_amount":
					ev.GrossAmount = cosmos.NewUintFromString(string(a.Value))
				case "fee_bps_tick":
					ev.FeeBpsTick = cosmos.NewUintFromString(string(a.Value)).BigInt().Uint64()
				case "fee_amount":
					ev.FeeAmount = cosmos.NewUintFromString(string(a.Value))
				case "sub_fee_bps":
					ev.SubFeeBps = cosmos.NewUintFromString(string(a.Value)).BigInt().Uint64()
				case "parent":
					ev.Parent = string(a.Value)
				}
			}
			c.Assert(ev.Asset.IsEmpty(), Equals, false)
			swapAmtInAsset := s.cacaoValueInAsset(c, swapAmtInCacao, ev.Asset)
			if ev.Mayaname != "" {
				receivedEventFor[ev.Mayaname] = true
			} else {
				receivedEventFor[ev.CacaoAddress.String()] = true
			}
			if !ev.CacaoAddress.IsEmpty() {
				for _, res := range expextedRes {
					// skip the first address which is the main swap eth destination
					if res.addr.IsChain(common.BASEChain, s.version) && res.addr.Equals(ev.CacaoAddress) {
						c.Assert(ev.GrossAmount, EqualTo10Bps, swapAmtInAsset)
						c.Assert(ev.CacaoAddress, EqualAddress, res.addr)
						c.Assert(ev.FeeBpsTick, Equals, res.ticks)
						exp := swapAmtInAsset.MulUint64(ev.FeeBpsTick).QuoUint64(1000000)
						c.Assert(ev.FeeAmount, EqualTo10Bps, exp)
						if ev.Mayaname == "cat" || ev.Mayaname == "pig" || ev.Mayaname == "ant" || ev.CacaoAddress.Equals(bee_) {
							c.Assert(ev.Parent, Equals, "")
						} else {
							c.Assert(ev.Parent, Not(Equals), "")
						}
						c.Assert(ev.SubFeeBps, Not(Equals), uint64(0))
					}
				}
			} else if ev.Mayaname == "fox" || ev.Mayaname == "cat" {
				// fox and cat go to affiliate collector
				ticks := expextedRes[1].ticks // fox is 5
				if ev.Mayaname == "cat" {
					ticks = expextedRes[4].ticks // cat is 35
				}
				c.Assert(ev.FeeBpsTick, Equals, ticks)
				exp := swapAmtInAsset.MulUint64(ev.FeeBpsTick).QuoUint64(1000000)
				c.Assert(ev.FeeAmount, EqualTo10Bps, exp)
			}
		}
	}
	// verify we received events for all affiliate mayanames
	for name, received := range receivedEventFor {
		if withZeroBps[name] {
			continue
		}
		c.Assert(received, Equals, true, Commentf("Event not received for affiliate mayaname %s", name))
	}
	s.clear()
}

func (s *MultipleAffiliatesSuite) TestAllAffiliateFeeSwaps(c *C) {
	cat := GetRandomBaseAddress()
	fox := GetRandomBaseAddress()
	pig := GetRandomBaseAddress()
	frog := GetRandomBaseAddress()
	bat := GetRandomBaseAddress()
	bee_ := GetRandomBaseAddress() // not a mayaname, just an address
	owl := GetRandomBaseAddress()
	ant := GetRandomBaseAddress()

	catEth := GetRandomETHAddress()
	foxEth := GetRandomETHAddress()
	owlEth := GetRandomETHAddress()
	destEth := GetRandomETHAddress()

	antBtc := GetRandomBTCAddress()

	catAcc, err := cat.AccAddress()
	c.Assert(err, IsNil)
	foxAcc, err := fox.AccAddress()
	c.Assert(err, IsNil)
	pigAcc, err := pig.AccAddress()
	c.Assert(err, IsNil)
	frogAcc, err := frog.AccAddress()
	c.Assert(err, IsNil)
	batAcc, err := bat.AccAddress()
	c.Assert(err, IsNil)
	// beeAcc, err := bee.AccAddress()
	c.Assert(err, IsNil)
	owlAcc, err := owl.AccAddress()
	c.Assert(err, IsNil)
	antAcc, err := ant.AccAddress()
	c.Assert(err, IsNil)

	// prepare mayanames
	mn := MAYAName{Name: "bat", Owner: batAcc, Aliases: []MAYANameAlias{{Chain: common.BASEChain, Address: bat}}, ExpireBlockHeight: 9999}
	s.mgr.Keeper().SetMAYAName(s.ctx, mn)
	mn = MAYAName{Name: "owl", Owner: owlAcc /* no preferred asset */, Aliases: []MAYANameAlias{{Chain: common.ETHChain, Address: owlEth}}, ExpireBlockHeight: 9999}
	s.mgr.Keeper().SetMAYAName(s.ctx, mn)
	mn = MAYAName{Name: "fox", Owner: foxAcc, PreferredAsset: common.ETHAsset, Aliases: []MAYANameAlias{{Chain: common.BASEChain, Address: fox}, {Chain: common.ETHChain, Address: foxEth}}, ExpireBlockHeight: 9999}
	s.mgr.Keeper().SetMAYAName(s.ctx, mn)
	antBps := cosmos.NewUint(50)
	mn = MAYAName{Name: "ant", Owner: antAcc /* no preferred asset */, AffiliateBps: &antBps, Aliases: []MAYANameAlias{{Chain: common.BTCChain, Address: antBtc}}, ExpireBlockHeight: 9999}
	s.mgr.Keeper().SetMAYAName(s.ctx, mn)
	pigBps := cosmos.NewUint(0)
	mn = MAYAName{Name: "pig", Owner: pigAcc, AffiliateBps: &pigBps, Subaffiliates: []MAYANameSubaffiliate{{Name: "owl", Bps: cosmos.NewUint(5000)}}, Aliases: []MAYANameAlias{{Chain: common.BASEChain, Address: pig}}, ExpireBlockHeight: 9999}
	s.mgr.Keeper().SetMAYAName(s.ctx, mn)
	frogBps := cosmos.NewUint(100)
	mn = MAYAName{Name: "frog", Owner: frogAcc, AffiliateBps: &frogBps, Subaffiliates: []MAYANameSubaffiliate{{Name: "bat", Bps: cosmos.NewUint(4000)}}, Aliases: []MAYANameAlias{{Chain: common.BASEChain, Address: frog}}, ExpireBlockHeight: 9999}
	s.mgr.Keeper().SetMAYAName(s.ctx, mn)
	catBps := cosmos.NewUint(50)
	mn = MAYAName{Name: "cat", Owner: catAcc, PreferredAsset: common.ETHAsset, AffiliateBps: &catBps, Subaffiliates: []MAYANameSubaffiliate{{Name: "fox", Bps: cosmos.NewUint(1000)}, {Name: "frog", Bps: cosmos.NewUint(2000)}}, Aliases: []MAYANameAlias{{Chain: common.BASEChain, Address: cat}, {Chain: common.ETHChain, Address: catEth}}, ExpireBlockHeight: 9999}
	s.mgr.Keeper().SetMAYAName(s.ctx, mn)

	// cat (0.5%, eth)
	//      - fox  (10%, eth)
	//      - frog (20%)
	//           - bat (40%)
	// pig (-%)
	//      - owl (50%, no maya alias only eth)
	// ant (0.5%, no maya alias only btc)

	expextedRes := []expectedResult{
		{"main", 981000, destEth},         // 10000 - 190
		{"fox-acc", 500, s.affColAddress}, // fox 5  (10% of 50)
		{"bat", 400, bat},                 // bat 4  (40% of 20% of 50)
		{"frog", 600, frog},               // frog 6 (60% of 20% of 50)
		{"cat-ac", 3500, s.affColAddress}, // cat 35 (70% of 50 = (50 - 30%))
		{"bee", 4000, bee_},               // bee 40
		{"owl", 2500, owl},                // owl 25 (50% of 50)
		{"pig", 2500, pig},                // pig 25 (50% of 50)
		{"ant", 5000, ant},                // ant 50
	}
	swapAmt := cosmos.NewUint(1000_0000000000)                                    // swap 1000 cacao
	memo := fmt.Sprintf("=:ETH.ETH:%s:0/1:cat/%s/pig/ant:/40/50/", destEth, bee_) // pig has no defbps
	s.tryMultiSwap(c, swapAmt, "cacao", memo, expextedRes)

	swapAmtOne := cosmos.NewUint(1 * common.One) // swap 1 btc
	memo = fmt.Sprintf("=:ETH.ETH:%s:0/1:cat/%s/pig/ant:/40/50/", destEth, bee_)
	s.tryMultiSwap(c, swapAmtOne, "btc.btc", memo, expextedRes)

	expextedRes = []expectedResult{
		{"main", 988840, destEth},        // 10000 - 110 = 9890 (20 + 20 + 20 + 50)
		{"fox-ac", 200, s.affColAddress}, // fox 2  (10% of 20)
		{"bat", 160, bat},                // bat 1.6  (40% of 20% of 50)
		{"frog", 240, frog},              // frog 2.4 (60% of 20% of 20)
		{"cat", 1400, s.affColAddress},   // cat 14  (70% of 20)
		{"bee", 2000, bee_},              // bee 20
		{"owl", 1000, owl},               // owl 10 (50% of 20)
		{"pig", 1000, pig},               // pig 10 (50% of 20)
		{"ant", 5000, ant},               // ant 50
	}
	// cat - 20, bee - 20, pig - 20, ant - 50, pig has no defbps
	memo = fmt.Sprintf("=:ETH.ETH:%s:0/1:cat/%s/pig/ant:20/20/20/", destEth, bee_)
	s.tryMultiSwap(c, swapAmtOne, "btc.btc", memo, expextedRes)

	expextedRes = []expectedResult{
		{"main", 991000, destEth},      //
		{"fox", 200, s.affColAddress},  // fox 5
		{"bat", 160, bat},              // bat 2.4
		{"frog", 240, frog},            // frog 6
		{"cat", 1400, s.affColAddress}, // cat 35
		{"bee", 2000, bee_},            // bee 50
		{"owl", 0, owl},                // owl 0
		{"pig", 0, pig},                // pig 0
		{"ant", 5000, ant},             // ant 50
	}
	// cat - 20, bee - 20, pig (& owl) - 0, ant - 50, pig has no defbps
	memo = fmt.Sprintf("=:ETH.ETH:%s:0/1:cat/%s/pig/ant:20", destEth, bee_)
	s.tryMultiSwap(c, swapAmtOne, "btc.btc", memo, expextedRes)

	// unhappy paths
	// affiliates & bps count mismatch
	s.tx.Memo = fmt.Sprintf("=:ETH.ETH:%s:0/1:cat/%s/pig/ant:/40/50", destEth, bee_)
	err = s.processTx()
	c.Assert(err, NotNil)
	c.Assert(strings.Contains(err.Error(), "affiliate mayanames and affiliate fee bps count mismatch"), Equals, true)
	s.errLogs.Clear()

	// affiliate bps too high
	s.tx.Memo = fmt.Sprintf("=:ETH.ETH:%s:0/1:cat/%s/pig/ant:/500/500/", destEth, bee_)
	err = s.processTx()
	c.Assert(err, NotNil)
	c.Assert(strings.Contains(err.Error(), "affiliate fee basis points must not exceed"), Equals, true)
	s.errLogs.Clear()

	// invalid address / mayaname
	s.tx.Memo = fmt.Sprintf("=:ETH.ETH:%s:0/1:cat/%s/pig/ant:/40/50/", destEth, "123")
	err = s.processTx()
	c.Assert(err, NotNil)
	c.Assert(strings.Contains(err.Error(), "invalid affiliate mayaname or address: 123 is not recognizable"), Equals, true)
	s.errLogs.Clear()

	// bee (=explicit addr) doesn't have aff bps
	s.tx.Memo = fmt.Sprintf("=:ETH.ETH:%s:0/1:cat/%s/pig/ant:10///", destEth, bee_)
	err = s.processTx()
	c.Assert(strings.Contains(err.Error(), fmt.Sprintf("cannot parse '%s' as a MAYAName while empty affiliate basis points provided at index", bee_)), Equals, true)
	c.Assert(err, NotNil)
	s.errLogs.Clear()
}

func (s *MultipleAffiliatesSuite) TestMultipleAffWithZeroBps(c *C) {
	ttds := s.getRandomTTHORAddress().String()
	// prepare the affiliate mayaname
	affBps := cosmos.NewUint(150)
	c.Assert(s.setMayanameSimple("hello", cosmos.NewUint(150), "", EmptyBps, ""), IsNil)
	hello, err := s.mgr.Keeper().GetMAYAName(s.ctx, "hello")
	c.Assert(err, IsNil)
	c.Assert(hello.Name, Equals, "hello")
	c.Assert(hello.GetAffiliateBps(), EqualUint, 150)
	c.Assert(s.setMayanameSimple("helloZero", cosmos.ZeroUint(), "", EmptyBps, ""), IsNil)
	helloZero, err := s.mgr.Keeper().GetMAYAName(s.ctx, "helloZero")
	c.Assert(err, IsNil)
	c.Assert(helloZero.Name, Equals, "helloZero")
	c.Assert(helloZero.GetAffiliateBps(), EqualUint, 0)
	helloBaseAliasAcc, _ := hello.GetAlias(common.BASEChain).AccAddress()
	helloBalanceBefore := cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, helloBaseAliasAcc).AmountOf(common.BaseAsset().Native()).Uint64())
	helloZeroBaseAliasAcc, _ := helloZero.GetAlias(common.BASEChain).AccAddress()
	helloZeroBalanceBefore := cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, helloZeroBaseAliasAcc).AmountOf(common.BaseAsset().Native()).Uint64())

	// swap with two affiliates, first with 150 bps second with zero bps
	swapAmt := cosmos.NewUint(100000 * common.One) // swap 10 cacao
	coins := common.Coins{common.NewCoin(common.BaseNative, swapAmt)}
	memo := fmt.Sprintf("=:THOR.RUNE:%s::hello/helloZero", ttds)
	msg := NewMsgDeposit(coins, memo, s.signer)
	c.Assert(s.handleDeposit(msg), IsNil)
	c.Assert(s.queueEndBlock(), IsNil)
	c.Assert(s.queueEndBlock(), IsNil)
	affFees := common.GetSafeShare(affBps, s.perc100, swapAmt)
	helloBalanceAfter := cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, helloBaseAliasAcc).AmountOf(common.BaseAsset().Native()).Uint64())
	c.Assert(helloBalanceBefore.Add(affFees).Sub(cacaoFee), EqualTo10Bps, helloBalanceAfter)
	helloBalanceBefore = helloBalanceAfter
	helloZeroBalanceAfter := cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, helloZeroBaseAliasAcc).AmountOf(common.BaseAsset().Native()).Uint64())
	c.Assert(helloZeroBalanceBefore, EqualUint, helloZeroBalanceAfter)

	// the same swap as above but with overridden bps (50)
	// only one bps provided so it will be used for first affiliates, second uses default mayaname bps, which is zero
	coins = common.Coins{common.NewCoin(common.BaseNative, swapAmt)}
	affBps = cosmos.NewUint(50)
	memo = fmt.Sprintf("=:THOR.RUNE:%s::hello/helloZero:50", ttds)
	msg = NewMsgDeposit(coins, memo, s.signer)
	c.Assert(s.handleDeposit(msg), IsNil)
	c.Assert(s.queueEndBlock(), IsNil)
	c.Assert(s.queueEndBlock(), IsNil)
	affFees = common.GetSafeShare(affBps, s.perc100, swapAmt)
	helloBalanceAfter = cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, helloBaseAliasAcc).AmountOf(common.BaseAsset().Native()).Uint64())
	c.Assert(helloBalanceBefore.Add(affFees).Sub(cacaoFee), EqualTo10Bps, helloBalanceAfter)
	helloZeroBalanceAfter = cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, helloZeroBaseAliasAcc).AmountOf(common.BaseAsset().Native()).Uint64())
	c.Assert(helloZeroBalanceBefore, EqualUint, helloZeroBalanceAfter)
	helloBalanceBefore = helloBalanceAfter

	// the same swap as above but with overridden bps (50) only for first affiliate
	// first affiliate uses 50, second uses default mayaname bps, which is zero
	coins = common.Coins{common.NewCoin(common.BaseNative, swapAmt)}
	affBps = cosmos.NewUint(50)
	memo = fmt.Sprintf("=:THOR.RUNE:%s::hello/helloZero:50/", ttds)
	msg = NewMsgDeposit(coins, memo, s.signer)
	c.Assert(s.handleDeposit(msg), IsNil)
	c.Assert(s.queueEndBlock(), IsNil)
	c.Assert(s.queueEndBlock(), IsNil)
	affFees = common.GetSafeShare(affBps, s.perc100, swapAmt)
	helloBalanceAfter = cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, helloBaseAliasAcc).AmountOf(common.BaseAsset().Native()).Uint64())
	c.Assert(helloBalanceBefore.Add(affFees).Sub(cacaoFee), EqualTo10Bps, helloBalanceAfter)
	helloZeroBalanceAfter = cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, helloZeroBaseAliasAcc).AmountOf(common.BaseAsset().Native()).Uint64())
	c.Assert(helloZeroBalanceBefore, EqualUint, helloZeroBalanceAfter)
	helloBalanceBefore = helloBalanceAfter

	// the same swap as above but with overridden bps (50) explicitly for both affiliate
	// first affiliate uses 50, second uses 50
	coins = common.Coins{common.NewCoin(common.BaseNative, swapAmt)}
	memo = fmt.Sprintf("=:THOR.RUNE:%s::hello/helloZero:50/50", ttds)
	msg = NewMsgDeposit(coins, memo, s.signer)
	c.Assert(s.handleDeposit(msg), IsNil)
	c.Assert(s.queueEndBlock(), IsNil)
	c.Assert(s.queueEndBlock(), IsNil)
	affFees = common.GetSafeShare(affBps, s.perc100, swapAmt)
	helloBalanceAfter = cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, helloBaseAliasAcc).AmountOf(common.BaseAsset().Native()).Uint64())
	c.Assert(helloBalanceBefore.Add(affFees).Sub(cacaoFee), EqualTo10Bps, helloBalanceAfter)
	helloZeroBalanceAfter = cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, helloZeroBaseAliasAcc).AmountOf(common.BaseAsset().Native()).Uint64())
	c.Assert(helloZeroBalanceBefore.Add(affFees).Sub(cacaoFee), EqualTo10Bps, helloZeroBalanceAfter)
}

func (s *MultipleAffiliatesSuite) TestMultipleAffiliatesErrors(c *C) {
	// Try to register two affiliate, which would result in the total fees exceeding 100%
	c.Assert(s.setMayanameSimple("alfa", EmptyBps, "", EmptyBps, ""), IsNil)
	c.Assert(s.setMayanameSimple("aff1", EmptyBps, "", EmptyBps, ""), IsNil)
	c.Assert(s.setMayanameSimple("aff2", EmptyBps, "", EmptyBps, ""), IsNil)
	c.Assert(s.setMayanameSimple("alfa", EmptyBps, "aff1", cosmos.NewUint(50_00), ""), IsNil)
	c.Assert(s.setMayanameSimple("alfa", EmptyBps, "aff2", cosmos.NewUint(60_00), ""), NotNil)
	// set self as subaffiliate - should fail
	c.Assert(s.setMayanameSimple("hello", EmptyBps, "", EmptyBps, ""), IsNil)
	c.Assert(s.setMayanameSimple("hello", EmptyBps, "hello", cosmos.NewUint(500), ""), NotNil)
	// set non-existent mayaname as subaffiliate - should fail
	c.Assert(s.setMayanameSimple("hello", EmptyBps, "nobody", cosmos.NewUint(500), ""), NotNil)
	// set out of range affiliate bps - should fail
	c.Assert(s.setMayanameSimple("aff1", EmptyBps, "", cosmos.NewUint(100_01), ""), NotNil)
	// set invalid preferred asset - should fail
	c.Assert(s.setMayanameSimple("aff1", EmptyBps, "", EmptyBps, "BAD.THOR"), NotNil)
}

func (s *MultipleAffiliatesSuite) TestMultipleAffiliatesMemos(c *C) {
	// register mayaname with default bps 500
	c.Assert(s.setMayanameSimple("hello", cosmos.NewUint(500), "", EmptyBps, ""), IsNil)
	hello, err := s.mgr.Keeper().GetMAYAName(s.ctx, "hello")
	c.Assert(err, IsNil)
	c.Assert(hello.Name, Equals, "hello")
	c.Assert(hello.GetAffiliateBps(), EqualUint, 500)
	// set mayaname default bps 0
	c.Assert(s.setMayanameSimple("hello", cosmos.ZeroUint(), "", EmptyBps, ""), IsNil)
	hello, err = s.mgr.Keeper().GetMAYAName(s.ctx, "hello")
	c.Assert(err, IsNil)
	c.Assert(hello.Name, Equals, "hello")
	c.Assert(hello.GetAffiliateBps().IsZero(), Equals, true)
	// set default bps back t 500
	c.Assert(s.setMayanameSimple("hello", cosmos.NewUint(500), "", EmptyBps, ""), IsNil)
	hello, err = s.mgr.Keeper().GetMAYAName(s.ctx, "hello")
	c.Assert(err, IsNil)
	c.Assert(hello.Name, Equals, "hello")
	c.Assert(hello.GetAffiliateBps(), EqualUint, 500)
	// register mayaname aff1
	c.Assert(s.setMayanameSimple("aff1", EmptyBps, "", EmptyBps, ""), IsNil)
	// register mayaname aff2
	c.Assert(s.setMayanameSimple("aff2", EmptyBps, "", EmptyBps, ""), IsNil)
	// set aff1 as subaff of hello with 2000 bps share
	c.Assert(s.setMayanameSimple("hello", EmptyBps, "aff1", cosmos.NewUint(2000), ""), IsNil)
	hello, err = s.mgr.Keeper().GetMAYAName(s.ctx, "hello")
	c.Assert(err, IsNil)
	c.Assert(hello.Subaffiliates, HasLen, 1)
	c.Assert(hello.Subaffiliates[0].Name, Equals, "aff1")
	c.Assert(hello.Subaffiliates[0].Bps, EqualUint, 2000)
	// set aff2 as subaff of hello with 3000 bps share
	c.Assert(s.setMayanameSimple("hello", EmptyBps, "aff2", cosmos.NewUint(3000), ""), IsNil)
	hello, err = s.mgr.Keeper().GetMAYAName(s.ctx, "hello")
	c.Assert(err, IsNil)
	c.Assert(hello.Subaffiliates, HasLen, 2)
	c.Assert(hello.Subaffiliates[0].Name, Equals, "aff1")
	c.Assert(hello.Subaffiliates[0].Bps, EqualUint, 2000)
	c.Assert(hello.Subaffiliates[1].Name, Equals, "aff2")
	c.Assert(hello.Subaffiliates[1].Bps, EqualUint, 3000)
	// remove aff1 as subaff of hello
	c.Assert(s.setMayanameSimple("hello", EmptyBps, "aff1", cosmos.ZeroUint(), ""), IsNil)
	hello, err = s.mgr.Keeper().GetMAYAName(s.ctx, "hello")
	c.Assert(err, IsNil)
	c.Assert(hello.Subaffiliates, HasLen, 1)
	c.Assert(hello.Subaffiliates[0].Name, Equals, "aff2")
	c.Assert(hello.Subaffiliates[0].Bps, EqualUint, 3000)
	// set aff1 as subaff of hello again with 4000 bps share
	c.Assert(s.setMayanameSimple("hello", EmptyBps, "aff1", cosmos.NewUint(4000), ""), IsNil)
	hello, err = s.mgr.Keeper().GetMAYAName(s.ctx, "hello")
	c.Assert(err, IsNil)
	c.Assert(hello.Subaffiliates, HasLen, 2)
	c.Assert(hello.Subaffiliates[0].Name, Equals, "aff2")
	c.Assert(hello.Subaffiliates[0].Bps, EqualUint, 3000)
	c.Assert(hello.Subaffiliates[1].Name, Equals, "aff1")
	c.Assert(hello.Subaffiliates[1].Bps, EqualUint, 4000)
}

func (s *MultipleAffiliatesSuite) TestMultipleAffiliatesWithStreamingSwaps(c *C) {
	destination := s.getRandomTTHORAddress().String()
	// swap memo format: =:ASSET:DESTADDR:0/1/10:AFFILIATE_ADDR_OR_MAYANAME:FEE:DEXAggregatorAddr:FinalTokenAddr:MinAmountOut
	// prepare the affiliate mayaname
	c.Assert(s.setMayanameSimple("hello", cosmos.NewUint(150), "", EmptyBps, ""), IsNil)
	hello, err := s.mgr.Keeper().GetMAYAName(s.ctx, "hello")
	c.Assert(err, IsNil)
	hello.PreferredAsset = common.EmptyAsset
	s.mgr.Keeper().SetMAYAName(s.ctx, hello)
	addrHelloOnMaya := hello.GetAlias(common.BASEChain)
	accHelloOnMaya, _ := addrHelloOnMaya.AccAddress()
	prevBalance := cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, accHelloOnMaya).AmountOf(common.BaseAsset().Native()).Uint64())
	swapAmt := cosmos.NewUint(1000000 * common.One)
	coins := common.Coins{
		common.NewCoin(common.BaseNative, swapAmt),
	}
	coins[0].Amount = swapAmt
	memo := "=:THOR.RUNE:" + destination + ":0/1/10:hello"
	msg := NewMsgDeposit(coins, memo, s.signer)
	err = s.handleDeposit(msg)
	c.Assert(err, IsNil)
	affCol, err := s.mgr.Keeper().GetAffiliateCollector(s.ctx, hello.Owner)
	c.Assert(err, IsNil)
	c.Check(affCol.CacaoAmount.IsZero(), Equals, true)
	c.Assert(s.queueEndBlock(), IsNil) // process the main swap
	c.Assert(s.queueEndBlock(), IsNil) // process the affiliate fee swap
	affFees := common.GetSafeShare(cosmos.NewUint(150), s.perc100, swapAmt)
	// verify that affiliate fees have been sent to the preferred asset alias on native chain
	curBalance := cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, accHelloOnMaya).AmountOf(common.BaseAsset().Native()).Uint64())
	c.Assert(prevBalance.Add(affFees).Sub(cacaoFee), EqualTo10Bps, curBalance)
}

func (s *MultipleAffiliatesSuite) TestSubaffiliateBps(c *C) {
	coins := common.Coins{common.NewCoin(common.BaseNative, cosmos.NewUint(3_000*common.One))}
	cat := GetRandomBaseAddress()
	fox := GetRandomBaseAddress()
	bee := GetRandomBaseAddress()

	// prepare mayanames
	// memo format:  ~:name:chain:address:?owner:?preferredAsset:?expiry:?affbps:?subaff:?subaffbps
	msg := NewMsgDeposit(coins, fmt.Sprintf("~:cat:MAYA:%s", cat), s.signer)
	c.Assert(s.handleDeposit(msg), IsNil)
	msg = NewMsgDeposit(coins, fmt.Sprintf("~:fox:MAYA:%s", fox), s.signer)
	c.Assert(s.handleDeposit(msg), IsNil)

	// test common subaffiliate bps
	// set address and fox with common sub-affiliate bps 2000
	msg = NewMsgDeposit(coins, fmt.Sprintf("~:cat:::::::%s/fox:2000", bee), s.signer)
	c.Assert(s.handleDeposit(msg), IsNil)
	catMn, err := s.mgr.Keeper().GetMAYAName(s.ctx, "cat")
	c.Assert(err, IsNil)
	c.Assert(catMn.Subaffiliates, HasLen, 2)
	c.Assert(catMn.Subaffiliates[0].Name, Equals, bee.String())
	c.Assert(catMn.Subaffiliates[0].Bps.BigInt().Uint64(), Equals, uint64(2000))
	c.Assert(catMn.Subaffiliates[1].Name, Equals, "fox")
	c.Assert(catMn.Subaffiliates[1].Bps.BigInt().Uint64(), Equals, uint64(2000))

	// test change and add subaffiliate at the same time
	owl := GetRandomBaseAddress()
	msg = NewMsgDeposit(coins, fmt.Sprintf("~:cat:::::::%s/fox:7000/10", owl), s.signer)
	c.Assert(s.handleDeposit(msg), IsNil)
	catMn, err = s.mgr.Keeper().GetMAYAName(s.ctx, "cat")
	c.Assert(err, IsNil)
	c.Assert(catMn.Subaffiliates, HasLen, 3)
	c.Assert(catMn.Subaffiliates[0].Name, Equals, bee.String())
	c.Assert(catMn.Subaffiliates[0].Bps.BigInt().Uint64(), Equals, uint64(2000))
	c.Assert(catMn.Subaffiliates[1].Name, Equals, "fox")
	c.Assert(catMn.Subaffiliates[1].Bps.BigInt().Uint64(), Equals, uint64(10))
	c.Assert(catMn.Subaffiliates[2].Name, Equals, owl.String())
	c.Assert(catMn.Subaffiliates[2].Bps.BigInt().Uint64(), Equals, uint64(7000))

	// test first empty subaff bps
	msg = NewMsgDeposit(coins, fmt.Sprintf("~:cat:::::::%s/fox:/10", owl), s.signer)
	c.Assert(s.handleDeposit(msg), NotNil)

	// test empty subaff bps
	msg = NewMsgDeposit(coins, fmt.Sprintf("~:cat:::::::%s/fox:/", owl), s.signer)
	c.Assert(s.handleDeposit(msg), NotNil)

	// test only one subaff bps with zero value - should delete both subaffiliates
	msg = NewMsgDeposit(coins, fmt.Sprintf("~:cat:::::::%s/fox:0", owl), s.signer)
	c.Assert(s.handleDeposit(msg), IsNil)
	catMn, err = s.mgr.Keeper().GetMAYAName(s.ctx, "cat")
	c.Assert(err, IsNil)
	c.Assert(catMn.Subaffiliates, HasLen, 1)
	c.Assert(catMn.Subaffiliates[0].Name, Equals, bee.String())
	c.Assert(catMn.Subaffiliates[0].Bps.BigInt().Uint64(), Equals, uint64(2000))

	// test first empty subaff bps
	msg = NewMsgDeposit(coins, fmt.Sprintf("~:cat:::::::%s/fox:/10", owl), s.signer)
	c.Assert(s.handleDeposit(msg), NotNil)
}

/*
func (s *MultipleAffiliatesSuite) TestAffCollPayout(c *C) {
	cat := GetRandomBaseAddress()
	catAcc, err := cat.AccAddress()
	c.Assert(err, IsNil)
	ttds := s.getRandomTTHORAddress().String()
	// fund the signer / sender & affiliate collector module
	FundAccount(c, s.ctx, s.mgr.Keeper(), s.signer, 500000000)
	FundModule(c, s.ctx, s.mgr.Keeper(), AffiliateCollectorName, 500000000)
	prevAC := cosmos.ZeroUint()

	fundAffCol := func(amt uint64) {
		affCol, errAffCol := s.mgr.Keeper().GetAffiliateCollector(s.ctx, catAcc)
		c.Assert(errAffCol, IsNil)
		affCol.CacaoAmount = cosmos.NewUint(amt * common.One)
		s.mgr.Keeper().SetAffiliateCollector(s.ctx, affCol)
		prevAC = affCol.CacaoAmount
	}

	// prepare the affiliate mayaname
	c.Assert(err, IsNil)
	catBps := cosmos.NewUint(150)
	catMn := MAYAName{Name: "cat", Owner: catAcc, AffiliateBps: &catBps, PreferredAsset: common.BaseNative, Aliases: []MAYANameAlias{{Chain: common.BASEChain, Address: cat}}, ExpireBlockHeight: 9999}
	s.mgr.Keeper().SetMAYAName(s.ctx, catMn)
	before := cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, catAcc).AmountOf(common.BaseAsset().Native()).Uint64())

	swapAmt := cosmos.NewUint(500000 * common.One)
	coins := common.Coins{common.NewCoin(common.BaseNative, swapAmt)}
	memo := fmt.Sprintf("=:THOR.RUNE:%s::cat", ttds)

	// preferred asset is set to MAYA.CACO
	// ONE cacao (100 * common.One) is in affiliate collector
	// swap -> affiliate fee + previous collector amount should be sent to the maya alias / owner
	fundAffCol(100) // one cacao
	msg := NewMsgDeposit(coins, memo, s.signer)
	c.Assert(s.handleDeposit(msg), IsNil)
	c.Assert(s.queueEndBlock(), IsNil) // main swap processing
	c.Assert(s.queueEndBlock(), IsNil) // affiliate fee swap processing
	affFees := common.GetSafeShare(catBps, s.perc100, swapAmt)
	// cat should have the current affiliate fee plus the affiliate collector previous amount
	after := cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, catAcc).AmountOf(common.BaseAsset().Native()).Uint64())
	ofRuneInCacao := s.mgr.GasMgr().GetFee(s.ctx, common.THORChain, common.BaseNative)
	c.Assert(after, EqualTo10Bps, before.Add(affFees).Sub(cacaoFee).Sub(ofRuneInCacao).Add(prevAC))
	// affiliate collector should be empty
	affCol, err := s.mgr.Keeper().GetAffiliateCollector(s.ctx, catAcc)
	c.Assert(err, IsNil)
	c.Assert(affCol.CacaoAmount, EqualUint, 0)
	s.clear()

	// preferred asset is set to MAYA.CACO
	// ONE cacao (100 * common.One) is in affiliate collector
	// change preferred asset -> previous collector amount should be sent
	// to the new preferred asset only if threshold is reached

	// set 0.5 cacao into affiliate collector again
	fundAffCol(50) // 0.5 cacao as BNB has low threshold
	// save cat's balance
	before = cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, catAcc).AmountOf(common.BaseAsset().Native()).Uint64())
	// change preferred asset
	c.Assert(s.setMayanameSimple("cat", EmptyBps, "", EmptyBps, "BNB.BNB"), IsNil) // change to BNB.BNB as preferred
	// no change in cat
	after = cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, catAcc).AmountOf(common.BaseAsset().Native()).Uint64())
	c.Assert(after, EqualUint, before.Sub(cacaoFee)) // cacao network fee for the mayaname preferred asset change
	affCol, err = s.mgr.Keeper().GetAffiliateCollector(s.ctx, catAcc)
	c.Assert(err, IsNil)
	// no change in affiliate collector of cat
	c.Assert(affCol.CacaoAmount, EqualUint, prevAC)
	prevAC = affCol.CacaoAmount

	// the same as above but with big affiliate collector amount, should be sent aout to the new preferred asset
	// preferred asset is set to MAYA.CACO
	// 1000 cacao (1000_00 * common.One) is in affiliate collector
	// change preferred asset -> previous collector amount should be sent
	// to the new preferred asset only of threshold is reached

	// set 1000 cacao into affiliate collector again
	fundAffCol(1000_00) // 1000 cacao
	// save cat's balance
	before = cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, catAcc).AmountOf(common.BaseAsset().Native()).Uint64())
	// reset the preferred asset to MAYA.CACAO
	catMn = MAYAName{Name: "cat", Owner: catAcc, AffiliateBps: &catBps, PreferredAsset: common.BaseNative, Aliases: []MAYANameAlias{{Chain: common.BASEChain, Address: cat}}, ExpireBlockHeight: 9999}
	s.mgr.Keeper().SetMAYAName(s.ctx, catMn)
	// change preferred asset again to BNB
	c.Assert(s.setMayanameSimple("cat", EmptyBps, "", EmptyBps, "BNB.BNB"), IsNil) // BNB.BNB as preferred
	// process the preferred asset swap
	c.Assert(s.queueEndBlock(), IsNil)
	// no change in cat
	after = cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, catAcc).AmountOf(common.BaseAsset().Native()).Uint64())
	c.Assert(before.Sub(cacaoFee), EqualUint, after)
	// affiliate collector should be empty
	affCol, err = s.mgr.Keeper().GetAffiliateCollector(s.ctx, catAcc)
	c.Assert(err, IsNil)
	c.Assert(affCol.CacaoAmount, EqualUint, 0)
	s.clear()
	prevAC = affCol.CacaoAmount

	// preferred asset is set to BNB.BNB (or any other)
	// ONE cacao (100 * common.One) is in affiliate collector
	// remove preferred asset -> previous collector amount should be sent
	// to the maya alias / owner
	fundAffCol(100) // 1 cacao
	// save cat's balance
	before = cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, catAcc).AmountOf(common.BaseAsset().Native()).Uint64())
	// set MAYA.CACAO as preferred = remove preferred asset
	c.Assert(s.setMayanameSimple("cat", EmptyBps, "", EmptyBps, "MAYA.CACAO"), IsNil)
	// process the preferred asset swap
	c.Assert(s.queueEndBlock(), IsNil)
	// cat should have the affiliate collector previous amount
	after = cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, catAcc).AmountOf(common.BaseAsset().Native()).Uint64())
	c.Assert(after, EqualUint, before.Add(prevAC).Sub(cacaoFee))
	// affiliate collector should be empty
	affCol, err = s.mgr.Keeper().GetAffiliateCollector(s.ctx, catAcc)
	c.Assert(err, IsNil)
	c.Assert(affCol.CacaoAmount, EqualUint, 0)
}
*/

func (s *MultipleAffiliatesSuite) TestAffCollPayout(c *C) {
	regfee := common.Coins{common.NewCoin(common.BaseNative, cosmos.NewUint(110000000000))}
	// create the cat mayaname
	cat := GetRandomBaseAddress()
	msg := NewMsgDeposit(regfee, fmt.Sprintf("~:cat:MAYA:%s", cat), s.signer)
	c.Assert(s.handleDeposit(msg), IsNil)
	// set eth as preferred asset
	msg = NewMsgDeposit(regfee, fmt.Sprintf("~:cat:ETH:%s::ETH.ETH", GetRandomETHAddress()), s.signer)
	c.Assert(s.handleDeposit(msg), IsNil)

	// swap small amount not to trigger preferred asset swap
	swapAmt := cosmos.NewUint(3000 * common.One)
	coins := common.Coins{common.NewCoin(common.BaseNative, swapAmt)}
	bps := cosmos.NewUint(100)
	memo := fmt.Sprintf("=:ETH.ETH:%s::cat:%s", GetRandomETHAddress(), bps)
	msg = NewMsgDeposit(coins, memo, s.signer)
	c.Assert(s.handleDeposit(msg), IsNil)
	err := s.queueEndBlock() // main swap
	c.Assert(err, IsNil)
	err = s.queueEndBlock() // affiliate fee swap (back to cacao)
	c.Assert(err, IsNil)
	// verify affiliate fee amount in affiliate collector
	fee := common.GetSafeShare(bps, s.perc100, swapAmt)
	affCol, err := s.mgr.Keeper().GetAffiliateCollector(s.ctx, s.signer)
	c.Assert(err, IsNil)
	c.Assert(affCol.CacaoAmount, EqualTo10Bps, fee.Sub(cacaoFee))
	// save cat balance before affiliate collector payout
	catAcc, err := cat.AccAddress()
	c.Assert(err, IsNil)
	before := cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, catAcc).AmountOf(common.BaseAsset().Native()).Uint64())
	// remove preferred asset
	msg = NewMsgDeposit(regfee, "~:cat::::MAYA.CACAO", s.signer)
	c.Assert(s.handleDeposit(msg), IsNil)
	// verify that the affiliate collector is empty now
	affCol, err = s.mgr.Keeper().GetAffiliateCollector(s.ctx, s.signer)
	c.Assert(err, IsNil)
	c.Assert(affCol.CacaoAmount.IsZero(), Equals, true)
	// verify that the previous affiliate collector amount has been sent to maya alias
	after := cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, catAcc).AmountOf(common.BaseAsset().Native()).Uint64())
	c.Assert(after, EqualTo10Bps, before.Add(fee).Sub(cacaoFee))
}

func (s *MultipleAffiliatesSuite) TestCacaoAsPreferredAssetClearAffCollOnSwap(c *C) {
	FundAccount(c, s.ctx, s.mgr.Keeper(), s.signer, 10*32100000)
	addrAffCol, err := s.mgr.Keeper().GetModuleAddress(AffiliateCollectorName)
	c.Assert(err, IsNil)
	accAffCol, err := addrAffCol.AccAddress()
	c.Assert(err, IsNil)
	ttds := s.getRandomTTHORAddress().String()

	// prepare the affiliate mayaname
	affBps := cosmos.NewUint(150)
	c.Assert(s.setMayanameSimple("hello", cosmos.NewUint(150), "", EmptyBps, ""), IsNil)
	hello, err := s.mgr.Keeper().GetMAYAName(s.ctx, "hello")
	c.Assert(err, IsNil)
	hello.PreferredAsset = common.BaseNative // MAYA.CACAO as preferred
	s.mgr.Keeper().SetMAYAName(s.ctx, hello)
	acc, _ := hello.GetAlias(common.BASEChain).AccAddress()
	before := cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, acc).AmountOf(common.BaseAsset().Native()).Uint64())

	FundModule(c, s.ctx, s.mgr.Keeper(), AffiliateCollectorName, 1)
	curAC := cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, accAffCol).AmountOf(common.BaseAsset().Native()).Uint64())
	affCol, err := s.mgr.Keeper().GetAffiliateCollector(s.ctx, hello.Owner)
	c.Assert(err, IsNil)
	affCol.CacaoAmount = curAC
	s.mgr.Keeper().SetAffiliateCollector(s.ctx, affCol)
	c.Check(curAC, EqualUint, common.One)
	// now 1 * common.One is in aff coll, preferred asset is set to MAYA.CACO

	// swap small
	prevAC := curAC
	swapAmt := cosmos.NewUint(2000 * common.One)
	coins := common.Coins{common.NewCoin(common.BaseNative, swapAmt)}
	memo := fmt.Sprintf("=:THOR.RUNE:%s::hello", ttds)
	msg := NewMsgDeposit(coins, memo, s.signer)
	c.Assert(s.handleDeposit(msg), IsNil)
	err = s.queueEndBlock()
	c.Assert(err, IsNil)
	err = s.queueEndBlock()
	c.Assert(err, IsNil)
	affFees := common.GetSafeShare(affBps, s.perc100, swapAmt)
	after := cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, acc).AmountOf(common.BaseAsset().Native()).Uint64())
	exp := before.Add(affFees).Add(prevAC).Sub(cacaoFee)
	c.Assert(after, EqualTo10Bps, exp)
	curAC = cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, accAffCol).AmountOf(common.BaseAsset().Native()).Uint64())
	c.Assert(curAC.IsZero(), Equals, true)
	affCol, err = s.mgr.Keeper().GetAffiliateCollector(s.ctx, hello.Owner)
	c.Assert(err, IsNil)
	c.Assert(affCol.CacaoAmount.IsZero(), Equals, true)
}

func (s *MultipleAffiliatesSuite) TestCacaoAsPreferredAssetClearAffCollOnPreferredAssetChange(c *C) {
	FundAccount(c, s.ctx, s.mgr.Keeper(), s.signer, 10*32100000)
	addrAffCol, err := s.mgr.Keeper().GetModuleAddress(AffiliateCollectorName)
	c.Assert(err, IsNil)
	accAffCol, err := addrAffCol.AccAddress()
	c.Assert(err, IsNil)

	// prepare the affiliate mayaname
	c.Assert(s.setMayanameSimple("hello", cosmos.NewUint(150), "", EmptyBps, ""), IsNil)
	hello, err := s.mgr.Keeper().GetMAYAName(s.ctx, "hello")
	c.Assert(err, IsNil)
	hello.PreferredAsset = common.BaseNative // MAYA.CACAO as preferred
	s.mgr.Keeper().SetMAYAName(s.ctx, hello)
	acc, _ := hello.GetAlias(common.BASEChain).AccAddress()
	before := cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, acc).AmountOf(common.BaseAsset().Native()).Uint64())

	FundModule(c, s.ctx, s.mgr.Keeper(), AffiliateCollectorName, 1)
	curAC := cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, accAffCol).AmountOf(common.BaseAsset().Native()).Uint64())
	affCol, err := s.mgr.Keeper().GetAffiliateCollector(s.ctx, hello.Owner)
	c.Assert(err, IsNil)
	affCol.CacaoAmount = curAC
	s.mgr.Keeper().SetAffiliateCollector(s.ctx, affCol)
	c.Check(curAC, EqualUint, common.One)
	// now 1 * common.One is in aff coll, preferred asset is set to MAYA.CACO

	// change preferred asset
	prevAC := curAC
	c.Assert(s.setMayanameSimple("hello", EmptyBps, "", EmptyBps, "BNB.BNB"), IsNil) // BNB.BNB as preferred
	after := cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, acc).AmountOf(common.BaseAsset().Native()).Uint64())
	got := after.String()
	exp := before.Add(prevAC).String()
	c.Check(exp, Equals, got)
	curAC = cosmos.NewUint(s.mgr.Keeper().GetBalance(s.ctx, accAffCol).AmountOf(common.BaseAsset().Native()).Uint64())
	c.Check(curAC.IsZero(), Equals, true)
	affCol, err = s.mgr.Keeper().GetAffiliateCollector(s.ctx, hello.Owner)
	c.Assert(err, IsNil)
	c.Check(affCol.CacaoAmount.IsZero(), Equals, true)
}

func (s *MultipleAffiliatesSuite) TestPreferredAssetPoolNotAvailable(c *C) {
	regfee := common.Coins{common.NewCoin(common.BaseNative, cosmos.NewUint(110000000000))}
	// create the cat mayaname
	cat := GetRandomBaseAddress()
	msg := NewMsgDeposit(regfee, fmt.Sprintf("~:cat:MAYA:%s", cat), s.signer)
	c.Assert(s.handleDeposit(msg), IsNil)
	// set eth as preferred asset
	ethAlias := GetRandomETHAddress()
	msg = NewMsgDeposit(regfee, fmt.Sprintf("~:cat:ETH:%s::ETH.ETH", ethAlias), s.signer)
	c.Assert(s.handleDeposit(msg), IsNil)

	// swap big amount to trigger preferred asset swap
	// fund signer
	funds, err := common.NewCoin(common.BaseNative, cosmos.NewUint(300000_000*common.One)).Native()
	c.Assert(err, IsNil)
	err = s.mgr.Keeper().AddCoins(s.ctx, s.signer, cosmos.NewCoins(funds))
	c.Assert(err, IsNil)

	swapAmt := cosmos.NewUint(5000000 * common.One)
	bps := cosmos.NewUint(100)
	coins := common.Coins{common.NewCoin(common.BaseNative, swapAmt)}
	memo := fmt.Sprintf("=:BTC.BTC:%s::cat:%s", GetRandomBTCAddress(), bps)
	msg = NewMsgDeposit(coins, memo, s.signer)
	c.Assert(s.handleDeposit(msg), IsNil)
	err = s.queueEndBlock() // main swap processed and affiliate fee swap queued
	c.Assert(err, IsNil)
	var swaps swapItems
	swaps, err = s.queue.FetchQueue(s.ctx, s.mgr)
	c.Assert(err, IsNil)
	c.Assert(swaps, HasLen, 1) // the queued affiliate fee swap
	c.Assert(swaps[0].msg.Destination, EqualAddress, s.affColAddress)
	// switch ETH pool to Staged
	var pool Pool
	pool, err = s.mgr.Keeper().GetPool(s.ctx, common.ETHAsset)
	c.Assert(err, IsNil)
	pool.Status = PoolStaged
	err = s.mgr.Keeper().SetPool(s.ctx, pool)
	c.Assert(err, IsNil)
	// now the preferred asset swap shouldn't be queued
	err = s.queueEndBlock() // affiliate fee swap (back to affiliate collector to cacao) processed and try to queue preferred asset swap
	c.Assert(err, NotNil)
	c.Assert(strings.Contains(err.Error(), "preferred asset (ETH.ETH) pool is not available"), Equals, true)
	swaps, err = s.queue.FetchQueue(s.ctx, s.mgr)
	c.Assert(err, IsNil)
	c.Assert(swaps, HasLen, 0) // preferred asset swap is not there
}
