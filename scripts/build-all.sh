#!/bin/bash
set -e

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

APP_NAME="bill"
PACKAGE_PREFIX="techfunway-bill"
VERSION=$(cat VERSION | tr -d '\n')
BUILD_TIME=$(date +%Y-%m-%dT%H:%M:%S)
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS="-X smallgo/server/version.Version=${VERSION} -X smallgo/server/version.BuildTime=${BUILD_TIME} -X smallgo/server/version.GitCommit=${GIT_COMMIT} -X smallgo/server/version.AppName=${APP_NAME}"
LOCAL_GOCACHE="${GOCACHE:-/tmp/bill-go-build}"

echo "Building frontend..."
cd web && npm ci && npm run build && cd ..

echo "Copying frontend..."
rm -rf server/static/dist
cp -r web/dist server/static/dist

BUILD_DIR="release/${VERSION}"
rm -rf "${BUILD_DIR}"
mkdir -p "${BUILD_DIR}"

# Ship the README screenshots with every release so the version directory is
# self-contained (the tracked source of truth stays docs/screenshots/).
if [ -d docs/screenshots ]; then
  mkdir -p "${BUILD_DIR}/screenshots"
  cp docs/screenshots/*.png "${BUILD_DIR}/screenshots/"
  echo "Copied screenshots"
fi

PLATFORMS=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
)

for PLATFORM in "${PLATFORMS[@]}"; do
  IFS="/" read -r GOOS GOARCH <<< "$PLATFORM"
  OUTPUT_NAME="${PACKAGE_PREFIX}-${VERSION}-${GOOS}-${GOARCH}"
  BINARY_NAME="${APP_NAME}-${GOOS}-${GOARCH}"

  echo "Building ${OUTPUT_NAME}..."

  if [ "$GOOS" = "linux" ]; then
    # Local cross-compilation with musl-cross (static CGO binaries).
    case "$GOARCH" in
      amd64) CC_COMPILER="x86_64-linux-musl-gcc" ;;
      arm64) CC_COMPILER="aarch64-linux-musl-gcc" ;;
      *) echo "Unsupported linux arch: ${GOARCH}"; exit 1 ;;
    esac
    if ! command -v "$CC_COMPILER" >/dev/null 2>&1; then
      echo "Error: ${CC_COMPILER} not found."
      echo "Install with: brew install FiloSottile/musl-cross/musl-cross"
      exit 1
    fi
    cd server
    GOCACHE="$LOCAL_GOCACHE" CGO_ENABLED=1 GOOS=$GOOS GOARCH=$GOARCH CC="$CC_COMPILER" \
      go build -ldflags "${LDFLAGS} -extldflags -static" -o "${BINARY_NAME}" .
    cd "$ROOT_DIR"
  elif [ "$GOOS" = "windows" ]; then
    cd server
    GOCACHE="$LOCAL_GOCACHE" GOOS=$GOOS GOARCH=$GOARCH go build -ldflags "${LDFLAGS}" -o "${BINARY_NAME}.exe" .
    cd "$ROOT_DIR"
  else
    cd server
    GOCACHE="$LOCAL_GOCACHE" CGO_ENABLED=1 GOOS=$GOOS GOARCH=$GOARCH go build -ldflags "${LDFLAGS}" -o "${BINARY_NAME}" .
    cd "$ROOT_DIR"
  fi

  mkdir -p "${BUILD_DIR}/${OUTPUT_NAME}"

  if [ "$GOOS" = "windows" ]; then
    cp "server/${BINARY_NAME}.exe" "${BUILD_DIR}/${OUTPUT_NAME}/bill.exe"
    rm "server/${BINARY_NAME}.exe"
  else
    cp "server/${BINARY_NAME}" "${BUILD_DIR}/${OUTPUT_NAME}/bill"
    rm "server/${BINARY_NAME}"
  fi

  cp -r server/static/dist "${BUILD_DIR}/${OUTPUT_NAME}/www"

  cd "${ROOT_DIR}/${BUILD_DIR}"
  if [ "$GOOS" = "windows" ]; then
    zip -r "${OUTPUT_NAME}.zip" "${OUTPUT_NAME}"
  else
    tar czf "${OUTPUT_NAME}.tar.gz" "${OUTPUT_NAME}"
  fi
  cd "$ROOT_DIR"

  rm -rf "${BUILD_DIR}/${OUTPUT_NAME}"

  echo "Built ${OUTPUT_NAME}"
done

# Copy the deployment compose files into the release directory. The version
# file gets its image tag replaced with the current version so it can run as
# soon as it is copied to the target host; the latest file tracks the latest
# image with Watchtower and is copied as-is.
sed "s|techfunways/bill:latest|techfunways/bill:${VERSION}|" deploy/docker-compose.yml > "${BUILD_DIR}/docker-compose.yml"
cp deploy/docker-compose.latest.yml "${BUILD_DIR}/docker-compose.latest.yml"
echo "Copied docker-compose.yml and docker-compose.latest.yml"

echo "All builds completed in ${BUILD_DIR}/"
