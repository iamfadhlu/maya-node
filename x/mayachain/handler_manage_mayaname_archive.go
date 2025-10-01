package mayachain

import (
	"bytes"
	"errors"
	"fmt"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

// validate V1 MsgManageMAYAName
func (h ManageMAYANameHandler) validateV1(ctx cosmos.Context, msg MsgManageMAYAName) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	exists := h.mgr.Keeper().MAYANameExists(ctx, msg.Name)

	if !exists {
		// mayaname doesn't appear to exist, let's validate the name
		if err := h.validateNameV1(msg.Name); err != nil {
			return err
		}
		registrationFee := h.mgr.GetConstants().GetInt64Value(constants.TNSRegisterFee)
		if msg.Coin.Amount.LTE(cosmos.NewUint(uint64(registrationFee))) {
			return fmt.Errorf("not enough funds")
		}
	} else {
		name, err := h.mgr.Keeper().GetMAYAName(ctx, msg.Name)
		if err != nil {
			return err
		}

		// if this mayaname is already owned, check signer has ownership. If
		// expiration is past, allow different user to take ownership
		if !name.Owner.Equals(msg.Signer) && ctx.BlockHeight() <= name.ExpireBlockHeight {
			ctx.Logger().Error("no authorization", "owner", name.Owner)
			return fmt.Errorf("no authorization: owned by %s", name.Owner)
		}

		// ensure user isn't inflating their expire block height artificaially
		if name.ExpireBlockHeight < msg.ExpireBlockHeight {
			return errors.New("cannot artificially inflate expire block height")
		}
	}

	return nil
}

// handle process MsgManageMAYAName
func (h ManageMAYANameHandler) handleV1(ctx cosmos.Context, msg MsgManageMAYAName) (*cosmos.Result, error) {
	var err error

	enable, _ := h.mgr.Keeper().GetMimir(ctx, "MAYANames")
	if enable == 0 {
		return nil, fmt.Errorf("MAYANames are currently disabled")
	}

	tn := MAYAName{Name: msg.Name, Owner: msg.Signer, PreferredAsset: common.EmptyAsset}
	exists := h.mgr.Keeper().MAYANameExists(ctx, msg.Name)
	if exists {
		tn, err = h.mgr.Keeper().GetMAYAName(ctx, msg.Name)
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
			registrationFee := fetchConfigInt64(ctx, h.mgr, constants.TNSRegisterFee)
			msg.Coin.Amount = common.SafeSub(msg.Coin.Amount, cosmos.NewUint(uint64(registrationFee)))
			registrationFeePaid = cosmos.NewUint(uint64(registrationFee))
			addBlocks = h.mgr.GetConstants().GetInt64Value(constants.BlocksPerYear) // registration comes with 1 free year
		}
		feePerBlock := fetchConfigInt64(ctx, h.mgr, constants.TNSFeePerBlock)
		fundPaid = msg.Coin.Amount
		addBlocks += (int64(msg.Coin.Amount.Uint64()) / feePerBlock)
		if tn.ExpireBlockHeight < ctx.BlockHeight() {
			tn.ExpireBlockHeight = ctx.BlockHeight() + addBlocks
		} else {
			tn.ExpireBlockHeight += addBlocks
		}
	}

	// check if we need to reduce the expire time, upon user request
	if msg.ExpireBlockHeight > 0 && msg.ExpireBlockHeight < tn.ExpireBlockHeight {
		tn.ExpireBlockHeight = msg.ExpireBlockHeight
	}

	// check if we need to update the preferred asset
	if !tn.PreferredAsset.Equals(msg.PreferredAsset) && !msg.PreferredAsset.IsEmpty() {
		tn.PreferredAsset = msg.PreferredAsset
	}

	tn.SetAlias(msg.Chain, msg.Address) // update address
	if !msg.Owner.Empty() {
		tn.Owner = msg.Owner // update owner
	}
	h.mgr.Keeper().SetMAYAName(ctx, tn)

	evt := NewEventMAYANameV111(tn.Name, msg.Chain, msg.Address, registrationFeePaid, fundPaid, tn.ExpireBlockHeight, tn.Owner)
	if err = h.mgr.EventMgr().EmitEvent(ctx, evt); nil != err {
		ctx.Logger().Error("fail to emit MAYAName event", "error", err)
	}

	return &cosmos.Result{}, nil
}

func (h ManageMAYANameHandler) handleV112(ctx cosmos.Context, msg MsgManageMAYAName) (*cosmos.Result, error) {
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
			registrationFee := fetchConfigInt64(ctx, h.mgr, constants.TNSRegisterFee)
			msg.Coin.Amount = common.SafeSub(msg.Coin.Amount, cosmos.NewUint(uint64(registrationFee)))
			registrationFeePaid = cosmos.NewUint(uint64(registrationFee))
			addBlocks = h.mgr.GetConstants().GetInt64Value(constants.BlocksPerYear) // registration comes with 1 free year
		}
		feePerBlock := fetchConfigInt64(ctx, h.mgr, constants.TNSFeePerBlock)
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

	// check if we need to update the preferred asset
	if !mn.PreferredAsset.Equals(msg.PreferredAsset) && !msg.PreferredAsset.IsEmpty() {
		// if preferred asset is native asset then clear the preferred asset (fees will be sent directly to native alias)
		if msg.PreferredAsset.IsNative() {
			mn.PreferredAsset = common.EmptyAsset
		} else {
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
		mn.Aliases = []types.MAYANameAlias{}
	}

	// check if we need to change the default affiliate fee bps
	if !msg.AffiliateBps.Equal(EmptyBps) && mn.GetAffiliateBps() != msg.AffiliateBps {
		mn.SetAffiliateBps(msg.AffiliateBps)
	}

	// set the subaffiliates & subaffiliates' fees
	if len(msg.SubaffiliateName) != 0 {
		err = h.manageSubAffiliates(ctx, msg, mn, registrationFeePaid, fundPaid)
		return &cosmos.Result{}, err
	}

	h.mgr.Keeper().SetMAYAName(ctx, mn)
	evt := NewEventMAYAName(mn.Name, msg.Chain, msg.Address, registrationFeePaid, fundPaid, mn.ExpireBlockHeight, mn.Owner, msg.AffiliateBps.BigInt().Int64(), msg.SubaffiliateName, msg.SubaffiliateBps)
	if err = h.mgr.EventMgr().EmitEvent(ctx, evt); nil != err {
		ctx.Logger().Error("fail to emit MAYAName event", "error", err)
	}
	return &cosmos.Result{}, nil
}

func (h ManageMAYANameHandler) validateV112(ctx cosmos.Context, msg MsgManageMAYAName) error {
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

	// validate preferred asset pool exists and is active
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

	maxAffiliateFeeBasisPoints := uint64(fetchConfigInt64(ctx, h.mgr, constants.MaxAffiliateFeeBasisPoints))
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
