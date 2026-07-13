# 本文件从 NATS 官方源码重建 nats-server,保留官方 Alpine 运行契约并升级 Go 标准库。
ARG GO_BUILDER_IMAGE=invalid.invalid/base/go-builder:required
FROM ${GO_BUILDER_IMAGE} AS build

ARG NATS_VERSION=v2.14.3
ARG NATS_COMMIT=9b17a586be69c9ff36e4c84a421ca47da410b561
ARG GO_MODULE_PROXY=
ENV CGO_ENABLED=0

# 固定官方源码 tag 构建 nats-server,生产 digest 由 CI 推送后写入 image-digests.lock。
USER root
RUN mkdir -p /out && chown 1000:1000 /out
USER 1000:1000
RUN git clone --branch "${NATS_VERSION}" --depth 1 https://github.com/nats-io/nats-server.git /src/nats-server \
    && test "$(git -C /src/nats-server rev-parse HEAD)" = "${NATS_COMMIT}"
WORKDIR /src/nats-server
RUN if [ -n "${GO_MODULE_PROXY}" ]; then go env -w GOPROXY="${GO_MODULE_PROXY}"; fi \
    && go build -trimpath -ldflags "-s -w" -o /out/nats-server ./main.go

FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40

LABEL org.opencontainers.image.title="middleware/nats"
LABEL org.opencontainers.image.description="Chaimir NATS middleware built from official source"

RUN apk add --no-cache ca-certificates \
    && addgroup -S -g 1000 nats \
    && adduser -S -u 1000 -G nats -h /data nats \
    && mkdir -p /data \
    && chown -R 1000:1000 /data

COPY --from=build --chown=1000:1000 /out/nats-server /usr/local/bin/nats-server

USER 1000:1000
WORKDIR /data
EXPOSE 4222 8222
ENTRYPOINT ["/usr/local/bin/nats-server"]
CMD ["--jetstream", "--store_dir", "/data", "--http_port", "8222"]
