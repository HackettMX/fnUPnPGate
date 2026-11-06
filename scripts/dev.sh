#!/usr/bin/env bash
# 本地开发：直接运行 Go 服务，前端已内嵌进二进制
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PORT="${FNPORTGATE_WEB_PORT:-8090}"

echo "==> 启动 fnPortGate：http://127.0.0.1:${PORT}"
echo "    数据目录：${FNPORTGATE_DATA_DIR:-$ROOT/go/data}"
echo "    提示：本机没有开启 UPnP 的路由器时，界面会显示「未连接」，属正常现象"

cd "${ROOT}/go"
FNPORTGATE_WEB_PORT="${PORT}" \
FNPORTGATE_DATA_DIR="${FNPORTGATE_DATA_DIR:-${ROOT}/go/data}" \
FNPORTGATE_LOG_LEVEL="${FNPORTGATE_LOG_LEVEL:-DEBUG}" \
  go run .
