#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BASE_URL="${BASE_URL:-http://localhost:8080}"
cd "${ROOT_DIR}"

echo "== restarting stack =="
sudo docker compose restart exchange postgres redis redpanda

echo "== waiting for health =="
for _ in $(seq 1 30); do
  if curl -fsS "${BASE_URL}/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

echo "== post-restart reads =="
curl -fsS "${BASE_URL}/v1/orderbook/BTC-USDT"
echo
curl -fsS "${BASE_URL}/v1/trades/BTC-USDT"
echo
curl -fsS "${BASE_URL}/v1/accounts/maker-1"
echo
curl -fsS "${BASE_URL}/v1/accounts/taker-1"
echo

echo "== checkpoints =="
echo "1. trades executed_at should be non-zero"
echo "2. maker locked balance should match open sell orders"
echo "3. orderbook and account locks should be consistent"
