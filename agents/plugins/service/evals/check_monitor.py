#!/usr/bin/env python3
"""Évalue les assertions mécaniques d'un run du skill monitor et imprime un résumé.

Usage: check_monitor.py <case> <run>
<run> est le stream-json claude (run.sh) ou le répertoire exporté par vibe (run_vibe.sh).
Les assertions sémantiques (constats attendus, format du rapport) restent à lire dans le rapport.
"""
import glob
import json
import os
import re
import sys
from datetime import datetime, timedelta, timezone

SKILL = r"(?:^|:)monitor$"
O2 = r"https://o2\.obs\.scw\.clevertechware\.fr"
PARIS = timezone(timedelta(hours=1))
WINDOWS = {
    "health-morning": (datetime(2026, 11, 25, 0, 0, tzinfo=PARIS), datetime(2026, 11, 26, 0, 0, tzinfo=PARIS)),
    "health-incident": (datetime(2026, 11, 26, 11, 0, tzinfo=PARIS), datetime(2026, 11, 26, 13, 30, tzinfo=PARIS)),
    "health-faults": (datetime(2026, 11, 26, 16, 30, tzinfo=PARIS), datetime(2026, 11, 26, 18, 0, tzinfo=PARIS)),
}
SERVICES = ("order-service", "inventory-service", "pricing-service")
FROM_TO = re.compile(r"ctwctl (?:-o \S+ )?o2 query\b[^;|&\n]*--from (\S+)[^;|&\n]*--to (\S+)")
FAILED = re.compile(r"unknown field|Search field not found|ctwctl: ")


def parse(raw):
    raw = raw.strip("'\"")
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


POD = re.compile(r"[a-z]+-service-[0-9a-f]+-[a-z0-9]+")
SECTION = r"^[#*\s]*{}\b"


def section(text, title):
    """Texte de la section `title`, jusqu'au titre suivant (Erreurs, Redémarrages, Lenteurs ou Liens)."""
    start = re.search(SECTION.format(title), text, re.M)
    if not start:
        return ""
    rest = text[start.end():]
    following = re.search(SECTION.format(r"(?:Erreurs|Redémarrages|Lenteurs|Liens)"), rest, re.M)
    return rest[: following.start()] if following else rest


CONSEQUENCE = r"conséquence|répercu|propag|à cause|dû à|due à|en aval|en amont|causée?s? par"
ROLLOUT_PODS = re.compile(r"inventory-service-[0-9a-f]+-[a-z0-9]+|pricing-service-6cd479f7b6-[a-z0-9]+")


def fault_checks(text):
    """Assertions propres au cas health-faults : lenteur d'inventory, OOMKilled de pricing, rollouts exclus."""
    restarts = section(text, "Redémarrages")
    slowness = section(text, "Lenteurs")
    return {
        "inventory-slowness-over-p99-budget": bool(
            re.search(r"(?s)inventory-service.{0,300}?(?:2[.,]\d+\s*s\b|2\s?\d{3}\s*ms)", slowness)
            and re.search(r"300\s*ms", slowness)
        ),
        "order-slowness-presented-as-consequence": bool(
            re.search(rf"(?is)order-service.{{0,250}}(?:{CONSEQUENCE})|(?:{CONSEQUENCE}).{{0,250}}order-service", slowness)
        ),
        "slow-sql-query-mentioned": bool(re.search(r"pg_sleep|s\.quantity", text)),
        "pricing-named-as-origin": bool(
            re.search(r"(?im)^[^\n]*origine[^\n]*pricing-service|^[^\n]*pricing-service[^\n]*origine", text)
        ),
        "pricing-oomkilled-around-1751": bool(re.search(r"(?s)OOMKilled.{0,400}?17[:h]5[01]|17[:h]5[01].{0,400}?OOMKilled", text)),
        "oomkilled-pod-listed-as-restart": "pricing-service-666cb5f467-nwn7r" in restarts,
        "rollout-pods-not-listed-as-restarts": not ROLLOUT_PODS.search(restarts),
    }


def inventory_not_sain(text):
    """La première ligne qui cite inventory-service porte son verdict : il ne doit pas être `sain`."""
    line = next((line for line in text.splitlines() if "inventory-service" in line), "")
    return bool(line) and not re.search(r"\bsain\b", line)


def is_raw_message(text, start):
    """Le rapport est le message : ni clôture de code, ni préambule avant le titre qui porte la date de début."""
    first = next((line for line in text.splitlines() if line.strip()), "")
    return "```" not in text and start.strftime("%d/%m/%Y") in first


def load_claude(path):
    with open(path, encoding="utf-8") as fh:
        events = [json.loads(line) for line in fh if line.strip()]
    final = next((e for e in reversed(events) if e.get("type") == "result"), {})

    commands, skills, bodies, failed = [], [], [], []
    for ev in events:
        if ev.get("type") == "assistant":
            for block in ev["message"]["content"]:
                if block.get("type") != "tool_use":
                    continue
                if block["name"] == "Bash":
                    commands.append(block["input"].get("command", ""))
                if block["name"] == "Skill":
                    skills.append(block["input"].get("skill", ""))
        elif ev.get("type") == "user" and not isinstance(ev["message"]["content"], str):
            for block in ev["message"]["content"]:
                if block.get("type") != "tool_result":
                    continue
                body = block.get("content")
                if isinstance(body, list):
                    body = "".join(p.get("text", "") for p in body)
                body = body or ""
                bodies.append(body)
                if block.get("is_error") or FAILED.search(body[:300]):
                    failed.append(body)
    stats = f"num_turns={final.get('num_turns')} total_cost_usd={final.get('total_cost_usd')} is_error={final.get('is_error')}"
    return final.get("result", "") or "", commands, skills, "\n".join(bodies), failed, stats


def load_vibe(run_dir):
    events = []
    for path in sorted(glob.glob(os.path.join(run_dir, "session", "journal", "*.jsonl"))):
        with open(path, encoding="utf-8") as fh:
            events.extend(json.loads(line) for line in fh if line.strip())

    commands, skills, bodies, failed = [], [], [], []
    for ev in events:
        payload = ev.get("payload", {})
        if ev["type"] == "action_intent" and payload.get("kind") == "tool":
            call = payload.get("request", {}).get("call", {})
            name, args = call.get("name", ""), call.get("arguments") or {}
            if name.endswith("bash"):
                commands.append(args.get("command", ""))
            if ".skill" in name or name.startswith("skill"):
                skills.append(args.get("name") or "")
        elif ev["type"] == "action_result":
            res = payload.get("result") or {}
            if "call_id" not in res:
                continue
            inner = res.get("result")
            if not isinstance(inner, dict) or "hook" in inner:
                continue
            structured = inner.get("structured_content") or {}
            body = ""
            if isinstance(structured, dict):
                body = f"{structured.get('stdout', '')}\n{structured.get('stderr', '')}"
            if not body and isinstance(inner.get("content"), list):
                body = "\n".join(p.get("text", "") for p in inner["content"] if isinstance(p, dict))
            bodies.append(body)
            if inner.get("type") != "success" or FAILED.search(body[:300]):
                failed.append(body or str(inner.get("type")))

    text = ""
    report = os.path.join(run_dir, "report.txt")
    if os.path.isfile(report):
        with open(report, encoding="utf-8") as fh:
            text = fh.read()

    export = {}
    export_path = os.path.join(run_dir, "export.json")
    if os.path.isfile(export_path):
        with open(export_path, encoding="utf-8") as fh:
            export = json.load(fh)
    usage = export.get("usage", {}) or {}
    warnings = export.get("warnings") or []
    stats = (
        f"steps={export.get('steps')} cost_usd={export.get('cost_usd')}"
        f" out_tokens={usage.get('output_tokens')} warnings={len(warnings)}"
    )
    for w in warnings:
        failed.append(str(w))
    return text, commands, skills, "\n".join(bodies), failed, stats


def main():
    case, path = sys.argv[1], sys.argv[2]
    if os.path.isdir(path):
        text, commands, skills, bodies, failed, stats = load_vibe(path)
    else:
        text, commands, skills, bodies, failed, stats = load_claude(path)

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
        checks["report-is-the-raw-message"] = is_raw_message(text, start)
        checks["restart-signal-queried"] = any("k8s_container_restarts" in c for c in commands)
        checks["error-logs-signal-queried"] = any("severity_number >= 17" in c for c in commands)
        checks["p99-signal-queried"] = any("approx_percentile_cont" in c for c in commands)
        checks["restart-mentioned"] = bool(re.search(r"redémarr|restart", text, re.I))
        urls = set(re.findall(O2 + r"/web/[^\s)>`\"']+", text))
        checks["links-built-by-ctwctl"] = (
            not urls or any(re.search(r"o2 link\b.*(?:--sql|--trace-id)", c, re.S) for c in commands)
        )
        checks["every-url-comes-from-ctwctl"] = bool(urls) and all(u in bodies for u in urls)
        if case == "health-faults":
            checks.update(fault_checks(text))
        if case == "health-incident":
            checks["inventory-not-sain"] = inventory_not_sain(text)
            checks["both-error-episodes-timed"] = bool(
                re.search(r"12[:h]0", text) and re.search(r"13[:h]2", text)
            )
    else:
        print(f"unknown case '{case}'")
        return False

    for name, ok in checks.items():
        print(f"{'PASS' if ok else 'FAIL'}  {name}")
    print(f"report_chars={len(text)} commands={len(commands)} failed_tool_results={len(failed)}")
    for f in failed:
        print(f"  ! {str(f)[:200].replace(chr(10), ' ')}")
    print(stats)
    return all(checks.values())


if __name__ == "__main__":
    sys.exit(0 if main() else 1)
