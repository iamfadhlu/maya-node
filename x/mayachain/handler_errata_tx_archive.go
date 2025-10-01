package mayachain

import (
	"fmt"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

// processErrataOutboundTx when the network detect an outbound tx which previously had been sent out to customer , however it get re-org , and it doesn't
// exist on the external chain anymore , then it will need to reschedule the tx
func (h ErrataTxHandler) processErrataOutboundTxV65(ctx cosmos.Context, msg MsgErrataTx) (*cosmos.Result, error) {
	txOutVoter, err := h.mgr.Keeper().GetObservedTxOutVoter(ctx, msg.GetTxID())
	if err != nil {
		return nil, fmt.Errorf("fail to get observed tx out voter for tx (%s) : %w", msg.GetTxID(), err)
	}
	if len(txOutVoter.Txs) == 0 {
		return nil, fmt.Errorf("cannot find tx: %s", msg.TxID)
	}
	if txOutVoter.Tx.IsEmpty() {
		return nil, fmt.Errorf("tx out voter is not finalised")
	}
	tx := txOutVoter.Tx.Tx
	if !tx.Chain.Equals(msg.Chain) || tx.Coins.IsEmpty() {
		return &cosmos.Result{}, nil
	}
	// parse the outbound tx memo, so we can figure out which inbound tx triggered the outbound
	m, err := ParseMemoWithMAYANames(ctx, h.mgr.Keeper(), tx.Memo)
	if err != nil {
		return nil, fmt.Errorf("fail to parse memo(%s): %w", tx.Memo, err)
	}
	if !m.IsOutbound() && !m.IsInternal() {
		return nil, fmt.Errorf("%s is not outbound or internal tx", m)
	}
	vaultPubKey := txOutVoter.Tx.ObservedPubKey
	if !vaultPubKey.IsEmpty() {
		var v Vault
		v, err = h.mgr.Keeper().GetVault(ctx, vaultPubKey)
		if err != nil {
			return nil, fmt.Errorf("fail to get vault with pubkey %s: %w", vaultPubKey, err)
		}
		compensate := true
		if v.IsAsgard() {
			// if the fund is sending out from asgard , then it need to be credit back to asgard
			// if the asgard has been retired (inactive), need to set it to Retiring again , so the fund can be migrated
			v.AddFunds(tx.Coins)
			compensate = false
			if v.Status == InactiveVault {
				ctx.Logger().Info("Errata cause retired vault to be resurrect", "vault pub key", v.PubKey)
				v.UpdateStatus(RetiringVault, ctx.BlockHeight())
			}
		}

		if v.IsYggdrasil() {
			var node NodeAccount
			node, err = h.mgr.Keeper().GetNodeAccountByPubKey(ctx, v.PubKey)
			if err != nil {
				return nil, fmt.Errorf("fail to get node account with pubkey: %s,err: %w", v.PubKey, err)
			}

			var nodeBond cosmos.Uint
			nodeBond, err = h.mgr.Keeper().CalcNodeLiquidityBond(ctx, node)
			if err != nil {
				return nil, fmt.Errorf("fail to calculate node liquidity with pubkey: %s,err: %w", v.PubKey, err)
			}

			if !node.IsEmpty() && !nodeBond.IsZero() {
				// as long as the node still has bond , we can just credit it back to it's yggdrasil vault.
				// if the node request to leave , but has not refund it's bond yet , then they will be slashed,
				// if the node stay in the network , then they can still hold the fund until they leave
				// if the node already left , but only has little bond left , the slash logic will take it all , and then
				// subsidise pool with reserve
				v.AddFunds(tx.Coins)
				compensate = false
			}
		}

		if !v.IsEmpty() {
			if err = h.mgr.Keeper().SetVault(ctx, v); err != nil {
				return nil, fmt.Errorf("fail to save vault: %w", err)
			}
		}
		if compensate {
			for _, coin := range tx.Coins {
				if coin.Asset.IsBase() {
					// it is using native rune, so outbound can't be RUNE
					continue
				}
				var p Pool
				p, err = h.mgr.Keeper().GetPool(ctx, coin.Asset)
				if err != nil {
					return nil, fmt.Errorf("fail to get pool(%s): %w", coin.Asset, err)
				}
				runeValue := p.AssetValueInRune(coin.Amount)
				p.BalanceCacao = p.BalanceCacao.Add(runeValue)
				p.BalanceAsset = common.SafeSub(p.BalanceAsset, coin.Amount)
				if err = h.mgr.Keeper().SendFromModuleToModule(ctx, ReserveName, AsgardName, common.Coins{
					common.NewCoin(common.BaseAsset(), runeValue),
				}); err != nil {
					return nil, fmt.Errorf("fail to send fund from reserve to asgard: %w", err)
				}
				if err = h.mgr.Keeper().SetPool(ctx, p); err != nil {
					return nil, fmt.Errorf("fail to save pool (%s) : %w", p.Asset, err)
				}
				// send errata event
				mods := PoolMods{
					NewPoolMod(p.Asset, runeValue, true, coin.Amount, false),
				}

				eventErrata := NewEventErrata(msg.TxID, mods)
				if err = h.mgr.EventMgr().EmitEvent(ctx, eventErrata); err != nil {
					return nil, ErrInternal(err, "fail to emit errata event")
				}
			}
		}
	}

	if !m.IsInternal() && !m.GetTxID().IsEmpty() && !m.GetTxID().Equals(common.BlankTxID) {
		var txInVoter ObservedTxVoter
		txInVoter, err = h.mgr.Keeper().GetObservedTxInVoter(ctx, m.GetTxID())
		if err != nil {
			return nil, fmt.Errorf("fail to get tx in voter for tx (%s): %w", m.GetTxID(), err)
		}

		for _, item := range txInVoter.Actions {
			if !item.OutHash.Equals(msg.GetTxID()) {
				continue
			}
			newTxOutItem := TxOutItem{
				Chain:     item.Chain,
				InHash:    item.InHash,
				ToAddress: item.ToAddress,
				Coin:      item.Coin,
				Memo:      item.Memo,
			}
			_, err = h.mgr.TxOutStore().TryAddTxOutItem(ctx, h.mgr, newTxOutItem, cosmos.ZeroUint())
			if err != nil {
				return nil, fmt.Errorf("fail to reschedule tx out item: %w", err)
			}
			break
		}
	}
	txOutVoter.SetReverted()
	h.mgr.Keeper().SetObservedTxOutVoter(ctx, txOutVoter)
	return &cosmos.Result{}, nil
}
