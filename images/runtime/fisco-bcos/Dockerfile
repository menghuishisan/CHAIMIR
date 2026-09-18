# 本文件定义 FISCO BCOS 运行时镜像,直接提取官方节点二进制并加入平台配置入口。
FROM fiscoorg/fiscobcos:v3.12.1@sha256:93db5277a5c896fde402f2165d133cc3d346effef4165ec417ef098eb2ede74b AS fisco-upstream

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

FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40

LABEL org.opencontainers.image.title="runtime/fisco-bcos"
LABEL org.opencontainers.image.description="Chaimir FISCO BCOS teaching runtime"

USER root

RUN apk add --no-cache bash gcompat libcrypto3=3.5.8-r0 libssl3=3.5.8-r0

COPY entrypoint.sh /usr/local/bin/chaimir-fisco-entrypoint
COPY fisco-config-init.sh /usr/local/bin/chaimir-fisco-config-init
COPY uuidgen.sh /usr/local/bin/uuidgen
COPY --from=fisco-upstream /usr/local/bin/fisco-bcos /usr/local/bin/fisco-bcos
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
