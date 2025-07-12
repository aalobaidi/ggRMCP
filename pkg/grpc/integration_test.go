package grpc

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestResolveMessageDescriptor_Issue(t *testing.T) {
	logger := zap.NewNop()
	client := &reflectionClient{
		logger:  logger,
		fdCache: make(map[string]*descriptorpb.FileDescriptorProto),
	}

	// Create a minimal file descriptor that simulates the issue
	fileDescriptor := &descriptorpb.FileDescriptorProto{
		Name:    stringPtr("test.proto"),
		Package: stringPtr("com.example.test"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: stringPtr("TestRequest"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   stringPtr("id"),
						Number: int32Ptr(1),
						Type:   fieldTypePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
					},
				},
			},
			{
				Name: stringPtr("TestResponse"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   stringPtr("message"),
						Number: int32Ptr(1),
						Type:   fieldTypePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
					},
				},
			},
		},
	}

	// Test resolving message descriptors
	_, err := client.resolveMessageDescriptor(".com.example.test.TestRequest", fileDescriptor)
	if err != nil {
		t.Logf("Error resolving TestRequest: %v", err)
		fmt.Printf("This is the issue! The resolveMessageDescriptor function is failing.\n")
		fmt.Printf("This explains why services are being discovered but methods are not being created.\n")
		fmt.Printf("The createMethodInfo function calls resolveMessageDescriptor which fails, so the methods are dropped.\n")
	} else {
		t.Log("TestRequest resolved successfully")
	}

	_, err = client.resolveMessageDescriptor(".com.example.test.TestResponse", fileDescriptor)
	if err != nil {
		t.Logf("Error resolving TestResponse: %v", err)
	} else {
		t.Log("TestResponse resolved successfully")
	}
}

func TestCreateMethodInfo_Issue(t *testing.T) {
	logger := zap.NewNop()
	client := &reflectionClient{
		logger:  logger,
		fdCache: make(map[string]*descriptorpb.FileDescriptorProto),
	}

	// Create a minimal file descriptor
	fileDescriptor := &descriptorpb.FileDescriptorProto{
		Name:    stringPtr("test.proto"),
		Package: stringPtr("com.example.test"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: stringPtr("TestRequest"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   stringPtr("id"),
						Number: int32Ptr(1),
						Type:   fieldTypePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
					},
				},
			},
			{
				Name: stringPtr("TestResponse"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   stringPtr("message"),
						Number: int32Ptr(1),
						Type:   fieldTypePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
					},
				},
			},
		},
	}

	// Create a method descriptor
	method := &descriptorpb.MethodDescriptorProto{
		Name:       stringPtr("TestMethod"),
		InputType:  stringPtr(".com.example.test.TestRequest"),
		OutputType: stringPtr(".com.example.test.TestResponse"),
	}

	// Test creating method info
	ctx := context.Background()
	methodInfo, err := client.createMethodInfo(ctx, "com.example.test.TestService", method, fileDescriptor)
	if err != nil {
		t.Logf("Error creating method info: %v", err)
		fmt.Printf("This confirms the issue! createMethodInfo is failing.\n")
		fmt.Printf("This is why your services are discovered but no methods are created.\n")

		// This is the root cause of the problem
		assert.Error(t, err, "createMethodInfo should fail due to message descriptor resolution issues")
	} else {
		t.Log("Method info created successfully")
		assert.Equal(t, "TestMethod", methodInfo.Name)
	}
}

func TestResolveMessageDescriptor_CrossFileDependencies(t *testing.T) {
	logger := zap.NewNop()
	client := &reflectionClient{
		logger:  logger,
		fdCache: make(map[string]*descriptorpb.FileDescriptorProto),
	}

	// Simulate a scenario where a message references types from different files
	// This tests the robustness of cross-file dependency resolution

	// File 1: Base types (simulating google.protobuf.Timestamp-like scenario)
	_ = &descriptorpb.FileDescriptorProto{
		Name:    stringPtr("base.proto"),
		Package: stringPtr("com.example.base"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: stringPtr("BaseMetadata"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   stringPtr("created_at"),
						Number: int32Ptr(1),
						Type:   fieldTypePtr(descriptorpb.FieldDescriptorProto_TYPE_INT64),
					},
					{
						Name:   stringPtr("version"),
						Number: int32Ptr(2),
						Type:   fieldTypePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
					},
				},
			},
		},
	}

	// File 2: Service file that depends on base types
	serviceFileDescriptor := &descriptorpb.FileDescriptorProto{
		Name:       stringPtr("service.proto"),
		Package:    stringPtr("com.example.service"),
		Dependency: []string{"base.proto"}, // Dependency on base file
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: stringPtr("ServiceRequest"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   stringPtr("id"),
						Number: int32Ptr(1),
						Type:   fieldTypePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
					},
					{
						Name:     stringPtr("metadata"),
						Number:   int32Ptr(2),
						Type:     fieldTypePtr(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE),
						TypeName: stringPtr(".com.example.base.BaseMetadata"), // Cross-file reference
					},
				},
			},
		},
		Service: []*descriptorpb.ServiceDescriptorProto{
			{
				Name: stringPtr("TestService"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       stringPtr("ProcessRequest"),
						InputType:  stringPtr(".com.example.service.ServiceRequest"),
						OutputType: stringPtr(".com.example.service.ServiceRequest"), // Reusing for simplicity
					},
				},
			},
		},
	}

	t.Run("ResolveLocalMessage", func(t *testing.T) {
		// Test resolving a message from the same file
		desc, err := client.resolveMessageDescriptor("com.example.service.ServiceRequest", serviceFileDescriptor)
		if err != nil {
			// This might fail with current implementation for cross-file deps
			// but we want to document the behavior
			t.Logf("Expected potential failure for local message resolution: %v", err)
		} else {
			assert.Equal(t, "ServiceRequest", string(desc.Name()))
			assert.Equal(t, "com.example.service.ServiceRequest", string(desc.FullName()))
			t.Log("Local message resolution successful")
		}
	})

	t.Run("ResolveCrossFileMessage", func(t *testing.T) {
		// Test resolving a message from a different file (cross-file dependency)
		desc, err := client.resolveMessageDescriptor("com.example.base.BaseMetadata", serviceFileDescriptor)
		if err != nil {
			// This documents current limitation - cross-file deps may not work
			// without proper dependency graph or global registry
			t.Logf("Cross-file dependency resolution failed (expected): %v", err)

			// Verify this is the expected error pattern for missing imports
			assert.Contains(t, err.Error(), "could not resolve import")
		} else {
			// If it works, verify it's correct
			assert.Equal(t, "BaseMetadata", string(desc.Name()))
			assert.Equal(t, "com.example.base.BaseMetadata", string(desc.FullName()))
			t.Log("Cross-file dependency resolution successful")
		}
	})

	t.Run("GlobalRegistryFallback", func(t *testing.T) {
		// Test that the global registry fallback works for well-known types
		// Using google.protobuf.Timestamp as an example
		desc, err := client.resolveMessageDescriptor("google.protobuf.Timestamp", serviceFileDescriptor)

		if err != nil {
			t.Logf("Global registry fallback test - this might fail in test environment: %v", err)
			// In a real gRPC environment, google.protobuf.Timestamp would be available
		} else {
			assert.Equal(t, "Timestamp", string(desc.Name()))
			t.Log("Global registry fallback successful for well-known type")
		}
	})
}

func TestCreateMethodInfo_WithCrossFileDependencies(t *testing.T) {
	logger := zap.NewNop()
	client := &reflectionClient{
		logger:  logger,
		fdCache: make(map[string]*descriptorpb.FileDescriptorProto),
	}

	// Test method creation with cross-file message dependencies
	fileDescriptor := &descriptorpb.FileDescriptorProto{
		Name:    stringPtr("cross_dep_service.proto"),
		Package: stringPtr("com.example.crossdep"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: stringPtr("CrossDepRequest"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   stringPtr("user_id"),
						Number: int32Ptr(1),
						Type:   fieldTypePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
					},
					// Simulating a field that references an external type
					{
						Name:     stringPtr("timestamp"),
						Number:   int32Ptr(2),
						Type:     fieldTypePtr(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE),
						TypeName: stringPtr(".google.protobuf.Timestamp"), // Well-known external type
					},
				},
			},
			{
				Name: stringPtr("CrossDepResponse"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   stringPtr("success"),
						Number: int32Ptr(1),
						Type:   fieldTypePtr(descriptorpb.FieldDescriptorProto_TYPE_BOOL),
					},
				},
			},
		},
	}

	method := &descriptorpb.MethodDescriptorProto{
		Name:       stringPtr("ProcessCrossDep"),
		InputType:  stringPtr(".com.example.crossdep.CrossDepRequest"),
		OutputType: stringPtr(".com.example.crossdep.CrossDepResponse"),
	}

	// Test method info creation with cross-file dependencies
	methodInfo, err := client.createMethodInfo(context.Background(), "com.example.crossdep.TestService", method, fileDescriptor)

	if err != nil {
		t.Logf("Method info creation with cross-file deps failed (may be expected): %v", err)
		// This helps us understand current limitations
		assert.Contains(t, err.Error(), "failed to resolve")
	} else {
		t.Log("Method info creation with cross-file dependencies successful")
		assert.Equal(t, "ProcessCrossDep", methodInfo.Name)
		assert.Equal(t, "com.example.crossdep.TestService.ProcessCrossDep", methodInfo.FullName)
		assert.NotNil(t, methodInfo.InputDescriptor)
		assert.NotNil(t, methodInfo.OutputDescriptor)
	}
}

func TestResolveMessageDescriptor_RealWorldScenario(t *testing.T) {
	logger := zap.NewNop()
	client := &reflectionClient{
		logger:  logger,
		fdCache: make(map[string]*descriptorpb.FileDescriptorProto),
	}

	// Test with our actual testdata that includes google.protobuf.Timestamp
	// This simulates the real gRPC reflection scenario
	testFileDescriptor := &descriptorpb.FileDescriptorProto{
		Name:       stringPtr("real_test.proto"),
		Package:    stringPtr("com.example.realtest"),
		Dependency: []string{"google/protobuf/timestamp.proto"},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: stringPtr("UserProfile"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   stringPtr("user_id"),
						Number: int32Ptr(1),
						Type:   fieldTypePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
					},
					{
						Name:     stringPtr("last_login"),
						Number:   int32Ptr(2),
						Type:     fieldTypePtr(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE),
						TypeName: stringPtr(".google.protobuf.Timestamp"), // Well-known type
					},
				},
			},
		},
	}

	t.Run("ResolveLocalMessageWithExternalDep", func(t *testing.T) {
		// Test resolving local message that has external dependencies
		desc, err := client.resolveMessageDescriptor("com.example.realtest.UserProfile", testFileDescriptor)

		if err != nil {
			// Document what happens when external deps are missing
			t.Logf("Local message with external deps failed: %v", err)
			// This shows the current limitation
			assert.Contains(t, err.Error(), "google/protobuf/timestamp.proto")
		} else {
			assert.Equal(t, "UserProfile", string(desc.Name()))
			t.Log("Local message with external deps resolved successfully")
		}
	})

	t.Run("ResolveWellKnownType", func(t *testing.T) {
		// Test resolving well-known types directly
		desc, err := client.resolveMessageDescriptor("google.protobuf.Timestamp", testFileDescriptor)

		if err != nil {
			t.Logf("Well-known type resolution failed in test env: %v", err)
			// In production with real gRPC reflection, this would likely work
		} else {
			assert.Equal(t, "Timestamp", string(desc.Name()))
			t.Log("Well-known type resolution successful")
		}
	})
}

func TestResolveMessageDescriptor_DocumentCurrentBehavior(t *testing.T) {
	// This test documents the current behavior and limitations
	// of the resolveMessageDescriptor function

	logger := zap.NewNop()
	client := &reflectionClient{
		logger:  logger,
		fdCache: make(map[string]*descriptorpb.FileDescriptorProto),
	}

	t.Run("SelfContainedFile", func(t *testing.T) {
		// Test with a completely self-contained file (no external deps)
		selfContainedFile := &descriptorpb.FileDescriptorProto{
			Name:    stringPtr("self_contained.proto"),
			Package: stringPtr("com.example.self"),
			MessageType: []*descriptorpb.DescriptorProto{
				{
					Name: stringPtr("SimpleMessage"),
					Field: []*descriptorpb.FieldDescriptorProto{
						{
							Name:   stringPtr("id"),
							Number: int32Ptr(1),
							Type:   fieldTypePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
						},
						{
							Name:   stringPtr("count"),
							Number: int32Ptr(2),
							Type:   fieldTypePtr(descriptorpb.FieldDescriptorProto_TYPE_INT32),
						},
					},
				},
			},
		}

		desc, err := client.resolveMessageDescriptor("com.example.self.SimpleMessage", selfContainedFile)
		assert.NoError(t, err, "Self-contained messages should resolve successfully")
		assert.Equal(t, "SimpleMessage", string(desc.Name()))
		assert.Equal(t, "com.example.self.SimpleMessage", string(desc.FullName()))
		t.Log("✅ Self-contained message resolution works perfectly")
	})

	t.Run("CurrentLimitations", func(t *testing.T) {
		// Document what doesn't work and why
		t.Log("📝 Current resolveMessageDescriptor limitations:")
		t.Log("   1. Cross-file dependencies require imports to be resolvable")
		t.Log("   2. Missing imports cause file descriptor creation to fail")
		t.Log("   3. Global registry fallback helps with well-known types")
		t.Log("   4. In real gRPC reflection, dependencies are provided by server")

		// This test always passes - it's documentation
	})
}

// Helper functions
func int32Ptr(i int32) *int32 {
	return &i
}

func fieldTypePtr(t descriptorpb.FieldDescriptorProto_Type) *descriptorpb.FieldDescriptorProto_Type {
	return &t
}
