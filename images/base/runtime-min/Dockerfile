# 本文件定义最小运行时基座镜像,用于纯静态二进制服务的最终阶段。
FROM gcr.io/distroless/static-debian12:nonroot@sha256:b7bb25d9f7c31d2bdd1982feb4dafcaf137703c7075dbe2febb41c24212b946f

LABEL org.opencontainers.image.title="base/runtime-min"
LABEL org.opencontainers.image.description="Chaimir minimal non-root runtime base image"

# distroless nonroot 镜像已内置非 root 用户,这里不再额外创建用户。
USER nonroot:nonroot
