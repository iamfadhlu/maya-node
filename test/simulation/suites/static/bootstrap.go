package static

import (
	"fmt"

	"github.com/rs/zerolog/log"

	openapi "gitlab.com/mayachain/mayanode/openapi/gen"
	"gitlab.com/mayachain/mayanode/test/simulation/actors"
	"gitlab.com/mayachain/mayanode/test/simulation/pkg/mayanode"
	. "gitlab.com/mayachain/mayanode/test/simulation/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////////////
// Bootstrap
////////////////////////////////////////////////////////////////////////////////////////

func Bootstrap() *Actor {
	a := &Actor{
		Name: "Bootstrap",
	}

	pools, err := mayanode.GetPools()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get pools")
	}

	// bootstrap pools for all chains
	for _, chain := range Chains {
		// skip bootstrapping existing pools
		found := false
		var gasPool openapi.Pool
		for _, pool := range pools {
			if pool.Asset == chain.GetGasAsset().String() {
				found = true
				gasPool = pool
				break
			}
		}

		// skip found, except BTC since it's used for liquidity nodes bootstrapping
		if found && gasPool.BalanceAsset != "0" && gasPool.BalanceCacao != "0" {
			log.Info().Str("chain", chain.GetGasAsset().String()).Msg("skip existing pool bootstrap")
			continue
		}

		a.Children = append(a.Children, actors.NewDualLPActor(chain.GetGasAsset()))
	}

	// verify pools
	a.Append(&Actor{
		Name: "Bootstrap-Verify",
		Ops: []Op{
			func(config *OpConfig) OpResult {
				pools, err := mayanode.GetPools()
				if err != nil {
					return OpResult{Finish: true, Error: err}
				}

				// all pools should be available
				for _, pool := range pools {
					if pool.Status != "Available" {
						return OpResult{
							Finish: true,
							Error:  fmt.Errorf("pool %s not available", pool.Asset),
						}
					}
				}

				// all chains should have pools
				if len(pools) != len(Chains) {
					return OpResult{
						Finish: true,
						Error:  fmt.Errorf("expected %d pools, got %d", len(Chains), len(pools)),
					}
				}

				return OpResult{Finish: true}
			},
		},
	})

	return a
}
