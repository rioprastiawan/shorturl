#!/usr/bin/env bash
# Continuously create short links until Ctrl+C or MAX_REQUESTS is reached.
set -uo pipefail

API_URL="${API_URL:-https://s.w4c.id/api/v1/links}"
DESTINATION_URL="${DESTINATION_URL:-https://waste4change.com}"
CONCURRENCY="${CONCURRENCY:-1}"
DELAY_SECONDS="${DELAY_SECONDS:-0.5}"
MAX_REQUESTS="${MAX_REQUESTS:-0}"

if [[ -z "${SHORTURL_API_KEY:-}" ]]; then
  echo "SHORTURL_API_KEY is required."
  echo "Usage: SHORTURL_API_KEY='shr_live_...' $0"
  exit 1
fi

if ! [[ "$CONCURRENCY" =~ ^[1-9][0-9]*$ ]]; then
  echo "CONCURRENCY must be a positive integer" >&2
  exit 1
fi
if ! [[ "$MAX_REQUESTS" =~ ^[0-9]+$ ]]; then
  echo "MAX_REQUESTS must be zero (unlimited) or a positive integer" >&2
  exit 1
fi

# Escape the two characters that can break a JSON string. URLs normally do not
# contain either, but handling them here keeps the payload valid.
json_destination="${DESTINATION_URL//\\/\\\\}"
json_destination="${json_destination//\"/\\\"}"

stopping=0
trap 'stopping=1; echo; echo "Stopping after the current batch..."' INT TERM

send_request() {
  local request_number="$1"
  local idempotency_key response status body
  idempotency_key="load-$(date +%s)-$$-${request_number}-${RANDOM}"

  if response="$(curl --silent --show-error \
    --request POST "$API_URL" \
    --header "Authorization: Bearer ${SHORTURL_API_KEY}" \
    --header "Content-Type: application/json" \
    --header "Idempotency-Key: ${idempotency_key}" \
    --data "{\"destination_url\":\"${json_destination}\"}" \
    --write-out $'\n%{http_code}')"; then
    status="${response##*$'\n'}"
    body="${response%$'\n'*}"
    if [[ "$status" == 2* ]]; then
      printf '[%06d] HTTP %s OK key=%s\n' "$request_number" "$status" "$idempotency_key"
    else
      printf '[%06d] HTTP %s ERROR %s\n' "$request_number" "$status" "$body" >&2
    fi
  else
    printf '[%06d] NETWORK ERROR key=%s\n' "$request_number" "$idempotency_key" >&2
  fi
}

echo "Target      : $API_URL"
echo "Destination : $DESTINATION_URL"
echo "Concurrency : $CONCURRENCY"
echo "Delay/batch : ${DELAY_SECONDS}s"
if (( MAX_REQUESTS == 0 )); then
  echo "Limit       : unlimited (press Ctrl+C to stop)"
else
  echo "Limit       : $MAX_REQUESTS requests"
fi
echo

request_number=0
while (( stopping == 0 )); do
  batch_size="$CONCURRENCY"
  if (( MAX_REQUESTS > 0 )); then
    remaining=$((MAX_REQUESTS - request_number))
    (( remaining <= 0 )) && break
    (( remaining < batch_size )) && batch_size="$remaining"
  fi

  pids=()
  for ((i = 0; i < batch_size; i++)); do
    request_number=$((request_number + 1))
    send_request "$request_number" &
    pids+=("$!")
  done

  for pid in "${pids[@]}"; do
    wait "$pid" || true
  done

  (( stopping == 0 )) && sleep "$DELAY_SECONDS"
done

echo "Finished. Requests attempted: $request_number"
