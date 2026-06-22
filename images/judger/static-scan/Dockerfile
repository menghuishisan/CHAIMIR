# 本文件定义静态安全扫描判题镜像。
FROM chaimir/base/judge-min:dev AS judge-base

FROM trailofbits/eth-security-toolbox:nightly-20260608

LABEL org.opencontainers.image.title="judger/static-scan"
LABEL org.opencontainers.image.description="Chaimir static scan judger image"

USER root
RUN apt-get -o Acquire::Retries=3 update \
    && apt-get -o Acquire::Retries=3 -y upgrade \
    && rm -rf /var/lib/apt/lists/* \
    && /home/ethsec/.vyper/bin/python -m pip install --no-cache-dir --upgrade "setuptools>=78.1.1" \
    && rm -rf /usr/local/lib/node_modules/npm /usr/local/lib/node_modules/yarn /usr/local/bin/npm /usr/local/bin/npx /usr/local/bin/yarn /usr/local/bin/corepack \
    && if ! getent group 1000 >/dev/null; then groupadd --gid 1000 chaimir; fi \
    && if ! id -u 1000 >/dev/null 2>&1; then useradd --uid 1000 --gid 1000 --home-dir /judge --create-home --shell /usr/sbin/nologin chaimir; fi \
    && mkdir -p /judge \
    && chown -R 1000:1000 /judge

COPY run_static_scan.sh /usr/local/bin/run-static-scan
COPY --from=judge-base /usr/local/bin/normalize-result /usr/local/bin/normalize-result
RUN chmod +x /usr/local/bin/run-static-scan /usr/local/bin/normalize-result

USER 1000:1000
WORKDIR /judge
ENTRYPOINT ["run-static-scan"]
