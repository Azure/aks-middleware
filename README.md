<!-- markdownlint-disable MD004 -->
# Overview

This directory is the root of the aks-middleware module. It implements interceptors to aid common service tasks. See the list below for details.

# Usage

After the code is generated, run the following command in the root directory of this module.

```bash
make
```

# requestid

ServerInterceptor. It adds x-request-id to MD if there is no such entry. This interceptor needs to be registered first so that its request-id can be used by autologger and applogger.

# mdforward

ClientInterceptor. It propagates MD from incoming to outgoing. Only need to be used in servers. No need to be used in a pure client app.

# autologger

This package only provides the default logger implementation to log request result and latency. We use go-grpc-middleware's logging.UnaryServerIncerceptor() and logging.UnaryClientInterceptor() to achieve this. This package is not an interceptor.

# ctxlogger (applogger)

ServerInterceptor. It adds a logger to ctx with important information (e.g., request-id, method-name) already poplulated. App's handler code can get the logger and output additional information. Needs to be registered after the requestid interceptor.

Only information in MD could be propagated to dependencies. The logger added via ctx.WithValue() is local to the ctx and won't be propagated to dependencies.

# retry

This is a client interceptor that resends a request based on the gRPC code that is returned from the server

All options for the interceptor (i.e. max retries, codes to retry on, type of backoff) are defined in the retryOpts.go file 

# protovalidate

This is a server side interceptor that we use to validate the requests that are coming in from the client

The validation rules are generated and executed by the protovalidate-go library, and the rules are applied to the variables in the api.proto file

# recovery

This is a server side interceptor that is implemented to handle panics in the code

Once a panic is detected, it is handled by a custom recovery function defined in recoveryOpts.go which logs gRPC code "unknown" along with the file name/line number of where the panic occurred and a link to the repo. The program continues and does not terminate.

# log filtering

This is a feature that allows the user to annotate within api.proto what request payload variables should be logged or not. Logic in the ctxlogger.go decides what to log based off the annoatation. The "loggable" annotation is true by default, so the user is only required to annotate the variables that should not be logged.

Ancestors need to be loggable in order to examine the annotations on the leaf nodes. For example, if loggable value for "address" is false, it will not look for the loggable values of "city", "state", "zipcode", etc.

The source code for the external loggable field option is located at https://github.com/toma3233/loggable. Whenever it is updated, it is pushed to the following Buf Schema Registry as a module: https://buf.build/service-hub/loggable. Command used to authenticate before pushing to the BSR is "buf registry login" which prompts for BSR username and BSR token. The .netrc file is updated with these credentials. 

# Logging REST API Interactions

The `LogRequest` function introduced in `logging/logging.go` is a shared utility for logging REST API interactions. It is designed to be used in various parts of a service or module to ensure consistent logging practices.

## Function Overview

`LogRequest` takes a `LogRequestParams` struct as input, which includes the logger instance, start time of the request, the request itself, the response, and any error that occurred. It logs the request method, URL, service, response status, and latency. In case of an error, it logs the error message.

## Usage Example

```go
logging.LogRequest(logging.LogRequestParams{
    Logger:    loggerInstance,
    StartTime: time.Now(),
    Request:   httpRequest,
    Response:  httpResponse,
    Error:     err,
    URL:       requestURL
})
```

This function is utilized in `policy/policy.go` and `restlogger/restlogger.go` to log interactions with external REST APIs, enhancing the observability of the service.

## Integration Guidance

To integrate `LogRequest` into other services or modules, instantiate a `LogRequestParams` struct with the appropriate values and call `LogRequest`. This ensures that all REST API interactions are logged in a standardized format, facilitating debugging and monitoring.

# aks-middleware



# Project

> This repo has been populated by an initial template to help get you started. Please
> make sure to update the content to build a great experience for community-building.

As the maintainer of this project, please make a few updates:

- Improving this README.MD file to provide a great experience
- Updating SUPPORT.MD with content about this project's support experience
- Understanding the security reporting process in SECURITY.MD
- Remove this section from the README

## Contributing

This project welcomes contributions and suggestions.  Most contributions require you to agree to a
Contributor License Agreement (CLA) declaring that you have the right to, and actually do, grant us
the rights to use your contribution. For details, visit https://cla.opensource.microsoft.com.

When you submit a pull request, a CLA bot will automatically determine whether you need to provide
a CLA and decorate the PR appropriately (e.g., status check, comment). Simply follow the instructions
provided by the bot. You will only need to do this once across all repos using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or
contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.

## Trademarks

This project may contain trademarks or logos for projects, products, or services. Authorized use of Microsoft 
trademarks or logos is subject to and must follow 
[Microsoft's Trademark & Brand Guidelines](https://www.microsoft.com/en-us/legal/intellectualproperty/trademarks/usage/general).
Use of Microsoft trademarks or logos in modified versions of this project must not cause confusion or imply Microsoft sponsorship.
Any use of third-party trademarks or logos are subject to those third-party's policies.
