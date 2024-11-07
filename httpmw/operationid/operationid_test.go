package operationid

import (
	"net/http"
	"net/http/httptest"

	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("OperationID Middleware", func() {
	var (
		router   *mux.Router
		recorder *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		router = mux.NewRouter()
		router.Use(Middleware())
		router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			operationID, _ := ctx.Value(OperationIDKey).(string)
			correlationID, _ := ctx.Value(CorrelationIDKey).(string)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(operationID + "," + correlationID))
		})
		recorder = httptest.NewRecorder()
	})

	It("should extract operationID and correlationID from headers", func() {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("x-ms-acs-operation-id", "test-operation-id")
		req.Header.Set("x-ms-correlation-request-id", "test-correlation-id")

		router.ServeHTTP(recorder, req)

		Expect(recorder.Code).To(Equal(http.StatusOK))
		Expect(recorder.Body.String()).To(Equal("test-operation-id,test-correlation-id"))
	})

	It("should handle missing headers gracefully", func() {
		req := httptest.NewRequest("GET", "/", nil)

		router.ServeHTTP(recorder, req)

		Expect(recorder.Code).To(Equal(http.StatusOK))
		Expect(recorder.Body.String()).To(Equal(","))
	})
})
