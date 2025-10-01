package tokenlist

import (
	"encoding/json"

	"github.com/blang/semver"
	"gitlab.com/mayachain/mayanode/common/tokenlist/ethtokens"
)

var (
	ethTokenListV93  EVMTokenList
	ethTokenListV95  EVMTokenList
	ethTokenListV106 EVMTokenList
	ethTokenListV109 EVMTokenList
	ethTokenListV110 EVMTokenList
	ethTokenListV112 EVMTokenList
	ethTokenListV119 EVMTokenList
)

func init() {
	if err := json.Unmarshal(ethtokens.ETHTokenListRawV93, &ethTokenListV93); err != nil {
		panic(err)
	}
	if err := json.Unmarshal(ethtokens.ETHTokenListRawV95, &ethTokenListV95); err != nil {
		panic(err)
	}
	if err := json.Unmarshal(ethtokens.ETHTokenListRawV106, &ethTokenListV106); err != nil {
		panic(err)
	}
	if err := json.Unmarshal(ethtokens.ETHTokenListRawV109, &ethTokenListV109); err != nil {
		panic(err)
	}
	if err := json.Unmarshal(ethtokens.ETHTokenListRawV110, &ethTokenListV110); err != nil {
		panic(err)
	}
	if err := json.Unmarshal(ethtokens.ETHTokenListRawV112, &ethTokenListV112); err != nil {
		panic(err)
	}
	if err := json.Unmarshal(ethtokens.ETHTokenListRawV119, &ethTokenListV119); err != nil {
		panic(err)
	}
}

func GetETHTokenList(version semver.Version) EVMTokenList {
	switch {
	case version.GTE(semver.MustParse("1.119.0")):
		return ethTokenListV119
	case version.GTE(semver.MustParse("1.112.0")):
		return ethTokenListV112
	case version.GTE(semver.MustParse("1.110.0")):
		return ethTokenListV110
	case version.GTE(semver.MustParse("1.109.0")):
		return ethTokenListV109
	case version.GTE(semver.MustParse("1.106.0")):
		return ethTokenListV106
	case version.GTE(semver.MustParse("1.95.0")):
		return ethTokenListV95
	case version.GTE(semver.MustParse("1.93.0")):
		return ethTokenListV93
	default:
		return ethTokenListV93
	}
}
