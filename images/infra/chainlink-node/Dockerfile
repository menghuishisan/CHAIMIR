# 本文件定义 Chainlink 预言机节点镜像,复用 Chainlink 官方节点镜像并固定上游 digest。
FROM smartcontract/chainlink@sha256:77a78d4e427c39b3810bf740ba7582240c4c6538d31d0e1069ba0852acb58bc5

LABEL org.opencontainers.image.title="infra/chainlink-node"
LABEL org.opencontainers.image.description="Chaimir Chainlink oracle node image"

USER root

# 创建平台运行目录,节点密钥和数据库凭据只允许运行期注入;未声明或独立发布的多链辅助 binary 不进入 EVM 教学镜像。
RUN apt-get update \
    && apt-get upgrade -y --no-install-recommends \
    && rm -f \
        /usr/local/bin/chainlink-aptos \
        /usr/local/bin/chainlink-cosmos \
        /usr/local/bin/chainlink-evm \
        /usr/local/bin/chainlink-feeds \
        /usr/local/bin/chainlink-mercury \
        /usr/local/bin/chainlink-solana \
        /usr/local/bin/chainlink-starknet \
        /usr/local/bin/chainlink-sui \
        /usr/local/bin/chainlink-ton \
        /usr/local/bin/chainlink-tron \
        /usr/local/bin/confidential-http \
    && rm -rf /var/lib/apt/lists/* \
    && (getent group 1000 >/dev/null || groupadd --gid 1000 chaimir) \
    && (id -u 1000 >/dev/null 2>&1 || useradd --uid 1000 --gid 1000 --home-dir /workspace --create-home --shell /usr/sbin/nologin chaimir) \
    && mkdir -p /chainlink /workspace \
    && chown -R 1000:1000 /chainlink /workspace

USER 1000:1000
WORKDIR /workspace
EXPOSE 6688
ENTRYPOINT ["chainlink"]
CMD ["node", "start", "--config", "/chainlink/config.toml", "--secrets", "/chainlink/secrets.toml"]
