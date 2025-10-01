package common

import (
	"encoding/json"
	"fmt"

	"github.com/cosmos/cosmos-sdk/testutil/testdata"
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common/cosmos"
)

type KeyDataAddr struct {
	mainnet string
	testnet string
	mocknet string
}

type KeyData struct {
	priv string
	pub  string
	// maya
	addrARB  KeyDataAddr
	addrBTC  KeyDataAddr
	addrDASH KeyDataAddr
	addrETH  KeyDataAddr
	addrKUJI KeyDataAddr
	addrTHOR KeyDataAddr
	addrXRD  KeyDataAddr
	// thor
	addrBNB KeyDataAddr
}

type PubKeyTestSuite struct {
	keyData []KeyData
}

var _ = Suite(&PubKeyTestSuite{})

func (s *PubKeyTestSuite) SetUpSuite(_ *C) {
	s.keyData = []KeyData{
		{
			priv: "ef235aacf90d9f4aadd8c92e4b2562e1d9eb97f0df9ba3b508258739cb013db2",
			pub:  "02b4632d08485ff1df2db55b9dafd23347d1c47a457072a1e87be26896549a8737",
			addrETH: KeyDataAddr{
				mainnet: "0x3fd2d4ce97b082d4bce3f9fee2a3d60668d2f473",
				testnet: "0x3fd2d4ce97b082d4bce3f9fee2a3d60668d2f473",
				mocknet: "0x3fd2d4ce97b082d4bce3f9fee2a3d60668d2f473",
			},
			addrARB: KeyDataAddr{
				mainnet: "0x3fd2d4ce97b082d4bce3f9fee2a3d60668d2f473",
				testnet: "0x3fd2d4ce97b082d4bce3f9fee2a3d60668d2f473",
				mocknet: "0x3fd2d4ce97b082d4bce3f9fee2a3d60668d2f473",
			},
			addrDASH: KeyDataAddr{
				mainnet: "XpANHDZNTgEyM2FN8QjERafkRaK4X8Uaeq",
				testnet: "yZnyJAdouDu3gmAuhG3dTc66hroS4AXxnL",
				mocknet: "yZnyJAdouDu3gmAuhG3dTc66hroS4AXxnL",
			},
			addrKUJI: KeyDataAddr{
				mainnet: "kujira1j08ys4ct2hzzc2hcz6h2hgrvlmsjynawc9h8rq",
				testnet: "kujira1j08ys4ct2hzzc2hcz6h2hgrvlmsjynawc9h8rq",
				mocknet: "kujira1j08ys4ct2hzzc2hcz6h2hgrvlmsjynawc9h8rq",
			},
			addrTHOR: KeyDataAddr{
				mainnet: "thor1j08ys4ct2hzzc2hcz6h2hgrvlmsjynaw02vym4",
				testnet: "tthor1j08ys4ct2hzzc2hcz6h2hgrvlmsjynawtaa5zs",
				mocknet: "tthor1j08ys4ct2hzzc2hcz6h2hgrvlmsjynawtaa5zs",
			},
			addrBTC: KeyDataAddr{
				mainnet: "bc1qj08ys4ct2hzzc2hcz6h2hgrvlmsjynawlht528",
				testnet: "tb1qj08ys4ct2hzzc2hcz6h2hgrvlmsjynaw43s835",
				mocknet: "bcrt1qj08ys4ct2hzzc2hcz6h2hgrvlmsjynawhcf2xa",
			},
			addrBNB: KeyDataAddr{
				mainnet: "bnb1j08ys4ct2hzzc2hcz6h2hgrvlmsjynawtf2n0y",
				testnet: "tbnb1j08ys4ct2hzzc2hcz6h2hgrvlmsjynaw9urh04",
				mocknet: "tbnb1j08ys4ct2hzzc2hcz6h2hgrvlmsjynaw9urh04",
			},
			addrXRD: KeyDataAddr{
				mainnet: "account_rdx16xk2jg3k2sz34r0ven7223pmjhnytu2j9q002qprdt976jhe4ltlpu",
				testnet: "account_tdx_18_16xk2jg3k2sz34r0ven7223pmjhnytu2j9q002qprdt976jheth6443",
				mocknet: "account_loc16xk2jg3k2sz34r0ven7223pmjhnytu2j9q002qprdt976jhe6zhn4a",
			},
		},
		{
			priv: "289c2857d4598e37fb9647507e47a309d6133539bf21a8b9cb6df88fd5232032",
			pub:  "037db227d7094ce215c3a0f57e1bcc732551fe351f94249471934567e0f5dc1bf7",
			addrETH: KeyDataAddr{
				mainnet: "0x970e8128ab834e8eac17ab8e3812f010678cf791",
				testnet: "0x970e8128ab834e8eac17ab8e3812f010678cf791",
				mocknet: "0x970e8128ab834e8eac17ab8e3812f010678cf791",
			},
			addrARB: KeyDataAddr{
				mainnet: "0x970e8128ab834e8eac17ab8e3812f010678cf791",
				testnet: "0x970e8128ab834e8eac17ab8e3812f010678cf791",
				mocknet: "0x970e8128ab834e8eac17ab8e3812f010678cf791",
			},
			addrTHOR: KeyDataAddr{
				mainnet: "thor1zupk5lmc84r2dh738a9g3zscavannjy3h4s0hw",
				testnet: "tthor1zupk5lmc84r2dh738a9g3zscavannjy3nzplwt",
				mocknet: "tthor1zupk5lmc84r2dh738a9g3zscavannjy3nzplwt",
			},
			addrKUJI: KeyDataAddr{
				mainnet: "kujira1zupk5lmc84r2dh738a9g3zscavannjy3q6tv0m",
				testnet: "kujira1zupk5lmc84r2dh738a9g3zscavannjy3q6tv0m",
				mocknet: "kujira1zupk5lmc84r2dh738a9g3zscavannjy3q6tv0m",
			},
			addrBNB: KeyDataAddr{
				mainnet: "bnb1zupk5lmc84r2dh738a9g3zscavannjy3nkkcrl",
				testnet: "tbnb1zupk5lmc84r2dh738a9g3zscavannjy3arlurw",
				mocknet: "tbnb1zupk5lmc84r2dh738a9g3zscavannjy3arlurw",
			},
			addrBTC: KeyDataAddr{
				mainnet: "bc1qzupk5lmc84r2dh738a9g3zscavannjy38ghlxu",
				testnet: "tb1qzupk5lmc84r2dh738a9g3zscavannjy3dwvva0",
				mocknet: "bcrt1qzupk5lmc84r2dh738a9g3zscavannjy3084p2x",
			},
			addrDASH: KeyDataAddr{
				mainnet: "XcnXU1m6d1pkoCd17SgknUdgV1dbPvBrcU",
				testnet: "yNR8UxqY4ZUq8wYYgJ19pW42mJ7xt2J8FU",
				mocknet: "yNR8UxqY4ZUq8wYYgJ19pW42mJ7xt2J8FU",
			},
			addrXRD: KeyDataAddr{
				mainnet: "account_rdx169uvf7dwf9glm5fmkq3ra5vv3v5pjdaekalc4at7xs0568h6c7mtm4",
				testnet: "account_tdx_18_169uvf7dwf9glm5fmkq3ra5vv3v5pjdaekalc4at7xs0568h6xk2p0c",
				mocknet: "account_loc169uvf7dwf9glm5fmkq3ra5vv3v5pjdaekalc4at7xs0568h6hr8805",
			},
		},
		{
			priv: "e810f1d7d6691b4a7a73476f3543bd87d601f9a53e7faf670eac2c5b517d83bf",
			pub:  "03f98464e8d3fc8e275e34c6f8dc9b99aa244e37b0d695d0dfb8884712ed6d4d35",
			addrETH: KeyDataAddr{
				mainnet: "0xf6da288748ec4c77642f6c5543717539b3ae001b",
				testnet: "0xf6da288748ec4c77642f6c5543717539b3ae001b",
				mocknet: "0xf6da288748ec4c77642f6c5543717539b3ae001b",
			},
			addrARB: KeyDataAddr{
				mainnet: "0xf6da288748ec4c77642f6c5543717539b3ae001b",
				testnet: "0xf6da288748ec4c77642f6c5543717539b3ae001b",
				mocknet: "0xf6da288748ec4c77642f6c5543717539b3ae001b",
			},
			addrTHOR: KeyDataAddr{
				mainnet: "thor1qqnde7kqe5sf96j6zf8jpzwr44dh4gkdftjnal",
				testnet: "tthor1qqnde7kqe5sf96j6zf8jpzwr44dh4gkddurry6",
				mocknet: "tthor1qqnde7kqe5sf96j6zf8jpzwr44dh4gkddurry6",
			},
			addrKUJI: KeyDataAddr{
				mainnet: "kujira1qqnde7kqe5sf96j6zf8jpzwr44dh4gkd7yfs92",
				testnet: "kujira1qqnde7kqe5sf96j6zf8jpzwr44dh4gkd7yfs92",
				mocknet: "kujira1qqnde7kqe5sf96j6zf8jpzwr44dh4gkd7yfs92",
			},
			addrBNB: KeyDataAddr{
				mainnet: "bnb1qqnde7kqe5sf96j6zf8jpzwr44dh4gkddg5yfw",
				testnet: "tbnb1qqnde7kqe5sf96j6zf8jpzwr44dh4gkdraaqfl",
				mocknet: "tbnb1qqnde7kqe5sf96j6zf8jpzwr44dh4gkdraaqfl",
			},
			addrBTC: KeyDataAddr{
				mainnet: "bc1qqqnde7kqe5sf96j6zf8jpzwr44dh4gkdek4rvd",
				testnet: "tb1qqqnde7kqe5sf96j6zf8jpzwr44dh4gkdnswsh7",
				mocknet: "bcrt1qqqnde7kqe5sf96j6zf8jpzwr44dh4gkd3ehaqh",
			},
			addrDASH: KeyDataAddr{
				mainnet: "XahePWt94WHuFrfABrfAaqDKZWvqEjuqVV",
				testnet: "yLLFQTxaW3wybbahkhyZcrdfqoRCnfzAV5",
				mocknet: "yLLFQTxaW3wybbahkhyZcrdfqoRCnfzAV5",
			},
			addrXRD: KeyDataAddr{
				mainnet: "account_rdx168l0lxnq5g03cc30d8eq3pzy8ur5v37yhqnfgje89xwxuvx8xd6tlp",
				testnet: "account_tdx_18_168l0lxnq5g03cc30d8eq3pzy8ur5v37yhqnfgje89xwxuvx8c9tptv",
				mocknet: "account_loc168l0lxnq5g03cc30d8eq3pzy8ur5v37yhqnfgje89xwxuvx8fsx8tq",
			},
		},
		{
			priv: "a96e62ed3955e65be32703f12d87b6b5cf26039ecfa948dc5107a495418e5330",
			pub:  "02950e1cdfcb133d6024109fd489f734eeb4502418e538c28481f22bce276f248c",
			addrETH: KeyDataAddr{
				mainnet: "0xfabb9cc6ec839b1214bb11c53377a56a6ed81762",
				testnet: "0xfabb9cc6ec839b1214bb11c53377a56a6ed81762",
				mocknet: "0xfabb9cc6ec839b1214bb11c53377a56a6ed81762",
			},
			addrARB: KeyDataAddr{
				mainnet: "0xfabb9cc6ec839b1214bb11c53377a56a6ed81762",
				testnet: "0xfabb9cc6ec839b1214bb11c53377a56a6ed81762",
				mocknet: "0xfabb9cc6ec839b1214bb11c53377a56a6ed81762",
			},
			addrTHOR: KeyDataAddr{
				mainnet: "thor10s4mg25tu6termrk8egltfyme4q7sg3hlyn2sr",
				testnet: "tthor10s4mg25tu6termrk8egltfyme4q7sg3hmnz6fx",
				mocknet: "tthor10s4mg25tu6termrk8egltfyme4q7sg3hmnz6fx",
			},
			addrKUJI: KeyDataAddr{
				mainnet: "kujira10s4mg25tu6termrk8egltfyme4q7sg3hgtgfgk",
				testnet: "kujira10s4mg25tu6termrk8egltfyme4q7sg3hgtgfgk",
				mocknet: "kujira10s4mg25tu6termrk8egltfyme4q7sg3hgtgfgk",
			},
			addrBNB: KeyDataAddr{
				mainnet: "bnb10s4mg25tu6termrk8egltfyme4q7sg3hm84ayj",
				testnet: "tbnb10s4mg25tu6termrk8egltfyme4q7sg3h4jueyr",
				mocknet: "tbnb10s4mg25tu6termrk8egltfyme4q7sg3h4jueyr",
			},
			addrBTC: KeyDataAddr{
				mainnet: "bc1q0s4mg25tu6termrk8egltfyme4q7sg3h0e56p3",
				testnet: "tb1q0s4mg25tu6termrk8egltfyme4q7sg3h9l0f6z",
				mocknet: "bcrt1q0s4mg25tu6termrk8egltfyme4q7sg3h8kkydt",
			},
			addrDASH: KeyDataAddr{
				mainnet: "Xn1PydbxxPrTHq4heBc2jr81gQ9qcdUp1X",
				testnet: "yXdzzagQPwWXdZzFD2vRmsYMxgeD6o9D2L",
				mocknet: "yXdzzagQPwWXdZzFD2vRmsYMxgeD6o9D2L",
			},
			addrXRD: KeyDataAddr{
				mainnet: "account_rdx169py5t2f9kvx9hlpxm5sf636wusxcjqm3v2kwykpaa62g7flke47x5",
				testnet: "account_tdx_18_169py5t2f9kvx9hlpxm5sf636wusxcjqm3v2kwykpaa62g7flg3y5je",
				mocknet: "account_loc169py5t2f9kvx9hlpxm5sf636wusxcjqm3v2kwykpaa62g7fleyfjj4",
			},
		},
		{
			priv: "9294f4d108465fd293f7fe299e6923ef71a77f2cb1eb6d4394839c64ec25d5c0",
			pub:  "0238383ee4d60176d27cf46f0863bfc6aea624fe9bfc7f4273cc5136d9eb483e4a",
			addrETH: KeyDataAddr{
				mainnet: "0x1f30a82340f08177aba70e6f48054917c74d7d38",
				testnet: "0x1f30a82340f08177aba70e6f48054917c74d7d38",
				mocknet: "0x1f30a82340f08177aba70e6f48054917c74d7d38",
			},
			addrARB: KeyDataAddr{
				mainnet: "0x1f30a82340f08177aba70e6f48054917c74d7d38",
				testnet: "0x1f30a82340f08177aba70e6f48054917c74d7d38",
				mocknet: "0x1f30a82340f08177aba70e6f48054917c74d7d38",
			},
			addrTHOR: KeyDataAddr{
				mainnet: "thor1jw8h4l3dtz5xxc7uyh5ys70qkezspgfunk4qmn",
				testnet: "tthor1jw8h4l3dtz5xxc7uyh5ys70qkezspgfuhpyszk",
				mocknet: "tthor1jw8h4l3dtz5xxc7uyh5ys70qkezspgfuhpyszk",
			},
			addrKUJI: KeyDataAddr{
				mainnet: "kujira1jw8h4l3dtz5xxc7uyh5ys70qkezspgfuyewrrx",
				testnet: "kujira1jw8h4l3dtz5xxc7uyh5ys70qkezspgfuyewrrx",
				mocknet: "kujira1jw8h4l3dtz5xxc7uyh5ys70qkezspgfuyewrrx",
			},
			addrBNB: KeyDataAddr{
				mainnet: "bnb1jw8h4l3dtz5xxc7uyh5ys70qkezspgfuh4nh0z",
				testnet: "tbnb1jw8h4l3dtz5xxc7uyh5ys70qkezspgfueq6n0n",
				mocknet: "tbnb1jw8h4l3dtz5xxc7uyh5ys70qkezspgfueq6n0n",
			},
			addrBTC: KeyDataAddr{
				mainnet: "bc1qjw8h4l3dtz5xxc7uyh5ys70qkezspgfurtjs2p",
				testnet: "tb1qjw8h4l3dtz5xxc7uyh5ys70qkezspgfufdfr3j",
				mocknet: "bcrt1qjw8h4l3dtz5xxc7uyh5ys70qkezspgfutyswxm",
			},
			addrDASH: KeyDataAddr{
				mainnet: "Xp953eZGiFzHfr5wHuA9DBTzxS37JMNykT",
				testnet: "yZmg4bdi9oeN1b1UrkUYFCtMEiXUoSazYy",
				mocknet: "yZmg4bdi9oeN1b1UrkUYFCtMEiXUoSazYy",
			},
			addrXRD: KeyDataAddr{
				mainnet: "account_rdx168zrqawc4z4wutaenmxvmczgn8pxusp285dy6sqdmje5tydgkdtu8s",
				testnet: "account_tdx_18_168zrqawc4z4wutaenmxvmczgn8pxusp285dy6sqdmje5tydgg96kna",
				mocknet: "account_loc168zrqawc4z4wutaenmxvmczgn8pxusp285dy6sqdmje5tydgeshsn3",
			},
		},
	}
}

// TestPubKey implementation
func (s *PubKeyTestSuite) TestPubKey(c *C) {
	_, pubKey, _ := testdata.KeyTestPubAddr()
	spk, err := cosmos.Bech32ifyPubKey(cosmos.Bech32PubKeyTypeAccPub, pubKey)
	c.Assert(err, IsNil)
	pk, err := NewPubKey(spk)
	c.Assert(err, IsNil)
	hexStr := pk.String()
	c.Assert(len(hexStr) > 0, Equals, true)
	pk1, err := NewPubKey(hexStr)
	c.Assert(err, IsNil)
	c.Assert(pk.Equals(pk1), Equals, true)

	result, err := json.Marshal(pk)
	c.Assert(err, IsNil)
	c.Log(result, Equals, fmt.Sprintf(`"%s"`, hexStr))
	var pk2 PubKey
	err = json.Unmarshal(result, &pk2)
	c.Assert(err, IsNil)
	c.Assert(pk2.Equals(pk), Equals, true)
}

func (s *PubKeyTestSuite) TestPubKeySet(c *C) {
	_, pubKey, _ := testdata.KeyTestPubAddr()
	spk, err := cosmos.Bech32ifyPubKey(cosmos.Bech32PubKeyTypeAccPub, pubKey)
	c.Assert(err, IsNil)
	pk, err := NewPubKey(spk)
	c.Assert(err, IsNil)

	c.Check(PubKeySet{}.Contains(pk), Equals, false)

	pks := PubKeySet{
		Secp256k1: pk,
	}
	c.Check(pks.Contains(pk), Equals, true)
	pks = PubKeySet{
		Ed25519: pk,
	}
	c.Check(pks.Contains(pk), Equals, true)
}

func (s *PubKeyTestSuite) TestEquals(c *C) {
	var pk1, pk2, pk3, pk4 PubKey
	_, pubKey1, _ := testdata.KeyTestPubAddr()
	tpk1, err1 := cosmos.Bech32ifyPubKey(cosmos.Bech32PubKeyTypeAccPub, pubKey1)
	c.Assert(err1, IsNil)
	pk1 = PubKey(tpk1)

	_, pubKey2, _ := testdata.KeyTestPubAddr()
	tpk2, err2 := cosmos.Bech32ifyPubKey(cosmos.Bech32PubKeyTypeAccPub, pubKey2)
	c.Assert(err2, IsNil)
	pk2 = PubKey(tpk2)

	_, pubKey3, _ := testdata.KeyTestPubAddr()
	tpk3, err3 := cosmos.Bech32ifyPubKey(cosmos.Bech32PubKeyTypeAccPub, pubKey3)
	c.Assert(err3, IsNil)
	pk3 = PubKey(tpk3)

	_, pubKey4, _ := testdata.KeyTestPubAddr()
	tpk4, err4 := cosmos.Bech32ifyPubKey(cosmos.Bech32PubKeyTypeAccPub, pubKey4)
	c.Assert(err4, IsNil)
	pk4 = PubKey(tpk4)

	c.Assert(PubKeys{
		pk1, pk2,
	}.Equals(nil), Equals, false)

	c.Assert(PubKeys{
		pk1, pk2, pk3,
	}.Equals(PubKeys{
		pk1, pk2,
	}), Equals, false)

	c.Assert(PubKeys{
		pk1, pk2, pk3, pk4,
	}.Equals(PubKeys{
		pk4, pk3, pk2, pk1,
	}), Equals, true)

	c.Assert(PubKeys{ // nolint
		pk1, pk2, pk3, pk4,
	}.Equals(PubKeys{
		pk1, pk2, pk3, pk4,
	}), Equals, true)
}
