# 本文件从 Prometheus 官方固定源码构建完整 Web UI 与安全二进制,并放入最小 Wolfi 运行层。
ARG GO_BUILDER_IMAGE=invalid.invalid/base/go-builder:required
ARG NODE_BUILDER_IMAGE=invalid.invalid/base/node-builder:required

FROM ${NODE_BUILDER_IMAGE} AS assets

ARG PROMETHEUS_SOURCE_URL=https://github.com/prometheus/prometheus/archive/refs/tags/v3.12.0.tar.gz
ARG PROMETHEUS_SOURCE_SHA256=ca7a8dd2c57048bb952a493a2957811c6f380089c2b158c2def484b874c3b6d7

USER root
RUN apk add --no-cache bash=5.3.3-r1 gzip=1.14-r2
USER chaimir

# Prometheus 的生产静态资源不发布 source map;限制 Node 堆避免 Docker Desktop 在资源紧张时断开 BuildKit 状态流。
ENV GENERATE_SOURCEMAP=false \
    NODE_OPTIONS=--max-old-space-size=2048

ADD --chown=100:101 --chmod=0644 ${PROMETHEUS_SOURCE_URL} /tmp/prometheus-source.tar.gz
RUN echo "${PROMETHEUS_SOURCE_SHA256}  /tmp/prometheus-source.tar.gz" | sha256sum -c - \
    && mkdir -p /src/prometheus \
    && tar -xzf /tmp/prometheus-source.tar.gz -C /src/prometheus --strip-components=1 \
    && rm -f /tmp/prometheus-source.tar.gz

WORKDIR /src/prometheus/web/ui
RUN --mount=type=cache,target=/src/.npm,uid=100,gid=101 \
    for attempt in 1 2 3 4 5 6 7 8; do \
        if npm_config_cache=/src/.npm npm ci --ignore-scripts --no-audit --no-fund \
            && cd react-app \
            && npm_config_cache=/src/.npm npm ci --ignore-scripts --no-audit --no-fund; then \
            break; \
        fi; \
        cd /src/prometheus/web/ui; \
        if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
        sleep $((attempt * 3)); \
    done \
    && cd /src/prometheus/web/ui \
    && ( \
        npm run build & \
        build_pid=$!; \
        while kill -0 "$build_pid" 2>/dev/null; do \
            echo "prometheus-ui-build heartbeat"; \
            sleep 30; \
        done; \
        wait "$build_pid" \
    )

WORKDIR /src/prometheus
RUN scripts/compress_assets.sh

FROM ${GO_BUILDER_IMAGE} AS build

ARG PROMETHEUS_VERSION=3.12.0
ARG PROMETHEUS_COMMIT=9f27dffc1f93ca23287972f632025879f2d1c658
ARG GO_MODULE_PROXY=https://proxy.golang.org|direct

# Keep the source build within the local Docker Engine memory envelope.
ENV GOMEMLIMIT=768MiB \
    GOGC=50 \
    GOPROXY=${GO_MODULE_PROXY}

COPY --from=assets --chown=1000:1000 /src/prometheus /src/prometheus
WORKDIR /src/prometheus
RUN --mount=type=cache,target=/go/pkg,uid=1000,gid=1000 \
    --mount=type=cache,target=/tmp/go-cache,uid=1000,gid=1000 \
    for attempt in 1 2 3 4 5 6 7 8; do \
        if timeout 900 go get golang.org/x/crypto@v0.53.0 golang.org/x/text@v0.39.0 google.golang.org/grpc@v1.82.1; then \
            break; \
        fi; \
        if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
        sleep $((attempt * 3)); \
    done \
    && common_ldflags="-s -w -X github.com/prometheus/common/version.Version=${PROMETHEUS_VERSION} -X github.com/prometheus/common/version.Revision=${PROMETHEUS_COMMIT} -X github.com/prometheus/common/version.Branch=HEAD -X github.com/prometheus/common/version.BuildUser=chaimir -X github.com/prometheus/common/version.BuildDate=19700101-00:00:00" \
    && ( \
        timeout 900 env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOMAXPROCS=1 go build -p=1 -tags netgo,builtinassets -trimpath -ldflags="${common_ldflags}" -o /out/prometheus ./cmd/prometheus & \
        build_pid=$!; \
        while kill -0 "$build_pid" 2>/dev/null; do echo "prometheus-go-build heartbeat"; sleep 30; done; \
        wait "$build_pid" \
    ) \
    && ( \
        timeout 900 env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOMAXPROCS=1 go build -p=1 -tags netgo,builtinassets -trimpath -ldflags="${common_ldflags}" -o /out/promtool ./cmd/promtool & \
        build_pid=$!; \
        while kill -0 "$build_pid" 2>/dev/null; do echo "prometheus-promtool-build heartbeat"; sleep 30; done; \
        wait "$build_pid" \
    )

FROM cgr.dev/chainguard/wolfi-base@sha256:02dab76bd852a70556b5b2002195c8a5fdab77d323c433bf6642aab080489795

LABEL org.opencontainers.image.title="observability/prometheus"
LABEL org.opencontainers.image.description="Chaimir Prometheus source build on Wolfi"

RUN apk add --no-cache ca-certificates=20260413-r0 tzdata=2026c-r0 \
    && mkdir -p /etc/prometheus /prometheus \
    && printf '%s\n' \
      'global:' \
      '  scrape_interval: 15s' \
      'scrape_configs:' \
      '  - job_name: prometheus' \
      '    static_configs:' \
      '      - targets: ["127.0.0.1:9090"]' \
      > /etc/prometheus/prometheus.yml \
    && chown -R 65532:65532 /etc/prometheus /prometheus

COPY --from=build /out/prometheus /bin/prometheus
COPY --from=build /out/promtool /bin/promtool

USER 65532:65532
WORKDIR /prometheus
EXPOSE 9090
VOLUME ["/prometheus"]
ENTRYPOINT ["/bin/prometheus"]
CMD ["--config.file=/etc/prometheus/prometheus.yml", "--storage.tsdb.path=/prometheus"]
