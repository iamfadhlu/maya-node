//go:build regtest
// +build regtest

package mayachain

import (
	abci "github.com/tendermint/tendermint/abci/types"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
)

func InitGenesis(ctx cosmos.Context, keeper keeper.Keeper, data GenesisState) []abci.ValidatorUpdate {
	validators := initGenesis(ctx, keeper, data)
	return validators[:1]
}
