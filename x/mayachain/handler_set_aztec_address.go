package mayachain

import (
	"fmt"

	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/types"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

// SetAztecAddressHandler process MsgSetAztecAddress
// MsgSetAztecAddress is used by operators to set their aztec address
type SetAztecAddressHandler struct {
	mgr Manager
}

// NewSetAztecAddressHandler create a new instance of SetAztecAddressHandler
func NewSetAztecAddressHandler(mgr Manager) SetAztecAddressHandler {
	return SetAztecAddressHandler{
		mgr: mgr,
	}
}

// Run is the main entry point to process MsgSetAztecAddress
func (h SetAztecAddressHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgSetAztecAddress)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("MsgSetAztecAddress failed validation", "error", err)
		return nil, err
	}
	result, err := h.handle(ctx, *msg)
	if err != nil {
		ctx.Logger().Error("fail to process MsgSetAztecAddress", "error", err)
	}
	return result, err
}

func (h SetAztecAddressHandler) validate(ctx cosmos.Context, msg MsgSetAztecAddress) error {
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("1.89.0")) {
		return h.validateV89(ctx, msg)
	}
	return errInvalidVersion
}

func (h SetAztecAddressHandler) validateV89(ctx cosmos.Context, msg MsgSetAztecAddress) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	nodeAccount, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.Signer)
	if err != nil {
		return cosmos.ErrUnauthorized(fmt.Sprintf("fail to get node account(%s):%s", msg.Signer.String(), err)) // notAuthorized
	}
	if nodeAccount.IsEmpty() {
		return cosmos.ErrUnauthorized(fmt.Sprintf("unauthorized account(%s)", msg.Signer))
	}

	cost, err := h.mgr.Keeper().GetMimir(ctx, constants.NativeTransactionFee.String())
	if err != nil || cost < 0 {
		cost = h.mgr.GetConstants().GetInt64Value(constants.NativeTransactionFee)
	}

	coins := types.NewCoins(types.NewCoin(common.BaseNative.Native(), types.NewInt(cost)))
	if !h.mgr.Keeper().HasCoins(ctx, msg.Signer, coins) {
		return cosmos.ErrUnauthorized("not enough balance")
	}

	if nodeAccount.Status == NodeDisabled {
		return fmt.Errorf("node %s is disabled, so it can't update itself", nodeAccount.NodeAddress)
	}

	if !nodeAccount.AztecAddress.IsEmpty() {
		return fmt.Errorf("node already has aztec address set: %s", nodeAccount.NodeAddress)
	}

	if err := h.mgr.Keeper().EnsureAztecAddressUnique(ctx, msg.AztecAddress); err != nil {
		return err
	}

	return nil
}

func (h SetAztecAddressHandler) handle(ctx cosmos.Context, msg MsgSetAztecAddress) (*cosmos.Result, error) {
	ctx.Logger().Info("handleMsgSetAztecAddress request")
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("1.89.0")) {
		return h.handleV89(ctx, msg)
	}
	return nil, errBadVersion
}

func (h SetAztecAddressHandler) handleV89(ctx cosmos.Context, msg MsgSetAztecAddress) (*cosmos.Result, error) {
	nodeAccount, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.Signer)
	if err != nil {
		ctx.Logger().Error("fail to get node account", "error", err, "address", msg.Signer.String())
		return nil, cosmos.ErrUnauthorized(fmt.Sprintf("%s is not authorized", msg.Signer))
	}

	c, err := h.mgr.Keeper().GetMimir(ctx, constants.NativeTransactionFee.String())
	if err != nil || c < 0 {
		c = h.mgr.GetConstants().GetInt64Value(constants.NativeTransactionFee)
	}
	cost := cosmos.NewUint(uint64(c))

	nodeAccount.UpdateStatus(NodeStandby, ctx.BlockHeight())
	err = h.mgr.Keeper().SendFromAccountToModule(ctx, msg.Signer, ReserveName, common.NewCoins(common.NewCoin(common.BaseAsset(), cost)))
	if err != nil {
		return nil, fmt.Errorf("fail to send from signer to Reserve: %w", err)
	}
	nodeAccount.AztecAddress = msg.AztecAddress
	if err := h.mgr.Keeper().SetNodeAccount(ctx, nodeAccount); err != nil {
		return nil, fmt.Errorf("fail to save node account: %w", err)
	}

	ctx.EventManager().EmitEvent(
		cosmos.NewEvent("set_aztec_address",
			cosmos.NewAttribute("node_address", msg.Signer.String()),
			cosmos.NewAttribute("aztec_address", msg.AztecAddress.String())))

	return &cosmos.Result{}, nil
}
