package types

import (
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

var _ cosmos.Msg = &MsgForgiveSlash{}

// NewMsgForgiveSlash is a constructor function for NewMsgForgiveSlash.
func NewMsgForgiveSlash(amount cosmos.Uint, addr, signer cosmos.AccAddress) *MsgForgiveSlash {
	return &MsgForgiveSlash{
		Blocks:      amount,
		NodeAddress: addr,
		Signer:      signer,
	}
}

// Route should return the name of the module
func (m *MsgForgiveSlash) Route() string { return RouterKey }

// Type should return the action
func (m MsgForgiveSlash) Type() string { return "forgive_slash" }

// ValidateBasic runs stateless checks on the message
func (m *MsgForgiveSlash) ValidateBasic() error {
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	if m.Blocks == cosmos.ZeroUint() {
		return cosmos.ErrUnknownRequest("need block amount to slash to forgive_slash")
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgForgiveSlash) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners return all the signer who signed this message
func (m *MsgForgiveSlash) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
