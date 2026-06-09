# 本文件定义 Go 构建基座镜像,供 Chaimir 后端和 Go 工具镜像多阶段构建复用。
FROM golang:1.25-bookworm

LABEL org.opencontainers.image.title="base/go-builder"
LABEL org.opencontainers.image.description="Chaimir Go builder base image"

# 创建非 root 构建用户,避免构建脚本默认拥有 root 权限。
RUN groupadd --system chaimir \
    && useradd --system --gid chaimir --home-dir /src --create-home chaimir \
    && chown -R chaimir:chaimir /go /src

# 统一构建工作目录,减少各服务镜像重复创建目录逻辑。
WORKDIR /src
USER chaimir

# 构建镜像不运行平台进程,默认输出 Go 版本用于自检和排障。
CMD ["go", "version"]
