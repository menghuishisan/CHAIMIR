# 本文件定义链工具公共基座,供需要通用 CLI 工具的 runtime/infra 镜像继承。
FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40

LABEL org.opencontainers.image.title="base/chain-tools"
LABEL org.opencontainers.image.description="Chaimir blockchain toolchain base image"

# 安装通用链实验工具,避免每个运行时镜像重复安装同一批基础命令。
RUN apk upgrade --no-cache \
    && apk add --no-cache ca-certificates curl jq bash git openssl \
    && addgroup -g 1000 -S chaimir \
    && adduser -u 1000 -G chaimir -h /workspace -s /sbin/nologin -S chaimir \
    && chown -R 1000:1000 /workspace

WORKDIR /workspace
USER 1000:1000

# 公共工具镜像默认输出工具版本,用于构建后自检。
CMD ["bash", "-lc", "curl --version >/dev/null && jq --version"]
