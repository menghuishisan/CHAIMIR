# 本文件定义 Slither 工具镜像,复用成熟安全工具箱。
FROM trailofbits/eth-security-toolbox:nightly-20260608

LABEL org.opencontainers.image.title="tool/slither"
LABEL org.opencontainers.image.description="Chaimir Slither security scanner image"

USER root
RUN apt-get -o Acquire::Retries=3 update \
    && apt-get -o Acquire::Retries=3 -y upgrade \
    && rm -rf /var/lib/apt/lists/* \
    && /home/ethsec/.vyper/bin/python -m pip install --no-cache-dir --upgrade "setuptools>=78.1.1" \
    && rm -rf /usr/local/lib/node_modules/npm /usr/local/lib/node_modules/yarn /usr/local/bin/npm /usr/local/bin/npx /usr/local/bin/yarn /usr/local/bin/corepack \
    && if ! getent group 1000 >/dev/null; then groupadd --gid 1000 chaimir; fi \
    && if ! id -u 1000 >/dev/null 2>&1; then useradd --uid 1000 --gid 1000 --home-dir /workspace --create-home --shell /usr/sbin/nologin chaimir; fi \
    && mkdir -p /workspace \
    && chown -R 1000:1000 /workspace

USER 1000:1000
WORKDIR /workspace
CMD ["slither", "--version"]
