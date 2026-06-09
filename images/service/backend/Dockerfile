# 本文件构建 Chaimir 后端服务镜像,最终产物只包含 server 二进制。
FROM golang:1.25-bookworm AS builder

WORKDIR /src

# 先复制依赖清单以复用构建缓存,再复制后端源码。
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./

# 使用静态二进制降低运行时依赖,生产部署按 digest 拉取并由 Cosign 校验签名。
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.title="service/backend"
LABEL org.opencontainers.image.description="Chaimir backend API and control-plane service"

WORKDIR /app
COPY --from=builder /out/server /app/server

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/server"]
