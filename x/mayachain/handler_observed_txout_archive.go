package mayachain

import (
	"context"
	"strings"

	"github.com/armon/go-metrics"
	"github.com/cosmos/cosmos-sdk/telemetry"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

func (h ObservedTxOutHandler) handleV112(ctx cosmos.Context, msg MsgObservedTxOut) (*cosmos.Result, error) {
	activeNodeAccounts, err := h.mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return nil, wrapError(ctx, err, "fail to get list of active node accounts")
	}

	handler := NewInternalHandler(h.mgr)

	for _, tx := range msg.Txs {
		// check we are sending from a valid vault
		if !h.mgr.Keeper().VaultExists(ctx, tx.ObservedPubKey) {
			ctx.Logger().Info("Not valid Observed Pubkey", tx.ObservedPubKey)
			continue
		}
		if tx.KeysignMs > 0 {
			keysignMetric, err := h.mgr.Keeper().GetTssKeysignMetric(ctx, tx.Tx.ID)
			if err != nil {
				ctx.Logger().Error("fail to get keysing metric", "error", err)
			} else {
				keysignMetric.AddNodeTssTime(msg.Signer, tx.KeysignMs)
				h.mgr.Keeper().SetTssKeysignMetric(ctx, keysignMetric)
			}
		}
		voter, err := h.mgr.Keeper().GetObservedTxOutVoter(ctx, tx.Tx.ID)
		if err != nil {
			ctx.Logger().Error("fail to get tx out voter", "error", err)
			continue
		}

		// check whether the tx has consensus
		voter, ok := h.preflight(ctx, voter, activeNodeAccounts, tx, msg.Signer)
		if !ok {
			if voter.FinalisedHeight == ctx.BlockHeight() {
				// we've already process the transaction, but we should still
				// update the observing addresses
				h.mgr.ObMgr().AppendObserver(tx.Tx.Chain, msg.GetSigners())
			}
			continue
		}
		ctx.Logger().Info("handleMsgObservedTxOut request", "Tx:", tx.String())

		// if memo isn't valid or its an inbound memo, and its funds moving
		// from a yggdrasil vault, slash the node
		memo, _ := ParseMemoWithMAYANames(ctx, h.mgr.Keeper(), tx.Tx.Memo)
		if memo.IsEmpty() || memo.IsInbound() {
			var vault Vault
			vault, err = h.mgr.Keeper().GetVault(ctx, tx.ObservedPubKey)
			if err != nil {
				ctx.Logger().Error("fail to get vault", "error", err)
				continue
			}
			toSlash := make(common.Coins, len(tx.Tx.Coins))
			copy(toSlash, tx.Tx.Coins)
			toSlash = toSlash.Adds_deprecated(tx.Tx.Gas.ToCoins())

			slashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{
				telemetry.NewLabel("reason", "sent_extra_funds"),
				telemetry.NewLabel("chain", string(tx.Tx.Chain)),
			}))

			if err = h.mgr.Slasher().SlashVaultToLP(slashCtx, tx.ObservedPubKey, toSlash, h.mgr, true); err != nil {
				ctx.Logger().Error("fail to slash account for sending extra fund", "error", err)
			}
			vault.SubFunds(toSlash)
			if err = h.mgr.Keeper().SetVault(ctx, vault); err != nil {
				ctx.Logger().Error("fail to save vault", "error", err)
			}

			continue
		}

		txOut := voter.GetTx(activeNodeAccounts) // get consensus tx, in case our for loop is incorrect
		txOut.Tx.Memo = tx.Tx.Memo
		m, err := processOneTxIn(ctx, h.mgr.GetVersion(), h.mgr.Keeper(), txOut, msg.Signer)
		if err != nil || tx.Tx.Chain.IsEmpty() {
			ctx.Logger().Error("fail to process txOut",
				"error", err,
				"tx", tx.Tx.String())
			continue
		}

		vault, err := h.mgr.Keeper().GetVault(ctx, tx.ObservedPubKey)
		if err != nil {
			ctx.Logger().Error("fail to get vault", "error", err)
			continue
		}
		// Apply Gas fees
		if vault.Status != InactiveVault {
			if err = addGasFees(ctx, h.mgr, tx); err != nil {
				ctx.Logger().Error("fail to add gas fee", "error", err)
				continue
			}
		}

		// add addresses to observing addresses. This is used to detect
		// active/inactive observing node accounts
		h.mgr.ObMgr().AppendObserver(tx.Tx.Chain, txOut.GetSigners())

		// emit tss keysign metrics
		if tx.KeysignMs > 0 {
			var keysignMetric *types.TssKeysignMetric
			keysignMetric, err = h.mgr.Keeper().GetTssKeysignMetric(ctx, tx.Tx.ID)
			if err != nil {
				ctx.Logger().Error("fail to get tss keysign metric", "error", err, "hash", tx.Tx.ID)
			} else {
				evt := NewEventTssKeysignMetric(keysignMetric.TxID, keysignMetric.GetMedianTime())
				if err = h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
					ctx.Logger().Error("fail to emit tss metric event", "error", err)
				}
			}
		}
		_, err = handler(ctx, m)
		if err != nil {
			ctx.Logger().Error("handler failed:", "error", err)
			continue
		}
		voter.SetDone()
		h.mgr.Keeper().SetObservedTxOutVoter(ctx, voter)
		// process the msg first , and then deduct the fund from vault last
		// If sending from one of our vaults, decrement coins
		vault, err = h.mgr.Keeper().GetVault(ctx, tx.ObservedPubKey)
		if err != nil {
			ctx.Logger().Error("fail to get vault", "error", err)
			continue
		}
		vault.SubFunds(tx.Tx.Coins)
		vault.OutboundTxCount++
		if vault.IsAsgard() && memo.IsType(TxMigrate) {
			// only remove the block height that had been specified in the memo
			vault.RemovePendingTxBlockHeights(memo.GetBlockHeight())
		}

		if !vault.HasFunds() && vault.Status == RetiringVault {
			// we have successfully removed all funds from a retiring vault,
			// mark it as inactive
			vault.Status = InactiveVault
		}
		// if the vault is frozen, then unfreeze it. Since we saw that a
		// transaction was signed
		for _, coin := range tx.Tx.Coins {
			for i := range vault.Frozen {
				if strings.EqualFold(coin.Asset.GetChain().String(), vault.Frozen[i]) {
					vault.Frozen = append(vault.Frozen[:i], vault.Frozen[i+1:]...)
					break
				}
			}
		}
		if err := h.mgr.Keeper().SetVault(ctx, vault); err != nil {
			ctx.Logger().Error("fail to save vault", "error", err)
			continue
		}
	}
	return &cosmos.Result{}, nil
}

func (h ObservedTxOutHandler) handleV104(ctx cosmos.Context, msg MsgObservedTxOut) (*cosmos.Result, error) {
	activeNodeAccounts, err := h.mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return nil, wrapError(ctx, err, "fail to get list of active node accounts")
	}

	handler := NewInternalHandler(h.mgr)

	for _, tx := range msg.Txs {
		// check we are sending from a valid vault
		if !h.mgr.Keeper().VaultExists(ctx, tx.ObservedPubKey) {
			ctx.Logger().Info("Not valid Observed Pubkey", tx.ObservedPubKey)
			continue
		}
		if tx.KeysignMs > 0 {
			keysignMetric, err := h.mgr.Keeper().GetTssKeysignMetric(ctx, tx.Tx.ID)
			if err != nil {
				ctx.Logger().Error("fail to get keysing metric", "error", err)
			} else {
				keysignMetric.AddNodeTssTime(msg.Signer, tx.KeysignMs)
				h.mgr.Keeper().SetTssKeysignMetric(ctx, keysignMetric)
			}
		}
		voter, err := h.mgr.Keeper().GetObservedTxOutVoter(ctx, tx.Tx.ID)
		if err != nil {
			ctx.Logger().Error("fail to get tx out voter", "error", err)
			continue
		}

		// check whether the tx has consensus
		voter, ok := h.preflight(ctx, voter, activeNodeAccounts, tx, msg.Signer)
		if !ok {
			if voter.FinalisedHeight == ctx.BlockHeight() {
				// we've already process the transaction, but we should still
				// update the observing addresses
				h.mgr.ObMgr().AppendObserver(tx.Tx.Chain, msg.GetSigners())
			}
			continue
		}
		ctx.Logger().Info("handleMsgObservedTxOut request", "Tx:", tx.String())

		// if memo isn't valid or its an inbound memo, and its funds moving
		// from a yggdrasil vault, slash the node
		memo, _ := ParseMemoWithMAYANames(ctx, h.mgr.Keeper(), tx.Tx.Memo)
		if memo.IsEmpty() || memo.IsInbound() {
			var vault Vault
			vault, err = h.mgr.Keeper().GetVault(ctx, tx.ObservedPubKey)
			if err != nil {
				ctx.Logger().Error("fail to get vault", "error", err)
				continue
			}
			toSlash := make(common.Coins, len(tx.Tx.Coins))
			copy(toSlash, tx.Tx.Coins)
			toSlash = toSlash.Adds_deprecated(tx.Tx.Gas.ToCoins())

			slashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{
				telemetry.NewLabel("reason", "sent_extra_funds"),
				telemetry.NewLabel("chain", string(tx.Tx.Chain)),
			}))

			if err = h.mgr.Slasher().SlashVaultToLP(slashCtx, tx.ObservedPubKey, toSlash, h.mgr, true); err != nil {
				ctx.Logger().Error("fail to slash account for sending extra fund", "error", err)
			}
			vault.SubFunds(toSlash)
			if err = h.mgr.Keeper().SetVault(ctx, vault); err != nil {
				ctx.Logger().Error("fail to save vault", "error", err)
			}

			continue
		}

		txOut := voter.GetTx(activeNodeAccounts) // get consensus tx, in case our for loop is incorrect
		txOut.Tx.Memo = tx.Tx.Memo
		m, err := processOneTxIn(ctx, h.mgr.GetVersion(), h.mgr.Keeper(), txOut, msg.Signer)
		if err != nil || tx.Tx.Chain.IsEmpty() {
			ctx.Logger().Error("fail to process txOut",
				"error", err,
				"tx", tx.Tx.String())
			continue
		}

		// Apply Gas fees
		if err = addGasFees(ctx, h.mgr, tx); err != nil {
			ctx.Logger().Error("fail to add gas fee", "error", err)
			continue
		}

		// add addresses to observing addresses. This is used to detect
		// active/inactive observing node accounts
		h.mgr.ObMgr().AppendObserver(tx.Tx.Chain, txOut.GetSigners())

		// emit tss keysign metrics
		if tx.KeysignMs > 0 {
			var keysignMetric *types.TssKeysignMetric
			keysignMetric, err = h.mgr.Keeper().GetTssKeysignMetric(ctx, tx.Tx.ID)
			if err != nil {
				ctx.Logger().Error("fail to get tss keysign metric", "error", err, "hash", tx.Tx.ID)
			} else {
				evt := NewEventTssKeysignMetric(keysignMetric.TxID, keysignMetric.GetMedianTime())
				if err = h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
					ctx.Logger().Error("fail to emit tss metric event", "error", err)
				}
			}
		}
		_, err = handler(ctx, m)
		if err != nil {
			ctx.Logger().Error("handler failed:", "error", err)
			continue
		}
		voter.SetDone()
		h.mgr.Keeper().SetObservedTxOutVoter(ctx, voter)
		// process the msg first , and then deduct the fund from vault last
		// If sending from one of our vaults, decrement coins
		vault, err := h.mgr.Keeper().GetVault(ctx, tx.ObservedPubKey)
		if err != nil {
			ctx.Logger().Error("fail to get vault", "error", err)
			continue
		}
		vault.SubFunds(tx.Tx.Coins)
		vault.OutboundTxCount++
		if vault.IsAsgard() && memo.IsType(TxMigrate) {
			// only remove the block height that had been specified in the memo
			vault.RemovePendingTxBlockHeights(memo.GetBlockHeight())
		}

		if !vault.HasFunds() && vault.Status == RetiringVault {
			// we have successfully removed all funds from a retiring vault,
			// mark it as inactive
			vault.Status = InactiveVault
		}
		// if the vault is frozen, then unfreeze it. Since we saw that a
		// transaction was signed
		for _, coin := range tx.Tx.Coins {
			for i := range vault.Frozen {
				if strings.EqualFold(coin.Asset.GetChain().String(), vault.Frozen[i]) {
					vault.Frozen = append(vault.Frozen[:i], vault.Frozen[i+1:]...)
					break
				}
			}
		}
		if err := h.mgr.Keeper().SetVault(ctx, vault); err != nil {
			ctx.Logger().Error("fail to save vault", "error", err)
			continue
		}
	}
	return &cosmos.Result{}, nil
}

// Handle a message to observe outbound tx
func (h ObservedTxOutHandler) handleV96(ctx cosmos.Context, msg MsgObservedTxOut) (*cosmos.Result, error) {
	activeNodeAccounts, err := h.mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return nil, wrapError(ctx, err, "fail to get list of active node accounts")
	}

	handler := NewInternalHandler(h.mgr)

	for _, tx := range msg.Txs {
		// check we are sending from a valid vault
		if !h.mgr.Keeper().VaultExists(ctx, tx.ObservedPubKey) {
			ctx.Logger().Info("Not valid Observed Pubkey", tx.ObservedPubKey)
			continue
		}
		if tx.KeysignMs > 0 {
			keysignMetric, err := h.mgr.Keeper().GetTssKeysignMetric(ctx, tx.Tx.ID)
			if err != nil {
				ctx.Logger().Error("fail to get keysing metric", "error", err)
			} else {
				keysignMetric.AddNodeTssTime(msg.Signer, tx.KeysignMs)
				h.mgr.Keeper().SetTssKeysignMetric(ctx, keysignMetric)
			}
		}
		voter, err := h.mgr.Keeper().GetObservedTxOutVoter(ctx, tx.Tx.ID)
		if err != nil {
			ctx.Logger().Error("fail to get tx out voter", "error", err)
			continue
		}

		// check whether the tx has consensus
		voter, ok := h.preflight(ctx, voter, activeNodeAccounts, tx, msg.Signer)
		if !ok {
			if voter.FinalisedHeight == ctx.BlockHeight() {
				// we've already process the transaction, but we should still
				// update the observing addresses
				h.mgr.ObMgr().AppendObserver(tx.Tx.Chain, msg.GetSigners())
			}
			continue
		}
		ctx.Logger().Info("handleMsgObservedTxOut request", "Tx:", tx.String())

		// if memo isn't valid or its an inbound memo, and its funds moving
		// from a yggdrasil vault, slash the node
		memo, _ := ParseMemoWithMAYANames(ctx, h.mgr.Keeper(), tx.Tx.Memo)
		if memo.IsEmpty() || memo.IsInbound() {
			var vault Vault
			vault, err = h.mgr.Keeper().GetVault(ctx, tx.ObservedPubKey)
			if err != nil {
				ctx.Logger().Error("fail to get vault", "error", err)
				continue
			}
			toSlash := make(common.Coins, len(tx.Tx.Coins))
			copy(toSlash, tx.Tx.Coins)
			toSlash = toSlash.Adds_deprecated(tx.Tx.Gas.ToCoins())

			slashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{
				telemetry.NewLabel("reason", "sent_extra_funds"),
				telemetry.NewLabel("chain", string(tx.Tx.Chain)),
			}))

			if err = h.mgr.Slasher().SlashVaultToLP(slashCtx, tx.ObservedPubKey, toSlash, h.mgr, true); err != nil {
				ctx.Logger().Error("fail to slash account for sending extra fund", "error", err)
			}
			vault.SubFunds(toSlash)
			if err = h.mgr.Keeper().SetVault(ctx, vault); err != nil {
				ctx.Logger().Error("fail to save vault", "error", err)
			}

			continue
		}

		txOut := voter.GetTx(activeNodeAccounts) // get consensus tx, in case our for loop is incorrect
		txOut.Tx.Memo = tx.Tx.Memo
		var m cosmos.Msg
		m, err = processOneTxIn(ctx, h.mgr.GetVersion(), h.mgr.Keeper(), txOut, msg.Signer)
		if err != nil || tx.Tx.Chain.IsEmpty() {
			ctx.Logger().Error("fail to process txOut",
				"error", err,
				"tx", tx.Tx.String())
			continue
		}

		// Apply Gas fees
		if err = addGasFees(ctx, h.mgr, tx); err != nil {
			ctx.Logger().Error("fail to add gas fee", "error", err)
			continue
		}

		// add addresses to observing addresses. This is used to detect
		// active/inactive observing node accounts
		h.mgr.ObMgr().AppendObserver(tx.Tx.Chain, txOut.GetSigners())

		// emit tss keysign metrics
		if tx.KeysignMs > 0 {
			var keysignMetric *types.TssKeysignMetric
			keysignMetric, err = h.mgr.Keeper().GetTssKeysignMetric(ctx, tx.Tx.ID)
			if err != nil {
				ctx.Logger().Error("fail to get tss keysign metric", "error", err, "hash", tx.Tx.ID)
			} else {
				evt := NewEventTssKeysignMetric(keysignMetric.TxID, keysignMetric.GetMedianTime())
				if err = h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
					ctx.Logger().Error("fail to emit tss metric event", "error", err)
				}
			}
		}
		_, err = handler(ctx, m)
		if err != nil {
			ctx.Logger().Error("handler failed:", "error", err)
			continue
		}
		voter.SetDone()
		h.mgr.Keeper().SetObservedTxOutVoter(ctx, voter)
		// process the msg first , and then deduct the fund from vault last
		// If sending from one of our vaults, decrement coins
		var vault Vault
		vault, err = h.mgr.Keeper().GetVault(ctx, tx.ObservedPubKey)
		if err != nil {
			ctx.Logger().Error("fail to get vault", "error", err)
			continue
		}
		vault.SubFunds(tx.Tx.Coins)
		vault.OutboundTxCount++
		if vault.IsAsgard() && memo.IsType(TxMigrate) {
			// only remove the block height that had been specified in the memo
			vault.RemovePendingTxBlockHeights(memo.GetBlockHeight())
		}

		if !vault.HasFunds() && vault.Status == RetiringVault {
			// we have successfully removed all funds from a retiring vault,
			// mark it as inactive
			vault.Status = InactiveVault
		}
		if err = h.mgr.Keeper().SetVault(ctx, vault); err != nil {
			ctx.Logger().Error("fail to save vault", "error", err)
			continue
		}
	}
	return &cosmos.Result{}, nil
}
