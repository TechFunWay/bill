#!/usr/bin/env bash
set -euo pipefail

port="${1:-}"

case "${port}" in
  ""|*[!0-9]*)
    echo "Invalid port: ${port:-<empty>}" >&2
    exit 2
    ;;
esac

if (( port < 1 || port > 65535 )); then
  echo "Invalid port: ${port}" >&2
  exit 2
fi

find_listeners() {
  if command -v lsof >/dev/null 2>&1; then
    lsof -nP -tiTCP:"${port}" -sTCP:LISTEN 2>/dev/null || true
    return
  fi
  if command -v fuser >/dev/null 2>&1; then
    fuser "${port}/tcp" 2>/dev/null || true
    return
  fi
  echo "Cannot inspect port ${port}: install lsof or fuser." >&2
  exit 1
}

pids="$(find_listeners)"
if [[ -z "${pids//[[:space:]]/}" ]]; then
  echo "Port ${port} is available."
  exit 0
fi

echo "Port ${port} is occupied by PID(s): ${pids//$'\n'/ }. Stopping..."
for pid in ${pids}; do
  if [[ "${pid}" != "$$" ]]; then
    kill -TERM "${pid}" 2>/dev/null || true
  fi
done

for _ in {1..50}; do
  if [[ -z "$(find_listeners)" ]]; then
    echo "Port ${port} has been released."
    exit 0
  fi
  sleep 0.1
done

remaining="$(find_listeners)"
if [[ -n "${remaining//[[:space:]]/}" ]]; then
  echo "PID(s) did not stop in time; forcing shutdown: ${remaining//$'\n'/ }"
  for pid in ${remaining}; do
    if [[ "${pid}" != "$$" ]]; then
      kill -KILL "${pid}" 2>/dev/null || true
    fi
  done
fi

for _ in {1..20}; do
  if [[ -z "$(find_listeners)" ]]; then
    echo "Port ${port} has been released."
    exit 0
  fi
  sleep 0.1
done

echo "Failed to release port ${port}." >&2
exit 1
