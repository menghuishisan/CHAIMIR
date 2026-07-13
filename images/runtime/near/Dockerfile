# 本文件定义 NEAR 本地网运行时镜像。
FROM nearprotocol/nearcore:2.2.1@sha256:66c87335fb18b7311e5f619c45b544743336d993bdc4c077430d6541743ae8a8

LABEL org.opencontainers.image.title="runtime/near"
LABEL org.opencontainers.image.description="Chaimir NEAR localnet runtime"

USER root

# 创建非 root 用户和 NEAR 数据目录;APT 缓存保留成功下载项以抵御临时镜像站故障。
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    rm -f /etc/apt/apt.conf.d/docker-clean \
    && for attempt in 1 2 3 4 5 6 7 8; do \
        apt-get -o Acquire::Retries=10 update \
        && DEBIAN_FRONTEND=noninteractive apt-get -o Acquire::Retries=10 --fix-missing -y upgrade \
        && DEBIAN_FRONTEND=noninteractive apt-get -o Acquire::Retries=10 -y upgrade \
        && break; \
        if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
        sleep $((attempt * 3)); \
    done \
    && groupadd --gid 1000 chaimir \
    && useradd --uid 1000 --gid 1000 --home-dir /workspace --create-home --shell /usr/sbin/nologin chaimir \
    && mkdir -p /runtime-state/near \
    && chown -R 1000:1000 /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-near-entrypoint
RUN chmod +x /usr/local/bin/chaimir-near-entrypoint

USER 1000:1000
WORKDIR /workspace
EXPOSE 3030
ENTRYPOINT ["/usr/local/bin/chaimir-near-entrypoint"]
