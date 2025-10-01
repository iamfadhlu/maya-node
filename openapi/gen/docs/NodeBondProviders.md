# NodeBondProviders

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**NodeAddress** | Pointer to **string** |  | [optional] 
**NodeOperatorFee** | **string** |  | 
**Providers** | [**[]NodeBondProvider**](NodeBondProvider.md) |  | 

## Methods

### NewNodeBondProviders

`func NewNodeBondProviders(nodeOperatorFee string, providers []NodeBondProvider, ) *NodeBondProviders`

NewNodeBondProviders instantiates a new NodeBondProviders object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewNodeBondProvidersWithDefaults

`func NewNodeBondProvidersWithDefaults() *NodeBondProviders`

NewNodeBondProvidersWithDefaults instantiates a new NodeBondProviders object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetNodeAddress

`func (o *NodeBondProviders) GetNodeAddress() string`

GetNodeAddress returns the NodeAddress field if non-nil, zero value otherwise.

### GetNodeAddressOk

`func (o *NodeBondProviders) GetNodeAddressOk() (*string, bool)`

GetNodeAddressOk returns a tuple with the NodeAddress field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNodeAddress

`func (o *NodeBondProviders) SetNodeAddress(v string)`

SetNodeAddress sets NodeAddress field to given value.

### HasNodeAddress

`func (o *NodeBondProviders) HasNodeAddress() bool`

HasNodeAddress returns a boolean if a field has been set.

### GetNodeOperatorFee

`func (o *NodeBondProviders) GetNodeOperatorFee() string`

GetNodeOperatorFee returns the NodeOperatorFee field if non-nil, zero value otherwise.

### GetNodeOperatorFeeOk

`func (o *NodeBondProviders) GetNodeOperatorFeeOk() (*string, bool)`

GetNodeOperatorFeeOk returns a tuple with the NodeOperatorFee field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNodeOperatorFee

`func (o *NodeBondProviders) SetNodeOperatorFee(v string)`

SetNodeOperatorFee sets NodeOperatorFee field to given value.


### GetProviders

`func (o *NodeBondProviders) GetProviders() []NodeBondProvider`

GetProviders returns the Providers field if non-nil, zero value otherwise.

### GetProvidersOk

`func (o *NodeBondProviders) GetProvidersOk() (*[]NodeBondProvider, bool)`

GetProvidersOk returns a tuple with the Providers field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetProviders

`func (o *NodeBondProviders) SetProviders(v []NodeBondProvider)`

SetProviders sets Providers field to given value.



[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


