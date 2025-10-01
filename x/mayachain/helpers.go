package mayachain

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/armon/go-metrics"
	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/telemetry"
	"github.com/hashicorp/go-multierror"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

var WhitelistedArbs = []string{ // treasury addresses
	"maya1a7gg93dgwlulsrqf6qtage985ujhpu068zllw7",
	"thor1a7gg93dgwlulsrqf6qtage985ujhpu0684pncw",
	"bc1qztdn5395243l3zwskwdxaghgrgs8swy5fjrhls",
	"0xef1c6f153afaf86424fd984728d32535902f1c3d",
	"bnb13sjakc98xrjz4we6d8a546xvlvrzver3pdfhap",
}

var WhitelistedArbsV107 = []string{ // treasury addresses
	"maya1a7gg93dgwlulsrqf6qtage985ujhpu068zllw7",
	"thor1a7gg93dgwlulsrqf6qtage985ujhpu0684pncw",
	"bc1qztdn5395243l3zwskwdxaghgrgs8swy5fjrhls",
	"0xef1c6f153afaf86424fd984728d32535902f1c3d",
	"bnb13sjakc98xrjz4we6d8a546xvlvrzver3pdfhap",
	"XmjZcjdymUo79tJikPJxKQWQuRVbcqkFp5",
	"kujira1a7gg93dgwlulsrqf6qtage985ujhpu06s66sqm",
}

func refundTx(ctx cosmos.Context, tx ObservedTx, mgr Manager, refundCode uint32, refundReason, sourceModuleName string) error {
	version := mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.124.0")): // trade-accounts
		return refundTxV124(ctx, tx, mgr, refundCode, refundReason, sourceModuleName)
	case version.GTE(semver.MustParse("1.123.0")): // trade-accounts
		return refundTxV123(ctx, tx, mgr, refundCode, refundReason, sourceModuleName)
	case version.GTE(semver.MustParse("1.112.0")):
		return refundTxV112(ctx, tx, mgr, refundCode, refundReason, sourceModuleName)
	case version.GTE(semver.MustParse("1.110.0")):
		return refundTxV110(ctx, tx, mgr, refundCode, refundReason, sourceModuleName)
	case version.GTE(semver.MustParse("1.104.0")):
		return refundTxV104(ctx, tx, mgr, refundCode, refundReason, sourceModuleName)
	case version.GTE(semver.MustParse("0.47.0")):
		return refundTxV47(ctx, tx, mgr, refundCode, refundReason, sourceModuleName)
	default:
		return errBadVersion
	}
}

func refundTxV124(ctx cosmos.Context, tx ObservedTx, mgr Manager, refundCode uint32, refundReason, nativeRuneModuleName string) error {
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

			var vault Vault
			if !tx.ObservedPubKey.IsEmpty() {
				vault, err = mgr.Keeper().GetVault(ctx, tx.ObservedPubKey)
				if err != nil {
					return fmt.Errorf("fail to get vault: %w", err)
				}
			}

			toi := TxOutItem{
				Chain:                 coin.Asset.GetChain(),
				ToAddress:             toAddr,
				VaultPubKey:           vault.PubKey,
				VaultPubKeyEddsa:      vault.PubKeyEddsa,
				Coin:                  coin,
				Memo:                  NewRefundMemo(tx.Tx.ID).String(),
				MaxGas:                []common.Coin{},
				GasRate:               0,
				InHash:                tx.Tx.ID,
				OutHash:               "",
				ModuleName:            nativeRuneModuleName,
				Aggregator:            "",
				AggregatorTargetAsset: "",
				AggregatorTargetLimit: &cosmos.Uint{},
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

func getFee(input, output common.Coins, transactionFee cosmos.Uint) common.Fee {
	var fee common.Fee
	assetTxCount := 0
	for _, out := range output {
		if !out.Asset.IsBase() {
			assetTxCount++
		}
	}
	for _, in := range input {
		outCoin := common.NoCoin
		for _, out := range output {
			if out.Asset.Equals(in.Asset) {
				outCoin = out
				break
			}
		}
		if outCoin.IsEmpty() {
			if !in.Amount.IsZero() {
				fee.Coins = append(fee.Coins, common.NewCoin(in.Asset, in.Amount))
			}
		} else {
			if !in.Amount.Sub(outCoin.Amount).IsZero() {
				fee.Coins = append(fee.Coins, common.NewCoin(in.Asset, in.Amount.Sub(outCoin.Amount)))
			}
		}
	}
	fee.PoolDeduct = transactionFee.MulUint64(uint64(assetTxCount))
	return fee
}

func subsidizePoolsWithSlashBond(ctx cosmos.Context, coins common.Coins, vault Vault, totalBaseStolen, slashedAmount cosmos.Uint, mgr Manager) error {
	version := mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.92.0")):
		return subsidizePoolWithSlashBondV92(ctx, coins, vault, totalBaseStolen, slashedAmount, mgr)
	default:
		return errBadVersion
	}
}

func subsidizePoolWithSlashBondV92(ctx cosmos.Context, coins common.Coins, vault Vault, totalBaseStolen cosmos.Uint, slashedAmount cosmos.Uint, mgr Manager) error {
	// Should never happen, but this prevents a divide-by-zero panic in case it does
	if totalBaseStolen.IsZero() || slashedAmount.IsZero() {
		ctx.Logger().Info("no stolen assets, no need to subsidize pools", "vault", vault.PubKey.String(), "type", vault.Type, "stolen", totalBaseStolen)
		return nil
	}

	polAddress, err := mgr.Keeper().GetModuleAddress(ReserveName)
	if err != nil {
		return err
	}

	// Calc the liquidity POL has on liquidity pools. Since these are the pools
	// we slashed the liquidity to the nodes
	polLiquidity, err := mgr.Keeper().CalcTotalBondableLiquidity(ctx, polAddress)
	if err != nil {
		return err
	}

	if polLiquidity.LT(totalBaseStolen) {
		ctx.Logger().Error("vault has more stolen assets than POL", "vault", vault.PubKey.String(), "type", vault.Type, "stolen", totalBaseStolen, "pol", polLiquidity)
		totalBaseStolen = polLiquidity
	}

	type fund struct {
		asset         common.Asset
		stolenAsset   cosmos.Uint
		subsidiseRune cosmos.Uint
	}
	subsidize := make([]fund, 0)
	for _, coin := range coins {
		if coin.IsEmpty() {
			continue
		}
		if coin.Asset.IsBase() {
			continue
		}
		f := fund{
			asset:         coin.Asset,
			stolenAsset:   cosmos.ZeroUint(),
			subsidiseRune: cosmos.ZeroUint(),
		}

		var pool Pool
		pool, err = mgr.Keeper().GetPool(ctx, coin.Asset)
		if err != nil {
			return err
		}
		f.stolenAsset = f.stolenAsset.Add(coin.Amount)
		runeValue := pool.AssetValueInRune(coin.Amount)
		if runeValue.IsZero() {
			ctx.Logger().Info("rune value of stolen asset is 0", "pool", pool.Asset, "asset amount", coin.Amount.String())
			continue
		}
		f.subsidiseRune = f.subsidiseRune.Add(runeValue)
		subsidize = append(subsidize, f)
	}

	// Check the balance of Reserve to see if  we just withdraw or withdraw and subsidize
	reserveBalance := mgr.Keeper().GetRuneBalanceOfModule(ctx, ReserveName)
	subsidizeReserveMultiplier := uint64(mgr.Keeper().GetConfigInt64(ctx, constants.SubsidizeReserveMultiplier))
	if reserveBalance.GT(totalBaseStolen.MulUint64(subsidizeReserveMultiplier)) {
		var pool Pool
		for _, f := range subsidize {
			pool, err = mgr.Keeper().GetPool(ctx, f.asset)
			if err != nil {
				ctx.Logger().Error("fail to get pool", "asset", f.asset, "error", err)
				continue
			}
			if pool.IsEmpty() {
				continue
			}

			pool.BalanceCacao = pool.BalanceCacao.Add(f.subsidiseRune)
			pool.BalanceAsset = common.SafeSub(pool.BalanceAsset, f.stolenAsset)

			if err = mgr.Keeper().SetPool(ctx, pool); err != nil {
				ctx.Logger().Error("fail to save pool", "asset", pool.Asset, "error", err)
				continue
			}

			// the value of the stolen assets is now on POL (reserve), so
			// we subsidize from directly from the reserve taking into account
			// that the value is stored in there
			runeToAsgard := common.NewCoin(common.BaseAsset(), f.subsidiseRune)
			if !runeToAsgard.Amount.IsZero() {
				if err = mgr.Keeper().SendFromModuleToModule(ctx, ReserveName, AsgardName, common.NewCoins(runeToAsgard)); err != nil {
					ctx.Logger().Error("fail to send subsidy from bond to asgard", "error", err)
					return err
				}
			}

			poolSlashAmt := []PoolAmt{
				{
					Asset:  pool.Asset,
					Amount: 0 - int64(f.stolenAsset.Uint64()),
				},
				{
					Asset:  common.BaseAsset(),
					Amount: int64(f.subsidiseRune.Uint64()),
				},
			}
			eventSlash := NewEventSlash(pool.Asset, poolSlashAmt)
			if err = mgr.EventMgr().EmitEvent(ctx, eventSlash); err != nil {
				ctx.Logger().Error("fail to emit slash event", "error", err)
			}
		}
	}

	handler := NewInternalHandler(mgr)

	asgardAddress, err := mgr.Keeper().GetModuleAddress(AsgardName)
	if err != nil {
		return err
	}

	nodeAccounts, err := mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return err
	}
	if len(nodeAccounts) == 0 {
		return fmt.Errorf("dev err: no active node accounts")
	}
	signer := nodeAccounts[0].NodeAddress

	// These is where nodes where slashed so we will take
	// the 1 from the 1.5X, withdraw it and send it to the pool
	liquidityPools := GetLiquidityPools(mgr.GetVersion())
	for _, asset := range liquidityPools {
		// The POL key for the ETH.ETH pool would be POL-ETH-ETH .
		key := "POL-" + asset.MimirString()
		var val int64
		val, err = mgr.Keeper().GetMimir(ctx, key)
		if err != nil {
			ctx.Logger().Error("fail to manage POL in pool", "pool", asset.String(), "error", err)
			continue
		}
		// -1 is unset default behaviour; 0 is off (paused); 1 is on; 2 (elsewhere) is forced withdraw.
		switch val {
		case -1:
			continue // unset default behaviour:  pause POL movements
		case 0:
			continue // off behaviour:  pause POL movements
		case 1:
			// on behaviour:  POL is enabled
		}

		// If subsidized from a LiquidityPool in which we already had liquidity
		// we only want to withdraw the difference between the stolen and the slash (.5X out of the 1.5X slashed) amount
		// else we would just be withdrawing what we subsidized
		part := totalBaseStolen
		for _, f := range subsidize {
			if f.asset.Equals(asset) {
				part = f.subsidiseRune.QuoUint64(2)
			}
		}

		basisPts := common.GetSafeShare(part, polLiquidity, cosmos.NewUint(10_000))
		coin := common.NewCoins(common.NewCoin(common.BaseAsset(), cosmos.OneUint()))
		tx := common.NewTx(common.BlankTxID, polAddress, asgardAddress, coin, nil, "MAYA-POL-REMOVE")
		msg := NewMsgWithdrawLiquidity(
			tx,
			polAddress,
			basisPts,
			asset,
			common.BaseAsset(),
			signer,
		)
		_, err = handler(ctx, msg)
		if err != nil {
			ctx.Logger().Error("fail to withdraw pol for subsidize", "error", err)
		}
	}

	return nil
}

// getTotalYggValueInRune will go through all the coins in ygg , and calculate the total value in RUNE
// return value will be totalValueInRune,error
func getTotalYggValueInRune(ctx cosmos.Context, keeper keeper.Keeper, ygg Vault) (cosmos.Uint, error) {
	yggRune := cosmos.ZeroUint()
	for _, coin := range ygg.Coins {
		if coin.Asset.IsBase() {
			yggRune = yggRune.Add(coin.Amount)
		} else {
			pool, err := keeper.GetPool(ctx, coin.Asset)
			if err != nil {
				return cosmos.ZeroUint(), err
			}
			yggRune = yggRune.Add(pool.AssetValueInRune(coin.Amount))
		}
	}
	return yggRune, nil
}

func refundBond(
	ctx cosmos.Context,
	tx common.Tx,
	acc cosmos.AccAddress,
	asset common.Asset,
	units cosmos.Uint,
	nodeAcc *NodeAccount,
	mgr Manager,
) error {
	version := mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.109.0")):
		return refundBondV109(ctx, tx, acc, asset, units, nodeAcc, mgr)
	case version.GTE(semver.MustParse("1.108.0")):
		return refundBondV108(ctx, tx, acc, asset, units, nodeAcc, mgr)
	case version.GTE(semver.MustParse("1.107.0")):
		return refundBondV107(ctx, tx, acc, asset, units, nodeAcc, mgr)
	case version.GTE(semver.MustParse("1.105.0")):
		return refundBondV105(ctx, tx, acc, asset, units, nodeAcc, mgr)
	case version.GTE(semver.MustParse("1.92.0")):
		return refundBondV92(ctx, tx, acc, nodeAcc, mgr)
	default:
		return errBadVersion
	}
}

func refundBondV109(
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
	penaltyPts := mgr.Keeper().GetConfigInt64(ctx, constants.SlashPenalty)
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
		var lps types.LiquidityProviders
		lps, err = mgr.Keeper().GetLiquidityProviderByAssets(ctx, assets, common.Address(acc.String()))
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
			withdrawnBondInCacao, err = calcLiquidityInCacao(ctx, mgr, lp.Asset, withdrawUnits)
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
	if err = mgr.Keeper().SetNodeAccount(ctx, *nodeAcc); err != nil {
		ctx.Logger().Error(fmt.Sprintf("fail to save node account(%s)", nodeAcc), "error", err)
		return err
	}
	if err = mgr.Keeper().SetBondProviders(ctx, bp); err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to save bond providers(%s)", bp.NodeAddress.String()))
	}

	if err = subsidizePoolsWithSlashBond(ctx, ygg.Coins, ygg, yggRune, slashedAmount, mgr); err != nil {
		ctx.Logger().Error("fail to subsidize pools with slash bond", "error", err)
	}
	// at this point , all coins in yggdrasil vault has been accounted for , and node already been slashed
	ygg.SubFunds(ygg.Coins)
	if err = mgr.Keeper().SetVault(ctx, ygg); err != nil {
		ctx.Logger().Error("fail to save yggdrasil vault", "error", err)
		return err
	}

	if err = mgr.Keeper().DeleteVault(ctx, ygg.PubKey); err != nil {
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

func payBondProviderReward(ctx cosmos.Context, mgr Manager, provider BondProvider, bp BondProviders) error {
	bpReward := provider.Reward
	coin := common.NewCoin(common.BaseNative, *bpReward)
	sdkErr := mgr.Keeper().SendFromModuleToAccount(ctx, BondName, provider.BondAddress, common.NewCoins(coin))
	if sdkErr != nil {
		return errors.New(sdkErr.Error())
	}

	bp.RemoveRewards(provider.BondAddress)
	err := mgr.Keeper().SetBondProviders(ctx, bp)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to save bond providers(%s)", bp.NodeAddress.String()))
	}

	var toAddress common.Address
	toAddress, err = common.NewAddress(provider.BondAddress.String(), mgr.GetVersion())
	if err != nil {
		return fmt.Errorf("fail to parse bond address: %w", err)
	}

	var fromAddress common.Address
	fromAddress, err = mgr.Keeper().GetModuleAddress(BondName)
	if err != nil {
		return fmt.Errorf("fail to get bond module address: %w", err)
	}

	// emit BondReturned event
	fakeTx := common.Tx{}
	fakeTx.ID = common.BlankTxID
	fakeTx.FromAddress = fromAddress
	fakeTx.ToAddress = toAddress
	bondRewardPaidEvent := NewEventBond(*bpReward, BondRewardPaid, fakeTx)
	if err := mgr.EventMgr().EmitEvent(ctx, bondRewardPaidEvent); err != nil {
		ctx.Logger().Error("fail to emit bond event", "error", err)
	}

	return nil
}

// isSignedByActiveNodeAccounts check if all signers are active validator nodes
func isSignedByActiveNodeAccounts(ctx cosmos.Context, k keeper.Keeper, signers []cosmos.AccAddress) bool {
	if len(signers) == 0 {
		return false
	}
	for _, signer := range signers {
		if signer.Equals(k.GetModuleAccAddress(AsgardName)) {
			continue
		}
		nodeAccount, err := k.GetNodeAccount(ctx, signer)
		if err != nil {
			ctx.Logger().Error("unauthorized account", "address", signer.String(), "error", err)
			return false
		}
		if nodeAccount.IsEmpty() {
			ctx.Logger().Error("unauthorized account", "address", signer.String())
			return false
		}
		if nodeAccount.Status != NodeActive {
			ctx.Logger().Error("unauthorized account, node account not active",
				"address", signer.String(),
				"status", nodeAccount.Status)
			return false
		}
		if nodeAccount.Type != NodeTypeValidator {
			ctx.Logger().Error("unauthorized account, node account must be a validator",
				"address", signer.String(),
				"type", nodeAccount.Type)
			return false
		}
	}
	return true
}

// TODO remove after hard fork
func fetchConfigInt64(ctx cosmos.Context, mgr Manager, key constants.ConstantName) int64 {
	val, err := mgr.Keeper().GetMimir(ctx, key.String())
	if val < 0 || err != nil {
		val = mgr.GetConstants().GetInt64Value(key)
		if err != nil {
			ctx.Logger().Error("fail to fetch mimir value", "key", key.String(), "error", err)
		}
	}
	return val
}

// polPoolValue - calculates how much the POL is worth in rune
func polPoolValue(ctx cosmos.Context, mgr Manager) (cosmos.Uint, error) {
	total := cosmos.ZeroUint()

	polAddress, err := mgr.Keeper().GetModuleAddress(ReserveName)
	if err != nil {
		return total, err
	}

	var pools Pools
	pools, err = mgr.Keeper().GetPools(ctx)
	if err != nil {
		return total, err
	}
	for _, pool := range pools {
		if pool.Asset.IsNative() {
			continue
		}
		if pool.BalanceCacao.IsZero() {
			continue
		}
		synthSupply := mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
		pool.CalcUnits(mgr.GetVersion(), synthSupply)
		var lp LiquidityProvider
		lp, err = mgr.Keeper().GetLiquidityProvider(ctx, pool.Asset, polAddress)
		if err != nil {
			return total, err
		}
		share := common.GetSafeShare(lp.Units, pool.GetPoolUnits(), pool.BalanceCacao)
		total = total.Add(share.MulUint64(2))
	}

	return total, nil
}

func wrapError(ctx cosmos.Context, err error, wrap string) error {
	err = fmt.Errorf("%s: %w", wrap, err)
	ctx.Logger().Error(err.Error())
	return multierror.Append(errInternal, err)
}

func addGasFees(ctx cosmos.Context, mgr Manager, tx ObservedTx) error {
	version := mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.108.0")):
		return addGasFeesV108(ctx, mgr, tx)
	default:
		return addGasFeesV1(ctx, mgr, tx)
	}
}

// addGasFees to vault
func addGasFeesV108(ctx cosmos.Context, mgr Manager, tx ObservedTx) error {
	if len(tx.Tx.Gas) == 0 {
		return nil
	}
	gasFee := tx.Tx.Gas
	// if tx is from THORChain get fee from chain, otherwise use from Tx Gas
	if tx.Tx.Chain.Equals(common.THORChain) {
		gasFeeAmt := mgr.GasMgr().GetGasRate(ctx, tx.Tx.Chain)
		gasFeeCoin := common.NewCoin(tx.Tx.Chain.GetGasAsset(), gasFeeAmt)
		gasFee = common.Gas{gasFeeCoin}
	}
	if mgr.Keeper().RagnarokInProgress(ctx) {
		// when ragnarok is in progress, if the tx is for gas coin then doesn't subsidise the pool with reserve
		// liquidity providers they need to pay their own gas
		// if the outbound coin is not gas asset, then reserve will subsidise it , otherwise the gas asset pool will be in a loss
		gasAsset := tx.Tx.Chain.GetGasAsset()
		if tx.Tx.Coins.GetCoin(gasAsset).IsEmpty() {
			mgr.GasMgr().AddGasAsset(gasFee, true)
		}
	} else {
		mgr.GasMgr().AddGasAsset(gasFee, true)
	}
	// Subtract from the vault
	if mgr.Keeper().VaultExists(ctx, tx.ObservedPubKey) {
		vault, err := mgr.Keeper().GetVault(ctx, tx.ObservedPubKey)
		if err != nil {
			return err
		}

		vault.SubFunds(gasFee.ToCoins())

		if err = mgr.Keeper().SetVault(ctx, vault); err != nil {
			return err
		}
	}
	return nil
}

func emitPoolBalanceChangedEvent(ctx cosmos.Context, poolMod PoolMod, reason string, mgr Manager) {
	evt := NewEventPoolBalanceChanged(poolMod, reason)
	if err := mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
		ctx.Logger().Error("fail to emit pool balance changed event", "error", err)
	}
}

// isLiquidityAuction checks for the LiquidityAuction mimir attribute
func isLiquidityAuction(ctx cosmos.Context, keeper keeper.Keeper) bool {
	liquidityAuction, err := keeper.GetMimir(ctx, constants.LiquidityAuction.String())
	if liquidityAuction < 0 || err != nil {
		return false
	}

	if liquidityAuction > 0 && ctx.BlockHeight() <= liquidityAuction {
		return true
	}

	return false
}

// isWithinWithdrawDaysLimit checks for the WithdrawDaysTierX mimir attribute or constant depending on tier
func isWithinWithdrawDaysLimit(ctx cosmos.Context, mgr Manager, cv constants.ConstantValues, addr common.Address) bool {
	version := mgr.GetVersion()
	if version.GTE(semver.MustParse("1.118.0")) {
		return isWithinWithdrawDaysLimitV118(ctx, mgr, cv, addr)
	}
	return isWithinWithdrawDaysLimitV102(ctx, mgr, cv, addr)
}

func isWithinWithdrawDaysLimitV118(ctx cosmos.Context, mgr Manager, cv constants.ConstantValues, addr common.Address) bool {
	var withdrawDays int64
	blocksPerDay := cv.GetInt64Value(constants.BlocksPerDay)
	tier, err := mgr.Keeper().GetLiquidityAuctionTier(ctx, addr)
	if err != nil {
		return false
	}

	var liquidityAuction int64
	liquidityAuction, err = mgr.Keeper().GetMimir(ctx, constants.LiquidityAuction.String())
	if liquidityAuction < 1 || err != nil {
		return false
	}

	switch tier {
	case mgr.GetConstants().GetInt64Value(constants.WithdrawTier1):
		withdrawDays = mgr.Keeper().GetConfigInt64(ctx, constants.WithdrawDaysTier1)
	case mgr.GetConstants().GetInt64Value(constants.WithdrawTier2):
		withdrawDays = mgr.Keeper().GetConfigInt64(ctx, constants.WithdrawDaysTier2)
	case mgr.GetConstants().GetInt64Value(constants.WithdrawTier3):
		withdrawDays = mgr.Keeper().GetConfigInt64(ctx, constants.WithdrawDaysTier3)
	default:
		return false
	}

	return ctx.BlockHeight() > liquidityAuction && ctx.BlockHeight() <= liquidityAuction+(withdrawDays*blocksPerDay)
}

func isWithinWithdrawDaysLimitV102(ctx cosmos.Context, mgr Manager, cv constants.ConstantValues, addr common.Address) bool {
	var withdrawDays int64
	blocksPerDay := cv.GetInt64Value(constants.BlocksPerDay)
	tier, err := mgr.Keeper().GetLiquidityAuctionTier(ctx, addr)
	if err != nil {
		return false
	}

	var liquidityAuction int64
	liquidityAuction, err = mgr.Keeper().GetMimir(ctx, constants.LiquidityAuction.String())
	if liquidityAuction < 1 || err != nil {
		return false
	}
	fmt.Println(mgr.GetConstants())
	switch tier {
	case mgr.GetConstants().GetInt64Value(constants.WithdrawTier1):
		withdrawDays = fetchConfigInt64(ctx, mgr, constants.WithdrawDaysTier1)
	case mgr.GetConstants().GetInt64Value(constants.WithdrawTier2):
		withdrawDays = fetchConfigInt64(ctx, mgr, constants.WithdrawDaysTier2)
	case mgr.GetConstants().GetInt64Value(constants.WithdrawTier3):
		withdrawDays = fetchConfigInt64(ctx, mgr, constants.WithdrawDaysTier3)
	default:
		return false
	}

	return ctx.BlockHeight() > liquidityAuction && ctx.BlockHeight() <= liquidityAuction+(withdrawDays*blocksPerDay)
}

// getWithdrawLimit returns the WithdrawLimitTierX mimir attribute or constant depending on tier
func getWithdrawLimit(ctx cosmos.Context, mgr Manager, cv constants.ConstantValues, addr common.Address) (int64, error) {
	version := mgr.GetVersion()
	if version.GTE(semver.MustParse("1.118.0")) {
		return getWithdrawLimitV118(ctx, mgr, cv, addr)
	}
	return getWithdrawLimitV102(ctx, mgr, cv, addr)
}

func getWithdrawLimitV118(ctx cosmos.Context, mgr Manager, cv constants.ConstantValues, addr common.Address) (int64, error) {
	var withdrawLimit int64
	tier, err := mgr.Keeper().GetLiquidityAuctionTier(ctx, addr)
	if err != nil {
		return 0, err
	}

	switch tier {
	case cv.GetInt64Value(constants.WithdrawTier1):
		withdrawLimit = mgr.Keeper().GetConfigInt64(ctx, constants.WithdrawLimitTier1)
	case cv.GetInt64Value(constants.WithdrawTier2):
		withdrawLimit = mgr.Keeper().GetConfigInt64(ctx, constants.WithdrawLimitTier2)
	case cv.GetInt64Value(constants.WithdrawTier3):
		withdrawLimit = mgr.Keeper().GetConfigInt64(ctx, constants.WithdrawLimitTier3)
	default:
		return 10000, nil
	}
	return withdrawLimit, nil
}

func getWithdrawLimitV102(ctx cosmos.Context, mgr Manager, cv constants.ConstantValues, addr common.Address) (int64, error) {
	var withdrawLimit int64
	tier, err := mgr.Keeper().GetLiquidityAuctionTier(ctx, addr)
	if err != nil {
		return 0, err
	}

	switch tier {
	case cv.GetInt64Value(constants.WithdrawTier1):
		withdrawLimit = fetchConfigInt64(ctx, mgr, constants.WithdrawLimitTier1)
	case cv.GetInt64Value(constants.WithdrawTier2):
		withdrawLimit = fetchConfigInt64(ctx, mgr, constants.WithdrawLimitTier2)
	case cv.GetInt64Value(constants.WithdrawTier3):
		withdrawLimit = fetchConfigInt64(ctx, mgr, constants.WithdrawLimitTier3)
	default:
		return 10000, nil
	}
	return withdrawLimit, nil
}

// isTradingHalt is to check the given msg against the key value store to decide it can be processed
// if trade is halt across all chain , then the message should be refund
// if trade for the target chain is halt , then the message should be refund as well
// isTradingHalt has been used in two handlers , thus put it here
func isTradingHalt(ctx cosmos.Context, msg cosmos.Msg, mgr Manager) bool {
	version := mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.123.0")):
		return isTradingHaltV123(ctx, msg, mgr)
	case version.GTE(semver.MustParse("0.65.0")):
		return isTradingHaltV65(ctx, msg, mgr)
	default:
		return false
	}
}

func getWhitelistedArbs(ctx cosmos.Context, mgr Manager) []string {
	version := mgr.GetVersion()
	if version.GTE(semver.MustParse("1.107.0")) {
		return WhitelistedArbsV107
	}

	return WhitelistedArbs
}

func isTradingHaltV123(ctx cosmos.Context, msg cosmos.Msg, mgr Manager) bool {
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
		sourceAsset := common.EmptyAsset
		if len(m.Tx.Coins) > 0 {
			sourceAsset = m.Tx.Coins[0].Asset
			source = sourceAsset.GetLayer1Asset().Chain
		}
		target := m.TargetAsset.GetLayer1Asset().Chain
		targetAsset := m.TargetAsset
		if !sourceAsset.IsEmpty() && isPoolTradingHalted(ctx, mgr, sourceAsset) {
			ctx.Logger().Info("trading is halted for source asset", "asset", sourceAsset.String())
			return true
		}
		if !targetAsset.IsEmpty() && isPoolTradingHalted(ctx, mgr, targetAsset) {
			ctx.Logger().Info("trading is halted for target asset", "asset", targetAsset.String())
			return true
		}

		return isChainTradingHalted(ctx, mgr, source) || isChainTradingHalted(ctx, mgr, target) || isGlobalTradingHalted(ctx, mgr)
	case *MsgAddLiquidity:
		if isPoolTradingHalted(ctx, mgr, m.Asset) {
			ctx.Logger().Info("trading is halted for asset", "asset", m.Asset.String())
			return true
		}
		return isChainTradingHalted(ctx, mgr, m.Asset.Chain) || isGlobalTradingHalted(ctx, mgr)
	default:
		return isGlobalTradingHalted(ctx, mgr)
	}
}

// isGlobalTradingHalted check whether trading has been halt at global level
func isGlobalTradingHalted(ctx cosmos.Context, mgr Manager) bool {
	haltTrading, err := mgr.Keeper().GetMimir(ctx, "HaltTrading")
	if err == nil && ((haltTrading > 0 && haltTrading < ctx.BlockHeight()) || mgr.Keeper().RagnarokInProgress(ctx)) {
		return true
	}
	return false
}

// isChainTradingHalted check whether trading on the given chain is halted
func isChainTradingHalted(ctx cosmos.Context, mgr Manager, chain common.Chain) bool {
	mimirKey := fmt.Sprintf("Halt%sTrading", chain)
	haltChainTrading, err := mgr.Keeper().GetMimir(ctx, mimirKey)
	if err == nil && (haltChainTrading > 0 && haltChainTrading < ctx.BlockHeight()) {
		ctx.Logger().Info("trading is halt", "chain", chain)
		return true
	}
	// further to check whether the chain is halted
	return isChainHalted(ctx, mgr, chain)
}

func isPoolTradingHalted(ctx cosmos.Context, mgr Manager, asset common.Asset) bool {
	mimirKey := fmt.Sprintf("HaltTrading-%s", asset.MimirString())
	haltPoolTrading, err := mgr.Keeper().GetMimir(ctx, mimirKey)
	if err == nil && (haltPoolTrading > 0 && haltPoolTrading < ctx.BlockHeight()) {
		ctx.Logger().Info("pool trading is halt", "asset", asset.String())
		return true
	}
	return false
}

func isChainHalted(ctx cosmos.Context, mgr Manager, chain common.Chain) bool {
	version := mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.87.0")):
		return isChainHaltedV87(ctx, mgr, chain)
	case version.GTE(semver.MustParse("0.65.0")):
		return isChainHaltedV65(ctx, mgr, chain)
	}
	return false
}

// isChainHalted check whether the given chain is halt
// chain halt is different as halt trading , when a chain is halt , there is no observation on the given chain
// outbound will not be signed and broadcast
func isChainHaltedV87(ctx cosmos.Context, mgr Manager, chain common.Chain) bool {
	haltChain, err := mgr.Keeper().GetMimir(ctx, "HaltChainGlobal")
	if err == nil && (haltChain > 0 && haltChain < ctx.BlockHeight()) {
		ctx.Logger().Info("global is halt")
		return true
	}

	haltChain, err = mgr.Keeper().GetMimir(ctx, "NodePauseChainGlobal")
	if err == nil && haltChain > ctx.BlockHeight() {
		ctx.Logger().Info("node global is halt")
		return true
	}

	haltMimirKey := fmt.Sprintf("Halt%sChain", chain)
	haltChain, err = mgr.Keeper().GetMimir(ctx, haltMimirKey)
	if err == nil && (haltChain > 0 && haltChain < ctx.BlockHeight()) {
		ctx.Logger().Info("chain is halt via admin or double-spend check", "chain", chain)
		return true
	}

	solvencyHaltMimirKey := fmt.Sprintf("SolvencyHalt%sChain", chain)
	haltChain, err = mgr.Keeper().GetMimir(ctx, solvencyHaltMimirKey)
	if err == nil && (haltChain > 0 && haltChain < ctx.BlockHeight()) {
		ctx.Logger().Info("chain is halt via solvency check", "chain", chain)
		return true
	}
	return false
}

// isChainHalted check whether the given chain is halt
// chain halt is different as halt trading , when a chain is halt , there is no observation on the given chain
// outbound will not be signed and broadcast
func isChainHaltedV65(ctx cosmos.Context, mgr Manager, chain common.Chain) bool {
	haltChain, err := mgr.Keeper().GetMimir(ctx, "HaltChainGlobal")
	if err == nil && (haltChain > 0 && haltChain < ctx.BlockHeight()) {
		ctx.Logger().Info("global is halt")
		return true
	}

	haltChain, err = mgr.Keeper().GetMimir(ctx, "NodePauseChainGlobal")
	if err == nil && haltChain > ctx.BlockHeight() {
		ctx.Logger().Info("node global is halt")
		return true
	}

	mimirKey := fmt.Sprintf("Halt%sChain", chain)
	haltChain, err = mgr.Keeper().GetMimir(ctx, mimirKey)
	if err == nil && (haltChain > 0 && haltChain < ctx.BlockHeight()) {
		ctx.Logger().Info("chain is halt", "chain", chain)
		return true
	}
	return false
}

// isSynthMintPaused fails validation if synth supply is already too high, relative to pool depth
func isSynthMintPaused(ctx cosmos.Context, mgr Manager, targetAsset common.Asset, outputAmt cosmos.Uint) error {
	version := mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.108.0")):
		return isSynthMintPaused108(ctx, mgr, targetAsset, outputAmt)
	default:
		return nil
	}
}

func isSynthMintPaused108(ctx cosmos.Context, mgr Manager, targetAsset common.Asset, outputAmt cosmos.Uint) error {
	maxSynths, err := mgr.Keeper().GetMimir(ctx, constants.MaxSynthPerPoolDepth.String())
	if maxSynths < 0 || err != nil {
		maxSynths = mgr.GetConstants().GetInt64Value(constants.MaxSynthPerPoolDepth)
	}

	synthSupply := mgr.Keeper().GetTotalSupply(ctx, targetAsset.GetSyntheticAsset())
	var pool Pool
	pool, err = mgr.Keeper().GetPool(ctx, targetAsset.GetLayer1Asset())
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}

	if pool.BalanceAsset.IsZero() {
		return fmt.Errorf("pool(%s) has zero asset balance", pool.Asset.String())
	}

	synthSupplyAfterSwap := synthSupply.Add(outputAmt)
	coverage := int64(synthSupplyAfterSwap.MulUint64(MaxWithdrawBasisPoints).Quo(pool.BalanceAsset.MulUint64(2)).Uint64())
	if coverage > maxSynths {
		return fmt.Errorf("synth quantity is too high relative to asset depth of related pool (%d/%d)", coverage, maxSynths)
	}

	return nil
}

func isLPPaused(ctx cosmos.Context, chain common.Chain, mgr Manager) bool {
	version := mgr.GetVersion()
	if version.GTE(semver.MustParse("0.1.0")) {
		return isLPPausedV1(ctx, chain, mgr)
	}
	return false
}

func isLPPausedV1(ctx cosmos.Context, chain common.Chain, mgr Manager) bool {
	// check if global LP is paused
	pauseLPGlobal, err := mgr.Keeper().GetMimir(ctx, "PauseLP")
	if err == nil && pauseLPGlobal > 0 && pauseLPGlobal < ctx.BlockHeight() {
		return true
	}

	var pauseLP int64
	pauseLP, err = mgr.Keeper().GetMimir(ctx, fmt.Sprintf("PauseLP%s", chain))
	if err == nil && pauseLP > 0 && pauseLP < ctx.BlockHeight() {
		ctx.Logger().Info("chain has paused LP actions", "chain", chain)
		return true
	}
	return false
}

// DollarInRune gets the amount of rune that is equal to 1 USD
func DollarInRune(ctx cosmos.Context, mgr Manager) cosmos.Uint {
	// check for mimir override
	dollarInRune, err := mgr.Keeper().GetMimir(ctx, "DollarInRune")
	if err == nil && dollarInRune > 0 {
		return cosmos.NewUint(uint64(dollarInRune))
	}

	busd, _ := common.NewAsset("BNB.BUSD-BD1")
	usdc, _ := common.NewAsset("ETH.USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48")
	usdt, _ := common.NewAsset("ETH.USDT-0XDAC17F958D2EE523A2206206994597C13D831EC7")
	usdAssets := common.Assets{busd, usdc, usdt}

	usd := make([]cosmos.Uint, 0)
	for _, asset := range usdAssets {
		if isGlobalTradingHalted(ctx, mgr) || isChainTradingHalted(ctx, mgr, asset.Chain) {
			continue
		}
		var pool Pool
		pool, err = mgr.Keeper().GetPool(ctx, asset)
		if err != nil {
			ctx.Logger().Error("fail to get usd pool", "asset", asset.String(), "error", err)
			continue
		}
		if pool.Status != PoolAvailable {
			continue
		}
		value := pool.AssetValueInRune(cosmos.NewUint(common.One))
		if !value.IsZero() {
			usd = append(usd, value)
		}
	}

	if len(usd) == 0 {
		return cosmos.ZeroUint()
	}

	sort.SliceStable(usd, func(i, j int) bool {
		return usd[i].Uint64() < usd[j].Uint64()
	})

	// calculate median of our USD figures
	var median cosmos.Uint
	if len(usd)%2 > 0 {
		// odd number of figures in our slice. Take the middle figure. Since
		// slices start with an index of zero, just need to length divide by two.
		medianSpot := len(usd) / 2
		median = usd[medianSpot]
	} else {
		// even number of figures in our slice. Average the middle two figures.
		pt1 := usd[len(usd)/2-1]
		pt2 := usd[len(usd)/2]
		median = pt1.Add(pt2).QuoUint64(2)
	}
	return median
}

func telem(input cosmos.Uint) float32 {
	if !input.BigInt().IsUint64() {
		return 0
	}
	i := input.Uint64()
	return float32(i) / 100000000
}

func telemInt(input cosmos.Int) float32 {
	if !input.BigInt().IsInt64() {
		return 0
	}
	i := input.Int64()
	return float32(i) / 100000000
}

func emitEndBlockTelemetry(ctx cosmos.Context, mgr Manager) error {
	// capture panics
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("panic while emitting end block telemetry", "error", err)
		}
	}()

	// emit network data
	network, err := mgr.Keeper().GetNetwork(ctx)
	if err != nil {
		return err
	}

	telemetry.SetGauge(telem(network.BondRewardRune), "mayanode", "network", "bond_reward_rune")
	telemetry.SetGauge(float32(network.TotalBondUnits.Uint64()), "mayanode", "network", "total_bond_units")

	// emit protocol owned liquidity data
	var pol ProtocolOwnedLiquidity
	pol, err = mgr.Keeper().GetPOL(ctx)
	if err != nil {
		return err
	}
	telemetry.SetGauge(telem(pol.CacaoDeposited), "mayanode", "pol", "cacao_deposited")
	telemetry.SetGauge(telem(pol.CacaoWithdrawn), "mayanode", "pol", "rune_withdrawn")
	telemetry.SetGauge(telemInt(pol.CurrentDeposit()), "mayanode", "pol", "current_deposit")
	polValue, err := polPoolValue(ctx, mgr)
	if err == nil {
		telemetry.SetGauge(telem(polValue), "mayanode", "pol", "value")
		telemetry.SetGauge(telemInt(pol.PnL(polValue)), "mayanode", "pol", "pnl")
	}

	// emit module balances
	for _, name := range []string{ReserveName, AsgardName, BondName} {
		modAddr := mgr.Keeper().GetModuleAccAddress(name)
		bal := mgr.Keeper().GetBalance(ctx, modAddr)
		for _, coin := range bal {
			modLabel := telemetry.NewLabel("module", name)
			denom := telemetry.NewLabel("denom", coin.Denom)
			telemetry.SetGaugeWithLabels(
				[]string{"mayanode", "module", "balance"},
				telem(cosmos.NewUint(coin.Amount.Uint64())),
				[]metrics.Label{modLabel, denom},
			)
		}
	}

	// emit node metrics
	yggs := make(Vaults, 0)
	nodes, err := mgr.Keeper().ListValidatorsWithBond(ctx)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if node.Status == NodeActive {
			var ygg Vault
			ygg, err = mgr.Keeper().GetVault(ctx, node.PubKeySet.Secp256k1)
			if err != nil {
				continue
			}
			yggs = append(yggs, ygg)
		}
		var nodeBond cosmos.Uint
		nodeBond, err = mgr.Keeper().CalcNodeLiquidityBond(ctx, node)
		if err != nil {
			return fmt.Errorf("fail to calculate node liquidity bond: %w", err)
		}
		telemetry.SetGaugeWithLabels(
			[]string{"mayanode", "node", "bond"},
			telem(cosmos.NewUint(nodeBond.Uint64())),
			[]metrics.Label{telemetry.NewLabel("node_address", node.NodeAddress.String()), telemetry.NewLabel("status", node.Status.String())},
		)
		var pts int64
		pts, err = mgr.Keeper().GetNodeAccountSlashPoints(ctx, node.NodeAddress)
		if err != nil {
			continue
		}
		telemetry.SetGaugeWithLabels(
			[]string{"mayanode", "node", "slash_points"},
			float32(pts),
			[]metrics.Label{telemetry.NewLabel("node_address", node.NodeAddress.String())},
		)

		age := cosmos.NewUint(uint64((ctx.BlockHeight() - node.StatusSince) * common.One))
		if pts > 0 {
			leaveScore := age.QuoUint64(uint64(pts))
			telemetry.SetGaugeWithLabels(
				[]string{"mayanode", "node", "leave_score"},
				float32(leaveScore.Uint64()),
				[]metrics.Label{telemetry.NewLabel("node_address", node.NodeAddress.String())},
			)
		}
	}

	// get 1 RUNE price in USD
	runeUSDPrice := 1 / telem(DollarInRune(ctx, mgr))
	telemetry.SetGauge(runeUSDPrice, "mayanode", "price", "usd", "thor", "rune")

	// emit pool metrics
	pools, err := mgr.Keeper().GetPools(ctx)
	if err != nil {
		return err
	}
	for _, pool := range pools {
		if pool.LPUnits.IsZero() {
			continue
		}
		synthSupply := mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
		labels := []metrics.Label{telemetry.NewLabel("pool", pool.Asset.String()), telemetry.NewLabel("status", pool.Status.String())}
		telemetry.SetGaugeWithLabels([]string{"mayanode", "pool", "balance", "synth"}, telem(synthSupply), labels)
		telemetry.SetGaugeWithLabels([]string{"mayanode", "pool", "balance", "rune"}, telem(pool.BalanceCacao), labels)
		telemetry.SetGaugeWithLabels([]string{"mayanode", "pool", "balance", "asset"}, telem(pool.BalanceAsset), labels)
		telemetry.SetGaugeWithLabels([]string{"mayanode", "pool", "pending", "rune"}, telem(pool.PendingInboundCacao), labels)
		telemetry.SetGaugeWithLabels([]string{"mayanode", "pool", "pending", "asset"}, telem(pool.PendingInboundAsset), labels)

		telemetry.SetGaugeWithLabels([]string{"mayanode", "pool", "units", "pool"}, telem(pool.CalcUnits(mgr.GetVersion(), synthSupply)), labels)
		telemetry.SetGaugeWithLabels([]string{"mayanode", "pool", "units", "lp"}, telem(pool.LPUnits), labels)
		telemetry.SetGaugeWithLabels([]string{"mayanode", "pool", "units", "synth"}, telem(pool.SynthUnits), labels)

		// pricing
		price := float32(0)
		if !pool.BalanceAsset.IsZero() {
			price = runeUSDPrice * telem(pool.BalanceCacao) / telem(pool.BalanceAsset)
		}
		telemetry.SetGaugeWithLabels([]string{"mayanode", "pool", "price", "usd"}, price, labels)

		// trade accounts
		var tu keeper.TradeUnit
		tu, err = mgr.Keeper().GetTradeUnit(ctx, pool.Asset.GetTradeAsset())
		if err != nil {
			ctx.Logger().Error("fail to get trade unit", "error", err)
			continue
		}
		telemetry.SetGaugeWithLabels([]string{"mayanode", "pool", "tradeasset", "units"}, telem(tu.Units), labels)
		telemetry.SetGaugeWithLabels([]string{"mayanode", "pool", "tradeasset", "depth"}, telem(tu.Depth), labels)
	}

	// emit vault metrics
	asgards, _ := mgr.Keeper().GetAsgardVaults(ctx)
	for _, vault := range append(asgards, yggs...) {
		if vault.Status != ActiveVault && vault.Status != RetiringVault {
			continue
		}

		// calculate the total value of this yggdrasil vault
		totalValue := cosmos.ZeroUint()
		for _, coin := range vault.Coins {
			if coin.Asset.IsBase() {
				totalValue = totalValue.Add(coin.Amount)
			} else {
				var pool Pool
				pool, err = mgr.Keeper().GetPool(ctx, coin.Asset.GetLayer1Asset())
				if err != nil {
					continue
				}
				totalValue = totalValue.Add(pool.AssetValueInRune(coin.Amount))
			}
		}
		labels := []metrics.Label{telemetry.NewLabel("vault_type", vault.Type.String()), telemetry.NewLabel("pubkey", vault.PubKey.String())}
		telemetry.SetGaugeWithLabels([]string{"mayanode", "vault", "total_value"}, telem(totalValue), labels)

		for _, coin := range vault.Coins {
			labels := []metrics.Label{
				telemetry.NewLabel("vault_type", vault.Type.String()),
				telemetry.NewLabel("pubkey", vault.PubKey.String()),
				telemetry.NewLabel("asset", coin.Asset.String()),
			}
			telemetry.SetGaugeWithLabels([]string{"mayanode", "vault", "balance"}, telem(coin.Amount), labels)
		}
	}

	// emit queue metrics
	signingTransactionPeriod := mgr.GetConstants().GetInt64Value(constants.SigningTransactionPeriod)
	startHeight := ctx.BlockHeight() - signingTransactionPeriod
	txOutDelayMax, err := mgr.Keeper().GetMimir(ctx, constants.TxOutDelayMax.String())
	if txOutDelayMax <= 0 || err != nil {
		txOutDelayMax = mgr.GetConstants().GetInt64Value(constants.TxOutDelayMax)
	}
	maxTxOutOffset, err := mgr.Keeper().GetMimir(ctx, constants.MaxTxOutOffset.String())
	if maxTxOutOffset <= 0 || err != nil {
		maxTxOutOffset = mgr.GetConstants().GetInt64Value(constants.MaxTxOutOffset)
	}
	var queueSwap, queueInternal, queueOutbound int64
	queueScheduledOutboundValue := cosmos.ZeroUint()
	iterator := mgr.Keeper().GetSwapQueueIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var msg MsgSwap
		if err := mgr.Keeper().Cdc().Unmarshal(iterator.Value(), &msg); err != nil {
			continue
		}
		queueSwap++
	}
	for height := startHeight; height <= ctx.BlockHeight(); height++ {
		txs, err := mgr.Keeper().GetTxOut(ctx, height)
		if err != nil {
			continue
		}
		for _, tx := range txs.TxArray {
			if tx.OutHash.IsEmpty() {
				memo, _ := ParseMemo(mgr.GetVersion(), tx.Memo)
				if memo.IsInternal() {
					queueInternal++
				} else if memo.IsOutbound() {
					queueOutbound++
				}
			}
		}
	}
	for height := ctx.BlockHeight() + 1; height <= ctx.BlockHeight()+txOutDelayMax; height++ {
		value, err := mgr.Keeper().GetTxOutValue(ctx, height)
		if err != nil {
			ctx.Logger().Error("fail to get tx out array from key value store", "error", err)
			continue
		}
		if height > ctx.BlockHeight()+maxTxOutOffset && value.IsZero() {
			// we've hit our max offset, and an empty block, we can assume the
			// rest will be empty as well
			break
		}
		queueScheduledOutboundValue = queueScheduledOutboundValue.Add(value)
	}
	telemetry.SetGauge(float32(queueInternal), "mayanode", "queue", "internal")
	telemetry.SetGauge(float32(queueOutbound), "mayanode", "queue", "outbound")
	telemetry.SetGauge(float32(queueSwap), "mayanode", "queue", "swap")
	telemetry.SetGauge(telem(queueScheduledOutboundValue), "mayanode", "queue", "scheduled", "value", "rune")
	telemetry.SetGauge(telem(queueScheduledOutboundValue)*runeUSDPrice, "mayanode", "queue", "scheduled", "value", "usd")

	return nil
}

// get the total bond of a set of NodeAccounts
func getNodeAccountsBond(ctx cosmos.Context, mgr Manager, nas NodeAccounts) []cosmos.Uint {
	var naBonds []cosmos.Uint
	for _, na := range nas {
		naBond, err := mgr.Keeper().CalcNodeLiquidityBond(ctx, na)
		if err != nil {
			ctx.Logger().Error("getHardBondCap: fail to get node bond: %w", err)
			return naBonds
		}

		if !naBond.IsZero() {
			naBonds = append(naBonds, naBond)
		}
	}
	return naBonds
}

// get the total bond of the bottom 2/3rds active nodes
func getEffectiveSecurityBond(ctx cosmos.Context, mgr Manager, nas NodeAccounts) cosmos.Uint {
	amt := cosmos.ZeroUint()

	naBonds := getNodeAccountsBond(ctx, mgr, nas)
	if len(naBonds) == 0 {
		return cosmos.ZeroUint()
	}

	sort.SliceStable(naBonds, func(i, j int) bool {
		return naBonds[i].LT(naBonds[j])
	})
	t := len(naBonds) * 2 / 3
	if len(naBonds)%3 == 0 {
		t -= 1
	}
	for i, naBond := range naBonds {
		if i <= t {
			amt = amt.Add(naBond)
		}
	}
	return amt
}

// find the bond size the highest of the bottom 2/3rds node bonds
func getHardBondCap(ctx cosmos.Context, mgr Manager, nas NodeAccounts) cosmos.Uint {
	if len(nas) == 0 {
		return cosmos.ZeroUint()
	}

	naBonds := getNodeAccountsBond(ctx, mgr, nas)
	if len(naBonds) == 0 {
		return cosmos.ZeroUint()
	}

	sort.SliceStable(naBonds, func(i, j int) bool {
		return naBonds[i].LT(naBonds[j])
	})
	i := len(naBonds) * 2 / 3
	if len(naBonds)%3 == 0 {
		i -= 1
	}
	return naBonds[i]
}

// In the case where the max gas of the chain of a queued outbound tx has changed
// Update the ObservedTxVoter so the network can still match the outbound with
// the observed inbound
func updateTxOutGas(ctx cosmos.Context, keeper keeper.Keeper, txOut types.TxOutItem, gas common.Gas) error {
	version := keeper.GetLowestActiveVersion(ctx)
	switch {
	case version.GTE(semver.MustParse("1.88.0")):
		return updateTxOutGasV88(ctx, keeper, txOut, gas)
	case version.GTE(semver.MustParse("0.1.0")):
		return updateTxOutGasV1(ctx, keeper, txOut, gas)
	default:
		return fmt.Errorf("updateTxOutGas: invalid version")
	}
}

func updateTxOutGasV88(ctx cosmos.Context, keeper keeper.Keeper, txOut types.TxOutItem, gas common.Gas) error {
	// When txOut.InHash is 0000000000000000000000000000000000000000000000000000000000000000 , which means the outbound is trigger by the network internally
	// For example , migration , yggdrasil funding etc. there is no related inbound observation , thus doesn't need to try to find it and update anything
	if txOut.InHash == common.BlankTxID {
		return nil
	}
	voter, err := keeper.GetObservedTxInVoter(ctx, txOut.InHash)
	if err != nil {
		return err
	}

	txOutIndex := -1
	for i, tx := range voter.Actions {
		if tx.Equals(txOut) {
			txOutIndex = i
			voter.Actions[txOutIndex].MaxGas = gas
			keeper.SetObservedTxInVoter(ctx, voter)
			break
		}
	}

	if txOutIndex == -1 {
		return fmt.Errorf("fail to find tx out in ObservedTxVoter %s", txOut.InHash)
	}

	return nil
}

// No-op
func updateTxOutGasV1(ctx cosmos.Context, keeper keeper.Keeper, txOut types.TxOutItem, gas common.Gas) error {
	return nil
}

// In the case where the gas rate of the chain of a queued outbound tx has changed
// Update the ObservedTxVoter so the network can still match the outbound with
// the observed inbound
func updateTxOutGasRate(ctx cosmos.Context, keeper keeper.Keeper, txOut types.TxOutItem, gasRate int64) error {
	// When txOut.InHash is 0000000000000000000000000000000000000000000000000000000000000000 , which means the outbound is trigger by the network internally
	// For example , migration , yggdrasil funding etc. there is no related inbound observation , thus doesn't need to try to find it and update anything
	if txOut.InHash == common.BlankTxID {
		return nil
	}
	voter, err := keeper.GetObservedTxInVoter(ctx, txOut.InHash)
	if err != nil {
		return err
	}

	txOutIndex := -1
	for i, tx := range voter.Actions {
		if tx.Equals(txOut) {
			txOutIndex = i
			voter.Actions[txOutIndex].GasRate = gasRate
			keeper.SetObservedTxInVoter(ctx, voter)
			break
		}
	}

	if txOutIndex == -1 {
		return fmt.Errorf("fail to find tx out in ObservedTxVoter %s", txOut.InHash)
	}

	return nil
}

func IsPeriodLastBlock(ctx cosmos.Context, blocksPerPeriod uint64) bool {
	return (uint64)(ctx.BlockHeight())%blocksPerPeriod == 0
}

// Calculate Maya Fund -->  gasFee = 90%, Maya Fund = 10%
func CalculateMayaFundPercentage(gas common.Coin, mgr Manager) (common.Coin, common.Coin) {
	mayaFundPerc := mgr.GetConstants().GetInt64Value(constants.MayaFundPerc)
	reservePerc := 100 - mayaFundPerc

	mayaGasAmt := gas.Amount.MulUint64(uint64(mayaFundPerc)).Quo(cosmos.NewUint(100))
	gas.Amount = gas.Amount.MulUint64(uint64(reservePerc)).Quo(cosmos.NewUint(100))
	mayaGas := common.NewCoin(gas.Asset, mayaGasAmt)

	return gas, mayaGas
}

func removeBondAddress(ctx cosmos.Context, mgr Manager, address common.Address) error {
	liquidityPools := GetLiquidityPools(mgr.GetVersion())
	liquidityProviders, err := mgr.Keeper().GetLiquidityProviderByAssets(ctx, liquidityPools, address)
	if err != nil {
		return ErrInternal(err, "fail to get lps in whitelisted pools")
	}

	// trunk-ignore(golangci-lint/staticcheck)
	liquidityProviders.SetNodeAccount(nil)
	mgr.Keeper().SetLiquidityProviders(ctx, liquidityProviders)

	return nil
}

func calcLiquidityInCacao(ctx cosmos.Context, mgr Manager, asset common.Asset, units cosmos.Uint) (cosmos.Uint, error) {
	pool, err := mgr.Keeper().GetPool(ctx, asset)
	if err != nil {
		return cosmos.ZeroUint(), err
	}

	if pool.LPUnits.LT(units) {
		return cosmos.ZeroUint(), fmt.Errorf("pool doesn't have enough LP units")
	}

	liquidity := common.GetSafeShare(units, pool.LPUnits, pool.BalanceCacao)
	liquidity = liquidity.Add(pool.AssetValueInRune(common.GetSafeShare(units, pool.LPUnits, pool.BalanceAsset)))
	return liquidity, nil
}

func getSlipFeeAddedBasisPoints(ctx cosmos.Context, mgr Manager) uint64 {
	slipFeeAddedBasisPoints := mgr.Keeper().GetConfigInt64(ctx, constants.SlipFeeAddedBasisPoints)
	if slipFeeAddedBasisPoints < 0 || slipFeeAddedBasisPoints > 50 {
		return 0
	}
	return uint64(slipFeeAddedBasisPoints)
}

func IsModuleAccAddress(keeper keeper.Keeper, accAddr cosmos.AccAddress) bool {
	version := keeper.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.118.0")):
		return isModuleAccAddressV118(keeper, accAddr)
	case version.GTE(semver.MustParse("1.112.0")):
		return isModuleAccAddressV112(keeper, accAddr)
	case version.GTE(semver.MustParse("1.110.0")):
		return isModuleAccAddressV110(keeper, accAddr)
	default:
		return false
	}
}

func isModuleAccAddressV118(keeper keeper.Keeper, accAddr cosmos.AccAddress) bool {
	return accAddr.Equals(keeper.GetModuleAccAddress(AsgardName)) ||
		accAddr.Equals(keeper.GetModuleAccAddress(BondName)) ||
		accAddr.Equals(keeper.GetModuleAccAddress(ReserveName)) ||
		accAddr.Equals(keeper.GetModuleAccAddress(ModuleName)) ||
		accAddr.Equals(keeper.GetModuleAccAddress(MayaFund)) ||
		accAddr.Equals(keeper.GetModuleAccAddress(CACAOPoolName)) ||
		accAddr.Equals(keeper.GetModuleAccAddress(AffiliateCollectorName))
}

func NewSimpleSwapMemo(ctx cosmos.Context, mgr Manager, targetAsset common.Asset, destination common.Address, limit cosmos.Uint, affiliate string) string {
	if affiliate == "" {
		return fmt.Sprintf("=:%s:%s:%s", targetAsset, destination, limit.String())
	} else {
		return fmt.Sprintf("=:%s:%s:%s:%s", targetAsset, destination, limit.String(), affiliate)
	}
}

// willSwapOutputExceedLimitAndFees returns true if the swap output will exceed the
// limit (if provided) + the outbound fee on the destination chain
func willSwapOutputExceedLimitAndFees(ctx cosmos.Context, mgr Manager, msg MsgSwap) (willSucceed bool, reason string) {
	swapper, err := GetSwapper(mgr.GetVersion())
	if err != nil {
		panic(err)
	}

	source := msg.Tx.Coins[0]
	target := common.NewCoin(msg.TargetAsset, msg.TradeTarget)

	var emit cosmos.Uint
	switch {
	case !source.Asset.IsNativeBase() && !target.Asset.IsNativeBase():
		sourcePool, err := mgr.Keeper().GetPool(ctx, source.Asset.GetLayer1Asset())
		if err != nil {
			return false, err.Error()
		}
		targetPool, err := mgr.Keeper().GetPool(ctx, target.Asset.GetLayer1Asset())
		if err != nil {
			return false, err.Error()
		}
		emit = swapper.CalcAssetEmission(sourcePool.BalanceAsset, source.Amount, sourcePool.BalanceCacao)
		emit = swapper.CalcAssetEmission(targetPool.BalanceCacao, emit, targetPool.BalanceAsset)
	case source.Asset.IsNativeBase():
		pool, err := mgr.Keeper().GetPool(ctx, target.Asset.GetLayer1Asset())
		if err != nil {
			return false, err.Error()
		}
		emit = swapper.CalcAssetEmission(pool.BalanceCacao, source.Amount, pool.BalanceAsset)
	case target.Asset.IsNativeBase():
		pool, err := mgr.Keeper().GetPool(ctx, source.Asset.GetLayer1Asset())
		if err != nil {
			return false, err.Error()
		}
		emit = swapper.CalcAssetEmission(pool.BalanceAsset, source.Amount, pool.BalanceCacao)
	}

	// Check that the swap will emit more than the limit amount + outbound fee
	transactionFeeAsset := mgr.GasMgr().GetFee(ctx, msg.TargetAsset.Chain, msg.TargetAsset)
	emitExceedFee := emit.GT(target.Amount.Add(transactionFeeAsset))
	if !emitExceedFee {
		reason = fmt.Sprintf("swap output is less than transaction fee, source amount: %s, source asset: %s, target asset: %s, emit: %s, transaction fee in target asset: %s",
			source.Amount, source.Asset, target.Asset, emit, transactionFeeAsset)
	}
	return emitExceedFee, reason
}

func getMaxSwapQuantity(ctx cosmos.Context, mgr Manager, sourceAsset, targetAsset common.Asset, swp StreamingSwap) (uint64, error) {
	version := mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.110.0")):
		return getMaxSwapQuantityV110(ctx, mgr, sourceAsset, targetAsset, swp)
	default:
		return 0, errBadVersion
	}
}

func getMaxSwapQuantityV110(ctx cosmos.Context, mgr Manager, sourceAsset, targetAsset common.Asset, swp StreamingSwap) (uint64, error) {
	if swp.Interval == 0 {
		return 0, nil
	}
	// collect pools involved in this swap
	var pools Pools
	totalCacaoDepth := cosmos.ZeroUint()
	for _, asset := range []common.Asset{sourceAsset, targetAsset} {
		if asset.IsNativeBase() {
			continue
		}

		pool, err := mgr.Keeper().GetPool(ctx, asset.GetLayer1Asset())
		if err != nil {
			ctx.Logger().Error("fail to fetch pool", "error", err)
			return 0, err
		}
		pools = append(pools, pool)
		totalCacaoDepth = totalCacaoDepth.Add(pool.BalanceCacao)
	}
	if len(pools) == 0 {
		return 0, fmt.Errorf("dev error: no pools selected during a streaming swap")
	}
	var virtualDepth cosmos.Uint
	switch len(pools) {
	case 1:
		// single swap, virtual depth is the same size as the single pool
		virtualDepth = totalCacaoDepth
	case 2:
		// double swap, dynamically calculate a virtual pool that is between the
		// depth of pool1 and pool2. This calculation should result in a
		// consistent swap fee (in bps) no matter the depth of the pools. The
		// larger the difference between the pools, the more the virtual pool
		// skews towards the smaller pool. This results in less rewards given
		// to the larger pool, and more rewards given to the smaller pool.

		// (2*r1*r2) / (r1+r2)
		r1 := pools[0].BalanceCacao
		r2 := pools[1].BalanceCacao
		num := r1.Mul(r2).MulUint64(2)
		denom := r1.Add(r2)
		if denom.IsZero() {
			return 0, fmt.Errorf("dev error: both pools have no rune balance")
		}
		virtualDepth = num.Quo(denom)
	default:
		return 0, fmt.Errorf("dev error: unsupported number of pools in a streaming swap: %d", len(pools))
	}
	if !sourceAsset.IsNativeBase() {
		// since the inbound asset is not rune, the virtual depth needs to be
		// recalculated to be the asset side
		virtualDepth = common.GetUncappedShare(virtualDepth, pools[0].BalanceCacao, pools[0].BalanceAsset)
	}
	// we multiply by 100 to ensure we can support decimal points (ie 5bps / 2 / 2 == 1.25)
	minBP := mgr.Keeper().GetConfigInt64(ctx, constants.StreamingSwapMinBPFee) * constants.StreamingSwapMinBPFeeMulti
	minBP /= int64(len(pools)) // since multiple swaps are executed, then minBP should be adjusted
	if minBP == 0 {
		return 0, fmt.Errorf("streaming swaps are not allowed with a min BP of zero")
	}
	// constants.StreamingSwapMinBPFee is in 10k basis point x 10, so we add an
	// addition zero here (_0)
	minSize := common.GetSafeShare(cosmos.SafeUintFromInt64(minBP), cosmos.SafeUintFromInt64(10_000*constants.StreamingSwapMinBPFeeMulti), virtualDepth)
	if minSize.IsZero() {
		return 1, nil
	}
	maxSwapQuantity := swp.Deposit.Quo(minSize)

	// make sure maxSwapQuantity doesn't infringe on max length that a
	// streaming swap can exist
	var maxLength int64
	if sourceAsset.IsNative() && targetAsset.IsNative() {
		maxLength = mgr.Keeper().GetConfigInt64(ctx, constants.StreamingSwapMaxLengthNative)
	} else {
		maxLength = mgr.Keeper().GetConfigInt64(ctx, constants.StreamingSwapMaxLength)
	}
	if swp.Interval == 0 {
		return 1, nil
	}
	maxSwapInMaxLength := uint64(maxLength) / swp.Interval
	if maxSwapQuantity.GT(cosmos.NewUint(maxSwapInMaxLength)) {
		return maxSwapInMaxLength, nil
	}

	// sanity check that max swap quantity is not zero
	if maxSwapQuantity.IsZero() {
		return 1, nil
	}

	return maxSwapQuantity.Uint64(), nil
}

// unrefundableCoinCleanup - update the accounting for a failed outbound of toi.Coin
// native rune: send to the reserve
// native coin besides rune: burn
// non-native coin: donate to its pool
func unrefundableCoinCleanup(ctx cosmos.Context, mgr Manager, toi TxOutItem, burnReason string) {
	coin := toi.Coin

	if coin.Asset.IsTradeAsset() {
		return
	}

	sourceModuleName := toi.GetModuleName() // Ensure that non-"".

	// For context in emitted events, retrieve the original transaction that prompted the cleanup.
	// If there is no retrievable transaction, leave those fields empty.
	voter, err := mgr.Keeper().GetObservedTxInVoter(ctx, toi.InHash)
	if err != nil {
		ctx.Logger().Error("fail to get observed tx in", "error", err, "hash", toi.InHash.String())
		return
	}
	tx := voter.Tx.Tx
	// For emitted events' amounts (such as EventDonate), replace the Coins with the coin being cleaned up.
	tx.Coins = common.NewCoins(toi.Coin)

	// Select course of action according to coin type:
	// External coin, native coin which isn't CACAO, or native CACAO (not from the Reserve).
	switch {
	case !coin.Asset.IsNative():
		// If unable to refund external-chain coins, add them to their pools
		// (so they aren't left in the vaults with no reflection in the pools).
		// Failed-refund external coins have earlier been established to have existing pools with non-zero BalanceCacao.

		ctx.Logger().Error("fail to refund non-native tx, leaving coins in vault", "toi.InHash", toi.InHash, "toi.Coin", toi.Coin)
		return
	case sourceModuleName != ReserveName:
		// If unable to refund MAYA.CACAO, send it to the Reserve.
		err := mgr.Keeper().SendFromModuleToModule(ctx, sourceModuleName, ReserveName, common.NewCoins(coin))
		if err != nil {
			ctx.Logger().Error("fail to send native coin to Reserve during cleanup", "error", err)
			return
		}

		reserveContributor := NewReserveContributor(tx.FromAddress, coin.Amount)
		reserveEvent := NewEventReserve(reserveContributor, tx)
		if err := mgr.EventMgr().EmitEvent(ctx, reserveEvent); err != nil {
			ctx.Logger().Error("fail to emit reserve event", "error", err)
		}
	default:
		// If not satisfying the other conditions this coin should be a native coin in the Reserve,
		// so leave it there.
	}
}

// atTVLCap - returns bool on if we've hit the TVL hard cap. Coins passed in
// are included in the calculation
func atTVLCap(ctx cosmos.Context, coins common.Coins, mgr Manager) bool {
	version := mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.110.0")):
		return atTVLCapV110(ctx, coins, mgr)
	default:
		return false
	}
}

func atTVLCapV110(ctx cosmos.Context, coins common.Coins, mgr Manager) bool {
	vaults, err := mgr.Keeper().GetAsgardVaults(ctx)
	if err != nil {
		ctx.Logger().Error("fail to get vaults for atTVLCap", "error", err)
		return true
	}

	// coins must be copied to a new variable to avoid modifying the original
	coins = coins.Copy()
	for _, vault := range vaults {
		if vault.IsAsgard() && (vault.IsActive() || vault.IsRetiring()) {
			coins = coins.Adds_deprecated(vault.Coins)
		}
	}

	cacaoCoin := coins.GetCoin(common.BaseAsset())
	totalCacaoValue := cacaoCoin.Amount
	for _, coin := range coins {
		if coin.IsEmpty() {
			continue
		}
		asset := coin.Asset
		// while asgard vaults don't contain native assets, the `coins`
		// parameter might
		if asset.IsSyntheticAsset() {
			asset = asset.GetLayer1Asset()
		}
		var pool Pool
		pool, err = mgr.Keeper().GetPool(ctx, asset)
		if err != nil {
			ctx.Logger().Error("fail to get pool for atTVLCap", "asset", coin.Asset, "error", err)
			continue
		}
		if !pool.IsAvailable() && !pool.IsStaged() {
			continue
		}
		if pool.BalanceCacao.IsZero() || pool.BalanceAsset.IsZero() {
			continue
		}
		totalCacaoValue = totalCacaoValue.Add(pool.AssetValueInRune(coin.Amount))
	}

	// get effectiveSecurity
	nodeAccounts, err := mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		ctx.Logger().Error("fail to get validators to calculate TVL cap", "error", err)
		return true
	}
	effectiveSecurity := getEffectiveSecurityBond(ctx, mgr, nodeAccounts)

	if totalCacaoValue.GT(effectiveSecurity) {
		ctx.Logger().Debug("reached TVL cap", "total cacao value", totalCacaoValue.String(), "effective security", effectiveSecurity.String())
		return true
	}
	return false
}

func getPreferredAssetSwapThreshold(ctx cosmos.Context, mgr Manager, preferredAsset common.Asset) cosmos.Uint {
	version := mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.118.0")):
		return getPreferredAssetSwapThresholdV118(ctx, mgr, preferredAsset)
	default:
		return getPreferredAssetSwapThresholdV1(ctx, mgr, preferredAsset)
	}
}

func getPreferredAssetSwapThresholdV118(ctx cosmos.Context, mgr Manager, preferredAsset common.Asset) cosmos.Uint {
	// trigger the preferred asset swap if accrued CACAO is greater than the value of
	// PreferredAssetOutboundFeeMultiplier (default = 100) x current outbound fee of preferred asset chain
	if preferredAsset.IsEmpty() || preferredAsset.IsNativeBase() {
		return cosmos.ZeroUint()
	}
	ofCacao := mgr.GasMgr().GetFee(ctx, preferredAsset.Chain, common.BaseNative)
	multiplier := mgr.Keeper().GetConfigInt64(ctx, constants.PreferredAssetOutboundFeeMultiplier)
	threshold := ofCacao.Mul(cosmos.NewUint(uint64(multiplier)))
	ctx.Logger().Debug("getPreferredAssetSwapThreshold", "preferredAsset", preferredAsset, "ofCacao", ofCacao, "multiplier", multiplier, "threshold", threshold)
	return threshold
}

// addSwapDirect adds the swap directly to the swap queue (no order book) - segmented
// out into its own function to allow easier maintenance of original behavior vs order
// book behavior.
func addSwapDirect(ctx cosmos.Context, mgr Manager, msg MsgSwap) {
	version := mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.118.0")):
		addSwapDirectV118(ctx, mgr, msg)
	default:
		addSwapDirectV112(ctx, mgr, msg)
	}
}

func addSwapDirectV118(ctx cosmos.Context, mgr Manager, msg MsgSwap) {
	if msg.Tx.Coins.IsEmpty() {
		return
	}
	// Queue the main swap
	if err := mgr.Keeper().SetSwapQueueItem(ctx, msg, 0); err != nil {
		ctx.Logger().Error("fail to add swap to queue", "error", err)
	}
}

////////////////////////////////////////////////////////////////////////////////////////
// CACAOPool and POL
////////////////////////////////////////////////////////////////////////////////////////

// cacaoPoolValue is the CACAO in the cacaopool module.
func cacaoPoolValue(ctx cosmos.Context, mgr Manager) (cosmos.Uint, error) {
	return mgr.Keeper().GetRuneBalanceOfModule(ctx, CACAOPoolName), nil
}
