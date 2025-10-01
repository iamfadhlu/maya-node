package mayachain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

// VersionHandler is to handle Version message
type VersionHandler struct {
	mgr Manager
}

// NewVersionHandler create new instance of VersionHandler
func NewVersionHandler(mgr Manager) VersionHandler {
	return VersionHandler{
		mgr: mgr,
	}
}

// Run it the main entry point to execute Version logic
func (h VersionHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgSetVersion)
	if !ok {
		return nil, errInvalidMessage
	}
	ctx.Logger().Info("receive version number", "version", msg.Version)
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

func (h VersionHandler) validate(ctx cosmos.Context, msg MsgSetVersion) error {
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.80.0")) {
		return h.validateV80(ctx, msg)
	}
	return errBadVersion
}

func (h VersionHandler) validateV80(ctx cosmos.Context, msg MsgSetVersion) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	v, err := semver.Parse(msg.Version)
	if err != nil {
		ctx.Logger().Info("invalid version", "version", msg.Version)
		return cosmos.ErrUnknownRequest(fmt.Sprintf("%s is invalid", msg.Version))
	}
	if len(v.Build) > 0 || len(v.Pre) > 0 {
		return cosmos.ErrUnknownRequest("BASEChain doesn't use Pre/Build version")
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
	if !h.mgr.Keeper().HasCoins(ctx, msg.Signer, cosmos.NewCoins(cosmos.NewCoin(common.BaseNative.Native(), cosmos.NewInt((cost))))) {
		return cosmos.ErrUnauthorized("not enough rune for transaction in node account")
	}

	return nil
}

func (h VersionHandler) handle(ctx cosmos.Context, msg MsgSetVersion) error {
	ctx.Logger().Info("handleMsgSetVersion request", "Version:", msg.Version)
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.105.0")):
		return h.handleV105(ctx, msg)
	case version.GTE(semver.MustParse("0.57.0")):
		return h.handleV57(ctx, msg)
	default:
		return errBadVersion
	}
}

func (h VersionHandler) handleV105(ctx cosmos.Context, msg MsgSetVersion) error {
	nodeAccount, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.Signer)
	if err != nil {
		return cosmos.ErrUnauthorized(fmt.Errorf("unable to find account(%s):%w", msg.Signer, err).Error())
	}

	version, err := msg.GetVersion()
	if err != nil {
		return fmt.Errorf("fail to parse version: %w", err)
	}

	if nodeAccount.GetVersion().LT(version) {
		nodeAccount.Version = version.String()
	}

	if err = h.mgr.Keeper().SetNodeAccount(ctx, nodeAccount); err != nil {
		return fmt.Errorf("fail to save node account: %w", err)
	}

	var c int64
	c, err = h.mgr.Keeper().GetMimir(ctx, constants.NativeTransactionFee.String())
	if err != nil || c < 0 {
		c = h.mgr.GetConstants().GetInt64Value(constants.NativeTransactionFee)
	}

	cost := cosmos.NewUint(uint64(c))
	coin := common.NewCoin(common.BaseNative, cost)
	if !cost.IsZero() {
		// cost has been deducted from node account's bond , thus just send the cost from bond to reserve
		if err = h.mgr.Keeper().SendFromAccountToModule(ctx, msg.Signer, ReserveName, common.NewCoins(coin)); err != nil {
			ctx.Logger().Error("unable to send gas to reserve", "error", err)
			return err
		}
	}

	ctx.EventManager().EmitEvent(
		cosmos.NewEvent("set_version",
			cosmos.NewAttribute("maya_address", msg.Signer.String()),
			cosmos.NewAttribute("version", msg.Version)))

	return nil
}
