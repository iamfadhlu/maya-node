package zcash

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcutil"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	cKeys "github.com/cosmos/cosmos-sdk/crypto/keyring"
	ctypes "gitlab.com/mayachain/binance-sdk/common/types"

	"gitlab.com/mayachain/mayanode/bifrost/mayaclient"
	"gitlab.com/mayachain/mayanode/bifrost/mayaclient/types"
	"gitlab.com/mayachain/mayanode/bifrost/metrics"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/shared/utxo"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/zcash/rpc"
	"gitlab.com/mayachain/mayanode/chain/zec/go/zec"
	"gitlab.com/mayachain/mayanode/cmd"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"

	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/config"
	types2 "gitlab.com/mayachain/mayanode/x/mayachain/types"
)

func TestPackage(t *testing.T) { TestingT(t) }

type ZcashSuite struct {
	client       *Client
	bridge       mayaclient.MayachainBridge
	mayaServer   *httptest.Server
	zcashServer  *httptest.Server
	cfg          config.BifrostChainConfiguration
	m            *metrics.Metrics
	keys         *mayaclient.Keys
	zecdSnapshot string
}

var _ = Suite(&ZcashSuite{})

var (
	testToAddressForGeneratingPayloadForTestBroadcastTx = "tmPBsn8p1rxKUvxu7sgXsZXkdSt1GpB12HD"
	testToAddressTestnet                                = []string{"tmNUFAr71YAW3eXetm8fhx7k8zpUJYQiKZP", "tmWJQXMC6h9hh3ohjxnnS25o3pEFe1HR8zr"}
	testToAddressMainnet                                = []string{"t1aJu87TESh1aWd5pZSAd71XWnCRNfc1N4Y", "t1Vot4rGfSqZf3JjT8sY3i7yT1rF9rN5kQp"}
)

var testSnapshotWallets = []struct{ addr, privKeyWIF, pubKeyHex, pubKey string }{
	{
		addr:       "tmCtf4gJ8eb8mXh7hBk6DVW2byu41wPo1GN",
		privKeyWIF: "cPNv8b9wphTyDzGzD1TuZchopJY9nrxGqHmoU1dkw69ML1bRagZv",
		pubKeyHex:  "0390fe79d1719ab60369c9e710739aefb9d7db411733aa711b467b60232f47d81f",
		pubKey:     "tmayapub1addwnpepqwg0u7w3wxdtvqmfe8n3quu6a7ua0k6pzue65ugmgeakqge0glvp7t0p3ng",
	},
	{
		addr:       "tmLqsSEy6cDjcTnRUd3ENKu1wWAckuiVUFx",
		privKeyWIF: "cQ24hdAZXPXSP5DHmRADhLEYQ67NsTSpZS57AsDNM2wiho4n1CvQ",
		pubKeyHex:  "03a340c6b85f4a85c7d63c7c604237a94b388be547728b02ac9bd944c893afa55b",
		pubKey:     "tmayapub1addwnpepqw35p34cta9gt37k837xqs3h499n3zl9gaegkq4vn0v5fjyn47j4kuutge7",
	},
	{
		addr:       "tmAYgqddmQhUY6GnbUmCMyRwC7HcVnVZgWU",
		privKeyWIF: "cT5r3oWRA7aezz7EA9h68QzscJkL2P69faJ5hrZN35RRp5C5eZYK",
		pubKeyHex:  "021c85731e9c8318ec8d29247ccfd0ec5695953b4df978e8507e40bbedef72fcbb",
		pubKey:     "tmayapub1addwnpepqgwg2uc7njp33myd9yj8en7sa3tft9fmfhuh36zs0eqthm00wt7tkahw5jy",
	},
}

func (s *ZcashSuite) SetUpSuite(c *C) {
	types2.SetupConfigForTest()
	ctypes.Network = ctypes.TestNetwork

	s.m = GetMetricForTest(c)
	s.cfg = config.BifrostChainConfiguration{
		ChainID:     "ZEC",
		ChainHost:   "localhost",
		RPCHost:     "localhost:18232", // :8232 for zcash mainnet
		UserName:    "mayachain",
		Password:    "password",
		DisableTLS:  true,
		HTTPostMode: true,
		BlockScanner: config.BifrostBlockScannerConfiguration{
			StartBlockHeight: 1, // avoids querying mayachain for block height
		},
	}
	ns := strconv.Itoa(time.Now().Nanosecond())

	mayadir := filepath.Join(os.TempDir(), ns, ".mayacli")
	cfg := config.BifrostClientConfiguration{
		ChainID:         "ZEC",
		ChainHost:       "localhost",
		ChainRPC:        "localhost",
		SignerName:      "mayachain",
		SignerPasswd:    "password",
		ChainHomeFolder: mayadir,
	}

	types2.SetupConfigForTest()
	ctypes.Network = ctypes.TestNetwork

	kb := cKeys.NewInMemory()
	_, _, err := kb.NewMnemonic("mayachain", cKeys.English, cmd.BASEChainHDPath, "password", hd.Secp256k1)
	c.Assert(err, IsNil)
	s.keys = mayaclient.NewKeysWithKeybase(kb, cfg.SignerName, cfg.SignerPasswd)
	c.Assert(err, IsNil)

	s.setupMayaMockServer(c)
	cfg.ChainHost = s.mayaServer.Listener.Addr().String()
	s.bridge, err = mayaclient.NewMayachainBridge(cfg, s.m, s.keys)
	c.Assert(err, IsNil)

	s.zecdSnapshot = os.Getenv("USE_TEST_ZECD_SNAPSHOT")
	// z := newMockZec(s.zecdSnapshot != "")
	// s.zec = &z
	// if using Zcasd node, verify that it is running
	if s.zecdSnapshot != "" {
		// currently only 1 snapshot named "default" is accepted
		// located in test/zcash/snapshots/default
		c.Assert(s.zecdSnapshot, Equals, "default")
		// prepare the snapshot folder & start Zcashd daemon
		err = runScript("test/zcash/use_snapshot.sh", "../../../..", s.zecdSnapshot)
		c.Assert(err, IsNil, Commentf("Failed to run use_snapshot.sh %s: %v", s.zecdSnapshot, err))
	} else {
		s.setupZcashMockServer(c)
		// if Zacd node is not running, mock the rpc calls
		s.cfg.RPCHost = s.zcashServer.Listener.Addr().String()
	}
	s.client, err = NewClient(s.keys, s.cfg, nil, s.bridge, s.m)
	c.Assert(err, IsNil)
	c.Assert(s.client, NotNil)
	if s.zecdSnapshot != "" {
		s.verifyCurrentTestSnapshot(c)
	}
	/// c.Assert(zec.InitZec(), IsNil)
}

func (s *ZcashSuite) TearDownSuite(c *C) {
	if s.zecdSnapshot != "" {
		err := runScript("test/zcash/use_snapshot.sh", "../../../..", "stop")
		c.Assert(err, IsNil, Commentf("Failed to run use_snapshot.sh stop: %v", err))
	}
}

func (s *ZcashSuite) SetUpTest(c *C) {
	// select the first wallet as vault
	s.selectWallet(c, 0)
	// clean spent utxos
	c.Assert(s.client.temporalStorage.PruneBlockMeta(999999999999999, nil), IsNil)
}

func (s *ZcashSuite) restartZecd(c *C) {
	if s.zecdSnapshot != "" {
		// currently only 1 snapshot named "default" is accepted
		// located in test/zcash/snapshots/default
		c.Assert(s.zecdSnapshot, Equals, "default")
		// prepare the snapshot folder & start Zcashd daemon
		err := runScript("test/zcash/use_snapshot.sh", "../../../..", s.zecdSnapshot)
		c.Assert(err, IsNil, Commentf("Failed to run use_snapshot.sh %s: %v", s.zecdSnapshot, err))
		s.client, err = NewClient(s.keys, s.cfg, nil, s.bridge, s.m)
		c.Assert(err, IsNil)
		c.Assert(s.client, NotNil)
		s.verifyCurrentTestSnapshot(c)
	}
	// select the first wallet as vault
	s.selectWallet(c, 0)
}

// runScript executes a shell script and returns an error if it fails
func runScript(scriptPath, workDir string, args ...string) error {
	// cmd := exec.Command("/bin/sh", append([]string{scriptPath}, args...)...)
	cmd := exec.Command(scriptPath, args...)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// run the script
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("script execution failed: %v", err)
	}
	return nil
}

func httpTestHandler(c *C, rw http.ResponseWriter, fixture string) {
	content, err := os.ReadFile(fixture)
	if err != nil {
		c.Fatal(err)
	}
	if len(content) == 0 {
		c.Fatalf("invalid fixture file name %s", fixture)
	}
	rw.Header().Set("Content-Type", "application/json")
	if _, err := rw.Write(content); err != nil {
		c.Fatal(err)
	}
}

func httpTestStringHandler(c *C, rw http.ResponseWriter, fixture string) {
	content := []byte(fixture)
	if len(content) == 0 {
		c.Fatalf("invalid fixture file name %s", fixture)
	}
	rw.Header().Set("Content-Type", "application/json")
	if _, err := rw.Write(content); err != nil {
		c.Fatal(err)
	}
}

func (s *ZcashSuite) setupMayaMockServer(c *C) {
	s.mayaServer = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch {
		case strings.HasPrefix(req.RequestURI, "/mayachain/node/"):
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/nodeaccount/template.json")
		case req.RequestURI == "/mayachain/lastblock":
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/lastblock/btc.json")
		case strings.HasPrefix(req.RequestURI, "/auth/accounts/"):
			_, err := rw.Write([]byte(`{ "jsonrpc": "2.0", "id": "", "result": { "height": "0", "result": { "value": { "account_number": "0", "sequence": "0" } } } }`))
			c.Assert(err, IsNil)
		case req.RequestURI == "/txs":
			_, err := rw.Write([]byte(`{"height": "1", "txhash": "AAAA000000000000000000000000000000000000000000000000000000000000", "logs": [{"success": "true", "log": ""}]}`))
			c.Assert(err, IsNil)
		case strings.HasPrefix(req.RequestURI, mayaclient.AsgardVault):
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/vaults/asgard.json")
		case req.RequestURI == "/mayachain/mimir/key/MaxUTXOsToSpend":
			_, err := rw.Write([]byte(`-1`))
			c.Assert(err, IsNil)
		case req.RequestURI == "/mayachain/vaults/pubkeys":
			if common.CurrentChainNetwork == common.MainNet {
				httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/vaults/pubKeys-Mainnet.json")
			} else {
				httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/vaults/pubKeys.json")
			}
		case strings.HasPrefix(req.RequestURI, "/mayachain/vaults") && strings.HasSuffix(req.RequestURI, "/signers"):
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/tss/keysign_party.json")
		}
	}))
}

func (s *ZcashSuite) setupZcashMockServer(c *C) {
	s.zcashServer = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		getIntParam := func(params []interface{}, i int) int64 {
			c.Assert(len(params) >= i+1, Equals, true, Commentf("getblockhash requires 1 parameter"))
			numFloat, ok := params[0].(float64) // JSON numbers are unmarshaled as float64
			c.Assert(ok, Equals, true, Commentf("getblockhash: invalid parameter type: %T, expected numeric", params[0]))
			return int64(numFloat) // Convert float64 to int64 (height)
		}

		if req.RequestURI == "/" { // nolint
			r := struct {
				Method string        `json:"method"`
				Params []interface{} `json:"params"`
			}{}
			_ = json.NewDecoder(req.Body).Decode(&r)

			switch r.Method {
			case "getblockhash":
				height := getIntParam(r.Params, 0)
				if height == 182 {
					httpTestHandler(c, rw, "../../../../test/fixtures/btc/blockhash.json")
				} else {
					c.Fatalf("unexpected rpc call params for method %s %+v", r.Method, r.Params)
				}
			case "getblockcount":
				if common.CurrentChainNetwork.SoftEquals(common.MainNet) {
					httpTestStringHandler(c, rw, `{"result": 2730000, "error": null, "id": 3}`)
				} else {
					httpTestStringHandler(c, rw, `{"result": 186, "error": null, "id": 3}`)
				}
			case "sendrawtransaction":
				switch r.Params[0] {
				case "68656c6c6f20776f726c64": // "hello world"
					errorResponse := map[string]interface{}{
						"error": map[string]interface{}{
							"code":    -22,
							"message": "TX decode failed",
						},
						"result": nil,
						"id":     1,
					}
					rw.WriteHeader(500)
					_ = json.NewEncoder(rw).Encode(errorResponse)
					return
				case "050000800a27a7265510e7c800000000d300000001d9cff0f746eae15e3f59eb448f1e0d95785ce7f213a26922ba5168407970a3ea000000006b483045022100b32d055ad1926f452e1ceeb012491653ed2d55f872d2a8d378cb7ff27cb91bde022047af55d2907559c0b18ea86c973029ef4dbe5d22a8eb35e5ecac1fbf8d8108cd01210390fe79d1719ab60369c9e710739aefb9d7db411733aa711b467b60232f47d81fffffffff0264000000000000001976a91493ce48570b55c42c2af816aeaba06cfee1224fae88ac1cf06759000000001976a91422db56f1d0f603ca56d85c2cb7b65d18c2802d1488ac000000":
					httpTestStringHandler(c, rw, `{"result": "1e51b1c68520804a64e96e4d6614d141e1e68bd2eeaeb20a2e887c7f189c9116","error": null,"id": 3}`) // ++++
				case "050000800a27a7265510e7c80000000000000000022ee093d5edb298fa2134117e96da929738c44bdfe4aab3ecbcc139fb1f43572a010000006b483045022100d3835d6ad9b1d0af50274d257515d3e4294734377c7742e82c7bf0e9e2b081a9022046332eb575b95dd8fdf721a743d31446cc4a0e6e0716a47cec67fc50cd78fc9601210390fe79d1719ab60369c9e710739aefb9d7db411733aa711b467b60232f47d81fffffffffd9cff0f746eae15e3f59eb448f1e0d95785ce7f213a26922ba5168407970a3ea000000006a473044022020870505659e6e1bbc10a7ddb44b3109bee662725830e35f5f293547981862e902204da1f4db021788f788ae2cf9e9e933d88ac0bfa89042cb64e15d1e686ebd14bc01210390fe79d1719ab60369c9e710739aefb9d7db411733aa711b467b60232f47d81fffffffff0264000000000000001976a9148beeab42389192446176efebc352684fc6a095fa88acf4be6d83010000001976a91422db56f1d0f603ca56d85c2cb7b65d18c2802d1488ac000000":
					httpTestStringHandler(c, rw, `{"result": "3ccb75c8c24f0a04364d8472664dbf8a3772ad866fa9b569d012a7555378d394","error": null,"id": 1}`)
				case "050000800a27a7265510e7c80000000000000000022ee093d5edb298fa2134117e96da929738c44bdfe4aab3ecbcc139fb1f43572a010000006a473044022053b3a63cc7a011a9803a0e0dd2e04b7464b45b219e5b04fc12c9596b8d96726b02203c012b13906c7a3ebd77444b63808193060d6aa607a4300f5223c12941b70f4b01210390fe79d1719ab60369c9e710739aefb9d7db411733aa711b467b60232f47d81fffffffffd9cff0f746eae15e3f59eb448f1e0d95785ce7f213a26922ba5168407970a3ea000000006a47304402202d74c7c05145d351b7e0d5b63aa123ae8cca71c89e411bcc75ff23821b9d865702205dcc44097b1b2b64dcc54c2be2c64f861aa6bb1d57d3df5f1366fe3f562a991f01210390fe79d1719ab60369c9e710739aefb9d7db411733aa711b467b60232f47d81fffffffff0264000000000000001976a914e1d352d21f78478ff14ba9fe1bb4964019c45b3588acf4be6d83010000001976a91422db56f1d0f603ca56d85c2cb7b65d18c2802d1488ac000000":
					httpTestStringHandler(c, rw, `{"result": "c4af7c4d1612153ecee1bb108ad0d905b9628289b2f33d27bc21a7f51ea7eb28","error": null,"id": 1}`)
				default:
					c.Fatalf("unexpected rpc call params for method %s %+v", r.Method, r.Params)
				}
			case "getblock":
				if r.Params[0] == "000000008de7a25f64f9780b6c894016d2c63716a89f7c9e704ebb7e8377a0c8" { // height 182
					httpTestHandler(c, rw, "../../../../test/fixtures/zcash/block.json")
				}
			case "getrawtransaction":
				switch r.Params[0] {
				case "e0f77e8fb935ed8046aeacc1a9d3e524a0fc99734a62badd128c1af1c643cfe2":
					httpTestHandler(c, rw, "../../../../test/fixtures/zcash/tx-e0f7.json")
				case "eaa37079406851ba2269a213f2e75c78950d1e8f44eb593f5ee1ea46f7f0cfd9":
					httpTestHandler(c, rw, "../../../../test/fixtures/zcash/tx-eaa3.json")
				case "3005b0052fcd17a91da4ab975fed5fa1dd518477b37dd95a2dc015ef04538577":
					httpTestHandler(c, rw, "../../../../test/fixtures/zcash/tx-b6a5.json")
				default:
					c.Fatalf("unexpected rpc call params for method %s %+v", r.Method, r.Params)
				}
			case "decoderawtransaction":
				switch r.Params[0] {
				case "050000800a27a7265510e7c80000000000000000022ee093d5edb298fa2134117e96da929738c44bdfe4aab3ecbcc139fb1f43572a010000006b483045022100d3835d6ad9b1d0af50274d257515d3e4294734377c7742e82c7bf0e9e2b081a9022046332eb575b95dd8fdf721a743d31446cc4a0e6e0716a47cec67fc50cd78fc9601210390fe79d1719ab60369c9e710739aefb9d7db411733aa711b467b60232f47d81fffffffffd9cff0f746eae15e3f59eb448f1e0d95785ce7f213a26922ba5168407970a3ea000000006a473044022020870505659e6e1bbc10a7ddb44b3109bee662725830e35f5f293547981862e902204da1f4db021788f788ae2cf9e9e933d88ac0bfa89042cb64e15d1e686ebd14bc01210390fe79d1719ab60369c9e710739aefb9d7db411733aa711b467b60232f47d81fffffffff0264000000000000001976a9148beeab42389192446176efebc352684fc6a095fa88acf4be6d83010000001976a91422db56f1d0f603ca56d85c2cb7b65d18c2802d1488ac000000":
					httpTestStringHandler(c, rw, `{"result": {"txid": "3ccb75c8c24f0a04364d8472664dbf8a3772ad866fa9b569d012a7555378d394"},"error": null,"id": 3}`)
				case "050000800a27a7265510e7c80000000000000000022ee093d5edb298fa2134117e96da929738c44bdfe4aab3ecbcc139fb1f43572a010000006a473044022053b3a63cc7a011a9803a0e0dd2e04b7464b45b219e5b04fc12c9596b8d96726b02203c012b13906c7a3ebd77444b63808193060d6aa607a4300f5223c12941b70f4b01210390fe79d1719ab60369c9e710739aefb9d7db411733aa711b467b60232f47d81fffffffffd9cff0f746eae15e3f59eb448f1e0d95785ce7f213a26922ba5168407970a3ea000000006a47304402202d74c7c05145d351b7e0d5b63aa123ae8cca71c89e411bcc75ff23821b9d865702205dcc44097b1b2b64dcc54c2be2c64f861aa6bb1d57d3df5f1366fe3f562a991f01210390fe79d1719ab60369c9e710739aefb9d7db411733aa711b467b60232f47d81fffffffff0264000000000000001976a914e1d352d21f78478ff14ba9fe1bb4964019c45b3588acf4be6d83010000001976a91422db56f1d0f603ca56d85c2cb7b65d18c2802d1488ac000000":
					httpTestStringHandler(c, rw, `{"result": {"txid": "c4af7c4d1612153ecee1bb108ad0d905b9628289b2f33d27bc21a7f51ea7eb28"},"error": null,"id": 3}`)
				default:
					c.Fatalf("unexpected rpc call params for method %s %+v", r.Method, r.Params)
				}
			case "getaddressbalance":
				httpTestHandler(c, rw, "../../../../test/fixtures/zcash/balance.json")
			case "getaddressutxos":
				var requestData struct {
					Addresses []string `json:"addresses"`
					ChainInfo bool     `json:"chainInfo"`
				}
				jsonData, err := json.Marshal(r.Params[0])
				if err == nil {
					err = json.Unmarshal(jsonData, &requestData)
				}
				if err != nil {
					httpTestHandler(c, rw, "../../../../test/fixtures/zcash/utxos.json")
				} else {
					switch requestData.Addresses[0] {
					case "tmLqsSEy6cDjcTnRUd3ENKu1wWAckuiVUFx":
						httpTestHandler(c, rw, "../../../../test/fixtures/zcash/utxos-fx.json") // used
					case "tmAYgqddmQhUY6GnbUmCMyRwC7HcVnVZgWU":
						httpTestHandler(c, rw, "../../../../test/fixtures/zcash/utxos-wu.json") // used
					case "tmCtf4gJ8eb8mXh7hBk6DVW2byu41wPo1GN":
						httpTestHandler(c, rw, "../../../../test/fixtures/zcash/utxos-gn.json") // used
					default:
						httpTestHandler(c, rw, "../../../../test/fixtures/zcash/utxos.json")
					}
				}
			case "generate":
				httpTestStringHandler(c, rw, `{"result": ["0e32bdebf0e4ecdeb3edb88767cb09193b54e2c2307f77eb2fe0211c47d1deae"],"error": null,"id": 3}`)
			default:
				c.Fatalf("unexpected rpc call params for method %s %+v", r.Method, r.Params)
			}
		}
	}))
}

func (s *ZcashSuite) verifyCurrentTestSnapshot(c *C) {
	for _, w := range testSnapshotWallets {
		privKeyWIF := w.privKeyWIF
		wif, err := btcutil.DecodeWIF(privKeyWIF)
		c.Assert(err, IsNil)
		// privKeyBytes := wif.PrivKey.Serialize()
		pubKey := btcec.PublicKey(wif.PrivKey.PublicKey)
		nodePubKey, err := pubKeyFromBtcecPubKey(&pubKey)
		c.Assert(err, IsNil)
		pubKeyBytes := pubKey.SerializeCompressed()
		pubKeyHex := fmt.Sprintf("%x", pubKeyBytes)
		c.Assert(pubKeyHex, Equals, w.pubKeyHex)

		if true { // test
			var pubKeyRef common.PubKey
			pubKeyRef, err = common.NewPubKey(w.pubKey)
			c.Assert(err, IsNil)
			c.Assert(nodePubKey.Equals(pubKeyRef), Equals, true)
			var nodePubKeyBytes []byte
			nodePubKeyBytes, err = nodePubKey.Bytes()
			c.Assert(err, IsNil)
			c.Assert(nodePubKeyBytes, NotNil)
			pubKeyHex = fmt.Sprintf("%x", pubKeyBytes)
			c.Assert(pubKeyHex, Equals, w.pubKeyHex)
			var nodeAddress common.Address
			nodeAddress, err = nodePubKey.GetAddress(common.ZECChain)
			c.Assert(err, IsNil)
			c.Assert(nodeAddress, NotNil)
		}

		utxos, err := s.client.client.GetAddressUTXOs(w.addr)
		c.Assert(err, IsNil)
		if len(utxos) == 0 {
			c.Fatalf("No utxos in Zecd node wallet (%s - %s)", s.zecdSnapshot, w.addr)
		}
	}
}

func (s *ZcashSuite) getTestSnapshotWallet(c *C, ndx int) (_addr string, _privKeyBytes, _pubKeyBytes []byte) {
	w := testSnapshotWallets[ndx]
	wif, err := btcutil.DecodeWIF(w.privKeyWIF)
	c.Assert(err, IsNil)
	privKeyBytes := wif.PrivKey.Serialize()
	pubKey := btcec.PublicKey(wif.PrivKey.PublicKey)
	pubKeyBytes := pubKey.SerializeCompressed()
	return w.addr, privKeyBytes, pubKeyBytes
}

func (s *ZcashSuite) updateClientKeys(c *C, privKeyBytes []byte) {
	var err error
	pkey, pubkey := btcec.PrivKeyFromBytes(btcec.S256(), privKeyBytes)
	// pkey, pubkey := btcec.PrivKeyFromBytes(privKeyBytes)
	s.client.ksWrapper, err = NewKeySignWrapper(pkey, s.client.ksWrapper.tssKeyManager)
	c.Assert(err, IsNil)
	// Convert btcec.PublicKey to Cosmos SDK secp256k1.PubKey
	cosmosPubKey := &secp256k1.PubKey{Key: pubkey.SerializeCompressed()}
	// Convert to Bech32 string
	bech32PubKey, err := cosmos.Bech32ifyPubKey(cosmos.Bech32PubKeyTypeAccPub, cosmosPubKey)
	c.Assert(err, IsNil)
	s.client.nodePubKey, err = common.NewPubKey(bech32PubKey)
	c.Assert(err, IsNil)
	s.client.nodeAddress, err = s.client.nodePubKey.GetAddress(common.ZECChain)
	c.Assert(err, IsNil)
}

func (s *ZcashSuite) selectWallet(c *C, i int) {
	_, privKeyBytes, _ := s.getTestSnapshotWallet(c, i)
	s.updateClientKeys(c, privKeyBytes)
}

func (s *ZcashSuite) setRandomClientKeys(c *C) {
	secret := make([]byte, 32)
	_, err := rand.Read(secret)
	c.Assert(err, IsNil)
	// Generate the private key using the secret
	privKeyHex := getPrivKeyHexFromSecret(secret)
	privKeyBytes, err := hex.DecodeString(privKeyHex)
	c.Assert(err, IsNil)
	s.updateClientKeys(c, privKeyBytes)
}

var m *metrics.Metrics

func GetMetricForTest(c *C) *metrics.Metrics {
	if m == nil {
		var err error
		m, err = metrics.NewMetrics(config.BifrostMetricsConfiguration{
			Enabled:      false,
			ListenPort:   9000,
			ReadTimeout:  time.Second / 10,
			WriteTimeout: time.Second / 10,
			Chains:       common.Chains{common.ZECChain},
		})
		c.Assert(m, NotNil)
		c.Assert(err, IsNil)
	}
	return m
}

func (s *ZcashSuite) TestZecToZats(c *C) {
	res, err := zec.ZecToUint(19.91065289999999877)
	c.Assert(err, IsNil)
	c.Assert(res.String(), Equals, "1991065290")
}

func (s *ZcashSuite) TestClientCreated(c *C) {
	c.Assert(s.client, NotNil)
	err := zec.ValidateAddress(s.client.nodeAddress.String(), s.client.getChainCfg().Net)
	c.Assert(err, IsNil)
}

func (s *ZcashSuite) TestGetBlock(c *C) {
	block, err := s.client.getBlock(182)
	c.Assert(err, IsNil)
	c.Assert(block.Hash, Equals, "07c7c32a978e9d697fc84b8a2fe8c90d0ab432761a043021fff3eec0851b665b")
	c.Assert(block.Tx, HasLen, 3)
	c.Assert(block.Tx[0].Txid, Equals, "f2e8e3a4163d93265210f03b6e64af4c33417d32af7af7421c8104c6a73dc609")
	c.Assert(block.Tx[1].Txid, Equals, "eaa37079406851ba2269a213f2e75c78950d1e8f44eb593f5ee1ea46f7f0cfd9")
}

func (s *ZcashSuite) TestFetchTxs(c *C) {
	var vaultPubKey common.PubKey
	var err error
	if common.CurrentChainNetwork == common.MainNet {
		vaultPubKey, err = common.NewPubKey("mayapub1addwnpepqw0anseu8gqs52equc5phn980d78p2c8q7t2pwl92eg4lflr92hmu9xl2za") // from PubKeys-Mainnet.json
	} else {
		vaultPubKey, err = common.NewPubKey("tmayapub1addwnpepqflvfv08t6qt95lmttd6wpf3ss8wx63e9vf6fvyuj2yy6nnyna5769e5dlr") // from PubKeys.json
	}
	c.Assert(err, IsNil, Commentf(vaultPubKey.String()))
	vaultAddress, err := vaultPubKey.GetAddress(s.client.GetChain())
	c.Assert(err, IsNil)
	vaultAddressString := vaultAddress.String()

	txs, err := s.client.FetchTxs(182, 182)
	c.Assert(err, IsNil)
	c.Assert(txs.Chain, Equals, common.ZECChain)
	c.Assert(txs.Count, Equals, "1")
	c.Assert(txs.TxArray, HasLen, 1)
	c.Assert(txs.TxArray[0].BlockHeight, Equals, int64(182))
	c.Assert(txs.TxArray[0].Tx, Equals, "eaa37079406851ba2269a213f2e75c78950d1e8f44eb593f5ee1ea46f7f0cfd9")
	c.Assert(txs.TxArray[0].Sender, Equals, "tmLqsSEy6cDjcTnRUd3ENKu1wWAckuiVUFx")
	c.Assert(txs.TxArray[0].To, Equals, vaultAddressString) // node
	c.Assert(txs.TxArray[0].Coins.Equals(common.Coins{common.NewCoin(common.ZECAsset, cosmos.NewUint(1500000000))}), Equals, true)
	c.Assert(txs.TxArray[0].Gas.Equals(common.Gas{common.NewCoin(common.ZECAsset, cosmos.NewUint(1000000))}), Equals, true)
}

func (s *ZcashSuite) TestGetSender(c *C) {
	tx := rpc.TxVerbose{
		Vin: []rpc.Vin{
			{
				Txid: "eaa37079406851ba2269a213f2e75c78950d1e8f44eb593f5ee1ea46f7f0cfd9",
				Vout: 0,
			},
		},
	}
	sender, err := s.client.getSender(&tx)
	c.Assert(err, IsNil)
	c.Assert(sender, Equals, testSnapshotWallets[0].addr) // "tmCtf4gJ8eb8mXh7hBk6DVW2byu41wPo1GN"

	tx.Vin[0].Vout = 1
	sender, err = s.client.getSender(&tx)
	c.Assert(err, IsNil)
	c.Assert(sender, Equals, testSnapshotWallets[1].addr) // "tmLqsSEy6cDjcTnRUd3ENKu1wWAckuiVUFx"
}

func (s *ZcashSuite) TestGetMemo(c *C) {
	tx := rpc.TxVerbose{
		Vout: []rpc.Vout{
			{
				ScriptPubKey: rpc.ScriptPubKey{
					Asm:       "OP_RETURN 74686f72636861696e3a636f6e736f6c6964617465",
					Hex:       "",
					ReqSigs:   0,
					Type:      "nulldata",
					Addresses: nil,
				},
			},
		},
	}
	memo, err := s.client.getMemo(&tx)
	c.Assert(err, IsNil)
	c.Assert(memo, Equals, "thorchain:consolidate")

	tx = rpc.TxVerbose{
		Vout: []rpc.Vout{
			{
				ScriptPubKey: rpc.ScriptPubKey{
					Asm:  "OP_RETURN 737761703a6574682e3078633534633135313236393646334541373935366264396144343130383138654563414443466666663a30786335346331353132363936463345413739353662643961443431",
					Type: "nulldata",
				},
			},
			{
				ScriptPubKey: rpc.ScriptPubKey{
					Asm:  "OP_RETURN 30383138654563414443466666663a3130303030303030303030",
					Type: "nulldata",
				},
			},
		},
	}
	memo, err = s.client.getMemo(&tx)
	c.Assert(err, IsNil)
	c.Assert(memo, Equals, "swap:eth.0xc54c1512696F3EA7956bd9aD410818eEcADCFfff:0xc54c1512696F3EA7956bd9aD410818eEcADCFfff:10000000000")

	tx = rpc.TxVerbose{
		Vout: []rpc.Vout{},
	}
	memo, err = s.client.getMemo(&tx)
	c.Assert(err, IsNil)
	c.Assert(memo, Equals, "")
}

func (s *ZcashSuite) TestIgnoreTx(c *C) {
	var currentHeight int64 = 100

	// tx with LockTime later than current height, so should be ignored
	tx := rpc.TxVerbose{
		Vin: []rpc.Vin{
			{
				Txid: "24ed2d26fd5d4e0e8fa86633e40faf1bdfc8d1903b1cd02855286312d48818a2",
				Vout: 0,
			},
		},
		Vout: []rpc.Vout{
			{
				Value: 0.12345678,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{"tb1qkq7weysjn6ljc2ywmjmwp8ttcckg8yyxjdz5k6"},
				},
			},
			{
				ScriptPubKey: rpc.ScriptPubKey{
					Asm:       "OP_RETURN 74686f72636861696e3a636f6e736f6c6964617465",
					Addresses: []string{"tb1qkq7weysjn6ljc2ywmjmwp8ttcckg8yyxjdz5k6"},
					Type:      "nulldata",
				},
			},
		},
		LockTime: uint32(currentHeight) + 1,
	}
	ignored, _ := s.client.ignoreTx(&tx, currentHeight)
	c.Assert(ignored, Equals, true)

	// tx with LockTime equal to current height, so should not be ignored
	tx = rpc.TxVerbose{
		Vin: []rpc.Vin{
			{
				Txid: "24ed2d26fd5d4e0e8fa86633e40faf1bdfc8d1903b1cd02855286312d48818a2",
				Vout: 0,
			},
		},
		Vout: []rpc.Vout{
			{
				Value: 0.12345678,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{"tb1qkq7weysjn6ljc2ywmjmwp8ttcckg8yyxjdz5k6"},
				},
			},
			{
				ScriptPubKey: rpc.ScriptPubKey{
					Asm:       "OP_RETURN 74686f72636861696e3a636f6e736f6c6964617465",
					Addresses: []string{"tb1qkq7weysjn6ljc2ywmjmwp8ttcckg8yyxjdz5k6"},
					Type:      "nulldata",
				},
			},
		},
		LockTime: uint32(currentHeight),
	}
	ignored, _ = s.client.ignoreTx(&tx, currentHeight)
	c.Assert(ignored, Equals, false)

	// valid tx that will NOT be ignored
	tx = rpc.TxVerbose{
		Vin: []rpc.Vin{
			{
				Txid: "24ed2d26fd5d4e0e8fa86633e40faf1bdfc8d1903b1cd02855286312d48818a2",
				Vout: 0,
			},
		},
		Vout: []rpc.Vout{
			{
				Value: 0.12345678,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{"tb1qkq7weysjn6ljc2ywmjmwp8ttcckg8yyxjdz5k6"},
				},
			},
			{
				ScriptPubKey: rpc.ScriptPubKey{
					Asm:       "OP_RETURN 74686f72636861696e3a636f6e736f6c6964617465",
					Addresses: []string{"tb1qkq7weysjn6ljc2ywmjmwp8ttcckg8yyxjdz5k6"},
					Type:      "nulldata",
				},
			},
		},
	}
	ignored, _ = s.client.ignoreTx(&tx, currentHeight)
	c.Assert(ignored, Equals, false)

	// invalid tx missing Vout
	tx = rpc.TxVerbose{
		Vin: []rpc.Vin{
			{
				Txid: "24ed2d26fd5d4e0e8fa86633e40faf1bdfc8d1903b1cd02855286312d48818a2",
				Vout: 0,
			},
		},
		Vout: []rpc.Vout{},
	}
	ignored, _ = s.client.ignoreTx(&tx, currentHeight)
	c.Assert(ignored, Equals, true)

	// invalid tx missing vout[0].Value == no coins
	tx = rpc.TxVerbose{
		Vin: []rpc.Vin{
			{
				Txid: "24ed2d26fd5d4e0e8fa86633e40faf1bdfc8d1903b1cd02855286312d48818a2",
				Vout: 0,
			},
		},
		Vout: []rpc.Vout{
			{
				Value: 0,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{"tb1qkq7weysjn6ljc2ywmjmwp8ttcckg8yyxjdz5k6"},
				},
			},
			{
				ScriptPubKey: rpc.ScriptPubKey{
					Asm:  "OP_RETURN 74686f72636861696e3a636f6e736f6c6964617465",
					Type: "nulldata",
				},
			},
		},
	}
	ignored, _ = s.client.ignoreTx(&tx, currentHeight)
	c.Assert(ignored, Equals, true)

	// invalid tx missing vin[0].Txid means coinbase
	tx = rpc.TxVerbose{
		Vin: []rpc.Vin{
			{
				Txid: "",
				Vout: 0,
			},
		},
		Vout: []rpc.Vout{
			{
				Value: 0.1234565,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{"tb1qkq7weysjn6ljc2ywmjmwp8ttcckg8yyxjdz5k6"},
				},
			},
			{
				ScriptPubKey: rpc.ScriptPubKey{
					Asm:  "OP_RETURN 74686f72636861696e3a636f6e736f6c6964617465",
					Type: "nulldata",
				},
			},
		},
	}
	ignored, _ = s.client.ignoreTx(&tx, currentHeight)
	c.Assert(ignored, Equals, true)

	// invalid tx missing vin
	tx = rpc.TxVerbose{
		Vin: []rpc.Vin{},
		Vout: []rpc.Vout{
			{
				Value: 0.1234565,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{"tb1qkq7weysjn6ljc2ywmjmwp8ttcckg8yyxjdz5k6"},
				},
			},
			{
				ScriptPubKey: rpc.ScriptPubKey{
					Asm:  "OP_RETURN 74686f72636861696e3a636f6e736f6c6964617465",
					Type: "nulldata",
				},
			},
		},
	}
	ignored, _ = s.client.ignoreTx(&tx, currentHeight)
	c.Assert(ignored, Equals, true)

	// invalid tx multiple vout[0].Addresses
	tx = rpc.TxVerbose{
		Vin: []rpc.Vin{
			{
				Txid: "24ed2d26fd5d4e0e8fa86633e40faf1bdfc8d1903b1cd02855286312d48818a2",
				Vout: 0,
			},
		},
		Vout: []rpc.Vout{
			{
				Value: 0.1234565,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{
						"tb1qkq7weysjn6ljc2ywmjmwp8ttcckg8yyxjdz5k6",
						"bc1q0s4mg25tu6termrk8egltfyme4q7sg3h0e56p3",
					},
				},
			},
			{
				ScriptPubKey: rpc.ScriptPubKey{
					Asm:  "OP_RETURN 74686f72636861696e3a636f6e736f6c6964617465",
					Type: "nulldata",
				},
			},
		},
	}
	ignored, _ = s.client.ignoreTx(&tx, currentHeight)
	c.Assert(ignored, Equals, true)

	// invalid tx > 2 vout with coins we only expect 2 max
	tx = rpc.TxVerbose{
		Vin: []rpc.Vin{
			{
				Txid: "24ed2d26fd5d4e0e8fa86633e40faf1bdfc8d1903b1cd02855286312d48818a2",
				Vout: 0,
			},
		},
		Vout: []rpc.Vout{
			{
				Value: 0.1234565,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{
						"bc1q0s4mg25tu6termrk8egltfyme4q7sg3h0e56p3",
					},
				},
			},
			{
				Value: 0.1234565,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{
						"tb1qkq7weysjn6ljc2ywmjmwp8ttcckg8yyxjdz5k6",
					},
				},
			},
			{
				Value: 0.1234565,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{
						"tb1qkq7weysjn6ljc2ywmjmwp8ttcckg8yyxjdz5k6",
					},
				},
			},
			{
				ScriptPubKey: rpc.ScriptPubKey{
					Asm:  "OP_RETURN 74686f72636861696e3a636f6e736f6c6964617465",
					Type: "nulldata",
				},
			},
		},
	}
	ignored, _ = s.client.ignoreTx(&tx, currentHeight)
	c.Assert(ignored, Equals, true)

	// valid tx == 2 vout with coins, 1 to vault, 1 with change back to user
	tx = rpc.TxVerbose{
		Vin: []rpc.Vin{
			{
				Txid: "24ed2d26fd5d4e0e8fa86633e40faf1bdfc8d1903b1cd02855286312d48818a2",
				Vout: 0,
			},
		},
		Vout: []rpc.Vout{
			{
				Value: 0.1234565,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{
						"bc1q0s4mg25tu6termrk8egltfyme4q7sg3h0e56p3",
					},
				},
			},
			{
				Value: 0.1234565,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{
						"tb1qkq7weysjn6ljc2ywmjmwp8ttcckg8yyxjdz5k6",
					},
				},
			},
			{
				ScriptPubKey: rpc.ScriptPubKey{
					Asm:  "OP_RETURN 74686f72636861696e3a636f6e736f6c6964617465",
					Type: "nulldata",
				},
			},
		},
	}
	ignored, _ = s.client.ignoreTx(&tx, currentHeight)
	c.Assert(ignored, Equals, false)

	// memo at first output should not ignore
	tx = rpc.TxVerbose{
		Vin: []rpc.Vin{
			{
				Txid: "24ed2d26fd5d4e0e8fa86633e40faf1bdfc8d1903b1cd02855286312d48818a2",
				Vout: 0,
			},
		},
		Vout: []rpc.Vout{
			{
				ScriptPubKey: rpc.ScriptPubKey{
					Asm:  "OP_RETURN 74686f72636861696e3a636f6e736f6c6964617465",
					Type: "nulldata",
				},
			},
			{
				Value: 0.1234565,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{
						"bc1q0s4mg25tu6termrk8egltfyme4q7sg3h0e56p3",
					},
				},
			},
			{
				Value: 0.1234565,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{
						"tb1qkq7weysjn6ljc2ywmjmwp8ttcckg8yyxjdz5k6",
					},
				},
			},
		},
	}
	ignored, _ = s.client.ignoreTx(&tx, currentHeight)
	c.Assert(ignored, Equals, false)

	// memo in the middle , should not ignore
	tx = rpc.TxVerbose{
		Vin: []rpc.Vin{
			{
				Txid: "24ed2d26fd5d4e0e8fa86633e40faf1bdfc8d1903b1cd02855286312d48818a2",
				Vout: 0,
			},
		},
		Vout: []rpc.Vout{
			{
				Value: 0.1234565,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{
						"bc1q0s4mg25tu6termrk8egltfyme4q7sg3h0e56p3",
					},
				},
			},
			{
				ScriptPubKey: rpc.ScriptPubKey{
					Asm:  "OP_RETURN 74686f72636861696e3a636f6e736f6c6964617465",
					Type: "nulldata",
				},
			},
			{
				Value: 0.1234565,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{
						"tb1qkq7weysjn6ljc2ywmjmwp8ttcckg8yyxjdz5k6",
					},
				},
			},
		},
	}
	ignored, _ = s.client.ignoreTx(&tx, currentHeight)
	c.Assert(ignored, Equals, false)
}

func (s *ZcashSuite) TestGetGas(c *C) {
	// vin[0] returns value 0.05020000
	tx := rpc.TxVerbose{
		Vin: []rpc.Vin{
			{
				Txid: "eaa37079406851ba2269a213f2e75c78950d1e8f44eb593f5ee1ea46f7f0cfd9",
				Vout: 0,
			},
		},
		Vout: []rpc.Vout{
			{
				Value: 0.01234567,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{"XuhjXrjKBJumufDkPFP9heunnMrgXoHoqx"},
				},
			},
			{
				ScriptPubKey: rpc.ScriptPubKey{
					Asm: "OP_RETURN 74686f72636861696e3a636f6e736f6c6964617465",
				},
			},
		},
	}
	gas, err := s.client.getGas(&tx)
	c.Assert(err, IsNil)
	c.Assert(gas.Equals(common.Gas{common.NewCoin(common.ZECAsset, cosmos.NewUint(1498765433))}), Equals, true)

	tx = rpc.TxVerbose{
		Vin: []rpc.Vin{
			{
				Txid: "eaa37079406851ba2269a213f2e75c78950d1e8f44eb593f5ee1ea46f7f0cfd9",
				Vout: 0,
			},
		},
		Vout: []rpc.Vout{
			{
				Value: 0.00195384,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{"tb1qkq7weysjn6ljc2ywmjmwp8ttcckg8yyxjdz5k6"},
				},
			},
			{
				Value: 0.00496556,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{"tb1qkq7weysjn6ljc2ywmjmwp8ttcckg8yyxjdz5k6"},
				},
			},
			{
				ScriptPubKey: rpc.ScriptPubKey{
					Asm: "OP_RETURN 74686f72636861696e3a636f6e736f6c6964617465",
				},
			},
		},
	}
	gas, err = s.client.getGas(&tx)
	c.Assert(err, IsNil)
	c.Assert(gas.Equals(common.Gas{common.NewCoin(common.ZECAsset, cosmos.NewUint(1499308060))}), Equals, true)
}

func (s *ZcashSuite) TestGetChain(c *C) {
	chain := s.client.GetChain()
	c.Assert(chain, Equals, common.ZECChain)
}

func (s *ZcashSuite) TestOnObservedTxIn(c *C) {
	pkey := types2.GetRandomPubKey()
	txIn := types.TxIn{
		Count: "1",
		Chain: common.ZECChain,
		TxArray: []types.TxInItem{
			{
				BlockHeight: 1,
				Tx:          "31f8699ce9028e9cd37f8a6d58a79e614a96e3fdd0f58be5fc36d2d95484716f",
				Sender:      "bc1q2gjc0rnhy4nrxvuklk6ptwkcs9kcr59mcl2q9j",
				To:          "bc1q0s4mg25tu6termrk8egltfyme4q7sg3h0e56p3",
				Coins: common.Coins{
					common.NewCoin(common.ZECAsset, cosmos.NewUint(123456789)),
				},
				Memo:                "MEMO",
				ObservedVaultPubKey: pkey,
			},
		},
	}
	blockMeta := utxo.NewBlockMeta("000000001ab8a8484eb89f04b87d90eb88e2cbb2829e84eb36b966dcb28af90b", 1, "00000000ffa57c95f4f226f751114e9b24fdf8dbe2dbc02a860da9320bebd63e")
	c.Assert(s.client.temporalStorage.SaveBlockMeta(blockMeta.Height, blockMeta), IsNil)
	s.client.OnObservedTxIn(txIn.TxArray[0], 1)
	blockMeta, err := s.client.temporalStorage.GetBlockMeta(1)
	c.Assert(err, IsNil)
	c.Assert(blockMeta, NotNil)

	txIn = types.TxIn{
		Count: "1",
		Chain: common.ZECChain,
		TxArray: []types.TxInItem{
			{
				BlockHeight: 2,
				Tx:          "24ed2d26fd5d4e0e8fa86633e40faf1bdfc8d1903b1cd02855286312d48818a2",
				Sender:      "bc1q0s4mg25tu6termrk8egltfyme4q7sg3h0e56p3",
				To:          "bc1q2gjc0rnhy4nrxvuklk6ptwkcs9kcr59mcl2q9j",
				Coins: common.Coins{
					common.NewCoin(common.ZECAsset, cosmos.NewUint(123456)),
				},
				Memo:                "MEMO",
				ObservedVaultPubKey: pkey,
			},
		},
	}
	blockMeta = utxo.NewBlockMeta("000000001ab8a8484eb89f04b87d90eb88e2cbb2829e84eb36b966dcb28af90b", 2, "00000000ffa57c95f4f226f751114e9b24fdf8dbe2dbc02a860da9320bebd63e")
	c.Assert(s.client.temporalStorage.SaveBlockMeta(blockMeta.Height, blockMeta), IsNil)
	s.client.OnObservedTxIn(txIn.TxArray[0], 2)
	blockMeta, err = s.client.temporalStorage.GetBlockMeta(2)
	c.Assert(err, IsNil)
	c.Assert(blockMeta, NotNil)

	txIn = types.TxIn{
		Count: "2",
		Chain: common.ZECChain,
		TxArray: []types.TxInItem{
			{
				BlockHeight: 3,
				Tx:          "44ed2d26fd5d4e0e8fa86633e40faf1bdfc8d1903b1cd02855286312d48818a2",
				Sender:      "bc1q0s4mg25tu6termrk8egltfyme4q7sg3h0e56p3",
				To:          "bc1q2gjc0rnhy4nrxvuklk6ptwkcs9kcr59mcl2q9j",
				Coins: common.Coins{
					common.NewCoin(common.ZECAsset, cosmos.NewUint(12345678)),
				},
				Memo:                "MEMO",
				ObservedVaultPubKey: pkey,
			},
			{
				BlockHeight: 3,
				Tx:          "54ed2d26fd5d4e0e8fa86633e40faf1bdfc8d1903b1cd02855286312d48818a2",
				Sender:      "bc1q0s4mg25tu6termrk8egltfyme4q7sg3h0e56p3",
				To:          "bc1q2gjc0rnhy4nrxvuklk6ptwkcs9kcr59mcl2q9j",
				Coins: common.Coins{
					common.NewCoin(common.ZECAsset, cosmos.NewUint(123456)),
				},
				Memo:                "MEMO",
				ObservedVaultPubKey: pkey,
			},
		},
	}
	blockMeta = utxo.NewBlockMeta("000000001ab8a8484eb89f04b87d90eb88e2cbb2829e84eb36b966dcb28af90b", 3, "00000000ffa57c95f4f226f751114e9b24fdf8dbe2dbc02a860da9320bebd63e")
	c.Assert(s.client.temporalStorage.SaveBlockMeta(blockMeta.Height, blockMeta), IsNil)
	for _, item := range txIn.TxArray {
		s.client.OnObservedTxIn(item, 3)
	}

	blockMeta, err = s.client.temporalStorage.GetBlockMeta(3)
	c.Assert(err, IsNil)
	c.Assert(blockMeta, NotNil)
}

func (s *ZcashSuite) TestGetOutput(c *C) {
	var vaultPubKey common.PubKey
	var err error
	if common.CurrentChainNetwork == common.MainNet {
		vaultPubKey, err = common.NewPubKey("mayapub1addwnpepqw0anseu8gqs52equc5phn980d78p2c8q7t2pwl92eg4lflr92hmu9xl2za") // from PubKeys-Mainnet.json
	} else {
		vaultPubKey, err = common.NewPubKey("tmayapub1addwnpepqflvfv08t6qt95lmttd6wpf3ss8wx63e9vf6fvyuj2yy6nnyna5769e5dlr") // from PubKeys.json
	}
	c.Assert(err, IsNil, Commentf(vaultPubKey.String()))
	vaultAddress, err := vaultPubKey.GetAddress(s.client.GetChain())
	c.Assert(err, IsNil)
	vaultAddressString := vaultAddress.String()

	tx := rpc.TxVerbose{
		Vin: []rpc.Vin{
			{
				Txid: "5b0876dcc027d2f0c671fc250460ee388df39697c3ff082007b6ddd9cb9a7513",
				Vout: 1,
			},
		},
		Vout: []rpc.Vout{
			{
				Value: 0.00195384,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{vaultAddressString},
				},
			},
			{
				Value: 1.49655603,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{"tb1qj08ys4ct2hzzc2hcz6h2hgrvlmsjynaw43s835"},
				},
			},
			{
				ScriptPubKey: rpc.ScriptPubKey{
					Asm:  "OP_RETURN 74686f72636861696e3a636f6e736f6c6964617465",
					Type: "nulldata",
				},
			},
		},
	}
	out, err := s.client.getOutput(vaultAddressString, &tx, false)
	c.Assert(err, IsNil)
	c.Assert(out.ScriptPubKey.Addresses[0], Equals, "tb1qj08ys4ct2hzzc2hcz6h2hgrvlmsjynaw43s835")
	c.Assert(out.Value, Equals, 1.49655603)

	tx = rpc.TxVerbose{
		Vin: []rpc.Vin{
			{
				Txid: "5b0876dcc027d2f0c671fc250460ee388df39697c3ff082007b6ddd9cb9a7513",
				Vout: 1,
			},
		},
		Vout: []rpc.Vout{
			{
				ScriptPubKey: rpc.ScriptPubKey{
					Asm:  "OP_RETURN 74686f72636861696e3a636f6e736f6c6964617465",
					Type: "nulldata",
				},
			},
			{
				Value: 0.00195384,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{vaultAddressString},
				},
			},
			{
				Value: 1.49655603,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{"tb1qj08ys4ct2hzzc2hcz6h2hgrvlmsjynaw43s835"},
				},
			},
		},
	}
	out, err = s.client.getOutput(vaultAddressString, &tx, false)
	c.Assert(err, IsNil)
	c.Assert(out.ScriptPubKey.Addresses[0], Equals, "tb1qj08ys4ct2hzzc2hcz6h2hgrvlmsjynaw43s835")
	c.Assert(out.Value, Equals, 1.49655603)

	tx = rpc.TxVerbose{
		Vin: []rpc.Vin{
			{
				Txid: "5b0876dcc027d2f0c671fc250460ee388df39697c3ff082007b6ddd9cb9a7513",
				Vout: 1,
			},
		},
		Vout: []rpc.Vout{
			{
				Value: 0.00195384,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{vaultAddressString},
				},
			},
			{
				ScriptPubKey: rpc.ScriptPubKey{
					Asm:  "OP_RETURN 74686f72636861696e3a636f6e736f6c6964617465",
					Type: "nulldata",
				},
			},
			{
				Value: 1.49655603,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{"tb1qj08ys4ct2hzzc2hcz6h2hgrvlmsjynaw43s835"},
				},
			},
		},
	}
	out, err = s.client.getOutput(vaultAddressString, &tx, false)
	c.Assert(err, IsNil)
	c.Assert(out.ScriptPubKey.Addresses[0], Equals, "tb1qj08ys4ct2hzzc2hcz6h2hgrvlmsjynaw43s835")
	c.Assert(out.Value, Equals, 1.49655603)

	tx = rpc.TxVerbose{
		Vin: []rpc.Vin{
			{
				Txid: "5b0876dcc027d2f0c671fc250460ee388df39697c3ff082007b6ddd9cb9a7513",
				Vout: 1,
			},
		},
		Vout: []rpc.Vout{
			{
				Value: 1.49655603,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{"tb1qj08ys4ct2hzzc2hcz6h2hgrvlmsjynaw43s835"},
				},
			},
			{
				Value: 0.00195384,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{vaultAddressString},
				},
			},
			{
				ScriptPubKey: rpc.ScriptPubKey{
					Asm:  "OP_RETURN 74686f72636861696e3a636f6e736f6c6964617465",
					Type: "nulldata",
				},
			},
		},
	}
	out, err = s.client.getOutput(vaultAddressString, &tx, false)
	c.Assert(err, IsNil)
	c.Assert(out.ScriptPubKey.Addresses[0], Equals, "tb1qj08ys4ct2hzzc2hcz6h2hgrvlmsjynaw43s835")
	c.Assert(out.Value, Equals, 1.49655603)

	tx = rpc.TxVerbose{
		Vin: []rpc.Vin{
			{
				Txid: "5b0876dcc027d2f0c671fc250460ee388df39697c3ff082007b6ddd9cb9a7513",
				Vout: 1,
			},
		},
		Vout: []rpc.Vout{
			{
				Value: 1.49655603,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{vaultAddressString},
				},
			},
			{
				Value: 0.00195384,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{vaultAddressString},
				},
			},
			{
				ScriptPubKey: rpc.ScriptPubKey{
					Asm:  "OP_RETURN 74686f72636861696e3a636f6e736f6c6964617465",
					Type: "nulldata",
				},
			},
		},
	}
	out, err = s.client.getOutput(vaultAddressString, &tx, true)
	c.Assert(err, IsNil)
	c.Assert(out.ScriptPubKey.Addresses[0], Equals, vaultAddressString)
	c.Assert(out.Value, Equals, 1.49655603)

	tx = rpc.TxVerbose{
		Vin: []rpc.Vin{
			{
				Txid: "5b0876dcc027d2f0c671fc250460ee388df39697c3ff082007b6ddd9cb9a7513",
				Vout: 1,
			},
		},
		Vout: []rpc.Vout{
			{
				ScriptPubKey: rpc.ScriptPubKey{
					Asm:  "OP_RETURN 74686f72636861696e3a636f6e736f6c6964617465",
					Type: "nulldata",
				},
			},
			{
				Value: 0.00195384,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{"tb1qkq7weysjn6ljc2ywmjmwp8ttcckg8yyxjdz5k6"},
				},
			},
			{
				Value: 1.49655603,
				ScriptPubKey: rpc.ScriptPubKey{
					Addresses: []string{vaultAddressString},
				},
			},
		},
	}
	out, err = s.client.getOutput("tb1qj08ys4ct2hzzc2hcz6h2hgrvlmsjynaw43s835", &tx, false)
	c.Assert(err, IsNil, Commentf(vaultAddressString))
	c.Assert(out.ScriptPubKey.Addresses[0], Equals, vaultAddressString)
	c.Assert(out.Value, Equals, 1.49655603)
}
