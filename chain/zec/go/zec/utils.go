package zec

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcutil/base58"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"

	"gitlab.com/mayachain/mayanode/common/cosmos"

	// trunk-ignore(golangci-lint/staticcheck)
	"golang.org/x/crypto/ripemd160"
)

// --- Basic Zcash Address Types ---

const ZatoshisPerZEC int64 = 100000000

type addressKind int

const (
	UnknownAddress addressKind = iota
	// Sprout           // Typically Base58Check zc/zt - often unsupported now
	Sapling // Bech32m zs/ztestsapling
	Unified // Bech32m u/utest
	P2PKH   // Base58Check t1/tm
	P2SH    // Base58Check t3/tn
	Tex     // Base58Check tx/? - Explicitly disallowed by requirement

	PkhOutputSize  = 34    // Size of a PKH output
	BaseRelayFee   = 10000 // Minimum relay fee (10k zsats)
	AddMarginalFee = 5000  // Additional fee per input/output after the first two (5k zsats)
)

// --- Network Parameters ---

type NetworkParams struct {
	Name           string  // human-readable identifier for the network
	Net            Network // value used to identify the network
	Bech32mHRPs    map[string]addressKind
	Base58Prefixes map[string]addressKind
	P2PKHPrefix    []byte // prefixes for P2PKH transparent addresses ([2]byte)
	P2SHPrefix     []byte // prefixes for P2SH transparent addresses ([2]byte)
}

var MainNetParams = NetworkParams{
	Name: "MainNet",
	Net:  NetworkMain,
	// Bech32m HRP -> Kind Lookup
	Bech32mHRPs: map[string]addressKind{
		"zs": Sapling,
		"u":  Unified,
	},
	// Base58 Hex Prefix -> Kind Lookup
	Base58Prefixes: map[string]addressKind{
		"1cb8": P2PKH, // Mainnet t1
		"1cbd": P2SH,  // Mainnet t3
		"1c9a": Tex,   // Mainnet TEX
	},
	// "t1" - 0x1C, 0xB8 for Mainnet P2PKH prefix
	P2PKHPrefix: []byte{0x1C, 0xB8},
	P2SHPrefix:  []byte{0x1C, 0xBD},
}

var RegtestNetParams = NetworkParams{
	Name: "RegtestNet",
	Net:  NetworkRegtest,
	// Bech32m HRP -> Kind Lookup (Include Regtest HRPs here)
	Bech32mHRPs: map[string]addressKind{
		"ztestsapling":    Sapling, // Testnet Sapling
		"zregtestsapling": Sapling, // Regtest Sapling
		"utest":           Unified, // Testnet Unified
		"uregtest":        Unified, // Regtest Unified
		// Regtest HRPs
		"regtestu": Unified,
		"regtesto": Unified, // Orchard HRP for UAs often mapped to Unified kind
	},
	// Base58 Hex Prefix -> Kind Lookup
	Base58Prefixes: map[string]addressKind{
		"1d25": P2PKH, // Testnet tm
		"1cba": P2SH,  // Testnet tn
		"1dac": Tex,   // Testnet TEX
	},
	// "tm" - 0x1D, 0x25 for Testnet/Regtest P2PKH prefix
	P2PKHPrefix: []byte{0x1D, 0x25},
	P2SHPrefix:  []byte{0x1C, 0xBA},
}

// Generate Zcash transparent address from Cosmos Bech32 secp256k1 public key
func NewAddressWitnessPubKeyHash(pks string, params *NetworkParams) (string, error) {
	// doubleSha256 calculates sha256(sha256(b))
	doubleSha256 := func(b []byte) []byte {
		h1 := sha256.Sum256(b)
		h2 := sha256.Sum256(h1[:])
		return h2[:]
	}

	// zcashBase58CheckEncode performs Base58Check encoding with Zcash's double-SHA256 checksum.
	// The input 'payload' should already contain the Zcash version prefix bytes.
	zcashBase58CheckEncode := func(payload []byte) string {
		// 1. Calculate checksum: first 4 bytes of double-sha256(payload)
		checksum := doubleSha256(payload)[:4]

		// 2. Append checksum to payload
		payload = append(payload, checksum...)

		// 3. Encode the combined data using standard Base58
		return base58.Encode(payload)
	}

	// decode Cosmos Bech32 pubkey
	// Use cosmos.AccPubKey Bech32 prefix
	pk, err := cosmos.GetPubKeyFromBech32(cosmos.Bech32PubKeyTypeAccPub, pks)
	if err != nil {
		return "", fmt.Errorf("invalid Bech32 pubkey: %w", err)
	}

	// convert to Cosmos SDK's secp256k1 type
	cosmosPk, ok := pk.(*secp256k1.PubKey)
	if !ok {
		// Alternatively, check pk.Type() == "secp256k1" and use pk.Bytes()
		return "", errors.New("only secp256k1 keys are supported")
	}

	// parse with btcec (ensure it handles compressed/uncompressed correctly)
	// Note: Cosmos SDK secp256k1.PubKey.Bytes() usually returns the compressed form (33 bytes)
	rawKey := cosmosPk.Bytes()
	if len(rawKey) != 33 {
		// We need compressed keys 33 bytes (not uncompressed 65 bytes)
		return "", fmt.Errorf("expected 33-byte compressed key, got %d bytes", len(rawKey))
	}

	pubKey, err := btcec.ParsePubKey(rawKey, btcec.S256())
	if err != nil {
		return "", fmt.Errorf("invalid secp256k1 key bytes: %w", err)
	}

	// generate Zcash address hash
	compressedKey := pubKey.SerializeCompressed() // Ensure it's compressed (33 bytes)
	shaHash := sha256.Sum256(compressedKey)
	ripemd160Hasher := ripemd160.New()
	_, err = ripemd160Hasher.Write(shaHash[:]) // Always check hash write errors (though unlikely here)
	if err != nil {
		// This should practically never happen with fixed-size input
		return "", fmt.Errorf("failed to write to ripemd160 hasher: %w", err)
	}
	prefix := []byte{}
	prefix = append(prefix, params.P2PKHPrefix...)
	pkh := ripemd160Hasher.Sum(nil) // 20-byte public key hash
	// Create the payload: prefix + pubKeyHash
	prefix = append(prefix, pkh...)
	payload := prefix
	if len(payload) != 22 {
		// Sanity check
		return "", fmt.Errorf("internal error: unexpected payload length (%d bytes)", len(payload))
	}

	// Use Zcash-specific Base58Check encoding
	address := zcashBase58CheckEncode(payload)

	// Correct return path
	return address, nil
}

// ZecToZats converts ZEC to Zats (*big.Int).
func ZecToZats(zec float64) (*big.Int, error) {
	zecBigFloat := new(big.Float).SetFloat64(zec)
	precision := new(big.Float).SetInt64(ZatoshisPerZEC)
	zatsBigFloat := new(big.Float).Mul(zecBigFloat, precision)
	zatsBigInt, _ := new(big.Float).Add(zatsBigFloat, big.NewFloat(0.5)).Int(nil) // round towards zero
	return zatsBigInt, nil
}

// ZecToUint converts ZEC to Zats (cosmos.Uint).
func ZecToUint(zec float64) (cosmos.Uint, error) {
	if zec < 0 {
		return cosmos.Uint{}, errors.New("amount cannot be negative")
	}
	zats, err := ZecToZats(zec)
	if err != nil {
		return cosmos.Uint{}, fmt.Errorf("failed to convert ZEC to Zats: %w", err)
	}
	return cosmos.NewUintFromBigInt(zats), nil
}

// CalculateFee calculates 25,000 zats for 1-in, 2-out, and this function aligns with the recommended ZIP-317 structure.
// This 25,000 zat fee should be robust enough for your v5 transparent transactions and very likely resolve the "tx unpaid action limit exceeded" error.
// It correctly implements the recommended ZIP-317 fee structure for purely transparent transactions.
// It uses the recommended conventional_fee (10,000 zats as BaseRelayFee).
// It applies the marginal_fee (5,000 zats as AddMarginalFee) to every input and every output.
// For a Version 5 transaction (which our Zcash node creates), this 25,000 zat fee is very likely sufficient to cover the "unpaid action limit" error.
// The 10,000 zat BaseRelayFee component can be considered as covering the "cost" of one Orchard action (the v5 structure itself).
// The additional fees for inputs/outputs cover the transparent parts.
//
// AddMarginalFee == 5000 - ZIP 317 marginal fee per T input & output
// (1+2)*5000 + 10000 = 2500 Zats is the fee for a normal tx
func CalculateFee(inCount, outCount uint64) uint64 {
	if inCount < 1 {
		inCount = 1
	}
	if outCount < 2 {
		outCount = 2
	}
	return inCount*AddMarginalFee + outCount*AddMarginalFee + BaseRelayFee
}

func CalculateFeeWithMemo(inCount, outCount uint64, memo string) uint64 {
	if len(memo) > 0 {
		// Additional outputs might be needed for memo
		memoLenWithOverhead := int64(len(memo) + 2)
		memoOutputSlots := uint64((memoLenWithOverhead + PkhOutputSize - 1) / PkhOutputSize)
		outCount += memoOutputSlots
	}

	// Calculate fee based on input/output count following ZIP-317
	fee := CalculateFee(inCount, outCount)

	// Ensure minimum relay fee is met
	fee = max(fee, BaseRelayFee)

	return fee
}
