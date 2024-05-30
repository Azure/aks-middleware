package restlogger_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Azure/aks-middleware/logging"
	"github.com/Azure/aks-middleware/restlogger"
	log "log/slog"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRestlogger(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Restlogger Suite")
}

var _ = Describe("LoggingRoundTripper", func() {
	var (
		mockServer *httptest.Server
		logger     *log.Logger
		logBuffer  bytes.Buffer
		client     *http.Client
	)

	BeforeEach(func() {
		logger = log.New(log.NewTextHandler(&logBuffer, nil))
		mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("mock response"))
		}))
		client = &http.Client{
			Transport: &restlogger.LoggingRoundTripper{
				Proxied: http.DefaultTransport,
				Logger:  logger,
			},
		}
	})

	AfterEach(func() {
		mockServer.Close()
	})

	Context("when making a successful request", func() {
		It("logs the request and response details", func() {
			req, _ := http.NewRequest("GET", mockServer.URL, nil)
			resp, err := client.Do(req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			logOutput := logBuffer.String()
			Expect(logOutput).To(ContainSubstring("finished call"))
			Expect(logOutput).To(ContainSubstring("GET"))
			Expect(logOutput).To(ContainSubstring("200"))
			Expect(logOutput).To(ContainSubstring(mockServer.URL))
		})
	})

	Context("when the server returns an error", func() {
		BeforeEach(func() {
			mockServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "mock error", http.StatusInternalServerError)
			})
		})

		It("logs the error", func() {
			req, _ := http.NewRequest("POST", mockServer.URL, nil)
			resp, err := client.Do(req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

			logOutput := logBuffer.String()
			Expect(logOutput).To(ContainSubstring("error finishing call"))
			Expect(logOutput).To(ContainSubstring("POST"))
			Expect(logOutput).To(ContainSubstring("500"))
			Expect(logOutput).To(ContainSubstring(mockServer.URL))
		})
	})
})
