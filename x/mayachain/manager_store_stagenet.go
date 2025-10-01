//go:build stagenet
// +build stagenet

package mayachain

import (
	"fmt"
	"strconv"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

func importPreRegistrationMAYANames(ctx cosmos.Context, mgr Manager) error {
	oneYear := mgr.Keeper().GetConfigInt64(ctx, constants.BlocksPerYear)
	names, err := getPreRegisterMAYANames(ctx, ctx.BlockHeight()+oneYear, mgr.GetVersion())
	if err != nil {
		return err
	}

	for _, name := range names {
		mgr.Keeper().SetMAYAName(ctx, name)
	}
	return nil
}

func migrateStoreV96(ctx cosmos.Context, mgr Manager)  {}
func migrateStoreV102(ctx cosmos.Context, mgr *Mgrs)   {}
func migrateStoreV104(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV105(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV106(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV107(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV108(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV109(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV110(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV111(ctx cosmos.Context, mgr *Mgrs)   {}
func migrateStoreV112(ctx cosmos.Context, mgr *Mgrs)   {}
func migrateStoreV113(ctx cosmos.Context, mgr *Mgrs)   {}
func migrateStoreV114(ctx cosmos.Context, mgr *Mgrs)   {}
func migrateStoreV115(ctx cosmos.Context, mgr *Mgrs)   {}

func migrateStoreV116(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v116", "error", err)
		}
	}()

	txID := common.TxID("ABD2D189814E8F069C5437EBC6FE672C7DABAF8C715448773DFE26341E964D76")

	newDestinationAddrString := "0xef1c6f153afaf86424fd984728d32535902f1c3d"
	newDestinationAddr, err := common.NewAddress(newDestinationAddrString, mgr.GetVersion())
	if err != nil {
		ctx.Logger().Error("fail to parse address", "error", err)
		return
	}

	voter, err := mgr.K.GetObservedTxInVoter(ctx, txID)
	if err != nil {
		ctx.Logger().Error("fail to get observed tx in voter", "error", err)
		return
	}

	newMemo := fmt.Sprintf("=:ETH.USDC:%s:0/5/10", newDestinationAddrString)
	voter.Tx.Tx.Memo = newMemo

	for i, tx := range voter.Txs {
		tx.Tx.Memo = newMemo
		voter.Txs[i] = tx
	}

	mgr.K.SetObservedTxInVoter(ctx, voter)

	iterator := mgr.Keeper().GetSwapQueueIterator(ctx)
	defer iterator.Close()
	index := 0
	for ; iterator.Valid(); iterator.Next() {
		var msg MsgSwap
		if err := mgr.Keeper().Cdc().Unmarshal(iterator.Value(), &msg); err != nil {
			ctx.Logger().Error("fail to fetch swap msg from queue", "error", err)
			continue
		}

		if msg.IsStreaming() && msg.Tx.ID.Equals(txID) {
			msg.Destination = newDestinationAddr
			msg.Tx.Memo = newMemo
			if err := mgr.Keeper().SetSwapQueueItem(ctx, msg, index); err != nil {
				ctx.Logger().Error("fail to save swap msg to queue", "error", err)
			}
			return
		}
		index++
	}
}

func migrateStoreV117(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v116", "error", err)
		}
	}()

	txID := common.TxID("ABD2D189814E8F069C5437EBC6FE672C7DABAF8C715448773DFE26341E964D76")

	newDestinationAddrString := "0xEf1C6F153afaf86424fd984728d32535902F1c3D"
	newDestinationAddr, err := common.NewAddress(newDestinationAddrString, mgr.GetVersion())
	if err != nil {
		ctx.Logger().Error("fail to parse address", "error", err)
		return
	}

	newMemo := fmt.Sprintf("=:ETH.USDC:%s:0/5/10", newDestinationAddrString)
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
			} else {
				msg.Destination = newDestinationAddr
				msg.Tx.Memo = newMemo
				if err := mgr.Keeper().SetSwapQueueItem(ctx, msg, 0); err != nil {
					ctx.Logger().Error("fail to save swap msg to queue", "error", err)
				}
			}
			return
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
	var err error
	if err = consolidateStagenetFundsV118(ctx, mgr); err != nil {
		ctx.Logger().Error("fail to consolidate stagenet funds", "error", err)
	}

	threeMillionCacao := uint64(3_000_000_0000000000)
	coinsToCacaoPool := common.NewCoin(common.BaseNative, cosmos.NewUint(threeMillionCacao))
	if err = mgr.Keeper().SendFromModuleToModule(ctx, ReserveName, CACAOPoolName, common.NewCoins(coinsToCacaoPool)); err != nil {
		ctx.Logger().Error("fail to move coins from Reserve to CACAOPool", "error", err)
	}
}

func consolidateStagenetFundsV118(ctx cosmos.Context, mgr *Mgrs) error {
	asgard := mgr.Keeper().GetModuleAccAddress(AsgardName)
	reserve := mgr.Keeper().GetModuleAccAddress(ReserveName)
	leftover, err := cosmos.AccAddressFromBech32("smaya18z343fsdlav47chtkyp0aawqt6sgxsh3ctcu6u")
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
		}

		ctx.Logger().Info("Sending cacao excess amount to mayachain module for burning", "from", leftover, "amount", excess)
		if err = mgr.Keeper().SendFromAccountToModule(ctx, leftover, ModuleName, common.NewCoins(common.NewCoin(common.BaseNative, excess))); err != nil {
			return fmt.Errorf("fail to send from leftover to mayachain module: %w", err)
		}
		if err = mgr.Keeper().BurnFromModule(ctx, ModuleName, common.NewCoin(common.BaseNative, excess)); err != nil {
			return fmt.Errorf("fail to burn excess coins: %w", err)
		}
	}
	return nil
}

func migrateStoreV119(ctx cosmos.Context, mgr *Mgrs) {}
func migrateStoreV120(ctx cosmos.Context, mgr *Mgrs) {}

func migrateStoreV121(ctx cosmos.Context, mgr *Mgrs) {
	cacaoPool, err := mgr.Keeper().GetCACAOPool(ctx)
	if err != nil {
		ctx.Logger().Error("fail to get cacao pool", "error", err)
	} else {
		threeMillionCacao := cosmos.NewUint(uint64(3_000_000_0000000000))
		cacaoPool.ReserveUnits = threeMillionCacao
		mgr.Keeper().SetCACAOPool(ctx, cacaoPool)
	}
}

// migrateStoreV122 is an empty migration for stagenet
func migrateStoreV122(ctx cosmos.Context, mgr *Mgrs) {}
func migrateStoreV123(ctx cosmos.Context, mgr *Mgrs) {}
