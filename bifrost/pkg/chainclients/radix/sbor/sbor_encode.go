package sbor

import (
	"bytes"
	"encoding/hex"
	"fmt"

	ret "github.com/radixdlt/radix-engine-toolkit-go/v2/radix_engine_toolkit_uniffi"
)

func EncodeAddressToManifestSborHex(value string) (string, error) {
	retAddress, err := ret.NewAddress(value)
	if err != nil {
		return "", fmt.Errorf("could not create RET address: %v", err)
	}
	manifestSborBytes := bytes.Join([][]byte{{0x4d, 0x80, 0x00}, retAddress.Bytes()}, []byte{})
	return hex.EncodeToString(manifestSborBytes), nil
}
