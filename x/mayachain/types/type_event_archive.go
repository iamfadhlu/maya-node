package types

import (
	"fmt"
	"strings"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

// NewEventBond create a new Bond Events
func NewEventBondV105(asset common.Asset, amount cosmos.Uint, bondType BondType, txIn common.Tx) *EventBondV105 {
	return &EventBondV105{
		Amount:   amount,
		BondType: bondType,
		TxIn:     txIn,
		Asset:    asset,
	}
}

// Type return bond event Type
func (m *EventBondV105) Type() string {
	return BondEventType
}

// Events return all the event attributes
func (m *EventBondV105) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("amount", m.Amount.String()),
		cosmos.NewAttribute("bond_type", string(m.BondType)))
	if !m.Asset.IsEmpty() {
		evt = evt.AppendAttributes(cosmos.NewAttribute("asset", m.Asset.String()))
	}
	evt = evt.AppendAttributes(m.TxIn.ToAttributes()...)
	return cosmos.Events{evt}, nil
}

// NewEventMAYANameV111 create a new instance of EventMAYANameV111
func NewEventMAYANameV111(name string, chain common.Chain, addr common.Address, reg_fee, fund_amt cosmos.Uint, expire int64, owner cosmos.AccAddress) *EventMAYANameV111 {
	return &EventMAYANameV111{
		Name:            name,
		Chain:           chain,
		Address:         addr,
		RegistrationFee: reg_fee,
		FundAmt:         fund_amt,
		Expire:          expire,
		Owner:           owner,
	}
}

// Type return a string which represent the type of this event
func (m *EventMAYANameV111) Type() string {
	return MAYANameEventType
}

// Events return cosmos sdk events
func (m *EventMAYANameV111) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("name", strings.ToLower(m.Name)),
		cosmos.NewAttribute("chain", m.Chain.String()),
		cosmos.NewAttribute("address", m.Address.String()),
		cosmos.NewAttribute("registration_fee", m.RegistrationFee.String()),
		cosmos.NewAttribute("fund_amount", m.FundAmt.String()),
		cosmos.NewAttribute("expire", fmt.Sprintf("%d", m.Expire)),
		cosmos.NewAttribute("owner", m.Owner.String()))
	return cosmos.Events{evt}, nil
}

// NewEventSwitchV87 create a new instance of EventSwitch
func NewEventSwitchV87(from common.Address, to cosmos.AccAddress, coin common.Coin, hash common.TxID, mint cosmos.Uint) *EventSwitchV87 {
	return &EventSwitchV87{
		TxID:        hash,
		ToAddress:   to,
		FromAddress: from,
		Burn:        coin,
		Mint:        mint,
	}
}

// Type return a string which represent the type of this event
func (m *EventSwitchV87) Type() string {
	return SwitchEventType
}

// Events return cosmos sdk events
func (m *EventSwitchV87) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("txid", m.TxID.String()),
		cosmos.NewAttribute("from", m.FromAddress.String()),
		cosmos.NewAttribute("to", m.ToAddress.String()),
		cosmos.NewAttribute("burn", m.Burn.String()),
		cosmos.NewAttribute("mint", m.Mint.String()))
	return cosmos.Events{evt}, nil
}
