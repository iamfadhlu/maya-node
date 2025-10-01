//go:build !testnet && !mocknet
// +build !testnet,!mocknet

package dash

import (
	"gitlab.com/mayachain/dashd-go/chaincfg"
	. "gopkg.in/check.v1"
)

func (s *DashSignerSuite) TestGetChainCfg(c *C) {
	param := s.client.getChainCfg()
	c.Assert(param, Equals, &chaincfg.MainNetParams)
}
