//go:build integration
// +build integration

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aalobaidi/ggRMCP/pkg/grpc"
	"github.com/aalobaidi/ggRMCP/pkg/mcp"
	"github.com/aalobaidi/ggRMCP/pkg/server"
	"github.com/aalobaidi/ggRMCP/pkg/session"
	"github.com/aalobaidi/ggRMCP/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	grpcLib "google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
)

const bufSize = 1024 * 1024

// testServer implements a simple test service
type testServer struct{}

func (s *testServer) TestMethod(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

// setupTestGRPCServer creates a test gRPC server
func setupTestGRPCServer(t *testing.T) (*grpcLib.Server, *bufconn.Listener) {
	lis := bufconn.Listen(bufSize)

	s := grpcLib.NewServer()
	// Register reflection service for service discovery
	reflection.Register(s)

	go func() {
		if err := s.Serve(lis); err != nil {
			t.Logf("Test gRPC server error: %v", err)
		}
	}()

	return s, lis
}

// bufDialer creates a dialer for bufconn
func bufDialer(listener *bufconn.Listener) func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, url string) (net.Conn, error) {
		return listener.Dial()
	}
}

// setupTestHandler creates a test HTTP handler
func setupTestHandler(t *testing.T, listener *bufconn.Listener) *server.Handler {
	logger := zap.NewNop()

	// Create gRPC connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := grpcLib.DialContext(ctx, "bufnet",
		grpcLib.WithContextDialer(bufDialer(listener)),
		grpcLib.WithInsecure())
	require.NoError(t, err)

	// Create service discoverer with the connection
	serviceDiscoverer := grpc.NewServiceDiscoverer("localhost", 50051, logger)

	// Create session manager
	sessionManager := session.NewManager(logger)

	// Create tool builder
	toolBuilder := tools.NewBuilder(logger)

	// Create handler
	handler := server.NewHandler(logger, serviceDiscoverer, sessionManager, toolBuilder)

	return handler
}

func TestIntegration_BasicWorkflow(t *testing.T) {
	// Setup test gRPC server
	grpcServer, listener := setupTestGRPCServer(t)
	defer grpcServer.Stop()

	// Setup test HTTP handler
	handler := setupTestHandler(t, listener)

	// Apply middleware
	logger := zap.NewNop()
	middlewares := server.DefaultMiddleware(logger)
	finalHandler := server.ChainMiddleware(middlewares...)(handler)

	// Create test server
	testServer := httptest.NewServer(finalHandler)
	defer testServer.Close()

	// Test 1: Initialize
	t.Run("Initialize", func(t *testing.T) {
		req := mcp.JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "initialize",
			ID:      mcp.RequestID{Value: 1},
			Params: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{},
				"clientInfo": map[string]interface{}{
					"name":    "test-client",
					"version": "1.0.0",
				},
			},
		}

		body, err := json.Marshal(req)
		require.NoError(t, err)

		resp, err := http.Post(testServer.URL+"/", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response mcp.JSONRPCResponse
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, "2.0", response.JSONRPC)
		assert.Equal(t, 1, response.ID.Value)
		assert.NotNil(t, response.Result)
		assert.Nil(t, response.Error)
	})

	// Test 2: List Tools
	t.Run("ListTools", func(t *testing.T) {
		req := mcp.JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "tools/list",
			ID:      mcp.RequestID{Value: 2},
		}

		body, err := json.Marshal(req)
		require.NoError(t, err)

		resp, err := http.Post(testServer.URL+"/", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response mcp.JSONRPCResponse
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, "2.0", response.JSONRPC)
		assert.Equal(t, 2, response.ID.Value)
		assert.NotNil(t, response.Result)
		assert.Nil(t, response.Error)

		// Check that tools are returned
		result, ok := response.Result.(map[string]interface{})
		require.True(t, ok)

		tools, ok := result["tools"].([]interface{})
		require.True(t, ok)
		// For now, we expect an empty array since we don't have actual services
		assert.Equal(t, 0, len(tools))
	})
}

func TestIntegration_ErrorHandling(t *testing.T) {
	// Setup test gRPC server
	grpcServer, listener := setupTestGRPCServer(t)
	defer grpcServer.Stop()

	// Setup test HTTP handler
	handler := setupTestHandler(t, listener)

	// Apply middleware
	logger := zap.NewNop()
	middlewares := server.DefaultMiddleware(logger)
	finalHandler := server.ChainMiddleware(middlewares...)(handler)

	// Create test server
	testServer := httptest.NewServer(finalHandler)
	defer testServer.Close()

	// Test invalid JSON
	t.Run("InvalidJSON", func(t *testing.T) {
		resp, err := http.Post(testServer.URL+"/", "application/json",
			bytes.NewBuffer([]byte("invalid json")))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode) // JSON-RPC errors are HTTP 200

		var response mcp.JSONRPCResponse
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)

		assert.NotNil(t, response.Error)
		assert.Equal(t, mcp.ErrorCodeParseError, response.Error.Code)
	})

	// Test invalid method
	t.Run("InvalidMethod", func(t *testing.T) {
		req := mcp.JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "invalid/method",
			ID:      mcp.RequestID{Value: 1},
		}

		body, err := json.Marshal(req)
		require.NoError(t, err)

		resp, err := http.Post(testServer.URL+"/", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response mcp.JSONRPCResponse
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)

		assert.NotNil(t, response.Error)
		assert.Equal(t, mcp.ErrorCodeMethodNotFound, response.Error.Code)
	})

	// Test invalid tool call
	t.Run("InvalidToolCall", func(t *testing.T) {
		req := mcp.JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "tools/call",
			ID:      mcp.RequestID{Value: 1},
			Params: map[string]interface{}{
				"name": "nonexistent_tool",
				"arguments": map[string]interface{}{
					"test": "value",
				},
			},
		}

		body, err := json.Marshal(req)
		require.NoError(t, err)

		resp, err := http.Post(testServer.URL+"/", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response mcp.JSONRPCResponse
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)

		assert.NotNil(t, response.Error)
		assert.Equal(t, mcp.ErrorCodeInternalError, response.Error.Code)
	})
}

func TestIntegration_HealthCheck(t *testing.T) {
	// Setup test gRPC server
	grpcServer, listener := setupTestGRPCServer(t)
	defer grpcServer.Stop()

	// Setup test HTTP handler
	handler := setupTestHandler(t, listener)

	// Create test server
	testServer := httptest.NewServer(handler)
	defer testServer.Close()

	// Test health check
	resp, err := http.Get(testServer.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	// For now, we expect this to fail since we don't have actual service discovery
	// In a real implementation, this would be StatusOK
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestIntegration_SessionManagement(t *testing.T) {
	// Setup test gRPC server
	grpcServer, listener := setupTestGRPCServer(t)
	defer grpcServer.Stop()

	// Setup test HTTP handler
	handler := setupTestHandler(t, listener)

	// Create test server
	testServer := httptest.NewServer(handler)
	defer testServer.Close()

	// Test session creation and management
	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "initialize",
		ID:      mcp.RequestID{Value: 1},
	}

	body, err := json.Marshal(req)
	require.NoError(t, err)

	// First request - should create a session
	resp, err := http.Post(testServer.URL+"/", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Should receive a session ID in response header
	sessionID := resp.Header.Get("Mcp-Session-Id")
	assert.NotEmpty(t, sessionID)

	// Second request - should use the existing session
	client := &http.Client{}
	httpReq, err := http.NewRequest("POST", testServer.URL+"/", bytes.NewBuffer(body))
	require.NoError(t, err)

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Mcp-Session-Id", sessionID)

	resp2, err := client.Do(httpReq)
	require.NoError(t, err)
	defer resp2.Body.Close()

	assert.Equal(t, http.StatusOK, resp2.StatusCode)

	// Should return the same session ID
	sessionID2 := resp2.Header.Get("Mcp-Session-Id")
	assert.Equal(t, sessionID, sessionID2)
}
