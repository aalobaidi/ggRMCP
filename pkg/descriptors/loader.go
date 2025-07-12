package descriptors

import (
	"fmt"
	"io"
	"os"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

// Loader handles loading and parsing FileDescriptorSet files
type Loader struct {
	logger *zap.Logger
	files  *protoregistry.Files
}

// NewLoader creates a new descriptor loader
func NewLoader(logger *zap.Logger) *Loader {
	return &Loader{
		logger: logger.Named("descriptors"),
		files:  &protoregistry.Files{},
	}
}

// LoadFromFile loads a FileDescriptorSet from a binary protobuf file
func (l *Loader) LoadFromFile(path string) (*descriptorpb.FileDescriptorSet, error) {
	l.logger.Info("Loading FileDescriptorSet", zap.String("path", path))

	// Open the file
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open descriptor file %s: %w", path, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			// File close errors are typically not critical in read operations
		}
	}()

	// Read the binary content
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read descriptor file %s: %w", path, err)
	}

	// Parse the FileDescriptorSet
	var fdSet descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(data, &fdSet); err != nil {
		return nil, fmt.Errorf("failed to unmarshal FileDescriptorSet from %s: %w", path, err)
	}

	l.logger.Info("Successfully loaded FileDescriptorSet",
		zap.String("path", path),
		zap.Int("fileCount", len(fdSet.File)))

	return &fdSet, nil
}

// BuildRegistry creates a protoregistry.Files from a FileDescriptorSet
func (l *Loader) BuildRegistry(fdSet *descriptorpb.FileDescriptorSet) (*protoregistry.Files, error) {
	files := &protoregistry.Files{}

	// Process files in dependency order
	processed := make(map[string]bool)
	var processFile func(*descriptorpb.FileDescriptorProto) error

	processFile = func(fdProto *descriptorpb.FileDescriptorProto) error {
		fileName := fdProto.GetName()
		if processed[fileName] {
			return nil
		}

		l.logger.Debug("Processing file descriptor", zap.String("file", fileName))

		// Process dependencies first
		for _, dep := range fdProto.Dependency {
			// Find dependency in the set
			var depFd *descriptorpb.FileDescriptorProto
			for _, f := range fdSet.File {
				if f.GetName() == dep {
					depFd = f
					break
				}
			}
			if depFd != nil {
				if err := processFile(depFd); err != nil {
					return err
				}
			} else {
				l.logger.Warn("Dependency not found in FileDescriptorSet",
					zap.String("file", fileName),
					zap.String("dependency", dep))
			}
		}

		// Create file descriptor
		fd, err := protodesc.NewFile(fdProto, files)
		if err != nil {
			// Try with global registry as resolver for well-known types
			fd, err = protodesc.NewFile(fdProto, protoregistry.GlobalFiles)
			if err != nil {
				return fmt.Errorf("failed to create file descriptor for %s: %w", fileName, err)
			}
		}

		// Register the file
		if err := files.RegisterFile(fd); err != nil {
			return fmt.Errorf("failed to register file descriptor for %s: %w", fileName, err)
		}

		processed[fileName] = true
		l.logger.Debug("Successfully processed file descriptor", zap.String("file", fileName))
		return nil
	}

	// Process all files
	for _, fdProto := range fdSet.File {
		if err := processFile(fdProto); err != nil {
			return nil, err
		}
	}

	l.logger.Info("Successfully built file registry",
		zap.Int("registeredFiles", len(fdSet.File)))

	return files, nil
}

// ExtractServiceInfo extracts service information with comments from file descriptors
func (l *Loader) ExtractServiceInfo(files *protoregistry.Files) ([]EnhancedServiceInfo, error) {
	var services []EnhancedServiceInfo

	// Iterate through all files in the registry
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		l.logger.Debug("Extracting services from file", zap.String("file", string(fd.FullName())))

		// Process each service in the file
		for i := 0; i < fd.Services().Len(); i++ {
			serviceDesc := fd.Services().Get(i)

			// Extract service name to match reflection discovery format
			// Convert from "com.example.hello.HelloService" to "hello.HelloService"
			fullName := string(serviceDesc.FullName())
			serviceName := extractServiceNameForCompatibility(fullName)

			serviceInfo := EnhancedServiceInfo{
				Name:        serviceName,
				Description: extractComments(serviceDesc),
				Methods:     []EnhancedMethodInfo{},
			}

			// Process each method in the service
			for j := 0; j < serviceDesc.Methods().Len(); j++ {
				methodDesc := serviceDesc.Methods().Get(j)

				methodInfo := EnhancedMethodInfo{
					Name:              string(methodDesc.Name()),
					FullName:          string(methodDesc.FullName()),
					Description:       extractComments(methodDesc),
					InputType:         string(methodDesc.Input().FullName()),
					OutputType:        string(methodDesc.Output().FullName()),
					InputDescriptor:   methodDesc.Input(),
					OutputDescriptor:  methodDesc.Output(),
					IsClientStreaming: methodDesc.IsStreamingClient(),
					IsServerStreaming: methodDesc.IsStreamingServer(),
				}

				serviceInfo.Methods = append(serviceInfo.Methods, methodInfo)
			}

			services = append(services, serviceInfo)
			l.logger.Debug("Extracted service",
				zap.String("service", serviceInfo.Name),
				zap.Int("methods", len(serviceInfo.Methods)))
		}

		return true // continue iteration
	})

	l.logger.Info("Extracted services from FileDescriptorSet",
		zap.Int("serviceCount", len(services)))

	return services, nil
}

// extractComments extracts leading and trailing comments from a descriptor
func extractComments(desc protoreflect.Descriptor) string {
	// Get source location info if available
	loc := desc.ParentFile().SourceLocations().ByDescriptor(desc)

	comments := ""

	// Leading comments
	if leading := loc.LeadingComments; leading != "" {
		comments = leading
	}

	// Trailing comments (append with newline if we have leading comments)
	if trailing := loc.TrailingComments; trailing != "" {
		if comments != "" {
			comments += "\n" + trailing
		} else {
			comments = trailing
		}
	}

	return comments
}

// extractServiceNameForCompatibility extracts service name to match reflection format
// Converts "com.example.hello.HelloService" to "hello.HelloService"
// This ensures compatibility between FileDescriptorSet and reflection discovery
func extractServiceNameForCompatibility(fullName string) string {
	// Split the full name by dots
	parts := strings.Split(fullName, ".")
	if len(parts) < 2 {
		// If there's no package, return as-is
		return fullName
	}

	// Take the last two parts (package.Service) to match reflection format
	// e.g., "com.example.hello.HelloService" -> "hello.HelloService"
	packageName := parts[len(parts)-2]
	serviceName := parts[len(parts)-1]

	return fmt.Sprintf("%s.%s", packageName, serviceName)
}

// EnhancedServiceInfo contains service information with comments
type EnhancedServiceInfo struct {
	Name        string
	Description string
	Methods     []EnhancedMethodInfo
}

// EnhancedMethodInfo contains method information with comments
type EnhancedMethodInfo struct {
	Name              string
	FullName          string
	Description       string
	InputType         string
	OutputType        string
	InputDescriptor   protoreflect.MessageDescriptor
	OutputDescriptor  protoreflect.MessageDescriptor
	IsClientStreaming bool
	IsServerStreaming bool
}
