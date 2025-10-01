package mayachain

import (
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"gitlab.com/mayachain/mayanode/common"
	cosmos "gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

type DummySlasher struct {
	pts map[string]int64
}

func NewDummySlasher() *DummySlasher {
	return &DummySlasher{
		pts: make(map[string]int64),
	}
}

func (d DummySlasher) BeginBlock(ctx cosmos.Context, req abci.RequestBeginBlock, constAccessor constants.ConstantValues) {
}

func (d DummySlasher) HandleDoubleSign(ctx cosmos.Context, addr crypto.Address, infractionHeight int64, constAccessor constants.ConstantValues) error {
	return errKaboom
}

func (d DummySlasher) LackObserving(ctx cosmos.Context, constAccessor constants.ConstantValues) error {
	return errKaboom
}

func (d DummySlasher) LackSigning(ctx cosmos.Context, mgr Manager) error {
	return errKaboom
}

func (d DummySlasher) SlashVault(ctx cosmos.Context, vaultPK common.PubKey, coins common.Coins, mgr Manager) error {
	return errKaboom
}

func (d DummySlasher) SlashVaultToLP(ctx cosmos.Context, vaultPK common.PubKey, coins common.Coins, mgr Manager, subsidize bool) error {
	return errKaboom
}

func (d DummySlasher) SlashNodeAccountLP(ctx cosmos.Context, na NodeAccount, slash cosmos.Uint) (cosmos.Uint, []types.PoolAmt, error) {
	return cosmos.ZeroUint(), []types.PoolAmt{}, errKaboom
}

func (d DummySlasher) IncSlashPoints(ctx cosmos.Context, point int64, addresses ...cosmos.AccAddress) {
	for _, addr := range addresses {
		found := false
		for k := range d.pts {
			if k == addr.String() {
				d.pts[k] += point
				found = true
				break
			}
		}
		if !found {
			d.pts[addr.String()] = point
		}
	}
}

func (d DummySlasher) DecSlashPoints(ctx cosmos.Context, point int64, addresses ...cosmos.AccAddress) {
	for _, addr := range addresses {
		found := false
		for k := range d.pts {
			if k == addr.String() {
				d.pts[k] -= point
				found = true
				break
			}
		}
		if !found {
			d.pts[addr.String()] = -point
		}
	}
}
