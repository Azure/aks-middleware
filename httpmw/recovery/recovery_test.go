package recovery

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/Azure/aks-middleware/httpmw/logging"
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

		It("should use default handler if custom handler is not passed", func() {
			logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
			router.Use(NewPanicHandling(logger, nil))
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
})

// Custom panic handler for testing
func customPanicHandler(logger logging.Logger, w http.ResponseWriter, r *http.Request, err interface{}) {
	logger.Info(fmt.Sprintf("Custom panic occurred: %v", err))
	// Additional custom handling logic here
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}
