package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/metadata"

	"github.com/stretchr/testify/assert"
)

func TestWithMetadataMiddlewareIntegration(t *testing.T) {
	// Initialize a flag to check if WithMetadata is called
	metadataCalled := false

	// Mock the WithMetadata function for integration
	withMetadataMock := func(ctx context.Context, r *http.Request) metadata.MD {
		metadataCalled = true // Set the flag to true when the function is called
		md := metadata.Pairs("x-custom-metadata-key", "test-value")
		return md
	}

	// // Set up allowedHeaders and headersToMetadata for the test
	// allowedHeaders := map[string]string{}
	// headersToMetadata := map[string]string{
	// 	"X-Custom-Request-Header": "x-custom-metadata-key",
	// }

	// Create the mux options using the mock function
	muxOptions := []runtime.ServeMuxOption{
		runtime.WithMetadata(withMetadataMock),
	}

	// Create a ServeMux with the mux options
	mux := runtime.NewServeMux(muxOptions...)

	// Register a handler with the mux
	handler := func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		// Check if metadata exists in context
		md, ok := metadata.FromIncomingContext(r.Context())
		if !ok || md == nil {
			t.Errorf("Expected metadata in context, but got none")
		}
		// Ensure the expected metadata is present
		assert.Contains(t, md.Get("x-custom-metadata-key"), "test-value")
		w.WriteHeader(http.StatusOK)
	}

	// Register the handler for a specific path
	err := mux.HandlePath("GET", "/", handler)
	if err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}

	// Create a test request
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Custom-Request-Header", "test-value")

	// Create a ResponseRecorder to capture the response
	rr := httptest.NewRecorder()

	// Serve the HTTP request (This is where the integration happens)
	mux.ServeHTTP(rr, req)

	// Assert that the response code is OK
	assert.Equal(t, http.StatusOK, rr.Code)

	// Assert that the WithMetadata function was called
	assert.True(t, metadataCalled, "Expected WithMetadata to be called")
}
