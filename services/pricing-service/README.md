# pricing-service

Service Rust (Axum) sans état de la série OpenTelemetry. Il calcule un prix unitaire à partir d'une table en mémoire
(`SKU-001` à `SKU-003`) et d'une remise par quantité. Il émet traces, métriques et logs en OTLP/HTTP.

## Ce que l'exemple montre

- trois providers OpenTelemetry (traces, métriques, logs) configurés par l'environnement, un propagateur W3C posé
  explicitement (`src/telemetry.rs`) ;
- une couche tower (`opentelemetry-instrumentation-tower`, feature `axum`) qui produit le span serveur et la métrique
  `http.server.request.duration` avec `http.route` (`src/lib.rs`) ;
- des logs `tracing` corrélés au span par `opentelemetry-appender-tracing` (`src/prices.rs`) ;
- deux pannes injectables (`src/faults.rs`) ;
- des tests avec exporters en mémoire (`tests/telemetry.rs`).

## Prérequis

- Rust 1.95 ou plus récent (l'image utilise 1.99.0) ;
- Docker pour construire l'image.

## API

| Route | Réponse |
|---|---|
| `GET /prices/{sku}?quantity=n` | 200 `{"sku","quantity","unitPrice"}`, 404 si le SKU est inconnu, 400 si `quantity` est absent, invalide ou nul |
| `GET /healthz` | 200 `ok`, non tracé |

Remises : 5 % à partir de 10 unités, 10 % à partir de 50, 15 % à partir de 100.

## Variables d'environnement

| Variable | Défaut | Rôle |
|---|---|---|
| `LISTEN_ADDR` | `0.0.0.0:8080` | adresse d'écoute |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://localhost:4318` | Collector ; `/v1/traces`, `/v1/metrics`, `/v1/logs` sont ajoutés |
| `OTEL_SERVICE_NAME` | `unknown_service:pricing-service` | `service.name` |
| `OTEL_RESOURCE_ATTRIBUTES` | vide | attributs de ressource `clé=valeur,...` |
| `RUST_LOG` | `info` | filtre des logs |
| `FAULT_ERROR_RATE` | `0` | proportion de réponses 500, entre 0 et 1 |
| `FAULT_LEAK_BYTES_PER_REQUEST` | `0` | octets retenus à chaque requête |

Une valeur invalide pour `FAULT_*` arrête le démarrage avec un message d'erreur.

## Build, lint, tests

```bash
cargo build --release --locked
cargo fmt --check
cargo clippy --all-targets -- -D warnings
cargo test
```

## Lancer

```bash
OTEL_SERVICE_NAME=pricing-service LISTEN_ADDR=127.0.0.1:8080 cargo run --release
curl 'http://127.0.0.1:8080/prices/SKU-002?quantity=50'
```

## Image

```bash
docker build -t pricing-service:local .
docker run --rm --memory 64m -p 8080:8080 -e FAULT_LEAK_BYTES_PER_REQUEST=1048576 pricing-service:local
```

Avec une limite de 64 Mio et 1 Mio retenu par requête, le conteneur est tué (`OOMKilled`, code 137) après une soixantaine
de requêtes.
