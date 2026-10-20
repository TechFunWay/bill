#!/bin/bash
set -euo pipefail

# 多平台（linux/amd64 + linux/arm64）合并镜像。
#
# 默认只导出本地 OCI 归档（release/v<版本>/techfunway-bill-v<版本>-multiarch.oci.tar），
# 随发行目录分发，不碰 registry。PUSH=1（或 --push）时额外把同一份合并 manifest 推到
# Docker Hub（techfunways/bill 的 v<版本> 与 latest 两个 tag），需已 docker login。
#
# 与 notepad / rental 的做法一致：用 build-all.sh 已交叉编译好的静态二进制 +
# FROM scratch 组镜像，避免 buildx 在 qemu 下重跑 npm 与 go 编译。
# scratch 里没有的三样东西必须随镜像带上，否则功能会坏：
#   - /etc/ssl/cert.pem：匿名统计与支持事件走 https，缺 CA 证书直接失败；
#   - /usr/share/zoneinfo/Asia/Shanghai：Go 靠它把 TZ 解析成本地时区，账单按月/按日
#     分组与账单导入的时间解析都用 time.Local，缺了就是 UTC，跨零点的流水会归错日期；
#   - /tmp：账单导入用 excelize 读 xlsx，内部要 os.CreateTemp，scratch 不建 /tmp，
#     缺目录会让 Excel 导入报「权限拒绝」。

PUSH="${PUSH:-0}"
for arg in "$@"; do
  [ "$arg" = "--push" ] && PUSH=1
done

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

APP_NAME="techfunway-bill"
BINARY_NAME="bill"
IMAGE_NAME="techfunways/bill"
BUILDER_NAME="${BUILDER_NAME:-techfunway-bill-multiarch}"
VERSION="$(cat VERSION | tr -d '\n')"
RELEASE_DIR="${ROOT_DIR}/release/${VERSION}"
OCI_FILE="${RELEASE_DIR}/${APP_NAME}-${VERSION}-multiarch.oci.tar"
SERVICE_PORT="8907"

if ! command -v docker >/dev/null 2>&1; then
  echo "Error: Docker is required to build the multi-platform image."
  exit 1
fi

if [ ! -d "$RELEASE_DIR" ]; then
  echo "Missing ${RELEASE_DIR}. Build release archives first: bash scripts/build-all.sh"
  exit 1
fi

# build-all.sh 压缩后即清理中间平台目录；多平台构建需要 linux 平台目录，
# 缺失时从对应的 tar.gz 解包，用完在脚本末尾清理。
DOCKER_TMP_DIRS=()
for arch in amd64 arm64; do
  DIR_NAME="${APP_NAME}-${VERSION}-linux-${arch}"
  if [ ! -d "${RELEASE_DIR}/${DIR_NAME}" ]; then
    TAR_FILE="${RELEASE_DIR}/${DIR_NAME}.tar.gz"
    if [ ! -f "$TAR_FILE" ]; then
      echo "Missing ${TAR_FILE}. Run: bash scripts/build-all.sh"
      exit 1
    fi
    tar -xzf "$TAR_FILE" -C "${RELEASE_DIR}"
    DOCKER_TMP_DIRS+=("${RELEASE_DIR}/${DIR_NAME}")
  fi
done

find "${RELEASE_DIR}" -name '.DS_Store' -delete 2>/dev/null || true
rm -f "${OCI_FILE}"

# 合并 manifest 的 OCI 归档必须用 docker-container 驱动的 builder，
# 默认驱动不推 registry 就导不出多平台合并结果。
if ! docker buildx inspect "${BUILDER_NAME}" >/dev/null 2>&1; then
  echo "Creating buildx builder: ${BUILDER_NAME}"
  docker buildx create --name "${BUILDER_NAME}" --driver docker-container --bootstrap >/dev/null
fi

CONTEXT_DIR="${ROOT_DIR}/build/docker-multi-context"
rm -rf "${CONTEXT_DIR}"
mkdir -p "${CONTEXT_DIR}/payload"
cleanup() {
  rm -rf "${CONTEXT_DIR}"
  for d in "${DOCKER_TMP_DIRS[@]:-}"; do
    [ -n "$d" ] && rm -rf "$d"
  done
}
trap cleanup EXIT

# 组装镜像内容：二进制与前端按 TARGETARCH 选，其余是平台无关的补充文件。
for arch in amd64 arm64; do
  SRC="${RELEASE_DIR}/${APP_NAME}-${VERSION}-linux-${arch}"
  DST="${CONTEXT_DIR}/payload/${arch}"
  mkdir -p "${DST}/www"
  cp "${SRC}/${BINARY_NAME}" "${DST}/${BINARY_NAME}"
  chmod 755 "${DST}/${BINARY_NAME}"
  cp -r "${SRC}/www/." "${DST}/www/"
done

# CA 证书：本机（macOS）的系统信任库。
CA_SRC="/etc/ssl/cert.pem"
if [ ! -f "$CA_SRC" ]; then
  echo "Missing host CA bundle at ${CA_SRC}; cannot build a TLS-capable image."
  exit 1
fi
mkdir -p "${CONTEXT_DIR}/payload/etc/ssl"
cp "$CA_SRC" "${CONTEXT_DIR}/payload/etc/ssl/cert.pem"

# 时区数据：只需要本机存在的这几个文件，Go 按 TZ 名字到
# /usr/share/zoneinfo 下找同名文件。
ZONEINFO_SRC="/usr/share/zoneinfo"
mkdir -p "${CONTEXT_DIR}/payload/usr/share/zoneinfo/Asia"
for z in Asia/Shanghai UTC GMT; do
  if [ -f "${ZONEINFO_SRC}/${z}" ]; then
    mkdir -p "$(dirname "${CONTEXT_DIR}/payload/usr/share/zoneinfo/${z}")"
    cp "${ZONEINFO_SRC}/${z}" "${CONTEXT_DIR}/payload/usr/share/zoneinfo/${z}"
  else
    echo "Warning: ${ZONEINFO_SRC}/${z} not found; that zone falls back to UTC."
  fi
done

# 可写的临时目录与数据目录（scratch 里什么都没有，两个都要显式准备）。
mkdir -p "${CONTEXT_DIR}/payload/tmp" "${CONTEXT_DIR}/payload/data"
touch "${CONTEXT_DIR}/payload/tmp/.keep" "${CONTEXT_DIR}/payload/data/.keep"
chmod 1777 "${CONTEXT_DIR}/payload/tmp"

cat > "${CONTEXT_DIR}/Dockerfile" <<EOF
FROM scratch

ARG TARGETARCH
ARG VERSION

LABEL org.opencontainers.image.title="${APP_NAME}"
LABEL org.opencontainers.image.description="个人与多人共享账单、分摊与结算工具"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.source="https://github.com/TechFunWay/${APP_NAME#techfunway-}"

WORKDIR /app

COPY payload/\${TARGETARCH}/${BINARY_NAME} /app/${BINARY_NAME}
COPY payload/\${TARGETARCH}/www /app/static/dist
COPY payload/etc/ssl/cert.pem /etc/ssl/cert.pem
COPY payload/usr/share/zoneinfo /usr/share/zoneinfo
COPY --chmod=1777 payload/tmp /tmp
COPY --chown=1000:1000 payload/data /app/data

EXPOSE ${SERVICE_PORT}
VOLUME ["/app/data"]

ENV PORT=${SERVICE_PORT}
ENV DATA_DIR=/app/data
ENV TZ=Asia/Shanghai

USER 1000

ENTRYPOINT ["/app/${BINARY_NAME}"]
CMD ["-data-dir", "/app/data", "-web-dir", "/app/static/dist"]
EOF

echo "Building ${IMAGE_NAME}:${VERSION} for linux/amd64,linux/arm64..."
echo "Writing local OCI image archive (no registry push): ${OCI_FILE}"

docker buildx build \
  --builder "${BUILDER_NAME}" \
  --platform linux/amd64,linux/arm64 \
  --build-arg "VERSION=${VERSION}" \
  -t "${IMAGE_NAME}:${VERSION}" \
  -t "${IMAGE_NAME}:latest" \
  --output "type=oci,dest=${OCI_FILE}" \
  "${CONTEXT_DIR}"

echo "Multi-platform OCI image archive completed: ${OCI_FILE}"
echo "Load it on a target with: docker load -i ${APP_NAME}-${VERSION}-multiarch.oci.tar"

# PUSH=1：把同一份多平台合并 manifest 推到 Docker Hub（需已 docker login）
if [ "${PUSH}" = "1" ]; then
  echo ""
  echo "Pushing multi-platform image to Docker Hub: ${IMAGE_NAME}:${VERSION} / :latest"
  docker buildx build \
    --builder "${BUILDER_NAME}" \
    --platform linux/amd64,linux/arm64 \
    --push \
    --build-arg "VERSION=${VERSION}" \
    -t "${IMAGE_NAME}:${VERSION}" \
    -t "${IMAGE_NAME}:latest" \
    "${CONTEXT_DIR}"
  echo "Pushed ${IMAGE_NAME}:${VERSION} and ${IMAGE_NAME}:latest (amd64+arm64 merged manifest)"
fi
