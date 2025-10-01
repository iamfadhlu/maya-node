package radix

import (
	"crypto/ecdsa"
	"errors"
	"fmt"

	"github.com/hashicorp/go-multierror"
	"gitlab.com/mayachain/mayanode/bifrost/mayaclient"
	stypes "gitlab.com/mayachain/mayanode/bifrost/mayaclient/types"

	ret "github.com/radixdlt/radix-engine-toolkit-go/v2/radix_engine_toolkit_uniffi"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/mayachain/mayanode/bifrost/tss"
	"gitlab.com/mayachain/mayanode/common"
)

type SignerHelper struct {
	privKey       *ecdsa.PrivateKey
	pubKey        common.PubKey
	tssKeyManager tss.ThorchainKeyManager
	mayaBridge    mayaclient.MayachainBridge
	logger        zerolog.Logger
}

func NewSignerHelper(privateKey *ecdsa.PrivateKey, pubKey common.PubKey, keyManager tss.ThorchainKeyManager, mayaBridge mayaclient.MayachainBridge) *SignerHelper {
	return &SignerHelper{
		privKey:       privateKey,
		pubKey:        pubKey,
		tssKeyManager: keyManager,
		logger:        log.With().Str("module", "signer").Str("chain", common.XRDChain.String()).Logger(),
		mayaBridge:    mayaBridge,
	}
}

func (h *SignerHelper) Sign(txOutItem stypes.TxOutItem, signedIntent *ret.SignedIntent, poolPubKey common.PubKey, mayaHeight int64) (*ret.NotarizedTransaction, error) {
	if signedIntent == nil {
		return nil, errors.New("tx is nil")
	}
	if poolPubKey.IsEmpty() {
		return nil, errors.New("pool public key is empty")
	}

	signedIntentHash, err := signedIntent.Hash()
	if err != nil {
		return nil, errors.New("failed to get signed intent hash")
	}

	var signature ret.Signature
	if h.pubKey.Equals(poolPubKey) {
		// The transaction should be signed with our local key, so just sign
		retPrivKey, err := ret.NewPrivateKey(h.privKey.D.Bytes(), ret.CurveSecp256k1)
		if err != nil {
			return nil, errors.New("failed to create RET private key")
		}
		signature = retPrivKey.SignToSignature(signedIntentHash.AsHash())
	} else {
		// The transaction should be signed with a TSS key
		sigBytes, err := h.signTSS(signedIntentHash.Bytes(), poolPubKey)
		if err != nil {
			var keysignError tss.KeysignError
			if errors.As(err, &keysignError) {
				if len(keysignError.Blame.BlameNodes) == 0 {
					return nil, err
				}
				txID, errPostKeysignFail := h.mayaBridge.PostKeysignFailure(keysignError.Blame, mayaHeight, txOutItem.Memo, txOutItem.Coins, poolPubKey)
				if errPostKeysignFail != nil {
					h.logger.Error().Err(errPostKeysignFail).Msg("fail to post keysign failure to maya")
					return nil, multierror.Append(err, errPostKeysignFail)
				}
				h.logger.Info().Str("tx_id", txID.String()).Msgf("post keysign failure to maya")
			}
			return nil, fmt.Errorf("fail to TSS sign: %w", err)
		}
		signature = ret.SignatureSecp256k1{Value: sigBytes}
	}

	notarizedTransaction := ret.NewNotarizedTransaction(signedIntent, signature)

	return notarizedTransaction, nil
}

func (h *SignerHelper) signTSS(hashToSign []byte, poolPubKey common.PubKey) ([]byte, error) {
	sig, recovery, err := h.tssKeyManager.RemoteSign(hashToSign, common.SigningAlgoSecp256k1, poolPubKey.String())
	if err != nil {
		return nil, err
	}

	if sig == nil {
		return nil, fmt.Errorf("fail to TSS sign: %w", err)
	}

	result := make([]byte, 65)
	result[0] = recovery[0] // The first byte is the recovery byte...
	copy(result[1:], sig)   // ...and then comes the signature.
	return result, nil
}
