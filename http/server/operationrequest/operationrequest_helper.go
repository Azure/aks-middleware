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
    Region           string
    ResourceType     string
    ResourceName     string
    // extra fields can be stored in Extras if needed
    Extras map[string]interface{}
}

type OperationRequestCustomizerFunc func(extras map[string]interface{}, headers http.Header, vars map[string]string) error

// NewBaseOperationRequest constructs the common part of OperationRequest.
// It applies the customizer (if provided) so that the caller can add extra information.
func NewBaseOperationRequest(req *http.Request, region string, customizer OperationRequestCustomizerFunc) (*BaseOperationRequest, error) {
    op := &BaseOperationRequest{
        Request: req,
        Extras:  make(map[string]interface{}),
    }
    query := req.URL.Query()
    headers := req.Header

    vars := mux.Vars(req)
    op.SubscriptionID = vars[common.SubscriptionIDKey]
    op.ResourceGroup = vars[common.ResourceGroupKey]
    op.ResourceType = vars[common.ResourceProviderKey] + "/" + vars[common.ResourceTypeKey]
    op.ResourceName = vars[common.ResourceNameKey]
    op.Region = region

    op.APIVersion = query.Get(common.APIVersionKey)
    if op.APIVersion == "" {
        return nil, errors.New("no api-version in URI's parameters")
    }
    op.CorrelationID = headers.Get(common.RequestCorrelationIDHeader)
    if opID := headers.Get(common.RequestAcsOperationIDHeader); opID == "" {
        op.OperationID = uuid.Must(uuid.NewV4()).String()
    } else {
        op.OperationID = uuid.Must(uuid.FromString(opID)).String()
    }
    op.AcceptedLanguage = strings.ToLower(headers.Get(common.RequestAcceptLanguageHeader))
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

    // Allow the caller to customize via Extras only.
    if customizer != nil {
        if err := customizer(op.Extras, headers, vars); err != nil {
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