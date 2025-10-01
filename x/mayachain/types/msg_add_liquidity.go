package types

import (
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

var _ cosmos.Msg = &MsgAddLiquidity{}

// NewMsgAddLiquidity is a constructor function for MsgAddLiquidity
func NewMsgAddLiquidity(tx common.Tx, asset common.Asset, baseAmount, assetAmount cosmos.Uint, runeAddr, assetAddr, affAddr common.Address, affPts cosmos.Uint, signer cosmos.AccAddress, tier int64) *MsgAddLiquidity {
	return &MsgAddLiquidity{
		Tx:                   tx,
		Asset:                asset,
		AssetAmount:          assetAmount,
		CacaoAmount:          baseAmount,
		CacaoAddress:         runeAddr,
		AssetAddress:         assetAddr,
		AffiliateAddress:     affAddr,
		AffiliateBasisPoints: affPts,
		Signer:               signer,
		LiquidityAuctionTier: tier,
	}
}

// Route should return the route key of the module
func (m *MsgAddLiquidity) Route() string { return RouterKey }

// Type should return the action
func (m MsgAddLiquidity) Type() string { return "add_liquidity" }

// ValidateBasicV112 runs stateless checks on the message
func (m *MsgAddLiquidity) ValidateBasicV112() error {
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
	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgAddLiquidity) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgAddLiquidity) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
