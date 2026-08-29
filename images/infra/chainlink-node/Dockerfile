# 本文件从固定 Chainlink 源码重建主节点二进制,并复用官方运行期依赖层。
FROM golang:1.26.6-bookworm@sha256:116d58cbd88c1297624acc6e967a060012422bacf9930927e23fb719189c6f36 AS builder
ARG GO_MODULE_PROXY=
ARG CHAINLINK_COMMIT=d09b59c7d76822bd0238a9bb07054cc6f973d724
ARG CHAINLINK_SOURCE_URL=https://github.com/smartcontractkit/chainlink/archive/d09b59c7d76822bd0238a9bb07054cc6f973d724.tar.gz
ARG CHAINLINK_SOURCE_SHA256=183f078af9fd61df59d2a6ca6f06e24bae05c18a397f4c90b36bc07192e8649a
WORKDIR /src
RUN for attempt in 1 2 3 4 5 6 7 8; do \
      curl --fail --location --retry 5 --retry-all-errors --connect-timeout 20 --max-time 300 \
        --user-agent 'chaimir-image-builder/1' \
        -o /tmp/chainlink-source.tar.gz "${CHAINLINK_SOURCE_URL}" && break; \
      if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
      sleep $((attempt * 3)); \
    done
RUN echo "${CHAINLINK_SOURCE_SHA256}  /tmp/chainlink-source.tar.gz" | sha256sum -c - \
    && mkdir -p /src/chainlink \
    && tar -xzf /tmp/chainlink-source.tar.gz -C /src/chainlink --strip-components=1 \
    && rm -f /tmp/chainlink-source.tar.gz
WORKDIR /src/chainlink
RUN if [ -n "${GO_MODULE_PROXY}" ]; then go env -w GOPROXY="${GO_MODULE_PROXY}"; fi \
    && go mod edit -require=golang.org/x/mod@v0.40.0 \
    && go mod download \
    && go mod tidy \
    && test "$(go list -m -f '{{.Version}}' golang.org/x/mod)" = "v0.40.0" \
    && CGO_ENABLED=1 go build -trimpath \
        -ldflags="-s -w -X github.com/smartcontractkit/chainlink/v2/core/static.Version=2.61.0-chaimir -X github.com/smartcontractkit/chainlink/v2/core/static.VersionTag=2.61.0-chaimir -X github.com/smartcontractkit/chainlink/v2/core/static.Sha=${CHAINLINK_COMMIT}" \
        -o /out/chainlink .

FROM smartcontract/chainlink:2.61.0@sha256:25e67bd3488958e6308772b56d10f117c667a47ecd1df3a68034c56ca77e811e
USER root
COPY --chmod=0755 --from=builder /out/chainlink /usr/local/bin/chainlink
RUN rm -f /usr/local/bin/chainlink-* /usr/local/bin/aptos /usr/local/bin/consensus /usr/local/bin/cron /usr/local/bin/evm /usr/local/bin/http_action /usr/local/bin/http_trigger /usr/local/bin/log-event-trigger /usr/local/bin/readcontract /usr/local/bin/solana /usr/local/bin/confidential-http /usr/local/bin/confidential-workflows /usr/local/bin/stellar \
    && (getent group 1000 >/dev/null || groupadd --gid 1000 chaimir) \
    && (id -u 1000 >/dev/null 2>&1 || useradd --uid 1000 --gid 1000 --home-dir /workspace --create-home --shell /usr/sbin/nologin chaimir) \
    && mkdir -p /chainlink /workspace \
    && chown -R 1000:1000 /chainlink /workspace
USER 1000:1000
WORKDIR /workspace
EXPOSE 6688
ENTRYPOINT ["chainlink"]
CMD ["node", "start", "--config", "/chainlink/config.toml", "--secrets", "/chainlink/secrets.toml"]
