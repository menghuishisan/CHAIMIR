# 本文件定义 Blockscout 区块链浏览器工具镜像。
FROM blockscout/blockscout:6.9.2@sha256:1d3f8fc81c5719e5b1ce37427ebb04082d9d6a57f8c49d0c693a21fbcfecc34d

LABEL org.opencontainers.image.title="tool/blockscout"
LABEL org.opencontainers.image.description="Chaimir Blockscout EVM explorer image"

USER root
RUN apk upgrade --no-cache \
    && mkdir -p /workspace /tmp/blockscout \
    && mkdir -p /tmp/tzdata \
    && chown -R 1000:1000 /workspace /tmp/blockscout /tmp/tzdata \
    && sed -i 's/{elixir,/{tzdata,[{autoupdate,disabled},{data_dir,<<47,116,109,112,47,116,122,100,97,116,97>>}]},{elixir,/' /app/releases/*/sys.config

COPY entrypoint.sh /usr/local/bin/chaimir-blockscout-entrypoint
RUN chmod +x /usr/local/bin/chaimir-blockscout-entrypoint

USER 1000:1000
WORKDIR /app
EXPOSE 4000
