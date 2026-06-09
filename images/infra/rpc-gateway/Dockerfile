# 本文件定义 RPC 网关镜像,复用 Envoy 作为生产级反向代理。
FROM envoyproxy/envoy:v1.32.3

LABEL org.opencontainers.image.title="infra/rpc-gateway"
LABEL org.opencontainers.image.description="Chaimir RPC gateway proxy image"

USER root

RUN mkdir -p /workspace /etc/chaimir \
    && chown -R envoy:envoy /workspace /etc/chaimir
COPY envoy.yaml /etc/chaimir/envoy.yaml

USER envoy
WORKDIR /workspace
EXPOSE 8080
CMD ["envoy", "-c", "/etc/chaimir/envoy.yaml"]
