package types

import (
	. "gopkg.in/check.v1"

	cosmos "gitlab.com/mayachain/mayanode/common/cosmos"
)

type ProtocolOwnedLiquiditySuite struct{}

var _ = Suite(&ProtocolOwnedLiquiditySuite{})

func (s *ProtocolOwnedLiquiditySuite) TestCalcNodeRewards(c *C) {
	pol := NewProtocolOwnedLiquidity()
	c.Check(pol.CacaoDeposited.Uint64(), Equals, cosmos.ZeroUint().Uint64())
	c.Check(pol.CacaoWithdrawn.Uint64(), Equals, cosmos.ZeroUint().Uint64())
}

func (s *ProtocolOwnedLiquiditySuite) TestCurrentDeposit(c *C) {
	pol := NewProtocolOwnedLiquidity()
	pol.CacaoDeposited = cosmos.NewUint(100)
	pol.CacaoWithdrawn = cosmos.NewUint(25)
	c.Check(pol.CurrentDeposit().Int64(), Equals, int64(75))

	pol = NewProtocolOwnedLiquidity()
	pol.CacaoDeposited = cosmos.NewUint(25)
	pol.CacaoWithdrawn = cosmos.NewUint(100)
	c.Check(pol.CurrentDeposit().Int64(), Equals, int64(-75))
}

func (s *ProtocolOwnedLiquiditySuite) PnL(c *C) {
	pol := NewProtocolOwnedLiquidity()
	pol.CacaoDeposited = cosmos.NewUint(100)
	pol.CacaoWithdrawn = cosmos.NewUint(25)
	c.Check(pol.PnL(cosmos.NewUint(30)).Int64(), Equals, int64(-45))

	pol = NewProtocolOwnedLiquidity()
	pol.CacaoDeposited = cosmos.NewUint(25)
	pol.CacaoWithdrawn = cosmos.NewUint(10)
	c.Check(pol.PnL(cosmos.NewUint(30)).Int64(), Equals, int64(15))
}
