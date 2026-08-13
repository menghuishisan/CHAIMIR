# 本文件定义 RPC 网关镜像,复用 Envoy 作为生产级反向代理。
FROM envoyproxy/envoy:v1.32.3@sha256:127e33c48c60be9a148cf9256d52e734ceda28aa344145023933116c7e524beb

LABEL org.opencontainers.image.title="infra/rpc-gateway"
LABEL org.opencontainers.image.description="Chaimir RPC gateway proxy image"

USER root

RUN mkdir -p /workspace /etc/chaimir \
    && sed -i 's|http://archive.ubuntu.com|https://archive.ubuntu.com|g; s|http://security.ubuntu.com|https://security.ubuntu.com|g' /etc/apt/sources.list \
    && apt-get -o Acquire::Retries=5 update \
    && apt-get -o Acquire::Retries=5 -y upgrade \
    && rm -rf /var/lib/apt/lists/* \
    && chown -R 1000:1000 /workspace /etc/chaimir
COPY envoy.yaml /etc/chaimir/envoy.yaml

USER 1000:1000
WORKDIR /workspace
EXPOSE 8080
CMD ["envoy", "-c", "/etc/chaimir/envoy.yaml"]
