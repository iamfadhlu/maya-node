package mayachain

import (
	"errors"
	"fmt"
	"strings"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
)

func isModuleAccAddressV112(keeper keeper.Keeper, accAddr cosmos.AccAddress) bool {
	return accAddr.Equals(keeper.GetModuleAccAddress(AsgardName)) ||
		accAddr.Equals(keeper.GetModuleAccAddress(BondName)) ||
		accAddr.Equals(keeper.GetModuleAccAddress(ReserveName)) ||
		accAddr.Equals(keeper.GetModuleAccAddress(ModuleName)) ||
		accAddr.Equals(keeper.GetModuleAccAddress(MayaFund)) ||
		accAddr.Equals(keeper.GetModuleAccAddress(AffiliateCollectorName))
}

func isModuleAccAddressV110(keeper keeper.Keeper, accAddr cosmos.AccAddress) bool {
	return accAddr.Equals(keeper.GetModuleAccAddress(AsgardName)) ||
		accAddr.Equals(keeper.GetModuleAccAddress(BondName)) ||
		accAddr.Equals(keeper.GetModuleAccAddress(ReserveName)) ||
		accAddr.Equals(keeper.GetModuleAccAddress(ModuleName)) ||
		accAddr.Equals(keeper.GetModuleAccAddress(MayaFund))
}

func isTradingHaltV65(ctx cosmos.Context, msg cosmos.Msg, mgr Manager) bool {
	switch m := msg.(type) {
	case *MsgSwap:
		whitelistedArbs := getWhitelistedArbs(ctx, mgr)
		for _, raw := range whitelistedArbs {
			address, err := common.NewAddress(strings.TrimSpace(raw), mgr.GetVersion())
			if err != nil {
				ctx.Logger().Error("fail to parse address for trading halt check", "address", raw, "error", err)
				continue
			}
			if address.Equals(m.Tx.FromAddress) {
				return false
			}
		}
		source := common.EmptyChain
		if len(m.Tx.Coins) > 0 {
			source = m.Tx.Coins[0].Asset.GetLayer1Asset().Chain
		}
		target := m.TargetAsset.GetLayer1Asset().Chain
		return isChainTradingHalted(ctx, mgr, source) || isChainTradingHalted(ctx, mgr, target) || isGlobalTradingHalted(ctx, mgr)
	case *MsgAddLiquidity:
		return isChainTradingHalted(ctx, mgr, m.Asset.Chain) || isGlobalTradingHalted(ctx, mgr)
	default:
		return isGlobalTradingHalted(ctx, mgr)
	}
}

func refundTxV110(ctx cosmos.Context, tx ObservedTx, mgr Manager, refundCode uint32, refundReason, sourceModuleName string) error {
	// If THORNode recognize one of the coins, and therefore able to refund
	// withholding fees, refund all coins.

	refundCoins := make(common.Coins, 0)
	for _, coin := range tx.Tx.Coins {
		if coin.Asset.IsBase() && coin.Asset.GetChain().Equals(common.ETHChain) {
			continue
		}
		pool, err := mgr.Keeper().GetPool(ctx, coin.Asset.GetLayer1Asset())
		if err != nil {
			return fmt.Errorf("fail to get pool: %w", err)
		}

		// Only attempt an outbound if a fee can be taken from the coin.
		if coin.Asset.IsNativeBase() || !pool.BalanceCacao.IsZero() {
			toi := TxOutItem{
				Chain:       coin.Asset.GetChain(),
				InHash:      tx.Tx.ID,
				ToAddress:   tx.Tx.FromAddress,
				VaultPubKey: tx.ObservedPubKey,
				Coin:        coin,
				Memo:        NewRefundMemo(tx.Tx.ID).String(),
				ModuleName:  sourceModuleName,
			}

			success, err := mgr.TxOutStore().TryAddTxOutItem(ctx, mgr, toi, cosmos.ZeroUint())
			if err != nil {
				ctx.Logger().Error("fail to prepare outbound tx", "error", err)
				// concatenate the refund failure to refundReason
				refundReason = fmt.Sprintf("%s; fail to refund (%s): %s", refundReason, toi.Coin.String(), err)

				unrefundableCoinCleanup(ctx, mgr, toi, "failed_refund")
			}
			if success {
				refundCoins = append(refundCoins, toi.Coin)
			}
		}
		// Zombie coins are just dropped.
	}

	// For refund events, emit the event after the txout attempt in order to include the 'fail to refund' reason if unsuccessful.
	eventRefund := NewEventRefund(refundCode, refundReason, tx.Tx, common.NewFee(common.Coins{}, cosmos.ZeroUint()))
	if len(refundCoins) > 0 {
		// create a new TX based on the coins thorchain refund , some of the coins thorchain doesn't refund
		// coin thorchain doesn't have pool with , likely airdrop
		newTx := common.NewTx(tx.Tx.ID, tx.Tx.FromAddress, tx.Tx.ToAddress, tx.Tx.Coins, tx.Tx.Gas, tx.Tx.Memo)
		eventRefund = NewEventRefund(refundCode, refundReason, newTx, common.Fee{}) // fee param not used in downstream event
	}
	if err := mgr.EventMgr().EmitEvent(ctx, eventRefund); err != nil {
		return fmt.Errorf("fail to emit refund event: %w", err)
	}

	return nil
}

func refundBondV108(
	ctx cosmos.Context,
	tx common.Tx,
	acc cosmos.AccAddress,
	asset common.Asset,
	units cosmos.Uint,
	nodeAcc *NodeAccount,
	mgr Manager,
) error {
	if nodeAcc.Status == NodeActive {
		ctx.Logger().Info("node still active, cannot refund bond", "node address", nodeAcc.NodeAddress, "node pub key", nodeAcc.PubKeySet.Secp256k1)
		return nil
	}

	// ensures nodes don't return bond while being churned into the network
	// (removing their bond last second)
	if nodeAcc.Status == NodeReady {
		ctx.Logger().Info("node ready, cannot refund bond", "node address", nodeAcc.NodeAddress, "node pub key", nodeAcc.PubKeySet.Secp256k1)
		return nil
	}

	nodeBond, err := mgr.Keeper().CalcNodeLiquidityBond(ctx, *nodeAcc)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get node liquidity bond (%s)", nodeAcc.NodeAddress))
	}

	ygg := Vault{}
	if mgr.Keeper().VaultExists(ctx, nodeAcc.PubKeySet.Secp256k1) {
		ygg, err = mgr.Keeper().GetVault(ctx, nodeAcc.PubKeySet.Secp256k1)
		if err != nil {
			return err
		}
		if !ygg.IsYggdrasil() {
			return errors.New("this is not a Yggdrasil vault")
		}
	}

	bp, err := mgr.Keeper().GetBondProviders(ctx, nodeAcc.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get bond providers(%s)", nodeAcc.NodeAddress))
	}

	// Calculate total value (in rune) the Yggdrasil pool has
	yggRune, err := getTotalYggValueInRune(ctx, mgr.Keeper(), ygg)
	if err != nil {
		return fmt.Errorf("fail to get total ygg value in RUNE: %w", err)
	}

	if nodeBond.LT(yggRune) {
		ctx.Logger().Error("Node Account left with more funds in their Yggdrasil vault than their bond's value", "address", nodeAcc.NodeAddress, "ygg-value", yggRune, "bond", nodeBond)
	}
	// slash yggdrasil remains
	penaltyPts := fetchConfigInt64(ctx, mgr, constants.SlashPenalty)
	slashRune := common.GetUncappedShare(cosmos.NewUint(uint64(penaltyPts)), cosmos.NewUint(10_000), yggRune)
	if slashRune.GT(nodeBond) {
		slashRune = nodeBond
	}

	slashedAmount, _, err := mgr.Slasher().SlashNodeAccountLP(ctx, *nodeAcc, slashRune)
	if err != nil {
		return ErrInternal(err, "fail to slash node account")
	}

	if slashedAmount.LT(slashRune) {
		ctx.Logger().Error("slashed amount is less than slash rune", "slashed amount", slashedAmount, "slash rune", slashRune)
	}

	provider := bp.Get(acc)

	assets := []common.Asset{asset}
	if asset.IsEmpty() {
		if !units.IsZero() {
			return fmt.Errorf("units must be zero when asset is empty")
		}

		// if asset is empty, it means we are refunding all the bonds
		liquidityPools := GetLiquidityPools(mgr.GetVersion())
		assets = liquidityPools
	}

	if !provider.IsEmpty() && !nodeBond.IsZero() {
		var totalLPBonded cosmos.Uint
		lps, err := mgr.Keeper().GetLiquidityProviderByAssets(ctx, assets, common.Address(acc.String()))
		if err != nil {
			return ErrInternal(err, fmt.Sprintf("fail to get liquidity provider %s, %s", acc, asset))
		}
		totalWithdrawnBondInCacao := cosmos.ZeroUint()
		for _, lp := range lps {
			withdrawUnits := units
			if withdrawUnits.IsZero() || withdrawUnits.GT(lp.Units) {
				withdrawUnits = lp.GetUnitsBondedToNode(nodeAcc.NodeAddress)
			}

			var withdrawnBondInCacao cosmos.Uint
			withdrawnBondInCacao, err = calcLiquidityInCacao(ctx, mgr, asset, withdrawUnits)
			if err != nil {
				return fmt.Errorf("fail to calc liquidity in CACAO: %w", err)
			}

			totalWithdrawnBondInCacao = totalWithdrawnBondInCacao.Add(withdrawnBondInCacao)
			// get total lp bonded value before unbonding lp
			totalLPBonded, err = mgr.Keeper().CalcLPLiquidityBond(ctx, common.Address(acc.String()), nodeAcc.NodeAddress)
			if err != nil {
				return fmt.Errorf("fail to calc lp liquidity bond: %w", err)
			}

			lp.Unbond(nodeAcc.NodeAddress, withdrawUnits)

			mgr.Keeper().SetLiquidityProvider(ctx, lp)

			// emit bond unlocked event
			fakeTx := common.Tx{}
			fakeTx.ID = common.BlankTxID
			fakeTx.FromAddress = nodeAcc.BondAddress
			fakeTx.ToAddress = common.Address(acc.String())
			unbondEvent := NewEventBondV105(lp.Asset, withdrawUnits, BondReturned, tx)
			if err = mgr.EventMgr().EmitEvent(ctx, unbondEvent); err != nil {
				ctx.Logger().Error("fail to emit unbond event", "error", err)
			}
		}

		if !totalWithdrawnBondInCacao.IsZero() {
			// If we are unbonding all of the units, remove the bond provider
			if totalLPBonded.Equal(totalWithdrawnBondInCacao) {
				bp.Unbond(provider.BondAddress)
			}
		}

		// calculate rewards for bond provider and payout
		if provider.HasRewards() {
			err = payBondProviderReward(ctx, mgr, provider, bp)
			if err != nil {
				return fmt.Errorf("fail to pay bond provider reward: %w", err)
			}
		}
	}

	if nodeAcc.RequestedToLeave {
		// when node already request to leave , it can't come back , here means the node already unbond
		// so set the node to disabled status
		nodeAcc.UpdateStatus(NodeDisabled, ctx.BlockHeight())
	}
	if err := mgr.Keeper().SetNodeAccount(ctx, *nodeAcc); err != nil {
		ctx.Logger().Error(fmt.Sprintf("fail to save node account(%s)", nodeAcc), "error", err)
		return err
	}
	if err := mgr.Keeper().SetBondProviders(ctx, bp); err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to save bond providers(%s)", bp.NodeAddress.String()))
	}

	if err := subsidizePoolsWithSlashBond(ctx, ygg.Coins, ygg, yggRune, slashedAmount, mgr); err != nil {
		ctx.Logger().Error("fail to subsidize pools with slash bond", "error", err)
	}
	// at this point , all coins in yggdrasil vault has been accounted for , and node already been slashed
	ygg.SubFunds(ygg.Coins)
	if err := mgr.Keeper().SetVault(ctx, ygg); err != nil {
		ctx.Logger().Error("fail to save yggdrasil vault", "error", err)
		return err
	}

	if err := mgr.Keeper().DeleteVault(ctx, ygg.PubKey); err != nil {
		return err
	}

	// Output bond events for the slashed and returned bond.
	if !slashRune.IsZero() {
		fakeTx := common.Tx{}
		fakeTx.ID = common.BlankTxID
		fakeTx.FromAddress = nodeAcc.BondAddress
		if err := mgr.EventMgr().EmitBondEvent(ctx, mgr, common.BaseNative, slashRune, BondCost, fakeTx); err != nil {
			ctx.Logger().Error("fail to emit bond event", "error", err)
		}
	}
	return nil
}

func refundBondV107(
	ctx cosmos.Context,
	tx common.Tx,
	acc cosmos.AccAddress,
	asset common.Asset,
	units cosmos.Uint,
	nodeAcc *NodeAccount,
	mgr Manager,
) error {
	if nodeAcc.Status == NodeActive {
		ctx.Logger().Info("node still active, cannot refund bond", "node address", nodeAcc.NodeAddress, "node pub key", nodeAcc.PubKeySet.Secp256k1)
		return nil
	}

	// ensures nodes don't return bond while being churned into the network
	// (removing their bond last second)
	if nodeAcc.Status == NodeReady {
		ctx.Logger().Info("node ready, cannot refund bond", "node address", nodeAcc.NodeAddress, "node pub key", nodeAcc.PubKeySet.Secp256k1)
		return nil
	}

	nodeBond, err := mgr.Keeper().CalcNodeLiquidityBond(ctx, *nodeAcc)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get node liquidity bond (%s)", nodeAcc.NodeAddress))
	}

	ygg := Vault{}
	if mgr.Keeper().VaultExists(ctx, nodeAcc.PubKeySet.Secp256k1) {
		ygg, err = mgr.Keeper().GetVault(ctx, nodeAcc.PubKeySet.Secp256k1)
		if err != nil {
			return err
		}
		if !ygg.IsYggdrasil() {
			return errors.New("this is not a Yggdrasil vault")
		}
	}

	bp, err := mgr.Keeper().GetBondProviders(ctx, nodeAcc.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get bond providers(%s)", nodeAcc.NodeAddress))
	}

	// Calculate total value (in rune) the Yggdrasil pool has
	yggRune, err := getTotalYggValueInRune(ctx, mgr.Keeper(), ygg)
	if err != nil {
		return fmt.Errorf("fail to get total ygg value in RUNE: %w", err)
	}

	if nodeBond.LT(yggRune) {
		ctx.Logger().Error("Node Account left with more funds in their Yggdrasil vault than their bond's value", "address", nodeAcc.NodeAddress, "ygg-value", yggRune, "bond", nodeBond)
	}
	// slash yggdrasil remains
	penaltyPts := fetchConfigInt64(ctx, mgr, constants.SlashPenalty)
	slashRune := common.GetUncappedShare(cosmos.NewUint(uint64(penaltyPts)), cosmos.NewUint(10_000), yggRune)
	if slashRune.GT(nodeBond) {
		slashRune = nodeBond
	}

	slashedAmount, _, err := mgr.Slasher().SlashNodeAccountLP(ctx, *nodeAcc, slashRune)
	if err != nil {
		return ErrInternal(err, "fail to slash node account")
	}

	if slashedAmount.LT(slashRune) {
		ctx.Logger().Error("slashed amount is less than slash rune", "slashed amount", slashedAmount, "slash rune", slashRune)
	}

	provider := bp.Get(acc)

	assets := []common.Asset{asset}
	if asset.IsEmpty() {
		if !units.IsZero() {
			return fmt.Errorf("units must be zero when asset is empty")
		}

		// if asset is empty, it means we are refunding all the bonds
		liquidityPools := GetLiquidityPools(mgr.GetVersion())
		assets = liquidityPools
	}

	if !provider.IsEmpty() && !nodeBond.IsZero() {
		lps, err := mgr.Keeper().GetLiquidityProviderByAssets(ctx, assets, common.Address(acc.String()))
		if err != nil {
			return ErrInternal(err, fmt.Sprintf("fail to get liquidity provider %s, %s", acc, asset))
		}
		totalWithdrawnBondInCacao := cosmos.ZeroUint()
		for _, lp := range lps {
			withdrawUnits := units
			if withdrawUnits.IsZero() || withdrawUnits.GT(lp.Units) {
				withdrawUnits = lp.GetUnitsBondedToNode(nodeAcc.NodeAddress)
			}

			var withdrawnBondInCacao cosmos.Uint
			withdrawnBondInCacao, err = calcLiquidityInCacao(ctx, mgr, asset, withdrawUnits)
			if err != nil {
				return fmt.Errorf("fail to calc liquidity in CACAO: %w", err)
			}

			totalWithdrawnBondInCacao = totalWithdrawnBondInCacao.Add(withdrawnBondInCacao)
			lp.Unbond(nodeAcc.NodeAddress, withdrawUnits)

			mgr.Keeper().SetLiquidityProvider(ctx, lp)

			// emit bond unlocked event
			fakeTx := common.Tx{}
			fakeTx.ID = common.BlankTxID
			fakeTx.FromAddress = nodeAcc.BondAddress
			fakeTx.ToAddress = common.Address(acc.String())
			unbondEvent := NewEventBondV105(lp.Asset, withdrawUnits, BondReturned, tx)
			if err = mgr.EventMgr().EmitEvent(ctx, unbondEvent); err != nil {
				ctx.Logger().Error("fail to emit unbond event", "error", err)
			}
		}

		if !totalWithdrawnBondInCacao.IsZero() {
			// If we are unbonding all of the units, remove the bond provider
			var totalLPBonded cosmos.Uint
			totalLPBonded, err = mgr.Keeper().CalcLPLiquidityBond(ctx, common.Address(acc.String()), nodeAcc.NodeAddress)
			if err != nil {
				return fmt.Errorf("fail to calc lp liquidity bond: %w", err)
			}
			if totalLPBonded.Equal(totalWithdrawnBondInCacao) {
				bp.Unbond(provider.BondAddress)
			}

		}

		// calculate rewards for bond provider and payout
		if provider.HasRewards() {
			err = payBondProviderReward(ctx, mgr, provider, bp)
			if err != nil {
				return fmt.Errorf("fail to pay bond provider reward: %w", err)
			}
		}
	}

	if nodeAcc.RequestedToLeave {
		// when node already request to leave , it can't come back , here means the node already unbond
		// so set the node to disabled status
		nodeAcc.UpdateStatus(NodeDisabled, ctx.BlockHeight())
	}
	if err := mgr.Keeper().SetNodeAccount(ctx, *nodeAcc); err != nil {
		ctx.Logger().Error(fmt.Sprintf("fail to save node account(%s)", nodeAcc), "error", err)
		return err
	}
	if err := mgr.Keeper().SetBondProviders(ctx, bp); err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to save bond providers(%s)", bp.NodeAddress.String()))
	}

	if err := subsidizePoolsWithSlashBond(ctx, ygg.Coins, ygg, yggRune, slashedAmount, mgr); err != nil {
		ctx.Logger().Error("fail to subsidize pools with slash bond", "error", err)
	}
	// at this point , all coins in yggdrasil vault has been accounted for , and node already been slashed
	ygg.SubFunds(ygg.Coins)
	if err := mgr.Keeper().SetVault(ctx, ygg); err != nil {
		ctx.Logger().Error("fail to save yggdrasil vault", "error", err)
		return err
	}

	if err := mgr.Keeper().DeleteVault(ctx, ygg.PubKey); err != nil {
		return err
	}

	// Output bond events for the slashed and returned bond.
	if !slashRune.IsZero() {
		fakeTx := common.Tx{}
		fakeTx.ID = common.BlankTxID
		fakeTx.FromAddress = nodeAcc.BondAddress
		if err := mgr.EventMgr().EmitBondEvent(ctx, mgr, common.BaseNative, slashRune, BondCost, fakeTx); err != nil {
			ctx.Logger().Error("fail to emit bond event", "error", err)
		}
	}
	return nil
}

func refundBondV105(
	ctx cosmos.Context,
	tx common.Tx,
	acc cosmos.AccAddress,
	asset common.Asset,
	units cosmos.Uint,
	nodeAcc *NodeAccount,
	mgr Manager,
) error {
	if nodeAcc.Status == NodeActive {
		ctx.Logger().Info("node still active, cannot refund bond", "node address", nodeAcc.NodeAddress, "node pub key", nodeAcc.PubKeySet.Secp256k1)
		return nil
	}

	// ensures nodes don't return bond while being churned into the network
	// (removing their bond last second)
	if nodeAcc.Status == NodeReady {
		ctx.Logger().Info("node ready, cannot refund bond", "node address", nodeAcc.NodeAddress, "node pub key", nodeAcc.PubKeySet.Secp256k1)
		return nil
	}

	nodeBond, err := mgr.Keeper().CalcNodeLiquidityBond(ctx, *nodeAcc)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get node liquidity bond (%s)", nodeAcc.NodeAddress))
	}

	ygg := Vault{}
	if mgr.Keeper().VaultExists(ctx, nodeAcc.PubKeySet.Secp256k1) {
		ygg, err = mgr.Keeper().GetVault(ctx, nodeAcc.PubKeySet.Secp256k1)
		if err != nil {
			return err
		}
		if !ygg.IsYggdrasil() {
			return errors.New("this is not a Yggdrasil vault")
		}
	}

	bp, err := mgr.Keeper().GetBondProviders(ctx, nodeAcc.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get bond providers(%s)", nodeAcc.NodeAddress))
	}

	// Calculate total value (in rune) the Yggdrasil pool has
	yggRune, err := getTotalYggValueInRune(ctx, mgr.Keeper(), ygg)
	if err != nil {
		return fmt.Errorf("fail to get total ygg value in RUNE: %w", err)
	}

	if nodeBond.LT(yggRune) {
		ctx.Logger().Error("Node Account left with more funds in their Yggdrasil vault than their bond's value", "address", nodeAcc.NodeAddress, "ygg-value", yggRune, "bond", nodeBond)
	}
	// slash yggdrasil remains
	penaltyPts := fetchConfigInt64(ctx, mgr, constants.SlashPenalty)
	slashRune := common.GetUncappedShare(cosmos.NewUint(uint64(penaltyPts)), cosmos.NewUint(10_000), yggRune)
	if slashRune.GT(nodeBond) {
		slashRune = nodeBond
	}

	slashedAmount, _, err := mgr.Slasher().SlashNodeAccountLP(ctx, *nodeAcc, slashRune)
	if err != nil {
		return ErrInternal(err, "fail to slash node account")
	}

	if slashedAmount.LT(slashRune) {
		ctx.Logger().Error("slashed amount is less than slash rune", "slashed amount", slashedAmount, "slash rune", slashRune)
	}

	provider := bp.Get(acc)

	assets := []common.Asset{asset}
	if asset.IsEmpty() {
		if !units.IsZero() {
			return fmt.Errorf("units must be zero when asset is empty")
		}

		// if asset is empty, it means we are refunding all the bonds
		liquidityPools := GetLiquidityPools(mgr.GetVersion())
		assets = liquidityPools
	}

	if !provider.IsEmpty() && !nodeBond.IsZero() {
		lps, err := mgr.Keeper().GetLiquidityProviderByAssets(ctx, assets, common.Address(acc.String()))
		if err != nil {
			return ErrInternal(err, fmt.Sprintf("fail to get liquidity provider %s, %s", acc, asset))
		}
		totalWithdrawnBondInCacao := cosmos.ZeroUint()
		for _, lp := range lps {
			withdrawUnits := units
			if withdrawUnits.IsZero() || withdrawUnits.GT(lp.Units) {
				withdrawUnits = lp.GetUnitsBondedToNode(nodeAcc.NodeAddress)
			}

			withdrawnBondInCacao, err := calcLiquidityInCacao(ctx, mgr, asset, withdrawUnits)
			if err != nil {
				return fmt.Errorf("fail to calc liquidity in CACAO: %w", err)
			}

			totalWithdrawnBondInCacao = totalWithdrawnBondInCacao.Add(withdrawnBondInCacao)
			lp.Unbond(nodeAcc.NodeAddress, withdrawUnits)

			mgr.Keeper().SetLiquidityProvider(ctx, lp)

			// emit bond returned event
			fakeTx := common.Tx{}
			fakeTx.ID = common.BlankTxID
			fakeTx.FromAddress = nodeAcc.BondAddress
			fakeTx.ToAddress = common.Address(acc.String())
			unbondEvent := NewEventBondV105(lp.Asset, withdrawUnits, BondReturned, tx)
			if err := mgr.EventMgr().EmitEvent(ctx, unbondEvent); err != nil {
				ctx.Logger().Error("fail to emit unbond event", "error", err)
			}
		}

		if !totalWithdrawnBondInCacao.IsZero() {
			// If we are unbonding all of the units, remove the bond provider
			totalLPBonded, err := mgr.Keeper().CalcLPLiquidityBond(ctx, common.Address(acc.String()), nodeAcc.NodeAddress)
			if err != nil {
				return fmt.Errorf("fail to calc lp liquidity bond: %w", err)
			}
			if totalLPBonded.Equal(totalWithdrawnBondInCacao) {
				bp.Unbond(provider.BondAddress)
			}

			// calculate rewards for bond provider
			// Rewards * (withdrawnBondInCACAO / NodeBond)
			if !nodeAcc.Reward.IsZero() {
				toAddress, err := common.NewAddress(provider.BondAddress.String(), GetCurrentVersion())
				if err != nil {
					return fmt.Errorf("fail to parse bond address: %w", err)
				}

				bondRewards := common.GetSafeShare(totalWithdrawnBondInCacao, nodeBond, nodeAcc.Reward)
				nodeAcc.Reward = common.SafeSub(nodeAcc.Reward, bondRewards)

				// refund bond rewards
				txOutItem := TxOutItem{
					Chain:      common.BaseAsset().Chain,
					ToAddress:  toAddress,
					InHash:     tx.ID,
					Coin:       common.NewCoin(common.BaseAsset(), bondRewards),
					ModuleName: BondName,
				}
				_, err = mgr.TxOutStore().TryAddTxOutItem(ctx, mgr, txOutItem, cosmos.ZeroUint())
				if err != nil {
					return fmt.Errorf("fail to add outbound tx: %w", err)
				}

				bondEvent := NewEventBondV105(common.BaseNative, bondRewards, BondReturned, tx)
				if err := mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
					ctx.Logger().Error("fail to emit bond event", "error", err)
				}
			}
		}
	}

	if nodeAcc.RequestedToLeave {
		// when node already request to leave , it can't come back , here means the node already unbond
		// so set the node to disabled status
		nodeAcc.UpdateStatus(NodeDisabled, ctx.BlockHeight())
	}
	if err := mgr.Keeper().SetNodeAccount(ctx, *nodeAcc); err != nil {
		ctx.Logger().Error(fmt.Sprintf("fail to save node account(%s)", nodeAcc), "error", err)
		return err
	}
	if err := mgr.Keeper().SetBondProviders(ctx, bp); err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to save bond providers(%s)", bp.NodeAddress.String()))
	}

	if err := subsidizePoolsWithSlashBond(ctx, ygg.Coins, ygg, yggRune, slashedAmount, mgr); err != nil {
		ctx.Logger().Error("fail to subsidize pools with slash bond", "error", err)
	}
	// at this point , all coins in yggdrasil vault has been accounted for , and node already been slashed
	ygg.SubFunds(ygg.Coins)
	if err := mgr.Keeper().SetVault(ctx, ygg); err != nil {
		ctx.Logger().Error("fail to save yggdrasil vault", "error", err)
		return err
	}

	if err := mgr.Keeper().DeleteVault(ctx, ygg.PubKey); err != nil {
		return err
	}

	// Output bond events for the slashed and returned bond.
	if !slashRune.IsZero() {
		fakeTx := common.Tx{}
		fakeTx.ID = common.BlankTxID
		fakeTx.FromAddress = nodeAcc.BondAddress
		if err := mgr.EventMgr().EmitBondEvent(ctx, mgr, common.BaseNative, slashRune, BondCost, fakeTx); err != nil {
			ctx.Logger().Error("fail to emit bond event", "error", err)
		}
	}
	return nil
}

func refundTxV47(ctx cosmos.Context, tx ObservedTx, mgr Manager, refundCode uint32, refundReason, nativeRuneModuleName string) error {
	// If THORNode recognize one of the coins, and therefore able to refund
	// withholding fees, refund all coins.

	addEvent := func(refundCoins common.Coins) error {
		eventRefund := NewEventRefund(refundCode, refundReason, tx.Tx, common.NewFee(common.Coins{}, cosmos.ZeroUint()))
		if len(refundCoins) > 0 {
			// create a new TX based on the coins thorchain refund , some of the coins thorchain doesn't refund
			// coin thorchain doesn't have pool with , likely airdrop
			newTx := common.NewTx(tx.Tx.ID, tx.Tx.FromAddress, tx.Tx.ToAddress, tx.Tx.Coins, tx.Tx.Gas, tx.Tx.Memo)

			// all the coins in tx.Tx should belongs to the same chain
			transactionFee := mgr.GasMgr().GetFee(ctx, tx.Tx.Chain, common.BaseAsset())
			fee := getFee(tx.Tx.Coins, refundCoins, transactionFee)
			eventRefund = NewEventRefund(refundCode, refundReason, newTx, fee)
		}
		if err := mgr.EventMgr().EmitEvent(ctx, eventRefund); err != nil {
			return fmt.Errorf("fail to emit refund event: %w", err)
		}
		return nil
	}

	// for BASEChain transactions, create the event before we txout. For other
	// chains, do it after. The reason for this is we need to make sure the
	// first event (refund) is created, before we create the outbound events
	// (second). Because its BASEChain, its safe to assume all the coins are
	// safe to send back. Where as for external coins, we cannot make this
	// assumption (ie coins we don't have pools for and therefore, don't know
	// the value of it relative to rune)
	if tx.Tx.Chain.Equals(common.BASEChain) {
		if err := addEvent(tx.Tx.Coins); err != nil {
			return err
		}
	}
	refundCoins := make(common.Coins, 0)
	for _, coin := range tx.Tx.Coins {
		if coin.Asset.IsBase() && coin.Asset.GetChain().Equals(common.ETHChain) {
			continue
		}
		pool, err := mgr.Keeper().GetPool(ctx, coin.Asset)
		if err != nil {
			return fmt.Errorf("fail to get pool: %w", err)
		}

		if coin.Asset.IsBase() || !pool.BalanceCacao.IsZero() {
			toi := TxOutItem{
				Chain:       coin.Asset.GetChain(),
				InHash:      tx.Tx.ID,
				ToAddress:   tx.Tx.FromAddress,
				VaultPubKey: tx.ObservedPubKey,
				Coin:        coin,
				Memo:        NewRefundMemo(tx.Tx.ID).String(),
				ModuleName:  nativeRuneModuleName,
			}

			success, err := mgr.TxOutStore().TryAddTxOutItem(ctx, mgr, toi, cosmos.ZeroUint())
			if err != nil {
				ctx.Logger().Error("fail to prepare outbund tx", "error", err)
				// concatenate the refund failure to refundReason
				refundReason = fmt.Sprintf("%s; fail to refund (%s): %s", refundReason, toi.Coin.String(), err)
			}
			if success {
				refundCoins = append(refundCoins, toi.Coin)
			}
		}
		// Zombie coins are just dropped.
	}
	if !tx.Tx.Chain.Equals(common.BASEChain) {
		if err := addEvent(refundCoins); err != nil {
			return err
		}
	}

	return nil
}

func refundTxV104(ctx cosmos.Context, tx ObservedTx, mgr Manager, refundCode uint32, refundReason, nativeRuneModuleName string) error {
	// If THORNode recognize one of the coins, and therefore able to refund
	// withholding fees, refund all coins.

	addEvent := func(refundCoins common.Coins) error {
		eventRefund := NewEventRefund(refundCode, refundReason, tx.Tx, common.NewFee(common.Coins{}, cosmos.ZeroUint()))
		if len(refundCoins) > 0 {
			// create a new TX based on the coins thorchain refund , some of the coins thorchain doesn't refund
			// coin thorchain doesn't have pool with , likely airdrop
			newTx := common.NewTx(tx.Tx.ID, tx.Tx.FromAddress, tx.Tx.ToAddress, tx.Tx.Coins, tx.Tx.Gas, tx.Tx.Memo)

			// all the coins in tx.Tx should belongs to the same chain
			transactionFee := mgr.GasMgr().GetFee(ctx, tx.Tx.Chain, common.BaseAsset())
			fee := getFee(tx.Tx.Coins, refundCoins, transactionFee)
			eventRefund = NewEventRefund(refundCode, refundReason, newTx, fee)
		}
		if err := mgr.EventMgr().EmitEvent(ctx, eventRefund); err != nil {
			return fmt.Errorf("fail to emit refund event: %w", err)
		}
		return nil
	}

	// for BASEChain transactions, create the event before we txout. For other
	// chains, do it after. The reason for this is we need to make sure the
	// first event (refund) is created, before we create the outbound events
	// (second). Because its BASEChain, its safe to assume all the coins are
	// safe to send back. Where as for external coins, we cannot make this
	// assumption (ie coins we don't have pools for and therefore, don't know
	// the value of it relative to rune)
	if tx.Tx.Chain.Equals(common.BASEChain) {
		if err := addEvent(tx.Tx.Coins); err != nil {
			return err
		}
	}
	refundCoins := make(common.Coins, 0)
	for _, coin := range tx.Tx.Coins {
		if coin.Asset.IsBase() && coin.Asset.GetChain().Equals(common.ETHChain) {
			continue
		}
		pool, err := mgr.Keeper().GetPool(ctx, coin.Asset.GetLayer1Asset())
		if err != nil {
			return fmt.Errorf("fail to get pool: %w", err)
		}

		if coin.Asset.IsBase() || !pool.BalanceCacao.IsZero() {
			toi := TxOutItem{
				Chain:       coin.Asset.GetChain(),
				InHash:      tx.Tx.ID,
				ToAddress:   tx.Tx.FromAddress,
				VaultPubKey: tx.ObservedPubKey,
				Coin:        coin,
				Memo:        NewRefundMemo(tx.Tx.ID).String(),
				ModuleName:  nativeRuneModuleName,
			}

			var success bool
			success, err = mgr.TxOutStore().TryAddTxOutItem(ctx, mgr, toi, cosmos.ZeroUint())
			if err != nil {
				ctx.Logger().Error("fail to prepare outbund tx", "error", err)
				// concatenate the refund failure to refundReason
				refundReason = fmt.Sprintf("%s; fail to refund (%s): %s", refundReason, toi.Coin.String(), err)
			}
			if success {
				refundCoins = append(refundCoins, toi.Coin)
			}
		}
		// Zombie coins are just dropped.
	}
	if !tx.Tx.Chain.Equals(common.BASEChain) {
		if err := addEvent(refundCoins); err != nil {
			return err
		}
	}

	return nil
}

func refundBondV92(ctx cosmos.Context, tx common.Tx, acc cosmos.AccAddress, nodeAcc *NodeAccount, mgr Manager) error {
	if nodeAcc.Status == NodeActive {
		ctx.Logger().Info("node still active, cannot refund bond", "node address", nodeAcc.NodeAddress, "node pub key", nodeAcc.PubKeySet.Secp256k1)
		return nil
	}

	// ensures nodes don't return bond while being churned into the network
	// (removing their bond last second)
	if nodeAcc.Status == NodeReady {
		ctx.Logger().Info("node ready, cannot refund bond", "node address", nodeAcc.NodeAddress, "node pub key", nodeAcc.PubKeySet.Secp256k1)
		return nil
	}

	nodeBond, err := mgr.Keeper().CalcNodeLiquidityBond(ctx, *nodeAcc)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get node liquidity bond (%s)", nodeAcc.NodeAddress))
	}

	ygg := Vault{}
	if mgr.Keeper().VaultExists(ctx, nodeAcc.PubKeySet.Secp256k1) {
		ygg, err = mgr.Keeper().GetVault(ctx, nodeAcc.PubKeySet.Secp256k1)
		if err != nil {
			return err
		}
		if !ygg.IsYggdrasil() {
			return errors.New("this is not a Yggdrasil vault")
		}
	}

	bp, err := mgr.Keeper().GetBondProviders(ctx, nodeAcc.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get bond providers(%s)", nodeAcc.NodeAddress))
	}

	// Calculate total value (in rune) the Yggdrasil pool has
	yggRune, err := getTotalYggValueInRune(ctx, mgr.Keeper(), ygg)
	if err != nil {
		return fmt.Errorf("fail to get total ygg value in RUNE: %w", err)
	}

	if nodeBond.LT(yggRune) {
		ctx.Logger().Error("Node Account left with more funds in their Yggdrasil vault than their bond's value", "address", nodeAcc.NodeAddress, "ygg-value", yggRune, "bond", nodeBond)
	}
	// slash yggdrasil remains
	penaltyPts := fetchConfigInt64(ctx, mgr, constants.SlashPenalty)
	slashRune := common.GetUncappedShare(cosmos.NewUint(uint64(penaltyPts)), cosmos.NewUint(10_000), yggRune)
	if slashRune.GT(nodeBond) {
		slashRune = nodeBond
	}

	slashedAmount, _, err := mgr.Slasher().SlashNodeAccountLP(ctx, *nodeAcc, slashRune)
	if err != nil {
		return ErrInternal(err, "fail to slash node account")
	}

	if slashedAmount.LT(slashRune) {
		ctx.Logger().Error("slashed amount is less than slash rune", "slashed amount", slashedAmount, "slash rune", slashRune)
	}

	provider := bp.Get(acc)

	nodeBond, err = mgr.Keeper().CalcNodeLiquidityBond(ctx, *nodeAcc)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get node liquidity bond (%s)", nodeAcc.NodeAddress))
	}

	providerBond, err := mgr.Keeper().CalcLPLiquidityBond(ctx, common.Address(provider.BondAddress.String()), nodeAcc.NodeAddress)
	if err != nil {
		return ErrInternal(err, "fail to get bond provider liquidity")
	}

	if !provider.IsEmpty() && !nodeBond.IsZero() && !providerBond.IsZero() {

		bp.Unbond(provider.BondAddress)

		toAddress, err := common.NewAddress(provider.BondAddress.String(), GetCurrentVersion())
		if err != nil {
			return fmt.Errorf("fail to parse bond address: %w", err)
		}

		// calculate rewards for bond provider
		//  Rewards * (ProviderBond / NodeBond)
		if !nodeAcc.Reward.IsZero() {
			bondRewards := common.GetSafeShare(providerBond, nodeBond, nodeAcc.Reward)
			nodeAcc.Reward = common.SafeSub(nodeAcc.Reward, bondRewards)

			// refund bond
			txOutItem := TxOutItem{
				Chain:      common.BaseAsset().Chain,
				ToAddress:  toAddress,
				InHash:     tx.ID,
				Coin:       common.NewCoin(common.BaseAsset(), bondRewards),
				ModuleName: BondName,
			}
			_, err = mgr.TxOutStore().TryAddTxOutItem(ctx, mgr, txOutItem, cosmos.ZeroUint())
			if err != nil {
				return fmt.Errorf("fail to add outbound tx: %w", err)
			}

			bondEvent := NewEventBond(bondRewards, BondReturned, tx)
			if err := mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
				ctx.Logger().Error("fail to emit bond event", "error", err)
			}
		}
	}

	if nodeAcc.RequestedToLeave {
		// when node already request to leave , it can't come back , here means the node already unbond
		// so set the node to disabled status
		nodeAcc.UpdateStatus(NodeDisabled, ctx.BlockHeight())
	}
	if err := mgr.Keeper().SetNodeAccount(ctx, *nodeAcc); err != nil {
		ctx.Logger().Error(fmt.Sprintf("fail to save node account(%s)", nodeAcc), "error", err)
		return err
	}
	if err := mgr.Keeper().SetBondProviders(ctx, bp); err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to save bond providers(%s)", bp.NodeAddress.String()))
	}

	if err := subsidizePoolsWithSlashBond(ctx, ygg.Coins, ygg, yggRune, slashedAmount, mgr); err != nil {
		ctx.Logger().Error("fail to subsidize pools with slash bond", "error", err)
	}
	// at this point , all coins in yggdrasil vault has been accounted for , and node already been slashed
	ygg.SubFunds(ygg.Coins)
	if err := mgr.Keeper().SetVault(ctx, ygg); err != nil {
		ctx.Logger().Error("fail to save yggdrasil vault", "error", err)
		return err
	}

	if err := mgr.Keeper().DeleteVault(ctx, ygg.PubKey); err != nil {
		return err
	}

	// Output bond events for the slashed and returned bond.
	if !slashRune.IsZero() {
		fakeTx := common.Tx{}
		fakeTx.ID = common.BlankTxID
		fakeTx.FromAddress = nodeAcc.BondAddress
		bondEvent := NewEventBond(slashRune, BondCost, fakeTx)
		if err := mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
			ctx.Logger().Error("fail to emit bond event", "error", err)
		}
	}
	return nil
}

// addGasFees to vault
func addGasFeesV1(ctx cosmos.Context, mgr Manager, tx ObservedTx) error {
	if len(tx.Tx.Gas) == 0 {
		return nil
	}
	if mgr.Keeper().RagnarokInProgress(ctx) {
		// when ragnarok is in progress, if the tx is for gas coin then doesn't subsidise the pool with reserve
		// liquidity providers they need to pay their own gas
		// if the outbound coin is not gas asset, then reserve will subsidise it , otherwise the gas asset pool will be in a loss
		gasAsset := tx.Tx.Chain.GetGasAsset()
		if tx.Tx.Coins.GetCoin(gasAsset).IsEmpty() {
			mgr.GasMgr().AddGasAsset(tx.Tx.Gas, true)
		}
	} else {
		mgr.GasMgr().AddGasAsset(tx.Tx.Gas, true)
	}
	// Subtract from the vault
	if mgr.Keeper().VaultExists(ctx, tx.ObservedPubKey) {
		vault, err := mgr.Keeper().GetVault(ctx, tx.ObservedPubKey)
		if err != nil {
			return err
		}

		vault.SubFunds(tx.Tx.Gas.ToCoins())

		if err := mgr.Keeper().SetVault(ctx, vault); err != nil {
			return err
		}
	}
	return nil
}

func addSwapDirectV112(ctx cosmos.Context, mgr Manager, msg MsgSwap) {
	if msg.Tx.Coins.IsEmpty() {
		return
	}

	totalAffFee := msg.GetTotalAffiliateFee()
	if !totalAffFee.IsZero() {
		// Distribute affiliate fees
		totalAffiliateFee, err := skimAffiliateFees(ctx, mgr, msg.Tx, msg.Signer, msg.Tx.Memo)
		if err != nil {
			ctx.Logger().Error("fail to skim affiliate fees", "error", err)
		}
		// Reduce main swap amount by total distributed affiliate fee
		msg.Tx.Coins[0].Amount = common.SafeSub(msg.Tx.Coins[0].Amount, totalAffiliateFee)
	}

	// Queue the main swap
	if err := mgr.Keeper().SetSwapQueueItem(ctx, msg, 0); err != nil {
		ctx.Logger().Error("fail to add swap to queue", "error", err)
	}
}

func getPreferredAssetSwapThresholdV1(ctx cosmos.Context, mgr Manager, preferredAsset common.Asset) cosmos.Uint {
	// trigger the preferred asset swap if accrued CACAO is greater than the value of
	// PreferredAssetOutboundFeeMultiplier (default = 100) x current outbound fee of preferred asset chain
	if preferredAsset.IsEmpty() {
		return cosmos.ZeroUint()
	}
	ofCacao := mgr.GasMgr().GetFee(ctx, preferredAsset.Chain, common.BaseNative)
	multiplier := fetchConfigInt64(ctx, mgr, constants.PreferredAssetOutboundFeeMultiplier)
	threshold := ofCacao.Mul(cosmos.NewUint(uint64(multiplier)))
	ctx.Logger().Debug("getPreferredAssetSwapThreshold", "preferredAsset", preferredAsset, "ofCacao", ofCacao, "multiplier", multiplier, "threshold", threshold)
	return threshold
}

func refundTxV112(ctx cosmos.Context, tx ObservedTx, mgr Manager, refundCode uint32, refundReason, nativeRuneModuleName string) error {
	// If THORNode recognize one of the coins, and therefore able to refund
	// withholding fees, refund all coins.

	refundCoins := make(common.Coins, 0)
	for _, coin := range tx.Tx.Coins {
		if coin.Asset.IsBase() && coin.Asset.GetChain().Equals(common.ETHChain) {
			continue
		}
		pool, err := mgr.Keeper().GetPool(ctx, coin.Asset.GetLayer1Asset())
		if err != nil {
			return fmt.Errorf("fail to get pool: %w", err)
		}

		// Only attempt an outbound if a fee can be taken from the coin.
		if coin.Asset.IsBase() || !pool.BalanceCacao.IsZero() {
			toAddr := tx.Tx.FromAddress
			memo, err := ParseMemoWithMAYANames(ctx, mgr.Keeper(), tx.Tx.Memo)
			if err == nil && memo.IsType(TxSwap) && !memo.GetRefundAddress().IsEmpty() && !coin.Asset.GetChain().IsBASEChain() {
				// If the memo specifies a refund address, send the refund to that address. If
				// refund memo can't be parsed or is invalid for the refund chain, it will
				// default back to the sender address
				if memo.GetRefundAddress().IsChain(coin.Asset.GetChain(), mgr.GetVersion()) {
					toAddr = memo.GetRefundAddress()
				}
			}

			toi := TxOutItem{
				Chain:       coin.Asset.GetChain(),
				InHash:      tx.Tx.ID,
				ToAddress:   toAddr,
				VaultPubKey: tx.ObservedPubKey,
				Coin:        coin,
				Memo:        NewRefundMemo(tx.Tx.ID).String(),
				ModuleName:  nativeRuneModuleName,
			}

			success, err := mgr.TxOutStore().TryAddTxOutItem(ctx, mgr, toi, cosmos.ZeroUint())
			if err != nil {
				ctx.Logger().Error("fail to prepare outbound tx", "error", err)
				// concatenate the refund failure to refundReason
				refundReason = fmt.Sprintf("%s; fail to refund (%s): %s", refundReason, toi.Coin.String(), err)

				unrefundableCoinCleanup(ctx, mgr, toi, "failed_refund")
			}
			if success {
				refundCoins = append(refundCoins, toi.Coin)
			}
		}
		// Zombie coins are just dropped.
	}

	// For refund events, emit the event after the txout attempt in order to include the 'fail to refund' reason if unsuccessful.
	eventRefund := NewEventRefund(refundCode, refundReason, tx.Tx, common.NewFee(common.Coins{}, cosmos.ZeroUint()))
	if len(refundCoins) > 0 {
		// create a new TX based on the coins thorchain refund , some of the coins thorchain doesn't refund
		// coin thorchain doesn't have pool with , likely airdrop
		newTx := common.NewTx(tx.Tx.ID, tx.Tx.FromAddress, tx.Tx.ToAddress, tx.Tx.Coins, tx.Tx.Gas, tx.Tx.Memo)
		// all the coins in tx.Tx should belongs to the same chain
		transactionFee := mgr.GasMgr().GetFee(ctx, tx.Tx.Chain, common.BaseNative)
		fee := getFee(tx.Tx.Coins, refundCoins, transactionFee)
		eventRefund = NewEventRefund(refundCode, refundReason, newTx, fee)
	}
	if err := mgr.EventMgr().EmitEvent(ctx, eventRefund); err != nil {
		return fmt.Errorf("fail to emit refund event: %w", err)
	}

	return nil
}

func refundTxV123(ctx cosmos.Context, tx ObservedTx, mgr Manager, refundCode uint32, refundReason, nativeRuneModuleName string) error {
	// If THORNode recognize one of the coins, and therefore able to refund
	// withholding fees, refund all coins.

	refundCoins := make(common.Coins, 0)
	for _, coin := range tx.Tx.Coins {
		// Do not emit Trade Asset refund event or attempt refund txout,
		// as Trade Asset withdrawals take place in the internal handlers
		// (state changes and event emission only if the internal handler succeeds)
		// and not in the deposit handler.
		if coin.Asset.IsTradeAsset() {
			continue
		}

		if coin.Asset.IsBase() && coin.Asset.GetChain().Equals(common.ETHChain) {
			continue
		}
		pool, err := mgr.Keeper().GetPool(ctx, coin.Asset.GetLayer1Asset())
		if err != nil {
			return fmt.Errorf("fail to get pool: %w", err)
		}

		// Only attempt an outbound if a fee can be taken from the coin.
		if coin.Asset.IsBase() || !pool.BalanceCacao.IsZero() {
			toAddr := tx.Tx.FromAddress
			memo, err := ParseMemoWithMAYANames(ctx, mgr.Keeper(), tx.Tx.Memo)
			if err == nil && memo.IsType(TxSwap) && !memo.GetRefundAddress().IsEmpty() && !coin.Asset.GetChain().IsBASEChain() {
				// If the memo specifies a refund address, send the refund to that address. If
				// refund memo can't be parsed or is invalid for the refund chain, it will
				// default back to the sender address
				if memo.GetRefundAddress().IsChain(coin.Asset.GetChain(), mgr.GetVersion()) {
					toAddr = memo.GetRefundAddress()
				}
			}

			toi := TxOutItem{
				Chain:       coin.Asset.GetChain(),
				InHash:      tx.Tx.ID,
				ToAddress:   toAddr,
				VaultPubKey: tx.ObservedPubKey,
				Coin:        coin,
				Memo:        NewRefundMemo(tx.Tx.ID).String(),
				ModuleName:  nativeRuneModuleName,
			}

			success, err := mgr.TxOutStore().TryAddTxOutItem(ctx, mgr, toi, cosmos.ZeroUint())
			if err != nil {
				ctx.Logger().Error("fail to prepare outbound tx", "error", err)
				// concatenate the refund failure to refundReason
				refundReason = fmt.Sprintf("%s; fail to refund (%s): %s", refundReason, toi.Coin.String(), err)

				unrefundableCoinCleanup(ctx, mgr, toi, "failed_refund")
			}
			if success {
				refundCoins = append(refundCoins, toi.Coin)
			}
		}
		// Zombie coins are just dropped.
	}

	// For refund events, emit the event after the txout attempt in order to include the 'fail to refund' reason if unsuccessful.
	eventRefund := NewEventRefund(refundCode, refundReason, tx.Tx, common.NewFee(common.Coins{}, cosmos.ZeroUint()))
	if len(refundCoins) > 0 {
		// create a new TX based on the coins thorchain refund , some of the coins thorchain doesn't refund
		// coin thorchain doesn't have pool with , likely airdrop
		newTx := common.NewTx(tx.Tx.ID, tx.Tx.FromAddress, tx.Tx.ToAddress, tx.Tx.Coins, tx.Tx.Gas, tx.Tx.Memo)
		// all the coins in tx.Tx should belongs to the same chain
		transactionFee := mgr.GasMgr().GetFee(ctx, tx.Tx.Chain, common.BaseNative)
		fee := getFee(tx.Tx.Coins, refundCoins, transactionFee)
		eventRefund = NewEventRefund(refundCode, refundReason, newTx, fee)
	}
	if err := mgr.EventMgr().EmitEvent(ctx, eventRefund); err != nil {
		return fmt.Errorf("fail to emit refund event: %w", err)
	}

	return nil
}
