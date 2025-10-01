package types

import (
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

// NewMsgBond create new MsgBond message
func NewMsgBond(
	txin common.Tx,
	nodeAddr cosmos.AccAddress,
	amount cosmos.Uint,
	bondAddress common.Address,
	provider, signer cosmos.AccAddress,
	operatorFee int64,
	asset common.Asset,
	units cosmos.Uint,
) *MsgBond {
	return &MsgBond{
		TxIn:                txin,
		NodeAddress:         nodeAddr,
		Amount:              amount,
		BondAddress:         bondAddress,
		BondProviderAddress: provider,
		Signer:              signer,
		OperatorFee:         operatorFee,
		Asset:               asset,
		Units:               units,
	}
}

// Route should return the router key of the module
func (m *MsgBond) Route() string { return RouterKey }

// Type should return the action
func (m MsgBond) Type() string { return "bond" }

// ValidateBasic runs stateless checks on the message
func (m *MsgBond) ValidateBasic() error {
	if m.NodeAddress.Empty() {
		return cosmos.ErrInvalidAddress("node address cannot be empty")
	}
	if m.Amount.IsZero() {
		return cosmos.ErrUnknownRequest("amount cannot be zero")
	}
	if m.BondAddress.IsEmpty() {
		return cosmos.ErrInvalidAddress("bond address cannot be empty")
	}
	if err := m.TxIn.Valid(); err != nil {
		return cosmos.ErrUnknownRequest(err.Error())
	}
	if len(m.TxIn.Coins) > 1 {
		return cosmos.ErrUnknownRequest("cannot bond more than one coin")
	}
	if len(m.TxIn.Coins) == 0 {
		return cosmos.ErrUnknownRequest("must send bond msg with at least one rune")
	}
	if !m.TxIn.Coins[0].Asset.IsNativeBase() {
		return cosmos.ErrUnknownRequest("cannot bond non-native rune asset")
	}
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress("empty signer address")
	}
	if m.OperatorFee < -1 || m.OperatorFee > 10000 {
		return cosmos.ErrUnknownRequest("operator fee must be 0-10000")
	}
	if m.Asset.IsEmpty() && !m.Units.IsZero() {
		return cosmos.ErrUnknownRequest("cannot set units without asset")
	}

	return nil
}

// ValidateBasicV96 runs stateless checks on the message
func (m *MsgBond) ValidateBasicV96() error {
	if m.NodeAddress.Empty() {
		return cosmos.ErrInvalidAddress("node address cannot be empty")
	}
	if m.Amount.IsZero() {
		return cosmos.ErrUnknownRequest("amount cannot be zero")
	}
	if m.BondAddress.IsEmpty() {
		return cosmos.ErrInvalidAddress("bond address cannot be empty")
	}
	if err := m.TxIn.Valid(); err != nil {
		return cosmos.ErrUnknownRequest(err.Error())
	}
	if len(m.TxIn.Coins) > 1 {
		return cosmos.ErrUnknownRequest("cannot bond more than one coin")
	}
	if len(m.TxIn.Coins) == 0 {
		return cosmos.ErrUnknownRequest("must send bond msg with at least one rune")
	}
	if !m.TxIn.Coins[0].Asset.IsNativeBase() {
		return cosmos.ErrUnknownRequest("cannot bond non-native rune asset")
	}
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress("empty signer address")
	}
	if m.OperatorFee < -1 || m.OperatorFee > 10000 {
		return cosmos.ErrUnknownRequest("operator fee must be 0-10000")
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgBond) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgBond) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
