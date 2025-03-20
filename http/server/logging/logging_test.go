package logging

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/Azure/aks-middleware/http/server/requestid"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Configuration for test routers
type routerConfig struct {
	buf     *bytes.Buffer
	logger  *slog.Logger
	source  string
	attrMgr AttributeManager
}

const (
	subIdKey        = "subscriptionID"
	rgnameKey       = "resourceGroupName"
	resultTypeKey   = "resultType"
	errorDetailsKey = "errorDetails"
)

var _ = Describe("Httpmw", func() {
	var (
		customExtractor = func(r *http.Request) map[string]string {
			return map[string]string{
				string(requestid.CorrelationIDKey): r.Header.Get(requestid.RequestCorrelationIDHeader),
				string(requestid.OperationIDKey):   r.Header.Get(requestid.RequestAcsOperationIDHeader),
			}
		}
	)

	routerCfg := map[string]*routerConfig{
		"default": {
			source:  "ApiRequestLog",
			attrMgr: AttributeManager{},
		},
		"without-initializer": {
			source: "ApiRequestLog",
			// only assigner provided
			attrMgr: AttributeManager{
				AttributeAssigner: func(w http.ResponseWriter, r *http.Request, attrs map[string]interface{}) map[string]interface{} {
					return map[string]interface{}{"hello": "world"}
				},
			},
		},
		"without-assigner": {
			source: "ApiRequestLog",
			// only initializer provided
			attrMgr: AttributeManager{
				AttributeInitializer: func(w http.ResponseWriter, r *http.Request) map[string]interface{} {
					return map[string]interface{}{"hello": "world"}
				},
			},
		},
		"extra-logging": {
			source: "customSource",
			attrMgr: AttributeManager{
				AttributeInitializer: func(w http.ResponseWriter, r *http.Request) map[string]interface{} {
					return map[string]interface{}{
						"subscriptionID":    "defaultSubIDvalue",
						"resourceGroupName": "defaultRGnamevalue",
						"resultType":        "defaultResultTypevalue",
						"errorDetails":      "defaultErrorDetailsvalue",
					}
				},
				AttributeAssigner: func(w http.ResponseWriter, r *http.Request, attrMap map[string]interface{}) map[string]interface{} {
					opReq := operationRequestFromContext(r.Context())
					if opReq != nil {
						attrMap["resourceGroupName"] = opReq.ResourceName
						attrMap["subscriptionID"] = opReq.SubscriptionID
					}
					attrMap["resultType"] = 2
					return attrMap
				},
			},
		},
	}

	buildRouter := func(cfg *routerConfig) *mux.Router {
		r := mux.NewRouter()
		r.Use(requestid.NewRequestIDMiddlewareWithExtractor(customExtractor))

		cfg.buf = new(bytes.Buffer)
		cfg.logger = slog.New(slog.NewJSONHandler(cfg.buf, nil))

		r.Use(NewLogging(cfg.logger, cfg.source, cfg.attrMgr))
		r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		})
		return r
	}

	routersMap := map[string]*mux.Router{}

	BeforeEach(func() {
		for name, cfg := range routerCfg {
			routersMap[name] = buildRouter(cfg)
		}
	})

	Describe("LoggingMiddleware", func() {
		It("should log and return OK status", func() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			routersMap["default"].ServeHTTP(w, req)

			cfg := routerCfg["default"]
			buf := cfg.buf
			Expect(cfg.buf).To(ContainSubstring("finished call"))
			Expect(buf).To(ContainSubstring(`"source":"ApiRequestLog"`))
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

			routersMap["default"].ServeHTTP(w, req)

			cfg := routerCfg["default"]
			buf := cfg.buf
			Expect(buf.String()).To(ContainSubstring(`"operationid":"test-operation-id"`))
			Expect(buf.String()).To(ContainSubstring(`"correlationid":"test-correlation-id"`))
			Expect(buf.String()).ToNot(ContainSubstring(`"armclientrequestid"`))
			Expect(w.Result().StatusCode).To(Equal(http.StatusOK))
		})

		It("should set values for extra attributes included for logging", func() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set(requestid.RequestAcsOperationIDHeader, "test-operation-id")
			req.Header.Set(requestid.RequestCorrelationIDHeader, "test-correlation-id")

			type ctxkey string
			rgkey := ctxkey(rgnameKey)
			subkey := ctxkey(subIdKey)

			ctx := context.Background()
			ctx = context.WithValue(ctx, rgkey, "test-rgname-value")
			ctx = context.WithValue(ctx, subkey, "test-subid-value")
			opReq := &OperationRequest{
				ResourceName:   "test-rgname-value",
				SubscriptionID: "test-subid-value",
			}
			ctx = context.WithValue(ctx, operationRequestKey, opReq)

			updatedReq := req.WithContext(ctx)
			routersMap["extra-logging"].ServeHTTP(w, updatedReq)
			cfg := routerCfg["extra-logging"]
			buf2 := cfg.buf
			Expect(buf2.String()).To(ContainSubstring(`"operationid":"test-operation-id"`))
			Expect(buf2.String()).To(ContainSubstring(`"correlationid":"test-correlation-id"`))
			Expect(buf2.String()).ToNot(ContainSubstring(`"armclientrequestid"`))

			checkDefaultAttributes(*buf2, cfg.source, w)

			// check extra attributes
			Expect(buf2.String()).To(ContainSubstring(`"%s":"%s"`, rgnameKey, "test-rgname-value"))
			Expect(buf2.String()).To(ContainSubstring(`"%s":"%s"`, subIdKey, "test-subid-value"))
			Expect(buf2.String()).To(ContainSubstring(`"%s":"%s"`, errorDetailsKey, "defaultErrorDetailsvalue"))
			Expect(buf2.String()).To(ContainSubstring(`"%s":%d`, resultTypeKey, 2))
			Expect(w.Result().StatusCode).To(Equal(http.StatusOK))
		})

		It("If either AttributeManager initializer or assigner is nil, default attributes should be set", func() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)

			routerWithoutInitializer := routersMap["without-initializer"]
			routerWithoutInitializer.ServeHTTP(w, req)
			cfg := routerCfg["without-initializer"]
			buf3 := cfg.buf

			checkDefaultAttributes(*buf3, cfg.source, w)
			Expect(buf3.String()).To(ContainSubstring(`"hello":"world"`)) // assigner was set by user without initializer, but assigner should not be overwritten

			w2 := httptest.NewRecorder()
			req2 := httptest.NewRequest("GET", "/", nil)
			routerWithoutAssigner := routersMap["without-assigner"]
			routerWithoutAssigner.ServeHTTP(w2, req2)
			cfg4 := routerCfg["without-assigner"]
			buf4 := cfg4.buf

			checkDefaultAttributes(*buf4, cfg.source, w)
			Expect(buf4.String()).To(ContainSubstring(`"hello":"world"`)) // initializer was set by user without assigner, but initializer should not be overwritten

			routerWithoutAssigner.ServeHTTP(w, req)
		})
	})
})

var _ = Describe("Test Helpers", func() {
	It("test setDefaultInitializerAndAssigner()", func() {
		attrMgr := &AttributeManager{}
		source := to.Ptr("")
		setInitializerAndAssignerIfNil(attrMgr, source)
		Expect(attrMgr.AttributeAssigner).ToNot(BeNil())
		Expect(attrMgr.AttributeInitializer).ToNot(BeNil())
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		initMap := attrMgr.AttributeInitializer(w, req)
		Expect(initMap).To(BeEmpty())
		finalMap := attrMgr.AttributeAssigner(w, req, initMap)
		Expect(finalMap).To(BeEmpty())
		Expect(*source).To(Equal("ApiRequestLog"))
	})

	It("Test flattenAttributes()", func() {
		attrMap := map[string]interface{}{
			"hello":   "world",
			"latency": 2,
		}

		attrList := flattenAttributes(attrMap)
		Expect(len(attrList)).To(Equal(4))

		for i, val := range attrList {
			if val == "hello" {
				Expect(attrList[i+1]).To(Equal("world"))
			}
			if val == "latency" {
				Expect(attrList[i+1]).To(Equal(2))
			}
		}
	})
})

// Used only for testing
type OperationRequest struct {
	ResourceName   string
	SubscriptionID string
}

type contextKey struct{}

var operationRequestKey = contextKey{}

func operationRequestFromContext(ctx context.Context) *OperationRequest {
	r, ok := ctx.Value(operationRequestKey).(*OperationRequest)
	if !ok || r == nil {
		return nil
	}
	return r.copy()
}

func (op *OperationRequest) copy() *OperationRequest {
	r := *op
	return &r
}

func checkDefaultAttributes(buf bytes.Buffer, source string, w *httptest.ResponseRecorder) {
	Expect(buf.String()).To(ContainSubstring("finished call"))
	Expect(buf.String()).To(ContainSubstring(`"source":"%s"`, source))
	Expect(buf.String()).To(ContainSubstring(`"protocol":"HTTP"`))
	Expect(buf.String()).To(ContainSubstring(`"method_type":"unary"`))
	Expect(buf.String()).To(ContainSubstring(`"component":"server"`))
	Expect(buf.String()).To(ContainSubstring(`"time_ms":`))
	Expect(buf.String()).To(ContainSubstring(`"service":"`))
	Expect(buf.String()).To(ContainSubstring(`"url":"`))
	Expect(w.Result().StatusCode).To(Equal(http.StatusOK))
}
