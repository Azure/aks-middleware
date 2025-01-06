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
})
```

This function is utilized in `policy/policy.go` and `restlogger/restlogger.go` to log interactions with external REST APIs, enhancing the observability of the service.

## Integration Guidance

To integrate `LogRequest` into other services or modules, instantiate a `LogRequestParams` struct with the appropriate values and call `LogRequest`. This ensures that all REST API interactions are logged in a standardized format, facilitating debugging and monitoring.