package customlogging

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/Azure/aks-middleware/http/server/requestid"
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
	subscriptionIDKey    = "subscriptionID"
	resourceGroupNameKey = "resourceGroupName"
	resultTypeKey        = "resultType"
	errorDetailsKey      = "errorDetails"

	defaultRouterName               = "default"
	onlyInitializerRouterName       = "only-initializer-set"
	onlyAssignerRouterName          = "only-assigner-set"
	extraLoggingVariablesRouterName = "extra-logging-variables-provided"

	customTestKey   = "testKey"
	customTestValue = "testValue"
)

var _ = Describe("HttpmwWithCustomAttributeLogging", Ordered, func() {
	var (
		customExtractor = func(r *http.Request) map[string]string {
			return map[string]string{
				string(requestid.CorrelationIDKey): r.Header.Get(requestid.RequestCorrelationIDHeader),
				string(requestid.OperationIDKey):   r.Header.Get(requestid.RequestAcsOperationIDHeader),
			}
		}
	)

	testRoutersConfiguationMap := map[string]*routerConfig{
		defaultRouterName: {
			source:  apiRequestLogSource,
			attrMgr: AttributeManager{},
		},
		onlyAssignerRouterName: {
			source: apiRequestLogSource,
			// only assigner provided
			attrMgr: AttributeManager{
				AttributeAssigner: func(w *ResponseRecord, r *http.Request, attrs map[string]interface{}) map[string]interface{} {
					return map[string]interface{}{customTestKey: customTestValue}
				},
			},
		},
		onlyInitializerRouterName: {
			source: apiRequestLogSource,
			// only initializer provided
			attrMgr: AttributeManager{
				AttributeInitializer: func(w *ResponseRecord, r *http.Request) map[string]interface{} {
					return map[string]interface{}{customTestKey: customTestValue}
				},
			},
		},
		extraLoggingVariablesRouterName: {
			source: "customSource",
			attrMgr: AttributeManager{
				AttributeInitializer: func(w *ResponseRecord, r *http.Request) map[string]interface{} {
					return map[string]interface{}{
						"subscriptionID":    "defaultSubIDvalue",
						"resourceGroupName": "defaultRGnamevalue",
						"resultType":        "defaultResultTypevalue",
						"errorDetails":      "defaultErrorDetailsvalue",
					}
				},
				AttributeAssigner: func(w *ResponseRecord, r *http.Request, attrMap map[string]interface{}) map[string]interface{} {
					opReq := operationRequestFromContext(r.Context())
					if opReq != nil {
						attrMap["resourceGroupName"] = opReq.ResourceGroupName
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

	BeforeAll(func() {
		for name, cfg := range testRoutersConfiguationMap {
			routersMap[name] = buildRouter(cfg)
		}
	})

	It("should log and return OK status", func() {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		routersMap[defaultRouterName].ServeHTTP(w, req)

		cfg := testRoutersConfiguationMap[defaultRouterName]
		buf := cfg.buf
		Expect(buf).To(ContainSubstring("finished call"))
		Expect(buf).To(ContainSubstring(`"source":"ApiRequestLog"`))
		Expect(w.Result().StatusCode).To(Equal(http.StatusOK))
	})

	It("should log operationID and correlationID from headers", func() {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set(requestid.RequestAcsOperationIDHeader, "test-operation-id")
		req.Header.Set(requestid.RequestCorrelationIDHeader, "test-correlation-id")

		routersMap[defaultRouterName].ServeHTTP(w, req)

		cfg := testRoutersConfiguationMap[defaultRouterName]
		buf := cfg.buf
		Expect(buf.String()).To(ContainSubstring(`"operationid":"test-operation-id"`))
		Expect(buf.String()).To(ContainSubstring(`"correlationid":"test-correlation-id"`))
		Expect(buf.String()).ToNot(ContainSubstring(`"armclientrequestid"`))
		Expect(w.Result().StatusCode).To(Equal(http.StatusOK))
	})

	It("If either AttributeManager initializer or assigner is nil, default attributes should be set", func() {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)

		routerWithoutInitializer := routersMap[onlyAssignerRouterName]
		routerWithoutInitializer.ServeHTTP(w, req)
		cfg := testRoutersConfiguationMap[onlyAssignerRouterName]
		buf3 := cfg.buf

		// assigner was set by user without initializer, but assigner should not be overwritten
		Expect(buf3.String()).To(ContainSubstring(`"%s":"%s"`, customTestKey, customTestValue))

		w2 := httptest.NewRecorder()
		req2 := httptest.NewRequest("GET", "/", nil)
		routerWithoutAssigner := routersMap[onlyInitializerRouterName]
		routerWithoutAssigner.ServeHTTP(w2, req2)
		cfg4 := testRoutersConfiguationMap[onlyInitializerRouterName]
		buf4 := cfg4.buf

		// initializer was set by user without assigner, but initializer should not be overwritten
		Expect(buf4.String()).To(ContainSubstring(`"%s":"%s"`, customTestKey, customTestValue))

		routerWithoutAssigner.ServeHTTP(w, req)
	})

	// Tests the primary difference between customAttributeLoggingMiddleware and loggingMiddleware
	// resourceGroupName, subscriptionID errorDetails, and resultType should be set in addition to pre-set headers
	It("should set values for extra attributes included for logging", func() {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set(requestid.RequestAcsOperationIDHeader, "test-operation-id")
		req.Header.Set(requestid.RequestCorrelationIDHeader, "test-correlation-id")

		opReq := &OperationRequest{
			ResourceGroupName: "test-rgname-value",
			SubscriptionID:    "test-subid-value",
		}

		ctx := context.WithValue(context.Background(), operationRequestKey, opReq)

		req = req.WithContext(ctx)
		routersMap[extraLoggingVariablesRouterName].ServeHTTP(w, req)
		cfg := testRoutersConfiguationMap[extraLoggingVariablesRouterName]
		buf2 := cfg.buf
		Expect(buf2.String()).To(ContainSubstring(`"operationid":"test-operation-id"`))
		Expect(buf2.String()).To(ContainSubstring(`"correlationid":"test-correlation-id"`))
		Expect(buf2.String()).ToNot(ContainSubstring(`"armclientrequestid"`))

		// check extra attributes
		Expect(buf2.String()).To(ContainSubstring(`"%s":"%s"`, resourceGroupNameKey, "test-rgname-value"))
		Expect(buf2.String()).To(ContainSubstring(`"%s":"%s"`, subscriptionIDKey, "test-subid-value"))
		Expect(buf2.String()).To(ContainSubstring(`"%s":"%s"`, errorDetailsKey, "defaultErrorDetailsvalue"))
		Expect(buf2.String()).To(ContainSubstring(`"%s":%d`, resultTypeKey, 2))
		Expect(w.Result().StatusCode).To(Equal(http.StatusOK))
	})
})

var _ = Describe("Test Helpers", func() {
	It("Test setInitializerAndAssignerIfNil()", func() {
		attrMgr := &AttributeManager{}
		setInitializerAndAssignerIfNil(attrMgr)
		Expect(attrMgr.AttributeAssigner).ToNot(BeNil())
		Expect(attrMgr.AttributeInitializer).ToNot(BeNil())
		w := &ResponseRecord{}
		req := httptest.NewRequest("GET", "/", nil)
		initMap := attrMgr.AttributeInitializer(w, req)
		Expect(initMap).To(BeEmpty())
		finalMap := attrMgr.AttributeAssigner(w, req, initMap)
		Expect(finalMap).To(BeEmpty())
	})

	It("Test flattenAttributes()", func() {
		attrMap := map[string]interface{}{
			customTestKey: customTestValue,
			"latency":     2,
		}

		attrList := flattenAttributes(attrMap)
		Expect(len(attrList)).To(Equal(4))

		for i, val := range attrList {
			if val == customTestKey {
				Expect(attrList[i+1]).To(Equal(customTestValue))
			}
			if val == "latency" {
				Expect(attrList[i+1]).To(Equal(2))
			}
		}
	})
})

// Used only for testing
type OperationRequest struct {
	ResourceGroupName string
	SubscriptionID    string
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
