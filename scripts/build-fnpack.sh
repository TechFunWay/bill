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
    mv "${BUILD_PACK}/${FNOS_PKG_NAME}.fpk" "${BUILD_DIR}/${FNOS_PKG_NAME}_${VERSION}_${ARCH}.fpk"
  elif [ -f "${BUILD_PACK}/../${FNOS_PKG_NAME}.fpk" ]; then
    mv "${BUILD_PACK}/../${FNOS_PKG_NAME}.fpk" "${BUILD_DIR}/${FNOS_PKG_NAME}_${VERSION}_${ARCH}.fpk"
  fi

  # Clean up
  rm -rf "${BUILD_PACK}"

  echo "Built ${FNOS_PKG_NAME}_${VERSION}_${ARCH}.fpk"
done

# Restore original manifest
mv fnpack/manifest.bak fnpack/manifest

echo "fnOS packages completed in ${BUILD_DIR}/"
