#!/usr/bin/env bash
#
# loadtest.sh generates load against a running echoserver instance by
# repeatedly calling all of its HTTP and gRPC endpoints. It is intended to
# produce telemetry (metrics, logs, traces and profiles) for testing the
# instrumentation of the echoserver.
#
# The echoserver must already be running. The script only depends on "curl"
# (HTTP) and "grpcurl" (gRPC). If "grpcurl" is not installed the gRPC endpoints
# are skipped.
#
# Configuration (via environment variables):
#   HTTP_ADDRESS   Address of the HTTP server        (default: localhost:8080)
#   GRPC_ADDRESS   Address of the gRPC server         (default: localhost:8081)
#   DURATION       Run time in seconds                (default: 60)
#   CONCURRENCY    Number of parallel workers         (default: 4)
#   SLEEP          Delay between requests per worker  (default: 0.1)
#   MAX_TIME       Per-request timeout in seconds     (default: 10)
#
# Example:
#   DURATION=120 CONCURRENCY=8 ./hack/loadtest.sh

set -uo pipefail

HTTP_ADDRESS="${HTTP_ADDRESS:-localhost:8080}"
GRPC_ADDRESS="${GRPC_ADDRESS:-localhost:8081}"
DURATION="${DURATION:-60}"
CONCURRENCY="${CONCURRENCY:-4}"
SLEEP="${SLEEP:-0.1}"
MAX_TIME="${MAX_TIME:-10}"

HTTP_URL="http://${HTTP_ADDRESS}"

command -v curl >/dev/null 2>&1 || {
	echo "error: curl is required but not installed" >&2
	exit 1
}

GRPC_ENABLED=1
if ! command -v grpcurl >/dev/null 2>&1; then
	echo "warning: grpcurl not found - skipping gRPC endpoints" >&2
	GRPC_ENABLED=0
fi

COUNT_DIR="$(mktemp -d)"
PIDS=()

cleanup() {
	for pid in "${PIDS[@]}"; do
		kill "$pid" >/dev/null 2>&1
	done
	rm -rf "$COUNT_DIR"
}
trap cleanup EXIT INT TERM

# count is a per-worker request counter. inc increments it after every request.
count=0
inc() {
	count=$((count + 1))
}

# http_requests fires one request against every HTTP endpoint.
http_requests() {
	curl -fsS -o /dev/null --max-time "$MAX_TIME" "$HTTP_URL/"; inc
	curl -fsS -o /dev/null --max-time "$MAX_TIME" -X POST -d 'load test body' "$HTTP_URL/"; inc
	curl -fsS -o /dev/null --max-time "$MAX_TIME" "$HTTP_URL/health"; inc
	curl -sS  -o /dev/null --max-time "$MAX_TIME" "$HTTP_URL/panic"; inc
	curl -sS  -o /dev/null --max-time "$MAX_TIME" "$HTTP_URL/status"; inc
	curl -sS  -o /dev/null --max-time "$MAX_TIME" "$HTTP_URL/status?status=200"; inc
	curl -sS  -o /dev/null --max-time "$MAX_TIME" "$HTTP_URL/status?status=404"; inc
	curl -fsS -o /dev/null --max-time "$MAX_TIME" "$HTTP_URL/timeout?timeout=200ms"; inc
	curl -fsS -o /dev/null --max-time "$MAX_TIME" "$HTTP_URL/timeout?timeout=500ms&flush=100ms"; inc
	curl -fsS -o /dev/null --max-time "$MAX_TIME" "$HTTP_URL/headersize?size=1024"; inc
	curl -fsS -o /dev/null --max-time "$MAX_TIME" -X POST \
		-d "{\"method\":\"GET\",\"url\":\"${HTTP_URL}/health\"}" "$HTTP_URL/request"; inc
	curl -fsS -o /dev/null --max-time "$MAX_TIME" "$HTTP_URL/fibonacci?n=100000"; inc
	curl -fsS -o /dev/null --max-time "$MAX_TIME" "$HTTP_URL/simulate?type=cpu&duration=200ms"; inc
	curl -fsS -o /dev/null --max-time "$MAX_TIME" "$HTTP_URL/simulate?type=memory&duration=200ms&size=1048576"; inc
	curl -fsS -o /dev/null --max-time "$MAX_TIME" "$HTTP_URL/simulate?type=goroutines&duration=200ms&count=100"; inc
	curl -fsS -o /dev/null --max-time "$MAX_TIME" "$HTTP_URL/simulate?type=mutex&duration=200ms&workers=16"; inc
	curl -fsS -o /dev/null --max-time "$MAX_TIME" "$HTTP_URL/simulate?type=block&duration=200ms&workers=16"; inc
	curl -sS  -o /dev/null --max-time "$MAX_TIME" "$HTTP_URL/metrics"; inc
}

# grpc_requests fires one request against every gRPC endpoint.
grpc_requests() {
	grpcurl -plaintext -d '{ "message": "Hello" }' "$GRPC_ADDRESS" Echoserver.Echo >/dev/null 2>&1; inc
	grpcurl -plaintext -d '{ "status": "random" }' "$GRPC_ADDRESS" Echoserver.Status >/dev/null 2>&1; inc
	grpcurl -plaintext -d '{ "status": "OK" }' "$GRPC_ADDRESS" Echoserver.Status >/dev/null 2>&1; inc
	grpcurl -plaintext -d "{ \"uri\": \"${GRPC_ADDRESS}\", \"method\": \"Echoserver.Echo\", \"message\": \"{ \\\"message\\\": \\\"Hello\\\" }\" }" \
		"$GRPC_ADDRESS" Echoserver.Request >/dev/null 2>&1; inc
	grpcurl -plaintext -d '{ "type": "cpu", "duration": "200ms" }' "$GRPC_ADDRESS" Echoserver.Simulate >/dev/null 2>&1; inc
	grpcurl -plaintext -d '{ "type": "memory", "duration": "200ms", "size": 1048576 }' "$GRPC_ADDRESS" Echoserver.Simulate >/dev/null 2>&1; inc
	grpcurl -plaintext -d '{ "type": "goroutines", "duration": "200ms", "count": 100 }' "$GRPC_ADDRESS" Echoserver.Simulate >/dev/null 2>&1; inc
	grpcurl -plaintext -d '{ "type": "mutex", "duration": "200ms", "workers": 16 }' "$GRPC_ADDRESS" Echoserver.Simulate >/dev/null 2>&1; inc
	grpcurl -plaintext -d '{ "type": "block", "duration": "200ms", "workers": 16 }' "$GRPC_ADDRESS" Echoserver.Simulate >/dev/null 2>&1; inc
}

# run_worker loops over all endpoints until the deadline is reached and writes
# the number of requests it made to a file.
run_worker() {
	local id="$1"
	local endtime="$2"

	count=0
	while [ "$(date +%s)" -lt "$endtime" ]; do
		http_requests
		[ "$GRPC_ENABLED" -eq 1 ] && grpc_requests
		sleep "$SLEEP"
	done

	echo "$count" >"${COUNT_DIR}/worker-${id}"
}

echo "Starting load test against ${HTTP_URL} (HTTP) and ${GRPC_ADDRESS} (gRPC)"
echo "  duration=${DURATION}s concurrency=${CONCURRENCY} sleep=${SLEEP}s"

start="$(date +%s)"
endtime=$((start + DURATION))

for i in $(seq 1 "$CONCURRENCY"); do
	run_worker "$i" "$endtime" &
	PIDS+=("$!")
done

wait

total=0
for file in "${COUNT_DIR}"/worker-*; do
	[ -f "$file" ] || continue
	total=$((total + $(cat "$file")))
done

elapsed=$(($(date +%s) - start))
[ "$elapsed" -eq 0 ] && elapsed=1

echo "Load test finished: ${total} requests in ${elapsed}s (~$((total / elapsed)) req/s)"
