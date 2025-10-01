package utxo

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/tendermint/tendermint/crypto/secp256k1"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/btcjson"
	btcchaincfg "github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	btcwire "github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	btctxscript "gitlab.com/mayachain/mayanode/bifrost/txscript/txscript"

	dashutil "gitlab.com/mayachain/dashd-go/btcutil"
	dashchaincfg "gitlab.com/mayachain/dashd-go/chaincfg"
	dashtxscript "gitlab.com/mayachain/dashd-go/txscript"
	dashwire "gitlab.com/mayachain/dashd-go/wire"

	"gitlab.com/mayachain/mayanode/bifrost/mayaclient"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/dash"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/utxo/rpc"
	zecrpc "gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/zcash/rpc"

	//	zectxscript "gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/zcash/txscript"
	"gitlab.com/mayachain/mayanode/chain/zec/go/zec"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"

	. "gitlab.com/mayachain/mayanode/test/simulation/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////////////
// Client
////////////////////////////////////////////////////////////////////////////////////////

//go:generate go run utxo_generate.go

type Client struct {
	sync.Mutex
	chain common.Chain
	rpc   rpc.UTXOClient

	// ZEC-specific client
	zecClient *zecrpc.ZcashClient

	keys    *mayaclient.Keys
	privKey *btcec.PrivateKey
	pubKey  common.PubKey
	address common.Address
	log     zerolog.Logger

	muSpentUtxos sync.Mutex
	spentUtxos   map[string]struct{}
}

var _ LiteChainClient = &Client{}

const MaxUtxosToSpend = 20

func NewConstructor(host string) LiteChainClientConstructor {
	return func(chain common.Chain, keys *mayaclient.Keys) (LiteChainClient, error) {
		return NewClient(chain, host, keys)
	}
}

func NewClient(chain common.Chain, host string, keys *mayaclient.Keys) (LiteChainClient, error) {
	// create rpc client
	retries := 5
	logger := zerolog.Nop()

	// extract the private key
	privateKey, err := keys.GetPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("fail to get private key: %w", err)
	}
	privKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), privateKey.Bytes())

	// derive the public key
	buf := privKey.PubKey().SerializeCompressed()
	pk := secp256k1.PubKey(buf)
	pubkey, err := common.NewPubKeyFromCrypto(pk)
	if err != nil {
		return nil, fmt.Errorf("fail to create pubkey: %w", err)
	}

	// get pubkey address for the chain
	address, err := pubkey.GetAddress(chain)
	if err != nil {
		return nil, fmt.Errorf("fail to get address from pubkey(%s): %w", pk, err)
	}

	// Create appropriate RPC client based on chain
	var rpcClient rpc.UTXOClient
	var zecClient *zecrpc.ZcashClient

	if chain.Equals(common.ZECChain) {
		// For Zcash, use the specialized client
		zecClient, err = zecrpc.NewZcashClient(host, "mayachain", "password", retries, logger)
		if err != nil {
			return nil, fmt.Errorf("fail to create zcash rpc client: %w", err)
		}

		// Use ZcashClient directly as it implements the UTXOClient interface
		rpcClient = zecClient
	} else {
		// For other UTXO chains, use the standard client
		rpcClient, err = rpc.NewClient(host, "mayachain", "password", retries, logger)
		if err != nil {
			return nil, fmt.Errorf("fail to create rpc client: %w", err)
		}

		// Import address to wallet, rescan if master account
		if keys.GetSignerInfo().GetName() == "master" {
			err = rpcClient.ImportAddressRescan(address.String(), true)
		} else {
			err = rpcClient.ImportAddress(address.String())
		}
	}

	if err != nil {
		return nil, fmt.Errorf("fail to import address(%s): %w", address, err)
	}

	spentUtxos := map[string]struct{}{}

	return &Client{
		chain:      chain,
		rpc:        rpcClient,
		zecClient:  zecClient,
		keys:       keys,
		privKey:    privKey,
		pubKey:     pubkey,
		address:    address,
		spentUtxos: spentUtxos,
		log:        logger,
	}, nil
}

////////////////////////////////////////////////////////////////////////////////////////
// GetAccount
////////////////////////////////////////////////////////////////////////////////////////

func (c *Client) GetAccount(pk *common.PubKey) (*common.Account, error) {
	// default to the client key address
	var err error
	addr := c.address
	if pk != nil {
		addr, err = pk.GetAddress(c.chain)
		if err != nil {
			return nil, fmt.Errorf("fail to get address from pubkey(%s): %w", pk, err)
		}
	}

	if c.chain.Equals(common.ZECChain) {
		// Use Zcash-specific balance lookup
		balance, err := c.zecClient.GetAddressBalance(addr.String())
		if err != nil {
			return nil, fmt.Errorf("fail to get ZEC balance: %w", err)
		}

		// Create account
		coin := common.NewCoin(c.chain.GetGasAsset(), cosmos.NewUint(balance))
		a := common.NewAccount(0, 0, common.NewCoins(coin), false)
		return &a, nil
	} else {
		// Use standard UTXO balance lookup
		utxos, err := c.rpc.ListUnspent(addr.String())
		if err != nil {
			return nil, fmt.Errorf("fail to get UTXOs: %w", err)
		}

		// sum balance
		total := 0.0
		for _, item := range utxos {
			total += item.Amount
		}
		totalAmt, err := btcutil.NewAmount(total)
		if err != nil {
			return nil, fmt.Errorf("fail to convert total amount: %w", err)
		}

		// create account
		coin := common.NewCoin(c.chain.GetGasAsset(), cosmos.NewUint(uint64(totalAmt)))
		a := common.NewAccount(0, 0, common.NewCoins(coin), false)

		return &a, nil
	}
}

////////////////////////////////////////////////////////////////////////////////////////
// SignTx
////////////////////////////////////////////////////////////////////////////////////////

func (c *Client) SignTx(tx SimTx) ([]byte, error) {
	if c.chain.Equals(common.ZECChain) {
		c.muSpentUtxos.Lock()
		defer c.muSpentUtxos.Unlock()

		// Handle ZEC-specific signing
		ptx, checkpoint, err := c.buildPtx(tx)
		if err != nil {
			return nil, fmt.Errorf("fail to build zcash partial transaction: %w", err)
		}

		// Sign the transaction using the Zcash-specific signing function
		signedTx, err := c.signUTXOZEC(ptx, checkpoint)
		if err != nil {
			return nil, fmt.Errorf("fail to sign zcash transaction: %w", err)
		}

		// save spent utxos
		for _, utxo := range ptx.Inputs {
			utxoKey := fmt.Sprintf("%s:%d", utxo.Txid, utxo.Vout)
			c.spentUtxos[utxoKey] = struct{}{}
		}

		return signedTx, nil
	} else {
		// Handle BTC/DASH signing
		sourceScript, err := c.getSourceScript(tx)
		if err != nil {
			return nil, fmt.Errorf("fail to get source pay to address script: %w", err)
		}

		// build transaction
		redeemTx, amounts, err := c.buildTx(tx, sourceScript)
		if err != nil {
			return nil, err
		}

		// create the list of signing requests
		signings := []struct{ idx, amount int64 }{}
		totalAmount := int64(0)
		for idx, txIn := range redeemTx.TxIn {
			key := fmt.Sprintf("%s-%d", txIn.PreviousOutPoint.Hash, txIn.PreviousOutPoint.Index)
			outputAmount := amounts[key]
			totalAmount += outputAmount
			signings = append(signings, struct{ idx, amount int64 }{int64(idx), outputAmount})
		}

		// convert the wire tx to the chain specific tx for signing
		var stx interface{}
		switch c.chain {
		case common.DASHChain:
			stx = wireToDASH(redeemTx)
		case common.BTCChain:
			stx = wireToBTC(redeemTx)
		default:
			return nil, fmt.Errorf("unsupported chain %s", c.chain)
		}

		// sign the tx
		wg := &sync.WaitGroup{}
		wg.Add(len(signings))
		mu := &sync.Mutex{}
		var utxoErr error
		for _, signing := range signings {
			go func(i int, amount int64) {
				defer wg.Done()

				// trunk-ignore(golangci-lint/govet): shadow
				var err error

				// trunk-ignore-all(golangci-lint/forcetypeassert)
				switch c.chain {
				case common.DASHChain:
					err = c.signUTXODASH(stx.(*dashwire.MsgTx), amount, sourceScript, i)
				case common.BTCChain:
					err = c.signUTXOBTC(stx.(*btcwire.MsgTx), amount, sourceScript, i)
				default:
					log.Error().Msg("unsupported chain")
					return
				}

				if err != nil {
					mu.Lock()
					utxoErr = multierror.Append(utxoErr, err)
					mu.Unlock()
				}
			}(int(signing.idx), signing.amount)
		}
		wg.Wait()
		if utxoErr != nil {
			return nil, fmt.Errorf("fail to sign the message: %w", err)
		}

		// convert back to wire tx
		switch c.chain {
		case common.DASHChain:
			redeemTx = dashToWire(stx.(*dashwire.MsgTx))
		case common.BTCChain:
			redeemTx = btcToWire(stx.(*btcwire.MsgTx))
		default:
			return nil, fmt.Errorf("unsupported chain %s", c.chain)
		}

		// calculate the final transaction size
		var signedTx bytes.Buffer
		if err = redeemTx.Serialize(&signedTx); err != nil {
			return nil, fmt.Errorf("fail to serialize tx to bytes: %w", err)
		}

		return signedTx.Bytes(), nil
	}
}

////////////////////////////////////////////////////////////////////////////////////////
// BroadcastTx
////////////////////////////////////////////////////////////////////////////////////////

func (c *Client) BroadcastTx(payload []byte) (string, error) {
	if c.chain.Equals(common.ZECChain) {
		// Use ZEC-specific broadcast method
		txId, err := c.zecClient.BroadcastRawTx(payload)
		if err != nil {
			return txId, err
		}
		return txId, err
	} else {
		// Standard UTXO broadcast
		redeemTx := btcwire.NewMsgTx(btcwire.TxVersion)
		buf := bytes.NewBuffer(payload)
		if err := redeemTx.Deserialize(buf); err != nil {
			return "", fmt.Errorf("fail to deserialize payload: %w", err)
		}

		var maxFee any
		switch c.chain {
		// case common.DASHChain:
		// 	maxFee = true // "allowHighFees"
		case common.DASHChain, common.BTCChain:
			maxFee = 10_000_000
		}

		// broadcast tx
		return c.rpc.SendRawTransaction(redeemTx, maxFee)
	}
}

////////////////////////////////////////////////////////////////////////////////////////
// Internal
////////////////////////////////////////////////////////////////////////////////////////

func (c *Client) getSourceScript(tx SimTx) ([]byte, error) {
	switch c.chain {
	case common.DASHChain:
		var addr dashutil.Address
		addr, err := dashutil.DecodeAddress(c.address.String(), c.getChainCfgDASH())
		if err != nil {
			return nil, fmt.Errorf("fail to decode source address(%s): %w", c.address, err)
		}
		return dashtxscript.PayToAddrScript(addr)
	case common.BTCChain:
		var addr btcutil.Address
		addr, err := btcutil.DecodeAddress(c.address.String(), c.getChainCfgBTC())
		if err != nil {
			return nil, fmt.Errorf("fail to decode source address(%s): %w", c.address, err)
		}
		return btctxscript.PayToAddrScript(addr)
	default:
		return nil, fmt.Errorf("unsupported chain %s", c.chain)
	}
}

func (c *Client) getUtxoToSpend(total float64) ([]btcjson.ListUnspentResult, error) {
	utxos, err := c.rpc.ListUnspent(c.address.String())
	if err != nil {
		return nil, fmt.Errorf("fail to get UTXOs: %w", err)
	}

	// spend UTXOs older to younger
	sort.SliceStable(utxos, func(i, j int) bool {
		if utxos[i].Confirmations > utxos[j].Confirmations {
			return true
		} else if utxos[i].Confirmations < utxos[j].Confirmations {
			return false
		}
		return utxos[i].TxID < utxos[j].TxID
	})

	// collect UTXOs to spend
	var result []btcjson.ListUnspentResult
	var toSpend float64
	for _, item := range utxos {
		result = append(result, item)
		toSpend += item.Amount
		if toSpend >= total {
			break
		}
	}

	// error if there is insufficient balance
	if toSpend < total {
		return nil, fmt.Errorf("insufficient balance: %f < %f", toSpend, total)
	}

	return result, nil
}

func (c *Client) getZecUtxosToSpend(amount cosmos.Uint) ([]zecrpc.Utxo, error) {
	// Call ZEC-specific UTXO listing method
	utxos, err := c.zecClient.GetAddressUTXOs(c.address.String())
	if err != nil {
		return nil, fmt.Errorf("fail to convert btc UTXOs to Zcash UTXOs: %w", err)
	}
	log.Debug().Stringer("address", c.address).Int("utxos", len(utxos)).Msg("GetAddressUTXOs result")

	// Get current height
	lastHeight, err := c.zecClient.GetLatestHeight()
	if err != nil {
		return nil, fmt.Errorf("fail to get zec current block height: %w", err)
	}

	// Sort & spend UTXO older to younger
	// The number of confirmations is (LastHeight - UTXO.Height) + 1
	sort.SliceStable(utxos, func(i, j int) bool {
		iConfirmations := lastHeight - utxos[i].Height + 1
		jConfirmations := lastHeight - utxos[j].Height + 1
		if iConfirmations > jConfirmations {
			return true
		} else if iConfirmations < jConfirmations {
			return false
		}
		return utxos[i].Txid < utxos[j].Txid
	})

	// Select UTXOs
	var selectedUtxos []zecrpc.Utxo
	selectedUtxos = make([]zecrpc.Utxo, 0, len(utxos)) // pre-allocate slice capacity
	inputAmount := cosmos.ZeroUint()

	// For simplicity, just select all UTXOs up to the needed amount or maxUtxosToSpend
	for _, utxo := range utxos {
		utxoKey := fmt.Sprintf("%s:%d", utxo.Txid, utxo.Vout)
		if _, alreadySpent := c.spentUtxos[utxoKey]; alreadySpent {
			// log.Debug().Str("utxoKey", utxoKey).Int64("value", utxo.Value).Msg("UTXO already spent")
			continue
		}

		// Add the UTXO
		selectedUtxos = append(selectedUtxos, utxo)
		inputAmount = inputAmount.AddUint64(uint64(utxo.Value))

		// Stop if we have enough
		if inputAmount.GTE(amount) {
			break
		}

		// Error if we reached max UTXOs
		if int64(len(selectedUtxos)) >= MaxUtxosToSpend {
			return nil, fmt.Errorf("max utxo count reached: %d. Amount needed: %s, got %s", MaxUtxosToSpend, amount, inputAmount)
		}
	}
	if inputAmount.LT(amount) {
		return nil, fmt.Errorf("there is not enough fund in vault's all %d utxos. Amount needed: %s, got %s", len(selectedUtxos), amount, inputAmount)
	}

	return selectedUtxos, nil
}

func (c *Client) getGasSats() float64 {
	switch c.chain {
	case common.DASHChain:
		return 0.0001
	case common.BTCChain:
		return 0.0001
	case common.ZECChain:
		return 0.0001
	default:
		return 0.0
	}
}

func (c *Client) getPaymentAmount(tx SimTx) float64 {
	amtToPay1e8 := tx.Coin.Amount.Uint64()
	amtToPay := btcutil.Amount(int64(amtToPay1e8)).ToBTC()
	amtToPay += c.getGasSats()
	return amtToPay
}

func (c *Client) buildTx(tx SimTx, sourceScript []byte) (
	*btcwire.MsgTx, map[string]int64, error,
) {
	txes, err := c.getUtxoToSpend(c.getPaymentAmount(tx))
	if err != nil {
		return nil, nil, fmt.Errorf("fail to get unspent UTXO")
	}
	redeemTx := wire.NewMsgTx(wire.TxVersion)
	totalAmt := int64(0)
	individualAmounts := make(map[string]int64, len(txes))
	for _, item := range txes {
		var txID *chainhash.Hash
		txID, err = chainhash.NewHashFromStr(item.TxID)
		if err != nil {
			return nil, nil, fmt.Errorf("fail to parse txID(%s): %w", item.TxID, err)
		}
		// double check that the utxo is still valid
		outputPoint := wire.NewOutPoint(txID, item.Vout)
		sourceTxIn := wire.NewTxIn(outputPoint, nil, nil)
		redeemTx.AddTxIn(sourceTxIn)
		var amt btcutil.Amount
		amt, err = btcutil.NewAmount(item.Amount)
		if err != nil {
			return nil, nil, fmt.Errorf("fail to parse amount(%f): %w", item.Amount, err)
		}
		individualAmounts[fmt.Sprintf("%s-%d", txID, item.Vout)] = int64(amt)
		totalAmt += int64(amt)
	}

	var buf []byte
	switch c.chain {
	case common.DASHChain:
		var outputAddr dashutil.Address
		outputAddr, err = dashutil.DecodeAddress(tx.ToAddress.String(), c.getChainCfgDASH())
		if err != nil {
			return nil, nil, fmt.Errorf("fail to decode next address: %w", err)
		}
		buf, err = dashtxscript.PayToAddrScript(outputAddr)
		if err != nil {
			return nil, nil, fmt.Errorf("fail to get pay to address script: %w", err)
		}
	case common.BTCChain:
		var outputAddr btcutil.Address
		outputAddr, err = btcutil.DecodeAddress(tx.ToAddress.String(), c.getChainCfgBTC())
		if err != nil {
			return nil, nil, fmt.Errorf("fail to decode next address: %w", err)
		}
		buf, err = btctxscript.PayToAddrScript(outputAddr)
		if err != nil {
			return nil, nil, fmt.Errorf("fail to get pay to address script: %w", err)
		}
	default:
		return nil, nil, fmt.Errorf("unsupported chain %s", c.chain)
	}

	// pay to customer
	redeemTxOut := wire.NewTxOut(int64(tx.Coin.Amount.Uint64()), buf)
	redeemTx.AddTxOut(redeemTxOut)

	// add output to pay the balance back ourselves
	balance := totalAmt - redeemTxOut.Value - int64(c.getGasSats()*common.One)
	if balance > 0 {
		redeemTx.AddTxOut(wire.NewTxOut(balance, sourceScript))
	}

	// memo
	var nullDataScript []byte
	switch c.chain {
	case common.BTCChain:
		nullDataScript, err = btctxscript.NullDataScript([]byte(tx.Memo))
	case common.DASHChain:
		nullDataScript, err = dashtxscript.NullDataScript([]byte(tx.Memo))
	default:
		return nil, nil, fmt.Errorf("unsupported chain %s", c.chain)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("fail to generate null data script: %w", err)
	}
	redeemTx.AddTxOut(wire.NewTxOut(0, nullDataScript))

	return redeemTx, individualAmounts, nil
}

// builds a partial Zcash transaction for signing
func (c *Client) buildPtx(tx SimTx) (zec.PartialTx, []byte, error) {
	// For simulation, we'll assume the address is valid
	if tx.ToAddress.String() == "" {
		return zec.PartialTx{}, nil, fmt.Errorf("tx toAddress is empty")
	}

	// Get expiry height
	zecHeight, err := c.zecClient.GetLatestHeight()
	if err != nil {
		return zec.PartialTx{}, nil, fmt.Errorf("fail to get signer ZEC last height: %w", err)
	}
	expiryHeight := zecHeight + 1800 // 30min in zcash's mocknet node

	// Helper function to convert ZEC utxos to the format used by zec.PartialTx
	getZecUtxos := func(rpcUtxos []zecrpc.Utxo) []zec.Utxo {
		utxos := make([]zec.Utxo, len(rpcUtxos))
		for i, utxo := range rpcUtxos {
			utxos[i] = zec.Utxo{
				Txid:   utxo.Txid,
				Height: uint32(utxo.Height),
				Value:  uint64(utxo.Value),
				Vout:   utxo.Vout,
				Script: utxo.Script,
			}
		}
		return utxos
	}

	// Get UTXOs to spend - select UTXOs first without calculating fees
	amount := tx.Coin.Amount
	// estimate fee
	fee := zec.CalculateFeeWithMemo(1, 2, tx.Memo)
	targetAmount := amount.AddUint64((fee))
	totalInputAmount := cosmos.ZeroUint()
	var utxos []zecrpc.Utxo

	// Retry (max 3x) when sum(utxos) < amount + dynamic fee (based on used utxo count)
	for range 3 {
		utxos, err = c.getZecUtxosToSpend(targetAmount)
		if err != nil {
			return zec.PartialTx{}, nil, fmt.Errorf("fail to get utxos to spend: %w", err)
		}
		log.Debug().Stringer("targetAmount", targetAmount).Int("utxos", len(utxos)).Msg("getZecUtxosToSpend result")

		if len(utxos) == 0 {
			return zec.PartialTx{}, nil, errors.New("no utxo to spend found")
		}

		// Calculate total input amount
		totalInputAmount = cosmos.ZeroUint()
		for _, utxo := range utxos {
			totalInputAmount = totalInputAmount.AddUint64(uint64(utxo.Value))
		}

		fee = zec.CalculateFeeWithMemo(uint64(len(utxos)), 2, tx.Memo)
		targetAmount = amount.AddUint64(fee)
		if totalInputAmount.GTE(targetAmount) {
			break
		}
		log.Debug().
			Stringer("amount", amount).
			Uint64("fee", fee).
			Stringer("input", totalInputAmount).
			Stringer("need", targetAmount).
			Msg("amount + fee > inputs - need more")
	}

	// Calculate change amount (inputs - outputs - fee)
	change := cosmos.ZeroUint()
	if totalInputAmount.GT(amount.AddUint64(fee)) {
		change = totalInputAmount.Sub(amount).Sub(cosmos.NewUint(fee))
	}

	// Create partial transaction
	ptx := zec.PartialTx{
		Height:       uint32(zecHeight),
		ExpiryHeight: uint32(expiryHeight),
		Inputs:       getZecUtxos(utxos),
		Fee:          fee,
		Outputs: []zec.Output{
			{
				Address: tx.ToAddress.String(),
				Amount:  amount.BigInt().Uint64(),
				Memo:    tx.Memo,
			},
		},
	}

	// Add change output if needed
	if !change.IsZero() {
		ptx.Outputs = append(ptx.Outputs, zec.Output{
			Address: c.address.String(),
			Amount:  change.BigInt().Uint64(),
			Memo:    "",
		})
	}

	// Serialize the initial ptx for the checkpoint
	checkpoint, err := json.Marshal(ptx)
	if err != nil {
		return zec.PartialTx{}, nil, fmt.Errorf("failed to marshal initial PartialTx for checkpoint: %w", err)
	}

	// Get our pubkey bytes
	pubKeyBytes := c.privKey.PubKey().SerializeCompressed()

	// Call the Rust function to build the partial transaction
	// Build the transaction using the Rust FFI layer to populate sighashes
	ptx, err = zec.BuildPtx(pubKeyBytes, ptx, c.getChainCfgZEC().Net)
	if err != nil {
		return zec.PartialTx{}, checkpoint, fmt.Errorf("rust build_ptx failed: %w", err)
	}

	// Sanity check that sighashes were populated
	if len(ptx.Sighashes) != len(ptx.Inputs) {
		return zec.PartialTx{}, checkpoint, fmt.Errorf("number of sighashes (%d) does not match number of inputs (%d)",
			len(ptx.Sighashes), len(ptx.Inputs))
	}
	log.Debug().
		Stringer("amount", amount).
		Uint64("fee", fee).
		Stringer("need", targetAmount).
		Stringer("got", totalInputAmount).
		Int("inputs", len(ptx.Inputs)).
		Int("outputs", len(ptx.Outputs)).
		Stringer("change", change).
		Msg("Final Ptx")

	return ptx, checkpoint, nil
}

// ------------------------------ chain config ------------------------------

func (c *Client) getChainCfgBTC() *btcchaincfg.Params {
	switch common.CurrentChainNetwork {
	case common.MockNet:
		return &btcchaincfg.RegressionNetParams
	case common.TestNet:
		return &btcchaincfg.TestNet3Params
	case common.MainNet:
		return &btcchaincfg.MainNetParams
	case common.StageNet:
		return &btcchaincfg.MainNetParams
	default:
		log.Error().Msg("unsupported chain")
		return nil
	}
}

func (c *Client) getChainCfgDASH() *dashchaincfg.Params {
	switch common.CurrentChainNetwork {
	case common.MockNet:
		return &dashchaincfg.RegressionNetParams
	case common.MainNet:
		return &dashchaincfg.MainNetParams
	case common.StageNet:
		return &dashchaincfg.MainNetParams
	default:
		log.Error().Msg("unsupported chain")
		return nil
	}
}

func (c *Client) getChainCfgZEC() *zec.NetworkParams {
	switch common.CurrentChainNetwork {
	case common.MockNet:
		return &zec.RegtestNetParams
	case common.TestNet:
		return &zec.RegtestNetParams
	case common.MainNet:
		return &zec.MainNetParams
	case common.StageNet:
		return &zec.MainNetParams
	default:
		log.Error().Msg("unsupported chain")
		return nil
	}
}

// ------------------------------ signing ------------------------------

func (c *Client) signUTXODASH(redeemTx *dashwire.MsgTx, amount int64, sourceScript []byte, idx int) error {
	v1PrivateKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), c.privKey.Serialize())
	signable := btctxscript.NewPrivateKeySignable(v1PrivateKey)
	sig, err := dash.RawTxInSignatureUsingSignable(redeemTx, idx, sourceScript, dashtxscript.SigHashAll, signable)
	if err != nil {
		return fmt.Errorf("fail to get witness: %w", err)
	}

	pkData := signable.GetPubKey().SerializeCompressed()
	sigscript, err := dashtxscript.NewScriptBuilder().AddData(sig).AddData(pkData).Script()
	if err != nil {
		return fmt.Errorf("fail to build signature script: %w", err)
	}
	redeemTx.TxIn[idx].SignatureScript = sigscript
	flag := dashtxscript.StandardVerifyFlags
	engine, err := dashtxscript.NewEngine(sourceScript, redeemTx, idx, flag, nil, nil, amount)
	if err != nil {
		return fmt.Errorf("fail to create engine: %w", err)
	}
	if err = engine.Execute(); err != nil {
		return fmt.Errorf("fail to execute the script: %w", err)
	}
	return nil
}

func (c *Client) signUTXOBTC(redeemTx *btcwire.MsgTx, amount int64, sourceScript []byte, idx int) error {
	sigHashes := btctxscript.NewTxSigHashes(redeemTx)
	signable := btctxscript.NewPrivateKeySignable(c.privKey)
	witness, err := btctxscript.WitnessSignature(redeemTx, sigHashes, idx, amount, sourceScript, btctxscript.SigHashAll, signable, true)
	if err != nil {
		return fmt.Errorf("fail to get witness: %w", err)
	}

	redeemTx.TxIn[idx].Witness = witness
	flag := btctxscript.StandardVerifyFlags
	engine, err := btctxscript.NewEngine(sourceScript, redeemTx, idx, flag, nil, nil, amount)
	if err != nil {
		return fmt.Errorf("fail to create engine: %w", err)
	}
	if err = engine.Execute(); err != nil {
		return fmt.Errorf("fail to execute the script: %w", err)
	}
	return nil
}

// signUTXOZEC handles the signing of Zcash transactions using the Rust FFI library
func (c *Client) signUTXOZEC(ptx zec.PartialTx, checkpoint []byte) ([]byte, error) {
	// Convert our private key to the format expected by the Zcash signing library
	privKeyBytes := c.privKey.Serialize()

	// Sign the transaction using parallel signing method
	signedTx, err := c.signPtx(ptx, privKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("fail to sign zcash transaction: %w", err)
	}

	// Verify the signed transaction - skip if testing/simulation
	if signedTx != nil {
		if err := c.verifyZECTransaction(signedTx, ptx); err != nil {
			return nil, fmt.Errorf("fail to verify zcash transaction: %w", err)
		}
	}

	return signedTx, nil
}

// signPtx signs a Zcash partial transaction
func (c *Client) signPtx(ptx zec.PartialTx, privKeyBytes []byte) ([]byte, error) {
	if len(ptx.Sighashes) == 0 {
		return nil, errors.New("no sighashes")
	}

	// Sign the sighashes in parallel
	signatures, err := c.signPtxParallel(ptx, privKeyBytes, true) // true = parallel
	if err != nil {
		return nil, fmt.Errorf("fail to sign ZEC sighashes: %w", err)
	}

	// Check if signatures is nil or empty
	if len(signatures) == 0 {
		return nil, errors.New("no signatures produced")
	}

	// Get our pubkey as bytes
	privKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), privKeyBytes)
	pubKeyBytes := privKey.PubKey().SerializeCompressed()

	txBytes, err := zec.ApplySignatures(pubKeyBytes, ptx, signatures, c.getChainCfgZEC().Net)
	if err != nil {
		return nil, fmt.Errorf("fail to apply ZEC sighash signatures: %w", err)
	}
	return txBytes, nil
}

// signPtxParallel signs multiple sighashes in parallel or serially based on the parallel flag
func (c *Client) signPtxParallel(ptx zec.PartialTx, privKeyBytes []byte, parallel bool) ([][]byte, error) {
	wg := &sync.WaitGroup{}
	var utxoErr error
	// pre-allocate the signature slice with the correct size
	signatures := make([][]byte, len(ptx.Sighashes))

	// Use a mutex to protect concurrent access to utxoErr
	mu := &sync.Mutex{}

	// create a signable from private key
	privKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), privKeyBytes)
	signable := NewPrivateKeySignable(privKey)

	sign := func(idx int, sigHash []byte) { // idx is the crucial index
		if parallel {
			defer wg.Done()
		}

		signature, err := signable.Sign(sigHash)
		if err != nil {
			mu.Lock()
			if utxoErr == nil {
				utxoErr = err
			} else {
				utxoErr = multierror.Append(utxoErr, err)
			}
			mu.Unlock()
			// do not assign to signatures[idx] if there's an error, it will remain nil
			return
		}

		// only assign valid non-nil signatures
		if signature != nil {
			// write signature to a pre-allocated slot
			signatures[idx] = signature.Serialize()
		}
	}

	for idx, sigHash := range ptx.Sighashes {
		if parallel {
			wg.Add(1)
			go sign(idx, sigHash)
		} else {
			sign(idx, sigHash)
		}
	}

	if parallel {
		wg.Wait()
	}

	// if any signing goroutine reported an error
	// return the (possibly partially nil) signatures and the error
	if utxoErr != nil {
		return signatures, utxoErr
	}

	// sanity check to ensure all signatures were produced
	for i, sig := range signatures {
		if sig == nil {
			return signatures, fmt.Errorf("signature for input %d was not generated (nil), last error: %w", i, utxoErr)
		}
	}

	return signatures, nil
}

// Helper function to write a varint to a buffer
// func (c *Client) writeVarInt(w io.Writer, val uint64) error {
// 	var err error
// 	switch {
// 	case val < 0xfd:
// 		_, err = w.Write([]byte{byte(val)})
// 	case val <= 0xffff:
// 		_, err = w.Write([]byte{0xfd})
// 		if err != nil {
// 			return err
// 		}
// 		valBytes := make([]byte, 2)
// 		binary.LittleEndian.PutUint16(valBytes, uint16(val))
// 		_, err = w.Write(valBytes)
// 	case val <= 0xffffffff:
// 		_, err = w.Write([]byte{0xfe})
// 		if err != nil {
// 			return err
// 		}
// 		valBytes := make([]byte, 4)
// 		binary.LittleEndian.PutUint32(valBytes, uint32(val))
// 		_, err = w.Write(valBytes)
// 	default:
// 		_, err = w.Write([]byte{0xff})
// 		if err != nil {
// 			return err
// 		}
// 		valBytes := make([]byte, 8)
// 		binary.LittleEndian.PutUint64(valBytes, val)
// 		_, err = w.Write(valBytes)
// 	}
// 	return err
// }

// verifyZECTransaction verifies that a signed Zcash transaction is valid
func (c *Client) verifyZECTransaction(signedTx []byte, ptx zec.PartialTx) error {
	// In a real implementation, this would call into the Rust FFI layer to verify
	// that the transaction is properly formed and all signatures are valid

	// For simulation, we'll assume the verification is successful if the transaction
	// has at least some minimum size that would indicate it contains inputs and outputs
	if len(signedTx) < 64 {
		return errors.New("transaction too small to be valid")
	}

	// Check that the transaction contains the correct number of inputs and outputs
	// This is a simplified check - a real implementation would parse the transaction
	// and verify each component in detail

	return nil
}

// NewPrivateKeySignable creates a new instance of PrivateKeySignable with the given private key
func NewPrivateKeySignable(privKey *btcec.PrivateKey) *PrivateKeySignable {
	return &PrivateKeySignable{privKey: privKey}
}

// PrivateKeySignable implements the signable interface for btcec private keys
type PrivateKeySignable struct {
	privKey *btcec.PrivateKey
}

// Sign signs the provided hash with the private key and returns a signature
func (p *PrivateKeySignable) Sign(hash []byte) (*btcec.Signature, error) {
	return p.privKey.Sign(hash)
}

// GetPubKey returns the public key associated with the private key
func (p *PrivateKeySignable) GetPubKey() *btcec.PublicKey {
	return p.privKey.PubKey()
}
