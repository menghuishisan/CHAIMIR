# 本文件定义 Solana 本地测试链运行时镜像,复用官方已验证运行层并统一 Chaimir 入口。
FROM solanalabs/solana:v1.18.26@sha256:098806e64d44bccdbedbf07c2edabd1c850b92c8a4a0f81eba9c789034813db6 AS solana-upstream

FROM golang:1.26.6-bookworm@sha256:116d58cbd88c1297624acc6e967a060012422bacf9930927e23fb719189c6f36 AS adapter-build
WORKDIR /src
COPY entrypoint.go tar-wrapper.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/chaimir-solana-entrypoint ./entrypoint.go \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/chaimir-tar-wrapper ./tar-wrapper.go

# Solana 只需要 glibc、libudev 和 libcap;不要把其上游 Debian 11 用户态整体带入运行镜像。
FROM debian:13-slim@sha256:abc9cb88a5587630d7f915f47b23b0668fe250fbfc6457aa4d52b534c1bbf73f AS runtime-libs

FROM cgr.dev/chainguard/wolfi-base@sha256:6677ae802613d558c16d8bbb2ef2ec89e2e33089bbf78c8f3090ac159bf02d4c AS runtime-tools

FROM cgr.dev/chainguard/glibc-dynamic@sha256:d0046044cd28948d3380eb0d98709dc7e63f98161fe7105135e1025650bad17a

COPY --from=solana-upstream /usr/lib/x86_64-linux-gnu/libudev.so.1* /usr/lib/
COPY --from=solana-upstream /usr/lib/x86_64-linux-gnu/libcap.so.2* /usr/lib/
COPY --from=solana-upstream /usr/lib/x86_64-linux-gnu/libzstd.so.1* /usr/lib/
COPY --from=solana-upstream /usr/lib/x86_64-linux-gnu/liblz4.so.1* /usr/lib/
# 保持 Solana 官方构建所用的 libstdc++ ABI,避免新工具链运行 RocksDB 时改变行为。
COPY --from=solana-upstream /usr/lib/x86_64-linux-gnu/libstdc++.so.6* /usr/lib/
# Solana 固定调用 tar jcfhS;适配器移除 BusyBox 不支持但非必要的 sparse 标志。
COPY --from=runtime-tools /bin/busybox /usr/bin/busybox
COPY --from=runtime-tools /bin/busybox /usr/bin/bzip2
COPY --from=adapter-build /out/chaimir-tar-wrapper /usr/bin/tar
COPY --from=runtime-libs /lib/x86_64-linux-gnu/libacl.so.1* /usr/lib/
COPY --from=runtime-libs /lib/x86_64-linux-gnu/libselinux.so.1* /usr/lib/
COPY --from=runtime-libs /lib/x86_64-linux-gnu/libpcre2-8.so.0* /usr/lib/
COPY --from=runtime-libs /lib/x86_64-linux-gnu/libcrypt.so.1* /usr/lib/

COPY --from=solana-upstream /usr/bin/solana* /usr/bin/
COPY --from=solana-upstream /usr/bin/spl-* /usr/bin/

LABEL org.opencontainers.image.title="runtime/solana"
LABEL org.opencontainers.image.description="Chaimir Solana test validator runtime"

COPY --from=adapter-build /out/chaimir-solana-entrypoint /usr/local/bin/chaimir-solana-entrypoint

USER 1000:1000
WORKDIR /workspace
EXPOSE 8899
ENTRYPOINT ["/usr/local/bin/chaimir-solana-entrypoint"]
