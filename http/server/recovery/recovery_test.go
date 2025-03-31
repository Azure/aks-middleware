package recovery

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("http recovery middleware", func() {
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
			router.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
				panic("oops")
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)

			router.ServeHTTP(w, req)

			Expect(w.Body.String()).To(ContainSubstring("Bad Request"))
			Expect(w.Result().StatusCode).To(Equal(400))
		})

		It("should use default handler if custom handler is not passed", func() {
			logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
			router.Use(NewPanicHandling(logger, nil))
			router.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
				panic("oops")
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)

			router.ServeHTTP(w, req)

			Expect(w.Body.String()).To(ContainSubstring("Internal Server Error"))
			Expect(w.Result().StatusCode).To(Equal(500))
		})
	})

	Describe("Test SavePanicInfoToCtx", func() {
		It("should save filepath and panic message to context", func() {
			panicMsg := "Test panic"
			testStackTrace := `panic: Test panic

			goroutine 1 [running]:
			runtime.gopanic(0x123456)
			/usr/local/go/src/runtime/panic.go:840 +0x254
			main.main()
			/home/user/project/main.go:15 +0x34
			runtime.main()
			/usr/local/go/src/runtime/proc.go:250 +0x212
			`
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req = SavePanicInfoToCtx(req, panicMsg, testStackTrace)

			gotFilePath, _ := req.Context().Value(FilePathKey).(string)
			gotPanicMsg, _ := req.Context().Value(PanicMessageKey).(string)

			expectedFilePath := "/home/user/project/main.go:15"
			expectedPanicMsg := panicMsg

			Expect(gotFilePath).To(Equal(expectedFilePath))
			Expect(gotPanicMsg).To(Equal(expectedPanicMsg))
		})
	})
})

// Custom panic handler for testing
func customPanicHandler(logger slog.Logger, w http.ResponseWriter, r *http.Request, err interface{}) {
	logger.Info(fmt.Sprintf("Custom panic occurred: %v", err))
	// Additional custom handling logic here
	http.Error(w, "Bad Request", http.StatusBadRequest)
}
