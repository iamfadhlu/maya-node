package mayachain

import (
	"crypto/sha256"
	"fmt"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

func skimAffiliateFeesV1(ctx cosmos.Context, mgr Manager, mainTx common.Tx, signer cosmos.AccAddress, memoStr string) (cosmos.Uint, error) {
	// Parse memo
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
	shares := calculateAffiliateShares(ctx, mgr, totalSwapAmount, affiliates, affiliatesBps, cosmos.ZeroUint())
	for _, share := range shares {
		if !share.amount.IsZero() {
			distributed := distributeShare(ctx, mgr, share, mainTx, signer, &swapIndex, AsgardName)
			totalDistributed = totalDistributed.Add(distributed)
		}
	}
	return totalDistributed, nil
}

func calculateAffiliateSharesV1(ctx cosmos.Context, mgr Manager, inputAmount cosmos.Uint, affiliates []string, affiliatesBps []cosmos.Uint) []affiliateFeeShare {
	// construct a virtual mayaname so we can use the calculateNestedAffiliateShares function for the root affiliates as well
	virtualSubAffiliates := make([]types.MAYANameSubaffiliate, len(affiliates))
	for i := range affiliates {
		virtualSubAffiliates[i].Name = affiliates[i]
		virtualSubAffiliates[i].Bps = affiliatesBps[i]
	}
	virtualMayaname := NewMAYAName("", 0, nil, common.EmptyAsset, nil, cosmos.ZeroUint(), virtualSubAffiliates)
	nestedAffShares, _ := calculateNestedAffiliateShares(ctx, mgr, virtualMayaname, inputAmount, cosmos.ZeroUint())
	return nestedAffShares
}

func calculateNestedAffiliateSharesV1(ctx cosmos.Context, mgr Manager, mayaname MAYAName, inputAmt cosmos.Uint) ([]affiliateFeeShare, cosmos.Uint) {
	keeper := mgr.Keeper()
	// get the direct subaffiliates of the MAYAName
	remainingAmt := inputAmt
	shares := make([]affiliateFeeShare, 0)
	for _, subAff := range mayaname.GetSubaffiliates() {
		subaffIsMayaname := keeper.MAYANameExists(ctx, subAff.Name)
		subaffExplicitAddress := common.NoAddress
		if !subaffIsMayaname {
			var err error
			if subaffExplicitAddress, err = FetchAddress(ctx, keeper, subAff.Name, common.BASEChain); err != nil {
				// remove the subaffiliate from subaffiliate list if invalid or not exists
				ctx.Logger().Info(fmt.Sprintf("invalid subaffiliate %s registered for %s, removing it", subAff.Name, mayaname.Name))
				mayaname.RemoveSubaffiliate(subAff.Name)
				keeper.SetMAYAName(ctx, mayaname)
				continue
			}
		}
		subAmt := common.GetSafeShare(subAff.Bps, cosmos.NewUint(constants.MaxBasisPts), inputAmt)
		ctx.Logger().Debug("affiliate share calculated", "(sub)affiliate", subAff.Name, "bps", subAff.Bps, "amount", subAmt)
		// if total subaffiliate shares exceed 100%, ignore them
		if subAmt.GT(remainingAmt) {
			break
		}
		remainingAmt = remainingAmt.Sub(subAmt)
		if subaffIsMayaname {
			subAffMn, err := keeper.GetMAYAName(ctx, subAff.Name)
			if err != nil {
				ctx.Logger().Error(fmt.Sprintf("fail to get sub-affiliate MAYAName %s for MAYAName %s", subAff.Name, mayaname.Name))
				continue
			}
			nestedAffShares, _ := calculateNestedAffiliateShares(ctx, mgr, subAffMn, subAmt, cosmos.ZeroUint())
			shares = append(shares, nestedAffShares...)
		} else {
			shares = append(shares, affiliateFeeShare{
				mayaname:  types.MAYAName{},
				amount:    subAmt,
				cacaoDest: subaffExplicitAddress,
			})
		}
	}
	if mayaname.Name != "" { // if empty, it is the virtual mayaname
		cacaoDest := common.NoAddress
		if mayaname.PreferredAsset.IsEmpty() {
			cacaoDest = mayaname.GetAlias(common.BASEChain)
			if cacaoDest.IsEmpty() {
				cacaoDest = common.Address(mayaname.Owner.String())
				ctx.Logger().Info("affiliate MAYAName doesn't have native chain alias, owner will be used instead", "mayaname", mayaname.Name, "owner", cacaoDest)
			}
		}
		shares = append(shares, affiliateFeeShare{
			mayaname:  mayaname,
			amount:    remainingAmt,
			cacaoDest: cacaoDest,
		})
		// only for debug log
		address := cacaoDest.String()
		if cacaoDest.IsEmpty() {
			address = fmt.Sprintf("affCol/%s:%s", mayaname.PreferredAsset, mayaname.GetAlias(mayaname.PreferredAsset.Chain))
		}
		ctx.Logger().Debug("affiliate share added", "mayaname", mayaname.Name, "amount", remainingAmt, "address", address)
	}
	return shares, cosmos.ZeroUint()
}

func sendShareV1(ctx cosmos.Context, mgr Manager, share affiliateFeeShare, swapIndex *int) error {
	coin := common.NewCoin(common.BaseNative, share.amount)
	if !share.cacaoDest.IsEmpty() {
		// either no mayaname or no preferred asset
		// send cacao to cacaoDest (mayaname alias/owner or explicit aff address)
		toAccAddress, err := share.cacaoDest.AccAddress()
		if err != nil {
			return fmt.Errorf("fail to convert address into AccAddress, address: %s, error: %w", share.cacaoDest, err)
		}
		// only for debug log
		mayaname := "n/a"
		if share.mayaname.Name != "" {
			mayaname = share.mayaname.Name
		}
		ctx.Logger().Info("sending affiliate fee to affiliate address", "amount", share.amount, "mayaname", mayaname, "address", share.cacaoDest)
		sdkErr := mgr.Keeper().SendFromModuleToAccount(ctx, AsgardName, toAccAddress, common.NewCoins(coin))
		if sdkErr != nil {
			return fmt.Errorf("fail to send native asset to affiliate, address: %s, error: %w", share.cacaoDest, sdkErr)
		}
	} else {
		// preferred asset provided
		// send cacao to affiliate collector
		ctx.Logger().Info("sending affiliate fee to affiliate collector", "amount", share.amount, "mayaname", share.mayaname.Name)
		err := addToAffiliateCollector(ctx, mgr, share.amount, share.mayaname, swapIndex, AsgardName)
		if err != nil {
			return fmt.Errorf("failed to send funds to affiliate collector, error: %w", err)
		}
	}
	return nil
}

func swapShareV1(ctx cosmos.Context, mgr Manager, share affiliateFeeShare, mainTx common.Tx, signer cosmos.AccAddress, swapIndex *int) error {
	// Copy mainTx coins so as not to modify the original
	mainTx.Coins = mainTx.Coins.Copy()

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

	affSwapMsg.Tx.Coins[0].Amount = share.amount

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

		// trigger preferred asset swap if needed
		err = checkAndTriggerPreferredAssetSwap(ctx, mgr, share.mayaname, swapIndex)
		if err != nil {
			ctx.Logger().Error("failed to check and trigger preferred asset swap", "mayaname", share.mayaname.Name)
		}
	}

	// only for debug log
	mayaname := "n/a"
	if share.mayaname.Name != "" {
		mayaname = share.mayaname.Name
	}
	ctx.Logger().Debug("affiliate fee swap queue", "index", *swapIndex, "amount", share.amount, "mayaname", mayaname, "address", affSwapMsg.Destination)
	// swap the affiliate fee
	if err := mgr.Keeper().SetSwapQueueItem(ctx, *affSwapMsg, *swapIndex); err != nil {
		return fmt.Errorf("fail to add swap to queue, error: %w", err)
	}
	*swapIndex++

	return nil
}

func triggerPreferredAssetSwapV112(ctx cosmos.Context, mgr Manager, mn MAYAName, affCol AffiliateFeeCollector, queueIndex int) error {
	// Check that the MAYAName has an address alias for the PreferredAsset
	alias := mn.GetAlias(mn.PreferredAsset.GetChain())
	if alias.Equals(common.NoAddress) {
		return fmt.Errorf("no alias for preferred asset, skip preferred asset swap: %s", mn.Name)
	}

	// Sanity check: don't swap 0 amount
	if affCol.CacaoAmount.IsZero() {
		return fmt.Errorf("can't execute preferred asset swap, accrued RUNE amount is zero")
	}
	// Sanity check: ensure the swap amount isn't more than the entire AffiliateCollector module
	acBalance := mgr.Keeper().GetRuneBalanceOfModule(ctx, AffiliateCollectorName)
	if affCol.CacaoAmount.GT(acBalance) {
		return fmt.Errorf("cacao amount greater than module balance: (%s/%s)", affCol.CacaoAmount.String(), acBalance.String())
	}

	affCacao := affCol.CacaoAmount
	affCoin := common.NewCoin(common.BaseAsset(), affCacao)

	networkMemo := "MAYA-PREFERRED-ASSET-" + mn.Name
	asgardAddress, err := mgr.Keeper().GetModuleAddress(AsgardName)
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

	ctx.Logger().Info("preferred asset swap hash", "hash", hash)

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
	// if err = mgr.Keeper().SendFromModuleToModule(ctx, AffiliateCollectorName, AsgardName, common.NewCoins(affCoin)); err != nil {
	// 	return fmt.Errorf("failed to send rune to asgard: %w", err)
	// }

	// affCol.CacaoAmount = cosmos.ZeroUint()
	// mgr.Keeper().SetAffiliateCollector(ctx, affCol)

	return nil
}
