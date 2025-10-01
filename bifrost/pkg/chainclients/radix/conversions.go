package radix

import (
	"math/big"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

const (
	RadixDecimals = 18
)

var mayaToRadixDecimalSubunitsScalingFactor = cosmos.NewUint(
	new(big.Int).
		Exp(
			big.NewInt(10),
			big.NewInt(RadixDecimals-common.BASEChainDecimals), nil).Uint64())

// MayaSubunitsToRadixDecimalSubunits
// Returns the value converted from Maya subunits (10^8 subunits = 1 unit) to Radix Decimal (10^18 subunits = 1 unit).
func MayaSubunitsToRadixDecimalSubunits(value cosmos.Uint) cosmos.Uint {
	return value.Mul(mayaToRadixDecimalSubunitsScalingFactor)
}

// DecimalSubunitsToMayaRoundingDown
// Returns the value converted from Radix Decimal subunits (10^18 subunits = 1 unit) to Maya (10^8 subunits = 1 unit)
// rounded down, if needed. This is used for conversions that affect deposits
// from vault accounts, so that the protocol's books are always less or equal to the actual balance on-chain.
func DecimalSubunitsToMayaRoundingDown(value cosmos.Uint) cosmos.Uint {
	return decimalSubunitsToMaya(value, false)
}

// DecimalSubunitsToMayaRoundingUp
// Returns the value converted from Radix Decimal subunits (10^18 subunits = 1 unit) to Maya (10^8 subunits = 1 unit)
// rounded up, if needed. This is used for conversions that affect withdrawals
// from vault accounts (incl. fee payments), so that the protocol's books
// are always less or equal to the actual balance on-chain.
func DecimalSubunitsToMayaRoundingUp(value cosmos.Uint) cosmos.Uint {
	return decimalSubunitsToMaya(value, true)
}

func decimalSubunitsToMaya(value cosmos.Uint, roundingUp bool) cosmos.Uint {
	roundedDown := value.Quo(mayaToRadixDecimalSubunitsScalingFactor)
	if roundingUp && !value.Mod(mayaToRadixDecimalSubunitsScalingFactor).IsZero() {
		return roundedDown.AddUint64(1)
	} else {
		return roundedDown
	}
}
