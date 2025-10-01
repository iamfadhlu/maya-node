package mayachain

import (
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

type NetworkMgrDummy struct {
	nas   NodeAccounts
	vault Vault
}

func NewNetworkMgrDummy() *NetworkMgrDummy {
	return &NetworkMgrDummy{}
}

func (vm *NetworkMgrDummy) EndBlock(ctx cosmos.Context, mgr Manager) error {
	return nil
}

func (vm *NetworkMgrDummy) TriggerKeygen(_ cosmos.Context, nas NodeAccounts) error {
	vm.nas = nas
	return nil
}

func (vm *NetworkMgrDummy) RotateVault(ctx cosmos.Context, vault Vault) error {
	vm.vault = vault
	return nil
}

func (vm *NetworkMgrDummy) UpdateNetwork(ctx cosmos.Context, constAccessor constants.ConstantValues, gasManager GasManager, eventMgr EventManager) error {
	return nil
}

func (vm *NetworkMgrDummy) RecallChainFunds(ctx cosmos.Context, chain common.Chain, mgr Manager, excludeNodeKeys common.PubKeys) error {
	return nil
}
