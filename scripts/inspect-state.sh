#!/usr/bin/env bash
set -euo pipefail

consume_topic() {
  local topic="$1"
  local limit="$2"

  timeout 5s sudo docker exec -i atlasx-redpanda rpk topic consume "${topic}" -n "${limit}" -f '%k\t%v\n' || true
}

consume_topic_latest_by_key() {
  local topic="$1"
  local limit="$2"

  timeout 5s sudo docker exec -i atlasx-redpanda rpk topic consume "${topic}" -n "${limit}" -f '%k\t%v\n' | python3 -c '
import json, sys

latest = {}
for line in sys.stdin.read().splitlines():
    if not line.strip():
        continue
    key, value = line.split("\t", 1)
    latest[key] = json.loads(value)

for key in sorted(latest):
    item = latest[key]
    print("{}\taccount={}\tsymbol={}\tside={}\tstatus={}\tqty={}\tremaining={}\treserved={}".format(
        key,
        item.get("account_id"),
        item.get("symbol"),
        item.get("side"),
        item.get("status"),
        item.get("quantity"),
        item.get("remaining"),
        item.get("reserved_amount"),
    ))
' || true
}

echo "== service logs: exchange =="
sudo docker compose logs exchange --tail=100 || true

echo "== service logs: redpanda =="
sudo docker compose logs redpanda --tail=100 || true

echo "== service logs: postgres =="
sudo docker compose logs postgres --tail=50 || true

echo "== service logs: redis =="
sudo docker compose logs redis --tail=50 || true

echo "== postgres: balances =="
sudo docker exec -i atlasx-postgres psql -U atlasx -d atlasx -c \
  "select * from account_balances order by account_id, asset;"

echo "== postgres: orders =="
sudo docker exec -i atlasx-postgres psql -U atlasx -d atlasx -c \
  "select id, account_id, symbol, side, quantity, remaining, reserved_amount, status, created_at, updated_at from orders order by created_at;"

echo "== postgres: trades =="
sudo docker exec -i atlasx-postgres psql -U atlasx -d atlasx -c \
  "select id, symbol, price, quantity, maker_order_id, taker_order_id, executed_at from trades order by executed_at, id;"

echo "== postgres: audit =="
sudo docker exec -i atlasx-postgres psql -U atlasx -d atlasx -c \
  "select id, event_type, created_at from audit_events order by created_at;"

echo "== redis: keys =="
sudo docker exec -i atlasx-redis redis-cli KEYS '*'

echo "== redis: maker account =="
sudo docker exec -i atlasx-redis redis-cli GET account:maker-1

echo "== redis: taker account =="
sudo docker exec -i atlasx-redis redis-cli GET account:taker-1

echo "== redis: orderbook =="
sudo docker exec -i atlasx-redis redis-cli GET orderbook:BTC-USDT

echo "== redis: trades =="
sudo docker exec -i atlasx-redis redis-cli GET trades:BTC-USDT

echo "== redpanda: topics =="
sudo docker exec -i atlasx-redpanda rpk topic list

echo "== redpanda: exchange.orders latest-by-key =="
consume_topic_latest_by_key exchange.orders 10

echo "== redpanda: exchange.orders raw-events =="
consume_topic exchange.orders 10

echo "== redpanda: exchange.trades =="
consume_topic exchange.trades 5

echo "== redpanda: exchange.audit =="
consume_topic exchange.audit 10
