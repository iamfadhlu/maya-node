# CACAOPoolResponseProviders

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Units** | **string** | the units of CACAOPool owned by providers (including pending) | 
**PendingUnits** | **string** | the units of CACAOPool owned by providers that remain pending | 
**PendingCacao** | **string** | the amount of CACAO pending | 
**Value** | **string** | the value of the provider share of the CACAOPool (includes pending CACAO) | 
**Pnl** | **string** | the profit and loss of the provider share of the CACAOPool | 
**CurrentDeposit** | **string** | the current CACAO deposited by providers | 

## Methods

### NewCACAOPoolResponseProviders

`func NewCACAOPoolResponseProviders(units string, pendingUnits string, pendingCacao string, value string, pnl string, currentDeposit string, ) *CACAOPoolResponseProviders`

NewCACAOPoolResponseProviders instantiates a new CACAOPoolResponseProviders object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewCACAOPoolResponseProvidersWithDefaults

`func NewCACAOPoolResponseProvidersWithDefaults() *CACAOPoolResponseProviders`

NewCACAOPoolResponseProvidersWithDefaults instantiates a new CACAOPoolResponseProviders object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetUnits

`func (o *CACAOPoolResponseProviders) GetUnits() string`

GetUnits returns the Units field if non-nil, zero value otherwise.

### GetUnitsOk

`func (o *CACAOPoolResponseProviders) GetUnitsOk() (*string, bool)`

GetUnitsOk returns a tuple with the Units field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetUnits

`func (o *CACAOPoolResponseProviders) SetUnits(v string)`

SetUnits sets Units field to given value.


### GetPendingUnits

`func (o *CACAOPoolResponseProviders) GetPendingUnits() string`

GetPendingUnits returns the PendingUnits field if non-nil, zero value otherwise.

### GetPendingUnitsOk

`func (o *CACAOPoolResponseProviders) GetPendingUnitsOk() (*string, bool)`

GetPendingUnitsOk returns a tuple with the PendingUnits field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPendingUnits

`func (o *CACAOPoolResponseProviders) SetPendingUnits(v string)`

SetPendingUnits sets PendingUnits field to given value.


### GetPendingCacao

`func (o *CACAOPoolResponseProviders) GetPendingCacao() string`

GetPendingCacao returns the PendingCacao field if non-nil, zero value otherwise.

### GetPendingCacaoOk

`func (o *CACAOPoolResponseProviders) GetPendingCacaoOk() (*string, bool)`

GetPendingCacaoOk returns a tuple with the PendingCacao field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPendingCacao

`func (o *CACAOPoolResponseProviders) SetPendingCacao(v string)`

SetPendingCacao sets PendingCacao field to given value.


### GetValue

`func (o *CACAOPoolResponseProviders) GetValue() string`

GetValue returns the Value field if non-nil, zero value otherwise.

### GetValueOk

`func (o *CACAOPoolResponseProviders) GetValueOk() (*string, bool)`

GetValueOk returns a tuple with the Value field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetValue

`func (o *CACAOPoolResponseProviders) SetValue(v string)`

SetValue sets Value field to given value.


### GetPnl

`func (o *CACAOPoolResponseProviders) GetPnl() string`

GetPnl returns the Pnl field if non-nil, zero value otherwise.

### GetPnlOk

`func (o *CACAOPoolResponseProviders) GetPnlOk() (*string, bool)`

GetPnlOk returns a tuple with the Pnl field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPnl

`func (o *CACAOPoolResponseProviders) SetPnl(v string)`

SetPnl sets Pnl field to given value.


### GetCurrentDeposit

`func (o *CACAOPoolResponseProviders) GetCurrentDeposit() string`

GetCurrentDeposit returns the CurrentDeposit field if non-nil, zero value otherwise.

### GetCurrentDepositOk

`func (o *CACAOPoolResponseProviders) GetCurrentDepositOk() (*string, bool)`

GetCurrentDepositOk returns a tuple with the CurrentDeposit field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCurrentDeposit

`func (o *CACAOPoolResponseProviders) SetCurrentDeposit(v string)`

SetCurrentDeposit sets CurrentDeposit field to given value.



[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


