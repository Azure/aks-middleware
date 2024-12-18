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
* 3. [gRPC client](#gRPCclient)
 	* 3.1. [mdforward](#mdforward)
 	* 3.2. [autologger (api request/response logger)](#autologgerapirequestresponselogger-1)
 	* 3.3. [retry](#retry)
* 4. [HTTP server](#HTTPserver)
 	* 4.1. [requestid](#requestid-1)
 	* 4.2. [ctxlogger (applogger)](#ctxloggerapplogger-1)
 	* 4.3. [logging (api request/response logger)](#loggingapirequestresponselogger)
 	* 4.4. [recovery](#recovery-1)
 	* 4.5. [inputvalidate](#inputvalidate)
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

## 1. <a name='Usage'></a>Usage

Run the following command in the root directory of this module.

```bash
make
```

## 2. <a name='gRPCserver'></a>gRPC server

### 2.1. <a name='requestid'></a>requestid

ServerInterceptor. It adds x-request-id to MD if there is no such entry. This interceptor needs to be registered first so that its request-id can be used by autologger and applogger.

### 2.2. <a name='ctxloggerapplogger'></a>ctxlogger (applogger)

ServerInterceptor. It adds a logger to ctx with important information (e.g., request-id, method-name) already populated. App's handler code can get the logger and output additional information. Needs to be registered after the requestid interceptor.

Only information in MD could be propagated to dependencies. The logger added via ctx.WithValue() is local to the ctx and won't be propagated to dependencies.

##### <a name='logfiltering'></a>log filtering

This is a feature of ctxlogger that allows the user to annotate within api.proto what request payload variables should be logged or not. Logic in the ctxlogger.go decides what to log based off the annoatation. The "loggable" annotation is true by default, so the user is only required to annotate the variables that should not be logged.

Ancestors need to be loggable in order to examine the annotations on the leaf nodes. For example, if loggable value for "address" is false, it will not look for the loggable values of "city", "state", "zipcode", etc.

The source code for the external loggable field option is located at <https://github.com/toma3233/loggable>. Whenever it is updated, it is pushed to the following Buf Schema Registry as a module: <https://buf.build/service-hub/loggable>. Command used to authenticate before pushing to the BSR is "buf registry login" which prompts for BSR username and BSR token. The .netrc file is updated with these credentials.

---

The following gRPC server interceptors are implemented by other open source projects. We enabled them in our default server interceptor list. [return []grpc.UnaryServerInterceptor{](https://github.com/Azure/aks-middleware/blob/afd08e520d5d70f1b24910d26c2a686a0468feaa/interceptor/interceptor.go#L137-L138)

---

### 2.3. <a name='autologgerapirequestresponselogger'></a>autologger (api request/response logger)

This package only provides the default logger implementation to log request result and latency. We use go-grpc-middleware's logging.UnaryServerIncerceptor() and logging.UnaryClientInterceptor() to achieve this. This package is not an interceptor.

### 2.4. <a name='recovery'></a>recovery

This is a server side interceptor that is implemented to handle panics in the code

Once a panic is detected, it is handled by a custom recovery function defined in recoveryOpts.go which logs gRPC code "unknown" along with the file name/line number of where the panic occurred and a link to the repo. The program continues and does not terminate.

### 2.5. <a name='protovalidate'></a>protovalidate

This is a server side interceptor that we use to validate the requests that are coming in from the client

The validation rules are generated and executed by the protovalidate-go library, and the rules are applied to the variables in the api.proto file

## 3. <a name='gRPCclient'></a>gRPC client

### 3.1. <a name='mdforward'></a>mdforward

ClientInterceptor. It propagates MD from incoming to outgoing. Only need to be used in servers. No need to be used in a pure client app.

---

The following gRPC client interceptors are implemented by other open source projects. We enabled them in our default client interceptor list. [[return []grpc.UnaryClientInterceptor{](https://github.com/Azure/aks-middleware/blob/afd08e520d5d70f1b24910d26c2a686a0468feaa/interceptor/interceptor.go#L76-L77)

---

### 3.2. <a name='autologgerapirequestresponselogger-1'></a>autologger (api request/response logger)

This package only provides the default logger implementation to log request result and latency. We use go-grpc-middleware's logging.UnaryServerIncerceptor() and logging.UnaryClientInterceptor() to achieve this. This package is not an interceptor.

### 3.3. <a name='retry'></a>retry

This is a client interceptor that resends a request based on the gRPC code that is returned from the server

All options for the interceptor (i.e. max retries, codes to retry on, type of backoff) are defined in the retryOpts.go file

## 4. <a name='HTTPserver'></a>HTTP server

The `httpmw` folder contains middleware for HTTP servers built using the `gorilla/mux` package. These are similar to gRPC server interceptors.

### 4.1. <a name='requestid-1'></a>requestid

It extract Azure Resource Manager required HTTP headers from the request and put them as metadata of the incoming context.

The current implementation is not consistent with its gRPC counterpart.

### 4.2. <a name='ctxloggerapplogger-1'></a>ctxlogger (applogger)

Missing.

### 4.3. <a name='loggingapirequestresponselogger'></a>logging (api request/response logger)

The logging middleware logs details about each HTTP request and response, including the request method, URL, status code, and duration.

##### <a name='Usage-1'></a>Usage

To use the logging middleware, you need to create a logger and then apply the middleware to your router.

Code example is included in the test code.

### 4.4. <a name='recovery-1'></a>recovery

The recovery middleware recovers from panics in your HTTP handlers and logs the error. It can use a custom panic handler if provided.

##### <a name='Usage-1'></a>Usage

To use the recovery middleware, you need to create a logger and apply the middleware to your router. You can also provide a custom panic handler.

Code example is included in the test code

### 4.5. <a name='inputvalidate'></a>inputvalidate

Missing.

## 5. <a name='HTTPclientviaAzureSDK'></a>HTTP client via Azure SDK

### 5.1. <a name='mdforward-1'></a>mdforward

Missing.

### 5.2. <a name='policyapirequestresponselogger'></a>policy (api request/response logger)

The `policy` package provides a logging policy for HTTP requests made via the Azure SDK for Go.

The logging policy logs details about each HTTP request and response, including the request method, URL, status code, and duration.

##### <a name='Usage-1'></a>Usage

To use the logging policy, you need to create a logger and then apply the policy to your HTTP client.

Code example is included in the test code

### 5.3. <a name='retry-1'></a>retry

Missing.

## 6. <a name='HTTPclientviaDirectHTTPrequest'></a>HTTP client via Direct HTTP request

### 6.1. <a name='mdforward-1'></a>mdforward

Missing.

### 6.2. <a name='Restloggerapirequestresponselogger'></a>Restlogger (api request/response logger)

The restlogger package provides a logging round tripper for HTTP clients.

The logging round tripper logs details about each HTTP request and response, including the request method, URL, status code, and duration.

##### <a name='Usage-1'></a>Usage

To use the logging round tripper, you need to create a logger and then apply the round tripper to your HTTP client.

Code example is included in the test code

### 6.3. <a name='retry-1'></a>retry

Missing.

## 7. <a name='Project'></a>Project

> This repo has been populated by an initial template to help get you started. Please
> make sure to update the content to build a great experience for community-building.

As the maintainer of this project, please make a few updates:

- Improving this README.MD file to provide a great experience
- Updating SUPPORT.MD with content about this project's support experience
- Understanding the security reporting process in SECURITY.MD
- Remove this section from the README

### 7.1. <a name='Contributing'></a>Contributing

This project welcomes contributions and suggestions.  Most contributions require you to agree to a
Contributor License Agreement (CLA) declaring that you have the right to, and actually do, grant us
the rights to use your contribution. For details, visit <https://cla.opensource.microsoft.com>.

When you submit a pull request, a CLA bot will automatically determine whether you need to provide
a CLA and decorate the PR appropriately (e.g., status check, comment). Simply follow the instructions
provided by the bot. You will only need to do this once across all repos using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or
contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.

### 7.2. <a name='Trademarks'></a>Trademarks

This project may contain trademarks or logos for projects, products, or services. Authorized use of Microsoft
trademarks or logos is subject to and must follow
[Microsoft's Trademark & Brand Guidelines](https://www.microsoft.com/en-us/legal/intellectualproperty/trademarks/usage/general).
Use of Microsoft trademarks or logos in modified versions of this project must not cause confusion or imply Microsoft sponsorship.
Any use of third-party trademarks or logos are subject to those third-party's policies.
