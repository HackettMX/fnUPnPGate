#!/usr/bin/env bash
# 本地开发启动脚本
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT}/src"

PY=".venv/bin/python"
[ -f "${PY}" ] || PY=".venv/Scripts/python.exe"

export FN_UPNP_DATA_DIR="${FN_UPNP_DATA_DIR:-${ROOT}/data}"
export FN_UPNP_WEB_PORT="${FN_UPNP_WEB_PORT:-8090}"
export FN_UPNP_LOG_LEVEL="${FN_UPNP_LOG_LEVEL:-INFO}"

echo "==> 启动 fnPortGate：http://127.0.0.1:${FN_UPNP_WEB_PORT}"
exec "${PY}" -m uvicorn app.main:app --host 0.0.0.0 --port "${FN_UPNP_WEB_PORT}" "$@"
