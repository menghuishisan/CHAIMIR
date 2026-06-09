# 本文件定义 Chainlink 预言机节点镜像,复用 Chainlink 官方节点镜像。
FROM smartcontract/chainlink:2.20.0

LABEL org.opencontainers.image.title="infra/chainlink-node"
LABEL org.opencontainers.image.description="Chaimir Chainlink oracle node image"

USER root

# 创建平台运行目录,节点密钥和数据库凭据只允许运行期注入。
RUN mkdir -p /chainlink /workspace \
    && chown -R chainlink:chainlink /chainlink /workspace

USER chainlink
WORKDIR /workspace
EXPOSE 6688
ENTRYPOINT ["chainlink"]
CMD ["node", "start", "--config", "/chainlink/config.toml", "--secrets", "/chainlink/secrets.toml"]
