package policy

import (
	"log/slog"
	log "log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	armPolicy "github.com/Azure/azure-sdk-for-go/sdk/azcore/arm/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	azcorePolicy "github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
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

	method := req.Raw().Method
	parsedURL, parseErr := url.Parse(req.Raw().URL.String())
	if parseErr != nil {
		p.logger.With(
			"component", "client",
			"method", "ERROR",
			"service", "ERROR",
			"url", req.Raw().URL.String(),
			"error", parseErr.Error(),
		).Error("Error parsing request URL: ", parseErr)
		return nil, parseErr
	}

	trimmedURL := trimURL(*parsedURL)
	// ex url: "https://management.azure.com/subscriptions/{sub_id}/resourcegroups?api-version={version}"
	splitPath := strings.Split(trimmedURL, "/")
	var resourceType string
	if len(splitPath) > 5 {
		resourceType = splitPath[5]
		if strings.ContainsAny(resourceType, "?/") {
			index := strings.IndexAny(resourceType, "?/")
			resourceType = resourceType[:index]
		}
	} else {
		resourceType = trimmedURL
	}

	// REST VERB + Resource Type
	methodInfo := method + " " + resourceType
	
	if err != nil {
		p.logger.With(
			"component", "client",
			"method", methodInfo,
			"service", parsedURL.Host,
			"url", trimmedURL,
			"error", err.Error(),
		).Error("Error finishing request: ", err)
		return nil, err
	}

	// Time is in ms
	latency := time.Since(startTime).Milliseconds()

	logEntry := p.logger.With(
		"code", resp.StatusCode,
		"component", "client",
		"time_ms", latency,
		"method", methodInfo,
		"service", parsedURL.Host,
		"url", trimmedURL,
	)
	
	// separate check here b/c previous error check only checks if there was an error in the req
	if 200 <= resp.StatusCode && resp.StatusCode < 300 {
		logEntry.Info("finished call")
	} else {
		logEntry.With("error", resp.Status).Error("finished call")
	}

	return resp, err
}

func (p *LoggingPolicy) Clone() azcorePolicy.Policy {
	return &LoggingPolicy{logger: p.logger}
}

func GetDefaultArmClientOptions() *armPolicy.ClientOptions {
	logOptions := new(policy.LogOptions)

	retryOptions := new(policy.RetryOptions)
	retryOptions.MaxRetries = 5

	clientOptions := new(policy.ClientOptions)
	clientOptions.Logging = *logOptions
	clientOptions.Retry = *retryOptions

	armClientOptions := new(armPolicy.ClientOptions)
	armClientOptions.ClientOptions = *clientOptions

	logger := log.New(slog.NewJSONHandler(os.Stdout, nil))
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
