package recovery

import (
	"log/slog"
	"net/http"

	"github.com/Azure/aks-middleware/http/server/logging"
	"github.com/gorilla/mux"
)

type PanicHandlerFunc func(logger slog.Logger, w http.ResponseWriter, r *http.Request, err interface{})

func defaultPanicHandler(logger slog.Logger, w http.ResponseWriter, r *http.Request, err interface{}) {
	attributes := logging.BuildAttributes(r.Context(), r, "error", err)
	logger.ErrorContext(r.Context(), "Panic occurred", attributes...)
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