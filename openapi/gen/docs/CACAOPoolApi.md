# \CACAOPoolApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**CacaoPool**](CACAOPoolApi.md#CacaoPool) | **Get** /mayachain/cacaopool | 
[**CacaoProvider**](CACAOPoolApi.md#CacaoProvider) | **Get** /mayachain/cacao_provider/{address} | 
[**CacaoProviders**](CACAOPoolApi.md#CacaoProviders) | **Get** /mayachain/cacao_providers | 



## CacaoPool

> CACAOPoolResponse CacaoPool(ctx).Height(height).Execute()





### Example

```go
package main

import (
    "context"
    "fmt"
    "os"
    openapiclient "./openapi"
)

func main() {
    height := int64(789) // int64 | optional block height, defaults to current tip (optional)

    configuration := openapiclient.NewConfiguration()
    apiClient := openapiclient.NewAPIClient(configuration)
    resp, r, err := apiClient.CACAOPoolApi.CacaoPool(context.Background()).Height(height).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `CACAOPoolApi.CacaoPool``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `CacaoPool`: CACAOPoolResponse
    fmt.Fprintf(os.Stdout, "Response from `CACAOPoolApi.CacaoPool`: %v\n", resp)
}
```

### Path Parameters



### Other Parameters

Other parameters are passed through a pointer to a apiCacaoPoolRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **height** | **int64** | optional block height, defaults to current tip | 

### Return type

[**CACAOPoolResponse**](CACAOPoolResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## CacaoProvider

> CACAOProvider CacaoProvider(ctx, address).Height(height).Execute()





### Example

```go
package main

import (
    "context"
    "fmt"
    "os"
    openapiclient "./openapi"
)

func main() {
    address := "maya1zupk5lmc84r2dh738a9g3zscavannjy3nzplwt" // string | 
    height := int64(789) // int64 | optional block height, defaults to current tip (optional)

    configuration := openapiclient.NewConfiguration()
    apiClient := openapiclient.NewAPIClient(configuration)
    resp, r, err := apiClient.CACAOPoolApi.CacaoProvider(context.Background(), address).Height(height).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `CACAOPoolApi.CacaoProvider``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `CacaoProvider`: CACAOProvider
    fmt.Fprintf(os.Stdout, "Response from `CACAOPoolApi.CacaoProvider`: %v\n", resp)
}
```

### Path Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**address** | **string** |  | 

### Other Parameters

Other parameters are passed through a pointer to a apiCacaoProviderRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **height** | **int64** | optional block height, defaults to current tip | 

### Return type

[**CACAOProvider**](CACAOProvider.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## CacaoProviders

> []CACAOProvider CacaoProviders(ctx).Height(height).Execute()





### Example

```go
package main

import (
    "context"
    "fmt"
    "os"
    openapiclient "./openapi"
)

func main() {
    height := int64(789) // int64 | optional block height, defaults to current tip (optional)

    configuration := openapiclient.NewConfiguration()
    apiClient := openapiclient.NewAPIClient(configuration)
    resp, r, err := apiClient.CACAOPoolApi.CacaoProviders(context.Background()).Height(height).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `CACAOPoolApi.CacaoProviders``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `CacaoProviders`: []CACAOProvider
    fmt.Fprintf(os.Stdout, "Response from `CACAOPoolApi.CacaoProviders`: %v\n", resp)
}
```

### Path Parameters



### Other Parameters

Other parameters are passed through a pointer to a apiCacaoProvidersRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **height** | **int64** | optional block height, defaults to current tip | 

### Return type

[**[]CACAOProvider**](CACAOProvider.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

