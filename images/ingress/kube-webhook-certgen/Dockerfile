# 本文件从固定 kube-webhook-certgen 源码构建 admission 证书工具，并锁定修复后的 Go 依赖。
FROM golang:1.26.6-bookworm@sha256:116d58cbd88c1297624acc6e967a060012422bacf9930927e23fb719189c6f36 AS builder
WORKDIR /workspace
COPY . .
RUN go mod edit -require=golang.org/x/net@v0.56.0 -require=golang.org/x/text@v0.39.0 \
    && go mod tidy \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -buildvcs=false -ldflags='-s -w' -o /out/kube-webhook-certgen main.go

FROM gcr.io/distroless/static-debian12:nonroot@sha256:b7bb25d9f7c31d2bdd1982feb4dafcaf137703c7075dbe2febb41c24212b946f
WORKDIR /
COPY --from=builder /out/kube-webhook-certgen /kube-webhook-certgen
USER 65532:65532
ENTRYPOINT ["/kube-webhook-certgen"]
