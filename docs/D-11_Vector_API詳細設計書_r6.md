# D-11 Vector API詳細設計書 (r6)

## ■セクション1: 概要

### 1.1 目的

本ドキュメントは、EAP-AKA RADIUS PoC環境における認証ベクター計算サーバー「Vector API」の実装レベル設計を定義する。

### 1.2 スコープ

**本書で扱う範囲：**

| 範囲 | 内容 |
|------|------|
| HTTPサーバー | Gin WebフレームワークによるAPI提供 |
| 認証ベクター計算 | Milenage アルゴリズムによる RAND/AUTN/XRES/CK/IK 生成 |
| SQN管理 | シーケンス番号のインクリメント・再同期処理 |
| 加入者データアクセス | Valkey から Ki/OPc/AMF/SQN の取得・更新 |
| エラー応答 | RFC 7807 準拠の Problem Details 形式 |

**本書で扱わない範囲：**

| 範囲 | 参照先 |
|------|--------|
| EAP-AKA ステートマシン | D-03, D-09 |
| 鍵導出処理（MK, MSK, K_aut） | D-09（Auth Server側で実施） |
| PLMNルーティング | D-12（Vector Gateway側で実施） |

### 1.3 関連ドキュメント

| No. | ドキュメント | 参照内容 |
|-----|-------------|---------|
| D-01 | ミニPC版設計仕様書 (r9) | システム構成、パッケージ利用マップ |
| D-02 | Valkeyデータ設計仕様書 (r10) | 加入者データ構造、キー設計、Go構造体、SQN競合制御 |
| D-03 | Vector-API/ステートマシン設計書 (r5) | API仕様、リクエスト/レスポンス定義 |
| D-04 | ログ仕様設計書 (r13) | event_id定義、ログフォーマット、SQN_CONFLICT_ERR |
| D-05 | Auth Server詳細設計書 (r5) | Auth Server連携仕様 |
| D-06 | エラーハンドリング詳細設計書 (r6) | エラー分類、タイムアウト設定、SQN競合エラー |
| D-07 | Admin TUI詳細設計書 (r3) | 管理用TUIアプリケーション仕様 |
| D-08 | インフラ設定・運用設計書 (r10) | 環境変数設定、テストベクターモード |
| D-12 | Vector Gateway詳細設計書 (r3) | X-Trace-ID伝搬、呼び出し元仕様 |
| E-02 | コーディング規約（簡易版） | コーディング規約 |
| E-03 | CI/CD設計書 (r2) | CI/CD設計 |

### 1.4 準拠規格

| 規格 | 内容 | 対応範囲 |
|------|------|---------|
| 3GPP TS 35.205 | 3G Security - Specification of the MILENAGE algorithm set | f1〜f5, f1*, f5* 関数 |
| 3GPP TS 35.206 | 3G Security - MILENAGE algorithm specification | 計算詳細、テストデータ |
| 3GPP TS 33.102 | 3G Security - Security architecture | 認証ベクター構造、SQN管理 |
| 3GPP TS 35.208 | 3G Security - Algorithm specification: Test data | テストベクター（E2Eテスト用） |
| RFC 7807 | Problem Details for HTTP APIs | エラーレスポンス形式 |

### 1.5 用語定義

| 用語 | 説明 |
|------|------|
| IMSI | International Mobile Subscriber Identity（15桁） |
| Ki | 加入者秘密鍵（128bit、Hex 32桁） |
| OPc | オペレータコード（OP から Ki で導出済み、128bit） |
| AMF | Authentication Management Field（16bit） |
| SQN | Sequence Number（48bit、Hex 12桁） |
| SEQ | SQNの上位43bit（シーケンスカウンタ） |
| IND | SQNの下位5bit（インデックス、0〜31） |
| RAND | 認証用乱数（128bit） |
| AUTN | Authentication Token（128bit = SQN⊕AK \|\| AMF \|\| MAC-A） |
| XRES | Expected Response（32-128bit、可変長） |
| CK | Cipher Key（128bit） |
| IK | Integrity Key（128bit） |
| AUTS | Re-synchronization Token（112bit = SQN⊕AK \|\| MAC-S） |
| AK | Anonymity Key（f5関数の出力、48bit） |
| Δ (Delta) | SQN許容範囲（2^28 = 268,435,456） |

---

## ■セクション2: パッケージ構成

### 2.1 ディレクトリ構造

```
apps/vector-api/
├── main.go                     # エントリーポイント
└── internal/
    ├── config/
    │   ├── config.go           # 環境変数読み込み、設定構造体
    │   └── config_test.go      # config パッケージテスト
    ├── dto/
    │   ├── error.go            # RFC 7807 エラーDTO
    │   ├── error_test.go       # error パッケージテスト
    │   ├── request.go          # リクエストDTO
    │   └── response.go         # レスポンスDTO
    ├── handler/
    │   ├── health.go           # GET /health ハンドラ
    │   ├── vector.go           # POST /api/v1/vector ハンドラ
    │   └── vector_test.go      # handler パッケージテスト
    ├── milenage/
    │   ├── calculator.go       # Milenage計算ラッパー
    │   ├── calculator_test.go  # calculator パッケージテスト
    │   ├── hex.go              # 16進数変換ユーティリティ
    │   ├── hex_test.go         # hex パッケージテスト
    │   ├── resync.go           # AUTS処理、SQN抽出（SQN再同期計算）
    │   └── resync_test.go      # resync パッケージテスト
    ├── server/
    │   ├── middleware.go       # ミドルウェア（Trace ID、ロギング、リカバリー）
    │   ├── router.go           # ルーティング定義
    │   └── server.go           # Ginサーバー設定・起動・シャットダウン
    ├── sqn/
    │   ├── manager.go          # SQN管理（インクリメント、更新）
    │   ├── manager_test.go     # manager パッケージテスト
    │   ├── validator.go        # SQN範囲検証
    │   └── validator_test.go   # validator パッケージテスト
    ├── store/
    │   ├── subscriber.go       # 加入者データアクセス
    │   └── valkey.go           # Valkeyクライアント初期化・管理
    ├── testmode/
    │   ├── testvector.go       # テストモード用固定ベクター
    │   └── testvector_test.go  # testmode パッケージテスト
    └── usecase/
        ├── error.go            # ユースケースエラー型定義
        ├── error_test.go       # error パッケージテスト
        ├── interfaces.go       # ユースケース層インターフェース定義
        ├── mock_interfaces.go  # テスト用モックインターフェース
        ├── vector.go           # ベクター生成・再同期ユースケース（統合）
        └── vector_test.go      # vector パッケージテスト
```

### 2.2 パッケージ依存関係図

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              main.go                                    │
│                                 │                                       │
│                    ┌────────────┴────────────┐                          │
│                    ▼                         ▼                          │
│              ┌──────────┐              ┌──────────┐                     │
│              │ config/  │              │ server/  │                     │
│              └────┬─────┘              └────┬─────┘                     │
│                   │                         │                           │
│                   │           ┌─────────────┼─────────────┐             │
│                   │           ▼             ▼             ▼             │
│                   │    ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│                   │    │middleware│  │ router/  │  │ handler/ │        │
│                   │    └──────────┘  └──────────┘  └────┬─────┘        │
│                   │                                      │              │
│                   │                         ┌────────────┴────────────┐ │
│                   │                         ▼                         ▼ │
│                   │                  ┌──────────┐               ┌──────────┐
│                   │                  │ usecase/ │               │   dto/   │
│                   │                  └────┬─────┘               └──────────┘
│                   │                       │                                 │
│                   │          ┌────────────┼────────────┬────────────┐      │
│                   │          ▼            ▼            ▼            ▼      │
│                   │   ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐
│                   │   │milenage/ │  │   sqn/   │  │  store/  │  │testmode/ │
│                   │   └──────────┘  └──────────┘  └──────────┘  └──────────┘
│                   │                                     │                  │
│                   └─────────────────────────────────────┘                  │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 2.3 パッケージ責務一覧

| パッケージ | 責務 | 主要な型・関数 |
|-----------|------|---------------|
| `config` | 環境変数読み込み、設定値管理 | `Config`, `Load()` |
| `server` | Ginサーバー管理、ルーティング、ミドルウェア | `Server`, `Router`, `TraceIDMiddleware` |
| `handler` | HTTPリクエスト処理、バリデーション、レスポンス生成 | `VectorHandler`, `HealthHandler` |
| `usecase` | ビジネスロジック（ベクター生成、再同期）、インターフェース定義、エラー型 | `VectorUseCase`, `ProblemError`, `MilenageCalculator`, `ResyncProcessor` |
| `milenage` | Milenage計算ラッパー、AUTS処理 | `Calculator`, `ResyncProcessor` |
| `sqn` | SQN管理、インクリメント、検証 | `Manager`, `Validator` |
| `store` | Valkeyアクセス抽象化 | `ValkeyClient`, `SubscriberStore` |
| `testmode` | E2Eテスト用固定ベクター生成 | `TestVectorProvider`, `IsTestIMSI()` |
| `dto` | データ転送オブジェクト、リクエスト/レスポンス構造体 | `VectorRequest`, `VectorResponse`, `ProblemDetail` |

### 2.4 外部パッケージ依存

D-01で定義されたパッケージ利用マップに基づく。

| カテゴリ | パッケージ | 用途 | 利用箇所 |
|---------|-----------|------|---------|
| **HTTP** | `github.com/gin-gonic/gin` | Web APIフレームワーク | `server/`, `handler/` |
| **Milenage** | `github.com/wmnsk/milenage` | AKA認証ベクター計算 | `milenage/` |
| **DB** | `github.com/redis/go-redis/v9` | Valkeyクライアント | `store/` |
| **Config** | `github.com/kelseyhightower/envconfig` | 環境変数読み込み | `config/` |
| **Logging** | `log/slog` (標準ライブラリ) | 構造化ログ | 全パッケージ |
| **Crypto** | `crypto/rand` (標準ライブラリ) | RAND生成 | `milenage/` |

### 2.5 パッケージ間インターフェース

レイヤー間の依存を疎結合に保つため、主要なインターフェースを定義する。

```go
// usecase/interfaces.go
type MilenageCalculator interface {
    GenerateVector(ki, opc, amf []byte, sqn uint64) (*Vector, error)
}

type ResyncProcessor interface {
    ExtractSQN(ki, opc, rand, auts []byte) (uint64, error)
}

type SQNManager interface {
    Increment(currentSQN uint64) uint64
    ValidateResyncSQN(sqnMS, sqnHE uint64) error
    ComputeResyncSQN(sqnMS uint64) uint64
    FormatHex(sqn uint64) string
    ParseHex(s string) (uint64, error)
}

type SubscriberRepository interface {
    Get(ctx context.Context, imsi string) (*Subscriber, error)
    UpdateSQN(ctx context.Context, imsi string, sqn uint64) error
}

type TestVectorProvider interface {
    IsTestIMSI(imsi string) bool
    GetTestVector(imsi string) (*Vector, error)
}

// handler/interfaces.go
type VectorUseCase interface {
    GenerateVector(ctx context.Context, req *VectorRequest) (*VectorResponse, error)
}
```

### 2.6 Dockerfile方針

#### 2.6.1 マルチステージビルド構成

```dockerfile
# ビルドステージ
FROM golang:1.25-bookworm AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o vector-api .

# ランタイムステージ
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/vector-api /usr/local/bin/vector-api

EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -fsS http://localhost:8080/health || exit 1

ENTRYPOINT ["/usr/local/bin/vector-api"]
```

#### 2.6.2 ベースイメージ選定

| ステージ   | イメージ               | 理由                                     |
| ---------- | ---------------------- | ---------------------------------------- |
| ビルド     | `golang:1.25-bookworm` | Go 1.25.x、Debian Bookwormベース         |
| ランタイム | `debian:bookworm-slim` | 最小構成、ヘルスチェック用curlが導入可能 |

#### 2.6.3 必須パッケージ

| パッケージ        | 用途                                                |
| ----------------- | --------------------------------------------------- |
| `ca-certificates` | TLS証明書（HTTPS通信用、将来の外部API連携に備える） |
| `curl`            | ヘルスチェック（`curl -fsS`）                       |

> **注記:** distrolessイメージは採用しない。ヘルスチェックに `curl` が必要なため。

### 2.7 ファイル別責務詳細

#### `internal/config/`

| ファイル | 責務 | 主要関数・型 |
|---------|------|-------------|
| `config.go` | 環境変数読み込み、設定構造体定義 | `Config`, `Load()` |

#### `internal/server/`

| ファイル | 責務 | 主要関数・型 |
|---------|------|-------------|
| `server.go` | Ginサーバー管理 | `Server`, `Run()`, `Shutdown()` |
| `router.go` | ルーティング定義 | `SetupRouter()` |
| `middleware.go` | ミドルウェア定義 | `TraceIDMiddleware()`, `LoggingMiddleware()`, `RecoveryMiddleware()` |

#### `internal/handler/`

| ファイル | 責務 | 主要関数・型 |
|---------|------|-------------|
| `vector.go` | ベクター生成APIハンドラ | `VectorHandler`, `HandleVector()` |
| `health.go` | ヘルスチェックAPIハンドラ | `HandleHealth()` |

#### `internal/usecase/`

| ファイル | 責務 | 主要関数・型 |
|---------|------|-------------|
| `vector.go` | ベクター生成・再同期ユースケース（統合） | `VectorUseCase`, `GenerateVector()`, `processResync()` |
| `interfaces.go` | ユースケース層インターフェース定義 | `MilenageCalculator`, `ResyncProcessor`, `SQNManager`, `SubscriberRepository`, `TestVectorProvider` |
| `error.go` | ユースケースエラー型定義 | `ProblemError`, `ErrSubscriberNotFound`, `ErrSQNConflict` 等 |
| `mock_interfaces.go` | テスト用モックインターフェース | 各インターフェースのモック実装 |

#### `internal/milenage/`

| ファイル | 責務 | 主要関数・型 |
|---------|------|-------------|
| `calculator.go` | Milenage計算ラッパー | `Calculator`, `GenerateVector()`, `ComputeF1()` 〜 `ComputeF5()` |
| `hex.go` | 16進数変換ユーティリティ | `HexDecode()`, `HexEncode()`, `VectorToResponse()` |
| `resync.go` | AUTS処理、SQN抽出、SQN再同期計算 | `ResyncProcessor`, `ExtractSQN()`, `VerifyMACS()` |

#### `internal/sqn/`

| ファイル | 責務 | 主要関数・型 |
|---------|------|-------------|
| `manager.go` | SQN管理、インクリメントロジック | `Manager`, `Increment()`, `FormatHex()` |
| `validator.go` | SQN範囲検証、デルタチェック | `Validator`, `ValidateResyncSQN()` |

#### `internal/store/`

| ファイル | 責務 | 主要関数・型 |
|---------|------|-------------|
| `valkey.go` | Valkeyクライアント初期化・管理 | `ValkeyClient`, `NewValkeyClient()`, `Ping()` |
| `subscriber.go` | 加入者データアクセス | `SubscriberStore`, `Get()`, `UpdateSQN()` |

#### `internal/testmode/`

| ファイル | 責務 | 主要関数・型 |
|---------|------|-------------|
| `testvector.go` | テストモード判定、固定ベクター生成 | `TestVectorProvider`, `IsTestIMSI()`, `GetTestVector()` |

#### `internal/dto/`

| ファイル | 責務 | 主要関数・型 |
|---------|------|-------------|
| `request.go` | リクエストDTO定義 | `VectorRequest`, `ResyncInfo` |
| `response.go` | レスポンスDTO定義 | `VectorResponse` |
| `error.go` | RFC 7807エラーDTO定義 | `ProblemDetail`, `NewProblemDetail()` |

---

## ■セクション3: 設定・初期化

### 3.1 環境変数一覧

| 環境変数 | 必須 | デフォルト | 型 | 説明 |
|---------|------|-----------|-----|------|
| `REDIS_HOST` | Yes | - | string | Valkeyホスト名 |
| `REDIS_PORT` | Yes | - | string | Valkeyポート番号 |
| `REDIS_PASS` | Yes | - | string | Valkeyパスワード |
| `LISTEN_ADDR` | No | `:8080` | string | HTTPリッスンアドレス |
| `LOG_LEVEL` | No | `INFO` | string | ログレベル（DEBUG/INFO/WARN/ERROR） |
| `LOG_MASK_IMSI` | No | `true` | bool | IMSIマスキング有効化 |
| `GIN_MODE` | No | `release` | string | Gin動作モード（debug/release） |
| `TEST_VECTOR_ENABLED` | No | `false` | bool | テストベクターモード有効化 |
| `TEST_VECTOR_IMSI_PREFIX` | No | `00101` | string | テスト対象IMSIプレフィックス（5-6桁） |

### 3.2 設定構造体

```go
// internal/config/config.go

type Config struct {
    // Valkey設定
    RedisHost string `envconfig:"REDIS_HOST" required:"true"`
    RedisPort string `envconfig:"REDIS_PORT" required:"true"`
    RedisPass string `envconfig:"REDIS_PASS" required:"true"`
    
    // サーバー設定
    ListenAddr string `envconfig:"LISTEN_ADDR" default:":8080"`
    LogLevel   string `envconfig:"LOG_LEVEL" default:"INFO"`
    LogMaskIMSI bool  `envconfig:"LOG_MASK_IMSI" default:"true"`
    GinMode    string `envconfig:"GIN_MODE" default:"release"`
    
    // テストモード設定
    TestVectorEnabled    bool   `envconfig:"TEST_VECTOR_ENABLED" default:"false"`
    TestVectorIMSIPrefix string `envconfig:"TEST_VECTOR_IMSI_PREFIX" default:"00101"`
}

func Load() (*Config, error) {
    var cfg Config
    if err := envconfig.Process("", &cfg); err != nil {
        return nil, fmt.Errorf("failed to load config: %w", err)
    }
    return &cfg, nil
}

// RedisAddr はValkey接続文字列を返す
func (c *Config) RedisAddr() string {
    return net.JoinHostPort(c.RedisHost, c.RedisPort)
}
```

### 3.3 起動処理フロー

```
main()
   │
   ├─1. config.Load()
   │      └─ 環境変数読み込み・検証
   │
   ├─2. slog.SetDefault()
   │      └─ ロガー初期化（JSON形式）
   │
   ├─3. store.NewValkeyClient()
   │      └─ Valkey接続確立・Ping確認
   │
   ├─4. 依存オブジェクト生成
   │      ├─ milenage.NewCalculator()
   │      ├─ sqn.NewManager()
   │      ├─ sqn.NewValidator()
   │      ├─ testmode.NewTestVectorProvider()  ※条件付き
   │      ├─ store.NewSubscriberStore()
   │      ├─ usecase.NewVectorUseCase()
   │      └─ handler.NewVectorHandler()
   │
   ├─5. server.New()
   │      ├─ Ginエンジン初期化
   │      ├─ ミドルウェア登録
   │      └─ ルーティング設定
   │
   ├─6. server.Run()
   │      └─ HTTPリッスン開始
   │
   └─7. シグナル待機
          └─ SIGINT/SIGTERM → Graceful Shutdown
```

### 3.4 Valkey初期化

```go
// internal/store/valkey.go

import (
    "context"
    "time"
    
    "github.com/redis/go-redis/v9"
)

type ValkeyClient struct {
    client *redis.Client
}

func NewValkeyClient(cfg *config.Config) (*ValkeyClient, error) {
    client := redis.NewClient(&redis.Options{
        Addr:         cfg.RedisAddr(),
        Password:     cfg.RedisPass,
        DB:           0,
        DialTimeout:  3 * time.Second,
        ReadTimeout:  2 * time.Second,
        WriteTimeout: 2 * time.Second,
        PoolSize:     10,
    })
    
    // 接続確認
    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()
    
    if err := client.Ping(ctx).Err(); err != nil {
        return nil, fmt.Errorf("failed to connect to Valkey: %w", err)
    }
    
    return &ValkeyClient{client: client}, nil
}

func (v *ValkeyClient) Close() error {
    return v.client.Close()
}
```

---

## ■セクション4: HTTPサーバー設計

### 4.1 Ginエンジン設定

```go
// internal/server/server.go

import (
    "context"
    "net/http"
    "time"
    
    "github.com/gin-gonic/gin"
)

type Server struct {
    engine *gin.Engine
    server *http.Server
    cfg    *config.Config
}

func New(cfg *config.Config, handler *handler.VectorHandler) *Server {
    // Ginモード設定
    gin.SetMode(cfg.GinMode)
    
    engine := gin.New()
    
    // ミドルウェア登録
    engine.Use(TraceIDMiddleware())
    engine.Use(LoggingMiddleware(cfg))
    engine.Use(RecoveryMiddleware())
    
    // ルーティング
    SetupRouter(engine, handler)
    
    return &Server{
        engine: engine,
        server: &http.Server{
            Addr:    cfg.ListenAddr,
            Handler: engine,
        },
        cfg: cfg,
    }
}

func (s *Server) Run() error {
    slog.Info("starting server", "addr", s.cfg.ListenAddr)
    return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
    slog.Info("shutting down server")
    return s.server.Shutdown(ctx)
}
```

### 4.2 ミドルウェア

#### TraceIDミドルウェア

Vector Gatewayから伝搬される `X-Trace-ID` ヘッダを読み取り、コンテキストに設定する。

```go
// internal/server/middleware.go

const TraceIDKey = "trace_id"
const TraceIDHeader = "X-Trace-ID"

func TraceIDMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        traceID := c.GetHeader(TraceIDHeader)
        if traceID == "" {
            traceID = "no-trace-id"
        }
        c.Set(TraceIDKey, traceID)
        c.Next()
    }
}
```

#### ロギングミドルウェア

```go
func LoggingMiddleware(cfg *config.Config) gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        
        c.Next()
        
        latency := time.Since(start)
        traceID, _ := c.Get(TraceIDKey)
        
        slog.Info("request completed",
            "trace_id", traceID,
            "method", c.Request.Method,
            "path", c.Request.URL.Path,
            "http_status", c.Writer.Status(),
            "latency_ms", latency.Milliseconds(),
        )
    }
}
```

#### リカバリーミドルウェア

```go
func RecoveryMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        defer func() {
            if err := recover(); err != nil {
                traceID, _ := c.Get(TraceIDKey)
                slog.Error("panic recovered",
                    "trace_id", traceID,
                    "error", err,
                )
                c.AbortWithStatusJSON(http.StatusInternalServerError, dto.NewProblemDetail(
                    http.StatusInternalServerError,
                    "Internal Server Error",
                    "An unexpected error occurred",
                ))
            }
        }()
        c.Next()
    }
}
```

### 4.3 ルーティング

```go
// internal/server/router.go

func SetupRouter(engine *gin.Engine, handler *handler.VectorHandler) {
    // ヘルスチェック
    engine.GET("/health", handler.HandleHealth)
    
    // API v1
    v1 := engine.Group("/api/v1")
    {
        v1.POST("/vector", handler.HandleVector)
    }
}
```

---

## ■セクション5: エンドポイント実装

### 5.1 POST /api/v1/vector

```go
// internal/handler/vector.go

type VectorHandler struct {
    useCase VectorUseCase
    cfg     *config.Config
}

func NewVectorHandler(useCase VectorUseCase, cfg *config.Config) *VectorHandler {
    return &VectorHandler{
        useCase: useCase,
        cfg:     cfg,
    }
}

func (h *VectorHandler) HandleVector(c *gin.Context) {
    traceID, _ := c.Get(TraceIDKey)
    ctx := c.Request.Context()
    
    // 1. リクエストバインド
    var req dto.VectorRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        slog.Warn("invalid request body",
            "trace_id", traceID,
            "event_id", "CALC_ERR",
            "error", err.Error(),
        )
        c.JSON(http.StatusBadRequest, dto.NewProblemDetail(
            http.StatusBadRequest,
            "Bad Request",
            "Invalid request body",
        ))
        return
    }
    
    // 2. IMSI検証
    if err := validateIMSI(req.IMSI); err != nil {
        slog.Warn("invalid IMSI format",
            "trace_id", traceID,
            "event_id", "CALC_ERR",
            "imsi", h.maskIMSI(req.IMSI),
            "error", err.Error(),
        )
        c.JSON(http.StatusBadRequest, dto.NewProblemDetail(
            http.StatusBadRequest,
            "Bad Request",
            "IMSI must be 15 digits",
        ))
        return
    }
    
    // 3. ユースケース実行
    resp, err := h.useCase.GenerateVector(ctx, &req)
    if err != nil {
        h.handleError(c, traceID.(string), req.IMSI, err)
        return
    }
    
    // 4. 成功レスポンス
    slog.Info("vector generated",
        "trace_id", traceID,
        "event_id", "CALC_OK",
        "imsi", h.maskIMSI(req.IMSI),
        "http_status", http.StatusOK,
    )
    c.JSON(http.StatusOK, resp)
}

func (h *VectorHandler) handleError(c *gin.Context, traceID, imsi string, err error) {
    var problemErr *usecase.ProblemError
    if errors.As(err, &problemErr) {
        slog.Log(c.Request.Context(), problemErr.LogLevel(), problemErr.Message,
            "trace_id", traceID,
            "event_id", problemErr.EventID,
            "imsi", h.maskIMSI(imsi),
            "http_status", problemErr.Status,
        )
        c.JSON(problemErr.Status, problemErr.ToProblemDetail())
        return
    }
    
    // 予期しないエラー
    slog.Error("unexpected error",
        "trace_id", traceID,
        "event_id", "CALC_ERR",
        "imsi", h.maskIMSI(imsi),
        "error", err.Error(),
    )
    c.JSON(http.StatusInternalServerError, dto.NewProblemDetail(
        http.StatusInternalServerError,
        "Internal Server Error",
        "An unexpected error occurred",
    ))
}

// validateIMSI はIMSI形式を検証する
func validateIMSI(imsi string) error {
    if len(imsi) != 15 {
        return fmt.Errorf("IMSI must be 15 digits, got %d", len(imsi))
    }
    for _, c := range imsi {
        if c < '0' || c > '9' {
            return fmt.Errorf("IMSI must contain only digits")
        }
    }
    return nil
}

// maskIMSI はログ出力用にIMSIをマスクする
// 環境変数 LOG_MASK_IMSI が false の場合はマスクしない
func (h *VectorHandler) maskIMSI(imsi string) string {
    if !h.cfg.LogMaskIMSI {
        return imsi
    }
    if len(imsi) <= 6 {
        return imsi
    }
    return imsi[:6] + "********" + imsi[len(imsi)-1:]
}
```

### 5.2 GET /health

```go
// internal/handler/health.go

type HealthResponse struct {
    Status string `json:"status"`
}

func (h *VectorHandler) HandleHealth(c *gin.Context) {
    c.JSON(http.StatusOK, HealthResponse{Status: "ok"})
}
```

---

## ■セクション6: Milenage計算

### 6.1 wmnsk/milenage ライブラリ

外部ライブラリ `github.com/wmnsk/milenage` を使用してMilenage計算を実行する。

#### ライブラリが提供する機能

| 関数 | 説明 | 用途 |
|------|------|------|
| `milenage.F1()` | MAC-A計算 | AUTN生成時のネットワーク認証 |
| `milenage.F1star()` | MAC-S計算 | AUTS検証（再同期時） |
| `milenage.F2345()` | RES, CK, IK, AK同時計算 | 認証ベクター生成 |
| `milenage.F5star()` | AK*計算 | 再同期時のSQN復号 |

### 6.2 Calculator実装

```go
// internal/milenage/calculator.go

import (
    "crypto/rand"
    "encoding/hex"
    "fmt"
    
    "github.com/wmnsk/milenage"
)

// Vector は認証ベクターを表す
type Vector struct {
    RAND []byte // 16 bytes
    AUTN []byte // 16 bytes
    XRES []byte // 4-16 bytes (通常8 bytes)
    CK   []byte // 16 bytes
    IK   []byte // 16 bytes
}

type Calculator struct{}

func NewCalculator() *Calculator {
    return &Calculator{}
}

// GenerateVector は認証ベクターを生成する
func (c *Calculator) GenerateVector(ki, opc, amf []byte, sqn uint64) (*Vector, error) {
    // 1. RAND生成（128bit乱数）
    randVal := make([]byte, 16)
    if _, err := rand.Read(randVal); err != nil {
        return nil, fmt.Errorf("failed to generate RAND: %w", err)
    }
    
    // 2. SQNをバイト列に変換（48bit = 6 bytes）
    sqnBytes := sqnToBytes(sqn)
    
    // 3. f2345計算（RES, CK, IK, AK）
    res, ck, ik, ak, err := milenage.F2345(ki, opc, randVal)
    if err != nil {
        return nil, fmt.Errorf("failed to compute f2345: %w", err)
    }
    
    // 4. f1計算（MAC-A）
    macA, err := milenage.F1(ki, opc, randVal, sqnBytes, amf)
    if err != nil {
        return nil, fmt.Errorf("failed to compute f1: %w", err)
    }
    
    // 5. AUTN = (SQN ⊕ AK) || AMF || MAC-A
    autn := c.computeAUTN(sqnBytes, ak, amf, macA)
    
    return &Vector{
        RAND: randVal,
        AUTN: autn,
        XRES: res,
        CK:   ck,
        IK:   ik,
    }, nil
}

// computeAUTN はAUTNを計算する
// AUTN = (SQN ⊕ AK) || AMF || MAC-A
func (c *Calculator) computeAUTN(sqn, ak, amf, macA []byte) []byte {
    autn := make([]byte, 16)
    
    // SQN ⊕ AK (6 bytes)
    for i := 0; i < 6; i++ {
        autn[i] = sqn[i] ^ ak[i]
    }
    
    // AMF (2 bytes)
    copy(autn[6:8], amf)
    
    // MAC-A (8 bytes)
    copy(autn[8:16], macA)
    
    return autn
}

// sqnToBytes はSQN（uint64）を6バイトのバイト列に変換する
func sqnToBytes(sqn uint64) []byte {
    b := make([]byte, 6)
    b[0] = byte(sqn >> 40)
    b[1] = byte(sqn >> 32)
    b[2] = byte(sqn >> 24)
    b[3] = byte(sqn >> 16)
    b[4] = byte(sqn >> 8)
    b[5] = byte(sqn)
    return b
}

// bytesToSQN は6バイトのバイト列をSQN（uint64）に変換する
func bytesToSQN(b []byte) uint64 {
    return uint64(b[0])<<40 | uint64(b[1])<<32 | uint64(b[2])<<24 |
           uint64(b[3])<<16 | uint64(b[4])<<8 | uint64(b[5])
}
```

### 6.3 ResyncProcessor実装

```go
// internal/milenage/resync.go

import (
    "fmt"
    
    "github.com/wmnsk/milenage"
)

type ResyncProcessor struct{}

func NewResyncProcessor() *ResyncProcessor {
    return &ResyncProcessor{}
}

// ExtractSQN はAUTSからSQN_MSを抽出する
// AUTS = (SQN_MS ⊕ AK*) || MAC-S
// 
// 処理手順:
// 1. f5*(AK*) を計算
// 2. SQN_MS = (SQN_MS ⊕ AK*) ⊕ AK* で復号
// 3. f1*(MAC-S) を計算して検証
func (r *ResyncProcessor) ExtractSQN(ki, opc, randVal, auts []byte) (uint64, error) {
    if len(auts) != 14 {
        return 0, fmt.Errorf("invalid AUTS length: expected 14, got %d", len(auts))
    }
    
    // 1. f5*計算（AK*）
    akStar, err := milenage.F5star(ki, opc, randVal)
    if err != nil {
        return 0, fmt.Errorf("failed to compute f5*: %w", err)
    }
    
    // 2. SQN_MS復号
    sqnMSXorAKStar := auts[:6]
    sqnMSBytes := make([]byte, 6)
    for i := 0; i < 6; i++ {
        sqnMSBytes[i] = sqnMSXorAKStar[i] ^ akStar[i]
    }
    
    // 3. MAC-S検証
    macSReceived := auts[6:14]
    
    // AMFは再同期時は固定値（0x0000）を使用
    amfResync := []byte{0x00, 0x00}
    
    macSComputed, err := milenage.F1star(ki, opc, randVal, sqnMSBytes, amfResync)
    if err != nil {
        return 0, fmt.Errorf("failed to compute f1*: %w", err)
    }
    
    // MAC-S比較（タイミング攻撃対策で定数時間比較）
    if !constantTimeCompare(macSReceived, macSComputed) {
        return 0, fmt.Errorf("MAC-S verification failed")
    }
    
    return bytesToSQN(sqnMSBytes), nil
}

// constantTimeCompare は定数時間でバイト列を比較する（タイミング攻撃対策）
func constantTimeCompare(a, b []byte) bool {
    if len(a) != len(b) {
        return false
    }
    result := byte(0)
    for i := 0; i < len(a); i++ {
        result |= a[i] ^ b[i]
    }
    return result == 0
}
```

### 6.4 Hex変換ユーティリティ

```go
// internal/milenage/hex.go

// HexDecode はHex文字列をバイト列に変換する
func HexDecode(s string) ([]byte, error) {
    return hex.DecodeString(s)
}

// HexEncode はバイト列をHex文字列に変換する
func HexEncode(b []byte) string {
    return hex.EncodeToString(b)
}

// VectorToResponse はVectorをVectorResponseに変換する
func VectorToResponse(v *Vector) *dto.VectorResponse {
    return &dto.VectorResponse{
        RAND: HexEncode(v.RAND),
        AUTN: HexEncode(v.AUTN),
        XRES: HexEncode(v.XRES),
        CK:   HexEncode(v.CK),
        IK:   HexEncode(v.IK),
    }
}
```

---

## ■セクション7: SQN管理

### 7.1 SQN構造

3GPP TS 33.102 に基づくSQN構造:

```
SQN (48 bits) = SEQ (43 bits) || IND (5 bits)

SEQ: シーケンスカウンタ（0〜8,796,093,022,207）
IND: インデックス（0〜31、認証ドメイン識別子）

┌──────────────────────────────────────────────────┐
│                    SQN (48 bits)                  │
├────────────────────────────────────┬─────────────┤
│           SEQ (43 bits)            │ IND (5 bits)│
│   bit 47 ─────────────────── bit 5 │ bit 4 ─ 0  │
└────────────────────────────────────┴─────────────┘
```

### 7.2 インクリメント戦略

本実装では **IND固定・SEQのみインクリメント** 方式を採用する。

**設計ポイント:**

1. **IND固定**: Admin TUIで加入者登録時に設定したSQNのIND部分は変更しない
2. **SEQのみインクリメント**: 認証ごとにSEQ部分のみ+1する
3. **簡略化実装**: SQN全体に+32（2^5）を加算することで、INDを変化させずにSEQを+1

```
インクリメント前: SQN = SEQ || IND
インクリメント後: SQN = (SEQ + 1) || IND

簡略化: SQN_new = SQN_old + 32

例:
  SQN = 0x000000000000 (SEQ=0, IND=0)
  +32 → SQN = 0x000000000020 (SEQ=1, IND=0)
  +32 → SQN = 0x000000000040 (SEQ=2, IND=0)
```

**運用上の意図:**
- 認証ドメインをINDで分離することが可能
- Admin TUIでのSQN初期値設定でINDを指定（例: IND=5 → SQN初期値=0x000000000005）

```go
// internal/sqn/manager.go

const (
    // MaxSQN は48bit SQNの最大値
    MaxSQN = (1 << 48) - 1
    
    // IncrementStep はSQNインクリメント時の加算値（SEQを+1するためにIND部分をスキップ）
    // SQN = SEQ(43bit) || IND(5bit) なので、SEQ+1 = SQN+32
    IncrementStep = 32
)

type Manager struct{}

func NewManager() *Manager {
    return &Manager{}
}

// Increment はSQNをインクリメントする
// IND部分を固定したまま、SEQ部分のみ+1する
// 実装上は SQN + 32 で簡略化
func (m *Manager) Increment(currentSQN uint64) (uint64, error) {
    newSQN := currentSQN + IncrementStep
    
    // 48bit上限チェック（SEQオーバーフロー）
    if newSQN > MaxSQN {
        return 0, fmt.Errorf("SQN overflow: SEQ reached maximum value")
    }
    
    return newSQN, nil
}

// GetSEQ はSQNからSEQ部分を抽出する
func (m *Manager) GetSEQ(sqn uint64) uint64 {
    return sqn >> 5
}

// GetIND はSQNからIND部分を抽出する
func (m *Manager) GetIND(sqn uint64) uint8 {
    return uint8(sqn & 0x1F)
}

// FormatHex はSQNを12桁Hex文字列に変換する
func (m *Manager) FormatHex(sqn uint64) string {
    return fmt.Sprintf("%012x", sqn)
}

// ParseHex は12桁Hex文字列をSQNに変換する
func (m *Manager) ParseHex(s string) (uint64, error) {
    if len(s) != 12 {
        return 0, fmt.Errorf("invalid SQN hex length: expected 12, got %d", len(s))
    }
    return strconv.ParseUint(s, 16, 48)
}
```

### 7.3 デルタ（Δ）検証

3GPP TS 33.102 C.3.2 Profile 2 に基づき、再同期時のSQN妥当性を検証する。

**Δ（デルタ）の定義:**
- 値: 2^28 = 268,435,456
- 意味: SQN_MS と SQN_HE の許容される最大差

```go
// internal/sqn/validator.go

const (
    // Delta は3GPP TS 33.102 C.3.2 Profile 2で定義されるSQN許容範囲
    // 2^28 = 268,435,456
    Delta = 1 << 28
)

type Validator struct{}

func NewValidator() *Validator {
    return &Validator{}
}

// ValidateResyncSQN は再同期時のSQN妥当性を検証する
// 
// 検証条件（3GPP TS 33.102 C.3.2 Profile 2）:
// 1. SQN_MS > SQN_HE（端末のSQNがネットワークより進んでいる）
// 2. SQN_MS - SQN_HE <= Δ（差がデルタ以内）
func (v *Validator) ValidateResyncSQN(sqnMS, sqnHE uint64) error {
    // 条件1: SQN_MS > SQN_HE
    if sqnMS <= sqnHE {
        return fmt.Errorf("SQN_MS (%d) must be greater than SQN_HE (%d)", sqnMS, sqnHE)
    }
    
    // 条件2: 差がΔ以内
    diff := sqnMS - sqnHE
    if diff > Delta {
        return fmt.Errorf("SQN difference exceeds delta: %d > %d", diff, Delta)
    }
    
    return nil
}

// ComputeResyncSQN は再同期後の新しいSQNを計算する
// 端末のSQN_MSを基準に、SEQを+1する
func (v *Validator) ComputeResyncSQN(sqnMS uint64) (uint64, error) {
    newSQN := sqnMS + IncrementStep
    
    if newSQN > MaxSQN {
        return 0, fmt.Errorf("SQN overflow after resync")
    }
    
    return newSQN, nil
}
```

### 7.4 再同期時のSQN更新

再同期プロセスでは、端末から受信したSQN_MSを基準に新しいSQNを設定する。

**処理フロー:**

```
1. AUTSから SQN_MS を抽出（MAC-S検証含む）
2. ValidateResyncSQN() で妥当性検証
3. ComputeResyncSQN() で新SQN計算（SQN_MS + 32）
4. 新SQNでベクター生成
5. Valkeyに新SQN保存
```

**注記:** +32方式は再同期プロセスにも適用され、端末のSQN_MSのIND部分を維持する。これにより端末とネットワーク間のIND同期が保たれる。

### 7.5 SQN競合制御の検討

以下の3方式を検討していたが、1. の **WATCH/MULTIによるCAS（Compare-And-Swap）方式** を採用する。

1. **楽観的ロック**: WATCHコマンドによるCAS操作
2. **分散ロック**: Redlock等による排他制御
3. **INDベース完全分離**: リクエストソース毎にINDを割り当て、カウンタを完全に独立管理

方式1の詳細については、セクション13.6: SQN競合制御 を参照すること。

---

## ■セクション8: Valkeyデータ操作

### 8.1 Valkeyクライアント初期化

（セクション3.4で定義済み）

### 8.2 加入者データアクセス

```go
// internal/store/subscriber.go

type Subscriber struct {
    IMSI string
    Ki   string // Hex 32桁
    OPc  string // Hex 32桁
    AMF  string // Hex 4桁
    SQN  string // Hex 12桁
}

type SubscriberStore struct {
    client *ValkeyClient
}

func NewSubscriberStore(client *ValkeyClient) *SubscriberStore {
    return &SubscriberStore{client: client}
}

// Get は加入者情報を取得する
// キー: sub:{IMSI}
func (s *SubscriberStore) Get(ctx context.Context, imsi string) (*Subscriber, error) {
    key := "sub:" + imsi
    
    result, err := s.client.client.HGetAll(ctx, key).Result()
    if err != nil {
        return nil, fmt.Errorf("failed to get subscriber: %w", err)
    }
    
    if len(result) == 0 {
        return nil, nil // 未登録
    }
    
    return &Subscriber{
        IMSI: imsi,
        Ki:   result["ki"],
        OPc:  result["opc"],
        AMF:  result["amf"],
        SQN:  result["sqn"],
    }, nil
}

// UpdateSQN は加入者のSQNを更新する
func (s *SubscriberStore) UpdateSQN(ctx context.Context, imsi string, sqn string) error {
    key := "sub:" + imsi
    
    err := s.client.client.HSet(ctx, key, "sqn", sqn).Err()
    if err != nil {
        return fmt.Errorf("failed to update SQN: %w", err)
    }
    
    return nil
}
```

### 8.3 リトライ処理

```go
// internal/store/subscriber.go (リトライ付き)

const (
    maxRetries    = 2
    retryInterval = 100 * time.Millisecond
)

func (s *SubscriberStore) GetWithRetry(ctx context.Context, imsi string) (*Subscriber, error) {
    var lastErr error
    
    for i := 0; i <= maxRetries; i++ {
        sub, err := s.Get(ctx, imsi)
        if err == nil {
            return sub, nil
        }
        
        lastErr = err
        
        // 接続エラーの場合のみリトライ
        if !isConnectionError(err) {
            return nil, err
        }
        
        if i < maxRetries {
            slog.Warn("Valkey connection failed, retrying",
                "event_id", "VALKEY_CONN_ERR",
                "retry", i+1,
                "error", err.Error(),
            )
            time.Sleep(retryInterval)
        }
    }
    
    return nil, lastErr
}

func isConnectionError(err error) bool {
    // go-redisの接続エラーを判定
    return err != nil && (
        strings.Contains(err.Error(), "connection refused") ||
        strings.Contains(err.Error(), "i/o timeout") ||
        strings.Contains(err.Error(), "connection reset"))
}
```

### 8.4 データアクセスフロー

#### 通常フロー

```
1. HGETALL sub:{IMSI}
   └─ Ki, OPc, AMF, SQN を取得

2. SQNインクリメント（メモリ上）
   └─ new_sqn = current_sqn + 32

3. Milenage計算
   └─ new_sqn で RAND, AUTN, XRES, CK, IK を生成

4. HSET sub:{IMSI} sqn {new_sqn}
   └─ 新しいSQNを保存

5. レスポンス返却
```

#### 再同期フロー

```
1. HGETALL sub:{IMSI}
   └─ Ki, OPc, AMF, SQN を取得

2. AUTS処理
   └─ SQN_MS を抽出（MAC-S検証含む）

3. デルタ検証
   └─ SQN_MS と SQN_HE の差を検証

4. SQN計算
   └─ new_sqn = sqn_ms + 32

5. Milenage計算
   └─ new_sqn で RAND, AUTN, XRES, CK, IK を生成

6. HSET sub:{IMSI} sqn {new_sqn}
   └─ 新しいSQNを保存

7. レスポンス返却
```

---

## ■セクション9: エラーハンドリング

### 9.1 エラー分類

D-06で定義されたエラー分類に基づく。

| カテゴリ | HTTPステータス | event_id | ログレベル |
|---------|---------------|----------|-----------|
| IMSI未登録 | 404 Not Found | `CALC_ERR` | INFO |
| IMSIフォーマット不正 | 400 Bad Request | `CALC_ERR` | WARN |
| AUTS MAC検証失敗 | 400 Bad Request | `SQN_RESYNC_MAC_ERR` | WARN |
| AUTS形式不正 | 400 Bad Request | `SQN_RESYNC_FORMAT_ERR` | WARN |
| SQNデルタ超過 | 400 Bad Request | `SQN_RESYNC_DELTA_ERR` | WARN |
| SQNオーバーフロー | 500 Internal Server Error | `SQN_OVERFLOW_ERR` | ERROR |
| Valkey接続失敗 | 500 Internal Server Error | `VALKEY_CONN_ERR` | ERROR |
| Milenage計算エラー | 500 Internal Server Error | `CALC_ERR` | ERROR |

### 9.2 RFC 7807 Problem Details

```go
// internal/dto/error.go

type ProblemDetail struct {
    Type   string `json:"type"`
    Title  string `json:"title"`
    Detail string `json:"detail"`
    Status int    `json:"status"`
}

func NewProblemDetail(status int, title, detail string) *ProblemDetail {
    return &ProblemDetail{
        Type:   "about:blank",
        Title:  title,
        Detail: detail,
        Status: status,
    }
}
```

### 9.3 ユースケースエラー型

```go
// internal/usecase/error.go

import "log/slog"

type ProblemError struct {
    Status  int
    Title   string
    Detail  string
    Message string // ログメッセージ
    EventID string
}

func (e *ProblemError) Error() string {
    return e.Detail
}

func (e *ProblemError) ToProblemDetail() *dto.ProblemDetail {
    return dto.NewProblemDetail(e.Status, e.Title, e.Detail)
}

func (e *ProblemError) LogLevel() slog.Level {
    switch {
    case e.Status >= 500:
        return slog.LevelError
    case e.Status == 404:
        return slog.LevelInfo
    default:
        return slog.LevelWarn
    }
}

// 定義済みエラー
var (
    ErrSubscriberNotFound = &ProblemError{
        Status:  404,
        Title:   "User Not Found",
        Detail:  "IMSI does not exist in subscriber DB",
        Message: "subscriber not found",
        EventID: "CALC_ERR",
    }
    
    ErrInvalidIMSI = &ProblemError{
        Status:  400,
        Title:   "Bad Request",
        Detail:  "IMSI must be 15 digits",
        Message: "invalid IMSI format",
        EventID: "CALC_ERR",
    }
    
    ErrResyncMACFailed = &ProblemError{
        Status:  400,
        Title:   "Bad Request",
        Detail:  "AUTS MAC verification failed",
        Message: "AUTS MAC verification failed",
        EventID: "SQN_RESYNC_MAC_ERR",
    }
    
    ErrResyncInvalidFormat = &ProblemError{
        Status:  400,
        Title:   "Bad Request",
        Detail:  "Invalid AUTS format",
        Message: "invalid AUTS format",
        EventID: "SQN_RESYNC_FORMAT_ERR",
    }
    
    ErrResyncDeltaExceeded = &ProblemError{
        Status:  400,
        Title:   "Bad Request",
        Detail:  "SQN difference exceeds allowed range",
        Message: "SQN delta exceeded",
        EventID: "SQN_RESYNC_DELTA_ERR",
    }
    
    ErrSQNOverflow = &ProblemError{
        Status:  500,
        Title:   "Internal Server Error",
        Detail:  "Sequence number overflow",
        Message: "SQN overflow",
        EventID: "SQN_OVERFLOW_ERR",
    }
    
    ErrValkeyConnection = &ProblemError{
        Status:  500,
        Title:   "Internal Server Error",
        Detail:  "Database connection error",
        Message: "Valkey connection error",
        EventID: "VALKEY_CONN_ERR",
    }
    
    ErrMilenageCalculation = &ProblemError{
        Status:  500,
        Title:   "Internal Server Error",
        Detail:  "Authentication vector calculation failed",
        Message: "Milenage calculation error",
        EventID: "CALC_ERR",
    }
)
```

### 9.4 Valkey障害時の動作

D-06に基づく障害時動作:

| 障害種別 | 検出条件 | 対処 | HTTP応答 |
|---------|---------|------|---------|
| 接続失敗 | TCP接続エラー | リトライ（2回） | 500 Internal Server Error |
| コマンドタイムアウト | 応答なし（2秒超過） | リトライ | 500 Internal Server Error |

---

## ■セクション10: ユースケース実装

### 10.1 VectorUseCase

```go
// internal/usecase/vector.go

type VectorUseCase struct {
    subscriberStore    SubscriberRepository
    calculator         MilenageCalculator
    sqnManager         *sqn.Manager
    sqnValidator       *sqn.Validator
    resyncProcessor    ResyncProcessor
    testVectorProvider TestVectorProvider // nilの場合はテストモード無効
    cfg                *config.Config
}

func NewVectorUseCase(
    subscriberStore SubscriberRepository,
    calculator MilenageCalculator,
    sqnManager *sqn.Manager,
    sqnValidator *sqn.Validator,
    testVectorProvider TestVectorProvider,
    cfg *config.Config,
) *VectorUseCase {
    return &VectorUseCase{
        subscriberStore:    subscriberStore,
        calculator:         calculator,
        sqnManager:         sqnManager,
        sqnValidator:       sqnValidator,
        resyncProcessor:    milenage.NewResyncProcessor(),
        testVectorProvider: testVectorProvider,
        cfg:                cfg,
    }
}

func (u *VectorUseCase) GenerateVector(ctx context.Context, req *dto.VectorRequest) (*dto.VectorResponse, error) {
    // 0. テストモード判定（有効な場合）
    if u.testVectorProvider != nil && u.testVectorProvider.IsTestIMSI(req.IMSI) {
        return u.generateTestVector(req.IMSI)
    }
    
    // 1. 加入者情報取得
    sub, err := u.subscriberStore.Get(ctx, req.IMSI)
    if err != nil {
        return nil, fmt.Errorf("%w: %v", ErrValkeyConnection, err)
    }
    if sub == nil {
        return nil, ErrSubscriberNotFound
    }
    
    // 2. 鍵情報をバイト列に変換
    ki, err := milenage.HexDecode(sub.Ki)
    if err != nil {
        return nil, fmt.Errorf("invalid Ki format: %w", err)
    }
    opc, err := milenage.HexDecode(sub.OPc)
    if err != nil {
        return nil, fmt.Errorf("invalid OPc format: %w", err)
    }
    amf, err := milenage.HexDecode(sub.AMF)
    if err != nil {
        return nil, fmt.Errorf("invalid AMF format: %w", err)
    }
    currentSQN, err := u.sqnManager.ParseHex(sub.SQN)
    if err != nil {
        return nil, fmt.Errorf("invalid SQN format: %w", err)
    }
    
    var newSQN uint64
    
    // 3. 再同期処理 or 通常処理
    if req.ResyncInfo != nil {
        newSQN, err = u.processResync(ki, opc, req.ResyncInfo, currentSQN)
        if err != nil {
            return nil, err
        }
    } else {
        newSQN, err = u.sqnManager.Increment(currentSQN)
        if err != nil {
            return nil, ErrSQNOverflow
        }
    }
    
    // 4. ベクター生成
    vector, err := u.calculator.GenerateVector(ki, opc, amf, newSQN)
    if err != nil {
        return nil, fmt.Errorf("%w: %v", ErrMilenageCalculation, err)
    }
    
    // 5. SQN更新
    newSQNHex := u.sqnManager.FormatHex(newSQN)
    if err := u.subscriberStore.UpdateSQN(ctx, req.IMSI, newSQNHex); err != nil {
        return nil, fmt.Errorf("%w: %v", ErrValkeyConnection, err)
    }
    
    // 6. レスポンス変換
    return milenage.VectorToResponse(vector), nil
}

func (u *VectorUseCase) processResync(ki, opc []byte, resyncInfo *dto.ResyncInfo, currentSQN uint64) (uint64, error) {
    // 1. RAND/AUTS をバイト列に変換
    rand, err := milenage.HexDecode(resyncInfo.RAND)
    if err != nil {
        return 0, fmt.Errorf("%w: invalid RAND format", ErrResyncInvalidFormat)
    }
    auts, err := milenage.HexDecode(resyncInfo.AUTS)
    if err != nil {
        return 0, fmt.Errorf("%w: invalid AUTS format", ErrResyncInvalidFormat)
    }
    
    // 2. AUTS長検証
    if len(auts) != 14 {
        return 0, ErrResyncInvalidFormat
    }
    
    // 3. SQN_MS抽出
    sqnMS, err := u.resyncProcessor.ExtractSQN(ki, opc, rand, auts)
    if err != nil {
        // MAC検証失敗
        return 0, ErrResyncMACFailed
    }
    
    // 4. デルタ検証
    if err := u.sqnValidator.ValidateResyncSQN(sqnMS, currentSQN); err != nil {
        slog.Warn("SQN delta validation failed",
            "event_id", "SQN_RESYNC_DELTA_ERR",
            "sqn_ms", fmt.Sprintf("%012x", sqnMS),
            "sqn_he", fmt.Sprintf("%012x", currentSQN),
            "error", err.Error(),
        )
        return 0, ErrResyncDeltaExceeded
    }
    
    // 5. 新SQN計算（SQN_MS + 32）
    newSQN, err := u.sqnValidator.ComputeResyncSQN(sqnMS)
    if err != nil {
        return 0, ErrSQNOverflow
    }
    
    // 6. SQN再同期成功ログ
    slog.Info("SQN resync successful",
        "event_id", "SQN_RESYNC",
        "sqn_old", fmt.Sprintf("%012x", currentSQN),
        "sqn_ms", fmt.Sprintf("%012x", sqnMS),
        "sqn_new", fmt.Sprintf("%012x", newSQN),
    )
    
    return newSQN, nil
}

// generateTestVector はテストモード用の固定ベクターを生成する
func (u *VectorUseCase) generateTestVector(imsi string) (*dto.VectorResponse, error) {
    vector, err := u.testVectorProvider.GetTestVector(imsi)
    if err != nil {
        return nil, fmt.Errorf("failed to generate test vector: %w", err)
    }
    
    slog.Info("test vector generated",
        "event_id", "CALC_OK",
        "test_mode", true,
    )
    
    return milenage.VectorToResponse(vector), nil
}
```

---

## ■セクション11: ログ出力

### 11.1 event_id一覧

D-04で定義されたVector API用event_id:

| event_id | レベル | 説明 |
|----------|--------|------|
| `CALC_OK` | INFO | ベクター生成成功 |
| `CALC_ERR` | INFO/WARN/ERROR | 計算・データエラー |
| `SQN_RESYNC` | INFO | SQN再同期成功 |
| `SQN_RESYNC_MAC_ERR` | WARN | AUTS MAC検証失敗 |
| `SQN_RESYNC_FORMAT_ERR` | WARN | AUTS形式不正 |
| `SQN_RESYNC_DELTA_ERR` | WARN | SQNデルタ超過 |
| `SQN_RESYNC_DECODE_ERR` | WARN | SQN抽出失敗 |
| `SQN_OVERFLOW_ERR` | ERROR | SQNオーバーフロー |
| `VALKEY_CONN_ERR` | ERROR | Valkey接続失敗 |
| `VALKEY_CONN_RESTORED` | INFO | Valkey接続復旧 |

### 11.2 ログ出力例

#### 成功時

```json
{
  "time": "2026-01-14T10:00:00.123Z",
  "level": "INFO",
  "app": "vector-api",
  "msg": "vector generated",
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "event_id": "CALC_OK",
  "imsi": "440101********0",
  "method": "POST",
  "path": "/api/v1/vector",
  "latency_ms": 15,
  "http_status": 200
}
```

#### IMSI未登録時

```json
{
  "time": "2026-01-14T10:00:00.456Z",
  "level": "INFO",
  "app": "vector-api",
  "msg": "subscriber not found",
  "trace_id": "550e8400-e29b-41d4-a716-446655440001",
  "event_id": "CALC_ERR",
  "imsi": "440109********0",
  "method": "POST",
  "path": "/api/v1/vector",
  "http_status": 404
}
```

#### 再同期成功時

```json
{
  "time": "2026-01-14T10:00:00.789Z",
  "level": "INFO",
  "app": "vector-api",
  "msg": "SQN resync successful",
  "trace_id": "550e8400-e29b-41d4-a716-446655440002",
  "event_id": "SQN_RESYNC",
  "sqn_old": "000000000020",
  "sqn_ms": "000000000060",
  "sqn_new": "000000000080"
}
```

#### デルタ超過時

```json
{
  "time": "2026-01-14T10:00:01.123Z",
  "level": "WARN",
  "app": "vector-api",
  "msg": "SQN delta validation failed",
  "trace_id": "550e8400-e29b-41d4-a716-446655440003",
  "event_id": "SQN_RESYNC_DELTA_ERR",
  "sqn_ms": "100000000000",
  "sqn_he": "000000000020",
  "error": "SQN difference exceeds delta"
}
```

### 11.3 IMSIマスキング設定

環境変数 `LOG_MASK_IMSI` によりマスキングのON/OFFを制御する。

| 設定値 | 動作 | 出力例 |
|--------|------|--------|
| `true`（デフォルト） | マスキング有効 | `440101********0` |
| `false` | マスキング無効 | `440101234567890` |

**用途:**
- 本番環境: `LOG_MASK_IMSI=true`（プライバシー保護）
- 開発・デバッグ環境: `LOG_MASK_IMSI=false`（問題調査用）

---

## ■セクション12: 主要構造体・インターフェース一覧

### 12.1 DTO定義

```go
// --- Request/Response DTOs ---

type VectorRequest struct {
    IMSI       string      `json:"imsi" binding:"required"`
    ResyncInfo *ResyncInfo `json:"resync_info,omitempty"`
}

type ResyncInfo struct {
    RAND string `json:"rand" binding:"required"`
    AUTS string `json:"auts" binding:"required"`
}

type VectorResponse struct {
    RAND string `json:"rand"`
    AUTN string `json:"autn"`
    XRES string `json:"xres"`
    CK   string `json:"ck"`
    IK   string `json:"ik"`
}

type ProblemDetail struct {
    Type   string `json:"type"`
    Title  string `json:"title"`
    Detail string `json:"detail"`
    Status int    `json:"status"`
}
```

### 12.2 ドメインモデル

```go
// --- Domain Models ---

type Subscriber struct {
    IMSI string
    Ki   string // Hex 32桁
    OPc  string // Hex 32桁
    AMF  string // Hex 4桁
    SQN  string // Hex 12桁
}

type Vector struct {
    RAND []byte // 16 bytes
    AUTN []byte // 16 bytes
    XRES []byte // 4-16 bytes
    CK   []byte // 16 bytes
    IK   []byte // 16 bytes
}
```

### 12.3 インターフェース

```go
// --- Interfaces ---

type MilenageCalculator interface {
    GenerateVector(ki, opc, amf []byte, sqn uint64) (*Vector, error)
}

type ResyncProcessor interface {
    ExtractSQN(ki, opc, rand, auts []byte) (uint64, error)
}

type SQNManager interface {
    Increment(currentSQN uint64) (uint64, error)
    FormatHex(sqn uint64) string
    ParseHex(s string) (uint64, error)
    GetSEQ(sqn uint64) uint64
    GetIND(sqn uint64) uint8
}

type SQNValidator interface {
    ValidateResyncSQN(sqnMS, sqnHE uint64) error
    ComputeResyncSQN(sqnMS uint64) (uint64, error)
}

type SubscriberRepository interface {
    Get(ctx context.Context, imsi string) (*Subscriber, error)
    UpdateSQN(ctx context.Context, imsi string, sqn string) error
}

type TestVectorProvider interface {
    IsTestIMSI(imsi string) bool
    GetTestVector(imsi string) (*Vector, error)
}

type VectorUseCase interface {
    GenerateVector(ctx context.Context, req *VectorRequest) (*VectorResponse, error)
}
```

---

## ■セクション13: 設計上の考慮事項

### 13.1 セキュリティ考慮

| 項目 | 対策 |
|------|------|
| 鍵情報の保護 | Ki/OPc はログに出力しない |
| IMSIの保護 | ログ出力時はマスキング（環境変数で制御可能） |
| 通信経路 | Docker内部ネットワーク（外部非公開） |
| エラーメッセージ | 内部詳細を外部に漏らさない |

**IMSIマスキング設定:**

| 環境変数 | デフォルト | 説明 |
|---------|-----------|------|
| `LOG_MASK_IMSI` | `true` | `false` でマスキング無効化（デバッグ用） |

### 13.2 パフォーマンス考慮

| 項目 | 設計 |
|------|------|
| Valkey接続 | 接続プール利用（PoolSize: 10） |
| タイムアウト | Read/Write: 2秒、Dial: 3秒 |
| RAND生成 | crypto/rand（暗号学的に安全） |

### 13.3 PoC制限事項

| 項目 | 制限 | 将来対応 |
|------|------|---------|
| 認証ベクター長 | XRES固定長（8バイト） | 可変長対応 |
| インクリメントパターン | IND固定・SEQのみインクリメント | IMSI単位でパターン指定可能 |

### 13.4 テスト戦略

| レイヤー | テスト方法 |
|---------|-----------|
| `handler` | Ginテストフレームワーク、モックusecase |
| `usecase` | 単体テスト、モックrepository/calculator |
| `milenage` | 3GPP TS 35.208テストベクター |
| `sqn` | 単体テスト（インクリメント、デルタ検証） |
| `store` | miniredisによるインメモリテスト |
| `E2E` | eapaka_testによる統合テスト |

### 13.5 テストモード実装（E2Eテスト対応）

#### 概要

E2Eテスト（eapaka_test等）で使用する固定ベクターを返却するテストモードを実装する。
**方式A + 環境変数ガード** を採用し、二重ガードで本番環境での誤発動を防止する。

#### 環境変数

| 環境変数 | デフォルト | 説明 |
|---------|-----------|------|
| `TEST_VECTOR_ENABLED` | `false` | テストベクターモード有効化 |
| `TEST_VECTOR_IMSI_PREFIX` | `00101` | テスト対象IMSIプレフィックス（5-6桁） |

#### 判定ロジック

```
1. TEST_VECTOR_ENABLED=true か確認
   └─ false → 通常処理（Valkey参照）

2. IMSIプレフィックスが TEST_VECTOR_IMSI_PREFIX に一致か
   └─ 不一致 → 通常処理（Valkey参照）

3. 一致 → 3GPP TS 35.208 テストベクターを返却
```

#### テストベクター実装

```go
// internal/testmode/testvector.go

// 3GPP TS 35.208 Test Set 1 ベースのテストベクター
var testSet1 = struct {
    Ki   []byte
    OPc  []byte
    RAND []byte
    SQN  uint64
    AMF  []byte
    XRES []byte
    CK   []byte
    IK   []byte
}{
    Ki:   hexMustDecode("465b5ce8b199b49faa5f0a2ee238a6bc"),
    OPc:  hexMustDecode("cd63cb71954a9f4e48a5994e37a02baf"),
    RAND: hexMustDecode("23553cbe9637a89d218ae64dae47bf35"),
    SQN:  0xff9bb4d0b607,
    AMF:  hexMustDecode("b9b9"),
    XRES: hexMustDecode("a54211d5e3ba50bf"),
    CK:   hexMustDecode("b40ba9a3c58b2a05bbf0d987b21bf8cb"),
    IK:   hexMustDecode("f769bcd751044604127672711c6d3441"),
}

type TestVectorProvider struct {
    enabled    bool
    imsiPrefix string
}

func NewTestVectorProvider(cfg *config.Config) *TestVectorProvider {
    if !cfg.TestVectorEnabled {
        return nil
    }
    return &TestVectorProvider{
        enabled:    cfg.TestVectorEnabled,
        imsiPrefix: cfg.TestVectorIMSIPrefix,
    }
}

func (p *TestVectorProvider) IsTestIMSI(imsi string) bool {
    if p == nil || !p.enabled {
        return false
    }
    return strings.HasPrefix(imsi, p.imsiPrefix)
}

func (p *TestVectorProvider) GetTestVector(imsi string) (*milenage.Vector, error) {
    // 固定RANDでAUTNを計算
    calc := milenage.NewCalculator()
    
    // テスト用に固定値で計算（SQNはインクリメントしない）
    vector := &milenage.Vector{
        RAND: testSet1.RAND,
        XRES: testSet1.XRES,
        CK:   testSet1.CK,
        IK:   testSet1.IK,
    }
    
    // AUTNを計算
    autn := calc.ComputeAUTNFromTestData(testSet1.SQN, testSet1.AMF)
    vector.AUTN = autn
    
    return vector, nil
}

func hexMustDecode(s string) []byte {
    b, err := hex.DecodeString(s)
    if err != nil {
        panic(err)
    }
    return b
}
```

#### eapaka_test 用テストケース例

```yaml
# testdata/cases/e2e_aka_success.yaml
version: 1
name: e2e_aka_success
identity: "0001010000000001@wlan.mnc001.mcc001.3gppnetwork.org"
radius:
  attributes:
    called_station_id: "aa-bb-cc-dd-ee-ff:TestSSID"
sqn:
  reset: true
expect:
  result: accept
  mppe:
    require_present: true
trace:
  level: verbose
```

#### 注意事項

- **本番環境**: `TEST_VECTOR_ENABLED=false`（デフォルト）で運用
- **テスト環境**: `TEST_VECTOR_ENABLED=true` + `TEST_VECTOR_IMSI_PREFIX=00101` を設定
- **SQN管理**: テストモードではSQNをインクリメント・永続化しない（固定値返却）
- **再同期テスト**: テストモードでは再同期リクエストもテストベクターを返却（実際の再同期処理はスキップ）

#### 13.5.1 本番環境でのテストベクターモード無効化

テストベクターモードは開発・テスト環境でのみ使用する機能であり、本番環境では**必ず無効化**すること。

**本番 `.env` ファイルでの設定:**

```bash
# テストベクターモードは本番環境で絶対に有効にしないこと
# 以下のいずれかの方法で無効化を保証する:
#
# 方法1: 環境変数を設定しない（デフォルトでfalse）
# 方法2: 明示的にfalseを設定
TEST_VECTOR_ENABLED=false
```

**無効化が必要な理由:**

| 項目 | 説明 |
|------|------|
| セキュリティリスク | 固定の認証ベクター（3GPP TS 35.208 Test Set 1）が返却され、攻撃者が認証を突破可能 |
| データ整合性 | Valkey上の正規加入者データが使用されず、不正な認証が成立する |
| 監査問題 | 本来の加入者認証が行われないため、監査ログの信頼性が損なわれる |

**確認方法:**

```bash
# 起動時ログで確認（TEST_VECTOR_ENABLEDがfalseまたは未出力であること）
docker compose logs vector-api | grep TEST_VECTOR
```

> **警告:** 本番環境で `TEST_VECTOR_ENABLED=true` が設定されている場合、**即座に無効化し、セキュリティインシデントとして調査**すること。

---

### 13.6 SQN競合制御

#### 13.6.1 概要

同一IMSIへの並行Access-Request（リトライ/再送含む）でSQNが巻き戻る/飛ぶリスクを回避するため、WATCH/MULTIによるCAS（Compare-And-Swap）方式を採用する。

#### 13.6.2 方式詳細

| 項目           | 内容                              |
| -------------- | --------------------------------- |
| 方式           | Valkey WATCH/MULTI/EXEC によるCAS |
| 対象キー       | `sub:{IMSI}`                      |
| 対象フィールド | `sqn`                             |
| リトライ上限   | 3回                               |
| 競合検出時動作 | EXEC失敗 → リトライ               |
| 上限超過時動作 | HTTP 409 Conflict 返却            |

#### 13.6.3 処理フロー

1. WATCH sub:{IMSI}
2. HGETALL sub:{IMSI} → Ki/OPc/AMF/SQN取得
3. 新SQN算出（通常: +32、再同期: SQN_MS + 32）
4. MULTI
5. HSET sub:{IMSI} sqn {新SQN}
6. EXEC
    ├─ 成功 → ベクター生成・応答
    └─ 失敗（競合検出）
        ├─ リトライ < 上限 → 手順1へ
        └─ リトライ >= 上限 → 409 Conflict

#### 13.6.4 エラー応答（競合上限超過時）

```json
{
  "type": "about:blank",
  "title": "Conflict",
  "detail": "SQN update conflict after 3 retries for IMSI 44010*****890",
  "status": 409
}
```

#### 13.6.5 実装例

```go
// internal/store/subscriber.go

const maxSQNRetries = 3

var ErrSQNConflict = errors.New("sqn update conflict")

func (s *SubscriberStore) UpdateSQNWithCAS(
    ctx context.Context,
    imsi string,
    computeNewSQN func(current uint64) (uint64, error),
) (uint64, error) {
    key := "sub:" + imsi

    for attempt := 1; attempt <= maxSQNRetries; attempt++ {
        err := s.client.Watch(ctx, func(tx *redis.Tx) error {
            // 1. 現在のSQN取得
            sqnHex, err := tx.HGet(ctx, key, "sqn").Result()
            if err != nil {
                if err == redis.Nil {
                    return ErrSubscriberNotFound
                }
                return fmt.Errorf("failed to get sqn: %w", err)
            }

            currentSQN, err := parseSQNHex(sqnHex)
            if err != nil {
                return fmt.Errorf("invalid sqn format: %w", err)
            }

            // 2. 新SQN算出
            newSQN, err := computeNewSQN(currentSQN)
            if err != nil {
                return err
            }

            // 3. トランザクション実行
            _, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
                pipe.HSet(ctx, key, "sqn", formatSQNHex(newSQN))
                return nil
            })
            return err
        }, key)

        if err == nil {
            return newSQN, nil
        }

        if errors.Is(err, redis.TxFailedErr) {
            // 競合検出 → リトライ
            slog.Warn("SQN update conflict, retrying",
                "event_id", "SQN_CONFLICT_RETRY",
                "imsi", maskIMSI(imsi),
                "attempt", attempt,
            )
            if attempt < maxSQNRetries {
                continue
            }
            // リトライ上限超過
            slog.Warn("SQN conflict exceeded retry limit",
                "event_id", "SQN_CONFLICT_ERR",
                "imsi", maskIMSI(imsi),
                "retry_count", maxSQNRetries,
            )
            return 0, ErrSQNConflict
        }

        // その他のエラー
        return 0, err
    }

    return 0, ErrSQNConflict
}

func parseSQNHex(s string) (uint64, error) {
    if len(s) != 12 {
        return 0, fmt.Errorf("expected 12 hex chars, got %d", len(s))
    }
    b, err := hex.DecodeString(s)
    if err != nil {
        return 0, err
    }
    var v uint64
    for _, c := range b {
        v = (v << 8) | uint64(c)
    }
    return v, nil
}

func formatSQNHex(v uint64) string {
    return fmt.Sprintf("%012x", v)
}
```

#### 13.6.6 ユースケース層の変更

```go
// internal/usecase/vector.go

func (u *VectorUseCase) GenerateVector(ctx context.Context, req *dto.VectorRequest) (*dto.VectorResponse, error) {
    // ... 省略（テストモード判定等）
    
    // 加入者情報取得（SQN以外）
    sub, err := u.subscriberStore.Get(ctx, req.IMSI)
    if err != nil {
        return nil, fmt.Errorf("%w: %v", ErrValkeyConnection, err)
    }
    if sub == nil {
        return nil, ErrSubscriberNotFound
    }
    
    // 鍵情報をバイト列に変換
    ki, opc, amf, err := u.parseKeyMaterial(sub)
    if err != nil {
        return nil, err
    }

    // SQN更新（CAS方式）
    var newSQN uint64
    var vector *milenage.Vector
    
    newSQN, err = u.subscriberStore.UpdateSQNWithCAS(ctx, req.IMSI, func(currentSQN uint64) (uint64, error) {
        if req.ResyncInfo != nil {
            return u.processResync(ki, opc, req.ResyncInfo, currentSQN)
        }
        return u.sqnManager.Increment(currentSQN)
    })
    
    if errors.Is(err, store.ErrSQNConflict) {
        return nil, ErrSQNConflict
    }
    if err != nil {
        return nil, fmt.Errorf("%w: %v", ErrValkeyConnection, err)
    }
    
    // ベクター生成
    vector, err = u.calculator.GenerateVector(ki, opc, amf, newSQN)
    if err != nil {
        return nil, fmt.Errorf("%w: %v", ErrMilenageCalculation, err)
    }
    
    return milenage.VectorToResponse(vector), nil
}
```

#### 13.6.7 エラー定義追加

```go
// internal/usecase/errors.go

var (
    // ... 既存エラー
    ErrSQNConflict = errors.New("sqn update conflict after max retries")
)
```

#### 13.6.8 ハンドラー層のエラーマッピング

```go
// internal/handler/vector.go

func (h *VectorHandler) HandleVector(c *gin.Context) {
    // ... 省略
    
    resp, err := h.usecase.GenerateVector(c.Request.Context(), req)
    if err != nil {
        switch {
        case errors.Is(err, usecase.ErrSubscriberNotFound):
            h.writeError(c, http.StatusNotFound, "User Not Found", err.Error())
        case errors.Is(err, usecase.ErrSQNConflict):
            h.writeError(c, http.StatusConflict, "Conflict",
                fmt.Sprintf("SQN update conflict after %d retries for IMSI %s",
                    store.MaxSQNRetries, maskIMSI(req.IMSI)))
        // ... その他のエラー処理
        }
        return
    }
    
    c.JSON(http.StatusOK, resp)
}
```

## 改訂履歴

| 版数 | 日付 | 内容 |
|------|------|------|
| r1 | 2026-01-14 | 初版作成 |
| r2 | 2026-01-17 | SQNインクリメント方式変更（+32方式）、デルタ検証追加、AMF取得方式明確化、IMSIマスキング環境変数対応、E2Eテストモード追加 |
| r3 | 2026-01-26 | SQN競合制御追加: セクション13.3の「SQN競合制御: 簡易実装」を削除、セクション13.6新設（WATCH/MULTIによるCAS方式、リトライ上限3回、HTTP 409エラー）、実装例・エラー定義追加、これらに伴うセクション7.5の記載変更 |
| r4 | 2026-01-26 | インフラ基盤統一: セクション2.6新設（Dockerfile方針 - ベースイメージdebian:bookworm-slim、curl/ca-certificates導入、ヘルスチェックcurl -fsS）。これに伴い、旧 2.6 ファイル別責務詳細 のセクション番号を 2.7 に移行 |
| r5 | 2026-01-27 | 本番環境注記追加: セクション13.5.1にテストベクターモードの本番無効化要件を新設、関連ドキュメント参照バージョン更新、D-08への参照追加 |
| r6 | 2026-02-18 | ディレクトリ構造全面更新、usecase統合反映、関連ドキュメント版数更新 |
