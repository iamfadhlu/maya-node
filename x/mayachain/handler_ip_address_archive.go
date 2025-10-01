package mayachain

import (
	"fmt"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

func (h IPAddressHandler) handleV57(ctx cosmos.Context, msg MsgSetIPAddress) error {
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
	nodeAccount.IPAddress = msg.IPAddress
	err = h.mgr.Keeper().SendFromAccountToModule(ctx, msg.Signer, ReserveName, common.NewCoins(common.NewCoin(common.BaseAsset(), cost)))
	if err != nil {
		return fmt.Errorf("fail to send from signer to Reserve: %w", err)
	}
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
