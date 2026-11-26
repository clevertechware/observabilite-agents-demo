#!/usr/bin/env python3
"""Évalue les assertions mécaniques d'un run stream-json du skill monitor et imprime un résumé.

Usage: check_monitor.py <case> <run.jsonl>
Les assertions sémantiques (constats attendus, format du rapport) restent à lire dans le rapport.
"""
import json
import re
import sys
from datetime import datetime, timedelta, timezone

SKILL = r"(?:^|:)monitor$"
O2 = r"https://o2\.obs\.scw\.clevertechware\.fr"
PARIS = timezone(timedelta(hours=1))
WINDOWS = {
    "health-morning": (datetime(2026, 11, 25, 0, 0, tzinfo=PARIS), datetime(2026, 11, 26, 0, 0, tzinfo=PARIS)),
    "health-incident": (datetime(2026, 11, 26, 11, 0, tzinfo=PARIS), datetime(2026, 11, 26, 13, 30, tzinfo=PARIS)),
}
SERVICES = ("order-service", "inventory-service", "pricing-service")
FROM_TO = re.compile(r"ctwctl (?:-o \S+ )?o2 query\b[^;|&\n]*--from (\S+)[^;|&\n]*--to (\S+)")


def parse(raw):
    try:
        return datetime.fromisoformat(raw.replace("Z", "+00:00"))
    except ValueError:
        return None


def covers_requested_window(command, start, end):
    """True quand une requête o2 couvre la fenêtre demandée (marge de 5 minutes)."""
    for raw_from, raw_to in FROM_TO.findall(command):
        begin, last = parse(raw_from), parse(raw_to)
        if begin is None or last is None:
            continue
        if begin <= start + timedelta(minutes=5) and last >= end - timedelta(minutes=5):
            return True
    return False


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
    if case == "not-monitor":
        checks["skill-not-invoked"] = not any(re.search(SKILL, s) for s in skills)
        checks["no-observability-query"] = not any("ctwctl o2" in c for c in commands)
    elif case in WINDOWS:
        start, end = WINDOWS[case]
        checks["skill-invoked"] = any(re.search(SKILL, s) for s in skills)
        checks["window-covers-requested-range"] = any(
            covers_requested_window(c, start, end) for c in commands
        )
        checks["three-services-listed"] = all(s in text for s in SERVICES)
        checks["report-under-5000"] = len(text) <= 5000
        checks["restart-signal-queried"] = any("k8s_container_restarts" in c for c in commands)
        checks["error-logs-signal-queried"] = any("severity_number >= 17" in c for c in commands)
        checks["p99-signal-queried"] = any("approx_percentile_cont" in c for c in commands)
        checks["restart-mentioned"] = bool(re.search(r"redémarr|restart", text, re.I))
        urls = set(re.findall(O2 + r"/web/[^\s)>`\"']+", text))
        checks["links-built-by-ctwctl"] = (
            not urls or any(re.search(r"o2 link\b.*(?:--sql|--trace-id)", c, re.S) for c in commands)
        )
        checks["every-url-comes-from-ctwctl"] = bool(urls) and all(u in results_text for u in urls)
        if case == "health-incident":
            checks["both-error-episodes-timed"] = bool(
                re.search(r"12[:h]0", text) and re.search(r"13[:h]2", text)
            )
    else:
        print(f"unknown case '{case}'")
        return False

    failed_queries = [
        body[:200].replace("\n", " ")
        for blk, body in tool_results(events)
        if blk.get("is_error") or re.search(r"unknown field|Search field not found|ctwctl: ", body[:300])
    ]

    for name, ok in checks.items():
        print(f"{'PASS' if ok else 'FAIL'}  {name}")
    print(f"report_chars={len(text)} commands={len(commands)} failed_tool_results={len(failed_queries)}")
    for f in failed_queries:
        print(f"  ! {f}")
    print(f"num_turns={final.get('num_turns')} total_cost_usd={final.get('total_cost_usd')} is_error={final.get('is_error')}")
    return all(checks.values())


if __name__ == "__main__":
    sys.exit(0 if main() else 1)
