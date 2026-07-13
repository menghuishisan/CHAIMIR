# 本文件定义 Grafana 的安全运行镜像,基于 Wolfi 包保留看板与 HTTP API 能力。
FROM cgr.dev/chainguard/wolfi-base@sha256:02dab76bd852a70556b5b2002195c8a5fdab77d323c433bf6642aab080489795

LABEL org.opencontainers.image.title="observability/grafana"
LABEL org.opencontainers.image.description="Chaimir Grafana observability image on Wolfi"

RUN apk add --no-cache grafana-13.0=13.0.2-r1 grafana-13.0-oci-entrypoint-compat=13.0.2-r1 \
    && mkdir -p /var/lib/grafana /var/log/grafana /etc/grafana/provisioning \
    && chown -R 65532:65532 /var/lib/grafana /var/log/grafana /etc/grafana

ENV GF_PATHS_HOME=/usr/share/grafana
ENV GF_PATHS_CONFIG=/etc/grafana/grafana.ini
ENV GF_PATHS_DATA=/var/lib/grafana
ENV GF_PATHS_LOGS=/var/log/grafana
ENV GF_PATHS_PLUGINS=/var/lib/grafana/plugins
ENV GF_PATHS_PROVISIONING=/etc/grafana/provisioning
ENV GF_PLUGINS_PREINSTALL=
ENV GF_ANALYTICS_CHECK_FOR_UPDATES=false
ENV GF_ANALYTICS_CHECK_FOR_PLUGIN_UPDATES=false

USER 65532:65532
WORKDIR /usr/share/grafana
EXPOSE 3000
ENTRYPOINT ["/run.sh"]
