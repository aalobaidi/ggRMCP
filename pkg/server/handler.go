package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aalobaidi/ggRMCP/pkg/config"
	"github.com/aalobaidi/ggRMCP/pkg/grpc"
	"github.com/aalobaidi/ggRMCP/pkg/headers"
	"github.com/aalobaidi/ggRMCP/pkg/mcp"
	"github.com/aalobaidi/ggRMCP/pkg/session"
	"github.com/aalobaidi/ggRMCP/pkg/tools"
	"go.uber.org/zap"
)

// Handler handles HTTP requests for the MCP gateway
type Handler struct {
	logger            *zap.Logger
	validator         *mcp.Validator
	serviceDiscoverer grpc.ServiceDiscoverer
	sessionManager    *session.Manager
	toolBuilder       *tools.Builder
	headerFilter      *headers.Filter
}

// NewHandler creates a new HTTP handler
func NewHandler(
	logger *zap.Logger,
	serviceDiscoverer grpc.ServiceDiscoverer,
	sessionManager *session.Manager,
	toolBuilder *tools.Builder,
	headerConfig config.HeaderForwardingConfig,
) *Handler {
	return &Handler{
		logger:            logger,
		validator:         mcp.NewValidator(),
		serviceDiscoverer: serviceDiscoverer,
		sessionManager:    sessionManager,
		toolBuilder:       toolBuilder,
		headerFilter:      headers.NewFilter(headerConfig),
	}
}

// ServeHTTP handles HTTP requests
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleGet(w, r)
	case http.MethodPost:
		h.handlePost(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGet handles GET requests (for capability discovery)
func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	// Extract session information
	sessionID := r.Header.Get("Mcp-Session-Id")
	sessionCtx := h.sessionManager.GetOrCreateSession(sessionID, extractHeaders(r))

	// Set session header in response
	w.Header().Set("Mcp-Session-Id", sessionCtx.ID)

	// Handle initialization
	initResult := h.handleInitialize()
	response := &mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      mcp.RequestID{Value: 1},
		Result:  initResult,
	}

	h.writeJSONResponse(w, response)
}

// handlePost handles POST requests (JSON-RPC)
func (h *Handler) handlePost(w http.ResponseWriter, r *http.Request) {
	// Parse JSON-RPC request
	var req mcp.JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode JSON-RPC request", zap.Error(err))
		h.writeErrorResponse(w, mcp.RequestID{Value: nil}, mcp.ErrorCodeParseError, "Parse error")
		return
	}

	// Validate request
	if err := h.validator.ValidateRequest(&req); err != nil {
		h.logger.Error("Request validation failed", zap.Error(err))
		h.writeErrorResponse(w, req.ID, mcp.ErrorCodeInvalidRequest, mcp.SanitizeError(err))
		return
	}

	// Extract session information
	sessionID := r.Header.Get("Mcp-Session-Id")
	sessionCtx := h.sessionManager.GetOrCreateSession(sessionID, extractHeaders(r))

	// Set session header in response
	w.Header().Set("Mcp-Session-Id", sessionCtx.ID)

	// Log the request
	h.logger.Info("Processing MCP request",
		zap.String("method", req.Method),
		zap.String("sessionId", sessionCtx.ID),
		zap.Any("params", req.Params))

	// Handle the request
	result, err := h.handleRequest(r.Context(), &req, sessionCtx)
	if err != nil {
		h.logger.Error("Request handling failed",
			zap.String("method", req.Method),
			zap.Error(err))

		// Determine error code
		var errorCode int
		if strings.Contains(err.Error(), "not found") {
			errorCode = mcp.ErrorCodeMethodNotFound
		} else if strings.Contains(err.Error(), "invalid") {
			errorCode = mcp.ErrorCodeInvalidParams
		} else {
			errorCode = mcp.ErrorCodeInternalError
		}

		h.writeErrorResponse(w, req.ID, errorCode, mcp.SanitizeError(err))
		return
	}

	// Write successful response
	response := &mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}

	h.writeJSONResponse(w, response)
}

// handleRequest handles individual JSON-RPC requests
func (h *Handler) handleRequest(ctx context.Context, req *mcp.JSONRPCRequest, sessionCtx *session.Context) (interface{}, error) {
	switch req.Method {
	case "initialize":
		return h.handleInitialize(), nil
	case "tools/list":
		return h.handleToolsList(ctx)
	case "tools/call":
		return h.handleToolsCall(ctx, req.Params, sessionCtx)
	case "prompts/list":
		return h.handlePromptsList(ctx)
	case "resources/list":
		return h.handleResourcesList(ctx)
	default:
		return nil, fmt.Errorf("method not found: %s", req.Method)
	}
}

// handleInitialize handles the initialize method
func (h *Handler) handleInitialize() *mcp.InitializationResult {
	return &mcp.InitializationResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: mcp.ServerCapabilities{
			Tools: &mcp.ToolsCapability{
				ListChanged: false,
			},
			Prompts: &mcp.PromptsCapability{
				ListChanged: false,
			},
			Resources: &mcp.ResourcesCapability{
				ListChanged: false,
			},
		},
		ServerInfo: mcp.ServerInfo{
			Name:    "ggRMCP",
			Version: "1.0.0",
		},
	}
}

// handleToolsList handles the tools/list method
func (h *Handler) handleToolsList(ctx context.Context) (*mcp.ToolsListResult, error) {
	// Get discovered services
	services := h.serviceDiscoverer.GetServices()

	h.logger.Info("Processing services for tools list",
		zap.Int("serviceCount", len(services)))

	// Log discovered service names for debugging
	serviceNames := make([]string, 0, len(services))
	for serviceName := range services {
		serviceNames = append(serviceNames, serviceName)
	}
	h.logger.Debug("Discovered services", zap.Strings("services", serviceNames))

	// Build tools from discovered services (descriptions will be included if available)
	tools, err := h.toolBuilder.BuildTools(services)
	if err != nil {
		h.logger.Error("Failed to build tools", zap.Error(err))
		return nil, fmt.Errorf("failed to build tools: %w", err)
	}

	h.logger.Info("Generated tools list", zap.Int("toolCount", len(tools)))

	return &mcp.ToolsListResult{
		Tools: tools,
	}, nil
}

// handleToolsCall handles the tools/call method
func (h *Handler) handleToolsCall(ctx context.Context, params map[string]interface{}, sessionCtx *session.Context) (*mcp.ToolCallResult, error) {
	// Validate parameters
	if err := h.validator.ValidateToolCallParams(params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Extract tool name and arguments
	toolName := params["name"].(string)

	var argumentsJSON string
	if args, exists := params["arguments"]; exists && args != nil {
		argBytes, err := json.Marshal(args)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal arguments: %w", err)
		}
		argumentsJSON = string(argBytes)
	}

	// Parse tool name to extract service and method
	serviceName, methodName, err := h.parseToolName(toolName)
	if err != nil {
		h.logger.Error("Failed to parse tool name",
			zap.String("toolName", toolName),
			zap.Error(err))
		return nil, fmt.Errorf("invalid tool name: %w", err)
	}

	h.logger.Debug("Invoking tool",
		zap.String("toolName", toolName),
		zap.String("serviceName", serviceName),
		zap.String("methodName", methodName),
		zap.String("arguments", argumentsJSON),
		zap.String("sessionId", sessionCtx.ID))

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Filter headers for forwarding
	filteredHeaders := h.headerFilter.FilterHeaders(sessionCtx.Headers)

	h.logger.Debug("Filtered headers for forwarding",
		zap.String("toolName", toolName),
		zap.Any("originalHeaders", sessionCtx.Headers),
		zap.Any("filteredHeaders", filteredHeaders))

	// Invoke the gRPC method with filtered headers
	var result string
	if len(filteredHeaders) > 0 {
		result, err = h.serviceDiscoverer.InvokeMethodWithHeaders(ctx, filteredHeaders, serviceName, methodName, argumentsJSON)
	} else {
		result, err = h.serviceDiscoverer.InvokeMethod(ctx, serviceName, methodName, argumentsJSON)
	}
	if err != nil {
		return &mcp.ToolCallResult{
			Content: []mcp.ContentBlock{
				mcp.TextContent(fmt.Sprintf("Error invoking method: %s", mcp.SanitizeError(err))),
			},
			IsError: true,
		}, nil
	}

	// Update session context
	sessionCtx.IncrementCallCount()
	sessionCtx.UpdateLastAccessed()

	return &mcp.ToolCallResult{
		Content: []mcp.ContentBlock{
			mcp.TextContent(result),
		},
		IsError: false,
	}, nil
}

// parseToolName parses a tool name to extract service and method names
func (h *Handler) parseToolName(toolName string) (string, string, error) {
	// Tool names are generated by replacing dots with underscores in service names
	// and appending the lowercase method name
	// We need to find the matching service from our discovered services

	h.logger.Debug("Parsing tool name", zap.String("toolName", toolName))

	// Get all discovered services
	services := h.serviceDiscoverer.GetServices()

	// Try to match the tool name with service + method combinations
	for serviceName, service := range services {
		// Generate expected tool prefix for this service
		servicePart := strings.ToLower(strings.ReplaceAll(serviceName, ".", "_"))

		for _, method := range service.Methods {
			methodPart := strings.ToLower(method.Name)
			expectedToolName := fmt.Sprintf("%s_%s", servicePart, methodPart)

			if expectedToolName == toolName {
				h.logger.Debug("Found matching service and method",
					zap.String("serviceName", serviceName),
					zap.String("methodName", method.Name),
					zap.String("toolName", toolName))
				return serviceName, method.Name, nil
			}
		}
	}

	// If no exact match found, log available services for debugging
	h.logger.Error("No matching service/method found for tool",
		zap.String("toolName", toolName),
		zap.Strings("availableServices", func() []string {
			var names []string
			for name := range services {
				names = append(names, name)
			}
			return names
		}()))

	return "", "", fmt.Errorf("tool name %s does not match any discovered service method", toolName)
}

// handlePromptsList handles the prompts/list method
func (h *Handler) handlePromptsList(ctx context.Context) (interface{}, error) {
	// Return empty prompts list since this implementation focuses on tools
	return map[string]interface{}{
		"prompts": []interface{}{},
	}, nil
}

// handleResourcesList handles the resources/list method
func (h *Handler) handleResourcesList(ctx context.Context) (interface{}, error) {
	// Return empty resources list since this implementation focuses on tools
	return map[string]interface{}{
		"resources": []interface{}{},
	}, nil
}

// writeJSONResponse writes a JSON response
func (h *Handler) writeJSONResponse(w http.ResponseWriter, response interface{}) {
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode JSON response", zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// writeErrorResponse writes an error response
func (h *Handler) writeErrorResponse(w http.ResponseWriter, id mcp.RequestID, code int, message string) {
	response := &mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &mcp.RPCError{
			Code:    code,
			Message: message,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // JSON-RPC errors are still HTTP 200

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode error response", zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// extractHeaders extracts HTTP headers into a map
func extractHeaders(r *http.Request) map[string]string {
	headers := make(map[string]string)
	for name, values := range r.Header {
		if len(values) > 0 {
			headers[name] = values[0]
		}
	}
	return headers
}

// HealthHandler handles health check requests
func (h *Handler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Check gRPC connection health
	if err := h.serviceDiscoverer.HealthCheck(ctx); err != nil {
		h.logger.Error("Health check failed", zap.Error(err))
		http.Error(w, "Service unhealthy", http.StatusServiceUnavailable)
		return
	}

	// Check service discovery
	if h.serviceDiscoverer.GetServiceCount() == 0 {
		h.logger.Warn("No services discovered")
		http.Error(w, "No services available", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	healthInfo := map[string]interface{}{
		"status":       "healthy",
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
		"serviceCount": h.serviceDiscoverer.GetServiceCount(),
		"methodCount":  h.serviceDiscoverer.GetMethodCount(),
	}

	json.NewEncoder(w).Encode(healthInfo)
}

// MetricsHandler handles metrics requests (placeholder)
func (h *Handler) MetricsHandler(w http.ResponseWriter, r *http.Request) {
	stats := h.serviceDiscoverer.GetServiceStats()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(stats)
}

// HandleToolsCall handles tool calls directly (for testing)
func (h *Handler) HandleToolsCall(ctx context.Context, params map[string]interface{}, sessionCtx *session.Context) (*mcp.ToolCallResult, error) {
	return h.handleToolsCall(ctx, params, sessionCtx)
}

// GetServiceDiscoverer returns the service discoverer (for testing)
func (h *Handler) GetServiceDiscoverer() grpc.ServiceDiscoverer {
	return h.serviceDiscoverer
}
