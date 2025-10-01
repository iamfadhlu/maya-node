//go:build stagenet
// +build stagenet

package config

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/rs/zerolog/log"
)

const (
	rpcPort = 27147
	p2pPort = 27146
)

func getSeedAddrs() (addrs []string) {
	// fetch seeds
	res, err := http.Get("http://seed.stagenet.mayachain.info")
	if err != nil {
		log.Error().Err(err).Msg("failed to get seeds")
		return
	}

	// unmarshal seeds response
	var seedsResponse []string
	dec := json.NewDecoder(res.Body)
	err = dec.Decode(&seedsResponse)
	if err != nil {
		log.Error().Err(err).Msg("failed to unmarshal seeds response")
	}

	return seedsResponse
}

func assertBifrostHasSeeds() {
	// fail if seed file is missing or empty since bifrost will hang
	seedPath := os.ExpandEnv("$HOME/.mayanode/address_book.seed")
	fi, err := os.Stat(seedPath)
	if os.IsNotExist(err) {
		log.Warn().Msg("no seed file found")
	}
	if err != nil {
		log.Warn().Err(err).Msg("failed to stat seed file")
		return
	}
	if fi.Size() == 0 {
		log.Warn().Msg("seed file is empty")
	}
}
