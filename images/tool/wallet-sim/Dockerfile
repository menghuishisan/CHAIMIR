# 本文件构建钱包模拟器工具镜像。
FROM cgr.dev/chainguard/wolfi-base@sha256:6677ae802613d558c16d8bbb2ef2ec89e2e33089bbf78c8f3090ac159bf02d4c

ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY

LABEL org.opencontainers.image.title="tool/wallet-sim"
LABEL org.opencontainers.image.description="Chaimir wallet simulator tool image"

RUN --mount=type=cache,target=/var/cache/apk,sharing=locked \
    export http_proxy="${HTTP_PROXY}" https_proxy="${HTTPS_PROXY}" no_proxy="${NO_PROXY}" \
    && packages='python-3.13=3.13.14-r3 py3.13-pip=26.1.2-r1' \
    && apk upgrade --no-cache \
    && for attempt in 1 2 3 4 5 6 7 8; do \
        apk add --cache-dir /var/cache/apk --update-cache ${packages} && break; \
        if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
        sleep $((attempt * 3)); \
    done \
    && python3.13 -m venv /opt/wallet-sim \
    && /opt/wallet-sim/bin/pip install --no-cache-dir "setuptools==80.9.0" "eth-account==0.13.7" \
    && addgroup -g 1000 chaimir \
    && adduser -D -u 1000 -G chaimir -h /app -s /sbin/nologin chaimir

RUN rm -rf /opt/wallet-sim/lib/python3.13/site-packages/pip /opt/wallet-sim/lib/python3.13/site-packages/pip-*.dist-info /opt/wallet-sim/lib/python3.13/site-packages/setuptools /opt/wallet-sim/lib/python3.13/site-packages/setuptools-*.dist-info \
    && apk del py3.13-pip py3.13-pip-base py3-pip-wheel

WORKDIR /app
COPY wallet_sim.py /app/wallet_sim.py

USER 1000:1000
ENV PATH=/opt/wallet-sim/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
EXPOSE 8080
CMD ["python3.13", "/app/wallet_sim.py"]
