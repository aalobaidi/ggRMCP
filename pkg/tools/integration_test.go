package tools

import (
	"testing"

	"github.com/aalobaidi/ggRMCP/pkg/descriptors"
	"github.com/aalobaidi/ggRMCP/pkg/grpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestToolGeneration_FileDescriptorSetIntegration(t *testing.T) {
	// End-to-end test: FileDescriptorSet -> ServiceDiscovery -> Tool Generation

	t.Run("GenerateToolsWithFileDescriptorSetComments", func(t *testing.T) {
		// This test demonstrates what should be tested for FileDescriptorSet integration
		// Currently requires access to internal methods or a running gRPC server

		t.Skip("Integration test requires either internal method access or running gRPC server")

		// What this test should verify:
		// 1. Load FileDescriptorSet with source comments
		// 2. Discover services using FileDescriptorSet
		// 3. Generate tools with proper method descriptions from comments
		// 4. Verify tool descriptions contain content from proto file comments
		// 5. Eventually verify field descriptions when that feature is implemented
	})

	t.Run("CompareToolDescriptions_FileDescriptorSetVsReflection", func(t *testing.T) {
		// This test would compare tool descriptions between FileDescriptorSet and reflection
		// to demonstrate the value of FileDescriptorSet comments

		t.Skip("Requires running gRPC server for reflection comparison")

		// Would test:
		// 1. Generate tools from FileDescriptorSet (with comments)
		// 2. Generate tools from reflection (without comments)
		// 3. Compare descriptions to show FileDescriptorSet provides richer information
	})
}

func TestToolBuilder_CommentPropagation(t *testing.T) {
	logger := zap.NewNop()
	builder := NewBuilder(logger)

	t.Run("MethodDescriptionPropagation", func(t *testing.T) {
		// Test that MethodInfo.Description properly flows to tool description
		methodWithDescription := grpc.MethodInfo{
			Name:        "SayHello",
			FullName:    "hello.HelloService.SayHello",
			Description: "Sends a personalized greeting to the user", // From FileDescriptorSet
			// ... other fields would be set by real discovery
		}

		methodWithoutDescription := grpc.MethodInfo{
			Name:        "SayHello",
			FullName:    "hello.HelloService.SayHello",
			Description: "", // No description
		}

		// Test with description
		descWithComments := builder.generateDescription("hello.HelloService", methodWithDescription)
		assert.Equal(t, "Sends a personalized greeting to the user", descWithComments)
		t.Logf("✅ Description with comments: '%s'", descWithComments)

		// Test without description (fallback)
		descWithoutComments := builder.generateDescription("hello.HelloService", methodWithoutDescription)
		assert.Equal(t, "Calls the SayHello method of the hello.HelloService service", descWithoutComments)
		t.Logf("✅ Fallback description: '%s'", descWithoutComments)
	})

	t.Run("FieldDescriptionExtraction", func(t *testing.T) {
		// Test that field descriptions are extracted from FileDescriptorSet comments
		// Load the FileDescriptorSet with comments
		loader := descriptors.NewLoader(zap.NewNop())

		// Load the hello.binpb file with comments
		fdSet, err := loader.LoadFromFile("../../examples/hello-service/build/hello.binpb")
		require.NoError(t, err, "Should load FileDescriptorSet")

		// Build file registry from FileDescriptorSet
		files, err := loader.BuildRegistry(fdSet)
		require.NoError(t, err, "Should build file registry")

		// Extract service information with comments
		services, err := loader.ExtractServiceInfo(files)
		require.NoError(t, err, "Should extract service info")
		require.NotEmpty(t, services, "Should have services")

		// Find the HelloService
		var helloService *descriptors.EnhancedServiceInfo
		for _, service := range services {
			if service.Name == "hello.HelloService" {
				helloService = &service
				break
			}
		}
		require.NotNil(t, helloService, "Should find HelloService")

		// Find the SayHello method
		var sayHelloMethod *descriptors.EnhancedMethodInfo
		for _, method := range helloService.Methods {
			if method.Name == "SayHello" {
				sayHelloMethod = &method
				break
			}
		}
		require.NotNil(t, sayHelloMethod, "Should find SayHello method")

		// Test field description extraction from input message (HelloRequest)
		inputDesc := sayHelloMethod.InputDescriptor

		// Test name field
		nameField := inputDesc.Fields().ByName("name")
		require.NotNil(t, nameField, "Should find name field")
		nameDescription := builder.GetFieldDescription(nameField)
		assert.Equal(t, "The name of the user", nameDescription, "Should extract name field comment")

		// Test email field
		emailField := inputDesc.Fields().ByName("email")
		require.NotNil(t, emailField, "Should find email field")
		emailDescription := builder.GetFieldDescription(emailField)
		assert.Equal(t, "The email of the user", emailDescription, "Should extract email field comment")

		// Test field description extraction from output message (HelloReply)
		outputDesc := sayHelloMethod.OutputDescriptor

		// Test message field
		messageField := outputDesc.Fields().ByName("message")
		require.NotNil(t, messageField, "Should find message field")
		messageDescription := builder.GetFieldDescription(messageField)
		assert.Equal(t, "The greeting message", messageDescription, "Should extract message field comment")

		t.Log("✅ Field descriptions extracted successfully:")
		t.Logf("   - name: '%s'", nameDescription)
		t.Logf("   - email: '%s'", emailDescription)
		t.Logf("   - message: '%s'", messageDescription)
	})
}

func TestEndToEndWorkflow_FileDescriptorSetToMCP(t *testing.T) {
	// Complete end-to-end test simulating the full MCP tool generation workflow

	t.Run("CompleteWorkflow", func(t *testing.T) {
		// Simulate the complete flow:
		// Proto with comments -> FileDescriptorSet -> ServiceDiscovery -> ToolBuilder -> MCP Tools

		t.Skip("Test requires running gRPC server or access to internal methods")

		// Would test:
		// 1. Create discoverer with FileDescriptorSet config
		// 2. Discover services from FileDescriptorSet
		// 3. Verify services have method descriptions
		// 4. Generate tools and verify descriptions propagate
		// 5. Verify tool structure is complete and valid
	})
}
