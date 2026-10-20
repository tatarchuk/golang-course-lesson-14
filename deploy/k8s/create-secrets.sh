#!/usr/bin/env bash
# Creates the Secrets of the albums namespace from values generated inside the VM; nothing secret is
# stored in Git (slide 43). The values are generated once into ~/.albums-k8s.env (mode 600) and
# reused on every run, because PostgreSQL keeps the password it was first initialised with.
# Run inside the VM after `kubectl apply -f /app/deploy/k8s/namespace.yaml`:
#   /app/deploy/k8s/create-secrets.sh
set -euo pipefail

env_file=$HOME/.albums-k8s.env
if [ ! -f "$env_file" ]; then
  umask 077
  {
    # Hex: DATABASE_URL is a URL and the app does not URL-encode the password.
    echo "DB_PASSWORD=$(openssl rand -hex 16)"
    echo "JWT_SECRET=$(openssl rand -base64 48 | tr -d '\n')"
    # The documented dev password (README), which scripts/smoke.sh logs in with.
    echo "ADMIN_PASSWORD=Albums-Dev-2026"
  } > "$env_file"
fi
. "$env_file"

# apply_secret NAME KEY=VALUE... creates or updates a generic Secret.
apply_secret() {
  local name=$1
  shift
  local args=()
  for kv in "$@"; do
    args+=(--from-literal="$kv")
  done
  kubectl -n albums create secret generic "$name" "${args[@]}" --dry-run=client -o yaml | kubectl apply -f -
}

apply_secret albums-db "POSTGRES_PASSWORD=$DB_PASSWORD"
apply_secret albums-app \
  "DATABASE_URL=postgres://albums:$DB_PASSWORD@postgres:5432/albums?sslmode=disable" \
  "JWT_SECRET=$JWT_SECRET"
apply_secret albums-admin "ADMIN_PASSWORD=$ADMIN_PASSWORD"
