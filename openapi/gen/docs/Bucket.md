# Bucket

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**BalanceAsset** | **string** |  | 
**Asset** | **string** |  | 
**LPUnits** | **string** | the total pool liquidity provider units | 
**Status** | **string** |  | 

## Methods

### NewBucket

`func NewBucket(balanceAsset string, asset string, lPUnits string, status string, ) *Bucket`

NewBucket instantiates a new Bucket object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewBucketWithDefaults

`func NewBucketWithDefaults() *Bucket`

NewBucketWithDefaults instantiates a new Bucket object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetBalanceAsset

`func (o *Bucket) GetBalanceAsset() string`

GetBalanceAsset returns the BalanceAsset field if non-nil, zero value otherwise.

### GetBalanceAssetOk

`func (o *Bucket) GetBalanceAssetOk() (*string, bool)`

GetBalanceAssetOk returns a tuple with the BalanceAsset field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetBalanceAsset

`func (o *Bucket) SetBalanceAsset(v string)`

SetBalanceAsset sets BalanceAsset field to given value.


### GetAsset

`func (o *Bucket) GetAsset() string`

GetAsset returns the Asset field if non-nil, zero value otherwise.

### GetAssetOk

`func (o *Bucket) GetAssetOk() (*string, bool)`

GetAssetOk returns a tuple with the Asset field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAsset

`func (o *Bucket) SetAsset(v string)`

SetAsset sets Asset field to given value.


### GetLPUnits

`func (o *Bucket) GetLPUnits() string`

GetLPUnits returns the LPUnits field if non-nil, zero value otherwise.

### GetLPUnitsOk

`func (o *Bucket) GetLPUnitsOk() (*string, bool)`

GetLPUnitsOk returns a tuple with the LPUnits field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLPUnits

`func (o *Bucket) SetLPUnits(v string)`

SetLPUnits sets LPUnits field to given value.


### GetStatus

`func (o *Bucket) GetStatus() string`

GetStatus returns the Status field if non-nil, zero value otherwise.

### GetStatusOk

`func (o *Bucket) GetStatusOk() (*string, bool)`

GetStatusOk returns a tuple with the Status field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetStatus

`func (o *Bucket) SetStatus(v string)`

SetStatus sets Status field to given value.



[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


