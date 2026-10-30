#!/usr/bin/env bash
# Vagrant provisioner "go" (runs as root), Lesson 14: Go for `make test` and `make bootstrap`
# inside the VM, the same release and download as the course VM (serverless/Vagrantfile), and a
# C toolchain, because `go test -race` needs cgo. Idempotent: a second run only reports what is
# installed.
set -euo pipefail

GO_VERSION="1.27.3"
if [ "$(/usr/local/go/bin/go env GOVERSION 2>/dev/null)" = "go${GO_VERSION}" ] && command -v gcc >/dev/null; then
  echo "go already installed: $(/usr/local/go/bin/go version)"
  exit 0
fi

export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq build-essential make curl

GO_TARBALL="go${GO_VERSION}.linux-$(dpkg --print-architecture).tar.gz"
curl -fsSLO "https://go.dev/dl/${GO_TARBALL}"
rm -rf /usr/local/go
tar -C /usr/local -xzf "${GO_TARBALL}"
rm "${GO_TARBALL}"
# /usr/local/bin is on the PATH of every shell, including `vagrant ssh -c` (not a login shell).
ln -sf /usr/local/go/bin/go /usr/local/go/bin/gofmt /usr/local/bin/
echo "go installed: $(go version); $(gcc --version | head -1)"
