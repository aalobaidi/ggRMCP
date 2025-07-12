//go:build integratio
// +build integratio

package tests

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/aalobaidi/ggRMCP/pkg/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComplexServiceDiscovery(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	services := DiscoverServices(t, env)

	// Should discover 4 services (Hello + 3 complex services)
	assert.Equal(t, 4, len(services))

	// Check expected services
	expectedServices := map[string][]string{
		"hello.HelloService":                     {"SayHello"},
		"com.example.complex.UserProfileService": {"GetUserProfile"},
		"com.example.complex.DocumentService":    {"CreateDocument"},
		"com.example.complex.NodeService":        {"ProcessNode"},
	}

	for serviceName, expectedMethods := range expectedServices {
		AssertServiceExists(t, services, serviceName, expectedMethods)
	}
}

func TestComplexServiceMCPToolGeneration(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	tools := BuildAllTools(t, env)

	// Should have 4 tools (one for each method)
	assert.Equal(t, 4, len(tools))

	// Verify expected tool names
	expectedToolNames := []string{
		"hello_helloservice_sayhello",
		"com_example_complex_userprofileservice_getuserprofile",
		"com_example_complex_documentservice_createdocument",
		"com_example_complex_nodeservice_processnode",
	}

	for _, expectedName := range expectedToolNames {
		AssertToolExists(t, tools, expectedName)
	}
}

func TestUserProfileServiceSchemaGeneration(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	service, exists := GetServiceByName(env, "com.example.complex.UserProfileService")
	require.True(t, exists)
	require.Equal(t, 1, len(service.Methods))

	tool, err := env.ToolBuilder.BuildTool(service.Name, service.Methods[0])
	require.NoError(t, err)

	// Verify basic tool structure
	assert.Equal(t, "com_example_complex_userprofileservice_getuserprofile", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, tool.InputSchema)
	assert.NotNil(t, tool.OutputSchema)

	// Validate schemas
	ValidateJSONSchema(t, tool.InputSchema)
	ValidateJSONSchema(t, tool.OutputSchema)

	// Verify input schema structure
	inputSchema := tool.InputSchema.(map[string]interface{})
	assert.Equal(t, "object", inputSchema["type"])

	// Check user_id field exists
	AssertSchemaField(t, inputSchema, []string{"properties", "user_id"}, "string")

	// Verify output schema structure
	outputSchema := tool.OutputSchema.(map[string]interface{})
	assert.Equal(t, "object", outputSchema["type"])

	// Check profile field and its nested structure
	profileField := AssertSchemaField(t, outputSchema, []string{"properties", "profile"}, "object")

	// Verify profile nested fields
	AssertSchemaField(t, profileField, []string{"properties", "user_id"}, "string")
	AssertSchemaField(t, profileField, []string{"properties", "display_name"}, "string")
	AssertSchemaField(t, profileField, []string{"properties", "email"}, "string")

	// Check enum field
	userTypeField := AssertSchemaField(t, profileField, []string{"properties", "user_type"}, "string")
	enumValues, ok := userTypeField["enum"].([]interface{})
	require.True(t, ok)
	assert.Contains(t, enumValues, "USER_TYPE_UNSPECIFIED")
	assert.Contains(t, enumValues, "STANDARD")
	assert.Contains(t, enumValues, "PREMIUM")
	assert.Contains(t, enumValues, "ADMIN")

	// Check timestamp field
	AssertSchemaField(t, profileField, []string{"properties", "last_login"}, "object")
}

func TestDocumentServiceOneofSchemaGeneration(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	service, exists := GetServiceByName(env, "com.example.complex.DocumentService")
	require.True(t, exists)
	require.Equal(t, 1, len(service.Methods))

	tool, err := env.ToolBuilder.BuildTool(service.Name, service.Methods[0])
	require.NoError(t, err)

	// Verify input schema has document field
	inputSchema := tool.InputSchema.(map[string]interface{})
	documentField := AssertSchemaField(t, inputSchema, []string{"properties", "document"}, "object")

	// Check basic document fields
	documentProperties, ok := documentField["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, documentProperties, "document_id")
	assert.Contains(t, documentProperties, "title")
	assert.Contains(t, documentProperties, "content")

	// Check oneof field (metadata)
	metadataField, ok := documentProperties["metadata"].(map[string]interface{})
	require.True(t, ok)

	// Should have oneOf structure
	oneOfOptions, ok := metadataField["oneOf"].([]interface{})
	require.True(t, ok)
	assert.Equal(t, 2, len(oneOfOptions)) // simple_summary and structured_metadata_wrapper

	// Verify options have expected structure
	var hasSimpleSummary, hasStructuredMetadata bool
	for _, option := range oneOfOptions {
		optionMap, ok := option.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "object", optionMap["type"])

		optionProperties, ok := optionMap["properties"].(map[string]interface{})
		require.True(t, ok)

		if _, exists := optionProperties["simple_summary"]; exists {
			hasSimpleSummary = true
		}
		if _, exists := optionProperties["structured_metadata_wrapper"]; exists {
			hasStructuredMetadata = true
		}
	}

	assert.True(t, hasSimpleSummary)
	assert.True(t, hasStructuredMetadata)
}

func TestNodeServiceRecursiveSchemaGeneration(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	service, exists := GetServiceByName(env, "com.example.complex.NodeService")
	require.True(t, exists)
	require.Equal(t, 1, len(service.Methods))

	tool, err := env.ToolBuilder.BuildTool(service.Name, service.Methods[0])
	require.NoError(t, err)

	// Verify input schema has root_node field
	inputSchema := tool.InputSchema.(map[string]interface{})
	rootNodeField := AssertSchemaField(t, inputSchema, []string{"properties", "root_node"}, "object")

	// Check basic node fields
	AssertSchemaField(t, rootNodeField, []string{"properties", "id"}, "string")
	AssertSchemaField(t, rootNodeField, []string{"properties", "value"}, "string")

	// Check recursive children field
	childrenField := AssertSchemaField(t, rootNodeField, []string{"properties", "children"}, "array")

	// Items should be a reference or object to handle recursion
	items, ok := childrenField["items"].(map[string]interface{})
	require.True(t, ok)

	// Should either be a reference or an object type
	if refValue, hasRef := items["$ref"]; hasRef {
		assert.Contains(t, refValue, "Node")
	} else {
		assert.Equal(t, "object", items["type"])
	}
}

func TestStructuredMetadataMapGeneration(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	service, exists := GetServiceByName(env, "com.example.complex.DocumentService")
	require.True(t, exists)

	tool, err := env.ToolBuilder.BuildTool(service.Name, service.Methods[0])
	require.NoError(t, err)

	// Debug: Print the schema for analysis
	schemaJSON, _ := json.MarshalIndent(tool.InputSchema, "", "  ")
	t.Logf("Input schema: %s", string(schemaJSON))

	// Navigate to metadata field and verify oneof structure
	inputSchema := tool.InputSchema.(map[string]interface{})
	documentField := AssertSchemaField(t, inputSchema, []string{"properties", "document"}, "object")

	documentProperties, ok := documentField["properties"].(map[string]interface{})
	require.True(t, ok)

	metadataField, ok := documentProperties["metadata"].(map[string]interface{})
	require.True(t, ok)

	oneOfOptions, ok := metadataField["oneOf"].([]interface{})
	require.True(t, ok)
	assert.Equal(t, 2, len(oneOfOptions))

	// Verify both options exist
	var hasSimpleSummary, hasStructuredMetadata bool
	for _, option := range oneOfOptions {
		optionMap, ok := option.(map[string]interface{})
		require.True(t, ok)

		optionProperties, ok := optionMap["properties"].(map[string]interface{})
		require.True(t, ok)

		if _, exists := optionProperties["simple_summary"]; exists {
			hasSimpleSummary = true
		}
		if _, exists := optionProperties["structured_metadata_wrapper"]; exists {
			hasStructuredMetadata = true
		}
	}

	assert.True(t, hasSimpleSummary, "Should have simple_summary option")
	assert.True(t, hasStructuredMetadata, "Should have structured_metadata_wrapper option")
}

func TestComplexServiceMCPToolInvocation(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	tools := BuildAllTools(t, env)

	// Test that all tools have valid JSON schemas
	for _, tool := range tools {
		t.Run(fmt.Sprintf("ValidateJSONSchema_%s", tool.Name), func(t *testing.T) {
			ValidateJSONSchema(t, tool.InputSchema)

			if tool.OutputSchema != nil {
				ValidateJSONSchema(t, tool.OutputSchema)
			}
		})
	}
}

func TestComplexServiceToolsListMCPResponse(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	tools := BuildAllTools(t, env)

	// Create MCP tools list response
	toolsListResult := mcp.ToolsListResult{
		Tools: tools,
	}

	// Verify it can be marshaled to JSON
	jsonBytes, err := json.Marshal(toolsListResult)
	require.NoError(t, err)

	// Verify it can be unmarshaled back
	var unmarshaledResult mcp.ToolsListResult
	err = json.Unmarshal(jsonBytes, &unmarshaledResult)
	require.NoError(t, err)

	// Verify the structure is preserved
	assert.Equal(t, len(tools), len(unmarshaledResult.Tools))

	for i, tool := range unmarshaledResult.Tools {
		assert.Equal(t, tools[i].Name, tool.Name)
		assert.Equal(t, tools[i].Description, tool.Description)
		assert.NotNil(t, tool.InputSchema)
		assert.NotNil(t, tool.OutputSchema)
	}
}

func TestComplexServiceErrorHandling(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	services := DiscoverServices(t, env)
	require.NotEmpty(t, services)

	// Test with valid service - ensure proper functionality
	validService := services[0]
	tool, err := env.ToolBuilder.BuildTool(validService.Name, validService.Methods[0])
	require.NoError(t, err)

	// Verify tool is valid
	assert.NotEmpty(t, tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, tool.InputSchema)
	assert.NotNil(t, tool.OutputSchema)

	// Test tool name generation contains expected elements
	if strings.Contains(validService.Name, "complex") {
		assert.Contains(t, tool.Name, "com_example_complex")
	}
	assert.Contains(t, tool.Name, strings.ToLower(validService.Methods[0].Name))
}
