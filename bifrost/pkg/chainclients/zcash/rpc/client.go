package rpc

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	baseutxorpc "gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/utxo/rpc"
)

// ZcashClient extends the base UTXO client with Zcash-specific functionality.
type ZcashClient struct {
	// Embed the base UTXO client
	baseClient baseutxorpc.UTXOClient

	// Zcash-specific fields
	logger zerolog.Logger
}

// Ensure ZcashClient implements the UTXOClient interface
var _ baseutxorpc.UTXOClient = (*ZcashClient)(nil)

// NewZcashClient creates a new client for Zcash.
func NewZcashClient(host, user, password string, maxRetries int, logger zerolog.Logger) (*ZcashClient, error) {
	baseClient, err := baseutxorpc.NewClient(host, user, password, maxRetries, logger)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create base UTXO client")
	}

	return &ZcashClient{
		baseClient: baseClient,
		logger:     logger,
	}, nil
}

// This makes best effort to extract and return the error as a btcjson.RPCError.
func extractBTCError(err error) error {
	if err == nil {
		return nil
	}

	// split the error into the HTTP status and the JSON response
	parts := strings.SplitN(err.Error(), ": ", 2)
	if len(parts) != 2 {
		return err
	}

	var response Response
	if jsonErr := json.Unmarshal([]byte(parts[1]), &response); jsonErr != nil {
		return err
	}

	// return the error message
	return btcjson.NewRPCError(response.Error.Code, response.Error.Message)
}

// GetLatestHeight retrieves the latest block height from the connected Zcash node.
func (c *ZcashClient) GetLatestHeight() (int64, error) {
	return c.baseClient.GetBlockCount()
}

// GetBlockHash retrieves the block hash for the height from the connected Zcash node.
// This implements the UTXOClient interface by returning a string hash.
func (c *ZcashClient) GetBlockHash(height int64) (string, error) {
	return c.baseClient.GetBlockHash(height)
}

// GetBlockHashAsHash retrieves the block hash for the height and returns it as a Hash object.
func (c *ZcashClient) GetBlockHashAsHash(height int64) (*chainhash.Hash, error) {
	hashStr, err := c.baseClient.GetBlockHash(height)
	if err != nil {
		return nil, errors.Wrapf(err, "rpcclient: failed to get block hash for height %d", height)
	}
	hash, err := chainhash.NewHashFromStr(hashStr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse block hash %s", hashStr)
	}
	return hash, nil
}

// GetBlockHeader returns the block header for a given block hash
func (c *ZcashClient) GetBlockHeader(blockHash string) (*BlockHeader, error) {
	var header BlockHeader
	err := c.baseClient.Call(&header, "getblockheader", blockHash, true) // true for verbose JSON object
	if err != nil {
		return nil, errors.Wrapf(err, "rpcclient: RawRequest failed for getblockheader (hash: %s)", blockHash)
	}
	return &header, nil
}

// GetBlockVerboseTx returns a block with verbose transaction data
// This implements the UTXOClient interface expecting a string hash
func (c *ZcashClient) GetBlockVerboseTx(hashStr string) (*BlockWithTx, error) {
	hash, err := chainhash.NewHashFromStr(hashStr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse block hash %s", hashStr)
	}
	return c.GetBlockVerboseTxHash(hash)
}

// GetBlockVerboseTxHash returns a block with verbose transaction data using a Hash object
// This is our custom ZEC-specific implementation
func (c *ZcashClient) GetBlockVerboseTxHash(blockHash *chainhash.Hash) (*BlockWithTx, error) {
	var block BlockWithTx
	err := c.baseClient.Call(&block, "getblock", blockHash.String(), 2) // Verbosity 2 for block with verbose transactions
	if err != nil {
		return nil, errors.Wrapf(err, "rpcclient: RawRequest failed for getblock verbosity 2 (hash: %s)", blockHash)
	}
	return &block, nil
}

// GetRawTransactionVerboseHash returns a transaction in verbose format using a chainhash.Hash
func (c *ZcashClient) GetRawTransactionVerboseHash(txHash *chainhash.Hash) (*TxVerbose, error) {
	if txHash == nil || txHash.String() == "" {
		return nil, errors.New("invalid tx hash: nil")
	}

	var txVerbose TxVerbose
	err := c.baseClient.Call(&txVerbose, "getrawtransaction", txHash.String(), 1) // Verbosity 1 for verbose JSON object
	if err != nil {
		// Check for specific error indicating transaction not found
		if rpcErr, ok := err.(*btcjson.RPCError); ok && rpcErr.Code == btcjson.ErrRPCNoTxInfo {
			return nil, errors.Wrapf(err, "transaction not found: %s", txHash.String())
		}
		return nil, errors.Wrapf(err, "rpcclient: RawRequest failed for getrawtransaction (hash: %s)", txHash.String())
	}
	return &txVerbose, nil
}

// GetMempoolEntryZec returns details about a transaction in the Zcash mempool using Zcash-specific types
func (c *ZcashClient) GetMempoolEntryZec(txHash string) (*MempoolEntry, error) {
	var entry MempoolEntry
	err := c.baseClient.Call(&entry, "getmempoolentry", txHash)
	if err != nil {
		return nil, errors.Wrapf(err, "rpcclient: RawRequest failed for getmempoolentry (hash: %s)", txHash)
	}
	return &entry, nil
}

// BroadcastRawTx sends a raw transaction hex to the Zcash node
func (c *ZcashClient) BroadcastRawTx(txb []byte) (string, error) {
	txHex := hex.EncodeToString(txb)
	var txid string
	err := c.baseClient.Call(&txid, "sendrawtransaction", txHex)
	if err != nil {
		// RawRequest usually returns *btcjson.RPCError
		if rpcErr, ok := err.(*btcjson.RPCError); ok {
			return "", errors.Wrapf(err, "rpc error broadcasting transaction (code: %d)", rpcErr.Code)
		}
		// Otherwise, wrap the generic error
		return "", errors.Wrap(err, "rpcclient: RawRequest failed for sendrawtransaction")
	}
	if txid == "" {
		return "", errors.New("sendrawtransaction succeeded but returned an empty txid")
	}
	return txid, nil
}

// DecodeRawTransaction decodes a raw transaction byte slice into our verbose Zcash format
func (c *ZcashClient) DecodeRawTransaction(payload []byte) (*TxVerbose, error) {
	txHex := hex.EncodeToString(payload)
	var decodedTx TxVerbose
	err := c.baseClient.Call(&decodedTx, "decoderawtransaction", txHex)
	if err != nil {
		return nil, errors.Wrap(err, "rpcclient: RawRequest failed for decoderawtransaction")
	}
	return &decodedTx, nil
}

// GetRawTxID decodes a raw transaction byte slice and returns only its transaction ID (txid)
func (c *ZcashClient) GetRawTxID(payload []byte) (string, error) {
	// Reuse the DecodeRawTransaction logic which handles RawRequest and unmarshalling
	decodedTx, err := c.DecodeRawTransaction(payload)
	if err != nil {
		// Error already wrapped by DecodeRawTransaction
		return "", err
	}
	if decodedTx.Txid == "" {
		// Should ideally not happen if DecodeRawTransaction succeeded and didn't error
		return "", errors.New("decoded transaction has empty txid")
	}
	return decodedTx.Txid, nil
}

// GetAddressBalance retrieves the total transparent balance for a single Zcash address
func (c *ZcashClient) GetAddressBalance(address string) (uint64, error) {
	// Basic input validation
	if address == "" {
		return 0, errors.New("address cannot be empty")
	}

	// Prepare the parameters for the RPC call
	// 'getaddressbalance' expects a JSON object: {"addresses": ["address1", ...]}
	params := map[string]interface{}{
		"addresses": []string{address},
	}

	// Call the generic rpc helper which uses RawRequest
	var balanceResult AddressBalanceResult
	err := c.baseClient.Call(&balanceResult, "getaddressbalance", params)
	if err != nil {
		return 0, fmt.Errorf("rpc call failed for getaddressbalance (address: %s), err: %w", address, err)
	}

	// Return the populated struct
	return uint64(balanceResult.Balance), nil
}

// GetAddressUTXOs retrieves all UTXOs for a single Zcash address
func (c *ZcashClient) GetAddressUTXOs(address string) ([]Utxo, error) {
	// Basic input validation
	if address == "" {
		return nil, errors.New("address cannot be empty")
	}

	// Prepare the parameters for the RPC call
	// 'getaddressutxos' expects a JSON object: {"addresses": ["address1", ...], "chaininfo": false}}
	params := map[string]interface{}{
		"addresses": []string{address},
	}

	// Call the generic rpc helper which uses RawRequest
	var utxosResult []Utxo
	err := c.baseClient.Call(&utxosResult, "getaddressutxos", params)
	if err != nil {
		return nil, fmt.Errorf("rpc call failed for getaddressutxos (address: %s), err: %w", address, err)
	}

	// Return the populated struct
	return utxosResult, nil
}

// only use in mocknet or tests
func (c *ZcashClient) GenerateBlocks(count int) error {
	// Basic input validation
	if count < 1 {
		return errors.New("invalid count: must be greater than 0")
	}

	err := c.baseClient.Call(nil, "generate", count)
	if err != nil {
		return errors.Wrap(err, "rpcclient: RawRequest failed for generate")
	}
	return nil
}

// ImportAddress imports the address with no rescan.
func (c *ZcashClient) ImportAddress(address string) error {
	return c.baseClient.ImportAddress(address)
}

// ImportAddressRescan imports the address with rescan.
func (c *ZcashClient) ImportAddressRescan(address string, rescan bool) error {
	return c.baseClient.ImportAddressRescan(address, rescan)
}

// Shutdown closes the connection to the RPC server
func (c *ZcashClient) Shutdown() {
	c.baseClient.Shutdown()
}

// The following methods are adapter methods to ensure ZcashClient implements the UTXOClient interface fully

// GetBlockCount returns the current block count
func (c *ZcashClient) GetBlockCount() (int64, error) {
	return c.baseClient.GetBlockCount()
}

// GetBlockVerbose returns the block with the given hash
func (c *ZcashClient) GetBlockVerbose(hash string) (*btcjson.GetBlockVerboseResult, error) {
	return c.baseClient.GetBlockVerbose(hash)
}

// GetBlockVerboseTxs returns the block with the given hash with full transaction details
func (c *ZcashClient) GetBlockVerboseTxs(hash string) (*btcjson.GetBlockVerboseTxResult, error) {
	return c.baseClient.GetBlockVerboseTxs(hash)
}

// GetRawTransactionVerbose returns the raw transaction with the given txid
// This implements the UTXOClient interface
func (c *ZcashClient) GetRawTransactionVerbose(txid string) (*btcjson.TxRawResult, error) {
	var tx btcjson.TxRawResult
	err := c.baseClient.Call(&tx, "getrawtransaction", txid, 1)
	return &tx, extractBTCError(err)
}

// GetRawTransaction returns the raw transaction with the given txid
func (c *ZcashClient) GetRawTransaction(txid string) (string, error) {
	return c.baseClient.GetRawTransaction(txid)
}

// GetMempoolEntry returns the mempool entry for the given txid
func (c *ZcashClient) GetMempoolEntry(txid string) (*btcjson.GetMempoolEntryResult, error) {
	return c.baseClient.GetMempoolEntry(txid)
}

// GetRawMempool returns the raw mempool
func (c *ZcashClient) GetRawMempool() ([]string, error) {
	return c.baseClient.GetRawMempool()
}

// ListUnspent returns the list of unspent transaction outputs for the given address
func (c *ZcashClient) ListUnspent(address string) ([]btcjson.ListUnspentResult, error) {
	return c.baseClient.ListUnspent(address)
}

// SendRawTransaction sends a raw transaction to the network
func (c *ZcashClient) SendRawTransaction(tx baseutxorpc.SerializableTx, maxFeeParam any) (string, error) {
	return c.baseClient.SendRawTransaction(tx, maxFeeParam)
}

// GetNetworkInfo returns information about the network
func (c *ZcashClient) GetNetworkInfo() (*btcjson.GetNetworkInfoResult, error) {
	return c.baseClient.GetNetworkInfo()
}

// BatchGetRawTransactionVerbose returns raw transactions for the given txids
func (c *ZcashClient) BatchGetRawTransactionVerbose(txids []string) ([]*btcjson.TxRawResult, []error, error) {
	return c.baseClient.BatchGetRawTransactionVerbose(txids)
}

// BatchGetMempoolEntry returns mempool entries for the given txids
func (c *ZcashClient) BatchGetMempoolEntry(txids []string) ([]*btcjson.GetMempoolEntryResult, []error, error) {
	return c.baseClient.BatchGetMempoolEntry(txids)
}

// Call executes a generic RPC call
func (c *ZcashClient) Call(result any, method string, args ...interface{}) error {
	return c.baseClient.Call(result, method, args...)
}
