# 本文件定义 Besu 企业 EVM 运行时镜像,复用官方 Besu 应用并使用 Wolfi 运行层。
FROM hyperledger/besu:26.6.1@sha256:ac85d23b4250308ce5f6d004fff7fa6779f82932d1ba32fd3d4811531d43d0bc AS upstream

FROM cgr.dev/chainguard/wolfi-base@sha256:02dab76bd852a70556b5b2002195c8a5fdab77d323c433bf6642aab080489795 AS patched-besu

ARG NETTY_VERSION=4.2.15.Final
ARG JACKSON_VERSION=2.21.4

RUN --mount=type=cache,target=/var/cache/apk,sharing=locked \
    packages='ca-certificates=20260413-r0 curl=8.21.0-r1' \
    && for attempt in 1 2 3 4 5 6 7 8; do \
        apk add --cache-dir /var/cache/apk --update-cache ${packages} && break; \
        if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
        sleep $((attempt * 3)); \
    done

COPY --from=upstream /opt/besu /opt/besu

# 官方 26.6.1 尚未发布高危依赖修复版;仅替换同一主线的 Netty/Jackson 安全补丁 jar,保留 Besu 应用本体。
RUN set -eux; \
    cd /opt/besu/lib; \
    for old in netty-*-4.2.10.Final*.jar; do \
        artifact="${old%%-4.2.10.Final*}"; \
        rest="${old#${artifact}-4.2.10.Final}"; \
        rest="${rest%.jar}"; \
        new="${artifact}-${NETTY_VERSION}${rest}.jar"; \
        curl -fL --retry 10 --retry-all-errors --connect-timeout 20 --retry-delay 3 \
            "https://repo.maven.apache.org/maven2/io/netty/${artifact}/${NETTY_VERSION}/${new}" \
            -o "${old}.patched"; \
        mv "${old}.patched" "${old}"; \
    done; \
    curl -fL --retry 10 --retry-all-errors --connect-timeout 20 --retry-delay 3 \
        "https://repo.maven.apache.org/maven2/com/fasterxml/jackson/core/jackson-core/${JACKSON_VERSION}/jackson-core-${JACKSON_VERSION}.jar" \
        -o "jackson-core-2.21.1.jar.patched"; \
    mv "jackson-core-2.21.1.jar.patched" "jackson-core-2.21.1.jar"; \
    curl -fL --retry 10 --retry-all-errors --connect-timeout 20 --retry-delay 3 \
        "https://repo.maven.apache.org/maven2/com/fasterxml/jackson/core/jackson-databind/${JACKSON_VERSION}/jackson-databind-${JACKSON_VERSION}.jar" \
        -o "jackson-databind-2.21.1.jar.patched"; \
    mv "jackson-databind-2.21.1.jar.patched" "jackson-databind-2.21.1.jar"; \
    curl -fL --retry 10 --retry-all-errors --connect-timeout 20 --retry-delay 3 \
        "https://repo.maven.apache.org/maven2/com/fasterxml/jackson/datatype/jackson-datatype-jdk8/${JACKSON_VERSION}/jackson-datatype-jdk8-${JACKSON_VERSION}.jar" \
        -o "jackson-datatype-jdk8-2.21.1.jar.patched"; \
    mv "jackson-datatype-jdk8-2.21.1.jar.patched" "jackson-datatype-jdk8-2.21.1.jar"

FROM cgr.dev/chainguard/wolfi-base@sha256:02dab76bd852a70556b5b2002195c8a5fdab77d323c433bf6642aab080489795

LABEL org.opencontainers.image.title="runtime/evm-besu"
LABEL org.opencontainers.image.description="Chaimir Hyperledger Besu enterprise EVM runtime"

RUN --mount=type=cache,target=/var/cache/apk,sharing=locked \
    packages='bash=5.3-r12 ca-certificates=20260413-r0 openjdk-25-jre=25.0.3-r5 libstdc++=16.1.0-r4' \
    && for attempt in 1 2 3 4 5 6 7 8; do \
        apk add --cache-dir /var/cache/apk --update-cache ${packages} && break; \
        if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
        sleep $((attempt * 3)); \
    done \
    && addgroup -g 1000 besu \
    && adduser -D -u 1000 -G besu -h /workspace -s /sbin/nologin besu \
    && mkdir -p /workspace /runtime-state/besu \
    && chown -R 1000:1000 /workspace /runtime-state

COPY --from=patched-besu /opt/besu /opt/besu

COPY entrypoint.sh /usr/local/bin/chaimir-besu-entrypoint
RUN chmod +x /usr/local/bin/chaimir-besu-entrypoint \
    && ln -s /opt/besu/bin/besu /usr/local/bin/besu \
    && chown -R 1000:1000 /opt/besu

ENV HOME=/workspace
ENV JAVA_HOME=/usr/lib/jvm/java-25-openjdk
ENV PATH="/opt/besu/bin:/usr/lib/jvm/java-25-openjdk/bin:${PATH}"
USER 1000:1000
WORKDIR /workspace
EXPOSE 8545
ENTRYPOINT ["/usr/local/bin/chaimir-besu-entrypoint"]
