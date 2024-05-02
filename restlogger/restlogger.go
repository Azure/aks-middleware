package restlogger

import (
	log "log/slog"
	"net/http"
	"strings"
	"time"
)

var resourceTypes = [4]string{"resourcegroups", "storageaccounts", "operationresults", "asyncoperations"}

type LoggingRoundTripper struct {
	Proxied http.RoundTripper
	Logger  *log.Logger
}

func (lrt *LoggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()

	resp, err := lrt.Proxied.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	latency := time.Since(start).Milliseconds()

	parts := strings.Split(req.URL.Path, "/")
	resource := ""
	foundResource := false
	// Start from the end of the split path and move backward
	counter := 0
	for counter = len(parts) - 1; counter >= 0; counter-- {
		resource = parts[counter]
		for _, rType := range resourceTypes {
			if strings.Compare(resource, rType) == 0 {
				// Found the appropriate resource type
				foundResource = true
				break
			}
		}
		if foundResource {
			break
		}
	}

	if req.Method == "GET" {
		// resource name is specified, so it is a READ op
		if counter != len(parts)-1 {
			resource = resource + " - READ"
		} else {
			resource = resource + " - LIST"
		}
	}
	lrt.Logger.With(
		"code", resp.StatusCode,
		"component", "client",
		"time_ms", latency,
		"method", req.Method+" "+resource,
		"service", req.Host,
		"source", "ApiAutoLog",
		"protocol", "REST",
		"method_type", "unary",
	).Info("finished call")

	return resp, err
}

func NewLoggingClient(logger *log.Logger) *http.Client {
	return &http.Client{
		Transport: &LoggingRoundTripper{
			Proxied: http.DefaultTransport,
			Logger:  logger,
		},
	}
}
