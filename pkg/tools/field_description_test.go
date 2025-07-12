package tools

import (
	"testing"

	"github.com/aalobaidi/ggRMCP/pkg/grpc"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestBuildTool_FieldDescriptions(t *testing.T) {
	// This test would verify that field descriptions are extracted correctly
	// Currently this will fail because getFieldDescription returns generic text

	// Create a method info with an actual protobuf descriptor that has comments
	// (This would require loading from a FileDescriptorSet with source info)

	t.Skip("This test demonstrates what we should be testing - field descriptions from comments")

	// Example of what we should verify:
	// tool, err := builder.BuildTool("hello.HelloService", methodInfoWithComments)
	// require.NoError(t, err)

	// inputSchema := tool.InputSchema.(map[string]interface{})
	// properties := inputSchema["properties"].(map[string]interface{})
	// nameField := properties["name"].(map[string]interface{})

	// This should pass if we properly extract field comments:
	// assert.Equal(t, "The name of the user", nameField["description"])
	//
	// But currently it would be:
	// assert.Equal(t, "Field name", nameField["description"]) // Generic!
}

func TestGenerateDescription_WithMethodComments(t *testing.T) {
	logger := zap.NewNop()
	builder := NewBuilder(logger)

	// Test that method descriptions come from MethodInfo.Description field
	methodInfo := grpc.MethodInfo{
		Name:        "SayHello",
		Description: "Sends a greeting to the user", // This should be used
	}

	description := builder.generateDescription("hello.HelloService", methodInfo)

	// Verify that the method description is used when available
	assert.Equal(t, "Sends a greeting to the user", description)
}

func TestGenerateDescription_WithoutComments(t *testing.T) {
	logger := zap.NewNop()
	builder := NewBuilder(logger)

	// Test fallback when no description is available
	methodInfo := grpc.MethodInfo{
		Name:        "SayHello",
		Description: "", // No description
	}

	description := builder.generateDescription("hello.HelloService", methodInfo)

	// Verify fallback description
	assert.Equal(t, "Calls the SayHello method of the hello.HelloService service", description)
}
