package logging

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/Azure/aks-middleware/http/server/requestid"
	"github.com/gorilla/mux"
	"github.com/microsoft/go-otel-audit/audit"
	"github.com/microsoft/go-otel-audit/audit/conn"
	"github.com/microsoft/go-otel-audit/audit/msgs"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Httpmw Integration Test", func() {
	var (
		router     *mux.Router
		otelConfig *OtelConfig
		buf        *bytes.Buffer
		slogLogger *slog.Logger
	)

	// Helper function to create a real audit client
	// https://github.com/microsoft/go-otel-audit/blob/baa5ee96eb7d46a8004b207d722d750dfa9d163b/audit/internal/scenarios/scenarios_test.go#L50
	createAuditClient := func() *audit.Client {
		// function creates a new connection to socket
		clienConn := func() (conn.Audit, error) {
			return conn.NewNoOP(), nil
		}
		// creates client that utilizes prev function to connect
		client, err := audit.New(clienConn)
		Expect(err).To(BeNil())
		return client
	}

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

		// Initialize OtelConfig with default values
		otelConfig = &OtelConfig{
			Client:                    createAuditClient(),
			CustomOperationDescs:      make(map[string]string),
			CustomOperationCategories: map[string]msgs.OperationCategory{},
			OperationAccessLevel:      "Test Contributor Role",
		}

		router.Use(requestid.NewRequestIDMiddlewareWithExtractor(customExtractor))
		router.Use(NewLogging(slogLogger, otelConfig))

		router.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		})
	})

	Describe("LoggingMiddleware with Real Audit Client", func() {
		It("should log and return OK status", func() {
			buf := new(bytes.Buffer)
			slogLogger := slog.New(slog.NewJSONHandler(buf, nil))
			router.Use(NewLogging(slogLogger, otelConfig))

			router.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
				time.Sleep(10 * time.Millisecond)
				w.WriteHeader(http.StatusOK)
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)

			router.ServeHTTP(w, req)

			Expect(buf.String()).To(ContainSubstring("finished call"))
			Expect(buf.String()).To(ContainSubstring(`"source":"ApiRequestLog"`))
			Expect(buf.String()).To(ContainSubstring(`"protocol":"HTTP"`))
			Expect(buf.String()).To(ContainSubstring(`"method_type":"unary"`))
			Expect(buf.String()).To(ContainSubstring(`"component":"server"`))
			Expect(buf.String()).To(ContainSubstring(`"time_ms":`))
			Expect(buf.String()).To(ContainSubstring(`"service":"`))
			Expect(buf.String()).To(ContainSubstring(`"url":"`))
			Expect(w.Result().StatusCode).To(Equal(http.StatusOK))
		})

		It("should log operationID and correlationID from headers", func() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set(requestid.RequestAcsOperationIDHeader, "test-operation-id")
			req.Header.Set(requestid.RequestCorrelationIDHeader, "test-correlation-id")

			router.ServeHTTP(w, req)

			Expect(buf.String()).To(ContainSubstring(`"operationid":"test-operation-id"`))
			Expect(buf.String()).To(ContainSubstring(`"correlationid":"test-correlation-id"`))
			Expect(buf.String()).ToNot(ContainSubstring(`"armclientrequestid"`))
			Expect(w.Result().StatusCode).To(Equal(http.StatusOK))
		})

		It("should send audit event on request completion", func() {
			buf := new(bytes.Buffer)
			slogLogger := slog.New(slog.NewJSONHandler(buf, nil))
			router.Use(NewLogging(slogLogger, otelConfig))

			router.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("User-Agent", "TestAgent")
			req.Header.Set("x-ms-client-app-id", "TestClientAppID")
			req.Header.Set("Region", "us-west")

			router.ServeHTTP(w, req)

			Expect(buf.String()).To(ContainSubstring("sending audit logs"))
			Expect(buf.String()).ToNot(ContainSubstring("failed to send audit event"))
		})

		It("should log error if audit client is nil", func() {
			nilConfig := &OtelConfig{
				Client:               nil,
				CustomOperationDescs: make(map[string]string),
				OperationAccessLevel: "Azure Kubernetes Fleet Manager Contributor Role",
			}

			buf := new(bytes.Buffer)
			slogLogger := slog.New(slog.NewJSONHandler(buf, nil))
			router.Use(NewLogging(slogLogger, nilConfig))

			router.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)

			router.ServeHTTP(w, req)

			Expect(buf.String()).To(ContainSubstring("otel configuration or client is nil"))
		})

		It("should log an error when method mapped to OCOther and no description is provided", func() {
			updatedOtelConfig := &OtelConfig{
				Client:               createAuditClient(),
				CustomOperationDescs: make(map[string]string),
				// custom mapping method to operation category
				CustomOperationCategories: map[string]msgs.OperationCategory{
					"GET": msgs.OCOther,
				},
				OperationAccessLevel: "Test Contributor Role",
			}

			buf := new(bytes.Buffer)
			slogLogger := slog.New(slog.NewJSONHandler(buf, nil))
			router.Use(NewLogging(slogLogger, updatedOtelConfig))

			router.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)

			router.ServeHTTP(w, req)

			Expect(buf.String()).To(ContainSubstring("failed to send audit event"))
			// This error means the custom operation category mapping has worked
			// but the description is missing
			Expect(buf.String()).To(ContainSubstring("operation category description is required for category OCOther"))

		})

		It("should log an error when the record object is invalid", func() {
			buf := new(bytes.Buffer)
			slogLogger := slog.New(slog.NewJSONHandler(buf, nil))
			router.Use(NewLogging(slogLogger, otelConfig))

			router.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			w := httptest.NewRecorder()
			// We don't set any headers here to simulate an invalid record object
			req := httptest.NewRequest("GET", "/", nil)

			router.ServeHTTP(w, req)

			Expect(buf.String()).To(ContainSubstring("failed to send audit event"))
		})

		It("should handle validation failure when caller identities are missing", func() {
			buf := new(bytes.Buffer)
			slogLogger := slog.New(slog.NewJSONHandler(buf, nil))
			router.Use(NewLogging(slogLogger, otelConfig))

			router.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("Region", "us-west")
			req.Header.Set("User-Agent", "TestAgent")
			// Intentionally omit identity headers

			router.ServeHTTP(w, req)

			Expect(buf.String()).To(ContainSubstring("failed to send audit event"))
			Expect(buf.String()).To(ContainSubstring("at least one caller identity is required"))
		})

		It("should handle invalid remote address format", func() {
			buf := new(bytes.Buffer)
			slogLogger := slog.New(slog.NewJSONHandler(buf, nil))
			router.Use(NewLogging(slogLogger, otelConfig))

			router.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = "invalid:address:format"
			req.Header.Set("Region", "us-west")
			req.Header.Set("x-ms-client-app-id", "TestClientAppID")

			router.ServeHTTP(w, req)

			Expect(buf.String()).To(ContainSubstring("failed to split host and port"))
		})
	})

	Describe("ServeHTTP testing", func() {
		It("should extract buffered response body as error message", func() {
			localRouter := mux.NewRouter()
			localRouter.Use(NewLogging(slogLogger, otelConfig))

			// Setup an endpoint that always returns an error.
			localRouter.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "simulated error occurred", http.StatusBadRequest)
			})

			// Simulate a client request to the error endpoint.
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/error", nil)
			req.Header.Set("User-Agent", "TestAgent")
			req.Header.Set("x-ms-client-app-id", "TestClientAppID")
			req.Header.Set("Region", "us-west")
			localRouter.ServeHTTP(w, req)

			// The middleware's ServeHTTP extracts the buffered error response.
			loggedOutput := buf.String()
			Expect(loggedOutput).To(ContainSubstring("simulated error occurred"))
			Expect(loggedOutput).To(ContainSubstring("sending audit logs"))
		})
	})
})
