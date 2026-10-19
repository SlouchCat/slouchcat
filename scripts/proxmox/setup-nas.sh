#!/usr/bin/env bash
# Configures the Slouchcat test container as a weak NAS (docs/dev/TESTSERVER.md): Samba share
# "media" on /srv/media, Docker with the Compose plugin for the Slouchcat stack, and the placeholder
# library tree for the SMB load measurement. Runs inside the container as root (pushed and started by
# create-testserver.sh, or by hand). Idempotent.
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y --no-install-recommends ca-certificates curl gnupg samba python3 git

echo "== Docker (official repository)"
if ! command -v docker >/dev/null 2>&1; then
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc
  chmod a+r /etc/apt/keyrings/docker.asc
  codename="$(. /etc/os-release && echo "$VERSION_CODENAME")"
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian $codename stable" \
    > /etc/apt/sources.list.d/docker.list
  apt-get update
  apt-get install -y --no-install-recommends docker-ce docker-ce-cli containerd.io docker-compose-plugin
fi
systemctl enable --now docker

echo "== media user and share"
id media >/dev/null 2>&1 || useradd --system --home /srv/media --shell /usr/sbin/nologin media
mkdir -p /srv/media
chown media:media /srv/media
chmod 0775 /srv/media

cat > /etc/samba/smb.conf <<'EOF'
[global]
   server string = Slouchcat test NAS
   workgroup = WORKGROUP
   server min protocol = SMB3_00
   map to guest = Bad User
   log level = 1
   # Deliberately plain: no oplocks tuning, no async; this box is meant to be slow.

[media]
   path = /srv/media
   browseable = yes
   guest ok = yes
   read only = yes
   write list = media
   force user = media
   create mask = 0664
   directory mask = 0775
EOF
systemctl enable --now smbd
systemctl restart smbd

echo "== placeholder tree (60'800 files) for the watcher measurement"
if [[ ! -d /srv/media/placeholder-tree ]]; then
  python3 -I /root/generate-library-tree.py --root /srv/media/placeholder-tree
  chown -R media:media /srv/media/placeholder-tree
fi

mkdir -p /opt/slouchcat
echo
echo "setup done. Set the write password once with: smbpasswd -a media"
echo "share: //$(hostname -I | awk '{print $1}')/media  (guest read-only)"
