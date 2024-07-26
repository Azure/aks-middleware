package httpmw

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Httpmw", func() {
	var (
		router *mux.Router
	)

	BeforeEach(func() {
		router = mux.NewRouter()
	})

	Describe("PanicMiddleware", func() {
		It("should handle panic and return Internal Server Error", func() {
			logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
			router.Use(NewPanicHandling(logger, customPanicHandler))
			router.HandleFunc("/", func(w http.ResponseWriter, e *http.Request) {
				panic("oops")
			})

			rw := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)

			router.ServeHTTP(rw, req)

			Expect(rw.Body.String()).To(ContainSubstring("Internal Server Error"))
			Expect(rw.Result().StatusCode).To(Equal(500))
		})
	})

	Describe("LoggingMiddleware", func() {
		It("should log and return OK status", func() {
			buf := new(bytes.Buffer)
			slogLogger := slog.New(slog.NewJSONHandler(buf, nil))
			router.Use(NewLogging(slogLogger))

			router.HandleFunc("/", func(w http.ResponseWriter, e *http.Request) {
				time.Sleep(10 * time.Millisecond)
				w.WriteHeader(http.StatusOK)
			})

			rw := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)

			router.ServeHTTP(rw, req)

			Expect(buf.String()).To(ContainSubstring("finished call"))
			Expect(buf.String()).To(ContainSubstring(`"source":"ApiRequestLog"`))
			Expect(buf.String()).To(ContainSubstring(`"protocol":"HTTP"`))
			Expect(buf.String()).To(ContainSubstring(`"method_type":"unary"`))
			Expect(buf.String()).To(ContainSubstring(`"component":"client"`))
			Expect(buf.String()).To(ContainSubstring(`"time_ms":`))
			Expect(buf.String()).To(ContainSubstring(`"service":"`))
			Expect(buf.String()).To(ContainSubstring(`"url":"`))
			Expect(rw.Result().StatusCode).To(Equal(200))

		})
	})
})

// Custom panic handler for testing
func customPanicHandler(logger Logger, w http.ResponseWriter, r *http.Request, err interface{}) {
	logger.Info(fmt.Sprintf("Custom panic occurred: %v", err))
	// Additional custom handling logic here
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}
