package keygen

import (
	"gitlab.com/mayachain/mayanode/bifrost/tss/go-tss/blame"
	"gitlab.com/mayachain/mayanode/bifrost/tss/go-tss/common"
	mcommon "gitlab.com/mayachain/mayanode/common"
)

// Response keygen response
type Response struct {
	Algo        mcommon.SigningAlgo `json:"algo"`
	PubKey      string              `json:"pub_key"`
	PoolAddress string              `json:"pool_address"`
	Status      common.Status       `json:"status"`
	Blame       blame.Blame         `json:"blame"`
}

// NewResponse create a new instance of keygen.Response
func NewResponse(algo mcommon.SigningAlgo, pk, addr string, status common.Status, blame blame.Blame) Response {
	return Response{
		Algo:        algo,
		PubKey:      pk,
		PoolAddress: addr,
		Status:      status,
		Blame:       blame,
	}
}
