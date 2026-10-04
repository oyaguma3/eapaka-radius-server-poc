# D-10 Acct Server詳細設計書 (r9)

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
| NAS起動/停止通知 | Accounting-On/Off処理、ログ出力 |

### 1.3 関連ドキュメント

| No. | ドキュメント | 参照内容 |
|-----|-------------|---------|
| D-01 | ミニPC版設計仕様書 (r10) | システム構成、パッケージ利用マップ |
| D-02 | Valkeyデータ設計仕様書 (r13) | データ構造、キー設計、Go構造体 |
| D-03 | Vector-APIインターフェース定義書およびEAP-AKAステートマシン設計書 (r6) | 認証フロー |
| D-04 | ログ仕様設計書 (r21) | event_id定義、ログフォーマット、IMSIマスキング |
| D-06 | エラーハンドリング詳細設計書 (r8) | エラー分類、タイムアウト、リトライ戦略 |
| D-11 | Vector API詳細設計書 (r7) | AKA認証ベクター生成 |
| D-08 | インフラ設定・運用設計書 (r14) | Docker Compose設定、環境変数 |
| D-09 | Auth Server詳細設計書 (r11) | セッション作成処理（Class属性設定） |
| E-02 | コーディング規約（簡易版） (r3) | コーディング規約 |
| E-03 | 共通ライブラリ(pkg)設計書 (r4) | 共通ライブラリ（pkg） |

### 1.4 PoC対象外機能

以下の機能は本PoCでは実装対象外とする。

| 機能 | RFC | 説明 | 備考 |
|------|-----|------|------|
| Acct-Delay-Time考慮 | RFC 2866 | NASでのパケット滞留時間補正 | 精密なタイムスタンプ管理は将来検討 |
| 複数Class属性 | RFC 2865 | 複数Class属性の処理 | 単一UUIDのみ対応 |

### 1.5 準拠規格

| 規格 | 内容 | 対応範囲 |
|------|------|---------|
| RFC 2865 | RADIUS | Proxy-State処理 |
| RFC 2866 | RADIUS Accounting | Accounting-Request/Response, Acct-Status-Type (Start/Interim/Stop/On/Off) |
| RFC 5997 | Status-Server | ヘルスチェック応答 |

### 1.6 用語定義

| 用語 | 説明 |
|------|------|
| Acct-Session-Id | NASが生成するセッション識別子（RADIUS属性） |
| Acct-Status-Type | 課金イベント種別（1:Start, 2:Stop, 3:Interim-Update, 7:Accounting-On, 8:Accounting-Off） |
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
    │   ├── stop_test.go              # stop.goのテスト
    │   ├── on_off.go                 # Acct-On/Off処理
    │   └── on_off_test.go            # on_off.goのテスト
    ├── config/
    │   ├── config.go                 # 環境変数読み込み、設定構造体
    │   ├── config_test.go            # config.goのテスト
    │   └── constants.go              # 定数定義
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
| `acct` | Accounting処理ロジック（Start/Interim/Stop/On/Off）、インターフェース定義 | `Processor`, `DuplicateDetector` |
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
        ┌───────────┬───────┼───────┬───────────┐
        │           │       │       │           │
        ▼           ▼       ▼       ▼           ▼
    ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐
    │Start(1)│ │Stop(2) │ │Intr.(3)│ │ On(7)  │ │Off(8)  │
    └───┬────┘ └───┬────┘ └───┬────┘ └───┬────┘ └───┬────┘
        │          │          │          │          │
        ▼          ▼          ▼          │          │
    ┌─────────────────────────────┐      │          │
    │  セッション状態更新         │      │          │
    │  ログ出力                   │      │          │
    └──────────────┬──────────────┘      │          │
                   │          ┌──────────┘          │
                   │          │     ┌───────────────┘
                   │          ▼     ▼
                   │    ┌─────────────────┐
                   │    │ ログ出力のみ    │
                   │    │（セッション操作 │
                   │    │ なし）          │
                   │    └────────┬────────┘
                   │             │
                   ▼             ▼
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
// internal/server/secret.go（実装）
// RADIUSSecret はリモートアドレスに対応するRADIUS Secretを返す。
func (s *DynamicSecretSource) RADIUSSecret(ctx context.Context, remoteAddr net.Addr) ([]byte, error) {
    ip := extractIP(remoteAddr)
    if ip == "" {
        if len(s.fallbackSecret) > 0 {
            return s.fallbackSecret, nil
        }
        return nil, nil
    }

    secret, err := s.clientStore.GetClientSecret(ctx, ip)
    if err != nil {
        slog.Warn("Valkeyクライアント検索エラー",
            "event_id", "RADIUS_SECRET_ERR",
            "src_ip", ip,
            "error", err,
        )
        if len(s.fallbackSecret) > 0 {
            return s.fallbackSecret, nil
        }
        return nil, nil
    }

    if secret != "" {
        return []byte(secret), nil
    }

    if len(s.fallbackSecret) > 0 {
        return s.fallbackSecret, nil
    }

    slog.Warn("RADIUS Secret不明",
        "event_id", "RADIUS_NO_SECRET",
        "src_ip", ip,
    )
    return nil, nil
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
// internal/radius/types.go
package radius

// AccountingAttributes はAccounting-Requestから抽出された属性を表す
type AccountingAttributes struct {
    AcctStatusType  uint32   // Acct-Status-Type（1:Start, 2:Stop, 3:Interim, 7:On, 8:Off）
    AcctSessionID   string   // Acct-Session-Id（Start/Stop/Interimでは必須、On/Offではオプション）
    ClassUUID       string   // Class属性からパースしたUUID（空文字列の場合あり）
    UserName        string   // User-Name（オプション）
    NasIdentifier   string   // NAS-Identifier（オプション）
    NasIPAddress    string   // NAS-IP-Address
    FramedIPAddress string   // Framed-IP-Address
    InputOctets     uint32   // Acct-Input-Octets
    OutputOctets    uint32   // Acct-Output-Octets
    SessionTime     uint32   // Acct-Session-Time
    ProxyStates     [][]byte // Proxy-State属性（複数可）
}

// Acct-Status-Type値（RFC 2866）
const (
    AcctStatusTypeStart   uint32 = 1
    AcctStatusTypeStop    uint32 = 2
    AcctStatusTypeInterim uint32 = 3
    AcctStatusTypeOn      uint32 = 7
    AcctStatusTypeOff     uint32 = 8
)
```

```go
// internal/radius/attributes.go
package radius

import (
    "encoding/binary"
    "errors"
    "net"

    "github.com/google/uuid"
    "layeh.com/radius"
)

// RADIUS属性タイプ定数（RFC 2865/2866）
const (
    AttrTypeUserName        = 1
    AttrTypeNASIPAddress    = 4
    AttrTypeFramedIPAddr    = 8
    AttrTypeClass           = 25
    AttrTypeNASIdentifier   = 32
    AttrTypeProxyState      = 33
    AttrTypeAcctStatusType  = 40
    AttrTypeAcctInputOct    = 42
    AttrTypeAcctOutputOct   = 43
    AttrTypeAcctSessionID   = 44
    AttrTypeAcctSessionTime = 46
)

var (
    ErrMissingStatusType = errors.New("missing Acct-Status-Type")
    ErrMissingSessionID  = errors.New("missing Acct-Session-Id")
)

func ExtractAccountingAttributes(packet *radius.Packet) (*AccountingAttributes, error) {
    attrs := &AccountingAttributes{}

    // Acct-Status-Type（必須）
    statusTypeAttr := packet.Get(radius.Type(AttrTypeAcctStatusType))
    if len(statusTypeAttr) < 4 {
        return nil, ErrMissingStatusType
    }
    attrs.AcctStatusType = binary.BigEndian.Uint32(statusTypeAttr)

    // Acct-Session-Id（Start/Stop/Interimでは必須、On/Offではオプション）
    sessionIDAttr := packet.Get(radius.Type(AttrTypeAcctSessionID))
    if len(sessionIDAttr) == 0 {
        if attrs.AcctStatusType != AcctStatusTypeOn && attrs.AcctStatusType != AcctStatusTypeOff {
            return nil, ErrMissingSessionID
        }
    } else {
        attrs.AcctSessionID = string(sessionIDAttr)
    }

    // Class（オプション - UUID抽出試行）
    classAttr := packet.Get(radius.Type(AttrTypeClass))
    if len(classAttr) > 0 {
        classValue := string(classAttr)
        if _, err := uuid.Parse(classValue); err == nil {
            attrs.ClassUUID = classValue
        }
    }

    // User-Name（オプション）
    userNameAttr := packet.Get(radius.Type(AttrTypeUserName))
    if len(userNameAttr) > 0 {
        attrs.UserName = string(userNameAttr)
    }

    // NAS-IP-Address
    nasIPAttr := packet.Get(radius.Type(AttrTypeNASIPAddress))
    if len(nasIPAttr) == 4 {
        attrs.NasIPAddress = net.IP(nasIPAttr).String()
    }

    // NAS-Identifier（オプション）
    nasIdentAttr := packet.Get(radius.Type(AttrTypeNASIdentifier))
    if len(nasIdentAttr) > 0 {
        attrs.NasIdentifier = string(nasIdentAttr)
    }

    // Framed-IP-Address
    framedIPAttr := packet.Get(radius.Type(AttrTypeFramedIPAddr))
    if len(framedIPAttr) == 4 {
        attrs.FramedIPAddress = net.IP(framedIPAttr).String()
    }

    // Acct-Input-Octets
    inputAttr := packet.Get(radius.Type(AttrTypeAcctInputOct))
    if len(inputAttr) >= 4 {
        attrs.InputOctets = binary.BigEndian.Uint32(inputAttr)
    }

    // Acct-Output-Octets
    outputAttr := packet.Get(radius.Type(AttrTypeAcctOutputOct))
    if len(outputAttr) >= 4 {
        attrs.OutputOctets = binary.BigEndian.Uint32(outputAttr)
    }

    // Acct-Session-Time
    timeAttr := packet.Get(radius.Type(AttrTypeAcctSessionTime))
    if len(timeAttr) >= 4 {
        attrs.SessionTime = binary.BigEndian.Uint32(timeAttr)
    }

    // Proxy-State（複数可）
    attrs.ProxyStates = extractProxyStatesRaw(packet)

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
func HandleStatusServer(request *radiuspkg.Packet, secret []byte, srcIP, traceID string) *radiuspkg.Packet {
    // 1. Message-Authenticator検証
    if !VerifyMessageAuthenticator(request, secret) {
        slog.Warn("Status-Server: Message-Authenticator検証失敗",
            "event_id", "RADIUS_AUTH_ERR",
            "trace_id", traceID,
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
    
    slog.Info("Status-Server: 応答送信",
        "event_id", "PKT_RECV",
        "trace_id", traceID,
        "src_ip", srcIP)
    
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
// internal/acct/start.go（実装）
// ProcessStart はAcct-Start処理を行う。
func (p *Processor) ProcessStart(ctx context.Context, attrs *radius.AccountingAttributes, srcIP, traceID string) error {
    // 1. 重複検出
    isDuplicate, err := p.duplicateDetector.CheckAndMarkStart(ctx, attrs.AcctSessionID)
    if err != nil {
        // SequenceError（Stop後Start）はログに出力して処理継続
        if seqErr, ok := err.(*SequenceError); ok {
            slog.Warn("sequence error",
                "event_id", "ACCT_SEQUENCE_ERR",
                "trace_id", traceID,
                "src_ip", srcIP,
                "acct_session_id", attrs.AcctSessionID,
                "reason", seqErr.Reason,
            )
        } else {
            slog.Error("duplicate check failed",
                "event_id", "VALKEY_CONN_ERR",
                "trace_id", traceID,
                "error", err.Error(),
            )
        }
    }
    if isDuplicate {
        slog.Warn("duplicate accounting start",
            "event_id", "ACCT_DUPLICATE_START",
            "trace_id", traceID,
            "src_ip", srcIP,
            "acct_session_id", attrs.AcctSessionID,
        )
        return nil
    }

    // 2. Class属性からセッションUUID取得
    sessionUUID := attrs.ClassUUID
    if sessionUUID == "" {
        slog.Warn("class attribute missing or invalid",
            "event_id", "ACCT_SESSION_NOT_FOUND",
            "trace_id", traceID,
            "src_ip", srcIP,
            "acct_session_id", attrs.AcctSessionID,
        )
    }

    // 3. セッション存在確認・更新
    if sessionUUID != "" {
        exists, err := p.sessionManager.Exists(ctx, sessionUUID)
        if err != nil {
            slog.Error("valkey error",
                "event_id", "VALKEY_CONN_ERR",
                "trace_id", traceID,
                "error", err.Error(),
            )
        } else if !exists {
            slog.Warn("session not found",
                "event_id", "ACCT_SESSION_NOT_FOUND",
                "trace_id", traceID,
                "src_ip", srcIP,
                "class_uuid", sessionUUID,
            )
        } else {
            err = p.sessionManager.UpdateOnStart(ctx, sessionUUID, &session.SessionStartData{
                StartTime: time.Now().Unix(),
                NasIP:     srcIP,
                AcctID:    attrs.AcctSessionID,
                ClientIP:  attrs.FramedIPAddress,
            })
            if err != nil {
                slog.Error("session update failed",
                    "event_id", "DB_WRITE_ERR",
                    "trace_id", traceID,
                    "error", err.Error(),
                )
            }
        }
    }

    // 4. ログ出力
    imsi := p.identifierResolver.ResolveIMSI(ctx, sessionUUID, attrs.UserName, attrs.ClassUUID)
    slog.Info("accounting start",
        "event_id", "ACCT_START",
        "trace_id", traceID,
        "src_ip", srcIP,
        "imsi", imsi,
        "acct_session_id", attrs.AcctSessionID,
    )

    return nil
}
```

### 5.4 Acct-Interim処理

Interim受信時は、まず `CheckInterim`（§5.8）で重複と順序異常を1回の判定で求める。

- 重複（直前と同一の `interim:{input}:{output}`）: WARN `ACCT_DUPLICATE_START`（msg `duplicate accounting interim`）を出力して処理を終了する。
- 順序異常（Start未受信: `no_start_received`、Stop受信後: `interim_after_stop`）: WARN `ACCT_SEQUENCE_ERR`（msg `interim sequence error`、`reason` 付き）を出力し、課金データの欠損を避けるため処理を継続する。
- 判定時のValkeyエラー（Get / Set 失敗）: ERROR `VALKEY_CONN_ERR` を出力して処理を継続する（Get 失敗時は重複・順序異常を判定できないため、正常として扱う）。

セッション更新は、Class属性のセッションUUIDに対応する `sess:{UUID}` が存在する場合だけ行う。`UpdateOnInterim` は `HSET` で書き込むため、不在のまま呼び出すと IMSI を持たない `sess:{UUID}` が新規作成されてしまう。このため、Start（§5.3）と同様に `Exists` で存在を確認し、不在なら更新せず WARN `ACCT_SESSION_NOT_FOUND`（`class_uuid` 付き）を出力する。存在確認の失敗は ERROR `VALKEY_CONN_ERR` とする。Class属性のないInterimは、セッション更新もこれらのログも行わない（Startと異なり、Class属性なしの `ACCT_SESSION_NOT_FOUND` は出力しない）。いずれの場合も最後に INFO `ACCT_INTERIM` を出力する。

```go
// internal/acct/interim.go（実装）
// ProcessInterim はAcct-Interim処理を行う。
func (p *Processor) ProcessInterim(ctx context.Context, attrs *radius.AccountingAttributes, srcIP, traceID string) error {
    // 1. 重複・順序異常の判定
    check, err := p.duplicateDetector.CheckInterim(ctx, attrs.AcctSessionID, attrs.InputOctets, attrs.OutputOctets)
    if err != nil {
        slog.Error("duplicate check failed",
            "event_id", "VALKEY_CONN_ERR",
            "trace_id", traceID,
            "error", err.Error(),
        )
    }
    if check.Duplicate {
        slog.Warn("duplicate accounting interim",
            "event_id", "ACCT_DUPLICATE_START",
            "trace_id", traceID,
            "src_ip", srcIP,
            "acct_session_id", attrs.AcctSessionID,
        )
        return nil
    }
    if check.SequenceReason != "" {
        // 順序異常でも課金データの欠損を避けるため処理を継続する
        slog.Warn("interim sequence error",
            "event_id", "ACCT_SEQUENCE_ERR",
            "trace_id", traceID,
            "src_ip", srcIP,
            "acct_session_id", attrs.AcctSessionID,
            "reason", check.SequenceReason,
        )
    }

    // 2. セッション更新（存在するセッションのみ。不在のキーを作らない）
    sessionUUID := attrs.ClassUUID
    if sessionUUID != "" {
        exists, err := p.sessionManager.Exists(ctx, sessionUUID)
        switch {
        case err != nil:
            slog.Error("valkey error",
                "event_id", "VALKEY_CONN_ERR",
                "trace_id", traceID,
                "error", err.Error(),
            )
        case !exists:
            slog.Warn("session not found",
                "event_id", "ACCT_SESSION_NOT_FOUND",
                "trace_id", traceID,
                "src_ip", srcIP,
                "class_uuid", sessionUUID,
            )
        default:
            err = p.sessionManager.UpdateOnInterim(ctx, sessionUUID, &session.SessionInterimData{
                NasIP:        srcIP,
                ClientIP:     attrs.FramedIPAddress,
                InputOctets:  int64(attrs.InputOctets),
                OutputOctets: int64(attrs.OutputOctets),
            })
            if err != nil {
                slog.Error("session update failed",
                    "event_id", "DB_WRITE_ERR",
                    "trace_id", traceID,
                    "error", err.Error(),
                )
            }
        }
    }

    // 3. ログ出力
    imsi := p.identifierResolver.ResolveIMSI(ctx, sessionUUID, attrs.UserName, attrs.ClassUUID)
    slog.Info("accounting interim",
        "event_id", "ACCT_INTERIM",
        "trace_id", traceID,
        "src_ip", srcIP,
        "imsi", imsi,
        "acct_session_id", attrs.AcctSessionID,
        "input_octets", attrs.InputOctets,
        "output_octets", attrs.OutputOctets,
    )

    return nil
}
```

### 5.5 Acct-Stop処理

```go
// internal/acct/stop.go（実装）
// ProcessStop はAcct-Stop処理を行う。
func (p *Processor) ProcessStop(ctx context.Context, attrs *radius.AccountingAttributes, srcIP, traceID string) error {
    // 1. Stop重複チェック
    isDuplicate, err := p.duplicateDetector.CheckStopDuplicate(ctx, attrs.AcctSessionID)
    if err != nil {
        // Valkey障害時は処理継続
        slog.Error("duplicate check failed",
            "event_id", "VALKEY_CONN_ERR",
            "trace_id", traceID,
            "error", err.Error(),
        )
    }
    if isDuplicate {
        // Stop重複時はログ出力なしで処理終了
        return nil
    }

    // 2. Stopとしてマーク
    if err := p.duplicateDetector.MarkAsStopped(ctx, attrs.AcctSessionID); err != nil {
        slog.Error("duplicate mark failed",
            "event_id", "DB_WRITE_ERR",
            "trace_id", traceID,
            "error", err.Error(),
        )
    }

    // 3. セッション削除
    sessionUUID := attrs.ClassUUID
    var imsiFromSession string
    if sessionUUID != "" {
        // IMSI取得（削除前に）
        sess, err := p.sessionManager.Get(ctx, sessionUUID)
        if err == nil && sess != nil {
            imsiFromSession = sess.IMSI
        }

        // セッション削除
        if err := p.sessionManager.Delete(ctx, sessionUUID); err != nil {
            slog.Error("session delete failed",
                "event_id", "DB_WRITE_ERR",
                "trace_id", traceID,
                "error", err.Error(),
            )
        }

        // インデックス削除
        if imsiFromSession != "" {
            if err := p.sessionManager.RemoveUserIndex(ctx, imsiFromSession, sessionUUID); err != nil {
                slog.Error("index delete failed",
                    "event_id", "DB_WRITE_ERR",
                    "trace_id", traceID,
                    "error", err.Error(),
                )
            }
        }
    }

    // 4. ログ出力
    imsi := p.identifierResolver.ResolveIMSI(ctx, sessionUUID, attrs.UserName, attrs.ClassUUID)
    slog.Info("accounting stop",
        "event_id", "ACCT_STOP",
        "trace_id", traceID,
        "src_ip", srcIP,
        "imsi", imsi,
        "acct_session_id", attrs.AcctSessionID,
        "input_octets", attrs.InputOctets,
        "output_octets", attrs.OutputOctets,
        "session_time", attrs.SessionTime,
    )

    return nil
}
```

### 5.6 Acct-On処理

NAS起動通知（Accounting-On, Acct-Status-Type=7）を処理する。

**設計方針:**
- ログ出力のみ（セッション操作・重複検出なし）
- 常にnil返却（エラーにならない）
- NAS-Identifier / NAS-IP-Address でNASを識別

```go
// internal/acct/on_off.go
func (p *Processor) ProcessOn(_ context.Context, attrs *radius.AccountingAttributes, srcIP, traceID string) error {
    slog.Info("accounting on",
        "event_id", "ACCT_ON",
        "trace_id", traceID,
        "src_ip", srcIP,
        "nas_ip_address", attrs.NasIPAddress,
        "nas_identifier", attrs.NasIdentifier,
    )
    return nil
}
```

### 5.7 Acct-Off処理

NASシャットダウン通知（Accounting-Off, Acct-Status-Type=8）を処理する。

**設計方針:** Acct-On処理と同一（ログ出力のみ、セッション操作なし）。

```go
// internal/acct/on_off.go
func (p *Processor) ProcessOff(_ context.Context, attrs *radius.AccountingAttributes, srcIP, traceID string) error {
    slog.Info("accounting off",
        "event_id", "ACCT_OFF",
        "trace_id", traceID,
        "src_ip", srcIP,
        "nas_ip_address", attrs.NasIPAddress,
        "nas_identifier", attrs.NasIdentifier,
    )
    return nil
}
```

### 5.8 重複・順序異常検出

Acct-Session-Idをキー（`acct:seen:{Acct-Session-Id}`、TTL 86400秒）として、重複および順序異常を検出する。値は直前に受信したパケットの種別（`start` / `interim:{input}:{output}` / `stop`）を表す。値の読み書きは `store.DuplicateStore`（`Get`: 未登録なら空文字列を返す、`Set`: TTL付きで上書き）を介して行う。

| 受信 | 直前の値 | 判定 | 記録する値 | ログ | 以降の処理 |
|------|---------|------|-----------|------|-----------|
| Start | 未登録 | 正常 | `start` | - | 継続 |
| Start | `start` / `interim:*` | 重複 | 変更しない | WARN `ACCT_DUPLICATE_START` | 終了 |
| Start | `stop` | 順序異常（`start_after_stop`） | `start` | WARN `ACCT_SEQUENCE_ERR` | 継続（新規セッションとして扱う） |
| Interim | 未登録 | 順序異常（`no_start_received`） | `interim:{input}:{output}` | WARN `ACCT_SEQUENCE_ERR` | 継続 |
| Interim | `start` | 正常 | `interim:{input}:{output}` | - | 継続 |
| Interim | 同一の `interim:{input}:{output}` | 重複 | 変更しない | WARN `ACCT_DUPLICATE_START` | 終了 |
| Interim | 別値の `interim:*` | 正常 | `interim:{input}:{output}` | - | 継続 |
| Interim | `stop` | 順序異常（`interim_after_stop`） | `interim:{input}:{output}` | WARN `ACCT_SEQUENCE_ERR` | 継続 |
| Stop | `stop` | 重複 | 変更しない | なし | 終了 |
| Stop | 上記以外（未登録を含む） | 正常 | `stop` | - | 継続 |

Interimの判定（`CheckInterim`）は、1回の `Get` で直前の値を取得して重複・順序異常を判定してから、重複でなければ受信値を `Set` する。判定の前に自身の書き込みで状態が変わることはない。Startなし（未登録）・Stop後のInterimは順序異常として報告するが、課金データの欠損を避けるため受信値を記録して処理を継続する。

> **注記:** 修正前（本書 r8 時点）の実装は、Interimの重複判定（`CheckInterimDuplicate`）が先に `interim:{input}:{output}` を書き込んでから Start受信有無（`HasSeenStart`）を確認していたため、StartなしのInterimでも `ACCT_SEQUENCE_ERR` が出力されなかった。修正で `CheckInterim` に一本化し、`CheckInterimDuplicate` / `HasSeenStart` / `MarkAsStart` は削除した。Stop後のInterim（`interim_after_stop`）の検出も同じ修正で追加した。

```go
// internal/acct/duplicate.go（実装）
package acct

import (
    "context"
    "fmt"
    "strings"

    "github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/store"
)

// duplicateDetector はDuplicateDetectorインターフェースの実装。
type duplicateDetector struct {
    dupStore store.DuplicateStore
}

// NewDuplicateDetector は新しいDuplicateDetectorを生成する。
func NewDuplicateDetector(ds store.DuplicateStore) DuplicateDetector {
    return &duplicateDetector{dupStore: ds}
}

// CheckAndMarkStart はStartの重複をチェックし、未登録ならマークする。
func (d *duplicateDetector) CheckAndMarkStart(ctx context.Context, acctSessionID string) (bool, error) {
    val, err := d.dupStore.Get(ctx, acctSessionID)
    if err != nil {
        return false, err
    }

    if val == "" {
        // 新規：マークして継続
        if err := d.dupStore.Set(ctx, acctSessionID, "start"); err != nil {
            return false, err
        }
        return false, nil
    }

    // Stop後のStart検出（順序異常だが新規セッションとして扱う）
    if val == "stop" {
        if err := d.dupStore.Set(ctx, acctSessionID, "start"); err != nil {
            return false, err
        }
        return false, &SequenceError{Reason: "start_after_stop"}
    }

    // Start重複
    if val == "start" || strings.HasPrefix(val, "interim:") {
        return true, nil
    }

    return false, nil
}

// 順序異常の理由
const (
    seqReasonNoStart          = "no_start_received"
    seqReasonInterimAfterStop = "interim_after_stop"
)

// CheckInterim はInterimの重複と順序異常を判定する。
// 1回のGetで直前の状態を取得してから判定するため、判定前に自身の書き込みで状態が変わることはない。
// 同一のinput/output値が直前に記録されていれば重複とし、記録を変更しない。
// 重複でなければ（順序異常の場合も含め）受信値を記録する。
func (d *duplicateDetector) CheckInterim(ctx context.Context, acctSessionID string, input, output uint32) (InterimCheckResult, error) {
    var result InterimCheckResult
    currentVal := fmt.Sprintf("interim:%d:%d", input, output)

    val, err := d.dupStore.Get(ctx, acctSessionID)
    if err != nil {
        return result, err
    }

    switch val {
    case currentVal:
        result.Duplicate = true
        return result, nil
    case "":
        result.SequenceReason = seqReasonNoStart
    case "stop":
        result.SequenceReason = seqReasonInterimAfterStop
    }

    if err := d.dupStore.Set(ctx, acctSessionID, currentVal); err != nil {
        return result, err
    }
    return result, nil
}

// CheckStopDuplicate はStopの重複をチェックする。
func (d *duplicateDetector) CheckStopDuplicate(ctx context.Context, acctSessionID string) (bool, error) {
    val, err := d.dupStore.Get(ctx, acctSessionID)
    if err != nil {
        return false, err
    }
    return val == "stop", nil
}

// MarkAsStopped はStopとしてマークする。
func (d *duplicateDetector) MarkAsStopped(ctx context.Context, acctSessionID string) error {
    return d.dupStore.Set(ctx, acctSessionID, "stop")
}

// internal/acct/errors.go（抜粋）
// SequenceError は順序異常エラー
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
// HSET は不在のキーを新規作成するため、呼び出し側（ProcessInterim）で Exists により存在を確認してから呼び出す
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
| 3 | IMSI抽出失敗、User-Name取得成功 | マスク済みUser-Name（`logging.MaskUserName`。§6.3） |
| 4 | User-Name取得失敗、Class取得成功 | ClassのUUID |
| 5 | Class取得失敗 | `"unknown"` |

### 6.2 IMSI抽出ロジック

`ResolveIMSI` は `Start` / `Interim` / `Stop` の各プロセッサから `attrs.UserName`・`attrs.ClassUUID` を渡して呼び出す（`internal/session/interfaces.go` の `IdentifierResolver` インターフェース）。

```go
// internal/session/identifier.go
package session

import (
    "context"
    "regexp"
    "strings"

    "github.com/oyaguma3/eapaka-radius-server-poc/pkg/logging"
)

var imsiPattern = regexp.MustCompile(`^[0-9]{15}$`)

// identifierResolver はIdentifierResolverインターフェースの実装。
type identifierResolver struct {
    sessionManager SessionManager
    maskEnabled    bool
}

// NewIdentifierResolver は新しいIdentifierResolverを生成する。
func NewIdentifierResolver(sm SessionManager, maskEnabled bool) IdentifierResolver {
    return &identifierResolver{
        sessionManager: sm,
        maskEnabled:    maskEnabled,
    }
}

// ResolveIMSI はログ出力用のIMSI/識別子を取得する。
// 優先順位: セッション→User-Name→Class UUID→"unknown"
func (r *identifierResolver) ResolveIMSI(ctx context.Context, sessionUUID, userName, classUUID string) string {
    // 1. セッションからIMSI取得
    if sessionUUID != "" {
        sess, err := r.sessionManager.Get(ctx, sessionUUID)
        if err == nil && sess != nil && sess.IMSI != "" {
            return logging.MaskIMSI(sess.IMSI, r.maskEnabled)
        }
    }

    // 2. User-NameからIMSI抽出
    if userName != "" {
        imsi := extractIMSIFromIdentity(userName)
        if imsi != "" {
            return logging.MaskIMSI(imsi, r.maskEnabled)
        }
        // 3. IMSI抽出失敗、User-Nameを（IMSIを含みうるため）マスクして返却
        return logging.MaskUserName(userName, r.maskEnabled)
    }

    // 4. Class UUID
    if classUUID != "" {
        return classUUID
    }

    // 5. 取得失敗
    return "unknown"
}

// extractIMSIFromIdentity はEAP Identity形式からIMSIを抽出する。
// 形式: "0<IMSI>@<realm>" または "6<IMSI>@<realm>"
func extractIMSIFromIdentity(identity string) string {
    // @でrealm部分を除去
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
```

### 6.3 IMSIマスキング処理

D-04で定義されたマスキング仕様（D-04 §4.4.2）を、共通ライブラリ `pkg/logging`（E-03）の関数で実装する。`maskEnabled` には `LOG_MASK_IMSI` の設定値を渡す。

| 関数 | 用途 | 出力例（`LOG_MASK_IMSI=true`） |
|------|------|------------------------------|
| `logging.MaskIMSI(imsi, enabled)` | セッション・User-Nameから取得したIMSI（優先度1・2） | `440101234567890` → `440101********0` |
| `logging.MaskUserName(userName, enabled)` | IMSIを抽出できないUser-Name（優先度3）。`"@"` より前だけをマスクし、realmは残す | `1440101234567890@realm` → `1440101********0@realm`、`2some-pseudonym@example` → `2some-p*******m@example` |

> **注記:** 優先度3のUser-Nameは、EAP-SIM（先頭 `1`）等の永続IDではIMSIを含むため、`MaskUserName` でマスクして出力する。種別1文字＋IMSI 15桁の形式は種別文字を残してIMSI部分を `MaskIMSI` と同様にマスクし、それ以外（仮名・不正形式）は先頭7文字と末尾1文字を残してマスクする（8文字以下はそのまま）。`LOG_MASK_IMSI=false` の場合はそのまま出力する。

---

## ■セクション7: エラーハンドリング

### 7.1 エラー種別と対処

D-06で定義されたエラーハンドリングに基づく。

#### 7.1.1 通信エラー

| エラー種別 | 検出条件 | 対処 | RADIUS応答 | ログ |
|-----------|---------|------|-----------|------|
| Valkey接続失敗（起動時） | TCP接続エラー・AUTH失敗 | 起動時エラー終了 | - | ERROR: `VALKEY_CONN_ERR`（`error`） |
| Valkey接続失敗 | TCP接続エラー | リトライ（3回） | Accounting-Response | ERROR: `VALKEY_CONN_ERR`（読み取り系）/ `DB_WRITE_ERR`（書き込み系） |
| Valkeyコマンドタイムアウト | 応答なし（2秒超過） | リトライ | Accounting-Response | ERROR: `VALKEY_CONN_ERR`（読み取り系）/ `DB_WRITE_ERR`（書き込み系） |
| Shared Secret解決時のValkeyエラー | client:{IP}検索失敗 | 環境変数 `RADIUS_SECRET` があればその値で継続 | （Secret解決できない場合）なし | WARN: `RADIUS_SECRET_ERR` |

**重要:** Valkey障害時も課金パケットにはAccounting-Responseを返す（クライアントの再送を防ぐため）。データ欠損はログから追跡可能とする。実行時の `VALKEY_CONN_ERR` は `trace_id`, `error` を持つ（`retry_count` はない）。Valkey接続の復旧検知ログは出力しない（§10.2）。

#### 7.1.2 プロトコルエラー

| エラー種別 | 検出条件 | 対処 | RADIUS応答 | ログ |
|-----------|---------|------|-----------|------|
| パケットパース失敗 | 不正なRADIUS形式 | パケット破棄 | なし | なし（デコードは `layeh.com/radius` が行い、失敗時のログは出力しない） |
| 属性抽出失敗 | Acct-Status-Typeなし、Accounting-On/Off以外でAcct-Session-Idなし | パケット破棄 | なし | WARN: `RADIUS_PARSE_ERR`（`reason`） |
| Authenticator検証失敗 | 計算値不一致 | パケット破棄 | なし | WARN: `RADIUS_AUTH_ERR` |
| Message-Authenticator検証失敗 | Status-ServerのMAC検証失敗 | パケット破棄 | なし | WARN: `RADIUS_AUTH_ERR` |
| Shared Secret不明 | client:{IP}不在かつ環境変数未設定 | パケット破棄 | なし | WARN: `RADIUS_NO_SECRET` |
| 未知のRADIUS Code | Accounting-Request / Status-Server以外 | パケット破棄 | なし | WARN: `RADIUS_UNKNOWN_CODE`（`code`） |
| 未知のAcct-Status-Type | 1,2,3,7,8以外 | パケット破棄 | なし | WARN: `RADIUS_UNKNOWN_CODE`（`acct_status_type`） |
| 応答送信失敗 | Accounting-Response / Status-Server応答の送信エラー | - | - | ERROR: `PKT_SEND_ERR` |

#### 7.1.3 データエラー

| エラー種別 | 検出条件 | 対処 | RADIUS応答 | ログ |
|-----------|---------|------|-----------|------|
| セッション不在 | Start時にClass属性なし・不正、またはStart / Interim時に sess:{UUID}不在（TTL超過による削除済みを含む。区別しない）。Class属性なしのInterimは対象外（セッション更新・ログなし） | セッション更新せず処理継続（不在の sess:{UUID} は作成しない） | Accounting-Response | WARN: `ACCT_SESSION_NOT_FOUND` |
| Start重複 | 同一Acct-Session-Idで再Start | 既存維持 | Accounting-Response | WARN: `ACCT_DUPLICATE_START` |
| Interim重複 | 同一Acct-Session-Idで直前と同一値のInterim | 既存維持（処理終了） | Accounting-Response | WARN: `ACCT_DUPLICATE_START` |
| StartなしでInterim | Acct-Session-Id未登録（acct:seen なし）でInterim | 受信値を記録して処理継続（reason=`no_start_received`） | Accounting-Response | WARN: `ACCT_SEQUENCE_ERR` |
| Stop後のInterim | 同一Acct-Session-IdでStop受信後にInterim | 受信値を記録して処理継続（reason=`interim_after_stop`） | Accounting-Response | WARN: `ACCT_SEQUENCE_ERR` |
| Stop後にStart | 同一Acct-Session-Idで再Start | 新規セッションとして処理継続（reason=`start_after_stop`） | Accounting-Response | WARN: `ACCT_SEQUENCE_ERR` |
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
| `PKT_RECV` | INFO | Status-Server応答送信（Accounting-Request受信時は出力しない） |
| `VALKEY_CONN_ERR` | ERROR | 起動時のValkey接続失敗、実行時の読み取り系Valkeyエラー（重複・順序判定（Interim受信値の記録失敗を含む）・セッション存在確認） |
| `DB_WRITE_ERR` | ERROR | Valkey書き込み失敗（セッション更新・停止マーク・セッション削除・インデックス削除） |
| `SYS_ERR` | ERROR | Accounting処理がエラーを返した（現行は常にnilのため通常出力されない） |
| `PKT_SEND_ERR` | ERROR | Accounting-Response / Status-Server応答の送信失敗 |
| `RADIUS_PARSE_ERR` | WARN | Accounting-Requestの属性抽出失敗 |
| `RADIUS_AUTH_ERR` | WARN | Request Authenticator検証失敗、Status-ServerのMessage-Authenticator検証失敗 |
| `RADIUS_SECRET_ERR` | WARN | Shared Secret解決時のValkey検索エラー |
| `RADIUS_NO_SECRET` | WARN | Shared Secret不明 |
| `RADIUS_UNKNOWN_CODE` | WARN | 未知のRADIUS Code / 未知のAcct-Status-Type |
| `ACCT_SESSION_NOT_FOUND` | WARN | Start時のClass属性なし、Start / Interim時のセッション不在（TTL超過を含む） |
| `ACCT_DUPLICATE_START` | WARN | 重複Start / 直前と同一値のInterim |
| `ACCT_SEQUENCE_ERR` | WARN | 順序異常（Stop後のStart: `start_after_stop`、StartなしのInterim: `no_start_received`、Stop後のInterim: `interim_after_stop`） |
| `ACCT_START` | INFO | Accounting-Start受信 |
| `ACCT_INTERIM` | INFO | Accounting-Interim受信 |
| `ACCT_STOP` | INFO | Accounting-Stop受信 |
| `ACCT_ON` | INFO | Accounting-On受信（NAS起動通知） |
| `ACCT_OFF` | INFO | Accounting-Off受信（NASシャットダウン通知） |

> **注記:** 各event_idの `msg`・属性はD-04 §3.2を参照。Valkey接続の復旧検知ログ、セッションTTL超過専用のevent_idはない。

### 8.2 ログ出力例

```json
// ACCT_START
{
  "time": "2026-01-20T10:00:00.123Z",
  "level": "INFO",
  "app": "acct-server",
  "event_id": "ACCT_START",
  "msg": "accounting start",
  "trace_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
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
  "trace_id": "8d0f7780-8536-41ef-a55c-f18fd2fa1bf8",
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
  "trace_id": "9e1a8891-9647-42f0-b66d-a29ae3ab2c09",
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
  "msg": "interim sequence error",
  "trace_id": "a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d",
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
  "trace_id": "b2c3d4e5-f6a7-4b8c-9d0e-1f2a3b4c5d6e",
  "src_ip": "192.168.1.100",
  "acct_session_id": "sess-abc123"
}

// ACCT_ON（NAS起動通知）
{
  "time": "2026-01-20T09:00:00.100Z",
  "level": "INFO",
  "app": "acct-server",
  "event_id": "ACCT_ON",
  "msg": "accounting on",
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "src_ip": "192.168.1.100",
  "nas_ip_address": "192.168.1.100",
  "nas_identifier": "AP-Floor3"
}

// ACCT_OFF（NASシャットダウン通知）
{
  "time": "2026-01-20T18:00:00.200Z",
  "level": "INFO",
  "app": "acct-server",
  "event_id": "ACCT_OFF",
  "msg": "accounting off",
  "trace_id": "660e8400-e29b-41d4-a716-446655440001",
  "src_ip": "192.168.1.100",
  "nas_ip_address": "192.168.1.100",
  "nas_identifier": "AP-Floor3"
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
// internal/acct/interfaces.go
package acct

import "context"

type AccountingProcessor interface {
    ProcessStart(ctx context.Context, attrs *radius.AccountingAttributes, srcIP, traceID string) error
    ProcessInterim(ctx context.Context, attrs *radius.AccountingAttributes, srcIP, traceID string) error
    ProcessStop(ctx context.Context, attrs *radius.AccountingAttributes, srcIP, traceID string) error
    ProcessOn(ctx context.Context, attrs *radius.AccountingAttributes, srcIP, traceID string) error
    ProcessOff(ctx context.Context, attrs *radius.AccountingAttributes, srcIP, traceID string) error
}

// DuplicateDetector は重複・順序異常検出のインターフェース（§5.8）
type DuplicateDetector interface {
    // CheckAndMarkStart はStartの重複をチェックし、未登録ならマークする
    CheckAndMarkStart(ctx context.Context, acctSessionID string) (isDuplicate bool, err error)
    // CheckInterim はInterimの重複と順序異常を判定し、重複でなければ受信値を記録する
    CheckInterim(ctx context.Context, acctSessionID string, input, output uint32) (InterimCheckResult, error)
    // CheckStopDuplicate はStopの重複をチェックする
    CheckStopDuplicate(ctx context.Context, acctSessionID string) (isDuplicate bool, err error)
    // MarkAsStopped はStopとしてマークする
    MarkAsStopped(ctx context.Context, acctSessionID string) error
}

// InterimCheckResult はInterim受信時の重複・順序判定の結果
type InterimCheckResult struct {
    // Duplicate は直前に受信したInterimと同一値（重複）かどうか
    Duplicate bool
    // SequenceReason は順序異常の理由（正常なら空）
    //   - "no_start_received": Start（およびInterim）を受信していない
    //   - "interim_after_stop": Stop受信後のInterim
    SequenceReason string
}

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

    "github.com/google/uuid"
    "github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/acct"
    radiuspkg "github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/radius"
    "layeh.com/radius"
)

type Handler struct {
    processor acct.AccountingProcessor
}

func (h *Handler) ServeRADIUS(w radius.ResponseWriter, r *radius.Request) {
    traceID := uuid.New().String()
    srcIP := extractIP(r.RemoteAddr)

    switch r.Code {
    case radius.CodeAccountingRequest:
        h.handleAccountingRequest(w, r, traceID, srcIP)

    case radius.CodeStatusServer:
        h.handleStatusServer(w, r, traceID, srcIP)

    default:
        slog.Warn("未対応のRADIUS Code",
            "event_id", "RADIUS_UNKNOWN_CODE",
            "trace_id", traceID,
            "src_ip", srcIP,
            "code", r.Code)
    }
}

func (h *Handler) handleAccountingRequest(w radius.ResponseWriter, r *radius.Request, traceID, srcIP string) {
    // 1. Request Authenticator検証
    if !radiuspkg.VerifyAccountingAuthenticator(r.Packet, r.Secret) {
        slog.Warn("Authenticator検証失敗",
            "event_id", "RADIUS_AUTH_ERR",
            "trace_id", traceID,
            "src_ip", srcIP)
        return
    }

    // 2. 属性抽出
    attrs, err := radiuspkg.ExtractAccountingAttributes(r.Packet)
    if err != nil {
        slog.Warn("属性抽出失敗",
            "event_id", "RADIUS_PARSE_ERR",
            "trace_id", traceID,
            "src_ip", srcIP,
            "reason", err.Error())
        return
    }

    // 3. Status-Type別処理
    ctx := context.Background()
    var procErr error
    switch attrs.AcctStatusType {
    case radiuspkg.AcctStatusTypeStart:
        procErr = h.processor.ProcessStart(ctx, attrs, srcIP, traceID)
    case radiuspkg.AcctStatusTypeStop:
        procErr = h.processor.ProcessStop(ctx, attrs, srcIP, traceID)
    case radiuspkg.AcctStatusTypeInterim:
        procErr = h.processor.ProcessInterim(ctx, attrs, srcIP, traceID)
    case radiuspkg.AcctStatusTypeOn:
        procErr = h.processor.ProcessOn(ctx, attrs, srcIP, traceID)
    case radiuspkg.AcctStatusTypeOff:
        procErr = h.processor.ProcessOff(ctx, attrs, srcIP, traceID)
    default:
        slog.Warn("未対応のAcct-Status-Type",
            "event_id", "RADIUS_UNKNOWN_CODE",
            "trace_id", traceID,
            "src_ip", srcIP,
            "acct_status_type", attrs.AcctStatusType)
        return // パケット破棄
    }

    // 4. 処理エラーがあってもAccounting-Responseは返す
    if procErr != nil {
        slog.Error("処理エラー",
            "event_id", "SYS_ERR",
            "trace_id", traceID,
            "error", procErr.Error())
    }

    // 5. Accounting-Response生成・送信
    response := radiuspkg.BuildAccountingResponse(r.Packet, attrs.ProxyStates)
    w.Write(response)
}
```

### 10.2 Valkey接続復旧検知

Valkey接続の復旧検知（`downtime_ms` の記録）は実装しない（D-04 §4.3）。go-redisのコネクションプールが次のコマンド実行時に接続を張り直すため、復旧は `VALKEY_CONN_ERR` / `DB_WRITE_ERR` の出力が止まったことで判断する。

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
| 1 | Acct-Delay-Time考慮 | タイムスタンプ補正 | PoC完了後 |
| 2 | 複数Class属性対応 | 複数UUIDの処理 | PoC完了後 |
| 3 | Event-Timestamp記録 | RFC 2869属性の参照記録 | PoC完了後 |
| 4 | 大量トラフィック対応 | 並行処理の最適化 | PoC完了後 |

---

## 改訂履歴

| 版数 | 日付 | 内容 |
|------|------|------|
| r1 | 2026-01-20 | 初版作成 |
| r2 | 2026-01-21 | Status-Server対応追加: セクション1.4からPoC対象外削除、セクション1.5にRFC 5997追加、セクション2.1/2.3にstatus.go追加、セクション4.7新設（Status-Server処理フロー・実装）、セクション7.1.2にMessage-Authenticator検証失敗追加、セクション8.1にPKT_RECV追加、セクション10.1ハンドラー更新、セクション11からStatus-Server削除 |
| r3 | 2026-01-26 | インフラ基盤統一: セクション2.5新設（Dockerfile方針 - ベースイメージdebian:bookworm-slim、curl/ca-certificates導入）、環境変数RADIUS_SECRETの統一に関する注記追加 |
| r4 | 2026-01-27 | ヘルスチェック整合性修正: セクション2.5.1 Dockerfileに`procps`パッケージ追加、セクション2.5.3必須パッケージに`procps`追記（pgrep用） |
| r5 | 2026-02-18 | ディレクトリ構造全面更新、関連ドキュメント版数更新 |
| r6 | 2026-03-05 | Accounting-On/Off対応: §1.2スコープ追加、§1.4対象外から削除、§1.5/§1.6更新、§2.1/§2.3更新、§4.1処理フロー拡張、§4.4属性抽出更新（NAS-Identifier追加・Acct-Session-Id On/Off省略許容・実装コード整合）、§5.6/§5.7新設（ProcessOn/ProcessOff）、§7.1.2/§8.1/§8.2更新、§9.2インターフェース更新、§10.1ハンドラー更新、§11から削除 |
| r7 | 2026-10-04 | D-04 r19 の event_id 全面整合に合わせて修正: §8.1 event_id一覧から実装に存在しない `VALKEY_CONN_RESTORED` / `ACCT_SESSION_EXPIRED` を削除し、`SYS_ERR` / `PKT_SEND_ERR` / `RADIUS_SECRET_ERR` を追加、各説明を実装の出力条件に修正。§7.1.1〜§7.1.3 のエラー表を修正（起動時 `VALKEY_CONN_ERR`、書き込み系 `DB_WRITE_ERR`、`RADIUS_SECRET_ERR`、`RADIUS_PARSE_ERR` は属性抽出失敗でありパケットデコード失敗はログなし、未知のRADIUS Code、`PKT_SEND_ERR` を追加。セッションTTL超過は `ACCT_SESSION_NOT_FOUND` と区別しない旨を明記）。§10.2 Valkey接続復旧検知を「実装しない」に改め実装例を削除。§4.2 Shared Secret解決、§5.3〜§5.5 Start/Interim/Stop処理のコード例を実装（`trace_id` 付与、`RADIUS_SECRET_ERR`、重複チェック時の `VALKEY_CONN_ERR`、Stop後Startの `ACCT_SEQUENCE_ERR`、停止マーク失敗の `DB_WRITE_ERR`）に合わせて更新。§4.7 Status-Server処理のログ（msg・`trace_id`、`packet_code` 削除）を実装に合わせて修正。§8.2 ログ出力例に `trace_id` を追加し、StartなしInterimの msg を `interim without start` に修正。§1.3 関連ドキュメントの版数を現行版に更新（D-01 r10、D-02 r12、D-03 r6（文書名も現行名に修正）、D-04 r19、D-06 r7、D-08 r14、D-09 r10、E-02 r3）。関連ドキュメント表の文書名を実在の文書に修正（存在しない D-05「Valkeyキー・TTL設計書」を削除、D-07「AKA Vector Server」→ D-11 Vector API詳細設計書、E-03 → 共通ライブラリ(pkg)設計書） |
| r8 | 2026-10-04 | ログのIMSIマスク漏れ修正の反映: §6.1 IMSI取得優先順位の優先度3（IMSI抽出失敗時のUser-Name）を「そのまま」からマスク済み（pkg/logging.MaskUserName）に修正、§6.2のResolveIMSIのコードを実装（`internal/session/identifier.go`。引数 userName / classUUID、pkg/logging の MaskIMSI / MaskUserName を使用）に合わせ更新、§6.3を pkg/logging の MaskIMSI / MaskUserName による実装に更新。関連ドキュメント参照版数更新（D-04 r19→r20、D-06 r7→r8、D-09 r10→r11、E-03 r3→r4）。あわせて、廃止済みの internal/logging（mask.go）をディレクトリ構成から削除（IMSIマスキングは pkg/logging を使用） |
| r9 | 2026-10-04 | Interimのシーケンス判定修正の反映: §5.4 Acct-Interim処理を実装（`CheckInterim` による重複・順序異常の一括判定、順序異常の msg `interim sequence error`、`sess:{UUID}` の存在確認と不在時の `ACCT_SESSION_NOT_FOUND`）に合わせて説明とコードを更新。§5.8 重複・順序異常検出を実装（`internal/acct/duplicate.go`）のコードに差し替え、`acct:seen` の値による判定表を追加（Stop後のInterim `interim_after_stop` を新設。削除した `CheckInterimDuplicate` / `HasSeenStart` / `MarkAsStart` の記載を削除し、旧実装でStartなしのInterimを検出できなかった旨を注記）。§5.7 `UpdateOnInterim` に存在確認後に呼び出す旨を注記。§7.1.3 データエラー表（セッション不在にInterimを追加、StartなしでInterimの対処を「受信値を記録して処理継続」に修正、Stop後のInterimを新設）、§8.1 event_id一覧（`VALKEY_CONN_ERR`、`ACCT_SESSION_NOT_FOUND`、`ACCT_DUPLICATE_START`、`ACCT_SEQUENCE_ERR` の説明）、§8.2 ログ出力例（StartなしInterimの msg）、§9.2 インターフェース定義（`DuplicateDetector`、`InterimCheckResult` を追加）を更新。関連ドキュメント参照版数更新（D-02 r12→r13、D-04 r20→r21） |
