package mayachain

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"strconv"
	"strings"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	abci "github.com/tendermint/tendermint/abci/types"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	openapi "gitlab.com/mayachain/mayanode/openapi/gen"
	mem "gitlab.com/mayachain/mayanode/x/mayachain/memo"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

// -------------------------------------------------------------------------------------
// Config
// -------------------------------------------------------------------------------------

const (
	// heightParam    = "height"
	fromAssetParam = "from_asset"
	toAssetParam   = "to_asset"
	assetParam     = "asset"
	addressParam   = "address"
	// loanOwnerParam           = "loan_owner"
	withdrawBasisPointsParam = "withdraw_bps"
	amountParam              = "amount"
	// repayBpsParam             = "repay_bps"
	destinationParam           = "destination"
	toleranceBasisPointsParam  = "tolerance_bps"
	affiliateParam             = "affiliate"
	affiliateBpsParam          = "affiliate_bps"
	liquidityToleranceBpsParam = "liquidity_tolerance_bps"
	// affiliateParams            = "affiliates"
	// affiliateBpsParam         = "affiliate_bps"
	minOutParam        = "min_out"
	intervalParam      = "streaming_interval"
	quantityParam      = "streaming_quantity"
	refundAddressParam = "refund_address"

	quoteWarning         = "Do not cache this response. Do not send funds after the expiry."
	quoteExpiration      = 15 * time.Minute
	ethBlockRewardAndFee = 3 * 1e18
)

// var nullLogger = &log.TendermintLogWrapper{Logger: zerolog.New(io.Discard)}

// debug logger
// var debug_logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Caller().Logger()
// var debug_tmlogger = &log.TendermintLogWrapper{Logger: debug_logger}
// use ctx.WithLogger(debug_tmlogger)

// -------------------------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------------------------

func quoteErrorResponse(err error) ([]byte, error) {
	return json.Marshal(map[string]string{"error": err.Error()})
}

func quoteParseParams(data []byte) (params url.Values, err error) {
	// parse the query parameters
	u, err := url.ParseRequestURI(string(data))
	if err != nil {
		return nil, fmt.Errorf("bad params: %w", err)
	}

	// error if parameters were not provided
	if len(u.Query()) == 0 {
		return nil, fmt.Errorf("no parameters provided")
	}

	return u.Query(), nil
}

func quoteParseAddress(ctx cosmos.Context, mgr *Mgrs, addrString string, chain common.Chain) (common.Address, error) {
	if addrString == "" {
		return common.NoAddress, nil
	}

	// attempt to parse a raw address
	addr, err := common.NewAddress(addrString, mgr.GetVersion())
	if err == nil {
		return addr, nil
	}

	// attempt to lookup a mayaname address
	name, err := mgr.Keeper().GetMAYAName(ctx, addrString)
	if err != nil {
		return common.NoAddress, fmt.Errorf("unable to parse address: %w", err)
	}

	// find the address for the correct chain
	for _, alias := range name.Aliases {
		if alias.Chain.Equals(chain) {
			return alias.Address, nil
		}
	}

	return common.NoAddress, fmt.Errorf("no mayaname alias for chain %s", chain)
}

func quoteParseAddressWithBps(ctx cosmos.Context, mgr *Mgrs, addrString string, chain common.Chain) (common.Address, cosmos.Uint, error) {
	bps := types.EmptyBps
	if addrString == "" {
		return common.NoAddress, bps, nil
	}

	// attempt to parse a raw address
	addr, err := common.NewAddress(addrString, mgr.GetVersion())
	if err == nil {
		return addr, bps, nil
	}

	// attempt to lookup a mayaname address
	name, err := mgr.Keeper().GetMAYAName(ctx, addrString)
	if err != nil {
		return common.NoAddress, bps, fmt.Errorf("unable to parse address: %w", err)
	}
	if !name.GetAffiliateBps().IsZero() {
		bps = name.GetAffiliateBps()
	}

	// find the address for the correct chain
	for _, alias := range name.Aliases {
		if alias.Chain.Equals(chain) {
			return alias.Address, bps, nil
		}
	}
	if chain.IsBASEChain() && !name.Owner.Empty() {
		owner := common.Address(name.Owner.String())
		return owner, bps, nil
	}

	return common.NoAddress, bps, fmt.Errorf("no mayaname alias for chain %s", chain)
}

// parseMultipleAffiliateParams - attempts to parse one or more affiliates + affiliate bps
func parseMultipleAffiliateParams(ctx cosmos.Context, mgr *Mgrs, affParamsIn []string, affFeeBpsStrIn []string) (affParams []string, affiliateBps []sdk.Uint, affiliateForMemo []sdk.Uint, totalBps cosmos.Uint, err error) {
	totalBps = cosmos.ZeroUint()

	// split every affiliate name param by '/'
	for i := 0; i < len(affParamsIn); i++ {
		affParamsStrs := strings.Split(affParamsIn[i], "/")
		affParams = append(affParams, affParamsStrs...)
	}

	// affiliate bpss as provided, empty bps set to EmptyBps, needed for constructing the correct memo
	for i := 0; i < len(affFeeBpsStrIn); i++ {
		// split every affiliate bps param by '/'
		affFeeBpsStrs := strings.Split(affFeeBpsStrIn[i], "/")
		for _, affFeeBps1 := range affFeeBpsStrs {
			if affFeeBps1 == "" {
				affiliateForMemo = append(affiliateForMemo, types.EmptyBps)
			} else {
				var val cosmos.Uint
				val, err = cosmos.ParseUint(affFeeBps1)
				if err != nil {
					return affParams, affiliateBps, affiliateForMemo, totalBps, fmt.Errorf("fail to parse Uint %s, error: %w", affFeeBps1, err)
				}
				affiliateForMemo = append(affiliateForMemo, val)
			}
		}
	}
	maxAffiliateFeeBasisPoints := uint64(mgr.Keeper().GetConfigInt64(ctx, constants.MaxAffiliateFeeBasisPoints))
	affiliateBps, totalBps, err = mem.GetMultipleAffiliatesAndBps(ctx, mgr.Keeper(), false, cosmos.NewUint(maxAffiliateFeeBasisPoints), affParams, affiliateForMemo)

	return affParams, affiliateBps, affiliateForMemo, totalBps, err
}

func quoteHandleAffiliate(ctx cosmos.Context, mgr *Mgrs, params url.Values, amount sdk.Uint) (affiliate common.Address, memo string, bps, newAmount, affiliateAmt sdk.Uint, explicitBps bool, err error) {
	// parse affiliate
	bps = sdk.ZeroUint()
	affAmt := sdk.ZeroUint()
	memo = "" // do not resolve mayaname for the memo
	if len(params[affiliateParam]) > 0 {
		affiliate, bps, err = quoteParseAddressWithBps(ctx, mgr, params[affiliateParam][0], common.BASEChain)
		if err != nil {
			err = fmt.Errorf("bad affiliate address: %w", err)
			return
		}
		if affiliate.String() != params[affiliateParam][0] {
			memo = params[affiliateParam][0]
		}
	}

	// parse affiliate fee
	if len(params[affiliateBpsParam]) > 0 {
		bps, err = sdk.ParseUint(params[affiliateBpsParam][0])
		if err != nil {
			err = fmt.Errorf("bad affiliate fee: %w", err)
			return
		}
		explicitBps = true
	}

	// verify affiliate fee
	// if AffiliateBasisPoints provided, it must not be greater than MaxAffiliateFeeBasisPoints
	maxAffiliateFeeBasisPoints := uint64(mgr.Keeper().GetConfigInt64(ctx, constants.MaxAffiliateFeeBasisPoints))
	if bps.GT(sdk.NewUint(maxAffiliateFeeBasisPoints)) {
		err = fmt.Errorf("affiliate fee basis points must not exceed %d", maxAffiliateFeeBasisPoints)
		return
	}

	// compute the new swap amount if an affiliate fee will be taken first
	if affiliate != common.NoAddress && !bps.IsZero() {
		// calculate the affiliate amount
		affAmt = common.GetSafeShare(
			bps,
			cosmos.NewUint(10000),
			amount,
		)

		// affiliate fee modifies amount at observation before the swap
		amount = amount.Sub(affAmt)
	}

	return affiliate, memo, bps, amount, affAmt, explicitBps, nil
}

func quoteReverseFuzzyAsset(ctx cosmos.Context, mgr *Mgrs, asset common.Asset) (common.Asset, error) {
	// get all pools
	pools, err := mgr.Keeper().GetPools(ctx)
	if err != nil {
		return asset, fmt.Errorf("failed to get pools: %w", err)
	}

	// return the asset if no symbol to shorten
	aSplit := strings.Split(asset.Symbol.String(), "-")
	if len(aSplit) == 1 {
		return asset, nil
	}

	// find all other assets that match the chain and ticker
	// (without exactly matching the symbol)
	addressMatches := []string{}
	for _, p := range pools {
		if p.IsAvailable() && !p.IsEmpty() && !p.Asset.IsVaultAsset() &&
			!p.Asset.Symbol.Equals(asset.Symbol) &&
			p.Asset.Chain.Equals(asset.Chain) && p.Asset.Ticker.Equals(asset.Ticker) {
			pSplit := strings.Split(p.Asset.Symbol.String(), "-")
			if len(pSplit) != 2 {
				return asset, fmt.Errorf("ambiguous match: %s", p.Asset.Symbol)
			}
			addressMatches = append(addressMatches, pSplit[1])
		}
	}

	if len(addressMatches) == 0 { // if only one match, drop the address
		asset.Symbol = common.Symbol(asset.Ticker)
	} else { // find the shortest unique suffix of the asset symbol
		address := aSplit[1]

		for i := len(address) - 1; i > 0; i-- {
			if !hasSuffixMatch(address[i:], addressMatches) {
				asset.Symbol = common.Symbol(
					fmt.Sprintf("%s-%s", asset.Ticker, address[i:]),
				)
				break
			}
		}
	}

	return asset, nil
}

func hasSuffixMatch(suffix string, values []string) bool {
	for _, value := range values {
		if strings.HasSuffix(value, suffix) {
			return true
		}
	}
	return false
}

// NOTE: streamingQuantity > 0 is a precondition.
func quoteSimulateSwap(ctx cosmos.Context, mgr *Mgrs, amount sdk.Uint, msg *MsgSwap, streamingQuantity uint64) (
	res *openapi.QuoteSwapResponse, emitAmount, outboundFeeAmount sdk.Uint, err error,
) {
	// should be unreachable
	if streamingQuantity == 0 {
		return nil, sdk.ZeroUint(), sdk.ZeroUint(), fmt.Errorf("streaming quantity must be greater than zero")
	}

	msg.Tx.Coins[0].Amount = msg.Tx.Coins[0].Amount.QuoUint64(streamingQuantity)

	// use the first active node account as the signer
	nodeAccounts, err := mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return nil, sdk.ZeroUint(), sdk.ZeroUint(), fmt.Errorf("no active node accounts: %w", err)
	}
	msg.Signer = nodeAccounts[0].NodeAddress

	// simulate the swap
	events, err := simulateInternal(ctx, mgr, msg)
	if err != nil {
		return nil, sdk.ZeroUint(), sdk.ZeroUint(), err
	}

	// extract events
	var swaps []map[string]string
	var fee map[string]string
	for _, e := range events {
		switch e.Type {
		case "swap":
			swaps = append(swaps, eventMap(e))
		case "fee":
			fee = eventMap(e)
		}
	}
	finalSwap := swaps[len(swaps)-1]

	// parse outbound fee from event
	outboundFeeCoin, err := common.ParseCoin(fee["coins"])
	if err != nil {
		return nil, sdk.ZeroUint(), sdk.ZeroUint(), fmt.Errorf("unable to parse outbound fee coin: %w", err)
	}
	outboundFeeAmount = outboundFeeCoin.Amount

	// parse outbound amount from event
	emitCoin, err := common.ParseCoin(finalSwap["emit_asset"])
	if err != nil {
		return nil, sdk.ZeroUint(), sdk.ZeroUint(), fmt.Errorf("unable to parse emit coin: %w", err)
	}
	emitAmount = emitCoin.Amount.MulUint64(streamingQuantity)

	// sum the liquidity fees and convert to target asset
	liquidityFee := sdk.ZeroUint()
	for _, s := range swaps {
		liquidityFee = liquidityFee.Add(sdk.NewUintFromString(s["liquidity_fee_in_cacao"]))
	}
	var targetPool types.Pool
	if !msg.TargetAsset.IsNativeBase() {
		targetPool, err = mgr.Keeper().GetPool(ctx, msg.TargetAsset.GetLayer1Asset())
		if err != nil {
			return nil, sdk.ZeroUint(), sdk.ZeroUint(), fmt.Errorf("unable to get pool: %w", err)
		}
		liquidityFee = targetPool.RuneValueInAsset(liquidityFee)
	}
	liquidityFee = liquidityFee.MulUint64(streamingQuantity)

	slipFeeAddedBasisPoints := mgr.Keeper().GetConfigInt64(ctx, constants.SlipFeeAddedBasisPoints)

	// compute slip based on emit amount instead of slip in event to handle double swap
	slippageBps := liquidityFee.MulUint64(10000).Quo(emitAmount.Add(liquidityFee))
	slippageBps = slippageBps.AddUint64(uint64(slipFeeAddedBasisPoints))

	// build fees
	totalFees := liquidityFee.Add(outboundFeeAmount)
	fees := openapi.QuoteFees{
		Asset:       msg.TargetAsset.String(),
		Liquidity:   liquidityFee.String(),
		Outbound:    wrapString(outboundFeeAmount.String()),
		Total:       totalFees.String(),
		SlippageBps: slippageBps.BigInt().Int64(),
		TotalBps:    totalFees.MulUint64(10000).Quo(emitAmount.Add(totalFees)).BigInt().Int64(),
	}
	// build response from simulation result events
	return &openapi.QuoteSwapResponse{
		ExpectedAmountOut: emitAmount.String(),
		Fees:              fees,
	}, emitAmount, outboundFeeAmount, nil
}

func quoteInboundInfo(ctx cosmos.Context, mgr *Mgrs, amount sdk.Uint, chain common.Chain, asset common.Asset) (address, router common.Address, confirmations int64, err error) {
	// If inbound chain is BASEChain there is no inbound address
	if chain.IsBASEChain() {
		address = common.NoAddress
		router = common.NoAddress
	} else {
		// get the most secure vault for inbound
		// trunk-ignore(golangci-lint/govet): shadow
		active, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
		if err != nil {
			return common.NoAddress, common.NoAddress, 0, err
		}
		constAccessor := mgr.GetConstants()
		signingTransactionPeriod := constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
		vault := mgr.Keeper().GetMostSecure(ctx, active, signingTransactionPeriod)
		address, err = vault.GetAddress(chain)
		if err != nil {
			return common.NoAddress, common.NoAddress, 0, err
		}

		router = common.NoAddress
		if chain.IsEVM() {
			router = vault.GetContract(chain).Router
		}
	}

	// estimate the inbound confirmation count blocks: ceil(amount/coinbase * conf adjustment)
	// Get the ConfMultiplierBasisPointsId mimir value for the given chain
	confMultiplierKey := fmt.Sprintf("ConfMultiplierBasisPoints-%s", chain.String())
	confMul, err := mgr.Keeper().GetMimir(ctx, confMultiplierKey)
	if confMul < 0 || err != nil {
		confMul = int64(constants.MaxBasisPts)
	}
	if chain.DefaultCoinbase() > 0 {
		confValue := common.GetUncappedShare(cosmos.NewUint(uint64(confMul)), cosmos.NewUint(constants.MaxBasisPts), cosmos.NewUint(uint64(chain.DefaultCoinbase())*common.One))
		confirmations = amount.Quo(confValue).BigInt().Int64()
		if !amount.Mod(confValue).IsZero() {
			confirmations++
		}
	} else if chain.Equals(common.ETHChain) {
		// copying logic from getBlockRequiredConfirmation of ethereum.go
		// convert amount to ETH
		gasAssetAmount, err := quoteConvertAsset(ctx, mgr, asset, amount, chain.GetGasAsset())
		if err != nil {
			return common.NoAddress, common.NoAddress, 0, fmt.Errorf("unable to convert asset: %w", err)
		}
		gasAssetAmountWei := common.ConvertMayachainAmountToWei(gasAssetAmount.BigInt())
		confValue := common.GetUncappedShare(cosmos.NewUint(uint64(confMul)), cosmos.NewUint(constants.MaxBasisPts), cosmos.NewUintFromBigInt(big.NewInt(ethBlockRewardAndFee)))
		confirmations = int64(cosmos.NewUintFromBigInt(gasAssetAmountWei).MulUint64(2).Quo(confValue).Uint64())
	}

	// max confirmation adjustment for btc and eth
	if chain.Equals(common.BTCChain) || chain.Equals(common.ETHChain) {
		maxConfKey := fmt.Sprintf("MaxConfirmations-%s", chain.String())
		maxConfirmations, err := mgr.Keeper().GetMimir(ctx, maxConfKey)
		if maxConfirmations < 0 || err != nil {
			maxConfirmations = 0
		}
		if maxConfirmations > 0 && confirmations > maxConfirmations {
			confirmations = maxConfirmations
		}
	}

	// min confirmation adjustment
	confFloor := map[common.Chain]int64{
		common.ETHChain:  2,
		common.DASHChain: 2,
	}
	if floor := confFloor[chain]; confirmations < floor {
		confirmations = floor
	}

	return address, router, confirmations, nil
}

func quoteOutboundInfo(ctx cosmos.Context, mgr *Mgrs, coin common.Coin) (int64, error) {
	toi := TxOutItem{
		Memo: "OUT:-",
		Coin: coin,
	}
	outboundHeight, err := mgr.txOutStore.CalcTxOutHeight(ctx, mgr.GetVersion(), toi)
	if err != nil {
		return 0, err
	}
	return outboundHeight - ctx.BlockHeight(), nil
}

// quoteConvertAsset - converts amount to target asset using MAYAChain pools
func quoteConvertAsset(ctx cosmos.Context, mgr *Mgrs, fromAsset common.Asset, amount sdk.Uint, toAsset common.Asset) (sdk.Uint, error) {
	// no conversion necessary
	if fromAsset.Equals(toAsset) {
		return amount, nil
	}

	// convert to rune
	if !fromAsset.IsBase() {
		// get the fromPool for the from asset
		fromPool, err := mgr.Keeper().GetPool(ctx, fromAsset.GetLayer1Asset())
		if err != nil {
			return sdk.ZeroUint(), fmt.Errorf("failed to get pool: %w", err)
		}

		// ensure pool exists
		if fromPool.IsEmpty() {
			return sdk.ZeroUint(), fmt.Errorf("pool does not exist")
		}

		amount = fromPool.AssetValueInRune(amount)
	}

	// convert to target asset
	if !toAsset.IsBase() {

		toPool, err := mgr.Keeper().GetPool(ctx, toAsset.GetLayer1Asset())
		if err != nil {
			return sdk.ZeroUint(), fmt.Errorf("failed to get pool: %w", err)
		}

		// ensure pool exists
		if toPool.IsEmpty() {
			return sdk.ZeroUint(), fmt.Errorf("pool does not exist")
		}

		amount = toPool.RuneValueInAsset(amount)
	}

	return amount, nil
}

// -------------------------------------------------------------------------------------
// Swap
// -------------------------------------------------------------------------------------

// calculateMinSwapAmount returns the recommended minimum swap amount
// The recommended min swap amount is:
// - MAX(outbound_fee(src_chain), outbound_fee(dest_chain)) * 4 (priced in the inbound asset)
//
// The reason the base value is the MAX of the outbound fees of each chain is because if
// the swap is refunded the input amount will need to cover the outbound fee of the
// source chain. A 4x buffer is applied because outbound fees can spike quickly, meaning
// the original input amount could be less than the new outbound fee. If this happens
// and the swap is refunded, the refund will fail, and the user will lose the entire
// input amount. The min amount could also be determined by the affiliate bps of the
// swap. The affiliate bps of the input amount needs to be enough to cover the native tx fee for the
// affiliate swap to RUNE. In this case, we give a 2x buffer on the native_tx_fee so the
// affiliate receives some amount after the fee is deducted.
func calculateMinSwapAmount(ctx cosmos.Context, mgr *Mgrs, fromAsset, toAsset common.Asset, affiliateBps cosmos.Uint) (cosmos.Uint, error) {
	srcOutboundFee := mgr.GasMgr().GetFee(ctx, fromAsset.GetChain(), fromAsset)
	destOutboundFee := mgr.GasMgr().GetFee(ctx, toAsset.GetChain(), toAsset)

	if fromAsset.GetChain().IsBASEChain() && toAsset.GetChain().IsBASEChain() {
		// If this is a purely THORChain swap, no need to give a 4x buffer since outbound fees do not change
		// 2x buffer should suffice
		return srcOutboundFee.Mul(cosmos.NewUint(2)), nil
	}

	destInSrcAsset, err := quoteConvertAsset(ctx, mgr, toAsset, destOutboundFee, fromAsset)
	if err != nil {
		return cosmos.ZeroUint(), fmt.Errorf("fail to convert dest fee to src asset %w", err)
	}

	minSwapAmount := srcOutboundFee
	if destInSrcAsset.GT(srcOutboundFee) {
		minSwapAmount = destInSrcAsset
	}

	minSwapAmount = minSwapAmount.Mul(cosmos.NewUint(4))

	if affiliateBps.GT(cosmos.ZeroUint()) {
		nativeTxFeeRune := mgr.GasMgr().GetFee(ctx, common.BASEChain, common.BaseNative)
		affSwapAmountRune := nativeTxFeeRune.Mul(cosmos.NewUint(2))
		mainSwapAmountRune := affSwapAmountRune.Mul(cosmos.NewUint(10_000)).Quo(affiliateBps)

		mainSwapAmount, err := quoteConvertAsset(ctx, mgr, common.BaseAsset(), mainSwapAmountRune, fromAsset)
		if err != nil {
			return cosmos.ZeroUint(), fmt.Errorf("fail to convert main swap amount to src asset %w", err)
		}

		if mainSwapAmount.GT(minSwapAmount) {
			minSwapAmount = mainSwapAmount
		}
	}

	return minSwapAmount, nil
}

func queryQuoteSwap(ctx cosmos.Context, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	// extract parameters
	params, err := quoteParseParams(req.Data)
	if err != nil {
		return quoteErrorResponse(err)
	}

	// validate required parameters
	for _, p := range []string{fromAssetParam, toAssetParam, amountParam} {
		if len(params[p]) == 0 {
			return quoteErrorResponse(fmt.Errorf("missing required parameter %s", p))
		}
	}

	// parse assets
	fromAsset, err := common.NewAssetWithShortCodes(mgr.GetVersion(), params[fromAssetParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad from asset: %w", err))
	}
	fromAsset = fuzzyAssetMatch(ctx, mgr.Keeper(), fromAsset)
	toAsset, err := common.NewAssetWithShortCodes(mgr.GetVersion(), params[toAssetParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad to asset: %w", err))
	}
	toAsset = fuzzyAssetMatch(ctx, mgr.Keeper(), toAsset)

	// parse amount
	amount, err := cosmos.ParseUint(params[amountParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad amount: %w", err))
	}

	if amount.LT(fromAsset.Chain.DustThreshold()) {
		return quoteErrorResponse(fmt.Errorf("amount less than dust threshold"))
	}

	if len(params[toleranceBasisPointsParam]) > 0 && len(params[liquidityToleranceBpsParam]) > 0 {
		return quoteErrorResponse(fmt.Errorf("must only include one of: tolerance_bps or liquidity_tolerance_bps"))
	}

	// parse streaming interval
	streamingInterval := uint64(0) // default value
	if len(params[intervalParam]) > 0 {
		streamingInterval, err = strconv.ParseUint(params[intervalParam][0], 10, 64)
		if err != nil {
			return quoteErrorResponse(fmt.Errorf("bad streaming interval amount: %w", err))
		}
	}
	streamingQuantity := uint64(0) // default value
	if len(params[quantityParam]) > 0 {
		streamingQuantity, err = strconv.ParseUint(params[quantityParam][0], 10, 64)
		if err != nil {
			return quoteErrorResponse(fmt.Errorf("bad streaming quantity amount: %w", err))
		}
	}
	swp := StreamingSwap{
		Interval: streamingInterval,
		Deposit:  amount,
	}
	maxSwapQuantity, err := getMaxSwapQuantity(ctx, mgr, fromAsset, toAsset, swp)
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("failed to calculate max streaming swap quantity: %w", err))
	}

	// cap the streaming quantity to the max swap quantity
	if streamingQuantity > maxSwapQuantity {
		streamingQuantity = maxSwapQuantity
	}

	// if from asset is a synth, transfer asset to asgard module
	if fromAsset.IsSyntheticAsset() {
		// mint required coins to asgard so swap can be simulated
		err = mgr.Keeper().MintToModule(ctx, ModuleName, common.NewCoin(fromAsset, amount))
		if err != nil {
			return quoteErrorResponse(fmt.Errorf("failed to mint coins to module: %w", err))
		}

		err = mgr.Keeper().SendFromModuleToModule(ctx, ModuleName, AsgardName, common.NewCoins(common.NewCoin(fromAsset, amount)))
		if err != nil {
			return quoteErrorResponse(fmt.Errorf("failed to send coins to asgard: %w", err))
		}
	}

	// parse destination address or generate a random one
	sendMemo := true
	var destination common.Address
	if len(params[destinationParam]) > 0 {
		destination, err = quoteParseAddress(ctx, mgr, params[destinationParam][0], toAsset.Chain)
		if err != nil {
			return quoteErrorResponse(fmt.Errorf("bad destination address: %w", err))
		}

	} else {
		chain := common.BASEChain
		if !toAsset.IsSyntheticAsset() {
			chain = toAsset.Chain
		}
		destination, err = types.GetRandomPubkeyForChain(chain).GetAddress(chain)
		if err != nil {
			return quoteErrorResponse(fmt.Errorf("failed to generate address: %w", err))
		}
		sendMemo = false // do not send memo if destination was random
	}

	// parse tolerance basis points
	limit := sdk.ZeroUint()
	liquidityToleranceBps := sdk.ZeroUint()
	if len(params[toleranceBasisPointsParam]) > 0 {
		// validate tolerance basis points
		var toleranceBasisPoints sdk.Uint
		toleranceBasisPoints, err = sdk.ParseUint(params[toleranceBasisPointsParam][0])
		if err != nil {
			return quoteErrorResponse(fmt.Errorf("bad tolerance basis points: %w", err))
		}
		if toleranceBasisPoints.GT(sdk.NewUint(10000)) {
			return quoteErrorResponse(fmt.Errorf("tolerance basis points must be less than 10000"))
		}

		// convert to a limit of target asset amount assuming zero fees and slip
		var feelessEmit sdk.Uint
		feelessEmit, err = quoteConvertAsset(ctx, mgr, fromAsset, amount, toAsset)
		if err != nil {
			return quoteErrorResponse(err)
		}

		limit = feelessEmit.MulUint64(10000 - toleranceBasisPoints.Uint64()).QuoUint64(10000)
	} else if len(params[liquidityToleranceBpsParam]) > 0 {
		liquidityToleranceBps, err = sdk.ParseUint(params[liquidityToleranceBpsParam][0])
		if err != nil {
			return quoteErrorResponse(fmt.Errorf("bad liquidity tolerance basis points: %w", err))
		}
		if liquidityToleranceBps.GTE(sdk.NewUint(10000)) {
			return quoteErrorResponse(fmt.Errorf("liquidity tolerance basis points must be less than 10000"))
		}
	}

	// custom refund addr
	refundAddress := common.NoAddress
	if len(params[refundAddressParam]) > 0 {
		refundAddress, err = quoteParseAddress(ctx, mgr, params[refundAddressParam][0], fromAsset.Chain)
		if err != nil {
			return quoteErrorResponse(fmt.Errorf("bad refund address: %w", err))
		}
	}

	// parse affiliate params
	affiliates, affiliateBps, affiliateForMemo, totalBps, err := parseMultipleAffiliateParams(ctx, mgr, params[affiliateParam], params[affiliateBpsParam])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad affiliate params: %w", err))
	}

	// create the memo
	memo := &SwapMemo{
		MemoBase: mem.MemoBase{
			TxType: TxSwap,
			Asset:  toAsset,
		},
		Destination:           destination,
		SlipLimit:             limit,
		Affiliates:            affiliates,
		AffiliatesBasisPoints: affiliateForMemo,
		AffiliateBasisPoints:  totalBps,
		StreamInterval:        streamingInterval,
		StreamQuantity:        streamingQuantity,
		RefundAddress:         refundAddress,
	}

	memoString := memo.String()

	// trade assets must have from address on the source tx
	fromChain := fromAsset.Chain
	if fromAsset.IsSyntheticAsset() || fromAsset.IsTradeAsset() {
		fromChain = common.BASEChain
	}
	fromPubkey := types.GetRandomPubKey()
	var fromAddress common.Address
	fromAddress, err = fromPubkey.GetAddress(fromChain)
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad from address: %w", err))
	}

	// if from asset is a trade asset, create fake balance
	if fromAsset.IsTradeAsset() {
		var mayaAddr cosmos.AccAddress
		mayaAddr, err = fromPubkey.GetThorAddress()
		if err != nil {
			return quoteErrorResponse(fmt.Errorf("failed to get maya address: %w", err))
		}
		_, err = mgr.TradeAccountManager().Deposit(ctx, fromAsset, amount, mayaAddr, common.NoAddress, common.BlankTxID)
		if err != nil {
			return quoteErrorResponse(fmt.Errorf("failed to deposit trade asset: %w", err))
		}
	}

	// create the swap message
	msg := &types.MsgSwap{
		Tx: common.Tx{
			ID:          common.BlankTxID,
			Chain:       fromAsset.Chain,
			FromAddress: fromAddress,
			ToAddress:   common.NoopAddress,
			Coins: []common.Coin{
				{
					Asset:  fromAsset,
					Amount: amount,
				},
			},
			Gas: []common.Coin{{
				Asset:  common.BaseAsset(),
				Amount: sdk.NewUint(1),
			}},
			Memo: memoString,
		},
		TargetAsset:          toAsset,
		TradeTarget:          limit,
		Destination:          destination,
		AffiliateAddress:     common.NoAddress,
		AffiliateBasisPoints: cosmos.ZeroUint(),
	}

	// simulate the swap
	res, emitAmount, outboundFeeAmount, err := quoteSimulateSwap(ctx, mgr, amount, msg, 1)
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("failed to simulate swap: %w", err))
	}

	// if we're using a streaming swap, calculate emit amount by a sub-swap amount instead
	// of the full amount, then multiply the result by the swap count
	if streamingInterval > 0 && streamingQuantity == 0 {
		streamingQuantity = maxSwapQuantity
	}
	if streamingInterval > 0 && streamingQuantity > 0 {
		msg.TradeTarget = msg.TradeTarget.QuoUint64(streamingQuantity)
		// simulate the swap
		var streamRes *openapi.QuoteSwapResponse
		streamRes, emitAmount, _, err = quoteSimulateSwap(ctx, mgr, amount, msg, streamingQuantity)
		if err != nil {
			return quoteErrorResponse(fmt.Errorf("failed to simulate swap: %w", err))
		}
		res.Fees = streamRes.Fees
	}

	// TODO: After UIs have transitioned everything below the message definition above
	// should reduce to the following:
	//
	// if streamingInterval > 0 && streamingQuantity == 0 {
	//   streamingQuantity = maxSwapQuantity
	// }
	// if streamingInterval > 0 && streamingQuantity > 0 {
	//   msg.TradeTarget = msg.TradeTarget.QuoUint64(streamingQuantity)
	// }
	// res, emitAmount, outboundFeeAmount, err := quoteSimulateSwap(ctx, mgr, amount, msg, streamingQuantity)
	// if err != nil {
	//   return quoteErrorResponse(fmt.Errorf("failed to simulate swap: %w", err))
	// }

	// get the pool to calculate the affiliate amount in cacao
	var pool Pool
	if !toAsset.IsNative() {
		pool, err = mgr.Keeper().GetPool(ctx, toAsset)
		if err != nil {
			return quoteErrorResponse(fmt.Errorf("failed to get pool: %w", err))
		}
	}
	// get the native (cacao) tx fee in cacao
	outboundTxFee, err := mgr.Keeper().GetMimir(ctx, constants.OutboundTransactionFee.String())
	if outboundTxFee < 0 || err != nil {
		outboundTxFee = mgr.constAccessor.GetInt64Value(constants.OutboundTransactionFee)
	}
	transactionFee := cosmos.NewUint(uint64(outboundTxFee))
	totalAffFee := cosmos.ZeroUint()
	// attempt each affiliate fee, skipping those that won't succeed
	if len(affiliates) > 0 && len(affiliateBps) > 0 {
		// Attempt each affiliate swap
		for _, bps := range affiliateBps {
			if bps.IsZero() {
				continue
			}
			affAmtInTargetAsset := common.GetSafeShare(bps, cosmos.NewUint(10000), emitAmount)
			var affAmountInCacao cosmos.Uint
			// if target asset is cacao the affAmtInTargetAsset is already in cacao
			if toAsset.IsNative() {
				affAmountInCacao = affAmtInTargetAsset
			} else {
				affAmountInCacao = pool.AssetValueInRune(affAmtInTargetAsset)
			}
			// skip affiliate if the affiliate amount is less than the cacao tx fee
			if affAmountInCacao.LTE(transactionFee) {
				continue
			}
			totalAffFee = totalAffFee.Add(affAmtInTargetAsset)
		}
	}
	// Update fees with affiliate fee & re-calculate total fee bps
	totalAffFeeStr := totalAffFee.String()
	res.Fees.Affiliate = &totalAffFeeStr
	totalFees, err := cosmos.ParseUint(res.Fees.Total)
	if err != nil {
		return nil, fmt.Errorf("failed to parse total fees: %w", err)
	}
	totalFees = totalFees.Add(totalAffFee)
	res.Fees.Total = totalFees.String()
	res.Fees.TotalBps = totalFees.MulUint64(10000).Quo(emitAmount.Add(totalFees)).BigInt().Int64()
	emitAmount = emitAmount.Sub(totalAffFee)

	// check invariant
	if emitAmount.LT(outboundFeeAmount) {
		return quoteErrorResponse(fmt.Errorf("invariant broken: emit %s less than outbound fee %s", emitAmount, outboundFeeAmount))
	}

	// the amount out will deduct the outbound fee
	res.ExpectedAmountOut = emitAmount.Sub(outboundFeeAmount).String()

	// add liquidty_tolerance_bps to the memo
	if liquidityToleranceBps.GT(sdk.ZeroUint()) {
		outputLimit := emitAmount.Sub(outboundFeeAmount).MulUint64(10000 - liquidityToleranceBps.Uint64()).QuoUint64(10000)
		memo.SlipLimit = outputLimit
		memoString = memo.String()
	}

	// shorten the memo if necessary
	memoShortString := memo.ShortString()
	// don't need to shorten memo for BASEChain
	if !fromAsset.IsNative() {
		if len(memoShortString) < len(memoString) { // use short codes if available
			memoString = memoShortString
		} else { // otherwise attempt to shorten
			var fuzzyAsset common.Asset
			fuzzyAsset, err = quoteReverseFuzzyAsset(ctx, mgr, toAsset)
			if err == nil {
				memo.Asset = fuzzyAsset
				memoString = memo.String()
			}
		}

		// this is the shortest we can make it
		if len(memoString) > fromAsset.GetChain().MaxMemoLength() {
			return quoteErrorResponse(fmt.Errorf("generated memo too long for source chain: %s (%d/%d)", memoString, len(memoString), fromAsset.Chain.MaxMemoLength()))
		}
	}

	maxQ := int64(maxSwapQuantity)
	res.MaxStreamingQuantity = &maxQ
	var streamSwapBlocks int64
	if streamingQuantity > 0 {
		streamSwapBlocks = int64(streamingInterval) * int64(streamingQuantity-1)
	}
	res.StreamingSwapBlocks = &streamSwapBlocks
	res.StreamingSwapSeconds = wrapInt64(streamSwapBlocks * common.THORChain.ApproximateBlockMilliseconds() / 1000)

	// estimate the inbound info
	inboundAddress, routerAddress, inboundConfirmations, err := quoteInboundInfo(ctx, mgr, amount, fromAsset.GetChain(), fromAsset)
	if err != nil {
		return quoteErrorResponse(err)
	}
	res.InboundAddress = wrapString(inboundAddress.String())
	if inboundConfirmations > 0 {
		res.InboundConfirmationBlocks = wrapInt64(inboundConfirmations)
		res.InboundConfirmationSeconds = wrapInt64(inboundConfirmations * msg.Tx.Chain.ApproximateBlockMilliseconds() / 1000)
	}

	res.OutboundDelayBlocks = 0
	res.OutboundDelaySeconds = 0
	if !toAsset.Chain.IsBASEChain() {
		// estimate the outbound info
		var outboundDelay int64
		outboundDelay, err = quoteOutboundInfo(ctx, mgr, common.Coin{Asset: toAsset, Amount: emitAmount})
		if err != nil {
			return quoteErrorResponse(err)
		}
		res.OutboundDelayBlocks = outboundDelay
		res.OutboundDelaySeconds = outboundDelay * common.BASEChain.ApproximateBlockMilliseconds() / 1000
	}

	totalSeconds := res.OutboundDelaySeconds
	if res.StreamingSwapSeconds != nil && res.OutboundDelaySeconds < *res.StreamingSwapSeconds {
		totalSeconds = *res.StreamingSwapSeconds
	}
	if inboundConfirmations > 0 {
		totalSeconds += *res.InboundConfirmationSeconds
	}
	res.TotalSwapSeconds = wrapInt64(totalSeconds)

	// send memo if the destination was provided
	if sendMemo {
		res.Memo = wrapString(memoString)
	}

	// set info fields
	if fromAsset.Chain.IsEVM() {
		res.Router = wrapString(routerAddress.String())
	}
	if !fromAsset.Chain.DustThreshold().IsZero() {
		res.DustThreshold = wrapString(fromAsset.Chain.DustThreshold().String())
	}

	res.Notes = fromAsset.GetChain().InboundNotes()
	res.Warning = quoteWarning
	res.Expiry = time.Now().Add(quoteExpiration).Unix()
	minSwapAmount, err := calculateMinSwapAmount(ctx, mgr, fromAsset, toAsset, totalBps)
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("failed to calculate min amount in: %s", err.Error()))
	}
	res.RecommendedMinAmountIn = wrapString(minSwapAmount.String())

	// set inbound recommended gas for non-native swaps
	if !fromAsset.Chain.IsBASEChain() {
		inboundGas := mgr.GasMgr().GetGasRate(ctx, fromAsset.Chain)
		res.RecommendedGasRate = wrapString(inboundGas.String())
		res.GasRateUnits = wrapString(fromAsset.Chain.GetGasUnits())
	}

	return json.MarshalIndent(res, "", "  ")
}

// -------------------------------------------------------------------------------------
// Saver Deposit
// -------------------------------------------------------------------------------------

func queryQuoteSaverDeposit(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	// extract parameters
	params, err := quoteParseParams(req.Data)
	if err != nil {
		return quoteErrorResponse(err)
	}

	// validate required parameters
	for _, p := range []string{assetParam, amountParam} {
		if len(params[p]) == 0 {
			return quoteErrorResponse(fmt.Errorf("missing required parameter %s", p))
		}
	}

	// parse asset
	asset, err := common.NewAssetWithShortCodes(mgr.GetVersion(), params[assetParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad asset: %w", err))
	}
	asset = fuzzyAssetMatch(ctx, mgr.Keeper(), asset)

	// parse amount
	amount, err := cosmos.ParseUint(params[amountParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad amount: %w", err))
	}

	// parse affiliate
	affiliate, affiliateMemo, affiliateBps, depositAmount, _, _, err := quoteHandleAffiliate(ctx, mgr, params, amount)
	if err != nil {
		return quoteErrorResponse(err)
	}

	// generate deposit memo
	depositMemoComponents := []string{
		"+",
		asset.GetSyntheticAsset().String(),
		"",
		affiliateMemo,
		affiliateBps.String(),
	}
	depositMemo := strings.Join(depositMemoComponents[:2], ":")
	if affiliate != common.NoAddress && !affiliateBps.IsZero() {
		depositMemo = strings.Join(depositMemoComponents, ":")
	}

	q := url.Values{}
	q.Add("from_asset", asset.String())
	q.Add("to_asset", asset.GetSyntheticAsset().String())
	q.Add("amount", depositAmount.String())
	q.Add("destination", string(GetRandomBaseAddress())) // required param, not actually used, spoof it

	// ssInterval := mgr.Keeper().GetConfigInt64(ctx, constants.SaversStreamingSwapsInterval)
	// if ssInterval > 0 {
	// 	q.Add("streaming_interval", fmt.Sprintf("%d", ssInterval))
	// 	q.Add("streaming_quantity", fmt.Sprintf("%d", 0))
	// }

	swapReq := abci.RequestQuery{Data: []byte("/mayachain/quote/swap?" + q.Encode())}
	swapResRaw, err := queryQuoteSwap(ctx, swapReq, mgr)
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("unable to queryQuoteSwap: %w", err))
	}

	var swapRes *openapi.QuoteSwapResponse
	err = json.Unmarshal(swapResRaw, &swapRes)
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("unable to unmarshal swapRes: %w", err))
	}

	expectedAmountOut, _ := sdk.ParseUint(swapRes.ExpectedAmountOut)
	outboundFee, _ := sdk.ParseUint(*swapRes.Fees.Outbound)
	depositAmount = expectedAmountOut.Add(outboundFee)

	// use the swap result info to generate the deposit quote
	res := &openapi.QuoteSaverDepositResponse{
		// TODO: deprecate ExpectedAmountOut in future version
		ExpectedAmountOut:          wrapString(depositAmount.String()),
		ExpectedAmountDeposit:      depositAmount.String(),
		Fees:                       swapRes.Fees,
		InboundConfirmationBlocks:  swapRes.InboundConfirmationBlocks,
		InboundConfirmationSeconds: swapRes.InboundConfirmationSeconds,
		Memo:                       depositMemo,
	}

	// estimate the inbound info
	inboundAddress, _, inboundConfirmations, err := quoteInboundInfo(ctx, mgr, amount, asset.GetLayer1Asset().Chain, asset)
	if err != nil {
		return quoteErrorResponse(err)
	}
	res.InboundAddress = inboundAddress.String()
	res.InboundConfirmationBlocks = wrapInt64(inboundConfirmations)

	// set info fields
	chain := asset.GetLayer1Asset().Chain
	if !chain.DustThreshold().IsZero() {
		res.DustThreshold = wrapString(chain.DustThreshold().String())
		res.RecommendedMinAmountIn = res.DustThreshold
	}
	res.Notes = chain.InboundNotes()
	res.Warning = quoteWarning
	res.Expiry = time.Now().Add(quoteExpiration).Unix()

	// set inbound recommended gas
	inboundGas := mgr.GasMgr().GetGasRate(ctx, chain)
	res.RecommendedGasRate = inboundGas.String()
	res.GasRateUnits = chain.GetGasUnits()

	return json.MarshalIndent(res, "", "  ")
}

// -------------------------------------------------------------------------------------
// Saver Withdraw
// -------------------------------------------------------------------------------------

func queryQuoteSaverWithdraw(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	// extract parameters
	params, err := quoteParseParams(req.Data)
	if err != nil {
		return quoteErrorResponse(err)
	}

	// validate required parameters
	for _, p := range []string{assetParam, addressParam, withdrawBasisPointsParam} {
		if len(params[p]) == 0 {
			return quoteErrorResponse(fmt.Errorf("missing required parameter %s", p))
		}
	}

	// parse asset
	asset, err := common.NewAssetWithShortCodes(mgr.GetVersion(), params[assetParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad asset: %w", err))
	}
	asset = fuzzyAssetMatch(ctx, mgr.Keeper(), asset)
	asset = asset.GetSyntheticAsset() // always use the vault asset

	// parse address
	address, err := common.NewAddress(params[addressParam][0], mgr.GetVersion())
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad address: %w", err))
	}

	// parse basis points
	basisPoints, err := cosmos.ParseUint(params[withdrawBasisPointsParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad basis points: %w", err))
	}

	// validate basis points
	if basisPoints.GT(sdk.NewUint(10_000)) {
		return quoteErrorResponse(fmt.Errorf("basis points must be less than 10000"))
	}

	// get liquidity provider
	lp, err := mgr.Keeper().GetLiquidityProvider(ctx, asset, address)
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("failed to get liquidity provider: %w", err))
	}

	// get the pool
	pool, err := mgr.Keeper().GetPool(ctx, asset)
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("failed to get pool: %w", err))
	}

	// get the liquidity provider share of the pool
	lpShare := lp.GetSaversAssetRedeemValue(pool)

	// calculate the withdraw amount
	amount := common.GetSafeShare(basisPoints, sdk.NewUint(10_000), lpShare)

	q := url.Values{}
	q.Add("from_asset", asset.String())
	q.Add("to_asset", asset.GetLayer1Asset().String())
	q.Add("amount", amount.String())
	q.Add("destination", address.String()) // required param, not actually used, spoof it

	swapReq := abci.RequestQuery{Data: []byte("/mayachain/quote/swap?" + q.Encode())}
	swapResRaw, err := queryQuoteSwap(ctx, swapReq, mgr)
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("unable to queryQuoteSwap: %w", err))
	}

	var swapRes *openapi.QuoteSwapResponse
	err = json.Unmarshal(swapResRaw, &swapRes)
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("unable to unmarshal swapRes: %w", err))
	}

	// use the swap result info to generate the withdraw quote
	res := &openapi.QuoteSaverWithdrawResponse{
		ExpectedAmountOut: swapRes.ExpectedAmountOut,
		Fees:              swapRes.Fees,
		Memo:              fmt.Sprintf("-:%s:%s", asset.String(), basisPoints.String()),
		DustAmount:        asset.GetLayer1Asset().Chain.DustThreshold().Add(basisPoints).String(),
	}

	// estimate the inbound info
	inboundAddress, _, _, err := quoteInboundInfo(ctx, mgr, amount, asset.GetLayer1Asset().Chain, asset)
	if err != nil {
		return quoteErrorResponse(err)
	}
	res.InboundAddress = inboundAddress.String()

	// estimate the outbound info
	expectedAmountOut, _ := sdk.ParseUint(swapRes.ExpectedAmountOut)
	outboundCoin := common.Coin{Asset: asset.GetLayer1Asset(), Amount: expectedAmountOut}
	outboundDelay, err := quoteOutboundInfo(ctx, mgr, outboundCoin)
	if err != nil {
		return quoteErrorResponse(err)
	}
	res.OutboundDelayBlocks = outboundDelay
	res.OutboundDelaySeconds = outboundDelay * common.BASEChain.ApproximateBlockMilliseconds() / 1000

	// set info fields
	chain := asset.GetLayer1Asset().Chain
	if !chain.DustThreshold().IsZero() {
		res.DustThreshold = wrapString(chain.DustThreshold().String())
	}
	res.Notes = chain.InboundNotes()
	res.Warning = quoteWarning
	res.Expiry = time.Now().Add(quoteExpiration).Unix()

	// set inbound recommended gas
	inboundGas := mgr.GasMgr().GetGasRate(ctx, chain)
	res.RecommendedGasRate = inboundGas.String()
	res.GasRateUnits = chain.GetGasUnits()

	return json.MarshalIndent(res, "", "  ")
}
