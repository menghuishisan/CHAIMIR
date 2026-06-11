# 本文件定义静态安全扫描判题镜像。
FROM trailofbits/eth-security-toolbox:nightly-20260608

LABEL org.opencontainers.image.title="judger/static-scan"
LABEL org.opencontainers.image.description="Chaimir static scan judger image"

USER root
RUN groupadd --gid 1000 chaimir \
    && useradd --uid 1000 --gid 1000 --home-dir /judge --create-home --shell /usr/sbin/nologin chaimir \
    && chown -R 1000:1000 /judge

COPY run_static_scan.sh /usr/local/bin/run-static-scan
COPY normalize_result.py /usr/local/bin/normalize-result
RUN chmod +x /usr/local/bin/run-static-scan /usr/local/bin/normalize-result

USER 1000:1000
WORKDIR /judge
ENTRYPOINT ["run-static-scan"]
