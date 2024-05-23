package httpmw

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Azure/aks-middleware/ctxlogger"
	"github.com/gorilla/mux"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
)

func TestPanicMiddleware(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	router := mux.NewRouter()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	router.Use(NewPanicHandling(logger, customPanicHandler))
	router.HandleFunc("/", func(w http.ResponseWriter, e *http.Request) {
		panic("oops")
	})

	rw := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	router.ServeHTTP(rw, req)

	g.Expect(rw.Body.String()).To(ContainSubstring("Internal Server Error"))
	g.Expect(rw.Result().StatusCode).To(Equal(500))
}

// Custom panic handler for testing
func customPanicHandler(logger *slog.Logger, w http.ResponseWriter, r *http.Request, err interface{}) {
	logger.Info(fmt.Sprintf("Custom panic occurred: %v", err))
	// Additional custom handling logic here
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}

func TestLoggingMiddleware(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	router := mux.NewRouter()
	router.Use(NewLogging())

	router.HandleFunc("/", func(w http.ResponseWriter, e *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req.WithContext(ctxlogger.WithLogger(req.Context(), slog.New(slog.NewJSONHandler(os.Stdout, nil)))))
	resp := w.Result()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	g.Expect(w.Body.String()).To(ContainSubstring("Finished call"))

	defer resp.Body.Close()
}
