package config

import (
	"fmt"
	"time"
)

// Config holds all configuration for the ggRMCP application
type Config struct {
	// Server configuration
	Server ServerConfig `json:"server" yaml:"server"`

	// gRPC configuration
	GRPC GRPCConfig `json:"grpc" yaml:"grpc"`

	// MCP configuration
	MCP MCPConfig `json:"mcp" yaml:"mcp"`

	// Session configuration
	Session SessionConfig `json:"session" yaml:"session"`

	// Tools configuration
	Tools ToolsConfig `json:"tools" yaml:"tools"`

	// Logging configuration
	Logging LoggingConfig `json:"logging" yaml:"logging"`
}

// ServerConfig contains HTTP server settings
type ServerConfig struct {
	// HTTP server port
	Port int `json:"port" yaml:"port"`

	// Request timeout
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	// Maximum request size
	MaxRequestSize int64 `json:"max_request_size" yaml:"max_request_size"`

	// Security headers configuration
	Security SecurityConfig `json:"security" yaml:"security"`
}

// SecurityConfig contains security-related settings
type SecurityConfig struct {
	// Enable security headers
	EnableHeaders bool `json:"enable_headers" yaml:"enable_headers"`

	// CORS settings
	CORS CORSConfig `json:"cors" yaml:"cors"`

	// Rate limiting
	RateLimit RateLimitConfig `json:"rate_limit" yaml:"rate_limit"`
}

// CORSConfig contains CORS settings
type CORSConfig struct {
	AllowedOrigins []string `json:"allowed_origins" yaml:"allowed_origins"`
	AllowedMethods []string `json:"allowed_methods" yaml:"allowed_methods"`
	AllowedHeaders []string `json:"allowed_headers" yaml:"allowed_headers"`
}

// RateLimitConfig contains rate limiting settings
type RateLimitConfig struct {
	RequestsPerMinute int           `json:"requests_per_minute" yaml:"requests_per_minute"`
	BurstSize         int           `json:"burst_size" yaml:"burst_size"`
	WindowSize        time.Duration `json:"window_size" yaml:"window_size"`
}

// GRPCConfig contains gRPC client settings
type GRPCConfig struct {
	// gRPC server host
	Host string `json:"host" yaml:"host"`

	// gRPC server port
	Port int `json:"port" yaml:"port"`

	// Connection timeout
	ConnectTimeout time.Duration `json:"connect_timeout" yaml:"connect_timeout"`

	// Request timeout
	RequestTimeout time.Duration `json:"request_timeout" yaml:"request_timeout"`

	// Keep-alive settings
	KeepAlive KeepAliveConfig `json:"keep_alive" yaml:"keep_alive"`

	// Reconnection settings
	Reconnect ReconnectConfig `json:"reconnect" yaml:"reconnect"`

	// Message size limits
	MaxMessageSize int `json:"max_message_size" yaml:"max_message_size"`

	// Header forwarding configuration
	HeaderForwarding HeaderForwardingConfig `json:"header_forwarding" yaml:"header_forwarding"`

	// FileDescriptorSet configuration
	DescriptorSet DescriptorSetConfig `json:"descriptor_set" yaml:"descriptor_set"`
}

// KeepAliveConfig contains keep-alive settings
type KeepAliveConfig struct {
	Time                time.Duration `json:"time" yaml:"time"`
	Timeout             time.Duration `json:"timeout" yaml:"timeout"`
	PermitWithoutStream bool          `json:"permit_without_stream" yaml:"permit_without_stream"`
}

// ReconnectConfig contains reconnection settings
type ReconnectConfig struct {
	Interval    time.Duration `json:"interval" yaml:"interval"`
	MaxAttempts int           `json:"max_attempts" yaml:"max_attempts"`
}

// HeaderForwardingConfig contains header forwarding settings
type HeaderForwardingConfig struct {
	// Enable header forwarding
	Enabled bool `json:"enabled" yaml:"enabled"`

	// List of headers to forward to gRPC server
	AllowedHeaders []string `json:"allowed_headers" yaml:"allowed_headers"`

	// List of headers to block (takes precedence over allowed)
	BlockedHeaders []string `json:"blocked_headers" yaml:"blocked_headers"`

	// Whether to forward all headers by default (not recommended for security)
	ForwardAll bool `json:"forward_all" yaml:"forward_all"`

	// Case sensitive header matching
	CaseSensitive bool `json:"case_sensitive" yaml:"case_sensitive"`
}

// DescriptorSetConfig contains FileDescriptorSet settings
type DescriptorSetConfig struct {
	// Enable FileDescriptorSet support
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Path to the FileDescriptorSet file (.binpb)
	Path string `json:"path" yaml:"path"`

	// Prefer descriptor set over reflection (if both available)
	PreferOverReflection bool `json:"prefer_over_reflection" yaml:"prefer_over_reflection"`

	// Include source location info for comment extraction
	IncludeSourceInfo bool `json:"include_source_info" yaml:"include_source_info"`
}

// MCPConfig contains MCP protocol settings
type MCPConfig struct {
	// Validation limits
	Validation ValidationConfig `json:"validation" yaml:"validation"`

	// Protocol version
	ProtocolVersion string `json:"protocol_version" yaml:"protocol_version"`
}

// ValidationConfig contains validation limits
type ValidationConfig struct {
	MaxFieldLength    int   `json:"max_field_length" yaml:"max_field_length"`
	MaxToolNameLength int   `json:"max_tool_name_length" yaml:"max_tool_name_length"`
	MaxRequestSize    int64 `json:"max_request_size" yaml:"max_request_size"`
	MaxResponseSize   int64 `json:"max_response_size" yaml:"max_response_size"`
}

// SessionConfig contains session management settings
type SessionConfig struct {
	// Session expiration time
	Expiration time.Duration `json:"expiration" yaml:"expiration"`

	// Cleanup interval
	CleanupInterval time.Duration `json:"cleanup_interval" yaml:"cleanup_interval"`

	// Maximum number of concurrent sessions
	MaxSessions int `json:"max_sessions" yaml:"max_sessions"`

	// Session rate limiting
	RateLimit SessionRateLimitConfig `json:"rate_limit" yaml:"rate_limit"`
}

// SessionRateLimitConfig contains session-specific rate limiting
type SessionRateLimitConfig struct {
	RequestsPerMinute int           `json:"requests_per_minute" yaml:"requests_per_minute"`
	BurstSize         int           `json:"burst_size" yaml:"burst_size"`
	WindowSize        time.Duration `json:"window_size" yaml:"window_size"`
}

// ToolsConfig contains tool building settings
type ToolsConfig struct {
	// Schema cache settings
	Cache CacheConfig `json:"cache" yaml:"cache"`

	// Schema generation limits
	MaxDepth      int `json:"max_depth" yaml:"max_depth"`
	MaxFields     int `json:"max_fields" yaml:"max_fields"`
	MaxEnumValues int `json:"max_enum_values" yaml:"max_enum_values"`
}

// CacheConfig contains caching settings
type CacheConfig struct {
	Enabled    bool          `json:"enabled" yaml:"enabled"`
	TTL        time.Duration `json:"ttl" yaml:"ttl"`
	MaxEntries int           `json:"max_entries" yaml:"max_entries"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level       string `json:"level" yaml:"level"`
	Format      string `json:"format" yaml:"format"`
	Development bool   `json:"development" yaml:"development"`
}

// Default returns a configuration with sensible defaults
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Port:           50053,
			Timeout:        30 * time.Second,
			MaxRequestSize: 4 * 1024 * 1024, // 4MB
			Security: SecurityConfig{
				EnableHeaders: true,
				CORS: CORSConfig{
					AllowedOrigins: []string{"*"},
					AllowedMethods: []string{"GET", "POST", "OPTIONS"},
					AllowedHeaders: []string{"Content-Type", "Authorization", "Mcp-Session-Id"},
				},
				RateLimit: RateLimitConfig{
					RequestsPerMinute: 1000,
					BurstSize:         100,
					WindowSize:        time.Minute,
				},
			},
		},
		GRPC: GRPCConfig{
			Host:           "localhost",
			Port:           50051,
			ConnectTimeout: 5 * time.Second,
			RequestTimeout: 30 * time.Second,
			KeepAlive: KeepAliveConfig{
				Time:                10 * time.Second,
				Timeout:             5 * time.Second,
				PermitWithoutStream: true,
			},
			Reconnect: ReconnectConfig{
				Interval:    5 * time.Second,
				MaxAttempts: 5,
			},
			MaxMessageSize: 4 * 1024 * 1024, // 4MB
			HeaderForwarding: HeaderForwardingConfig{
				Enabled: true,
				AllowedHeaders: []string{
					"authorization",
					"x-trace-id",
					"x-user-id",
					"x-request-id",
					"user-agent",
					"x-forwarded-for",
					"x-real-ip",
				},
				BlockedHeaders: []string{
					"cookie",
					"set-cookie",
					"host",
					"content-length",
					"content-type",
					"connection",
					"upgrade",
					"mcp-session-id",
				},
				ForwardAll:    false,
				CaseSensitive: false,
			},
			DescriptorSet: DescriptorSetConfig{
				Enabled:              false, // Disabled by default
				Path:                 "",
				PreferOverReflection: false,
				IncludeSourceInfo:    true,
			},
		},
		MCP: MCPConfig{
			ProtocolVersion: "2024-11-05",
			Validation: ValidationConfig{
				MaxFieldLength:    1024,
				MaxToolNameLength: 128,
				MaxRequestSize:    4 * 1024 * 1024,  // 4MB
				MaxResponseSize:   16 * 1024 * 1024, // 16MB
			},
		},
		Session: SessionConfig{
			Expiration:      30 * time.Minute,
			CleanupInterval: 5 * time.Minute,
			MaxSessions:     10000,
			RateLimit: SessionRateLimitConfig{
				RequestsPerMinute: 100,
				BurstSize:         20,
				WindowSize:        time.Minute,
			},
		},
		Tools: ToolsConfig{
			Cache: CacheConfig{
				Enabled:    true,
				TTL:        1 * time.Hour,
				MaxEntries: 1000,
			},
			MaxDepth:      10,
			MaxFields:     100,
			MaxEnumValues: 50,
		},
		Logging: LoggingConfig{
			Level:       "info",
			Format:      "json",
			Development: false,
		},
	}
}

// Development returns a configuration suitable for development
func Development() *Config {
	config := Default()

	// Override development-specific settings
	config.Logging.Level = "debug"
	config.Logging.Development = true
	config.Server.Security.CORS.AllowedOrigins = []string{"http://localhost:3000", "http://127.0.0.1:3000"}
	config.Session.RateLimit.RequestsPerMinute = 1000 // Higher limit for development

	return config
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.GRPC.Port <= 0 || c.GRPC.Port > 65535 {
		return fmt.Errorf("invalid gRPC port: %d", c.GRPC.Port)
	}

	if c.Server.Timeout <= 0 {
		return fmt.Errorf("server timeout must be positive")
	}

	if c.GRPC.ConnectTimeout <= 0 {
		return fmt.Errorf("gRPC connect timeout must be positive")
	}

	if c.Session.MaxSessions <= 0 {
		return fmt.Errorf("max sessions must be positive")
	}

	// Validate descriptor set configuration
	if c.GRPC.DescriptorSet.Enabled {
		if c.GRPC.DescriptorSet.Path == "" {
			return fmt.Errorf("descriptor set path must be specified when enabled")
		}
	}

	return nil
}
