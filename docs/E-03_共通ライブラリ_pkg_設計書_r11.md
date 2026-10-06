# E-03 共通ライブラリ(pkg)設計書 (r11)

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
| D-02 Valkeyデータ設計仕様書 (r15) | Go構造体定義、ストア層変換方式 |
| D-04 ログ仕様設計書 (r31) | IMSIマスキング仕様（User-Nameのマスク規則を含む）、ログレベル設定（LOG_LEVEL）、`RADIUS_LIB_ERR` |
| D-06 エラーハンドリング詳細設計書 (r17) | エラー定義パターン |
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
│   ├── masking.go            # IMSI・User-Nameマスキング・Masker構造体
│   ├── fields.go             # フィールド定数・CommonFields・AuthLogFields
│   ├── level.go              # LOG_LEVEL文字列→slog.Level変換（ParseLevel）
│   └── radiuslib.go          # RADIUSライブラリのログのJSON化（NewRADIUSLibraryLogger・EventRADIUSLibError）
├── model/                    # 共通データ構造体
│   ├── subscriber.go         # Subscriber構造体・NewSubscriber
│   ├── client.go             # RadiusClient構造体・NewRadiusClient
│   ├── session.go            # Session・EAPContext・Stage型・NewSession・NewEAPContext
│   └── policy.go             # Policy・PolicyRule構造体・NewPolicy・Clone
├── validation/               # マスタデータの入力検証（§8）
│   ├── rules.go              # 正規表現・上限値
│   ├── subscriber.go         # 加入者の検証・正規化
│   ├── client.go             # RADIUSクライアントの検証・正規化
│   └── policy.go             # 認可ポリシーの検証・正規化
├── masterdata/               # マスタデータの Valkey アクセス（§9）
│   ├── keys.go               # キー定義（sub: / client: / policy:）
│   ├── hash.go               # 原子的な作成・変更の Lua スクリプト
│   ├── subscriber.go         # SubscriberStore・SubscriberPatch
│   ├── client.go             # ClientStore
│   └── policy.go             # PolicyStore
└── httputil/                 # HTTPユーティリティ
    ├── problem.go            # ProblemDetail構造体・コンストラクタ・ContentType定数
    └── gin.go                # Ginフレームワーク統合（WriteError, AbortWithError）
```

### 2.2 パッケージ一覧

| パッケージ | 責務 | 主要な型・関数 |
|-----------|------|---------------|
| `apperr` | 共通エラー定義 | センチネルエラー、カスタムエラー型（ValidationError, BackendError, ValkeyError, EAPIdentityError） |
| `valkey` | Valkeyクライアント初期化 | `NewClient()`, `Options`, `DefaultOptions()`, `TUIOptions()`, `BuildAddr()` |
| `logging` | ログユーティリティ | `MaskIMSI()`, `MaskUserName()`, `Masker`, `CommonFields`, `AuthLogFields()`, `ParseLevel()`, `NewRADIUSLibraryLogger()`, `EventRADIUSLibError`, フィールド定数8種 |
| `model` | 共通データ構造体 | `Subscriber`, `RadiusClient`, `Session`, `EAPContext`, `Policy`（`Clone` を含む）, `PolicyRule`, `Stage` |
| `validation` | マスタデータの入力検証・正規化 | `ValidateSubscriber()`, `ValidateClient()`, `ValidatePolicy()`, `Normalize*Input()`, `*ValidationError` |
| `masterdata` | マスタデータの Valkey アクセス | `SubscriberStore`, `ClientStore`, `PolicyStore`, `SubscriberPatch`, `Err*NotFound` / `Err*Exists`, `SubscriberKey()` 等 |
| `httputil` | HTTPユーティリティ | `ProblemDetail`, `ContentType`, `WriteError()`, `AbortWithError()` |

### 2.3 利用コンポーネント対応表

| パッケージ | Auth Server | Acct Server | Vector Gateway | Vector API | Admin TUI |
|-----------|:-----------:|:-----------:|:--------------:|:----------:|:---------:|
| `apperr` | ◎ | ◎ | ◎ | ◎ | ○ |
| `valkey` | ◎ | ◎ | - | ◎ | ◎ |
| `logging` | ◎ | ◎ | ◎ | ◎ | - |
| `model` | ◎ | ◎ | - | ◎ | ◎ |
| `httputil` | - | - | ◎ | ◎ | - |
| `validation` | - | - | - | - | ◎ |
| `masterdata` | - | - | - | - | ◎ |

**凡例:** ◎=必須, ○=任意, -=不使用

> **注記:** `validation` / `masterdata` は現時点では Admin TUI だけが使うが、Provisioning API（D-13。実装予定）でも使うため、§1.4 の「2つ以上のアプリで使用」を見込んで先に pkg に置いた。

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

- IMSIマスキング処理の提供（User-Name（EAP Identity）に含まれるIMSIのマスキングを含む）
- 共通ログフィールドの定義
- マスキング設定の一元管理
- ログレベル設定（環境変数 `LOG_LEVEL`）の文字列から `slog.Level` への変換
- RADIUSライブラリ（`layeh.com/radius`）が出すエラーのログを、JSONのslog（`event_id`=`RADIUS_LIB_ERR`）に流す `*log.Logger` の提供（§5.8）

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
// マスキング仕様（D-04 §4.4.2準拠）:
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

// MaskUserName はEAP Identity形式のユーザー名（User-Name属性）をマスキングする
//
// マスキング仕様（D-04 §4.4.2準拠）:
//   - "@" より前（ローカル部）だけを対象とし、realm（"@" 以降）はそのまま残す
//   - 種別1文字＋IMSI 15桁（16桁の数字）: 種別文字を残し、IMSI部分に MaskIMSI を適用
//   - IMSI 15桁のみ: MaskIMSI を適用
//   - それ以外（仮名・再認証ID・不正形式）: MaskPartial(local, 7, 1, '*')
//     （先頭7文字と末尾1文字を保持。7+1文字以下はそのまま）
//
// 例: 0440101234567890@realm  → 0440101********0@realm
//     440101234567890         → 440101********0
//     2some-pseudonym@example → 2some-p*******m@example
//
// enabled=false の場合はマスキングせずそのまま返す
func MaskUserName(userName string, enabled bool) string

// Masker はマスキング設定を保持する構造体
type Masker struct {
    enabled bool
}

func NewMasker(enabled bool) *Masker
func (m *Masker) IMSI(imsi string) string         // MaskIMSI(imsi, m.enabled)
func (m *Masker) UserName(userName string) string // MaskUserName(userName, m.enabled)
func (m *Masker) IsEnabled() bool
```

**利用箇所（User-Name）:** Auth Serverの `EAP_UNSUPPORTED_TYPE` / `EAP_IDENTITY_INVALID` の `user_name` 属性、Acct Serverの課金ログでUser-NameからIMSIを抽出できない場合の `imsi` 属性（D-04 §4.5）。

### 5.6 ログレベル変換

**ファイル: `pkg/logging/level.go`**

```go
// ParseLevel はLOG_LEVEL環境変数の値（DEBUG / INFO / WARN / ERROR、大文字小文字を区別しない）を
// slog.Levelに変換する。未知の値や空文字はINFOとして扱う。
//
//   - 前後の空白を除去してから大文字に変換して比較する
//   - "DEBUG" → slog.LevelDebug
//   - "WARN" / "WARNING" → slog.LevelWarn
//   - "ERROR" → slog.LevelError
//   - それ以外（"INFO"、空文字、未知の値） → slog.LevelInfo
func ParseLevel(s string) slog.Level
```

**利用箇所:** Auth Server / Acct Server / Vector Gateway / Vector API のロガー初期化（`apps/auth-server/main.go`、`apps/acct-server/main.go`、`apps/vector-gateway/main.go` と `apps/vector-api/main.go` の `initLogger`。いずれも `slog.HandlerOptions.Level` に `logging.ParseLevel(cfg.LogLevel)` を渡す）。このため4コンポーネントとも `LOG_LEVEL` の解釈（大文字小文字を区別しない、前後の空白を無視、`WARNING` も `WARN`、未知の値は `INFO`）は同じである（D-04 §4.6）。

### 5.7 使用例

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

// User-Name（EAP Identity）のマスキング（realmは残る）
slog.Warn("非対応のIdentity種別",
    "event_id", "EAP_UNSUPPORTED_TYPE",
    "trace_id", traceID,
    "user_name", logging.MaskUserName(userName, cfg.LogMaskIMSI), // 1440101********0@realm
)

// ロガー初期化（LOG_LEVEL の値で出力レベルを設定。既定 INFO）
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: logging.ParseLevel(cfg.LogLevel),
})).With("app", "auth-server")
slog.SetDefault(logger)
```

### 5.8 RADIUSライブラリのログ

**ファイル: `pkg/logging/radiuslib.go`**

```go
// EventRADIUSLibError は RADIUS ライブラリ（layeh.com/radius）が出すエラーの event_id。
const EventRADIUSLibError = "RADIUS_LIB_ERR"

// NewRADIUSLibraryLogger は layeh.com/radius の PacketServer.ErrorLog に設定する *log.Logger を返す。
// ライブラリは受信処理のエラー（パケットの解析失敗など）を標準の log パッケージで素のテキストとして出すため、
// JSON の slog（event_id: RADIUS_LIB_ERR）に流して、他のログと同じ形式で扱えるようにする。
//
// 共有シークレットが決まらずにパケットを捨てた場合（"empty secret returned from secret source"）は、
// アプリの SecretSource が送信元IP付きでログ（RADIUS_NO_SECRET 等）を出しているため、DEBUG にとどめる。
func NewRADIUSLibraryLogger() *log.Logger
```

**目的:** `layeh.com/radius` の `PacketServer` は、受信処理のエラー（パケットのデコード失敗、受信エラー、共有シークレットが決まらずに捨てたこと等）を `ErrorLog`（`*log.Logger`）に書く。`ErrorLog` が `nil` だと標準の `log` パッケージで標準エラーにテキスト1行を出すため、`event_id` がなく、JSONのログとして集計・検索できない。`NewRADIUSLibraryLogger` が返す Logger を `ErrorLog` に設定すると、これらを他のログと同じJSON形式で出力できる。

**動作:**

- ライブラリが書き込んだ1行（前後の空白・改行を除去）を、`slog.Log` でデフォルトのロガー（`slog.Default()`）に出力する。msg は `RADIUSライブラリのエラー`、属性は `event_id`=`RADIUS_LIB_ERR`（`EventRADIUSLibError`）と `error`=ライブラリのメッセージ（例: `radius: unable to parse packet: radius: packet not at least 20 bytes long`）。
- レベルは WARN。ただし、メッセージに `empty secret returned from secret source` を含む行は DEBUG とする（同じパケットでアプリの SecretSource が `RADIUS_NO_SECRET` 等を送信元の情報付きで出しているため。`LOG_LEVEL=DEBUG` のときだけ出る）。
- ライブラリのメッセージに送信元IPは含まれないため、`src_ip` は出力しない。`app` などはデフォルトのロガーに付けた属性がそのまま付く。
- Logger の prefix・flag は空（`log.New(w, "", 0)`）であり、日時は slog の `time` だけになる。
- 標準ライブラリ（`log`, `log/slog`, `strings`, `context`）のみを使い、`layeh.com/radius` には依存しない（`*log.Logger` を返すだけ。§8.3）。

**使い方:** Auth Server / Acct Server の `internal/server/server.go`（`NewServer`）で `PacketServer` に設定する（D-09 §4.3、D-10 §10.3）。`main.go` で `slog.SetDefault` した後に `NewServer` を呼ぶため、出力は `app` 付きのJSONになる。

```go
ps := &radius.PacketServer{
    Addr:         addr,
    SecretSource: secretSource,
    Handler:      handler,
    // パケットの認証はハンドラーで検証する（D-09 §4.3）
    InsecureSkipVerify: true,
    // ライブラリが出すエラー（パケットの解析失敗など）を JSON の slog に流す
    ErrorLog: logging.NewRADIUSLibraryLogger(),
}
```

出力例（WARN。形の壊れたパケットを受信したとき）:

```json
{"time":"2026-10-04T21:00:00.000000000+09:00","level":"WARN","msg":"RADIUSライブラリのエラー","app":"acct-server","event_id":"RADIUS_LIB_ERR","error":"radius: unable to parse packet: radius: packet not at least 20 bytes long"}
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
    NasIP         string `json:"nas_ip"`          // NAS IPアドレス（パケットの送信元IP。プロキシ経由ではプロキシのIP）
    NasIdentifier string `json:"nas_identifier"`  // NAS-Identifier（プロキシ経由でもNASを識別できる）
    ClientIP      string `json:"client_ip"`       // クライアントIPアドレス
    AcctSessionID string `json:"acct_session_id"` // アカウンティングセッションID
    StartTime     int64  `json:"start_time"`      // セッション開始時刻（Unix秒）
    InputOctets   int64  `json:"input_octets"`    // 受信バイト数
    OutputOctets  int64  `json:"output_octets"`   // 送信バイト数
}

// NewSession は新しいSessionを生成する（NasIdentifier は引数に含めないため、必要なら生成後に設定する）。
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
    NasID          string   `json:"nas_id"`                    // NAS識別子（"*" 単独で任意のNASに一致。それ以外は完全一致）
    AllowedSSIDs   []string `json:"allowed_ssids"`             // 許可SSIDリスト
    VlanID         string   `json:"vlan_id,omitempty"`         // VLAN ID（空文字は未設定）
    SessionTimeout int      `json:"session_timeout,omitempty"` // セッションタイムアウト秒（0は未設定）
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

// Clone はポリシーのディープコピーを作成する（Admin TUI の編集画面で使う）。
func (p *Policy) Clone() *Policy
```

**PolicyRule JSONサンプル:**

```json
[
  {"nas_id": "AP-OFFICE-01", "allowed_ssids": ["CORP-WIFI", "GUEST-WIFI"], "vlan_id": "100", "session_timeout": 3600},
  {"nas_id": "*", "allowed_ssids": ["GUEST-WIFI"], "vlan_id": "300"}
]
```

> ルールの評価（`nas_id` / `allowed_ssids` の一致判定、評価順、default）は pkg/model ではなく Auth Server（`internal/policy/evaluator.go`）が行う。評価仕様は D-02 セクション2.C を参照。

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

## 8. pkg/validation（マスタデータの入力検証）

### 8.1 責務

- 加入者・RADIUSクライアント・認可ポリシーの入力値の検証と正規化を提供する
- Admin TUI（画面の保存時と CSV インポート）と Provisioning API（D-13）が同じ規則で検証できるようにする（2026-10-07 に Admin TUI の `internal/validation` から移動）
- 検証規則の詳細（文字種・長さ・範囲、エラーメッセージ）は D-05 §5 を参照

### 8.2 主要な型・関数

**ファイル: `pkg/validation/rules.go` / `subscriber.go` / `client.go` / `policy.go`**

| 対象 | 検証 | 正規化 | エラー型 |
|------|------|--------|---------|
| 加入者 | `ValidateIMSI` / `ValidateKi` / `ValidateOPc` / `ValidateAMF` / `ValidateSQN`、まとめて `ValidateSubscriber(*SubscriberInput) []error` | `NormalizeSubscriberInput`（前後の空白を除去し、16進を大文字に） | `SubscriberValidationError`（`Field`, `Message`） |
| RADIUSクライアント | `ValidateIPv4` / `ValidateSecret` / `ValidateClientName` / `ValidateVendor`、まとめて `ValidateClient(*ClientInput) []error` | `NormalizeClientInput` | `ClientValidationError` |
| 認可ポリシー | `ValidateDefaultAction` / `ValidateNasID` / `ValidateSSID` / `ValidateAllowedSSIDs` / `ValidateVlanID` / `ValidateSessionTimeout`、`ValidatePolicyRule(*model.PolicyRule)`、まとめて `ValidatePolicy(*PolicyInput) []error` | `NormalizePolicyInput`（空白の除去、`default` を小文字に） | `PolicyValidationError`（ルールの項目は `Rules[i].NasID` 等） |

- 正規表現（`IMSIPattern` 等）と上限値（`MaxSecretLength` 等）は `rules.go` に定数として公開する
- `pkg/model` の `PolicyRule` に依存する（§10.2）

### 8.3 使用例

```go
input := validation.NormalizeSubscriberInput(&validation.SubscriberInput{
    IMSI: imsi, Ki: ki, OPc: opc, AMF: amf, SQN: sqn,
})
if errs := validation.ValidateSubscriber(input); len(errs) > 0 {
    return errs[0] // Admin TUI は最初の1件を表示する
}
```

---

## 9. pkg/masterdata（マスタデータの Valkey アクセス）

### 9.1 責務

- 加入者（`sub:{IMSI}`）・RADIUSクライアント（`client:{IP}`）・認可ポリシー（`policy:{IMSI}`）の Valkey の読み書きを提供する（キーとフィールドの形式は D-02）
- Admin TUI と Provisioning API（D-13）が同じ処理（Lua スクリプトを含む）で書き込むようにする（2026-10-07 に Admin TUI の `internal/store` から移動）
- セッション（`sess:`）・統計のストアは Admin TUI の `internal/store` に残す（Admin TUI だけが使うため）

### 9.2 主要な型・関数

| 型 | 主なメソッド | 備考 |
|----|------------|------|
| `SubscriberStore` | `Get`, `Create`, `Update`, `UpdateWithSQN`, `Patch`, `Delete`, `List`, `Count`, `Exists`, `BulkCreate` | `Update` は SQN を書き換えない。`UpdateWithSQN` は編集開始時の SQN との比較・置き換え（D-02 §2.A）。`Patch` は `SubscriberPatch` の nil でない項目だけを書き換える（SQN を指定しなければ触れない。D-13 §3.1） |
| `ClientStore` | `Get`, `Create`, `Update`, `Delete`, `List`, `Count`, `Exists`, `BulkCreate` | |
| `PolicyStore` | `Get`, `Create`, `Update`, `Upsert`, `Put`, `Delete`, `List`, `Count`, `Exists`, `BulkCreate`, `GetIMSIsWithPolicy` | `Put` は作成したか（存在しなかったか）を返す（D-13 §3.3 の PUT の 201 / 200 用） |
| キー | `SubscriberKey`, `ClientKey`, `PolicyKey`、`PrefixSubscriber` 等 | |

**センチネルエラー:**

| エラー | 条件 |
|-------|------|
| `ErrSubscriberNotFound` / `ErrClientNotFound` / `ErrPolicyNotFound` | 取得・変更・削除の対象が存在しない |
| `ErrSubscriberExists` / `ErrClientExists` / `ErrPolicyExists` | 作成の対象が既に存在する（メッセージは `subscriber already exists` 等。Admin TUI の表示は従来と同じ） |
| `ErrSQNChanged` | `UpdateWithSQN` で、SQN が編集開始時の値から変わっていた |

### 9.3 原子的な書き込み

作成（`Create`）と変更（`Update`、`Patch`）は、存在確認と書き込みを1つの Lua スクリプトで行う（`hash.go`）。Admin TUI と Provisioning API を同時に使った場合（D-13 §2.3）にも、次を保証する。

| 操作 | 保証 |
|------|------|
| 作成 | 同じキーを同時に作成しても成功するのは1つだけで、後の方は `Err*Exists` になり既存の値を上書きしない |
| 変更 | 途中で削除されたキーを、一部のフィールドだけで作り直さない（`Err*NotFound`） |
| 一括作成（`BulkCreate`）・`Upsert` / `Put` | 既存の値を上書きする（CSV インポート、PUT の仕様） |

> **注記:** 2026-10-07 より前の Admin TUI の作成・変更は「存在確認 → 書き込み」の2回の操作で行っており、同時に作成すると後の方が上書きしていた。

### 9.4 使用例

```go
subs := masterdata.NewSubscriberStore(client)
if err := subs.Create(ctx, sub); errors.Is(err, masterdata.ErrSubscriberExists) {
    // 409 等
}
amf := "B9B9"
err := subs.Patch(ctx, imsi, &masterdata.SubscriberPatch{AMF: &amf}) // SQN には触れない
```

---

## 10. パッケージ間依存関係

### 10.1 依存関係図

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
│  ┌─────────────┐     ┌─────────────┐                                   │
│  │ masterdata  │     │ validation  │                                   │
│  │ → go-redis  │     │             │                                   │
│  │ → model     │     │ → model     │                                   │
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

### 10.2 依存ルール

#### 許可される依存

| From | To | 備考 |
|------|-----|------|
| apps/* | pkg/* | 全パッケージへの依存を許可 |
| pkg/valkey | go-redis/v9 | 外部パッケージへの依存（必須） |
| pkg/httputil | gin | Ginヘルパー関数使用時のみ |
| pkg/masterdata | go-redis/v9 | Valkey の読み書き |
| pkg/masterdata, pkg/validation | pkg/model | 共通データ構造体（`pkg/model` は依存なしの最下層のため、循環しない） |

#### 禁止される依存

| From | To | 理由 |
|------|-----|------|
| pkg/* | pkg/*（`pkg/model` を除く） | pkg内の相互依存禁止。例外として、依存を持たない `pkg/model` への依存だけを許可する |
| pkg/* | apps/* | 上位層への依存禁止 |
| pkg/apperr | 外部パッケージ | 最下層として依存なしを維持 |
| pkg/logging | 外部パッケージ | 標準ライブラリのみ使用 |
| pkg/model | 外部パッケージ | 標準ライブラリのみ使用 |

### 10.3 外部パッケージ依存一覧

| パッケージ | 外部依存 | 必要理由 |
|-----------|---------|---------|
| `pkg/apperr` | なし | エラー定義のみ |
| `pkg/valkey` | `github.com/redis/go-redis/v9` | Valkeyクライアント |
| `pkg/logging` | なし | 標準ライブラリ（`log/slog`、`log` 等）のみ使用（`NewRADIUSLibraryLogger` も `*log.Logger` を返すだけで `layeh.com/radius` には依存しない） |
| `pkg/model` | なし | 構造体定義のみ（encoding/jsonは標準ライブラリ） |
| `pkg/httputil` | `github.com/gin-gonic/gin`（任意） | Ginヘルパー関数 |
| `pkg/validation` | なし | 標準ライブラリ（`regexp` 等）と `pkg/model` |
| `pkg/masterdata` | `github.com/redis/go-redis/v9` | Valkey の読み書き（Lua スクリプト）。`pkg/model` |

> **注記:** `pkg/httputil` のGin依存は、Ginヘルパー関数（`WriteError`, `AbortWithError`）を使用する場合のみ必要。`ProblemDetail` 構造体自体はGinに依存しない。

---

## 11. 将来拡張

### 11.1 pkg配置検討中の機能

以下の機能はPoC期間中の状況に応じてpkg配置を検討する。

| 機能 | 現状 | 配置検討理由 | 判断時期 |
|------|------|-------------|---------|
| UUID生成ヘルパー | 各アプリで`google/uuid`直接利用 | 利用箇所が2コンポーネント（Auth, Acct） | 実装時 |
| Trace ID伝搬 | 各アプリで個別実装 | コンテキスト操作の標準化 | 実装時 |
| HTTPクライアント | Auth, Gatewayで個別実装 | Circuit Breaker設定が異なる | PoC完了後 |

### 11.2 PoC完了後の検討事項

| 項目 | 内容 | 優先度 |
|------|------|--------|
| テスト用モック | モック生成の共通化（mockgen連携） | 中 |
| メトリクス収集 | Prometheus対応の共通化 | 低 |
| 設定ローダー | envconfig共通ラッパー | 低 |

### 11.3 pkg拡張時の注意事項

新しいパッケージをpkgに追加する際は、以下を確認する。

1. **配置基準の確認:** セクション1.4の基準を満たすか
2. **依存関係の確認:** セクション10.2の禁止ルールに違反しないか
3. **ドキュメント更新:** 本ドキュメントのセクション2, 10を更新
4. **go.mod更新:** 外部依存が増える場合はgo.modを更新

---

## 改訂履歴

| 版数 | 日付 | 内容 |
|------|------|------|
| r1 | 2026-01-25 | 初版作成。pkg/apperr, pkg/valkey, pkg/logging, pkg/model, pkg/httputil の設計を定義。 |
| r2 | 2026-02-18 | 実装との整合: apperr/httputil ファイル分割反映、PolicyRule構造変更（SSID/Action/TimeMin/TimeMax）、model構造体をjsonタグのみに修正（redisタグ除去・ストア層変換方式）、Stage型（`type Stage string`）と8定数追加、全コンストラクタシグネチャを実装に合わせて更新、valkey DefaultOptions/TUIOptionsのデフォルト値明記、logging フィールド定数8種・nilガード・AuthLogFields追記、httputil ContentType定数・BadGateway/NotImplemented/ServiceUnavailable追記、関連ドキュメント版数更新。 |
| r3 | 2026-03-01 | 実装・現行ドキュメントとの整合: IMSIマスキング仕様をD-04 r17準拠に修正（先頭6桁+末尾1桁）、関連ドキュメント版数更新 |
| r4 | 2026-10-04 | ログのIMSIマスク漏れ修正に伴う pkg/logging の公開API追加の反映: §5.5に `MaskUserName()`（User-Name（EAP Identity）の "@" より前をマスクし realm を残す）と `Masker.UserName()` を追加し利用箇所を追記、§5.6に使用例を追加、§2.1 / §2.2 / §5.1を更新。§1.3関連ドキュメント参照版数更新（D-04 r17→r20） |
| r5 | 2026-10-04 | ポリシーの `nas_id` で `"*"` を任意の NAS に一致させた Auth Server の実装修正に伴う pkg/model のコメント更新の反映: §6.4 の `PolicyRule` を実装（`NasID` / `AllowedSSIDs` / `VlanID` / `SessionTimeout`、`NasID` のコメント「`"*"` 単独で任意のNASに一致。それ以外は完全一致」）に合わせて修正（r2 で記載した `SSID` / `Action` / `TimeMin` / `TimeMax` は実装に存在しないため削除）。PolicyRule JSONサンプルを実装の形式に修正し、評価仕様は D-02 セクション2.C を参照する旨を追記。関連ドキュメントの D-02 参照版数を更新（r11→r15） |
| r6 | 2026-10-04 | auth-server の LOG_LEVEL 対応に伴う pkg/logging の公開API追加の反映: §5.6 ログレベル変換を新設し `ParseLevel()`（`pkg/logging/level.go`。DEBUG / INFO / WARN（WARNING）/ ERROR を大文字小文字を区別せず変換、前後の空白を除去、未知の値・空文字は INFO）と利用箇所（Auth Server のロガー初期化。Vector Gateway / Vector API は未使用）を追加、旧 §5.6 使用例を §5.7 に繰り下げてロガー初期化の例を追加、§2.1 ディレクトリ構造に `level.go`、§2.2 パッケージ一覧・§5.1 責務に `ParseLevel()` / ログレベル変換を追加。§1.3 参照版数更新（D-04 r20→r23） |
| r7 | 2026-10-04 | vector-api / vector-gateway のログレベル変換を `pkg/logging.ParseLevel` に統一した実装修正の反映: §5.6 の利用箇所に Vector Gateway / Vector API（各 `main.go` の `initLogger`）を追加し、「Vector Gateway / Vector API は独自に変換しており本関数を使っていない」旨の記述を削除（3コンポーネントで `LOG_LEVEL` の解釈が同じになった。`WARNING` も `WARN`） |
| r8 | 2026-10-04 | acct-server の LOG_LEVEL 対応の実装修正の反映: §5.6 の `ParseLevel` の利用箇所に Acct Server（`apps/acct-server/main.go` のロガー初期化）を追加して4コンポーネントとし、「Acct Server は LOG_LEVEL に対応しておらず使っていない」を削除。§1.3 関連ドキュメントの D-04 の版数を r25 に更新 |
| r9 | 2026-10-04 | RADIUSライブラリのログをJSONにした実装修正（`pkg/logging/radiuslib.go` 新設）の反映: §5.8 RADIUSライブラリのログを新設し、`NewRADIUSLibraryLogger()`（`layeh.com/radius` の `PacketServer.ErrorLog` に設定する `*log.Logger`。ライブラリの1行を msg `RADIUSライブラリのエラー`・`event_id`=`RADIUS_LIB_ERR`・`error` で slog に出力、WARN（`empty secret returned from secret source` を含む行は DEBUG）、`src_ip` なし）と定数 `EventRADIUSLibError` の目的・動作・使い方を記載。§2.1 / §2.2 / §5.1 に追加し、§8.3 に `layeh.com/radius` に依存しない旨を追記。§1.3 参照版数更新（D-04 r25→r31、D-06 r6→r17） |
| r10 | 2026-10-06 | `model.Session` に `NasIdentifier`（`json:"nas_identifier"`。Valkey の `sess:{UUID}` の `nas_identifier`。D-02 r20）を追加。radsecproxy 等のプロキシ経由では `NasIP` がプロキシのIPになり NAS を区別できないため。`NewSession` の引数は変えない |
| r11 | 2026-10-07 | Admin TUI の加入者・RADIUSクライアント・認可ポリシーの store と validation を pkg に移した実装修正（Provisioning API（D-13）と共通で使うため）の反映: §8 `pkg/validation`、§9 `pkg/masterdata`（センチネルエラー `Err*Exists` の追加、作成・変更を Lua スクリプトで原子的に、`SubscriberStore.Patch`、`PolicyStore.Put` / `Count`）を新設し、旧 §8 / §9 を §10 / §11 に繰り下げ。§2.1 / §2.2 / §2.3、§6.4（`Policy.Clone`。Admin TUI の `internal/model` を廃止して `pkg/model` に統合）、§10.1〜§10.3 を更新し、§10.2 の依存ルールに `pkg/model` への依存だけを許可する例外を追加。§11.2 から実施済みの「バリデーションの共通化」を削除 |
