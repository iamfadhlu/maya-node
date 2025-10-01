package mayachain

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/armon/go-metrics"
	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/telemetry"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

// TssHandler handle MsgTssPool
type TssHandler struct {
	mgr Manager
}

// NewTssHandler create a new handler to process MsgTssPool
func NewTssHandler(mgr Manager) TssHandler {
	return TssHandler{
		mgr: mgr,
	}
}

// Run is the main entry for TssHandler
func (h TssHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgTssPool)
	if !ok {
		return nil, errInvalidMessage
	}
	err := h.validate(ctx, *msg)
	if err != nil {
		ctx.Logger().Error("msg_tss_pool failed validation", "error", err)
		return nil, err
	}
	result, err := h.handle(ctx, *msg)
	if err != nil {
		ctx.Logger().Error("failed to process MsgTssPool", "error", err)
		return nil, err
	}
	return result, err
}

func (h TssHandler) validate(ctx cosmos.Context, msg MsgTssPool) error {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.124.0")):
		return h.validateV124(ctx, msg)
	case version.GTE(semver.MustParse("1.110.0")):
		return h.validateV110(ctx, msg)
	case version.GTE(semver.MustParse("0.71.0")):
		return h.validateV71(ctx, msg)
	default:
		return errBadVersion
	}
}

func (h TssHandler) validateV124(ctx cosmos.Context, msg MsgTssPool) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	// PoolPubKey can't be empty only when keygen success
	if msg.IsSuccessV2() {
		if msg.PoolPubKey.IsEmpty() {
			return cosmos.ErrUnknownRequest("Pool pubkey cannot be empty")
		}
	}

	if !msg.PoolPubKeyEddsa.IsEmpty() {
		if _, err := common.NewPubKey(msg.PoolPubKeyEddsa.String()); err != nil {
			return cosmos.ErrUnknownRequest(err.Error())
		}
	}

	// Convert []*Blame to []Blame for validation
	blameValues := make([]Blame, len(msg.Blame))
	for i, b := range msg.Blame {
		if b != nil {
			blameValues[i] = *b
		}
	}
	newMsg, err := NewMsgTssPoolV2(msg.PubKeys, msg.PoolPubKey, nil, msg.KeygenType, msg.Height, blameValues, msg.Chains, msg.Signer, msg.KeygenTime, msg.PoolPubKeyEddsa, msg.KeysharesBackupEddsa)
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

func (h TssHandler) validateSigner(ctx cosmos.Context, signer cosmos.AccAddress) error {
	nodeSigner, err := h.mgr.Keeper().GetNodeAccount(ctx, signer)
	if err != nil {
		return fmt.Errorf("invalid signer")
	}
	if nodeSigner.IsEmpty() {
		return fmt.Errorf("invalid signer")
	}
	if nodeSigner.Status != NodeActive && nodeSigner.Status != NodeReady {
		return fmt.Errorf("invalid signer status(%s)", nodeSigner.Status)
	}
	nodeSignerBond, err := h.mgr.Keeper().CalcNodeLiquidityBond(ctx, nodeSigner)
	if err != nil {
		return fmt.Errorf("fail to calculate node liquidity bond: %w", err)
	}
	// ensure we have enough rune
	minBond, err := h.mgr.Keeper().GetMimir(ctx, constants.MinimumBondInCacao.String())
	if minBond < 0 || err != nil {
		minBond = h.mgr.GetConstants().GetInt64Value(constants.MinimumBondInCacao)
	}
	if nodeSignerBond.LT(cosmos.NewUint(uint64(minBond))) {
		return fmt.Errorf("signer doesn't have enough rune")
	}
	return nil
}

func (h TssHandler) handle(ctx cosmos.Context, msg MsgTssPool) (*cosmos.Result, error) {
	ctx.Logger().Info("handleMsgTssPool request", "ID:", msg.ID)
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.124.0")):
		return h.handleV124(ctx, msg)
	case version.GTE(semver.MustParse("1.123.0")):
		return h.handleV123(ctx, msg)
	case version.GTE(semver.MustParse("1.122.0")):
		return h.handleV122(ctx, msg)
	case version.GTE(semver.MustParse("1.93.0")):
		return h.handleV93(ctx, msg)
	default:
		return nil, errBadVersion
	}
}

func (h TssHandler) handleV124(ctx cosmos.Context, msg MsgTssPool) (*cosmos.Result, error) {
	ctx.Logger().Info("handler tss", "current version", h.mgr.GetVersion())
	blames := make([]string, 0)
	if len(msg.Blame) > 0 {
		blames = make([]string, len(msg.Blame))
		var failReason string
		for _, b := range msg.Blame {
			for _, bn := range b.BlameNodes {
				pk, err := common.NewPubKey(bn.Pubkey)
				if err != nil {
					ctx.Logger().Error("fail to get tss keygen pubkey", "pubkey", bn.Pubkey, "error", err)
					continue
				}
				acc, err := pk.GetThorAddress()
				if err != nil {
					ctx.Logger().Error("fail to get tss keygen thor address", "pubkey", bn.Pubkey, "error", err)
					continue
				}
				blames = append(blames, acc.String())
			}
			if len(failReason) == 0 {
				failReason = b.FailReason
			} else {
				failReason = fmt.Sprintf("%s: %s", failReason, b.FailReason)
			}
		}
		sort.Strings(blames)
		ctx.Logger().Info(
			"tss keygen results blame",
			"height", msg.Height,
			"id", msg.ID,
			"pubkey", msg.PoolPubKey,
			"pubkey_ecdsa", msg.PoolPubKey,
			"pubkey_eddsa", msg.PoolPubKeyEddsa,
			"blames", strings.Join(blames, ", "),
			"reason", failReason,
			"blamer", msg.Signer,
		)
	}
	// only record TSS metric when keygen is success
	if msg.IsSuccessV2() && !msg.PoolPubKey.IsEmpty() {
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
		voter.PoolPubKeyEddsa = msg.PoolPubKeyEddsa
		voter.PubKeys = msg.PubKeys
	}
	// voter's pool pubkey is the same as the one in message
	if !voter.PoolPubKey.Equals(msg.PoolPubKey) || !voter.PoolPubKeyEddsa.Equals(msg.PoolPubKeyEddsa) {
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
	if msg.IsSuccessV2() {
		h.judgeLateSigner(ctx, msg, voter)
		if !voter.HasCompleteConsensus() {
			return &cosmos.Result{}, nil
		}
	}

	if voter.BlockHeight == 0 {
		voter.BlockHeight = ctx.BlockHeight()
		h.mgr.Keeper().SetTssVoter(ctx, voter)
		h.mgr.Slasher().DecSlashPoints(slashCtx, observeSlashPoints, voter.GetSigners()...)
		if msg.IsSuccessV2() {
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
			vault := NewVaultV2(ctx.BlockHeight(), InitVault, vaultType, voter.PoolPubKey, chains.Strings(), h.mgr.Keeper().GetChainContracts(ctx, chains), voter.PoolPubKeyEddsa)
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
			for _, b := range msg.Blame {
				for _, node := range b.BlameNodes {
					nodePubKey, err = common.NewPubKey(node.Pubkey)
					if err != nil {
						return nil, ErrInternal(err, fmt.Sprintf("fail to parse pubkey(%s)", node.Pubkey))
					}

					var na NodeAccount
					na, err = h.mgr.Keeper().GetNodeAccountByPubKey(ctx, nodePubKey)
					if err != nil {
						return nil, fmt.Errorf("fail to get node from it's pub key: %w", err)
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

						var naBond cosmos.Uint
						naBond, err = h.mgr.Keeper().CalcNodeLiquidityBond(ctx, na)
						if err != nil {
							return nil, fmt.Errorf("fail to calculate node liquidity bond: %w", err)
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

				if err := h.mgr.EventMgr().EmitEvent(ctx, NewEventTssKeygenFailure(b.FailReason, b.Round, b.IsUnicast, msg.Height, blames)); err != nil {
					ctx.Logger().Error("fail to emit keygen failure event")
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

func (h TssHandler) judgeLateSigner(ctx cosmos.Context, msg MsgTssPool, voter TssVoter) {
	// if the voter doesn't reach 2/3 majority consensus , this method should not take any actions
	if !voter.HasConsensus() || !msg.IsSuccessV2() {
		return
	}
	slashPoints := h.mgr.GetConstants().GetInt64Value(constants.FailKeygenSlashPoints)
	slashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{
		telemetry.NewLabel("reason", "failed_observe_tss_pool"),
	}))

	// when voter already has 2/3 majority signers , restore current message signer's slash points
	if voter.MajorityConsensusBlockHeight > 0 {
		h.mgr.Slasher().DecSlashPoints(slashCtx, slashPoints, msg.Signer)
		if err := h.mgr.Keeper().ReleaseNodeAccountFromJail(ctx, msg.Signer); err != nil {
			ctx.Logger().Error("fail to release node account from jail", "node address", msg.Signer, "error", err)
		}
		return
	}

	voter.MajorityConsensusBlockHeight = ctx.BlockHeight()
	h.mgr.Keeper().SetTssVoter(ctx, voter)
	for _, member := range msg.PubKeys {
		pkey, err := common.NewPubKey(member)
		if err != nil {
			ctx.Logger().Error("fail to get pub key", "error", err)
			continue
		}
		thorAddr, err := pkey.GetThorAddress()
		if err != nil {
			ctx.Logger().Error("fail to get thor address", "error", err)
			continue
		}
		// whoever is in the keygen list , but didn't broadcast MsgTssPool
		if !voter.HasSigned(thorAddr) {
			h.mgr.Slasher().IncSlashPoints(slashCtx, slashPoints, thorAddr)
			// go to jail
			jailTime := h.mgr.GetConstants().GetInt64Value(constants.JailTimeKeygen)
			releaseHeight := ctx.BlockHeight() + jailTime
			reason := "failed to vote keygen in time"
			if err = h.mgr.Keeper().SetNodeAccountJail(ctx, thorAddr, releaseHeight, reason); err != nil {
				ctx.Logger().Error("fail to set node account jail", "node address", thorAddr, "reason", reason, "error", err)
			}
		}
	}
}
