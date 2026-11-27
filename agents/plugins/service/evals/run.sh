#!/usr/bin/env bash
# Usage: run.sh <version> <case> <run-number>
# Runs one case read-only with `claude -p` and stores the stream-json output outside the repository.
# EVAL_ACCESS=mcp swaps the ctwctl queries for the OpenObserve MCP server (evals/mcp): the same case then runs
# with the MCP variant of the diagnose skill, and ctwctl only builds the UI links. EVAL_MCP_TOOL_SEARCH=1 adds the
# ToolSearch tool, so Claude Code defers the MCP tool definitions instead of loading them on the first call.
# The agent works in an empty temporary directory holding only the case files: Claude Code runs read-only
# commands such as grep or git grep without asking inside its working directory, which would expose the code
# and the fault injection of the demo.
# Prerequisites: ctwctl built (make -C cli/ctwctl build), SKILL_RUNS_DIR and CTWCTL_O2_TOKEN_FILE exported.
set -euo pipefail

version=${1:?version}
case_name=${2:?case}
run=${3:?run number}

evals_dir=$(cd "$(dirname "$0")" && pwd)
repo_root=$(cd "$evals_dir/../../../.." && pwd)
out_root=${SKILL_RUNS_DIR:?SKILL_RUNS_DIR must point outside the repository}
case $(cd "$out_root" 2>/dev/null && pwd -P || echo "$out_root")/ in
  "$repo_root"/*)
    echo "run.sh: SKILL_RUNS_DIR must be outside the repository" >&2
    exit 2
    ;;
esac
token_file=${CTWCTL_O2_TOKEN_FILE:?CTWCTL_O2_TOKEN_FILE must name a file holding the token}
prompt_file="$evals_dir/$case_name/prompt.md"

if [[ ! -f $prompt_file ]]; then
  echo "run.sh: unknown case '$case_name'" >&2
  exit 2
fi

mkdir -p "$out_root/$version"
out="$out_root/$version/$case_name-$run.jsonl"

prompt=$(awk 'BEGIN{n=0} /^---$/{n++; next} n>=2' "$prompt_file")

export PATH="$repo_root/cli/ctwctl/bin:$PATH"
export CTWCTL_O2_URL=${CTWCTL_O2_URL:-https://o2.obs.scw.clevertechware.fr}
export CTWCTL_O2_USER=${CTWCTL_O2_USER:-ctwctl@sa.internal}
CTWCTL_O2_TOKEN=$(cat "$token_file")
export CTWCTL_O2_TOKEN
export CLAUDE_CODE_DISABLE_CLAUDE_MDS=1 CLAUDE_CODE_DISABLE_AUTO_MEMORY=1

plugin_dir=$repo_root/agents/plugins/service
builtin_tools=Bash,Read,Skill
mcp_args=(--strict-mcp-config)
access_tools=("Bash(ctwctl o2 *)" "Bash(ctwctl -o json o2 *)")
if [[ ${EVAL_ACCESS:-cli} == mcp ]]; then
  O2_MCP_AUTHORIZATION="Basic $(printf '%s:%s' "$CTWCTL_O2_USER" "$CTWCTL_O2_TOKEN" | base64 | tr -d '\n')"
  export O2_MCP_AUTHORIZATION
  plugin_dir=$evals_dir/mcp/plugin
  mcp_args=(--strict-mcp-config --mcp-config "$evals_dir/mcp/o2.mcp.json")
  access_tools=("Bash(ctwctl o2 link *)" mcp__o2__StreamList mcp__o2__StreamSchema mcp__o2__SearchSQL
    mcp__o2__GetLatestTraces)
  if [[ ${EVAL_MCP_TOOL_SEARCH:-0} == 1 ]]; then
    builtin_tools=$builtin_tools,ToolSearch
    access_tools+=(ToolSearch)
  fi
fi

# --allowedTools only pre-approves; --permission-mode dontAsk and --tools keep the run read-only.
# A "*" in a Bash rule also matches spaces: pinning "-o json" keeps "ctwctl -o json orders create ... o2" refused.
workdir=$(mktemp -d)
trap 'rm -rf "$workdir"' EXIT
case_copy="$workdir/agents/plugins/service/evals/$case_name"
mkdir -p "$case_copy"
find "$evals_dir/$case_name" -maxdepth 1 -type f ! -name prompt.md -exec cp {} "$case_copy/" \;
cd "$workdir"
claude -p "$prompt" \
  --plugin-dir "$plugin_dir" \
  --model sonnet \
  --output-format stream-json --verbose \
  --setting-sources project \
  --permission-mode dontAsk \
  --tools "$builtin_tools" \
  --allowedTools "${access_tools[@]}" "Bash(date)" "Bash(date +%u)" "Read(./**)" Skill \
  --disallowedTools "Read(agents/plugins/service/evals/*/graders/**)" "Read(services/**)" "Read(deploy/**)" "Read(infra/**)" \
    mcp__o2__tools_call \
  "${mcp_args[@]}" \
  --max-turns 40 \
  --max-budget-usd 5 \
  < /dev/null > "$out"
echo "$out"
