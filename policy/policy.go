package policy

import (
	log "log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/Azure/aks-middleware/logging"
	armPolicy "github.com/Azure/azure-sdk-for-go/sdk/azcore/arm/policy"
	azcorePolicy "github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"google.golang.org/grpc/codes"
)

type LoggingPolicy struct {
	logger log.Logger
}

func NewLoggingPolicy(logger log.Logger) *LoggingPolicy {
	return &LoggingPolicy{logger: logger}
}

func (p *LoggingPolicy) Do(req *azcorePolicy.Request) (*http.Response, error) {
	startTime := time.Now()
	resp, err := req.Next()

	parsedURL, parseErr := url.Parse(req.Raw().URL.String())
	if parseErr != nil {
		p.logger.With(
			"code", "na",
			"component", "client",
			"time_ms", "na",
			"method", "na",
			"service", "na",
			"url", req.Raw().URL.String(),
			"error", parseErr.Error(),
		).Error("error parsing request URL")
	} else {
		// Example URLs: "https://management.azure.com/subscriptions/{sub_id}/resourcegroups?api-version={version}"
		// https://management.azure.com/subscriptions/{sub_id}/resourceGroups/{rg_name}/providers/Microsoft.Storage/storageAccounts/{sa_name}?api-version={version}
		trimmedURL := trimURL(*parsedURL)
		req.Raw().URL.Path = trimmedURL
	}

	logging.LogRequest(logging.LogRequestParams{
		Logger:    &p.logger,
		StartTime: startTime,
		Request:   req.Raw(),
		Response:  resp,
		Error:     err,
	})
	return resp, err
}

func (p *LoggingPolicy) Clone() azcorePolicy.Policy {
	return &LoggingPolicy{logger: p.logger}
}

func GetDefaultArmClientOptions(logger *log.Logger) *armPolicy.ClientOptions {
	logOptions := new(azcorePolicy.LogOptions)

	retryOptions := new(azcorePolicy.RetryOptions)
	retryOptions.MaxRetries = 5

	clientOptions := new(azcorePolicy.ClientOptions)
	clientOptions.Logging = *logOptions
	clientOptions.Retry = *retryOptions

	armClientOptions := new(armPolicy.ClientOptions)
	armClientOptions.ClientOptions = *clientOptions

	policyLogger := logger.With(
		"source", "ApiAutoLog",
		"method_type", "unary",
		"protocol", "REST",
	)
	loggingPolicy := NewLoggingPolicy(*policyLogger)

	armClientOptions.PerCallPolicies = append(armClientOptions.PerCallPolicies, loggingPolicy)

	return armClientOptions
}

func trimURL(parsedURL url.URL) string {

	query := parsedURL.Query()
	apiVersion := query.Get("api-version")

	// Remove all other query parameters
	for key := range query {
		if key != "api-version" {
			query.Del(key)
		}
	}

	// Set the api-version query parameter
	query.Set("api-version", apiVersion)

	// Encode the query parameters and set them in the parsed URL
	parsedURL.RawQuery = query.Encode()

	return parsedURL.String()
}

// Based off of gRPC standard here: https://chromium.googlesource.com/external/github.com/grpc/grpc/+/refs/tags/v1.21.4-pre1/doc/statuscodes.md
func ConvertHTTPStatusToGRPCError(httpStatusCode int) codes.Code {
	var code codes.Code

	switch httpStatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted:
		code = codes.OK
	case http.StatusBadRequest:
		code = codes.InvalidArgument
	case http.StatusGatewayTimeout:
		code = codes.DeadlineExceeded
	case http.StatusUnauthorized:
		code = codes.Unauthenticated
	case http.StatusForbidden:
		code = codes.PermissionDenied
	case http.StatusNotFound:
		code = codes.NotFound
	case http.StatusConflict:
		code = codes.Aborted
	case http.StatusTooManyRequests:
		code = codes.ResourceExhausted
	case http.StatusInternalServerError:
		code = codes.Internal
	case http.StatusNotImplemented:
		code = codes.Unimplemented
	case http.StatusServiceUnavailable:
		code = codes.Unavailable
	default:
		code = codes.Unknown
	}

	return code
}
