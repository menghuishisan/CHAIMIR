# 本文件定义 Prometheus 安全运行镜像,基于 Wolfi 包保留 prometheus/promtool 能力。
FROM cgr.dev/chainguard/wolfi-base:latest

LABEL org.opencontainers.image.title="observability/prometheus"
LABEL org.opencontainers.image.description="Chaimir Prometheus on Wolfi"

RUN apk add --no-cache prometheus-3.12 \
    && addgroup -g 65532 nonroot || true \
    && adduser -D -H -u 65532 -G nonroot nonroot || true \
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

USER 65532:65532
WORKDIR /prometheus
EXPOSE 9090
VOLUME ["/prometheus"]
ENTRYPOINT ["/bin/prometheus"]
CMD ["--config.file=/etc/prometheus/prometheus.yml", "--storage.tsdb.path=/prometheus"]
