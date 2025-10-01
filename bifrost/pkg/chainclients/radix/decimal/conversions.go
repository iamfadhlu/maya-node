package decimal

import (
	"fmt"
	"math/big"
	"slices"

	ret "github.com/radixdlt/radix-engine-toolkit-go/v2/radix_engine_toolkit_uniffi"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

const DecimalBytes = 24

var MaxDecimalSubunits = cosmos.NewUintFromString("3138550867693340381917894711603833208051177722232017256447")

func UintSubunitsToDecimal(uint cosmos.Uint) (*ret.Decimal, error) {
	if uint.GT(MaxDecimalSubunits) {
		return nil, fmt.Errorf("decimal overflow")
	}
	bytes := make([]byte, DecimalBytes)
	uintBytes := uint.BigInt().Bytes()
	slices.Reverse(uintBytes)
	copy(bytes, uintBytes)
	return ret.DecimalFromLeBytes(bytes), nil
}

func NonNegativeDecimalToUintSubunits(retDecimal *ret.Decimal) (cosmos.Uint, error) {
	if retDecimal.IsNegative() {
		return cosmos.Uint{}, fmt.Errorf("can't convert a negative Decimal to uint")
	}
	bytes := retDecimal.ToLeBytes()
	slices.Reverse(bytes)
	bigInt := big.Int{}
	bigInt.SetBytes(bytes)
	return cosmos.NewUintFromBigInt(&bigInt), nil
}
