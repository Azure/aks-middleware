package logging

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/microsoft/go-otel-audit/audit"
	"github.com/microsoft/go-otel-audit/audit/msgs"
)

type OtelConfig struct {
	Client                    *audit.Client
	CustomOperationDescs      map[string]string
	CustomOperationCategories map[string]msgs.OperationCategory
	OperationAccessLevel      string
	// map of HTTP method to list of request URLs to exclude from audit
	// key is the HTTP method (e.g. "GET", "POST") and the value is a list of request URIs to ignore
	ExcludeAuditEvents map[string][]string
}

func (l *loggingMiddleware) sendOtelAuditEvent(ctx context.Context, statusCode int, req *http.Request, errorMsg string) {
	if l.otelConfig == nil || l.otelConfig.Client == nil {
		l.logger.Error("otel configuration or client is nil")
		return
	}

	if shouldExcludeAudit(req, l.otelConfig.ExcludeAuditEvents) {
		return
	}

	msg, err := createOtelAuditEvent(l.logger, statusCode, req, l.otelConfig, errorMsg)
	if err != nil {
		l.logger.Error("failed to create audit event", "error", err)
		return
	}

	l.logger.Info("sending audit logs")
	if err := l.otelConfig.Client.Send(ctx, msg); err != nil {
		l.logger.Error("failed to send audit event", "error", err)
	}
}

func shouldExcludeAudit(req *http.Request, excludeMap map[string][]string) bool {
	if excludeMap == nil {
		return false
	}

	if patterns, ok := excludeMap[req.Method]; ok {
		for _, pattern := range patterns {
			if strings.Contains(req.RequestURI, pattern) {
				return true
			}
		}
	}
	return false
}

func createOtelAuditEvent(logger *slog.Logger, statusCode int, req *http.Request, otelConfig *OtelConfig, errorMsg string) (msgs.Msg, error) {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		logger.Error("failed to split host and port", "error", err)
		return msgs.Msg{}, err
	}
	addr, err := msgs.ParseAddr(host)
	if err != nil {
		logger.Error("failed to parse address", "error", err)
		return msgs.Msg{}, err
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
		OperationCategories:          []msgs.OperationCategory{getOperationCategory(req.Method, otelConfig.CustomOperationCategories)},
		OperationCategoryDescription: getOperationCategoryDescription(req.Method, otelConfig.CustomOperationDescs),
		TargetResources:              tr,
		CallerAccessLevels:           []string{"NA"},
		OperationAccessLevel:         otelConfig.OperationAccessLevel,
		OperationName:                req.Method,
		CallerAgent:                  req.UserAgent(),
		OperationType:                getOperationType(req.Method),
		OperationResult:              getOperationResult(statusCode),
		OperationResultDescription:   getOperationResultDescription(statusCode, errorMsg),
	}

	return msgs.Msg{
		Type:   msgs.ControlPlane,
		Record: record,
	}, nil
}

func getCallerIdentities(req *http.Request) map[msgs.CallerIdentityType][]msgs.CallerIdentityEntry {
	caller := make(map[msgs.CallerIdentityType][]msgs.CallerIdentityEntry)

	// Context:
	// Callers will setup their frontend routes like so:
	// r.HandleFunc("/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}", handler)...

	vars := mux.Vars(req)                    // Extract variables from the URL
	subscriptionID := vars["subscriptionId"] // Get subscription ID from the URL

	clientAppID := req.Header.Get("x-ms-client-app-id")
	clientPrincipalName := req.Header.Get("x-ms-client-principal-name")
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

func getOperationCategory(method string, opCategoryMapping map[string]msgs.OperationCategory) msgs.OperationCategory {
	if opCategoryMapping != nil {
		if cat, ok := opCategoryMapping[method]; ok {
			return cat
		}
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

func getOperationResultDescription(statusCode int, errorMsg string) string {
	if statusCode >= 400 {
		if errorMsg != "" {
			return fmt.Sprintf("operation failed with status code: %d, error: %s", statusCode, errorMsg)
		}
		return fmt.Sprintf("operation failed with status code: %d", statusCode)
	}
	return "succeeded to run the operation"
}
