package mayachain

import (
	"fmt"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

func (h DonateHandler) validateV80(ctx cosmos.Context, msg MsgDonate) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	if msg.Asset.IsSyntheticAsset() {
		ctx.Logger().Error("asset cannot be synth", "error", errInvalidMessage)
		return errInvalidMessage
	}

	if isLiquidityAuction(ctx, h.mgr.Keeper()) {
		pool, err := h.mgr.Keeper().GetPool(ctx, msg.Asset)
		if err != nil {
			return ErrInternal(err, fmt.Sprintf("fail to get pool(%s)", msg.Asset))
		}

		if pool.Status != PoolAvailable {
			return errInvalidPoolStatus
		}
	}

	return nil
}

func (h DonateHandler) addLiquidityFromDonateV1(ctx cosmos.Context, msg MsgDonate, pool types.Pool) error {
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

func (h DonateHandler) handleV1(ctx cosmos.Context, msg MsgDonate) error {
	pool, err := h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get pool for (%s)", msg.Asset))
	}
	if pool.Asset.IsEmpty() {
		return cosmos.ErrUnknownRequest(fmt.Sprintf("pool %s not exist", msg.Asset.String()))
	}

	if isLiquidityAuction(ctx, h.mgr.Keeper()) {
		err = h.addLiquidityFromDonate(ctx, msg, pool)
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
