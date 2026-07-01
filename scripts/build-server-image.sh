#!/usr/bin/env bash

set -euo pipefail

# build-server-image.sh 负责统一收口 Private_Browser_Server 的正式 Docker 构建流程。
#
# 设计来源：
# - Node Server 当前已经进入可部署阶段，需要和 Client 一样拥有稳定的 buildx 构建入口；
# - 项目协作规范要求基础镜像入口、Debian 源、Go proxy 三层都可配置；
# - 因此这里把平台、镜像名、tag、load/push 和源配置统一收进脚本，避免 README、人工命令和 CI 写散。
#
# 职责边界：
# - 这个脚本只负责构建镜像；
# - 不负责运行容器，不修改数据库，不生成或改写业务配置；
# - 默认 `--load` 方便本地调试，正式发布用 `--push`。

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DOCKERFILE="${PROJECT_ROOT}/Dockerfile"

IMAGE_NAME="private-browser-node-server"
IMAGE_TAG="latest"
PLATFORM="linux/amd64"
OUTPUT_MODE="load"

DOCKERHUB_MIRROR="${DOCKERHUB_MIRROR:-docker.m.daocloud.io}"
DEBIAN_MIRROR="${DEBIAN_MIRROR:-mirrors.tuna.tsinghua.edu.cn}"
GOPROXY_VALUE="${GOPROXY:-https://goproxy.cn,direct}"
GOSUMDB_VALUE="${GOSUMDB:-sum.golang.google.cn}"

usage() {
  cat <<'EOF'
Usage:
  ./scripts/build-server-image.sh [options]

Options:
  --platform <value>     Target platform, default: linux/amd64
  --image <value>        Image name, default: private-browser-node-server
  --tag <value>          Image tag, default: latest
  --push                 Build and push image
  --load                 Build and load image to local Docker, default
  --output <load|push>   Explicit output mode
  --dockerfile <path>    Custom Dockerfile path
  --help                 Show help

Environment overrides:
  DOCKERHUB_MIRROR
  DEBIAN_MIRROR
  GOPROXY
  GOSUMDB

Examples:
  ./scripts/build-server-image.sh
  ./scripts/build-server-image.sh --platform linux/amd64 --image private-browser-node-server --tag amd64
  ./scripts/build-server-image.sh --platform linux/arm64 --image private-browser-node-server --tag arm64
  ./scripts/build-server-image.sh --platform linux/arm/v7 --image private-browser-node-server --tag arm-v7
  ./scripts/build-server-image.sh --platform linux/amd64 --image repo/private-browser-node-server --tag 0.1.1-amd64 --push
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --platform)
      PLATFORM="${2:?missing value for --platform}"
      shift 2
      ;;
    --image)
      IMAGE_NAME="${2:?missing value for --image}"
      shift 2
      ;;
    --tag)
      IMAGE_TAG="${2:?missing value for --tag}"
      shift 2
      ;;
    --push)
      OUTPUT_MODE="push"
      shift
      ;;
    --load)
      OUTPUT_MODE="load"
      shift
      ;;
    --output)
      OUTPUT_MODE="${2:?missing value for --output}"
      shift 2
      ;;
    --dockerfile)
      DOCKERFILE="${2:?missing value for --dockerfile}"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "unknown option: $1" >&2
      usage
      exit 1
      ;;
  esac
done

if [[ "${OUTPUT_MODE}" != "load" && "${OUTPUT_MODE}" != "push" ]]; then
  echo "invalid --output value: ${OUTPUT_MODE}, expected load or push" >&2
  exit 1
fi

if [[ ! -f "${DOCKERFILE}" ]]; then
  echo "dockerfile not found: ${DOCKERFILE}" >&2
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "docker command not found" >&2
  exit 1
fi

if ! docker buildx version >/dev/null 2>&1; then
  echo "docker buildx is required but not available" >&2
  exit 1
fi

if [[ "${PLATFORM}" == "linux/arm" ]]; then
  echo "warning: --platform linux/arm usually means 32-bit ARM. If your server is 64-bit ARM, use linux/arm64." >&2
fi

FULL_IMAGE="${IMAGE_NAME}:${IMAGE_TAG}"

echo "==> project root: ${PROJECT_ROOT}"
echo "==> dockerfile:   ${DOCKERFILE}"
echo "==> image:        ${FULL_IMAGE}"
echo "==> platform:     ${PLATFORM}"
echo "==> output:       ${OUTPUT_MODE}"
echo "==> mirror:       ${DOCKERHUB_MIRROR}"
echo "==> apt mirror:   ${DEBIAN_MIRROR}"
echo "==> goproxy:      ${GOPROXY_VALUE}"
echo "==> gosumdb:      ${GOSUMDB_VALUE}"

BUILD_ARGS=(
  --build-arg "DOCKERHUB_MIRROR=${DOCKERHUB_MIRROR}"
  --build-arg "DEBIAN_MIRROR=${DEBIAN_MIRROR}"
  --build-arg "GOPROXY=${GOPROXY_VALUE}"
  --build-arg "GOSUMDB=${GOSUMDB_VALUE}"
)

OUTPUT_ARGS=(--load)
if [[ "${OUTPUT_MODE}" == "push" ]]; then
  OUTPUT_ARGS=(--push)
fi

docker buildx build \
  --platform "${PLATFORM}" \
  -f "${DOCKERFILE}" \
  -t "${FULL_IMAGE}" \
  "${BUILD_ARGS[@]}" \
  "${OUTPUT_ARGS[@]}" \
  "${PROJECT_ROOT}"
