package mayaclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/std"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"

	"gitlab.com/mayachain/mayanode/app"
	"gitlab.com/mayachain/mayanode/bifrost/mayaclient/types"
	"gitlab.com/mayachain/mayanode/bifrost/metrics"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/config"
	"gitlab.com/mayachain/mayanode/constants"
	openapi "gitlab.com/mayachain/mayanode/openapi/gen"
	stypes "gitlab.com/mayachain/mayanode/x/mayachain/types"
)

// Endpoint urls
const (
	AuthAccountEndpoint      = "/auth/accounts"
	BroadcastTxsEndpoint     = "/"
	KeygenEndpoint           = "/mayachain/keygen"
	KeysignEndpoint          = "/mayachain/keysign"
	LastBlockEndpoint        = "/mayachain/lastblock"
	NodeAccountEndpoint      = "/mayachain/node"
	SignerMembershipEndpoint = "/mayachain/vaults/%s/signers"
	StatusEndpoint           = "/status"
	VaultEndpoint            = "/mayachain/vault/%s"
	AsgardVault              = "/mayachain/vaults/asgard"
	PubKeysEndpoint          = "/mayachain/vaults/pubkeys"
	MayachainConstants       = "/mayachain/constants"
	RagnarokEndpoint         = "/mayachain/ragnarok"
	MimirEndpoint            = "/mayachain/mimir"
	ChainVersionEndpoint     = "/mayachain/version"
	InboundAddressesEndpoint = "/mayachain/inbound_addresses"
	PoolsEndpoint            = "/mayachain/pools"
	MAYANameEndpoint         = "/mayachain/mayaname/%s"
)

// mayachainBridge will be used to send tx to MAYAChain
type mayachainBridge struct {
	logger        zerolog.Logger
	cfg           config.BifrostClientConfiguration
	keys          *Keys
	errCounter    *prometheus.CounterVec
	m             *metrics.Metrics
	blockHeight   int64
	accountNumber uint64
	seqNumber     uint64
	httpClient    *retryablehttp.Client
	broadcastLock *sync.RWMutex

	lastBlockHeightCheck     time.Time
	lastMayachainBlockHeight int64
}

type MayachainBridge interface {
	EnsureNodeWhitelisted() error
	EnsureNodeWhitelistedWithTimeout() error
	FetchNodeStatus() (stypes.NodeStatus, error)
	GetAsgards() (stypes.Vaults, error)
	GetVault(pubkey string) (stypes.Vault, error)
	GetConfig() config.BifrostClientConfiguration
	GetConstants() (map[string]int64, error)
	GetContext() client.Context
	GetContractAddress() ([]PubKeyContractAddressPair, error)
	GetErrataMsg(txID common.TxID, chain common.Chain) sdk.Msg
	GetKeygenStdTx(poolPubKey common.PubKey, keyshares []byte, blame []stypes.Blame, inputPks common.PubKeys, keygenType stypes.KeygenType, chains common.Chains, height, keygenTime int64, poolPubKeyEddsa common.PubKey, keysharesBackupEddsa []byte) (sdk.Msg, error)
	GetKeysignParty(vaultPubKey common.PubKey) (common.PubKeys, error)
	GetMimir(key string) (int64, error)
	GetObservationsStdTx(txIns stypes.ObservedTxs) ([]cosmos.Msg, error)
	GetPools() (stypes.Pools, error)
	GetPubKeys() ([]PubKeyContractAddressPair, error)
	GetAsgardPubKeys() ([]PubKeyContractAddressPair, error)
	GetSolvencyMsg(height int64, chain common.Chain, pubKey common.PubKey, coins common.Coins) sdk.Msg
	GetMAYAName(name string) (stypes.MAYAName, error)
	GetMayachainVersion() (semver.Version, error)
	IsCatchingUp() (bool, error)
	HasNetworkFee(chain common.Chain) (bool, error)
	PostKeysignFailure(blame stypes.Blame, height int64, memo string, coins common.Coins, pubkey common.PubKey) (common.TxID, error)
	PostNetworkFee(height int64, chain common.Chain, transactionSize, transactionRate uint64) (common.TxID, error)
	RagnarokInProgress() (bool, error)
	WaitToCatchUp() error
	GetBlockHeight() (int64, error)
	GetLastObservedInHeight(chain common.Chain) (int64, error)
	GetLastSignedOutHeight(chain common.Chain) (int64, error)
	Broadcast(msgs ...sdk.Msg) (common.TxID, error)
	GetKeysign(blockHeight int64, pk string) (types.TxOut, error)
	GetNodeAccount(string) (*stypes.NodeAccount, error)
	GetKeygenBlock(int64, string) (stypes.KeygenBlock, error)
}

// NewMayachainBridge create a new instance of MayachainBridge
func NewMayachainBridge(cfg config.BifrostClientConfiguration, m *metrics.Metrics, k *Keys) (MayachainBridge, error) {
	// main module logger
	logger := log.With().Str("module", "mayachain_client").Logger()

	if len(cfg.ChainID) == 0 {
		return nil, errors.New("chain id is empty")
	}
	if len(cfg.ChainHost) == 0 {
		return nil, errors.New("chain host is empty")
	}

	httpClient := retryablehttp.NewClient()
	httpClient.Logger = nil

	// Reduce timeouts
	httpClient.RetryWaitMin = 250 * time.Millisecond
	httpClient.RetryWaitMax = 1 * time.Second
	// Reduce max retries - default is 4
	httpClient.RetryMax = 2

	return &mayachainBridge{
		logger:        logger,
		cfg:           cfg,
		keys:          k,
		errCounter:    m.GetCounterVec(metrics.MayachainClientError),
		httpClient:    httpClient,
		m:             m,
		broadcastLock: &sync.RWMutex{},
	}, nil
}

func MakeCodec() codec.ProtoCodecMarshaler {
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	std.RegisterInterfaces(interfaceRegistry)
	stypes.RegisterInterfaces(interfaceRegistry)
	return codec.NewProtoCodec(interfaceRegistry)
}

// MakeLegacyCodec creates codec
func MakeLegacyCodec() *codec.LegacyAmino {
	cdc := codec.NewLegacyAmino()
	banktypes.RegisterLegacyAminoCodec(cdc)
	authtypes.RegisterLegacyAminoCodec(cdc)
	cosmos.RegisterCodec(cdc)
	stypes.RegisterCodec(cdc)
	return cdc
}

// GetContext return a valid context with all relevant values set
func (b *mayachainBridge) GetContext() client.Context {
	ctx := client.Context{}
	ctx = ctx.WithKeyring(b.keys.GetKeybase())
	ctx = ctx.WithChainID(string(b.cfg.ChainID))
	ctx = ctx.WithHomeDir(b.cfg.ChainHomeFolder)
	ctx = ctx.WithFromName(b.cfg.SignerName)
	ctx = ctx.WithFromAddress(b.keys.GetSignerInfo().GetAddress())
	ctx = ctx.WithBroadcastMode("sync")

	encodingConfig := app.MakeEncodingConfig()
	ctx = ctx.WithCodec(encodingConfig.Marshaler)
	ctx = ctx.WithInterfaceRegistry(encodingConfig.InterfaceRegistry)
	ctx = ctx.WithTxConfig(encodingConfig.TxConfig)
	ctx = ctx.WithLegacyAmino(encodingConfig.Amino)
	ctx = ctx.WithAccountRetriever(authtypes.AccountRetriever{})

	remote := b.cfg.ChainRPC
	if !strings.HasSuffix(b.cfg.ChainHost, "http") {
		remote = fmt.Sprintf("tcp://%s", remote)
	}
	ctx = ctx.WithNodeURI(remote)
	client, err := rpchttp.New(remote, "/websocket")
	if err != nil {
		panic(err)
	}
	ctx = ctx.WithClient(client)
	return ctx
}

func (b *mayachainBridge) getWithPath(path string) ([]byte, int, error) {
	return b.get(b.getMayaChainURL(path))
}

// get handle all the low level http GET calls using retryablehttp.MayachainBridge
func (b *mayachainBridge) get(url string) ([]byte, int, error) {
	resp, err := b.httpClient.Get(url)
	if err != nil {
		b.errCounter.WithLabelValues("fail_get_from_mayachain", "").Inc()
		return nil, http.StatusNotFound, fmt.Errorf("failed to GET from mayachain: %w", err)
	}
	defer func() {
		if err = resp.Body.Close(); err != nil {
			b.logger.Error().Err(err).Msg("failed to close response body")
		}
	}()

	buf, err := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return buf, resp.StatusCode, errors.New("Status code: " + resp.Status + " returned")
	}
	if err != nil {
		b.errCounter.WithLabelValues("fail_read_mayachain_resp", "").Inc()
		return nil, resp.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}
	return buf, resp.StatusCode, nil
}

// getMayaChainURL with the given path
func (b *mayachainBridge) getMayaChainURL(path string) string {
	uri := url.URL{
		Scheme: "http",
		Host:   b.cfg.ChainHost,
		Path:   path,
	}
	return uri.String()
}

// getAccountNumberAndSequenceNumber returns account and Sequence number required to post into mayachain
func (b *mayachainBridge) getAccountNumberAndSequenceNumber() (uint64, uint64, error) {
	path := fmt.Sprintf("%s/%s", AuthAccountEndpoint, b.keys.GetSignerInfo().GetAddress())

	body, _, err := b.getWithPath(path)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get auth accounts: %w", err)
	}

	var resp types.AccountResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, 0, fmt.Errorf("failed to unmarshal account resp: %w", err)
	}
	acc := resp.Result.Value

	return acc.AccountNumber, acc.Sequence, nil
}

// GetConfig return the configuration
func (b *mayachainBridge) GetConfig() config.BifrostClientConfiguration {
	return b.cfg
}

// PostKeysignFailure generate and  post a keysign fail tx to mayachan
func (b *mayachainBridge) PostKeysignFailure(blame stypes.Blame, height int64, memo string, coins common.Coins, pubkey common.PubKey) (common.TxID, error) {
	start := time.Now()
	defer func() {
		b.m.GetHistograms(metrics.SignToMayachainDuration).Observe(time.Since(start).Seconds())
	}()

	if blame.IsEmpty() {
		// MsgTssKeysignFail will fail validation if having no FailReason.
		blame.FailReason = "no fail reason available"
	}
	msg, err := stypes.NewMsgTssKeysignFail(height, blame, memo, coins, b.keys.GetSignerInfo().GetAddress(), pubkey)
	if err != nil {
		return common.BlankTxID, fmt.Errorf("fail to create keysign fail message: %w", err)
	}
	return b.Broadcast(msg)
}

// GetErrataMsg get errata tx from params
func (b *mayachainBridge) GetErrataMsg(txID common.TxID, chain common.Chain) sdk.Msg {
	return stypes.NewMsgErrataTx(txID, chain, b.keys.GetSignerInfo().GetAddress())
}

// GetSolvencyMsg create MsgSolvency from the given parameters
func (b *mayachainBridge) GetSolvencyMsg(height int64, chain common.Chain, pubKey common.PubKey, coins common.Coins) sdk.Msg {
	// To prevent different MsgSolvency ID incompatibility between nodes with different coin-observation histories,
	// only report coins for which the amounts are not currently 0.
	coins = coins.NoneEmpty()
	msg, err := stypes.NewMsgSolvency(chain, pubKey, coins, height, b.keys.GetSignerInfo().GetAddress())
	if err != nil {
		b.logger.Err(err).Msg("fail to create MsgSolvency")
		return nil
	}
	return msg
}

// GetKeygenStdTx get keygen tx from params
func (b *mayachainBridge) GetKeygenStdTx(poolPubKey common.PubKey, keysharesBackup []byte, blame []stypes.Blame, inputPks common.PubKeys, keygenType stypes.KeygenType, chains common.Chains, height, keygenTime int64, poolPubKeyEddsa common.PubKey, keysharesBackupEddsa []byte) (sdk.Msg, error) {
	// Get the current version to determine which message constructor to use
	version, err := b.GetMayachainVersion()
	if err != nil {
		// If we can't get version, default to V2 (current behavior)
		b.logger.Warn().Err(err).Msg("failed to get mayachain version, defaulting to NewMsgTssPoolV2")
		return stypes.NewMsgTssPoolV2(inputPks.Strings(), poolPubKey, keysharesBackup, keygenType, height, blame, chains.Strings(), b.keys.GetSignerInfo().GetAddress(), keygenTime, poolPubKeyEddsa, keysharesBackupEddsa)
	}

	// Pre-v124: Use NewMsgTssPool which populates BlameLegacy field
	if version.LT(semver.MustParse("1.124.0")) {
		// Convert []Blame to single Blame for pre-v124
		var singleBlame stypes.Blame
		if len(blame) > 0 {
			singleBlame = blame[0]
		}
		return stypes.NewMsgTssPool(inputPks.Strings(), poolPubKey, keysharesBackup, keygenType, height, singleBlame, chains.Strings(), b.keys.GetSignerInfo().GetAddress(), keygenTime)
	}

	// v124+: Use NewMsgTssPoolV2 which populates Blame field
	return stypes.NewMsgTssPoolV2(inputPks.Strings(), poolPubKey, keysharesBackup, keygenType, height, blame, chains.Strings(), b.keys.GetSignerInfo().GetAddress(), keygenTime, poolPubKeyEddsa, keysharesBackupEddsa)
}

// GetObservationsStdTx get observations tx from txIns
func (b *mayachainBridge) GetObservationsStdTx(txIns stypes.ObservedTxs) ([]cosmos.Msg, error) {
	if len(txIns) == 0 {
		return nil, nil
	}
	inbound := stypes.ObservedTxs{}
	outbound := stypes.ObservedTxs{}

	// spilt our txs into inbound vs outbound txs
	for _, tx := range txIns {
		chain := common.BNBChain
		if len(tx.Tx.Coins) > 0 {
			chain = tx.Tx.Coins[0].Asset.Chain
		}

		obAddr, err := tx.ObservedPubKey.GetAddress(chain)
		if err != nil {
			b.logger.Err(err).Msgf("fail to parse observed pool address: %s", tx.ObservedPubKey.String())
			continue
		}
		// for consolidate UTXO tx, both From & To address will be the asgard address
		// thus here we need to make sure that one add to inbound , the other add to outbound
		if tx.Tx.ToAddress.Equals(obAddr) && !inbound.Contains(tx) { // nolint
			inbound = append(inbound, tx)
		} else if tx.Tx.FromAddress.Equals(obAddr) && !outbound.Contains(tx) {
			// for outbound transaction , there is no need to do confirmation counting
			tx.FinaliseHeight = tx.BlockHeight
			outbound = append(outbound, tx)
		} else {
			return nil, errors.New("could not determine if this tx as inbound or outbound")
		}
	}

	var msgs []cosmos.Msg
	if len(inbound) > 0 {
		msgs = append(msgs, stypes.NewMsgObservedTxIn(inbound, b.keys.GetSignerInfo().GetAddress()))
	}
	if len(outbound) > 0 {
		msgs = append(msgs, stypes.NewMsgObservedTxOut(outbound, b.keys.GetSignerInfo().GetAddress()))
	}

	return msgs, nil
}

// EnsureNodeWhitelistedWithTimeout check node is whitelisted with timeout retry
func (b *mayachainBridge) EnsureNodeWhitelistedWithTimeout() error {
	for {
		select {
		case <-time.After(time.Hour):
			return errors.New("Observer is not whitelisted yet")
		default:
			err := b.EnsureNodeWhitelisted()
			if err == nil {
				// node had been whitelisted
				return nil
			}
			b.logger.Error().Err(err).Msg("observer is not whitelisted , will retry a bit later")
			time.Sleep(time.Second * 30)
		}
	}
}

// EnsureNodeWhitelisted will call to mayachain to check whether the observer had been whitelist or not
func (b *mayachainBridge) EnsureNodeWhitelisted() error {
	status, err := b.FetchNodeStatus()
	if err != nil {
		return fmt.Errorf("failed to get node status: %w", err)
	}
	if status == stypes.NodeStatus_Unknown {
		return fmt.Errorf("node account status %s , will not be able to forward transaction to mayachain", status)
	}
	return nil
}

// FetchNodeStatus get current node status from mayachain
func (b *mayachainBridge) FetchNodeStatus() (stypes.NodeStatus, error) {
	bepAddr := b.keys.GetSignerInfo().GetAddress().String()
	if len(bepAddr) == 0 {
		return stypes.NodeStatus_Unknown, errors.New("bep address is empty")
	}
	na, err := b.GetNodeAccount(bepAddr)
	if err != nil {
		return stypes.NodeStatus_Unknown, fmt.Errorf("failed to get node status: %w", err)
	}
	return na.Status, nil
}

// GetKeysignParty call into mayachain to get the node accounts that should be join together to sign the message
func (b *mayachainBridge) GetKeysignParty(vaultPubKey common.PubKey) (common.PubKeys, error) {
	p := fmt.Sprintf(SignerMembershipEndpoint, vaultPubKey.String())
	result, _, err := b.getWithPath(p)
	if err != nil {
		return common.PubKeys{}, fmt.Errorf("fail to get key sign party from mayachain: %w", err)
	}
	var keys common.PubKeys
	if err := json.Unmarshal(result, &keys); err != nil {
		return common.PubKeys{}, fmt.Errorf("fail to unmarshal result to pubkeys:%w", err)
	}
	return keys, nil
}

// IsCatchingUp returns bool for if mayachain is catching up to the rest of the
// nodes. Returns yes, if it is, false if it is caught up.
func (b *mayachainBridge) IsCatchingUp() (bool, error) {
	uri := url.URL{
		Scheme: "http",
		Host:   b.cfg.ChainRPC,
		Path:   StatusEndpoint,
	}

	body, _, err := b.get(uri.String())
	if err != nil {
		return false, fmt.Errorf("failed to get status data: %w", err)
	}

	var resp struct {
		Result struct {
			SyncInfo struct {
				CatchingUp bool `json:"catching_up"`
			} `json:"sync_info"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return false, fmt.Errorf("failed to unmarshal tendermint status: %w", err)
	}
	return resp.Result.SyncInfo.CatchingUp, nil
}

// HasNetworkFee checks whether the given chain has set a network fee - determined by
// whether the `outbound_tx_size` for the inbound address response is non-zero.
func (b *mayachainBridge) HasNetworkFee(chain common.Chain) (bool, error) {
	buf, s, err := b.getWithPath(InboundAddressesEndpoint)
	if err != nil {
		return false, fmt.Errorf("fail to get inbound addresses: %w", err)
	}
	if s != http.StatusOK {
		return false, fmt.Errorf("unexpected status code: %d", s)
	}

	var resp []openapi.InboundAddress
	if err := json.Unmarshal(buf, &resp); err != nil {
		return false, fmt.Errorf("fail to unmarshal inbound addresses: %w", err)
	}

	for _, addr := range resp {
		if addr.Chain != nil && *addr.Chain == chain.String() && addr.OutboundTxSize != nil {
			size, err := strconv.ParseInt(*addr.OutboundTxSize, 10, 64)
			if err != nil {
				return false, fmt.Errorf("fail to parse outbound_tx_size: %w", err)
			}
			return size > 0, nil
		}
	}

	return false, fmt.Errorf("no inbound address found for chain: %s", chain)
}

// WaitToCatchUp wait for mayachain to catch up
func (b *mayachainBridge) WaitToCatchUp() error {
	for {
		yes, err := b.IsCatchingUp()
		if err != nil {
			return err
		}
		if !yes {
			break
		}
		b.logger.Info().Msg("mayachain is not caught up... waiting...")
		time.Sleep(constants.MayachainBlockTime)
	}
	return nil
}

// GetAsgards retrieve all the asgard vaults from mayachain
func (b *mayachainBridge) GetAsgards() (stypes.Vaults, error) {
	buf, s, err := b.getWithPath(AsgardVault)
	if err != nil {
		return nil, fmt.Errorf("fail to get asgard vaults: %w", err)
	}
	if s != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d", s)
	}
	var vaults stypes.Vaults
	if err := json.Unmarshal(buf, &vaults); err != nil {
		return nil, fmt.Errorf("fail to unmarshal asgard vaults from json: %w", err)
	}
	return vaults, nil
}

// GetVault retrieves a specific vault from thorchain.
func (b *mayachainBridge) GetVault(pubkey string) (stypes.Vault, error) {
	buf, s, err := b.getWithPath(fmt.Sprintf(VaultEndpoint, pubkey))
	if err != nil {
		return stypes.Vault{}, fmt.Errorf("fail to get vault: %w", err)
	}
	if s != http.StatusOK {
		return stypes.Vault{}, fmt.Errorf("unexpected status code %d", s)
	}
	var vault stypes.Vault
	if err = json.Unmarshal(buf, &vault); err != nil {
		return stypes.Vault{}, fmt.Errorf("fail to unmarshal vault from json: %w", err)
	}
	return vault, nil
}

func (b *mayachainBridge) getVaultPubkeys() ([]byte, error) {
	buf, s, err := b.getWithPath(PubKeysEndpoint)
	if err != nil {
		return nil, fmt.Errorf("fail to get asgard vaults: %w", err)
	}
	if s != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d", s)
	}
	return buf, nil
}

// GetPubKeys retrieve vault pub keys and their relevant smart contracts
func (b *mayachainBridge) GetPubKeys() ([]PubKeyContractAddressPair, error) {
	buf, err := b.getVaultPubkeys()
	if err != nil {
		return nil, fmt.Errorf("fail to get vault pubkeys ,err: %w", err)
	}
	var result openapi.VaultPubkeysResponse
	if err = json.Unmarshal(buf, &result); err != nil {
		return nil, fmt.Errorf("fail to unmarshal pubkeys: %w", err)
	}
	var addressPairs []PubKeyContractAddressPair
	for _, v := range append(result.Asgard, append(result.Yggdrasil, result.Inactive...)...) {
		if v.PubKeyEddsa != nil && *v.PubKeyEddsa != "" {
			kp := PubKeyContractAddressPair{
				PubKey:    common.PubKey(*v.PubKeyEddsa),
				Contracts: make(map[common.Chain]common.Address),
				Algo:      common.SigningAlgoEd25519,
			}
			for _, item := range v.Routers {
				kp.Contracts[common.Chain(*item.Chain)] = common.Address(*item.Router)
			}
			addressPairs = append(addressPairs, kp)
		}
		kp := PubKeyContractAddressPair{
			PubKey:    common.PubKey(v.PubKey),
			Contracts: make(map[common.Chain]common.Address),
			Algo:      common.SigningAlgoSecp256k1,
		}
		for _, item := range v.Routers {
			kp.Contracts[common.Chain(*item.Chain)] = common.Address(*item.Router)
		}
		addressPairs = append(addressPairs, kp)
	}
	return addressPairs, nil
}

// GetAsgardPubKeys retrieve asgard vaults, and it's relevant smart contracts
func (b *mayachainBridge) GetAsgardPubKeys() ([]PubKeyContractAddressPair, error) {
	buf, err := b.getVaultPubkeys()
	if err != nil {
		return nil, fmt.Errorf("fail to get vault pubkeys ,err: %w", err)
	}
	var result openapi.VaultPubkeysResponse
	if err = json.Unmarshal(buf, &result); err != nil {
		return nil, fmt.Errorf("fail to unmarshal pubkeys: %w", err)
	}
	var addressPairs []PubKeyContractAddressPair
	for _, v := range append(result.Asgard, result.Inactive...) {
		if v.PubKeyEddsa != nil && *v.PubKeyEddsa != "" {
			kp := PubKeyContractAddressPair{
				PubKey:    common.PubKey(*v.PubKeyEddsa),
				Contracts: make(map[common.Chain]common.Address),
				Algo:      common.SigningAlgoEd25519,
			}
			for _, item := range v.Routers {
				kp.Contracts[common.Chain(*item.Chain)] = common.Address(*item.Router)
			}
			addressPairs = append(addressPairs, kp)
		}
		kp := PubKeyContractAddressPair{
			PubKey:    common.PubKey(v.PubKey),
			Contracts: make(map[common.Chain]common.Address),
			Algo:      common.SigningAlgoSecp256k1,
		}
		for _, item := range v.Routers {
			kp.Contracts[common.Chain(*item.Chain)] = common.Address(*item.Router)
		}
		addressPairs = append(addressPairs, kp)
	}
	return addressPairs, nil
}

// PostNetworkFee send network fee message to MAYANode
func (b *mayachainBridge) PostNetworkFee(height int64, chain common.Chain, transactionSize, transactionRate uint64) (common.TxID, error) {
	nodeStatus, err := b.FetchNodeStatus()
	if err != nil {
		return common.BlankTxID, fmt.Errorf("failed to get node status: %w", err)
	}

	if nodeStatus != stypes.NodeStatus_Active {
		return common.BlankTxID, nil
	}
	start := time.Now()
	defer func() {
		b.m.GetHistograms(metrics.SignToMayachainDuration).Observe(time.Since(start).Seconds())
	}()
	msg := stypes.NewMsgNetworkFee(height, chain, transactionSize, transactionRate, b.keys.GetSignerInfo().GetAddress())
	return b.Broadcast(msg)
}

// GetConstants from mayanode
func (b *mayachainBridge) GetConstants() (map[string]int64, error) {
	var result struct {
		Int64Values map[string]int64 `json:"int_64_values"`
	}
	buf, s, err := b.getWithPath(MayachainConstants)
	if err != nil {
		return nil, fmt.Errorf("fail to get constants: %w", err)
	}
	if s != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", s)
	}
	if err := json.Unmarshal(buf, &result); err != nil {
		return nil, fmt.Errorf("fail to unmarshal to json: %w", err)
	}
	return result.Int64Values, nil
}

// RagnarokInProgress is to query mayachain to check whether ragnarok had been triggered
func (b *mayachainBridge) RagnarokInProgress() (bool, error) {
	buf, s, err := b.getWithPath(RagnarokEndpoint)
	if err != nil {
		return false, fmt.Errorf("fail to get ragnarok status: %w", err)
	}
	if s != http.StatusOK {
		return false, fmt.Errorf("unexpected status code: %d", s)
	}
	var ragnarok bool
	if err := json.Unmarshal(buf, &ragnarok); err != nil {
		return false, fmt.Errorf("fail to unmarshal ragnarok status: %w", err)
	}
	return ragnarok, nil
}

// GetMayachainVersion retrieve mayachain version
func (b *mayachainBridge) GetMayachainVersion() (semver.Version, error) {
	buf, s, err := b.getWithPath(ChainVersionEndpoint)
	if err != nil {
		return semver.Version{}, fmt.Errorf("fail to get BASEChain version: %w", err)
	}
	if s != http.StatusOK {
		return semver.Version{}, fmt.Errorf("unexpected status code: %d", s)
	}
	var version openapi.VersionResponse
	if err := json.Unmarshal(buf, &version); err != nil {
		return semver.Version{}, fmt.Errorf("fail to unmarshal BASEChain version : %w", err)
	}
	return semver.MustParse(version.Current), nil
}

// GetMimir - get mimir settings
func (b *mayachainBridge) GetMimir(key string) (int64, error) {
	buf, s, err := b.getWithPath(MimirEndpoint + "/key/" + key)
	if err != nil {
		return 0, fmt.Errorf("fail to get mimir: %w", err)
	}
	if s != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d", s)
	}
	var value int64
	if err := json.Unmarshal(buf, &value); err != nil {
		return 0, fmt.Errorf("fail to unmarshal mimir: %w", err)
	}
	return value, nil
}

// PubKeyContractAddressPair is an entry to map pubkey and contract addresses
type PubKeyContractAddressPair struct {
	PubKey    common.PubKey
	Contracts map[common.Chain]common.Address
	Algo      common.SigningAlgo
}

// GetContractAddress retrieve the contract address from asgard
func (b *mayachainBridge) GetContractAddress() ([]PubKeyContractAddressPair, error) {
	buf, s, err := b.getWithPath(InboundAddressesEndpoint)
	if err != nil {
		return nil, fmt.Errorf("fail to get inbound addresses: %w", err)
	}
	if s != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", s)
	}
	type address struct {
		Chain   common.Chain   `json:"chain"`
		PubKey  common.PubKey  `json:"pub_key"`
		Address common.Address `json:"address"`
		Router  common.Address `json:"router"`
		Halted  bool           `json:"halted"`
	}
	var resp []address
	if err := json.Unmarshal(buf, &resp); err != nil {
		return nil, fmt.Errorf("fail to unmarshal response: %w", err)
	}
	var result []PubKeyContractAddressPair
	for _, item := range resp {
		exist := false
		for _, pair := range result {
			if item.PubKey.Equals(pair.PubKey) {
				pair.Contracts[item.Chain] = item.Router
				exist = true
				break
			}
		}
		if !exist {
			pair := PubKeyContractAddressPair{
				PubKey:    item.PubKey,
				Contracts: map[common.Chain]common.Address{},
			}
			pair.Contracts[item.Chain] = item.Router
			result = append(result, pair)
		}
	}
	return result, nil
}

// GetPools get pools from MAYAChain
func (b *mayachainBridge) GetPools() (stypes.Pools, error) {
	buf, s, err := b.getWithPath(PoolsEndpoint)
	if err != nil {
		return nil, fmt.Errorf("fail to get pools addresses: %w", err)
	}
	if s != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", s)
	}
	var pools stypes.Pools
	if err := json.Unmarshal(buf, &pools); err != nil {
		return nil, fmt.Errorf("fail to unmarshal pools from json: %w", err)
	}
	return pools, nil
}

// GetMAYAName get MAYAName from MAYAChain
func (b *mayachainBridge) GetMAYAName(name string) (stypes.MAYAName, error) {
	p := fmt.Sprintf(MAYANameEndpoint, name)
	buf, s, err := b.getWithPath(p)
	if err != nil {
		return stypes.MAYAName{}, fmt.Errorf("fail to get MAYAName: %w", err)
	}
	if s != http.StatusOK {
		return stypes.MAYAName{}, fmt.Errorf("unexpected status code: %d", s)
	}
	var tn stypes.MAYAName
	if err := json.Unmarshal(buf, &tn); err != nil {
		return stypes.MAYAName{}, fmt.Errorf("fail to unmarshal MAYANames from json: %w", err)
	}
	return tn, nil
}
