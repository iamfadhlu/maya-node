package types

import (
	"errors"

	"github.com/cosmos/cosmos-sdk/codec"
)

var _ codec.ProtoMarshaler = &LiquidityAuctionTier{}

// LiquidityAuctionTiers a list of liquidity providers
type LiquidityAuctionTiers []LiquidityAuctionTier

// Valid check whether laTier represent valid information
func (m *LiquidityAuctionTier) Valid() error {
	if m.Address.IsEmpty() {
		return errors.New("the address should not be empty")
	}
	return nil
}

// Key return a string which can be used to identify liquidity auction tier
func (laTier LiquidityAuctionTier) Key() string {
	return laTier.GetAddress().String()
}
