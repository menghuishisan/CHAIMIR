# 本文件定义静态安全扫描判题镜像,使用 Slither 与官方 solc 进行 Solidity 安全判题。
ARG JUDGE_MIN_IMAGE=invalid.invalid/base/judge-min:required
ARG SLITHER_IMAGE=invalid.invalid/tool/slither:required

FROM ${JUDGE_MIN_IMAGE} AS judge-base
FROM ghcr.io/foundry-rs/foundry:v1.7.1@sha256:bdc584033cefab8be282c7f57c263cda42f04700cd1ec88d64bebe2f645aba72 AS foundry
FROM ${SLITHER_IMAGE} AS solc-source

FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40

ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY

ARG PIP_INDEX_URL=https://pypi.tuna.tsinghua.edu.cn/simple

LABEL org.opencontainers.image.title="judger/static-scan"
LABEL org.opencontainers.image.description="Chaimir static scan judger image"

USER root
RUN export http_proxy="${HTTP_PROXY}" https_proxy="${HTTPS_PROXY}" no_proxy="${NO_PROXY}" \
    && sed -i 's#https://dl-cdn.alpinelinux.org/alpine#https://mirrors.aliyun.com/alpine#g' /etc/apk/repositories \
    && for attempt in 1 2 3 4 5; do \
        if apk upgrade --no-cache --timeout 300; then break; fi; \
        if [ "$attempt" -eq 5 ]; then echo "apk upgrade failed after 5 attempts" >&2; exit 1; fi; \
        sleep 3; \
    done \
    && for attempt in 1 2 3 4 5; do \
        if apk add --no-cache --timeout 300 bash ca-certificates gcompat git libstdc++ py3-pip 'py3-setuptools>=80.9.0-r2' python3; then break; fi; \
        if [ "$attempt" -eq 5 ]; then echo "apk package installation failed after 5 attempts" >&2; exit 1; fi; \
        sleep 3; \
    done \
    && true \
    && python3 -m venv /opt/slither \
    && /opt/slither/bin/pip install --index-url "${PIP_INDEX_URL}" --no-cache-dir --timeout 300 --retries 8 --upgrade \
        "pip>=25.3" \
        "setuptools>=80.0.0" \
        "wheel>=0.46.0" \
        "msgpack>=1.2.1" \
        "slither-analyzer==0.11.3" \
        "aiohttp>=3.14.3" \
    && rm -rf /opt/slither/lib/python3.12/site-packages/pip /opt/slither/lib/python3.12/site-packages/pip-*.dist-info \
    && apk del py3-pip py3-setuptools \
    && /opt/slither/bin/slither --version >/dev/null \
    && addgroup -S -g 1000 chaimir \
    && adduser -S -u 1000 -G chaimir -h /judge -s /sbin/nologin chaimir \
    && mkdir -p /judge /judge/.solc-select/artifacts /tmp/slither \
    && chown -R 1000:1000 /judge /tmp/slither

COPY run_static_scan.sh /usr/local/bin/run-static-scan
COPY --from=judge-base /usr/local/bin/normalize-result /usr/local/bin/normalize-result
COPY --from=foundry /usr/local/bin/forge /usr/local/bin/forge
COPY --from=solc-source /usr/local/bin/solc-0.8.20 /usr/local/bin/solc-0.8.20
COPY --from=solc-source /usr/local/bin/solc-0.8.24 /usr/local/bin/solc-0.8.24
COPY --from=solc-source /usr/local/bin/solc-0.8.26 /usr/local/bin/solc-0.8.26
COPY --from=foundry /lib/x86_64-linux-gnu/libgcc_s.so.1 /lib/x86_64-linux-gnu/libgcc_s.so.1
COPY --from=foundry /lib/x86_64-linux-gnu/libc.so.6 /lib/x86_64-linux-gnu/libc.so.6
COPY --from=foundry /lib/x86_64-linux-gnu/libm.so.6 /lib/x86_64-linux-gnu/libm.so.6
COPY --from=foundry /lib/x86_64-linux-gnu/libpthread.so.0 /lib/x86_64-linux-gnu/libpthread.so.0
COPY --from=foundry /lib/x86_64-linux-gnu/libdl.so.2 /lib/x86_64-linux-gnu/libdl.so.2
COPY --from=foundry /lib/x86_64-linux-gnu/libstdc++.so.6 /lib/x86_64-linux-gnu/libstdc++.so.6
COPY --from=foundry /lib64/ld-linux-x86-64.so.2 /lib64/ld-linux-x86-64.so.2
RUN chmod +x /usr/local/bin/run-static-scan /usr/local/bin/normalize-result
RUN echo "0479d44fdf9c501c25337fdc540419f1593b884a87b47f023da4f1c700fda782  /usr/local/bin/solc-0.8.20" | sha256sum -c - \
    && echo "fb03a29a517452b9f12bcf459ef37d0a543765bb3bbc911e70a87d6a37c30d5f  /usr/local/bin/solc-0.8.24" | sha256sum -c - \
    && echo "d5f23436f443edb85d8e76906d12f0a86ce0490e7663a9e608efeb7a93f149ef  /usr/local/bin/solc-0.8.26" | sha256sum -c - \
    && for version in 0.8.20 0.8.24 0.8.26; do chmod 0755 /usr/local/bin/solc-${version}; done

USER 1000:1000
ENV HOME=/judge
ENV TMPDIR=/tmp/slither
ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/opt/slither/bin
WORKDIR /judge
ENTRYPOINT ["run-static-scan"]
