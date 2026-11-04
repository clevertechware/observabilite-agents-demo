#!/usr/bin/env bash
set -euo pipefail

infra_dir="$(cd "$(dirname "$0")" && pwd)"

values_args=()
for values in "${@:-traefik-values.yaml}"; do
  if [[ ! -f "$values" ]]; then
    echo "usage: $0 [chemin/vers/traefik-values.yaml ...]" >&2
    exit 1
  fi
  values_args+=(--values "$values")
done

tofu -chdir="$infra_dir" apply -auto-approve -var running=true
helm install traefik traefik/traefik \
  --namespace traefik --version 41.6.1 \
  "${values_args[@]}"
