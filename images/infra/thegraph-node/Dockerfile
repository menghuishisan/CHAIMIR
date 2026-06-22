# 本文件定义 The Graph 索引节点镜像,复用官方 graph-node 镜像。
FROM graphprotocol/graph-node:v0.35.1

LABEL org.opencontainers.image.title="infra/thegraph-node"
LABEL org.opencontainers.image.description="Chaimir The Graph indexing node image"

USER root
ARG DEBIAN_MIRROR=http://deb.debian.org/debian
ARG DEBIAN_SECURITY_MIRROR=http://deb.debian.org/debian-security

# graph-node 在沙箱内按平台固定 UID/GID 运行,避免保留 root group。
RUN mkdir -p /workspace \
    && sed -i "s|http://deb.debian.org/debian-security|${DEBIAN_SECURITY_MIRROR}|g; s|http://deb.debian.org/debian|${DEBIAN_MIRROR}|g" /etc/apt/sources.list \
    && apt-get -o Acquire::Retries=5 update \
    && apt-get -o Acquire::Retries=5 -y upgrade \
    && rm -rf /var/lib/apt/lists/* \
    && rm -f /usr/local/bin/envsubst \
    && chown -R 1000:1000 /workspace

USER 1000:1000
WORKDIR /workspace
EXPOSE 8000 8020 8030 8040
CMD ["graph-node"]
