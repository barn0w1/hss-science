package grpc

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// New creates a gRPC server pre-configured for the HSS Science platform.
// This function centralizes server configuration to ensure consistency across services.
func New(opts ...grpc.ServerOption) *grpc.Server {
	// Placeholder for future shared interceptors:
	// - Logging: Ensure request/response visibility.
	// - Recovery: Prevent panics from crashing the server.
	// - Auth: Enforce security policies (e.g., JWT, API Key).
	// - Tracing: Enable distributed tracing (e.g., OpenTelemetry).

	defaultOpts := []grpc.ServerOption{
		// Example: grpc.UnaryInterceptor(...),
	}

	// Merge user-provided options with defaults to allow customization while preserving defaults.
	opts = append(defaultOpts, opts...)

	s := grpc.NewServer(opts...)

	// Enable gRPC Server Reflection to improve developer experience.
	// Tools like grpcurl and Postman can use this to explore APIs dynamically.
	reflection.Register(s)

	return s
}
