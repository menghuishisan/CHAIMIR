# 本文件定义 FISCO BCOS 运行时镜像,基于已验证 digest 更新严格配置入口。
FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40 AS fisco-config-assets

ARG FISCO_BUILD_CHAIN_URL=https://github.com/FISCO-BCOS/FISCO-BCOS/releases/download/v3.11.0/build_chain.sh
ARG FISCO_BUILD_CHAIN_SHA256=288efe6154b3bf40e6849448b496bee92517c917f3dd8a82a47dfcbabbd30364
ARG FISCO_TASSL_URL=https://github.com/FISCO-BCOS/TASSL/releases/download/V_1.4/tassl-1.1.1b-linux-x86_64.tar.gz
ARG FISCO_TASSL_SHA256=97700126df491b4ae15afae1bad99979390ded5d7d1655632e00b40003a9a9e8

RUN apk add --no-cache curl \
    && mkdir -p /out \
    && curl -fL --retry 8 --retry-all-errors --retry-delay 3 --connect-timeout 30 --max-time 600 -o /out/build_chain.sh "${FISCO_BUILD_CHAIN_URL}" \
    && echo "${FISCO_BUILD_CHAIN_SHA256}  /out/build_chain.sh" | sha256sum -c - \
    && curl -fL --retry 8 --retry-all-errors --retry-delay 3 --connect-timeout 30 --max-time 600 -o /tmp/tassl.tar.gz "${FISCO_TASSL_URL}" \
    && echo "${FISCO_TASSL_SHA256}  /tmp/tassl.tar.gz" | sha256sum -c - \
    && tar -xzf /tmp/tassl.tar.gz -C /out \
    && mv /out/tassl-1.1.1b-linux-x86_64 /out/tassl-1.1.1b \
    && chmod 0555 /out/build_chain.sh /out/tassl-1.1.1b \
    && rm -f /tmp/tassl.tar.gz

FROM registry.chaimir.io/runtime/fisco-bcos@sha256:fb4f78010f1088d00f6b921e1f63203ed637e4e0b56f820e29bf2c26d30ccfe3

LABEL org.opencontainers.image.title="runtime/fisco-bcos"
LABEL org.opencontainers.image.description="Chaimir FISCO BCOS teaching runtime"

USER root

RUN apk add --no-cache bash gcompat

COPY entrypoint.sh /usr/local/bin/chaimir-fisco-entrypoint
COPY fisco-config-init.sh /usr/local/bin/chaimir-fisco-config-init
COPY uuidgen.sh /usr/local/bin/uuidgen
COPY --from=fisco-config-assets /out/build_chain.sh /usr/local/lib/chaimir/fisco/build_chain.sh
COPY --from=fisco-config-assets /out/tassl-1.1.1b /usr/local/lib/chaimir/fisco/tassl-1.1.1b
RUN chmod +x /usr/local/bin/chaimir-fisco-entrypoint /usr/local/bin/chaimir-fisco-config-init /usr/local/bin/uuidgen \
    && mkdir -p /workspace /runtime-state/fisco \
    && chown -R 1000:1000 /workspace /runtime-state \
    && chmod 0555 /usr/local/lib/chaimir/fisco/build_chain.sh /usr/local/lib/chaimir/fisco/tassl-1.1.1b

USER 1000:1000
WORKDIR /workspace
EXPOSE 30300 20200
ENTRYPOINT ["/usr/local/bin/chaimir-fisco-entrypoint"]
