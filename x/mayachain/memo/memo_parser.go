package mayachain

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/blang/semver"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

type parser struct {
	memo           string
	txType         TxType
	ctx            cosmos.Context
	keeper         keeper.Keeper
	parts          []string
	errs           []error
	version        semver.Version
	requiredFields int
}

func newParser(ctx cosmos.Context, keeper keeper.Keeper, version semver.Version, memo string) (parser, error) {
	if len(memo) == 0 {
		return parser{}, fmt.Errorf("memo can't be empty")
	}
	if version.GTE(semver.MustParse("1.108.0")) {
		memo = strings.Split(memo, "|")[0]
	}
	parts := strings.Split(memo, ":")
	memoType, err := StringToTxType(parts[0])
	if err != nil {
		return parser{}, err
	}

	return parser{
		memo:    memo,
		txType:  memoType,
		ctx:     ctx,
		keeper:  keeper,
		version: version,
		parts:   parts,
		errs:    make([]error, 0),
	}, nil
}

func (p *parser) getType() TxType {
	return p.txType
}

func (p *parser) incRequired(required bool) {
	if required {
		p.requiredFields += 1
	}
}

func (p *parser) parse() (mem Memo, err error) {
	defer func() {
		if err == nil {
			err = p.Error()
		}
	}()
	switch p.getType() {
	case TxLeave:
		return p.ParseLeaveMemo()
	case TxDonate:
		return p.ParseDonateMemo()
	case TxAdd:
		return p.ParseAddLiquidityMemo()
	case TxWithdraw:
		return p.ParseWithdrawLiquidityMemo()
	case TxCacaoPoolDeposit:
		return p.ParseCacaoPoolDepositMemo()
	case TxCacaoPoolWithdraw:
		return p.ParseCacaoPoolWithdrawMemo()
	case TxSwap:
		return p.ParseSwapMemo()
	case TxOutbound:
		return p.ParseOutboundMemo()
	case TxRefund:
		return p.ParseRefundMemo()
	case TxBond:
		return p.ParseBondMemo()
	case TxUnbond:
		return p.ParseUnbondMemo()
	case TxYggdrasilFund:
		return p.ParseYggdrasilFundMemo()
	case TxYggdrasilReturn:
		return p.ParseYggdrasilReturnMemo()
	case TxReserve:
		return p.ParseReserveMemo()
	case TxMigrate:
		return p.ParseMigrateMemo()
	case TxRagnarok:
		return p.ParseRagnarokMemo()
	case TxNoOp:
		return p.ParseNoOpMemo()
	case TxConsolidate:
		return p.ParseConsolidateMemo()
	case TxMAYAName:
		return p.ParseManageMAYANameMemo()
	case TxTradeAccountDeposit:
		return p.ParseTradeAccountDeposit()
	case TxTradeAccountWithdrawal:
		return p.ParseTradeAccountWithdrawal()
	default:
		return EmptyMemo, fmt.Errorf("TxType not supported: %s", p.getType().String())
	}
}

func (p *parser) addErr(err error) {
	p.errs = append(p.errs, err)
}

func (p *parser) Error() error {
	p.hasMinParams(p.requiredFields + 1)
	if len(p.errs) == 0 {
		return nil
	}
	errStrs := make([]string, len(p.errs))
	for i, err := range p.errs {
		errStrs[i] = err.Error()
	}
	err := fmt.Errorf("MEMO: %s\nPARSE FAILURE(S): %s", p.memo, strings.Join(errStrs, "-"))
	return err
}

// check if memo has enough parameters
func (p *parser) hasMinParams(count int) {
	if len(p.parts) < count {
		p.addErr(fmt.Errorf("not enough parameters: %d/%d", len(p.parts), count))
	}
}

// Safe accessor for split memo parts - always returns empty
// string for indices that are out of bounds.
func (p *parser) get(idx int) string {
	if idx < 0 || len(p.parts) <= idx {
		return ""
	}
	return p.parts[idx]
}

func (p *parser) getInt64(idx int, required bool, def int64) int64 {
	p.incRequired(required)
	value, err := strconv.ParseInt(p.get(idx), 10, 64)
	if err != nil {
		if required || p.get(idx) != "" {
			p.addErr(fmt.Errorf("cannot parse '%s' as an int64: %w", p.get(idx), err))
		}
		return def
	}
	return value
}

func (p *parser) getUint(idx int, required bool, def uint64) cosmos.Uint {
	p.incRequired(required)
	value, err := cosmos.ParseUint(p.get(idx))
	if err != nil {
		if required || p.get(idx) != "" {
			p.addErr(fmt.Errorf("cannot parse '%s' as an uint: %w", p.get(idx), err))
		}
		return cosmos.NewUint(def)
	}
	return value
}

func (p *parser) getUintWithScientificNotation(idx int, required bool, def uint64) cosmos.Uint {
	p.incRequired(required)
	f, _, err := big.ParseFloat(p.get(idx), 10, 0, big.ToZero)
	if err != nil {
		if required || p.get(idx) != "" {
			p.addErr(fmt.Errorf("cannot parse '%s' as an uint with sci notation: %w", p.get(idx), err))
		}
		return cosmos.NewUint(def)
	}
	i := new(big.Int)
	f.Int(i) // Note: fractional part will be discarded
	result := cosmos.NewUintFromBigInt(i)
	return result
}

func (p *parser) getUintWithMaxValue(idx int, required bool, def, max uint64) cosmos.Uint {
	value := p.getUint(idx, required, def)
	if value.GT(cosmos.NewUint(max)) {
		if required || p.get(idx) != "" {
			p.addErr(fmt.Errorf("%s cannot exceed '%d'", p.get(idx), max))
		}
		return cosmos.NewUint(max)
	}
	return value
}

func (p *parser) getAccAddress(idx int, required bool, def cosmos.AccAddress) cosmos.AccAddress {
	p.incRequired(required)
	value, err := cosmos.AccAddressFromBech32(p.get(idx))
	if err != nil {
		if required || p.get(idx) != "" {
			p.addErr(fmt.Errorf("cannot parse '%s' as an AccAddress: %w", p.get(idx), err))
		}
		return def
	}
	return value
}

func (p *parser) getAddress(idx int, required bool, def common.Address, version semver.Version) common.Address {
	p.incRequired(required)
	value, err := common.NewAddress(p.get(idx), version)
	if err != nil {
		if required || p.get(idx) != "" {
			p.addErr(fmt.Errorf("cannot parse '%s' as an Address: %w", p.get(idx), err))
		}
		return def
	}
	return value
}

func (p *parser) getAddressWithKeeper(idx int, required bool, def common.Address, chain common.Chain, version semver.Version) common.Address {
	p.incRequired(required)
	if p.keeper == nil {
		return p.getAddress(2, required, common.NoAddress, version)
	}
	addr, err := FetchAddress(p.ctx, p.keeper, p.get(idx), chain)
	if err != nil {
		if required || p.get(idx) != "" {
			p.addErr(fmt.Errorf("cannot parse '%s' as an Address: %w", p.get(idx), err))
		}
	}
	return addr
}

func (p *parser) getStringArrayBySeparator(idx int, required bool, separator string) []string {
	p.incRequired(required)
	value := p.get(idx)
	if value == "" {
		return []string{}
	}
	return strings.Split(value, separator)
}

func (p *parser) getUintArrayBySeparator(idx int, required bool, separator string, def, max cosmos.Uint) []cosmos.Uint {
	switch {
	case p.version.GTE(semver.MustParse("1.121.0")): // cacaopool-tx
		return p.getUintArrayBySeparatorV121(idx, required, separator, def, max)
	default:
		return p.getUintArrayBySeparatorV1(idx, required, separator, def, max)
	}
}

func (p *parser) getUintArrayBySeparatorV121(idx int, required bool, separator string, def, max cosmos.Uint) []cosmos.Uint {
	p.incRequired(required)
	value := p.get(idx)
	if value == "" {
		return []cosmos.Uint{}
	}
	strArray := strings.Split(value, separator)
	result := make([]cosmos.Uint, 0, len(strArray))
	for _, str := range strArray {
		u, err := cosmos.ParseUint(str)
		if err != nil {
			if required {
				p.addErr(fmt.Errorf("cannot parse '%s' as an uint: %w", str, err))
				return []cosmos.Uint{}
			}
			u = def
		}
		if !u.Equal(def) && !max.IsZero() && u.GT(max) {
			p.addErr(fmt.Errorf("uint value %s is greater than max value %s", u, max))
		}
		result = append(result, u)
	}
	return result
}

func (p *parser) getAddressAndRefundAddressWithKeeper(idx int, required bool, def common.Address, chain common.Chain) (common.Address, common.Address) {
	p.incRequired(required)

	//nolint:ineffassign
	destination := common.NoAddress
	refundAddress := common.NoAddress
	addresses := p.get(idx)

	if strings.Contains(addresses, "/") {
		parts := strings.SplitN(addresses, "/", 2)
		if p.keeper == nil {
			dest, err := common.NewAddress(parts[0], p.version)
			if err != nil {
				if required || parts[0] != "" {
					p.addErr(fmt.Errorf("cannot parse '%s' as an Address: %w", parts[0], err))
				}
			}
			destination = dest
		} else {
			destination = p.getAddressFromString(parts[0], chain, required)
		}
		if len(parts) > 1 {
			refundAddress, _ = common.NewAddress(parts[1], p.version)
		}
	} else {
		destination = p.getAddressWithKeeper(idx, false, common.NoAddress, chain, p.version)
	}

	if destination.IsEmpty() && !refundAddress.IsEmpty() {
		p.addErr(fmt.Errorf("refund address is set but destination address is empty"))
	}

	return destination, refundAddress
}

func (p *parser) getAddressFromString(val string, chain common.Chain, required bool) common.Address {
	addr, err := FetchAddress(p.ctx, p.keeper, val, chain)
	if err != nil {
		if required || val != "" {
			p.addErr(fmt.Errorf("cannot parse '%s' as an Address: %w", val, err))
		}
	}
	return addr
}

func (p *parser) getChain(idx int, required bool, def common.Chain) common.Chain {
	p.incRequired(required)
	value, err := common.NewChain(p.get(idx))
	if err != nil {
		if required || p.get(idx) != "" {
			p.addErr(fmt.Errorf("cannot parse '%s' as a chain: %w", p.get(idx), err))
		}
		return def
	}
	return value
}

func (p *parser) getAsset(idx int, required bool, def common.Asset) common.Asset {
	switch {
	case p.version.GTE(semver.MustParse("1.123.0")): // trade-accounts
		return p.getAssetV123(idx, required, def)
	default:
		return p.getAssetV1(idx, required, def)
	}
}

func (p *parser) getAssetV123(idx int, required bool, def common.Asset) common.Asset {
	p.incRequired(required)
	value, err := common.NewAssetWithShortCodes(p.version, p.get(idx))
	if err != nil && (required || p.get(idx) != "") {
		p.addErr(fmt.Errorf("cannot parse '%s' as an asset: %w", p.get(idx), err))
		return def
	}
	return value
}

func (p *parser) getAssetV1(idx int, required bool, def common.Asset) common.Asset {
	p.incRequired(required)
	value, err := common.NewAssetWithShortCodes(p.version, p.get(idx))
	if err != nil {
		if required || p.get(idx) != "" {
			p.addErr(fmt.Errorf("cannot parse '%s' as an asset: %w", p.get(idx), err))
		}
		return def
	}
	return value
}

func (p *parser) getTxID(idx int, required bool, def common.TxID) common.TxID {
	p.incRequired(required)
	value, err := common.NewTxID(p.get(idx))
	if err != nil {
		if required || p.get(idx) != "" {
			p.addErr(fmt.Errorf("cannot parse '%s' as tx hash: %w", p.get(idx), err))
		}
		return def
	}
	return value
}

// getMultipleAffiliatesAndBps parses multiple affiliate names and their corresponding fee basis points.
//
// Parameters:
// - idx (int): The index in the input where the affiliate data starts in memo.
// - bpsRequired (bool): Indicates if BPS values for affiliates are required (must not be empty).
// - totalBpsMax (cosmos.Uint): The maximum allowed sum of all affiliate BPS values.
//
// Returns:
// - affiliatesNames ([]string): A list of affiliate names (parsed from input).
// - affiliatesBps ([]cosmos.Uint): A list of affiliate BPS values (parsed from input).
// - totalAffBps (cosmos.Uint): The total sum of the affiliate BPS values.
//
// Functionality:
// 1. Parses affiliate names or addresses and their corresponding BPS values from input, using a separator ("/").
// 2. Verifies that each parsed affiliate MAYAName exists or is a valid address.
// 3. If only one BPS value is provided for multiple affiliates, applies that value to all affiliates (eg. ":a/b/c:10" - uses 10 bps for a, b and c).
// 4. Fills missing BPS values with the default BPS from MAYANames, if allowed (bpsRequired==false) (eg. ":a/b/c::" - will use default bps' from a, b and c).
// 5. Ensures that the number of parsed affiliate names and BPS values match.
// 6. Optionally, validates that the total sum of BPS values does not exceed the specified `totalBpsMax`.
//
// Errors:
// - If an affiliate name does not exist or is an invalid address.
// - If there is a mismatch between the number of affiliate names and BPS values.
// - If the total sum of BPS values exceeds `totalBpsMax`.

func (p *parser) getMultipleAffiliatesAndBps(idx int, bpsRequired bool, totalBpsMax cosmos.Uint) (affiliatesNames []string, affiliatesBps []cosmos.Uint, totalAffBps cosmos.Uint) {
	version := p.keeper.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.118.0")):
		var err error
		separator := "/"
		// Parse multiple (sub)affiliate mayanames + fee bps
		affiliatesNames = p.getStringArrayBySeparator(idx, false, separator)
		affiliatesBps = p.getUintArrayBySeparator(idx+1, bpsRequired, separator, types.EmptyBps, totalBpsMax)
		affiliatesBps, totalAffBps, err = GetMultipleAffiliatesAndBps(p.ctx, p.keeper, bpsRequired, totalBpsMax, affiliatesNames, affiliatesBps)
		if err != nil {
			p.addErr(fmt.Errorf("failed to process affiliate parameters: %w", err))
		}
		return affiliatesNames, affiliatesBps, totalAffBps
		// return getMultipleAffiliatesAndBpsV118(ctx, keeper, bpsRequired, totalBpsMax, affiliatesNames, affiliatesBpsIn)
	default:
		return p.getMultipleAffiliatesAndBpsV1(idx, bpsRequired, totalBpsMax)
	}
}

func (p *parser) getMultipleAffiliatesAndBpsV1(idx int, bpsRequired bool, totalBpsMax cosmos.Uint) (affiliatesNames []string, affiliatesBps []cosmos.Uint, totalAffBps cosmos.Uint) {
	separator := "/"
	totalAffBps = cosmos.ZeroUint()
	// Parse multiple (sub)affiliate mayanames + fee bps
	affiliatesNames = p.getStringArrayBySeparator(idx, false, separator)
	affiliatesBps = p.getUintArrayBySeparator(idx+1, bpsRequired, separator, types.EmptyBps, totalBpsMax)

	if p.keeper != nil {
		// verify all mayanames and/or addresses
		for _, name := range affiliatesNames {
			// check if address is valid
			_, err := FetchAddress(p.ctx, p.keeper, name, common.BASEChain)
			if err != nil {
				p.addErr(fmt.Errorf("invalid affiliate mayaname or address: %w", err))
			}
		}
	}

	// if only one aff bps defined, apply to all address affiliates
	if len(affiliatesBps) == 1 && len(affiliatesNames) > 1 {
		fee := affiliatesBps[0]
		affiliatesBps = make([]cosmos.Uint, len(affiliatesNames))
		affiliatesBps[0] = fee
		for i := 1; i < len(affiliatesNames); i++ {
			// bpsRequired is true when we parse subaffiliates;
			// we need to set the first bps to all subaffiliates
			// as we don't use the default bps in mayaname in case of subaffiliates
			if bpsRequired || !p.keeper.MAYANameExists(p.ctx, affiliatesNames[i]) {
				// mayaname doesn't exist it is an address, use the provided fee
				affiliatesBps[i] = fee
			} else {
				// if mayaname exists, set default here, it will be populated later with bps from mayaname
				affiliatesBps[i] = types.EmptyBps
			}
		}
	}

	// if no affiliates basis points provided, set all to EmptyBps, later we try to populate the affiliates bps from MAYANames
	if len(affiliatesBps) == 0 && len(affiliatesNames) > 0 && !bpsRequired {
		affiliatesBps = make([]cosmos.Uint, len(affiliatesNames))
		for i := range affiliatesBps {
			affiliatesBps[i] = types.EmptyBps
		}
	}

	// at this point the size of names and bps should be the same
	if len(affiliatesNames) != len(affiliatesBps) {
		p.addErr(fmt.Errorf("affiliate mayanames and affiliate fee bps count mismatch (%d / %d)", len(affiliatesNames), len(affiliatesBps)))
		return
	}

	// calculate the sum of all bps
	for i := range affiliatesBps {
		if affiliatesBps[i] == types.EmptyBps {
			if p.keeper != nil {
				if !p.keeper.MAYANameExists(p.ctx, affiliatesNames[i]) {
					p.addErr(fmt.Errorf("cannot parse '%s' as a MAYAName while empty affiliate basis points provided at index %d", affiliatesNames[i], i))
					continue
				}
				mn, err := p.keeper.GetMAYAName(p.ctx, affiliatesNames[i])
				if err != nil {
					p.addErr(fmt.Errorf("fail to get MAYAName %s", affiliatesNames[i]))
				}
				affiliatesBps[i] = mn.GetAffiliateBps()
			}
		}
		totalAffBps = totalAffBps.Add(affiliatesBps[i])
	}

	// check if we should verify the sum of all affiliates bps
	if !totalBpsMax.IsZero() && totalAffBps.GT(totalBpsMax) {
		p.addErr(fmt.Errorf("total affiliate fee basis points must not exceed %s (totalBps: %s)", totalBpsMax, totalAffBps))
	}
	return
}

func GetMultipleAffiliatesAndBps(ctx cosmos.Context, keeper keeper.Keeper, bpsRequired bool, totalBpsMax cosmos.Uint, affiliatesNames []string, affiliatesBpsIn []cosmos.Uint) (affiliatesBps []cosmos.Uint, totalAffBps cosmos.Uint, err error) {
	version := keeper.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.118.0")):
		return getMultipleAffiliatesAndBpsV118(ctx, keeper, bpsRequired, totalBpsMax, affiliatesNames, affiliatesBpsIn)
	default:
		return getMultipleAffiliatesAndBpsV1(ctx, keeper, bpsRequired, totalBpsMax, affiliatesNames, affiliatesBpsIn)
	}
}

func getMultipleAffiliatesAndBpsV118(ctx cosmos.Context, keeper keeper.Keeper, bpsRequired bool, totalBpsMax cosmos.Uint, affiliatesNames []string, affiliatesBpsIn []cosmos.Uint) (affiliatesBps []cosmos.Uint, totalAffBps cosmos.Uint, err error) {
	totalAffBps = cosmos.ZeroUint()
	affiliatesBps = affiliatesBpsIn
	if keeper != nil {
		// verify all mayanames and/or addresses
		for _, name := range affiliatesNames {
			// check if address is valid
			_, err := FetchAddress(ctx, keeper, name, common.BASEChain)
			if err != nil {
				return []cosmos.Uint{}, cosmos.ZeroUint(), fmt.Errorf("invalid affiliate mayaname or address: %w", err)
			}
		}
	}

	// if only one aff bps defined, apply to all address affiliates
	if len(affiliatesBps) == 1 && len(affiliatesNames) > 1 {
		fee := affiliatesBps[0]
		affiliatesBps = make([]cosmos.Uint, len(affiliatesNames))
		affiliatesBps[0] = fee
		for i := 1; i < len(affiliatesNames); i++ {
			// bpsRequired is true when we parse subaffiliates;
			// we need to set the first bps to all subaffiliates
			// as we don't use the default bps in mayaname in case of subaffiliates
			if bpsRequired || !keeper.MAYANameExists(ctx, affiliatesNames[i]) {
				// mayaname doesn't exist it is an address, use the provided fee
				affiliatesBps[i] = fee
			} else {
				mn, err := keeper.GetMAYAName(ctx, affiliatesNames[i])
				if err != nil {
					return []cosmos.Uint{}, cosmos.ZeroUint(), fmt.Errorf("fail to get MAYAName %s", affiliatesNames[i])
				}
				if mn.AffiliateBps == nil {
					// if mayaname exists but has no bps yet, use the provided bps
					affiliatesBps[i] = fee
				} else {
					// if mayaname exists, set default here, it will be populated later with bps from mayaname
					affiliatesBps[i] = types.EmptyBps
				}
			}
		}
	}

	// if no affiliates basis points provided, set all to EmptyBps, later we try to populate the affiliates bps from MAYANames
	if len(affiliatesBps) == 0 && len(affiliatesNames) > 0 && !bpsRequired {
		affiliatesBps = make([]cosmos.Uint, len(affiliatesNames))
		for i := range affiliatesBps {
			affiliatesBps[i] = types.EmptyBps
		}
	}

	// at this point the size of names and bps should be the same
	if len(affiliatesNames) != len(affiliatesBps) {
		return []cosmos.Uint{}, cosmos.ZeroUint(), fmt.Errorf("affiliate mayanames and affiliate fee bps count mismatch (%d / %d)", len(affiliatesNames), len(affiliatesBps))
	}

	// calculate the sum of all bps
	for i := range affiliatesBps {
		if affiliatesBps[i] == types.EmptyBps {
			if keeper != nil {
				if !keeper.MAYANameExists(ctx, affiliatesNames[i]) {
					return []cosmos.Uint{}, cosmos.ZeroUint(), fmt.Errorf("cannot parse '%s' as a MAYAName while empty affiliate basis points provided at index %d", affiliatesNames[i], i)
				}
				mn, err := keeper.GetMAYAName(ctx, affiliatesNames[i])
				if err != nil {
					return []cosmos.Uint{}, cosmos.ZeroUint(), fmt.Errorf("fail to get MAYAName %s", affiliatesNames[i])
				}
				affiliatesBps[i] = mn.GetAffiliateBps()
			}
		}
		totalAffBps = totalAffBps.Add(affiliatesBps[i])
	}

	// verify the sum of all affiliates bps
	if !totalBpsMax.IsZero() && totalAffBps.GT(totalBpsMax) {
		return []cosmos.Uint{}, cosmos.ZeroUint(), fmt.Errorf("total affiliate fee basis points must not exceed %s (provided total bps: %s)", totalBpsMax, totalAffBps)
	}

	return
}

func getMultipleAffiliatesAndBpsV1(ctx cosmos.Context, keeper keeper.Keeper, bpsRequired bool, totalBpsMax cosmos.Uint, affiliatesNames []string, affiliatesBpsIn []cosmos.Uint) (affiliatesBps []cosmos.Uint, totalAffBps cosmos.Uint, err error) {
	totalAffBps = cosmos.ZeroUint()
	affiliatesBps = affiliatesBpsIn
	if keeper != nil {
		// verify all mayanames and/or addresses
		for _, name := range affiliatesNames {
			// check if address is valid
			_, err := FetchAddress(ctx, keeper, name, common.BASEChain)
			if err != nil {
				return []cosmos.Uint{}, cosmos.ZeroUint(), fmt.Errorf("invalid affiliate mayaname or address: %w", err)
			}
		}
	}

	// if only one aff bps defined, apply to all address affiliates
	if len(affiliatesBps) == 1 && len(affiliatesNames) > 1 {
		fee := affiliatesBps[0]
		affiliatesBps = make([]cosmos.Uint, len(affiliatesNames))
		affiliatesBps[0] = fee
		for i := 1; i < len(affiliatesNames); i++ {
			// bpsRequired is true when we parse subaffiliates;
			// we need to set the first bps to all subaffiliates
			// as we don't use the default bps in mayaname in case of subaffiliates
			if bpsRequired || !keeper.MAYANameExists(ctx, affiliatesNames[i]) {
				// mayaname doesn't exist it is an address, use the provided fee
				affiliatesBps[i] = fee
			} else {
				// if mayaname exists, set default here, it will be populated later with bps from mayaname
				affiliatesBps[i] = types.EmptyBps
			}
		}
	}

	// if no affiliates basis points provided, set all to EmptyBps, later we try to populate the affiliates bps from MAYANames
	if len(affiliatesBps) == 0 && len(affiliatesNames) > 0 && !bpsRequired {
		affiliatesBps = make([]cosmos.Uint, len(affiliatesNames))
		for i := range affiliatesBps {
			affiliatesBps[i] = types.EmptyBps
		}
	}

	// at this point the size of names and bps should be the same
	if len(affiliatesNames) != len(affiliatesBps) {
		return []cosmos.Uint{}, cosmos.ZeroUint(), fmt.Errorf("affiliate mayanames and affiliate fee bps count mismatch (%d / %d)", len(affiliatesNames), len(affiliatesBps))
	}

	// calculate the sum of all bps
	for i := range affiliatesBps {
		if affiliatesBps[i] == types.EmptyBps {
			if keeper != nil {
				if !keeper.MAYANameExists(ctx, affiliatesNames[i]) {
					return []cosmos.Uint{}, cosmos.ZeroUint(), fmt.Errorf("cannot parse '%s' as a MAYAName while empty affiliate basis points provided at index %d", affiliatesNames[i], i)
				}
				mn, err := keeper.GetMAYAName(ctx, affiliatesNames[i])
				if err != nil {
					return []cosmos.Uint{}, cosmos.ZeroUint(), fmt.Errorf("fail to get MAYAName %s", affiliatesNames[i])
				}
				affiliatesBps[i] = mn.GetAffiliateBps()
			}
		}
		totalAffBps = totalAffBps.Add(affiliatesBps[i])
	}

	// verify the sum of all affiliates bps
	if !totalBpsMax.IsZero() && totalAffBps.GT(totalBpsMax) {
		return []cosmos.Uint{}, cosmos.ZeroUint(), fmt.Errorf("total affiliate fee basis points must not exceed %s (provided total bps: %s)", totalBpsMax, totalAffBps)
	}

	return
}

// TODO: remove when we have keeper constants
func (p *parser) getConfigInt64(key constants.ConstantName) int64 {
	val := int64(-1)
	var err error
	if p.keeper != nil {
		val, err = p.keeper.GetMimir(p.ctx, key.String())
		if err != nil {
			p.ctx.Logger().Error("fail to fetch mimir value", "key", key.String(), "error", err)
		}
	}
	if val < 0 || err != nil {
		val = constants.GetConstantValues(p.version).GetInt64Value(key)
	}
	return val
}

func getAffiliatesMemoString(affiliates []string, affiliatesBasisPoints []cosmos.Uint) (string, string) {
	// affiliates
	mns := make([]string, len(affiliates))
	copy(mns, affiliates)
	for i, aff := range affiliates {
		// shorten the addresses, if possible
		affiliateAddress, err := common.NewAddress(aff, common.LatestVersion)
		if err == nil && !affiliateAddress.IsEmpty() {
			mns[i] = affiliateAddress.AbbreviatedString(common.LatestVersion)
		}
	}

	// bps
	affbps := make([]string, len(affiliatesBasisPoints))
	for i, bps := range affiliatesBasisPoints {
		// set empty string if bps is EmptyBps
		if bps != types.EmptyBps {
			affbps[i] = bps.String()
		}
	}

	affString := strings.Join(mns, "/")
	affBpsString := strings.Join(affbps, "/")

	return affString, affBpsString
}
