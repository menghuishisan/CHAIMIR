# 本文件定义判题执行器安全基座,供具体 judger 镜像继承。
FROM alpine:3.20

LABEL org.opencontainers.image.title="base/judge-min"
LABEL org.opencontainers.image.description="Chaimir minimal judge runner base image"

# 安装证书和基础 shell 工具,满足判题执行器拉取受控依赖与执行脚本的最小需要。
RUN apk add --no-cache ca-certificates bash coreutils \
    && addgroup -S chaimir \
    && adduser -S -G chaimir -h /workspace chaimir

WORKDIR /workspace
USER chaimir

# 基座镜像只做环境自检,不承担具体判题任务。
CMD ["bash", "-lc", "id -u && test -d /workspace"]
