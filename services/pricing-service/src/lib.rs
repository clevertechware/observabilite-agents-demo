use axum::{routing::get, Router};
use opentelemetry_instrumentation_tower::http::server::Layer as OtelLayer;

mod catalog;
mod error;
pub mod faults;
mod prices;
pub mod telemetry;

use faults::Faults;

pub fn app(faults: Faults) -> Router {
    Router::new()
        .route("/prices/{sku}", get(prices::get_price))
        .with_state(faults)
        .layer(OtelLayer::new())
        .route("/healthz", get(|| async { "ok" }))
}
