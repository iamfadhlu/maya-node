package mayachain

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"

	"github.com/blang/semver"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

var IsValidMAYANameV1 = regexp.MustCompile(`^[a-zA-Z0-9+_-]+$`).MatchString

// ManageMAYANameHandler a handler to process MsgNetworkFee messages
type ManageMAYANameHandler struct {
	mgr Manager
}

// NewManageMAYANameHandler create a new instance of network fee handler
func NewManageMAYANameHandler(mgr Manager) ManageMAYANameHandler {
	return ManageMAYANameHandler{mgr: mgr}
}

// Run is the main entry point for network fee logic
func (h ManageMAYANameHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgManageMAYAName)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("MsgManageMAYAName failed validation", "error", err)
		return nil, err
	}
	result, err := h.handle(ctx, *msg)
	if err != nil {
		ctx.Logger().Error("fail to process MsgManageMAYAName", "error", err)
	}
	return result, err
}

func (h ManageMAYANameHandler) validate(ctx cosmos.Context, msg MsgManageMAYAName) error {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.118.0")):
		return h.validateV118(ctx, msg)
	case version.GTE(semver.MustParse("1.112.0")):
		return h.validateV112(ctx, msg)
	case version.GTE(semver.MustParse("0.1.0")):
		return h.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (h ManageMAYANameHandler) validateNameV1(n string) error {
	// validate MAYAName
	if len(n) > 30 {
		return errors.New("MAYAName cannot exceed 30 characters")
	}
	if !IsValidMAYANameV1(n) {
		return errors.New("invalid MAYAName")
	}
	return nil
}

func (h ManageMAYANameHandler) validateV118(ctx cosmos.Context, msg MsgManageMAYAName) error {
	if err := msg.ValidateBasicV112(h.mgr.GetVersion()); err != nil {
		return err
	}
	exists := h.mgr.Keeper().MAYANameExists(ctx, msg.Name)
	var mn types.MAYAName
	var err error
	if !exists {
		// mayaname doesn't appear to exist, let's validate the name
		if err = h.validateNameV1(msg.Name); err != nil {
			return err
		}
		registrationFee := h.mgr.GetConstants().GetInt64Value(constants.TNSRegisterFee)
		if msg.Coin.Amount.LTE(cosmos.SafeUintFromInt64(registrationFee)) {
			return fmt.Errorf("not enough funds, needed registration fee %d, got %d", registrationFee, msg.Coin.Amount.Uint64())
		}
	} else {
		mn, err = h.mgr.Keeper().GetMAYAName(ctx, msg.Name)
		if err != nil {
			return err
		}
		// If this mayaname is already owned, check signer has ownership.
		if !mn.Owner.Equals(msg.Signer) {
			return fmt.Errorf("no authorization: owned by %s", mn.Owner)
		}
		// explicit height in the memo can only reduce expiration
		if mn.ExpireBlockHeight < msg.ExpireBlockHeight {
			return errors.New("cannot artificially inflate expire block height")
		}
	}

	// validate preferred asset pool exists and is active except when preferred asset is cacao
	if !msg.PreferredAsset.IsEmpty() && !msg.PreferredAsset.IsNative() {
		// alias must exist for preferred asset or must be set with this msg
		if mn.GetAlias(msg.PreferredAsset.GetChain()).IsEmpty() && !msg.Chain.Equals(msg.PreferredAsset.GetChain()) {
			return fmt.Errorf("alias for preferred asset %s not set", msg.PreferredAsset)
		}
		if !h.mgr.Keeper().PoolExist(ctx, msg.PreferredAsset) {
			return fmt.Errorf("pool %s does not exist", msg.PreferredAsset)
		}
		pool, err2 := h.mgr.Keeper().GetPool(ctx, msg.PreferredAsset)
		if err2 != nil {
			return err2
		}
		if pool.Status != PoolAvailable {
			return fmt.Errorf("pool %s is not available", msg.PreferredAsset)
		}
	}

	maxAffiliateFeeBasisPoints := uint64(h.mgr.Keeper().GetConfigInt64(ctx, constants.MaxAffiliateFeeBasisPoints))
	if !msg.AffiliateBps.Equal(EmptyBps) && msg.AffiliateBps.GT(cosmos.NewUint(maxAffiliateFeeBasisPoints)) {
		return fmt.Errorf("affiliate fee basis points must not exceed %d", maxAffiliateFeeBasisPoints)
	}

	// verify new subaffiliate names
	subaffiliateNameExists := make(map[string]bool)
	for _, subaffiliateName := range msg.SubaffiliateName {
		// subaffiliate mayaname must exist or it must be a valid maya address
		_, err = FetchAddress(ctx, h.mgr.Keeper(), subaffiliateName, common.BASEChain)
		if err != nil {
			return fmt.Errorf("invalid affiliate mayaname or address: %w", err)
		}
		subaffiliateNameExists[subaffiliateName] = true

		// verify there are no circular sub-affiliate references
		if h.mgr.Keeper().MAYANameExists(ctx, subaffiliateName) {
			var subMn MAYAName
			subMn, err = h.mgr.Keeper().GetMAYAName(ctx, subaffiliateName)
			if err != nil {
				return fmt.Errorf("failed to get mayaname %s, error: %w", subaffiliateName, err)
			}
			var cycled bool
			if cycled, err = h.isCycled(ctx, msg.Name, subMn); err != nil {
				return fmt.Errorf("failed to verify cycling on mayaname %s, error: %w", subMn.Name, err)
			}
			if cycled {
				return fmt.Errorf("circular reference detected in mayaname hierarchy for sub-affiliate: %s", subMn.Name)
			}
		}
	}

	// first, calculate the sum of the existing sub-affiliates, excluding those whose names are in the set of new sub-affiliates
	sum := cosmos.ZeroUint()
	if exists {
		for _, sub := range mn.Subaffiliates {
			if !subaffiliateNameExists[sub.Name] {
				sum = sum.Add(sub.Bps)
			}
		}
	}

	// now add the new sub-affiliates bps'
	for _, subaffiliateBps := range msg.SubaffiliateBps {
		sum = sum.Add(subaffiliateBps)
	}

	// verify whether sum of all subaffiliate bps don't exceed 100%
	if sum.GT(cosmos.NewUint(constants.MaxBasisPts)) {
		return fmt.Errorf("the total basis points for subaffiliates must not exceed %d", constants.MaxBasisPts)
	}

	return nil
}

// handle process MsgManageMAYAName
func (h ManageMAYANameHandler) handle(ctx cosmos.Context, msg MsgManageMAYAName) (*cosmos.Result, error) {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.118.0")):
		return h.handleV118(ctx, msg)
	case version.GTE(semver.MustParse("1.112.0")):
		return h.handleV112(ctx, msg)
	case version.GTE(semver.MustParse("0.1.0")):
		return h.handleV1(ctx, msg)
	}
	return nil, errBadVersion
}

// handle process MsgManageMAYAName
func (h ManageMAYANameHandler) handleV118(ctx cosmos.Context, msg MsgManageMAYAName) (*cosmos.Result, error) {
	var err error

	enable, _ := h.mgr.Keeper().GetMimir(ctx, "MAYANames")
	if enable == 0 {
		return nil, fmt.Errorf("MAYANames are currently disabled")
	}

	mn := MAYAName{Name: msg.Name, Owner: msg.Signer, PreferredAsset: common.EmptyAsset}
	exists := h.mgr.Keeper().MAYANameExists(ctx, msg.Name)
	if exists {
		mn, err = h.mgr.Keeper().GetMAYAName(ctx, msg.Name)
		if err != nil {
			return nil, err
		}
	}

	registrationFeePaid := cosmos.ZeroUint()
	fundPaid := cosmos.ZeroUint()

	// check if user is trying to extend expiration
	if !msg.Coin.Amount.IsZero() {
		// check that MAYAName is still valid, can't top up an invalid MAYAName
		if err = h.validateNameV1(msg.Name); err != nil {
			return nil, err
		}
		var addBlocks int64
		// registration fee is for BASEChain addresses only
		if !exists {
			// minus registration fee
			registrationFee := h.mgr.Keeper().GetConfigInt64(ctx, constants.TNSRegisterFee)
			msg.Coin.Amount = common.SafeSub(msg.Coin.Amount, cosmos.NewUint(uint64(registrationFee)))
			registrationFeePaid = cosmos.NewUint(uint64(registrationFee))
			addBlocks = h.mgr.GetConstants().GetInt64Value(constants.BlocksPerYear) // registration comes with 1 free year
		}
		feePerBlock := h.mgr.Keeper().GetConfigInt64(ctx, constants.TNSFeePerBlock)
		fundPaid = msg.Coin.Amount
		addBlocks += (int64(msg.Coin.Amount.Uint64()) / feePerBlock)
		if mn.ExpireBlockHeight < ctx.BlockHeight() {
			mn.ExpireBlockHeight = ctx.BlockHeight() + addBlocks
		} else {
			mn.ExpireBlockHeight += addBlocks
		}
	}

	// check if we need to reduce the expire time, upon user request
	if msg.ExpireBlockHeight > 0 && msg.ExpireBlockHeight < mn.ExpireBlockHeight {
		mn.ExpireBlockHeight = msg.ExpireBlockHeight
	}

	shouldTriggerPreferredAssetSwap := false
	// check if we need to update the preferred asset
	if (mn.PreferredAsset.IsNativeBase() || !mn.PreferredAsset.Equals(msg.PreferredAsset)) && !msg.PreferredAsset.IsEmpty() {
		if msg.PreferredAsset.IsNativeBase() {
			// if preferred asset is cacao then clear the preferred asset (fees will be sent directly to native alias)
			mn.PreferredAsset = common.EmptyAsset
			// send any remaining funds from affiliate collector to the cacao alias (or to the owner)
			ctx.Logger().Info(fmt.Sprintf("Releasing affiliate collector on preferred asset delete for %s", mn.Name))
			if err = releaseAffiliateCollector(ctx, h.mgr, mn); err != nil {
				return nil, err
			}
		} else {
			// if previous preferred asset is cacao or empty,
			// try to trigger preferred asset swap
			// as aff fees could have been accumulated in V112
			// if cacao was set as preferred asset pre V112
			if mn.PreferredAsset.IsNativeBase() || mn.PreferredAsset.IsEmpty() {
				ctx.Logger().Info(fmt.Sprintf("Releasing affiliate collector on preferred asset change (from MAYA.CACAO to %s)  for %s", msg.PreferredAsset, mn.Name))
				if err = releaseAffiliateCollector(ctx, h.mgr, mn); err != nil {
					return nil, err
				}
				shouldTriggerPreferredAssetSwap = true
			}
			mn.PreferredAsset = msg.PreferredAsset
		}
	}

	// update address if provided
	if !msg.Chain.IsEmpty() {
		mn.SetAlias(msg.Chain, msg.Address)
	}

	// Update owner if it has changed
	// Also, if owner has changed, null out the PreferredAsset/Aliases so the new owner is forced to reset it.
	if !msg.Owner.Empty() && !bytes.Equal(msg.Owner, mn.Owner) {
		mn.Owner = msg.Owner
		mn.PreferredAsset = common.EmptyAsset
		mn.Aliases = []MAYANameAlias{}
	}

	// check if we need to change the default affiliate fee bps
	if !msg.AffiliateBps.Equal(EmptyBps) && mn.GetAffiliateBps() != msg.AffiliateBps {
		mn.SetAffiliateBps(msg.AffiliateBps)
	}

	h.mgr.Keeper().SetMAYAName(ctx, mn)

	// set the subaffiliates & subaffiliates' fees
	if len(msg.SubaffiliateName) != 0 {
		if err = h.manageSubAffiliates(ctx, msg, mn, registrationFeePaid, fundPaid); err != nil {
			return &cosmos.Result{}, err
		}
	}

	evt := NewEventMAYAName(mn.Name, msg.Chain, msg.Address, registrationFeePaid, fundPaid, mn.ExpireBlockHeight, mn.Owner, msg.AffiliateBps.BigInt().Int64(), msg.SubaffiliateName, msg.SubaffiliateBps)
	if err = h.mgr.EventMgr().EmitEvent(ctx, evt); nil != err {
		ctx.Logger().Error("fail to emit MAYAName event", "error", err)
	}

	// check if we need to update the preferred asset
	if shouldTriggerPreferredAssetSwap {
		swapindex := 0
		if err = checkAndTriggerPreferredAssetSwap(ctx, h.mgr, mn, &swapindex); err != nil {
			ctx.Logger().Error("failed to check and trigger preferred asset swap", "error", err)
		}
	}

	return &cosmos.Result{}, nil
}

func (h ManageMAYANameHandler) isCycled(ctx cosmos.Context, checkedName string, mn MAYAName) (bool, error) {
	for _, sub := range mn.Subaffiliates {
		if sub.Name == checkedName {
			return true, nil
		}
		if !h.mgr.Keeper().MAYANameExists(ctx, sub.Name) {
			continue
		}
		subMn, err := h.mgr.Keeper().GetMAYAName(ctx, sub.Name)
		if err != nil {
			return false, err
		}
		if cycled, err := h.isCycled(ctx, checkedName, subMn); cycled || err != nil {
			return cycled, err
		}
	}
	return false, nil
}

func (h ManageMAYANameHandler) manageSubAffiliates(ctx cosmos.Context, msg MsgManageMAYAName, mn types.MAYAName, registrationFeePaid, fundPaid cosmos.Uint) error {
	for index, subaffiliateName := range msg.SubaffiliateName {
		if !msg.SubaffiliateBps[index].IsZero() {
			// set the subaffiliate fee bps for subaffiliate mayaname
			if err := mn.SetSubaffiliate(subaffiliateName, msg.SubaffiliateBps[index]); err != nil {
				return err
			}
		} else {
			// SubaffiliateBps == 0 or subaffiliate doesn't exist that means remove the subaffiliate
			mn.RemoveSubaffiliate(subaffiliateName)
		}
		h.mgr.Keeper().SetMAYAName(ctx, mn)
	}
	return nil
}
