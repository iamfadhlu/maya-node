# NodeBondProvider

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**BondAddress** | **string** |  | 
**Bonded** | **bool** |  | 
**Reward** | **string** |  | 
**Pools** | **map[string]string** | Pools the Bond Provider is bonded from to that Node Address | 

## Methods

### NewNodeBondProvider

`func NewNodeBondProvider(bondAddress string, bonded bool, reward string, pools map[string]string, ) *NodeBondProvider`

NewNodeBondProvider instantiates a new NodeBondProvider object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewNodeBondProviderWithDefaults

`func NewNodeBondProviderWithDefaults() *NodeBondProvider`

NewNodeBondProviderWithDefaults instantiates a new NodeBondProvider object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetBondAddress

`func (o *NodeBondProvider) GetBondAddress() string`

GetBondAddress returns the BondAddress field if non-nil, zero value otherwise.

### GetBondAddressOk

`func (o *NodeBondProvider) GetBondAddressOk() (*string, bool)`

GetBondAddressOk returns a tuple with the BondAddress field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetBondAddress

`func (o *NodeBondProvider) SetBondAddress(v string)`

SetBondAddress sets BondAddress field to given value.


### GetBonded

`func (o *NodeBondProvider) GetBonded() bool`

GetBonded returns the Bonded field if non-nil, zero value otherwise.

### GetBondedOk

`func (o *NodeBondProvider) GetBondedOk() (*bool, bool)`

GetBondedOk returns a tuple with the Bonded field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetBonded

`func (o *NodeBondProvider) SetBonded(v bool)`

SetBonded sets Bonded field to given value.


### GetReward

`func (o *NodeBondProvider) GetReward() string`

GetReward returns the Reward field if non-nil, zero value otherwise.

### GetRewardOk

`func (o *NodeBondProvider) GetRewardOk() (*string, bool)`

GetRewardOk returns a tuple with the Reward field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetReward

`func (o *NodeBondProvider) SetReward(v string)`

SetReward sets Reward field to given value.


### GetPools

`func (o *NodeBondProvider) GetPools() map[string]string`

GetPools returns the Pools field if non-nil, zero value otherwise.

### GetPoolsOk

`func (o *NodeBondProvider) GetPoolsOk() (*map[string]string, bool)`

GetPoolsOk returns a tuple with the Pools field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPools

`func (o *NodeBondProvider) SetPools(v map[string]string)`

SetPools sets Pools field to given value.



[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


