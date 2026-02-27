# D-02 Valkey データ設計仕様書 (r11)

## 1. 全体方針

- **接続方式:** TCP `127.0.0.1:6379` (ACLパスワード認証必須)
- **データ永続化:** AOF (Append Only File) 有効化
- **ネーミング規則:** キーの役割を明確にするため、以下のプレフィックスを厳守する。

| **プレフィックス** | **カテゴリ** | **用途**                          | **永続性** |
| ------------------ | ------------ | --------------------------------- | ---------- |
| **`sub:`**         | Master       | **加入者情報 (Subscriber)**       | 永続       |
| **`client:`**      | Config       | **RADIUSクライアント設定**        | 永続       |
| **`policy:`**      | Config       | **認可ポリシー (Authorization)**  | 永続       |
| **`eap:`**         | State        | **EAP認証コンテキスト** (認証中)  | 一時 (60s) |
| **`sess:`**        | State        | **アクティブセッション** (認証後) | 長期 (24h) |
| **`acct:seen:`**   | State        | **Accounting重複検出キャッシュ**  | 一時 (24h) |
| **`idx:user:`**    | Index        | **ユーザー検索用インデックス**    | 動的       |

------

## 2. マスタデータ / 設定データ (永続)

Admin TUI等で管理者が事前に設定するデータ群です。

### A. 加入者情報 (Subscriber)

Vector APIがEAP-AKA認証ベクターを計算するための鍵情報。

- **Key:** `sub:{IMSI}`
- **Type:** `Hash`

| **Field**    | **必須** | **説明**               | **備考**                              |
| ------------ | -------- | ---------------------- | ------------------------------------- |
| `ki`         | Yes      | 秘密鍵 (K)             | Hex 32桁                              |
| `opc`        | Yes      | オペレータコード (OPc) | Hex 32桁                              |
| `amf`        | Yes      | AMF                    | Hex 4桁 (例: 8000)                    |
| `sqn`        | Yes      | シーケンス番号 (SQN)   | **Vector APIが認証毎にIncrementする（CAS更新）** |
| `created_at` | -        | 作成日時               |                                       |
> **SQN更新方式（競合制御）:**
> - Vector APIは `sqn` フィールドを **WATCH/MULTIによるCAS（Compare-And-Swap）** で原子更新する
> - 同一IMSIへの並行リクエスト時、Valkeyが競合を検出し EXEC が失敗する
> - 競合時はリトライ（上限3回）を行い、上限超過時は HTTP 409 Conflict を返却
> - 詳細は D-11「Vector API詳細設計書」セクション13.6を参照

### B. RADIUSクライアント設定 (Client Config)

パケット受信時にIPアドレスで照合し、共有秘密鍵を取得するためのデータ。

- **Key:** `client:{IP_ADDRESS}`
- **Type:** `Hash`

| **Field** | **必須** | **説明**       | **備考**                     |
| --------- | -------- | -------------- | ---------------------------- |
| `secret`  | Yes      | **共有秘密鍵** | Auth/Acct Serverが検証に使用 |
| `name`    | -        | クライアント名 | ログ出力用                   |
| `vendor`  | -        | ベンダー名     | VSA解析用 (例: cisco)        |

> ※利用時の優先順位に関する注意:
>
> パケット受信時は、まずこの client:{IP} を検索する。
>
> レコードが存在しない場合、フォールバックとして環境変数 RADIUS_SECRET の値を使用する。

### C. 認可ポリシー (Authorization Policy)

認証成功後(Post-Auth)に参照される、接続許可ルールおよびパラメータ設定。

- **Key:** `policy:{IMSI}`
- **Type:** `Hash`

| **Field** | **必須** | **説明**                  | **備考**              |
| --------- | -------- | ------------------------- | --------------------- |
| `rules`   | Yes      | **認可ルール (JSON配列)** | 詳細は後述            |
| `default` | Yes      | デフォルト動作            | `deny` または `allow` |

#### JSON構造 (`rules` フィールド)

NAS-IDとSSIDのマッチング条件に加え、VLAN・セッションパラメータを定義する。

```json
[
  {
    "nas_id": "AP-OFFICE-01",
    "allowed_ssids": ["CORP-WIFI", "GUEST-WIFI"],
    "vlan_id": "100",
    "session_timeout": 3600
  },
  {
    "nas_id": "*",
    "allowed_ssids": ["*"],
    "vlan_id": "",
    "session_timeout": 0
  }
]
```

| フィールド | 型 | 説明 | 備考 |
|-----------|-----|------|------|
| `nas_id` | string | NAS識別子 | ワイルドカード`*`可、完全一致 |
| `allowed_ssids` | []string | 許可SSIDリスト | `["*"]`で全SSID対象、大文字小文字区別なし |
| `vlan_id` | string | VLAN ID | 空文字は未設定、省略可 |
| `session_timeout` | int | セッションタイムアウト秒 | 0は未設定、省略可 |

------

## 3. ステートデータ (一時・動的)

認証・課金プロセスの中で自動的に生成・削除されるデータ群です。

### D. EAP認証コンテキスト (EAP Context)

IdentityフェーズからChallenge応答フェーズへ情報を持ち回るための一時データ。

**重要:** ここで使用するUUIDは、**ログのTrace ID**、**RADIUS State属性**、**Vector API連携用ヘッダ(X-Trace-ID)** として統一して利用する。

- **Key:** `eap:{UUID}`
- **UUIDフォーマット:** RFC 4122準拠、ハイフン含む36文字（例: `550e8400-e29b-41d4-a716-446655440000`）
- **生成:** `github.com/google/uuid` パッケージの `uuid.NewString()` を使用
- **Type:** `Hash`
- **TTL:** **60秒** (短い寿命)

| **Field** | **型** | **説明** |
| --------- | ------ | -------- |
| `imsi`    | String | 認証中のIMSI |
| `eap_type` | uint8 | EAP方式 (23=EAP-AKA, 50=EAP-AKA') |
| `stage`   | String | 認証フェーズ（D-03/D-09で定義された状態名、下記参照） |
| `rand`    | Hex | Vector Gatewayから取得したRAND (16 bytes) |
| `autn`    | Hex | Vector Gatewayから取得したAUTN (16 bytes) |
| `xres`    | Hex | Vector Gatewayから取得した期待値レスポンス (4-16 bytes) |
| `k_aut`   | Hex | MAC計算・検証用鍵 (EAP-AKA: 16 bytes, EAP-AKA': 32 bytes) |
| `msk`     | Hex | Master Session Key (64 bytes) |
| `resync_count` | int | 再同期試行回数（上限32回） |
| `permanent_id_requested` | bool | フル認証誘導済みフラグ |

> **セキュリティ方針（CK/IKの取り扱い）:**
> - Vector Gatewayから受信したCK/IKは、鍵導出処理の一時変数としてのみ使用する
> - 導出後の鍵（K_aut, MSK等）のみをEAPコンテキストに保存する
> - **CK/IKはValkeyに永続化しない**（セキュリティ上の理由）

> **eap_typeについて:**
> - RFC 3748で定義されるEAP Type番号
> - EAP-AKA: 23 (RFC 4187)
> - EAP-AKA': 50 (RFC 9048)
> - Identity解析時に先頭文字から決定し、以降の処理で参照

> **stageフィールドの値:**
> `pkg/model` パッケージで `type Stage string` として定義されている。D-03およびD-09で定義された以下の状態名を使用する。
>
> | 状態名 | model定数 | 説明 |
> |--------|-----------|------|
> | `new` | `StageNew` | 初期状態 |
> | `waiting_identity` | `StageWaitingIdentity` | AT_PERMANENT_ID_REQ送信済み、永続ID応答待ち |
> | `identity_received` | `StageIdentityReceived` | 永続ID受領済み |
> | `waiting_vector` | `StageWaitingVector` | Vector Gateway応答待ち |
> | `challenge_sent` | `StageChallengeSent` | Challenge送信済み |
> | `resync_sent` | `StageResyncSent` | 再同期処理中 |
> | `success` | `StageSuccess` | 認証成功（終了状態） |
> | `failure` | `StageFailure` | 認証失敗（終了状態） |

> **autnの保存理由:**
> - EAP-AKA'では、CK'/IK'導出にAUTNが必要（RFC 9048 Section 3.3）
> - Challenge応答検証時にAT_MAC計算で使用する場合がある

> **k_autについて:**
> - EAP-AKA: K_aut (16 bytes) - MKからPRFで導出
> - EAP-AKA': K_aut (32 bytes) - PRF'で導出、HMAC-SHA-256-128に使用

> **resync_countについて:**
> - SQN再同期（AKA-Synchronization-Failure）の試行回数をカウント
> - 上限32回（SQN INDフィールド1サイクル分）
> - 上限超過時は認証失敗として処理

> **permanent_id_requestedについて:**
> - 仮名ID(2,7)または高速再認証ID(4,8)を受信した場合にtrueをセット
> - AT_PERMANENT_ID_REQを送信してフル認証に誘導したことを示す
> - 永続ID応答後の通常フローへの継続時に参照

### E. アクティブセッション (Active Session)

認証完了から切断まで維持されるセッション情報。

RADIUS属性 Class にこのUUIDが格納される。

- **Key:** `sess:{UUID}`
- **Type:** `Hash`
- **TTL:** **24時間** (Start/Interim受信時にリセット)
- **UUIDフォーマット:** RFC 4122準拠、ハイフン含む36文字（例: `550e8400-e29b-41d4-a716-446655440000`）
- **生成:** Auth Serverが `github.com/google/uuid` パッケージで生成し、Class属性に格納
- **検証:** Acct Serverが `uuid.Parse()` でRFC 4122準拠を検証

> **Session UUIDとTrace IDの関係:**
> - Trace ID: EAP認証プロセス中の追跡用（`eap:{UUID}`のキー、数秒〜数十秒の寿命）
> - Session UUID: セッション管理用（`sess:{UUID}`のキー、Class属性、最大24時間の寿命）
> - 両者は独立して生成される別のUUID

| **Field**      | **説明**           | **更新タイミング**   |
| -------------- | ------------------ | -------------------- |
| `imsi`         | IMSI               | Auth Accept時        |
| `start_time`   | 接続開始時刻       | Acct Start時         |
| `nas_ip`       | NAS IPアドレス     | Auth/Acct時          |
| `client_ip`    | 端末IP (Framed-IP) | Acct Start/Interim時 |
| `acct_id`      | Acct-Session-Id    | Acct Start時         |
| `input_octets` | 受信通信量         | Acct Interim/Stop時  |
| `output_octets`| 送信通信量         | Acct Interim/Stop時  |

> **TTL超過時の動作：**
> - `sess:{UUID}` は24時間経過で自動削除される
> - Acct Server が Interim/Stop を受信した際に該当セッションが存在しない場合：
>   - 警告ログ出力（`ACCT_SESSION_EXPIRED`）
>   - Accounting-Response は返却（クライアント再送防止）
>   - データ欠損はログから追跡可能とする

### F. ユーザー検索インデックス (Index)

IMSIから現在のセッションIDを逆引きするためのセット。

- **Key:** `idx:user:{IMSI}`
- **Type:** `Set` (Members: `session_uuid`)

> **クリーンアップ方針:**
> - `idx:user:{IMSI}` はTTLなしのSetであり、Acct-Stop未達やセッションTTL切れでゴミが残る可能性がある
> - **クリーンアップは読み取り時（Admin TUI）に実施する**
>   - Session Detail画面でセッション一覧を取得する際、`sess:{UUID}` の存在を確認
>   - 存在しないUUIDは `SREM idx:user:{IMSI}` で自動削除
> - これにより、データ構造を変更せずにゴミを解消できる（PoCスコープの最小変更方針）
> - 詳細はD-07「Admin TUI詳細設計書【後半】」セクション6.10を参照

### G. Accounting重複検出キャッシュ (Duplicate Detection)

Acct Serverが重複パケットおよび順序異常を検出するためのキャッシュ。

- **Key:** `acct:seen:{Acct-Session-Id}`
- **Type:** `String`
- **TTL:** **24時間**

| **値** | **意味** |
| ------ | -------- |
| `start` | Acct-Start受信済み |
| `interim:{input}:{output}` | 最新Interim受信済み（通信量で重複判定） |
| `stop` | Acct-Stop受信済み |

> **重複・順序異常の検出ロジック:**
> - **Start重複:** 値が `start` または `interim:*` の状態で再度Startを受信 → `ACCT_DUPLICATE_START`
> - **Interim重複:** 同一の `interim:{input}:{output}` 値で再度Interimを受信 → `ACCT_DUPLICATE_START`
> - **StartなしでInterim:** キーが存在しない状態でInterimを受信 → `ACCT_SEQUENCE_ERR`、セッション新規作成
> - **Stop後にStart:** 値が `stop` の状態でStartを受信 → `ACCT_SEQUENCE_ERR`、セッション新規作成
> - **Stop重複:** 値が `stop` の状態で再度Stopを受信 → ログ出力なし、処理継続
>
> **設計意図:**
> - Acct-Session-IdはNASが生成する識別子であり、セッションUUID（Auth Server生成）とは独立
> - NASの再起動やネットワーク障害による再送パケットを適切に処理するため、24時間キャッシュを保持
> - 詳細は D-10「Acct Server詳細設計書」セクション5.6を参照

------

## 4. データアクセスフロー (処理ロジック)

方針変更（Shared Secret優先順位、Trace ID統合）を反映した、各サーバーの処理手順です。

### Auth Server (UDP 1812)

1. **受信時 (共通):**
   - 送信元IPで `client:{IP}` を検索。
   - **ヒット時:** その `secret` を使用。
   - **なし:** 環境変数 `RADIUS_SECRET` を使用（未設定なら破棄）。
2. **Identity 受信時:**
   - **Trace ID (UUID) 生成:** このUUIDをログの `trace_id` に設定。
   - **Identity種別判定:** 先頭文字とrealm有無で認証方式を判別。
     - 永続ID(0,6): 通常フロー継続、`eap_type`を決定
     - 仮名/再認証ID(2,4,7,8): AT_PERMANENT_ID_REQでフル認証誘導
     - 非対応(1,3,5,realmなし): EAP-Failure返却
   - Vector Gateway (`POST /api/v1/vector`) をコールしてベクター取得。
     - Header `X-Trace-ID` に上記UUIDを付与。
     - *Note: ここにサーキットブレーカーを実装し、Vector Gateway過負荷時はエラー応答する。*
   - **鍵導出処理:** Vector Gateway応答（RAND, AUTN, XRES, CK, IK）から K_aut, MSK を導出。
     - EAP-AKA: MK = SHA1(Identity|IK|CK) → PRF で K_encr, K_aut, MSK, EMSK 導出
     - EAP-AKA': CK', IK' = f(CK, IK, AUTN, Network Name) → PRF' で K_aut, MSK 等を導出
   - `eap:{UUID}` をキーとして、IMSI, eap_type, stage, rand, autn, xres, k_aut, msk を保存。
   - `Access-Challenge` (State=UUID) を返却。
3. **Challenge Response 受信時:**
   - State属性から `eap:{UUID}` を復元。
   - `k_aut` を使用して `AT_MAC` を検証。
   - `XRES` と `AT_RES` を比較検証。不一致なら Reject。
   - **【Post-Auth Policy Check】**
     - `policy:{IMSI}` を取得・パース。
     - RADIUSリクエスト内の `NAS-Identifier` / `Called-Station-Id`(SSID) とルールを照合。
     - **不一致:** `Access-Reject` を返却。
     - **一致:** `Access-Accept` を返却。
       - ルール内の `vlan_id` -> `Tunnel-Private-Group-Id` AVPへ。
       - ルール内の `session_timeout` -> `Session-Timeout` AVPへ。
       - `msk` から MS-MPPE-Recv-Key, MS-MPPE-Send-Key を生成。
       - `sess:{UUID}` の枠を作成し、Class属性にUUIDをセット。
4. **再同期要求受信時:**
   - `eap:{UUID}` から `resync_count` を取得・インクリメント。
   - **上限チェック（32回）:** 超過時は `Access-Reject` を返却。
   - Vector Gateway をResyncモードでコール。
   - 成功時は新しいVectorで鍵を再導出し、新しいChallengeを送信。

### Acct Server (UDP 1813)

1. **受信時 (共通):**
   - Auth Server同様、Valkey `client:{IP}` -> 環境変数 の順でSecretを特定し検証。
2. **Acct-Start / Interim 受信時:**
   - Class属性から `sess:{UUID}` を特定。
   - **セッション不在時:** 警告ログ出力（`ACCT_SESSION_NOT_FOUND` or `ACCT_SESSION_EXPIRED`）、処理継続。
   - 情報を保存/更新 (HSET)。TTL延長 (EXPIRE)。
   - `idx:user:{IMSI}` にUUIDを追加 (SADD)。
3. **Acct-Stop 受信時:**
   - `sess:{UUID}` を削除 (DEL)。
   - `idx:user:{IMSI}` からUUIDを削除 (SREM)。

### Admin TUI

1. **管理機能:**
   - `sub:{IMSI}`, `client:{IP}`, `policy:{IMSI}` の CRUD操作。
2. **モニタリング:**
   - `idx:user:{IMSI}` をスキャンして、特定ユーザーの通信状況(`sess:{UUID}`)を表示。

------

## 5. Go 構造体定義例 (最終版)

`pkg/model` パッケージで定義される構造体。model構造体はjsonタグのみを使用し、redisタグは付与しない。Valkey Hash⇔構造体の変換は各アプリの `internal/store/convert.go` で行う。

```go
// --- Master Data ---

type Subscriber struct {
    IMSI      string `json:"imsi"`       // 国際移動体加入者識別番号（15桁）
    Ki        string `json:"ki"`         // 秘密鍵（32文字16進数）
    OPc       string `json:"opc"`        // オペレータ定数（32文字16進数）
    AMF       string `json:"amf"`        // 認証管理フィールド（4文字16進数）
    SQN       string `json:"sqn"`        // シーケンス番号（12文字16進数）
    CreatedAt string `json:"created_at"` // 作成日時（RFC3339形式）
}

func NewSubscriber(imsi, ki, opc, amf, sqn, createdAt string) *Subscriber

type RadiusClient struct {
    IP     string `json:"ip"`     // クライアントIPアドレス
    Secret string `json:"secret"` // 共有シークレット
    Name   string `json:"name"`   // クライアント名（識別用）
    Vendor string `json:"vendor"` // ベンダー名（任意）
}

func NewRadiusClient(ip, secret, name, vendor string) *RadiusClient

type Policy struct {
    IMSI      string       `json:"imsi"`       // 加入者IMSI
    Default   string       `json:"default"`    // デフォルトアクション（"allow" or "deny"）
    RulesJSON string       `json:"rules_json"` // ルールのJSON文字列（Valkey保存用）
    Rules     []PolicyRule `json:"-"`          // パース済みルール（メモリ上のみ）
}

func NewPolicy(imsi, defaultAction string) *Policy
func (p *Policy) ParseRules() error        // RulesJSONをパースしてRulesに格納
func (p *Policy) EncodeRules() error       // RulesをJSON文字列にエンコード
func (p *Policy) IsAllowByDefault() bool   // デフォルトアクションが許可か判定

type PolicyRule struct {
    NasID          string   `json:"nas_id"`                    // NAS識別子（ワイルドカード可）
    AllowedSSIDs   []string `json:"allowed_ssids"`             // 許可SSIDリスト
    VlanID         string   `json:"vlan_id,omitempty"`         // VLAN ID（空文字は未設定）
    SessionTimeout int      `json:"session_timeout,omitempty"` // セッションタイムアウト秒（0は未設定）
}

// --- State Data ---

// Stage はEAP認証のステージを表す型
type Stage string

const (
    StageNew              Stage = "new"
    StageWaitingIdentity  Stage = "waiting_identity"
    StageIdentityReceived Stage = "identity_received"
    StageWaitingVector    Stage = "waiting_vector"
    StageChallengeSent    Stage = "challenge_sent"
    StageResyncSent       Stage = "resync_sent"
    StageSuccess          Stage = "success"
    StageFailure          Stage = "failure"
)

// EAPContext は認証中のEAPセッション情報を保持する。
// CK/IKは鍵導出後に破棄し、導出後の鍵（K_aut, MSK）のみを保存する。
type EAPContext struct {
    TraceID              string `json:"trace_id"`               // トレース識別子
    IMSI                 string `json:"imsi"`                   // 加入者IMSI
    EAPType              uint8  `json:"eap_type"`               // 23=EAP-AKA, 50=EAP-AKA'
    Stage                Stage  `json:"stage"`                  // 認証ステージ（Stage型）
    RAND                 string `json:"rand"`                   // Hex
    AUTN                 string `json:"autn"`                   // Hex (EAP-AKA'のCK'/IK'導出に必要)
    XRES                 string `json:"xres"`                   // Hex
    Kaut                 string `json:"kaut"`                   // Hex (MAC計算用鍵)
    MSK                  string `json:"msk"`                    // Hex (MS-MPPE-Key生成用)
    ResyncCount          int    `json:"resync_count"`           // 再同期試行回数
    PermanentIDRequested bool   `json:"permanent_id_requested"` // フル認証誘導済みフラグ
}

func NewEAPContext(traceID, imsi string, eapType uint8) *EAPContext

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

func NewSession(uuid, imsi, nasIP, clientIP, acctSessionID string, startTime int64) *Session
```

> **ストア層変換方式の補足:**
> - model構造体にはredisタグを付与しない設計としている
> - 各アプリケーションの `internal/store/convert.go` にて、`map[string]string` ⇔ model構造体の変換関数を実装
> - この方式により、model構造体がValkey実装に依存せず、テスタビリティを向上させている

------

## 改訂履歴

| 版数 | 日付 | 内容 |
|------|------|------|
| r1 | - | 初版 |
| r2 | - | Shared Secret優先順位、Trace ID統合 |
| r3 | 2025-12-30 | EAPコンテキストにresync_count/permanent_id_requestedフィールド追加、セッションTTL超過時の動作明記、Go構造体更新 |
| r4 | 2026-01-12 | EAPコンテキスト構造体の変更: eap_type/autn/k_aut/msk追加、ck/ik削除。CK/IK非保存方針（セキュリティ上の理由）を明記。セクション4のデータアクセスフローに鍵導出処理の説明追加。 |
| r5 | 2026-01-20 | Accounting重複検出キャッシュ追加: セクション1プレフィックス一覧に`acct:seen:`追加、セクション3にサブセクションG新設 |
| r6 | 2026-01-20 | UUID仕様明記: セクション3.D/3.EにUUIDフォーマット（RFC 4122準拠、36文字）・生成パッケージ・検証方法を追記。Session UUIDとTrace IDの関係を明記。セッションTTL更新タイミングを「Interim受信ごと」から「Start/Interim受信時にリセット」に修正。 |
| r7 | 2026-01-21 | stageフィールド値明記: セクション3.Dにstageフィールドで使用する状態名一覧を追記（D-03/D-09との整合性確保）。旧仕様値（"identity"/"challenge"）の非推奨を明記。 |
| r8 | 2026-01-26 | SQN競合制御明記: セクション2.Aの`sqn`フィールド備考にCAS更新を追記、WATCH/MULTIによる原子更新方式の補足説明を追加 |
| r9 | 2026-01-27 | idx:userクリーンアップ方針追記: セクション3 F項にAdmin TUIでの読み取り時クリーンアップ方針を明記 |
| r10 | 2026-02-18 | 実装との整合: PolicyDoc→Policy型名変更、PolicyRuleフィールド更新（SSID/Action/TimeMin/TimeMax）、rulesのJSONサンプル更新、stage値を小文字に変更しmodel.Stage型として定義されている旨を明記、Go構造体定義例からredisタグ除去（ストア層変換方式の補足追記）、全コンストラクタシグネチャ追記 |
| r11 | 2026-02-27 | PolicyRule構造を実装コードに合わせて修正: フィールドをNasID/AllowedSSIDs/VlanID/SessionTimeoutに変更、JSONサンプルをNAS-ID/SSIDマッチング＋VLAN・セッションパラメータ形式に更新、Go構造体定義例も同期 |
