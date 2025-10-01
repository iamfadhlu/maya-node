package kuji

import (
	ctypes "github.com/cosmos/cosmos-sdk/types"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	. "gopkg.in/check.v1"
)

type UtilTestSuite struct{}

var _ = Suite(&UtilTestSuite{})

func (s *UtilTestSuite) SetUpSuite(c *C) {}

func (s *UtilTestSuite) TestFromKujiToMayachain(c *C) {
	// 5 KUJI, 6 decimals
	kujiraCoin := cosmos.NewCoin("ukuji", ctypes.NewInt(5000000))
	mayachainCoin, err := fromKujiToMayachain(kujiraCoin)
	c.Assert(err, IsNil)

	// 5 KUJI, 8 decimals
	expectedMayachainAsset, err := common.NewAsset("KUJI.KUJI")
	c.Assert(err, IsNil)
	expectedMayachainAmount := ctypes.NewUint(500000000)
	c.Check(mayachainCoin.Asset.Equals(expectedMayachainAsset), Equals, true)
	c.Check(mayachainCoin.Amount.BigInt().Int64(), Equals, expectedMayachainAmount.BigInt().Int64())
	c.Check(mayachainCoin.Decimals, Equals, int64(6))
}

func (s *UtilTestSuite) TestFromMayachainToKuji(c *C) {
	// 6 KUJI.KUJI, 8 decimals
	mayachainAsset, err := common.NewAsset("KUJI.KUJI")
	c.Assert(err, IsNil)
	mayachainCoin := common.Coin{
		Asset:    mayachainAsset,
		Amount:   cosmos.NewUint(600000000),
		Decimals: 6,
	}
	kujiraCoin, err := fromMayachainToKuji(mayachainCoin)
	c.Assert(err, IsNil)

	// 6 ukuji, 6 decimals
	expectedKujiDenom := "ukuji"
	expectedKujiAmount := int64(6000000)
	c.Check(kujiraCoin.Denom, Equals, expectedKujiDenom)
	c.Check(kujiraCoin.Amount.Int64(), Equals, expectedKujiAmount)
}
