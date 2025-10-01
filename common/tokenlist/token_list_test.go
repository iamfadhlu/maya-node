package tokenlist

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	. "gopkg.in/check.v1"
)

func TestPackage(t *testing.T) { TestingT(t) }

func TestNoDuplicateRadixSymbols(t *testing.T) {
	file, err := os.ReadFile("radixtokens/radix_mainnet_latest.json")
	if err != nil {
		t.Fatalf("Failed to read JSON file: %v", err)
	}

	var tokens []RadixToken
	if err := json.Unmarshal(file, &tokens); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	symbols := make(map[string]bool)
	for _, token := range tokens {
		upper := strings.ToUpper(token.Symbol)
		if symbols[upper] {
			t.Errorf("Duplicate symbol found: %s", token.Symbol)
		}
		symbols[upper] = true
	}
}
