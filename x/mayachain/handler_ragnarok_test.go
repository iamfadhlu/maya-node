package mayachain

import (
	"fmt"

	"github.com/blang/semver"
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
)

type HandlerRagnarokSuite struct{}

var _ = Suite(&HandlerRagnarokSuite{})

type TestRagnarokKeeper struct {
	keeper.KVStoreDummy
	activeNodeAccount NodeAccount
	vault             Vault
}

func (k *TestRagnarokKeeper) GetNodeAccount(_ cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error) {
	if k.activeNodeAccount.NodeAddress.Equals(addr) {
		return k.activeNodeAccount, nil
	}
	return NodeAccount{}, nil
}

func (HandlerRagnarokSuite) TestRagnarok(c *C) {
	ctx, _ := setupKeeperForTest(c)

	keeper := &TestRagnarokKeeper{
		activeNodeAccount: GetRandomValidatorNode(NodeActive),
		vault:             GetRandomVault(),
	}

	handler := NewRagnarokHandler(NewDummyMgrWithKeeper(keeper))

	// invalid message should result errors
	msg := NewMsgNetworkFee(ctx.BlockHeight(), common.BNBChain, 1, bnbSingleTxFee.Uint64(), GetRandomBech32Addr())
	result, err := handler.Run(ctx, msg)
	c.Check(result, IsNil, Commentf("invalid message should result an error"))
	c.Check(err, NotNil, Commentf("invalid message should result an error"))
	addr, err := keeper.vault.PubKey.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)

	tx := NewObservedTx(common.Tx{
		ID:          GetRandomTxHash(),
		Chain:       common.BNBChain,
		Coins:       common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(1*common.One))},
		Memo:        "",
		FromAddress: GetRandomBNBAddress(),
		ToAddress:   addr,
		Gas:         BNBGasFeeSingleton,
	}, 12, GetRandomPubKey(), 12)

	msgRagnarok := NewMsgRagnarok(tx, 1, keeper.activeNodeAccount.NodeAddress)
	err = handler.validate(ctx, *msgRagnarok)
	c.Assert(err, IsNil)

	// invalid msg
	msgRagnarok = &MsgRagnarok{}
	err = handler.validate(ctx, *msgRagnarok)
	c.Assert(err, NotNil)
	result, err = handler.Run(ctx, msgRagnarok)
	c.Check(err, NotNil, Commentf("invalid message should fail validation"))
	c.Check(result, IsNil, Commentf("invalid message should fail validation"))
}

type TestRagnarokKeeperHappyPath struct {
	keeper.KVStoreDummy
	activeNodeAccount     NodeAccount
	activeNodeAccountBond cosmos.Uint
	bp                    BondProviders
	lp                    LiquidityProvider
	newVault              Vault
	retireVault           Vault
	txout                 *TxOut
	pool                  Pool
}

func (k *TestRagnarokKeeperHappyPath) GetTxOut(ctx cosmos.Context, blockHeight int64) (*TxOut, error) {
	if k.txout != nil && k.txout.Height == blockHeight {
		return k.txout, nil
	}
	return nil, errKaboom
}

func (k *TestRagnarokKeeperHappyPath) SetTxOut(ctx cosmos.Context, blockOut *TxOut) error {
	if k.txout.Height == blockOut.Height {
		k.txout = blockOut
		return nil
	}
	return errKaboom
}

func (k *TestRagnarokKeeperHappyPath) GetVault(_ cosmos.Context, pk common.PubKey) (Vault, error) {
	if pk.Equals(k.retireVault.PubKey) {
		return k.retireVault, nil
	}
	if pk.Equals(k.newVault.PubKey) {
		return k.newVault, nil
	}
	return Vault{}, fmt.Errorf("vault not found")
}

func (k *TestRagnarokKeeperHappyPath) GetNodeAccountByPubKey(_ cosmos.Context, _ common.PubKey) (NodeAccount, error) {
	return k.activeNodeAccount, nil
}

func (k *TestRagnarokKeeperHappyPath) SetNodeAccount(_ cosmos.Context, na NodeAccount) error {
	k.activeNodeAccount = na
	return nil
}

func (k *TestRagnarokKeeperHappyPath) GetPool(_ cosmos.Context, _ common.Asset) (Pool, error) {
	return k.pool, nil
}

func (k *TestRagnarokKeeperHappyPath) SetPool(_ cosmos.Context, p Pool) error {
	k.pool = p
	return nil
}

func (k *TestRagnarokKeeperHappyPath) CalcNodeLiquidityBond(_ cosmos.Context, _ NodeAccount) (cosmos.Uint, error) {
	return k.activeNodeAccountBond, nil
}

func (k *TestRagnarokKeeperHappyPath) SetBondProviders(ctx cosmos.Context, bp BondProviders) error {
	k.bp = bp
	return nil
}

func (k *TestRagnarokKeeperHappyPath) GetBondProviders(ctx cosmos.Context, add cosmos.AccAddress) (BondProviders, error) {
	return k.bp, nil
}

func (k *TestRagnarokKeeperHappyPath) SetLiquidityProviders(_ cosmos.Context, lps LiquidityProviders) {
	for _, lp := range lps {
		if lp.CacaoAddress.Equals(k.lp.CacaoAddress) {
			k.lp = lp
			return
		}
	}
}

func (k *TestRagnarokKeeperHappyPath) GetLiquidityProvider(_ cosmos.Context, asset common.Asset, addr common.Address) (LiquidityProvider, error) {
	if k.lp.CacaoAddress.Equals(addr) {
		return k.lp, nil
	}
	return LiquidityProvider{
		Asset:        k.lp.Asset,
		CacaoAddress: GetRandomBaseAddress(),
		AssetAddress: GetRandomBNBAddress(),
		Units:        cosmos.ZeroUint(),
	}, nil
}

func (k *TestRagnarokKeeperHappyPath) GetLiquidityProviderByAssets(_ cosmos.Context, assets common.Assets, addr common.Address) (LiquidityProviders, error) {
	return LiquidityProviders{k.lp}, nil
}

func (k *TestRagnarokKeeperHappyPath) GetModuleAddress(module string) (common.Address, error) {
	return GetRandomBaseAddress(), nil
}

func (k *TestRagnarokKeeperHappyPath) ListActiveValidators(_ cosmos.Context) (NodeAccounts, error) {
	return NodeAccounts{k.activeNodeAccount}, nil
}

func (k *TestRagnarokKeeperHappyPath) GetConfigInt64(ctx cosmos.Context, key constants.ConstantName) int64 {
	if key == constants.SlashPenalty {
		return constants.GetConstantValues(semver.MustParse("9999999.0.0")).GetInt64Value(constants.SlashPenalty)
	}
	if key == constants.PauseOnSlashThreshold {
		return constants.GetConstantValues(semver.MustParse("9999999.0.0")).GetInt64Value(constants.PauseOnSlashThreshold)
	}
	return -1
}

func (HandlerRagnarokSuite) TestRagnarokHappyPath(c *C) {
	ctx, _ := setupKeeperForTest(c)
	retireVault := GetRandomVault()

	newVault := GetRandomVault()
	txout := NewTxOut(1)
	newVaultAddr, err := newVault.PubKey.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)
	txout.TxArray = append(txout.TxArray, TxOutItem{
		Chain:       common.BNBChain,
		InHash:      common.BlankTxID,
		ToAddress:   newVaultAddr,
		VaultPubKey: retireVault.PubKey,
		Coin:        common.NewCoin(common.BNBAsset, cosmos.NewUint(1024)),
		Memo:        NewRagnarokMemo(1).String(),
	})
	keeper := &TestRagnarokKeeperHappyPath{
		activeNodeAccount: GetRandomValidatorNode(NodeActive),
		newVault:          newVault,
		retireVault:       retireVault,
		txout:             txout,
	}
	addr, err := keeper.retireVault.PubKey.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)
	handler := NewRagnarokHandler(NewDummyMgrWithKeeper(keeper))
	tx := NewObservedTx(common.Tx{
		ID:    GetRandomTxHash(),
		Chain: common.BNBChain,
		Coins: common.Coins{
			common.NewCoin(common.BNBAsset, cosmos.NewUint(1024)),
		},
		Memo:        NewRagnarokMemo(1).String(),
		FromAddress: addr,
		ToAddress:   newVaultAddr,
		Gas:         BNBGasFeeSingleton,
	}, 1, retireVault.PubKey, 1)

	msgRagnarok := NewMsgRagnarok(tx, 1, keeper.activeNodeAccount.NodeAddress)
	_, err = handler.handle(ctx, *msgRagnarok)
	c.Assert(err, IsNil)
	c.Assert(keeper.txout.TxArray[0].OutHash.Equals(tx.Tx.ID), Equals, true)

	// fail to get tx out
	msgRagnarok1 := NewMsgRagnarok(tx, 1024, keeper.activeNodeAccount.NodeAddress)
	result, err := handler.handle(ctx, *msgRagnarok1)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)
}

func (HandlerRagnarokSuite) TestSlash(c *C) {
	ctx, _ := setupKeeperForTest(c)
	retireVault := GetRandomVault()

	newVault := GetRandomVault()
	txout := NewTxOut(1)
	newVaultAddr, err := newVault.PubKey.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)

	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.BalanceCacao = cosmos.NewUint(100 * common.One)
	pool.LPUnits = cosmos.NewUint(100 * common.One)

	na := GetRandomValidatorNode(NodeActive)
	naBond := cosmos.NewUint(100 * common.One)

	bp := NewBondProviders(na.NodeAddress)
	acc, err := na.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp.Providers = append(bp.Providers, NewBondProvider(acc))
	bp.Providers[0].Bonded = true

	retireVault.Membership = []string{
		na.PubKeySet.Secp256k1.String(),
	}
	retireVault.Coins = common.NewCoins(common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One)))
	lp := LiquidityProvider{
		Asset:        common.BNBAsset,
		Units:        naBond,
		CacaoAddress: common.Address(na.BondAddress.String()),
		AssetAddress: GetRandomBNBAddress(),
	}
	lp.Bond(na.NodeAddress, cosmos.NewUint(100*common.One))
	keeper := &TestRagnarokKeeperHappyPath{
		activeNodeAccount:     na,
		activeNodeAccountBond: naBond,
		bp:                    bp,
		lp:                    lp,
		newVault:              newVault,
		retireVault:           retireVault,
		txout:                 txout,
		pool:                  pool,
	}
	addr, err := keeper.retireVault.PubKey.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)

	mgr := NewDummyMgrWithKeeper(keeper)
	mgr.slasher = newSlasherVCUR(keeper, NewDummyEventMgr())
	handler := NewRagnarokHandler(mgr)

	tx := NewObservedTx(common.Tx{
		ID:    GetRandomTxHash(),
		Chain: common.BNBChain,
		Coins: common.Coins{
			common.NewCoin(common.BNBAsset, cosmos.NewUint(1024)),
		},
		Memo:        NewRagnarokMemo(1).String(),
		FromAddress: addr,
		ToAddress:   newVaultAddr,
		Gas:         BNBGasFeeSingleton,
	}, 1, retireVault.PubKey, 1)

	msgRagnarok := NewMsgRagnarok(tx, 1, keeper.activeNodeAccount.NodeAddress)
	_, err = handler.handle(ctx, *msgRagnarok)
	c.Assert(err, IsNil)
	expectedUnits := cosmos.NewUint(9999942214)
	c.Assert(keeper.lp.Units, DeepEquals, expectedUnits, Commentf("expected %s, got %s", expectedUnits, keeper.lp.Units))
}
