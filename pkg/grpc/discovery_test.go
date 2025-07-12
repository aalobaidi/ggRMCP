package grpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// mockConnectionManager implements ConnectionManager for testing
type mockConnectionManager struct {
	mock.Mock
}

func (m *mockConnectionManager) Connect(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockConnectionManager) Reconnect(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockConnectionManager) GetConnection() *grpc.ClientConn {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*grpc.ClientConn)
}

func (m *mockConnectionManager) IsConnected() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *mockConnectionManager) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockConnectionManager) Close() error {
	args := m.Called()
	return args.Error(0)
}

// mockReflectionClient implements ReflectionClient for testing
type mockReflectionClient struct {
	mock.Mock
}

func (m *mockReflectionClient) DiscoverServices(ctx context.Context) ([]ServiceInfo, error) {
	args := m.Called(ctx)
	return args.Get(0).([]ServiceInfo), args.Error(1)
}

func (m *mockReflectionClient) InvokeMethod(ctx context.Context, method MethodInfo, inputJSON string) (string, error) {
	args := m.Called(ctx, method, inputJSON)
	return args.String(0), args.Error(1)
}

func (m *mockReflectionClient) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockReflectionClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestServiceDiscoverer_InvokeMethodWithHeaders(t *testing.T) {
	// Create logger
	logger := zap.NewNop()

	// Create mock connection manager
	mockConnMgr := &mockConnectionManager{}
	mockConnMgr.On("IsConnected").Return(true)

	// Create service discoverer
	discoverer := newServiceDiscovererWithConnManager(mockConnMgr, logger)

	// Create mock reflection client
	mockReflClient := &mockReflectionClient{}

	// Set up test data
	methodInfo := MethodInfo{
		Name:              "TestMethod",
		FullName:          "test.Service.TestMethod",
		InputType:         "test.Request",
		OutputType:        "test.Response",
		IsClientStreaming: false,
		IsServerStreaming: false,
	}

	serviceInfo := ServiceInfo{
		Name:    "test.Service",
		Methods: []MethodInfo{methodInfo},
	}

	// Populate services in discoverer
	discoverer.services = map[string]ServiceInfo{
		"test.Service": serviceInfo,
	}

	// Set mock reflection client
	discoverer.reflectionClient = mockReflClient

	// Test headers to forward
	headers := map[string]string{
		"authorization": "Bearer token123",
		"x-trace-id":    "trace-456",
		"user-agent":    "test-client",
	}

	// Expected method invocation
	mockReflClient.On("InvokeMethod", mock.MatchedBy(func(ctx context.Context) bool {
		// Verify that headers were added to context metadata
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			return false
		}

		// Check all expected headers are present
		for key, expectedValue := range headers {
			values := md.Get(key)
			if len(values) == 0 || values[0] != expectedValue {
				return false
			}
		}

		return true
	}), methodInfo, `{"input":"test"}`).Return(`{"output":"result"}`, nil)

	// Test the header forwarding
	result, err := discoverer.InvokeMethodWithHeaders(
		context.Background(),
		headers,
		"test.Service",
		"TestMethod",
		`{"input":"test"}`,
	)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, `{"output":"result"}`, result)

	// Verify all expectations were met
	mockReflClient.AssertExpectations(t)
}

func TestServiceDiscoverer_InvokeMethodWithHeaders_EmptyHeaders(t *testing.T) {
	// Create logger
	logger := zap.NewNop()

	// Create mock connection manager
	mockConnMgr := &mockConnectionManager{}
	mockConnMgr.On("IsConnected").Return(true)

	// Create service discoverer
	discoverer := newServiceDiscovererWithConnManager(mockConnMgr, logger)

	// Create mock reflection client
	mockReflClient := &mockReflectionClient{}

	// Set up test data
	methodInfo := MethodInfo{
		Name:              "TestMethod",
		FullName:          "test.Service.TestMethod",
		InputType:         "test.Request",
		OutputType:        "test.Response",
		IsClientStreaming: false,
		IsServerStreaming: false,
	}

	serviceInfo := ServiceInfo{
		Name:    "test.Service",
		Methods: []MethodInfo{methodInfo},
	}

	// Populate services in discoverer
	discoverer.services = map[string]ServiceInfo{
		"test.Service": serviceInfo,
	}

	// Set mock reflection client
	discoverer.reflectionClient = mockReflClient

	// Expected method invocation with no headers
	mockReflClient.On("InvokeMethod", mock.MatchedBy(func(ctx context.Context) bool {
		// Verify that no headers were added to context metadata
		md, ok := metadata.FromOutgoingContext(ctx)
		return !ok || len(md) == 0
	}), methodInfo, `{"input":"test"}`).Return(`{"output":"result"}`, nil)

	// Test with empty headers
	result, err := discoverer.InvokeMethodWithHeaders(
		context.Background(),
		map[string]string{}, // empty headers
		"test.Service",
		"TestMethod",
		`{"input":"test"}`,
	)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, `{"output":"result"}`, result)

	// Verify all expectations were met
	mockReflClient.AssertExpectations(t)
}

func TestServiceDiscoverer_InvokeMethodWithHeaders_NotConnected(t *testing.T) {
	// Create logger
	logger := zap.NewNop()

	// Create mock connection manager
	mockConnMgr := &mockConnectionManager{}

	// Create service discoverer
	discoverer := newServiceDiscovererWithConnManager(mockConnMgr, logger)

	// Don't set reflection client (simulating not connected state)

	// Test headers
	headers := map[string]string{
		"authorization": "Bearer token123",
	}

	// Test the header forwarding when not connected
	result, err := discoverer.InvokeMethodWithHeaders(
		context.Background(),
		headers,
		"test.Service",
		"TestMethod",
		`{"input":"test"}`,
	)

	// Assertions
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected to gRPC server")
	assert.Empty(t, result)
}

func TestServiceDiscoverer_InvokeMethodWithHeaders_ServiceNotFound(t *testing.T) {
	// Create logger
	logger := zap.NewNop()

	// Create mock connection manager
	mockConnMgr := &mockConnectionManager{}
	mockConnMgr.On("IsConnected").Return(true)

	// Create service discoverer
	discoverer := newServiceDiscovererWithConnManager(mockConnMgr, logger)

	// Create mock reflection client
	mockReflClient := &mockReflectionClient{}

	// Set mock reflection client but don't populate services
	discoverer.reflectionClient = mockReflClient

	// Test headers
	headers := map[string]string{
		"authorization": "Bearer token123",
	}

	// Test with non-existent service
	result, err := discoverer.InvokeMethodWithHeaders(
		context.Background(),
		headers,
		"nonexistent.Service",
		"TestMethod",
		`{"input":"test"}`,
	)

	// Assertions
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Empty(t, result)
}

func TestServiceDiscoverer_InvokeMethodWithHeaders_MultipleHeaders(t *testing.T) {
	// Create logger
	logger := zap.NewNop()

	// Create mock connection manager
	mockConnMgr := &mockConnectionManager{}
	mockConnMgr.On("IsConnected").Return(true)

	// Create service discoverer
	discoverer := newServiceDiscovererWithConnManager(mockConnMgr, logger)

	// Create mock reflection client
	mockReflClient := &mockReflectionClient{}

	// Set up test data
	methodInfo := MethodInfo{
		Name:              "TestMethod",
		FullName:          "test.Service.TestMethod",
		InputType:         "test.Request",
		OutputType:        "test.Response",
		IsClientStreaming: false,
		IsServerStreaming: false,
	}

	serviceInfo := ServiceInfo{
		Name:    "test.Service",
		Methods: []MethodInfo{methodInfo},
	}

	// Populate services in discoverer
	discoverer.services = map[string]ServiceInfo{
		"test.Service": serviceInfo,
	}

	// Set mock reflection client
	discoverer.reflectionClient = mockReflClient

	// Test multiple headers with same key (should use AppendToOutgoingContext behavior)
	headers := map[string]string{
		"authorization":   "Bearer token123",
		"x-trace-id":      "trace-456",
		"x-request-id":    "req-789",
		"user-agent":      "test-client/1.0",
		"content-type":    "application/json",
		"x-forwarded-for": "192.168.1.1",
	}

	// Expected method invocation
	mockReflClient.On("InvokeMethod", mock.MatchedBy(func(ctx context.Context) bool {
		// Verify that all headers were added to context metadata
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			return false
		}

		// Check all expected headers are present
		for key, expectedValue := range headers {
			values := md.Get(key)
			if len(values) == 0 || values[0] != expectedValue {
				return false
			}
		}

		return true
	}), methodInfo, `{"complex":"input","with":{"nested":"data"}}`).Return(`{"complex":"response"}`, nil)

	// Test the header forwarding with multiple headers
	result, err := discoverer.InvokeMethodWithHeaders(
		context.Background(),
		headers,
		"test.Service",
		"TestMethod",
		`{"complex":"input","with":{"nested":"data"}}`,
	)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, `{"complex":"response"}`, result)

	// Verify all expectations were met
	mockReflClient.AssertExpectations(t)
}
