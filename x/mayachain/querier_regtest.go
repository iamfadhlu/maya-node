//go:build regtest
// +build regtest

package mayachain

import (
	"fmt"

	abci "github.com/tendermint/tendermint/abci/types"

	"gitlab.com/mayachain/mayanode/common/cosmos"
	q "gitlab.com/mayachain/mayanode/x/mayachain/query"
)

func init() {
	initManager = func(mgr *Mgrs, ctx cosmos.Context) {
		_ = mgr.BeginBlock(ctx)
	}

	optionalQuery = func(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
		switch path[0] {
		case q.QueryExport.Key:
			return queryExport(ctx, path[1:], req, mgr)
		default:
			return nil, cosmos.ErrUnknownRequest(
				fmt.Sprintf("unknown mayachain query endpoint: %s", path[0]),
			)
		}
	}
}

func queryExport(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	return jsonify(ctx, ExportGenesis(ctx, mgr.Keeper()))
}
