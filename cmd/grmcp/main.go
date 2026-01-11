package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	appconfig "github.com/aalobaidi/ggRMCP/pkg/config"
	"github.com/aalobaidi/ggRMCP/pkg/grpc"
	"github.com/aalobaidi/ggRMCP/pkg/server"
	"github.com/aalobaidi/ggRMCP/pkg/session"
	"github.com/aalobaidi/ggRMCP/pkg/tools"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config holds application configuration
type Config struct {
	GRPCHost       string
	GRPCPort       int
	HTTPPort       int
	LogLevel       string
	Development    bool
	DescriptorPath string
}

// parseFlags parses command line flags
func parseFlags() *Config {
	config := &Config{}

	flag.StringVar(&config.GRPCHost, "grpc-host", "localhost", "gRPC server host")
	flag.IntVar(&config.GRPCPort, "grpc-port", 50051, "gRPC server port")
	flag.IntVar(&config.HTTPPort, "http-port", 50052, "HTTP server port")
	flag.StringVar(&config.LogLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	flag.BoolVar(&config.Development, "dev", false, "Enable development mode")
	flag.StringVar(&config.DescriptorPath, "descriptor", "", "Path to protobuf descriptor file (optional)")

	flag.Parse()

	return config
}

// setupLogger creates a configured logger
func setupLogger(config *Config) (*zap.Logger, error) {
	var zapConfig zap.Config

	if config.Development {
		zapConfig = zap.NewDevelopmentConfig()
		zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		zapConfig = zap.NewProductionConfig()
	}

	// Set log level
	switch config.LogLevel {
	case "debug":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		zapConfig.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	return zapConfig.Build()
}

// setupRouter creates the HTTP router with all routes
func setupRouter(handler *server.Handler) *mux.Router {
	router := mux.NewRouter()

	// Main MCP endpoint
	router.HandleFunc("/", handler.ServeHTTP).Methods("GET", "POST", "OPTIONS")

	// Health check endpoint
	router.HandleFunc("/health", handler.HealthHandler).Methods("GET", "HEAD")

	// Metrics endpoint
	router.HandleFunc("/metrics", handler.MetricsHandler).Methods("GET")

	return router
}

// gracefulShutdown handles graceful shutdown of the HTTP server
func gracefulShutdown(server *http.Server, logger *zap.Logger) {
	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Create a context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown the server
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}

func main() {
	// Parse command line flags
	config := parseFlags()

	// Setup logger
	logger, err := setupLogger(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup logger: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if syncErr := logger.Sync(); syncErr != nil {
			fmt.Fprintf(os.Stderr, "Failed to sync logger: %v\n", syncErr)
		}
	}()

	logger.Info("Starting GrMCP Gateway",
		zap.String("grpc_host", config.GRPCHost),
		zap.Int("grpc_port", config.GRPCPort),
		zap.Int("http_port", config.HTTPPort),
		zap.String("log_level", config.LogLevel),
		zap.Bool("development", config.Development))

	// Create service discoverer with FileDescriptorSet support
	descriptorConfig := appconfig.DescriptorSetConfig{
		Enabled:              config.DescriptorPath != "",
		Path:                 config.DescriptorPath,
		PreferOverReflection: false, // Use reflection as primary, descriptor as enhancement
		IncludeSourceInfo:    true,
	}

	serviceDiscoverer, err := grpc.NewServiceDiscoverer(
		config.GRPCHost,
		config.GRPCPort,
		logger,
		descriptorConfig,
	)
	if err != nil {
		logger.Fatal("Failed to create service discoverer", zap.Error(err))
	}

	// Connect to gRPC server
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := serviceDiscoverer.Connect(ctx); err != nil {
		logger.Fatal("Failed to connect to gRPC server", zap.Error(err))
	}
	defer func() {
		if err := serviceDiscoverer.Close(); err != nil {
			logger.Warn("Failed to close service discoverer", zap.Error(err))
		}
	}()

	// Discover services (will use FileDescriptorSet if available, fallback to reflection)
	if err := serviceDiscoverer.DiscoverServices(ctx); err != nil {
		logger.Fatal("Failed to discover services", zap.Error(err))
	}

	// Log service discovery completion
	stats := serviceDiscoverer.GetServiceStats()
	logger.Info("Service discovery completed",
		zap.Any("serviceCount", stats["serviceCount"]),
		zap.Int("methodCount", serviceDiscoverer.GetMethodCount()))

	// Create session manager
	sessionManager := session.NewManager(logger)
	defer func() {
		if err := sessionManager.Close(); err != nil {
			logger.Warn("Failed to close session manager", zap.Error(err))
		}
	}()

	// Create tool builder
	toolBuilder := tools.NewMCPToolBuilder(logger)

	// Create HTTP handler with default header forwarding config
	defaultConfig := appconfig.Default()
	handler := server.NewHandler(logger, serviceDiscoverer, sessionManager, toolBuilder, defaultConfig.GRPC.HeaderForwarding)

	// Setup router
	router := setupRouter(handler)

	// Apply middleware
	middlewares := server.DefaultMiddleware(logger)
	finalHandler := server.ChainMiddleware(middlewares...)(router)

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", config.HTTPPort),
		Handler:      finalHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		logger.Info("Starting HTTP server", zap.Int("port", config.HTTPPort))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start HTTP server", zap.Error(err))
		}
	}()

	// Wait for shutdown signal
	gracefulShutdown(httpServer, logger)
}
