#!/usr/bin/env bash
set -euo pipefail

readonly BASE_URL=http://localhost:8080
readonly PPROF_ADDR=localhost:6060
readonly DATA=testdata/urls.txt
readonly STORE=/tmp/shortener-load.json
readonly DURATION=20
readonly WORKERS=50
readonly PROFILE_DIR=profiles

HEY="$(go env GOPATH)/bin/hey"
if [ ! -x "$HEY" ]; then
  echo "hey not found: $HEY (install with: go install github.com/rakyll/hey@latest)" >&2
  exit 1
fi

mkdir -p "$PROFILE_DIR"
rm -f "$STORE"

echo "==> building server and loadgen"
go build -o /tmp/shortener-server ./cmd/shortener
go build -o /tmp/shortener-loadgen ./cmd/loadgen

echo "==> starting server (file storage, pprof on $PPROF_ADDR)"
/tmp/shortener-server -a localhost:8080 -b "$BASE_URL" -f "$STORE" -pprof-address "$PPROF_ADDR" \
  > /tmp/shortener-load.log 2>&1 &
SERVER_PID=$!
trap 'kill "$SERVER_PID" 2>/dev/null || true' EXIT

ready=no
for _ in $(seq 1 50); do
  code=$(curl -s --max-time 2 -o /dev/null -w '%{http_code}' "$BASE_URL/ping" 2>/dev/null || true)
  if [ -n "$code" ] && [ "$code" != "000" ]; then
    ready=yes
    break
  fi
  sleep 0.2
done
if [ "$ready" != "yes" ]; then
  echo "server did not become ready; log:" >&2
  tail -20 /tmp/shortener-load.log >&2
  exit 1
fi

step() { echo "==> [$(date +%s)] $1"; }

step "capturing CPU profile (12s, starting in 3s), heap during load, then running loadgen"
(
  sleep 3
  curl -s --max-time 20 "http://$PPROF_ADDR/debug/pprof/profile?seconds=12" > "$PROFILE_DIR/load-cpu.pprof"
) &
CPU_PID=$!
(
  sleep 10
  curl -s --max-time 10 "http://$PPROF_ADDR/debug/pprof/heap" > "$PROFILE_DIR/load-during.pprof"
) &
HEAP_PID=$!

/tmp/shortener-loadgen -base "$BASE_URL" -data "$DATA" -duration "${DURATION}s" -workers "$WORKERS" \
  > /tmp/loadgen.out 2>&1

step "loadgen finished; capturing heap right after load"
curl -s --max-time 10 "http://$PPROF_ADDR/debug/pprof/heap" > "$PROFILE_DIR/load.pprof"
wait "$CPU_PID" || true
wait "$HEAP_PID" || true

SAMPLE=$(grep -m1 'sample short URL:' /tmp/loadgen.out | sed 's/.*sample short URL: //')
if [ -n "$SAMPLE" ]; then
  step "hey soak on a single hot short link: $SAMPLE"
  "$HEY" -z 10s -c "$WORKERS" -disable-redirects "$SAMPLE" > /tmp/hey-get.out 2>&1 || true
fi

echo ""
echo "==> store: $(wc -c < "$STORE") bytes, $(wc -l < "$STORE") records"
echo "==> profiles:"
ls -la "$PROFILE_DIR"/load*.pprof
echo ""
echo "==> loadgen (mixed: shorten + resolve) summary:"
cat /tmp/loadgen.out
if [ -s /tmp/hey-get.out ]; then
  echo ""
  echo "==> hey (hot-code GET soak) summary:"
  cat /tmp/hey-get.out
fi