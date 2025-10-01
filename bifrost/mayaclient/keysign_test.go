package mayaclient

import (
	"net/http"
	"net/http/httptest"
	"strings"

	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/config"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

type KeysignSuite struct {
	server  *httptest.Server
	bridge  *mayachainBridge
	cfg     config.BifrostClientConfiguration
	fixture string
}

var _ = Suite(&KeysignSuite{})

func (s *KeysignSuite) SetUpSuite(c *C) {
	s.server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if strings.HasPrefix(req.RequestURI, KeysignEndpoint) {
			httpTestHandler(c, rw, s.fixture)
		}
	}))

	cfg, _, kb := SetupMayachainForTest(c)
	s.cfg = cfg
	s.cfg.ChainHost = s.server.Listener.Addr().String()
	var err error
	bridge, err := NewMayachainBridge(s.cfg, GetMetricForTest(c), NewKeysWithKeybase(kb, cfg.SignerName, cfg.SignerPasswd))
	var ok bool
	s.bridge, ok = bridge.(*mayachainBridge)
	c.Assert(ok, Equals, true)
	s.bridge.httpClient.RetryMax = 1
	c.Assert(err, IsNil)
	c.Assert(s.bridge, NotNil)
}

func (s *KeysignSuite) TearDownSuite(c *C) {
	s.server.Close()
}

func (s *KeysignSuite) TestGetKeysign(c *C) {
	// GENERATE SIGNATURE
	//	txOut := &types.TxOut{
	//		Height: 1718,
	//		TxArray: []types.TxOutItem{
	//			{
	//				Chain: "BNB",
	//				Coin: common.Coin{
	//					Amount: cosmos.NewUint(10000000000),
	//					Asset:  common.BNBAsset,
	//				},
	//				InHash:      "ENULZOBGZHEKFOIBYRLLBELKFZVGXOBLTRQGTOWNDHMPZQMBLGJETOXJLHPVQIKY",
	//				ToAddress:   "tbnb145wcuncewfkuc4v6an0r9laswejygcul43c3wu",
	//				VaultPubKey: "tmayapub1addwnpepqfgpnhk4z80fglgp4cget35dsdzpfxdkudtzw09pm2t0wv6rvhds200t3jf",
	//				GasRate:     1,
	//			},
	//		},
	//	}
	//	buf, err := json.Marshal(txOut)
	//	if err != nil {
	//		fmt.Print(err)
	//		return
	//	}
	//	sig, _, err := kb.Sign("mayachain", buf)
	//	b := base64.StdEncoding.EncodeToString(sig)
	//	fmt.Println(b)
	s.fixture = "../../test/fixtures/endpoints/keysign/template.json"
	pk := types.GetRandomPubKey()
	keysign, err := s.bridge.GetKeysign(1718, pk.String())
	c.Assert(err, IsNil)
	c.Assert(keysign, NotNil)
	c.Assert(keysign.Height, Equals, int64(1718))
	c.Assert(keysign.TxArray[0].Chain, Equals, common.BNBChain)
}
