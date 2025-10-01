package mayachain

import (
	"fmt"

	"github.com/blang/semver"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

// EmitEventItem define the method all event need to implement
type EmitEventItem interface {
	Events() (cosmos.Events, error)
}

// EventMgrVCUR implement EventManager interface
type EventMgrVCUR struct{}

// newEventMgrVCUR create a new instance of EventMgrVCUR
func newEventMgrVCUR() *EventMgrVCUR {
	return &EventMgrVCUR{}
}

// EmitEvent to block
func (m *EventMgrVCUR) EmitEvent(ctx cosmos.Context, evt EmitEventItem) error {
	events, err := evt.Events()
	if err != nil {
		return fmt.Errorf("fail to get events: %w", err)
	}
	ctx.EventManager().EmitEvents(events)
	return nil
}

// EmitGasEvent emit gas events
func (m *EventMgrVCUR) EmitGasEvent(ctx cosmos.Context, gasEvent *EventGas) error {
	if gasEvent == nil {
		return nil
	}
	return m.EmitEvent(ctx, gasEvent)
}

// EmitSwapEvent emit swap event to block
func (m *EventMgrVCUR) EmitSwapEvent(ctx cosmos.Context, swap *EventSwap) error {
	// OutTxs is a temporary field that we used, as for now we need to keep backward compatibility so the
	// events change doesn't break midgard and smoke test, for double swap , we first swap the source asset to RUNE ,
	// and then from RUNE to target asset, so the first will be marked as success
	if !swap.OutTxs.IsEmpty() {
		outboundEvt := NewEventOutbound(swap.InTx.ID, swap.OutTxs)
		if err := m.EmitEvent(ctx, outboundEvt); err != nil {
			return fmt.Errorf("fail to emit an outbound event for double swap: %w", err)
		}
	}
	return m.EmitEvent(ctx, swap)
}

// EmitFeeEvent emit a fee event through event manager
func (m *EventMgrVCUR) EmitFeeEvent(ctx cosmos.Context, feeEvent *EventFee) error {
	if feeEvent.Fee.Coins.IsEmpty() && feeEvent.Fee.PoolDeduct.IsZero() {
		return nil
	}
	events, err := feeEvent.Events()
	if err != nil {
		return fmt.Errorf("fail to emit fee event: %w", err)
	}
	ctx.EventManager().EmitEvents(events)
	return nil
}

// EmitBondEvent emit a bond event through event manager taking version into account
func (m *EventMgrVCUR) EmitBondEvent(ctx cosmos.Context, mgr Manager, asset common.Asset, amount cosmos.Uint, bondType types.BondType, txIn common.Tx) error {
	var bondEvent EmitEventItem
	version := mgr.GetVersion()

	switch {
	case version.GTE(semver.MustParse("1.105.0")):
		bondEvent = NewEventBondV105(asset, amount, bondType, txIn)
	default:
		bondEvent = NewEventBond(amount, bondType, txIn)
	}
	return m.EmitEvent(ctx, bondEvent)
}
