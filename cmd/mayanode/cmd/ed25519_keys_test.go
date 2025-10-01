package cmd

import (
	stded25519 "crypto/ed25519"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/codec"
	bech32 "github.com/cosmos/cosmos-sdk/types/bech32/legacybech32" // nolint
	cmted25519 "github.com/tendermint/tendermint/crypto/ed25519"
	"gitlab.com/mayachain/mayanode/common/crypto/ed25519"
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/x/mayachain"
)

func TestPackage(t *testing.T) { TestingT(t) }

type ED25519TestSuite struct{}

var _ = Suite(&ED25519TestSuite{})

func (s *ED25519TestSuite) SetUpTest(c *C) {
	mayachain.SetupConfigForTest()
}

func (*ED25519TestSuite) TestGetEd25519Keys(c *C) {
	mayachain.SetupConfigForTest()
	mnemonic := `grape safe sound obtain bachelor festival profit iron meat moon exit garbage chapter promote noble grocery blood letter junk click mesh arm shop decorate`
	result, err := ed25519.DeriveKeypairFromMnemonic(mnemonic, "", ed25519.HDPath)
	c.Assert(err, IsNil)
	// now we test the ed25519 key can sign and verify
	pk := stded25519.PrivateKey(result)
	pub, ok := pk.Public().(stded25519.PublicKey)
	c.Assert(ok, Equals, true)
	pkey := cmted25519.PubKey(pub)
	tmp, err := codec.FromTmPubKeyInterface(pkey)
	c.Assert(err, IsNil)
	// nolint
	pubKey, err := bech32.MarshalPubKey(bech32.AccPK, tmp)
	c.Assert(err, IsNil)
	c.Assert(pubKey, Equals, "tmayapub1zcjduepqrcthx0ke3r2z39rp42xrr777af7qfcs6wcxtxck6tj9j0ap8cl0qlgpdy0")

	mnemonic = `invalid grape safe sound obtain bachelor festival profit iron meat moon exit garbage chapter promote noble grocery blood letter junk click mesh arm shop decorate`
	result, err = ed25519.DeriveKeypairFromMnemonic(mnemonic, "", ed25519.HDPath)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)
}
