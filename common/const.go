package common

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
