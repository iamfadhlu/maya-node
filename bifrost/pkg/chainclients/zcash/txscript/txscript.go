package txscript

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/btcsuite/btcutil/base58"
	"github.com/pkg/errors"

	// trunk-ignore(golangci-lint/gosec)
	"golang.org/x/crypto/ripemd160"

	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/zcash/rpc"
	"gitlab.com/mayachain/mayanode/chain/zec/go/zec"
)

type AddressType int

const (
	p2pkh AddressType = iota
	p2sh

	OP_DUP                = 0x76
	OP_HASH160            = 0xa9
	OP_PUSH20             = 0x14
	OP_EQUALVERIFY        = 0x88
	OP_CHECKSIG           = 0xac
	OP_EQUAL              = 0x87
	OP_PUSH33             = 0x21
	OP_PUSH65             = 0x41
	p2shScriptSize        = 23
	p2pkhScriptSize       = 25
	compressedKeyLen      = 33
	uncompressedKeyLen    = 65
	compressedKeyPrefix1  = 0x02
	compressedKeyPrefix2  = 0x03
	uncompressedKeyPrefix = 0x04
)

func IsTransactionShielded(tx *rpc.TxVerbose) bool {
	return len(tx.VShieldedSpend) > 0 || len(tx.VShieldedOutput) > 0
}

// ExtractPkScriptAddrs returns addresses from Zcash scriptPubKey (transparent only)
func ExtractPkScriptAddrs(pkScript []byte, networkParams *zec.NetworkParams) ([]string, error) {
	switch {
	case isPayToPubKeyHash(pkScript):
		hash := extractHash160PKH(pkScript) // Renamed for clarity
		if hash == nil {
			return nil, errors.New("could not extract hash160 from P2PKH script")
		}
		addr, err := encodeTransparentAddress(hash, p2pkh, networkParams)
		if err != nil {
			return nil, err
		}
		return []string{addr}, nil

	case isPayToScriptHash(pkScript):
		hash := extractHash160SH(pkScript) // Renamed for clarity
		if hash == nil {
			return nil, errors.New("could not extract hash160 from P2SH script")
		}
		addr, err := encodeTransparentAddress(hash, p2sh, networkParams)
		if err != nil {
			return nil, err
		}
		return []string{addr}, nil

	case isPayToPubKey(pkScript):
		pubkey, err := extractPubKey(pkScript)
		if err != nil {
			return nil, errors.Wrap(err, "failed to extract Pub key from pkScript")
		}
		// P2PK scripts resolve to P2PKH addresses
		addr, err := encodeTransparentAddress(hash160(pubkey), p2pkh, networkParams)
		if err != nil {
			return nil, err
		}
		return []string{addr}, nil

	default:
		return nil, nil // Not a standard transparent script or OP_RETURN etc.
	}
}

// GetAddressesFromScriptPubKey - helper function for utxo ownership verifycation
func GetAddressesFromScriptPubKey(scriptPubKey rpc.ScriptPubKey, networkParams *zec.NetworkParams) ([]string, error) {
	if len(scriptPubKey.Hex) == 0 {
		// if Addresses field is populated by RPC, use it directly
		if len(scriptPubKey.Addresses) > 0 {
			return scriptPubKey.Addresses, nil
		}
		return nil, fmt.Errorf("scriptPubKey hex is empty and no pre-parsed addresses available")
	}
	buf, err := hex.DecodeString(scriptPubKey.Hex)
	if err != nil {
		return nil, fmt.Errorf("fail to decode scriptPubKey hex '%s': %w", scriptPubKey.Hex, err)
	}
	extractedAddresses, err := ExtractPkScriptAddrs(buf, networkParams) // Pass network
	if err != nil {
		return nil, fmt.Errorf("fail to ExtractPkScriptAddrs from scriptPubKey hex '%s': %w", scriptPubKey.Hex, err)
	}
	// If RPC also provided addresses, you might want to compare/validate them against extracted ones
	// For now, if extractedAddresses is nil (e.g. OP_RETURN), but RPC provided one, it could be an issue.
	// However, for P2PKH/P2SH, extracted should be the source of truth if scriptPubKey.Hex is present.
	if extractedAddresses == nil && len(scriptPubKey.Addresses) > 0 {
		log.Printf("WARN: Script %s did not parse to a standard address, but RPC provided addresses: %v", scriptPubKey.Hex, scriptPubKey.Addresses)
		// Decide how to handle this: trust RPC, trust local parsing, or error.
		// For now, let's prioritize local parsing if script hex is valid.
	}

	return extractedAddresses, nil
}

// isPayToPubKeyHash checks if a script is a P2PKH script
func isPayToPubKeyHash(script []byte) bool {
	return len(script) == p2pkhScriptSize &&
		script[0] == OP_DUP &&
		script[1] == OP_HASH160 &&
		script[2] == OP_PUSH20 &&
		script[23] == OP_EQUALVERIFY &&
		script[24] == OP_CHECKSIG
}

// isPayToScriptHash checks if a script is a P2SH script
func isPayToScriptHash(script []byte) bool {
	return len(script) == p2shScriptSize &&
		script[0] == OP_HASH160 &&
		script[1] == OP_PUSH20 &&
		script[22] == OP_EQUAL
}

// isPayToPubKey checks if a script is a P2PK script
func isPayToPubKey(script []byte) bool {
	scriptLen := len(script)
	if scriptLen != 35 && scriptLen != 67 {
		return false
	}
	return (script[0] == OP_PUSH33 || script[0] == OP_PUSH65) &&
		script[scriptLen-1] == OP_CHECKSIG
}

// extractHash160PKH extracts the 20-byte hash from a P2PKH script
func extractHash160PKH(script []byte) []byte {
	// P2PKH: OP_DUP OP_HASH160 <0x14> <20-byte-hash> OP_EQUALVERIFY OP_CHECKSIG
	// Hex:   76     a9        14     <20-byte-hash> 88             ac
	if len(script) == 25 && script[2] == 0x14 {
		return script[3 : 3+20]
	}
	return nil
}

// extractHash160PKH extracts the 20-byte hash from a P2SH script
func extractHash160SH(script []byte) []byte {
	// P2SH: OP_HASH160 <0x14> <20-byte-hash> OP_EQUAL
	// Hex:  a9         14     <20-byte-hash> 87
	if len(script) == 23 && script[1] == 0x14 {
		return script[2 : 2+20]
	}
	return nil
}

// encodeTransparentAddress encodes a hash to a base58-encoded Zcash transparent address
// the appropriate version byte for P2PKH addresses
func encodeTransparentAddress(hash []byte, addrType AddressType, networkParams *zec.NetworkParams) (string, error) {
	var prefixBytes []byte
	switch addrType {
	case p2pkh:
		prefixBytes = networkParams.P2PKHPrefix
	case p2sh:
		prefixBytes = networkParams.P2SHPrefix
	default:
		return "", errors.New("unknown address type for prefix")
	}

	if len(prefixBytes) != 2 {
		return "", fmt.Errorf("invalid prefix length for Zcash transparent address: expected 2, got %d for network %s", len(prefixBytes), networkParams.Name)
	}

	data := prefixBytes
	data = append(data, hash...) // Use the 2-byte prefix slice
	checksum := calculateChecksum(data)
	return base58.Encode(append(data, checksum...)), nil
}

// calculateChecksum calculates the checksum for an address payload (double SHA256 hash, first 4 bytes)
func calculateChecksum(data []byte) []byte {
	firstHash := sha256.Sum256(data)
	secondHash := sha256.Sum256(firstHash[:])
	return secondHash[:4]
}

// hash160 computes SHA-256 followed by RIPEMD-160
func hash160(data []byte) []byte {
	shaHash := sha256.Sum256(data)
	// trunk-ignore(golangci-lint/gosec)
	hasher := ripemd160.New()
	_, err := hasher.Write(shaHash[:])
	if err != nil {
		return nil // Or return error if you change function signature
	}
	return hasher.Sum(nil)
}

// extractPubKey extracts the public key from a pay-to-pubkey script
func extractPubKey(script []byte) ([]byte, error) {
	scriptLen := len(script)

	if scriptLen < 2 {
		return nil, errors.New("script too short")
	}

	pubKey := script[:scriptLen-1] // All bytes except last OP_CHECKSIG byte (0xac)
	pubKeyLen := len(pubKey)

	switch {
	case pubKeyLen == compressedKeyLen && (pubKey[0] == compressedKeyPrefix1 || pubKey[0] == compressedKeyPrefix2): // Compressed key
		return pubKey, nil
	case pubKeyLen == uncompressedKeyLen && pubKey[0] == uncompressedKeyPrefix: // Uncompressed key
		return pubKey, nil
	default:
		return nil, errors.New("invalid public key format")
	}
}
