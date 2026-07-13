# 本文件从 MinIO 官方源码构建对象存储服务镜像,避免依赖已停止更新的社区 Docker tag。
ARG GO_BUILDER_IMAGE=invalid.invalid/base/go-builder:required
FROM ${GO_BUILDER_IMAGE} AS build

ARG MINIO_COMMIT=7aac2a2c5b7c882e68c1ce017d8256be2feea27f
ARG MINIO_VERSION=RELEASE.2026-06-27T00-00-00Z.chaimir.7aac2a2c5b7c
ARG GO_MODULE_PROXY=
ENV CGO_ENABLED=0
ENV GOMAXPROCS=2

# 固定官方源码 commit 构建可复现二进制;生产 digest 由 CI 推送后写入 image-digests.lock。
RUN mkdir -p /src/minio \
    && curl -fL --retry 5 --retry-delay 3 --connect-timeout 30 --max-time 900 \
        -o /tmp/minio.tar.gz "https://github.com/minio/minio/archive/${MINIO_COMMIT}.tar.gz" \
    && tar -xzf /tmp/minio.tar.gz -C /src/minio --strip-components=1 \
    && rm -f /tmp/minio.tar.gz
WORKDIR /src/minio
RUN if [ -n "${GO_MODULE_PROXY}" ]; then go env -w GOPROXY="${GO_MODULE_PROXY}"; fi \
    && go get \
        github.com/apache/thrift@v0.23.0 \
        github.com/buger/jsonparser@v1.1.2 \
        github.com/go-jose/go-jose/v4@v4.1.4 \
        github.com/prometheus/prometheus@v0.311.3 \
        go.opentelemetry.io/otel/sdk@v1.43.0 \
        google.golang.org/grpc@v1.79.3 \
        golang.org/x/crypto@v0.52.0 \
        golang.org/x/net@v0.55.0 \
    && GOFLAGS=-mod=mod go build -tags kqueue -p 2 -trimpath \
        -ldflags "-s -w -X github.com/minio/minio/cmd.Version=${MINIO_VERSION} -X github.com/minio/minio/cmd.ReleaseTag=${MINIO_VERSION} -X github.com/minio/minio/cmd.CommitID=${MINIO_COMMIT} -X github.com/minio/minio/cmd.ShortCommitID=7aac2a2c5b7c" \
        -o /out/minio .

FROM gcr.io/distroless/static-debian12:nonroot@sha256:b7bb25d9f7c31d2bdd1982feb4dafcaf137703c7075dbe2febb41c24212b946f

LABEL org.opencontainers.image.title="middleware/minio"
LABEL org.opencontainers.image.description="Chaimir MinIO object storage built from official source"

COPY --from=build --chown=nonroot:nonroot /out/minio /usr/local/bin/minio

USER nonroot:nonroot
WORKDIR /data
VOLUME ["/data"]
EXPOSE 9000 9001

ENTRYPOINT ["/usr/local/bin/minio"]
CMD ["server", "/data", "--console-address", ":9001"]
