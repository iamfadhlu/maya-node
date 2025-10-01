# NetworkResponse

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**BondRewardCacao** | **string** | total amount of cacao awarded to node operators | 
**TotalBondUnits** | **string** | total bonded cacao | 
**TotalReserve** | **string** | total reserve cacao | 
**TotalAsgard** | **string** | total asgard cacao | 
**GasSpentCacao** | **string** | Sum of the gas the network has spent to send outbounds | 
**GasWithheldCacao** | **string** | Sum of the gas withheld from users to cover outbound gas | 
**OutboundFeeMultiplier** | Pointer to **string** | Current outbound fee multiplier, in basis points | [optional] 

## Methods

### NewNetworkResponse

`func NewNetworkResponse(bondRewardCacao string, totalBondUnits string, totalReserve string, totalAsgard string, gasSpentCacao string, gasWithheldCacao string, ) *NetworkResponse`

NewNetworkResponse instantiates a new NetworkResponse object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewNetworkResponseWithDefaults

`func NewNetworkResponseWithDefaults() *NetworkResponse`

NewNetworkResponseWithDefaults instantiates a new NetworkResponse object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetBondRewardCacao

`func (o *NetworkResponse) GetBondRewardCacao() string`

GetBondRewardCacao returns the BondRewardCacao field if non-nil, zero value otherwise.

### GetBondRewardCacaoOk

`func (o *NetworkResponse) GetBondRewardCacaoOk() (*string, bool)`

GetBondRewardCacaoOk returns a tuple with the BondRewardCacao field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetBondRewardCacao

`func (o *NetworkResponse) SetBondRewardCacao(v string)`

SetBondRewardCacao sets BondRewardCacao field to given value.


### GetTotalBondUnits

`func (o *NetworkResponse) GetTotalBondUnits() string`

GetTotalBondUnits returns the TotalBondUnits field if non-nil, zero value otherwise.

### GetTotalBondUnitsOk

`func (o *NetworkResponse) GetTotalBondUnitsOk() (*string, bool)`

GetTotalBondUnitsOk returns a tuple with the TotalBondUnits field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTotalBondUnits

`func (o *NetworkResponse) SetTotalBondUnits(v string)`

SetTotalBondUnits sets TotalBondUnits field to given value.


### GetTotalReserve

`func (o *NetworkResponse) GetTotalReserve() string`

GetTotalReserve returns the TotalReserve field if non-nil, zero value otherwise.

### GetTotalReserveOk

`func (o *NetworkResponse) GetTotalReserveOk() (*string, bool)`

GetTotalReserveOk returns a tuple with the TotalReserve field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTotalReserve

`func (o *NetworkResponse) SetTotalReserve(v string)`

SetTotalReserve sets TotalReserve field to given value.


### GetTotalAsgard

`func (o *NetworkResponse) GetTotalAsgard() string`

GetTotalAsgard returns the TotalAsgard field if non-nil, zero value otherwise.

### GetTotalAsgardOk

`func (o *NetworkResponse) GetTotalAsgardOk() (*string, bool)`

GetTotalAsgardOk returns a tuple with the TotalAsgard field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTotalAsgard

`func (o *NetworkResponse) SetTotalAsgard(v string)`

SetTotalAsgard sets TotalAsgard field to given value.


### GetGasSpentCacao

`func (o *NetworkResponse) GetGasSpentCacao() string`

GetGasSpentCacao returns the GasSpentCacao field if non-nil, zero value otherwise.

### GetGasSpentCacaoOk

`func (o *NetworkResponse) GetGasSpentCacaoOk() (*string, bool)`

GetGasSpentCacaoOk returns a tuple with the GasSpentCacao field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetGasSpentCacao

`func (o *NetworkResponse) SetGasSpentCacao(v string)`

SetGasSpentCacao sets GasSpentCacao field to given value.


### GetGasWithheldCacao

`func (o *NetworkResponse) GetGasWithheldCacao() string`

GetGasWithheldCacao returns the GasWithheldCacao field if non-nil, zero value otherwise.

### GetGasWithheldCacaoOk

`func (o *NetworkResponse) GetGasWithheldCacaoOk() (*string, bool)`

GetGasWithheldCacaoOk returns a tuple with the GasWithheldCacao field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetGasWithheldCacao

`func (o *NetworkResponse) SetGasWithheldCacao(v string)`

SetGasWithheldCacao sets GasWithheldCacao field to given value.


### GetOutboundFeeMultiplier

`func (o *NetworkResponse) GetOutboundFeeMultiplier() string`

GetOutboundFeeMultiplier returns the OutboundFeeMultiplier field if non-nil, zero value otherwise.

### GetOutboundFeeMultiplierOk

`func (o *NetworkResponse) GetOutboundFeeMultiplierOk() (*string, bool)`

GetOutboundFeeMultiplierOk returns a tuple with the OutboundFeeMultiplier field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetOutboundFeeMultiplier

`func (o *NetworkResponse) SetOutboundFeeMultiplier(v string)`

SetOutboundFeeMultiplier sets OutboundFeeMultiplier field to given value.

### HasOutboundFeeMultiplier

`func (o *NetworkResponse) HasOutboundFeeMultiplier() bool`

HasOutboundFeeMultiplier returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


