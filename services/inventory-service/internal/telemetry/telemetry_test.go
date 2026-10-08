package telemetry_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"

	"github.com/clevertechware/observabilite-agents-demo/services/inventory-service/internal/telemetry"
)

func TestSetupInstallsW3CTraceContextAndBaggagePropagator(t *testing.T) {
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_METRICS_EXPORTER", "none")
	t.Setenv("OTEL_LOGS_EXPORTER", "none")

	shutdown, err := telemetry.Setup(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, shutdown(context.Background())) })

	assert.ElementsMatch(t, []string{"traceparent", "tracestate", "baggage"}, otel.GetTextMapPropagator().Fields())
}

func TestSetupFailsOnUnknownExporter(t *testing.T) {
	t.Setenv("OTEL_TRACES_EXPORTER", "carrier-pigeon")

	_, err := telemetry.Setup(context.Background())

	assert.Error(t, err)
}
