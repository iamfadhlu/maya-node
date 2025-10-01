package mayachain

import (
	"fmt"

	"github.com/blang/semver"
	"github.com/hashicorp/go-multierror"
	tmtypes "github.com/tendermint/tendermint/types"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

// TradeAccountWithdrawalHandler is handler to process MsgTradeAccountWithdrawal
type TradeAccountWithdrawalHandler struct {
	mgr Manager
}

// NewTradeAccountWithdrawalHandler create a new instance of TradeAccountWithdrawalHandler
func NewTradeAccountWithdrawalHandler(mgr Manager) TradeAccountWithdrawalHandler {
	return TradeAccountWithdrawalHandler{
		mgr: mgr,
	}
}

// Run is the main entry point for TradeAccountWithdrawalHandler
func (h TradeAccountWithdrawalHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgTradeAccountWithdrawal)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("MsgTradeAccountWithdrawal failed validation", "error", err)
		return nil, err
	}
	err := h.handle(ctx, *msg)
	if err != nil {
		ctx.Logger().Error("fail to process MsgTradeAccountWithdrawal", "error", err)
	}
	return &cosmos.Result{}, err
}

func (h TradeAccountWithdrawalHandler) validate(ctx cosmos.Context, msg MsgTradeAccountWithdrawal) error {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.123.0")): // trade-accounts
		return h.validateV123(ctx, msg)
	default:
		return errBadVersion
	}
}

func (h TradeAccountWithdrawalHandler) validateV123(ctx cosmos.Context, msg MsgTradeAccountWithdrawal) error {
	tradeAccountsEnabled := h.mgr.Keeper().GetConfigInt64(ctx, constants.TradeAccountsEnabled)
	tradeAccountsWithdrawEnabled := h.mgr.Keeper().GetConfigInt64(ctx, constants.TradeAccountsWithdrawEnabled)
	if tradeAccountsEnabled <= 0 {
		return fmt.Errorf("trade accounts are disabled")
	}
	if tradeAccountsWithdrawEnabled <= 0 {
		ctx.Logger().Debug("Trade account deposits are disabled")
		return fmt.Errorf("trade accounts withdrawals are disabled")
	}
	return msg.ValidateBasicVersioned(h.mgr.GetVersion())
}

func (h TradeAccountWithdrawalHandler) handle(ctx cosmos.Context, msg MsgTradeAccountWithdrawal) error {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.123.0")): // trade-accounts
		return h.handleV123(ctx, msg)
	default:
		return errBadVersion
	}
}

// handle process MsgTradeAccountWithdrawal
func (h TradeAccountWithdrawalHandler) handleV123(ctx cosmos.Context, msg MsgTradeAccountWithdrawal) error {
	withdraw, err := h.mgr.TradeAccountManager().Withdrawal(ctx, msg.Asset, msg.Amount, msg.Signer, msg.AssetAddress, msg.Tx.ID)
	if err != nil {
		return err
	}
	if withdraw.IsZero() {
		return fmt.Errorf("nothing to withdraw")
	}

	var ok bool
	layer1Asset := msg.Asset.GetLayer1Asset()

	hash := tmtypes.Tx(ctx.TxBytes()).Hash()
	txID, err := common.NewTxID(fmt.Sprintf("%X", hash))
	if err != nil {
		return err
	}

	if err != nil {
		return err
	}
	toi := TxOutItem{
		Chain:     layer1Asset.GetChain(),
		InHash:    txID,
		ToAddress: msg.AssetAddress,
		Coin:      common.NewCoin(layer1Asset, withdraw),
	}

	ok, err = h.mgr.TxOutStore().TryAddTxOutItem(ctx, h.mgr, toi, cosmos.ZeroUint())
	if err != nil {
		return multierror.Append(errFailAddOutboundTx, err)
	}
	if !ok {
		return errFailAddOutboundTx
	}

	return nil
}
