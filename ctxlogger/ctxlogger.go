package ctxlogger

import (
	"context"

	"go.goms.io/aks/rp/aks-middleware/requestid"
	"encoding/json"

	loggable "buf.build/gen/go/service-hub/loggable/protocolbuffers/go/proto"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// ExtractFunction extracts information from the ctx and/or the request and put it in the logger.
// This function is called before the application's handler is called so that it can add more context
// to the logger.
type ExtractFunction func(ctx context.Context, req any, info *grpc.UnaryServerInfo, logger log.FieldLogger) log.FieldLogger

type loggerKeyType int

const (
	loggerKey loggerKeyType = iota
)

func WithLogger(ctx context.Context, logger log.FieldLogger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

func GetLogger(ctx context.Context) log.FieldLogger {
	logger := log.New().WithField("src", "self gen, not available in ctx")
	if ctx == nil {
		return logger
	}
	if ctxlogger, ok := ctx.Value(loggerKey).(log.FieldLogger); ok {
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
func UnaryServerInterceptor(logger log.FieldLogger, extractFunction ExtractFunction) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (resp any, err error) {
		if extractFunction != nil {
			logger = extractFunction(ctx, req, info, logger)
		} else {
			logger = defaultExtractFunction(ctx, req, info, logger)
		}
		logger = logger.WithField(requestContentLogKey, FilterLogs(req))
		ctx = WithLogger(ctx, logger)
		// log.Print("logger ctx: ", ctx)
		return handler(ctx, req)
	}
}

const (
	methodLogKey         = "method"
	requestContentLogKey = "request"
)

func defaultExtractFunction(ctx context.Context, req any, info *grpc.UnaryServerInfo, logger log.FieldLogger) log.FieldLogger {
	fields := log.Fields{}
	fields[methodLogKey] = info.FullMethod
	fields[requestid.RequestIDLogKey] = requestid.GetRequestID(ctx)
	message, ok := req.(proto.Message)
	if ok {
		fields[requestContentLogKey] = message
	}
	return logger.WithFields(fields)
}

func filterLoggableFields(currentMap map[string]interface{}, message protoreflect.Message) map[string]interface{} {
	// Check if the map or the message is nil
	if currentMap == nil || message == nil {
		return currentMap
	}
	for name, value := range currentMap {
		// Get the field descriptor by name
		fd := message.Descriptor().Fields().ByName(protoreflect.Name(name))
		// Check if the field descriptor is nil
		if fd == nil {
			continue
		}
		opts := fd.Options()
		fdOpts := opts.(*descriptorpb.FieldOptions)
		loggable := proto.GetExtension(fdOpts, loggable.E_Loggable)

		// Delete the field from the map if it is not loggable
		if !loggable.(bool) {
			delete(currentMap, name)
			continue
		}
		// Check if the value is another map[string]interface{}
		if subMap, ok := value.(map[string]interface{}); ok {
			// Check if its a simple map or one containing messages
			if fd.Message() != nil && !fd.Message().IsMapEntry() {
				// Get the sub-message for the field
				subMessage := message.Get(fd).Message()
				// Call the helper function recursively on the subMap and subMessage
				currentMap[name] = filterLoggableFields(subMap, subMessage)
			}
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
			log.Println(err)
		}
		// Unmarshal the JSON bytes to a map[string]interface{}
		err = json.Unmarshal(jsonBytes, &reqPayload)
		if err != nil {
			log.Println(err)
		}
		// Filter out the fields that are not loggable using the helper function
		reqPayload = filterLoggableFields(reqPayload, message)

	}
	return reqPayload
}
