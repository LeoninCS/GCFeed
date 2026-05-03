#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
API_DIR="${ROOT_DIR}/apps/api"

if ! command -v go >/dev/null 2>&1; then
  echo "go command is required"
  exit 1
fi

cd "${API_DIR}"

echo "Starting feed API"
echo "Working directory: ${API_DIR}"
echo "Config: ${API_DIR}/configs/config.yaml"

exec go run ./cmd/feed
