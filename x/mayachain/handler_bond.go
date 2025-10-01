package mayachain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

// BondHandler a handler to process bond
type BondHandler struct {
	mgr Manager
}

// NewBondHandler create new BondHandler
func NewBondHandler(mgr Manager) BondHandler {
	return BondHandler{
		mgr: mgr,
	}
}

// Run execute the handler
func (h BondHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgBond)
	if !ok {
		return nil, errInvalidMessage
	}
	ctx.Logger().Info("receive MsgBond",
		"node address", msg.NodeAddress,
		"request hash", msg.TxIn.ID,
		"signer", msg.Signer)
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("msg bond fail validation", "error", err)
		return nil, err
	}

	err := h.handle(ctx, *msg)
	if err != nil {
		ctx.Logger().Error("fail to process msg bond", "error", err)
		return nil, err
	}

	return &cosmos.Result{}, nil
}

func (h BondHandler) validate(ctx cosmos.Context, msg MsgBond) error {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.107.0")):
		return h.validateV107(ctx, msg)
	case version.GTE(semver.MustParse("1.105.0")):
		return h.validateV105(ctx, msg)
	case version.GTE(semver.MustParse("1.96.0")):
		return h.validateV96(ctx, msg)
	default:
		return errBadVersion
	}
}

func (h BondHandler) validateV107(ctx cosmos.Context, msg MsgBond) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	// When RUNE is on thorchain , pay bond doesn't need to be active node
	// in fact , usually the node will not be active at the time it bond
	nodeAccount, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get node account(%s)", msg.NodeAddress))
	}

	if nodeAccount.Status == NodeReady {
		return ErrInternal(err, "cannot add bond while node is ready status")
	}

	if h.mgr.GetVersion().GTE(semver.MustParse("1.88.1")) {
		if h.mgr.Keeper().GetConfigInt64(ctx, constants.PauseBond) > 0 {
			return ErrInternal(err, "bonding has been paused")
		}
	}

	if !msg.BondAddress.IsChain(common.BASEChain, h.mgr.GetVersion()) {
		return cosmos.ErrUnknownRequest(fmt.Sprintf("bonding address is NOT a BASEChain address: %s", msg.BondAddress.String()))
	}

	bp, err := h.mgr.Keeper().GetBondProviders(ctx, msg.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get bond providers(%s)", msg.NodeAddress))
	}

	// Only Node Operator can set fee
	if msg.OperatorFee > -1 &&
		!nodeAccount.BondAddress.IsEmpty() &&
		!msg.BondAddress.Equals(nodeAccount.BondAddress) {
		return cosmos.ErrUnknownRequest("only node operator can set fee")
	}

	// If node has no bond address or the bond address is not the node account's bond address,
	// then it must be in the list of bond providers
	if !nodeAccount.BondAddress.IsEmpty() && !nodeAccount.BondAddress.Equals(msg.BondAddress) {
		var from cosmos.AccAddress
		from, err = msg.BondAddress.AccAddress()
		if err != nil {
			return ErrInternal(err, fmt.Sprintf("fail to get msg bond address(%s)", msg.BondAddress))
		}
		if !bp.Has(from) {
			return cosmos.ErrUnauthorized("address is not a valid bond provider for this node")
		}
	}

	if !msg.Asset.IsEmpty() {
		if !msg.BondProviderAddress.Empty() || msg.OperatorFee < -1 {
			return cosmos.ErrUnknownRequest("cannot set provider address or operator fee when bonding with an asset")
		}

		var lp LiquidityProvider
		lp, err = h.mgr.Keeper().GetLiquidityProvider(ctx, msg.Asset, msg.BondAddress)
		if err != nil {
			return ErrInternal(err, fmt.Sprintf("fail to get liquidity provider: %s, %s", msg.BondAddress.String(), msg.Asset.String()))
		}
		remLPUnits := lp.GetRemainingUnits()
		if remLPUnits.IsZero() {
			return cosmos.ErrUnknownRequest(fmt.Sprintf("no free liquidity units in pool to bond: %s, %s", msg.BondAddress, msg.Asset))
		}
		if remLPUnits.LT(msg.Units) {
			return cosmos.ErrUnknownRequest(fmt.Sprintf("insufficient free liquidity units in pool to bond: %s, %s only has %s free units", msg.BondAddress, msg.Asset, remLPUnits))
		}
		var maxLPBondedNodes int64
		maxLPBondedNodes, err = h.mgr.Keeper().GetMimir(ctx, "MaximumLPBondedNodes")
		if maxLPBondedNodes > 0 && err == nil {
			// Don't allow to bond to a new node if the maximum number of nodes has been reached
			if len(lp.BondedNodes) >= int(maxLPBondedNodes) && lp.GetUnitsBondedToNode(msg.NodeAddress).IsZero() {
				return cosmos.ErrUnknownRequest("lp has reached maximum bonded nodes")
			}
		}

		units := msg.Units
		if msg.Units.IsZero() {
			units = remLPUnits
		}

		var nodeBond, lpBond cosmos.Uint
		nodeBond, err = h.mgr.Keeper().CalcNodeLiquidityBond(ctx, nodeAccount)
		if err != nil {
			return ErrInternal(err, fmt.Sprintf("fail to calculate node liquidity: %s", nodeAccount.NodeAddress.String()))
		}
		lpBond, err := calcLiquidityInCacao(ctx, h.mgr, msg.Asset, units)
		if err != nil {
			return ErrInternal(err, fmt.Sprintf("fail to calculate LP bond: %s", msg.BondAddress.String()))
		}
		bond := nodeBond.Add(lpBond)
		maxBond, err := h.mgr.Keeper().GetMimir(ctx, "MaximumBondInRune")
		if maxBond > 0 && err == nil {
			maxValidatorBond := cosmos.NewUint(uint64(maxBond))
			if bond.GT(maxValidatorBond) {
				return cosmos.ErrUnknownRequest(
					fmt.Sprintf("too much bond, max validator bond (%s), bond(%s)", maxValidatorBond.String(), bond),
				)
			}
		}

		liquidityPools := GetLiquidityPools(h.mgr.GetVersion())
		if found := common.ContainsAsset(msg.Asset, liquidityPools); !found {
			return cosmos.ErrUnknownRequest("asset is not in valid liquidity pools list")
		}
	}

	return nil
}

func (h BondHandler) handle(ctx cosmos.Context, msg MsgBond) error {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.107.0")):
		return h.handleV107(ctx, msg)
	case version.GTE(semver.MustParse("1.105.0")):
		return h.handleV105(ctx, msg)
	case version.GTE(semver.MustParse("1.95.0")):
		return h.handleV95(ctx, msg)
	default:
		return errBadVersion
	}
}

func (h BondHandler) handleV107(ctx cosmos.Context, msg MsgBond) error {
	nodeAccount, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get node account(%s)", msg.NodeAddress))
	}

	acct := h.mgr.Keeper().GetAccount(ctx, msg.NodeAddress)

	if nodeAccount.Status == NodeUnknown {
		// THORNode will not have pub keys at the moment, so have to leave it empty
		emptyPubKeySet := common.PubKeySet{
			Secp256k1: common.EmptyPubKey,
			Ed25519:   common.EmptyPubKey,
		}
		// white list the given bep address
		nodeAccount = NewNodeAccount(msg.NodeAddress, NodeWhiteListed, emptyPubKeySet, "", "", cosmos.ZeroUint(), msg.BondAddress, ctx.BlockHeight())
		ctx.EventManager().EmitEvent(
			cosmos.NewEvent("new_node",
				cosmos.NewAttribute("address", msg.NodeAddress.String()),
			))
	}

	// when node bond for the first time , send 1 RUNE to node address
	// so as the node address will be created on BASEChain otherwise node account won't be able to send tx
	if acct == nil && msg.Amount.GTE(cosmos.NewUint(common.One)) {
		// Send the same amount sent in the msg to the Node Account
		// TODO: Refund any extra amount sent?
		coins := common.NewCoins(common.NewCoin(common.BaseAsset(), msg.Amount))
		if err = h.mgr.Keeper().SendFromModuleToAccount(ctx, BondName, msg.NodeAddress, coins); err != nil {
			ctx.Logger().Error("fail to msg RUNE to node address", "error", err)
			nodeAccount.Status = NodeUnknown
		}

		tx := common.Tx{}
		tx.ID = common.BlankTxID
		tx.ToAddress = common.Address(nodeAccount.String())
		bondEvent := NewEventBondV105(common.BaseNative, coins[0].Amount, BondCost, tx)
		if err = h.mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
			ctx.Logger().Error("fail to emit bond event", "error", err)
		}
	}

	var bp BondProviders
	bp, err = h.mgr.Keeper().GetBondProviders(ctx, nodeAccount.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get bond providers(%s)", msg.NodeAddress))
	}

	// if no providers yet, add node operator bond address to the bond provider list
	if len(bp.Providers) == 0 {
		// no providers yet, add node operator bond address to the bond provider list
		var nodeOpBondAddr cosmos.AccAddress
		nodeOpBondAddr, err = nodeAccount.BondAddress.AccAddress()
		if err != nil {
			return ErrInternal(err, fmt.Sprintf("fail to parse bond address(%s)", msg.BondAddress))
		}
		p := NewBondProvider(nodeOpBondAddr)
		bp.Providers = append(bp.Providers, p)
		defaultNodeOperationFee := h.mgr.Keeper().GetConfigInt64(ctx, constants.NodeOperatorFee)
		bp.NodeOperatorFee = cosmos.NewUint(uint64(defaultNodeOperationFee))
	}

	// if bonder is node operator, add additional bonding address
	if msg.BondAddress.Equals(nodeAccount.BondAddress) && !msg.BondProviderAddress.Empty() {
		var max int64
		max, err = h.mgr.Keeper().GetMimir(ctx, constants.MaxBondProviders.String())
		if err != nil || max < 0 {
			max = h.mgr.GetConstants().GetInt64Value(constants.MaxBondProviders)
		}
		if int64(len(bp.Providers)) >= max {
			return fmt.Errorf("additional bond providers are not allowed, maximum reached")
		}
		if !bp.Has(msg.BondProviderAddress) {
			bp.Providers = append(bp.Providers, NewBondProvider(msg.BondProviderAddress))
		}
	}

	// Update operator fee (-1 means operator fee is not being set)
	if msg.OperatorFee > -1 && msg.OperatorFee <= 10000 {
		bp.NodeOperatorFee = cosmos.NewUint(uint64(msg.OperatorFee))
	}

	units := msg.Units
	if !msg.Asset.IsEmpty() {
		var lp LiquidityProvider
		lp, err = h.mgr.Keeper().GetLiquidityProvider(ctx, msg.Asset, msg.BondAddress)
		if err != nil {
			return ErrInternal(err, fmt.Sprintf("fail to get liquidity provider: %s, %s", msg.BondAddress, msg.Asset))
		}
		if units.IsZero() {
			units = lp.GetRemainingUnits()
		}

		lp.Bond(msg.NodeAddress, units)
		h.mgr.Keeper().SetLiquidityProvider(ctx, lp)

		from, err := msg.BondAddress.AccAddress()
		if err != nil {
			return ErrInternal(err, fmt.Sprintf("fail to get msg bond address(%s)", msg.BondAddress))
		}
		bp.BondLiquidity(from)
	}

	// we want to pay the rewards of the user that bonded not the specified one
	if bp.HasRewards(msg.Signer) {
		provider := bp.Get(msg.Signer)
		err := payBondProviderReward(ctx, h.mgr, provider, bp)
		if err != nil {
			// we don't want to disrupt the bond process if we fail to pay the bond provider
			ctx.Logger().Error("fail to pay bond provider reward", "error", err)
		}
	}

	if err := h.mgr.Keeper().SetNodeAccount(ctx, nodeAccount); err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to save node account(%s)", nodeAccount.String()))
	}

	if err := h.mgr.Keeper().SetBondProviders(ctx, bp); err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to save bond providers(%s)", bp.NodeAddress.String()))
	}

	bondEvent := NewEventBondV105(msg.Asset, units, BondPaid, msg.TxIn)
	if err := h.mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
		ctx.Logger().Error("fail to emit bond event", "error", err)
	}

	return nil
}
