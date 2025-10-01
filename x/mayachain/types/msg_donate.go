package types

import (
	"errors"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

// NewMsgDonate is a constructor function for MsgDonate
func NewMsgDonate(tx common.Tx, asset common.Asset, r, amount cosmos.Uint, signer cosmos.AccAddress) *MsgDonate {
	return &MsgDonate{
		Asset:       asset,
		AssetAmount: amount,
		CacaoAmount: r,
		Tx:          tx,
		Signer:      signer,
	}
}

// Route should return the route key of the module
func (m *MsgDonate) Route() string { return RouterKey }

// Type should return the action
func (m MsgDonate) Type() string { return "donate" }

// ValidateBasic runs stateless checks on the message
func (m *MsgDonate) ValidateBasic() error {
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	if m.Asset.IsEmpty() {
		return cosmos.ErrUnknownRequest("donate asset cannot be empty")
	}
	if m.Asset.IsBase() {
		return cosmos.ErrUnknownRequest("asset cannot be cacao")
	}
	if m.CacaoAmount.IsZero() && m.AssetAmount.IsZero() {
		return errors.New("rune and asset amount cannot be zero")
	}
	if err := m.Tx.Valid(); err != nil {
		return cosmos.ErrUnknownRequest(err.Error())
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgDonate) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgDonate) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
