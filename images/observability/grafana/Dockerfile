# Grafana 13.2.0 source rebuild with a minimal, verified monitoring plugin set.
# The core server and Prometheus datasource are rebuilt from pinned upstream source;
# the final image is assembled on the pinned official distroless runtime without
# carrying the upstream catalog binaries that fail the platform vulnerability gate.
FROM golang:1.26.6-alpine@sha256:3889b425f035be855a72fb4755265311293b6d414521f0a519d819df32222d83 AS builder

ARG GRAFANA_SOURCE_URL=https://codeload.github.com/grafana/grafana/tar.gz/f681b1359f6a0b8ecb9f2c49a88ac72b75bde73b
ARG GRAFANA_SOURCE_SHA256=8ea494789b17d13d7e6032ff158af581d512b9ba3c7e99fae33ba5c19e78a9bd
ARG PROMETHEUS_SOURCE_URL=https://codeload.github.com/grafana/grafana-prometheus-datasource/tar.gz/2e10053dd7d940f9eb91701df5122c8e348a0d18
ARG PROMETHEUS_SOURCE_SHA256=8e66640a7c45b6762d1ac14aa7a94b87fa21c771b4d0d739a3a6716d6a019954
ARG GO_MODULE_PROXY=https://goproxy.cn,direct

RUN apk add --no-cache bash make tar

WORKDIR /src/grafana
ADD ${GRAFANA_SOURCE_URL} /tmp/grafana-source.tar.gz
RUN echo "${GRAFANA_SOURCE_SHA256}  /tmp/grafana-source.tar.gz" | sha256sum -c - \
    && tar -xzf /tmp/grafana-source.tar.gz --strip-components=1 -C /src/grafana \
    && go env -w GOPROXY="${GO_MODULE_PROXY}" \
    && go mod download \
    && make build-go GO_BUILD_TAGS=oss WIRE_TAGS=oss

WORKDIR /src/prometheus
ADD ${PROMETHEUS_SOURCE_URL} /tmp/prometheus-source.tar.gz
RUN echo "${PROMETHEUS_SOURCE_SHA256}  /tmp/prometheus-source.tar.gz" | sha256sum -c - \
    && tar -xzf /tmp/prometheus-source.tar.gz --strip-components=1 -C /src/prometheus \
    && go env -w GOPROXY="${GO_MODULE_PROXY}" \
    && go mod download \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=false -trimpath -ldflags="-s -w" -o /out/grafana-prometheus-datasource ./pkg

FROM grafana/grafana:13.2.0-distroless@sha256:08e6ab6d67a5e21e6540724d3673a572840a43b1ba02d03922d0a21a7dbb38f9 AS upstream
FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b AS sanitize

COPY --from=upstream /usr/share/grafana /usr/share/grafana
COPY --from=upstream /etc/grafana /etc/grafana
COPY --from=upstream /etc/passwd /etc/passwd
COPY --from=upstream /etc/group /etc/group
COPY --from=upstream /var/lib/grafana /var/lib/grafana
COPY --from=upstream /var/log/grafana /var/log/grafana
RUN rm -rf /usr/share/grafana/data/plugins-bundled \
    && mkdir -p /usr/share/grafana/data/plugins-bundled/prometheus
COPY --from=upstream /usr/share/grafana/data/plugins-bundled/prometheus/plugin.json /usr/share/grafana/data/plugins-bundled/prometheus/plugin.json
COPY --from=upstream /usr/share/grafana/data/plugins-bundled/prometheus/img /usr/share/grafana/data/plugins-bundled/prometheus/img
COPY --from=builder /out/grafana-prometheus-datasource /usr/share/grafana/data/plugins-bundled/prometheus/gpx_grafana-prometheus-datasource_linux_amd64

# COPY materializes files as root; restore only the writable runtime paths for UID 472.
RUN chown -R 472:0 /var/lib/grafana /var/log/grafana

FROM gcr.io/distroless/static-debian13@sha256:f2ea2709ac8db56323cbd7d014277f32cb572d9ea124b0076f7aafe5980678fe

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
    GF_PATHS_PROVISIONING=/etc/grafana/provisioning

WORKDIR /usr/share/grafana
EXPOSE 3000
USER 472
ENTRYPOINT ["/usr/share/grafana/bin/grafana", "server", "--homepath=/usr/share/grafana", "--config=/etc/grafana/grafana.ini", "--packaging=docker"]
CMD ["cfg:default.log.mode=console"]
