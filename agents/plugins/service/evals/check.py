#!/usr/bin/env python3
"""Évalue les assertions mécaniques d'un run stream-json et imprime un résumé.

Usage: check.py <case> <run.jsonl>
Les assertions sémantiques (cause racine, fenêtre, champs inventés) restent à lire dans le rapport.
"""
import json
import re
import sys
from datetime import datetime, timedelta, timezone

SKILL = r"(?:^|:)diagnose$"
O2 = r"https://o2\.obs\.scw\.clevertechware\.fr"


INCIDENT = datetime(2026, 11, 26, 12, 1, 43, tzinfo=timezone(timedelta(hours=1)))
FROM_FLAG = re.compile(r"ctwctl (?:-o \S+ )?o2 query\b[^;|&\n]*--from (\S+)")


def starts_before_incident(command):
    """True when an o2 query window opens within the hour before the incident, in any UTC offset."""
    for raw in FROM_FLAG.findall(command):
        try:
            start = datetime.fromisoformat(raw.replace("Z", "+00:00"))
        except ValueError:
            continue
        if start.tzinfo is not None and INCIDENT - timedelta(hours=1) <= start <= INCIDENT:
            return True
    return False


def search_sql_starts_before_incident(call):
    """True when a SearchSQL call of the o2 MCP server opens its window within the hour before the incident."""
    query = call["input"].get("request_body", {}).get("query", {})
    start_us = query.get("start_time")
    if not isinstance(start_us, int):
        return False
    start = datetime.fromtimestamp(start_us / 1_000_000, tz=timezone.utc)
    return INCIDENT - timedelta(hours=1) <= start <= INCIDENT


def load(path):
    with open(path, encoding="utf-8") as fh:
        return [json.loads(line) for line in fh if line.strip()]


def tool_calls(events):
    for ev in events:
        if ev.get("type") != "assistant":
            continue
        for block in ev["message"]["content"]:
            if block.get("type") == "tool_use":
                yield block


def tool_results(events):
    for ev in events:
        if ev.get("type") != "user":
            continue
        content = ev["message"]["content"]
        if isinstance(content, str):
            continue
        for block in content:
            if block.get("type") == "tool_result":
                body = block.get("content")
                if isinstance(body, list):
                    body = "".join(part.get("text", "") for part in body)
                yield block, body or ""


def main():
    case, path = sys.argv[1], sys.argv[2]
    events = load(path)
    final = next((e for e in reversed(events) if e.get("type") == "result"), {})
    text = final.get("result", "") or ""
    calls = list(tool_calls(events))
    commands = [c["input"].get("command", "") for c in calls if c["name"] == "Bash"]
    skills = [c["input"].get("skill", "") for c in calls if c["name"] == "Skill"]
    results_text = "\n".join(body for _, body in tool_results(events))

    checks = {}
    if case == "not-diagnose":
        checks["skill-not-invoked"] = not any(re.search(SKILL, s) for s in skills)
        checks["no-observability-query"] = not any("ctwctl o2" in c for c in commands)
    else:
        checks["skill-invoked"] = any(re.search(SKILL, s) for s in skills)
        forward = r"traefik[\s\S]{0,400}order-service[\s\S]{0,400}inventory-service[\s\S]{0,400}pricing-service"
        backward = r"pricing-service[\s\S]{0,400}inventory-service[\s\S]{0,400}order-service[\s\S]{0,400}traefik"
        checks["call-chain"] = bool(re.search(forward + "|" + backward, text, re.I))
        checks["logs-link"] = bool(re.search(O2 + r"/web/logs\?", text))
        checks["trace-link"] = bool(re.search(O2 + r"/web/traces/trace-details\?[^\s)]*trace_id=[0-9a-f]{32}", text))
        checks["logs-link-built-by-ctwctl"] = any(re.search(r"o2 link\b.*--sql", c, re.S) for c in commands)
        checks["trace-link-built-by-ctwctl"] = any(re.search(r"o2 link\b.*--trace-id", c, re.S) for c in commands)
        urls = set(re.findall(O2 + r"/web/[^\s)>`\"']+", text))
        checks["every-url-comes-from-ctwctl"] = bool(urls) and all(u in results_text for u in urls)
        searches = [c for c in calls if c["name"] == "mcp__o2__SearchSQL"]
        checks["window-uses-absolute-range"] = any(starts_before_incident(c) for c in commands) or any(
            search_sql_starts_before_incident(c) for c in searches
        )
        if case == "order-error-screenshot":
            checks["reads-image"] = any(c["name"] == "Read" and "erreur-commande.png" in json.dumps(c["input"]) for c in calls)
            checks["uses-screenshot-time"] = "12:01:43" in text

    failed_queries = [
        body[:200].replace("\n", " ")
        for blk, body in tool_results(events)
        if blk.get("is_error") or re.search(r"unknown field|Search field not found|ctwctl: ", body[:300])
    ]

    for name, ok in checks.items():
        print(f"{'PASS' if ok else 'FAIL'}  {name}")
    print(f"commands={len(commands)} failed_tool_results={len(failed_queries)}")
    for f in failed_queries:
        print(f"  ! {f}")
    print(f"num_turns={final.get('num_turns')} total_cost_usd={final.get('total_cost_usd')} is_error={final.get('is_error')}")
    return all(checks.values())


if __name__ == "__main__":
    sys.exit(0 if main() else 1)
