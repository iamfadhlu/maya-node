package common

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/blang/semver"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/bech32"
	eth "github.com/ethereum/go-ethereum/common"
	ret "github.com/radixdlt/radix-engine-toolkit-go/v2/radix_engine_toolkit_uniffi"
	dashutil "gitlab.com/mayachain/dashd-go/btcutil"
	dashchaincfg "gitlab.com/mayachain/dashd-go/chaincfg"

	"gitlab.com/mayachain/mayanode/chain/zec/go/zec"

	"gitlab.com/mayachain/mayanode/common/cosmos"
)

type Address string

const (
	NoAddress      = Address("")
	NoopAddress    = Address("noop")
	EVMNullAddress = Address("0x0000000000000000000000000000000000000000")
)

var addressRegexV111 = regexp.MustCompile("^[:_A-Za-z0-9]*$")

func NewAddress(address string, version semver.Version) (Address, error) {
	switch {
	case version.GTE(semver.MustParse("1.120.0")): // zcash
		return NewAddressV120(address)
	case version.GTE(semver.MustParse("1.111.0")):
		return NewAddressV111(address)
	default:
		return NewAddressV1(address)
	}
}

// NewAddress create a new Address. Supports Binance, Bitcoin, and Ethereum
func NewAddressV120(address string) (Address, error) {
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

	err = zec.ValidateAddress(address, zec.MainNetParams.Net)
	if err == nil {
		return Address(address), nil
	}
	err = zec.ValidateAddress(address, zec.RegtestNetParams.Net)
	if err == nil {
		return Address(address), nil
	}
	return NoAddress, fmt.Errorf("address format not supported: %s, err: %w", address, err)
}

// tryUnabbreviateAddress try to unabbreviate the address with specific rule
func tryUnabbreviateAddress(addr string, chain Chain) string {
	if chain == XRDChain {
		// unabbreviate into an account address, but only if this isn't already a valid address
		if _, err := ret.NewAddress(addr); err != nil {
			addrWithoutPrefix := addr
			match := false
			for _, nextPrefix := range RadixPrefixAbbreviations {
				addrWithoutPrefix, match = strings.CutPrefix(addr, nextPrefix)
				if match {
					break
				}
			}
			return RadixAccountAddressPrefix + addrWithoutPrefix
		}
	}

	return addr
}

func (addr Address) IsChain(chain Chain, version semver.Version) bool {
	switch {
	case version.GTE(semver.MustParse("1.120.0")): // zcash
		return addr.IsChainV120(chain)
	case version.GTE(semver.MustParse("1.112.0")):
		return addr.IsChainV112(chain)
	case version.GTE(semver.MustParse("1.111.0")):
		return addr.IsChainV111(chain)
	case version.GTE(semver.MustParse("1.108.0")):
		return addr.IsChainV108(chain)
	default:
		return addr.IsChainV107(chain)
	}
}

func (addr Address) IsChainV120(chain Chain) bool {
	if addr.String() == "" {
		return false
	}

	if chain.IsEVM() {
		return strings.HasPrefix(addr.String(), "0x")
	}
	switch chain {
	case BNBChain:
		prefix, _, _ := bech32.Decode(addr.String())
		return prefix == "bnb" || prefix == "tbnb"
	case AZTECChain:
		prefix, _, _ := bech32.Decode(addr.String())
		return prefix == "aztec" || prefix == "taztec" || prefix == "saztec"
	case BASEChain:
		prefix, _, _ := bech32.Decode(addr.String())
		return prefix == "maya" || prefix == "tmaya" || prefix == "smaya"
	case THORChain:
		prefix, _, _ := bech32.Decode(addr.String())
		return prefix == "thor" || prefix == "tthor"
	case KUJIChain:
		prefix, _, _ := bech32.Decode(addr.String())
		return prefix == "kujira"
	case BTCChain:
		prefix, _, err := bech32.Decode(addr.String())
		if err == nil && (prefix == "bc" || prefix == "tb") {
			return true
		}
		// Check mainnet other formats
		_, err = btcutil.DecodeAddress(addr.String(), &chaincfg.MainNetParams)
		if err == nil {
			return true
		}
		// Check testnet other formats
		_, err = btcutil.DecodeAddress(addr.String(), &chaincfg.TestNet3Params)
		if err == nil {
			return true
		}
		return false
	case DASHChain:
		// Check mainnet other formats
		_, err := dashutil.DecodeAddress(addr.String(), &dashchaincfg.MainNetParams)
		if err == nil {
			return true
		}
		// Check testnet other formats
		_, err = dashutil.DecodeAddress(addr.String(), &dashchaincfg.TestNet3Params)
		if err == nil {
			return true
		}
		// Check mocknet / regression other formats
		_, err = dashutil.DecodeAddress(addr.String(), &dashchaincfg.RegressionNetParams)
		if err == nil {
			return true
		}
		return false
	case XRDChain:
		_, err := ret.NewAddress(addr.String())
		return err == nil
	case ZECChain:
		err := zec.ValidateAddress(addr.String(), zec.MainNetParams.Net)
		if err == nil {
			return true
		}
		err = zec.ValidateAddress(addr.String(), zec.RegtestNetParams.Net)
		if err == nil {
			return true
		}
		return false
	default:
		return true // if THORNode don't specifically check a chain yet, assume its ok.
	}
}

func (addr Address) IsChainV112(chain Chain) bool {
	if addr.String() == "" {
		return false
	}

	if chain.IsEVM() {
		return strings.HasPrefix(addr.String(), "0x")
	}
	switch chain {
	case BNBChain:
		prefix, _, _ := bech32.Decode(addr.String())
		return prefix == "bnb" || prefix == "tbnb"
	case AZTECChain:
		prefix, _, _ := bech32.Decode(addr.String())
		return prefix == "aztec" || prefix == "taztec" || prefix == "saztec"
	case BASEChain:
		prefix, _, _ := bech32.Decode(addr.String())
		return prefix == "maya" || prefix == "tmaya" || prefix == "smaya"
	case THORChain:
		prefix, _, _ := bech32.Decode(addr.String())
		return prefix == "thor" || prefix == "tthor"
	case KUJIChain:
		prefix, _, _ := bech32.Decode(addr.String())
		return prefix == "kujira"
	case BTCChain:
		prefix, _, err := bech32.Decode(addr.String())
		if err == nil && (prefix == "bc" || prefix == "tb") {
			return true
		}
		// Check mainnet other formats
		_, err = btcutil.DecodeAddress(addr.String(), &chaincfg.MainNetParams)
		if err == nil {
			return true
		}
		// Check testnet other formats
		_, err = btcutil.DecodeAddress(addr.String(), &chaincfg.TestNet3Params)
		if err == nil {
			return true
		}
		return false
	case DASHChain:
		// Check mainnet other formats
		_, err := dashutil.DecodeAddress(addr.String(), &dashchaincfg.MainNetParams)
		if err == nil {
			return true
		}
		// Check testnet other formats
		_, err = dashutil.DecodeAddress(addr.String(), &dashchaincfg.TestNet3Params)
		if err == nil {
			return true
		}
		// Check mocknet / regression other formats
		_, err = dashutil.DecodeAddress(addr.String(), &dashchaincfg.RegressionNetParams)
		if err == nil {
			return true
		}
		return false
	case XRDChain:
		_, err := ret.NewAddress(addr.String())
		return err == nil
	default:
		return true // if THORNode don't specifically check a chain yet, assume its ok.
	}
}

func (addr Address) IsChainV111(chain Chain) bool {
	if chain.IsEVM() {
		return strings.HasPrefix(addr.String(), "0x")
	}
	switch chain {
	case BNBChain:
		prefix, _, _ := bech32.Decode(addr.String())
		return prefix == "bnb" || prefix == "tbnb"
	case AZTECChain:
		prefix, _, _ := bech32.Decode(addr.String())
		return prefix == "aztec" || prefix == "taztec" || prefix == "saztec"
	case BASEChain:
		prefix, _, _ := bech32.Decode(addr.String())
		return prefix == "maya" || prefix == "tmaya" || prefix == "smaya"
	case THORChain:
		prefix, _, _ := bech32.Decode(addr.String())
		return prefix == "thor" || prefix == "tthor"
	case KUJIChain:
		prefix, _, _ := bech32.Decode(addr.String())
		return prefix == "kujira"
	case BTCChain:
		prefix, _, err := bech32.Decode(addr.String())
		if err == nil && (prefix == "bc" || prefix == "tb") {
			return true
		}
		// Check mainnet other formats
		_, err = btcutil.DecodeAddress(addr.String(), &chaincfg.MainNetParams)
		if err == nil {
			return true
		}
		// Check testnet other formats
		_, err = btcutil.DecodeAddress(addr.String(), &chaincfg.TestNet3Params)
		if err == nil {
			return true
		}
		return false
	case DASHChain:
		// Check mainnet other formats
		_, err := dashutil.DecodeAddress(addr.String(), &dashchaincfg.MainNetParams)
		if err == nil {
			return true
		}
		// Check testnet other formats
		_, err = dashutil.DecodeAddress(addr.String(), &dashchaincfg.TestNet3Params)
		if err == nil {
			return true
		}
		// Check mocknet / regression other formats
		_, err = dashutil.DecodeAddress(addr.String(), &dashchaincfg.RegressionNetParams)
		if err == nil {
			return true
		}
		return false
	case XRDChain:
		_, err := ret.NewAddress(addr.String())
		return err == nil
	default:
		return true // if THORNode don't specifically check a chain yet, assume its ok.
	}
}

func (addr Address) IsChainV108(chain Chain) bool {
	if chain.IsEVM() {
		return strings.HasPrefix(addr.String(), "0x")
	}
	switch chain {
	case BNBChain:
		prefix, _, _ := bech32.Decode(addr.String())
		return prefix == "bnb" || prefix == "tbnb"
	case AZTECChain:
		prefix, _, _ := bech32.Decode(addr.String())
		return prefix == "aztec" || prefix == "taztec" || prefix == "saztec"
	case BASEChain:
		prefix, _, _ := bech32.Decode(addr.String())
		return prefix == "maya" || prefix == "tmaya" || prefix == "smaya"
	case THORChain:
		prefix, _, _ := bech32.Decode(addr.String())
		return prefix == "thor" || prefix == "tthor"
	case KUJIChain:
		prefix, _, _ := bech32.Decode(addr.String())
		return prefix == "kujira"
	case BTCChain:
		prefix, _, err := bech32.Decode(addr.String())
		if err == nil && (prefix == "bc" || prefix == "tb") {
			return true
		}
		// Check mainnet other formats
		_, err = btcutil.DecodeAddress(addr.String(), &chaincfg.MainNetParams)
		if err == nil {
			return true
		}
		// Check testnet other formats
		_, err = btcutil.DecodeAddress(addr.String(), &chaincfg.TestNet3Params)
		if err == nil {
			return true
		}
		return false
	case DASHChain:
		// Check mainnet other formats
		_, err := dashutil.DecodeAddress(addr.String(), &dashchaincfg.MainNetParams)
		if err == nil {
			return true
		}
		// Check testnet other formats
		_, err = dashutil.DecodeAddress(addr.String(), &dashchaincfg.TestNet3Params)
		if err == nil {
			return true
		}
		// Check mocknet / regression other formats
		_, err = dashutil.DecodeAddress(addr.String(), &dashchaincfg.RegressionNetParams)
		if err == nil {
			return true
		}
		return false
	default:
		return true // if THORNode don't specifically check a chain yet, assume its ok.
	}
}

func (addr Address) IsChainV107(chain Chain) bool {
	if chain.IsEVM() {
		return strings.HasPrefix(addr.String(), "0x")
	}
	switch chain {
	case BNBChain:
		prefix, _, _ := bech32.Decode(addr.String())
		return prefix == "bnb" || prefix == "tbnb"
	case AZTECChain:
		prefix, _, _ := bech32.Decode(addr.String())
		return prefix == "aztec" || prefix == "taztec" || prefix == "saztec"
	case BASEChain:
		prefix, _, _ := bech32.Decode(addr.String())
		return prefix == "maya" || prefix == "tmaya" || prefix == "smaya"
	case THORChain:
		prefix, _, _ := bech32.Decode(addr.String())
		return prefix == "thor" || prefix == "tthor"
	case BTCChain:
		prefix, _, err := bech32.Decode(addr.String())
		if err == nil && (prefix == "bc" || prefix == "tb") {
			return true
		}
		// Check mainnet other formats
		_, err = btcutil.DecodeAddress(addr.String(), &chaincfg.MainNetParams)
		if err == nil {
			return true
		}
		// Check testnet other formats
		_, err = btcutil.DecodeAddress(addr.String(), &chaincfg.TestNet3Params)
		if err == nil {
			return true
		}
		return false
	case DASHChain:
		// Check mainnet other formats
		_, err := dashutil.DecodeAddress(addr.String(), &dashchaincfg.MainNetParams)
		if err == nil {
			return true
		}
		// Check testnet other formats
		_, err = dashutil.DecodeAddress(addr.String(), &dashchaincfg.TestNet3Params)
		if err == nil {
			return true
		}
		// Check mocknet / regression other formats
		_, err = dashutil.DecodeAddress(addr.String(), &dashchaincfg.RegressionNetParams)
		if err == nil {
			return true
		}
		return false
	case XRDChain:
		_, err := ret.NewAddress(addr.String())
		return err == nil
	default:
		return true // if THORNode don't specifically check a chain yet, assume its ok.
	}
}

func (addr Address) GetChain(version semver.Version) Chain {
	switch {
	case version.GTE(semver.MustParse("1.120.0")): // zcash
		return addr.getChainV120(version)
	case version.GTE(semver.MustParse("1.112.0")):
		return addr.getChainV112(version)
	case version.GTE(semver.MustParse("1.109.0")):
		return addr.getChainV109(version)
	case version.GTE(semver.MustParse("1.107.0")):
		return addr.getChainV107(version)
	default:
		return addr.getChainV105(version)
	}
}

func (addr Address) getChainV120(version semver.Version) Chain {
	for _, chain := range []Chain{ETHChain, BNBChain, BASEChain, BTCChain, DASHChain, THORChain, KUJIChain, AVAXChain, ARBChain, XRDChain, ZECChain} {
		if addr.IsChain(chain, version) {
			return chain
		}
	}
	return EmptyChain
}

func (addr Address) getChainV112(version semver.Version) Chain {
	for _, chain := range []Chain{ETHChain, BNBChain, BASEChain, BTCChain, DASHChain, THORChain, KUJIChain, AVAXChain, ARBChain, XRDChain} {
		if addr.IsChain(chain, version) {
			return chain
		}
	}
	return EmptyChain
}

func (addr Address) getChainV109(version semver.Version) Chain {
	for _, chain := range []Chain{ETHChain, BNBChain, BASEChain, BTCChain, DASHChain, THORChain, KUJIChain, AVAXChain, ARBChain} {
		if addr.IsChain(chain, version) {
			return chain
		}
	}
	return EmptyChain
}

func (addr Address) getChainV107(version semver.Version) Chain {
	for _, chain := range []Chain{ETHChain, BNBChain, BASEChain, BTCChain, DASHChain, THORChain, KUJIChain, AVAXChain} {
		if addr.IsChain(chain, version) {
			return chain
		}
	}
	return EmptyChain
}

func (addr Address) getChainV105(version semver.Version) Chain {
	for _, chain := range []Chain{ETHChain, BNBChain, BASEChain, BTCChain, DASHChain, BASEChain, AVAXChain} {
		if addr.IsChain(chain, version) {
			return chain
		}
	}
	return EmptyChain
}

func (addr Address) GetNetwork(version semver.Version, chain Chain) ChainNetwork {
	switch {
	case version.GTE(semver.MustParse("1.120.0")): // zcash
		return addr.getNetworkV120(chain)
	case version.GTE(semver.MustParse("1.111.0")):
		return addr.getNetworkV111(chain)
	default:
		return addr.getNetworkV1(version, chain)
	}
}

func (addr Address) getNetworkV120(chain Chain) ChainNetwork {
	mainNetPredicate := func() ChainNetwork {
		if CurrentChainNetwork == StageNet {
			return StageNet
		}
		return MainNet
	}
	if addr == NoAddress {
		return mainNetPredicate()
	}
	// EVM addresses don't have different prefixes per network
	if chain.IsEVM() {
		return CurrentChainNetwork
	}
	switch chain {
	case BNBChain:
		prefix, _, _ := bech32.Decode(addr.String())
		if strings.EqualFold(prefix, "bnb") {
			return mainNetPredicate()
		}
		if strings.EqualFold(prefix, "tbnb") {
			return TestNet
		}
	case AZTECChain:
		return CurrentChainNetwork
	case BASEChain:
		prefix, _, _ := bech32.Decode(addr.String())
		if strings.EqualFold(prefix, "maya") {
			return mainNetPredicate()
		}
		if strings.EqualFold(prefix, "tmaya") {
			return TestNet
		}
		if strings.EqualFold(prefix, "smaya") {
			return StageNet
		}
	case KUJIChain:
		return CurrentChainNetwork
	case THORChain:
		prefix, _, _ := bech32.Decode(addr.String())
		if strings.EqualFold(prefix, "thor") {
			return mainNetPredicate()
		}
		if strings.EqualFold(prefix, "tthor") {
			return TestNet
		}
	case BTCChain:
		prefix, _, _ := bech32.Decode(addr.String())
		switch prefix {
		case "bc":
			return mainNetPredicate()
		case "tb":
			return TestNet
		case "bcrt":
			return MockNet
		default:
			_, err := btcutil.DecodeAddress(addr.String(), &chaincfg.MainNetParams)
			if err == nil {
				return mainNetPredicate()
			}
			_, err = btcutil.DecodeAddress(addr.String(), &chaincfg.TestNet3Params)
			if err == nil {
				return TestNet
			}
			_, err = btcutil.DecodeAddress(addr.String(), &chaincfg.RegressionNetParams)
			if err == nil {
				return MockNet
			}
		}
	case DASHChain:
		// Check mainnet other formats
		_, err := dashutil.DecodeAddress(addr.String(), &dashchaincfg.MainNetParams)
		if err == nil {
			return mainNetPredicate()
		}
		// Check testnet other formats
		_, err = dashutil.DecodeAddress(addr.String(), &dashchaincfg.TestNet3Params)
		if err == nil {
			return TestNet
		}
		// Check mocknet / regression other formats
		_, err = dashutil.DecodeAddress(addr.String(), &dashchaincfg.RegressionNetParams)
		if err == nil {
			return MockNet
		}
	case XRDChain:
		retAddr, err := ret.NewAddress(addr.String())
		if err != nil {
			return mainNetPredicate()
		}
		switch retAddr.NetworkId() {
		case 1:
			return mainNetPredicate()
		case 2:
			return StageNet
		case 34:
			return TestNet
		case 240:
			return MockNet
		default:
			return mainNetPredicate()
		}
	case ZECChain:
		// Check testnet prefixes
		if strings.HasPrefix(addr.String(), "tm") ||
			strings.HasPrefix(addr.String(), "t2") ||
			strings.HasPrefix(addr.String(), "ztestsapling") ||
			strings.HasPrefix(addr.String(), "uregtest") ||
			strings.HasPrefix(addr.String(), "utest") {
			return TestNet
		}
		// All other valid Zcash addresses are mainnet
		if addr.IsChain(ZECChain, LatestVersion) {
			return mainNetPredicate()
		}
	}
	return CurrentChainNetwork
}

func (addr Address) getNetworkV111(chain Chain) ChainNetwork {
	mainNetPredicate := func() ChainNetwork {
		if CurrentChainNetwork == StageNet {
			return StageNet
		}
		return MainNet
	}
	if addr == NoAddress {
		return mainNetPredicate()
	}
	// EVM addresses don't have different prefixes per network
	if chain.IsEVM() {
		return CurrentChainNetwork
	}
	switch chain {
	case BNBChain:
		prefix, _, _ := bech32.Decode(addr.String())
		if strings.EqualFold(prefix, "bnb") {
			return mainNetPredicate()
		}
		if strings.EqualFold(prefix, "tbnb") {
			return TestNet
		}
	case AZTECChain:
		return CurrentChainNetwork
	case BASEChain:
		prefix, _, _ := bech32.Decode(addr.String())
		if strings.EqualFold(prefix, "maya") {
			return mainNetPredicate()
		}
		if strings.EqualFold(prefix, "tmaya") {
			return TestNet
		}
		if strings.EqualFold(prefix, "smaya") {
			return StageNet
		}
	case KUJIChain:
		return CurrentChainNetwork
	case THORChain:
		prefix, _, _ := bech32.Decode(addr.String())
		if strings.EqualFold(prefix, "thor") {
			return mainNetPredicate()
		}
		if strings.EqualFold(prefix, "tthor") {
			return TestNet
		}
	case BTCChain:
		prefix, _, _ := bech32.Decode(addr.String())
		switch prefix {
		case "bc":
			return mainNetPredicate()
		case "tb":
			return TestNet
		case "bcrt":
			return MockNet
		default:
			_, err := btcutil.DecodeAddress(addr.String(), &chaincfg.MainNetParams)
			if err == nil {
				return mainNetPredicate()
			}
			_, err = btcutil.DecodeAddress(addr.String(), &chaincfg.TestNet3Params)
			if err == nil {
				return TestNet
			}
			_, err = btcutil.DecodeAddress(addr.String(), &chaincfg.RegressionNetParams)
			if err == nil {
				return MockNet
			}
		}
	case DASHChain:
		// Check mainnet other formats
		_, err := dashutil.DecodeAddress(addr.String(), &dashchaincfg.MainNetParams)
		if err == nil {
			return mainNetPredicate()
		}
		// Check testnet other formats
		_, err = dashutil.DecodeAddress(addr.String(), &dashchaincfg.TestNet3Params)
		if err == nil {
			return TestNet
		}
		// Check mocknet / regression other formats
		_, err = dashutil.DecodeAddress(addr.String(), &dashchaincfg.RegressionNetParams)
		if err == nil {
			return MockNet
		}
	case XRDChain:
		retAddr, err := ret.NewAddress(addr.String())
		if err != nil {
			return mainNetPredicate()
		}
		switch retAddr.NetworkId() {
		case 1:
			return mainNetPredicate()
		case 2:
			return StageNet
		case 34:
			return TestNet
		case 240:
			return MockNet
		default:
			return mainNetPredicate()
		}
	}
	return CurrentChainNetwork
}

func (addr Address) getNetworkV1(ver semver.Version, chain Chain) ChainNetwork {
	mainNetPredicate := func() ChainNetwork {
		if CurrentChainNetwork == StageNet {
			return StageNet
		}
		return MainNet
	}
	// EVM addresses don't have different prefixes per network
	if chain.IsEVM() {
		return CurrentChainNetwork
	}
	switch chain {
	case BNBChain:
		prefix, _, _ := bech32.Decode(addr.String())
		if strings.EqualFold(prefix, "bnb") {
			return mainNetPredicate()
		}
		if strings.EqualFold(prefix, "tbnb") {
			return TestNet
		}
	case AZTECChain:
		return CurrentChainNetwork
	case BASEChain:
		prefix, _, _ := bech32.Decode(addr.String())
		if strings.EqualFold(prefix, "maya") {
			return mainNetPredicate()
		}
		if strings.EqualFold(prefix, "tmaya") {
			return TestNet
		}
		if strings.EqualFold(prefix, "smaya") {
			return StageNet
		}
	case KUJIChain:
		return CurrentChainNetwork
	case THORChain:
		prefix, _, _ := bech32.Decode(addr.String())
		if strings.EqualFold(prefix, "thor") {
			return mainNetPredicate()
		}
		if strings.EqualFold(prefix, "tthor") {
			return TestNet
		}
	case BTCChain:
		prefix, _, _ := bech32.Decode(addr.String())
		switch prefix {
		case "bc":
			return mainNetPredicate()
		case "tb":
			return TestNet
		case "bcrt":
			return MockNet
		default:
			_, err := btcutil.DecodeAddress(addr.String(), &chaincfg.MainNetParams)
			if err == nil {
				return mainNetPredicate()
			}
			_, err = btcutil.DecodeAddress(addr.String(), &chaincfg.TestNet3Params)
			if err == nil {
				return TestNet
			}
			_, err = btcutil.DecodeAddress(addr.String(), &chaincfg.RegressionNetParams)
			if err == nil {
				return MockNet
			}
		}
	case DASHChain:
		// Check mainnet other formats
		_, err := dashutil.DecodeAddress(addr.String(), &dashchaincfg.MainNetParams)
		if err == nil {
			return mainNetPredicate()
		}
		// Check testnet other formats
		_, err = dashutil.DecodeAddress(addr.String(), &dashchaincfg.TestNet3Params)
		if err == nil {
			return TestNet
		}
		// Check mocknet / regression other formats
		_, err = dashutil.DecodeAddress(addr.String(), &dashchaincfg.RegressionNetParams)
		if err == nil {
			return MockNet
		}
	}
	switch {
	case ver.GTE(semver.MustParse("1.93.0")):
		return CurrentChainNetwork
	default:
		return MockNet
	}
}

func (addr Address) AccAddress() (cosmos.AccAddress, error) {
	return cosmos.AccAddressFromBech32(addr.String())
}

func (addr Address) Equals(addr2 Address) bool {
	return strings.EqualFold(addr.String(), addr2.String())
}

func (addr Address) IsEmpty() bool {
	return strings.TrimSpace(addr.String()) == ""
}

func (addr Address) IsNoop() bool {
	return addr.Equals(NoopAddress)
}

func (addr Address) String() string {
	return string(addr)
}

func (addr Address) AbbreviatedString(version semver.Version) string {
	if addr.GetChain(version) == XRDChain {
		abbreviatedAddr, _ := strings.CutPrefix(addr.String(), RadixAccountAddressPrefix)
		return abbreviatedAddr
	}

	return addr.String()
}
