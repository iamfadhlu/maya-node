package mayachain

import (
	"strings"

	"github.com/blang/semver"
	"gitlab.com/mayachain/mayanode/common/tokenlist"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
)

func externalAssetMatchV95(version semver.Version, chain common.Chain, hint string) string {
	if len(hint) == 0 {
		return hint
	}
	if chain.IsEVM() {
		// find all potential matches
		matches := []string{}
		for _, token := range tokenlist.GetEVMTokenList(chain, version).Tokens {
			if strings.HasSuffix(strings.ToLower(token.Address), strings.ToLower(hint)) {
				matches = append(matches, token.Address)
				if len(matches) > 1 {
					break
				}
			}
		}
		// if we only have one match, lets go with it, otherwise leave the
		// user's input alone. It may still work, if it doesn't, should get the
		// gas asset instead of the erc20 desired.
		if len(matches) == 1 {
			return matches[0]
		}

		return hint
	}
	return hint
}

func externalAssetMatchV93(version semver.Version, chain common.Chain, hint string) string {
	if len(hint) == 0 {
		return hint
	}
	switch chain {
	case common.ETHChain:
		// find all potential matches
		matches := []string{}
		for _, token := range tokenlist.GetETHTokenList(version).Tokens {
			if strings.HasSuffix(strings.ToLower(token.Address), strings.ToLower(hint)) {
				matches = append(matches, token.Address)
				if len(matches) > 1 {
					break
				}
			}
		}

		// if we only have one match, lets go with it, otherwise leave the
		// user's input alone. It may still work, if it doesn't, should get the
		// gas asset instead of the erc20 desired.
		if len(matches) == 1 {
			return matches[0]
		}

		return hint
	default:
		return hint
	}
}

func fuzzyAssetMatchV1(ctx cosmos.Context, keeper keeper.Keeper, asset common.Asset) common.Asset {
	// if its already an exact match, return it immediately
	if keeper.PoolExist(ctx, asset.GetLayer1Asset()) {
		return asset
	}

	matches := make(Pools, 0)

	iterator := keeper.GetPoolIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var pool Pool
		if err := keeper.Cdc().Unmarshal(iterator.Value(), &pool); err != nil {
			ctx.Logger().Error("fail to fetch pool", "asset", asset, "err", err)
			continue
		}

		// check chain match
		if !asset.Chain.Equals(pool.Asset.Chain) {
			continue
		}

		// check ticker match
		if !asset.Ticker.Equals(pool.Asset.Ticker) {
			continue
		}

		// check symbol
		parts := strings.Split(asset.Symbol.String(), "-")
		// check if no symbol given (ie "USDT" or "USDT-")
		if len(parts) < 2 || strings.EqualFold(parts[1], "") {
			matches = append(matches, pool)
			continue
		}

		if strings.HasSuffix(strings.ToLower(pool.Asset.Symbol.String()), strings.ToLower(parts[1])) {
			matches = append(matches, pool)
			continue
		}
	}

	// if we found no matches, return the argument given
	if len(matches) == 0 {
		return asset
	}

	// find the deepest pool
	winner := NewPool()
	for _, pool := range matches {
		if winner.BalanceCacao.LT(pool.BalanceCacao) {
			winner = pool
		}
	}

	return winner.Asset
}

func getInternalHandlerMappingV1(mgr Manager) map[string]MsgHandler {
	// New arch handlers
	m := make(map[string]MsgHandler)
	m[MsgOutboundTx{}.Type()] = NewOutboundTxHandler(mgr)
	m[MsgYggdrasil{}.Type()] = NewYggdrasilHandler(mgr)
	m[MsgSwap{}.Type()] = NewSwapHandler(mgr)
	m[MsgReserveContributor{}.Type()] = NewReserveContributorHandler(mgr)
	m[MsgBond{}.Type()] = NewBondHandler(mgr)
	m[MsgUnBond{}.Type()] = NewUnBondHandler(mgr)
	m[MsgLeave{}.Type()] = NewLeaveHandler(mgr)
	m[MsgDonate{}.Type()] = NewDonateHandler(mgr)
	m[MsgWithdrawLiquidity{}.Type()] = NewWithdrawLiquidityHandler(mgr)
	m[MsgAddLiquidity{}.Type()] = NewAddLiquidityHandler(mgr)
	m[MsgRefundTx{}.Type()] = NewRefundHandler(mgr)
	m[MsgMigrate{}.Type()] = NewMigrateHandler(mgr)
	m[MsgRagnarok{}.Type()] = NewRagnarokHandler(mgr)
	m[MsgNoOp{}.Type()] = NewNoOpHandler(mgr)
	m[MsgConsolidate{}.Type()] = NewConsolidateHandler(mgr)
	m[MsgManageMAYAName{}.Type()] = NewManageMAYANameHandler(mgr)
	m[MsgForgiveSlash{}.Type()] = NewForgiveSlashHandler(mgr)
	return m
}

func getInternalHandlerMappingV118(mgr Manager) map[string]MsgHandler {
	// New arch handlers
	m := make(map[string]MsgHandler)
	m[MsgOutboundTx{}.Type()] = NewOutboundTxHandler(mgr)
	m[MsgYggdrasil{}.Type()] = NewYggdrasilHandler(mgr)
	m[MsgSwap{}.Type()] = NewSwapHandler(mgr)
	m[MsgReserveContributor{}.Type()] = NewReserveContributorHandler(mgr)
	m[MsgBond{}.Type()] = NewBondHandler(mgr)
	m[MsgUnBond{}.Type()] = NewUnBondHandler(mgr)
	m[MsgLeave{}.Type()] = NewLeaveHandler(mgr)
	m[MsgDonate{}.Type()] = NewDonateHandler(mgr)
	m[MsgWithdrawLiquidity{}.Type()] = NewWithdrawLiquidityHandler(mgr)
	m[MsgAddLiquidity{}.Type()] = NewAddLiquidityHandler(mgr)
	m[MsgRefundTx{}.Type()] = NewRefundHandler(mgr)
	m[MsgMigrate{}.Type()] = NewMigrateHandler(mgr)
	m[MsgRagnarok{}.Type()] = NewRagnarokHandler(mgr)
	m[MsgNoOp{}.Type()] = NewNoOpHandler(mgr)
	m[MsgConsolidate{}.Type()] = NewConsolidateHandler(mgr)
	m[MsgManageMAYAName{}.Type()] = NewManageMAYANameHandler(mgr)
	m[MsgForgiveSlash{}.Type()] = NewForgiveSlashHandler(mgr)
	m[MsgCacaoPoolDeposit{}.Type()] = NewCacaoPoolDepositHandler(mgr)
	m[MsgCacaoPoolWithdraw{}.Type()] = NewCacaoPoolWithdrawHandler(mgr)
	return m
}

func processOneTxInV63(ctx cosmos.Context, keeper keeper.Keeper, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	memo, err := ParseMemoWithMAYANames(ctx, keeper, tx.Tx.Memo)
	if err != nil {
		ctx.Logger().Error("fail to parse memo", "error", err)
		return nil, err
	}
	// THORNode should not have one tx across chain, if it is cross chain it should be separate tx
	var newMsg cosmos.Msg
	// interpret the memo and initialize a corresponding msg event
	switch m := memo.(type) {
	case AddLiquidityMemo:
		m.Asset = fuzzyAssetMatch(ctx, keeper, m.Asset)
		newMsg, err = getMsgAddLiquidityFromMemo(ctx, m, tx, signer, m.Tier)
	case WithdrawLiquidityMemo:
		m.Asset = fuzzyAssetMatch(ctx, keeper, m.Asset)
		newMsg, err = getMsgWithdrawFromMemo(m, tx, signer, keeper.GetVersion())
	case SwapMemo:
		m.Asset = fuzzyAssetMatch(ctx, keeper, m.Asset)
		m.DexTargetAddress = externalAssetMatch(keeper.GetVersion(), m.Asset.GetChain(), m.DexTargetAddress)
		newMsg, err = getMsgSwapFromMemo(m, tx, signer)
	case DonateMemo:
		m.Asset = fuzzyAssetMatch(ctx, keeper, m.Asset)
		newMsg, err = getMsgDonateFromMemo(m, tx, signer)
	case RefundMemo:
		newMsg, err = getMsgRefundFromMemo(m, tx, signer)
	case OutboundMemo:
		newMsg, err = getMsgOutboundFromMemo(m, tx, signer)
	case MigrateMemo:
		newMsg, err = getMsgMigrateFromMemo(m, tx, signer)
	case BondMemo:
		newMsg, err = getMsgBondFromMemo(m, tx, signer)
	case UnbondMemo:
		newMsg, err = getMsgUnbondFromMemo(m, tx, signer)
	case RagnarokMemo:
		newMsg, err = getMsgRagnarokFromMemo(m, tx, signer)
	case LeaveMemo:
		newMsg, err = getMsgLeaveFromMemo(m, tx, signer)
	case YggdrasilFundMemo:
		newMsg = NewMsgYggdrasil(tx.Tx, tx.ObservedPubKey, m.GetBlockHeight(), true, tx.Tx.Coins, signer)
	case YggdrasilReturnMemo:
		newMsg = NewMsgYggdrasil(tx.Tx, tx.ObservedPubKey, m.GetBlockHeight(), false, tx.Tx.Coins, signer)
	case ReserveMemo:
		res := NewReserveContributor(tx.Tx.FromAddress, tx.Tx.Coins.GetCoin(common.BaseAsset()).Amount)
		newMsg = NewMsgReserveContributor(tx.Tx, res, signer)
	case NoOpMemo:
		newMsg = NewMsgNoOp(tx, signer, m.Action)
	case ConsolidateMemo:
		newMsg = NewMsgConsolidate(tx, signer)
	case ManageMAYANameMemo:
		newMsg, err = getMsgManageMAYANameFromMemo(m, tx, signer)
	case ForgiveSlashMemo:
		newMsg, err = getMsgForgiveSlashFromMemo(m, tx, signer)
	default:
		return nil, errInvalidMemo
	}

	if err != nil {
		return newMsg, err
	}
	// MsgAddLiquidity, MsgSwap, MsgSetAztecAddress & MsgManageMAYAName has a new version of validateBasic
	version := keeper.GetVersion()
	switch m := newMsg.(type) {
	case *MsgAddLiquidity:
		switch {
		case version.GTE(semver.MustParse("1.108.0")):
			return newMsg, m.ValidateBasicV108()
		case version.GTE(semver.MustParse("0.63.0")):
			return newMsg, m.ValidateBasicV63()
		default:
			return newMsg, m.ValidateBasic()
		}
	case *MsgSwap:
		switch {
		case version.GTE(semver.MustParse("1.112.0")):
			return newMsg, m.ValidateBasicV112(version)
		case version.GTE(semver.MustParse("0.63.0")):
			return newMsg, m.ValidateBasicV63(version)
		default:
			return newMsg, m.ValidateBasic()
		}
	case *MsgSetAztecAddress:
		switch {
		case version.GTE(semver.MustParse("1.108.0")):
			return newMsg, m.ValidateBasicV108(version)
		default:
			return newMsg, m.ValidateBasic()
		}
	case *MsgManageMAYAName:
		switch {
		case version.GTE(semver.MustParse("1.112.0")):
			return newMsg, m.ValidateBasicV112(version)
		case version.GTE(semver.MustParse("1.108.0")):
			return newMsg, m.ValidateBasicV108(version)
		default:
			return newMsg, m.ValidateBasic()
		}
	}
	return newMsg, newMsg.ValidateBasic()
}

func processOneTxInV118(ctx cosmos.Context, keeper keeper.Keeper, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	memo, err := ParseMemoWithMAYANames(ctx, keeper, tx.Tx.Memo)
	if err != nil {
		ctx.Logger().Error("fail to parse memo", "error", err)
		return nil, err
	}
	// THORNode should not have one tx across chain, if it is cross chain it should be separate tx
	var newMsg cosmos.Msg
	// interpret the memo and initialize a corresponding msg event
	switch m := memo.(type) {
	case AddLiquidityMemo:
		m.Asset = fuzzyAssetMatch(ctx, keeper, m.Asset)
		newMsg, err = getMsgAddLiquidityFromMemo(ctx, m, tx, signer, m.Tier)
	case WithdrawLiquidityMemo:
		m.Asset = fuzzyAssetMatch(ctx, keeper, m.Asset)
		newMsg, err = getMsgWithdrawFromMemo(m, tx, signer, keeper.GetVersion())
	case SwapMemo:
		m.Asset = fuzzyAssetMatch(ctx, keeper, m.Asset)
		m.DexTargetAddress = externalAssetMatch(keeper.GetVersion(), m.Asset.GetChain(), m.DexTargetAddress)
		newMsg, err = getMsgSwapFromMemo(m, tx, signer)
	case DonateMemo:
		m.Asset = fuzzyAssetMatch(ctx, keeper, m.Asset)
		newMsg, err = getMsgDonateFromMemo(m, tx, signer)
	case RefundMemo:
		newMsg, err = getMsgRefundFromMemo(m, tx, signer)
	case OutboundMemo:
		newMsg, err = getMsgOutboundFromMemo(m, tx, signer)
	case MigrateMemo:
		newMsg, err = getMsgMigrateFromMemo(m, tx, signer)
	case BondMemo:
		newMsg, err = getMsgBondFromMemo(m, tx, signer)
	case UnbondMemo:
		newMsg, err = getMsgUnbondFromMemo(m, tx, signer)
	case RagnarokMemo:
		newMsg, err = getMsgRagnarokFromMemo(m, tx, signer)
	case LeaveMemo:
		newMsg, err = getMsgLeaveFromMemo(m, tx, signer)
	case YggdrasilFundMemo:
		newMsg = NewMsgYggdrasil(tx.Tx, tx.ObservedPubKey, m.GetBlockHeight(), true, tx.Tx.Coins, signer)
	case YggdrasilReturnMemo:
		newMsg = NewMsgYggdrasil(tx.Tx, tx.ObservedPubKey, m.GetBlockHeight(), false, tx.Tx.Coins, signer)
	case ReserveMemo:
		res := NewReserveContributor(tx.Tx.FromAddress, tx.Tx.Coins.GetCoin(common.BaseAsset()).Amount)
		newMsg = NewMsgReserveContributor(tx.Tx, res, signer)
	case NoOpMemo:
		newMsg = NewMsgNoOp(tx, signer, m.Action)
	case ConsolidateMemo:
		newMsg = NewMsgConsolidate(tx, signer)
	case ManageMAYANameMemo:
		newMsg, err = getMsgManageMAYANameFromMemo(m, tx, signer)
	case ForgiveSlashMemo:
		newMsg, err = getMsgForgiveSlashFromMemo(m, tx, signer)
	case CacaoPoolDepositMemo:
		newMsg = NewMsgCacaoPoolDeposit(signer, tx.Tx)
	case CacaoPoolWithdrawMemo:
		newMsg = NewMsgCacaoPoolWithdraw(signer, tx.Tx, m.GetBasisPts(), m.GetAffiliateBasisPoints())
	default:
		return nil, errInvalidMemo
	}

	if err != nil {
		return newMsg, err
	}
	// MsgAddLiquidity, MsgSwap, MsgSetAztecAddress & MsgManageMAYAName has a new version of validateBasic
	version := keeper.GetVersion()
	switch m := newMsg.(type) {
	case *MsgAddLiquidity:
		switch {
		case keeper.GetVersion().GTE(semver.MustParse("1.108.0")):
			return newMsg, m.ValidateBasicV108()
		case keeper.GetVersion().GTE(semver.MustParse("0.63.0")):
			return newMsg, m.ValidateBasicV63()
		default:
			return newMsg, m.ValidateBasic()
		}
	case *MsgSwap:
		switch {
		case keeper.GetVersion().GTE(semver.MustParse("1.112.0")):
			return newMsg, m.ValidateBasicV112(keeper.GetVersion())
		case keeper.GetVersion().GTE(semver.MustParse("0.63.0")):
			return newMsg, m.ValidateBasicV63(keeper.GetVersion())
		default:
			return newMsg, m.ValidateBasic()
		}
	case *MsgSetAztecAddress:
		switch {
		case version.GTE(semver.MustParse("1.108.0")):
			return newMsg, m.ValidateBasicV108(version)
		default:
			return newMsg, m.ValidateBasic()
		}
	case *MsgManageMAYAName:
		switch {
		case version.GTE(semver.MustParse("1.112.0")):
			return newMsg, m.ValidateBasicV112(version)
		case version.GTE(semver.MustParse("1.108.0")):
			return newMsg, m.ValidateBasicV108(version)
		default:
			return newMsg, m.ValidateBasic()
		}
	}
	return newMsg, newMsg.ValidateBasic()
}

func fuzzyAssetMatchV83(ctx cosmos.Context, keeper keeper.Keeper, origAsset common.Asset) common.Asset {
	asset := origAsset.GetLayer1Asset()
	// if its already an exact match, return it immediately
	if keeper.PoolExist(ctx, asset.GetLayer1Asset()) {
		return origAsset
	}

	matches := make(Pools, 0)

	iterator := keeper.GetPoolIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var pool Pool
		if err := keeper.Cdc().Unmarshal(iterator.Value(), &pool); err != nil {
			ctx.Logger().Error("fail to fetch pool", "asset", asset, "err", err)
			continue
		}

		// check chain match
		if !asset.Chain.Equals(pool.Asset.Chain) {
			continue
		}

		// check ticker match
		if !asset.Ticker.Equals(pool.Asset.Ticker) {
			continue
		}

		// check symbol
		parts := strings.Split(asset.Symbol.String(), "-")
		// check if no symbol given (ie "USDT" or "USDT-")
		if len(parts) < 2 || strings.EqualFold(parts[1], "") {
			matches = append(matches, pool)
			continue
		}

		if strings.HasSuffix(strings.ToLower(pool.Asset.Symbol.String()), strings.ToLower(parts[1])) {
			matches = append(matches, pool)
			continue
		}
	}

	// if we found no matches, return the argument given
	if len(matches) == 0 {
		return origAsset
	}

	// find the deepest pool
	winner := NewPool()
	for _, pool := range matches {
		if winner.BalanceCacao.LT(pool.BalanceCacao) {
			winner = pool
		}
	}

	winner.Asset.Synth = origAsset.Synth

	return winner.Asset
}
