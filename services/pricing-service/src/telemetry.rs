use opentelemetry::global;
use opentelemetry_appender_tracing::layer::OpenTelemetryTracingBridge;
use opentelemetry_otlp::{ExporterBuildError, LogExporter, MetricExporter, SpanExporter};
use opentelemetry_sdk::{
    error::OTelSdkError, logs::SdkLoggerProvider, metrics::SdkMeterProvider,
    propagation::TraceContextPropagator, trace::SdkTracerProvider, Resource,
};
use tracing_subscriber::{
    fmt::MakeWriter, layer::SubscriberExt, util::SubscriberInitExt, util::TryInitError, EnvFilter,
};

#[derive(Debug, thiserror::Error)]
pub enum TelemetryError {
    #[error("cannot build the OTLP exporter: {0}")]
    Exporter(#[from] ExporterBuildError),
    #[error("cannot install the tracing subscriber: {0}")]
    Subscriber(#[from] TryInitError),
    #[error("cannot shut down the telemetry providers: {0}")]
    Shutdown(#[from] OTelSdkError),
}

pub struct Telemetry {
    tracer: SdkTracerProvider,
    meter: SdkMeterProvider,
    logger: SdkLoggerProvider,
}

pub fn init_telemetry() -> Result<Telemetry, TelemetryError> {
    let resource = Resource::builder().build();

    let tracer = SdkTracerProvider::builder()
        .with_resource(resource.clone())
        .with_batch_exporter(SpanExporter::builder().with_http().build()?)
        .build();
    let meter = SdkMeterProvider::builder()
        .with_resource(resource.clone())
        .with_periodic_exporter(MetricExporter::builder().with_http().build()?)
        .build();
    let logger = SdkLoggerProvider::builder()
        .with_resource(resource)
        .with_batch_exporter(LogExporter::builder().with_http().build()?)
        .build();

    Telemetry::install(tracer, meter, logger)
}

impl Telemetry {
    pub fn install(
        tracer: SdkTracerProvider,
        meter: SdkMeterProvider,
        logger: SdkLoggerProvider,
    ) -> Result<Self, TelemetryError> {
        Self::install_with_writer(tracer, meter, logger, std::io::stdout)
    }

    pub fn install_with_writer<W>(
        tracer: SdkTracerProvider,
        meter: SdkMeterProvider,
        logger: SdkLoggerProvider,
        writer: W,
    ) -> Result<Self, TelemetryError>
    where
        W: for<'a> MakeWriter<'a> + Send + Sync + 'static,
    {
        global::set_tracer_provider(tracer.clone());
        global::set_meter_provider(meter.clone());
        global::set_text_map_propagator(TraceContextPropagator::new());

        let filter = EnvFilter::try_from_default_env().unwrap_or_else(|_| EnvFilter::new("info"));
        tracing_subscriber::registry()
            .with(filter)
            .with(tracing_subscriber::fmt::layer().json().with_writer(writer))
            .with(OpenTelemetryTracingBridge::new(&logger))
            .try_init()?;

        Ok(Self {
            tracer,
            meter,
            logger,
        })
    }

    pub fn shutdown(self) -> Result<(), TelemetryError> {
        let results = [
            self.tracer.shutdown(),
            self.meter.shutdown(),
            self.logger.shutdown(),
        ];
        results.into_iter().collect::<Result<(), _>>()?;
        Ok(())
    }
}
