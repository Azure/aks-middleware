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
type BaseOperationRequest[T any] struct {
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
    Extras T
}

// OperationRequestCustomizerFunc is a function to customize the extras.
type OperationRequestCustomizerFunc[T any] func(extras *T, headers http.Header, vars map[string]string) error

type OperationRequestOptions[T any] struct {
    Extras     T
    Customizer OperationRequestCustomizerFunc[T]
}

// NewBaseOperationRequest constructs the BaseOperationRequest.
// It extracts data from the HTTP request in the following order:
//   1. URL and Query: Extract api-version and target URI.
//   2. Headers: Extract correlation ID, accepted language, and operation ID.
//   3. Route Variables: Extract subscription ID, resource group, resource provider/type, and resource name.
//   4. Method & Body: Capture the HTTP method and read the request body.
//   5. Route Name: Optionally capture the route name from mux.CurrentRoute.
//   6. Customization: Allow further customization of extras.
func NewBaseOperationRequest[T any](req *http.Request, region string, opts OperationRequestOptions[T]) (*BaseOperationRequest[T], error) {
    op := &BaseOperationRequest[T]{
        Request: req,
        Extras:  opts.Extras,
    }
    query := req.URL.Query()
    op.APIVersion = query.Get(common.APIVersionKey)
    // if the api-version is not present in the URL, return an error
    // this is a required parameter for the operation
    if op.APIVersion == "" {
        return nil, errors.New("no api-version in URI's parameters")
    }
    op.TargetURI = req.URL.String()

    headers := req.Header
    op.CorrelationID = headers.Get(common.RequestCorrelationIDHeader)
    op.AcceptedLanguage = strings.ToLower(headers.Get(common.RequestAcceptLanguageHeader))
    if opID := headers.Get(common.RequestAcsOperationIDHeader); opID == "" {
        op.OperationID = uuid.Must(uuid.NewV4()).String()
    } else {
        op.OperationID = uuid.Must(uuid.FromString(opID)).String()
    }
    op.HttpMethod = req.Method

    vars := mux.Vars(req)
    op.SubscriptionID = vars[common.SubscriptionIDKey]
    op.ResourceGroup = vars[common.ResourceGroupKey]
    op.ResourceType = vars[common.ResourceProviderKey] + "/" + vars[common.ResourceTypeKey]
    op.ResourceName = vars[common.ResourceNameKey]
    op.Region = region

    body, err := io.ReadAll(req.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read HTTP body: %w", err)
    }
    op.Body = body

    if currRoute := mux.CurrentRoute(req); currRoute != nil {
        op.RouteName = currRoute.GetName()
    }

    if opts.Customizer != nil {
        if err := opts.Customizer(&op.Extras, headers, vars); err != nil {
            return nil, err
        }
    }
    return op, nil
}

type contextKey struct{}

func OperationRequestWithContext[T any](ctx context.Context, op *BaseOperationRequest[T]) context.Context {
    return context.WithValue(ctx, contextKey{}, op)
}

func OperationRequestFromContext[T any](ctx context.Context) *BaseOperationRequest[T] {
    if op, ok := ctx.Value(contextKey{}).(*BaseOperationRequest[T]); ok {
        return op
    }
    return nil
}