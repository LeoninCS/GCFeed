#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
API_DIR="${ROOT_DIR}/apps/api"
WEB_DIR="${ROOT_DIR}/apps/web"

if ! command -v go >/dev/null 2>&1; then
  echo "go command is required"
  exit 1
fi

if ! command -v npm >/dev/null 2>&1; then
  echo "npm command is required"
  exit 1
fi

cleanup() {
  local code=$?
  trap - EXIT INT TERM

  if [[ -n "${API_PID:-}" ]] && kill -0 "${API_PID}" 2>/dev/null; then
    kill "${API_PID}" 2>/dev/null || true
  fi
  if [[ -n "${WEB_PID:-}" ]] && kill -0 "${WEB_PID}" 2>/dev/null; then
    kill "${WEB_PID}" 2>/dev/null || true
  fi

  wait "${API_PID:-}" "${WEB_PID:-}" 2>/dev/null || true
  exit "${code}"
}

trap cleanup EXIT INT TERM

echo "Starting feed API"
echo "Working directory: ${API_DIR}"
echo "Config: ${API_DIR}/configs/config.yaml"
(cd "${API_DIR}" && go run ./cmd/feed) &
API_PID=$!

echo "Starting web dev server"
echo "Working directory: ${WEB_DIR}"
echo "URL: http://127.0.0.1:5173/"
(cd "${WEB_DIR}" && npm run dev) &
WEB_PID=$!

echo "API PID: ${API_PID}"
echo "Web PID: ${WEB_PID}"
echo "Press Ctrl+C to stop both services"

while true; do
  if ! kill -0 "${API_PID}" 2>/dev/null; then
    wait "${API_PID}"
    exit $?
  fi
  if ! kill -0 "${WEB_PID}" 2>/dev/null; then
    wait "${WEB_PID}"
    exit $?
  fi
  sleep 1
done
