package mayachain

import (
	"errors"
	"fmt"

	"github.com/blang/semver"
	se "github.com/cosmos/cosmos-sdk/types/errors"
	tmtypes "github.com/tendermint/tendermint/types"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

// DepositHandler is to process native messages on BASEChain
type DepositHandler struct {
	mgr Manager
}

// NewDepositHandler create a new instance of DepositHandler
func NewDepositHandler(mgr Manager) DepositHandler {
	return DepositHandler{
		mgr: mgr,
	}
}

// Run is the main entry of DepositHandler
func (h DepositHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgDeposit)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("MsgDeposit failed validation", "error", err)
		return nil, err
	}
	result, err := h.handle(ctx, *msg)
	if err != nil {
		ctx.Logger().Error("fail to process MsgDeposit", "error", err)
		return nil, err
	}
	return result, nil
}

func (h DepositHandler) validate(ctx cosmos.Context, msg MsgDeposit) error {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.111.0")):
		return h.validateV111(ctx, msg)
	case version.GTE(semver.MustParse("0.1.0")):
		return h.validateV1(ctx, msg)
	}
	return errInvalidVersion
}

func (h DepositHandler) validateV111(ctx cosmos.Context, msg MsgDeposit) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	if len(msg.Coins) != 1 {
		return errors.New("only one coin is allowed")
	}

	// TODO on hard fork move to Coin.Valid() and call that from ValidateBasic
	if err := msg.Coins[0].Asset.Valid(); err != nil {
		return fmt.Errorf("invalid coin: %w", err)
	}

	return nil
}

func (h DepositHandler) handle(ctx cosmos.Context, msg MsgDeposit) (*cosmos.Result, error) {
	ctx.Logger().Info("receive MsgDeposit", "from", msg.GetSigners()[0], "coins", msg.Coins, "memo", msg.Memo)
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.123.0")): // trade-accounts
		return h.handleV123(ctx, msg)
	case version.GTE(semver.MustParse("1.110.0")):
		return h.handleV110(ctx, msg)
	case version.GTE(semver.MustParse("1.90.0")):
		return h.handleV90(ctx, msg)
	default:
		return nil, errInvalidVersion
	}
}

func (h DepositHandler) handleV123(ctx cosmos.Context, msg MsgDeposit) (*cosmos.Result, error) {
	version := h.mgr.GetVersion()
	var haltHeight int64
	var err error

	haltHeight, err = h.mgr.Keeper().GetMimir(ctx, "HaltMAYAChain")
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
		return nil, fmt.Errorf("fail to get gas fee: %w", err)
	}

	if msg.Coins[0].Asset.IsTradeAsset() {
		balance := h.mgr.TradeAccountManager().BalanceOf(ctx, msg.Coins[0].Asset, msg.Signer)
		if msg.Coins[0].Amount.GT(balance) {
			ctx.Logger().Info("TradeAccountManager", "signer", msg.Signer, "have", balance, "needed", msg.Coins[0].Amount, "asset", msg.Coins[0].Asset)
			return nil, se.ErrInsufficientFunds
		}
		if !h.mgr.Keeper().HasCoins(ctx, msg.GetSigners()[0], cosmos.NewCoins(gasFee)) {
			return nil, cosmos.ErrInsufficientCoins(err, "insufficient funds")
		}
	} else {
		var coins cosmos.Coins
		coins, err = msg.Coins.Native()
		if err != nil {
			return nil, ErrInternal(err, "coins are not native to MAYAChain")
		}

		totalCoins := cosmos.NewCoins(gasFee).Add(coins...)
		if !h.mgr.Keeper().HasCoins(ctx, msg.GetSigners()[0], totalCoins) {
			return nil, cosmos.ErrInsufficientCoins(err, "insufficient funds")
		}
	}

	memo, _ := ParseMemoWithMAYANames(ctx, h.mgr.Keeper(), msg.Memo) // ignore err
	if memo.IsOutbound() || memo.IsInternal() {
		return nil, fmt.Errorf("cannot send inbound an outbound or internal transaction")
	}

	// Only calculate percentages if the msg is not bond, otherwise send 100% to Reserve
	switch memo.GetType() {
	case TxBond, TxUnBond, TxLeave:
		// send gas to reserve
		sdkErr := h.mgr.Keeper().SendFromAccountToModule(ctx, msg.GetSigners()[0], ReserveName, common.NewCoins(gas))
		if sdkErr != nil {
			return nil, fmt.Errorf("unable to send gas to reserve: %w", sdkErr)
		}
	default:
		// Calculate Maya Fund -->  gasFee = 90%, Maya Fund = 10%
		newGas, mayaGas := CalculateMayaFundPercentage(gas, h.mgr)

		// send gas to reserve
		sdkErr := h.mgr.Keeper().SendFromAccountToModule(ctx, msg.GetSigners()[0], ReserveName, common.NewCoins(newGas))
		if sdkErr != nil {
			return nil, fmt.Errorf("unable to send gas to reserve: %w", sdkErr)
		}

		// send corresponding fees to Maya Fund
		sdkErr = h.mgr.Keeper().SendFromAccountToModule(ctx, msg.GetSigners()[0], MayaFund, common.NewCoins(mayaGas))
		if sdkErr != nil {
			return nil, fmt.Errorf("unable to send gas to maya fund: %w", sdkErr)
		}

	}

	hash := tmtypes.Tx(ctx.TxBytes()).Hash()
	txID, err := common.NewTxID(fmt.Sprintf("%X", hash))
	if err != nil {
		return nil, fmt.Errorf("fail to get tx hash: %w", err)
	}
	from, err := common.NewAddress(msg.GetSigners()[0].String(), version)
	if err != nil {
		return nil, fmt.Errorf("fail to get from address: %w", err)
	}

	handler := NewInternalHandler(h.mgr)

	var targetModule string
	switch memo.GetType() {
	case TxBond, TxUnBond, TxLeave:
		targetModule = BondName
	case TxReserve, TxMAYAName:
		targetModule = ReserveName
	default:
		targetModule = AsgardName
	}
	coinsInMsg := msg.Coins
	if !coinsInMsg.IsEmpty() && !coinsInMsg[0].Asset.IsTradeAsset() {
		// send funds to target module
		sdkErr := h.mgr.Keeper().SendFromAccountToModule(ctx, msg.GetSigners()[0], targetModule, msg.Coins)
		if sdkErr != nil {
			return nil, sdkErr
		}
	}

	to, err := h.mgr.Keeper().GetModuleAddress(targetModule)
	if err != nil {
		return nil, fmt.Errorf("fail to get to address: %w", err)
	}

	tx := common.NewTx(txID, from, to, coinsInMsg, common.Gas{gas}, msg.Memo)
	tx.Chain = common.BASEChain

	// construct msg from memo
	txIn := ObservedTx{Tx: tx}
	txInVoter := NewObservedTxVoter(txIn.Tx.ID, []ObservedTx{txIn})
	txInVoter.Height = ctx.BlockHeight() // While FinalisedHeight may be overwritten, Height records the consensus height
	txInVoter.FinalisedHeight = ctx.BlockHeight()
	txInVoter.Tx = txIn
	h.mgr.Keeper().SetObservedTxInVoter(ctx, txInVoter)
	m, txErr := processOneTxIn(ctx, h.mgr.GetVersion(), h.mgr.Keeper(), txIn, msg.Signer)
	if txErr != nil {
		ctx.Logger().Error("fail to process native inbound tx", "error", txErr.Error(), "tx hash", tx.ID.String())
		if txIn.Tx.Coins.IsEmpty() {
			return &cosmos.Result{}, nil
		}
		if newErr := refundTx(ctx, txIn, h.mgr, CodeInvalidMemo, txErr.Error(), targetModule); nil != newErr {
			return nil, newErr
		}

		return &cosmos.Result{}, nil
	}

	// check if we've halted trading
	_, isSwap := m.(*MsgSwap)
	_, isAddLiquidity := m.(*MsgAddLiquidity)
	if isSwap || isAddLiquidity {
		if isSwap && isLiquidityAuction(ctx, h.mgr.Keeper()) {
			if newErr := refundTx(ctx, txIn, h.mgr, se.ErrUnauthorized.ABCICode(), "cannot swap, liquidity auction enabled", targetModule); nil != newErr {
				return nil, ErrInternal(newErr, "liquidity auction enabled, fail to refund")
			}
			return &cosmos.Result{}, nil
		}

		if isTradingHalt(ctx, m, h.mgr) || h.mgr.Keeper().RagnarokInProgress(ctx) {
			if txIn.Tx.Coins.IsEmpty() {
				return &cosmos.Result{}, nil
			}
			if newErr := refundTx(ctx, txIn, h.mgr, se.ErrUnauthorized.ABCICode(), "trading halted", targetModule); nil != newErr {
				return nil, ErrInternal(newErr, "trading is halted, fail to refund")
			}
			return &cosmos.Result{}, nil
		}
	}

	// if its a swap, send it to our queue for processing later
	if isSwap {
		msg, ok := m.(*MsgSwap)
		if ok {
			h.addSwap(ctx, *msg)
		}
		return &cosmos.Result{}, nil
	}

	result, err := handler(ctx, m)
	if err != nil {
		code := uint32(1)
		var e se.Error
		if errors.As(err, &e) {
			code = e.ABCICode()
		}
		if txIn.Tx.Coins.IsEmpty() {
			return &cosmos.Result{}, nil
		}
		if err = refundTx(ctx, txIn, h.mgr, code, err.Error(), targetModule); err != nil {
			return nil, fmt.Errorf("fail to refund tx: %w", err)
		}
		return &cosmos.Result{}, nil
	}
	// for those Memo that will not have outbound at all , set the observedTx to done
	if !memo.GetType().HasOutbound() {
		txInVoter.SetDone()
		h.mgr.Keeper().SetObservedTxInVoter(ctx, txInVoter)
	}
	return result, nil
}

func (h DepositHandler) addSwap(ctx cosmos.Context, msg MsgSwap) {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.112.0")):
		h.addSwapV112(ctx, msg)
	case version.GTE(semver.MustParse("0.65.0")):
		h.addSwapV65(ctx, msg)
	}
}

func (h DepositHandler) addSwapV112(ctx cosmos.Context, msg MsgSwap) {
	addSwapDirect(ctx, h.mgr, msg)
}
