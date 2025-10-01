package mayachain

import (
	"errors"
	"fmt"

	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
)

type HandlerMigrateSuite struct{}

var _ = Suite(&HandlerMigrateSuite{})

type TestMigrateKeeper struct {
	keeper.KVStoreDummy
	activeNodeAccount NodeAccount
	vault             Vault
}

// GetNodeAccount
func (k *TestMigrateKeeper) GetNodeAccount(_ cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error) {
	if k.activeNodeAccount.NodeAddress.Equals(addr) {
		return k.activeNodeAccount, nil
	}
	return NodeAccount{}, nil
}

func (HandlerMigrateSuite) TestMigrate(c *C) {
	ctx, _ := setupKeeperForTest(c)

	keeper := &TestMigrateKeeper{
		activeNodeAccount: GetRandomValidatorNode(NodeActive),
		vault:             GetRandomVault(),
	}

	handler := NewMigrateHandler(NewDummyMgrWithKeeper(keeper))

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

	msgMigrate := NewMsgMigrate(tx, 1, keeper.activeNodeAccount.NodeAddress)
	err = handler.validate(ctx, *msgMigrate)
	c.Assert(err, IsNil)

	// invalid msg
	msgMigrate = &MsgMigrate{}
	err = handler.validate(ctx, *msgMigrate)
	c.Assert(err, NotNil)
}

type TestMigrateKeeperHappyPath struct {
	keeper.Keeper
	activeNodeAccount     NodeAccount
	activeNodeAccountBond cosmos.Uint
	bp                    BondProviders
	lp                    LiquidityProvider
	newVault              Vault
	retireVault           Vault
	txout                 *TxOut
	pool                  Pool
}

func (k *TestMigrateKeeperHappyPath) GetVault(_ cosmos.Context, pk common.PubKey) (Vault, error) {
	if pk.Equals(k.retireVault.PubKey) {
		return k.retireVault, nil
	}
	if pk.Equals(k.newVault.PubKey) {
		return k.newVault, nil
	}
	return Vault{}, fmt.Errorf("vault not found")
}

func (k *TestMigrateKeeperHappyPath) GetTxOut(ctx cosmos.Context, blockHeight int64) (*TxOut, error) {
	if k.txout != nil && k.txout.Height == blockHeight {
		return k.txout, nil
	}
	return nil, errKaboom
}

func (k *TestMigrateKeeperHappyPath) SetTxOut(ctx cosmos.Context, blockOut *TxOut) error {
	if k.txout.Height == blockOut.Height {
		k.txout = blockOut
		return nil
	}
	return errKaboom
}

func (k *TestMigrateKeeperHappyPath) GetNodeAccountByPubKey(_ cosmos.Context, _ common.PubKey) (NodeAccount, error) {
	return k.activeNodeAccount, nil
}

func (k *TestMigrateKeeperHappyPath) SetNodeAccount(_ cosmos.Context, na NodeAccount) error {
	k.activeNodeAccount = na
	return nil
}

func (k *TestMigrateKeeperHappyPath) GetPool(_ cosmos.Context, _ common.Asset) (Pool, error) {
	return k.pool, nil
}

func (k *TestMigrateKeeperHappyPath) SetPool(_ cosmos.Context, p Pool) error {
	k.pool = p
	return nil
}

func (k *TestMigrateKeeperHappyPath) CalcNodeLiquidityBond(_ cosmos.Context, _ NodeAccount) (cosmos.Uint, error) {
	return k.activeNodeAccountBond, nil
}

func (k *TestMigrateKeeperHappyPath) SetBondProviders(ctx cosmos.Context, bp BondProviders) error {
	k.bp = bp
	return nil
}

func (k *TestMigrateKeeperHappyPath) GetBondProviders(ctx cosmos.Context, add cosmos.AccAddress) (BondProviders, error) {
	return k.bp, nil
}

func (k *TestMigrateKeeperHappyPath) SetLiquidityProvider(_ cosmos.Context, lp LiquidityProvider) {
	k.lp = lp
}

func (k *TestMigrateKeeperHappyPath) GetLiquidityProvider(_ cosmos.Context, asset common.Asset, addr common.Address) (LiquidityProvider, error) {
	return k.lp, nil
}

func (k *TestMigrateKeeperHappyPath) GetLiquidityProviderByAssets(_ cosmos.Context, assets common.Assets, addr common.Address) (LiquidityProviders, error) {
	return LiquidityProviders{k.lp}, nil
}

func (k *TestMigrateKeeperHappyPath) SetLiquidityProviders(ctx cosmos.Context, lps LiquidityProviders) {
	for _, lp := range lps {
		if lp.CacaoAddress.Equals(k.lp.CacaoAddress) {
			k.lp = lp
			return
		}
	}
}

func (k *TestMigrateKeeperHappyPath) GetModuleAddress(module string) (common.Address, error) {
	return GetRandomBaseAddress(), nil
}

func (k *TestMigrateKeeperHappyPath) ListActiveValidators(_ cosmos.Context) (NodeAccounts, error) {
	return NodeAccounts{k.activeNodeAccount}, nil
}

func (HandlerMigrateSuite) TestMigrateHappyPath(c *C) {
	ctx, k := setupKeeperForTest(c)
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
		Memo:        NewMigrateMemo(1).String(),
	})
	keeper := &TestMigrateKeeperHappyPath{
		Keeper:            k,
		activeNodeAccount: GetRandomValidatorNode(NodeActive),
		newVault:          newVault,
		retireVault:       retireVault,
		txout:             txout,
	}
	addr, err := keeper.retireVault.PubKey.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)
	handler := NewMigrateHandler(NewDummyMgrWithKeeper(keeper))
	tx := NewObservedTx(common.Tx{
		ID:    GetRandomTxHash(),
		Chain: common.BNBChain,
		Coins: common.Coins{
			common.NewCoin(common.BNBAsset, cosmos.NewUint(1024)),
		},
		Memo:        NewMigrateMemo(1).String(),
		FromAddress: addr,
		ToAddress:   newVaultAddr,
		Gas:         BNBGasFeeSingleton,
	}, 1, retireVault.PubKey, 1)

	msgMigrate := NewMsgMigrate(tx, 1, keeper.activeNodeAccount.NodeAddress)
	_, err = handler.Run(ctx, msgMigrate)
	c.Assert(err, IsNil)
	c.Assert(keeper.txout.TxArray[0].OutHash.Equals(tx.Tx.ID), Equals, true)
}

func (HandlerMigrateSuite) TestSlash(c *C) {
	ctx, k := setupKeeperForTest(c)
	retireVault := GetRandomVault()
	newVault := GetRandomVault()
	vaultCoins := common.Coins{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(2*common.One)),
	}
	retireVault.AddFunds(vaultCoins)
	retireVault.AddFunds(vaultCoins)
	txout := NewTxOut(1)
	newVaultAddr, err := newVault.PubKey.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)

	lpUnits := cosmos.NewUint(100 * common.One)
	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.BalanceCacao = lpUnits
	pool.LPUnits = lpUnits
	na := GetRandomValidatorNode(NodeActive)
	retireVault.Membership = []string{
		na.PubKeySet.Secp256k1.String(),
	}
	retireVault.Coins = common.NewCoins(
		common.NewCoin(common.BNBAsset, cosmos.NewUint(1024)),
	)
	bp := NewBondProviders(na.NodeAddress)
	acc, err := na.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp.Providers = append(bp.Providers, NewBondProvider(acc))
	bp.Providers[0].Bonded = true
	lp := LiquidityProvider{
		Asset:        common.BNBAsset,
		Units:        lpUnits,
		CacaoAddress: common.Address(na.BondAddress.String()),
		AssetAddress: GetRandomBNBAddress(),
	}
	lp.Bond(na.NodeAddress, cosmos.NewUint(100*common.One))
	keeper := &TestMigrateKeeperHappyPath{
		Keeper:                k,
		activeNodeAccount:     na,
		activeNodeAccountBond: cosmos.NewUint(100 * common.One),
		bp:                    bp,
		newVault:              newVault,
		retireVault:           retireVault,
		txout:                 txout,
		pool:                  pool,
		lp:                    lp,
	}
	addr, err := keeper.retireVault.PubKey.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)
	mgr := NewDummyMgrWithKeeper(keeper)
	mgr.slasher = newSlasherVCUR(keeper, NewDummyEventMgr())
	mgr.slasher = newSlasherVCUR(keeper, NewDummyEventMgr())
	handler := NewMigrateHandler(mgr)
	tx := NewObservedTx(common.Tx{
		ID:    GetRandomTxHash(),
		Chain: common.BNBChain,
		Coins: common.Coins{
			common.NewCoin(common.BNBAsset, cosmos.NewUint(1024)),
		},
		Memo:        NewMigrateMemo(1).String(),
		FromAddress: addr,
		ToAddress:   newVaultAddr,
		Gas:         BNBGasFeeSingleton,
	}, 1, retireVault.PubKey, 1)

	msgMigrate := NewMsgMigrate(tx, 1, keeper.activeNodeAccount.NodeAddress)
	_, err = handler.handle(ctx, *msgMigrate)
	c.Assert(err, IsNil)
	expectedUnits := cosmos.NewUint(9999998464)
	c.Assert(keeper.lp.Units, DeepEquals, expectedUnits, Commentf("expected %s, got %s", expectedUnits, keeper.lp.Units))
}

func (HandlerMigrateSuite) TestHandlerMigrateValidation(c *C) {
	// invalid message should return an error
	ctx, mgr := setupManagerForTest(c)
	h := NewMigrateHandler(mgr)
	result, err := h.Run(ctx, NewMsgNetworkFee(ctx.BlockHeight(), common.BNBChain, 1, bnbSingleTxFee.Uint64(), GetRandomBech32Addr()))
	c.Check(err, NotNil)
	c.Check(result, IsNil)
	c.Check(errors.Is(err, errInvalidMessage), Equals, true)
}
