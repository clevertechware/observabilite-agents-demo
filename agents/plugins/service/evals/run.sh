#!/usr/bin/env bash
# Usage: run.sh <version> <case> <run-number>
# Runs one case read-only with `claude -p` and stores the stream-json output outside the repository.
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

# --allowedTools only pre-approves; --permission-mode dontAsk and --tools keep the run read-only.
# A "*" in a Bash rule also matches spaces: pinning "-o json" keeps "ctwctl -o json orders create ... o2" refused.
workdir=$(mktemp -d)
trap 'rm -rf "$workdir"' EXIT
case_copy="$workdir/agents/plugins/service/evals/$case_name"
mkdir -p "$case_copy"
find "$evals_dir/$case_name" -maxdepth 1 -type f ! -name prompt.md -exec cp {} "$case_copy/" \;
cd "$workdir"
claude -p "$prompt" \
  --plugin-dir "$repo_root/agents/plugins/service" \
  --model sonnet \
  --output-format stream-json --verbose \
  --setting-sources project \
  --permission-mode dontAsk \
  --tools "Bash,Read,Skill" \
  --allowedTools "Bash(ctwctl o2 *)" "Bash(ctwctl -o json o2 *)" "Bash(date)" "Bash(date +%u)" "Read(./**)" Skill \
  --disallowedTools "Read(agents/plugins/service/evals/*/graders/**)" "Read(services/**)" "Read(deploy/**)" "Read(infra/**)" \
  --strict-mcp-config \
  --max-turns 40 \
  --max-budget-usd 5 \
  < /dev/null > "$out"
echo "$out"
