package dash

import (
	"fmt"

	"github.com/btcsuite/btcd/btcec"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	dashec "gitlab.com/mayachain/dashd-go/btcec"
	"gitlab.com/mayachain/mayanode/bifrost/tss"
	"gitlab.com/mayachain/mayanode/bifrost/txscript/txscript"
	"gitlab.com/mayachain/mayanode/common"
)

// KeySignWrapper is a wrap of private key and also tss instance
// it also implement the txscript.Signable interface, and will decide which method to use based on the pubkey
type KeySignWrapper struct {
	privateKey    *dashec.PrivateKey
	pubKey        common.PubKey
	tssKeyManager tss.ThorchainKeyManager
	logger        zerolog.Logger
}

// NewKeySignWrapper create a new instance of Keysign Wrapper
func NewKeySignWrapper(privateKey *dashec.PrivateKey, tssKeyManager tss.ThorchainKeyManager) (*KeySignWrapper, error) {
	pubKey, err := GetBech32AccountPubKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("fail to get the pubkey: %w", err)
	}
	return &KeySignWrapper{
		privateKey:    privateKey,
		pubKey:        pubKey,
		tssKeyManager: tssKeyManager,
		logger:        log.With().Str("module", "keysign_wrapper").Logger(),
	}, nil
}

// GetBech32AccountPubKey convert the given private key to
func GetBech32AccountPubKey(key *dashec.PrivateKey) (common.PubKey, error) {
	buf := key.PubKey().SerializeCompressed()
	pk := secp256k1.PubKey(buf)
	return common.NewPubKeyFromCrypto(pk)
}

// GetSignable based on the given poolPubKey
func (w *KeySignWrapper) GetSignable(poolPubKey common.PubKey) txscript.Signable {
	if w.pubKey.Equals(poolPubKey) {
		v1PrivateKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), w.privateKey.Serialize())
		return txscript.NewPrivateKeySignable(v1PrivateKey)
	}
	s, err := NewTssSignable(poolPubKey, w.tssKeyManager)
	if err != nil {
		w.logger.Err(err).Msg("fail to create tss signable")
		return nil
	}
	return s
}
