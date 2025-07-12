package mcp

import (
	"encoding/json"
	"fmt"
)

// RequestID can be either a string or a number
type RequestID struct {
	Value interface{}
}

// MarshalJSON implements json.Marshaler
func (r RequestID) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.Value)
}

// UnmarshalJSON implements json.Unmarshaler
func (r *RequestID) UnmarshalJSON(data []byte) error {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	switch v := v.(type) {
	case string, float64:
		r.Value = v
	default:
		return fmt.Errorf("invalid request ID type: %T", v)
	}

	return nil
}

// String returns string representation of RequestID
func (r RequestID) String() string {
	return fmt.Sprintf("%v", r.Value)
}

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
	ID      RequestID              `json:"id"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
	ID      RequestID   `json:"id"`
}

// RPCError represents a JSON-RPC 2.0 error
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Error implements the error interface
func (e *RPCError) Error() string {
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

// Common JSON-RPC error codes
const (
	ErrorCodeParseError     = -32700
	ErrorCodeInvalidRequest = -32600
	ErrorCodeMethodNotFound = -32601
	ErrorCodeInvalidParams  = -32602
	ErrorCodeInternalError  = -32603
)

// ServerInfo represents the server information
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ClientInfo represents the client information
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ServerCapabilities represents server capabilities
type ServerCapabilities struct {
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
}

// ToolsCapability represents tools capability
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// PromptsCapability represents prompts capability
type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapability represents resources capability
type ResourcesCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// InitializationResult represents the initialization result
type InitializationResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

// ContentType represents different content types
type ContentType string

const (
	ContentTypeText  ContentType = "text"
	ContentTypeImage ContentType = "image"
	ContentTypeAudio ContentType = "audio"
)

// ContentBlock represents a content block
type ContentBlock struct {
	Type     ContentType `json:"type"`
	Text     string      `json:"text,omitempty"`
	Data     string      `json:"data,omitempty"`
	MimeType string      `json:"mimeType,omitempty"`
}

// TextContent creates a text content block
func TextContent(text string) ContentBlock {
	return ContentBlock{
		Type: ContentTypeText,
		Text: text,
	}
}

// ImageContent creates an image content block
func ImageContent(data, mimeType string) ContentBlock {
	return ContentBlock{
		Type:     ContentTypeImage,
		Data:     data,
		MimeType: mimeType,
	}
}

// AudioContent creates an audio content block
func AudioContent(data, mimeType string) ContentBlock {
	return ContentBlock{
		Type:     ContentTypeAudio,
		Data:     data,
		MimeType: mimeType,
	}
}

// ToolCallResult represents the result of a tool call
type ToolCallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// Tool represents an MCP tool
type Tool struct {
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	InputSchema  interface{} `json:"inputSchema"`
	OutputSchema interface{} `json:"outputSchema,omitempty"`
}

// ToolsListResult represents the result of listing tools
type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

// Role represents different roles in MCP
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

// Annotation represents an annotation
type Annotation struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Audience string `json:"audience,omitempty"`
	Priority string `json:"priority,omitempty"`
}

// AnnotationType represents different annotation types
type AnnotationType string

const (
	AnnotationTypeInfo    AnnotationType = "info"
	AnnotationTypeWarning AnnotationType = "warning"
	AnnotationTypeError   AnnotationType = "error"
)

// Priority represents annotation priority
type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
)

// Audience represents annotation audience
type Audience string

const (
	AudienceUser      Audience = "user"
	AudienceAssistant Audience = "assistant"
)

// CreateAnnotation creates a new annotation
func CreateAnnotation(annoType AnnotationType, text string, audience Audience, priority Priority) Annotation {
	return Annotation{
		Type:     string(annoType),
		Text:     text,
		Audience: string(audience),
		Priority: string(priority),
	}
}

// ResourceContents represents resource contents
type ResourceContents struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
}

// ResourceLink represents a resource link
type ResourceLink struct {
	URI         string `json:"uri"`
	Description string `json:"description,omitempty"`
}

// EmbeddedResource represents an embedded resource
type EmbeddedResource struct {
	Type     string           `json:"type"`
	Resource ResourceContents `json:"resource"`
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error implements the error interface
func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s': %s", e.Field, e.Message)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

// Error implements the error interface
func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "validation errors"
	}
	return fmt.Sprintf("validation errors: %s", e[0].Message)
}

// Add adds a validation error
func (e *ValidationErrors) Add(field, message string) {
	*e = append(*e, ValidationError{Field: field, Message: message})
}

// HasErrors returns true if there are validation errors
func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}
