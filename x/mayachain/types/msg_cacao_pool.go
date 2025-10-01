package types

import (
	"fmt"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

// NewMsgCacaoPoolDeposit create new MsgCacaoPoolDeposit message
func NewMsgCacaoPoolDeposit(signer cosmos.AccAddress, tx common.Tx) *MsgCacaoPoolDeposit {
	return &MsgCacaoPoolDeposit{
		Signer: signer,
		Tx:     tx,
	}
}

// Route should return the router key of the module
func (m *MsgCacaoPoolDeposit) Route() string { return RouterKey }

// Type should return the action
func (m MsgCacaoPoolDeposit) Type() string { return "cacao_pool_deposit" }

// ValidateBasic runs stateless checks on the message
func (m *MsgCacaoPoolDeposit) ValidateBasic() error {
	if !m.Tx.Chain.Equals(common.BASEChain) {
		return cosmos.ErrUnauthorized("chain must be MAYAChain")
	}
	if len(m.Tx.Coins) != 1 {
		return cosmos.ErrInvalidCoins("coins must be length 1 (CACAO)")
	}
	if !m.Tx.Coins[0].Asset.Chain.IsBASEChain() {
		return cosmos.ErrInvalidCoins("coin chain must be MAYAChain")
	}
	if !m.Tx.Coins[0].Asset.IsNativeBase() {
		return cosmos.ErrInvalidCoins("coin must be CACAO")
	}
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress("signer must not be empty")
	}
	if m.Tx.Coins[0].Amount.IsZero() {
		return cosmos.ErrUnknownRequest("coins amount must not be zero")
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgCacaoPoolDeposit) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgCacaoPoolDeposit) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}

// NewMsgCacaoPoolWithdraw create new MsgCacaoPoolWithdraw message
func NewMsgCacaoPoolWithdraw(signer cosmos.AccAddress, tx common.Tx, basisPoints cosmos.Uint, affiliateBps cosmos.Uint) *MsgCacaoPoolWithdraw {
	return &MsgCacaoPoolWithdraw{
		Signer:               signer,
		Tx:                   tx,
		BasisPoints:          basisPoints,
		AffiliateBasisPoints: affiliateBps,
	}
}

// Route should return the router key of the module
func (m *MsgCacaoPoolWithdraw) Route() string { return RouterKey }

// Type should return the action
func (m MsgCacaoPoolWithdraw) Type() string { return "cacao_pool_withdraw" }

// ValidateBasic runs stateless checks on the message
func (m *MsgCacaoPoolWithdraw) ValidateBasic() error {
	if !m.Tx.Coins.IsEmpty() {
		return cosmos.ErrInvalidCoins("coins must be empty (zero amount)")
	}
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress("signer must not be empty")
	}
	if m.BasisPoints.IsZero() || m.BasisPoints.GT(cosmos.NewUint(constants.MaxBasisPts)) {
		return cosmos.ErrUnknownRequest("invalid basis points")
	}
	if m.AffiliateBasisPoints.GT(cosmos.NewUint(constants.MaxBasisPts)) {
		return cosmos.ErrUnknownRequest(fmt.Sprintf("the total basis points for subaffiliates must not exceed %d", constants.MaxBasisPts))
	}

	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgCacaoPoolWithdraw) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgCacaoPoolWithdraw) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
