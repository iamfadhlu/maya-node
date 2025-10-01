package types

import (
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

type LiquidityProviderSuite struct{}

var _ = Suite(&LiquidityProviderSuite{})

func (LiquidityProviderSuite) TestLiquidityProvider(c *C) {
	lp := LiquidityProvider{
		Asset:         common.BNBAsset,
		CacaoAddress:  GetRandomBNBAddress(),
		AssetAddress:  GetRandomBTCAddress(),
		LastAddHeight: 12,
	}
	c.Check(lp.Valid(), IsNil)
	c.Check(len(lp.Key()) > 0, Equals, true)
	lp1 := LiquidityProvider{
		Asset:         common.BNBAsset,
		CacaoAddress:  GetRandomBNBAddress(),
		AssetAddress:  GetRandomBTCAddress(),
		LastAddHeight: 0,
	}
	c.Check(lp1.Valid(), NotNil)

	lp2 := LiquidityProvider{
		Asset:         common.BNBAsset,
		CacaoAddress:  common.NoAddress,
		AssetAddress:  common.NoAddress,
		LastAddHeight: 100,
	}
	c.Check(lp2.Valid(), NotNil)
}

func (LiquidityProviderSuite) TestLiquidityProviderBond(c *C) {
	lp := LiquidityProvider{}

	nodeAddr := GetRandomBech32Addr()
	lp.Bond(nodeAddr, cosmos.NewUint(100))
	c.Check(lp.BondedNodes, HasLen, 1)
	c.Check(lp.BondedNodes[0].NodeAddress.Equals(nodeAddr), Equals, true)
	c.Check(lp.BondedNodes[0].Units.Equal(cosmos.NewUint(100)), Equals, true)

	// Bond to the same node should increase the units
	lp.Bond(nodeAddr, cosmos.NewUint(100))
	c.Check(lp.BondedNodes, HasLen, 1)
	c.Check(lp.BondedNodes[0].NodeAddress.Equals(nodeAddr), Equals, true)
	c.Check(lp.BondedNodes[0].Units.Equal(cosmos.NewUint(200)), Equals, true)

	// Bond to a different node should add a new record
	nodeAddr2 := GetRandomBech32Addr()
	lp.Bond(nodeAddr2, cosmos.NewUint(1000))
	c.Check(lp.BondedNodes, HasLen, 2)
	c.Check(lp.BondedNodes[1].NodeAddress.Equals(nodeAddr2), Equals, true)
	c.Check(lp.BondedNodes[1].Units.Equal(cosmos.NewUint(1000)), Equals, true)
}

func (LiquidityProviderSuite) TestLiquidityProviderUnbond(c *C) {
	// Soft migration from old model
	nodeAddr := GetRandomBech32Addr()
	lp := LiquidityProvider{
		NodeBondAddress: nodeAddr,
		Units:           cosmos.NewUint(100),
	}
	lp.Unbond(nodeAddr, cosmos.NewUint(50))
	c.Check(lp.NodeBondAddress, IsNil)
	c.Check(lp.BondedNodes, HasLen, 1)
	c.Check(lp.BondedNodes[0].NodeAddress.Equals(nodeAddr), Equals, true)
	c.Check(lp.BondedNodes[0].Units.Equal(cosmos.NewUint(50)), Equals, true)

	lp = LiquidityProvider{
		NodeBondAddress: nodeAddr,
		Units:           cosmos.NewUint(100),
	}
	lp.Unbond(nodeAddr, cosmos.NewUint(100))
	c.Check(lp.NodeBondAddress, IsNil)
	c.Check(lp.BondedNodes, HasLen, 0)

	// New model
	nodeAddr1 := GetRandomBech32Addr()
	nodeAddr2 := GetRandomBech32Addr()
	lp = LiquidityProvider{
		BondedNodes: []LPBondedNode{
			{
				NodeAddress: nodeAddr1,
				Units:       cosmos.NewUint(1000),
			},
			{
				NodeAddress: nodeAddr2,
				Units:       cosmos.NewUint(100),
			},
		},
	}
	lp.Unbond(nodeAddr1, cosmos.NewUint(100))
	c.Check(lp.BondedNodes, HasLen, 2)
	c.Check(lp.BondedNodes[0].NodeAddress.Equals(nodeAddr1), Equals, true)
	c.Check(lp.BondedNodes[0].Units.Equal(cosmos.NewUint(900)), Equals, true)
	c.Check(lp.BondedNodes[1].NodeAddress.Equals(nodeAddr2), Equals, true)
	c.Check(lp.BondedNodes[1].Units.Equal(cosmos.NewUint(100)), Equals, true)

	lp.Unbond(nodeAddr2, cosmos.NewUint(100))
	c.Check(lp.BondedNodes, HasLen, 1)
	c.Check(lp.BondedNodes[0].NodeAddress.Equals(nodeAddr1), Equals, true)
	c.Check(lp.BondedNodes[0].Units.Equal(cosmos.NewUint(900)), Equals, true)
}

func (LiquidityProviderSuite) TestLiquidityProviderGetRemainingUnits(c *C) {
	// Old model
	lp := LiquidityProvider{
		NodeBondAddress: GetRandomBech32Addr(),
	}
	c.Check(lp.GetRemainingUnits().Equal(cosmos.ZeroUint()), Equals, true)

	// New model
	lp = LiquidityProvider{
		Units: cosmos.NewUint(500),
		BondedNodes: []LPBondedNode{
			{
				NodeAddress: GetRandomBech32Addr(),
				Units:       cosmos.NewUint(100),
			},
			{
				NodeAddress: GetRandomBech32Addr(),
				Units:       cosmos.NewUint(200),
			},
		},
	}
	c.Check(lp.GetRemainingUnits().Equal(cosmos.NewUint(200)), Equals, true)
}

func (LiquidityProviderSuite) TestLiquidityProviderGetUnitsBondedToNode(c *C) {
	// Old model
	nodeAddr := GetRandomBech32Addr()
	lp := LiquidityProvider{
		NodeBondAddress: nodeAddr,
		Units:           cosmos.NewUint(100),
	}
	c.Check(lp.GetUnitsBondedToNode(nodeAddr).Equal(cosmos.NewUint(100)), Equals, true)

	// New model
	nodeAddr1 := GetRandomBech32Addr()
	nodeAddr2 := GetRandomBech32Addr()
	lp = LiquidityProvider{
		BondedNodes: []LPBondedNode{
			{
				NodeAddress: nodeAddr1,
				Units:       cosmos.NewUint(100),
			},
			{
				NodeAddress: nodeAddr2,
				Units:       cosmos.NewUint(200),
			},
		},
	}
	c.Check(lp.GetUnitsBondedToNode(nodeAddr1).Equal(cosmos.NewUint(100)), Equals, true)
	c.Check(lp.GetUnitsBondedToNode(nodeAddr2).Equal(cosmos.NewUint(200)), Equals, true)
	c.Check(lp.GetUnitsBondedToNode(GetRandomBech32Addr()).Equal(cosmos.ZeroUint()), Equals, true)
}
