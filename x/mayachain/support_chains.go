package mayachain

import (
	"github.com/blang/semver"
	"gitlab.com/mayachain/mayanode/common"
)

func GetSupportChains(version semver.Version) common.Chains {
	switch {
	case version.GTE(semver.MustParse("1.120.0")): // zcash
		return SUPPORT_CHAINS_V120
	case version.GTE(semver.MustParse("1.111.0")):
		return SUPPORT_CHAINS_V111
	case version.GTE(semver.MustParse("1.109.0")):
		return SUPPORT_CHAINS_V109
	case version.GTE(semver.MustParse("1.107.0")):
		return SUPPORT_CHAINS_V107
	case version.GTE(semver.MustParse("1.105.0")):
		return SUPPORT_CHAINS_V105
	default:
		return SUPPORT_CHAINS_V104
	}
}
