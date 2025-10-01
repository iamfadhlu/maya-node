package mayachain

import (
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
)

type DummyTradeAccountManager struct{}

func NewDummyTradeAccountManager() *DummyTradeAccountManager {
	return &DummyTradeAccountManager{}
}

func (d DummyTradeAccountManager) EndBlock(ctx cosmos.Context, keeper keeper.Keeper) error {
	return nil
}

func (d DummyTradeAccountManager) BalanceOf(_ cosmos.Context, _ common.Asset, _ cosmos.AccAddress) cosmos.Uint {
	return cosmos.ZeroUint()
}

func (d DummyTradeAccountManager) Deposit(ctx cosmos.Context, asset common.Asset, amount cosmos.Uint, owner cosmos.AccAddress, assetAddr common.Address, _ common.TxID) (cosmos.Uint, error) {
	return cosmos.ZeroUint(), nil
}

func (d DummyTradeAccountManager) Withdrawal(ctx cosmos.Context, asset common.Asset, amount cosmos.Uint, owner cosmos.AccAddress, assetAddr common.Address, _ common.TxID) (cosmos.Uint, error) {
	return cosmos.ZeroUint(), nil
}
