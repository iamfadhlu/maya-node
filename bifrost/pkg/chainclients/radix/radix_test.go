package radix

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	cKeys "github.com/cosmos/cosmos-sdk/crypto/keyring"
	ctypes "gitlab.com/mayachain/binance-sdk/common/types"
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/bifrost/mayaclient"
	stypes "gitlab.com/mayachain/mayanode/bifrost/mayaclient/types"
	"gitlab.com/mayachain/mayanode/bifrost/metrics"
	"gitlab.com/mayachain/mayanode/bifrost/pubkeymanager"
	"gitlab.com/mayachain/mayanode/cmd"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/config"
	openapi "gitlab.com/mayachain/mayanode/openapi/gen"
	ttypes "gitlab.com/mayachain/mayanode/x/mayachain/types"
)

const (
	bob      = "bob"
	password = "password"
)

func TestPackage(t *testing.T) { TestingT(t) }

type radixTestFixtures struct {
	transactionPreview string
	networkStatus      string
}

type RadixTestSuite struct {
	client   *Client
	server   *httptest.Server
	bridge   mayaclient.MayachainBridge
	cfg      config.BifrostChainConfiguration
	m        *metrics.Metrics
	keys     *mayaclient.Keys
	fixtures struct {
		transactionPreview string
		networkStatus      string
	}
}

var _ = Suite(&RadixTestSuite{})

var m *metrics.Metrics

func GetMetricForTest(c *C) *metrics.Metrics {
	if m == nil {
		var err error
		m, err = metrics.NewMetrics(config.BifrostMetricsConfiguration{
			Enabled:      false,
			ListenPort:   9000,
			ReadTimeout:  time.Second,
			WriteTimeout: time.Second,
			Chains:       common.Chains{common.XRDChain},
		})
		c.Assert(m, NotNil)
		c.Assert(err, IsNil)
	}
	return m
}

func (s *RadixTestSuite) SetUpSuite(c *C) {
	ttypes.SetupConfigForTest()
	kb := cKeys.NewInMemory()
	_, _, err := kb.NewMnemonic(bob, cKeys.English, cmd.BASEChainHDPath, password, hd.Secp256k1)
	c.Assert(err, IsNil)
	s.keys = mayaclient.NewKeysWithKeybase(kb, bob, password)
}

func (s *RadixTestSuite) SetUpTest(c *C) {
	s.m = GetMetricForTest(c)
	s.cfg = config.BifrostChainConfiguration{
		ChainID:     "XRD",
		UserName:    bob,
		Password:    password,
		DisableTLS:  true,
		HTTPostMode: true,
		BlockScanner: config.BifrostBlockScannerConfiguration{
			StartBlockHeight:   1,
			HTTPRequestTimeout: 30 * time.Second,
		},
	}

	ns := strconv.Itoa(time.Now().Nanosecond())
	ctypes.Network = ctypes.TestNetwork

	thordir := filepath.Join(os.TempDir(), ns, ".thorcli")
	cfg := config.BifrostClientConfiguration{
		ChainID:         "mayachain",
		ChainHost:       "http://localhost",
		SignerName:      bob,
		SignerPasswd:    password,
		ChainHomeFolder: thordir,
	}

	s.server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch req.RequestURI {
		case "/":
			r := struct {
				Method string   `json:"method"`
				Params []string `json:"params"`
			}{}
			_ = json.NewDecoder(req.Body).Decode(&r)

			if r.Method == "createwallet" {
				_, err := rw.Write([]byte(`{ "result": null, "error": null, "id": 1 }`))
				c.Assert(err, IsNil)
			}
		case mayaclient.PubKeysEndpoint:
			priKey, _ := s.keys.GetPrivateKey()
			tm, _ := codec.ToTmPubKeyInterface(priKey.PubKey())
			pk, err := common.NewPubKeyFromCrypto(tm)
			c.Assert(err, IsNil)
			content, err := os.ReadFile("../../../../test/fixtures/endpoints/vaults/pubKeys.json")
			c.Assert(err, IsNil)
			var pubKeysVault openapi.VaultPubkeysResponse
			c.Assert(json.Unmarshal(content, &pubKeysVault), IsNil)
			chain := common.XRDChain.String()
			mainRouter := "component_rdx1cp7hrk7k0pjavnpt5h6dsel096kzlj96r8ukw2ywqgdc5tlvpvn0as"

			pubKeysVault.Asgard = append(pubKeysVault.Asgard, openapi.VaultInfo{
				PubKey: pk.String(),
				Routers: []openapi.VaultRouter{
					{
						Chain:  &chain,
						Router: &mainRouter,
					},
				},
			})

			buf, err := json.MarshalIndent(pubKeysVault, "", "	")
			c.Assert(err, IsNil)
			_, err = rw.Write(buf)
			c.Assert(err, IsNil)
		case mayaclient.AsgardVault:
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/vaults/asgard.json")
		case "/status/network-configuration":
			httpTestHandler(c, rw, "../../../../test/fixtures/xrd/network_configuration.json")
		case "/status/network-status":
			httpTestHandler(c, rw, s.fixtures.networkStatus)
		case "/transaction/preview":
			httpTestHandler(c, rw, s.fixtures.transactionPreview)
		default:
			panic("No handler found for " + req.RequestURI)
		}
	}))

	var err error
	cfg.ChainHost = s.server.Listener.Addr().String()
	s.bridge, err = mayaclient.NewMayachainBridge(cfg, s.m, s.keys)
	c.Assert(err, IsNil)
	s.cfg.RPCHost = "http://" + s.server.Listener.Addr().String()

	var pubkeyMgr *pubkeymanager.PubKeyManager
	pubkeyMgr, err = pubkeymanager.NewPubKeyManager(s.bridge, s.m)
	if err != nil {
		log.Fatal().Err(err).Msg("fail to create pubkey manager")
	}
	if err = pubkeyMgr.Start(); err != nil {
		log.Fatal().Err(err).Msg("fail to start pubkey manager")
	}
	s.client, err = NewClient(s.keys, s.cfg, nil, s.bridge, s.m, pubkeyMgr)
	c.Assert(err, IsNil)
	c.Assert(s.client, NotNil)
}

func (s *RadixTestSuite) TearDownTest(_ *C) {
	s.server.Close()
}

func httpTestHandler(c *C, rw http.ResponseWriter, fixture string) {
	content, err := os.ReadFile(fixture)
	if err != nil {
		c.Fatal(err)
	}
	rw.Header().Set("Content-Type", "application/json")
	if _, err := rw.Write(content); err != nil {
		c.Fatal(err)
	}
}

func (s *RadixTestSuite) TestNewClient(c *C) {
	pubkeyMgr, err := pubkeymanager.NewPubKeyManager(s.bridge, s.m)
	c.Assert(err, IsNil)

	tests := []struct {
		name        string
		keys        *mayaclient.Keys
		cfg         config.BifrostChainConfiguration
		bridge      mayaclient.MayachainBridge
		pubkeyMgr   *pubkeymanager.PubKeyManager
		expectError bool
	}{
		{
			name:        "missing bridge",
			keys:        s.keys,
			bridge:      nil,
			pubkeyMgr:   pubkeyMgr,
			expectError: true,
		},
		{
			name:        "missing pubkey manager",
			keys:        s.keys,
			bridge:      s.bridge,
			pubkeyMgr:   nil,
			expectError: true,
		},
		{
			name:        "missing keys",
			keys:        nil,
			bridge:      s.bridge,
			pubkeyMgr:   pubkeyMgr,
			expectError: true,
		},
	}

	for _, tc := range tests {
		c.Log(tc.name)
		client, err := NewClient(tc.keys, tc.cfg, nil, tc.bridge, s.m, tc.pubkeyMgr)
		if tc.expectError {
			c.Assert(err, NotNil)
			c.Assert(client, IsNil)
		} else {
			c.Assert(err, IsNil)
			c.Assert(client, NotNil)
		}
	}
}

func (s *RadixTestSuite) TestSignTx(c *C) {
	s.fixtures = radixTestFixtures{
		transactionPreview: "../../../../test/fixtures/xrd/transaction_preview.json",
		networkStatus:      "../../../../test/fixtures/xrd/network_status.json",
	}

	pubkeyMgr, err := pubkeymanager.NewPubKeyManager(s.bridge, s.m)
	c.Assert(err, IsNil)
	e, err := NewClient(s.keys, config.BifrostChainConfiguration{
		RPCHost: "http://" + s.server.Listener.Addr().String(),
		BlockScanner: config.BifrostBlockScannerConfiguration{
			StartBlockHeight:   1, // avoids querying thorchain for block height
			HTTPRequestTimeout: time.Second,
			MaxGasLimit:        80000,
		},
		AggregatorMaxGasMultiplier: 10,
		TokenMaxGasMultiplier:      3,
	}, nil, s.bridge, s.m, pubkeyMgr)
	c.Assert(err, IsNil)
	c.Assert(e, NotNil)
	c.Assert(pubkeyMgr.Start(), IsNil)
	defer func() { c.Assert(pubkeyMgr.Stop(), IsNil) }()
	pubkeys := pubkeyMgr.GetPubKeys()
	addr, err := pubkeys[len(pubkeys)-1].GetAddress(common.XRDChain)
	c.Assert(err, IsNil)

	// Test cases
	// Test 1: Empty to address
	signed1, signed2, obs, err := s.client.SignTx(stypes.TxOutItem{
		Chain:       common.XRDChain,
		ToAddress:   "",
		VaultPubKey: s.client.localPubKey,
		Coins: common.Coins{
			common.NewCoin(common.XRDAsset, cosmos.NewUint(1e18)),
		},
		MaxGas: common.Gas{
			common.NewCoin(common.XRDAsset, cosmos.NewUint(1000)),
		},
		Memo: "OUT:ABCD",
	}, 1)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "to address is empty")
	c.Assert(signed1, IsNil)
	c.Assert(signed2, IsNil)
	c.Assert(obs, IsNil)

	// Test 2: Empty vault pubkey
	signed1, signed2, obs, err = s.client.SignTx(stypes.TxOutItem{
		Chain:       common.XRDChain,
		ToAddress:   addr,
		VaultPubKey: "",
		Coins: common.Coins{
			common.NewCoin(common.XRDAsset, cosmos.NewUint(1e18)),
		},
		MaxGas: common.Gas{
			common.NewCoin(common.XRDAsset, cosmos.NewUint(1000)),
		},
		Memo: "OUT:ABCD",
	}, 1)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "vault public key is empty")
	c.Assert(signed1, IsNil)
	c.Assert(signed2, IsNil)
	c.Assert(obs, IsNil)

	// Test 3: Empty memo
	signed1, signed2, obs, err = s.client.SignTx(stypes.TxOutItem{
		Chain:       common.XRDChain,
		ToAddress:   addr,
		VaultPubKey: s.client.localPubKey,
		Coins: common.Coins{
			common.NewCoin(common.XRDAsset, cosmos.NewUint(1e18)),
		},
		MaxGas: common.Gas{
			common.NewCoin(common.XRDAsset, cosmos.NewUint(1000)),
		},
		Memo: "",
	}, 1)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "can't sign tx when it doesn't have memo")
	c.Assert(signed1, IsNil)
	c.Assert(signed2, IsNil)
	c.Assert(obs, IsNil)

	// Test 4: Valid outbound transaction
	signed1, signed2, obs, err = s.client.SignTx(stypes.TxOutItem{
		Chain:       common.XRDChain,
		ToAddress:   addr,
		VaultPubKey: s.client.localPubKey,
		Coins: common.Coins{
			common.NewCoin(common.XRDAsset, cosmos.NewUint(1e18)),
		},
		MaxGas: common.Gas{
			common.NewCoin(common.XRDAsset, cosmos.NewUint(1000)),
		},
		GasRate: 1,
		Memo:    "OUT:4D91ADAFA69765E7805B5FF2F3A0BA1DBE69E37A1CFCD20C48B99C528AA3EE87",
	}, 1)
	c.Assert(err, IsNil)
	c.Assert(signed1, NotNil)
	c.Assert(signed2, IsNil)
	c.Assert(obs, IsNil)

	// Test 5: Valid refund transaction
	signed1, signed2, obs, err = s.client.SignTx(stypes.TxOutItem{
		Chain:       common.XRDChain,
		ToAddress:   addr,
		VaultPubKey: s.client.localPubKey,
		Coins: common.Coins{
			common.NewCoin(common.XRDAsset, cosmos.NewUint(1e18)),
		},
		MaxGas: common.Gas{
			common.NewCoin(common.XRDAsset, cosmos.NewUint(1000)),
		},
		GasRate: 1,
		Memo:    "REFUND:4D91ADAFA69765E7805B5FF2F3A0BA1DBE69E37A1CFCD20C48B99C528AA3EE87",
	}, 1)
	c.Assert(err, IsNil)
	c.Assert(signed1, NotNil)
	c.Assert(signed2, IsNil)
	c.Assert(obs, IsNil)

	// Test 6: Valid migrate transaction
	signed1, signed2, obs, err = s.client.SignTx(stypes.TxOutItem{
		Chain:       common.XRDChain,
		ToAddress:   addr,
		VaultPubKey: s.client.localPubKey,
		Coins: common.Coins{
			common.NewCoin(common.XRDAsset, cosmos.NewUint(1e18)),
		},
		MaxGas: common.Gas{
			common.NewCoin(common.XRDAsset, cosmos.NewUint(1000)),
		},
		GasRate: 1,
		Memo:    "MIGRATE:1024",
	}, 1)
	c.Assert(err, IsNil)
	c.Assert(signed1, NotNil)
	c.Assert(signed2, IsNil)
	c.Assert(obs, IsNil)
}

func (s *RadixTestSuite) TestGetAccount(c *C) {
	s.fixtures = radixTestFixtures{
		transactionPreview: "../../../../test/fixtures/xrd/transaction_preview.json",
		networkStatus:      "../../../../test/fixtures/xrd/network_status.json",
	}
	pubkey := ttypes.GetRandomPubKey()
	acct, err := s.client.GetAccount(pubkey, big.NewInt(0))
	c.Assert(err, IsNil)
	c.Assert(acct.Coins, HasLen, 1)
	c.Assert(acct.Coins[0].Amount.BigInt().Cmp(big.NewInt(102019290166504)) == 0, Equals, true)
}

func (s *RadixTestSuite) TestGetAccountInsufficientFee(c *C) {
	s.fixtures = radixTestFixtures{
		transactionPreview: "../../../../test/fixtures/xrd/transaction_preview_insufficient_fee.json",
		networkStatus:      "../../../../test/fixtures/xrd/network_status_insufficient_fee.json",
	}
	pubkey := ttypes.GetRandomPubKey()
	acct, err := s.client.GetAccount(pubkey, big.NewInt(0))
	c.Assert(err, IsNil)
	c.Assert(acct.Coins, HasLen, 0)
}

type mayaMockBridge struct {
	mayaclient.MayachainBridge
	getAsgardsFunc func() (ttypes.Vaults, error)
}

func (m *mayaMockBridge) GetAsgards() (ttypes.Vaults, error) {
	return m.getAsgardsFunc()
}

func (s *RadixTestSuite) TestShouldReportSolvency(c *C) {
	testCases := []struct {
		name                    string
		height                  int64
		lastSolvencyCheckHeight int64
		shouldReport            bool
	}{
		{
			name:                    "should report when difference > 1000",
			height:                  2000,
			lastSolvencyCheckHeight: 900,
			shouldReport:            true,
		},
		{
			name:                    "should not report when difference <= 1000",
			height:                  1900,
			lastSolvencyCheckHeight: 900,
			shouldReport:            false,
		},
		{
			name:                    "should report when height is much higher",
			height:                  5000,
			lastSolvencyCheckHeight: 900,
			shouldReport:            true,
		},
	}

	for _, tc := range testCases {
		s.client.lastSolvencyCheckHeight = tc.lastSolvencyCheckHeight
		result := s.client.ShouldReportSolvency(tc.height)
		c.Assert(tc.shouldReport, Equals, result)
	}
}

func (s *RadixTestSuite) TestReportSolvency(c *C) {
	asgards, err := s.client.mayaBridge.GetAsgards()
	c.Assert(err, IsNil)

	testCases := []struct {
		name                    string
		height                  int64
		lastSolvencyCheckHeight int64
		mockSetup               func(*mayaMockBridge)
		expectedError           string
		expectSolvencyReport    bool
	}{
		{
			name:                    "skip report when height difference is too small",
			height:                  1500,
			lastSolvencyCheckHeight: 1000,
			mockSetup:               func(_ *mayaMockBridge) {},
			expectedError:           "",
			expectSolvencyReport:    false,
		},
		{
			name:                    "should handle error from GetAsgards",
			height:                  2500,
			lastSolvencyCheckHeight: 1000,
			mockSetup: func(bridge *mayaMockBridge) {
				bridge.getAsgardsFunc = func() (ttypes.Vaults, error) {
					return nil, fmt.Errorf("failed to get asgards")
				}
			},
			expectedError:        "fail to get asgards: failed to get asgards",
			expectSolvencyReport: false,
		},
		{
			name:                    "should report when height difference is large enough",
			height:                  2500,
			lastSolvencyCheckHeight: 1000,
			mockSetup: func(bridge *mayaMockBridge) {
				bridge.getAsgardsFunc = func() (ttypes.Vaults, error) {
					return asgards, nil
				}
			},
			expectedError:        "",
			expectSolvencyReport: true,
		},
	}

	s.fixtures = radixTestFixtures{
		transactionPreview: "../../../../test/fixtures/xrd/transaction_preview.json",
		networkStatus:      "../../../../test/fixtures/xrd/network_status.json",
	}

	for _, tc := range testCases {
		mockBridge := &mayaMockBridge{}
		tc.mockSetup(mockBridge)

		s.client.mayaBridge = mockBridge
		s.client.lastSolvencyCheckHeight = tc.lastSolvencyCheckHeight
		s.client.globalSolvencyQueue = make(chan stypes.Solvency, 1)

		// consume from the queue to prevent timeouts
		go func() {
			for range s.client.globalSolvencyQueue {
				// Just consume the messages
			}
		}()

		err := s.client.ReportSolvency(tc.height)

		if tc.expectedError != "" {
			c.Assert(err, NotNil)
			c.Assert(err.Error(), Equals, tc.expectedError)
		} else {
			c.Assert(err, IsNil)
		}

		if tc.expectSolvencyReport {
			c.Assert(tc.height, Equals, s.client.lastSolvencyCheckHeight)
		} else {
			c.Assert(tc.lastSolvencyCheckHeight, Equals, s.client.lastSolvencyCheckHeight)
		}
	}
}
