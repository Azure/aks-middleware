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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Httpmw", func() {
	var (
		router                 *mux.Router
		routerWithExtraLogging *mux.Router
		buf                    *bytes.Buffer
		buf2                   *bytes.Buffer
		slogLogger             *slog.Logger
		rgnameKey              string
		subIdKey               string
		resultTypeKey          string
		errorDetailsKey        string
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
		router.Use(NewLogging(slogLogger, "", CustomAttributes{}))

		router.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		})

		routerWithExtraLogging = mux.NewRouter()
		routerWithExtraLogging.Use(requestid.NewRequestIDMiddlewareWithExtractor(customExtractor))
		subIdKey = "subscriptionID"
		rgnameKey = "resourceGroupName"
		resultTypeKey = "resultType"
		errorDetailsKey = "errorDetails"

		var testInitializer initFunc = func(w http.ResponseWriter, r *http.Request) map[string]interface{} {
			attrMap := map[string]interface{}{
				subIdKey:        subIdKey + "value",
				rgnameKey:       rgnameKey + "value",
				resultTypeKey:   resultTypeKey + "value",
				errorDetailsKey: errorDetailsKey + "value",
			}
			return attrMap
		}

		var testAssigner loggingFunc = func(w http.ResponseWriter, r *http.Request, attrMap map[string]interface{}) map[string]interface{} {
			opreq := operationRequestFromContext(r.Context())
			// Overwrite the extra attributes. These assignments update the map directly.
			attrMap[rgnameKey] = opreq.ResourceName
			attrMap[subIdKey] = opreq.SubscriptionID

			attrMap[resultTypeKey] = 2
			return attrMap
		}
		customAttributes := CustomAttributes{
			AttributeInitializer: testInitializer,
			AttributeAssigner:    testAssigner,
		}

		buf2 = new(bytes.Buffer)
		slogLogger2 := slog.New(slog.NewJSONHandler(buf2, nil))
		routerWithExtraLogging.Use(NewLogging(slogLogger2, "customSource", customAttributes))

		routerWithExtraLogging.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
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

			// Update the request with the prepared context.
			updatedReq := req.WithContext(ctx)
			routerWithExtraLogging.ServeHTTP(w, updatedReq)

			Expect(buf2.String()).To(ContainSubstring(`"operationid":"test-operation-id"`))
			Expect(buf2.String()).To(ContainSubstring(`"correlationid":"test-correlation-id"`))
			Expect(buf2.String()).ToNot(ContainSubstring(`"armclientrequestid"`))

			Expect(buf2.String()).To(ContainSubstring("finished call"))
			Expect(buf2.String()).To(ContainSubstring(`"source":"customSource"`)) // source should equal custom value
			Expect(buf2.String()).To(ContainSubstring(`"protocol":"HTTP"`))
			Expect(buf2.String()).To(ContainSubstring(`"method_type":"unary"`))
			Expect(buf2.String()).To(ContainSubstring(`"component":"server"`))
			Expect(buf2.String()).To(ContainSubstring(`"time_ms":`))
			Expect(buf2.String()).To(ContainSubstring(`"service":"`))
			Expect(buf2.String()).To(ContainSubstring(`"url":"`))

			// check extra attributes
			Expect(buf2.String()).To(ContainSubstring(`"%s":"%s"`, rgnameKey, "test-rgname-value"))
			Expect(buf2.String()).To(ContainSubstring(`"%s":"%s"`, subIdKey, "test-subid-value"))
			Expect(buf2.String()).To(ContainSubstring(`"%s":"%s"`, errorDetailsKey, errorDetailsKey+"value"))
			Expect(buf2.String()).To(ContainSubstring(`"%s":%d`, resultTypeKey, 2))
			Expect(w.Result().StatusCode).To(Equal(http.StatusOK))
		})
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
