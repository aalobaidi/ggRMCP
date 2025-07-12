package tools

import (
	"fmt"
	"strings"

	"github.com/aalobaidi/ggRMCP/pkg/grpc"
	"github.com/aalobaidi/ggRMCP/pkg/mcp"
	"go.uber.org/zap"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Builder builds MCP tools from gRPC service definitions
type Builder struct {
	logger *zap.Logger

	// Cache for generated schemas
	schemaCache map[string]interface{}

	// Configuration
	maxRecursionDepth int
	includeComments   bool
}

// NewBuilder creates a new tool builder
func NewBuilder(logger *zap.Logger) *Builder {
	return &Builder{
		logger:            logger,
		schemaCache:       make(map[string]interface{}),
		maxRecursionDepth: 10,
		includeComments:   true,
	}
}

// BuildTool builds an MCP tool from a gRPC method
func (b *Builder) BuildTool(serviceName string, method grpc.MethodInfo) (mcp.Tool, error) {
	// Generate tool name
	toolName := b.generateToolName(serviceName, method.Name)

	// Generate description
	description := b.generateDescription(serviceName, method)

	// Generate input schema
	b.logger.Debug("Generating input schema",
		zap.String("toolName", toolName),
		zap.String("inputType", string(method.InputDescriptor.FullName())))

	inputSchema, err := b.generateMessageSchema(method.InputDescriptor, make(map[string]bool))
	if err != nil {
		b.logger.Error("Failed to generate input schema",
			zap.String("toolName", toolName),
			zap.String("inputType", string(method.InputDescriptor.FullName())),
			zap.Error(err))
		return mcp.Tool{}, fmt.Errorf("failed to generate input schema: %w", err)
	}

	// Generate output schema
	b.logger.Debug("Generating output schema",
		zap.String("toolName", toolName),
		zap.String("outputType", string(method.OutputDescriptor.FullName())))

	outputSchema, err := b.generateMessageSchema(method.OutputDescriptor, make(map[string]bool))
	if err != nil {
		b.logger.Error("Failed to generate output schema",
			zap.String("toolName", toolName),
			zap.String("outputType", string(method.OutputDescriptor.FullName())),
			zap.Error(err))
		return mcp.Tool{}, fmt.Errorf("failed to generate output schema: %w", err)
	}

	tool := mcp.Tool{
		Name:         toolName,
		Description:  description,
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
	}

	// Validate the tool
	if err := b.validateTool(tool); err != nil {
		return mcp.Tool{}, fmt.Errorf("tool validation failed: %w", err)
	}

	b.logger.Debug("Built tool",
		zap.String("toolName", toolName),
		zap.String("service", serviceName),
		zap.String("method", method.Name))

	return tool, nil
}

// generateToolName generates a tool name from service and method names
func (b *Builder) generateToolName(serviceName, methodName string) string {
	// Convert service name to lowercase and replace dots with underscores
	servicePart := strings.ToLower(strings.ReplaceAll(serviceName, ".", "_"))

	// Convert method name to lowercase
	methodPart := strings.ToLower(methodName)

	return fmt.Sprintf("%s_%s", servicePart, methodPart)
}

// generateDescription generates a tool description
func (b *Builder) generateDescription(serviceName string, method grpc.MethodInfo) string {
	// Use description from method if available (could be from FileDescriptorSet comments)
	if method.Description != "" {
		return method.Description
	}

	// Fallback to generic description
	return fmt.Sprintf("Calls the %s method of the %s service", method.Name, serviceName)
}

// generateMessageSchema generates a JSON schema from a protobuf message descriptor
func (b *Builder) generateMessageSchema(desc protoreflect.MessageDescriptor, visited map[string]bool) (interface{}, error) {
	if desc == nil {
		return nil, fmt.Errorf("nil message descriptor")
	}

	// Check for circular references
	fullName := string(desc.FullName())
	if visited[fullName] {
		// Return a reference to break the cycle
		b.logger.Debug("Found circular reference, using $ref",
			zap.String("messageType", fullName))
		return map[string]interface{}{
			"$ref": "#/definitions/" + fullName,
		}, nil
	}
	visited[fullName] = true

	b.logger.Debug("Generating schema for message",
		zap.String("messageType", fullName),
		zap.Int("fieldCount", desc.Fields().Len()))

	// Check cache
	if cached, exists := b.schemaCache[fullName]; exists {
		return cached, nil
	}

	schema := map[string]interface{}{
		"type":       "object",
		"properties": make(map[string]interface{}),
	}

	// Add required fields
	var required []string

	// Process fields
	properties := schema["properties"].(map[string]interface{})
	fields := desc.Fields()

	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		fieldName := string(field.Name())

		fieldSchema, err := b.generateFieldSchema(field, visited)
		if err != nil {
			return nil, fmt.Errorf("failed to generate schema for field %s: %w", fieldName, err)
		}

		properties[fieldName] = fieldSchema

		// Add to required if not optional
		if !field.HasOptionalKeyword() && field.Cardinality() == protoreflect.Required {
			required = append(required, fieldName)
		}
	}

	// Handle oneofs
	oneofs := desc.Oneofs()
	for i := 0; i < oneofs.Len(); i++ {
		oneof := oneofs.Get(i)
		oneofSchema, err := b.generateOneofSchema(oneof, visited)
		if err != nil {
			return nil, fmt.Errorf("failed to generate schema for oneof %s: %w", oneof.Name(), err)
		}

		if oneofSchema != nil {
			// Add oneof as a property that has multiple possible schemas
			properties[string(oneof.Name())] = oneofSchema
		}
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	// Cache the schema
	b.schemaCache[fullName] = schema

	return schema, nil
}

// generateFieldSchema generates a JSON schema for a protobuf field
func (b *Builder) generateFieldSchema(field protoreflect.FieldDescriptor, visited map[string]bool) (interface{}, error) {
	var baseSchema interface{}
	var err error

	switch field.Kind() {
	case protoreflect.BoolKind:
		baseSchema = map[string]interface{}{
			"type": "boolean",
		}
	case protoreflect.Int32Kind, protoreflect.Int64Kind,
		protoreflect.Sint32Kind, protoreflect.Sint64Kind,
		protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind:
		baseSchema = map[string]interface{}{
			"type": "integer",
		}
	case protoreflect.Uint32Kind, protoreflect.Uint64Kind,
		protoreflect.Fixed32Kind, protoreflect.Fixed64Kind:
		baseSchema = map[string]interface{}{
			"type":    "integer",
			"minimum": 0,
		}
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		baseSchema = map[string]interface{}{
			"type": "number",
		}
	case protoreflect.StringKind:
		baseSchema = map[string]interface{}{
			"type": "string",
		}
	case protoreflect.BytesKind:
		baseSchema = map[string]interface{}{
			"type":   "string",
			"format": "byte",
		}
	case protoreflect.EnumKind:
		baseSchema, err = b.generateEnumSchema(field.Enum())
		if err != nil {
			return nil, err
		}
	case protoreflect.MessageKind:
		baseSchema, err = b.generateMessageSchema(field.Message(), visited)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported field kind: %v", field.Kind())
	}

	// Handle repeated fields
	if field.IsList() {
		return map[string]interface{}{
			"type":  "array",
			"items": baseSchema,
		}, nil
	}

	// Handle map fields
	if field.IsMap() {
		_, err := b.generateFieldSchema(field.MapKey(), visited)
		if err != nil {
			return nil, fmt.Errorf("failed to generate map key schema: %w", err)
		}

		valueSchema, err := b.generateFieldSchema(field.MapValue(), visited)
		if err != nil {
			return nil, fmt.Errorf("failed to generate map value schema: %w", err)
		}

		return map[string]interface{}{
			"type": "object",
			"patternProperties": map[string]interface{}{
				".*": valueSchema,
			},
			"additionalProperties": false,
		}, nil
	}

	// Add field description if available
	if schema, ok := baseSchema.(map[string]interface{}); ok {
		if description := b.GetFieldDescription(field); description != "" {
			schema["description"] = description
		}
	}

	return baseSchema, nil
}

// generateEnumSchema generates a JSON schema for an enum
func (b *Builder) generateEnumSchema(enum protoreflect.EnumDescriptor) (interface{}, error) {
	values := enum.Values()
	var enumValues []interface{}

	for i := 0; i < values.Len(); i++ {
		value := values.Get(i)
		enumValues = append(enumValues, string(value.Name()))
	}

	return map[string]interface{}{
		"type": "string",
		"enum": enumValues,
	}, nil
}

// generateOneofSchema generates a JSON schema for a oneof field
func (b *Builder) generateOneofSchema(oneof protoreflect.OneofDescriptor, visited map[string]bool) (interface{}, error) {
	fields := oneof.Fields()
	if fields.Len() == 0 {
		return nil, nil
	}

	var oneOfOptions []interface{}

	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		fieldName := string(field.Name())

		fieldSchema, err := b.generateFieldSchema(field, visited)
		if err != nil {
			return nil, fmt.Errorf("failed to generate schema for oneof field %s: %w", fieldName, err)
		}

		// Wrap in an object with the field name as key
		properties := map[string]interface{}{
			fieldName: fieldSchema,
		}

		oneOfOption := map[string]interface{}{
			"type":       "object",
			"properties": properties,
			"required":   []string{fieldName},
		}

		oneOfOptions = append(oneOfOptions, oneOfOption)
	}

	return map[string]interface{}{
		"oneOf": oneOfOptions,
	}, nil
}

// GetFieldDescription extracts field description from comments
func (b *Builder) GetFieldDescription(field protoreflect.FieldDescriptor) string {
	// Extract comments from the field descriptor
	loc := field.ParentFile().SourceLocations().ByDescriptor(field)

	var comments string

	// Leading comments
	if leading := loc.LeadingComments; leading != "" {
		comments = leading
	}

	// Trailing comments (append with newline if we have leading comments)
	if trailing := loc.TrailingComments; trailing != "" {
		if comments != "" {
			comments += "\n" + trailing
		} else {
			comments = trailing
		}
	}

	// Return the comment if found, otherwise fallback to generic description
	if comments != "" {
		return strings.TrimSpace(comments)
	}

	return fmt.Sprintf("Field %s", field.Name())
}

// validateTool validates a generated tool
func (b *Builder) validateTool(tool mcp.Tool) error {
	if tool.Name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	if tool.Description == "" {
		return fmt.Errorf("tool description cannot be empty")
	}

	if tool.InputSchema == nil {
		return fmt.Errorf("tool input schema cannot be nil")
	}

	// Validate that the name follows the expected pattern
	if !strings.Contains(tool.Name, "_") {
		return fmt.Errorf("tool name must contain underscore separator")
	}

	return nil
}

// ClearCache clears the schema cache
func (b *Builder) ClearCache() {
	b.schemaCache = make(map[string]interface{})
}

// GetCacheSize returns the size of the schema cache
func (b *Builder) GetCacheSize() int {
	return len(b.schemaCache)
}

// SetMaxRecursionDepth sets the maximum recursion depth for schema generation
func (b *Builder) SetMaxRecursionDepth(depth int) {
	b.maxRecursionDepth = depth
}

// SetIncludeComments sets whether to include comments in generated schemas
func (b *Builder) SetIncludeComments(include bool) {
	b.includeComments = include
}

// BuildTools builds MCP tools for all methods in all services
func (b *Builder) BuildTools(services map[string]grpc.ServiceInfo) ([]mcp.Tool, error) {
	var tools []mcp.Tool

	for serviceName, service := range services {
		for _, method := range service.Methods {
			// Skip streaming methods
			if method.IsClientStreaming || method.IsServerStreaming {
				b.logger.Debug("Skipping streaming method",
					zap.String("service", serviceName),
					zap.String("method", method.Name))
				continue
			}

			tool, err := b.BuildTool(serviceName, method)
			if err != nil {
				b.logger.Error("Failed to build tool",
					zap.String("service", serviceName),
					zap.String("method", method.Name),
					zap.Error(err))
				continue
			}

			tools = append(tools, tool)
		}
	}

	b.logger.Info("Built tools", zap.Int("count", len(tools)))
	return tools, nil
}

// GetStats returns builder statistics
func (b *Builder) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"cache_size":          b.GetCacheSize(),
		"max_recursion_depth": b.maxRecursionDepth,
		"include_comments":    b.includeComments,
	}
}
