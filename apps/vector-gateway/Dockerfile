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
RUN cd apps/vector-gateway && go mod download && \
    cd /workspace/pkg && go mod download

# ソースコードのコピー（対象アプリ + pkg のみ）
COPY apps/vector-gateway/ ./apps/vector-gateway/
COPY pkg/ ./pkg/

RUN cd apps/vector-gateway && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/vector-gateway .

# ランタイムステージ
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/vector-gateway /usr/local/bin/vector-gateway

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -fsS http://localhost:8080/health || exit 1

ENTRYPOINT ["/usr/local/bin/vector-gateway"]
