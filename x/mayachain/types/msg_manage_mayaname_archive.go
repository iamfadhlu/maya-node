package types

import (
	"github.com/blang/semver"
	cosmos "gitlab.com/mayachain/mayanode/common/cosmos"
)

// ValidateBasic runs stateless checks on the message
func (m *MsgManageMAYAName) ValidateBasic() error {
	// validate n
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	if m.Chain.IsEmpty() {
		return cosmos.ErrUnknownRequest("chain can't be empty")
	}
	if m.Address.IsEmpty() {
		return cosmos.ErrUnknownRequest("address can't be empty")
	}
	if !m.Address.IsChain(m.Chain, semver.Version{}) {
		return cosmos.ErrUnknownRequest("address and chain must match")
	}
	if !m.Coin.Asset.IsNativeBase() {
		return cosmos.ErrUnknownRequest("coin must be native rune")
	}
	return nil
}

// ValidateBasicV108 runs stateless checks on the message
func (m *MsgManageMAYAName) ValidateBasicV108(version semver.Version) error {
	// validate n
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	if m.Chain.IsEmpty() {
		return cosmos.ErrUnknownRequest("chain can't be empty")
	}
	if m.Address.IsEmpty() {
		return cosmos.ErrUnknownRequest("address can't be empty")
	}
	if !m.Address.IsChain(m.Chain, version) {
		return cosmos.ErrUnknownRequest("address and chain must match")
	}
	if !m.Coin.Asset.IsNativeBase() {
		return cosmos.ErrUnknownRequest("coin must be native rune")
	}
	return nil
}
