# 本文件从固定 External Secrets Operator 源码构建控制器，使用修复后的 Go 运行时和依赖。
FROM golang:1.26.6-bookworm@sha256:116d58cbd88c1297624acc6e967a060012422bacf9930927e23fb719189c6f36 AS builder
ARG SOURCE_URL=https://github.com/external-secrets/external-secrets/archive/refs/tags/v2.10.0.tar.gz
ARG SOURCE_SHA256=a0c8834a9ecc7b98b5107f748a48a40189ff7a16e5b7033418e9b89992a159df
ARG GO_MODULE_PROXY=https://goproxy.cn,direct
WORKDIR /src
RUN curl --fail --location --retry 5 --retry-all-errors --connect-timeout 20 --max-time 300 \
      -o /tmp/external-secrets.tar.gz "${SOURCE_URL}" \
    && echo "${SOURCE_SHA256}  /tmp/external-secrets.tar.gz" | sha256sum -c - \
    && tar -xzf /tmp/external-secrets.tar.gz -C /src --strip-components=1 \
    && rm -f /tmp/external-secrets.tar.gz
RUN go env -w GOPROXY="${GO_MODULE_PROXY}" \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -buildvcs=false \
      -ldflags='-s -w' -o /out/external-secrets-linux-amd64 ./main.go

FROM gcr.io/distroless/static@sha256:f2ea2709ac8db56323cbd7d014277f32cb572d9ea124b0076f7aafe5980678fe
COPY --from=builder /out/external-secrets-linux-amd64 /bin/external-secrets
USER 65534
ENTRYPOINT ["/bin/external-secrets"]
