package thorchain

import (
	abcitypes "github.com/cometbft/cometbft/abci/types"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	. "gopkg.in/check.v1"

	btypes "gitlab.com/mayachain/mayanode/bifrost/mayaclient/types"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	mtypes "gitlab.com/mayachain/mayanode/x/mayachain/types"
)

type MessageParserTestSuite struct{}

var _ = Suite(&MessageParserTestSuite{})

func (MessageParserTestSuite) TestNewTxData(c *C) {
	// Create sample data
	hash := "test_hash"
	height := int64(100)
	index := 0
	memo := "test_memo"
	fees := cosmos.NewCoins(cosmos.NewCoin("rune", cosmos.NewInt(1000)))

	// Create sample events
	events := []abcitypes.Event{
		{
			Type: "transfer",
			Attributes: []abcitypes.EventAttribute{
				{Key: "sender", Value: "addr1"},
				{Key: "recipient", Value: "addr2"},
				{Key: "amount", Value: "100rune"},
			},
		},
	}

	// Create block results
	blockResults := &coretypes.ResultBlockResults{
		TxsResults: []*abcitypes.ExecTxResult{
			{
				Code:      0,
				Log:       "success",
				GasUsed:   50000,
				GasWanted: 100000,
				Events:    events,
			},
		},
	}

	// Create TxData
	txData := NewTxData(hash, height, index, memo, fees, blockResults)

	// Verify TxContext
	c.Assert(hash, Equals, txData.Tx.Hash)
	c.Assert(height, Equals, txData.Tx.Height)
	c.Assert(index, Equals, txData.Tx.Index)
	c.Assert(memo, Equals, txData.Tx.Memo)
	c.Assert(fees.String(), Equals, txData.Tx.Fees.String())

	// Verify BlockInfo
	c.Assert(height, Equals, txData.Block.Height)
	c.Assert(blockResults, Equals, txData.Block.Results)
	c.Assert(int64(50000), Equals, txData.Block.GasUsed)
	c.Assert(int64(100000), Equals, txData.Block.GasWanted)

	// Verify TxResult
	c.Assert(uint32(0), Equals, txData.TxResult.Code)
	c.Assert("success", Equals, txData.TxResult.Log)
	c.Assert(int64(50000), Equals, txData.TxResult.GasUsed)
	c.Assert(int64(100000), Equals, txData.TxResult.GasWanted)

	// Verify Events
	c.Assert(len(txData.TxResult.Events), Equals, 1)
	c.Assert("transfer", Equals, txData.TxResult.Events[0].Type)
	c.Assert("addr1", Equals, txData.TxResult.Events[0].Attributes["sender"])
	c.Assert("addr2", Equals, txData.TxResult.Events[0].Attributes["recipient"])
	c.Assert("100rune", Equals, txData.TxResult.Events[0].Attributes["amount"])
}

func (MessageParserTestSuite) TestProcessThorchainMsgSend_Success(c *C) {
	// Create sample MsgSend
	fromAddr := types.AccAddress("thor1user1")
	toAddr := types.AccAddress("thor1user2")
	coins := types.NewCoins(types.NewCoin("rune", types.NewInt(1000)))
	msg := mtypes.NewMsgSend(fromAddr, toAddr, coins)

	txData := NewTxData("test_hash", 100, 0, "test_memo", cosmos.NewCoins(cosmos.NewCoin("rune", cosmos.NewInt(100))), &coretypes.ResultBlockResults{
		TxsResults: []*abcitypes.ExecTxResult{
			{
				Code: 0,
				Log:  "success",
			},
		},
	})

	// Process message
	result, err := processThorchainMsgSend(msg, txData, common.RUNEAsset, cosmos.NewUint(200000))
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)

	// Verify result
	c.Assert("test_hash", Equals, result.Tx)
	c.Assert(int64(100), Equals, result.BlockHeight)
	c.Assert("test_memo", Equals, result.Memo)
	c.Assert("1000 THOR.RUNE", Equals, result.Coins.String())
	c.Assert("100 THOR.RUNE", Equals, result.Gas.ToCoins().String())
	c.Assert(fromAddr.String(), Equals, result.Sender)
	c.Assert(toAddr.String(), Equals, result.To)
}

func (MessageParserTestSuite) TestProcessThorchainMsgSend_Failed(c *C) {
	// Create sample MsgSend
	fromAddr := types.AccAddress("thor1user1")
	toAddr := types.AccAddress("thor1user2")
	coins := types.NewCoins(types.NewCoin("rune", types.NewInt(1000)))
	msg := mtypes.NewMsgSend(fromAddr, toAddr, coins)

	// Create failed TxData
	txData := TxData{
		TxResult: TxResult{
			Code: 1,
			Log:  "insufficient funds",
		},
	}

	// Process message
	result, err := processThorchainMsgSend(msg, txData, common.RUNEAsset, cosmos.NewUint(200000))
	c.Assert(err, NotNil)
	c.Assert(result, DeepEquals, btypes.TxInItem{})
	c.Assert(err.Error(), Equals, "transaction failed with code: 1, log: insufficient funds")
}

func (MessageParserTestSuite) TestProcessBankMsgSend_Success(c *C) {
	// Create sample MsgSend
	fromAddr := "thor1user1"
	toAddr := "thor1user2"
	coins := types.NewCoins(types.NewCoin("rune", types.NewInt(1000)))
	msg := &banktypes.MsgSend{
		FromAddress: fromAddr,
		ToAddress:   toAddr,
		Amount:      coins,
	}

	txData := NewTxData("test_hash", 100, 0, "test_memo", cosmos.NewCoins(cosmos.NewCoin("rune", cosmos.NewInt(100))), &coretypes.ResultBlockResults{
		TxsResults: []*abcitypes.ExecTxResult{
			{
				Code: 0,
				Log:  "success",
			},
		},
	})

	// Process message
	result, err := processBankMsgSend(msg, txData, common.RUNEAsset, cosmos.NewUint(200000))
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)

	// Verify result
	c.Assert("test_hash", Equals, result.Tx)
	c.Assert(int64(100), Equals, result.BlockHeight)
	c.Assert("test_memo", Equals, result.Memo)
	c.Assert(fromAddr, Equals, result.Sender)
	c.Assert(toAddr, Equals, result.To)
	c.Assert(1, Equals, len(result.Coins))
	c.Assert(1, Equals, len(result.Gas))
}

func (MessageParserTestSuite) TestProcessBankMsgSend_Failed(c *C) {
	// Create sample MsgSend
	fromAddr := "thor1user1"
	toAddr := "thor1user2"
	coins := types.NewCoins(types.NewCoin("rune", types.NewInt(1000)))
	msg := banktypes.NewMsgSend(types.AccAddress(fromAddr), types.AccAddress(toAddr), coins)

	// Create failed TxData
	txData := TxData{
		TxResult: TxResult{
			Code: 1,
			Log:  "insufficient funds",
		},
	}

	// Process message
	result, err := processBankMsgSend(msg, txData, common.RUNEAsset, cosmos.NewUint(200000))
	c.Assert(err, NotNil)
	c.Assert(result, DeepEquals, btypes.TxInItem{})
	c.Assert(err.Error(), Equals, "transaction failed with code: 1, log: insufficient funds")
}
