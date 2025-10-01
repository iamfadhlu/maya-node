package kuji

import (
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	cKeys "github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	btypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"gitlab.com/mayachain/mayanode/bifrost/mayaclient"
	stypes "gitlab.com/mayachain/mayanode/bifrost/mayaclient/types"
	"gitlab.com/mayachain/mayanode/bifrost/metrics"
	"gitlab.com/mayachain/mayanode/cmd"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/config"
	. "gopkg.in/check.v1"
)

func TestPackage(t *testing.T) { TestingT(t) }

type KujiTestSuite struct {
	mayadir  string
	thorKeys *mayaclient.Keys
	bridge   mayaclient.MayachainBridge
	m        *metrics.Metrics
}

var _ = Suite(&KujiTestSuite{})

var m *metrics.Metrics

func GetMetricForTest(c *C) *metrics.Metrics {
	if m == nil {
		var err error
		m, err = metrics.NewMetrics(config.BifrostMetricsConfiguration{
			Enabled:      false,
			ListenPort:   9000,
			ReadTimeout:  time.Second,
			WriteTimeout: time.Second,
			Chains:       common.Chains{common.KUJIChain},
		})
		c.Assert(m, NotNil)
		c.Assert(err, IsNil)
	}
	return m
}

func (s *KujiTestSuite) SetUpSuite(c *C) {
	cosmosSDKConfg := cosmos.GetConfig()
	cosmosSDKConfg.SetBech32PrefixForAccount("smaya", "smayapub")

	s.m = GetMetricForTest(c)
	c.Assert(s.m, NotNil)
	ns := strconv.Itoa(time.Now().Nanosecond())
	c.Assert(os.Setenv("NET", "stagenet"), IsNil)

	s.mayadir = filepath.Join(os.TempDir(), ns, ".mayacli")
	cfg := config.BifrostClientConfiguration{
		ChainID:         "mayachain",
		ChainHost:       "localhost",
		SignerName:      "bob",
		SignerPasswd:    "password",
		ChainHomeFolder: s.mayadir,
	}

	kb := cKeys.NewInMemory()
	_, _, err := kb.NewMnemonic(cfg.SignerName, cKeys.English, cmd.BASEChainHDPath, cfg.SignerPasswd, hd.Secp256k1)
	c.Assert(err, IsNil)
	s.thorKeys = mayaclient.NewKeysWithKeybase(kb, cfg.SignerName, cfg.SignerPasswd)
	c.Assert(err, IsNil)
	s.bridge, err = mayaclient.NewMayachainBridge(cfg, s.m, s.thorKeys)
	c.Assert(err, IsNil)
}

func (s *KujiTestSuite) TearDownSuite(c *C) {
	c.Assert(os.Unsetenv("NET"), IsNil)
	if err := os.RemoveAll(s.mayadir); err != nil {
		c.Error(err)
	}
}

func (s *KujiTestSuite) TestGetAddress(c *C) {
	mockBankServiceClient := NewMockBankServiceClient()
	mockAccountServiceClient := NewMockAccountServiceClient()

	cc := KujiClient{
		cfg:           config.BifrostChainConfiguration{ChainID: common.KUJIChain},
		bankClient:    mockBankServiceClient,
		accountClient: mockAccountServiceClient,
	}

	addr := "kujira1nay0rxpjl2gk3nw7gmj3at50nc2xq3fnnf6jh4"
	atom, _ := common.NewAsset("KUJI.KUJI")
	expectedCoins := common.NewCoins(
		common.NewCoin(atom, cosmos.NewUint(496694100)),
	)

	acc, err := cc.GetAccountByAddress(addr, big.NewInt(0))
	c.Assert(err, IsNil)
	c.Check(acc.AccountNumber, Equals, int64(3530305))
	c.Check(acc.Sequence, Equals, int64(3))
	c.Check(acc.Coins.Equals(expectedCoins), Equals, true)

	pk := common.PubKey("smayapub1addwnpepqtwrfkp5xh77vy8jluajk0lcral3hr7ugfd83kfhu9kzhd2pdq5z6gtdymh")
	acc, err = cc.GetAccount(pk, big.NewInt(0))
	c.Assert(err, IsNil)
	c.Check(acc.AccountNumber, Equals, int64(3530305))
	c.Check(acc.Sequence, Equals, int64(3))
	c.Check(acc.Coins.Equals(expectedCoins), Equals, true)

	resultAddr := cc.GetAddress(pk)

	c.Logf(resultAddr)
	c.Check(addr, Equals, resultAddr)
}

func (s *KujiTestSuite) TestProcessOutboundTx(c *C) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
	}))

	client, err := NewCosmosClient(s.thorKeys, config.BifrostChainConfiguration{
		ChainID: common.KUJIChain,
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

	vaultPubKey, err := common.NewPubKey("smayapub1addwnpepqtwrfkp5xh77vy8jluajk0lcral3hr7ugfd83kfhu9kzhd2pdq5z6gtdymh")
	c.Assert(err, IsNil)
	outAsset, err := common.NewAsset("KUJI.KUJI")
	c.Assert(err, IsNil)
	toAddress, err := common.NewAddress("kujira1my9h2fzjsvt67ntdcd4zwf2p7828qz6f4ete2r", common.LatestVersion)
	c.Assert(err, IsNil)
	txOut := stypes.TxOutItem{
		Chain:       common.KUJIChain,
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

	expectedAmount := int64(245283)
	expectedDenom := "ukuji"
	c.Check(msg.Amount[0].Amount.Int64(), Equals, expectedAmount)
	c.Check(msg.Amount[0].Denom, Equals, expectedDenom)
	c.Logf(msg.FromAddress)
	c.Check(msg.FromAddress, Equals, "kujira1nay0rxpjl2gk3nw7gmj3at50nc2xq3fnnf6jh4")
	c.Check(msg.ToAddress, Equals, toAddress.String())
}

func (s *KujiTestSuite) TestSign(c *C) {
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
	interfaceRegistry.RegisterImplementations((*types.Msg)(nil), &btypes.MsgSend{})
	marshaler := codec.NewProtoCodec(interfaceRegistry)

	clientConfig := config.BifrostChainConfiguration{ChainID: common.KUJIChain}
	scannerConfig := config.BifrostBlockScannerConfiguration{ChainID: common.KUJIChain}
	txConfig := tx.NewTxConfig(marshaler, []signingtypes.SignMode{signingtypes.SignMode_SIGN_MODE_DIRECT})

	mockTmServiceClient := NewMockTmServiceClient()
	mockAccountServiceClient := NewMockAccountServiceClient()
	mockBankServiceClient := NewMockBankServiceClient()

	client := KujiClient{
		cfg:             clientConfig,
		txConfig:        txConfig,
		kujiScanner:     &KujiBlockScanner{cfg: scannerConfig, tmService: mockTmServiceClient},
		bankClient:      mockBankServiceClient,
		accountClient:   mockAccountServiceClient,
		chainID:         "columbus-5",
		localKeyManager: localKm,
		accts:           NewKujiMetaDataStore(),
	}

	vaultPubKey, err := common.NewPubKey(pk.String())
	c.Assert(err, IsNil)
	outAsset, err := common.NewAsset("KUJI.KUJI")
	c.Assert(err, IsNil)
	toAddress, err := common.NewAddress("kujira1my9h2fzjsvt67ntdcd4zwf2p7828qz6f4ete2r", common.LatestVersion)
	c.Assert(err, IsNil)
	txOut := stypes.TxOutItem{
		Chain:       common.KUJIChain,
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

	gas := types.NewCoins(types.NewCoin("ukuji", types.NewInt(100)))

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
