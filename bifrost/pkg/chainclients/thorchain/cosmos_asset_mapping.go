package thorchain

import "strings"

type CosmosAssetMapping struct {
	CosmosDenom     string
	CosmosDecimals  int
	THORChainSymbol string
}

// CosmosAssetMappings maps a Cosmos denom to a THORChain symbol and provides the asset decimals
var CosmosAssetMappings = []CosmosAssetMapping{
	{
		CosmosDenom:     "rune",
		CosmosDecimals:  8,
		THORChainSymbol: "RUNE",
	},
}

func GetAssetByCosmosDenom(denom string) (CosmosAssetMapping, bool) {
	for _, asset := range CosmosAssetMappings {
		if strings.EqualFold(asset.CosmosDenom, denom) {
			return asset, true
		}
	}
	return CosmosAssetMapping{}, false
}

func GetAssetByThorchainSymbol(symbol string) (CosmosAssetMapping, bool) {
	for _, asset := range CosmosAssetMappings {
		if strings.EqualFold(asset.THORChainSymbol, symbol) {
			return asset, true
		}
	}
	return CosmosAssetMapping{}, false
}
