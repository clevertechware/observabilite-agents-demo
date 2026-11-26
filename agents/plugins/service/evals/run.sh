#!/usr/bin/env bash
# Usage: run.sh <version> <case> <run-number>
# Lance un cas en lecture seule avec `claude -p` et enregistre le flux stream-json hors du dépôt.
# Prérequis : ctwctl construit (make -C cli/ctwctl build), jeton OpenObserve dans CTWCTL_O2_TOKEN_FILE.
set -euo pipefail

version=${1:?version}
case_name=${2:?case}
run=${3:?run number}

evals_dir=$(cd "$(dirname "$0")" && pwd)
repo_root=$(cd "$evals_dir/../../../.." && pwd)
out_root=${MONITOR_RUNS_DIR:?MONITOR_RUNS_DIR must point outside the repository}
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

cd "$repo_root"
claude -p "$prompt" \
  --plugin-dir agents/plugins/service \
  --model sonnet \
  --output-format stream-json --verbose \
  --allowedTools "Bash(ctwctl o2 *)" Read Skill \
  --strict-mcp-config \
  --max-budget-usd 5 \
  < /dev/null > "$out"
echo "$out"
