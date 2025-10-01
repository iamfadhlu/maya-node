package types

import (
	"github.com/blang/semver"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/liquiditypools"
)

func GetLiquidityPools(version semver.Version) common.Assets {
	switch {
	case version.GTE(semver.MustParse("1.123.0")):
		return liquiditypools.LiquidityPoolsVCUR
	case version.GTE(semver.MustParse("1.112.0")):
		return liquiditypools.LiquidityPoolsV122
	case version.GTE(semver.MustParse("1.111.0")):
		return liquiditypools.LiquidityPoolsV111
	case version.GTE(semver.MustParse("1.110.0")):
		return liquiditypools.LiquidityPoolsV110
	case version.GTE(semver.MustParse("1.109.0")):
		return liquiditypools.LiquidityPoolsV109
	case version.GTE(semver.MustParse("1.108.0")):
		return liquiditypools.LiquidityPoolsV108
	case version.GTE(semver.MustParse("1.107.0")):
		return liquiditypools.LiquidityPoolsV107
	case version.GTE(semver.MustParse("1.105.0")):
		return liquiditypools.LiquidityPoolsV105
	default:
		return liquiditypools.LiquidityPoolsV104
	}
}
