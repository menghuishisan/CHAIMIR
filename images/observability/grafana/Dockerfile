# Grafana 13.1.6 使用固定源码重建,Prometheus 数据源随核心服务提供。
# 最终镜像基于固定的官方 distroless 运行时组装,不引入未签名或未审计的外置插件。
ARG GO_BUILDER_IMAGE=invalid.invalid/base/go-builder:required

FROM ${GO_BUILDER_IMAGE} AS builder

USER 1000:1000
ARG GRAFANA_SOURCE_URL
ARG GRAFANA_SOURCE_SHA256
ARG GO_MODULE_PROXY
ARG TEMPO_SOURCE_URL
ARG TEMPO_SOURCE_SHA256

ARG GRAFANA_SOURCE_URL=https://codeload.github.com/grafana/grafana/tar.gz/25d4f6c5c796b3a1390163406484ade5bd150851
ARG GRAFANA_SOURCE_SHA256=e59bc214c9c424b927c6c5ea97c594802d756d83dad92c5e7feafea4ba043614
ARG GO_MODULE_PROXY=
ARG TEMPO_SOURCE_URL=https://codeload.github.com/grafana/tempo/tar.gz/refs/tags/v2.10.3
ARG TEMPO_SOURCE_SHA256=f4f95d0f9cb035789645503185670be311c29fd3ea3d3443c302936f6b8993f7

WORKDIR /src/grafana
ADD --chmod=0644 ${GRAFANA_SOURCE_URL} /tmp/grafana-source.tar.gz
ADD --chmod=0644 ${TEMPO_SOURCE_URL} /tmp/tempo-source.tar.gz
RUN echo "${GRAFANA_SOURCE_SHA256}  /tmp/grafana-source.tar.gz" | sha256sum -c - \
    && echo "${TEMPO_SOURCE_SHA256}  /tmp/tempo-source.tar.gz" | sha256sum -c - \
    && tar -xzf /tmp/grafana-source.tar.gz --strip-components=1 -C /src/grafana \
    && mkdir -p /tmp/tempo-source \
    && tar -xzf /tmp/tempo-source.tar.gz --strip-components=1 -C /tmp/tempo-source \
    && sed -i 's/^module github.com\/grafana\/tempo$/module github.com\/grafana\/tempo\/v2/' /tmp/tempo-source/go.mod \
    && find /src/grafana /tmp/tempo-source -type f \( -name '*.go' -o -name 'go.mod' -o -name 'go.sum' \) -exec sed -i 's#github.com/grafana/tempo/#github.com/grafana/tempo/v2/#g' {} + \
    && if [ -n "${GO_MODULE_PROXY}" ]; then go env -w GOPROXY="${GO_MODULE_PROXY}"; fi \
    && go mod edit -droprequire=github.com/grafana/tempo \
    && go mod edit -require=github.com/grafana/tempo/v2@v2.10.3 -require=github.com/apache/thrift@v0.24.0 -require=google.golang.org/grpc@v1.83.2 \
    && go mod edit -replace=github.com/grafana/tempo/v2=/tmp/tempo-source \
    && for attempt in 1 2 3; do \
        if timeout 600 go mod download; then \
            break; \
        fi; \
        if [ "${attempt}" -eq 3 ]; then exit 1; fi; \
        sleep $((attempt * 5)); \
    done \
    && ( \
        timeout 3600 make build-go GO_BUILD_TAGS=oss WIRE_TAGS=oss & \
        build_pid=$!; \
        while kill -0 "$build_pid" 2>/dev/null; do \
            echo "grafana-go-build heartbeat"; \
            sleep 30; \
        done; \
        wait "$build_pid" \
    )

FROM grafana/grafana:13.1.6-distroless@sha256:99ec731555ea3be88a491da7cc28a5801b0240309e4e15e1cac4a592b099ec14 AS upstream
FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b AS sanitize

COPY --from=upstream /usr/share/grafana /usr/share/grafana
COPY --from=upstream /etc/grafana /etc/grafana
COPY --from=upstream /etc/passwd /etc/passwd
COPY --from=upstream /etc/group /etc/group
COPY --from=upstream /var/lib/grafana /var/lib/grafana
COPY --from=upstream /var/log/grafana /var/log/grafana
# COPY 会以 root 写入文件;这里只为 UID 472 恢复必须可写的运行目录。
RUN rm -rf /usr/share/grafana/data/plugins-bundled \
    && mkdir -p /usr/share/grafana/data/plugins-bundled \
    && chown -R 472:0 /var/lib/grafana /var/log/grafana

FROM gcr.io/distroless/base-nossl-debian13@sha256:8c563c1fb5e120606f0d85733049775faed6192e2bd2223ef283a5393eec22b9

COPY --from=sanitize /etc/passwd /etc/passwd
COPY --from=sanitize /etc/group /etc/group
COPY --from=sanitize /etc/grafana /etc/grafana
COPY --from=sanitize --chown=472:0 /var/lib/grafana /var/lib/grafana
COPY --from=sanitize --chown=472:0 /var/log/grafana /var/log/grafana
COPY --from=sanitize /usr/share/grafana /usr/share/grafana
COPY --from=builder /src/grafana/bin/linux/amd64/grafana /usr/share/grafana/bin/grafana

ENV PATH="/usr/share/grafana/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin" \
    SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt \
    GF_PATHS_CONFIG=/etc/grafana/grafana.ini \
    GF_PATHS_DATA=/var/lib/grafana \
    GF_PATHS_HOME=/usr/share/grafana \
    GF_PATHS_LOGS=/var/log/grafana \
    GF_PATHS_PLUGINS=/var/lib/grafana/plugins \
    GF_PATHS_PROVISIONING=/etc/grafana/provisioning \
    GF_ANALYTICS_REPORTING_ENABLED=false \
    GF_ANALYTICS_CHECK_FOR_UPDATES=false \
    GF_ANALYTICS_CHECK_FOR_PLUGIN_UPDATES=false \
    GF_PLUGINS_PREINSTALL_DISABLED=true \
    GF_PLUGINS_PREINSTALL_AUTO_UPDATE=false \
    GF_PLUGINS_PUBLIC_KEY_RETRIEVAL_DISABLED=true \
    GF_PLUGINS_PLUGIN_ADMIN_ENABLED=false \
    GF_PLUGINS_PREINSTALL=

WORKDIR /usr/share/grafana
EXPOSE 3000
USER 472
ENTRYPOINT ["/usr/share/grafana/bin/grafana", "server", "--homepath=/usr/share/grafana", "--config=/etc/grafana/grafana.ini", "--packaging=docker"]
CMD ["cfg:default.log.mode=console"]
