package mayachain

import (
	"fmt"

	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/types"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

// IPAddressHandler is to handle ip address message
type IPAddressHandler struct {
	mgr Manager
}

// NewIPAddressHandler create new instance of IPAddressHandler
func NewIPAddressHandler(mgr Manager) IPAddressHandler {
	return IPAddressHandler{
		mgr: mgr,
	}
}

// Run it the main entry point to execute ip address logic
func (h IPAddressHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgSetIPAddress)
	if !ok {
		return nil, errInvalidMessage
	}
	ctx.Logger().Info("receive ip address", "address", msg.IPAddress)
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("msg set version failed validation", "error", err)
		return nil, err
	}
	if err := h.handle(ctx, *msg); err != nil {
		ctx.Logger().Error("fail to process msg set version", "error", err)
		return nil, err
	}

	return &cosmos.Result{}, nil
}

func (h IPAddressHandler) validate(ctx cosmos.Context, msg MsgSetIPAddress) error {
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (h IPAddressHandler) validateV1(ctx cosmos.Context, msg MsgSetIPAddress) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	nodeAccount, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.Signer)
	if err != nil {
		ctx.Logger().Error("fail to get node account", "error", err, "address", msg.Signer.String())
		return cosmos.ErrUnauthorized(fmt.Sprintf("%s is not authorizaed", msg.Signer))
	}
	if nodeAccount.IsEmpty() {
		ctx.Logger().Error("unauthorized account", "address", msg.Signer.String())
		return cosmos.ErrUnauthorized(fmt.Sprintf("%s is not authorizaed", msg.Signer))
	}
	if nodeAccount.Type != NodeTypeValidator {
		ctx.Logger().Error("unauthorized account, node account must be a validator", "address", msg.Signer.String(), "type", nodeAccount.Type)
		return cosmos.ErrUnauthorized(fmt.Sprintf("%s is not authorized", msg.Signer))
	}

	cost, err := h.mgr.Keeper().GetMimir(ctx, constants.NativeTransactionFee.String())
	if err != nil || cost < 0 {
		cost = h.mgr.GetConstants().GetInt64Value(constants.NativeTransactionFee)
	}

	coins := types.NewCoins(types.NewCoin(common.BaseNative.Native(), types.NewInt(cost)))
	if !h.mgr.Keeper().HasCoins(ctx, msg.Signer, coins) {
		return cosmos.ErrUnauthorized("not enough balance")
	}

	return nil
}

func (h IPAddressHandler) handle(ctx cosmos.Context, msg MsgSetIPAddress) error {
	ctx.Logger().Info("handleMsgSetIPAddress request", "ip address", msg.IPAddress)
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.105.0")):
		return h.handleV105(ctx, msg)
	case version.GTE(semver.MustParse("0.57.0")):
		return h.handleV57(ctx, msg)
	default:
		ctx.Logger().Error(errInvalidVersion.Error())
		return errBadVersion
	}
}

func (h IPAddressHandler) handleV105(ctx cosmos.Context, msg MsgSetIPAddress) error {
	nodeAccount, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.Signer)
	if err != nil {
		ctx.Logger().Error("fail to get node account", "error", err, "address", msg.Signer.String())
		return cosmos.ErrUnauthorized(fmt.Sprintf("unable to find account: %s", msg.Signer))
	}

	c, err := h.mgr.Keeper().GetMimir(ctx, constants.NativeTransactionFee.String())
	if err != nil || c < 0 {
		c = h.mgr.GetConstants().GetInt64Value(constants.NativeTransactionFee)
	}
	cost := cosmos.NewUint(uint64(c))
	if !cost.IsZero() {
		err = h.mgr.Keeper().SendFromAccountToModule(ctx, msg.Signer, ReserveName, common.NewCoins(common.NewCoin(common.BaseAsset(), cost)))
		if err != nil {
			return fmt.Errorf("fail to send from signer to Reserve: %w", err)
		}
	}

	nodeAccount.IPAddress = msg.IPAddress
	if err = h.mgr.Keeper().SetNodeAccount(ctx, nodeAccount); err != nil {
		return fmt.Errorf("fail to save node account: %w", err)
	}

	tx := common.Tx{}
	tx.ID = common.BlankTxID
	tx.FromAddress = nodeAccount.BondAddress
	if err = h.mgr.EventMgr().EmitBondEvent(ctx, h.mgr, common.BaseNative, cost, BondCost, tx); err != nil {
		return fmt.Errorf("fail to emit bond event: %w", err)
	}

	ctx.EventManager().EmitEvent(
		cosmos.NewEvent("set_ip_address",
			cosmos.NewAttribute("maya_address", msg.Signer.String()),
			cosmos.NewAttribute("address", msg.IPAddress)))

	return nil
}
