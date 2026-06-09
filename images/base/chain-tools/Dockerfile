# 本文件定义链工具公共基座,供需要通用 CLI 工具的 runtime/infra 镜像继承。
FROM ubuntu:24.04

LABEL org.opencontainers.image.title="base/chain-tools"
LABEL org.opencontainers.image.description="Chaimir blockchain toolchain base image"

# 安装通用链实验工具,避免每个运行时镜像重复安装同一批基础命令。
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl jq bash git openssl \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --system chaimir \
    && useradd --system --gid chaimir --home-dir /workspace --create-home chaimir

WORKDIR /workspace
USER chaimir

# 公共工具镜像默认输出工具版本,用于构建后自检。
CMD ["bash", "-lc", "curl --version >/dev/null && jq --version"]
