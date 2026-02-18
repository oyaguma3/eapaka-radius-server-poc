# ビルドステージ
FROM golang:1.25-bookworm AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o auth-server .

# ランタイムステージ
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    procps \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/auth-server /usr/local/bin/auth-server

EXPOSE 1812/udp
# UDPサービスのためHTTPヘルスチェックなし（pgrepで代替）

ENTRYPOINT ["/usr/local/bin/auth-server"]
