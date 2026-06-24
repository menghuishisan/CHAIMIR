# 本文件定义 Mythril 符号执行安全分析工具镜像。
FROM mythril/myth:0.24.8

LABEL org.opencontainers.image.title="tool/mythril"
LABEL org.opencontainers.image.description="Chaimir Mythril symbolic execution tool image"

USER root
RUN apt-get -o Acquire::Retries=3 update \
    && apt-get -o Acquire::Retries=3 -y upgrade \
    && python -m pip install --no-cache-dir --upgrade \
        "urllib3>=2.6.0" \
        "setuptools>=78.1.1" \
        "cryptography>=46.0.0" \
        "requests>=2.32.5" \
    && rm -rf /var/lib/apt/lists/* /root/.cache/pip \
    && if ! getent group 1000 >/dev/null; then groupadd --gid 1000 chaimir; fi \
    && if ! id -u 1000 >/dev/null 2>&1; then useradd --uid 1000 --gid 1000 --home-dir /workspace --create-home --shell /usr/sbin/nologin chaimir; fi \
    && mkdir -p /workspace \
    && chown -R 1000:1000 /workspace

USER 1000:1000
WORKDIR /workspace
ENTRYPOINT ["myth"]
