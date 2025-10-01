package types

import (
	"strings"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/rs/zerolog/log"

	"gitlab.com/mayachain/mayanode/bifrost/mayaclient"
	"gitlab.com/mayachain/mayanode/cmd"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/common/crypto/ed25519"
	"gitlab.com/mayachain/mayanode/config"
)

////////////////////////////////////////////////////////////////////////////////////////
// Account
////////////////////////////////////////////////////////////////////////////////////////

// Account holds a set of chain clients configured with a given private key.
type Account struct {
	// Mayachain is the mayachain client for the account.
	Mayachain mayaclient.MayachainBridge

	// ChainClients is a map of chain to the corresponding client for the account.
	ChainClients map[common.Chain]LiteChainClient

	lock        chan struct{}
	pubkey      common.PubKey
	eddsaPubkey common.PubKey
	mnemonic    string
}

// NewAccount returns a new client using the private key from the given mnemonic.
func NewAccount(mnemonic string, constructors map[common.Chain]LiteChainClientConstructor) *Account {
	// create pubkey for mnemonic
	derivedPriv, err := hd.Secp256k1.Derive()(mnemonic, "", cmd.BASEChainHDPath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to derive private key")
	}
	privKey := hd.Secp256k1.Generate()(derivedPriv)
	s, err := cosmos.Bech32ifyPubKey(cosmos.Bech32PubKeyTypeAccPub, privKey.PubKey())
	if err != nil {
		log.Fatal().Err(err).Msg("failed to bech32ify pubkey")
	}
	pubkey := common.PubKey(s)

	// EdDSA pubkey
	edPriv, err := ed25519.Ed25519.Derive()(mnemonic, "", ed25519.HDPath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to derive ed25519 private key")
	}
	edPrivKey := ed25519.Ed25519.Generate()(edPriv)
	edS, err := cosmos.Bech32ifyPubKey(cosmos.Bech32PubKeyTypeAccPub, edPrivKey.PubKey())
	if err != nil {
		log.Fatal().Err(err).Msg("failed to bech32ify eddsa pubkey")
	}
	edPubkey := common.PubKey(edS)

	// add key to keyring
	kr := keyring.NewInMemory(func(options *keyring.Options) {
		options.SupportedAlgos = keyring.SigningAlgoList{hd.Secp256k1, ed25519.Ed25519}
	})
	name := strings.Split(mnemonic, " ")[0]
	_, err = kr.NewAccount(name, mnemonic, "", cmd.BASEChainHDPath, hd.Secp256k1)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to add account to keyring")
	}

	// Add Ed25519 key to keyring
	_, err = kr.NewAccount(ed25519.SignerNameEDDSA(name), mnemonic, "", ed25519.HDPath, ed25519.Ed25519)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to add account to keyring")
	}

	// create mayaclient.Keys for chain client construction
	keys := mayaclient.NewKeysWithKeybase(kr, name, "")

	// bifrost config for chain client construction
	cfg := config.GetBifrost()

	// create chain clients
	chainClients := make(map[common.Chain]LiteChainClient)
	for chain := range constructors {
		pKeys := keys
		chainClients[chain], err = constructors[chain](chain, pKeys)
		if err != nil {
			log.Fatal().Err(err).Stringer("chain", chain).Msg("failed to create chain client")
		}
	}

	// create mayachain bridge
	mayachainCfg := cfg.MayaChain
	mayachainCfg.SignerName = name
	mayachain, err := mayaclient.NewMayachainBridge(mayachainCfg, nil, keys)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create mayachain client")
	}

	return &Account{
		ChainClients: chainClients,
		Mayachain:    mayachain,
		lock:         make(chan struct{}, 1),
		pubkey:       pubkey,
		eddsaPubkey:  edPubkey,
		mnemonic:     mnemonic,
	}
}

// Name returns the name of the account.
func (a *Account) Name() string {
	return strings.Split(a.mnemonic, " ")[0]
}

// Acquire will attempt to acquire the lock. If the lock is already acquired, it will
// return false. If true is returned, the caller has locked and must release when done.
func (a *Account) Acquire() bool {
	select {
	case a.lock <- struct{}{}:
		return true
	default:
		return false
	}
}

// Release will release the lock.
func (a *Account) Release() {
	<-a.lock
}

// PubKey returns the public key of the client.
func (a *Account) PubKey(chain common.Chain) common.PubKey {
	if chain.GetSigningAlgo() == common.SigningAlgoEd25519 {
		return a.eddsaPubkey
	}
	return a.pubkey
}

// Address returns the address of the client for the given chain.
func (a *Account) Address(chain common.Chain) common.Address {
	if chain.GetSigningAlgo() == common.SigningAlgoEd25519 {
		address, err := a.eddsaPubkey.GetAddress(chain)
		if err != nil {
			log.Fatal().Err(err).Stringer("chain", chain).Msg("failed to get address")
		}
		return address
	}
	address, err := a.pubkey.GetAddress(chain)
	if err != nil {
		log.Fatal().Err(err).Stringer("chain", chain).Msg("failed to get address")
	}
	return address
}
