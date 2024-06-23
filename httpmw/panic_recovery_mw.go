package httpmw

import (
	"net/http"

	"github.com/gorilla/mux"
)

type PanicHandlerFunc func(logger Logger, w http.ResponseWriter, r *http.Request, err interface{})

func NewPanicHandling(logger Logger, panicHandler PanicHandlerFunc) mux.MiddlewareFunc {
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
	logger       Logger
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
