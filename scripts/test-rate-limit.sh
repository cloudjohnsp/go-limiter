#!/usr/bin/env bash
# Tests nginx rate limiting in front of the Go API (compose stack).
#
# Usage:
#   ./scripts/test-rate-limit.sh
#   ./scripts/test-rate-limit.sh --base-url http://localhost --request-count 150
#   ./scripts/test-rate-limit.sh --direct-api   # bypass nginx (no rate limit expected)

set -euo pipefail

BASE_URL="http://localhost"
REQUEST_COUNT=100
DIRECT_API=false

usage() {
    cat <<EOF
Usage: $(basename "$0") [OPTIONS]

Options:
  --base-url URL         Target base URL (default: http://localhost)
  --request-count N      Parallel requests for burst test (default: 100)
  --direct-api           Hit API on :8080 directly (no nginx rate limit)
  -h, --help             Show this help
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --base-url)
            BASE_URL="${2:?missing value for --base-url}"
            shift 2
            ;;
        --request-count)
            REQUEST_COUNT="${2:?missing value for --request-count}"
            shift 2
            ;;
        --direct-api)
            DIRECT_API=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
            usage >&2
            exit 1
            ;;
    esac
done

if [[ "$DIRECT_API" == true ]]; then
    BASE_URL="http://localhost:8080"
fi

GREEN=$'\033[0;32m'
RED=$'\033[0;31m'
YELLOW=$'\033[1;33m'
CYAN=$'\033[0;36m'
DIM=$'\033[2m'
RESET=$'\033[0m'

step() {
    printf '\n%s==> %s%s\n' "$CYAN" "$1" "$RESET"
}

get_status_code() {
    curl -s -o /dev/null -w $'%{http_code}\n' --max-time 5 "$1" | tr -d '\r'
}

show_status_summary() {
    tr -d '\r' | sort -n | uniq -c | while read -r count code; do
        printf '  HTTP %s: %s\n' "$code" "$count"
    done
}

count_status() {
    local code="$1"
    tr -d '\r' | grep -cx "$code" || true
}

through=$([[ "$DIRECT_API" == true ]] && echo "API directly (no nginx limit)" || echo "nginx proxy (rate limited)")

printf 'Rate limiter test\n'
printf '  Target:     %s\n' "$BASE_URL"
printf '  Requests:   %s\n' "$REQUEST_COUNT"
printf '  Through:    %s\n' "$through"

# 1. Health check (always exempt from rate limiting on nginx)
step "Health check (GET /healthz)"
if ! health_body=$(curl -sf --max-time 5 "${BASE_URL}/healthz"); then
    printf '%s  FAILED: could not reach %s/healthz%s\n' "$RED" "$BASE_URL" "$RESET"
    printf '%s\nStart the stack with: docker compose up -d%s\n' "$YELLOW" "$RESET"
    exit 1
fi

if [[ "$health_body" != "OK" ]]; then
    printf '%s  FAILED: unexpected health response: %s%s\n' "$RED" "$health_body" "$RESET"
    exit 1
fi

printf '%s  OK (200)%s\n' "$GREEN" "$RESET"

# 2. Baseline single request to a rate-limited path
step "Baseline request (GET /)"
baseline_code=$(get_status_code "${BASE_URL}/" | tr -d '\n')
printf '  HTTP %s\n' "$baseline_code"

# 3. Burst traffic against rate-limited path
step "Burst test (GET /api x ${REQUEST_COUNT} in parallel)"
burst_file=$(mktemp)
trap 'rm -f "$burst_file"' EXIT

start_ms=$(date +%s%3N)
for _ in $(seq 1 "$REQUEST_COUNT"); do
    curl -s -o /dev/null -w $'%{http_code}\n' --max-time 10 "${BASE_URL}/api" >>"$burst_file" &
done
wait
end_ms=$(date +%s%3N)
elapsed_ms=$((end_ms - start_ms))

show_status_summary <"$burst_file"
printf '  Completed in %s ms\n' "$elapsed_ms"

rate_limited=$(count_status 429 <"$burst_file")
passed=false

if [[ "$DIRECT_API" == true ]]; then
    if [[ "$rate_limited" -eq 0 ]]; then
        passed=true
        printf '\n%sPASS: Direct API returned no 429 responses (nginx not in path).%s\n' "$GREEN" "$RESET"
    else
        printf '\n%sFAIL: Direct API returned %s x 429 (unexpected).%s\n' "$RED" "$rate_limited" "$RESET"
    fi
else
    if [[ "$rate_limited" -gt 0 ]]; then
        passed=true
        printf '\n%sPASS: nginx rate limiting triggered (%s x 429).%s\n' "$GREEN" "$rate_limited" "$RESET"
        printf '%s  Config: 30 req/s, burst 20 (see nginx/nginx.conf)%s\n' "$DIM" "$RESET"
    else
        printf '\n%sFAIL: No 429 responses. Rate limiting may not be active.%s\n' "$RED" "$RESET"
    fi
fi

# 4. Recovery after cooldown
step "Recovery test (wait 2s, then 5 sequential requests)"
sleep 2

recovery_file=$(mktemp)
trap 'rm -f "$burst_file" "$recovery_file"' EXIT

for _ in $(seq 1 5); do
    get_status_code "${BASE_URL}/" >>"$recovery_file"
done

show_status_summary <"$recovery_file"

recovery_429=$(count_status 429 <"$recovery_file")
if [[ "$recovery_429" -eq 0 ]]; then
    printf '%s  OK: traffic allowed again after cooldown.%s\n' "$GREEN" "$RESET"
else
    printf '%s  WARN: %s recovery requests still returned 429.%s\n' "$YELLOW" "$recovery_429" "$RESET"
    passed=false
fi

if [[ "$passed" != true ]]; then
    exit 1
fi
