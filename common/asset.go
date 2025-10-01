package common

import (
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"github.com/blang/semver"
	"github.com/gogo/protobuf/jsonpb"
)

type Assets []Asset

const (
	MayachainAmountToWeiFactor      = One * 100
	MayachainAmountToCentiWeiFactor = 10_000_000
	AssetSeparators                 = "[~./]"
)

var (
	// EmptyAsset empty asset, not valid
	EmptyAsset = Asset{Chain: EmptyChain, Symbol: "", Ticker: "", Synth: false}
	// RUNEAsset RUNE
	RUNEAsset = Asset{Chain: THORChain, Symbol: "RUNE", Ticker: "RUNE", Synth: false}
	// BNBAsset BNB
	BNBAsset = Asset{Chain: BNBChain, Symbol: "BNB", Ticker: "BNB", Synth: false}
	// BTCAsset BTC
	BTCAsset = Asset{Chain: BTCChain, Symbol: "BTC", Ticker: "BTC", Synth: false}
	// DASHAsset DASH
	DASHAsset = Asset{Chain: DASHChain, Symbol: "DASH", Ticker: "DASH", Synth: false}

	// ETHAsset ETH
	ETHAsset = Asset{Chain: ETHChain, Symbol: "ETH", Ticker: "ETH", Synth: false}
	// USDTAsset ETH
	USDTAsset   = Asset{Chain: ETHChain, Symbol: "USDT-0xdAC17F958D2ee523a2206206994597C13D831ec7", Ticker: "USDT", Synth: false}
	USDTAssetV1 = Asset{Chain: ETHChain, Symbol: "USDT-0xdAC17F958D2ee523a2206206994597C13D831ec7", Ticker: "ETH", Synth: false}
	// USDCAsset ETH
	USDCAsset   = Asset{Chain: ETHChain, Symbol: "USDC-0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", Ticker: "USDC", Synth: false}
	USDCAssetV1 = Asset{Chain: ETHChain, Symbol: "USDC-0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", Ticker: "ETH", Synth: false}
	// WSTETHAsset ETH
	WSTETHAsset   = Asset{Chain: ETHChain, Symbol: "WSTETH-0X7F39C581F595B53C5CB19BD0B3F8DA6C935E2CA0", Ticker: "WSTETH", Synth: false}
	WSTETHAssetV1 = Asset{Chain: ETHChain, Symbol: "WSTETH-0X7F39C581F595B53C5CB19BD0B3F8DA6C935E2CA0", Ticker: "ETH", Synth: false}
	// PEPEAsset ETH
	PEPEAssetV1 = Asset{Chain: ETHChain, Symbol: "PEPE-0x25D887CE7A35172C62FEBFD67A1856F20FAEBB00", Ticker: "PEPE", Synth: false}
	// PEPEAsset ETH
	PEPEAsset = Asset{Chain: ETHChain, Symbol: "PEPE-0X6982508145454CE325DDBE47A25D4EC3D2311933", Ticker: "PEPE", Synth: false}

	// ETHAsset ARB
	AETHAsset = Asset{Chain: ARBChain, Symbol: "ETH", Ticker: "ETH", Synth: false}
	// USDTAsset ARB
	AUSDTAsset = Asset{Chain: ARBChain, Symbol: "USDT-0XFD086BC7CD5C481DCC9C85EBE478A1C0B69FCBB9", Ticker: "USDT", Synth: false}
	// USDCAsset ARB
	AUSDCAsset = Asset{Chain: ARBChain, Symbol: "USDC-0XAF88D065E77C8CC2239327C5EDB3A432268E5831", Ticker: "USDC", Synth: false}
	// DAIAsset ARB
	ADAIAsset = Asset{Chain: ARBChain, Symbol: "DAI-0XDA10009CBD5D07DD0CECC66161FC93D7C9000DA1", Ticker: "DAI", Synth: false}
	// PEPEAsset ARB
	APEPEAsset = Asset{Chain: ARBChain, Symbol: "PEPE-0X25D887CE7A35172C62FEBFD67A1856F20FAEBB00", Ticker: "PEPE", Synth: false}
	// WSTETHAsset ARB
	AWSTETHAsset = Asset{Chain: ARBChain, Symbol: "WSTETH-0X5979D7B546E38E414F7E9822514BE443A4800529", Ticker: "WSTETH", Synth: false}
	// WBTCAsset ARB
	AWBTCAsset = Asset{Chain: ARBChain, Symbol: "WBTC-0X2F2A2543B76A4166549F7AAB2E75BEF0AEFC5B0F", Ticker: "WBTC", Synth: false}
	// ATGTAsset ARB
	ATGTAsset = Asset{Chain: ARBChain, Symbol: "TGT-0x429FED88F10285E61B12BDF00848315FBDFCC341", Ticker: "TGT", Synth: false}

	// KUJIAsset KUJI
	KUJIAsset = Asset{Chain: KUJIChain, Symbol: "KUJI", Ticker: "KUJI", Synth: false}
	// USKAsset KUJI
	USKAsset = Asset{Chain: KUJIChain, Symbol: "USK", Ticker: "KUJI", Synth: false}

	// AVAXAsset AVAX
	AVAXAsset = Asset{Chain: AVAXChain, Symbol: "AVAX", Ticker: "AVAX", Synth: false}

	// XRDAsset XRD
	XRDAsset = Asset{Chain: XRDChain, Symbol: "XRD", Ticker: "XRD", Synth: false}

	// ZECAsset ZEC
	ZECAsset = Asset{Chain: ZECChain, Symbol: "ZEC", Ticker: "ZEC", Synth: false}

	// BaseNative CACAO on mayachain
	BaseNative = Asset{Chain: BASEChain, Symbol: "CACAO", Ticker: "CACAO", Synth: false}
	MayaNative = Asset{Chain: BASEChain, Symbol: "MAYA", Ticker: "MAYA", Synth: false}
)

// NewAsset parse the given input into Asset object
func NewAsset(input string) (Asset, error) {
	var err error
	var asset Asset
	var sym string
	var parts []string
	re := regexp.MustCompile(AssetSeparators)

	match := re.FindString(input)

	switch match {
	case "~":
		parts = strings.SplitN(input, match, 2)
		asset.Trade = true
	case "/":
		parts = strings.SplitN(input, match, 2)
		asset.Synth = true
	case ".":
		parts = strings.SplitN(input, match, 2)
	default: // Handles both "" and unexpected separators
		parts = []string{input}
	}
	if len(parts) == 1 {
		asset.Chain = BASEChain
		sym = parts[0]
	} else {
		asset.Chain, err = NewChain(parts[0])
		if err != nil {
			return EmptyAsset, err
		}
		sym = parts[1]
	}

	asset.Symbol, err = NewSymbol(sym)
	if err != nil {
		return EmptyAsset, err
	}

	parts = strings.SplitN(sym, "-", 2)
	asset.Ticker, err = NewTicker(parts[0])
	if err != nil {
		return EmptyAsset, err
	}

	return asset, nil
}

func NewAssetWithShortCodes(version semver.Version, input string) (Asset, error) {
	switch {
	case version.GTE(semver.MustParse("1.120.0")):
		return NewAssetWithShortCodesV120(version, input)
	case version.GTE(semver.MustParse("1.112.0")):
		return NewAssetWithShortCodesV112(version, input)
	case version.GTE(semver.MustParse("1.111.0")):
		return NewAssetWithShortCodesV111(version, input)
	case version.GTE(semver.MustParse("1.110.0")):
		return NewAssetWithShortCodesV110(version, input)
	default:
		return NewAsset(input)
	}
}

func NewAssetWithShortCodesV120(version semver.Version, input string) (Asset, error) {
	if input == "" {
		return NewAsset(input)
	}

	shorts := make(map[string]string)

	// One letter
	shorts[AETHAsset.ShortCode()] = AETHAsset.String()
	shorts[BaseAsset().ShortCode()] = BaseAsset().String()
	shorts[BTCAsset.ShortCode()] = BTCAsset.String()
	shorts[DASHAsset.ShortCode()] = DASHAsset.String()
	shorts[ETHAsset.ShortCode()] = ETHAsset.String()
	shorts[KUJIAsset.ShortCode()] = KUJIAsset.String()
	shorts[RUNEAsset.ShortCode()] = RUNEAsset.String()
	shorts[XRDAsset.ShortCode()] = XRDAsset.String()
	shorts[ZECAsset.ShortCode()] = ZECAsset.String()

	// Two letter
	shorts[AETHAsset.TwoLetterShortCode(version)] = AETHAsset.String()
	shorts[BaseAsset().TwoLetterShortCode(version)] = BaseAsset().String()
	shorts[BTCAsset.TwoLetterShortCode(version)] = BTCAsset.String()
	shorts[DASHAsset.TwoLetterShortCode(version)] = DASHAsset.String()
	shorts[ETHAsset.TwoLetterShortCode(version)] = ETHAsset.String()
	shorts[KUJIAsset.TwoLetterShortCode(version)] = KUJIAsset.String()
	shorts[RUNEAsset.TwoLetterShortCode(version)] = RUNEAsset.String()
	shorts[XRDAsset.TwoLetterShortCode(version)] = XRDAsset.String()
	shorts[ZECAsset.TwoLetterShortCode(version)] = ZECAsset.String()

	// Two letter non-gas assets
	// ARB
	shorts[AUSDTAsset.TwoLetterShortCode(version)] = AUSDTAsset.String()
	shorts[AUSDCAsset.TwoLetterShortCode(version)] = AUSDCAsset.String()
	shorts[ADAIAsset.TwoLetterShortCode(version)] = ADAIAsset.String()
	shorts[APEPEAsset.TwoLetterShortCode(version)] = APEPEAsset.String()
	shorts[AWSTETHAsset.TwoLetterShortCode(version)] = AWSTETHAsset.String()
	shorts[AWBTCAsset.TwoLetterShortCode(version)] = AWBTCAsset.String()
	// ETH
	shorts[USDTAsset.TwoLetterShortCode(version)] = USDTAsset.String()
	shorts[USDCAsset.TwoLetterShortCode(version)] = USDCAsset.String()
	shorts[PEPEAsset.TwoLetterShortCode(version)] = PEPEAsset.String()
	shorts[WSTETHAsset.TwoLetterShortCode(version)] = WSTETHAsset.String()
	// KUJI
	shorts[USKAsset.TwoLetterShortCode(version)] = USKAsset.String()

	long, ok := shorts[input]
	if ok {
		input = long
	}

	return NewAsset(input)
}

func NewAssetWithShortCodesV112(version semver.Version, input string) (Asset, error) {
	if input == "" {
		return NewAsset(input)
	}

	shorts := make(map[string]string)

	// One letter
	shorts[AETHAsset.ShortCode()] = AETHAsset.String()
	shorts[BaseAsset().ShortCode()] = BaseAsset().String()
	shorts[BTCAsset.ShortCode()] = BTCAsset.String()
	shorts[DASHAsset.ShortCode()] = DASHAsset.String()
	shorts[ETHAsset.ShortCode()] = ETHAsset.String()
	shorts[KUJIAsset.ShortCode()] = KUJIAsset.String()
	shorts[RUNEAsset.ShortCode()] = RUNEAsset.String()
	shorts[XRDAsset.ShortCode()] = XRDAsset.String()

	// Two letter
	shorts[AETHAsset.TwoLetterShortCode(version)] = AETHAsset.String()
	shorts[BaseAsset().TwoLetterShortCode(version)] = BaseAsset().String()
	shorts[BTCAsset.TwoLetterShortCode(version)] = BTCAsset.String()
	shorts[DASHAsset.TwoLetterShortCode(version)] = DASHAsset.String()
	shorts[ETHAsset.TwoLetterShortCode(version)] = ETHAsset.String()
	shorts[KUJIAsset.TwoLetterShortCode(version)] = KUJIAsset.String()
	shorts[RUNEAsset.TwoLetterShortCode(version)] = RUNEAsset.String()
	shorts[XRDAsset.TwoLetterShortCode(version)] = XRDAsset.String()

	// Two letter non-gas assets
	// ARB
	shorts[AUSDTAsset.TwoLetterShortCode(version)] = AUSDTAsset.String()
	shorts[AUSDCAsset.TwoLetterShortCode(version)] = AUSDCAsset.String()
	shorts[ADAIAsset.TwoLetterShortCode(version)] = ADAIAsset.String()
	shorts[APEPEAsset.TwoLetterShortCode(version)] = APEPEAsset.String()
	shorts[AWSTETHAsset.TwoLetterShortCode(version)] = AWSTETHAsset.String()
	shorts[AWBTCAsset.TwoLetterShortCode(version)] = AWBTCAsset.String()
	// ETH
	shorts[USDTAsset.TwoLetterShortCode(version)] = USDTAsset.String()
	shorts[USDCAsset.TwoLetterShortCode(version)] = USDCAsset.String()
	shorts[PEPEAsset.TwoLetterShortCode(version)] = PEPEAsset.String()
	shorts[WSTETHAsset.TwoLetterShortCode(version)] = WSTETHAsset.String()
	// KUJI
	shorts[USKAsset.TwoLetterShortCode(version)] = USKAsset.String()

	long, ok := shorts[input]
	if ok {
		input = long
	}

	return NewAsset(input)
}

func NewAssetWithShortCodesV111(version semver.Version, input string) (Asset, error) {
	shorts := make(map[string]string)

	// One letter
	shorts[AETHAsset.ShortCode()] = AETHAsset.String()
	shorts[BaseAsset().ShortCode()] = BaseAsset().String()
	shorts[BTCAsset.ShortCode()] = BTCAsset.String()
	shorts[DASHAsset.ShortCode()] = DASHAsset.String()
	shorts[ETHAsset.ShortCode()] = ETHAsset.String()
	shorts[KUJIAsset.ShortCode()] = KUJIAsset.String()
	shorts[RUNEAsset.ShortCode()] = RUNEAsset.String()
	shorts[XRDAsset.ShortCode()] = XRDAsset.String()

	// Two letter
	shorts[AETHAsset.TwoLetterShortCode(version)] = AETHAsset.String()
	shorts[BaseAsset().TwoLetterShortCode(version)] = BaseAsset().String()
	shorts[BTCAsset.TwoLetterShortCode(version)] = BTCAsset.String()
	shorts[DASHAsset.TwoLetterShortCode(version)] = DASHAsset.String()
	shorts[ETHAsset.TwoLetterShortCode(version)] = ETHAsset.String()
	shorts[KUJIAsset.TwoLetterShortCode(version)] = KUJIAsset.String()
	shorts[RUNEAsset.TwoLetterShortCode(version)] = RUNEAsset.String()
	shorts[XRDAsset.TwoLetterShortCode(version)] = XRDAsset.String()

	// Two letter non-gas assets
	// ARB
	shorts[AUSDTAsset.TwoLetterShortCode(version)] = AUSDTAsset.String()
	shorts[AUSDCAsset.TwoLetterShortCode(version)] = AUSDCAsset.String()
	shorts[ADAIAsset.TwoLetterShortCode(version)] = ADAIAsset.String()
	shorts[APEPEAsset.TwoLetterShortCode(version)] = APEPEAsset.String()
	shorts[AWSTETHAsset.TwoLetterShortCode(version)] = AWSTETHAsset.String()
	shorts[AWBTCAsset.TwoLetterShortCode(version)] = AWBTCAsset.String()
	// ETH
	shorts[USDTAsset.TwoLetterShortCode(version)] = USDTAsset.String()
	shorts[USDCAsset.TwoLetterShortCode(version)] = USDCAsset.String()
	shorts[PEPEAssetV1.TwoLetterShortCode(version)] = PEPEAssetV1.String()
	shorts[WSTETHAsset.TwoLetterShortCode(version)] = WSTETHAsset.String()
	// KUJI
	shorts[USKAsset.TwoLetterShortCode(version)] = USKAsset.String()

	long, ok := shorts[input]
	if ok {
		input = long
	}

	return NewAsset(input)
}

func NewAssetWithShortCodesV110(version semver.Version, input string) (Asset, error) {
	shorts := make(map[string]string)

	// One letter
	shorts[AETHAsset.ShortCode()] = AETHAsset.String()
	shorts[BaseAsset().ShortCode()] = BaseAsset().String()
	shorts[BTCAsset.ShortCode()] = BTCAsset.String()
	shorts[DASHAsset.ShortCode()] = DASHAsset.String()
	shorts[ETHAsset.ShortCode()] = ETHAsset.String()
	shorts[KUJIAsset.ShortCode()] = KUJIAsset.String()
	shorts[RUNEAsset.ShortCode()] = RUNEAsset.String()

	// Two letter
	shorts[AETHAsset.TwoLetterShortCode(version)] = AETHAsset.String()
	shorts[BaseAsset().TwoLetterShortCode(version)] = BaseAsset().String()
	shorts[BTCAsset.TwoLetterShortCode(version)] = BTCAsset.String()
	shorts[DASHAsset.TwoLetterShortCode(version)] = DASHAsset.String()
	shorts[ETHAsset.TwoLetterShortCode(version)] = ETHAsset.String()
	shorts[KUJIAsset.TwoLetterShortCode(version)] = KUJIAsset.String()
	shorts[RUNEAsset.TwoLetterShortCode(version)] = RUNEAsset.String()

	// Two letter non-gas assets
	// ARB
	shorts[AUSDTAsset.TwoLetterShortCode(version)] = AUSDTAsset.String()
	shorts[AUSDCAsset.TwoLetterShortCode(version)] = AUSDCAsset.String()
	shorts[ADAIAsset.TwoLetterShortCode(version)] = ADAIAsset.String()
	shorts[APEPEAsset.TwoLetterShortCode(version)] = APEPEAsset.String()
	shorts[AWSTETHAsset.TwoLetterShortCode(version)] = AWSTETHAsset.String()
	shorts[AWBTCAsset.TwoLetterShortCode(version)] = AWBTCAsset.String()
	// ETH
	shorts[USDTAsset.TwoLetterShortCode(version)] = USDTAsset.String()
	shorts[USDCAsset.TwoLetterShortCode(version)] = USDCAsset.String()
	shorts[PEPEAssetV1.TwoLetterShortCode(version)] = PEPEAssetV1.String()
	shorts[WSTETHAsset.TwoLetterShortCode(version)] = WSTETHAsset.String()
	// KUJI
	shorts[USKAsset.TwoLetterShortCode(version)] = USKAsset.String()

	long, ok := shorts[input]
	if ok {
		input = long
	}

	return NewAsset(input)
}

func (a Asset) Valid() error {
	if err := a.Chain.Valid(); err != nil {
		return fmt.Errorf("invalid chain: %w", err)
	}
	if err := a.Symbol.Valid(); err != nil {
		return fmt.Errorf("invalid symbol: %w", err)
	}
	if a.Synth && a.Chain.IsBASEChain() {
		return fmt.Errorf("synth asset cannot have chain MAYA: %s", a)
	}
	return nil
}

// Equals determinate whether two assets are equivalent
func (a Asset) Equals(a2 Asset) bool {
	return a.Chain.Equals(a2.Chain) && a.Symbol.Equals(a2.Symbol) && a.Ticker.Equals(a2.Ticker) && a.Synth == a2.Synth && a.Trade == a2.Trade
}

func (a Asset) GetChain() Chain {
	if a.Synth || a.Trade {
		return BASEChain
	}
	return a.Chain
}

// Get layer1 asset version
func (a Asset) GetLayer1Asset() Asset {
	if !a.IsSyntheticAsset() && !a.IsTradeAsset() {
		return a
	}
	return Asset{
		Chain:  a.Chain,
		Symbol: a.Symbol,
		Ticker: a.Ticker,
		Synth:  false,
		Trade:  false,
	}
}

// Get synthetic asset of asset
func (a Asset) GetSyntheticAsset() Asset {
	if a.IsSyntheticAsset() {
		return a
	}
	return Asset{
		Chain:  a.Chain,
		Symbol: a.Symbol,
		Ticker: a.Ticker,
		Synth:  true,
	}
}

// Get trade asset of asset
func (a Asset) GetTradeAsset() Asset {
	if a.IsTradeAsset() {
		return a
	}
	return Asset{
		Chain:  a.Chain,
		Symbol: a.Symbol,
		Ticker: a.Ticker,
		Trade:  true,
	}
}

// Check if asset is a pegged asset
func (a Asset) IsSyntheticAsset() bool {
	return a.Synth
}

func (a Asset) IsTradeAsset() bool {
	return a.Trade
}

func (a Asset) IsVaultAsset() bool {
	return a.IsSyntheticAsset()
}

// Native return native asset, only relevant on THORChain
func (a Asset) Native() string {
	if a.IsBase() {
		return "cacao"
	}
	if a.Equals(MayaNative) {
		return "maya"
	}
	return strings.ToLower(a.String())
}

// IsEmpty will be true when any of the field is empty, chain,symbol or ticker
func (a Asset) IsEmpty() bool {
	return a.Chain.IsEmpty() || a.Symbol.IsEmpty() || a.Ticker.IsEmpty()
}

// String implement fmt.Stringer , return the string representation of Asset
func (a Asset) String() string {
	div := "."
	if a.Synth {
		div = "/"
	}
	if a.Trade {
		div = "~"
	}
	return fmt.Sprintf("%s%s%s", a.Chain.String(), div, a.Symbol.String())
}

// ShortCode returns the short code for the asset.
func (a Asset) ShortCode() string {
	switch a.String() {
	case "ARB.ETH":
		return "a"
	case "BTC.BTC":
		return "b"
	case "DASH.DASH":
		return "d"
	case "ETH.ETH":
		return "e"
	case "KUJI.KUJI":
		return "k"
	case "MAYA.CACAO":
		return "c"
	case "THOR.RUNE":
		return "r"
	case "XRD.XRD":
		return "x"
	case "ZEC.ZEC":
		return "z"
	default:
		return ""
	}
}

// TwoLetterShortCode returns the two letter short code for the asset.
func (a Asset) TwoLetterShortCode(version semver.Version) string {
	switch {
	case version.GTE(semver.MustParse("1.120.0")):
		return a.TwoLetterShortCodeV120()
	case version.GTE(semver.MustParse("1.112.0")):
		return a.TwoLetterShortCodeV112()
	default:
		return a.TwoLetterShortCodeV1()
	}
}

// TwoLetterShortCodeV118 returns the two letter short code for the asset on version 118.
func (a Asset) TwoLetterShortCodeV120() string {
	switch a.String() {
	case "ARB.ETH":
		return "ae"
	case "ARB.USDT-0XFD086BC7CD5C481DCC9C85EBE478A1C0B69FCBB9":
		return "at"
	case "ARB.USDC-0XAF88D065E77C8CC2239327C5EDB3A432268E5831":
		return "ac"
	case "ARB.DAI-0XDA10009CBD5D07DD0CECC66161FC93D7C9000DA1":
		return "ad"
	case "ARB.PEPE-0X25D887CE7A35172C62FEBFD67A1856F20FAEBB00":
		return "ap"
	case "ARB.WSTETH-0X5979D7B546E38E414F7E9822514BE443A4800529":
		return "aw"
	case "ARB.WBTC-0X2F2A2543B76A4166549F7AAB2E75BEF0AEFC5B0F":
		return "ab"
	case "BTC.BTC":
		return "bb"
	case "DASH.DASH":
		return "dd"
	case "ETH.USDT-0XDAC17F958D2EE523A2206206994597C13D831EC7":
		return "et"
	case "ETH.USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48":
		return "ec"
	case "ETH.PEPE-0X6982508145454CE325DDBE47A25D4EC3D2311933":
		return "ep"
	case "ETH.WSTETH-0X7F39C581F595B53C5CB19BD0B3F8DA6C935E2CA0":
		return "ew"
	case "KUJI.USK":
		return "ku"
	case "MAYA.CACAO":
		return "mc"
	case "THOR.RUNE":
		return "tr"
	case "XRD.XRD":
		return "xx"
	case "ETH.ETH":
		return "ee"
	case "KUJI.KUJI":
		return "kk"
	case "ZEC.ZEC":
		return "zz"
	default:
		return ""
	}
}

// TwoLetterShortCodeV112 returns the two letter short code for the asset on version 112.
func (a Asset) TwoLetterShortCodeV112() string {
	switch a.String() {
	case "ARB.ETH":
		return "ae"
	case "ARB.USDT-0XFD086BC7CD5C481DCC9C85EBE478A1C0B69FCBB9":
		return "at"
	case "ARB.USDC-0XAF88D065E77C8CC2239327C5EDB3A432268E5831":
		return "ac"
	case "ARB.DAI-0XDA10009CBD5D07DD0CECC66161FC93D7C9000DA1":
		return "ad"
	case "ARB.PEPE-0X25D887CE7A35172C62FEBFD67A1856F20FAEBB00":
		return "ap"
	case "ARB.WSTETH-0X5979D7B546E38E414F7E9822514BE443A4800529":
		return "aw"
	case "ARB.WBTC-0X2F2A2543B76A4166549F7AAB2E75BEF0AEFC5B0F":
		return "ab"
	case "BTC.BTC":
		return "bb"
	case "DASH.DASH":
		return "dd"
	case "ETH.USDT-0XDAC17F958D2EE523A2206206994597C13D831EC7":
		return "et"
	case "ETH.USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48":
		return "ec"
	case "ETH.PEPE-0X6982508145454CE325DDBE47A25D4EC3D2311933":
		return "ep"
	case "ETH.WSTETH-0X7F39C581F595B53C5CB19BD0B3F8DA6C935E2CA0":
		return "ew"
	case "KUJI.USK":
		return "ku"
	case "MAYA.CACAO":
		return "mc"
	case "THOR.RUNE":
		return "tr"
	case "XRD.XRD":
		return "xx"
	case "ETH.ETH":
		return "ee"
	case "KUJI.KUJI":
		return "kk"
	default:
		return ""
	}
}

// TwoLetterShortCodeV1 returns the two letter short code for the asset.
func (a Asset) TwoLetterShortCodeV1() string {
	switch a.String() {
	case "ARB.ETH":
		return "ae"
	case "ARB.USDT-0XFD086BC7CD5C481DCC9C85EBE478A1C0B69FCBB9":
		return "at"
	case "ARB.USDC-0XAF88D065E77C8CC2239327C5EDB3A432268E5831":
		return "ac"
	case "ARB.DAI-0XDA10009CBD5D07DD0CECC66161FC93D7C9000DA1":
		return "ad"
	case "ARB.PEPE-0X25D887CE7A35172C62FEBFD67A1856F20FAEBB00":
		return "ap"
	case "ARB.WSTETH-0X5979D7B546E38E414F7E9822514BE443A4800529":
		return "aw"
	case "ARB.WBTC-0X2F2A2543B76A4166549F7AAB2E75BEF0AEFC5B0F":
		return "ab"
	case "BTC.BTC":
		return "bb"
	case "DASH.DASH":
		return "dd"
	case "ETH.USDT-0XDAC17F958D2EE523A2206206994597C13D831EC7":
		return "et"
	case "ETH.USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48":
		return "ec"
	case "ETH.PEPE-0X6982508145454CE325DDBE47A25D4EC3D2311933":
		return "ep"
	case "ETH.WSTETH-0X7F39C581F595B53C5CB19BD0B3F8DA6C935E2CA0":
		return "ew"
	case "KUJI.USK":
		return "ku"
	case "MAYA.CACAO":
		return "mc"
	case "THOR.RUNE":
		return "tr"
	case "XRD.XRD":
		return "xx"
	default:
		return ""
	}
}

// IsGasAsset check whether asset is base asset used to pay for gas
func (a Asset) IsGasAsset() bool {
	gasAsset := a.GetChain().GetGasAsset()
	if gasAsset.IsEmpty() {
		return false
	}
	return a.Equals(gasAsset)
}

// IsCacao is a helper function ,return true only when the asset represent RUNE
func (a Asset) IsBase() bool {
	return a.Equals(BaseNative)
}

// IsNativeRune is a helper function, return true only when the asset represent NATIVE RUNE
func (a Asset) IsNativeBase() bool {
	return a.IsBase() && a.Chain.IsBASEChain()
}

// IsNative is a helper function, returns true when the asset is a native
// asset to THORChain (ie rune, a synth, etc)
func (a Asset) IsNative() bool {
	return a.GetChain().IsBASEChain()
}

// IsBNB is a helper function, return true only when the asset represent BNB
func (a Asset) IsBNB() bool {
	return a.Equals(BNBAsset)
}

// MarshalJSON implement Marshaler interface
func (a Asset) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}

// UnmarshalJSON implement Unmarshaler interface
func (a *Asset) UnmarshalJSON(data []byte) error {
	var err error
	var assetStr string
	if err = json.Unmarshal(data, &assetStr); err != nil {
		return err
	}
	if assetStr == "." {
		*a = EmptyAsset
		return nil
	}
	*a, err = NewAsset(assetStr)
	return err
}

// MarshalJSONPB implement jsonpb.Marshaler
func (a Asset) MarshalJSONPB(*jsonpb.Marshaler) ([]byte, error) {
	return a.MarshalJSON()
}

// UnmarshalJSONPB implement jsonpb.Unmarshaler
func (a *Asset) UnmarshalJSONPB(unmarshal *jsonpb.Unmarshaler, content []byte) error {
	return a.UnmarshalJSON(content)
}

// Contains checks if the array contains the specified element
func (as *Assets) Contains(a Asset) bool {
	for _, asset := range *as {
		if asset.Equals(a) {
			return true
		}
	}
	return false
}

// BaseAsset return RUNE Asset depends on different environment
func BaseAsset() Asset {
	return BaseNative
}

// Replace pool name "." with a "-" for Mimir key checking.
func (a Asset) MimirString() string {
	return a.Chain.String() + "-" + a.Symbol.String()
}

// GetAsset returns true if the asset exists in the list of assets
func ContainsAsset(asset Asset, assets []Asset) bool {
	for _, a := range assets {
		if a.Equals(asset) {
			return true
		}
	}
	return false
}

// ConvertMayachainAmountToWei converts amt in 1e8 decimals to wei (1e18 decimals)
func ConvertMayachainAmountToWei(amt *big.Int) *big.Int {
	return big.NewInt(0).Mul(amt, big.NewInt(MayachainAmountToWeiFactor))
}

// ConvertMayachainAmountToCentiWei converts amt in 1e8 decimals to centiwei (1e16 decimals)
func ConvertMayachainAmountToCentiWei(amt *big.Int) *big.Int {
	return big.NewInt(0).Mul(amt, big.NewInt(MayachainAmountToCentiWeiFactor))
}
