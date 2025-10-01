package mayachain

import (
	"fmt"

	"gitlab.com/mayachain/mayanode/common/cosmos"
)

func (h MimirHandler) validateV95(ctx cosmos.Context, msg MsgMimir) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	if !mimirValidKeyV95(msg.Key) || len(msg.Key) > 64 {
		return cosmos.ErrUnknownRequest("invalid mimir key")
	}
	if !h.isAdmin(msg.Signer) && !isSignedByActiveNodeAccounts(ctx, h.mgr.Keeper(), msg.GetSigners()) {
		return cosmos.ErrUnauthorized(fmt.Sprintf("%s is not authorizaed", msg.Signer))
	}
	return nil
}
