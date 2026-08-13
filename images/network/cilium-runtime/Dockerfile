# 本文件从固定上游组件重建 Cilium runtime，并移除 Ubuntu 基座中未被 Cilium 使用的高危 Pebble 可执行文件。
FROM quay.io/cilium/cilium-llvm:19.1.7-1782898005-bed4da5@sha256:270bf0e70853c388b72c2247e4ade91d35d74898e2ec12edd5b72ace9ee34333 AS llvm-dist
FROM quay.io/cilium/cilium-bpftool:7.7.0-1782898005-bed4da5@sha256:dcfaf42007b0ba55cb73010d078e57139450c89fd697b21817c62dbfeb65095c AS bpftool-dist
FROM quay.io/cilium/iptables:1.8.8-1-1782898005-bed4da5@sha256:09d2fb75a2e6750135f1c550b477f24798cd210fe74f767c5fdfdad1e71241d4 AS iptables-dist

FROM docker.io/library/golang:1.26.5@sha256:3aff6657219a4d9c14e27fb1d8976c49c29fddb70ba835014f477e1c70636647 AS go-builder

ARG TARGETARCH
ADD --checksum=sha256:c4637684ac8efbcc0bc55faf64e3920a7764a9ae8f4580945084c1a5b8b2d051 https://github.com/google/gops/archive/refs/tags/v0.3.29.tar.gz /tmp/gops.tar.gz
ADD --checksum=sha256:34bd82d47e981940751619c9cc44c095bb90bfcaf8d71865cbb822c37690a764 https://github.com/containernetworking/plugins/archive/refs/tags/v1.9.1.tar.gz /tmp/cni-plugins.tar.gz
ADD --checksum=sha256:15dc9570470ee8bdd418259a93d40a10225db30cf094983f5529fe89168ed993 https://github.com/kubernetes-sigs/iptables-wrappers/archive/refs/tags/v3.tar.gz /tmp/iptables-wrappers.tar.gz

RUN mkdir -p /src/gops /src/cni /src/iptables-wrappers /out/usr/bin /out/cni /out/usr/sbin \
    && tar -xzf /tmp/gops.tar.gz -C /src/gops --strip-components=1 \
    && tar -xzf /tmp/cni-plugins.tar.gz -C /src/cni --strip-components=1 \
    && tar -xzf /tmp/iptables-wrappers.tar.gz -C /src/iptables-wrappers --strip-components=1 \
    && cd /src/gops \
    && GOARCH="${TARGETARCH}" CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/usr/bin/gops github.com/google/gops \
    && cd /src/cni \
    && GOARCH="${TARGETARCH}" CGO_ENABLED=0 go build -trimpath -ldflags='-s -w -X github.com/containernetworking/plugins/pkg/utils/buildversion.BuildVersion=v1.9.1' -o /out/cni/loopback ./plugins/main/loopback \
    && cd /src/iptables-wrappers \
    && GOARCH="${TARGETARCH}" CGO_ENABLED=0 go build -trimpath -ldflags='-s -w -extldflags="-static" -buildid=' -o /out/usr/sbin/iptables-wrapper github.com/kubernetes-sigs/iptables-wrappers \
    && rm -rf /tmp/*.tar.gz /src

FROM docker.io/library/ubuntu:26.04@sha256:3131b4cc82a783df6c9df078f86e01819a13594b865c2cad47bd1bca2b7063bb AS rootfs

# 从固定基座获得安全更新和 Cilium 所需运行依赖。Pebble 不属于 Cilium、也不被任何 Cilium 二进制调用，删除以消除基座遗留高危项。
RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get upgrade -y \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        bash-completion ca-certificates iproute2 ipset iptables jq kmod libatomic1 \
    && apt-get purge --auto-remove -y \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/* \
    && rm -f /usr/bin/pebble \
    && test ! -e /usr/bin/pebble

COPY --from=iptables-dist /iptables /iptables
COPY --from=go-builder /out /
RUN dpkg -i /iptables/*.deb \
    && rm -rf /iptables \
    && /usr/sbin/iptables-wrapper install

COPY --from=llvm-dist /usr/local/bin/clang /usr/local/bin/llc /usr/local/bin/
COPY --from=bpftool-dist /usr/local/bin/bpftool /usr/local/bin/bpftool

FROM scratch
LABEL org.opencontainers.image.title="network/cilium-runtime"
LABEL org.opencontainers.image.description="Cilium v1.20.0 runtime rebuilt without unused vulnerable Pebble"
COPY --from=rootfs / /
