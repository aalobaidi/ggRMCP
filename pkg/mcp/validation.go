package mcp

import (
	"fmt"
	"regexp"
	"strings"
)

// Validator provides validation functionality
type Validator struct {
	maxFieldLength int
	maxToolName    int
}

// NewValidator creates a new validator with default settings
func NewValidator() *Validator {
	return &Validator{
		maxFieldLength: 1024,
		maxToolName:    128,
	}
}

// ValidateRequest validates a JSON-RPC request
func (v *Validator) ValidateRequest(req *JSONRPCRequest) error {
	var errors ValidationErrors

	// Validate JSON-RPC version
	if req.JSONRPC != "2.0" {
		errors.Add("jsonrpc", "must be '2.0'")
	}

	// Validate method
	if req.Method == "" {
		errors.Add("method", "is required")
	} else if len(req.Method) > v.maxFieldLength {
		errors.Add("method", fmt.Sprintf("must be less than %d characters", v.maxFieldLength))
	}

	// Validate method name format
	if req.Method != "" && !isValidMethodName(req.Method) {
		errors.Add("method", "contains invalid characters")
	}

	// Validate ID
	if req.ID.Value == nil {
		errors.Add("id", "is required")
	}

	// Validate params if present
	if req.Params != nil {
		if err := v.validateParams(req.Params); err != nil {
			errors.Add("params", err.Error())
		}
	}

	if errors.HasErrors() {
		return errors
	}

	return nil
}

// ValidateTool validates a tool definition
func (v *Validator) ValidateTool(tool *Tool) error {
	var errors ValidationErrors

	// Validate name
	if tool.Name == "" {
		errors.Add("name", "is required")
	} else if len(tool.Name) > v.maxToolName {
		errors.Add("name", fmt.Sprintf("must be less than %d characters", v.maxToolName))
	} else if !isValidToolName(tool.Name) {
		errors.Add("name", "contains invalid characters")
	}

	// Validate description
	if tool.Description == "" {
		errors.Add("description", "is required")
	} else if len(tool.Description) > v.maxFieldLength {
		errors.Add("description", fmt.Sprintf("must be less than %d characters", v.maxFieldLength))
	}

	// Validate input schema
	if tool.InputSchema == nil {
		errors.Add("inputSchema", "is required")
	}

	if errors.HasErrors() {
		return errors
	}

	return nil
}

// ValidateToolCallParams validates tool call parameters
func (v *Validator) ValidateToolCallParams(params map[string]interface{}) error {
	var errors ValidationErrors

	// Validate tool name
	name, exists := params["name"]
	if !exists {
		errors.Add("name", "is required")
	} else if nameStr, ok := name.(string); !ok {
		errors.Add("name", "must be a string")
	} else if nameStr == "" {
		errors.Add("name", "cannot be empty")
	} else if len(nameStr) > v.maxToolName {
		errors.Add("name", fmt.Sprintf("must be less than %d characters", v.maxToolName))
	} else if !isValidToolName(nameStr) {
		errors.Add("name", "contains invalid characters")
	}

	// Validate arguments if present
	if args, exists := params["arguments"]; exists {
		if err := v.validateArguments(args); err != nil {
			errors.Add("arguments", err.Error())
		}
	}

	if errors.HasErrors() {
		return errors
	}

	return nil
}

// validateParams validates request parameters
func (v *Validator) validateParams(params map[string]interface{}) error {
	// Check for deeply nested objects
	if err := v.validateDepth(params, 0, 10); err != nil {
		return err
	}

	// Check for oversized parameters
	if err := v.validateSize(params); err != nil {
		return err
	}

	return nil
}

// validateArguments validates tool arguments
func (v *Validator) validateArguments(args interface{}) error {
	switch val := args.(type) {
	case map[string]interface{}:
		return v.validateParams(val)
	case []interface{}:
		for i, arg := range val {
			if err := v.validateArguments(arg); err != nil {
				return fmt.Errorf("argument[%d]: %v", i, err)
			}
		}
	case string:
		if len(val) > v.maxFieldLength {
			return fmt.Errorf("string too long (max %d)", v.maxFieldLength)
		}
	}

	return nil
}

// validateDepth validates object nesting depth
func (v *Validator) validateDepth(obj interface{}, depth, maxDepth int) error {
	if depth > maxDepth {
		return fmt.Errorf("object nesting too deep (max %d)", maxDepth)
	}

	switch val := obj.(type) {
	case map[string]interface{}:
		for _, value := range val {
			if err := v.validateDepth(value, depth+1, maxDepth); err != nil {
				return err
			}
		}
	case []interface{}:
		for _, value := range val {
			if err := v.validateDepth(value, depth+1, maxDepth); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateSize validates object size
func (v *Validator) validateSize(obj interface{}) error {
	size := calculateSize(obj)
	const maxSize = 1024 * 1024 // 1MB

	if size > maxSize {
		return fmt.Errorf("object too large (max %d bytes)", maxSize)
	}

	return nil
}

// calculateSize calculates approximate size of an object
func calculateSize(obj interface{}) int {
	switch v := obj.(type) {
	case string:
		return len(v)
	case map[string]interface{}:
		size := 0
		for key, value := range v {
			size += len(key) + calculateSize(value)
		}
		return size
	case []interface{}:
		size := 0
		for _, value := range v {
			size += calculateSize(value)
		}
		return size
	default:
		return 8 // approximate size for numbers, booleans, etc.
	}
}

// isValidMethodName checks if a method name is valid
func isValidMethodName(method string) bool {
	// Method names should contain only alphanumeric characters, underscores, and forward slashes
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_/]+$`, method)
	return matched
}

// isValidToolName checks if a tool name is valid
func isValidToolName(name string) bool {
	// Tool names should be alphanumeric with underscores and dots
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_\.]+$`, name)
	return matched
}

// SanitizeString sanitizes a string by removing/replacing dangerous characters
func SanitizeString(s string) string {
	// Remove control characters
	s = regexp.MustCompile(`[\x00-\x1F\x7F]`).ReplaceAllString(s, "")

	// Limit length
	if len(s) > 1024 {
		s = s[:1024]
	}

	return strings.TrimSpace(s)
}

// SanitizeError sanitizes error messages to prevent information disclosure
func SanitizeError(err error) string {
	if err == nil {
		return ""
	}

	msg := err.Error()

	// Remove sensitive patterns
	sensitive := []string{
		"password",
		"token",
		"key",
		"secret",
		"credential",
		"auth",
	}

	for _, pattern := range sensitive {
		re := regexp.MustCompile(`(?i)` + pattern + `[^\s]*`)
		msg = re.ReplaceAllString(msg, "[REDACTED]")
	}

	return SanitizeString(msg)
}
