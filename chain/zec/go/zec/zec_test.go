package zec_test

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/btcsuite/btcd/btcec"
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/chain/zec/go/zec"
)

func Test(t *testing.T) { TestingT(t) }

type ZecSuite struct{}

var _ = Suite(&ZecSuite{})

func (s ZecSuite) SetUpSuite(c *C) {
	fmt.Println("Initializing Rust logger for tests...")
	c.Assert(zec.InitZec(), IsNil)
}

var testPtx zec.PartialTx = zec.PartialTx{
	Height: 211,
	Txid:   nil,
	Inputs: []zec.Utxo{
		{
			Txid:   "eaa37079406851ba2269a213f2e75c78950d1e8f44eb593f5ee1ea46f7f0cfd9",
			Height: 182,
			Vout:   0,
			Script: "76a91422db56f1d0f603ca56d85c2cb7b65d18c2802d1488ac",
			Value:  1500000000,
		},
	},
	Outputs: []zec.Output{
		{
			Address: "tmNUFAr71YAW3eXetm8fhx7k8zpUJYQiKZP",
			Amount:  100,
			Memo:    "",
		},
		{
			Address: "tmCtf4gJ8eb8mXh7hBk6DVW2byu41wPo1GN",
			Amount:  1499989900,
			Memo:    "",
		},
	},
	Fee:          10000,
	ExpiryHeight: 211,
}

func signSighashWithPrivKey(c *C, hash []byte) []byte {
	privBytes, err := hex.DecodeString("359cbcfd5d68301339cc2a5cf198b3cb1d60c98378f96dd38f81dbd52aaf276c")
	c.Assert(err, IsNil)
	pkey, _ := btcec.PrivKeyFromBytes(btcec.S256(), privBytes)
	sig, err := pkey.Sign(hash)
	c.Assert(err, IsNil)
	return sig.Serialize()
}

func (s ZecSuite) TestRustFunctions(c *C) {
	pubKeyBytes, err := hex.DecodeString("0390fe79d1719ab60369c9e710739aefb9d7db411733aa711b467b60232f47d81f")
	c.Assert(err, IsNil)

	// test ComputeTxid
	expectedTxid := "e7deb9a4bdf73c4ddcc94cb3023ecc9b829ee5835b308d05e8783272ccb2d9d1"
	ptx := testPtx
	var gotTxid string
	gotTxid, err = zec.ComputeTxid(pubKeyBytes, ptx, zec.NetworkRegtest)
	c.Assert(err, IsNil)
	c.Assert(gotTxid, Equals, expectedTxid)

	// test BuildPtx
	expectedSigHash := "40a2165b0a296ed39b034f0adc93249fa2f2bb44c4123d26f01e7c9a164465f8"
	ptx = testPtx
	ptx, err = zec.BuildPtx(pubKeyBytes, ptx, zec.NetworkRegtest)
	c.Assert(err, IsNil)
	gotTxid = hex.EncodeToString(ptx.Txid)
	c.Assert(gotTxid, Equals, expectedTxid)
	c.Assert(ptx.Sighashes, HasLen, 1)
	gotSighash := hex.EncodeToString(ptx.Sighashes[0])
	c.Assert(gotSighash, Equals, expectedSigHash)

	// test ApplySignatures
	signature := signSighashWithPrivKey(c, ptx.Sighashes[0])
	c.Assert(err, IsNil)
	c.Assert(err, IsNil)
	var tx []byte
	tx, err = zec.ApplySignatures(pubKeyBytes, ptx, [][]byte{signature}, zec.NetworkRegtest)
	c.Assert(err, IsNil)
	expectedTx := "050000800a27a7265510e7c800000000d300000001d9cff0f746eae15e3f59eb448f1e0d95785ce7f213a26922ba5168407970a3ea000000006b483045022100f6f3ab40a6943c28a0cc5f0507fee3db290d635d84c6762b8e192370a6596d4d022003ccba38f1fe3ddd6f671bb8e7fa0f97309a97ed77e169416e6c8871734763fa01210390fe79d1719ab60369c9e710739aefb9d7db411733aa711b467b60232f47d81fffffffff0264000000000000001976a9148beeab42389192446176efebc352684fc6a095fa88ac8c076859000000001976a91422db56f1d0f603ca56d85c2cb7b65d18c2802d1488ac000000"
	c.Assert(hex.EncodeToString(tx), Equals, expectedTx)
}

func (s ZecSuite) TestSeconInitZec(c *C) {
	c.Assert(zec.InitZec(), IsNil)
}

func (s ZecSuite) TestValidateAddress(c *C) {
	type addrTest struct {
		addr  string
		valid bool
	}
	zcashAddresses := []addrTest{
		// zcash mainnet
		// Transparent Address
		{"tmPqRmAf3fPtBBCbHigHbXCt4dP9B9LA5ua", true},
		{"t2HifwjUj9uyxr9bknR8LFuQbc98c3vkXtu", true},
		{"tmH5S6sCj6rEc1DFCc9jX3zn2VTtwwVE4dS", true},
		{"t2HifwjUj9uyxr9bknR8LFuQbc98c3vkXtu", true},
		// Unified Address
		{"uregtest177ng78krqe8mz8qhme9lj6jndp58m6z459ve4w08gekk5098y4yeskaghjgt9ruz6da60875z0hr4k8cftdszhqgxyzwmuse9rqn2f540rnch6ud93lzt5nqv74evamnry726exlyle7wfjwy3ucf4t0tz85vgrpv276sjh5lj7r79qjjd7a8fxrh3s2cpejj8l2m26mr3v7xdy2e04", false}, // unsupported
		{"uregtest1hyj2drnehygun73nk86rtt3z2j27z22kxtc7slrjmhv8ghcrujd444hcrq6alhmwdfxa68uchymue70899vgazhhx5h890mvufm43uj0ejxvc22zu8hume429fqv9eus4q228869jmdy5jpx4untlu66wyh8mtnj5c39cu7k8kz6nr36nl72jhm922j4086n85gf9l0p04mcvqsnnyl", false}, // unsupported
		// Sapling Shielded Address

		// Regtest Sapling Address
		{"zregtestsapling18wz9q62v2t872a0dt70xtrfrk06hpa8a8kgej4hs7xsazk86u9er3xlkzkkmc9llx2zvz85l232", false}, // unsupported
		{"zregtestsapling1y4hqt7t04wgexam3gkpuadd24hmher52n6hnjt4xxpgehv67z7eyn434mu5pse7fls2u6u9z2z9", false}, // unsupported
		{"zregtestsapling1pd6lvfn68meaxda353na8vq30vsxppps7wgsualz6whwcjptgnt6ljftvp4w3q2rn9a9v6kk6zk", false}, // unsupported
		{"zregtestsapling1weyf5psjm58zlxugp5gt6gn3c7g20x4h4aqvufcyt50fk3g60wj2w2cx29ukw8mmsf0p7glcpvk", false}, // unsupported

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

	// run the test
	for _, addrForTest := range zcashAddresses {
		addr := addrForTest.addr
		err := zec.ValidateAddress(addr, zec.NetworkRegtest)
		if addrForTest.valid {
			c.Assert(err, IsNil)
		} else {
			c.Assert(err, NotNil)
		}
	}
}
