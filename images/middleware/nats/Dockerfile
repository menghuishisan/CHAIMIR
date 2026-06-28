# 本文件从 NATS 官方源码重建 nats-server,保留官方 Alpine 运行契约并升级 Go 标准库。
ARG GO_BUILDER_IMAGE
FROM ${GO_BUILDER_IMAGE} AS build

ARG NATS_VERSION=v2.14.2
ARG GO_MODULE_PROXY=
ENV CGO_ENABLED=0

# 固定官方源码 tag 构建 nats-server,生产 digest 由 CI 推送后写入 image-digests.lock。
USER root
RUN mkdir -p /out && chown 1000:1000 /out
USER 1000:1000
RUN git clone --branch "${NATS_VERSION}" --depth 1 https://github.com/nats-io/nats-server.git /src/nats-server
WORKDIR /src/nats-server
RUN if [ -n "${GO_MODULE_PROXY}" ]; then go env -w GOPROXY="${GO_MODULE_PROXY}"; fi \
    && go build -trimpath -ldflags "-s -w" -o /out/nats-server ./main.go

FROM alpine:3.23

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
