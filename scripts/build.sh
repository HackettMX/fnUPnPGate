#!/usr/bin/env bash
# 构建 fnPortGate 原生二进制并打包为 .fpk（无需 Docker）
# 用法：bash scripts/build.sh [版本号] [架构]
#   例：bash scripts/build.sh 2.0.0 amd64
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

MANIFEST_VERSION="$(sed -n 's/^version="\(.*\)"/\1/p' fpk/manifest | head -n1)"
VERSION="${1:-${MANIFEST_VERSION:-2.0.0}}"
ARCH="${2:-amd64}"

echo "==> 编译 fnPortGate ${VERSION}（linux/${ARCH}，静态链接）"
mkdir -p fpk/app/bin dist
CGO_ENABLED=0 GOOS=linux GOARCH="${ARCH}" \
  go build -C go -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
  -o "${ROOT}/fpk/app/bin/fnportgate" .

# Windows 上无法保留 Unix 执行位，这里显式设置；CI 在 Linux 上同样需要
chmod +x fpk/app/bin/fnportgate fpk/cmd/*

echo "==> 查找 fnpack"
FNPACK="$(command -v fnpack || true)"
if [ -z "${FNPACK}" ] && [ -x .tools/fnpack.exe ]; then FNPACK=".tools/fnpack.exe"; fi
if [ -z "${FNPACK}" ] && [ -x .tools/fnpack ]; then FNPACK=".tools/fnpack"; fi
if [ -z "${FNPACK}" ]; then
  echo "!! 未找到 fnpack，请从 https://developer.fnnas.com/docs/cli/fnpack/ 下载放到 .tools/ 或加入 PATH"
  exit 1
fi

echo "==> 打包 .fpk"
"${FNPACK}" build -d fpk

echo "==> 修正权限位与 manifest checksum"
PYTHON="$(command -v python3 || command -v python || true)"
[ -n "${PYTHON}" ] || { echo "!! 需要 python 来修正权限位"; exit 1; }
"${PYTHON}" scripts/fix_fpk_perms.py fnportgate.fpk

mv -f fnportgate.fpk "dist/fnportgate-${VERSION}.fpk"
echo ""
echo "==> 完成：dist/fnportgate-${VERSION}.fpk"
ls -lh "dist/fnportgate-${VERSION}.fpk"
