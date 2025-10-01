package types

import (
	"fmt"
	"strconv"
	"strings"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

// all event types support by BASEChain
const (
	AddLiquidityEventType         = "add_liquidity"
	BondEventType                 = "bond"
	DonateEventType               = "donate"
	ErrataEventType               = "errata"
	FeeEventType                  = "fee"
	GasEventType                  = "gas"
	OutboundEventType             = "outbound"
	PendingLiquidity              = "pending_liquidity"
	PoolBalanceChangeEventType    = "pool_balance_change"
	PoolEventType                 = "pool"
	RefundEventType               = "refund"
	ReserveEventType              = "reserve"
	RewardEventType               = "rewards"
	ScheduledOutboundEventType    = "scheduled_outbound"
	SecurityEventType             = "security"
	SetMimirEventType             = "set_mimir"
	SetNodeMimirEventType         = "set_node_mimir"
	SlashEventType                = "slash"
	SlashLiquidityEventType       = "slash_liquidity"
	SlashPointEventType           = "slash_points"
	StreamingSwapEventType        = "streaming_swap"
	SwapEventType                 = "swap"
	AffiliateFeeEventType         = "affiliate_fee"
	SwitchEventType               = "switch"
	MAYANameEventType             = "mayaname"
	CACAOPoolDepositEventType     = "cacao_pool_deposit"
	CACAOPoolWithdrawEventType    = "cacao_pool_withdraw"
	TSSKeygenSuccess              = "tss_keygen_success"
	TSSKeygenFailure              = "tss_keygen_failure"
	TSSKeygenMetricEventType      = "tss_keygen"
	TSSKeysignMetricEventType     = "tss_keysign"
	WithdrawEventType             = "withdraw"
	TradeAccountDepositEventType  = "trade_account_deposit"
	TradeAccountWithdrawEventType = "trade_account_withdraw"
)

// PoolMods a list of pool modifications
type PoolMods []PoolMod

// NewPoolMod create a new instance of PoolMod
func NewPoolMod(asset common.Asset, mayaAmt cosmos.Uint, mayaAdd bool, assetAmt cosmos.Uint, assetAdd bool) PoolMod {
	return PoolMod{
		Asset:    asset,
		CacaoAmt: mayaAmt,
		CacaoAdd: mayaAdd,
		AssetAmt: assetAmt,
		AssetAdd: assetAdd,
	}
}

// NewEventSwap create a new swap event
func NewEventSwap(pool common.Asset, swapTarget, fee, swapSlip, liquidityFeeInCacao cosmos.Uint, inTx common.Tx, emitAsset common.Coin, synthUnits cosmos.Uint) *EventSwap {
	return &EventSwap{
		Pool:                  pool,
		SwapTarget:            swapTarget,
		SwapSlip:              swapSlip,
		LiquidityFee:          fee,
		LiquidityFeeInCacao:   liquidityFeeInCacao,
		InTx:                  inTx,
		EmitAsset:             emitAsset,
		SynthUnits:            synthUnits,
		StreamingSwapQuantity: 0,
		StreamingSwapCount:    0,
	}
}

// Type return a string that represent the type, it should not duplicated with other event
func (m *EventSwap) Type() string {
	return SwapEventType
}

// Events convert EventSwap to key value pairs used in cosmos
func (m *EventSwap) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("pool", m.Pool.String()),
		cosmos.NewAttribute("swap_target", m.SwapTarget.String()),
		cosmos.NewAttribute("swap_slip", m.SwapSlip.String()),
		cosmos.NewAttribute("liquidity_fee", m.LiquidityFee.String()),
		cosmos.NewAttribute("liquidity_fee_in_cacao", m.LiquidityFeeInCacao.String()),
		cosmos.NewAttribute("emit_asset", m.EmitAsset.String()),
		cosmos.NewAttribute("streaming_swap_quantity", strconv.FormatUint(m.StreamingSwapQuantity, 10)),
		cosmos.NewAttribute("streaming_swap_count", strconv.FormatUint(m.StreamingSwapCount, 10)),
	)
	if !m.SynthUnits.IsZero() {
		evt = evt.AppendAttributes(cosmos.NewAttribute("synth_units", m.SynthUnits.String()))
	}
	evt = evt.AppendAttributes(m.InTx.ToAttributes()...)
	return cosmos.Events{evt}, nil
}

// NewEventStreamingSwap create a new streaming swap event
func NewEventStreamingSwap(inAsset, outAsset common.Asset, swp StreamingSwap) *EventStreamingSwap {
	return &EventStreamingSwap{
		TxID:              swp.TxID,
		Interval:          swp.Interval,
		Quantity:          swp.Quantity,
		Count:             swp.Count,
		LastHeight:        swp.LastHeight,
		Deposit:           common.NewCoin(inAsset, swp.Deposit),
		In:                common.NewCoin(inAsset, swp.In),
		Out:               common.NewCoin(outAsset, swp.Out),
		FailedSwaps:       swp.FailedSwaps,
		FailedSwapReasons: swp.FailedSwapReasons,
	}
}

// Type return a string that represent the type, it should not duplicated with other event
func (m *EventStreamingSwap) Type() string {
	return StreamingSwapEventType
}

// Events convert EventSwap to key value pairs used in cosmos
func (m *EventStreamingSwap) Events() (cosmos.Events, error) {
	failedSwaps := make([]string, len(m.FailedSwaps))
	for i, num := range m.FailedSwaps {
		failedSwaps[i] = strconv.FormatUint(num, 10)
	}

	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("tx_id", m.TxID.String()),
		cosmos.NewAttribute("interval", strconv.FormatUint(m.Interval, 10)),
		cosmos.NewAttribute("quantity", strconv.FormatUint(m.Quantity, 10)),
		cosmos.NewAttribute("count", strconv.FormatUint(m.Count, 10)),
		cosmos.NewAttribute("last_height", strconv.FormatInt(m.LastHeight, 10)),
		cosmos.NewAttribute("deposit", m.Deposit.String()),
		cosmos.NewAttribute("in", m.In.String()),
		cosmos.NewAttribute("out", m.Out.String()),
		cosmos.NewAttribute("failed_swaps", strings.Join(failedSwaps, ", ")),
		cosmos.NewAttribute("failed_swap_reasons", strings.Join(m.FailedSwapReasons, "\n ")),
	)
	return cosmos.Events{evt}, nil
}

func NewEventAffiliateFee(txId common.TxID, memo, mayaname string, cacaoAddress common.Address, asset common.Asset, feeBpsTick uint64, grossAmount, feeAmt cosmos.Uint, parent string, subFeeBps uint64) *EventAffiliateFee {
	return &EventAffiliateFee{
		TxID:         txId,
		Memo:         memo,
		Mayaname:     mayaname,
		CacaoAddress: cacaoAddress,
		Asset:        asset,
		GrossAmount:  grossAmount,
		FeeBpsTick:   feeBpsTick,
		FeeAmount:    feeAmt,
		Parent:       parent,
		SubFeeBps:    subFeeBps,
	}
}

func (m *EventAffiliateFee) Type() string {
	return AffiliateFeeEventType
}

func (m *EventAffiliateFee) Events() (cosmos.Events, error) {
	return cosmos.Events{
		cosmos.NewEvent(m.Type(),
			cosmos.NewAttribute("tx_id", m.TxID.String()),
			cosmos.NewAttribute("memo", m.Memo),
			cosmos.NewAttribute("mayaname", m.Mayaname),
			cosmos.NewAttribute("cacao_address", m.CacaoAddress.String()),
			cosmos.NewAttribute("asset", m.Asset.String()),
			cosmos.NewAttribute("gross_amount", m.GrossAmount.String()),
			cosmos.NewAttribute("fee_bps_tick", strconv.FormatUint(m.FeeBpsTick, 10)),
			cosmos.NewAttribute("fee_amount", m.FeeAmount.String()),
			cosmos.NewAttribute("parent", m.Parent),
			cosmos.NewAttribute("sub_fee_bps", strconv.FormatUint(m.SubFeeBps, 10)),
		),
	}, nil
}

// NewEventAddLiquidity create a new add liquidity event
func NewEventAddLiquidity(pool common.Asset,
	su cosmos.Uint,
	cacaoAddress common.Address,
	cacaoAmount,
	assetAmount cosmos.Uint,
	cacaoTxID,
	assetTxID common.TxID,
	assetAddress common.Address,
) *EventAddLiquidity {
	return &EventAddLiquidity{
		Pool:          pool,
		ProviderUnits: su,
		CacaoAddress:  cacaoAddress,
		CacaoAmount:   cacaoAmount,
		AssetAmount:   assetAmount,
		RuneTxID:      cacaoTxID,
		AssetTxID:     assetTxID,
		AssetAddress:  assetAddress,
	}
}

// Type return the event type
func (m *EventAddLiquidity) Type() string {
	return AddLiquidityEventType
}

// Events return cosmos.Events which is cosmos.Attribute(key value pairs)
func (m *EventAddLiquidity) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("pool", m.Pool.String()),
		cosmos.NewAttribute("liquidity_provider_units", m.ProviderUnits.String()),
		cosmos.NewAttribute("cacao_address", m.CacaoAddress.String()),
		cosmos.NewAttribute("cacao_amount", m.CacaoAmount.String()),
		cosmos.NewAttribute("asset_amount", m.AssetAmount.String()),
		cosmos.NewAttribute("asset_address", m.AssetAddress.String()),
	)
	if !m.RuneTxID.Equals(m.AssetTxID) && !m.RuneTxID.IsEmpty() {
		evt = evt.AppendAttributes(cosmos.NewAttribute(fmt.Sprintf("%s_txid", common.BaseAsset().Chain), m.RuneTxID.String()))
	}

	if !m.AssetTxID.IsEmpty() {
		evt = evt.AppendAttributes(cosmos.NewAttribute(fmt.Sprintf("%s_txid", m.Pool.Chain), m.AssetTxID.String()))
	}
	return cosmos.Events{
		evt,
	}, nil
}

// NewEventWithdraw create a new withdraw event
func NewEventWithdraw(pool common.Asset, su cosmos.Uint, basisPts int64, asym cosmos.Dec, inTx common.Tx, emitAsset, emitCacao, impLoss cosmos.Uint) *EventWithdraw {
	return &EventWithdraw{
		Pool:              pool,
		ProviderUnits:     su,
		BasisPoints:       basisPts,
		Asymmetry:         asym,
		InTx:              inTx,
		EmitAsset:         emitAsset,
		EmitCacao:         emitCacao,
		ImpLossProtection: impLoss,
	}
}

// Type return the withdraw event type
func (m *EventWithdraw) Type() string {
	return WithdrawEventType
}

// Events return the cosmos event
func (m *EventWithdraw) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("pool", m.Pool.String()),
		cosmos.NewAttribute("liquidity_provider_units", m.ProviderUnits.String()),
		cosmos.NewAttribute("basis_points", strconv.FormatInt(m.BasisPoints, 10)),
		cosmos.NewAttribute("asymmetry", m.Asymmetry.String()),
		cosmos.NewAttribute("emit_asset", m.EmitAsset.String()),
		cosmos.NewAttribute("emit_cacao", m.EmitCacao.String()),
		cosmos.NewAttribute("imp_loss_protection", m.ImpLossProtection.String()))
	evt = evt.AppendAttributes(m.InTx.ToAttributes()...)
	return cosmos.Events{evt}, nil
}

// NewEventDonate create a new donate event
func NewEventDonate(pool common.Asset, inTx common.Tx) *EventDonate {
	return &EventDonate{
		Pool: pool,
		InTx: inTx,
	}
}

// Type return donate event type
func (m *EventDonate) Type() string {
	return DonateEventType
}

// Events get all events
func (m *EventDonate) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("pool", m.Pool.String()))
	evt = evt.AppendAttributes(m.InTx.ToAttributes()...)
	return cosmos.Events{evt}, nil
}

// NewEventPool create a new pool change event
func NewEventPool(pool common.Asset, status PoolStatus) *EventPool {
	return &EventPool{
		Pool:   pool,
		Status: status,
	}
}

// Type return pool event type
func (m *EventPool) Type() string {
	return PoolEventType
}

// Events provide an instance of cosmos.Events
func (m *EventPool) Events() (cosmos.Events, error) {
	return cosmos.Events{
		cosmos.NewEvent(m.Type(),
			cosmos.NewAttribute("pool", m.Pool.String()),
			cosmos.NewAttribute("pool_status", m.Status.String())),
	}, nil
}

// NewEventRewards create a new reward event
func NewEventRewardsV1(bondReward cosmos.Uint, poolRewards []PoolAmt) *EventRewardsV1 {
	return &EventRewardsV1{
		BondReward:  bondReward,
		PoolRewards: poolRewards,
	}
}

// Type return reward event type
func (m *EventRewardsV1) Type() string {
	return RewardEventType
}

// Events return a standard cosmos event
func (m *EventRewardsV1) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("bond_reward", m.BondReward.String()),
	)
	for _, item := range m.PoolRewards {
		evt = evt.AppendAttributes(cosmos.NewAttribute(item.Asset.String(), strconv.FormatInt(item.Amount, 10)))
	}
	return cosmos.Events{evt}, nil
}

// NewEventRewards create a new reward event
func NewEventRewards(bondReward cosmos.Uint, poolRewards []PoolAmt, cacaoPoolReward, mayaFundReward cosmos.Uint) *EventRewards {
	return &EventRewards{
		BondReward:      bondReward,
		PoolRewards:     poolRewards,
		CacaoPoolReward: cacaoPoolReward,
		MayaFundReward:  mayaFundReward,
	}
}

// Type return reward event type
func (m *EventRewards) Type() string {
	return RewardEventType
}

// Events return a standard cosmos event
func (m *EventRewards) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("bond_reward", m.BondReward.String()),
	)
	for _, item := range m.PoolRewards {
		evt = evt.AppendAttributes(cosmos.NewAttribute(item.Asset.String(), strconv.FormatInt(item.Amount, 10)))
	}
	return cosmos.Events{evt}, nil
}

// NewEventRefund create a new EventRefund
func NewEventRefund(code uint32, reason string, inTx common.Tx, fee common.Fee) *EventRefund {
	return &EventRefund{
		Code:   code,
		Reason: reason,
		InTx:   inTx,
		Fee:    fee,
	}
}

// Type return reward event type
func (m *EventRefund) Type() string {
	return RefundEventType
}

// Events return events
func (m *EventRefund) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("code", strconv.FormatUint(uint64(m.Code), 10)),
		cosmos.NewAttribute("reason", m.Reason),
	)
	evt = evt.AppendAttributes(m.InTx.ToAttributes()...)
	return cosmos.Events{evt}, nil
}

// NewEventBond create a new Bond Events
func NewEventBond(amount cosmos.Uint, bondType BondType, txIn common.Tx) *EventBond {
	return &EventBond{
		Amount:   amount,
		BondType: bondType,
		TxIn:     txIn,
	}
}

// Type return bond event Type
func (m *EventBond) Type() string {
	return BondEventType
}

// Events return all the event attributes
func (m *EventBond) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("amount", m.Amount.String()),
		cosmos.NewAttribute("bond_type", string(m.BondType)))
	evt = evt.AppendAttributes(m.TxIn.ToAttributes()...)
	return cosmos.Events{evt}, nil
}

// NewEventGas create a new EventGas instance
func NewEventGas() *EventGas {
	return &EventGas{
		Pools: make([]GasPool, 0),
	}
}

// UpsertGasPool update the Gas Pools hold by EventGas instance
// if the given gasPool already exist, then it merge the gasPool with internal one , otherwise add it to the list
func (m *EventGas) UpsertGasPool(pool GasPool) {
	for i, p := range m.Pools {
		if p.Asset == pool.Asset {
			m.Pools[i].CacaoAmt = p.CacaoAmt.Add(pool.CacaoAmt)
			m.Pools[i].AssetAmt = p.AssetAmt.Add(pool.AssetAmt)
			return
		}
	}
	m.Pools = append(m.Pools, pool)
}

// Type return event type
func (m *EventGas) Type() string {
	return GasEventType
}

// Events return a standard cosmos events
func (m *EventGas) Events() (cosmos.Events, error) {
	events := make(cosmos.Events, 0, len(m.Pools))
	for _, item := range m.Pools {
		evt := cosmos.NewEvent(m.Type(),
			cosmos.NewAttribute("asset", item.Asset.String()),
			cosmos.NewAttribute("asset_amt", item.AssetAmt.String()),
			cosmos.NewAttribute("cacao_amt", item.CacaoAmt.String()),
			cosmos.NewAttribute("transaction_count", strconv.FormatInt(item.Count, 10)))
		events = append(events, evt)
	}
	return events, nil
}

// NewEventReserve create a new instance of EventReserve
func NewEventReserve(contributor ReserveContributor, inTx common.Tx) *EventReserve {
	return &EventReserve{
		ReserveContributor: contributor,
		InTx:               inTx,
	}
}

// Type return the event Type
func (m *EventReserve) Type() string {
	return ReserveEventType
}

// Events return standard cosmos event
func (m *EventReserve) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("contributor_address", m.ReserveContributor.Address.String()),
		cosmos.NewAttribute("amount", m.ReserveContributor.Amount.String()),
	)
	evt = evt.AppendAttributes(m.InTx.ToAttributes()...)
	return cosmos.Events{
		evt,
	}, nil
}

// NewEventScheduledOutbound creates a new scheduled outbound event.
func NewEventScheduledOutbound(tx TxOutItem) *EventScheduledOutbound {
	return &EventScheduledOutbound{
		OutTx: tx,
	}
}

// Type returns the scheduled outbound event type.
func (m *EventScheduledOutbound) Type() string {
	return ScheduledOutboundEventType
}

// Events returns the cosmos events for the scheduled outbound event.
func (m *EventScheduledOutbound) Events() (cosmos.Events, error) {
	attrs := []cosmos.Attribute{
		cosmos.NewAttribute("chain", m.OutTx.Chain.String()),
		cosmos.NewAttribute("to_address", m.OutTx.ToAddress.String()),
		cosmos.NewAttribute("vault_pub_key", m.OutTx.VaultPubKey.String()),
		cosmos.NewAttribute("coin_asset", m.OutTx.Coin.Asset.String()),
		cosmos.NewAttribute("coin_amount", m.OutTx.Coin.Amount.String()),
		cosmos.NewAttribute("coin_decimals", strconv.FormatInt(m.OutTx.Coin.Decimals, 10)),
		cosmos.NewAttribute("memo", m.OutTx.Memo),
		cosmos.NewAttribute("gas_rate", strconv.FormatInt(m.OutTx.GasRate, 10)),
		cosmos.NewAttribute("in_hash", m.OutTx.InHash.String()),
		cosmos.NewAttribute("out_hash", m.OutTx.OutHash.String()),
		cosmos.NewAttribute("module_name", m.OutTx.ModuleName),
	}

	for i, gas := range m.OutTx.MaxGas {
		attrs = append(attrs, cosmos.NewAttribute(fmt.Sprintf("max_gas_asset_%d", i), gas.Asset.String()))
		attrs = append(attrs, cosmos.NewAttribute(fmt.Sprintf("max_gas_amount_%d", i), gas.Amount.String()))
		attrs = append(attrs, cosmos.NewAttribute(fmt.Sprintf("max_gas_decimals_%d", i), strconv.FormatInt(gas.Decimals, 10)))
	}

	return cosmos.Events{cosmos.NewEvent(m.Type(), attrs...)}, nil
}

// NewEventSecurity creates a new security event.
func NewEventSecurity(tx common.Tx, msg string) *EventSecurity {
	return &EventSecurity{
		Msg: msg,
		Tx:  tx,
	}
}

// Type returns the security event type.
func (m *EventSecurity) Type() string {
	return SecurityEventType
}

// Events returns the cosmos events for the security event.
func (m *EventSecurity) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(), cosmos.NewAttribute("msg", m.Msg))
	evt = evt.AppendAttributes(m.Tx.ToAttributes()...)
	return cosmos.Events{evt}, nil
}

// NewEventSlash create a new slash event
func NewEventSlash(pool common.Asset, slash []PoolAmt) *EventSlash {
	return &EventSlash{
		Pool:        pool,
		SlashAmount: slash,
	}
}

// Type return slash event type
func (m *EventSlash) Type() string {
	return SlashEventType
}

// Events return a standard cosmos events
func (m *EventSlash) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("pool", m.Pool.String()))
	for _, item := range m.SlashAmount {
		evt = evt.AppendAttributes(cosmos.NewAttribute(item.Asset.String(), strconv.FormatInt(item.Amount, 10)))
	}
	return cosmos.Events{evt}, nil
}

// NewEventSlashLiquidity create a new slash event
func NewEventSlashLiquidity(na cosmos.AccAddress, asset common.Asset, address common.Address, lpUnits cosmos.Uint) *EventSlashLiquidity {
	return &EventSlashLiquidity{
		NodeBondAddress: na,
		Asset:           asset,
		Address:         address,
		LpUnits:         lpUnits,
	}
}

// Type return slash event type
func (m *EventSlashLiquidity) Type() string {
	return SlashLiquidityEventType
}

// Events return a standard cosmos events
func (m *EventSlashLiquidity) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("bond_address", m.NodeBondAddress.String()),
		cosmos.NewAttribute("lp_address", m.Address.String()),
		cosmos.NewAttribute("asset", m.Asset.String()),
		cosmos.NewAttribute("lp_units", m.LpUnits.String()),
	)
	return cosmos.Events{evt}, nil
}

// NewEventErrata create a new errata event
func NewEventErrata(txID common.TxID, pools PoolMods) *EventErrata {
	return &EventErrata{
		TxID:  txID,
		Pools: pools,
	}
}

// Type return slash event type
func (m *EventErrata) Type() string {
	return ErrataEventType
}

// Events return a cosmos.Events type
func (m *EventErrata) Events() (cosmos.Events, error) {
	events := make(cosmos.Events, 0, len(m.Pools))
	for _, item := range m.Pools {
		evt := cosmos.NewEvent(m.Type(),
			cosmos.NewAttribute("in_tx_id", m.TxID.String()),
			cosmos.NewAttribute("asset", item.Asset.String()),
			cosmos.NewAttribute("cacao_amt", item.CacaoAmt.String()),
			cosmos.NewAttribute("cacao_add", strconv.FormatBool(item.CacaoAdd)),
			cosmos.NewAttribute("asset_amt", item.AssetAmt.String()),
			cosmos.NewAttribute("asset_add", strconv.FormatBool(item.AssetAdd)))
		events = append(events, evt)
	}
	return events, nil
}

// NewEventFee create a new EventFee
func NewEventFee(txID common.TxID, fee common.Fee, synthUnits cosmos.Uint) *EventFee {
	return &EventFee{
		TxID:       txID,
		Fee:        fee,
		SynthUnits: synthUnits,
	}
}

// Type get a string represent the event type
func (m *EventFee) Type() string {
	return FeeEventType
}

// Events return events of cosmos.Event type
func (m *EventFee) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("tx_id", m.TxID.String()),
		cosmos.NewAttribute("coins", m.Fee.Coins.String()),
		cosmos.NewAttribute("pool_deduct", m.Fee.PoolDeduct.String()))
	if !m.SynthUnits.IsZero() {
		evt = evt.AppendAttributes(
			cosmos.NewAttribute("synth_units", m.SynthUnits.String()),
		)
	}
	return cosmos.Events{evt}, nil
}

// NewEventOutbound create a new instance of EventOutbound
func NewEventOutbound(inTxID common.TxID, tx common.Tx) *EventOutbound {
	return &EventOutbound{
		InTxID: inTxID,
		Tx:     tx,
	}
}

// Type return a string which represent the type of this event
func (m *EventOutbound) Type() string {
	return OutboundEventType
}

// Events return sdk events
func (m *EventOutbound) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("in_tx_id", m.InTxID.String()))
	evt = evt.AppendAttributes(m.Tx.ToAttributes()...)
	return cosmos.Events{evt}, nil
}

// NewEventTssKeygenSuccess create a new EventTssKeygenSuccess
func NewEventTssKeygenSuccess(pubkey common.PubKey, height int64, members []string) *EventTssKeygenSuccess {
	return &EventTssKeygenSuccess{
		PubKey:  pubkey,
		Height:  height,
		Members: members,
	}
}

// Type  return a string which represent the type of this event
func (m *EventTssKeygenSuccess) Type() string {
	return TSSKeygenSuccess
}

// Events return cosmos sdk events
func (m *EventTssKeygenSuccess) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("pubkey", m.PubKey.String()),
		cosmos.NewAttribute("height", strconv.FormatInt(m.Height, 10)),
		cosmos.NewAttribute("members", strings.Join(m.Members, ", ")),
	)
	return cosmos.Events{evt}, nil
}

// NewEventTssKeygenFailure create a new EventTssKeygenFailure
func NewEventTssKeygenFailure(reason, round string, unicast bool, height int64, blame []string) *EventTssKeygenFailure {
	return &EventTssKeygenFailure{
		FailReason: reason,
		IsUnicast:  unicast,
		Round:      round,
		Height:     height,
		BlameNodes: blame,
	}
}

// Type  return a string which represent the type of this event
func (m *EventTssKeygenFailure) Type() string {
	return TSSKeygenFailure
}

// Events return cosmos sdk events
func (m *EventTssKeygenFailure) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("reason", m.FailReason),
		cosmos.NewAttribute("round", m.Round),
		cosmos.NewAttribute("is_unicast", fmt.Sprintf("%v", m.IsUnicast)),
		cosmos.NewAttribute("height", strconv.FormatInt(m.Height, 10)),
		cosmos.NewAttribute("blame", strings.Join(m.BlameNodes, ", ")),
	)
	return cosmos.Events{evt}, nil
}

// NewEventTssKeygenMetric create a new EventTssMetric
func NewEventTssKeygenMetric(pubkey common.PubKey, medianDurationMS int64) *EventTssKeygenMetric {
	return &EventTssKeygenMetric{
		PubKey:           pubkey,
		MedianDurationMs: medianDurationMS,
	}
}

// Type  return a string which represent the type of this event
func (m *EventTssKeygenMetric) Type() string {
	return TSSKeygenMetricEventType
}

// Events return cosmos sdk events
func (m *EventTssKeygenMetric) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("pubkey", m.PubKey.String()),
		cosmos.NewAttribute("median_duration_ms", strconv.FormatInt(m.MedianDurationMs, 10)))
	return cosmos.Events{evt}, nil
}

// NewEventTssKeysignMetric create a new EventTssMetric
func NewEventTssKeysignMetric(txID common.TxID, medianDurationMS int64) *EventTssKeysignMetric {
	return &EventTssKeysignMetric{
		TxID:             txID,
		MedianDurationMs: medianDurationMS,
	}
}

// Type  return a string which represent the type of this event
func (m *EventTssKeysignMetric) Type() string {
	return TSSKeysignMetricEventType
}

// Events return cosmos sdk events
func (m *EventTssKeysignMetric) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("txid", m.TxID.String()),
		cosmos.NewAttribute("median_duration_ms", strconv.FormatInt(m.MedianDurationMs, 10)))
	return cosmos.Events{evt}, nil
}

// NewEventSlashPoint create a new slash point event
func NewEventSlashPoint(addr cosmos.AccAddress, slashPoints int64, reason string) *EventSlashPoint {
	return &EventSlashPoint{
		NodeAddress: addr,
		SlashPoints: slashPoints,
		Reason:      reason,
	}
}

// Type return a string which represent the type of this event
func (m *EventSlashPoint) Type() string {
	return SlashPointEventType
}

// Events return cosmos sdk events
func (m *EventSlashPoint) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("node_address", m.NodeAddress.String()),
		cosmos.NewAttribute("slash_points", strconv.FormatInt(m.SlashPoints, 10)),
		cosmos.NewAttribute("reason", m.Reason))
	return cosmos.Events{evt}, nil
}

// NewEventPoolBalanceChanged create a new instance of EventPoolBalanceChanged
func NewEventPoolBalanceChanged(poolMod PoolMod, reason string) *EventPoolBalanceChanged {
	return &EventPoolBalanceChanged{
		PoolChange: poolMod,
		Reason:     reason,
	}
}

// Type return a string which represent the type of this event
func (m *EventPoolBalanceChanged) Type() string {
	return PoolBalanceChangeEventType
}

// Events return cosmos sdk events
func (m *EventPoolBalanceChanged) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("asset", m.PoolChange.Asset.String()),
		cosmos.NewAttribute("cacao_amt", m.PoolChange.CacaoAmt.String()),
		cosmos.NewAttribute("cacao_add", strconv.FormatBool(m.PoolChange.CacaoAdd)),
		cosmos.NewAttribute("asset_amt", m.PoolChange.AssetAmt.String()),
		cosmos.NewAttribute("asset_add", strconv.FormatBool(m.PoolChange.AssetAdd)),
		cosmos.NewAttribute("reason", m.GetReason()))
	return cosmos.Events{evt}, nil
}

// NewEventSwitch create a new instance of EventSwitch
func NewEventSwitch(from common.Address, to cosmos.AccAddress, coin common.Coin, hash common.TxID) *EventSwitch {
	return &EventSwitch{
		TxID:        hash,
		ToAddress:   to,
		FromAddress: from,
		Burn:        coin,
	}
}

// Type return a string which represent the type of this event
func (m *EventSwitch) Type() string {
	return SwitchEventType
}

// Events return cosmos sdk events
func (m *EventSwitch) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("txid", m.TxID.String()),
		cosmos.NewAttribute("from", m.FromAddress.String()),
		cosmos.NewAttribute("to", m.ToAddress.String()),
		cosmos.NewAttribute("burn", m.Burn.String()))
	return cosmos.Events{evt}, nil
}

// NewEventPendingLiquidity create a new add pending liquidity event
func NewEventPendingLiquidity(pool common.Asset,
	ptype PendingLiquidityType,
	mayaAddress common.Address,
	mayaAmount cosmos.Uint,
	assetAddress common.Address,
	assetAmount cosmos.Uint,
	cacaoTxID,
	assetTxID common.TxID,
) *EventPendingLiquidity {
	return &EventPendingLiquidity{
		Pool:         pool,
		PendingType:  ptype,
		CacaoAddress: mayaAddress,
		CacaoAmount:  mayaAmount,
		AssetAddress: assetAddress,
		AssetAmount:  assetAmount,
		RuneTxID:     cacaoTxID,
		AssetTxID:    assetTxID,
	}
}

// Type return the event type
func (m *EventPendingLiquidity) Type() string {
	return PendingLiquidity
}

// Events return cosmos.Events which is cosmos.Attribute(key value pairs)
func (m *EventPendingLiquidity) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("pool", m.Pool.String()),
		cosmos.NewAttribute("type", m.PendingType.String()),
		cosmos.NewAttribute("cacao_address", m.CacaoAddress.String()),
		cosmos.NewAttribute("cacao_amount", m.CacaoAmount.String()),
		cosmos.NewAttribute("asset_amount", m.AssetAmount.String()),
		cosmos.NewAttribute("asset_address", m.AssetAddress.String()),
	)
	if !m.RuneTxID.Equals(m.AssetTxID) && !m.RuneTxID.IsEmpty() {
		evt = evt.AppendAttributes(cosmos.NewAttribute(fmt.Sprintf("%s_txid", common.BaseAsset().Chain), m.RuneTxID.String()))
	}

	if !m.AssetTxID.IsEmpty() {
		evt = evt.AppendAttributes(cosmos.NewAttribute(fmt.Sprintf("%s_txid", m.Pool.Chain), m.AssetTxID.String()))
	}
	return cosmos.Events{
		evt,
	}, nil
}

// NewEventMAYAName create a new instance of EventMAYAName
func NewEventMAYAName(name string, chain common.Chain, addr common.Address, reg_fee, fund_amt cosmos.Uint, expire int64, owner cosmos.AccAddress, affiliate_bps int64, subaffiliate_name []string, subaffiliate_bps []cosmos.Uint) *EventMAYAName {
	return &EventMAYAName{
		Name:             name,
		Chain:            chain,
		Address:          addr,
		RegistrationFee:  reg_fee,
		FundAmt:          fund_amt,
		Expire:           expire,
		Owner:            owner,
		AffiliateBps:     affiliate_bps,
		SubaffiliateName: subaffiliate_name,
		SubaffiliateBps:  subaffiliate_bps,
	}
}

// Type return a string which represent the type of this event
func (m *EventMAYAName) Type() string {
	return MAYANameEventType
}

// Events return cosmos sdk events
func (m *EventMAYAName) Events() (cosmos.Events, error) {
	subaffiliatesBps := make([]string, len(m.SubaffiliateBps))
	for i, num := range m.SubaffiliateBps {
		subaffiliatesBps[i] = num.String()
	}
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("name", strings.ToLower(m.Name)),
		cosmos.NewAttribute("chain", m.Chain.String()),
		cosmos.NewAttribute("address", m.Address.String()),
		cosmos.NewAttribute("registration_fee", m.RegistrationFee.String()),
		cosmos.NewAttribute("fund_amount", m.FundAmt.String()),
		cosmos.NewAttribute("expire", fmt.Sprintf("%d", m.Expire)),
		cosmos.NewAttribute("owner", m.Owner.String()),
		cosmos.NewAttribute("affiliate_bps", fmt.Sprintf("%d", m.AffiliateBps)),
		cosmos.NewAttribute("subaffiliate_name", strings.Join(m.SubaffiliateName, "/")),
		cosmos.NewAttribute("subaffiliate_bps", strings.Join(subaffiliatesBps, "/")),
	)
	return cosmos.Events{evt}, nil
}

// NewEventCACAOPoolWithdrawV118 create a new CACAOPool withdraw event
func NewEventCACAOPoolWithdrawV118(
	cacaoAddress cosmos.AccAddress,
	basisPts int64,
	cacaoAmount cosmos.Uint,
	units cosmos.Uint,
	txID common.TxID,
	affAddr common.Address,
	affBps int64,
	affAmt cosmos.Uint,
) *EventCACAOPoolWithdrawV118 {
	return &EventCACAOPoolWithdrawV118{
		CacaoAddress:      cacaoAddress,
		BasisPoints:       basisPts,
		CacaoAmount:       cacaoAmount,
		Units:             units,
		TxId:              txID,
		AffiliateAddress:  affAddr,
		AffiliateBasisPts: affBps,
		AffiliateAmount:   affAmt,
	}
}

// Type return the withdraw event type
func (m *EventCACAOPoolWithdrawV118) Type() string {
	return CACAOPoolWithdrawEventType
}

// Events return the cosmos event
func (m *EventCACAOPoolWithdrawV118) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("cacao_address", m.CacaoAddress.String()),
		cosmos.NewAttribute("basis_points", strconv.FormatInt(m.BasisPoints, 10)),
		cosmos.NewAttribute("cacao_amount", m.CacaoAmount.String()),
		cosmos.NewAttribute("units", m.Units.String()),
		cosmos.NewAttribute("tx_id", m.TxId.String()),
		cosmos.NewAttribute("affiliate_address", m.AffiliateAddress.String()),
		cosmos.NewAttribute("affiliate_basis_points", strconv.FormatInt(m.AffiliateBasisPts, 10)),
		cosmos.NewAttribute("affiliate_amount", m.AffiliateAmount.String()))
	return cosmos.Events{evt}, nil
}

// NewEventCACAOPoolWithdraw create a new CACAOPool withdraw event
func NewEventCACAOPoolWithdraw(
	cacaoAddress cosmos.AccAddress,
	basisPts int64,
	cacaoAmount cosmos.Uint,
	units cosmos.Uint,
	txID common.TxID,
	affBps int64,
	affAmt cosmos.Uint,
) *EventCACAOPoolWithdraw {
	return &EventCACAOPoolWithdraw{
		CacaoAddress:      cacaoAddress,
		BasisPoints:       basisPts,
		CacaoAmount:       cacaoAmount,
		Units:             units,
		TxId:              txID,
		AffiliateBasisPts: affBps,
		AffiliateAmount:   affAmt,
	}
}

// Type return the withdraw event type
func (m *EventCACAOPoolWithdraw) Type() string {
	return CACAOPoolWithdrawEventType
}

// Events return the cosmos event
func (m *EventCACAOPoolWithdraw) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("cacao_address", m.CacaoAddress.String()),
		cosmos.NewAttribute("basis_points", strconv.FormatInt(m.BasisPoints, 10)),
		cosmos.NewAttribute("cacao_amount", m.CacaoAmount.String()),
		cosmos.NewAttribute("units", m.Units.String()),
		cosmos.NewAttribute("tx_id", m.TxId.String()),
		cosmos.NewAttribute("affiliate_basis_points", strconv.FormatInt(m.AffiliateBasisPts, 10)),
		cosmos.NewAttribute("affiliate_amount", m.AffiliateAmount.String()))
	return cosmos.Events{evt}, nil
}

// NewEventCACAOPoolDeposit create a new CACAOPool deposit event
func NewEventCACAOPoolDeposit(
	cacaoAddress cosmos.AccAddress,
	cacaoAmount cosmos.Uint,
	units cosmos.Uint,
	txid common.TxID,
) *EventCACAOPoolDeposit {
	return &EventCACAOPoolDeposit{
		CacaoAddress: cacaoAddress,
		CacaoAmount:  cacaoAmount,
		Units:        units,
		TxId:         txid,
	}
}

// Type return the deposit event type
func (m *EventCACAOPoolDeposit) Type() string {
	return CACAOPoolDepositEventType
}

// Events return the cosmos event
func (m *EventCACAOPoolDeposit) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("cacao_address", m.CacaoAddress.String()),
		cosmos.NewAttribute("cacao_amount", m.CacaoAmount.String()),
		cosmos.NewAttribute("units", m.Units.String()),
		cosmos.NewAttribute("tx_id", m.TxId.String()),
	)
	return cosmos.Events{evt}, nil
}

// NewEventWithdraw create a new withdraw event
func NewEventTradeAccountDeposit(
	amt cosmos.Uint,
	asset common.Asset,
	assetAddress common.Address,
	cacaoAddress common.Address,
	txID common.TxID,
) *EventTradeAccountDeposit {
	return &EventTradeAccountDeposit{
		Amount:       amt,
		Asset:        asset,
		AssetAddress: assetAddress,
		CacaoAddress: cacaoAddress,
		TxID:         txID,
	}
}

// Type return the withdraw event type
func (m *EventTradeAccountDeposit) Type() string {
	return TradeAccountDepositEventType
}

// Events return the cosmos event
func (m *EventTradeAccountDeposit) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("amount", m.Amount.String()),
		cosmos.NewAttribute("asset", m.Asset.String()),
		cosmos.NewAttribute("cacao_address", m.CacaoAddress.String()),
		cosmos.NewAttribute("asset_address", m.AssetAddress.String()),
		cosmos.NewAttribute("tx_id", m.TxID.String()))
	return cosmos.Events{evt}, nil
}

// NewEventWithdraw create a new withdraw event
func NewEventTradeAccountWithdraw(
	amt cosmos.Uint,
	asset common.Asset,
	assetAddress common.Address,
	mayaAddress common.Address,
	txID common.TxID,
) *EventTradeAccountWithdraw {
	return &EventTradeAccountWithdraw{
		Amount:       amt,
		Asset:        asset,
		AssetAddress: assetAddress,
		CacaoAddress: mayaAddress,
		TxID:         txID,
	}
}

// Type return the withdraw event type
func (m *EventTradeAccountWithdraw) Type() string {
	return TradeAccountWithdrawEventType
}

// Events return the cosmos event
func (m *EventTradeAccountWithdraw) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("amount", m.Amount.String()),
		cosmos.NewAttribute("asset", m.Asset.String()),
		cosmos.NewAttribute("cacao_address", m.CacaoAddress.String()),
		cosmos.NewAttribute("asset_address", m.AssetAddress.String()),
		cosmos.NewAttribute("tx_id", m.TxID.String()))
	return cosmos.Events{evt}, nil
}

// NewEventSetMimir create a new instance of EventSetMimir
func NewEventSetMimir(key, value string) *EventSetMimir {
	return &EventSetMimir{
		Key:   key,
		Value: value,
	}
}

// Type return a string which represent the type of this event
func (m *EventSetMimir) Type() string {
	return SetMimirEventType
}

// Events return cosmos sdk events
func (m *EventSetMimir) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("key", m.Key),
		cosmos.NewAttribute("value", m.Value),
	)
	return cosmos.Events{evt}, nil
}

// NewEventSetNodeMimir create a new instance of EventSetNodeMimir
func NewEventSetNodeMimir(key, value, address string) *EventSetNodeMimir {
	return &EventSetNodeMimir{
		Key:     key,
		Value:   value,
		Address: address,
	}
}

// Type return a string which represent the type of this event
func (m *EventSetNodeMimir) Type() string {
	return SetNodeMimirEventType
}

// Events return cosmos sdk events
func (m *EventSetNodeMimir) Events() (cosmos.Events, error) {
	evt := cosmos.NewEvent(m.Type(),
		cosmos.NewAttribute("key", m.Key),
		cosmos.NewAttribute("value", m.Value),
		cosmos.NewAttribute("address", m.Address),
	)
	return cosmos.Events{evt}, nil
}
