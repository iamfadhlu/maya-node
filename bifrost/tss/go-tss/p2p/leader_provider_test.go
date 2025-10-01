package p2p

import (
	"testing"

	. "gopkg.in/check.v1"
)

func TestPackage(t *testing.T) { TestingT(t) }

type LeaderProviderTestSuite struct{}

var _ = Suite(&LeaderProviderTestSuite{})

func (t *LeaderProviderTestSuite) TestLeaderNode(c *C) {
	testPeers := []string{
		"16Uiu2HAmACG5DtqmQsHtXg4G2sLS65ttv84e7MrL4kapkjfmhxAp", "16Uiu2HAm4TmEzUqy3q3Dv7HvdoSboHk5sFj2FH3npiN5vDbJC6gh",
		"16Uiu2HAm2FzqoUdS6Y9Esg2EaGcAG5rVe1r6BFNnmmQr2H3bqafa",
	}
	ret, err := LeaderNode("HelloWorld", 10, testPeers)
	c.Assert(err, IsNil)
	c.Assert(ret, Equals, testPeers[1])
}

func (t *LeaderProviderTestSuite) TestLeaderNodeEdgeCases(c *C) {
	// Test empty peer list
	_, err := LeaderNode("test", 10, []string{})
	c.Assert(err, NotNil)

	// Test empty message ID
	testPeers := []string{"peer1", "peer2"}
	_, err = LeaderNode("", 10, testPeers)
	c.Assert(err, NotNil)

	// Test zero block height
	_, err = LeaderNode("test", 0, testPeers)
	c.Assert(err, NotNil)

	// Test single peer
	ret, err := LeaderNode("test", 10, []string{"peer1"})
	c.Assert(err, IsNil)
	c.Assert(ret, Equals, "peer1")
}
