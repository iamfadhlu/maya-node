package common

import (
	"github.com/blang/semver"
	. "gopkg.in/check.v1"
)

type AssetSuite struct{}

var _ = Suite(&AssetSuite{})

func (s AssetSuite) TestAsset(c *C) {
	asset, err := NewAsset("maya.cacao")
	c.Assert(err, IsNil)
	c.Check(asset.Equals(BaseNative), Equals, true)
	c.Check(asset.IsBase(), Equals, true)
	c.Check(asset.IsEmpty(), Equals, false)
	c.Check(asset.Synth, Equals, false)
	c.Check(asset.String(), Equals, "MAYA.CACAO")

	asset, err = NewAsset("maya/cacao")
	c.Assert(err, IsNil)
	err = asset.Valid()
	c.Check(err, NotNil)
	c.Check(err.Error(), Equals, "synth asset cannot have chain MAYA: MAYA/CACAO")

	c.Check(asset.Chain.Equals(BASEChain), Equals, true)
	c.Check(asset.Symbol.Equals(Symbol("CACAO")), Equals, true)
	c.Check(asset.Ticker.Equals(Ticker("CACAO")), Equals, true)

	asset, err = NewAsset("BNB.SWIPE.B-DC0")
	c.Assert(err, IsNil)
	c.Check(asset.String(), Equals, "BNB.SWIPE.B-DC0")
	c.Check(asset.Chain.Equals(BNBChain), Equals, true)
	c.Check(asset.Symbol.Equals(Symbol("SWIPE.B-DC0")), Equals, true)
	c.Check(asset.Ticker.Equals(Ticker("SWIPE.B")), Equals, true)

	// parse without chain
	asset, err = NewAsset("cacao")
	c.Assert(err, IsNil)
	c.Check(asset.Equals(BaseNative), Equals, true)

	// ETH test
	asset, err = NewAsset("eth.knc")
	c.Assert(err, IsNil)
	c.Check(asset.Chain.Equals(ETHChain), Equals, true)
	c.Check(asset.Symbol.Equals(Symbol("KNC")), Equals, true)
	c.Check(asset.Ticker.Equals(Ticker("KNC")), Equals, true)
	asset, err = NewAsset("ETH.CACAO-0x3155ba85d5f96b2d030a4966af206230e46849cb")
	c.Assert(err, IsNil)

	// DASH test
	asset, err = NewAsset("dash.dash")
	c.Assert(err, IsNil)
	c.Check(asset.Chain.Equals(DASHChain), Equals, true)
	c.Check(asset.Equals(DASHAsset), Equals, true)
	c.Check(asset.IsBase(), Equals, false)
	c.Check(asset.IsEmpty(), Equals, false)
	c.Check(asset.String(), Equals, "DASH.DASH")

	// btc/btc
	asset, err = NewAsset("btc/btc")
	c.Check(err, IsNil)
	c.Check(asset.Chain.Equals(BTCChain), Equals, true)
	c.Check(asset.Equals(BTCAsset), Equals, false)
	c.Check(asset.IsEmpty(), Equals, false)
	c.Check(asset.String(), Equals, "BTC/BTC")

	// radix test
	asset, err = NewAsset("xrd.xrd")
	c.Assert(err, IsNil)
	c.Check(asset.Chain.Equals(XRDChain), Equals, true)
	c.Check(asset.Equals(XRDAsset), Equals, true)
	c.Check(asset.IsBase(), Equals, false)
	c.Check(asset.IsEmpty(), Equals, false)
	c.Check(asset.String(), Equals, "XRD.XRD")

	// test shorts
	asset, err = NewAssetWithShortCodes(semver.MustParse("1.110.0"), "b")
	c.Assert(err, IsNil)
	c.Check(asset.String(), Equals, "BTC.BTC")

	asset, err = NewAssetWithShortCodes(semver.MustParse("1.110.0"), "et")
	c.Assert(err, IsNil)
	c.Check(asset.String(), Equals, "ETH.USDT-0XDAC17F958D2EE523A2206206994597C13D831EC7")

	asset, err = NewAssetWithShortCodes(semver.MustParse("1.110.0"), "BLAH.BLAH")
	c.Assert(err, IsNil)
	c.Check(asset.String(), Equals, "BLAH.BLAH")

	asset, err = NewAssetWithShortCodes(semver.MustParse("1.110.0"), "")
	c.Assert(err, IsNil)
	c.Check(asset.IsEmpty(), Equals, false)

	asset, err = NewAssetWithShortCodes(semver.MustParse("1.111.0"), "")
	c.Assert(err, IsNil)
	c.Check(asset.IsEmpty(), Equals, false)

	// Empty input should return empty asset on latest version
	asset, err = NewAssetWithShortCodes(semver.MustParse("1.999.0"), "")
	c.Assert(err, NotNil)
	c.Check(asset.IsEmpty(), Equals, true)

	asset, err = NewAssetWithShortCodes(semver.MustParse("0.0.0"), "BTC.BTC")
	c.Assert(err, IsNil)
	c.Check(asset.String(), Equals, "BTC.BTC")
}
