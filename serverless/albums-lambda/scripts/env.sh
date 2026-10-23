#!/usr/bin/env bash
# Source it: `source scripts/env.sh`. Points the AWS CLI, the SDK and Terraform at the local Floci
# emulator and picks the Lambda architecture of this machine. Adapted from the course demo's env.sh.

export AWS_ENDPOINT_URL="${AWS_ENDPOINT_URL:-http://localhost:4566}"
export AWS_DEFAULT_REGION="${AWS_DEFAULT_REGION:-us-east-1}"
export AWS_REGION="$AWS_DEFAULT_REGION"
export AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-test}"
export AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-test}"
export AWS_PAGER=""                       # print CLI output instead of opening a pager

# Floci runs Lambda containers in the Docker host's architecture, so the binary, the function and
# Terraform all follow this machine (slide 32: runtime and architecture must match the build).
case "$(uname -m)" in
  arm64|aarch64) export GOARCH_LAMBDA=arm64 LAMBDA_ARCH=arm64 ;;
  *)             export GOARCH_LAMBDA=amd64 LAMBDA_ARCH=x86_64 ;;
esac
export TF_VAR_lambda_architecture="$LAMBDA_ARCH"

# Terraform keeps its AWS provider (hundreds of MB) on the VM's own disk, not in the shared folder.
export TF_DATA_DIR="$HOME/.terraform-data/albums-lambda"

export WEBHOOK_FN="albums-webhook"
export API_NAME="albums-webhook-api"     # Terraform names the REST API <function_name>-api
export STAGE="dev"

# An explicit --endpoint-url also works with AWS CLI versions older than 2.13.
awsf() { aws --endpoint-url "$AWS_ENDPOINT_URL" "$@"; }
