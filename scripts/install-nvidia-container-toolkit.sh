#!/usr/bin/env bash
# Installs the NVIDIA Container Toolkit so Docker can pass the GPU (NVENC/NVDEC/CUDA) into
# the Slouchcat image (deploy/compose.nvidia.yaml). Needed on the development machine to
# measure tonemap_cuda inside the container (plan M4, P-6).
# Source: https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html
# Run as: sudo bash scripts/install-nvidia-container-toolkit.sh
# Prerequisites: NVIDIA driver installed (nvidia-smi works) and Docker Engine running
# (scripts/install-system-tools.sh).
set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "run with sudo: sudo bash $0" >&2
  exit 1
fi

if ! command -v nvidia-smi >/dev/null 2>&1; then
  echo "nvidia-smi not found: install the NVIDIA driver first" >&2
  exit 1
fi
if ! command -v docker >/dev/null 2>&1; then
  echo "docker not found: run scripts/install-system-tools.sh first" >&2
  exit 1
fi

apt-get update
apt-get install -y ca-certificates curl gpg
install -m 0755 -d /etc/apt/keyrings
# The toolkit repository is distribution independent ("stable/deb").
curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey \
  | gpg --dearmor --yes -o /etc/apt/keyrings/nvidia-container-toolkit-keyring.gpg
chmod a+r /etc/apt/keyrings/nvidia-container-toolkit-keyring.gpg
curl -fsSL https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list \
  | sed 's#deb https://#deb [signed-by=/etc/apt/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' \
  > /etc/apt/sources.list.d/nvidia-container-toolkit.list
apt-get update
apt-get install -y nvidia-container-toolkit

# Registers the "nvidia" runtime in /etc/docker/daemon.json (keeps other settings) and
# enables CDI specs; then restarts Docker so the runtime is available.
nvidia-ctk runtime configure --runtime=docker
nvidia-ctk cdi generate --output=/etc/cdi/nvidia.yaml || echo "CDI spec generation failed (optional, continuing)" >&2
systemctl restart docker

echo
echo "verification:"
docker info --format '{{json .Runtimes}}' | tr ',' '\n' | grep -i nvidia || {
  echo "nvidia runtime not registered in Docker" >&2
  exit 1
}
docker run --rm --gpus all --entrypoint nvidia-smi nvidia/cuda:12.6.3-base-ubuntu24.04 \
  --query-gpu=name,driver_version --format=csv 2>/dev/null \
  || echo "note: GPU smoke test image could not be pulled or run; try: docker run --rm --gpus all ubuntu nvidia-smi"
echo "done: Docker can now pass the GPU; Slouchcat: task deploy:up with deploy/compose.nvidia.yaml"
