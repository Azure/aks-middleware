package restlogger

import (
	log "log/slog"
	"net/http"
	"time"

	"github.com/Azure/aks-middleware/logging"
)

type LoggingRoundTripper struct {
	Proxied http.RoundTripper
	Logger  *log.Logger
}

func (lrt *LoggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := lrt.Proxied.RoundTrip(req)
	logging.LogRequest(logging.LogRequestParams{
		Logger:    lrt.Logger,
		StartTime: start,
		Request:   req,
		Response:  resp,
		Error:     err,
	})
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
