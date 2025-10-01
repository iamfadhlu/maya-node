package mayachain

import (
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

type DummyGasManager struct{}

func NewDummyGasManager() *DummyGasManager {
	return &DummyGasManager{}
}

func (m *DummyGasManager) BeginBlock(mgr Manager) {}
func (m *DummyGasManager) EndBlock(ctx cosmos.Context, keeper keeper.Keeper, eventManager EventManager) {
}
func (m *DummyGasManager) AddGasAsset(gas common.Gas, increaseTxCount bool)    {}
func (m *DummyGasManager) SubGas(gas common.Gas)                               {}
func (m *DummyGasManager) AddGas(gas common.Gas)                               {}
func (m *DummyGasManager) GetGas() common.Gas                                  { return nil }
func (m *DummyGasManager) ProcessGas(ctx cosmos.Context, keeper keeper.Keeper) {}
func (m *DummyGasManager) GetFee(ctx cosmos.Context, chain common.Chain, _ common.Asset) cosmos.Uint {
	return cosmos.ZeroUint()
}

func (gm *DummyGasManager) GetNetworkFee(ctx cosmos.Context, chain common.Chain) (NetworkFee, error) {
	return types.NetworkFee{}, nil
}

func (m *DummyGasManager) GetMaxGas(ctx cosmos.Context, chain common.Chain) (common.Coin, error) {
	if chain.Equals(common.BNBChain) {
		return common.NewCoin(common.BNBAsset, bnbSingleTxFee), nil
	}
	if chain.Equals(common.BTCChain) {
		return common.NewCoin(common.BTCAsset, cosmos.NewUint(1000)), nil
	}
	return common.NoCoin, errKaboom
}

func (m *DummyGasManager) GetGasRate(ctx cosmos.Context, chain common.Chain) cosmos.Uint {
	return cosmos.OneUint()
}

func (m *DummyGasManager) CalcOutboundFeeMultiplier(ctx cosmos.Context, targetSurplusRune, gasSpentRune, gasWithheldRune, maxMultiplier, minMultiplier cosmos.Uint) cosmos.Uint {
	return cosmos.ZeroUint()
}
