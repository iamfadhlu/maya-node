//go:build !testnet && !mocknet && !stagenet
// +build !testnet,!mocknet,!stagenet

package common

import (
	"github.com/blang/semver"
	. "gopkg.in/check.v1"
)

type AddressSuite struct{}

var _ = Suite(&AddressSuite{})

var maxVer semver.Version = semver.MustParse("999.0.0")

func (s *AddressSuite) TestAddress(c *C) {
	// bnb tests
	addr, err := NewAddress("bnb1lejrrtta9cgr49fuh7ktu3sddhe0ff7wenlpn6", LatestVersion)
	c.Assert(err, IsNil)
	c.Check(addr.IsEmpty(), Equals, false)
	c.Check(addr.Equals(Address("bnb1lejrrtta9cgr49fuh7ktu3sddhe0ff7wenlpn6")), Equals, true)
	c.Check(addr.String(), Equals, "bnb1lejrrtta9cgr49fuh7ktu3sddhe0ff7wenlpn6")
	c.Check(addr.IsChain(BNBChain, maxVer), Equals, true)
	c.Check(addr.IsChain(ETHChain, maxVer), Equals, false)
	c.Check(addr.IsChain(BTCChain, maxVer), Equals, false)
	c.Check(addr.IsChain(THORChain, maxVer), Equals, false)
	c.Check(addr.GetNetwork(maxVer, BNBChain), Equals, MainNet)

	addr, err = NewAddress("tbnb12ymaslcrhnkj0tvmecyuejdvk25k2nnurqjvyp", LatestVersion)
	c.Check(addr.IsChain(BNBChain, maxVer), Equals, true)
	c.Check(addr.IsChain(ETHChain, maxVer), Equals, false)
	c.Check(addr.IsChain(BTCChain, maxVer), Equals, false)
	c.Check(addr.IsChain(THORChain, maxVer), Equals, false)
	c.Check(addr.GetNetwork(maxVer, BNBChain), Equals, TestNet)

	// random
	c.Check(err, IsNil)
	_, err = NewAddress("1lejrrtta9cgr49fuh7ktu3sddhe0ff7wenlpn6", LatestVersion)
	c.Check(err, NotNil)
	_, err = NewAddress("bnb1lejrrtta9cgr49fuh7ktu3sddhe0ff7wenlpn6X", LatestVersion)
	c.Check(err, NotNil)
	_, err = NewAddress("bogus", LatestVersion)
	c.Check(err, NotNil)
	c.Check(Address("").IsEmpty(), Equals, true)
	c.Check(NoAddress.Equals(Address("")), Equals, true)
	_, err = NewAddress("", LatestVersion)
	c.Assert(err, IsNil)

	// maya tests
	addr, err = NewAddress("maya1kljxxccrheghavaw97u78le6yy3sdj7h6jylf9", LatestVersion)
	c.Assert(err, IsNil)
	c.Check(addr.IsChain(BASEChain, maxVer), Equals, true)
	c.Check(addr.IsChain(THORChain, maxVer), Equals, false)
	c.Check(addr.IsChain(BNBChain, maxVer), Equals, false)
	c.Check(addr.IsChain(ETHChain, maxVer), Equals, false)
	c.Check(addr.IsChain(BTCChain, maxVer), Equals, false)
	c.Check(addr.GetNetwork(maxVer, THORChain), Equals, MainNet)
	addr, err = NewAddress("tmaya1x6m28lezv00ugcahqv5w2eagrm9396j2gf6zjpd4auytle0w", LatestVersion)
	c.Assert(err, IsNil)
	c.Check(addr.IsChain(BASEChain, maxVer), Equals, true)
	c.Check(addr.IsChain(THORChain, maxVer), Equals, false)
	c.Check(addr.IsChain(BNBChain, maxVer), Equals, false)
	c.Check(addr.IsChain(ETHChain, maxVer), Equals, false)
	c.Check(addr.IsChain(BTCChain, maxVer), Equals, false)
	c.Check(addr.GetNetwork(maxVer, BASEChain), Equals, TestNet)

	// thor tests
	addr, err = NewAddress("thor1kljxxccrheghavaw97u78le6yy3sdj7h696nl4", LatestVersion)
	c.Assert(err, IsNil)
	c.Check(addr.IsChain(THORChain, maxVer), Equals, true)
	c.Check(addr.IsChain(BNBChain, maxVer), Equals, false)
	c.Check(addr.IsChain(ETHChain, maxVer), Equals, false)
	c.Check(addr.IsChain(BTCChain, maxVer), Equals, false)
	c.Check(addr.GetNetwork(maxVer, THORChain), Equals, MainNet)
	addr, err = NewAddress("tthor1x6m28lezv00ugcahqv5w2eagrm9396j2gf6zjpd4auf9mv4h", LatestVersion)
	c.Assert(err, IsNil)
	c.Check(addr.IsChain(THORChain, maxVer), Equals, true)
	c.Check(addr.IsChain(BNBChain, maxVer), Equals, false)
	c.Check(addr.IsChain(ETHChain, maxVer), Equals, false)
	c.Check(addr.IsChain(BTCChain, maxVer), Equals, false)
	c.Check(addr.GetNetwork(maxVer, THORChain), Equals, TestNet)

	// eth tests
	addr, err = NewAddress("0x90f2b1ae50e6018230e90a33f98c7844a0ab635a", LatestVersion)
	c.Check(err, IsNil)
	c.Check(addr.IsChain(ETHChain, maxVer), Equals, true)
	c.Check(addr.IsChain(BNBChain, maxVer), Equals, false)
	c.Check(addr.IsChain(BTCChain, maxVer), Equals, false)
	c.Check(addr.IsChain(THORChain, maxVer), Equals, false)
	// wrong length
	_, err = NewAddress("0x90f2b1ae50e6018230e90a33f98c7844a0ab635aaaaaaaaa", LatestVersion)
	c.Check(err, NotNil)

	// good length but not valid hex string
	_, err = NewAddress("0x90f2b1ae50e6018230e90a33f98c7844a0ab63zz", LatestVersion)
	c.Check(err, NotNil)

	// btc tests
	// mainnet p2pkh
	addr, err = NewAddress("1MirQ9bwyQcGVJPwKUgapu5ouK2E2Ey4gX", LatestVersion)
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain, maxVer), Equals, true)
	c.Check(addr.IsChain(ETHChain, maxVer), Equals, false)
	c.Check(addr.IsChain(BNBChain, maxVer), Equals, false)
	c.Check(addr.IsChain(THORChain, maxVer), Equals, false)
	c.Check(addr.GetNetwork(maxVer, BTCChain), Equals, MainNet)

	// tesnet p2pkh
	addr, err = NewAddress("mrX9vMRYLfVy1BnZbc5gZjuyaqH3ZW2ZHz", LatestVersion)
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain, maxVer), Equals, true)
	c.Check(addr.IsChain(ETHChain, maxVer), Equals, false)
	c.Check(addr.IsChain(BNBChain, maxVer), Equals, false)
	c.Check(addr.IsChain(THORChain, maxVer), Equals, false)
	c.Check(addr.GetNetwork(maxVer, BTCChain), Equals, TestNet)

	// mainnet p2pkh
	addr, err = NewAddress("12MzCDwodF9G1e7jfwLXfR164RNtx4BRVG", LatestVersion)
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain, maxVer), Equals, true)
	c.Check(addr.IsChain(ETHChain, maxVer), Equals, false)
	c.Check(addr.IsChain(BNBChain, maxVer), Equals, false)
	c.Check(addr.IsChain(THORChain, maxVer), Equals, false)
	c.Check(addr.GetNetwork(maxVer, BTCChain), Equals, MainNet)

	// mainnet p2sh
	addr, err = NewAddress("3QJmV3qfvL9SuYo34YihAf3sRCW3qSinyC", LatestVersion)
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain, maxVer), Equals, true)
	c.Check(addr.IsChain(ETHChain, maxVer), Equals, false)
	c.Check(addr.IsChain(BNBChain, maxVer), Equals, false)
	c.Check(addr.IsChain(THORChain, maxVer), Equals, false)
	c.Check(addr.GetNetwork(maxVer, BTCChain), Equals, MainNet)

	// mainnet p2sh 2
	addr, err = NewAddress("3NukJ6fYZJ5Kk8bPjycAnruZkE5Q7UW7i8", LatestVersion)
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain, maxVer), Equals, true)
	c.Check(addr.IsChain(ETHChain, maxVer), Equals, false)
	c.Check(addr.IsChain(BNBChain, maxVer), Equals, false)
	c.Check(addr.IsChain(THORChain, maxVer), Equals, false)
	c.Check(addr.GetNetwork(maxVer, BTCChain), Equals, MainNet)

	// testnet p2sh
	addr, err = NewAddress("2NBFNJTktNa7GZusGbDbGKRZTxdK9VVez3n", LatestVersion)
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain, maxVer), Equals, true)
	c.Check(addr.IsChain(ETHChain, maxVer), Equals, false)
	c.Check(addr.IsChain(BNBChain, maxVer), Equals, false)
	c.Check(addr.IsChain(THORChain, maxVer), Equals, false)
	c.Check(addr.GetNetwork(maxVer, BTCChain), Equals, TestNet)

	// mainnet p2pk compressed (0x02)
	addr, err = NewAddress("02192d74d0cb94344c9569c2e77901573d8d7903c3ebec3a957724895dca52c6b4", LatestVersion)
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain, maxVer), Equals, true)
	c.Check(addr.IsChain(ETHChain, maxVer), Equals, false)
	c.Check(addr.IsChain(BNBChain, maxVer), Equals, false)
	c.Check(addr.IsChain(THORChain, maxVer), Equals, false)
	c.Check(addr.GetNetwork(maxVer, BTCChain), Equals, MainNet)

	// mainnet p2pk compressed (0x03)
	addr, err = NewAddress("03b0bd634234abbb1ba1e986e884185c61cf43e001f9137f23c2c409273eb16e65", LatestVersion)
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain, maxVer), Equals, true)
	c.Check(addr.IsChain(ETHChain, maxVer), Equals, false)
	c.Check(addr.IsChain(BNBChain, maxVer), Equals, false)
	c.Check(addr.IsChain(THORChain, maxVer), Equals, false)
	c.Check(addr.GetNetwork(maxVer, BTCChain), Equals, MainNet)

	// mainnet p2pk uncompressed (0x04)
	addr, err = NewAddress("0411db93e1dcdb8a016b49840f8c53bc1eb68a382e97b1482ecad7b148a6909a5cb2"+
		"e0eaddfb84ccf9744464f82e160bfa9b8b64f9d4c03f999b8643f656b412a3", LatestVersion)
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain, maxVer), Equals, true)
	c.Check(addr.IsChain(ETHChain, maxVer), Equals, false)
	c.Check(addr.IsChain(BNBChain, maxVer), Equals, false)
	c.Check(addr.IsChain(THORChain, maxVer), Equals, false)
	c.Check(addr.GetNetwork(maxVer, BTCChain), Equals, MainNet)

	// mainnet p2pk hybrid (0x06)
	addr, err = NewAddress("06192d74d0cb94344c9569c2e77901573d8d7903c3ebec3a957724895dca52c6b4"+
		"0d45264838c0bd96852662ce6a847b197376830160c6d2eb5e6a4c44d33f453e", LatestVersion)
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain, maxVer), Equals, true)
	c.Check(addr.IsChain(ETHChain, maxVer), Equals, false)
	c.Check(addr.IsChain(BNBChain, maxVer), Equals, false)
	c.Check(addr.IsChain(THORChain, maxVer), Equals, false)
	c.Check(addr.GetNetwork(maxVer, BTCChain), Equals, MainNet)

	// mainnet p2pk hybrid (0x07)
	addr, err = NewAddress("07b0bd634234abbb1ba1e986e884185c61cf43e001f9137f23c2c409273eb16e65"+
		"37a576782eba668a7ef8bd3b3cfb1edb7117ab65129b8a2e681f3c1e0908ef7b", LatestVersion)
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain, maxVer), Equals, true)
	c.Check(addr.IsChain(ETHChain, maxVer), Equals, false)
	c.Check(addr.IsChain(BNBChain, maxVer), Equals, false)
	c.Check(addr.IsChain(THORChain, maxVer), Equals, false)
	c.Check(addr.GetNetwork(maxVer, BTCChain), Equals, MainNet)

	// testnet p2pk compressed (0x02)
	addr, err = NewAddress("02192d74d0cb94344c9569c2e77901573d8d7903c3ebec3a957724895dca52c6b4", LatestVersion)
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain, maxVer), Equals, true)
	c.Check(addr.IsChain(ETHChain, maxVer), Equals, false)
	c.Check(addr.IsChain(BNBChain, maxVer), Equals, false)
	c.Check(addr.IsChain(THORChain, maxVer), Equals, false)
	c.Check(addr.GetNetwork(maxVer, BTCChain), Equals, MainNet)

	// segwit mainnet p2wpkh v0
	addr, err = NewAddress("BC1QW508D6QEJXTDG4Y5R3ZARVARY0C5XW7KV8F3T4", LatestVersion)
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain, maxVer), Equals, true)
	c.Check(addr.IsChain(ETHChain, maxVer), Equals, false)
	c.Check(addr.IsChain(BNBChain, maxVer), Equals, false)
	c.Check(addr.IsChain(THORChain, maxVer), Equals, false)
	c.Check(addr.GetNetwork(maxVer, BTCChain), Equals, MainNet)

	// segwit mainnet p2wsh v0
	addr, err = NewAddress("bc1qrp33g0q5c5txsp9arysrx4k6zdkfs4nce4xj0gdcccefvpysxf3qccfmv3", LatestVersion)
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain, maxVer), Equals, true)
	c.Check(addr.IsChain(ETHChain, maxVer), Equals, false)
	c.Check(addr.IsChain(BNBChain, maxVer), Equals, false)
	c.Check(addr.IsChain(THORChain, maxVer), Equals, false)
	c.Check(addr.GetNetwork(maxVer, BTCChain), Equals, MainNet)

	// segwit testnet p2wpkh v0
	addr, err = NewAddress("tb1qw508d6qejxtdg4y5r3zarvary0c5xw7kxpjzsx", LatestVersion)
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain, maxVer), Equals, true)
	c.Check(addr.IsChain(ETHChain, maxVer), Equals, false)
	c.Check(addr.IsChain(BNBChain, maxVer), Equals, false)
	c.Check(addr.IsChain(THORChain, maxVer), Equals, false)
	c.Check(addr.GetNetwork(maxVer, BTCChain), Equals, TestNet)

	// segwit testnet p2wsh witness v0
	addr, err = NewAddress("tb1qqqqqp399et2xygdj5xreqhjjvcmzhxw4aywxecjdzew6hylgvsesrxh6hy", LatestVersion)
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain, maxVer), Equals, true)
	c.Check(addr.IsChain(ETHChain, maxVer), Equals, false)
	c.Check(addr.IsChain(BNBChain, maxVer), Equals, false)
	c.Check(addr.IsChain(THORChain, maxVer), Equals, false)
	c.Check(addr.GetNetwork(maxVer, BTCChain), Equals, TestNet)

	// segwit mainnet witness v1
	addr, err = NewAddress("bc1pw508d6qejxtdg4y5r3zarvary0c5xw7kw508d6qejxtdg4y5r3zarvary0c5xw7k7grplx", LatestVersion)
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain, maxVer), Equals, true)
	c.Check(addr.IsChain(ETHChain, maxVer), Equals, false)
	c.Check(addr.IsChain(BNBChain, maxVer), Equals, false)
	c.Check(addr.IsChain(THORChain, maxVer), Equals, false)
	c.Check(addr.GetNetwork(maxVer, BTCChain), Equals, MainNet)

	// segwit mainnet witness v16
	addr, err = NewAddress("BC1SW50QA3JX3S", LatestVersion)
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain, maxVer), Equals, true)
	c.Check(addr.IsChain(ETHChain, maxVer), Equals, false)
	c.Check(addr.IsChain(BNBChain, maxVer), Equals, false)
	c.Check(addr.IsChain(THORChain, maxVer), Equals, false)
	c.Check(addr.GetNetwork(maxVer, BTCChain), Equals, MainNet)
	addr, err = NewAddress("bcrt1qqqnde7kqe5sf96j6zf8jpzwr44dh4gkd3ehaqh", LatestVersion)
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain, maxVer), Equals, true)
	c.Check(addr.IsChain(ETHChain, maxVer), Equals, false)
	c.Check(addr.IsChain(BNBChain, maxVer), Equals, false)
	c.Check(addr.IsChain(THORChain, maxVer), Equals, false)
	c.Check(addr.GetNetwork(maxVer, BTCChain), Equals, MockNet)

	// segwit invalid hrp bech32 succeed but IsChain fails
	addr, err = NewAddress("tc1qw508d6qejxtdg4y5r3zarvary0c5xw7kg3g4ty", LatestVersion)
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain, maxVer), Equals, false)
	c.Check(addr.IsChain(ETHChain, maxVer), Equals, false)
	c.Check(addr.IsChain(BNBChain, maxVer), Equals, false)
	c.Check(addr.IsChain(THORChain, maxVer), Equals, false)

	// radix tests
	validRadixMainnetAddresses := []string{
		"account_rdx128ewp0axlq9rvna4ehyjkgh3l5uv6mxru6vhcxms7fu0wrfdazdtxj",
		"ardx128ewp0axlq9rvna4ehyjkgh3l5uv6mxru6vhcxms7fu0wrfdazdtxj",
		"rdx128ewp0axlq9rvna4ehyjkgh3l5uv6mxru6vhcxms7fu0wrfdazdtxj",
		"128ewp0axlq9rvna4ehyjkgh3l5uv6mxru6vhcxms7fu0wrfdazdtxj",
		"28ewp0axlq9rvna4ehyjkgh3l5uv6mxru6vhcxms7fu0wrfdazdtxj",
	}
	for _, addrStr := range validRadixMainnetAddresses {
		addr, err = NewAddress(addrStr, LatestVersion)
		c.Assert(err, IsNil)
		c.Check(addr.IsChain(THORChain, maxVer), Equals, false)
		c.Check(addr.IsChain(BASEChain, maxVer), Equals, false)
		c.Check(addr.IsChain(BNBChain, maxVer), Equals, false)
		c.Check(addr.IsChain(ETHChain, maxVer), Equals, false)
		c.Check(addr.IsChain(BTCChain, maxVer), Equals, false)
		c.Check(addr.IsChain(DASHChain, maxVer), Equals, false)
		c.Check(addr.IsChain(KUJIChain, maxVer), Equals, false)
		c.Check(addr.IsChain(XRDChain, maxVer), Equals, true)
		c.Check(addr.GetNetwork(maxVer, XRDChain), Equals, MainNet)
	}
	addr, err = NewAddress("account_rdx128ewp0axlq9rvna4ehyjkgh3l5uv6mxru6vhcxms7fu0wrfdazdtxj", LatestVersion)
	c.Assert(err, IsNil)
	c.Check(addr.GetChain(LatestVersion), Equals, XRDChain)
	c.Check(addr.AbbreviatedString(LatestVersion), Equals, "28ewp0axlq9rvna4ehyjkgh3l5uv6mxru6vhcxms7fu0wrfdazdtxj")

	// ----zcash tests
	// zcash tests
	type addrTest struct {
		addr  string
		valid bool
	}
	zcashAddresses := []addrTest{
		// zcash testnet
		{"t1MMkDXvUzcVwdeMEEE1A874sijWrX3NoSJ", true},
		{"t1fQXP7eqaGTaYmc98vqKH2iTSdMBq8kfdW", true},
		{"t3cFfPt1Bcvgez9ZbMBFWeZsskxTkPzGCow", true},
		// unified ["sapling"]
		{"u1xpa9hdpjkg0e9y9netn20chvuqtl9j7tdx7w5qp2wdcvrr809lqpul2wk53p7c5p900y8qlm4mzy63uu5g4k2w7fhk84jef4cc7fmjqn", false}, // unsupported
		// unified ["sapling", "p2pkh"]
		{"u1pyxxps4psrq2uzrwv2ky5e42dz5d09qp3sens70m5x9x0fkchrawkrwvxh73v9uzt7pt4pps26w5z594zhnz6kwlj7kr8v7qqm0vj0qxwz42mtzsuts7e3slsusef208s4v3g3z89km", false}, // unsupported
		// unified ["orchard", "p2pkh"]
		{"u178zartp5j6894a6frhm2uxajlk5f6swkg8x0d2g2wz37as50sp2updnf330nxfckq0r0dvt4jwkm89jx0748n434ltttum2damkzq40k9xpwxc0gu3mt42hwfmy76sp22dsf5pecu7u", false}, // unsupported
		// unified ["orchard"]
		{"u1k399t95pnvkz46tz632mgfyr38ncs97h7g8cdp6e06ktzasqam7g7n96n3yw4ytva6gepkzjpnph7e8r658vvzqcadhw2lm5x57p0yw4", false}, // unsupported

		// Invalid Addresses:
		// Transparent Address
		{"tmPqRm123fPtBBCbHigHbXCt4dP9B9LA5ua", false},
		{"t2HifwjUj123xr9bknR8LFuQbc98c3vkXtu", false},
		{"tmH5S6sCj6rEc1DFCc1233zn2VTtwwVE4dS", false},
		{"t2HifwjUj9uyxr9bknR8LFuQbc98c3v412344u", false},
		// Unified Address
		{"uregtest177ng78krqe8mz8qhme9lj6jndp58m6z459ve4w08gekk5098y4yeskaghjgt9r123da60875z0hr4k8cftdszhqgxyzwmuse9rqn2f540rnch6ud93lzt5nqv74evamnry726exlyle7wfjwy3ucf4t0tz85vgrpv276sjh5lj7r79qjjd7a8fxrh3s2cpejj8l2m26mr3v7xdy2e04", false},
		{"uregtest1hyj2drnehygun73nk86rtt3z2j27z22kxtc7slrjmhv8ghcrujd444hcrq6alh123fxa68uchymue70899vgazhhx5h890mvufm43uj0ejxvc22zu8hume429fqv9eus4q228869jmdy5jpx4untlu66wyh8mtnj5c39cu7k8kz6nr36nl72jhm922j4086n85gf9l0p04mcvqsnnyl", false},
		// Sapling Shielded Address
		{"zregtestsapling18wz9q62v2t872a0dt70xtrfrk06hpa8a8kgej4hs7xsazk86u9er123kzkkmc9llx2zvz85l232", false},
		{"zregtestsapling1y4hqt7t04wgexam3gkpuadd24hmher52n6hnjt4xxpgehv67z7eyn434mu5pse7fls2u6u9z2z9123", false},
	}

	for _, addrForTest := range zcashAddresses {
		addr, err = NewAddress(addrForTest.addr, LatestVersion)
		if addrForTest.valid {
			c.Assert(err, IsNil)
			c.Check(addr.IsChain(ZECChain, maxVer), Equals, true)
			c.Check(addr.IsChain(BTCChain, maxVer), Equals, false)
			c.Check(addr.IsChain(ETHChain, maxVer), Equals, false)
			c.Check(addr.IsChain(BNBChain, maxVer), Equals, false)
			c.Check(addr.IsChain(THORChain, maxVer), Equals, false)
			c.Check(addr.IsChain(BASEChain, maxVer), Equals, false)
			c.Check(addr.IsChain(DASHChain, maxVer), Equals, false)
			c.Check(addr.IsChain(XRDChain, maxVer), Equals, false)
			c.Check(addr.IsChain(ZECChain, maxVer), Equals, true)
		} else if err != nil {
			c.Check(addr.IsChain(ZECChain, maxVer), Equals, false)
		}
	}
	// ----zcash tests
}
