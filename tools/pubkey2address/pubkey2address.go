package main

import (
	"flag"
	"fmt"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

func main() {
	raw := flag.String("p", "", "thor bech32 pubkey")
	flag.Parse()

	if len(*raw) == 0 {
		panic("no pubkey provided")
	}

	// Read in the configuration file for the sdk
	nw := common.CurrentChainNetwork
	switch nw {
	case common.MockNet, common.TestNet:
		fmt.Println("BASEChain testnet:")
		config := cosmos.GetConfig()
		config.SetBech32PrefixForAccount("tmaya", "tmayapub")
		config.SetBech32PrefixForValidator("tmayav", "tmayavpub")
		config.SetBech32PrefixForConsensusNode("tmayac", "tmayacpub")
		config.Seal()
	case common.StageNet:
		fmt.Println("BASEChain stagenet:")
		config := cosmos.GetConfig()
		config.SetBech32PrefixForAccount("smaya", "smayapub")
		config.SetBech32PrefixForValidator("smayav", "smayavpub")
		config.SetBech32PrefixForConsensusNode("smayac", "smayacpub")
		config.Seal()
	case common.MainNet:
		fmt.Println("BASEChain mainnet:")
		config := cosmos.GetConfig()
		config.SetBech32PrefixForAccount("maya", "mayapub")
		config.SetBech32PrefixForValidator("mayav", "mayavpub")
		config.SetBech32PrefixForConsensusNode("mayac", "mayacpub")
		config.Seal()
	}

	pk, err := common.NewPubKey(*raw)
	if err != nil {
		panic(err)
	}

	for _, chain := range common.AllChains {
		addr, err := pk.GetAddress(chain)
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s Address: %s\n", chain.String(), addr)
	}
}
