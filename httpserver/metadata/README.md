**gRPC-Gateway is a plugin that allows us to accept incoming REST requests and converts them to gRPC calls**
  - layer over the  gRPC service that uses the api.proto file to generate an api.pb.gw.go file
  - This is the reverse proxy that is responsible for translating gRPC into RESTful JSON APIs

![grpc-gateway.png](/docs/images/grpc-gateway.png)

REST is still a very popular protocol used by many, and by allowing our gRPC service to accept REST requests, we are making our service more accessible. Especially with cases like ARM, which does not communicate using gRPC. 

Using the gateway, we can also propagate http headers to our backend gRPC service by utilizing runtime serveMuxOptions. In our [aks-middleware](https://github.com/Azure/aks-middleware/blob/main/httpmw/metadata/metadata.go), we define a NewMetadataMiddleware which accepts an metadataToHeader map, headerToMetadata map, and returns an array of runtime.ServeMuxOptions. This contains two options:
- runtime.WithMetadata
    - this loops through the headerToMetdata map, and checks if the headerName exists in the request headers. If it does, it creates a new metadata pair with the metadata key and value. This is how we are able to transform http request headers into gRPC metadata
- runtime.WithOutgoingHeaderMatcher
    - When sending the gRPC response back to the mux, there may be certain metadata that we do not want to convert to a http header and send back as part of the response. This matcher uses the metadataToHeader map to ensure only the headers we allow will be returned in the response

These options can be easily imported from the aks-middleware and used within the gRPC-gateway mux like so:
```go
import aksMiddlewareMetadata "github.com/Azure/aks-middleware/httpmw/metadata"

metadataToHeader := map[string]string{
    "custom-header":      "X-Custom-Header",
    "another-header":     "X-Another-Header",
}
headerToMetadata := map[string]string{
    "X-Custom-Header":      "custom-header",
    "X-Another-Header":     "another-header",
			
}

gwmux := runtime.NewServeMux(
        aksMiddlewareMetadata.NewMetadataMiddleware(headerToMetadata, metadataToHeader)...,
    )
```

In our middleware, we also have a [responseheader interceptor](https://github.com/Azure/aks-middleware/blob/main/responseheader/responseheader.go) that copies the desired incoming metadata to the gRPC response headers. We use this interceptor to send back selected metadata which is then converted to headers by the outgoing header matcher.

The following is a end-to-end request lifecycle diagram to show how http headers are processed:
![updated diagram.png](/docs/images/request-lifecycle.png)


