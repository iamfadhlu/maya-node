//go:build !testnet && !mocknet
// +build !testnet,!mocknet

package dash

import (
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
)

func (s *DashSuite) TestGetAddress(c *C) {
	pubkey := common.PubKey("mayapub1addwnpepqt7qug8vk9r3saw8n4r803ydj2g3dqwx0mvq5akhnze86fc536xcyvg4c5e")
	addr := s.client.GetAddress(pubkey)
	c.Assert(addr, Equals, "XiBCCabtPW9w8EfADdZJZiAmYYaNaZQDsf")
}
