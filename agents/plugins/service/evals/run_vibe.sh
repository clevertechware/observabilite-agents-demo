#!/usr/bin/env bash
# Usage: run_vibe.sh <version> <case> <run-number>
# Same cases as run.sh, run with the vibe CLI (portability arm of the eval).
# The agent works in an empty temporary directory holding only the case files and a copy of
# .agents/skills, discovered with --trust: with read_file limited to the working directory, the code
# and the fault injection of the demo stay out of reach. Read-only: the versioned
# vibe-eval-config.toml is copied to $VIBE_EVAL_HOME/config.toml before each run. Its bash allowlist
# is the prefixes "ctwctl o2", "ctwctl -o json o2" and "date"; read_file is denied on graders,
# services, deploy and infra. In headless mode every call outside the allowlists is denied instead
# of prompted. Vibe's documentation does not say how the bash prefixes apply to compound commands
# (;, &&, |), and the denylist globs and the migration key have not been exercised by a run.
# Prerequisites: ctwctl built (make -C cli/ctwctl build), SKILL_RUNS_DIR and
# CTWCTL_O2_TOKEN_FILE exported, VIBE_EVAL_HOME pointing at the isolated vibe home (credentials,
# trusted folders).
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
vibe_home=${VIBE_EVAL_HOME:?VIBE_EVAL_HOME must point at the isolated vibe home}
prompt_file="$evals_dir/$case_name/prompt.md"

if [[ ! -f $prompt_file || ! -d $vibe_home ]]; then
  echo "run_vibe.sh: unknown case '$case_name' or missing vibe home $vibe_home" >&2
  exit 2
fi
cp "$evals_dir/vibe-eval-config.toml" "$vibe_home/config.toml"

out="$out_root/$version/$case_name-$run"
mkdir -p "$out"
prompt_tmp=$(mktemp)
workdir=$(mktemp -d)
trap 'rm -f "$prompt_tmp"; rm -rf "$workdir"' EXIT
awk 'BEGIN{n=0} /^---$/{n++; next} n>=2' "$prompt_file" > "$prompt_tmp"
max_turns=$(awk 'NR==1 && $0=="---"{infm=1; next} infm && $0=="---"{exit} infm && /^max_turns:[ ]*/{sub(/^max_turns:[ ]*/,""); print; exit}' "$prompt_file")
max_turns=${max_turns:-40}

export PATH="$repo_root/cli/ctwctl/bin:$PATH"
export CTWCTL_O2_URL=${CTWCTL_O2_URL:-https://o2.obs.scw.clevertechware.fr}
export CTWCTL_O2_USER=${CTWCTL_O2_USER:-ctwctl@sa.internal}
CTWCTL_O2_TOKEN=$(cat "$token_file")
export CTWCTL_O2_TOKEN

# Headless mode never prompts: a call needing approval is denied, so the copied config and the
# approval policy keep the run read-only. Vibe's remote defaults change between runs (edits approved,
# subagents, background processes started from run_typescript outside the bash allowlist): the config
# turns them off, --enabled-tools limits the tools and the model is pinned. run_typescript stays
# available, but its bash calls go through the same allowlist.
mkdir -p "$workdir/.agents"
cp -RL "$repo_root/.agents/skills" "$workdir/.agents/"
case_copy="$workdir/agents/plugins/service/evals/$case_name"
mkdir -p "$case_copy"
find "$evals_dir/$case_name" -maxdepth 1 -type f ! -name prompt.md -exec cp {} "$case_copy/" \;
cd "$workdir"
VIBE_HOME="$vibe_home" vibe --trust \
  --model mistral-large-4-0 \
  --approval-policy ask \
  --enabled-tools bash --enabled-tools read_file --enabled-tools skill \
  --prompt-file "$prompt_tmp" \
  --output-dir "$out" \
  --max-turns "$max_turns" \
  --time-limit 260 \
  --max-price 5 \
  > "$out/report.txt" 2> "$out/run.stderr"
echo "$out"
