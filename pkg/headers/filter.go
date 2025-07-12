package headers

import (
	"strings"

	"github.com/aalobaidi/ggRMCP/pkg/config"
)

// Filter handles header filtering based on configuration
type Filter struct {
	config config.HeaderForwardingConfig
}

// NewFilter creates a new header filter with the given configuration
func NewFilter(config config.HeaderForwardingConfig) *Filter {
	return &Filter{
		config: config,
	}
}

// ShouldForward determines if a header should be forwarded based on configuration
func (f *Filter) ShouldForward(headerName string) bool {
	if !f.config.Enabled {
		return false
	}

	// Normalize header name for comparison if not case sensitive
	name := headerName
	if !f.config.CaseSensitive {
		name = strings.ToLower(headerName)
	}

	// Check blocked headers first (takes precedence)
	for _, blocked := range f.config.BlockedHeaders {
		blockedName := blocked
		if !f.config.CaseSensitive {
			blockedName = strings.ToLower(blocked)
		}
		if name == blockedName {
			return false
		}
	}

	// If ForwardAll is enabled, forward unless blocked
	if f.config.ForwardAll {
		return true
	}

	// Check allowed headers
	for _, allowed := range f.config.AllowedHeaders {
		allowedName := allowed
		if !f.config.CaseSensitive {
			allowedName = strings.ToLower(allowed)
		}
		if name == allowedName {
			return true
		}
	}

	// Not in allowed list and ForwardAll is false
	return false
}

// FilterHeaders filters a map of headers, returning only those that should be forwarded
func (f *Filter) FilterHeaders(headers map[string]string) map[string]string {
	if !f.config.Enabled {
		return make(map[string]string)
	}

	filtered := make(map[string]string)
	for name, value := range headers {
		if f.ShouldForward(name) {
			filtered[name] = value
		}
	}

	return filtered
}

// GetAllowedHeaders returns the list of allowed headers
func (f *Filter) GetAllowedHeaders() []string {
	return f.config.AllowedHeaders
}

// GetBlockedHeaders returns the list of blocked headers
func (f *Filter) GetBlockedHeaders() []string {
	return f.config.BlockedHeaders
}

// IsEnabled returns whether header forwarding is enabled
func (f *Filter) IsEnabled() bool {
	return f.config.Enabled
}
