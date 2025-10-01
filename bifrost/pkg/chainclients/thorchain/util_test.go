package thorchain

import (
	"bytes"

	ctypes "github.com/cosmos/cosmos-sdk/types"
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

type UtilTestSuite struct{}

var _ = Suite(&UtilTestSuite{})

func (s *UtilTestSuite) SetUpSuite(c *C) {}

func (s *UtilTestSuite) TestFromCosmosToThorchain(c *C) {
	// 5 RUNE, 8 decimals
	cosmosCoin := cosmos.NewCoin("rune", ctypes.NewInt(500000000))
	thorchainCoin, err := fromThorchainToMayachain(cosmosCoin)
	c.Assert(err, IsNil)

	// 5 RUNE, 8 decimals
	expectedThorchainAsset, err := common.NewAsset("THOR.RUNE")
	c.Assert(err, IsNil)
	expectedThorchainAmount := ctypes.NewUint(500000000)
	c.Check(thorchainCoin.Asset.Equals(expectedThorchainAsset), Equals, true)
	c.Check(thorchainCoin.Amount.BigInt().Int64(), Equals, expectedThorchainAmount.BigInt().Int64())
	c.Check(thorchainCoin.Decimals, Equals, int64(8))
}

func (s *UtilTestSuite) TestFromThorchainToCosmos(c *C) {
	// 6 RUNE.RUNE, 8 decimals
	thorchainAsset, err := common.NewAsset("THOR.RUNE")
	c.Assert(err, IsNil)
	thorchainCoin := common.Coin{
		Asset:    thorchainAsset,
		Amount:   cosmos.NewUint(600000000),
		Decimals: 8,
	}
	cosmosCoin, err := fromMayachainToThorchain(thorchainCoin)
	c.Assert(err, IsNil)

	// 6 rune, 8 decimals
	expectedCosmosDenom := "rune"
	expectedCosmosAmount := int64(600000000)
	c.Check(cosmosCoin.Denom, Equals, expectedCosmosDenom)
	c.Check(cosmosCoin.Amount.Int64(), Equals, expectedCosmosAmount)
}

func (s *UtilTestSuite) TestAccAddressToString(c *C) {
	bytes := cosmos.AccAddress([]byte{86, 141, 37, 176, 152, 170, 8, 128, 245, 236, 74, 64, 239, 243, 31, 190, 217, 52, 161, 98})
	add, err := accAddressToString(bytes, "thor")
	c.Assert(err, IsNil)
	c.Assert(add, Equals, "thor126xjtvyc4gygpa0vffqwluclhmvnfgtznlpzyg")

	bytes = cosmos.AccAddress([]byte{87, 141, 37, 176, 152, 170, 8, 128, 245, 236, 74, 64, 239, 243, 31, 190, 217, 52, 161, 98})
	add, err = accAddressToString(bytes, "tthor")
	c.Assert(err, IsNil)
	c.Assert(add, Equals, "tthor127xjtvyc4gygpa0vffqwluclhmvnfgtzwehs9w")
}

func (s *UtilTestSuite) TestAccAddressFromBech32(c *C) {
	// happy path
	add, err := common.NewAddress("thor126xjtvyc4gygpa0vffqwluclhmvnfgtznlpzyg", common.LatestVersion)
	c.Assert(err, IsNil)
	acc, err := accAddressFromBech32(add)
	c.Assert(err, IsNil)
	c.Assert(bytes.Equal(acc.Bytes(), []byte{86, 141, 37, 176, 152, 170, 8, 128, 245, 236, 74, 64, 239, 243, 31, 190, 217, 52, 161, 98}), Equals, true)

	// wrong prefix
	add, err = common.NewAddress("maya126xjtvyc4gygpa0vffqwluclhmvnfgtznglwjc", common.LatestVersion)
	c.Assert(err, IsNil)
	acc, err = accAddressFromBech32(add)
	c.Check(err.Error(), Equals, "invalid address prefix maya")
	c.Assert(bytes.Equal(acc.Bytes(), []byte{86, 141, 37, 176, 152, 170, 8, 128, 245, 236, 74, 64, 239, 243, 31, 190, 217, 52, 161, 98}), Equals, false)
}
