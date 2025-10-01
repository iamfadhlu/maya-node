package mayachain

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/blang/semver"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

type SwapMemo struct {
	MemoBase
	Destination           common.Address
	SlipLimit             cosmos.Uint
	AffiliateAddress      common.Address // TODO: remove on hardfork
	AffiliateBasisPoints  cosmos.Uint    // TODO: remove on hardfork
	DexAggregator         string
	DexTargetAddress      string
	DexTargetLimit        *cosmos.Uint
	OrderType             types.OrderType
	StreamInterval        uint64
	StreamQuantity        uint64
	RefundAddress         common.Address
	Affiliates            []string
	AffiliatesBasisPoints []cosmos.Uint
}

func (m SwapMemo) GetDestination() common.Address          { return m.Destination }
func (m SwapMemo) GetSlipLimit() cosmos.Uint               { return m.SlipLimit }
func (m SwapMemo) GetAffiliateAddress() common.Address     { return m.AffiliateAddress }
func (m SwapMemo) GetAffiliateBasisPoints() cosmos.Uint    { return m.AffiliateBasisPoints }
func (m SwapMemo) GetDexAggregator() string                { return m.DexAggregator }
func (m SwapMemo) GetDexTargetAddress() string             { return m.DexTargetAddress }
func (m SwapMemo) GetDexTargetLimit() *cosmos.Uint         { return m.DexTargetLimit }
func (m SwapMemo) GetOrderType() types.OrderType           { return m.OrderType }
func (m SwapMemo) GetStreamQuantity() uint64               { return m.StreamQuantity }
func (m SwapMemo) GetStreamInterval() uint64               { return m.StreamInterval }
func (m SwapMemo) GetRefundAddress() common.Address        { return m.RefundAddress }
func (m SwapMemo) GetAffiliates() []string                 { return m.Affiliates }
func (m SwapMemo) GetAffiliatesBasisPoints() []cosmos.Uint { return m.AffiliatesBasisPoints }

func (m SwapMemo) String() string {
	return m.string(false)
}

func (m SwapMemo) ShortString() string {
	return m.string(true)
}

func (m SwapMemo) string(short bool) string {
	slipLimit := m.SlipLimit.String()
	if m.SlipLimit.IsZero() {
		slipLimit = ""
	}

	// prefer short notation for generate swap memo
	txType := m.TxType.String()
	if m.TxType == TxSwap {
		txType = "="
	}

	if m.StreamInterval > 0 || m.StreamQuantity > 1 {
		slipLimit = fmt.Sprintf("%s/%d/%d", m.SlipLimit.String(), m.StreamInterval, m.StreamQuantity)
	}

	var assetString string
	if short && len(m.Asset.ShortCode()) > 0 {
		assetString = m.Asset.ShortCode()
	} else {
		assetString = m.Asset.String()
	}

	// shorten the addresses, if possible
	destString := m.Destination.AbbreviatedString(common.LatestVersion)

	// destination + custom refund addr
	if !m.RefundAddress.IsEmpty() {
		destString = destString + "/" + m.RefundAddress.AbbreviatedString(common.LatestVersion)
	}

	affString, affBpsString := getAffiliatesMemoString(m.Affiliates, m.AffiliatesBasisPoints)

	args := []string{
		txType,
		assetString,
		destString,
		slipLimit,
		affString,
		affBpsString,
		m.DexAggregator,
		m.DexTargetAddress,
	}

	last := 3
	if !m.SlipLimit.IsZero() || m.StreamInterval > 0 || m.StreamQuantity > 1 {
		last = 4
	}

	if len(m.Affiliates) > 0 {
		last = 5
	}
	if len(m.AffiliatesBasisPoints) > 0 {
		last = 6
	}

	if m.DexAggregator != "" {
		last = 8
	}

	if m.DexTargetLimit != nil && !m.DexTargetLimit.IsZero() {
		args = append(args, m.DexTargetLimit.String())
		last = 9
	}

	return strings.Join(args[:last], ":")
}

func NewSwapMemo(asset common.Asset, dest common.Address, slip cosmos.Uint, affiliateAddress common.Address, affiliateBasisPoints cosmos.Uint, dexAgg, dexTargetAddress string, dexTargetLimit cosmos.Uint, orderType types.OrderType, quan, interval uint64, refundAddress common.Address, affiliates []string, affiliatesFeeBps []cosmos.Uint) SwapMemo {
	swapMemo := SwapMemo{
		MemoBase:              MemoBase{TxType: TxSwap, Asset: asset},
		Destination:           dest,
		SlipLimit:             slip,
		AffiliateAddress:      affiliateAddress,
		AffiliateBasisPoints:  affiliateBasisPoints,
		DexAggregator:         dexAgg,
		DexTargetAddress:      dexTargetAddress,
		OrderType:             orderType,
		StreamQuantity:        quan,
		StreamInterval:        interval,
		RefundAddress:         refundAddress,
		Affiliates:            affiliates,
		AffiliatesBasisPoints: affiliatesFeeBps,
	}
	if !dexTargetLimit.IsZero() {
		swapMemo.DexTargetLimit = &dexTargetLimit
	}

	return swapMemo
}

func (p *parser) ParseSwapMemo() (SwapMemo, error) {
	if p.keeper == nil {
		return ParseSwapMemoV1(p.ctx, p.keeper, p.getAsset(1, true, common.EmptyAsset), p.parts)
	}
	switch {
	case p.keeper.GetVersion().GTE(semver.MustParse("1.121.0")):
		return p.ParseSwapMemoV121()
	case p.keeper.GetVersion().GTE(semver.MustParse("1.118.0")):
		return p.ParseSwapMemoV118()
	case p.keeper.GetVersion().GTE(semver.MustParse("1.112.0")):
		return p.ParseSwapMemoV112()
	case p.keeper.GetVersion().GTE(semver.MustParse("1.110.0")):
		return ParseSwapMemoV110(p.ctx, p.keeper, p.version, p.getAsset(1, true, common.EmptyAsset), p.parts)
	case p.keeper.GetVersion().GTE(semver.MustParse("1.92.0")):
		return ParseSwapMemoV92(p.ctx, p.keeper, p.getAsset(1, true, common.EmptyAsset), p.parts)
	default:
		return ParseSwapMemoV1(p.ctx, p.keeper, p.getAsset(1, true, common.EmptyAsset), p.parts)
	}
}

func (p *parser) ParseSwapMemoV121() (SwapMemo, error) {
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
		slip, err = parseTradeTarget(parts[0])
		if err != nil {
			return SwapMemo{}, fmt.Errorf("swap price limit:%s is invalid: %s", parts[0], err)
		}
		if len(parts) > 1 {
			streamInterval, err = strconv.ParseUint(parts[1], 10, 64)
			if err != nil {
				return SwapMemo{}, fmt.Errorf("failed to parse stream frequency: %s: %s", parts[1], err)
			}
		}
		if len(parts) > 2 {
			streamQuantity, err = strconv.ParseUint(parts[2], 10, 64)
			if err != nil {
				return SwapMemo{}, fmt.Errorf("failed to parse stream quantity: %s: %s", parts[2], err)
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
