package mayachain

import (
	"fmt"

	"github.com/armon/go-metrics"
	"github.com/cosmos/cosmos-sdk/telemetry"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

func (h CacaoPoolWithdrawHandler) handleV118(ctx cosmos.Context, msg MsgCacaoPoolWithdraw) error {
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
	affiliateAmount := cosmos.ZeroUint()

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

	// send the affiliate fee
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

	withdrawEvent := NewEventCACAOPoolWithdrawV118(
		cacaoProvider.CacaoAddress,
		int64(msg.BasisPoints.Uint64()),
		withdrawAmount,
		withdrawUnits,
		msg.Tx.ID,
		common.NoAddress, 0, cosmos.ZeroUint(),
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
