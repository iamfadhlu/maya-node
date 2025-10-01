package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	tmhttp "github.com/tendermint/tendermint/rpc/client/http"

	"gitlab.com/mayachain/mayanode/app"
	"gitlab.com/mayachain/mayanode/app/params"
	"gitlab.com/mayachain/mayanode/cmd"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	keeperv1 "gitlab.com/mayachain/mayanode/x/mayachain/keeper/v1"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	eddsaKey "github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

////////////////////////////////////////////////////////////////////////////////////////
// Cosmos
////////////////////////////////////////////////////////////////////////////////////////

var encodingConfig params.EncodingConfig

func init() {
	// initialize the bech32 prefix for mocknet
	config := cosmos.GetConfig()
	config.SetBech32PrefixForAccount("tmaya", "tmayapub")
	config.SetBech32PrefixForValidator("tmayav", "tmayavpub")
	config.SetBech32PrefixForConsensusNode("tmayac", "tmayacpub")
	config.Seal()

	// initialize the codec
	encodingConfig = app.MakeEncodingConfig()
}

func clientContextAndFactory(routine int) (client.Context, tx.Factory) {
	// create new rpc client
	node := fmt.Sprintf("http://localhost:%d", 26657+routine)
	rpcClient, err := tmhttp.New(node, "/websocket")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create tendermint client")
	}

	// create cosmos-sdk client context
	clientCtx := client.Context{
		Client:            rpcClient,
		ChainID:           "mayachain",
		JSONCodec:         encodingConfig.Marshaler,
		Codec:             encodingConfig.Marshaler,
		InterfaceRegistry: encodingConfig.InterfaceRegistry,
		Keyring:           keyRing,
		BroadcastMode:     flags.BroadcastSync,
		SkipConfirm:       true,
		TxConfig:          encodingConfig.TxConfig,
		AccountRetriever:  authtypes.AccountRetriever{},
		NodeURI:           node,
		LegacyAmino:       encodingConfig.Amino,
	}

	// create tx factory
	txFactory := tx.Factory{}
	txFactory = txFactory.WithKeybase(clientCtx.Keyring)
	txFactory = txFactory.WithTxConfig(clientCtx.TxConfig)
	txFactory = txFactory.WithAccountRetriever(clientCtx.AccountRetriever)
	txFactory = txFactory.WithChainID(clientCtx.ChainID)
	txFactory = txFactory.WithGas(0)
	txFactory = txFactory.WithSignMode(signing.SignMode_SIGN_MODE_DIRECT)

	return clientCtx, txFactory
}

////////////////////////////////////////////////////////////////////////////////////////
// Logging
////////////////////////////////////////////////////////////////////////////////////////

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	log.Logger = log.With().Caller().Logger()

	// set to info level if DEBUG is not set (debug is the default level)
	if os.Getenv("DEBUG") == "" {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}

////////////////////////////////////////////////////////////////////////////////////////
// Colors
////////////////////////////////////////////////////////////////////////////////////////

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorPurple = "\033[35m"

	// save for later
	// ColorYellow = "\033[33m"
	// ColorBlue   = "\033[34m"
	// ColorCyan   = "\033[36m"
	// ColorGray   = "\033[37m"
	// ColorWhite  = "\033[97m"
)

////////////////////////////////////////////////////////////////////////////////////////
// HTTP
////////////////////////////////////////////////////////////////////////////////////////

var httpClient = &http.Client{
	Transport: &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
	},
	Timeout: 5 * time.Second,
}

////////////////////////////////////////////////////////////////////////////////////////
// Mayachain Module Addresses
////////////////////////////////////////////////////////////////////////////////////////

// TODO: determine how to return these programmatically without keeper
const (
	ModuleAddrMayachain    = "tmaya1zxw7mpq9zc4pe97unf85lljcwnhf4h2ky5dcyf"
	ModuleAddrAsgard       = "tmaya1g98cy3n9mmjrpn0sxmn63lztelera37nrn4zh6"
	ModuleAddrBond         = "tmaya17gw75axcnr8747pkanye45pnrwk7p9c3uquyle"
	ModuleAddrTransfer     = "tmaya1yl6hdjhmkf37639730gffanpzndzdpmhvcqwdf"
	ModuleAddrReserve      = "tmaya1dheycdevq39qlkxs2a6wuuzyn4aqxhve3qfhf7"
	ModuleAddrFeeCollector = "tmaya17xpfvakm2amg962yls6f84z3kell8c5lj7483h"
	ModuleAddrCacaoPool    = "tmaya167q2uvp3pzqjkefx3qd275yfdgee9fadhejgwg"
	ModuleAddrMayaFund     = "tmaya1577sz8j7xnthm3cl3vgfvmdmkrp7dqrhfj6dsg"
)

////////////////////////////////////////////////////////////////////////////////////////
// Invariants
////////////////////////////////////////////////////////////////////////////////////////

var invariants []string

func init() {
	k := keeperv1.KVStore{}
	for _, ir := range k.InvariantRoutes() {
		invariants = append(invariants, ir.Route)
	}
}

////////////////////////////////////////////////////////////////////////////////////////
// Keys
////////////////////////////////////////////////////////////////////////////////////////

var (
	keyRing            = keyring.NewInMemory()
	addressToName      = map[string]string{} // maya...->dog, 0x...->dog
	templateAddress    = map[string]string{} // addr_maya_dog->maya..., addr_eth_dog->0x...
	templatePubKey     = map[string]string{} // pubkey_dog->mayapub...
	templateConsPubKey = map[string]string{} // cons_pubkey_dog->mayacpub...

	birdMnemonic   = strings.Repeat("bird ", 23) + "asthma"
	catMnemonic    = strings.Repeat("cat ", 23) + "crawl"
	deerMnemonic   = strings.Repeat("deer ", 23) + "diesel"
	dogMnemonic    = strings.Repeat("dog ", 23) + "fossil"
	duckMnemonic   = strings.Repeat("duck ", 23) + "face"
	fishMnemonic   = strings.Repeat("fish ", 23) + "fade"
	foxMnemonic    = strings.Repeat("fox ", 23) + "filter"
	frogMnemonic   = strings.Repeat("frog ", 23) + "flat"
	goatMnemonic   = strings.Repeat("goat ", 23) + "install"
	hawkMnemonic   = strings.Repeat("hawk ", 23) + "juice"
	lionMnemonic   = strings.Repeat("lion ", 23) + "misery"
	mouseMnemonic  = strings.Repeat("mouse ", 23) + "option"
	muleMnemonic   = strings.Repeat("mule ", 23) + "major"
	pigMnemonic    = strings.Repeat("pig ", 23) + "quick"
	rabbitMnemonic = strings.Repeat("rabbit ", 23) + "rent"
	wolfMnemonic   = strings.Repeat("wolf ", 23) + "victory"

	// mnemonics contains the set of all mnemonics for accounts used in tests
	mnemonics = [...]string{
		dogMnemonic,
		catMnemonic,
		foxMnemonic,
		pigMnemonic,
		birdMnemonic,
		deerMnemonic,
		duckMnemonic,
		fishMnemonic,
		frogMnemonic,
		goatMnemonic,
		hawkMnemonic,
		lionMnemonic,
		mouseMnemonic,
		muleMnemonic,
		rabbitMnemonic,
		wolfMnemonic,
	}
)

func init() {
	// register functions for all mnemonic-chain addresses
	for _, m := range mnemonics {
		name := strings.Split(m, " ")[0]

		// create pubkey for mnemonic
		derivedPriv, err := hd.Secp256k1.Derive()(m, "", cmd.BASEChainHDPath)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to derive private key")
		}
		privKey := hd.Secp256k1.Generate()(derivedPriv)
		ecdsaPubKey, err := cosmos.Bech32ifyPubKey(cosmos.Bech32PubKeyTypeAccPub, privKey.PubKey())
		if err != nil {
			log.Fatal().Err(err).Msg("failed to bech32ify ecdsa pubkey")
		}

		ed25519PrivKey := eddsaKey.GenPrivKeyFromSecret([]byte(m))
		edd2519ConsPubKey, err := cosmos.Bech32ifyPubKey(cosmos.Bech32PubKeyTypeConsPub, ed25519PrivKey.PubKey())
		if err != nil {
			log.Fatal().Err(err).Msg("failed to bech32ify EdDSA cons pubkey")
		}
		ed25519PubKey, err := cosmos.Bech32ifyPubKey(cosmos.Bech32PubKeyTypeAccPub, ed25519PrivKey.PubKey())
		if err != nil {
			log.Fatal().Err(err).Msg("failed to bech32ify EdDSA acc pubkey")
		}

		// add key to keyring
		_, err = keyRing.NewAccount(name, m, "", cmd.BASEChainHDPath, hd.Secp256k1)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to add account to keyring")
		}

		for _, chain := range common.AllChains {

			// register template address for all chains
			var addr common.Address
			if chain.GetSigningAlgo() == common.SigningAlgoSecp256k1 {
				addr, err = common.PubKey(ecdsaPubKey).GetAddress(chain)
			} else if chain.GetSigningAlgo() == common.SigningAlgoEd25519 {
				addr, err = common.PubKey(ed25519PubKey).GetAddress(chain)
			}
			if err != nil {
				log.Fatal().Err(err).Msgf("failed to get address for chain %s", chain)
			}
			lowerChain := strings.ToLower(chain.String())
			templateAddress[fmt.Sprintf("addr_%s_%s", lowerChain, name)] = addr.String()

			// register address to name
			addressToName[addr.String()] = name

			// register pubkey for mayachain
			if chain == common.BASEChain {
				templatePubKey[fmt.Sprintf("pubkey_%s", name)] = ecdsaPubKey
				templateConsPubKey[fmt.Sprintf("cons_pubkey_%s", name)] = edd2519ConsPubKey
				templatePubKey[fmt.Sprintf("pubkey_%s_eddsa", name)] = ed25519PubKey
			}
		}
	}
}
