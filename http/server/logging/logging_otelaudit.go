package logging

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/microsoft/go-otel-audit/audit"
	"github.com/microsoft/go-otel-audit/audit/msgs"
)

type OtelConfig struct {
	Client               *audit.Client
	CustomOperationDescs map[string]string
	OperationAccessLevel string
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

func createOtelAuditEvent(logger *slog.Logger, statusCode int, req *http.Request, otelConfig *OtelConfig) msgs.Msg {
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
