package types

import (
	"github.com/blang/semver"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

var (
	_ cosmos.Msg = &MsgTradeAccountDeposit{}
	_ cosmos.Msg = &MsgTradeAccountWithdrawal{}
)

// NewMsgTradeAccountDeposit is a constructor function for MsgTradeAccountDeposit
func NewMsgTradeAccountDeposit(asset common.Asset, amount cosmos.Uint, acc, signer cosmos.AccAddress, tx common.Tx) *MsgTradeAccountDeposit {
	return &MsgTradeAccountDeposit{
		Tx:      tx,
		Asset:   asset,
		Amount:  amount,
		Address: acc,
		Signer:  signer,
	}
}

// Route should return the pooldata of the module
func (m *MsgTradeAccountDeposit) Route() string { return RouterKey }

// Type should return the action
func (m MsgTradeAccountDeposit) Type() string { return "set_trade_account_deposit" }

// ValidateBasic runs stateless checks on the message
func (m *MsgTradeAccountDeposit) ValidateBasic() error {
	if m.Asset.IsEmpty() {
		return cosmos.ErrUnknownRequest("asset cannot be empty")
	}
	if m.Asset.GetChain().IsBASEChain() {
		return cosmos.ErrUnknownRequest("asset cannot be mayaChain asset")
	}
	if m.Amount.IsZero() {
		return cosmos.ErrUnknownRequest("amount cannot be zero")
	}
	if m.Address.Empty() {
		return cosmos.ErrInvalidAddress(m.Address.String())
	}
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	if m.Tx.ID.IsEmpty() {
		return cosmos.ErrUnknownRequest("txID cannot be empty")
	}
	return nil
}

// GetSignBytes Implements Msg.
func (m *MsgTradeAccountDeposit) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgTradeAccountDeposit) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}

// NewMsgTradeAccountWithdrawal is a constructor function for MsgTradeAccountWithdrawal
func NewMsgTradeAccountWithdrawal(asset common.Asset, amount cosmos.Uint, addr common.Address, signer cosmos.AccAddress, tx common.Tx) *MsgTradeAccountWithdrawal {
	return &MsgTradeAccountWithdrawal{
		Asset:        asset,
		Amount:       amount,
		AssetAddress: addr,
		Signer:       signer,
		Tx:           tx,
	}
}

// Route should return the pooldata of the module
func (m *MsgTradeAccountWithdrawal) Route() string { return RouterKey }

// Type should return the action
func (m MsgTradeAccountWithdrawal) Type() string { return "set_trade_account_withdrawal" }

// ValidateBasic runs stateless checks on the message
func (m *MsgTradeAccountWithdrawal) ValidateBasic() error {
	if m.Asset.IsEmpty() {
		return cosmos.ErrUnknownRequest("asset cannot be empty")
	}
	if !m.Asset.IsTradeAsset() {
		return cosmos.ErrUnknownRequest("asset must be a trade asset")
	}
	if m.Amount.IsZero() {
		return cosmos.ErrUnknownRequest("amount cannot be zero")
	}
	if m.AssetAddress.IsEmpty() {
		return cosmos.ErrUnknownRequest("asset address cannot be empty")
	}
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	if m.Tx.ID.IsEmpty() {
		return cosmos.ErrUnknownRequest("txID cannot be empty")
	}
	return nil
}

// ValidateBasic runs stateless checks on the message
func (m *MsgTradeAccountWithdrawal) ValidateBasicVersioned(version semver.Version) error {
	if err := m.ValidateBasic(); err != nil {
		return err
	}
	if !m.AssetAddress.IsChain(m.Asset.GetLayer1Asset().GetChain(), version) {
		return cosmos.ErrUnknownRequest("asset address does not match asset chain")
	}
	return nil
}

// GetSignBytes Implements Msg.
func (m *MsgTradeAccountWithdrawal) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgTradeAccountWithdrawal) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
