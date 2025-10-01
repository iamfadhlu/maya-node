package mayachain

import (
	"errors"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
)

type TestDoubleSlashKeeper struct {
	keeper.KVStoreDummy
	na          NodeAccount
	naBond      cosmos.Uint
	bp          BondProviders
	lp          LiquidityProvider
	network     Network
	slashPoints map[string]int64
	mimir       map[string]int64
	modules     map[string]int64
}

func (k *TestDoubleSlashKeeper) SendFromModuleToModule(_ cosmos.Context, from, to string, coins common.Coins) error {
	k.modules[from] -= int64(coins[0].Amount.Uint64())
	k.modules[to] += int64(coins[0].Amount.Uint64())
	return nil
}

func (k *TestDoubleSlashKeeper) ListActiveValidators(ctx cosmos.Context) (NodeAccounts, error) {
	return NodeAccounts{k.na}, nil
}

func (k *TestDoubleSlashKeeper) GetNodeAccount(ctx cosmos.Context, nodeAddress cosmos.AccAddress) (NodeAccount, error) {
	if nodeAddress.String() == k.na.NodeAddress.String() {
		return k.na, nil
	}
	return NodeAccount{}, errors.New("kaboom")
}

func (k *TestDoubleSlashKeeper) SetNodeAccount(ctx cosmos.Context, na NodeAccount) error {
	k.na = na
	return nil
}

func (k *TestDoubleSlashKeeper) GetNetwork(ctx cosmos.Context) (Network, error) {
	return k.network, nil
}

func (k *TestDoubleSlashKeeper) SetNetwork(ctx cosmos.Context, data Network) error {
	k.network = data
	return nil
}

func (k *TestDoubleSlashKeeper) IncNodeAccountSlashPoints(ctx cosmos.Context, addr cosmos.AccAddress, pts int64) error {
	k.slashPoints[addr.String()] += pts
	return nil
}

func (k *TestDoubleSlashKeeper) DecNodeAccountSlashPoints(ctx cosmos.Context, addr cosmos.AccAddress, pts int64) error {
	k.slashPoints[addr.String()] -= pts
	return nil
}

func (k *TestDoubleSlashKeeper) GetMimir(_ cosmos.Context, key string) (int64, error) {
	return k.mimir[key], nil
}
