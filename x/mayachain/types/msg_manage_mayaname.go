package types

import (
	"fmt"
	"regexp"

	"github.com/blang/semver"
	"gitlab.com/mayachain/mayanode/common"
	cosmos "gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

var EmptyBps = cosmos.NewUint(5_01) // must be greater than the maximal allowed affiliate basis points

// should be the same as in handler_manage_mayaname.go
// TODO: move to a common package
var IsValidMAYANameV1 = regexp.MustCompile(`^[a-zA-Z0-9+_-]+$`).MatchString

// NewMsgManageMAYAName create a new instance of MsgManageMAYAName
func NewMsgManageMAYAName(name string, chain common.Chain, addr common.Address, coin common.Coin, exp int64, preferredAsset common.Asset, affiliateBps cosmos.Uint, subaffiliateBps []cosmos.Uint, subaffiliateName []string, owner, signer cosmos.AccAddress) *MsgManageMAYAName {
	return &MsgManageMAYAName{
		Name:              name,
		Chain:             chain,
		Address:           addr,
		Coin:              coin,
		ExpireBlockHeight: exp,
		PreferredAsset:    preferredAsset,
		AffiliateBps:      affiliateBps,
		SubaffiliateName:  subaffiliateName,
		SubaffiliateBps:   subaffiliateBps,
		Owner:             owner,
		Signer:            signer,
	}
}

// Route should return the Route of the module
func (m *MsgManageMAYAName) Route() string { return RouterKey }

// Type should return the action
func (m MsgManageMAYAName) Type() string { return "manage_mayaname" }

// ValidateBasicV112 runs stateless checks on the message
func (m *MsgManageMAYAName) ValidateBasicV112(version semver.Version) error {
	// validate Basic
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}

	if len(m.Name) > 30 {
		return fmt.Errorf("MAYAName cannot exceed 30 characters")
	}

	if !IsValidMAYANameV1(m.Name) {
		return fmt.Errorf("invalid MAYAName")
	}

	// chain and address are optional, but if one is provided, the other must be provided as well
	if !m.Chain.IsEmpty() || !m.Address.IsEmpty() {
		if m.Chain.IsEmpty() {
			return cosmos.ErrUnknownRequest("chain can't be empty: chain and address must be provided together")
		}
		if m.Address.IsEmpty() {
			return cosmos.ErrUnknownRequest("address can't be empty: chain and address must be provided together")
		}
		if !m.Address.IsChain(m.Chain, version) {
			return cosmos.ErrUnknownRequest("address and chain must match")
		}
	}
	if !m.Coin.Asset.IsNativeBase() {
		return cosmos.ErrUnknownRequest("coin must be native cacao")
	}

	// MAYANAme must not begin with TIER due to possible conflict with tiers in AddLiquidity memo in liquidity auctions
	if len(m.Name) >= 4 && m.Name[:4] == "TIER" {
		return cosmos.ErrUnknownRequest("invalid MAYAName")
	}

	// verify subaffiliate and bps counts
	if len(m.SubaffiliateName) != len(m.SubaffiliateBps) && len(m.SubaffiliateBps) != 1 {
		return fmt.Errorf("subaffiliate mayanames and subaffiliate fee bps count mismatch (%d / %d)", len(m.SubaffiliateName), len(m.SubaffiliateBps))
	}

	for _, subaffiliateName := range m.SubaffiliateName {
		// subaffiliate name must be different than the main mayaname
		if subaffiliateName == m.Name {
			return cosmos.ErrUnknownRequest("subaffiliate MAYAName must be different than the main affiliate")
		}
	}

	for _, subaffiliateBps := range m.SubaffiliateBps {
		if subaffiliateBps.GT(cosmos.NewUint(constants.MaxBasisPts)) {
			return cosmos.ErrUnknownRequest(fmt.Sprintf("subaffiliate fee basis points must not exceed %d", constants.MaxBasisPts))
		}
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgManageMAYAName) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgManageMAYAName) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
