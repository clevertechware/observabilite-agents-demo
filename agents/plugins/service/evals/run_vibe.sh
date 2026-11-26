#!/usr/bin/env bash
# Usage: run_vibe.sh <version> <case> <run-number>
# Same cases as run.sh, run with the vibe CLI (portability arm of the eval).
# The agent runs through .agents/skills, discovered with --trust. Read-only: VIBE_EVAL_HOME
# holds a config.toml whose [tools.bash] allowlist is ctwctl/date; in headless mode every
# other bash command, and every pending approval, is denied instead of prompted.
# Prerequisites: ctwctl built (make -C cli/ctwctl build), SKILL_RUNS_DIR and
# CTWCTL_O2_TOKEN_FILE exported, VIBE_EVAL_HOME pointing at the isolated vibe home.
set -euo pipefail

version=${1:?version}
case_name=${2:?case}
run=${3:?run number}

evals_dir=$(cd "$(dirname "$0")" && pwd)
repo_root=$(cd "$evals_dir/../../../.." && pwd)
out_root=${SKILL_RUNS_DIR:?SKILL_RUNS_DIR must point outside the repository}
case $(cd "$out_root" 2>/dev/null && pwd -P || echo "$out_root")/ in
  "$repo_root"/*)
    echo "run_vibe.sh: SKILL_RUNS_DIR must be outside the repository" >&2
    exit 2
    ;;
esac
token_file=${CTWCTL_O2_TOKEN_FILE:?CTWCTL_O2_TOKEN_FILE must name a file holding the token}
vibe_home=${VIBE_EVAL_HOME:?VIBE_EVAL_HOME must point at the isolated vibe home (config.toml with [tools.bash] allowlist)}
prompt_file="$evals_dir/$case_name/prompt.md"

if [[ ! -f $prompt_file || ! -f $vibe_home/config.toml ]]; then
  echo "run_vibe.sh: unknown case '$case_name' or missing $vibe_home/config.toml" >&2
  exit 2
fi

out="$out_root/$version/$case_name-$run"
mkdir -p "$out"
prompt_tmp=$(mktemp)
trap 'rm -f "$prompt_tmp"' EXIT
awk 'BEGIN{n=0} /^---$/{n++; next} n>=2' "$prompt_file" > "$prompt_tmp"
max_turns=$(awk 'NR==1 && $0=="---"{infm=1; next} infm && $0=="---"{exit} infm && /^max_turns:[ ]*/{sub(/^max_turns:[ ]*/,""); print; exit}' "$prompt_file")
max_turns=${max_turns:-40}

export PATH="$repo_root/cli/ctwctl/bin:$PATH"
export CTWCTL_O2_URL=${CTWCTL_O2_URL:-https://o2.obs.scw.clevertechware.fr}
export CTWCTL_O2_USER=${CTWCTL_O2_USER:-ctwctl@sa.internal}
CTWCTL_O2_TOKEN=$(cat "$token_file")
export CTWCTL_O2_TOKEN

# Headless mode never prompts: a call needing approval is denied, so the isolated
# config is what keeps the run read-only (bash allowlist ctwctl/date, ask elsewhere).
cd "$repo_root"
VIBE_HOME="$vibe_home" vibe --trust \
  --prompt-file "$prompt_tmp" \
  --output-dir "$out" \
  --max-turns "$max_turns" \
  --time-limit 260 \
  --max-price 5 \
  > "$out/report.txt" 2> "$out/run.stderr"
echo "$out"
