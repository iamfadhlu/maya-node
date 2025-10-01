package signer

import (
	"math"
	"strings"
)

const (
	special = `_-., !@$&*"#%'()+/:;<=>?[\]^{|}~`
	lower   = `abcdefghijklmnopqrstuvwxyz`
	upper   = `ABCDEFGHIJKLMNOPQRSTUVWXYZ`
	digits  = `0123456789`
)

// Return the passphrase entropy based in  'Entropy = log2(S^L)'
func GetEntropy(passphrase string) float64 {
	// S stands for Symbol pool
	symbolPool := getSymbolPool(passphrase)
	// L stands for length
	length := len(passphrase)

	return math.Log2(math.Pow(float64(symbolPool), float64(length)))
}

func getSymbolPool(passphrase string) int {
	hasSpecial := false
	hasLower := false
	hasUpper := false
	hasDigits := false
	s := 0

	chars := map[rune]struct{}{}
	for _, c := range passphrase {
		chars[c] = struct{}{}
	}

	for c := range chars {
		switch {
		case strings.ContainsRune(special, c):
			hasSpecial = true
		case strings.ContainsRune(lower, c):
			hasLower = true
		case strings.ContainsRune(upper, c):
			hasUpper = true
		case strings.ContainsRune(digits, c):
			hasDigits = true
		default:
			s++
		}
	}

	if hasSpecial {
		s += len(special)
	}
	if hasLower {
		s += len(lower)
	}
	if hasUpper {
		s += len(upper)
	}
	if hasDigits {
		s += len(digits)
	}
	return s
}
