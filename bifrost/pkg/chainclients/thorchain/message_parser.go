package thorchain

import (
	"fmt"
	"time"

	abcitypes "github.com/cometbft/cometbft/abci/types"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	ctypes "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"gitlab.com/mayachain/mayanode/common/cosmos"

	"gitlab.com/mayachain/mayanode/bifrost/mayaclient/types"
	"gitlab.com/mayachain/mayanode/common"
	mtypes "gitlab.com/mayachain/mayanode/x/mayachain/types"
)

// TxContext contains all transaction-related data
type TxContext struct {
	Hash   string
	Height int64
	Index  int
	Memo   string
	Fees   cosmos.Coins
}

// BlockInfo contains block-specific data and results
type BlockInfo struct {
	Height    int64
	Time      time.Time
	Results   *coretypes.ResultBlockResults
	GasUsed   int64
	GasWanted int64
}

// EventInfo contains parsed event data
type EventInfo struct {
	Type       string
	Attributes map[string]string
}

// TxResult wraps the ABCI transaction result
type TxResult struct {
	Code      uint32
	Log       string
	GasUsed   int64
	GasWanted int64
	Events    []EventInfo
}

// TxData contains all context needed for processing messages
type TxData struct {
	Tx        TxContext
	Block     BlockInfo
	TxResult  TxResult
	RawEvents []abcitypes.Event // Original events for custom processing
}

// NewTxData creates a new ProcessableMsg from raw components
func NewTxData(hash string, height int64, index int, memo string, fees cosmos.Coins, blockResults *coretypes.ResultBlockResults) TxData {
	txResult := blockResults.TxsResults[index]

	// Parse events into our structure
	events := make([]EventInfo, 0, len(txResult.Events))
	for _, event := range txResult.Events {
		attrs := make(map[string]string)
		for _, attr := range event.Attributes {
			attrs[attr.Key] = attr.Value
		}
		events = append(events, EventInfo{
			Type:       event.Type,
			Attributes: attrs,
		})
	}

	return TxData{
		Tx: TxContext{
			Hash:   hash,
			Height: height,
			Index:  index,
			Memo:   memo,
			Fees:   fees,
		},
		Block: BlockInfo{
			Height:    height,
			Results:   blockResults,
			GasUsed:   txResult.GasUsed,
			GasWanted: txResult.GasWanted,
		},
		TxResult: TxResult{
			Code:      txResult.Code,
			Log:       txResult.Log,
			GasUsed:   txResult.GasUsed,
			GasWanted: txResult.GasWanted,
			Events:    events,
		},
		RawEvents: txResult.Events,
	}
}

/****************************************
* /types.MsgSend
****************************************/

func processThorchainMsgSend(msg *mtypes.MsgSend, txData TxData, asset common.Asset, lastFee ctypes.Uint) (types.TxInItem, error) {
	// Transaction contains a relevant MsgSend, check if the transaction was successful...
	if txData.TxResult.Code != 0 {
		return types.TxInItem{}, fmt.Errorf("transaction failed with code: %d, log: %s", txData.TxResult.Code, txData.TxResult.Log)
	}

	// Convert cosmos coins to thorchain coins (taking into account asset decimal precision)
	coins := common.Coins{}
	for _, coin := range msg.Amount {
		cCoin, err := fromThorchainToMayachain(coin)
		if err != nil {
			return types.TxInItem{}, fmt.Errorf("unable to convert coin: %w", err)
		}
		coins = append(coins, cCoin)
	}

	// Ignore the tx when no coins exist
	if coins.IsEmpty() {
		return types.TxInItem{}, fmt.Errorf("no valid coins in transaction")
	}

	// Convert cosmos gas to thorchain coins (taking into account gas asset decimal precision)
	gasFees := common.Gas{}
	for _, fee := range txData.Tx.Fees {
		cCoin, err := fromThorchainToMayachain(fee)
		if err != nil {
			return types.TxInItem{}, fmt.Errorf("unable to convert coin: %w", err)
		}
		gasFees = append(gasFees, cCoin)
	}

	if gasFees.IsEmpty() {
		gasFees = append(gasFees, common.NewCoin(asset, lastFee))
	}

	// Change AccAddress to strings
	// Can't use AccAddress.String() because uses cosmos config with Maya prefixes
	fromAddr, err := accAddressToString(msg.FromAddress, common.THORChain.AddressPrefix(common.CurrentChainNetwork))
	if err != nil {
		return types.TxInItem{}, fmt.Errorf("unable to convert from_address: %w", err)
	}

	toAddr, err := accAddressToString(msg.ToAddress, common.THORChain.AddressPrefix(common.CurrentChainNetwork))
	if err != nil {
		return types.TxInItem{}, fmt.Errorf("unable to convert to_address: %w", err)
	}

	return types.TxInItem{
		Tx:          txData.Tx.Hash,
		BlockHeight: txData.Tx.Height,
		Memo:        txData.Tx.Memo,
		Sender:      fromAddr,
		To:          toAddr,
		Coins:       coins,
		Gas:         gasFees,
	}, nil
}

/****************************************
* /cosmos.bank.v1beta1.MsgSend
****************************************/

func processBankMsgSend(msg *banktypes.MsgSend, txData TxData, asset common.Asset, lastFee ctypes.Uint) (types.TxInItem, error) {
	// Check if transaction was successful
	if txData.TxResult.Code != 0 {
		return types.TxInItem{}, fmt.Errorf("transaction failed with code: %d, log: %s", txData.TxResult.Code, txData.TxResult.Log)
	}

	// Process coins
	coins := common.Coins{}
	for _, coin := range msg.Amount {
		cCoin, err := fromThorchainToMayachain(coin)
		if err != nil {
			return types.TxInItem{}, fmt.Errorf("unable to convert coin: %w", err)
		}
		coins = append(coins, cCoin)
	}

	// Return early if no valid coins
	if coins.IsEmpty() {
		return types.TxInItem{}, fmt.Errorf("no valid coins in transaction")
	}

	// Convert cosmos gas to thorchain coins (taking into account gas asset decimal precision)
	gasFees := common.Gas{}
	for _, fee := range txData.Tx.Fees {
		cCoin, err := fromThorchainToMayachain(fee)
		if err != nil {
			return types.TxInItem{}, fmt.Errorf("unable to convert coin: %w", err)
		}
		gasFees = append(gasFees, cCoin)
	}

	if gasFees.IsEmpty() {
		gasFees = append(gasFees, common.NewCoin(asset, lastFee))
	}

	// Create single TxInItem for the send
	txIn := types.TxInItem{
		Tx:          txData.Tx.Hash,
		BlockHeight: txData.Block.Height,
		Memo:        txData.Tx.Memo,
		Sender:      msg.FromAddress,
		To:          msg.ToAddress,
		Coins:       coins,
		Gas:         gasFees,
	}

	return txIn, nil
}

/****************************************
 * MsgExecuteContractProcessor
****************************************/

// func processWasmMsgExecuteContract(msg *wasmtypes.MsgWasmExec, txData *TxData) ([]types.TxInItem, error) {
// 	// Check if contract is whitelisted
// 	if !whitelistedContracts[msg.Contract] {
// 		return nil, fmt.Errorf("contract not whitelisted: %s", msg.Contract)
// 	}
//
// 	// Check if transaction was successful
// 	if txData.TxResult.Code != 0 {
// 		return nil, fmt.Errorf("transaction failed with code: %d", txData.TxResult.Code)
// 	}
//
// 	var execMsgMap map[string]interface{}
// 	if err := json.Unmarshal(msg.Msg, &execMsgMap); err != nil {
// 		p.logger.Error().Err(err).Msg("Failed to unmarshal execute_msg.")
// 		return nil
// 	}
//
// 	var txInItems []types.TxInItem
// 	// Process transfer events from the transaction
// 	transferItems, err := processTransferEvents(msg, txData)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to process transfer events: %w", err)
// 	} else {
// 		txInItems = append(txInItems, transferItems...)
// 	}
//
// 	return txInItems, nil
// }
//
// func processTransferEvents(_ *wasmtypes.MsgWasmExec, txData *TxData) ([]types.TxInItem, error) {
// 	var items []types.TxInItem
//
// 	// Get events from transaction result
// 	events := txData.TxResult.Events
//
// 	for _, event := range events {
// 		fmt.Println("event: ", event)
// 	}
//
// 	return items, nil
// }
//
// func createTxInItemFromEvent(attrs map[string]string, msg *wasmtypes.MsgWasmExec, txData *TxData) (*types.TxInItem, error) {
// 	// Extract transfer details from attributes
// 	from := attrs["from"]
// 	to := attrs["to"]
// 	coinString := attrs["amount"]
//
// 	ccoins, err := cosmos.ParseCoins(coinString)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to parse coins: %w", err)
// 	}
//
// 	coins := make(common.Coins, len(ccoins))
// 	for _, ccoin := range ccoins {
// 		coin, err := fromThorchainToMayachain(ccoin)
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to convert coin: %w", err)
// 		}
// 		coins = append(coins, coin)
// 	}
//
// 	return &types.TxInItem{
// 		Tx:          txData.Tx.Hash,
// 		BlockHeight: txData.Block.Height,
// 		Memo:        txData.Tx.Memo,
// 		Sender:      from,
// 		To:          to,
// 		Coins:       coins,
// 		Gas:         txData.Fees,
// 	}, nil
// }
