# 本文件从 NATS 官方源码重建 nats-server,保留官方 Alpine 运行契约并升级 Go 标准库。
ARG GO_BUILDER_IMAGE=invalid.invalid/base/go-builder:required
ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY
FROM ${GO_BUILDER_IMAGE} AS build

ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY

ARG NATS_VERSION=v2.14.3
ARG NATS_COMMIT=9b17a586be69c9ff36e4c84a421ca47da410b561
ARG GO_MODULE_PROXY=
ENV CGO_ENABLED=0

# 固定官方源码 tag 构建 nats-server,生产 digest 由 CI 推送后写入 image-digests.lock。
USER root
RUN mkdir -p /out && chown 1000:1000 /out
USER 1000:1000
RUN export http_proxy="${HTTP_PROXY}" https_proxy="${HTTPS_PROXY}" no_proxy="${NO_PROXY}" \
    && for attempt in 1 2 3 4 5 6 7 8; do \
        rm -rf /src/nats-server; \
        if git clone --branch "${NATS_VERSION}" --depth 1 https://github.com/nats-io/nats-server.git /src/nats-server; then break; fi; \
        if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
        sleep $((attempt * 3)); \
    done \
    && test "$(git -C /src/nats-server rev-parse HEAD)" = "${NATS_COMMIT}"
WORKDIR /src/nats-server
RUN export http_proxy="${HTTP_PROXY}" https_proxy="${HTTPS_PROXY}" no_proxy="${NO_PROXY}" \
    && if [ -n "${GO_MODULE_PROXY}" ]; then go env -w GOPROXY="${GO_MODULE_PROXY}"; fi \
    && grep -qxF 'golang.org/x/crypto v0.55.0 h1:+KWHjbgOaAQ66dh/YlkZKHlz9ZUlq61AFirAR9ntP8M=' go.sum || echo 'golang.org/x/crypto v0.55.0 h1:+KWHjbgOaAQ66dh/YlkZKHlz9ZUlq61AFirAR9ntP8M=' >> go.sum \
    && grep -qxF 'golang.org/x/crypto v0.55.0/go.mod h1:uq0V9dE/fzQuJtbnL+2EhWOE63vo164FY8xqEnV9xis=' go.sum || echo 'golang.org/x/crypto v0.55.0/go.mod h1:uq0V9dE/fzQuJtbnL+2EhWOE63vo164FY8xqEnV9xis=' >> go.sum \
    && for attempt in 1 2 3 4 5 6 7 8; do \
        if go get golang.org/x/crypto@v0.55.0; then break; fi; \
        if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
        sleep $((attempt * 3)); \
    done \
    && for attempt in 1 2 3 4 5 6 7 8; do \
        if go build -trimpath -ldflags "-s -w" -o /out/nats-server ./main.go; then break; fi; \
        if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
        sleep $((attempt * 3)); \
    done

FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40

ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY

LABEL org.opencontainers.image.title="middleware/nats"
LABEL org.opencontainers.image.description="Chaimir NATS middleware built from official source"

RUN --mount=type=cache,target=/var/cache/apk,sharing=locked \
    export http_proxy="${HTTP_PROXY}" https_proxy="${HTTPS_PROXY}" no_proxy="${NO_PROXY}" \
    && for attempt in 1 2 3 4 5 6 7 8; do \
        apk add --cache-dir /var/cache/apk --update-cache ca-certificates openssl=3.5.8-r0 && break; \
        if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
        sleep $((attempt * 3)); \
    done \
    && addgroup -S -g 1000 nats \
    && adduser -S -u 1000 -G nats -h /data nats \
    && mkdir -p /data \
    && chown -R 1000:1000 /data

COPY --from=build --chown=1000:1000 /out/nats-server /usr/local/bin/nats-server
COPY --chown=1000:1000 nats-entrypoint.sh /usr/local/bin/chaimir-nats-entrypoint
COPY --chown=1000:1000 nats-probe.sh /usr/local/bin/chaimir-nats-probe
RUN chmod 0555 /usr/local/bin/chaimir-nats-entrypoint /usr/local/bin/chaimir-nats-probe

USER 1000:1000
WORKDIR /data
EXPOSE 4222 8222
ENTRYPOINT ["/usr/local/bin/chaimir-nats-entrypoint"]
CMD ["--jetstream", "--store_dir", "/data", "--http_port", "8222"]
