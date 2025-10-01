//go:build testnet || mocknet
// +build testnet mocknet

package radixtokens

import (
	_ "embed"
)

//go:embed radix_testnet_V111.json
var RadixTokenListRawV111 []byte

//go:embed radix_testnet_V112.json
var RadixTokenListRawV112 []byte

//go:embed radix_testnet_latest.json
var RadixTokenListRawV118 []byte
