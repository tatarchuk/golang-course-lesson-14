#!/usr/bin/env bash
# Terraform against Floci: init and validate, then plan (the default), apply or destroy.
# Usage: scripts/deploy-tf.sh [plan|apply|destroy]
# Plan is the default on purpose: read the diff first, apply afterwards (slides 32 and 43).
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/env.sh

TF=$(command -v terraform || command -v tofu) || { echo "Terraform >= 1.5 or OpenTofu is required"; exit 1; }
[[ -f build/webhook.zip ]] || { echo "Run make build first"; exit 1; }

cd terraform
mode=${1:-plan}
"$TF" init -input=false
"$TF" validate
case "$mode" in
  plan)    "$TF" plan -input=false
           echo; echo "This is a plan only. Read it, then run: scripts/deploy-tf.sh apply" ;;
  apply)   "$TF" apply -input=false -auto-approve ;;
  destroy) "$TF" destroy -input=false -auto-approve ;;
  *)       echo "Unknown mode: $mode (plan, apply or destroy)"; exit 1 ;;
esac
