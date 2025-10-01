package mayachain

import (
	"errors"
	"fmt"
	"strings"

	"github.com/armon/go-metrics"
	"github.com/cosmos/cosmos-sdk/telemetry"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

func (h AddLiquidityHandler) validateV118(ctx cosmos.Context, msg MsgAddLiquidity) error {
	// check if it is an addition to Savers & if saver deposits are enabled
	saverDepositDisabled, err := h.mgr.Keeper().GetMimir(ctx, constants.SaverDepositDisabled.String())
	if saverDepositDisabled < 0 || err != nil {
		saverDepositDisabled = 0
	}
	if saverDepositDisabled != 0 && msg.Asset.IsSyntheticAsset() {
		ctx.Logger().Info("Saver deposits are disabled", "msg", msg)
		return errors.New("saver deposits are disabled")
	}

	if !msg.Tx.ID.IsBlank() { // don't validate tx if internal txn
		if err = msg.ValidateBasicV112(); err != nil {
			ctx.Logger().Error(err.Error())
			return errAddLiquidityFailValidation
		}
	}

	maxAffiliateFeeBasisPoints := uint64(h.mgr.Keeper().GetConfigInt64(ctx, constants.MaxAffiliateFeeBasisPoints))
	// if AffiliateBasisPoints provided, it must not be greater than MaxAffiliateFeeBasisPoints
	if !msg.AffiliateBasisPoints.Equal(EmptyBps) && msg.AffiliateBasisPoints.GT(cosmos.NewUint(maxAffiliateFeeBasisPoints)) {
		return fmt.Errorf("affiliate fee basis points must not exceed %d", maxAffiliateFeeBasisPoints)
	}

	if msg.Asset.IsVaultAsset() {
		if !msg.Asset.GetLayer1Asset().IsGasAsset() {
			return fmt.Errorf("asset must be a gas asset for the layer1 protocol")
		}
		if !msg.AssetAddress.IsChain(msg.Asset.GetLayer1Asset().GetChain(), h.mgr.GetVersion()) {
			return fmt.Errorf("asset address must be layer1 chain")
		}
		if !msg.CacaoAmount.IsZero() {
			return fmt.Errorf("cannot deposit rune into a vault")
		}
	}

	if !msg.CacaoAddress.IsEmpty() && !msg.CacaoAddress.IsChain(common.BASEChain, h.mgr.GetVersion()) {
		ctx.Logger().Error("rune address must be BASEChain")
		return errAddLiquidityFailValidation
	}

	// check if swap meets standards
	if h.needsSwap(msg) {
		if !msg.Asset.IsVaultAsset() {
			return fmt.Errorf("swap & add liquidity is only available for synthetic pools")
		}
		if !msg.Asset.GetLayer1Asset().Equals(msg.Tx.Coins[0].Asset) {
			return fmt.Errorf("deposit asset must be the layer1 equivalent for the synthetic asset")
		}
	}

	pool, err := h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}
	if err = pool.EnsureValidPoolStatus(&msg); err != nil {
		ctx.Logger().Error("fail to check pool status", "error", err)
		return errInvalidPoolStatus
	}

	if isChainHalted(ctx, h.mgr, msg.Asset.Chain) || isLPPaused(ctx, msg.Asset.Chain, h.mgr) {
		return fmt.Errorf("unable to add liquidity while chain has paused LP actions")
	}

	liquidityPools := GetLiquidityPools(h.mgr.GetVersion())
	if liquidityPools.Contains(msg.Asset) {
		var lp LiquidityProvider
		lp, err = h.mgr.Keeper().GetLiquidityProvider(ctx, msg.Asset, msg.CacaoAddress)
		if err != nil {
			return ErrInternal(err, "failed to get LP")
		}

		// Is a liquidity bond provider. If this is set it means the pool is already available
		if !lp.NodeBondAddress.Empty() {
			// check if it's a genesis node
			for _, genesis := range GenesisNodes {
				var genAddr cosmos.AccAddress
				genAddr, err = cosmos.AccAddressFromBech32(genesis)
				if err != nil {
					return ErrInternal(err, "fail to parse genesis node address")
				}
				if genAddr.Equals(lp.NodeBondAddress) {
					ctx.Logger().Error("cannot add liquidity to genesis node", "node", lp.NodeBondAddress)
					return fmt.Errorf("cannot add liquidity to genesis node: %s", lp.NodeBondAddress)
				}
			}

			var na NodeAccount
			na, err = h.mgr.Keeper().GetNodeAccount(ctx, lp.NodeBondAddress)
			if err != nil {
				ctx.Logger().Error("failed to get node account for lbp")
				return ErrInternal(err, "failed to get node account for lbp")
			}

			if na.Status == NodeReady {
				ctx.Logger().Error("cannot add bond while node is ready status", "node", lp.NodeBondAddress)
				return fmt.Errorf("cannot add bond while node is ready status: %s", lp.NodeBondAddress)
			}

			if h.mgr.Keeper().GetConfigInt64(ctx, constants.PauseBond) > 0 {
				ctx.Logger().Error("bonding has been paused", "node", lp.NodeBondAddress)
				return fmt.Errorf("bonding has been paused")
			}

			var liquidityBond cosmos.Uint
			liquidityBond, err = h.mgr.Keeper().CalcNodeLiquidityBond(ctx, na)
			if err != nil {
				return ErrInternal(err, fmt.Sprintf("fail to calculate liquidity bond for address: %s", lp.CacaoAddress))
			}

			bond := liquidityBond.Add(msg.CacaoAmount)
			var maxBond int64
			maxBond, err = h.mgr.Keeper().GetMimir(ctx, "MaximumBondInRune")
			if maxBond > 0 && err == nil {
				maxValidatorBond := cosmos.NewUint(uint64(maxBond))
				if bond.GT(maxValidatorBond) {
					return cosmos.ErrUnknownRequest(fmt.Sprintf("too much bond, max validator bond (%s), bond(%s)", maxValidatorBond.String(), bond))
				}
			}

		}
	}

	ensureLiquidityNoLargerThanBondInt64, err := h.mgr.Keeper().GetMimir(ctx, "EnsureLiquidityNoLargerThanBond")
	if err != nil {
		ctx.Logger().Error("fail to get mimir", "error", err)
	}
	// 0 not active, 1 active, -1 use default
	var ensureLiquidityNoLargerThanBond bool
	if ensureLiquidityNoLargerThanBondInt64 < 0 {
		ensureLiquidityNoLargerThanBond = h.mgr.GetConstants().GetBoolValue(constants.StrictBondLiquidityRatio)
	} else {
		ensureLiquidityNoLargerThanBond = ensureLiquidityNoLargerThanBondInt64 == 1
	}

	// if the pool is THORChain no need to check economic security
	if msg.Asset.IsVaultAsset() || !ensureLiquidityNoLargerThanBond {
		return nil
	}

	totalLiquidity, err := h.getTotalLiquidityBase(ctx)
	if err != nil {
		return ErrInternal(err, "fail to get total liquidity CACAO")
	}

	// total liquidity RUNE after current add liquidity
	pool, err = h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}
	totalLiquidity = totalLiquidity.Add(msg.CacaoAmount)
	totalLiquidity = totalLiquidity.Add(pool.AssetValueInRune(msg.AssetAmount))
	maximumLiquidityRune, err := h.mgr.Keeper().GetMimir(ctx, constants.MaximumLiquidityCacao.String())
	if maximumLiquidityRune < 0 || err != nil {
		maximumLiquidityRune = h.mgr.GetConstants().GetInt64Value(constants.MaximumLiquidityCacao)
	}
	if maximumLiquidityRune > 0 {
		if totalLiquidity.GT(cosmos.NewUint(uint64(maximumLiquidityRune))) {
			return errAddLiquidityRUNEOverLimit
		}
	}

	coins := common.NewCoins(
		common.NewCoin(common.BaseAsset(), msg.CacaoAmount),
		common.NewCoin(msg.Asset, msg.AssetAmount),
	)

	if atTVLCap(ctx, coins, h.mgr) {
		return errAddLiquidityCACAOMoreThanBond
	}

	return nil
}

func (h AddLiquidityHandler) validateV112(ctx cosmos.Context, msg MsgAddLiquidity) error {
	if !msg.Tx.ID.IsBlank() { // don't validate tx if internal txn
		if err := msg.ValidateBasicV112(); err != nil {
			ctx.Logger().Error(err.Error())
			return errAddLiquidityFailValidation
		}
	}

	maxAffiliateFeeBasisPoints := uint64(h.mgr.Keeper().GetConfigInt64(ctx, constants.MaxAffiliateFeeBasisPoints))
	// if AffiliateBasisPoints provided, it must not be greater than MaxAffiliateFeeBasisPoints
	if !msg.AffiliateBasisPoints.Equal(EmptyBps) && msg.AffiliateBasisPoints.GT(cosmos.NewUint(maxAffiliateFeeBasisPoints)) {
		return fmt.Errorf("affiliate fee basis points must not exceed %d", maxAffiliateFeeBasisPoints)
	}

	if msg.Asset.IsVaultAsset() {
		if !msg.Asset.GetLayer1Asset().IsGasAsset() {
			return fmt.Errorf("asset must be a gas asset for the layer1 protocol")
		}
		if !msg.AssetAddress.IsChain(msg.Asset.GetLayer1Asset().GetChain(), h.mgr.GetVersion()) {
			return fmt.Errorf("asset address must be layer1 chain")
		}
		if !msg.CacaoAmount.IsZero() {
			return fmt.Errorf("cannot deposit rune into a vault")
		}
	}

	if !msg.CacaoAddress.IsEmpty() && !msg.CacaoAddress.IsChain(common.BASEChain, h.mgr.GetVersion()) {
		ctx.Logger().Error("rune address must be BASEChain")
		return errAddLiquidityFailValidation
	}

	// check if swap meets standards
	if h.needsSwap(msg) {
		if !msg.Asset.IsVaultAsset() {
			return fmt.Errorf("swap & add liquidity is only available for synthetic pools")
		}
		if !msg.Asset.GetLayer1Asset().Equals(msg.Tx.Coins[0].Asset) {
			return fmt.Errorf("deposit asset must be the layer1 equivalent for the synthetic asset")
		}
	}

	pool, err := h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}
	if err = pool.EnsureValidPoolStatus(&msg); err != nil {
		ctx.Logger().Error("fail to check pool status", "error", err)
		return errInvalidPoolStatus
	}

	if isChainHalted(ctx, h.mgr, msg.Asset.Chain) || isLPPaused(ctx, msg.Asset.Chain, h.mgr) {
		return fmt.Errorf("unable to add liquidity while chain has paused LP actions")
	}

	liquidityPools := GetLiquidityPools(h.mgr.GetVersion())
	if liquidityPools.Contains(msg.Asset) {
		var lp LiquidityProvider
		lp, err = h.mgr.Keeper().GetLiquidityProvider(ctx, msg.Asset, msg.CacaoAddress)
		if err != nil {
			return ErrInternal(err, "failed to get LP")
		}

		// Is a liquidity bond provider. If this is set it means the pool is already available
		if !lp.NodeBondAddress.Empty() {
			// check if it's a genesis node
			for _, genesis := range GenesisNodes {
				var genAddr cosmos.AccAddress
				genAddr, err = cosmos.AccAddressFromBech32(genesis)
				if err != nil {
					return ErrInternal(err, "fail to parse genesis node address")
				}
				if genAddr.Equals(lp.NodeBondAddress) {
					ctx.Logger().Error("cannot add liquidity to genesis node", "node", lp.NodeBondAddress)
					return fmt.Errorf("cannot add liquidity to genesis node: %s", lp.NodeBondAddress)
				}
			}

			var na NodeAccount
			na, err = h.mgr.Keeper().GetNodeAccount(ctx, lp.NodeBondAddress)
			if err != nil {
				ctx.Logger().Error("failed to get node account for lbp")
				return ErrInternal(err, "failed to get node account for lbp")
			}

			if na.Status == NodeReady {
				ctx.Logger().Error("cannot add bond while node is ready status", "node", lp.NodeBondAddress)
				return fmt.Errorf("cannot add bond while node is ready status: %s", lp.NodeBondAddress)
			}

			if h.mgr.Keeper().GetConfigInt64(ctx, constants.PauseBond) > 0 {
				ctx.Logger().Error("bonding has been paused", "node", lp.NodeBondAddress)
				return fmt.Errorf("bonding has been paused")
			}

			var liquidityBond cosmos.Uint
			liquidityBond, err = h.mgr.Keeper().CalcNodeLiquidityBond(ctx, na)
			if err != nil {
				return ErrInternal(err, fmt.Sprintf("fail to calculate liquidity bond for address: %s", lp.CacaoAddress))
			}

			bond := liquidityBond.Add(msg.CacaoAmount)
			var maxBond int64
			maxBond, err = h.mgr.Keeper().GetMimir(ctx, "MaximumBondInRune")
			if maxBond > 0 && err == nil {
				maxValidatorBond := cosmos.NewUint(uint64(maxBond))
				if bond.GT(maxValidatorBond) {
					return cosmos.ErrUnknownRequest(fmt.Sprintf("too much bond, max validator bond (%s), bond(%s)", maxValidatorBond.String(), bond))
				}
			}

		}
	}

	ensureLiquidityNoLargerThanBondInt64, err := h.mgr.Keeper().GetMimir(ctx, "EnsureLiquidityNoLargerThanBond")
	if err != nil {
		ctx.Logger().Error("fail to get mimir", "error", err)
	}
	// 0 not active, 1 active, -1 use default
	var ensureLiquidityNoLargerThanBond bool
	if ensureLiquidityNoLargerThanBondInt64 < 0 {
		ensureLiquidityNoLargerThanBond = h.mgr.GetConstants().GetBoolValue(constants.StrictBondLiquidityRatio)
	} else {
		ensureLiquidityNoLargerThanBond = ensureLiquidityNoLargerThanBondInt64 == 1
	}

	// if the pool is THORChain no need to check economic security
	if msg.Asset.IsVaultAsset() || !ensureLiquidityNoLargerThanBond {
		return nil
	}

	totalLiquidity, err := h.getTotalLiquidityBase(ctx)
	if err != nil {
		return ErrInternal(err, "fail to get total liquidity CACAO")
	}

	// total liquidity RUNE after current add liquidity
	pool, err = h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}
	totalLiquidity = totalLiquidity.Add(msg.CacaoAmount)
	totalLiquidity = totalLiquidity.Add(pool.AssetValueInRune(msg.AssetAmount))
	maximumLiquidityRune, err := h.mgr.Keeper().GetMimir(ctx, constants.MaximumLiquidityCacao.String())
	if maximumLiquidityRune < 0 || err != nil {
		maximumLiquidityRune = h.mgr.GetConstants().GetInt64Value(constants.MaximumLiquidityCacao)
	}
	if maximumLiquidityRune > 0 {
		if totalLiquidity.GT(cosmos.NewUint(uint64(maximumLiquidityRune))) {
			return errAddLiquidityRUNEOverLimit
		}
	}

	coins := common.NewCoins(
		common.NewCoin(common.BaseAsset(), msg.CacaoAmount),
		common.NewCoin(msg.Asset, msg.AssetAmount),
	)

	if atTVLCap(ctx, coins, h.mgr) {
		return errAddLiquidityCACAOMoreThanBond
	}

	return nil
}

func (h AddLiquidityHandler) handleV110(ctx cosmos.Context, msg MsgAddLiquidity) (errResult error) {
	// check if we need to swap before adding asset
	if h.needsSwap(msg) {
		return h.swap(ctx, msg)
	}

	pool, err := h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}

	if pool.IsEmpty() {
		ctx.Logger().Info("pool doesn't exist yet, creating a new one...", "symbol", msg.Asset.String(), "creator", msg.CacaoAddress)

		pool.Asset = msg.Asset

		defaultPoolStatus := PoolAvailable.String()
		// only set the pool to default pool status if not for gas asset on the chain
		if !pool.Asset.Equals(pool.Asset.GetChain().GetGasAsset()) &&
			!pool.Asset.IsVaultAsset() {
			defaultPoolStatus = h.mgr.GetConstants().GetStringValue(constants.DefaultPoolStatus)
		}
		pool.Status = GetPoolStatus(defaultPoolStatus)

		if err = h.mgr.Keeper().SetPool(ctx, pool); err != nil {
			return ErrInternal(err, "fail to save pool to key value store")
		}
	}

	// if the pool decimals hasn't been set, it will still be 0. If we have a
	// pool asset coin, get the decimals from that transaction. This will only
	// set the decimals once.
	if pool.Decimals == 0 {
		coin := msg.GetTx().Coins.GetCoin(pool.Asset)
		if !coin.IsEmpty() {
			if coin.Decimals > 0 {
				pool.Decimals = coin.Decimals
			}
			ctx.Logger().Info("try update pool decimals", "asset", msg.Asset, "pool decimals", pool.Decimals)
			if err = h.mgr.Keeper().SetPool(ctx, pool); err != nil {
				return ErrInternal(err, "fail to save pool to key value store")
			}
		}
	}

	// figure out if we need to stage the funds and wait for a follow on
	// transaction to commit all funds atomically. For pools of native assets
	// only, stage is always false
	stage := false
	if !msg.Asset.IsVaultAsset() {
		if !msg.AssetAddress.IsEmpty() && msg.AssetAmount.IsZero() {
			stage = true
		}
		if !msg.CacaoAddress.IsEmpty() && msg.CacaoAmount.IsZero() {
			stage = true
		}
	}

	if msg.AffiliateBasisPoints.IsZero() {
		return h.addLiquidity(
			ctx,
			msg.Asset,
			msg.CacaoAmount,
			msg.AssetAmount,
			msg.CacaoAddress,
			msg.AssetAddress,
			msg.Tx,
			stage,
			h.mgr.GetConstants(),
			msg.LiquidityAuctionTier)
	}

	// add liquidity has an affiliate fee, add liquidity for both the user and their affiliate
	affiliateRune := common.GetSafeShare(msg.AffiliateBasisPoints, cosmos.NewUint(10000), msg.CacaoAmount)
	affiliateAsset := common.GetSafeShare(msg.AffiliateBasisPoints, cosmos.NewUint(10000), msg.AssetAmount)
	userRune := common.SafeSub(msg.CacaoAmount, affiliateRune)
	userAsset := common.SafeSub(msg.AssetAmount, affiliateAsset)

	err = h.addLiquidity(
		ctx,
		msg.Asset,
		userRune,
		userAsset,
		msg.CacaoAddress,
		msg.AssetAddress,
		msg.Tx,
		stage,
		h.mgr.GetConstants(),
		msg.LiquidityAuctionTier,
	)
	if err != nil {
		return err
	}
	affiliateRuneAddress := common.NoAddress
	affiliateAssetAddress := common.NoAddress
	if msg.AffiliateAddress.IsChain(common.BASEChain, h.mgr.GetVersion()) {
		affiliateRuneAddress = msg.AffiliateAddress
	} else {
		affiliateAssetAddress = msg.AffiliateAddress
	}

	err = h.addLiquidity(
		ctx,
		msg.Asset,
		affiliateRune,
		affiliateAsset,
		affiliateRuneAddress,
		affiliateAssetAddress,
		msg.Tx,
		false,
		h.mgr.GetConstants(),
		msg.LiquidityAuctionTier,
	)
	if err != nil {
		ctx.Logger().Error("fail to add liquidity for affiliate", "address", msg.AffiliateAddress, "error", err)
		return err
	}
	return nil
}

func (h AddLiquidityHandler) validateV96(ctx cosmos.Context, msg MsgAddLiquidity) error {
	if err := msg.ValidateBasicV63(); err != nil {
		ctx.Logger().Error(err.Error())
		return errAddLiquidityFailValidation
	}

	if msg.Asset.IsVaultAsset() {
		if !msg.Asset.GetLayer1Asset().IsGasAsset() {
			return fmt.Errorf("asset must be a gas asset for the layer1 protocol")
		}
		if !msg.AssetAddress.IsChain(common.BASEChain, h.mgr.GetVersion()) {
			return fmt.Errorf("asset address must be a thor address")
		}
		if !msg.CacaoAmount.IsZero() {
			return fmt.Errorf("cannot deposit rune into a vault")
		}
	}

	if !msg.CacaoAddress.IsEmpty() && !msg.CacaoAddress.IsChain(common.BASEChain, h.mgr.GetVersion()) {
		ctx.Logger().Error("rune address must be BASEChain")
		return errAddLiquidityFailValidation
	}

	// check if swap meets standards
	if h.needsSwap(msg) {
		if !msg.Asset.IsVaultAsset() {
			return fmt.Errorf("swap & add liquidity is only available for synthetic pools")
		}
		if !msg.Asset.GetLayer1Asset().Equals(msg.Tx.Coins[0].Asset) {
			return fmt.Errorf("deposit asset must be the layer1 equivalent for the synthetic asset")
		}
	}

	pool, err := h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}
	if err = pool.EnsureValidPoolStatus(&msg); err != nil {
		ctx.Logger().Error("fail to check pool status", "error", err)
		return errInvalidPoolStatus
	}

	if isChainHalted(ctx, h.mgr, msg.Asset.Chain) || isLPPaused(ctx, msg.Asset.Chain, h.mgr) {
		return fmt.Errorf("unable to add liquidity while chain has paused LP actions")
	}

	liquidityPools := GetLiquidityPools(h.mgr.GetVersion())
	if liquidityPools.Contains(msg.Asset) {
		var lp LiquidityProvider
		lp, err = h.mgr.Keeper().GetLiquidityProvider(ctx, msg.Asset, msg.CacaoAddress)
		if err != nil {
			return ErrInternal(err, "failed to get LP")
		}

		// Is a liquidity bond provider. If this is set it means the pool is already available
		if !lp.NodeBondAddress.Empty() {
			// check if it's a genesis node
			for _, genesis := range GenesisNodes {
				var genAddr cosmos.AccAddress
				genAddr, err = cosmos.AccAddressFromBech32(genesis)
				if err != nil {
					return ErrInternal(err, "fail to parse genesis node address")
				}
				if genAddr.Equals(lp.NodeBondAddress) {
					ctx.Logger().Error("cannot add liquidity to genesis node", "node", lp.NodeBondAddress)
					return fmt.Errorf("cannot add liquidity to genesis node: %s", lp.NodeBondAddress)
				}
			}

			var na NodeAccount
			na, err = h.mgr.Keeper().GetNodeAccount(ctx, lp.NodeBondAddress)
			if err != nil {
				ctx.Logger().Error("failed to get node account for lbp")
				return ErrInternal(err, "failed to get node account for lbp")
			}

			if na.Status == NodeReady {
				ctx.Logger().Error("cannot add bond while node is ready status", "node", lp.NodeBondAddress)
				return fmt.Errorf("cannot add bond while node is ready status: %s", lp.NodeBondAddress)
			}

			if fetchConfigInt64(ctx, h.mgr, constants.PauseBond) > 0 {
				ctx.Logger().Error("bonding has been paused", "node", lp.NodeBondAddress)
				return fmt.Errorf("bonding has been paused")
			}

			var liquidityBond cosmos.Uint
			liquidityBond, err = h.mgr.Keeper().CalcNodeLiquidityBond(ctx, na)
			if err != nil {
				return ErrInternal(err, fmt.Sprintf("fail to calculate liquidity bond for address: %s", lp.CacaoAddress))
			}

			bond := liquidityBond.Add(msg.CacaoAmount)
			var maxBond int64
			maxBond, err = h.mgr.Keeper().GetMimir(ctx, "MaximumBondInRune")
			if maxBond > 0 && err == nil {
				maxValidatorBond := cosmos.NewUint(uint64(maxBond))
				if bond.GT(maxValidatorBond) {
					return cosmos.ErrUnknownRequest(fmt.Sprintf("too much bond, max validator bond (%s), bond(%s)", maxValidatorBond.String(), bond))
				}
			}

		}
	}

	ensureLiquidityNoLargerThanBondInt64, err := h.mgr.Keeper().GetMimir(ctx, "EnsureLiquidityNoLargerThanBond")
	if err != nil {
		ctx.Logger().Error("fail to get mimir", "error", err)
	}
	// 0 not active, 1 active, -1 use default
	var ensureLiquidityNoLargerThanBond bool
	if ensureLiquidityNoLargerThanBondInt64 < 0 {
		ensureLiquidityNoLargerThanBond = h.mgr.GetConstants().GetBoolValue(constants.StrictBondLiquidityRatio)
	} else {
		ensureLiquidityNoLargerThanBond = ensureLiquidityNoLargerThanBondInt64 == 1
	}
	// if the pool is THORChain no need to check economic security
	if msg.Asset.IsVaultAsset() || !ensureLiquidityNoLargerThanBond {
		return nil
	}

	totalLiquidity, err := h.getTotalLiquidityBase(ctx)
	if err != nil {
		return ErrInternal(err, "fail to get total liquidity RUNE")
	}

	// total liquidity RUNE after current add liquidity
	pool, err = h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}
	totalLiquidity = totalLiquidity.Add(msg.CacaoAmount)
	totalLiquidity = totalLiquidity.Add(pool.AssetValueInRune(msg.AssetAmount))
	maximumLiquidityRune, err := h.mgr.Keeper().GetMimir(ctx, constants.MaximumLiquidityCacao.String())
	if maximumLiquidityRune < 0 || err != nil {
		maximumLiquidityRune = h.mgr.GetConstants().GetInt64Value(constants.MaximumLiquidityCacao)
	}
	if maximumLiquidityRune > 0 {
		if totalLiquidity.GT(cosmos.NewUint(uint64(maximumLiquidityRune))) {
			return errAddLiquidityRUNEOverLimit
		}
	}

	if !ensureLiquidityNoLargerThanBond {
		return nil
	}

	// the following  only applicable for chaosnet
	totalBondRune, err := h.getTotalActiveBond(ctx)
	if err != nil {
		return ErrInternal(err, "fail to get total bond RUNE")
	}

	totalNonNodeLiquidity := common.SafeSub(totalLiquidity, totalBondRune)
	if totalNonNodeLiquidity.GT(totalBondRune) {
		ctx.Logger().Info("total non node liquidity RUNE is more than total Bond", "rune", totalNonNodeLiquidity.String(), "bond", totalBondRune.String())
		return errAddLiquidityCACAOMoreThanBond
	}

	return nil
}

func (h AddLiquidityHandler) handleV96(ctx cosmos.Context, msg MsgAddLiquidity) (errResult error) {
	// check if we need to swap before adding asset
	if h.needsSwap(msg) {
		return h.swapV93(ctx, msg)
	}

	pool, err := h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}

	if pool.IsEmpty() {
		ctx.Logger().Info("pool doesn't exist yet, creating a new one...", "symbol", msg.Asset.String(), "creator", msg.CacaoAddress)
		pool.Asset = msg.Asset
		if err = h.mgr.Keeper().SetPool(ctx, pool); err != nil {
			return ErrInternal(err, "fail to save pool to key value store")
		}
	}

	// if the pool decimals hasn't been set, it will still be 0. If we have a
	// pool asset coin, get the decimals from that transaction. This will only
	// set the decimals once.
	if pool.Decimals == 0 {
		coin := msg.GetTx().Coins.GetCoin(pool.Asset)
		if !coin.IsEmpty() {
			if coin.Decimals > 0 {
				pool.Decimals = coin.Decimals
			}
			ctx.Logger().Info("try update pool decimals", "asset", msg.Asset, "pool decimals", pool.Decimals)
			if err = h.mgr.Keeper().SetPool(ctx, pool); err != nil {
				return ErrInternal(err, "fail to save pool to key value store")
			}
		}
	}

	// figure out if we need to stage the funds and wait for a follow on
	// transaction to commit all funds atomically. For pools of native assets
	// only, stage is always false
	stage := false
	if !msg.Asset.IsVaultAsset() {
		if !msg.AssetAddress.IsEmpty() && msg.AssetAmount.IsZero() {
			stage = true
		}
		if !msg.CacaoAddress.IsEmpty() && msg.CacaoAmount.IsZero() {
			stage = true
		}
	}

	if msg.AffiliateBasisPoints.IsZero() {
		return h.addLiquidity(
			ctx,
			msg.Asset,
			msg.CacaoAmount,
			msg.AssetAmount,
			msg.CacaoAddress,
			msg.AssetAddress,
			msg.Tx,
			stage,
			h.mgr.GetConstants(),
			msg.LiquidityAuctionTier)
	}

	// add liquidity has an affiliate fee, add liquidity for both the user and their affiliate
	affiliateRune := common.GetSafeShare(msg.AffiliateBasisPoints, cosmos.NewUint(10000), msg.CacaoAmount)
	affiliateAsset := common.GetSafeShare(msg.AffiliateBasisPoints, cosmos.NewUint(10000), msg.AssetAmount)
	userRune := common.SafeSub(msg.CacaoAmount, affiliateRune)
	userAsset := common.SafeSub(msg.AssetAmount, affiliateAsset)

	err = h.addLiquidity(
		ctx,
		msg.Asset,
		userRune,
		userAsset,
		msg.CacaoAddress,
		msg.AssetAddress,
		msg.Tx,
		stage,
		h.mgr.GetConstants(),
		msg.LiquidityAuctionTier,
	)
	if err != nil {
		return err
	}

	err = h.addLiquidity(
		ctx,
		msg.Asset,
		affiliateRune,
		affiliateAsset,
		msg.AffiliateAddress,
		common.NoAddress,
		msg.Tx,
		stage,
		h.mgr.GetConstants(),
		msg.LiquidityAuctionTier,
	)
	if err != nil {
		// we swallow this error so we don't trigger a refund, when we've
		// already successfully added liquidity for the user. If we were to
		// refund here, funds could be leaked from the network. In order, to
		// error here, we would need to revert the user addLiquidity
		// function first (TODO).
		ctx.Logger().Error("fail to add liquidity for affiliate", "address", msg.AffiliateAddress, "error", err)
	}
	return nil
}

func (h AddLiquidityHandler) addLiquidityV96(ctx cosmos.Context,
	asset common.Asset,
	addCacaoAmount, addAssetAmount cosmos.Uint,
	runeAddr, assetAddr common.Address,
	tx common.Tx,
	stage bool,
	constAccessor constants.ConstantValues,
	tier int64,
) error {
	ctx.Logger().Info("liquidity provision", "asset", asset, "rune amount", addCacaoAmount, "asset amount", addAssetAmount)
	if err := h.validateAddLiquidityMessage(ctx, h.mgr.Keeper(), asset, tx, runeAddr, assetAddr); err != nil {
		return fmt.Errorf("add liquidity message fail validation: %w", err)
	}

	pool, err := h.mgr.Keeper().GetPool(ctx, asset)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get pool(%s)", asset))
	}
	synthSupply := h.mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
	originalUnits := pool.CalcUnits(h.mgr.GetVersion(), synthSupply)

	// if THORNode have no balance, set the default pool status
	if originalUnits.IsZero() {
		defaultPoolStatus := PoolAvailable.String()
		// if the pools is for gas asset on the chain, automatically enable it
		if !pool.Asset.Equals(pool.Asset.GetChain().GetGasAsset()) && !isLiquidityAuction(ctx, h.mgr.Keeper()) {
			defaultPoolStatus = constAccessor.GetStringValue(constants.DefaultPoolStatus)
		}
		pool.Status = GetPoolStatus(defaultPoolStatus)
	}

	fetchAddr := runeAddr
	if fetchAddr.IsEmpty() {
		fetchAddr = assetAddr
	}
	su, err := h.mgr.Keeper().GetLiquidityProvider(ctx, asset, fetchAddr)
	if err != nil {
		return ErrInternal(err, "fail to get liquidity provider")
	}

	su.LastAddHeight = ctx.BlockHeight()
	if su.Units.IsZero() {
		if su.PendingTxID.IsEmpty() {
			if su.CacaoAddress.IsEmpty() {
				su.CacaoAddress = runeAddr
			}
			if su.AssetAddress.IsEmpty() {
				su.AssetAddress = assetAddr
			}
		}

		if asset.IsVaultAsset() {
			// new SU, by default, places the thor address to the rune address,
			// but here we want it to be on the asset address only
			su.AssetAddress = assetAddr
			su.CacaoAddress = common.NoAddress // no rune to add/withdraw
		} else {
			// ensure input addresses match LP position addresses
			if !runeAddr.Equals(su.CacaoAddress) {
				return errAddLiquidityMismatchAddr
			}
			if !assetAddr.Equals(su.AssetAddress) {
				return errAddLiquidityMismatchAddr
			}
		}
	}

	if !assetAddr.IsEmpty() && !su.AssetAddress.Equals(assetAddr) && !asset.IsVaultAsset() {
		// mismatch of asset addresses from what is known to the address
		// given. Refund it.
		return errAddLiquidityMismatchAddr
	}

	// get tx hashes
	cacaoTxID := tx.ID
	assetTxID := tx.ID
	if addCacaoAmount.IsZero() {
		cacaoTxID = su.PendingTxID
	} else {
		assetTxID = su.PendingTxID
	}

	pendingCacaoAmt := su.PendingCacao.Add(addCacaoAmount)
	pendingAssetAmt := su.PendingAsset.Add(addAssetAmount)

	// if we have an asset address and no asset amount, put the rune pending
	if stage && pendingAssetAmt.IsZero() {
		pool.PendingInboundCacao = pool.PendingInboundCacao.Add(addCacaoAmount)
		su.PendingCacao = pendingCacaoAmt
		su.PendingTxID = tx.ID
		h.mgr.Keeper().SetLiquidityProvider(ctx, su)
		if err = h.mgr.Keeper().SetPool(ctx, pool); err != nil {
			ctx.Logger().Error("fail to save pool pending inbound rune", "error", err)
		}

		// add pending liquidity event
		evt := NewEventPendingLiquidity(pool.Asset, AddPendingLiquidity, su.CacaoAddress, addCacaoAmount, su.AssetAddress, cosmos.ZeroUint(), tx.ID, common.TxID(""))
		if err = h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
			return ErrInternal(err, "fail to emit partial add liquidity event")
		}
		return nil
	}

	// if we have a rune address and no rune asset, put the asset in pending
	if stage && pendingCacaoAmt.IsZero() {
		pool.PendingInboundAsset = pool.PendingInboundAsset.Add(addAssetAmount)
		su.PendingAsset = pendingAssetAmt
		su.PendingTxID = tx.ID
		h.mgr.Keeper().SetLiquidityProvider(ctx, su)
		if err = h.mgr.Keeper().SetPool(ctx, pool); err != nil {
			ctx.Logger().Error("fail to save pool pending inbound asset", "error", err)
		}

		// Set Liquidity Auction Tier
		if isLiquidityAuction(ctx, h.mgr.Keeper()) {
			tier1 := constAccessor.GetInt64Value(constants.WithdrawTier1)
			tier3 := constAccessor.GetInt64Value(constants.WithdrawTier3)
			var oldTier int64
			oldTier, err = h.mgr.Keeper().GetLiquidityAuctionTier(ctx, su.CacaoAddress)
			if err != nil {
				return ErrInternal(err, "fail to get liquidity auction tier")
			}

			if oldTier != 0 && tier > oldTier {
				tier = oldTier
			} else if tier < tier1 || tier > tier3 {
				tier = tier3
			}

			err = h.mgr.Keeper().SetLiquidityAuctionTier(ctx, runeAddr, tier)
			if err != nil {
				ctx.Logger().Error("fail to set liquidity auction tier", "error", err)
			}
		}

		evt := NewEventPendingLiquidity(pool.Asset, AddPendingLiquidity, su.CacaoAddress, cosmos.ZeroUint(), su.AssetAddress, addAssetAmount, common.TxID(""), tx.ID)
		if err = h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
			return ErrInternal(err, "fail to emit partial add liquidity event")
		}
		return nil
	}

	pool.PendingInboundCacao = common.SafeSub(pool.PendingInboundCacao, su.PendingCacao)
	pool.PendingInboundAsset = common.SafeSub(pool.PendingInboundAsset, su.PendingAsset)
	su.PendingAsset = cosmos.ZeroUint()
	su.PendingCacao = cosmos.ZeroUint()
	su.PendingTxID = ""

	ctx.Logger().Info("pre add liquidity", "pool", pool.Asset, "rune", pool.BalanceCacao, "asset", pool.BalanceAsset, "LP units", pool.LPUnits, "synth units", pool.SynthUnits)
	ctx.Logger().Info("adding liquidity", "rune", addCacaoAmount, "asset", addAssetAmount)

	balanceCacao := pool.BalanceCacao
	balanceAsset := pool.BalanceAsset

	oldPoolUnits := pool.GetPoolUnits()
	var newPoolUnits, liquidityUnits cosmos.Uint
	if asset.IsVaultAsset() {
		pendingCacaoAmt = cosmos.ZeroUint() // sanity check
		newPoolUnits, liquidityUnits = calculateVaultUnitsV1(oldPoolUnits, balanceAsset, pendingAssetAmt)
	} else {
		newPoolUnits, liquidityUnits, err = calculatePoolUnitsV1(oldPoolUnits, balanceCacao, balanceAsset, pendingCacaoAmt, pendingAssetAmt)
		if err != nil {
			return ErrInternal(err, "fail to calculate pool unit")
		}
	}
	ctx.Logger().Info("current pool status", "pool units", newPoolUnits, "liquidity units", liquidityUnits)
	poolRune := balanceCacao.Add(pendingCacaoAmt)
	poolAsset := balanceAsset.Add(pendingAssetAmt)
	pool.LPUnits = pool.LPUnits.Add(liquidityUnits)
	pool.BalanceCacao = poolRune
	pool.BalanceAsset = poolAsset
	ctx.Logger().Info("post add liquidity", "pool", pool.Asset, "rune", pool.BalanceCacao, "asset", pool.BalanceAsset, "LP units", pool.LPUnits, "synth units", pool.SynthUnits, "add liquidity units", liquidityUnits)
	if (pool.BalanceCacao.IsZero() && !asset.IsVaultAsset()) || pool.BalanceAsset.IsZero() {
		return ErrInternal(err, "pool cannot have zero rune or asset balance")
	}
	if err = h.mgr.Keeper().SetPool(ctx, pool); err != nil {
		return ErrInternal(err, "fail to save pool")
	}
	if originalUnits.IsZero() && !pool.GetPoolUnits().IsZero() {
		poolEvent := NewEventPool(pool.Asset, pool.Status)
		if err = h.mgr.EventMgr().EmitEvent(ctx, poolEvent); err != nil {
			ctx.Logger().Error("fail to emit pool event", "error", err)
		}
	}

	su.Units = su.Units.Add(liquidityUnits)
	if pool.Status == PoolAvailable {
		if su.AssetDepositValue.IsZero() && su.CacaoDepositValue.IsZero() {
			su.CacaoDepositValue = common.GetSafeShare(su.Units, pool.GetPoolUnits(), pool.BalanceCacao)
			su.AssetDepositValue = common.GetSafeShare(su.Units, pool.GetPoolUnits(), pool.BalanceAsset)
		} else {
			su.CacaoDepositValue = su.CacaoDepositValue.Add(common.GetSafeShare(liquidityUnits, pool.GetPoolUnits(), pool.BalanceCacao))
			su.AssetDepositValue = su.AssetDepositValue.Add(common.GetSafeShare(liquidityUnits, pool.GetPoolUnits(), pool.BalanceAsset))
		}
		// Recalculate bond provided to node if lp is bonder
		liquidityPools := GetLiquidityPools(h.mgr.GetVersion())
		if liquidityPools.Contains(pool.Asset) && !su.NodeBondAddress.Empty() {
			var na NodeAccount
			na, err = h.mgr.Keeper().GetNodeAccount(ctx, su.NodeBondAddress)
			if err != nil {
				ctx.Logger().Error("fail to get bonded node account of LP %s", su.CacaoAddress)
			}

			var bp BondProviders
			bp, err = h.mgr.Keeper().GetBondProviders(ctx, su.NodeBondAddress)
			if err != nil {
				ctx.Logger().Error("fail to get bonded providers for node account of LP %s", su.CacaoAddress)
			}

			addedUnits := su.Units.Sub(originalUnits)
			addedBond := common.GetSafeShare(addedUnits, pool.LPUnits, pool.BalanceCacao)
			bondEvent := NewEventBond(addedBond, BondPaid, tx)

			var from cosmos.AccAddress
			from, err = su.CacaoAddress.AccAddress()
			if err != nil {
				ctx.Logger().Error("fail to get lp account", "error", err)
			}

			if bp.Has(from) {
				bp.BondLiquidity(from)
			}

			if err = h.mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
				ctx.Logger().Error("fail to emit bond event", "error", err)
			}

			if err = h.mgr.Keeper().SetNodeAccount(ctx, na); err != nil {
				return ErrInternal(err, fmt.Sprintf("fail to save node account(%s)", na.String()))
			}

			if err = h.mgr.Keeper().SetBondProviders(ctx, bp); err != nil {
				return ErrInternal(err, fmt.Sprintf("fail to save bond providers(%s)", bp.NodeAddress.String()))
			}
		}
	}

	h.mgr.Keeper().SetLiquidityProvider(ctx, su)

	evt := NewEventAddLiquidity(asset, liquidityUnits, su.CacaoAddress, pendingCacaoAmt, pendingAssetAmt, cacaoTxID, assetTxID, su.AssetAddress)
	if err = h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
		return ErrInternal(err, "fail to emit add liquidity event")
	}

	// if its the POL is adding, track rune added
	polAddress, err := h.mgr.Keeper().GetModuleAddress(ReserveName)
	if err != nil {
		return err
	}

	if polAddress.Equals(su.CacaoAddress) {
		var pol ProtocolOwnedLiquidity
		pol, err = h.mgr.Keeper().GetPOL(ctx)
		if err != nil {
			return err
		}
		pol.CacaoDeposited = pol.CacaoDeposited.Add(pendingCacaoAmt)

		if err = h.mgr.Keeper().SetPOL(ctx, pol); err != nil {
			return err
		}

		ctx.Logger().Info("POL deposit", "pool", pool.Asset, "rune", pendingCacaoAmt)
		telemetry.IncrCounterWithLabels(
			[]string{"mayanode", "pol", "pool", "rune_deposited"},
			telem(pendingCacaoAmt),
			[]metrics.Label{telemetry.NewLabel("pool", pool.Asset.String())},
		)
	}
	return nil
}

func (h AddLiquidityHandler) addLiquidityV105(ctx cosmos.Context,
	asset common.Asset,
	addCacaoAmount, addAssetAmount cosmos.Uint,
	runeAddr, assetAddr common.Address,
	tx common.Tx,
	stage bool,
	constAccessor constants.ConstantValues,
	tier int64,
) error {
	ctx.Logger().Info("liquidity provision", "asset", asset, "rune amount", addCacaoAmount, "asset amount", addAssetAmount)
	if err := h.validateAddLiquidityMessage(ctx, h.mgr.Keeper(), asset, tx, runeAddr, assetAddr); err != nil {
		return fmt.Errorf("add liquidity message fail validation: %w", err)
	}

	pool, err := h.mgr.Keeper().GetPool(ctx, asset)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get pool(%s)", asset))
	}
	synthSupply := h.mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
	originalUnits := pool.CalcUnits(h.mgr.GetVersion(), synthSupply)

	// if THORNode have no balance, set the default pool status
	if originalUnits.IsZero() {
		defaultPoolStatus := PoolAvailable.String()
		// if the pools is for gas asset on the chain, automatically enable it
		if !pool.Asset.Equals(pool.Asset.GetChain().GetGasAsset()) && !isLiquidityAuction(ctx, h.mgr.Keeper()) {
			defaultPoolStatus = constAccessor.GetStringValue(constants.DefaultPoolStatus)
		}
		pool.Status = GetPoolStatus(defaultPoolStatus)
	}

	fetchAddr := runeAddr
	if fetchAddr.IsEmpty() {
		fetchAddr = assetAddr
	}
	su, err := h.mgr.Keeper().GetLiquidityProvider(ctx, asset, fetchAddr)
	if err != nil {
		return ErrInternal(err, "fail to get liquidity provider")
	}

	su.LastAddHeight = ctx.BlockHeight()
	if su.Units.IsZero() {
		if su.PendingTxID.IsEmpty() {
			if su.CacaoAddress.IsEmpty() {
				su.CacaoAddress = runeAddr
			}
			if su.AssetAddress.IsEmpty() {
				su.AssetAddress = assetAddr
			}
		}

		if asset.IsVaultAsset() {
			// new SU, by default, places the thor address to the rune address,
			// but here we want it to be on the asset address only
			su.AssetAddress = assetAddr
			su.CacaoAddress = common.NoAddress // no rune to add/withdraw
		} else {
			// ensure input addresses match LP position addresses
			if !runeAddr.Equals(su.CacaoAddress) {
				return errAddLiquidityMismatchAddr
			}
			if !assetAddr.Equals(su.AssetAddress) {
				return errAddLiquidityMismatchAddr
			}
		}
	}

	if !assetAddr.IsEmpty() && !su.AssetAddress.Equals(assetAddr) && !asset.IsVaultAsset() {
		// mismatch of asset addresses from what is known to the address
		// given. Refund it.
		return errAddLiquidityMismatchAddr
	}

	// get tx hashes
	cacaoTxID := tx.ID
	assetTxID := tx.ID
	if addCacaoAmount.IsZero() {
		cacaoTxID = su.PendingTxID
	} else {
		assetTxID = su.PendingTxID
	}

	pendingCacaoAmt := su.PendingCacao.Add(addCacaoAmount)
	pendingAssetAmt := su.PendingAsset.Add(addAssetAmount)

	// if we have an asset address and no asset amount, put the rune pending
	if stage && pendingAssetAmt.IsZero() {
		pool.PendingInboundCacao = pool.PendingInboundCacao.Add(addCacaoAmount)
		su.PendingCacao = pendingCacaoAmt
		su.PendingTxID = tx.ID
		h.mgr.Keeper().SetLiquidityProvider(ctx, su)
		if err = h.mgr.Keeper().SetPool(ctx, pool); err != nil {
			ctx.Logger().Error("fail to save pool pending inbound rune", "error", err)
		}

		// add pending liquidity event
		evt := NewEventPendingLiquidity(pool.Asset, AddPendingLiquidity, su.CacaoAddress, addCacaoAmount, su.AssetAddress, cosmos.ZeroUint(), tx.ID, common.TxID(""))
		if err = h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
			return ErrInternal(err, "fail to emit partial add liquidity event")
		}
		return nil
	}

	// if we have a rune address and no rune asset, put the asset in pending
	if stage && pendingCacaoAmt.IsZero() {
		pool.PendingInboundAsset = pool.PendingInboundAsset.Add(addAssetAmount)
		su.PendingAsset = pendingAssetAmt
		su.PendingTxID = tx.ID
		h.mgr.Keeper().SetLiquidityProvider(ctx, su)
		if err = h.mgr.Keeper().SetPool(ctx, pool); err != nil {
			ctx.Logger().Error("fail to save pool pending inbound asset", "error", err)
		}

		// Set Liquidity Auction Tier
		if isLiquidityAuction(ctx, h.mgr.Keeper()) {
			tier1 := constAccessor.GetInt64Value(constants.WithdrawTier1)
			tier3 := constAccessor.GetInt64Value(constants.WithdrawTier3)
			var oldTier int64
			oldTier, err = h.mgr.Keeper().GetLiquidityAuctionTier(ctx, su.CacaoAddress)
			if err != nil {
				return ErrInternal(err, "fail to get liquidity auction tier")
			}

			if oldTier != 0 && tier > oldTier {
				tier = oldTier
			} else if tier < tier1 || tier > tier3 {
				tier = tier3
			}

			err = h.mgr.Keeper().SetLiquidityAuctionTier(ctx, runeAddr, tier)
			if err != nil {
				ctx.Logger().Error("fail to set liquidity auction tier", "error", err)
			}
		}

		evt := NewEventPendingLiquidity(pool.Asset, AddPendingLiquidity, su.CacaoAddress, cosmos.ZeroUint(), su.AssetAddress, addAssetAmount, common.TxID(""), tx.ID)
		if err = h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
			return ErrInternal(err, "fail to emit partial add liquidity event")
		}
		return nil
	}

	pool.PendingInboundCacao = common.SafeSub(pool.PendingInboundCacao, su.PendingCacao)
	pool.PendingInboundAsset = common.SafeSub(pool.PendingInboundAsset, su.PendingAsset)
	su.PendingAsset = cosmos.ZeroUint()
	su.PendingCacao = cosmos.ZeroUint()
	su.PendingTxID = ""

	ctx.Logger().Info("pre add liquidity", "pool", pool.Asset, "rune", pool.BalanceCacao, "asset", pool.BalanceAsset, "LP units", pool.LPUnits, "synth units", pool.SynthUnits)
	ctx.Logger().Info("adding liquidity", "rune", addCacaoAmount, "asset", addAssetAmount)

	balanceCacao := pool.BalanceCacao
	balanceAsset := pool.BalanceAsset

	oldPoolUnits := pool.GetPoolUnits()
	var newPoolUnits, liquidityUnits cosmos.Uint
	if asset.IsVaultAsset() {
		pendingCacaoAmt = cosmos.ZeroUint() // sanity check
		newPoolUnits, liquidityUnits = calculateVaultUnitsV1(oldPoolUnits, balanceAsset, pendingAssetAmt)
	} else {
		newPoolUnits, liquidityUnits, err = calculatePoolUnitsV1(oldPoolUnits, balanceCacao, balanceAsset, pendingCacaoAmt, pendingAssetAmt)
		if err != nil {
			return ErrInternal(err, "fail to calculate pool unit")
		}
	}
	ctx.Logger().Info("current pool status", "pool units", newPoolUnits, "liquidity units", liquidityUnits)
	poolRune := balanceCacao.Add(pendingCacaoAmt)
	poolAsset := balanceAsset.Add(pendingAssetAmt)
	pool.LPUnits = pool.LPUnits.Add(liquidityUnits)
	pool.BalanceCacao = poolRune
	pool.BalanceAsset = poolAsset
	ctx.Logger().Info("post add liquidity", "pool", pool.Asset, "rune", pool.BalanceCacao, "asset", pool.BalanceAsset, "LP units", pool.LPUnits, "synth units", pool.SynthUnits, "add liquidity units", liquidityUnits)
	if (pool.BalanceCacao.IsZero() && !asset.IsVaultAsset()) || pool.BalanceAsset.IsZero() {
		return ErrInternal(err, "pool cannot have zero rune or asset balance")
	}
	if err = h.mgr.Keeper().SetPool(ctx, pool); err != nil {
		return ErrInternal(err, "fail to save pool")
	}
	if originalUnits.IsZero() && !pool.GetPoolUnits().IsZero() {
		poolEvent := NewEventPool(pool.Asset, pool.Status)
		if err = h.mgr.EventMgr().EmitEvent(ctx, poolEvent); err != nil {
			ctx.Logger().Error("fail to emit pool event", "error", err)
		}
	}

	su.Units = su.Units.Add(liquidityUnits)
	if pool.Status == PoolAvailable {
		if su.AssetDepositValue.IsZero() && su.CacaoDepositValue.IsZero() {
			su.CacaoDepositValue = common.GetSafeShare(su.Units, pool.GetPoolUnits(), pool.BalanceCacao)
			su.AssetDepositValue = common.GetSafeShare(su.Units, pool.GetPoolUnits(), pool.BalanceAsset)
		} else {
			su.CacaoDepositValue = su.CacaoDepositValue.Add(common.GetSafeShare(liquidityUnits, pool.GetPoolUnits(), pool.BalanceCacao))
			su.AssetDepositValue = su.AssetDepositValue.Add(common.GetSafeShare(liquidityUnits, pool.GetPoolUnits(), pool.BalanceAsset))
		}
	}

	h.mgr.Keeper().SetLiquidityProvider(ctx, su)

	evt := NewEventAddLiquidity(asset, liquidityUnits, su.CacaoAddress, pendingCacaoAmt, pendingAssetAmt, cacaoTxID, assetTxID, su.AssetAddress)
	if err = h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
		return ErrInternal(err, "fail to emit add liquidity event")
	}

	// if its the POL is adding, track rune added
	polAddress, err := h.mgr.Keeper().GetModuleAddress(ReserveName)
	if err != nil {
		return err
	}

	if polAddress.Equals(su.CacaoAddress) {
		var pol ProtocolOwnedLiquidity
		pol, err = h.mgr.Keeper().GetPOL(ctx)
		if err != nil {
			return err
		}
		pol.CacaoDeposited = pol.CacaoDeposited.Add(pendingCacaoAmt)

		if err = h.mgr.Keeper().SetPOL(ctx, pol); err != nil {
			return err
		}

		ctx.Logger().Info("POL deposit", "pool", pool.Asset, "rune", pendingCacaoAmt)
		telemetry.IncrCounterWithLabels(
			[]string{"mayanode", "pol", "pool", "rune_deposited"},
			telem(pendingCacaoAmt),
			[]metrics.Label{telemetry.NewLabel("pool", pool.Asset.String())},
		)
	}
	return nil
}

func (h AddLiquidityHandler) handleV108(ctx cosmos.Context, msg MsgAddLiquidity) (errResult error) {
	// check if we need to swap before adding asset
	if h.needsSwap(msg) {
		return h.swapV93(ctx, msg)
	}

	pool, err := h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}

	if pool.IsEmpty() {
		ctx.Logger().Info("pool doesn't exist yet, creating a new one...", "symbol", msg.Asset.String(), "creator", msg.CacaoAddress)

		pool.Asset = msg.Asset

		defaultPoolStatus := PoolAvailable.String()
		// only set the pool to default pool status if not for gas asset on the chain
		if !pool.Asset.Equals(pool.Asset.GetChain().GetGasAsset()) &&
			!pool.Asset.IsVaultAsset() {
			defaultPoolStatus = h.mgr.GetConstants().GetStringValue(constants.DefaultPoolStatus)
		}
		pool.Status = GetPoolStatus(defaultPoolStatus)

		if err = h.mgr.Keeper().SetPool(ctx, pool); err != nil {
			return ErrInternal(err, "fail to save pool to key value store")
		}
	}

	// if the pool decimals hasn't been set, it will still be 0. If we have a
	// pool asset coin, get the decimals from that transaction. This will only
	// set the decimals once.
	if pool.Decimals == 0 {
		coin := msg.GetTx().Coins.GetCoin(pool.Asset)
		if !coin.IsEmpty() {
			if coin.Decimals > 0 {
				pool.Decimals = coin.Decimals
			}
			ctx.Logger().Info("try update pool decimals", "asset", msg.Asset, "pool decimals", pool.Decimals)
			if err = h.mgr.Keeper().SetPool(ctx, pool); err != nil {
				return ErrInternal(err, "fail to save pool to key value store")
			}
		}
	}

	// figure out if we need to stage the funds and wait for a follow on
	// transaction to commit all funds atomically. For pools of native assets
	// only, stage is always false
	stage := false
	if !msg.Asset.IsVaultAsset() {
		if !msg.AssetAddress.IsEmpty() && msg.AssetAmount.IsZero() {
			stage = true
		}
		if !msg.CacaoAddress.IsEmpty() && msg.CacaoAmount.IsZero() {
			stage = true
		}
	}

	if msg.AffiliateBasisPoints.IsZero() {
		return h.addLiquidity(
			ctx,
			msg.Asset,
			msg.CacaoAmount,
			msg.AssetAmount,
			msg.CacaoAddress,
			msg.AssetAddress,
			msg.Tx,
			stage,
			h.mgr.GetConstants(),
			msg.LiquidityAuctionTier)
	}

	// add liquidity has an affiliate fee, add liquidity for both the user and their affiliate
	affiliateRune := common.GetSafeShare(msg.AffiliateBasisPoints, cosmos.NewUint(10000), msg.CacaoAmount)
	affiliateAsset := common.GetSafeShare(msg.AffiliateBasisPoints, cosmos.NewUint(10000), msg.AssetAmount)
	userRune := common.SafeSub(msg.CacaoAmount, affiliateRune)
	userAsset := common.SafeSub(msg.AssetAmount, affiliateAsset)

	err = h.addLiquidity(
		ctx,
		msg.Asset,
		userRune,
		userAsset,
		msg.CacaoAddress,
		msg.AssetAddress,
		msg.Tx,
		stage,
		h.mgr.GetConstants(),
		msg.LiquidityAuctionTier,
	)
	if err != nil {
		return err
	}
	affiliateRuneAddress := common.NoAddress
	affiliateAssetAddress := common.NoAddress
	if msg.AffiliateAddress.IsChain(common.THORChain, h.mgr.GetVersion()) {
		affiliateRuneAddress = msg.AffiliateAddress
	} else {
		affiliateAssetAddress = msg.AffiliateAddress
	}

	err = h.addLiquidity(
		ctx,
		msg.Asset,
		affiliateRune,
		affiliateAsset,
		affiliateRuneAddress,
		affiliateAssetAddress,
		msg.Tx,
		false,
		h.mgr.GetConstants(),
		msg.LiquidityAuctionTier,
	)
	if err != nil {
		ctx.Logger().Error("fail to add liquidity for affiliate", "address", msg.AffiliateAddress, "error", err)
		return err
	}
	return nil
}

func (h AddLiquidityHandler) handleV109(ctx cosmos.Context, msg MsgAddLiquidity) (errResult error) {
	// check if we need to swap before adding asset
	if h.needsSwap(msg) {
		return h.swapV93(ctx, msg)
	}

	pool, err := h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}

	if pool.IsEmpty() {
		ctx.Logger().Info("pool doesn't exist yet, creating a new one...", "symbol", msg.Asset.String(), "creator", msg.CacaoAddress)

		pool.Asset = msg.Asset

		defaultPoolStatus := PoolAvailable.String()
		// only set the pool to default pool status if not for gas asset on the chain
		if !pool.Asset.Equals(pool.Asset.GetChain().GetGasAsset()) &&
			!pool.Asset.IsVaultAsset() {
			defaultPoolStatus = h.mgr.GetConstants().GetStringValue(constants.DefaultPoolStatus)
		}
		pool.Status = GetPoolStatus(defaultPoolStatus)

		if err = h.mgr.Keeper().SetPool(ctx, pool); err != nil {
			return ErrInternal(err, "fail to save pool to key value store")
		}
	}

	// if the pool decimals hasn't been set, it will still be 0. If we have a
	// pool asset coin, get the decimals from that transaction. This will only
	// set the decimals once.
	if pool.Decimals == 0 {
		coin := msg.GetTx().Coins.GetCoin(pool.Asset)
		if !coin.IsEmpty() {
			if coin.Decimals > 0 {
				pool.Decimals = coin.Decimals
			}
			ctx.Logger().Info("try update pool decimals", "asset", msg.Asset, "pool decimals", pool.Decimals)
			if err = h.mgr.Keeper().SetPool(ctx, pool); err != nil {
				return ErrInternal(err, "fail to save pool to key value store")
			}
		}
	}

	// figure out if we need to stage the funds and wait for a follow on
	// transaction to commit all funds atomically. For pools of native assets
	// only, stage is always false
	stage := false
	if !msg.Asset.IsVaultAsset() {
		if !msg.AssetAddress.IsEmpty() && msg.AssetAmount.IsZero() {
			stage = true
		}
		if !msg.CacaoAddress.IsEmpty() && msg.CacaoAmount.IsZero() {
			stage = true
		}
	}

	if msg.AffiliateBasisPoints.IsZero() {
		return h.addLiquidity(
			ctx,
			msg.Asset,
			msg.CacaoAmount,
			msg.AssetAmount,
			msg.CacaoAddress,
			msg.AssetAddress,
			msg.Tx,
			stage,
			h.mgr.GetConstants(),
			msg.LiquidityAuctionTier)
	}

	// add liquidity has an affiliate fee, add liquidity for both the user and their affiliate
	affiliateRune := common.GetSafeShare(msg.AffiliateBasisPoints, cosmos.NewUint(10000), msg.CacaoAmount)
	affiliateAsset := common.GetSafeShare(msg.AffiliateBasisPoints, cosmos.NewUint(10000), msg.AssetAmount)
	userRune := common.SafeSub(msg.CacaoAmount, affiliateRune)
	userAsset := common.SafeSub(msg.AssetAmount, affiliateAsset)

	err = h.addLiquidity(
		ctx,
		msg.Asset,
		userRune,
		userAsset,
		msg.CacaoAddress,
		msg.AssetAddress,
		msg.Tx,
		stage,
		h.mgr.GetConstants(),
		msg.LiquidityAuctionTier,
	)
	if err != nil {
		return err
	}
	affiliateRuneAddress := common.NoAddress
	affiliateAssetAddress := common.NoAddress
	if msg.AffiliateAddress.IsChain(common.BASEChain, h.mgr.GetVersion()) {
		affiliateRuneAddress = msg.AffiliateAddress
	} else {
		affiliateAssetAddress = msg.AffiliateAddress
	}

	err = h.addLiquidity(
		ctx,
		msg.Asset,
		affiliateRune,
		affiliateAsset,
		affiliateRuneAddress,
		affiliateAssetAddress,
		msg.Tx,
		false,
		h.mgr.GetConstants(),
		msg.LiquidityAuctionTier,
	)
	if err != nil {
		ctx.Logger().Error("fail to add liquidity for affiliate", "address", msg.AffiliateAddress, "error", err)
		return err
	}
	return nil
}

func (h AddLiquidityHandler) swapV93(ctx cosmos.Context, msg MsgAddLiquidity) error {
	// ensure TxID does NOT have a collision with another swap, this could
	// happen if the user submits two identical loan requests in the same
	// block
	if ok := h.mgr.Keeper().HasSwapQueueItem(ctx, msg.Tx.ID, 0); ok {
		return fmt.Errorf("txn hash conflict")
	}

	// sanity check, ensure address or asset doesn't have separator within them
	if strings.Contains(fmt.Sprintf("%s%s", msg.Asset, msg.AffiliateAddress), ":") {
		return fmt.Errorf("illegal character")
	}
	memo := fmt.Sprintf("+:%s::%s:%d", msg.Asset, msg.AffiliateAddress, msg.AffiliateBasisPoints.Uint64())
	msg.Tx.Memo = memo
	swapMsg := NewMsgSwap(msg.Tx, msg.Asset, common.NoopAddress, cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(), "", "", nil, MarketOrder, 0, 0, msg.Signer)

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

func (h AddLiquidityHandler) validateV108(ctx cosmos.Context, msg MsgAddLiquidity) error {
	if err := msg.ValidateBasicV108(); err != nil {
		ctx.Logger().Error(err.Error())
		return errAddLiquidityFailValidation
	}

	if msg.Asset.IsVaultAsset() {
		if !msg.Asset.GetLayer1Asset().IsGasAsset() {
			return fmt.Errorf("asset must be a gas asset for the layer1 protocol")
		}
		if !msg.AssetAddress.IsChain(msg.Asset.GetLayer1Asset().GetChain(), h.mgr.GetVersion()) {
			return fmt.Errorf("asset address must be layer1 chain")
		}
		if !msg.CacaoAmount.IsZero() {
			return fmt.Errorf("cannot deposit rune into a vault")
		}
	}

	if !msg.CacaoAddress.IsEmpty() && !msg.CacaoAddress.IsChain(common.BASEChain, h.mgr.GetVersion()) {
		ctx.Logger().Error("rune address must be BASEChain")
		return errAddLiquidityFailValidation
	}

	// check if swap meets standards
	if h.needsSwap(msg) {
		if !msg.Asset.IsVaultAsset() {
			return fmt.Errorf("swap & add liquidity is only available for synthetic pools")
		}
		if !msg.Asset.GetLayer1Asset().Equals(msg.Tx.Coins[0].Asset) {
			return fmt.Errorf("deposit asset must be the layer1 equivalent for the synthetic asset")
		}
	}

	pool, err := h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}
	if err = pool.EnsureValidPoolStatus(&msg); err != nil {
		ctx.Logger().Error("fail to check pool status", "error", err)
		return errInvalidPoolStatus
	}

	if isChainHalted(ctx, h.mgr, msg.Asset.Chain) || isLPPaused(ctx, msg.Asset.Chain, h.mgr) {
		return fmt.Errorf("unable to add liquidity while chain has paused LP actions")
	}

	liquidityPools := GetLiquidityPools(h.mgr.GetVersion())
	if liquidityPools.Contains(msg.Asset) {
		var lp LiquidityProvider
		lp, err = h.mgr.Keeper().GetLiquidityProvider(ctx, msg.Asset, msg.CacaoAddress)
		if err != nil {
			return ErrInternal(err, "failed to get LP")
		}

		// Is a liquidity bond provider. If this is set it means the pool is already available
		if !lp.NodeBondAddress.Empty() {
			// check if it's a genesis node
			for _, genesis := range GenesisNodes {
				var genAddr cosmos.AccAddress
				genAddr, err = cosmos.AccAddressFromBech32(genesis)
				if err != nil {
					return ErrInternal(err, "fail to parse genesis node address")
				}
				if genAddr.Equals(lp.NodeBondAddress) {
					ctx.Logger().Error("cannot add liquidity to genesis node", "node", lp.NodeBondAddress)
					return fmt.Errorf("cannot add liquidity to genesis node: %s", lp.NodeBondAddress)
				}
			}

			var na NodeAccount
			na, err = h.mgr.Keeper().GetNodeAccount(ctx, lp.NodeBondAddress)
			if err != nil {
				ctx.Logger().Error("failed to get node account for lbp")
				return ErrInternal(err, "failed to get node account for lbp")
			}

			if na.Status == NodeReady {
				ctx.Logger().Error("cannot add bond while node is ready status", "node", lp.NodeBondAddress)
				return fmt.Errorf("cannot add bond while node is ready status: %s", lp.NodeBondAddress)
			}

			if fetchConfigInt64(ctx, h.mgr, constants.PauseBond) > 0 {
				ctx.Logger().Error("bonding has been paused", "node", lp.NodeBondAddress)
				return fmt.Errorf("bonding has been paused")
			}

			var liquidityBond cosmos.Uint
			liquidityBond, err = h.mgr.Keeper().CalcNodeLiquidityBond(ctx, na)
			if err != nil {
				return ErrInternal(err, fmt.Sprintf("fail to calculate liquidity bond for address: %s", lp.CacaoAddress))
			}

			bond := liquidityBond.Add(msg.CacaoAmount)
			var maxBond int64
			maxBond, err = h.mgr.Keeper().GetMimir(ctx, "MaximumBondInRune")
			if maxBond > 0 && err == nil {
				maxValidatorBond := cosmos.NewUint(uint64(maxBond))
				if bond.GT(maxValidatorBond) {
					return cosmos.ErrUnknownRequest(fmt.Sprintf("too much bond, max validator bond (%s), bond(%s)", maxValidatorBond.String(), bond))
				}
			}

		}
	}

	ensureLiquidityNoLargerThanBondInt64, err := h.mgr.Keeper().GetMimir(ctx, "EnsureLiquidityNoLargerThanBond")
	if err != nil {
		ctx.Logger().Error("fail to get mimir", "error", err)
	}
	// 0 not active, 1 active, -1 use default
	var ensureLiquidityNoLargerThanBond bool
	if ensureLiquidityNoLargerThanBondInt64 < 0 {
		ensureLiquidityNoLargerThanBond = h.mgr.GetConstants().GetBoolValue(constants.StrictBondLiquidityRatio)
	} else {
		ensureLiquidityNoLargerThanBond = ensureLiquidityNoLargerThanBondInt64 == 1
	}
	// if the pool is THORChain no need to check economic security
	if msg.Asset.IsVaultAsset() || !ensureLiquidityNoLargerThanBond {
		return nil
	}

	totalLiquidity, err := h.getTotalLiquidityBase(ctx)
	if err != nil {
		return ErrInternal(err, "fail to get total liquidity RUNE")
	}

	// total liquidity RUNE after current add liquidity
	pool, err = h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}
	totalLiquidity = totalLiquidity.Add(msg.CacaoAmount)
	totalLiquidity = totalLiquidity.Add(pool.AssetValueInRune(msg.AssetAmount))
	maximumLiquidityRune, err := h.mgr.Keeper().GetMimir(ctx, constants.MaximumLiquidityCacao.String())
	if maximumLiquidityRune < 0 || err != nil {
		maximumLiquidityRune = h.mgr.GetConstants().GetInt64Value(constants.MaximumLiquidityCacao)
	}
	if maximumLiquidityRune > 0 {
		if totalLiquidity.GT(cosmos.NewUint(uint64(maximumLiquidityRune))) {
			return errAddLiquidityRUNEOverLimit
		}
	}

	if !ensureLiquidityNoLargerThanBond {
		return nil
	}

	// the following  only applicable for chaosnet
	totalBondRune, err := h.getTotalActiveBond(ctx)
	if err != nil {
		return ErrInternal(err, "fail to get total bond RUNE")
	}

	totalNonNodeLiquidity := common.SafeSub(totalLiquidity, totalBondRune)
	if totalNonNodeLiquidity.GT(totalBondRune) {
		ctx.Logger().Info("total non node liquidity RUNE is more than total Bond", "rune", totalNonNodeLiquidity.String(), "bond", totalBondRune.String())
		return errAddLiquidityCACAOMoreThanBond
	}

	return nil
}

func (h AddLiquidityHandler) validateV110(ctx cosmos.Context, msg MsgAddLiquidity) error {
	if err := msg.ValidateBasicV108(); err != nil {
		ctx.Logger().Error(err.Error())
		return errAddLiquidityFailValidation
	}

	if msg.Asset.IsVaultAsset() {
		if !msg.Asset.GetLayer1Asset().IsGasAsset() {
			return fmt.Errorf("asset must be a gas asset for the layer1 protocol")
		}
		if !msg.AssetAddress.IsChain(msg.Asset.GetLayer1Asset().GetChain(), h.mgr.GetVersion()) {
			return fmt.Errorf("asset address must be layer1 chain")
		}
		if !msg.CacaoAmount.IsZero() {
			return fmt.Errorf("cannot deposit rune into a vault")
		}
	}

	if !msg.CacaoAddress.IsEmpty() && !msg.CacaoAddress.IsChain(common.BASEChain, h.mgr.GetVersion()) {
		ctx.Logger().Error("rune address must be BASEChain")
		return errAddLiquidityFailValidation
	}

	// check if swap meets standards
	if h.needsSwap(msg) {
		if !msg.Asset.IsVaultAsset() {
			return fmt.Errorf("swap & add liquidity is only available for synthetic pools")
		}
		if !msg.Asset.GetLayer1Asset().Equals(msg.Tx.Coins[0].Asset) {
			return fmt.Errorf("deposit asset must be the layer1 equivalent for the synthetic asset")
		}
	}

	pool, err := h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}
	if err = pool.EnsureValidPoolStatus(&msg); err != nil {
		ctx.Logger().Error("fail to check pool status", "error", err)
		return errInvalidPoolStatus
	}

	if isChainHalted(ctx, h.mgr, msg.Asset.Chain) || isLPPaused(ctx, msg.Asset.Chain, h.mgr) {
		return fmt.Errorf("unable to add liquidity while chain has paused LP actions")
	}

	liquidityPools := GetLiquidityPools(h.mgr.GetVersion())
	if liquidityPools.Contains(msg.Asset) {
		var lp LiquidityProvider
		lp, err = h.mgr.Keeper().GetLiquidityProvider(ctx, msg.Asset, msg.CacaoAddress)
		if err != nil {
			return ErrInternal(err, "failed to get LP")
		}

		// Is a liquidity bond provider. If this is set it means the pool is already available
		if !lp.NodeBondAddress.Empty() {
			// check if it's a genesis node
			for _, genesis := range GenesisNodes {
				var genAddr cosmos.AccAddress
				genAddr, err = cosmos.AccAddressFromBech32(genesis)
				if err != nil {
					return ErrInternal(err, "fail to parse genesis node address")
				}
				if genAddr.Equals(lp.NodeBondAddress) {
					ctx.Logger().Error("cannot add liquidity to genesis node", "node", lp.NodeBondAddress)
					return fmt.Errorf("cannot add liquidity to genesis node: %s", lp.NodeBondAddress)
				}
			}

			var na NodeAccount
			na, err = h.mgr.Keeper().GetNodeAccount(ctx, lp.NodeBondAddress)
			if err != nil {
				ctx.Logger().Error("failed to get node account for lbp")
				return ErrInternal(err, "failed to get node account for lbp")
			}

			if na.Status == NodeReady {
				ctx.Logger().Error("cannot add bond while node is ready status", "node", lp.NodeBondAddress)
				return fmt.Errorf("cannot add bond while node is ready status: %s", lp.NodeBondAddress)
			}

			if fetchConfigInt64(ctx, h.mgr, constants.PauseBond) > 0 {
				ctx.Logger().Error("bonding has been paused", "node", lp.NodeBondAddress)
				return fmt.Errorf("bonding has been paused")
			}

			var liquidityBond cosmos.Uint
			liquidityBond, err = h.mgr.Keeper().CalcNodeLiquidityBond(ctx, na)
			if err != nil {
				return ErrInternal(err, fmt.Sprintf("fail to calculate liquidity bond for address: %s", lp.CacaoAddress))
			}

			bond := liquidityBond.Add(msg.CacaoAmount)
			var maxBond int64
			maxBond, err = h.mgr.Keeper().GetMimir(ctx, "MaximumBondInRune")
			if maxBond > 0 && err == nil {
				maxValidatorBond := cosmos.NewUint(uint64(maxBond))
				if bond.GT(maxValidatorBond) {
					return cosmos.ErrUnknownRequest(fmt.Sprintf("too much bond, max validator bond (%s), bond(%s)", maxValidatorBond.String(), bond))
				}
			}

		}
	}

	ensureLiquidityNoLargerThanBondInt64, err := h.mgr.Keeper().GetMimir(ctx, "EnsureLiquidityNoLargerThanBond")
	if err != nil {
		ctx.Logger().Error("fail to get mimir", "error", err)
	}
	// 0 not active, 1 active, -1 use default
	var ensureLiquidityNoLargerThanBond bool
	if ensureLiquidityNoLargerThanBondInt64 < 0 {
		ensureLiquidityNoLargerThanBond = h.mgr.GetConstants().GetBoolValue(constants.StrictBondLiquidityRatio)
	} else {
		ensureLiquidityNoLargerThanBond = ensureLiquidityNoLargerThanBondInt64 == 1
	}

	// if the pool is THORChain no need to check economic security
	if msg.Asset.IsVaultAsset() || !ensureLiquidityNoLargerThanBond {
		return nil
	}

	totalLiquidity, err := h.getTotalLiquidityBase(ctx)
	if err != nil {
		return ErrInternal(err, "fail to get total liquidity CACAO")
	}

	// total liquidity RUNE after current add liquidity
	pool, err = h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}
	totalLiquidity = totalLiquidity.Add(msg.CacaoAmount)
	totalLiquidity = totalLiquidity.Add(pool.AssetValueInRune(msg.AssetAmount))
	maximumLiquidityRune, err := h.mgr.Keeper().GetMimir(ctx, constants.MaximumLiquidityCacao.String())
	if maximumLiquidityRune < 0 || err != nil {
		maximumLiquidityRune = h.mgr.GetConstants().GetInt64Value(constants.MaximumLiquidityCacao)
	}
	if maximumLiquidityRune > 0 {
		if totalLiquidity.GT(cosmos.NewUint(uint64(maximumLiquidityRune))) {
			return errAddLiquidityRUNEOverLimit
		}
	}

	coins := common.NewCoins(
		common.NewCoin(common.BaseAsset(), msg.CacaoAmount),
		common.NewCoin(msg.Asset, msg.AssetAmount),
	)

	if atTVLCap(ctx, coins, h.mgr) {
		return errAddLiquidityCACAOMoreThanBond
	}

	return nil
}
