package decimal

import (
	"testing"

	ret "github.com/radixdlt/radix-engine-toolkit-go/v2/radix_engine_toolkit_uniffi"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

func TestConvertUintToDecimal(t *testing.T) {
	testData := []string{
		"718476392820000000", "0.71847639282",
		"1000000000000000000", "1",
		"3138550867693340381917894711603833208051177722232017256447", "3138550867693340381917894711603833208051.177722232017256447",
		"1", "0.000000000000000001",
		"0", "0",
		"15283567607957252306", "15.283567607957252306",
	}

	for i := 0; i < len(testData); i += 2 {
		uintInputStr := testData[i]
		expectedDecimalStr := testData[i+1]
		cosmosUint := cosmos.NewUintFromString(uintInputStr)
		retDecimal, err := UintSubunitsToDecimal(cosmosUint)
		if err != nil {
			t.Error(err)
		}
		if retDecimal.AsStr() != expectedDecimalStr {
			t.Fatalf("%s is not equal to %s", retDecimal.AsStr(), expectedDecimalStr)
		}
	}
}

func TestDecimalOverflowError(t *testing.T) {
	cosmosUint := cosmos.NewUintFromString("3138550867693340381917894711603833208051177722232017256448")
	_, err := UintSubunitsToDecimal(cosmosUint)
	if err == nil {
		t.Fatalf("expected an error")
	}
}

func TestConvertDecimalToUint(t *testing.T) {
	testData := []string{
		"0.71847639282", "718476392820000000",
		"1", "1000000000000000000",
		"0", "0",
		"3138550867693340381917894711603833208051.177722232017256447", "3138550867693340381917894711603833208051177722232017256447",
		"0.000000000000000001", "1",
		"0.000000000000000000", "0",
		"15.283567607957252306", "15283567607957252306",
	}

	for i := 0; i < len(testData); i += 2 {
		decimalInputStr := testData[i]
		expectedUintStr := testData[i+1]
		retDecimal, err := ret.NewDecimal(decimalInputStr)
		if err != nil {
			t.Error(err)
		}
		cosmosUint, err := NonNegativeDecimalToUintSubunits(retDecimal)
		if err != nil {
			t.Error(err)
		}
		if cosmosUint.String() != expectedUintStr {
			t.Fatalf("%s is not equal to %s", cosmosUint.String(), expectedUintStr)
		}
	}
}

func TestNegativeDecimalToUintError(t *testing.T) {
	retDecimal, err := ret.NewDecimal("-1")
	if err != nil {
		t.Error(err)
	}
	_, err = NonNegativeDecimalToUintSubunits(retDecimal)
	if err == nil {
		t.Fatalf("expected an error")
	}
}
