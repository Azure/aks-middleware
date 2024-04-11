package restlogger

import (
	log "log/slog"
	"net/http"
	"strings"
	"time"
)

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
	resource := parts[2]
	if req.Method == "GET" {
		// resource name is specified, so it is a READ op
		if len(parts) >= 4 {
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
