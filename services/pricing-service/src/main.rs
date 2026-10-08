use std::error::Error;
use std::process::ExitCode;

use pricing_service::{app, faults::Faults, telemetry};

const DEFAULT_LISTEN_ADDR: &str = "0.0.0.0:8080";

#[tokio::main]
async fn main() -> ExitCode {
    match run().await {
        Ok(()) => ExitCode::SUCCESS,
        Err(error) => {
            eprintln!("pricing-service: {error}");
            ExitCode::FAILURE
        }
    }
}

async fn run() -> Result<(), Box<dyn Error>> {
    let faults = Faults::from_env()?;
    let telemetry = telemetry::init_telemetry()?;

    let listen_addr =
        std::env::var("LISTEN_ADDR").unwrap_or_else(|_| DEFAULT_LISTEN_ADDR.to_string());
    let listener = tokio::net::TcpListener::bind(&listen_addr).await?;
    tracing::info!(%listen_addr, "pricing-service started");

    axum::serve(listener, app(faults))
        .with_graceful_shutdown(shutdown_signal())
        .await?;

    telemetry.shutdown()?;
    Ok(())
}

async fn shutdown_signal() {
    let ctrl_c = async {
        let _ = tokio::signal::ctrl_c().await;
    };

    #[cfg(unix)]
    let terminate = async {
        match tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate()) {
            Ok(mut sigterm) => {
                sigterm.recv().await;
            }
            Err(_) => std::future::pending::<()>().await,
        }
    };
    #[cfg(not(unix))]
    let terminate = std::future::pending::<()>();

    tokio::select! {
        () = ctrl_c => {},
        () = terminate => {},
    }
}
