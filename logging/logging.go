package logging

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	azcorePolicy "github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

var resourceTypes = map[string]bool{
	"resourcegroups":   true,
	"storageaccounts":  true,
	"operationresults": true,
	"asyncoperations":  true,
}

type LogRequestParams struct {
	Logger    *slog.Logger
	StartTime time.Time
	Request   interface{}
	Response  *http.Response
	Error     error
	URL       string
}

// Shared logging function for REST API interactions
func GetMethodInfo(method string, rawURL string) string {
	urlParts := strings.Split(rawURL, "?api-version")
	// malformed url
	if len(urlParts) < 2 {
		return method + " " + rawURL
	}
	parts := strings.Split(urlParts[0], "/")
	resource := urlParts[0]
	counter := 0
	// Start from the end of the split path and move backward
	// to get nested resource type
	for counter = len(parts) - 1; counter >= 0; counter-- {
		currToken := parts[counter]
		if resourceTypes[strings.ToLower(currToken)] {
			resource = currToken
			break
		}
	}

	if method == "GET" {
		// resource name is specified, so it is a READ op
		if counter != len(parts)-1 {
			resource = resource + " - READ"
		} else {
			resource = resource + " - LIST"
		}
	}

	// REST VERB + Resource Type
	methodInfo := method + " " + resource

	return methodInfo
}

func LogRequest(params LogRequestParams) {
	var method, service string
	rawURL := params.URL
	switch req := params.Request.(type) {
	case *http.Request:
		method = req.Method
		service = req.Host
	case *azcorePolicy.Request:
		method = req.Raw().Method
		service = req.Raw().Host
	default:
		return // Unknown request type, do nothing
	}

	methodInfo := GetMethodInfo(method, rawURL)
	logEntry := params.Logger.With(
		"source", "ApiAutoLog",
		"protocol", "REST",
		"method_type", "unary",
	)

	latency := time.Since(params.StartTime).Milliseconds()
	logEntry = logEntry.With(
		"code", params.Response.StatusCode,
		"component", "client",
		"time_ms", latency,
		"method", methodInfo,
		"service", service,
		"url", rawURL,
	)

	if 200 <= params.Response.StatusCode && params.Response.StatusCode < 300 {
		logEntry.With("error", "na").Info("finished call")
	} else if params.Error != nil {
		logEntry.With("error", params.Error.Error()).Error("finished call")
	} else {
		logEntry.With("error", params.Response.Status).Error("finished call")
	}
}
