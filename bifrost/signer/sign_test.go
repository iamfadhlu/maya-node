package signer

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	cKeys "github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/rs/zerolog/log"
	"github.com/tendermint/tendermint/crypto"
	"gitlab.com/mayachain/mayanode/bifrost/tss/go-tss/blame"
	"gitlab.com/mayachain/mayanode/bifrost/tss/go-tss/keysign"
	tssMessages "gitlab.com/mayachain/mayanode/bifrost/tss/go-tss/messages"
	ctypes "gitlab.com/thorchain/binance-sdk/common/types"

	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/bifrost/blockscanner"
	"gitlab.com/mayachain/mayanode/bifrost/mayaclient"
	"gitlab.com/mayachain/mayanode/bifrost/mayaclient/types"
	stypes "gitlab.com/mayachain/mayanode/bifrost/mayaclient/types"
	"gitlab.com/mayachain/mayanode/bifrost/metrics"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients"
	"gitlab.com/mayachain/mayanode/bifrost/pubkeymanager"
	"gitlab.com/mayachain/mayanode/bifrost/tss"
	"gitlab.com/mayachain/mayanode/cmd"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/config"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain"
	types2 "gitlab.com/mayachain/mayanode/x/mayachain/types"
)

////////////////////////////////////////////////////////////////////////////////////////
// Mocks
////////////////////////////////////////////////////////////////////////////////////////

// -------------------------------- bridge ---------------------------------

type fakeBridge struct {
	mayaclient.MayachainBridge
}

func (b fakeBridge) GetBlockHeight() (int64, error) {
	return 100, nil
}

func (b fakeBridge) GetThorchainVersion() (semver.Version, error) {
	return semver.MustParse("1.0.0"), nil
}

func (b fakeBridge) GetConstants() (map[string]int64, error) {
	return map[string]int64{
		constants.SigningTransactionPeriod.String(): 300,
	}, nil
}

func (b fakeBridge) GetMimir(key string) (int64, error) {
	if strings.HasPrefix(key, "HALT") {
		return 0, nil
	}

	if key == constants.MaxOutboundAttempts.String() {
		return 100, nil
	}

	if key == constants.SignerConcurrency.String() {
		return 3, nil
	}
	panic("not implemented")
}

func (b fakeBridge) GetVault(pubkey string) (types2.Vault, error) {
	pk, err := common.NewPubKey(pubkey)
	if err != nil {
		return types2.Vault{}, err
	}
	return types2.Vault{
		PubKey: pk,
		Status: types2.VaultStatus_ActiveVault,
	}, nil
}

// -------------------------------- tss ---------------------------------

type fakeTssServer struct {
	counter int
	results map[int]keysign.Response
	fixed   *keysign.Response
}

func (tss *fakeTssServer) KeySign(req keysign.Request) (keysign.Response, error) {
	tss.counter += 1

	if tss.fixed != nil {
		return *tss.fixed, nil
	}

	result, ok := tss.results[tss.counter]
	if ok {
		return result, nil
	}

	return keysign.Response{}, fmt.Errorf("unhandled counter")
}

func newFakeTss(msg string, succeedOnly bool) *fakeTssServer {
	success := keysign.Response{
		Status: 1, // 1 is success
		Signatures: []keysign.Signature{
			{
				R:   base64.StdEncoding.EncodeToString([]byte("R")),
				S:   base64.StdEncoding.EncodeToString([]byte("S")),
				Msg: base64.StdEncoding.EncodeToString([]byte(msg)),
			},
		},
	}

	if succeedOnly {
		return &fakeTssServer{
			fixed: &success,
		}
	}

	results := make(map[int]keysign.Response)
	results[1] = keysign.Response{
		Status: 2, // 2 is fail
		Blame: blame.Blame{
			Round: tssMessages.KEYSIGN7,
			BlameNodes: []blame.Node{
				{Pubkey: "node1"},
			},
		},
	}
	results[2] = keysign.Response{
		Status: 2, // 2 is fail
		Blame: blame.Blame{
			Round: tssMessages.KEYSIGN7,
			BlameNodes: []blame.Node{
				{Pubkey: "node2"},
			},
		},
	}
	results[3] = keysign.Response{
		Status: 2, // 2 is fail, as non-round7
		Blame: blame.Blame{
			Round: tssMessages.KEYSIGN3,
			BlameNodes: []blame.Node{
				{Pubkey: "node2"},
			},
		},
	}
	results[4] = keysign.Response{
		Status: 2, // 2 is fail
		Blame: blame.Blame{
			Round: tssMessages.KEYSIGN7,
			BlameNodes: []blame.Node{
				{Pubkey: "node3"},
			},
		},
	}
	results[5] = success
	results[6] = success
	results[7] = success

	return &fakeTssServer{
		counter: 0,
		results: results,
	}
}

// --------------------------------- chain client ---------------------------------

type MockChainClient struct {
	account            common.Account
	signCount          int
	broadcastCount     int
	ks                 *tss.KeySign
	assertCheckpoint   bool
	broadcastFailCount int
}

func (b *MockChainClient) IsBlockScannerHealthy() bool {
	return true
}

func (b *MockChainClient) SignTx(tai stypes.TxOutItem, height int64) ([]byte, []byte, *stypes.TxInItem, error) {
	if b.ks == nil {
		return nil, nil, nil, nil
	}

	// assert that this signing should have the checkpoint set
	if b.assertCheckpoint {
		if !bytes.Equal(tai.Checkpoint, []byte(tai.Memo)) {
			panic("checkpoint should be set")
		}
	} else {
		if bytes.Equal(tai.Checkpoint, []byte(tai.Memo)) {
			panic("checkpoint should not be set")
		}
	}

	b.signCount += 1
	sig, _, err := b.ks.RemoteSign([]byte(tai.Memo), common.SigningAlgoSecp256k1, tai.VaultPubKey.String())

	return sig, []byte(tai.Memo), nil, err
}

func (b *MockChainClient) GetConfig() config.BifrostChainConfiguration {
	return config.BifrostChainConfiguration{}
}

func (b *MockChainClient) GetHeight() (int64, error) {
	return 0, nil
}

func (b *MockChainClient) GetGasFee(count uint64) common.Gas {
	coins := make(common.Coins, count)
	return common.CalcBinanceGasPrice(common.Tx{Coins: coins}, common.BNBAsset, []cosmos.Uint{cosmos.NewUint(37500), cosmos.NewUint(30000)})
}

func (b *MockChainClient) CheckIsTestNet() (string, bool) {
	return "", true
}

func (b *MockChainClient) GetChain() common.Chain {
	return common.BNBChain
}

func (b *MockChainClient) GetBlockScannerHeight() (int64, error) {
	return 0, nil
}

func (b *MockChainClient) GetLatestTxForVault(vault string) (string, string, error) {
	return "", "", nil
}

func (b *MockChainClient) Churn(pubKey common.PubKey, height int64) error {
	return nil
}

func (b *MockChainClient) BroadcastTx(_ stypes.TxOutItem, tx []byte) (string, error) {
	b.broadcastCount += 1
	if b.broadcastCount > b.broadcastFailCount {
		return "", nil
	}
	return "", fmt.Errorf("broadcast failed")
}

func (b *MockChainClient) GetAddress(poolPubKey common.PubKey) string {
	return "0dd3d0a4a6eacc98cc4894791702e46c270bde76"
}

func (b *MockChainClient) GetAccount(poolPubKey common.PubKey, _ *big.Int) (common.Account, error) {
	return b.account, nil
}

func (b *MockChainClient) GetAccountByAddress(address string, _ *big.Int) (common.Account, error) {
	return b.account, nil
}

func (b *MockChainClient) GetPubKey() crypto.PubKey {
	return nil
}

func (b *MockChainClient) OnObservedTxIn(txIn types.TxInItem, blockHeight int64) {
}

func (b *MockChainClient) Start(globalTxsQueue chan stypes.TxIn, globalErrataQueue chan stypes.ErrataBlock, globalSolvencyQueue chan stypes.Solvency) {
}

func (b *MockChainClient) Stop() {}
func (b *MockChainClient) ConfirmationCountReady(txIn stypes.TxIn) bool {
	return true
}

func (b *MockChainClient) GetConfirmationCount(txIn stypes.TxIn) int64 {
	return 0
}

////////////////////////////////////////////////////////////////////////////////////////
// Tests
////////////////////////////////////////////////////////////////////////////////////////

var m *metrics.Metrics

func GetMetricForTest(c *C) *metrics.Metrics {
	if m == nil {
		var err error
		m, err = metrics.NewMetrics(config.BifrostMetricsConfiguration{
			Enabled:      false,
			ListenPort:   9000,
			ReadTimeout:  time.Second,
			WriteTimeout: time.Second,
			Chains:       common.Chains{common.BNBChain},
		})
		c.Assert(m, NotNil)
		c.Assert(err, IsNil)
	}
	return m
}

type SignSuite struct {
	thordir  string
	thorKeys *mayaclient.Keys
	bridge   mayaclient.MayachainBridge
	m        *metrics.Metrics
	rpcHost  string
	storage  *SignerStore
}

var _ = Suite(&SignSuite{})

func (s *SignSuite) SetUpSuite(c *C) {
	mayachain.SetupConfigForTest()
	s.m = GetMetricForTest(c)
	c.Assert(s.m, NotNil)
	ns := strconv.Itoa(time.Now().Nanosecond())
	types2.SetupConfigForTest()
	ctypes.Network = ctypes.TestNetwork
	c.Assert(os.Setenv("NET", "testnet"), IsNil)

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		c.Logf("requestUri:%s", req.RequestURI)
		if strings.HasPrefix(req.RequestURI, "/txs") { // nolint
			_, err := rw.Write([]byte(`{ "jsonrpc": "2.0", "id": "", "result": { "height": "1", "txhash": "AAAA000000000000000000000000000000000000000000000000000000000000", "logs": [{"success": "true", "log": ""}] } }`))
			c.Assert(err, IsNil)
		} else if strings.HasPrefix(req.RequestURI, "/mayachain/lastblock/BNB") {
			_, err := rw.Write([]byte(`{ "jsonrpc": "2.0", "id": "", "result": { "chain": "BNB", "lastobservedin": "0", "lastsignedout": "0", "mayachain": "0" } }`))
			c.Assert(err, IsNil)
		} else if strings.HasPrefix(req.RequestURI, "/mayachain/lastblock") {
			_, err := rw.Write([]byte(`{ "jsonrpc": "2.0", "id": "", "result": { "chain": "mayachain", "lastobservedin": "0", "lastsignedout": "0", "mayachain": "0" } }`))
			c.Assert(err, IsNil)
		} else if strings.HasPrefix(req.RequestURI, "/auth/accounts/") {
			_, err := rw.Write([]byte(`{ "jsonrpc": "2.0", "id": "", "result": { "height": "0", "result": { "value": { "account_number": "0", "sequence": "0" } } } }`))
			c.Assert(err, IsNil)
		} else if strings.HasPrefix(req.RequestURI, "/thorchain/vaults/pubkeys") {
			_, err := rw.Write([]byte(`{ "jsonrpc": "2.0", "id": "", "result": { "asgard": ["tmayapub1addwnpepq2jgpsw2lalzuk7sgtmyakj7l6890f5cfpwjyfp8k4y4t7cw2vk8vvgpt43"], "yggdrasil": ["tmayapub1addwnpepqdqvd4r84lq9m54m5kk9sf4k6kdgavvch723pcgadulxd6ey9u70kujkcf2"] } }`))
			c.Assert(err, IsNil)
		} else if strings.HasPrefix(req.RequestURI, "/mayachain/keysign") {
			_, err := rw.Write([]byte(`{
			"chains": {
				"BNB": {
					"chain": "BNB",
					"hash": "",
					"height": "1",
					"tx_array": [
						{
							"chain": "BNB",
							"coin": {
								"amount": "10000000000",
								"asset": "BNB.BNB"
							},
							"in_hash": "ENULZOBGZHEKFOIBYRLLBELKFZVGXOBLTRQGTOWNDHMPZQMBLGJETOXJLHPVQIKY",
							"memo": "",
							"out_hash": "",
							"to": "tbnb145wcuncewfkuc4v6an0r9laswejygcul43c3wu",
							"vault_pubkey": "thorpub1addwnpepqfgfxharps79pqv8fv9ndqh90smw8c3slrtrssn58ryc5g3p9sx856x07yn"
						}
					]
				}
			}
		}
	`))
			c.Assert(err, IsNil)
		} else if strings.HasSuffix(req.RequestURI, "/signers") {
			_, err := rw.Write([]byte(`[
  "tmayapub1addwnpepqflvfv08t6qt95lmttd6wpf3ss8wx63e9vf6fvyuj2yy6nnyna5769e5dlr",
  "tmayapub1addwnpepq2flfr96skc5lkwdv0n5xjsnhmuju20x3zndgu42zd8dtkrud9m2vmzp38f",
  "tmayapub1addwnpepqwhnus6xs4208d4ynm05lv493amz3fexfjfx4vptntedd7k0ajlcu40le0r"
]`))
			c.Assert(err, IsNil)
		}
	}))

	s.thordir = filepath.Join(os.TempDir(), ns, ".thorcli")
	split := strings.SplitAfter(server.URL, ":")
	s.rpcHost = split[len(split)-1]
	cfg := config.BifrostClientConfiguration{
		ChainID:         "mayachain",
		ChainHost:       "localhost:" + s.rpcHost,
		SignerName:      "bob",
		SignerPasswd:    "1passw0rd1",
		ChainHomeFolder: s.thordir,
	}

	kb := cKeys.NewInMemory()
	_, _, err := kb.NewMnemonic(cfg.SignerName, cKeys.English, cmd.BASEChainHDPath, cfg.SignerPasswd, hd.Secp256k1)
	c.Assert(err, IsNil)
	s.thorKeys = mayaclient.NewKeysWithKeybase(kb, cfg.SignerName, cfg.SignerPasswd)
	c.Assert(err, IsNil)
	s.bridge, err = mayaclient.NewMayachainBridge(cfg, s.m, s.thorKeys)
	c.Assert(err, IsNil)
	s.storage, err = NewSignerStore("", config.LevelDBOptions{}, "S0m4@P4ssphr4s3")
	c.Assert(err, IsNil)
}

func (s *SignSuite) TearDownSuite(c *C) {
	c.Assert(os.Unsetenv("NET"), IsNil)

	if err := os.RemoveAll(s.thordir); err != nil {
		c.Error(err)
	}

	if err := os.RemoveAll("signer_data"); err != nil {
		c.Error(err)
	}
	tempPath := filepath.Join(os.TempDir(), "/var/data/bifrost/signer")
	if err := os.RemoveAll(tempPath); err != nil {
		c.Error(err)
	}

	if err := os.RemoveAll("signer/var"); err != nil {
		c.Error(err)
	}
}

func (s *SignSuite) TestHandleYggReturn_Success_FeeSingleton(c *C) {
	sign := &Signer{
		chains: map[common.Chain]chainclients.ChainClient{
			common.BNBChain: &MockChainClient{
				account: common.Account{
					Coins: common.Coins{
						common.NewCoin(common.BNBAsset, cosmos.NewUint(1000000)),
					},
				},
			},
		},
		pubkeyMgr: pubkeymanager.NewMockPoolAddressValidator(),
	}
	input := `{ "chain": "BNB", "memo": "", "to": "tbnb1yycn4mh6ffwpjf584t8lpp7c27ghu03gpvqkfj", "coins": [] }`
	var item stypes.TxOutItem
	err := json.Unmarshal([]byte(input), &item)
	c.Check(err, IsNil)

	newItem, err := sign.handleYggReturn(12, item)
	c.Assert(err, IsNil)
	c.Check(newItem.Coins[0].Amount.Uint64(), Equals, uint64(1000000))
}

func (s *SignSuite) TestHandleYggReturn_Success_FeeMulti(c *C) {
	sign := &Signer{
		chains: map[common.Chain]chainclients.ChainClient{
			common.BNBChain: &MockChainClient{
				account: common.Account{
					Coins: common.Coins{
						common.NewCoin(common.BNBAsset, cosmos.NewUint(1000000)),
						common.NewCoin(common.BaseAsset(), cosmos.NewUint(1000000)),
					},
				},
			},
		},
		pubkeyMgr: pubkeymanager.NewMockPoolAddressValidator(),
	}
	input := `{ "chain": "BNB", "memo": "", "to": "tbnb1yycn4mh6ffwpjf584t8lpp7c27ghu03gpvqkfj", "coins": [] }`
	var item stypes.TxOutItem
	err := json.Unmarshal([]byte(input), &item)
	c.Check(err, IsNil)

	newItem, err := sign.handleYggReturn(22, item)
	c.Assert(err, IsNil)
	c.Check(newItem.Coins[0].Amount.Uint64(), Equals, uint64(1000000))
}

func (s *SignSuite) TestHandleYggReturn_Success_NotEnough(c *C) {
	sign := &Signer{
		chains: map[common.Chain]chainclients.ChainClient{
			common.BNBChain: &MockChainClient{
				account: common.Account{
					Coins: common.Coins{
						common.NewCoin(common.BNBAsset, cosmos.NewUint(0)),
					},
				},
			},
		},
		pubkeyMgr: pubkeymanager.NewMockPoolAddressValidator(),
	}
	input := `{ "chain": "BNB", "memo": "", "to": "tbnb1yycn4mh6ffwpjf584t8lpp7c27ghu03gpvqkfj", "coins": [] }`
	var item stypes.TxOutItem
	err := json.Unmarshal([]byte(input), &item)
	c.Check(err, IsNil)

	newItem, err := sign.handleYggReturn(33, item)
	c.Assert(err, IsNil)
	c.Check(newItem.Coins, HasLen, 0)
}

func (s *SignSuite) TestProcess(c *C) {
	cfg := config.BifrostSignerConfiguration{
		SignerDbPath: filepath.Join(os.TempDir(), "/var/data/bifrost/signer"),
		BlockScanner: config.BifrostBlockScannerConfiguration{
			RPCHost:                    "127.0.0.1:" + s.rpcHost,
			ChainID:                    "mayachain",
			StartBlockHeight:           1,
			EnforceBlockHeight:         true,
			BlockScanProcessors:        1,
			BlockHeightDiscoverBackoff: time.Second,
			BlockRetryInterval:         10 * time.Second,
		},
		RetryInterval: 2 * time.Second,
	}

	chains := map[common.Chain]chainclients.ChainClient{
		common.BNBChain: &MockChainClient{
			account: common.Account{
				Coins: common.Coins{
					common.NewCoin(common.BNBAsset, cosmos.NewUint(1000000)),
					common.NewCoin(common.BaseAsset(), cosmos.NewUint(1000000)),
				},
			},
		},
	}

	blockScan, err := NewThorchainBlockScan(cfg.BlockScanner, s.storage, s.bridge, s.m, pubkeymanager.NewMockPoolAddressValidator())
	c.Assert(err, IsNil)

	blockScanner, err := blockscanner.NewBlockScanner(cfg.BlockScanner, s.storage, m, s.bridge, blockScan)
	c.Assert(err, IsNil)

	sign := &Signer{
		logger:                log.With().Str("module", "signer").Logger(),
		cfg:                   cfg,
		wg:                    &sync.WaitGroup{},
		stopChan:              make(chan struct{}),
		blockScanner:          blockScanner,
		thorchainBlockScanner: blockScan,
		chains:                chains,
		m:                     s.m,
		storage:               s.storage,
		errCounter:            s.m.GetCounterVec(metrics.SignerError),
		pubkeyMgr:             pubkeymanager.NewMockPoolAddressValidator(),
		mayachainBridge:       s.bridge,
	}
	c.Assert(sign, NotNil)
	err = sign.Start()
	c.Assert(err, IsNil)
	time.Sleep(time.Second * 2)
	// nolint
	go sign.Stop()
}

func (s *SignSuite) TestBroadcastRetry(c *C) {
	vaultPubKey, err := common.NewPubKey(pubkeymanager.MockPubkey)
	c.Assert(err, IsNil)

	// start a mock keysign
	msg := "foobar"
	tssServer := newFakeTss(msg, true)
	bridge := fakeBridge{s.bridge}
	ks, err := tss.NewKeySign(tssServer, bridge)
	c.Assert(err, IsNil)
	ks.Start()

	// creat mock chain client and signer
	cc := &MockChainClient{ks: ks, broadcastFailCount: 2}
	sign := &Signer{
		chains: map[common.Chain]chainclients.ChainClient{
			common.BNBChain: cc,
		},
		pubkeyMgr:           pubkeymanager.NewMockPoolAddressValidator(),
		stopChan:            make(chan struct{}),
		wg:                  &sync.WaitGroup{},
		mayachainBridge:     bridge,
		constantsProvider:   NewConstantsProvider(bridge),
		tssKeysignMetricMgr: metrics.NewTssKeysignMetricMgr(),
		logger:              log.With().Str("module", "signer").Logger(),
	}

	// create a signer store with fake txouts
	sign.storage, err = NewSignerStore("", config.LevelDBOptions{}, "S0m4@P4ssphr4s3")
	c.Assert(err, IsNil)
	err = sign.storage.Set(TxOutStoreItem{
		TxOutItem: stypes.TxOutItem{
			Chain:       common.BNBChain,
			ToAddress:   "tbnb1yycn4mh6ffwpjf584t8lpp7c27ghu03gpvqkfj",
			Memo:        msg,
			VaultPubKey: vaultPubKey,
			Coins: common.Coins{ // must be set or signer overrides memo
				common.NewCoin(common.BNBAsset, cosmos.NewUint(1000000)),
			},
		},
	})
	c.Assert(err, IsNil)

	// first attempt should fail broadcast and set signed tx
	sign.processTransactions()
	sign.pipeline.Wait()
	c.Assert(cc.signCount, Equals, 1)
	c.Assert(tssServer.counter, Equals, 1)
	c.Assert(cc.broadcastCount, Equals, 1)
	tois := sign.storage.List()
	c.Assert(err, IsNil)
	c.Assert(tois, HasLen, 1)
	c.Assert(tois[0].Checkpoint, IsNil)
	c.Assert(len(tois[0].SignedTx), Equals, 64)

	// second attempt should not sign and still fail broadcast
	sign.processTransactions()
	sign.pipeline.Wait()
	c.Assert(cc.signCount, Equals, 1)
	c.Assert(tssServer.counter, Equals, 1)
	c.Assert(cc.broadcastCount, Equals, 2)
	tois = sign.storage.List()
	c.Assert(err, IsNil)
	c.Assert(tois, HasLen, 1)
	c.Assert(tois[0].Checkpoint, IsNil)
	c.Assert(len(tois[0].SignedTx), Equals, 64)

	// third attempt should not sign and succeed broadcast
	sign.processTransactions()
	sign.pipeline.Wait()
	c.Assert(cc.signCount, Equals, 1)
	c.Assert(tssServer.counter, Equals, 1)
	c.Assert(cc.broadcastCount, Equals, 3)
	tois = sign.storage.List()
	c.Assert(err, IsNil)
	c.Assert(tois, HasLen, 0)

	// stop signer
	close(sign.stopChan)
	sign.wg.Wait()
	ks.Stop()
}

func (s *SignSuite) TestRound7Retry(c *C) {
	vaultPubKey, err := common.NewPubKey(pubkeymanager.MockPubkey)
	c.Assert(err, IsNil)

	// start a mock keysign, succeeds on 5th try
	msg := "foobar"
	tssServer := newFakeTss(msg, false)
	bridge := fakeBridge{s.bridge}
	ks, err := tss.NewKeySign(tssServer, bridge)
	c.Assert(err, IsNil)
	ks.Start()

	// creat mock chain client and signer
	cc := &MockChainClient{ks: ks}
	sign := &Signer{
		chains: map[common.Chain]chainclients.ChainClient{
			common.BNBChain: cc,
		},
		pubkeyMgr:           pubkeymanager.NewMockPoolAddressValidator(),
		stopChan:            make(chan struct{}),
		wg:                  &sync.WaitGroup{},
		mayachainBridge:     bridge,
		constantsProvider:   NewConstantsProvider(bridge),
		tssKeysignMetricMgr: metrics.NewTssKeysignMetricMgr(),
		logger:              log.With().Str("module", "signer").Logger(),
	}

	// create a signer store with fake txouts
	sign.storage, err = NewSignerStore("", config.LevelDBOptions{}, "S0m4@P4ssphr4s3")
	c.Assert(err, IsNil)
	err = sign.storage.Set(TxOutStoreItem{
		TxOutItem: stypes.TxOutItem{
			Chain:       common.BNBChain,
			ToAddress:   "tbnb1yycn4mh6ffwpjf584t8lpp7c27ghu03gpvqkfj",
			Memo:        msg,
			VaultPubKey: vaultPubKey,
			Coins: common.Coins{ // must be set or signer overrides memo
				common.NewCoin(common.BNBAsset, cosmos.NewUint(1000000)),
			},
		},
	})
	c.Assert(err, IsNil)
	err = sign.storage.Set(TxOutStoreItem{
		TxOutItem: stypes.TxOutItem{
			Chain:       common.BNBChain,
			ToAddress:   "tbnb145wcuncewfkuc4v6an0r9laswejygcul43c3wu",
			Memo:        msg,
			VaultPubKey: vaultPubKey,
			Coins: common.Coins{ // must be set or signer overrides memo
				common.NewCoin(common.BNBAsset, cosmos.NewUint(1000000)),
			},
		},
	})
	c.Assert(err, IsNil)
	err = sign.storage.Set(TxOutStoreItem{
		TxOutItem: stypes.TxOutItem{
			Chain:       common.BNBChain,
			ToAddress:   "tbnb1yxfyeda8pnlxlmx0z3cwx74w9xevspwdpzdxpj",
			Memo:        msg,
			VaultPubKey: vaultPubKey,
			Coins: common.Coins{ // must be set or signer overrides memo
				common.NewCoin(common.BNBAsset, cosmos.NewUint(1000000)),
			},
		},
	})
	c.Assert(err, IsNil)

	// this will be ignored entirely since the vault pubkey is different
	err = sign.storage.Set(TxOutStoreItem{
		TxOutItem: stypes.TxOutItem{
			Chain:       common.BNBChain,
			ToAddress:   "tbnb145wcuncewfkuc4v6an0r9laswejygcul43c3wu",
			Memo:        msg,
			VaultPubKey: "tmayapub1addwnpepqfup3y8p0egd7ml7vrnlxgl3wvnp89mpn0tjpj0p2nm2gh0n9hlrv9mjq5h",
			Coins: common.Coins{ // must be set or signer overrides memo
				common.NewCoin(common.BNBAsset, cosmos.NewUint(1000000)),
			},
		},
	})
	c.Assert(err, IsNil)

	// create the same on different chain, should move independently
	msg2 := "foobar2"
	tssServer2 := newFakeTss(msg2, true) // this one succeeds on first try
	bridge2 := fakeBridge{s.bridge}
	ks2, err := tss.NewKeySign(tssServer2, bridge2)
	c.Assert(err, IsNil)
	ks2.Start()
	cc2 := &MockChainClient{ks: ks2}
	sign.chains[common.BTCChain] = cc2
	tois2 := TxOutStoreItem{
		TxOutItem: stypes.TxOutItem{
			Chain:       common.BTCChain,
			ToAddress:   "tbtc1yycn4mh6ffwpjf584t8lpp7c27ghu03gpvqkfj",
			VaultPubKey: vaultPubKey,
			Memo:        msg2,
			Coins: common.Coins{
				common.NewCoin(common.BTCAsset, cosmos.NewUint(1000000)),
			},
		},
	}
	err = sign.storage.Set(tois2)
	c.Assert(err, IsNil)

	// first round only btc tx should go through
	sign.processTransactions()
	sign.pipeline.Wait()
	c.Assert(cc.signCount, Equals, 1)
	c.Assert(cc.broadcastCount, Equals, 0)
	c.Assert(tssServer.counter, Equals, 1)
	c.Assert(cc2.signCount, Equals, 1)
	c.Assert(cc2.broadcastCount, Equals, 1)
	c.Assert(tssServer2.counter, Equals, 1)

	// all bnb txs should be remaining, first marked round 7
	tois := sign.storage.List()
	c.Assert(len(tois), Equals, 3)
	c.Assert(tois[0].Round7Retry, Equals, true)
	c.Assert(bytes.Equal(tois[0].Checkpoint, []byte(msg)), Equals, true)
	c.Assert(tois[1].Round7Retry, Equals, false)
	c.Assert(tois[2].Round7Retry, Equals, false)

	// process transactions 3 more times
	cc.assertCheckpoint = true // the following signs should pass checkpoint
	for i := 0; i < 3; i++ {
		sign.processTransactions()
		sign.pipeline.Wait()
	}

	// first bnb tx should have been retried 3 times, no broadcast yet
	c.Assert(cc.signCount, Equals, 4)
	c.Assert(cc.broadcastCount, Equals, 0)
	c.Assert(tssServer.counter, Equals, 4)

	// this round should sign and broadcast the round 7 retry
	sign.processTransactions()
	sign.pipeline.Wait()
	c.Assert(cc.signCount, Equals, 5)
	c.Assert(cc.broadcastCount, Equals, 1)
	c.Assert(tssServer.counter, Equals, 5)
	tois = sign.storage.List()
	c.Assert(len(tois), Equals, 2)
	c.Assert(tois[0].Round7Retry, Equals, false)
	c.Assert(tois[1].Round7Retry, Equals, false)

	// the next 2 rounds should sign and broadcast the remaining
	cc.assertCheckpoint = false // the following signs should not pass checkpoint

	// only processes 1 per vault/chain in the pipeline
	sign.processTransactions()
	sign.pipeline.Wait()
	c.Assert(cc.signCount, Equals, 6)
	c.Assert(cc.broadcastCount, Equals, 2)
	c.Assert(tssServer.counter, Equals, 6)
	c.Assert(len(sign.storage.List()), Equals, 1)

	// last one
	sign.processTransactions()
	sign.pipeline.Wait()
	c.Assert(cc.signCount, Equals, 7)
	c.Assert(cc.broadcastCount, Equals, 3)
	c.Assert(tssServer.counter, Equals, 7)
	c.Assert(len(sign.storage.List()), Equals, 0)

	// nothing more should have happened on btc
	for i := 0; i < 3; i++ {
		sign.processTransactions()
		sign.pipeline.Wait()
	}
	c.Assert(cc.signCount, Equals, 7)
	c.Assert(cc.broadcastCount, Equals, 3)
	c.Assert(tssServer.counter, Equals, 7)
	c.Assert(cc2.signCount, Equals, 1)
	c.Assert(cc2.broadcastCount, Equals, 1)
	c.Assert(tssServer2.counter, Equals, 1)
	c.Assert(len(sign.storage.List()), Equals, 0)

	// stop signer
	close(sign.stopChan)
	sign.wg.Wait()
	ks.Stop()
	ks2.Stop()
}
