package exporters

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// OTLPConfig holds configuration for the OTLP exporter
type OTLPConfig struct {
	// Endpoint is the OTLP collector endpoint (e.g., "localhost:4317" for gRPC, "localhost:4318" for HTTP)
	Endpoint string

	// Protocol is either "grpc" or "http"
	Protocol string

	// Insecure disables TLS (for local development)
	Insecure bool

	// Headers to include with each request
	Headers map[string]string

	// Timeout for the exporter
	Timeout time.Duration
}

// DefaultOTLPConfig returns a default configuration for local development
func DefaultOTLPConfig() OTLPConfig {
	return OTLPConfig{
		Endpoint: "localhost:4317",
		Protocol: "grpc",
		Insecure: true,
		Timeout:  10 * time.Second,
	}
}

// NewOTLPExporter creates a new OTLP trace exporter
func NewOTLPExporter(ctx context.Context, config OTLPConfig) (*otlptrace.Exporter, error) {
	switch config.Protocol {
	case "grpc":
		return newGRPCExporter(ctx, config)
	case "http":
		return newHTTPExporter(ctx, config)
	default:
		return nil, fmt.Errorf("unsupported OTLP protocol: %s (use 'grpc' or 'http')", config.Protocol)
	}
}

// newGRPCExporter creates a gRPC-based OTLP exporter
func newGRPCExporter(ctx context.Context, config OTLPConfig) (*otlptrace.Exporter, error) {
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(config.Endpoint),
		otlptracegrpc.WithTimeout(config.Timeout),
	}

	if config.Insecure {
		opts = append(opts, otlptracegrpc.WithDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())))
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	if len(config.Headers) > 0 {
		opts = append(opts, otlptracegrpc.WithHeaders(config.Headers))
	}

	return otlptracegrpc.New(ctx, opts...)
}

// newHTTPExporter creates an HTTP-based OTLP exporter
func newHTTPExporter(ctx context.Context, config OTLPConfig) (*otlptrace.Exporter, error) {
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(config.Endpoint),
		otlptracehttp.WithTimeout(config.Timeout),
	}

	if config.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	if len(config.Headers) > 0 {
		opts = append(opts, otlptracehttp.WithHeaders(config.Headers))
	}

	return otlptracehttp.New(ctx, opts...)
}

