# 本文件定义跨链桥中继器镜像,薄封装 Hyperlane 官方 relayer agent。
ARG GO_BUILDER_IMAGE=invalid.invalid/base/go-builder:required

FROM ${GO_BUILDER_IMAGE} AS config-build
WORKDIR /src/bridge-relayer
COPY config.go .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/chaimir-hyperlane-config ./config.go

FROM ghcr.io/hyperlane-xyz/hyperlane-agent@sha256:ed2a0dcc7ebe7accd2bec88f82ce196bb455f4cf9a4d84d7a9192052b1911551

LABEL org.opencontainers.image.title="infra/bridge-relayer"
LABEL org.opencontainers.image.description="Chaimir Hyperlane bridge relayer image"

USER root
RUN apt-get -o Acquire::Retries=5 update \
    && apt-get -o Acquire::Retries=5 upgrade -y --no-install-recommends \
    && groupadd --gid 1000 chaimir \
    && useradd --uid 1000 --gid 1000 --home-dir /workspace --create-home --shell /usr/sbin/nologin chaimir \
    && mkdir -p /workspace /runtime-state /hyperlane-config \
    && chown -R 1000:1000 /workspace /runtime-state /hyperlane-config \
    && rm -rf /var/lib/apt/lists/*

COPY entrypoint.sh /usr/local/bin/chaimir-bridge-relayer-entrypoint
COPY --from=config-build /out/chaimir-hyperlane-config /usr/local/bin/chaimir-hyperlane-config
RUN chmod +x /usr/local/bin/chaimir-bridge-relayer-entrypoint

ENV HOME=/workspace
WORKDIR /workspace
USER 1000:1000
ENTRYPOINT ["/usr/local/bin/chaimir-bridge-relayer-entrypoint"]
