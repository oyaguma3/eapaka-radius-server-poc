# D-10 Acct Server詳細設計書 (r5)

## ■セクション1: 概要

### 1.1 目的

本ドキュメントは、EAP-AKA RADIUS PoC環境における課金サーバー「Acct Server」の実装レベル設計を定義する。

### 1.2 スコープ

**本書で扱う範囲：**

| 範囲 | 内容 |
|------|------|
| RADIUS Accounting処理 | UDP 1813受信、パケットパース、応答生成 |
| セッション状態管理 | Start/Interim/Stop処理、TTL管理 |
| 重複・順序異常検出 | Acct-Session-Idベースの検出ロジック |
| Proxy-State処理 | RFC 2866準拠のエコーバック |
| IMSIマスキング | ログ出力時のプライバシー保護 |

### 1.3 関連ドキュメント

| No. | ドキュメント | 参照内容 |
|-----|-------------|---------|
| D-01 | ミニPC版設計仕様書 (r9) | システム構成、パッケージ利用マップ |
| D-02 | Valkeyデータ設計仕様書 (r10) | データ構造、キー設計、Go構造体 |
| D-03 | RADIUS認証フロー設計書 (r5) | 認証フロー |
| D-04 | ログ仕様設計書 (r13) | event_id定義、ログフォーマット、IMSIマスキング |
| D-05 | Valkeyキー・TTL設計書 (r5) | キー設計、TTL管理 |
| D-06 | エラーハンドリング詳細設計書 (r6) | エラー分類、タイムアウト、リトライ戦略 |
| D-07 | AKA Vector Server詳細設計書 (r3) | AKA認証ベクター生成 |
| D-08 | インフラ設定・運用設計書 (r10) | Docker Compose設定、環境変数 |
| D-09 | Auth Server詳細設計書 (r4) | セッション作成処理（Class属性設定） |
| E-02 | コーディング規約（簡易版） | コーディング規約 |
| E-03 | ドキュメント管理規約 (r2) | ドキュメント管理 |

### 1.4 PoC対象外機能

以下の機能は本PoCでは実装対象外とする。

| 機能 | RFC | 説明 | 備考 |
|------|-----|------|------|
| Acct-On/Acct-Off | RFC 2866 | NAS起動/停止通知 | `RADIUS_UNKNOWN_CODE` でログ記録し破棄 |
| Acct-Delay-Time考慮 | RFC 2866 | NASでのパケット滞留時間補正 | 精密なタイムスタンプ管理は将来検討 |
| 複数Class属性 | RFC 2865 | 複数Class属性の処理 | 単一UUIDのみ対応 |

### 1.5 準拠規格

| 規格 | 内容 | 対応範囲 |
|------|------|---------|
| RFC 2865 | RADIUS | Proxy-State処理 |
| RFC 2866 | RADIUS Accounting | Accounting-Request/Response, Acct-Status-Type (Start/Interim/Stop) |
| RFC 5997 | Status-Server | ヘルスチェック応答 |

### 1.6 用語定義

| 用語 | 説明 |
|------|------|
| Acct-Session-Id | NASが生成するセッション識別子（RADIUS属性） |
| Acct-Status-Type | 課金イベント種別（1:Start, 2:Stop, 3:Interim-Update） |
| Class | Auth Serverが設定したセッションUUID（RADIUS属性、36バイト） |
| Session UUID | Auth Serverが生成したRFC 4122準拠UUID（ハイフン含む36文字） |
| Proxy-State | プロキシ経由時に保持される属性（エコーバック必須） |

---

## ■セクション2: パッケージ構成

### 2.1 ディレクトリ構造

```
apps/acct-server/
├── main.go                           # エントリーポイント
└── internal/
    ├── acct/
    │   ├── duplicate.go              # 重複・順序異常検出
    │   ├── duplicate_test.go         # duplicate.goのテスト
    │   ├── errors.go                 # acctパッケージエラー定義
    │   ├── errors_test.go            # errors.goのテスト
    │   ├── interfaces.go             # acctパッケージインターフェース定義
    │   ├── interim.go                # Acct-Interim処理
    │   ├── interim_test.go           # interim.goのテスト
    │   ├── processor.go              # Accounting処理メインロジック
    │   ├── start.go                  # Acct-Start処理
    │   ├── start_test.go             # start.goのテスト
    │   ├── stop.go                   # Acct-Stop処理
    │   └── stop_test.go              # stop.goのテスト
    ├── config/
    │   ├── config.go                 # 環境変数読み込み、設定構造体
    │   ├── config_test.go            # config.goのテスト
    │   └── constants.go              # 定数定義
    ├── logging/
    │   ├── mask.go                    # IMSIマスキング処理
    │   └── mask_test.go              # mask.goのテスト
    ├── mocks/
    │   ├── acct_mock.go              # acctパッケージモック
    │   ├── session_mock.go           # sessionパッケージモック
    │   └── store_mock.go             # storeパッケージモック
    ├── radius/
    │   ├── attributes.go             # 属性抽出ヘルパー
    │   ├── attributes_test.go        # attributes.goのテスト
    │   ├── authenticator.go          # Request Authenticator検証
    │   ├── authenticator_test.go     # authenticator.goのテスト
    │   ├── message_authenticator.go  # Message-Authenticator処理
    │   ├── message_authenticator_test.go # message_authenticator.goのテスト
    │   ├── proxystate.go             # Proxy-State処理
    │   ├── response.go               # Accounting-Response生成
    │   ├── response_test.go          # response.goのテスト
    │   ├── status.go                 # Status-Server処理
    │   ├── status_test.go            # status.goのテスト
    │   └── types.go                  # radiusパッケージ型定義
    ├── server/
    │   ├── handler.go                # radius.Handler実装、処理振り分け
    │   ├── handler_test.go           # handler.goのテスト
    │   ├── secret.go                 # radius.SecretSource実装
    │   ├── secret_test.go            # secret.goのテスト
    │   └── server.go                 # PacketServer設定・起動・シャットダウン
    ├── session/
    │   ├── errors.go                 # sessionパッケージエラー定義
    │   ├── identifier.go             # IMSI取得ロジック
    │   ├── identifier_test.go        # identifier.goのテスト
    │   ├── interfaces.go             # sessionパッケージインターフェース定義
    │   ├── manager.go                # セッション状態管理
    │   ├── manager_test.go           # manager.goのテスト
    │   └── types.go                  # sessionパッケージ型定義
    └── store/
        ├── client.go                 # RADIUSクライアントデータアクセス
        ├── client_test.go            # client.goのテスト
        ├── convert.go                # Valkey Hash ↔ struct変換
        ├── convert_test.go           # convert.goのテスト
        ├── duplicate.go              # 重複検出用Valkeyアクセス
        ├── duplicate_test.go         # duplicate.goのテスト
        ├── errors.go                 # storeパッケージエラー定義
        ├── interfaces.go             # storeパッケージインターフェース定義
        ├── keys.go                   # Valkeyキー生成ヘルパー
        ├── session.go                # セッションデータアクセス
        ├── session_test.go           # session.goのテスト
        └── valkey.go                 # Valkeyクライアント初期化
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
    │  │              ▼                                  │  │
    │  │         radius/                                 │  │
    │  │              │                                  │  │
    │  │              ▼                                  │  │
    │  │          acct/                                  │  │
    │  │              │                                  │  │
    │  │              ▼                                  │  │
    │  │        session/                                 │  │
    │  │              │                                  │  │
    │  │              ▼                                  │  │
    │  │         store/                                  │  │
    │  │              │                                  │  │
    │  │              ▼                                  │  │
    │  │        logging/                                 │  │
    │  └─────────────────────────────────────────────────┘  │
    └───────────────────────────────────────────────────────┘
```

### 2.3 パッケージ責務一覧

| パッケージ | 責務 | 主要な型・関数 |
|-----------|------|---------------|
| `config` | 環境変数読み込み、設定値管理、定数定義 | `Config`, `Load()` |
| `server` | PacketServer管理、SecretSource実装、radius.Handler実装 | `Server`, `SecretSource`, `Handler` |
| `radius` | RADIUSパケット処理（属性抽出、応答生成、Proxy-State、Message-Authenticator、Status-Server） | `ResponseBuilder`, `AttributeExtractor`, `StatusHandler` |
| `acct` | Accounting処理ロジック（Start/Interim/Stop）、インターフェース定義 | `Processor`, `DuplicateDetector` |
| `session` | セッション状態管理、IMSI取得、インターフェース定義 | `Manager`, `IdentifierResolver` |
| `store` | Valkeyアクセス抽象化（クライアント、セッション、重複検出、変換） | `ValkeyClient`, `ClientStore`, `SessionStore` |
| `logging` | IMSIマスキング処理 | `MaskIMSI()` |
| `mocks` | テスト用モック（acct、session、store） | 各パッケージのモック実装 |

### 2.4 外部パッケージ依存

D-01で定義されたパッケージ利用マップに基づく。

| カテゴリ | パッケージ | 用途 | 利用箇所 |
|---------|-----------|------|---------|
| **RADIUS** | `layeh.com/radius` | RADIUSプロトコル処理、PacketServer | `server/`, `radius/` |
| **DB** | `github.com/redis/go-redis/v9` | Valkeyクライアント | `store/` |
| **Config** | `github.com/kelseyhightower/envconfig` | 環境変数読み込み | `config/` |
| **UUID** | `github.com/google/uuid` | Class属性パース検証 | `session/` |
| **Logging** | `log/slog` (標準ライブラリ) | 構造化ログ | 全パッケージ |

### 2.5 Dockerfile方針

#### 2.5.1 マルチステージビルド構成

```dockerfile
# ビルドステージ
FROM golang:1.25-bookworm AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o acct-server .

# ランタイムステージ
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    procps \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/acct-server /usr/local/bin/acct-server

EXPOSE 1813/udp
# UDPサービスのためHTTPヘルスチェックなし（pgrepで代替）

ENTRYPOINT ["/usr/local/bin/acct-server"]
```

#### 2.5.2 ベースイメージ選定

| ステージ   | イメージ               | 理由                                                 |
| ---------- | ---------------------- | ---------------------------------------------------- |
| ビルド     | `golang:1.25-bookworm` | Go 1.25.x、Debian Bookwormベース                     |
| ランタイム | `debian:bookworm-slim` | 最小構成、将来のHTTPヘルスエンドポイント追加に備える |

#### 2.5.3 必須パッケージ

| パッケージ        | 用途                                 |
| ----------------- | ------------------------------------ |
| `ca-certificates` | TLS証明書（将来のHTTPS通信に備える） |
| `curl`            | 将来のHTTPヘルスチェック対応に備える |
| `procps`          | ヘルスチェック用（`pgrep`コマンド提供）|

> **注記:** Acct ServerはUDPサービスのため、現時点ではプロセス存在確認（`pgrep`）でヘルスチェックを行う。`pgrep` は `procps` パッケージに含まれる。将来的にHTTPヘルスエンドポイントを追加する場合は `curl -fsS` を使用する。

---

## ■セクション3: 環境変数・設定

### 3.1 環境変数一覧

| 環境変数 | 必須 | デフォルト | 説明 |
|---------|------|-----------|------|
| `REDIS_HOST` | Yes | - | Valkeyホスト名 |
| `REDIS_PORT` | Yes | - | Valkeyポート番号 |
| `REDIS_PASS` | Yes | - | Valkeyパスワード |
| `RADIUS_SECRET` | No | - | デフォルトShared Secret（フォールバック用） |
| `LOG_MASK_IMSI` | No | `true` | IMSIマスキング有効化 |
> **注記:** 環境変数名 `RADIUS_SECRET` はシステム全体で統一されている。D-01およびD-08の `.env` ファイルでも同名を使用すること。

### 3.2 設定構造体

```go
// internal/config/config.go
package config

import "github.com/kelseyhightower/envconfig"

type Config struct {
    // Valkey接続設定
    RedisHost string `envconfig:"REDIS_HOST" required:"true"`
    RedisPort string `envconfig:"REDIS_PORT" required:"true"`
    RedisPass string `envconfig:"REDIS_PASS" required:"true"`
    
    // RADIUS設定
    RadiusSecret string `envconfig:"RADIUS_SECRET" default:""`
    
    // ログ設定
    LogMaskIMSI bool `envconfig:"LOG_MASK_IMSI" default:"true"`
}

func Load() (*Config, error) {
    var cfg Config
    if err := envconfig.Process("", &cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}
```

### 3.3 接続タイムアウト設定

D-01で定義された値を使用する。

| 項目 | 値 | 備考 |
|------|-----|------|
| Valkey接続タイムアウト | 3秒 | `DialTimeout` |
| Valkeyコマンドタイムアウト | 2秒 | `ReadTimeout`, `WriteTimeout` |

---

## ■セクション4: RADIUS Accounting処理フロー

### 4.1 全体処理フロー

```
[NAS/AP] ──UDP 1813──> [Acct Server]
                            │
                            ▼
                    ┌───────────────┐
                    │ パケット受信  │
                    └───────┬───────┘
                            │
                            ▼
                    ┌───────────────┐
                    │ Secret解決    │ ← client:{IP} or 環境変数
                    └───────┬───────┘
                            │
                            ▼
                    ┌───────────────┐
                    │ Authenticator │
                    │ 検証          │
                    └───────┬───────┘
                            │NG → パケット破棄（応答なし）
                            │OK
                            ▼
                    ┌───────────────┐
                    │ 属性抽出      │
                    │ - Acct-Status-Type
                    │ - Acct-Session-Id
                    │ - Class (UUID)
                    │ - User-Name
                    │ - Proxy-State
                    └───────┬───────┘
                            │
                            ▼
                    ┌───────────────┐
                    │ Status-Type   │
                    │ 振り分け      │
                    └───────┬───────┘
                            │
            ┌───────────────┼───────────────┐
            │               │               │
            ▼               ▼               ▼
    ┌───────────┐   ┌───────────┐   ┌───────────┐
    │ Start(1)  │   │ Stop(2)   │   │ Interim(3)│
    └─────┬─────┘   └─────┬─────┘   └─────┬─────┘
          │               │               │
          ▼               ▼               ▼
    ┌─────────────────────────────────────────┐
    │        セッション状態更新               │
    │        ログ出力                         │
    └───────────────────┬─────────────────────┘
                        │
                        ▼
                ┌───────────────┐
                │ Accounting-   │
                │ Response生成  │ ← Proxy-Stateエコーバック
                └───────┬───────┘
                        │
                        ▼
                    [応答送信]
```

### 4.2 Shared Secret解決

Auth Serverと同一のロジックを使用する。

```go
// internal/server/secret.go
func (s *SecretSource) RADIUSSecret(ctx context.Context, remoteAddr net.Addr, raw []byte) ([]byte, error) {
    ip := extractIP(remoteAddr)
    
    // 1. Valkey client:{IP} を検索
    secret, err := s.clientStore.GetClientSecret(ctx, ip)
    if err == nil && secret != "" {
        return []byte(secret), nil
    }
    
    // 2. 環境変数フォールバック
    if s.defaultSecret != "" {
        return []byte(s.defaultSecret), nil
    }
    
    // 3. Secret不明
    slog.Warn("shared secret not found",
        "event_id", "RADIUS_NO_SECRET",
        "src_ip", ip)
    return nil, ErrSecretNotFound
}
```

### 4.3 Request Authenticator検証

Accounting-Requestの検証はRFC 2866に基づく。

**検証式:**
```
Authenticator = MD5(Code + Identifier + Length + 16 zero octets + Request Attributes + Secret)
```

```go
// internal/radius/authenticator.go
func VerifyAccountingAuthenticator(packet *radius.Packet, secret []byte) bool {
    // パケットのAuthenticatorフィールドを検証
    expected := calculateAccountingAuthenticator(packet, secret)
    return hmac.Equal(packet.Authenticator[:], expected)
}

func calculateAccountingAuthenticator(packet *radius.Packet, secret []byte) []byte {
    h := md5.New()
    
    // Code (1 byte)
    h.Write([]byte{byte(packet.Code)})
    
    // Identifier (1 byte)
    h.Write([]byte{packet.Identifier})
    
    // Length (2 bytes)
    length := make([]byte, 2)
    binary.BigEndian.PutUint16(length, uint16(len(packet.Encode())))
    h.Write(length)
    
    // 16 zero octets
    h.Write(make([]byte, 16))
    
    // Request Attributes
    h.Write(packet.Attributes.Encode())
    
    // Secret
    h.Write(secret)
    
    return h.Sum(nil)
}
```

### 4.4 属性抽出

```go
// internal/radius/attributes.go
package radius

import (
    "github.com/google/uuid"
    radiuspkg "layeh.com/radius"
    "layeh.com/radius/rfc2866"
)

type AccountingAttributes struct {
    AcctStatusType  rfc2866.AcctStatusType
    AcctSessionID   string
    ClassUUID       string      // パース済みUUID（空文字列の場合あり）
    UserName        string
    NasIPAddress    string
    FramedIPAddress string
    InputOctets     uint32
    OutputOctets    uint32
    SessionTime     uint32
    ProxyStates     [][]byte
}

func ExtractAccountingAttributes(packet *radiuspkg.Packet) (*AccountingAttributes, error) {
    attrs := &AccountingAttributes{}
    
    // Acct-Status-Type (必須)
    statusType := rfc2866.AcctStatusType_Get(packet)
    if statusType == 0 {
        return nil, ErrMissingStatusType
    }
    attrs.AcctStatusType = statusType
    
    // Acct-Session-Id (必須)
    attrs.AcctSessionID = rfc2866.AcctSessionID_GetString(packet)
    if attrs.AcctSessionID == "" {
        return nil, ErrMissingSessionID
    }
    
    // Class (オプション - UUID抽出試行)
    if classAttr := packet.Get(rfc2865.Class_Type); classAttr != nil {
        classValue := string(classAttr)
        // RFC 4122 UUID形式の検証
        if _, err := uuid.Parse(classValue); err == nil {
            attrs.ClassUUID = classValue
        }
    }
    
    // User-Name (オプション)
    attrs.UserName = rfc2865.UserName_GetString(packet)
    
    // NAS-IP-Address
    if nasIP := rfc2865.NASIPAddress_Get(packet); nasIP != nil {
        attrs.NasIPAddress = nasIP.String()
    }
    
    // Framed-IP-Address
    if framedIP := rfc2865.FramedIPAddress_Get(packet); framedIP != nil {
        attrs.FramedIPAddress = framedIP.String()
    }
    
    // 通信量 (Interim/Stop)
    attrs.InputOctets = rfc2866.AcctInputOctets_Get(packet)
    attrs.OutputOctets = rfc2866.AcctOutputOctets_Get(packet)
    
    // セッション時間 (Stop)
    attrs.SessionTime = rfc2866.AcctSessionTime_Get(packet)
    
    // Proxy-State (複数可)
    attrs.ProxyStates = ExtractProxyStates(packet)
    
    return attrs, nil
}
```

### 4.5 Proxy-State処理

RFC 2866に準拠し、受信したProxy-State属性をAccounting-Responseにエコーバックする。

```go
// internal/radius/proxystate.go
package radius

import radiuspkg "layeh.com/radius"

const ProxyStateType = 33 // RFC 2865

// ExtractProxyStates は全てのProxy-State属性を抽出する
func ExtractProxyStates(packet *radiuspkg.Packet) [][]byte {
    var states [][]byte
    for _, attr := range packet.Attributes {
        if attr.Type == ProxyStateType {
            states = append(states, attr.Attribute)
        }
    }
    return states
}

// ApplyProxyStates はProxy-State属性を応答パケットに設定する
func ApplyProxyStates(packet *radiuspkg.Packet, states [][]byte) {
    for _, state := range states {
        packet.Attributes.Add(ProxyStateType, state)
    }
}
```

### 4.6 Accounting-Response生成

```go
// internal/radius/response.go
package radius

import radiuspkg "layeh.com/radius"

// BuildAccountingResponse はAccounting-Responseパケットを生成する
func BuildAccountingResponse(request *radiuspkg.Packet, secret []byte, proxyStates [][]byte) *radiuspkg.Packet {
    response := request.Response(radiuspkg.CodeAccountingResponse)
    
    // Proxy-Stateエコーバック
    ApplyProxyStates(response, proxyStates)
    
    // Response Authenticator計算・設定
    // MD5(Code+ID+Length+RequestAuth+Attributes+Secret)
    response.Authenticator = calculateResponseAuthenticator(response, request.Authenticator, secret)
    
    return response
}

func calculateResponseAuthenticator(response *radiuspkg.Packet, requestAuth [16]byte, secret []byte) [16]byte {
    h := md5.New()
    
    // Code
    h.Write([]byte{byte(response.Code)})
    
    // Identifier
    h.Write([]byte{response.Identifier})
    
    // Length
    encoded := response.Encode()
    length := make([]byte, 2)
    binary.BigEndian.PutUint16(length, uint16(len(encoded)))
    h.Write(length)
    
    // Request Authenticator
    h.Write(requestAuth[:])
    
    // Response Attributes
    h.Write(response.Attributes.Encode())
    
    // Secret
    h.Write(secret)
    
    var auth [16]byte
    copy(auth[:], h.Sum(nil))
    return auth
}
```

### 4.7 Status-Server対応

**ファイル:** `internal/radius/status.go`

**責務:** RFC 5997 Status-Server 応答

#### 4.7.1 処理フロー

1. Status-Server (Code=12) 受信
2. Message-Authenticator 検証
3. Accounting-Response (Code=5) 応答

#### 4.7.2 実装

```go
// internal/radius/status.go
package radius

import (
    "crypto/hmac"
    "crypto/md5"
    "log/slog"
    
    radiuspkg "layeh.com/radius"
    "layeh.com/radius/rfc2869"
)

// HandleStatusServer はStatus-Server (Code=12) を処理する
func HandleStatusServer(request *radiuspkg.Packet, secret []byte, srcIP string) *radiuspkg.Packet {
    // 1. Message-Authenticator検証
    if !VerifyMessageAuthenticator(request, secret) {
        slog.Warn("message authenticator verification failed",
            "event_id", "RADIUS_AUTH_ERR",
            "src_ip", srcIP)
        return nil
    }
    
    // 2. Accounting-Response生成
    response := request.Response(radiuspkg.CodeAccountingResponse)
    
    // 3. Proxy-Stateエコーバック
    proxyStates := ExtractProxyStates(request)
    ApplyProxyStates(response, proxyStates)
    
    // 4. Message-Authenticator生成・追加
    AddMessageAuthenticator(response, secret)
    
    // 5. Response Authenticator計算
    response.Authenticator = calculateResponseAuthenticator(response, request.Authenticator, secret)
    
    slog.Info("status-server response",
        "event_id", "PKT_RECV",
        "src_ip", srcIP,
        "packet_code", "Status-Server")
    
    return response
}

// internal/radius/message_authenticator.go

// VerifyMessageAuthenticator はMessage-Authenticator属性を検証する
func VerifyMessageAuthenticator(packet *radiuspkg.Packet, secret []byte) bool {
    msgAuth := rfc2869.MessageAuthenticator_Get(packet)
    if msgAuth == nil {
        return false
    }
    
    // 検証用にMessage-Authenticatorを16個のゼロバイトに置き換え
    originalAuth := make([]byte, 16)
    copy(originalAuth, msgAuth)
    rfc2869.MessageAuthenticator_Set(packet, make([]byte, 16))
    
    // HMAC-MD5計算
    h := hmac.New(md5.New, secret)
    h.Write(packet.Encode())
    expected := h.Sum(nil)
    
    // 元に戻す
    rfc2869.MessageAuthenticator_Set(packet, originalAuth)
    
    return hmac.Equal(msgAuth, expected)
}

// AddMessageAuthenticator は応答にMessage-Authenticator属性を追加する
func AddMessageAuthenticator(packet *radiuspkg.Packet, secret []byte) {
    // プレースホルダーとして16個のゼロバイトを設定
    rfc2869.MessageAuthenticator_Set(packet, make([]byte, 16))
    
    // HMAC-MD5計算
    h := hmac.New(md5.New, secret)
    h.Write(packet.Encode())
    
    // 計算結果で上書き
    rfc2869.MessageAuthenticator_Set(packet, h.Sum(nil))
}
```

#### 4.7.3 設計方針

| 項目 | 方針 | 理由 |
|------|------|------|
| 応答コード | Accounting-Response (Code=5) | RFC 5997: 課金ポートでの応答 |
| Valkey死活確認 | 行わない | シンプルな応答、Auth Serverと統一 |
| Message-Authenticator | 検証必須 | RFC 5997推奨、Auth Serverと統一 |

#### 4.7.4 注意点

- Status-Server は課金処理ではなく死活監視用
- ログ出力は INFO レベル（`PKT_RECV`）
- Valkey の状態確認は行わない（シンプルな応答）

---

## ■セクション5: セッション状態管理

### 5.1 セッション状態モデル

Acct ServerはValkeyのセッションデータ（`sess:{UUID}`）を管理する。

```
                    ┌─────────────┐
                    │  (不在)     │
                    └──────┬──────┘
                           │ Acct-Start
                           │ (Auth Server が sess:{UUID} 作成済み)
                           ▼
                    ┌─────────────┐
        Interim ───>│   ACTIVE    │<─── Interim
       (TTL延長)    └──────┬──────┘    (TTL延長)
                           │ Acct-Stop
                           │ (sess:{UUID} 削除)
                           ▼
                    ┌─────────────┐
                    │  (削除済み) │
                    └─────────────┘
```

### 5.2 Valkeyキー構造

| キー | 用途 | TTL |
|------|------|-----|
| `sess:{UUID}` | アクティブセッション | 24時間（Start/Interim時にリセット） |
| `idx:user:{IMSI}` | ユーザー検索インデックス | なし（明示的削除） |
| `acct:seen:{Acct-Session-Id}` | 重複検出用 | 86400秒（24時間） |

### 5.3 Acct-Start処理

```go
// internal/acct/start.go
func (p *Processor) ProcessStart(ctx context.Context, attrs *AccountingAttributes, srcIP string) error {
    // 1. 重複検出
    isDuplicate, err := p.duplicateDetector.CheckAndMark(ctx, attrs.AcctSessionID, StatusStart)
    if err != nil {
        return err
    }
    if isDuplicate {
        slog.Warn("duplicate accounting start",
            "event_id", "ACCT_DUPLICATE_START",
            "src_ip", srcIP,
            "acct_session_id", attrs.AcctSessionID)
        return nil // 既存セッション維持、応答は返す
    }
    
    // 2. Class属性からセッションUUID取得
    sessionUUID := attrs.ClassUUID
    if sessionUUID == "" {
        slog.Warn("class attribute missing or invalid",
            "event_id", "ACCT_SESSION_NOT_FOUND",
            "src_ip", srcIP,
            "acct_session_id", attrs.AcctSessionID)
        // セッション不在でも処理継続
    }
    
    // 3. セッション存在確認・更新
    if sessionUUID != "" {
        exists, err := p.sessionManager.Exists(ctx, sessionUUID)
        if err != nil {
            slog.Error("valkey error",
                "event_id", "VALKEY_CONN_ERR",
                "error", err.Error())
            // Valkey障害時も処理継続
        } else if !exists {
            slog.Warn("session not found",
                "event_id", "ACCT_SESSION_NOT_FOUND",
                "src_ip", srcIP,
                "class_uuid", sessionUUID)
        } else {
            // セッション更新
            err = p.sessionManager.UpdateOnStart(ctx, sessionUUID, &SessionStartData{
                StartTime:  time.Now().Unix(),
                NasIP:      srcIP,
                AcctID:     attrs.AcctSessionID,
                ClientIP:   attrs.FramedIPAddress,
            })
            if err != nil {
                slog.Error("session update failed",
                    "event_id", "DB_WRITE_ERR",
                    "error", err.Error())
            }
        }
    }
    
    // 4. ログ出力
    imsi := p.resolveIMSI(ctx, sessionUUID, attrs)
    slog.Info("accounting start",
        "event_id", "ACCT_START",
        "src_ip", srcIP,
        "imsi", p.maskIMSI(imsi),
        "acct_session_id", attrs.AcctSessionID)
    
    return nil
}
```

### 5.4 Acct-Interim処理

```go
// internal/acct/interim.go
func (p *Processor) ProcessInterim(ctx context.Context, attrs *AccountingAttributes, srcIP string) error {
    // 1. 重複検出（Interim重複もDUPLICATE_STARTとしてログ）
    isDuplicate, err := p.duplicateDetector.CheckInterimDuplicate(ctx, attrs.AcctSessionID, attrs.InputOctets, attrs.OutputOctets)
    if err != nil {
        return err
    }
    if isDuplicate {
        slog.Warn("duplicate accounting interim",
            "event_id", "ACCT_DUPLICATE_START",
            "src_ip", srcIP,
            "acct_session_id", attrs.AcctSessionID)
        return nil
    }
    
    // 2. Startなしチェック
    seenStart, err := p.duplicateDetector.HasSeenStart(ctx, attrs.AcctSessionID)
    if err != nil {
        // Valkey障害時は処理継続
        slog.Error("valkey error",
            "event_id", "VALKEY_CONN_ERR",
            "error", err.Error())
    } else if !seenStart {
        slog.Warn("interim without start",
            "event_id", "ACCT_SEQUENCE_ERR",
            "src_ip", srcIP,
            "acct_session_id", attrs.AcctSessionID,
            "reason", "no_start_received")
        // セッション新規作成（Start相当の処理）
        p.duplicateDetector.MarkAsStart(ctx, attrs.AcctSessionID)
    }
    
    // 3. セッション更新
    sessionUUID := attrs.ClassUUID
    if sessionUUID != "" {
        err = p.sessionManager.UpdateOnInterim(ctx, sessionUUID, &SessionInterimData{
            NasIP:        srcIP,
            ClientIP:     attrs.FramedIPAddress,
            InputOctets:  int64(attrs.InputOctets),
            OutputOctets: int64(attrs.OutputOctets),
        })
        if err != nil {
            slog.Error("session update failed",
                "event_id", "DB_WRITE_ERR",
                "error", err.Error())
        }
    }
    
    // 4. ログ出力
    imsi := p.resolveIMSI(ctx, sessionUUID, attrs)
    slog.Info("accounting interim",
        "event_id", "ACCT_INTERIM",
        "src_ip", srcIP,
        "imsi", p.maskIMSI(imsi),
        "acct_session_id", attrs.AcctSessionID,
        "input_octets", attrs.InputOctets,
        "output_octets", attrs.OutputOctets)
    
    return nil
}
```

### 5.5 Acct-Stop処理

```go
// internal/acct/stop.go
func (p *Processor) ProcessStop(ctx context.Context, attrs *AccountingAttributes, srcIP string) error {
    // 1. Stop重複チェック（重複の場合はログなしで処理継続）
    isDuplicate, err := p.duplicateDetector.CheckStopDuplicate(ctx, attrs.AcctSessionID)
    if err != nil {
        // Valkey障害時は処理継続
    }
    if isDuplicate {
        // Stop重複時はエラーログ出力なし
        return nil
    }
    
    // 2. Stop後Start チェック用にマーク
    p.duplicateDetector.MarkAsStopped(ctx, attrs.AcctSessionID)
    
    // 3. セッション削除
    sessionUUID := attrs.ClassUUID
    if sessionUUID != "" {
        // IMSI取得（削除前に）
        session, err := p.sessionManager.Get(ctx, sessionUUID)
        var imsi string
        if err == nil && session != nil {
            imsi = session.IMSI
        }
        
        // セッション削除
        err = p.sessionManager.Delete(ctx, sessionUUID)
        if err != nil {
            slog.Error("session delete failed",
                "event_id", "DB_WRITE_ERR",
                "error", err.Error())
        }
        
        // インデックス削除
        if imsi != "" {
            err = p.sessionManager.RemoveUserIndex(ctx, imsi, sessionUUID)
            if err != nil {
                slog.Error("index delete failed",
                    "event_id", "DB_WRITE_ERR",
                    "error", err.Error())
            }
        }
    }
    
    // 4. ログ出力
    imsi := p.resolveIMSI(ctx, sessionUUID, attrs)
    slog.Info("accounting stop",
        "event_id", "ACCT_STOP",
        "src_ip", srcIP,
        "imsi", p.maskIMSI(imsi),
        "acct_session_id", attrs.AcctSessionID,
        "input_octets", attrs.InputOctets,
        "output_octets", attrs.OutputOctets,
        "session_time", attrs.SessionTime)
    
    return nil
}
```

### 5.6 重複・順序異常検出

Acct-Session-Idをキーとして、重複および順序異常を検出する。

```go
// internal/acct/duplicate.go
package acct

import (
    "context"
    "fmt"
    "time"
    
    "github.com/redis/go-redis/v9"
)

type StatusType int

const (
    StatusStart StatusType = iota
    StatusInterim
    StatusStop
)

const (
    seenKeyPrefix = "acct:seen:"
    seenTTL       = 24 * time.Hour
)

type DuplicateDetector struct {
    rdb *redis.Client
}

// seenキー: acct:seen:{Acct-Session-Id}
// 値: "start", "interim:{input}:{output}", "stop"

func (d *DuplicateDetector) seenKey(acctSessionID string) string {
    return seenKeyPrefix + acctSessionID
}

// CheckAndMark はStartの重複をチェックし、未登録ならマークする
func (d *DuplicateDetector) CheckAndMark(ctx context.Context, acctSessionID string, status StatusType) (bool, error) {
    key := d.seenKey(acctSessionID)
    
    // 既存値を取得
    val, err := d.rdb.Get(ctx, key).Result()
    if err == redis.Nil {
        // 新規：マークして継続
        if status == StatusStart {
            d.rdb.Set(ctx, key, "start", seenTTL)
        }
        return false, nil
    }
    if err != nil {
        return false, err
    }
    
    // Stop後のStart検出
    if status == StatusStart && val == "stop" {
        // 順序異常（Stop→Start）：新規セッションとして扱う
        d.rdb.Set(ctx, key, "start", seenTTL)
        return false, &SequenceError{Reason: "start_after_stop"}
    }
    
    // Start重複
    if status == StatusStart && (val == "start" || hasPrefix(val, "interim:")) {
        return true, nil
    }
    
    return false, nil
}

// HasSeenStart はStartを受信済みかチェック
func (d *DuplicateDetector) HasSeenStart(ctx context.Context, acctSessionID string) (bool, error) {
    key := d.seenKey(acctSessionID)
    val, err := d.rdb.Get(ctx, key).Result()
    if err == redis.Nil {
        return false, nil
    }
    if err != nil {
        return false, err
    }
    return val == "start" || hasPrefix(val, "interim:"), nil
}

// MarkAsStart はStartとしてマーク（StartなしInterim受信時に使用）
func (d *DuplicateDetector) MarkAsStart(ctx context.Context, acctSessionID string) error {
    key := d.seenKey(acctSessionID)
    return d.rdb.Set(ctx, key, "start", seenTTL).Err()
}

// CheckInterimDuplicate はInterimの重複をチェック
// 同一のinput/output値の場合は重複とみなす
func (d *DuplicateDetector) CheckInterimDuplicate(ctx context.Context, acctSessionID string, input, output uint32) (bool, error) {
    key := d.seenKey(acctSessionID)
    currentVal := fmt.Sprintf("interim:%d:%d", input, output)
    
    val, err := d.rdb.Get(ctx, key).Result()
    if err != nil && err != redis.Nil {
        return false, err
    }
    
    if val == currentVal {
        return true, nil
    }
    
    // 値を更新
    d.rdb.Set(ctx, key, currentVal, seenTTL)
    return false, nil
}

// CheckStopDuplicate はStopの重複をチェック
func (d *DuplicateDetector) CheckStopDuplicate(ctx context.Context, acctSessionID string) (bool, error) {
    key := d.seenKey(acctSessionID)
    val, err := d.rdb.Get(ctx, key).Result()
    if err == redis.Nil {
        return false, nil
    }
    if err != nil {
        return false, err
    }
    return val == "stop", nil
}

// MarkAsStopped はStopとしてマーク
func (d *DuplicateDetector) MarkAsStopped(ctx context.Context, acctSessionID string) error {
    key := d.seenKey(acctSessionID)
    return d.rdb.Set(ctx, key, "stop", seenTTL).Err()
}

type SequenceError struct {
    Reason string
}

func (e *SequenceError) Error() string {
    return "sequence error: " + e.Reason
}
```

### 5.7 セッション状態操作

```go
// internal/session/manager.go
package session

import (
    "context"
    "time"
    
    "github.com/redis/go-redis/v9"
)

const (
    sessionKeyPrefix = "sess:"
    indexKeyPrefix   = "idx:user:"
    sessionTTL       = 24 * time.Hour
)

type Manager struct {
    rdb *redis.Client
}

func (m *Manager) sessionKey(uuid string) string {
    return sessionKeyPrefix + uuid
}

func (m *Manager) indexKey(imsi string) string {
    return indexKeyPrefix + imsi
}

// Exists はセッションの存在を確認
func (m *Manager) Exists(ctx context.Context, uuid string) (bool, error) {
    n, err := m.rdb.Exists(ctx, m.sessionKey(uuid)).Result()
    return n > 0, err
}

// Get はセッション情報を取得
func (m *Manager) Get(ctx context.Context, uuid string) (*Session, error) {
    key := m.sessionKey(uuid)
    result, err := m.rdb.HGetAll(ctx, key).Result()
    if err != nil {
        return nil, err
    }
    if len(result) == 0 {
        return nil, ErrSessionNotFound
    }
    return parseSession(result), nil
}

// UpdateOnStart はStart受信時のセッション更新
func (m *Manager) UpdateOnStart(ctx context.Context, uuid string, data *SessionStartData) error {
    key := m.sessionKey(uuid)
    pipe := m.rdb.Pipeline()
    
    pipe.HSet(ctx, key,
        "start_time", data.StartTime,
        "nas_ip", data.NasIP,
        "acct_id", data.AcctID,
    )
    if data.ClientIP != "" {
        pipe.HSet(ctx, key, "client_ip", data.ClientIP)
    }
    
    // TTL 24時間にリセット
    pipe.Expire(ctx, key, sessionTTL)
    
    _, err := pipe.Exec(ctx)
    return err
}

// UpdateOnInterim はInterim受信時のセッション更新
func (m *Manager) UpdateOnInterim(ctx context.Context, uuid string, data *SessionInterimData) error {
    key := m.sessionKey(uuid)
    pipe := m.rdb.Pipeline()
    
    pipe.HSet(ctx, key,
        "nas_ip", data.NasIP,
        "input_octets", data.InputOctets,
        "output_octets", data.OutputOctets,
    )
    if data.ClientIP != "" {
        pipe.HSet(ctx, key, "client_ip", data.ClientIP)
    }
    
    // TTL 24時間にリセット
    pipe.Expire(ctx, key, sessionTTL)
    
    _, err := pipe.Exec(ctx)
    return err
}

// Delete はセッションを削除
func (m *Manager) Delete(ctx context.Context, uuid string) error {
    return m.rdb.Del(ctx, m.sessionKey(uuid)).Err()
}

// RemoveUserIndex はユーザーインデックスからセッションを削除
func (m *Manager) RemoveUserIndex(ctx context.Context, imsi, uuid string) error {
    return m.rdb.SRem(ctx, m.indexKey(imsi), uuid).Err()
}
```

---

## ■セクション6: ログ出力用IMSI取得

### 6.1 IMSI取得優先順位

セッション不在時のログ出力用IMSIは、以下の優先順位で取得を試みる。

| 優先度 | 条件 | 出力値 |
|--------|------|--------|
| 1 | セッション（`sess:{UUID}`）からIMSI取得成功 | マスク済みIMSI |
| 2 | User-NameからIMSI抽出成功 | マスク済みIMSI |
| 3 | IMSI抽出失敗、User-Name取得成功 | User-Nameの値（そのまま） |
| 4 | User-Name取得失敗、Class取得成功 | ClassのUUID |
| 5 | Class取得失敗 | `"unknown"` |

### 6.2 IMSI抽出ロジック

```go
// internal/session/identifier.go
package session

import (
    "regexp"
    "strings"
)

var imsiPattern = regexp.MustCompile(`^[0-9]{15}$`)

// IdentifierResolver はログ出力用の識別子を解決する
type IdentifierResolver struct {
    sessionManager *Manager
    maskEnabled    bool
}

// ResolveIMSI はログ出力用のIMSI/識別子を取得する
func (r *IdentifierResolver) ResolveIMSI(ctx context.Context, sessionUUID string, attrs *AccountingAttributes) string {
    // 1. セッションからIMSI取得
    if sessionUUID != "" {
        session, err := r.sessionManager.Get(ctx, sessionUUID)
        if err == nil && session != nil && session.IMSI != "" {
            return r.mask(session.IMSI)
        }
    }
    
    // 2. User-NameからIMSI抽出
    if attrs.UserName != "" {
        imsi := extractIMSIFromIdentity(attrs.UserName)
        if imsi != "" {
            return r.mask(imsi)
        }
        // 3. IMSI抽出失敗、User-Nameをそのまま返却
        return attrs.UserName
    }
    
    // 4. Class UUID
    if attrs.ClassUUID != "" {
        return attrs.ClassUUID
    }
    
    // 5. 取得失敗
    return "unknown"
}

// extractIMSIFromIdentity はEAP Identity形式からIMSIを抽出する
// 形式: "0<IMSI>@<realm>" または "6<IMSI>@<realm>"
func extractIMSIFromIdentity(identity string) string {
    // @ でrealm部分を除去
    atIndex := strings.Index(identity, "@")
    if atIndex > 0 {
        identity = identity[:atIndex]
    }
    
    // 先頭文字が0または6の場合、IMSI部分を抽出
    if len(identity) >= 16 {
        prefix := identity[0]
        if prefix == '0' || prefix == '6' {
            candidate := identity[1:]
            if len(candidate) == 15 && imsiPattern.MatchString(candidate) {
                return candidate
            }
        }
    }
    
    // 直接15桁の数字列の場合
    if imsiPattern.MatchString(identity) {
        return identity
    }
    
    return ""
}

func (r *IdentifierResolver) mask(imsi string) string {
    if !r.maskEnabled {
        return imsi
    }
    return MaskIMSI(imsi)
}
```

### 6.3 IMSIマスキング処理

D-04で定義されたマスキング仕様を実装する。

```go
// internal/logging/mask.go
package logging

// MaskIMSI はIMSIをマスクする
// 入力: 440101234567890
// 出力: 440101********0
func MaskIMSI(imsi string) string {
    if len(imsi) <= 6 {
        return imsi
    }
    return imsi[:6] + "********" + imsi[len(imsi)-1:]
}
```

---

## ■セクション7: エラーハンドリング

### 7.1 エラー種別と対処

D-06で定義されたエラーハンドリングに基づく。

#### 7.1.1 通信エラー

| エラー種別 | 検出条件 | 対処 | RADIUS応答 | ログ |
|-----------|---------|------|-----------|------|
| Valkey接続失敗 | TCP接続エラー | リトライ（3回） | Accounting-Response | ERROR: `VALKEY_CONN_ERR` |
| Valkeyコマンドタイムアウト | 応答なし（2秒超過） | リトライ | Accounting-Response | ERROR: `VALKEY_CONN_ERR` |

**重要:** Valkey障害時も課金パケットにはAccounting-Responseを返す（クライアントの再送を防ぐため）。データ欠損はログから追跡可能とする。

#### 7.1.2 プロトコルエラー

| エラー種別 | 検出条件 | 対処 | RADIUS応答 | ログ |
|-----------|---------|------|-----------|------|
| パケットパース失敗 | 不正なRADIUS形式 | パケット破棄 | なし | WARN: `RADIUS_PARSE_ERR` |
| Authenticator検証失敗 | 計算値不一致 | パケット破棄 | なし | WARN: `RADIUS_AUTH_ERR` |
| Message-Authenticator検証失敗 | Status-ServerのMAC検証失敗 | パケット破棄 | なし | WARN: `RADIUS_AUTH_ERR` |
| Shared Secret不明 | client:{IP}不在かつ環境変数未設定 | パケット破棄 | なし | WARN: `RADIUS_NO_SECRET` |
| 未知のAcct-Status-Type | 1,2,3以外（7,8含む） | パケット破棄 | なし | WARN: `RADIUS_UNKNOWN_CODE` |

#### 7.1.3 データエラー

| エラー種別 | 検出条件 | 対処 | RADIUS応答 | ログ |
|-----------|---------|------|-----------|------|
| セッション不在 | sess:{UUID}不在 | 処理継続 | Accounting-Response | WARN: `ACCT_SESSION_NOT_FOUND` |
| Start重複 | 同一Acct-Session-Idで再Start | 既存維持 | Accounting-Response | WARN: `ACCT_DUPLICATE_START` |
| Interim重複 | 同一Acct-Session-Idで同一値Interim | 既存維持 | Accounting-Response | WARN: `ACCT_DUPLICATE_START` |
| StartなしでInterim | Acct-Session-Id未登録でInterim | 新規作成 | Accounting-Response | WARN: `ACCT_SEQUENCE_ERR` |
| Stop後にStart | 同一Acct-Session-Idで再Start | 新規作成 | Accounting-Response | WARN: `ACCT_SEQUENCE_ERR` |
| Stop重複 | 同一Acct-Session-Idで再Stop | 処理継続 | Accounting-Response | なし |

### 7.2 リトライ戦略

Valkey操作のリトライ設定。

```go
// internal/store/valkey.go
type RetryConfig struct {
    MaxRetries     int           // 最大リトライ回数: 3
    MinRetryDelay  time.Duration // 最小待機時間: 100ms
    MaxRetryDelay  time.Duration // 最大待機時間: 1s
}

func NewValkeyClient(cfg *config.Config) *redis.Client {
    return redis.NewClient(&redis.Options{
        Addr:         cfg.RedisHost + ":" + cfg.RedisPort,
        Password:     cfg.RedisPass,
        DB:           0,
        DialTimeout:  3 * time.Second,
        ReadTimeout:  2 * time.Second,
        WriteTimeout: 2 * time.Second,
        MaxRetries:   3,
        MinRetryBackoff: 100 * time.Millisecond,
        MaxRetryBackoff: 1 * time.Second,
    })
}
```

### 7.3 エラー定義

```go
// internal/acct/errors.go
package acct

import "errors"

var (
    ErrMissingStatusType = errors.New("missing Acct-Status-Type")
    ErrMissingSessionID  = errors.New("missing Acct-Session-Id")
    ErrUnknownStatusType = errors.New("unknown Acct-Status-Type")
)

// internal/session/errors.go
package session

import "errors"

var (
    ErrSessionNotFound = errors.New("session not found")
)

// internal/server/errors.go
package server

import "errors"

var (
    ErrSecretNotFound = errors.New("shared secret not found")
)
```

---

## ■セクション8: ログ出力仕様

### 8.1 event_id一覧

D-04で定義されたevent_idを使用する。

| event_id | レベル | 説明 |
|----------|--------|------|
| `PKT_RECV` | INFO | パケット受信（Status-Server） |
| `VALKEY_CONN_ERR` | ERROR | Valkey接続失敗 |
| `VALKEY_CONN_RESTORED` | INFO | Valkey接続復旧 |
| `DB_WRITE_ERR` | ERROR | Valkey書き込み失敗 |
| `RADIUS_PARSE_ERR` | WARN | RADIUSパケットパース失敗 |
| `RADIUS_AUTH_ERR` | WARN | Authenticator検証失敗 |
| `RADIUS_NO_SECRET` | WARN | Shared Secret不明 |
| `RADIUS_UNKNOWN_CODE` | WARN | 未知のAcct-Status-Type |
| `ACCT_SESSION_NOT_FOUND` | WARN | セッション不在 |
| `ACCT_SESSION_EXPIRED` | WARN | セッションTTL超過 |
| `ACCT_DUPLICATE_START` | WARN | 重複Start/Interim |
| `ACCT_SEQUENCE_ERR` | WARN | 順序異常 |
| `ACCT_START` | INFO | Accounting-Start受信 |
| `ACCT_INTERIM` | INFO | Accounting-Interim受信 |
| `ACCT_STOP` | INFO | Accounting-Stop受信 |

### 8.2 ログ出力例

```json
// ACCT_START
{
  "time": "2026-01-20T10:00:00.123Z",
  "level": "INFO",
  "app": "acct-server",
  "event_id": "ACCT_START",
  "msg": "accounting start",
  "src_ip": "192.168.1.100",
  "imsi": "440101********0",
  "acct_session_id": "sess-abc123"
}

// ACCT_INTERIM
{
  "time": "2026-01-20T10:05:00.456Z",
  "level": "INFO",
  "app": "acct-server",
  "event_id": "ACCT_INTERIM",
  "msg": "accounting interim",
  "src_ip": "192.168.1.100",
  "imsi": "440101********0",
  "acct_session_id": "sess-abc123",
  "input_octets": 1234567,
  "output_octets": 2345678
}

// ACCT_STOP
{
  "time": "2026-01-20T10:30:00.789Z",
  "level": "INFO",
  "app": "acct-server",
  "event_id": "ACCT_STOP",
  "msg": "accounting stop",
  "src_ip": "192.168.1.100",
  "imsi": "440101********0",
  "acct_session_id": "sess-abc123",
  "input_octets": 12345678,
  "output_octets": 23456789,
  "session_time": 1800
}

// ACCT_SEQUENCE_ERR (StartなしでInterim)
{
  "time": "2026-01-20T10:00:00.123Z",
  "level": "WARN",
  "app": "acct-server",
  "event_id": "ACCT_SEQUENCE_ERR",
  "msg": "sequence error",
  "src_ip": "192.168.1.100",
  "acct_session_id": "sess-xyz789",
  "reason": "no_start_received"
}

// ACCT_DUPLICATE_START
{
  "time": "2026-01-20T10:00:01.234Z",
  "level": "WARN",
  "app": "acct-server",
  "event_id": "ACCT_DUPLICATE_START",
  "msg": "duplicate accounting start",
  "src_ip": "192.168.1.100",
  "acct_session_id": "sess-abc123"
}
```

---

## ■セクション9: Go型定義

### 9.1 セッション関連

```go
// internal/session/types.go
package session

type Session struct {
    IMSI         string `redis:"imsi"`
    StartTime    int64  `redis:"start_time"`
    NasIP        string `redis:"nas_ip"`
    ClientIP     string `redis:"client_ip"`
    AcctID       string `redis:"acct_id"`
    InputOctets  int64  `redis:"input_octets"`
    OutputOctets int64  `redis:"output_octets"`
}

type SessionStartData struct {
    StartTime int64
    NasIP     string
    AcctID    string
    ClientIP  string
}

type SessionInterimData struct {
    NasIP        string
    ClientIP     string
    InputOctets  int64
    OutputOctets int64
}
```

### 9.2 インターフェース定義

```go
// internal/session/interfaces.go
package session

import "context"

type SessionStore interface {
    Exists(ctx context.Context, uuid string) (bool, error)
    Get(ctx context.Context, uuid string) (*Session, error)
    UpdateOnStart(ctx context.Context, uuid string, data *SessionStartData) error
    UpdateOnInterim(ctx context.Context, uuid string, data *SessionInterimData) error
    Delete(ctx context.Context, uuid string) error
    RemoveUserIndex(ctx context.Context, imsi, uuid string) error
}

// internal/store/interfaces.go
package store

import "context"

type ClientStore interface {
    GetClientSecret(ctx context.Context, ip string) (string, error)
}
```

---

## ■セクション10: 実装ノート

### 10.1 メインハンドラー実装

```go
// internal/server/handler.go
package server

import (
    "context"
    "log/slog"
    "net"
    
    radiuspkg "layeh.com/radius"
    "layeh.com/radius/rfc2866"
)

type Handler struct {
    processor *acct.Processor
    secretSrc *SecretSource
}

func (h *Handler) ServeRADIUS(w radiuspkg.ResponseWriter, r *radiuspkg.Request) {
    ctx := context.Background()
    srcIP := extractIP(r.RemoteAddr)
    
    // Code判定
    switch r.Code {
    case radiuspkg.CodeAccountingRequest:
        // Accounting処理
        h.handleAccountingRequest(ctx, w, r, srcIP)
        
    case radiuspkg.CodeStatusServer:
        // Status-Server処理
        response := radius.HandleStatusServer(r.Packet, r.Secret, srcIP)
        if response != nil {
            w.Write(response)
        }
        
    default:
        slog.Warn("unknown radius code",
            "event_id", "RADIUS_UNKNOWN_CODE",
            "src_ip", srcIP,
            "code", r.Code)
        return
    }
}

func (h *Handler) handleAccountingRequest(ctx context.Context, w radiuspkg.ResponseWriter, r *radiuspkg.Request, srcIP string) {
    // 1. 属性抽出
    attrs, err := radius.ExtractAccountingAttributes(r.Packet)
    if err != nil {
        slog.Warn("attribute extraction failed",
            "event_id", "RADIUS_PARSE_ERR",
            "src_ip", srcIP,
            "reason", err.Error())
        return
    }
    
    // 2. Status-Type別処理
    var procErr error
    switch attrs.AcctStatusType {
    case rfc2866.AcctStatusType_Value_Start:
        procErr = h.processor.ProcessStart(ctx, attrs, srcIP)
    case rfc2866.AcctStatusType_Value_Stop:
        procErr = h.processor.ProcessStop(ctx, attrs, srcIP)
    case rfc2866.AcctStatusType_Value_InterimUpdate:
        procErr = h.processor.ProcessInterim(ctx, attrs, srcIP)
    default:
        // Acct-On(7), Acct-Off(8)等は未対応
        slog.Warn("unsupported acct-status-type",
            "event_id", "RADIUS_UNKNOWN_CODE",
            "src_ip", srcIP,
            "code", attrs.AcctStatusType)
        return
    }
    
    // 3. 処理エラーがあってもAccounting-Responseは返す
    if procErr != nil {
        slog.Error("processing error",
            "event_id", "SYS_ERR",
            "error", procErr.Error())
    }
    
    // 4. Accounting-Response生成・送信
    response := radius.BuildAccountingResponse(r.Packet, r.Secret, attrs.ProxyStates)
    w.Write(response)
}

func extractIP(addr net.Addr) string {
    switch a := addr.(type) {
    case *net.UDPAddr:
        return a.IP.String()
    case *net.TCPAddr:
        return a.IP.String()
    default:
        return addr.String()
    }
}
```

### 10.2 Valkey接続復旧検知

D-04で定義された復旧検知ロジックを実装する。

```go
// internal/store/valkey.go
var (
    lastConnError time.Time
    connMu        sync.Mutex
)

func executeWithConnTracking(ctx context.Context, rdb *redis.Client, fn func() error) error {
    err := fn()
    
    connMu.Lock()
    defer connMu.Unlock()
    
    if err != nil {
        if isConnectionError(err) {
            lastConnError = time.Now()
        }
        return err
    }
    
    // 接続復旧を検知
    if !lastConnError.IsZero() {
        downtime := time.Since(lastConnError)
        slog.Info("valkey connection restored",
            "event_id", "VALKEY_CONN_RESTORED",
            "downtime_ms", downtime.Milliseconds())
        lastConnError = time.Time{}
    }
    return nil
}

func isConnectionError(err error) bool {
    // ネットワークエラー、タイムアウトエラーを判定
    return errors.Is(err, context.DeadlineExceeded) ||
           errors.Is(err, redis.ErrClosed) ||
           strings.Contains(err.Error(), "connection refused") ||
           strings.Contains(err.Error(), "i/o timeout")
}
```

### 10.3 起動・シャットダウン

```go
// main.go
package main

import (
    "context"
    "log/slog"
    "os"
    "os/signal"
    "syscall"
    
    "acct-server/internal/config"
    "acct-server/internal/server"
)

func main() {
    // ログ設定
    slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    })))
    
    // 設定読み込み
    cfg, err := config.Load()
    if err != nil {
        slog.Error("failed to load config", "error", err.Error())
        os.Exit(1)
    }
    
    // サーバー初期化
    srv, err := server.New(cfg)
    if err != nil {
        slog.Error("failed to create server", "error", err.Error())
        os.Exit(1)
    }
    
    // シグナルハンドリング
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    
    go func() {
        <-sigCh
        slog.Info("shutdown signal received")
        cancel()
    }()
    
    // サーバー起動
    slog.Info("starting acct-server", "port", 1813)
    if err := srv.ListenAndServe(ctx); err != nil {
        slog.Error("server error", "error", err.Error())
        os.Exit(1)
    }
    
    slog.Info("acct-server stopped")
}
```

---

## ■セクション11: 未決事項・将来検討

| No. | 項目 | 内容 | 判断時期 |
|-----|------|------|---------|
| 1 | Acct-On/Acct-Off対応 | NAS起動/停止通知の処理 | PoC完了後 |
| 2 | Acct-Delay-Time考慮 | タイムスタンプ補正 | PoC完了後 |
| 3 | 複数Class属性対応 | 複数UUIDの処理 | PoC完了後 |
| 4 | Event-Timestamp記録 | RFC 2869属性の参照記録 | PoC完了後 |
| 5 | 大量トラフィック対応 | 並行処理の最適化 | PoC完了後 |

---

## 改訂履歴

| 版数 | 日付 | 内容 |
|------|------|------|
| r1 | 2026-01-20 | 初版作成 |
| r2 | 2026-01-21 | Status-Server対応追加: セクション1.4からPoC対象外削除、セクション1.5にRFC 5997追加、セクション2.1/2.3にstatus.go追加、セクション4.7新設（Status-Server処理フロー・実装）、セクション7.1.2にMessage-Authenticator検証失敗追加、セクション8.1にPKT_RECV追加、セクション10.1ハンドラー更新、セクション11からStatus-Server削除 |
| r3 | 2026-01-26 | インフラ基盤統一: セクション2.5新設（Dockerfile方針 - ベースイメージdebian:bookworm-slim、curl/ca-certificates導入）、環境変数RADIUS_SECRETの統一に関する注記追加 |
| r4 | 2026-01-27 | ヘルスチェック整合性修正: セクション2.5.1 Dockerfileに`procps`パッケージ追加、セクション2.5.3必須パッケージに`procps`追記（pgrep用） |
| r5 | 2026-02-18 | ディレクトリ構造全面更新、関連ドキュメント版数更新 |
