package mayachain

import (
	"errors"
	"fmt"

	"github.com/armon/go-metrics"
	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/telemetry"
	"github.com/hashicorp/go-multierror"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

// WithdrawLiquidityHandler to process withdraw requests
type WithdrawLiquidityHandler struct {
	mgr Manager
}

// NewWithdrawLiquidityHandler create a new instance of WithdrawLiquidityHandler to process withdraw request
func NewWithdrawLiquidityHandler(mgr Manager) WithdrawLiquidityHandler {
	return WithdrawLiquidityHandler{
		mgr: mgr,
	}
}

// Run is the main entry point of withdraw
func (h WithdrawLiquidityHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgWithdrawLiquidity)
	if !ok {
		return nil, errInvalidMessage
	}
	ctx.Logger().Info("receive MsgWithdrawLiquidity", "withdraw address", msg.WithdrawAddress, "withdraw basis points", msg.BasisPoints)
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("MsgWithdrawLiquidity failed validation", "error", err)
		return nil, err
	}

	result, err := h.handle(ctx, *msg)
	if err != nil {
		ctx.Logger().Error("fail to process msg withdraw", "error", err)
		return nil, err
	}
	return result, err
}

func (h WithdrawLiquidityHandler) validate(ctx cosmos.Context, msg MsgWithdrawLiquidity) error {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.118.0")):
		return h.validateV118(ctx, msg)
	case version.GTE(semver.MustParse("1.105.0")):
		return h.validateV105(ctx, msg)
	case version.GTE(semver.MustParse("1.96.0")):
		return h.validateV96(ctx, msg)
	default:
		return errBadVersion
	}
}

func (h WithdrawLiquidityHandler) validateV118(ctx cosmos.Context, msg MsgWithdrawLiquidity) error {
	if err := msg.ValidateBasic(); err != nil {
		return errWithdrawFailValidation
	}

	// check if it is a withdrawal from Savers & if saver withdrawals are enabled
	saverWithdrawalDisabled, err := h.mgr.Keeper().GetMimir(ctx, constants.SaverWithdrawalDisabled.String())
	if saverWithdrawalDisabled < 0 || err != nil {
		saverWithdrawalDisabled = 0
	}
	if saverWithdrawalDisabled != 0 && msg.Asset.IsSyntheticAsset() {
		ctx.Logger().Info("Saver withdrawals are disabled", "msg", msg)
		return errors.New("saver withdrawals are disabled")
	}

	pool, err := h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		errMsg := fmt.Sprintf("fail to get pool(%s)", msg.Asset)
		return ErrInternal(err, errMsg)
	}

	if err = pool.EnsureValidPoolStatus(&msg); err != nil {
		return multierror.Append(errInvalidPoolStatus, err)
	}

	// when ragnarok kicks off,  all pool will be set PoolStaged , the ragnarok tx's hash will be common.BlankTxID
	if pool.Status != PoolAvailable && !msg.WithdrawalAsset.IsEmpty() && !msg.Tx.ID.Equals(common.BlankTxID) {
		return fmt.Errorf("cannot specify a withdrawal asset while the pool is not available")
	}

	if isChainHalted(ctx, h.mgr, msg.Asset.Chain) || isLPPaused(ctx, msg.Asset.Chain, h.mgr) {
		return fmt.Errorf("unable to withdraw liquidity while chain is halted or paused LP actions")
	}

	lp, err := h.mgr.Keeper().GetLiquidityProvider(ctx, msg.Asset, msg.WithdrawAddress)
	if err != nil {
		return multierror.Append(errFailGetLiquidityProvider, err)
	}

	// If it's a symmetric LP withdrawing from external chain, match the pair address unless it's ragnarok
	if !msg.Tx.Chain.Equals(common.BASEChain) && msg.WithdrawAddress.IsChain(common.BASEChain, h.mgr.GetVersion()) && !msg.Tx.ID.Equals(common.BlankTxID) {
		// asymmetric LP trying symmetric withdraw from external chain
		if !lp.AssetAddress.Equals(msg.Tx.FromAddress) {
			return errWithdrawLiquidityMismatchAddr
		}
	}

	return nil
}

func (h WithdrawLiquidityHandler) handle(ctx cosmos.Context, msg MsgWithdrawLiquidity) (*cosmos.Result, error) {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.110.0")):
		return h.handleV110(ctx, msg)
	case version.GTE(semver.MustParse("1.108.0")):
		return h.handleV108(ctx, msg)
	case version.GTE(semver.MustParse("1.105.0")):
		return h.handleV105(ctx, msg)
	case version.GTE(semver.MustParse("1.95.0")):
		return h.handleV95(ctx, msg)
	default:
		return nil, errBadVersion
	}
}

func (h WithdrawLiquidityHandler) handleV110(ctx cosmos.Context, msg MsgWithdrawLiquidity) (*cosmos.Result, error) {
	lp, err := h.mgr.Keeper().GetLiquidityProvider(ctx, msg.Asset, msg.WithdrawAddress)
	if err != nil {
		return nil, multierror.Append(errFailGetLiquidityProvider, err)
	}

	runeAmt, assetAmt, impLossProtection, units, gasAsset, err := withdraw(ctx, msg, h.mgr)
	if err != nil {
		return nil, ErrInternal(err, "fail to process withdraw request")
	}

	memo := ""
	if msg.Tx.ID.Equals(common.BlankTxID) {
		// tx id is blank, must be triggered by the ragnarok protocol
		memo = NewRagnarokMemo(ctx.BlockHeight()).String()
	}

	// Thanks to CacheContext, the withdraw event can be emitted before handling outbounds,
	// since if there's a later error the event emission will not take place.
	if units.IsZero() && impLossProtection.IsZero() {
		// withdraw pending liquidity event
		runeHash := common.TxID("")
		assetHash := common.TxID("")
		if msg.Tx.Chain.Equals(common.BASEChain) {
			runeHash = msg.Tx.ID
		} else {
			assetHash = msg.Tx.ID
		}
		evt := NewEventPendingLiquidity(
			msg.Asset,
			WithdrawPendingLiquidity,
			lp.CacaoAddress,
			runeAmt,
			lp.AssetAddress,
			assetAmt,
			runeHash,
			assetHash,
		)
		if err = h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
			return nil, multierror.Append(errFailSaveEvent, err)
		}
	} else {
		withdrawEvt := NewEventWithdraw(
			msg.Asset,
			units,
			int64(msg.BasisPoints.Uint64()),
			cosmos.ZeroDec(),
			msg.Tx,
			assetAmt,
			runeAmt,
			impLossProtection,
		)
		if err = h.mgr.EventMgr().EmitEvent(ctx, withdrawEvt); err != nil {
			return nil, multierror.Append(errFailSaveEvent, err)
		}
	}

	transfer := func(coin common.Coin, addr common.Address) error {
		toi := TxOutItem{
			Chain:     coin.Asset.GetChain(),
			InHash:    msg.Tx.ID,
			ToAddress: addr,
			Coin:      coin,
			Memo:      memo,
		}
		if !gasAsset.IsZero() {
			// TODO: chain specific logic should be in a single location
			if msg.Asset.IsBNB() {
				toi.MaxGas = common.Gas{
					common.NewCoin(common.BaseAsset().GetChain().GetGasAsset(), gasAsset.QuoUint64(2)),
				}
			} else if msg.Asset.GetChain().GetGasAsset().Equals(msg.Asset) {
				toi.MaxGas = common.Gas{
					common.NewCoin(msg.Asset.GetChain().GetGasAsset(), gasAsset),
				}
			}
			toi.GasRate = int64(h.mgr.GasMgr().GetGasRate(ctx, msg.Asset.GetChain()).Uint64())
		}

		var ok bool
		ok, err = h.mgr.TxOutStore().TryAddTxOutItem(ctx, h.mgr, toi, cosmos.ZeroUint())
		if err != nil {
			return multierror.Append(errFailAddOutboundTx, err)
		}
		if !ok {
			return errFailAddOutboundTx
		}

		return nil
	}

	if !assetAmt.IsZero() {
		coin := common.NewCoin(msg.Asset, assetAmt)
		// TODO: this might be an issue for single sided/AVAX->ETH, ETH -> AVAX
		if !msg.Asset.IsNativeBase() && !lp.AssetAddress.IsChain(msg.Asset.GetChain(), h.mgr.GetVersion()) {
			if err = h.swap(ctx, msg, coin, lp.AssetAddress); err != nil {
				return nil, err
			}
		} else {
			if err = transfer(coin, lp.AssetAddress); err != nil {
				return nil, err
			}
		}
	}

	polAddress, err := h.mgr.Keeper().GetModuleAddress(ReserveName)
	if err != nil {
		return nil, err
	}

	isPolTriggered := true
	if !runeAmt.IsZero() {
		coin := common.NewCoin(common.BaseAsset(), runeAmt)
		// if the withdrawal comes from a genesis node, send it to the reserve
		if len(GenesisNodes) > 0 {
			for _, genesis := range GenesisNodes {
				var address common.Address
				address, err = common.NewAddress(genesis, h.mgr.GetVersion())
				if err != nil {
					return nil, err
				}
				if lp.CacaoAddress.Equals(address) {
					lp.CacaoAddress = polAddress
					isPolTriggered = false
					break
				}
			}
		}

		if err = transfer(coin, lp.CacaoAddress); err != nil {
			return nil, err
		}

		// if its the POL withdrawing, track rune withdrawn
		if polAddress.Equals(lp.CacaoAddress) && isPolTriggered {
			var pol ProtocolOwnedLiquidity
			pol, err = h.mgr.Keeper().GetPOL(ctx)
			if err != nil {
				return nil, err
			}
			pol.CacaoWithdrawn = pol.CacaoWithdrawn.Add(runeAmt)

			if err = h.mgr.Keeper().SetPOL(ctx, pol); err != nil {
				return nil, err
			}

			ctx.Logger().Info("POL withdrawn", "pool", msg.Asset, "rune", runeAmt)
			telemetry.IncrCounterWithLabels(
				[]string{"mayanode", "pol", "pool", "rune_withdrawn"},
				telem(runeAmt),
				[]metrics.Label{telemetry.NewLabel("pool", msg.Asset.String())},
			)
		}
	}

	// any extra rune in the transaction will be donated to reserve
	reserveCoin := msg.Tx.Coins.GetCoin(common.BaseAsset())
	if !reserveCoin.IsEmpty() {
		if err = h.mgr.Keeper().AddPoolFeeToReserve(ctx, reserveCoin.Amount); err != nil {
			ctx.Logger().Error("fail to add fee to reserve", "error", err)
			return nil, err
		}
	}

	telemetry.IncrCounterWithLabels(
		[]string{"mayanode", "withdraw", "implossprotection"},
		telem(impLossProtection),
		[]metrics.Label{telemetry.NewLabel("asset", msg.Asset.String())},
	)

	return &cosmos.Result{}, nil
}

func (h WithdrawLiquidityHandler) swap(ctx cosmos.Context, msg MsgWithdrawLiquidity, coin common.Coin, addr common.Address) error {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.110.0")):
		return h.swapV110(ctx, msg, coin, addr)
	default:
		return h.swapV93(ctx, msg, coin, addr)
	}
}

func (h WithdrawLiquidityHandler) swapV110(ctx cosmos.Context, msg MsgWithdrawLiquidity, coin common.Coin, addr common.Address) error {
	// ensure TxID does NOT have a collision with another swap, this could
	// happen if the user submits two identical loan requests in the same
	// block
	if ok := h.mgr.Keeper().HasSwapQueueItem(ctx, msg.Tx.ID, 0); ok {
		return fmt.Errorf("txn hash conflict")
	}

	// Use layer 1 asset in case msg.Asset is synthetic (i.e. savers withdraw)
	targetAsset := msg.Asset.GetLayer1Asset()

	// Get streaming swaps interval to use for synth -> native swap
	ssInterval := h.mgr.Keeper().GetConfigInt64(ctx, constants.SaversStreamingSwapsInterval)
	if ssInterval <= 0 {
		ssInterval = 0
	}

	// if the asset is in ragnarok, disable streaming withdraw
	key := "RAGNAROK-" + targetAsset.MimirString()
	ragnarok, err := h.mgr.Keeper().GetMimir(ctx, key)
	if err == nil && ragnarok > 0 {
		ssInterval = 0
	}

	target := addr.GetChain(h.mgr.GetVersion()).GetGasAsset()
	memo := fmt.Sprintf("=:%s:%s", target, addr)
	msg.Tx.Memo = memo
	msg.Tx.Coins = common.NewCoins(coin)
	swapMsg := NewMsgSwap(msg.Tx, target, addr, cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(), "", "", nil, MarketOrder, 0, uint64(ssInterval), msg.Signer)

	// sanity check swap msg
	handler := NewSwapHandler(h.mgr)
	if err := handler.validate(ctx, *swapMsg); err != nil {
		return err
	}

	if err := h.mgr.Keeper().SetSwapQueueItem(ctx, *swapMsg, 0); err != nil {
		ctx.Logger().Error("fail to add swap to queue", "error", err)
		return err
	}

	return nil
}
