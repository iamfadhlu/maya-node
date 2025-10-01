package shared

import (
	"fmt"
	"strings"

	"gitlab.com/mayachain/mayanode/bifrost/mayaclient"
	"gitlab.com/mayachain/mayanode/common"
)

var ETHChainConfig = ChainConfig{ConfMultiplierBasisPoints: 50000, MaxConfirmations: 5}

type ChainConfig struct {
	ConfMultiplierBasisPoints int64
	MaxConfirmations          int64
}

type MockMayaChainBridge struct {
	mayaclient.MayachainBridge
	chainConfigs map[common.Chain]ChainConfig
}

func NewMockMayaChainBridge() *MockMayaChainBridge {
	bridge := &MockMayaChainBridge{
		chainConfigs: make(map[common.Chain]ChainConfig),
	}

	// Initialize chain configurations
	configs := map[common.Chain]ChainConfig{
		common.BTCChain: {ConfMultiplierBasisPoints: 50000, MaxConfirmations: 5},
		common.ETHChain: ETHChainConfig,
	}

	bridge.chainConfigs = configs
	return bridge
}

// parseMimirKey splits a Mimir key into its components
func parseMimirKey(key string) (keyType string, chain common.Chain, err error) {
	validPrefixes := []string{"ConfMultiplierBasisPoints-", "MaxConfirmations-"}

	for _, prefix := range validPrefixes {
		if strings.HasPrefix(key, prefix) {
			parts := strings.Split(key, "-")
			if len(parts) != 2 {
				return "", "", fmt.Errorf("invalid key format: %s", key)
			}
			return parts[0], common.Chain(parts[1]), nil
		}
	}

	return "", "", fmt.Errorf("unsupported key prefix: %s", key)
}

// GetMimir retrieves Mimir values for the specified key
func (m *MockMayaChainBridge) GetMimir(key string) (int64, error) {
	keyType, chain, err := parseMimirKey(key)
	if err != nil {
		return -1, err
	}

	config, exists := m.chainConfigs[chain]
	if !exists {
		return -1, fmt.Errorf("unsupported chain: %s", chain)
	}

	switch keyType {
	case "ConfMultiplierBasisPoints":
		return config.ConfMultiplierBasisPoints, nil
	case "MaxConfirmations":
		return config.MaxConfirmations, nil
	default:
		return -1, fmt.Errorf("invalid key type: %s", keyType)
	}
}
