//go:build testnet || mocknet
// +build testnet mocknet

package cmd

const (
	Bech32PrefixAccAddr         = "tmaya"
	Bech32PrefixAccPub          = "tmayapub"
	Bech32PrefixValAddr         = "tmayav"
	Bech32PrefixValPub          = "tmayavpub"
	Bech32PrefixConsAddr        = "tmayac"
	Bech32PrefixConsPub         = "tmayacpub"
	DenomRegex                  = `[a-zA-Z][a-zA-Z0-9:\\/\\\-\\_\\.]{2,127}`
	BASEChainCoinType    uint32 = 931
	BASEChainCoinPurpose uint32 = 44

	// secp256k1 derivation. ed25519 derivation is in common/crypto/ed25519
	BASEChainHDPath string = `m/44'/931'/0'/0/0`
)
