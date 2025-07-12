package grpc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aalobaidi/ggRMCP/pkg/config"
	"github.com/aalobaidi/ggRMCP/pkg/descriptors"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

// serviceDiscoverer implements ServiceDiscoverer with unified approach
// Similar to Java ServiceDiscoverer - handles both reflection and file descriptor cases
type serviceDiscoverer struct {
	logger           *zap.Logger
	connManager      ConnectionManager
	reflectionClient ReflectionClient
	services         map[string]ServiceInfo
	mu               sync.RWMutex

	// Method extraction components
	descriptorLoader *descriptors.Loader
	descriptorConfig config.DescriptorSetConfig

	// Configuration
	reconnectInterval    time.Duration
	maxReconnectAttempts int
}

// NewServiceDiscoverer creates a new service discoverer with descriptor support
func NewServiceDiscoverer(host string, port int, logger *zap.Logger, descriptorConfig config.DescriptorSetConfig) (ServiceDiscoverer, error) {
	baseConfig := ConnectionManagerConfig{
		Host:           host,
		Port:           port,
		ConnectTimeout: 5 * time.Second,
		KeepAlive: KeepAliveConfig{
			Time:                10 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		},
		MaxMessageSize: 4 * 1024 * 1024, // 4MB
	}

	connManager := NewConnectionManager(baseConfig, logger)

	return &serviceDiscoverer{
		logger:               logger.Named("discovery"),
		connManager:          connManager,
		services:             make(map[string]ServiceInfo),
		descriptorLoader:     descriptors.NewLoader(logger),
		descriptorConfig:     descriptorConfig,
		reconnectInterval:    5 * time.Second,
		maxReconnectAttempts: 5,
	}, nil
}

// Connect establishes connection to the gRPC server
func (d *serviceDiscoverer) Connect(ctx context.Context) error {
	d.logger.Info("Connecting to gRPC server via connection manager")

	// Use connection manager to establish connection
	if err := d.connManager.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect via connection manager: %w", err)
	}

	// Create reflection client with the managed connection
	conn := d.connManager.GetConnection()
	if conn == nil {
		return fmt.Errorf("connection manager returned nil connection")
	}

	d.reflectionClient = NewReflectionClient(conn, d.logger)

	// Verify connection with health check
	if err := d.reflectionClient.HealthCheck(ctx); err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	d.logger.Info("Successfully connected to gRPC server")
	return nil
}

// DiscoverServices discovers services using unified approach
func (d *serviceDiscoverer) DiscoverServices(ctx context.Context) error {
	if d.reflectionClient == nil {
		return fmt.Errorf("not connected to gRPC server")
	}

	d.logger.Info("Starting service discovery")

	// Try FileDescriptorSet first if enabled and available
	if d.descriptorConfig.Enabled && d.descriptorConfig.Path != "" {
		if err := d.discoverFromFileDescriptor(); err == nil {
			d.logger.Info("Successfully discovered services from FileDescriptorSet")
			return nil
		} else {
			d.logger.Warn("Failed to discover from FileDescriptorSet, falling back to reflection",
				zap.Error(err))
		}
	}

	// Use reflection discovery
	return d.discoverFromReflection(ctx)
}

// discoverFromFileDescriptor discovers services from FileDescriptorSet
func (d *serviceDiscoverer) discoverFromFileDescriptor() error {
	d.logger.Info("Discovering services from FileDescriptorSet", zap.String("path", d.descriptorConfig.Path))

	// Load FileDescriptorSet
	fdSet, err := d.descriptorLoader.LoadFromFile(d.descriptorConfig.Path)
	if err != nil {
		return fmt.Errorf("failed to load descriptor set: %w", err)
	}

	// Build registry
	files, err := d.descriptorLoader.BuildRegistry(fdSet)
	if err != nil {
		return fmt.Errorf("failed to build file registry: %w", err)
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	d.services = make(map[string]ServiceInfo)

	// Process each file descriptor using enhanced services
	enhancedServices, err := d.descriptorLoader.ExtractServiceInfo(files)
	if err != nil {
		return fmt.Errorf("failed to extract enhanced service info: %w", err)
	}

	// Convert enhanced services to ServiceInfo
	for _, enhancedService := range enhancedServices {
		serviceInfo := ServiceInfo{
			Name:    enhancedService.Name,
			Methods: make([]MethodInfo, len(enhancedService.Methods)),
		}

		for i, enhancedMethod := range enhancedService.Methods {
			serviceInfo.Methods[i] = MethodInfo{
				Name:              enhancedMethod.Name,
				FullName:          enhancedMethod.FullName,
				Description:       enhancedMethod.Description,
				InputType:         enhancedMethod.InputType,
				OutputType:        enhancedMethod.OutputType,
				InputDescriptor:   enhancedMethod.InputDescriptor,
				OutputDescriptor:  enhancedMethod.OutputDescriptor,
				IsClientStreaming: enhancedMethod.IsClientStreaming,
				IsServerStreaming: enhancedMethod.IsServerStreaming,
			}
		}

		d.services[enhancedService.Name] = serviceInfo
	}

	d.logger.Info("FileDescriptorSet discovery completed", zap.Int("serviceCount", len(d.services)))
	return nil
}

// discoverFromReflection discovers services from reflection
func (d *serviceDiscoverer) discoverFromReflection(ctx context.Context) error {
	d.logger.Info("Discovering services from reflection")

	services, err := d.reflectionClient.DiscoverServices(ctx)
	if err != nil {
		return fmt.Errorf("failed to discover services via reflection: %w", err)
	}

	d.mu.Lock()
	d.services = make(map[string]ServiceInfo)
	for _, service := range services {
		d.services[service.Name] = service
	}
	d.mu.Unlock()

	d.logger.Info("Reflection discovery completed", zap.Int("serviceCount", len(services)))
	return nil
}

// GetServices returns all discovered services
func (d *serviceDiscoverer) GetServices() map[string]ServiceInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Return a copy to prevent external modification
	services := make(map[string]ServiceInfo)
	for name, service := range d.services {
		services[name] = service
	}

	return services
}

// GetService returns information about a specific service
func (d *serviceDiscoverer) GetService(name string) (ServiceInfo, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	service, exists := d.services[name]
	return service, exists
}

// GetMethod returns information about a specific method
func (d *serviceDiscoverer) GetMethod(serviceName, methodName string) (MethodInfo, bool) {
	service, exists := d.GetService(serviceName)
	if !exists {
		return MethodInfo{}, false
	}

	for _, method := range service.Methods {
		if method.Name == methodName {
			return method, true
		}
	}

	return MethodInfo{}, false
}

// InvokeMethod invokes a gRPC method
func (d *serviceDiscoverer) InvokeMethod(ctx context.Context, serviceName, methodName string, inputJSON string) (string, error) {
	if d.reflectionClient == nil {
		return "", fmt.Errorf("not connected to gRPC server")
	}

	// Get method info
	method, exists := d.GetMethod(serviceName, methodName)
	if !exists {
		return "", fmt.Errorf("method %s.%s not found", serviceName, methodName)
	}

	// Check for streaming methods (not supported in this implementation)
	if method.IsClientStreaming || method.IsServerStreaming {
		return "", fmt.Errorf("streaming methods are not supported")
	}

	d.logger.Debug("Invoking gRPC method",
		zap.String("service", serviceName),
		zap.String("method", methodName),
		zap.String("input", inputJSON))

	// Invoke the method
	result, err := d.reflectionClient.InvokeMethod(ctx, method, inputJSON)
	if err != nil {
		return "", fmt.Errorf("failed to invoke method: %w", err)
	}

	return result, nil
}

// InvokeMethodWithHeaders invokes a gRPC method with forwarded headers
func (d *serviceDiscoverer) InvokeMethodWithHeaders(ctx context.Context, headers map[string]string, serviceName, methodName string, inputJSON string) (string, error) {
	if d.reflectionClient == nil {
		return "", fmt.Errorf("not connected to gRPC server")
	}

	// Add headers to context metadata
	for key, value := range headers {
		ctx = metadata.AppendToOutgoingContext(ctx, key, value)
	}

	d.logger.Debug("Invoking gRPC method with headers",
		zap.String("service", serviceName),
		zap.String("method", methodName),
		zap.Any("headers", headers),
		zap.String("input", inputJSON))

	// Use the regular InvokeMethod with the enhanced context
	return d.InvokeMethod(ctx, serviceName, methodName, inputJSON)
}

// Reconnect attempts to reconnect to the gRPC server
func (d *serviceDiscoverer) Reconnect(ctx context.Context) error {
	d.logger.Info("Attempting to reconnect to gRPC server")

	var lastErr error
	for i := 0; i < d.maxReconnectAttempts; i++ {
		if i > 0 {
			d.logger.Info("Reconnect attempt",
				zap.Int("attempt", i+1),
				zap.Int("maxAttempts", d.maxReconnectAttempts))

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(d.reconnectInterval):
			}
		}

		// Use connection manager to reconnect
		if err := d.connManager.Reconnect(ctx); err != nil {
			lastErr = err
			d.logger.Warn("Reconnect attempt failed",
				zap.Int("attempt", i+1),
				zap.Error(err))
			continue
		}

		// Recreate reflection client with new connection
		conn := d.connManager.GetConnection()
		if conn == nil {
			lastErr = fmt.Errorf("connection manager returned nil connection after reconnect")
			continue
		}
		d.reflectionClient = NewReflectionClient(conn, d.logger)

		// Rediscover services after reconnection
		if err := d.DiscoverServices(ctx); err != nil {
			lastErr = err
			d.logger.Warn("Service rediscovery failed",
				zap.Int("attempt", i+1),
				zap.Error(err))
			continue
		}

		d.logger.Info("Successfully reconnected to gRPC server")
		return nil
	}

	return fmt.Errorf("failed to reconnect after %d attempts: %w", d.maxReconnectAttempts, lastErr)
}

// IsConnected checks if the discoverer is connected
func (d *serviceDiscoverer) IsConnected() bool {
	return d.connManager.IsConnected() && d.reflectionClient != nil
}

// HealthCheck performs a health check
func (d *serviceDiscoverer) HealthCheck(ctx context.Context) error {
	// Check connection manager health first
	if err := d.connManager.HealthCheck(ctx); err != nil {
		return fmt.Errorf("connection manager health check failed: %w", err)
	}

	if d.reflectionClient == nil {
		return fmt.Errorf("reflection client not initialized")
	}

	return d.reflectionClient.HealthCheck(ctx)
}

// Close closes the service discoverer
func (d *serviceDiscoverer) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.reflectionClient != nil {
		if err := d.reflectionClient.Close(); err != nil {
			d.logger.Error("Failed to close reflection client", zap.Error(err))
		}
		d.reflectionClient = nil
	}

	// Close connection manager
	if err := d.connManager.Close(); err != nil {
		d.logger.Error("Failed to close connection manager", zap.Error(err))
	}

	d.services = make(map[string]ServiceInfo)

	d.logger.Info("Service discoverer closed")
	return nil
}

// GetServiceCount returns the number of discovered services
func (d *serviceDiscoverer) GetServiceCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return len(d.services)
}

// GetMethodCount returns the total number of methods across all services
func (d *serviceDiscoverer) GetMethodCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	count := 0
	for _, service := range d.services {
		count += len(service.Methods)
	}

	return count
}

// GetServiceStats returns statistics about discovered services
func (d *serviceDiscoverer) GetServiceStats() map[string]interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()

	serviceNames := make([]string, 0, len(d.services))
	for name := range d.services {
		serviceNames = append(serviceNames, name)
	}

	stats := map[string]interface{}{
		"serviceCount": len(d.services),
		"methodCount":  d.GetMethodCount(),
		"isConnected":  d.IsConnected(),
		"services":     serviceNames,
	}

	return stats
}

// newServiceDiscovererWithConnManager creates a service discoverer with a custom connection manager (for testing)
func newServiceDiscovererWithConnManager(connManager ConnectionManager, logger *zap.Logger) *serviceDiscoverer {
	return &serviceDiscoverer{
		logger:               logger.Named("discovery"),
		connManager:          connManager,
		services:             make(map[string]ServiceInfo),
		descriptorLoader:     descriptors.NewLoader(logger),
		descriptorConfig:     config.DescriptorSetConfig{},
		reconnectInterval:    5 * time.Second,
		maxReconnectAttempts: 5,
	}
}
