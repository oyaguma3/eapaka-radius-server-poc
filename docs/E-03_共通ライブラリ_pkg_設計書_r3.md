# E-03 共通ライブラリ(pkg)設計書 (r3)

## 1. 概要

### 1.1 目的

本ドキュメントは、EAP-AKA RADIUS PoC環境において複数コンポーネントで共有する共通ライブラリ（`pkg/`ディレクトリ）の設計を定義する。

### 1.2 スコープ

**本書で扱う範囲：**

- `pkg/` ディレクトリ配下のパッケージ構成
- 各パッケージの責務・インターフェース定義
- パッケージ間の依存関係

**本書で扱わない範囲：**

| 範囲 | 参照先 |
|------|--------|
| アプリケーション固有のinternal実装 | D-09〜D-12（各詳細設計書） |
| コーディング規約全般 | E-02 コーディング規約（簡易版） |
| Valkeyデータ構造の詳細 | D-02 Valkeyデータ設計仕様書 |

### 1.3 関連ドキュメント

| ドキュメント | 参照内容 |
|-------------|---------|
| D-01 ミニPC版設計仕様書 (r9) | リポジトリ構成、パッケージ利用マップ |
| D-02 Valkeyデータ設計仕様書 (r11) | Go構造体定義、ストア層変換方式 |
| D-04 ログ仕様設計書 (r17) | IMSIマスキング仕様 |
| D-06 エラーハンドリング詳細設計書 (r6) | エラー定義パターン |
| D-11 Vector API詳細設計書 (r6) | RFC 7807 Problem Details |
| E-02 コーディング規約（簡易版）(r1) | pkg配置方針、命名規則 |

### 1.4 pkg配置方針

E-02 セクション3.3で定義された方針に基づき、以下の基準を満たすコードのみpkgに配置する。

| 基準 | 説明 | 判定例 |
|------|------|--------|
| **利用数** | 2つ以上のアプリケーションで使用 | Auth + Acct で使用 → ○ |
| **安定性** | APIが安定しており、頻繁な変更が予想されない | エラー定義 → ○ |
| **依存最小化** | 外部パッケージへの依存が最小限 | go-redis のみ → ○ |
| **汎用性** | ドメイン固有ロジックを含まない | EAP処理 → × |

**配置判断フロー:**

```
コードの共有が必要
    │
    ├─ 2つ以上のアプリで使用する？
    │       │
    │       ├─ No → internal/ に配置
    │       │
    │       └─ Yes ─┬─ APIは安定している？
    │               │       │
    │               │       ├─ No → internal/ に配置（将来移行検討）
    │               │       │
    │               │       └─ Yes ─┬─ ドメイン固有ロジックを含む？
    │               │               │       │
    │               │               │       ├─ Yes → internal/ に配置
    │               │               │       │
    │               │               │       └─ No → pkg/ に配置 ✓
```

---

## 2. パッケージ構成

### 2.1 ディレクトリ構造

```
pkg/
├── go.mod                    # モジュール定義
├── apperr/                   # 共通エラー定義
│   ├── errors.go             # センチネルエラー定義
│   └── custom.go             # カスタムエラー型定義
├── valkey/                   # Valkeyクライアント共通化
│   ├── client.go             # クライアント初期化・ヘルパー関数
│   └── options.go            # 接続オプション・BuildAddr
├── logging/                  # ログユーティリティ
│   ├── masking.go            # IMSIマスキング・Masker構造体
│   └── fields.go             # フィールド定数・CommonFields・AuthLogFields
├── model/                    # 共通データ構造体
│   ├── subscriber.go         # Subscriber構造体・NewSubscriber
│   ├── client.go             # RadiusClient構造体・NewRadiusClient
│   ├── session.go            # Session・EAPContext・Stage型・NewSession・NewEAPContext
│   └── policy.go             # Policy・PolicyRule構造体・NewPolicy
└── httputil/                 # HTTPユーティリティ
    ├── problem.go            # ProblemDetail構造体・コンストラクタ・ContentType定数
    └── gin.go                # Ginフレームワーク統合（WriteError, AbortWithError）
```

### 2.2 パッケージ一覧

| パッケージ | 責務 | 主要な型・関数 |
|-----------|------|---------------|
| `apperr` | 共通エラー定義 | センチネルエラー、カスタムエラー型（ValidationError, BackendError, ValkeyError, EAPIdentityError） |
| `valkey` | Valkeyクライアント初期化 | `NewClient()`, `Options`, `DefaultOptions()`, `TUIOptions()`, `BuildAddr()` |
| `logging` | ログユーティリティ | `MaskIMSI()`, `CommonFields`, `AuthLogFields()`, フィールド定数8種 |
| `model` | 共通データ構造体 | `Subscriber`, `RadiusClient`, `Session`, `EAPContext`, `Policy`, `PolicyRule`, `Stage` |
| `httputil` | HTTPユーティリティ | `ProblemDetail`, `ContentType`, `WriteError()`, `AbortWithError()` |

### 2.3 利用コンポーネント対応表

| パッケージ | Auth Server | Acct Server | Vector Gateway | Vector API | Admin TUI |
|-----------|:-----------:|:-----------:|:--------------:|:----------:|:---------:|
| `apperr` | ◎ | ◎ | ◎ | ◎ | ○ |
| `valkey` | ◎ | ◎ | - | ◎ | ◎ |
| `logging` | ◎ | ◎ | ◎ | ◎ | - |
| `model` | ◎ | ◎ | - | ◎ | ◎ |
| `httputil` | - | - | ◎ | ◎ | - |

**凡例:** ◎=必須, ○=任意, -=不使用

### 2.4 go.mod 定義

```go
// pkg/go.mod

module eap-aka-radius-poc/pkg

go 1.25

require (
    github.com/redis/go-redis/v9 v9.x.x
)
```

---

## 3. pkg/apperr（共通エラー定義）

### 3.1 責務

- プロジェクト共通のセンチネルエラー定義
- 詳細情報を持つカスタムエラー型の提供
- `errors.Is` / `errors.As` での判定をサポート

### 3.2 センチネルエラー定義

D-06「エラーハンドリング詳細設計書」で定義されたエラーを集約する。

**ファイル: `pkg/apperr/errors.go`**

```go
package apperr

import "errors"

// ============================================================================
// 認証関連エラー
// ============================================================================

// ErrIMSINotFound はIMSIが見つからない場合のエラー
var ErrIMSINotFound = errors.New("IMSI not found")

// ErrAuthFailed は認証失敗エラー
var ErrAuthFailed = errors.New("authentication failed")

// ErrAuthResMismatch はRES不一致エラー
var ErrAuthResMismatch = errors.New("authentication response mismatch")

// ErrAuthMACInvalid はMAC検証失敗エラー
var ErrAuthMACInvalid = errors.New("invalid MAC")

// ErrAuthTimeout は認証タイムアウトエラー
var ErrAuthTimeout = errors.New("authentication timeout")

// ErrAuthResyncLimit は再同期回数上限エラー
var ErrAuthResyncLimit = errors.New("resync limit exceeded")

// ErrUnsupportedEAPType は未サポートのEAPタイプエラー
var ErrUnsupportedEAPType = errors.New("unsupported EAP type")

// ============================================================================
// セッション関連エラー
// ============================================================================

// ErrSessionNotFound はセッションが見つからない場合のエラー
var ErrSessionNotFound = errors.New("session not found")

// ErrSessionExpired はセッション有効期限切れエラー
var ErrSessionExpired = errors.New("session expired")

// ErrContextNotFound はEAPコンテキストが見つからない場合のエラー
var ErrContextNotFound = errors.New("EAP context not found")

// ============================================================================
// ポリシー関連エラー
// ============================================================================

// ErrPolicyNotFound はポリシーが見つからない場合のエラー
var ErrPolicyNotFound = errors.New("policy not found")

// ErrPolicyDenied はポリシーによる拒否エラー
var ErrPolicyDenied = errors.New("policy denied")

// ============================================================================
// インフラ関連エラー
// ============================================================================

// ErrValkeyConnection はValkey接続エラー
var ErrValkeyConnection = errors.New("valkey connection error")

// ErrValkeyCommand はValkeyコマンド実行エラー
var ErrValkeyCommand = errors.New("valkey command error")

// ErrVectorAPI はVector Gateway APIエラー
var ErrVectorAPI = errors.New("vector API error")

// ============================================================================
// Vector Gateway関連エラー
// ============================================================================

// ErrBackendNotImplemented はバックエンド未実装エラー
var ErrBackendNotImplemented = errors.New("backend not implemented")

// ErrBackendCommunication はバックエンド通信エラー
var ErrBackendCommunication = errors.New("backend communication error")

// ErrInvalidRequest は不正なリクエストエラー
var ErrInvalidRequest = errors.New("invalid request")

// ============================================================================
// RADIUS関連エラー
// ============================================================================

// ErrClientNotFound はRADIUSクライアントが見つからない場合のエラー
var ErrClientNotFound = errors.New("RADIUS client not found")

// ErrInvalidAuthenticator は不正なAuthenticatorエラー
var ErrInvalidAuthenticator = errors.New("invalid authenticator")

// ============================================================================
// バリデーション関連エラー
// ============================================================================

// ErrInvalidIMSI は不正なIMSI形式エラー
var ErrInvalidIMSI = errors.New("invalid IMSI format")

// ErrInvalidHex は不正な16進数文字列エラー
var ErrInvalidHex = errors.New("invalid hex string")
```

### 3.3 カスタムエラー型

詳細情報が必要なエラーはカスタム構造体で定義する。

**ファイル: `pkg/apperr/custom.go`**

```go
package apperr

import "fmt"

// ============================================================================
// ValidationError: バリデーションエラー
// ============================================================================

type ValidationError struct {
    Field   string // エラーが発生したフィールド名
    Message string // エラーメッセージ
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation error: field=%s, message=%s", e.Field, e.Message)
}

func NewValidationError(field, message string) *ValidationError {
    return &ValidationError{
        Field:   field,
        Message: message,
    }
}

// ============================================================================
// BackendError: バックエンド通信エラー
// ============================================================================

type BackendError struct {
    BackendID  string // バックエンドの識別子
    StatusCode int    // HTTPステータスコード
    Cause      error  // 根本原因
}

func (e *BackendError) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("backend error: backendID=%s, statusCode=%d, cause=%v",
            e.BackendID, e.StatusCode, e.Cause)
    }
    return fmt.Sprintf("backend error: backendID=%s, statusCode=%d",
        e.BackendID, e.StatusCode)
}

func (e *BackendError) Unwrap() error {
    return e.Cause
}

func NewBackendError(backendID string, statusCode int, cause error) *BackendError {
    return &BackendError{
        BackendID:  backendID,
        StatusCode: statusCode,
        Cause:      cause,
    }
}

// ============================================================================
// ValkeyError: Valkey操作エラー
// ============================================================================

type ValkeyError struct {
    Operation string // 操作名（GET, SET, DEL等）
    Key       string // 操作対象のキー
    Cause     error  // 根本原因
}

func (e *ValkeyError) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("valkey error: operation=%s, key=%s, cause=%v",
            e.Operation, e.Key, e.Cause)
    }
    return fmt.Sprintf("valkey error: operation=%s, key=%s", e.Operation, e.Key)
}

func (e *ValkeyError) Unwrap() error {
    return e.Cause
}

func NewValkeyError(operation, key string, cause error) *ValkeyError {
    return &ValkeyError{
        Operation: operation,
        Key:       key,
        Cause:     cause,
    }
}

// ============================================================================
// EAPIdentityError: EAP Identity解析エラー
// ============================================================================

type EAPIdentityError struct {
    Identity     string // 受け取ったIdentity文字列
    IdentityType string // Identityの種類（permanent, pseudonym等）
    Reason       string // エラーの理由
}

func (e *EAPIdentityError) Error() string {
    return fmt.Sprintf("EAP identity error: identity=%s, type=%s, reason=%s",
        e.Identity, e.IdentityType, e.Reason)
}

func NewEAPIdentityError(identity, identityType, reason string) *EAPIdentityError {
    return &EAPIdentityError{
        Identity:     identity,
        IdentityType: identityType,
        Reason:       reason,
    }
}
```

### 3.4 使用例

```go
// apps/auth-server/internal/store/ でのセンチネルエラー使用例

func (s *Store) GetSubscriber(ctx context.Context, imsi string) (*model.Subscriber, error) {
    // ...
    if len(result) == 0 {
        return nil, apperr.ErrIMSINotFound
    }
    // ...
}

// errors.Is / errors.As による判定例

func handleError(err error) {
    switch {
    case errors.Is(err, apperr.ErrIMSINotFound):
        // Access-Reject応答
    case errors.Is(err, apperr.ErrPolicyDenied):
        // ポリシー拒否によるAccess-Reject応答
    default:
        var valkeyErr *apperr.ValkeyError
        if errors.As(err, &valkeyErr) {
            // Valkeyエラーの詳細をログ出力
        }
    }
}
```

---

## 4. pkg/valkey（Valkeyクライアント共通化）

### 4.1 責務

- Valkeyクライアントの初期化処理の統一
- 接続オプションのデフォルト値提供
- 接続確認（PING）の共通化

### 4.2 接続オプション

**ファイル: `pkg/valkey/options.go`**

```go
package valkey

import (
    "fmt"
    "time"
)

// Options はValkeyクライアントの接続オプション
type Options struct {
    Addr           string        // 接続先アドレス（host:port形式）
    Password       string        // 認証パスワード
    DB             int           // データベース番号
    ConnectTimeout time.Duration // 接続タイムアウト
    ReadTimeout    time.Duration // 読み取りタイムアウト
    WriteTimeout   time.Duration // 書き込みタイムアウト
    PoolSize       int           // コネクションプールサイズ
    MinIdleConns   int           // 最小アイドルコネクション数
}

// DefaultOptions はデフォルトのOptionsを返す
func DefaultOptions() *Options {
    return &Options{
        Addr:           "localhost:6379",
        Password:       "",
        DB:             0,
        ConnectTimeout: 3 * time.Second,
        ReadTimeout:    2 * time.Second,
        WriteTimeout:   2 * time.Second,
        PoolSize:       10,
        MinIdleConns:   2,
    }
}

// TUIOptions はTUIアプリケーション向けのOptionsを返す
func TUIOptions() *Options {
    return &Options{
        Addr:           "localhost:6379",
        Password:       "",
        DB:             0,
        ConnectTimeout: 5 * time.Second,
        ReadTimeout:    5 * time.Second,
        WriteTimeout:   5 * time.Second,
        PoolSize:       5,
        MinIdleConns:   1,
    }
}
```

**DefaultOptions / TUIOptions のデフォルト値比較:**

| 項目 | DefaultOptions | TUIOptions |
|------|---------------|------------|
| Addr | `localhost:6379` | `localhost:6379` |
| ConnectTimeout | 3秒 | 5秒 |
| ReadTimeout | 2秒 | 5秒 |
| WriteTimeout | 2秒 | 5秒 |
| PoolSize | 10 | 5 |
| MinIdleConns | 2 | 1 |

**ビルダーメソッド:**

```go
func (o *Options) WithAddr(addr string) *Options
func (o *Options) WithPassword(password string) *Options
func (o *Options) WithDB(db int) *Options
func (o *Options) WithTimeouts(connect, read, write time.Duration) *Options
func (o *Options) WithPool(poolSize, minIdle int) *Options

// BuildAddr はホストとポートからアドレス文字列を生成する（options.goに定義）
func BuildAddr(host string, port int) string {
    return fmt.Sprintf("%s:%d", host, port)
}
```

### 4.3 クライアント初期化

**ファイル: `pkg/valkey/client.go`**

```go
package valkey

import (
    "context"
    "errors"
    "net"
    "time"

    "github.com/redis/go-redis/v9"
)

// NewClient は新しいValkeyクライアントを生成する。
// 接続確認のためPINGを実行し、失敗した場合はエラーを返す。
func NewClient(opts *Options) (*redis.Client, error) {
    ctx, cancel := context.WithTimeout(context.Background(), opts.ConnectTimeout)
    defer cancel()
    return NewClientWithContext(ctx, opts)
}

// NewClientWithContext は指定されたコンテキストでValkeyクライアントを生成する。
func NewClientWithContext(ctx context.Context, opts *Options) (*redis.Client, error) {
    if opts == nil {
        opts = DefaultOptions()
    }

    client := redis.NewClient(&redis.Options{
        Addr:         opts.Addr,
        Password:     opts.Password,
        DB:           opts.DB,
        DialTimeout:  opts.ConnectTimeout,
        ReadTimeout:  opts.ReadTimeout,
        WriteTimeout: opts.WriteTimeout,
        PoolSize:     opts.PoolSize,
        MinIdleConns: opts.MinIdleConns,
    })

    // 接続確認
    if err := client.Ping(ctx).Err(); err != nil {
        _ = client.Close()
        return nil, err
    }

    return client, nil
}

// MustNewClient は新しいValkeyクライアントを生成する。
// 接続に失敗した場合はパニックする。
func MustNewClient(opts *Options) *redis.Client {
    client, err := NewClient(opts)
    if err != nil {
        panic(err)
    }
    return client
}
```

### 4.4 ヘルパー関数

```go
// pkg/valkey/client.go

// IsConnectionError は接続関連のエラーかどうかを判定する。
// net.Error（タイムアウト）、net.OpError（接続拒否等）、コンテキストエラーを検出する。
func IsConnectionError(err error) bool {
    if err == nil {
        return false
    }

    // タイムアウトエラー
    var netErr net.Error
    if errors.As(err, &netErr) {
        return netErr.Timeout()
    }

    // 接続拒否など
    var opErr *net.OpError
    if errors.As(err, &opErr) {
        return true
    }

    // コンテキストエラー
    if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
        return true
    }

    return false
}

// IsKeyNotFound はキーが見つからないエラーかどうかを判定する。
func IsKeyNotFound(err error) bool {
    return errors.Is(err, redis.Nil)
}

// DefaultPingInterval はヘルスチェック用のデフォルトPING間隔。
const DefaultPingInterval = 30 * time.Second
```

### 4.5 使用例

```go
// apps/auth-server/main.go
func main() {
    cfg, _ := config.Load()

    valkeyOpts := valkey.DefaultOptions().
        WithAddr(valkey.BuildAddr(cfg.RedisHost, cfg.RedisPort)).
        WithPassword(cfg.RedisPass)

    client, err := valkey.NewClient(valkeyOpts)
    if err != nil {
        slog.Error("failed to connect to valkey", "error", err)
        os.Exit(1)
    }
    defer client.Close()
    // ...
}

// apps/admin-tui/main.go（TUI用オプション使用）
func main() {
    cfg, _ := config.Load()

    valkeyOpts := valkey.TUIOptions().
        WithAddr(valkey.BuildAddr(cfg.RedisHost, cfg.RedisPort)).
        WithPassword(cfg.RedisPass)

    client, err := valkey.NewClient(valkeyOpts)
    // ...
}
```

---

## 5. pkg/logging（ログユーティリティ）

### 5.1 責務

- IMSIマスキング処理の提供
- 共通ログフィールドの定義
- マスキング設定の一元管理

### 5.2 フィールド名定数

**ファイル: `pkg/logging/fields.go`**

```go
const (
    FieldTraceID    = "trace_id"
    FieldEventID    = "event_id"
    FieldError      = "error"
    FieldSrcIP      = "src_ip"
    FieldLatencyMs  = "latency_ms"
    FieldHTTPStatus = "http_status"
    FieldRetryCount = "retry_count"
    FieldIMSI       = "imsi"
)
```

### 5.3 フィールド生成関数

```go
// パッケージレベル関数（マスキング不要なフィールド）
func WithTraceID(traceID string) slog.Attr
func WithEventID(eventID string) slog.Attr
func WithError(err error) slog.Attr      // err==nilの場合は空文字列のAttrを返す
func WithSrcIP(ip string) slog.Attr
func WithLatency(ms int64) slog.Attr
func WithHTTPStatus(status int) slog.Attr
func WithRetryCount(count int) slog.Attr
```

### 5.4 CommonFields（マスキング対応フィールド生成器）

```go
// CommonFields はマスキング設定を保持するログフィールド生成器
type CommonFields struct {
    masker *Masker
}

// NewCommonFields は新しいCommonFieldsを生成する。
// maskerがnilの場合はマスキング無効のMaskerを自動生成する（nilガード）。
func NewCommonFields(masker *Masker) *CommonFields {
    if masker == nil {
        masker = NewMasker(false)
    }
    return &CommonFields{masker: masker}
}

// WithIMSI はマスキングされたIMSIのslog.Attrを返す
func (cf *CommonFields) WithIMSI(imsi string) slog.Attr

// AuthLogFields は認証ログ用の共通フィールドセットを返す
// 返り値: []any{WithTraceID(traceID), WithEventID(eventID), cf.WithIMSI(imsi)}
func (cf *CommonFields) AuthLogFields(traceID, eventID, imsi string) []any
```

### 5.5 IMSIマスキング

D-04「ログ仕様設計書」で定義されたマスキング仕様を実装する。

**ファイル: `pkg/logging/masking.go`**

```go
// MaskIMSI はIMSIをマスキングする
//
// マスキング仕様（D-04 r17準拠）:
//   - 先頭6桁を保持
//   - 末尾1桁を保持
//   - 中間部分をアスタリスク(*)でマスク
//   - 内部的に MaskPartial(imsi, 6, 1, '*') を使用
//
// 例: 440101234567890 → 440101********0
//
// enabled=false の場合はマスキングせずそのまま返す
func MaskIMSI(imsi string, enabled bool) string

// MaskPartial は文字列の一部をマスキングする汎用関数
// 文字列が keepPrefix+keepSuffix 以下の長さの場合はそのまま返す
func MaskPartial(s string, keepPrefix, keepSuffix int, maskChar rune) string

// Masker はマスキング設定を保持する構造体
type Masker struct {
    enabled bool
}

func NewMasker(enabled bool) *Masker
func (m *Masker) IMSI(imsi string) string
func (m *Masker) IsEnabled() bool
```

### 5.6 使用例

```go
// マスキング有効での認証ログ出力
masker := logging.NewMasker(true)
logFields := logging.NewCommonFields(masker)

slog.Info("authentication started",
    logFields.AuthLogFields(traceID, "AUTH_START", imsi)...)

// 個別フィールドの使用
slog.Info("EAP challenge sent",
    logging.WithEventID("EAP_CHALLENGE_SENT"),
    logging.WithTraceID(traceID),
    logFields.WithIMSI(imsi),
)
```

---

## 6. pkg/model（共通データ構造体）

### 6.1 責務

- D-02「Valkeyデータ設計仕様書」で定義された構造体の共有
- 複数コンポーネントで使用するデータ型の一元管理
- 各アプリのストア層（`internal/store/`）でValkey Hashフィールドとの変換を行う。model構造体自体にはredisタグを付与せず、jsonタグのみを使用する

### 6.2 マスターデータ構造体

#### Subscriber（加入者情報）

**ファイル: `pkg/model/subscriber.go`**

```go
package model

// Subscriber は加入者情報を表す。
// Valkeyキー: sub:{IMSI}
type Subscriber struct {
    IMSI      string `json:"imsi"`       // 国際移動体加入者識別番号（15桁）
    Ki        string `json:"ki"`         // 秘密鍵（32文字16進数）
    OPc       string `json:"opc"`        // オペレータ定数（32文字16進数）
    AMF       string `json:"amf"`        // 認証管理フィールド（4文字16進数）
    SQN       string `json:"sqn"`        // シーケンス番号（12文字16進数）
    CreatedAt string `json:"created_at"` // 作成日時（RFC3339形式）
}

// NewSubscriber は新しいSubscriberを生成する。
func NewSubscriber(imsi, ki, opc, amf, sqn, createdAt string) *Subscriber {
    return &Subscriber{
        IMSI:      imsi,
        Ki:        ki,
        OPc:       opc,
        AMF:       amf,
        SQN:       sqn,
        CreatedAt: createdAt,
    }
}
```

#### RadiusClient（RADIUSクライアント情報）

**ファイル: `pkg/model/client.go`**

```go
package model

// RadiusClient はRADIUSクライアント情報を表す。
// Valkeyキー: client:{IP}
type RadiusClient struct {
    IP     string `json:"ip"`     // クライアントIPアドレス
    Secret string `json:"secret"` // 共有シークレット
    Name   string `json:"name"`   // クライアント名（識別用）
    Vendor string `json:"vendor"` // ベンダー名（任意）
}

// NewRadiusClient は新しいRadiusClientを生成する。
func NewRadiusClient(ip, secret, name, vendor string) *RadiusClient {
    return &RadiusClient{
        IP:     ip,
        Secret: secret,
        Name:   name,
        Vendor: vendor,
    }
}
```

### 6.3 セッション関連構造体

#### Stage型（EAP認証ステージ定数）

**ファイル: `pkg/model/session.go`**

```go
// Stage はEAP認証のステージを表す定数。
type Stage string

const (
    StageNew              Stage = "new"               // 新規セッション
    StageWaitingIdentity  Stage = "waiting_identity"  // Identity待ち状態
    StageIdentityReceived Stage = "identity_received" // Identity受信済み状態
    StageWaitingVector    Stage = "waiting_vector"    // Vector待ち状態
    StageChallengeSent    Stage = "challenge_sent"    // Challenge送信済み状態
    StageResyncSent       Stage = "resync_sent"       // 再同期要求送信済み状態
    StageSuccess          Stage = "success"           // 認証成功状態
    StageFailure          Stage = "failure"           // 認証失敗状態
)
```

#### Session（RADIUSセッション情報）

```go
// Session はRADIUSセッション情報を表す。
// Valkeyキー: sess:{UUID}
// TTL: 24時間
type Session struct {
    UUID          string `json:"uuid"`            // セッション識別子
    IMSI          string `json:"imsi"`            // 加入者IMSI
    NasIP         string `json:"nas_ip"`          // NAS IPアドレス
    ClientIP      string `json:"client_ip"`       // クライアントIPアドレス
    AcctSessionID string `json:"acct_session_id"` // アカウンティングセッションID
    StartTime     int64  `json:"start_time"`      // セッション開始時刻（Unix秒）
    InputOctets   int64  `json:"input_octets"`    // 受信バイト数
    OutputOctets  int64  `json:"output_octets"`   // 送信バイト数
}

// NewSession は新しいSessionを生成する。
func NewSession(uuid, imsi, nasIP, clientIP, acctSessionID string, startTime int64) *Session {
    return &Session{
        UUID:          uuid,
        IMSI:          imsi,
        NasIP:         nasIP,
        ClientIP:      clientIP,
        AcctSessionID: acctSessionID,
        StartTime:     startTime,
        InputOctets:   0,
        OutputOctets:  0,
    }
}
```

#### EAPContext（EAP認証コンテキスト）

```go
// EAPContext はEAP認証コンテキストを表す。
// Valkeyキー: eap:{TraceID}
// TTL: 60秒
type EAPContext struct {
    TraceID              string `json:"trace_id"`               // トレース識別子
    IMSI                 string `json:"imsi"`                   // 加入者IMSI
    EAPType              uint8  `json:"eap_type"`               // EAPタイプ（23=AKA, 50=AKA'）
    Stage                Stage  `json:"stage"`                  // 認証ステージ（Stage型）
    RAND                 string `json:"rand"`                   // ランダム値（32文字16進数）
    AUTN                 string `json:"autn"`                   // 認証トークン（32文字16進数）
    XRES                 string `json:"xres"`                   // 期待される応答（16文字16進数）
    Kaut                 string `json:"kaut"`                   // 認証鍵
    MSK                  string `json:"msk"`                    // マスターセッションキー
    ResyncCount          int    `json:"resync_count"`           // 再同期試行回数
    PermanentIDRequested bool   `json:"permanent_id_requested"` // 永続ID要求フラグ
}

// NewEAPContext は新しいEAPContextを生成する。
func NewEAPContext(traceID, imsi string, eapType uint8) *EAPContext {
    return &EAPContext{
        TraceID:              traceID,
        IMSI:                 imsi,
        EAPType:              eapType,
        Stage:                StageNew,
        RAND:                 "",
        AUTN:                 "",
        XRES:                 "",
        Kaut:                 "",
        MSK:                  "",
        ResyncCount:          0,
        PermanentIDRequested: false,
    }
}
```

### 6.4 ポリシー関連構造体

**ファイル: `pkg/model/policy.go`**

```go
package model

import "encoding/json"

// Policy は加入者のアクセスポリシーを表す。
// Valkeyキー: policy:{IMSI}
type Policy struct {
    IMSI      string       `json:"imsi"`       // 加入者IMSI
    Default   string       `json:"default"`    // デフォルトアクション（"allow" or "deny"）
    RulesJSON string       `json:"rules_json"` // ルールのJSON文字列（Valkey保存用）
    Rules     []PolicyRule `json:"-"`          // パース済みルール（メモリ上のみ）
}

// PolicyRule はポリシールールを表す。
type PolicyRule struct {
    SSID    string `json:"ssid"`     // 対象SSID（ワイルドカード可）
    Action  string `json:"action"`   // アクション（"allow" or "deny"）
    TimeMin string `json:"time_min"` // 許可開始時刻（HH:MM形式、空で制限なし）
    TimeMax string `json:"time_max"` // 許可終了時刻（HH:MM形式、空で制限なし）
}

// NewPolicy は新しいPolicyを生成する。
func NewPolicy(imsi, defaultAction string) *Policy {
    return &Policy{
        IMSI:      imsi,
        Default:   defaultAction,
        RulesJSON: "[]",
        Rules:     []PolicyRule{},
    }
}

// ParseRules はRulesJSONをパースしてRulesに格納する。
func (p *Policy) ParseRules() error {
    if p.RulesJSON == "" || p.RulesJSON == "[]" {
        p.Rules = []PolicyRule{}
        return nil
    }
    return json.Unmarshal([]byte(p.RulesJSON), &p.Rules)
}

// EncodeRules はRulesをJSON文字列にエンコードしてRulesJSONに格納する。
func (p *Policy) EncodeRules() error {
    data, err := json.Marshal(p.Rules)
    if err != nil {
        return err
    }
    p.RulesJSON = string(data)
    return nil
}

// IsAllowByDefault はデフォルトアクションが許可かどうかを返す。
func (p *Policy) IsAllowByDefault() bool {
    return p.Default == "allow"
}
```

**PolicyRule JSONサンプル:**

```json
[
  {"ssid": "CORP-WIFI", "action": "allow", "time_min": "09:00", "time_max": "18:00"},
  {"ssid": "GUEST-WIFI", "action": "allow", "time_min": "", "time_max": ""},
  {"ssid": "*", "action": "deny", "time_min": "", "time_max": ""}
]
```

### 6.5 使用例

```go
// ストア層での変換例（apps/auth-server/internal/store/convert.go）
// model構造体にはredisタグがないため、ストア層でmap[string]string⇔構造体の変換を行う

func subscriberFromMap(imsi string, m map[string]string) *model.Subscriber {
    return model.NewSubscriber(
        imsi,
        m["ki"],
        m["opc"],
        m["amf"],
        m["sqn"],
        m["created_at"],
    )
}
```

---

## 7. pkg/httputil（HTTPユーティリティ）

### 7.1 責務

- RFC 7807 Problem Details形式のエラーレスポンス生成
- HTTPエラーハンドリングの共通化
- Ginフレームワークとの統合

### 7.2 RFC 7807 Problem Details

**ファイル: `pkg/httputil/problem.go`**

```go
package httputil

import (
    "encoding/json"
    "net/http"
)

// ContentType はRFC 7807で定義されたContent-Typeヘッダー値
const ContentType = "application/problem+json"

// ProblemDetail はRFC 7807準拠のエラーレスポンス構造体
type ProblemDetail struct {
    Type   string `json:"type"`             // エラータイプのURI（通常は"about:blank"）
    Title  string `json:"title"`            // エラータイトル
    Status int    `json:"status"`           // HTTPステータスコード
    Detail string `json:"detail,omitempty"` // 詳細説明
}

// NewProblemDetail は新しいProblemDetailを生成する
func NewProblemDetail(status int, title, detail string) *ProblemDetail

// JSON はProblemDetailをJSON形式にエンコードする
func (p *ProblemDetail) JSON() ([]byte, error)

// MustJSON はProblemDetailをJSON形式にエンコードする（エラー時パニック）
func (p *ProblemDetail) MustJSON() []byte
```

**標準HTTPエラーコンストラクタ一覧:**

| 関数名 | HTTPステータス | 用途 |
|--------|--------------|------|
| `BadRequest(detail)` | 400 | リクエスト形式不正 |
| `NotFound(detail)` | 404 | リソース不在 |
| `InternalServerError(detail)` | 500 | サーバー内部エラー |
| `NotImplemented(detail)` | 501 | 未実装機能 |
| `BadGateway(detail)` | 502 | バックエンド通信エラー |
| `ServiceUnavailable(detail)` | 503 | サービス一時停止 |

### 7.3 Ginフレームワーク統合

**ファイル: `pkg/httputil/gin.go`**

```go
package httputil

import "github.com/gin-gonic/gin"

// WriteError はProblemDetailをGinレスポンスとして書き込む。
// Content-Typeヘッダーに "application/problem+json" を設定する。
func WriteError(c *gin.Context, problem *ProblemDetail)

// AbortWithError はProblemDetailをGinレスポンスとして書き込み、リクエスト処理を中断する。
// Content-Typeヘッダーに "application/problem+json" を設定する。
func AbortWithError(c *gin.Context, problem *ProblemDetail)
```

### 7.4 使用例

```go
// Vector APIでのエラーハンドリング
func (h *VectorHandler) handleError(c *gin.Context, err error) {
    switch {
    case errors.Is(err, apperr.ErrIMSINotFound):
        httputil.WriteError(c, httputil.NotFound("IMSI not found"))
    case errors.Is(err, apperr.ErrValkeyConnection):
        httputil.WriteError(c, httputil.ServiceUnavailable("Database temporarily unavailable"))
    default:
        httputil.WriteError(c, httputil.InternalServerError("An unexpected error occurred"))
    }
}

// Vector Gatewayでのバックエンドエラーハンドリング
func (h *GatewayHandler) handleBackendError(c *gin.Context, err error) {
    if errors.Is(err, apperr.ErrBackendNotImplemented) {
        httputil.WriteError(c, httputil.NotImplemented("Requested backend is not implemented"))
        return
    }
    httputil.WriteError(c, httputil.BadGateway("Failed to communicate with internal API"))
}
```

---

## 8. パッケージ間依存関係

### 8.1 依存関係図

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           外部パッケージ                                 │
│  ┌─────────────────────┐  ┌─────────────────────┐                       │
│  │ github.com/redis/   │  │ github.com/gin-     │                       │
│  │ go-redis/v9         │  │ gonic/gin           │                       │
│  └──────────┬──────────┘  └──────────┬──────────┘                       │
│             │                        │                                  │
└─────────────┼────────────────────────┼──────────────────────────────────┘
              │                        │
              ▼                        ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                              pkg/                                       │
│                                                                         │
│  ┌─────────────┐     ┌─────────────┐     ┌─────────────┐               │
│  │   apperr    │     │   logging   │     │    model    │               │
│  │             │     │             │     │             │               │
│  │ (依存なし)  │     │ (依存なし)  │     │ (依存なし)  │               │
│  └─────────────┘     └─────────────┘     └─────────────┘               │
│                                                                         │
│  ┌─────────────┐     ┌─────────────┐                                   │
│  │   valkey    │     │  httputil   │                                   │
│  │             │     │             │                                   │
│  │ → go-redis  │     │ → gin (任意)│                                   │
│  └─────────────┘     └─────────────┘                                   │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
              │
              │ pkg は apps から参照される
              ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                              apps/                                      │
│                                                                         │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐   │
│  │ auth-server │  │ acct-server │  │   vector-   │  │ vector-api  │   │
│  │             │  │             │  │   gateway   │  │             │   │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘   │
│                                                                         │
│  ┌─────────────┐                                                       │
│  │  admin-tui  │                                                       │
│  └─────────────┘                                                       │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 8.2 依存ルール

#### 許可される依存

| From | To | 備考 |
|------|-----|------|
| apps/* | pkg/* | 全パッケージへの依存を許可 |
| pkg/valkey | go-redis/v9 | 外部パッケージへの依存（必須） |
| pkg/httputil | gin | Ginヘルパー関数使用時のみ |

#### 禁止される依存

| From | To | 理由 |
|------|-----|------|
| pkg/* | pkg/* | pkg内の相互依存禁止 |
| pkg/* | apps/* | 上位層への依存禁止 |
| pkg/apperr | 外部パッケージ | 最下層として依存なしを維持 |
| pkg/logging | 外部パッケージ | 標準ライブラリのみ使用 |
| pkg/model | 外部パッケージ | 標準ライブラリのみ使用 |

### 8.3 外部パッケージ依存一覧

| パッケージ | 外部依存 | 必要理由 |
|-----------|---------|---------|
| `pkg/apperr` | なし | エラー定義のみ |
| `pkg/valkey` | `github.com/redis/go-redis/v9` | Valkeyクライアント |
| `pkg/logging` | なし | 標準slogのみ使用 |
| `pkg/model` | なし | 構造体定義のみ（encoding/jsonは標準ライブラリ） |
| `pkg/httputil` | `github.com/gin-gonic/gin`（任意） | Ginヘルパー関数 |

> **注記:** `pkg/httputil` のGin依存は、Ginヘルパー関数（`WriteError`, `AbortWithError`）を使用する場合のみ必要。`ProblemDetail` 構造体自体はGinに依存しない。

---

## 9. 将来拡張

### 9.1 pkg配置検討中の機能

以下の機能はPoC期間中の状況に応じてpkg配置を検討する。

| 機能 | 現状 | 配置検討理由 | 判断時期 |
|------|------|-------------|---------|
| UUID生成ヘルパー | 各アプリで`google/uuid`直接利用 | 利用箇所が2コンポーネント（Auth, Acct） | 実装時 |
| Trace ID伝搬 | 各アプリで個別実装 | コンテキスト操作の標準化 | 実装時 |
| HTTPクライアント | Auth, Gatewayで個別実装 | Circuit Breaker設定が異なる | PoC完了後 |

### 9.2 PoC完了後の検討事項

| 項目 | 内容 | 優先度 |
|------|------|--------|
| テスト用モック | モック生成の共通化（mockgen連携） | 中 |
| メトリクス収集 | Prometheus対応の共通化 | 低 |
| 設定ローダー | envconfig共通ラッパー | 低 |
| バリデーション | IMSI/Hex形式検証の共通化 | 中 |

### 9.3 pkg拡張時の注意事項

新しいパッケージをpkgに追加する際は、以下を確認する。

1. **配置基準の確認:** セクション1.4の基準を満たすか
2. **依存関係の確認:** セクション8.2の禁止ルールに違反しないか
3. **ドキュメント更新:** 本ドキュメントのセクション2, 8を更新
4. **go.mod更新:** 外部依存が増える場合はgo.modを更新

---

## 改訂履歴

| 版数 | 日付 | 内容 |
|------|------|------|
| r1 | 2026-01-25 | 初版作成。pkg/apperr, pkg/valkey, pkg/logging, pkg/model, pkg/httputil の設計を定義。 |
| r2 | 2026-02-18 | 実装との整合: apperr/httputil ファイル分割反映、PolicyRule構造変更（SSID/Action/TimeMin/TimeMax）、model構造体をjsonタグのみに修正（redisタグ除去・ストア層変換方式）、Stage型（`type Stage string`）と8定数追加、全コンストラクタシグネチャを実装に合わせて更新、valkey DefaultOptions/TUIOptionsのデフォルト値明記、logging フィールド定数8種・nilガード・AuthLogFields追記、httputil ContentType定数・BadGateway/NotImplemented/ServiceUnavailable追記、関連ドキュメント版数更新。 |
| r3 | 2026-03-01 | 実装・現行ドキュメントとの整合: IMSIマスキング仕様をD-04 r17準拠に修正（先頭6桁+末尾1桁）、関連ドキュメント版数更新 |
