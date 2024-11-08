package logging

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/Azure/aks-middleware/httpmw/operationid"
	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Httpmw", func() {
	var (
		router     *mux.Router
		buf        *bytes.Buffer
		slogLogger *slog.Logger
	)

	const (
		RequestAcsOperationIDHeader = "x-ms-acs-operation-id"
	)

	BeforeEach(func() {

		buf = new(bytes.Buffer)
		slogLogger = slog.New(slog.NewJSONHandler(buf, nil))

		router = mux.NewRouter()

		customExtractor := func(r *http.Request) map[string]string {
			return map[string]string{
				string(operationid.CorrelationIDKey): r.Header.Get(operationid.RequestCorrelationIDHeader),
				"acsoperationID":                     r.Header.Get(RequestAcsOperationIDHeader),
			}
		}
		router.Use(operationid.NewOperationIDMiddlewareWithExtractor(customExtractor))
		router.Use(NewLogging(slogLogger))

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

			Expect(buf.String()).To(ContainSubstring("finished call"))
			Expect(buf.String()).To(ContainSubstring(`"source":"ApiRequestLog"`))
			Expect(buf.String()).To(ContainSubstring(`"protocol":"HTTP"`))
			Expect(buf.String()).To(ContainSubstring(`"method_type":"unary"`))
			Expect(buf.String()).To(ContainSubstring(`"component":"client"`))
			Expect(buf.String()).To(ContainSubstring(`"time_ms":`))
			Expect(buf.String()).To(ContainSubstring(`"service":"`))
			Expect(buf.String()).To(ContainSubstring(`"url":"`))
			Expect(w.Result().StatusCode).To(Equal(http.StatusOK))
		})

		It("should log operationID and correlationID from headers", func() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set(RequestAcsOperationIDHeader, "test-operation-id")
			req.Header.Set(operationid.RequestCorrelationIDHeader, "test-correlation-id")

			router.ServeHTTP(w, req)

			Expect(buf.String()).To(ContainSubstring(`"acsoperationid":"test-operation-id"`))
			Expect(buf.String()).To(ContainSubstring(`"correlationid":"test-correlation-id"`))
			Expect(w.Result().StatusCode).To(Equal(http.StatusOK))
		})
	})
})
