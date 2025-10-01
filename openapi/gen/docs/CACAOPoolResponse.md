# CACAOPoolResponse

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Pol** | [**POL**](POL.md) |  | 
**Providers** | [**CACAOPoolResponseProviders**](CACAOPoolResponseProviders.md) |  | 
**Reserve** | [**CACAOPoolResponseReserve**](CACAOPoolResponseReserve.md) |  | 

## Methods

### NewCACAOPoolResponse

`func NewCACAOPoolResponse(pol POL, providers CACAOPoolResponseProviders, reserve CACAOPoolResponseReserve, ) *CACAOPoolResponse`

NewCACAOPoolResponse instantiates a new CACAOPoolResponse object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewCACAOPoolResponseWithDefaults

`func NewCACAOPoolResponseWithDefaults() *CACAOPoolResponse`

NewCACAOPoolResponseWithDefaults instantiates a new CACAOPoolResponse object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetPol

`func (o *CACAOPoolResponse) GetPol() POL`

GetPol returns the Pol field if non-nil, zero value otherwise.

### GetPolOk

`func (o *CACAOPoolResponse) GetPolOk() (*POL, bool)`

GetPolOk returns a tuple with the Pol field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPol

`func (o *CACAOPoolResponse) SetPol(v POL)`

SetPol sets Pol field to given value.


### GetProviders

`func (o *CACAOPoolResponse) GetProviders() CACAOPoolResponseProviders`

GetProviders returns the Providers field if non-nil, zero value otherwise.

### GetProvidersOk

`func (o *CACAOPoolResponse) GetProvidersOk() (*CACAOPoolResponseProviders, bool)`

GetProvidersOk returns a tuple with the Providers field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetProviders

`func (o *CACAOPoolResponse) SetProviders(v CACAOPoolResponseProviders)`

SetProviders sets Providers field to given value.


### GetReserve

`func (o *CACAOPoolResponse) GetReserve() CACAOPoolResponseReserve`

GetReserve returns the Reserve field if non-nil, zero value otherwise.

### GetReserveOk

`func (o *CACAOPoolResponse) GetReserveOk() (*CACAOPoolResponseReserve, bool)`

GetReserveOk returns a tuple with the Reserve field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetReserve

`func (o *CACAOPoolResponse) SetReserve(v CACAOPoolResponseReserve)`

SetReserve sets Reserve field to given value.



[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


