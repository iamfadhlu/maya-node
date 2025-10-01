package features

import (
	"fmt"
	"sync"
	"time"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	. "gitlab.com/mayachain/mayanode/test/simulation/actors/common"
	"gitlab.com/mayachain/mayanode/test/simulation/pkg/mayanode"
	. "gitlab.com/mayachain/mayanode/test/simulation/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////////////
// Consolidate
////////////////////////////////////////////////////////////////////////////////////////

func Consolidate() *Actor {
	a := Actor{
		Name: "Consolidate",
		Ops:  []Op{},
	}

	a.Children = []*Actor{
		NewConsolidateActor(common.BTCAsset, 10),
		NewConsolidateActor(common.ZECAsset, 10),
	}

	return &a
}

////////////////////////////////////////////////////////////////////////////////////////
// ConsolidateActor
////////////////////////////////////////////////////////////////////////////////////////

type ConsolidateActor struct {
	Actor

	asset            common.Asset
	account          *Account
	consolidateCount int64
	donateAmount     cosmos.Uint
	scanHeight       int64
	mu               sync.Mutex
}

func NewConsolidateActor(asset common.Asset, consolidateCount int64) *Actor {
	a := &ConsolidateActor{
		Actor: Actor{
			Name: fmt.Sprintf("feature-Consolidate-%s", asset),
			Ops:  []Op{},
		},
		asset:            asset,
		consolidateCount: consolidateCount,
	}

	a.SetLogger(a.Log().With().Str("asset", asset.String()).Logger())

	// lock a user that has sufficient L1 balance
	a.Ops = append(a.Ops, a.acquireUser)

	// get the current block height
	a.Ops = append(a.Ops, a.getHeight)

	// donate l1 balance with the consolidate count of inbounds
	for i := int64(0); i < a.consolidateCount; i++ {
		a.Ops = append(a.Ops, a.donate)
	}

	// ensure the saver is ejected and release the account
	a.Ops = append(a.Ops, a.verifyConsolidate)

	return &a.Actor
}

////////////////////////////////////////////////////////////////////////////////////////
// Ops
////////////////////////////////////////////////////////////////////////////////////////

func (a *ConsolidateActor) acquireUser(config *OpConfig) OpResult {
	// determine the asset amount
	pool, err := mayanode.GetPool(a.asset)
	if err != nil {
		a.Log().Error().Err(err).Msg("failed to get pool")
		return OpResult{
			Continue: false,
		}
	}

	// donate amount is 1% of the pool asset depth
	a.donateAmount = cosmos.NewUintFromString(pool.BalanceAsset).QuoUint64(100)

	for _, user := range config.UserAccounts {
		a.SetLogger(a.Log().With().Str("user", user.Name()).Logger())

		// skip users already being used
		if !user.Acquire() {
			continue
		}

		// skip users that with insufficient L1 balance
		l1Acct, err := user.ChainClients[a.asset.Chain].GetAccount(nil)
		if err != nil {
			a.Log().Error().Err(err).Msg("failed to get L1 account")
			user.Release()
			continue
		}
		if l1Acct.Coins.GetCoin(a.asset).Amount.LTE(a.donateAmount) {
			a.Log().Error().Msg("user has insufficient L1 balance")
			user.Release()
			continue
		}

		// get l1 address to store in state context
		l1Address, err := user.PubKey(common.BASEChain).GetAddress(a.asset.Chain)
		if err != nil {
			a.Log().Error().Err(err).Msg("failed to get L1 address")
			user.Release()
			continue
		}

		// set acquired account and amounts in state context
		a.Log().Info().Stringer("l1Address", l1Address).Msg("acquired user")
		a.account = user

		break
	}

	// remain pending if no user is available
	return OpResult{
		Continue: a.account != nil,
	}
}

func (a *ConsolidateActor) getHeight(config *OpConfig) OpResult {
	// get the current block height
	block, err := mayanode.GetBlock(0)
	if err != nil {
		a.Log().Error().Err(err).Msg("failed to get block")
		return OpResult{
			Continue: false,
		}
	}

	a.Log().Info().Int64("height", block.Header.Height).Msg("got block height")
	a.scanHeight = block.Header.Height

	return OpResult{
		Continue: true,
	}
}

func (a *ConsolidateActor) donate(config *OpConfig) OpResult {
	a.Log().Info().Stringer("asset", a.asset).Stringer("donateAmount", a.donateAmount).Stringer("account-from", a.account.Address(common.ZECChain)).Msg("starting DONATE")
	memo := fmt.Sprintf("DONATE:%s", a.asset)
	client := a.account.ChainClients[a.asset.Chain]
	if a.asset.Chain.Equals(common.ZECChain) {
		a.mu.Lock()
		defer a.mu.Unlock()
	}
	txid, err := DepositL1(a.Log(), client, a.asset, memo, a.donateAmount)
	if a.asset.Chain.Equals(common.ZECChain) {
		// wait for the tx to be mined
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		a.Log().Error().Err(err).Msg("failed to deposit L1")
		return OpResult{
			Continue: false,
		}
	}

	a.Log().Info().Str("txid", txid).Msg("broadcasted donate")
	return OpResult{
		Continue: true,
	}
}

func (a *ConsolidateActor) verifyConsolidate(config *OpConfig) OpResult {
	type FilterBlockResponse struct {
		Txs []struct {
			Tx struct {
				Body struct {
					Messages []struct {
						Type   string `json:"@type"`
						ObsTxs []struct {
							Tx struct {
								Chain string `json:"chain"`
								Memo  string `json:"memo"`
							} `json:"tx"`
						} `json:"txs,omitempty"`
					} `json:"messages"`
				} `json:"body"`
			} `json:"tx"`
		} `json:"txs"`
	}

	// scan blocks until we find the consolidate tx observation
	foundConsolidate := false
	for {
		a.Log().Info().Int64("height", a.scanHeight).Msg("scanning block for consolidate")

		url := fmt.Sprintf("%s/mayachain/block?height=%d", mayanode.BaseURL(), a.scanHeight)
		var block FilterBlockResponse
		err := mayanode.Get(url, &block)
		if err != nil {
			return OpResult{
				Continue: false,
			}
		}
		a.scanHeight++

		// scan for a consolidate inbound
		for _, tx := range block.Txs {
			for _, msg := range tx.Tx.Body.Messages {
				if msg.Type == "/types.MsgObservedTxIn" {
					for _, observedTxWrapper := range msg.ObsTxs {
						actualObservedTx := observedTxWrapper.Tx
						if actualObservedTx.Memo == "consolidate" && actualObservedTx.Chain == a.asset.Chain.String() {
							a.Log().Info().Msg("Consolidate transaction found!")
							foundConsolidate = true
							break
						}
					}
				}
				if foundConsolidate {
					break
				}
			}
			if foundConsolidate {
				break
			}
		}
		if foundConsolidate {
			break
		}
	}

	if !foundConsolidate {
		a.Log().Error().Msg("failed to find consolidate tx")
		return OpResult{
			Continue: false,
		}
	}

	a.account.Release() // release the user
	return OpResult{
		Continue: true,
	}
}
