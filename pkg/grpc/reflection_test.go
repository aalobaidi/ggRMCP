package grpc

import (
	"context"
	"testing"

	"go.uber.org/zap"
	"google.golang.org/protobuf/types/descriptorpb"
	"github.com/stretchr/testify/assert"
)

func TestExtractServicesFromFileDescriptor(t *testing.T) {
	logger := zap.NewNop()
	client := &reflectionClient{
		logger:  logger,
		fdCache: make(map[string]*descriptorpb.FileDescriptorProto),
	}

	// Create a mock file descriptor with multiple services
	fileDescriptor := &descriptorpb.FileDescriptorProto{
		Name:    stringPtr("complex.proto"),
		Package: stringPtr("com.example.complex"),
		Service: []*descriptorpb.ServiceDescriptorProto{
			{
				Name: stringPtr("UserProfileService"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       stringPtr("GetUserProfile"),
						InputType:  stringPtr(".com.example.complex.GetUserProfileRequest"),
						OutputType: stringPtr(".com.example.complex.GetUserProfileResponse"),
					},
				},
			},
			{
				Name: stringPtr("DocumentService"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       stringPtr("CreateDocument"),
						InputType:  stringPtr(".com.example.complex.CreateDocumentRequest"),
						OutputType: stringPtr(".com.example.complex.CreateDocumentResponse"),
					},
				},
			},
			{
				Name: stringPtr("NodeService"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       stringPtr("ProcessNode"),
						InputType:  stringPtr(".com.example.complex.ProcessNodeRequest"),
						OutputType: stringPtr(".com.example.complex.ProcessNodeResponse"),
					},
				},
			},
		},
	}

	// Target services list
	targetServices := []string{
		"com.example.complex.UserProfileService",
		"com.example.complex.DocumentService",
		"com.example.complex.NodeService",
	}

	// Test extracting services from file descriptor
	ctx := context.Background()
	services := client.extractServicesFromFileDescriptor(ctx, fileDescriptor, targetServices)

	// Verify all three services were extracted
	assert.Len(t, services, 3, "Should extract all three services")

	// Verify service names
	serviceNames := make(map[string]bool)
	for _, service := range services {
		serviceNames[service.Name] = true
	}

	assert.True(t, serviceNames["com.example.complex.UserProfileService"], "Should include UserProfileService")
	assert.True(t, serviceNames["com.example.complex.DocumentService"], "Should include DocumentService")
	assert.True(t, serviceNames["com.example.complex.NodeService"], "Should include NodeService")

	// Note: Methods will be empty because createMethodInfo fails without proper message descriptors
	// This test focuses on verifying that the service extraction logic works correctly
	// The method resolution is tested separately in integration tests
}

func TestExtractServicesFromFileDescriptor_PartialTargetList(t *testing.T) {
	logger := zap.NewNop()
	client := &reflectionClient{
		logger:  logger,
		fdCache: make(map[string]*descriptorpb.FileDescriptorProto),
	}

	// Create a mock file descriptor with multiple services
	fileDescriptor := &descriptorpb.FileDescriptorProto{
		Name:    stringPtr("complex.proto"),
		Package: stringPtr("com.example.complex"),
		Service: []*descriptorpb.ServiceDescriptorProto{
			{
				Name: stringPtr("UserProfileService"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       stringPtr("GetUserProfile"),
						InputType:  stringPtr(".com.example.complex.GetUserProfileRequest"),
						OutputType: stringPtr(".com.example.complex.GetUserProfileResponse"),
					},
				},
			},
			{
				Name: stringPtr("DocumentService"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       stringPtr("CreateDocument"),
						InputType:  stringPtr(".com.example.complex.CreateDocumentRequest"),
						OutputType: stringPtr(".com.example.complex.CreateDocumentResponse"),
					},
				},
			},
			{
				Name: stringPtr("NodeService"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       stringPtr("ProcessNode"),
						InputType:  stringPtr(".com.example.complex.ProcessNodeRequest"),
						OutputType: stringPtr(".com.example.complex.ProcessNodeResponse"),
					},
				},
			},
		},
	}

	// Target services list - only include two services
	targetServices := []string{
		"com.example.complex.UserProfileService",
		"com.example.complex.NodeService",
	}

	// Test extracting services from file descriptor
	ctx := context.Background()
	services := client.extractServicesFromFileDescriptor(ctx, fileDescriptor, targetServices)

	// Verify only two services were extracted
	assert.Len(t, services, 2, "Should extract only two services")

	// Verify service names
	serviceNames := make(map[string]bool)
	for _, service := range services {
		serviceNames[service.Name] = true
	}

	assert.True(t, serviceNames["com.example.complex.UserProfileService"], "Should include UserProfileService")
	assert.False(t, serviceNames["com.example.complex.DocumentService"], "Should not include DocumentService")
	assert.True(t, serviceNames["com.example.complex.NodeService"], "Should include NodeService")
}

func TestExtractServicesFromFileDescriptor_NoPackage(t *testing.T) {
	logger := zap.NewNop()
	client := &reflectionClient{
		logger:  logger,
		fdCache: make(map[string]*descriptorpb.FileDescriptorProto),
	}

	// Create a mock file descriptor without package
	fileDescriptor := &descriptorpb.FileDescriptorProto{
		Name: stringPtr("simple.proto"),
		// No package
		Service: []*descriptorpb.ServiceDescriptorProto{
			{
				Name: stringPtr("SimpleService"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       stringPtr("SimpleMethod"),
						InputType:  stringPtr(".SimpleRequest"),
						OutputType: stringPtr(".SimpleResponse"),
					},
				},
			},
		},
	}

	// Target services list
	targetServices := []string{
		"SimpleService",
	}

	// Test extracting services from file descriptor
	ctx := context.Background()
	services := client.extractServicesFromFileDescriptor(ctx, fileDescriptor, targetServices)

	// Verify service was extracted
	assert.Len(t, services, 1, "Should extract one service")
	assert.Equal(t, "SimpleService", services[0].Name)
}

func TestFilterInternalServices(t *testing.T) {
	logger := zap.NewNop()
	client := &reflectionClient{
		logger:  logger,
		fdCache: make(map[string]*descriptorpb.FileDescriptorProto),
	}

	services := []string{
		"com.example.hello.HelloService",
		"com.example.complex.UserProfileService",
		"com.example.complex.DocumentService",
		"com.example.complex.NodeService",
		"grpc.reflection.v1alpha.ServerReflection",
		"grpc.health.v1.Health",
		"grpc.channelz.v1.Channelz",
		"grpc.testing.TestService",
	}

	filtered := client.filterInternalServices(services)

	expected := []string{
		"com.example.hello.HelloService",
		"com.example.complex.UserProfileService",
		"com.example.complex.DocumentService",
		"com.example.complex.NodeService",
	}

	assert.Equal(t, expected, filtered, "Should filter out internal gRPC services")
}

func TestGetSimpleServiceName(t *testing.T) {
	tests := []struct {
		fullName string
		expected string
	}{
		{
			fullName: "com.example.complex.UserProfileService",
			expected: "UserProfileService",
		},
		{
			fullName: "com.example.complex.DocumentService",
			expected: "DocumentService",
		},
		{
			fullName: "com.example.complex.NodeService",
			expected: "NodeService",
		},
		{
			fullName: "SimpleService",
			expected: "SimpleService",
		},
		{
			fullName: "",
			expected: "",
		},
	}

	for _, test := range tests {
		t.Run(test.fullName, func(t *testing.T) {
			result := getSimpleServiceName(test.fullName)
			assert.Equal(t, test.expected, result)
		})
	}
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}