# D-11 Vector API詳細設計書 (r11)

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
| D-01 | ミニPC版設計仕様書 (r10) | システム構成、パッケージ利用マップ |
| D-02 | Valkeyデータ設計仕様書 (r19) | 加入者データ構造、キー設計、Go構造体、SQN更新方式（Lua による比較・置き換え） |
| D-03 | Vector-API/ステートマシン設計書 (r8) | API仕様、リクエスト/レスポンス定義、409 Conflict |
| D-04 | ログ仕様設計書 (r29) | event_id定義、ログフォーマット |
| D-09 | Auth Server詳細設計書 (r10) | Auth Server連携仕様 |
| D-06 | エラーハンドリング詳細設計書 (r15) | エラー分類、タイムアウト設定、SQN競合エラー（409） |
| D-07 | Admin TUI詳細設計書【後半】 (r8) | 管理用TUIアプリケーション仕様 |
| D-08 | インフラ設定・運用設計書 (r14) | 環境変数設定、テストベクターモード |
| D-12 | Vector Gateway詳細設計書 (r5) | X-Trace-ID伝搬、呼び出し元仕様 |
| E-02 | コーディング規約（簡易版） (r3) | コーディング規約 |
| E-03 | 共通ライブラリ(pkg)設計書 (r7) | 共通ライブラリ（pkg）。`logging.MaskIMSI`, `logging.ParseLevel` |

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
    │   ├── subscriber.go       # 加入者データアクセス、SQNの比較・置き換え（Lua）
    │   ├── subscriber_test.go  # store パッケージテスト（miniredis）
    │   └── valkey.go           # Valkeyクライアント初期化・管理
    ├── testmode/
    │   ├── testvector.go       # テストモード用固定パラメータ（Ki/OPc/AMF）
    │   └── testvector_test.go  # testmode パッケージテスト
    └── usecase/
        ├── error.go            # ユースケースエラー型定義
        ├── error_test.go       # error パッケージテスト
        ├── interfaces.go       # ユースケース層インターフェース定義
        ├── mock_interfaces.go  # テスト用モックインターフェース
        ├── trace.go            # Trace IDのcontext受け渡し（ContextWithTraceID）
        ├── vector.go           # ベクター生成・再同期ユースケース（統合）、SQN競合時のやり直し
        ├── vector_test.go      # vector パッケージテスト
        ├── vector_cas_test.go  # SQN競合時のやり直し・再同期のテスト
        └── vector_concurrency_test.go # 同一IMSIへの並行リクエストのテスト（miniredis）
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
| `testmode` | テストベクターモード用の固定 Ki/OPc/AMF 提供 | `TestVectorProvider`, `IsTestIMSI()`, `GetTestCryptoParams()` |
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
    // SQNが oldSQN のときだけ newSQN に書き換え、書き換えたかどうかを返す（§13.6）
    CompareAndSetSQN(ctx context.Context, imsi, oldSQN, newSQN string) (bool, error)
}

type TestVectorProvider interface {
    IsTestIMSI(imsi string) bool
    GetTestVector(imsi string) (*Vector, error)
    GetTestCryptoParams() (ki, opc, amf []byte) // テスト用 Ki/OPc/AMF（防御的コピー）
}

// usecase/interfaces.go（ハンドラーが依存するユースケースのインターフェース）
type VectorUseCaseInterface interface {
    GenerateVector(ctx context.Context, req *VectorRequest) (*VectorResponse, error)
    IsTestMode(imsi string) bool // テストベクターモードの対象IMSIか（CALC_OK の test_mode 属性用）
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
| `vector.go` | ベクター生成・再同期ユースケース（統合）、SQN競合時のやり直し | `VectorUseCase`, `GenerateVector()`, `generateOnce()`, `processResync()`, `logResync()`, `waitRandom()`, `IsTestMode()` |
| `interfaces.go` | ユースケース層インターフェース定義 | `MilenageCalculator`, `ResyncProcessor`, `SQNManager`, `SQNValidator`, `SubscriberRepository`, `TestVectorProvider`, `VectorUseCaseInterface` |
| `trace.go` | Trace IDのcontext受け渡し | `ContextWithTraceID()`（ハンドラーが呼ぶ）, `traceIDFromContext()` |
| `error.go` | ユースケースエラー型定義 | `ProblemError`, `ErrSubscriberNotFound`, `ErrResyncMACFailed`, `ErrSQNConflict` 等（IMSI形式不正はハンドラーが直接400を返すため `ErrInvalidIMSI` は定義しない） |
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
| `subscriber.go` | 加入者データアクセス（アプリ独自のリトライなし）、SQNの比較・置き換え（Lua） | `SubscriberStore`, `Get()`, `CompareAndSetSQN()` |

#### `internal/testmode/`

| ファイル | 責務 | 主要関数・型 |
|---------|------|-------------|
| `testvector.go` | テストモード判定、テスト用固定 Ki/OPc/AMF の提供 | `TestVectorProvider`, `IsTestIMSI()`, `GetTestCryptoParams()`, `GetTestVector()`（ユースケースからは未使用） |

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
| `LOG_LEVEL` | No | `INFO` | string | ログレベル（DEBUG/INFO/WARN/ERROR）。`pkg/logging.ParseLevel` で変換（大文字小文字を区別せず前後の空白を無視、`WARNING` も `WARN`、未知の値は `INFO`） |
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
   │      └─ ロガー初期化（JSON形式。LOG_LEVEL は logging.ParseLevel で変換）
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
    useCase usecase.VectorUseCaseInterface
    cfg     *config.Config
}

func NewVectorHandler(useCase usecase.VectorUseCaseInterface, cfg *config.Config) *VectorHandler {
    return &VectorHandler{
        useCase: useCase,
        cfg:     cfg,
    }
}

func (h *VectorHandler) HandleVector(c *gin.Context) {
    traceID, _ := c.Get(TraceIDKey)
    // Trace IDをcontextに載せ、ユースケース層のログ（SQN_RESYNC）にも出力させる
    ctx := usecase.ContextWithTraceID(c.Request.Context(), fmt.Sprint(traceID))
    
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
            "imsi", logging.MaskIMSI(req.IMSI, h.cfg.LogMaskIMSI),
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
        h.handleError(c, traceID, req.IMSI, err)
        return
    }
    
    // 4. 成功レスポンス
    slog.Info("vector generated",
        "trace_id", traceID,
        "event_id", "CALC_OK",
        "imsi", logging.MaskIMSI(req.IMSI, h.cfg.LogMaskIMSI),
        "http_status", http.StatusOK,
        "test_mode", h.useCase.IsTestMode(req.IMSI),
    )
    c.JSON(http.StatusOK, resp)
}

func (h *VectorHandler) handleError(c *gin.Context, traceID any, imsi string, err error) {
    var problemErr *usecase.ProblemError
    if errors.As(err, &problemErr) {
        slog.Log(c.Request.Context(), problemErr.LogLevel(), problemErr.Message,
            "trace_id", traceID,
            "event_id", problemErr.EventID,
            "imsi", logging.MaskIMSI(imsi, h.cfg.LogMaskIMSI),
            "http_status", problemErr.Status,
            // 原因（Valkeyのエラー、SQN値など）を含むエラー文。応答の detail には含めない
            "error", err.Error(),
        )
        c.JSON(problemErr.Status, problemErr.ToProblemDetail())
        return
    }
    
    // 予期しないエラー
    slog.Error("unexpected error",
        "trace_id", traceID,
        "event_id", "CALC_ERR",
        "imsi", logging.MaskIMSI(imsi, h.cfg.LogMaskIMSI),
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
```

> **注記（r9）:** IMSIのマスクは共通ライブラリの `logging.MaskIMSI`（E-03。`LOG_MASK_IMSI` に従う）を使う。ProblemError の経路のログ（`handleError`）は `error` 属性に原因を含むエラー文（`err.Error()`）を出力するが、HTTP応答の `detail` は `ToProblemDetail()` による定義済みエラーの `Detail` のみで、原因は含めない（§9.3）。`CALC_OK` の `test_mode` は `IsTestMode()` の結果（テストベクターモードの対象IMSIは `true`）。

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

### 7.5 SQN競合制御

同一IMSIへの並行リクエストで SQN が同値になる・巻き戻るのを防ぐため、SQN の書き戻しを **Lua スクリプトによる `sqn` フィールドの比較・置き換え（CAS: Compare-And-Swap）** で行う。読んだ値から変わっていないときだけ書き換え、変わっていたときは加入者の読み出しからやり直す（最大3回。超過時は 409 Conflict）。詳細はセクション13.6 を参照すること。

検討した方式と採否:

| 方式 | 採否 | 理由 |
|------|------|------|
| Lua スクリプトによる `sqn` の比較・置き換え | **採用（r10）** | 1往復で原子的に比較・更新できる。比較対象が `sqn` だけなので、Admin TUI が他のフィールドを更新しても競合にならない。SQN の計算は Go 側に残せる |
| WATCH/MULTI による楽観的ロック | 不採用（r9 まではこの方式で設計していた） | キー全体を監視するため、`sqn` 以外のフィールドの更新でも競合になる。トランザクション中はコネクションを占有する |
| Lua スクリプトでインクリメントまで行う | 不採用 | SQN の計算（+32、48bit 上限）が Go と Lua に分かれる。再同期では結局 CAS が必要 |
| 分散ロック（Redlock 等）／プロセス内の IMSI ごとのロック | 不採用 | 構成が複雑になる／Vector API が1台であることが前提で、Admin TUI の書き込みを防げない |
| INDベース完全分離 | 不採用 | リクエストソースごとの IND 割り当てが必要で、PoC の運用に合わない |

**守る性質:** 発行するベクターの SQN は IMSI ごとに一意で、単調に増える（同値・巻き戻りを起こさない）。SQN が飛ぶのは許容する（USIM は SEQ が増えていれば受け入れる）。

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

// compareAndSetSQNScript は sqn が期待値と一致するときだけ新しい値に書き換える。
// キーまたは sqn フィールドが無い場合は書き換えず（キーを新たに作らない）、0 を返す。
var compareAndSetSQNScript = redis.NewScript(`
local cur = redis.call('HGET', KEYS[1], 'sqn')
if cur == false or cur ~= ARGV[1] then
  return 0
end
redis.call('HSET', KEYS[1], 'sqn', ARGV[2])
return 1
`)

// CompareAndSetSQN は加入者のSQNが oldSQN と一致するときだけ newSQN に更新する。
// 更新したときは true を返す。一致しない（他のリクエストが先に更新した）ときや、
// 加入者が削除されていたときは false を返す。
// oldSQN には Get で読んだ値をそのまま渡す（大文字小文字を含めて文字列で比較する）。
func (s *SubscriberStore) CompareAndSetSQN(ctx context.Context, imsi, oldSQN, newSQN string) (bool, error) {
    key := "sub:" + imsi

    n, err := compareAndSetSQNScript.Run(ctx, s.client.client, []string{key}, oldSQN, newSQN).Int()
    if err != nil {
        return false, fmt.Errorf("failed to compare and set SQN: %w", err)
    }

    return n == 1, nil
}
```

> **注記（r10）:** r9 まで定義していた単純な書き戻し `UpdateSQN`（`HSET sub:{IMSI} sqn {新SQN}`。後勝ち）は、`CompareAndSetSQN` に置き換えて削除した。`redis.NewScript` の `Run` は EVALSHA で実行し、スクリプトが未ロードなら EVAL にフォールバックする（go-redis の機能）。比較の期待値は `Get` で読んだ `sqn` の生の文字列で、正規化しない（Admin TUI は大文字で保存し、Vector API は小文字で書き戻すため、正規化すると Admin TUI による書き換えを見逃すおそれがある）。

### 8.3 リトライ処理

Valkeyエラー時のアプリ独自のリトライは行わない（go-redis v9 の既定の自動リトライ（`redis.Options` で `MaxRetries` を指定していないため既定の最大3回、バックオフ 8ms〜512ms）は働く）。`Get` / `CompareAndSetSQN` がエラーを返すと、ユースケースは `ErrValkeyConnection` に原因を付けたエラー（`Database connection error: <Valkeyのエラー>`）を返し、ハンドラーが500と `VALKEY_CONN_ERR`（ERROR、`error` 属性に原因）を出力する（§9.4）。接続の張り直しはgo-redisのコネクションプールが次のコマンド実行時に行う。

> **注記（r9）:** r8 まで本節に記載していたリトライ付き取得 `GetWithRetry`（最大2回リトライ、間隔100ms、`isConnectionError` で接続エラーのみリトライ、リトライ時に `VALKEY_CONN_ERR`（WARN、`Valkey connection failed, retrying`）を出力）は、どこからも呼び出されていなかったため実装から削除した。

### 8.4 データアクセスフロー

1回の試行の流れを示す。手順4で競合した（`sqn` が読んだ値から変わっていた）ときは、1〜10msのランダムな時間待ってから手順1からやり直す（最大3回。§13.6）。

#### 通常フロー

```
1. HGETALL sub:{IMSI}
   └─ Ki, OPc, AMF, SQN を取得

2. SQNインクリメント（メモリ上）
   └─ new_sqn = current_sqn + 32

3. （再同期フローの手順のみ）

4. EVALSHA（Lua）: sqn が current_sqn のときだけ HSET sub:{IMSI} sqn {new_sqn}
   ├─ 書き換えた → 5へ
   └─ 変わっていた（競合）→ 待ってから 1 へ（3回目なら 409）

5. Milenage計算
   └─ new_sqn で RAND, AUTN, XRES, CK, IK を生成

6. レスポンス返却
```

#### 再同期フロー

```
1. HGETALL sub:{IMSI}
   └─ Ki, OPc, AMF, SQN を取得

2. AUTS処理
   └─ SQN_MS を抽出（MAC-S検証含む）

3. デルタ検証・SQN計算
   ├─ やり直し（2回目以降の試行）で SQN_HE >= SQN_MS
   │    └─ 同期済みとみなし new_sqn = sqn_he + 32（デルタ検証しない）
   └─ それ以外
        └─ SQN_MS と SQN_HE の差を検証し、new_sqn = sqn_ms + 32

4. EVALSHA（Lua）: sqn が sqn_he のときだけ HSET sub:{IMSI} sqn {new_sqn}
   ├─ 書き換えた → SQN_RESYNC ログを出して 5へ
   └─ 変わっていた（競合）→ 待ってから 1 へ（3回目なら 409）

5. Milenage計算
   └─ new_sqn で RAND, AUTN, XRES, CK, IK を生成

6. レスポンス返却
```

> **注記（r10）:** r9 までは Milenage 計算の後に SQN を書き戻していたが、r10 で SQN の書き換えに成功してからベクターを生成する順序に変更した。書き換えた後に計算が失敗した場合は SQN が飛ぶだけで、端末の検証には影響しない。

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
| SQN更新の競合がやり直しの上限を超過 | 409 Conflict | `SQN_CONFLICT_ERR` | WARN |
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
    
    // ErrSQNConflict は、SQNの書き換えが他のリクエストと競合し、やり直しの上限を超えたことを表す。
    ErrSQNConflict = &ProblemError{
        Status:  409,
        Title:   "Conflict",
        Detail:  "SQN update conflict",
        Message: "SQN update conflict exceeded retry limit",
        EventID: "SQN_CONFLICT_ERR",
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

> **注記（r9）:**
> - IMSI形式不正（15桁の数字でない）はハンドラーが `validateIMSI` で検出して直接400（`CALC_ERR`、msg `invalid IMSI format`）を返すため、ユースケースのエラーとしては定義しない（r8 まで記載していた未使用の `ErrInvalidIMSI` は実装から削除した）。
> - ユースケースは原因があるエラーを `fmt.Errorf("%w: ...", ErrXxx, ...)` でラップして返す。ハンドラーは `errors.As` で `ProblemError` を取り出し、応答は `ToProblemDetail()`（定義済みの `Detail` のみ）、ログの `error` 属性はラップ後のエラー文（`<Detail>: <原因>`）とする。例: `Database connection error: failed to get subscriber: ...`、`SQN difference exceeds allowed range: sqn_ms=xxxxxxxxxxxx sqn_he=xxxxxxxxxxxx: <原因>`。原因のないエラー（`ErrSubscriberNotFound`、`ErrResyncMACFailed`、`ErrSQNOverflow` 等）は `Detail` と同じ文になる。
> - `ErrSQNConflict`（r10 追加）は 409・WARN（`LogLevel()` の既定）で、ハンドラーに専用の分岐はない。応答の `detail` は `SQN update conflict` で、IMSI や原因を含めない（r9 までの設計例 §13.6.4 の detail に含めていた IMSI は含めない）。ログの `error` 属性は `SQN update conflict: conflicted 3 times`、または待っている間に期限が切れた場合の `SQN update conflict: retry aborted after N attempts: context deadline exceeded` 等になる。

### 9.4 Valkey障害時の動作

D-06に基づく障害時動作:

| 障害種別 | 検出条件 | 対処 | HTTP応答 |
|---------|---------|------|---------|
| 接続失敗 | TCP接続エラー | アプリ独自のリトライなし（§8.3） | 500 Internal Server Error（ERROR: `VALKEY_CONN_ERR`。`error` 属性にValkeyのエラー） |
| コマンドタイムアウト | 応答なし（2秒超過） | 同上 | 500 Internal Server Error（ERROR: `VALKEY_CONN_ERR`。`error` 属性にValkeyのエラー） |

---

## ■セクション10: ユースケース実装

### 10.1 VectorUseCase

```go
// internal/usecase/vector.go

// VectorUseCase はベクター生成ユースケースを実装する。
type VectorUseCase struct {
    subscriberStore    SubscriberRepository
    calculator         MilenageCalculator
    sqnManager         SQNManager
    sqnValidator       SQNValidator
    resyncProcessor    ResyncProcessor
    testVectorProvider TestVectorProvider // nilの場合はテストモード無効
    cfg                *config.Config
    // waitBeforeRetry はSQNの競合後、やり直す前に待つ（テストで差し替える）
    waitBeforeRetry func(ctx context.Context) error
}

// NewVectorUseCase は新しいVectorUseCaseを生成する。
func NewVectorUseCase(
    subscriberStore SubscriberRepository,
    calculator MilenageCalculator,
    sqnManager SQNManager,
    sqnValidator SQNValidator,
    resyncProcessor ResyncProcessor,
    testVectorProvider TestVectorProvider,
    cfg *config.Config,
) *VectorUseCase {
    return &VectorUseCase{
        subscriberStore:    subscriberStore,
        calculator:         calculator,
        sqnManager:         sqnManager,
        sqnValidator:       sqnValidator,
        resyncProcessor:    resyncProcessor,
        testVectorProvider: testVectorProvider,
        cfg:                cfg,
        waitBeforeRetry:    waitRandom,
    }
}

// maxSQNAttempts はSQNの書き換えを試みる最大回数（競合したときのやり直しを含む）。
const maxSQNAttempts = 3

// errSQNChanged は、加入者を読んでからSQNを書き換えるまでの間に、
// 他のリクエストがSQNを書き換えていたことを表す（やり直しの対象）。
var errSQNChanged = errors.New("SQN was changed by another request")

// GenerateVector はベクターを生成する。
// テストモード（TEST_VECTOR_ENABLED=true かつ対象プレフィックスのIMSI）では、
// Ki/OPc/AMF をテスト用の固定値に置き換える。加入者の取得・SQNの管理・エラー処理は通常モードと同じ。
//
// SQNは、読んだ値から変わっていないときだけ書き換える（CAS）。他のリクエストと競合したときは、
// 加入者の読み出しからやり直す。maxSQNAttempts 回とも競合したときは ErrSQNConflict を返す。
func (u *VectorUseCase) GenerateVector(ctx context.Context, req *dto.VectorRequest) (*dto.VectorResponse, error) {
    testMode := u.IsTestMode(req.IMSI)

    for attempt := 1; attempt <= maxSQNAttempts; attempt++ {
        if attempt > 1 {
            if err := u.waitBeforeRetry(ctx); err != nil {
                return nil, fmt.Errorf("%w: retry aborted after %d attempts: %v", ErrSQNConflict, attempt-1, err)
            }
        }

        resp, err := u.generateOnce(ctx, req, testMode, attempt > 1)
        if !errors.Is(err, errSQNChanged) {
            return resp, err
        }

        if attempt < maxSQNAttempts {
            slog.Warn("SQN update conflict, retrying",
                "event_id", "SQN_CONFLICT_RETRY",
                "trace_id", traceIDFromContext(ctx),
                "imsi", u.maskIMSI(req.IMSI),
                "attempt", attempt,
            )
        }
    }

    return nil, fmt.Errorf("%w: conflicted %d times", ErrSQNConflict, maxSQNAttempts)
}

// generateOnce は加入者の取得からSQNの書き換え・ベクター生成までを1回行う。
// 他のリクエストが先にSQNを書き換えていたときは errSQNChanged を返す。
// retried は競合によるやり直しかどうか（再同期の判定に使う）。
func (u *VectorUseCase) generateOnce(ctx context.Context, req *dto.VectorRequest, testMode, retried bool) (*dto.VectorResponse, error) {
    // 1. 加入者情報取得（テストモードでも登録が必要）
    sub, err := u.subscriberStore.Get(ctx, req.IMSI)
    if err != nil {
        return nil, fmt.Errorf("%w: %v", ErrValkeyConnection, err)
    }
    if sub == nil {
        return nil, ErrSubscriberNotFound
    }

    // 2. 鍵情報をバイト列に変換（テストモードは固定値）
    var ki, opc, amf []byte
    if testMode {
        ki, opc, amf = u.testVectorProvider.GetTestCryptoParams()
    } else {
        if ki, err = milenage.HexDecode(sub.Ki); err != nil {
            return nil, fmt.Errorf("invalid Ki format: %w", err)
        }
        if opc, err = milenage.HexDecode(sub.OPc); err != nil {
            return nil, fmt.Errorf("invalid OPc format: %w", err)
        }
        if amf, err = milenage.HexDecode(sub.AMF); err != nil {
            return nil, fmt.Errorf("invalid AMF format: %w", err)
        }
    }
    currentSQN, err := u.sqnManager.ParseHex(sub.SQN)
    if err != nil {
        return nil, fmt.Errorf("invalid SQN format: %w", err)
    }

    // 3. 新SQNの計算（再同期 or 通常）
    var newSQN uint64
    var resync *resyncResult
    if req.ResyncInfo != nil {
        resync, err = u.processResync(ki, opc, req.ResyncInfo, currentSQN, retried)
        if err != nil {
            return nil, err
        }
        newSQN = resync.newSQN
    } else {
        newSQN, err = u.sqnManager.Increment(currentSQN)
        if err != nil {
            return nil, ErrSQNOverflow
        }
    }

    // 4. SQN更新（読んだ値から変わっていないときだけ書き換える）
    updated, err := u.subscriberStore.CompareAndSetSQN(ctx, req.IMSI, sub.SQN, u.sqnManager.FormatHex(newSQN))
    if err != nil {
        return nil, fmt.Errorf("%w: %v", ErrValkeyConnection, err)
    }
    if !updated {
        return nil, errSQNChanged
    }

    // 5. 再同期のログ（SQNを書き換えた後に1回だけ出す）
    if resync != nil {
        u.logResync(ctx, req.IMSI, currentSQN, resync)
    }

    // 6. ベクター生成（SQNの書き換え後に行う。失敗してもSQNが飛ぶだけで、端末の検証には影響しない）
    vector, err := u.calculator.GenerateVector(ki, opc, amf, newSQN)
    if err != nil {
        return nil, fmt.Errorf("%w: %v", ErrMilenageCalculation, err)
    }

    // 7. レスポンス変換
    return milenage.VectorToResponse(vector), nil
}

// resyncResult は再同期で決めた新SQNを表す。
type resyncResult struct {
    sqnMS  uint64
    newSQN uint64
    // alreadySynced は、競合後のやり直しで、別のリクエストがすでにSQN_MS以上まで
    // SQNを進めていたため、通常どおり+32した場合に true
    alreadySynced bool
}

// processResync は再同期処理を行い、新しいSQNを決める。
// retried は競合によるやり直しかどうか。
func (u *VectorUseCase) processResync(ki, opc []byte, resyncInfo *dto.ResyncInfo, currentSQN uint64, retried bool) (*resyncResult, error) {
    // 1. RAND/AUTS をバイト列に変換
    randVal, err := milenage.HexDecode(resyncInfo.RAND)
    if err != nil {
        return nil, fmt.Errorf("%w: invalid RAND format", ErrResyncInvalidFormat)
    }
    auts, err := milenage.HexDecode(resyncInfo.AUTS)
    if err != nil {
        return nil, fmt.Errorf("%w: invalid AUTS format", ErrResyncInvalidFormat)
    }

    // 2. AUTS長検証
    if len(auts) != 14 {
        return nil, ErrResyncInvalidFormat
    }

    // 3. SQN_MS抽出
    sqnMS, err := u.resyncProcessor.ExtractSQN(ki, opc, randVal, auts)
    if err != nil {
        // MAC検証失敗
        return nil, ErrResyncMACFailed
    }

    // 4. 競合後のやり直しで、別のリクエスト（同じ再同期の再送など）がすでにSQN_MS以上まで
    // 進めていた場合は、同期済みとみなして通常どおり+32する（端末のSQN_MSより大きいので受け入れられる）
    if retried && currentSQN >= sqnMS {
        newSQN, err := u.sqnManager.Increment(currentSQN)
        if err != nil {
            return nil, ErrSQNOverflow
        }
        return &resyncResult{sqnMS: sqnMS, newSQN: newSQN, alreadySynced: true}, nil
    }

    // 5. デルタ検証
    if err := u.sqnValidator.ValidateResyncSQN(sqnMS, currentSQN); err != nil {
        // ログはハンドラーが1行で出力する（SQN値はエラー文に含める）
        return nil, fmt.Errorf("%w: sqn_ms=%012x sqn_he=%012x: %v", ErrResyncDeltaExceeded, sqnMS, currentSQN, err)
    }

    // 6. 新SQN計算（SQN_MS + 32）
    newSQN, err := u.sqnValidator.ComputeResyncSQN(sqnMS)
    if err != nil {
        return nil, ErrSQNOverflow
    }

    return &resyncResult{sqnMS: sqnMS, newSQN: newSQN}, nil
}

// logResync はSQN再同期のログを出す。
func (u *VectorUseCase) logResync(ctx context.Context, imsi string, currentSQN uint64, r *resyncResult) {
    msg := "SQN resync successful"
    if r.alreadySynced {
        msg = "SQN resync already applied by another request"
    }
    slog.Info(msg,
        "event_id", "SQN_RESYNC",
        "trace_id", traceIDFromContext(ctx),
        "imsi", u.maskIMSI(imsi),
        "sqn_old", fmt.Sprintf("%012x", currentSQN),
        "sqn_ms", fmt.Sprintf("%012x", r.sqnMS),
        "sqn_new", fmt.Sprintf("%012x", r.newSQN),
    )
}

// waitRandom は競合後のやり直しの前に、1〜10msのランダムな時間だけ待つ。
// 同時に競合したリクエストが同じタイミングでやり直さないようにする。
// 待っている間に ctx が終わったときは ctx のエラーを返す。
func waitRandom(ctx context.Context) error {
    timer := time.NewTimer(time.Millisecond + rand.N(9*time.Millisecond))
    defer timer.Stop()

    select {
    case <-ctx.Done():
        return ctx.Err()
    case <-timer.C:
        return nil
    }
}

// IsTestMode はIMSIがテストベクターモードの対象かを返す。
func (u *VectorUseCase) IsTestMode(imsi string) bool {
    return u.testVectorProvider != nil && u.testVectorProvider.IsTestIMSI(imsi)
}

// maskIMSI はログ出力用にIMSIをマスキングする。
// 設定が無い場合はマスキングを有効として扱う（LOG_MASK_IMSI の既定値 true に合わせる）。
func (u *VectorUseCase) maskIMSI(imsi string) string {
    enabled := true
    if u.cfg != nil {
        enabled = u.cfg.LogMaskIMSI
    }
    return logging.MaskIMSI(imsi, enabled)
}
```

```go
// internal/usecase/trace.go

type traceIDKey struct{}

// ContextWithTraceID はTrace IDをコンテキストに設定する（ハンドラーから呼ぶ）
func ContextWithTraceID(ctx context.Context, traceID string) context.Context {
    return context.WithValue(ctx, traceIDKey{}, traceID)
}

// traceIDFromContext はコンテキストからTrace IDを取り出す（未設定なら空文字）
func traceIDFromContext(ctx context.Context) string {
    id, _ := ctx.Value(traceIDKey{}).(string)
    return id
}
```

> **注記（SQN競合制御、r10）:** `GenerateVector` は `generateOnce`（加入者の取得からSQNの書き換え・ベクター生成までの1回分）を最大 `maxSQNAttempts`（3）回試す。SQNの書き換えは `CompareAndSetSQN`（§8.2）で、`Get` で読んだ値から変わっていないときだけ行う。変わっていた（競合）ときは `errSQNChanged` を返し、`GenerateVector` は `SQN_CONFLICT_RETRY`（WARN）を出して `waitBeforeRetry`（既定は `waitRandom`。1〜10msのランダムな時間待つ。テストでは差し替える）の後にやり直す。やり直しでは加入者を読み直し、鍵情報・SQN・新SQN（再同期では MAC-S 検証も）を計算し直す。3回とも競合したとき、または待っている間に ctx が終わった（期限切れ・キャンセル）ときは `ErrSQNConflict`（409）を返す。再同期のやり直しで SQN_HE がすでに SQN_MS 以上のときは、別のリクエスト（同じ再同期の再送など）が先に同期済みとみなして通常どおり +32 する（§13.6.3）。詳細は §13.6。

> **注記（ログ、r10）:** ユースケース層が出力するログは、再同期成功時の `SQN_RESYNC`（SQNの書き換えに成功した後に1回だけ。r9 までは `processResync` の中で出していた）と、競合してやり直すときの `SQN_CONFLICT_RETRY` だけである。`trace_id` はハンドラーが context に載せた値（`X-Trace-ID` ヘッダの値。ない場合は `no-trace-id`）、`imsi` はマスク済み。SQNデルタ超過はログを出さず、SQN値をエラー文に含めて返し、ハンドラーが `SQN_RESYNC_DELTA_ERR` を1行出力する。テストモードの成功時もユースケース層はログを出さず、ハンドラーの `CALC_OK` の `test_mode` 属性で区別する（r8 まで出力していた `test vector generated`（`test_mode` / `sqn` 属性）は削除）。

> **注記（テストモード、r8）:** テストモードで通常モードと異なるのは、Milenage 計算に使う Ki / OPc / AMF を `GetTestCryptoParams()` の固定値（3GPP TS 35.208 Test Set 1、AMF `B9B9`）に置き換える点だけである。加入者 `sub:{IMSI}` の取得・SQN の解析と書き戻し・再同期・エラー処理は通常モードと同じで、未登録IMSIは `ErrSubscriberNotFound`（404）、Valkeyエラーと SQN 書き戻し失敗は `ErrValkeyConnection`（500）、SQN の解析失敗はエラー（500）になる。`sub:{IMSI}` の `ki` / `opc` / `amf` は参照しない（形式チェックもしない）が、`sqn` を使うため加入者の登録は必須である。r7 以前の実装にあった既定 SQN（`ff9bb4d0b607`）へのフォールバック（`TEST_SQN_FALLBACK` / `TEST_SQN_PARSE_ERR` / `TEST_SQN_PERSIST_ERR`）と `GetDefaultSQN()` は削除した（未登録IMSIに `sqn` だけを持つ `sub:{IMSI}` が作られる問題を解消）。

---

## ■セクション11: ログ出力

### 11.1 event_id一覧

D-04で定義されたVector API用event_id:

| event_id | レベル | 説明 |
|----------|--------|------|
| `CALC_OK` | INFO | ベクター生成成功（`test_mode` 属性でテストベクターモードかを区別。テストベクターモードも1行） |
| `CALC_ERR` | INFO/WARN/ERROR | 計算・データエラー（リクエスト不正・IMSI形式不正=WARN、IMSI不在=INFO（テストベクターモードの対象IMSIも同じ）、Milenage計算エラー・予期しないエラー=ERROR） |
| `SQN_RESYNC` | INFO | SQN再同期成功（ユースケース層。SQNの書き換え成功後に1回。`trace_id`・マスク済み `imsi` あり。やり直しで同期済みとみなした場合は msg `SQN resync already applied by another request`） |
| `SQN_RESYNC_MAC_ERR` | WARN | AUTS MAC検証失敗（AUTSからのSQN抽出失敗を含む） |
| `SQN_RESYNC_FORMAT_ERR` | WARN | AUTS形式不正（RAND/AUTSのHexデコード失敗を含む） |
| `SQN_RESYNC_DELTA_ERR` | WARN | SQNデルタ超過（ハンドラー層の1行。SQN値は `error` 属性の文中） |
| `SQN_OVERFLOW_ERR` | ERROR | SQNオーバーフロー |
| `SQN_CONFLICT_RETRY` | WARN | SQN更新の競合を検出し、やり直す（ユースケース層。`attempt` に競合した試行の回数） |
| `SQN_CONFLICT_ERR` | WARN | SQN更新の競合がやり直しの上限を超過（409返却。ハンドラー層の1行） |
| `VALKEY_CONN_ERR` | ERROR | 加入者取得・SQN更新時のValkeyエラー（500返却。テストベクターモードの対象IMSIも同じ） |

> **注記:** 各event_idの `msg`・属性はD-04 §3.4を参照。ハンドラーが定義済みエラー（`ProblemError`）から出力するログ（`CALC_ERR` の一部 / `SQN_RESYNC_*_ERR` / `SQN_OVERFLOW_ERR` / `VALKEY_CONN_ERR`）は `error` 属性に原因を含むエラー文を持つ（§9.3）。起動時のValkey接続失敗は event_id なしの `failed to connect to Valkey` で出力する。Valkey接続の復旧検知ログは出力しない。`SQN_CONFLICT_ERR` もハンドラーが `ProblemError` から出力し、`error` 属性に原因（例: `SQN update conflict: conflicted 3 times`）を持つ。テストベクターモード専用のエラー系 event_id はない（r7 まで記載していた `TEST_SQN_FALLBACK` / `TEST_SQN_PARSE_ERR` / `TEST_SQN_PERSIST_ERR` は r8 で廃止。エラーは通常モードと同じ event_id で出力する）。

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
  "http_status": 200,
  "test_mode": false
}
```

> **注記:** `CALC_OK` は `method`・`path`・`latency_ms` を持たない。リクエスト全体の処理時間はミドルウェアの `request completed`（event_id なし）の `latency_ms` で確認する。

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
  "http_status": 404,
  "error": "IMSI does not exist in subscriber DB"
}
```

#### 再同期成功時

```json
{
  "time": "2026-01-14T10:00:00.789Z",
  "level": "INFO",
  "app": "vector-api",
  "msg": "SQN resync successful",
  "event_id": "SQN_RESYNC",
  "trace_id": "550e8400-e29b-41d4-a716-446655440002",
  "imsi": "440101********0",
  "sqn_old": "000000000020",
  "sqn_ms": "000000000060",
  "sqn_new": "000000000080"
}
```

#### デルタ超過時

ハンドラー層の1行のみ（r8 まではユースケース層の `SQN delta validation failed` も出力していた）。SQN_MS・SQN_HEは `error` 属性の文中に12桁Hexで含まれる。

```json
{
  "time": "2026-01-14T10:00:01.123Z",
  "level": "WARN",
  "app": "vector-api",
  "msg": "SQN delta exceeded",
  "trace_id": "550e8400-e29b-41d4-a716-446655440003",
  "event_id": "SQN_RESYNC_DELTA_ERR",
  "imsi": "440101********0",
  "http_status": 400,
  "error": "SQN difference exceeds allowed range: sqn_ms=100000000000 sqn_he=000000000020: SQN difference exceeds delta: 17592186044384 > 268435456"
}
```

#### SQN更新の競合時

競合してやり直すたびにユースケース層が `SQN_CONFLICT_RETRY` を出す（1リクエストで最大2回）。やり直しで書き換えに成功すれば、その後は通常どおり `CALC_OK` になる。

```json
{
  "time": "2026-01-14T10:00:01.456Z",
  "level": "WARN",
  "app": "vector-api",
  "msg": "SQN update conflict, retrying",
  "event_id": "SQN_CONFLICT_RETRY",
  "trace_id": "550e8400-e29b-41d4-a716-446655440004",
  "imsi": "440101********0",
  "attempt": 1
}
```

3回とも競合した場合は、ハンドラー層が `SQN_CONFLICT_ERR` を1行出して 409 を返す。

```json
{
  "time": "2026-01-14T10:00:01.478Z",
  "level": "WARN",
  "app": "vector-api",
  "msg": "SQN update conflict exceeded retry limit",
  "trace_id": "550e8400-e29b-41d4-a716-446655440004",
  "event_id": "SQN_CONFLICT_ERR",
  "imsi": "440101********0",
  "http_status": 409,
  "error": "SQN update conflict: conflicted 3 times"
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
    // CompareAndSetSQN はSQNが oldSQN のときだけ newSQN に書き換え、書き換えたかどうかを返す。
    CompareAndSetSQN(ctx context.Context, imsi, oldSQN, newSQN string) (bool, error)
}

type TestVectorProvider interface {
    IsTestIMSI(imsi string) bool
    GetTestVector(imsi string) (*Vector, error)
    GetTestCryptoParams() (ki, opc, amf []byte) // テスト用 Ki/OPc/AMF（防御的コピー）
}

type VectorUseCaseInterface interface {
    GenerateVector(ctx context.Context, req *VectorRequest) (*VectorResponse, error)
    IsTestMode(imsi string) bool // テストベクターモードの対象IMSIか（ログ出力用）
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
| SQN競合制御 | Vector API の書き戻しは CAS（§13.6）。Admin TUI の加入者編集も、SQN を変えていないときは `sqn` を書かず、変えたときは編集開始時の値との比較・置き換えで書き込む（§13.6.9）。ただし Admin TUI の新規作成と CSV インポート（既存キーの上書き）は比較せずに `sqn` を書き込む | - |

### 13.4 テスト戦略

| レイヤー | テスト方法 |
|---------|-----------|
| `handler` | Ginテストフレームワーク、モックusecase |
| `usecase` | 単体テスト、モックrepository/calculator |
| `milenage` | 3GPP TS 35.208テストベクター |
| `sqn` | 単体テスト（インクリメント、デルタ検証） |
| `store` | miniredisによるインメモリテスト（`CompareAndSetSQN` の一致・不一致・キーなし） |
| `usecase`（並行性） | miniredis と実物の store / sqn / milenage で同一IMSIに並行リクエストを送り、SQN が一意・単調増加であること（巻き戻らないこと）を確認 |
| `E2E` | eapaka_testによる統合テスト |

### 13.5 テストモード実装（E2Eテスト対応）

#### 概要

E2Eテスト・結合テスト（eapaka_test等）向けに、対象IMSIの Ki / OPc / AMF を 3GPP TS 35.208 Test Set 1 の固定値に置き換えてベクターを計算するテストモードを実装する。加入者の登録（`sub:{IMSI}`）と SQN 管理は通常モードと同じく必要である。
**方式A + 環境変数ガード** を採用し、二重ガードで本番環境での誤発動を防止する。

#### 環境変数

| 環境変数 | デフォルト | 説明 |
|---------|-----------|------|
| `TEST_VECTOR_ENABLED` | `false` | テストベクターモード有効化 |
| `TEST_VECTOR_IMSI_PREFIX` | `00101` | テスト対象IMSIプレフィックス（5-6桁） |

#### 判定ロジック

```
1. TEST_VECTOR_ENABLED=true か確認（起動時。false なら TestVectorProvider を生成しない）
   └─ false → 通常処理（Valkey参照）

2. IMSIプレフィックスが TEST_VECTOR_IMSI_PREFIX に一致か
   └─ 不一致 → 通常処理（Valkey参照）

3. 一致 → 通常処理と同じ流れで、Ki/OPc/AMF だけを Test Set 1 の固定値に置き換える
   ├─ sub:{IMSI} を取得（未登録 → 404、Valkeyエラー → 500）
   ├─ sub:{IMSI} の sqn を解析（失敗 → 500）。ki / opc / amf は参照しない
   ├─ SQN +32（再同期時は SQN_MS+32）、乱数 RAND で Milenage 計算
   └─ 新SQNを sub:{IMSI} に書き戻す（失敗 → 500。ベクターは返さない）
```

#### テストベクター実装

```go
// internal/testmode/testvector.go（抜粋）

// 3GPP TS 35.208 Test Set 1
var (
    testRAND = []byte{ /* 23553cbe9637a89d218ae64dae47bf35 */ }
    testKi   = []byte{ /* 465b5ce8b199b49faa5f0a2ee238a6bc */ }
    testOPc  = []byte{ /* cd63cb71954a9f4e48a5994e37a02baf */ }
    testAMF  = []byte{0xb9, 0xb9}
    testSQN  uint64 = 0xff9bb4d0b607 // GetTestVector 専用（ユースケースの SQN には使わない）
)

type TestVectorProvider struct {
    imsiPrefix string
}

// main.go で cfg.TestVectorEnabled が true のときだけ生成する（false なら nil を渡し、テストモード無効）
func NewTestVectorProvider(imsiPrefix string) *TestVectorProvider {
    return &TestVectorProvider{imsiPrefix: imsiPrefix}
}

func (p *TestVectorProvider) IsTestIMSI(imsi string) bool {
    return strings.HasPrefix(imsi, p.imsiPrefix)
}

// GetTestCryptoParams はテスト用の Ki / OPc / AMF を返す（防御的コピー）。
// ユースケース（§10.1）はこの値で Milenage を計算する。
func (p *TestVectorProvider) GetTestCryptoParams() (ki, opc, amf []byte) {
    ki = append([]byte(nil), testKi...)
    opc = append([]byte(nil), testOPc...)
    amf = append([]byte(nil), testAMF...)
    return
}

// GetTestVector は固定 RAND・固定 SQN で Test Set 1 のベクターを返す。
// インターフェースには残っているが、現行のユースケースからは呼ばれない。
func (p *TestVectorProvider) GetTestVector(imsi string) (*milenage.Vector, error) {
    if !p.IsTestIMSI(imsi) {
        return nil, fmt.Errorf("IMSI %s is not a test IMSI", imsi)
    }
    calc := milenage.NewCalculator()
    return calc.GenerateVectorWithRAND(testKi, testOPc, testAMF, testSQN, testRAND)
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
- **加入者登録**: テストモードでも `sub:{IMSI}` の登録が必要（未登録なら通常モードと同じく 404）。`ki` / `opc` / `amf` フィールドは参照しないが、`sqn` を使う。テスト用加入者は T-03 の事前準備（test_subscriber_data_scripts）で登録する
- **SQN管理**: 通常モードと同じく `sub:{IMSI}` の `sqn` を +32 して書き戻す。`sqn` を解析できない場合や書き戻しに失敗した場合はエラー（500）とし、既定 SQN へのフォールバックはしない
- **RAND**: 毎回乱数（固定 RAND の `GetTestVector()` はユースケースからは使わない）
- **再同期テスト**: テストモードでも再同期（AUTS）処理は通常どおり行う（固定 Ki/OPc で MAC-S を検証し、SQN_MS+32 を書き戻す）

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

> **実装状況（r10）:** 本節は実装済みである。r9 までは WATCH/MULTI による CAS として設計していた（未実装）が、r10 で Lua スクリプトによる `sqn` フィールドの比較・置き換えに方式を変えて実装した（方式の比較は §7.5）。

#### 13.6.1 概要

同一IMSIへの並行Access-Request（AP の再送、短時間の再認証、並行テスト等）で SQN が同値になる・巻き戻るのを防ぐため、SQN の書き戻しを CAS（Compare-And-Swap）で行う。読んだ値から変わっていないときだけ書き換え、変わっていたときは加入者の読み出しからやり直す。

| 起きていた現象（r9 まで） | 流れ | 影響 |
|------|------|------|
| SQNが同値になる | 通常リクエスト2本が同じ X を読み、両方が X+32 を書く | 同じSQNのベクターが2つ発行され、後で使う方は端末で同期失敗になる（再同期が1往復増える） |
| SQNが巻き戻る | 再同期が SQN_MS+32 を書いた後に、古い X を読んでいた通常リクエストが X+32 で上書きする | 次の認証で再び同期失敗になる |

#### 13.6.2 方式詳細

| 項目           | 内容                              |
| -------------- | --------------------------------- |
| 方式           | Lua スクリプト（`redis.NewScript`、EVALSHA）による `sqn` フィールドの比較・置き換え |
| 対象キー       | `sub:{IMSI}`                      |
| 対象フィールド | `sqn`（比較・更新とも。他のフィールドは比較しない） |
| 期待値         | `Get`（HGETALL）で読んだ `sqn` の生の文字列（正規化しない） |
| 試行回数       | 最大3回（`maxSQNAttempts`。最初の1回＋やり直し2回） |
| やり直しの前の待ち | 1〜10ms のランダムな時間（`waitRandom`。同時に競合したリクエストのやり直しをずらす） |
| 競合検出時動作 | `SQN_CONFLICT_RETRY`（WARN）を出し、待ってから加入者の読み出しからやり直す |
| 上限超過時動作 | HTTP 409 Conflict（`ErrSQNConflict`、`SQN_CONFLICT_ERR`（WARN）） |
| 待ち中に ctx が終わった | やり直さずに 409（`ErrSQNConflict`） |
| キーが無い（途中で削除） | 書き換えない（キーを作らない）。やり直しの `Get` で未登録 → 404 |

#### 13.6.3 処理フロー

```
GenerateVector:
  for attempt = 1..3:
    attempt > 1 なら 1〜10ms 待つ（ctx が終わったら 409）
    generateOnce:
      1. HGETALL sub:{IMSI} → Ki/OPc/AMF/SQN取得（未登録は 404）
      2. 鍵情報の変換（テストモードは Ki/OPc/AMF を固定値に）・SQNの解析
      3. 新SQN算出
         ├─ 通常: SQN_HE + 32
         └─ 再同期: RAND/AUTS の変換・長さ検証 → MAC-S 検証で SQN_MS 抽出 →
              ├─ attempt > 1 かつ SQN_HE >= SQN_MS: 同期済みとみなし SQN_HE + 32（デルタ検証しない）
              └─ それ以外: デルタ検証（SQN_MS > SQN_HE、差が Δ 以内）→ SQN_MS + 32
      4. EVALSHA（Lua）: sqn == 読んだ値 なら HSET sub:{IMSI} sqn {新SQN}
         ├─ 書き換えた → 5へ
         ├─ 変わっていた（競合）→ errSQNChanged
         │    └─ attempt < 3 なら SQN_CONFLICT_RETRY を出して次の attempt へ
         └─ Valkey エラー → 500（VALKEY_CONN_ERR）
      5. 再同期なら SQN_RESYNC を出す（ここで1回だけ）
      6. Milenage計算（失敗したら 500。SQNは飛ぶだけ）→ 応答
  3回とも競合 → 409 Conflict（SQN_CONFLICT_ERR）
```

**再同期のやり直しで同期済みとみなす理由:** 同じ再同期要求が2本並行して届いた場合（RADIUS の再送など）、1本目が SQN_MS+32 を書くと、2本目のやり直しでは SQN_HE（= SQN_MS+32）が SQN_MS 以上になり、デルタ検証（SQN_MS > SQN_HE）で 400 になってしまう。このとき SQN_HE はすでに端末の SQN_MS より進んでいるので、通常どおり SQN_HE+32 で生成したベクターは端末に受け入れられる（3GPP TS 33.102 の「SQN_HE が受け入れられる範囲にあればリセットしない」という考え方に合う）。1回目の試行では今までどおり検証し、SQN_MS <= SQN_HE なら 400（`SQN_RESYNC_DELTA_ERR`）とする。このとき `SQN_RESYNC` の msg は `SQN resync already applied by another request` になる。

**MAC-S 検証を試行ごとに行う理由:** やり直しでは鍵情報も読み直すため、その鍵で検証し直す（Admin TUI で途中に鍵が変わった場合も正しく検証できる）。計算コストは小さい。

#### 13.6.4 エラー応答（競合上限超過時）

```json
{
  "type": "about:blank",
  "title": "Conflict",
  "detail": "SQN update conflict",
  "status": 409
}
```

`detail` には IMSI や原因を含めない（§9.3。r9 までの設計例では IMSI と試行回数を含めていた）。試行回数はログの `error` 属性で確認する。

#### 13.6.5 実装（store）

§8.2 の `CompareAndSetSQN` を参照。Lua スクリプトは次のとおり。

```lua
local cur = redis.call('HGET', KEYS[1], 'sqn')
if cur == false or cur ~= ARGV[1] then
  return 0
end
redis.call('HSET', KEYS[1], 'sqn', ARGV[2])
return 1
```

Valkey は Lua スクリプトを原子的に実行するため、HGET と HSET の間に他のコマンドが割り込まない。store はリトライもログも行わず、書き換えたかどうか（bool）だけを返す。やり直し・ログ・409 の判断はユースケース層が行う（r9 までの設計例ではリトライとログを store 層の `UpdateSQNWithCAS` に置いていた）。

#### 13.6.6 実装（ユースケース層）

§10.1 の `GenerateVector` / `generateOnce` / `processResync` / `logResync` / `waitRandom` を参照。

#### 13.6.7 エラー定義

§9.3 の `ErrSQNConflict`（`*ProblemError`、409、`SQN_CONFLICT_ERR`）を参照。

#### 13.6.8 ハンドラー層・呼び出し側

- ハンドラーに 409 専用の分岐はない。`ErrSQNConflict` は他の `ProblemError` と同じく `handleError` で応答とログ（`SQN_CONFLICT_ERR`、WARN、`error` 属性に原因）になる（§5.1）。
- Vector Gateway は 4xx をそのまま中継する（D-12）。Auth Server は 409 を Circuit Breaker の失敗に数えず（D-09 §7.8.1）、`VECTOR_API_ERR`（`http_status`=409）を出して Access-Reject（EAP-Failure）を返す。端末は認証をやり直す。
- 409 を 5xx にしない理由: 5xx は Auth Server の Circuit Breaker の失敗に数えられ、1つの IMSI の競合で全体の認証が止まりうるため。

#### 13.6.9 Admin TUI の書き込みとの関係

Admin TUI の加入者編集（`apps/admin-tui/internal/store/subscriber.go`）は、Lua スクリプトで存在チェックと更新をまとめて行い、`sqn` を次のように扱う（D-02 §2.A、D-05 §4.2.2）。

- SQN を変えていない（大文字小文字の違いだけを含む）ときは `ki` / `opc` / `amf` だけを更新し、`sqn` は書かない（`Update`）。Vector API の CAS は `sqn` だけを比較するため、競合にならない
- SQN を変えたときは、現在の `sqn` が編集開始時に読んだ値と一致するときだけ `ki` / `opc` / `amf` と `sqn` を更新する（`UpdateWithSQN`）。編集中に Vector API が `sqn` を進めていれば Admin TUI 側が何も更新せずにエラーとし、Admin TUI の書き込みが Vector API の読み出しから書き換えまでの間に行われれば Vector API 側が競合として検出してやり直す

これにより、Admin TUI の保存で SQN が巻き戻ることはない。比較せずに `sqn` を書き込むのは、Admin TUI の新規作成と CSV インポート（既存キーを上書きする）だけである。

**注記（r11）:** r10 では、Admin TUI の加入者編集が SQN を変えていなくても `sqn` をフォームの値で上書きする（編集画面を開いている間に認証が進むと SQN が巻き戻る可能性がある）ことを「残っている制約」として記載していた。Admin TUI の実装修正で解消したため、本節を改めた。

## 改訂履歴

| 版数 | 日付 | 内容 |
|------|------|------|
| r1 | 2026-01-14 | 初版作成 |
| r2 | 2026-01-17 | SQNインクリメント方式変更（+32方式）、デルタ検証追加、AMF取得方式明確化、IMSIマスキング環境変数対応、E2Eテストモード追加 |
| r3 | 2026-01-26 | SQN競合制御追加: セクション13.3の「SQN競合制御: 簡易実装」を削除、セクション13.6新設（WATCH/MULTIによるCAS方式、リトライ上限3回、HTTP 409エラー）、実装例・エラー定義追加、これらに伴うセクション7.5の記載変更 |
| r4 | 2026-01-26 | インフラ基盤統一: セクション2.6新設（Dockerfile方針 - ベースイメージdebian:bookworm-slim、curl/ca-certificates導入、ヘルスチェックcurl -fsS）。これに伴い、旧 2.6 ファイル別責務詳細 のセクション番号を 2.7 に移行 |
| r5 | 2026-01-27 | 本番環境注記追加: セクション13.5.1にテストベクターモードの本番無効化要件を新設、関連ドキュメント参照バージョン更新、D-08への参照追加 |
| r6 | 2026-02-18 | ディレクトリ構造全面更新、usecase統合反映、関連ドキュメント版数更新 |
| r7 | 2026-10-04 | D-04 r19 の event_id 全面整合に合わせて修正: §11.1 event_id一覧から実装に存在しない `SQN_RESYNC_DECODE_ERR`（AUTSからのSQN抽出失敗は `SQN_RESYNC_MAC_ERR`）と `VALKEY_CONN_RESTORED` を削除し、`TEST_SQN_FALLBACK` / `TEST_SQN_PARSE_ERR` / `TEST_SQN_PERSIST_ERR` を追加、各説明を実装に合わせて修正。§11.2 ログ出力例を実装の属性に修正（`CALC_OK` / `CALC_ERR` から `method`・`path`・`latency_ms` を削除、ユースケース層の `SQN_RESYNC` / `SQN_RESYNC_DELTA_ERR` から `trace_id` を削除）。SQN競合制御（WATCH/MULTI による CAS、リトライ上限3回、HTTP 409、`SQN_CONFLICT_RETRY` / `SQN_CONFLICT_ERR`）は設計を残したまま「設計済み・未実装（現行は単純な HSET による後勝ち）」と明記（§7.5、§13.3、§13.6 冒頭・各見出し、§2.7 の `ErrSQNConflict` 記載、§1.3）。§8.3 `GetWithRetry` が未使用でリトライログが出力されない旨を注記し、§9.4 のリトライ記載を修正。§1.3 関連ドキュメントの版数を現行版に更新（D-01 r10、D-02 r12、D-03 r6、D-04 r19、D-06 r7、D-07 r8、D-08 r14、D-12 r5、E-02 r3。Auth Server詳細設計書の文書番号を D-09 に修正）。関連ドキュメント表の E-03 を実在の文書名（共通ライブラリ(pkg)設計書）に修正 |
| r8 | 2026-10-04 | テストベクターモードでも加入者登録を必須にした実装修正の反映: §10.1 `GenerateVector` をテストモードと通常モードの共通処理に更新（テストモードで置き換えるのは Ki/OPc/AMF だけ。加入者の取得・SQN の解析と書き戻し・エラー処理は通常モードと同じで、未登録IMSIは404、Valkeyエラー・SQN書き戻し失敗は500、SQN解析失敗はエラー）し、旧 `generateTestVector` を削除、テストモードの注記を追加。§11.1 から `TEST_SQN_FALLBACK` / `TEST_SQN_PARSE_ERR` / `TEST_SQN_PERSIST_ERR` を削除し、`CALC_ERR` / `VALKEY_CONN_ERR` / `CALC_OK` の説明にテストベクターモードの扱いを追記。§13.5 の概要・判定ロジック・注意事項を実装どおりに修正（固定ベクター返却・SQN非永続・再同期スキップの記述を、固定 Ki/OPc/AMF で計算・加入者登録必須・SQN は通常どおり管理・再同期も通常どおりに訂正）し、テストベクター実装のコード例を現行の `testvector.go`（`NewTestVectorProvider(imsiPrefix)`、`GetTestCryptoParams()`）に更新。§2.5・§12.3 の `TestVectorProvider` インターフェースに `GetTestCryptoParams()` を追記し、§2.1 ディレクトリ構造のコメント・§2.3 パッケージ一覧・§2.7 ファイル別責務を更新。§1.3 関連ドキュメントの版数を更新（D-02 r14、D-04 r22） |
| r9 | 2026-10-04 | vector-api のログ整理・未使用コード削除の実装修正の反映: §5.1 ハンドラーのコードを実装に合わせ更新（`usecase.ContextWithTraceID` で Trace ID を context に載せる、`CALC_OK` に `test_mode`、ProblemError 経路のログに `error` 属性（原因を含むエラー文。応答の detail は従来どおり）、IMSIマスクは `logging.MaskIMSI`、依存インターフェースは `usecase.VectorUseCaseInterface`）。§10.1 `processResync` に `ctx` / `imsi` 引数を追加し `SQN_RESYNC` に `trace_id`・マスク済み `imsi` を出力、SQNデルタ超過時のユースケース層のログを削除しSQN値をエラー文に含める形に、テストモードの `test vector generated` ログを削除、`IsTestMode()` / `maskIMSI()` と `internal/usecase/trace.go`（`ContextWithTraceID`）を追記。§8.3 から未使用で削除した `GetWithRetry`（リトライ・`isConnectionError`）のコードを削除しアプリ独自のリトライなし（go-redis の既定の自動リトライのみ）の説明に変更、§9.3 から削除した `ErrInvalidIMSI` を削除しエラー文の扱いを注記、§9.4 に `error` 属性を追記。§11.1・§11.2 のログ（`CALC_OK` の `test_mode`、`SQN_RESYNC` の `trace_id` / `imsi`、`SQN_RESYNC_DELTA_ERR` の1行化、`error` 属性）を更新。§2.1・§2.5・§2.7・§12.3 に `trace.go` と `VectorUseCaseInterface`（`IsTestMode` を追加）を反映。§3.1・§3.3 に `LOG_LEVEL` を `pkg/logging.ParseLevel` で変換する旨を追記。§1.3 の D-04 / D-06 / E-03 の参照版数を更新 |
| r10 | 2026-10-04 | SQN競合制御（CAS）の実装の反映: 方式を WATCH/MULTI から Lua スクリプトによる `sqn` フィールドの比較・置き換えに変更して実装した。§7.5 を「SQN競合制御の検討」から「SQN競合制御」に改め、検討した方式の採否と理由・守る性質（SQN は IMSI ごとに一意で単調増加、飛ぶのは許容）を記載。§8.2 の `UpdateSQN`（単純な HSET、後勝ち）を `CompareAndSetSQN`（Lua、期待値は読んだ生の文字列、キーが無ければ作らない）に置き換え、§8.3 を合わせて修正。§8.4 のデータアクセスフローを1回の試行（CAS → 成功後に Milenage 計算）と競合時のやり直しに更新（r9 までは計算後に書き戻していた）。§9.1 / §9.3 に `ErrSQNConflict`（409、`SQN_CONFLICT_ERR`、WARN、detail に IMSI・原因を含めない）を追加。§10.1 のコードを実装（`GenerateVector` の最大3回の試行、`generateOnce`、`processResync` の戻り値 `resyncResult` と再同期のやり直しで SQN_HE >= SQN_MS なら同期済みとみなして +32、`logResync` で CAS 成功後に `SQN_RESYNC` を1回、`waitRandom` で 1〜10ms 待つ）に更新し、SQN競合制御の注記を追加。§11.1 / §11.2 に `SQN_CONFLICT_RETRY`（WARN、`attempt`）・`SQN_CONFLICT_ERR` と出力例を追加し、`SQN_RESYNC` の出力タイミングと msg（2種類）を明記。§13.6 を実装済みとして全面的に書き直し（方式詳細、処理フロー、再同期のやり直しの扱いと理由、409 応答、Lua スクリプト、ハンドラー・呼び出し側の扱い、409 を 5xx にしない理由、残っている制約）。§13.3 の SQN競合制御の行を Admin TUI の `sqn` 上書きの制約に、§13.4 に store・並行性のテストを追記。§2.1 のディレクトリ構造（テストファイル追加）、§2.5 / §2.7 / §12.3 のインターフェース・ファイル責務を更新。§1.3 参照版数更新（D-02 r18、D-03 r8、D-04 r28、D-06 r15） |
| r11 | 2026-10-04 | Admin TUI の加入者編集による `sqn` の上書き（巻き戻り）を解消した実装修正の反映: §13.6.9 を「残っている制約」から「Admin TUI の書き込みとの関係」に改め、Admin TUI は SQN を変えていなければ `sqn` を書かず、変えたときは編集開始時の値と一致するときだけ書き込むこと（Vector API の CAS との関係）、比較せずに書き込むのは新規作成と CSV インポートだけであることを記載。§13.3 の SQN競合制御の行を合わせて修正し、将来対応を削除。§1.3 参照版数更新（D-02 r19、D-04 r29） |
