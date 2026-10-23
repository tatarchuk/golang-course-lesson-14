#!/usr/bin/env bash
# Cold and warm calls of the deployed function on Floci (deck step 4, slide 44), adapted from the
# course demo's script. A cold start is forced by removing the function's warm containers first;
# each round also times the AWS CLI on its own, because every invoke number includes it.
# Usage: scripts/bench-cold-start.sh [N=10] [function=albums-webhook]
set -uo pipefail
cd "$(dirname "$0")/.."
source scripts/env.sh

N=${1:-10}
FN=${2:-$WEBHOOK_FN}
EVENT=events/valid.json
command -v docker >/dev/null || { echo "docker is required to remove the warm containers"; exit 1; }

now_ms() {
  local t; t=$(date +%s%3N 2>/dev/null)
  if [[ "$t" =~ ^[0-9]+$ ]]; then echo "$t"; else python3 -c 'import time; print(int(time.time() * 1000))'; fi
}
invoke_ms() {
  local s e; s=$(now_ms)
  awsf lambda invoke --function-name "$FN" --payload "fileb://$EVENT" \
    --cli-binary-format raw-in-base64-out /dev/null >/dev/null 2>&1
  e=$(now_ms); echo $((e - s))
}
cli_ms() { # the AWS CLI on its own: a call that does not run the function
  local s e; s=$(now_ms)
  awsf lambda get-function --function-name "$FN" >/dev/null 2>&1
  e=$(now_ms); echo $((e - s))
}
reset_warm() { docker ps -aq --filter "name=${FN}" | xargs -r docker rm -f >/dev/null 2>&1; }
stats() { awk '{s+=$1; if(min==""||$1<min)min=$1; if($1>max)max=$1} END{printf "avg=%d ms  min=%d ms  max=%d ms", s/NR, min, max}'; }

echo "Function: $FN, $N rounds (architecture ${LAMBDA_ARCH})"
invoke_ms >/dev/null # warm-up, so the runtime image is already pulled
cold=(); warm=(); cli=()
for i in $(seq 1 "$N"); do
  reset_warm
  c=$(invoke_ms); w=$(invoke_ms); b=$(cli_ms)
  cold+=("$c"); warm+=("$w"); cli+=("$b")
  printf "  #%-2d  cold=%5s ms   warm=%5s ms   CLI alone=%5s ms\n" "$i" "$c" "$w" "$b"
done
echo
echo "COLD: $(printf '%s\n' "${cold[@]}" | stats)"
echo "WARM: $(printf '%s\n' "${warm[@]}" | stats)"
echo "CLI:  $(printf '%s\n' "${cli[@]}" | stats)"
cat <<'NOTE'

How to read it (slide 44, "compare like with like"): every invoke number includes the AWS CLI
start-up (the CLI line), so the function's own share is the difference. A cold start on Floci is a
new Docker container in this VM, not a Firecracker micro-VM: the absolute numbers do not carry over
to AWS, the cold-minus-warm difference does.
NOTE
