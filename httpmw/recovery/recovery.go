package recovery

import (
	"net/http"

	"github.com/Azure/aks-middleware/httpmw/logging"
	"github.com/gorilla/mux"
)

// comment for testing

type PanicHandlerFunc func(logger logging.Logger, w http.ResponseWriter, r *http.Request, err interface{})

func defaultPanicHandler(logger logging.Logger, w http.ResponseWriter, r *http.Request, err interface{}) {
	logger.Error("Panic occurred", "error", err)
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}

func NewPanicHandling(logger logging.Logger, panicHandler PanicHandlerFunc) mux.MiddlewareFunc {
	if panicHandler == nil {
		panicHandler = defaultPanicHandler
	}
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
	logger       logging.Logger
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
