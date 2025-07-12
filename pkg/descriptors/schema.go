package descriptors

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// SchemaExtractor generates JSON schemas with comments from protobuf descriptors
type SchemaExtractor struct {
	logger *zap.Logger
	files  *protoregistry.Files
}

// NewSchemaExtractor creates a new schema extractor
func NewSchemaExtractor(logger *zap.Logger, files *protoregistry.Files) *SchemaExtractor {
	return &SchemaExtractor{
		logger: logger.Named("schema"),
		files:  files,
	}
}

// ExtractMessageSchema generates a JSON schema for a message with comments
func (e *SchemaExtractor) ExtractMessageSchema(msgDesc protoreflect.MessageDescriptor) (map[string]interface{}, error) {
	// Use internal method with visited tracking
	return e.extractMessageSchemaInternal(msgDesc, make(map[string]bool))
}

// extractMessageSchemaInternal generates a JSON schema with circular reference detection
func (e *SchemaExtractor) extractMessageSchemaInternal(msgDesc protoreflect.MessageDescriptor, visited map[string]bool) (map[string]interface{}, error) {
	// Check for circular references
	fullName := string(msgDesc.FullName())
	if visited[fullName] {
		// Return a reference to break the cycle
		e.logger.Debug("Found circular reference, using $ref",
			zap.String("messageType", fullName))
		return map[string]interface{}{
			"$ref": "#/definitions/" + fullName,
		}, nil
	}
	visited[fullName] = true
	defer func() { delete(visited, fullName) }() // Clean up on exit

	schema := map[string]interface{}{
		"type":       "object",
		"properties": make(map[string]interface{}),
	}

	// Add message-level description if available
	if desc := extractComments(msgDesc); desc != "" {
		schema["description"] = desc
	}

	required := []string{}
	properties := schema["properties"].(map[string]interface{})

	// Process each field
	for i := 0; i < msgDesc.Fields().Len(); i++ {
		field := msgDesc.Fields().Get(i)
		fieldName := string(field.Name())

		fieldSchema, err := e.extractFieldSchemaInternal(field, visited)
		if err != nil {
			e.logger.Warn("Failed to extract field schema",
				zap.String("message", string(msgDesc.FullName())),
				zap.String("field", fieldName),
				zap.Error(err))
			continue
		}

		properties[fieldName] = fieldSchema

		// Add to required if field is required (not optional)
		if field.HasOptionalKeyword() || field.HasPresence() {
			// Field is optional
		} else {
			required = append(required, fieldName)
		}
	}

	// Process oneofs
	for i := 0; i < msgDesc.Oneofs().Len(); i++ {
		oneof := msgDesc.Oneofs().Get(i)
		oneofName := string(oneof.Name())

		oneofSchema := map[string]interface{}{
			"type":  "object",
			"oneOf": []map[string]interface{}{},
		}

		// Add oneof description if available
		if desc := extractComments(oneof); desc != "" {
			oneofSchema["description"] = desc
		}

		// Process oneof fields
		for j := 0; j < oneof.Fields().Len(); j++ {
			field := oneof.Fields().Get(j)
			fieldName := string(field.Name())

			fieldSchema, err := e.extractFieldSchemaInternal(field, visited)
			if err != nil {
				e.logger.Warn("Failed to extract field schema for oneof",
					zap.String("field", fieldName),
					zap.Error(err))
				continue
			}

			oneofOption := map[string]interface{}{
				"properties": map[string]interface{}{
					fieldName: fieldSchema,
				},
				"required": []string{fieldName},
			}

			oneofSchema["oneOf"] = append(oneofSchema["oneOf"].([]map[string]interface{}), oneofOption)
		}

		properties[oneofName] = oneofSchema
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	return schema, nil
}

// extractFieldSchema generates schema for a single field
func (e *SchemaExtractor) extractFieldSchema(field protoreflect.FieldDescriptor) (map[string]interface{}, error) {
	// Use internal method with visited tracking
	return e.extractFieldSchemaInternal(field, make(map[string]bool))
}

// extractFieldSchemaInternal generates schema for a single field with circular reference detection
func (e *SchemaExtractor) extractFieldSchemaInternal(field protoreflect.FieldDescriptor, visited map[string]bool) (map[string]interface{}, error) {
	schema := make(map[string]interface{})

	// Add field description if available
	if desc := extractComments(field); desc != "" {
		schema["description"] = desc
	}

	// Handle repeated fields
	if field.IsList() {
		itemSchema, err := e.extractFieldTypeSchemaInternal(field, visited)
		if err != nil {
			return nil, err
		}

		schema["type"] = "array"
		schema["items"] = itemSchema
		return schema, nil
	}

	// Handle map fields
	if field.IsMap() {
		valueField := field.MapValue()
		valueSchema, err := e.extractFieldTypeSchemaInternal(valueField, visited)
		if err != nil {
			return nil, err
		}

		schema["type"] = "object"
		schema["additionalProperties"] = valueSchema
		return schema, nil
	}

	// Handle regular fields
	return e.extractFieldTypeSchemaInternal(field, visited)
}

// extractFieldTypeSchema generates schema for the field's type
func (e *SchemaExtractor) extractFieldTypeSchema(field protoreflect.FieldDescriptor) (map[string]interface{}, error) {
	// Use internal method with visited tracking
	return e.extractFieldTypeSchemaInternal(field, make(map[string]bool))
}

// extractFieldTypeSchemaInternal generates schema for the field's type with circular reference detection
func (e *SchemaExtractor) extractFieldTypeSchemaInternal(field protoreflect.FieldDescriptor, visited map[string]bool) (map[string]interface{}, error) {
	schema := make(map[string]interface{})

	switch field.Kind() {
	case protoreflect.BoolKind:
		schema["type"] = "boolean"

	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		schema["type"] = "integer"
		schema["format"] = "int32"

	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		schema["type"] = "integer"
		schema["format"] = "int64"

	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		schema["type"] = "integer"
		schema["format"] = "uint32"
		schema["minimum"] = 0

	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		schema["type"] = "integer"
		schema["format"] = "uint64"
		schema["minimum"] = 0

	case protoreflect.FloatKind:
		schema["type"] = "number"
		schema["format"] = "float"

	case protoreflect.DoubleKind:
		schema["type"] = "number"
		schema["format"] = "double"

	case protoreflect.StringKind:
		schema["type"] = "string"

	case protoreflect.BytesKind:
		schema["type"] = "string"
		schema["format"] = "byte"

	case protoreflect.EnumKind:
		enumDesc := field.Enum()
		enumValues := []string{}
		enumDescriptions := make(map[string]string)

		for i := 0; i < enumDesc.Values().Len(); i++ {
			enumValue := enumDesc.Values().Get(i)
			valueName := string(enumValue.Name())
			enumValues = append(enumValues, valueName)

			// Add enum value description if available
			if desc := extractComments(enumValue); desc != "" {
				enumDescriptions[valueName] = desc
			}
		}

		schema["type"] = "string"
		schema["enum"] = enumValues

		// Add enum description if available
		if desc := extractComments(enumDesc); desc != "" {
			schema["description"] = desc
		}

		// Add enum value descriptions
		if len(enumDescriptions) > 0 {
			schema["enumDescriptions"] = enumDescriptions
		}

	case protoreflect.MessageKind:
		msgDesc := field.Message()

		// Handle well-known types
		switch msgDesc.FullName() {
		case "google.protobuf.Any":
			schema["type"] = "object"
			schema["description"] = "Any contains an arbitrary serialized protocol buffer message"

		case "google.protobuf.Timestamp":
			schema["type"] = "string"
			schema["format"] = "date-time"
			schema["description"] = "RFC 3339 formatted timestamp"

		case "google.protobuf.Duration":
			schema["type"] = "string"
			schema["format"] = "duration"
			schema["description"] = "Duration in seconds with up to 9 fractional digits"

		case "google.protobuf.Struct":
			schema["type"] = "object"
			schema["description"] = "Arbitrary JSON-like structure"

		case "google.protobuf.Value":
			schema["description"] = "Any JSON value"

		case "google.protobuf.ListValue":
			schema["type"] = "array"
			schema["description"] = "Array of JSON values"

		case "google.protobuf.StringValue",
			"google.protobuf.BytesValue":
			schema["type"] = "string"

		case "google.protobuf.BoolValue":
			schema["type"] = "boolean"

		case "google.protobuf.Int32Value",
			"google.protobuf.UInt32Value",
			"google.protobuf.Int64Value",
			"google.protobuf.UInt64Value":
			schema["type"] = "integer"

		case "google.protobuf.FloatValue",
			"google.protobuf.DoubleValue":
			schema["type"] = "number"

		default:
			// Custom message type - extract schema recursively
			messageSchema, err := e.extractMessageSchemaInternal(msgDesc, visited)
			if err != nil {
				return nil, fmt.Errorf("failed to extract schema for message %s: %w", msgDesc.FullName(), err)
			}
			return messageSchema, nil
		}

	default:
		return nil, fmt.Errorf("unsupported field kind: %v", field.Kind())
	}

	return schema, nil
}

// ExtractEnhancedToolSchema generates an enhanced tool schema with descriptions
func (e *SchemaExtractor) ExtractEnhancedToolSchema(serviceName string, method EnhancedMethodInfo) (map[string]interface{}, error) {
	// Generate input schema
	inputSchema, err := e.ExtractMessageSchema(method.InputDescriptor)
	if err != nil {
		return nil, fmt.Errorf("failed to extract input schema: %w", err)
	}

	// Create tool schema
	toolSchema := map[string]interface{}{
		"name": generateToolName(serviceName, method.Name),
		"inputSchema": map[string]interface{}{
			"type":       "object",
			"properties": inputSchema["properties"],
		},
	}

	// Add description from method comments
	description := method.Description
	if description == "" {
		description = fmt.Sprintf("Calls %s.%s gRPC method", serviceName, method.Name)
	}
	toolSchema["description"] = description

	// Add required fields if any
	if required, ok := inputSchema["required"]; ok {
		toolSchema["inputSchema"].(map[string]interface{})["required"] = required
	}

	return toolSchema, nil
}

// generateToolName creates a tool name from service and method names
func generateToolName(serviceName, methodName string) string {
	// Convert service name: com.example.Service -> com_example_service
	servicePart := strings.ToLower(strings.ReplaceAll(serviceName, ".", "_"))
	methodPart := strings.ToLower(methodName)
	return fmt.Sprintf("%s_%s", servicePart, methodPart)
}
