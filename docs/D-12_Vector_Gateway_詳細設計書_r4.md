# D-12 Vector Gateway 詳細設計書 (r4)

## 1. 概要

### 1.1 目的

本ドキュメントは、EAP-AKA RADIUS PoC環境における「Vector Gateway」の詳細設計を定義する。Vector Gatewayは、Auth ServerとVector API（および将来の外部API）の間に位置する中間ノードであり、PLMNベースのルーティングと外部API連携機能を提供する。

**主な責務:**

- PLMNベースのバックエンドルーティング
- 内部Vector APIへのリクエスト転送
- 将来的な外部API連携の拡張ポイント
- X-Trace-IDによるトレーサビリティ確保

### 1.2 背景

現在のアーキテクチャでは、Auth ServerからVector APIに対して認証ベクターを取得する構成となっている。しかし、以下のユースケースに対応するため、外部API連携機能の追加を検討する。

| ユースケース | 説明 |
|-------------|------|
| パートナーMNO連携 | 他事業者のHSS/AuCから認証ベクターを取得 |
| クラウドベース認証 | 外部クラウドサービスとの連携 |
| ハイブリッド運用 | 一部加入者はローカル認証、一部は外部委託 |

### 1.3 設計方針

- **Auth Serverへの影響最小化**: 環境変数 `VECTOR_API_URL` の向き先変更のみで対応
- **既存Vector APIの活用**: 内部認証ベクター計算機能はそのまま利用
- **責務分離**: ルーティング・変換ロジックをVector Gatewayに集約
- **段階的実装**: PoC段階はシンプルに、将来拡張の余地を残す

### 1.4 PoC対象外機能

以下の機能はPoC段階では実装対象外とする。

| 機能 | 理由 | 将来方針 |
|------|------|---------|
| チャレンジ・レスポンス分離型認証（モデル2） | CK/IK未提供によりAT_MAC導出不可 | プロトコル制約のため対応困難 |
| フォールバック機能 | SQN競合問題が重い課題 | 未実装継続の見込み |
| 外部API接続方式の実装 | 接続先仕様未確定 | 仕様確定後に順次実装 |

> **注記:** SORACOM Endorse API等のチャレンジ・レスポンス分離型（RAND/AUTNのみ提供、RES検証はAPI側）は、CK/IKが提供されないためEAP-AKAのAT_MAC計算ができず、端末がClient-Errorと判定する問題があり対応を中止した。

---

## 2. アーキテクチャ設計

### 2.1 全体構成

```
┌─────────────────────────────────────────────────────────────────────────┐
│ Docker Compose Network                                                  │
│                                                                         │
│  ┌─────────────┐      ┌─────────────────┐      ┌─────────────┐         │
│  │ Auth Server │─────►│ Vector Gateway  │─────►│ Vector API  │         │
│  └─────────────┘      │ (新規追加)       │      │ (既存)      │         │
│                       └────────┬────────┘      └─────────────┘         │
│                                │                                        │
└────────────────────────────────┼────────────────────────────────────────┘
                                 │ HTTPS (Outbound) ※将来実装
                                 ▼
                       ┌─────────────────┐
                       │ 外部API         │
                       │ (将来実装)      │
                       └─────────────────┘
```

### 2.2 通信フロー

#### PoC段階: 全リクエストをVector APIへ転送

```
Auth Server → Vector Gateway → Vector API → Valkey
                                  ↓
              ← RAND/AUTN/XRES/CK/IK ←
```

#### 将来: PLMNベースルーティング

```
Auth Server → Vector Gateway ─┬→ Vector API (PLMN未登録 or ID:00)
                              │
                              └→ 外部API (PLMN登録済み、ID:01以降)
```

### 2.3 Vector Gatewayの責務

| 責務 | PoC実装 | 将来実装 |
|------|---------|---------|
| **ルーティング** | 全てVector APIへ転送 | PLMNベースでバックエンド選択 |
| **プロトコル変換** | なし（パススルー） | 外部API形式への変換 |
| **認証情報付与** | なし | 外部API呼び出し時のAPI Key/Token付与 |
| **エラー変換** | なし | 外部APIエラーを内部形式に変換 |
| **トレーサビリティ** | X-Trace-IDの伝搬（内部API） | 下記参照 |

#### トレーサビリティの責務範囲

| バックエンド | 責務範囲 | 説明 |
|-------------|---------|------|
| **内部（Vector API）** | エンドツーエンド保証 | X-Trace-IDを伝搬し、Auth Server〜Vector APIまで追跡可能 |
| **外部API** | Vector Gatewayまで保証 | 外部APIへのヘッダ付与はベストエフォート（外部側の対応は期待しない） |

外部API利用時は、Vector Gatewayのログでトレーサビリティの境界を記録する。

---

## 3. ルーティング設計

### 3.1 ルーティング方式

PLMNベースのルーティングを採用する。

| 項目 | 仕様 |
|------|------|
| キー | PLMN（MCC+MNC結合形式、例: `44010`） |
| 値 | 接続方式ID（2桁数字、例: `00`, `01`） |
| デフォルト動作 | PLMNマップに未登録の場合はVector API（ID:00相当） |

### 3.2 接続方式ID定義

| ID | 名称 | 説明 | PoC実装 |
|----|------|------|---------|
| `00` | Vector API | 内部Vector APIへ転送 | ○ |
| `01`〜`99` | 外部API | 外部API接続方式（将来実装） | × (501エラー) |

### 3.3 PLMN形式

| 項目 | 仕様 |
|------|------|
| 形式 | MCC + MNC 結合（ハイフンなし） |
| 長さ | 5桁（MNC 2桁）または 6桁（MNC 3桁） |
| 例 | `44010`（日本 docomo）、`310260`（米国 T-Mobile） |
| 照合方式 | 固定長照合（環境変数指定形式に完全一致） |

### 3.4 IMSIからのPLMN抽出

```go
// IMSIからPLMNを抽出（固定長照合用）
func extractPLMNs(imsi string) []string {
    if len(imsi) < 6 {
        return nil
    }
    // MNC 2桁と3桁の両方を候補として返す
    return []string{
        imsi[:5],  // MCC(3) + MNC(2)
        imsi[:6],  // MCC(3) + MNC(3)
    }
}

// PLMNマップとの照合
func (r *Router) matchPLMN(imsi string) (string, bool) {
    candidates := extractPLMNs(imsi)
    for _, plmn := range candidates {
        if backendID, ok := r.plmnMap[plmn]; ok {
            return backendID, true
        }
    }
    return "", false
}
```

### 3.5 ルーティングロジック

```go
func (r *Router) SelectBackend(ctx context.Context, imsi string) (Backend, error) {
    // PLMNマップから接続方式IDを検索
    backendID, matched := r.matchPLMN(imsi)
    
    if !matched {
        // 未登録PLMN → デフォルト動作（Vector API）
        slog.Debug("PLMN not matched, using default",
            "event_id", "PLMN_ROUTE_UNMATCH",
            "imsi", maskIMSI(imsi))
        return r.internalBackend, nil
    }
    
    slog.Debug("PLMN matched",
        "event_id", "PLMN_ROUTE_MATCH",
        "plmn", extractPLMN(imsi),
        "backend_id", backendID)
    
    // 接続方式IDからバックエンド取得
    backend, ok := r.backends[backendID]
    if !ok {
        // 未実装の接続方式ID
        slog.Warn("backend not implemented",
            "event_id", "BACKEND_NOT_IMPLEMENTED",
            "backend_id", backendID)
        return nil, &BackendNotImplementedError{ID: backendID}
    }
    
    return backend, nil
}
```

---

## 4. 接続方式設計

### 4.1 バックエンドインターフェース

```go
// Backend は認証ベクター取得の共通インターフェース
type Backend interface {
    // GetVector は認証ベクターを取得する
    GetVector(ctx context.Context, req *VectorRequest) (*VectorResponse, error)
    
    // ID は接続方式IDを返す
    ID() string
    
    // Name は接続方式名を返す（ログ用）
    Name() string
}
```

### 4.2 PoC実装: 内部バックエンド（ID:00）

```go
type InternalBackend struct {
    url        string
    httpClient *http.Client
}

func (b *InternalBackend) ID() string   { return "00" }
func (b *InternalBackend) Name() string { return "vector-api" }

func (b *InternalBackend) GetVector(ctx context.Context, req *VectorRequest) (*VectorResponse, error) {
    traceID := ctx.Value("trace_id").(string)
    
    slog.Info("calling internal vector API",
        "event_id", "BACKEND_INTERNAL_CALL",
        "trace_id", traceID,
        "imsi", maskIMSI(req.IMSI))
    
    // Vector APIへHTTPリクエスト
    resp, err := b.doRequest(ctx, traceID, req)
    if err != nil {
        return nil, fmt.Errorf("internal backend call failed: %w", err)
    }
    
    return resp, nil
}
```

### 4.3 未実装バックエンドのエラー処理

未実装の接続方式IDが指定された場合、501 Not Implementedを返却する。

```go
type BackendNotImplementedError struct {
    ID string
}

func (e *BackendNotImplementedError) Error() string {
    return fmt.Sprintf("backend not implemented: %s", e.ID)
}

// HTTPハンドラでのエラー変換
func (h *VectorHandler) handleError(w http.ResponseWriter, err error) {
    var notImplErr *BackendNotImplementedError
    if errors.As(err, &notImplErr) {
        h.writeError(w, http.StatusNotImplemented, "Backend Not Implemented",
            fmt.Sprintf("Backend ID %s is not implemented", notImplErr.ID))
        return
    }
    // その他のエラー処理...
}
```

### 4.4 接続方式IDと実装の紐付け（PoC）

PoC段階ではハードコーディングで管理する。

```go
func NewBackendRegistry(cfg *Config) *BackendRegistry {
    registry := &BackendRegistry{
        backends:  make(map[string]Backend),
        defaultID: "00",
    }
    
    // ID:00 - 内部Vector API（常に登録）
    registry.backends["00"] = NewInternalBackend(cfg.InternalVectorURL)
    
    // ID:01以降 - 将来実装
    // registry.backends["01"] = NewSoracomBackend(cfg.SoracomConfig)
    // registry.backends["02"] = NewPartnerABackend(cfg.PartnerAConfig)
    
    return registry
}
```

### 4.5 将来: 設定ファイル方式

将来的には設定ファイル（YAML）で接続方式を管理する。

```yaml
# configs/vector-gateway/backends.yaml
backends:
  - id: "00"
    type: internal
    config:
      url: "http://vector-api:8080/api/v1/vector"
  
  - id: "01"
    type: soracom
    config:
      endpoint: "https://g.api.soracom.io"
      auth_key_id: "${SORACOM_AUTH_KEY_ID}"
      auth_key_secret: "${SORACOM_AUTH_KEY_SECRET}"
  
  - id: "02"
    type: partner-a
    config:
      endpoint: "https://api.partner-a.example.com"
      api_key: "${PARTNER_A_API_KEY}"

plmn_map:
  "44010": "01"  # docomo → SORACOM
  "44020": "01"  # SoftBank → SORACOM
  "310260": "02" # T-Mobile → Partner A
```

Docker Composeでのマウント:

```yaml
vector-gateway:
  volumes:
    - ./configs/vector-gateway/backends.yaml:/app/config/backends.yaml:ro
```

### 4.6 Dockerfile方針

#### 4.6.1 マルチステージビルド構成

```dockerfile
# ビルドステージ
FROM golang:1.25-bookworm AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o vector-gateway .

# ランタイムステージ
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/vector-gateway /usr/local/bin/vector-gateway

EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -fsS http://localhost:8080/health || exit 1

ENTRYPOINT ["/usr/local/bin/vector-gateway"]
```

#### 4.6.2 ベースイメージ選定

| ステージ   | イメージ               | 理由                                     |
| ---------- | ---------------------- | ---------------------------------------- |
| ビルド     | `golang:1.25-bookworm` | Go 1.25.x、Debian Bookwormベース         |
| ランタイム | `debian:bookworm-slim` | 最小構成、ヘルスチェック用curlが導入可能 |

#### 4.6.3 必須パッケージ

| パッケージ        | 用途                             |
| ----------------- | -------------------------------- |
| `ca-certificates` | TLS証明書（外部API連携に備える） |
| `curl`            | ヘルスチェック（`curl -fsS`）    |

> **注記:** distrolessイメージは採用しない。ヘルスチェックに `curl` が必要なため。

---

## 5. 環境変数設計

### 5.1 PoC版

```bash
# =============================================================================
# Vector Gateway 環境変数設定（PoC版）
# =============================================================================

# -----------------------------------------------------------------------------
# 動作モード
# -----------------------------------------------------------------------------
# gateway    : PLMNマップに基づきルーティング（デフォルト）
# passthrough: 全リクエストをVector APIに転送（開発・デバッグ用）
VECTOR_GATEWAY_MODE=gateway

# -----------------------------------------------------------------------------
# 内部Vector API設定
# -----------------------------------------------------------------------------
VECTOR_GATEWAY_INTERNAL_URL=http://vector-api:8080/api/v1/vector

# -----------------------------------------------------------------------------
# PLMNマッピング
# -----------------------------------------------------------------------------
# 形式: "PLMN:ID,PLMN:ID,..."
# - PLMN: MCC+MNC結合形式（5-6桁）
# - ID: 接続方式ID（2桁）
#   - 00: Vector API（内部）
#   - 01-99: 外部API（将来実装、現時点では501エラー）
#
# 空文字列の場合: 全てVector APIへ転送
#
# 例: VECTOR_GATEWAY_PLMN_MAP="44010:01,44020:01,310260:02"
VECTOR_GATEWAY_PLMN_MAP=""

# -----------------------------------------------------------------------------
# タイムアウト設定
# -----------------------------------------------------------------------------
VECTOR_GATEWAY_INTERNAL_TIMEOUT=5s

# -----------------------------------------------------------------------------
# ログ設定
# -----------------------------------------------------------------------------
# IMSIマスキング有効化（デフォルト: true）
# - true : IMSI中央部分をマスク（本番環境推奨）
# - false: IMSI全桁表示（デバッグ用）
LOG_MASK_IMSI=true
```

### 5.2 docker-compose.yml（抜粋）

```yaml
services:
  auth-server:
    environment:
      # vector-api → vector-gateway に変更
      - VECTOR_API_URL=http://vector-gateway:8080/api/v1/vector
      - LOG_MASK_IMSI=${LOG_MASK_IMSI:-true}
    depends_on:
      - vector-gateway

  vector-gateway:
    build: ./apps/vector-gateway
    expose:
      - "8080"
    environment:
      - VECTOR_GATEWAY_MODE=${VECTOR_GATEWAY_MODE:-gateway}
      - VECTOR_GATEWAY_INTERNAL_URL=http://vector-api:8080/api/v1/vector
      - VECTOR_GATEWAY_PLMN_MAP=${VECTOR_GATEWAY_PLMN_MAP:-}
      - VECTOR_GATEWAY_INTERNAL_TIMEOUT=${VECTOR_GATEWAY_INTERNAL_TIMEOUT:-5s}
      - LOG_MASK_IMSI=${LOG_MASK_IMSI:-true}
    depends_on:
      - vector-api
    restart: always
    logging: *fluent-bit-logging

  vector-api:
    # 既存のまま変更なし（LOG_MASK_IMSIはD-11で定義済み）
```

---

## 6. パッケージ構成

### 6.1 ディレクトリ構造

```
apps/vector-gateway/
├── main.go
└── internal/
    ├── backend/
    │   ├── errors.go          # エラー定義
    │   ├── errors_test.go
    │   ├── interface.go       # Backend共通インターフェース
    │   ├── internal.go        # 内部Vector API呼び出し（ID:00）
    │   ├── internal_test.go
    │   ├── registry.go        # バックエンド登録管理
    │   └── registry_test.go
    ├── config/
    │   ├── config.go          # 環境変数読み込み
    │   └── config_test.go
    ├── handler/
    │   ├── health.go          # /health ヘルスチェックハンドラ
    │   ├── vector.go          # /api/v1/vector ハンドラ
    │   └── vector_test.go
    ├── logging/
    │   ├── mask.go            # IMSIマスキング
    │   └── mask_test.go
    ├── router/
    │   ├── router.go          # PLMNベースルーティング
    │   └── router_test.go
    └── server/
        ├── middleware.go      # X-Trace-ID伝搬等ミドルウェア
        ├── router.go          # HTTPルーター設定
        └── server.go          # HTTPサーバー起動・管理
```

> **注記（r3からの変更点）:**
> - `cmd/` パッケージは廃止し、エントリポイントは `main.go` に統合
> - `middleware/` 独立パッケージは `server/middleware.go` に統合
> - `logging/` パッケージを新設（IMSIマスキング機能）
> - `handler/health.go` を新設（ヘルスチェックハンドラ）
> - `server/` パッケージを新設（HTTPサーバー管理・ルーター設定）
> - `resty`（HTTPクライアントライブラリ）は不使用、`net/http` を直接使用

### 6.2 設定構造体

```go
// internal/config/config.go

type Config struct {
    // 動作モード
    Mode string `envconfig:"VECTOR_GATEWAY_MODE" default:"gateway"`
    
    // 内部Vector API
    InternalURL     string        `envconfig:"VECTOR_GATEWAY_INTERNAL_URL" required:"true"`
    InternalTimeout time.Duration `envconfig:"VECTOR_GATEWAY_INTERNAL_TIMEOUT" default:"5s"`
    
    // PLMNマッピング（カンマ区切り）
    PLMNMapRaw string `envconfig:"VECTOR_GATEWAY_PLMN_MAP" default:""`
    
    // パース済みPLMNマップ
    PLMNMap map[string]string `ignored:"true"`

    // ログ設定
    LogMaskIMSI bool `envconfig:"LOG_MASK_IMSI" default:"true"`
}

func (c *Config) ParsePLMNMap() error {
    c.PLMNMap = make(map[string]string)
    
    if c.PLMNMapRaw == "" {
        return nil
    }
    
    entries := strings.Split(c.PLMNMapRaw, ",")
    for _, entry := range entries {
        entry = strings.TrimSpace(entry)
        if entry == "" {
            continue
        }
        
        parts := strings.Split(entry, ":")
        if len(parts) != 2 {
            return fmt.Errorf("invalid PLMN map entry: %s", entry)
        }
        
        plmn := strings.TrimSpace(parts[0])
        backendID := strings.TrimSpace(parts[1])
        
        // バリデーション
        if len(plmn) < 5 || len(plmn) > 6 {
            return fmt.Errorf("invalid PLMN format: %s", plmn)
        }
        if len(backendID) != 2 {
            return fmt.Errorf("invalid backend ID format: %s", backendID)
        }
        
        c.PLMNMap[plmn] = backendID
    }
    
    return nil
}
```

---

## 7. API仕様

### 7.1 エンドポイント

| メソッド | パス | 説明 |
|---------|------|------|
| POST | `/api/v1/vector` | 認証ベクター取得（既存互換） |
| GET | `/health` | ヘルスチェック |

### 7.2 リクエスト/レスポンス（既存互換）

```
POST /api/v1/vector
Headers:
  Content-Type: application/json
  X-Trace-ID: {uuid}

Request Body:
{
  "imsi": "440101234567890",
  "resync_info": {           // オプション（再同期時のみ）
    "rand": "f4b3...",
    "auts": "9a2c..."
  }
}

Response (200 OK):
{
  "rand": "f4b38a...",
  "autn": "2b9e10...",
  "xres": "d8a1...",
  "ck": "91e3...",
  "ik": "c42f..."
}
```

### 7.3 エラーレスポンス

| HTTPステータス | 状況 | レスポンス例 |
|---------------|------|------------|
| 400 Bad Request | リクエスト不正 | `{"error": "invalid IMSI format"}` |
| 404 Not Found | IMSI未登録（内部API） | `{"error": "IMSI not found"}` |
| 501 Not Implemented | 未実装バックエンド | `{"error": "Backend ID 01 is not implemented"}` |
| 500 Internal Server Error | 内部エラー | `{"error": "internal server error"}` |
| 502 Bad Gateway | バックエンド通信エラー | `{"error": "backend communication failed"}` |

---

## 8. エラーハンドリング

### 8.1 エラー分類

| カテゴリ | HTTPステータス | event_id | 対処 |
|---------|---------------|----------|------|
| リクエスト不正 | 400 | `REQUEST_INVALID` | エラー返却 |
| バックエンド未実装 | 501 | `BACKEND_NOT_IMPLEMENTED` | エラー返却 |
| 内部API通信エラー | 502 | `BACKEND_INTERNAL_ERR` | エラー返却 |
| 内部API 404応答 | 404 | （内部APIからの伝搬） | エラー返却 |
| 内部APIその他エラー | 500 | `BACKEND_INTERNAL_ERR` | エラー返却 |

### 8.2 将来: 外部API用エラー

| カテゴリ | HTTPステータス | event_id | 対処 |
|---------|---------------|----------|------|
| 外部API通信エラー | 502 | `BACKEND_EXTERNAL_ERR` | CB適用後エラー返却 |
| 外部API認証失敗 | 502 | `EXTERNAL_AUTH_ERR` | エラー返却 |
| 外部API Rate Limit | 503 | `EXTERNAL_RATE_LIMIT` | リトライ後エラー返却 |

---

## 9. トレーサビリティ

### 9.1 Trace ID伝搬

#### 内部バックエンド（Vector API）利用時

エンドツーエンドでX-Trace-IDを伝搬し、完全な追跡を保証する。

```
Auth Server (trace_id生成)
     │
     │ Header: X-Trace-ID: {uuid}
     ▼
Vector Gateway (伝搬・ログ出力)
     │
     │ Header: X-Trace-ID: {uuid}
     ▼
Vector API (伝搬・ログ出力)
```

#### 外部バックエンド利用時（将来）

Vector Gatewayまでのトレーサビリティを保証する。外部APIへのヘッダ付与はベストエフォートとし、外部側の対応は期待しない。

```
Auth Server (trace_id生成)
     │
     │ Header: X-Trace-ID: {uuid}
     ▼
Vector Gateway (ログ出力で境界記録)
     │
     │ Header: X-Trace-ID: {uuid} ※ベストエフォート
     ▼
外部API (対応は保証されない)
```

外部API呼び出し時は、Vector Gatewayのログで以下の情報を記録し、トレーサビリティの境界を明確化する：

```json
{
  "event_id": "BACKEND_EXTERNAL_CALL",
  "trace_id": "550e8400-e29b-...",
  "imsi": "31026*****890",
  "backend_id": "01",
  "external_endpoint": "https://api.example.com/v1/vector",
  "latency_ms": 150,
  "http_status": 200
}
```

### 9.2 ログ設計

#### event_id一覧（PoC）

| event_id | 発生条件 | レベル |
|----------|---------|--------|
| `PLMN_ROUTE_MATCH` | PLMNマッチでバックエンド選択 | DEBUG |
| `PLMN_ROUTE_UNMATCH` | PLMNマップに未登録（デフォルト動作） | DEBUG |
| `BACKEND_NOT_IMPLEMENTED` | 未実装接続方式IDが指定された | WARN |
| `BACKEND_INTERNAL_CALL` | 内部Vector API呼び出し | INFO |
| `BACKEND_INTERNAL_ERR` | 内部Vector API呼び出し失敗 | ERROR |
| `REQUEST_INVALID` | リクエスト形式不正 | WARN |

#### event_id一覧（将来追加予定）

| event_id | 発生条件 | レベル |
|----------|---------|--------|
| `BACKEND_EXTERNAL_CALL` | 外部API呼び出し | INFO |
| `BACKEND_EXTERNAL_ERR` | 外部API呼び出し失敗 | ERROR |
| `EXTERNAL_AUTH_ERR` | 外部API認証失敗 | ERROR |
| `EXTERNAL_RATE_LIMIT` | 外部API Rate Limit | WARN |

#### ログ出力例

```json
{
  "time": "2026-01-05T12:00:00.000Z",
  "level": "INFO",
  "app": "vector-gateway",
  "event_id": "BACKEND_INTERNAL_CALL",
  "trace_id": "550e8400-e29b-...",
  "msg": "calling internal vector API",
  "imsi": "44010*****890",
  "backend_id": "00",
  "backend_name": "vector-api"
}
```

### 9.3 IMSIマスキング設定

セキュリティ上、ログに出力するIMSIは中央部分をマスクする。D-04「ログ仕様設計書」で定義された仕様に準拠する。

#### 9.3.1 環境変数による制御

| 環境変数 | デフォルト | 説明 |
|---------|-----------|------|
| `LOG_MASK_IMSI` | `true` | `false` でマスキング無効化（デバッグ用） |

#### 9.3.2 マスキング仕様

| 設定値 | 動作 | 出力例（入力: `440101234567890`） |
|--------|------|--------------------------------|
| `true`（デフォルト） | 先頭6桁 + マスク + 末尾1桁 | `440101********0` |
| `false` | マスクなし（全桁表示） | `440101234567890` |

#### 9.3.3 実装

```go
// internal/logging/mask.go

package logging

// MaskIMSI はIMSIをマスキングする
func MaskIMSI(imsi string, enabled bool) string {
    if !enabled {
        return imsi
    }
    if len(imsi) <= 6 {
        return imsi
    }
    return imsi[:6] + "********" + imsi[len(imsi)-1:]
}
```

#### 9.3.4 適用箇所

Vector Gatewayにおいて、以下のevent_idを含むログ出力時にマスキングを適用する。

| event_id | 出力箇所 | imsiフィールド |
|----------|---------|---------------|
| `PLMN_ROUTE_MATCH` | PLMNマッチ時 | マスキング対象 |
| `PLMN_ROUTE_UNMATCH` | PLMNマップ未登録時 | マスキング対象 |
| `BACKEND_INTERNAL_CALL` | 内部API呼び出し時 | マスキング対象 |
| `BACKEND_INTERNAL_ERR` | 内部API呼び出し失敗時 | マスキング対象 |
| `BACKEND_EXTERNAL_CALL` | 外部API呼び出し時（将来） | マスキング対象 |
| `GW_REQUEST_OK` | リクエスト成功時 | マスキング対象 |

**実装例（セクション3.5/4.2のコード修正）:**

```go
// ルーティングロジック内のログ出力
slog.Debug("PLMN matched",
    "event_id", "PLMN_ROUTE_MATCH",
    "plmn", extractPLMN(imsi),
    "imsi", logging.MaskIMSI(imsi, cfg.LogMaskIMSI),  // 環境変数で制御
    "backend_id", backendID)

// 内部バックエンド呼び出し
slog.Info("calling internal vector API",
    "event_id", "BACKEND_INTERNAL_CALL",
    "trace_id", traceID,
    "imsi", logging.MaskIMSI(req.IMSI, cfg.LogMaskIMSI),  // 環境変数で制御
    "backend_id", "00",
    "backend_name", "vector-api")
```

#### 9.3.5 注意事項

- `LOG_MASK_IMSI=false` の設定は、ログファイルへのアクセス制御が適切に行われている環境でのみ使用すること
- Vector APIとマスキング設定を統一するため、同じ環境変数名 `LOG_MASK_IMSI` を使用する

---

## 10. 実装フェーズ

### 10.1 Phase 1: PoC（内部パススルー）

**目標:** Vector Gatewayの基盤実装、既存動作に影響なし

| 項目 | 内容 |
|------|------|
| 実装範囲 | 内部Vector APIへのパススルー |
| ルーティング | PLMNマップ対応（空設定で全て内部へ） |
| 接続方式 | ID:00（Vector API）のみ実装 |
| 未登録PLMNデフォルト | Vector API利用 |
| 未実装ID | 501 Not Implemented返却 |
| Auth Server変更 | `VECTOR_API_URL` のみ変更 |
| テスト | 既存テストが通ること |

**工数目安:** 2-3日

### 10.2 将来フェーズ

| フェーズ | 内容 | 前提条件 |
|---------|------|---------|
| **Phase 2** | 外部API接続方式の実装 | 接続先API仕様確定 |
| **Phase 3** | 設定ファイル方式への移行 | Phase 2完了 |
| **Phase 4** | Valkey + Admin TUI管理 | Phase 3完了 |

---

## 11. PoC実装スコープサマリ

| 項目 | PoC実装 | 将来拡張 |
|------|---------|---------|
| ルーティング | 環境変数ベース（PLMNマップ） | 設定ファイル → Valkey + Admin TUI |
| 接続方式 | ID:00（Vector API）のみ実動作 | 外部API接続方式を順次追加 |
| PLMN照合 | 固定長照合 | 必要に応じてMCC判定追加 |
| 未実装ID | 501エラー返却 | 接続方式実装に応じて解消 |
| 未登録PLMNデフォルト | Vector API利用 | エラー返却（CB発動抑制目的） |
| フォールバック | なし | SQN競合問題のため未実装継続見込み |
| モデル2（2フェーズ認証） | 未対応 | AT_MAC問題のため対応困難 |
| トレーサビリティ | 内部API: エンドツーエンド | 外部API: Gatewayまで保証 |

---

## 12. ドキュメント・設計への影響

### 12.1 新規作成ドキュメント

| No. | ドキュメント名 | 内容 |
|-----|---------------|------|
| D-12 | Vector Gateway詳細設計書 | パッケージ構成、API仕様、バックエンド連携 |

### 12.2 改訂対象ドキュメント

| No. | ドキュメント名 | 現行版数 | 改訂内容 |
|-----|---------------|---------|---------|
| D-01 | ミニPC版設計仕様書 | r9 | アーキテクチャ図にVector Gateway追加、パッケージマップ更新 |
| D-02 | Valkeyデータ設計仕様書 | r10 | （参考） |
| D-03 | Vector-APIインターフェース定義書 | r5 | （参考） |
| D-04 | ログ仕様設計書 | r13 | Vector Gateway用event_id追加 |
| D-05 | Admin TUI詳細設計書（前半） | r5 | （参考） |
| D-06 | エラーハンドリング詳細設計書 | r6 | 501エラー処理追加 |
| D-07 | Admin TUI詳細設計書（後半） | r3 | （参考） |
| D-08 | インフラ設定・運用設計書 | r10（予定） | Docker Compose設定追加 |
| E-02 | コーディング規約（簡易版） | r1 | （参考） |
| E-03 | 共通ライブラリ pkg 設計書 | r2 | （参考） |

### 12.3 ドキュメント一覧への反映

```
D-12: Vector Gateway詳細設計書 (未) ◄── 新規追加
```

---

## 13. リスクと対策

| リスク | 影響 | 対策 |
|--------|------|------|
| 外部API仕様変更 | 変換ロジック修正必要 | 接続方式ごとにモジュール化、変更を局所化 |
| 未登録PLMN大量流入 | Vector APIへの負荷増大 | 将来的にデフォルトをエラーに変更検討 |
| 設定ミス（PLMN形式） | ルーティング失敗 | 起動時バリデーション、ログ出力 |

---

## 14. 次のステップ

### 14.1 PoC実装タスク

| No. | タスク | 優先度 |
|-----|--------|--------|
| 1 | パッケージ雛形作成 | 高 |
| 2 | 環境変数読み込み・バリデーション | 高 |
| 3 | PLMNルーティングロジック | 高 |
| 4 | 内部バックエンド実装 | 高 |
| 5 | HTTPハンドラ実装 | 高 |
| 6 | Dockerfile作成 | 高 |
| 7 | docker-compose.yml更新 | 高 |
| 8 | 単体テスト | 中 |
| 9 | 結合テスト | 中 |

### 14.2 将来検討事項

| No. | 事項 | 検討時期 |
|-----|------|---------|
| 1 | 外部API接続先の仕様調査 | 接続先確定時 |
| 2 | 設定ファイル方式の詳細設計 | Phase 2開始前 |
| 3 | Valkey管理の詳細設計 | Phase 3開始前 |
| 4 | 未登録PLMNデフォルト動作の変更 | 本番運用検討時 |

---

## 改訂履歴

| 版数 | 日付 | 内容 |
|------|------|------|
| Draft | 2026-01-04 | 初版ドラフト作成 |
| Draft r2 | 2026-01-05 | レビュー反映: SORACOM Endorse対応中止（AT_MAC問題）、PLMN形式変更（ハイフンなし結合形式）、未実装エラー501採用、デフォルト動作の将来変更検討追記、event_id更新（PLMN_ROUTE_UNMATCH） |
| r1 | 2026-01-05 | 正式版: トレーサビリティの責務境界を明確化（セクション2.3, 9.1）、内部/外部バックエンドでのX-Trace-ID伝搬範囲を整理 |
| r2 | 2026-01-18 | ドキュメント名変更（実装レベル検討書→詳細設計書）、IMSIマスキング設定追加: セクション5.1/5.2に環境変数LOG_MASK_IMSI追加、セクション6.2の設定構造体更新、セクション9.3新設（マスキング仕様・実装・適用箇所） |
| r3 | 2026-01-26 | インフラ基盤統一: セクション4.6新設（Dockerfile方針 - ベースイメージdebian:bookworm-slim、curl/ca-certificates導入、ヘルスチェックcurl -fsS） |
| r4 | 2026-02-18 | ディレクトリ構造全面更新、関連ドキュメント版数更新 |
