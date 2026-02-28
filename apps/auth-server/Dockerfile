# ビルドステージ
FROM golang:1.25-bookworm AS builder

WORKDIR /workspace

# go.workと全モジュールのgo.mod/go.sumをコピー（go.workのuse解決に全モジュール必要）
COPY go.work go.work.sum ./
COPY apps/acct-server/go.mod apps/acct-server/go.sum ./apps/acct-server/
COPY apps/admin-tui/go.mod apps/admin-tui/go.sum ./apps/admin-tui/
COPY apps/auth-server/go.mod apps/auth-server/go.sum ./apps/auth-server/
COPY apps/vector-api/go.mod apps/vector-api/go.sum ./apps/vector-api/
COPY apps/vector-gateway/go.mod apps/vector-gateway/go.sum ./apps/vector-gateway/
COPY pkg/go.mod pkg/go.sum ./pkg/

# 依存関係のダウンロード（対象アプリ + pkg）
RUN cd apps/auth-server && go mod download && \
    cd /workspace/pkg && go mod download

# ソースコードのコピー（対象アプリ + pkg のみ）
COPY apps/auth-server/ ./apps/auth-server/
COPY pkg/ ./pkg/

RUN cd apps/auth-server && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/auth-server .

# ランタイムステージ
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    procps \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/auth-server /usr/local/bin/auth-server

EXPOSE 1812/udp
# UDPサービスのためHTTPヘルスチェックなし（pgrepで代替）

ENTRYPOINT ["/usr/local/bin/auth-server"]
