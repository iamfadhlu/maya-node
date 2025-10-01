package mayachain

import (
	"github.com/blang/semver"
	se "github.com/cosmos/cosmos-sdk/types/errors"

	"gitlab.com/mayachain/mayanode/common/cosmos"
)

// ForgiveSlash is to handle Ban message
type ForgiveSlash struct {
	mgr Manager
}

// NewForgiveSlash create new instance of ForgiveSlash
func NewForgiveSlashHandler(mgr Manager) ForgiveSlash {
	return ForgiveSlash{
		mgr: mgr,
	}
}

// Run is the main entry point to execute Ban logic
func (h ForgiveSlash) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgForgiveSlash)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("msg forgive slash failed validation", "error", err)
		return nil, err
	}
	return h.handle(ctx, *msg)
}

func (h ForgiveSlash) validate(ctx cosmos.Context, msg MsgForgiveSlash) error {
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (h ForgiveSlash) validateV1(ctx cosmos.Context, msg MsgForgiveSlash) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	if !isSignedByActiveNodeAccounts(ctx, h.mgr.Keeper(), msg.GetSigners()) {
		return cosmos.ErrUnauthorized(errNotAuthorized.Error())
	}

	return nil
}

func (h ForgiveSlash) handle(ctx cosmos.Context, msg MsgForgiveSlash) (*cosmos.Result, error) {
	ctx.Logger().Info("handleMsgForgiveSlash request", "node address", msg.NodeAddress.String())
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg)
	}
	ctx.Logger().Error(errInvalidVersion.Error())
	return nil, errBadVersion
}

func (h ForgiveSlash) handleV1(ctx cosmos.Context, msg MsgForgiveSlash) (*cosmos.Result, error) {
	// If NodeAccount is specified, only vote to forgive the account.
	if msg.NodeAddress != nil {
		naToForgive, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.NodeAddress)
		if err != nil {
			err = wrapError(ctx, err, "fail to get node account")
			return nil, err
		}
		if err = naToForgive.Valid(); err != nil {
			return nil, err
		}
		switch naToForgive.Status {
		case NodeActive, NodeStandby:
			// we can forgive slash of an active or standby node
		default:
			return nil, se.Wrap(errInternal, "cannot forgive slash of a node account that is not currently active or standby")
		}

	}

	forgiver, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.Signer)
	if err != nil {
		err = wrapError(ctx, err, "failed to get forgiver node account")
		return nil, err
	}
	if err = forgiver.Valid(); err != nil {
		return nil, err
	}

	active, err := h.mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		err = wrapError(ctx, err, "fail to get list of active node accounts")
		return nil, err
	}

	forgiveSlashVoter, err := h.mgr.Keeper().GetForgiveSlashVoter(ctx, msg.NodeAddress)
	if err != nil {
		return nil, err
	}

	// If first time proposed set request proposed block height.
	if !forgiveSlashVoter.HasSigned(msg.Signer) && forgiveSlashVoter.ProposedBlockHeight == 0 {
		forgiveSlashVoter.ProposedBlockHeight = ctx.BlockHeight()
	}

	// Verify if forgive slash is still even relevant (within 24 hours of proposal).
	if forgiveSlashVoter.HasExpired(ctx) {
		// Slash is expired.
		ctx.Logger().Info("slash forgive request has expired, return")
		return &cosmos.Result{}, nil
	}

	forgiveSlashVoter.Sign(msg.Signer)
	h.mgr.Keeper().SetForgiveSlashVoter(ctx, forgiveSlashVoter)

	// Doesn't have consensus yet.
	if !forgiveSlashVoter.HasConsensus(active) {
		ctx.Logger().Info("not having consensus yet, return")
		return &cosmos.Result{}, nil
	}

	if forgiveSlashVoter.BlockHeight > 0 {
		// forgive slash already processed
		return &cosmos.Result{}, nil
	}

	// Set forgive slash.
	forgiveSlashVoter.BlockHeight = ctx.BlockHeight()
	h.mgr.Keeper().SetForgiveSlashVoter(ctx, forgiveSlashVoter)

	// Logic to DecSlash.
	targetNodeAddress := msg.GetNodeAddress()
	blockAmount := msg.Blocks.BigInt().Uint64()
	if targetNodeAddress != nil {
		h.mgr.Slasher().DecSlashPoints(ctx, int64(blockAmount), targetNodeAddress)
	} else {
		// Run DecSlash on all active validators.
		for _, ana := range active {
			h.mgr.Slasher().DecSlashPoints(ctx, int64(blockAmount), ana.NodeAddress)
		}
	}

	return &cosmos.Result{}, nil
}
