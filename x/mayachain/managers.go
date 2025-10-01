package mayachain

import (
	"errors"
	"fmt"

	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/codec"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	ibctransferkeeper "github.com/cosmos/ibc-go/v2/modules/apps/transfer/keeper"
	abci "github.com/tendermint/tendermint/abci/types"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/keeper"
	kv1 "gitlab.com/mayachain/mayanode/x/mayachain/keeper/v1"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

const (
	genesisBlockHeight = 1
)

// ErrNotEnoughToPayFee will happen when the emitted asset is not enough to pay for fee
var ErrNotEnoughToPayFee = errors.New("not enough asset to pay for fees")

// Manager is an interface to define all the required methods
type Manager interface {
	GetConstants() constants.ConstantValues
	// TODO remove after hard fork
	GetConfigInt64(ctx cosmos.Context, key constants.ConstantName) int64
	GetVersion() semver.Version
	Keeper() keeper.Keeper
	GasMgr() GasManager
	EventMgr() EventManager
	TxOutStore() TxOutStore
	NetworkMgr() NetworkManager
	ValidatorMgr() ValidatorManager
	ObMgr() ObserverManager
	PoolMgr() PoolManager
	SwapQ() SwapQueue
	OrderBookMgr() OrderBook
	Slasher() Slasher
	YggManager() YggManager
	TradeAccountManager() TradeAccountManager
}

// TradeAccountManager define all the methods required to manage trade accounts
type TradeAccountManager interface {
	EndBlock(ctx cosmos.Context, keeper keeper.Keeper) error
	Deposit(_ cosmos.Context, _ common.Asset, amount cosmos.Uint, owner cosmos.AccAddress, assetAddr common.Address, _ common.TxID) (cosmos.Uint, error)
	Withdrawal(_ cosmos.Context, _ common.Asset, amount cosmos.Uint, owner cosmos.AccAddress, assetAddr common.Address, _ common.TxID) (cosmos.Uint, error)
	BalanceOf(_ cosmos.Context, _ common.Asset, owner cosmos.AccAddress) cosmos.Uint
}

// GasManager define all the methods required to manage gas
type GasManager interface {
	BeginBlock(mgr Manager)
	EndBlock(ctx cosmos.Context, keeper keeper.Keeper, eventManager EventManager)
	AddGasAsset(gas common.Gas, increaseTxCount bool)
	ProcessGas(ctx cosmos.Context, keeper keeper.Keeper)
	GetGas() common.Gas
	GetFee(ctx cosmos.Context, chain common.Chain, asset common.Asset) cosmos.Uint
	GetNetworkFee(ctx cosmos.Context, chain common.Chain) (NetworkFee, error)
	GetMaxGas(ctx cosmos.Context, chain common.Chain) (common.Coin, error)
	GetGasRate(ctx cosmos.Context, chain common.Chain) cosmos.Uint
	SubGas(gas common.Gas)
	CalcOutboundFeeMultiplier(ctx cosmos.Context, targetSurplusRune, gasSpentRune, gasWithheldRune, maxMultiplier, minMultiplier cosmos.Uint) cosmos.Uint
}

// EventManager define methods need to be support to manage events
type EventManager interface {
	EmitEvent(ctx cosmos.Context, evt EmitEventItem) error
	EmitGasEvent(ctx cosmos.Context, gasEvent *EventGas) error
	EmitSwapEvent(ctx cosmos.Context, swap *EventSwap) error
	EmitFeeEvent(ctx cosmos.Context, feeEvent *EventFee) error
	EmitBondEvent(ctx cosmos.Context, mgr Manager, asset common.Asset, amount cosmos.Uint, bondType types.BondType, txIn common.Tx) error
}

// TxOutStore define the method required for TxOutStore
type TxOutStore interface {
	EndBlock(ctx cosmos.Context, mgr Manager) error
	GetBlockOut(ctx cosmos.Context) (*TxOut, error)
	ClearOutboundItems(ctx cosmos.Context)
	GetOutboundItems(ctx cosmos.Context) ([]TxOutItem, error)
	TryAddTxOutItem(ctx cosmos.Context, mgr Manager, toi TxOutItem, minOut cosmos.Uint) (bool, error)
	UnSafeAddTxOutItem(ctx cosmos.Context, mgr Manager, toi TxOutItem, height int64) error
	GetOutboundItemByToAddress(cosmos.Context, common.Address) []TxOutItem
	CalcTxOutHeight(cosmos.Context, semver.Version, TxOutItem) (int64, error)
}

// ObserverManager define the method to manage observes
type ObserverManager interface {
	BeginBlock()
	EndBlock(ctx cosmos.Context, keeper keeper.Keeper)
	AppendObserver(chain common.Chain, addrs []cosmos.AccAddress)
	List() []cosmos.AccAddress
}

// ValidatorManager define the method to manage validators
type ValidatorManager interface {
	BeginBlock(ctx cosmos.Context, mgr Manager, existingValidators []string) error
	EndBlock(ctx cosmos.Context, mgr Manager) []abci.ValidatorUpdate
	RequestYggReturn(ctx cosmos.Context, node NodeAccount, mgr Manager) error
	processRagnarok(ctx cosmos.Context, mgr Manager) error
	NodeAccountPreflightCheck(ctx cosmos.Context, na NodeAccount, constAccessor constants.ConstantValues) (NodeStatus, error)
}

// NetworkManager interface define the contract of network Manager
type NetworkManager interface {
	TriggerKeygen(ctx cosmos.Context, nas NodeAccounts) error
	RotateVault(ctx cosmos.Context, vault Vault) error
	EndBlock(ctx cosmos.Context, mgr Manager) error
	UpdateNetwork(ctx cosmos.Context, constAccessor constants.ConstantValues, gasManager GasManager, eventMgr EventManager) error
	RecallChainFunds(ctx cosmos.Context, chain common.Chain, mgr Manager, excludeNode common.PubKeys) error
}

// PoolManager interface define the contract of PoolManager
type PoolManager interface {
	EndBlock(ctx cosmos.Context, mgr Manager) error
}

// SwapQueue interface define the contract of Swap Queue
type SwapQueue interface {
	EndBlock(ctx cosmos.Context, mgr Manager) error
}

// OrderBook interface define the contract of Order Book
type OrderBook interface {
	EndBlock(ctx cosmos.Context, mgr Manager) error
}

// Slasher define all the method to perform slash
type Slasher interface {
	BeginBlock(ctx cosmos.Context, req abci.RequestBeginBlock, constAccessor constants.ConstantValues)
	LackObserving(ctx cosmos.Context, constAccessor constants.ConstantValues) error
	LackSigning(ctx cosmos.Context, mgr Manager) error
	SlashVaultToLP(ctx cosmos.Context, vaultPK common.PubKey, coins common.Coins, mgr Manager, subsidize bool) error
	SlashNodeAccountLP(ctx cosmos.Context, na NodeAccount, slash cosmos.Uint) (cosmos.Uint, []PoolAmt, error)
	IncSlashPoints(ctx cosmos.Context, point int64, addresses ...cosmos.AccAddress)
	DecSlashPoints(ctx cosmos.Context, point int64, addresses ...cosmos.AccAddress)
}

// YggManager define method to fund yggdrasil
type YggManager interface {
	Fund(ctx cosmos.Context, mgr Manager) error
}

type Swapper interface {
	Swap(ctx cosmos.Context,
		keeper keeper.Keeper,
		tx common.Tx,
		target common.Asset,
		destination common.Address,
		swapTarget cosmos.Uint,
		dexAgg string,
		dexAggTargetAsset string,
		dexAggLimit *cosmos.Uint,
		swp StreamingSwap,
		transactionFee cosmos.Uint,
		synthVirtualDepthMult int64,
		mgr Manager,
	) (cosmos.Uint, []*EventSwap, error)
	CalcAssetEmission(X, x, Y cosmos.Uint) cosmos.Uint
	CalcLiquidityFee(X, x, Y cosmos.Uint) cosmos.Uint
	CalcSwapSlip(Xi, xi cosmos.Uint, feebasePoint cosmos.Uint) cosmos.Uint
}

// Mgrs is an implementation of Manager interface
type Mgrs struct {
	currentVersion semver.Version
	constAccessor  constants.ConstantValues
	gasMgr         GasManager
	eventMgr       EventManager
	txOutStore     TxOutStore
	networkMgr     NetworkManager
	validatorMgr   ValidatorManager
	obMgr          ObserverManager
	poolMgr        PoolManager
	swapQ          SwapQueue
	orderBook      OrderBook
	slasher        Slasher
	yggManager     YggManager
	tradeManager   TradeAccountManager

	K                 keeper.Keeper
	cdc               codec.BinaryCodec
	coinKeeper        bankkeeper.Keeper
	accountKeeper     authkeeper.AccountKeeper
	ibcTransferkeeper ibctransferkeeper.Keeper
	storeKey          cosmos.StoreKey
}

// NewManagers  create a new Manager
func NewManagers(keeper keeper.Keeper, cdc codec.BinaryCodec, coinKeeper bankkeeper.Keeper, accountKeeper authkeeper.AccountKeeper, ibcTransferkeeper ibctransferkeeper.Keeper, storeKey cosmos.StoreKey) *Mgrs {
	return &Mgrs{
		K:                 keeper,
		cdc:               cdc,
		coinKeeper:        coinKeeper,
		accountKeeper:     accountKeeper,
		ibcTransferkeeper: ibcTransferkeeper,
		storeKey:          storeKey,
	}
}

func (mgr *Mgrs) GetVersion() semver.Version {
	return mgr.currentVersion
}

func (mgr *Mgrs) GetConstants() constants.ConstantValues {
	return mgr.constAccessor
}

// TODO remove after hard fork
func (mgr *Mgrs) GetConfigInt64(ctx cosmos.Context, key constants.ConstantName) int64 {
	val, err := mgr.Keeper().GetMimir(ctx, key.String())
	if val < 0 || err != nil {
		val = mgr.constAccessor.GetInt64Value(key)
		if err != nil {
			ctx.Logger().Error("fail to get mimir", "key", key.String(), "error", err)
		}
	}
	return val
}

// BeginBlock detect whether there are new version available, if it is available then create a new version of Mgr
func (mgr *Mgrs) BeginBlock(ctx cosmos.Context) error {
	v := mgr.K.GetLowestActiveVersion(ctx)
	if v.Equals(mgr.GetVersion()) {
		return nil
	}
	// version is different , thus all the manager need to re-create
	mgr.currentVersion = v
	mgr.constAccessor = constants.GetConstantValues(v)
	var err error

	mgr.K, err = GetKeeper(v, mgr.cdc, mgr.coinKeeper, mgr.accountKeeper, mgr.ibcTransferkeeper, mgr.storeKey)
	if err != nil {
		return fmt.Errorf("fail to create keeper: %w", err)
	}

	if v.GTE(semver.MustParse("1.96.0")) { // TODO remove version checks after fork
		storedVer, hasStoredVer := mgr.Keeper().GetVersionWithCtx(ctx)
		if !hasStoredVer || v.GT(storedVer) {
			// store the version for contextual lookups if it has been upgraded
			mgr.Keeper().SetVersionWithCtx(ctx, v)
		}
	}

	mgr.gasMgr, err = GetGasManager(v, mgr.K)
	if err != nil {
		return fmt.Errorf("fail to create gas manager: %w", err)
	}
	mgr.eventMgr, err = GetEventManager(v)
	if err != nil {
		return fmt.Errorf("fail to get event manager: %w", err)
	}
	mgr.txOutStore, err = GetTxOutStore(v, mgr.K, mgr.eventMgr, mgr.gasMgr)
	if err != nil {
		return fmt.Errorf("fail to get tx out store: %w", err)
	}

	mgr.networkMgr, err = GetNetworkManager(v, mgr.K, mgr.txOutStore, mgr.eventMgr)
	if err != nil {
		return fmt.Errorf("fail to get vault manager: %w", err)
	}

	mgr.poolMgr, err = GetPoolManager(v, mgr.K)
	if err != nil {
		return fmt.Errorf("fail to get pool manager: %w", err)
	}

	mgr.validatorMgr, err = GetValidatorManager(v, mgr.K, mgr.networkMgr, mgr.txOutStore, mgr.eventMgr)
	if err != nil {
		return fmt.Errorf("fail to get validator manager: %w", err)
	}

	mgr.obMgr, err = GetObserverManager(v)
	if err != nil {
		return fmt.Errorf("fail to get observer manager: %w", err)
	}

	mgr.swapQ, err = GetSwapQueue(v, mgr.K)
	if err != nil {
		return fmt.Errorf("fail to create swap queue: %w", err)
	}

	mgr.orderBook, err = GetOrderBook(v, mgr.K)
	if err != nil {
		return fmt.Errorf("fail to create order book: %w", err)
	}

	mgr.slasher, err = GetSlasher(v, mgr.K, mgr.eventMgr)
	if err != nil {
		return fmt.Errorf("fail to create swap queue: %w", err)
	}

	mgr.yggManager, err = GetYggManager(v, mgr.K)
	if err != nil {
		return fmt.Errorf("fail to create swap queue: %w", err)
	}

	if v.GTE(semver.MustParse("1.123.0")) { // trade-accounts // TODO: remove on hard fork
		mgr.tradeManager, err = GetTradeAccountManager(v, mgr.K, mgr.eventMgr)
		if err != nil {
			return fmt.Errorf("fail to create trade manager: %w", err)
		}
	}
	return nil
}

// Keeper return Keeper
func (mgr *Mgrs) Keeper() keeper.Keeper { return mgr.K }

// GasMgr return GasManager
func (mgr *Mgrs) GasMgr() GasManager { return mgr.gasMgr }

// EventMgr return EventMgr
func (mgr *Mgrs) EventMgr() EventManager { return mgr.eventMgr }

// TxOutStore return an TxOutStore
func (mgr *Mgrs) TxOutStore() TxOutStore { return mgr.txOutStore }

// VaultMgr return a valid NetworkManager
func (mgr *Mgrs) NetworkMgr() NetworkManager { return mgr.networkMgr }

// PoolMgr return a valid PoolManager
func (mgr *Mgrs) PoolMgr() PoolManager { return mgr.poolMgr }

// ValidatorMgr return an implementation of ValidatorManager
func (mgr *Mgrs) ValidatorMgr() ValidatorManager { return mgr.validatorMgr }

// ObMgr return an implementation of ObserverManager
func (mgr *Mgrs) ObMgr() ObserverManager { return mgr.obMgr }

// SwapQ return an implementation of SwapQueue
func (mgr *Mgrs) SwapQ() SwapQueue { return mgr.swapQ }

// OrderBookMgr
func (mgr *Mgrs) OrderBookMgr() OrderBook { return mgr.orderBook }

// Slasher return an implementation of Slasher
func (mgr *Mgrs) Slasher() Slasher { return mgr.slasher }

// YggManager return an implementation of YggManager
func (mgr *Mgrs) YggManager() YggManager { return mgr.yggManager }

// TradeAccountManager return an implementation of TradeAccountManager
func (mgr *Mgrs) TradeAccountManager() TradeAccountManager { return mgr.tradeManager } // may be nil before v1.121

// GetKeeper return Keeper
func GetKeeper(version semver.Version, cdc codec.BinaryCodec, coinKeeper bankkeeper.Keeper, accountKeeper authkeeper.AccountKeeper, ibcTransferkeeper ibctransferkeeper.Keeper, storeKey cosmos.StoreKey) (keeper.Keeper, error) {
	if version.GTE(semver.MustParse("0.1.0")) {
		return kv1.NewKVStore(cdc, coinKeeper, accountKeeper, ibcTransferkeeper, storeKey, version), nil
	}
	return nil, errInvalidVersion
}

// GetGasManager return GasManager
func GetGasManager(version semver.Version, keeper keeper.Keeper) (GasManager, error) {
	constAcessor := constants.GetConstantValues(version)
	switch {
	case version.GTE(semver.MustParse("1.123.0")): // trade-accounts
		return newGasMgrVCUR(constAcessor, keeper), nil
	case version.GTE(semver.MustParse("1.112.0")):
		return newGasMgrV112(constAcessor, keeper), nil
	case version.GTE(semver.MustParse("1.109.0")):
		return newGasMgrV109(constAcessor, keeper), nil
	case version.GTE(semver.MustParse("1.108.0")):
		return newGasMgrV108(constAcessor, keeper), nil
	case version.GTE(semver.MustParse("1.106.0")):
		return newGasMgrV106(constAcessor, keeper), nil
	case version.GTE(semver.MustParse("1.104.0")):
		return newGasMgrV104(constAcessor, keeper), nil
	case version.GTE(semver.MustParse("1.103.0")):
		return newGasMgrV103(constAcessor, keeper), nil
	case version.GTE(semver.MustParse("1.100.0")):
		return newGasMgrV100(constAcessor, keeper), nil
	case version.GTE(semver.MustParse("1.98.0")):
		return newGasMgrV98(constAcessor, keeper), nil
	case version.GTE(semver.MustParse("1.94.0")):
		return newGasMgrV94(constAcessor, keeper), nil
	default:
		return nil, errInvalidVersion
	}
}

// GetEventManager will return an implementation of EventManager
func GetEventManager(version semver.Version) (EventManager, error) {
	switch {
	case version.GTE(semver.MustParse("0.1.0")):
		return newEventMgrVCUR(), nil
	default:
		return nil, errInvalidVersion
	}
}

// GetTxOutStore will return an implementation of the txout store that
func GetTxOutStore(version semver.Version, keeper keeper.Keeper, eventMgr EventManager, gasManager GasManager) (TxOutStore, error) {
	constAccessor := constants.GetConstantValues(version)
	switch {
	case version.GTE(semver.MustParse("1.123.0")): // trade-accounts
		return newTxOutStorageVCUR(keeper, constAccessor, eventMgr, gasManager), nil
	case version.GTE(semver.MustParse("1.120.0")):
		return newTxOutStorageV120(keeper, constAccessor, eventMgr, gasManager), nil
	case version.GTE(semver.MustParse("1.118.0")):
		return newTxOutStorageV118(keeper, constAccessor, eventMgr, gasManager), nil
	case version.GTE(semver.MustParse("1.113.0")):
		return newTxOutStorageV113(keeper, constAccessor, eventMgr, gasManager), nil
	case version.GTE(semver.MustParse("1.112.0")):
		return newTxOutStorageV112(keeper, constAccessor, eventMgr, gasManager), nil
	case version.GTE(semver.MustParse("1.111.0")):
		return newTxOutStorageV111(keeper, constAccessor, eventMgr, gasManager), nil
	case version.GTE(semver.MustParse("1.110.0")):
		return newTxOutStorageV110(keeper, constAccessor, eventMgr, gasManager), nil
	case version.GTE(semver.MustParse("1.108.0")):
		return newTxOutStorageV108(keeper, constAccessor, eventMgr, gasManager), nil
	case version.GTE(semver.MustParse("1.106.0")):
		return newTxOutStorageV106(keeper, constAccessor, eventMgr, gasManager), nil
	case version.GTE(semver.MustParse("1.104.0")):
		return newTxOutStorageV104(keeper, constAccessor, eventMgr, gasManager), nil
	case version.GTE(semver.MustParse("1.97.0")):
		return newTxOutStorageV97(keeper, constAccessor, eventMgr, gasManager), nil
	case version.GTE(semver.MustParse("1.95.0")):
		return newTxOutStorageV95(keeper, constAccessor, eventMgr, gasManager), nil
	default:
		return nil, errInvalidVersion
	}
}

// GetNetworkManager  retrieve a NetworkManager that is compatible with the given version
func GetNetworkManager(version semver.Version, keeper keeper.Keeper, txOutStore TxOutStore, eventMgr EventManager) (NetworkManager, error) {
	switch {
	// FIXME: check NetworkMgrVCUR.UpdateNetwork when adding new version
	// - Remove version check for new CACAOPoolRewardsEnabled mimir
	case version.GTE(semver.MustParse("1.124.0")):
		return newNetworkMgrVCUR(keeper, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.118.0")):
		return newNetworkMgrV118(keeper, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.109.0")):
		return newNetworkMgrV109(keeper, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.108.0")):
		return newNetworkMgrV108(keeper, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.107.0")):
		return newNetworkMgrV107(keeper, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.102.0")):
		return newNetworkMgrV102(keeper, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.96.0")):
		return newNetworkMgrV96(keeper, txOutStore, eventMgr), nil
	default:
		return nil, errInvalidVersion
	}
}

// GetValidatorManager create a new instance of Validator Manager
func GetValidatorManager(version semver.Version, keeper keeper.Keeper, networkMgr NetworkManager, txOutStore TxOutStore, eventMgr EventManager) (ValidatorManager, error) {
	switch {
	case version.GTE(semver.MustParse("1.123.0")):
		return newValidatorMgrVCUR(keeper, networkMgr, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.114.0")):
		return newValidatorMgrV114(keeper, networkMgr, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.110.0")):
		return newValidatorMgrV110(keeper, networkMgr, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.108.0")):
		return newValidatorMgrV108(keeper, networkMgr, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.107.0")):
		return newValidatorMgrV107(keeper, networkMgr, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.102.0")):
		return newValidatorMgrV102(keeper, networkMgr, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.95.0")):
		return newValidatorMgrV95(keeper, networkMgr, txOutStore, eventMgr), nil
	default:
		return nil, errInvalidVersion
	}
}

// GetObserverManager return an instance that implements ObserverManager interface
// when there is no version can match the given semver , it will return nil
func GetObserverManager(version semver.Version) (ObserverManager, error) {
	switch {
	case version.GTE(semver.MustParse("0.1.0")):
		return newObserverMgrVCUR(), nil
	default:
		return nil, errInvalidVersion
	}
}

// GetPoolManager return an implementation of PoolManager
func GetPoolManager(version semver.Version, keeper keeper.Keeper) (PoolManager, error) {
	switch {
	case version.GTE(semver.MustParse("1.112.0")):
		return newPoolMgrVCUR(keeper), nil
	case version.GTE(semver.MustParse("1.108.0")):
		return newPoolMgrV108(keeper), nil
	case version.GTE(semver.MustParse("1.95.0")):
		return newPoolMgrV95(keeper), nil
	default:
		return nil, errInvalidVersion
	}
}

// GetSwapQueue retrieve a SwapQueue that is compatible with the given version
func GetSwapQueue(version semver.Version, keeper keeper.Keeper) (SwapQueue, error) {
	switch {
	case version.GTE(semver.MustParse("1.118.0")):
		return newSwapQueueVCUR(keeper), nil
	case version.GTE(semver.MustParse("1.112.0")):
		return newSwapQueueV112(keeper), nil
	case version.GTE(semver.MustParse("1.110.0")):
		return newSwapQueueV110(keeper), nil
	case version.GTE(semver.MustParse("1.95.0")):
		return newSwapQueueV95(keeper), nil
	default:
		return nil, errInvalidVersion
	}
}

// GetOrderBook retrieve a OrderBook that is compatible with the given version
func GetOrderBook(version semver.Version, keeper keeper.Keeper) (OrderBook, error) {
	switch {
	case version.GTE(semver.MustParse("0.1.0")):
		return newOrderBookVCUR(keeper), nil
	default:
		return nil, errInvalidVersion
	}
}

// GetSlasher return an implementation of Slasher
func GetSlasher(version semver.Version, keeper keeper.Keeper, eventMgr EventManager) (Slasher, error) {
	switch {
	case version.GTE(semver.MustParse("1.115.0")):
		return newSlasherVCUR(keeper, eventMgr), nil
	case version.GTE(semver.MustParse("1.113.0")):
		return newSlasherV113(keeper, eventMgr), nil
	case version.GTE(semver.MustParse("1.112.0")):
		return newSlasherV112(keeper, eventMgr), nil
	case version.GTE(semver.MustParse("1.110.0")):
		return newSlasherV110(keeper, eventMgr), nil
	case version.GTE(semver.MustParse("1.108.0")):
		return newSlasherV108(keeper, eventMgr), nil
	case version.GTE(semver.MustParse("1.105.0")):
		return newSlasherV105(keeper, eventMgr), nil
	case version.GTE(semver.MustParse("1.104.0")):
		return newSlasherV104(keeper, eventMgr), nil
	case version.GTE(semver.MustParse("1.102.0")):
		return newSlasherV102(keeper, eventMgr), nil
	case version.GTE(semver.MustParse("1.92.0")):
		return newSlasherV92(keeper, eventMgr), nil
	default:
		return nil, errInvalidVersion
	}
}

// GetYggManager return an implementation of YggManager
func GetYggManager(version semver.Version, keeper keeper.Keeper) (YggManager, error) {
	switch {
	case version.GTE(semver.MustParse("0.79.0")):
		return newYggMgrVCUR(keeper), nil
	default:
		return nil, errInvalidVersion
	}
}

// GetSwapper return an implementation of Swapper
func GetSwapper(version semver.Version) (Swapper, error) {
	switch {
	case version.GTE(semver.MustParse("1.123.0")): // trade-accounts
		return newSwapperVCUR(), nil
	case version.GTE(semver.MustParse("1.112.0")):
		return newSwapperV112(), nil
	case version.GTE(semver.MustParse("1.111.0")):
		return newSwapperV111(), nil
	case version.GTE(semver.MustParse("1.110.0")):
		return newSwapperV110(), nil
	case version.GTE(semver.MustParse("1.109.0")):
		return newSwapperV109(), nil
	case version.GTE(semver.MustParse("1.107.0")):
		return newSwapperV107(), nil
	case version.GTE(semver.MustParse("1.95.0")):
		return newSwapperV95(), nil
	default:
		return nil, errInvalidVersion
	}
}

func GetTradeAccountManager(version semver.Version, keeper keeper.Keeper, eventMgr EventManager) (TradeAccountManager, error) {
	switch {
	case version.GTE(semver.MustParse("1.123.0")): // trade-accounts
		return newTradeMgrVCUR(keeper, eventMgr), nil
	default:
		return nil, errInvalidVersion
	}
}
