# 第一阶段负责编译 Go 服务。
#
# 设计来源：
# - Node Server 当前使用 modernc.org/sqlite，不依赖 CGO，本地和 ARM 目标机都更适合直接静态编译；
# - Settings、docs、public 都属于运行时事实文件，必须随镜像一起复制，避免部署后 Swagger 和配置说明缺失；
# - 2026-06-12 项目已经明确：正式 docker build 链路不能只改 apt / GOPROXY，FROM 也必须支持国内镜像入口；
# - 2026-06-13 对照 Client 正式 Dockerfile 后再次确认：清华 TUNA 当前不承担 Docker Hub 基础镜像入口，
#   且项目依赖在清华 goproxy 路径上会回退 direct，导致镜像构建长时间卡住；
# - 因此 Server 和 Client 保持同一正式口径：FROM 走可用国内 Docker Hub 镜像前缀，Go 依赖走 goproxy.cn。
#
# 职责边界：
# - DOCKERHUB_MIRROR 只控制基础镜像来源；
# - Go 依赖下载继续由 GOPROXY 控制；
# - 运行期 SQLite 目录固定挂载到 /app/data，由外部卷负责持久化，不在镜像里预置业务库。
ARG DOCKERHUB_MIRROR=docker.m.daocloud.io
FROM ${DOCKERHUB_MIRROR}/library/golang:1.22-bookworm AS builder

WORKDIR /src

ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}
ARG GOSUMDB=sum.golang.google.cn
ENV GOSUMDB=${GOSUMDB}

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-arm64} go build \
  -ldflags="-s -w -X private_browser_server/Settings.BuildEnv=docker" \
  -o /out/private_browser_server .

# 第二阶段改为 scratch 运行时。
#
# 设计原因：
# - 本次实测发现 builder 层正常，但运行时 debian 基础层在国内镜像链路上拉取极慢，正式交付会拖慢封版；
# - 当前 Server 二进制已静态编译，不依赖 libc，运行时只需要证书、时区库和项目静态资源；
# - SQLite 数据库仍由外挂卷 `/app/data` 持久化，镜像本身不内置业务数据。
#
# 维护边界：
# - 如果后续引入 CGO、本地共享库或系统命令依赖，不能继续直接使用 scratch，必须重新评估运行时基座；
# - 当前 scratch 方案只服务纯 Go Node Server 进程，不承担 shell 诊断职责。
FROM scratch AS runtime

WORKDIR /app

ENV ENV=docker
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
ENV ZONEINFO=/usr/share/zoneinfo

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /out/private_browser_server /app/private_browser_server
COPY Settings /app/Settings
COPY docs /app/docs
COPY public /app/public

EXPOSE 3400 43000/udp

CMD ["/app/private_browser_server"]
