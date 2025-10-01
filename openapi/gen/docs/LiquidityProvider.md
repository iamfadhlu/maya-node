# LiquidityProvider

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Asset** | **string** |  | 
**CacaoAddress** | Pointer to **string** |  | [optional] 
**AssetAddress** | Pointer to **string** |  | [optional] 
**LastAddHeight** | Pointer to **int64** |  | [optional] 
**LastWithdrawHeight** | Pointer to **int64** |  | [optional] 
**Units** | **string** |  | 
**PendingCacao** | **string** |  | 
**PendingAsset** | **string** |  | 
**PendingTxId** | Pointer to **string** |  | [optional] 
**CacaoDepositValue** | **string** |  | 
**AssetDepositValue** | **string** |  | 
**NodeBondAddress** | Pointer to **string** |  | [optional] 
**WithdrawCounter** | **string** |  | 
**LastWithdrawCounterHeight** | Pointer to **int64** |  | [optional] 
**BondedNodes** | [**[]LPBondedNode**](LPBondedNode.md) |  | 
**CacaoRedeemValue** | Pointer to **string** |  | [optional] 
**AssetRedeemValue** | Pointer to **string** |  | [optional] 
**LuviDepositValue** | Pointer to **string** |  | [optional] 
**LuviRedeemValue** | Pointer to **string** |  | [optional] 
**LuviGrowthPct** | Pointer to **string** |  | [optional] 

## Methods

### NewLiquidityProvider

`func NewLiquidityProvider(asset string, units string, pendingCacao string, pendingAsset string, cacaoDepositValue string, assetDepositValue string, withdrawCounter string, bondedNodes []LPBondedNode, ) *LiquidityProvider`

NewLiquidityProvider instantiates a new LiquidityProvider object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewLiquidityProviderWithDefaults

`func NewLiquidityProviderWithDefaults() *LiquidityProvider`

NewLiquidityProviderWithDefaults instantiates a new LiquidityProvider object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetAsset

`func (o *LiquidityProvider) GetAsset() string`

GetAsset returns the Asset field if non-nil, zero value otherwise.

### GetAssetOk

`func (o *LiquidityProvider) GetAssetOk() (*string, bool)`

GetAssetOk returns a tuple with the Asset field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAsset

`func (o *LiquidityProvider) SetAsset(v string)`

SetAsset sets Asset field to given value.


### GetCacaoAddress

`func (o *LiquidityProvider) GetCacaoAddress() string`

GetCacaoAddress returns the CacaoAddress field if non-nil, zero value otherwise.

### GetCacaoAddressOk

`func (o *LiquidityProvider) GetCacaoAddressOk() (*string, bool)`

GetCacaoAddressOk returns a tuple with the CacaoAddress field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCacaoAddress

`func (o *LiquidityProvider) SetCacaoAddress(v string)`

SetCacaoAddress sets CacaoAddress field to given value.

### HasCacaoAddress

`func (o *LiquidityProvider) HasCacaoAddress() bool`

HasCacaoAddress returns a boolean if a field has been set.

### GetAssetAddress

`func (o *LiquidityProvider) GetAssetAddress() string`

GetAssetAddress returns the AssetAddress field if non-nil, zero value otherwise.

### GetAssetAddressOk

`func (o *LiquidityProvider) GetAssetAddressOk() (*string, bool)`

GetAssetAddressOk returns a tuple with the AssetAddress field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAssetAddress

`func (o *LiquidityProvider) SetAssetAddress(v string)`

SetAssetAddress sets AssetAddress field to given value.

### HasAssetAddress

`func (o *LiquidityProvider) HasAssetAddress() bool`

HasAssetAddress returns a boolean if a field has been set.

### GetLastAddHeight

`func (o *LiquidityProvider) GetLastAddHeight() int64`

GetLastAddHeight returns the LastAddHeight field if non-nil, zero value otherwise.

### GetLastAddHeightOk

`func (o *LiquidityProvider) GetLastAddHeightOk() (*int64, bool)`

GetLastAddHeightOk returns a tuple with the LastAddHeight field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLastAddHeight

`func (o *LiquidityProvider) SetLastAddHeight(v int64)`

SetLastAddHeight sets LastAddHeight field to given value.

### HasLastAddHeight

`func (o *LiquidityProvider) HasLastAddHeight() bool`

HasLastAddHeight returns a boolean if a field has been set.

### GetLastWithdrawHeight

`func (o *LiquidityProvider) GetLastWithdrawHeight() int64`

GetLastWithdrawHeight returns the LastWithdrawHeight field if non-nil, zero value otherwise.

### GetLastWithdrawHeightOk

`func (o *LiquidityProvider) GetLastWithdrawHeightOk() (*int64, bool)`

GetLastWithdrawHeightOk returns a tuple with the LastWithdrawHeight field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLastWithdrawHeight

`func (o *LiquidityProvider) SetLastWithdrawHeight(v int64)`

SetLastWithdrawHeight sets LastWithdrawHeight field to given value.

### HasLastWithdrawHeight

`func (o *LiquidityProvider) HasLastWithdrawHeight() bool`

HasLastWithdrawHeight returns a boolean if a field has been set.

### GetUnits

`func (o *LiquidityProvider) GetUnits() string`

GetUnits returns the Units field if non-nil, zero value otherwise.

### GetUnitsOk

`func (o *LiquidityProvider) GetUnitsOk() (*string, bool)`

GetUnitsOk returns a tuple with the Units field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetUnits

`func (o *LiquidityProvider) SetUnits(v string)`

SetUnits sets Units field to given value.


### GetPendingCacao

`func (o *LiquidityProvider) GetPendingCacao() string`

GetPendingCacao returns the PendingCacao field if non-nil, zero value otherwise.

### GetPendingCacaoOk

`func (o *LiquidityProvider) GetPendingCacaoOk() (*string, bool)`

GetPendingCacaoOk returns a tuple with the PendingCacao field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPendingCacao

`func (o *LiquidityProvider) SetPendingCacao(v string)`

SetPendingCacao sets PendingCacao field to given value.


### GetPendingAsset

`func (o *LiquidityProvider) GetPendingAsset() string`

GetPendingAsset returns the PendingAsset field if non-nil, zero value otherwise.

### GetPendingAssetOk

`func (o *LiquidityProvider) GetPendingAssetOk() (*string, bool)`

GetPendingAssetOk returns a tuple with the PendingAsset field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPendingAsset

`func (o *LiquidityProvider) SetPendingAsset(v string)`

SetPendingAsset sets PendingAsset field to given value.


### GetPendingTxId

`func (o *LiquidityProvider) GetPendingTxId() string`

GetPendingTxId returns the PendingTxId field if non-nil, zero value otherwise.

### GetPendingTxIdOk

`func (o *LiquidityProvider) GetPendingTxIdOk() (*string, bool)`

GetPendingTxIdOk returns a tuple with the PendingTxId field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPendingTxId

`func (o *LiquidityProvider) SetPendingTxId(v string)`

SetPendingTxId sets PendingTxId field to given value.

### HasPendingTxId

`func (o *LiquidityProvider) HasPendingTxId() bool`

HasPendingTxId returns a boolean if a field has been set.

### GetCacaoDepositValue

`func (o *LiquidityProvider) GetCacaoDepositValue() string`

GetCacaoDepositValue returns the CacaoDepositValue field if non-nil, zero value otherwise.

### GetCacaoDepositValueOk

`func (o *LiquidityProvider) GetCacaoDepositValueOk() (*string, bool)`

GetCacaoDepositValueOk returns a tuple with the CacaoDepositValue field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCacaoDepositValue

`func (o *LiquidityProvider) SetCacaoDepositValue(v string)`

SetCacaoDepositValue sets CacaoDepositValue field to given value.


### GetAssetDepositValue

`func (o *LiquidityProvider) GetAssetDepositValue() string`

GetAssetDepositValue returns the AssetDepositValue field if non-nil, zero value otherwise.

### GetAssetDepositValueOk

`func (o *LiquidityProvider) GetAssetDepositValueOk() (*string, bool)`

GetAssetDepositValueOk returns a tuple with the AssetDepositValue field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAssetDepositValue

`func (o *LiquidityProvider) SetAssetDepositValue(v string)`

SetAssetDepositValue sets AssetDepositValue field to given value.


### GetNodeBondAddress

`func (o *LiquidityProvider) GetNodeBondAddress() string`

GetNodeBondAddress returns the NodeBondAddress field if non-nil, zero value otherwise.

### GetNodeBondAddressOk

`func (o *LiquidityProvider) GetNodeBondAddressOk() (*string, bool)`

GetNodeBondAddressOk returns a tuple with the NodeBondAddress field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNodeBondAddress

`func (o *LiquidityProvider) SetNodeBondAddress(v string)`

SetNodeBondAddress sets NodeBondAddress field to given value.

### HasNodeBondAddress

`func (o *LiquidityProvider) HasNodeBondAddress() bool`

HasNodeBondAddress returns a boolean if a field has been set.

### GetWithdrawCounter

`func (o *LiquidityProvider) GetWithdrawCounter() string`

GetWithdrawCounter returns the WithdrawCounter field if non-nil, zero value otherwise.

### GetWithdrawCounterOk

`func (o *LiquidityProvider) GetWithdrawCounterOk() (*string, bool)`

GetWithdrawCounterOk returns a tuple with the WithdrawCounter field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetWithdrawCounter

`func (o *LiquidityProvider) SetWithdrawCounter(v string)`

SetWithdrawCounter sets WithdrawCounter field to given value.


### GetLastWithdrawCounterHeight

`func (o *LiquidityProvider) GetLastWithdrawCounterHeight() int64`

GetLastWithdrawCounterHeight returns the LastWithdrawCounterHeight field if non-nil, zero value otherwise.

### GetLastWithdrawCounterHeightOk

`func (o *LiquidityProvider) GetLastWithdrawCounterHeightOk() (*int64, bool)`

GetLastWithdrawCounterHeightOk returns a tuple with the LastWithdrawCounterHeight field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLastWithdrawCounterHeight

`func (o *LiquidityProvider) SetLastWithdrawCounterHeight(v int64)`

SetLastWithdrawCounterHeight sets LastWithdrawCounterHeight field to given value.

### HasLastWithdrawCounterHeight

`func (o *LiquidityProvider) HasLastWithdrawCounterHeight() bool`

HasLastWithdrawCounterHeight returns a boolean if a field has been set.

### GetBondedNodes

`func (o *LiquidityProvider) GetBondedNodes() []LPBondedNode`

GetBondedNodes returns the BondedNodes field if non-nil, zero value otherwise.

### GetBondedNodesOk

`func (o *LiquidityProvider) GetBondedNodesOk() (*[]LPBondedNode, bool)`

GetBondedNodesOk returns a tuple with the BondedNodes field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetBondedNodes

`func (o *LiquidityProvider) SetBondedNodes(v []LPBondedNode)`

SetBondedNodes sets BondedNodes field to given value.


### GetCacaoRedeemValue

`func (o *LiquidityProvider) GetCacaoRedeemValue() string`

GetCacaoRedeemValue returns the CacaoRedeemValue field if non-nil, zero value otherwise.

### GetCacaoRedeemValueOk

`func (o *LiquidityProvider) GetCacaoRedeemValueOk() (*string, bool)`

GetCacaoRedeemValueOk returns a tuple with the CacaoRedeemValue field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCacaoRedeemValue

`func (o *LiquidityProvider) SetCacaoRedeemValue(v string)`

SetCacaoRedeemValue sets CacaoRedeemValue field to given value.

### HasCacaoRedeemValue

`func (o *LiquidityProvider) HasCacaoRedeemValue() bool`

HasCacaoRedeemValue returns a boolean if a field has been set.

### GetAssetRedeemValue

`func (o *LiquidityProvider) GetAssetRedeemValue() string`

GetAssetRedeemValue returns the AssetRedeemValue field if non-nil, zero value otherwise.

### GetAssetRedeemValueOk

`func (o *LiquidityProvider) GetAssetRedeemValueOk() (*string, bool)`

GetAssetRedeemValueOk returns a tuple with the AssetRedeemValue field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAssetRedeemValue

`func (o *LiquidityProvider) SetAssetRedeemValue(v string)`

SetAssetRedeemValue sets AssetRedeemValue field to given value.

### HasAssetRedeemValue

`func (o *LiquidityProvider) HasAssetRedeemValue() bool`

HasAssetRedeemValue returns a boolean if a field has been set.

### GetLuviDepositValue

`func (o *LiquidityProvider) GetLuviDepositValue() string`

GetLuviDepositValue returns the LuviDepositValue field if non-nil, zero value otherwise.

### GetLuviDepositValueOk

`func (o *LiquidityProvider) GetLuviDepositValueOk() (*string, bool)`

GetLuviDepositValueOk returns a tuple with the LuviDepositValue field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLuviDepositValue

`func (o *LiquidityProvider) SetLuviDepositValue(v string)`

SetLuviDepositValue sets LuviDepositValue field to given value.

### HasLuviDepositValue

`func (o *LiquidityProvider) HasLuviDepositValue() bool`

HasLuviDepositValue returns a boolean if a field has been set.

### GetLuviRedeemValue

`func (o *LiquidityProvider) GetLuviRedeemValue() string`

GetLuviRedeemValue returns the LuviRedeemValue field if non-nil, zero value otherwise.

### GetLuviRedeemValueOk

`func (o *LiquidityProvider) GetLuviRedeemValueOk() (*string, bool)`

GetLuviRedeemValueOk returns a tuple with the LuviRedeemValue field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLuviRedeemValue

`func (o *LiquidityProvider) SetLuviRedeemValue(v string)`

SetLuviRedeemValue sets LuviRedeemValue field to given value.

### HasLuviRedeemValue

`func (o *LiquidityProvider) HasLuviRedeemValue() bool`

HasLuviRedeemValue returns a boolean if a field has been set.

### GetLuviGrowthPct

`func (o *LiquidityProvider) GetLuviGrowthPct() string`

GetLuviGrowthPct returns the LuviGrowthPct field if non-nil, zero value otherwise.

### GetLuviGrowthPctOk

`func (o *LiquidityProvider) GetLuviGrowthPctOk() (*string, bool)`

GetLuviGrowthPctOk returns a tuple with the LuviGrowthPct field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLuviGrowthPct

`func (o *LiquidityProvider) SetLuviGrowthPct(v string)`

SetLuviGrowthPct sets LuviGrowthPct field to given value.

### HasLuviGrowthPct

`func (o *LiquidityProvider) HasLuviGrowthPct() bool`

HasLuviGrowthPct returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


