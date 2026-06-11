# 本文件从 MinIO 官方源码构建对象存储服务镜像,避免依赖已停止更新的社区 Docker tag。
FROM golang:1.24.8-bookworm AS build

ARG MINIO_VERSION=RELEASE.2025-10-15T17-29-55Z
ENV CGO_ENABLED=0

# 固定源码 tag 构建可复现二进制;生产 digest 由 CI 推送后写入 image-digests.lock。
RUN git clone --branch "${MINIO_VERSION}" --depth 1 https://github.com/minio/minio.git /src/minio
WORKDIR /src/minio
RUN go build -trimpath -ldflags "-s -w" -o /out/minio .

FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.title="middleware/minio"
LABEL org.opencontainers.image.description="Chaimir MinIO object storage built from official source"

COPY --from=build --chown=nonroot:nonroot /out/minio /usr/local/bin/minio

USER nonroot:nonroot
WORKDIR /data
VOLUME ["/data"]
EXPOSE 9000 9001

ENTRYPOINT ["/usr/local/bin/minio"]
CMD ["server", "/data", "--console-address", ":9001"]
