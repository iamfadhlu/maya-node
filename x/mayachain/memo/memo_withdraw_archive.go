package mayachain

import (
	"fmt"

	"github.com/blang/semver"
	"gitlab.com/mayachain/mayanode/common"
	cosmos "gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

func ParseWithdrawLiquidityMemoV1(ctx cosmos.Context, keeper keeper.Keeper, asset common.Asset, parts []string, version semver.Version) (WithdrawLiquidityMemo, error) {
	var err error
	if len(parts) < 2 {
		return WithdrawLiquidityMemo{}, fmt.Errorf("not enough parameters")
	}
	withdrawalBasisPts := cosmos.ZeroUint()
	withdrawalAsset := common.EmptyAsset
	pairAddress := common.NoAddress
	if len(parts) > 2 {
		withdrawalBasisPts, err = cosmos.ParseUint(parts[2])
		if err != nil {
			return WithdrawLiquidityMemo{}, err
		}
		if withdrawalBasisPts.IsZero() || withdrawalBasisPts.GT(cosmos.NewUint(types.MaxWithdrawBasisPoints)) {
			return WithdrawLiquidityMemo{}, fmt.Errorf("withdraw amount %s is invalid", parts[2])
		}
	}
	if len(parts) > 3 {
		withdrawalAsset, err = common.NewAssetWithShortCodes(version, parts[3])
		if err != nil {
			return WithdrawLiquidityMemo{}, err
		}
	}
	if len(parts) > 4 {
		if keeper == nil {
			pairAddress, err = common.NewAddress(parts[4], semver.MustParse("0.1.0"))
			if err != nil {
				return WithdrawLiquidityMemo{}, err
			}
		} else {
			pairAddress, err = FetchAddress(ctx, keeper, parts[4], asset.Chain)
			if err != nil {
				return WithdrawLiquidityMemo{}, err
			}
		}
	}
	return NewWithdrawLiquidityMemo(asset, withdrawalBasisPts, withdrawalAsset, pairAddress), nil
}
