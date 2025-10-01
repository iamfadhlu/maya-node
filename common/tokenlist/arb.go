package tokenlist

import (
	"encoding/json"

	"github.com/blang/semver"
	"gitlab.com/mayachain/mayanode/common/tokenlist/arbtokens"
)

var (
	arbTokenListV109 EVMTokenList
	arbTokenListV110 EVMTokenList
	arbTokenListV111 EVMTokenList
	arbTokenListV112 EVMTokenList
	arbTokenListV119 EVMTokenList
)

func init() {
	if err := json.Unmarshal(arbtokens.ARBTokenListRawV109, &arbTokenListV109); err != nil {
		panic(err)
	}
	if err := json.Unmarshal(arbtokens.ARBTokenListRawV110, &arbTokenListV110); err != nil {
		panic(err)
	}
	if err := json.Unmarshal(arbtokens.ARBTokenListRawV111, &arbTokenListV111); err != nil {
		panic(err)
	}
	if err := json.Unmarshal(arbtokens.ARBTokenListRawV112, &arbTokenListV112); err != nil {
		panic(err)
	}
	if err := json.Unmarshal(arbtokens.ARBTokenListRawV119, &arbTokenListV119); err != nil {
		panic(err)
	}
}

func GetARBTokenList(version semver.Version) EVMTokenList {
	switch {
	case version.GTE(semver.MustParse("1.119.0")):
		return arbTokenListV119
	case version.GTE(semver.MustParse("1.112.0")):
		return arbTokenListV112
	case version.GTE(semver.MustParse("1.111.0")):
		return arbTokenListV111
	case version.GTE(semver.MustParse("1.110.0")):
		return arbTokenListV110
	default:
		return arbTokenListV109
	}
}
