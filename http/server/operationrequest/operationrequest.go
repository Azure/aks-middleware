package operationrequest

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Azure/aks-middleware/http/common"
	"github.com/gorilla/mux"
)

// NewOperationRequest creates an operationRequestMiddleware with an optional customizer.
// Use nil for customizer when no customization is needed.
func NewOperationRequest(region string, customizer OperationRequestCustomizerFunc) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return &operationRequestMiddleware{
			next:       next,
			region:     region,
			customizer: customizer,
		}
	}
}

const ARMTimeout = 60 * time.Second

var _ http.Handler = &operationRequestMiddleware{}

type operationRequestMiddleware struct {
	next       http.Handler
	region     string
	customizer OperationRequestCustomizerFunc
}

func (op *operationRequestMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	opReq, err := NewBaseOperationRequest(r, op.region, op.customizer)
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
