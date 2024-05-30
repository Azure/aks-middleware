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
}

// Shared logging function for REST API interactions
func GetMethodInfo(method string, rawURL string) string {
	url := strings.Split(rawURL, "?api-version")
	parts := strings.Split(url[0], "/")
	resource := url[0]
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
	var method, rawURL, service string
	switch req := params.Request.(type) {
	case *http.Request:
		method = req.Method
		rawURL = req.URL.String()
		service = req.Host
	case *azcorePolicy.Request:
		method = req.Raw().Method
		rawURL = req.Raw().URL.String()
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

	if params.Error != nil {
		logEntry.With(
			"code", "na",
			"component", "client",
			"time_ms", "na",
			"method", methodInfo,
			"service", service,
			"url", rawURL,
			"error", params.Error.Error(),
		).Error("error finishing call")
		return
	}

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
	} else {
		logEntry.With("error", params.Response.Status).Error("finished call")
	}
}
