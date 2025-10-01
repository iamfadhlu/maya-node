package dash

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/bifrost/mayaclient"
	"gitlab.com/mayachain/mayanode/common"
)

type jsonmap map[string]interface{}

// fixtureCache caches fixture data to avoid repeated disk reads
var fixtureCache = struct {
	sync.RWMutex
	data map[string][]byte
}{
	data: make(map[string][]byte),
}

// getCachedFixture returns fixture data from cache or reads from disk if not cached
func getCachedFixture(path string, c *C) []byte {
	// Check cache first
	fixtureCache.RLock()
	content, exists := fixtureCache.data[path]
	fixtureCache.RUnlock()

	if exists {
		return content
	}

	// Read from disk if not in cache
	fullPath := "../../../../test/fixtures" + path
	content, err := os.ReadFile(fullPath)
	if err != nil {
		c.Fatal(err)
	}

	// Cache the content
	fixtureCache.Lock()
	fixtureCache.data[path] = content
	fixtureCache.Unlock()

	return content
}

func mockHttpResponses(c *C) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		fixture := func(path string) {
			content := getCachedFixture(path, c)
			rw.Header().Set("Content-Type", "application/json")
			if _, err := rw.Write(content); err != nil {
				c.Fatal(err)
			}
		}

		mockDashNodeResponse := func() {
			r := struct {
				Method string        `json:"method"`
				Params []interface{} `json:"params"`
			}{}

			err := json.NewDecoder(req.Body).Decode(&r)
			c.Assert(err, IsNil)

			switch r.Method {
			case "createwallet":
				fixture("/dash/createwallet.json")
			case "getnetworkinfo":
				fixture("/dash/getnetworkinfo.json")
			case "getblockhash":
				fixture("/dash/blockhash.json")
			case "getbestblockhash":
				fixture("/dash/getbestblockhash.json")
			case "getbestchainlock":
				fixture("/dash/getbestchainlock.json")
			case "getblock":
				verbosity := "0"
				if len(r.Params) >= 2 {
					verbosity = fmt.Sprintf("%v", r.Params[1])
				}
				switch verbosity {
				case "2":
					fixture("/dash/block_verbose.json")
				default:
					fixture("/dash/block.json")
				}
			case "getrawtransaction":
				txid := ""
				if len(r.Params) >= 1 {
					txid = fmt.Sprintf("%v", r.Params[0])
				}
				fixtureTxids := []string{"5da7", "5e6d", "8b3e", "9d34", "937c", "f3fc"}
				for _, fixtureTxid := range fixtureTxids {
					if strings.HasPrefix(txid, fixtureTxid) {
						fixture("/dash/tx-" + fixtureTxid + ".json")
						return
					}
				}
				fixture("/dash/tx.json")
			case "getblockcount":
				fixture("/dash/blockcount.json")
			case "importaddress":
				fixture("/dash/importaddress.json")
			case "getblockstats":
				fixture("/dash/blockstats.json")
			case "sendrawtransaction":
				fixture("/dash/sendrawtransaction.json")
			case "estimatesmartfee":
				fixture("/dash/estimatesmartfee.json")
			case "listunspent":
				fixture("/dash/listunspent.json")
			default:
				c.Fatalf("Dash node response for method '%s' has not been mocked.", r.Method)
			}
		}

		mockThorchainResponse := func() {
			switch {
			case req.RequestURI == "/mayachain/lastblock":
				fixture("/endpoints/lastblock/dash.json")

			case req.RequestURI == "/mayachain/mimir/key/MaxUTXOsToSpend":
				_, err := rw.Write([]byte(`-1`))
				c.Assert(err, IsNil)

			case req.RequestURI == "/txs":
				response, _ := json.Marshal(jsonmap{
					"height": "1",
					"txhash": "AAAA000000000000000000000000000000000000000000000000000000000000",
					"logs": []jsonmap{
						{
							"success": "true",
							"log":     "",
						},
					},
				})
				_, err := rw.Write(response)
				c.Assert(err, IsNil)

			case req.RequestURI == "/mayachain/vaults/tmayapub1addwnpepqwznsrgk2t5vn2cszr6ku6zned6tqxknugzw3vhdcjza284d7djp59sf99q/signers":
				_, err := rw.Write([]byte("[]"))
				c.Assert(err, IsNil)

			case req.RequestURI == "/mayachain/version":
				fixture("/endpoints/version/version.json")

			case strings.HasPrefix(req.RequestURI, "/mayachain/node/"):
				fixture("/endpoints/nodeaccount/template.json")

			case strings.HasPrefix(req.RequestURI, "/auth/accounts/"):
				response, _ := json.Marshal(jsonmap{
					"jsonrpc": "2.0",
					"id":      "",
					"result": jsonmap{
						"height": 0,
						"result": jsonmap{
							"value": jsonmap{
								"account_number": "0",
								"sequence":       "0",
							},
						},
					},
				})
				_, err := rw.Write(response)
				c.Assert(err, IsNil)

			case strings.HasPrefix(req.RequestURI, mayaclient.AsgardVault):
				fixture("/endpoints/vaults/asgard.json")

			case strings.HasPrefix(req.RequestURI, "/mayachain/vaults") &&
				strings.HasSuffix(req.RequestURI, "/signers"):
				fixture("/endpoints/tss/keysign_party.json")

			case strings.HasPrefix(req.RequestURI, "/mayachain/vaults") &&
				strings.HasSuffix(req.RequestURI, "/pubkeys"):
				if common.CurrentChainNetwork == common.MainNet {
					fixture("/endpoints/vaults/pubKeys-Mainnet.json")
				} else {
					fixture("/endpoints/vaults/pubKeys.json")
				}

			default:
				c.Fatalf("No Thorchain response for uri '%s' has been mocked.", req.RequestURI)
			}
		}

		if req.RequestURI == "/" {
			mockDashNodeResponse()
		} else {
			mockThorchainResponse()
		}
	}
}
