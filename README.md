<!-- markdownlint-disable MD004 -->

# AKS middleware for Go

<!-- vscode-markdown-toc -->
* 1. [Usage](#Usage)
* 2. [gRPC server](#gRPCserver)
 	* 2.1. [requestid](#requestid)
 	* 2.2. [ctxlogger (applogger)](#ctxloggerapplogger)
 	* 2.3. [autologger (api request/response logger)](#autologgerapirequestresponselogger)
 	* 2.4. [recovery](#recovery)
 	* 2.5. [protovalidate](#protovalidate)
	* 2.6. [responseheader](#responseheader)
* 3. [gRPC client](#gRPCclient)
 	* 3.1. [mdforward](#mdforward)
 	* 3.2. [autologger (api request/response logger)](#autologgerapirequestresponselogger-1)
 	* 3.3. [retry](#retry)
* 4. [HTTP server](#HTTPserver)
 	* 4.1. [requestid](#requestid-1)
 	* 4.2. [logging (api request/response logger)](#loggingapirequestresponselogger)
 	* 4.3. [ctxlogger (applogger)](#httpctxlogger)
 	* 4.4. [recovery](#recovery-1)
 	* 4.5. [inputvalidate](#inputvalidate)
	* 4.6  [operationrequest](#operationrequest)
	* 4.7. [otel audit logging](#otelauditlogging)
* 5. [HTTP client via Azure SDK](#HTTPclientviaAzureSDK)
 	* 5.1. [mdforward](#mdforward-1)
 	* 5.2. [policy (api request/response logger)](#policyapirequestresponselogger)
 	* 5.3. [retry](#retry-1)
* 6. [HTTP client via Direct HTTP request](#HTTPclientviaDirectHTTPrequest)
 	* 6.1. [mdforward](#mdforward-1)
 	* 6.2. [Restlogger (api request/response logger)](#Restloggerapirequestresponselogger)
 	* 6.3. [retry](#retry-1)
* 7. [Project](#Project)
 	* 7.1. [Contributing](#Contributing)
 	* 7.2. [Trademarks](#Trademarks)

<!-- vscode-markdown-toc-config
	numbering=true
	autoSave=true
	/vscode-markdown-toc-config -->
<!-- /vscode-markdown-toc -->

This directory is the root of the aks-middleware module. It implements interceptors to aid common service tasks. See the list below for details.

## 1. <a id='Usage'></a>Usage

Run the following command in the root directory of this module.

```bash
make
```

## 2. <a id='gRPCserver'></a>gRPC server

The following gRPC server interceptors are used by default. Some interceptors are implemented in this repo. Some are implemented in existing open source projects and are used by this repo.

### 2.1. <a id='requestid'></a>requestid

It adds x-request-id to MD if there is no such entry. This interceptor needs to be registered first so that its request-id can be used by autologger and ctxlogger.

### 2.2. <a id='ctxloggerapplogger'></a>ctxlogger (applogger)

It adds a logger to ctx with important information (e.g., request-id, method-name) already populated. App's handler code can get the logger and output additional information. Needs to be registered after the requestid interceptor.

The logger added via ctx.WithValue() is local to the ctx and won't be propagated to dependencies. Only information in MD could be propagated to dependencies. 

##### <a id='logfiltering'></a>log filtering

This is a feature of ctxlogger that allows the user to annotate within api.proto what request payload variables should be logged or not. Logic in the ctxlogger.go decides what to log based off the annotation. The "loggable" annotation is true by default, so the user is only required to annotate the variables that should not be logged.

Ancestors need to be loggable in order to examine the annotations on the leaf nodes. For example, if loggable value for "address" is false, it will not look for the loggable values of "city", "state", "zipcode", etc.

The source code for the external loggable field option is located at <https://github.com/toma3233/loggable>. Whenever it is updated, it is pushed to the following Buf Schema Registry as a module: <https://buf.build/service-hub/loggable>. Command used to authenticate before pushing to the BSR is "buf registry login" which prompts for BSR username and BSR token. The .netrc file is updated with these credentials.

---

The following gRPC server interceptors are implemented by other open source projects. We enabled them in our default server interceptor list. [return []grpc.UnaryServerInterceptor{](https://github.com/Azure/aks-middleware/blob/afd08e520d5d70f1b24910d26c2a686a0468feaa/interceptor/interceptor.go#L137-L138)

---

### 2.3. <a id='autologgerapirequestresponselogger'></a>autologger (api request/response logger)

This is to provide the default logger implementation to log incoming request/response result and latency. We use go-grpc-middleware's logging.UnaryServerIncerceptor() to achieve this.

### 2.4. <a id='recovery'></a>recovery

This is to handle panics in the code.

Once a panic is detected, it is handled by a custom recovery function defined in recoveryOpts.go which logs gRPC code "unknown" along with the file name/line number of where the panic occurred and a link to the repo. The program continues and does not terminate.

### 2.5. <a id='protovalidate'></a>protovalidate

This is to validate the requests from the client.

The validation rules are generated and executed by the protovalidate-go library, and the rules are applied to the variables in the api.proto file.

### 2.6. <a id='responseheader'></a>responseheader

This is to copy the metadata that the server receives from the incoming request into the response header.

The interceptor accepts a map of strings that it uses to determine which metadata will be copied into the response.

## 3. <a id='gRPCclient'></a>gRPC client

The following gRPC client interceptors are used by default.

### 3.1. <a id='mdforward'></a>mdforward

It propagates MD from incoming to outgoing. Only need to be used in servers that have both incoming requests and outgoing requests. No need to be used in a pure client app that doesn't have incoming requests.

---

The following gRPC client interceptors are implemented by other open source projects. We enabled them in our default client interceptor list. [return []grpc.UnaryClientInterceptor{](https://github.com/Azure/aks-middleware/blob/afd08e520d5d70f1b24910d26c2a686a0468feaa/interceptor/interceptor.go#L76-L77)

---

### 3.2. <a id='autologgerapirequestresponselogger-1'></a>autologger (api request/response logger)

This package only provides the default logger implementation to log outgoing request/response result and latency. We use go-grpc-middleware's logging.UnaryClientInterceptor() to achieve this.

### 3.3. <a id='retry'></a>retry

It resends a request based on the gRPC code that is returned from the server

All options for the interceptor (i.e. max retries, codes to retry on, type of backoff) are defined in the retryOpts.go file

## 4. <a id='HTTPserver'></a>HTTP server

The `httpmw` folder contains middleware for HTTP servers built using the `gorilla/mux` package. These are similar to gRPC server interceptors.

### 4.1. <a id='requestid-1'></a>requestid

It extracts Azure Resource Manager required HTTP headers from the request and put them as metadata of the incoming context.

The current implementation is not consistent with its gRPC counterpart.


### 4.2. <a id='loggingapirequestresponselogger'></a>logging (api request/response logger)

The logging middleware logs details about each HTTP request and response, including the request method, URL, status code, and duration.

##### <a id='Usage-1'></a>Usage

To use the logging middleware, you need to create a logger and then apply the middleware to your router.

Code example is included in the test code.

### 4.3. <a id='httpctxlogger'></a>ctx logging (applogging)

The context logger middleware adds a logger to the context that can be used to log out anything that happens during the request lifecycle. These logs get sent to the CtxLog table and can be used for debugging issues in your service. The caller has the option to pass in extra attributes to log out info beyond the defaults the middleware logs. These logs are in json format and can be be unmarshaled and used like so: 
| extend logjson = parse_json(log)
| where logjson.operationID == "ce99dc86-930a-4011-b196-fd8a4fa3c958"

 Addtionally, this middleware can be used in conjunction with the OperationRequest middleware to grab operation specific info from the context and include it in the context log attributes. 

##### <a id='Usage-1'></a>Usage

To add extra logging fields and values, you can provide an `extraAttributes` map and/or specify operation-specific fields (`opFields`) to include in the logging context. 

- The `extraAttributes` map contains static key-value pairs that are merged with the default attributes.
- The `opFields` slice specifies operation-specific fields to include in the logging context. These fields are extracted from the `OperationRequest` object in the request context.

For instance, if you wanted to include specific operation request fields and custom attributes, you could configure the middleware like so:

```go
import (
    "github.com/Azure/aks-middleware/http/server/contextlogger"
    "github.com/Azure/aks-middleware/http/server/operationrequest"
    "github.com/gorilla/mux"
    "log/slog"
)

func main() {
    router := mux.NewRouter()

    // Define operation request options
    opFields := []string{
        "SubscriptionID",
        "ResourceGroup",
        "ResourceName",
        "APIVersion",
        "CorrelationID",
        "OperationID",
    }

    extraAttributes := map[string]interface{}{
        "customKey": "customValue",
    }

    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

    // Apply the context logger middleware
    router.Use(contextlogger.New(*logger, extraAttributes, opFields))

    // Define your routes
    router.HandleFunc("/subscriptions/sub123/resourceGroups/rg123/providers/Microsoft.Test/resourceType1/resourceName/default?api-version=2021-12-01", func(w http.ResponseWriter, r *http.Request) {
        l := contextlogger.GetLogger(r.Context())
        if l != nil {
            l.Info("Example log message")
        }
        w.WriteHeader(http.StatusOK)
    })

    http.ListenAndServe(":8080", router)
}
```

The `BuildAttributes` function in the middleware automatically merges the default attributes, extra attributes, and operation-specific fields into the logging context. This ensures that all relevant information is included in the logs.

More examples included in the test code

### 4.4. <a id='recovery-1'></a>recovery

The recovery middleware recovers from panics in your HTTP handlers and logs the error. It can use a custom panic handler if provided.

##### <a id='Usage-1'></a>Usage

To use the recovery middleware, you need to create a logger and apply the middleware to your router. You can also provide a custom panic handler.

Code example is included in the test code

### 4.5. <a id='inputvalidate'></a>inputvalidate

Missing.

### 4.6. <a id='operationrequest'></a>operationrequest

The `operationrequest` middleware is designed to handle and enrich incoming HTTP requests with additional context and metadata. It extracts common fields from the request, such as subscription ID, resource group, correlation ID, operation ID, and more. It also allows for customization through the use of a customizer function.

The interceptor uses the HTTP header defined by `common.RequestAcsOperationIDHeader` to determine the operation identifier:

- **Provided ID:** If the request includes a header with a valid operation ID, that value is used.
- **Generated ID:** If the operation ID header is missing, a new UUID is automatically generated.

This ensures that each request has a unique `OperationID`, either supplied by the caller or generated by the system.

#### <a id='Usage-2'></a>Usage

This middleware is intended to be used by RPs that whose URLs follow the below pattern:
```go
        routePattern := "/subscriptions/{subscriptionID}/resourceGroups/{resourceGroup}/providers/{resourceProvider}/{resourceType}/{resourceName}/default?api-version=XXXX-XX-XX"

```

To use the `operationrequest` middleware, you need to create an instance of the middleware with the desired options and apply it to your router. Ensure this middleware is only applied to routes that require the Operation Request struct to be created and injected into the context. The interceptor is only meant to be used for "operation related" paths, otherwise it will return an internal error if it deems the URL to be incomplete/invalid (i.e. missing api version).

In an actual REST service, there can be multiple paths (i.e. health check). In this case, caller should create a subrouter to apply the middleware to certain paths only. Example included in integration test.

```go
import (
    "github.com/Azure/aks-middleware/http/server/operationrequest"
    "github.com/gorilla/mux"
)

type MyExtras struct {
    MyCustomHeader string
}

func myCustomizer(extras *MyExtras, headers http.Header, vars map[string]string) error {
    if customHeader := headers.Get("X-My-Custom-Header"); customHeader != "" {
        extras.MyCustomHeader = customHeader
    }
    return nil
}

func main() {
    router := mux.NewRouter()
    opts := operationrequest.OperationRequestOptions[MyExtras]{
        Extras:     MyExtras{},
        Customizer: myCustomizer,
    }
    router.Use(operationrequest.NewOperationRequest("region-name", opts))
    // Add your routes here
    http.ListenAndServe(":8080", router)
}
```

This middleware will enrich the incoming requests with additional context and metadata, making it easier to handle and process the requests in your application.
### 4.7 <a id='otelauditlogging'></a>OTel Audit Logging

The OTEL audit middleware is designed to provide a unified way of logging security events for all Azure internal services. It acts as a logging client that sends logs to a Unix domain socket or TCP connection, eliminating the need for specific knowledge of Azure environments, Geneva accounts, namespaces, endpoints, and certificates. The middleware relies on the Geneva Agent (mdsd) to push logs to the Geneva backend.

It is based off of the go otel audit framework here: https://github.com/microsoft/go-otel-audit

Key Features:
- **Unified Logging**: The middleware provides a generic way to record every operation made using an OTEL client.
- **Security and Compliance**: Audit logs are essential for meeting security needs, customer expectations, and compliance requirements for standards such as FISMA/FedRAMP, EU Model Clauses, and ISO 270013.
- **Middleware Implementation**: The middleware includes a function that takes care of sending the audit logs, and the mw gathers other information by inspecting request/response elements. The URLs must follow the general Azure Resource Manager pattern so the necessary information can be extracted from the URL
    - ````go
      routePattern := "/{subscriptionID}/resourceGroups/{resourceGroup}/providers/{resourceProvider}/{resourceType}/{resourceName}"
      ````

- **Customization**:  Consumers can pass their own configuration to customize the audit logging further. Available options include:
  - **CustomOperationDescs** (map[string]string): Custom descriptions for different operations. Key must match what is returned by GetMethodInfo() for the given URL
  - **CustomOperationCategories** (map[string]msgs.OperationCategory): Custom mappings for operation categories. Key must match what is returned by GetMethodInfo() for the given URL
  - **OperationAccessLevel:** The access level for the operation.
  - **ExcludeAuditEvents:** A map of HTTP methods to URL substrings; if a request URL contains any of these substrings for the given method, the audit event is excluded.

Usage examples included in test code

## 5. <a id='HTTPclientviaAzureSDK'></a>HTTP client via Azure SDK

### 5.1. <a id='mdforward-1'></a>mdforward

Strictly speaking, it is not metadata forwarding. But it serves the same purpose: instead of propagating the id from the incoming context to the outgoing context, the id is propagated from incoming context to Azure HTTP request header.

If we choose to not implement it, we can let Azure SDK to decide the request id. The mapping between the Azure request id and the opertion/correlation id will be logged by the policy middleware below.

### 5.2. <a id='policyapirequestresponselogger'></a>policy (api request/response logger)

The `policy` package provides a logging policy for HTTP requests made via the Azure SDK for Go.

The logging policy logs details about each HTTP request and response, including the request method, URL, status code, and duration.

##### <a id='Usage-1'></a>Usage

To use the logging policy, you need to create a logger and then apply the policy to your HTTP client.

Code example is included in the test code

### 5.3. <a id='retry-1'></a>retry

Missing.

## 6. <a id='HTTPclientviaDirectHTTPrequest'></a>HTTP client via Direct HTTP request

### 6.1. <a id='mdforward-1'></a>mdforward

Missing.

### 6.2. <a id='Restloggerapirequestresponselogger'></a>Restlogger (api request/response logger)

The restlogger package provides a logging round tripper for HTTP clients.

The logging round tripper logs details about each HTTP request and response, including the request method, URL, status code, and duration.

##### <a id='Usage-1'></a>Usage

To use the logging round tripper, you need to create a logger and then apply the round tripper to your HTTP client.

Code example is included in the test code

### 6.3. <a id='retry-1'></a>retry

Missing.

## 7. <a id='Project'></a>Project

> This repo has been populated by an initial template to help get you started. Please
> make sure to update the content to build a great experience for community-building.

As the maintainer of this project, please make a few updates:

- Improving this README.MD file to provide a great experience
- Updating SUPPORT.MD with content about this project's support experience
- Understanding the security reporting process in SECURITY.MD
- Remove this section from the README

### 7.1. <a id='Contributing'></a>Contributing

This project welcomes contributions and suggestions.  Most contributions require you to agree to a
Contributor License Agreement (CLA) declaring that you have the right to, and actually do, grant us
the rights to use your contribution. For details, visit <https://cla.opensource.microsoft.com>.

When you submit a pull request, a CLA bot will automatically determine whether you need to provide
a CLA and decorate the PR appropriately (e.g., status check, comment). Simply follow the instructions
provided by the bot. You will only need to do this once across all repos using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or
contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.

### 7.2. <a id='Trademarks'></a>Trademarks

This project may contain trademarks or logos for projects, products, or services. Authorized use of Microsoft
trademarks or logos is subject to and must follow
[Microsoft's Trademark & Brand Guidelines](https://www.microsoft.com/en-us/legal/intellectualproperty/trademarks/usage/general).
Use of Microsoft trademarks or logos in modified versions of this project must not cause confusion or imply Microsoft sponsorship.
Any use of third-party trademarks or logos are subject to those third-party's policies.
