#!/bin/bash
set -e

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

APP_NAME=$(python3 -c "import json; print(json.load(open('app.json'))['appname'])")
APP_BINARY="bill"
FNOS_PKG_NAME=$(awk -F'=' '/^appname/ {gsub(/^[ \t]+|[ \t]+$/, "", $2); print $2}' fnpack/manifest)
VERSION=$(cat VERSION | tr -d '\n')
BUILD_TIME=$(date +%Y-%m-%dT%H:%M:%S)
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS="-X smallgo/server/version.Version=${VERSION} -X smallgo/server/version.BuildTime=${BUILD_TIME} -X smallgo/server/version.GitCommit=${GIT_COMMIT} -X smallgo/server/version.AppName=${APP_BINARY}"
LOCAL_GOCACHE="${GOCACHE:-/tmp/bill-go-build}"

# 可见的桌面主入口必须走统一网关，且必须是 type=url 新页面打开
# （2026-10-16 改造，与家族标准一致）。应用页面跑在网关域上，飞牛会话同源，
# 飞牛授权登录零跳转；网关域上应用自身的 Authorization 凭证会被接入层拦截，
# 登录态由服务端种下的 HttpOnly 会话 Cookie 维持（网关域不发送该头）。
# 网关另保留一个隐藏入口，供直连端口（旧书签）的访问者走一次性票据跳板。
python3 - <<'PY'
import json

with open("fnpack/app/ui/config", encoding="utf-8") as source:
    config = json.load(source)
entries = config[".url"]
main = entries.get("techfunway-bill.main")
login = entries.get("techfunway-bill.FnOSLogin")
if not main or main.get("type") != "url":
    raise SystemExit("fnOS main entry must use type=url (opens in a new page, not an in-desktop window)")
if main.get("protocol") != "" or main.get("gatewayPrefix") != "/app/techfunway-bill" or main.get("gatewaySocket") != "app.sock":
    raise SystemExit("fnOS main entry must be a unified-gateway entry (protocol='' + gatewayPrefix + gatewaySocket)")
if main.get("url") != "/app/techfunway-bill":
    raise SystemExit("fnOS main entry must land on the gateway-prefixed SPA root")
if not login or login.get("gatewayPrefix") != "/app/techfunway-bill" or login.get("gatewaySocket") != "app.sock":
    raise SystemExit("fnOS hidden entry must use the unified gateway")
if not login.get("noDisplay") or login.get("url") != "/app/techfunway-bill/fnos-entry.html":
    raise SystemExit("fnOS hidden entry must point to the login trampoline and stay hidden")
PY

echo "Building frontend..."
VITE_FNOS_APP=true VITE_BASE_PATH="/app/${FNOS_PKG_NAME}/" npm --prefix web ci
VITE_FNOS_APP=true VITE_BASE_PATH="/app/${FNOS_PKG_NAME}/" npm --prefix web run build

echo "Copying frontend..."
rm -rf server/static/dist
cp -r web/dist server/static/dist

BUILD_DIR="release/${VERSION}"
mkdir -p ${BUILD_DIR}

# Keep the version directory self-contained: every release ships the README
# screenshots alongside the packages.
if [ -d docs/screenshots ]; then
  mkdir -p "${BUILD_DIR}/screenshots"
  cp docs/screenshots/*.png "${BUILD_DIR}/screenshots/"
  echo "Copied screenshots"
fi

# Save original manifest
cp fnpack/manifest fnpack/manifest.bak

for ARCH in "amd64" "arm64"; do
  FILE_ARCH="$ARCH"
  if [ "$ARCH" = "amd64" ]; then FILE_ARCH="x86"; fi
  echo "Building fnOS package for ${ARCH}..."

  echo "  Compiling Go binary (CGO_ENABLED=1, linux/${ARCH})..."
  case "$ARCH" in
    amd64) CC_COMPILER="x86_64-linux-musl-gcc" ;;
    arm64) CC_COMPILER="aarch64-linux-musl-gcc" ;;
    *) echo "Unsupported arch: ${ARCH}"; exit 1 ;;
  esac
  if ! command -v "$CC_COMPILER" >/dev/null 2>&1; then
    echo "Error: ${CC_COMPILER} not found."
    echo "Install with: brew install FiloSottile/musl-cross/musl-cross"
    exit 1
  fi
  cd server
  GOCACHE="$LOCAL_GOCACHE" CGO_ENABLED=1 GOOS=linux GOARCH=$ARCH CC="$CC_COMPILER" \
    go build -ldflags "${LDFLAGS} -extldflags -static" -o "bill-linux-${ARCH}" .
  cd "$ROOT_DIR"

  # Prepare build directory
  BUILD_PACK="${BUILD_DIR}/${APP_NAME}_${ARCH}"
  rm -rf "${BUILD_PACK}"
  mkdir -p "${BUILD_PACK}"

  # Copy fnpack template (only essential directories)
  cp -r fnpack/cmd "${BUILD_PACK}/"
  cp -r fnpack/config "${BUILD_PACK}/"
  cp -r fnpack/wizard "${BUILD_PACK}/"
  mkdir -p "${BUILD_PACK}/app"
  cp -r fnpack/app/ui "${BUILD_PACK}/app/"
  cp fnpack/ICON.PNG "${BUILD_PACK}/"
  cp fnpack/ICON_256.PNG "${BUILD_PACK}/"

  # Copy binary
  cp "server/bill-linux-${ARCH}" "${BUILD_PACK}/app/bill"
  chmod +x "${BUILD_PACK}/app/bill"
  rm "server/bill-linux-${ARCH}"

  # Copy frontend to app/ui
  cp -r server/static/dist/* "${BUILD_PACK}/app/ui/"

  # Generate manifest with correct platform
  if [ "$ARCH" = "amd64" ]; then
    FNOS_PLATFORM="x86"
  else
    FNOS_PLATFORM="arm"
  fi
  sed "s/^platform.*/platform              = ${FNOS_PLATFORM}/" fnpack/manifest > "${BUILD_PACK}/manifest"

  # Update version in manifest
  sed -i '' "s/^version.*/version               = ${VERSION#v}/" "${BUILD_PACK}/manifest" 2>/dev/null || \
  sed -i "s/^version.*/version               = ${VERSION#v}/" "${BUILD_PACK}/manifest"

  # Strip macOS metadata so it never ships inside the package
  find "${BUILD_PACK}" -name '.DS_Store' -delete

  # Build with fnpack
  cd "${BUILD_PACK}"
  fnpack build
  cd "$ROOT_DIR"

  # Move the built fpk to release directory
  if [ -f "${BUILD_PACK}/${FNOS_PKG_NAME}.fpk" ]; then
    mv "${BUILD_PACK}/${FNOS_PKG_NAME}.fpk" "${BUILD_DIR}/${FNOS_PKG_NAME}_${VERSION}_${FILE_ARCH}.fpk"
  elif [ -f "${BUILD_PACK}/../${FNOS_PKG_NAME}.fpk" ]; then
    mv "${BUILD_PACK}/../${FNOS_PKG_NAME}.fpk" "${BUILD_DIR}/${FNOS_PKG_NAME}_${VERSION}_${FILE_ARCH}.fpk"
  fi

  # Clean up
  rm -rf "${BUILD_PACK}"

  echo "Built ${FNOS_PKG_NAME}_${VERSION}_${FILE_ARCH}.fpk"
done

# Restore original manifest
mv fnpack/manifest.bak fnpack/manifest

echo "fnOS packages completed in ${BUILD_DIR}/"
