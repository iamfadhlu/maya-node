# \MayanamesApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**Mayaname**](MayanamesApi.md#Mayaname) | **Get** /mayachain/mayaname/{name} | 



## Mayaname

> []Mayaname1 Mayaname(ctx, name).Height(height).Execute()





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
    name := "name_example" // string | the mayanode to lookup
    height := int64(789) // int64 | optional block height, defaults to current tip (optional)

    configuration := openapiclient.NewConfiguration()
    apiClient := openapiclient.NewAPIClient(configuration)
    resp, r, err := apiClient.MayanamesApi.Mayaname(context.Background(), name).Height(height).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `MayanamesApi.Mayaname``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `Mayaname`: []Mayaname1
    fmt.Fprintf(os.Stdout, "Response from `MayanamesApi.Mayaname`: %v\n", resp)
}
```

### Path Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**name** | **string** | the mayanode to lookup | 

### Other Parameters

Other parameters are passed through a pointer to a apiMayanameRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **height** | **int64** | optional block height, defaults to current tip | 

### Return type

[**[]Mayaname1**](Mayaname1.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

