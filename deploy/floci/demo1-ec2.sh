#!/usr/bin/env bash
# Deck Demo 1 (slide 32): EC2 on the local AWS emulator Floci, run inside the Vagrant VM:
#   vagrant ssh -c /app/deploy/floci/demo1-ec2.sh
# awslocal is the AWS CLI with --endpoint-url http://localhost:4566 (Floci). Floci runs EC2 in mock
# mode (deploy/floci/compose.floci.yaml): an instance is a record, not a VM, as the slide says.
# Re-runnable: the instances and the key pair of an earlier run are removed first.
set -euo pipefail

run() {
  printf '\n$ %s\n' "$*"
  "$@"
}

old=$(awslocal ec2 describe-instances \
  --filters Name=key-name,Values=demo Name=instance-state-name,Values=pending,running \
  --query 'Reservations[].Instances[].InstanceId' --output text)
if [ -n "$old" ]; then
  awslocal ec2 terminate-instances --instance-ids $old >/dev/null
fi
awslocal ec2 delete-key-pair --key-name demo >/dev/null 2>&1 || true

# 1. A key pair; only the first line of the private key is shown.
printf '$ %s\n' 'awslocal ec2 create-key-pair --key-name demo --query KeyMaterial --output text'
key=$(awslocal ec2 create-key-pair --key-name demo --query KeyMaterial --output text)
printf '%s\n' "${key%%$'\n'*}" "... (private key, $(printf '%s\n' "$key" | wc -l | tr -d ' ') lines)"

# 2. The images the emulator offers.
run awslocal ec2 describe-images --query 'Images[].[ImageId,Name,Architecture]' --output table

# 3. An instance from the deck's AMI.
run awslocal ec2 run-instances --image-id ami-0abcdef1234567892 --instance-type t3.micro \
  --key-name demo --query 'Instances[].[InstanceId,ImageId,InstanceType,State.Name]' --output table

# 4. The instances Floci knows.
run awslocal ec2 describe-instances \
  --query 'Reservations[].Instances[].[InstanceId,ImageId,InstanceType,KeyName,State.Name]' \
  --output table
