package operationid

import (
	"net/http"

	"github.com/gorilla/mux"
	"google.golang.org/grpc/metadata"
)

type contextKey string

const (
	CorrelationIDKey        contextKey = "correlationID"
	ARMClientRequestIDKey   contextKey = "armClientRequestID"
	GraphClientRequestIDKey contextKey = "graphClientRequestID"
	ClientSessionIDKey      contextKey = "clientSessionID"
	ClientApplicationIDKey  contextKey = "clientApplicationID"
	ClientPrincipalNameKey  contextKey = "clientPrincipalName"

	// Details can be found here:
	// https://github.com/Azure/azure-resource-manager-rpc/blob/master/v1.0/common-api-details.md#client-request-headers
	RequestCorrelationIDHeader = "x-ms-correlation-request-id"
	// RequestARMClientRequestIDHeader  Caller-specified value identifying the request, in the form of a GUID
	RequestARMClientRequestIDHeader = "x-ms-client-request-id"
	// RequestGraphClientRequestIDHeader is the http header name graph adds for the client request id.
	RequestGraphClientRequestIDHeader = "client-request-id"
	// RequestClientSessionIDHeader is the http header name ARM adds for the client session id. AKS treats it as operation
	// AKS choses to populate it with operation id.
	RequestClientSessionIDHeader = "x-ms-client-session-id"
	// RequestClientApplicationIDHeader is the http header name ARM adds for the client app id
	//https://github.com/Azure/azure-resource-manager-rpc/blob/8231d7df51aec87e6ccb5bcc04478136758b0a4c/v1.0/common-api-details.md?plain=1#L29
	RequestClientApplicationIDHeader = "x-ms-client-app-id"
	// RequestClientPrincipalNameHeader is the http header name ARM adds for the client principal name
	RequestClientPrincipalNameHeader = "x-ms-client-principal-name"
)

// HeaderExtractor defines a function to extract headers from an HTTP request.
// It returns a map where keys are metadata keys and values are the corresponding header values.
type HeaderExtractor func(r *http.Request) map[string]string

// NewOperationIDMiddleware creates a new OperationID middleware with the default extractor.
func NewOperationIDMiddleware() mux.MiddlewareFunc {
	return NewOperationIDMiddlewareWithExtractor(DefaultHeaderExtractor)
}

// NewOperationIDMiddlewareWithExtractor creates a new OperationID middleware with a custom extractor.
func NewOperationIDMiddlewareWithExtractor(extractor HeaderExtractor) mux.MiddlewareFunc {
	if extractor == nil {
		extractor = DefaultHeaderExtractor
	}
	return func(next http.Handler) http.Handler {
		return &operationIDMiddleware{
			next:      next,
			extractor: extractor,
		}
	}
}

type operationIDMiddleware struct {
	next      http.Handler
	extractor HeaderExtractor
}

func (m *operationIDMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
		string(CorrelationIDKey):        r.Header.Get(RequestCorrelationIDHeader),
		string(ARMClientRequestIDKey):   r.Header.Get(RequestARMClientRequestIDHeader),
		string(GraphClientRequestIDKey): r.Header.Get(RequestGraphClientRequestIDHeader),
		string(ClientSessionIDKey):      r.Header.Get(RequestClientSessionIDHeader),
		string(ClientApplicationIDKey):  r.Header.Get(RequestClientApplicationIDHeader),
		string(ClientPrincipalNameKey):  r.Header.Get(RequestClientPrincipalNameHeader),
	}
	return headers
}
