//go:build testnet || mocknet
// +build testnet mocknet

package arbtokens

import _ "embed"

//go:embed arb_testnet_V109.json
var ARBTokenListRawV109 []byte

//go:embed arb_testnet_V110.json
var ARBTokenListRawV110 []byte

//go:embed arb_testnet_V111.json
var ARBTokenListRawV111 []byte

//go:embed arb_testnet_V112.json
var ARBTokenListRawV112 []byte

//go:embed arb_testnet_latest.json
var ARBTokenListRawV119 []byte
