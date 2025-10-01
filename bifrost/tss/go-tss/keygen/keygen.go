package keygen

import (
	bcrypto "github.com/binance-chain/tss-lib/crypto"

	"gitlab.com/mayachain/mayanode/bifrost/tss/go-tss/common"
	"gitlab.com/mayachain/mayanode/bifrost/tss/go-tss/p2p"
)

type TssKeyGen interface {
	GenerateNewKey(keygenReq Request) (*bcrypto.ECPoint, error)
	GetTssKeyGenChannels() chan *p2p.Message
	GetTssCommonStruct() *common.TssCommon
}
