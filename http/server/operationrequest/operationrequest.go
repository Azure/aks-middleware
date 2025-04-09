package operationrequest

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Azure/aks-middleware/http/common"
	"github.com/gorilla/mux"
)

const ARMTimeout = 60 * time.Second

var _ http.Handler = &operationRequestMiddleware{}

// NewOperationRequest creates an operationRequestMiddleware using the provided options.
// The options contains both the Extras value and its customizer
func NewOperationRequest(region string, opts OperationRequestOptions) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return &operationRequestMiddleware{
			next:   next,
			region: region,
			opts:   opts,
		}
	}
}

type operationRequestMiddleware struct {
	next   http.Handler
	region string
	opts   OperationRequestOptions
}

func (op *operationRequestMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	opReq, err := NewBaseOperationRequest(r, op.region, op.opts)
	if err != nil {
		http.Error(w, fmt.Errorf("failed to create operation request: %w", err).Error(), http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	ctx = OperationRequestWithContext(ctx, opReq)
	ctx, cancel := context.WithTimeout(ctx, ARMTimeout)
	defer cancel()
	enrichedReq := r.WithContext(ctx)
	enrichedReq.Header.Set(common.RequestAcsOperationIDHeader, opReq.OperationID)
	op.next.ServeHTTP(w, enrichedReq)
}
