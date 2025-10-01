package radix

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"time"

	"gitlab.com/mayachain/mayanode/common/tokenlist"

	"github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	cKeys "github.com/cosmos/cosmos-sdk/crypto/keyring"
	"gitlab.com/mayachain/mayanode/bifrost/blockscanner"
	"gitlab.com/mayachain/mayanode/bifrost/mayaclient"
	"gitlab.com/mayachain/mayanode/bifrost/metrics"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/radix/coreapi"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/radix/types"
	"gitlab.com/mayachain/mayanode/bifrost/pubkeymanager"
	"gitlab.com/mayachain/mayanode/cmd"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/config"
	openapi "gitlab.com/mayachain/mayanode/openapi/gen"
	. "gopkg.in/check.v1"
)

type BlockScannerTestSuite struct {
	m           *metrics.Metrics
	bridge      mayaclient.MayachainBridge
	keys        *mayaclient.Keys
	coreApiMock *CoreAPIMock
	server      *httptest.Server
}

type CoreAPIMock struct {
	server *httptest.Server
}

func NewCoreAPIMock() *CoreAPIMock {
	mock := &CoreAPIMock{}
	mock.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mock.handler(w, r)
	}))
	return mock
}

func (m *CoreAPIMock) handler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/stream/transactions") {
		content, err := loadTestFixture("xrd/transactions.json")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		var response map[string]interface{}
		if err := json.Unmarshal(content, &response); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Connection", "keep-alive")

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		return
	}
}

func (m *CoreAPIMock) GetURL() string {
	return m.server.URL
}

func (m *CoreAPIMock) Close() {
	m.server.Close()
}

var _ = Suite(&BlockScannerTestSuite{})

func (s *BlockScannerTestSuite) SetUpSuite(c *C) {
	s.m = GetMetricForTest(c)
	c.Assert(s.m, NotNil)

	cfg := config.BifrostClientConfiguration{
		ChainID:         "mayachain",
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
	s.keys = thorKeys

	s.server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.RequestURI, mayaclient.ChainVersionEndpoint):
			_, err = rw.Write([]byte(`{"current":"1.113.0"}`))
			c.Assert(err, IsNil)

		case strings.HasPrefix(r.RequestURI, mayaclient.PubKeysEndpoint):
			priKey, _ := s.keys.GetPrivateKey()
			tm, _ := codec.ToTmPubKeyInterface(priKey.PubKey())
			var pk common.PubKey
			pk, err = common.NewPubKeyFromCrypto(tm)
			c.Assert(err, IsNil)
			var content []byte
			content, err = loadTestFixture("endpoints/vaults/pubKeys.json")
			c.Assert(err, IsNil)
			var pubKeysVault openapi.VaultPubkeysResponse
			c.Assert(json.Unmarshal(content, &pubKeysVault), IsNil)
			chain := common.XRDChain.String()
			mainRouter := "component_rdx1cqhr7fj5fspzccwht5e587pt6c44e4j5nzt647z2v4y3zazqe2mw59"

			pubKeysVault.Asgard = append(pubKeysVault.Asgard, openapi.VaultInfo{
				PubKey: pk.String(),
				Routers: []openapi.VaultRouter{
					{
						Chain:  &chain,
						Router: &mainRouter,
					},
				},
			})

			var buf []byte
			buf, err = json.MarshalIndent(pubKeysVault, "", "	")
			c.Assert(err, IsNil)
			_, err = rw.Write(buf)
			c.Assert(err, IsNil)
		}
	}))

	u, err := url.Parse(s.server.URL)
	c.Assert(err, IsNil)
	cfg.ChainHost = u.Host
	s.bridge, err = mayaclient.NewMayachainBridge(cfg, s.m, s.keys)
	c.Assert(err, IsNil)

	s.coreApiMock = NewCoreAPIMock()
}

func (s *BlockScannerTestSuite) TearDownSuite(c *C) {
	if s.server != nil {
		s.server.Close()
	}
	if s.coreApiMock != nil {
		s.coreApiMock.Close()
	}
}

func (s *BlockScannerTestSuite) TestNewBlockScanner(c *C) {
	storage, err := blockscanner.NewBlockScannerStorage("", config.LevelDBOptions{})
	c.Assert(err, IsNil)

	pubKeyManager, err := pubkeymanager.NewPubKeyManager(s.bridge, s.m)
	c.Assert(err, IsNil)

	radixApiClient, err := CreateRadixCoreApiClient(s.coreApiMock.GetURL())
	c.Assert(err, IsNil)

	network := types.NetworkFromChainNetwork(common.CurrentChainNetwork)
	coreApiWrapper := coreapi.NewCoreApiWrapper(radixApiClient, network, time.Second)

	cfg := getConfigForTest(s.coreApiMock.GetURL())

	bs, err := NewRadixScanner(cfg, storage, &coreApiWrapper, s.bridge, s.m, pubKeyManager, network, tokensByAddress())
	c.Assert(err, IsNil)
	c.Assert(bs, NotNil)

	bs, err = NewRadixScanner(cfg, storage, &coreApiWrapper, s.bridge, nil, pubKeyManager, network, tokensByAddress())
	c.Assert(err, NotNil)
	c.Assert(bs, IsNil)
}

func (s *BlockScannerTestSuite) TestProcessBlock(c *C) {
	storage, err := blockscanner.NewBlockScannerStorage("", config.LevelDBOptions{})
	c.Assert(err, IsNil)

	pubKeyManager, err := pubkeymanager.NewPubKeyManager(s.bridge, s.m)
	c.Assert(err, IsNil)
	c.Assert(pubKeyManager.Start(), IsNil)
	defer func() {
		c.Assert(pubKeyManager.Stop(), IsNil)
	}()

	radixApiClient, err := CreateRadixCoreApiClient(s.coreApiMock.GetURL())
	c.Assert(err, IsNil)

	network := types.NetworkFromChainNetwork(common.CurrentChainNetwork)
	coreApiWrapper := coreapi.NewCoreApiWrapper(radixApiClient, network, time.Second)

	bs, err := NewRadixScanner(getConfigForTest(s.coreApiMock.GetURL()), storage, &coreApiWrapper, s.bridge, s.m, pubKeyManager, network, tokensByAddress())
	c.Assert(err, IsNil)
	c.Assert(bs, NotNil)

	txIn, err := bs.FetchTxs(int64(169291385), int64(169291385))
	c.Assert(err, IsNil)
	c.Assert(txIn.TxArray, NotNil)
}

func tokensByAddress() map[string]tokenlist.RadixToken {
	ret := make(map[string]tokenlist.RadixToken)
	symbols := make(map[string]bool)
	for _, token := range tokenlist.GetRadixTokenList(common.LatestVersion) {
		upperAddr := strings.ToUpper(token.Address)
		symbol := strings.ToUpper(token.Symbol)
		if _, exists := ret[upperAddr]; exists {
			panic(fmt.Sprintf("Duplicate token address detected: %s", upperAddr))
		}
		if _, exists := symbols[symbol]; exists {
			panic(fmt.Sprintf("Duplicate token symbol detected: %s", symbol))
		}
		ret[upperAddr] = token
		symbols[symbol] = true
	}
	return ret
}

func getConfigForTest(rpcHost string) config.BifrostBlockScannerConfiguration {
	return config.BifrostBlockScannerConfiguration{
		ChainID:                    common.XRDChain,
		RPCHost:                    rpcHost,
		StartBlockHeight:           1,
		BlockScanProcessors:        1,
		HTTPRequestTimeout:         time.Second,
		HTTPRequestReadTimeout:     time.Second * 30,
		HTTPRequestWriteTimeout:    time.Second * 30,
		MaxHTTPRequestRetry:        3,
		BlockHeightDiscoverBackoff: time.Second,
		BlockRetryInterval:         time.Second,
		Concurrency:                1,
	}
}

func loadTestFixture(path string) ([]byte, error) {
	return os.ReadFile("../../../../test/fixtures/" + path)
}
