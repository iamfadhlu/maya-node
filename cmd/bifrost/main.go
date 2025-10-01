package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	btsskeygen "github.com/binance-chain/tss-lib/ecdsa/keygen"
	golog "github.com/ipfs/go-log"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	flag "github.com/spf13/pflag"

	"gitlab.com/mayachain/mayanode/bifrost/tss/go-tss/common"
	"gitlab.com/mayachain/mayanode/bifrost/tss/go-tss/tss"

	"gitlab.com/mayachain/mayanode/app"
	"gitlab.com/mayachain/mayanode/bifrost/mayaclient"
	"gitlab.com/mayachain/mayanode/bifrost/metrics"
	"gitlab.com/mayachain/mayanode/bifrost/observer"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients"
	"gitlab.com/mayachain/mayanode/bifrost/pubkeymanager"
	"gitlab.com/mayachain/mayanode/bifrost/signer"
	btss "gitlab.com/mayachain/mayanode/bifrost/tss"
	"gitlab.com/mayachain/mayanode/cmd"
	tcommon "gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/config"
	"gitlab.com/mayachain/mayanode/constants"
)

// THORNode define version / revision here , so THORNode could inject the version from CI pipeline if THORNode want to
var (
	version  string
	revision string
)

const (
	serverIdentity = "bifrost"
)

func printVersion() {
	fmt.Printf("%s v%s, rev %s\n", serverIdentity, version, revision)
}

func main() {
	showVersion := flag.Bool("version", false, "Shows version")
	logLevel := flag.StringP("log-level", "l", "info", "Log Level")
	pretty := flag.BoolP("pretty-log", "p", false, "Enables unstructured prettified logging. This is useful for local debugging")
	tssPreParam := flag.StringP("preparm", "t", "", "pre-generated PreParam file used for tss")
	flag.Parse()

	if *showVersion {
		printVersion()
		return
	}

	initPrefix()
	initLog(*logLevel, *pretty)
	config.Init()
	config.InitBifrost()
	cfg := config.GetBifrost()

	// metrics
	m, err := metrics.NewMetrics(cfg.Metrics)
	if err != nil {
		log.Fatal().Err(err).Msg("fail to create metric instance")
	}
	if err = m.Start(); err != nil {
		log.Fatal().Err(err).Msg("fail to start metric collector")
	}
	if len(cfg.MayaChain.SignerName) == 0 {
		log.Fatal().Msg("signer name is empty")
	}
	if len(cfg.MayaChain.SignerPasswd) == 0 {
		log.Fatal().Msg("signer password is empty")
	}
	kb, _, err := mayaclient.GetKeyringKeybase(cfg.MayaChain.ChainHomeFolder, cfg.MayaChain.SignerName, cfg.MayaChain.SignerPasswd)
	if err != nil {
		log.Fatal().Err(err).Msg("fail to get keyring keybase")
	}

	k := mayaclient.NewKeysWithKeybase(kb, cfg.MayaChain.SignerName, cfg.MayaChain.SignerPasswd)
	// thorchain bridge
	var mayachainBridge mayaclient.MayachainBridge
	mayachainBridge, err = mayaclient.NewMayachainBridge(cfg.MayaChain, m, k)
	if err != nil {
		log.Fatal().Err(err).Msg("fail to create new thorchain bridge")
	}
	if err = mayachainBridge.EnsureNodeWhitelistedWithTimeout(); err != nil {
		log.Fatal().Err(err).Msg("node account is not whitelisted, can't start")
	}
	// PubKey Manager
	var pubkeyMgr *pubkeymanager.PubKeyManager
	pubkeyMgr, err = pubkeymanager.NewPubKeyManager(mayachainBridge, m)
	if err != nil {
		log.Fatal().Err(err).Msg("fail to create pubkey manager")
	}
	if err = pubkeyMgr.Start(); err != nil {
		log.Fatal().Err(err).Msg("fail to start pubkey manager")
	}

	// automatically attempt to recover TSS keyshares if they are missing
	if err = btss.RecoverKeyShares(cfg, mayachainBridge); err != nil {
		log.Error().Err(err).Msg("fail to recover key shares")
	}

	// setup TSS signing
	priKey, err := k.GetPrivateKey()
	if err != nil {
		log.Fatal().Err(err).Msg("fail to get secp256k1 private key")
	}

	bootstrapPeers, err := cfg.TSS.GetBootstrapPeers()
	if err != nil {
		log.Fatal().Err(err).Msg("fail to get bootstrap peers")
	}
	tmPrivateKey := tcommon.CosmosPrivateKeyToTMPrivateKey(priKey)

	priKeyEddsa, err := k.GetPrivateKeyEDDSA()
	if err != nil {
		log.Fatal().Err(err).Msg("fail to get eddsa private key")
	}

	tmPrivateKeyEddsa := tcommon.CosmosPrivateKeyToTMPrivateKey(priKeyEddsa)

	consts := constants.NewConstantValue010()
	jailTimeKeygen := time.Duration(consts.GetInt64Value(constants.JailTimeKeygen)) * constants.MayachainBlockTime
	jailTimeKeysign := time.Duration(consts.GetInt64Value(constants.JailTimeKeysign)) * constants.MayachainBlockTime
	if cfg.Signer.KeygenTimeout >= jailTimeKeygen {
		log.Fatal().
			Stringer("keygenTimeout", cfg.Signer.KeygenTimeout).
			Stringer("keygenJail", jailTimeKeygen).
			Msg("keygen timeout must be shorter than jail time")
	}
	if cfg.Signer.KeysignTimeout >= jailTimeKeysign {
		log.Fatal().
			Stringer("keysignTimeout", cfg.Signer.KeysignTimeout).
			Stringer("keysignJail", jailTimeKeysign).
			Msg("keysign timeout must be shorter than jail time")
	}

	tssIns, err := tss.NewTss(
		bootstrapPeers,
		cfg.TSS.P2PPort,
		tmPrivateKey,
		tmPrivateKeyEddsa,
		cfg.TSS.Rendezvous,
		app.DefaultNodeHome(),
		common.TssConfig{
			EnableMonitor:   true,
			KeyGenTimeout:   cfg.Signer.KeygenTimeout,
			KeySignTimeout:  cfg.Signer.KeysignTimeout,
			PartyTimeout:    cfg.Signer.PartyTimeout,
			PreParamTimeout: cfg.Signer.PreParamTimeout,
		},
		getLocalPreParam(*tssPreParam),
		cfg.TSS.ExternalIP,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("fail to create tss instance")
	}

	if err = tssIns.Start(); err != nil {
		log.Err(err).Msg("fail to start tss instance")
	}

	if len(cfg.Chains) == 0 {
		log.Fatal().Err(err).Msg("missing chains")
		return
	}

	// ensure we have a protocol for chain RPC Hosts
	for _, chainCfg := range cfg.Chains {
		if chainCfg.Disabled {
			continue
		}
		if len(chainCfg.RPCHost) == 0 {
			log.Fatal().Err(err).Stringer("chain", chainCfg.ChainID).Msg("missing chain RPC host")
			return
		}
		if !strings.HasPrefix(chainCfg.RPCHost, "http") {
			chainCfg.RPCHost = fmt.Sprintf("http://%s", chainCfg.RPCHost)
		}

		if len(chainCfg.BlockScanner.RPCHost) == 0 {
			log.Fatal().Err(err).Msg("missing chain RPC host")
			return
		}
		if !strings.HasPrefix(chainCfg.BlockScanner.RPCHost, "http") {
			chainCfg.BlockScanner.RPCHost = fmt.Sprintf("http://%s", chainCfg.BlockScanner.RPCHost)
		}
	}
	poolMgr := mayaclient.NewPoolMgr(mayachainBridge)
	chains, restart := chainclients.LoadChains(k, cfg.Chains, tssIns, mayachainBridge, m, pubkeyMgr, poolMgr)
	if len(chains) == 0 {
		log.Fatal().Msg("fail to load any chains")
	}
	tssKeysignMetricMgr := metrics.NewTssKeysignMetricMgr()
	healthServer := NewHealthServer(cfg.TSS.InfoAddress, tssIns, chains)
	go func() {
		defer log.Info().Msg("health server exit")
		if err = healthServer.Start(); err != nil {
			log.Error().Err(err).Msg("fail to start health server")
		}
	}()
	// start observer
	obs, err := observer.NewObserver(pubkeyMgr, chains, mayachainBridge, m, cfg.Chains[tcommon.BTCChain].BlockScanner.DBPath, tssKeysignMetricMgr)
	if err != nil {
		log.Fatal().Err(err).Msg("fail to create observer")
	}
	if err = obs.Start(); err != nil {
		log.Fatal().Err(err).Msg("fail to start observer")
	}

	// start signer
	sign, err := signer.NewSigner(cfg.Signer, mayachainBridge, k, pubkeyMgr, tssIns, chains, m, tssKeysignMetricMgr, obs)
	if err != nil {
		log.Fatal().Err(err).Msg("fail to create instance of signer")
	}
	if err := sign.Start(); err != nil {
		log.Fatal().Err(err).Msg("fail to start signer")
	}

	// wait....
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-ch:
	case <-restart:
	}
	log.Info().Msg("stop signal received")

	// stop observer
	if err := obs.Stop(); err != nil {
		log.Fatal().Err(err).Msg("fail to stop observer")
	}
	// stop signer
	if err := sign.Stop(); err != nil {
		log.Fatal().Err(err).Msg("fail to stop signer")
	}
	// stop go tss
	tssIns.Stop()
	if err := healthServer.Stop(); err != nil {
		log.Fatal().Err(err).Msg("fail to stop health server")
	}
}

func initPrefix() {
	cosmosSDKConfg := cosmos.GetConfig()
	cosmosSDKConfg.SetBech32PrefixForAccount(cmd.Bech32PrefixAccAddr, cmd.Bech32PrefixAccPub)
	cosmosSDKConfg.Seal()
}

func initLog(level string, pretty bool) {
	l, err := zerolog.ParseLevel(level)
	if err != nil {
		log.Warn().Msgf("%s is not a valid log-level, falling back to 'info'", level)
	}
	var out io.Writer = os.Stdout
	if pretty {
		out = zerolog.ConsoleWriter{Out: os.Stdout}
	}
	zerolog.SetGlobalLevel(l)
	log.Logger = log.Output(out).With().Caller().Str("service", serverIdentity).Logger()

	logLevel := golog.LevelInfo
	switch l {
	case zerolog.DebugLevel:
		logLevel = golog.LevelDebug
	case zerolog.InfoLevel:
		logLevel = golog.LevelInfo
	case zerolog.ErrorLevel:
		logLevel = golog.LevelError
	case zerolog.FatalLevel:
		logLevel = golog.LevelFatal
	case zerolog.PanicLevel:
		logLevel = golog.LevelPanic
	}
	golog.SetAllLoggers(logLevel)
	if err := golog.SetLogLevel("tss-lib", level); err != nil {
		log.Fatal().Err(err).Msg("fail to set tss-lib loglevel")
	}
}

func getLocalPreParam(file string) *btsskeygen.LocalPreParams {
	if len(file) == 0 {
		return nil
	}
	// #nosec G304 this is to read a file provided by a start up parameter , it will not be any random user input
	buf, err := os.ReadFile(file)
	if err != nil {
		log.Fatal().Msgf("fail to read file:%s", file)
		return nil
	}
	buf = bytes.Trim(buf, "\n")
	log.Info().Msg(string(buf))
	result, err := hex.DecodeString(string(buf))
	if err != nil {
		log.Fatal().Msg("fail to hex decode the file content")
		return nil
	}
	var preParam btsskeygen.LocalPreParams
	if err := json.Unmarshal(result, &preParam); err != nil {
		log.Fatal().Msg("fail to unmarshal file content to LocalPreParams")
		return nil
	}
	return &preParam
}
