# ─── Build Stage ──────────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

# 安装 CGO 依赖（sqlite 等本地库不需要，但 mysql/milvus 纯 Go 驱动无需 gcc）
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# 先复制 go.mod / go.sum，利用 Docker 层缓存
COPY go.mod go.sum ./
RUN go mod download

# 复制源码并构建
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o compete-ai .

# ─── Runtime Stage ─────────────────────────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata curl

WORKDIR /app

# 从构建阶段拷贝二进制
COPY --from=builder /build/compete-ai /app/compete-ai

# 默认暴露 API 端口
EXPOSE 8080

# 默认启动 API；docker-compose worker 服务会通过 command 覆盖为 worker 子命令
ENTRYPOINT ["/app/compete-ai"]
CMD ["server"]
