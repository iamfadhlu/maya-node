package keeper

import (
	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/codec"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	ibctransferkeeper "github.com/cosmos/ibc-go/v2/modules/apps/transfer/keeper"
	ibctransfertypes "github.com/cosmos/ibc-go/v2/modules/apps/transfer/types"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	kvTypes "gitlab.com/mayachain/mayanode/x/mayachain/keeper/types"
	kv1 "gitlab.com/mayachain/mayanode/x/mayachain/keeper/v1"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

type Keeper interface {
	Cdc() codec.BinaryCodec
	GetVersion() semver.Version
	GetVersionWithCtx(ctx cosmos.Context) (semver.Version, bool)
	SetVersionWithCtx(ctx cosmos.Context, v semver.Version)
	GetKey(ctx cosmos.Context, prefix kvTypes.DbPrefix, key string) string
	GetStoreVersion(ctx cosmos.Context) int64
	SetStoreVersion(ctx cosmos.Context, ver int64)
	GetRuneBalanceOfModule(ctx cosmos.Context, moduleName string) cosmos.Uint
	GetBalanceOfModule(ctx cosmos.Context, moduleName, denom string) cosmos.Uint
	SendFromModuleToModule(ctx cosmos.Context, from, to string, coin common.Coins) error
	SendFromAccountToModule(ctx cosmos.Context, from cosmos.AccAddress, to string, coin common.Coins) error
	SendFromModuleToAccount(ctx cosmos.Context, from string, to cosmos.AccAddress, coin common.Coins) error
	MintToModule(ctx cosmos.Context, module string, coin common.Coin) error
	BurnFromModule(ctx cosmos.Context, module string, coin common.Coin) error
	MintAndSendToAccount(ctx cosmos.Context, to cosmos.AccAddress, coin common.Coin) error
	GetModuleAddress(module string) (common.Address, error)
	GetModuleAccAddress(module string) cosmos.AccAddress
	GetBalance(ctx cosmos.Context, addr cosmos.AccAddress) cosmos.Coins
	HasCoins(ctx cosmos.Context, addr cosmos.AccAddress, coins cosmos.Coins) bool
	GetAccount(ctx cosmos.Context, addr cosmos.AccAddress) cosmos.Account
	GetIBCTransferParams(ctx cosmos.Context) ibctransfertypes.Params
	SetIBCTransferParams(ctx cosmos.Context, params ibctransfertypes.Params)

	// passthrough funcs
	SendCoins(ctx cosmos.Context, from, to cosmos.AccAddress, coins cosmos.Coins) error
	AddCoins(ctx cosmos.Context, addr cosmos.AccAddress, coins cosmos.Coins) error

	InvariantRoutes() []common.InvariantRoute

	GetConstants() constants.ConstantValues
	GetConfigInt64(ctx cosmos.Context, key constants.ConstantName) int64

	// Keeper Interfaces
	KeeperConfig
	KeeperPool
	KeeperLastHeight
	KeeperLiquidityProvider
	KeeperNodeAccount
	KeeperObserver
	KeeperObservedTx
	KeeperTxOut
	KeeperLiquidityFees
	KeeperVault
	KeeperReserveContributors
	KeeperNetwork
	KeeperTss
	KeeperTssKeysignFail
	KeeperKeygen
	KeeperRagnarok
	KeeperErrataTx
	KeeperBanVoter
	KeeperSwapQueue
	KeeperOrderBooks
	KeeperMimir
	KeeperNetworkFee
	KeeperObservedNetworkFeeVoter
	KeeperChainContract
	KeeperSolvencyVoter
	KeeperMAYAName
	KeeperForgiveSlashVoter
	KeeperStreamingSwap
	KeeperCACAOPool
	KeeperTradeAccount
}

type KeeperConfig interface {
	GetConstants() constants.ConstantValues
	GetConfigInt64(ctx cosmos.Context, key constants.ConstantName) int64
}
type KeeperPool interface {
	GetPoolIterator(ctx cosmos.Context) cosmos.Iterator
	GetPool(ctx cosmos.Context, asset common.Asset) (Pool, error)
	GetPools(ctx cosmos.Context) (Pools, error)
	SetPool(ctx cosmos.Context, pool Pool) error
	PoolExist(ctx cosmos.Context, asset common.Asset) bool
	RemovePool(ctx cosmos.Context, asset common.Asset)
	SetPoolLUVI(ctx cosmos.Context, asset common.Asset, luvi cosmos.Uint)
	GetPoolLUVI(ctx cosmos.Context, asset common.Asset) (cosmos.Uint, error)
}

type KeeperStreamingSwap interface {
	GetStreamingSwapIterator(ctx cosmos.Context) cosmos.Iterator
	SetStreamingSwap(ctx cosmos.Context, _ StreamingSwap)
	GetStreamingSwap(ctx cosmos.Context, _ common.TxID) (StreamingSwap, error)
	StreamingSwapExists(ctx cosmos.Context, _ common.TxID) bool
	RemoveStreamingSwap(ctx cosmos.Context, _ common.TxID)
}

type KeeperLastHeight interface {
	SetLastSignedHeight(ctx cosmos.Context, height int64) error
	GetLastSignedHeight(ctx cosmos.Context) (int64, error)
	SetLastChainHeight(ctx cosmos.Context, chain common.Chain, height int64) error
	GetLastChainHeight(ctx cosmos.Context, chain common.Chain) (int64, error)
	GetLastChainHeights(ctx cosmos.Context) (map[common.Chain]int64, error)
	SetLastObserveHeight(ctx cosmos.Context, chain common.Chain, address cosmos.AccAddress, height int64) error
	GetLastObserveHeight(ctx cosmos.Context, address cosmos.AccAddress) (map[common.Chain]int64, error)
	ForceSetLastObserveHeight(ctx cosmos.Context, chain common.Chain, address cosmos.AccAddress, height int64) error
	ForceSetLastChainHeight(ctx cosmos.Context, chain common.Chain, height int64) error
}

type KeeperLiquidityProvider interface {
	GetLiquidityProviderIterator(ctx cosmos.Context, _ common.Asset) cosmos.Iterator
	GetLiquidityProvider(ctx cosmos.Context, asset common.Asset, addr common.Address) (LiquidityProvider, error)
	GetLiquidityProviderByAssets(ctx cosmos.Context, assets common.Assets, addr common.Address) (LiquidityProviders, error)
	GetLiquidityAuctionTier(ctx cosmos.Context, addr common.Address) (int64, error)
	SetLiquidityProvider(ctx cosmos.Context, lp LiquidityProvider)
	SetLiquidityProviders(ctx cosmos.Context, lps LiquidityProviders)
	SetLiquidityAuctionTier(ctx cosmos.Context, addr common.Address, tier int64) error
	RemoveLiquidityProvider(ctx cosmos.Context, lp LiquidityProvider)
	GetTotalSupply(ctx cosmos.Context, asset common.Asset) cosmos.Uint
	CalcLPLiquidityBond(ctx cosmos.Context, bondAddr common.Address, nodeAddr cosmos.AccAddress) (cosmos.Uint, error)
	CalcTotalBondableLiquidity(ctx cosmos.Context, addr common.Address) (cosmos.Uint, error)
	GetLPLiquidityBond(ctx cosmos.Context, bondAddr common.Address, nodeAddr cosmos.AccAddress) (cosmos.Uint, LiquidityProviders, error)
}

type KeeperNodeAccount interface {
	TotalActiveValidators(ctx cosmos.Context) (int, error)
	ListValidatorsWithBond(ctx cosmos.Context) (NodeAccounts, error)
	ListValidatorsByStatus(ctx cosmos.Context, status NodeStatus) (NodeAccounts, error)
	ListValidatorsByStatuses(ctx cosmos.Context, statuses ...NodeStatus) (NodeAccounts, error)
	ListActiveValidators(ctx cosmos.Context) (NodeAccounts, error)
	GetLowestActiveVersion(ctx cosmos.Context) semver.Version
	GetMinJoinVersion(ctx cosmos.Context) semver.Version
	GetNodeAccount(ctx cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error)
	GetNodeAccountByPubKey(ctx cosmos.Context, pk common.PubKey) (NodeAccount, error)
	SetNodeAccount(ctx cosmos.Context, na NodeAccount) error
	EnsureNodeKeysUnique(ctx cosmos.Context, consensusPubKey string, pubKeys common.PubKeySet) error
	EnsureAztecAddressUnique(ctx cosmos.Context, aztecAddress common.Address) error
	GetNodeAccountIterator(ctx cosmos.Context) cosmos.Iterator
	GetNodeAccountSlashPoints(_ cosmos.Context, _ cosmos.AccAddress) (int64, error)
	SetNodeAccountSlashPoints(_ cosmos.Context, _ cosmos.AccAddress, _ int64)
	IncNodeAccountSlashPoints(_ cosmos.Context, _ cosmos.AccAddress, _ int64) error
	DecNodeAccountSlashPoints(_ cosmos.Context, _ cosmos.AccAddress, _ int64) error
	ResetNodeAccountSlashPoints(_ cosmos.Context, _ cosmos.AccAddress)
	GetNodeAccountJail(ctx cosmos.Context, addr cosmos.AccAddress) (Jail, error)
	SetNodeAccountJail(ctx cosmos.Context, addr cosmos.AccAddress, height int64, reason string) error
	ReleaseNodeAccountFromJail(ctx cosmos.Context, addr cosmos.AccAddress) error
	SetBondProviders(ctx cosmos.Context, _ BondProviders) error
	GetBondProviders(ctx cosmos.Context, add cosmos.AccAddress) (BondProviders, error)
	CalcNodeLiquidityBond(ctx cosmos.Context, na NodeAccount) (cosmos.Uint, error)
	CalcNodeBondProvidersLiquidityBond(ctx cosmos.Context, na NodeAccount) (cosmos.Uint, []cosmos.Uint, error)
}

type KeeperObserver interface {
	GetObservingAddresses(ctx cosmos.Context) ([]cosmos.AccAddress, error)
	AddObservingAddresses(ctx cosmos.Context, inAddresses []cosmos.AccAddress) error
	ClearObservingAddresses(ctx cosmos.Context)
}

type KeeperObservedTx interface {
	SetObservedTxInVoter(ctx cosmos.Context, tx ObservedTxVoter)
	GetObservedTxInVoterIterator(ctx cosmos.Context) cosmos.Iterator
	GetObservedTxInVoter(ctx cosmos.Context, hash common.TxID) (ObservedTxVoter, error)
	SetObservedTxOutVoter(ctx cosmos.Context, tx ObservedTxVoter)
	GetObservedTxOutVoterIterator(ctx cosmos.Context) cosmos.Iterator
	GetObservedTxOutVoter(ctx cosmos.Context, hash common.TxID) (ObservedTxVoter, error)
	SetObservedLink(ctx cosmos.Context, _, _ common.TxID)
	GetObservedLink(ctx cosmos.Context, inhash common.TxID) []common.TxID
}

type KeeperTxOut interface {
	SetTxOut(ctx cosmos.Context, blockOut *TxOut) error
	AppendTxOut(ctx cosmos.Context, height int64, item TxOutItem) error
	ClearTxOut(ctx cosmos.Context, height int64) error
	GetTxOutIterator(ctx cosmos.Context) cosmos.Iterator
	GetTxOut(ctx cosmos.Context, height int64) (*TxOut, error)
	GetTxOutValue(ctx cosmos.Context, height int64) (cosmos.Uint, error)
}

type KeeperLiquidityFees interface {
	AddToLiquidityFees(ctx cosmos.Context, asset common.Asset, fee cosmos.Uint) error
	GetTotalLiquidityFees(ctx cosmos.Context, height uint64) (cosmos.Uint, error)
	GetPoolLiquidityFees(ctx cosmos.Context, height uint64, asset common.Asset) (cosmos.Uint, error)
	GetRollingPoolLiquidityFee(ctx cosmos.Context, asset common.Asset) (uint64, error)
	ResetRollingPoolLiquidityFee(ctx cosmos.Context, asset common.Asset)
}

type KeeperCACAOPool interface {
	GetCACAOPool(ctx cosmos.Context) (CACAOPool, error)
	SetCACAOPool(ctx cosmos.Context, pool CACAOPool)
	GetCACAOProviderIterator(ctx cosmos.Context) cosmos.Iterator
	GetCACAOProvider(ctx cosmos.Context, addr cosmos.AccAddress) (CACAOProvider, error)
	SetCACAOProvider(ctx cosmos.Context, rp CACAOProvider)
	RemoveCACAOProvider(ctx cosmos.Context, rp CACAOProvider)
}

type KeeperVault interface {
	GetVaultIterator(ctx cosmos.Context) cosmos.Iterator
	VaultExists(ctx cosmos.Context, pk common.PubKey) bool
	SetVault(ctx cosmos.Context, vault Vault) error
	GetVault(ctx cosmos.Context, pk common.PubKey) (Vault, error)
	HasValidVaultPools(ctx cosmos.Context) (bool, error)
	GetAsgardVaults(ctx cosmos.Context) (Vaults, error)
	GetAsgardVaultsByStatus(_ cosmos.Context, _ VaultStatus) (Vaults, error)
	GetLeastSecure(_ cosmos.Context, _ Vaults, _ int64) Vault
	GetMostSecure(_ cosmos.Context, _ Vaults, _ int64) Vault
	SortBySecurity(_ cosmos.Context, _ Vaults, _ int64) Vaults
	GetPendingOutbounds(_ cosmos.Context, _ common.Asset) []TxOutItem
	DeleteVault(ctx cosmos.Context, pk common.PubKey) error
	RemoveFromAsgardIndex(ctx cosmos.Context, pubkey common.PubKey) error
}

type KeeperReserveContributors interface {
	AddPoolFeeToReserve(ctx cosmos.Context, fee cosmos.Uint) error
	AddBondFeeToReserve(ctx cosmos.Context, fee cosmos.Uint) error
}

// KeeperNetwork func to access network data in key value store
type KeeperNetwork interface {
	GetNetwork(ctx cosmos.Context) (Network, error)
	SetNetwork(ctx cosmos.Context, data Network) error
	GetPOL(ctx cosmos.Context) (ProtocolOwnedLiquidity, error)
	SetPOL(ctx cosmos.Context, data ProtocolOwnedLiquidity) error
}

type KeeperTss interface {
	SetTssVoter(_ cosmos.Context, tss TssVoter)
	GetTssVoterIterator(_ cosmos.Context) cosmos.Iterator
	GetTssVoter(_ cosmos.Context, _ string) (TssVoter, error)
	SetTssKeygenMetric(_ cosmos.Context, metric *TssKeygenMetric)
	GetTssKeygenMetric(_ cosmos.Context, key common.PubKey) (*TssKeygenMetric, error)
	SetTssKeysignMetric(_ cosmos.Context, metric *TssKeysignMetric)
	GetTssKeysignMetric(_ cosmos.Context, txID common.TxID) (*TssKeysignMetric, error)
	GetLatestTssKeysignMetric(_ cosmos.Context) (*TssKeysignMetric, error)
}

type KeeperTssKeysignFail interface {
	SetTssKeysignFailVoter(_ cosmos.Context, tss TssKeysignFailVoter)
	GetTssKeysignFailVoterIterator(_ cosmos.Context) cosmos.Iterator
	GetTssKeysignFailVoter(_ cosmos.Context, _ string) (TssKeysignFailVoter, error)
}

type KeeperKeygen interface {
	SetKeygenBlock(ctx cosmos.Context, keygenBlock KeygenBlock)
	GetKeygenBlockIterator(ctx cosmos.Context) cosmos.Iterator
	GetKeygenBlock(ctx cosmos.Context, height int64) (KeygenBlock, error)
}

type KeeperBanVoter interface {
	SetBanVoter(_ cosmos.Context, _ BanVoter)
	GetBanVoter(_ cosmos.Context, _ cosmos.AccAddress) (BanVoter, error)
	GetBanVoterIterator(_ cosmos.Context) cosmos.Iterator
}

type KeeperForgiveSlashVoter interface {
	SetForgiveSlashVoter(_ cosmos.Context, _ ForgiveSlashVoter)
	GetForgiveSlashVoter(_ cosmos.Context, _ cosmos.AccAddress) (ForgiveSlashVoter, error)
}

type KeeperRagnarok interface {
	RagnarokInProgress(_ cosmos.Context) bool
	GetRagnarokBlockHeight(_ cosmos.Context) (int64, error)
	SetRagnarokBlockHeight(_ cosmos.Context, _ int64)
	GetRagnarokNth(_ cosmos.Context) (int64, error)
	SetRagnarokNth(_ cosmos.Context, _ int64)
	GetRagnarokPending(_ cosmos.Context) (int64, error)
	SetRagnarokPending(_ cosmos.Context, _ int64)
	GetRagnarokWithdrawPosition(ctx cosmos.Context) (RagnarokWithdrawPosition, error)
	SetRagnarokWithdrawPosition(ctx cosmos.Context, position RagnarokWithdrawPosition)
	SetPoolRagnarokStart(ctx cosmos.Context, asset common.Asset)
	GetPoolRagnarokStart(ctx cosmos.Context, asset common.Asset) (int64, error)
}

type KeeperErrataTx interface {
	SetErrataTxVoter(_ cosmos.Context, _ ErrataTxVoter)
	GetErrataTxVoterIterator(_ cosmos.Context) cosmos.Iterator
	GetErrataTxVoter(_ cosmos.Context, _ common.TxID, _ common.Chain) (ErrataTxVoter, error)
}

type KeeperSwapQueue interface {
	SetSwapQueueItem(ctx cosmos.Context, msg MsgSwap, i int) error
	GetSwapQueueIterator(ctx cosmos.Context) cosmos.Iterator
	GetSwapQueueItem(ctx cosmos.Context, txID common.TxID, i int) (MsgSwap, error)
	HasSwapQueueItem(ctx cosmos.Context, txID common.TxID, i int) bool
	RemoveSwapQueueItem(ctx cosmos.Context, txID common.TxID, i int)
}

type KeeperOrderBooks interface {
	SetOrderBookItem(ctx cosmos.Context, msg MsgSwap) error
	GetOrderBookItemIterator(ctx cosmos.Context) cosmos.Iterator
	GetOrderBookItem(ctx cosmos.Context, txID common.TxID) (MsgSwap, error)
	HasOrderBookItem(ctx cosmos.Context, txID common.TxID) bool
	RemoveOrderBookItem(ctx cosmos.Context, txID common.TxID) error
	GetOrderBookIndexIterator(_ cosmos.Context, _ types.OrderType, _, _ common.Asset) cosmos.Iterator
	SetOrderBookIndex(_ cosmos.Context, _ MsgSwap) error
	GetOrderBookIndex(_ cosmos.Context, _ MsgSwap) (common.TxIDs, error)
	HasOrderBookIndex(_ cosmos.Context, _ MsgSwap) (bool, error)
	RemoveOrderBookIndex(_ cosmos.Context, _ MsgSwap) error
	SetOrderBookProcessor(_ cosmos.Context, _ []bool) error
	GetOrderBookProcessor(_ cosmos.Context) ([]bool, error)
}

type KeeperMimir interface {
	GetMimir(_ cosmos.Context, key string) (int64, error)
	SetMimir(_ cosmos.Context, key string, value int64)
	GetNodeMimirs(ctx cosmos.Context, key string) (NodeMimirs, error)
	SetNodeMimir(_ cosmos.Context, key string, value int64, acc cosmos.AccAddress) error
	GetMimirIterator(ctx cosmos.Context) cosmos.Iterator
	GetNodeMimirIterator(ctx cosmos.Context) cosmos.Iterator
	DeleteMimir(_ cosmos.Context, key string) error
	GetNodePauseChain(ctx cosmos.Context, acc cosmos.AccAddress) int64
	SetNodePauseChain(ctx cosmos.Context, acc cosmos.AccAddress)
}

type KeeperNetworkFee interface {
	GetNetworkFee(ctx cosmos.Context, chain common.Chain) (NetworkFee, error)
	SaveNetworkFee(ctx cosmos.Context, chain common.Chain, networkFee NetworkFee) error
	GetNetworkFeeIterator(ctx cosmos.Context) cosmos.Iterator
	DistributeMayaFund(ctx cosmos.Context, constAccessor constants.ConstantValues)
	DynamicInflation(ctx cosmos.Context, constAccessor constants.ConstantValues) error
}

type KeeperObservedNetworkFeeVoter interface {
	SetObservedNetworkFeeVoter(ctx cosmos.Context, networkFeeVoter ObservedNetworkFeeVoter)
	GetObservedNetworkFeeVoterIterator(ctx cosmos.Context) cosmos.Iterator
	GetObservedNetworkFeeVoter(ctx cosmos.Context, height int64, chain common.Chain, rate int64) (ObservedNetworkFeeVoter, error)
}

type KeeperChainContract interface {
	SetChainContract(ctx cosmos.Context, cc ChainContract)
	GetChainContract(ctx cosmos.Context, chain common.Chain) (ChainContract, error)
	GetChainContracts(ctx cosmos.Context, chains common.Chains) []ChainContract
	GetChainContractIterator(ctx cosmos.Context) cosmos.Iterator
}

type KeeperSolvencyVoter interface {
	SetSolvencyVoter(_ cosmos.Context, _ SolvencyVoter)
	GetSolvencyVoter(_ cosmos.Context, _ common.TxID, _ common.Chain) (SolvencyVoter, error)
}

// NewKeeper creates new instances of the thorchain Keeper
type KeeperMAYAName interface {
	MAYANameExists(ctx cosmos.Context, name string) bool
	GetMAYAName(ctx cosmos.Context, name string) (MAYAName, error)
	SetMAYAName(ctx cosmos.Context, name MAYAName)
	GetMAYANameIterator(ctx cosmos.Context) cosmos.Iterator
	DeleteMAYAName(ctx cosmos.Context, name string) error
	SetAffiliateCollector(ctx_ cosmos.Context, affCol AffiliateFeeCollector)
	GetAffiliateCollector(ctx cosmos.Context, accAddress cosmos.AccAddress) (AffiliateFeeCollector, error)
	GetAffiliateCollectorIterator(ctx cosmos.Context) cosmos.Iterator
	GetAffiliateCollectors(ctx cosmos.Context) ([]AffiliateFeeCollector, error)
}

type KeeperTradeAccount interface {
	GetTradeAccount(ctx cosmos.Context, addr cosmos.AccAddress, asset common.Asset) (TradeAccount, error)
	SetTradeAccount(ctx cosmos.Context, record TradeAccount)
	RemoveTradeAccount(ctx cosmos.Context, record TradeAccount)
	GetTradeAccountIterator(ctx cosmos.Context) cosmos.Iterator
	GetTradeAccountIteratorWithAddress(ctx cosmos.Context, addr cosmos.AccAddress) cosmos.Iterator
	GetTradeUnit(ctx cosmos.Context, asset common.Asset) (TradeUnit, error)
	SetTradeUnit(ctx cosmos.Context, unit TradeUnit)
	GetTradeUnitIterator(ctx cosmos.Context) cosmos.Iterator
}

// NewKVStore creates new instances of the thorchain Keeper
func NewKeeper(cdc codec.BinaryCodec, coinKeeper bankkeeper.Keeper, accountKeeper authkeeper.AccountKeeper, ibcTransferkeeper ibctransferkeeper.Keeper, storeKey cosmos.StoreKey) Keeper {
	version := semver.MustParse("0.0.0")
	return kv1.NewKVStore(cdc, coinKeeper, accountKeeper, ibcTransferkeeper, storeKey, version)
}
