#!/bin/sh
# Start air for `mise run dev` and open the app in Chrome once it is healthy.

port="${PORT:-8080}"
url="http://localhost:${port}"

air &
air_pid=$!

# shellcheck disable=SC2329
shutdown() {
  kill -TERM "$air_pid" 2>/dev/null || true
}
trap shutdown INT TERM

# Wait for the first successful build in the background, then open the app
# once. The main shell stays in `wait` so air keeps the terminal.
(
  if ! command -v open >/dev/null 2>&1; then
    echo "dev: no desktop opener; visit $url" >&2
    exit 0
  fi

  attempts=0
  while [ "$attempts" -lt 120 ]; do
    if curl -fsS --max-time 1 "$url/healthz" >/dev/null 2>&1; then
      if [ -d "/Applications/Google Chrome.app" ] && open -a "Google Chrome" "$url" 2>/dev/null; then
        exit 0
      fi
      if open "$url" 2>/dev/null; then
        exit 0
      fi
      echo "dev: could not open a browser; visit $url" >&2
      exit 0
    fi
    attempts=$((attempts + 1))
    sleep 0.5
  done
  echo "dev: server did not become healthy; visit $url" >&2
) &
opener_pid=$!

status=0
wait "$air_pid" || status=$?
if [ "$status" -ge 128 ]; then
  # A signal interrupted the wait; keep waiting for air to finish shutting down.
  wait "$air_pid" 2>/dev/null
  status=$?
  [ "$status" -ge 128 ] && status=0
fi

kill "$opener_pid" 2>/dev/null || true
wait "$opener_pid" 2>/dev/null || true

exit "$status"
