# 本文件定义链工具公共基座,供需要通用 CLI 工具的 runtime/infra 镜像继承。
FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40

LABEL org.opencontainers.image.title="base/chain-tools"
LABEL org.opencontainers.image.description="Chaimir blockchain toolchain base image"

# 安装通用链实验工具,避免每个运行时镜像重复安装同一批基础命令。
RUN apk add --no-cache ca-certificates curl=8.22.0-r0 libexpat=2.8.4-r0 jq=1.8.2-r0 bash git openssl tar findutils coreutils \
    && addgroup -g 1000 -S chaimir \
    && adduser -u 1000 -G chaimir -h /workspace -s /sbin/nologin -S chaimir \
    && chown -R 1000:1000 /workspace

WORKDIR /workspace
USER 1000:1000

# 该镜像只作为构建和网络诊断输入,不提供沙箱工作区或学生入口。
CMD ["jq", "--version"]
