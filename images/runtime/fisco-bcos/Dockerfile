# 本文件定义 FISCO BCOS 运行时镜像,复用官方节点镜像能力。
FROM fiscoorg/fiscobcos:v3.12.1@sha256:93db5277a5c896fde402f2165d133cc3d346effef4165ec417ef098eb2ede74b AS fisco-upstream

FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40

LABEL org.opencontainers.image.title="runtime/fisco-bcos"
LABEL org.opencontainers.image.description="Chaimir FISCO BCOS teaching runtime"

USER root

# 建立非 root 工作区和链状态目录,组网细节由 M2 runtime manifest 编排。
RUN apk upgrade --no-cache \
	&& mkdir -p /workspace /runtime-state/fisco \
    && chown -R 1000:1000 /workspace /runtime-state

COPY --from=fisco-upstream /usr/local/bin/fisco-bcos /usr/local/bin/fisco-bcos
COPY entrypoint.sh /usr/local/bin/chaimir-fisco-entrypoint
RUN chmod +x /usr/local/bin/chaimir-fisco-entrypoint

USER 1000:1000
WORKDIR /workspace
EXPOSE 20200 8545
ENTRYPOINT ["/usr/local/bin/chaimir-fisco-entrypoint"]
