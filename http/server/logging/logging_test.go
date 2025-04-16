package logging

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/Azure/aks-middleware/http/server/requestid"
	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Httpmw Integration Test", func() {
	var (
		router     *mux.Router
		buf        *bytes.Buffer
		slogLogger *slog.Logger
	)

	BeforeEach(func() {
		buf = new(bytes.Buffer)
		slogLogger = slog.New(slog.NewJSONHandler(buf, nil))
		router = mux.NewRouter()

		customExtractor := func(r *http.Request) map[string]string {
			return map[string]string{
				string(requestid.CorrelationIDKey): r.Header.Get(requestid.RequestCorrelationIDHeader),
				string(requestid.OperationIDKey):   r.Header.Get(requestid.RequestAcsOperationIDHeader),
			}
		}

		router.Use(requestid.NewRequestIDMiddlewareWithExtractor(customExtractor))
		router.Use(NewLogging(slogLogger))
		// Common simple endpoint
		router.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		})
	})

	Describe("LoggingMiddleware", func() {
		It("should log and return OK status", func() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)

			router.ServeHTTP(w, req)

			logOutput := buf.String()
			Expect(logOutput).To(ContainSubstring("finished call"))
			Expect(logOutput).To(ContainSubstring(`"source":"ApiRequestLog"`))
			Expect(logOutput).To(ContainSubstring(`"protocol":"HTTP"`))
			Expect(logOutput).To(ContainSubstring(`"method_type":"unary"`))
			Expect(logOutput).To(ContainSubstring(`"component":"server"`))
			Expect(logOutput).To(ContainSubstring(`"time_ms":`))
			Expect(logOutput).To(ContainSubstring(`"service":"`))
			Expect(logOutput).To(ContainSubstring(`"url":"`))
			Expect(w.Result().StatusCode).To(Equal(http.StatusOK))
		})

		It("should log operationID and correlationID from headers", func() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set(requestid.RequestAcsOperationIDHeader, "test-operation-id")
			req.Header.Set(requestid.RequestCorrelationIDHeader, "test-correlation-id")

			router.ServeHTTP(w, req)

			logOutput := buf.String()
			Expect(logOutput).To(ContainSubstring(`"operationid":"test-operation-id"`))
			Expect(logOutput).To(ContainSubstring(`"correlationid":"test-correlation-id"`))
			Expect(w.Result().StatusCode).To(Equal(http.StatusOK))
		})
	})
})
