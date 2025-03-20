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
    createAuditClient := func() *audit.Client {
        clienConn := func() (conn.Audit, error) {
            return conn.NewNoOP(), nil
        }
        client, err := audit.New(clienConn)
        Expect(err).To(BeNil())
        return client
    }

    BeforeEach(func() {
        // Initialize common OtelConfig
        otelConfig = &OtelConfig{
            Client:                    createAuditClient(),
            CustomOperationDescs:      make(map[string]string),
            CustomOperationCategories: map[string]msgs.OperationCategory{},
            OperationAccessLevel:      "Test Contributor Role",
        }
        // Initialize common logger and router
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
        router.Use(NewLogging(slogLogger, otelConfig))
        // Common simple endpoint
        router.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
            time.Sleep(10 * time.Millisecond)
            w.WriteHeader(http.StatusOK)
        })
    })

    Describe("LoggingMiddleware with Real Audit Client", func() {
        It("should log and return OK status", func() {
            // No need to reinitialize logger or middleware here.
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
            Expect(logOutput).ToNot(ContainSubstring(`"armclientrequestid"`))
            Expect(w.Result().StatusCode).To(Equal(http.StatusOK))
        })

        It("should send audit event on request completion", func() {
            // Reuse the existing router & logger
            // Instead of reinitializing, simply set needed headers.
            w := httptest.NewRecorder()
            req := httptest.NewRequest("GET", "/", nil)
            req.Header.Set("User-Agent", "TestAgent")
            req.Header.Set("x-ms-client-app-id", "TestClientAppID")
            req.Header.Set("Region", "us-west")

            router.ServeHTTP(w, req)

            logOutput := buf.String()
            Expect(logOutput).To(ContainSubstring("sending audit logs"))
            Expect(logOutput).ToNot(ContainSubstring("failed to send audit event"))
        })

        It("should log error if audit client is nil", func() {
            nilConfig := &OtelConfig{
                Client:               nil,
                CustomOperationDescs: make(map[string]string),
                OperationAccessLevel: "Azure Kubernetes Fleet Manager Contributor Role",
            }
            // Use a separate router for this test to avoid altering the global one.
            localRouter := mux.NewRouter()
            localRouter.Use(NewLogging(slogLogger, nilConfig))
            localRouter.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
                w.WriteHeader(http.StatusInternalServerError)
            })

            w := httptest.NewRecorder()
            req := httptest.NewRequest("GET", "/", nil)

            localRouter.ServeHTTP(w, req)

            Expect(buf.String()).To(ContainSubstring("otel configuration or client is nil"))
        })

        It("should log an error when method mapped to OCOther and no description is provided", func() {
            updatedOtelConfig := &OtelConfig{
                Client:  createAuditClient(),
                CustomOperationDescs: make(map[string]string),
                CustomOperationCategories: map[string]msgs.OperationCategory{
                    "GET": msgs.OCOther,
                },
                OperationAccessLevel: "Test Contributor Role",
            }
            // Use a separate router for this test.
            localRouter := mux.NewRouter()
            localRouter.Use(NewLogging(slogLogger, updatedOtelConfig))
            localRouter.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
                w.WriteHeader(http.StatusInternalServerError)
            })

            w := httptest.NewRecorder()
            req := httptest.NewRequest("GET", "/", nil)
            localRouter.ServeHTTP(w, req)

            logOutput := buf.String()
            Expect(logOutput).To(ContainSubstring("failed to send audit event"))
            Expect(logOutput).To(ContainSubstring("operation category description is required for category OCOther"))
        })

        It("should log an error when the record object is invalid", func() {
            // Do not set any headers so that caller identities are missing.
            w := httptest.NewRecorder()
            req := httptest.NewRequest("GET", "/", nil)

            router.ServeHTTP(w, req)

            Expect(buf.String()).To(ContainSubstring("failed to send audit event"))
        })

        It("should handle validation failure when caller identities are missing", func() {
            w := httptest.NewRecorder()
            req := httptest.NewRequest("GET", "/", nil)
            req.Header.Set("Region", "us-west")
            req.Header.Set("User-Agent", "TestAgent")
            // Intentionally omit identity headers

            router.ServeHTTP(w, req)

            logOutput := buf.String()
            Expect(logOutput).To(ContainSubstring("failed to send audit event"))
            Expect(logOutput).To(ContainSubstring("at least one caller identity is required"))
        })

        It("should handle invalid remote address format", func() {
            w := httptest.NewRecorder()
            req := httptest.NewRequest("GET", "/", nil)
            req.RemoteAddr = "invalid:address:format"
            req.Header.Set("Region", "us-west")
            req.Header.Set("x-ms-client-app-id", "TestClientAppID")

            router.ServeHTTP(w, req)

            Expect(buf.String()).To(ContainSubstring("failed to split host and port"))
        })

        It("should extract buffered response body as error message", func() {
            localRouter := mux.NewRouter()
            localRouter.Use(NewLogging(slogLogger, otelConfig))
            // Setup an error endpoint.
            localRouter.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
                http.Error(w, "simulated error occurred", http.StatusBadRequest)
            })

            w := httptest.NewRecorder()
            req := httptest.NewRequest("GET", "/error", nil)
            req.Header.Set("User-Agent", "TestAgent")
            req.Header.Set("x-ms-client-app-id", "TestClientAppID")
            req.Header.Set("Region", "us-west")

            localRouter.ServeHTTP(w, req)

            logOutput := buf.String()
            Expect(logOutput).To(ContainSubstring("simulated error occurred"))
            Expect(logOutput).To(ContainSubstring("sending audit logs"))
        })
    })
})