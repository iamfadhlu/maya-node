package mayachain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

func (h BondHandler) validateV105(ctx cosmos.Context, msg MsgBond) error {
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
		if fetchConfigInt64(ctx, h.mgr, constants.PauseBond) > 0 {
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

	// Attempting to set Operator Fee. If the Node has no bond address yet, it will have no fee set, continue
	if msg.OperatorFee > -1 && !nodeAccount.BondAddress.IsEmpty() {
		// Only Node Operator can set fee
		if !msg.BondAddress.Equals(nodeAccount.BondAddress) {
			return cosmos.ErrUnknownRequest("only node operator can set fee")
		}

		var nodeOpAddr cosmos.AccAddress
		nodeOpAddr, err = nodeAccount.BondAddress.AccAddress()
		if err != nil {
			return ErrInternal(err, fmt.Sprintf("fail to parse node account bond address(%s)", nodeAccount.BondAddress))
		}

		// Can't increase operator fee after a (non-operator) provider has bonded
		if msg.OperatorFee > bp.NodeOperatorFee.BigInt().Int64() {
			if bp.HasProviderBonded(nodeOpAddr) {
				return cosmos.ErrUnknownRequest("can't increase operator fee after a provider has bonded")
			}
		}
		// After that if they want to add more bond they have to do it through add_liquidity
	}

	// If node has no bond address or the bond address is not the node account's bond address,
	// then it must be in the list of bond providers
	if !nodeAccount.BondAddress.IsEmpty() && !nodeAccount.BondAddress.Equals(msg.BondAddress) {
		var from cosmos.AccAddress
		from, err = msg.BondAddress.AccAddress()
		if err != nil {
			return ErrInternal(err, fmt.Sprintf("fail to get msg bond address(%s)", msg.BondAddress))
		}
		if p := bp.Get(from); p.IsEmpty() {
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
		lpBond, err = calcLiquidityInCacao(ctx, h.mgr, msg.Asset, units)
		if err != nil {
			return ErrInternal(err, fmt.Sprintf("fail to calculate LP bond: %s", msg.BondAddress.String()))
		}
		bond := nodeBond.Add(lpBond)
		var maxBond int64
		maxBond, err = h.mgr.Keeper().GetMimir(ctx, "MaximumBondInRune")
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

func (h BondHandler) handleV105(ctx cosmos.Context, msg MsgBond) error {
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

	bp, err := h.mgr.Keeper().GetBondProviders(ctx, nodeAccount.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get bond providers(%s)", msg.NodeAddress))
	}

	// backfill bond provider information (passive migration code)
	if len(bp.Providers) == 0 {
		// no providers yet, add node operator bond address to the bond provider list
		var nodeOpBondAddr cosmos.AccAddress
		nodeOpBondAddr, err = nodeAccount.BondAddress.AccAddress()
		if err != nil {
			return ErrInternal(err, fmt.Sprintf("fail to parse bond address(%s)", msg.BondAddress))
		}
		p := NewBondProvider(nodeOpBondAddr)
		bp.Providers = append(bp.Providers, p)
		defaultNodeOperationFee := fetchConfigInt64(ctx, h.mgr, constants.NodeOperatorFee)
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
	if msg.OperatorFee > -1 && msg.OperatorFee < 10000 {
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

		var from cosmos.AccAddress
		from, err = msg.BondAddress.AccAddress()
		if err != nil {
			return ErrInternal(err, fmt.Sprintf("fail to get msg bond address(%s)", msg.BondAddress))
		}
		bp.BondLiquidity(from)
	}

	if err = h.mgr.Keeper().SetNodeAccount(ctx, nodeAccount); err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to save node account(%s)", nodeAccount.String()))
	}

	if err = h.mgr.Keeper().SetBondProviders(ctx, bp); err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to save bond providers(%s)", bp.NodeAddress.String()))
	}

	bondEvent := NewEventBondV105(msg.Asset, units, BondPaid, msg.TxIn)
	if err = h.mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
		ctx.Logger().Error("fail to emit bond event", "error", err)
	}

	return nil
}

func (h BondHandler) validateV96(ctx cosmos.Context, msg MsgBond) error {
	if err := msg.ValidateBasicV96(); err != nil {
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
		if fetchConfigInt64(ctx, h.mgr, constants.PauseBond) > 0 {
			return ErrInternal(err, "bonding has been paused")
		}
	}

	if !msg.BondAddress.IsChain(common.BASEChain, h.mgr.GetVersion()) {
		return cosmos.ErrUnknownRequest(fmt.Sprintf("bonding address is NOT a BASEChain address: %s", msg.BondAddress.String()))
	}

	nodeBond, err := h.mgr.Keeper().CalcNodeLiquidityBond(ctx, nodeAccount)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to calculate node liquidity: %s", nodeAccount.NodeAddress.String()))
	}

	lpBond, err := h.mgr.Keeper().CalcLPLiquidityBond(ctx, msg.BondAddress, msg.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to calculate lp liquidity: %s", msg.BondAddress))
	}

	bp, err := h.mgr.Keeper().GetBondProviders(ctx, msg.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get bond providers(%s)", msg.NodeAddress))
	}

	// Attempting to set Operator Fee. If the Node has no bond address yet, it will have no fee set, continue
	if msg.OperatorFee > -1 && !nodeAccount.BondAddress.IsEmpty() {
		// Only Node Operator can set fee
		if !msg.BondAddress.Equals(nodeAccount.BondAddress) {
			return cosmos.ErrUnknownRequest("only node operator can set fee")
		}

		var nodeOpAddr cosmos.AccAddress
		nodeOpAddr, err = nodeAccount.BondAddress.AccAddress()
		if err != nil {
			return ErrInternal(err, fmt.Sprintf("fail to parse node account bond address(%s)", nodeAccount.BondAddress))
		}

		// Can't increase operator fee after a (non-operator) provider has bonded
		if msg.OperatorFee > bp.NodeOperatorFee.BigInt().Int64() {
			if bp.HasProviderBonded(nodeOpAddr) {
				return cosmos.ErrUnknownRequest("can't increase operator fee after a provider has bonded")
			}
		}
		// After that if they want to add more bond they have to do it through add_liquidity
	}

	if lpBond.LTE(cosmos.ZeroUint()) {
		return cosmos.ErrUnknownRequest(fmt.Sprintf("insufficient liquidity in whitelisted pools: %s", msg.BondAddress))
	}

	// Validate bond address
	if msg.BondAddress.Equals(nodeAccount.BondAddress) {
		return nil
	}

	if nodeAccount.BondAddress.IsEmpty() {
		// no bond address yet, allow it to be bonded by any address
		return nil
	}

	from, err := msg.BondAddress.AccAddress()
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get msg bond address(%s)", msg.BondAddress))
	}

	if !bp.Has(from) {
		return cosmos.ErrUnknownRequest("bond address is not valid for node account")
	}

	if b := bp.Get(from); b.HasBonded() {
		return cosmos.ErrUnknownRequest("cannot add more bond through msg_bond")
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

	return nil
}

func (h BondHandler) handleV95(ctx cosmos.Context, msg MsgBond) error {
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
		coins := common.NewCoins(common.NewCoin(common.BaseAsset(), msg.Amount))
		if err = h.mgr.Keeper().SendFromModuleToAccount(ctx, BondName, msg.NodeAddress, coins); err != nil {
			ctx.Logger().Error("fail to msg RUNE to node address", "error", err)
			nodeAccount.Status = NodeUnknown
		}

		tx := common.Tx{}
		tx.ID = common.BlankTxID
		tx.ToAddress = common.Address(nodeAccount.String())
		bondEvent := NewEventBond(coins[0].Amount, BondCost, tx)
		if err = h.mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
			ctx.Logger().Error("fail to emit bond event", "error", err)
		}
	}

	bp, err := h.mgr.Keeper().GetBondProviders(ctx, nodeAccount.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get bond providers(%s)", msg.NodeAddress))
	}

	from, err := msg.BondAddress.AccAddress()
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get msg bond address(%s)", msg.BondAddress))
	}

	// backfill bond provider information (passive migration code)
	if len(bp.Providers) == 0 {
		// no providers yet, add node operator bond address to the bond provider list
		var nodeOpBondAddr cosmos.AccAddress
		nodeOpBondAddr, err = nodeAccount.BondAddress.AccAddress()
		if err != nil {
			return ErrInternal(err, fmt.Sprintf("fail to parse bond address(%s)", msg.BondAddress))
		}
		p := NewBondProvider(nodeOpBondAddr)
		bp.Providers = append(bp.Providers, p)
		defaultNodeOperationFee := fetchConfigInt64(ctx, h.mgr, constants.NodeOperatorFee)
		bp.NodeOperatorFee = cosmos.NewUint(uint64(defaultNodeOperationFee))
	}

	// Only add bond if it's the first time this bp is providing liquidity
	// afterwards they should do it through add_liquidity
	liquidityBond := cosmos.ZeroUint()
	lps := LiquidityProviders{}
	if b := bp.Get(from); len(bp.Providers) == 0 || !b.IsEmpty() && !b.HasBonded() {
		liquidityPools := GetLiquidityPools(h.mgr.GetVersion())
		lps, err = h.mgr.Keeper().GetLiquidityProviderByAssets(ctx, liquidityPools, msg.BondAddress)
		if err != nil {
			return ErrInternal(err, fmt.Sprintf("fail to get lps for signer: %s", msg.NodeAddress))
		}

		// trunk-ignore(golangci-lint/staticcheck)
		lps.SetNodeAccount(nodeAccount.NodeAddress)
		bp.BondLiquidity(from)
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
	if msg.OperatorFee > -1 && msg.OperatorFee < 10000 {
		bp.NodeOperatorFee = cosmos.NewUint(uint64(msg.OperatorFee))
	}

	if err = h.mgr.Keeper().SetNodeAccount(ctx, nodeAccount); err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to save node account(%s)", nodeAccount.String()))
	}

	if err = h.mgr.Keeper().SetBondProviders(ctx, bp); err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to save bond providers(%s)", bp.NodeAddress.String()))
	}

	// Add the NodeBondAddress to lps
	h.mgr.Keeper().SetLiquidityProviders(ctx, lps)

	bondEvent := NewEventBond(liquidityBond, BondPaid, msg.TxIn)
	if err := h.mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
		ctx.Logger().Error("fail to emit bond event", "error", err)
	}

	return nil
}
