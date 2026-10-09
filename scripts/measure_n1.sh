#!/usr/bin/env bash
# Evidence for homework part 3: how many SQL SELECT statements one
# GET /api/v1/albums?limit=100 issues, the log of that request, and its latency.
# Run it before and after the Preload fix and paste the numbers into the README.
#
# Usage: scripts/measure_n1.sh [BASE_URL] [LOG]
#   BASE_URL  default http://localhost:8080
#   LOG       "compose" (default) reads `docker compose logs api`; otherwise a file,
#             for a host run started with: go run ./cmd/api > app.log 2>&1
#   AB_REQUESTS  requests sent by ab one at a time (default 200)
set -uo pipefail

BASE_URL=${1:-http://localhost:8080}
LOG=${2:-compose}
N=${AB_REQUESTS:-200}
URL="$BASE_URL/api/v1/albums?limit=100"

logs() {
  if [ "$LOG" = compose ]; then docker compose logs --no-log-prefix api 2>/dev/null; else cat "$LOG"; fi
}
count_selects() { logs | grep -c 'SELECT' || true; }

albums=$(curl -s "$URL" | python3 -c 'import json,sys; print(len(json.load(sys.stdin)))')
sleep 0.3; before=$(count_selects)
curl -s -o /dev/null "$URL"
sleep 0.3; after=$(count_selects)
queries=$((after - before))

echo "GET $URL"
echo "albums returned: $albums"
echo "SELECT statements for one request: $queries"
echo "--- SQL log of that request (GORM logger, info level) ---"
logs | grep 'SELECT' | tail -n "$queries" | sed -E 's/^[0-9]{4}\/[0-9]{2}\/[0-9]{2} [0-9:]+ //' | cut -c1-150
echo "--- latency: ab -n $N -c 1 ---"
ab -q -n "$N" -c 1 "$URL" | grep -E 'Requests per second|Time per request.*mean\)|  50%|  95%|  99%'
