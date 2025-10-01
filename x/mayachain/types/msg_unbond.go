package types

import (
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

// NewMsgUnBond create new MsgUnBond message
func NewMsgUnBond(
	txin common.Tx,
	nodeAddr cosmos.AccAddress,
	bondAddress common.Address,
	provider,
	signer cosmos.AccAddress,
	asset common.Asset,
	units cosmos.Uint,
) *MsgUnBond {
	return &MsgUnBond{
		TxIn:                txin,
		NodeAddress:         nodeAddr,
		BondAddress:         bondAddress,
		BondProviderAddress: provider,
		Signer:              signer,
		Asset:               asset,
		Units:               units,
	}
}

// Route should return the router key of the module
func (m *MsgUnBond) Route() string { return RouterKey }

// Type should return the action
func (m MsgUnBond) Type() string { return "unbond" }

// ValidateBasic runs stateless checks on the message
func (m *MsgUnBond) ValidateBasic() error {
	if m.NodeAddress.Empty() {
		return cosmos.ErrInvalidAddress("node address cannot be empty")
	}
	if m.BondAddress.IsEmpty() {
		return cosmos.ErrInvalidAddress("bond address cannot be empty")
	}
	// here we can't call m.TxIn.Valid , because we allow user to send unbond request without any coins in it
	// m.TxIn.Valid will reject this kind request , which result unbond to fail
	if m.TxIn.ID.IsEmpty() {
		return cosmos.ErrUnknownRequest("tx id cannot be empty")
	}
	if m.TxIn.FromAddress.IsEmpty() {
		return cosmos.ErrInvalidAddress("tx from address cannot be empty")
	}
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress("empty signer address")
	}
	if !m.BondProviderAddress.Empty() && !m.Asset.IsEmpty() {
		return cosmos.ErrUnknownRequest("bond provider address and asset cannot be set at the same time")
	}
	if m.Asset.IsEmpty() && !m.Units.IsZero() {
		return cosmos.ErrUnknownRequest("asset cannot be empty when units is not zero")
	}

	return nil
}

func (m *MsgUnBond) ValidateBasicV88() error {
	if m.NodeAddress.Empty() {
		return cosmos.ErrInvalidAddress("node address cannot be empty")
	}
	if m.BondAddress.IsEmpty() {
		return cosmos.ErrInvalidAddress("bond address cannot be empty")
	}
	// here we can't call m.TxIn.Valid , because we allow user to send unbond request without any coins in it
	// m.TxIn.Valid will reject this kind request , which result unbond to fail
	if m.TxIn.ID.IsEmpty() {
		return cosmos.ErrUnknownRequest("tx id cannot be empty")
	}
	if m.TxIn.FromAddress.IsEmpty() {
		return cosmos.ErrInvalidAddress("tx from address cannot be empty")
	}
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress("empty signer address")
	}
	if !m.BondProviderAddress.Empty() && !m.Asset.IsEmpty() {
		return cosmos.ErrUnknownRequest("bond provider address and asset cannot be set at the same time")
	}
	if m.Asset.IsEmpty() && !m.Units.IsZero() {
		return cosmos.ErrUnknownRequest("asset cannot be empty when units is not zero")
	}

	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgUnBond) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgUnBond) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
