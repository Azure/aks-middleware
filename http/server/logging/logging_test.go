package logging

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/Azure/aks-middleware/http/server/requestid"
	"github.com/gorilla/mux"
	"github.com/microsoft/go-otel-audit/audit"
	"github.com/microsoft/go-otel-audit/audit/conn"
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
		cc := func() (conn.Audit, error) {
			return conn.NewNoOP(), nil
		}
		client, err := audit.New(cc)
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
			Client:               createAuditClient(),
			CustomOperationDescs: make(map[string]string),
			OperationAccessLevel: "Test Contributor Role",
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

			mw := &loggingMiddleware{
				next:       router,
				logger:     slogLogger,
				otelConfig: otelConfig,
			}

			msgCtx := context.TODO()
			mw.sendOtelAuditEvent(msgCtx, w.Result().StatusCode, req)

			Expect(buf.String()).To(ContainSubstring("sending audit logs"))
			Expect(buf.String()).To(ContainSubstring("audit event sent successfully"))
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

			mw := &loggingMiddleware{
				next:       router,
				logger:     slogLogger,
				otelConfig: nilConfig,
			}

			msgCtx := context.TODO()
			mw.sendOtelAuditEvent(msgCtx, w.Result().StatusCode, req)

			Expect(buf.String()).To(ContainSubstring("otel configuration or client is nil"))
		})

		It("should log an error when the record object is invalid", func() {
			buf := new(bytes.Buffer)
			slogLogger := slog.New(slog.NewJSONHandler(buf, nil))
			router.Use(NewLogging(slogLogger, otelConfig))

			router.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)

			router.ServeHTTP(w, req)

			mw := &loggingMiddleware{
				next:       router,
				logger:     slogLogger,
				otelConfig: otelConfig,
			}

			msgCtx := context.TODO()
			mw.sendOtelAuditEvent(msgCtx, w.Result().StatusCode, req)

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
		
			mw := &loggingMiddleware{
				next:       router,
				logger:     slogLogger,
				otelConfig: otelConfig,
			}
		
			msgCtx := context.TODO()
			mw.sendOtelAuditEvent(msgCtx, w.Result().StatusCode, req)
		
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

            mw := &loggingMiddleware{
                next:       router,
                logger:    slogLogger,
                otelConfig: otelConfig,
            }

            msgCtx := context.TODO()
            mw.sendOtelAuditEvent(msgCtx, w.Result().StatusCode, req)

            Expect(buf.String()).To(ContainSubstring("failed to split host and port"))
        })
	})
})
