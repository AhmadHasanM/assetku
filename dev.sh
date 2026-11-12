#!/usr/bin/env bash
# Local development: embedded Postgres (:55432), API (:8080) and web (:3000)
# in one terminal. Ctrl+C stops all three.
#   ./dev.sh
set -euo pipefail
cd "$(dirname "$0")"

for port in 55432 8080 3000; do
  if (exec 3<>"/dev/tcp/127.0.0.1/$port") 2>/dev/null; then
    echo "Port $port is already in use. Stop whatever is running there first." >&2
    exit 1
  fi
done

[ -f api/.env ] || cp api/.env.example api/.env
[ -d web/node_modules ] || (cd web && npm install)

pids=()
# start NAME DIR CMD...: run CMD in DIR in its own process group, output prefixed with NAME.
start() {
  local name=$1 dir=$2
  shift 2
  (cd "$dir" && exec setsid "$@") > >(sed -u "s/^/[$name] /") 2>&1 &
  pids+=("$!")
}

stop() {
  trap - INT TERM EXIT
  echo "Stopping..."
  # web and API first, the database last so it shuts down cleanly.
  for ((i = ${#pids[@]} - 1; i >= 0; i--)); do
    kill -TERM -- "-${pids[i]}" 2>/dev/null || true
    wait "${pids[i]}" 2>/dev/null || true
  done
}
trap stop INT TERM EXIT

start db api go run ./cmd/devdb
echo "Waiting for the database..."
for _ in $(seq 120); do
  (exec 3<>/dev/tcp/127.0.0.1/55432) 2>/dev/null && break
  kill -0 "${pids[0]}" 2>/dev/null || { echo "Database failed to start." >&2; exit 1; }
  sleep 1
done

start api api go run ./cmd/server
start web web npm run dev

echo "Open http://localhost:3000 (Ctrl+C to stop)"
wait -n "${pids[@]}" || true
echo "A process exited." >&2
