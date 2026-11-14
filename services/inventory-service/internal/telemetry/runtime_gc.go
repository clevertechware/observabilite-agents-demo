package telemetry

import (
	"context"
	"math"
	"runtime/metrics"
	"time"

	otelruntime "go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/semconv/v1.43.0/goconv"
)

// The Go runtime metrics below are opt-in in the semantic conventions and are not emitted by contrib runtime
// v0.72.0. They reuse the names, units and attributes of the goconv package so that a later contrib release can
// replace this file without changing the dashboard.

const (
	scopeName = "github.com/clevertechware/observabilite-agents-demo/services/inventory-service/internal/telemetry"

	gcCyclesMetric = "/gc/cycles/total:gc-cycles"
	gcPausesMetric = "/sched/pauses/total/gc:seconds"
)

var cpuClasses = []struct {
	runtimeMetric string
	state         goconv.CPUStateAttr
	detailedState string
}{
	{"/cpu/classes/user:cpu-seconds", goconv.CPUStateUser, "user"},
	{"/cpu/classes/gc/mark/assist:cpu-seconds", goconv.CPUStateGC, "gc/mark/assist"},
	{"/cpu/classes/gc/mark/dedicated:cpu-seconds", goconv.CPUStateGC, "gc/mark/dedicated"},
	{"/cpu/classes/gc/mark/idle:cpu-seconds", goconv.CPUStateGC, "gc/mark/idle"},
	{"/cpu/classes/gc/pause:cpu-seconds", goconv.CPUStateGC, "gc/pause"},
	{"/cpu/classes/scavenge/assist:cpu-seconds", goconv.CPUStateScavenge, "scavenge/assist"},
	{"/cpu/classes/scavenge/background:cpu-seconds", goconv.CPUStateScavenge, "scavenge/background"},
	{"/cpu/classes/idle:cpu-seconds", goconv.CPUStateIdle, "idle"},
}

// registerGCMetrics publishes go.memory.gc.cycles and go.cpu.time, read from runtime/metrics at each collection.
func registerGCMetrics(meter metric.Meter) error {
	cycles, err := goconv.NewMemoryGCCyclesObservable(meter)
	if err != nil {
		return err
	}
	cpuTime, err := goconv.NewCPUTimeObservable(meter)
	if err != nil {
		return err
	}
	cpuAttributes := make([]metric.ObserveOption, len(cpuClasses))
	for i, class := range cpuClasses {
		cpuAttributes[i] = metric.WithAttributeSet(attribute.NewSet(
			cpuTime.AttrCPUState(class.state),
			cpuTime.AttrCPUDetailedState(class.detailedState),
		))
	}

	_, err = meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		samples := make([]metrics.Sample, 1+len(cpuClasses))
		samples[0].Name = gcCyclesMetric
		for i, class := range cpuClasses {
			samples[i+1].Name = class.runtimeMetric
		}
		metrics.Read(samples)

		o.ObserveInt64(cycles.Inst(), int64(samples[0].Value.Uint64()))
		for i := range cpuClasses {
			o.ObserveFloat64(cpuTime.Inst(), samples[i+1].Value.Float64(), cpuAttributes[i])
		}
		return nil
	}, cycles.Inst(), cpuTime.Inst())
	return err
}

// runtimeHistogramProducer exports the two runtime histograms that cannot be observed through a callback:
// go.schedule.duration, from contrib's producer, and go.memory.gc.pause.duration.
type runtimeHistogramProducer struct {
	schedule  *otelruntime.Producer
	startTime time.Time
}

func newRuntimeHistogramProducer() *runtimeHistogramProducer {
	return &runtimeHistogramProducer{schedule: otelruntime.NewProducer(), startTime: time.Now()}
}

func (p *runtimeHistogramProducer) Produce(ctx context.Context) ([]metricdata.ScopeMetrics, error) {
	scopes, err := p.schedule.Produce(ctx)
	if err != nil {
		return nil, err
	}

	sample := []metrics.Sample{{Name: gcPausesMetric}}
	metrics.Read(sample)
	pauses := goconv.MemoryGCPauseDuration{}
	return append(scopes, metricdata.ScopeMetrics{
		Scope: instrumentation.Scope{Name: scopeName},
		Metrics: []metricdata.Metrics{{
			Name:        pauses.Name(),
			Description: pauses.Description(),
			Unit:        pauses.Unit(),
			Data: metricdata.Histogram[float64]{
				Temporality: metricdata.CumulativeTemporality,
				DataPoints: []metricdata.HistogramDataPoint[float64]{
					toHistogramDataPoint(sample[0].Value.Float64Histogram(), p.startTime, time.Now()),
				},
			},
		}},
	}), nil
}

var _ sdkmetric.Producer = (*runtimeHistogramProducer)(nil)

// toHistogramDataPoint converts a runtime histogram, whose Buckets holds len(Counts)+1 boundaries including the
// lower bound of the first bucket, into an OpenTelemetry data point, whose Bounds only holds upper bounds and
// leaves +Inf implicit. The sum is a lower estimate: each observation counts as its bucket's lower bound.
func toHistogramDataPoint(h *metrics.Float64Histogram, start, now time.Time) metricdata.HistogramDataPoint[float64] {
	bounds := h.Buckets[1:]
	counts := h.Counts
	if math.IsInf(bounds[len(bounds)-1], 1) {
		bounds = bounds[:len(bounds)-1]
	} else {
		counts = append(counts[:len(counts):len(counts)], 0)
	}

	var count uint64
	var sum float64
	for i, c := range counts {
		count += c
		if i > 0 {
			sum += bounds[i-1] * float64(c)
		}
	}
	return metricdata.HistogramDataPoint[float64]{
		StartTime:    start,
		Time:         now,
		Count:        count,
		Sum:          sum,
		Bounds:       bounds,
		BucketCounts: counts,
	}
}
