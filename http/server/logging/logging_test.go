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
		router *mux.Router
		//routerWithExtraLogging *mux.Router
		buf        *bytes.Buffer
		slogLogger *slog.Logger
		// rgnameKey              string
		// subIdKey               string
		// resultType             string
		// errorDetailsKey        string
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
		router.Use(NewLogging(slogLogger, CustomAttributes{}))

		router.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		})

		// routerWithExtraLogging = mux.NewRouter()
		// routerWithExtraLogging.Use(requestid.NewRequestIDMiddlewareWithExtractor(customExtractor))
		// subIdKey := "subscriptionID"
		// rgnameKey := "resourceGroupName"
		// resultTypeKey := "resultType"
		// errorDetailsKey := "errorDetails"

		// var testInitializer initFunc = func(w http.ResponseWriter, r *http.Request) map[string]interface{} {
		// 	fmt.Println("in test initializer!!")
		// 	attrMap := map[string]interface{}{
		// 		subIdKey:        subIdKey + "value",
		// 		rgnameKey:       rgnameKey + "value",
		// 		resultTypeKey:   resultTypeKey + "value",
		// 		errorDetailsKey: errorDetailsKey + "value",
		// 	}
		// 	return attrMap
		// }

		// var testAssigner loggingFunc = func(w http.ResponseWriter, r *http.Request, attrMap map[string]interface{}) map[string]interface{} {
		// 	// could overwrite some values based on request data
		// 	// ex: get operation request from r
		// 	fmt.Println("in test assigner!!")
		// 	opreq := operationRequestFromContext(r.Context())
		// 	attrMap[rgnameKey] = opreq.ResourceName
		// 	attrMap[subIdKey] = opreq.SubscriptionID
		// 	attrMap[resultTypeKey] = 2
		// 	// don't set the error details if there was no error, shouldn't be an issue
		// 	fmt.Println("returning from test assigner!!")
		// 	return attrMap
		// }

		// customAttributes := CustomAttributes{
		// 	AttributeInitializer: &testInitializer,
		// 	AttributeAssigner:    &testAssigner,
		// }

		// routerWithExtraLogging.Use(NewLogging(slogLogger, customAttributes))

		// routerWithExtraLogging.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		// 	time.Sleep(10 * time.Millisecond)
		// 	w.WriteHeader(http.StatusOK)
		// })
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

		// It("should set values for extra attributes included for logging", func() {
		// 	w := httptest.NewRecorder()
		// 	req := httptest.NewRequest("GET", "/", nil)
		// 	req.Header.Set(requestid.RequestAcsOperationIDHeader, "test-operation-id")
		// 	req.Header.Set(requestid.RequestCorrelationIDHeader, "test-correlation-id")
		// 	ctx := context.WithValue(context.Background(), rgnameKey, "test-rgname-value")
		// 	ctx = context.WithValue(ctx, subIdKey, "test-subid-value")
		// 	req.WithContext(ctx)

		// 	routerWithExtraLogging.ServeHTTP(w, req)

		// 	Expect(buf.String()).To(ContainSubstring(`"operationid":"test-operation-id"`))
		// 	Expect(buf.String()).To(ContainSubstring(`"correlationid":"test-correlation-id"`))
		// 	Expect(buf.String()).ToNot(ContainSubstring(`"armclientrequestid"`))

		// 	// test extra values
		// 	Expect(buf.String()).To(ContainSubstring(`"%s:%s"`, rgnameKey, "test-rgname-value"))
		// 	Expect(buf.String()).To(ContainSubstring(`"%s:%s"`, subIdKey, "test-subid-value"))
		// 	Expect(w.Result().StatusCode).To(Equal(http.StatusOK))
		// })
	})
})

type OperationRequest struct {
	ResourceName   string
	SubscriptionID string
}

type contextKey struct{}

func operationRequestFromContext(ctx context.Context) *OperationRequest {
	r, ok := ctx.Value(contextKey{}).(*OperationRequest)
	if !ok || r == nil {
		return nil
	}
	// always return a copy to avoid other middleware change it's value
	return r.copy()
}

func (op *OperationRequest) copy() *OperationRequest {
	r := *op
	return &r
}
