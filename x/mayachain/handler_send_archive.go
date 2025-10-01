package mayachain

import (
	"errors"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

func (h SendHandler) validateV1(ctx cosmos.Context, msg MsgSend) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	// check if we're sending to asgard, bond modules. If we are, forward to the native tx handler
	if msg.ToAddress.Equals(h.mgr.Keeper().GetModuleAccAddress(AsgardName)) || msg.ToAddress.Equals(h.mgr.Keeper().GetModuleAccAddress(BondName)) {
		return errors.New("cannot use MsgSend for Asgard or Bond transactions, use MsgDeposit instead")
	}

	return nil
}

func (h SendHandler) validateV87(ctx cosmos.Context, msg MsgSend) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	// disallow sends to modules, they should only be interacted with via deposit messages
	if msg.ToAddress.Equals(h.mgr.Keeper().GetModuleAccAddress(AsgardName)) ||
		msg.ToAddress.Equals(h.mgr.Keeper().GetModuleAccAddress(BondName)) ||
		msg.ToAddress.Equals(h.mgr.Keeper().GetModuleAccAddress(ReserveName)) ||
		msg.ToAddress.Equals(h.mgr.Keeper().GetModuleAccAddress(ModuleName)) ||
		msg.ToAddress.Equals(h.mgr.Keeper().GetModuleAccAddress(MayaFund)) {
		return errors.New("cannot use MsgSend for Module transactions, use MsgDeposit instead")
	}

	accFounders, err := cosmos.AccAddressFromBech32(FOUNDERS)
	if err != nil {
		return errors.New("fail to get founders account")
	}

	ctx.Logger().Info("validateV87", "msg.FromAddress", msg.FromAddress, "accFounders", accFounders, "msg.Amount", msg.Amount, "msg.Amount.AmountOf(common.MayaNative.Native())", msg.Amount.AmountOf(common.MayaNative.Native()), "cosmos.ZeroInt()", cosmos.ZeroInt())
	if msg.FromAddress.Equals(accFounders) && msg.Amount.AmountOf(common.MayaNative.Native()).GT(cosmos.ZeroInt()) {
		return errors.New("cannot send maya from founders address")
	}

	return nil
}

func (h SendHandler) validateV110(ctx cosmos.Context, msg MsgSend) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	// disallow sends to modules, they should only be interacted with via deposit messages
	if IsModuleAccAddress(h.mgr.Keeper(), msg.ToAddress) {
		return errors.New("cannot use MsgSend for Module transactions, use MsgDeposit instead")
	}

	accFounders, err := cosmos.AccAddressFromBech32(FOUNDERS)
	if err != nil {
		return errors.New("fail to get founders account")
	}

	ctx.Logger().Info("validateV87", "msg.FromAddress", msg.FromAddress, "accFounders", accFounders, "msg.Amount", msg.Amount, "msg.Amount.AmountOf(common.MayaNative.Native())", msg.Amount.AmountOf(common.MayaNative.Native()), "cosmos.ZeroInt()", cosmos.ZeroInt())
	if msg.FromAddress.Equals(accFounders) && msg.Amount.AmountOf(common.MayaNative.Native()).GT(cosmos.ZeroInt()) {
		return errors.New("cannot send maya from founders address")
	}

	return nil
}
