# 本文件定义 Go 构建基座镜像,供 Chaimir 后端和 Go 工具镜像多阶段构建复用。
FROM debian:12-slim

LABEL org.opencontainers.image.title="base/go-builder"
LABEL org.opencontainers.image.description="Chaimir Go builder base image"

ARG GO_VERSION=go1.26.4
ARG GO_SHA256=1153d3d50e0ac764b447adfe05c2bcf08e889d42a02e0fe0259bd47f6733ad7f

ADD https://go.dev/dl/${GO_VERSION}.linux-amd64.tar.gz /tmp/go.tar.gz

# 构建基座必须应用发行版安全更新,并用官方 SHA256 校验 Go 工具链。
RUN apt-get update \
    && apt-get upgrade -y --no-install-recommends \
    && apt-get install -y --no-install-recommends ca-certificates git \
    && rm -rf /var/lib/apt/lists/* \
    && echo "${GO_SHA256}  /tmp/go.tar.gz" | sha256sum -c - \
    && tar -C /usr/local -xzf /tmp/go.tar.gz \
    && rm -f /tmp/go.tar.gz \
    && groupadd --gid 1000 chaimir \
    && useradd --uid 1000 --gid 1000 --home-dir /src --create-home --shell /usr/sbin/nologin chaimir \
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
