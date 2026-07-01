# 第一阶段负责编译 Node Server Go 服务。
#
# 设计来源：
# - Node Server 当前和 Client 一样依赖 SQLite CGO 驱动，正式镜像不能用纯静态极简构建糊过去；
# - `Settings`、`docs`、`public` 是运行时事实文件，必须随镜像复制，保证 `/swagger`、`/scalar`、`/openapi.yaml`
#   在容器部署后仍然可访问；
# - 根据当前仓库协作规范，正式构建链路必须同时收口基础镜像入口、Debian 源和 Go proxy，不能只改容器内 apt。
#
# 职责边界：
# - `DOCKERHUB_MIRROR` 只负责基础镜像入口；
# - `DEBIAN_MIRROR` 只负责容器内 apt 源；
# - `GOPROXY/GOSUMDB` 只负责 Go 依赖下载；
# - 具体构建 amd64、arm64 还是 arm，由外层 buildx 的 `--platform` 统一决定。
ARG DOCKERHUB_MIRROR=docker.m.daocloud.io
FROM ${DOCKERHUB_MIRROR}/library/golang:1.23-bookworm AS builder

WORKDIR /src

ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}

ARG GOSUMDB=sum.golang.google.cn
ENV GOSUMDB=${GOSUMDB}

ARG DEBIAN_MIRROR=mirrors.tuna.tsinghua.edu.cn
RUN if [ "${DEBIAN_MIRROR}" = "deb.debian.org" ]; then DEBIAN_SCHEME="http"; else DEBIAN_SCHEME="https"; fi \
  && sed -i "s|http://deb.debian.org/debian|${DEBIAN_SCHEME}://${DEBIAN_MIRROR}/debian|g" /etc/apt/sources.list.d/debian.sources \
  && sed -i "s|http://deb.debian.org/debian-security|${DEBIAN_SCHEME}://${DEBIAN_MIRROR}/debian-security|g" /etc/apt/sources.list.d/debian.sources \
  && apt-get update \
  && apt-get install -y --no-install-recommends gcc libc6-dev \
  && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# buildx 会按 `--platform` 注入 TARGETOS/TARGETARCH/TARGETVARIANT。
#
# 这里特别处理 `linux/arm`：
# - `linux/arm64` 会得到 TARGETARCH=arm64，不需要 GOARM；
# - `linux/arm` 会得到 TARGETARCH=arm，必要时从 TARGETVARIANT=v7/v6 推导 GOARM；
# - 如果未指定 variant，Go 默认 GOARM 通常可用，但正式发布仍建议写清 `linux/arm64` 或 `linux/arm/v7`。
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
RUN set -eux; \
  GOOS_VALUE="${TARGETOS:-linux}"; \
  GOARCH_VALUE="${TARGETARCH:-$(go env GOARCH)}"; \
  if [ "${GOARCH_VALUE}" = "arm" ] && [ -n "${TARGETVARIANT}" ]; then \
    export GOARM="${TARGETVARIANT#v}"; \
  fi; \
  CGO_ENABLED=1 GOOS="${GOOS_VALUE}" GOARCH="${GOARCH_VALUE}" go build \
    -o /out/private_browser_server .

FROM ${DOCKERHUB_MIRROR}/library/debian:bookworm-slim AS runtime

WORKDIR /app

ARG DEBIAN_MIRROR=mirrors.tuna.tsinghua.edu.cn
RUN if [ "${DEBIAN_MIRROR}" = "deb.debian.org" ]; then DEBIAN_SCHEME="http"; else DEBIAN_SCHEME="https"; fi \
  && sed -i "s|http://deb.debian.org/debian|${DEBIAN_SCHEME}://${DEBIAN_MIRROR}/debian|g" /etc/apt/sources.list.d/debian.sources \
  && sed -i "s|http://deb.debian.org/debian-security|${DEBIAN_SCHEME}://${DEBIAN_MIRROR}/debian-security|g" /etc/apt/sources.list.d/debian.sources \
  && apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates tzdata sqlite3 \
  && rm -rf /var/lib/apt/lists/*

ENV ENV=docker

COPY --from=builder /out/private_browser_server /app/private_browser_server
COPY Settings /app/Settings
COPY docs /app/docs
COPY public /app/public

RUN mkdir -p /app/data

EXPOSE 3400/tcp
EXPOSE 43000/udp

CMD ["/app/private_browser_server"]
