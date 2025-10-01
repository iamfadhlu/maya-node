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

// CacaoPoolDepositHandler a handler to process deposits to CacaoPool
type CacaoPoolDepositHandler struct {
	mgr Manager
}

// NewCacaoPoolDepositHandler create new CacaoPoolDepositHandler
func NewCacaoPoolDepositHandler(mgr Manager) CacaoPoolDepositHandler {
	return CacaoPoolDepositHandler{
		mgr: mgr,
	}
}

// Run execute the handler
func (h CacaoPoolDepositHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgCacaoPoolDeposit)
	if !ok {
		return nil, errInvalidMessage
	}
	ctx.Logger().Info("receive CacaoPoolDeposit",
		"tx_id", msg.Tx.ID,
		"cacao_address", msg.Signer,
		"deposit_asset", msg.Tx.Coins[0].Asset,
		"deposit_amount", msg.Tx.Coins[0].Amount,
	)

	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("msg cacao pool deposit failed validation", "error", err)
		return nil, err
	}

	err := h.handle(ctx, *msg)
	if err != nil {
		ctx.Logger().Error("fail to process msg cacao pool deposit", "error", err)
		return nil, err
	}

	return &cosmos.Result{}, nil
}

func (h CacaoPoolDepositHandler) validate(ctx cosmos.Context, msg MsgCacaoPoolDeposit) error {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.118.0")):
		return h.validateV118(ctx, msg)
	default:
		return errBadVersion
	}
}

func (h CacaoPoolDepositHandler) validateV118(ctx cosmos.Context, msg MsgCacaoPoolDeposit) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	cacaoPoolEnabled := h.mgr.GetConfigInt64(ctx, constants.CACAOPoolEnabled)
	if cacaoPoolEnabled <= 0 {
		return fmt.Errorf("CACAOPool disabled")
	}
	return nil
}

func (h CacaoPoolDepositHandler) handle(ctx cosmos.Context, msg MsgCacaoPoolDeposit) error {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.118.0")):
		return h.handleV118(ctx, msg)
	default:
		return errBadVersion
	}
}

func (h CacaoPoolDepositHandler) handleV118(ctx cosmos.Context, msg MsgCacaoPoolDeposit) error {
	// get cacao pool value before deposit
	cacaoPoolValue, err := cacaoPoolValue(ctx, h.mgr)
	if err != nil {
		return fmt.Errorf("fail to get cacao pool value: %s", err)
	}

	// send deposit to cacaopool module
	err = h.mgr.Keeper().SendFromModuleToModule(
		ctx,
		AsgardName,
		CACAOPoolName,
		common.Coins{msg.Tx.Coins[0]},
	)
	if err != nil {
		return fmt.Errorf("unable to SendFromModuleToModule: %s", err)
	}

	cacaoProvider, err := h.mgr.Keeper().GetCACAOProvider(ctx, msg.Signer)
	if err != nil {
		return fmt.Errorf("unable to GetCACAOProvider: %s", err)
	}

	cacaoProvider.LastDepositHeight = ctx.BlockHeight()
	cacaoProvider.DepositAmount = cacaoProvider.DepositAmount.Add(msg.Tx.Coins[0].Amount)

	// cacao pool tracks the reserve and pooler unit shares of pol
	cacaoPool, err := h.mgr.Keeper().GetCACAOPool(ctx)
	if err != nil {
		return fmt.Errorf("fail to get cacao pool: %s", err)
	}

	// if there are no units, this is the initial deposit
	depositUnits := msg.Tx.Coins[0].Amount

	// compute deposit units
	if !cacaoPool.TotalUnits().IsZero() {
		depositCacao := msg.Tx.Coins[0].Amount
		depositUnits = common.GetSafeShare(depositCacao, cacaoPoolValue, cacaoPool.TotalUnits())
	}

	// update the provider and cacao pool records
	cacaoProvider.Units = cacaoProvider.Units.Add(depositUnits)
	h.mgr.Keeper().SetCACAOProvider(ctx, cacaoProvider)
	cacaoPool.PoolUnits = cacaoPool.PoolUnits.Add(depositUnits)
	cacaoPool.CacaoDeposited = cacaoPool.CacaoDeposited.Add(msg.Tx.Coins[0].Amount)
	h.mgr.Keeper().SetCACAOPool(ctx, cacaoPool)

	ctx.Logger().Info(
		"cacaopool deposit",
		"address", msg.Signer,
		"units", depositUnits,
		"amount", msg.Tx.Coins[0].Amount,
	)

	depositEvent := NewEventCACAOPoolDeposit(
		cacaoProvider.CacaoAddress,
		msg.Tx.Coins[0].Amount,
		depositUnits,
		msg.Tx.ID,
	)
	if err := h.mgr.EventMgr().EmitEvent(ctx, depositEvent); err != nil {
		ctx.Logger().Error("fail to emit cacao pool deposit event", "error", err)
	}

	telemetry.IncrCounterWithLabels(
		[]string{"mayanode", "cacao_pool", "deposit_count"},
		float32(1),
		[]metrics.Label{},
	)
	telemetry.IncrCounterWithLabels(
		[]string{"mayanode", "cacao_pool", "deposit_amount"},
		telem(depositEvent.CacaoAmount),
		[]metrics.Label{},
	)

	return nil
}
