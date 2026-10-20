#!/usr/bin/env bash
# Deck Demo 2 (slides 33-36): the Go Lambda of deploy/floci/lambda on the local AWS emulator Floci.
# Build on the Mac first (slide 35, arm64 because the VM and Floci's Lambda containers are arm64):
#   cd deploy/floci/lambda && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags lambda.norpc -o bootstrap . \
#     && zip function.zip bootstrap
# then run inside the VM: vagrant ssh -c /app/deploy/floci/demo2-lambda.sh
# The slides stop at get-function; the two invokes show the code running. Re-runnable: an earlier
# demo-api function is deleted first.
set -euo pipefail
cd "$(dirname "$0")/lambda"

run() {
  printf '\n$ %s\n' "$*"
  "$@"
}

if [ ! -f function.zip ]; then
  echo "function.zip is missing: build it on the Mac first (see the header of this script)" >&2
  exit 1
fi
awslocal lambda delete-function --function-name demo-api >/dev/null 2>&1 || true

# Slide 36, plus --architectures arm64 to match the binary.
run awslocal lambda create-function --function-name demo-api \
  --runtime provided.al2023 --handler bootstrap --architectures arm64 \
  --zip-file fileb://function.zip --role arn:aws:iam::000000000000:role/lambda \
  --query '[FunctionName,Runtime,Architectures[0],State]' --output text
run awslocal lambda wait function-active-v2 --function-name demo-api || echo "(wait not supported)"
run awslocal lambda get-function --function-name demo-api \
  --query 'Configuration.[FunctionName,Runtime,Handler,Architectures[0],State]' --output table

# Invoke it: with a name, then with an empty event (the handler falls back to "world").
run awslocal lambda invoke --function-name demo-api --cli-binary-format raw-in-base64-out \
  --payload '{"name":"albums"}' /tmp/demo-api-out.json
run cat /tmp/demo-api-out.json
echo
run awslocal lambda invoke --function-name demo-api --cli-binary-format raw-in-base64-out \
  --payload '{}' /tmp/demo-api-out.json
run cat /tmp/demo-api-out.json
echo

# Where it ran: Floci's CloudWatch Logs and the Lambda runtime container it started. Floci 2.3.0
# creates the log group and one stream per container but stores no events for this function
# (storedBytes 0), so `logs tail` (slide 37, Demo 3) prints nothing; the invokes above are the proof.
run awslocal logs describe-log-streams --log-group-name /aws/lambda/demo-api \
  --query 'logStreams[].[logStreamName,storedBytes]' --output text
run awslocal logs tail /aws/lambda/demo-api || echo "(logs not supported)"
run docker ps --format '{{.Names}}\t{{.Image}}\t{{.Status}}'
