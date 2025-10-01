package keeperv1

import (
	"errors"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

func (k KVStore) setNetworkFee(ctx cosmos.Context, key string, record NetworkFee) {
	store := ctx.KVStore(k.storeKey)
	buf := k.cdc.MustMarshal(&record)
	if buf == nil {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), buf)
	}
}

func (k KVStore) getNetworkFee(ctx cosmos.Context, key string, record *NetworkFee) (bool, error) {
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return false, nil
	}

	bz := store.Get([]byte(key))
	if err := k.cdc.Unmarshal(bz, record); err != nil {
		return true, dbError(ctx, fmt.Sprintf("Unmarshal kvstore: (%T) %s", record, key), err)
	}
	return true, nil
}

func (k KVStore) GetCacaoOnPools(ctx cosmos.Context, poolStatus types.PoolStatus) (sdk.Uint, error) {
	cacao := sdk.ZeroUint()
	pools, err := k.GetPools(ctx)
	if err != nil {
		return cacao, err
	}
	for _, pool := range pools {
		if poolStatus != AllPoolStatus && pool.Status != poolStatus {
			continue
		}
		cacao = cacao.Add(pool.BalanceCacao)
	}

	return cacao, nil
}

// GetNetworkFee get the network fee of the given chain from kv store , if it doesn't exist , it will create an empty one
func (k KVStore) GetNetworkFee(ctx cosmos.Context, chain common.Chain) (NetworkFee, error) {
	record := NetworkFee{
		Chain:              chain,
		TransactionSize:    0,
		TransactionFeeRate: 0,
	}
	_, err := k.getNetworkFee(ctx, k.GetKey(ctx, prefixNetworkFee, chain.String()), &record)
	return record, err
}

// SaveNetworkFee save the network fee to kv store
func (k KVStore) SaveNetworkFee(ctx cosmos.Context, chain common.Chain, networkFee NetworkFee) error {
	if err := networkFee.Valid(); err != nil {
		return err
	}
	k.setNetworkFee(ctx, k.GetKey(ctx, prefixNetworkFee, chain.String()), networkFee)
	return nil
}

// GetNetworkFeeIterator
func (k KVStore) GetNetworkFeeIterator(ctx cosmos.Context) cosmos.Iterator {
	return k.getIterator(ctx, prefixNetworkFee)
}

// Distribute the tokens that are in the MayaFund in MinRuneForMayaFundDist multiples, the rest
// should stay on the MayaFund and should be distributed until it reaches another multiple.
// Loss of precision in token.Amount is desired and expected.
func (k KVStore) DistributeMayaFund(ctx cosmos.Context, constAccessor constants.ConstantValues) {
	accounts := k.accountKeeper.GetAllAccounts(ctx)
	mayaFundBalance := k.GetRuneBalanceOfModule(ctx, MayaFund)
	token := common.NewCoin(common.BaseNative, mayaFundBalance)
	totalAmountOfMaya := k.GetTotalSupply(ctx, common.MayaNative)

	// Check if we have a new mimir value
	minMultiple, err := k.GetMimir(ctx, constants.MinCacaoForMayaFundDist.String())
	if minMultiple < 0 || err != nil {
		minMultiple = constAccessor.GetInt64Value(constants.MinCacaoForMayaFundDist)
	}

	// Distribute only if the amount of token is at least MinMultiple
	if token.Amount.GTE(sdk.NewUint((uint64)(minMultiple))) {
		// Iterate for all the available accounts
		for _, acc := range accounts {

			// Get the amount of MayaToken from the account
			accBalance := k.GetBalance(ctx, acc.GetAddress())
			mayaBalance := accBalance.AmountOf(common.MayaNative.Native())

			// Check if account has MayaToken
			if !mayaBalance.IsZero() {
				millionToken := token.Amount.QuoUint64((uint64)(minMultiple))
				tokenAmt := sdk.NewUint((uint64)(minMultiple)).Mul(millionToken)
				mayaAmt := common.GetSafeShare(sdk.NewUint(mayaBalance.Uint64()), totalAmountOfMaya, tokenAmt)
				mayaCoins := common.NewCoins(common.NewCoin(token.Asset, mayaAmt))

				err = k.SendFromModuleToAccount(ctx, MayaFund, acc.GetAddress(), mayaCoins)
				if err != nil {
					ctx.Logger().Error("fail to send RUNE on MayaFund", "error", err)
				}
			}
		}
	}
}

// This function will mint some percentage of the inflation and distribute it to the pools and system income
// in case a threshold is not reach.
func (k KVStore) DynamicInflation(ctx cosmos.Context, constAccessor constants.ConstantValues) error {
	reserveBalance := k.GetRuneBalanceOfModule(ctx, ReserveName)
	amtCacaoOnChain := k.GetTotalSupply(ctx, common.BaseNative).Sub(reserveBalance)
	amtCacaoOnPools, err := k.GetCacaoOnPools(ctx, AllPoolStatus)
	if err != nil {
		return err
	}
	if amtCacaoOnPools.IsZero() {
		ctx.Logger().Info("DynamicInflation: No cacao on pools")
		return nil
	}

	mulValue, err := k.GetMimir(ctx, constants.InflationFormulaMulValue.String())
	if err != nil || mulValue < 0 || mulValue > 10000 {
		mulValue = constAccessor.GetInt64Value(constants.InflationFormulaMulValue)
	}
	sumValue, err := k.GetMimir(ctx, constants.InflationFormulaSumValue.String())
	if err != nil || sumValue < 0 || mulValue > 10000 {
		sumValue = constAccessor.GetInt64Value(constants.InflationFormulaSumValue)
	}
	yThold, err := k.GetMimir(ctx, constants.InflationPercentageThreshold.String())
	if err != nil || yThold < 0 || mulValue > 10000 {
		yThold = constAccessor.GetInt64Value(constants.InflationPercentageThreshold)
	}

	// y = CACAO in Pools divided by CACAO total supply minus CACAO on Reserve			[multiplied by 10000 to get decimals]
	y := amtCacaoOnPools.Mul(sdk.NewUint(10000)).Quo(amtCacaoOnChain)
	// inflation = ((1-y) * 40%) + 1%							                                  [multiplied by 10000 to get decimals]
	inflation := ((sdk.NewUint(10000).Sub(y)).Mul(sdk.NewUint(uint64(mulValue)))).Add(sdk.NewUint(uint64(10000 * sumValue)))

	// If 'y' is over a threshold or equal to zero it should not mint anything,
	// when ratio between cacao in pools and total supply is below 1 we won't distribute
	if y.GTE(sdk.NewUint(uint64(yThold))) || y.IsZero() {
		return nil
	}

	// Get the amount of cacao to mint based on the inflation, divied by BlocksPerYear to get value PerBlock.	[dividied by 10000*10000 from previous multiplications]
	ctx.Logger().Info("DynamicInflation", "Inflation value", float32(inflation.Uint64())/float32(1000000))
	cacaoToMint := amtCacaoOnChain.Mul(inflation).Quo(sdk.NewUint(uint64(constAccessor.GetInt64Value(constants.BlocksPerYear))).Mul(sdk.NewUint(100000000)))
	return k.distributeDynamicInflation(ctx, constAccessor, cacaoToMint)
}

// Distribute the amount of cacao specified on the DynamicInflation calculation
func (k KVStore) distributeDynamicInflation(ctx cosmos.Context, constAccessor constants.ConstantValues, cacaoToMint sdk.Uint) error {
	if cacaoToMint.IsZero() {
		return errors.New("nothing to mint on distributeDynamicInflation")
	}

	if err := k.MintToModule(ctx, ModuleName, common.NewCoin(common.BaseNative, cacaoToMint)); err != nil {
		return err
	}

	poolPerc, err := k.GetMimir(ctx, constants.InflationPoolPercentage.String())
	if err != nil || poolPerc < 0 {
		poolPerc = constAccessor.GetInt64Value(constants.InflationPoolPercentage)
	}

	// Distribute the cacao minted into the pools
	cacaoForPools := cacaoToMint.Mul(sdk.NewUint(uint64(poolPerc))).Quo((sdk.NewUint(100)))
	if !cacaoForPools.IsZero() {
		var amtCacaoOnPools sdk.Uint
		amtCacaoOnPools, err = k.GetCacaoOnPools(ctx, PoolAvailable)
		if err != nil {
			return err
		}
		var pools []types.Pool
		pools, err = k.GetPools(ctx)
		if err != nil {
			return err
		}
		for _, pool := range pools {
			if pool.Status != PoolAvailable {
				continue
			}
			perc := pool.BalanceCacao.Mul(sdk.NewUint(100)).Quo(amtCacaoOnPools)
			pool.BalanceCacao = pool.BalanceCacao.Add(cacaoForPools.Mul(perc).Quo(sdk.NewUint(100)))
			if err = k.SetPool(ctx, pool); err != nil {
				ctx.Logger().Error("fail to set pool on DynamicInflation", "error", err)
			}
		}
	}

	// Distribute the cacao minted into system income (90% Reserve, 10% MayaFund)
	cacaoForSystemIncome := cacaoToMint.Sub(cacaoForPools)
	if !cacaoForSystemIncome.IsZero() {
		mayaAmt := cacaoForSystemIncome.Mul(sdk.NewUint(uint64(constAccessor.GetInt64Value(constants.MayaFundPerc)))).Quo(sdk.NewUint(100))
		reserveAmt := cacaoForSystemIncome.Sub(mayaAmt)
		err = k.SendFromModuleToModule(ctx, ModuleName, ReserveName, common.NewCoins(common.NewCoin(common.BaseNative, reserveAmt)))
		if err != nil {
			return err
		}
		err = k.SendFromModuleToModule(ctx, ModuleName, MayaFund, common.NewCoins(common.NewCoin(common.BaseNative, mayaAmt)))
		if err != nil {
			return err
		}
	}

	ctx.EventManager().EmitEvent(
		cosmos.NewEvent("dynamic_inflation",
			cosmos.NewAttribute("cacao to mint", cacaoToMint.String()),
			cosmos.NewAttribute("cacao for pools", cacaoForPools.String()),
			cosmos.NewAttribute("cacao for system income", cacaoForSystemIncome.String())))

	return nil
}
