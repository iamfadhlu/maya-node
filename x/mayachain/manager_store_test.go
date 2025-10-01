package mayachain

import (
	"crypto/sha256"
	"fmt"

	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

type StoreManagerTestSuite struct{}

var _ = Suite(&StoreManagerTestSuite{})

func (s *StoreManagerTestSuite) TestRefundLPSlashesMigrationStorev111(c *C) {
	ctx, mgr := setupManagerForTest(c)
	SetupConfigForTest()

	// Use pool status: https://stagenet.mayanode.mayachain.info/mayachain/pool/btc.btc?height=1705338
	btcPool := NewPool()
	btcPool.Asset = common.BTCAsset
	btcPool.BalanceAsset = cosmos.NewUint(163549)
	btcPool.BalanceCacao = cosmos.NewUint(109413402535576)
	btcPool.LPUnits = cosmos.NewUint(101211742002894)
	btcPool.Status = PoolAvailable
	btcPool.Decimals = 8
	btcPool.SynthUnits = cosmos.ZeroUint()
	btcPool.PendingInboundAsset = cosmos.ZeroUint()
	btcPool.PendingInboundCacao = cosmos.ZeroUint()
	c.Assert(mgr.K.SetPool(ctx, btcPool), IsNil)

	lp1Address := GetRandomBaseAddress()
	lp2Address := GetRandomBaseAddress()
	lp3Address := GetRandomBaseAddress()
	lp4Address := GetRandomBaseAddress()
	lp5Address := GetRandomBaseAddress()
	lp6Address := GetRandomBaseAddress()
	lp7Address := GetRandomBaseAddress()
	lp8Address := GetRandomBaseAddress()
	nodeAddress := GetRandomBech32Addr()
	btcAddress := GetRandomBTCAddress()
	reserveLPAddress := GetRandomBaseAddress()

	// Use lps: https://stagenet.mayanode.mayachain.info/mayachain/pool/btc.btc/liquidity_providers?height=1705338
	liquidityProviders := LiquidityProviders{
		{
			Asset:        common.BTCAsset,
			CacaoAddress: lp1Address,
			Units:        cosmos.NewUint(6249321602997),
			BondedNodes: []types.LPBondedNode{
				{
					NodeAddress: nodeAddress,
					Units:       cosmos.NewUint(6249321602997),
				},
			},
		},
		{
			Asset:        common.BTCAsset,
			CacaoAddress: reserveLPAddress,
			Units:        cosmos.NewUint(57501356794006),
		},
		{
			Asset:        common.BTCAsset,
			CacaoAddress: lp2Address,
			Units:        cosmos.ZeroUint(),
		},
		{
			Asset:        common.BTCAsset,
			CacaoAddress: lp3Address,
			Units:        cosmos.ZeroUint(),
		},
		{
			Asset:        common.BTCAsset,
			CacaoAddress: lp4Address,
			Units:        cosmos.ZeroUint(),
		},
		{
			Asset:        common.BTCAsset,
			CacaoAddress: lp5Address,
			Units:        cosmos.ZeroUint(),
		},
		{
			Asset:        common.BTCAsset,
			CacaoAddress: lp6Address,
			Units:        cosmos.NewUint(6249321602997),
			BondedNodes: []types.LPBondedNode{
				{
					NodeAddress: nodeAddress,
					Units:       cosmos.NewUint(6249321602997),
				},
			},
		},
		{
			Asset:             common.BTCAsset,
			CacaoAddress:      lp7Address,
			AssetAddress:      btcAddress,
			Units:             cosmos.NewUint(30211742002886),
			CacaoDepositValue: cosmos.NewUint(21848545684804),
			AssetDepositValue: cosmos.NewUint(54496),
		},
		{
			Asset:        common.BTCAsset,
			CacaoAddress: lp8Address,
			Units:        cosmos.ZeroUint(),
		},
	}
	mgr.K.SetLiquidityProviders(ctx, liquidityProviders)

	liquidityProvidersSlash := []LiquidityProvidersSlash{
		{
			Asset:             common.BTCAsset,
			BondAddressString: nodeAddress.String(),
			LPAddressString:   lp2Address.String(),
			LPUnits:           cosmos.NewUint(5679541434897),
		},
		{
			Asset:             common.BTCAsset,
			BondAddressString: nodeAddress.String(),
			LPAddressString:   lp3Address.String(),
			LPUnits:           cosmos.NewUint(5679541434897),
		},
		{
			Asset:             common.BTCAsset,
			BondAddressString: nodeAddress.String(),
			LPAddressString:   lp4Address.String(),
			LPUnits:           cosmos.NewUint(5679541434897),
		},
		{
			Asset:             common.BTCAsset,
			BondAddressString: nodeAddress.String(),
			LPAddressString:   lp5Address.String(),
			LPUnits:           cosmos.NewUint(5679541434897),
		},
		{
			Asset:             common.BTCAsset,
			BondAddressString: nodeAddress.String(),
			LPAddressString:   lp8Address.String(),
			LPUnits:           cosmos.NewUint(5679541434897),
		},
	}

	c.Assert(refundLPSlashes(ctx, mgr, liquidityProvidersSlash, reserveLPAddress.String()), IsNil)

	// Check new units and bonded nodes
	// verify with https://stagenet.mayanode.mayachain.info/mayachain/pool/btc.btc/liquidity_providers?height=1705337
	lp2, err := mgr.K.GetLiquidityProvider(ctx, common.BTCAsset, lp2Address)
	c.Assert(err, IsNil)
	c.Assert(lp2.Units.Equal(cosmos.NewUint(5679541434897)), Equals, true)
	c.Assert(lp2.BondedNodes[0].NodeAddress.String(), Equals, nodeAddress.String())
	c.Assert(lp2.BondedNodes[0].Units.Equal(cosmos.NewUint(5679541434897)), Equals, true)

	lp3, err := mgr.K.GetLiquidityProvider(ctx, common.BTCAsset, lp3Address)
	c.Assert(err, IsNil)
	c.Assert(lp3.Units.Equal(cosmos.NewUint(5679541434897)), Equals, true)
	c.Assert(lp3.BondedNodes[0].NodeAddress.String(), Equals, nodeAddress.String())
	c.Assert(lp3.BondedNodes[0].Units.Equal(cosmos.NewUint(5679541434897)), Equals, true)

	lp4, err := mgr.K.GetLiquidityProvider(ctx, common.BTCAsset, lp3Address)
	c.Assert(err, IsNil)
	c.Assert(lp4.Units.Equal(cosmos.NewUint(5679541434897)), Equals, true)
	c.Assert(lp4.BondedNodes[0].NodeAddress.String(), Equals, nodeAddress.String())
	c.Assert(lp4.BondedNodes[0].Units.Equal(cosmos.NewUint(5679541434897)), Equals, true)

	lp5, err := mgr.K.GetLiquidityProvider(ctx, common.BTCAsset, lp5Address)
	c.Assert(err, IsNil)
	c.Assert(lp5.Units.Equal(cosmos.NewUint(5679541434897)), Equals, true)
	c.Assert(lp5.BondedNodes[0].NodeAddress.String(), Equals, nodeAddress.String())
	c.Assert(lp5.BondedNodes[0].Units.Equal(cosmos.NewUint(5679541434897)), Equals, true)

	lp8, err := mgr.K.GetLiquidityProvider(ctx, common.BTCAsset, lp8Address)
	c.Assert(err, IsNil)
	c.Assert(lp8.Units.Equal(cosmos.NewUint(5679541434897)), Equals, true)
	c.Assert(lp8.BondedNodes[0].NodeAddress.String(), Equals, nodeAddress.String())
	c.Assert(lp8.BondedNodes[0].Units.Equal(cosmos.NewUint(5679541434897)), Equals, true)

	reserveLP, err := mgr.K.GetLiquidityProvider(ctx, common.BTCAsset, reserveLPAddress)
	c.Assert(err, IsNil)
	c.Assert(reserveLP.Units.Equal(cosmos.NewUint(29103649619521)), Equals, true)
}

// Check that the hashing behaves as expected.
func (s *StoreManagerTestSuite) TestMemoHash(c *C) {
	inboundTxID := "B07A6B1B40ADBA2E404D9BCE1BEF6EDE6F70AD135E83806E4F4B6863CF637D0B"
	memo := fmt.Sprintf("REFUND:%s", inboundTxID)

	// This is the hash produced if using sha256 instead of Keccak-256
	// (which gave EE31ACC02D631DC3220990A1DD2E9030F4CFC227A61E975B5DEF1037106D1CCD)
	hash := fmt.Sprintf("%X", sha256.Sum256([]byte(memo)))
	fakeTxID, err := common.NewTxID(hash)
	c.Assert(err, IsNil)
	c.Assert(fakeTxID.String(), Equals, "AC0605F714563B3D5A34C64CCB6D90C1EA4EF13E1BA5E8638FE1FC796547332F")
}

func (s *StoreManagerTestSuite) TestRemoveBondedNodeFromLP(c *C) {
	ctx, mgr := setupManagerForTest(c)
	SetupConfigForTest()

	accAddress, err := GetRandomBaseAddress().AccAddress()
	c.Assert(err, IsNil)
	accAddress2, err := GetRandomBaseAddress().AccAddress()
	c.Assert(err, IsNil)
	address3 := GetRandomBaseAddress()
	accAddress3, err := address3.AccAddress()
	c.Assert(err, IsNil)
	accAddress4, err := GetRandomBaseAddress().AccAddress()
	c.Assert(err, IsNil)

	lp := LiquidityProvider{
		Asset:             common.AETHAsset,
		CacaoAddress:      GetRandomBaseAddress(),
		AssetAddress:      GetRandomETHAddress(),
		Units:             cosmos.NewUint(100_000_000_000),
		CacaoDepositValue: cosmos.NewUint(100_000_000_000),
		AssetDepositValue: cosmos.NewUint(100_000_000_000),
		BondedNodes: []LPBondedNode{
			{
				NodeAddress: accAddress,
				Units:       cosmos.NewUint(25_000_000_000),
			},
			{
				NodeAddress: accAddress2,
				Units:       cosmos.NewUint(25_000_000_000),
			},
			{
				NodeAddress: accAddress3,
				Units:       cosmos.NewUint(25_000_000_000),
			},
			{
				NodeAddress: accAddress4,
				Units:       cosmos.NewUint(25_000_000_000),
			},
		},
	}

	mgr.K.SetLiquidityProvider(ctx, lp)

	removed, err := removeBondedNodeFromLP(ctx, mgr, BondedNodeRemovalParams{
		LPAddress:         lp.CacaoAddress,
		BondedNodeAddress: address3,
		Asset:             lp.Asset,
	})
	c.Assert(err, IsNil)
	c.Assert(removed, Equals, true)

	newLp, err := mgr.K.GetLiquidityProvider(ctx, lp.Asset, lp.CacaoAddress)
	c.Assert(err, IsNil)

	c.Assert(len(newLp.BondedNodes), Equals, 3)
	c.Assert(newLp.BondedNodes[0].NodeAddress.Equals(accAddress), Equals, true)
	c.Assert(newLp.BondedNodes[1].NodeAddress.Equals(accAddress2), Equals, true)
	c.Assert(newLp.BondedNodes[2].NodeAddress.Equals(accAddress4), Equals, true)
}
