package mayachain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

// DonateHandler is to handle donate message
type DonateHandler struct {
	mgr Manager
}

// NewDonateHandler create a new instance of DonateHandler
func NewDonateHandler(mgr Manager) DonateHandler {
	return DonateHandler{
		mgr: mgr,
	}
}

// Run is the main entry point to execute donate logic
func (h DonateHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgDonate)
	if !ok {
		return nil, errInvalidMessage
	}
	ctx.Logger().Info("receive msg donate", "tx_id", msg.Tx.ID)
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("msg donate failed validation", "error", err)
		return nil, err
	}
	if err := h.handle(ctx, *msg); err != nil {
		ctx.Logger().Error("fail to process msg donate", "error", err)
		return nil, err
	}
	return &cosmos.Result{}, nil
}

func (h DonateHandler) validate(ctx cosmos.Context, msg MsgDonate) error {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("0.80.0")):
		return h.validateV80(ctx, msg)
	default:
		return errBadVersion
	}
}

// handle process MsgDonate, MsgDonate add asset and RUNE to the asset pool
// it simply increase the pool asset/RUNE balance but without taking any of the pool units
func (h DonateHandler) handle(ctx cosmos.Context, msg MsgDonate) error {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.101.0")):
		return h.handleV101(ctx, msg)
	case version.GTE(semver.MustParse("0.1.0")):
		return h.handleV1(ctx, msg)
	default:
		return errBadVersion
	}
}

func (h DonateHandler) handleV101(ctx cosmos.Context, msg MsgDonate) error {
	pool, err := h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get pool for (%s)", msg.Asset))
	}
	if pool.Asset.IsEmpty() {
		return cosmos.ErrUnknownRequest(fmt.Sprintf("pool %s not exist", msg.Asset.String()))
	}

	if isLiquidityAuction(ctx, h.mgr.Keeper()) {
		err = h.addLiquidityFromDonate(ctx, msg, pool)

		isAdmin := false
		for _, admin := range ADMINS {
			var acc cosmos.AccAddress
			acc, err = cosmos.AccAddressFromBech32(admin)
			if err != nil {
				return err
			}

			if msg.Signer.Equals(acc) {
				isAdmin = true
			}
		}

		if !isAdmin {
			return cosmos.ErrUnauthorized("only admin can donate to liquidity auction")
		}

		if err != nil {
			return ErrInternal(err, "fail to add liquidity with donate memo")
		}
	} else {
		pool.BalanceAsset = pool.BalanceAsset.Add(msg.AssetAmount)
		pool.BalanceCacao = pool.BalanceCacao.Add(msg.CacaoAmount)

		if err = h.mgr.Keeper().SetPool(ctx, pool); err != nil {
			return ErrInternal(err, fmt.Sprintf("fail to set pool(%s)", pool))
		}
	}

	// emit event
	donateEvt := NewEventDonate(pool.Asset, msg.Tx)
	if err = h.mgr.EventMgr().EmitEvent(ctx, donateEvt); err != nil {
		return cosmos.Wrapf(errFailSaveEvent, "fail to save donate events: %w", err)
	}
	return nil
}

func (h DonateHandler) addLiquidityFromDonate(ctx cosmos.Context, msg MsgDonate, pool types.Pool) error {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.102.0")):
		return h.addLiquidityFromDonateV102(ctx, msg, pool)
	default:
		return h.addLiquidityFromDonateV1(ctx, msg, pool)
	}
}

func (h DonateHandler) addLiquidityFromDonateV102(ctx cosmos.Context, msg MsgDonate, pool types.Pool) error {
	iterator := h.mgr.Keeper().GetLiquidityProviderIterator(ctx, msg.Asset)
	defer iterator.Close()

	type donate struct {
		lp    LiquidityProvider
		claim cosmos.Uint
		tier  int64
	}

	tierTotal := map[int64]cosmos.Uint{
		1: cosmos.ZeroUint(),
		2: cosmos.ZeroUint(),
		3: cosmos.ZeroUint(),
	}
	donations := make([]donate, 0)
	tier1Count := 0
	for ; iterator.Valid(); iterator.Next() {
		var lp LiquidityProvider
		h.mgr.Keeper().Cdc().MustUnmarshal(iterator.Value(), &lp)
		if !lp.PendingAsset.IsZero() && lp.NodeBondAddress.Empty() {
			tier, err := h.mgr.Keeper().GetLiquidityAuctionTier(ctx, lp.CacaoAddress)
			if err != nil {
				ctx.Logger().Error("fail to get liquidity auction tier for address", "address", lp.CacaoAddress, "pendingAsset", lp.PendingAsset.String(), "error", err)
				continue
			}

			if tier < 1 || tier > 3 {
				ctx.Logger().Error("invalid liquidity auction tier", "address", lp.CacaoAddress, "pendingAsset", lp.PendingAsset.String(), "tier", tier)
				continue
			}

			if tier == 1 {
				tier1Count++
			}

			lpShareInCacao := common.GetSafeShare(lp.PendingAsset, pool.PendingInboundAsset, msg.CacaoAmount)
			tierTotal[tier] = tierTotal[tier].Add(lpShareInCacao)

			donations = append(donations, donate{
				lp:    lp,
				tier:  tier,
				claim: lpShareInCacao,
			})
		}
	}

	tier2Share := common.GetSafeShare(cosmos.NewUint(1000), cosmos.NewUint(10000), tierTotal[2])
	tier3Share := common.GetSafeShare(cosmos.NewUint(3300), cosmos.NewUint(10000), tierTotal[3])
	tier1Share := tier2Share.Add(tier3Share)
	runeClaimed := cosmos.ZeroUint()
	totalPendingAsset := pool.PendingInboundAsset
	for _, d := range donations {
		switch d.tier {
		case h.mgr.GetConstants().GetInt64Value(constants.WithdrawTier1):
			d.claim = d.claim.Add(common.GetSafeShare(d.claim, tierTotal[1], tier1Share))
		case h.mgr.GetConstants().GetInt64Value(constants.WithdrawTier2):
			d.claim = d.claim.Sub(common.GetSafeShare(d.claim, tierTotal[2], tier2Share))
		case h.mgr.GetConstants().GetInt64Value(constants.WithdrawTier3):
			d.claim = d.claim.Sub(common.GetSafeShare(d.claim, tierTotal[3], tier3Share))
		}

		if d.claim.IsZero() {
			continue
		}

		pool.PendingInboundAsset = common.SafeSub(pool.PendingInboundAsset, d.lp.PendingAsset)
		pool.BalanceCacao = pool.BalanceCacao.Add(d.claim)
		pool.BalanceAsset = pool.BalanceAsset.Add(d.lp.PendingAsset)
		pool.LPUnits = pool.LPUnits.Add(d.claim)

		d.lp.Units = d.claim
		d.lp.CacaoDepositValue = d.claim
		d.lp.AssetDepositValue = common.GetSafeShare(d.claim, msg.CacaoAmount, totalPendingAsset)
		d.lp.PendingAsset = cosmos.ZeroUint()
		d.lp.PendingTxID = ""
		d.lp.LastAddHeight = ctx.BlockHeight()
		h.mgr.Keeper().SetLiquidityProvider(ctx, d.lp)

		runeClaimed = runeClaimed.Add(d.claim)

		evt := NewEventAddLiquidity(msg.Asset, d.claim, d.lp.CacaoAddress, d.claim, d.lp.AssetDepositValue, msg.Tx.ID, d.lp.PendingTxID, d.lp.AssetAddress)
		if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
			return ErrInternal(err, "fail to emit add liquidity event")
		}
	}

	if !runeClaimed.Equal(msg.CacaoAmount) {
		ctx.Logger().Error("rune claimed is not equal to rune amount", "runeClaimed", runeClaimed.String(), "runeAmount", msg.CacaoAmount.String())
	}

	if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
		return ErrInternal(err, "fail to save pool")
	}

	return nil
}
