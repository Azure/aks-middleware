package restlogger

import (
	log "log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

type LoggingRoundTripper struct {
	Proxied http.RoundTripper
}

func (lrt *LoggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()

	resp, err := lrt.Proxied.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	latency := time.Since(start).Milliseconds()

	logger := log.New(log.NewJSONHandler(os.Stdout, nil))
	parts := strings.Split(req.URL.Path, "/")
	resource := parts[2]
	logger.With(
		"grpc.code", resp.StatusCode,
		"grpc.component", "client",
		"grpc.time_ms", latency,
		"grpc.method", req.Method+" "+resource,
		"grpc.service", req.Host,
		"source", "ApiAutoLog",
		"protocol", "REST",
		"grpc.method_type", "unary",
	).Info("finished call")

	return resp, err
}

func NewLoggingClient() *http.Client {
	return &http.Client{
		Transport: &LoggingRoundTripper{
			Proxied: http.DefaultTransport,
		},
	}
}
