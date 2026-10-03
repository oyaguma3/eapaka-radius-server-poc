# D-12 Vector Gateway 詳細設計書 (r5)

## 1. 概要

### 1.1 目的

本ドキュメントは、EAP-AKA RADIUS PoC環境における「Vector Gateway」の詳細設計を定義する。Vector Gatewayは、Auth ServerとVector API（および外部の認証ベクター払い出しサーバー）の間に位置する中間ノードであり、PLMNベースのルーティングと外部API連携機能を提供する。外部API連携としては、接続方式ID `01`（aka-only-server）を実装済みである。

**主な責務:**

- PLMNベースのバックエンドルーティング
- 内部Vector APIへのリクエスト転送（接続方式ID `00`）
- 外部の aka-only-server への接続とIF変換（接続方式ID `01`、mTLS）
- 将来的な外部API連携の拡張ポイント（接続方式ID `02`〜`99`）
- X-Trace-IDによるトレーサビリティ確保

### 1.2 背景

現在のアーキテクチャでは、Auth ServerからVector APIに対して認証ベクターを取得する構成となっている。しかし、以下のユースケースに対応するため、外部API連携機能の追加を検討する。

| ユースケース | 説明 |
|-------------|------|
| パートナーMNO連携 | 他事業者のHSS/AuCから認証ベクターを取得 |
| クラウドベース認証 | 外部クラウドサービスとの連携 |
| ハイブリッド運用 | 一部加入者はローカル認証、一部は外部委託 |

r5 では、ハイブリッド運用の具体例として、外部の aka-only-server（4.5節）から認証ベクターを取得する接続方式 `01` を追加した。

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
| 接続方式 `02`〜`99` の実装 | 接続先未定 | 接続先確定時に順次実装（それまでは501） |
| デフォルトバックエンドの切り替え設定 | 未一致PLMNは `00` で十分 | 必要になった時点で検討 |

> **注記:** SORACOM Endorse API等のチャレンジ・レスポンス分離型（RAND/AUTNのみ提供、RES検証はAPI側）は、CK/IKが提供されないためEAP-AKAのAT_MAC計算ができず、端末がClient-Errorと判定する問題があり対応を中止した。
>
> 接続方式 `01`（aka-only-server）で実装しない事項は 4.5.6 を参照。

---

## 2. アーキテクチャ設計

### 2.1 全体構成

```
┌─────────────────────────────────────────────────────────────────────────┐
│ Docker Compose Network                                                  │
│                                                                         │
│  ┌─────────────┐      ┌─────────────────┐      ┌─────────────┐         │
│  │ Auth Server │─────►│ Vector Gateway  │─────►│ Vector API  │         │
│  └─────────────┘      │                 │ ID:00│ (内部)      │         │
│                       └────────┬────────┘      └─────────────┘         │
│                                │                                        │
└────────────────────────────────┼────────────────────────────────────────┘
                                 │ ID:01  HTTPS + mTLS（既定）/ 平文HTTP（同一ホスト限定）
                                 │ 同一ホスト: 共有ネットワーク aka-av 経由
                                 ▼
                       ┌──────────────────────────┐
                       │ aka-only-server（外部）  │
                       │ Nudm_UEAU GenerateAv     │
                       │ ※任意。URL設定時のみ有効 │
                       └──────────────────────────┘
                       （ID:02〜99 は将来実装。指定すると501）
```

### 2.2 通信フロー

#### PLMNマップ未設定時（既定）: 全リクエストをVector APIへ転送

```
Auth Server → Vector Gateway → Vector API → Valkey
                                  ↓
              ← RAND/AUTN/XRES/CK/IK ←
```

#### PLMNベースルーティング

```
Auth Server → Vector Gateway ─┬→ Vector API      (PLMN未一致 or ID:00、passthroughモード)
                              │
                              ├→ aka-only-server (ID:01 の PLMN、URL設定時)
                              │
                              └→ 501 Not Implemented (ID:02〜99、または URL未設定で ID:01)
```

### 2.3 Vector Gatewayの責務

| 責務 | 内部（ID:00） | aka-only-server（ID:01） | 将来（ID:02〜） |
|------|--------------|-------------------------|----------------|
| **ルーティング** | PLMN未一致・passthroughの既定先 | PLMNマップで `01` を指定したPLMN | PLMNマップで指定 |
| **プロトコル変換** | なし（パススルー） | 内部IF ⇔ GenerateAv（4.5.2〜4.5.3） | 外部API形式への変換 |
| **認証情報付与** | なし | mTLSのクライアント証明書で識別（API Key/Tokenは使わない） | 外部API呼び出し時のAPI Key/Token付与等 |
| **エラー変換** | 4xxはそのまま伝搬 | aka-only-serverのProblemDetailsを内部形式に変換（4.5.4） | 外部APIエラーを内部形式に変換 |
| **トレーサビリティ** | X-Trace-IDの伝搬 | 下記参照 | 下記参照 |

#### トレーサビリティの責務範囲

| バックエンド | 責務範囲 | 説明 |
|-------------|---------|------|
| **内部（Vector API）** | エンドツーエンド保証 | X-Trace-IDを伝搬し、Auth Server〜Vector APIまで追跡可能 |
| **外部API（aka-only-server を含む）** | Vector Gatewayまで保証 | 外部APIへのヘッダ付与はベストエフォート（外部側の対応は期待しない）。aka-only-server は X-Trace-ID を参照・記録しない |

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
| passthroughモード | PLMNマップに関係なく常にVector API（ID:00） |

### 3.2 接続方式ID定義

| ID | 名称 | 説明 | 実装 |
|----|------|------|---------|
| `00` | Vector API | 内部Vector APIへ転送 | ○（常に有効） |
| `01` | aka-only-server | 外部の aka-only-server（Nudm_UEAU GenerateAvベースAPI）から取得（4.5節） | ○（`VECTOR_GATEWAY_AKAONLY_URL` 設定時のみ有効。未設定時は501） |
| `02`〜`99` | 外部API | 外部API接続方式（将来実装） | × (501エラー) |

### 3.3 PLMN形式

| 項目 | 仕様 |
|------|------|
| 形式 | MCC + MNC 結合（ハイフンなし） |
| 長さ | 5桁（MNC 2桁）または 6桁（MNC 3桁） |
| 例 | `44010`（日本 docomo）、`310260`（米国 T-Mobile） |
| 照合方式 | 固定長照合（環境変数指定形式に完全一致） |

### 3.4 IMSIからのPLMN抽出

IMSIの先頭6桁（MCC+MNC 3桁）を優先し、次に先頭5桁（MCC+MNC 2桁）の順で照合する。

```go
// internal/router/router.go

// extractPLMNs はIMSIからPLMN候補を抽出する。
// IMSIの先頭6桁候補と5桁候補を返す（6桁優先）。
func extractPLMNs(imsi string) []string {
    var candidates []string
    if len(imsi) >= 6 {
        candidates = append(candidates, imsi[:6])
    }
    if len(imsi) >= 5 {
        candidates = append(candidates, imsi[:5])
    }
    return candidates
}
```

### 3.5 ルーティングロジック

接続方式IDからバックエンドへの解決はバックエンドレジストリ（4.4節）が行う。レジストリに登録されていないID（`02`〜`99`、および URL 未設定時の `01`）は `BackendNotImplementedError` となり、ハンドラーで501に変換される。

```go
// internal/router/router.go

// SelectBackend はIMSIからPLMNを抽出し、対応するバックエンドを選択する。
// passthroughモードの場合は常にデフォルト（内部Vector API）を返す。
// PLMNマップにマッチしない場合もデフォルトを返す。
func (r *Router) SelectBackend(imsi string) (backend.Backend, error) {
    // passthroughモード: 常にデフォルト
    if r.passthrough {
        return r.registry.Default(), nil
    }

    // PLMNマップが空の場合はデフォルト
    if len(r.plmnMap) == 0 {
        return r.registry.Default(), nil
    }

    // IMSIからPLMN候補を抽出（6桁優先、次に5桁）
    candidates := extractPLMNs(imsi)
    for _, plmn := range candidates {
        if backendID, ok := r.plmnMap[plmn]; ok {
            b, err := r.registry.Get(backendID)
            if err != nil {
                return nil, err
            }
            return b, nil
        }
    }

    // マッチなし: デフォルト
    return r.registry.Default(), nil
}
```

> **注記:** 選択結果のログはハンドラーが `GW_ROUTE`（INFO、`backend_id` / `backend_name` 付き）として出力する（9.2節）。

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
// internal/backend/errors.go
type BackendNotImplementedError struct {
    ID string
}

func (e *BackendNotImplementedError) Error() string {
    return fmt.Sprintf("backend %q is not implemented", e.ID)
}

// internal/handler/vector.go（ルーティングエラーの変換、抜粋）
var notImpl *backend.BackendNotImplementedError
if errors.As(err, &notImpl) {
    c.JSON(http.StatusNotImplemented, httputil.NewProblemDetail(
        http.StatusNotImplemented,
        "Not Implemented",
        fmt.Sprintf("Backend %q is not implemented", notImpl.ID),
    ))
    return
}
```

バックエンドが返すエラーは、ハンドラーで次のように変換する（接続方式共通）。

| バックエンドのエラー | auth-server への応答 |
|---------------------|---------------------|
| `BackendResponseError` | `StatusCode` と `Problem`（ProblemDetail）をそのまま返す（4xx） |
| `BackendCommunicationError` | 502 Bad Gateway |
| その他 | 500 Internal Server Error |

### 4.4 接続方式IDと実装の紐付け（バックエンドレジストリ）

PoC段階ではハードコーディングで管理する。`NewRegistry` は `00` を常に登録し、`01` は `VECTOR_GATEWAY_AKAONLY_URL` が設定されている場合にのみ登録する。`01` の生成（証明書の読み込み等）に失敗した場合は `NewRegistry` がエラーを返し、`main.go` が `failed to initialize backends` を出力して起動を中止する。

```go
// internal/backend/registry.go
func NewRegistry(cfg *config.Config) (*Registry, error) {
    r := &Registry{
        backends:  make(map[string]Backend),
        defaultID: defaultBackendID,
    }

    // 内部Vector APIバックエンドを登録
    internal := NewInternalBackend(cfg.InternalURL, cfg.InternalTimeout)
    r.backends[internalBackendID] = internal

    // aka-only-serverバックエンドを登録（URL設定時のみ）
    if cfg.AKAOnlyEnabled() {
        akaOnly, err := NewAKAOnlyBackend(AKAOnlyOptions{
            BaseURL:        cfg.AKAOnlyURL,
            ClientCertFile: cfg.AKAOnlyClientCert,
            ClientKeyFile:  cfg.AKAOnlyClientKey,
            ServerCertFile: cfg.AKAOnlyServerCert,
            Timeout:        cfg.AKAOnlyTimeout,
            MaskIMSI:       cfg.LogMaskIMSI,
        })
        if err != nil {
            return nil, fmt.Errorf("failed to create aka-only-server backend: %w", err)
        }
        r.backends[akaOnlyBackendID] = akaOnly
    }

    return r, nil
}
```

| メソッド | 動作 |
|---------|------|
| `Get(id)` | 登録済みならそのバックエンド、未登録なら `BackendNotImplementedError` |
| `Default()` | 内部Vector API（ID:00） |

### 4.5 接続方式01: aka-only-server バックエンド

#### 4.5.1 概要・目的

aka-only-server（https://github.com/oyaguma3/aka-only-server 、管理GUI: https://github.com/oyaguma3/web-gui-for-aka-only-server ）は、3GPP TS 29.503 Nudm_UEAU GenerateAv ベースの API で AKA 認証ベクターを払い出す外部サーバーである。クライアントは mTLS のクライアント証明書（証明書ピン留め）で識別される。

接続方式 `01` は、`VECTOR_GATEWAY_PLMN_MAP` で `01` を指定した PLMN の加入者についてだけ、認証ベクターを aka-only-server から取得する。目的は、加入者情報（Ki/OPc）と SQN を PoC の外部で管理する構成を、auth-server を変更せずに実現することである。

| 項目 | 仕様 |
|------|------|
| 実装 | `internal/backend/akaonly.go`（`AKAOnlyBackend`、ID `01`、Name `aka-only-server`） |
| 有効化 | `VECTOR_GATEWAY_AKAONLY_URL` を設定し、PLMNマップで `01` を指定（既定は無効。未一致PLMNは従来どおり `00`） |
| 接続 | mTLS（既定、`https://`）／平文HTTP（`http://`、同一ホスト内に限る）。URL のスキームで切り替える |
| 要求する認証タイプ | EAP-AKA' でも常に `EAP_AKA`（`eap-aka`）で要求して CK/IK を受け取る。CK'/IK' は従来どおり auth-server が導出する |
| 影響範囲 | auth-server / acct-server / vector-api / admin-tui のコード、内部IF（D-03 の `POST /api/v1/vector`）は変更なし |

#### 4.5.2 IF変換仕様（リクエスト）

```
POST {VECTOR_GATEWAY_AKAONLY_URL}/nudm-ueau/v1/imsi-{IMSI}/hss-security-information/eap-aka/generate-av
Content-Type: application/json
Accept: application/json, application/problem+json
X-Trace-ID: {uuid}   ※ベストエフォート（aka-only-server は参照しない）
```

| 内部IF（受信） | GenerateAv（送信） |
|---------------|-------------------|
| `imsi` | パスの `imsi-{IMSI}` |
| （なし） | `hssAuthType`: `"EAP_AKA"` 固定 |
| （なし） | `numOfRequestedVectors`: `1` 固定 |
| `resync_info.rand` | `resynchronizationInfo.rand` |
| `resync_info.auts` | `resynchronizationInfo.auts` |

- `resync_info` がなければ `resynchronizationInfo` は送らない。
- `anId` は送らない（`eap-aka` では無視されるため）。

#### 4.5.3 IF変換仕様（レスポンス）

```json
{"hssAuthenticationVectors":[{"avType":"EAP_AKA","rand":"...","xres":"...","autn":"...","ck":"...","ik":"..."}]}
```

- 200 応答の `hssAuthenticationVectors[0]` の `rand` / `autn` / `xres` / `ck` / `ik` を、内部IF（7.2節）の同名フィールドに詰め替える。
- 配列が空、JSONとして解釈できない、必須フィールド（`rand` / `autn` / `xres` / `ck` / `ik`）のいずれかが欠けている場合は `BackendCommunicationError`（→ 502）とする。
- 応答ボディの読み込み上限は 64KiB。超過した場合も `BackendCommunicationError` とする。

#### 4.5.4 エラー変換

aka-only-server のエラー本文は `application/problem+json` で、`title` / `status` / `detail` / `cause` を持つ（`type` はない）。

| aka-only-server の応答 | `cause` | gateway 内のエラー | auth-server への応答 |
|-----------------------|---------|-------------------|---------------------|
| 404 | `USER_NOT_FOUND` | `BackendResponseError` | 404 |
| 403 | `AUTHENTICATION_REJECTED` | `BackendResponseError` | 403 |
| 400 | `INVALID_MSG_FORMAT` ほか | `BackendResponseError` | 400 |
| 501 | `AUTH_TYPE_NOT_SUPPORTED` | `BackendCommunicationError` | 502 |
| 5xx | `SYSTEM_FAILURE` | `BackendCommunicationError` | 502 |
| タイムアウト、接続失敗、TLSハンドシェイク失敗（未登録・無効なクライアント証明書、信頼しないサーバー証明書、ホスト名不一致） | — | `BackendCommunicationError` | 502 |
| aka-only-server の ProblemDetails でない 4xx（JSONでない、`cause` が空。例: URL誤りによる `404 page not found`） | — | `BackendCommunicationError` | 502 |
| 200 / 4xx / 5xx 以外（例: 204） | — | `BackendCommunicationError` | 502 |

- 4xx は `httputil.ProblemDetail`（`type` は `about:blank`）に詰め替えて `BackendResponseError.Problem` に入れ、`title` / `status` / `detail` を引き継ぐ。`title` が空の場合は HTTP ステータス文言で補う。
- `cause` は内部IFの ProblemDetail にないため、gateway のログ（9.2節）にだけ出力する。
- 403 になるのは次の3つで、`detail` で区別する。
  1. クライアントが加入者の許可クライアントに含まれていない（`client is not allowed for this subscriber`）
  2. 再同期で AUTS の検証に失敗した（`AUTS verification failed`。内部 vector-api では 400 だった点に注意）
  3. 平文HTTP で、加入者の平文HTTP許可フラグが立っていない（`detail` は 1 と同じ `client is not allowed for this subscriber`）
- auth-server は 200 以外の 4xx を Circuit Breaker 対象外の `APIError` として Reject するため、403 が返っても auth-server の変更は不要。auth-server のログは 404 → `VECTOR_IMSI_NOT_FOUND`、403 → `VECTOR_API_ERR`（`http_status=403`）となる。
- 502 は auth-server の Circuit Breaker の失敗回数に数えられる。Circuit Breaker は接続方式ごとに分かれていないため、開くと `00` 向けの認証も止まる。
- 通信エラーのメッセージ（`url.Error`）には URL（パスに生 IMSI を含む）が含まれるため、原因のエラーだけを残してログに出す。

#### 4.5.5 TLS設定

`https://` の場合、起動時に `newAKAOnlyTLSConfig` で次の TLS 設定を組み立てる。

| 項目 | 設定 |
|------|------|
| `RootCAs` | `VECTOR_GATEWAY_AKAONLY_SERVER_CERT` の証明書だけを入れたプール（システムのCAは使わない＝証明書ピン留め） |
| `Certificates` | `VECTOR_GATEWAY_AKAONLY_CLIENT_CERT`（と `CLIENT_KEY`）から読んだクライアント証明書 |
| `MinVersion` | TLS 1.2 |
| ホスト名検証 | 有効（無効にしない）。URL のホスト名がサーバー証明書の SAN に入っている必要がある。aka-only-server の既定 SAN は `localhost,127.0.0.1,aka-only-server`（変更は aka-only-server 側の `AKA_AV_TLS_HOSTS`） |
| 読み込み時期 | 起動時に1回だけ。証明書の差し替えは vector-gateway の再起動で反映する |

- 秘密鍵は `CLIENT_KEY` が空なら `CLIENT_CERT` と同じファイルから読む（`client gen-cert` の出力は証明書と秘密鍵が1ファイル）。証明書と鍵の対応も `tls.X509KeyPair` で検証する。
- `http://` の場合、証明書の設定は使わない。CK/IK が平文で流れるため、起動時に WARN を出す（5.3節）。

#### 4.5.6 やらないこと

| 事項 | 理由 |
|------|------|
| フォールバック（`01` で失敗したら `00` を試す等） | 同じ加入者を2か所で管理すると SQN が競合するため（1.4節） |
| `eap-aka-prime` での要求（CK'/IK' を aka-only-server 側で導出） | CK'/IK' の導出は auth-server が担っており、内部IFに認証タイプを追加しないため |
| 複数ベクターの先取り | 内部IFは1回1ベクター。`numOfRequestedVectors` は `1` 固定 |
| デフォルトバックエンドの切り替え設定 | 未一致PLMNは従来どおり `00` で十分なため |

### 4.6 将来: 設定ファイル方式

将来的には設定ファイル（YAML）で接続方式を管理する（現時点では未実装。接続方式 `01` も環境変数で設定する）。以下は構想例である。

```yaml
# configs/vector-gateway/backends.yaml
backends:
  - id: "00"
    type: internal
    config:
      url: "http://vector-api:8080"
  
  - id: "01"
    type: aka-only-server
    config:
      url: "https://aka-only-server:8443"
      client_cert: "/certs/av-client.pem"   # mTLSで識別（API Key/Tokenは使わない）
      server_cert: "/certs/av-server.pem"
  
  - id: "02"
    type: partner-a                          # 将来構想
    config:
      endpoint: "https://api.partner-a.example.com"
      api_key: "${PARTNER_A_API_KEY}"

plmn_map:
  "44010": "01"  # → aka-only-server
  "310260": "02" # → Partner A（将来構想）
```

Docker Composeでのマウント:

```yaml
vector-gateway:
  volumes:
    - ./configs/vector-gateway/backends.yaml:/app/config/backends.yaml:ro
```

### 4.7 Dockerfile方針

#### 4.7.1 マルチステージビルド構成

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

#### 4.7.2 ベースイメージ選定

| ステージ   | イメージ               | 理由                                     |
| ---------- | ---------------------- | ---------------------------------------- |
| ビルド     | `golang:1.25-bookworm` | Go 1.25.x、Debian Bookwormベース         |
| ランタイム | `debian:bookworm-slim` | 最小構成、ヘルスチェック用curlが導入可能 |

#### 4.7.3 必須パッケージ

| パッケージ        | 用途                             |
| ----------------- | -------------------------------- |
| `ca-certificates` | TLS証明書（外部API連携に備える） |
| `curl`            | ヘルスチェック（`curl -fsS`）    |

> **注記:** distrolessイメージは採用しない。ヘルスチェックに `curl` が必要なため。
>
> 接続方式 `01`（aka-only-server）はシステムのCAを使わず、`deployments/certs/` からマウントした証明書だけを信頼する（4.5.5）。証明書はイメージに含めない。

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
# 内部Vector API設定（ベースURL。パス /api/v1/vector は内部バックエンドが付与）
# -----------------------------------------------------------------------------
VECTOR_GATEWAY_INTERNAL_URL=http://vector-api:8080

# -----------------------------------------------------------------------------
# PLMNマッピング
# -----------------------------------------------------------------------------
# 形式: "PLMN:ID,PLMN:ID,..."
# - PLMN: MCC+MNC結合形式（5-6桁）
# - ID: 接続方式ID（2桁）
#   - 00: Vector API（内部。マップに一致しないPLMNもこちら）
#   - 01: aka-only-server（VECTOR_GATEWAY_AKAONLY_URL の設定が必要。未設定なら501）
#   - 02-99: 将来実装（指定すると501）
#
# 空文字列の場合: 全てVector APIへ転送
#
# 例: VECTOR_GATEWAY_PLMN_MAP="44010:01"
VECTOR_GATEWAY_PLMN_MAP=""

# -----------------------------------------------------------------------------
# タイムアウト設定
# -----------------------------------------------------------------------------
VECTOR_GATEWAY_INTERNAL_TIMEOUT=5s

# -----------------------------------------------------------------------------
# aka-only-server 接続設定（接続方式ID:01、オプション）
# -----------------------------------------------------------------------------
# ベースURL。空なら 01 は登録しない（01 に向けたPLMNは501）
#   https://: mTLS で接続（既定・推奨）。CLIENT_CERT と SERVER_CERT が必須
#   http:// : 平文HTTP（同一ホスト内に限る）。証明書設定は無視
VECTOR_GATEWAY_AKAONLY_URL=
# クライアント証明書（PEM。証明書と秘密鍵が1ファイルでよい）
VECTOR_GATEWAY_AKAONLY_CLIENT_CERT=
# 秘密鍵（PEM。空なら CLIENT_CERT のファイルから読む）
VECTOR_GATEWAY_AKAONLY_CLIENT_KEY=
# aka-only-server の AV 用サーバー証明書（PEM。これだけを信頼する）
VECTOR_GATEWAY_AKAONLY_SERVER_CERT=
# 呼び出しタイムアウト
VECTOR_GATEWAY_AKAONLY_TIMEOUT=5s

# -----------------------------------------------------------------------------
# ログ設定
# -----------------------------------------------------------------------------
# IMSIマスキング有効化（デフォルト: true）
# - true : IMSI中央部分をマスク（本番環境推奨）
# - false: IMSI全桁表示（デバッグ用）
LOG_MASK_IMSI=true
```

接続方式 `01` の環境変数（r5で追加）:

| 環境変数 | 既定値 | 内容 |
|---------|-------|------|
| `VECTOR_GATEWAY_AKAONLY_URL` | （空） | aka-only-server のベースURL（例: `https://aka-only-server:8443`）。空なら `01` を登録しない（`01` に向けたPLMNは従来どおり501） |
| `VECTOR_GATEWAY_AKAONLY_CLIENT_CERT` | （空） | クライアント証明書の PEM。`client gen-cert` の出力のように証明書と秘密鍵が1ファイルでよい |
| `VECTOR_GATEWAY_AKAONLY_CLIENT_KEY` | （空） | 秘密鍵の PEM。空なら `CLIENT_CERT` と同じファイルから読む |
| `VECTOR_GATEWAY_AKAONLY_SERVER_CERT` | （空） | aka-only-server の AV 用サーバー証明書（`av-cert` の出力）。これだけを信頼する |
| `VECTOR_GATEWAY_AKAONLY_TIMEOUT` | `5s` | 呼び出しのタイムアウト |

### 5.2 docker-compose.yml（抜粋）

`deployments/docker-compose.yml` の該当部分（ロギング・ヘルスチェック等の共通設定は省略）。

```yaml
services:
  auth-server:
    environment:
      VECTOR_API_URL: http://vector-gateway:8080
      LOG_MASK_IMSI: ${LOG_MASK_IMSI:-true}
    depends_on:
      vector-gateway:
        condition: service_started

  vector-gateway:
    build:
      context: ..
      dockerfile: apps/vector-gateway/Dockerfile
    expose:
      - "8080"
    environment:
      <<: *timezone
      VECTOR_GATEWAY_MODE: ${VECTOR_GATEWAY_MODE:-gateway}
      VECTOR_GATEWAY_INTERNAL_URL: http://vector-api:8080
      VECTOR_GATEWAY_PLMN_MAP: ${VECTOR_GATEWAY_PLMN_MAP:-}
      VECTOR_GATEWAY_INTERNAL_TIMEOUT: ${VECTOR_GATEWAY_INTERNAL_TIMEOUT:-5s}
      # aka-only-server（接続方式ID:01）。URLが空なら01は無効
      VECTOR_GATEWAY_AKAONLY_URL: ${VECTOR_GATEWAY_AKAONLY_URL:-}
      VECTOR_GATEWAY_AKAONLY_CLIENT_CERT: ${VECTOR_GATEWAY_AKAONLY_CLIENT_CERT:-}
      VECTOR_GATEWAY_AKAONLY_CLIENT_KEY: ${VECTOR_GATEWAY_AKAONLY_CLIENT_KEY:-}
      VECTOR_GATEWAY_AKAONLY_SERVER_CERT: ${VECTOR_GATEWAY_AKAONLY_SERVER_CERT:-}
      VECTOR_GATEWAY_AKAONLY_TIMEOUT: ${VECTOR_GATEWAY_AKAONLY_TIMEOUT:-5s}
      LOG_MASK_IMSI: ${LOG_MASK_IMSI:-true}
    volumes:
      # aka-only-server接続用の証明書（av-client.pem / av-server.pem）
      - ./certs:/certs:ro
    depends_on:
      vector-api:
        condition: service_healthy
    restart: always

  vector-api:
    # 変更なし（LOG_MASK_IMSIはD-11で定義済み）
```

### 5.3 起動時検証と起動ログ

`config.Load()`（`validateAKAOnly`）と `backend.NewRegistry()` で接続方式 `01` の設定を検証する。検証に失敗すると起動しない（コンテナは再起動を繰り返す）。

| 検証 | 失敗時のログ（ERROR） |
|------|---------------------|
| URL が `http://` または `https://` で始まる（末尾の `/` は取り除く） | `failed to load config` |
| `VECTOR_GATEWAY_AKAONLY_TIMEOUT` が正の値 | `failed to load config` |
| `https://` の場合、`CLIENT_CERT` と `SERVER_CERT` が指定されている | `failed to load config` |
| `https://` の場合、証明書・秘密鍵ファイルが読めて解釈でき、証明書と鍵が対応している | `failed to initialize backends` |

URL が空の場合は `01` を使わないため検証しない。

起動を止めない注意事項は WARN で出力する（`warnBackendConfig`）。

| 条件 | WARN メッセージ | 付加項目 |
|------|----------------|---------|
| `http://` で接続する | `aka-only-server is connected over plain HTTP; CK/IK are transmitted unencrypted` | `akaonly_url` |
| PLMNマップが登録されていないバックエンドID（例: URL 空なのに `01`、または `02`〜`99`）を参照している（passthroughモードでは出さない） | `PLMN map refers to a backend that is not configured; requests will fail with 501` | `plmn`, `backend_id` |

起動ログ `starting vector-gateway`（INFO）には `listen_addr` / `log_level` / `mode` / `plmn_map_entries` に加えて、次の項目を出力する。

| 項目 | 値 |
|------|----|
| `akaonly_enabled` | `VECTOR_GATEWAY_AKAONLY_URL` が設定されているか（bool） |
| `akaonly_transport` | `mtls`（https）／`plain`（http）／`disabled`（URL 未設定） |

### 5.4 aka-only-server との接続構成

#### 5.4.1 ネットワーク（docker-compose.aka-av.yml）

aka-only-server を同一ホストで動かす場合は、オーバーレイ `deployments/docker-compose.aka-av.yml` を重ね、vector-gateway だけを aka-only-server の compose が作る共有ネットワーク（external、名前 `${AKA_SHARED_NETWORK:-aka-av}`）に参加させる。共有ネットワークは aka-only-server 側が作るため、先に aka-only-server を起動しておく。Web GUI リポジトリの `compose.aka-av.yaml` と同じ方式である。

```yaml
# deployments/docker-compose.aka-av.yml
services:
  vector-gateway:
    networks:
      - default
      - aka-av

networks:
  aka-av:
    external: true
    name: ${AKA_SHARED_NETWORK:-aka-av}
```

```bash
docker compose -f docker-compose.yml -f docker-compose.aka-av.yml up -d
```

| 配置 | `VECTOR_GATEWAY_AKAONLY_URL` | 備考 |
|------|-----------------------------|------|
| 同一ホスト（mTLS） | `https://aka-only-server:8443` | オーバーレイを重ねる |
| 同一ホスト（平文HTTP） | `http://aka-only-server:8080` | aka-only-server 側で `AKA_AV_PLAIN_ADDR=:8080` を設定し、加入者ごとに平文HTTP許可（`-allow-plain`）を立てる |
| 別ホスト | `https://<相手ホストのアドレス>:8443` | オーバーレイは重ねない。そのアドレスを aka-only-server 側の `AKA_AV_TLS_HOSTS` に入れておく（SAN に入れるため） |

#### 5.4.2 証明書の配置

- 証明書は `deployments/certs/` に置き、コンテナ内には `/certs`（読み取り専用）としてマウントする。環境変数にはコンテナ内のパス（`/certs/...`）を指定する。
- `deployments/certs/` は `.gitkeep` だけをコミットし、中身は `.gitignore`（`deployments/certs/*` と `!deployments/certs/.gitkeep`）で除外する。

#### 5.4.3 接続手順

aka-only-server 側で、クライアント証明書の発行・登録、AV 用サーバー証明書の取得、加入者の登録を行う（Web GUI からも可）。

```bash
docker compose exec -T aka-only-server /aka-only-server client gen-cert -name radius-gw > av-client.pem
docker compose exec -T aka-only-server /aka-only-server client add -name radius-gw -cert - < av-client.pem
docker compose exec -T aka-only-server /aka-only-server av-cert > av-server.pem
docker compose exec -T aka-only-server /aka-only-server subscriber add -imsi <IMSI> -ki <Ki> -opc <OPc> -clients <発番されたクライアントID>
```

- `client add` は `client 1 added (name=radius-gw fingerprint=... not-after=...)` のように発番されたIDを表示する。
- `subscriber add` の主なオプション: `-sqn`（既定 `000000000000`）、`-amf`（既定 `8000`）、`-sqn-type`（`inc1` / `inc32` / `inc33`、既定 `inc32`）、`-allow-plain`（平文HTTP許可）、`-clients`（許可クライアントID、カンマ区切り）。

PoC 側では、2つの PEM を `deployments/certs/` に置き、`.env` を設定して vector-gateway を（再）起動する。

```
VECTOR_GATEWAY_AKAONLY_URL=https://aka-only-server:8443
VECTOR_GATEWAY_AKAONLY_CLIENT_CERT=/certs/av-client.pem
VECTOR_GATEWAY_AKAONLY_SERVER_CERT=/certs/av-server.pem
VECTOR_GATEWAY_PLMN_MAP=44010:01
```

### 5.5 運用上の注意（接続方式01）

| 項目 | 注意事項 |
|------|---------|
| 加入者の登録先が2か所 | `01` に向けたPLMNの加入者は、Ki/OPc を aka-only-server（Web GUI またはコマンド）に登録する。認可ポリシー（`policy:{IMSI}`）と RADIUS クライアントは従来どおり Admin TUI で PoC の Valkey に登録する。auth-server はポリシー未登録だと Reject するため両方が必要。Admin TUI の加入者画面は `01` に向けたPLMNの加入者には使わない |
| SQN | 内部 vector-api は +32 固定。aka-only-server は SQN 増加タイプ `inc32`（既定）が同じ挙動。内部 vector-api で使っていた SIM を移すと、初回に再同期が1回走って追従する |
| 振り分け単位 | PLMN 単位。同じ PLMN の IMSI を `00` と `01` に分けることはできない |
| 証明書の差し替え | AV 用サーバー証明書を作り直したら `av-server.pem` を取り直して vector-gateway を再起動する。クライアント証明書の期限（既定 825 日）に注意する |
| Circuit Breaker | 未登録・無効のクライアント証明書は aka-only-server が TLS ハンドシェイクで拒否し、gateway からは通信失敗（502）に見える。502 は auth-server の Circuit Breaker の失敗回数に数えられ、Circuit Breaker は接続方式共通のため、開くと `00` 向けの認証も止まる |
| ログの突き合わせ | aka-only-server は Trace ID を記録せず、IMSI をマスクしない。gateway のログ（`trace_id`、時刻）と aka-only-server のログ（`client_id`、`imsi`、時刻）で突き合わせる |
| 平文HTTP | 同一ホスト内に限る（5.4.1） |

障害切り分けのポイント（gateway の `BACKEND_EXTERNAL_ERR` の `http_status` / `cause` / `error` を見る）:

| 症状 | 主な原因 |
|------|---------|
| `http_status=0`、`error` に `tls: failed to verify certificate` | サーバー証明書不一致・ホスト名不一致（`av-server.pem` の取り直し、URL のホスト名と `AKA_AV_TLS_HOSTS` を確認） |
| `http_status=0`、`error` に `remote error: tls: bad certificate` など | クライアント証明書が未登録・無効・期限切れ |
| `http_status=0`、`error` に `connection refused` / `no such host` | aka-only-server 停止、共有ネットワーク未参加（オーバーレイ未適用、aka-only-server 未起動）、URL 誤り |
| 404 `USER_NOT_FOUND` | aka-only-server に加入者未登録 |
| 403 `AUTHENTICATION_REJECTED` | 許可クライアント外 / AUTS 検証失敗 / 平文HTTP許可なし（`detail` で区別、4.5.4） |
| 起動しない | `failed to load config` / `failed to initialize backends` のエラー文を確認（5.3） |

---

## 6. パッケージ構成

### 6.1 ディレクトリ構造

```
apps/vector-gateway/
├── main.go                    # エントリポイント（起動ログ・設定WARN出力を含む）
├── main_test.go
└── internal/
    ├── backend/
    │   ├── akaonly.go         # aka-only-server呼び出し（ID:01、mTLS/平文HTTP）
    │   ├── akaonly_test.go
    │   ├── errors.go          # エラー定義
    │   ├── errors_test.go
    │   ├── interface.go       # Backend共通インターフェース
    │   ├── internal.go        # 内部Vector API呼び出し（ID:00）
    │   ├── internal_test.go
    │   ├── registry.go        # バックエンド登録管理
    │   └── registry_test.go
    ├── config/
    │   ├── config.go          # 環境変数読み込み・検証
    │   └── config_test.go
    ├── handler/
    │   ├── health.go          # /health ヘルスチェックハンドラ
    │   ├── vector.go          # /api/v1/vector ハンドラ
    │   └── vector_test.go
    ├── router/
    │   ├── router.go          # PLMNベースルーティング
    │   └── router_test.go
    └── server/
        ├── middleware.go      # X-Trace-ID伝搬等ミドルウェア
        ├── router.go          # HTTPルーター設定
        ├── server.go          # HTTPサーバー起動・管理
        └── server_test.go
```

> **注記（r5での変更点）:**
> - `backend/akaonly.go` を新設（接続方式 `01`）
> - IMSIマスキングは共通ライブラリ `pkg/logging.MaskIMSI` を使用（`internal/logging/` は存在しない）
> - `main_test.go`、`server/server_test.go` を追加

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
    // Gateway動作モード（"gateway" or "passthrough"）
    Mode string `envconfig:"VECTOR_GATEWAY_MODE" default:"gateway"`

    // 内部Vector API接続先URL
    InternalURL string `envconfig:"VECTOR_GATEWAY_INTERNAL_URL" required:"true"`

    // 内部Vector APIへのタイムアウト
    InternalTimeout time.Duration `envconfig:"VECTOR_GATEWAY_INTERNAL_TIMEOUT" default:"5s"`

    // PLMNマッピング文字列（"44010:01,44020:01" 形式）
    PLMNMapRaw string `envconfig:"VECTOR_GATEWAY_PLMN_MAP" default:""`

    // aka-only-server（接続方式ID:01）のベースURL。空なら01を登録しない
    AKAOnlyURL string `envconfig:"VECTOR_GATEWAY_AKAONLY_URL" default:""`

    // aka-only-serverへのmTLSで使うクライアント証明書（PEM）。秘密鍵を同じファイルに含めてよい
    AKAOnlyClientCert string `envconfig:"VECTOR_GATEWAY_AKAONLY_CLIENT_CERT" default:""`

    // クライアント証明書の秘密鍵（PEM）。空ならAKAOnlyClientCertから読む
    AKAOnlyClientKey string `envconfig:"VECTOR_GATEWAY_AKAONLY_CLIENT_KEY" default:""`

    // aka-only-serverのAV用サーバー証明書（PEM）。これだけを信頼する
    AKAOnlyServerCert string `envconfig:"VECTOR_GATEWAY_AKAONLY_SERVER_CERT" default:""`

    // aka-only-serverへのタイムアウト
    AKAOnlyTimeout time.Duration `envconfig:"VECTOR_GATEWAY_AKAONLY_TIMEOUT" default:"5s"`

    // サーバー設定
    ListenAddr  string `envconfig:"LISTEN_ADDR" default:":8080"`
    LogLevel    string `envconfig:"LOG_LEVEL" default:"INFO"`
    LogMaskIMSI bool   `envconfig:"LOG_MASK_IMSI" default:"true"`
    GinMode     string `envconfig:"GIN_MODE" default:"release"`
}
```

| メソッド | 内容 |
|---------|------|
| `Load()` | 環境変数を読み込み、`validateAKAOnly()` で接続方式 `01` の設定を検証する（5.3節） |
| `validateAKAOnly()` | URL が空なら何もしない。URL のスキーム、TIMEOUT、https 時の証明書指定を検証し、URL 末尾の `/` を取り除く |
| `AKAOnlyEnabled()` | `AKAOnlyURL` が空でないか |
| `AKAOnlyUseTLS()` | `AKAOnlyURL` が `https://` で始まるか |
| `ParsePLMNMap()` | `PLMNMapRaw` を `map[string]string` にパースして返す。PLMN は5〜6桁の数字、接続方式IDは2桁の数字であることを検証する（エラー時は `failed to parse PLMN map` で起動中止） |
| `IsPassthrough()` | `Mode` が `passthrough` か |

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
| 400 Bad Request | リクエスト不正、または aka-only-server が 400 を返した（ID:01） | `{"error": "invalid IMSI format"}` |
| 403 Forbidden | aka-only-server が 403 を返した（ID:01。許可クライアント外、AUTS検証失敗、平文HTTP許可なし） | `{"error": "client is not allowed for this subscriber"}` |
| 404 Not Found | IMSI未登録（内部API、または aka-only-server の `USER_NOT_FOUND`） | `{"error": "IMSI not found"}` |
| 501 Not Implemented | 未実装バックエンド（ID:02〜99、URL未設定時のID:01） | `{"error": "Backend ID 02 is not implemented"}` |
| 500 Internal Server Error | 内部エラー | `{"error": "internal server error"}` |
| 502 Bad Gateway | バックエンド通信エラー（aka-only-server の 5xx・不正応答・TLS失敗を含む） | `{"error": "backend communication failed"}` |

> **注記:** レスポンス例は概略である。実際のエラー本文は RFC 7807 準拠の ProblemDetail（`pkg/httputil.ProblemDetail`、`type` / `title` / `status` / `detail`）で返す。aka-only-server 由来の 4xx は `title` / `status` / `detail` を引き継ぎ、`type` は `about:blank` とする（4.5.4）。

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

### 8.2 外部API用エラー

接続方式 `01`（aka-only-server）で実装済みのもの（詳細な変換は 4.5.4）:

| カテゴリ | HTTPステータス | event_id | 対処 |
|---------|---------------|----------|------|
| aka-only-server の 4xx 応答（加入者・クライアント起因） | 400 / 403 / 404（そのまま伝搬） | `BACKEND_EXTERNAL_ERR`（WARN） | エラー返却（auth-server はCB対象外として Reject） |
| aka-only-server 通信エラー・5xx・不正応答 | 502 | `BACKEND_EXTERNAL_ERR`（ERROR） | エラー返却（auth-server のCB失敗回数に計上） |

将来追加予定のもの（`01` では使用しない）:

| カテゴリ | HTTPステータス | event_id | 対処 |
|---------|---------------|----------|------|
| 外部API認証失敗 | 502 | `EXTERNAL_AUTH_ERR` | エラー返却 |
| 外部API Rate Limit | 503 | `EXTERNAL_RATE_LIMIT` | リトライ後エラー返却 |

> **注記:** `01` ではクライアント認証を mTLS で行うため、認証失敗（未登録・無効なクライアント証明書）は TLS ハンドシェイク失敗として通信エラー（502、`BACKEND_EXTERNAL_ERR`）に含まれる。

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

#### 外部バックエンド（aka-only-server 等）利用時

Vector Gatewayまでのトレーサビリティを保証する。外部APIへのヘッダ付与はベストエフォートとし、外部側の対応は期待しない。aka-only-server は X-Trace-ID を参照・記録しないため、両者のログは gateway 側の `trace_id`・時刻と aka-only-server 側の `client_id`・`imsi`・時刻で突き合わせる（5.5節）。

```
Auth Server (trace_id生成)
     │
     │ Header: X-Trace-ID: {uuid}
     ▼
Vector Gateway (ログ出力で境界記録)
     │
     │ Header: X-Trace-ID: {uuid} ※ベストエフォート
     ▼
外部API / aka-only-server (対応は保証されない)
```

外部API呼び出し時は、Vector Gatewayのログで以下の情報を記録し、トレーサビリティの境界を明確化する（接続方式 `01` の例）：

```json
{
  "time": "2026-10-04T12:00:00.000+09:00",
  "level": "INFO",
  "msg": "aka-only-server call succeeded",
  "app": "vector-gateway",
  "trace_id": "550e8400-e29b-...",
  "event_id": "BACKEND_EXTERNAL_CALL",
  "imsi": "440100********1",
  "backend_id": "01",
  "external_endpoint": "https://aka-only-server:8443",
  "http_status": 200,
  "latency_ms": 12,
  "resync": false
}
```

`external_endpoint` はベースURLであり、IMSI を含むパスは出力しない。

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

#### event_id一覧（接続方式01: aka-only-server、r5で実装）

1回の aka-only-server 呼び出しにつき、次のいずれか1行を出力する（`internal/backend/akaonly.go`）。

| event_id | レベル | msg | 発生条件 | 項目 |
|----------|--------|-----|---------|------|
| `BACKEND_EXTERNAL_CALL` | INFO | `aka-only-server call succeeded` | 呼び出し成功（200 かつ変換成功） | `trace_id`, `imsi`（マスク）, `backend_id`（`01`）, `external_endpoint`（ベースURL）, `http_status`, `latency_ms`, `resync`（bool） |
| `BACKEND_EXTERNAL_ERR` | WARN | `aka-only-server rejected request` | aka-only-server が 4xx を返した（加入者・クライアント起因） | `trace_id`, `imsi`（マスク）, `backend_id`, `http_status`, `latency_ms`, `cause`, `error` |
| `BACKEND_EXTERNAL_ERR` | ERROR | `aka-only-server call failed` | 通信失敗・5xx・不正応答 | 同上（応答がない場合 `http_status=0`、`cause` は空） |

- CK / IK / XRES はログに出力しない。`imsi` は `pkg/logging.MaskIMSI`（`LOG_MASK_IMSI` に従う）でマスクする。
- `cause` は aka-only-server の ProblemDetails の `cause`（`USER_NOT_FOUND` 等）。lnav フォーマット（`deployments/lnav_formats/eap_aka_log.json`）にも `cause` を追加済み。
- ハンドラーが出力する `GW_ROUTE` / `GW_OK` / `GW_ERR` は接続方式によらず従来どおり出力される。

#### event_id一覧（将来追加予定）

| event_id | 発生条件 | レベル |
|----------|---------|--------|
| `EXTERNAL_AUTH_ERR` | 外部API認証失敗 | ERROR |
| `EXTERNAL_RATE_LIMIT` | 外部API Rate Limit | WARN |

> **注記:** 実装のハンドラー・ルーターが出力する event_id は `GW_ROUTE`（バックエンド選択、INFO）、`GW_OK`（転送成功、INFO）、`GW_ERR`（リクエスト不正・501・バックエンドエラー等、WARN/ERROR）であり、上記「PoC」表の `PLMN_ROUTE_MATCH` 等とは一致していない（D-04 の記載とも差異あり）。本版では既存部分の記載は変更していない。

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

共通ライブラリ `pkg/logging`（E-03）の `MaskIMSI` を使用する（vector-gateway 内に独自実装は持たない）。

```go
// pkg/logging/masking.go

// MaskIMSI はIMSIをマスキングする
func MaskIMSI(imsi string, enabled bool) string {
    if !enabled {
        return imsi
    }
    return MaskPartial(imsi, 6, 1, '*')
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
| `BACKEND_EXTERNAL_CALL` | 外部API（aka-only-server）呼び出し成功時 | マスキング対象 |
| `BACKEND_EXTERNAL_ERR` | 外部API（aka-only-server）呼び出し失敗時 | マスキング対象（エラー文からも URL を除去し、生IMSIを出さない） |
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

### 10.2 Phase 2: 外部API接続方式（接続方式01: aka-only-server）【実装済み】

| 項目 | 内容 |
|------|------|
| 実装範囲 | 接続方式 `01`（aka-only-server、Nudm_UEAU GenerateAvベース）の追加（4.5節） |
| 接続 | mTLS（既定）／平文HTTP（同一ホスト限定） |
| 有効化 | `VECTOR_GATEWAY_AKAONLY_URL` 設定時のみ。未設定時の挙動は Phase 1 と同じ |
| Auth Server変更 | なし（内部IF D-03 も変更なし） |
| テスト | 単体テスト（vector-gateway 全体カバレッジ 82.0%）、E2E（T-04） |

### 10.3 将来フェーズ

| フェーズ | 内容 | 前提条件 |
|---------|------|---------|
| **Phase 2（追加）** | 接続方式 `02` 以降の実装 | 接続先API仕様確定 |
| **Phase 3** | 設定ファイル方式への移行 | Phase 2完了 |
| **Phase 4** | Valkey + Admin TUI管理 | Phase 3完了 |

---

## 11. PoC実装スコープサマリ

| 項目 | PoC実装 | 将来拡張 |
|------|---------|---------|
| ルーティング | 環境変数ベース（PLMNマップ） | 設定ファイル → Valkey + Admin TUI |
| 接続方式 | ID:00（Vector API）、ID:01（aka-only-server、URL設定時） | ID:02以降の外部API接続方式を順次追加 |
| PLMN照合 | 固定長照合 | 必要に応じてMCC判定追加 |
| 未実装ID | 501エラー返却（ID:02〜99、URL未設定時のID:01） | 接続方式実装に応じて解消 |
| 未登録PLMNデフォルト | Vector API利用 | エラー返却（CB発動抑制目的） |
| フォールバック | なし | SQN競合問題のため未実装継続見込み |
| モデル2（2フェーズ認証） | 未対応 | AT_MAC問題のため対応困難 |
| トレーサビリティ | 内部API: エンドツーエンド | 外部API（aka-only-server）: Gatewayまで保証 |

---

## 12. ドキュメント・設計への影響

### 12.1 新規作成ドキュメント

| No. | ドキュメント名 | 内容 |
|-----|---------------|------|
| D-12 | Vector Gateway詳細設計書 | パッケージ構成、API仕様、バックエンド連携 |

### 12.2 改訂対象ドキュメント（初版作成時点）

| No. | ドキュメント名 | 当時の版数 | 改訂内容 |
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

### 12.4 接続方式01（aka-only-server）追加に伴う改訂（r5）

| No. | ドキュメント名 | 改訂後版数 | 改訂内容 |
|-----|---------------|-----------|---------|
| D-01 | ミニPC版設計仕様書 | r10 | 構成図・環境変数の説明に接続方式 `01` を追加 |
| D-03 | Vector-APIインターフェース定義書 | r6 | Vector Gateway が 403 を返す場合があることを追記 |
| D-04 | ログ仕様設計書 | r19 | `BACKEND_EXTERNAL_CALL` / `BACKEND_EXTERNAL_ERR` を実装済みに、`cause` フィールド追加 |
| D-06 | エラーハンドリング詳細設計書 | r7 | aka-only-server のエラー変換を追記 |
| D-08 | インフラ設定・運用設計書 | r14 | compose の変更、共有ネットワーク、証明書の配置 |
| B-02 | アプリケーションデプロイ手順書 | r10 | aka-only-server への接続手順 |
| O-01 / O-03 | 操作ガイド / 障害対応手順書 | r2 / r2 | 加入者の登録先、切り分けポイント |
| T-02 / T-03 / T-04 | 単体 / 結合 / E2E テスト仕様書 | r2 / r8 / r5 | 接続方式 `01` のテストケース |

---

## 13. リスクと対策

| リスク | 影響 | 対策 |
|--------|------|------|
| 外部API仕様変更 | 変換ロジック修正必要 | 接続方式ごとにモジュール化、変更を局所化 |
| 未登録PLMN大量流入 | Vector APIへの負荷増大 | 将来的にデフォルトをエラーに変更検討 |
| 設定ミス（PLMN形式） | ルーティング失敗 | 起動時バリデーション、ログ出力 |
| aka-only-server 側の障害・証明書不整合（ID:01） | 502 が auth-server の CB 失敗回数に計上され、CB が開くと `00` 向けの認証も停止 | 起動時の証明書検証、`BACKEND_EXTERNAL_ERR` での切り分け（5.5節）、証明書期限の管理 |
| 加入者の登録漏れ（ID:01） | aka-only-server 未登録で 404、ポリシー未登録で Reject | 登録先が2か所（aka-only-server と Admin TUI）であることを運用手順に明記（5.5節） |
| SQN の不整合（内部 vector-api から移行した SIM） | 初回認証で再同期が発生 | aka-only-server の SQN 増加タイプを `inc32`（既定）とし、再同期で追従させる |

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
| 1 | 外部API接続先（接続方式 `02` 以降）の仕様調査 | 接続先確定時 |
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
| r5 | 2026-10-04 | 接続方式ID `01`（aka-only-server、mTLS/平文HTTP）を追加: 1.1/1.2/1.4更新、2.1〜2.3（構成図・通信フロー・責務）更新、3.1/3.2更新、3.4/3.5・4.3・4.4を実装コードに合わせて更新（レジストリの `01` 登録条件）、4.5新設（IF変換・エラー変換・TLS設定・やらないこと）、旧4.5/4.6を4.6/4.7に繰り下げ、5.1/5.2更新（`VECTOR_GATEWAY_AKAONLY_*`、docker-compose.yml を実ファイルに同期）、5.3〜5.5新設（起動時検証・起動ログ/WARN、docker-compose.aka-av.yml・証明書配置・接続手順、運用上の注意）、6.1/6.2を実装に合わせて更新、7.3に403追加、8.2・9.1・9.2・9.3.3・9.3.4で `BACKEND_EXTERNAL_CALL` / `BACKEND_EXTERNAL_ERR` を実装済みに更新、10.2/10.3・11・12.4・13・14.2更新 |
