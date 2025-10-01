package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/binance-chain/tss-lib/crypto/vss"
	"github.com/btcsuite/btcd/btcec"
	. "github.com/decred/dcrd/dcrec/secp256k1"
)

func main() {
	n := *(flag.Int("n", 3, "signing party size"))
	threshold := n - 1
	export := flag.String("export", "", "path to export keyfile")
	password := flag.String("password", "", "encryption password for keyfile")
	flag.Parse()
	files := flag.Args()

	setupBech32Prefix()
	allSecret := make([]KeygenLocalState, len(files))
	for i, f := range files {
		tssSecret, err := getTssSecretFile(f)
		if err != nil {
			fmt.Printf("---%v\n", err)
		}
		allSecret[i] = tssSecret
	}

	vssShares := make(vss.Shares, len(allSecret))

	for i, el := range allSecret {
		share := vss.Share{
			Threshold: threshold,
			ID:        el.LocalData.ShareID,
			Share:     el.LocalData.Xi,
		}
		vssShares[i] = &share
	}

	tssPrivateKey, err := vssShares[:n].ReConstruct(btcec.S256())
	if err != nil {
		fmt.Printf("error in tss verify: %v\n", err)
		os.Exit(1)
	}

	privKey := NewPrivateKey(tssPrivateKey)

	pk := privKey.PubKey()
	mayachainpk, address, err := getTssPubKey(pk.X, pk.Y)
	if err != nil {
		fmt.Printf("--->%v", err)
	}
	fmt.Printf("---recoverd sk:%v\n", privKey)
	fmt.Printf("---recoverd pk:%v\n", mayachainpk)
	fmt.Printf("-------%v\n", address)

	if len(*export) > 0 && len(*password) >= 8 {
		keyfile, err := exportKeyStore(privKey.Serialize(), *password)
		if err != nil {
			fmt.Printf("--->%v", err)
		}

		jsonString, _ := json.Marshal(keyfile)
		err = ioutil.WriteFile(*export, jsonString, os.ModePerm)
		if err != nil {
			fmt.Printf("--->%v", err)
		}
		fmt.Printf("---wrote to: %+v\n", *export)
	}
}
