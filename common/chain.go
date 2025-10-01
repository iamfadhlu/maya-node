package common

import (
	"errors"
	"strings"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/hashicorp/go-multierror"
	dashchaincfg "gitlab.com/mayachain/dashd-go/chaincfg"
	btypes "gitlab.com/thorchain/binance-sdk/common/types"

	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

var (
	EmptyChain = Chain("")
	BNBChain   = Chain("BNB")
	ETHChain   = Chain("ETH")
	BTCChain   = Chain("BTC")
	DASHChain  = Chain("DASH")
	BASEChain  = Chain("MAYA")
	AZTECChain = Chain("AZTEC")
	THORChain  = Chain("THOR")
	AVAXChain  = Chain("AVAX")
	KUJIChain  = Chain("KUJI")
	ARBChain   = Chain("ARB")
	XRDChain   = Chain("XRD")
	ZECChain   = Chain("ZEC")

	SigningAlgoSecp256k1 = SigningAlgo("secp256k1")
	SigningAlgoEd25519   = SigningAlgo("ed25519")
)

var AllChains = Chains{
	ETHChain,
	BTCChain,
	DASHChain,
	BASEChain,
	THORChain,
	KUJIChain,
	ARBChain,
	XRDChain,
	ZECChain,
}

type SigningAlgo string

type Chain string

// Chains represent a slice of Chain
type Chains []Chain

// Validate validates chain format, should consist only of uppercase letters
func (c Chain) Validate() error {
	if len(c) < 3 {
		return errors.New("chain id len is less than 3")
	}
	if len(c) > 10 {
		return errors.New("chain id len is more than 10")
	}
	for _, ch := range string(c) {
		if ch < 'A' || ch > 'Z' {
			return errors.New("chain id can consist only of uppercase letters")
		}
	}
	return nil
}

// Valid validates chain format, should consist only of uppercase letters
func (c Chain) Valid() error {
	if len(c) < 3 {
		return errors.New("chain id len is less than 3")
	}
	if len(c) > 10 {
		return errors.New("chain id len is more than 10")
	}
	for _, ch := range string(c) {
		if ch < 'A' || ch > 'Z' {
			return errors.New("chain id can consist only of uppercase letters")
		}
	}
	return nil
}

// NewChain create a new Chain and default the siging_algo to Secp256k1
func NewChain(chainID string) (Chain, error) {
	chain := Chain(strings.ToUpper(chainID))
	if err := chain.Validate(); err != nil {
		return chain, err
	}
	return chain, nil
}

// Equals compare two chain to see whether they represent the same chain
func (c Chain) Equals(c2 Chain) bool {
	return strings.EqualFold(c.String(), c2.String())
}

func (c Chain) IsBASEChain() bool {
	return c.Equals(BASEChain)
}

// GetEVMChains returns all "EVM" chains connected to THORChain
// "EVM" is defined, in thornode's context, as a chain that:
// - uses 0x as an address prefix
// - has a "Router" Smart Contract
func GetEVMChains() []Chain {
	return []Chain{ETHChain, AVAXChain, ARBChain}
}

// GetUTXOChains returns all "UTXO" chains connected to THORChain.
func GetUTXOChains() []Chain {
	return []Chain{BTCChain, DASHChain}
}

// IsEVM returns true if given chain is an EVM chain.
// See working definition of an "EVM" chain in the
// `GetEVMChains` function description
func (c Chain) IsEVM() bool {
	evmChains := GetEVMChains()
	for _, evm := range evmChains {
		if c.Equals(evm) {
			return true
		}
	}
	return false
}

// IsARB determinate whether it is ARBChain
func (c Chain) IsARB() bool {
	return c.Equals(ARBChain)
}

// IsUTXO returns true if given chain is a UTXO chain.
func (c Chain) IsUTXO() bool {
	utxoChains := GetUTXOChains()
	for _, utxo := range utxoChains {
		if c.Equals(utxo) {
			return true
		}
	}
	return false
}

// IsEmpty is to determinate whether the chain is empty
func (c Chain) IsEmpty() bool {
	return strings.TrimSpace(c.String()) == ""
}

// String implement fmt.Stringer
func (c Chain) String() string {
	// convert it to upper case again just in case someone created a ticker via Chain("rune")
	return strings.ToUpper(string(c))
}

// IsBNB determinate whether it is BNBChain
func (c Chain) IsBNB() bool {
	return c.Equals(BNBChain)
}

// GetSigningAlgo get the signing algorithm for the given chain
func (c Chain) GetSigningAlgo() SigningAlgo {
	// Only SigningAlgoSecp256k1 is supported for now
	return SigningAlgoSecp256k1
}

// GetGasAsset chain's base asset
func (c Chain) GetGasAsset() Asset {
	switch c {
	case AZTECChain:
		return BaseNative
	case BASEChain:
		return BaseNative
	case BNBChain:
		return BNBAsset
	case BTCChain:
		return BTCAsset
	case DASHChain:
		return DASHAsset
	case ETHChain:
		return ETHAsset
	case THORChain:
		return RUNEAsset
	case AVAXChain:
		return AVAXAsset
	case KUJIChain:
		return KUJIAsset
	case ARBChain:
		return AETHAsset
	case XRDChain:
		return XRDAsset
	case ZECChain:
		return ZECAsset
	default:
		return EmptyAsset
	}
}

// GetGasUnits returns name of the gas unit for each chain
func (c Chain) GetGasUnits() string {
	switch c {
	case AVAXChain:
		return "nAVAX"
	case BNBChain:
		return "ubnb"
	case BTCChain:
		return "satsperbyte"
	case DASHChain:
		return "duffsperbyte"
	case ETHChain:
		return "gwei"
	case ARBChain:
		return "centigwei"
	case KUJIChain:
		return "ukuji"
	case THORChain:
		return "rune"
	case XRDChain:
		return "costunits"
	case ZECChain:
		return "satsperbyte"
	default:
		return ""
	}
}

// GetGasAssetDecimal for the gas asset of given chain , what kind of precision it is using
// BASEChain is using 1E8, if an external chain's gas asset is larger than 1E8, just return cosmos.DefaultCoinDecimals
func (c Chain) GetGasAssetDecimal() int64 {
	switch c {
	case BASEChain, THORChain:
		return 8
	case KUJIChain:
		return 6
	default:
		return cosmos.DefaultCoinDecimals
	}
}

// IsValidAddress make sure the address is correct for the chain
// And this also make sure testnet doesn't use mainnet address vice versa
func (c Chain) IsValidAddress(addr Address) bool {
	network := CurrentChainNetwork
	prefix := c.AddressPrefix(network)
	return strings.HasPrefix(addr.String(), prefix)
}

// AddressPrefix return the address prefix used by the given network (testnet/mainnet)
func (c Chain) AddressPrefix(cn ChainNetwork) string {
	if c.IsEVM() {
		return "0x"
	}
	switch cn {
	case MockNet:
		switch c {
		case AZTECChain:
			return "taztec"
		case BNBChain:
			return btypes.TestNetwork.Bech32Prefixes()
		case THORChain:
			return "tthor"
		case KUJIChain:
			return "kujira"
		case ETHChain:
			return "0x"
		case BASEChain:
			// TODO update this to use testnet address prefix
			return types.GetConfig().GetBech32AccountAddrPrefix()
		case BTCChain:
			return chaincfg.RegressionNetParams.Bech32HRPSegwit
		case DASHChain:
			return dashchaincfg.RegressionNetParams.Bech32HRPSegwit
		case XRDChain:
			return "account_loc"
		case ZECChain:
			return "tm"
		}
	case TestNet:
		switch c {
		case AZTECChain:
			return "taztec"
		case BNBChain:
			return btypes.TestNetwork.Bech32Prefixes()
		case THORChain:
			return "tthor"
		case KUJIChain:
			return "kujira"
		case ETHChain:
			return "0x"
		case BASEChain:
			// TODO update this to use testnet address prefix
			return types.GetConfig().GetBech32AccountAddrPrefix()
		case BTCChain:
			return chaincfg.TestNet3Params.Bech32HRPSegwit
		case DASHChain:
			return dashchaincfg.TestNet3Params.Bech32HRPSegwit
		case XRDChain:
			return "account_tdx_22_"
		case ZECChain:
			return "tm"
		}
	case StageNet:
		switch c {
		case AZTECChain:
			return "saztec"
		case BNBChain:
			return btypes.ProdNetwork.Bech32Prefixes()
		case THORChain:
			return "thor"
		case KUJIChain:
			return "kujira"
		case ETHChain:
			return "0x"
		case BASEChain:
			return types.GetConfig().GetBech32AccountAddrPrefix()
		case BTCChain:
			return chaincfg.MainNetParams.Bech32HRPSegwit
		case DASHChain:
			return dashchaincfg.MainNetParams.Bech32HRPSegwit
		case XRDChain:
			return "account_rdx"
		case ZECChain:
			return "t"
		}
	case MainNet:
		switch c {
		case AZTECChain:
			return "aztec"
		case BNBChain:
			return btypes.ProdNetwork.Bech32Prefixes()
		case THORChain:
			return "thor"
		case KUJIChain:
			return "kujira"
		case ETHChain:
			return "0x"
		case BASEChain:
			return types.GetConfig().GetBech32AccountAddrPrefix()
		case BTCChain:
			return chaincfg.MainNetParams.Bech32HRPSegwit
		case DASHChain:
			return dashchaincfg.MainNetParams.Bech32HRPSegwit
		case XRDChain:
			return "account_rdx"
		case ZECChain:
			return "t"
		}
	}
	return ""
}

// DustThreshold returns the min dust threshold for each chain
// The min dust threshold defines the lower end of the withdraw range of memoless savers txs
// The native coin value provided in a memoless tx defines a basis points amount of Withdraw or Add to a savers position as follows:
// Withdraw range: (dust_threshold + 1) -> (dust_threshold + 10_000)
// Add range: dust_threshold -> Inf
// NOTE: these should all be in 8 decimal places
func (c Chain) DustThreshold() cosmos.Uint {
	switch c {
	case BTCChain, DASHChain, ZECChain:
		return cosmos.NewUint(10_000)
	case ETHChain, BNBChain, BASEChain, THORChain, AZTECChain, KUJIChain, ARBChain:
		return cosmos.NewUint(0)
	default:
		return cosmos.NewUint(0)
	}
}

// MaxMemoLength returns the max memo length for each chain. Returns 0 if no max is configured.
func (c Chain) MaxMemoLength() int {
	switch c {
	case BTCChain, DASHChain, ZECChain:
		return 80
	default:
		// Default to the max memo size that we will process, regardless
		// of any higher memo size capable on other chains.
		return constants.MaxMemoSize
	}
}

// DefaultCoinbase returns the default coinbase address for each chain, returns 0 if no
// coinbase emission is used. This is used used at the time of writing as a fallback
// value in Bifrost, and for inbound confirmation count estimates in the quote APIs.
func (c Chain) DefaultCoinbase() float64 {
	switch c {
	case BTCChain:
		return 3.125 // halving every 210,000 blocks
	case DASHChain:
		return 1.902 // nSubsidy/14 every 210,240 blocks (https://github.com/dashpay/dash/blob/develop/src/validation.cpp#L1071)
	case ZECChain:
		return 1.5625 // halving every 840,000 blocks
	default:
		return 0
	}
}

func (c Chain) ApproximateBlockMilliseconds() int64 {
	switch c {
	case BTCChain:
		return 600_000
	case DASHChain:
		return 150_000
	case ETHChain:
		return 12_000
	case AVAXChain:
		return 3_000
	case BNBChain:
		return 500
	case KUJIChain:
		return 3_500
	case THORChain:
		return 6_000
	case BASEChain:
		return 6_000
	case ARBChain:
		return 260
	case XRDChain:
		return 500
	case ZECChain:
		return 75_000
	default:
		return 0
	}
}

func (c Chain) InboundNotes() string {
	switch c {
	case BTCChain, DASHChain, ZECChain:
		return "First output should be to inbound_address, second output should be change back to self, third output should be OP_RETURN, limited to 80 bytes. Do not send below the dust threshold. Do not use exotic spend scripts, locks or address formats (P2WSH with Bech32 address format preferred)."
	case ETHChain, AVAXChain, ARBChain:
		return "Base Asset: Send the inbound_address the asset with the memo encoded in hex in the data field. Tokens: First approve router to spend tokens from user: asset.approve(router, amount). Then call router.depositWithExpiry(inbound_address, asset, amount, memo, expiry). Asset is the token contract address. Amount should be in native asset decimals (eg 1e18 for most tokens). Do not send to or from contract addresses."
	case BNBChain, THORChain, KUJIChain:
		return "Transfer the inbound_address the asset with the memo. Do not use multi-in, multi-out transactions."
	case BASEChain:
		return "Broadcast a MsgDeposit to the MAYAChain network with the appropriate memo. Do not use multi-in, multi-out transactions."
	case XRDChain:
		return "Transfer a bucket of resources to the router component (component_rdx1...) using the `user_deposit` method call. Router address doesn't usually change (although it can be upgraded in the future), a single router instance handles all inbound addresses. The `sender` parameter is user's Radix address, which must be one of the signers of the transaction. `vault_address` parameter is the Asgard vault address (AKA inbound_address). Do not send the funds directly to the vault address, always use a router! Do not send less than 1 XRD."
	default:
		return ""
	}
}

func NewChains(raw []string) (Chains, error) {
	var returnErr error
	var chains Chains
	for _, c := range raw {
		chain, err := NewChain(c)
		if err == nil {
			chains = append(chains, chain)
		} else {
			returnErr = multierror.Append(returnErr, err)
		}
	}
	return chains, returnErr
}

// Has check whether chain c is in the list
func (chains Chains) Has(c Chain) bool {
	for _, ch := range chains {
		if ch.Equals(c) {
			return true
		}
	}
	return false
}

// Distinct return a distinct set of chains, no duplicates
func (chains Chains) Distinct() Chains {
	var newChains Chains
	for _, chain := range chains {
		if !newChains.Has(chain) {
			newChains = append(newChains, chain)
		}
	}
	return newChains
}

func (chains Chains) Strings() []string {
	strings := make([]string, len(chains))
	for i, c := range chains {
		strings[i] = c.String()
	}
	return strings
}

// ChainsThatCanBundleMultipleBlocks
// Returns a list of chains for which bifrost observer is allowed to bundle
// transactions from multiple blocks in a single TxIn.
// These are the chains with a very short block time (e.g. shorter than `MayachainBlockTime`).
func ChainsThatCanBundleMultipleBlocks() []Chain {
	return []Chain{ARBChain, BNBChain, KUJIChain, XRDChain}
}
