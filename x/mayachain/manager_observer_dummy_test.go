package mayachain

import (
	"gitlab.com/mayachain/mayanode/common"
	cosmos "gitlab.com/mayachain/mayanode/common/cosmos"
	keeper "gitlab.com/mayachain/mayanode/x/mayachain/keeper"
)

type DummyObserverManager struct{}

func NewDummyObserverManager() *DummyObserverManager {
	return &DummyObserverManager{}
}

func (m *DummyObserverManager) BeginBlock()                                                  {}
func (m *DummyObserverManager) EndBlock(ctx cosmos.Context, keeper keeper.Keeper)            {}
func (m *DummyObserverManager) AppendObserver(chain common.Chain, addrs []cosmos.AccAddress) {}
func (m *DummyObserverManager) List() []cosmos.AccAddress                                    { return nil }
