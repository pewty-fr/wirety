#!/usr/bin/env bash
set -euo pipefail

# Simple integration script to exercise the Wirety API.
# Creates a network and a jump peer plus three peers:
# 1. isolated + full encapsulation
# 2. isolated + partial (default) encapsulation
# 3. non-isolated + partial encapsulation
#
# Requirements: curl, jq

BASE_URL="${WIRETY_BASE_URL:-http://localhost:8080/api/v1}"

echo "==> Creating network"
NETWORK_RESP=$(curl -s -X POST "${BASE_URL}/networks" \
	-H 'Content-Type: application/json' \
	-d '{"name":"demo-net","cidr":"10.0.0.0/24","domain":"demo.local"}')
echo "$NETWORK_RESP" | jq '.'
NETWORK_ID=$(echo "$NETWORK_RESP" | jq -r '.id')
if [[ -z "$NETWORK_ID" || "$NETWORK_ID" == "null" ]]; then
	echo "Failed to create network" >&2
	exit 1
fi
echo "Network ID: $NETWORK_ID"

echo "==> Creating jump peer"
JUMP_RESP=$(curl -s -X POST "${BASE_URL}/networks/${NETWORK_ID}/peers" \
	-H 'Content-Type: application/json' \
	-d '{"name":"jump-1","is_jump":true,"jump_nat_interface":"eth0", "endpoint":"192.168.0.52", "listener_port":51820}')
echo "$JUMP_RESP" | jq '.'
JUMP_ID=$(echo "$JUMP_RESP" | jq -r '.id')

echo "==> Creating isolated full encapsulation peer"
PEER1_RESP=$(curl -s -X POST "${BASE_URL}/networks/${NETWORK_ID}/peers" \
	-H 'Content-Type: application/json' \
	-d '{"name":"iso-full","is_isolated":true,"full_encapsulation":true}')
echo "$PEER1_RESP" | jq '.'
PEER1_ID=$(echo "$PEER1_RESP" | jq -r '.id')

echo "==> Creating isolated normal encapsulation peer"
PEER2_RESP=$(curl -s -X POST "${BASE_URL}/networks/${NETWORK_ID}/peers" \
	-H 'Content-Type: application/json' \
	-d '{"name":"iso-partial","is_isolated":true,"full_encapsulation":false}')
echo "$PEER2_RESP" | jq '.'
PEER2_ID=$(echo "$PEER2_RESP" | jq -r '.id')

echo "==> Creating non-isolated normal encapsulation peer"
PEER3_RESP=$(curl -s -X POST "${BASE_URL}/networks/${NETWORK_ID}/peers" \
	-H 'Content-Type: application/json' \
	-d '{"name":"shared","is_isolated":false,"full_encapsulation":false}')
echo "$PEER3_RESP" | jq '.'
PEER3_ID=$(echo "$PEER3_RESP" | jq -r '.id')

echo "==> Fetching generated WireGuard configs"
for PID in "$JUMP_ID" "$PEER1_ID" "$PEER2_ID" "$PEER3_ID"; do
	echo "--- Config for peer $PID ---"
	curl -s "${BASE_URL}/networks/${NETWORK_ID}/peers/${PID}/config" || echo "(failed)"
	echo
done

echo "==> Summary"
jq -n --arg network "$NETWORK_ID" \
			 --arg jump "$JUMP_ID" \
			 --arg iso_full "$PEER1_ID" \
			 --arg iso_partial "$PEER2_ID" \
			 --arg shared "$PEER3_ID" \
	'{network_id:$network, jump_peer:$jump, isolated_full:$iso_full, isolated_partial:$iso_partial, shared_peer:$shared}'

echo "Done."
