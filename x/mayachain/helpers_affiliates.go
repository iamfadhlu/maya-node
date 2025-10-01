package mayachain

import (
	"crypto/sha256"
	"fmt"

	"github.com/blang/semver"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

// affiliate fee share
type affiliateFeeShare struct {
	amount    cosmos.Uint    // amount sent to the affiliate
	cacaoDest common.Address // mayaname alias/owner or explicit native affiliate address or NoAddress if preferred address is set
	mayaname  types.MAYAName //
	// additional fields for events
	parent    string      // parent mayaname or empty string if it's one of the main affiliates
	subFeeBps cosmos.Uint // fee bps of the parent affiliate (or of the swap amount if it's one of the main affiliates)
}

// skimAffiliateFeesWithMaxTotal - attempts to distribute the affiliate fee to the main affiliate in the memo and to each nested subaffiliate.
// Returns the total fee distributed priced in inboundCoin.Asset.
// Logic:
// 1. Parse the memo to get the affiliate mayaname or address and the main affiliate fee
// 2. For affiliate and each subaffiliate
// - If inbound coin is CACAO transfer to the affiliate
// - If inbound coin is not CACAO, swap the coin to CACAO and transfer to the affiliate
// - If affiliate is a mayaname and has a preferred asset, send CACAO to the affiliate collector

// versioned dispatcher functions
func skimAffiliateFeesWithMaxTotal(ctx cosmos.Context, mgr Manager, mainTx common.Tx, signer cosmos.AccAddress, memoStr string, maxTotalAffBps cosmos.Uint, fromModule string) (cosmos.Uint, error) {
	version := mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.118.0")):
		return skimAffiliateFeesV118(ctx, mgr, mainTx, signer, memoStr, maxTotalAffBps, fromModule)
	default:
		return skimAffiliateFeesV1(ctx, mgr, mainTx, signer, memoStr)
	}
}

func skimAffiliateFees(ctx cosmos.Context, mgr Manager, mainTx common.Tx, signer cosmos.AccAddress, memoStr string) (cosmos.Uint, error) {
	return skimAffiliateFeesWithMaxTotal(ctx, mgr, mainTx, signer, memoStr, cosmos.ZeroUint(), AsgardName)
}

func calculateAffiliateShares(ctx cosmos.Context, mgr Manager, inputAmount cosmos.Uint, affiliates []string, affiliatesBps []cosmos.Uint, maxTotalAffBps cosmos.Uint) []affiliateFeeShare {
	version := mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.118.0")):
		return calculateAffiliateSharesV118(ctx, mgr, inputAmount, affiliates, affiliatesBps, maxTotalAffBps)
	default:
		return calculateAffiliateSharesV1(ctx, mgr, inputAmount, affiliates, affiliatesBps)
	}
}

func calculateNestedAffiliateShares(ctx cosmos.Context, mgr Manager, mayaname MAYAName, inputAmt, maxTotalAffBps cosmos.Uint) (shares []affiliateFeeShare, remainingAffBps cosmos.Uint) {
	version := mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.118.0")):
		return calculateNestedAffiliateSharesV118(ctx, mgr, mayaname, inputAmt, maxTotalAffBps)
	default:
		return calculateNestedAffiliateSharesV1(ctx, mgr, mayaname, inputAmt)
	}
}

func sendShare(ctx cosmos.Context, mgr Manager, share affiliateFeeShare, swapIndex *int, fromModule string) error {
	version := mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.118.0")):
		return sendShareV118(ctx, mgr, share, swapIndex, fromModule)
	default:
		return sendShareV1(ctx, mgr, share, swapIndex)
	}
}

func swapShare(ctx cosmos.Context, mgr Manager, share affiliateFeeShare, mainTx common.Tx, signer cosmos.AccAddress, swapIndex *int) error {
	version := mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.118.0")):
		return swapShareV118(ctx, mgr, share, mainTx, signer, swapIndex)
	default:
		return swapShareV1(ctx, mgr, share, mainTx, signer, swapIndex)
	}
}

func triggerPreferredAssetSwap(ctx cosmos.Context, mgr Manager, mn MAYAName, affCol AffiliateFeeCollector, queueIndex int) error {
	version := mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.118.0")):
		return triggerPreferredAssetSwapV118(ctx, mgr, mn, affCol, queueIndex)
	case version.GTE(semver.MustParse("1.112.0")):
		return triggerPreferredAssetSwapV112(ctx, mgr, mn, affCol, queueIndex)
	}
	return fmt.Errorf("bad version (%s) for triggerPreferredAssetSwap", version.String())
}

// current version functions
func skimAffiliateFeesV118(ctx cosmos.Context, mgr Manager, mainTx common.Tx, signer cosmos.AccAddress, memoStr string, maxTotalAffBps cosmos.Uint, fromModule string) (cosmos.Uint, error) {
	// sanity checks
	if mainTx.IsEmpty() {
		return cosmos.ZeroUint(), fmt.Errorf("main tx is empty")
	}
	if mainTx.Coins[0].IsEmpty() {
		return cosmos.ZeroUint(), fmt.Errorf("coin is empty")
	}
	affiliateFeeTickGranularity := uint64(mgr.Keeper().GetConfigInt64(ctx, constants.AffiliateFeeTickGranularity))
	memo, err := ParseMemoWithMAYANames(ctx, mgr.Keeper(), memoStr)
	if err != nil {
		ctx.Logger().Error("fail to parse swap memo", "memo", memoStr, "error", err)
		return cosmos.ZeroUint(), err
	}
	affiliates := memo.GetAffiliates()
	affiliatesBps := memo.GetAffiliatesBasisPoints()
	if len(affiliates) == 0 || len(affiliatesBps) == 0 {
		return cosmos.ZeroUint(), nil
	}

	// initialize swapIndex for affiliate swaps (index 0 is reserved for the main swap)
	swapIndex := 1
	totalDistributed := cosmos.ZeroUint()
	totalSwapAmount := mainTx.Coins[0].Amount
	shares := calculateAffiliateShares(ctx, mgr, totalSwapAmount, affiliates, affiliatesBps, maxTotalAffBps)
	for _, share := range shares {
		if !share.amount.IsZero() {
			// fill fields for events
			distributed := distributeShare(ctx, mgr, share, mainTx, signer, &swapIndex, fromModule)
			totalDistributed = totalDistributed.Add(distributed)

			// Emit affiliate fee events
			// calculate the fee microbps (1/10,000th of a basis point, or 0.000001%)
			feeBpsTick := common.GetSafeShare(
				share.amount,
				totalSwapAmount,
				cosmos.NewUint(constants.MaxBasisPts).MulUint64(affiliateFeeTickGranularity),
			).BigInt().Uint64()

			event := NewEventAffiliateFee(
				mainTx.ID,                         // txId
				mainTx.Memo,                       // memo
				share.mayaname.Name,               // mayaname
				share.cacaoDest,                   // cacaoAddress
				mainTx.Coins[0].Asset,             // asset
				feeBpsTick,                        // feeBpsTick
				totalSwapAmount,                   // grossAmount
				share.amount,                      // feeAmt
				share.parent,                      // parent
				share.subFeeBps.BigInt().Uint64(), // subFeeBps
			)
			if err = mgr.EventMgr().EmitEvent(ctx, event); err != nil {
				ctx.Logger().Error("fail to emit affiliate fee event", "error", err)
			}
			// if mayaname preferred asset is empty or cacao, release the affiliate collector
			if share.mayaname.Name != "" && (share.mayaname.PreferredAsset.IsEmpty() || share.mayaname.PreferredAsset.IsNativeBase()) {
				ctx.Logger().Info(fmt.Sprintf("Releasing affiliate collector on swap for %s", share.mayaname.Name))
				if err = releaseAffiliateCollector(ctx, mgr, share.mayaname); err != nil {
					ctx.Logger().Error(fmt.Sprintf("failed to release affiliate collector funds for %s. Error: %s", share.mayaname.Name, err))
				}
			}
		}
	}

	return totalDistributed, nil
}

func calculateAffiliateSharesV118(ctx cosmos.Context, mgr Manager, inputAmount cosmos.Uint, affiliates []string, affiliatesBps []cosmos.Uint, maxTotalAffBps cosmos.Uint) []affiliateFeeShare {
	// construct a virtual mayaname so we can use the calculateNestedAffiliateShares function for the root affiliates as well
	virtualSubAffiliates := make([]types.MAYANameSubaffiliate, len(affiliates))
	for i := range affiliates {
		virtualSubAffiliates[i].Name = affiliates[i]
		virtualSubAffiliates[i].Bps = affiliatesBps[i]
	}
	virtualMayaname := NewMAYAName("", 0, nil, common.EmptyAsset, nil, cosmos.ZeroUint(), virtualSubAffiliates)
	shares, _ := calculateNestedAffiliateShares(ctx, mgr, virtualMayaname, inputAmount, maxTotalAffBps)
	return shares
}

func calculateNestedAffiliateSharesV118(ctx cosmos.Context, mgr Manager, mayaname MAYAName, inputAmt, maxTotalAffBps cosmos.Uint) (shares []affiliateFeeShare, remainingAffBps cosmos.Uint) {
	affiliateFeeTickGranularity := uint64(mgr.Keeper().GetConfigInt64(ctx, constants.AffiliateFeeTickGranularity))
	var err error
	keeper := mgr.Keeper()
	maxBps := cosmos.NewUint(constants.MaxBasisPts) // 100%
	if mayaname.Name == "" {
		// for the root affiliates the total max allowed bps is maxTotalAffBps
		// if maxTotalAffBps is zero, use the default MaxAffiliateFeeBasisPoints = 2%
		if maxTotalAffBps.IsZero() {
			maxTotalAffBps = cosmos.NewUint(uint64(mgr.Keeper().GetConfigInt64(ctx, constants.MaxAffiliateFeeBasisPoints)))
		}
		remainingAffBps = maxTotalAffBps
	} else {
		// for sub-affiliates the total max allowed bps is MaxBasisPts (100%)
		remainingAffBps = maxBps
	}
	// get the direct subaffiliates of the MAYAName
	shares = make([]affiliateFeeShare, 0)
	for _, subAff := range mayaname.GetSubaffiliates() {
		var subAffMayaname MAYAName
		cacaoDest := common.NoAddress
		// if subaffiliate is a mayaname, fetch it
		if keeper.MAYANameExists(ctx, subAff.Name) {
			if subAffMayaname, err = keeper.GetMAYAName(ctx, subAff.Name); err != nil {
				ctx.Logger().Error(fmt.Sprintf("fail to get mayaname %s", subAff.Name))
				continue
			}
			// if preferred asset is set, leave the cacaDest empty (fee will be sent to affiliate collector)
			// else set it to cacao alias or owner
			if subAffMayaname.PreferredAsset.IsEmpty() {
				// fetch the cacao address of the mayaname
				cacaoDest = subAffMayaname.GetAlias(common.BASEChain)
				if cacaoDest.IsEmpty() {
					cacaoDest = common.Address(subAffMayaname.Owner.String())
					ctx.Logger().Info("affiliate MAYAName doesn't have native chain alias, owner will be used instead", "mayaname", subAffMayaname.Name, "owner", cacaoDest)
				}
			}
		} else {
			// if subaffiliate is not a mayaname, fetch the cacao address
			if cacaoDest, err = FetchAddress(ctx, keeper, subAff.Name, common.BASEChain); err != nil {
				// remove the subaffiliate from subaffiliate list if invalid or not exists
				ctx.Logger().Info(fmt.Sprintf("invalid subaffiliate %s registered for %s, removing it", subAff.Name, mayaname.Name))
				mayaname.RemoveSubaffiliate(subAff.Name)
				keeper.SetMAYAName(ctx, mayaname)
				continue
			}
		}
		// if the remaining bps (starting from 100%) is less than the current subaff bps means that
		/// the sum of subaffiliate shares exceed 100%, in this case ignore this and all subsequent subaffiliate
		if remainingAffBps.LT(subAff.Bps) {
			ctx.Logger().Error(fmt.Sprintf("sum of subaffiliate shares bps exceeds %s on subaffiliate %s", maxBps, subAff.Name))
			break
		}
		remainingAffBps = remainingAffBps.Sub(subAff.Bps)
		finalAffBps := subAff.Bps.MulUint64(affiliateFeeTickGranularity) // increase sub-affiliate bps calculation granularity for more precise distribution
		// process the subaffiliates if there are any
		if subAffMayaname.Name != "" && len(subAffMayaname.Subaffiliates) > 0 {
			// calculate the amount for this affiliate together with it's sub-affiliates
			subAmt := common.GetSafeShare(subAff.Bps, maxBps, inputAmt)
			// add subaffiliates' shares first
			subShares, remainingSubAffBps := calculateNestedAffiliateShares(ctx, mgr, subAffMayaname, subAmt, cosmos.ZeroUint())
			shares = append(shares, subShares...)
			// now add this affiliate as well with the remaining bps from subaffs (eg dog has subaff cat with subaffbps 40%, dog gets 60%)
			finalAffBps = remainingSubAffBps.Mul(subAff.Bps).QuoUint64(affiliateFeeTickGranularity)
		}
		subAmt := common.GetSafeShare(finalAffBps, maxBps.MulUint64(affiliateFeeTickGranularity), inputAmt)
		ctx.Logger().Debug("affiliate share calculated", "(sub)affiliate", subAff.Name, "bpsTicks", finalAffBps, "fee %", float64(finalAffBps.BigInt().Uint64())/float64(affiliateFeeTickGranularity/100), "input_amount", inputAmt, "aff_amount", subAmt)
		shares = append(shares, affiliateFeeShare{
			mayaname:  subAffMayaname,
			amount:    subAmt,
			cacaoDest: cacaoDest,
			// event fields
			parent:    mayaname.Name,
			subFeeBps: finalAffBps,
		})
	}

	return shares, remainingAffBps
}

func distributeShare(ctx cosmos.Context, mgr Manager, share affiliateFeeShare, mainTx common.Tx, signer cosmos.AccAddress, swapIndex *int, fromModule string) cosmos.Uint {
	ctx.Logger().Info("distributing affiliate fee", "txid", mainTx.ID.String(), "affiliate", share.mayaname.Name, "fee bps", share.subFeeBps.String(), "asset", mainTx.Coins[0].Asset, "amount", share.amount)
	var err error
	if mainTx.Coins[0].Asset.IsNativeBase() {
		err = sendShare(ctx, mgr, share, swapIndex, fromModule)
	} else {
		err = swapShare(ctx, mgr, share, mainTx, signer, swapIndex)
	}
	if err != nil {
		// just log the error, fee distribution to other affiliates will continue
		var to string
		if share.mayaname.Name != "" {
			to = share.mayaname.Name
		} else {
			to = share.cacaoDest.String()
		}
		ctx.Logger().Error("fail to distribute affiliate fee share", "to", to, "error", err)
		return cosmos.ZeroUint()
	}

	return share.amount
}

func sendShareV118(ctx cosmos.Context, mgr Manager, share affiliateFeeShare, swapIndex *int, fromModule string) error {
	coin := common.NewCoin(common.BaseNative, share.amount)
	if !share.cacaoDest.IsEmpty() {
		// either no mayaname or no preferred asset
		// send cacao to cacaoDest (mayaname alias/owner or explicit aff address)
		toAccAddress, err := share.cacaoDest.AccAddress()
		if err != nil {
			return fmt.Errorf("fail to convert address into AccAddress, address: %s, error: %w", share.cacaoDest, err)
		}
		ctx.Logger().Debug("sending affiliate fee to affiliate address", "amount", share.amount, "mayaname", share.mayaname.Name, "address", share.cacaoDest)
		sdkErr := mgr.Keeper().SendFromModuleToAccount(ctx, fromModule, toAccAddress, common.NewCoins(coin))
		if sdkErr != nil {
			return fmt.Errorf("fail to send native asset from %s module to affiliate, address: %s, error: %w", fromModule, share.cacaoDest, sdkErr)
		}
	} else {
		// preferred asset provided, send cacao to affiliate collector
		ctx.Logger().Info("sending affiliate fee to affiliate collector", "amount", share.amount, "mayaname", share.mayaname.Name)
		err := addToAffiliateCollector(ctx, mgr, share.amount, share.mayaname, swapIndex, fromModule)
		if err != nil {
			return fmt.Errorf("failed to send funds to affiliate collector, error: %w", err)
		}
	}
	return nil
}

func swapShareV118(ctx cosmos.Context, mgr Manager, share affiliateFeeShare, mainTx common.Tx, signer cosmos.AccAddress, swapIndex *int) error {
	// Copy mainTx coins so as not to modify the original
	mainTx.Coins = mainTx.Coins.Copy()
	mainTx.Coins[0].Amount = share.amount

	// memo will include this affiliate if this is a swap to affiliate collector
	memoStr := NewSimpleSwapMemo(ctx, mgr, common.BaseAsset(), share.cacaoDest, cosmos.ZeroUint(), share.mayaname.Name)
	mainTx.Memo = memoStr

	// Construct preferred asset swap tx
	affSwapMsg := NewMsgSwap(
		mainTx,
		common.BaseAsset(),
		share.cacaoDest, // mayaname alias/owner or explicit aff address or NoAddress if preferred address is set
		cosmos.ZeroUint(),
		common.NoAddress,
		cosmos.ZeroUint(),
		"", "", nil,
		MarketOrder,
		0, 0,
		signer,
	)

	// check if swap will succeed, if not, skip
	willSucceed, failReason := willSwapOutputExceedLimitAndFees(ctx, mgr, *affSwapMsg)
	if !willSucceed {
		destination := share.mayaname.Name
		if destination == "" {
			destination = share.cacaoDest.String()
		}
		return fmt.Errorf("affiliate fee swap for affiliate '%s' will fail: %s", destination, failReason)
	}

	// if cacaoDest is empty that means preferred asset is set
	if share.cacaoDest.IsEmpty() {
		// Set AffiliateCollector Module as destination (toAddress) and populate the AffiliateAddress
		// so that the swap handler can increment the emitted CACAO for the affiliate in the AffiliateCollector
		affiliateColllector, err := mgr.Keeper().GetModuleAddress(AffiliateCollectorName)
		if err != nil {
			return err
		}
		affSwapMsg.AffiliateAddress = common.Address(share.mayaname.Owner.String())
		affSwapMsg.Destination = affiliateColllector // affiliate collector module address
	}
	// swap the affiliate fee
	if err := mgr.Keeper().SetSwapQueueItem(ctx, *affSwapMsg, *swapIndex); err != nil {
		return fmt.Errorf("fail to add swap to queue, error: %w", err)
	}
	ctx.Logger().Debug("affiliate fee swap queued", "index", *swapIndex, "amount", share.amount, "mayaname", share.mayaname.Name, "address", affSwapMsg.Destination)
	*swapIndex++

	return nil
}

func updateAffiliateCollector(ctx cosmos.Context, mgr Manager, amount cosmos.Uint, mayaname MAYAName, swapIndex *int) error {
	affCol, err := mgr.Keeper().GetAffiliateCollector(ctx, mayaname.Owner)
	if err != nil {
		return fmt.Errorf("failed to get affiliate collector, MAYAName %s, error: %w", mayaname.Name, err)
	}

	affCol.CacaoAmount = affCol.CacaoAmount.Add(amount)
	mgr.Keeper().SetAffiliateCollector(ctx, affCol)
	return checkAndTriggerPreferredAssetSwap(ctx, mgr, mayaname, swapIndex)
}

func addToAffiliateCollector(ctx cosmos.Context, mgr Manager, amount cosmos.Uint, mayaname MAYAName, swapIndex *int, fromModule string) error {
	// send funds to affiliate collector module
	coin := common.NewCoin(common.BaseNative, amount)
	if err := mgr.Keeper().SendFromModuleToModule(ctx, fromModule, AffiliateCollectorName, common.NewCoins(coin)); err != nil {
		return fmt.Errorf("failed to send funds to affiliate collector, error: %w", err)
	}
	// update the affiliate collector's amount
	return updateAffiliateCollector(ctx, mgr, amount, mayaname, swapIndex)
}

func checkAndTriggerPreferredAssetSwap(ctx cosmos.Context, mgr Manager, mayaname MAYAName, swapIndex *int) error {
	affCol, err := mgr.Keeper().GetAffiliateCollector(ctx, mayaname.Owner)
	if err != nil {
		ctx.Logger().Error("failed to get affiliate collector", "MAYAName", mayaname.Name, "error", err)
		return err
	}
	// Trigger a preferred asset swap if the accrued CACAO exceeds the threshold:
	// Threshold = PreferredAssetOutboundFeeMultiplier (default = 100) x current outbound fee of the preferred asset chain.
	// If the affiliate collector's CACAO amount exceeds the threshold, initiate the preferred asset swap.
	threshold := getPreferredAssetSwapThreshold(ctx, mgr, mayaname.PreferredAsset)
	if affCol.CacaoAmount.GT(threshold) {
		ctx.Logger().Info("preferred asset swap triggered", "mayaname", mayaname.Name, "prefAsset", mayaname.PreferredAsset, "threshold", threshold, "affcol amount", affCol.CacaoAmount)
		if err = triggerPreferredAssetSwap(ctx, mgr, mayaname, affCol, *swapIndex); err != nil {
			ctx.Logger().Error("fail to swap to preferred asset", "mayaname", mayaname.Name, "err", err)
		} else {
			*swapIndex++
		}
	} else {
		ctx.Logger().Info("preferred asset swap not yet triggered", "mayaname", mayaname.Name, "prefAsset", mayaname.PreferredAsset, "threshold", threshold, "affcol amount", affCol.CacaoAmount)
	}
	return nil
}

func triggerPreferredAssetSwapV118(ctx cosmos.Context, mgr Manager, mn MAYAName, affCol AffiliateFeeCollector, queueIndex int) error {
	// Sanity check: don't swap 0 amount
	if affCol.CacaoAmount.IsZero() {
		return fmt.Errorf("can't execute preferred asset swap, accrued RUNE amount is zero")
	}
	// Sanity check: ensure the swap amount isn't more than the entire AffiliateCollector module
	acBalance := mgr.Keeper().GetRuneBalanceOfModule(ctx, AffiliateCollectorName)
	if affCol.CacaoAmount.GT(acBalance) {
		return fmt.Errorf("cacao amount greater than module balance: (%s/%s)", affCol.CacaoAmount.String(), acBalance.String())
	}

	// if the preferred asset is empty or cacao, just release the affiliate collector
	if mn.PreferredAsset.IsEmpty() || mn.PreferredAsset.IsNativeBase() {
		if err := releaseAffiliateCollector(ctx, mgr, mn); err != nil {
			return fmt.Errorf("failed to release affiliate collector funds for %s. Error: %w", mn.Name, err)
		}
		return nil
	}

	// Check that the MAYAName has an address alias for the PreferredAsset
	alias := mn.GetAlias(mn.PreferredAsset.GetChain())
	if alias.Equals(common.NoAddress) {
		return fmt.Errorf("no alias for preferred asset, skip preferred asset swap: %s", mn.Name)
	}
	pool, err := mgr.Keeper().GetPool(ctx, mn.PreferredAsset)
	if err != nil {
		return err
	}
	if pool.Status != PoolAvailable {
		return fmt.Errorf("preferred asset (%s) pool is not available", mn.PreferredAsset)
	}

	affCacao := affCol.CacaoAmount
	affCoin := common.NewCoin(common.BaseAsset(), affCacao)

	networkMemo := fmt.Sprintf("%s-%s", PreferredAssetSwapMemoPrefix, mn.Name)
	var asgardAddress common.Address
	asgardAddress, err = mgr.Keeper().GetModuleAddress(AsgardName)
	if err != nil {
		ctx.Logger().Error("failed to retrieve asgard address", "error", err)
		return err
	}
	affColAddress, err := mgr.Keeper().GetModuleAddress(AffiliateCollectorName)
	if err != nil {
		ctx.Logger().Error("failed to retrieve affiliate collector module address", "error", err)
		return err
	}

	// Generate a unique ID for the preferred asset swap, which is a hash of the MAYAName,
	// affCoin, and BlockHeight This is to prevent the network thinking it's an outbound
	// of the swap that triggered it
	str := fmt.Sprintf("%s|%s|%d", mn.GetName(), affCoin.String(), ctx.BlockHeight())
	hash := fmt.Sprintf("%X", sha256.Sum256([]byte(str)))

	paTxID, err := common.NewTxID(hash)
	if err != nil {
		return err
	}

	existingVoter, err := mgr.Keeper().GetObservedTxInVoter(ctx, paTxID)
	if err != nil {
		return fmt.Errorf("fail to get existing voter: %w", err)
	}
	if len(existingVoter.Txs) > 0 {
		return fmt.Errorf("preferred asset tx: %s already exists", str)
	}

	// Construct preferred asset swap tx
	tx := common.NewTx(
		paTxID,
		affColAddress,
		asgardAddress,
		common.NewCoins(affCoin),
		common.Gas{},
		networkMemo,
	)

	preferredAssetSwap := NewMsgSwap(
		tx,
		mn.PreferredAsset,
		alias,
		cosmos.ZeroUint(),
		common.NoAddress,
		cosmos.ZeroUint(),
		"", "", nil,
		MarketOrder,
		0, 0,
		mn.Owner,
	)

	// Construct preferred asset swap inbound tx voter
	txIn := ObservedTx{Tx: tx}
	txInVoter := NewObservedTxVoter(txIn.Tx.ID, []ObservedTx{txIn})
	txInVoter.Height = ctx.BlockHeight()
	txInVoter.FinalisedHeight = ctx.BlockHeight()
	txInVoter.Tx = txIn
	mgr.Keeper().SetObservedTxInVoter(ctx, txInVoter)

	// Queue the preferred asset swap
	if err = mgr.Keeper().SetSwapQueueItem(ctx, *preferredAssetSwap, queueIndex); err != nil {
		ctx.Logger().Error("fail to add preferred asset swap to queue", "error", err)
		return err
	}
	ctx.Logger().Debug("preferred asset swap has been queued", "MAYAname", mn.Name, "amt", affCacao.String(), "dest", alias)

	// Send CACAO from AffiliateCollector to Asgard and update AffiliateCollector
	if err = mgr.Keeper().SendFromModuleToModule(ctx, AffiliateCollectorName, AsgardName, common.NewCoins(affCoin)); err != nil {
		return fmt.Errorf("failed to send rune to asgard: %w", err)
	}

	affCol.CacaoAmount = cosmos.ZeroUint()
	mgr.Keeper().SetAffiliateCollector(ctx, affCol)

	return nil
}

func releaseAffiliateCollector(ctx cosmos.Context, mgr Manager, mayaname MAYAName) error {
	var err error
	var destAcc cosmos.AccAddress
	affCol, err := mgr.Keeper().GetAffiliateCollector(ctx, mayaname.Owner)
	if err != nil {
		return fmt.Errorf("fail to get affiliate collector: %w", err)
	}
	if affCol.CacaoAmount.IsZero() {
		return nil
	}
	coins := common.NewCoins(common.NewCoin(common.BaseNative, affCol.CacaoAmount))
	destAddr := mayaname.GetAlias(common.BASEChain)
	if destAddr.IsEmpty() {
		destAcc = mayaname.Owner
	} else {
		destAcc, err = destAddr.AccAddress()
		if err != nil {
			return fmt.Errorf("fail to convert address into AccAddress, address: %s, error: %w", destAddr, err)
		}
	}
	sdkErr := mgr.Keeper().SendFromModuleToAccount(ctx, AffiliateCollectorName, destAcc, coins)
	if sdkErr != nil {
		return fmt.Errorf("fail to send native asset to affiliate, address: %s, error: %w", destAcc, sdkErr)
	}
	affCol.CacaoAmount = cosmos.ZeroUint()
	mgr.Keeper().SetAffiliateCollector(ctx, affCol)

	return nil
}
