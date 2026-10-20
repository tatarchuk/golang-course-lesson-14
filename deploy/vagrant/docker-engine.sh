#!/usr/bin/env bash
# Vagrant provisioner "docker-engine" (runs as root): Docker Engine and the Compose v2 plugin from
# Docker's own apt repository (deck slide 10; Ubuntu's docker.io package from slide 23 lags behind
# and ships Compose separately), plus jq for reading JSON. Idempotent (slide 7): a second run only
# reports what is installed.
set -euo pipefail

if docker compose version >/dev/null 2>&1 && command -v jq >/dev/null; then
  echo "docker already installed: $(docker --version)"
  exit 0
fi

export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq ca-certificates curl jq

install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc

. /etc/os-release
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu ${VERSION_CODENAME} stable" \
  > /etc/apt/sources.list.d/docker.list

apt-get update -qq
apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

# The vagrant user may talk to the Docker socket without sudo (from the next login on).
usermod -aG docker vagrant
echo "docker installed: $(docker --version); $(docker compose version)"
