package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Azure/aks-middleware/http/common"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	azcorePolicy "github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

type LogRequestParams struct {
	Logger    *slog.Logger
	StartTime time.Time
	Request   interface{}
	Response  *http.Response
	Error     error
}

func trimToSubscription(rawURL string) string {
	// Find the index of "/subscriptions"
	if idx := strings.Index(rawURL, "/subscriptions"); idx != -1 {
		return rawURL[idx:]
	}
	return rawURL
}

func sanitizeResourceType(rt string, rawURL string) string {
	// Keep only the substring after the last slash.
	if idx := strings.LastIndex(rt, "/"); idx != -1 && idx < len(rt)-1 {
		rt = rt[idx+1:]
	}
	// Remove everything after the first '?'.
	if idx := strings.Index(rt, "?"); idx != -1 {
		rt = rt[:idx]
	}
	rt = strings.ToLower(rt)

	return rt
}

func GetMethodInfo(method string, rawURL string) string {
	// Trim the URL to ensure it starts with "/subscriptions"
	validURL := trimToSubscription(rawURL)

	// First, try to parse validURL as a full resource ID.
	id, err := arm.ParseResourceID(validURL)
	if err != nil {
		// Retry by appending a false resource name ("dummy")
		// To be a valid resource ID, the URL must end with the resource name.
		fakeURL := validURL
		if !strings.HasSuffix(validURL, "/dummy") {
			fakeURL = validURL + "/dummy"
		}
		id, err = arm.ParseResourceID(fakeURL)
		if err != nil {
			// Fallback: if parsing still fails, use the full URL.
			return method + " " + rawURL
		}
		// We know a fake resource name was added.
		if method == "GET" {
			// For GET requests with a fake name, we assume it's a list operation.
			return method + " " + sanitizeResourceType(id.ResourceType.String(), rawURL) + " - LIST"
		}
		return method + " " + sanitizeResourceType(id.ResourceType.String(), rawURL)
	}

	// If parsing was successful on the first try.
	if method == "GET" {
		op := " - READ"
		if strings.TrimSpace(id.Name) == "" {
			op = " - LIST"
		}
		return method + " " + sanitizeResourceType(id.ResourceType.String(), rawURL) + op
	}
	return method + " " + sanitizeResourceType(id.ResourceType.String(), rawURL)
}

func TrimURL(parsedURL url.URL) string {
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
		reqURL = TrimURL(*parsedURL)
	}

	methodInfo := GetMethodInfo(method, reqURL)
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

// ResponseWriter is a custom response writer that buffers the response body and tracks the status code.
type ResponseWriter struct {
	http.ResponseWriter
	StatusCode int
	Buf        *bytes.Buffer
}

func NewResponseWriter(w http.ResponseWriter) *ResponseWriter {
	return &ResponseWriter{
		ResponseWriter: w,
		Buf:            new(bytes.Buffer),
	}
}

func (w *ResponseWriter) WriteHeader(code int) {
	w.StatusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *ResponseWriter) Write(b []byte) (int, error) {
	if w.StatusCode == 0 {
		w.StatusCode = http.StatusOK
	}
	if w.StatusCode >= http.StatusBadRequest {
		if w.Buf == nil {
			w.Buf = new(bytes.Buffer)
		}
		// Write only if status code indicates error
		w.Buf.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

func UnmarshalHeaders(log string) (map[string]interface{}, error) {
	var outer map[string]interface{}
	if err := json.Unmarshal([]byte(log), &outer); err != nil {
		return nil, fmt.Errorf("failed to unmarshal headers log output: %w", err)
	}
	headersStr, ok := outer["headers"]
	if !ok {
		return nil, fmt.Errorf("headers key not found or not a string in log output")
	}
	var inner map[string]interface{}
	err := json.Unmarshal([]byte(headersStr.(string)), &inner)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal headers log string: %w", err)
	}
	return inner, nil
}
