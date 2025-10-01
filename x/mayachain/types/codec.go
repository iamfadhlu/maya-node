package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"

	"gitlab.com/mayachain/mayanode/common/cosmos"
)

var (
	amino     = codec.NewLegacyAmino()
	ModuleCdc = codec.NewAminoCodec(amino)
)

func init() {
	RegisterCodec(amino)
}

// RegisterCodec register the msg types for amino
func RegisterCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgSwap{}, "mayachain/Swap", nil)
	cdc.RegisterConcrete(&MsgTssPool{}, "mayachain/TssPool", nil)
	cdc.RegisterConcrete(&MsgTssKeysignFail{}, "mayachain/TssKeysignFail", nil)
	cdc.RegisterConcrete(&MsgAddLiquidity{}, "mayachain/AddLiquidity", nil)
	cdc.RegisterConcrete(&MsgWithdrawLiquidity{}, "mayachain/WidthdrawLiquidity", nil)
	cdc.RegisterConcrete(&MsgObservedTxIn{}, "mayachain/ObservedTxIn", nil)
	cdc.RegisterConcrete(&MsgObservedTxOut{}, "mayachain/ObservedTxOut", nil)
	cdc.RegisterConcrete(&MsgDonate{}, "mayachain/MsgDonate", nil)
	cdc.RegisterConcrete(&MsgBond{}, "mayachain/MsgBond", nil)
	cdc.RegisterConcrete(&MsgUnBond{}, "mayachain/MsgUnBond", nil)
	cdc.RegisterConcrete(&MsgLeave{}, "mayachain/MsgLeave", nil)
	cdc.RegisterConcrete(&MsgNoOp{}, "mayachain/MsgNoOp", nil)
	cdc.RegisterConcrete(&MsgOutboundTx{}, "mayachain/MsgOutboundTx", nil)
	cdc.RegisterConcrete(&MsgSetVersion{}, "mayachain/MsgSetVersion", nil)
	cdc.RegisterConcrete(&MsgSetNodeKeys{}, "mayachain/MsgSetNodeKeys", nil)
	cdc.RegisterConcrete(&MsgSetAztecAddress{}, "mayachain/MsgSetAztecAddress", nil)
	cdc.RegisterConcrete(&MsgSetIPAddress{}, "mayachain/MsgSetIPAddress", nil)
	cdc.RegisterConcrete(&MsgYggdrasil{}, "mayachain/MsgYggdrasil", nil)
	cdc.RegisterConcrete(&MsgReserveContributor{}, "mayachain/MsgReserveContributor", nil)
	cdc.RegisterConcrete(&MsgErrataTx{}, "mayachain/MsgErrataTx", nil)
	cdc.RegisterConcrete(&MsgBan{}, "mayachain/MsgBan", nil)
	cdc.RegisterConcrete(&MsgMimir{}, "mayachain/MsgMimir", nil)
	cdc.RegisterConcrete(&MsgDeposit{}, "mayachain/MsgDeposit", nil)
	cdc.RegisterConcrete(&MsgNetworkFee{}, "mayachain/MsgNetworkFee", nil)
	cdc.RegisterConcrete(&MsgMigrate{}, "mayachain/MsgMigrate", nil)
	cdc.RegisterConcrete(&MsgRagnarok{}, "mayachain/MsgRagnarok", nil)
	cdc.RegisterConcrete(&MsgRefundTx{}, "mayachain/MsgRefundTx", nil)
	cdc.RegisterConcrete(&MsgSend{}, "mayachain/MsgSend", nil)
	cdc.RegisterConcrete(&MsgNodePauseChain{}, "mayachain/MsgNodePauseChain", nil)
	cdc.RegisterConcrete(&MsgSolvency{}, "mayachain/MsgSolvency", nil)
	cdc.RegisterConcrete(&MsgManageMAYAName{}, "mayachain/MsgManageMAYAName", nil)
	cdc.RegisterConcrete(&MsgTradeAccountDeposit{}, "mayachain/MsgTradeAccountDeposit", nil)
	cdc.RegisterConcrete(&MsgTradeAccountWithdrawal{}, "mayachain/MsgTradeAccountWithdrawal", nil)
}

// RegisterInterfaces register the types
func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgSwap{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgTssPool{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgTssKeysignFail{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgAddLiquidity{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgWithdrawLiquidity{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgObservedTxIn{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgObservedTxOut{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgDonate{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgBond{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgUnBond{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgLeave{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgNoOp{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgOutboundTx{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgSetVersion{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgSetNodeKeys{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgSetAztecAddress{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgSetIPAddress{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgYggdrasil{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgReserveContributor{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgErrataTx{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgBan{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgMimir{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgDeposit{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgNetworkFee{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgMigrate{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgRagnarok{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgRefundTx{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgSend{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgNodePauseChain{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgManageMAYAName{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgSolvency{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgTradeAccountDeposit{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgTradeAccountWithdrawal{})
}
