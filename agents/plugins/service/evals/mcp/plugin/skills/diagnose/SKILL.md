---
name: diagnose
description: Investigate an incident or an error reported by a user, either described in natural language or shown in a screenshot, by searching OpenObserve logs and traces through the OpenObserve MCP server. Produces a detailed report with the root cause and links to the error logs and to a trace in the OpenObserve UI. Use when someone reports that something failed, was slow or returned an error.
---

# Diagnose

Turn an incident report into a diagnosed report backed by OpenObserve logs and traces.

## Input

- Natural language: note the time, the action the user tried and the message they saw.
- Screenshot: read the image and extract the timestamp, the command or page, and the exact error message.

## Tools

Query OpenObserve with the tools of the `o2` MCP server:

- `StreamList` and `StreamSchema` list the streams and their fields.
- `SearchSQL` runs a SQL query. Put the SQL in `request_body.query.sql`, the window in `request_body.query.start_time`
  and `request_body.query.end_time` as epoch **microseconds**, the row limit in `request_body.query.size`, and the
  stream type (`logs`, `traces`) in `type`.
- `GetLatestTraces` lists the traces of a stream in a window, with the same microsecond timestamps.

Build the UI links with the `ctwctl` CLI, which only prints a URL and queries nothing:

```
ctwctl o2 link (--sql "<SQL>" | --trace-id <id>) [--from <RFC 3339> --to <RFC 3339>]
```

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
