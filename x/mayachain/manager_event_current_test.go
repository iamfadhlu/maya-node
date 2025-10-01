package mayachain

import (
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

type EventManagerTestSuite struct{}

var _ = Suite(&EventManagerTestSuite{})

func (s *EventManagerTestSuite) TestEmitPoolEvent(c *C) {
	ctx, _ := setupKeeperForTest(c)
	eventMgr := newEventMgrVCUR()
	c.Assert(eventMgr, NotNil)
	ctx = ctx.WithBlockHeight(1024)
	c.Assert(eventMgr.EmitEvent(ctx, NewEventPool(common.BNBAsset, PoolAvailable)), IsNil)
}

func (s *EventManagerTestSuite) TestEmitErrataEvent(c *C) {
	ctx, _ := setupKeeperForTest(c)
	eventMgr := newEventMgrVCUR()
	c.Assert(eventMgr, NotNil)
	ctx = ctx.WithBlockHeight(1024)
	errataEvent := NewEventErrata(GetRandomTxHash(), PoolMods{
		PoolMod{
			Asset:    common.BNBAsset,
			CacaoAmt: cosmos.ZeroUint(),
			CacaoAdd: false,
			AssetAmt: cosmos.NewUint(100),
			AssetAdd: true,
		},
	})
	c.Assert(eventMgr.EmitEvent(ctx, errataEvent), IsNil)
}

func (s *EventManagerTestSuite) TestEmitGasEvent(c *C) {
	ctx, _ := setupKeeperForTest(c)
	eventMgr := newEventMgrVCUR()
	c.Assert(eventMgr, NotNil)
	ctx = ctx.WithBlockHeight(1024)
	gasEvent := NewEventGas()
	gasEvent.Pools = append(gasEvent.Pools, GasPool{
		Asset:    common.BNBAsset,
		AssetAmt: cosmos.ZeroUint(),
		CacaoAmt: cosmos.NewUint(1024),
		Count:    1,
	})
	c.Assert(eventMgr.EmitGasEvent(ctx, gasEvent), IsNil)
	c.Assert(eventMgr.EmitGasEvent(ctx, nil), IsNil)
}
