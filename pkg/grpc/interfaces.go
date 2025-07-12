package grpc

import (
	"context"
	"time"

	grpcLib "google.golang.org/grpc"
)

// ConnectionManager manages gRPC connections with health checking and reconnection
type ConnectionManager interface {
	// Connect establishes a connection to the gRPC server
	Connect(ctx context.Context) error

	// GetConnection returns the current connection
	GetConnection() *grpcLib.ClientConn

	// IsConnected checks if the connection is healthy
	IsConnected() bool

	// Reconnect attempts to reconnect to the server
	Reconnect(ctx context.Context) error

	// HealthCheck performs a health check on the connection
	HealthCheck(ctx context.Context) error

	// Close closes the connection
	Close() error
}

// ServiceDiscoverer discovers and manages gRPC services
type ServiceDiscoverer interface {
	// Connect establishes connection to the gRPC server
	Connect(ctx context.Context) error

	// DiscoverServices discovers all available services
	DiscoverServices(ctx context.Context) error

	// GetServices returns all discovered services
	GetServices() map[string]ServiceInfo

	// GetService returns information about a specific service
	GetService(name string) (ServiceInfo, bool)

	// GetMethod returns information about a specific method
	GetMethod(serviceName, methodName string) (MethodInfo, bool)

	// InvokeMethod invokes a gRPC method
	InvokeMethod(ctx context.Context, serviceName, methodName string, inputJSON string) (string, error)

	// InvokeMethodWithHeaders invokes a gRPC method with forwarded headers
	InvokeMethodWithHeaders(ctx context.Context, headers map[string]string, serviceName, methodName string, inputJSON string) (string, error)

	// IsConnected checks if the discoverer is connected
	IsConnected() bool

	// HealthCheck performs a health check
	HealthCheck(ctx context.Context) error

	// Close closes the service discoverer
	Close() error

	// GetServiceCount returns the number of discovered services
	GetServiceCount() int

	// GetMethodCount returns the total number of methods across all services
	GetMethodCount() int

	// GetServiceStats returns statistics about discovered services
	GetServiceStats() map[string]interface{}
}

// ReflectionClient handles gRPC reflection API
type ReflectionClient interface {
	// DiscoverServices discovers services using reflection
	DiscoverServices(ctx context.Context) ([]ServiceInfo, error)

	// InvokeMethod invokes a method using dynamic protobuf messages
	InvokeMethod(ctx context.Context, method MethodInfo, inputJSON string) (string, error)

	// HealthCheck performs a health check
	HealthCheck(ctx context.Context) error

	// Close closes the reflection client
	Close() error
}

// ConnectionManagerConfig contains configuration for connection management
type ConnectionManagerConfig struct {
	Host           string          `json:"host"`
	Port           int             `json:"port"`
	ConnectTimeout time.Duration   `json:"connect_timeout"`
	KeepAlive      KeepAliveConfig `json:"keep_alive"`
	MaxMessageSize int             `json:"max_message_size"`
}

// KeepAliveConfig contains keep-alive settings for gRPC connections
type KeepAliveConfig struct {
	Time                time.Duration `json:"time"`
	Timeout             time.Duration `json:"timeout"`
	PermitWithoutStream bool          `json:"permit_without_stream"`
}
