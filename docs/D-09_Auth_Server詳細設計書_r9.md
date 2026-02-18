# D-09 Auth Server詳細設計書 (r9)

## ■セクション1: 概要

### 1.1 目的

本ドキュメントは、EAP-AKA RADIUS PoC環境における認証サーバー「Auth Server」の実装レベル設計を定義する。

### 1.2 スコープ

**本書で扱う範囲：**

| 範囲 | 内容 |
|------|------|
| RADIUS認証処理 | UDP 1812受信、パケットパース、応答生成 |
| EAP-AKA/AKA'制御 | ステートマシン実装、各フェーズ処理 |
| Vector Gateway連携 | HTTPクライアント |
| 認可処理 | ポリシー評価（SSIDマッチング、Action/時間帯条件）、AVP生成 |
| セッション管理 | EAPコンテキスト、セッション作成 |
| Status-Server応答 | RFC 5997準拠のヘルスチェック応答 |

### 1.3 関連ドキュメント

| No. | ドキュメント | 参照内容 |
|-----|-------------|---------|
| D-01 | ミニPC版設計仕様書 (r9) | システム構成、パッケージ利用マップ |
| D-02 | Valkeyデータ設計仕様書 (r10) | データ構造、キー設計、Go構造体 |
| D-03 | Vector-API/ステートマシン設計書 (r5) | API仕様、EAP状態遷移、Vector Gateway経由接続 |
| D-04 | ログ仕様設計書 (r13) | event_id定義、ログフォーマット、IMSIマスキング |
| D-05 | Acct Server詳細設計書 (r5) | セッション管理連携、Accounting処理 |
| D-06 | エラーハンドリング詳細設計書 (r6) | エラー分類、タイムアウト、Circuit Breaker |
| D-07 | TUI管理ツール詳細設計書 (r3) | 管理用TUIアプリケーション |
| D-08 | デプロイメント設計書 (r10) (予定) | Docker Compose構成、環境変数 |
| D-12 | Vector Gateway詳細設計書 (r2) | Gateway API仕様、ルーティング、IMSIマスキング |
| E-02 | コーディング規約（簡易版） | コーディング規約 |
| E-03 | テストガイドライン (r2) | テスト方針、カバレッジ基準 |

### 1.4 PoC対象外機能

以下の機能は本PoCでは実装対象外とする。

| 機能 | RFC | 説明 | 備考 |
|------|-----|------|------|
| 仮名認証 | RFC 4187/5448 | Pseudonym IDによる認証 | 受信時はフル認証へ誘導 |
| 高速再認証 | RFC 4187/5448 | Fast Re-authentication | 受信時はフル認証へ誘導 |
| EAP-Notification | RFC 4187 | 通知メッセージ送信 | 使用しない |
| RadSec | RFC 6614 | RADIUS over TLS | UDP のみ対応 |
| Chargeable User Identity | RFC 4372 | 課金用ユーザー識別子 | PoC完了後に検討 |
| AT_KDFネゴシエーション | RFC 5448 | KDF=1以外のサポート | KDF=1のみ対応 |
| EAP-SIM | RFC 4186 | SIMベース認証 | 受信時はEAP-Failure |

### 1.5 準拠規格

| 規格 | 内容 | 対応範囲 |
|------|------|---------|
| RFC 2865 | RADIUS | Access-Request/Accept/Reject/Challenge, Proxy-State |
| RFC 3579 | RADIUS EAP Support | EAP-Message, Message-Authenticator, State |
| RFC 3748 | EAP | EAPフレームワーク |
| RFC 4187 | EAP-AKA | EAP-AKA認証（フル認証のみ） |
| RFC 5448 | EAP-AKA' | EAP-AKA'認証（フル認証のみ、KDF=1） |
| RFC 5997 | Status-Server | ヘルスチェック応答 |

### 1.6 用語定義

| 用語 | 説明 |
|------|------|
| EAP-AKA | EAP Method for 3rd Generation Authentication and Key Agreement |
| EAP-AKA' | Improved EAP-AKA（SHA-256ベースの鍵導出） |
| IMSI | International Mobile Subscriber Identity（15桁） |
| RAND | 認証用乱数（128bit） |
| AUTN | 認証トークン（128bit） |
| XRES | 期待応答値（Expected Response） |
| CK/IK | 暗号鍵/完全性鍵（Cipher Key / Integrity Key） |
| CK'/IK' | EAP-AKA'用の導出済み鍵 |
| MSK | Master Session Key（MPPE-Key導出元） |
| K_aut | AT_MAC計算用の認証キー |
| PRF' | EAP-AKA'で使用するHMAC-SHA-256ベースの疑似乱数関数 |
| ANID | Access Network Identity（AT_KDF_INPUTの値） |
| Trace ID | リクエスト追跡用UUID（ログ・State属性・Valkeyキーで統一使用） |
| Session UUID | セッション管理用UUID（RFC 4122準拠、36文字、Class属性に格納） |

---

## ■セクション2: パッケージ構成

### 2.1 ディレクトリ構造

```
apps/auth-server/
├── main.go                        # エントリーポイント
└── internal/
    ├── config/
    │   ├── config.go              # 環境変数読み込み、設定構造体
    │   ├── config_test.go         # config テスト
    │   └── constants.go           # 定数定義
    ├── eap/
    │   ├── constants.go           # EAP関連定数
    │   ├── errors.go              # EAPエラー定義
    │   ├── identity.go            # Identity処理、種別判定
    │   ├── identity_test.go       # identity テスト
    │   ├── packet.go              # EAPパケットパース・構築
    │   ├── packet_test.go         # packet テスト
    │   ├── statemachine.go        # ステートマシン制御
    │   ├── statemachine_test.go   # statemachine テスト
    │   ├── types.go               # EAP型定義
    │   ├── aka/
    │   │   ├── challenge.go       # EAP-AKA Challenge生成・検証
    │   │   ├── challenge_test.go  # challenge テスト
    │   │   ├── keys.go            # MK/MSK/K_aut導出、AT_MAC計算
    │   │   └── keys_test.go       # keys テスト
    │   └── akaprime/
    │       ├── challenge.go       # EAP-AKA' Challenge生成・検証
    │       ├── challenge_test.go  # challenge テスト
    │       ├── keys.go            # CK'/IK'/MK'/MSK/K_aut導出、AT_MAC計算
    │       └── keys_test.go       # keys テスト
    ├── engine/
    │   ├── engine.go              # 認証エンジン（EAP処理オーケストレーション）
    │   └── engine_test.go         # engine テスト
    ├── logging/
    │   ├── mask.go                # IMSIマスキング
    │   └── mask_test.go           # mask テスト
    ├── mocks/
    │   ├── eap_mock.go            # EAPモック
    │   ├── policy_mock.go         # ポリシーモック
    │   ├── session_mock.go        # セッションモック
    │   ├── store_mock.go          # ストアモック
    │   └── vector_mock.go         # ベクターモック
    ├── policy/
    │   ├── avp.go                 # VLAN/Timeout AVP生成
    │   ├── avp_test.go            # avp テスト
    │   ├── errors.go              # ポリシーエラー定義
    │   ├── evaluator.go           # ポリシー評価ロジック
    │   ├── evaluator_test.go      # evaluator テスト
    │   ├── interfaces.go          # ポリシーインターフェース
    │   └── types.go               # ポリシー型定義
    ├── radius/
    │   ├── authenticator.go       # Message-Authenticator検証・生成
    │   ├── authenticator_test.go  # authenticator テスト
    │   ├── errors.go              # RADIUSエラー定義
    │   ├── packet.go              # RADIUSパケット操作ヘルパー
    │   ├── packet_test.go         # packet テスト
    │   ├── proxystate.go          # Proxy-State処理
    │   ├── proxystate_test.go     # proxystate テスト
    │   ├── response.go            # 応答パケット生成
    │   ├── response_test.go       # response テスト
    │   ├── status.go              # Status-Server処理
    │   └── status_test.go         # status テスト
    ├── server/
    │   ├── handler.go             # radius.Handler実装、処理振り分け
    │   ├── handler_test.go        # handler テスト
    │   ├── secret.go              # radius.SecretSource実装
    │   ├── secret_test.go         # secret テスト
    │   ├── server.go              # PacketServer設定・起動・シャットダウン
    │   └── server_test.go         # server テスト
    ├── session/
    │   ├── context.go             # EAPコンテキスト管理
    │   ├── context_test.go        # context テスト
    │   ├── errors.go              # セッションエラー定義
    │   ├── interfaces.go          # セッションインターフェース
    │   ├── session.go             # セッション作成
    │   └── session_test.go        # session テスト
    ├── store/
    │   ├── client.go              # RADIUSクライアントデータアクセス
    │   ├── convert.go             # Valkey Hash ↔ struct変換ユーティリティ
    │   ├── convert_test.go        # convert テスト
    │   ├── errors.go              # ストアエラー定義
    │   ├── interfaces.go          # ストアインターフェース
    │   ├── keys.go                # Valkeyキープレフィックス定数
    │   ├── policy.go              # ポリシーデータアクセス
    │   ├── policy_test.go         # policy テスト
    │   ├── store_test.go          # store テスト
    │   └── valkey.go              # Valkeyクライアント初期化
    └── vector/
        ├── client.go              # Vector Gateway HTTPクライアント
        ├── client_test.go         # client テスト
        ├── constants.go           # HTTPヘッダ定数
        ├── errors.go              # Vector Gatewayエラー定義
        ├── interfaces.go          # ベクターインターフェース
        └── types.go               # ベクター型定義
```

### 2.2 パッケージ依存関係

```
main.go
    │
    └── internal/config
            │
            ▼
    ┌───────────────────────────────────────────────────────┐
    │                  internal/server                      │
    │  ┌─────────────────────────────────────────────────┐  │
    │  │  server.go (PacketServer)                       │  │
    │  │      │                                          │  │
    │  │      ├── secret.go (SecretSource)               │  │
    │  │      │       └── store/ (Valkey経由でclient取得)│  │
    │  │      │                                          │  │
    │  │      └── handler.go (radius.Handler)            │  │
    │  │              │                                  │  │
    │  │  ┌──────────┴─────────────┐                     │  │
    │  │  │                        │                     │  │
    │  │  ▼                        ▼                     │  │
    │  │ radius/              engine/                    │  │
    │  │  │                    │                         │  │
    │  │  │          ┌─────────┼──────────┐              │  │
    │  │  │          ▼         ▼          ▼              │  │
    │  │  │        eap/    vector/    policy/            │  │
    │  │  │     ┌────┴────┐                             │  │
    │  │  │     │         │                             │  │
    │  │  │  eap/aka/ eap/akaprime/                     │  │
    │  └──┼─────┴─────────┴─────────────────────────────┘  │
    │     │                                                │
    │     │      ┌───────────────┐   ┌──────────────┐      │
    │     │      │   session/    │   │   logging/   │      │
    │     │      └───────┬───────┘   └──────────────┘      │
    │     │              │                                 │
    │     │              ▼                                 │
    │     │      ┌───────────────┐                         │
    │     └─────►│    store/     │                         │
    │            └───────────────┘                         │
    └──────────────────────────────────────────────────────┘

    mocks/ はテスト用モックパッケージ（各インターフェースのモック実装を提供）
```

### 2.3 パッケージ責務一覧

| パッケージ | 責務 | 主要な型・関数 |
|-----------|------|---------------|
| `config` | 環境変数読み込み、設定値管理、定数定義 | `Config`, `Load()` |
| `server` | PacketServer管理、SecretSource実装、radius.Handler実装 | `Server`, `SecretSource`, `Handler` |
| `radius` | RADIUSパケット処理（パース補助、応答生成、Proxy-State、Status-Server） | `ResponseBuilder`, `ProxyStateHandler` |
| `eap` | EAPパケット解析、ステートマシン制御、型・定数・エラー定義 | `Parser`, `StateMachine`, `IdentityParser` |
| `eap/aka` | EAP-AKA固有処理（Challenge生成・検証、鍵導出・MAC計算） | `ChallengeBuilder`, `KeyDeriver` |
| `eap/akaprime` | EAP-AKA'固有処理（Challenge生成・検証、鍵導出・MAC計算） | `ChallengeBuilder`, `KeyDeriver` |
| `engine` | 認証エンジン（EAP処理のオーケストレーション） | `Engine` |
| `logging` | IMSIマスキングなどのログユーティリティ | `MaskIMSI()` |
| `mocks` | テスト用モック実装 | 各インターフェースのモック |
| `vector` | Vector Gateway連携 | `Client` |
| `policy` | 認可ポリシー評価（SSIDマッチング、Action/時間帯条件）、AVP生成 | `Evaluator`, `AVPBuilder` |
| `session` | EAPコンテキスト・セッション管理 | `ContextManager`, `SessionManager` |
| `store` | Valkeyアクセス抽象化、Hash↔struct変換 | `ValkeyClient`, `ClientStore`, `PolicyStore`, `Convert` |

### 2.4 外部パッケージ依存

D-01で定義されたパッケージ利用マップに基づく。

| カテゴリ | パッケージ | 用途 | 利用箇所 |
|---------|-----------|------|---------|
| **RADIUS** | `layeh.com/radius` | RADIUSプロトコル処理、PacketServer | `server/`, `radius/` |
| **EAP** | `github.com/oyaguma3/go-eapaka` | EAP-AKAパケット処理 | `eap/` |
| **HTTP** | `github.com/go-resty/resty/v2` | HTTPクライアント | `vector/` |
| **DB** | `github.com/redis/go-redis/v9` | Valkeyクライアント | `store/` |
| **Config** | `github.com/kelseyhightower/envconfig` | 環境変数読み込み | `config/` |
| **UUID** | `github.com/google/uuid` | Trace ID生成 | `server/` |
| **Logging** | `log/slog` (標準ライブラリ) | 構造化ログ | 全パッケージ |
| **Crypto** | `crypto/hmac`, `crypto/sha1`, `crypto/sha256` | MAC計算 | `eap/aka/`, `eap/akaprime/` |

### 2.5 パッケージ間インターフェース

レイヤー間の依存を疎結合に保つため、主要なインターフェースを定義する。

```go
// session/interfaces.go
type ContextStore interface {
    Create(ctx context.Context, traceID string, eapCtx *EAPContext) error
    Get(ctx context.Context, traceID string) (*EAPContext, error)
    Update(ctx context.Context, traceID string, updates map[string]interface{}) error
    Delete(ctx context.Context, traceID string) error
}

type SessionStore interface {
    Create(ctx context.Context, sessionID string, sess *Session) error
    AddUserIndex(ctx context.Context, imsi string, sessionID string) error
}

// vector/interfaces.go
type VectorClient interface {
    GetVector(ctx context.Context, req *VectorRequest) (*VectorResponse, error)
}

// policy/interfaces.go
type PolicyStore interface {
    GetPolicy(ctx context.Context, imsi string) (*Policy, error)
}

// store/interfaces.go
type ClientStore interface {
    GetClientSecret(ctx context.Context, ip string) (string, error)
}
```

**注記:** `server/secret.go` は `layeh.com/radius` の `SecretSource` インターフェースを直接実装する。内部で `store.ClientStore` を利用してValkey検索を行う。

### 2.6 Dockerfile方針

#### 2.6.1 マルチステージビルド構成

```dockerfile
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
```

#### 2.6.2 ベースイメージ選定

| ステージ   | イメージ               | 理由                                                 |
| ---------- | ---------------------- | ---------------------------------------------------- |
| ビルド     | `golang:1.25-bookworm` | Go 1.25.x、Debian Bookwormベース                     |
| ランタイム | `debian:bookworm-slim` | 最小構成、将来のHTTPヘルスエンドポイント追加に備える |

#### 2.6.3 必須パッケージ

| パッケージ        | 用途                                               |
| ----------------- | -------------------------------------------------- |
| `ca-certificates` | TLS証明書（Vector Gateway経由のHTTPS通信に備える） |
| `curl`            | 将来のHTTPヘルスチェック対応に備える               |
| `procps`          | ヘルスチェック用（`pgrep`コマンド提供）            |

> **注記:** Auth ServerはUDPサービスのため、現時点ではプロセス存在確認（`pgrep`）でヘルスチェックを行う。`pgrep` は `procps` パッケージに含まれる。将来的にHTTPヘルスエンドポイントを追加する場合は `curl -fsS` を使用する。

### 2.7 ファイル別責務詳細

#### `internal/server/`

| ファイル | 責務 | 主要関数・型 |
|---------|------|-------------|
| `server.go` | PacketServer初期化・起動・シャットダウン | `Server`, `New()`, `ListenAndServe()`, `Shutdown()` |
| `handler.go` | `radius.Handler`実装、Code判定、処理振り分け | `Handler`, `ServeRADIUS()` |
| `secret.go` | `radius.SecretSource`実装、Valkey/フォールバック解決 | `SecretSource`, `RADIUSSecret()` |

#### `internal/radius/`

| ファイル | 責務 | 主要関数 |
|---------|------|---------|
| `packet.go` | AVP抽出・設定ヘルパー | `GetEAPMessage()`, `GetNASIdentifier()`, `GetCalledStationID()`, `GetState()` |
| `authenticator.go` | Message-Authenticator処理 | `Verify()`, `Sign()` |
| `proxystate.go` | Proxy-State属性処理 | `Extract()`, `Apply()` |
| `response.go` | 応答パケット構築 | `BuildAccept()`, `BuildReject()`, `BuildChallenge()` |
| `status.go` | Status-Server応答 | `HandleStatusServer()` |

#### `internal/eap/`

| ファイル | 責務 | 主要関数 |
|---------|------|---------|
| `constants.go` | EAP関連定数定義 | ステージ定数、Identity接頭辞定数 |
| `errors.go` | EAPエラー定義 | `ErrInvalidIdentity`, `ErrMACInvalid` 等 |
| `identity.go` | Identity解析・種別判定 | `ParseIdentity()`, `DetermineEAPType()`, `ExtractIMSI()` |
| `packet.go` | EAPパケット解析・構築 | `Parse()`, `GetType()`, `GetSubtype()`, `BuildEAPSuccess()`, `BuildEAPFailure()` |
| `statemachine.go` | 状態遷移制御 | `Process()`, `HandleIdentity()`, `HandleChallenge()` |
| `types.go` | EAP型定義 | `IdentityType`, `ParsedIdentity` |

#### `internal/eap/aka/` と `internal/eap/akaprime/`

| ファイル | 責務 | aka | akaprime差異 |
|---------|------|-----|-------------|
| `challenge.go` | Challenge生成・検証 | AT_RAND, AT_AUTN, AT_MAC | +AT_KDF, AT_KDF_INPUT |
| `keys.go` | 鍵導出・AT_MAC計算 | MK→MSK,K_aut (HMAC-SHA-1-128) | CK'/IK'→MK'→MSK,K_aut (HMAC-SHA-256-128), PRF' |

> **注記（r9変更）：** 旧設計の`mac.go`（AT_MAC計算）および`prf.go`（PRF'関数）は`keys.go`に統合された。`go-eapaka`パッケージが`CalculateAndSetMac()`/`VerifyMac()`メソッドを提供しているため、個別ファイルでの自前実装は不要となった。

#### `internal/store/`

| ファイル | 責務 | 主要関数・型 |
|---------|------|-------------|
| `valkey.go` | Valkeyクライアント初期化・管理 | `ValkeyClient`, `NewValkeyClient()` |
| `client.go` | RADIUSクライアントデータアクセス | `ClientStore`, `GetClientSecret()` |
| `convert.go` | Valkey Hash ↔ struct変換ユーティリティ | `structToMap()`, `mapToStruct()` |
| `errors.go` | ストアエラー定義 | `ErrValkeyUnavailable`, `ErrKeyNotFound` |
| `interfaces.go` | ストアインターフェース | `ClientStore` |
| `keys.go` | Valkeyキープレフィックス定数 | `KeyPrefix*` |
| `policy.go` | ポリシーデータアクセス | `PolicyStore`, `GetPolicy()` |

#### `internal/engine/`

| ファイル | 責務 | 主要関数・型 |
|---------|------|-------------|
| `engine.go` | 認証エンジン（EAP処理のオーケストレーション） | `Engine`, 各サービス層の統合制御 |

#### `internal/logging/`

| ファイル | 責務 | 主要関数・型 |
|---------|------|-------------|
| `mask.go` | IMSIマスキング | `MaskIMSI()` |

#### `internal/mocks/`

| ファイル | 責務 |
|---------|------|
| `eap_mock.go` | EAPインターフェースのモック実装 |
| `policy_mock.go` | ポリシーインターフェースのモック実装 |
| `session_mock.go` | セッションインターフェースのモック実装 |
| `store_mock.go` | ストアインターフェースのモック実装 |
| `vector_mock.go` | ベクターインターフェースのモック実装 |

---

## ■セクション3: 設定・初期化

### 3.1 環境変数一覧

| 環境変数 | 必須 | デフォルト | 型 | 説明 |
|---------|------|-----------|-----|------|
| `REDIS_HOST` | Yes | - | string | Valkeyホスト名 |
| `REDIS_PORT` | Yes | - | string | Valkeyポート番号 |
| `REDIS_PASS` | Yes | - | string | Valkeyパスワード |
| `VECTOR_API_URL` | Yes | - | string | Vector Gateway エンドポイントURL（例: `http://vector-gateway:8080/api/v1/vector`）。D-03参照。 |
| `RADIUS_SECRET` | No | - | string | フォールバックShared Secret |
| `LISTEN_ADDR` | No | `:1812` | string | UDPリッスンアドレス |
| `EAP_AKA_PRIME_NETWORK_NAME` | No | `WLAN` | string | EAP-AKA' AT_KDF_INPUT値（ANID） |
| `LOG_MASK_IMSI` | No | `true` | bool | IMSIマスキング有効化（ログ出力時） |
> **注記:** 環境変数名 `RADIUS_SECRET` はシステム全体で統一されている。D-01およびD-08の `.env` ファイルでも同名を使用すること。

### 3.2 設定構造体

```go
// internal/config/config.go

package config

import (
    "fmt"
    "time"

    "github.com/kelseyhightower/envconfig"
)

type Config struct {
    // Valkey接続設定
    RedisHost string `envconfig:"REDIS_HOST" required:"true"`
    RedisPort string `envconfig:"REDIS_PORT" required:"true"`
    RedisPass string `envconfig:"REDIS_PASS" required:"true"`

    // Vector Gateway設定
    VectorAPIURL string `envconfig:"VECTOR_API_URL" required:"true"`

    // RADIUS設定
    RadiusSecret string `envconfig:"RADIUS_SECRET"`
    ListenAddr   string `envconfig:"LISTEN_ADDR" default:":1812"`

    // EAP-AKA'設定
    NetworkName string `envconfig:"EAP_AKA_PRIME_NETWORK_NAME" default:"WLAN"`

    // ログ設定
    LogMaskIMSI bool `envconfig:"LOG_MASK_IMSI" default:"true"`
}

// 定数（コードに埋め込み）
const (
    // Valkeyタイムアウト（D-06準拠）
    ValkeyConnectTimeout = 3 * time.Second
    ValkeyCommandTimeout = 2 * time.Second

    // Vector Gatewayタイムアウト（D-06準拠）
    VectorConnectTimeout = 2 * time.Second
    VectorRequestTimeout = 5 * time.Second

    // EAPコンテキストTTL（D-02準拠）
    EAPContextTTL = 60 * time.Second

    // シャットダウンタイムアウト
    ShutdownTimeout = 5 * time.Second
)

func Load() (*Config, error) {
    var cfg Config
    if err := envconfig.Process("", &cfg); err != nil {
        return nil, fmt.Errorf("failed to load config: %w", err)
    }
    return &cfg, nil
}

func (c *Config) ValkeyAddr() string {
    return fmt.Sprintf("%s:%s", c.RedisHost, c.RedisPort)
}
```

### 3.3 初期化シーケンス

```
┌─────────────────────────────────────────────────────────────┐
│                      main.go                                │
└─────────────────────────────────────────────────────────────┘
                            │
        1. 環境変数読み込み │
                            ▼
                    ┌───────────────┐
                    │ config.Load() │
                    └───────┬───────┘
                            │ 失敗時: ログ出力して終了
                            │
        2. ロガー初期化     │
                            ▼
                    ┌───────────────────────┐
                    │ slog.SetDefault(...)  │
                    │ JSON形式、INFO以上    │
                    └───────────┬───────────┘
                                │
        3. Valkeyクライアント  │
           初期化              ▼
                    ┌───────────────────────┐
                    │ store.NewValkeyClient │
                    │ - 接続確認 (PING)     │
                    └───────────┬───────────┘
                                │ 失敗時: ログ出力して終了
                                │
        4. Vector Gateway      │
           クライアント初期化  ▼
                    ┌───────────────────────┐
                    │ vector.NewClient      │
                    │ - HTTPクライアント    │
                    │ - Circuit Breaker     │
                    └───────────┬───────────┘
                                │
        5. サービス層初期化    │
                               ▼
                    ┌───────────────────────┐
                    │ store.NewClientStore  │
                    │ store.NewPolicyStore  │
                    │ session.NewManager    │
                    │ policy.NewEvaluator   │
                    └───────────┬───────────┘
                                │
        6. PacketServer        │
           初期化・起動        ▼
                    ┌───────────────────────┐
                    │ server.NewSecretSource│
                    │ server.NewHandler     │
                    │ server.New            │
                    │ srv.ListenAndServe()  │
                    └───────────┬───────────┘
                                │
        7. シグナルハンドラ    │
           登録・待機          ▼
                    ┌───────────────────────┐
                    │ SIGTERM/SIGINT待機    │
                    │ → srv.Shutdown(ctx)   │
                    └───────────────────────┘
```

### 3.4 main.go 実装

```go
// main.go

package main

import (
    "context"
    "log/slog"
    "os"
    "os/signal"
    "syscall"
    "time"

    "auth-server/internal/config"
    "auth-server/internal/policy"
    "auth-server/internal/server"
    "auth-server/internal/session"
    "auth-server/internal/store"
    "auth-server/internal/vector"
)

func main() {
    // 1. 環境変数読み込み
    cfg, err := config.Load()
    if err != nil {
        slog.Error("failed to load config", "error", err)
        os.Exit(1)
    }

    // 2. ロガー初期化
    logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))
    slog.SetDefault(logger)

    slog.Info("starting auth-server",
        "listen_addr", cfg.ListenAddr,
        "vector_api_url", cfg.VectorAPIURL,
        "network_name", cfg.NetworkName)

    // 3. Valkeyクライアント初期化
    valkeyClient, err := store.NewValkeyClient(cfg)
    if err != nil {
        slog.Error("failed to connect to valkey",
            "event_id", "VALKEY_CONN_ERR",
            "error", err)
        os.Exit(1)
    }
    defer valkeyClient.Close()

    // 4. Vector Gatewayクライアント初期化
    vectorClient := vector.NewClient(cfg)

    // 5. サービス層初期化
    clientStore := store.NewClientStore(valkeyClient)
    policyStore := store.NewPolicyStore(valkeyClient)
    contextStore := session.NewContextStore(valkeyClient)
    sessionStore := session.NewSessionStore(valkeyClient)

    policyEvaluator := policy.NewEvaluator(policyStore)
    sessionManager := session.NewManager(contextStore, sessionStore)

    // 6. PacketServer初期化・起動
    secretSource := server.NewSecretSource(clientStore, cfg.RadiusSecret)
    handler := server.NewHandler(server.Dependencies{
        VectorClient:    vectorClient,
        PolicyEvaluator: policyEvaluator,
        SessionManager:  sessionManager,
        Config:          cfg,
    })
    srv := server.New(cfg.ListenAddr, secretSource, handler)

    go func() {
        slog.Info("starting RADIUS server", "addr", cfg.ListenAddr)
        if err := srv.ListenAndServe(); err != nil {
            slog.Error("server error", "error", err)
        }
    }()

    // 7. シグナルハンドラ
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

    sig := <-sigCh
    slog.Info("received signal, shutting down", "signal", sig)

    ctx, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
    defer cancel()

    if err := srv.Shutdown(ctx); err != nil {
        slog.Warn("shutdown error", "error", err)
    }

    slog.Info("auth-server stopped")
}
```

### 3.5 IMSIマスキング設定

セキュリティ上、ログに出力するIMSIは中央部分をマスクする。D-04「ログ仕様設計書」で定義された仕様に準拠する。

#### 3.5.1 環境変数による制御

| 環境変数 | デフォルト | 説明 |
|---------|-----------|------|
| `LOG_MASK_IMSI` | `true` | `false` でマスキング無効化（デバッグ用） |

**用途:**
- **本番環境**: `LOG_MASK_IMSI=true`（デフォルト）でプライバシー保護
- **開発・デバッグ環境**: `LOG_MASK_IMSI=false` で問題調査時にIMSI全桁を確認可能

#### 3.5.2 マスキング仕様

| 設定値 | 動作 | 出力例（入力: `440101234567890`） |
|--------|------|--------------------------------|
| `true`（デフォルト） | 先頭6桁 + マスク + 末尾1桁 | `440101********0` |
| `false` | マスクなし（全桁表示） | `440101234567890` |

#### 3.5.3 実装

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

#### 3.5.4 適用箇所

Auth Serverにおいて、以下のevent_idを含むログ出力時にマスキングを適用する。

| event_id | 出力箇所 | imsiフィールド |
|----------|---------|---------------|
| `AUTH_OK` | 認証成功時 | マスキング対象 |
| `AUTH_RES_MISMATCH` | AT_RES不一致時 | マスキング対象 |
| `AUTH_MAC_INVALID` | AT_MAC検証失敗時 | マスキング対象 |
| `AUTH_IMSI_NOT_FOUND` | IMSI未登録時 | マスキング対象 |
| `AUTH_POLICY_NOT_FOUND` | ポリシー未設定時 | マスキング対象 |
| `AUTH_POLICY_DENIED` | ポリシー拒否時 | マスキング対象 |
| `AUTH_RESYNC_LIMIT` | 再同期上限超過時 | マスキング対象 |
| `SESSION_CREATED` | セッション作成時 | マスキング対象 |

**実装例:**

```go
// 認証成功時のログ出力
slog.Info("authentication successful",
    "event_id", "AUTH_OK",
    "trace_id", traceID,
    "imsi", logging.MaskIMSI(imsi, cfg.LogMaskIMSI),
    "session_uuid", sessionID,
    "latency_ms", latency.Milliseconds())
```

#### 3.5.5 注意事項

- Auth Serverでは、IMSIは認証フロー全体（Identity受信、Vector API呼び出し、ポリシー評価、セッション作成）で使用されるため、マスキング設定の影響範囲が広い
- `LOG_MASK_IMSI=false` の設定は、ログファイルへのアクセス制御が適切に行われている環境でのみ使用すること
- デバッグ目的でマスキングを無効化した場合、調査完了後は速やかに `true` に戻すこと

### 3.6 Valkeyクライアント初期化

```go
// internal/store/valkey.go

package store

import (
    "context"
    "fmt"

    "github.com/redis/go-redis/v9"

    "auth-server/internal/config"
)

type ValkeyClient struct {
    *redis.Client
}

func NewValkeyClient(cfg *config.Config) (*ValkeyClient, error) {
    client := redis.NewClient(&redis.Options{
        Addr:         cfg.ValkeyAddr(),
        Password:     cfg.RedisPass,
        DB:           0,
        DialTimeout:  config.ValkeyConnectTimeout,
        ReadTimeout:  config.ValkeyCommandTimeout,
        WriteTimeout: config.ValkeyCommandTimeout,
        PoolSize:     10,
        MinIdleConns: 2,
    })

    // 接続確認
    ctx, cancel := context.WithTimeout(context.Background(), config.ValkeyConnectTimeout)
    defer cancel()

    if err := client.Ping(ctx).Err(); err != nil {
        client.Close()
        return nil, fmt.Errorf("valkey ping failed: %w", err)
    }

    return &ValkeyClient{Client: client}, nil
}
```

### 3.7 Vector Gatewayクライアント初期化

```go
// internal/vector/client.go

package vector

import (
    "github.com/go-resty/resty/v2"

    "auth-server/internal/config"
)

type Client struct {
    httpClient *resty.Client
    baseURL    string
}

func NewClient(cfg *config.Config) *Client {
    // HTTPクライアント設定
    httpClient := resty.New().
        SetTimeout(config.VectorRequestTimeout).
        SetHeader("Content-Type", "application/json")

    return &Client{
        httpClient: httpClient,
        baseURL:    cfg.VectorAPIURL,
    }
}
```

> **注記（r9変更）：** Circuit Breaker（`gobreaker`パッケージ）は現在のPoC実装では使用しない。将来的に必要になった場合に再導入する。

### 3.8 slogロガー設定

**ログ出力形式（JSON）：** D-04 ログ仕様設計書に準拠

```json
{
    "time": "2026-01-09T10:00:00.123Z",
    "level": "INFO",
    "msg": "starting auth-server",
    "listen_addr": ":1812",
    "vector_api_url": "http://vector-gateway:8080/api/v1/vector",
    "network_name": "WLAN"
}
```

**ログ属性の付与方法：**

- リクエスト単位で `trace_id` を付与
- `context.WithValue` でTrace IDを伝搬
- 各処理層でロガーを生成する際に `slog.With("trace_id", traceID)` を使用

### 3.9 シャットダウン処理

**方針：**

- `PacketServer.Shutdown(ctx)` を使用
- contextにタイムアウト（5秒）を設定
- 処理中リクエストの完了を待機
- タイムアウト時は警告ログを出力

**処理フロー：**

```
SIGTERM/SIGINT受信
    │
    ├── context.WithTimeout(5秒)
    │
    ├── srv.Shutdown(ctx)
    │       │
    │       ├── 新規リクエスト受付停止
    │       ├── 処理中リクエスト完了待機
    │       └── タイムアウト時はエラー返却
    │
    └── ログ出力、終了
```

### 3.10 設定値の妥当性検証

起動時に追加の検証を行う。

```go
// internal/config/config.go（追加）

func (c *Config) Validate() error {
    // Network Name検証（空でないこと）
    if c.NetworkName == "" {
        return fmt.Errorf("EAP_AKA_PRIME_NETWORK_NAME must not be empty")
    }

    // Vector API URL形式検証
    if !strings.HasPrefix(c.VectorAPIURL, "http://") &&
       !strings.HasPrefix(c.VectorAPIURL, "https://") {
        return fmt.Errorf("VECTOR_API_URL must start with http:// or https://")
    }

    return nil
}
```

---

## ■セクション4: UDPサーバー

### 4.1 概要

| 項目              | 内容                                 |
| ----------------- | ------------------------------------ |
| 使用パッケージ    | `layeh.com/radius` の `PacketServer` |
| プロトコル        | UDP                                  |
| リッスンポート    | 1812（環境変数で変更可）             |
| Shared Secret解決 | `SecretSource` インターフェース実装  |

### 4.2 処理フロー

```
PacketServer
    │
    ├── パケット受信（内部処理）
    │
    ├── SecretSource.RADIUSSecret() 呼び出し
    │       ├── client:{IP} 検索
    │       ├── ヒット → その secret
    │       └── ミス → 環境変数 RADIUS_SECRET（フォールバック）
    │
    ├── パケットパース・検証（内部処理）
    │
    └── Handler.ServeRADIUS() 呼び出し
            ├── Trace ID生成
            ├── Code判定・処理振り分け
            │       ├── Code=1  → EAP認証処理
            │       ├── Code=12 → Status-Server処理
            │       └── その他  → ログ出力、破棄
            └── 応答パケット返却
```

### 4.3 実装方針

#### SecretSource実装

- `SecretSource` インターフェースを実装した構造体を作成
- Valkeyから `client:{IP}` を検索
- 見つからない場合は環境変数のフォールバック値を使用
- 両方なければエラー返却（PacketServerがパケット破棄）

#### Handler実装

- `radius.Handler` インターフェースを実装
- `ServeRADIUS(w ResponseWriter, r *Request)` メソッドで処理
- Trace ID生成は `ServeRADIUS` の冒頭で実施

#### シャットダウン

- `PacketServer.Shutdown(ctx)` を使用
- context.Cancelでgraceful shutdown

### 4.4 実装上の注意点

#### SecretSource

- `remoteAddr` から `*net.UDPAddr` へのtype assertionが必要
- Valkey接続エラー時はフォールバックを試行し、それもなければエラー
- エラー時は `RADIUS_NO_SECRET` をログ出力

#### Handler

- `ResponseWriter.Write(packet)` で応答送信
- 応答不要の場合（パース失敗等）は `Write` を呼ばずにreturn
- パニックリカバリは `PacketServer` が処理

#### Trace ID

- `ServeRADIUS` 冒頭でUUID生成
- `context.WithValue` でTrace IDを伝搬
- 全てのログ出力に `trace_id` を付与

### 4.5 主要な型・関数

| 型・関数                     | 責務                                  |
| ---------------------------- | ------------------------------------- |
| `DynamicSecretSource` 構造体 | `SecretSource` 実装、IP別Secret解決   |
| `Handler` 構造体             | `radius.Handler` 実装、リクエスト処理 |
| `NewServer()`                | `PacketServer` 初期化                 |
| `Run(ctx)`                   | サーバー起動（`ListenAndServe`）      |

### 4.6 ログ出力

| タイミング     | event_id           | レベル          | 内容                   |
| -------------- | ------------------ | --------------- | ---------------------- |
| サーバー起動   | -                  | INFO            | リッスンアドレス       |
| Secret解決失敗 | `RADIUS_NO_SECRET` | WARN            | 送信元IP               |
| パケット受信   | `PKT_RECV`         | INFO            | 送信元IP、RADIUSコード |
| 処理完了       | `AUTH_OK` 等       | INFO/WARN/ERROR | 結果に応じたevent_id   |

### 4.7 依存関係

```
server/
├── server.go       ← layeh.com/radius (PacketServer)
├── secret.go       ← store/ (SecretSource実装)
└── handler.go      ← radius/, eap/, policy/, session/, vector/
```

---

## ■セクション5: RADIUS処理層

### 5.1 概要

本セクションでは、RADIUS受信からEAP処理層への橋渡しまでのRADIUSプロトコル処理を定義する。

**対象ファイル：**

- `internal/server/handler.go`
- `internal/server/secret.go`
- `internal/server/server.go`
- `internal/radius/packet.go`
- `internal/radius/authenticator.go`
- `internal/radius/proxystate.go`
- `internal/radius/response.go`
- `internal/radius/status.go`
- `internal/radius/errors.go`

### 5.2 SecretSource実装

**ファイル:** `internal/server/secret.go`

**責務：** `layeh.com/radius` の `SecretSource` インターフェース実装

**実装方針：**

- `RADIUSSecret(ctx, remoteAddr, raw)` メソッドを実装
- `remoteAddr` からIPアドレスを抽出（ポート番号除去）
- Valkey `client:{IP}` から `secret` フィールドを取得
- 未登録の場合はフォールバック値（環境変数 `RADIUS_SECRET`）を返却
- フォールバックも未設定の場合は `nil` を返却（PacketServerがパケット破棄）

**注意点：**

- `net.Addr` から IP 抽出時、`net.UDPAddr` へのtype assertionを使用
- Valkey エラー時（接続断等）はフォールバック値を使用し、ログ出力
- Secret 取得結果はログに出力しない（機密情報）
- `nil` 返却時のログは `RADIUS_NO_SECRET` イベント

**ログ出力：**

| 条件                  | event_id           | レベル              |
| --------------------- | ------------------ | ------------------- |
| Valkey検索成功        | -                  | DEBUG（出力しない） |
| フォールバック使用    | -                  | DEBUG               |
| Secret不明（nil返却） | `RADIUS_NO_SECRET` | WARN                |

### 5.3 Handler実装

**ファイル:** `internal/server/handler.go`

**責務：** `radius.Handler` インターフェース実装、処理振り分け

**インターフェース：**

```go
type Handler interface {
    ServeRADIUS(w radius.ResponseWriter, r *radius.Request)
}
```

**実装方針：**

- `ServeRADIUS` エントリ時点でTrace ID（UUID）を生成
- `r.Packet.Code` で処理を振り分け
- 応答は `w.Write(packet)` で返却

**Code別処理：**

| Code            | 値   | 処理               |
| --------------- | ---- | ------------------ |
| `AccessRequest` | 1    | EAP認証処理へ      |
| `StatusServer`  | 12   | Status-Server応答  |
| その他          | -    | ログ出力、応答なし |

**Trace ID生成・伝搬：**

- `google/uuid` で UUIDv4 を生成
- `context.WithValue` でコンテキストに格納
- 以降の全処理でこのコンテキストを使用
- ログ出力時は `slog.With("trace_id", traceID)` を使用

**注意点：**

- `r.RemoteAddr` はログ出力用に `src_ip` として記録
- Panic recover は PacketServer 側で行われるため、Handler内では不要
- `w.Write` のエラーはログ出力のみ（UDP送信失敗は復旧不可）

### 5.4 パケット処理ヘルパー

**ファイル:** `internal/radius/packet.go`

**責務：** AVP抽出・設定の共通処理

**主要関数：**

| 関数                     | 用途                   | 戻り値         |
| ------------------------ | ---------------------- | -------------- |
| `GetEAPMessage(p)`       | EAP-Message属性取得    | `[]byte, bool` |
| `GetState(p)`            | State属性取得          | `[]byte, bool` |
| `GetNASIdentifier(p)`    | NAS-Identifier取得     | `string, bool` |
| `GetNASIPAddress(p)`     | NAS-IP-Address取得     | `net.IP, bool` |
| `GetCalledStationID(p)`  | Called-Station-Id取得  | `string, bool` |
| `GetCallingStationID(p)` | Calling-Station-Id取得 | `string, bool` |
| `GetUserName(p)`         | User-Name取得          | `string, bool` |

**実装方針：**

- `layeh.com/radius` の `rfc2865` サブパッケージを活用
- 属性未存在時は `false` を返却（エラーではない）
- EAP-Message は複数属性の結合が必要（RFC 3579）

**EAP-Message結合の注意点：**

- EAP-Message は253バイト上限のため分割される場合がある
- 複数の EAP-Message 属性を受信順に結合
- `radius.Packet.Attributes.GetAll()` で全属性取得

### 5.5 Message-Authenticator処理

**ファイル:** `internal/radius/authenticator.go`

**責務：** Message-Authenticator属性の検証・生成

**検証処理（受信時）：**

1. Access-Request から Message-Authenticator 属性を取得
2. 属性値を16バイトのゼロで置換したパケットを構築
3. HMAC-MD5(Shared Secret, パケット全体) を計算
4. 計算結果と属性値を比較

**生成処理（送信時）：**

1. 応答パケットに Message-Authenticator 属性を追加（16バイトゼロ）
2. Authenticator フィールドを設定（Access-Challengeの場合はリクエストのAuthenticator）
3. HMAC-MD5(Shared Secret, パケット全体) を計算
4. Message-Authenticator 属性値を計算結果で上書き

**実装方針：**

- `layeh.com/radius` のパケットエンコード機能を活用
- `crypto/hmac` + `crypto/md5` を使用
- 検証失敗時は `false` を返却、呼び出し元でログ出力

**注意点：**

- EAP認証では Message-Authenticator は必須（RFC 3579）
- 属性が存在しない Access-Request は拒否
- Response の Authenticator 計算では Request の Authenticator を使用

### 5.6 Proxy-State処理

**ファイル:** `internal/radius/proxystate.go`

**責務：** Proxy-State属性の保持と応答への反映

**RFC 2865準拠要件：**

- 受信した Proxy-State 属性をすべて応答にコピー
- 受信順序を維持
- 値の解釈・変更は行わない

**実装方針：**

- リクエストから `Proxy-State` 属性を全て抽出
- 応答パケット構築時に同じ順序で追加
- 構造体で抽出結果を保持し、応答生成時に渡す

**主要型：**

```go
type ProxyStates struct {
    Values [][]byte
}

func Extract(p *radius.Packet) *ProxyStates
func (ps *ProxyStates) Apply(p *radius.Packet)
```

**注意点：**

- Access-Challenge → Access-Request の往復で Proxy-State が変わる可能性
- 各ラウンドトリップで最新のリクエストから抽出すること
- 空の場合も正常（Proxy-State なしのクライアントも存在）

### 5.7 レスポンス生成

**ファイル:** `internal/radius/response.go`

**責務：** RADIUS応答パケットの構築

**応答種別：**

| 種別             | Code | 用途     |
| ---------------- | ---- | -------- |
| Access-Accept    | 2    | 認証成功 |
| Access-Reject    | 3    | 認証失敗 |
| Access-Challenge | 11   | EAP継続  |

**共通処理：**

1. 応答パケット作成（Code, Identifier設定）
2. Proxy-State 属性追加
3. EAP-Message 属性追加（分割対応）
4. Message-Authenticator 生成・追加
5. Response Authenticator 計算

**Access-Accept 追加属性：**

- `EAP-Message`: EAP-Success
- `MS-MPPE-Recv-Key`, `MS-MPPE-Send-Key`: MSKから導出した鍵
- `Class`: セッションUUID
- `Tunnel-Type`, `Tunnel-Medium-Type`, `Tunnel-Private-Group-Id`: VLAN設定（ポリシー指定時）
- `Session-Timeout`: セッションタイムアウト（ポリシー指定時）

**Access-Challenge 追加属性：**

- `EAP-Message`: EAP-Request/AKA-Challenge 等
- `State`: Trace ID（EAPコンテキスト識別用）

**Access-Reject 追加属性：**

- `EAP-Message`: EAP-Failure

**EAP-Message分割の注意点：**

- 253バイト超の場合は複数属性に分割
- 分割位置は253バイト境界
- 順序を維持して追加

**MS-MPPE-Key暗号化：**

- RFC 2548 に準拠した暗号化が必要
- Salt（2バイト乱数）+ 暗号化データの形式
- `layeh.com/radius/vendors/microsoft` パッケージを活用

### 5.8 Status-Server対応

**ファイル:** `internal/radius/status.go`

**責務：** RFC 5997 Status-Server 応答

**処理フロー：**

1. Status-Server (Code=12) 受信
2. Message-Authenticator 検証
3. Access-Accept (Code=2) 応答

**実装方針：**

- Status-Server 専用のシンプルな処理
- EAP-Message は含まない
- Proxy-State は通常通り処理

**応答内容：**

- Code: Access-Accept (2)
- Proxy-State: リクエストからコピー
- Message-Authenticator: 生成して追加

**注意点：**

- Status-Server は認証処理ではなく死活監視用
- ログ出力は INFO レベル（`PKT_RECV` 相当）
- Valkey/Vector Gateway の状態確認は行わない（シンプルな応答）

### 5.9 処理フロー図

```
PacketServer
    │
    ├── SecretSource.RADIUSSecret()
    │       │
    │       ├── client:{IP} 検索
    │       ├── フォールバック判定
    │       └── Secret 返却（nil時はパケット破棄）
    │
    ├── パケットパース・検証（PacketServer内部）
    │
    └── Handler.ServeRADIUS()
            │
            ├── Trace ID 生成
            │
            ├── Code 判定
            │       │
            │       ├── [Code=1] Access-Request
            │       │       │
            │       │       ├── Message-Authenticator 検証
            │       │       │       └── 失敗時: ログ出力、応答なし
            │       │       │
            │       │       ├── Proxy-State 抽出
            │       │       │
            │       │       ├── EAP-Message 抽出・結合
            │       │       │
            │       │       └── EAP処理層へ（セクション6）
            │       │
            │       ├── [Code=12] Status-Server
            │       │       │
            │       │       ├── Message-Authenticator 検証
            │       │       │
            │       │       └── Access-Accept 応答
            │       │
            │       └── [その他]
            │               └── ログ出力、応答なし
            │
            └── 応答パケット送信
```

### 5.10 ログ出力仕様

| 処理                          | event_id              | レベル | 追加フィールド          |
| ----------------------------- | --------------------- | ------ | ----------------------- |
| Access-Request受信            | `PKT_RECV`            | INFO   | `src_ip`, `packet_code` |
| Status-Server受信             | `PKT_RECV`            | INFO   | `src_ip`, `packet_code` |
| Message-Authenticator検証失敗 | `RADIUS_AUTH_ERR`     | WARN   | `src_ip`                |
| Secret不明                    | `RADIUS_NO_SECRET`    | WARN   | `src_ip`                |
| 未知のCode                    | `RADIUS_UNKNOWN_CODE` | WARN   | `src_ip`, `code`        |

### 5.11 実装時の注意点まとめ

**全般：**

- `layeh.com/radius` のAPI仕様を熟読すること
- `rfc2865`, `rfc2866`, `rfc3579` サブパッケージを活用
- Microsoft VSA用に `vendors/microsoft` パッケージを使用

**SecretSource：**

- Valkey接続断時のフォールバック処理を確実に実装
- goroutine-safe な実装（複数リクエスト同時処理）

**Message-Authenticator：**

- 検証と生成で Authenticator フィールドの扱いが異なる点に注意
- Request は Request Authenticator を使用
- Response は Request Authenticator を使用（Response Authenticator ではない）

**Proxy-State：**

- 順序維持が重要
- 各ラウンドトリップで最新のものを使用

**EAP-Message：**

- 253バイト分割を忘れずに実装
- 受信時の結合、送信時の分割の両方が必要

------

## ■セクション6: EAP処理層

### 6.1 概要

本セクションでは、EAP-AKA/AKA'認証プロトコルの処理を定義する。D-03「EAP-AKAステートマシン設計書」に基づく状態遷移を実装する。

**外部パッケージ：**

`github.com/oyaguma3/go-eapaka` パッケージを活用し、EAP-AKA/AKA'プロトコル処理の大部分を委譲する。

**対象ファイル：**

| ファイル                             | 責務                                        |
| ------------------------------------ | ------------------------------------------- |
| `internal/eap/packet.go`             | EAPパケットパース・構築                     |
| `internal/eap/identity.go`           | Identity解析、EAP方式判定                   |
| `internal/eap/statemachine.go`       | ステートマシン制御                          |
| `internal/eap/constants.go`          | EAP関連定数                                 |
| `internal/eap/errors.go`             | EAPエラー定義                               |
| `internal/eap/types.go`              | EAP型定義                                   |
| `internal/eap/aka/challenge.go`      | EAP-AKA Challenge生成・検証                 |
| `internal/eap/aka/keys.go`           | EAP-AKA鍵導出・MAC計算（パッケージラッパー）|
| `internal/eap/akaprime/challenge.go` | EAP-AKA' Challenge生成・検証                |
| `internal/eap/akaprime/keys.go`      | EAP-AKA'鍵導出・MAC計算（パッケージラッパー）|
| `internal/engine/engine.go`          | 認証エンジン（EAP処理オーケストレーション） |

### 6.2 EAPパケット構造

**EAPヘッダ（4バイト）：**

| オフセット | 長さ | フィールド | 説明                                        |
| ---------- | ---- | ---------- | ------------------------------------------- |
| 0          | 1    | Code       | 1=Request, 2=Response, 3=Success, 4=Failure |
| 1          | 1    | Identifier | リクエスト/レスポンス対応付け               |
| 2          | 2    | Length     | パケット全体長（ヘッダ含む）                |
| 4          | 1    | Type       | 1=Identity, 23=AKA, 50=AKA'                 |
| 5          | 1    | Subtype    | AKA/AKA'サブタイプ                          |
| 6          | 2    | Reserved   | 予約（0x0000）                              |
| 8          | -    | Attributes | AT_xxx属性群                                |

**EAP-AKA/AKA' Subtype（パッケージ定数）：**

| 定数                            | 値   | 名称                        | 方向          |
| ------------------------------- | ---- | --------------------------- | ------------- |
| `SubtypeChallenge`              | 1    | AKA-Challenge               | Server→Client |
| `SubtypeAuthenticationReject`   | 2    | AKA-Authentication-Reject   | Client→Server |
| `SubtypeSynchronizationFailure` | 4    | AKA-Synchronization-Failure | Client→Server |
| `SubtypeIdentity`               | 5    | AKA-Identity                | 双方向        |
| `SubtypeClientError`            | 14   | AKA-Client-Error            | Client→Server |

### 6.3 EAPパケット処理

**ファイル:** `internal/eap/packet.go`

#### 6.3.1 パケットパース

`go-eapaka`パッケージの`Parse`関数を使用する。

```go
import (
    eapaka "github.com/oyaguma3/go-eapaka"
)

// ParseEAPPacket はEAP-Messageバイト列をパースする
func ParseEAPPacket(data []byte) (*eapaka.Packet, error) {
    return eapaka.Parse(data)
}
```

**パッケージによるバリデーション：**

- パケット長の整合性チェック
- EAP-AKA/AKA'ヘッダの検証
- 属性長のオーバーフローチェック
- EAP-AKA'でのAT_BIDDING使用禁止

#### 6.3.2 属性抽出ヘルパー

```go
// GetAttribute は指定型の属性を取得する
func GetAttribute[T eapaka.Attribute](pkt *eapaka.Packet) (T, bool) {
    var zero T
    for _, attr := range pkt.Attributes {
        if v, ok := attr.(T); ok {
            return v, true
        }
    }
    return zero, false
}
```

**主要属性型：**

| 属性                 | 型                          | 主要フィールド           |
| -------------------- | --------------------------- | ------------------------ |
| AT_RAND              | `*eapaka.AtRand`            | `Rand []byte` (16バイト) |
| AT_AUTN              | `*eapaka.AtAutn`            | `Autn []byte` (16バイト) |
| AT_RES               | `*eapaka.AtRes`             | `Res []byte` (可変長)    |
| AT_AUTS              | `*eapaka.AtAuts`            | `Auts []byte` (14バイト) |
| AT_MAC               | `*eapaka.AtMac`             | `MAC []byte` (16バイト)  |
| AT_IDENTITY          | `*eapaka.AtIdentity`        | `Identity string`        |
| AT_KDF               | `*eapaka.AtKdf`             | `KDF uint16`             |
| AT_KDF_INPUT         | `*eapaka.AtKdfInput`        | `NetworkName string`     |
| AT_CLIENT_ERROR_CODE | `*eapaka.AtClientErrorCode` | `Code uint16`            |

#### 6.3.3 パケット構築

`go-eapaka`パッケージの`Packet`構造体と`Marshal`メソッドを使用する。

```go
// EAP-Successパケット構築
func BuildEAPSuccess(identifier uint8) ([]byte, error) {
    pkt := &eapaka.Packet{
        Code:       eapaka.CodeSuccess,
        Identifier: identifier,
    }
    return pkt.Marshal()
}

// EAP-Failureパケット構築
func BuildEAPFailure(identifier uint8) ([]byte, error) {
    pkt := &eapaka.Packet{
        Code:       eapaka.CodeFailure,
        Identifier: identifier,
    }
    return pkt.Marshal()
}
```

### 6.4 Identity処理

**ファイル:** `internal/eap/identity.go`

**責務：** Identity文字列の解析、EAP方式判定、IMSI抽出

#### 6.4.1 Identity形式

```
<type_char><identifier>@<realm>
```

| 先頭文字 | 方式     | 識別子種別     | PoC対応          |
| -------- | -------- | -------------- | ---------------- |
| 0        | EAP-AKA  | 永続ID（IMSI） | ○                |
| 2        | EAP-AKA  | 仮名           | フル認証誘導     |
| 4        | EAP-AKA  | 高速再認証ID   | フル認証誘導     |
| 6        | EAP-AKA' | 永続ID（IMSI） | ○                |
| 7        | EAP-AKA' | 仮名           | フル認証誘導     |
| 8        | EAP-AKA' | 高速再認証ID   | フル認証誘導     |
| 1,3,5    | EAP-SIM  | -              | 非対応（Reject） |

#### 6.4.2 主要型・関数

```go
type IdentityType int

const (
    IdentityTypePermanentAKA      IdentityType = iota // 0: EAP-AKA永続ID
    IdentityTypePseudonymAKA                          // 2: EAP-AKA仮名
    IdentityTypeReauthAKA                             // 4: EAP-AKA再認証ID
    IdentityTypePermanentAKAPrime                     // 6: EAP-AKA'永続ID
    IdentityTypePseudonymAKAPrime                     // 7: EAP-AKA'仮名
    IdentityTypeReauthAKAPrime                        // 8: EAP-AKA'再認証ID
    IdentityTypeUnsupported                           // EAP-SIM等
    IdentityTypeInvalid                               // 不正形式
)

type ParsedIdentity struct {
    Type     IdentityType
    IMSI     string  // 永続IDの場合のみ有効
    Raw      string  // 元のIdentity文字列
    Realm    string  // @以降の部分
    EAPType  uint8   // eapaka.TypeAKA or eapaka.TypeAKAPrime
}

// ParseIdentity はIdentity文字列を解析する
func ParseIdentity(identity string) (*ParsedIdentity, error)

// RequiresFullAuth は仮名/再認証IDでフル認証誘導が必要か判定
func (p *ParsedIdentity) RequiresFullAuth() bool
```

#### 6.4.3 実装方針

- `@` でsplitしてrealm部分を分離
- 先頭1文字でIdentity種別を判定
- realm がない場合は `IdentityTypeInvalid`
- EAP-SIM系（1,3,5）は `IdentityTypeUnsupported`

### 6.5 ステートマシン

**ファイル:** `internal/eap/statemachine.go`

**責務：** D-03準拠のEAP認証状態遷移制御

#### 6.5.1 状態定義

D-03「EAP-AKA/AKA'ステートマシン設計書」準拠の状態定義。

```go
type EAPState string

const (
    StateNew              EAPState = "NEW"               // 初期状態
    StateWaitingIdentity  EAPState = "WAITING_IDENTITY"  // 仮名/再認証ID受信後、永続ID待ち
    StateIdentityReceived EAPState = "IDENTITY_RECEIVED" // 永続ID受領済み
    StateWaitingVector    EAPState = "WAITING_VECTOR"    // Vector Gateway応答待ち
    StateChallengeSent    EAPState = "CHALLENGE_SENT"    // Challenge送信済み
    StateResyncSent       EAPState = "RESYNC_SENT"       // 再同期処理中
    StateSuccess          EAPState = "SUCCESS"           // 認証成功
    StateFailure          EAPState = "FAILURE"           // 認証失敗
)
```

| 状態名 | 説明 | タイムアウト時 |
|-------|------|--------------|
| NEW | セッション開始直後。初期状態 | FAILURE |
| WAITING_IDENTITY | AT_PERMANENT_ID_REQ送信済み。永続ID応答待ち | FAILURE |
| IDENTITY_RECEIVED | 永続ID（IMSI）受領済み。Vector Gateway呼び出し前 | FAILURE |
| WAITING_VECTOR | Vector Gatewayへリクエスト中 | FAILURE |
| CHALLENGE_SENT | EAP-Request/AKA-Challenge送信済み | FAILURE |
| RESYNC_SENT | 再同期処理中（Vector Gatewayへ再同期リクエスト中） | FAILURE |
| SUCCESS | EAP-Success送信済み（終了状態） | - |
| FAILURE | EAP-Failure送信済み（終了状態） | - |

#### 6.5.2 状態遷移図

D-03 r3準拠の状態遷移図。

```
[NEW]
   │
   ├── EAP-Response/Identity受信
   │       │
   │       ├── [永続ID (0,6)] ────────────────────────────► [IDENTITY_RECEIVED]
   │       │                                                       │
   │       ├── [仮名/再認証ID (2,4,7,8)] ──► [WAITING_IDENTITY]    │
   │       │                                       │               │
   │       │                                       │ EAP-Response/ │
   │       │                                       │ AKA-Identity  │
   │       │                                       │       │       │
   │       │                                       │       ├── [永続ID] ─┘
   │       │                                       │       │
   │       │                                       │       └── [非対応/不正] ─► [FAILURE]
   │       │                                       │
   │       │                                       └── [Client-Error] ─► [FAILURE]
   │       │
   │       └── [非対応/不正 (1,3,5,realmなし)] ──► [FAILURE]

[IDENTITY_RECEIVED]
   │
   └── Vector Gateway呼び出し ──► [WAITING_VECTOR]
                                        │
                                        ├── [API成功] ─► Challenge送信 ─► [CHALLENGE_SENT]
                                        │
                                        └── [APIエラー] ─► [FAILURE]

[CHALLENGE_SENT]
   │
   ├── EAP-Response/AKA-Challenge受信
   │       │
   │       ├── [MAC/RES検証OK] ──► Post-Auth Policy評価
   │       │                              │
   │       │                              ├── [ルール一致] ─► [SUCCESS]
   │       │                              │
   │       │                              ├── [ルール不一致 + default=allow] ─► [SUCCESS]
   │       │                              │
   │       │                              ├── [ルール不一致 + default=deny] ─► [FAILURE]
   │       │                              │
   │       │                              └── [ポリシー未設定/不正] ─► [FAILURE]
   │       │
   │       └── [検証NG] ──► [FAILURE]
   │
   ├── EAP-Response/AKA-Synchronization-Failure受信
   │       │
   │       ├── [resync_count < 32] ──► [RESYNC_SENT]
   │       │
   │       └── [resync_count >= 32] ──► [FAILURE]
   │
   ├── EAP-Response/AKA-Authentication-Reject受信 ──► [FAILURE]
   │
   └── EAP-Response/AKA-Client-Error受信 ──► [FAILURE]

[RESYNC_SENT]
   │
   └── Vector Gateway再同期呼び出し
           │
           ├── [再同期成功] ──► 新Challenge送信 ──► [CHALLENGE_SENT]
           │
           └── [再同期失敗] ──► [FAILURE]

[SUCCESS] ──► (終了)

[FAILURE] ──► (終了)
```

#### 6.5.3 実装方針

- 状態遷移はEAPコンテキスト（Valkey）の`stage`フィールドで管理
- 各ハンドラは現在の状態を検証してから処理
- 不正な状態遷移は`EAP_INVALID_STATE`ログ出力後、Failure

### 6.6 EAPコンテキスト管理

**Valkeyキー:** `eap:{Trace ID}` **TTL:** 60秒（D-02準拠）

#### 6.6.1 EAPコンテキスト構造

```go
type EAPContext struct {
    IMSI                 string `redis:"imsi"`
    Stage                string `redis:"stage"`
    EAPType              uint8  `redis:"eap_type"`       // 23=AKA, 50=AKA'
    RAND                 string `redis:"rand"`           // Hex
    AUTN                 string `redis:"autn"`           // Hex（AKA'のCK'/IK'導出に必要）
    XRES                 string `redis:"xres"`           // Hex
    Kaut                 string `redis:"k_aut"`          // Hex
    MSK                  string `redis:"msk"`            // Hex
    ResyncCount          int    `redis:"resync_count"`
    PermanentIDRequested bool   `redis:"permanent_id_requested"`
}
```

**注記：** `autn`フィールドはEAP-AKA'のCK'/IK'導出時に必要なため保存する。

**セキュリティ方針（CK/IKの取り扱い）：**

- Vector Gatewayから受信したCK/IKは、鍵導出処理の一時変数としてのみ使用する
- 導出後の鍵（K_aut, MSK等）のみをEAPコンテキストに保存する
- **CK/IKはValkeyに永続化しない**（セキュリティ上の理由）

これにより、仮にValkeyのデータが漏洩した場合でも、生のCK/IKは含まれず、既に導出済みの鍵のみが露出するリスクに限定される。


#### 6.6.2 コンテキスト操作

| 操作 | タイミング             | 内容                         |
| ---- | ---------------------- | ---------------------------- |
| 作成 | Identity受信（永続ID） | IMSI, Stage, EAPType設定     |
| 更新 | Vector応答受信         | RAND, AUTN, XRES, 鍵情報追加 |
| 更新 | フル認証誘導時         | PermanentIDRequested=true    |
| 更新 | 再同期時               | ResyncCount++, 新Vector情報  |
| 削除 | 認証完了（成功/失敗）  | TTL任せでも可                |

### 6.7 EAP-AKA処理

**ファイル:** `internal/eap/aka/`

#### 6.7.1 鍵導出

**ファイル:** `internal/eap/aka/keys.go`

`go-eapaka`パッケージの`DeriveKeysAKA`関数を使用する。

```go
import (
    eapaka "github.com/oyaguma3/go-eapaka"
)

// DeriveKeysAKA はEAP-AKAの鍵階層を導出する
func DeriveKeysAKA(identity string, ck, ik []byte) eapaka.AkaKeys {
    return eapaka.DeriveKeysAKA(identity, ck, ik)
}
```

**パッケージ提供の`AkaKeys`構造体：**

| フィールド | サイズ   | 用途                                  |
| ---------- | -------- | ------------------------------------- |
| `K_encr`   | 16バイト | AT_ENCR_DATA暗号化（本PoCでは未使用） |
| `K_aut`    | 16バイト | AT_MAC計算・検証                      |
| `MSK`      | 64バイト | MS-MPPE-Key導出                       |
| `EMSK`     | 64バイト | 拡張MSK（本PoCでは未使用）            |

**導出処理（RFC 4187 Section 7）：**

1. MK = SHA-1(Identity || IK || CK)
2. PRF(MK, 0x00) で160バイト生成
3. K_encr(0-15), K_aut(16-31), MSK(32-95), EMSK(96-159)

#### 6.7.2 Challenge生成

**ファイル:** `internal/eap/aka/challenge.go`

```go
import (
    eapaka "github.com/oyaguma3/go-eapaka"
)

// BuildAKAChallenge はEAP-Request/AKA-Challengeパケットを構築する
func BuildAKAChallenge(identifier uint8, rand, autn, kAut []byte) ([]byte, error) {
    pkt := &eapaka.Packet{
        Code:       eapaka.CodeRequest,
        Identifier: identifier,
        Type:       eapaka.TypeAKA,
        Subtype:    eapaka.SubtypeChallenge,
        Attributes: []eapaka.Attribute{
            &eapaka.AtRand{Rand: rand},
            &eapaka.AtAutn{Autn: autn},
            &eapaka.AtMac{MAC: make([]byte, 16)}, // プレースホルダ
        },
    }

    // MAC計算・設定（HMAC-SHA-1-128）
    if err := pkt.CalculateAndSetMac(kAut); err != nil {
        return nil, fmt.Errorf("calculate MAC: %w", err)
    }

    return pkt.Marshal()
}
```

#### 6.7.3 Challenge応答検証

```go
// VerifyAKAChallengeResponse はEAP-Response/AKA-Challengeを検証する
func VerifyAKAChallengeResponse(pkt *eapaka.Packet, kAut, xres []byte) error {
    // 1. AT_MAC検証
    valid, err := pkt.VerifyMac(kAut)
    if err != nil {
        return fmt.Errorf("MAC verification error: %w", err)
    }
    if !valid {
        return ErrMACInvalid // AUTH_MAC_INVALID
    }

    // 2. AT_RES検証
    atRes, found := GetAttribute[*eapaka.AtRes](pkt)
    if !found {
        return ErrRESNotFound
    }

    if len(atRes.Res) != len(xres) {
        return ErrRESLengthMismatch // AUTH_RES_MISMATCH
    }

    if subtle.ConstantTimeCompare(atRes.Res, xres) != 1 {
        return ErrRESMismatch // AUTH_RES_MISMATCH
    }

    return nil
}
```

### 6.8 EAP-AKA'処理

**ファイル:** `internal/eap/akaprime/`

#### 6.8.1 EAP-AKA'とEAP-AKAの差異

| 項目       | EAP-AKA        | EAP-AKA'             |
| ---------- | -------------- | -------------------- |
| EAP Type   | 23 (`TypeAKA`) | 50 (`TypeAKAPrime`)  |
| 鍵導出     | CK, IK直接使用 | CK', IK'を導出       |
| PRF        | SHA-1ベース    | SHA-256ベース (PRF') |
| AT_MAC     | HMAC-SHA-1-128 | HMAC-SHA-256-128     |
| K_aut長    | 16バイト       | 32バイト             |
| 追加属性   | -              | AT_KDF, AT_KDF_INPUT |
| AT_BIDDING | 使用可         | 使用不可             |

> **注記（AT_BIDDING）：**
> - RFC 9048 Section 4により、AT_BIDDINGはEAP-AKAメッセージにのみ含まれ、EAP-AKA'メッセージには含まれない
> - AT_BIDDINGはBidding Down攻撃防止のための属性
> - **本PoC段階ではAT_BIDDING機能はサポートしない**（実機検証が困難なため）


#### 6.8.2 AT_KDF / AT_KDF_INPUT

**AT_KDF_INPUT:**

- Network Name（Access Network Identity）を格納
- 本PoCでは固定値 `WLAN`（環境変数 `EAP_AKA_PRIME_NETWORK_NAME` で設定可能）
- 空文字列は不可（パッケージ側でバリデーション）

**AT_KDF:**

- 鍵導出関数の識別子
- 本PoCでは `KDFAKAPrimeWithCKIK`（値=1）のみサポート

**パッケージ提供の定数：**

```go
const (
    KDFReserved         uint16 = 0  // 予約値、使用不可
    KDFAKAPrimeWithCKIK uint16 = 1  // 本PoCでサポート
)
```

**AT_KDFネゴシエーション：**

本PoCでは`KDFAKAPrimeWithCKIK`（値=1）のみサポートするため、クライアントが別の値を要求した場合はEAP-Failureを返す。

```go
// validateKdfInResponse はChallenge応答のAT_KDFを検証する
func validateKdfInResponse(pkt *eapaka.Packet) error {
    values := eapaka.KdfValuesFromAttributes(pkt.Attributes)
    
    // AT_KDFが含まれていない場合は正常（受け入れ）
    if len(values) == 0 {
        return nil
    }
    
    // KDF=1以外を要求された場合はエラー
    if len(values) != 1 || values[0] != eapaka.KDFAKAPrimeWithCKIK {
        return ErrKDFNotSupported // EAP_KDF_MISMATCH
    }
    
    return nil
}
```

#### 6.8.3 CK'/IK'導出

**ファイル:** `internal/eap/akaprime/keys.go`

`go-eapaka`パッケージの`DeriveCKPrimeIKPrime`関数を使用する。

```go
import (
    eapaka "github.com/oyaguma3/go-eapaka"
)

// DeriveCKPrimeIKPrime はCK'とIK'を導出する
func DeriveCKPrimeIKPrime(ck, ik []byte, networkName string, autn []byte) (ckPrime, ikPrime []byte, err error) {
    return eapaka.DeriveCKPrimeIKPrime(ck, ik, networkName, autn)
}
```

**RFC 9048 Section 3.3, TS 33.402 Annex A.2準拠：**

```
CK' || IK' = HMAC-SHA-256(CK||IK, S)

S = FC || P0 || L0 || P1 || L1

FC = 0x20
P0 = Access Network Identity（= Network Name）
L0 = length of P0（2バイトビッグエンディアン）
P1 = SQN ⊕ AK（AUTNの先頭6バイト）
L1 = 0x0006
```

**入力パラメータ：**

| パラメータ    | 取得元             | サイズ   | 備考                |
| ------------- | ------------------ | -------- | ------------------- |
| `ck`          | Vector Gateway応答 | 16バイト | 必須                |
| `ik`          | Vector Gateway応答 | 16バイト | 必須                |
| `networkName` | 環境変数           | 可変     | 空文字列不可        |
| `autn`        | Vector Gateway応答 | 16バイト | 先頭6バイトがSQN⊕AK |

**出力：**

- `ckPrime`: 16バイト（HMAC出力の先頭16バイト）
- `ikPrime`: 16バイト（HMAC出力の後半16バイト）

#### 6.8.4 セッション鍵導出

`go-eapaka`パッケージの`DeriveKeysAKAPrime`関数を使用する。

```go
// DeriveKeysAKAPrime はEAP-AKA'の鍵階層を導出する
func DeriveKeysAKAPrime(identity string, ckPrime, ikPrime []byte) eapaka.AkaPrimeKeys {
    return eapaka.DeriveKeysAKAPrime(identity, ckPrime, ikPrime)
}
```

**パッケージ提供の`AkaPrimeKeys`構造体：**

| フィールド | サイズ   | 用途                                  |
| ---------- | -------- | ------------------------------------- |
| `K_encr`   | 16バイト | AT_ENCR_DATA暗号化（本PoCでは未使用） |
| `K_aut`    | 32バイト | AT_MAC計算・検証                      |
| `K_re`     | 32バイト | 高速再認証（本PoCでは未使用）         |
| `MSK`      | 64バイト | MS-MPPE-Key導出                       |
| `EMSK`     | 64バイト | 拡張MSK（本PoCでは未使用）            |

**導出処理（RFC 9048 Section 3.3）：**

1. Key = IK' || CK'
2. Seed = "EAP-AKA'" || Identity
3. PRF'(Key, Seed) で208バイト生成
4. K_encr(0-15), K_aut(16-47), K_re(48-79), MSK(80-143), EMSK(144-207)

#### 6.8.5 Challenge生成

**ファイル:** `internal/eap/akaprime/challenge.go`

```go
import (
    eapaka "github.com/oyaguma3/go-eapaka"
)

// BuildAKAPrimeChallenge はEAP-Request/AKA'-Challengeパケットを構築する
func BuildAKAPrimeChallenge(
    identifier uint8,
    rand, autn []byte,
    networkName string,
    kAut []byte,
) ([]byte, error) {
    pkt := &eapaka.Packet{
        Code:       eapaka.CodeRequest,
        Identifier: identifier,
        Type:       eapaka.TypeAKAPrime,
        Subtype:    eapaka.SubtypeChallenge,
        Attributes: []eapaka.Attribute{
            &eapaka.AtRand{Rand: rand},
            &eapaka.AtAutn{Autn: autn},
            &eapaka.AtKdfInput{NetworkName: networkName},
            &eapaka.AtKdf{KDF: eapaka.KDFAKAPrimeWithCKIK},
            &eapaka.AtMac{MAC: make([]byte, 16)}, // プレースホルダ
        },
    }

    // MAC計算・設定（HMAC-SHA-256-128）
    if err := pkt.CalculateAndSetMac(kAut); err != nil {
        return nil, fmt.Errorf("calculate MAC: %w", err)
    }

    return pkt.Marshal()
}
```

#### 6.8.6 Challenge応答検証

```go
// VerifyAKAPrimeChallengeResponse はEAP-Response/AKA'-Challengeを検証する
func VerifyAKAPrimeChallengeResponse(pkt *eapaka.Packet, kAut, xres []byte) error {
    // 1. AT_KDF検証（ネゴシエーション要求の有無）
    if err := validateKdfInResponse(pkt); err != nil {
        return err
    }

    // 2. AT_MAC検証（パッケージがHMAC-SHA-256-128を自動使用）
    valid, err := pkt.VerifyMac(kAut)
    if err != nil {
        return fmt.Errorf("MAC verification error: %w", err)
    }
    if !valid {
        return ErrMACInvalid // AUTH_MAC_INVALID
    }

    // 3. AT_RES検証
    atRes, found := GetAttribute[*eapaka.AtRes](pkt)
    if !found {
        return ErrRESNotFound
    }

    if len(atRes.Res) != len(xres) {
        return ErrRESLengthMismatch // AUTH_RES_MISMATCH
    }

    if subtle.ConstantTimeCompare(atRes.Res, xres) != 1 {
        return ErrRESMismatch // AUTH_RES_MISMATCH
    }

    return nil
}
```

#### 6.8.7 鍵導出フロー全体

```go
// deriveAKAPrimeKeys はEAP-AKA'の全鍵導出を行う
func deriveAKAPrimeKeys(
    identity string,
    ck, ik, autn []byte,
    networkName string,
) (*eapaka.AkaPrimeKeys, error) {
    // 1. CK'/IK'導出
    ckPrime, ikPrime, err := eapaka.DeriveCKPrimeIKPrime(ck, ik, networkName, autn)
    if err != nil {
        return nil, fmt.Errorf("derive CK'/IK': %w", err)
    }

    // 2. セッション鍵導出
    keys := eapaka.DeriveKeysAKAPrime(identity, ckPrime, ikPrime)

    return &keys, nil
}
```

### 6.9 再同期処理

**トリガー:** EAP-Response/AKA-Synchronization-Failure受信

**含まれる属性：**

- AT_AUTS: 14バイト（AUTS値）

**処理フロー：**

1. EAPコンテキストから `resync_count` 取得
2. 上限チェック（32回）
    - 超過 → `AUTH_RESYNC_LIMIT` ログ、Failure
3. Vector Gateway呼び出し（resync_infoを含む）
4. 新しいVector情報でChallenge再生成
5. `resync_count` インクリメント、コンテキスト更新
6. 新Challenge送信

**AUTS抽出：**

```go
func extractAUTS(pkt *eapaka.Packet) ([]byte, error) {
    atAuts, found := GetAttribute[*eapaka.AtAuts](pkt)
    if !found {
        return nil, ErrAUTSNotFound
    }
    return atAuts.Auts, nil
}
```

**Vector Gateway呼び出し：**

```json
{
    "imsi": "440101234567890",
    "resync_info": {
        "rand": "<元のRAND(Hex)>",
        "auts": "<受信したAUTS(Hex)>"
    }
}
```

**注意点：**

- 元のRANDはEAPコンテキストから取得
- resync_countはセッション単位でカウント
- 再同期成功後も同一Trace IDを継続使用
- EAP-AKA/AKA'両方で同一処理

### 6.10 フル認証誘導

**トリガー:** 仮名ID(2,7)または高速再認証ID(4,8)受信

**処理フロー：**

1. Identity種別判定で仮名/再認証IDと判定
2. `EAP_PSEUDONYM_FALLBACK` ログ出力
3. EAPコンテキスト作成（`permanent_id_requested=true`）
4. EAP-Request/AKA-Identity送信（AT_PERMANENT_ID_REQ含む）
5. 永続ID応答を待機

**AKA-Identity構築：**

```go
// BuildAKAIdentityRequest はEAP-Request/AKA-Identityパケットを構築する
func BuildAKAIdentityRequest(identifier uint8, eapType uint8) ([]byte, error) {
    pkt := &eapaka.Packet{
        Code:       eapaka.CodeRequest,
        Identifier: identifier,
        Type:       eapType, // TypeAKA or TypeAKAPrime
        Subtype:    eapaka.SubtypeIdentity,
        Attributes: []eapaka.Attribute{
            &eapaka.AtPermanentIdReq{},
        },
    }
    return pkt.Marshal()
}
```

**注意点：**

- `permanent_id_requested=true` の状態で再度仮名/再認証IDを受信した場合はFailure
- EAP TypeはIdentityの先頭文字から判定した方式を継続

### 6.11 実装時の注意点まとめ

#### 6.11.1 go-eapakaパッケージ活用一覧

| 処理               | 自前実装 | パッケージ関数/メソッド            |
| ------------------ | -------- | ---------------------------------- |
| パケットパース     | 不要     | `eapaka.Parse()`                   |
| パケット構築       | 不要     | `Packet.Marshal()`                 |
| MAC計算・設定      | 不要     | `Packet.CalculateAndSetMac()`      |
| MAC検証            | 不要     | `Packet.VerifyMac()`               |
| EAP-AKA鍵導出      | 不要     | `eapaka.DeriveKeysAKA()`           |
| CK'/IK'導出        | 不要     | `eapaka.DeriveCKPrimeIKPrime()`    |
| EAP-AKA'鍵導出     | 不要     | `eapaka.DeriveKeysAKAPrime()`      |
| MS-MPPE-Key暗号化  | 不要     | `eapaka.EncryptMPPEKey()`          |
| AT_KDF値抽出       | 不要     | `eapaka.KdfValuesFromAttributes()` |
| AT_KDFオファー構築 | 不要     | `eapaka.BuildKdfOfferAttributes()` |

#### 6.11.2 削減されたファイル

以下のファイルは`go-eapaka`パッケージの機能により不要となり、`keys.go`に統合された：

- `internal/eap/aka/mac.go` → `keys.go`に統合済み
- `internal/eap/akaprime/mac.go` → `keys.go`に統合済み
- `internal/eap/akaprime/prf.go` → `keys.go`に統合済み

#### 6.11.3 パッケージによるバリデーション

| 条件                               | エラー内容                   |
| ---------------------------------- | ---------------------------- |
| AT_KDF_INPUTのNetworkNameが空      | Marshal時エラー              |
| EAP-AKA'でAT_BIDDINGを含む         | Marshal/Parse時エラー        |
| Request/ResponseでType != AKA/AKA' | Marshal時エラー              |
| CK/IKが16バイト以外                | DeriveCKPrimeIKPrime時エラー |
| AUTNが6バイト未満                  | DeriveCKPrimeIKPrime時エラー |

#### 6.11.4 EAPコンテキスト保存項目

| フィールド               | EAP-AKA | EAP-AKA' | 用途                   |
| ------------------------ | ------- | -------- | ---------------------- |
| `imsi`                   | ○       | ○        | 加入者識別             |
| `stage`                  | ○       | ○        | 状態管理               |
| `eap_type`               | 23      | 50       | MAC計算方式判定        |
| `rand`                   | ○       | ○        | 再同期時に必要         |
| `autn`                   | -       | ○        | CK'/IK'再導出時に必要  |
| `xres`                   | ○       | ○        | RES検証用              |
| `k_aut`                  | ○       | ○        | MAC検証用              |
| `msk`                    | ○       | ○        | MS-MPPE-Key導出用      |
| `resync_count`           | ○       | ○        | 再同期上限管理         |
| `permanent_id_requested` | ○       | ○        | フル認証誘導済みフラグ |

#### 6.11.5 RFC参照

| 項目                | 参照RFC                                   |
| ------------------- | ----------------------------------------- |
| EAP-AKA             | RFC 4187                                  |
| EAP-AKA'            | RFC 9048（RFC 5448を廃止）                |
| CK'/IK'導出         | RFC 9048 Section 3.3, TS 33.402 Annex A.2 |
| AT_KDF/AT_KDF_INPUT | RFC 9048 Section 3.1, 3.2                 |
| AT_BIDDING          | RFC 9048 Section 4                        |
| MS-MPPE-Key暗号化   | RFC 2548 Section 2.4.2, 2.4.3             |

---

## ■セクション7: Vector Gateway連携

### 7.1 概要

本セクションでは、Auth ServerからVector Gatewayへの認証ベクター取得処理を定義する。D-12「Vector Gateway実装レベル検討書」およびD-06「エラーハンドリング詳細設計書」に基づく。

**対象ファイル：**

- `internal/vector/client.go`
- `internal/vector/constants.go`
- `internal/vector/errors.go`
- `internal/vector/interfaces.go`
- `internal/vector/types.go`

> **注記（r9変更）：** 旧設計の`breaker.go`（Circuit Breaker設定・管理）は削除された。Circuit Breaker機能は現在のPoC実装では使用しない。

### 7.2 通信仕様

**エンドポイント:** `POST /api/v1/vector`

**プロトコル:** HTTP/1.1

**タイムアウト設定（D-06準拠）：**

| 項目                   | 値   |
| ---------------------- | ---- |
| 接続タイムアウト       | 2秒  |
| リクエストタイムアウト | 5秒  |

### 7.3 リクエスト仕様

#### 7.3.1 通常認証リクエスト

```json
{
    "imsi": "440101234567890"
}
```

#### 7.3.2 再同期リクエスト

```json
{
    "imsi": "440101234567890",
    "resync_info": {
        "rand": "0123456789ABCDEF0123456789ABCDEF",
        "auts": "0123456789ABCDEF01234567890A"
    }
}
```

#### 7.3.3 リクエストヘッダ

| ヘッダ         | 値                 | 必須 |
| -------------- | ------------------ | ---- |
| `Content-Type` | `application/json` | Yes  |
| `X-Trace-ID`   | Trace ID (UUID)    | Yes  |

### 7.4 レスポンス仕様

#### 7.4.1 成功レスポンス (200 OK)

```json
{
    "rand": "F4B38A...",
    "autn": "2B9E10...",
    "xres": "D8A1...",
    "ck": "91E3...",
    "ik": "C42F..."
}
```

**フィールド説明：**

| フィールド | 長さ                     | 説明               |
| ---------- | ------------------------ | ------------------ |
| `rand`     | 32文字 (16バイトHex)     | ランダムチャレンジ |
| `autn`     | 32文字 (16バイトHex)     | 認証トークン       |
| `xres`     | 8-32文字 (4-16バイトHex) | 期待レスポンス     |
| `ck`       | 32文字 (16バイトHex)     | 暗号鍵             |
| `ik`       | 32文字 (16バイトHex)     | 整合性鍵           |

#### 7.4.2 エラーレスポンス

**形式:** RFC 7807 Problem Details

```json
{
    "type": "about:blank",
    "title": "User Not Found",
    "detail": "IMSI 440101234567890 does not exist in subscriber DB.",
    "status": 404
}
```

**HTTPステータス別の意味：**

| ステータス | 意味                       | Auth Serverの対処         |
| ---------- | -------------------------- | ------------------------- |
| 400        | リクエスト不正、再同期失敗 | Failure応答               |
| 404        | IMSI未登録                 | Failure応答               |
| 501        | 未実装バックエンド         | Failure応答（CB対象外）   |
| 502        | バックエンド通信エラー     | Failure応答（CBカウント） |
| 500        | 内部エラー                 | Failure応答（CBカウント） |

### 7.5 クライアント実装

**ファイル:** `internal/vector/client.go`

#### 7.5.1 構造体設計

**主要型：**

| 型               | 責務                                  |
| ---------------- | ------------------------------------- |
| `Client`         | HTTPクライアント、Circuit Breaker統合 |
| `VectorRequest`  | リクエストボディ                      |
| `ResyncInfo`     | 再同期情報                            |
| `VectorResponse` | レスポンスボディ                      |
| `ProblemDetails` | エラーレスポンス                      |

#### 7.5.2 実装方針

**初期化：**

- `resty.Client` をラップ
- タイムアウト設定を適用
- Circuit Breakerを初期化・保持

**GetVector メソッド：**

- Circuit Breaker経由でHTTP呼び出し
- Trace IDをヘッダに付与
- レスポンスのパース・エラー変換

**リトライ：**

- Auth Server → Vector Gateway間ではリトライしない（D-06準拠）
- Circuit Breakerに委ねる

#### 7.5.3 注意点

- Hex文字列のバリデーション（レスポンス受信時）
- resync_infoはnilの場合JSONに含めない（`omitempty`）
- contextによるキャンセル伝搬
- HTTPクライアントのコネクションプール設定

### 7.6 Circuit Breaker

> **注記（r9変更）：** 本セクションの内容は参考情報として残すが、現在のPoC実装ではCircuit Breaker機能（`breaker.go`）は削除されている。`gobreaker`パッケージの依存も除去済み。将来的に必要になった場合は、以下の設計に基づいて再導入する。

**ファイル:** ~~`internal/vector/breaker.go`~~ （r9で削除済み）

#### 7.6.1 設定値（D-06準拠）

| 項目             | 値                 | 説明                 |
| ---------------- | ------------------ | -------------------- |
| Name             | `"vector-gateway"` | ログ識別用           |
| MaxRequests      | 3                  | Half-Open時の許可数  |
| Interval         | 10秒               | 計測ウィンドウ       |
| Timeout          | 30秒               | Open維持時間         |
| FailureThreshold | 5                  | Open遷移の連続失敗数 |

#### 7.6.2 状態遷移

```
        連続失敗5回
    ┌─────────────────┐
    │                 ▼
┌───────┐        ┌────────┐
│Closed │        │  Open  │
└───┬───┘        └────┬───┘
    ▲                 │
    │ 成功2回         │ 30秒経過
    │                 ▼
    │           ┌──────────┐
    └───────────┤Half-Open │
                └──────┬───┘
                       │ 失敗
                       ▼
                   [Openへ]
```

#### 7.6.3 失敗判定条件

**失敗としてカウント：**

- 接続エラー（TCP接続失敗）
- タイムアウト
- HTTPステータス 5xx
- HTTPステータス 502（バックエンド通信エラー）

**失敗としてカウントしない：**

- HTTPステータス 400（リクエスト不正）
- HTTPステータス 404（IMSI未登録）
- HTTPステータス 501（未実装バックエンド）

#### 7.6.4 実装方針

**gobreaker.Settings：**

- `ReadyToTrip`: 連続失敗数で判定
- `IsSuccessful`: HTTPステータスコードで判定（カスタム関数）
- `OnStateChange`: 状態遷移時にログ出力

**IsSuccessful関数：**

```go
func isSuccessful(err error) bool {
    if err == nil {
        return true
    }
    var apiErr *APIError
    if errors.As(err, &apiErr) {
        // 4xx系は成功扱い（CB発動させない）
        return apiErr.StatusCode >= 400 && apiErr.StatusCode < 500
    }
    return false
}
```

#### 7.6.5 注意点

- Circuit Breaker Open時は即座にエラー返却（API呼び出ししない）
- Open遷移時のログは `CB_OPEN` イベント
- 状態遷移ログには `cb_name` を含める

### 7.7 Trace ID伝搬

#### 7.7.1 伝搬フロー

```
Auth Server                    Vector Gateway                 Vector API
    │                               │                              │
    ├── X-Trace-ID: {uuid} ────────►│                              │
    │                               ├── X-Trace-ID: {uuid} ───────►│
    │                               │                              │
    │◄────────── Response ──────────┤◄────────── Response ─────────┤
    │                               │                              │
```

#### 7.7.2 実装方針

- contextからTrace IDを取得
- リクエストヘッダ `X-Trace-ID` に設定
- ログ出力時は同一Trace IDを使用

**注意点：**

- Trace IDが空の場合はエラー（呼び出し元の実装ミス）
- レスポンスのTrace IDは検証しない（Gateway/APIが同じIDを使う保証はない）

### 7.8 エラーハンドリング

#### 7.8.1 エラー種別

| エラー種別   | 検出条件                 | CB対象 | 対処                |
| ------------ | ------------------------ | ------ | ------------------- |
| 接続エラー   | TCP接続失敗              | Yes    | CBカウント、Failure |
| タイムアウト | 応答なし（5秒超過）      | Yes    | CBカウント、Failure |
| CB Open      | Circuit Breaker Open状態 | -      | 即座にFailure       |
| HTTP 400     | Bad Request              | No     | Failure             |
| HTTP 404     | Not Found                | No     | Failure             |
| HTTP 501     | Not Implemented          | No     | Failure             |
| HTTP 502     | Bad Gateway              | Yes    | CBカウント、Failure |
| HTTP 5xx     | Server Error             | Yes    | CBカウント、Failure |

#### 7.8.2 エラー型定義

**実装方針：**

- カスタムエラー型でHTTPステータスを保持
- `errors.Is` / `errors.As` で判定可能に
- 元エラーをラップして詳細保持

**エラー型：**

| 型                | 用途                                   |
| ----------------- | -------------------------------------- |
| `ErrCircuitOpen`  | CB Open状態                            |
| `APIError`        | HTTP応答エラー（ステータスコード保持） |
| `ConnectionError` | 接続・タイムアウトエラー               |

#### 7.8.3 呼び出し元での処理

**EAP処理層での判定：**

| エラー種別             | event_id                      | RADIUS応答    |
| ---------------------- | ----------------------------- | ------------- |
| CB Open                | （CB_OPENは遷移時に出力済み） | Access-Reject |
| 接続エラー             | `VECTOR_API_ERR`              | Access-Reject |
| HTTP 404               | `AUTH_IMSI_NOT_FOUND`         | Access-Reject |
| HTTP 400（再同期失敗） | `SQN_RESYNC_MAC_ERR`等        | Access-Reject |
| その他                 | `VECTOR_API_ERR`              | Access-Reject |

### 7.9 処理フロー

```
EAP処理層
    │
    ├── VectorClient.GetVector(ctx, req)
    │       │
    │       ├── Circuit Breaker チェック
    │       │       │
    │       │       ├── [Open] → ErrCircuitOpen返却
    │       │       │
    │       │       └── [Closed/Half-Open] → HTTP呼び出しへ
    │       │
    │       ├── HTTP POST /api/v1/vector
    │       │       │
    │       │       ├── Header: X-Trace-ID
    │       │       ├── Header: Content-Type
    │       │       └── Body: VectorRequest (JSON)
    │       │
    │       ├── レスポンス受信
    │       │       │
    │       │       ├── [200] → VectorResponse返却
    │       │       │
    │       │       ├── [4xx] → APIError返却（CB対象外）
    │       │       │
    │       │       └── [5xx/502] → APIError返却（CBカウント）
    │       │
    │       └── Circuit Breaker 結果記録
    │
    └── エラー判定・ログ出力
```

### 7.10 ログ出力仕様

| 処理             | event_id         | レベル | 追加フィールド                       |
| ---------------- | ---------------- | ------ | ------------------------------------ |
| API呼び出し成功  | -                | DEBUG  | `latency_ms`                         |
| API呼び出し失敗  | `VECTOR_API_ERR` | ERROR  | `error`, `http_status`, `latency_ms` |
| CB Open遷移      | `CB_OPEN`        | WARN   | `cb_name`, `failure_count`           |
| CB Half-Open遷移 | `CB_HALF_OPEN`   | INFO   | `cb_name`                            |
| CB Close遷移     | `CB_CLOSE`       | INFO   | `cb_name`, `recovery_time_ms`        |

### 7.11 実装時の注意点まとめ

**HTTPクライアント：**

- `resty` のリトライ機能は無効化（CBに委ねる）
- コネクションプールのサイズ設定を適切に
- Keep-Alive有効（デフォルト）

**Circuit Breaker：**

- `gobreaker` パッケージを使用
- `IsSuccessful` 関数で4xx系を成功扱いにする
- 状態遷移ログは `OnStateChange` コールバックで出力
- Open時間計測用に遷移時刻を記録

**Trace ID：**

- contextから取得、ヘッダに設定
- 空の場合はパニックではなくエラー返却

**エラーハンドリング：**

- HTTPステータスコードを保持するエラー型を使用
- 呼び出し元でステータスに応じた分岐が可能に
- 元エラーのラップを忘れずに

**レスポンスパース：**

- Hexフィールドの長さ検証
- 不正な形式は即座にエラー（Failureへ）

**再同期：**

- `resync_info` が nil の場合はJSONに含めない
- RANDとAUTSはHex文字列（大文字に正規化推奨）

------

## ■セクション8: 認可処理

### 8.1 概要

本セクションでは、EAP認証成功後の認可（Authorization）処理を定義する。D-02「Valkeyデータ設計仕様書」のポリシー構造に基づき、接続可否の判定およびRADIUS AVP生成を行う。

**対象ファイル：**

- `internal/policy/evaluator.go`
- `internal/policy/avp.go`
- `internal/policy/errors.go`
- `internal/policy/interfaces.go`
- `internal/policy/types.go`
- `internal/store/policy.go`

### 8.2 認可処理の位置づけ

**処理タイミング：**

- EAP-AKA/AKA' Challenge応答の検証成功後
- Access-Accept/Reject送信前

**処理フロー：**

```
Challenge応答検証成功
    │
    ├── ポリシー取得 (policy:{IMSI})
    │       │
    │       ├── [不在] → Access-Reject
    │       │
    │       └── [存在] → ルール評価
    │               │
    │               ├── [一致ルールあり] → AVP生成 → Access-Accept
    │               │
    │               ├── [一致ルールなし + default=allow] → Access-Accept
    │               │
    │               └── [一致ルールなし + default=deny] → Access-Reject
    │
    └── RADIUS応答送信
```

### 8.3 ポリシーデータ構造

**Valkeyキー:** `policy:{IMSI}` **Type:** Hash

#### 8.3.1 フィールド定義（D-02準拠）

| フィールド | 型            | 必須 | 説明                                |
| ---------- | ------------- | ---- | ----------------------------------- |
| `rules`    | String (JSON) | Yes  | 認可ルール配列                      |
| `default`  | String        | Yes  | デフォルト動作（`allow` or `deny`） |

#### 8.3.2 rulesフィールド構造

```json
[
    {
        "ssid": "Staff",
        "action": "allow",
        "time_min": "08:00",
        "time_max": "22:00"
    },
    {
        "ssid": "Guest",
        "action": "allow"
    },
    {
        "ssid": "*",
        "action": "deny"
    }
]
```

#### 8.3.3 ルールフィールド定義

| フィールド   | 型     | 必須 | 説明                                          |
| ------------ | ------ | ---- | --------------------------------------------- |
| `ssid`       | String | Yes  | 対象SSID（完全一致、`"*"` でワイルドカード）  |
| `action`     | String | Yes  | 動作（`"allow"` or `"deny"`）                 |
| `time_min`   | String | No   | 許可開始時刻（`"HH:MM"` 形式、省略時は制限なし） |
| `time_max`   | String | No   | 許可終了時刻（`"HH:MM"` 形式、省略時は制限なし） |

> **注記（r9変更）：** 旧仕様のPolicyRule（`nas_id`, `allowed_ssids`, `vlan_id`, `session_timeout`）は廃止された。新仕様ではSSIDマッチングとAction（allow/deny）、時間帯条件（time_min/time_max）による判定に変更された。VLAN IDやSession Timeoutの設定はポリシールール外で管理される。

### 8.4 ポリシー取得

**ファイル:** `internal/store/policy.go`

#### 8.4.1 実装方針

- `HGETALL policy:{IMSI}` でポリシー全体を取得
- キー不在の場合は `ErrPolicyNotFound` を返却
- `rules` フィールドをJSONパース
- パースエラーは `ErrPolicyInvalid` として処理

#### 8.4.2 主要型

```go
type Policy struct {
    Rules   []PolicyRule
    Default string  // "allow" or "deny"
}

type PolicyRule struct {
    SSID    string `json:"ssid"`
    Action  string `json:"action"`           // "allow" or "deny"
    TimeMin string `json:"time_min,omitempty"` // "HH:MM" 形式
    TimeMax string `json:"time_max,omitempty"` // "HH:MM" 形式
}
```

#### 8.4.3 注意点

- `default` フィールドの値は小文字で統一（`allow`/`deny`）
- 不正な `default` 値は `deny` として扱う
- `rules` が空配列の場合は `default` に従う

### 8.5 ルール評価

**ファイル:** `internal/policy/evaluator.go`

#### 8.5.1 評価入力

| パラメータ        | 取得元               | 説明                   |
| ----------------- | -------------------- | ---------------------- |
| Called-Station-Id | RADIUS AVP (Type 30) | BSSID:SSID形式が一般的 |
| 現在時刻          | システム時刻         | 時間帯条件の判定に使用 |

#### 8.5.2 SSID抽出

**Called-Station-Id形式：**

- 一般的な形式: `AA-BB-CC-DD-EE-FF:SSID_NAME`
- コロン(`:`)で分割し、後半部分をSSIDとして使用
- コロンがない場合は全体をSSIDとして扱う

#### 8.5.3 評価ロジック

```
評価開始
    │
    ├── SSID抽出（Called-Station-Id）
    │       └── [取得失敗] → default判定へ
    │
    ├── ルール配列を順次評価
    │       │
    │       └── 各ルール:
    │               │
    │               ├── ssid一致チェック
    │               │       ├── "*" → 一致（ワイルドカード）
    │               │       ├── SSIDが一致 → 一致
    │               │       └── [不一致] → 次のルールへ
    │               │
    │               ├── 時間帯条件チェック（time_min/time_max指定時）
    │               │       ├── 現在時刻が範囲内 → 条件成立
    │               │       └── 範囲外 → 次のルールへ
    │               │
    │               └── action判定
    │                       ├── action="allow" → 結果: Allow
    │                       └── action="deny"  → 結果: Deny
    │
    └── [一致ルールなし]
            │
            ├── default="allow" → 結果: Allow（AVP設定なし）
            │
            └── default="deny" → 結果: Deny
```

#### 8.5.4 評価結果型

```go
type EvaluationResult struct {
    Allowed        bool
    MatchedRule    *PolicyRule  // nilの場合はdefault適用
    DenyReason     string       // Deny時の理由
}
```

#### 8.5.5 注意点

- ルールは配列順に評価、最初にSSIDと時間帯条件が一致したルールを適用
- SSIDの比較は大文字小文字を区別しない（一般的なWi-Fi実装に合わせる）
- `ssid: "*"` はワイルドカード（全SSID一致）
- `time_min`/`time_max` が省略された場合は時間帯制限なし（常時適用）
- `time_min`/`time_max` は `"HH:MM"` 形式（24時間表記）

### 8.6 AVP生成

**ファイル:** `internal/policy/avp.go`

#### 8.6.1 生成対象AVP

| 条件                  | AVP                     | Type | 値             |
| --------------------- | ----------------------- | ---- | -------------- |
| 常時                  | Class                   | 25   | セッションUUID |
| 常時                  | EAP-Message             | 79   | EAP-Success    |
| 常時                  | MS-MPPE-Recv-Key        | VSA  | MSKから導出    |
| 常時                  | MS-MPPE-Send-Key        | VSA  | MSKから導出    |
| vlan_id設定時         | Tunnel-Type             | 64   | 13 (VLAN)      |
| vlan_id設定時         | Tunnel-Medium-Type      | 65   | 6 (IEEE-802)   |
| vlan_id設定時         | Tunnel-Private-Group-Id | 81   | VLAN ID文字列  |
| session_timeout設定時 | Session-Timeout         | 27   | タイムアウト秒 |

> **注記（r9変更）：** VLAN IDやSession Timeoutの設定はポリシールール（PolicyRule）から分離された。これらの値はポリシールール外の設定（加入者情報等）から取得される。

#### 8.6.2 VLAN AVP生成

**Tunnel系AVP（RFC 2868）：**

- Tag付き属性として送信可能だが、Tag=0（タグなし）で送信するのが一般的
- 3属性をセットで追加

**実装方針：**

- `vlan_id` が空文字列または未設定の場合は生成しない
- VLAN IDは文字列としてそのまま設定

#### 8.6.3 MS-MPPE-Key生成

**RFC 2548準拠：**

- MS-MPPE-Recv-Key: MSKの先頭32バイト
- MS-MPPE-Send-Key: MSKの後半32バイト

**実装方針：**

`oyaguma3/go-eapaka`パッケージの`EncryptMPPEKey`関数を利用する。

```go
import (
    eapaka "github.com/oyaguma3/go-eapaka"
)

// generateMPPEKeys はMSKからMS-MPPE-Send/Recv-Key AVP値を生成する
func generateMPPEKeys(
    msk []byte,
    secret []byte,
    reqAuth []byte,
) (recvKeyEncrypted, sendKeyEncrypted []byte, err error) {
    if len(msk) < 64 {
        return nil, nil, errors.New("MSK must be at least 64 bytes")
    }
    
    // MSKを分割（RFC 3748準拠）
    recvKeyPlain := msk[:32]   // MS-MPPE-Recv-Key用
    sendKeyPlain := msk[32:64] // MS-MPPE-Send-Key用
    
    // go-eapakaパッケージのMPPE暗号化機能を利用
    recvKeyEncrypted, err = eapaka.EncryptMPPEKey(recvKeyPlain, secret, reqAuth)
    if err != nil {
        return nil, nil, fmt.Errorf("encrypt recv key: %w", err)
    }
    
    sendKeyEncrypted, err = eapaka.EncryptMPPEKey(sendKeyPlain, secret, reqAuth)
    if err != nil {
        return nil, nil, fmt.Errorf("encrypt send key: %w", err)
    }
    
    return recvKeyEncrypted, sendKeyEncrypted, nil
}
```

**注意点：**

- `EncryptMPPEKey`がRFC 2548 Section 2.4.2/2.4.3の暗号化処理を実装済み
- Request Authenticator（16バイト）はAccess-Requestパケットから取得
- Shared SecretはRADIUSクライアント設定から取得（SecretSource経由）
- 戻り値は `Salt(2バイト) || 暗号化データ` 形式

#### 8.6.4 Class属性

**用途：**

- Accountingパケットとの紐付け
- セッションUUID（Trace IDとは別）を格納

**形式：**

- RFC 4122準拠UUID（ハイフン含む36文字）
- 例: `550e8400-e29b-41d4-a716-446655440000`
- `github.com/google/uuid` パッケージの `uuid.NewString()` で生成

**Acct Serverとの連携：**

- Acct ServerはClass属性を `uuid.Parse()` でRFC 4122準拠を検証
- 検証成功時のみ `sess:{UUID}` キーでセッション検索

### 8.7 ポリシー未設定時の動作

**方針（D-02/D-03準拠）：**

- `policy:{IMSI}` が存在しない場合は**認証拒否**
- 加入者登録とポリシー登録はセットで行う運用

**処理：**

1. ポリシー取得で `ErrPolicyNotFound`
2. `AUTH_POLICY_NOT_FOUND` ログ出力
3. Access-Reject（EAP-Failure含む）送信

### 8.8 エラーハンドリング

| エラー種別          | 検出条件                        | 対処        | ログ                          |
| ------------------- | ------------------------------- | ----------- | ----------------------------- |
| ポリシー未設定      | `policy:{IMSI}` 不在            | Reject      | INFO: `AUTH_POLICY_NOT_FOUND` |
| ポリシー不正        | JSONパース失敗                  | Reject      | WARN: `POLICY_PARSE_ERR`      |
| ルール不一致        | 全ルール評価後マッチなし + deny | Reject      | INFO: `AUTH_POLICY_DENIED`    |
| NAS-ID/SSID取得失敗 | AVP不在                         | default判定 | DEBUG                         |

### 8.9 処理フロー図

```
Challenge応答検証成功
    │
    ├── PolicyStore.GetPolicy(imsi)
    │       │
    │       ├── [ErrPolicyNotFound]
    │       │       │
    │       │       ├── ログ: AUTH_POLICY_NOT_FOUND
    │       │       └── → Access-Reject
    │       │
    │       ├── [ErrPolicyInvalid]
    │       │       │
    │       │       ├── ログ: POLICY_PARSE_ERR
    │       │       └── → Access-Reject
    │       │
    │       └── [成功] → Policy取得
    │
    ├── NAS-Identifier/SSID抽出
    │       │
    │       └── RADIUSパケットから取得
    │
    ├── Evaluator.Evaluate(policy, nasID, ssid)
    │       │
    │       ├── [Allowed + MatchedRule]
    │       │       │
    │       │       ├── セッション作成
    │       │       ├── AVP生成（VLAN, Timeout等）
    │       │       └── → Access-Accept
    │       │
    │       ├── [Allowed + default]
    │       │       │
    │       │       ├── セッション作成
    │       │       └── → Access-Accept（AVP最小限）
    │       │
    │       └── [Denied]
    │               │
    │               ├── ログ: AUTH_POLICY_DENIED
    │               └── → Access-Reject
    │
    └── RADIUS応答送信
```

### 8.10 ログ出力仕様

| 処理                  | event_id                | レベル | 追加フィールド                      |
| --------------------- | ----------------------- | ------ | ----------------------------------- |
| ポリシー未設定        | `AUTH_POLICY_NOT_FOUND` | INFO   | `imsi`                              |
| ポリシーパースエラー  | `POLICY_PARSE_ERR`      | WARN   | `imsi`, `error`                     |
| ルール不一致でDeny    | `AUTH_POLICY_DENIED`    | INFO   | `imsi`, `nas_id`, `ssid`            |
| ルール一致でAccept    | -                       | DEBUG  | `imsi`, `nas_id`, `ssid`, `vlan_id` |
| default=allowでAccept | -                       | DEBUG  | `imsi`, `nas_id`, `ssid`            |

### 8.11 実装時の注意点まとめ

**ポリシー取得：**

- Valkey HGETALLで一括取得
- JSONパースエラーは適切にハンドリング
- キャッシュは行わない（常に最新を取得）

**ルール評価：**

- 配列順の評価を厳守（最初にSSID・時間帯が一致したルールを適用）
- SSID比較は大文字小文字区別なし（`strings.EqualFold`使用）
- ワイルドカード `"*"` の判定を忘れずに
- 時間帯条件（time_min/time_max）省略時は制限なし

**Called-Station-Id解析：**

- コロン区切りでSSID抽出
- 形式が異なる場合の fallback 処理を実装
- 空文字列の場合の扱いを明確に

**VLAN AVP：**

- Tunnel系3属性をセットで追加
- Tagは0（タグなし）で統一
- `layeh.com/radius/rfc2868` パッケージを使用

**MS-MPPE-Key：**

- `oyaguma3/go-eapaka`パッケージの`EncryptMPPEKey`関数を利用
- MSKの分割：先頭32バイト=Recv-Key、後半32バイト=Send-Key
- Request Authenticator（16バイト）の取得を確実に
- 暗号化処理の自前実装は不要

**セキュリティ：**

- ポリシー内容はログに出力しない（allowed_ssids等）
- deny理由は最小限の情報のみログ出力

------

## ■セクション9: セッション管理

### 9.1 概要

本セクションでは、Auth Serverにおけるセッション管理機能を定義する。EAP認証プロセス中の一時的なコンテキスト管理と、認証成功後のセッション作成を扱う。

**対象ファイル：**

| ファイル                         | 責務                               |
| -------------------------------- | ---------------------------------- |
| `internal/session/context.go`    | EAPコンテキストCRUD操作            |
| `internal/session/session.go`    | セッション作成、インデックス管理   |
| `internal/session/errors.go`     | セッションエラー定義               |
| `internal/session/interfaces.go` | セッションインターフェース         |
| `internal/store/valkey.go`       | Valkeyクライアント初期化・共通操作 |
| `internal/store/convert.go`      | Valkey Hash ↔ struct変換          |

**管理対象データ：**

| データ種別           | Valkeyキー          | TTL    | 用途                       |
| -------------------- | ------------------- | ------ | -------------------------- |
| EAPコンテキスト      | `eap:{Trace ID}`    | 60秒   | 認証プロセス中の一時データ |
| アクティブセッション | `sess:{Session ID}` | 24時間 | 認証成功後のセッション情報 |
| ユーザーインデックス | `idx:user:{IMSI}`   | なし   | IMSI→セッション逆引き      |

### 9.2 EAPコンテキスト管理

**ファイル:** `internal/session/context.go`

#### 9.2.1 EAPコンテキスト構造体

セクション6で定義した構造体をValkey操作用に拡張する。

> **注記（状態定義の統一）：**
> - `Stage`フィールドの値は、セクション6.5.1で定義された状態定義（NEW, WAITING_IDENTITY, IDENTITY_RECEIVED, WAITING_VECTOR, CHALLENGE_SENT, RESYNC_SENT, SUCCESS, FAILURE）を使用する
> - 旧仕様の`"identity"`/`"challenge"`値は非推奨

```go
// EAPContext はEAP認証プロセス中の一時データを保持する
type EAPContext struct {
    IMSI                 string `redis:"imsi"`
    Stage                string `redis:"stage"`
    EAPType              uint8  `redis:"eap_type"`
    RAND                 string `redis:"rand"`
    AUTN                 string `redis:"autn"`
    XRES                 string `redis:"xres"`
    Kaut                 string `redis:"k_aut"`
    MSK                  string `redis:"msk"`
    ResyncCount          int    `redis:"resync_count"`
    PermanentIDRequested bool   `redis:"permanent_id_requested"`
}

// 状態定数
// セクション6.5.1で定義された状態定義を使用する
// StateNew, StateWaitingIdentity, StateIdentityReceived, StateWaitingVector,
// StateChallengeSent, StateResyncSent, StateSuccess, StateFailure
```

#### 9.2.2 コンテキストストアインターフェース

```go
// ContextStore はEAPコンテキストの永続化操作を定義する
type ContextStore interface {
    // Create は新しいEAPコンテキストを作成する
    Create(ctx context.Context, traceID string, eapCtx *EAPContext) error
    
    // Get は指定されたTrace IDのEAPコンテキストを取得する
    Get(ctx context.Context, traceID string) (*EAPContext, error)
    
    // Update は既存のEAPコンテキストを部分更新する
    Update(ctx context.Context, traceID string, updates map[string]interface{}) error
    
    // Delete は指定されたTrace IDのEAPコンテキストを削除する
    Delete(ctx context.Context, traceID string) error
    
    // Exists は指定されたTrace IDのEAPコンテキストが存在するか確認する
    Exists(ctx context.Context, traceID string) (bool, error)
}
```

#### 9.2.3 コンテキスト作成

**処理タイミング:** Identity受信時（永続ID確認後）

```go
const (
    EAPContextTTL = 60 * time.Second
    EAPContextKeyPrefix = "eap:"
)

// Create は新しいEAPコンテキストを作成する
func (s *contextStore) Create(ctx context.Context, traceID string, eapCtx *EAPContext) error {
    key := EAPContextKeyPrefix + traceID
    
    // 構造体をマップに変換
    values := structToMap(eapCtx)
    
    // HSET + EXPIRE をパイプラインで実行
    pipe := s.client.Pipeline()
    pipe.HSet(ctx, key, values)
    pipe.Expire(ctx, key, EAPContextTTL)
    
    _, err := pipe.Exec(ctx)
    if err != nil {
        return fmt.Errorf("create eap context: %w", err)
    }
    
    return nil
}
```

**初期値設定：**

| フィールド               | 初期値       | 備考                     |
| ------------------------ | ------------ | ------------------------ |
| `imsi`                   | 抽出したIMSI | 必須                     |
| `stage`                  | `"identity"` | 初期ステージ             |
| `eap_type`               | 23 or 50     | Identity先頭文字から判定 |
| `resync_count`           | 0            | 再同期カウンタ初期化     |
| `permanent_id_requested` | false        | フル認証誘導フラグ       |

#### 9.2.4 コンテキスト取得

**処理タイミング:** Challenge応答受信時（State属性からTrace ID復元）

```go
// Get は指定されたTrace IDのEAPコンテキストを取得する
func (s *contextStore) Get(ctx context.Context, traceID string) (*EAPContext, error) {
    key := EAPContextKeyPrefix + traceID
    
    result, err := s.client.HGetAll(ctx, key).Result()
    if err != nil {
        return nil, fmt.Errorf("get eap context: %w", err)
    }
    
    if len(result) == 0 {
        return nil, ErrContextNotFound
    }
    
    eapCtx := &EAPContext{}
    if err := mapToStruct(result, eapCtx); err != nil {
        return nil, fmt.Errorf("parse eap context: %w", err)
    }
    
    return eapCtx, nil
}
```

**エラー定義：**

```go
var (
    ErrContextNotFound = errors.New("eap context not found")
    ErrContextExpired  = errors.New("eap context expired")
)
```

#### 9.2.5 コンテキスト更新

**処理タイミング:** Vector応答受信後、再同期時

```go
// Update は既存のEAPコンテキストを部分更新する
func (s *contextStore) Update(ctx context.Context, traceID string, updates map[string]interface{}) error {
    key := EAPContextKeyPrefix + traceID
    
    // 存在確認
    exists, err := s.client.Exists(ctx, key).Result()
    if err != nil {
        return fmt.Errorf("check eap context exists: %w", err)
    }
    if exists == 0 {
        return ErrContextNotFound
    }
    
    // HSET + EXPIRE をパイプラインで実行（TTLリフレッシュ）
    pipe := s.client.Pipeline()
    pipe.HSet(ctx, key, updates)
    pipe.Expire(ctx, key, EAPContextTTL)
    
    _, err = pipe.Exec(ctx)
    if err != nil {
        return fmt.Errorf("update eap context: %w", err)
    }
    
    return nil
}
```

**更新パターン：**

| タイミング       | 更新フィールド                                         |
| ---------------- | ------------------------------------------------------ |
| Vector応答受信後 | `rand`, `autn`, `xres`, `k_aut`, `msk`, `stage`        |
| フル認証誘導時   | `permanent_id_requested`                               |
| 再同期時         | `rand`, `autn`, `xres`, `k_aut`, `msk`, `resync_count` |

#### 9.2.6 コンテキスト削除

**処理タイミング:** 認証完了時（成功/失敗問わず）

```go
// Delete は指定されたTrace IDのEAPコンテキストを削除する
func (s *contextStore) Delete(ctx context.Context, traceID string) error {
    key := EAPContextKeyPrefix + traceID
    
    _, err := s.client.Del(ctx, key).Result()
    if err != nil {
        return fmt.Errorf("delete eap context: %w", err)
    }
    
    return nil
}
```

**注記:** TTL（60秒）による自動削除も許容するため、明示的な削除は必須ではない。ただし、認証完了時に即座に削除することでメモリ効率を向上させる。

### 9.3 セッション作成

**ファイル:** `internal/session/session.go`

#### 9.3.1 セッション構造体

```go
// Session は認証成功後のアクティブセッション情報を保持する
type Session struct {
    IMSI         string `redis:"imsi"`
    NasIP        string `redis:"nas_ip"`
    StartTime    int64  `redis:"start_time"`    // Unix timestamp（Acct Startで設定）
    ClientIP     string `redis:"client_ip"`     // Acct Start/Interimで設定
    AcctID       string `redis:"acct_id"`       // Acct Startで設定
    InputOctets  int64  `redis:"input_octets"`  // Acct Interim/Stopで更新
    OutputOctets int64  `redis:"output_octets"` // Acct Interim/Stopで更新
}
```

#### 9.3.2 セッションストアインターフェース

```go
// SessionStore はセッションの永続化操作を定義する
type SessionStore interface {
    // Create は新しいセッションを作成する
    Create(ctx context.Context, sessionID string, sess *Session) error
    
    // Get は指定されたSession IDのセッションを取得する
    Get(ctx context.Context, sessionID string) (*Session, error)
    
    // AddUserIndex はIMSI→SessionIDの逆引きインデックスを追加する
    AddUserIndex(ctx context.Context, imsi string, sessionID string) error
}
```

#### 9.3.3 セッション作成処理

**処理タイミング:** 認証成功（Access-Accept送信前）

```go
const (
    SessionTTL       = 24 * time.Hour
    SessionKeyPrefix = "sess:"
    UserIndexKeyPrefix = "idx:user:"
)

// Create は新しいセッションを作成する
func (s *sessionStore) Create(ctx context.Context, sessionID string, sess *Session) error {
    key := SessionKeyPrefix + sessionID
    
    // 構造体をマップに変換
    values := structToMap(sess)
    
    // HSET + EXPIRE をパイプラインで実行
    pipe := s.client.Pipeline()
    pipe.HSet(ctx, key, values)
    pipe.Expire(ctx, key, SessionTTL)
    
    _, err := pipe.Exec(ctx)
    if err != nil {
        return fmt.Errorf("create session: %w", err)
    }
    
    return nil
}
```

**初期値設定（Auth Server側）：**

| フィールド | 値                       | 備考                    |
| ---------- | ------------------------ | ----------------------- |
| `imsi`     | 認証済みIMSI             | EAPコンテキストから取得 |
| `nas_ip`   | RADIUSパケットの送信元IP | Access-Requestから取得  |

**後続更新（Acct Server側）：**

- `start_time`, `client_ip`, `acct_id` → Acct-Start時
- `input_octets`, `output_octets` → Acct-Interim/Stop時

#### 9.3.4 Session IDの生成

**方針:** Session IDはTrace IDとは別のUUIDを生成する。

```go
// GenerateSessionID は新しいセッションIDを生成する
func GenerateSessionID() string {
    return uuid.New().String()
}
```

**理由：**

- Trace IDはEAP認証プロセスのライフサイクル（数秒〜数十秒）
- Session IDはセッションのライフサイクル（数時間〜24時間）
- 用途が異なるため分離する

**RADIUS Class属性との関係：**

- Session IDはAccess-AcceptのClass属性に格納
- Acct ServerはClass属性からSession IDを復元してセッションを特定

### 9.4 ユーザーインデックス管理

**目的:** IMSIから現在のアクティブセッションを逆引きする。

#### 9.4.1 インデックス追加

```go
// AddUserIndex はIMSI→SessionIDの逆引きインデックスを追加する
func (s *sessionStore) AddUserIndex(ctx context.Context, imsi string, sessionID string) error {
    key := UserIndexKeyPrefix + imsi
    
    _, err := s.client.SAdd(ctx, key, sessionID).Result()
    if err != nil {
        return fmt.Errorf("add user index: %w", err)
    }
    
    return nil
}
```

**注記:** インデックス追加はAcct Server側（Acct-Start受信時）で実行する。Auth Serverではセッション枠の作成のみ行う。

#### 9.4.2 データ型

- **Key:** `idx:user:{IMSI}`
- **Type:** `Set`
- **Members:** Session UUID

**Set型を使用する理由：**

- 同一IMSIで複数セッションが存在する可能性（マルチデバイス）
- 重複追加を自動的に排除
- メンバー削除が効率的

### 9.5 TTL管理

#### 9.5.1 TTL設定一覧

| データ種別           | TTL    | 更新タイミング                   | 備考                                 |
| -------------------- | ------ | -------------------------------- | ------------------------------------ |
| EAPコンテキスト      | 60秒   | 更新操作時にリフレッシュ         | 認証タイムアウト                     |
| セッション           | 24時間 | Acct Interim受信時にリフレッシュ | セッションタイムアウト               |
| ユーザーインデックス | なし   | -                                | セッション削除時に手動クリーンアップ |

#### 9.5.2 TTLリフレッシュ

**EAPコンテキスト：**

```go
// RefreshContextTTL はEAPコンテキストのTTLをリフレッシュする
func (s *contextStore) RefreshTTL(ctx context.Context, traceID string) error {
    key := EAPContextKeyPrefix + traceID
    
    _, err := s.client.Expire(ctx, key, EAPContextTTL).Result()
    if err != nil {
        return fmt.Errorf("refresh eap context ttl: %w", err)
    }
    
    return nil
}
```

**セッション（Acct Server用）：**

```go
// RefreshSessionTTL はセッションのTTLをリフレッシュする
func (s *sessionStore) RefreshTTL(ctx context.Context, sessionID string) error {
    key := SessionKeyPrefix + sessionID
    
    _, err := s.client.Expire(ctx, key, SessionTTL).Result()
    if err != nil {
        return fmt.Errorf("refresh session ttl: %w", err)
    }
    
    return nil
}
```

#### 9.5.3 TTL超過時の動作

**EAPコンテキスト（TTL=60秒）：**

- 自動削除される
- Challenge応答受信時にコンテキスト不在 → `EAP_CONTEXT_NOT_FOUND`ログ、EAP-Failure

**セッション（TTL=24時間）：**

- 自動削除される
- Acct Interim/Stop受信時にセッション不在 → `ACCT_SESSION_EXPIRED`ログ、Accounting-Response返却（D-02準拠）

### 9.6 Valkey操作ユーティリティ

**ファイル:** `internal/store/valkey.go`

#### 9.6.1 クライアント初期化

```go
// ValkeyClient はValkeyへの接続を管理する
type ValkeyClient struct {
    client *redis.Client
}

// NewValkeyClient は新しいValkeyクライアントを作成する
func NewValkeyClient(cfg *config.Config) (*ValkeyClient, error) {
    client := redis.NewClient(&redis.Options{
        Addr:         fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort),
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
        return nil, fmt.Errorf("valkey ping failed: %w", err)
    }
    
    return &ValkeyClient{client: client}, nil
}

// Close はValkeyクライアントを閉じる
func (v *ValkeyClient) Close() error {
    return v.client.Close()
}
```

#### 9.6.2 構造体⇔マップ変換

**ファイル:** `internal/store/convert.go`

```go
// structToMap は構造体をmap[string]interface{}に変換する
func structToMap(v interface{}) map[string]interface{} {
    result := make(map[string]interface{})
    val := reflect.ValueOf(v).Elem()
    typ := val.Type()
    
    for i := 0; i < val.NumField(); i++ {
        field := typ.Field(i)
        tag := field.Tag.Get("redis")
        if tag == "" || tag == "-" {
            continue
        }
        result[tag] = val.Field(i).Interface()
    }
    
    return result
}

// mapToStruct はmap[string]stringを構造体に変換する
func mapToStruct(m map[string]string, v interface{}) error {
    val := reflect.ValueOf(v).Elem()
    typ := val.Type()
    
    for i := 0; i < val.NumField(); i++ {
        field := typ.Field(i)
        tag := field.Tag.Get("redis")
        if tag == "" || tag == "-" {
            continue
        }
        
        strVal, ok := m[tag]
        if !ok {
            continue
        }
        
        if err := setFieldValue(val.Field(i), strVal); err != nil {
            return fmt.Errorf("set field %s: %w", field.Name, err)
        }
    }
    
    return nil
}
```

### 9.7 認証フロー内でのセッション操作

#### 9.7.1 Identity受信時

```go
// handleIdentity はIdentity受信時のセッション操作を行う
func (h *Handler) handleIdentity(ctx context.Context, traceID string, parsed *ParsedIdentity) error {
    // EAPコンテキスト作成
    eapCtx := &session.EAPContext{
        IMSI:    parsed.IMSI,
        Stage:   session.StageIdentity,
        EAPType: parsed.EAPType,
    }
    
    if err := h.contextStore.Create(ctx, traceID, eapCtx); err != nil {
        return fmt.Errorf("create eap context: %w", err)
    }
    
    return nil
}
```

#### 9.7.2 Vector応答受信後

```go
// updateContextWithVector はVector応答でEAPコンテキストを更新する
func (h *Handler) updateContextWithVector(
    ctx context.Context,
    traceID string,
    vectorResp *vector.Response,
    keys interface{}, // AkaKeys or AkaPrimeKeys
) error {
    updates := map[string]interface{}{
        "rand":  hex.EncodeToString(vectorResp.RAND),
        "autn":  hex.EncodeToString(vectorResp.AUTN),
        "xres":  hex.EncodeToString(vectorResp.XRES),
        "stage": session.StageChallenge,
    }
    
    // 鍵情報を追加（型に応じて処理）
    switch k := keys.(type) {
    case eapaka.AkaKeys:
        updates["k_aut"] = hex.EncodeToString(k.K_aut)
        updates["msk"] = hex.EncodeToString(k.MSK)
    case eapaka.AkaPrimeKeys:
        updates["k_aut"] = hex.EncodeToString(k.K_aut)
        updates["msk"] = hex.EncodeToString(k.MSK)
    }
    
    return h.contextStore.Update(ctx, traceID, updates)
}
```

#### 9.7.3 認証成功時

```go
// handleAuthSuccess は認証成功時のセッション操作を行う
func (h *Handler) handleAuthSuccess(
    ctx context.Context,
    traceID string,
    eapCtx *session.EAPContext,
    nasIP string,
) (string, error) {
    // セッションID生成
    sessionID := session.GenerateSessionID()
    
    // セッション作成
    sess := &session.Session{
        IMSI:  eapCtx.IMSI,
        NasIP: nasIP,
    }
    
    if err := h.sessionStore.Create(ctx, sessionID, sess); err != nil {
        return "", fmt.Errorf("create session: %w", err)
    }
    
    // EAPコンテキスト削除（オプション、TTL任せでも可）
    _ = h.contextStore.Delete(ctx, traceID)
    
    return sessionID, nil
}
```

### 9.8 エラーハンドリング

#### 9.8.1 エラー定義

```go
var (
    // EAPコンテキスト関連
    ErrContextNotFound   = errors.New("eap context not found")
    ErrContextExpired    = errors.New("eap context expired")
    ErrContextInvalid    = errors.New("eap context invalid")
    
    // セッション関連
    ErrSessionNotFound   = errors.New("session not found")
    ErrSessionExpired    = errors.New("session expired")
    
    // Valkey関連
    ErrValkeyUnavailable = errors.New("valkey unavailable")
)
```

#### 9.8.2 エラー時の動作

| エラー                 | 検出タイミング       | 対処        | ログ                    |
| ---------------------- | -------------------- | ----------- | ----------------------- |
| `ErrContextNotFound`   | Challenge応答受信時  | EAP-Failure | `EAP_CONTEXT_NOT_FOUND` |
| `ErrContextInvalid`    | コンテキストパース時 | EAP-Failure | `EAP_CONTEXT_INVALID`   |
| `ErrValkeyUnavailable` | 全Valkey操作時       | EAP-Failure | `VALKEY_CONN_ERR`       |

#### 9.8.3 Valkey接続エラー時の再試行

```go
// withRetry はValkey操作を再試行付きで実行する
func withRetry(ctx context.Context, maxRetries int, fn func() error) error {
    var lastErr error
    
    for i := 0; i < maxRetries; i++ {
        if err := fn(); err != nil {
            lastErr = err
            
            // 再試行可能なエラーか判定
            if !isRetryableError(err) {
                return err
            }
            
            // バックオフ
            time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
            continue
        }
        return nil
    }
    
    return fmt.Errorf("max retries exceeded: %w", lastErr)
}

func isRetryableError(err error) bool {
    // 接続エラー、タイムアウトは再試行可能
    return errors.Is(err, redis.ErrClosed) ||
           errors.Is(err, context.DeadlineExceeded)
}
```

### 9.9 ログ出力

| 処理                    | event_id                | レベル | 追加フィールド       |
| ----------------------- | ----------------------- | ------ | -------------------- |
| EAPコンテキスト作成     | -                       | DEBUG  | `trace_id`, `imsi`   |
| EAPコンテキスト取得失敗 | `EAP_CONTEXT_NOT_FOUND` | WARN   | `trace_id`           |
| セッション作成          | `SESSION_CREATED`       | INFO   | `session_id`, `imsi` |
| Valkey接続エラー        | `VALKEY_CONN_ERR`       | ERROR  | `error`              |
| Valkey接続復旧          | `VALKEY_CONN_RESTORED`  | INFO   | -                    |

### 9.10 実装時の注意点まとめ

**EAPコンテキスト：**

- 全フィールドはHex文字列で保存（バイナリデータの安全な永続化）
- TTL（60秒）は更新操作時に毎回リフレッシュ
- 認証完了時の明示的削除は推奨だが必須ではない

**セッション：**

- Session IDとTrace IDは別々に生成
- Auth ServerではIMSIとNasIPのみ設定、他はAcct Serverで更新
- Class属性にSession IDを格納してAcct Serverと連携

**ユーザーインデックス：**

- Set型を使用（複数セッション対応）
- インデックス追加はAcct Server側で実行
- セッション削除時のインデックスクリーンアップはAcct Server側で実行

**Valkey操作：**

- パイプラインでHSET + EXPIREを一括実行（ラウンドトリップ削減）
- 接続エラー時は再試行（最大3回、バックオフ付き）
- 構造体⇔マップ変換にはリフレクションを使用

**セキュリティ：**

- MSK、K_autは絶対にログ出力しない
- EAPコンテキスト全体のダンプは禁止

------

## ■セクション10: Go実装定義

### 10.1 概要

本セクションでは、Auth Serverの実装に必要なGo型定義を一覧化する。セクション1-9で定義した構造体、インターフェース、エラー型、定数を統合し、実装時の参照として提供する。

**構成：**

| 項目 | 内容                         |
| ---- | ---------------------------- |
| 10.2 | 定数定義                     |
| 10.3 | 設定・構成                   |
| 10.4 | データ構造体                 |
| 10.5 | インターフェース             |
| 10.6 | エラー型                     |
| 10.7 | パッケージ別エクスポート一覧 |

### 10.2 定数定義

#### 10.2.1 タイムアウト・TTL

**ファイル:** `internal/config/constants.go`

```go
package config

import "time"

// Valkey接続設定（D-06準拠）
const (
    ValkeyConnectTimeout = 3 * time.Second
    ValkeyCommandTimeout = 2 * time.Second
    ValkeyPoolSize       = 10
)

// Vector Gateway接続設定（D-06準拠）
const (
    VectorConnectTimeout = 2 * time.Second
    VectorRequestTimeout = 5 * time.Second
)

// セッション管理（D-02準拠）
const (
    EAPContextTTL = 60 * time.Second  // EAPコンテキストTTL
    SessionTTL    = 24 * time.Hour    // セッションTTL
)

// 再同期上限（D-02準拠）
const (
    MaxResyncCount = 32 // SQN INDフィールド1サイクル分
)
```

#### 10.2.2 Valkeyキープレフィックス

**ファイル:** `internal/store/keys.go`

```go
package store

// Valkeyキープレフィックス（D-02準拠）
const (
    KeyPrefixSubscriber  = "sub:"      // 加入者情報
    KeyPrefixClient      = "client:"   // RADIUSクライアント設定
    KeyPrefixPolicy      = "policy:"   // 認可ポリシー
    KeyPrefixEAPContext  = "eap:"      // EAP認証コンテキスト
    KeyPrefixSession     = "sess:"     // アクティブセッション
    KeyPrefixUserIndex   = "idx:user:" // ユーザー検索インデックス
)
```

#### 10.2.3 EAP関連定数

**ファイル:** `internal/eap/constants.go`

```go
package eap

// EAPコンテキストステージ
const (
    StageIdentity  = "identity"
    StageChallenge = "challenge"
)

// Identity種別（先頭文字）
const (
    IdentityPrefixAKAPermanent      = '0'
    IdentityPrefixAKAPseudonym      = '2'
    IdentityPrefixAKAReauth         = '4'
    IdentityPrefixAKAPrimePermanent = '6'
    IdentityPrefixAKAPrimePseudonym = '7'
    IdentityPrefixAKAPrimeReauth    = '8'
)

// EAP-SIM（非対応）
const (
    IdentityPrefixSIMPermanent = '1'
    IdentityPrefixSIMPseudonym = '3'
    IdentityPrefixSIMReauth    = '5'
)
```

#### 10.2.4 HTTPヘッダ

**ファイル:** `internal/vector/constants.go`

```go
package vector

// HTTPヘッダ名
const (
    HeaderTraceID     = "X-Trace-ID"
    HeaderContentType = "Content-Type"
)

// Content-Type
const (
    ContentTypeJSON = "application/json"
)
```

### 10.3 設定・構成

#### 10.3.1 アプリケーション設定

**ファイル:** `internal/config/config.go`

```go
package config

// Config はアプリケーション設定を保持する
type Config struct {
    // Valkey接続設定
    RedisHost string `envconfig:"REDIS_HOST" required:"true"`
    RedisPort string `envconfig:"REDIS_PORT" required:"true"`
    RedisPass string `envconfig:"REDIS_PASS" required:"true"`

    // Vector Gateway設定
    VectorAPIURL string `envconfig:"VECTOR_API_URL" required:"true"`

    // RADIUS設定
    RadiusSecret string `envconfig:"RADIUS_SECRET"`
    ListenAddr   string `envconfig:"LISTEN_ADDR" default:":1812"`

    // EAP-AKA'設定
    NetworkName string `envconfig:"EAP_AKA_PRIME_NETWORK_NAME" default:"WLAN"`

    // ログ設定
    LogMaskIMSI bool `envconfig:"LOG_MASK_IMSI" default:"true"`
}

// Load は環境変数から設定を読み込む
func Load() (*Config, error)

// ValkeyAddr はValkey接続アドレスを返す
func (c *Config) ValkeyAddr() string
```

### 10.4 データ構造体

#### 10.4.1 EAPコンテキスト

**ファイル:** `internal/session/context.go`

```go
package session

// EAPContext はEAP認証プロセス中の一時データを保持する
type EAPContext struct {
    // 基本情報
    IMSI    string `redis:"imsi"`
    Stage   string `redis:"stage"`
    EAPType uint8  `redis:"eap_type"` // 23=AKA, 50=AKA'

    // Vector情報（Hex文字列）
    RAND string `redis:"rand"`
    AUTN string `redis:"autn"` // EAP-AKA'のCK'/IK'導出に必要
    XRES string `redis:"xres"`

    // 導出済みキー（Hex文字列）
    Kaut string `redis:"k_aut"` // AT_MAC検証用
    MSK  string `redis:"msk"`   // MS-MPPE-Key導出用

    // 状態管理
    ResyncCount          int  `redis:"resync_count"`
    PermanentIDRequested bool `redis:"permanent_id_requested"`
}
```

#### 10.4.2 セッション

**ファイル:** `internal/session/session.go`

```go
package session

// Session は認証成功後のアクティブセッション情報を保持する
type Session struct {
    IMSI         string `redis:"imsi"`
    NasIP        string `redis:"nas_ip"`
    StartTime    int64  `redis:"start_time"`    // Unix timestamp
    ClientIP     string `redis:"client_ip"`     // Framed-IP-Address
    AcctID       string `redis:"acct_id"`       // Acct-Session-Id
    InputOctets  int64  `redis:"input_octets"`
    OutputOctets int64  `redis:"output_octets"`
}
```

#### 10.4.3 ポリシー

**ファイル:** `internal/policy/types.go`

```go
package policy

// Policy は認可ポリシーを表す
type Policy struct {
    Rules   []PolicyRule
    Default string // "allow" or "deny"
}

// PolicyRule は個別の認可ルールを表す
type PolicyRule struct {
    SSID    string `json:"ssid"`
    Action  string `json:"action"`             // "allow" or "deny"
    TimeMin string `json:"time_min,omitempty"` // "HH:MM" 形式
    TimeMax string `json:"time_max,omitempty"` // "HH:MM" 形式
}

// EvaluationResult はポリシー評価結果を表す
type EvaluationResult struct {
    Allowed     bool
    MatchedRule *PolicyRule // nilの場合はdefault適用
    DenyReason  string      // Deny時の理由
}
```

#### 10.4.4 RADIUSクライアント

**ファイル:** `internal/store/client.go`

```go
package store

// RadiusClient はRADIUSクライアント設定を表す
type RadiusClient struct {
    IP     string `redis:"-"`
    Secret string `redis:"secret"`
    Name   string `redis:"name"`
    Vendor string `redis:"vendor"`
}
```

#### 10.4.5 Vector Gateway通信

**ファイル:** `internal/vector/types.go`

```go
package vector

// VectorRequest はVector Gateway へのリクエストを表す
type VectorRequest struct {
    IMSI       string      `json:"imsi"`
    ResyncInfo *ResyncInfo `json:"resync_info,omitempty"`
}

// ResyncInfo は再同期情報を表す
type ResyncInfo struct {
    RAND string `json:"rand"` // Hex文字列
    AUTS string `json:"auts"` // Hex文字列
}

// VectorResponse はVector Gatewayからのレスポンスを表す
type VectorResponse struct {
    RAND []byte // 16バイト
    AUTN []byte // 16バイト
    XRES []byte // 4-16バイト
    CK   []byte // 16バイト
    IK   []byte // 16バイト
}

// vectorResponseJSON はJSONパース用の内部構造体
type vectorResponseJSON struct {
    RAND string `json:"rand"`
    AUTN string `json:"autn"`
    XRES string `json:"xres"`
    CK   string `json:"ck"`
    IK   string `json:"ik"`
}

// ProblemDetails はRFC 7807エラーレスポンスを表す
type ProblemDetails struct {
    Type   string `json:"type"`
    Title  string `json:"title"`
    Detail string `json:"detail"`
    Status int    `json:"status"`
}
```

#### 10.4.6 Identity解析結果

**ファイル:** `internal/eap/identity.go`

```go
package eap

// IdentityType はIdentityの種別を表す
type IdentityType int

const (
    IdentityTypePermanentAKA      IdentityType = iota // 0: EAP-AKA永続ID
    IdentityTypePseudonymAKA                          // 2: EAP-AKA仮名
    IdentityTypeReauthAKA                             // 4: EAP-AKA再認証ID
    IdentityTypePermanentAKAPrime                     // 6: EAP-AKA'永続ID
    IdentityTypePseudonymAKAPrime                     // 7: EAP-AKA'仮名
    IdentityTypeReauthAKAPrime                        // 8: EAP-AKA'再認証ID
    IdentityTypeUnsupported                           // EAP-SIM等
    IdentityTypeInvalid                               // 不正形式
)

// ParsedIdentity はIdentity文字列の解析結果を表す
type ParsedIdentity struct {
    Type    IdentityType
    IMSI    string // 永続IDの場合のみ有効
    Raw     string // 元のIdentity文字列
    Realm   string // @以降の部分
    EAPType uint8  // eapaka.TypeAKA or eapaka.TypeAKAPrime
}

// RequiresFullAuth は仮名/再認証IDでフル認証誘導が必要か判定する
func (p *ParsedIdentity) RequiresFullAuth() bool

// IsPermanent は永続IDかどうかを判定する
func (p *ParsedIdentity) IsPermanent() bool

// IsAKAPrime はEAP-AKA'かどうかを判定する
func (p *ParsedIdentity) IsAKAPrime() bool
```

#### 10.4.7 RADIUS処理用コンテキスト

**ファイル:** `internal/radius/types.go`

```go
package radius

// RequestContext はRADIUSリクエスト処理中のコンテキスト情報を保持する
type RequestContext struct {
    TraceID     string
    SrcIP       string
    NASIdentifier string
    SSID        string
    ProxyStates [][]byte
}
```

### 10.5 インターフェース

#### 10.5.1 セッション管理

**ファイル:** `internal/session/interfaces.go`

```go
package session

import "context"

// ContextStore はEAPコンテキストの永続化操作を定義する
type ContextStore interface {
    // Create は新しいEAPコンテキストを作成する
    Create(ctx context.Context, traceID string, eapCtx *EAPContext) error

    // Get は指定されたTrace IDのEAPコンテキストを取得する
    Get(ctx context.Context, traceID string) (*EAPContext, error)

    // Update は既存のEAPコンテキストを部分更新する
    Update(ctx context.Context, traceID string, updates map[string]interface{}) error

    // Delete は指定されたTrace IDのEAPコンテキストを削除する
    Delete(ctx context.Context, traceID string) error

    // Exists は指定されたTrace IDのEAPコンテキストが存在するか確認する
    Exists(ctx context.Context, traceID string) (bool, error)
}

// SessionStore はセッションの永続化操作を定義する
type SessionStore interface {
    // Create は新しいセッションを作成する
    Create(ctx context.Context, sessionID string, sess *Session) error

    // Get は指定されたSession IDのセッションを取得する
    Get(ctx context.Context, sessionID string) (*Session, error)

    // AddUserIndex はIMSI→SessionIDの逆引きインデックスを追加する
    AddUserIndex(ctx context.Context, imsi string, sessionID string) error
}
```

#### 10.5.2 ポリシー管理

**ファイル:** `internal/policy/interfaces.go`

```go
package policy

import "context"

// PolicyStore はポリシーデータへのアクセスを定義する
type PolicyStore interface {
    // GetPolicy は指定されたIMSIのポリシーを取得する
    GetPolicy(ctx context.Context, imsi string) (*Policy, error)
}

// Evaluator はポリシー評価を定義する
type Evaluator interface {
    // Evaluate はポリシーを評価し結果を返す
    Evaluate(policy *Policy, nasID string, ssid string) *EvaluationResult
}
```

#### 10.5.3 RADIUSクライアント管理

**ファイル:** `internal/store/interfaces.go`

```go
package store

import "context"

// ClientStore はRADIUSクライアントデータへのアクセスを定義する
type ClientStore interface {
    // GetClientSecret は指定されたIPのShared Secretを取得する
    // 未登録の場合は空文字列とnilを返す
    GetClientSecret(ctx context.Context, ip string) (string, error)
}
```

#### 10.5.4 Vector Gateway連携

**ファイル:** `internal/vector/interfaces.go`

```go
package vector

import "context"

// VectorClient はVector Gatewayへのアクセスを定義する
type VectorClient interface {
    // GetVector は認証ベクターを取得する
    GetVector(ctx context.Context, req *VectorRequest) (*VectorResponse, error)
}
```

#### 10.5.5 Valkeyクライアント

**ファイル:** `internal/store/valkey.go`

```go
package store

import (
    "context"
    "github.com/redis/go-redis/v9"
)

// ValkeyClient はValkeyへの接続を管理する
type ValkeyClient struct {
    client *redis.Client
}

// NewValkeyClient は新しいValkeyクライアントを作成する
func NewValkeyClient(cfg *config.Config) (*ValkeyClient, error)

// Close はValkeyクライアントを閉じる
func (v *ValkeyClient) Close() error

// Client は内部のredis.Clientを返す
func (v *ValkeyClient) Client() *redis.Client

// Ping は接続確認を行う
func (v *ValkeyClient) Ping(ctx context.Context) error
```

### 10.6 エラー型

#### 10.6.1 セッション関連エラー

**ファイル:** `internal/session/errors.go`

```go
package session

import "errors"

var (
    // EAPコンテキスト関連
    ErrContextNotFound = errors.New("eap context not found")
    ErrContextExpired  = errors.New("eap context expired")
    ErrContextInvalid  = errors.New("eap context invalid")

    // セッション関連
    ErrSessionNotFound = errors.New("session not found")
    ErrSessionExpired  = errors.New("session expired")
)
```

#### 10.6.2 ポリシー関連エラー

**ファイル:** `internal/policy/errors.go`

```go
package policy

import "errors"

var (
    ErrPolicyNotFound = errors.New("policy not found")
    ErrPolicyInvalid  = errors.New("policy invalid")
    ErrPolicyDenied   = errors.New("policy denied")
)
```

#### 10.6.3 Vector Gateway関連エラー

**ファイル:** `internal/vector/errors.go`

```go
package vector

import (
    "errors"
    "fmt"
)

// センチネルエラー
var (
    ErrInvalidResponse = errors.New("invalid response from vector gateway")
    ErrTraceIDMissing  = errors.New("trace id missing in context")
)

// APIError はHTTP APIエラーを表す
type APIError struct {
    StatusCode int
    Message    string
    Details    *ProblemDetails
}

func (e *APIError) Error() string {
    if e.Details != nil {
        return fmt.Sprintf("vector api error: %d %s - %s", e.StatusCode, e.Details.Title, e.Details.Detail)
    }
    return fmt.Sprintf("vector api error: %d %s", e.StatusCode, e.Message)
}

// IsNotFound はIMSI未登録エラーかどうかを判定する
func (e *APIError) IsNotFound() bool {
    return e.StatusCode == 404
}

// IsBadRequest はリクエスト不正エラーかどうかを判定する
func (e *APIError) IsBadRequest() bool {
    return e.StatusCode == 400
}

// IsServerError はサーバーエラーかどうかを判定する
func (e *APIError) IsServerError() bool {
    return e.StatusCode >= 500
}

// ConnectionError は接続エラーを表す
type ConnectionError struct {
    Cause error
}

func (e *ConnectionError) Error() string {
    return fmt.Sprintf("connection error: %v", e.Cause)
}

func (e *ConnectionError) Unwrap() error {
    return e.Cause
}
```

#### 10.6.4 EAP処理関連エラー

**ファイル:** `internal/eap/errors.go`

```go
package eap

import "errors"

var (
    // Identity解析エラー
    ErrInvalidIdentity     = errors.New("invalid identity format")
    ErrUnsupportedIdentity = errors.New("unsupported identity type")
    ErrMissingRealm        = errors.New("missing realm in identity")

    // Challenge検証エラー
    ErrMACInvalid        = errors.New("AT_MAC verification failed")
    ErrRESNotFound       = errors.New("AT_RES not found")
    ErrRESLengthMismatch = errors.New("AT_RES length mismatch")
    ErrRESMismatch       = errors.New("AT_RES mismatch")

    // AT_KDFエラー
    ErrKDFNotSupported = errors.New("unsupported KDF value")

    // 再同期エラー
    ErrAUTSNotFound     = errors.New("AT_AUTS not found")
    ErrResyncLimitExceeded = errors.New("resync limit exceeded")

    // 状態エラー
    ErrInvalidState = errors.New("invalid eap state")
)
```

#### 10.6.5 Store関連エラー

**ファイル:** `internal/store/errors.go`

```go
package store

import "errors"

var (
    ErrValkeyUnavailable = errors.New("valkey unavailable")
    ErrKeyNotFound       = errors.New("key not found")
)
```

### 10.7 パッケージ別エクスポート一覧

#### 10.7.1 internal/config

| エクスポート           | 種別   | 説明                         |
| ---------------------- | ------ | ---------------------------- |
| `Config`               | struct | アプリケーション設定         |
| `Load()`               | func   | 環境変数から設定読み込み     |
| `ValkeyConnectTimeout` | const  | Valkey接続タイムアウト       |
| `ValkeyCommandTimeout` | const  | Valkeyコマンドタイムアウト   |
| `VectorConnectTimeout` | const  | Vector接続タイムアウト       |
| `VectorRequestTimeout` | const  | Vectorリクエストタイムアウト |
| `EAPContextTTL`        | const  | EAPコンテキストTTL           |
| `SessionTTL`           | const  | セッションTTL                |
| `MaxResyncCount`       | const  | 再同期上限                   |

#### 10.7.2 internal/server

| エクスポート        | 種別   | 説明             |
| ------------------- | ------ | ---------------- |
| `Server`            | struct | RADIUSサーバー   |
| `NewServer()`       | func   | サーバー作成     |
| `ListenAndServe()`  | method | サーバー起動     |
| `Shutdown()`        | method | サーバー停止     |
| `Handler`           | struct | RADIUSハンドラ   |
| `NewHandler()`      | func   | ハンドラ作成     |
| `SecretSource`      | struct | Secret解決       |
| `NewSecretSource()` | func   | SecretSource作成 |

#### 10.7.3 internal/radius

| エクスポート                   | 種別   | 説明                   |
| ------------------------------ | ------ | ---------------------- |
| `GetEAPMessage()`              | func   | EAP-Message取得        |
| `GetState()`                   | func   | State属性取得          |
| `GetNASIdentifier()`           | func   | NAS-Identifier取得     |
| `GetCalledStationID()`         | func   | Called-Station-Id取得  |
| `ExtractSSID()`                | func   | SSIDを抽出             |
| `ExtractProxyStates()`         | func   | Proxy-State抽出        |
| `ApplyProxyStates()`           | func   | Proxy-State適用        |
| `BuildAccept()`                | func   | Access-Accept構築      |
| `BuildReject()`                | func   | Access-Reject構築      |
| `BuildChallenge()`             | func   | Access-Challenge構築   |
| `VerifyMessageAuthenticator()` | func   | MA検証                 |
| `SignMessageAuthenticator()`   | func   | MA設定                 |
| `HandleStatusServer()`         | func   | Status-Server応答      |
| `RequestContext`               | struct | リクエストコンテキスト |

#### 10.7.4 internal/eap

| エクスポート      | 種別   | 説明              |
| ----------------- | ------ | ----------------- |
| `ParseIdentity()` | func   | Identity解析      |
| `ParsedIdentity`  | struct | Identity解析結果  |
| `IdentityType`    | type   | Identity種別      |
| `StageIdentity`   | const  | Identityステージ  |
| `StageChallenge`  | const  | Challengeステージ |
| `Err*`            | var    | 各種エラー        |

#### 10.7.5 internal/eap/aka

| エクスポート                | 種別 | 説明                  |
| --------------------------- | ---- | --------------------- |
| `BuildChallenge()`          | func | AKA Challenge構築     |
| `VerifyChallengeResponse()` | func | AKA Challenge応答検証 |
| `DeriveKeys()`              | func | AKA鍵導出（ラッパー） |

#### 10.7.6 internal/eap/akaprime

| エクスポート                | 種別 | 説明                    |
| --------------------------- | ---- | ----------------------- |
| `BuildChallenge()`          | func | AKA' Challenge構築      |
| `VerifyChallengeResponse()` | func | AKA' Challenge応答検証  |
| `DeriveCKPrimeIKPrime()`    | func | CK'/IK'導出（ラッパー） |
| `DeriveKeys()`              | func | AKA'鍵導出（ラッパー）  |

#### 10.7.7 internal/vector

| エクスポート      | 種別      | 説明                         |
| ----------------- | --------- | ---------------------------- |
| `Client`          | struct    | Vector Gatewayクライアント   |
| `NewClient()`     | func      | クライアント作成             |
| `VectorClient`    | interface | クライアントインターフェース |
| `VectorRequest`   | struct    | リクエスト                   |
| `VectorResponse`  | struct    | レスポンス                   |
| `ResyncInfo`      | struct    | 再同期情報                   |
| `APIError`        | struct    | APIエラー                    |
| `ConnectionError` | struct    | 接続エラー                   |

#### 10.7.8 internal/policy

| エクスポート       | 種別      | 説明                 |
| ------------------ | --------- | -------------------- |
| `Policy`           | struct    | ポリシー             |
| `PolicyRule`       | struct    | ポリシールール       |
| `EvaluationResult` | struct    | 評価結果             |
| `PolicyStore`      | interface | ポリシーストア       |
| `Evaluator`        | interface | 評価インターフェース |
| `NewEvaluator()`   | func      | 評価器作成           |
| `Err*`             | var       | 各種エラー           |

#### 10.7.9 internal/session

| エクスポート          | 種別      | 説明                   |
| --------------------- | --------- | ---------------------- |
| `EAPContext`          | struct    | EAPコンテキスト        |
| `Session`             | struct    | セッション             |
| `ContextStore`        | interface | コンテキストストア     |
| `SessionStore`        | interface | セッションストア       |
| `NewContextStore()`   | func      | コンテキストストア作成 |
| `NewSessionStore()`   | func      | セッションストア作成   |
| `GenerateSessionID()` | func      | セッションID生成       |
| `Err*`                | var       | 各種エラー             |

#### 10.7.10 internal/store

| エクスポート        | 種別      | 説明                           |
| ------------------- | --------- | ------------------------------ |
| `ValkeyClient`      | struct    | Valkeyクライアント             |
| `NewValkeyClient()` | func      | クライアント作成               |
| `ClientStore`       | interface | RADIUSクライアントストア       |
| `NewClientStore()`  | func      | クライアントストア作成         |
| `StructToMap()`     | func      | 構造体→map変換（convert.go）  |
| `MapToStruct()`     | func      | map→構造体変換（convert.go）  |
| `Key*`              | const     | キープレフィックス             |
| `Err*`              | var       | 各種エラー                     |

#### 10.7.11 internal/engine

| エクスポート      | 種別   | 説明                                         |
| ----------------- | ------ | -------------------------------------------- |
| `Engine`          | struct | 認証エンジン（EAP処理オーケストレーション）   |

#### 10.7.12 internal/logging

| エクスポート      | 種別 | 説明             |
| ----------------- | ---- | ---------------- |
| `MaskIMSI()`      | func | IMSIマスキング   |

### 10.8 依存関係図

```
main.go
    │
    ├── config.Load()
    │
    └── server.NewServer()
            │
            ├── store.NewValkeyClient()
            │       │
            │       ├── session.NewContextStore()
            │       ├── session.NewSessionStore()
            │       ├── store.NewClientStore()
            │       └── store.NewPolicyStore()
            │
            ├── vector.NewClient()
            │
            ├── policy.NewEvaluator()
            │
            ├── engine.NewEngine()
            │
            └── server.NewHandler()
                    │
                    ├── radius.*  (パケット処理)
                    ├── logging.* (IMSIマスキング等)
                    │
                    └── engine.*  (認証エンジン)
                            │
                            └── eap.*  (EAP処理)
                                    │
                                    ├── eap/aka.*
                                    └── eap/akaprime.*
```

### 10.9 go-eapaka パッケージ利用型

**参照パッケージ:** `github.com/oyaguma3/go-eapaka`

Auth Server内で直接参照する外部パッケージの型：

| 型                    | 用途                    |
| --------------------- | ----------------------- |
| `eapaka.Packet`       | EAPパケット             |
| `eapaka.AkaKeys`      | EAP-AKA鍵セット         |
| `eapaka.AkaPrimeKeys` | EAP-AKA'鍵セット        |
| `eapaka.Attribute`    | EAP属性インターフェース |
| `eapaka.AtRand`       | AT_RAND属性             |
| `eapaka.AtAutn`       | AT_AUTN属性             |
| `eapaka.AtRes`        | AT_RES属性              |
| `eapaka.AtAuts`       | AT_AUTS属性             |
| `eapaka.AtMac`        | AT_MAC属性              |
| `eapaka.AtIdentity`   | AT_IDENTITY属性         |
| `eapaka.AtKdf`        | AT_KDF属性              |
| `eapaka.AtKdfInput`   | AT_KDF_INPUT属性        |

**利用関数：**

| 関数                               | 用途                 |
| ---------------------------------- | -------------------- |
| `eapaka.Parse()`                   | EAPパケットパース    |
| `eapaka.DeriveKeysAKA()`           | EAP-AKA鍵導出        |
| `eapaka.DeriveCKPrimeIKPrime()`    | CK'/IK'導出          |
| `eapaka.DeriveKeysAKAPrime()`      | EAP-AKA'鍵導出       |
| `eapaka.EncryptMPPEKey()`          | MS-MPPE-Key暗号化    |
| `eapaka.KdfValuesFromAttributes()` | AT_KDF値抽出         |
| `Packet.Marshal()`                 | パケットシリアライズ |
| `Packet.CalculateAndSetMac()`      | MAC計算・設定        |
| `Packet.VerifyMac()`               | MAC検証              |

### 10.10 実装時の注意点

**命名規則：**

- 構造体: PascalCase（`EAPContext`, `PolicyRule`）
- インターフェース: 末尾に動詞または`er`（`ContextStore`, `Evaluator`）
- エラー変数: `Err`プレフィックス（`ErrContextNotFound`）
- 定数: PascalCaseまたはALL_CAPS（`StageIdentity`, `CB_TIMEOUT`）

**redisタグ：**

- 構造体フィールドには`redis:"field_name"`タグを付与
- JSONとの互換性が必要な場合は`json`タグも併記
- Valkeyキー自体はタグ対象外（`redis:"-"`）

**エラー処理：**

- センチネルエラーは`errors.New`で定義
- 詳細情報が必要なエラーはカスタム構造体
- `errors.Is`/`errors.As`で判定可能に
- 元エラーは`Unwrap`でアクセス可能に

**インターフェース：**

- 必要最小限のメソッドのみ定義
- 実装側ではなく利用側のパッケージで定義
- モック生成ツール（`mockgen`等）との互換性を考慮


## 改訂履歴

| 版数 | 日付 | 内容 |
|------|------|------|
| r1 | 2026-01-12 | 初版作成 |
| r2 | 2026-01-12 | D-03 r3との整合: 状態定義をNEW/WAITING_IDENTITY/IDENTITY_RECEIVED/WAITING_VECTOR/CHALLENGE_SENT/RESYNC_SENT/SUCCESS/FAILUREに変更、状態遷移図をD-03準拠に更新、Post-Auth Policy評価のデフォルトポリシー分岐を反映。AT_BIDDINGのPoC非サポートを明記（RFC 9048準拠）。CK/IK非保存方針（セキュリティ）を追記。セクション9.2.1のStage定数をセクション6.5.1の状態定義に統一。 |
| r3 | 2026-01-18 | IMSIマスキング設定追加: セクション3.1に環境変数LOG_MASK_IMSI追加、セクション3.2/10.3.1の設定構造体更新、セクション3.5新設（マスキング仕様・実装・適用箇所）、関連ドキュメント参照バージョン更新 |
| r4 | 2026-01-20 | UUID仕様明記: セクション1.6にSession UUID用語追加、セクション8.6.4にRFC 4122準拠フォーマット・生成パッケージ・Acct Server連携を追記 |
| r4 | 2026-01-20 | セッション管理セクション追加（セクション9新設）、UUID仕様明記（RFC 4122準拠、36文字）、IMSIマスキング対応 |
| r5 | 2026-01-21 | 互換性エイリアス削除: セクション9.2.1から旧仕様の互換性エイリアス（StageIdentity, StageChallenge）を削除。新規実装では新状態定数のみを使用する方針に統一。 |
| r6 | 2026-01-26 | インフラ基盤統一: セクション2.6新設（Dockerfile方針 - ベースイメージdebian:bookworm-slim、curl/ca-certificates導入）、環境変数RADIUS_SECRETの統一に関する注記追加。これに伴い、旧 2.6 ファイル別責務詳細 のセクション番号を 2.7 に移行。 |
| r7 | 2026-01-27 | API接続設計統一: VECTOR_API_URL環境変数の説明にD-03参照と設定例を追加、関連ドキュメント版数更新（D-03 r3→r4） |
| r8 | 2026-01-27 | ヘルスチェック整合性修正: セクション2.6.1 Dockerfileに`procps`パッケージ追加、セクション2.6.3必須パッケージに`procps`追記（pgrep用） |
| r9 | 2026-02-18 | ディレクトリ構造全面更新、ポリシー評価ロジック更新、関連ドキュメント版数更新 |
