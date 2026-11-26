---
name: monitor
description: Investigate an incident or an error reported by a user, either described in natural language or shown in a screenshot, by searching OpenObserve logs and traces with the ctwctl CLI. Produces a detailed report with the root cause and links to the error logs and to a trace in the OpenObserve UI. Use when someone reports that something failed, was slow or returned an error.
---

# Monitor

Turn an incident report into a diagnosed report backed by OpenObserve logs and traces.

## Input

- Natural language: note the time, the action the user tried and the message they saw.
- Screenshot: read the image and extract the timestamp, the command or page, and the exact error message.

## Tools

Use the `ctwctl` CLI:

```
ctwctl o2 streams
ctwctl o2 logs --service <name> --level error --since 1h
ctwctl o2 query "<SQL>" [--type logs|traces] [--since 1h | --from <RFC 3339> --to <RFC 3339>] [--limit 100]
ctwctl o2 link (--sql "<SQL>" | --trace-id <id>) [--from <RFC 3339> --to <RFC 3339>]
```

Add `-o json` for machine-readable output. Run `ctwctl o2 --help` for the details.

## Steps

1. Find the incident time from the report.
2. Search the error logs around that time.
3. Find a trace involved in the errors and follow it across services.
4. Identify the root cause.
5. Build the links with `ctwctl o2 link`.
6. Write the report.

## Report

Write it in the language of the request, with:

- a summary with the root cause
- a timeline
- the call chain between services
- the evidence: error log lines and trace ids
- a link to the error logs and a link to a trace
- what to do next
