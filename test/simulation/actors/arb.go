package actors

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	openapi "gitlab.com/mayachain/mayanode/openapi/gen"
	. "gitlab.com/mayachain/mayanode/test/simulation/actors/common"
	"gitlab.com/mayachain/mayanode/test/simulation/pkg/mayanode"
	. "gitlab.com/mayachain/mayanode/test/simulation/pkg/types"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

////////////////////////////////////////////////////////////////////////////////////////
// ArbActor
////////////////////////////////////////////////////////////////////////////////////////

const maxSwapCount = 3

type ArbActor struct {
	Actor

	account     *Account
	mayaAddress cosmos.AccAddress

	swapCount int
}

func NewArbActor() *Actor {
	a := &ArbActor{
		Actor: Actor{
			Name: "Arbitrage",
			Ops:  []Op{},
		},
	}
	a.Timeout = time.Hour

	// init pool balances
	a.Ops = append(a.Ops, a.init)

	// lock an account to use for arb
	a.Ops = append(a.Ops, a.acquireUser)

	// enable trade assets
	a.Ops = append(a.Ops, a.enableTradeAssets)

	// convert all assets to trade assets
	a.Ops = append(a.Ops, a.bootstrapTradeAssets)

	// verify all assets are converted to trade assets
	a.Ops = append(a.Ops, a.verifyTradeAsset)

	// arb until pools are drained
	a.Ops = append(a.Ops, a.arb)

	return &a.Actor
}

////////////////////////////////////////////////////////////////////////////////////////
// Ops
////////////////////////////////////////////////////////////////////////////////////////

func (a *ArbActor) init(config *OpConfig) OpResult {
	return OpResult{
		Continue: true,
	}
}

func (a *ArbActor) acquireUser(config *OpConfig) OpResult {
	for _, user := range config.UserAccounts {
		// skip users already being used
		if !user.Acquire() {
			continue
		}

		cl := a.Log().With().Str("user", user.Name()).Logger()
		a.SetLogger(cl)

		// set acquired account and amounts in state context
		a.account = user

		// set mayachain address for later use
		mayaAddress, err := user.PubKey(common.BASEChain).GetThorAddress()
		if err != nil {
			a.Log().Error().Err(err).Msg("failed to get thor address")
			user.Release()
			continue
		}
		a.mayaAddress = mayaAddress

		break
	}

	// continue if we acquired a user
	if a.account != nil {
		a.Log().Info().Msg("acquired user")
		return OpResult{
			Continue: true,
		}
	}

	// remain pending if no user is available
	a.Log().Info().Msg("waiting for user with sufficient balance")
	return OpResult{
		Continue: false,
	}
}

func (a *ArbActor) enableTradeAssets(config *OpConfig) OpResult {
	node := config.AdminAccount
	// wait to acquire the node user
	if !node.Acquire() {
		return OpResult{
			Continue: false,
		}
	}
	// Release all the node users at the end of the function.
	defer node.Release()

	accAddr, err := node.PubKey(common.BASEChain).GetThorAddress()
	if err != nil {
		a.Log().Error().Err(err).Msg("failed to get thor address")
		return OpResult{
			Continue: false,
		}
	}
	mimirMsg := types.NewMsgMimir("TradeAccountsEnabled", 1, accAddr)
	txid, err := node.Mayachain.Broadcast(mimirMsg)
	if err != nil {
		a.Log().Error().Err(err).Msg("failed to broadcast tx")
		return OpResult{
			Continue: false,
		}
	}
	a.Log().Info().
		Stringer("txid", txid).
		Msg("broadcasted admin mimir tx to enable trade assets")

	return OpResult{
		Continue: true,
	}
}

func (a *ArbActor) bootstrapTradeAssets(config *OpConfig) OpResult {
	// get all pools
	pools, err := mayanode.GetPools()
	if err != nil {
		a.Log().Error().Err(err).Msg("failed to get pools")
		return OpResult{
			Continue: false,
			Error:    err,
		}
	}

	chainLocks := struct {
		sync.Mutex
		m map[common.Chain]*sync.Mutex
	}{m: make(map[common.Chain]*sync.Mutex)}

	bootstrap := func(pool openapi.Pool) {
		var asset common.Asset
		asset, err = common.NewAsset(pool.Asset)
		if err != nil {
			a.Log().Fatal().Err(err).Str("asset", pool.Asset).Msg("failed to create asset")
		}

		// lock chain
		chainLocks.Lock()
		chainLock, ok := chainLocks.m[asset.Chain]
		if !ok {
			chainLock = &sync.Mutex{}
			chainLocks.m[asset.Chain] = chainLock
		}
		chainLocks.Unlock()
		chainLock.Lock()
		defer chainLock.Unlock()

		// get deposit parameters for 90% of asset balance
		client := a.account.ChainClients[asset.Chain]
		memo := fmt.Sprintf("trade+:%s", a.mayaAddress)
		var l1Acct *common.Account
		l1Acct, err = a.account.ChainClients[asset.Chain].GetAccount(nil)
		if err != nil {
			a.Log().Fatal().Err(err).Msg("failed to get L1 account")
		}
		depositAmount := l1Acct.Coins.GetCoin(asset).Amount.QuoUint64(10).MulUint64(9)

		// make deposit
		var txid string
		if asset.Chain.IsEVM() && !asset.IsGasAsset() {
			txid, err = DepositL1Token(a.Log(), client, asset, memo, depositAmount)
		} else {
			txid, err = DepositL1(a.Log(), client, asset, memo, depositAmount)
		}
		if err != nil {
			a.Log().Fatal().
				Err(err).
				Str("asset", asset.String()).
				Stringer("amount", depositAmount).
				Msg("failed to deposit trade asset")
		}
		a.Log().Info().
			Stringer("asset", asset).
			Stringer("amount", depositAmount).
			Str("txid", txid).
			Msg("deposited trade asset")
	}

	// deposit trade assets for all pools
	wg := sync.WaitGroup{}
	for _, pool := range pools {
		wg.Add(1)
		go func(pool openapi.Pool) {
			defer wg.Done()
			bootstrap(pool)
		}(pool)
	}
	wg.Wait()

	// mark actor as backgrounded
	// a.Log().Info().Msg("moving arbitrage actor to background")
	// a.Background()

	return OpResult{
		Continue: true,
	}
}

func (a *ArbActor) verifyTradeAsset(config *OpConfig) OpResult {
	// get all pools
	pools, err := mayanode.GetPools()
	if err != nil {
		a.Log().Error().Err(err).Msg("failed to get pools")
		return OpResult{
			Continue: false,
			Error:    err,
		}
	}

	// get user trade assets
	mayaAddress, err := a.account.PubKey(common.BASEChain).GetAddress(common.BASEChain)
	if err != nil {
		a.Log().Error().Err(err).Msg("failed to get maya address")
		return OpResult{
			Continue: false,
			Error:    err,
		}
	}
	tas, err := mayanode.GetTradeAccount(mayaAddress)
	if err != nil {
		a.Log().Error().Err(err).Msg("failed to get trade account")
		return OpResult{
			Continue: false,
			Error:    err,
		}
	}

	// verify user has trade asset for every pool asset
	for _, pool := range pools {
		found := false
		poolAsset := strings.Replace(pool.Asset, ".", "~", 1)
		for _, ta := range tas {
			if poolAsset == ta.Asset && ta.Units != "0" {
				a.Log().Info().Str("asset", poolAsset).Msg("trade asset funding successful")
				found = true
				break
			}
		}
		if !found {
			a.Log().Info().Str("asset", poolAsset).Msg("pending trade asset funding")
			// remain pending if any asset is not available
			return OpResult{
				Continue: false,
			}
		}
	}

	a.Log().Info().Msg("all trade assets funded")

	// all trade assets funded
	return OpResult{
		Continue: true,
	}
}

func (a *ArbActor) arb(config *OpConfig) OpResult {
	cacaoNativeFee := cosmos.NewUint(2000000000)

	// if user doesn't have enough CACAO to pay tx fee then we are done
	mayaAddress, err := a.account.PubKey(common.BASEChain).GetAddress(common.BASEChain)
	if err != nil {
		a.Log().Error().Err(err).Msg("failed to get maya address")
		return OpResult{
			Continue: false,
			Error:    err,
		}
	}
	mayaBalances, err := mayanode.GetBalances(mayaAddress)
	if err != nil {
		a.Log().Error().Err(err).Msg("failed to get mayachain balances")
		return OpResult{
			Continue: false,
			Error:    err,
		}
	}
	if mayaBalances.GetCoin(common.BaseAsset()).Amount.LT(cacaoNativeFee) {
		a.Log().Info().Msg("user has insufficient CACAO balance")
		return OpResult{
			Continue: false,
			Error:    err,
		}
	}

	// get all pools
	pools, err := mayanode.GetPools()
	if err != nil {
		a.Log().Error().Err(err).Msg("failed to get pools")
		return OpResult{
			Continue: false,
			Error:    err,
		}
	}

	// if pools are drained then we are done
	if len(pools) == 0 {
		a.account.Release()
		a.Log().Info().Msg("pools are drained, nothing more to arb")
		return OpResult{
			Finish: true,
			Error:  nil,
		}
	}

	adjustmentBps := uint64(10)
	minCacaoValue := cacaoNativeFee.MulUint64(2)

	// gather pools we have seen
	arbPools := []openapi.Pool{}
	for _, pool := range pools {
		// skip unavailable pools and those with no liquidity
		if pool.BalanceCacao == "0" || pool.BalanceAsset == "0" || pool.Status != types.PoolStatus_Available.String() {
			continue
		}

		balanceAsset := cosmos.NewUintFromString(pool.BalanceAsset)
		balanceCacao := cosmos.NewUintFromString(pool.BalanceCacao)
		assetForMinCacao := calculateRequiredInputForOutputUint(balanceAsset, balanceCacao, minCacaoValue)
		cacaoForMinAsset := calculateRequiredInputForOutputUint(balanceCacao, balanceAsset, assetForMinCacao)

		depth := cosmos.NewUintFromString(pool.BalanceCacao)
		// skip if depth is too small
		if depth.LT(cacaoForMinAsset.MulUint64(2)) {
			continue
		}

		cacaoValue := depth.MulUint64(adjustmentBps).QuoUint64(2).QuoUint64(constants.MaxBasisPts)
		if cacaoValue.LT(minCacaoValue) {
			cacaoValue = minCacaoValue
		}
		// skip if depth is too small for successful swap
		if depth.LT(cacaoValue.MulUint64(2)) {
			continue
		}

		arbPools = append(arbPools, pool)
	}

	// skip if there are not enough pools to arb
	if len(arbPools) < 2 {
		a.Log().Info().Msg("not enough pools to arb")
		return OpResult{
			Finish: true,
		}
	}

	numPools := len(arbPools)
	var i1, i2 int
	// nolint
	i1 = rand.Intn(numPools)
	// nolint
	i2 = rand.Intn(numPools)
	for i2 == i1 { // keep generating i2 until it's different from i1
		// nolint
		i2 = rand.Intn(numPools)
	}
	send := arbPools[i1]
	receive := arbPools[i2]

	// build the swap
	minCacaoDepth := common.Min(cosmos.NewUintFromString(send.BalanceCacao).Uint64(), cosmos.NewUintFromString(receive.BalanceCacao).Uint64())
	cacaoValue := cosmos.NewUint(adjustmentBps * minCacaoDepth / 2 / constants.MaxBasisPts)
	if cacaoValue.LT(minCacaoValue) {
		cacaoValue = minCacaoValue
	}
	receiveBalanceAsset := cosmos.NewUintFromString(receive.BalanceAsset)
	receiveBalanceCacao := cosmos.NewUintFromString(receive.BalanceCacao)
	sendBalanceAsset := cosmos.NewUintFromString(send.BalanceAsset)
	sendBalanceCacao := cosmos.NewUintFromString(send.BalanceCacao)
	assetForCacao := calculateRequiredInputForOutputUint(receiveBalanceAsset, receiveBalanceCacao, cacaoValue)
	cacaoForAsset := calculateRequiredInputForOutputUint(receiveBalanceCacao, receiveBalanceAsset, assetForCacao)
	assetAmount := calculateRequiredInputForOutputUint(sendBalanceAsset, sendBalanceCacao, cacaoForAsset)
	// if result is zero this is not a good combination of pools for double swap
	if assetAmount.IsZero() {
		return OpResult{
			Continue: true,
		}
	}

	memo := fmt.Sprintf("=:%s", strings.Replace(receive.Asset, ".", "~", 1))
	asset, err := common.NewAsset(strings.Replace(send.Asset, ".", "~", 1))
	if err != nil {
		a.Log().Fatal().Err(err).Str("asset", send.Asset).Msg("failed to create asset")
	}

	coin := common.NewCoin(asset, assetAmount)

	// build the swap
	deposit := types.NewMsgDeposit(common.NewCoins(coin), memo, a.mayaAddress)
	a.Log().Info().Interface("deposit", deposit).Str("send_pool_cacao", send.BalanceCacao).Str("receive_pool_cacao", receive.BalanceCacao).Stringer("cacaoValue", cacaoValue).Msg("sending arb deposit msg")

	// broadcast the swap
	txid, err := a.account.Mayachain.Broadcast(deposit)
	if err != nil {
		a.Log().Error().Err(err).Msg("failed to broadcast tx")
		return OpResult{
			Continue: false,
		}
	}

	a.Log().Info().Stringer("txid", txid).Str("memo", memo).Msg("broadcasted arb tx")

	a.swapCount++
	if a.swapCount >= maxSwapCount {
		a.Log().Info().Msg("max swap count reached")
		return OpResult{
			Finish: true,
		}
	}

	return OpResult{
		Continue: false,
	}
}

// calculateRequiredInputForOutputUint calculates the required input amount of AssetX
// to get a `desiredOutputAmount` of AssetY
//
// Args:
//
//	X - poolBalanceInputAsset: Balance of the input asset (AssetX) in the pool (e.g., BTC balance).
//	Y - poolBalanceOutputAsset: Balance of the output asset (AssetY) in the pool (e.g., CACAO balance).
//	y - desiredOutputAmount: The desired amount of AssetY to be emitted (e.g., desired CACAO).
//
// Returns:
//
//	y - The required amount of Asset X, or an error if impossible/invalid.
//
// nolint
func calculateRequiredInputForOutputUint(X, Y, y cosmos.Uint) (x cosmos.Uint) {
	if y.IsZero() {
		return cosmos.ZeroUint()
	}
	if y.GTE(Y) {
		return cosmos.ZeroUint()
	}
	numerator := X.Mul(y)
	denominator := Y.Sub(y)
	if denominator.IsZero() {
		return cosmos.ZeroUint() // internal error: denominator is zero
	}
	x = numerator.Quo(denominator)
	return
}
