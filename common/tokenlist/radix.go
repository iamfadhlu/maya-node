package tokenlist

import (
	"encoding/json"

	"gitlab.com/mayachain/mayanode/common/tokenlist/radixtokens"

	"github.com/blang/semver"
)

type RadixToken struct {
	Address  string `json:"address"`
	Symbol   string `json:"symbol"`
	Name     string `json:"name"`
	Decimals int32  `json:"decimals"`
}

var (
	radixTokenListV111 []RadixToken
	radixTokenListV112 []RadixToken
	radixTokenListV118 []RadixToken
)

func init() {
	if err := json.Unmarshal(radixtokens.RadixTokenListRawV111, &radixTokenListV111); err != nil {
		panic(err)
	}
	if err := json.Unmarshal(radixtokens.RadixTokenListRawV112, &radixTokenListV112); err != nil {
		panic(err)
	}
	if err := json.Unmarshal(radixtokens.RadixTokenListRawV118, &radixTokenListV118); err != nil {
		panic(err)
	}
}

func GetRadixTokenList(version semver.Version) []RadixToken {
	switch {
	case version.GTE(semver.MustParse("1.118.1")):
		return radixTokenListV118
	case version.GTE(semver.MustParse("1.112.0")):
		return radixTokenListV112
	default:
		return radixTokenListV111
	}
}
