package ctxlogger

import (
	"context"

	"encoding/json"

	log "log/slog"

	loggable "buf.build/gen/go/service-hub/loggable/protocolbuffers/go/proto"
	"github.com/Azure/aks-middleware/grpc/common"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// ExtractFunction extracts information from the ctx and/or the request and put it in the logger.
// This function is called before the application's handler is called so that it can add more context
// to the logger.
type ExtractFunction func(ctx context.Context, req any, info *grpc.UnaryServerInfo, logger *log.Logger) *log.Logger

type loggerKeyType int

const (
	loggerKey loggerKeyType = iota
)

func WithLogger(ctx context.Context, logger *log.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

func GetLogger(ctx context.Context) *log.Logger {
	logger := log.Default().With("src", "self gen, not available in ctx")
	if ctx == nil {
		return logger
	}
	if ctxlogger, ok := ctx.Value(loggerKey).(*log.Logger); ok {
		return ctxlogger
	}
	return logger
}

// UnaryServerInterceptor returns a UnaryServerInterceptor.
// extractFunction can be nil if the defaultExtractFunction() is good enough.
// extractFunction is for ctx or request specific information.
// For information that doesn't change with ctx/request, pass the information via logger.

// The first registerred interceptor will be called first.
// Need to register requestid first to add request-id.
// Then the logger can get the request-id.
func UnaryServerInterceptor(logger *log.Logger, extractFunction ExtractFunction) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (resp any, err error) {
		l := logger
		if extractFunction != nil {
			l = extractFunction(ctx, req, info, l)
		} else {
			l = defaultExtractFunction(ctx, req, info, l)
		}
		l = l.With(requestContentLogKey, FilterLogs(req))
		ctx = WithLogger(ctx, l)
		// log.Print("logger ctx: ", ctx)
		return handler(ctx, req)
	}
}

const (
	methodLogKey         = "method"
	requestContentLogKey = "request"
)

func defaultExtractFunction(ctx context.Context, req any, info *grpc.UnaryServerInfo, logger *log.Logger) *log.Logger {
	l := logger
	l = l.With(methodLogKey, info.FullMethod)
	l = l.With(common.GetFields(ctx)...)
	return l
}

func filterLoggableFields(currentMap map[string]interface{}, message protoreflect.Message) map[string]interface{} {
	if currentMap == nil || message == nil {
		return currentMap
	}

	// Iterate through all fields in the JSON map (keys are JSON field names like "attestedData")
	for name, value := range currentMap {
		// Find the protobuf field descriptor using the JSON field name
		// This handles the mapping between JSON names ("attestedData") and proto names ("attested_data")
		fd := message.Descriptor().Fields().ByJSONName(name)
		if fd == nil {
			// Field not found in protobuf descriptor - skip this field (keep it in the map)
			continue
		}

		// Get the field options from the protobuf descriptor
		opts, ok := fd.Options().(*descriptorpb.FieldOptions)
		if !ok {
			// No field options available - skip this field (keep it in the map)
			continue
		}

		// Check for the loggable extension: [(servicehub.fieldoptions.loggable) = false]
		if loggableExt := proto.GetExtension(opts, loggable.E_Loggable); loggableExt != nil {
			// Extension exists, check if it's a boolean and if logging is disabled
			if allowed, ok := loggableExt.(bool); ok && !allowed {
				// Field is marked as not loggable - remove it from the map
				delete(currentMap, name)
				continue
			}
		}

		// Handle nested messages recursively
		if subMap, ok := value.(map[string]interface{}); ok && fd.Message() != nil && !fd.Message().IsMapEntry() {
			// This field contains a nested message, recursively filter its fields
			subMessage := message.Get(fd).Message()
			currentMap[name] = filterLoggableFields(subMap, subMessage)
		}
	}
	return currentMap
}

func FilterLogs(req any) map[string]interface{} {
	in, ok := req.(proto.Message)
	var reqPayload map[string]interface{}
	if ok {
		// Get the protoreflect.Message interface for the message
		message := in.ProtoReflect()
		// Marshal the message to JSON bytes
		jsonBytes, err := protojson.Marshal(message.Interface().(protoreflect.ProtoMessage))
		if err != nil {
			log.Error(err.Error())
		}
		// Unmarshal the JSON bytes to a map[string]interface{}
		err = json.Unmarshal(jsonBytes, &reqPayload)
		if err != nil {
			log.Error(err.Error())
		}
		// Filter out the fields that are not loggable using the helper function
		reqPayload = filterLoggableFields(reqPayload, message)

	}
	return reqPayload
}
