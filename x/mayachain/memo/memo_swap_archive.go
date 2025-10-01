package mayachain

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/blang/semver"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

func ParseSwapMemoV1(ctx cosmos.Context, keeper keeper.Keeper, asset common.Asset, parts []string) (SwapMemo, error) {
	var err error
	var order types.OrderType
	if len(parts) < 2 {
		return SwapMemo{}, fmt.Errorf("not enough parameters")
	}
	// DESTADDR can be empty , if it is empty , it will swap to the sender address
	destination := common.NoAddress
	affAddr := common.NoAddress
	affPts := cosmos.ZeroUint()
	if len(parts) > 2 {
		if len(parts[2]) > 0 {
			if keeper == nil {
				destination, err = common.NewAddress(parts[2], semver.MustParse("0.1.0"))
			} else {
				destination, err = FetchAddress(ctx, keeper, parts[2], asset.Chain)
			}
			if err != nil {
				return SwapMemo{}, err
			}
		}
	}
	// price limit can be empty , when it is empty , there is no price protection
	var amount cosmos.Uint
	slip := cosmos.ZeroUint()
	if len(parts) > 3 && len(parts[3]) > 0 {
		amount, err = cosmos.ParseUint(parts[3])
		if err != nil {
			return SwapMemo{}, fmt.Errorf("swap price limit:%s is invalid", parts[3])
		}
		slip = amount
	}

	if len(parts) > 5 && len(parts[4]) > 0 && len(parts[5]) > 0 {
		if keeper == nil {
			affAddr, err = common.NewAddress(parts[4], semver.MustParse("0.1.0"))
		} else {
			affAddr, err = FetchAddress(ctx, keeper, parts[4], common.THORChain)
		}
		if err != nil {
			return SwapMemo{}, err
		}
		var pts uint64
		pts, err = strconv.ParseUint(parts[5], 10, 64)
		if err != nil {
			return SwapMemo{}, err
		}
		affPts = cosmos.NewUint(pts)
	}

	return NewSwapMemo(asset, destination, slip, affAddr, affPts, "", "", cosmos.ZeroUint(), order, 0, 0, "", nil, nil), nil
}

func ParseSwapMemoV92(ctx cosmos.Context, keeper keeper.Keeper, asset common.Asset, parts []string) (SwapMemo, error) {
	var err error
	var order types.OrderType
	dexAgg := ""
	dexTargetAddress := ""
	dexTargetLimit := cosmos.ZeroUint()
	if len(parts) < 2 {
		return SwapMemo{}, fmt.Errorf("not enough parameters")
	}
	// DESTADDR can be empty , if it is empty , it will swap to the sender address
	destination := common.NoAddress
	affAddr := common.NoAddress
	affPts := cosmos.ZeroUint()
	if len(parts) > 2 {
		if len(parts[2]) > 0 {
			if keeper == nil {
				destination, err = common.NewAddress(parts[2], semver.MustParse("1.92.0"))
			} else {
				destination, err = FetchAddress(ctx, keeper, parts[2], asset.Chain)
			}
			if err != nil {
				return SwapMemo{}, err
			}
		}
	}
	// price limit can be empty , when it is empty , there is no price protection
	var amount cosmos.Uint
	slip := cosmos.ZeroUint()
	if len(parts) > 3 && len(parts[3]) > 0 {
		amount, err = cosmos.ParseUint(parts[3])
		if err != nil {
			return SwapMemo{}, fmt.Errorf("swap price limit:%s is invalid", parts[3])
		}
		slip = amount
	}

	if len(parts) > 5 && len(parts[4]) > 0 && len(parts[5]) > 0 {
		if keeper == nil {
			affAddr, err = common.NewAddress(parts[4], semver.MustParse("1.92.0"))
		} else {
			affAddr, err = FetchAddress(ctx, keeper, parts[4], common.BASEChain)
		}
		if err != nil {
			return SwapMemo{}, err
		}
		var pts uint64
		pts, err = strconv.ParseUint(parts[5], 10, 64)
		if err != nil {
			return SwapMemo{}, err
		}
		affPts = cosmos.NewUint(pts)
	}

	if len(parts) > 6 && len(parts[6]) > 0 {
		dexAgg = parts[6]
	}

	if len(parts) > 7 && len(parts[7]) > 0 {
		dexTargetAddress = parts[7]
	}

	if len(parts) > 8 && len(parts[8]) > 0 {
		dexTargetLimit, err = cosmos.ParseUint(parts[8])
		if err != nil {
			ctx.Logger().Error("invalid dex target limit, ignore it", "limit", parts[8])
			dexTargetLimit = cosmos.ZeroUint()
		}
	}

	return NewSwapMemo(asset, destination, slip, affAddr, affPts, dexAgg, dexTargetAddress, dexTargetLimit, order, 0, 0, "", nil, nil), nil
}

func ParseSwapMemoV110(ctx cosmos.Context, keeper keeper.Keeper, version semver.Version, asset common.Asset, parts []string) (SwapMemo, error) {
	var err error
	var order types.OrderType
	dexAgg := ""
	dexTargetAddress := ""
	dexTargetLimit := cosmos.ZeroUint()
	if len(parts) < 2 {
		return SwapMemo{}, fmt.Errorf("not enough parameters")
	}
	// DESTADDR can be empty , if it is empty , it will swap to the sender address
	destination := common.NoAddress
	affAddr := common.NoAddress
	affPts := cosmos.ZeroUint()
	if len(parts) > 2 {
		if len(parts[2]) > 0 {
			if keeper == nil {
				destination, err = common.NewAddress(parts[2], version)
			} else {
				destination, err = FetchAddress(ctx, keeper, parts[2], asset.Chain)
			}
			if err != nil {
				return SwapMemo{}, err
			}
		}
	}
	// price limit can be empty , when it is empty , there is no price protection
	var limitStr string
	slip := cosmos.ZeroUint()
	streamInterval := uint64(0)
	streamQuantity := uint64(0)
	if len(parts) > 3 && len(parts[3]) > 0 {
		limitStr = parts[3]
		if strings.Contains(parts[3], "/") {
			split := strings.SplitN(limitStr, "/", 3)
			for i := range split {
				if split[i] == "" {
					split[i] = "0"
				}
			}
			if len(split) < 1 {
				return SwapMemo{}, fmt.Errorf("invalid streaming swap format: %s", parts[3])
			}
			slip, err = cosmos.ParseUint(split[0])
			if err != nil {
				return SwapMemo{}, fmt.Errorf("swap price limit:%s is invalid", parts[3])
			}
			if len(split) > 1 {
				streamInterval, err = strconv.ParseUint(split[1], 10, 64)
				if err != nil {
					return SwapMemo{}, fmt.Errorf("swap stream interval:%s is invalid", parts[3])
				}
			}

			if len(split) > 2 {
				streamQuantity, err = strconv.ParseUint(split[2], 10, 64)
				if err != nil {
					return SwapMemo{}, fmt.Errorf("swap stream quantity:%s is invalid", parts[3])
				}
			}
		} else {
			var amount cosmos.Uint
			amount, err = cosmos.ParseUint(parts[3])
			if err != nil {
				return SwapMemo{}, fmt.Errorf("swap price limit:%s is invalid", parts[3])
			}
			slip = amount
		}
	}

	if len(parts) > 5 && len(parts[4]) > 0 && len(parts[5]) > 0 {
		if keeper == nil {
			affAddr, err = common.NewAddress(parts[4], version)
		} else {
			affAddr, err = FetchAddress(ctx, keeper, parts[4], common.BASEChain)
		}
		if err != nil {
			return SwapMemo{}, err
		}
		var pts uint64
		pts, err = strconv.ParseUint(parts[5], 10, 64)
		if err != nil {
			return SwapMemo{}, err
		}
		affPts = cosmos.NewUint(pts)
	}

	if len(parts) > 6 && len(parts[6]) > 0 {
		dexAgg = parts[6]
	}

	if len(parts) > 7 && len(parts[7]) > 0 {
		dexTargetAddress = parts[7]
	}

	if len(parts) > 8 && len(parts[8]) > 0 {
		dexTargetLimit, err = cosmos.ParseUint(parts[8])
		if err != nil {
			ctx.Logger().Error("invalid dex target limit, ignore it", "limit", parts[8])
			dexTargetLimit = cosmos.ZeroUint()
		}
	}

	return NewSwapMemo(asset, destination, slip, affAddr, affPts, dexAgg, dexTargetAddress, dexTargetLimit, order, streamQuantity, streamInterval, "", nil, nil), nil
}

func (p *parser) ParseSwapMemoV112() (SwapMemo, error) {
	var err error
	var order types.OrderType
	asset := p.getAsset(1, true, common.EmptyAsset)

	// DESTADDR can be empty , if it is empty , it will swap to the sender address
	destination, refundAddress := p.getAddressAndRefundAddressWithKeeper(2, false, common.NoAddress, asset.Chain)

	// price limit can be empty , when it is empty , there is no price protection
	var slip cosmos.Uint
	streamInterval := uint64(0)
	streamQuantity := uint64(0)
	if strings.Contains(p.get(3), "/") {
		parts := strings.SplitN(p.get(3), "/", 3)
		for i := range parts {
			if parts[i] == "" {
				parts[i] = "0"
			}
		}
		if len(parts) < 1 {
			return SwapMemo{}, fmt.Errorf("invalid streaming swap format: %s", p.get(3))
		}
		slip, err = cosmos.ParseUint(parts[0])
		if err != nil {
			return SwapMemo{}, fmt.Errorf("swap price limit:%s is invalid", parts[0])
		}
		if len(parts) > 1 {
			streamInterval, err = strconv.ParseUint(parts[1], 10, 64)
			if err != nil {
				return SwapMemo{}, fmt.Errorf("swap stream interval:%s is invalid", parts[1])
			}
		}

		if len(parts) > 2 {
			streamQuantity, err = strconv.ParseUint(parts[2], 10, 64)
			if err != nil {
				return SwapMemo{}, fmt.Errorf("swap stream quantity:%s is invalid", parts[2])
			}
		}
	} else {
		slip = p.getUintWithScientificNotation(3, false, 0)
	}

	maxAffiliateFeeBasisPoints := cosmos.NewUint(uint64(p.getConfigInt64(constants.MaxAffiliateFeeBasisPoints)))
	affiliates, affFeeBps, totalAffBps := p.getMultipleAffiliatesAndBps(4, false, maxAffiliateFeeBasisPoints)

	maxAffiliates := p.getConfigInt64(constants.MultipleAffiliatesMaxCount)
	if len(affiliates) > int(maxAffiliates) {
		return SwapMemo{}, fmt.Errorf("maximum allowed affiliates is %d", maxAffiliates)
	}

	// TODO: Remove on hardfork
	// Set a affiliate address (even though it is not used) - to pass validation
	affAddr := common.NoAddress
	if !totalAffBps.IsZero() && len(affiliates) > 0 {
		affAddr = p.getAddressFromString(affiliates[0], common.BASEChain, false)
	}

	dexAgg := p.get(6)
	dexTargetAddress := p.get(7)
	dexTargetLimit := p.getUintWithScientificNotation(8, false, 0)

	return NewSwapMemo(asset, destination, slip, affAddr, totalAffBps, dexAgg, dexTargetAddress, dexTargetLimit, order, streamQuantity, streamInterval, refundAddress, affiliates, affFeeBps), p.Error()
}

func (p *parser) ParseSwapMemoV118() (SwapMemo, error) {
	var err error
	var order types.OrderType
	asset := p.getAsset(1, true, common.EmptyAsset)

	// DESTADDR can be empty , if it is empty , it will swap to the sender address
	destination, refundAddress := p.getAddressAndRefundAddressWithKeeper(2, false, common.NoAddress, asset.Chain)

	// price limit can be empty , when it is empty , there is no price protection
	var slip cosmos.Uint
	streamInterval := uint64(0)
	streamQuantity := uint64(0)
	if strings.Contains(p.get(3), "/") {
		parts := strings.SplitN(p.get(3), "/", 3)
		for i := range parts {
			if parts[i] == "" {
				parts[i] = "0"
			}
		}
		if len(parts) < 1 {
			return SwapMemo{}, fmt.Errorf("invalid streaming swap format: %s", p.get(3))
		}
		slip, err = cosmos.ParseUint(parts[0])
		if err != nil {
			return SwapMemo{}, fmt.Errorf("swap price limit:%s is invalid", parts[0])
		}
		if len(parts) > 1 {
			streamInterval, err = strconv.ParseUint(parts[1], 10, 64)
			if err != nil {
				return SwapMemo{}, fmt.Errorf("swap stream interval:%s is invalid", parts[1])
			}
		}

		if len(parts) > 2 {
			streamQuantity, err = strconv.ParseUint(parts[2], 10, 64)
			if err != nil {
				return SwapMemo{}, fmt.Errorf("swap stream quantity:%s is invalid", parts[2])
			}
		}
	} else {
		slip = p.getUintWithScientificNotation(3, false, 0)
	}

	maxAffiliateFeeBasisPoints := cosmos.NewUint(uint64(p.getConfigInt64(constants.MaxAffiliateFeeBasisPoints)))
	affiliates, affFeeBps, totalAffBps := p.getMultipleAffiliatesAndBps(4, false, maxAffiliateFeeBasisPoints)

	maxAffiliates := p.getConfigInt64(constants.MultipleAffiliatesMaxCount)
	if len(affiliates) > int(maxAffiliates) {
		return SwapMemo{}, fmt.Errorf("maximum allowed affiliates is %d", maxAffiliates)
	}

	// TODO: Remove on hardfork
	// Set a affiliate address (even though it is not used) - to pass validation
	affAddr := common.NoAddress
	if !totalAffBps.IsZero() && len(affiliates) > 0 {
		affAddr = p.getAddressFromString(affiliates[0], common.BASEChain, false)
		// if affiliate address is empty and mayaname exists, that means mayaname doesn't have maya alias, use the owner address
		if affAddr.IsEmpty() && p.keeper.MAYANameExists(p.ctx, affiliates[0]) {
			var mn types.MAYAName
			mn, err = p.keeper.GetMAYAName(p.ctx, affiliates[0])
			if err != nil {
				return SwapMemo{}, fmt.Errorf("failed to get MAYAName %s: %w", affiliates[0], err)
			}
			affAddr = common.Address(mn.Owner.String())
			// if owner is empty, try maya alias
			if affAddr.IsEmpty() {
				affAddr = mn.GetAlias(common.BASEChain)
			}
			// if for some reason both owner and maya alias are empty, set affiliate collector module as affiliate address (used only for validation, no funds will be sent there)
			if affAddr.IsEmpty() {
				affAddr, err = p.keeper.GetModuleAddress(types.AffiliateCollectorName)
				if err != nil {
					return SwapMemo{}, fmt.Errorf("failed to get affiliate collector module address: %w", err)
				}
			}
		}
	}

	dexAgg := p.get(6)
	dexTargetAddress := p.get(7)
	dexTargetLimit := p.getUintWithScientificNotation(8, false, 0)

	return NewSwapMemo(asset, destination, slip, affAddr, totalAffBps, dexAgg, dexTargetAddress, dexTargetLimit, order, streamQuantity, streamInterval, refundAddress, affiliates, affFeeBps), p.Error()
}
