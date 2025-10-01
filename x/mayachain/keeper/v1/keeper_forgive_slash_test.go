package keeperv1

import (
	. "gopkg.in/check.v1"
)

type KeeperForgiveSlashSuite struct{}

var _ = Suite(&KeeperBanSuite{})

func (s *KeeperForgiveSlashSuite) TestForgiveSlashVoter(c *C) {
	ctx, k := setupKeeperForTest(c)
	addr := GetRandomBech32Addr()
	voter := NewForgiveSlashVoter(addr)
	k.SetForgiveSlashVoter(ctx, voter)
	voter, err := k.GetForgiveSlashVoter(ctx, addr)
	c.Assert(err, IsNil)
	c.Check(voter.NodeAddress.Equals(addr), Equals, true)

	voter1, err := k.GetForgiveSlashVoter(ctx, GetRandomBech32Addr())
	c.Check(err, IsNil)
	c.Check(voter1.IsEmpty(), Equals, false)
	iter := k.GetForgiveSlashVoterIterator(ctx)
	c.Check(iter, NotNil)
	iter.Close()
}
