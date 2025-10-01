package mayachain

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/armon/go-metrics"
	"github.com/cosmos/cosmos-sdk/telemetry"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

func (h TssHandler) handleV123(ctx cosmos.Context, msg MsgTssPool) (*cosmos.Result, error) {
	ctx.Logger().Info("handler tss", "current version", h.mgr.GetVersion())
	blames := make([]string, 0)
	if !msg.BlameLegacy.IsEmpty() {
		for i := range msg.BlameLegacy.BlameNodes {
			pk, err := common.NewPubKey(msg.BlameLegacy.BlameNodes[i].Pubkey)
			if err != nil {
				ctx.Logger().Error("fail to get tss keygen pubkey", "pubkey", msg.BlameLegacy.BlameNodes[i].Pubkey, "error", err)
				continue
			}
			acc, err := pk.GetThorAddress()
			if err != nil {
				ctx.Logger().Error("fail to get tss keygen thor address", "pubkey", msg.BlameLegacy.BlameNodes[i].Pubkey, "error", err)
				continue
			}
			blames = append(blames, acc.String())
		}
		sort.Strings(blames)
		ctx.Logger().Info(
			"tss keygen results blame",
			"height", msg.Height,
			"id", msg.ID,
			"pubkey", msg.PoolPubKey,
			"round", msg.BlameLegacy.Round,
			"blames", strings.Join(blames, ", "),
			"reason", msg.BlameLegacy.FailReason,
			"blamer", msg.Signer,
		)
	}
	// only record TSS metric when keygen is success
	if msg.IsSuccess() && !msg.PoolPubKey.IsEmpty() {
		metric, err := h.mgr.Keeper().GetTssKeygenMetric(ctx, msg.PoolPubKey)
		if err != nil {
			ctx.Logger().Error("fail to get keygen metric", "error", err)
		} else {
			ctx.Logger().Info("save keygen metric to db")
			metric.AddNodeTssTime(msg.Signer, msg.KeygenTime)
			h.mgr.Keeper().SetTssKeygenMetric(ctx, metric)
		}
	}
	voter, err := h.mgr.Keeper().GetTssVoter(ctx, msg.ID)
	if err != nil {
		return nil, fmt.Errorf("fail to get tss voter: %w", err)
	}

	// when PoolPubKey is empty , which means TssVoter with id(msg.ID) doesn't
	// exist before, this is the first time to create it
	// set the PoolPubKey to the one in msg, there is no reason voter.PubKeys
	// have anything in it either, thus override it with msg.PubKeys as well
	if voter.PoolPubKey.IsEmpty() {
		voter.PoolPubKey = msg.PoolPubKey
		voter.PubKeys = msg.PubKeys
	}
	// voter's pool pubkey is the same as the one in message
	if !voter.PoolPubKey.Equals(msg.PoolPubKey) {
		return nil, fmt.Errorf("invalid pool pubkey")
	}
	observeSlashPoints := h.mgr.GetConstants().GetInt64Value(constants.ObserveSlashPoints)
	observeFlex := h.mgr.GetConstants().GetInt64Value(constants.ObservationDelayFlexibility)

	slashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{
		telemetry.NewLabel("reason", "failed_observe_tss_pool"),
	}))
	h.mgr.Slasher().IncSlashPoints(slashCtx, observeSlashPoints, msg.Signer)

	if !voter.Sign(msg.Signer, msg.Chains) {
		ctx.Logger().Info("signer already signed MsgTssPool", "signer", msg.Signer.String(), "txid", msg.ID)
		return &cosmos.Result{}, nil

	}
	h.mgr.Keeper().SetTssVoter(ctx, voter)

	// doesn't have 2/3 majority consensus yet
	if !voter.HasConsensus() {
		return &cosmos.Result{}, nil
	}

	// when keygen success
	if msg.IsSuccess() {
		h.judgeLateSigner(ctx, msg, voter)
		if !voter.HasCompleteConsensus() {
			return &cosmos.Result{}, nil
		}
	}

	if voter.BlockHeight == 0 {
		voter.BlockHeight = ctx.BlockHeight()
		h.mgr.Keeper().SetTssVoter(ctx, voter)
		h.mgr.Slasher().DecSlashPoints(slashCtx, observeSlashPoints, voter.GetSigners()...)
		if msg.IsSuccess() {
			ctx.Logger().Info(
				"tss keygen results success",
				"height", msg.Height,
				"id", msg.ID,
				"pubkey", msg.PoolPubKey,
			)
			vaultType := YggdrasilVault
			if msg.KeygenType == AsgardKeygen {
				vaultType = AsgardVault
			}
			chains := voter.ConsensusChains()
			vault := NewVault(ctx.BlockHeight(), InitVault, vaultType, voter.PoolPubKey, chains.Strings(), h.mgr.Keeper().GetChainContracts(ctx, chains))
			vault.Membership = voter.PubKeys

			if err = h.mgr.Keeper().SetVault(ctx, vault); err != nil {
				return nil, fmt.Errorf("fail to save vault: %w", err)
			}
			var keygenBlock KeygenBlock
			keygenBlock, err = h.mgr.Keeper().GetKeygenBlock(ctx, msg.Height)
			if err != nil {
				return nil, fmt.Errorf("fail to get keygen block, err: %w, height: %d", err, msg.Height)
			}
			var initVaults Vaults
			initVaults, err = h.mgr.Keeper().GetAsgardVaultsByStatus(ctx, InitVault)
			if err != nil {
				return nil, fmt.Errorf("fail to get init vaults: %w", err)
			}

			var metric *types.TssKeygenMetric
			metric, err = h.mgr.Keeper().GetTssKeygenMetric(ctx, msg.PoolPubKey)
			if err != nil {
				ctx.Logger().Error("fail to get keygen metric", "error", err)
			} else {
				var total int64
				for _, item := range metric.NodeTssTimes {
					total += item.TssTime
				}
				evt := NewEventTssKeygenMetric(metric.PubKey, metric.GetMedianTime())
				if err = h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
					ctx.Logger().Error("fail to emit tss metric event", "error", err)
				}
			}

			if len(initVaults) == len(keygenBlock.Keygens) {
				ctx.Logger().Info("tss keygen results churn", "asgards", len(initVaults))
				for _, v := range initVaults {
					if err = h.mgr.NetworkMgr().RotateVault(ctx, v); err != nil {
						return nil, fmt.Errorf("fail to rotate vault: %w", err)
					}
				}
			} else {
				ctx.Logger().Info("not enough keygen yet", "expecting", len(keygenBlock.Keygens), "current", len(initVaults))
			}

			var addrs []cosmos.AccAddress
			addrs, err = vault.GetMembership().Addresses()
			members := make([]string, len(addrs))
			if err != nil {
				ctx.Logger().Error("fail to get member addresses", "error", err)
			} else {
				for i, addr := range addrs {
					members[i] = addr.String()
				}
				if err = h.mgr.EventMgr().EmitEvent(ctx, NewEventTssKeygenSuccess(msg.PoolPubKey, msg.Height, members)); err != nil {
					ctx.Logger().Error("fail to emit keygen success event")
				}
			}

		} else {
			// since the keygen failed, its now safe to reset all nodes in
			// ready status back to standby status
			var ready NodeAccounts
			ready, err = h.mgr.Keeper().ListValidatorsByStatus(ctx, NodeReady)
			if err != nil {
				ctx.Logger().Error("fail to get list of ready node accounts", "error", err)
			}
			for _, na := range ready {
				na.UpdateStatus(NodeStandby, ctx.BlockHeight())
				if err = h.mgr.Keeper().SetNodeAccount(ctx, na); err != nil {
					ctx.Logger().Error("fail to set node account", "error", err)
				}
			}

			// if a node fail to join the keygen, thus hold off the network
			// from churning then it will be slashed accordingly
			slashPoints := h.mgr.GetConstants().GetInt64Value(constants.FailKeygenSlashPoints)
			totalSlash := cosmos.ZeroUint()
			var nodePubKey common.PubKey
			for _, node := range msg.BlameLegacy.BlameNodes {
				nodePubKey, err = common.NewPubKey(node.Pubkey)
				if err != nil {
					return nil, ErrInternal(err, fmt.Sprintf("fail to parse pubkey(%s)", node.Pubkey))
				}

				var na NodeAccount
				na, err = h.mgr.Keeper().GetNodeAccountByPubKey(ctx, nodePubKey)
				if err != nil {
					return nil, fmt.Errorf("fail to get node from it's pub key: %w", err)
				}

				var naBond cosmos.Uint
				naBond, err = h.mgr.Keeper().CalcNodeLiquidityBond(ctx, na)
				if err != nil {
					return nil, fmt.Errorf("fail to calculate node liquidity bond: %w", err)
				}

				if na.Status == NodeActive {
					failedKeygenSlashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{
						telemetry.NewLabel("reason", "failed_keygen"),
					}))
					if err = h.mgr.Keeper().IncNodeAccountSlashPoints(failedKeygenSlashCtx, na.NodeAddress, slashPoints); err != nil {
						ctx.Logger().Error("fail to inc slash points", "error", err)
					}

					if err = h.mgr.EventMgr().EmitEvent(ctx, NewEventSlashPoint(na.NodeAddress, slashPoints, "fail keygen")); err != nil {
						ctx.Logger().Error("fail to emit slash point event")
					}
				} else {
					// go to jail
					jailTime := h.mgr.GetConstants().GetInt64Value(constants.JailTimeKeygen)
					releaseHeight := ctx.BlockHeight() + jailTime
					reason := "failed to perform keygen"
					if err = h.mgr.Keeper().SetNodeAccountJail(ctx, na.NodeAddress, releaseHeight, reason); err != nil {
						ctx.Logger().Error("fail to set node account jail", "node address", na.NodeAddress, "reason", reason, "error", err)
					}

					var network Network
					network, err = h.mgr.Keeper().GetNetwork(ctx)
					if err != nil {
						return nil, fmt.Errorf("fail to get network: %w", err)
					}

					slashBond := network.CalcNodeRewards(cosmos.NewUint(uint64(slashPoints)))
					if slashBond.GT(naBond) {
						slashBond = naBond
					}
					ctx.Logger().Info("fail keygen , slash bond", "address", na.NodeAddress, "amount", slashBond.String())
					totalSlash = totalSlash.Add(slashBond)

					var slashedAmount cosmos.Uint
					slashedAmount, _, err = h.mgr.Slasher().SlashNodeAccountLP(ctx, na, slashBond)
					if err != nil {
						return nil, fmt.Errorf("fail to slash node account: %w", err)
					}

					slashFloat, _ := new(big.Float).SetInt(slashedAmount.BigInt()).Float32()
					telemetry.IncrCounterWithLabels(
						[]string{"mayanode", "bond_slash"},
						slashFloat,
						[]metrics.Label{
							telemetry.NewLabel("address", na.NodeAddress.String()),
							telemetry.NewLabel("reason", "failed_keygen"),
						},
					)
				}
				if err = h.mgr.Keeper().SetNodeAccount(ctx, na); err != nil {
					return nil, fmt.Errorf("fail to save node account: %w", err)
				}

				tx := common.Tx{}
				tx.ID = common.BlankTxID
				tx.FromAddress = na.BondAddress
				if err = h.mgr.EventMgr().EmitBondEvent(ctx, h.mgr, common.BaseNative, totalSlash, BondCost, tx); err != nil {
					return nil, fmt.Errorf("fail to emit bond event: %w", err)
				}

			}

			if err := h.mgr.EventMgr().EmitEvent(ctx, NewEventTssKeygenFailure(msg.BlameLegacy.FailReason, msg.BlameLegacy.Round, msg.BlameLegacy.IsUnicast, msg.Height, blames)); err != nil {
				ctx.Logger().Error("fail to emit keygen failure event")
			}

		}
		return &cosmos.Result{}, nil
	}

	if (voter.BlockHeight + observeFlex) >= ctx.BlockHeight() {
		h.mgr.Slasher().DecSlashPoints(slashCtx, observeSlashPoints, msg.Signer)
	}

	return &cosmos.Result{}, nil
}

func (h TssHandler) handleV122(ctx cosmos.Context, msg MsgTssPool) (*cosmos.Result, error) {
	ctx.Logger().Info("handler tss", "current version", h.mgr.GetVersion())
	blames := make([]string, 0)
	if !msg.BlameLegacy.IsEmpty() {
		for i := range msg.BlameLegacy.BlameNodes {
			pk, err := common.NewPubKey(msg.BlameLegacy.BlameNodes[i].Pubkey)
			if err != nil {
				ctx.Logger().Error("fail to get tss keygen pubkey", "pubkey", msg.BlameLegacy.BlameNodes[i].Pubkey, "error", err)
				continue
			}
			acc, err := pk.GetThorAddress()
			if err != nil {
				ctx.Logger().Error("fail to get tss keygen thor address", "pubkey", msg.BlameLegacy.BlameNodes[i].Pubkey, "error", err)
				continue
			}
			blames = append(blames, acc.String())
		}
		sort.Strings(blames)
		ctx.Logger().Info(
			"tss keygen results blame",
			"height", msg.Height,
			"id", msg.ID,
			"pubkey", msg.PoolPubKey,
			"round", msg.BlameLegacy.Round,
			"blames", strings.Join(blames, ", "),
			"reason", msg.BlameLegacy.FailReason,
			"blamer", msg.Signer,
		)
	}
	// only record TSS metric when keygen is success
	if msg.IsSuccess() && !msg.PoolPubKey.IsEmpty() {
		metric, err := h.mgr.Keeper().GetTssKeygenMetric(ctx, msg.PoolPubKey)
		if err != nil {
			ctx.Logger().Error("fail to get keygen metric", "error", err)
		} else {
			ctx.Logger().Info("save keygen metric to db")
			metric.AddNodeTssTime(msg.Signer, msg.KeygenTime)
			h.mgr.Keeper().SetTssKeygenMetric(ctx, metric)
		}
	}
	voter, err := h.mgr.Keeper().GetTssVoter(ctx, msg.ID)
	if err != nil {
		return nil, fmt.Errorf("fail to get tss voter: %w", err)
	}

	// when PoolPubKey is empty , which means TssVoter with id(msg.ID) doesn't
	// exist before, this is the first time to create it
	// set the PoolPubKey to the one in msg, there is no reason voter.PubKeys
	// have anything in it either, thus override it with msg.PubKeys as well
	if voter.PoolPubKey.IsEmpty() {
		voter.PoolPubKey = msg.PoolPubKey
		voter.PubKeys = msg.PubKeys
	}
	// voter's pool pubkey is the same as the one in message
	if !voter.PoolPubKey.Equals(msg.PoolPubKey) {
		return nil, fmt.Errorf("invalid pool pubkey")
	}
	observeSlashPoints := h.mgr.GetConstants().GetInt64Value(constants.ObserveSlashPoints)
	observeFlex := h.mgr.GetConstants().GetInt64Value(constants.ObservationDelayFlexibility)

	slashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{
		telemetry.NewLabel("reason", "failed_observe_tss_pool"),
	}))
	h.mgr.Slasher().IncSlashPoints(slashCtx, observeSlashPoints, msg.Signer)

	if !voter.Sign(msg.Signer, msg.Chains) {
		ctx.Logger().Info("signer already signed MsgTssPool", "signer", msg.Signer.String(), "txid", msg.ID)
		return &cosmos.Result{}, nil

	}
	h.mgr.Keeper().SetTssVoter(ctx, voter)

	// doesn't have 2/3 majority consensus yet
	if !voter.HasConsensus() {
		return &cosmos.Result{}, nil
	}

	// when keygen success
	if msg.IsSuccess() {
		h.judgeLateSigner(ctx, msg, voter)
		if !voter.HasCompleteConsensus() {
			return &cosmos.Result{}, nil
		}
	}

	if voter.BlockHeight == 0 {
		voter.BlockHeight = ctx.BlockHeight()
		h.mgr.Keeper().SetTssVoter(ctx, voter)
		h.mgr.Slasher().DecSlashPoints(slashCtx, observeSlashPoints, voter.GetSigners()...)
		if msg.IsSuccess() {
			ctx.Logger().Info(
				"tss keygen results success",
				"height", msg.Height,
				"id", msg.ID,
				"pubkey", msg.PoolPubKey,
			)
			vaultType := YggdrasilVault
			if msg.KeygenType == AsgardKeygen {
				vaultType = AsgardVault
			}
			chains := voter.ConsensusChains()
			vault := NewVault(ctx.BlockHeight(), InitVault, vaultType, voter.PoolPubKey, chains.Strings(), h.mgr.Keeper().GetChainContracts(ctx, chains))
			vault.Membership = voter.PubKeys

			if err = h.mgr.Keeper().SetVault(ctx, vault); err != nil {
				return nil, fmt.Errorf("fail to save vault: %w", err)
			}
			var keygenBlock KeygenBlock
			keygenBlock, err = h.mgr.Keeper().GetKeygenBlock(ctx, msg.Height)
			if err != nil {
				return nil, fmt.Errorf("fail to get keygen block, err: %w, height: %d", err, msg.Height)
			}
			var initVaults Vaults
			initVaults, err = h.mgr.Keeper().GetAsgardVaultsByStatus(ctx, InitVault)
			if err != nil {
				return nil, fmt.Errorf("fail to get init vaults: %w", err)
			}

			var metric *types.TssKeygenMetric
			metric, err = h.mgr.Keeper().GetTssKeygenMetric(ctx, msg.PoolPubKey)
			if err != nil {
				ctx.Logger().Error("fail to get keygen metric", "error", err)
			} else {
				var total int64
				for _, item := range metric.NodeTssTimes {
					total += item.TssTime
				}
				evt := NewEventTssKeygenMetric(metric.PubKey, metric.GetMedianTime())
				if err = h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
					ctx.Logger().Error("fail to emit tss metric event", "error", err)
				}
			}

			if len(initVaults) == len(keygenBlock.Keygens) {
				ctx.Logger().Info("tss keygen results churn", "asgards", len(initVaults))
				for _, v := range initVaults {
					if err = h.mgr.NetworkMgr().RotateVault(ctx, v); err != nil {
						return nil, fmt.Errorf("fail to rotate vault: %w", err)
					}
				}
			} else {
				ctx.Logger().Info("not enough keygen yet", "expecting", len(keygenBlock.Keygens), "current", len(initVaults))
			}

			var addrs []cosmos.AccAddress
			addrs, err = vault.GetMembership().Addresses()
			members := make([]string, len(addrs))
			if err != nil {
				ctx.Logger().Error("fail to get member addresses", "error", err)
			} else {
				for i, addr := range addrs {
					members[i] = addr.String()
				}
				if err = h.mgr.EventMgr().EmitEvent(ctx, NewEventTssKeygenSuccess(msg.PoolPubKey, msg.Height, members)); err != nil {
					ctx.Logger().Error("fail to emit keygen success event")
				}
			}

		} else {
			// if a node fail to join the keygen, thus hold off the network
			// from churning then it will be slashed accordingly
			slashPoints := h.mgr.GetConstants().GetInt64Value(constants.FailKeygenSlashPoints)
			totalSlash := cosmos.ZeroUint()
			var nodePubKey common.PubKey
			for _, node := range msg.BlameLegacy.BlameNodes {
				nodePubKey, err = common.NewPubKey(node.Pubkey)
				if err != nil {
					return nil, ErrInternal(err, fmt.Sprintf("fail to parse pubkey(%s)", node.Pubkey))
				}

				var na NodeAccount
				na, err = h.mgr.Keeper().GetNodeAccountByPubKey(ctx, nodePubKey)
				if err != nil {
					return nil, fmt.Errorf("fail to get node from it's pub key: %w", err)
				}

				var naBond cosmos.Uint
				naBond, err = h.mgr.Keeper().CalcNodeLiquidityBond(ctx, na)
				if err != nil {
					return nil, fmt.Errorf("fail to calculate node liquidity bond: %w", err)
				}

				if na.Status == NodeActive {
					failedKeygenSlashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{
						telemetry.NewLabel("reason", "failed_keygen"),
					}))
					if err = h.mgr.Keeper().IncNodeAccountSlashPoints(failedKeygenSlashCtx, na.NodeAddress, slashPoints); err != nil {
						ctx.Logger().Error("fail to inc slash points", "error", err)
					}

					if err = h.mgr.EventMgr().EmitEvent(ctx, NewEventSlashPoint(na.NodeAddress, slashPoints, "fail keygen")); err != nil {
						ctx.Logger().Error("fail to emit slash point event")
					}
				} else {
					// go to jail
					jailTime := h.mgr.GetConstants().GetInt64Value(constants.JailTimeKeygen)
					releaseHeight := ctx.BlockHeight() + jailTime
					reason := "failed to perform keygen"
					if err = h.mgr.Keeper().SetNodeAccountJail(ctx, na.NodeAddress, releaseHeight, reason); err != nil {
						ctx.Logger().Error("fail to set node account jail", "node address", na.NodeAddress, "reason", reason, "error", err)
					}

					var network Network
					network, err = h.mgr.Keeper().GetNetwork(ctx)
					if err != nil {
						return nil, fmt.Errorf("fail to get network: %w", err)
					}

					slashBond := network.CalcNodeRewards(cosmos.NewUint(uint64(slashPoints)))
					if slashBond.GT(naBond) {
						slashBond = naBond
					}
					ctx.Logger().Info("fail keygen , slash bond", "address", na.NodeAddress, "amount", slashBond.String())
					totalSlash = totalSlash.Add(slashBond)

					var slashedAmount cosmos.Uint
					slashedAmount, _, err = h.mgr.Slasher().SlashNodeAccountLP(ctx, na, slashBond)
					if err != nil {
						return nil, fmt.Errorf("fail to slash node account: %w", err)
					}

					slashFloat, _ := new(big.Float).SetInt(slashedAmount.BigInt()).Float32()
					telemetry.IncrCounterWithLabels(
						[]string{"mayanode", "bond_slash"},
						slashFloat,
						[]metrics.Label{
							telemetry.NewLabel("address", na.NodeAddress.String()),
							telemetry.NewLabel("reason", "failed_keygen"),
						},
					)
				}
				if err = h.mgr.Keeper().SetNodeAccount(ctx, na); err != nil {
					return nil, fmt.Errorf("fail to save node account: %w", err)
				}

				tx := common.Tx{}
				tx.ID = common.BlankTxID
				tx.FromAddress = na.BondAddress
				if err = h.mgr.EventMgr().EmitBondEvent(ctx, h.mgr, common.BaseNative, totalSlash, BondCost, tx); err != nil {
					return nil, fmt.Errorf("fail to emit bond event: %w", err)
				}

			}

			if err := h.mgr.EventMgr().EmitEvent(ctx, NewEventTssKeygenFailure(msg.BlameLegacy.FailReason, msg.BlameLegacy.Round, msg.BlameLegacy.IsUnicast, msg.Height, blames)); err != nil {
				ctx.Logger().Error("fail to emit keygen failure event")
			}

		}
		return &cosmos.Result{}, nil
	}

	if (voter.BlockHeight + observeFlex) >= ctx.BlockHeight() {
		h.mgr.Slasher().DecSlashPoints(slashCtx, observeSlashPoints, msg.Signer)
	}

	return &cosmos.Result{}, nil
}

func (h TssHandler) handleV93(ctx cosmos.Context, msg MsgTssPool) (*cosmos.Result, error) {
	ctx.Logger().Info("handler tss", "current version", h.mgr.GetVersion())
	if !msg.BlameLegacy.IsEmpty() {
		blames := make([]string, len(msg.BlameLegacy.BlameNodes))
		for i := range msg.BlameLegacy.BlameNodes {
			pk, err := common.NewPubKey(msg.BlameLegacy.BlameNodes[i].Pubkey)
			if err != nil {
				ctx.Logger().Error("fail to get tss keygen pubkey", "pubkey", msg.BlameLegacy.BlameNodes[i].Pubkey, "error", err)
				continue
			}
			acc, err := pk.GetThorAddress()
			if err != nil {
				ctx.Logger().Error("fail to get tss keygen thor address", "pubkey", msg.BlameLegacy.BlameNodes[i].Pubkey, "error", err)
				continue
			}
			blames[i] = acc.String()
		}
		sort.Strings(blames)
		ctx.Logger().Info(
			"tss keygen results blame",
			"height", msg.Height,
			"id", msg.ID,
			"pubkey", msg.PoolPubKey,
			"round", msg.BlameLegacy.Round,
			"blames", strings.Join(blames, ", "),
			"reason", msg.BlameLegacy.FailReason,
			"blamer", msg.Signer,
		)
	}
	// only record TSS metric when keygen is success
	if msg.IsSuccess() && !msg.PoolPubKey.IsEmpty() {
		metric, err := h.mgr.Keeper().GetTssKeygenMetric(ctx, msg.PoolPubKey)
		if err != nil {
			ctx.Logger().Error("fail to get keygen metric", "error", err)
		} else {
			ctx.Logger().Info("save keygen metric to db")
			metric.AddNodeTssTime(msg.Signer, msg.KeygenTime)
			h.mgr.Keeper().SetTssKeygenMetric(ctx, metric)
		}
	}
	voter, err := h.mgr.Keeper().GetTssVoter(ctx, msg.ID)
	if err != nil {
		return nil, fmt.Errorf("fail to get tss voter: %w", err)
	}

	// when PoolPubKey is empty , which means TssVoter with id(msg.ID) doesn't
	// exist before, this is the first time to create it
	// set the PoolPubKey to the one in msg, there is no reason voter.PubKeys
	// have anything in it either, thus override it with msg.PubKeys as well
	if voter.PoolPubKey.IsEmpty() {
		voter.PoolPubKey = msg.PoolPubKey
		voter.PubKeys = msg.PubKeys
	}
	// voter's pool pubkey is the same as the one in message
	if !voter.PoolPubKey.Equals(msg.PoolPubKey) {
		return nil, fmt.Errorf("invalid pool pubkey")
	}
	observeSlashPoints := h.mgr.GetConstants().GetInt64Value(constants.ObserveSlashPoints)
	observeFlex := h.mgr.GetConstants().GetInt64Value(constants.ObservationDelayFlexibility)

	slashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{
		telemetry.NewLabel("reason", "failed_observe_tss_pool"),
	}))
	h.mgr.Slasher().IncSlashPoints(slashCtx, observeSlashPoints, msg.Signer)

	if !voter.Sign(msg.Signer, msg.Chains) {
		ctx.Logger().Info("signer already signed MsgTssPool", "signer", msg.Signer.String(), "txid", msg.ID)
		return &cosmos.Result{}, nil

	}
	h.mgr.Keeper().SetTssVoter(ctx, voter)

	// doesn't have 2/3 majority consensus yet
	if !voter.HasConsensus() {
		return &cosmos.Result{}, nil
	}

	// when keygen success
	if msg.IsSuccess() {
		h.judgeLateSigner(ctx, msg, voter)
		if !voter.HasCompleteConsensus() {
			return &cosmos.Result{}, nil
		}
	}

	if voter.BlockHeight == 0 {
		voter.BlockHeight = ctx.BlockHeight()
		h.mgr.Keeper().SetTssVoter(ctx, voter)
		h.mgr.Slasher().DecSlashPoints(slashCtx, observeSlashPoints, voter.GetSigners()...)
		if msg.IsSuccess() {
			ctx.Logger().Info(
				"tss keygen results success",
				"height", msg.Height,
				"id", msg.ID,
				"pubkey", msg.PoolPubKey,
			)
			vaultType := YggdrasilVault
			if msg.KeygenType == AsgardKeygen {
				vaultType = AsgardVault
			}
			chains := voter.ConsensusChains()
			vault := NewVault(ctx.BlockHeight(), InitVault, vaultType, voter.PoolPubKey, chains.Strings(), h.mgr.Keeper().GetChainContracts(ctx, chains))
			vault.Membership = voter.PubKeys

			if err = h.mgr.Keeper().SetVault(ctx, vault); err != nil {
				return nil, fmt.Errorf("fail to save vault: %w", err)
			}
			var keygenBlock KeygenBlock
			keygenBlock, err = h.mgr.Keeper().GetKeygenBlock(ctx, msg.Height)
			if err != nil {
				return nil, fmt.Errorf("fail to get keygen block, err: %w, height: %d", err, msg.Height)
			}
			var initVaults Vaults
			initVaults, err = h.mgr.Keeper().GetAsgardVaultsByStatus(ctx, InitVault)
			if err != nil {
				return nil, fmt.Errorf("fail to get init vaults: %w", err)
			}

			var metric *types.TssKeygenMetric
			metric, err = h.mgr.Keeper().GetTssKeygenMetric(ctx, msg.PoolPubKey)
			if err != nil {
				ctx.Logger().Error("fail to get keygen metric", "error", err)
			} else {
				var total int64
				for _, item := range metric.NodeTssTimes {
					total += item.TssTime
				}
				evt := NewEventTssKeygenMetric(metric.PubKey, metric.GetMedianTime())
				if err = h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
					ctx.Logger().Error("fail to emit tss metric event", "error", err)
				}
			}

			if len(initVaults) == len(keygenBlock.Keygens) {
				ctx.Logger().Info("tss keygen results churn", "asgards", len(initVaults))
				for _, v := range initVaults {
					if err = h.mgr.NetworkMgr().RotateVault(ctx, v); err != nil {
						return nil, fmt.Errorf("fail to rotate vault: %w", err)
					}
				}
			} else {
				ctx.Logger().Info("not enough keygen yet", "expecting", len(keygenBlock.Keygens), "current", len(initVaults))
			}
		} else {
			// if a node fail to join the keygen, thus hold off the network
			// from churning then it will be slashed accordingly
			slashPoints := h.mgr.GetConstants().GetInt64Value(constants.FailKeygenSlashPoints)
			totalSlash := cosmos.ZeroUint()
			var nodePubKey common.PubKey
			for _, node := range msg.BlameLegacy.BlameNodes {
				nodePubKey, err = common.NewPubKey(node.Pubkey)
				if err != nil {
					return nil, ErrInternal(err, fmt.Sprintf("fail to parse pubkey(%s)", node.Pubkey))
				}

				var na NodeAccount
				na, err = h.mgr.Keeper().GetNodeAccountByPubKey(ctx, nodePubKey)
				if err != nil {
					return nil, fmt.Errorf("fail to get node from it's pub key: %w", err)
				}

				var naBond cosmos.Uint
				naBond, err = h.mgr.Keeper().CalcNodeLiquidityBond(ctx, na)
				if err != nil {
					return nil, fmt.Errorf("fail to calculate node liquidity bond: %w", err)
				}

				if na.Status == NodeActive {
					failedKeygenSlashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{
						telemetry.NewLabel("reason", "failed_keygen"),
					}))
					if err = h.mgr.Keeper().IncNodeAccountSlashPoints(failedKeygenSlashCtx, na.NodeAddress, slashPoints); err != nil {
						ctx.Logger().Error("fail to inc slash points", "error", err)
					}

					if err = h.mgr.EventMgr().EmitEvent(ctx, NewEventSlashPoint(na.NodeAddress, slashPoints, "fail keygen")); err != nil {
						ctx.Logger().Error("fail to emit slash point event")
					}
				} else {
					// go to jail
					jailTime := h.mgr.GetConstants().GetInt64Value(constants.JailTimeKeygen)
					releaseHeight := ctx.BlockHeight() + jailTime
					reason := "failed to perform keygen"
					if err = h.mgr.Keeper().SetNodeAccountJail(ctx, na.NodeAddress, releaseHeight, reason); err != nil {
						ctx.Logger().Error("fail to set node account jail", "node address", na.NodeAddress, "reason", reason, "error", err)
					}

					var network Network
					network, err = h.mgr.Keeper().GetNetwork(ctx)
					if err != nil {
						return nil, fmt.Errorf("fail to get network: %w", err)
					}

					slashBond := network.CalcNodeRewards(cosmos.NewUint(uint64(slashPoints)))
					if slashBond.GT(naBond) {
						slashBond = naBond
					}
					ctx.Logger().Info("fail keygen , slash bond", "address", na.NodeAddress, "amount", slashBond.String())
					totalSlash = totalSlash.Add(slashBond)

					var slashedAmount cosmos.Uint
					slashedAmount, _, err = h.mgr.Slasher().SlashNodeAccountLP(ctx, na, slashBond)
					if err != nil {
						return nil, fmt.Errorf("fail to slash node account: %w", err)
					}

					slashFloat, _ := new(big.Float).SetInt(slashedAmount.BigInt()).Float32()
					telemetry.IncrCounterWithLabels(
						[]string{"mayanode", "bond_slash"},
						slashFloat,
						[]metrics.Label{
							telemetry.NewLabel("address", na.NodeAddress.String()),
							telemetry.NewLabel("reason", "failed_keygen"),
						},
					)
				}
				if err = h.mgr.Keeper().SetNodeAccount(ctx, na); err != nil {
					return nil, fmt.Errorf("fail to save node account: %w", err)
				}

				tx := common.Tx{}
				tx.ID = common.BlankTxID
				tx.FromAddress = na.BondAddress
				if err = h.mgr.EventMgr().EmitBondEvent(ctx, h.mgr, common.BaseNative, totalSlash, BondCost, tx); err != nil {
					return nil, fmt.Errorf("fail to emit bond event: %w", err)
				}

			}

		}
		return &cosmos.Result{}, nil
	}

	if (voter.BlockHeight + observeFlex) >= ctx.BlockHeight() {
		h.mgr.Slasher().DecSlashPoints(slashCtx, observeSlashPoints, msg.Signer)
	}

	return &cosmos.Result{}, nil
}

func (h TssHandler) validateV71(ctx cosmos.Context, msg MsgTssPool) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	newMsg, err := NewMsgTssPool(msg.PubKeys, msg.PoolPubKey, nil, msg.KeygenType, msg.Height, msg.BlameLegacy, msg.Chains, msg.Signer, msg.KeygenTime)
	if err != nil {
		return fmt.Errorf("fail to recreate MsgTssPool,err: %w", err)
	}
	if msg.ID != newMsg.ID {
		return cosmos.ErrUnknownRequest("invalid tss message")
	}

	churnRetryBlocks := h.mgr.GetConstants().GetInt64Value(constants.ChurnRetryInterval)
	if msg.Height <= ctx.BlockHeight()-churnRetryBlocks {
		return cosmos.ErrUnknownRequest("invalid keygen block")
	}

	keygenBlock, err := h.mgr.Keeper().GetKeygenBlock(ctx, msg.Height)
	if err != nil {
		return fmt.Errorf("fail to get keygen block from data store: %w", err)
	}

	for _, keygen := range keygenBlock.Keygens {
		keyGenMembers := keygen.GetMembers()
		if !msg.GetPubKeys().Equals(keyGenMembers) {
			continue
		}
		// Make sure the keygen type are consistent
		if msg.KeygenType != keygen.Type {
			continue
		}
		var addr cosmos.AccAddress
		for _, member := range keygen.GetMembers() {
			addr, err = member.GetThorAddress()
			if err == nil && addr.Equals(msg.Signer) {
				return h.validateSigner(ctx, msg.Signer)
			}
		}
	}

	return cosmos.ErrUnauthorized("not authorized")
}

func (h TssHandler) validateV110(ctx cosmos.Context, msg MsgTssPool) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	// PoolPubKey can't be empty only when keygen success
	if msg.IsSuccess() {
		if msg.PoolPubKey.IsEmpty() {
			return cosmos.ErrUnknownRequest("Pool pubkey cannot be empty")
		}
	}

	newMsg, err := NewMsgTssPool(msg.PubKeys, msg.PoolPubKey, nil, msg.KeygenType, msg.Height, msg.BlameLegacy, msg.Chains, msg.Signer, msg.KeygenTime)
	if err != nil {
		return fmt.Errorf("fail to recreate MsgTssPool,err: %w", err)
	}
	if msg.ID != newMsg.ID {
		return cosmos.ErrUnknownRequest("invalid tss message")
	}

	churnRetryBlocks := h.mgr.Keeper().GetConfigInt64(ctx, constants.ChurnRetryInterval)
	if msg.Height <= ctx.BlockHeight()-churnRetryBlocks {
		return cosmos.ErrUnknownRequest("invalid keygen block")
	}

	keygenBlock, err := h.mgr.Keeper().GetKeygenBlock(ctx, msg.Height)
	if err != nil {
		return fmt.Errorf("fail to get keygen block from data store: %w", err)
	}

	for _, keygen := range keygenBlock.Keygens {
		keyGenMembers := keygen.GetMembers()
		if !msg.GetPubKeys().Equals(keyGenMembers) {
			continue
		}
		// Make sure the keygen type are consistent
		if msg.KeygenType != keygen.Type {
			continue
		}
		var addr cosmos.AccAddress
		for _, member := range keygen.GetMembers() {
			addr, err = member.GetThorAddress()
			if err == nil && addr.Equals(msg.Signer) {
				return h.validateSigner(ctx, msg.Signer)
			}
		}
	}

	return cosmos.ErrUnauthorized("not authorized")
}
