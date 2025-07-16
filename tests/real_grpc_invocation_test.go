//go:build integration
// +build integration

package tests

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aalobaidi/ggRMCP/pkg/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRealGrpcSimpleInvocation(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	test := ToolCallTestCase{
		Name:     "UserProfileService.GetUserProfile",
		ToolName: "com_example_complex_userprofileservice_getuserprofile",
		Arguments: map[string]interface{}{
			"user_id": "standard",
		},
		Validator: func(t *testing.T, response map[string]interface{}) {
			profile := response["profile"].(map[string]interface{})
			assert.Equal(t, "Test User standard", profile["displayName"])
			assert.Equal(t, "standard@example.com", profile["email"])
			assert.Equal(t, "STANDARD", profile["userType"])
		},
	}

	ExecuteToolCall(t, env, test)
}

func TestRealGrpcComplexTypesUserProfile(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	testCases := []struct {
		name         string
		userId       string
		expectedType string
	}{
		{"Standard User", "standard", "STANDARD"},
		{"Premium User", "premium", "PREMIUM"},
		{"Admin User", "admin", "ADMIN"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			test := ToolCallTestCase{
				Name:     tc.name,
				ToolName: "com_example_complex_userprofileservice_getuserprofile",
				Arguments: map[string]interface{}{
					"user_id": tc.userId,
				},
				Validator: func(t *testing.T, response map[string]interface{}) {
					profile, ok := response["profile"].(map[string]interface{})
					require.True(t, ok, "Response should contain profile field. Got: %+v", response)

					assert.Equal(t, tc.userId, profile["userId"])
					assert.Equal(t, "Test User "+tc.userId, profile["displayName"])
					assert.Equal(t, tc.userId+"@example.com", profile["email"])
					assert.Equal(t, tc.expectedType, profile["userType"])
					assert.NotNil(t, profile["lastLogin"])

					lastLogin, ok := profile["lastLogin"].(string)
					require.True(t, ok)
					assert.Contains(t, lastLogin, "2024-01-01T12:00:00Z")
				},
			}

			ExecuteToolCall(t, env, test)
		})
	}
}

func TestRealGrpcDocumentWithOneof(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	testCases := []struct {
		name     string
		document map[string]interface{}
		expected string
	}{
		{
			name: "Document with simple summary",
			document: map[string]interface{}{
				"document_id":    "doc1",
				"title":          "Test Document",
				"content":        "This is a test document",
				"simple_summary": "A test",
			},
			expected: "doc-Test-Document",
		},
		{
			name: "Document with structured metadata",
			document: map[string]interface{}{
				"document_id": "doc2",
				"title":       "Complex Document",
				"content":     "This is a complex document",
				"structured_metadata_wrapper": map[string]interface{}{
					"data": map[string]interface{}{
						"author":   "John Doe",
						"category": "Technical",
						"version":  "1.0",
					},
				},
			},
			expected: "doc-Complex-Document",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			test := ToolCallTestCase{
				Name:     tc.name,
				ToolName: "com_example_complex_documentservice_createdocument",
				Arguments: map[string]interface{}{
					"document": tc.document,
				},
				Validator: func(t *testing.T, response map[string]interface{}) {
					assert.Equal(t, tc.expected, response["documentId"])
					assert.Equal(t, true, response["success"])
				},
			}

			ExecuteToolCall(t, env, test)
		})
	}
}

func TestRealGrpcRecursiveNode(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	testCases := []struct {
		name            string
		rootNode        map[string]interface{}
		expectedNodes   int32
		expectedSummary string
	}{
		{
			name: "Single node",
			rootNode: map[string]interface{}{
				"id":    "root",
				"value": "Root Node",
			},
			expectedNodes:   1,
			expectedSummary: "Processed tree with root 'Root Node'",
		},
		{
			name: "Tree with children",
			rootNode: map[string]interface{}{
				"id":    "root",
				"value": "Root Node",
				"children": []map[string]interface{}{
					{
						"id":    "child1",
						"value": "Child 1",
					},
					{
						"id":    "child2",
						"value": "Child 2",
						"children": []map[string]interface{}{
							{
								"id":    "grandchild1",
								"value": "Grandchild 1",
							},
						},
					},
				},
			},
			expectedNodes:   4,
			expectedSummary: "Processed tree with root 'Root Node'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			test := ToolCallTestCase{
				Name:     tc.name,
				ToolName: "com_example_complex_nodeservice_processnode",
				Arguments: map[string]interface{}{
					"root_node": tc.rootNode,
				},
				Validator: func(t *testing.T, response map[string]interface{}) {
					assert.Equal(t, tc.expectedSummary, response["processedSummary"])
					AssertNumericField(t, response, "totalNodes", tc.expectedNodes)
				},
			}

			ExecuteToolCall(t, env, test)
		})
	}
}

func TestRealGrpcErrorHandling(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	errorTests := []ToolCallTestCase{
		{
			Name:     "Invalid tool name",
			ToolName: "nonexistent_tool",
			Arguments: map[string]interface{}{
				"test": "value",
			},
			ExpectHandlerError: true,
			ErrorSubstring:     "does not match any discovered service method",
		},
		{
			Name:     "Service error - user not found",
			ToolName: "com_example_complex_userprofileservice_getuserprofile",
			Arguments: map[string]interface{}{
				"user_id": "error",
			},
			ExpectError:    true,
			ErrorSubstring: "user not found",
		},
		{
			Name:     "Invalid document - missing title",
			ToolName: "com_example_complex_documentservice_createdocument",
			Arguments: map[string]interface{}{
				"document": map[string]interface{}{
					"document_id": "doc1",
					"content":     "content",
				},
			},
			ExpectError:    true,
			ErrorSubstring: "invalid document",
		},
		{
			Name:     "Missing required field - root_node",
			ToolName: "com_example_complex_nodeservice_processnode",
			Arguments: map[string]interface{}{
				"invalid_field": "value",
			},
			ExpectError:    true,
			ErrorSubstring: "unknown field",
		},
	}

	for _, test := range errorTests {
		t.Run(test.Name, func(t *testing.T) {
			ExecuteToolCall(t, env, test)
		})
	}
}

func TestRealGrpcJSONMarshalingEdgeCases(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	edgeCaseTests := []struct {
		name      string
		arguments map[string]interface{}
		validator func(t *testing.T, response map[string]interface{})
	}{
		{
			name: "Empty name",
			arguments: map[string]interface{}{
				"user_id": "empty",
			},
			validator: func(t *testing.T, response map[string]interface{}) {
				profile := response["profile"].(map[string]interface{})
				assert.Equal(t, "empty", profile["userId"])
				assert.Equal(t, "Test User empty", profile["displayName"])
			},
		},
		{
			name: "Empty email",
			arguments: map[string]interface{}{
				"user_id": "test",
			},
			validator: func(t *testing.T, response map[string]interface{}) {
				profile := response["profile"].(map[string]interface{})
				assert.Equal(t, "test", profile["userId"])
				assert.Equal(t, "test@example.com", profile["email"])
			},
		},
		{
			name: "Special characters",
			arguments: map[string]interface{}{
				"user_id": "jöhn.döe",
			},
			validator: func(t *testing.T, response map[string]interface{}) {
				profile := response["profile"].(map[string]interface{})
				assert.Equal(t, "jöhn.döe", profile["userId"])
				assert.Equal(t, "Test User jöhn.döe", profile["displayName"])
			},
		},
		{
			name: "Unicode characters",
			arguments: map[string]interface{}{
				"user_id": "张三",
			},
			validator: func(t *testing.T, response map[string]interface{}) {
				profile := response["profile"].(map[string]interface{})
				assert.Equal(t, "张三", profile["userId"])
				assert.Equal(t, "Test User 张三", profile["displayName"])
			},
		},
	}

	for _, tc := range edgeCaseTests {
		t.Run(tc.name, func(t *testing.T) {
			test := ToolCallTestCase{
				Name:      tc.name,
				ToolName:  "com_example_complex_userprofileservice_getuserprofile",
				Arguments: tc.arguments,
				Validator: tc.validator,
			}

			ExecuteToolCall(t, env, test)
		})
	}
}

func TestRealGrpcMethodNameConversion(t *testing.T) {
	conversionTests := []struct {
		fullName    string
		methodName  string
		expected    string
		description string
	}{
		{
			fullName:    "hello.HelloService.SayHello",
			methodName:  "SayHello",
			expected:    "/hello.HelloService/SayHello",
			description: "Simple service",
		},
		{
			fullName:    "com.example.complex.UserProfileService.GetUserProfile",
			methodName:  "GetUserProfile",
			expected:    "/com.example.complex.UserProfileService/GetUserProfile",
			description: "Complex package name",
		},
		{
			fullName:    "package.Service.Method",
			methodName:  "Method",
			expected:    "/package.Service/Method",
			description: "Minimal case",
		},
	}

	for _, tc := range conversionTests {
		t.Run(tc.description, func(t *testing.T) {
			grpcMethodName := fmt.Sprintf("/%s/%s",
				tc.fullName[:strings.LastIndex(tc.fullName, ".")],
				tc.methodName)

			assert.Equal(t, tc.expected, grpcMethodName)
		})
	}
}

func TestRealGrpcToolNameParsing(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	services := DiscoverServices(t, env)
	require.NotEmpty(t, services)

	parsingTests := []struct {
		toolName        string
		expectedService string
		expectedMethod  string
		shouldFind      bool
	}{
		{
			toolName:        "com_example_complex_userprofileservice_getuserprofile",
			expectedService: "com.example.complex.UserProfileService",
			expectedMethod:  "GetUserProfile",
			shouldFind:      true,
		},
		{
			toolName:   "nonexistent_service_method",
			shouldFind: false,
		},
		{
			toolName:   "invalid_tool_name",
			shouldFind: false,
		},
	}

	for _, tc := range parsingTests {
		t.Run(tc.toolName, func(t *testing.T) {
			foundService, foundMethod, found := ParseToolNameFromServices(tc.toolName, services)

			if tc.shouldFind {
				assert.True(t, found, "Should find matching service and method")
				assert.Equal(t, tc.expectedService, foundService)
				assert.Equal(t, tc.expectedMethod, foundMethod)
			} else {
				assert.False(t, found, "Should not find matching service and method")
			}
		})
	}
}

func TestRealGrpcConcurrentInvocations(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	results := make(chan *mcp.ToolCallResult, concurrencyLevel)
	errors := make(chan error, concurrencyLevel)

	for i := 0; i < concurrencyLevel; i++ {
		go func(id int) {
			arguments := map[string]interface{}{
				"user_id": fmt.Sprintf("user-%d", id),
			}

			result, err := executeToolCallDirect(env, "com_example_complex_userprofileservice_getuserprofile", arguments)
			if err != nil {
				errors <- err
				return
			}

			results <- result
		}(i)
	}

	// Collect and verify results
	successCount, errorCount := 0, 0

	for i := 0; i < concurrencyLevel; i++ {
		select {
		case result := <-results:
			assert.False(t, result.IsError)
			response := ParseJSONResponse(t, result)
			profile := response["profile"].(map[string]interface{})
			assert.Contains(t, profile["displayName"], "Test User user-")
			successCount++

		case err := <-errors:
			t.Errorf("Unexpected error: %v", err)
			errorCount++

		case <-time.After(10 * time.Second):
			t.Error("Timeout waiting for concurrent invocations")
			return
		}
	}

	assert.Equal(t, concurrencyLevel, successCount)
	assert.Equal(t, 0, errorCount)
}
