# 本文件定义 Go 构建基座镜像,供 Chaimir 后端和 Go 工具镜像多阶段构建复用。
FROM golang:1.26.6-alpine@sha256:3889b425f035be855a72fb4755265311293b6d414521f0a519d819df32222d83

LABEL org.opencontainers.image.title="base/go-builder"
LABEL org.opencontainers.image.description="Chaimir Go builder base image"

# 官方固定 digest 已经包含 Go;项目构建链要求的 make、git 和 curl 在同一固定 Alpine 仓库内显式锁定安装。
RUN apk add --no-cache \
        ca-certificates=20260611-r0 \
        curl=8.22.0-r0 \
        git=2.54.0-r0 \
        make=4.4.1-r4 \
        openssl=3.5.8-r0 \
    && addgroup -g 1000 -S chaimir \
    && adduser -u 1000 -G chaimir -h /src -s /sbin/nologin -S chaimir \
    && mkdir -p /src /out /tmp/go-cache \
    && chown -R 1000:1000 /go /src /out /tmp/go-cache

# 统一构建工作目录,减少各服务镜像重复创建目录逻辑。
ENV GOROOT=/usr/local/go
ENV GOPATH=/go
ENV GOCACHE=/tmp/go-cache
ENV PATH=/usr/local/go/bin:/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
WORKDIR /src
USER 1000:1000

# 构建镜像不运行平台进程,默认输出 Go 版本用于自检和排障。
CMD ["go", "version"]
