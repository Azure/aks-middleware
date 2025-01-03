package common

const (
	CorrelationIDKey      = "correlationid"
	OperationIDKey        = "operationid"
	ARMClientRequestIDKey = "armclientrequestid"
	RequestIDLogKey       = "request-id"

	// Details can be found here:
	// https://github.com/Azure/azure-resource-manager-rpc/blob/master/v1.0/common-api-details.md#client-request-headers
	RequestCorrelationIDHeader = "x-ms-correlation-request-id"
	// RequestAcsOperationIDHeader is the http header name ACS RP adds for operation ID (AKS specific)
	RequestAcsOperationIDHeader = "x-ms-acs-operation-id"
	// RequestARMClientRequestIDHeader  Caller-specified value identifying the request, in the form of a GUID
	RequestARMClientRequestIDHeader = "x-ms-client-request-id"
	// RequestIDMetadataKey is the key in the gRPC
	// metadata.
	RequestIDMetadataHeader = "x-request-id"
)

var (
	MetadataToHeader = map[string]string{
		OperationIDKey:        RequestAcsOperationIDHeader,
		ARMClientRequestIDKey: RequestARMClientRequestIDHeader,
	}

	HeaderToMetadata = map[string]string{
		RequestCorrelationIDHeader:      CorrelationIDKey,
		RequestAcsOperationIDHeader:     OperationIDKey,
		RequestARMClientRequestIDHeader: ARMClientRequestIDKey,
	}
)
