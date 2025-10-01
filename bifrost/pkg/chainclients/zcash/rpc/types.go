package rpc

import (
	"io"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

// --- ZEC-Specific Types ---

// parse the JSON response
type Response struct {
	Error struct {
		Code    btcjson.RPCErrorCode `json:"code"`
		Message string               `json:"message"`
	} `json:"error"`
}

// TxBlock represents a block containing simplified transaction identifiers.
// Note: For full tx data, GetBlockVerboseTx returns BlockWithTx.
type TxBlock struct {
	Hash   string
	Height int64 // Match BlockWithTx Height type
	Txs    []Tx
}

// Tx represents a simplified Zcash transaction identifier.
type Tx struct {
	TxID   string
	Height int64 // Match BlockWithTx Height type
}

// BlockHeader represents Zcash block header info (Matches btcjson.GetBlockHeaderVerboseResult partially)
type BlockHeader struct {
	Hash          string  `json:"hash"`
	Height        int64   `json:"height"`
	Confirmations int64   `json:"confirmations"` // Often included
	Version       int32   `json:"version"`       // Often included
	MerkleRoot    string  `json:"merkleroot"`    // Often included
	Time          int64   `json:"time"`          // Often included
	Nonce         string  `json:"nonce"`         // Often included
	PreviousHash  string  `json:"previousblockhash"`
	NextHash      *string `json:"nextblockhash"`
}

// AddressDelta represents transaction delta for an address (Zcash specific indexer call)
type AddressDelta struct {
	Address  string `json:"address"`
	TxID     string `json:"txid"`
	Height   uint32 `json:"height"`
	Satoshis int64  `json:"satoshis"`
}

// AddressDeltasResponse represents the getaddressdeltas response (Zcash specific indexer call)
type AddressDeltasResponse struct {
	Deltas []AddressDelta `json:"deltas"`
	Start  struct {
		Hash   string `json:"hash"`
		Height uint32 `json:"height"`
	} `json:"start"`
	End struct {
		Hash   string `json:"hash"`
		Height uint32 `json:"height"`
	} `json:"end"`
}

// Block represents a block with transaction hashes, including Zcash specifics.
type Block struct {
	// Standard fields (common with Bitcoin)
	Hash          string   `json:"hash"`
	Confirmations int64    `json:"confirmations"`
	Size          int32    `json:"size"`
	Height        int64    `json:"height"`
	Version       int32    `json:"version"`
	MerkleRoot    string   `json:"merkleroot"`
	Tx            []string `json:"tx"` // List of transaction IDs
	Time          int64    `json:"time"`
	Nonce         string   `json:"nonce"` // Note: Zcash uses string for nonce
	Bits          string   `json:"bits"`
	Difficulty    float64  `json:"difficulty"`
	Chainwork     string   `json:"chainwork"`
	PreviousHash  string   `json:"previousblockhash"`
	NextHash      string   `json:"nextblockhash,omitempty"`

	// Zcash-specific fields
	Blockcommitments string `json:"blockcommitments"`
	Authdataroot     string `json:"authdataroot"`
	Finalsaplingroot string `json:"finalsaplingroot"`
	Finalorchardroot string `json:"finalorchardroot"`
	Chainhistoryroot string `json:"chainhistoryroot"`
	Solution         string `json:"solution"`
	Anchor           string `json:"anchor"`

	// Value pools
	ChainSupply struct {
		Monitored     bool    `json:"monitored"`
		ChainValue    float64 `json:"chainValue"`
		ChainValueZat int64   `json:"chainValueZat"`
		ValueDelta    float64 `json:"valueDelta"`
		ValueDeltaZat int64   `json:"valueDeltaZat"`
	} `json:"chainSupply"`

	ValuePools []struct {
		ID            string  `json:"id"`
		Monitored     bool    `json:"monitored"`
		ChainValue    float64 `json:"chainValue"`
		ChainValueZat int64   `json:"chainValueZat"`
		ValueDelta    float64 `json:"valueDelta"`
		ValueDeltaZat int64   `json:"valueDeltaZat"`
	} `json:"valuePools"`

	Trees struct {
		Sapling struct {
			Size int `json:"size"`
		} `json:"sapling"`
		Orchard struct {
			Size int `json:"size"`
		} `json:"orchard"`
	} `json:"trees"`
}

// BlockWithTx represents a block with verbose transaction data, including Zcash specifics.
type BlockWithTx struct {
	Block
	Tx []TxVerbose `json:"tx"`
}

// TxVerbose represents verbose transaction data, including Zcash specifics.
type TxVerbose struct {
	// Fields commonly found in btcjson.TxRawResult (copied)
	Txid          string `json:"txid"`
	Hash          string `json:"hash"` // Often same as Txid for non-segwit
	Version       int32  `json:"version"`
	Size          int32  `json:"size"`
	Vsize         int32  `json:"vsize,omitempty"`  // Optional field
	Weight        int32  `json:"weight,omitempty"` // Optional field
	LockTime      uint32 `json:"locktime"`
	Vin           []Vin  `json:"vin"`  // Use your FULLY DEFINED Vin struct type here
	Vout          []Vout `json:"vout"` // Use your FULLY DEFINED Vout struct type here
	BlockHash     string `json:"blockhash,omitempty"`
	Confirmations uint32 `json:"confirmations,omitempty"`
	Time          int64  `json:"time,omitempty"`
	BlockTime     int64  `json:"blocktime,omitempty"`
	Hex           string `json:"hex"` // The raw tx hex

	// Zcash specific fields (match JSON keys from zcashd)
	Vjoinsplit      []interface{} `json:"vjoinsplit,omitempty"`
	ValueBalance    float64       `json:"valueBalance,omitempty"`
	ValueBalanceZat int64         `json:"valueBalanceZat,omitempty"`
	VShieldedSpend  []interface{} `json:"vShieldedSpend,omitempty"`
	VShieldedOutput []interface{} `json:"vShieldedOutput,omitempty"`
	AuthDigest      string        `json:"authdigest,omitempty"`
	Overwintered    bool          `json:"overwintered,omitempty"`
	VersionGroupID  string        `json:"versiongroupid,omitempty"`
	ExpiryHeight    int64         `json:"expiryheight,omitempty"`
}

type Vin struct {
	Coinbase  string     `json:"coinbase,omitempty"`
	Txid      string     `json:"txid,omitempty"`
	Vout      uint32     `json:"vout,omitempty"`
	ScriptSig *ScriptSig `json:"scriptSig,omitempty"`
	Sequence  uint32     `json:"sequence"`
}

// IsCoinBase returns a bool to show if a Vin is a Coinbase one or not.
func (v *Vin) IsCoinBase() bool {
	return len(v.Coinbase) > 0
}

type Vout struct {
	Value        float64      `json:"value"`
	N            uint32       `json:"n"`
	ScriptPubKey ScriptPubKey `json:"scriptPubKey"`
}

type ScriptPubKey struct {
	Asm       string   `json:"asm"`
	Hex       string   `json:"hex"`
	ReqSigs   int32    `json:"reqSigs,omitempty"`
	Type      string   `json:"type"`
	Addresses []string `json:"addresses,omitempty"`
}

// Example ScriptSig:
type ScriptSig struct {
	Asm string `json:"asm"`
	Hex string `json:"hex"`
}

// MempoolEntry models the data from the getmempoolentry command.
type MempoolEntry struct {
	// Fields commonly found in btcjson.GetMempoolEntryResult (copied)
	Size              int32    `json:"size"`
	Vsize             int32    `json:"vsize"`
	Fee               float64  `json:"fee"`
	ModifiedFee       float64  `json:"modifiedfee"`
	Time              int64    `json:"time"`
	Height            int64    `json:"height"`
	DescendantCount   int64    `json:"descendantcount"`
	DescendantSize    int64    `json:"descendantsize"`
	DescendantFees    float64  `json:"descendantfees"`
	AncestorCount     int64    `json:"ancestorcount"`
	AncestorSize      int64    `json:"ancestorsize"`
	AncestorFees      float64  `json:"ancestorfees"`
	Depends           []string `json:"depends"`
	SpentBy           []string `json:"spentby"`
	BIP125Replaceable bool     `json:"bip125-replaceable"`

	// --- Zcash Specific Fields ---
}

// AddressBalanceResult represents the result from the getaddressbalance RPC call
type AddressBalanceResult struct {
	Balance  int64 `json:"balance"`  // Balance in Zatoshi
	Received int64 `json:"received"` // Total received in Zatoshi
}

// Utxo represents a single Unspent Transaction Output (UTXO)
// as returned by the Zcash RPC call 'getaddressutxos'.
type Utxo struct {
	Txid   string `json:"txid"`
	Vout   uint32 `json:"outputIndex"`
	Script string `json:"script"`
	Value  int64  `json:"satoshis"`
	Height int64  `json:"height"`
}

// ZecHasher wraps a chainhash.Hash to make it serializable for RPC
type ZecHasher struct {
	hash *chainhash.Hash
}

// SerializeSize returns the number of bytes needed to serialize the transaction
func (tx *ZecHasher) SerializeSize() int {
	return chainhash.HashSize
}

// Serialize writes the transaction to the given writer.
func (tx *ZecHasher) Serialize(w io.Writer) error {
	_, err := w.Write(tx.hash.CloneBytes())
	return err
}

// NewZecHasher creates a new ZecHasher from a chainhash.Hash
func NewZecHasher(hash *chainhash.Hash) *ZecHasher {
	return &ZecHasher{hash: hash}
}
