package mayachain

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/rs/zerolog/log"
	abci "github.com/tendermint/tendermint/abci/types"
	tmhttp "github.com/tendermint/tendermint/rpc/client/http"

	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"

	"gitlab.com/mayachain/mayanode/bifrost/tss/go-tss/conversion"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/config"
	"gitlab.com/mayachain/mayanode/constants"
	openapi "gitlab.com/mayachain/mayanode/openapi/gen"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
	q "gitlab.com/mayachain/mayanode/x/mayachain/query"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

var (
	initManager   = func(mgr *Mgrs, ctx cosmos.Context) {}
	optionalQuery = func(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
		return nil, cosmos.ErrUnknownRequest(
			fmt.Sprintf("unknown mayachain query endpoint: %s", path[0]),
		)
	}
	tendermintClient   *tmhttp.HTTP
	initTendermintOnce = sync.Once{}
)

func initTendermint() {
	// get tendermint port from config
	portSplit := strings.Split(config.GetThornode().Tendermint.RPC.ListenAddress, ":")
	port := portSplit[len(portSplit)-1]

	// setup tendermint client
	var err error
	tendermintClient, err = tmhttp.New(fmt.Sprintf("tcp://localhost:%s", port), "/websocket")
	if err != nil {
		log.Fatal().Err(err).Msg("fail to create tendermint client")
	}
}

// NewQuerier is the module level router for state queries
func NewQuerier(mgr *Mgrs, kbs cosmos.KeybaseStore) cosmos.Querier {
	return func(ctx cosmos.Context, path []string, req abci.RequestQuery) (res []byte, err error) {
		initManager(mgr, ctx) // NOOP except regtest

		defer telemetry.MeasureSince(time.Now(), path[0])
		switch path[0] {
		case q.QueryPool.Key:
			return queryPool(ctx, path[1:], req, mgr)
		case q.QueryPools.Key:
			return queryPools(ctx, req, mgr)
		case q.QuerySavers.Key:
			return queryLiquidityProviders(ctx, path[1:], req, mgr, true)
		case q.QuerySaver.Key:
			return queryLiquidityProvider(ctx, path[1:], req, mgr, true)
		case q.QueryLiquidityProviders.Key:
			return queryLiquidityProviders(ctx, path[1:], req, mgr, false)
		case q.QueryLiquidityProvider.Key:
			return queryLiquidityProvider(ctx, path[1:], req, mgr, false)
		case q.QueryTradeUnit.Key:
			return queryTradeUnit(ctx, path[1:], mgr)
		case q.QueryTradeUnits.Key:
			return queryTradeUnits(ctx, path[1:], mgr)
		case q.QueryTradeAccount.Key:
			return queryTradeAccount(ctx, path[1:], mgr)
		case q.QueryTradeAccounts.Key:
			return queryTradeAccounts(ctx, path[1:], mgr)
		case q.QueryTxStages.Key:
			return queryTxStages(ctx, path[1:], req, mgr)
		case q.QueryTxStatus.Key:
			return queryTxStatus(ctx, path[1:], req, mgr)
		case q.QueryTxVoter.Key:
			return queryTxVoters(ctx, path[1:], req, mgr)
		case q.QueryTxVoterOld.Key:
			return queryTxVotersOld(ctx, path[1:], req, mgr)
		case q.QueryTxInVoter.Key:
			return queryTxInVoter(ctx, path[1:], req, mgr)
		case q.QueryTxOutVoter.Key:
			return queryTxOutVoter(ctx, path[1:], req, mgr)
		case q.QueryTx.Key:
			return queryTx(ctx, path[1:], req, mgr)
		case q.QueryKeysignArray.Key:
			return queryKeysign(ctx, kbs, path[1:], req, mgr)
		case q.QueryKeysignArrayPubkey.Key:
			return queryKeysign(ctx, kbs, path[1:], req, mgr)
		case q.QueryKeygensPubkey.Key:
			return queryKeygen(ctx, kbs, path[1:], req, mgr)
		case q.QueryQueue.Key:
			return queryQueue(ctx, path[1:], req, mgr)
		case q.QueryHeights.Key:
			return queryLastBlockHeights(ctx, path[1:], req, mgr)
		case q.QueryChainHeights.Key:
			return queryLastBlockHeights(ctx, path[1:], req, mgr)
		case q.QueryNode.Key:
			return queryNode(ctx, path[1:], req, mgr)
		case q.QueryNodes.Key:
			return queryNodes(ctx, path[1:], req, mgr)
		case q.QueryInboundAddresses.Key:
			return queryInboundAddresses(ctx, path[1:], req, mgr)
		case q.QueryNetwork.Key:
			return queryNetwork(ctx, mgr)
		case q.QueryPOL.Key:
			return queryPOL(ctx, mgr)
		case q.QueryBalanceModule.Key:
			return queryBalanceModule(ctx, path[1:], mgr)
		case q.QueryVaultsAsgard.Key:
			return queryAsgardVaults(ctx, mgr)
		case q.QueryVaultsYggdrasil.Key:
			return queryYggdrasilVaults(ctx, mgr)
		case q.QueryVault.Key:
			return queryVault(ctx, path[1:], mgr)
		case q.QueryVaultPubkeys.Key:
			return queryVaultsPubkeys(ctx, mgr)
		case q.QueryConstantValues.Key:
			return queryConstantValues(ctx, path[1:], req, mgr)
		case q.QueryVersion.Key:
			return queryVersion(ctx, path[1:], req, mgr)
		case q.QueryMimirValues.Key:
			return queryMimirValues(ctx, path[1:], req, mgr)
		case q.QueryMimirWithKey.Key:
			return queryMimirWithKey(ctx, path[1:], req, mgr)
		case q.QueryMimirAdminValues.Key:
			return queryMimirAdminValues(ctx, path[1:], req, mgr)
		case q.QueryMimirNodesAllValues.Key:
			return queryMimirNodesAllValues(ctx, path[1:], req, mgr)
		case q.QueryMimirNodesValues.Key:
			return queryMimirNodesValues(ctx, path[1:], req, mgr)
		case q.QueryMimirNodeValues.Key:
			return queryMimirNodeValues(ctx, path[1:], req, mgr)
		case q.QueryBan.Key:
			return queryBan(ctx, path[1:], req, mgr)
		case q.QueryRagnarok.Key:
			return queryRagnarok(ctx, mgr)
		case q.QueryCACAOPool.Key:
			return queryCACAOPool(ctx, mgr)
		case q.QueryCACAOProvider.Key:
			return queryCACAOProvider(ctx, path[1:], mgr)
		case q.QueryCACAOProviders.Key:
			return queryCACAOProviders(ctx, mgr)
		case q.QueryPendingOutbound.Key:
			return queryPendingOutbound(ctx, mgr)
		case q.QueryScheduledOutbound.Key:
			return queryScheduledOutbound(ctx, mgr)
		case q.QuerySwapQueue.Key:
			return querySwapQueue(ctx, mgr)
		case q.QueryStreamingSwap.Key:
			return queryStreamingSwap(ctx, path[1:], mgr)
		case q.QueryStreamingSwaps.Key:
			return queryStreamingSwaps(ctx, mgr)
		case q.QueryTssKeygenMetrics.Key:
			return queryTssKeygenMetric(ctx, path[1:], req, mgr)
		case q.QueryTssMetrics.Key:
			return queryTssMetric(ctx, path[1:], req, mgr)
		case q.QueryMAYAName.Key:
			return queryMAYAName(ctx, path[1:], req, mgr)
		case q.QueryLiquidityAuctionTier.Key:
			return queryLiquidityAuctionTier(ctx, path[1:], req, mgr)
		case q.QueryQuoteSwap.Key:
			return queryQuoteSwap(ctx, req, mgr)
		case q.QueryQuoteSaverDeposit.Key:
			return queryQuoteSaverDeposit(ctx, path[1:], req, mgr)
		case q.QueryQuoteSaverWithdraw.Key:
			return queryQuoteSaverWithdraw(ctx, path[1:], req, mgr)
		case q.QueryInvariants.Key:
			return queryInvariants(ctx, mgr)
		case q.QueryInvariant.Key:
			return queryInvariant(ctx, path[1:], mgr)
		case q.QueryBlock.Key:
			return queryBlock(ctx, mgr)
		default:
			return optionalQuery(ctx, path, req, mgr)
		}
	}
}

func getPeerIDFromPubKey(pubkey common.PubKey) string {
	peerID, err := conversion.GetPeerIDFromPubKey(pubkey.String())
	if err != nil {
		// Don't break the entire endpoint if something goes wrong with the Peer ID derivation.
		return err.Error()
	}

	return peerID.String()
}

func jsonify(ctx cosmos.Context, r any) ([]byte, error) {
	res, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		ctx.Logger().Error("fail to marshal response to json", "error", err)
		return nil, fmt.Errorf("fail to marshal response to json: %w", err)
	}
	return res, nil
}

func queryRagnarok(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	ragnarokInProgress := mgr.Keeper().RagnarokInProgress(ctx)
	return jsonify(ctx, ragnarokInProgress)
}

func queryBalanceModule(ctx cosmos.Context, path []string, mgr *Mgrs) ([]byte, error) {
	moduleName := path[0]
	if len(moduleName) == 0 {
		moduleName = AsgardName
	}

	modAddr := mgr.Keeper().GetModuleAccAddress(moduleName)
	bal := mgr.Keeper().GetBalance(ctx, modAddr)
	balance := struct {
		Name    string            `json:"name"`
		Address cosmos.AccAddress `json:"address"`
		Coins   sdk.Coins         `json:"coins"`
	}{
		Name:    moduleName,
		Address: modAddr,
		Coins:   bal,
	}
	return jsonify(ctx, balance)
}

func queryMAYAName(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	name, err := mgr.Keeper().GetMAYAName(ctx, path[0])
	if err != nil {
		return nil, ErrInternal(err, "fail to fetch MAYAName")
	}

	affRune := cosmos.ZeroUint()
	affCol, err := mgr.Keeper().GetAffiliateCollector(ctx, name.Owner)
	if err == nil {
		affRune = affCol.CacaoAmount
	}

	// convert aliases to openapi types
	aliases := []openapi.MayanameAlias{}
	for _, alias := range name.Aliases {
		aliases = append(aliases, openapi.MayanameAlias{
			Chain:   wrapString(alias.Chain.String()),
			Address: wrapString(alias.Address.String()),
		})
	}

	// convert subaffiliates to openapi types
	subaffiliates := []openapi.MayanameSubaffiliate{}
	for _, subaff := range name.Subaffiliates {
		subaffiliates = append(subaffiliates, openapi.MayanameSubaffiliate{
			Name: wrapString(subaff.Name),
			Bps:  wrapInt64(int64(subaff.Bps.Uint64())),
		})
	}

	threshold := getPreferredAssetSwapThreshold(ctx, mgr, name.PreferredAsset)

	resp := openapi.Mayaname{
		Name:                             wrapString(name.Name),
		ExpireBlockHeight:                wrapInt64(name.ExpireBlockHeight),
		Owner:                            wrapString(name.Owner.String()),
		PreferredAsset:                   name.PreferredAsset.String(),
		PreferredAssetSwapThresholdCacao: wrapString(threshold.String()),
		AffiliateCollectorCacao:          wrapString(affRune.String()),
		Aliases:                          aliases,
		AffiliateBps:                     wrapInt64(int64(name.GetAffiliateBps().BigInt().Uint64())),
		Subaffiliates:                    subaffiliates,
	}

	return jsonify(ctx, resp)
}

func queryVault(ctx cosmos.Context, path []string, mgr *Mgrs) ([]byte, error) {
	if len(path) < 1 {
		return nil, errors.New("not enough parameters")
	}
	pubkey, err := common.NewPubKey(path[0])
	if err != nil {
		return nil, fmt.Errorf("%s is invalid pubkey", path[0])
	}
	v, err := mgr.Keeper().GetVault(ctx, pubkey)
	if err != nil {
		return nil, fmt.Errorf("fail to get vault with pubkey(%s),err:%w", pubkey, err)
	}
	if v.IsEmpty() {
		return nil, errors.New("vault not found")
	}

	resp := openapi.Vault{
		BlockHeight:           wrapInt64(v.BlockHeight),
		PubKey:                wrapString(v.PubKey.String()),
		Coins:                 castCoins(v.Coins...),
		Type:                  wrapString(v.Type.String()),
		Status:                v.Status.String(),
		StatusSince:           wrapInt64(v.StatusSince),
		Membership:            v.Membership,
		Chains:                v.Chains,
		InboundTxCount:        wrapInt64(v.InboundTxCount),
		OutboundTxCount:       wrapInt64(v.OutboundTxCount),
		PendingTxBlockHeights: v.PendingTxBlockHeights,
		Routers:               castVaultRouters(v.Routers),
		Frozen:                v.Frozen,
		Addresses:             getVaultChainAddresses(ctx, v),
	}
	return jsonify(ctx, resp)
}

func queryAsgardVaults(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	vaults, err := mgr.Keeper().GetAsgardVaults(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get asgard vaults: %w", err)
	}

	var vaultsWithFunds []openapi.Vault
	for _, vault := range vaults {
		if vault.Status == InactiveVault {
			continue
		}
		if !vault.IsAsgard() {
			continue
		}
		// Being in a RetiringVault blocks a node from unbonding, so display them even if having no funds.
		if vault.HasFunds() || vault.Status == ActiveVault || vault.Status == RetiringVault {
			vaultsWithFunds = append(vaultsWithFunds, openapi.Vault{
				BlockHeight:           wrapInt64(vault.BlockHeight),
				PubKey:                wrapString(vault.PubKey.String()),
				Coins:                 castCoins(vault.Coins...),
				Type:                  wrapString(vault.Type.String()),
				Status:                vault.Status.String(),
				StatusSince:           wrapInt64(vault.StatusSince),
				Membership:            vault.Membership,
				Chains:                vault.Chains,
				InboundTxCount:        wrapInt64(vault.InboundTxCount),
				OutboundTxCount:       wrapInt64(vault.OutboundTxCount),
				PendingTxBlockHeights: vault.PendingTxBlockHeights,
				Routers:               castVaultRouters(vault.Routers),
				Frozen:                vault.Frozen,
				Addresses:             getVaultChainAddresses(ctx, vault),
			})
		}
	}

	return jsonify(ctx, vaultsWithFunds)
}

func getVaultChainAddresses(ctx cosmos.Context, vault Vault) []openapi.VaultAddress {
	var result []openapi.VaultAddress
	allChains := append(vault.GetChains(), common.BASEChain)
	for _, c := range allChains.Distinct() {
		addr, err := vault.GetAddress(c)
		if err != nil {
			ctx.Logger().Error("fail to get address for %s:%w", c.String(), err)
			continue
		}
		result = append(result,
			openapi.VaultAddress{
				Chain:   c.String(),
				Address: addr.String(),
			})
	}
	return result
}

func queryYggdrasilVaults(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	vaults := make(Vaults, 0)
	iter := mgr.Keeper().GetVaultIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var vault Vault
		if err := mgr.Keeper().Cdc().Unmarshal(iter.Value(), &vault); err != nil {
			ctx.Logger().Error("fail to unmarshal yggdrasil", "error", err)
			return nil, fmt.Errorf("fail to unmarshal yggdrasil: %w", err)
		}
		if vault.IsYggdrasil() && vault.HasFunds() {
			vaults = append(vaults, vault)
		}
	}

	respVaults := make([]openapi.YggdrasilVault, len(vaults))
	for i, vault := range vaults {
		totalValue := cosmos.ZeroUint()

		// find the bond of this node account
		na, err := mgr.Keeper().GetNodeAccountByPubKey(ctx, vault.PubKey)
		if err != nil {
			ctx.Logger().Error("fail to get node account by pubkey", "error", err)
			continue
		}

		// calculate the total value of this yggdrasil vault
		for _, coin := range vault.Coins {
			if coin.Asset.IsBase() {
				totalValue = totalValue.Add(coin.Amount)
			} else {
				pool, err := mgr.Keeper().GetPool(ctx, coin.Asset)
				if err != nil {
					ctx.Logger().Error("fail to get pool", "error", err)
					continue
				}
				totalValue = totalValue.Add(pool.AssetValueInRune(coin.Amount))
			}
		}

		respVaults[i] = openapi.YggdrasilVault{
			BlockHeight:           wrapInt64(vault.BlockHeight),
			PubKey:                wrapString(vault.PubKey.String()),
			Coins:                 castCoins(vault.Coins...),
			Type:                  wrapString(vault.Type.String()),
			StatusSince:           wrapInt64(vault.StatusSince),
			Membership:            vault.Membership,
			Chains:                vault.Chains,
			InboundTxCount:        wrapInt64(vault.InboundTxCount),
			OutboundTxCount:       wrapInt64(vault.OutboundTxCount),
			PendingTxBlockHeights: vault.PendingTxBlockHeights,
			Routers:               castVaultRouters(vault.Routers),
			Status:                na.Status.String(),
			Bond:                  na.Bond.String(),
			TotalValue:            totalValue.String(),
			Addresses:             getVaultChainAddresses(ctx, vault),
		}
	}

	return jsonify(ctx, respVaults)
}

func queryVaultsPubkeys(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	var resp openapi.VaultPubkeysResponse
	resp.Asgard = make([]openapi.VaultInfo, 0)
	resp.Yggdrasil = make([]openapi.VaultInfo, 0)
	resp.Inactive = make([]openapi.VaultInfo, 0)
	iter := mgr.Keeper().GetVaultIterator(ctx)

	active, err := mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return nil, err
	}
	cutOffAge := ctx.BlockHeight() - config.GetThornode().VaultPubkeysCutoffBlocks
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var vault Vault
		if err := mgr.Keeper().Cdc().Unmarshal(iter.Value(), &vault); err != nil {
			ctx.Logger().Error("fail to unmarshal vault", "error", err)
			return nil, fmt.Errorf("fail to unmarshal vault: %w", err)
		}
		if vault.IsYggdrasil() { // TODO remove ygg on hard fork
			na, err := mgr.Keeper().GetNodeAccountByPubKey(ctx, vault.PubKey)
			if err != nil {
				ctx.Logger().Error("fail to unmarshal vault", "error", err)
				return nil, fmt.Errorf("fail to unmarshal vault: %w", err)
			}
			if !na.Bond.IsZero() {
				resp.Yggdrasil = append(resp.Yggdrasil, openapi.VaultInfo{
					PubKey:  vault.PubKey.String(),
					Routers: castVaultRouters(vault.Routers),
				})
			}
		} else if vault.IsAsgard() {
			switch vault.Status {
			case ActiveVault, RetiringVault:
				resp.Asgard = append(resp.Asgard, openapi.VaultInfo{
					PubKey:  vault.PubKey.String(),
					Routers: castVaultRouters(vault.Routers),
				})
			case InactiveVault:
				// skip inactive vaults that have never received an inbound
				if vault.InboundTxCount == 0 {
					continue
				}

				// skip inactive vaults older than the cutoff age
				if vault.BlockHeight < cutOffAge {
					continue
				}

				activeMembers, err := vault.GetMembers(active.GetNodeAddresses())
				if err != nil {
					ctx.Logger().Error("fail to get active members of vault", "error", err)
					continue
				}
				allMembers := vault.Membership
				if HasSuperMajority(len(activeMembers), len(allMembers)) {
					resp.Inactive = append(resp.Inactive, openapi.VaultInfo{
						PubKey:  vault.PubKey.String(),
						Routers: castVaultRouters(vault.Routers),
					})
				}
			}
		}
	}
	return jsonify(ctx, resp)
}

func queryCACAOPool(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	// gather pol data
	pol, err := mgr.Keeper().GetPOL(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get POL: %w", err)
	}
	polValue, err := polPoolValue(ctx, mgr)
	if err != nil {
		return nil, fmt.Errorf("fail to fetch POL value: %w", err)
	}
	pnl := pol.PnL(polValue)

	// gather cacaopool data
	cacaoPool, err := mgr.Keeper().GetCACAOPool(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get CACAO pool: %w", err)
	}

	// calculate pending units
	cacaoPoolValue, err := cacaoPoolValue(ctx, mgr)
	if err != nil {
		return nil, fmt.Errorf("fail to get cacao pool value: %w", err)
	}
	pendingCacao := mgr.Keeper().GetRuneBalanceOfModule(ctx, CACAOPoolName)
	pendingUnits := common.GetSafeShare(pendingCacao, cacaoPoolValue, cacaoPool.TotalUnits())

	// calculate provider shares
	providerValue := common.GetSafeShare(cacaoPool.PoolUnits, cacaoPool.TotalUnits(), cacaoPoolValue)
	providerPnl := sdk.NewIntFromBigInt(providerValue.BigInt()).Sub(cacaoPool.CurrentDeposit())

	// calculate reserve shares
	reserveValue := common.GetSafeShare(cacaoPool.ReserveUnits, cacaoPool.TotalUnits(), cacaoPoolValue)
	reserveCurrentDeposit := pol.CurrentDeposit().
		Sub(cacaoPool.CurrentDeposit()).
		Add(cosmos.NewIntFromBigInt(pendingCacao.BigInt()))
	reservePnl := sdk.NewIntFromBigInt(reserveValue.BigInt()).Sub(reserveCurrentDeposit)

	result := openapi.CACAOPoolResponse{
		Pol: openapi.POL{
			CacaoDeposited: pol.CacaoDeposited.String(),
			CacaoWithdrawn: pol.CacaoWithdrawn.String(),
			Value:          polValue.String(),
			Pnl:            pnl.String(),
			CurrentDeposit: pol.CurrentDeposit().String(),
		},
		Providers: openapi.CACAOPoolResponseProviders{
			Units:          cacaoPool.PoolUnits.String(),
			PendingUnits:   pendingUnits.String(),
			PendingCacao:   pendingCacao.String(),
			Value:          providerValue.String(),
			Pnl:            providerPnl.String(),
			CurrentDeposit: cacaoPool.CurrentDeposit().String(),
		},
		Reserve: openapi.CACAOPoolResponseReserve{
			Units:          cacaoPool.ReserveUnits.String(),
			Value:          reserveValue.String(),
			Pnl:            reservePnl.String(),
			CurrentDeposit: reserveCurrentDeposit.String(),
		},
	}

	return jsonify(ctx, result)
}

// queryCACAOProvider
func queryCACAOProvider(ctx cosmos.Context, path []string, mgr *Mgrs) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("address not provided")
	}
	addr, err := cosmos.AccAddressFromBech32(path[0])
	if err != nil {
		return nil, errors.New("unable to decode address")
	}
	rp, err := mgr.Keeper().GetCACAOProvider(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("unable to GetCACAOProvider: %s", err)
	}

	// get cacaopool value to determine current value and pnl
	cacaoPool, err := mgr.Keeper().GetCACAOPool(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get CACAO pool: %w", err)
	}
	cacaoPoolValue, err := cacaoPoolValue(ctx, mgr)
	if err != nil {
		return nil, fmt.Errorf("fail to get cacao pool value: %w", err)
	}
	providerValue := common.GetSafeShare(rp.Units, cacaoPool.TotalUnits(), cacaoPoolValue)
	providerPnl := providerValue.BigInt()
	providerPnl.Sub(providerPnl, rp.DepositAmount.BigInt())
	providerPnl.Add(providerPnl, rp.WithdrawAmount.BigInt())

	result := openapi.CACAOProvider{
		CacaoAddress:       rp.CacaoAddress.String(),
		Units:              rp.Units.String(),
		Value:              providerValue.String(),
		Pnl:                providerPnl.String(),
		DepositAmount:      rp.DepositAmount.String(),
		WithdrawAmount:     rp.WithdrawAmount.String(),
		LastDepositHeight:  rp.LastDepositHeight,
		LastWithdrawHeight: rp.LastWithdrawHeight,
	}
	return jsonify(ctx, result)
}

// queryCACAOProviders
func queryCACAOProviders(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	// get cacaopool value to determine current value and pnl
	cacaoPool, err := mgr.Keeper().GetCACAOPool(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get CACAO pool: %w", err)
	}
	cacaoPoolValue, err := cacaoPoolValue(ctx, mgr)
	if err != nil {
		return nil, fmt.Errorf("fail to get cacao pool value: %w", err)
	}

	var cacaoProviders []openapi.CACAOProvider
	iterator := mgr.Keeper().GetCACAOProviderIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var rp types.CACAOProvider
		mgr.Keeper().Cdc().MustUnmarshal(iterator.Value(), &rp)

		providerValue := common.GetSafeShare(rp.Units, cacaoPool.TotalUnits(), cacaoPoolValue)
		providerPnl := providerValue.BigInt()
		providerPnl.Sub(providerPnl, rp.DepositAmount.BigInt())
		providerPnl.Add(providerPnl, rp.WithdrawAmount.BigInt())

		cacaoProviders = append(cacaoProviders, openapi.CACAOProvider{
			CacaoAddress:       rp.CacaoAddress.String(),
			Units:              rp.Units.String(),
			Value:              providerValue.String(),
			Pnl:                providerPnl.String(),
			DepositAmount:      rp.DepositAmount.String(),
			WithdrawAmount:     rp.WithdrawAmount.String(),
			LastDepositHeight:  rp.LastDepositHeight,
			LastWithdrawHeight: rp.LastWithdrawHeight,
		})
	}
	return jsonify(ctx, cacaoProviders)
}

func queryNetwork(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	data, err := mgr.Keeper().GetNetwork(ctx)
	if err != nil {
		ctx.Logger().Error("fail to get vault", "error", err)
		return nil, fmt.Errorf("fail to get vault: %w", err)
	}
	targetOutboundFeeSurplus := mgr.Keeper().GetConfigInt64(ctx, constants.TargetOutboundFeeSurplusRune)
	maxMultiplierBasisPoints := mgr.Keeper().GetConfigInt64(ctx, constants.MaxOutboundFeeMultiplierBasisPoints)
	minMultiplierBasisPoints := mgr.Keeper().GetConfigInt64(ctx, constants.MinOutboundFeeMultiplierBasisPoints)
	outboundFeeMultiplier := mgr.gasMgr.CalcOutboundFeeMultiplier(ctx, cosmos.NewUint(uint64(targetOutboundFeeSurplus)), cosmos.NewUint(data.OutboundGasSpentCacao), cosmos.NewUint(data.OutboundGasWithheldCacao), cosmos.NewUint(uint64(maxMultiplierBasisPoints)), cosmos.NewUint(uint64(minMultiplierBasisPoints)))

	result := openapi.NetworkResponse{
		BondRewardCacao:       data.BondRewardRune.String(),
		TotalBondUnits:        data.TotalBondUnits.String(),
		TotalReserve:          mgr.Keeper().GetRuneBalanceOfModule(ctx, ReserveName).String(),
		TotalAsgard:           mgr.Keeper().GetRuneBalanceOfModule(ctx, AsgardName).String(),
		GasSpentCacao:         cosmos.NewUint(data.OutboundGasSpentCacao).String(),
		GasWithheldCacao:      cosmos.NewUint(data.OutboundGasWithheldCacao).String(),
		OutboundFeeMultiplier: wrapString(outboundFeeMultiplier.String()),
	}

	return jsonify(ctx, result)
}

func queryPOL(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	data, err := mgr.Keeper().GetPOL(ctx)
	if err != nil {
		ctx.Logger().Error("fail to get POL", "error", err)
		return nil, fmt.Errorf("fail to get POL: %w", err)
	}
	polValue, err := polPoolValue(ctx, mgr)
	if err != nil {
		ctx.Logger().Error("fail to fetch POL value", "error", err)
		return nil, fmt.Errorf("fail to fetch POL value: %w", err)
	}
	pnl := data.PnL(polValue)
	result := openapi.POLResponse{
		CacaoDeposited: data.CacaoDeposited.String(),
		CacaoWithdrawn: data.CacaoWithdrawn.String(),
		Value:          polValue.String(),
		Pnl:            pnl.String(),
		CurrentDeposit: data.CurrentDeposit().String(),
	}

	res, err := json.MarshalIndent(result, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal protocol owned liquidity data to json", "error", err)
		return nil, fmt.Errorf("fail to marshal response to json: %w", err)
	}
	return res, nil
}

func queryInboundAddresses(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	active, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		ctx.Logger().Error("fail to get active vaults", "error", err)
		return nil, fmt.Errorf("fail to get active vaults: %w", err)
	}

	var resp []openapi.InboundAddress
	constAccessor := mgr.GetConstants()
	signingTransactionPeriod := constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
	if mgr.Keeper() == nil {
		ctx.Logger().Error("keeper is nil, can't fulfill query")
		return nil, errors.New("keeper is nil, can't fulfill query")
	}
	// select vault that is most secure
	vault := mgr.Keeper().GetMostSecure(ctx, active, signingTransactionPeriod)

	chains := vault.GetChains()

	if len(chains) == 0 {
		chains = common.Chains{common.BaseAsset().Chain}
	}

	isGlobalTradingHalt := isGlobalTradingHalted(ctx, mgr)

	for _, chain := range chains {
		// tx send to mayachain doesn't need an address , thus here skip it
		if chain == common.BASEChain {
			continue
		}

		isChainTradingPaused := isChainTradingHalted(ctx, mgr, chain)
		isChainLpPaused := isLPPaused(ctx, chain, mgr)

		var vaultAddress common.Address
		vaultAddress, err = vault.GetAddress(chain)
		if err != nil {
			ctx.Logger().Error("fail to get address for chain", "error", err)
			return nil, fmt.Errorf("fail to get address for chain: %w", err)
		}
		cc := vault.GetContract(chain)
		gasRate := mgr.GasMgr().GetGasRate(ctx, chain)
		var networkFeeInfo NetworkFee
		networkFeeInfo, err = mgr.GasMgr().GetNetworkFee(ctx, chain)
		if err != nil {
			ctx.Logger().Error("fail to get network fee info", "error", err)
			return nil, fmt.Errorf("fail to get network fee info: %w", err)
		}

		// because THORNode is using 1e8, while GWei in ETH is in 1e9, thus the minimum THORNode can represent is 10Gwei
		// here convert the gas rate to Gwei , so api user don't need to convert it , make it easier for people to understand
		if chain.IsEVM() && !chain.Equals(common.ARBChain) {
			gasRate = gasRate.MulUint64(10)
		}
		// Retrieve the value the network charges for outbound fees for this chain
		// Note: If calculating the outbound fee of a non-gas asset outbound, the returned value must be converted to CACAO,
		// then to the non-gas asset using the pool depths. That will be the value deducted from the non-gas asset outbound amount
		outboundFee := mgr.GasMgr().GetFee(ctx, chain, chain.GetGasAsset())

		addr := openapi.InboundAddress{
			Chain:                wrapString(chain.String()),
			PubKey:               wrapString(vault.PubKey.String()),
			Address:              wrapString(vaultAddress.String()),
			Router:               wrapString(cc.Router.String()),
			Halted:               isGlobalTradingHalt || isChainTradingPaused,
			GlobalTradingPaused:  &isGlobalTradingHalt,
			ChainTradingPaused:   &isChainTradingPaused,
			ChainLpActionsPaused: &isChainLpPaused,
			GasRate:              wrapString(gasRate.String()),
			GasRateUnits:         wrapString(chain.GetGasUnits()),
			OutboundTxSize:       wrapString(cosmos.NewUint(networkFeeInfo.TransactionSize).String()),
			OutboundFee:          wrapString(outboundFee.String()),
			DustThreshold:        wrapString(chain.DustThreshold().String()),
		}

		resp = append(resp, addr)
	}

	res, err := json.MarshalIndent(resp, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal current pool address to json", "error", err)
		return nil, fmt.Errorf("fail to marshal current pool address to json: %w", err)
	}

	return res, nil
}

func populateBondAndBondedPools(ctx cosmos.Context, mgr *Mgrs, result *openapi.Node, nodeAddress cosmos.AccAddress) (cosmos.Uint, error) {
	var providers []openapi.NodeBondProvider
	bond := cosmos.ZeroUint()

	bp, err := mgr.Keeper().GetBondProviders(ctx, nodeAddress)
	if err != nil {
		return cosmos.ZeroUint(), fmt.Errorf("fail to get bond providers: %w", err)
	}

	// Leave this nil (null rather than []) if the source is nil.
	if bp.Providers != nil {
		providers = make([]openapi.NodeBondProvider, len(bp.Providers))
		for i, provider := range bp.Providers {
			providers[i].BondAddress = provider.BondAddress.String()
			providers[i].Bonded = provider.Bonded
			if provider.Reward != nil {
				providers[i].Reward = provider.Reward.String()
			} else {
				providers[i].Reward = "0"
			}

			providers[i].Pools = make(map[string]string)
			lpBond, lps, err := mgr.Keeper().GetLPLiquidityBond(ctx, common.Address(provider.BondAddress.String()), nodeAddress)
			if err != nil {
				return cosmos.ZeroUint(), fmt.Errorf("failed to find bond nodes for provider: %w", err)
			}

			// Set bond in the result
			bond = bond.Add(lpBond)

			for _, lp := range lps {
				assetStr := lp.Asset.String()
				unitsStr := lp.Units.String()
				providers[i].Pools[assetStr] = unitsStr
			}
		}
	}

	// Set bonded pools
	result.BondProviders = openapi.NodeBondProviders{
		// Since redundant, leave out the node address
		NodeAddress:     wrapString(nodeAddress.String()),
		NodeOperatorFee: bp.NodeOperatorFee.String(),
		Providers:       providers,
	}

	// Set bond
	result.Bond = bond.String()

	return bond, nil
}

// queryNode return the Node information related to the request node address
// /mayachain/node/{nodeaddress}
func queryNode(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("node address not provided")
	}
	nodeAddress := path[0]
	addr, err := cosmos.AccAddressFromBech32(nodeAddress)
	if err != nil {
		return nil, cosmos.ErrUnknownRequest("invalid account address")
	}

	nodeAcc, err := mgr.Keeper().GetNodeAccount(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("fail to get node accounts: %w", err)
	}

	slashPts, err := mgr.Keeper().GetNodeAccountSlashPoints(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("fail to get node slash points: %w", err)
	}
	jail, err := mgr.Keeper().GetNodeAccountJail(ctx, nodeAcc.NodeAddress)
	if err != nil {
		return nil, fmt.Errorf("fail to get node jail: %w", err)
	}

	result := openapi.Node{
		NodeAddress: nodeAcc.NodeAddress.String(),
		Status:      nodeAcc.Status.String(),
		PubKeySet: openapi.NodePubKeySet{
			Secp256k1: wrapString(nodeAcc.PubKeySet.Secp256k1.String()),
			Ed25519:   wrapString(nodeAcc.PubKeySet.Ed25519.String()),
		},
		ValidatorConsPubKey: nodeAcc.ValidatorConsPubKey,
		ActiveBlockHeight:   nodeAcc.ActiveBlockHeight,
		StatusSince:         nodeAcc.StatusSince,
		BondAddress:         nodeAcc.BondAddress.String(),
		Bond:                "",
		SignerMembership:    nodeAcc.GetSignerMembership().Strings(),
		RequestedToLeave:    nodeAcc.RequestedToLeave,
		ForcedToLeave:       nodeAcc.ForcedToLeave,
		LeaveHeight:         int64(nodeAcc.LeaveScore), // OpenAPI can only represent uint64 as int64
		IpAddress:           nodeAcc.IPAddress,
		Version:             nodeAcc.GetVersion().String(),
		Reward:              nodeAcc.Reward.String(),
	}
	result.PeerId = getPeerIDFromPubKey(nodeAcc.PubKeySet.Secp256k1)
	result.SlashPoints = slashPts
	result.Jail = openapi.NodeJail{
		// Since redundant, leave out the node address
		ReleaseHeight: wrapInt64(jail.ReleaseHeight),
		Reason:        wrapString(jail.Reason),
	}

	// Populate the bond and bonded pools
	bond, err := populateBondAndBondedPools(ctx, mgr, &result, nodeAcc.NodeAddress)
	if err != nil {
		return nil, fmt.Errorf("fail to populate bond and bonded pools: %w", err)
	}

	// CurrentAward is an estimation of reward for node in active status
	// Node in other status should not have current reward
	if nodeAcc.Status == NodeActive && !bond.IsZero() {
		var network Network
		network, err = mgr.Keeper().GetNetwork(ctx)
		if err != nil {
			return nil, fmt.Errorf("fail to get network: %w", err)
		}
		var vaults Vaults
		vaults, err = mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
		if err != nil {
			return nil, fmt.Errorf("fail to get active vaults: %w", err)
		}
		if len(vaults) == 0 {
			return nil, fmt.Errorf("no active vaults")
		}

		// Note that unlike actual BondRewardRune distribution in manager_validator_current.go ,
		// this estimate treats lastChurnHeight as the block_height of the first (oldest) Asgard vault,
		// rather than the active_block_height of the youngest active node.
		// As an example, note from the below URLs that these are 5293728 and 5293733 respectively in block 5336942.
		// https://thornode.ninerealms.com/thorchain/vaults/asgard?height=5336942
		// https://thornode.ninerealms.com/thorchain/nodes?height=5336942
		// (Nodes .cxmy and .uy3a .)
		lastChurnHeight := vaults[0].BlockHeight

		var reward cosmos.Uint
		reward, err = getNodeCurrentRewards(ctx, mgr, nodeAcc, lastChurnHeight, network.BondRewardRune)
		if err != nil {
			return nil, fmt.Errorf("fail to get current node rewards: %w", err)
		}

		result.Reward = reward.String()
	}

	chainHeights, err := mgr.Keeper().GetLastObserveHeight(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("fail to get last observe chain height: %w", err)
	}

	// analyze-ignore(map-iteration)
	for c, h := range chainHeights {
		result.ObserveChains = append(result.ObserveChains, openapi.ChainHeight{
			Chain:  c.String(),
			Height: h,
		})
	}

	preflightCheckResult, err := getNodePreflightResult(ctx, mgr, nodeAcc)
	if err != nil {
		ctx.Logger().Error("fail to get node preflight result", "error", err)
	} else {
		result.PreflightStatus = preflightCheckResult
	}
	return jsonify(ctx, result)
}

func getNodePreflightResult(ctx cosmos.Context, mgr *Mgrs, nodeAcc NodeAccount) (openapi.NodePreflightStatus, error) {
	constAccessor := mgr.GetConstants()
	preflightResult := openapi.NodePreflightStatus{}
	status, err := mgr.ValidatorMgr().NodeAccountPreflightCheck(ctx, nodeAcc, constAccessor)
	preflightResult.Status = status.String()
	if err != nil {
		preflightResult.Reason = err.Error()
		preflightResult.Code = 1
	} else {
		preflightResult.Reason = "OK"
		preflightResult.Code = 0
	}
	return preflightResult, nil
}

// Estimates current rewards for the NodeAccount taking into account slash points
func getNodeCurrentRewards(ctx cosmos.Context, mgr *Mgrs, nodeAcc NodeAccount, lastChurnHeight int64, totalBondReward cosmos.Uint) (cosmos.Uint, error) {
	slashPts, err := mgr.Keeper().GetNodeAccountSlashPoints(ctx, nodeAcc.NodeAddress)
	if err != nil {
		return cosmos.ZeroUint(), fmt.Errorf("fail to get node slash points: %w", err)
	}

	// Find number of blocks since the last churn (the last bond reward payout)
	totalActiveBlocks := ctx.BlockHeight() - lastChurnHeight

	// find number of blocks they were well behaved (ie active - slash points)
	earnedBlocks := totalActiveBlocks - slashPts
	if earnedBlocks < 0 {
		earnedBlocks = 0
	}

	active, err := mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return cosmos.ZeroUint(), fmt.Errorf("fail to get all active node account: %w", err)
	}

	// reward = totalBondReward/# active validators * unslashed blocks since last churn / blocks since last churn)
	reward := totalBondReward.QuoUint64(uint64(len(active)))
	reward = common.GetUncappedShare(cosmos.NewUint(uint64(earnedBlocks)), cosmos.NewUint(uint64(totalActiveBlocks)), reward)
	return reward, nil
}

// queryNodes return all the nodes that has bond
// /thorchain/nodes
func queryNodes(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	nodeAccounts, err := mgr.Keeper().ListValidatorsWithBond(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get node accounts: %w", err)
	}

	network, err := mgr.Keeper().GetNetwork(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get network: %w", err)
	}

	vaults, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		return nil, fmt.Errorf("fail to get active vaults: %w", err)
	}
	if len(vaults) == 0 {
		return nil, fmt.Errorf("no active vaults")
	}

	lastChurnHeight := vaults[0].BlockHeight
	result := make([]openapi.Node, len(nodeAccounts))
	for i, na := range nodeAccounts {
		var naBond cosmos.Uint
		naBond, err = mgr.Keeper().CalcNodeLiquidityBond(ctx, na)
		if err != nil {
			return nil, fmt.Errorf("fail to get node bond: %w", err)
		}
		if na.RequestedToLeave && naBond.LTE(cosmos.NewUint(common.One)) {
			// ignore the node , it left and also has very little bond
			// Set the default display for fields which would otherwise be "".
			result[i] = openapi.Node{
				Status:          types.NodeStatus_Unknown.String(),
				Bond:            "",
				BondProviders:   openapi.NodeBondProviders{NodeOperatorFee: cosmos.ZeroUint().String()},
				Version:         semver.MustParse("0.0.0").String(),
				Reward:          cosmos.ZeroUint().String(),
				PreflightStatus: openapi.NodePreflightStatus{Status: types.NodeStatus_Unknown.String()},
			}
			continue
		}

		var slashPts int64
		slashPts, err = mgr.Keeper().GetNodeAccountSlashPoints(ctx, na.NodeAddress)
		if err != nil {
			return nil, fmt.Errorf("fail to get node slash points: %w", err)
		}

		result[i] = openapi.Node{
			NodeAddress: na.NodeAddress.String(),
			Status:      na.Status.String(),
			PubKeySet: openapi.NodePubKeySet{
				Secp256k1: wrapString(na.PubKeySet.Secp256k1.String()),
				Ed25519:   wrapString(na.PubKeySet.Ed25519.String()),
			},
			ValidatorConsPubKey: na.ValidatorConsPubKey,
			ActiveBlockHeight:   na.ActiveBlockHeight,
			StatusSince:         na.StatusSince,
			BondAddress:         na.BondAddress.String(),
			Bond:                "",
			SignerMembership:    na.GetSignerMembership().Strings(),
			RequestedToLeave:    na.RequestedToLeave,
			ForcedToLeave:       na.ForcedToLeave,
			LeaveHeight:         int64(na.LeaveScore), // OpenAPI can only represent uint64 as int64
			IpAddress:           na.IPAddress,
			Version:             na.GetVersion().String(),
			Reward:              cosmos.ZeroUint().String(), // Default display for if not overwritten.
		}
		result[i].PeerId = getPeerIDFromPubKey(na.PubKeySet.Secp256k1)
		result[i].SlashPoints = slashPts
		if na.Status == NodeActive {
			var reward cosmos.Uint
			reward, err = getNodeCurrentRewards(ctx, mgr, na, lastChurnHeight, network.BondRewardRune)
			if err != nil {
				return nil, fmt.Errorf("fail to get current node rewards: %w", err)
			}

			result[i].Reward = reward.String()
		}

		var jail Jail
		jail, err = mgr.Keeper().GetNodeAccountJail(ctx, na.NodeAddress)
		if err != nil {
			return nil, fmt.Errorf("fail to get node jail: %w", err)
		}
		result[i].Jail = openapi.NodeJail{
			// Since redundant, leave out the node address
			ReleaseHeight: wrapInt64(jail.ReleaseHeight),
			Reason:        wrapString(jail.Reason),
		}

		// TODO: Represent this map as the field directly, instead of making an array?
		// It would then always be represented in alphabetical order.
		var chainHeights map[common.Chain]int64
		chainHeights, err = mgr.Keeper().GetLastObserveHeight(ctx, na.NodeAddress)
		if err != nil {
			return nil, fmt.Errorf("fail to get last observe chain height: %w", err)
		}
		// analyze-ignore(map-iteration)
		for c, h := range chainHeights {
			result[i].ObserveChains = append(result[i].ObserveChains, openapi.ChainHeight{
				Chain:  c.String(),
				Height: h,
			})
		}

		// Populate the bond and bonded pools
		_, err = populateBondAndBondedPools(ctx, mgr, &result[i], na.NodeAddress)
		if err != nil {
			return nil, fmt.Errorf("fail to populate bond and bonded pools: %w", err)
		}

		var preflightCheckResult openapi.NodePreflightStatus
		preflightCheckResult, err = getNodePreflightResult(ctx, mgr, na)
		if err != nil {
			ctx.Logger().Error("fail to get node preflight result", "error", err)
		} else {
			result[i].PreflightStatus = preflightCheckResult
		}
	}

	res, err := json.MarshalIndent(result, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal observers to json", "error", err)
		return nil, fmt.Errorf("fail to marshal observers to json: %w", err)
	}

	return res, nil
}

func newSaver(lp LiquidityProvider, pool Pool) openapi.Saver {
	assetRedeemableValue := lp.GetSaversAssetRedeemValue(pool)

	gp := cosmos.NewDec(0)
	if !lp.AssetDepositValue.IsZero() {
		adv := cosmos.NewDec(lp.AssetDepositValue.BigInt().Int64())
		arv := cosmos.NewDec(assetRedeemableValue.BigInt().Int64())
		gp = arv.Sub(adv)
		gp = gp.Quo(adv)
	}

	return openapi.Saver{
		Asset:              lp.Asset.GetLayer1Asset().String(),
		AssetAddress:       lp.AssetAddress.String(),
		LastAddHeight:      wrapInt64(lp.LastAddHeight),
		LastWithdrawHeight: wrapInt64(lp.LastWithdrawHeight),
		Units:              lp.Units.String(),
		AssetDepositValue:  lp.AssetDepositValue.String(),
		AssetRedeemValue:   assetRedeemableValue.String(),
		GrowthPct:          gp.String(),
	}
}

// queryLiquidityProviders
// isSavers is true if request is for the savers of a Savers Pool, if false the request is for an L1 pool
func queryLiquidityProviders(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs, isSavers bool) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("asset not provided")
	}
	if isSavers {
		path[0] = strings.Replace(path[0], ".", "/", 1)
	}
	asset, err := common.NewAsset(path[0])
	if err != nil {
		ctx.Logger().Error("fail to get parse asset", "error", err)
		return nil, fmt.Errorf("fail to parse asset: %w", err)
	}
	if isSavers && !asset.IsVaultAsset() {
		return nil, fmt.Errorf("invalid request: requested pool is not a SaversPool")
	} else if !isSavers && asset.IsVaultAsset() {
		return nil, fmt.Errorf("invalid request: requested pool is a SaversPool")
	}

	poolAsset := asset
	if isSavers {
		poolAsset = asset.GetSyntheticAsset()
	}

	pool, err := mgr.Keeper().GetPool(ctx, poolAsset)
	if err != nil {
		ctx.Logger().Error("fail to get pool", "error", err)
		return nil, fmt.Errorf("fail to get pool: %w", err)
	}

	var lps []openapi.LiquidityProvider
	var savers []openapi.Saver
	iterator := mgr.Keeper().GetLiquidityProviderIterator(ctx, asset)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var lp LiquidityProvider
		mgr.Keeper().Cdc().MustUnmarshal(iterator.Value(), &lp)
		if !isSavers {
			bondedNodes := make([]openapi.LPBondedNode, len(lp.BondedNodes))

			for i := range lp.BondedNodes {
				bondedNodes[i] = openapi.LPBondedNode{
					NodeAddress: lp.BondedNodes[i].NodeAddress.String(),
					Units:       lp.BondedNodes[i].Units.String(),
				}
			}

			lps = append(lps, openapi.LiquidityProvider{
				// No redeem or LUVI calculations for the array response.
				Asset:              lp.Asset.GetLayer1Asset().String(),
				CacaoAddress:       wrapString(lp.CacaoAddress.String()),
				AssetAddress:       wrapString(lp.AssetAddress.String()),
				LastAddHeight:      wrapInt64(lp.LastAddHeight),
				LastWithdrawHeight: wrapInt64(lp.LastWithdrawHeight),
				Units:              lp.Units.String(),
				PendingCacao:       lp.PendingCacao.String(),
				PendingAsset:       lp.PendingAsset.String(),
				PendingTxId:        wrapString(lp.PendingTxID.String()),
				CacaoDepositValue:  lp.CacaoDepositValue.String(),
				AssetDepositValue:  lp.AssetDepositValue.String(),
				BondedNodes:        bondedNodes,
				WithdrawCounter:    lp.WithdrawCounter.String(),
			})
		} else {
			savers = append(savers, newSaver(lp, pool))
		}
	}
	if !isSavers {
		return jsonify(ctx, lps)
	} else {
		return jsonify(ctx, savers)
	}
}

// queryLiquidityProvider
// isSavers is true if request is for the savers of a Savers Pool, if false the request is for an L1 pool
func queryLiquidityProvider(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs, isSavers bool) ([]byte, error) {
	if len(path) < 2 {
		return nil, errors.New("asset/lp not provided")
	}
	if isSavers {
		path[0] = strings.Replace(path[0], ".", "/", 1)
	}
	asset, err := common.NewAsset(path[0])
	if err != nil {
		ctx.Logger().Error("fail to get parse asset", "error", err)
		return nil, fmt.Errorf("fail to parse asset: %w", err)
	}

	if isSavers && !asset.IsVaultAsset() {
		return nil, fmt.Errorf("invalid request: requested pool is not a SaversPool")
	} else if !isSavers && asset.IsVaultAsset() {
		return nil, fmt.Errorf("invalid request: requested pool is a SaversPool")
	}

	addr, err := common.NewAddress(path[1], mgr.GetVersion())
	if err != nil {
		ctx.Logger().Error("fail to get parse address", "error", err)
		return nil, fmt.Errorf("fail to parse address: %w", err)
	}
	lp, err := mgr.Keeper().GetLiquidityProvider(ctx, asset, addr)
	if err != nil {
		ctx.Logger().Error("fail to get liquidity provider", "error", err)
		return nil, fmt.Errorf("fail to liquidity provider: %w", err)
	}

	poolAsset := asset
	if isSavers {
		poolAsset = asset.GetSyntheticAsset()
	}

	pool, err := mgr.Keeper().GetPool(ctx, poolAsset)
	if err != nil {
		ctx.Logger().Error("fail to get pool", "error", err)
		return nil, fmt.Errorf("fail to get pool: %w", err)
	}

	if !isSavers {
		synthSupply := mgr.Keeper().GetTotalSupply(ctx, poolAsset.GetSyntheticAsset())
		_, cacaoRedeemValue := lp.GetRuneRedeemValue(mgr.GetVersion(), pool, synthSupply)
		_, assetRedeemValue := lp.GetAssetRedeemValue(mgr.GetVersion(), pool, synthSupply)
		_, luviDepositValue := lp.GetLuviDepositValue(pool)
		_, luviRedeemValue := lp.GetLuviRedeemValue(cacaoRedeemValue, assetRedeemValue)

		lgp := cosmos.NewDec(0)
		if !luviDepositValue.IsZero() {
			ldv := cosmos.NewDec(luviDepositValue.BigInt().Int64())
			lrv := cosmos.NewDec(luviRedeemValue.BigInt().Int64())
			lgp = lrv.Sub(ldv)
			lgp = lgp.Quo(ldv)
		}

		bondedNodes := make([]openapi.LPBondedNode, len(lp.GetBondedNodes()))

		for i := range lp.GetBondedNodes() {
			bondedNodes[i] = openapi.LPBondedNode{
				NodeAddress: lp.BondedNodes[i].NodeAddress.String(),
				Units:       lp.BondedNodes[i].Units.String(),
			}
		}

		liqp := openapi.LiquidityProvider{
			Asset:              lp.Asset.GetLayer1Asset().String(),
			CacaoAddress:       wrapString(lp.CacaoAddress.String()),
			AssetAddress:       wrapString(lp.AssetAddress.String()),
			LastAddHeight:      wrapInt64(lp.LastAddHeight),
			LastWithdrawHeight: wrapInt64(lp.LastWithdrawHeight),
			Units:              lp.Units.String(),
			PendingCacao:       lp.PendingCacao.String(),
			PendingAsset:       lp.PendingAsset.String(),
			PendingTxId:        wrapString(lp.PendingTxID.String()),
			CacaoDepositValue:  lp.CacaoDepositValue.String(),
			AssetDepositValue:  lp.AssetDepositValue.String(),
			CacaoRedeemValue:   wrapString(cacaoRedeemValue.String()),
			AssetRedeemValue:   wrapString(assetRedeemValue.String()),
			LuviDepositValue:   wrapString(luviDepositValue.String()),
			LuviRedeemValue:    wrapString(luviRedeemValue.String()),
			LuviGrowthPct:      wrapString(lgp.String()),
			BondedNodes:        bondedNodes,
			WithdrawCounter:    lp.WithdrawCounter.String(),
		}
		return jsonify(ctx, liqp)
	} else {
		saver := newSaver(lp, pool)
		return jsonify(ctx, saver)
	}
}

func newStreamingSwap(streamingSwap StreamingSwap, msgSwap MsgSwap) openapi.StreamingSwap {
	var sourceAsset common.Asset
	// Leave the source_asset field empty if there is more than a single input Coin.
	if len(msgSwap.Tx.Coins) == 1 {
		sourceAsset = msgSwap.Tx.Coins[0].Asset
	}

	var failedSwaps []int64
	// Leave this nil (null rather than []) if the source is nil.
	if streamingSwap.FailedSwaps != nil {
		failedSwaps = make([]int64, len(streamingSwap.FailedSwaps))
		for i := range streamingSwap.FailedSwaps {
			failedSwaps[i] = int64(streamingSwap.FailedSwaps[i])
		}
	}

	return openapi.StreamingSwap{
		TxId:              wrapString(streamingSwap.TxID.String()),
		Interval:          wrapInt64(int64(streamingSwap.Interval)),
		Quantity:          wrapInt64(int64(streamingSwap.Quantity)),
		Count:             wrapInt64(int64(streamingSwap.Count)),
		LastHeight:        wrapInt64(streamingSwap.LastHeight),
		TradeTarget:       streamingSwap.TradeTarget.String(),
		SourceAsset:       wrapString(sourceAsset.String()),
		TargetAsset:       wrapString(msgSwap.TargetAsset.String()),
		Destination:       wrapString(msgSwap.Destination.String()),
		Deposit:           streamingSwap.Deposit.String(),
		In:                streamingSwap.In.String(),
		Out:               streamingSwap.Out.String(),
		FailedSwaps:       failedSwaps,
		FailedSwapReasons: streamingSwap.FailedSwapReasons,
	}
}

func queryStreamingSwaps(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	var streams []openapi.StreamingSwap
	iter := mgr.Keeper().GetStreamingSwapIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var stream StreamingSwap
		mgr.Keeper().Cdc().MustUnmarshal(iter.Value(), &stream)

		var msgSwap MsgSwap
		// Check up to the first two indices (0 through 1) for the MsgSwap; if not found, leave the fields blank.
		for i := 0; i <= 1; i++ {
			swapQueueItem, err := mgr.Keeper().GetSwapQueueItem(ctx, stream.TxID, i)
			if err != nil {
				// GetSwapQueueItem returns an error if there is no MsgSwap set for that index, a normal occurrence here.
				continue
			}
			if !swapQueueItem.IsStreaming() {
				continue
			}
			// In case there are multiple streaming swaps with the same TxID, check the input amount.
			if len(swapQueueItem.Tx.Coins) == 0 || !swapQueueItem.Tx.Coins[0].Amount.Equal(stream.Deposit) {
				continue
			}
			msgSwap = swapQueueItem
			break
		}

		streams = append(streams, newStreamingSwap(stream, msgSwap))
	}
	return jsonify(ctx, streams)
}

func queryStreamingSwap(ctx cosmos.Context, path []string, mgr *Mgrs) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("tx id not provided")
	}
	txid, err := common.NewTxID(path[0])
	if err != nil {
		ctx.Logger().Error("fail to parse txid", "error", err)
		return nil, fmt.Errorf("could not parse txid: %w", err)
	}

	streamingSwap, err := mgr.Keeper().GetStreamingSwap(ctx, txid)
	if err != nil {
		ctx.Logger().Error("fail to get streaming swap", "error", err)
		return nil, fmt.Errorf("could not get streaming swap: %w", err)
	}

	var msgSwap MsgSwap
	// Check up to the first two indices (0 through 1) for the MsgSwap; if not found, leave the fields blank.
	for i := 0; i <= 1; i++ {
		swapQueueItem, err := mgr.Keeper().GetSwapQueueItem(ctx, txid, i)
		if err != nil {
			// GetSwapQueueItem returns an error if there is no MsgSwap set for that index, a normal occurrence here.
			continue
		}
		if !swapQueueItem.IsStreaming() {
			continue
		}
		// In case there are multiple streaming swaps with the same TxID, check the input amount.
		if len(swapQueueItem.Tx.Coins) == 0 || !swapQueueItem.Tx.Coins[0].Amount.Equal(streamingSwap.Deposit) {
			continue
		}
		msgSwap = swapQueueItem
		break
	}

	result := newStreamingSwap(streamingSwap, msgSwap)

	return jsonify(ctx, result)
}

func queryPool(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("asset not provided")
	}
	asset, err := common.NewAsset(path[0])
	if err != nil {
		ctx.Logger().Error("fail to parse asset", "error", err)
		return nil, fmt.Errorf("could not parse asset: %w", err)
	}

	pool, err := mgr.Keeper().GetPool(ctx, asset)
	if err != nil {
		ctx.Logger().Error("fail to get pool", "error", err)
		return nil, fmt.Errorf("could not get pool: %w", err)
	}
	if pool.IsEmpty() {
		return nil, fmt.Errorf("pool: %s doesn't exist", path[0])
	}

	// Get Savers Vault for this L1 pool if it's a gas asset
	saversAsset := pool.Asset.GetSyntheticAsset()
	saversPool, err := mgr.Keeper().GetPool(ctx, saversAsset)
	if err != nil {
		return nil, fmt.Errorf("fail to unmarshal savers vault: %w", err)
	}

	saversDepth := saversPool.BalanceAsset
	saversUnits := saversPool.LPUnits

	synthSupply := mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
	pool.CalcUnits(mgr.GetVersion(), synthSupply)
	synthMintPausedErr := isSynthMintPaused(ctx, mgr, saversAsset, cosmos.ZeroUint())

	bondablePools := GetLiquidityPools(mgr.GetVersion())
	bondable := bondablePools.Contains(pool.Asset)

	p := &openapi.Pool{
		BalanceCacao:        pool.BalanceCacao.String(),
		BalanceAsset:        pool.BalanceAsset.String(),
		Asset:               pool.Asset.String(),
		LPUnits:             pool.LPUnits.String(),
		PoolUnits:           pool.GetPoolUnits().String(),
		Status:              pool.Status.String(),
		Decimals:            wrapInt64(pool.Decimals),
		SynthUnits:          pool.SynthUnits.String(),
		SynthSupply:         synthSupply.String(),
		PendingInboundCacao: pool.PendingInboundCacao.String(),
		PendingInboundAsset: pool.PendingInboundAsset.String(),
		SaversDepth:         saversDepth.String(),
		SaversUnits:         saversUnits.String(),
		SynthMintPaused:     synthMintPausedErr != nil,
		Bondable:            bondable,
	}

	res, err := json.MarshalIndent(p, "", "	")
	if err != nil {
		return nil, fmt.Errorf("could not marshal result to JSON: %w", err)
	}
	return res, nil
}

func queryPools(ctx cosmos.Context, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	pools := make([]openapi.Pool, 0)
	iterator := mgr.Keeper().GetPoolIterator(ctx)
	bondablePools := GetLiquidityPools(mgr.GetVersion())
	for ; iterator.Valid(); iterator.Next() {
		var pool Pool
		if err := mgr.Keeper().Cdc().Unmarshal(iterator.Value(), &pool); err != nil {
			return nil, fmt.Errorf("fail to unmarshal pool: %w", err)
		}
		// ignore pool if no liquidity provider units
		if pool.LPUnits.IsZero() {
			continue
		}

		// Ignore synth asset pool (savers). Info will be on the L1 pool
		if pool.Asset.IsVaultAsset() {
			continue
		}

		// Get Savers Vault
		saversAsset := pool.Asset.GetSyntheticAsset()
		saversPool, err := mgr.Keeper().GetPool(ctx, saversAsset)
		if err != nil {
			return nil, fmt.Errorf("fail to unmarshal savers vault: %w", err)
		}

		saversDepth := saversPool.BalanceAsset
		saversUnits := saversPool.LPUnits

		synthSupply := mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
		pool.CalcUnits(mgr.GetVersion(), synthSupply)
		synthMintPausedErr := isSynthMintPaused(ctx, mgr, saversAsset, cosmos.ZeroUint())

		bondable := bondablePools.Contains(pool.Asset)

		p := openapi.Pool{
			BalanceCacao:        pool.BalanceCacao.String(),
			BalanceAsset:        pool.BalanceAsset.String(),
			Asset:               pool.Asset.String(),
			LPUnits:             pool.LPUnits.String(),
			PoolUnits:           pool.GetPoolUnits().String(),
			Status:              pool.Status.String(),
			Decimals:            wrapInt64(pool.Decimals),
			SynthUnits:          pool.SynthUnits.String(),
			SynthSupply:         synthSupply.String(),
			PendingInboundCacao: pool.PendingInboundCacao.String(),
			PendingInboundAsset: pool.PendingInboundAsset.String(),
			SaversDepth:         saversDepth.String(),
			SaversUnits:         saversUnits.String(),
			SynthMintPaused:     synthMintPausedErr != nil,
			Bondable:            bondable,
		}
		pools = append(pools, p)
	}
	res, err := json.MarshalIndent(pools, "", "	")
	if err != nil {
		return nil, fmt.Errorf("could not marshal pools result to json: %w", err)
	}
	return res, nil
}

// TODO: Remove isSwap and isPending code when SwapFinalised field deprecated.
func checkPending(ctx cosmos.Context, keeper keeper.Keeper, voter ObservedTxVoter) (isSwap, isPending, pending bool, streamingSwap StreamingSwap) {
	// If there's no (confirmation-counting-complete) consensus transaction yet, don't spend time checking the swap status.
	if voter.Tx.IsEmpty() || !voter.Tx.IsFinal() {
		return
	}

	pending = keeper.HasSwapQueueItem(ctx, voter.TxID, 0) || keeper.HasOrderBookItem(ctx, voter.TxID)

	// Only look for streaming information when a swap is pending.
	if pending {
		var err error
		streamingSwap, err = keeper.GetStreamingSwap(ctx, voter.TxID)
		if err != nil {
			// Log the error, but continue without streaming information.
			ctx.Logger().Error("fail to get streaming swap", "error", err)
		}
	}

	memo, err := ParseMemoWithMAYANames(ctx, keeper, voter.Tx.Tx.Memo)
	if err != nil {
		// If unable to parse, assume not a (valid) swap or limit order memo.
		return
	}

	memoType := memo.GetType()
	// If the memo asset is a synth, as with Savers add liquidity or withdraw, a swap is assumed to be involved.
	if memoType == TxSwap || memoType == TxLimitOrder || memo.GetAsset().IsVaultAsset() {
		isSwap = true
		// Only check the KVStore when the inbound transaction has already been finalised
		// and when there haven't been any Actions planned.
		// This will also check the KVStore when an inbound transaction has no output,
		// such as the output being not enough to cover a fee.
		if voter.FinalisedHeight != 0 && len(voter.Actions) == 0 {
			// Use of Swap Queue or Order Book depends on Mimir key EnableOrderBooks rather than memo type, so check both.
			isPending = pending
		}
	}

	return
}

func queryTradeUnit(ctx cosmos.Context, path []string, mgr *Mgrs) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("asset not provided")
	}
	asset, err := common.NewAsset(path[0])
	if err != nil {
		ctx.Logger().Error("fail to parse asset", "error", err)
		return nil, fmt.Errorf("could not parse asset: %w", err)
	}

	tu, err := mgr.Keeper().GetTradeUnit(ctx, asset)
	if err != nil {
		ctx.Logger().Error("fail to get trade unit", "error", err)
		return nil, fmt.Errorf("could not get trade unit: %w", err)
	}
	tuResp := openapi.TradeUnitResponse{
		Asset: tu.Asset.String(),
		Units: tu.Units.String(),
		Depth: tu.Depth.String(),
	}
	return jsonify(ctx, tuResp)
}

func queryTradeUnits(ctx cosmos.Context, path []string, mgr *Mgrs) ([]byte, error) {
	pools, err := mgr.Keeper().GetPools(ctx)
	if err != nil {
		return nil, errors.New("failed to get pools")
	}
	units := make([]openapi.TradeUnitResponse, 0)
	for _, pool := range pools {
		// skip non-layer1 pools
		if pool.Asset.GetChain().IsBASEChain() {
			continue
		}
		asset := pool.Asset.GetTradeAsset()
		tu, err := mgr.Keeper().GetTradeUnit(ctx, asset)
		if err != nil {
			ctx.Logger().Error("fail to get trade unit", "error", err)
			return nil, fmt.Errorf("could not get trade unit: %w", err)
		}
		tuResp := openapi.TradeUnitResponse{
			Asset: tu.Asset.String(),
			Units: tu.Units.String(),
			Depth: tu.Depth.String(),
		}
		units = append(units, tuResp)
	}

	return jsonify(ctx, units)
}

func queryTradeAccounts(ctx cosmos.Context, path []string, mgr *Mgrs) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("asset not provided")
	}
	asset, err := common.NewAsset(path[0])
	if err != nil {
		ctx.Logger().Error("fail to parse address", "error", err)
		return nil, fmt.Errorf("could not parse address: %w", err)
	}

	accounts := make([]openapi.TradeAccountResponse, 0)
	iter := mgr.Keeper().GetTradeAccountIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var ta TradeAccount
		if err := mgr.Keeper().Cdc().Unmarshal(iter.Value(), &ta); err != nil {
			continue
		}
		if !ta.Asset.Equals(asset) {
			continue
		}
		if ta.Units.IsZero() {
			continue
		}
		taResp := openapi.TradeAccountResponse{
			Asset:              ta.Asset.String(),
			Units:              ta.Units.String(),
			Owner:              ta.Owner.String(),
			LastAddHeight:      &ta.LastAddHeight,
			LastWithdrawHeight: &ta.LastWithdrawHeight,
		}
		accounts = append(accounts, taResp)
	}

	return jsonify(ctx, accounts)
}

func queryTradeAccount(ctx cosmos.Context, path []string, mgr *Mgrs) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("address not provided")
	}
	addr, err := cosmos.AccAddressFromBech32(path[0])
	if err != nil {
		ctx.Logger().Error("fail to parse address", "error", err)
		return nil, fmt.Errorf("could not parse address: %w", err)
	}

	accounts := make([]openapi.TradeAccountResponse, 0)
	iter := mgr.Keeper().GetTradeAccountIteratorWithAddress(ctx, addr)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var ta TradeAccount
		if err := mgr.Keeper().Cdc().Unmarshal(iter.Value(), &ta); err != nil {
			continue
		}
		if ta.Units.IsZero() {
			continue
		}

		taResp := openapi.TradeAccountResponse{
			Asset:              ta.Asset.String(),
			Units:              ta.Units.String(),
			Owner:              ta.Owner.String(),
			LastAddHeight:      &ta.LastAddHeight,
			LastWithdrawHeight: &ta.LastWithdrawHeight,
		}
		accounts = append(accounts, taResp)
	}

	return jsonify(ctx, accounts)
}

func extractVoter(ctx cosmos.Context, path []string, mgr *Mgrs) (common.TxID, ObservedTxVoter, error) {
	if len(path) == 0 {
		return "", ObservedTxVoter{}, errors.New("tx id not provided")
	}
	hash, err := common.NewTxID(path[0])
	if err != nil {
		ctx.Logger().Error("fail to parse tx id", "error", err)
		return "", ObservedTxVoter{}, fmt.Errorf("fail to parse tx id: %w", err)
	}
	voter, err := mgr.Keeper().GetObservedTxInVoter(ctx, hash)
	if err != nil {
		ctx.Logger().Error("fail to get observed tx voter", "error", err)
		return "", ObservedTxVoter{}, fmt.Errorf("fail to get observed tx voter: %w", err)
	}
	return hash, voter, nil
}

func queryTxVoters(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	hash, voter, err := extractVoter(ctx, path, mgr)
	if err != nil {
		return nil, err
	}
	// when tx in voter doesn't exist , double check tx out voter
	if len(voter.Txs) == 0 {
		voter, err = mgr.Keeper().GetObservedTxOutVoter(ctx, hash)
		if err != nil {
			return nil, fmt.Errorf("fail to get observed tx out voter: %w", err)
		}
		if len(voter.Txs) == 0 {
			return nil, fmt.Errorf("tx: %s doesn't exist", hash)
		}
	}

	var txs []openapi.ObservedTx
	fmt.Println("voter", voter.String())
	// Leave this nil (null rather than []) if the source is nil.
	if voter.Txs != nil {
		txs = make([]openapi.ObservedTx, len(voter.Txs))
		for i := range voter.Txs {
			txs[i] = castObservedTx(voter.Txs[i])
		}
	}

	var actions []openapi.TxOutItem
	// Leave this nil (null rather than []) if the source is nil.
	if voter.Actions != nil {
		actions = make([]openapi.TxOutItem, len(voter.Actions))
		for i := range voter.Actions {
			actions[i] = castTxOutItem(voter.Actions[i], 0) // Omitted Height field
		}
	}

	var outTxs []openapi.Tx
	// Leave this nil (null rather than []) if the source is nil.
	if voter.OutTxs != nil {
		outTxs = make([]openapi.Tx, len(voter.OutTxs))
		for i := range voter.OutTxs {
			outTxs[i] = castTx(voter.OutTxs[i])
		}
	}

	result := openapi.TxDetailsResponse{
		TxId:            wrapString(voter.TxID.String()),
		Tx:              castObservedTx(voter.Tx),
		Txs:             txs,
		Actions:         actions,
		OutTxs:          outTxs,
		ConsensusHeight: wrapInt64(voter.Height),
		FinalisedHeight: wrapInt64(voter.FinalisedHeight),
		UpdatedVault:    wrapBool(voter.UpdatedVault),
		Reverted:        wrapBool(voter.Reverted),
		OutboundHeight:  wrapInt64(voter.OutboundHeight),
	}

	return jsonify(ctx, result)
}

func queryTxVotersOld(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("tx id not provided")
	}
	hash, err := common.NewTxID(path[0])
	if err != nil {
		ctx.Logger().Error("fail to parse tx id", "error", err)
		return nil, fmt.Errorf("fail to parse tx id: %w", err)
	}
	voter, err := mgr.Keeper().GetObservedTxInVoter(ctx, hash)
	if err != nil {
		ctx.Logger().Error("fail to get observed tx voter", "error", err)
		return nil, fmt.Errorf("fail to get observed tx voter: %w", err)
	}
	// when tx in voter doesn't exist , double check tx out voter
	if len(voter.Txs) == 0 {
		voter, err = mgr.Keeper().GetObservedTxOutVoter(ctx, hash)
		if err != nil {
			return nil, fmt.Errorf("fail to get observed tx out voter: %w", err)
		}
		if len(voter.Txs) == 0 {
			return nil, fmt.Errorf("tx: %s doesn't exist", hash)
		}
	}

	res, err := json.MarshalIndent(voter, "", "        ")
	if err != nil {
		ctx.Logger().Error("fail to marshal tx hash to json", "error", err)
		return nil, fmt.Errorf("fail to marshal tx hash to json: %w", err)
	}
	return res, nil
}

func queryTxInVoter(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("tx id not provided")
	}
	hash, err := common.NewTxID(path[0])
	if err != nil {
		ctx.Logger().Error("fail to parse tx id", "error", err)
		return nil, fmt.Errorf("fail to parse tx id: %w", err)
	}
	voter, err := mgr.Keeper().GetObservedTxInVoter(ctx, hash)
	if err != nil {
		ctx.Logger().Error("fail to get observed tx in voter", "error", err)
		return nil, fmt.Errorf("fail to get observed tx in voter: %w", err)
	}
	if len(voter.Txs) == 0 {
		return nil, fmt.Errorf("tx in voter: %s doesn't exist", hash)
	}

	var txs []openapi.ObservedTx
	// Leave this nil (null rather than []) if the source is nil.
	if voter.Txs != nil {
		txs = make([]openapi.ObservedTx, len(voter.Txs))
		for i := range voter.Txs {
			txs[i] = castObservedTx(voter.Txs[i])
		}
	}

	var actions []openapi.TxOutItem
	// Leave this nil (null rather than []) if the source is nil.
	if voter.Actions != nil {
		actions = make([]openapi.TxOutItem, len(voter.Actions))
		for i := range voter.Actions {
			actions[i] = castTxOutItem(voter.Actions[i], 0) // Omitted Height field
		}
	}

	var outTxs []openapi.Tx
	// Leave this nil (null rather than []) if the source is nil.
	if voter.OutTxs != nil {
		outTxs = make([]openapi.Tx, len(voter.OutTxs))
		for i := range voter.OutTxs {
			outTxs[i] = castTx(voter.OutTxs[i])
		}
	}

	result := openapi.TxDetailsResponse{
		TxId:            wrapString(voter.TxID.String()),
		Tx:              castObservedTx(voter.Tx),
		Txs:             txs,
		Actions:         actions,
		OutTxs:          outTxs,
		ConsensusHeight: wrapInt64(voter.Height),
		FinalisedHeight: wrapInt64(voter.FinalisedHeight),
		UpdatedVault:    wrapBool(voter.UpdatedVault),
		Reverted:        wrapBool(voter.Reverted),
		OutboundHeight:  wrapInt64(voter.OutboundHeight),
	}

	return jsonify(ctx, result)
}

func queryTxOutVoter(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("tx id not provided")
	}
	hash, err := common.NewTxID(path[0])
	if err != nil {
		ctx.Logger().Error("fail to parse tx id", "error", err)
		return nil, fmt.Errorf("fail to parse tx id: %w", err)
	}
	voter, err := mgr.Keeper().GetObservedTxOutVoter(ctx, hash)
	if err != nil {
		ctx.Logger().Error("fail to get observed tx out voter", "error", err)
		return nil, fmt.Errorf("fail to get observed tx out voter: %w", err)
	}
	if len(voter.Txs) == 0 {
		return nil, fmt.Errorf("tx out voter: %s doesn't exist", hash)
	}

	var txs []openapi.ObservedTx
	// Leave this nil (null rather than []) if the source is nil.
	if voter.Txs != nil {
		txs = make([]openapi.ObservedTx, len(voter.Txs))
		for i := range voter.Txs {
			txs[i] = castObservedTx(voter.Txs[i])
		}
	}

	var actions []openapi.TxOutItem
	// Leave this nil (null rather than []) if the source is nil.
	if voter.Actions != nil {
		actions = make([]openapi.TxOutItem, len(voter.Actions))
		for i := range voter.Actions {
			actions[i] = castTxOutItem(voter.Actions[i], 0) // Omitted Height field
		}
	}

	var outTxs []openapi.Tx
	// Leave this nil (null rather than []) if the source is nil.
	if voter.OutTxs != nil {
		outTxs = make([]openapi.Tx, len(voter.OutTxs))
		for i := range voter.OutTxs {
			outTxs[i] = castTx(voter.OutTxs[i])
		}
	}

	result := openapi.TxDetailsResponse{
		TxId:            wrapString(voter.TxID.String()),
		Tx:              castObservedTx(voter.Tx),
		Txs:             txs,
		Actions:         actions,
		OutTxs:          outTxs,
		ConsensusHeight: wrapInt64(voter.Height),
		FinalisedHeight: wrapInt64(voter.FinalisedHeight),
		UpdatedVault:    wrapBool(voter.UpdatedVault),
		Reverted:        wrapBool(voter.Reverted),
		OutboundHeight:  wrapInt64(voter.OutboundHeight),
	}

	return jsonify(ctx, result)
}

// Get the largest number of signers for a not-final (pre-confirmation-counting) and final Txs respectively.
func countSigners(voter ObservedTxVoter) (*int64, int64) {
	var notFinalCount, finalCount int64
	for i, refTx := range voter.Txs {
		signersMap := make(map[string]bool)
		final := refTx.IsFinal()
		for f, tx := range voter.Txs {
			// Earlier Txs already checked against all, so no need to check,
			// but do include the signers of the current Txs.
			if f < i {
				continue
			}
			// Count larger number of signers for not-final and final observations separately.
			if tx.IsFinal() != final {
				continue
			}
			if !refTx.Tx.EqualsEx(tx.Tx) {
				continue
			}

			for _, signer := range tx.GetSigners() {
				signersMap[signer.String()] = true
			}
		}
		if final && int64(len(signersMap)) > finalCount {
			finalCount = int64(len(signersMap))
		} else if int64(len(signersMap)) > notFinalCount {
			notFinalCount = int64(len(signersMap))
		}
	}
	return wrapInt64(notFinalCount), finalCount
}

// Call newTxStagesResponse from both queryTxStatus (which includes the stages) and queryTxStages.
// TODO: Remove isSwap and isPending arguments when SwapFinalised deprecated in favour of SwapStatus.
// TODO: Deprecate InboundObserved.Started field in favour of the observation counting.
func newTxStagesResponse(ctx cosmos.Context, voter ObservedTxVoter, isSwap, isPending, pending bool, streamingSwap StreamingSwap) (result openapi.TxStagesResponse) {
	result.InboundObserved.PreConfirmationCount, result.InboundObserved.FinalCount = countSigners(voter)
	result.InboundObserved.Completed = !voter.Tx.IsEmpty()

	// If not Completed, fill in Started and do not proceed.
	if !result.InboundObserved.Completed {
		obStart := (len(voter.Txs) != 0)
		result.InboundObserved.Started = &obStart
		return result
	}

	// Current block height is relevant in the confirmation counting and outbound stages.
	currentHeight := ctx.BlockHeight()

	// Only fill in InboundConfirmationCounted when confirmation counting took place.
	if voter.Height != 0 {
		var confCount openapi.InboundConfirmationCountedStage

		// Set the Completed state first.
		extObsHeight := voter.Tx.BlockHeight
		extConfDelayHeight := voter.Tx.FinaliseHeight
		confCount.Completed = !(extConfDelayHeight > extObsHeight)

		// Only fill in other fields if not Completed.
		if !confCount.Completed {
			countStartHeight := voter.Height
			confCount.CountingStartHeight = wrapInt64(countStartHeight)
			confCount.Chain = wrapString(voter.Tx.Tx.Chain.String())
			confCount.ExternalObservedHeight = wrapInt64(extObsHeight)
			confCount.ExternalConfirmationDelayHeight = wrapInt64(extConfDelayHeight)

			estConfMs := voter.Tx.Tx.Chain.ApproximateBlockMilliseconds() * (extConfDelayHeight - extObsHeight)
			if currentHeight > countStartHeight {
				estConfMs -= (currentHeight - countStartHeight) * common.BASEChain.ApproximateBlockMilliseconds()
			}
			estConfSec := estConfMs / 1000
			// Floor at 0.
			if estConfSec < 0 {
				estConfSec = 0
			}
			confCount.RemainingConfirmationSeconds = &estConfSec
		}

		result.InboundConfirmationCounted = &confCount
	}

	var inboundFinalised openapi.InboundFinalisedStage
	inboundFinalised.Completed = (voter.FinalisedHeight != 0)
	result.InboundFinalised = &inboundFinalised

	var swapStatus openapi.SwapStatus
	swapStatus.Pending = pending
	// Only display the SwapStatus stage's Streaming field when there's streaming information available.
	if streamingSwap.Valid() == nil {
		streaming := openapi.StreamingStatus{
			Interval: int64(streamingSwap.Interval),
			Quantity: int64(streamingSwap.Quantity),
			Count:    int64(streamingSwap.Count),
		}
		swapStatus.Streaming = &streaming
	}
	result.SwapStatus = &swapStatus

	// Whether there's an external outbound or not, show the SwapFinalised stage from the start.
	if isSwap {
		var swapFinalisedState openapi.SwapFinalisedStage

		swapFinalisedState.Completed = false
		if !isPending && result.InboundFinalised.Completed {
			// Record as completed only when not pending after the inbound has already been finalised.
			swapFinalisedState.Completed = true
		}

		result.SwapFinalised = &swapFinalisedState
	}

	// Only fill ExternalOutboundDelay and ExternalOutboundKeysign for inbound transactions with an external outbound;
	// namely, transactions with an outbound_height .
	if voter.OutboundHeight == 0 {
		return result
	}

	// Only display the OutboundDelay stage when there's a delay.
	if voter.OutboundHeight > voter.FinalisedHeight {
		var outDelay openapi.OutboundDelayStage

		// Set the Completed state first.
		outDelay.Completed = (currentHeight >= voter.OutboundHeight)

		// Only fill in other fields if not Completed.
		if !outDelay.Completed {
			remainBlocks := voter.OutboundHeight - currentHeight
			outDelay.RemainingDelayBlocks = &remainBlocks

			remainSec := remainBlocks * common.THORChain.ApproximateBlockMilliseconds() / 1000
			outDelay.RemainingDelaySeconds = &remainSec
		}

		result.OutboundDelay = &outDelay
	}

	var outSigned openapi.OutboundSignedStage

	// Set the Completed state first.
	outSigned.Completed = (voter.Tx.Status != types.Status_incomplete)

	// Only fill in other fields if not Completed.
	if !outSigned.Completed {
		scheduledHeight := voter.OutboundHeight
		outSigned.ScheduledOutboundHeight = &scheduledHeight

		// Only fill in BlocksSinceScheduled if the outbound delay is complete.
		if currentHeight >= scheduledHeight {
			sinceScheduled := currentHeight - scheduledHeight
			outSigned.BlocksSinceScheduled = &sinceScheduled
		}
	}

	result.OutboundSigned = &outSigned

	return result
}

func queryTxStages(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	// First, get the ObservedTxVoter of interest.
	_, voter, err := extractVoter(ctx, path, mgr)
	if err != nil {
		return nil, err
	}
	// when no TxIn voter don't check TxOut voter, as TxOut THORChain observation or not matters little to the user once signed and broadcast
	// Rather than a "tx: %s doesn't exist" result, allow a response to an existing-but-unobserved hash with Observation.Started 'false'.

	isSwap, isPending, pending, streamingSwap := checkPending(ctx, mgr.Keeper(), voter)

	result := newTxStagesResponse(ctx, voter, isSwap, isPending, pending, streamingSwap)

	return jsonify(ctx, result)
}

func queryTxStatus(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	// First, get the ObservedTxVoter of interest.
	_, voter, err := extractVoter(ctx, path, mgr)
	if err != nil {
		return nil, err
	}
	// when no TxIn voter don't check TxOut voter, as TxOut THORChain observation or not matters little to the user once signed and broadcast
	// Rather than a "tx: %s doesn't exist" result, allow a response to an existing-but-unobserved hash with Stages.Observation.Started 'false'.

	// TODO: Remove isSwap and isPending arguments when SwapFinalised deprecated.
	isSwap, isPending, pending, streamingSwap := checkPending(ctx, mgr.Keeper(), voter)

	var result openapi.TxStatusResponse

	// If there's a consensus Tx, display that.
	// If not, but there's at least one observation, display the first observation's Tx.
	// If there are no observations yet, don't display a Tx (only showing the 'Observation' stage with 'Started' false).
	if !voter.Tx.Tx.IsEmpty() {
		tx := castTx(voter.Tx.Tx)
		result.Tx = &tx
	} else if len(voter.Txs) > 0 {
		tx := castTx(voter.Txs[0].Tx)
		result.Tx = &tx
	}

	// Leave this nil (null rather than []) if the source is nil.
	if voter.Actions != nil {
		result.PlannedOutTxs = make([]openapi.PlannedOutTx, len(voter.Actions))
		for i := range voter.Actions {
			result.PlannedOutTxs[i] = openapi.PlannedOutTx{
				Chain:     voter.Actions[i].Chain.String(),
				ToAddress: voter.Actions[i].ToAddress.String(),
				Coin:      castCoin(voter.Actions[i].Coin),
				Refund:    strings.HasPrefix(voter.Actions[i].Memo, "REFUND"),
			}
		}
	}

	// Leave this nil (null rather than []) if the source is nil.
	if voter.OutTxs != nil {
		result.OutTxs = make([]openapi.Tx, len(voter.OutTxs))
		for i := range voter.OutTxs {
			result.OutTxs[i] = castTx(voter.OutTxs[i])
		}
	}

	result.Stages = newTxStagesResponse(ctx, voter, isSwap, isPending, pending, streamingSwap)

	return jsonify(ctx, result)
}

func queryTx(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	hash, voter, err := extractVoter(ctx, path, mgr)
	if err != nil {
		return nil, err
	}
	if len(voter.Txs) == 0 {
		voter, err = mgr.Keeper().GetObservedTxOutVoter(ctx, hash)
		if err != nil {
			return nil, fmt.Errorf("fail to get observed tx out voter: %w", err)
		}
		if len(voter.Txs) == 0 {
			return nil, fmt.Errorf("tx: %s doesn't exist", hash)
		}
	}

	nodeAccounts, err := mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get node accounts: %w", err)
	}
	keysignMetric, err := mgr.Keeper().GetTssKeysignMetric(ctx, hash)
	if err != nil {
		ctx.Logger().Error("fail to get keysign metrics", "error", err)
	}
	result := struct {
		ObservedTx      openapi.ObservedTx     `json:"observed_tx"`
		ConsensusHeight int64                  `json:"consensus_height,omitempty"`
		FinalisedHeight int64                  `json:"finalised_height,omitempty"`
		OutboundHeight  int64                  `json:"outbound_height,omitempty"`
		KeysignMetrics  types.TssKeysignMetric `json:"keysign_metric"`
	}{
		ObservedTx:      castObservedTx(voter.GetTx(nodeAccounts)),
		ConsensusHeight: voter.Height,
		FinalisedHeight: voter.FinalisedHeight,
		OutboundHeight:  voter.OutboundHeight,
		KeysignMetrics:  *keysignMetric,
	}
	return jsonify(ctx, result)
}

func extractBlockHeight(ctx cosmos.Context, path []string) (int64, error) {
	if len(path) == 0 {
		return -1, errors.New("block height not provided")
	}
	height, err := strconv.ParseInt(path[0], 0, 64)
	if err != nil {
		ctx.Logger().Error("fail to parse block height", "error", err)
		return -1, fmt.Errorf("fail to parse block height: %w", err)
	}
	if height > ctx.BlockHeight() {
		return -1, fmt.Errorf("block height not available yet")
	}
	return height, nil
}

func queryKeygen(ctx cosmos.Context, kbs cosmos.KeybaseStore, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	height, err := extractBlockHeight(ctx, path)
	if err != nil {
		return nil, err
	}

	keygenBlock, err := mgr.Keeper().GetKeygenBlock(ctx, height)
	if err != nil {
		ctx.Logger().Error("fail to get keygen block", "error", err)
		return nil, fmt.Errorf("fail to get keygen block: %w", err)
	}

	if len(path) > 1 {
		var pk common.PubKey
		pk, err = common.NewPubKey(path[1])
		if err != nil {
			ctx.Logger().Error("fail to parse pubkey", "error", err)
			return nil, fmt.Errorf("fail to parse pubkey: %w", err)
		}
		// only return those keygen contains the request pub key
		newKeygenBlock := NewKeygenBlock(keygenBlock.Height)
		for _, keygen := range keygenBlock.Keygens {
			if keygen.GetMembers().Contains(pk) {
				newKeygenBlock.Keygens = append(newKeygenBlock.Keygens, keygen)
			}
		}
		keygenBlock = newKeygenBlock
	}

	buf, err := json.Marshal(keygenBlock)
	if err != nil {
		ctx.Logger().Error("fail to marshal keygen block to json", "error", err)
		return nil, fmt.Errorf("fail to marshal keygen block to json: %w", err)
	}
	sig, _, err := kbs.Keybase.Sign("mayachain", buf)
	if err != nil {
		ctx.Logger().Error("fail to sign keygen", "error", err)
		return nil, fmt.Errorf("fail to sign keygen: %w", err)
	}

	var keygens []openapi.Keygen
	// Leave this nil (null rather than []) if the source is nil.
	if keygenBlock.Keygens != nil {
		keygens = make([]openapi.Keygen, len(keygenBlock.Keygens))
		for i := range keygenBlock.Keygens {
			keygens[i] = openapi.Keygen{
				Id:      wrapString(keygenBlock.Keygens[i].ID.String()),
				Type:    wrapString(keygenBlock.Keygens[i].Type.String()),
				Members: keygenBlock.Keygens[i].Members,
			}
		}
	}

	query := openapi.KeygenResponse{
		KeygenBlock: openapi.KeygenBlock{
			Height:  wrapInt64(keygenBlock.Height),
			Keygens: keygens,
		},
		Signature: base64.StdEncoding.EncodeToString(sig),
	}

	return jsonify(ctx, query)
}

func queryKeysign(ctx cosmos.Context, kbs cosmos.KeybaseStore, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	height, err := extractBlockHeight(ctx, path)
	if err != nil {
		return nil, err
	}

	pk := common.EmptyPubKey
	if len(path) > 1 {
		pk, err = common.NewPubKey(path[1])
		if err != nil {
			ctx.Logger().Error("fail to parse pubkey", "error", err)
			return nil, fmt.Errorf("fail to parse pubkey: %w", err)
		}
	}

	txs, err := mgr.Keeper().GetTxOut(ctx, height)
	if err != nil {
		ctx.Logger().Error("fail to get tx out array from key value store", "error", err)
		return nil, fmt.Errorf("fail to get tx out array from key value store: %w", err)
	}

	if !pk.IsEmpty() {
		newTxs := &TxOut{
			Height: txs.Height,
		}
		for _, tx := range txs.TxArray {
			if pk.Equals(tx.VaultPubKey) {
				newTxs.TxArray = append(newTxs.TxArray, tx)
			}
		}
		txs = newTxs
	}

	buf, err := json.Marshal(txs)
	if err != nil {
		ctx.Logger().Error("fail to marshal keysign block to json", "error", err)
		return nil, fmt.Errorf("fail to marshal keysign block to json: %w", err)
	}
	sig, _, err := kbs.Keybase.Sign("mayachain", buf)
	if err != nil {
		ctx.Logger().Error("fail to sign keysign", "error", err)
		return nil, fmt.Errorf("fail to sign keysign: %w", err)
	}

	// TODO: use openapi type after Bifrost uses the same so signatures match.
	type QueryKeysign struct {
		Keysign   TxOut  `json:"keysign"`
		Signature string `json:"signature"`
	}

	query := QueryKeysign{
		Keysign:   *txs,
		Signature: base64.StdEncoding.EncodeToString(sig),
	}

	return jsonify(ctx, query)
}

// queryOutQueue - iterates over txout, counting how many transactions are waiting to be sent
func queryQueue(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	constAccessor := mgr.GetConstants()
	signingTransactionPeriod := constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
	startHeight := ctx.BlockHeight() - signingTransactionPeriod
	var query openapi.QueueResponse
	scheduledOutboundValue := cosmos.ZeroUint()

	iterator := mgr.Keeper().GetSwapQueueIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var msg MsgSwap
		if err := mgr.Keeper().Cdc().Unmarshal(iterator.Value(), &msg); err != nil {
			continue
		}
		query.Swap++
	}

	iter2 := mgr.Keeper().GetOrderBookItemIterator(ctx)
	defer iter2.Close()
	for ; iter2.Valid(); iter2.Next() {
		var msg MsgSwap
		if err := mgr.Keeper().Cdc().Unmarshal(iterator.Value(), &msg); err != nil {
			ctx.Logger().Error("failed to load MsgSwap", "error", err)
			continue
		}
		query.Swap++
	}

	for height := startHeight; height <= ctx.BlockHeight(); height++ {
		txs, err := mgr.Keeper().GetTxOut(ctx, height)
		if err != nil {
			ctx.Logger().Error("fail to get tx out array from key value store", "error", err)
			return nil, fmt.Errorf("fail to get tx out array from key value store: %w", err)
		}
		for _, tx := range txs.TxArray {
			if tx.OutHash.IsEmpty() {
				memo, _ := ParseMemoWithMAYANames(ctx, mgr.Keeper(), tx.Memo)
				if memo.IsInternal() {
					query.Internal++
				} else if memo.IsOutbound() {
					query.Outbound++
				}
			}
		}
	}

	// sum outbound value
	maxTxOutOffset, err := mgr.Keeper().GetMimir(ctx, constants.MaxTxOutOffset.String())
	if maxTxOutOffset < 0 || err != nil {
		maxTxOutOffset = constAccessor.GetInt64Value(constants.MaxTxOutOffset)
	}
	txOutDelayMax, err := mgr.Keeper().GetMimir(ctx, constants.TxOutDelayMax.String())
	if txOutDelayMax <= 0 || err != nil {
		txOutDelayMax = constAccessor.GetInt64Value(constants.TxOutDelayMax)
	}

	for height := ctx.BlockHeight() + 1; height <= ctx.BlockHeight()+txOutDelayMax; height++ {
		value, err := mgr.Keeper().GetTxOutValue(ctx, height)
		if err != nil {
			ctx.Logger().Error("fail to get tx out array from key value store", "error", err)
			continue
		}
		if height > ctx.BlockHeight()+maxTxOutOffset && value.IsZero() {
			// we've hit our max offset, and an empty block, we can assume the
			// rest will be empty as well
			break
		}
		scheduledOutboundValue = scheduledOutboundValue.Add(value)
	}

	query.ScheduledOutboundValue = scheduledOutboundValue.String()

	return jsonify(ctx, query)
}

func queryLastBlockHeights(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	var chains common.Chains
	if len(path) > 0 && len(path[0]) > 0 {
		var err error
		chain, err := common.NewChain(path[0])
		if err != nil {
			ctx.Logger().Error("fail to parse chain", "error", err, "chain", path[0])
			return nil, fmt.Errorf("fail to retrieve chain: %w", err)
		}
		chains = append(chains, chain)
	} else {
		asgards, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
		if err != nil {
			return nil, fmt.Errorf("fail to get active asgard: %w", err)
		}
		for _, vault := range asgards {
			chains = vault.GetChains().Distinct()
			break
		}
	}
	var result []openapi.LastBlock
	for _, c := range chains {
		if c == common.BASEChain {
			continue
		}
		chainHeight, err := mgr.Keeper().GetLastChainHeight(ctx, c)
		if err != nil {
			return nil, fmt.Errorf("fail to get last chain height: %w", err)
		}

		signed, err := mgr.Keeper().GetLastSignedHeight(ctx)
		if err != nil {
			return nil, fmt.Errorf("fail to get last sign height: %w", err)
		}
		result = append(result, openapi.LastBlock{
			Chain:          c.String(),
			LastObservedIn: chainHeight,
			LastSignedOut:  signed,
			Mayachain:      ctx.BlockHeight(),
		})
	}

	return jsonify(ctx, result)
}

func queryConstantValues(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	constAccessor := mgr.GetConstants()
	return jsonify(ctx, constAccessor)
}

func queryVersion(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	v, hasV := mgr.Keeper().GetVersionWithCtx(ctx)
	if !hasV {
		// re-compute version if not stored
		v = mgr.Keeper().GetLowestActiveVersion(ctx)
	}
	ver := openapi.VersionResponse{
		Current: v.String(),
		Next:    mgr.Keeper().GetMinJoinVersion(ctx).String(),
		Querier: constants.SWVersion.String(),
	}
	return jsonify(ctx, ver)
}

func queryMimirWithKey(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	if len(path) == 0 && len(path[0]) == 0 {
		return nil, fmt.Errorf("no mimir key")
	}
	v, err := mgr.Keeper().GetMimir(ctx, path[0])
	if err != nil {
		return nil, fmt.Errorf("fail to get mimir with key:%s, err : %w", path[0], err)
	}
	return jsonify(ctx, v)
}

func queryMimirValues(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	values := make(map[string]int64)

	// collect keys
	iter := mgr.Keeper().GetMimirIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		k := strings.TrimPrefix(string(iter.Key()), "mimir//")
		values[k] = 0
	}
	iterNode := mgr.Keeper().GetNodeMimirIterator(ctx)
	defer iterNode.Close()
	for ; iterNode.Valid(); iterNode.Next() {
		k := strings.TrimPrefix(string(iterNode.Key()), "nodemimir//")
		values[k] = 0
	}

	// analyze-ignore(map-iteration)
	for k := range values {
		v, err := mgr.Keeper().GetMimir(ctx, k)
		if err != nil {
			return nil, fmt.Errorf("fail to get mimir, err: %w", err)
		}
		// v from GetMimir is of type int64.
		if v == -1 {
			// This key has node votes but no node consensus or Admin-set value,
			// so do not display its unset status in the Mimir endpoint.
			delete(values, k)
			continue
		}
		values[k] = v
	}

	return jsonify(ctx, values)
}

func queryMimirAdminValues(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	values := make(map[string]int64)
	iter := mgr.Keeper().GetMimirIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		value := types.ProtoInt64{}
		if err := mgr.Keeper().Cdc().Unmarshal(iter.Value(), &value); err != nil {
			ctx.Logger().Error("fail to unmarshal mimir value", "error", err)
			return nil, fmt.Errorf("fail to unmarshal mimir value: %w", err)
		}
		k := strings.TrimPrefix(string(iter.Key()), "mimir//")
		values[k] = value.GetValue()
	}
	return jsonify(ctx, values)
}

func queryMimirNodesAllValues(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	mimirs := NodeMimirs{}
	iter := mgr.Keeper().GetNodeMimirIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		m := NodeMimirs{}
		if err := mgr.Keeper().Cdc().Unmarshal(iter.Value(), &m); err != nil {
			ctx.Logger().Error("fail to unmarshal node mimir value", "error", err)
			return nil, fmt.Errorf("fail to unmarshal node mimir value: %w", err)
		}
		mimirs.Mimirs = append(mimirs.Mimirs, m.Mimirs...)
	}

	return jsonify(ctx, mimirs)
}

func queryMimirNodesValues(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	activeNodes, err := mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		ctx.Logger().Error("fail to fetch active node accounts", "error", err)
		return nil, fmt.Errorf("fail to fetch active node accounts: %w", err)
	}
	active := activeNodes.GetNodeAddresses()

	values := make(map[string]int64)
	iter := mgr.Keeper().GetNodeMimirIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		mimirs := NodeMimirs{}
		if err = mgr.Keeper().Cdc().Unmarshal(iter.Value(), &mimirs); err != nil {
			ctx.Logger().Error("fail to unmarshal node mimir value", "error", err)
			return nil, fmt.Errorf("fail to unmarshal node mimir value: %w", err)
		}
		k := strings.TrimPrefix(string(iter.Key()), "nodemimir//")
		if v, ok := mimirs.HasSuperMajority(k, active); ok {
			values[k] = v
		}
	}

	return jsonify(ctx, values)
}

func queryMimirNodeValues(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	acc, err := cosmos.AccAddressFromBech32(path[0])
	if err != nil {
		ctx.Logger().Error("fail to parse thor address", "error", err)
		return nil, fmt.Errorf("fail to parse thor address: %w", err)
	}

	values := make(map[string]int64)
	iter := mgr.Keeper().GetNodeMimirIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		mimirs := NodeMimirs{}
		if err = mgr.Keeper().Cdc().Unmarshal(iter.Value(), &mimirs); err != nil {
			ctx.Logger().Error("fail to unmarshal node mimir value", "error", err)
			return nil, fmt.Errorf("fail to unmarshal node mimir value: %w", err)
		}

		k := strings.TrimPrefix(string(iter.Key()), "nodemimir//")
		if v, ok := mimirs.Get(k, acc); ok {
			values[k] = v
		}
	}

	res, err := json.MarshalIndent(values, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal mimir values to json", "error", err)
		return nil, fmt.Errorf("fail to marshal mimir values to json: %w", err)
	}
	return res, nil
}

func queryBan(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("node address not available")
	}
	addr, err := cosmos.AccAddressFromBech32(path[0])
	if err != nil {
		ctx.Logger().Error("invalid node address", "error", err)
		return nil, fmt.Errorf("invalid node address: %w", err)
	}

	ban, err := mgr.Keeper().GetBanVoter(ctx, addr)
	if err != nil {
		ctx.Logger().Error("fail to get ban voter", "error", err)
		return nil, fmt.Errorf("fail to get ban voter: %w", err)
	}

	res, err := json.MarshalIndent(ban, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal ban voter to json", "error", err)
		return nil, fmt.Errorf("fail to ban voter to json: %w", err)
	}
	return res, nil
}

func queryScheduledOutbound(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	result := make([]openapi.TxOutItem, 0)
	constAccessor := mgr.GetConstants()
	maxTxOutOffset, err := mgr.Keeper().GetMimir(ctx, constants.MaxTxOutOffset.String())
	if maxTxOutOffset < 0 || err != nil {
		maxTxOutOffset = constAccessor.GetInt64Value(constants.MaxTxOutOffset)
	}
	for height := ctx.BlockHeight() + 1; height <= ctx.BlockHeight()+17280; height++ {
		txOut, err := mgr.Keeper().GetTxOut(ctx, height)
		if err != nil {
			ctx.Logger().Error("fail to get tx out array from key value store", "error", err)
			continue
		}
		if height > ctx.BlockHeight()+maxTxOutOffset && len(txOut.TxArray) == 0 {
			// we've hit our max offset, and an empty block, we can assume the
			// rest will be empty as well
			break
		}
		for _, toi := range txOut.TxArray {
			result = append(result, castTxOutItem(toi, height))
		}
	}

	return jsonify(ctx, result)
}

func queryPendingOutbound(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	constAccessor := mgr.GetConstants()
	signingTransactionPeriod := constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
	rescheduleCoalesceBlocks := mgr.Keeper().GetConfigInt64(ctx, constants.RescheduleCoalesceBlocks)
	startHeight := ctx.BlockHeight() - signingTransactionPeriod
	if startHeight < 1 {
		startHeight = 1
	}

	// outbounds can be rescheduled to a future height which is the rounded-up nearest multiple of reschedule coalesce blocks
	lastOutboundHeight := ctx.BlockHeight()
	if rescheduleCoalesceBlocks > 1 {
		overBlocks := lastOutboundHeight % rescheduleCoalesceBlocks
		if overBlocks != 0 {
			lastOutboundHeight += rescheduleCoalesceBlocks - overBlocks
		}
	}

	result := make([]openapi.TxOutItem, 0)
	for height := startHeight; height <= lastOutboundHeight; height++ {
		txs, err := mgr.Keeper().GetTxOut(ctx, height)
		if err != nil {
			ctx.Logger().Error("fail to get tx out array from key value store", "error", err)
			return nil, fmt.Errorf("fail to get tx out array from key value store: %w", err)
		}
		for _, tx := range txs.TxArray {
			if tx.OutHash.IsEmpty() {
				result = append(result, castTxOutItem(tx, height))
			}
		}
	}

	return jsonify(ctx, result)
}

func querySwapQueue(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	result := make([]openapi.MsgSwap, 0)

	iterator := mgr.Keeper().GetSwapQueueIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var msg MsgSwap
		if err := mgr.Keeper().Cdc().Unmarshal(iterator.Value(), &msg); err != nil {
			continue
		}
		result = append(result, castMsgSwap(msg))
	}

	return jsonify(ctx, result)
}

func queryTssKeygenMetric(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	var pubKeys common.PubKeys
	if len(path) > 0 {
		pkey, err := common.NewPubKey(path[0])
		if err != nil {
			return nil, fmt.Errorf("fail to parse pubkey(%s) err:%w", path[0], err)
		}
		pubKeys = append(pubKeys, pkey)
	}
	var result []*types.TssKeygenMetric
	for _, pkey := range pubKeys {
		m, err := mgr.Keeper().GetTssKeygenMetric(ctx, pkey)
		if err != nil {
			return nil, fmt.Errorf("fail to get tss keygen metric for pubkey(%s):%w", pkey, err)
		}
		result = append(result, m)
	}
	res, err := json.MarshalIndent(result, "", "	")
	if err != nil {
		return nil, fmt.Errorf("fail to marshal keygen metrics to json: %w", err)
	}
	return res, nil
}

func queryTssMetric(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	var pubKeys common.PubKeys
	// get all active asgard
	vaults, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		return nil, fmt.Errorf("fail to get active asgards:%w", err)
	}
	for _, v := range vaults {
		pubKeys = append(pubKeys, v.PubKey)
	}
	var keygenMetrics []*types.TssKeygenMetric
	for _, pkey := range pubKeys {
		var m *types.TssKeygenMetric
		m, err = mgr.Keeper().GetTssKeygenMetric(ctx, pkey)
		if err != nil {
			return nil, fmt.Errorf("fail to get tss keygen metric for pubkey(%s):%w", pkey, err)
		}
		if len(m.NodeTssTimes) == 0 {
			continue
		}
		keygenMetrics = append(keygenMetrics, m)
	}
	keysignMetric, err := mgr.Keeper().GetLatestTssKeysignMetric(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get keysign metric:%w", err)
	}
	m := struct {
		KeygenMetrics []*types.TssKeygenMetric `json:"keygen"`
		KeysignMetric *types.TssKeysignMetric  `json:"keysign"`
	}{
		KeygenMetrics: keygenMetrics,
		KeysignMetric: keysignMetric,
	}
	return json.MarshalIndent(m, "", "	")
}

// queryLiquidityAuctionTier
func queryLiquidityAuctionTier(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	if len(path) < 2 {
		return nil, errors.New("address not provided")
	}

	asset, err := common.NewAsset(path[0])
	if err != nil {
		ctx.Logger().Error("fail to get parse asset", "error", err)
		return nil, fmt.Errorf("fail to parse asset: %w", err)
	}

	addr, err := common.NewAddress(path[1], mgr.GetVersion())
	if err != nil {
		ctx.Logger().Error("fail to get parse address", "error", err)
		return nil, fmt.Errorf("fail to parse address: %w", err)
	}

	tier, err := mgr.Keeper().GetLiquidityAuctionTier(ctx, addr)
	if err != nil {
		ctx.Logger().Error("fail to get tier", "error", err)
		return nil, fmt.Errorf("fail to tier: %w", err)
	}

	lp, err := mgr.Keeper().GetLiquidityProvider(ctx, asset, addr)
	if err != nil {
		ctx.Logger().Error("fail to get liquidity provider", "error", err)
		return nil, fmt.Errorf("fail to liquidity provider: %w", err)
	}

	liquidityAuction, err := mgr.Keeper().GetMimir(ctx, constants.LiquidityAuction.String())
	if err != nil {
		ctx.Logger().Error("fail to get liquidity auction mimir", "error", err)
		return nil, fmt.Errorf("fail to liquidity auction mimir: %w", err)
	}

	height := ctx.BlockHeight()
	withdrawDays := int64(0)
	withdrawLimitStopBlock := int64(0)
	if liquidityAuction > 0 && height > liquidityAuction {
		switch tier {
		case mgr.GetConstants().GetInt64Value(constants.WithdrawTier1):
			withdrawDays = mgr.Keeper().GetConfigInt64(ctx, constants.WithdrawDaysTier1)
		case mgr.GetConstants().GetInt64Value(constants.WithdrawTier2):
			withdrawDays = mgr.Keeper().GetConfigInt64(ctx, constants.WithdrawDaysTier2)
		case mgr.GetConstants().GetInt64Value(constants.WithdrawTier3):
			withdrawDays = mgr.Keeper().GetConfigInt64(ctx, constants.WithdrawDaysTier3)
		}
		withdrawLimitStopBlock = liquidityAuction + withdrawDays
	}

	m := struct {
		Address                common.Address          `json:"address"`
		Tier                   int64                   `json:"tier"`
		LiquidityProvider      types.LiquidityProvider `json:"liquidity_provider"`
		WithdrawLimitStopBlock int64                   `json:"withdraw_limit_stop_block"`
	}{
		Address:                addr,
		Tier:                   tier,
		LiquidityProvider:      lp,
		WithdrawLimitStopBlock: withdrawLimitStopBlock,
	}

	res, err := json.MarshalIndent(m, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal tier to json", "error", err)
		return nil, fmt.Errorf("fail to marshal tier to json: %w", err)
	}
	return res, nil
}

func queryInvariants(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	result := openapi.InvariantsResponse{}
	for _, route := range mgr.Keeper().InvariantRoutes() {
		result.Invariants = append(result.Invariants, route.Route)
	}
	return jsonify(ctx, result)
}

func queryInvariant(ctx cosmos.Context, path []string, mgr *Mgrs) ([]byte, error) {
	if len(path) < 1 {
		return nil, fmt.Errorf("invalid path: %v", path)
	}
	for _, route := range mgr.Keeper().InvariantRoutes() {
		if strings.EqualFold(route.Route, path[0]) {
			msg, broken := route.Invariant(ctx)
			result := openapi.InvariantResponse{
				Invariant: route.Route,
				Broken:    broken,
				Msg:       msg,
			}
			return jsonify(ctx, result)
		}
	}
	return nil, fmt.Errorf("invariant not registered: %s", path[0])
}

func queryBlock(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	initTendermintOnce.Do(initTendermint)
	height := ctx.BlockHeight()

	// get the block and results from tendermint rpc
	block, err := tendermintClient.Block(ctx.Context(), &height)
	if err != nil {
		return nil, fmt.Errorf("fail to get block from tendermint rpc: %w", err)
	}
	results, err := tendermintClient.BlockResults(ctx.Context(), &height)
	if err != nil {
		return nil, fmt.Errorf("fail to get block results from tendermint rpc: %w", err)
	}

	res := types.QueryBlockResponse{
		BlockResponse: openapi.BlockResponse{
			Id: openapi.BlockResponseId{
				Hash: block.BlockID.Hash.String(),
				Parts: openapi.BlockResponseIdParts{
					Total: int64(block.BlockID.PartSetHeader.Total),
					Hash:  block.BlockID.PartSetHeader.Hash.String(),
				},
			},
			Header: openapi.BlockResponseHeader{
				Version: openapi.BlockResponseHeaderVersion{
					Block: strconv.FormatUint(block.Block.Header.Version.Block, 10),
					App:   strconv.FormatUint(block.Block.Header.Version.App, 10),
				},
				ChainId: block.Block.Header.ChainID,
				Height:  block.Block.Header.Height,
				Time:    block.Block.Header.Time.Format(time.RFC3339Nano),
				LastBlockId: openapi.BlockResponseId{
					Hash: block.Block.Header.LastBlockID.Hash.String(),
					Parts: openapi.BlockResponseIdParts{
						Total: int64(block.Block.Header.LastBlockID.PartSetHeader.Total),
						Hash:  block.Block.Header.LastBlockID.PartSetHeader.Hash.String(),
					},
				},
				LastCommitHash:     block.Block.Header.LastCommitHash.String(),
				DataHash:           block.Block.Header.DataHash.String(),
				ValidatorsHash:     block.Block.Header.ValidatorsHash.String(),
				NextValidatorsHash: block.Block.Header.NextValidatorsHash.String(),
				ConsensusHash:      block.Block.Header.ConsensusHash.String(),
				AppHash:            block.Block.Header.AppHash.String(),
				LastResultsHash:    block.Block.Header.LastResultsHash.String(),
				EvidenceHash:       block.Block.Header.EvidenceHash.String(),
				ProposerAddress:    block.Block.Header.ProposerAddress.String(),
			},
			BeginBlockEvents: []map[string]string{},
			EndBlockEvents:   []map[string]string{},
		},
		Txs: make([]types.QueryBlockTx, len(block.Block.Txs)),
	}

	// parse the events
	for _, event := range results.BeginBlockEvents {
		res.BeginBlockEvents = append(res.BeginBlockEvents, eventMap(sdk.Event(event)))
	}
	for _, event := range results.EndBlockEvents {
		res.EndBlockEvents = append(res.EndBlockEvents, eventMap(sdk.Event(event)))
	}

	for i, tx := range block.Block.Txs {
		res.Txs[i].Hash = strings.ToUpper(hex.EncodeToString(tx.Hash()))

		// decode the protobuf and encode to json
		dtx, err := authtx.DefaultTxDecoder(mgr.cdc.(*codec.ProtoCodec))(tx)
		if err != nil {
			return nil, fmt.Errorf("fail to decode tx: %w", err)
		}
		res.Txs[i].Tx, err = authtx.DefaultJSONTxEncoder(mgr.cdc.(*codec.ProtoCodec))(dtx)
		if err != nil {
			return nil, fmt.Errorf("fail to encode tx: %w", err)
		}

		// parse the tx events
		code := int64(results.TxsResults[i].Code)
		res.Txs[i].Result.Code = &code
		res.Txs[i].Result.Data = wrapString(string(results.TxsResults[i].Data))
		res.Txs[i].Result.Log = wrapString(results.TxsResults[i].Log)
		res.Txs[i].Result.Info = wrapString(results.TxsResults[i].Info)
		res.Txs[i].Result.GasWanted = wrapString(strconv.FormatInt(results.TxsResults[i].GasWanted, 10))
		res.Txs[i].Result.GasUsed = wrapString(strconv.FormatInt(results.TxsResults[i].GasUsed, 10))
		res.Txs[i].Result.Events = []map[string]string{}
		for _, event := range results.TxsResults[i].Events {
			res.Txs[i].Result.Events = append(res.Txs[i].Result.Events, eventMap(sdk.Event(event)))
		}
	}

	return jsonify(ctx, res)
}

// -------------------------------------------------------------------------------------
// Generic Helpers
// -------------------------------------------------------------------------------------

func wrapBool(b bool) *bool {
	if !b {
		return nil
	}
	return &b
}

func wrapString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func wrapInt64(d int64) *int64 {
	if d == 0 {
		return nil
	}
	return &d
}

func wrapUintPtr(uintPtr *cosmos.Uint) *string {
	if uintPtr == nil {
		return nil
	}
	return wrapString(uintPtr.String())
}

func castCoin(sourceCoin common.Coin) openapi.Coin {
	return openapi.Coin{
		Asset:    sourceCoin.Asset.String(),
		Amount:   sourceCoin.Amount.String(),
		Decimals: wrapInt64(sourceCoin.Decimals),
	}
}

func castCoins(sourceCoins ...common.Coin) []openapi.Coin {
	// Leave this nil (null rather than []) if the source is nil.
	if sourceCoins == nil {
		return nil
	}

	coins := make([]openapi.Coin, len(sourceCoins))
	for i := range sourceCoins {
		coins[i] = castCoin(sourceCoins[i])
	}
	return coins
}

func castTxOutItem(toi TxOutItem, height int64) openapi.TxOutItem {
	return openapi.TxOutItem{
		Chain:                 toi.Chain.String(),
		ToAddress:             toi.ToAddress.String(),
		VaultPubKey:           wrapString(toi.VaultPubKey.String()),
		Coin:                  castCoin(toi.Coin),
		Memo:                  wrapString(toi.Memo),
		MaxGas:                castCoins(toi.MaxGas...),
		GasRate:               wrapInt64(toi.GasRate),
		InHash:                wrapString(toi.InHash.String()),
		OutHash:               wrapString(toi.OutHash.String()),
		Aggregator:            wrapString(toi.Aggregator),
		AggregatorTargetAsset: wrapString(toi.AggregatorTargetAsset),
		AggregatorTargetLimit: wrapUintPtr(toi.AggregatorTargetLimit),
		Height:                wrapInt64(height), // Omitted if 0, for use in openapi.TxDetailsResponse
	}
}

func castTx(tx common.Tx) openapi.Tx {
	return openapi.Tx{
		Id:          wrapString(tx.ID.String()),
		Chain:       wrapString(tx.Chain.String()),
		FromAddress: wrapString(tx.FromAddress.String()),
		ToAddress:   wrapString(tx.ToAddress.String()),
		Coins:       castCoins(tx.Coins...),
		Gas:         castCoins(tx.Gas...),
		Memo:        wrapString(tx.Memo),
	}
}

func castObservedTx(observedTx ObservedTx) openapi.ObservedTx {
	// Only display the Status if it is "done", not if "incomplete".
	var status *string
	if observedTx.Status != types.Status_incomplete {
		status = wrapString(observedTx.Status.String())
	}

	return openapi.ObservedTx{
		Tx:                              castTx(observedTx.Tx),
		ObservedPubKey:                  wrapString(observedTx.ObservedPubKey.String()),
		ExternalObservedHeight:          wrapInt64(observedTx.BlockHeight),
		ExternalConfirmationDelayHeight: wrapInt64(observedTx.FinaliseHeight),
		Aggregator:                      wrapString(observedTx.Aggregator),
		AggregatorTarget:                wrapString(observedTx.AggregatorTarget),
		AggregatorTargetLimit:           wrapUintPtr(observedTx.AggregatorTargetLimit),
		Signers:                         observedTx.Signers,
		KeysignMs:                       wrapInt64(observedTx.KeysignMs),
		OutHashes:                       observedTx.OutHashes,
		Status:                          status,
	}
}

func castMsgSwap(msg MsgSwap) openapi.MsgSwap {
	// Only display the OrderType if it is "limit", not if "market".
	var orderType *string
	if msg.OrderType != types.OrderType_market {
		orderType = wrapString(msg.OrderType.String())
	}
	// TODO: After order books implementation,
	// always display the OrderType?

	return openapi.MsgSwap{
		Tx:                      castTx(msg.Tx),
		TargetAsset:             msg.TargetAsset.String(),
		Destination:             wrapString(msg.Destination.String()),
		TradeTarget:             msg.TradeTarget.String(),
		AffiliateAddress:        wrapString(msg.AffiliateAddress.String()),
		AffiliateBasisPoints:    msg.AffiliateBasisPoints.String(),
		Signer:                  wrapString(msg.Signer.String()),
		Aggregator:              wrapString(msg.Aggregator),
		AggregatorTargetAddress: wrapString(msg.AggregatorTargetAddress),
		AggregatorTargetLimit:   wrapUintPtr(msg.AggregatorTargetLimit),
		OrderType:               orderType,
		StreamQuantity:          wrapInt64(int64(msg.StreamQuantity)),
		StreamInterval:          wrapInt64(int64(msg.StreamInterval)),
	}
}

func castVaultRouters(chainContracts []ChainContract) []openapi.VaultRouter {
	// Leave this nil (null rather than []) if the source is nil.
	if chainContracts == nil {
		return nil
	}

	routers := make([]openapi.VaultRouter, len(chainContracts))
	for i := range chainContracts {
		routers[i] = openapi.VaultRouter{
			Chain:  wrapString(chainContracts[i].Chain.String()),
			Router: wrapString(chainContracts[i].Router.String()),
		}
	}
	return routers
}

// TODO: Migrate callers to use simulate instead.
func simulateInternal(ctx cosmos.Context, mgr *Mgrs, msg sdk.Msg) (sdk.Events, error) {
	// validate
	err := msg.ValidateBasic()
	if err != nil {
		return nil, fmt.Errorf("failed validate: %w", err)
	}

	// intercept events and avoid modifying state
	cms := ctx.MultiStore().CacheMultiStore() // never call cms.Write()
	em := cosmos.NewEventManager()
	ctx = ctx.WithMultiStore(cms).WithEventManager(em)

	// disable logging
	// ctx = ctx.WithLogger(nullLogger)
	ctx = ctx.WithLogger(ctx.Logger().With("mode", "*** Simulate Internal ***"))

	// simulate the message handler
	_, err = NewInternalHandler(mgr)(ctx, msg)
	return em.Events(), err
}

func eventMap(e sdk.Event) map[string]string {
	m := map[string]string{}
	m["type"] = e.Type
	for _, a := range e.Attributes {
		m[string(a.Key)] = string(a.Value)
	}
	return m
}
