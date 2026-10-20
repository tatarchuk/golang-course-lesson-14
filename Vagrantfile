# Lesson 10 extra part (DevOps): one Ubuntu 24.04 arm64 VM that runs Docker (the Compose stack and
# the Floci AWS emulator) and k3s. Deck slides 5-9; differences from the slide 6 Vagrantfile:
# - an arm64 box: VirtualBox on Apple Silicon runs ARM guests only, and ubuntu/jammy64 is amd64-only;
# - the box version is pinned (slide 6: "Pin the box version for repeatable results");
# - every forwarded port listens on the Mac's loopback interface only.

# macOS 27: Vagrant's host-port probe reports every port as taken; see the file for details.
require_relative "deploy/vagrant/port_check_fix"

Vagrant.configure("2") do |config|
  config.vm.box = "bento/ubuntu-24.04"
  config.vm.box_version = "202510.26.0"
  config.vm.box_architecture = "arm64"
  config.vm.hostname = "albums-devops"

  config.vm.provider "virtualbox" do |vb|
    vb.name = "albums-devops"
    vb.memory = 6144
    vb.cpus = 4
  end

  # Mac 127.0.0.1:<host> -> VM :<guest>
  config.vm.network "forwarded_port", guest: 8080, host: 8080, host_ip: "127.0.0.1"   # albums in Docker Compose
  config.vm.network "forwarded_port", guest: 30080, host: 30080, host_ip: "127.0.0.1" # albums in k3s (NodePort)
  config.vm.network "forwarded_port", guest: 80, host: 8081, host_ip: "127.0.0.1"     # k3s Traefik Ingress
  config.vm.network "forwarded_port", guest: 6443, host: 6443, host_ip: "127.0.0.1"   # Kubernetes API (kubectl/Lens on the Mac)

  # The repository is mounted at /app, as on slide 6; the default /vagrant share is not needed.
  config.vm.synced_folder ".", "/app"
  config.vm.synced_folder ".", "/vagrant", disabled: true

  # Provisioners: idempotent setup scripts that run on the first `vagrant up` and on
  # `vagrant provision [--provision-with NAME]` (slides 5 and 7).
  config.vm.provision "docker-engine", type: "shell", path: "deploy/vagrant/docker-engine.sh"
  # Floci, the AWS emulator (slides 29-30), and the Lambda runtime image it starts for Demo 2.
  # `up -d` leaves a running container untouched, so this is safe to re-run.
  config.vm.provision "floci", type: "shell", inline: <<~SH
    set -euo pipefail
    docker compose -f /app/deploy/floci/compose.floci.yaml up -d
    docker pull -q public.ecr.aws/lambda/provided:al2023
  SH
  config.vm.provision "aws-tools", type: "shell", path: "deploy/vagrant/aws-tools.sh"
  config.vm.provision "k3s", type: "shell", path: "deploy/vagrant/k3s.sh"
end
