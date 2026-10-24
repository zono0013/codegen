FROM golang:1.27.3-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/api ./cmd/api

FROM alpine:3.24

# 実行は専用の非 root ユーザーで行う（コンテナが乗っ取られたときの影響を小さくする）。
# USER は数値で指定する（Kubernetes の runAsNonRoot は名前のユーザーを root でないと確かめられないため）
RUN apk add --no-cache ca-certificates \
    && addgroup -S -g 10001 app \
    && adduser -S -u 10001 -G app -H -s /sbin/nologin app
WORKDIR /app
COPY --from=builder /app/bin/api /app/api
USER 10001:10001

EXPOSE 8080
ENV PORT=:8080
CMD ["/app/api"]
