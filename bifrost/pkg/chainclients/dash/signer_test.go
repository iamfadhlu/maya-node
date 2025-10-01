package dash

import (
	"encoding/base64"
	"encoding/hex"
	"net/http/httptest"
	"os"
	"path/filepath"

	"github.com/btcsuite/btcd/btcec"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	cKeys "github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/storage"
	ctypes "gitlab.com/mayachain/binance-sdk/common/types"
	dashec "gitlab.com/mayachain/dashd-go/btcec"
	"gitlab.com/mayachain/dashd-go/btcjson"
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/bifrost/mayaclient"
	stypes "gitlab.com/mayachain/mayanode/bifrost/mayaclient/types"
	"gitlab.com/mayachain/mayanode/bifrost/metrics"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/shared/utxo"
	"gitlab.com/mayachain/mayanode/cmd"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/config"
	types2 "gitlab.com/mayachain/mayanode/x/mayachain/types"
)

type DashSignerSuite struct {
	client *Client
	bridge mayaclient.MayachainBridge
	cfg    config.BifrostChainConfiguration
	m      *metrics.Metrics
	db     *leveldb.DB
	kb     cKeys.Keyring
}

var _ = Suite(&DashSignerSuite{})

// Static shared vars for signer tests
var (
	signerSharedBridge mayaclient.MayachainBridge
	signerSharedClient *Client
)

// SetUpSuite initializes shared resources that are reused across all tests
func (s *DashSignerSuite) SetUpSuite(c *C) {
	// Initialize shared HTTP server once for all tests if not already created
	if server == nil {
		server = httptest.NewServer(mockHttpResponses(c))
	}

	// Reuse the keybase created in the main suite if possible
	if keybase == nil {
		keybase = cKeys.NewInMemory()
		_, _, err := keybase.NewMnemonic(bob, cKeys.English, cmd.BASEChainHDPath, password, hd.Secp256k1)
		c.Assert(err, IsNil)
	}
	s.kb = keybase

	// Initialize metrics once
	s.m = GetMetricForTest(c)

	// Set up shared config for all tests
	s.cfg = config.BifrostChainConfiguration{
		ChainID:     "DASH",
		UserName:    "bob",
		Password:    "password",
		DisableTLS:  true,
		HTTPostMode: true,
		BlockScanner: config.BifrostBlockScannerConfiguration{
			StartBlockHeight: 1, // avoids querying mayachain for block height
		},
	}
	s.cfg.RPCHost = server.Listener.Addr().String()

	// Set up the initial network configuration
	types2.SetupConfigForTest()
	ctypes.Network = ctypes.TestNetwork

	// Create a shared maya directory
	mayadir := filepath.Join(os.TempDir(), "dash_signer_test_shared", ".mayacli")

	// Set up maya bridge config
	cfg := config.BifrostClientConfiguration{
		ChainID:         "mayachain",
		ChainHost:       server.Listener.Addr().String(),
		SignerName:      "bob",
		SignerPasswd:    "password",
		ChainHomeFolder: mayadir,
	}

	// Create shared bridge and client only once
	if signerSharedBridge == nil {
		mayaKeys := mayaclient.NewKeysWithKeybase(s.kb, cfg.SignerName, cfg.SignerPasswd)
		var err error
		signerSharedBridge, err = mayaclient.NewMayachainBridge(cfg, s.m, mayaKeys)
		c.Assert(err, IsNil)

		signerSharedClient, err = NewClient(mayaKeys, s.cfg, nil, signerSharedBridge, s.m)
		c.Assert(err, IsNil)

	}

	// Set the shared instances on the suite
	s.bridge = signerSharedBridge
	s.client = signerSharedClient

	// Create a new database for each test suite
	var err error
	memStorage := storage.NewMemStorage()
	db, err := leveldb.Open(memStorage, nil)
	c.Assert(err, IsNil)
	s.client.temporalStorage, err = utxo.NewTemporalStorage(db, 0)
	c.Assert(err, IsNil)
	s.db = db
	c.Assert(s.client, NotNil)
}

func (s *DashSignerSuite) TearDownSuite(c *C) {
	if s.db != nil {
		c.Assert(s.db.Close(), IsNil)
	}
}

func (s *DashSignerSuite) TestGetDASHPrivateKey(c *C) {
	input := "YjQwNGM1ZWM1ODExNmI1ZjBmZTEzNDY0YTkyZTQ2NjI2ZmM1ZGIxMzBlNDE4Y2JjZTk4ZGY4NmZmZTkzMTdjNQ=="
	buf, err := base64.StdEncoding.DecodeString(input)
	c.Assert(err, IsNil)
	c.Assert(buf, NotNil)
	prikeyByte, err := hex.DecodeString(string(buf))
	c.Assert(err, IsNil)
	pk := secp256k1.GenPrivKeyFromSecret(prikeyByte)
	dashPrivateKey, err := getDASHPrivateKey(pk)
	c.Assert(err, IsNil)
	c.Assert(dashPrivateKey, NotNil)
}

func (s *DashSignerSuite) TestSignTx(c *C) {
	txOutItem := stypes.TxOutItem{
		Chain:       common.BNBChain,
		ToAddress:   types2.GetRandomBNBAddress(),
		VaultPubKey: types2.GetRandomPubKey(),
		Coins: common.Coins{
			common.NewCoin(common.DASHAsset, cosmos.NewUint(10000000000)),
		},
		MaxGas: common.Gas{
			common.NewCoin(common.DASHAsset, cosmos.NewUint(1001)),
		},
		InHash:  "",
		OutHash: "",
	}
	// incorrect chain should return an error
	result, _, _, err := s.client.SignTx(txOutItem, 1)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)

	// invalid pubkey should return an error
	txOutItem.Chain = common.DASHChain
	txOutItem.VaultPubKey = common.PubKey("helloworld")
	result, _, _, err = s.client.SignTx(txOutItem, 2)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)

	// invalid to address should return an error
	txOutItem.VaultPubKey = types2.GetRandomPubKey()
	result, _, _, err = s.client.SignTx(txOutItem, 3)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)

	addr, err := types2.GetRandomPubKey().GetAddress(common.DASHChain)
	c.Assert(err, IsNil)
	txOutItem.ToAddress = addr

	// nothing to sign , because there is not enough UTXO
	result, _, _, err = s.client.SignTx(txOutItem, 4)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)

	// Test 6: Valid migrate transaction
	txOutItem.Coins[0].Amount = cosmos.NewUint(10673374) // this is exactly the sum of amounts of utxos - MaxGas
	txOutItem.MaxGas[0].Amount = cosmos.NewUint(1001)
	txOutItem.Memo = "MIGRATE:1024"
	txOutItem.VaultPubKey = s.client.nodePubKey
	signed1, signed2, obs, err2 := s.client.SignTx(txOutItem, 1)
	c.Assert(err2, IsNil)
	c.Assert(signed1, NotNil)
	c.Assert(signed2, IsNil)
	c.Assert(obs, NotNil)
}

func (s *DashSignerSuite) TestSignTxHappyPathWithPrivateKey(c *C) {
	addr, err := types2.GetRandomPubKey().GetAddress(common.DASHChain)
	c.Assert(err, IsNil)
	txOutItem := stypes.TxOutItem{
		Chain:       common.DASHChain,
		ToAddress:   addr,
		VaultPubKey: "tmayapub1addwnpepqw2k68efthm08f0f5akhjs6fk5j2pze4wkwt4fmnymf9yd463puru38eqd3",
		Coins: common.Coins{
			common.NewCoin(common.DASHAsset, cosmos.NewUint(10)),
		},
		MaxGas: common.Gas{
			common.NewCoin(common.DASHAsset, cosmos.NewUint(1000)),
		},
		InHash:  "",
		OutHash: "",
	}
	txHash := "256222fb25a9950479bb26049a2c00e75b89abbb7f0cf646c623b93e942c4c34"
	c.Assert(err, IsNil)
	blockMeta := utxo.NewBlockMeta("000000000000008a0da55afa8432af3b15c225cc7e04d32f0de912702dd9e2ae",
		100,
		"0000000000000068f0710c510e94bd29aa624745da43e32a1de887387306bfda")
	blockMeta.AddCustomerTransaction(txHash)
	c.Assert(s.client.temporalStorage.SaveBlockMeta(blockMeta.Height, blockMeta), IsNil)
	priKeyBuf, err := hex.DecodeString("b404c5ec58116b5f0fe13464a92e46626fc5db130e418cbce98df86ffe9317c5")
	c.Assert(err, IsNil)
	pkey, _ := btcec.PrivKeyFromBytes(btcec.S256(), priKeyBuf)
	c.Assert(pkey, NotNil)
	pkeyV2, _ := dashec.PrivKeyFromBytes(pkey.Serialize())
	ksw, err := NewKeySignWrapper(pkeyV2, s.client.keySignWrapper.tssKeyManager)
	c.Assert(err, IsNil)
	s.client.privateKey = pkey
	s.client.keySignWrapper = ksw
	vaultPubKey, err := GetBech32AccountPubKey(pkeyV2)
	c.Assert(err, IsNil)
	txOutItem.VaultPubKey = vaultPubKey
	buf, _, _, err := s.client.SignTx(txOutItem, 1)
	c.Assert(err, IsNil)
	c.Assert(buf, NotNil)
}

func (s *DashSignerSuite) TestSignTxWithoutPredefinedMaxGas(c *C) {
	addr, err := types2.GetRandomPubKey().GetAddress(common.DASHChain)
	c.Assert(err, IsNil)
	txOutItem := stypes.TxOutItem{
		Chain:       common.DASHChain,
		ToAddress:   addr,
		VaultPubKey: "tmayapub1addwnpepqw2k68efthm08f0f5akhjs6fk5j2pze4wkwt4fmnymf9yd463puru38eqd3",
		Coins: common.Coins{
			common.NewCoin(common.DASHAsset, cosmos.NewUint(10)),
		},
		Memo:    "YGGDRASIL-:101",
		GasRate: 25,
		InHash:  "",
		OutHash: "",
	}
	txHash := "256222fb25a9950479bb26049a2c00e75b89abbb7f0cf646c623b93e942c4c34"
	c.Assert(err, IsNil)
	blockMeta := utxo.NewBlockMeta("000000000000008a0da55afa8432af3b15c225cc7e04d32f0de912702dd9e2ae",
		100,
		"0000000000000068f0710c510e94bd29aa624745da43e32a1de887387306bfda")
	blockMeta.AddCustomerTransaction(txHash)
	c.Assert(s.client.temporalStorage.SaveBlockMeta(blockMeta.Height, blockMeta), IsNil)
	priKeyBuf, err := hex.DecodeString("b404c5ec58116b5f0fe13464a92e46626fc5db130e418cbce98df86ffe9317c5")
	c.Assert(err, IsNil)
	pkey, _ := btcec.PrivKeyFromBytes(btcec.S256(), priKeyBuf)
	c.Assert(pkey, NotNil)
	pkeyV2, _ := dashec.PrivKeyFromBytes(pkey.Serialize())
	ksw, err := NewKeySignWrapper(pkeyV2, s.client.keySignWrapper.tssKeyManager)
	c.Assert(err, IsNil)
	s.client.privateKey = pkey
	s.client.keySignWrapper = ksw
	vaultPubKey, err := GetBech32AccountPubKey(pkeyV2)
	c.Assert(err, IsNil)
	txOutItem.VaultPubKey = vaultPubKey
	buf, _, _, err := s.client.SignTx(txOutItem, 1)
	c.Assert(err, IsNil)
	c.Assert(buf, NotNil)

	c.Assert(s.client.temporalStorage.UpsertTransactionFee(0.001, 10), IsNil)
	buf, _, _, err = s.client.SignTx(txOutItem, 1)
	c.Assert(err, IsNil)
	c.Assert(buf, NotNil)
}

func (s *DashSignerSuite) TestBroadcastTx(c *C) {
	txOutItem := stypes.TxOutItem{
		Chain:       common.DASHChain,
		ToAddress:   types2.GetRandomDASHAddress(),
		VaultPubKey: types2.GetRandomPubKey(),
		Coins: common.Coins{
			common.NewCoin(common.DASHAsset, cosmos.NewUint(10)),
		},
		MaxGas: common.Gas{
			common.NewCoin(common.DASHAsset, cosmos.NewUint(1)),
		},
		InHash:  "",
		OutHash: "",
	}
	input := []byte("hello world")
	_, err := s.client.BroadcastTx(txOutItem, input)
	c.Assert(err, NotNil)
	input1, err := hex.DecodeString("01000000000103c7d45551ff54354be6711396560348ebbf273b989b542be36645568ed1dbecf10000000000ffffffff951ed70edc0bf2a4b3e1cbfe55d191a72850c5595c381309f69fc084c9af0b540100000000ffffffffc5db14c562b96bfd95f97d74a558a3e3b91841a96e1b09546208c9fb67424f420000000000ffffffff02231710000000000016001417acb08a31369e7666d94664d7e64f0e048220900000000000000000176a1574686f72636861696e3a636f6e736f6c6964617465024730440220756d15a363b78b070b583dfc1a6aba0dd605550407d5d3d92f5e785ef7e42aca02200db19dab144033c9c353481be30469da42c0c0a7580a513f49282bea77d7a29301210223da2ff73fa9b2258d335a4e63a4e7ef88211b8e800588280ed8b51e285ec0ff02483045022100a695f0fece36de02212b10bf6aa2f06dc6ef84ba30cae0c78749deddba1574530220315b490111c830c27e6cb810559c2a37cd00b123de82df79e061df26c8deb14301210223da2ff73fa9b2258d335a4e63a4e7ef88211b8e800588280ed8b51e285ec0ff0247304402207e586439b04985a90a53cf9fc511a53d86acece57b3e5571118562449d4f27ac02206d84f0fba1a68cf55efc8a1c2ec768924479b97ceaf2029ed6941176f004bf8101210223da2ff73fa9b2258d335a4e63a4e7ef88211b8e800588280ed8b51e285ec0ff00000000")
	c.Assert(err, IsNil)
	_, err = s.client.BroadcastTx(txOutItem, input1)
	c.Assert(err, IsNil)
}

func (s *DashSignerSuite) TestIsSelfTransaction(c *C) {
	c.Check(s.client.isSelfTransaction("66d2d6b5eb564972c59e4797683a1225a02515a41119f0a8919381236b63e948"), Equals, false)
	bm := utxo.NewBlockMeta("", 1500000, "")
	hash := "66d2d6b5eb564972c59e4797683a1225a02515a41119f0a8919381236b63e948"
	bm.AddSelfTransaction(hash)
	c.Assert(s.client.temporalStorage.SaveBlockMeta(1500000, bm), IsNil)
	c.Check(s.client.isSelfTransaction("66d2d6b5eb564972c59e4797683a1225a02515a41119f0a8919381236b63e948"), Equals, true)
}

func (s *DashSignerSuite) TestEstimateTxSize(c *C) {
	size := s.client.estimateTxSize("OUT:2180B871F2DEA2546E1403DBFE9C26B062ABAFFD979CF3A65F2B4D2230105CF1", []btcjson.ListUnspentResult{
		{
			TxID:      "66d2d6b5eb564972c59e4797683a1225a02515a41119f0a8919381236b63e948",
			Vout:      0,
			Spendable: true,
		},
		{
			TxID:      "c5946215d82d5870ba2b1e8f245d8aa1446783975aa3a592cf55589fccbf285f",
			Vout:      0,
			Spendable: true,
		},
	})
	c.Assert(size, Equals, int64(417))
}
