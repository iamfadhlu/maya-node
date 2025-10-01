package thorchain

import (

	// tcTypes "gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/thorchain/thorchain"
	mayachaintypes "gitlab.com/mayachain/mayanode/x/mayachain/types"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	cKeys "github.com/cosmos/cosmos-sdk/crypto/keyring"
	ctypes "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/rs/zerolog/log"

	"gitlab.com/mayachain/mayanode/bifrost/metrics"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/kuji/wasm"
	"gitlab.com/mayachain/mayanode/config"

	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/bifrost/mayaclient"
	"gitlab.com/mayachain/mayanode/cmd"
	"gitlab.com/mayachain/mayanode/common"
)

// -------------------------------------------------------------------------------------
// Mock FeeTx
// -------------------------------------------------------------------------------------

type MockFeeTx struct {
	fee ctypes.Coins
	gas uint64
}

func (m *MockFeeTx) GetMsgs() []ctypes.Msg {
	return nil
}

func (m *MockFeeTx) ValidateBasic() error {
	return nil
}

func (m *MockFeeTx) GetGas() uint64 {
	return m.gas
}

func (m *MockFeeTx) GetFee() ctypes.Coins {
	return m.fee
}

func (m *MockFeeTx) FeePayer() ctypes.AccAddress {
	return nil
}

func (m *MockFeeTx) FeeGranter() ctypes.AccAddress {
	return nil
}

// -------------------------------------------------------------------------------------
// Tests
// -------------------------------------------------------------------------------------

type BlockScannerTestSuite struct {
	m      *metrics.Metrics
	bridge mayaclient.MayachainBridge
	keys   *mayaclient.Keys
}

var _ = Suite(&BlockScannerTestSuite{})

func (s *BlockScannerTestSuite) SetUpSuite(c *C) {
	s.m = GetMetricForTest(c)
	c.Assert(s.m, NotNil)
	cfg := config.BifrostClientConfiguration{
		ChainID:         "thorchain",
		ChainHost:       "localhost",
		SignerName:      "bob",
		SignerPasswd:    "password",
		ChainHomeFolder: "",
	}

	kb := cKeys.NewInMemory()
	_, _, err := kb.NewMnemonic(cfg.SignerName, cKeys.English, cmd.BASEChainHDPath, cfg.SignerPasswd, hd.Secp256k1)
	c.Assert(err, IsNil)
	thorKeys := mayaclient.NewKeysWithKeybase(kb, cfg.SignerName, cfg.SignerPasswd)
	c.Assert(err, IsNil)
	s.bridge, err = mayaclient.NewMayachainBridge(cfg, s.m, thorKeys)
	c.Assert(err, IsNil)
	s.keys = thorKeys
}

// TC has a fixed fee
// func (s *BlockScannerTestSuite) TestCalculateAverageGasFees(c *C) {
// 	cfg := config.BifrostBlockScannerConfiguration{ChainID: common.THORChain}
// 	blockScanner := CosmosBlockScanner{cfg: cfg}
//
// 	blockScanner.updateGasCache(&MockFeeTx{
// 		gas: GasLimit / 2,
// 		fee: ctypes.Coins{ctypes.NewCoin("rune", ctypes.NewInt(1000000))},
// 	})
// 	c.Check(len(blockScanner.feeCache), Equals, 1)
// 	c.Check(blockScanner.averageFee().Uint64(), Equals, uint64(2000000), Commentf("expected %s, got %d", blockScanner.averageFee().String(), 2000000))
//
// 	blockScanner.updateGasCache(&MockFeeTx{
// 		gas: GasLimit / 2,
// 		fee: ctypes.Coins{ctypes.NewCoin("rune", ctypes.NewInt(1000000))},
// 	})
// 	c.Check(len(blockScanner.feeCache), Equals, 2)
// 	c.Check(blockScanner.averageFee().Uint64(), Equals, uint64(2000000), Commentf("expected %s, got %d", blockScanner.averageFee().String(), 2000000))
//
// 	// two blocks at half fee should average to 75% of last
// 	blockScanner.updateGasCache(&MockFeeTx{
// 		gas: GasLimit,
// 		fee: ctypes.Coins{ctypes.NewCoin("rune", ctypes.NewInt(1000000))},
// 	})
// 	blockScanner.updateGasCache(&MockFeeTx{
// 		gas: GasLimit,
// 		fee: ctypes.Coins{ctypes.NewCoin("rune", ctypes.NewInt(1000000))},
// 	})
// 	c.Check(len(blockScanner.feeCache), Equals, 4)
// 	c.Check(blockScanner.averageFee().Uint64(), Equals, uint64(1500000), Commentf("expected %s, got %d", blockScanner.averageFee().String(), 1500000))
//
// 	// skip transactions with multiple coins
// 	blockScanner.updateGasCache(&MockFeeTx{
// 		gas: GasLimit,
// 		fee: ctypes.Coins{
// 			ctypes.NewCoin("rune", ctypes.NewInt(1000000)),
// 			ctypes.NewCoin("deletethis", ctypes.NewInt(1000000)),
// 		},
// 	})
// 	c.Check(len(blockScanner.feeCache), Equals, 4)
// 	c.Check(blockScanner.averageFee().Uint64(), Equals, uint64(1500000), Commentf("expected %s, got %d", blockScanner.averageFee().String(), 15000))
//
// 	// skip transactions with zero fee
// 	blockScanner.updateGasCache(&MockFeeTx{
// 		gas: GasLimit,
// 		fee: ctypes.Coins{
// 			ctypes.NewCoin("rune", ctypes.NewInt(0)),
// 		},
// 	})
// 	c.Check(len(blockScanner.feeCache), Equals, 4)
// 	c.Check(blockScanner.averageFee().Uint64(), Equals, uint64(1500000), Commentf("expected %s, got %d", blockScanner.averageFee().String(), 15000))
//
// 	// ensure we only cache the transaction limit number of blocks
// 	for i := 0; i < GasCacheTransactions; i++ {
// 		blockScanner.updateGasCache(&MockFeeTx{
// 			gas: GasLimit,
// 			fee: ctypes.Coins{
// 				ctypes.NewCoin("rune", ctypes.NewInt(1000000)),
// 			},
// 		})
// 	}
// 	c.Check(len(blockScanner.feeCache), Equals, GasCacheTransactions)
// 	c.Check(blockScanner.averageFee().Uint64(), Equals, uint64(1000000), Commentf("expected %s, got %d", blockScanner.averageFee().String(), 15000))
// }

func (s *BlockScannerTestSuite) TestGetBlock(c *C) {
	cfg := config.BifrostBlockScannerConfiguration{ChainID: common.THORChain}

	blockScanner := CosmosBlockScanner{
		cfg: cfg,
		rpc: &mockCometBFTRPC{},
	}

	block, err := blockScanner.GetBlock(1)

	c.Assert(err, IsNil)
	c.Assert(len(block.Data.Txs), Equals, 3)
	c.Assert(block.Header.Height, Equals, int64(162))
}

func (s *BlockScannerTestSuite) TestProcessTxs(c *C) {
	cfg := config.BifrostBlockScannerConfiguration{ChainID: common.THORChain}

	registry := s.bridge.GetContext().InterfaceRegistry
	registry.RegisterImplementations((*ctypes.Msg)(nil), &mayachaintypes.MsgSend{})
	registry.RegisterImplementations((*ctypes.Msg)(nil), &banktypes.MsgSend{})
	registry.RegisterImplementations((*ctypes.Msg)(nil), &wasm.MsgExecuteContract{})

	cdc := codec.NewProtoCodec(registry)

	blockScanner := CosmosBlockScanner{
		cfg:    cfg,
		rpc:    &mockCometBFTRPC{},
		cdc:    cdc,
		logger: log.Logger.With().Str("module", "blockscanner").Str("chain", common.THORChain.String()).Logger(),
	}

	block, err := blockScanner.GetBlock(3)
	c.Assert(err, IsNil)

	txInItems, err := blockScanner.processTxs(3, block.Data.Txs)
	c.Assert(err, IsNil)
	c.Assert(len(txInItems), Equals, 2)
	// c.Assert(len(txInItems), Equals, 2) // TODO: change when MsgExecuteContract is implemented

	// /types.MsgSend
	c.Assert(txInItems[0].Sender, Equals, "tthor13wrmhnh2qe98rjse30pl7u6jxszjjwl4f6yycr")
	c.Assert(txInItems[0].To, Equals, "tthor1f6z8lvkkj4u0f4g0cz6akzhydn6d9yv64t7ss7")

	// /cosmos.bank.v1beta1.MsgSend
	c.Assert(txInItems[1].To, Equals, "tthor1f6z8lvkkj4u0f4g0cz6akzhydn6d9yv64t7ss7")
	c.Assert(txInItems[1].Sender, Equals, "tthor13wrmhnh2qe98rjse30pl7u6jxszjjwl4f6yycr")
}
