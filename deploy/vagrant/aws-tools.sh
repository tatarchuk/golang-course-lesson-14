#!/usr/bin/env bash
# Vagrant provisioner "aws-tools" (runs as root): the AWS CLI v2 and awslocal for the deck's demos
# against Floci (slides 32-36). Idempotent: installed tools are skipped, environment lines are
# added once.
set -euo pipefail

AWS_CLI_VERSION=2.38.2     # pinned (slide 38)
AWSCLI_LOCAL_VERSION=0.22.2

missing=()
for cmd in zip unzip gpg pipx; do
  command -v "$cmd" >/dev/null || missing+=("$cmd")
done
if [ ${#missing[@]} -gt 0 ]; then
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -qq
  apt-get install -y -qq zip unzip gnupg pipx
fi

if ! command -v aws >/dev/null; then
  # The deck's official installer (slide 32), system-wide (/usr/local/bin) and pinned.
  curl -fsSL https://awscli.amazonaws.com/v2/install.sh | bash -s -- --system --version "$AWS_CLI_VERSION"
fi

if ! command -v awslocal >/dev/null; then
  # The slide's `pip install awscli-local` fails on Ubuntu 24.04 (PEP 668: an externally managed
  # Python); pipx installs it into its own virtual environment.
  PIPX_HOME=/opt/pipx PIPX_BIN_DIR=/usr/local/bin pipx install "awscli-local==$AWSCLI_LOCAL_VERSION"
fi

# Test credentials and the emulator's default region for every login, including
# `vagrant ssh -c` (pam_env reads /etc/environment). Floci accepts any credentials.
for line in AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test AWS_DEFAULT_REGION=us-east-1 DISABLE_DEPRECATION_NOTICE=1; do
  grep -qx "$line" /etc/environment || echo "$line" >> /etc/environment
done

echo "aws-tools ready: $(aws --version 2>&1); awslocal at $(command -v awslocal)"
