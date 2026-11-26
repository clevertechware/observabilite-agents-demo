---
name: monitor
description: Produce the daily health report of the three demo services (order-service, inventory-service, pricing-service) by scanning OpenObserve logs, traces and metrics with the ctwctl CLI. Covers the 5xx rate per service, P99 latency against fixed budgets, container restarts and error logs, and filters known noise. Use when someone asks for the morning health check of the services, or a health report over an explicit period.
---

# Monitor

Turn a window of telemetry into a short health report of the three services, ready to post as-is.

## Window

- Morning report without dates: scan the last 24 hours with `--since 24h`. On Monday, when `date +%u` prints `1`, scan the weekend with `--since 72h`.
- A period given in the request overrides the rules above: use its start and end as an absolute window `--from <RFC 3339> --to <RFC 3339>`, in the timezone of the request.
- Use the same window for every query and every link. With `--since`, run `date` once and compute the absolute `--from`/`--to` from it: use them for every query and for `ctwctl o2 link`.

## Tools

Use the `ctwctl` CLI:

```
ctwctl o2 query "<SQL>" [--type logs|traces|metrics] [--since 24h | --from <RFC 3339> --to <RFC 3339>] [--limit 100]
ctwctl o2 link (--sql "<SQL>" | --trace-id <id>) [--from <RFC 3339> --to <RFC 3339>]
```

Add `-o json` for machine-readable output. Run `ctwctl o2 --help` for the details. If a query fails on a field name, list the fields first (`ctwctl o2 streams`, or `SELECT * ... LIMIT 1`) and adjust.

## Signals

Namespace `demo`. Traces are sampled at 100 %, so the P99 is computed on the spans, without PromQL.

1. 5xx per service — `--type traces`:
   `SELECT service_name, http_route, count(*) AS n FROM "default" WHERE span_kind='2' AND http_response_status_code LIKE '5%' AND service_name IN ('order-service','inventory-service','pricing-service') GROUP BY service_name, http_route`
   Total per service: `SELECT service_name, count(*) AS total FROM "default" WHERE span_kind='2' AND service_name IN ('order-service','inventory-service','pricing-service') GROUP BY service_name`. Compute the rate against it, and the first and last hour with `histogram(_timestamp, '1 hour')`.
2. P99 latency — `--type traces`, `duration` is in microseconds:
   `SELECT service_name, approx_percentile_cont(duration, 0.99) AS p99_us FROM "default" WHERE span_kind='2' AND service_name IN ('order-service','inventory-service','pricing-service') GROUP BY service_name`
   Fixed budgets (P99): order-service 500 ms, inventory-service 300 ms, pricing-service 50 ms. A service is slow when it has 20 requests or more in the window and its P99 is over budget.
   Slowness origin: the client span that holds the time (database call, outbound HTTP call); 50 ms is the smallest budget, read the largest groups first:
   `SELECT service_name, operation_name, db_query_text, count(*) AS n, approx_percentile_cont(duration, 0.99) AS p99_us FROM "default" WHERE span_kind='3' AND duration > 50000 AND service_name IN ('order-service','inventory-service','pricing-service') GROUP BY service_name, operation_name, db_query_text ORDER BY n DESC`
3. Restarts — `--type metrics`, cumulative counter per pod, a jump inside the window is a restart. The reason label is set by the restart itself, so the counter is split in two series (`""` then `OOMKilled`): never group by the reason.
   `SELECT k8s_pod_name, k8s_container_name, max(value) - min(value) AS restarts, max(k8s_container_status_last_terminated_reason) AS reason FROM "k8s_container_restarts" WHERE k8s_namespace_name='demo' GROUP BY k8s_pod_uid, k8s_pod_name, k8s_container_name HAVING max(value) > min(value)`
   `max(reason)` is the alphabetical maximum, not the latest reason: with several restarts of different reasons, query the pod alone.
4. Error logs — `--type logs`:
   `SELECT service_name, coalesce(exception_type, body) AS signature, count(*) AS n FROM "default" WHERE severity_number >= 17 AND k8s_namespace_name='demo' GROUP BY service_name, signature ORDER BY n DESC`

## Noise

- Ignore 4xx: 409 insufficient stock, 429 rate limit and 403 allowlist are expected.
- A 5xx that follows a 5xx of a downstream call in the same trace is a consequence, not a cause: attribute the errors of the whole trace to the origin service, the deepest failing span. pricing 500 → inventory 500 → order 502 counts once, under pricing-service, and order-service appears as a consequence.
- A failed client span (`connection refused`, timeout) names its target service as the origin, even when that service recorded no span: a pod being killed answers nothing.
- Slowness propagates the same way: a server span slow because of a slow child span is a consequence. The origin is the deepest slow client span; report its service, operation and `db_query_text`. A service slow only because of its call to another service is listed as a consequence of that service.
- A rollout creates a new pod with a new `k8s_pod_uid` and no restart; only `max - min > 0` on the same pod is a restart.
- Health probes are already excluded from spans and metrics.

## Verdicts

One line per service, in this order; a service with no telemetry in the window is listed as such, never omitted.

- `critique`: 5xx rate of 5 % or more on 20 requests or more, or a restart with reason `OOMKilled`.
- `dégradé`: any 5xx below that rate, the service is slow, or any restart.
- `sain`: none of the above.

A service whose only 5xx are consequences of another service is `dégradé` at most, never `critique`; its line names the origin service. Its consequences are not added to the origin's count.

## Report

The report is the message itself: it starts with the title and ends with the last link. No code fence around it, no preamble, no notes to the requester after it. 5 000 characters at most (Rocket.Chat limit).

- a title with the window in full and its timezone
- one line per service with its verdict
- sections Erreurs (service, route, count, first and last hour), Redémarrages (pod, reason, count), Lenteurs (service, P99, budget)
- 3 to 5 links at most, built with `ctwctl o2 link`: the error logs of the main finding with `--sql`, a trace with `--trace-id`
- write it in the language of the request
