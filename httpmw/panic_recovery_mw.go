package httpmw

import (
	"net/http"
	"log/slog"
	"github.com/gorilla/mux"
)

type PanicHandlerFunc func(logger *slog.Logger, w http.ResponseWriter, r *http.Request, err interface{})

func NewPanicHandling(logger *slog.Logger, panicHandler PanicHandlerFunc) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return &panicHandlingMiddleware{
			next:         next,
			logger:       logger,
			panicHandler: panicHandler,
		}
	}
}

type panicHandlingMiddleware struct {
	next         http.Handler
	logger       *slog.Logger
	panicHandler PanicHandlerFunc
}

func (op *panicHandlingMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			op.panicHandler(op.logger, w, r, err)
		}
	}()
	op.next.ServeHTTP(w, r)
}

