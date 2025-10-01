package mayanode

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
	"gitlab.com/mayachain/mayanode/common"
	sdk "gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/config"
	openapi "gitlab.com/mayachain/mayanode/openapi/gen"
)

////////////////////////////////////////////////////////////////////////////////////////
// Init
////////////////////////////////////////////////////////////////////////////////////////

var mayanodeURL string

func init() {
	config.Init()
	mayanodeURL = config.GetBifrost().MayaChain.ChainHost
	if !strings.HasPrefix(mayanodeURL, "http") {
		mayanodeURL = "http://" + mayanodeURL
	}
}

func BaseURL() string {
	return mayanodeURL
}

////////////////////////////////////////////////////////////////////////////////////////
// Exported
////////////////////////////////////////////////////////////////////////////////////////

func GetBalances(addr common.Address) (common.Coins, error) {
	url := fmt.Sprintf("%s/cosmos/bank/v1beta1/balances/%s", mayanodeURL, addr)
	var balances struct {
		Balances []struct {
			Denom  string `json:"denom"`
			Amount string `json:"amount"`
		} `json:"balances"`
	}
	err := Get(url, &balances)
	if err != nil {
		return nil, err
	}

	// convert to common.Coins
	coins := make(common.Coins, 0, len(balances.Balances))
	for _, balance := range balances.Balances {
		amount, err := strconv.ParseUint(balance.Amount, 10, 64)
		if err != nil {
			return nil, err
		}
		asset, err := common.NewAsset(strings.ToUpper(balance.Denom))
		if err != nil {
			return nil, err
		}
		coins = append(coins, common.NewCoin(asset, sdk.NewUint(amount)))
	}

	return coins, nil
}

func GetInboundAddress(chain common.Chain) (address common.Address, router *common.Address, err error) {
	url := fmt.Sprintf("%s/mayachain/inbound_addresses", mayanodeURL)
	var inboundAddresses []openapi.InboundAddress
	err = Get(url, &inboundAddresses)
	if err != nil {
		return "", nil, err
	}

	// find address for chain
	for _, inboundAddress := range inboundAddresses {
		if *inboundAddress.Chain == string(chain) {
			if inboundAddress.Router != nil {
				router = new(common.Address)
				*router = common.Address(*inboundAddress.Router)
			}
			return common.Address(*inboundAddress.Address), router, nil
		}
	}

	return "", nil, fmt.Errorf("no inbound address found for chain %s", chain)
}

func GetLiquidityProviders(asset common.Asset) ([]openapi.LiquidityProvider, error) {
	url := fmt.Sprintf("%s/mayachain/pool/%s/liquidity_providers", mayanodeURL, asset.String())
	var liquidityProviders []openapi.LiquidityProvider
	err := Get(url, &liquidityProviders)
	return liquidityProviders, err
}

func GetPools() ([]openapi.Pool, error) {
	url := fmt.Sprintf("%s/mayachain/pools", mayanodeURL)
	var pools []openapi.Pool
	err := Get(url, &pools)
	return pools, err
}

func GetPool(asset common.Asset) (openapi.Pool, error) {
	url := fmt.Sprintf("%s/mayachain/pool/%s", mayanodeURL, asset.String())
	var pool openapi.Pool
	err := Get(url, &pool)
	return pool, err
}

func GetSwapQuote(from, to common.Asset, amount sdk.Uint) (openapi.QuoteSwapResponse, error) {
	baseURL := fmt.Sprintf("%s/mayachain/quote/swap", mayanodeURL)
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return openapi.QuoteSwapResponse{}, err
	}
	params := url.Values{}
	params.Add("from_asset", from.String())
	params.Add("to_asset", to.String())
	params.Add("amount", amount.String())
	parsedURL.RawQuery = params.Encode()
	url := parsedURL.String()

	var quote openapi.QuoteSwapResponse
	err = Get(url, &quote)
	log.Debug().Str("url", url).Msg("get swap quote")
	return quote, err
}

func GetTxStages(txid string) (openapi.TxStagesResponse, error) {
	url := fmt.Sprintf("%s/mayachain/tx/stages/%s", mayanodeURL, txid)
	var stages openapi.TxStagesResponse
	err := Get(url, &stages)
	return stages, err
}

func GetBlock(height int64) (openapi.BlockResponse, error) {
	url := fmt.Sprintf("%s/mayachain/block", mayanodeURL)
	if height > 0 {
		url = fmt.Sprintf("%s?height=%d", url, height)
	}
	var block openapi.BlockResponse
	err := Get(url, &block)
	return block, err
}

func GetTradeAccount(address common.Address) ([]openapi.TradeAccountResponse, error) {
	url := fmt.Sprintf("%s/mayachain/trade/account/%s", mayanodeURL, address.String())
	var tradeAccount []openapi.TradeAccountResponse
	err := Get(url, &tradeAccount)
	return tradeAccount, err
}

func Get(url string, target interface{}) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("(%s) HTTP: %d => %s", url, resp.StatusCode, body)
	}

	// extract error if the request failed
	type ErrorResponse struct {
		ErrorMsg string `json:"error"`
	}
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	errResp := ErrorResponse{}
	err = json.Unmarshal(buf, &errResp)
	if err == nil && errResp.ErrorMsg != "" {
		return fmt.Errorf("error message: %s", errResp.ErrorMsg)
	}

	// decode response
	return json.Unmarshal(buf, target)
}
