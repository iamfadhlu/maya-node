package mayachain

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/types"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

func (h SetNodeKeysHandler) handleV57(ctx cosmos.Context, msg MsgSetNodeKeys) (*cosmos.Result, error) {
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

	// Here make sure THORNode don't change the node account's bond
	nodeAccount.UpdateStatus(NodeStandby, ctx.BlockHeight())
	nodeAccount.PubKeySet = msg.PubKeySetSet
	err = h.mgr.Keeper().SendFromAccountToModule(ctx, msg.Signer, ReserveName, common.NewCoins(common.NewCoin(common.BaseAsset(), cost)))
	if err != nil {
		return nil, fmt.Errorf("fail to send from signer to Reserve: %w", err)
	}
	nodeAccount.ValidatorConsPubKey = msg.ValidatorConsPubKey
	if err := h.mgr.Keeper().SetNodeAccount(ctx, nodeAccount); err != nil {
		return nil, fmt.Errorf("fail to save node account: %w", err)
	}

	// add 10 bond to reserve
	coin := common.NewCoin(common.BaseNative, cost)
	if !cost.IsZero() {
		if err := h.mgr.Keeper().SendFromModuleToModule(ctx, BondName, ReserveName, common.NewCoins(coin)); err != nil {
			ctx.Logger().Error("fail to transfer funds from bond to reserve", "error", err)
			return nil, err
		}
	}

	tx := common.Tx{}
	tx.ID = common.BlankTxID
	tx.FromAddress = nodeAccount.BondAddress
	if err := h.mgr.EventMgr().EmitBondEvent(ctx, h.mgr, common.BaseNative, cost, BondCost, tx); err != nil {
		return nil, fmt.Errorf("fail to emit bond event: %w", err)
	}

	ctx.EventManager().EmitEvent(
		cosmos.NewEvent("set_node_keys",
			cosmos.NewAttribute("node_address", msg.Signer.String()),
			cosmos.NewAttribute("node_secp256k1_pubkey", msg.PubKeySetSet.Secp256k1.String()),
			cosmos.NewAttribute("node_ed25519_pubkey", msg.PubKeySetSet.Ed25519.String()),
			cosmos.NewAttribute("validator_consensus_pub_key", msg.ValidatorConsPubKey)))

	return &cosmos.Result{}, nil
}

func (h SetNodeKeysHandler) validateV64(ctx cosmos.Context, msg MsgSetNodeKeys) error {
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

	// You should not able to update node address when the node is in active mode
	// for example if they update observer address
	if nodeAccount.Status == NodeActive {
		return fmt.Errorf("node %s is active, so it can't update itself", nodeAccount.NodeAddress)
	}
	if nodeAccount.Status == NodeDisabled {
		return fmt.Errorf("node %s is disabled, so it can't update itself", nodeAccount.NodeAddress)
	}

	if err := h.mgr.Keeper().EnsureNodeKeysUnique(ctx, msg.ValidatorConsPubKey, msg.PubKeySetSet); err != nil {
		return err
	}

	if !nodeAccount.PubKeySet.IsEmpty() {
		return fmt.Errorf("node %s already has pubkey set assigned", nodeAccount.NodeAddress)
	}

	return nil
}
