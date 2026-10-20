#!/usr/bin/env bash
# Vagrant provisioner "k3s" (runs as root): k3s, Kubernetes in one binary (deck slide 39), pinned to
# one release, plus a kubeconfig for the vagrant user (slide 40). The admin kubeconfig
# /etc/rancher/k3s/k3s.yaml stays readable by root only. Idempotent: an installed k3s of the pinned
# version is left alone. k3s runs its own containerd next to Docker; they do not share images.
set -euo pipefail

K3S_VERSION=v1.36.5+k3s1 # pinned (slide 38): the stable channel on 2026-10-20

if k3s --version 2>/dev/null | grep -qF "$K3S_VERSION"; then
  echo "k3s already installed: $K3S_VERSION"
else
  curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION="$K3S_VERSION" sh -
fi

# Slide 40: the vagrant user's own copy of the kubeconfig. kubectl here is k3s itself
# (/usr/local/bin/kubectl -> k3s), which reads the admin file unless KUBECONFIG is set, so every
# login gets KUBECONFIG (pam_env reads /etc/environment, also for `vagrant ssh -c`).
install -d -m 0700 -o vagrant -g vagrant /home/vagrant/.kube
install -m 0600 -o vagrant -g vagrant /etc/rancher/k3s/k3s.yaml /home/vagrant/.kube/config
line=KUBECONFIG=/home/vagrant/.kube/config
grep -qx "$line" /etc/environment || echo "$line" >> /etc/environment

# Wait (up to 2 minutes) until the node reports Ready.
for _ in $(seq 1 60); do
  k3s kubectl get nodes 2>/dev/null | grep -q ' Ready ' && break
  sleep 2
done
k3s kubectl get nodes
