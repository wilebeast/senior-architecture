#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"

request_with_retry() {
  local method="$1"
  local path="$2"
  local payload="${3:-}"

  for _ in $(seq 1 5); do
    if [[ -n "${payload}" ]]; then
      if curl -fsS -X "${method}" "${BASE_URL}${path}" \
        -H "Content-Type: application/json" \
        -d "${payload}"; then
        echo
        return 0
      fi
    else
      if curl -fsS -X "${method}" "${BASE_URL}${path}"; then
        echo
        return 0
      fi
    fi
    sleep 2
  done

  echo "request failed after retries: ${method} ${path}" >&2
  return 1
}

post_json() {
  local path="$1"
  local payload="$2"
  request_with_retry "POST" "${path}" "${payload}"
}

get_json() {
  local path="$1"
  request_with_retry "GET" "${path}"
}

echo "== deposits =="
post_json "/v1/accounts/deposit" '{"account_id":"maker-1","asset":"BTC","amount":"2"}'
post_json "/v1/accounts/deposit" '{"account_id":"taker-1","asset":"USDT","amount":"100000"}'

echo "== orders =="
post_json "/v1/orders" '{"account_id":"maker-1","symbol":"BTC-USDT","side":"sell","price":"65000","quantity":"1"}'
post_json "/v1/orders" '{"account_id":"taker-1","symbol":"BTC-USDT","side":"buy","price":"66000","quantity":"1"}'

echo "== reads =="
get_json "/v1/orderbook/BTC-USDT"
get_json "/v1/trades/BTC-USDT"
get_json "/v1/accounts/maker-1"
get_json "/v1/accounts/taker-1"
get_json "/v1/audit"
