package mayachain

import (
	"strconv"

	"github.com/blang/semver"
	"gitlab.com/mayachain/mayanode/common"
	cosmos "gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
)

func ParseAddLiquidityMemoV1(ctx cosmos.Context, keeper keeper.Keeper, asset common.Asset, parts []string) (AddLiquidityMemo, error) {
	var err error
	addr := common.NoAddress
	affAddr := common.NoAddress
	affPts := cosmos.ZeroUint()
	if len(parts) >= 3 && len(parts[2]) > 0 {
		if keeper == nil {
			addr, err = common.NewAddress(parts[2], semver.MustParse("0.1.0"))
		} else {
			addr, err = FetchAddress(ctx, keeper, parts[2], asset.Chain)
		}
		if err != nil {
			return AddLiquidityMemo{}, err
		}
	}

	if len(parts) > 4 && len(parts[3]) > 0 && len(parts[4]) > 0 {
		if keeper == nil {
			affAddr, err = common.NewAddress(parts[3], semver.MustParse("0.1.0"))
		} else {
			affAddr, err = FetchAddress(ctx, keeper, parts[3], common.BASEChain)
		}
		if err != nil {
			return AddLiquidityMemo{}, err
		}
		var pts uint64
		pts, err = strconv.ParseUint(parts[4], 10, 64)
		if err != nil {
			return AddLiquidityMemo{}, err
		}
		affPts = cosmos.NewUint(pts)
	}
	return NewAddLiquidityMemo(asset, addr, affAddr, affPts, 0), nil
}

func ParseAddLiquidityMemoV110(ctx cosmos.Context, keeper keeper.Keeper, asset common.Asset, parts []string, version semver.Version) (AddLiquidityMemo, error) {
	var err error
	tier := DefaultTierValue
	addr := common.NoAddress
	affAddr := common.NoAddress
	affPts := cosmos.ZeroUint()
	if len(parts) >= 3 && len(parts[2]) > 0 {
		if keeper == nil {
			addr, err = common.NewAddress(parts[2], version)
		} else {
			addr, err = FetchAddress(ctx, keeper, parts[2], asset.Chain)
		}
		if err != nil {
			return AddLiquidityMemo{}, err
		}
	}

	// Check optional parameters
	if len(parts) == 4 && len(parts[3]) > 0 && (parts[3] == "TIER1" || parts[3] == "TIER2") {
		tier, err = strconv.Atoi(parts[3][4:5])
		if err != nil {
			return AddLiquidityMemo{}, err
		}
	} else if len(parts) > 4 && len(parts[3]) > 0 && len(parts[4]) > 0 {
		if keeper == nil {
			affAddr, err = common.NewAddress(parts[3], version)
		} else {
			affAddr, err = FetchAddress(ctx, keeper, parts[3], common.BASEChain)
		}
		if err != nil {
			return AddLiquidityMemo{}, err
		}
		var pts uint64
		pts, err = strconv.ParseUint(parts[4], 10, 64)
		if err != nil {
			return AddLiquidityMemo{}, err
		}
		affPts = cosmos.NewUint(pts)

		if len(parts) > 5 && len(parts[5]) > 0 && (parts[5] == "TIER1" || parts[5] == "TIER2") {
			tier, err = strconv.Atoi(parts[5][4:5])
			if err != nil {
				return AddLiquidityMemo{}, err
			}
		}
	}

	return NewAddLiquidityMemo(asset, addr, affAddr, affPts, int64(tier)), nil
}
