package common

import (
	"fmt"
	"regexp"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/bech32"
	eth "github.com/ethereum/go-ethereum/common"
	ret "github.com/radixdlt/radix-engine-toolkit-go/v2/radix_engine_toolkit_uniffi"
	dashutil "gitlab.com/mayachain/dashd-go/btcutil"
	dashchaincfg "gitlab.com/mayachain/dashd-go/chaincfg"
)

var alphaNumRegex = regexp.MustCompile("^[:A-Za-z0-9]*$")

// NewAddress create a new Address. Supports Binance, Bitcoin, and Ethereum
func NewAddressV1(address string) (Address, error) {
	if len(address) == 0 {
		return NoAddress, nil
	}

	if !alphaNumRegex.MatchString(address) {
		return NoAddress, fmt.Errorf("address format not supported: %s", address)
	}

	// Check is eth address
	if eth.IsHexAddress(address) {
		return Address(address), nil
	}

	// Check bech32 addresses, would succeed any string bech32 encoded (e.g. MAYA, THOR, BNB, ATOM)
	_, _, err := bech32.Decode(address)
	if err == nil {
		return Address(address), nil
	}

	// Check other BTC address formats with mainnet
	_, err = btcutil.DecodeAddress(address, &chaincfg.MainNetParams)
	if err == nil {
		return Address(address), nil
	}

	// Check BTC address formats with testnet
	_, err = btcutil.DecodeAddress(address, &chaincfg.TestNet3Params)
	if err == nil {
		return Address(address), nil
	}

	// Check DASH address formats with mainnet
	_, err = dashutil.DecodeAddress(address, &dashchaincfg.MainNetParams)
	if err == nil {
		return Address(address), nil
	}

	// Check DASH address formats with testnet
	_, err = dashutil.DecodeAddress(address, &dashchaincfg.TestNet3Params)
	if err == nil {
		return Address(address), nil
	}

	// Check DASH address formats with mocknet
	_, err = dashutil.DecodeAddress(address, &dashchaincfg.RegressionNetParams)
	if err == nil {
		return Address(address), nil
	}

	return NoAddress, fmt.Errorf("address format not supported: %s", address)
}

// NewAddress create a new Address. Supports Binance, Bitcoin, and Ethereum
func NewAddressV111(address string) (Address, error) {
	if len(address) == 0 {
		return NoAddress, nil
	}

	if !addressRegexV111.MatchString(address) {
		return NoAddress, fmt.Errorf("address format not supported: %s", address)
	}

	// Check is eth address
	if eth.IsHexAddress(address) {
		return Address(address), nil
	}

	// Check bech32 addresses, would succeed any string bech32 encoded (e.g. MAYA, THOR, BNB, ATOM)
	_, _, err := bech32.Decode(address)
	if err == nil {
		return Address(address), nil
	}

	// Check other BTC address formats with mainnet
	_, err = btcutil.DecodeAddress(address, &chaincfg.MainNetParams)
	if err == nil {
		return Address(address), nil
	}

	// Check BTC address formats with testnet
	_, err = btcutil.DecodeAddress(address, &chaincfg.TestNet3Params)
	if err == nil {
		return Address(address), nil
	}

	// Check DASH address formats with mainnet
	_, err = dashutil.DecodeAddress(address, &dashchaincfg.MainNetParams)
	if err == nil {
		return Address(address), nil
	}

	// Check DASH address formats with testnet
	_, err = dashutil.DecodeAddress(address, &dashchaincfg.TestNet3Params)
	if err == nil {
		return Address(address), nil
	}

	// Check DASH address formats with mocknet
	_, err = dashutil.DecodeAddress(address, &dashchaincfg.RegressionNetParams)
	if err == nil {
		return Address(address), nil
	}

	// Check XRD address formats including abbreviated one
	processedAddr := tryUnabbreviateAddress(address, XRDChain)
	_, err = ret.NewAddress(processedAddr)
	if err == nil {
		return Address(processedAddr), nil
	}

	return NoAddress, fmt.Errorf("address format not supported: %s", address)
}
