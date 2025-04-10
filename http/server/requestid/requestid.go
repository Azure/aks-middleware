package requestid

import (
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/http"

	"github.com/gorilla/mux"
	"google.golang.org/grpc/metadata"
)

const (
	CorrelationIDKey      = "correlationID"
	OperationIDKey        = "operationID"
	ARMClientRequestIDKey = "armClientRequestID"

	// Details can be found here:
	// https://github.com/Azure/azure-resource-manager-rpc/blob/master/v1.0/common-api-details.md#client-request-headers
	RequestCorrelationIDHeader = "x-ms-correlation-request-id"
	// RequestAcsOperationIDHeader is the http header name ACS RP adds for operation ID (AKS specific)
	RequestAcsOperationIDHeader = "x-ms-acs-operation-id"
	// RequestARMClientRequestIDHeader  Caller-specified value identifying the request, in the form of a GUID
	RequestARMClientRequestIDHeader = "x-ms-client-request-id"
)

// HeaderExtractor defines a function to extract headers from an HTTP request.
// It returns a map where keys are metadata keys and values are the corresponding header values.
type HeaderExtractor func(r *http.Request) map[string]string

// NewRequestIDMiddleware creates a new RequestID middleware with the default extractor.
func NewRequestIDMiddleware() mux.MiddlewareFunc {
	return NewRequestIDMiddlewareWithExtractor(DefaultHeaderExtractor)
}

// NewRequestIDMiddlewareWithExtractor creates a new RequestID middleware with a custom extractor.
func NewRequestIDMiddlewareWithExtractor(extractor HeaderExtractor) mux.MiddlewareFunc {
	if extractor == nil {
		extractor = DefaultHeaderExtractor
	}
	return func(next http.Handler) http.Handler {
		return &requestIDMiddleware{
			next:      next,
			extractor: extractor,
		}
	}
}

type requestIDMiddleware struct {
	next      http.Handler
	extractor HeaderExtractor
}

func (m *requestIDMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	headers := m.extractor(r)

	// Create metadata pairs from the extracted headers
	var mdPairs []string
	for key, value := range headers {
		mdPairs = append(mdPairs, key, value)
	}
	md := metadata.Pairs(mdPairs...)

	// Add headers to incoming context metadata to make them available for forwarding
	ctx = metadata.NewIncomingContext(ctx, md)

	m.next.ServeHTTP(w, r.WithContext(ctx))
}

func DefaultHeaderExtractor(r *http.Request) map[string]string {
	headers := map[string]string{
		string(CorrelationIDKey):      r.Header.Get(RequestCorrelationIDHeader),
		string(OperationIDKey):        r.Header.Get(RequestAcsOperationIDHeader),
		string(ARMClientRequestIDKey): r.Header.Get(RequestARMClientRequestIDHeader),
	}

	// Check if AcsOperationIDHeader is missing and generate a new one if needed
	if headers[string(OperationIDKey)] == "" {
		newRequestID := generateRequestID()
		headers["request-id"] = newRequestID
	}

	return headers
}

func generateRequestID() string {
	b := make([]byte, 6)
	io.ReadFull(rand.Reader, b)
	return base64.RawURLEncoding.EncodeToString(b)
}
