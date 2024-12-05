package logging

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Azure/aks-middleware/common"
	azcorePolicy "github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

var resourceTypes = map[string]bool{
	"resourcegroups":        true,
	"storageaccounts":       true,
	"operationresults":      true,
	"asyncoperations":       true,
	"checknameavailability": true,
}

type LogRequestParams struct {
	Logger    *slog.Logger
	StartTime time.Time
	Request   interface{}
	Response  *http.Response
	Error     error
}

// Shared logging function for REST API interactions
func getMethodInfo(method string, rawURL string) string {
	urlParts := strings.Split(rawURL, "?api-version")
	// malformed url
	// check for v1 to ensure we aren't classifying restlogger as malformed
	if len(urlParts) < 2 && !strings.Contains(urlParts[0], "v1") {
		return method + " " + rawURL
	}
	parts := strings.Split(urlParts[0], "/")
	resource := urlParts[0]
	counter := 0
	// Start from the end of the split path and move backward
	// to get nested resource type
	for counter = len(parts) - 1; counter >= 0; counter-- {
		currToken := strings.ToLower(parts[counter])
		if strings.ContainsAny(currToken, "?/") {
			index := strings.IndexAny(currToken, "?/")
			currToken = currToken[:index]
		}
		if resourceTypes[currToken] {
			resource = currToken
			break
		}
	}

	if method == "GET" {
		// resource name is specified, so it is a READ op
		if counter == len(parts)-1 {
			resource = resource + " - LIST"
		} else {
			resource = resource + " - READ"
		}
	}

	// REST VERB + Resource Type
	methodInfo := method + " " + resource

	return methodInfo
}

func trimURL(parsedURL url.URL) string {
	// Extract the `api-version` parameter
	apiVersion := parsedURL.Query().Get("api-version")

	// Reconstruct the URL with only the `api-version` parameter
	baseURL := parsedURL.Scheme + "://" + parsedURL.Host + parsedURL.Path
	if apiVersion != "" {
		return baseURL + "?api-version=" + apiVersion
	}
	return baseURL
}

func LogRequest(params LogRequestParams) {
	var method, service, reqURL string
	switch req := params.Request.(type) {
	case *http.Request:
		method = req.Method
		service = req.Host
		reqURL = req.URL.String()

	case *azcorePolicy.Request:
		method = req.Raw().Method
		service = req.Raw().Host
		reqURL = req.Raw().URL.String()
	default:
		return // Unknown request type, do nothing
	}

	parsedURL, parseErr := url.Parse(reqURL)
	if parseErr != nil {
		params.Logger.With(
			"source", "ApiRequestLog",
			"protocol", "REST",
			"method_type", "unary",
			"code", "na",
			"component", "client",
			"time_ms", "na",
			"method", method,
			"service", service,
			"url", reqURL,
			"error", parseErr.Error(),
		).Error("error parsing request URL")
	} else {
		reqURL = trimURL(*parsedURL)
	}

	methodInfo := getMethodInfo(method, reqURL)
	latency := time.Since(params.StartTime).Milliseconds()

	var headers map[string]string
	if params.Response != nil {
		headers = extractHeaders(params.Response.Header)
	}

	logEntry := params.Logger.With(
		"source", "ApiRequestLog",
		"protocol", "REST",
		"method_type", "unary",
		"component", "client",
		"time_ms", latency,
		"method", methodInfo,
		"service", service,
		"url", reqURL,
		"headers", headers,
	)

	if params.Error != nil || params.Response == nil {
		logEntry.With("error", params.Error.Error(), "code", "na").Error("finished call")
	} else if 200 <= params.Response.StatusCode && params.Response.StatusCode < 300 {
		logEntry.With("error", "na", "code", params.Response.StatusCode).Info("finished call")
	} else {
		logEntry.With("error", params.Response.Status, "code", params.Response.StatusCode).Error("finished call")
	}
}

func extractHeaders(header http.Header) map[string]string {
	headers := make(map[string]string)

	// List of headers to extract
	headerKeys := []string{
		common.RequestCorrelationIDHeader,
		common.RequestAcsOperationIDHeader,
		common.RequestARMClientRequestIDHeader,
	}

	// Convert header keys to lowercase
	lowerHeader := make(http.Header)
	for key, values := range header {
		lowerHeader[strings.ToLower(key)] = values
	}

	for _, key := range headerKeys {
		lowerKey := strings.ToLower(key)
		if values, ok := lowerHeader[lowerKey]; ok && len(values) > 0 {
			headers[key] = values[0]
		}
	}

	return headers
}
