#!/usr/bin/env bash
# Creates the Slouchcat test container ("weak NAS", docs/dev/TESTSERVER.md) on a Proxmox VE host.
# Run as root on the host:
#   bash create-testserver.sh --vmid 190 --storage local-lvm --bridge vmbr0 --ssh-key ~/.ssh/id_ed25519.pub
# Idempotent for the template download; refuses to overwrite an existing VMID.
set -euo pipefail

VMID=190
STORAGE=local-lvm
TEMPLATE_STORAGE=local
BRIDGE=vmbr0
HOSTNAME=slouchcat-nas
CORES=2
CPUUNITS=512
MEMORY_MB=4096
SWAP_MB=1024
ROOTFS_GB=16
DATA_GB=64
SSH_KEY=""
SETUP_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

usage() {
  sed -n '2,6p' "${BASH_SOURCE[0]}"
  exit 2
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --vmid) VMID="$2"; shift 2 ;;
    --storage) STORAGE="$2"; shift 2 ;;
    --template-storage) TEMPLATE_STORAGE="$2"; shift 2 ;;
    --bridge) BRIDGE="$2"; shift 2 ;;
    --hostname) HOSTNAME="$2"; shift 2 ;;
    --ssh-key) SSH_KEY="$2"; shift 2 ;;
    --cores) CORES="$2"; shift 2 ;;
    --memory) MEMORY_MB="$2"; shift 2 ;;
    --data-gb) DATA_GB="$2"; shift 2 ;;
    -h|--help) usage ;;
    *) echo "unknown argument: $1" >&2; usage ;;
  esac
done

for bin in pct pveam; do
  command -v "$bin" >/dev/null 2>&1 || { echo "error: $bin not found; run this on the Proxmox host" >&2; exit 1; }
done
[[ -n "$SSH_KEY" && -f "$SSH_KEY" ]] || { echo "error: --ssh-key must name an existing public key file" >&2; exit 1; }
if pct status "$VMID" >/dev/null 2>&1; then
  echo "error: VMID $VMID exists already; pick another --vmid or remove it first" >&2
  exit 1
fi

echo "== template (Debian 13 standard)"
pveam update >/dev/null
# Match the host architecture: a plain sort would pick arm64 over amd64 and the CT would never start.
arch="$(dpkg --print-architecture)"
template="$(pveam available --section system | awk -v a="_${arch}.tar" '/debian-13-standard/ && index($2, a) {print $2}' | sort -V | tail -1)"
[[ -n "$template" ]] || { echo "error: no debian-13-standard template for $arch offered by pveam" >&2; exit 1; }
if ! pveam list "$TEMPLATE_STORAGE" | grep -q "$template"; then
  pveam download "$TEMPLATE_STORAGE" "$template"
fi

echo "== create CT $VMID ($HOSTNAME): $CORES cores, ${MEMORY_MB} MB, rootfs ${ROOTFS_GB} GB, data ${DATA_GB} GB"
pct create "$VMID" "$TEMPLATE_STORAGE:vztmpl/$template" \
  --hostname "$HOSTNAME" \
  --unprivileged 1 \
  --features nesting=1,keyctl=1 \
  --cores "$CORES" --cpuunits "$CPUUNITS" \
  --memory "$MEMORY_MB" --swap "$SWAP_MB" \
  --rootfs "$STORAGE:$ROOTFS_GB" \
  --mp0 "$STORAGE:$DATA_GB,mp=/srv/media" \
  --net0 "name=eth0,bridge=$BRIDGE,ip=dhcp" \
  --ssh-public-keys "$SSH_KEY" \
  --onboot 1 \
  --description "Slouchcat test server (weak NAS), docs/dev/TESTSERVER.md"

pct start "$VMID"
echo "== waiting for the network"
for _ in $(seq 1 60); do
  ip="$(pct exec "$VMID" -- hostname -I 2>/dev/null | awk '{print $1}')"
  [[ -n "$ip" ]] && break
  sleep 1
done
[[ -n "${ip:-}" ]] || { echo "error: container got no IP address" >&2; exit 1; }

echo "== setup inside the container"
pct push "$VMID" "$SETUP_DIR/setup-nas.sh" /root/setup-nas.sh --perms 0755
pct push "$VMID" "$SETUP_DIR/../../testdata/generate-library-tree.py" /root/generate-library-tree.py --perms 0644
pct exec "$VMID" -- bash /root/setup-nas.sh

echo
echo "container $VMID ($HOSTNAME) is ready at $ip"
echo "next: pct enter $VMID; smbpasswd -a media   (sets the write password of the share, once)"
echo "SMB share: //$ip/media (guests read-only), placeholder tree under /srv/media/placeholder-tree"
