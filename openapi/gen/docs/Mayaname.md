# Mayaname

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Name** | Pointer to **string** |  | [optional] 
**ExpireBlockHeight** | Pointer to **int64** |  | [optional] 
**Owner** | Pointer to **string** |  | [optional] 
**PreferredAsset** | **string** |  | 
**PreferredAssetSwapThresholdCacao** | Pointer to **string** | Amount of CACAO currently required to swap to preferred asset (this is variable based on outbound fee of the asset). | [optional] 
**AffiliateCollectorCacao** | Pointer to **string** | Amount of CACAO currently accrued by this mayaname in affiliate fees waiting to be swapped to preferred asset. | [optional] 
**Aliases** | [**[]MayanameAlias**](MayanameAlias.md) |  | 
**AffiliateBps** | Pointer to **int64** | Affiliate basis points for calculating affiliate fees, which are applied as the default basis points when the MAYAName is listed as an affiliate in swap memo. | [optional] 
**Subaffiliates** | Pointer to [**[]MayanameSubaffiliate**](MayanameSubaffiliate.md) | List of subaffiliates and the corresponding affiliate basis points. If a MAYAName is specified as an affiliate in a swap memo, the shares of the affiliate fee are distributed among the listed subaffiliates based on the basis points assigned to each subaffiliate. | [optional] 

## Methods

### NewMayaname

`func NewMayaname(preferredAsset string, aliases []MayanameAlias, ) *Mayaname`

NewMayaname instantiates a new Mayaname object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewMayanameWithDefaults

`func NewMayanameWithDefaults() *Mayaname`

NewMayanameWithDefaults instantiates a new Mayaname object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetName

`func (o *Mayaname) GetName() string`

GetName returns the Name field if non-nil, zero value otherwise.

### GetNameOk

`func (o *Mayaname) GetNameOk() (*string, bool)`

GetNameOk returns a tuple with the Name field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetName

`func (o *Mayaname) SetName(v string)`

SetName sets Name field to given value.

### HasName

`func (o *Mayaname) HasName() bool`

HasName returns a boolean if a field has been set.

### GetExpireBlockHeight

`func (o *Mayaname) GetExpireBlockHeight() int64`

GetExpireBlockHeight returns the ExpireBlockHeight field if non-nil, zero value otherwise.

### GetExpireBlockHeightOk

`func (o *Mayaname) GetExpireBlockHeightOk() (*int64, bool)`

GetExpireBlockHeightOk returns a tuple with the ExpireBlockHeight field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetExpireBlockHeight

`func (o *Mayaname) SetExpireBlockHeight(v int64)`

SetExpireBlockHeight sets ExpireBlockHeight field to given value.

### HasExpireBlockHeight

`func (o *Mayaname) HasExpireBlockHeight() bool`

HasExpireBlockHeight returns a boolean if a field has been set.

### GetOwner

`func (o *Mayaname) GetOwner() string`

GetOwner returns the Owner field if non-nil, zero value otherwise.

### GetOwnerOk

`func (o *Mayaname) GetOwnerOk() (*string, bool)`

GetOwnerOk returns a tuple with the Owner field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetOwner

`func (o *Mayaname) SetOwner(v string)`

SetOwner sets Owner field to given value.

### HasOwner

`func (o *Mayaname) HasOwner() bool`

HasOwner returns a boolean if a field has been set.

### GetPreferredAsset

`func (o *Mayaname) GetPreferredAsset() string`

GetPreferredAsset returns the PreferredAsset field if non-nil, zero value otherwise.

### GetPreferredAssetOk

`func (o *Mayaname) GetPreferredAssetOk() (*string, bool)`

GetPreferredAssetOk returns a tuple with the PreferredAsset field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPreferredAsset

`func (o *Mayaname) SetPreferredAsset(v string)`

SetPreferredAsset sets PreferredAsset field to given value.


### GetPreferredAssetSwapThresholdCacao

`func (o *Mayaname) GetPreferredAssetSwapThresholdCacao() string`

GetPreferredAssetSwapThresholdCacao returns the PreferredAssetSwapThresholdCacao field if non-nil, zero value otherwise.

### GetPreferredAssetSwapThresholdCacaoOk

`func (o *Mayaname) GetPreferredAssetSwapThresholdCacaoOk() (*string, bool)`

GetPreferredAssetSwapThresholdCacaoOk returns a tuple with the PreferredAssetSwapThresholdCacao field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPreferredAssetSwapThresholdCacao

`func (o *Mayaname) SetPreferredAssetSwapThresholdCacao(v string)`

SetPreferredAssetSwapThresholdCacao sets PreferredAssetSwapThresholdCacao field to given value.

### HasPreferredAssetSwapThresholdCacao

`func (o *Mayaname) HasPreferredAssetSwapThresholdCacao() bool`

HasPreferredAssetSwapThresholdCacao returns a boolean if a field has been set.

### GetAffiliateCollectorCacao

`func (o *Mayaname) GetAffiliateCollectorCacao() string`

GetAffiliateCollectorCacao returns the AffiliateCollectorCacao field if non-nil, zero value otherwise.

### GetAffiliateCollectorCacaoOk

`func (o *Mayaname) GetAffiliateCollectorCacaoOk() (*string, bool)`

GetAffiliateCollectorCacaoOk returns a tuple with the AffiliateCollectorCacao field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAffiliateCollectorCacao

`func (o *Mayaname) SetAffiliateCollectorCacao(v string)`

SetAffiliateCollectorCacao sets AffiliateCollectorCacao field to given value.

### HasAffiliateCollectorCacao

`func (o *Mayaname) HasAffiliateCollectorCacao() bool`

HasAffiliateCollectorCacao returns a boolean if a field has been set.

### GetAliases

`func (o *Mayaname) GetAliases() []MayanameAlias`

GetAliases returns the Aliases field if non-nil, zero value otherwise.

### GetAliasesOk

`func (o *Mayaname) GetAliasesOk() (*[]MayanameAlias, bool)`

GetAliasesOk returns a tuple with the Aliases field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAliases

`func (o *Mayaname) SetAliases(v []MayanameAlias)`

SetAliases sets Aliases field to given value.


### GetAffiliateBps

`func (o *Mayaname) GetAffiliateBps() int64`

GetAffiliateBps returns the AffiliateBps field if non-nil, zero value otherwise.

### GetAffiliateBpsOk

`func (o *Mayaname) GetAffiliateBpsOk() (*int64, bool)`

GetAffiliateBpsOk returns a tuple with the AffiliateBps field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAffiliateBps

`func (o *Mayaname) SetAffiliateBps(v int64)`

SetAffiliateBps sets AffiliateBps field to given value.

### HasAffiliateBps

`func (o *Mayaname) HasAffiliateBps() bool`

HasAffiliateBps returns a boolean if a field has been set.

### GetSubaffiliates

`func (o *Mayaname) GetSubaffiliates() []MayanameSubaffiliate`

GetSubaffiliates returns the Subaffiliates field if non-nil, zero value otherwise.

### GetSubaffiliatesOk

`func (o *Mayaname) GetSubaffiliatesOk() (*[]MayanameSubaffiliate, bool)`

GetSubaffiliatesOk returns a tuple with the Subaffiliates field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSubaffiliates

`func (o *Mayaname) SetSubaffiliates(v []MayanameSubaffiliate)`

SetSubaffiliates sets Subaffiliates field to given value.

### HasSubaffiliates

`func (o *Mayaname) HasSubaffiliates() bool`

HasSubaffiliates returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


