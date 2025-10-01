package types

import (
	"fmt"

	"github.com/blang/semver"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

// ValidateBasic runs stateless checks on the message
func (m *MsgAddLiquidity) ValidateBasic() error {
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	if m.Asset.IsEmpty() {
		return cosmos.ErrUnknownRequest("add liquidity asset cannot be empty")
	}
	if err := m.Tx.Valid(); err != nil {
		return cosmos.ErrUnknownRequest(err.Error())
	}
	// There is no dedicate pool for RUNE, because every pool will have RUNE, that's by design
	if m.Asset.IsBase() {
		return cosmos.ErrUnknownRequest("asset cannot be cacao")
	}
	// test scenario we get two coins, but none are rune, invalid liquidity provider
	if len(m.Tx.Coins) == 2 && (m.AssetAmount.IsZero() || m.CacaoAmount.IsZero()) {
		return cosmos.ErrUnknownRequest("did not find both coins")
	}
	if len(m.Tx.Coins) > 2 {
		return cosmos.ErrUnknownRequest("not expecting more than two coins in adding liquidity")
	}
	if m.CacaoAmount.IsZero() && m.AssetAmount.IsZero() {
		return cosmos.ErrUnknownRequest("rune and asset amounts cannot both be empty")
	}
	if m.CacaoAddress.IsEmpty() && m.AssetAddress.IsEmpty() {
		return cosmos.ErrUnknownRequest("rune address and asset address cannot be empty")
	}
	if m.AffiliateAddress.IsEmpty() && !m.AffiliateBasisPoints.IsZero() {
		return cosmos.ErrUnknownRequest("affiliate address is empty while affiliate basis points is non-zero")
	}
	if !m.AffiliateAddress.IsEmpty() && !m.AffiliateAddress.IsChain(common.BASEChain, semver.Version{}) {
		return cosmos.ErrUnknownRequest("affiliate address must be a THOR address")
	}
	return nil
}

func (m *MsgAddLiquidity) ValidateBasicV63() error {
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	if m.Asset.IsEmpty() {
		return cosmos.ErrUnknownRequest("add liquidity asset cannot be empty")
	}
	if err := m.Tx.Valid(); err != nil {
		return cosmos.ErrUnknownRequest(err.Error())
	}
	// There is no dedicate pool for RUNE, because every pool will have RUNE, that's by design
	if m.Asset.IsBase() {
		return cosmos.ErrUnknownRequest("asset cannot be cacao")
	}
	// test scenario we get two coins, but none are rune, invalid liquidity provider
	if len(m.Tx.Coins) == 2 && (m.AssetAmount.IsZero() || m.CacaoAmount.IsZero()) {
		return cosmos.ErrUnknownRequest("did not find both coins")
	}
	if len(m.Tx.Coins) > 2 {
		return cosmos.ErrUnknownRequest("not expecting more than two coins in adding liquidity")
	}
	if m.CacaoAmount.IsZero() && m.AssetAmount.IsZero() {
		return cosmos.ErrUnknownRequest("rune and asset amounts cannot both be empty")
	}
	if m.CacaoAddress.IsEmpty() && m.AssetAddress.IsEmpty() {
		return cosmos.ErrUnknownRequest("rune address and asset address cannot be empty")
	}
	if m.AffiliateAddress.IsEmpty() && !m.AffiliateBasisPoints.IsZero() {
		return cosmos.ErrUnknownRequest("affiliate address is empty while affiliate basis points is non-zero")
	}
	if !m.AffiliateBasisPoints.IsZero() && m.AffiliateBasisPoints.GT(cosmos.NewUint(LegacyMaxAffiliateFeeBasisPoints)) {
		return cosmos.ErrUnknownRequest(fmt.Sprintf("affiliate fee basis points can't be more than %d", LegacyMaxAffiliateFeeBasisPoints))
	}
	if !m.AffiliateAddress.IsEmpty() && !m.AffiliateAddress.IsChain(common.BASEChain, semver.Version{}) {
		return cosmos.ErrUnknownRequest("affiliate address must be a THOR address")
	}
	return nil
}

// ValidateBasicV108 runs stateless checks on the message
func (m *MsgAddLiquidity) ValidateBasicV108() error {
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	if m.Asset.IsEmpty() {
		return cosmos.ErrUnknownRequest("add liquidity asset cannot be empty")
	}
	if err := m.Tx.Valid(); err != nil {
		return cosmos.ErrUnknownRequest(err.Error())
	}
	// There is no dedicate pool for RUNE, because every pool will have RUNE, that's by design
	if m.Asset.IsBase() {
		return cosmos.ErrUnknownRequest("asset cannot be cacao")
	}
	// test scenario we get two coins, but none are rune, invalid liquidity provider
	if len(m.Tx.Coins) == 2 && (m.AssetAmount.IsZero() || m.CacaoAmount.IsZero()) {
		return cosmos.ErrUnknownRequest("did not find both coins")
	}
	if len(m.Tx.Coins) > 2 {
		return cosmos.ErrUnknownRequest("not expecting more than two coins in adding liquidity")
	}
	if m.CacaoAddress.IsEmpty() && m.AssetAddress.IsEmpty() {
		return cosmos.ErrUnknownRequest("rune address and asset address cannot be empty")
	}
	if m.AffiliateAddress.IsEmpty() && !m.AffiliateBasisPoints.IsZero() {
		return cosmos.ErrUnknownRequest("affiliate address is empty while affiliate basis points is non-zero")
	}
	if !m.AffiliateBasisPoints.IsZero() && m.AffiliateBasisPoints.GT(cosmos.NewUint(LegacyMaxAffiliateFeeBasisPoints)) {
		return cosmos.ErrUnknownRequest(fmt.Sprintf("affiliate fee basis points can't be more than %d", LegacyMaxAffiliateFeeBasisPoints))
	}
	return nil
}
