package restlogger

import (
	"net/http"
	"time"

	"github.com/Azure/aks-middleware/logging"
	"github.com/Azure/aks-middleware/unifiedlogger"
)

type LoggingRoundTripper struct {
	Proxied http.RoundTripper
	Logger  *unifiedlogger.LoggerWrapper
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

func NewLoggingClient(logger *unifiedlogger.LoggerWrapper) *http.Client {
	return &http.Client{
		Transport: &LoggingRoundTripper{
			Proxied: http.DefaultTransport,
			Logger:  logger,
		},
	}
}
