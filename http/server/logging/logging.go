package logging

import (
    "context"
    "fmt"
    "log/slog"
    "net"
    "net/http"
    "time"

    "github.com/gorilla/mux"
    "google.golang.org/grpc/metadata"
    "github.com/microsoft/go-otel-audit/audit"
    "github.com/microsoft/go-otel-audit/audit/msgs"
)

type OtelConfig struct {
    Client                    *audit.Client
    CustomOperationDescs      map[string]string
    OperationAccessLevel      string
}

// TODO (Tom): Add a logger wrapper in its own package
// https://medium.com/@ansujain/building-a-logger-wrapper-in-go-with-support-for-multiple-logging-libraries-48092b826bee

// more info about http handler here: https://pkg.go.dev/net/http#Handler
func NewLogging(logger *slog.Logger, otelConfig *OtelConfig) mux.MiddlewareFunc {
    return func(next http.Handler) http.Handler {
        return &loggingMiddleware{
            next:         next,
            now:         time.Now,
            logger:      *logger,
            otelConfig:  otelConfig,
        }
    }
}

// enforcing that loggingMiddleware implements the http.Handler interface to ensure safety at compile time
var _ http.Handler = &loggingMiddleware{}

type loggingMiddleware struct {
    next        http.Handler
    now         func() time.Time
    logger      slog.Logger
    otelConfig  *OtelConfig
}

type responseWriter struct {
    http.ResponseWriter
    statusCode int
}

func (w *responseWriter) WriteHeader(code int) {
    w.statusCode = code
    w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriter) Write(b []byte) (int, error) {
    if w.statusCode == 0 {
        w.statusCode = http.StatusOK
    }
    return w.ResponseWriter.Write(b)
}

func (l *loggingMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    customWriter := &responseWriter{ResponseWriter: w}

    startTime := l.now()
    ctx := r.Context()

    l.LogRequestStart(ctx, r, "RequestStart")
    l.next.ServeHTTP(customWriter, r.WithContext(ctx))
    endTime := l.now()

    latency := endTime.Sub(startTime)
    l.LogRequestEnd(ctx, r, "RequestEnd", customWriter.statusCode, latency)
    l.LogRequestEnd(ctx, r, "finished call", customWriter.statusCode, latency)
    l.sendOtelAuditEvent(ctx, customWriter.statusCode, r)
}

func BuildAttributes(ctx context.Context, r *http.Request, extra ...interface{}) []interface{} {
    md, ok := metadata.FromIncomingContext(ctx)
    attributes := []interface{}{
        "source", "ApiRequestLog",
        "protocol", "HTTP",
        "method_type", "unary",
        "component", "server",
        "method", r.Method,
        "service", r.Host,
        "url", r.URL.String(),
    }

    headers := make(map[string]string)
    if ok {
        for key, values := range md {
            if len(values) > 0 {
                headers[key] = values[0]
            }
        }
    }

    attributes = append(attributes, "headers", headers)
    attributes = append(attributes, extra...)
    return attributes
}

func (l *loggingMiddleware) LogRequestStart(ctx context.Context, r *http.Request, msg string) {
    attributes := BuildAttributes(ctx, r)
    l.logger.InfoContext(ctx, msg, attributes...)
}

func (l *loggingMiddleware) LogRequestEnd(ctx context.Context, r *http.Request, msg string, statusCode int, duration time.Duration) {
    attributes := BuildAttributes(ctx, r, "code", statusCode, "time_ms", duration.Milliseconds())
    l.logger.InfoContext(ctx, msg, attributes...)
}

func (l *loggingMiddleware) sendOtelAuditEvent(ctx context.Context, statusCode int, req *http.Request) {
    if l.otelConfig == nil || l.otelConfig.Client == nil {
        l.logger.Error("otel configuration or client is nil")
        return
    }

    msg := createOtelAuditEvent(l.logger, statusCode, req, l.otelConfig)
    l.logger.Info("sending audit logs")
    if err := l.otelConfig.Client.Send(ctx, msg); err != nil {
        l.logger.Error("failed to send audit event", "error", err)
    } else {
        l.logger.Info("audit event sent successfully")
    }
}

func createOtelAuditEvent(logger slog.Logger, statusCode int, req *http.Request, otelConfig *OtelConfig) msgs.Msg {
    host, _, err := net.SplitHostPort(req.RemoteAddr)
    if err != nil {
        logger.Error("failed to split host and port", "error", err)
    }
    addr, err := msgs.ParseAddr(host)
    if err != nil {
        logger.Error("failed to parse address", "error", err)
    }

    tr := map[string][]msgs.TargetResourceEntry{
        "ResourceType": {
            {
                Name:   req.RequestURI,
                Region: req.Header.Get("Region"), // Assume the region is in the header
            },
        },
    }

    record := msgs.Record{
        CallerIpAddress:              addr,
        CallerIdentities:             getCallerIdentities(req),
        OperationCategories:          []msgs.OperationCategory{getOperationCategory(req.Method, otelConfig.CustomOperationDescs)},
        OperationCategoryDescription: getOperationCategoryDescription(req.Method, otelConfig.CustomOperationDescs),
        TargetResources:              tr,
        CallerAccessLevels:           []string{"NA"},
        OperationAccessLevel:         otelConfig.OperationAccessLevel,
        OperationName:                req.Method,
        CallerAgent:                  req.UserAgent(),
        OperationType:                getOperationType(req.Method),
        OperationResult:              getOperationResult(statusCode),
        OperationResultDescription:   getOperationResultDescription(statusCode),
    }

    return msgs.Msg{
        Type:   msgs.ControlPlane,
        Record: record,
    }
}

func getCallerIdentities(req *http.Request) map[msgs.CallerIdentityType][]msgs.CallerIdentityEntry {
    caller := make(map[msgs.CallerIdentityType][]msgs.CallerIdentityEntry)

    clientAppID := req.Header.Get("x-ms-client-app-id")
    clientPrincipalName := req.Header.Get("x-ms-client-principal-name")
    subscriptionID := req.Header.Get("subscriptionID") // Assuming subscription ID is in the header
    clientTenantID := req.Header.Get("x-ms-client-tenant-id")

    if clientAppID != "" {
        caller[msgs.ApplicationID] = []msgs.CallerIdentityEntry{
            {
                Identity:    clientAppID,
                Description: "client application ID",
            },
        }
    }

    if clientPrincipalName != "" {
        caller[msgs.UPN] = []msgs.CallerIdentityEntry{
            {
                Identity:    clientPrincipalName,
                Description: "client principal name",
            },
        }
    }

    if subscriptionID != "" {
        caller[msgs.SubscriptionID] = []msgs.CallerIdentityEntry{
            {
                Identity:    subscriptionID,
                Description: "client subscription ID",
            },
        }
    }

    if clientTenantID != "" {
        caller[msgs.TenantID] = []msgs.CallerIdentityEntry{
            {
                Identity:    clientTenantID,
                Description: "client tenant ID",
            },
        }
    }

    return caller
}

func getOperationCategory(method string, opCategoryDesc map[string]string) msgs.OperationCategory {
    if _, ok := opCategoryDesc[method]; ok {
        return msgs.OCOther
    }
    return msgs.ResourceManagement
}

func getOperationCategoryDescription(method string, opCategoryDesc map[string]string) string {
    if desc, ok := opCategoryDesc[method]; ok {
        return desc
    }
    return ""
}

func getOperationType(method string) msgs.OperationType {
    switch method {
    case http.MethodGet:
        return msgs.Read
    case http.MethodPatch, http.MethodPost, http.MethodPut:
        return msgs.Update
    case http.MethodDelete:
        return msgs.Delete
    default:
        return msgs.Read
    }
}

func getOperationResult(statusCode int) msgs.OperationResult {
    if statusCode >= 400 {
        return msgs.Failure
    }
    return msgs.Success
}

func getOperationResultDescription(statusCode int) string {
    if statusCode >= 400 {
        return fmt.Sprintf("operation failed with status code: %d", statusCode)
    }
    return "succeeded to run the operation"
}