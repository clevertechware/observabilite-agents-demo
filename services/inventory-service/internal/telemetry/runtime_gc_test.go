package telemetry

import (
	"context"
	"math"
	"runtime"
	"runtime/metrics"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func collect(t *testing.T, reader *sdkmetric.ManualReader) map[string]metricdata.Metrics {
	t.Helper()
	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	byName := map[string]metricdata.Metrics{}
	for _, scope := range rm.ScopeMetrics {
		for _, m := range scope.Metrics {
			byName[m.Name] = m
		}
	}
	return byName
}

func sortedValues(m map[string][]string) map[string][]string {
	for _, v := range m {
		slices.Sort(v)
	}
	return m
}

func gcMetricsReader(t *testing.T) *sdkmetric.ManualReader {
	t.Helper()
	reader := sdkmetric.NewManualReader(sdkmetric.WithProducer(newRuntimeHistogramProducer()))
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { assert.NoError(t, provider.Shutdown(context.Background())) })
	require.NoError(t, registerGCMetrics(provider.Meter(scopeName)))
	return reader
}

func TestGCCyclesCounterIncreasesAfterAForcedCollection(t *testing.T) {
	reader := gcMetricsReader(t)
	before := collect(t, reader)["go.memory.gc.cycles"].Data.(metricdata.Sum[int64]).DataPoints[0].Value

	runtime.GC()

	cycles := collect(t, reader)["go.memory.gc.cycles"]
	sum := cycles.Data.(metricdata.Sum[int64])
	assert.True(t, sum.IsMonotonic)
	assert.Equal(t, "{gc_cycle}", cycles.Unit)
	assert.Greater(t, sum.DataPoints[0].Value, before)
}

func TestCPUTimeIsSplitByStateAndDetailedStateLikeTheSemanticConventions(t *testing.T) {
	reader := gcMetricsReader(t)
	runtime.GC()

	cpuTime := collect(t, reader)["go.cpu.time"]

	sum := cpuTime.Data.(metricdata.Sum[float64])
	assert.Equal(t, "s", cpuTime.Unit)
	detailedByState := map[string][]string{}
	var gcSeconds float64
	for _, dp := range sum.DataPoints {
		state, _ := dp.Attributes.Value(attribute.Key("go.cpu.state"))
		detail, _ := dp.Attributes.Value(attribute.Key("go.cpu.detailed_state"))
		detailedByState[state.AsString()] = append(detailedByState[state.AsString()], detail.AsString())
		if state.AsString() == "gc" {
			gcSeconds += dp.Value
		}
	}
	assert.Equal(t, map[string][]string{
		"user":     {"user"},
		"gc":       {"gc/mark/assist", "gc/mark/dedicated", "gc/mark/idle", "gc/pause"},
		"scavenge": {"scavenge/assist", "scavenge/background"},
		"idle":     {"idle"},
	}, sortedValues(detailedByState))
	assert.Positive(t, gcSeconds)
}

func TestProducerExportsGCPausesAndSchedulingLatencyAsCumulativeHistograms(t *testing.T) {
	reader := gcMetricsReader(t)
	runtime.GC()

	byName := collect(t, reader)

	for _, name := range []string{"go.memory.gc.pause.duration", "go.schedule.duration"} {
		h, ok := byName[name].Data.(metricdata.Histogram[float64])
		require.True(t, ok, name)
		assert.Equal(t, metricdata.CumulativeTemporality, h.Temporality, name)
		assert.Positive(t, h.DataPoints[0].Count, name)
	}
}

func TestHistogramConversionDropsTheLowerBoundAndTheExplicitInfinity(t *testing.T) {
	h := &metrics.Float64Histogram{
		Buckets: []float64{0, 1, 2, math.Inf(1)},
		Counts:  []uint64{3, 2, 1},
	}

	dp := toHistogramDataPoint(h, time.Unix(0, 0), time.Unix(60, 0))

	assert.Equal(t, []float64{1, 2}, dp.Bounds)
	assert.Equal(t, []uint64{3, 2, 1}, dp.BucketCounts)
	assert.Equal(t, uint64(6), dp.Count)
	assert.Equal(t, 1.0*2+2.0*1, dp.Sum)
}

func TestHistogramConversionAddsAnEmptyOverflowBucketWhenTheLastBoundIsFinite(t *testing.T) {
	h := &metrics.Float64Histogram{
		Buckets: []float64{0, 1, 2},
		Counts:  []uint64{4, 5},
	}

	dp := toHistogramDataPoint(h, time.Unix(0, 0), time.Unix(60, 0))

	assert.Equal(t, []float64{1, 2}, dp.Bounds)
	assert.Equal(t, []uint64{4, 5, 0}, dp.BucketCounts)
	assert.Equal(t, []uint64{4, 5}, h.Counts)
}
