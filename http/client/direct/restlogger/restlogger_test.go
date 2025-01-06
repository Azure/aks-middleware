package restlogger_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"

	log "log/slog"

	"github.com/Azure/aks-middleware/http/client/direct/restlogger"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("LoggingRoundTripper", func() {
	var (
		fakeServer *httptest.Server
		logger     *log.Logger
		logBuffer  bytes.Buffer
		client     *http.Client
	)

	BeforeEach(func() {
		logger = log.New(log.NewTextHandler(&logBuffer, nil))
		client = &http.Client{
			Transport: &restlogger.LoggingRoundTripper{
				Proxied: http.DefaultTransport,
				Logger:  logger,
			},
		}
	})

	AfterEach(func() {
		fakeServer.Close()
		logBuffer.Reset()

	})

	Context("when making a successful request", func() {
		It("logs the request and response details", func() {
			fakeServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("mock response"))
			}))
			req, _ := http.NewRequest("GET", fakeServer.URL, nil)
			resp, err := client.Do(req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			logOutput := logBuffer.String()
			Expect(logOutput).To(ContainSubstring("finished call"))
		})
	})

	Context("when the server returns an error", func() {
		It("logs the error", func() {
			fakeServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("mock response"))
			}))
			req, _ := http.NewRequest("POST", fakeServer.URL, nil)
			resp, err := client.Do(req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

			logOutput := logBuffer.String()
			Expect(logOutput).To(ContainSubstring("finished call"))
			Expect(logOutput).To(ContainSubstring("500 Internal Server Error"))
		})
	})
})
