package thorchain

import (
	// btypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"bytes"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	// tcTypes "gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/thorchain/thorchain"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	cKeys "github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/bifrost/mayaclient"
	stypes "gitlab.com/mayachain/mayanode/bifrost/mayaclient/types"
	"gitlab.com/mayachain/mayanode/bifrost/metrics"
	"gitlab.com/mayachain/mayanode/cmd"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/config"
	tcTypes "gitlab.com/mayachain/mayanode/x/mayachain/types"
)

func TestPackage(t *testing.T) { TestingT(t) }

type ThorTestSuite struct {
	thordir  string
	thorKeys *mayaclient.Keys
	bridge   mayaclient.MayachainBridge
	m        *metrics.Metrics
}

var _ = Suite(&ThorTestSuite{})

var m *metrics.Metrics

func GetMetricForTest(c *C) *metrics.Metrics {
	if m == nil {
		var err error
		m, err = metrics.NewMetrics(config.BifrostMetricsConfiguration{
			Enabled:      false,
			ListenPort:   9000,
			ReadTimeout:  time.Second,
			WriteTimeout: time.Second,
			Chains:       common.Chains{common.THORChain},
		})
		c.Assert(m, NotNil)
		c.Assert(err, IsNil)
	}
	return m
}

func (s *ThorTestSuite) SetUpSuite(c *C) {
	cosmosSDKConfg := cosmos.GetConfig()
	cosmosSDKConfg.SetBech32PrefixForAccount("tthor", "tthorpub")
	cosmosSDKConfg.Seal()

	s.m = GetMetricForTest(c)
	c.Assert(s.m, NotNil)
	ns := strconv.Itoa(time.Now().Nanosecond())
	c.Assert(os.Setenv("NET", "mocknet"), IsNil)

	s.thordir = filepath.Join(os.TempDir(), ns, ".thorcli")
	cfg := config.BifrostClientConfiguration{
		ChainID:         "thorchain",
		ChainHost:       "localhost",
		SignerName:      "bob",
		SignerPasswd:    "password",
		ChainHomeFolder: s.thordir,
	}

	kb := cKeys.NewInMemory()
	_, _, err := kb.NewMnemonic(cfg.SignerName, cKeys.English, cmd.BASEChainHDPath, cfg.SignerPasswd, hd.Secp256k1)
	c.Assert(err, IsNil)
	s.thorKeys = mayaclient.NewKeysWithKeybase(kb, cfg.SignerName, cfg.SignerPasswd)
	c.Assert(err, IsNil)
	s.bridge, err = mayaclient.NewMayachainBridge(cfg, s.m, s.thorKeys)
	c.Assert(err, IsNil)
}

func (s *ThorTestSuite) TearDownSuite(c *C) {
	c.Assert(os.Unsetenv("NET"), IsNil)
	if err := os.RemoveAll(s.thordir); err != nil {
		c.Error(err)
	}
}

func (s *ThorTestSuite) TestGetAddress(c *C) {
	mockBankServiceClient := NewMockBankServiceClient()
	mockAccountServiceClient := NewMockAccountServiceClient()

	cc := CosmosClient{
		cfg:           config.BifrostChainConfiguration{ChainID: common.THORChain},
		bankClient:    mockBankServiceClient,
		accountClient: mockAccountServiceClient,
	}

	addr := "tthor10tjz4ave7znpctgd2rfu6v2r6zkeup2datgqsc"

	rune, _ := common.NewAsset("THOR.RUNE")
	expectedCoins := common.NewCoins(
		common.NewCoin(rune, cosmos.NewUint(496694100)))

	acc, err := cc.GetAccountByAddress(addr, big.NewInt(0))
	c.Assert(err, IsNil)
	c.Check(acc.AccountNumber, Equals, int64(3530305))
	c.Check(acc.Sequence, Equals, int64(3))
	c.Check(acc.Coins.Equals(expectedCoins), Equals, true, Commentf("expected %v, got %v", expectedCoins, acc.Coins))

	pk := common.PubKey("tthorpub1addwnpepqf72ur2e8zk8r5augtrly40cuy94f7e663zh798tyms6pu2k8qdswaqd3uv")
	acc, err = cc.GetAccount(pk, big.NewInt(0))
	c.Assert(err, IsNil)
	c.Check(acc.AccountNumber, Equals, int64(3530305))
	c.Check(acc.Sequence, Equals, int64(3))
	c.Check(acc.Coins.Equals(expectedCoins), Equals, true)

	resultAddr := cc.GetAddress(pk)
	c.Check(addr, Equals, resultAddr)
}

func (s *ThorTestSuite) TestProcessOutboundTx(c *C) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jsonFile, err := os.Open("./test-data/status.json")
		if err != nil {
			c.Fatal("unable to load status.json")
		}
		defer jsonFile.Close()

		res, _ := io.ReadAll(jsonFile)
		if _, err := w.Write(res); err != nil {
			c.Fatal("unable to write /status", err)
		}
	})
	server := httptest.NewServer(h)

	client, err := NewCosmosClient(s.thorKeys, config.BifrostChainConfiguration{
		ChainID: common.THORChain,
		RPCHost: server.URL,
		// See https://github.com/grpc/grpc-go/pull/5732
		CosmosGRPCHost: "0.0.0.0:9090",
		CosmosGRPCTLS:  false,
		BlockScanner: config.BifrostBlockScannerConfiguration{
			RPCHost:          server.URL,
			StartBlockHeight: 1, // avoids querying thorchain for block height
			// See https://github.com/grpc/grpc-go/pull/5732
			CosmosGRPCHost: "0.0.0.0:9090",
			CosmosGRPCTLS:  false,
		},
	}, nil, s.bridge, s.m)
	c.Assert(err, IsNil)

	vaultPubKey, err := common.NewPubKey("tthorpub1addwnpepqda0q2avvxnferqasee42lu5492jlc4zvf6u264famvg9dywgq2kzmhfc7c")
	c.Assert(err, IsNil)
	outAsset, err := common.NewAsset("THOR.RUNE")
	c.Assert(err, IsNil)
	toAddress, err := common.NewAddress("tthor10tjz4ave7znpctgd2rfu6v2r6zkeup2datgqsc", common.LatestVersion)
	c.Assert(err, IsNil)
	txOut := stypes.TxOutItem{
		Chain:       common.THORChain,
		ToAddress:   toAddress,
		VaultPubKey: vaultPubKey,
		Coins:       common.Coins{common.NewCoin(outAsset, cosmos.NewUint(24528352))},
		Memo:        "memo",
		MaxGas:      common.Gas{common.NewCoin(outAsset, cosmos.NewUint(235824))},
		GasRate:     750000,
		InHash:      "hash",
	}

	msg, err := client.processOutboundTx(txOut, 0)
	c.Assert(err, IsNil)

	expectedAmount := int64(24528352)
	expectedDenom := "rune"
	c.Check(msg.Amount[0].Amount.Int64(), Equals, expectedAmount)
	c.Check(msg.Amount[0].Denom, Equals, expectedDenom)
	// got from using cosmos AccAddressFromBech32 since we use our own implementation we use what their result
	// as the expected value
	fromBytes := []byte{86, 172, 20, 229, 203, 254, 61, 212, 2, 67, 115, 126, 167, 37, 15, 89, 172, 27, 195, 174}
	toBytes := []byte{122, 228, 42, 245, 153, 240, 166, 28, 45, 13, 80, 211, 205, 49, 67, 208, 173, 158, 5, 77}
	c.Check(bytes.Equal(msg.FromAddress.Bytes(), fromBytes), Equals, true, Commentf("expected %s, got %s", fromBytes, msg.FromAddress.Bytes()))
	c.Check(bytes.Equal(msg.ToAddress.Bytes(), toBytes), Equals, true, Commentf("expected %s, got %s", toBytes, msg.ToAddress.Bytes()))
}

func (s *ThorTestSuite) TestSign(c *C) {
	priv, err := s.thorKeys.GetPrivateKey()
	c.Assert(err, IsNil)

	temp, err := cryptocodec.ToTmPubKeyInterface(priv.PubKey())
	c.Assert(err, IsNil)

	pk, err := common.NewPubKeyFromCrypto(temp)
	c.Assert(err, IsNil)

	localKm := &keyManager{
		privKey: priv,
		addr:    types.AccAddress(priv.PubKey().Address()),
		pubkey:  pk,
	}

	interfaceRegistry := codectypes.NewInterfaceRegistry()
	interfaceRegistry.RegisterImplementations((*types.Msg)(nil), &tcTypes.MsgSend{})
	marshaler := codec.NewProtoCodec(interfaceRegistry)

	clientConfig := config.BifrostChainConfiguration{ChainID: common.THORChain}
	scannerConfig := config.BifrostBlockScannerConfiguration{ChainID: common.THORChain}
	txConfig := tx.NewTxConfig(marshaler, []signingtypes.SignMode{signingtypes.SignMode_SIGN_MODE_DIRECT})

	mockAccountServiceClient := NewMockAccountServiceClient()
	mockBankServiceClient := NewMockBankServiceClient()

	client := CosmosClient{
		cfg:             clientConfig,
		txConfig:        txConfig,
		cosmosScanner:   &CosmosBlockScanner{cfg: scannerConfig, rpc: &mockCometBFTRPC{}},
		bankClient:      mockBankServiceClient,
		accountClient:   mockAccountServiceClient,
		chainID:         "thorchain",
		localKeyManager: localKm,
		accts:           NewCosmosMetaDataStore(),
	}

	vaultPubKey, err := common.NewPubKey(pk.String())
	c.Assert(err, IsNil)
	outAsset, err := common.NewAsset("THOR.RUNE")
	c.Assert(err, IsNil)
	toAddress, err := common.NewAddress("tthor10tjz4ave7znpctgd2rfu6v2r6zkeup2datgqsc", common.LatestVersion)
	c.Assert(err, IsNil)
	txOut := stypes.TxOutItem{
		Chain:       common.THORChain,
		ToAddress:   toAddress,
		VaultPubKey: vaultPubKey,
		Coins:       common.Coins{common.NewCoin(outAsset, cosmos.NewUint(24528352))},
		Memo:        "memo",
		MaxGas:      common.Gas{common.NewCoin(outAsset, cosmos.NewUint(235824))},
		GasRate:     750000,
		InHash:      "hash",
	}

	msg, err := client.processOutboundTx(txOut, 1)
	c.Assert(err, IsNil)

	meta := client.accts.Get(pk)
	c.Check(meta.AccountNumber, Equals, int64(0))
	c.Check(meta.SeqNumber, Equals, int64(0))

	gas := types.NewCoins(types.NewCoin("rune", types.NewInt(100)))

	txb, err := buildUnsigned(
		txConfig,
		msg,
		vaultPubKey,
		"memo",
		gas,
		uint64(meta.AccountNumber),
		uint64(meta.SeqNumber),
	)
	c.Assert(err, IsNil)
	c.Check(txb.GetTx().GetFee().IsEqual(gas), Equals, true)
	c.Check(txb.GetTx().GetMemo(), Equals, "memo")
	pks, err := txb.GetTx().GetPubKeys()
	c.Assert(err, IsNil)

	c.Check(pks[0].Address().String(), Equals, priv.PubKey().Address().String())

	// Ensure the signature is present but tranasaction has not been signed yet
	sigs, err := txb.GetTx().GetSignaturesV2()
	c.Assert(err, IsNil)
	c.Check(sigs[0].PubKey.String(), Equals, priv.PubKey().String())
	sigData, ok := sigs[0].Data.(*signingtypes.SingleSignatureData)
	c.Check(ok, Equals, true)
	c.Check(sigData.SignMode, Equals, signingtypes.SignMode_SIGN_MODE_DIRECT)
	c.Check(len(sigData.Signature), Equals, 0)

	// Sign the message
	_, err = client.signMsg(
		txb,
		vaultPubKey,
		uint64(meta.AccountNumber),
		uint64(meta.SeqNumber),
	)
	c.Assert(err, IsNil)
}

func (s *ThorTestSuite) TestGetThorchainChainID(c *C) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jsonFile, err := os.Open("./test-data/status.json")
		if err != nil {
			c.Fatal("unable to load status.json")
		}
		defer jsonFile.Close()

		res, _ := io.ReadAll(jsonFile)
		if _, err := w.Write(res); err != nil {
			c.Fatal("unable to write /status", err)
		}
	})
	server := httptest.NewServer(h)
	defer server.Close()

	chainID, err := getThorchainsChainID(server.URL)
	c.Assert(err, IsNil)
	c.Assert(chainID, Equals, "thorchain-mainnet-v2")
}
