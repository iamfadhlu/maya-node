package kuji

import "strings"

type KujiAssetMapping struct {
	KujiDenom       string
	KujiDecimals    int
	BASEChainSymbol string
}

// KujiAssetMappings maps a Kuji denom to a BASEChain symbol and provides the asset decimals
// CHANGEME: define assets that should be observed by BASEChain here. This also acts a whitelist.
var KujiAssetMappings = []KujiAssetMapping{
	{
		KujiDenom:       "ukuji",
		KujiDecimals:    6,
		BASEChainSymbol: "KUJI",
	},
	{
		KujiDenom:       "factory/kujira1qk00h5atutpsv900x202pxx42npjr9thg58dnqpa72f2p7m2luase444a7/uusk",
		KujiDecimals:    6,
		BASEChainSymbol: "USK",
	},
	{ // deprecated
		KujiDenom:       "factory/kujira1ygfxn0er40klcnck8thltuprdxlck6wvnpkf2k/uyum",
		KujiDecimals:    6,
		BASEChainSymbol: "YUM",
	},
	{
		KujiDenom:       "ibc/507BE7E33F06026652F519AD4D36716251F2D34DF04514A905D3B19A7D8130F7",
		KujiDecimals:    6,
		BASEChainSymbol: "AXLYUM",
	},
	{ // denom trace: "transfer/channel-95/erc20/tether/usdt"
		KujiDenom:       "ibc/20014F963CC9E6488B299622F87B60C6DE71632864859EC08B4753478DAB2BB8",
		KujiDecimals:    6,
		BASEChainSymbol: "USDT",
	},
	{ // denom trace: "transfer/channel-62/uusdc"
		KujiDenom:       "ibc/FE98AAD68F02F03565E9FA39A5E627946699B2B07115889ED812D8BA639576A9",
		KujiDecimals:    6,
		BASEChainSymbol: "USDC",
	},
	{ // denom trace: "transfer/channel-0/uatom"
		KujiDenom:       "ibc/27394FB092D2ECCD56123C74F36E4C1F926001CEADA9CA97EA622B25F41E5EB2",
		KujiDecimals:    6,
		BASEChainSymbol: "ATOM",
	},
	{ // denom trace: "transfer/channel-3/uosmo"
		KujiDenom:       "ibc/47BD209179859CDE4A2806763D7189B6E6FE13A17880FE2B42DE1E6C1E329E23",
		KujiDecimals:    6,
		BASEChainSymbol: "OSMO",
	},
}

func GetAssetByKujiDenom(denom string) (KujiAssetMapping, bool) {
	for _, asset := range KujiAssetMappings {
		if strings.EqualFold(asset.KujiDenom, denom) {
			return asset, true
		}
	}
	return KujiAssetMapping{}, false
}

func GetAssetByMayachainSymbol(symbol string) (KujiAssetMapping, bool) {
	for _, asset := range KujiAssetMappings {
		if strings.EqualFold(asset.BASEChainSymbol, symbol) {
			return asset, true
		}
	}
	return KujiAssetMapping{}, false
}
