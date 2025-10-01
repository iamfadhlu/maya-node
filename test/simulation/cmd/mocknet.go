package main

import (
	"fmt"
	"os"
	"strings"
	"sync"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/rs/zerolog/log"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/config"
	"gitlab.com/mayachain/mayanode/test/simulation/pkg/evm"
	"gitlab.com/mayachain/mayanode/test/simulation/pkg/mayanode"
	. "gitlab.com/mayachain/mayanode/test/simulation/pkg/types"
	ttypes "gitlab.com/mayachain/mayanode/x/mayachain/types"
)

////////////////////////////////////////////////////////////////////////////////////////
// Chain RPCs
////////////////////////////////////////////////////////////////////////////////////////

var chainRPCs = map[common.Chain]string{
	common.BTCChain:  "http://localhost:18443",
	common.ZECChain:  "http://localhost:18232",
	common.DASHChain: "http://localhost:19898",
	common.ETHChain:  "http://localhost:8545",
	common.ARBChain:  "http://localhost:8547",
	common.KUJIChain: "localhost:9091",
}

////////////////////////////////////////////////////////////////////////////////////////
// Mocknet Mnemonics
////////////////////////////////////////////////////////////////////////////////////////

var (
	mocknetMasterMnemonic = strings.Repeat("master ", 23) + "notice"

	mocknetValidatorMnemonics = [...]string{
		strings.Repeat("dog ", 23) + "fossil",
		// strings.Repeat("cat ", 23) + "crawl",
		// strings.Repeat("fox ", 23) + "filter",
		// strings.Repeat("pig ", 23) + "quick",
	}

	mocknetUserMnemonics = [...]string{
		strings.Repeat("bird ", 23) + "asthma",
		strings.Repeat("deer ", 23) + "diesel",
		strings.Repeat("duck ", 23) + "face",
		strings.Repeat("fish ", 23) + "fade",
		strings.Repeat("frog ", 23) + "flat",
		strings.Repeat("goat ", 23) + "install",
		strings.Repeat("hawk ", 23) + "juice",
		strings.Repeat("lion ", 23) + "misery",
		strings.Repeat("mouse ", 23) + "option",
		strings.Repeat("mule ", 23) + "major",
		strings.Repeat("rabbit ", 23) + "rent",
		strings.Repeat("wolf ", 23) + "victory",
	}
)

////////////////////////////////////////////////////////////////////////////////////////
// Init
////////////////////////////////////////////////////////////////////////////////////////

func InitConfig(parallelism int) *OpConfig {
	if parallelism > len(mocknetUserMnemonics) {
		log.Fatal().Msg("parallelism exceeds number of user accounts")
	}
	log.Info().Msg("initializing mocknet simulation user accounts")

	c := &OpConfig{}
	mu := &sync.Mutex{}
	wg := &sync.WaitGroup{}
	sem := make(chan struct{}, 8)

	// since we reuse the bifrost mayaclient, load endpoints into config package
	os.Setenv("BIFROST_MAYACHAIN_CHAIN_HOST", "localhost:1317")
	os.Setenv("BIFROST_MAYACHAIN_CHAIN_RPC", "localhost:26657")
	config.Init()

	// validators
	for _, mnemonic := range mocknetValidatorMnemonics {
		wg.Add(1)
		sem <- struct{}{}
		go func(mnemonic string) {
			a := NewAccount(mnemonic, liteClientConstructors)
			mu.Lock()
			c.NodeAccounts = append(c.NodeAccounts, a)
			mu.Unlock()

			// send gaia network fee observation
			// log.Info().Msg("posting gaia network fee")
			// for {
			// 	_, err := a.Mayachain.PostNetworkFee(1, common.KUJIChain, 1, 1_000_000)
			// 	if err == nil {
			// 		break
			// 	}
			// 	log.Error().Err(err).Msg("failed to post network fee")
			// 	time.Sleep(5 * time.Second)
			// }

			<-sem
			wg.Done()
		}(mnemonic)
	}

	// users
	for _, mnemonic := range mocknetUserMnemonics[:parallelism] {
		wg.Add(1)
		sem <- struct{}{}
		go func(mnemonic string) {
			a := NewAccount(mnemonic, liteClientConstructors)
			mu.Lock()
			c.UserAccounts = append(c.UserAccounts, a)
			mu.Unlock()
			<-sem
			wg.Done()
		}(mnemonic)
	}

	// wait for all accounts to be created
	wg.Wait()

	// fund all user accounts from master
	master := NewAccount(mocknetMasterMnemonic, liteClientConstructors)

	// log all configured tokens, their decimals, and master balance
	for chain := range liteClientConstructors {
		account, err := master.ChainClients[chain].GetAccount(nil)
		if err != nil {
			log.Fatal().Stringer("chain", chain).Err(err).Msg("failed to get master account")
		}
		for _, coin := range account.Coins {
			ctxLog := log.Info().
				Stringer("chain", chain).
				Stringer("asset", coin.Asset).
				Stringer("address", master.Address(chain)).
				Str("amount", coin.Amount.String())

			// on evm chains, also wait for token to be deployed and log token decimals for debugging
			if chain.IsEVM() && !coin.Asset.IsGasAsset() {
				token, found := evm.Tokens(chain)[coin.Asset]
				if !found {
					log.Fatal().Stringer("asset", coin.Asset).Stringer("chain", chain).Msg("asset not found in token list")
				}
				evmClient, ok := master.ChainClients[chain].(*evm.Client)
				if !ok {
					log.Fatal().Stringer("chain client is not instance of evm %s", chain)
				}

				var isDeployed bool
				isDeployed, err = evmClient.IsContractDeployed(token.Address)
				if err != nil {
					log.Fatal().Err(err).Msg("failed to get token decimals")
				}
				if !isDeployed {
					log.Fatal().Str("token", token.Address).Stringer("asset", coin.Asset).Stringer("chain", chain).Msg("contract not deployed")
				}

				tokenDecimals, err := evmClient.GetTokenDecimals(token.Address)
				if err != nil {
					log.Fatal().Err(err).Msg("failed to get token decimals")
				}

				// sanity check our configured token decimals
				if tokenDecimals != token.Decimals {
					log.Fatal().
						Int("actual", tokenDecimals).
						Int("configured", token.Decimals).
						Stringer("chain", chain).
						Stringer("asset", coin.Asset).
						Err(err).
						Msg("token decimals mismatch")
				}

				ctxLog = ctxLog.Int("decimals", tokenDecimals)
			}

			// log balance
			ctxLog.Msg("master account balance")
		}
	}

	// master account is also mimir admin
	c.AdminAccount = master

	// fund all user accounts
	funded := []*Account{}
	for _, user := range c.UserAccounts {
		if fundUserMayaAccount(master, user) {
			funded = append(funded, user)
		}
	}

	// fund user accounts with one goroutine per chain
	wg = &sync.WaitGroup{}
	for _, chain := range common.AllChains {
		chainSeedAmount := sdk.ZeroUint()
		switch chain {
		case common.BTCChain, common.ETHChain:
			chainSeedAmount = sdk.NewUint(10 * common.One)
		case common.ZECChain, common.DASHChain:
			chainSeedAmount = sdk.NewUint(100 * common.One)
		// case common.KUJIChain:
		// 	chainSeedAmount = sdk.NewUint(1000 * common.One)
		default:
			continue // all other chains currently unsupported
		}

		wg.Add(1)
		go func(chain common.Chain, amount sdk.Uint) {
			defer wg.Done()
			fundUserChainAccounts(master, funded, chain, chainSeedAmount)
		}(chain, chainSeedAmount)
	}
	wg.Wait()

	return c
}

////////////////////////////////////////////////////////////////////////////////////////
// Helpers
////////////////////////////////////////////////////////////////////////////////////////

func fundUserChainAccounts(master *Account, users []*Account, chain common.Chain, amount sdk.Uint) {
	for _, user := range users {
		fundUserChainAccount(master, user, chain, amount)
	}
}

func fundUserChainAccount(master, user *Account, chain common.Chain, amount sdk.Uint) {
	// Check if the chain client exists in the map
	client, ok := master.ChainClients[chain]
	if !ok {
		log.Warn().Stringer("chain", chain).Msg("no chain client found for chain, skipping funding")
		return
	}

	// build tx
	addr, err := user.PubKey(chain).GetAddress(chain)
	if err != nil {
		log.Error().Err(err).Stringer("chain", chain).Msg("failed to get address, skipping funding")
		return
	}
	tx := SimTx{
		Chain:     chain,
		ToAddress: addr,
		Coin:      common.NewCoin(chain.GetGasAsset(), amount),
		Memo:      fmt.Sprintf("SIMULATION:%s", user.Name()),
	}

	from, err := master.PubKey(chain).GetAddress(chain)
	if err != nil {
		log.Error().Err(err).Stringer("chain", chain).Msg("fail to get address for master pubkey")
		return
	}

	// sign tx
	signed, err := client.SignTx(tx)
	if err != nil {
		log.Error().Err(err).Stringer("chain", chain).Msgf("failed to sign master tx, skipping funding: %s → %s (%d)", from, tx.ToAddress, tx.Coin.Amount.Uint64())
		return
	}

	// broadcast tx
	txid, err := client.BroadcastTx(signed)
	if err != nil {
		log.Error().Err(err).
			Str("account", user.Name()).
			Stringer("chain", chain).
			Stringer("address", addr).
			Stringer("from", from).
			Msg("failed to broadcast funding tx, skipping funding")
		return
	}

	amountFloat := float64(amount.Uint64()) / float64(common.One)
	log.Info().
		Str("txid", txid).
		Str("account", user.Name()).
		Stringer("chain", chain).
		Stringer("address", addr).
		Str("amount", fmt.Sprintf("%08f", amountFloat)).
		Msg("account funded")
}

func fundUserMayaAccount(master, user *Account) bool {
	masterMayaAddress, err := master.PubKey(common.BASEChain).GetThorAddress()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get master maya address")
	}

	// skip seeding user if mayachain account has balance
	userMayaAddress, err := user.PubKey(common.BASEChain).GetAddress(common.BASEChain)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get user maya address")
	}
	coins, _ := mayanode.GetBalances(userMayaAddress)
	if len(coins) > 0 {
		log.Info().Str("account", user.Name()).Msg("user has rune, skipping seed")
		return false
	}

	// seed mayachain account
	userMayaAccAddress, err := user.PubKey(common.BASEChain).GetThorAddress()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get user maya address")
	}
	seedAmount := sdk.NewInt(1000000 * common.One) // zlyzol: added 3 zeros
	seedAmountFloat := float64(seedAmount.Uint64()) / float64(common.One)
	tx := &ttypes.MsgSend{
		FromAddress: masterMayaAddress,
		ToAddress:   userMayaAccAddress,
		Amount:      sdk.NewCoins(sdk.NewCoin("cacao", seedAmount)),
	}
	mayaTxid, err := master.Mayachain.Broadcast(tx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to broadcast tx")
	}
	log.Info().
		Stringer("txid", mayaTxid).
		Str("account", user.Name()).
		Stringer("chain", common.BASEChain).
		Stringer("address", userMayaAccAddress).
		Str("amount", fmt.Sprintf("%08f", seedAmountFloat)).
		Msg("account funded")

	return true
}
