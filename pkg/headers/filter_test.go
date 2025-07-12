package headers

import (
	"testing"

	"github.com/aalobaidi/ggRMCP/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeaderFilter_ShouldForward(t *testing.T) {
	tests := []struct {
		name           string
		config         config.HeaderForwardingConfig
		headerName     string
		expectedResult bool
	}{
		{
			name: "Disabled_filter_blocks_all",
			config: config.HeaderForwardingConfig{
				Enabled: false,
			},
			headerName:     "authorization",
			expectedResult: false,
		},
		{
			name: "Allowed_header_is_forwarded",
			config: config.HeaderForwardingConfig{
				Enabled:        true,
				AllowedHeaders: []string{"authorization", "x-trace-id"},
				ForwardAll:     false,
				CaseSensitive:  false,
			},
			headerName:     "authorization",
			expectedResult: true,
		},
		{
			name: "Blocked_header_is_not_forwarded",
			config: config.HeaderForwardingConfig{
				Enabled:        true,
				AllowedHeaders: []string{"authorization", "cookie"},
				BlockedHeaders: []string{"cookie"},
				ForwardAll:     false,
				CaseSensitive:  false,
			},
			headerName:     "cookie",
			expectedResult: false,
		},
		{
			name: "Case_insensitive_matching",
			config: config.HeaderForwardingConfig{
				Enabled:        true,
				AllowedHeaders: []string{"Authorization", "X-Trace-ID"},
				ForwardAll:     false,
				CaseSensitive:  false,
			},
			headerName:     "authorization",
			expectedResult: true,
		},
		{
			name: "Case_sensitive_matching_fails",
			config: config.HeaderForwardingConfig{
				Enabled:        true,
				AllowedHeaders: []string{"Authorization"},
				ForwardAll:     false,
				CaseSensitive:  true,
			},
			headerName:     "authorization",
			expectedResult: false,
		},
		{
			name: "Case_sensitive_matching_succeeds",
			config: config.HeaderForwardingConfig{
				Enabled:        true,
				AllowedHeaders: []string{"Authorization"},
				ForwardAll:     false,
				CaseSensitive:  true,
			},
			headerName:     "Authorization",
			expectedResult: true,
		},
		{
			name: "Forward_all_allows_everything",
			config: config.HeaderForwardingConfig{
				Enabled:    true,
				ForwardAll: true,
			},
			headerName:     "random-header",
			expectedResult: true,
		},
		{
			name: "Forward_all_respects_blocked_headers",
			config: config.HeaderForwardingConfig{
				Enabled:        true,
				BlockedHeaders: []string{"cookie"},
				ForwardAll:     true,
			},
			headerName:     "cookie",
			expectedResult: false,
		},
		{
			name: "Unknown_header_not_forwarded_without_forward_all",
			config: config.HeaderForwardingConfig{
				Enabled:        true,
				AllowedHeaders: []string{"authorization"},
				ForwardAll:     false,
			},
			headerName:     "unknown-header",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewFilter(tt.config)
			result := filter.ShouldForward(tt.headerName)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestHeaderFilter_FilterHeaders(t *testing.T) {
	config := config.HeaderForwardingConfig{
		Enabled: true,
		AllowedHeaders: []string{
			"authorization",
			"x-trace-id",
			"user-agent",
		},
		BlockedHeaders: []string{
			"cookie",
			"set-cookie",
		},
		ForwardAll:    false,
		CaseSensitive: false,
	}

	filter := NewFilter(config)

	headers := map[string]string{
		"authorization": "Bearer token123",
		"x-trace-id":    "trace-123",
		"user-agent":    "test-agent",
		"cookie":        "session=abc123",
		"content-type":  "application/json",
		"random-header": "random-value",
	}

	filtered := filter.FilterHeaders(headers)

	expected := map[string]string{
		"authorization": "Bearer token123",
		"x-trace-id":    "trace-123",
		"user-agent":    "test-agent",
	}

	assert.Equal(t, expected, filtered)
}

func TestHeaderFilter_FilterHeaders_Disabled(t *testing.T) {
	config := config.HeaderForwardingConfig{
		Enabled: false,
	}

	filter := NewFilter(config)

	headers := map[string]string{
		"authorization": "Bearer token123",
		"x-trace-id":    "trace-123",
	}

	filtered := filter.FilterHeaders(headers)

	assert.Empty(t, filtered)
}

func TestHeaderFilter_FilterHeaders_ForwardAll(t *testing.T) {
	config := config.HeaderForwardingConfig{
		Enabled:        true,
		BlockedHeaders: []string{"cookie"},
		ForwardAll:     true,
		CaseSensitive:  false,
	}

	filter := NewFilter(config)

	headers := map[string]string{
		"authorization": "Bearer token123",
		"x-trace-id":    "trace-123",
		"cookie":        "session=abc123",
		"user-agent":    "test-agent",
	}

	filtered := filter.FilterHeaders(headers)

	expected := map[string]string{
		"authorization": "Bearer token123",
		"x-trace-id":    "trace-123",
		"user-agent":    "test-agent",
		// cookie should be blocked
	}

	assert.Equal(t, expected, filtered)
}

func TestHeaderFilter_GetMethods(t *testing.T) {
	config := config.HeaderForwardingConfig{
		Enabled: true,
		AllowedHeaders: []string{
			"authorization",
			"x-trace-id",
		},
		BlockedHeaders: []string{
			"cookie",
			"set-cookie",
		},
	}

	filter := NewFilter(config)

	assert.True(t, filter.IsEnabled())
	assert.Equal(t, []string{"authorization", "x-trace-id"}, filter.GetAllowedHeaders())
	assert.Equal(t, []string{"cookie", "set-cookie"}, filter.GetBlockedHeaders())
}

func TestDefaultConfiguration(t *testing.T) {
	// Test that the default configuration is sensible
	defaultConfig := config.Default()

	require.NotNil(t, defaultConfig.GRPC.HeaderForwarding)

	hf := defaultConfig.GRPC.HeaderForwarding
	assert.True(t, hf.Enabled)
	assert.False(t, hf.ForwardAll) // Should not forward all by default for security
	assert.False(t, hf.CaseSensitive)

	// Should have reasonable defaults for allowed headers
	assert.Contains(t, hf.AllowedHeaders, "authorization")
	assert.Contains(t, hf.AllowedHeaders, "x-trace-id")
	assert.Contains(t, hf.AllowedHeaders, "user-agent")

	// Should block dangerous headers
	assert.Contains(t, hf.BlockedHeaders, "cookie")
	assert.Contains(t, hf.BlockedHeaders, "set-cookie")
	assert.Contains(t, hf.BlockedHeaders, "host")
	assert.Contains(t, hf.BlockedHeaders, "content-length")
}
