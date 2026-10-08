package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func Setup(ctx context.Context) (func(context.Context) error, error) {
	res, err := resource.New(ctx, resource.WithFromEnv(), resource.WithTelemetrySDK())
	if err != nil {
		return nil, err
	}

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	spanExporter, err := autoexport.NewSpanExporter(ctx)
	if err != nil {
		return nil, err
	}
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(spanExporter),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tracerProvider)

	metricReader, err := autoexport.NewMetricReader(ctx)
	if err != nil {
		return nil, errors.Join(err, tracerProvider.Shutdown(ctx))
	}
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(metricReader),
	)
	otel.SetMeterProvider(meterProvider)

	logExporter, err := autoexport.NewLogExporter(ctx)
	if err != nil {
		return nil, errors.Join(err, tracerProvider.Shutdown(ctx), meterProvider.Shutdown(ctx))
	}
	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
	)
	otel.SetLoggerProvider(loggerProvider)
	slog.SetDefault(slog.New(slog.NewMultiHandler(
		slog.NewJSONHandler(os.Stdout, nil),
		otelslog.NewHandler("inventory-service"),
	)))

	return func(ctx context.Context) error {
		return errors.Join(
			tracerProvider.Shutdown(ctx),
			meterProvider.Shutdown(ctx),
			loggerProvider.Shutdown(ctx),
		)
	}, nil
}
