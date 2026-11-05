#!/usr/bin/env bash
# One-command setup/update for an Ubuntu VPS.
#   First run : ./deploy/install.sh            (optional: EASYPOS_DOMAIN=pos.contoh.com ./deploy/install.sh)
#   Update    : git pull && ./deploy/install.sh
set -euo pipefail
cd "$(dirname "$0")/.."

if ! command -v docker >/dev/null 2>&1; then
  echo ">> Installing Docker..."
  curl -fsSL https://get.docker.com | sudo sh
  sudo usermod -aG docker "$USER" || true
fi
DC="docker compose"
if ! docker info >/dev/null 2>&1; then DC="sudo docker compose"; fi

rand() { head -c 24 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c "${1:-20}"; }

if [ ! -f .env ]; then
  echo ">> Creating .env with generated secrets..."
  OWNER_PASS="$(rand 16)"
  {
    echo "POSTGRES_PASSWORD=$(rand 24)"
    echo "EASYPOS_BOOTSTRAP_OWNER_USERNAME=owner"
    echo "EASYPOS_BOOTSTRAP_OWNER_PASSWORD=${OWNER_PASS}"
    if [ -n "${EASYPOS_DOMAIN:-}" ]; then
      echo "EASYPOS_DOMAIN=${EASYPOS_DOMAIN}"
      echo "EASYPOS_SECURE_COOKIES=true"
    fi
  } > .env
  chmod 600 .env
  FIRST_RUN=1
fi

if command -v ufw >/dev/null 2>&1 && sudo ufw status | grep -q "Status: active"; then
  sudo ufw allow 80/tcp >/dev/null; sudo ufw allow 443/tcp >/dev/null
fi

echo ">> Building and starting..."
$DC up -d --build

echo ">> Waiting for the app to become healthy..."
for i in $(seq 1 40); do
  if curl -fsS http://127.0.0.1/healthz >/dev/null 2>&1; then OK=1; break; fi
  sleep 3
done
[ "${OK:-0}" = 1 ] || { echo "App did not become healthy. Check: $DC logs app"; exit 1; }

HOST="$(grep -E '^EASYPOS_DOMAIN=' .env | cut -d= -f2 || true)"
if [ -n "$HOST" ]; then URL="https://$HOST"; else URL="http://$(curl -fsS https://api.ipify.org 2>/dev/null || hostname -I | awk '{print $1}')"; fi
echo
echo "easyPOS is running: $URL"
if [ "${FIRST_RUN:-0}" = 1 ]; then
  echo "Login Owner  -> username: owner   password: $(grep ^EASYPOS_BOOTSTRAP_OWNER_PASSWORD .env | cut -d= -f2)"
  echo "(Also stored in $(pwd)/.env. Change the password after first login.)"
fi
