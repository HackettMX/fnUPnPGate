#!/usr/bin/env bash
# 构建 fnPortGate 镜像并打包为 .fpk
# 用法：bash scripts/build.sh [镜像地址]
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="1.0.0"
IMAGE="${1:-ghcr.io/hackettmx/fnupnpgate:${VERSION}}"

echo "==> 构建镜像 ${IMAGE}"
docker build -t "${IMAGE}" "${ROOT}/src"

echo "==> 更新 compose 中的镜像地址"
COMPOSE_FILE="${ROOT}/fpk/app/docker/docker-compose.yaml"
sed -i.bak "s|image:.*|image: ${IMAGE}|" "${COMPOSE_FILE}"
rm -f "${COMPOSE_FILE}.bak"

echo "==> 打包 .fpk（需已安装 fnpack）"
if command -v fnpack >/dev/null 2>&1; then
  (cd "${ROOT}/fpk" && fnpack build)
  echo "==> 完成，产物位于 fpk/ 目录"
else
  echo "!! 未检测到 fnpack，请先安装：https://developer.fnnas.com/docs/cli/fnpack/"
  echo "   然后在 ${ROOT}/fpk 目录执行：fnpack build"
fi

echo "==> 如需发布，请先推送镜像：docker push ${IMAGE}"
