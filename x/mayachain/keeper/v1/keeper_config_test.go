package keeperv1

import (
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/constants"
)

type KeeperConfigSuite struct{}

var _ = Suite(&KeeperConfigSuite{})

func (s *KeeperConfigSuite) TestGetConfigInt64(c *C) {
	ctx, k := setupKeeperForTest(c)

	c.Assert(k.GetConfigInt64(ctx, constants.BlocksPerDay), Equals, int64(14400))

	k.SetMimir(ctx, constants.BlocksPerDay.String(), 1000)
	c.Assert(k.GetConfigInt64(ctx, constants.BlocksPerDay), Equals, int64(1000))

	k.SetMimir(ctx, constants.BlocksPerDay.String(), -1)
	c.Assert(k.GetConfigInt64(ctx, constants.BlocksPerDay), Equals, int64(14400))
}
