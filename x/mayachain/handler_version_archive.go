package mayachain

import (
	"fmt"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

func (h VersionHandler) handleV57(ctx cosmos.Context, msg MsgSetVersion) error {
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
		if err = h.mgr.Keeper().SendFromAccountToModule(ctx, nodeAccount.NodeAddress, ReserveName, common.NewCoins(coin)); err != nil {
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
