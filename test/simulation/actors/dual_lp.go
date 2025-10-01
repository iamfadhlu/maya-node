package actors

import (
	"fmt"

	"github.com/hashicorp/go-multierror"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/test/simulation/pkg/mayanode"
	. "gitlab.com/mayachain/mayanode/test/simulation/pkg/types"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

////////////////////////////////////////////////////////////////////////////////////////
// DualLPActor
////////////////////////////////////////////////////////////////////////////////////////

type DualLPActor struct {
	Actor

	asset       common.Asset
	account     *Account
	mayaAddress common.Address
	l1Address   common.Address
	runeAmount  cosmos.Uint
	l1Amount    cosmos.Uint
}

func NewDualLPActor(asset common.Asset) *Actor {
	a := &DualLPActor{
		Actor: Actor{
			Name: fmt.Sprintf("DualLP-%s", asset),
			Ops:  []Op{},
		},
		asset: asset,
	}

	// lock a user that has L1 and CACAO balance
	a.Ops = append(a.Ops, a.acquireUser)

	// deposit 10% of the user CACAO balance
	a.Ops = append(a.Ops, a.depositCacao)

	// deposit 10% of the user L1 balance to match
	a.Ops = append(a.Ops, a.depositL1)

	// ensure the lp is created and release the account
	a.Ops = append(a.Ops, a.verifyLP)

	return &a.Actor
}

////////////////////////////////////////////////////////////////////////////////////////
// Ops
////////////////////////////////////////////////////////////////////////////////////////

func (a *DualLPActor) acquireUser(config *OpConfig) OpResult {
	for _, user := range config.UserAccounts {
		a.SetLogger(a.Log().With().Str("user", user.Name()).Logger())

		// skip users already being used
		if !user.Acquire() {
			continue
		}

		// skip users that don't have CACAO balance
		mayaAddress, err := user.PubKey(common.BASEChain).GetAddress(common.BASEChain)
		if err != nil {
			a.Log().Error().Err(err).Msg("failed to get maya address")
			user.Release()
			continue
		}
		mayaBalances, err := mayanode.GetBalances(mayaAddress)
		if err != nil {
			a.Log().Error().Err(err).Msg("failed to get mayachain balances")
			user.Release()
			continue
		}
		if mayaBalances.GetCoin(common.BaseAsset()).Amount.IsZero() {
			a.Log().Error().Msg("user has no CACAO balance")
			user.Release()
			continue
		}

		// skip users that don't have L1 balance
		l1Acct, err := user.ChainClients[a.asset.Chain].GetAccount(nil)
		if err != nil {
			a.Log().Error().Err(err).Msg("failed to get L1 account")
			user.Release()
			continue
		}
		if l1Acct.Coins.GetCoin(a.asset).Amount.IsZero() {
			a.Log().Error().Msg("user has no L1 balance")
			user.Release()
			continue
		}

		// TODO: skip users that already have a position in this pool

		// get l1 address to store in state context
		l1Address, err := user.PubKey(common.BASEChain).GetAddress(a.asset.Chain)
		if err != nil {
			a.Log().Error().Err(err).Msg("failed to get L1 address")
			user.Release()
			continue
		}

		// set acquired account and amounts in state context
		a.Log().Info().
			Stringer("address", mayaAddress).
			Stringer("l1_address", l1Address).
			Msg("acquired user")
		a.mayaAddress = mayaAddress
		a.l1Address = l1Address
		a.runeAmount = mayaBalances.GetCoin(common.BaseAsset()).Amount.QuoUint64(5)
		a.l1Amount = l1Acct.Coins.GetCoin(a.asset).Amount.QuoUint64(5)
		a.account = user

		// TODO: above amounts could check for existing pool and use same exchange rate

		break
	}

	// remain pending if no user is available
	return OpResult{
		Continue: a.account != nil,
	}
}

func (a *DualLPActor) depositL1(config *OpConfig) OpResult {
	// get inbound address
	inboundAddr, _, err := mayanode.GetInboundAddress(a.asset.Chain)
	if err != nil {
		a.Log().Error().Err(err).Msg("failed to get inbound address")
		return OpResult{
			Continue: false,
		}
	}

	// create tx out
	memo := fmt.Sprintf("+:%s:%s", a.asset, a.mayaAddress)
	tx := SimTx{
		Chain:     a.asset.Chain,
		ToAddress: inboundAddr,
		Coin:      common.NewCoin(a.asset, a.l1Amount),
		Memo:      memo,
	}

	client := a.account.ChainClients[a.asset.Chain]

	// sign transaction
	signed, err := client.SignTx(tx)
	if err != nil {
		a.Log().Error().Err(err).Msg("failed to sign tx")
		return OpResult{
			Continue: false,
		}
	}

	// broadcast transaction
	txid, err := client.BroadcastTx(signed)
	if err != nil {
		a.Log().Error().Err(err).Msg("failed to broadcast tx")
		return OpResult{
			Continue: false,
		}
	}

	a.Log().Info().Str("txid", txid).Msg("broadcasted L1 add liquidity tx")
	return OpResult{
		Continue: true,
	}
}

func (a *DualLPActor) depositCacao(config *OpConfig) OpResult {
	memo := fmt.Sprintf("+:%s:%s", a.asset, a.l1Address)
	accAddr, err := a.account.PubKey(common.BASEChain).GetThorAddress()
	if err != nil {
		a.Log().Error().Err(err).Msg("failed to get maya address")
		return OpResult{
			Continue: false,
		}
	}
	deposit := types.NewMsgDeposit(
		common.NewCoins(common.NewCoin(common.BaseAsset(), a.runeAmount)),
		memo,
		accAddr,
	)
	txid, err := a.account.Mayachain.Broadcast(deposit)
	if err != nil {
		a.Log().Error().Err(err).Msg("failed to broadcast tx")
		return OpResult{
			Continue: false,
		}
	}

	a.Log().Info().Stringer("txid", txid).Msg("broadcasted CACAO add liquidity tx")
	return OpResult{
		Continue: true,
	}
}

func (a *DualLPActor) verifyLP(config *OpConfig) OpResult {
	lps, err := mayanode.GetLiquidityProviders(a.asset)
	if err != nil {
		a.Log().Error().Err(err).Msg("failed to get liquidity providers")
		return OpResult{
			Continue: false,
		}
	}

	for _, lp := range lps {
		// skip pending lps
		if lp.PendingAsset != "0" || lp.PendingCacao != "0" {
			continue
		}

		// find the matching lp record
		if lp.CacaoAddress == nil || lp.AssetAddress == nil {
			continue
		}

		if common.Address(*lp.CacaoAddress).Equals(a.mayaAddress) &&
			common.Address(*lp.AssetAddress).Equals(a.l1Address) {

			// found the matching lp record
			res := OpResult{
				Finish: true,
			}

			// verify the amounts
			if lp.CacaoDepositValue != a.runeAmount.String() {
				err = fmt.Errorf("mismatch CACAO amount: %s != %s", lp.CacaoDepositValue, a.runeAmount)
				res.Error = multierror.Append(res.Error, err)
			}
			if lp.AssetDepositValue != a.l1Amount.String() {
				err = fmt.Errorf("mismatch L1 amount: %s != %s", lp.AssetDepositValue, a.l1Amount)
				res.Error = multierror.Append(res.Error, err)
			}
			if res.Error != nil {
				a.Log().Error().Err(res.Error).Msg("invalid liquidity provider")
			}

			a.account.Release() // release the user on success
			return res
		}
	}

	// remain pending if no lp is available
	return OpResult{
		Continue: false,
	}
}
