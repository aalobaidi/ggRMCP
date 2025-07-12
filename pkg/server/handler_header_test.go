package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aalobaidi/ggRMCP/pkg/config"
	"github.com/aalobaidi/ggRMCP/pkg/grpc"
	"github.com/aalobaidi/ggRMCP/pkg/mcp"
	"github.com/aalobaidi/ggRMCP/pkg/session"
	"github.com/aalobaidi/ggRMCP/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// mockServiceDiscoverer implements ServiceDiscoverer for testing header forwarding
type mockServiceDiscoverer struct {
	mock.Mock
}

func (m *mockServiceDiscoverer) Connect(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockServiceDiscoverer) DiscoverServices(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockServiceDiscoverer) GetServices() map[string]grpc.ServiceInfo {
	args := m.Called()
	return args.Get(0).(map[string]grpc.ServiceInfo)
}

func (m *mockServiceDiscoverer) GetService(name string) (grpc.ServiceInfo, bool) {
	args := m.Called(name)
	return args.Get(0).(grpc.ServiceInfo), args.Bool(1)
}

func (m *mockServiceDiscoverer) GetMethod(serviceName, methodName string) (grpc.MethodInfo, bool) {
	args := m.Called(serviceName, methodName)
	return args.Get(0).(grpc.MethodInfo), args.Bool(1)
}

func (m *mockServiceDiscoverer) InvokeMethod(ctx context.Context, serviceName, methodName string, inputJSON string) (string, error) {
	args := m.Called(ctx, serviceName, methodName, inputJSON)
	return args.String(0), args.Error(1)
}

func (m *mockServiceDiscoverer) InvokeMethodWithHeaders(ctx context.Context, headers map[string]string, serviceName, methodName string, inputJSON string) (string, error) {
	args := m.Called(ctx, headers, serviceName, methodName, inputJSON)
	return args.String(0), args.Error(1)
}

func (m *mockServiceDiscoverer) Reconnect(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockServiceDiscoverer) IsConnected() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *mockServiceDiscoverer) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockServiceDiscoverer) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockServiceDiscoverer) GetServiceCount() int {
	args := m.Called()
	return args.Int(0)
}

func (m *mockServiceDiscoverer) GetMethodCount() int {
	args := m.Called()
	return args.Int(0)
}

func (m *mockServiceDiscoverer) GetServiceStats() map[string]interface{} {
	args := m.Called()
	return args.Get(0).(map[string]interface{})
}

func TestHandler_HeaderFilteringAndForwarding(t *testing.T) {
	// Create logger
	logger := zap.NewNop()

	// Create mock service discoverer
	mockDiscoverer := &mockServiceDiscoverer{}

	// Create session manager
	sessionManager := session.NewManager(logger)
	defer func() { _ = sessionManager.Close() }()

	// Create tool builder
	toolBuilder := tools.NewBuilder(logger)

	// Configure header forwarding to allow specific headers
	// Note: Header matching should work with both canonical and non-canonical forms
	headerConfig := config.HeaderForwardingConfig{
		Enabled: true,
		AllowedHeaders: []string{
			"authorization",
			"x-trace-id",
			"user-agent",
		},
		BlockedHeaders: []string{
			"cookie",
			"set-cookie",
		},
		ForwardAll:    false,
		CaseSensitive: false,
	}

	// Create handler
	handler := NewHandler(logger, mockDiscoverer, sessionManager, toolBuilder, headerConfig)

	// Set up test services data
	testServices := map[string]grpc.ServiceInfo{
		"test.Service": {
			Name: "test.Service",
			Methods: []grpc.MethodInfo{
				{
					Name:              "TestMethod",
					FullName:          "test.Service.TestMethod",
					IsClientStreaming: false,
					IsServerStreaming: false,
				},
			},
		},
	}

	// Set up mock expectations
	mockDiscoverer.On("GetServices").Return(testServices)

	// Expected filtered headers (should include authorization, x-trace-id, user-agent but not cookie)
	// Note: HTTP headers are canonicalized by Go's http package
	expectedFilteredHeaders := map[string]string{
		"Authorization": "Bearer token123",
		"X-Trace-Id":    "trace-456",
		"User-Agent":    "test-client",
	}

	mockDiscoverer.On("InvokeMethodWithHeaders",
		mock.Anything, // context (could be various types due to timeout/cancellation)
		expectedFilteredHeaders,
		"test.Service",
		"TestMethod",
		`{"input":"test"}`,
	).Return(`{"output":"success"}`, nil)

	// Create test request with headers
	requestBody := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      mcp.RequestID{Value: 1},
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "test_service_testmethod",
			"arguments": map[string]interface{}{
				"input": "test",
			},
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)

	// Create HTTP request with various headers
	req := httptest.NewRequest("POST", "/", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token123") // Should be forwarded
	req.Header.Set("X-Trace-ID", "trace-456")          // Should be forwarded
	req.Header.Set("User-Agent", "test-client")        // Should be forwarded
	req.Header.Set("Cookie", "session=abc123")         // Should be blocked
	req.Header.Set("X-Random-Header", "random-value")  // Should be blocked (not in allowed list)
	req.Header.Set("Mcp-Session-Id", "test-session-123")

	// Create response recorder
	w := httptest.NewRecorder()

	// Call the handler
	handler.ServeHTTP(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	// Parse response
	var response mcp.JSONRPCResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Check that the response is successful
	assert.Nil(t, response.Error)
	assert.NotNil(t, response.Result)

	// Verify mock expectations
	mockDiscoverer.AssertExpectations(t)
}

func TestHandler_HeaderFilteringDisabled(t *testing.T) {
	// Create logger
	logger := zap.NewNop()

	// Create mock service discoverer
	mockDiscoverer := &mockServiceDiscoverer{}

	// Create session manager
	sessionManager := session.NewManager(logger)
	defer func() { _ = sessionManager.Close() }()

	// Create tool builder
	toolBuilder := tools.NewBuilder(logger)

	// Configure header forwarding to be disabled
	headerConfig := config.HeaderForwardingConfig{
		Enabled: false, // Disabled
	}

	// Create handler
	handler := NewHandler(logger, mockDiscoverer, sessionManager, toolBuilder, headerConfig)

	// Set up test services data
	testServices := map[string]grpc.ServiceInfo{
		"test.Service": {
			Name: "test.Service",
			Methods: []grpc.MethodInfo{
				{
					Name:              "TestMethod",
					FullName:          "test.Service.TestMethod",
					IsClientStreaming: false,
					IsServerStreaming: false,
				},
			},
		},
	}

	// Set up mock expectations
	mockDiscoverer.On("GetServices").Return(testServices)

	// Should call InvokeMethod (without headers) when header forwarding is disabled
	mockDiscoverer.On("InvokeMethod",
		mock.Anything, // context (could be various types due to timeout/cancellation)
		"test.Service",
		"TestMethod",
		`{"input":"test"}`,
	).Return(`{"output":"success"}`, nil)

	// Create test request
	requestBody := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      mcp.RequestID{Value: 1},
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "test_service_testmethod",
			"arguments": map[string]interface{}{
				"input": "test",
			},
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)

	// Create HTTP request with headers (they should be ignored)
	req := httptest.NewRequest("POST", "/", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Set("X-Trace-ID", "trace-456")
	req.Header.Set("Mcp-Session-Id", "test-session-123")

	// Create response recorder
	w := httptest.NewRecorder()

	// Call the handler
	handler.ServeHTTP(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify mock expectations
	mockDiscoverer.AssertExpectations(t)
}

func TestHandler_HeaderFilteringForwardAll(t *testing.T) {
	// Create logger
	logger := zap.NewNop()

	// Create mock service discoverer
	mockDiscoverer := &mockServiceDiscoverer{}

	// Create session manager
	sessionManager := session.NewManager(logger)
	defer func() { _ = sessionManager.Close() }()

	// Create tool builder
	toolBuilder := tools.NewBuilder(logger)

	// Configure header forwarding to forward all headers except blocked ones
	headerConfig := config.HeaderForwardingConfig{
		Enabled: true,
		BlockedHeaders: []string{
			"cookie",
			"set-cookie",
			"host",
			"mcp-session-id", // Block MCP-specific headers
		},
		ForwardAll:    true, // Forward all except blocked
		CaseSensitive: false,
	}

	// Create handler
	handler := NewHandler(logger, mockDiscoverer, sessionManager, toolBuilder, headerConfig)

	// Set up test services data
	testServices := map[string]grpc.ServiceInfo{
		"test.Service": {
			Name: "test.Service",
			Methods: []grpc.MethodInfo{
				{
					Name:              "TestMethod",
					FullName:          "test.Service.TestMethod",
					IsClientStreaming: false,
					IsServerStreaming: false,
				},
			},
		},
	}

	// Set up mock expectations
	mockDiscoverer.On("GetServices").Return(testServices)

	// Expected filtered headers (should include all except blocked ones)
	// Note: HTTP headers are canonicalized by Go's http package
	// Mcp-Session-Id and Cookie should be filtered out because they're in BlockedHeaders
	expectedFilteredHeaders := map[string]string{
		"Authorization":   "Bearer token123",
		"X-Trace-Id":      "trace-456",
		"User-Agent":      "test-client",
		"X-Custom-Header": "custom-value",
		"Content-Type":    "application/json",
		// Cookie should be filtered out
		// Mcp-Session-Id should be filtered out (in blocked headers)
	}

	mockDiscoverer.On("InvokeMethodWithHeaders",
		mock.Anything, // context (could be various types due to timeout/cancellation)
		expectedFilteredHeaders,
		"test.Service",
		"TestMethod",
		`{"input":"test"}`,
	).Return(`{"output":"success"}`, nil)

	// Create test request
	requestBody := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      mcp.RequestID{Value: 1},
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "test_service_testmethod",
			"arguments": map[string]interface{}{
				"input": "test",
			},
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)

	// Create HTTP request with various headers
	req := httptest.NewRequest("POST", "/", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Set("X-Trace-ID", "trace-456")
	req.Header.Set("User-Agent", "test-client")
	req.Header.Set("X-Custom-Header", "custom-value")
	req.Header.Set("Cookie", "session=abc123")           // Should be blocked
	req.Header.Set("Mcp-Session-Id", "test-session-123") // Should be filtered out by extractHeaders

	// Create response recorder
	w := httptest.NewRecorder()

	// Call the handler
	handler.ServeHTTP(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify mock expectations
	mockDiscoverer.AssertExpectations(t)
}

func TestHandler_HeaderFilteringCaseSensitive(t *testing.T) {
	// Create logger
	logger := zap.NewNop()

	// Create mock service discoverer
	mockDiscoverer := &mockServiceDiscoverer{}

	// Create session manager
	sessionManager := session.NewManager(logger)
	defer func() { _ = sessionManager.Close() }()

	// Create tool builder
	toolBuilder := tools.NewBuilder(logger)

	// Configure header forwarding with case sensitive matching
	headerConfig := config.HeaderForwardingConfig{
		Enabled: true,
		AllowedHeaders: []string{
			"Authorization", // This will match canonicalized "Authorization"
			"X-Trace-Id",    // This will match canonicalized "X-Trace-Id" (Go canonicalizes X-Trace-ID to X-Trace-Id)
		},
		ForwardAll:    false,
		CaseSensitive: true, // Case sensitive
	}

	// Create handler
	handler := NewHandler(logger, mockDiscoverer, sessionManager, toolBuilder, headerConfig)

	// Set up test services data
	testServices := map[string]grpc.ServiceInfo{
		"test.Service": {
			Name: "test.Service",
			Methods: []grpc.MethodInfo{
				{
					Name:              "TestMethod",
					FullName:          "test.Service.TestMethod",
					IsClientStreaming: false,
					IsServerStreaming: false,
				},
			},
		},
	}

	// Set up mock expectations
	mockDiscoverer.On("GetServices").Return(testServices)

	// Expected filtered headers (only exact case matches should be forwarded)
	// Since HTTP headers are canonicalized by Go's http package, we need to test
	// the case sensitivity at the filter level, not at the HTTP level
	expectedFilteredHeaders := map[string]string{
		"Authorization": "Bearer token123",
		"X-Trace-Id":    "trace-456", // X-Trace-ID gets canonicalized to X-Trace-Id
		// Other headers should not be forwarded due to case sensitivity
	}

	mockDiscoverer.On("InvokeMethodWithHeaders",
		mock.Anything, // context (could be various types due to timeout/cancellation)
		expectedFilteredHeaders,
		"test.Service",
		"TestMethod",
		`{"input":"test"}`,
	).Return(`{"output":"success"}`, nil)

	// Create test request
	requestBody := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      mcp.RequestID{Value: 1},
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "test_service_testmethod",
			"arguments": map[string]interface{}{
				"input": "test",
			},
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)

	// Create HTTP request with headers
	// Note: Since HTTP headers are case-insensitive and Go canonicalizes them,
	// we can't actually test case sensitivity at the HTTP level
	// This test verifies that the filter respects case sensitivity for the canonicalized names
	req := httptest.NewRequest("POST", "/", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token123") // Canonicalized header - should be forwarded
	req.Header.Set("X-Trace-ID", "trace-456")          // Canonicalized to X-Trace-Id - should be forwarded
	req.Header.Set("X-Custom-Header", "custom-value")  // Not in allowed list - should not be forwarded
	req.Header.Set("Mcp-Session-Id", "test-session-123")

	// Create response recorder
	w := httptest.NewRecorder()

	// Call the handler
	handler.ServeHTTP(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify mock expectations
	mockDiscoverer.AssertExpectations(t)
}
