package recovery

import (
	"log/slog"
	"net/http"

	"github.com/gorilla/mux"
)

type PanicHandlerFunc func(logger slog.Logger, w http.ResponseWriter, r *http.Request, err interface{})

func defaultPanicHandler(logger slog.Logger, w http.ResponseWriter, r *http.Request, err interface{}) {
	logger.Error("Panic occurred", "error", err)
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}

func NewPanicHandling(logger *slog.Logger, panicHandler PanicHandlerFunc) mux.MiddlewareFunc {
	if panicHandler == nil {
		panicHandler = defaultPanicHandler
	}
	return func(next http.Handler) http.Handler {
		return &panicHandlingMiddleware{
			next:         next,
			logger:       *logger,
			panicHandler: panicHandler,
		}
	}
}

type panicHandlingMiddleware struct {
	next         http.Handler
	logger       slog.Logger
	panicHandler PanicHandlerFunc
}

func (p *panicHandlingMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			p.panicHandler(p.logger, w, r, err)
		}
	}()
	p.next.ServeHTTP(w, r)
}
