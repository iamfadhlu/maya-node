package rpc

import (
	"io"

	"github.com/btcsuite/btcd/btcjson"
)

// SerializableTx defines a transaction that can be serialized to bytes.
type SerializableTx interface {
	SerializeSize() int
	Serialize(io.Writer) error
}

// UTXOClient defines the common interface for all UTXO-based blockchains.
type UTXOClient interface {
	// Core RPC methods common to all UTXO chains
	GetBlockCount() (int64, error)
	GetBlockHash(height int64) (string, error)
	GetBlockVerbose(hash string) (*btcjson.GetBlockVerboseResult, error)
	GetBlockVerboseTxs(hash string) (*btcjson.GetBlockVerboseTxResult, error)
	GetRawTransactionVerbose(txid string) (*btcjson.TxRawResult, error)
	GetRawTransaction(txid string) (string, error)
	GetMempoolEntry(txid string) (*btcjson.GetMempoolEntryResult, error)
	GetRawMempool() ([]string, error)
	ImportAddress(address string) error
	ImportAddressRescan(address string, rescan bool) error
	ListUnspent(address string) ([]btcjson.ListUnspentResult, error)
	SendRawTransaction(tx SerializableTx, maxFeeParam any) (string, error)
	GetNetworkInfo() (*btcjson.GetNetworkInfoResult, error)

	// Helper methods
	BatchGetRawTransactionVerbose(txids []string) ([]*btcjson.TxRawResult, []error, error)
	BatchGetMempoolEntry(txids []string) ([]*btcjson.GetMempoolEntryResult, []error, error)

	// Generic call method for chain-specific extensions
	Call(result any, method string, args ...interface{}) error

	// Cleanup/shutdown
	Shutdown()
}
