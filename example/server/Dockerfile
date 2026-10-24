FROM golang:1.27.3-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/api ./cmd/api

FROM alpine:3.24

# 実行は専用の非 root ユーザーで行う（コンテナが乗っ取られたときの影響を小さくする）
RUN apk add --no-cache ca-certificates \
    && addgroup -S app \
    && adduser -S -G app -H -s /sbin/nologin app
WORKDIR /app
COPY --from=builder /app/bin/api /app/api
USER app

EXPOSE 8080
ENV PORT=:8080
CMD ["/app/api"]
