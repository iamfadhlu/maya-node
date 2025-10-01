package main

import (
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	prefix "gitlab.com/mayachain/mayanode/cmd"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/test/simulation/actors/features"
	"gitlab.com/mayachain/mayanode/test/simulation/pkg/dag"
	"gitlab.com/mayachain/mayanode/test/simulation/pkg/evm"
	. "gitlab.com/mayachain/mayanode/test/simulation/pkg/types"
	"gitlab.com/mayachain/mayanode/test/simulation/pkg/utxo"
	"gitlab.com/mayachain/mayanode/test/simulation/suites/static"
	"gitlab.com/mayachain/mayanode/test/simulation/watchers"
)

////////////////////////////////////////////////////////////////////////////////////////
// Config
////////////////////////////////////////////////////////////////////////////////////////

const (
	DefaultParallelism = "8"
)

var liteClientConstructors = map[common.Chain]LiteChainClientConstructor{
	common.BTCChain: utxo.NewConstructor(chainRPCs[common.BTCChain]),
	common.ZECChain: utxo.NewConstructor(chainRPCs[common.ZECChain]),
	common.ETHChain: evm.NewConstructor(chainRPCs[common.ETHChain]),
	// common.KUJIChain: pkgcosmos.NewConstructor(chainRPCs[common.KUJIChain]),
}

////////////////////////////////////////////////////////////////////////////////////////
// Main
////////////////////////////////////////////////////////////////////////////////////////

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout}).With().Caller().Logger()

	// init prefixes
	ccfg := cosmos.GetConfig()
	ccfg.SetBech32PrefixForAccount(prefix.Bech32PrefixAccAddr, prefix.Bech32PrefixAccPub)
	ccfg.SetBech32PrefixForValidator(prefix.Bech32PrefixValAddr, prefix.Bech32PrefixValPub)
	ccfg.SetBech32PrefixForConsensusNode(prefix.Bech32PrefixConsAddr, prefix.Bech32PrefixConsPub)
	ccfg.SetCoinType(prefix.BASEChainCoinType)
	ccfg.SetPurpose(prefix.BASEChainCoinPurpose)
	ccfg.Seal()

	// wait until bifrost is ready
	for {
		res, err := http.Get("http://localhost:6040/p2pid")
		if err == nil && res.StatusCode == 200 {
			break
		}
		log.Info().Msg("waiting for bifrost to be ready")
		time.Sleep(time.Second)
	}

	// combine all actor dags for the complete test run
	root := &Actor{Name: "root"}
	root.Append(static.Bootstrap())
	root.Append(static.Swaps())
	root.Append(static.Arbs())
	root.Append(features.Consolidate())

	// root.Append(static.Ragnarok())

	// gather config from the environment
	parallelism := os.Getenv("PARALLELISM")
	if parallelism == "" {
		parallelism = DefaultParallelism
	}
	parallelismInt, err := strconv.Atoi(parallelism)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse PARALLELISM")
	}

	cfg := InitConfig(parallelismInt)

	// start watchers
	for _, w := range []*Watcher{watchers.NewInvariants()} {
		log.Info().Str("watcher", w.Name).Msg("starting watcher")
		go func(w *Watcher) {
			err := w.Execute(cfg, log.Output(os.Stderr))
			if err != nil {
				log.Fatal().Err(err).Msg("watcher failed")
			}
		}(w)
	}

	// run the simulation
	dag.Execute(cfg, root, parallelismInt)
}
