package chainclients

import (
	"time"

	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/evm"

	"github.com/rs/zerolog/log"
	"gitlab.com/mayachain/mayanode/bifrost/tss/go-tss/tss"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/config"

	"gitlab.com/mayachain/mayanode/bifrost/mayaclient"
	"gitlab.com/mayachain/mayanode/bifrost/metrics"
	"gitlab.com/mayachain/mayanode/bifrost/pubkeymanager"

	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/binance"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/bitcoin"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/dash"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/ethereum"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/kuji"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/radix"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/thorchain"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/zcash"
)

// LoadChains returns chain clients from chain configuration
func LoadChains(thorKeys *mayaclient.Keys,
	cfg map[common.Chain]config.BifrostChainConfiguration,
	server *tss.TssServer,
	mayachainBridge mayaclient.MayachainBridge,
	m *metrics.Metrics,
	pubKeyValidator pubkeymanager.PubKeyValidator,
	poolMgr mayaclient.PoolManager,
) (chains map[common.Chain]ChainClient, restart chan struct{}) {
	logger := log.Logger.With().Str("module", "bifrost").Logger()

	chains = make(map[common.Chain]ChainClient)
	restart = make(chan struct{})
	failedChains := []common.Chain{}

	loadChain := func(chain config.BifrostChainConfiguration) (ChainClient, error) {
		switch chain.ChainID {
		case common.ARBChain, common.AVAXChain:
			return evm.NewEVMClient(thorKeys, chain, server, mayachainBridge, m, pubKeyValidator, poolMgr)
		case common.BNBChain:
			return binance.NewBinance(thorKeys, chain, server, mayachainBridge, m)
		case common.BTCChain:
			return bitcoin.NewClient(thorKeys, chain, server, mayachainBridge, m)
		case common.DASHChain:
			return dash.NewClient(thorKeys, chain, server, mayachainBridge, m)
		case common.ETHChain:
			return ethereum.NewClient(thorKeys, chain, server, mayachainBridge, m, pubKeyValidator, poolMgr)
		case common.KUJIChain:
			return kuji.NewCosmosClient(thorKeys, chain, server, mayachainBridge, m)
		case common.THORChain:
			return thorchain.NewCosmosClient(thorKeys, chain, server, mayachainBridge, m)
		case common.XRDChain:
			return radix.NewClient(thorKeys, chain, server, mayachainBridge, m, pubKeyValidator)
		case common.ZECChain:
			return zcash.NewClient(thorKeys, chain, server, mayachainBridge, m)
		default:
			log.Fatal().Msgf("chain %s is not supported", chain.ChainID)
			return nil, nil
		}
	}

	for _, chain := range cfg {
		if chain.Disabled {
			logger.Info().Msgf("%s chain is disabled by configure", chain.ChainID)
			continue
		}

		client, err := loadChain(chain)

		// trunk-ignore-all(golangci-lint/forcetypeassert)
		switch chain.ChainID {
		case common.BTCChain:
			if err == nil {
				pubKeyValidator.RegisterCallback(client.(*bitcoin.Client).RegisterPublicKey)
			}
		case common.DASHChain:
			if err == nil {
				pubKeyValidator.RegisterCallback(client.(*dash.Client).RegisterPublicKey)
			}
		case common.ZECChain:
			if err == nil {
				pubKeyValidator.RegisterCallback(client.(*zcash.Client).RegisterPublicKey)
			}
		}

		if err != nil {
			logger.Error().Err(err).Stringer("chain", chain.ChainID).Msg("failed to load chain")
			failedChains = append(failedChains, chain.ChainID)
		} else {
			chains[chain.ChainID] = client
		}
	}

	// watch failed chains minutely and restart bifrost if any succeed init
	if len(failedChains) > 0 {
		go func() {
			tick := time.NewTicker(time.Minute)
			for range tick.C {
				for _, chain := range failedChains {
					ccfg := cfg[chain]
					ccfg.BlockScanner.DBPath = "" // in-memory db

					_, err := loadChain(ccfg)
					if err == nil {
						logger.Info().Stringer("chain", chain).Msg("chain loaded, restarting bifrost")
						close(restart)
						return
					} else {
						logger.Error().Err(err).Stringer("chain", chain).Msg("failed to load chain")
					}
				}
			}
		}()
	}

	return chains, restart
}
