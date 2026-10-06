# D-13 Provisioning API 詳細設計書 (r1)

**作成日:** 2026-10-07
**ステータス:** 設計中（実装前）

## 1. 概要

### 1.1 目的

本ドキュメントは、EAP-AKA RADIUS PoC環境に追加する「Provisioning API」（`provisioning-api`）の設計を定義する。Provisioning API は、Admin TUI が行っている加入者・RADIUSクライアント・認可ポリシーの CRUD 操作を、REST API として外部に提供する。

**主な責務:**

- 加入者（`sub:{IMSI}`）・RADIUSクライアント（`client:{IP}`）・認可ポリシー（`policy:{IMSI}`）の作成・参照・変更・削除
- 秘密の値（Ki / OPc、共有シークレット）の読み出しを専用の経路に限り、監査ログに記録する
- 変更操作の監査ログの出力
- mTLS によるクライアント（BFF 等）の認証

### 1.2 背景と位置づけ

本PoCのマスタデータは Admin TUI でしか操作できない。本PoCの外（同じホスト、または別のノード）に置く BFF（Backend for Frontend）から操作できるようにするため、Provisioning API を追加する。さらに将来は、別リポジトリの aka-only-server もあわせて操作する BFF や、両方への操作を統合する API サーバー（仮称 `eapaka-node-provisioner`）を開発する想定である。

```
現在（本書の対象）:
  [BFF（本PoCの外。同ホストでも別ノードでも可）]
       │ HTTPS + mTLS（X-Operator-Id 付き）
       ▼
  [provisioning-api]（本PoCのコンテナ）──→ Valkey（sub: / client: / policy:）
                                               ▲
  [Admin TUI]（既存・継続）────────────────────┘   ※原則として併用しない（§2.3）

将来:
  [BFF] ──→ [eapaka-node-provisioner]（統合API。別リポジトリ）
                 ├─→ provisioning-api（本PoC: RADIUSクライアント・認可ポリシー・加入者）
                 └─→ aka-only-server 管理API（/admin/v1: 加入者の鍵・AVクライアント）
```

Provisioning API は「1ノード分のマスタデータを操作する薄い API」とし、画面のロジックや、複数ノードにまたがる操作の調整は持たない。それらは BFF や `eapaka-node-provisioner` の責務とする。

> **注記（X-01 との関係）:** 2026-02 の拡張案 X-01（api-server と、管理者PCで SSH トンネルを張る admin-web）は、本書で置き換える。admin-web の代わりに本PoCの外の BFF を想定し、SSH トンネルの代わりに mTLS で保護する。

### 1.3 設計方針

| 方針 | 内容 |
|------|------|
| aka-only-server の管理API に作法を揃える | BFF と `eapaka-node-provisioner` が両方を同じクライアントの書き方で扱えるよう、パス（`/admin/v1`）、JSON の camelCase、JSON Merge Patch、ProblemDetails の `cause` / `invalidParams`、cursor によるページング、mTLS とフィンガープリントの固定、`X-Operator-Id` を揃える（§4） |
| Admin TUI と同じ規則 | 入力の検証・正規化と Valkey への書き込み（Lua スクリプトを含む）は、Admin TUI と同じ実装を共通ライブラリ（`pkg/`）として使う（§7.2） |
| SQN を巻き戻さない | SQN は認証のたびに Vector API が進めるため、加入者の変更は指定した項目だけを書き換える（JSON Merge Patch）。Ki / OPc / AMF だけを変えても SQN には触れない（§3.1） |
| 秘密の値は専用の経路だけで返す | Ki / OPc、共有シークレットは一覧・取得の応答に含めず、専用のエンドポイントでだけ返し、そのたびに監査ログに記録する（§3.4） |
| 1ノードの操作に専念 | 加入者を削除しても認可ポリシーは削除しない等、リソースをまたぐ連動はしない。連動は呼び出し側（BFF / provisioner）が行う |

### 1.4 スコープ

| 区分 | 内容 |
|------|------|
| 対象 | 加入者・RADIUSクライアント・認可ポリシーの CRUD、秘密の値の読み出し、状態取得（`/status`）、監査ログの出力、mTLS 認証 |
| 対象外（将来検討。§10） | 一括インポート / エクスポート、セッション（`sess:`）・統計の参照、監査ログの参照 API、操作者ごとの権限、IPv6 の RADIUSクライアント |

### 1.5 関連ドキュメント

| ドキュメント | 参照内容 |
|-------------|---------|
| D-01 設計仕様書 | システム構成、リポジトリ構成 |
| D-02 Valkeyデータ設計仕様書 | `sub:{IMSI}`、`client:{IP}`、`policy:{IMSI}` のデータ構造 |
| D-04 ログ仕様設計書 | ログ形式、Admin TUI の監査ログ（§3.5） |
| D-05 Admin TUI詳細設計書【前半】 | 入力の検証規則（§3〜§5） |
| D-08 インフラ設定・運用設計書 | Docker Compose、公開範囲 |
| `docs/openapi/provisioning-api.yaml` | 本APIの OpenAPI 定義（BFF との契約） |
| aka-only-server `docs/openapi/admin-api.yaml` | 作法を揃える相手の管理API |

---

## 2. アーキテクチャ

### 2.1 コンポーネント

| コンポーネント | 実行場所 | 役割 | 通信先 |
|---------------|---------|------|--------|
| provisioning-api | 本PoCの Docker Compose（新規） | REST API の提供、Valkey のマスタデータの操作、監査ログの出力 | Valkey（compose 内部ネットワーク） |
| BFF | 本PoCの外（同ホスト / 別ノード） | 画面、利用者の認証・権限、provisioning-api の呼び出し | provisioning-api（HTTPS + mTLS） |
| Admin TUI | ホストOS（既存） | TUI によるマスタデータの操作 | Valkey（127.0.0.1:6379） |

既存の Auth Server・Acct Server・Vector Gateway・Vector API には変更を加えない。provisioning-api が書き込むデータは、Admin TUI と同じキー・同じ形式（D-02）である。

### 2.2 公開範囲

| 配置 | 公開方法 |
|------|---------|
| 同じホストの BFF | `127.0.0.1` にだけ公開する（既定） |
| 別ノードの BFF | mTLS を前提に、公開するアドレスを `.env` の `PROVISIONING_API_BIND` で指定する（§8.1.2）。インターネットを経由する場合は、WireGuard 等で保護するか、クラウド側のファイアウォールで送信元を絞る |

mTLS により、登録されていないクライアント証明書の接続は TLS ハンドシェイクで拒否される（§5）。

### 2.3 Admin TUI との併用

Admin TUI と provisioning-api は、原則として同時に使わない（運用で使い分ける）。同時に使った場合の競合は許容する。競合時の動作は次のとおりで、データが壊れることはない。

| 操作 | 同時に使った場合 |
|------|----------------|
| 作成 | 作成は存在確認と書き込みを1回の操作（Lua スクリプト）で行うため、両方が同じキーを作ろうとしても、後の方は「既に存在する」エラーになる（上書きしない） |
| 変更・削除 | 後勝ち。変更は存在確認と書き込みを1回の操作で行うため、削除済みのキーを途中まで作り直すことはない |
| SQN | 加入者の変更で `sqn` を指定しなければ、SQN には触れない。Admin TUI の SQN の書き換えは、編集開始時の値との比較・置き換えで行う（D-02 §2.A） |

> **注記:** 現行の Admin TUI の作成処理は「存在確認→書き込み」の2回の操作で行っており、同時に作成すると後の方が上書きする。共通ライブラリに移すときに、Lua スクリプトによる1回の操作に改める（§7.2）。

---

## 3. リソースモデル

API のリソースと Valkey のキーの対応は次のとおり。

| リソース | Valkey | 識別子 | 秘密の値 |
|---------|--------|--------|---------|
| 加入者（Subscriber） | `sub:{IMSI}`（Hash） | IMSI | Ki、OPc |
| RADIUSクライアント（Client） | `client:{IP}`（Hash） | IPv4 アドレス | 共有シークレット |
| 認可ポリシー（Policy） | `policy:{IMSI}`（Hash） | IMSI | なし |

### 3.1 加入者

| API の項目 | Valkey のフィールド | 型・形式 | 作成時 | 応答 |
|-----------|-------------------|---------|--------|------|
| `imsi` | キー `sub:{IMSI}` | 数字15桁 | 必須 | 含む |
| `ki` | `ki` | 16進32桁 | 必須 | 含まない（`/keys` だけ） |
| `opc` | `opc` | 16進32桁 | 必須 | 含まない（`/keys` だけ） |
| `amf` | `amf` | 16進4桁 | 任意（既定 `8000`） | 含む |
| `sqn` | `sqn` | 16進12桁 | 任意（既定 `000000000000`） | 含む |
| `createdAt` | `created_at` | RFC 3339（UTC） | サーバーが設定 | 含む（ない場合は省略） |

- **変更（PATCH）:** JSON Merge Patch（RFC 7396）で、指定した項目（`ki` / `opc` / `amf` / `sqn`）だけを書き換える。`sqn` を指定しなければ SQN には触れない。`sqn` を指定した場合は、そのまま書き換える（比較・置き換えはしない。aka-only-server と同じ）。
- **削除:** `sub:{IMSI}` だけを削除する。同じ IMSI の認可ポリシー（`policy:{IMSI}`）、セッション（`sess:`）、インデックス（`idx:user:`）は削除しない。
- **応答に含めない値:** Ki と OPc は、一覧・取得・作成・変更の応答に含めない。読み出しは `GET /subscribers/{imsi}/keys` だけで行い、監査ログに記録する。
- **`updatedAt` はない:** Valkey に最終変更日時を持たないため、aka-only-server の `updatedAt` に相当する項目は返さない。
- **接続方式01との関係:** aka-only-server から認証ベクターを取得する加入者（接続方式 `01`）は、鍵を aka-only-server が持つため、本PoCの `sub:{IMSI}` は不要である（認可ポリシーは必要）。

### 3.2 RADIUSクライアント

| API の項目 | Valkey のフィールド | 型・形式 | 作成時 | 応答 |
|-----------|-------------------|---------|--------|------|
| `ip` | キー `client:{IP}` | IPv4 | 必須 | 含む |
| `secret` | `secret` | 印字可能ASCII（空白を除く）1〜128文字 | 必須 | 含まない（`/secret` だけ） |
| `name` | `name` | 英数字・`-`・`_` 1〜64文字 | 必須 | 含む |
| `vendor` | `vendor` | 英数字・空白・`-` 0〜64文字 | 任意（既定は空文字） | 含む |

- IP アドレスがそのまま識別子になる（aka-only-server の AVクライアントのような、サーバーが採番する ID は持たない）。IP アドレスの変更は「削除して作成」で行う。
- **変更（PATCH）:** `secret` / `name` / `vendor` のうち指定した項目だけを書き換える。
- **応答に含めない値:** 共有シークレットは、読み出しを `GET /clients/{ip}/secret` だけで行い、監査ログに記録する。

### 3.3 認可ポリシー

| API の項目 | Valkey のフィールド | 型・形式 | 必須 |
|-----------|-------------------|---------|------|
| `imsi` | キー `policy:{IMSI}` | 数字15桁 | パスで指定 |
| `default` | `default` | `allow` / `deny` | 必須 |
| `rules` | `rules`（JSON 配列の文字列） | 下表の配列（0件以上） | 必須（空配列可） |

`rules` の要素:

| API の項目 | Valkey の JSON のキー | 型・形式 | 必須 |
|-----------|---------------------|---------|------|
| `nasId` | `nas_id` | 印字可能ASCII 1〜253文字。`*` 単独は任意の NAS に一致 | 必須 |
| `allowedSsids` | `allowed_ssids` | 1〜32バイトの文字列の配列（`*` は任意の SSID。比較は大文字小文字を区別しない） | 必須（1件以上） |
| `vlanId` | `vlan_id` | 0〜4094 の数字の文字列（省略・空文字は未設定） | 任意 |
| `sessionTimeout` | `session_timeout` | 0〜86400（秒。省略・0 は未設定） | 任意 |

- 認可ポリシーは、加入者の下ではなく独立したリソースとする。接続方式01では `sub:{IMSI}` がなく `policy:{IMSI}` だけが存在するため（§3.1）。
- **作成・変更（PUT）:** ポリシー全体を置き換える（`default` と `rules` を1回の HSET で書き込む）。存在しなければ作成する（201）、存在すれば置き換える（200）。加入者（`sub:{IMSI}`）の有無は確認しない。
- API では `rules` を JSON の配列として扱い、Valkey へは D-02 の形式（snake_case のキーを持つ JSON 文字列）で保存する。

### 3.4 秘密の値の読み出し

| エンドポイント | 返す値 | 監査ログ |
|--------------|-------|---------|
| `GET /subscribers/{imsi}/keys` | `ki`、`opc` | 必ず記録する（値は記録しない） |
| `GET /clients/{ip}/secret` | `secret` | 必ず記録する（値は記録しない） |

そのほかの応答・ログ・監査ログには、秘密の値を含めない。

### 3.5 表記

| 項目 | 規則 |
|------|------|
| 16進文字列 | 大文字・小文字のどちらも受け付ける。Valkey へは Admin TUI と同じく大文字に正規化して保存し（D-02 §2.A）、応答では小文字で返す（aka-only-server と同じ） |
| 日時 | RFC 3339 形式の UTC |
| JSON の項目名 | camelCase（Valkey のフィールド名は snake_case のまま。DTO で変換する） |

---

## 4. API 仕様（概要）

詳細は OpenAPI 定義（`docs/openapi/provisioning-api.yaml`）による。

### 4.1 共通規約

| 項目 | 規約 |
|------|------|
| ベースURL | `https://{host}:9444/admin/v1` |
| 通信 | HTTPS（TLS 1.2 以上）、mTLS 必須（§5） |
| 要求の形式 | `application/json`。PATCH は `application/merge-patch+json`（RFC 7396。`null` は指定できず、1項目以上を指定する。配列は全体を置き換える） |
| 応答の形式 | `application/json`。エラーは `application/problem+json`（§4.3） |
| 作成 | `201 Created` と `Location` ヘッダー（相対パス）。既に存在すれば `409` |
| 削除 | `204 No Content`。存在しなければ `404` |
| 操作者 | 任意のヘッダー `X-Operator-Id`（`^[A-Za-z0-9._@-]{1,64}$`）を監査ログに記録する |
| トレース | 任意のヘッダー `X-Trace-ID` を受け取り、ログの `trace_id` に使う（ない場合は採番する）。応答の `X-Trace-ID` ヘッダーで返す |
| ページング | 加入者・認可ポリシーの一覧は `cursor` / `limit`（1〜500、既定50）/ `prefix`（IMSI の前方一致）。応答は `items`、`total`、次のページがあるときだけ `nextCursor`。IMSI の昇順 |

### 4.2 エンドポイント一覧

| メソッド | パス | 操作 | 成功 | 監査ログ |
|---------|------|------|------|---------|
| GET | `/status` | 状態の取得 | 200 | - |
| GET | `/subscribers` | 加入者の一覧 | 200 | - |
| POST | `/subscribers` | 加入者の登録 | 201 | create |
| GET | `/subscribers/{imsi}` | 加入者の取得 | 200 | - |
| PATCH | `/subscribers/{imsi}` | 加入者の変更 | 200 | update |
| DELETE | `/subscribers/{imsi}` | 加入者の削除 | 204 | delete |
| GET | `/subscribers/{imsi}/keys` | Ki / OPc の取得 | 200 | read |
| GET | `/clients` | RADIUSクライアントの一覧（全件） | 200 | - |
| POST | `/clients` | RADIUSクライアントの登録 | 201 | create |
| GET | `/clients/{ip}` | RADIUSクライアントの取得 | 200 | - |
| PATCH | `/clients/{ip}` | RADIUSクライアントの変更 | 200 | update |
| DELETE | `/clients/{ip}` | RADIUSクライアントの削除 | 204 | delete |
| GET | `/clients/{ip}/secret` | 共有シークレットの取得 | 200 | read |
| GET | `/policies` | 認可ポリシーの一覧 | 200 | - |
| GET | `/policies/{imsi}` | 認可ポリシーの取得 | 200 | - |
| PUT | `/policies/{imsi}` | 認可ポリシーの作成・置き換え | 201 / 200 | create / update |
| DELETE | `/policies/{imsi}` | 認可ポリシーの削除 | 204 | delete |

RADIUSクライアントの一覧は件数が少ないため、ページングせず全件を IP アドレスの順（数値として比較）で返す（aka-only-server の AVクライアントの一覧と同じ）。

### 4.3 エラー

ProblemDetails（RFC 7807）の `title` / `status` / `detail` に、aka-only-server と同じく 3GPP 風の `cause` と、項目ごとの理由 `invalidParams`（`param`, `reason`）を加える。

| HTTP | `cause` | 条件 |
|------|---------|------|
| 400 | `INVALID_MSG_FORMAT` | JSON として解釈できない、Content-Type が違う |
| 400 | `INVALID_QUERY_PARAM` | クエリパラメーターの値が不正 |
| 400 | `MANDATORY_IE_MISSING` | 必須項目がない |
| 400 | `MANDATORY_IE_INCORRECT` | 必須項目の値が不正（パスの IMSI / IP を含む） |
| 400 | `OPTIONAL_IE_INCORRECT` | 任意項目の値が不正 |
| 404 | `USER_NOT_FOUND` | 加入者が存在しない |
| 404 | `CLIENT_NOT_FOUND` | RADIUSクライアントが存在しない |
| 404 | `POLICY_NOT_FOUND` | 認可ポリシーが存在しない |
| 409 | `SUBSCRIBER_ALREADY_EXISTS` | 同じ IMSI の加入者が既に存在する |
| 409 | `CLIENT_ALREADY_EXISTS` | 同じ IP の RADIUSクライアントが既に存在する |
| 500 | `SYSTEM_FAILURE` | Valkey のエラー等、サーバー内部のエラー |

- `POLICY_NOT_FOUND`、`CLIENT_ALREADY_EXISTS` は aka-only-server にはない、本APIで追加する値である。
- 既存の `pkg/httputil.ProblemDetail`（Vector API が使用）は変えず、本API用の型を別に用意する（`type` は出力しない。aka-only-server と同じ）。

### 4.4 状態（`/status`）

| 項目 | 内容 |
|------|------|
| `version` | provisioning-api のバージョン |
| `nodeName` | ノードの識別名（環境変数 `PROVISIONING_API_NODE_NAME`。BFF / provisioner がノードを区別するため） |
| `startedAt` | 起動日時 |
| `subscriberCount` / `clientCount` / `policyCount` | 各キーの件数 |

Valkey に接続できない場合は `500`（`SYSTEM_FAILURE`）を返す。

---

## 5. 認証・認可

| 項目 | 方式 |
|------|------|
| サーバー証明書 | ファイルで指定する（`PROVISIONING_API_TLS_CERT` / `PROVISIONING_API_TLS_KEY`）。自己署名でもよい（クライアント側で証明書を固定する） |
| クライアント認証 | mTLS 必須。クライアント証明書の SHA-256 フィンガープリント（DER のハッシュ）を、環境変数 `PROVISIONING_API_ADMIN_CLIENTS`（`name=fingerprint` のカンマ区切り）で固定する。CA による検証は行わない（aka-only-server と同じ） |
| 未登録の証明書 | 証明書なし、または登録されていない証明書の接続は、TLS ハンドシェイクで拒否する（HTTP の応答は返さない） |
| 権限 | 登録されたクライアントはすべて全権限とする。利用者・権限の管理は BFF が行う |
| 操作者 | BFF はログインした利用者の ID を `X-Operator-Id` で渡す。監査ログには、操作者 ID と、クライアント証明書に対応する識別名（`name`）の両方を記録する |

> **注記:** web-gui-for-aka-only-server の aka-only-server 管理API 用クライアント（mTLS、サーバー証明書の固定）と同じ方式で接続できる。

---

## 6. ログ・監査

### 6.1 アプリケーションログ

他のコンポーネントと同じく `log/slog` の JSON を標準出力に出し、fluent-bit で収集する（`app` は `provisioning-api`）。

| ログ | 内容 |
|------|------|
| 起動・停止 | 起動時に `listen_addr`、`log_level`、`node_name`、登録クライアント数を出す（フィンガープリントは出さない） |
| `request completed` | メソッド、パス、`http_status`、`latency_ms`、`trace_id`、クライアントの識別名。パスに含まれる IMSI は `LOG_MASK_IMSI` に従ってマスクする |
| エラー | Valkey のエラー等（500 を返す場合） |

### 6.2 監査ログ

変更操作と秘密の値の読み出しを、Admin TUI の監査ログ（D-04 §3.5）と同じ形式で記録する（`event_id` は `AUDIT_LOG`、`app` は `provisioning-api`）。

| 項目 | 内容 |
|------|------|
| `operation` | `create` / `update` / `delete` / `read`（秘密の値の読み出し） |
| `target_type` | `subscriber` / `client` / `policy` |
| `target_key` | Valkey のキー（`sub:{IMSI}` 等） |
| `target_imsi` | 加入者・認可ポリシーの IMSI（生値。Admin TUI と同じ） |
| `admin_user` | `X-Operator-Id` の値（省略時は空文字） |
| `mgmt_client` | クライアント証明書に対応する識別名（本APIで追加する項目） |
| `details` | 変更した項目（加入者の SQN / AMF、ポリシーの `default` 等は変更前後の値。Ki / OPc / 共有シークレットは「変更あり」だけで値は含めない）。読み出しでは読み出した項目名（`ki,opc` / `secret`） |

- 失敗した操作（400 / 404 / 409 / 500）は監査ログに記録しない（`request completed` には記録する）。
- 監査ログの参照 API は本書の対象外とする（ホストOSのログファイルを参照する）。

### 6.3 D-04 への反映

実装時に、D-04 に Provisioning API のログ（§3.x を新設）と監査ログの項目（`mgmt_client`、`operation` の `read`）を追加し、lnav フォーマットの対象ファイルに `provisioning-api.log` を加える。

---

## 7. 実装構成

### 7.1 アプリケーション

```
apps/provisioning-api/
├── main.go                 # 起動、設定読み込み、TLS サーバー
├── Dockerfile
├── go.mod
└── internal/
    ├── config/             # 環境変数（envconfig）
    ├── auth/               # mTLS のフィンガープリント照合、識別名の取得
    ├── server/             # ルーター（Gin）、ミドルウェア（トレース、ログ、Content-Type）
    ├── handler/            # HTTP ハンドラー
    ├── dto/                # 要求・応答の JSON（camelCase）、ProblemDetails
    ├── service/            # 検証・正規化、ストア呼び出し、監査ログ
    └── audit/              # 監査ログの出力
```

- Vector API と同じく Gin と envconfig を使う（D-11）。新しい外部パッケージは追加しない。
- ハンドラーは DTO と HTTP の変換だけを行い、検証・正規化・監査ログは service 層で行う。

### 7.2 共通ライブラリへの移動

Admin TUI と provisioning-api が同じ検証規則と同じ Valkey 操作を使うよう、Admin TUI の次の実装を `pkg/` に移す（CLAUDE.md の「各 app で重複実装せず pkg を使う」方針）。

| 移動元（Admin TUI） | 移動先 | 内容 |
|-------------------|-------|------|
| `internal/validation`（加入者・クライアント・ポリシー） | `pkg/validation` | 検証規則、正規化（16進の大文字化等） |
| `internal/store` の加入者・クライアント・ポリシー（`subscriber.go`、`client.go`、`policy.go`、`keys.go` の該当部分） | `pkg/masterdata` | Valkey の読み書き（Lua スクリプトを含む） |

- セッション・統計のストア（`session.go`、`statistics.go`）と CSV は Admin TUI に残す。
- 移すときに、作成（加入者・クライアント・ポリシー）と変更（クライアント）を Lua スクリプトによる1回の操作に改める（§2.3）。加入者の変更は、指定した項目だけを書き換える操作（PATCH 用）を追加する。
- Admin TUI の動作は変えない（既存のテストで確認する）。移動は provisioning-api の実装より前に、単独の変更として行う。

---

## 8. 設定

### 8.1 環境変数

#### 8.1.1 コンテナに渡す環境変数

provisioning-api のプロセスが読む環境変数。

| 環境変数 | 既定値 | 必須 | 説明 |
|---------|-------|------|------|
| `PROVISIONING_API_LISTEN_ADDR` | `:9444` | - | 待ち受けアドレス（aka-only-server の管理API の 9443 と重ならない番号） |
| `PROVISIONING_API_TLS_CERT` | - | ○ | サーバー証明書（PEM）のパス |
| `PROVISIONING_API_TLS_KEY` | - | ○ | サーバー証明書の秘密鍵（PEM）のパス |
| `PROVISIONING_API_ADMIN_CLIENTS` | - | ○ | 管理クライアントの `name=fingerprint`（SHA-256、16進）のカンマ区切り。空なら起動しない |
| `PROVISIONING_API_NODE_NAME` | ホスト名 | - | `/status` の `nodeName`。コンテナでは既定のホスト名がコンテナIDになるため、§8.1.2 の `.env` で指定することを推奨する |
| `REDIS_HOST` / `REDIS_PORT` / `REDIS_PASS` | `valkey` / `6379` / - | - | Valkey の接続先（他のコンポーネントと同じ） |
| `LOG_LEVEL` | `INFO` | - | ログレベル（`pkg/logging.ParseLevel`） |
| `LOG_MASK_IMSI` | `true` | - | アプリケーションログの IMSI マスク（監査ログは生値） |

#### 8.1.2 `.env` で設定する変数（Docker Compose が使う）

`deployments/.env` に書き、Docker Compose が `docker-compose.yml` の展開に使う変数。実装時に `.env.example`（D-08 §4.2）に追加する。

| 変数 | 既定値 | 使い道 |
|------|-------|--------|
| `PROVISIONING_API_BIND` | `127.0.0.1` | ホスト側で 9444/tcp を公開するアドレス（compose の `ports` の展開にだけ使い、コンテナには渡さない）。別ノードの BFF から使う場合は、受け付けるインターフェースのアドレス（WireGuard のアドレス等）または `0.0.0.0` にする |
| `PROVISIONING_API_ADMIN_CLIENTS` | （空） | そのままコンテナの `PROVISIONING_API_ADMIN_CLIENTS` に渡す（§8.1.1。空なら provisioning-api は起動しない） |
| `PROVISIONING_API_NODE_NAME` | （空） | そのままコンテナの `PROVISIONING_API_NODE_NAME` に渡す（空ならコンテナのホスト名） |
| `VALKEY_PASSWORD` | - | コンテナの `REDIS_PASS` に渡す（他のコンポーネントと同じ） |
| `LOG_LEVEL` / `LOG_MASK_IMSI` | `INFO` / `true` | 他のコンポーネントと同じく、コンテナに渡す |

compose では、`PROVISIONING_API_LISTEN_ADDR` は既定値（`:9444`）のまま、`PROVISIONING_API_TLS_CERT` / `PROVISIONING_API_TLS_KEY` はマウントした証明書のパス（`/certs/server.pem` / `/certs/server.key`）に固定する。

### 8.2 Docker Compose

- `provisioning-api` サービスを追加する。証明書の準備が要るため、compose の profile（`provisioning`）に入れ、既定の `docker compose up` では起動しない。
- 公開は `${PROVISIONING_API_BIND:-127.0.0.1}:9444:9444/tcp`。別ノードの BFF から使う場合だけ `.env` の `PROVISIONING_API_BIND` を変える（§8.1.2）。
- 証明書は `deployments/certs/provisioning/` をコンテナの `/certs` に読み取り専用でマウントする。
- ヘルスチェックはプロセスの確認（`pgrep`）とする（`/status` は mTLS が要るため）。
- ログは他のサービスと同じく fluent-bit に送り、`provisioning-api.log` に出力する。

---

## 9. テスト方針

| レベル | 内容 |
|-------|------|
| 単体（`pkg/validation`、`pkg/masterdata`） | Admin TUI から移した既存のテストに加え、原子的な作成・変更（同じキーの同時作成で片方が 409 になる等）と PATCH 用の変更を miniredis で確認する |
| 単体（provisioning-api） | ハンドラー・service を httptest と miniredis で確認する（各エンドポイントの正常系・異常系、`cause`、`invalidParams`、秘密の値が応答・ログに出ないこと、監査ログの内容）。mTLS は `httptest.NewUnstartedServer` に TLS を設定して、登録済み・未登録・証明書なしの接続を確認する |
| 結合（simwifi 実機） | compose で起動し、curl とクライアント証明書で全エンドポイントを操作する。provisioning-api で登録した加入者・ポリシー・RADIUSクライアントで eapaka_test の認証が通ること、Admin TUI で同じデータが見えること、Admin TUI で登録したデータを API で読めることを確認する |

カバレッジは他のアプリと同じく 80% 以上を目標とする（T-01）。

---

## 10. 将来拡張

| 項目 | 内容 |
|------|------|
| eapaka-node-provisioner | 加入者を「IMSI＋鍵の置き場所（本PoCの Vector API / aka-only-server）＋認可ポリシー」として扱い、本APIと aka-only-server の管理API を組み合わせて操作する。複数ノードへの操作の失敗時は、補償（作成したものを消す等）で戻す。本APIは、作成の 409、ポリシーの PUT（置き換え）により、やり直しやすい形にしておく |
| 一括操作 | Admin TUI の CSV インポート / エクスポートに相当する操作 |
| 参照系 | セッション（`sess:`）・統計の参照（読み取りのみ） |
| 監査ログの参照 | aka-only-server の `/audit-logs` に相当する API（監査ログを Valkey Stream 等に保存する必要がある） |
| 権限 | 管理クライアントごとの読み取り専用等 |
| IPv6 | RADIUSクライアントの IPv6 アドレス（Admin TUI・Auth Server を含めた対応が必要） |

---

## 11. 実装ステップ

1. 本書と OpenAPI 定義の作成（本書 r1）
2. 共通ライブラリへの移動（§7.2。`pkg/validation`、`pkg/masterdata`。Admin TUI の動作は変えない）
3. provisioning-api の実装（§4〜§8）、D-01 / D-02 / D-04 / D-08 / T-02 等の更新、simwifi での結合確認
4. 将来拡張（§10）は別途検討

---

## 改版履歴

| 版数 | 日付 | 内容 |
|------|------|------|
| r1 | 2026-10-07 | 初版作成。Admin TUI の加入者・RADIUSクライアント・認可ポリシーの CRUD を REST API として提供する provisioning-api の設計（位置づけ、リソースモデル、aka-only-server の管理API に揃えた作法、秘密の値の読み出しと監査、mTLS 認証、ログ・監査、共通ライブラリへの移動、設定、テスト方針、将来拡張）。拡張案 X-01 を置き換える |
