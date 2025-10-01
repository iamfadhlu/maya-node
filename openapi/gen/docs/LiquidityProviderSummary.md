# LiquidityProviderSummary

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

## Methods

### NewLiquidityProviderSummary

`func NewLiquidityProviderSummary(asset string, units string, pendingCacao string, pendingAsset string, cacaoDepositValue string, assetDepositValue string, ) *LiquidityProviderSummary`

NewLiquidityProviderSummary instantiates a new LiquidityProviderSummary object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewLiquidityProviderSummaryWithDefaults

`func NewLiquidityProviderSummaryWithDefaults() *LiquidityProviderSummary`

NewLiquidityProviderSummaryWithDefaults instantiates a new LiquidityProviderSummary object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetAsset

`func (o *LiquidityProviderSummary) GetAsset() string`

GetAsset returns the Asset field if non-nil, zero value otherwise.

### GetAssetOk

`func (o *LiquidityProviderSummary) GetAssetOk() (*string, bool)`

GetAssetOk returns a tuple with the Asset field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAsset

`func (o *LiquidityProviderSummary) SetAsset(v string)`

SetAsset sets Asset field to given value.


### GetCacaoAddress

`func (o *LiquidityProviderSummary) GetCacaoAddress() string`

GetCacaoAddress returns the CacaoAddress field if non-nil, zero value otherwise.

### GetCacaoAddressOk

`func (o *LiquidityProviderSummary) GetCacaoAddressOk() (*string, bool)`

GetCacaoAddressOk returns a tuple with the CacaoAddress field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCacaoAddress

`func (o *LiquidityProviderSummary) SetCacaoAddress(v string)`

SetCacaoAddress sets CacaoAddress field to given value.

### HasCacaoAddress

`func (o *LiquidityProviderSummary) HasCacaoAddress() bool`

HasCacaoAddress returns a boolean if a field has been set.

### GetAssetAddress

`func (o *LiquidityProviderSummary) GetAssetAddress() string`

GetAssetAddress returns the AssetAddress field if non-nil, zero value otherwise.

### GetAssetAddressOk

`func (o *LiquidityProviderSummary) GetAssetAddressOk() (*string, bool)`

GetAssetAddressOk returns a tuple with the AssetAddress field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAssetAddress

`func (o *LiquidityProviderSummary) SetAssetAddress(v string)`

SetAssetAddress sets AssetAddress field to given value.

### HasAssetAddress

`func (o *LiquidityProviderSummary) HasAssetAddress() bool`

HasAssetAddress returns a boolean if a field has been set.

### GetLastAddHeight

`func (o *LiquidityProviderSummary) GetLastAddHeight() int64`

GetLastAddHeight returns the LastAddHeight field if non-nil, zero value otherwise.

### GetLastAddHeightOk

`func (o *LiquidityProviderSummary) GetLastAddHeightOk() (*int64, bool)`

GetLastAddHeightOk returns a tuple with the LastAddHeight field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLastAddHeight

`func (o *LiquidityProviderSummary) SetLastAddHeight(v int64)`

SetLastAddHeight sets LastAddHeight field to given value.

### HasLastAddHeight

`func (o *LiquidityProviderSummary) HasLastAddHeight() bool`

HasLastAddHeight returns a boolean if a field has been set.

### GetLastWithdrawHeight

`func (o *LiquidityProviderSummary) GetLastWithdrawHeight() int64`

GetLastWithdrawHeight returns the LastWithdrawHeight field if non-nil, zero value otherwise.

### GetLastWithdrawHeightOk

`func (o *LiquidityProviderSummary) GetLastWithdrawHeightOk() (*int64, bool)`

GetLastWithdrawHeightOk returns a tuple with the LastWithdrawHeight field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLastWithdrawHeight

`func (o *LiquidityProviderSummary) SetLastWithdrawHeight(v int64)`

SetLastWithdrawHeight sets LastWithdrawHeight field to given value.

### HasLastWithdrawHeight

`func (o *LiquidityProviderSummary) HasLastWithdrawHeight() bool`

HasLastWithdrawHeight returns a boolean if a field has been set.

### GetUnits

`func (o *LiquidityProviderSummary) GetUnits() string`

GetUnits returns the Units field if non-nil, zero value otherwise.

### GetUnitsOk

`func (o *LiquidityProviderSummary) GetUnitsOk() (*string, bool)`

GetUnitsOk returns a tuple with the Units field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetUnits

`func (o *LiquidityProviderSummary) SetUnits(v string)`

SetUnits sets Units field to given value.


### GetPendingCacao

`func (o *LiquidityProviderSummary) GetPendingCacao() string`

GetPendingCacao returns the PendingCacao field if non-nil, zero value otherwise.

### GetPendingCacaoOk

`func (o *LiquidityProviderSummary) GetPendingCacaoOk() (*string, bool)`

GetPendingCacaoOk returns a tuple with the PendingCacao field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPendingCacao

`func (o *LiquidityProviderSummary) SetPendingCacao(v string)`

SetPendingCacao sets PendingCacao field to given value.


### GetPendingAsset

`func (o *LiquidityProviderSummary) GetPendingAsset() string`

GetPendingAsset returns the PendingAsset field if non-nil, zero value otherwise.

### GetPendingAssetOk

`func (o *LiquidityProviderSummary) GetPendingAssetOk() (*string, bool)`

GetPendingAssetOk returns a tuple with the PendingAsset field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPendingAsset

`func (o *LiquidityProviderSummary) SetPendingAsset(v string)`

SetPendingAsset sets PendingAsset field to given value.


### GetPendingTxId

`func (o *LiquidityProviderSummary) GetPendingTxId() string`

GetPendingTxId returns the PendingTxId field if non-nil, zero value otherwise.

### GetPendingTxIdOk

`func (o *LiquidityProviderSummary) GetPendingTxIdOk() (*string, bool)`

GetPendingTxIdOk returns a tuple with the PendingTxId field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPendingTxId

`func (o *LiquidityProviderSummary) SetPendingTxId(v string)`

SetPendingTxId sets PendingTxId field to given value.

### HasPendingTxId

`func (o *LiquidityProviderSummary) HasPendingTxId() bool`

HasPendingTxId returns a boolean if a field has been set.

### GetCacaoDepositValue

`func (o *LiquidityProviderSummary) GetCacaoDepositValue() string`

GetCacaoDepositValue returns the CacaoDepositValue field if non-nil, zero value otherwise.

### GetCacaoDepositValueOk

`func (o *LiquidityProviderSummary) GetCacaoDepositValueOk() (*string, bool)`

GetCacaoDepositValueOk returns a tuple with the CacaoDepositValue field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCacaoDepositValue

`func (o *LiquidityProviderSummary) SetCacaoDepositValue(v string)`

SetCacaoDepositValue sets CacaoDepositValue field to given value.


### GetAssetDepositValue

`func (o *LiquidityProviderSummary) GetAssetDepositValue() string`

GetAssetDepositValue returns the AssetDepositValue field if non-nil, zero value otherwise.

### GetAssetDepositValueOk

`func (o *LiquidityProviderSummary) GetAssetDepositValueOk() (*string, bool)`

GetAssetDepositValueOk returns a tuple with the AssetDepositValue field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAssetDepositValue

`func (o *LiquidityProviderSummary) SetAssetDepositValue(v string)`

SetAssetDepositValue sets AssetDepositValue field to given value.



[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


