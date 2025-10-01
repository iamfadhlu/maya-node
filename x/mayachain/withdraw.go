package mayachain

import (
	"github.com/blang/semver"

	"gitlab.com/mayachain/mayanode/common/cosmos"
)

func withdraw(ctx cosmos.Context, msg MsgWithdrawLiquidity, mgr Manager) (cosmos.Uint, cosmos.Uint, cosmos.Uint, cosmos.Uint, cosmos.Uint, error) {
	version := mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.124.0")):
		return withdrawV124(ctx, msg, mgr)
	case version.GTE(semver.MustParse("1.108.0")):
		return withdrawV108(ctx, msg, mgr)
	case version.GTE(semver.MustParse("1.105.0")):
		return withdrawV105(ctx, msg, mgr)
	case version.GTE(semver.MustParse("1.102.0")):
		return withdrawV102(ctx, msg, mgr)
	case version.GTE(semver.MustParse("1.91.0")):
		return withdrawV91(ctx, msg, mgr)
	default:
		zero := cosmos.ZeroUint()
		return zero, zero, zero, zero, zero, errInvalidVersion
	}
}
