# 本文件定义 Go 构建基座镜像,供 Chaimir 后端和 Go 工具镜像多阶段构建复用。
FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40

LABEL org.opencontainers.image.title="base/go-builder"
LABEL org.opencontainers.image.description="Chaimir Go builder base image"

ARG GO_VERSION=go1.26.5
ARG GO_SHA256=5c2c3b16caefa1d968a94c1daca04a7ca301a496d9b086e17ad77bb81393f053

# 构建基座必须应用发行版安全更新,并用官方 SHA256 校验 Go 工具链。
RUN apk upgrade --no-cache \
    && apk add --no-cache ca-certificates curl git \
    && curl -fL --retry 8 --retry-all-errors --retry-delay 3 --connect-timeout 30 --max-time 600 \
        -o /tmp/go.tar.gz "https://go.dev/dl/${GO_VERSION}.linux-amd64.tar.gz" \
    && echo "${GO_SHA256}  /tmp/go.tar.gz" | sha256sum -c - \
    && tar -C /usr/local -xzf /tmp/go.tar.gz \
    && rm -f /tmp/go.tar.gz \
    && addgroup -g 1000 -S chaimir \
    && adduser -u 1000 -G chaimir -h /src -s /sbin/nologin -S chaimir \
    && mkdir -p /go /src /out /tmp/go-cache \
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
