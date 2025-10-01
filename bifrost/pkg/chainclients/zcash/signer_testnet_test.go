//go:build mocknet || testnet
// +build mocknet testnet

package zcash

import (
	"gitlab.com/mayachain/mayanode/chain/zec/go/zec"
	. "gopkg.in/check.v1"
)

func (s *ZcashSuite) TestAddressIsValid(c *C) {
	err := zec.ValidateAddress("tmP9jLgTnhDdKdWJCm4BT2t6acGnxqP14yU", s.client.getChainCfg().Net)
	c.Assert(err, IsNil)
	err = zec.ValidateAddress("t1R97mnhVqcE7Yq8p7yL4E29gy8etq9V9pG", s.client.getChainCfg().Net)
	c.Assert(err, NotNil)
}

func (s *ZcashSuite) TestGetHeight(c *C) {
	height, err := s.client.GetHeight()
	c.Assert(err, IsNil)
	c.Assert(height > 185 && height < 210, Equals, true)
}
