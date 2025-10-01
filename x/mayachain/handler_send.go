package mayachain

import (
	"errors"
	"fmt"

	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/x/gov/types"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

// SendHandler handle MsgSend
type SendHandler struct {
	mgr Manager
}

// NewSendHandler create a new instance of SendHandler
func NewSendHandler(mgr Manager) SendHandler {
	return SendHandler{
		mgr: mgr,
	}
}

// Run the main entry point to process MsgSend
func (h SendHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgSend)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("MsgSend failed validation", "error", err)
		return nil, err
	}
	result, err := h.handle(ctx, *msg)
	if err != nil {
		ctx.Logger().Error("fail to process MsgSend", "error", err)
	}
	return result, err
}

func (h SendHandler) validate(ctx cosmos.Context, msg MsgSend) error {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.111.0")):
		return h.validateV111(ctx, msg)
	case version.GTE(semver.MustParse("1.110.0")):
		return h.validateV110(ctx, msg)
	case version.GTE(semver.MustParse("1.87.0")):
		return h.validateV87(ctx, msg)
	case version.GTE(semver.MustParse("0.1.0")):
		return h.validateV1(ctx, msg)
	default:
		return errInvalidVersion
	}
}

func (h SendHandler) validateV111(ctx cosmos.Context, msg MsgSend) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	if len(msg.Amount) != 1 {
		return errors.New("only one coin is allowed")
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

func (h SendHandler) handle(ctx cosmos.Context, msg MsgSend) (*cosmos.Result, error) {
	ctx.Logger().Info("receive MsgSend", "from", msg.FromAddress, "to", msg.ToAddress, "coins", msg.Amount)
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg)
	}
	return nil, errBadVersion
}

func (h SendHandler) handleV1(ctx cosmos.Context, msg MsgSend) (*cosmos.Result, error) {
	haltHeight, err := h.mgr.Keeper().GetMimir(ctx, "HaltBASEChain")
	if err != nil {
		return nil, fmt.Errorf("failed to get mimir setting: %w", err)
	}
	if haltHeight > 0 && ctx.BlockHeight() > haltHeight {
		return nil, fmt.Errorf("mimir has halted MAYAChain transactions")
	}

	nativeTxFee, err := h.mgr.Keeper().GetMimir(ctx, constants.NativeTransactionFee.String())
	if err != nil || nativeTxFee < 0 {
		nativeTxFee = h.mgr.GetConstants().GetInt64Value(constants.NativeTransactionFee)
	}

	gas := common.NewCoin(common.BaseNative, cosmos.NewUint(uint64(nativeTxFee)))
	gasFee, err := gas.Native()
	if err != nil {
		return nil, ErrInternal(err, "fail to get gas fee")
	}

	totalCoins := cosmos.NewCoins(gasFee).Add(msg.Amount...)

	if !h.mgr.Keeper().HasCoins(ctx, msg.FromAddress, totalCoins) {
		return nil, cosmos.ErrInsufficientCoins(err, "insufficient funds")
	}

	// Calculate Maya Fund -->  gasFee = 90%, Maya Fund = 10%
	newGas, mayaGas := CalculateMayaFundPercentage(gas, h.mgr)

	// send newGas to reserve
	sdkErr := h.mgr.Keeper().SendFromAccountToModule(ctx, msg.FromAddress, ReserveName, common.NewCoins(newGas))
	if sdkErr != nil {
		return nil, fmt.Errorf("unable to send gas to reserve: %w", sdkErr)
	}

	// send corresponding fees to Maya Fund
	sdkErr = h.mgr.Keeper().SendFromAccountToModule(ctx, msg.FromAddress, MayaFund, common.NewCoins(mayaGas))
	if sdkErr != nil {
		return nil, fmt.Errorf("unable to send gas to maya fund: %w", sdkErr)
	}

	sdkErr = h.mgr.Keeper().SendCoins(ctx, msg.FromAddress, msg.ToAddress, msg.Amount)
	if sdkErr != nil {
		return nil, sdkErr
	}

	ctx.EventManager().EmitEvent(
		cosmos.NewEvent(
			cosmos.EventTypeMessage,
			cosmos.NewAttribute(cosmos.AttributeKeyModule, types.AttributeValueCategory),
		),
	)

	return &cosmos.Result{}, nil
}
