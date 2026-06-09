# 本文件定义静态安全扫描判题镜像。
FROM trailofbits/eth-security-toolbox:2024.12.0

LABEL org.opencontainers.image.title="judger/static-scan"
LABEL org.opencontainers.image.description="Chaimir static scan judger image"

USER root
RUN useradd --system --create-home --home-dir /judge chaimir \
    && chown -R chaimir:chaimir /judge

COPY run_static_scan.sh /usr/local/bin/run-static-scan
COPY normalize_result.py /usr/local/bin/normalize-result
RUN chmod +x /usr/local/bin/run-static-scan /usr/local/bin/normalize-result

USER chaimir
WORKDIR /judge
ENTRYPOINT ["run-static-scan"]
