//go:build !testnet && !mocknet
// +build !testnet,!mocknet

package zcash

import (
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/chain/zec/go/zec"
)

func (s *ZcashSuite) TestAddressIsValidMainnet(c *C) {
	err := zec.ValidateAddress("tmP9jLgTnhDdKdWJCm4BT2t6acGnxqP14yU", s.client.getChainCfg().Net)
	c.Assert(err, NotNil)
	err = zec.ValidateAddress("t1R97mnhVqcE7Yq8p7yL4E29gy8etq9V9pG", s.client.getChainCfg().Net)
	c.Assert(err, IsNil)
}

func (s *ZcashSuite) TestGetHeight(c *C) {
	height, err := s.client.GetHeight()
	c.Assert(err, IsNil)
	c.Assert(height >= 2730000, Equals, true)
}
