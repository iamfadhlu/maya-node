package common

import (
	"github.com/btcsuite/btcd/chaincfg"
	dashchaincfg "gitlab.com/mayachain/dashd-go/chaincfg"
	btypes "gitlab.com/thorchain/binance-sdk/common/types"
	. "gopkg.in/check.v1"
)

type ChainSuite struct{}

var _ = Suite(&ChainSuite{})

func (s ChainSuite) TestChain(c *C) {
	bnbChain, err := NewChain("bnb")
	c.Assert(err, IsNil)
	c.Check(bnbChain.Equals(BNBChain), Equals, true)
	c.Check(bnbChain.IsBNB(), Equals, true)
	c.Check(bnbChain.IsEmpty(), Equals, false)
	c.Check(bnbChain.String(), Equals, "BNB")

	_, err = NewChain("B") // too short
	c.Assert(err, NotNil)

	chains := Chains{"BNB", "BNB", "BTC"}
	c.Check(chains.Has("BTC"), Equals, true)
	c.Check(chains.Has("ETH"), Equals, false)
	uniq := chains.Distinct()
	c.Assert(uniq, HasLen, 2)

	algo := bnbChain.GetSigningAlgo()
	c.Assert(algo, Equals, SigningAlgoSecp256k1)

	c.Assert(BNBChain.GetGasAsset(), Equals, BNBAsset)
	c.Assert(BTCChain.GetGasAsset(), Equals, BTCAsset)
	c.Assert(ETHChain.GetGasAsset(), Equals, ETHAsset)
	c.Assert(DASHChain.GetGasAsset(), Equals, DASHAsset)
	c.Assert(XRDChain.GetGasAsset(), Equals, XRDAsset)
	c.Assert(ZECChain.GetGasAsset(), Equals, ZECAsset)
	c.Assert(EmptyChain.GetGasAsset(), Equals, EmptyAsset)

	c.Assert(BNBChain.AddressPrefix(MockNet), Equals, btypes.TestNetwork.Bech32Prefixes())
	c.Assert(BNBChain.AddressPrefix(TestNet), Equals, btypes.TestNetwork.Bech32Prefixes())
	c.Assert(BNBChain.AddressPrefix(MainNet), Equals, btypes.ProdNetwork.Bech32Prefixes())
	c.Assert(BNBChain.AddressPrefix(StageNet), Equals, btypes.ProdNetwork.Bech32Prefixes())

	c.Assert(BTCChain.AddressPrefix(MockNet), Equals, chaincfg.RegressionNetParams.Bech32HRPSegwit)
	c.Assert(BTCChain.AddressPrefix(TestNet), Equals, chaincfg.TestNet3Params.Bech32HRPSegwit)
	c.Assert(BTCChain.AddressPrefix(MainNet), Equals, chaincfg.MainNetParams.Bech32HRPSegwit)
	c.Assert(BTCAsset.Chain.AddressPrefix(StageNet), Equals, chaincfg.MainNetParams.Bech32HRPSegwit)

	c.Assert(DASHChain.AddressPrefix(MockNet), Equals, dashchaincfg.RegressionNetParams.Bech32HRPSegwit)
	c.Assert(DASHChain.AddressPrefix(TestNet), Equals, dashchaincfg.TestNet3Params.Bech32HRPSegwit)
	c.Assert(DASHChain.AddressPrefix(MainNet), Equals, dashchaincfg.MainNetParams.Bech32HRPSegwit)
	c.Assert(DASHChain.AddressPrefix(StageNet), Equals, dashchaincfg.MainNetParams.Bech32HRPSegwit)

	c.Assert(XRDChain.AddressPrefix(MockNet), Equals, "account_loc")
	c.Assert(XRDChain.AddressPrefix(TestNet), Equals, "account_tdx_22_")
	c.Assert(XRDChain.AddressPrefix(StageNet), Equals, "account_rdx")
	c.Assert(XRDChain.AddressPrefix(MainNet), Equals, "account_rdx")

	c.Assert(ZECChain.AddressPrefix(MockNet), Equals, "tm")
	c.Assert(ZECChain.AddressPrefix(TestNet), Equals, "tm")
	c.Assert(ZECChain.AddressPrefix(MainNet), Equals, "t")
	c.Assert(ZECChain.AddressPrefix(StageNet), Equals, "t")
}
