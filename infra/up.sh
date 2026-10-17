#!/usr/bin/env bash
set -euo pipefail

infra_dir="$(cd "$(dirname "$0")" && pwd)"
traefik_values="${1:-traefik-values.yaml}"

if [[ ! -f "$traefik_values" ]]; then
  echo "usage: $0 [chemin/vers/traefik-values.yaml]" >&2
  exit 1
fi

tofu -chdir="$infra_dir" apply -auto-approve -var running=true
helm install traefik traefik/traefik \
  --namespace traefik --version 41.6.1 \
  --values "$traefik_values"
