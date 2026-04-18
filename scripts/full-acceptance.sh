#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BASE_URL="${BASE_URL:-http://localhost:8080}"
cd "${ROOT_DIR}"

consume_topic_json_count() {
  local topic="$1"
  local limit="$2"

  timeout 5s sudo docker exec -i atlasx-redpanda rpk topic consume "${topic}" -n "${limit}" -f '%v\n' | python3 -c '
import json, sys
count = 0
for line in sys.stdin.read().splitlines():
    if line.strip():
        json.loads(line)
        count += 1
print(count)
' || true
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
print(json.dumps(latest))
' || true
}

echo "== step 1/4: reset stack =="
./scripts/reset-stack.sh

echo "== step 2/4: smoke test =="
./scripts/smoke-test.sh

echo "== step 3/4: restart check =="
./scripts/restart-check.sh

echo "== step 4/4: inspect state =="
./scripts/inspect-state.sh

echo "== step 5/5: assertions =="

orderbook_json="$(curl -fsS "${BASE_URL}/v1/orderbook/BTC-USDT")"
trades_json="$(curl -fsS "${BASE_URL}/v1/trades/BTC-USDT")"
maker_json="$(curl -fsS "${BASE_URL}/v1/accounts/maker-1")"
taker_json="$(curl -fsS "${BASE_URL}/v1/accounts/taker-1")"
audit_json="$(curl -fsS "${BASE_URL}/v1/audit")"

pg_balances_json="$(sudo docker exec -i atlasx-postgres psql -U atlasx -d atlasx -At -F $'\t' -c "select account_id, asset, available, locked from account_balances order by account_id, asset;" | python3 -c '
import json, sys
out = {}
for line in sys.stdin.read().splitlines():
    if not line.strip():
        continue
    account_id, asset, available, locked = line.split("\t")
    out.setdefault(account_id, {})[asset] = {
        "available": float(available),
        "locked": float(locked),
    }
print(json.dumps(out))
')"

pg_orders_json="$(sudo docker exec -i atlasx-postgres psql -U atlasx -d atlasx -At -F $'\t' -c "select id, account_id, symbol, side, quantity, remaining, reserved_amount, status from orders order by id;" | python3 -c '
import json, sys
out = {}
for line in sys.stdin.read().splitlines():
    if not line.strip():
        continue
    order_id, account_id, symbol, side, quantity, remaining, reserved_amount, status = line.split("\t")
    out[order_id] = {
        "id": order_id,
        "account_id": account_id,
        "symbol": symbol,
        "side": side,
        "quantity": float(quantity),
        "remaining": float(remaining),
        "reserved_amount": float(reserved_amount),
        "status": status,
    }
print(json.dumps(out))
')"
pg_trades_count="$(sudo docker exec -i atlasx-postgres psql -U atlasx -d atlasx -At -c "select count(*) from trades;")"
pg_audit_count="$(sudo docker exec -i atlasx-postgres psql -U atlasx -d atlasx -At -c "select count(*) from audit_events;")"

redis_maker_json="$(sudo docker exec -i atlasx-redis redis-cli --raw GET account:maker-1)"
redis_taker_json="$(sudo docker exec -i atlasx-redis redis-cli --raw GET account:taker-1)"
redis_trades_json="$(sudo docker exec -i atlasx-redis redis-cli --raw GET trades:BTC-USDT)"

orders_topic_latest_json="$(consume_topic_latest_by_key exchange.orders 10)"
trades_topic_count="$(consume_topic_json_count exchange.trades 5)"
audit_topic_count="$(consume_topic_json_count exchange.audit 10)"

orders_topic_latest_json="${orders_topic_latest_json:-\{\}}"
trades_topic_count="${trades_topic_count:-0}"
audit_topic_count="${audit_topic_count:-0}"

python3 - <<'PY' \
  "$orderbook_json" "$trades_json" "$maker_json" "$taker_json" "$audit_json" \
  "$pg_balances_json" "$pg_orders_json" "$pg_trades_count" "$pg_audit_count" \
  "$redis_maker_json" "$redis_taker_json" "$redis_trades_json" \
  "$orders_topic_latest_json" "$trades_topic_count" "$audit_topic_count"
import json
import math
import sys

(
    orderbook_raw,
    trades_raw,
    maker_raw,
    taker_raw,
    audit_raw,
    pg_balances_raw,
    pg_orders_raw,
    pg_trades_count_raw,
    pg_audit_count_raw,
    redis_maker_raw,
    redis_taker_raw,
    redis_trades_raw,
    orders_topic_latest_raw,
    trades_topic_count_raw,
    audit_topic_count_raw,
) = sys.argv[1:]

orderbook = json.loads(orderbook_raw)
trades = json.loads(trades_raw)
maker = json.loads(maker_raw)
taker = json.loads(taker_raw)
audit = json.loads(audit_raw)
pg_balances = json.loads(pg_balances_raw)
pg_orders = json.loads(pg_orders_raw)
redis_maker = json.loads(redis_maker_raw)
redis_taker = json.loads(redis_taker_raw)
redis_trades = json.loads(redis_trades_raw)
orders_topic_latest = json.loads(orders_topic_latest_raw)

pg_trades_count = int(pg_trades_count_raw.strip())
pg_audit_count = int(pg_audit_count_raw.strip())
trades_topic_count = int(trades_topic_count_raw.strip())
audit_topic_count = int(audit_topic_count_raw.strip())

def fail(message: str) -> None:
    print(f"ASSERTION FAILED: {message}", file=sys.stderr)
    sys.exit(1)

def close(a: float, b: float) -> bool:
    return math.isclose(a, b, rel_tol=0, abs_tol=1e-9)

if not trades:
    fail("expected at least one trade in API response")

for trade in trades:
    if trade["executed_at"] == "0001-01-01T00:00:00Z":
        fail("trade executed_at is zero")

expected_locked = {
    "maker-1": {"BTC": 0.0, "USDT": 0.0},
    "taker-1": {"BTC": 0.0, "USDT": 0.0},
}

for ask in orderbook.get("asks", []):
    expected_locked.setdefault(ask["account_id"], {"BTC": 0.0, "USDT": 0.0})
    expected_locked[ask["account_id"]]["BTC"] += float(ask["remaining"])

for bid in orderbook.get("bids", []):
    expected_locked.setdefault(bid["account_id"], {"BTC": 0.0, "USDT": 0.0})
    expected_locked[bid["account_id"]]["USDT"] += float(bid["price"]) * float(bid["remaining"])

for account in (maker, taker):
    account_id = account["id"]
    balances = account["balances"]
    for asset in ("BTC", "USDT"):
        actual_locked = float(balances.get(asset, {}).get("locked", 0.0))
        expected = float(expected_locked.get(account_id, {}).get(asset, 0.0))
        if not close(actual_locked, expected):
            fail(f"locked balance mismatch for {account_id} {asset}: expected {expected}, got {actual_locked}")

for account in (maker, taker):
    account_id = account["id"]
    api_balances = account["balances"]
    pg_account = pg_balances.get(account_id, {})
    redis_account = json.loads(json.dumps(redis_maker if account_id == "maker-1" else redis_taker))

    for asset, api_balance in api_balances.items():
        pg_balance = pg_account.get(asset)
        if pg_balance is None:
            fail(f"missing postgres balance for {account_id} {asset}")
        if not close(float(api_balance["available"]), float(pg_balance["available"])):
            fail(f"postgres available mismatch for {account_id} {asset}")
        if not close(float(api_balance["locked"]), float(pg_balance["locked"])):
            fail(f"postgres locked mismatch for {account_id} {asset}")

        redis_balance = redis_account["balances"].get(asset)
        if redis_balance is None:
            fail(f"missing redis balance for {account_id} {asset}")
        if not close(float(api_balance["available"]), float(redis_balance["available"])):
            fail(f"redis available mismatch for {account_id} {asset}")
        if not close(float(api_balance["locked"]), float(redis_balance["locked"])):
            fail(f"redis locked mismatch for {account_id} {asset}")

if len(redis_trades) != len(trades):
    fail(f"redis trades count mismatch: api={len(trades)} redis={len(redis_trades)}")

if pg_trades_count != len(trades) or trades_topic_count != len(trades):
    fail(f"trade count mismatch across systems: api={len(trades)} pg={pg_trades_count} topic={trades_topic_count}")

if len(pg_orders) != len(orders_topic_latest):
    fail(f"distinct order count mismatch across systems: pg={len(pg_orders)} topic_latest={len(orders_topic_latest)}")

for order_id, pg_order in pg_orders.items():
    topic_order = orders_topic_latest.get(order_id)
    if topic_order is None:
        fail(f"missing order in topic stream: {order_id}")
    for field in ("account_id", "symbol", "side", "status"):
        if str(pg_order[field]) != str(topic_order[field]):
            fail(f"order field mismatch for {order_id} field={field}: pg={pg_order[field]} topic={topic_order[field]}")
    for field in ("quantity", "remaining", "reserved_amount"):
        if not close(float(pg_order[field]), float(topic_order[field])):
            fail(f"order field mismatch for {order_id} field={field}: pg={pg_order[field]} topic={topic_order[field]}")

if pg_audit_count != len(audit) or audit_topic_count != len(audit):
    fail(f"audit count mismatch across systems: api={len(audit)} pg={pg_audit_count} topic={audit_topic_count}")

print("ASSERTIONS PASSED")
print(f"trades={len(trades)} orders={len(pg_orders)} audit={len(audit)}")
PY

echo "== acceptance complete =="
