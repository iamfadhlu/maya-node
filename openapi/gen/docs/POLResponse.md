# POLResponse

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**CacaoDeposited** | **string** | total amount of CACAO deposited into the pools | 
**CacaoWithdrawn** | **string** | total amount of CACAO withdrawn from the pools | 
**Value** | **string** | total value of protocol&#39;s LP position in CACAO value | 
**Pnl** | **string** | profit and loss of protocol owned liquidity | 
**CurrentDeposit** | **string** | current amount of cacao deposited | 

## Methods

### NewPOLResponse

`func NewPOLResponse(cacaoDeposited string, cacaoWithdrawn string, value string, pnl string, currentDeposit string, ) *POLResponse`

NewPOLResponse instantiates a new POLResponse object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewPOLResponseWithDefaults

`func NewPOLResponseWithDefaults() *POLResponse`

NewPOLResponseWithDefaults instantiates a new POLResponse object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetCacaoDeposited

`func (o *POLResponse) GetCacaoDeposited() string`

GetCacaoDeposited returns the CacaoDeposited field if non-nil, zero value otherwise.

### GetCacaoDepositedOk

`func (o *POLResponse) GetCacaoDepositedOk() (*string, bool)`

GetCacaoDepositedOk returns a tuple with the CacaoDeposited field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCacaoDeposited

`func (o *POLResponse) SetCacaoDeposited(v string)`

SetCacaoDeposited sets CacaoDeposited field to given value.


### GetCacaoWithdrawn

`func (o *POLResponse) GetCacaoWithdrawn() string`

GetCacaoWithdrawn returns the CacaoWithdrawn field if non-nil, zero value otherwise.

### GetCacaoWithdrawnOk

`func (o *POLResponse) GetCacaoWithdrawnOk() (*string, bool)`

GetCacaoWithdrawnOk returns a tuple with the CacaoWithdrawn field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCacaoWithdrawn

`func (o *POLResponse) SetCacaoWithdrawn(v string)`

SetCacaoWithdrawn sets CacaoWithdrawn field to given value.


### GetValue

`func (o *POLResponse) GetValue() string`

GetValue returns the Value field if non-nil, zero value otherwise.

### GetValueOk

`func (o *POLResponse) GetValueOk() (*string, bool)`

GetValueOk returns a tuple with the Value field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetValue

`func (o *POLResponse) SetValue(v string)`

SetValue sets Value field to given value.


### GetPnl

`func (o *POLResponse) GetPnl() string`

GetPnl returns the Pnl field if non-nil, zero value otherwise.

### GetPnlOk

`func (o *POLResponse) GetPnlOk() (*string, bool)`

GetPnlOk returns a tuple with the Pnl field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPnl

`func (o *POLResponse) SetPnl(v string)`

SetPnl sets Pnl field to given value.


### GetCurrentDeposit

`func (o *POLResponse) GetCurrentDeposit() string`

GetCurrentDeposit returns the CurrentDeposit field if non-nil, zero value otherwise.

### GetCurrentDepositOk

`func (o *POLResponse) GetCurrentDepositOk() (*string, bool)`

GetCurrentDepositOk returns a tuple with the CurrentDeposit field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCurrentDeposit

`func (o *POLResponse) SetCurrentDeposit(v string)`

SetCurrentDeposit sets CurrentDeposit field to given value.



[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


