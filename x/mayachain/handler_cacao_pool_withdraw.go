package mayachain

import (
	"fmt"

	"github.com/armon/go-metrics"
	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/telemetry"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

// CacaoPoolWithdrawHandler a handler to process withdrawals from CacaoPool
type CacaoPoolWithdrawHandler struct {
	mgr Manager
}

// NewCacaoPoolWithdrawHandler create new CacaoPoolWithdrawHandler
func NewCacaoPoolWithdrawHandler(mgr Manager) CacaoPoolWithdrawHandler {
	return CacaoPoolWithdrawHandler{
		mgr: mgr,
	}
}

// Run execute the handler
func (h CacaoPoolWithdrawHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgCacaoPoolWithdraw)
	if !ok {
		return nil, errInvalidMessage
	}
	ctx.Logger().Info("receive MsgCacaoPoolWithdraw",
		"tx_id", msg.Tx.ID,
		"signer", msg.Signer,
		"basis_points", msg.BasisPoints,
		"affiliates_basis_points", msg.AffiliateBasisPoints,
		"memo", msg.Tx.Memo,
	)

	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("msg cacao pool withdraw failed validation", "error", err)
		return nil, err
	}

	err := h.handle(ctx, *msg)
	if err != nil {
		ctx.Logger().Error("fail to process msg cacao pool withdraw", "error", err)
		return nil, err
	}

	return &cosmos.Result{}, nil
}

func (h CacaoPoolWithdrawHandler) validate(ctx cosmos.Context, msg MsgCacaoPoolWithdraw) error {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.118.0")):
		return h.validateV118(ctx, msg)
	default:
		return errBadVersion
	}
}

func (h CacaoPoolWithdrawHandler) validateV118(ctx cosmos.Context, msg MsgCacaoPoolWithdraw) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	cacaoPoolEnabled := h.mgr.GetConfigInt64(ctx, constants.CACAOPoolEnabled)
	if cacaoPoolEnabled <= 0 {
		return fmt.Errorf("CACAOPool disabled")
	}
	return nil
}

func (h CacaoPoolWithdrawHandler) handle(ctx cosmos.Context, msg MsgCacaoPoolWithdraw) error {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.121.0")): // cacaopool-aff
		return h.handleV121(ctx, msg)
	case version.GTE(semver.MustParse("1.118.0")):
		return h.handleV118(ctx, msg)
	default:
		return errBadVersion
	}
}

func (h CacaoPoolWithdrawHandler) handleV121(ctx cosmos.Context, msg MsgCacaoPoolWithdraw) error {
	cacaoProvider, err := h.mgr.Keeper().GetCACAOProvider(ctx, msg.Signer)
	if err != nil {
		return fmt.Errorf("unable to GetCACAOProvider: %s", err)
	}

	// ensure the deposit has reached maturity
	depositMaturity := h.mgr.GetConfigInt64(ctx, constants.CACAOPoolDepositMaturityBlocks)
	currentBlockHeight := ctx.BlockHeight()
	blocksSinceLastDeposit := currentBlockHeight - cacaoProvider.LastDepositHeight
	if blocksSinceLastDeposit < depositMaturity {
		return fmt.Errorf("deposit reaches maturity in %d blocks", depositMaturity-blocksSinceLastDeposit)
	}

	// cacao pool tracks the reserve and pooler unit shares of pol
	cacaoPool, err := h.mgr.Keeper().GetCACAOPool(ctx)
	if err != nil {
		return fmt.Errorf("fail to get cacao pool: %s", err)
	}

	// compute withdraw units
	maxBps := cosmos.NewUint(constants.MaxBasisPts)
	withdrawUnits := common.GetSafeShare(msg.BasisPoints, maxBps, cacaoProvider.Units)

	totalCacaoPoolValue, err := cacaoPoolValue(ctx, h.mgr)
	if err != nil {
		return fmt.Errorf("fail to get cacao pool value: %w", err)
	}

	// determine the profit of the withdraw amount to share with affiliate
	withdrawYield := cosmos.ZeroUint()

	// process affiliates
	if !msg.AffiliateBasisPoints.IsZero() {
		totalUnits := cacaoPool.TotalUnits()
		currentValue := common.GetSafeShare(cacaoProvider.Units, totalUnits, totalCacaoPoolValue)
		depositRemaining := common.SafeSub(cacaoProvider.DepositAmount, cacaoProvider.WithdrawAmount)
		currentYield := common.SafeSub(currentValue, depositRemaining)
		withdrawYield = common.GetSafeShare(msg.BasisPoints, maxBps, currentYield)
	}

	// compute withdraw amount
	withdrawAmount := common.GetSafeShare(withdrawUnits, cacaoPool.TotalUnits(), totalCacaoPoolValue)

	// if insufficient pending units, reserve should enter to create space for withdraw
	pendingCacao := h.mgr.Keeper().GetRuneBalanceOfModule(ctx, CACAOPoolName)
	if withdrawAmount.GT(pendingCacao) {
		return fmt.Errorf("not enough CACAO in CACAOPool module")
	}
	// update provider and cacao pool records
	cacaoProvider.Units = common.SafeSub(cacaoProvider.Units, withdrawUnits)
	cacaoProvider.WithdrawAmount = cacaoProvider.WithdrawAmount.Add(withdrawAmount)
	cacaoProvider.LastWithdrawHeight = ctx.BlockHeight()
	h.mgr.Keeper().SetCACAOProvider(ctx, cacaoProvider)
	cacaoPool.PoolUnits = common.SafeSub(cacaoPool.PoolUnits, withdrawUnits)
	cacaoPool.CacaoWithdrawn = cacaoPool.CacaoWithdrawn.Add(withdrawAmount)
	h.mgr.Keeper().SetCACAOPool(ctx, cacaoPool)

	// process the affiliate fee
	affiliateAmount := cosmos.ZeroUint()
	if !msg.AffiliateBasisPoints.IsZero() && !withdrawYield.IsZero() {
		tx := msg.Tx
		tx.Coins = common.NewCoins(common.NewCoin(common.BaseNative, withdrawYield))
		// for cacao pool withdraw we allow affiliate fees up to 100% of the yield
		affiliateAmount, err = skimAffiliateFeesWithMaxTotal(ctx, h.mgr, tx, msg.Signer, tx.Memo, maxBps, CACAOPoolName)
		if err != nil {
			return fmt.Errorf("fail to skim affiliate fees: %w", err)
		}
		// sanity check
		if affiliateAmount.GT(withdrawYield) {
			affiliateAmount = withdrawYield
		}
	}
	userAmount := common.SafeSub(withdrawAmount, affiliateAmount)

	// send the user the withdraw
	userCoins := common.NewCoins(common.NewCoin(common.BaseNative, userAmount))
	err = h.mgr.Keeper().SendFromModuleToAccount(ctx, CACAOPoolName, msg.Signer, userCoins)
	if err != nil {
		return fmt.Errorf("fail to send user withdraw: %w", err)
	}

	ctx.Logger().Info(
		"cacaopool withdraw",
		"address", msg.Signer,
		"units", withdrawUnits,
		"amount", userAmount,
		"affiliate_amount", affiliateAmount,
	)

	withdrawEvent := NewEventCACAOPoolWithdraw(
		cacaoProvider.CacaoAddress,
		int64(msg.BasisPoints.Uint64()),
		withdrawAmount,
		withdrawUnits,
		msg.Tx.ID,
		int64(msg.AffiliateBasisPoints.Uint64()),
		affiliateAmount,
	)
	if err := h.mgr.EventMgr().EmitEvent(ctx, withdrawEvent); err != nil {
		ctx.Logger().Error("fail to emit cacao pool withdraw event", "error", err)
	}

	telemetry.IncrCounterWithLabels(
		[]string{"mayanode", "cacao_pool", "withdraw_count"},
		float32(1),
		[]metrics.Label{},
	)
	telemetry.IncrCounterWithLabels(
		[]string{"mayanode", "cacao_pool", "withdraw_amount"},
		telem(withdrawEvent.CacaoAmount),
		[]metrics.Label{},
	)

	return nil
}
