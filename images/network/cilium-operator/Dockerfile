# 本文件从统一的已修复 Cilium 源码构建 generic operator，不保留云厂商变体或调试分支。
ARG CILIUM_BUILDER_IMAGE=invalid.invalid/network/cilium-builder:required

FROM ${CILIUM_BUILDER_IMAGE} AS builder

ARG TARGETOS
ARG TARGETARCH
WORKDIR /src/cilium
RUN --mount=type=cache,target=/go/pkg \
    --mount=type=cache,target=/tmp/go-cache \
    make GOARCH="${TARGETARCH}" DESTDIR="/out/${TARGETOS}/${TARGETARCH}" \
        build-container-operator-generic install-container-binary-operator-generic licenses-all \
    && mv LICENSE.all "/out/${TARGETOS}/${TARGETARCH}/LICENSE.all" \
    && mkdir -p /home/gops \
    && chmod 0777 /home/gops

FROM scratch
ARG TARGETOS
ARG TARGETARCH
LABEL org.opencontainers.image.title="network/cilium-operator"
LABEL org.opencontainers.image.description="Cilium v1.20.1 generic operator rebuilt from verified official source"

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/bin/gops /usr/bin/gops
COPY --from=builder /out/${TARGETOS}/${TARGETARCH}/usr/bin/cilium-operator-generic /usr/bin/cilium-operator-generic
COPY --from=builder /out/${TARGETOS}/${TARGETARCH}/LICENSE.all /LICENSE.all
COPY --from=builder /home/gops /home/gops

ENV GOPS_CONFIG_DIR=/home/gops
WORKDIR /
CMD ["/usr/bin/cilium-operator-generic"]
