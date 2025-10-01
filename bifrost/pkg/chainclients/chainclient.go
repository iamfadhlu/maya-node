package chainclients

import (
	"math/big"

	"gitlab.com/mayachain/mayanode/bifrost/mayaclient/types"
	stypes "gitlab.com/mayachain/mayanode/bifrost/mayaclient/types"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/config"
)

// ChainClient is the interface that wraps basic chain client methods
//
// SignTx       signs transactions
// BroadcastTx  broadcast transactions on the chain associated with the client
// GetChain     get chain id
// SignTx       sign transaction
// GetHeight    get chain height
// GetAddress   gets address for public key pool in chain
// GetAccount   gets account from mayaclient in cain
// GetConfig	gets the chain configuration
// Start
// Stop
type ChainClient interface {
	SignTx(tx stypes.TxOutItem, height int64) ([]byte, []byte, *stypes.TxInItem, error)
	BroadcastTx(_ stypes.TxOutItem, _ []byte) (string, error)
	GetHeight() (int64, error)
	GetBlockScannerHeight() (int64, error)
	GetLatestTxForVault(vault string) (string, string, error)
	GetAddress(poolPubKey common.PubKey) string
	GetAccount(poolPubKey common.PubKey, height *big.Int) (common.Account, error)
	GetAccountByAddress(address string, height *big.Int) (common.Account, error)
	GetChain() common.Chain
	OnObservedTxIn(txIn types.TxInItem, blockHeight int64)
	Start(globalTxsQueue chan stypes.TxIn, globalErrataQueue chan stypes.ErrataBlock, globalSolvencyQueue chan stypes.Solvency)
	GetConfig() config.BifrostChainConfiguration
	GetConfirmationCount(txIn stypes.TxIn) int64
	ConfirmationCountReady(txIn stypes.TxIn) bool
	IsBlockScannerHealthy() bool
	Stop()
}
