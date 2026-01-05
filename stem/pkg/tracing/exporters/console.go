package exporters

import (
	"context"

	"go.opentelemetry.io/otel/sdk/trace"
)

type ConsoleExporter struct{}

func (c *ConsoleExporter) ExportSpans(ctx context.Context, spans []trace.ReadOnlySpan) error {
	return nil
}

func (c *ConsoleExporter) Shutdown(ctx context.Context) error {
	return nil
}

