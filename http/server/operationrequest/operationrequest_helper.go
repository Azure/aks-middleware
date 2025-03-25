package operationrequest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Azure/aks-middleware/http/common"
	"github.com/gofrs/uuid"
	"github.com/gorilla/mux"
)

// BaseOperationRequest contains the common fields.
type BaseOperationRequest struct {
	APIVersion       string
	TenantID         string
	SubscriptionID   string
	ResourceGroup    string
	CorrelationID    string
	OperationID      string
	AcceptedLanguage string
	TargetURI        string
	HttpMethod       string
	Body             []byte
	RouteName        string
	Request          *http.Request
	// extra fields can be stored in Extras if needed
	Extras map[string]interface{}
}

// OperationRequestCustomizer is an interface to allow caller-defined customizations.
type OperationRequestCustomizer interface {
	Customize(op *BaseOperationRequest, headers http.Header, vars map[string]string) error
}

type OperationRequestCustomizerFunc func(op *BaseOperationRequest, headers http.Header, vars map[string]string) error

func (f OperationRequestCustomizerFunc) Customize(op *BaseOperationRequest, headers http.Header, vars map[string]string) error {
	return f(op, headers, vars)
}

// NewBaseOperationRequest constructs the common part of OperationRequest.
// It applies the OperationRequestCustomizer if provided.
func NewBaseOperationRequest(req *http.Request, region string, OperationRequestCustomizer OperationRequestCustomizer) (*BaseOperationRequest, error) {
	op := &BaseOperationRequest{
		Request: req,
		Extras:  make(map[string]interface{}),
	}
	query := req.URL.Query()
	headers := req.Header

	op.TenantID = headers.Get("x-ms-home-tenant-id")
	vars := mux.Vars(req)
	op.SubscriptionID = vars["subscriptionId"]
	op.ResourceGroup = vars["resourceGroup"]
	op.APIVersion = query.Get("api-version")
	if op.APIVersion == "" {
		return nil, errors.New("no api-version in URI's parameters")
	}
	op.CorrelationID = headers.Get("x-ms-correlation-id")
	if opID := headers.Get(common.RequestAcsOperationIDHeader); opID == "" {
		op.OperationID = uuid.Must(uuid.NewV4()).String()
	} else {
		op.OperationID = uuid.Must(uuid.FromString(opID)).String()
	}
	op.AcceptedLanguage = strings.ToLower(headers.Get("Accept-Language"))
	op.TargetURI = req.URL.String()
	op.HttpMethod = req.Method

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read HTTP body: %w", err)
	}
	op.Body = body

	if currRoute := mux.CurrentRoute(req); currRoute != nil {
		op.RouteName = currRoute.GetName()
	}

	// Allow the caller to customize.
	if OperationRequestCustomizer != nil {
		if err := OperationRequestCustomizer.Customize(op, headers, vars); err != nil {
			return nil, err
		}
	}
	return op, nil
}

type contextKey struct{}

func OperationRequestWithContext(ctx context.Context, op *BaseOperationRequest) context.Context {
	return context.WithValue(ctx, contextKey{}, op)
}

func OperationRequestFromContext(ctx context.Context) *BaseOperationRequest {
	if op, ok := ctx.Value(contextKey{}).(*BaseOperationRequest); ok {
		return op
	}
	return nil
}
