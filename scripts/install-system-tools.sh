#!/usr/bin/env bash
# Installs the system tools Slouchcat development needs that require root:
# Docker Engine with Compose and Buildx (Docker's official apt repository, Ubuntu noble base)
# and ffmpeg/ffprobe for the test media generator.
# Run as: sudo bash scripts/install-system-tools.sh
# Everything else (Go, Node, pnpm, task, sqlc, goose) is installed without root under ~/.local.
set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "run with sudo: sudo bash $0" >&2
  exit 1
fi

TARGET_USER="${SUDO_USER:-}"
CODENAME="${UBUNTU_CODENAME:-}"
if [[ -z "$CODENAME" ]]; then
  # Linux Mint reports its own codename; Docker publishes packages for the Ubuntu base.
  CODENAME="$(. /etc/os-release && echo "${UBUNTU_CODENAME:-$VERSION_CODENAME}")"
fi

apt-get update
apt-get install -y ca-certificates curl ffmpeg
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu ${CODENAME} stable" \
  > /etc/apt/sources.list.d/docker.list
apt-get update
apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
systemctl enable --now docker

if [[ -n "$TARGET_USER" ]]; then
  usermod -aG docker "$TARGET_USER"
  echo "added $TARGET_USER to group docker (takes effect after a new login; until then: sg docker -c 'docker ps')"
fi

docker --version
docker compose version
ffmpeg -version | head -1
