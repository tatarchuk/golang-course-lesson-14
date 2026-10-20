#!/usr/bin/env bash
# One screen for the homework's environment part: the VM, Docker with its containers (the albums
# Compose stack and Floci), Floci's health and the k3s cluster. Run inside the VM:
#   vagrant ssh -c /app/deploy/vagrant/env-status.sh
set -uo pipefail

. /etc/os-release
echo "== VM $(hostname): $PRETTY_NAME, $(uname -m), $(nproc) CPUs, $(free -m | awk '/^Mem:/ {print $2}') MB"
echo
echo "== Docker $(docker version --format '{{.Server.Version}}'): containers"
docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}'
echo
echo "== Floci (the AWS emulator) on localhost:4566"
curl -s localhost:4566/_localstack/health | jq -c '{version, ec2: .services.ec2, lambda: .services.lambda}'
echo
echo "== k3s: nodes"
kubectl get nodes -o wide
echo
echo "== k3s: pods in all namespaces"
kubectl get pods -A
