#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BASE_URL="${BASE_URL:-http://localhost:8080}"
cd "${ROOT_DIR}"

sudo docker compose down -v
sudo docker compose up --build -d

for _ in $(seq 1 60); do
  if curl -fsS "${BASE_URL}/healthz" >/dev/null 2>&1; then
    echo "exchange health check passed"
    echo "stack reset complete"
    echo "api: ${BASE_URL}"
    exit 0
  fi
  sleep 2
done

echo "exchange failed to become healthy in time" >&2
sudo docker compose logs exchange --tail=200 >&2
exit 1
