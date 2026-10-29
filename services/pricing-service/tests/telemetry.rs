use std::{
    io,
    sync::{Arc, Mutex, OnceLock},
};

use axum::{body::Body, http::Request, http::StatusCode, Router};
use opentelemetry::{
    trace::{SpanId, TraceId},
    Key, KeyValue, Value,
};
use opentelemetry_sdk::{
    logs::{InMemoryLogExporter, SdkLoggerProvider},
    metrics::{
        data::{AggregatedMetrics, MetricData},
        InMemoryMetricExporter, InMemoryMetricExporterBuilder, SdkMeterProvider, Temporality,
    },
    trace::{InMemorySpanExporter, SdkTracerProvider, SpanData},
};
use pricing_service::{app, faults::Faults, telemetry::Telemetry};
use serial_test::serial;
use tower::ServiceExt;

const TRACE_ID: &str = "0af7651916cd43dd8448eb211c80319c";
const TRACEPARENT: &str = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01";

#[derive(Clone, Default)]
struct StdoutCapture(Arc<Mutex<Vec<u8>>>);

impl io::Write for StdoutCapture {
    fn write(&mut self, buf: &[u8]) -> io::Result<usize> {
        self.0
            .lock()
            .map_err(|_| io::Error::other("capture buffer poisoned"))?
            .extend_from_slice(buf);
        Ok(buf.len())
    }

    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

impl StdoutCapture {
    fn json_lines(&self) -> Vec<serde_json::Value> {
        let bytes = self.0.lock().expect("capture buffer must not be poisoned");
        String::from_utf8_lossy(&bytes)
            .lines()
            .map(|line| serde_json::from_str(line).expect("every stdout line must be JSON"))
            .collect()
    }

    fn clear(&self) {
        self.0
            .lock()
            .expect("capture buffer must not be poisoned")
            .clear();
    }
}

struct Harness {
    stdout: StdoutCapture,
    spans: InMemorySpanExporter,
    metrics: InMemoryMetricExporter,
    logs: InMemoryLogExporter,
    meter: SdkMeterProvider,
    _telemetry: Telemetry,
}

fn harness() -> &'static Harness {
    static HARNESS: OnceLock<Harness> = OnceLock::new();
    HARNESS.get_or_init(|| {
        let spans = InMemorySpanExporter::default();
        let metrics = InMemoryMetricExporterBuilder::new()
            .with_temporality(Temporality::Delta)
            .build();
        let logs = InMemoryLogExporter::default();
        let tracer = SdkTracerProvider::builder()
            .with_simple_exporter(spans.clone())
            .build();
        let meter = SdkMeterProvider::builder()
            .with_periodic_exporter(metrics.clone())
            .build();
        let logger = SdkLoggerProvider::builder()
            .with_simple_exporter(logs.clone())
            .build();
        let stdout = StdoutCapture::default();
        let writer = stdout.clone();
        let telemetry =
            Telemetry::install_with_writer(tracer, meter.clone(), logger, move || writer.clone())
                .expect("telemetry must install once per test binary");
        Harness {
            stdout,
            spans,
            metrics,
            logs,
            meter,
            _telemetry: telemetry,
        }
    })
}

impl Harness {
    fn start(&self, faults: Faults) -> Router {
        self.spans.reset();
        self.meter.force_flush().expect("meter must flush");
        self.metrics.reset();
        self.logs.reset();
        self.stdout.clear();
        app(faults)
    }

    fn server_spans(&self) -> Vec<SpanData> {
        self.spans
            .get_finished_spans()
            .expect("spans must be readable")
    }

    fn duration_data_points_attributes(&self) -> Vec<Vec<KeyValue>> {
        self.meter.force_flush().expect("meter must flush");
        let mut attributes = Vec::new();
        for resource_metrics in self
            .metrics
            .get_finished_metrics()
            .expect("metrics must be readable")
        {
            for scope in resource_metrics.scope_metrics() {
                for metric in scope
                    .metrics()
                    .filter(|m| m.name() == "http.server.request.duration")
                {
                    if let AggregatedMetrics::F64(MetricData::Histogram(histogram)) = metric.data()
                    {
                        attributes.extend(
                            histogram
                                .data_points()
                                .map(|p| p.attributes().cloned().collect()),
                        );
                    }
                }
            }
        }
        attributes
    }
}

async fn get(app: &Router, uri: &str) -> StatusCode {
    get_with_headers(app, uri, &[]).await
}

async fn get_with_headers(app: &Router, uri: &str, headers: &[(&str, &str)]) -> StatusCode {
    let mut request = Request::get(uri);
    for (name, value) in headers {
        request = request.header(*name, *value);
    }
    let request = request.body(Body::empty()).expect("request must be valid");
    app.clone()
        .oneshot(request)
        .await
        .expect("router is infallible")
        .status()
}

fn attribute<'a>(attributes: &'a [KeyValue], key: &str) -> Option<&'a Value> {
    attributes
        .iter()
        .find(|kv| kv.key == Key::new(key.to_owned()))
        .map(|kv| &kv.value)
}

#[tokio::test]
#[serial]
async fn server_span_carries_the_route_pattern_and_the_status_code() {
    let app = harness().start(Faults::default());

    let status = get(&app, "/prices/SKU-001?quantity=2").await;

    assert_eq!(status, StatusCode::OK);
    let spans = harness().server_spans();
    assert_eq!(spans.len(), 1);
    assert_eq!(spans[0].name, "GET /prices/{sku}");
    assert_eq!(
        attribute(&spans[0].attributes, "http.route"),
        Some(&Value::from("/prices/{sku}"))
    );
    assert_eq!(
        attribute(&spans[0].attributes, "http.response.status_code"),
        Some(&Value::from(200_i64))
    );
}

#[tokio::test]
#[serial]
async fn unknown_sku_returns_404_and_the_span_records_it() {
    let app = harness().start(Faults::default());

    let status = get(&app, "/prices/SKU-999?quantity=1").await;

    assert_eq!(status, StatusCode::NOT_FOUND);
    let spans = harness().server_spans();
    assert_eq!(
        attribute(&spans[0].attributes, "http.response.status_code"),
        Some(&Value::from(404_i64))
    );
}

#[tokio::test]
#[serial]
async fn incoming_traceparent_is_the_parent_of_the_server_span() {
    let app = harness().start(Faults::default());

    get_with_headers(
        &app,
        "/prices/SKU-001?quantity=1",
        &[("traceparent", TRACEPARENT)],
    )
    .await;

    let spans = harness().server_spans();
    assert_eq!(
        spans[0].span_context.trace_id(),
        TraceId::from_hex(TRACE_ID).expect("valid trace id")
    );
    assert_eq!(
        spans[0].parent_span_id,
        SpanId::from_hex("b7ad6b7169203331").expect("valid span id")
    );
    assert!(spans[0].parent_span_is_remote);
}

#[tokio::test]
#[serial]
async fn healthz_is_not_traced() {
    let app = harness().start(Faults::default());

    let status = get(&app, "/healthz").await;

    assert_eq!(status, StatusCode::OK);
    assert!(harness().server_spans().is_empty());
}

#[tokio::test]
#[serial]
async fn request_duration_metric_carries_the_route_and_the_status_code() {
    let app = harness().start(Faults::default());

    get(&app, "/prices/SKU-002?quantity=3").await;

    let data_points = harness().duration_data_points_attributes();
    assert_eq!(data_points.len(), 1);
    assert_eq!(
        attribute(&data_points[0], "http.route"),
        Some(&Value::from("/prices/{sku}"))
    );
    assert_eq!(
        attribute(&data_points[0], "http.request.method"),
        Some(&Value::from("GET"))
    );
    assert_eq!(
        attribute(&data_points[0], "http.response.status_code"),
        Some(&Value::from(200_i64))
    );
}

#[tokio::test]
#[serial]
async fn log_emitted_in_the_handler_carries_the_trace_and_span_ids_of_the_server_span() {
    let app = harness().start(Faults::default());

    get_with_headers(
        &app,
        "/prices/SKU-001?quantity=1",
        &[("traceparent", TRACEPARENT)],
    )
    .await;

    let logs = harness()
        .logs
        .get_emitted_logs()
        .expect("logs must be readable");
    let price_log = logs
        .iter()
        .find(|log| format!("{:?}", log.record.body()).contains("price computed"))
        .expect("the handler must log the computed price");
    let context = price_log
        .record
        .trace_context()
        .expect("the log must be correlated");
    let server_span = &harness().server_spans()[0];
    assert_eq!(
        context.trace_id,
        TraceId::from_hex(TRACE_ID).expect("valid trace id")
    );
    assert_eq!(context.span_id, server_span.span_context.span_id());
}

#[tokio::test]
#[serial]
async fn error_rate_of_one_returns_500_and_marks_the_span_as_error() {
    let faults = Faults::new(1.0, 0).expect("valid faults");
    let app = harness().start(faults);

    let status = get(&app, "/prices/SKU-001?quantity=1").await;

    assert_eq!(status, StatusCode::INTERNAL_SERVER_ERROR);
    let spans = harness().server_spans();
    assert_eq!(
        attribute(&spans[0].attributes, "http.response.status_code"),
        Some(&Value::from(500_i64))
    );
    assert!(matches!(
        spans[0].status,
        opentelemetry::trace::Status::Error { .. }
    ));
}

#[tokio::test]
#[serial]
async fn each_request_grows_the_retained_memory_by_the_configured_leak() {
    let faults = Faults::new(0.0, 4096).expect("valid faults");
    let app = harness().start(faults.clone());

    for _ in 0..3 {
        get(&app, "/prices/SKU-001?quantity=1").await;
    }

    assert_eq!(faults.leaked_bytes(), 3 * 4096);
}

#[tokio::test]
#[serial]
async fn handler_log_is_written_to_stdout_as_json_even_without_a_collector() {
    let app = harness().start(Faults::default());

    get(&app, "/prices/SKU-001?quantity=1").await;

    let lines = harness().stdout.json_lines();
    let price_log = lines
        .iter()
        .find(|line| line["fields"]["message"] == "price computed")
        .expect("the handler log must reach stdout");
    assert_eq!(price_log["level"], "INFO");
}

#[tokio::test]
#[serial]
async fn http_client_logs_stay_off_the_otlp_export_to_avoid_a_telemetry_loop() {
    let _app = harness().start(Faults::default());

    tracing::info!(target: "reqwest::connect", "connecting to the collector");
    tracing::info!(target: "hyper_util::client", "pooling the collector connection");
    tracing::info!(target: "pricing_service::probe", "application log");

    let exported: Vec<String> = harness()
        .logs
        .get_emitted_logs()
        .expect("logs must be readable")
        .iter()
        .map(|log| format!("{:?}", log.record.body()))
        .collect();
    assert!(exported.iter().any(|body| body.contains("application log")));
    assert!(!exported.iter().any(|body| body.contains("collector")));
    let on_stdout = harness().stdout.json_lines();
    assert!(on_stdout
        .iter()
        .any(|line| line["fields"]["message"] == "connecting to the collector"));
}
