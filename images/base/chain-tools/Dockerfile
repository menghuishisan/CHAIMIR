# 本文件定义链工具公共基座,供需要通用 CLI 工具的 runtime/infra 镜像继承。
ARG GO_BUILDER_IMAGE=invalid.invalid/base/go-builder:required
FROM ${GO_BUILDER_IMAGE} AS adapter-build
WORKDIR /src/chain-adapter
COPY images/base/chain-tools/go.mod images/base/chain-tools/go.sum ./
RUN go mod download
COPY images/base/chain-tools/chaimir-chain.go ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/chaimir-chain ./chaimir-chain.go

FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40

LABEL org.opencontainers.image.title="base/chain-tools"
LABEL org.opencontainers.image.description="Chaimir blockchain toolchain base image"

# 安装通用链实验工具,避免每个运行时镜像重复安装同一批基础命令。
RUN apk upgrade --no-cache \
    && apk add --no-cache ca-certificates curl=8.20.0-r0 jq bash git openssl tar findutils coreutils \
    && addgroup -g 1000 -S chaimir \
    && adduser -u 1000 -G chaimir -h /workspace -s /sbin/nologin -S chaimir \
    && chown -R 1000:1000 /workspace

COPY --from=adapter-build /out/chaimir-chain /usr/local/bin/chaimir-chain
RUN chmod 0755 /usr/local/bin/chaimir-chain

WORKDIR /workspace
USER 1000:1000

# 该镜像只作为构建和网络诊断输入,不提供沙箱工作区或学生入口。
CMD ["/usr/local/bin/chaimir-chain", "--version"]
