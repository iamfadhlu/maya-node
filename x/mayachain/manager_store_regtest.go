//go:build regtest
// +build regtest

package mayachain

import (
	"fmt"
	"strconv"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

func migrateStoreV86(ctx cosmos.Context, mgr *Mgrs)    {}
func migrateStoreV88(ctx cosmos.Context, mgr Manager)  {}
func migrateStoreV90(ctx cosmos.Context, mgr Manager)  {}
func migrateStoreV96(ctx cosmos.Context, mgr Manager)  {}
func migrateStoreV102(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV104(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV105(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV106(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV107(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV108(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV109(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV110(ctx cosmos.Context, mgr Manager) {}

func migrateStoreV111(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v111", "error", err)
		}
	}()

	// For any in-progress streaming swaps to non-RUNE Native coins,
	// mint the current Out amount to the Pool Module.
	var coinsToMint common.Coins

	iterator := mgr.Keeper().GetSwapQueueIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var msg MsgSwap
		if err := mgr.Keeper().Cdc().Unmarshal(iterator.Value(), &msg); err != nil {
			ctx.Logger().Error("fail to fetch swap msg from queue", "error", err)
			continue
		}

		if !msg.IsStreaming() || !msg.TargetAsset.IsNative() || msg.TargetAsset.IsBase() {
			continue
		}

		swp, err := mgr.Keeper().GetStreamingSwap(ctx, msg.Tx.ID)
		if err != nil {
			ctx.Logger().Error("fail to fetch streaming swap", "error", err)
			continue
		}

		if !swp.Out.IsZero() {
			mintCoin := common.NewCoin(msg.TargetAsset, swp.Out)
			coinsToMint = coinsToMint.Add(mintCoin)
		}
	}

	// The minted coins are for in-progress swaps, so keeping the "swap" in the event field and logs.
	var coinsToTransfer common.Coins
	for _, mintCoin := range coinsToMint {
		if err := mgr.Keeper().MintToModule(ctx, ModuleName, mintCoin); err != nil {
			ctx.Logger().Error("fail to mint coins during swap", "error", err)
		} else {
			// MintBurn event is not currently implemented, will ignore

			// mintEvt := NewEventMintBurn(MintSupplyType, mintCoin.Asset.Native(), mintCoin.Amount, "swap")
			// if err := mgr.EventMgr().EmitEvent(ctx, mintEvt); err != nil {
			// 	ctx.Logger().Error("fail to emit mint event", "error", err)
			// }
			coinsToTransfer = coinsToTransfer.Add(mintCoin)
		}
	}

	if err := mgr.Keeper().SendFromModuleToModule(ctx, ModuleName, AsgardName, coinsToTransfer); err != nil {
		ctx.Logger().Error("fail to move coins during swap", "error", err)
	}
}

func migrateStoreV112(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v112", "error", err)
		}
	}()

	// Mock ObservedTxVoter state from
	// https://mayanode.mayachain.info/mayachain/tx/details/6F4F5801E5BEA96BC521BB00A7542EACB5FBDC161FD43431FEF3245D93F9C0AB
	tx6F4F5801E5BEA96BC521BB00A7542EACB5FBDC161FD43431FEF3245D93F9C0AB := types.ObservedTxVoter{
		TxID: common.TxID("6F4F5801E5BEA96BC521BB00A7542EACB5FBDC161FD43431FEF3245D93F9C0AB"),
		Tx: types.ObservedTx{
			Tx: common.Tx{
				ID:          "6F4F5801E5BEA96BC521BB00A7542EACB5FBDC161FD43431FEF3245D93F9C0AB",
				Chain:       common.THORChain,
				FromAddress: common.Address("thor1kfqzcr8m73qd97z46wha2w8u6f22tj9dxyquj6"),
				Coins: common.Coins{
					{
						Asset:    common.RUNEAsset,
						Amount:   cosmos.NewUint(2000000000000),
						Decimals: 8,
					},
				},
				Gas: common.Gas{
					{
						Asset:  common.RUNEAsset,
						Amount: cosmos.NewUint(2000000),
					},
				},
				Memo: "=:ETH.USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48:0xAa287489e76B11B56dBa7ca03e155369400f3d65:9749038937200/3/0:ts:50",
			},
			ObservedPubKey: "tmayapub1addwnpepqfshsq2y6ejy2ysxmq4gj8n8mzuzyulk9wh4n946jv5w2vpwdn2yuz3v0gx",
			Signers:        []string{""},
			OutHashes:      []string{"0000000000000000000000000000000000000000000000000000000000000000"},
			Status:         types.Status_done,
		},
		Txs: types.ObservedTxs{
			{
				Tx: common.Tx{
					ID:          "6F4F5801E5BEA96BC521BB00A7542EACB5FBDC161FD43431FEF3245D93F9C0AB",
					Chain:       common.THORChain,
					FromAddress: common.Address("thor1kfqzcr8m73qd97z46wha2w8u6f22tj9dxyquj6"),
					Coins: common.Coins{
						{
							Asset:    common.RUNEAsset,
							Amount:   cosmos.NewUint(2000000000000),
							Decimals: 8,
						},
					},
					Gas: common.Gas{
						{
							Asset:  common.RUNEAsset,
							Amount: cosmos.NewUint(2000000),
						},
					},
					Memo: "=:ETH.USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48:0xAa287489e76B11B56dBa7ca03e155369400f3d65:9749038937200/3/0:ts:50",
				},
				ObservedPubKey: "tmayapub1addwnpepqfshsq2y6ejy2ysxmq4gj8n8mzuzyulk9wh4n946jv5w2vpwdn2yuz3v0gx",
				Signers:        []string{""},
				OutHashes:      []string{"0000000000000000000000000000000000000000000000000000000000000000"},
				Status:         types.Status_done,
			},
		},
		Actions: []types.TxOutItem{
			{
				Chain:     common.BASEChain,
				ToAddress: common.Address("maya1rm0xppz0ypgr3zqymnrdtnnjs4kpxqgy8tfmuk"),
				Coin: common.Coin{
					Asset:  common.BaseAsset(),
					Amount: cosmos.NewUint(8888139344845),
				},
				Memo: "OUT:6F4F5801E5BEA96BC521BB00A7542EACB5FBDC161FD43431FEF3245D93F9C0AB",
				MaxGas: common.Gas{
					{
						Asset:    common.BaseAsset(),
						Amount:   cosmos.ZeroUint(),
						Decimals: 8,
					},
				},
				GasRate: 2000000000,
				InHash:  common.TxID("6F4F5801E5BEA96BC521BB00A7542EACB5FBDC161FD43431FEF3245D93F9C0AB"),
			},
		},
		OutTxs: common.Txs{
			{
				ID:        common.TxID("0000000000000000000000000000000000000000000000000000000000000000"),
				Chain:     common.BASEChain,
				ToAddress: common.Address("maya1rm0xppz0ypgr3zqymnrdtnnjs4kpxqgy8tfmuk"),
				Coins: common.Coins{
					{
						Asset:  common.BaseAsset(),
						Amount: cosmos.NewUint(8888139344845),
					},
				},
				Gas: common.Gas{
					{
						Asset:  common.BaseAsset(),
						Amount: cosmos.NewUint(2000000000),
					},
				},
				Memo: "OUT:6F4F5801E5BEA96BC521BB00A7542EACB5FBDC161FD43431FEF3245D93F9C0AB",
			},
		},
		FinalisedHeight: 1,
		UpdatedVault:    true,
	}
	mgr.Keeper().SetObservedTxInVoter(ctx, tx6F4F5801E5BEA96BC521BB00A7542EACB5FBDC161FD43431FEF3245D93F9C0AB)

	// https://mayanode.mayachain.info/mayachain/tx/details/80559CC3CCF2665531AAA7DD6B59F986721C6B76F1DD056DAE58DCC4878C5D56
	// Tx doesn't have planned "actions" nor "out_txs", send it with TryAddTxOutItem()
	var err error
	originalTxID := "80559CC3CCF2665531AAA7DD6B59F986721C6B76F1DD056DAE58DCC4878C5D56"
	maxGas, err := mgr.gasMgr.GetMaxGas(ctx, common.THORChain)
	if err != nil {
		ctx.Logger().Error("unable to GetMaxGas while retrying issue 1", "err", err)
	} else {
		gasRate := mgr.gasMgr.GetGasRate(ctx, common.THORChain)
		droppedRescue := types.TxOutItem{
			Chain:       common.THORChain,
			ToAddress:   common.Address("thor1sucvdnzcf4j6ynep4n4skjpq8tvqv8ags3a4ky"),
			VaultPubKey: common.PubKey("tmayapub1addwnpepqfshsq2y6ejy2ysxmq4gj8n8mzuzyulk9wh4n946jv5w2vpwdn2yuz3v0gx"),
			Coin: common.NewCoin(
				common.RUNEAsset,
				cosmos.NewUint(uint64(199757582857)),
			),
			Memo:    fmt.Sprintf("OUT:%s", originalTxID),
			InHash:  common.TxID(originalTxID),
			GasRate: int64(gasRate.Uint64()),
			MaxGas:  common.Gas{maxGas},
		}

		ok, err := mgr.txOutStore.TryAddTxOutItem(ctx, mgr, droppedRescue, cosmos.ZeroUint())
		if err != nil {
			ctx.Logger().Error("fail to retry THOR rescue tx", "error", err)
		}
		if !ok {
			ctx.Logger().Error("TryAddTxOutItem didn't success for tx")
		}
	}

	// https://mayanode.mayachain.info/mayachain/tx/details/6F4F5801E5BEA96BC521BB00A7542EACB5FBDC161FD43431FEF3245D93F9C0AB
	// Tx have planned "actions" but doesn't have "out_txs", send it with TryAddTxOutItem()
	originalTxID = "6F4F5801E5BEA96BC521BB00A7542EACB5FBDC161FD43431FEF3245D93F9C0AB"
	maxGas, err = mgr.gasMgr.GetMaxGas(ctx, common.ETHChain)
	if err != nil {
		ctx.Logger().Error("unable to GetMaxGas while retrying issue 1", "err", err)
	} else {
		gasRate := mgr.gasMgr.GetGasRate(ctx, common.ETHChain)
		droppedRescue := types.TxOutItem{
			Chain:       common.ETHChain,
			ToAddress:   common.Address("0xAa287489e76B11B56dBa7ca03e155369400f3d65"),
			VaultPubKey: common.PubKey("tmayapub1addwnpepqfshsq2y6ejy2ysxmq4gj8n8mzuzyulk9wh4n946jv5w2vpwdn2yuz3v0gx"), // regtests pubkey
			Coin: common.NewCoin(
				common.USDCAsset,
				// Check out value for https://mayanode.mayachain.info/mayachain/block?height=8292990
				cosmos.NewUint(uint64(9861109874700)),
			),
			Memo:    fmt.Sprintf("OUT:%s", originalTxID),
			InHash:  common.TxID(originalTxID),
			GasRate: int64(gasRate.Uint64()),
			MaxGas:  common.Gas{maxGas},
		}

		ok, err := mgr.txOutStore.TryAddTxOutItem(ctx, mgr, droppedRescue, cosmos.ZeroUint())
		if err != nil {
			ctx.Logger().Error("fail to retry THOR rescue tx", "error", err)
		}
		if !ok {
			ctx.Logger().Error("TryAddTxOutItem didn't success for tx")
		}
	}
}

func migrateStoreV113(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v113", "error", err)
		}
	}()

	txIds := common.TxIDs{}

	// Observed, but not paid txs
	observedTxIds := common.TxIDs{
		"431A6446957894DF77FA4220844990898F96BF46E1C8752910386A299AF72455",
	}

	for _, observedTxID := range observedTxIds {
		voter, err := mgr.K.GetObservedTxInVoter(ctx, observedTxID)
		if err != nil {
			ctx.Logger().Error("fail to get observed tx in voter", "error", err)
			continue
		}

		if len(voter.OutTxs) == 0 {
			continue
		}

		outboundTxID := voter.OutTxs[0].ID
		outVoter, err := mgr.K.GetObservedTxOutVoter(ctx, outboundTxID)
		if err != nil {
			ctx.Logger().Error("fail to get observed tx out voter", "error", err)
			continue
		}

		outVoter.SetReverted()
		voter.OutTxs = nil

		activeAsgards, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
		if err != nil || len(activeAsgards) == 0 {
			ctx.Logger().Error("fail to get active asgard vaults", "error", err)
			return
		}

		// we actually know there's only one active asgard
		if len(voter.Actions) > 0 {
			coin := voter.Actions[0].Coin
			gas := voter.Actions[0].MaxGas.ToCoins().GetCoin(coin.Asset).Amount
			coin.Amount = coin.Amount.Add(gas)
			activeAsgards[0].AddFunds(common.Coins{coin})

			// add the amount back to the vault
			if err := mgr.Keeper().SetVault(ctx, activeAsgards[0]); err != nil {
				ctx.Logger().Error("fail to save asgard vault", "error", err, "hash", observedTxID)
			}
			mgr.K.SetObservedTxOutVoter(ctx, outVoter)
			mgr.K.SetObservedTxInVoter(ctx, voter)
			ctx.Logger().Info("vault added funds back", "vault", activeAsgards[0].PubKey, "amount", coin.Amount)

			txIds = append(txIds, observedTxID)
		}
	}

	requeueDanglingActions(ctx, mgr, txIds)
}

func migrateStoreV114(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v114", "error", err)
		}
	}()

	activeAsgards, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil || len(activeAsgards) == 0 {
		ctx.Logger().Error("fail to get active asgard vaults", "error", err)
		return
	}

	// https://mayanode.mayachain.info/mayachain/tx/details/83AEC95CE5BC2B4AE8835B23DE57ACDF14CC3B30B00095ADB9D7278840CABD2D
	coin := common.NewCoin(common.DASHAsset, cosmos.NewUint(616040018514))
	activeAsgards[0].AddFunds(common.Coins{coin})
	if err := mgr.Keeper().SetVault(ctx, activeAsgards[0]); err != nil {
		ctx.Logger().Error("fail to save asgard vault", "error", err)
	}
}

func migrateStoreV115(ctx cosmos.Context, mgr *Mgrs) {}
func migrateStoreV116(ctx cosmos.Context, mgr *Mgrs) {}

func migrateStoreV117(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v116", "error", err)
		}
	}()

	txID := common.TxID("FD4CA0CEEE107E4A077BB178BC0A031EEB5BE6E55B6F529EE65C5A1A2487A621")

	newDestinationAddrString := "0xEf1C6F153afaf86424fd984728d32535902F1c3D"
	newDestinationAddr, err := common.NewAddress(newDestinationAddrString, mgr.GetVersion())
	if err != nil {
		ctx.Logger().Error("fail to parse address", "error", err)
		return
	}

	newMemo := fmt.Sprintf("=:ETH.ETH:%s:0/1/10:wr:20", newDestinationAddrString)
	iterator := mgr.Keeper().GetSwapQueueIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var msg MsgSwap
		if err := mgr.Keeper().Cdc().Unmarshal(iterator.Value(), &msg); err != nil {
			ctx.Logger().Error("fail to fetch swap msg from queue", "error", err)
			continue
		}

		if msg.IsStreaming() && msg.Tx.ID.Equals(txID) {
			ss := strings.Split(string(iterator.Key()), "-")
			i, err := strconv.Atoi(ss[len(ss)-1])
			if err != nil {
				ctx.Logger().Error("fail to parse swap queue msg index", "key", iterator.Key(), "error", err)
				continue
			}

			if i != 0 {
				mgr.Keeper().RemoveSwapQueueItem(ctx, msg.Tx.ID, i)
				ctx.Logger().Info("Swap Queue Item Removed", "index", i)
			} else {
				oldMsg := msg
				msg.Destination = newDestinationAddr
				msg.Tx.Memo = newMemo
				if err := mgr.Keeper().SetSwapQueueItem(ctx, msg, 0); err != nil {
					ctx.Logger().Error("fail to save swap msg to queue", "error", err)
				}
				ctx.Logger().Info("Swap Queue Item Changed", "old msg", oldMsg, "new msg", msg)
			}
		}
	}
}

func migrateStoreV118(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v118", "error", err)
		}
	}()

	fundCacaoPoolV118(ctx, mgr)
}

func fundCacaoPoolV118(ctx cosmos.Context, mgr *Mgrs) {
	if err := consolidateStagenetFundsV118(ctx, mgr); err != nil {
		ctx.Logger().Error("fail to consolidate stagenet funds", "error", err)
	}

	threeMillionCacao := cosmos.NewUint(uint64(3_000_000_0000000000))
	coinsToCacaoPool := common.NewCoin(common.BaseNative, threeMillionCacao)
	if err := mgr.Keeper().SendFromModuleToModule(ctx, ReserveName, CACAOPoolName, common.NewCoins(coinsToCacaoPool)); err != nil {
		ctx.Logger().Error("fail to move coins from Reserve to CACAOPool", "error", err)
	} else {
		cacaoPool, err := mgr.Keeper().GetCACAOPool(ctx)
		if err != nil {
			ctx.Logger().Error("fail to get cacao pool", "error", err)
		} else {
			cacaoPool.ReserveUnits = threeMillionCacao
			mgr.Keeper().SetCACAOPool(ctx, cacaoPool)
		}
	}
}

func consolidateStagenetFundsV118(ctx cosmos.Context, mgr *Mgrs) error {
	asgard := mgr.Keeper().GetModuleAccAddress(AsgardName)
	reserve := mgr.Keeper().GetModuleAccAddress(ReserveName)
	leftover, err := cosmos.AccAddressFromBech32("tmaya13wrmhnh2qe98rjse30pl7u6jxszjjwl4fd6gwn")
	if err != nil {
		return fmt.Errorf("fail to parse leftover address")
	}

	type wallet struct {
		acc  cosmos.AccAddress
		coin cosmos.Coin
	}

	leftoverAmount := cosmos.ZeroUint()
	wallets := make([]wallet, 0)

	mgr.coinKeeper.IterateAllBalances(ctx, func(addr sdk.AccAddress, coin sdk.Coin) bool {
		if coin.Denom == common.BaseAsset().Native() && !coin.Amount.IsZero() {
			if addr.Equals(leftover) {
				leftoverAmount = cosmos.NewUintFromBigInt(coin.Amount.BigInt())
			}

			wallets = append(wallets, wallet{
				acc:  addr,
				coin: coin,
			})
		}

		return false
	})

	// move everything to leftover except asgard, reserve
	for _, wallet := range wallets {
		amount := wallet.coin
		if !amount.IsZero() {
			if wallet.acc.Equals(asgard) || wallet.acc.Equals(reserve) || wallet.acc.Equals(leftover) {
				continue
			}
			ctx.Logger().Info("Sending cacao", "from", wallet.acc, "to", leftover, "amount", amount)
			if err = mgr.Keeper().SendCoins(ctx, wallet.acc, leftover, cosmos.NewCoins(wallet.coin)); err != nil {
				return fmt.Errorf("fail to send coins: %w", err)
			}

			leftoverAmount = leftoverAmount.Add(cosmos.NewUintFromBigInt(wallet.coin.Amount.BigInt()))
		}
	}

	desiredSupply := cosmos.NewUint(100_000_000_0000000000)
	supplyCoin := mgr.coinKeeper.GetSupply(ctx, common.BaseAsset().Native())
	supply := cosmos.NewUintFromBigInt(supplyCoin.Amount.BigInt())
	// all coins sent to leftover, amounts are calculated
	if supply.GT(desiredSupply) {
		excess := supply.Sub(desiredSupply)

		if leftoverAmount.LT(excess) {
			ctx.Logger().Error("Unable to burn desired excess, module accounts have more than accounts", "excess", excess, "leftover", leftoverAmount)
			excess = leftoverAmount
			leftoverAmount = cosmos.ZeroUint()
		} else {
			leftoverAmount = leftoverAmount.Sub(excess)
		}

		ctx.Logger().Info("Sending cacao excess amount to mayachain module for burning", "from", leftover, "amount", excess)
		if err = mgr.Keeper().SendFromAccountToModule(ctx, leftover, ModuleName, common.NewCoins(common.NewCoin(common.BaseNative, excess))); err != nil {
			return fmt.Errorf("fail to send from leftover to mayachain module: %w", err)
		}
		if err = mgr.Keeper().BurnFromModule(ctx, ModuleName, common.NewCoin(common.BaseNative, excess)); err != nil {
			return fmt.Errorf("fail to burn excess coins: %w", err)
		}
	}
	// send 3M cacao to Reserve if there is less than 3M
	ctx.Logger().Info("Leftover", "amount", leftoverAmount)
	amountToReserve := cosmos.NewUint(uint64(3_000_000_0000000000))
	reserveBalance := mgr.Keeper().GetRuneBalanceOfModule(ctx, ReserveName)
	if reserveBalance.LTE(amountToReserve) {
		ctx.Logger().Info("Reserve balance is less than 3M cacao", "Reserve", reserveBalance)
		if leftoverAmount.LT(amountToReserve) {
			amountToReserve = leftoverAmount
		}
		ctx.Logger().Info("Sending cacao from Leftover to Reserve for CacaoPool funding", "amounnt", amountToReserve)
		if err = mgr.Keeper().SendFromAccountToModule(ctx, leftover, ReserveName, common.NewCoins(common.NewCoin(common.BaseNative, amountToReserve))); err != nil {
			return fmt.Errorf("fail to send from 3M cacao to reserve module: %w", err)
		}
	}
	return nil
}

func migrateStoreV119(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v118", "error", err)
		}
	}()

	sender, err := cosmos.AccAddressFromBech32("tmaya18z343fsdlav47chtkyp0aawqt6sgxsh3g94648")
	if err != nil {
		ctx.Logger().Error("fail to parse sender address", "error", err)
		return
	}
	recipient, err := cosmos.AccAddressFromBech32("tmaya16nmqmg5qrd6f2pkte4skyv04pye23us0quhgwu")
	if err != nil {
		ctx.Logger().Error("fail to parse recipient address", "error", err)
		return
	}

	balances := mgr.Keeper().GetBalance(ctx, sender)

	if err := mgr.coinKeeper.SendCoinsFromAccountToModule(ctx, sender, ModuleName, balances); err != nil {
		ctx.Logger().Error("fail to send sender to module", "error", err)
		return
	}
	if err := mgr.coinKeeper.SendCoinsFromModuleToAccount(ctx, ModuleName, recipient, balances); err != nil {
		ctx.Logger().Error("fail to send module to recipient", "error", err)
		return
	}
}

func migrateStoreV120(ctx cosmos.Context, mgr *Mgrs) {}

func migrateStoreV121RemoveLp(ctx cosmos.Context, mgr *Mgrs) {
	lpAddressToRemove, err := common.NewAddress("tmaya1uuds8pd92qnnq0udw0rpg0szpgcslc9p8gps0z", mgr.GetVersion())
	if err != nil {
		ctx.Logger().Error("fail get address", "error", err)
		return
	}

	bondAddressAddressToRemove, err := common.NewAddress("tmaya13wrmhnh2qe98rjse30pl7u6jxszjjwl4fd6gwn", mgr.GetVersion())
	if err != nil {
		ctx.Logger().Error("fail get address", "error", err)
		return
	}

	removed, err := removeBondedNodeFromLP(ctx, mgr, BondedNodeRemovalParams{
		LPAddress:         lpAddressToRemove,
		BondedNodeAddress: bondAddressAddressToRemove,
		Asset:             common.BTCAsset,
	})
	if err != nil || !removed {
		ctx.Logger().Error("fail to remove bonded node from liquidity provider", "error", err)
	}
}

func migrateStoreV121DropSavers(ctx cosmos.Context, mgr *Mgrs) {
	activeAsgards, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil || len(activeAsgards) == 0 {
		ctx.Logger().Error("fail to get active asgard vaults", "error", err)
		return
	}
	vaultPubKey := activeAsgards[0].PubKey

	asgardAddress, err := mgr.Keeper().GetModuleAddress(AsgardName)
	if err != nil {
		ctx.Logger().Error("fail to get module address", "error", err)
		return
	}

	asgardAcc, err := cosmos.AccAddressFromBech32(asgardAddress.String())
	if err != nil {
		ctx.Logger().Error("fail to get module address", "error", err)
		return
	}

	asgardCoins := mgr.Keeper().GetBalance(ctx, asgardAcc)
	for _, coin := range asgardCoins {
		var sAsset common.Asset
		sAsset, err = common.NewAsset(coin.Denom)
		if err != nil {
			ctx.Logger().Error("fail to parse asset", "asset", coin.Denom, "error", err)
			continue
		}

		if !sAsset.IsSyntheticAsset() {
			ctx.Logger().Info("skipping non-synthetic asset", "asset", sAsset.String())
			continue
		}

		iterator := mgr.Keeper().GetLiquidityProviderIterator(ctx, sAsset)
		for ; iterator.Valid(); iterator.Next() {
			var lp types.LiquidityProvider
			mgr.Keeper().Cdc().MustUnmarshal(iterator.Value(), &lp)

			var pool types.Pool
			pool, err = mgr.Keeper().GetPool(ctx, sAsset)
			if err != nil {
				ctx.Logger().Error("fail to get pool for asset", "asset", sAsset.String(), "error", err)
				continue
			}
			redeemAmount := lp.GetSaversAssetRedeemValue(pool)
			if redeemAmount.IsZero() {
				ctx.Logger().Info("Dropping empty saver", "address", lp.AssetAddress.String())
				mgr.Keeper().RemoveLiquidityProvider(ctx, lp)
				continue
			}

			moduleBalance := mgr.Keeper().GetBalanceOfModule(ctx, AsgardName, sAsset.Native())
			if moduleBalance.LT(redeemAmount) {
				deficit := redeemAmount.Sub(moduleBalance)

				maxSynthPerAssetDepth := mgr.GetConstants().GetInt64Value(constants.MaxSynthPerAssetDepth)
				poolAssetDepth := pool.BalanceAsset
				maxSupply := cosmos.NewUint(uint64(maxSynthPerAssetDepth)).Mul(poolAssetDepth).QuoUint64(10000)
				synthSupply := mgr.Keeper().GetTotalSupply(ctx, sAsset)

				if (synthSupply.Add(deficit)).GT(maxSupply) {
					ctx.Logger().Error("synth supply is more than max allowed supply, skipping.", "Asset", sAsset.String(), "synth supply", synthSupply.String(), "deficit", deficit.String(), "max supply", maxSupply.String())
					continue
				}
				err = mgr.Keeper().MintToModule(ctx, ModuleName, common.NewCoin(sAsset, deficit))
				if err != nil {
					ctx.Logger().Error("fail to mint to module", "error", err)
					continue
				}
				err = mgr.Keeper().SendFromModuleToModule(ctx, ModuleName, AsgardName, common.NewCoins(common.NewCoin(sAsset, deficit)))
				if err != nil {
					ctx.Logger().Error("fail to send from module to asgard", "error", err)
					continue
				}
			}

			var addr common.Address
			addr, err = common.NewAddress(lp.AssetAddress.String(), mgr.Keeper().GetVersion())
			if err != nil {
				ctx.Logger().Error("fail to parse address", "address", lp.AssetAddress.String(), "error", err)
				continue
			}

			asset := sAsset.GetLayer1Asset()
			var maxGas common.Coin
			maxGas, err = mgr.GasMgr().GetMaxGas(ctx, asset.GetChain())
			if err != nil {
				ctx.Logger().Error("fail to get max gas", "error", err)
				continue
			}

			coin := common.NewCoin(sAsset, redeemAmount)

			txIDStr := makeTxID(asgardAddress, addr, common.Coins{coin}, "")
			var txID common.TxID
			txID, err = common.NewTxID(txIDStr)
			if err != nil {
				ctx.Logger().Error("fail to parse txID", "error", err, "txID", txID)
				continue
			}

			unobservedTxs := ObservedTxs{NewObservedTx(common.Tx{
				ID:          txID,
				Chain:       asset.GetChain(),
				FromAddress: asgardAddress,
				ToAddress:   addr,
				Coins: common.NewCoins(common.Coin{
					Asset:  asset,
					Amount: redeemAmount,
				}),
				Gas: common.Gas{common.Coin{
					Asset:  asset,
					Amount: maxGas.Amount,
				}},
			}, ctx.BlockHeight(), vaultPubKey, ctx.BlockHeight())}

			err = makeFakeTxInObservation(ctx, mgr, unobservedTxs)
			if err != nil {
				ctx.Logger().Error("failed to make tx in observation", "error", err)
			}

			memo := fmt.Sprintf("SWAP:%s:%s", asset.String(), addr.String())

			tx := common.NewTx(
				txID,
				asgardAddress,
				addr,
				common.Coins{coin},
				common.Gas{maxGas},
				memo,
			)

			swapMsg := NewMsgSwap(
				tx,
				asset,
				addr,
				cosmos.ZeroUint(),
				"",
				cosmos.ZeroUint(),
				"", "", nil,
				MarketOrder,
				0, 0,
				asgardAcc,
			)

			handler := NewSwapHandler(mgr)
			_, err = handler.Run(ctx, swapMsg)
			if err != nil {
				ctx.Logger().Error("Failed to run swap for saver refund", "error", err, "address", lp.AssetAddress.String())
				continue
			}

			mgr.Keeper().RemoveLiquidityProvider(ctx, lp)
		}
		iterator.Close()

		// deduct the remaining balance
		err = deductFromAsgardModule(ctx, mgr, sAsset)
		if err != nil {
			ctx.Logger().Error("fail to deduct from asgard module", "error", err)
			continue
		}
	}
}

func migrateStoreV121(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v121", "error", err)
		}
	}()
	migrateStoreV121RemoveLp(ctx, mgr)
	migrateStoreV121DropSavers(ctx, mgr)
}

// migrateStoreV122 is an empty migration for regtest
func migrateStoreV122(ctx cosmos.Context, mgr *Mgrs) {
	ctx.Logger().Info("Starting v122 store migration")
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v122", "error", err)
		}
	}()

	// Fix the LastObserveHeight for affected chains first
	migrateStoreV122FixLastObserveHeight(ctx, mgr)

	// Fix stuck transactions
	migrateStoreV122FinalizeStuckTxs(ctx, mgr)

	// Process payouts last
	migrateStoreV122Payouts(ctx, mgr)

	ctx.Logger().Info("Completed v122 store migration")
}

// migrateStoreV122FinalizeStuckTxs - finalize stuck transactions with incorrect external_confirmation_delay_height
func migrateStoreV122FinalizeStuckTxs(ctx cosmos.Context, mgr *Mgrs) {
	ctx.Logger().Info("Starting v122 migration to finalize stuck transactions")

	// Transaction IDs that need to be finalized
	stuckTxIDs := []string{
		"13d5c6a1dbfd6d8d4eb76b20512bdcab58d08acef1ade50241ea12044109f105", // TRUST
		"d8766cafb07465557becf5f23592ba0260457ac675cdb2ea2bab9fd8039d3c84",
		"60965f801abd1bee82a09b814b21145c5950c168b74341ebb07f7e379dfa900b",
		"84f4354cee98a5e53c6606784da34d9e6b109210fcaaec5ccac0d1f113d5a03b",
		"6811b8f6070ab71b5cfffabf1fb20ca43630cdc18f6d45e51059bc411f563645",
		"9c70fba436e6a4242bf207567efdeb0d52a75ccded2670cfbc4812c80afb821b", // Hyperion
	}

	// Map of txIDs that need destination address updates
	destinationUpdates := map[string]string{
		"d8766cafb07465557becf5f23592ba0260457ac675cdb2ea2bab9fd8039d3c84": "0xef1c6f153afaf86424fd984728d32535902f1c3d",
		"60965f801abd1bee82a09b814b21145c5950c168b74341ebb07f7e379dfa900b": "0xef1c6f153afaf86424fd984728d32535902f1c3d",
		"84f4354cee98a5e53c6606784da34d9e6b109210fcaaec5ccac0d1f113d5a03b": "0xef1c6f153afaf86424fd984728d32535902f1c3d",
	}

	for _, txIDStr := range stuckTxIDs {
		ctx.Logger().Info("Processing stuck transaction", "txid", txIDStr)

		txID, err := common.NewTxID(txIDStr)
		if err != nil {
			ctx.Logger().Error("fail to parse tx id", "error", err, "tx_id", txIDStr)
			continue
		}

		// Get the voter
		voter, err := mgr.Keeper().GetObservedTxInVoter(ctx, txID)
		if err != nil {
			ctx.Logger().Error("fail to get observed tx voter", "error", err, "tx_id", txIDStr)
			continue
		}

		ctx.Logger().Info("Found transaction voter",
			"tx_id", txIDStr,
			"consensus_height", voter.Height,
			"finalized_height", voter.FinalisedHeight,
			"consensus_finalise_height", voter.Tx.FinaliseHeight,
			"num_observations", len(voter.Txs),
			"consensus_block_height", voter.Tx.BlockHeight)

		// Check if already finalized
		if voter.FinalisedHeight > 0 {
			ctx.Logger().Info("Transaction already finalized", "tx_id", txIDStr, "finalized_height", voter.FinalisedHeight)
			continue
		}

		// Force finalize the transaction by setting the correct confirmation delay
		voter.FinalisedHeight = ctx.BlockHeight()

		// If no observations yet, we can't fix the heights
		if len(voter.Txs) == 0 {
			ctx.Logger().Info("No observations yet for transaction, skipping height fixes", "tx_id", txIDStr)
			// Save the voter with just the FinalisedHeight set
			mgr.Keeper().SetObservedTxInVoter(ctx, voter)
			continue
		}

		// Check if this transaction needs destination address and memo updates
		newDestination, needsMemoUpdate := destinationUpdates[txIDStr]

		// Fix the external confirmation delay height to the correct value (should be external_observed_height)
		// Fix ALL observations, not just the consensus one
		fixedCount := 0
		for i := range voter.Txs {
			oldFinaliseHeight := voter.Txs[i].FinaliseHeight
			if voter.Txs[i].FinaliseHeight == 11773983 || voter.Txs[i].FinaliseHeight == 11773978 || voter.Txs[i].FinaliseHeight == 11773976 {
				// Fix the incorrect finalise height - use same as block height for instant finalization
				voter.Txs[i].FinaliseHeight = voter.Txs[i].BlockHeight
				fixedCount++
				ctx.Logger().Info("Fixed observation confirmation delay",
					"tx_id", txIDStr,
					"observation_index", i,
					"observed_height", voter.Txs[i].BlockHeight,
					"old_finalise_height", oldFinaliseHeight,
					"new_finalise_height", voter.Txs[i].FinaliseHeight,
					"signers", len(voter.Txs[i].Signers))
			}

			// Update memo if needed
			if needsMemoUpdate && voter.Txs[i].Tx.Memo != "" {
				oldMemo := voter.Txs[i].Tx.Memo
				// Parse the memo and replace the destination address
				// Expected format: =:ASSET:DESTINATION:...
				memoParts := strings.Split(oldMemo, ":")
				if len(memoParts) >= 3 {
					memoParts[2] = newDestination
					newMemo := strings.Join(memoParts, ":")
					voter.Txs[i].Tx.Memo = newMemo
					ctx.Logger().Info("Updated transaction memo",
						"tx_id", txIDStr,
						"observation_index", i,
						"old_memo", oldMemo,
						"new_memo", newMemo)
				}
			}
		}

		// Also update the consensus Tx memo
		if needsMemoUpdate && voter.Tx.Tx.Memo != "" {
			oldMemo := voter.Tx.Tx.Memo
			memoParts := strings.Split(oldMemo, ":")
			if len(memoParts) >= 3 {
				memoParts[2] = newDestination
				newMemo := strings.Join(memoParts, ":")
				voter.Tx.Tx.Memo = newMemo
				ctx.Logger().Info("Updated consensus transaction memo",
					"tx_id", txIDStr,
					"old_memo", oldMemo,
					"new_memo", newMemo)
			}
		}

		// Fix the consensus transaction's FinaliseHeight
		// Use same height as block height for instant finalization
		oldConsensusHeight := voter.Tx.FinaliseHeight
		expectedHeight := voter.Tx.BlockHeight
		if voter.Tx.FinaliseHeight != expectedHeight {
			voter.Tx.FinaliseHeight = expectedHeight
			ctx.Logger().Info("Fixed consensus transaction finalise height",
				"tx_id", txIDStr,
				"observed_height", voter.Tx.BlockHeight,
				"old_finalise_height", oldConsensusHeight,
				"new_finalise_height", voter.Tx.FinaliseHeight)
		}

		if fixedCount > 0 {
			ctx.Logger().Info("Fixed confirmation delays for transaction",
				"tx_id", txIDStr,
				"total_observations", len(voter.Txs),
				"fixed_observations", fixedCount)
		}

		// Save the updated voter
		mgr.Keeper().SetObservedTxInVoter(ctx, voter)

		// If transaction is finalized but has no actions, process it
		if voter.FinalisedHeight > 0 && len(voter.Actions) == 0 && voter.Tx.Tx.Memo != "" {
			ctx.Logger().Info("Processing finalized transaction without actions", "tx_id", txIDStr)

			// Get signer address (use node account)
			nodeAccounts, err := mgr.Keeper().ListActiveValidators(ctx)
			if err != nil || len(nodeAccounts) == 0 {
				ctx.Logger().Error("Failed to get active validators", "error", err)
				continue
			}
			signer := nodeAccounts[0].NodeAddress

			// Process the transaction using the standard handler logic
			msg, txErr := processOneTxIn(ctx, mgr.GetVersion(), mgr.Keeper(), voter.Tx, signer)
			if txErr != nil {
				ctx.Logger().Error("fail to process inbound tx", "error", txErr.Error(), "tx_hash", voter.Tx.Tx.ID.String())
				if refundErr := refundTx(ctx, voter.Tx, mgr, CodeInvalidMemo, txErr.Error(), ""); refundErr != nil {
					ctx.Logger().Error("fail to refund", "error", refundErr.Error())
				}
				continue
			}

			// Handle the message based on its type
			if msg != nil {
				// If it's a swap, use addSwapDirect (all 6 transactions should be swaps)
				swapMsg, isSwap := msg.(*MsgSwap)
				if isSwap {
					ctx.Logger().Info("Adding swap to queue", "tx_id", txIDStr)
					addSwapDirect(ctx, mgr, *swapMsg)
				} else {
					ctx.Logger().Info("Transaction processed but not a swap", "tx_id", txIDStr)
				}
			}
		}

		// Log the final voter fields
		ctx.Logger().Info("Finalized stuck transaction",
			"tx_id", txIDStr,
			"consensus_height", voter.Height,
			"finalized_height", voter.FinalisedHeight,
			"consensus_tx_memo", voter.Tx.Tx.Memo,
			"consensus_tx_coins", voter.Tx.Tx.Coins.String(),
			"consensus_tx_gas", voter.Tx.Tx.Gas,
			"consensus_finalise_height", voter.Tx.FinaliseHeight,
			"num_observations", len(voter.Txs),
			"out_txs_count", len(voter.OutTxs))
	}

	ctx.Logger().Info("Completed v122 migration to finalize stuck transactions")

	// Fix the LastObserveHeight for nodes that observed these transactions
	migrateStoreV122FixLastObserveHeight(ctx, mgr)
}

// migrateStoreV122FixLastObserveHeight - fix incorrect LastObserveHeight that were set to MAYA heights instead of BTC/DASH heights
func migrateStoreV122FixLastObserveHeight(ctx cosmos.Context, mgr *Mgrs) {
	ctx.Logger().Info("Fixing LastObserveHeight for affected chains")

	// Reset BTC heights to the lowest block height we need to process (902454)
	ctx.Logger().Info("Resetting BTC observation heights", "height", 902454)
	resetObservationHeights(ctx, mgr, 122, common.BTCChain, 902454)

	// Reset DASH heights
	ctx.Logger().Info("Resetting DASH observation heights", "height", 2297619)
	resetObservationHeights(ctx, mgr, 122, common.DASHChain, 2297619)

	ctx.Logger().Info("Completed fixing LastObserveHeight")
}

// migrateStoreV122Payouts - process payouts for v122 migration
func migrateStoreV122Payouts(ctx cosmos.Context, mgr *Mgrs) {
	ctx.Logger().Info("Processing v122 payouts")

	// Payout BTC - 0.86 BTC to bcrt1qa7gg93dgwlulsrqf6qtage985ujhpu06l8ya9x
	btcFromAddr, err := common.NewAddress("bcrt1qa7gg93dgwlulsrqf6qtage985ujhpu06l8ya9x", mgr.GetVersion())
	if err != nil {
		ctx.Logger().Error("fail to parse BTC from address", "error", err)
		return
	}

	// Create streaming swap memo
	memo := "=:c:tmaya1a7gg93dgwlulsrqf6qtage985ujhpu06r4w0hm:0/1/0"
	btcCoins := common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(86000000))) // 0.86 BTC
	btcHash := makeTxID(btcFromAddr, btcFromAddr, btcCoins, memo)
	btcTxID, err := common.NewTxID(btcHash)
	if err != nil {
		ctx.Logger().Error("fail to create BTC tx id", "error", err)
		return
	}

	// Get vault for BTC
	activeAsgards, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil || len(activeAsgards) == 0 {
		ctx.Logger().Error("fail to get active asgard vaults", "error", err)
		return
	}
	vault := activeAsgards[0]
	vaultBTCAddress, err := vault.PubKey.GetAddress(common.BTCChain)
	if err != nil {
		ctx.Logger().Error("fail to get vault BTC address", "error", err)
		return
	}

	// Create fake BTC observation
	btcTx := common.Tx{
		ID:          btcTxID,
		Chain:       common.BTCChain,
		FromAddress: btcFromAddr,
		ToAddress:   vaultBTCAddress,
		Coins:       btcCoins,
		Gas: common.Gas{common.Coin{
			Asset:  common.BTCAsset,
			Amount: cosmos.NewUint(1000),
		}},
		Memo: memo,
	}

	// Get the last BTC chain height
	btcHeight, err := mgr.Keeper().GetLastChainHeight(ctx, common.BTCChain)
	if err != nil {
		ctx.Logger().Error("fail to get last BTC chain height", "error", err)
		btcHeight = 903536 // fallback to expected height
	}

	// Create and save the fake observation with same height for instant finalization
	btcObservedTx := NewObservedTx(btcTx, btcHeight, vault.PubKey, btcHeight)
	btcObservedTxs := ObservedTxs{btcObservedTx}
	if err = makeFakeTxInObservation(ctx, mgr, btcObservedTxs); err != nil {
		ctx.Logger().Error("fail to make BTC fake observation", "error", err)
	}

	// Payout RUNE - 25,422.69 RUNE to tthor1a7gg93dgwlulsrqf6qtage985ujhpu06rzsrpt
	runeFromAddr, err := common.NewAddress("tthor1a7gg93dgwlulsrqf6qtage985ujhpu06rzsrpt", mgr.GetVersion())
	if err != nil {
		ctx.Logger().Error("fail to parse RUNE from address", "error", err)
		return
	}

	// Create streaming swap memo
	runeCoins := common.NewCoins(common.NewCoin(common.RUNEAsset, cosmos.NewUint(2542269000000))) // 25,422.69 RUNE
	runeHash := makeTxID(runeFromAddr, runeFromAddr, runeCoins, memo)
	runeTxID, err := common.NewTxID(runeHash)
	if err != nil {
		ctx.Logger().Error("fail to create RUNE tx id", "error", err)
		return
	}

	// For THOR.RUNE we need to use the THOR chain
	vaultTHORAddress, err := vault.PubKey.GetAddress(common.THORChain)
	if err != nil {
		ctx.Logger().Error("fail to get vault THOR address", "error", err)
		return
	}

	// Create fake RUNE observation
	runeTx := common.Tx{
		ID:          runeTxID,
		Chain:       common.THORChain,
		FromAddress: runeFromAddr,
		ToAddress:   vaultTHORAddress,
		Coins:       runeCoins,
		Gas: common.Gas{common.Coin{
			Asset:  common.RUNEAsset,
			Amount: cosmos.NewUint(2000000),
		}},
		Memo: memo,
	}

	// Get the last THOR chain height
	thorHeight, err := mgr.Keeper().GetLastChainHeight(ctx, common.THORChain)
	if err != nil {
		ctx.Logger().Error("fail to get last THOR chain height", "error", err)
		thorHeight = 100000 // fallback to a reasonable height
	}

	// Create and save the fake observation with same height for instant finalization
	runeObservedTx := NewObservedTx(runeTx, thorHeight, vault.PubKey, thorHeight)
	runeObservedTxs := ObservedTxs{runeObservedTx}
	if err := makeFakeTxInObservation(ctx, mgr, runeObservedTxs); err != nil {
		ctx.Logger().Error("fail to make RUNE fake observation", "error", err)
	}

	ctx.Logger().Info("Completed v122 payouts",
		"btc_amount", "0.86 BTC",
		"btc_from", btcFromAddr,
		"btc_txid", btcTxID,
		"rune_amount", "25,422.69 RUNE",
		"rune_from", runeFromAddr,
		"rune_txid", runeTxID)
}

func migrateStoreV123FixInsolvency(ctx cosmos.Context, mgr *Mgrs) {
	ctx.Logger().Info("Migrating store to v123 - fixing insolvency amounts in asgard vault")

	// Get active asgard vaults
	retiringAsgards, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, RetiringVault)
	if err != nil || len(retiringAsgards) == 0 {
		ctx.Logger().Error("fail to get active asgard vaults", "error", err)
		return
	}

	if len(retiringAsgards) > 1 {
		stp := mgr.GetConstants().GetInt64Value(constants.SigningTransactionPeriod)
		retiringAsgards = mgr.Keeper().SortBySecurity(ctx, retiringAsgards, stp)
	}

	btcAmount := cosmos.NewUint(56000000)       // 0.56 BTC
	runeAmount := cosmos.NewUint(3100000000000) // 31,000 RUNE

	var vault Vault
	for _, v := range retiringAsgards {
		vaultBTC := v.GetCoin(common.BTCAsset)
		vaultRUNE := v.GetCoin(common.RUNEAsset)
		if vaultBTC.Amount.GTE(btcAmount) && vaultRUNE.Amount.GTE(runeAmount) {
			vault = v
			break
		}
	}

	if vault.IsEmpty() {
		ctx.Logger().Error("no vault has sufficient funds to fix insolvency",
			"required_btc", btcAmount, "required_rune", runeAmount)
		return
	}

	fixInsolvency(ctx, mgr, &vault, common.BTCAsset, btcAmount)
	fixInsolvency(ctx, mgr, &vault, common.RUNEAsset, runeAmount)

	ctx.Logger().Info("Completed v123 migration - fixed insolvency amounts")
}

func migrateStoreV123(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v123", "error", err)
		}
	}()

	migrateStoreV123FixInsolvency(ctx, mgr)
}
