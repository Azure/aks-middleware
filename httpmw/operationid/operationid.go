package operationid

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
)

type contextKey string

const (
	OperationIDKey   contextKey = "operationID"
	CorrelationIDKey contextKey = "correlationID"
	// http header name ACS RP adds for it's operation ID
	RequestAcsOperationIDHeader = "x-ms-acs-operation-id"
	// Details can be found here:
	// https://github.com/Azure/azure-resource-manager-rpc/blob/master/v1.0/common-api-details.md#client-request-headers
	RequestCorrelationIDHeader = "x-ms-correlation-request-id"
)

func NewOperationIDMiddleware() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return &operationIDMiddleware{
			next: next,
		}
	}
}

type operationIDMiddleware struct {
	next http.Handler
}

func (m *operationIDMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	operationID := r.Header.Get(RequestAcsOperationIDHeader)
	correlationID := r.Header.Get(RequestCorrelationIDHeader)

	ctx = context.WithValue(ctx, OperationIDKey, operationID)
	ctx = context.WithValue(ctx, CorrelationIDKey, correlationID)

	m.next.ServeHTTP(w, r.WithContext(ctx))
}
