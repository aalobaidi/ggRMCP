package grpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestDiscoverServices_MultipleServicesInOneFile(t *testing.T) {
	logger := zap.NewNop()
	client := &reflectionClient{
		logger:  logger,
		fdCache: make(map[string]*descriptorpb.FileDescriptorProto),
	}

	// Simulate the exact scenario from your Java server
	// This tests the complete flow from service names to final ServiceInfo
	services := []string{
		"com.example.hello.HelloService",
		"com.example.complex.UserProfileService",
		"com.example.complex.DocumentService",
		"com.example.complex.NodeService",
	}

	// Filter internal services
	filtered := client.filterInternalServices(services)
	assert.Equal(t, services, filtered, "All services should pass filtering")

	// Test the new discovery logic
	// Create file descriptors for each service
	fileDescriptorMap := make(map[string]*descriptorpb.FileDescriptorProto)
	serviceToFileMap := make(map[string]string)

	// Create hello service file descriptor
	helloFileDescriptor := &descriptorpb.FileDescriptorProto{
		Name:    stringPtr("hello.proto"),
		Package: stringPtr("com.example.hello"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: stringPtr("HelloRequest"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   stringPtr("name"),
						Number: int32Ptr(1),
						Type:   fieldTypePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
					},
					{
						Name:   stringPtr("email"),
						Number: int32Ptr(2),
						Type:   fieldTypePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
					},
				},
			},
			{
				Name: stringPtr("HelloResponse"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   stringPtr("message"),
						Number: int32Ptr(1),
						Type:   fieldTypePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
					},
				},
			},
		},
		Service: []*descriptorpb.ServiceDescriptorProto{
			{
				Name: stringPtr("HelloService"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       stringPtr("SayHello"),
						InputType:  stringPtr(".com.example.hello.HelloRequest"),
						OutputType: stringPtr(".com.example.hello.HelloResponse"),
					},
				},
			},
		},
	}

	// Create complex services file descriptor (multiple services in one file)
	complexFileDescriptor := &descriptorpb.FileDescriptorProto{
		Name:    stringPtr("complex.proto"),
		Package: stringPtr("com.example.complex"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: stringPtr("GetUserProfileRequest"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   stringPtr("user_id"),
						Number: int32Ptr(1),
						Type:   fieldTypePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
					},
				},
			},
			{
				Name: stringPtr("GetUserProfileResponse"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   stringPtr("user_id"),
						Number: int32Ptr(1),
						Type:   fieldTypePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
					},
				},
			},
			{
				Name: stringPtr("CreateDocumentRequest"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   stringPtr("document_id"),
						Number: int32Ptr(1),
						Type:   fieldTypePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
					},
				},
			},
			{
				Name: stringPtr("CreateDocumentResponse"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   stringPtr("document_id"),
						Number: int32Ptr(1),
						Type:   fieldTypePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
					},
				},
			},
			{
				Name: stringPtr("ProcessNodeRequest"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   stringPtr("node_id"),
						Number: int32Ptr(1),
						Type:   fieldTypePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
					},
				},
			},
			{
				Name: stringPtr("ProcessNodeResponse"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   stringPtr("result"),
						Number: int32Ptr(1),
						Type:   fieldTypePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
					},
				},
			},
		},
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

	// Simulate the file descriptor mapping
	fileDescriptorMap["hello.proto"] = helloFileDescriptor
	fileDescriptorMap["complex.proto"] = complexFileDescriptor

	serviceToFileMap["com.example.hello.HelloService"] = "hello.proto"
	serviceToFileMap["com.example.complex.UserProfileService"] = "complex.proto"
	serviceToFileMap["com.example.complex.DocumentService"] = "complex.proto"
	serviceToFileMap["com.example.complex.NodeService"] = "complex.proto"

	// Test the new service extraction logic
	ctx := context.Background()
	var allServices []ServiceInfo
	processedServices := make(map[string]bool)

	for fileName, fileDescriptor := range fileDescriptorMap {
		t.Logf("Processing file descriptor: %s", fileName)

		// Extract all services from this file descriptor
		fileServices := client.extractServicesFromFileDescriptor(ctx, fileDescriptor, filtered)

		t.Logf("Found %d services in file %s", len(fileServices), fileName)
		for _, service := range fileServices {
			t.Logf("  - Service: %s with %d methods", service.Name, len(service.Methods))

			if !processedServices[service.Name] {
				allServices = append(allServices, service)
				processedServices[service.Name] = true
			}
		}
	}

	// Verify all services were extracted
	t.Logf("Total services extracted: %d", len(allServices))

	// This test should reveal if the issue is in the service extraction or elsewhere
	assert.Len(t, allServices, 4, "Should extract all 4 services")

	// Count services with methods
	servicesWithMethods := 0
	for _, service := range allServices {
		if len(service.Methods) > 0 {
			servicesWithMethods++
		}
	}

	t.Logf("Services with methods: %d", servicesWithMethods)

	// If this is less than 4, then the problem is in createMethodInfo
	// If it's 4, then the problem is elsewhere in the pipeline
	assert.Equal(t, 4, servicesWithMethods, "All services should have methods")
}
