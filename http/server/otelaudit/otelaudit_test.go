package otelaudit

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"

	"github.com/Azure/aks-middleware/http/common"
	"github.com/gorilla/mux"
	"github.com/microsoft/go-otel-audit/audit"
	"github.com/microsoft/go-otel-audit/audit/conn"
	"github.com/microsoft/go-otel-audit/audit/msgs"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("createOtelAuditEvent", func() {
	var (
		router     *mux.Router
		otelConf   *OtelConfig
		buf        *bytes.Buffer
		logger     *slog.Logger
		auditEvent msgs.Msg
		auditErr   error
	)

	BeforeEach(func() {
		buf = new(bytes.Buffer)
		logger = slog.New(slog.NewJSONHandler(buf, nil))

		otelConf = &OtelConfig{
			CustomOperationDescs: map[string]string{
				"GET": "CustomDesc for GET",
			},
			CustomOperationCategories: map[string]msgs.OperationCategory{
				"GET": msgs.OCOther,
			},
			OperationAccessLevel: "Test Access Level",
		}

		router = mux.NewRouter()
		routePattern := "/{" + common.SubscriptionIDKey + "}/resourceGroups/{resourceGroup}/providers/{resourceProvider}/{resourceType}/{resourceName}/default"
		router.HandleFunc(routePattern, func(w http.ResponseWriter, r *http.Request) {
			auditEvent, auditErr = createOtelAuditEvent(logger, http.StatusOK, r, otelConf, "")
			w.WriteHeader(http.StatusOK)
		}).Methods("GET")
	})

	It("should extract URL variables and create a valid audit event message", func() {
		reqURL := "http://example.com/sub-123/resourceGroups/rg-test/providers/Microsoft.Test/resourceType/testResource/default"
		req := httptest.NewRequest("GET", reqURL, nil)
		req.RemoteAddr = "127.0.0.1:8080"
		req.Header.Set("User-Agent", "TestAgent")
		req.Header.Set("x-ms-client-app-id", "TestClientAppID")
		req.Header.Set("Region", "us-west")

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		Expect(auditErr).To(BeNil())

		Expect(auditEvent.Record.CallerIdentities).To(HaveKey(msgs.SubscriptionID))
		subs := auditEvent.Record.CallerIdentities[msgs.SubscriptionID]
		Expect(len(subs)).To(Equal(1))
		Expect(subs[0].Identity).To(Equal("sub-123"))

		Expect(auditEvent.Record.OperationAccessLevel).To(Equal("Test Access Level"))
		Expect(auditEvent.Record.OperationName).To(Equal("GET"))
		Expect(auditEvent.Record.OperationCategoryDescription).To(Equal("CustomDesc for GET"))
		Expect(auditEvent.Record.OperationType).ToNot(BeNil())
		Expect(auditEvent.Record.CallerAgent).To(Equal("TestAgent"))
	})
})

var _ = Describe("Otel Audit Integration Test", func() {
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
			ExcludeAuditEvents:        map[string][]string{},
		}
		// Initialize common logger and router
		buf = new(bytes.Buffer)
		slogLogger = slog.New(slog.NewJSONHandler(buf, nil))
		router = mux.NewRouter()

		router.Use(NewOtelAuditLogging(slogLogger, otelConfig))
		// Common simple endpoint
		router.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	})

	Describe("OtelAuditLogging Middleware with Real Audit Client", func() {
		It("should send audit event on request completion", func() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("User-Agent", "TestAgent")
			req.Header.Set("x-ms-client-app-id", "TestClientAppID")
			req.Header.Set("Region", "us-west")

			router.ServeHTTP(w, req)

			logOutput := buf.String()
			Expect(logOutput).ToNot(ContainSubstring("failed to send audit event"))
		})

		It("should log error if audit client is nil", func() {
			nilConfig := &OtelConfig{
				Client:               nil,
				CustomOperationDescs: make(map[string]string),
				OperationAccessLevel: "Azure Kubernetes Fleet Manager Contributor Role",
			}
			localRouter := mux.NewRouter()
			localRouter.Use(NewOtelAuditLogging(slogLogger, nilConfig))
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
				Client:               createAuditClient(),
				CustomOperationDescs: make(map[string]string),
				CustomOperationCategories: map[string]msgs.OperationCategory{
					"GET": msgs.OCOther,
				},
				OperationAccessLevel: "Test Contributor Role",
			}
			localRouter := mux.NewRouter()
			localRouter.Use(NewOtelAuditLogging(slogLogger, updatedOtelConfig))
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

		It("should not send audit event if request matches exclusion criteria", func() {
			localOtelConfig := &OtelConfig{
				Client:                    createAuditClient(),
				CustomOperationDescs:      make(map[string]string),
				CustomOperationCategories: map[string]msgs.OperationCategory{},
				OperationAccessLevel:      "Test Contributor Role",
				ExcludeAuditEvents: map[string][]string{
					"GET": {"/exclude"},
				},
			}
			localRouter := mux.NewRouter()
			localRouter.Use(NewOtelAuditLogging(slogLogger, localOtelConfig))
			localRouter.HandleFunc("/exclude", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/exclude", nil)
			localRouter.ServeHTTP(w, req)

			Expect(w.Result().StatusCode).To(Equal(http.StatusOK))
		})
	})
})
