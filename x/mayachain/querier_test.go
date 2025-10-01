package mayachain

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/blang/semver"

	abci "github.com/tendermint/tendermint/abci/types"
	. "gopkg.in/check.v1"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	ckeys "github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"gitlab.com/mayachain/mayanode/cmd"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	openapi "gitlab.com/mayachain/mayanode/openapi/gen"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
	"gitlab.com/mayachain/mayanode/x/mayachain/query"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

type QuerierSuite struct {
	kb      cosmos.KeybaseStore
	mgr     *Mgrs
	k       keeper.Keeper
	querier cosmos.Querier
	ctx     cosmos.Context
}

var _ = Suite(&QuerierSuite{})

type TestQuerierKeeper struct {
	keeper.KVStoreDummy
	txOut *TxOut
}

func (k *TestQuerierKeeper) GetTxOut(_ cosmos.Context, _ int64) (*TxOut, error) {
	return k.txOut, nil
}

func (s *QuerierSuite) SetUpTest(c *C) {
	kb := ckeys.NewInMemory()
	username := "mayachain"
	password := "password"

	_, _, err := kb.NewMnemonic(username, ckeys.English, cmd.BASEChainHDPath, password, hd.Secp256k1)
	c.Assert(err, IsNil)
	s.kb = cosmos.KeybaseStore{
		SignerName:   username,
		SignerPasswd: password,
		Keybase:      kb,
	}
	s.ctx, s.mgr = setupManagerForTest(c)
	s.k = s.mgr.Keeper()
	s.querier = NewQuerier(s.mgr, s.kb)
}

func (s *QuerierSuite) TestQueryKeysign(c *C) {
	ctx, _ := setupKeeperForTest(c)
	ctx = ctx.WithBlockHeight(12)

	pk := GetRandomPubKey()
	toAddr := GetRandomBNBAddress()
	txOut := NewTxOut(1)
	txOutItem := TxOutItem{
		Chain:       common.BNBChain,
		VaultPubKey: pk,
		ToAddress:   toAddr,
		InHash:      GetRandomTxHash(),
		Coin:        common.NewCoin(common.BNBAsset, cosmos.NewUint(100*common.One)),
	}
	txOut.TxArray = append(txOut.TxArray, txOutItem)
	keeper := &TestQuerierKeeper{
		txOut: txOut,
	}

	_, mgr := setupManagerForTest(c)
	mgr.K = keeper
	querier := NewQuerier(mgr, s.kb)

	path := []string{
		"keysign",
		"5",
		pk.String(),
	}
	res, err := querier(ctx, path, abci.RequestQuery{})
	c.Assert(err, IsNil)
	c.Assert(res, NotNil)
}

func (s *QuerierSuite) TestQueryPool(c *C) {
	ctx, mgr := setupManagerForTest(c)
	querier := NewQuerier(mgr, s.kb)
	path := []string{"pools"}

	pubKey := GetRandomPubKey()
	asgard := NewVault(ctx.BlockHeight(), ActiveVault, AsgardVault, pubKey, common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	c.Assert(mgr.Keeper().SetVault(ctx, asgard), IsNil)

	poolBNB := NewPool()
	poolBNB.Asset = common.BNBAsset
	poolBNB.LPUnits = cosmos.NewUint(100)

	poolBTC := NewPool()
	poolBTC.Asset = common.BTCAsset
	poolBTC.LPUnits = cosmos.NewUint(0)

	err := mgr.Keeper().SetPool(ctx, poolBNB)
	c.Assert(err, IsNil)

	err = mgr.Keeper().SetPool(ctx, poolBTC)
	c.Assert(err, IsNil)

	res, err := querier(ctx, path, abci.RequestQuery{})
	c.Assert(err, IsNil)

	var out Pools

	err = json.Unmarshal(res, &out)
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 1)

	poolBTC.LPUnits = cosmos.NewUint(100)
	err = mgr.Keeper().SetPool(ctx, poolBTC)
	c.Assert(err, IsNil)

	res, err = querier(ctx, path, abci.RequestQuery{})
	c.Assert(err, IsNil)

	err = json.Unmarshal(res, &out)
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 2)

	result, err := s.querier(s.ctx, []string{query.QueryPool.Key, "BNB.BNB"}, abci.RequestQuery{})
	c.Assert(result, HasLen, 0)
	c.Assert(err, NotNil)
}

func (s *QuerierSuite) TestVaultss(c *C) {
	ctx, mgr := setupManagerForTest(c)
	querier := NewQuerier(mgr, s.kb)
	path := []string{"pools"}

	pubKey := GetRandomPubKey()
	asgard := NewVault(ctx.BlockHeight(), ActiveVault, AsgardVault, pubKey, common.Chains{common.BNBChain}.Strings(), nil)
	c.Assert(mgr.Keeper().SetVault(ctx, asgard), IsNil)

	poolBNB := NewPool()
	poolBNB.Asset = common.BNBAsset
	poolBNB.LPUnits = cosmos.NewUint(100)

	poolBTC := NewPool()
	poolBTC.Asset = common.BTCAsset
	poolBTC.LPUnits = cosmos.NewUint(0)

	err := mgr.Keeper().SetPool(ctx, poolBNB)
	c.Assert(err, IsNil)

	err = mgr.Keeper().SetPool(ctx, poolBTC)
	c.Assert(err, IsNil)

	res, err := querier(ctx, path, abci.RequestQuery{})
	c.Assert(err, IsNil)

	var out Pools
	err = json.Unmarshal(res, &out)
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 1)

	poolBTC.LPUnits = cosmos.NewUint(100)
	err = mgr.Keeper().SetPool(ctx, poolBTC)
	c.Assert(err, IsNil)

	res, err = querier(ctx, path, abci.RequestQuery{})
	c.Assert(err, IsNil)

	err = json.Unmarshal(res, &out)
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 2)

	result, err := s.querier(s.ctx, []string{query.QueryPool.Key, "BNB.BNB"}, abci.RequestQuery{})
	c.Assert(result, HasLen, 0)
	c.Assert(err, NotNil)
}

func (s *QuerierSuite) TestSaverPools(c *C) {
	ctx, mgr := setupManagerForTest(c)
	querier := NewQuerier(mgr, s.kb)
	path := []string{"pools"}

	poolBNB := NewPool()
	poolBNB.Asset = common.BNBAsset.GetSyntheticAsset()
	poolBNB.LPUnits = cosmos.NewUint(100)

	poolBTC := NewPool()
	poolBTC.Asset = common.BTCAsset
	poolBTC.LPUnits = cosmos.NewUint(1000)

	poolETH := NewPool()
	poolETH.Asset = common.ETHAsset.GetSyntheticAsset()
	poolETH.LPUnits = cosmos.NewUint(100)

	err := mgr.Keeper().SetPool(ctx, poolBNB)
	c.Assert(err, IsNil)

	err = mgr.Keeper().SetPool(ctx, poolBTC)
	c.Assert(err, IsNil)

	err = mgr.Keeper().SetPool(ctx, poolETH)
	c.Assert(err, IsNil)

	res, err := querier(ctx, path, abci.RequestQuery{})
	c.Assert(err, IsNil)

	var out []openapi.Pool
	err = json.Unmarshal(res, &out)
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 1)
}

func (s *QuerierSuite) TestQueryNodeAccounts(c *C) {
	ctx, keeper := setupKeeperForTest(c)

	_, mgr := setupManagerForTest(c)
	querier := NewQuerier(mgr, s.kb)
	path := []string{"nodes"}

	nodeAccount := GetRandomValidatorNode(NodeActive)
	bp := NewBondProviders(nodeAccount.NodeAddress)
	acc, err := nodeAccount.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp.Providers = append(bp.Providers, NewBondProvider(acc))
	bp.Providers[0].Bonded = true
	SetupLiquidityBondForTestV105(c, ctx, keeper, common.BNBAsset, nodeAccount.BondAddress, nodeAccount, cosmos.NewUint(1000*common.One))
	c.Assert(keeper.SetBondProviders(ctx, bp), IsNil)
	c.Assert(keeper.SetNodeAccount(ctx, nodeAccount), IsNil)
	vault := GetRandomVault()
	vault.Status = ActiveVault
	vault.BlockHeight = 1
	c.Assert(keeper.SetVault(ctx, vault), IsNil)
	res, err := querier(ctx, path, abci.RequestQuery{})
	c.Assert(err, IsNil)

	var out types.NodeAccounts
	err1 := json.Unmarshal(res, &out)
	c.Assert(err1, IsNil)
	c.Assert(len(out), Equals, 1)

	nodeAccount2 := GetRandomValidatorNode(NodeActive)
	bp = NewBondProviders(nodeAccount2.NodeAddress)
	acc, err = nodeAccount2.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp.Providers = append(bp.Providers, NewBondProvider(acc))
	bp.Providers[0].Bonded = true
	SetupLiquidityBondForTestV105(c, ctx, keeper, common.BNBAsset, nodeAccount2.BondAddress, nodeAccount2, cosmos.NewUint(3000*common.One))
	c.Assert(keeper.SetBondProviders(ctx, bp), IsNil)
	c.Assert(keeper.SetNodeAccount(ctx, nodeAccount2), IsNil)

	/* Check Bond-weighted rewards estimation works*/
	var nodeAccountResp []openapi.Node

	// Add bond rewards + set min bond for bond-weighted system
	network, _ := keeper.GetNetwork(ctx)
	network.BondRewardRune = cosmos.NewUint(common.One * 1000)
	c.Assert(keeper.SetNetwork(ctx, network), IsNil)
	keeper.SetMimir(ctx, "MinimumBondInCacao", common.One*100000)

	res, err = querier(ctx, path, abci.RequestQuery{})
	c.Assert(err, IsNil)

	err1 = json.Unmarshal(res, &nodeAccountResp)
	c.Assert(err1, IsNil)
	c.Assert(len(nodeAccountResp), Equals, 2)

	for _, node := range nodeAccountResp {
		c.Assert(node.Reward, Equals, cosmos.NewUint(common.One*500).String(), Commentf("expected %s, got %s", cosmos.NewUint(500*common.One), node.Reward))
		bondProviderPool := node.BondProviders.Providers[0].Pools
		c.Check(len(bondProviderPool), Equals, 1)
		switch node.NodeAddress {
		case nodeAccount.NodeAddress.String():
			c.Assert(bondProviderPool[common.BNBAsset.String()], Equals, cosmos.NewUint(common.One*1000).String(), Commentf("expected %s, got %s", cosmos.NewUint(1000*common.One), bondProviderPool[common.BNBAsset.String()]))
		case nodeAccount2.NodeAddress.String():
			c.Assert(bondProviderPool[common.BNBAsset.String()], Equals, cosmos.NewUint(common.One*3000).String(), Commentf("expected %s, got %s", cosmos.NewUint(3000*common.One), bondProviderPool[common.BNBAsset.String()]))
		default:
			c.Fail()
		}
	}

	/* Check querier only returns nodes with bond */
	c.Assert(keeper.SetNodeAccount(ctx, nodeAccount2), IsNil)

	res, err = querier(ctx, path, abci.RequestQuery{})
	c.Assert(err, IsNil)

	err1 = json.Unmarshal(res, &out)
	c.Assert(err1, IsNil)
	c.Assert(len(out), Equals, 2)
}

func (s *QuerierSuite) TestQuerierRagnarokInProgress(c *C) {
	req := abci.RequestQuery{
		Data:   nil,
		Path:   query.QueryRagnarok.Key,
		Height: s.ctx.BlockHeight(),
		Prove:  false,
	}
	// test ragnarok
	result, err := s.querier(s.ctx, []string{query.QueryRagnarok.Key}, req)
	c.Assert(result, NotNil)
	c.Assert(err, IsNil)
	var ragnarok bool
	c.Assert(json.Unmarshal(result, &ragnarok), IsNil)
	c.Assert(ragnarok, Equals, false)
}

func (s *QuerierSuite) TestQueryLiquidityProviders(c *C) {
	req := abci.RequestQuery{
		Data:   nil,
		Path:   query.QueryLiquidityProviders.Key,
		Height: s.ctx.BlockHeight(),
		Prove:  false,
	}
	// test liquidity providers
	result, err := s.querier(s.ctx, []string{query.QueryLiquidityProviders.Key, "BNB.BNB"}, req)
	c.Assert(result, NotNil)
	c.Assert(err, IsNil)
	s.k.SetLiquidityProvider(s.ctx, LiquidityProvider{
		Asset:              common.BNBAsset,
		CacaoAddress:       GetRandomBNBAddress(),
		AssetAddress:       GetRandomBNBAddress(),
		LastAddHeight:      1024,
		LastWithdrawHeight: 0,
		Units:              cosmos.NewUint(10),
	})
	result, err = s.querier(s.ctx, []string{query.QueryLiquidityProviders.Key, "BNB.BNB"}, req)
	c.Assert(err, IsNil)
	var lps LiquidityProviders
	c.Assert(json.Unmarshal(result, &lps), IsNil)
	c.Assert(lps, HasLen, 1)

	req = abci.RequestQuery{
		Data:   nil,
		Path:   query.QuerySavers.Key,
		Height: s.ctx.BlockHeight(),
		Prove:  false,
	}

	s.k.SetLiquidityProvider(s.ctx, LiquidityProvider{
		Asset:              common.BNBAsset.GetSyntheticAsset(),
		CacaoAddress:       GetRandomBNBAddress(),
		AssetAddress:       GetRandomBaseAddress(),
		LastAddHeight:      1024,
		LastWithdrawHeight: 0,
		Units:              cosmos.NewUint(10),
	})

	// Query Savers from SaversPool
	result, err = s.querier(s.ctx, []string{query.QuerySavers.Key, "BNB.BNB"}, req)
	c.Assert(err, IsNil)
	var savers LiquidityProviders
	c.Assert(json.Unmarshal(result, &savers), IsNil)
	c.Assert(lps, HasLen, 1)
}

func (s *QuerierSuite) TestQueryTxInVoter(c *C) {
	req := abci.RequestQuery{
		Data:   nil,
		Path:   query.QueryTxVoter.Key,
		Height: s.ctx.BlockHeight(),
		Prove:  false,
	}
	tx := GetRandomTx()
	// test getTxInVoter
	result, err := s.querier(s.ctx, []string{query.QueryTxVoter.Key, tx.ID.String()}, req)
	c.Assert(result, IsNil)
	c.Assert(err, NotNil)
	observedTxInVote := NewObservedTxVoter(tx.ID, []ObservedTx{NewObservedTx(tx, s.ctx.BlockHeight(), GetRandomPubKey(), s.ctx.BlockHeight())})
	s.k.SetObservedTxInVoter(s.ctx, observedTxInVote)
	result, err = s.querier(s.ctx, []string{query.QueryTxVoter.Key, tx.ID.String()}, req)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
	var voter openapi.TxDetailsResponse
	c.Assert(json.Unmarshal(result, &voter), IsNil)

	// common.Tx Valid cannot be used for openapi.Tx, so checking some criteria individually.
	c.Assert(voter.TxId == nil, Equals, false)
	c.Assert(len(voter.Txs) == 1, Equals, true)
	c.Assert(voter.Txs[0].ExternalObservedHeight == nil, Equals, false)
	c.Assert(*voter.Txs[0].ExternalObservedHeight <= 0, Equals, false)
	c.Assert(voter.Txs[0].ObservedPubKey == nil, Equals, false)
	c.Assert(voter.Txs[0].ExternalConfirmationDelayHeight == nil, Equals, false)
	c.Assert(*voter.Txs[0].ExternalConfirmationDelayHeight <= 0, Equals, false)
	c.Assert(voter.Txs[0].Tx.Id == nil, Equals, false)
	c.Assert(voter.Txs[0].Tx.FromAddress == nil, Equals, false)
	c.Assert(voter.Txs[0].Tx.ToAddress == nil, Equals, false)
	c.Assert(voter.Txs[0].Tx.Chain == nil, Equals, false)
	c.Assert(len(voter.Txs[0].Tx.Coins) == 0, Equals, false)
}

func (s *QuerierSuite) TestQueryTxInVoterNew(c *C) {
	req := abci.RequestQuery{
		Data:   nil,
		Path:   query.QueryTxInVoter.Key,
		Height: s.ctx.BlockHeight(),
		Prove:  false,
	}
	tx := GetRandomTx()

	// test getTxInVoter - should fail when voter doesn't exist
	result, err := s.querier(s.ctx, []string{query.QueryTxInVoter.Key, tx.ID.String()}, req)
	c.Assert(result, IsNil)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, fmt.Sprintf("tx in voter: %s doesn't exist", tx.ID))

	// Add TxInVoter
	observedTxInVote := NewObservedTxVoter(tx.ID, []ObservedTx{NewObservedTx(tx, s.ctx.BlockHeight(), GetRandomPubKey(), s.ctx.BlockHeight())})
	s.k.SetObservedTxInVoter(s.ctx, observedTxInVote)

	// Query should succeed now
	result, err = s.querier(s.ctx, []string{query.QueryTxInVoter.Key, tx.ID.String()}, req)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
	var voter openapi.TxDetailsResponse
	c.Assert(json.Unmarshal(result, &voter), IsNil)

	// Verify response fields
	c.Assert(voter.TxId == nil, Equals, false)
	c.Assert(*voter.TxId, Equals, tx.ID.String())
	c.Assert(len(voter.Txs) == 1, Equals, true)
	c.Assert(voter.Txs[0].ExternalObservedHeight == nil, Equals, false)
	c.Assert(*voter.Txs[0].ExternalObservedHeight <= 0, Equals, false)
	c.Assert(voter.Txs[0].ObservedPubKey == nil, Equals, false)
	c.Assert(voter.Txs[0].ExternalConfirmationDelayHeight == nil, Equals, false)
	c.Assert(*voter.Txs[0].ExternalConfirmationDelayHeight <= 0, Equals, false)
	c.Assert(voter.Txs[0].Tx.Id == nil, Equals, false)
	c.Assert(voter.Txs[0].Tx.FromAddress == nil, Equals, false)
	c.Assert(voter.Txs[0].Tx.ToAddress == nil, Equals, false)
	c.Assert(voter.Txs[0].Tx.Chain == nil, Equals, false)
	c.Assert(len(voter.Txs[0].Tx.Coins) == 0, Equals, false)

	// Test with no path - should fail
	result, err = s.querier(s.ctx, []string{query.QueryTxInVoter.Key}, req)
	c.Assert(result, IsNil)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "tx id not provided")

	// Test with invalid tx id
	result, err = s.querier(s.ctx, []string{query.QueryTxInVoter.Key, "invalid-tx-id"}, req)
	c.Assert(result, IsNil)
	c.Assert(err, NotNil)
}

func (s *QuerierSuite) TestQueryTxOutVoterNew(c *C) {
	req := abci.RequestQuery{
		Data:   nil,
		Path:   query.QueryTxOutVoter.Key,
		Height: s.ctx.BlockHeight(),
		Prove:  false,
	}
	tx := GetRandomTx()

	// test getTxOutVoter - should fail when voter doesn't exist
	result, err := s.querier(s.ctx, []string{query.QueryTxOutVoter.Key, tx.ID.String()}, req)
	c.Assert(result, IsNil)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, fmt.Sprintf("tx out voter: %s doesn't exist", tx.ID))

	// Add TxOutVoter
	observedTxOutVote := NewObservedTxVoter(tx.ID, []ObservedTx{NewObservedTx(tx, s.ctx.BlockHeight(), GetRandomPubKey(), s.ctx.BlockHeight())})
	s.k.SetObservedTxOutVoter(s.ctx, observedTxOutVote)

	// Query should succeed now
	result, err = s.querier(s.ctx, []string{query.QueryTxOutVoter.Key, tx.ID.String()}, req)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
	var voter openapi.TxDetailsResponse
	c.Assert(json.Unmarshal(result, &voter), IsNil)

	// Verify response fields
	c.Assert(voter.TxId == nil, Equals, false)
	c.Assert(*voter.TxId, Equals, tx.ID.String())
	c.Assert(len(voter.Txs) == 1, Equals, true)
	c.Assert(voter.Txs[0].ExternalObservedHeight == nil, Equals, false)
	c.Assert(*voter.Txs[0].ExternalObservedHeight <= 0, Equals, false)
	c.Assert(voter.Txs[0].ObservedPubKey == nil, Equals, false)
	c.Assert(voter.Txs[0].ExternalConfirmationDelayHeight == nil, Equals, false)
	c.Assert(*voter.Txs[0].ExternalConfirmationDelayHeight <= 0, Equals, false)
	c.Assert(voter.Txs[0].Tx.Id == nil, Equals, false)
	c.Assert(voter.Txs[0].Tx.FromAddress == nil, Equals, false)
	c.Assert(voter.Txs[0].Tx.ToAddress == nil, Equals, false)
	c.Assert(voter.Txs[0].Tx.Chain == nil, Equals, false)
	c.Assert(len(voter.Txs[0].Tx.Coins) == 0, Equals, false)

	// Test with no path - should fail
	result, err = s.querier(s.ctx, []string{query.QueryTxOutVoter.Key}, req)
	c.Assert(result, IsNil)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "tx id not provided")

	// Test with invalid tx id
	result, err = s.querier(s.ctx, []string{query.QueryTxOutVoter.Key, "invalid-tx-id"}, req)
	c.Assert(result, IsNil)
	c.Assert(err, NotNil)
}

// func (s *QuerierSuite) TestQueryTxStages(c *C) {
// 	req := abci.RequestQuery{
// 		Data:   nil,
// 		Path:   query.QueryTxStages.Key,
// 		Height: s.ctx.BlockHeight(),
// 		Prove:  false,
// 	}
// 	tx := GetRandomTx()
// 	// test getTxInVoter
// 	result, err := s.querier(s.ctx, []string{query.QueryTxStages.Key, tx.ID.String()}, req)
// 	c.Assert(result, NotNil) // Expecting a not-started Observation stage.
// 	c.Assert(err, IsNil)     // Expecting no error for an unobserved hash.
// 	observedTxInVote := NewObservedTxVoter(tx.ID, []ObservedTx{NewObservedTx(tx, s.ctx.BlockHeight(), GetRandomPubKey(), s.ctx.BlockHeight())})
// 	s.k.SetObservedTxInVoter(s.ctx, observedTxInVote)
// 	result, err = s.querier(s.ctx, []string{query.QueryTxStages.Key, tx.ID.String()}, req)
// 	c.Assert(err, IsNil)
// 	c.Assert(result, NotNil)
// }

// func (s *QuerierSuite) TestQueryTxStatus(c *C) {
// 	req := abci.RequestQuery{
// 		Data:   nil,
// 		Path:   query.QueryTxStatus.Key,
// 		Height: s.ctx.BlockHeight(),
// 		Prove:  false,
// 	}
// 	tx := GetRandomTx()
// 	// test getTxInVoter
// 	result, err := s.querier(s.ctx, []string{query.QueryTxStatus.Key, tx.ID.String()}, req)
// 	c.Assert(result, NotNil) // Expecting a not-started Observation stage.
// 	c.Assert(err, IsNil)     // Expecting no error for an unobserved hash.
// 	observedTxInVote := NewObservedTxVoter(tx.ID, []ObservedTx{NewObservedTx(tx, s.ctx.BlockHeight(), GetRandomPubKey(), s.ctx.BlockHeight())})
// 	s.k.SetObservedTxInVoter(s.ctx, observedTxInVote)
// 	result, err = s.querier(s.ctx, []string{query.QueryTxStatus.Key, tx.ID.String()}, req)
// 	c.Assert(err, IsNil)
// 	c.Assert(result, NotNil)
// }

func (s *QuerierSuite) TestQueryTx(c *C) {
	req := abci.RequestQuery{
		Data:   nil,
		Path:   query.QueryTx.Key,
		Height: s.ctx.BlockHeight(),
		Prove:  false,
	}
	tx := GetRandomTx()
	// test get tx in
	result, err := s.querier(s.ctx, []string{query.QueryTx.Key, tx.ID.String()}, req)
	c.Assert(result, IsNil)
	c.Assert(err, NotNil)
	nodeAccount := GetRandomValidatorNode(NodeActive)
	c.Assert(s.k.SetNodeAccount(s.ctx, nodeAccount), IsNil)
	voter, err := s.k.GetObservedTxInVoter(s.ctx, tx.ID)
	c.Assert(err, IsNil)
	voter.Add(NewObservedTx(tx, s.ctx.BlockHeight(), nodeAccount.PubKeySet.Secp256k1, s.ctx.BlockHeight()), nodeAccount.NodeAddress)
	s.k.SetObservedTxInVoter(s.ctx, voter)
	result, err = s.querier(s.ctx, []string{query.QueryTx.Key, tx.ID.String()}, req)
	c.Assert(err, IsNil)
	var newTx struct {
		openapi.ObservedTx `json:"observed_tx"`
		KeysignMetrics     types.TssKeysignMetric `json:"keysign_metric,omitempty"`
	}
	c.Assert(json.Unmarshal(result, &newTx), IsNil)

	// common.Tx Valid cannot be used for openapi.Tx, so checking some criteria individually.
	c.Assert(newTx.ExternalObservedHeight == nil, Equals, false)
	c.Assert(*newTx.ExternalObservedHeight <= 0, Equals, false)
	c.Assert(newTx.ObservedPubKey == nil, Equals, false)
	c.Assert(newTx.ExternalConfirmationDelayHeight == nil, Equals, false)
	c.Assert(*newTx.ExternalConfirmationDelayHeight <= 0, Equals, false)
	c.Assert(newTx.Tx.Id == nil, Equals, false)
	c.Assert(newTx.Tx.FromAddress == nil, Equals, false)
	c.Assert(newTx.Tx.ToAddress == nil, Equals, false)
	c.Assert(newTx.Tx.Chain == nil, Equals, false)
	c.Assert(len(newTx.Tx.Coins) == 0, Equals, false)
}

func (s *QuerierSuite) TestQueryKeyGen(c *C) {
	req := abci.RequestQuery{
		Data:   nil,
		Path:   query.QueryKeygensPubkey.Key,
		Height: s.ctx.BlockHeight(),
		Prove:  false,
	}

	result, err := s.querier(s.ctx, []string{
		query.QueryKeygensPubkey.Key,
		"whatever",
	}, req)

	c.Assert(result, IsNil)
	c.Assert(err, NotNil)

	result, err = s.querier(s.ctx, []string{
		query.QueryKeygensPubkey.Key,
		"10000",
	}, req)

	c.Assert(result, IsNil)
	c.Assert(err, NotNil)

	result, err = s.querier(s.ctx, []string{
		query.QueryKeygensPubkey.Key,
		strconv.FormatInt(s.ctx.BlockHeight(), 10),
	}, req)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)

	result, err = s.querier(s.ctx, []string{
		query.QueryKeygensPubkey.Key,
		strconv.FormatInt(s.ctx.BlockHeight(), 10),
		GetRandomPubKey().String(),
	}, req)
	c.Assert(result, NotNil)
	c.Assert(err, IsNil)
}

func (s *QuerierSuite) TestQueryQueue(c *C) {
	result, err := s.querier(s.ctx, []string{
		query.QueryQueue.Key,
		strconv.FormatInt(s.ctx.BlockHeight(), 10),
	}, abci.RequestQuery{})
	c.Assert(result, NotNil)
	c.Assert(err, IsNil)
	var q openapi.QueueResponse
	c.Assert(json.Unmarshal(result, &q), IsNil)
}

func (s *QuerierSuite) TestQueryHeights(c *C) {
	result, err := s.querier(s.ctx, []string{
		query.QueryHeights.Key,
		strconv.FormatInt(s.ctx.BlockHeight(), 10),
	}, abci.RequestQuery{})
	c.Assert(result, IsNil)
	c.Assert(err, NotNil)

	result, err = s.querier(s.ctx, []string{
		query.QueryHeights.Key,
	}, abci.RequestQuery{})
	c.Assert(result, NotNil)
	c.Assert(err, IsNil)
	var q []openapi.LastBlock
	c.Assert(json.Unmarshal(result, &q), IsNil)

	result, err = s.querier(s.ctx, []string{
		query.QueryHeights.Key,
		"BTC",
	}, abci.RequestQuery{})
	c.Assert(result, NotNil)
	c.Assert(err, IsNil)
	c.Assert(json.Unmarshal(result, &q), IsNil)

	result, err = s.querier(s.ctx, []string{
		query.QueryChainHeights.Key,
		"BTC",
	}, abci.RequestQuery{})
	c.Assert(result, NotNil)
	c.Assert(err, IsNil)
	c.Assert(json.Unmarshal(result, &q), IsNil)
}

func (s *QuerierSuite) TestQueryConstantValues(c *C) {
	result, err := s.querier(s.ctx, []string{
		query.QueryConstantValues.Key,
	}, abci.RequestQuery{})
	c.Assert(result, NotNil)
	c.Assert(err, IsNil)
}

func (s *QuerierSuite) TestQueryMimir(c *C) {
	s.k.SetMimir(s.ctx, "hello", 111)
	result, err := s.querier(s.ctx, []string{
		query.QueryMimirValues.Key,
	}, abci.RequestQuery{})
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
	var m map[string]int64
	c.Assert(json.Unmarshal(result, &m), IsNil)
	c.Assert(m, HasLen, 1)
	c.Assert(m["HELLO"], Equals, int64(111))
}

func (s *QuerierSuite) TestQueryBan(c *C) {
	result, err := s.querier(s.ctx, []string{
		query.QueryBan.Key,
	}, abci.RequestQuery{})
	c.Assert(result, IsNil)
	c.Assert(err, NotNil)

	result, err = s.querier(s.ctx, []string{
		query.QueryBan.Key,
		"Whatever",
	}, abci.RequestQuery{})
	c.Assert(result, IsNil)
	c.Assert(err, NotNil)

	result, err = s.querier(s.ctx, []string{
		query.QueryBan.Key,
		GetRandomBech32Addr().String(),
	}, abci.RequestQuery{})
	c.Assert(result, NotNil)
	c.Assert(err, IsNil)
}

func (s *QuerierSuite) TestQueryNodeAccount(c *C) {
	result, err := s.querier(s.ctx, []string{
		query.QueryNode.Key,
	}, abci.RequestQuery{})
	c.Assert(result, IsNil)
	c.Assert(err, NotNil)

	result, err = s.querier(s.ctx, []string{
		query.QueryNode.Key,
		"Whatever",
	}, abci.RequestQuery{})
	c.Assert(result, IsNil)
	c.Assert(err, NotNil)

	na := GetRandomValidatorNode(NodeActive)
	bp := NewBondProviders(na.NodeAddress)
	acc, err := na.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp.Providers = append(bp.Providers, NewBondProvider(acc))
	bp.Providers[0].Bonded = true
	SetupLiquidityBondForTest(c, s.ctx, s.k, common.BNBAsset, na.BondAddress, na, cosmos.NewUint(1000*common.One))
	c.Assert(s.k.SetBondProviders(s.ctx, bp), IsNil)
	c.Assert(s.k.SetNodeAccount(s.ctx, na), IsNil)
	vault := GetRandomVault()
	vault.Status = ActiveVault
	vault.BlockHeight = 1
	c.Assert(s.k.SetVault(s.ctx, vault), IsNil)
	result, err = s.querier(s.ctx, []string{
		query.QueryNode.Key,
		na.NodeAddress.String(),
	}, abci.RequestQuery{})
	c.Assert(result, NotNil)
	c.Assert(err, IsNil)
	var r openapi.Node
	c.Assert(json.Unmarshal(result, &r), IsNil)

	/* Check bond-weighted rewards estimation works */
	// Add another node with 75% of the bond
	nodeAccount2 := GetRandomValidatorNode(NodeActive)
	bp = NewBondProviders(nodeAccount2.NodeAddress)
	acc, err = nodeAccount2.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp.Providers = append(bp.Providers, NewBondProvider(acc))
	bp.Providers[0].Bonded = true
	SetupLiquidityBondForTest(c, s.ctx, s.k, common.BNBAsset, nodeAccount2.BondAddress, nodeAccount2, cosmos.NewUint(3000*common.One))
	c.Assert(s.k.SetBondProviders(s.ctx, bp), IsNil)
	c.Assert(s.k.SetNodeAccount(s.ctx, nodeAccount2), IsNil)

	// Add bond rewards + set min bond for bond-weighted system
	network, _ := s.k.GetNetwork(s.ctx)
	network.BondRewardRune = cosmos.NewUint(common.One * 1000)
	c.Assert(s.k.SetNetwork(s.ctx, network), IsNil)
	s.k.SetMimir(s.ctx, "MinimumBondInCacao", common.One*100000)

	// Get first node
	result, err = s.querier(s.ctx, []string{
		query.QueryNode.Key,
		na.NodeAddress.String(),
	}, abci.RequestQuery{})
	c.Assert(result, NotNil)
	c.Assert(err, IsNil)
	var r2 openapi.Node
	c.Assert(json.Unmarshal(result, &r2), IsNil)

	// Node rewards are distributed equally, so 50% of the rewards
	c.Assert(r2.Bond, Equals, cosmos.NewUint(common.One*2000).String(), Commentf("expected %s, got %s", cosmos.NewUint(2000*common.One).String(), r2.Bond))
	c.Assert(r2.Reward, Equals, cosmos.NewUint(common.One*500).String(), Commentf("expected %s, got %s", cosmos.NewUint(500*common.One).String(), r2.Reward))

	// Get second node
	result, err = s.querier(s.ctx, []string{
		query.QueryNode.Key,
		nodeAccount2.NodeAddress.String(),
	}, abci.RequestQuery{})
	c.Assert(result, NotNil)
	c.Assert(err, IsNil)
	var r3 openapi.Node
	c.Assert(json.Unmarshal(result, &r3), IsNil)

	// Second node has 75% of bond, but should have 50% of rewards too
	c.Assert(r3.Bond, Equals, cosmos.NewUint(common.One*6000).String(), Commentf("expected %s, got %s", cosmos.NewUint(6000*common.One).String(), r3.Bond))
	c.Assert(r3.Reward, Equals, cosmos.NewUint(common.One*500).String(), Commentf("expected %s, got %s", cosmos.NewUint(500*common.One).String(), r3.Reward))
}

func (s *QuerierSuite) TestQueryPoolAddresses(c *C) {
	na := GetRandomValidatorNode(NodeActive)
	c.Assert(s.k.SetNodeAccount(s.ctx, na), IsNil)
	result, err := s.querier(s.ctx, []string{
		query.QueryInboundAddresses.Key,
		na.NodeAddress.String(),
	}, abci.RequestQuery{})
	c.Assert(result, NotNil)
	c.Assert(err, IsNil)

	var resp struct {
		Current []struct {
			Chain   common.Chain   `json:"chain"`
			PubKey  common.PubKey  `json:"pub_key"`
			Address common.Address `json:"address"`
			Halted  bool           `json:"halted"`
		} `json:"current"`
	}
	c.Assert(json.Unmarshal(result, &resp), IsNil)
}

func (s *QuerierSuite) TestQueryKeysignArrayPubKey(c *C) {
	na := GetRandomValidatorNode(NodeActive)
	c.Assert(s.k.SetNodeAccount(s.ctx, na), IsNil)
	result, err := s.querier(s.ctx, []string{
		query.QueryKeysignArrayPubkey.Key,
	}, abci.RequestQuery{})
	c.Assert(result, IsNil)
	c.Assert(err, NotNil)

	result, err = s.querier(s.ctx, []string{
		query.QueryKeysignArrayPubkey.Key,
		"asdf",
	}, abci.RequestQuery{})
	c.Assert(result, IsNil)
	c.Assert(err, NotNil)

	result, err = s.querier(s.ctx, []string{
		query.QueryKeysignArrayPubkey.Key,
		strconv.FormatInt(s.ctx.BlockHeight(), 10),
	}, abci.RequestQuery{})
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
	var r openapi.KeysignResponse
	c.Assert(json.Unmarshal(result, &r), IsNil)
}

func (s *QuerierSuite) TestQueryNetwork(c *C) {
	result, err := s.querier(s.ctx, []string{
		query.QueryNetwork.Key,
	}, abci.RequestQuery{})
	c.Assert(result, NotNil)
	c.Assert(err, IsNil)
	var r Network
	c.Assert(json.Unmarshal(result, &r), IsNil)
}

func (s *QuerierSuite) TestQueryAsgardVault(c *C) {
	c.Assert(s.k.SetVault(s.ctx, GetRandomVault()), IsNil)
	result, err := s.querier(s.ctx, []string{
		query.QueryVaultsAsgard.Key,
	}, abci.RequestQuery{})
	c.Assert(result, NotNil)
	c.Assert(err, IsNil)
	var r Vaults
	c.Assert(json.Unmarshal(result, &r), IsNil)
}

func (s *QuerierSuite) TestQueryYggdrasilVault(c *C) {
	vault := GetRandomVault()
	vault.Type = YggdrasilVault
	vault.AddFunds(common.Coins{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One*100)),
	})
	c.Assert(s.k.SetVault(s.ctx, vault), IsNil)
	result, err := s.querier(s.ctx, []string{
		query.QueryVaultsYggdrasil.Key,
	}, abci.RequestQuery{})
	c.Assert(result, NotNil)
	c.Assert(err, IsNil)
	var r []openapi.YggdrasilVault
	c.Assert(json.Unmarshal(result, &r), IsNil)
}

func (s *QuerierSuite) TestQueryVaultPubKeys(c *C) {
	node := GetRandomValidatorNode(NodeActive)
	c.Assert(s.k.SetNodeAccount(s.ctx, node), IsNil)
	vault := GetRandomVault()
	vault.PubKey = node.PubKeySet.Secp256k1
	vault.Type = YggdrasilVault
	vault.AddFunds(common.Coins{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One*100)),
	})
	vault.Routers = []types.ChainContract{
		{
			Chain:  "ETH",
			Router: "0xE65e9d372F8cAcc7b6dfcd4af6507851Ed31bb44",
		},
	}
	c.Assert(s.k.SetVault(s.ctx, vault), IsNil)
	vault1 := GetRandomVault()
	vault1.Routers = vault.Routers
	c.Assert(s.k.SetVault(s.ctx, vault1), IsNil)
	result, err := s.querier(s.ctx, []string{
		query.QueryVaultPubkeys.Key,
	}, abci.RequestQuery{})
	c.Assert(result, NotNil)
	c.Assert(err, IsNil)
	var r openapi.VaultPubkeysResponse
	c.Assert(json.Unmarshal(result, &r), IsNil)
}

func (s *QuerierSuite) TestQueryBalanceModule(c *C) {
	c.Assert(s.k.SetVault(s.ctx, GetRandomVault()), IsNil)
	result, err := s.querier(s.ctx, []string{
		query.QueryBalanceModule.Key,
		"asgard",
	}, abci.RequestQuery{})
	c.Assert(result, NotNil)
	c.Assert(err, IsNil)
	var r struct {
		Name    string            `json:"name"`
		Address cosmos.AccAddress `json:"address"`
		Coins   sdk.Coins         `json:"coins"`
	}
	c.Assert(json.Unmarshal(result, &r), IsNil)
}

func (s *QuerierSuite) TestQueryVault(c *C) {
	vault := GetRandomVault()

	// Not enough argument
	result, err := s.querier(s.ctx, []string{
		query.QueryVault.Key,
		"BNB",
	}, abci.RequestQuery{})

	c.Assert(result, IsNil)
	c.Assert(err, NotNil)

	c.Assert(s.k.SetVault(s.ctx, vault), IsNil)
	result, err = s.querier(s.ctx, []string{
		query.QueryVault.Key,
		vault.PubKey.String(),
	}, abci.RequestQuery{})
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
	var returnVault Vault
	c.Assert(json.Unmarshal(result, &returnVault), IsNil)
	c.Assert(vault.PubKey.Equals(returnVault.PubKey), Equals, true)
	c.Assert(vault.Type, Equals, returnVault.Type)
	c.Assert(vault.Status, Equals, returnVault.Status)
	c.Assert(vault.BlockHeight, Equals, returnVault.BlockHeight)
}

func (s *QuerierSuite) TestQueryVersion(c *C) {
	result, err := s.querier(s.ctx, []string{
		query.QueryVersion.Key,
	}, abci.RequestQuery{})
	c.Assert(result, NotNil)
	c.Assert(err, IsNil)
	var r openapi.VersionResponse
	c.Assert(json.Unmarshal(result, &r), IsNil)

	verComputed := s.k.GetLowestActiveVersion(s.ctx)
	c.Assert(r.Current, Equals, verComputed.String(),
		Commentf("query should return same version as computed"))

	// override the version computed in BeginBlock
	s.k.SetVersionWithCtx(s.ctx, semver.MustParse("4.5.6"))

	result, err = s.querier(s.ctx, []string{
		query.QueryVersion.Key,
	}, abci.RequestQuery{})
	c.Assert(result, NotNil)
	c.Assert(err, IsNil)
	c.Assert(json.Unmarshal(result, &r), IsNil)
	c.Assert(r.Current, Equals, "4.5.6",
		Commentf("query should use stored version"))
}

func (s *QuerierSuite) TestQueryLiquidityAuctionTier(c *C) {
	// Not enough argument
	result, err := s.querier(s.ctx, []string{
		query.QueryLiquidityAuctionTier.Key,
		"BNB.BNB",
	}, abci.RequestQuery{})

	c.Assert(result, IsNil)
	c.Assert(err, NotNil)

	// liquidity auction hasn't passed
	address := GetRandomBaseAddress()
	lp := types.LiquidityProvider{
		Asset:                     common.BNBAsset,
		CacaoAddress:              address,
		AssetAddress:              GetRandomBNBAddress(),
		Units:                     cosmos.NewUint(100000),
		WithdrawCounter:           cosmos.NewUint(0),
		LastWithdrawCounterHeight: 0,
	}
	s.k.SetLiquidityProvider(s.ctx, lp)

	result, err = s.querier(s.ctx, []string{
		query.QueryLiquidityAuctionTier.Key,
		common.BNBAsset.String(),
		address.String(),
	}, abci.RequestQuery{})

	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
	var returnLATier struct {
		Address                common.Address          `json:"address"`
		Tier                   int64                   `json:"tier"`
		LiquidityProvider      types.LiquidityProvider `json:"liquidity_provider"`
		WithdrawLimitStopBlock int64                   `json:"withdraw_limit_stop_block"`
	}
	c.Assert(json.Unmarshal(result, &returnLATier), IsNil)
	c.Assert(lp.Asset.Equals(common.BNBAsset), Equals, true)
	c.Assert(int64(0), Equals, returnLATier.Tier)
	c.Assert(lp.CacaoAddress, Equals, returnLATier.Address)
	c.Assert(lp.WithdrawCounter.Uint64(), Equals, returnLATier.LiquidityProvider.WithdrawCounter.Uint64())
	c.Assert(lp.LastWithdrawCounterHeight, Equals, returnLATier.LiquidityProvider.LastWithdrawCounterHeight)
	c.Assert(lp.Units.Uint64(), Equals, returnLATier.LiquidityProvider.Units.Uint64())
	c.Assert(returnLATier.WithdrawLimitStopBlock, Equals, int64(0))

	// liquidity auction
	s.k.SetMimir(s.ctx, constants.LiquidityAuction.String(), 20)
	lp = types.LiquidityProvider{
		Asset:                     common.BNBAsset,
		CacaoAddress:              address,
		AssetAddress:              GetRandomBNBAddress(),
		Units:                     cosmos.NewUint(100000),
		WithdrawCounter:           cosmos.NewUint(15),
		LastWithdrawCounterHeight: 19,
	}
	s.k.SetLiquidityProvider(s.ctx, lp)
	c.Assert(s.k.SetLiquidityAuctionTier(s.ctx, address, 1), IsNil)
	s.ctx = s.ctx.WithBlockHeight(21)

	result, err = s.querier(s.ctx, []string{
		query.QueryLiquidityAuctionTier.Key,
		common.BNBAsset.String(),
		address.String(),
	}, abci.RequestQuery{})

	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
	c.Assert(json.Unmarshal(result, &returnLATier), IsNil)
	c.Assert(lp.Asset.Equals(common.BNBAsset), Equals, true)
	c.Assert(int64(1), Equals, returnLATier.Tier)
	c.Assert(lp.CacaoAddress, Equals, returnLATier.Address)
	c.Assert(lp.WithdrawCounter.Uint64(), Equals, returnLATier.LiquidityProvider.WithdrawCounter.Uint64())
	c.Assert(lp.LastWithdrawCounterHeight, Equals, returnLATier.LiquidityProvider.LastWithdrawCounterHeight)
	c.Assert(lp.Units.Uint64(), Equals, returnLATier.LiquidityProvider.Units.Uint64())
	c.Assert(returnLATier.WithdrawLimitStopBlock, Equals, int64(220))
}

func (s *QuerierSuite) TestPeerIDFromPubKey(c *C) {
	// Success example, secp256k1 pubkey from Mocknet node tthor1jgnk2mg88m57csrmrlrd6c3qe4lag3e33y2f3k
	var mocknetPubKey common.PubKey = "tmayapub1addwnpepqt8tnluxnk3y5quyq952klgqnlmz2vmaynm40fp592s0um7ucvjh5eguqr3"
	c.Assert(getPeerIDFromPubKey(mocknetPubKey), Equals, "16Uiu2HAm9LeTqHJWSa67eHNZzSz3yKb64dbj7A4V1Ckv9hXyDkQR")

	// Failure example.
	expectedErrorString := "fail to parse account pub key(nonsense): decoding bech32 failed: invalid separator index -1"
	c.Assert(getPeerIDFromPubKey("nonsense"), Equals, expectedErrorString)
}

func (s *QuerierSuite) TestQueryMayaname(c *C) {
	addr := GetRandomBaseAddress()
	owner, _ := addr.AccAddress()
	mn := NewMAYAName("hello", 50,
		[]MAYANameAlias{
			{
				Chain:   common.BASEChain,
				Address: GetRandomBaseAddress(),
			},
			{
				Chain:   common.THORChain,
				Address: GetRandomTHORAddress(),
			},
		}, common.BNBAsset, owner, cosmos.NewUint(2000),
		[]types.MAYANameSubaffiliate{
			{
				Name: "alfa",
				Bps:  cosmos.NewUint(1000),
			},
			{
				Name: "beta",
				Bps:  cosmos.NewUint(2000),
			},
		},
	)
	s.mgr.Keeper().SetMAYAName(s.ctx, mn)
	result, err := s.querier(s.ctx, []string{
		query.QueryMAYAName.Key,
		"hello",
	}, abci.RequestQuery{})
	c.Assert(result, NotNil)
	c.Assert(err, IsNil)
	var r openapi.Mayaname
	c.Assert(json.Unmarshal(result, &r), IsNil)
	c.Assert(*r.AffiliateBps, Equals, int64(2000))
	c.Assert(r.Subaffiliates, HasLen, 2)
	c.Assert(*r.Subaffiliates[0].Name, Equals, "alfa")
	c.Assert(*r.Subaffiliates[0].Bps, Equals, int64(1000))
	c.Assert(*r.Subaffiliates[1].Name, Equals, "beta")
	c.Assert(*r.Subaffiliates[1].Bps, Equals, int64(2000))
}

func (s *QuerierSuite) TestQueryQuoteSwap(c *C) {
	addr := GetRandomBaseAddress()
	owner, _ := addr.AccAddress()
	mn := NewMAYAName("hello", 50,
		[]MAYANameAlias{
			{
				Chain:   common.BASEChain,
				Address: GetRandomBaseAddress(),
			},
			{
				Chain:   common.BNBChain,
				Address: GetRandomBNBAddress(),
			},
		}, common.AVAXAsset, owner, cosmos.NewUint(2000),
		[]types.MAYANameSubaffiliate{
			{
				Name: "alfa",
				Bps:  cosmos.NewUint(1000),
			},
			{
				Name: "beta",
				Bps:  cosmos.NewUint(2000),
			},
		},
	)
	s.mgr.Keeper().SetMAYAName(s.ctx, mn)

	pubKey := GetRandomPubKey()
	asgard := NewVault(s.ctx.BlockHeight(), ActiveVault, AsgardVault, pubKey, common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	c.Assert(s.mgr.Keeper().SetVault(s.ctx, asgard), IsNil)

	poolBNB := NewPool()
	poolBNB.Asset = common.BNBAsset
	poolBNB.LPUnits = cosmos.NewUint(100000 * common.One)
	poolBNB.BalanceAsset = cosmos.NewUint(100000 * common.One)
	poolBNB.BalanceCacao = cosmos.NewUint(10000000000 * common.One)
	err := s.mgr.Keeper().SetPool(s.ctx, poolBNB)
	c.Assert(err, IsNil)
	nodeAccount := GetRandomValidatorNode(NodeActive)
	c.Assert(s.mgr.Keeper().SetNodeAccount(s.ctx, nodeAccount), IsNil)
	q := url.Values{}
	q.Add(fromAssetParam, "MAYA.CACAO")
	q.Add(toAssetParam, "BNB.BNB")
	q.Add(amountParam, cosmos.NewUint(50000000000*common.One).String())
	q.Add(destinationParam, string(GetRandomBNBAddress())) // required param, not actually used, spoof it
	q.Add(affiliateParam, "hello")
	q.Add(affiliateBpsParam, "2")
	q.Add(refundAddressParam, string(GetRandomBaseAddress()))

	swapReq := abci.RequestQuery{Data: []byte("/mayachain/quote/swap?" + q.Encode())}

	res, err := s.querier(s.ctx, []string{query.QueryQuoteSwap.Key}, swapReq)
	c.Assert(err, IsNil)
	c.Assert(strings.Contains(string(res), "failed to simulate swap"), Equals, true)
}

func (s *QuerierSuite) TestQueryQuoteAnotherSwap(c *C) {
	addr := GetRandomBaseAddress()
	owner, _ := addr.AccAddress()
	mn := NewMAYAName("xx", 50,
		[]MAYANameAlias{
			{
				Chain:   common.BASEChain,
				Address: GetRandomBaseAddress(),
			},
		}, common.EmptyAsset, owner, cosmos.NewUint(150),
		[]types.MAYANameSubaffiliate{},
	)
	s.mgr.Keeper().SetMAYAName(s.ctx, mn)

	pubKey := GetRandomPubKey()
	asgard := NewVault(s.ctx.BlockHeight(), ActiveVault, AsgardVault, pubKey, common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	c.Assert(s.mgr.Keeper().SetVault(s.ctx, asgard), IsNil)

	poolBNB := NewPool()
	poolBNB.Asset = common.BNBAsset
	poolBNB.LPUnits = cosmos.NewUint(100000 * common.One)
	poolBNB.BalanceAsset = cosmos.NewUint(100000 * common.One)
	poolBNB.BalanceCacao = cosmos.NewUint(10000000000 * common.One)
	err := s.mgr.Keeper().SetPool(s.ctx, poolBNB)
	c.Assert(err, IsNil)
	nodeAccount := GetRandomValidatorNode(NodeActive)
	c.Assert(s.mgr.Keeper().SetNodeAccount(s.ctx, nodeAccount), IsNil)
	q := url.Values{}
	q.Add(fromAssetParam, "BNB.BNB")
	q.Add(toAssetParam, "MAYA.CACAO")
	q.Add(toleranceBasisPointsParam, "1000")
	q.Add(amountParam, "1_00000000")
	q.Add(destinationParam, string(GetRandomBaseAddress()))
	q.Add(affiliateParam, "xx")

	swapReq := abci.RequestQuery{Data: []byte("/mayachain/quote/swap?" + q.Encode())}
	res, err := s.querier(s.ctx, []string{query.QueryQuoteSwap.Key}, swapReq)
	c.Assert(err, IsNil)
	var qsr openapi.QuoteSwapResponse
	c.Assert(json.Unmarshal(res, &qsr), IsNil)
	checkSwapQuoteResults(c, qsr, poolBNB, 1_00000000, 150)
}

func (s *QuerierSuite) TestQueryQuoteSwapWithAffiliates(c *C) {
	affFeeBps := uint64(150)
	addr := GetRandomBaseAddress()
	owner, err := addr.AccAddress()
	c.Assert(err, IsNil)
	mn := NewMAYAName("xx", 50,
		[]MAYANameAlias{
			{
				Chain:   common.BASEChain,
				Address: addr,
			},
		}, common.EmptyAsset, owner, cosmos.NewUint(affFeeBps),
		[]types.MAYANameSubaffiliate{},
	)
	s.mgr.Keeper().SetMAYAName(s.ctx, mn)

	pubKey := GetRandomPubKey()
	asgard := NewVault(s.ctx.BlockHeight(), ActiveVault, AsgardVault, pubKey, common.Chains{common.ETHChain}.Strings(), []ChainContract{})
	asgard.Coins = common.Coins{common.NewCoin(common.ETHAsset, cosmos.NewUint(10000000*common.One))}
	c.Assert(s.mgr.Keeper().SetVault(s.ctx, asgard), IsNil)

	poolETH := NewPool()
	poolETH.Asset = common.ETHAsset
	poolETH.LPUnits = cosmos.NewUint(10000000000000)
	poolETH.BalanceAsset = cosmos.NewUint(1000000000)
	poolETH.BalanceCacao = cosmos.NewUint(100000000000000)
	err = s.mgr.Keeper().SetPool(s.ctx, poolETH)
	c.Assert(err, IsNil)

	nodeAccount := GetRandomValidatorNode(NodeActive)
	c.Assert(s.mgr.Keeper().SetNodeAccount(s.ctx, nodeAccount), IsNil)

	toAddr := GetRandomBaseAddress()
	q := url.Values{}
	q.Add(fromAssetParam, "ETH.ETH")
	q.Add(toAssetParam, "MAYA.CACAO")
	q.Add(toleranceBasisPointsParam, "1000")
	q.Add(amountParam, "10000000") // 0.1 eth = 1000 cacao / fee (150 bps) = 15 cacao
	q.Add(destinationParam, string(toAddr))
	q.Add(affiliateParam, "xx")

	swapReq := abci.RequestQuery{Data: []byte("/mayachain/quote/swap?" + q.Encode())}
	res, err := s.querier(s.ctx, []string{query.QueryQuoteSwap.Key}, swapReq)
	c.Assert(err, IsNil)
	var qsr openapi.QuoteSwapResponse
	c.Assert(json.Unmarshal(res, &qsr), IsNil)
	c.Assert(*qsr.Memo, Equals, fmt.Sprintf("=:c:%s:900000000000:xx", toAddr))
	checkSwapQuoteResults(c, qsr, poolETH, 10000000, affFeeBps)
}

func checkSwapQuoteResults(c *C, qsr openapi.QuoteSwapResponse, pool Pool, amtInAsset uint64, affBps uint64) {
	// verify fees
	feesLiquidity := cosmos.NewUintFromString(qsr.Fees.Liquidity)
	feesOutbound := cosmos.NewUintFromString(*qsr.Fees.Outbound)
	feesAffiliate := cosmos.NewUintFromString(*qsr.Fees.Affiliate)
	expTotal := feesLiquidity.Add(feesOutbound).Add(feesAffiliate)
	feesTotal := cosmos.NewUintFromString(qsr.Fees.Total)
	c.Assert(feesTotal, EqualUint, expTotal)

	// verify that the affiliate fee is within a 1% tolerance of the expected value
	amt := pool.AssetValueInRune(cosmos.NewUint(amtInAsset))
	expectedAffFee := amt.Sub(feesLiquidity).MulUint64(affBps).QuoUint64(10000).Sub(cacaoFee.QuoUint64(100))
	c.Assert(feesAffiliate, EqualTo1Percent, expectedAffFee)
}

func (s *QuerierSuite) TestQueryNodesShowsRequestedToLeave(c *C) {
	ctx, mgr := setupManagerForTest(c)
	querier := NewQuerier(mgr, s.kb)

	// Create a node that has requested to leave with significant bond
	nodeLeavingWithBond := GetRandomValidatorNode(NodeActive)
	nodeLeavingWithBond.Bond = cosmos.NewUint(100 * common.One)
	nodeLeavingWithBond.RequestedToLeave = true
	bp := NewBondProviders(nodeLeavingWithBond.NodeAddress)
	acc, err := nodeLeavingWithBond.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp.Providers = append(bp.Providers, NewBondProvider(acc))
	bp.Providers[0].Bonded = true
	SetupLiquidityBondForTest(c, ctx, mgr.Keeper(), common.BNBAsset, nodeLeavingWithBond.BondAddress, nodeLeavingWithBond, cosmos.NewUint(100*common.One))
	c.Assert(mgr.Keeper().SetBondProviders(ctx, bp), IsNil)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, nodeLeavingWithBond), IsNil)

	// Create a node that has requested to leave with very little bond
	nodeLeavingNoBond := GetRandomValidatorNode(NodeActive)
	nodeLeavingNoBond.Bond = cosmos.NewUint(common.One / 2)
	nodeLeavingNoBond.RequestedToLeave = true
	bp2 := NewBondProviders(nodeLeavingNoBond.NodeAddress)
	acc2, err := nodeLeavingNoBond.BondAddress.AccAddress()
	c.Assert(err, IsNil)
	bp2.Providers = append(bp2.Providers, NewBondProvider(acc2))
	bp2.Providers[0].Bonded = true
	SetupLiquidityBondForTest(c, ctx, mgr.Keeper(), common.BNBAsset, nodeLeavingNoBond.BondAddress, nodeLeavingNoBond, cosmos.NewUint(common.One/2))
	c.Assert(mgr.Keeper().SetBondProviders(ctx, bp2), IsNil)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, nodeLeavingNoBond), IsNil)

	// Create active vault to satisfy the queryNodes requirements
	vault := GetRandomVault()
	vault.Status = ActiveVault
	vault.BlockHeight = 1
	c.Assert(mgr.Keeper().SetVault(ctx, vault), IsNil)

	result, err := querier(ctx, []string{query.QueryNodes.Key}, abci.RequestQuery{})
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)

	var nodes []openapi.Node
	err = json.Unmarshal(result, &nodes)
	c.Assert(err, IsNil)

	// Verify the node with bond is in the results
	foundWithBond := false
	for _, n := range nodes {
		if n.NodeAddress == nodeLeavingWithBond.NodeAddress.String() {
			foundWithBond = true
			c.Assert(n.RequestedToLeave, Equals, true)
			c.Assert(n.Bond, Not(Equals), "")
			break
		}
	}
	c.Assert(foundWithBond, Equals, true, Commentf("Node that requested to leave with bond should be in results"))

	// Verify the node with very little bond is NOT in the results
	foundNoBond := false
	for _, n := range nodes {
		if n.NodeAddress == nodeLeavingNoBond.NodeAddress.String() {
			foundNoBond = true
			break
		}
	}
	c.Assert(foundNoBond, Equals, false, Commentf("Node that requested to leave with little bond should not be in results"))
}
