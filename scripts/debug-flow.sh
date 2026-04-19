#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BASE_URL="${BASE_URL:-http://localhost:8080}"
cd "${ROOT_DIR}"

pause() {
  local message="$1"
  read -r -p "${message} Press Enter to continue... " _
}

show_api_state() {
  echo "== api: orderbook =="
  curl -fsS "${BASE_URL}/v1/orderbook/BTC-USDT" || true
  echo

  echo "== api: trades =="
  curl -fsS "${BASE_URL}/v1/trades/BTC-USDT" || true
  echo

  echo "== api: maker account =="
  curl -fsS "${BASE_URL}/v1/accounts/maker-1" || true
  echo

  echo "== api: taker account =="
  curl -fsS "${BASE_URL}/v1/accounts/taker-1" || true
  echo
}

show_pg_state() {
  echo "== postgres: balances =="
  sudo docker exec -i atlasx-postgres psql -U atlasx -d atlasx -c \
    "select * from account_balances order by account_id, asset;"

  echo "== postgres: orders =="
  sudo docker exec -i atlasx-postgres psql -U atlasx -d atlasx -c \
    "select id, account_id, symbol, side, quantity, remaining, reserved_amount, status from orders order by created_at;"

  echo "== postgres: trades =="
  sudo docker exec -i atlasx-postgres psql -U atlasx -d atlasx -c \
    "select id, symbol, price, quantity, maker_order_id, taker_order_id, executed_at from trades order by executed_at, id;"
}

show_redis_state() {
  echo "== redis: maker account =="
  sudo docker exec -i atlasx-redis redis-cli --raw GET account:maker-1 || true

  echo "== redis: taker account =="
  sudo docker exec -i atlasx-redis redis-cli --raw GET account:taker-1 || true

  echo "== redis: orderbook =="
  sudo docker exec -i atlasx-redis redis-cli --raw GET orderbook:BTC-USDT || true

  echo "== redis: trades =="
  sudo docker exec -i atlasx-redis redis-cli --raw GET trades:BTC-USDT || true
}

consume_topic() {
  local topic="$1"
  local limit="$2"
  timeout 5s sudo docker exec -i atlasx-redpanda rpk topic consume "${topic}" -n "${limit}" -f '%k\t%v\n' || true
}

show_topic_state() {
  echo "== redpanda: exchange.orders =="
  consume_topic exchange.orders 10

  echo "== redpanda: exchange.trades =="
  consume_topic exchange.trades 5

  echo "== redpanda: exchange.audit =="
  consume_topic exchange.audit 10
}

show_logs() {
  echo "== exchange logs =="
  sudo docker compose logs exchange --tail=50 || true
}

inspect_all() {
  show_logs
  show_api_state
  show_pg_state
  show_redis_state
  show_topic_state
}

post_json() {
  local path="$1"
  local payload="$2"
  curl -fsS -X POST "${BASE_URL}${path}" \
    -H "Content-Type: application/json" \
    -d "${payload}"
  echo
}

echo "== step 0: reset stack =="
./scripts/reset-stack.sh
inspect_all
pause "Stack is ready."

echo "== step 1: maker deposit BTC =="
post_json "/v1/accounts/deposit" '{"account_id":"maker-1","asset":"BTC","amount":"2"}'
inspect_all
pause "Maker deposit completed."

echo "== step 2: taker deposit USDT =="
post_json "/v1/accounts/deposit" '{"account_id":"taker-1","asset":"USDT","amount":"100000"}'
inspect_all
pause "Taker deposit completed."

echo "== step 3: maker places sell order =="
post_json "/v1/orders" '{"account_id":"maker-1","symbol":"BTC-USDT","side":"sell","price":"65000","quantity":"1"}'
inspect_all
pause "Maker order accepted."

echo "== step 4: taker places buy order =="
post_json "/v1/orders" '{"account_id":"taker-1","symbol":"BTC-USDT","side":"buy","price":"66000","quantity":"1"}'
inspect_all
pause "Taker order completed."

echo "== debug flow complete =="
