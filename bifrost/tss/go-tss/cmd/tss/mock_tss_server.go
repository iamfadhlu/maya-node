package main

import (
	"errors"

	"gitlab.com/mayachain/mayanode/bifrost/tss/go-tss/blame"
	"gitlab.com/mayachain/mayanode/bifrost/tss/go-tss/common"
	"gitlab.com/mayachain/mayanode/bifrost/tss/go-tss/conversion"
	"gitlab.com/mayachain/mayanode/bifrost/tss/go-tss/keygen"
	"gitlab.com/mayachain/mayanode/bifrost/tss/go-tss/keysign"
	"gitlab.com/mayachain/mayanode/bifrost/tss/go-tss/tss"
	mcommon "gitlab.com/mayachain/mayanode/common"
)

type MockTssServer struct {
	failToStart   bool
	failToKeyGen  bool
	failToKeySign bool
}

func (mts *MockTssServer) Start() error {
	if mts.failToStart {
		return errors.New("you ask for it")
	}
	return nil
}

func (mts *MockTssServer) Stop() {
}

func (mts *MockTssServer) GetLocalPeerID() string {
	return conversion.GetRandomPeerID().String()
}

func (mts *MockTssServer) GetKnownPeers() []tss.PeerInfo {
	return []tss.PeerInfo{}
}

func (mts *MockTssServer) Keygen(req keygen.Request) (keygen.Response, error) {
	if mts.failToKeyGen {
		return keygen.Response{}, errors.New("you ask for it")
	}

	return keygen.NewResponse(mcommon.SigningAlgoSecp256k1, conversion.GetRandomPubKey(), "whatever", common.Success, blame.Blame{}), nil
}

func (mts *MockTssServer) KeygenAllAlgo(req keygen.Request) ([]keygen.Response, error) {
	if mts.failToKeyGen {
		return []keygen.Response{}, errors.New("you ask for it")
	}
	return []keygen.Response{
		keygen.NewResponse(mcommon.SigningAlgoSecp256k1, conversion.GetRandomPubKey(), "whatever", common.Success, blame.Blame{}),
		keygen.NewResponse(mcommon.SigningAlgoEd25519, conversion.GetRandomPubKey(), "whatever", common.Success, blame.Blame{}),
	}, nil
}

func (mts *MockTssServer) KeySign(req keysign.Request) (keysign.Response, error) {
	if mts.failToKeySign {
		return keysign.Response{}, errors.New("you ask for it")
	}
	newSig := keysign.NewSignature("", "", "", "")
	return keysign.NewResponse([]keysign.Signature{newSig}, common.Success, blame.Blame{}), nil
}
