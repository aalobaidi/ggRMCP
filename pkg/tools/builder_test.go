package tools

import (
	"testing"

	"github.com/aalobaidi/ggRMCP/pkg/grpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	_ "github.com/aalobaidi/ggRMCP/pkg/testproto"
)

func TestBuildTool_RecursiveTypes(t *testing.T) {
	logger := zap.NewNop()
	builder := NewBuilder(logger)

	// Get the ProcessNodeRequest message descriptor
	messageDesc, err := protoregistry.GlobalFiles.FindDescriptorByName("com.example.complex.ProcessNodeRequest")
	require.NoError(t, err)

	inputDesc := messageDesc.(protoreflect.MessageDescriptor)

	// Get the ProcessNodeResponse message descriptor
	messageDesc, err = protoregistry.GlobalFiles.FindDescriptorByName("com.example.complex.ProcessNodeResponse")
	require.NoError(t, err)

	outputDesc := messageDesc.(protoreflect.MessageDescriptor)

	// Create a mock method info
	methodInfo := grpc.MethodInfo{
		Name:             "ProcessNode",
		FullName:         "com.example.complex.NodeService.ProcessNode",
		InputType:        "com.example.complex.ProcessNodeRequest",
		OutputType:       "com.example.complex.ProcessNodeResponse",
		InputDescriptor:  inputDesc,
		OutputDescriptor: outputDesc,
	}

	// Build the tool
	tool, err := builder.BuildTool("com.example.complex.NodeService", methodInfo)
	require.NoError(t, err)

	// Verify tool properties
	assert.Equal(t, "com_example_complex_nodeservice_processnode", tool.Name)
	assert.Contains(t, tool.Description, "ProcessNode")
	assert.Contains(t, tool.Description, "com.example.complex.NodeService")
	assert.NotNil(t, tool.InputSchema)
	assert.NotNil(t, tool.OutputSchema)

	// Verify input schema structure
	inputSchema, ok := tool.InputSchema.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "object", inputSchema["type"])

	properties, ok := inputSchema["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, properties, "root_node")

	// Verify the root_node property (should handle recursive Node type)
	rootNode, ok := properties["root_node"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "object", rootNode["type"])
}

func TestBuildTool_OneofTypes(t *testing.T) {
	logger := zap.NewNop()
	builder := NewBuilder(logger)

	// Get the CreateDocumentRequest message descriptor
	messageDesc, err := protoregistry.GlobalFiles.FindDescriptorByName("com.example.complex.CreateDocumentRequest")
	require.NoError(t, err)

	inputDesc := messageDesc.(protoreflect.MessageDescriptor)

	// Get the CreateDocumentResponse message descriptor
	messageDesc, err = protoregistry.GlobalFiles.FindDescriptorByName("com.example.complex.CreateDocumentResponse")
	require.NoError(t, err)

	outputDesc := messageDesc.(protoreflect.MessageDescriptor)

	// Create a mock method info
	methodInfo := grpc.MethodInfo{
		Name:             "CreateDocument",
		FullName:         "com.example.complex.DocumentService.CreateDocument",
		InputType:        "com.example.complex.CreateDocumentRequest",
		OutputType:       "com.example.complex.CreateDocumentResponse",
		InputDescriptor:  inputDesc,
		OutputDescriptor: outputDesc,
	}

	// Build the tool
	tool, err := builder.BuildTool("com.example.complex.DocumentService", methodInfo)
	require.NoError(t, err)

	// Verify tool properties
	assert.Equal(t, "com_example_complex_documentservice_createdocument", tool.Name)
	assert.Contains(t, tool.Description, "CreateDocument")
	assert.Contains(t, tool.Description, "com.example.complex.DocumentService")
	assert.NotNil(t, tool.InputSchema)
	assert.NotNil(t, tool.OutputSchema)

	// Verify input schema structure
	inputSchema, ok := tool.InputSchema.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "object", inputSchema["type"])

	properties, ok := inputSchema["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, properties, "document")

	// Verify the document property (should handle oneof metadata)
	document, ok := properties["document"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "object", document["type"])

	docProperties, ok := document["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, docProperties, "document_id")
	assert.Contains(t, docProperties, "title")
	assert.Contains(t, docProperties, "content")
	assert.Contains(t, docProperties, "metadata") // oneof field
}

func TestBuildTool_EnumTypes(t *testing.T) {
	logger := zap.NewNop()
	builder := NewBuilder(logger)

	// Get the GetUserProfileRequest message descriptor
	messageDesc, err := protoregistry.GlobalFiles.FindDescriptorByName("com.example.complex.GetUserProfileRequest")
	require.NoError(t, err)

	inputDesc := messageDesc.(protoreflect.MessageDescriptor)

	// Get the GetUserProfileResponse message descriptor
	messageDesc, err = protoregistry.GlobalFiles.FindDescriptorByName("com.example.complex.GetUserProfileResponse")
	require.NoError(t, err)

	outputDesc := messageDesc.(protoreflect.MessageDescriptor)

	// Create a mock method info
	methodInfo := grpc.MethodInfo{
		Name:             "GetUserProfile",
		FullName:         "com.example.complex.UserProfileService.GetUserProfile",
		InputType:        "com.example.complex.GetUserProfileRequest",
		OutputType:       "com.example.complex.GetUserProfileResponse",
		InputDescriptor:  inputDesc,
		OutputDescriptor: outputDesc,
	}

	// Build the tool
	tool, err := builder.BuildTool("com.example.complex.UserProfileService", methodInfo)
	require.NoError(t, err)

	// Verify tool properties
	assert.Equal(t, "com_example_complex_userprofileservice_getuserprofile", tool.Name)
	assert.Contains(t, tool.Description, "GetUserProfile")
	assert.Contains(t, tool.Description, "com.example.complex.UserProfileService")
	assert.NotNil(t, tool.InputSchema)
	assert.NotNil(t, tool.OutputSchema)

	// Verify output schema structure (should handle UserType enum)
	outputSchema, ok := tool.OutputSchema.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "object", outputSchema["type"])

	properties, ok := outputSchema["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, properties, "profile")

	// Verify the profile property (should handle UserType enum)
	profile, ok := properties["profile"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "object", profile["type"])

	profileProperties, ok := profile["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, profileProperties, "user_type")

	// Verify the user_type field (should be an enum)
	userType, ok := profileProperties["user_type"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "string", userType["type"])
	assert.Contains(t, userType, "enum")
}

func TestBuildTool_MapTypes(t *testing.T) {
	logger := zap.NewNop()
	builder := NewBuilder(logger)

	// Get the StructuredMetadata message descriptor
	messageDesc, err := protoregistry.GlobalFiles.FindDescriptorByName("com.example.complex.StructuredMetadata")
	require.NoError(t, err)

	structuredMetadataDesc := messageDesc.(protoreflect.MessageDescriptor)

	// Test generating schema for the map field
	visited := make(map[string]bool)
	schema, err := builder.generateMessageSchema(structuredMetadataDesc, visited)
	require.NoError(t, err)

	// Verify schema structure
	schemaMap, ok := schema.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "object", schemaMap["type"])

	properties, ok := schemaMap["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, properties, "data")

	// Verify the data property (should handle map<string, string>)
	data, ok := properties["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "object", data["type"])
	assert.Contains(t, data, "patternProperties")
}

func TestGenerateMessageSchema_CircularReference(t *testing.T) {
	logger := zap.NewNop()
	builder := NewBuilder(logger)

	// Get the Node message descriptor (which has recursive children field)
	messageDesc, err := protoregistry.GlobalFiles.FindDescriptorByName("com.example.complex.Node")
	require.NoError(t, err)

	nodeDesc := messageDesc.(protoreflect.MessageDescriptor)

	// Test generating schema for the recursive Node type
	visited := make(map[string]bool)
	schema, err := builder.generateMessageSchema(nodeDesc, visited)
	require.NoError(t, err)

	// Verify schema structure
	schemaMap, ok := schema.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "object", schemaMap["type"])

	properties, ok := schemaMap["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, properties, "id")
	assert.Contains(t, properties, "value")
	assert.Contains(t, properties, "children")

	// Verify the children property (should handle recursive reference)
	children, ok := properties["children"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "array", children["type"])
	assert.Contains(t, children, "items")

	// The items should either be a reference or a proper schema
	items := children["items"]
	assert.NotNil(t, items)
}

func TestBuildTools_MultipleServices(t *testing.T) {
	logger := zap.NewNop()
	builder := NewBuilder(logger)

	// Create mock service info for all three services
	services := make(map[string]grpc.ServiceInfo)

	// Mock UserProfileService
	userProfileInputDesc, err := protoregistry.GlobalFiles.FindDescriptorByName("com.example.complex.GetUserProfileRequest")
	require.NoError(t, err)
	userProfileOutputDesc, err := protoregistry.GlobalFiles.FindDescriptorByName("com.example.complex.GetUserProfileResponse")
	require.NoError(t, err)

	services["com.example.complex.UserProfileService"] = grpc.ServiceInfo{
		Name: "com.example.complex.UserProfileService",
		Methods: []grpc.MethodInfo{
			{
				Name:             "GetUserProfile",
				FullName:         "com.example.complex.UserProfileService.GetUserProfile",
				InputType:        "com.example.complex.GetUserProfileRequest",
				OutputType:       "com.example.complex.GetUserProfileResponse",
				InputDescriptor:  userProfileInputDesc.(protoreflect.MessageDescriptor),
				OutputDescriptor: userProfileOutputDesc.(protoreflect.MessageDescriptor),
			},
		},
	}

	// Mock DocumentService
	documentInputDesc, err := protoregistry.GlobalFiles.FindDescriptorByName("com.example.complex.CreateDocumentRequest")
	require.NoError(t, err)
	documentOutputDesc, err := protoregistry.GlobalFiles.FindDescriptorByName("com.example.complex.CreateDocumentResponse")
	require.NoError(t, err)

	services["com.example.complex.DocumentService"] = grpc.ServiceInfo{
		Name: "com.example.complex.DocumentService",
		Methods: []grpc.MethodInfo{
			{
				Name:             "CreateDocument",
				FullName:         "com.example.complex.DocumentService.CreateDocument",
				InputType:        "com.example.complex.CreateDocumentRequest",
				OutputType:       "com.example.complex.CreateDocumentResponse",
				InputDescriptor:  documentInputDesc.(protoreflect.MessageDescriptor),
				OutputDescriptor: documentOutputDesc.(protoreflect.MessageDescriptor),
			},
		},
	}

	// Mock NodeService
	nodeInputDesc, err := protoregistry.GlobalFiles.FindDescriptorByName("com.example.complex.ProcessNodeRequest")
	require.NoError(t, err)
	nodeOutputDesc, err := protoregistry.GlobalFiles.FindDescriptorByName("com.example.complex.ProcessNodeResponse")
	require.NoError(t, err)

	services["com.example.complex.NodeService"] = grpc.ServiceInfo{
		Name: "com.example.complex.NodeService",
		Methods: []grpc.MethodInfo{
			{
				Name:             "ProcessNode",
				FullName:         "com.example.complex.NodeService.ProcessNode",
				InputType:        "com.example.complex.ProcessNodeRequest",
				OutputType:       "com.example.complex.ProcessNodeResponse",
				InputDescriptor:  nodeInputDesc.(protoreflect.MessageDescriptor),
				OutputDescriptor: nodeOutputDesc.(protoreflect.MessageDescriptor),
			},
		},
	}

	// Build all tools
	tools, err := builder.BuildTools(services)
	require.NoError(t, err)

	// Verify all three tools were built
	assert.Len(t, tools, 3, "Should build tools for all three services")

	// Verify tool names
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	assert.True(t, toolNames["com_example_complex_userprofileservice_getuserprofile"], "Should include UserProfileService tool")
	assert.True(t, toolNames["com_example_complex_documentservice_createdocument"], "Should include DocumentService tool")
	assert.True(t, toolNames["com_example_complex_nodeservice_processnode"], "Should include NodeService tool")
}
