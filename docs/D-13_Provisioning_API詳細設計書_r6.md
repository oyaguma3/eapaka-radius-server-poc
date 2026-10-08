# D-13 Provisioning API 詳細設計書 (r6)

**作成日:** 2026-10-07
**更新日:** 2026-10-09
**ステータス:** 実装済み（`apps/provisioning-api`。simwifi 実機での結合確認済み。§9）

## 1. 概要

### 1.1 目的

本ドキュメントは、EAP-AKA RADIUS PoC環境に追加する「Provisioning API」（`provisioning-api`）の設計を定義する。Provisioning API は、Admin TUI が行っている加入者・RADIUSクライアント・認可ポリシーの CRUD 操作を、REST API として外部に提供する。

**主な責務:**

- 加入者（`sub:{IMSI}`）・RADIUSクライアント（`client:{IP}`）・認可ポリシー（`policy:{IMSI}`）の作成・参照・変更・削除
- 秘密の値（Ki / OPc、共有シークレット）の読み出しを専用の経路に限り、監査ログに記録する
- 変更操作の監査ログの出力と、その参照（r6）
- アクティブセッションの参照（読み取りだけ。r6）
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
| 対象 | 加入者・RADIUSクライアント・認可ポリシーの CRUD、秘密の値の読み出し、状態取得（`/status`。セッション数を含む）、監査ログの出力と参照（`/audit-logs`。r6）、アクティブセッションの参照（`/sessions`。r6）、mTLS 認証 |
| 対象外 | 一括インポート / エクスポート（CSV。Admin TUI だけで行う。§10）、セッションの切断、操作者ごとの権限、IPv6 の RADIUSクライアント（将来検討。§10） |

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
| 同じホストの BFF（別の compose） | 共有の Docker ネットワーク（既定名 `eapaka-prov`。§8.2）に BFF が参加し、`https://provisioning-api:9444/admin/v1` で接続する。ホストへの公開は `127.0.0.1` のまま（ホストからの curl 等の確認用） |
| 同じホストのプロセス（コンテナでないもの） | `127.0.0.1:9444`（既定の公開先）に接続する |
| 別ノードの BFF | mTLS を前提に、公開するアドレスを `.env` の `PROVISIONING_API_BIND` で指定する（§8.1.2）。インターネットを経由する場合は、WireGuard 等で保護するか、クラウド側のファイアウォールで送信元を絞る |

mTLS により、登録されていないクライアント証明書の接続は TLS ハンドシェイクで拒否される（§5）。

> **注記（共有ネットワーク）:** コンテナの `127.0.0.1` はそのコンテナ自身を指すため、別の compose で動く BFF のコンテナからは、ホストの `127.0.0.1:9444` に届かない。aka-only-server と web-gui-for-aka-only-server の共有ネットワーク（`aka-av`）と同じく、サーバー側の本PoCがネットワークを作り、BFF が external として参加する形にした（2026-10-08）。共有ネットワークに参加するのは provisioning-api だけで、BFF から Valkey 等の他のサービスには名前解決も接続もできない（simwifi で確認）。BFF はホスト名を検証するため、サーバー証明書の SAN に `DNS:provisioning-api` が要る。

### 2.3 Admin TUI との併用

Admin TUI と provisioning-api は、原則として同時に使わない（運用で使い分ける）。同時に使った場合の競合は許容する。競合時の動作は次のとおりで、データが壊れることはない。

| 操作 | 同時に使った場合 |
|------|----------------|
| 作成 | 作成は存在確認と書き込みを1回の操作（Lua スクリプト）で行うため、両方が同じキーを作ろうとしても、後の方は「既に存在する」エラーになる（上書きしない） |
| 変更・削除 | 後勝ち。変更は存在確認と書き込みを1回の操作で行うため、削除済みのキーを途中まで作り直すことはない |
| SQN | 加入者の変更で `sqn` を指定しなければ、SQN には触れない。Admin TUI の SQN の書き換えは、編集開始時の値との比較・置き換えで行う（D-02 §2.A） |

> **注記:** 2026-10-07 より前の Admin TUI の作成・変更は「存在確認→書き込み」の2回の操作で行っており、同時に作成すると後の方が上書きしていた。共通ライブラリに移すときに、Lua スクリプトによる1回の操作に改めた（§7.2、E-03 §9.3）。

---

## 3. リソースモデル

API のリソースと Valkey のキーの対応は次のとおり。

| リソース | Valkey | 識別子 | 秘密の値 |
|---------|--------|--------|---------|
| 加入者（Subscriber） | `sub:{IMSI}`（Hash） | IMSI | Ki、OPc |
| RADIUSクライアント（Client） | `client:{IP}`（Hash）。索引 `idx:client:{ID}`、カウンター `seq:client` | サーバー採番の ID（IP アドレスは変更できる属性） | 共有シークレット |
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
| `id` | `id`（索引 `idx:client:{ID}` → IP） | 1 からの整数（int64） | サーバーが採番 | 含む |
| `ip` | キー `client:{IP}` | IPv4 | 必須（PATCH で変更可） | 含む |
| `secret` | `secret` | 印字可能ASCII（空白を除く）1〜128文字 | 必須 | 含まない（`/secret` だけ） |
| `name` | `name` | 英数字・`-`・`_` 1〜64文字 | 必須 | 含む |
| `vendor` | `vendor` | 英数字・空白・`-` 0〜64文字 | 任意（既定は空文字） | 含む |

- **識別子:** サーバーが採番する ID（1 からの連番。再利用しない）で識別し、パスは `/clients/{clientId}` とする（aka-only-server の AVクライアントと同じ）。IP アドレスは識別子ではなく、変更できる属性として扱う。
  - Valkey のキーは従来どおり `client:{IP}` で、Auth / Acct Server は送信元IPでこれを直接引く（変更なし）。ID はその Hash の `id` フィールドに持ち、ID から IP を引く索引 `idx:client:{ID}`（String）と、採番のカウンター `seq:client`（INCR）を使う（D-02）。`client:*` の SCAN に混ざらないよう、索引とカウンターは `client:` で始めない。
  - 同じ IP のクライアントは1件だけである（RADIUS では送信元IPで共有シークレットを決めるため）。
  - 作成（採番と索引の作成）、IP の変更（キーの付け替えと索引の更新）、削除（索引の削除）は、それぞれ Lua スクリプトで1回の操作として行う（E-03 §9）。
  - ID の導入前に登録されたクライアント（`id` を持たない）には、provisioning-api と Admin TUI の起動時に ID を採番する（`ClientStore.EnsureIDs`。何度実行しても結果は同じ。`seq:client` が既存の最大の ID より小さければ合わせる）。
  - `GET /clients?ip={IP}` で、IP アドレスから探せる（0 件または 1 件）。
  - 2026-10-08 までの版（r4 / API 0.1.0）は IP アドレスをそのまま識別子とし、パスに使っていた。IP は NAS の位置を示す変わり得る属性で、変更が「削除して作成」になり、IPv6 の長い表記やサブネット単位の登録（`/` を含む）をパスに入れにくいため、ID に改めた（FreeRADIUS も、クライアントの定義に名前を付け、IP を照合の属性として持つ）。
- IP アドレスは、`pkg/validation` の規則（Admin TUI と同じ）に加え、表記が一意であること（各オクテットの先頭に `0` を付けない）を求める。Auth Server は送信元IPの文字列表記で `client:{IP}` を引くため、`192.168.010.1` のような表記で登録すると一致しない（作成では `MANDATORY_IE_INCORRECT`、PATCH では `OPTIONAL_IE_INCORRECT`、`?ip=` では `INVALID_QUERY_PARAM`）。パスの `clientId` が 1 以上の整数でなければ `MANDATORY_IE_INCORRECT`。
- **変更（PATCH）:** `ip` / `secret` / `name` / `vendor` のうち指定した項目だけを書き換える。`ip` を変えると、キーを `client:{新しいIP}` に付け替える（ID は変わらない。Auth / Acct Server は新しい IP で引くようになる）。他のクライアントが使っている IP には変更できない（409 `CLIENT_ALREADY_EXISTS`）。
- **応答に含めない値:** 共有シークレットは、読み出しを `GET /clients/{clientId}/secret` だけで行い、監査ログに記録する。

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
| `vlanId` | `vlan_id` | 0〜4094 の数字だけの文字列（省略・空文字は未設定。`+5` 等は不可） | 任意 |
| `sessionTimeout` | `session_timeout` | 0〜86400（秒。省略・0 は未設定） | 任意 |

- 認可ポリシーは、加入者の下ではなく独立したリソースとする。接続方式01では `sub:{IMSI}` がなく `policy:{IMSI}` だけが存在するため（§3.1）。
- **作成・変更（PUT）:** ポリシー全体を置き換える（`default` と `rules` を1回の HSET で書き込む）。存在しなければ作成する（201）、存在すれば置き換える（200）。加入者（`sub:{IMSI}`）の有無は確認しない。
- API では `rules` を JSON の配列として扱い、Valkey へは D-02 の形式（snake_case のキーを持つ JSON 文字列）で保存する。
- 正規化は Admin TUI と同じ（`pkg/validation.NormalizePolicyInput`）: `default` は小文字にし（`ALLOW` も受け付ける）、`nasId` と各 SSID は前後の空白を除く。
- `rules` の要素のうち、`nasId` / `allowedSsids` の欠落は `MANDATORY_IE_MISSING`、値の不正（`allowedSsids` が空の配列を含む）は `MANDATORY_IE_INCORRECT`、`vlanId` / `sessionTimeout` の不正は `OPTIONAL_IE_INCORRECT` とする。`invalidParams` の `param` は `rules[0].allowedSsids[1]` のように位置を示す。

### 3.4 秘密の値の読み出し

| エンドポイント | 返す値 | 監査ログ |
|--------------|-------|---------|
| `GET /subscribers/{imsi}/keys` | `ki`、`opc` | 必ず記録する（値は記録しない） |
| `GET /clients/{clientId}/secret` | `secret` | 必ず記録する（値は記録しない） |

そのほかの応答・ログ・監査ログには、秘密の値を含めない。

### 3.5 表記

| 項目 | 規則 |
|------|------|
| 16進文字列 | 大文字・小文字のどちらも受け付ける。Valkey へは Admin TUI と同じく大文字に正規化して保存し（D-02 §2.A）、応答では小文字で返す（aka-only-server と同じ） |
| 日時 | RFC 3339 形式の UTC |
| JSON の項目名 | camelCase（Valkey のフィールド名は snake_case のまま。DTO で変換する） |

### 3.6 監査ログ（r6）

本APIの監査ログ（§6.2）を、`GET /audit-logs` で新しい順に返す。BFF の画面や `eapaka-node-provisioner` が、ホストのログファイルを読まずに本APIの操作を追えるようにするためである。

- **保存先:** 監査ログは従来どおり標準出力に出し（fluent-bit が `provisioning-api.log` に書く。これが正本）、あわせて Valkey の Stream `audit:prov`（D-02 §2.H）に保存する。件数の上限は `PROVISIONING_API_AUDIT_MAX`（既定 10000。§8.1.1）で、超えた古いものから消す（`XADD MAXLEN ~`。おおよその上限）。
- **保存の失敗:** Stream への保存に失敗しても、操作自体は成功として扱い、`PROV_AUDIT_STORE_ERR`（ERROR）を記録する（§6.1）。ログファイルには残るので、そちらで確かめる。
- **対象:** 本APIの操作だけである。Admin TUI の操作は含まない（Admin TUI の監査ログはログファイルだけ。D-04 §3.5）。
- **作法:** aka-only-server の管理API の `GET /audit-logs`（`before` / `limit`、`nextBefore`、新しい順）に揃える。

| API の項目 | 内容 | Stream のフィールド |
|-----------|------|-------------------|
| `id` | エントリID（Stream のID。`before` に渡せる） | （ID） |
| `time` | 記録日時（ミリ秒まで。エントリID から求める） | （ID） |
| `operator` | 操作者（`X-Operator-Id`。省略された操作では空文字） | `admin_user` |
| `mgmtClient` | 管理クライアントの識別名 | `mgmt_client` |
| `action` | 操作（下表） | `target_type` と `operation` から求める |
| `target` | 対象。加入者・認可ポリシーは IMSI、RADIUSクライアントは ID | `target_imsi` / `target_id` |
| `targetKey` | 対象の Valkey のキー | `target_key` |
| `traceId` | トレースID（ログファイルの `trace_id` と同じ） | `trace_id` |
| `details` | 変更内容（ログファイルの `details` と同じ。秘密の値は含まない。ない場合は省略） | `details` |

`action` は aka-only-server と同じ命名にする: `subscriber.create` / `subscriber.update` / `subscriber.delete` / `subscriber.keys.read`、`client.create` / `client.update` / `client.delete` / `client.secret.read`、`policy.create` / `policy.update` / `policy.delete`。

### 3.7 セッション（r6）

アクティブセッション（`sess:{UUID}`。D-02 §3.E）を、`GET /sessions` で読み取りだけで返す（Admin TUI のセッション一覧・検索と同じ情報。D-07 §5・§6）。

| API の項目 | Valkey のフィールド | 内容 |
|-----------|-------------------|------|
| `id` | キー `sess:{UUID}` | セッションの UUID（RADIUS の Class 属性の値） |
| `imsi` | `imsi` | IMSI（生値） |
| `nasIp` | `nas_ip` | NAS の IP アドレス |
| `nasIdentifier` | `nas_identifier` | NAS-Identifier（ない場合は空文字） |
| `startTime` | `start_time`（Unix 秒） | 接続開始日時（RFC 3339、UTC。値がなければ省略） |
| `clientIp` | `client_ip` | 端末の IP アドレス（Accounting-Request を受けるまでは空文字） |
| `acctSessionId` | `acct_id` | Acct-Session-Id |
| `inputOctets` / `outputOctets` | `input_octets` / `output_octets` | 通信量（Interim で更新） |

- **並び順と件数:** 接続開始の新しい順（同じ時刻は UUID の順）。`limit`（1〜1000、既定 100）件まで返し、`total` に条件に一致する件数を返す。セッションの件数は少ない前提で、ページング（cursor）はしない。
- **IMSI での絞り込み:** `?imsi=` を指定すると、`idx:user:{IMSI}` で引く。索引が空なら、全セッションを SCAN して絞り込む（Admin TUI と同じ）。索引に残った、もう存在しないセッションの UUID は結果に含めず、**索引の掃除（SREM）はしない**（読み取りだけの API とし、掃除は Admin TUI が行う。D-02 §3.F）。
- **件数:** `/status` の `sessionCount` に、`sess:*` の件数（SCAN）を返す（Admin TUI の統計と同じ）。
- **読み出しの実装:** Admin TUI のセッションの読み出しを `pkg/masterdata` の `SessionStore` に移し、両方で使う（§7.2）。
- セッションの切断（RADIUS の Disconnect / CoA）は扱わない（Admin TUI にもない）。

---

## 4. API 仕様（概要）

詳細は OpenAPI 定義（`docs/openapi/provisioning-api.yaml`）による。

### 4.1 共通規約

| 項目 | 規約 |
|------|------|
| ベースURL | `https://{host}:9444/admin/v1` |
| 通信 | HTTPS（TLS 1.2 以上）、mTLS 必須（§5） |
| 要求の形式 | `application/json`。PATCH は `application/merge-patch+json`（RFC 7396。`null` は指定できず、1項目以上を指定する。配列は全体を置き換える）。PATCH は `application/json` も受け付ける。`charset` 等のパラメーターは無視する。本文の上限は 256KiB |
| 要求の解釈 | 未知の項目、型の違う値、JSON の後ろに続くデータは、要求全体を `INVALID_MSG_FORMAT` とする（aka-only-server の作成の要求と同じ）。PATCH の `null` と値の不正は `OPTIONAL_IE_INCORRECT`、空のオブジェクトは `MANDATORY_IE_MISSING`（`detail` に `at least one field is required`） |
| 応答の形式 | `application/json`。エラーは `application/problem+json`（§4.3） |
| 作成 | `201 Created` と `Location` ヘッダー（相対パス）。既に存在すれば `409` |
| 削除 | `204 No Content`。存在しなければ `404` |
| 操作者 | 任意のヘッダー `X-Operator-Id`（`^[A-Za-z0-9._@-]{1,64}$`）を監査ログに記録する |
| トレース | 任意のヘッダー `X-Trace-ID`（印字可能ASCII 1〜64文字）を受け取り、ログの `trace_id` に使う。ない場合・形式が違う場合は採番する（16バイトの乱数の16進32桁）。使った値を応答の `X-Trace-ID` ヘッダーで返す |
| ページング | 加入者・認可ポリシーの一覧は `cursor` / `limit`（1〜500、既定50）/ `prefix`（IMSI の前方一致。数字1〜15桁）。応答は `items`、`total`（`prefix` に一致する件数）、次のページがあるときだけ `nextCursor`（そのページの最後の IMSI）。IMSI の昇順。Valkey に一覧用のインデックスはないため、`SCAN` でキーを集めて並べ、そのページの分だけ読む（`pkg/masterdata` の `ListPage`。PoC の件数を前提とする） |
| 監査ログ・セッションの一覧（r6） | 監査ログは `before`（エントリID）/ `limit`（1〜500、既定100）で、新しい順。続きがあれば `nextBefore`（そのページの最後のエントリID）。セッションは `imsi` / `limit`（1〜1000、既定100）で、ページングせず `total` を返す（§3.6・§3.7）。値の不正は `INVALID_QUERY_PARAM` |
| 該当しないパス・メソッド | `404` / `405` の ProblemDetails（`cause` なし） |

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
| GET | `/clients` | RADIUSクライアントの一覧（全件。`?ip=` で絞り込み） | 200 | - |
| POST | `/clients` | RADIUSクライアントの登録 | 201 | create |
| GET | `/clients/{clientId}` | RADIUSクライアントの取得 | 200 | - |
| PATCH | `/clients/{clientId}` | RADIUSクライアントの変更 | 200 | update |
| DELETE | `/clients/{clientId}` | RADIUSクライアントの削除 | 204 | delete |
| GET | `/clients/{clientId}/secret` | 共有シークレットの取得 | 200 | read |
| GET | `/policies` | 認可ポリシーの一覧 | 200 | - |
| GET | `/policies/{imsi}` | 認可ポリシーの取得 | 200 | - |
| PUT | `/policies/{imsi}` | 認可ポリシーの作成・置き換え | 201 / 200 | create / update |
| DELETE | `/policies/{imsi}` | 認可ポリシーの削除 | 204 | delete |
| GET | `/audit-logs` | 監査ログの取得（r6） | 200 | - |
| GET | `/sessions` | アクティブセッションの取得（r6） | 200 | - |

RADIUSクライアントの一覧は件数が少ないため、ページングせず全件を IP アドレスの順（数値として比較）で返す（aka-only-server の AVクライアントの一覧と同じ）。IP アドレスとして解釈できないキーは後ろに置く。

PATCH・PUT・DELETE は、監査ログに変更前の値を残すため、書き込みの前に対象を読む（変更は「存在確認と書き込み」を1回の操作で行うため、読んだ後に削除されていれば `404` になる。§2.3）。PATCH の応答は、書き込んだ後に読み直した値を返す（加入者では、認証で進んだ最新の SQN が入る）。

### 4.3 エラー

ProblemDetails（RFC 7807）の `title` / `status` / `detail` に、aka-only-server と同じく 3GPP 風の `cause` と、項目ごとの理由 `invalidParams`（`param`, `reason`）を加える。

| HTTP | `cause` | 条件 |
|------|---------|------|
| 400 | `INVALID_MSG_FORMAT` | JSON として解釈できない、Content-Type が違う |
| 400 | `INVALID_QUERY_PARAM` | クエリパラメーターの値が不正 |
| 400 | `MANDATORY_IE_MISSING` | 必須項目がない |
| 400 | `MANDATORY_IE_INCORRECT` | 必須項目の値が不正（パスの IMSI / IP を含む） |
| 400 | `OPTIONAL_IE_INCORRECT` | 任意項目の値が不正（PATCH の項目、`X-Operator-Id` の形式を含む） |
| 404 | `USER_NOT_FOUND` | 加入者が存在しない |
| 404 | `CLIENT_NOT_FOUND` | RADIUSクライアントが存在しない |
| 404 | `POLICY_NOT_FOUND` | 認可ポリシーが存在しない |
| 409 | `SUBSCRIBER_ALREADY_EXISTS` | 同じ IMSI の加入者が既に存在する |
| 409 | `CLIENT_ALREADY_EXISTS` | 同じ IP の RADIUSクライアントが既に存在する（作成、PATCH の IP の変更） |
| 500 | `SYSTEM_FAILURE` | Valkey のエラー等、サーバー内部のエラー |

- `POLICY_NOT_FOUND`、`CLIENT_ALREADY_EXISTS` は aka-only-server にはない、本APIで追加する値である。
- 400 の `cause` は、`INVALID_QUERY_PARAM`、`MANDATORY_IE_MISSING`、`MANDATORY_IE_INCORRECT`、`OPTIONAL_IE_INCORRECT` の順に優先し、`invalidParams` には該当したすべての項目を同じ順で並べる（aka-only-server と同じ）。`reason` は `pkg/validation` の理由（例 `must be 15 digits`）、または `is required` / `must not be null` 等。
- 500 は `detail` を返さず、エラーの内容は `PROV_REQUEST_ERR` としてログに残す（§6.1）。
- 既存の `pkg/httputil.ProblemDetail`（Vector API が使用）は変えず、本API用の型を別に用意する（`type` は出力しない。aka-only-server と同じ）。

### 4.4 状態（`/status`）

| 項目 | 内容 |
|------|------|
| `version` | provisioning-api のバージョン |
| `nodeName` | ノードの識別名（環境変数 `PROVISIONING_API_NODE_NAME`。BFF / provisioner がノードを区別するため） |
| `startedAt` | 起動日時 |
| `subscriberCount` / `clientCount` / `policyCount` | 各キーの件数 |
| `sessionCount` | アクティブセッション（`sess:*`）の件数（r6。API 0.3.0） |

Valkey に接続できない場合は `500`（`SYSTEM_FAILURE`）を返す。

---

## 5. 認証・認可

| 項目 | 方式 |
|------|------|
| サーバー証明書 | ファイルで指定する（`PROVISIONING_API_TLS_CERT` / `PROVISIONING_API_TLS_KEY`）。自己署名でもよい（クライアント側で証明書を固定する） |
| クライアント認証 | mTLS 必須。クライアント証明書の SHA-256 フィンガープリント（DER のハッシュ）を、環境変数 `PROVISIONING_API_ADMIN_CLIENTS`（`name=fingerprint` のカンマ区切り）で固定する。CA による検証は行わない（aka-only-server と同じ） |
| 未登録の証明書 | 証明書なし、登録されていない証明書、有効期間外の証明書の接続は、TLS ハンドシェイクで拒否する（HTTP の応答は返さない）。拒否したことを WARN（`PROV_CLIENT_REJECTED`）で記録する（§6.1） |
| 権限 | 登録されたクライアントはすべて全権限とする。利用者・権限の管理は BFF が行う |
| 操作者 | BFF はログインした利用者の ID を `X-Operator-Id` で渡す。監査ログには、操作者 ID と、クライアント証明書に対応する識別名（`name`）の両方を記録する |

> **注記:** web-gui-for-aka-only-server の aka-only-server 管理API 用クライアント（mTLS、サーバー証明書の固定）と同じ方式で接続できる。

**実装（`internal/auth`）:**

- `PROVISIONING_API_ADMIN_CLIENTS` のフィンガープリントは、大文字・コロン区切り（`openssl x509 -noout -fingerprint -sha256` の出力形式）も受け付ける。識別名は英数字・`.`・`_`・`-` の1〜64文字。同じフィンガープリントの重複、形式の誤り、0件はいずれも起動時のエラーにする（aka-only-server の `AKA_ADMIN_CLIENTS` と同じ規則）。
- TLS 設定は TLS 1.2 以上、`ClientAuth` は `RequestClientCert` とし、証明書の有無・登録・有効期間を `VerifyConnection` で確認する（`RequireAnyClientCert` では証明書なしの接続が `VerifyConnection` の前に拒否され、ログに残らないため）。提示された証明書の秘密鍵の所持（CertificateVerify）は `ClientAuth` によらず検証される。
- 拒否のログに送信元IPを残すため、`GetConfigForClient` で接続ごとに `VerifyConnection` を差し替える。
- HTTP/2 も使える（Go の標準の動作）。

---

## 6. ログ・監査

### 6.1 アプリケーションログ

他のコンポーネントと同じく `log/slog` の JSON を標準出力に出し、fluent-bit で収集する（`app` は `provisioning-api`、出力先は `provisioning-api.log`）。

| ログ | Level | event_id | 内容 |
|------|-------|----------|------|
| `starting provisioning-api` | INFO | - | 起動時に `version`、`listen_addr`、`log_level`、`node_name`、`admin_clients`（登録クライアントの件数）、`audit_max`（Stream に保存する監査ログの件数の上限。r6）を出す（フィンガープリントは出さない）。続いて `connected to Valkey`（`addr`）、`starting server`（`addr`） |
| `assigned client ids` | INFO | - | 起動時に ID の導入前の RADIUSクライアントに ID を採番したとき（`count`。採番がなければ出さない）。採番に失敗したら `failed to assign client ids`（ERROR）を出して終了する |
| `request completed` | INFO | - | `trace_id`、`method`、`path`、`http_status`、`latency_ms`、`mgmt_client`（クライアントの識別名）、`src_ip`。パスのうち7桁を超える数字だけのセグメント（IMSI。15桁でない誤った IMSI も含む）は `LOG_MASK_IMSI` に従ってマスクする。クエリパラメーター（`prefix` 等）は出さない |
| `request failed` | ERROR | `PROV_REQUEST_ERR` | 500 を返したとき（Valkey のエラー等）。`trace_id`、`method`、`path`（マスク済み）、`error` |
| `panic recovered` | ERROR | `PROV_REQUEST_ERR` | ハンドラーのパニックから復旧して 500 を返したとき |
| `failed to store audit log` | ERROR | `PROV_AUDIT_STORE_ERR` | 監査ログを Valkey の Stream（`audit:prov`）に保存できなかったとき（r6。§3.6）。`trace_id`、`operation`、`target_type`、`error`。操作自体は成功として応答し、監査ログはログファイルには出ている |
| `admin client certificate rejected` | WARN | `PROV_CLIENT_REJECTED` | TLS ハンドシェイクでクライアント証明書を拒否したとき。`reason`（`no client certificate` / `not configured` / `outside validity period`）、`fingerprint`（証明書なしでは空文字。未登録の証明書を登録するときに確かめられる）、`src_ip` |
| `http: TLS handshake error ...` | DEBUG | - | `http.Server` の `ErrorLog`（TLS ハンドシェイクの失敗等）。拒否の記録は上の WARN で足りるため DEBUG にする |
| 設定エラー等 | ERROR | - | `failed to load config`、`failed to load server certificate`、`failed to connect to Valkey`（いずれも終了する） |

> **注記（`src_ip`）:** `src_ip` はコンテナから見た送信元である。Docker のポート公開を経由する接続では、compose ネットワークのゲートウェイ（例 `172.18.0.1`）になることがある（simwifi では、ホストからの接続でも、別ノードから Tailscale 経由で接続した場合でもゲートウェイIPになった）。共有ネットワーク（§2.2）経由の接続では、BFF のコンテナの共有ネットワーク上の IP（例 `172.19.0.3`）になる（2026-10-08 に simwifi で確認）。いずれも BFF の配置から決まる値で、`src_ip` で BFF を区別できることを前提にせず、正常な接続は `mgmt_client`、拒否した接続は `fingerprint` で見分ける（運用上の確認方法は O-05 §11.6）。

### 6.2 監査ログ

変更操作と秘密の値の読み出しを、Admin TUI の監査ログ（D-04 §3.5）と同じ形式で記録する（`event_id` は `AUDIT_LOG`、`app` は `provisioning-api`）。

| 項目 | 内容 |
|------|------|
| `operation` | `create` / `update` / `delete` / `read`（秘密の値の読み出し） |
| `target_type` | `subscriber` / `client` / `policy` |
| `target_key` | Valkey のキー（`sub:{IMSI}` 等。RADIUSクライアントの IP の変更では変更後のキー） |
| `target_id` | RADIUSクライアントの ID（RADIUSクライアントだけ。IP を変えても変わらないので、同じクライアントの記録を追える。本APIで追加する項目） |
| `target_imsi` | 加入者・認可ポリシーの IMSI（生値。Admin TUI と同じ） |
| `admin_user` | `X-Operator-Id` の値（省略時は空文字） |
| `mgmt_client` | クライアント証明書に対応する識別名（本APIで追加する項目） |
| `details` | 変更した項目（加入者の SQN / AMF、ポリシーの `default` 等は変更前後の値。Ki / OPc / 共有シークレットは「変更あり」だけで値は含めない）。読み出しでは読み出した項目名（`ki,opc` / `secret`） |
| `trace_id` | リクエストのトレースID（`request completed` と突き合わせる。本APIで追加する項目） |

`msg` は `{target_type} created` / `updated` / `deleted` / `secret read`（例 `subscriber created`、`client secret read`）。`details` の形式は次のとおり（16進は小文字、RADIUSクライアントの名前・ベンダーは引用符付き）。

| 操作 | `details` の例 |
|------|---------------|
| 加入者の作成・削除 | `amf=8000, sqn=000000000000`（削除は削除時点の値） |
| 加入者の変更 | `ki: changed, opc: unchanged, amf: 8000 -> b9b9, sqn: ff9bb4d0b627 -> 000000000020`（指定した項目だけ。Ki / OPc は値が変わったかだけ） |
| RADIUSクライアントの作成・削除 | `ip=192.168.10.1, name="AP-01", vendor="generic"` |
| RADIUSクライアントの変更 | `ip: 192.168.10.1 -> 192.168.10.9, secret: changed, name: "AP-01" -> "AP-02"`（指定した項目だけ。IP は変わったときだけ） |
| 認可ポリシーの作成・削除 | `default=deny, rules=2` |
| 認可ポリシーの置き換え | `default: allow -> deny, rules: changed (1 -> 2)`（`rules` は内容が変わったかと件数） |
| 秘密の値の読み出し | `ki,opc` / `secret` |

- 失敗した操作（400 / 404 / 409 / 500）は監査ログに記録しない（`request completed` には記録する）。
- 監査ログは `LOG_LEVEL` によらず出力する（アプリケーションログとは別のロガーで、同じ標準出力に1行ずつ書く）。`time` の形式は他のコンポーネントの `log/slog` と同じ（Admin TUI の監査ログの秒精度・UTC とは異なる）。
- 変更前の値が読めない場合（認可ポリシーの `rules` が壊れている等）でも書き込みは行い、`details` は作成と同じ形式にする。
- 監査ログは、参照用に Valkey の Stream `audit:prov` にも保存し、`GET /audit-logs` で返す（r6。§3.6）。正本はログファイルで、Stream は件数の上限（`PROVISIONING_API_AUDIT_MAX`）を超えた古いものから消える。r5 までは参照 API はなく、ホストOSのログファイルを参照していた。

### 6.3 D-04 への反映

D-04 に Provisioning API のログ（§3.6 を新設）と監査ログの項目（`mgmt_client`、`trace_id`、`operation` の `read`）を追加し、lnav フォーマット（`deployments/lnav_formats/eap_aka_log.json`）の対象ファイルに `provisioning-api.log` を、値に `mgmt_client`・`admin_user`・`operation`・`target_type` を加えた（2026-10-08）。

---

## 7. 実装構成

### 7.1 アプリケーション

```
apps/provisioning-api/
├── main.go                 # 起動、設定読み込み、TLS サーバー、Graceful Shutdown
├── Dockerfile
├── go.mod
└── internal/
    ├── config/             # 環境変数（envconfig）
    ├── auth/               # mTLS のフィンガープリント照合（Verifier / TLSConfig）、識別名の取得
    ├── server/             # ルーター（Gin）、ミドルウェア（トレース、ログ、復旧、管理クライアント、X-Operator-Id）、HTTPS サーバー
    ├── handler/            # HTTP ハンドラー（要求の読み込み、エラーの対応付け、応答）
    ├── dto/                # 要求・応答の JSON（camelCase）、ProblemDetails、JSON Merge Patch の項目（Optional）
    ├── service/            # 検証・正規化、ストア呼び出し、監査ログ
    └── audit/              # 監査ログの出力と、Stream（audit:prov）への保存・読み出し（r6）
```

- Vector API と同じく Gin と envconfig を使う（D-11）。新しい外部パッケージは追加していない。
- ハンドラーは DTO と HTTP の変換だけを行い、検証・正規化・監査ログは service 層で行う。service は検証エラーを `ValidationError`（`cause` と `invalidParams` を持つ）で、存在しない・既に存在するを `pkg/masterdata` のセンチネルエラーで返し、handler が HTTP のステータスに対応付ける。
- アプリケーションログと監査ログは、同じ標準出力に排他して書き込む（1行が混ざらないようにする）。
- `version` は `main.version`（r6 で `0.3.0`。`-ldflags "-X main.version=..."` で上書きできる）。
- Go Workspace（`go.work`）にモジュールを加えたため、他のアプリの Dockerfile も `apps/provisioning-api/go.mod` / `go.sum` をコピーする（`go.work` の `use` の解決に全モジュールの go.mod が要る）。Makefile と CI の対象にも加えた。

**共通ライブラリに追加したもの（2026-10-08）:**

| 追加 | 内容 |
|------|------|
| `SubscriberStore.ListPage` / `PolicyStore.ListPage`（`pkg/masterdata/page.go`） | IMSI の前方一致・cursor・件数で1ページを返す（§4.1 のページング）。`rules` を解釈できないポリシーは `List` と同じく結果に含めない（`total` には数える） |
| `ClientStore.Patch` | 指定した項目だけを書き換える（PATCH 用。存在確認と書き込みを1回の操作で行う）。r5 で IP の変更（キーの付け替え）に対応 |
| `SessionStore`（`pkg/masterdata/session.go`。r6） | セッションの読み出し（`Get` / `List` / `Count` / `ListByIMSI` / `IndexCount`）。Admin TUI から移した（§7.2）。`ListByIMSI` は索引に残った古い UUID を返すだけで、索引からは消さない |
| `ClientStore.GetByID` / `EnsureIDs`、`Create` の採番（r5） | ID からの取得（索引 `idx:client:{ID}` を引き、Hash の `id` と一致するときだけ返す）、ID の導入前のデータへの採番、作成時の採番（`model.RadiusClient.ID` に設定）。`BulkCreate`（CSV）は既存の ID を引き継ぐ |

### 7.2 共通ライブラリへの移動

Admin TUI と provisioning-api が同じ検証規則と同じ Valkey 操作を使うよう、Admin TUI の次の実装を `pkg/` に移した（2026-10-07 実装済み。詳細は E-03 §8、§9）（CLAUDE.md の「各 app で重複実装せず pkg を使う」方針）。

| 移動元（Admin TUI） | 移動先 | 内容 |
|-------------------|-------|------|
| `internal/validation`（加入者・クライアント・ポリシー） | `pkg/validation` | 検証規則、正規化（16進の大文字化等） |
| `internal/store` の加入者・クライアント・ポリシー（`subscriber.go`、`client.go`、`policy.go`、`keys.go` の該当部分） | `pkg/masterdata` | Valkey の読み書き（Lua スクリプトを含む） |

- 統計のストア（`statistics.go`）と CSV は Admin TUI に残す。セッションのストア（`session.go`）は、r6 で読み出しを `pkg/masterdata.SessionStore` に移し、Admin TUI には IMSI で読むときの索引の掃除（存在しないセッションの UUID の SREM）だけを残した（Admin TUI の動作は変えていない。既存のテストで確認）。
| `internal/model`（`Policy`。`pkg/model` と同じ構造） | `pkg/model` | Admin TUI 専用の `Clone` を `pkg/model.Policy` に移し、`internal/model` は廃止した |

- 移すときに、作成（加入者・クライアント・ポリシー）と変更（クライアント・ポリシー）を Lua スクリプトによる1回の操作に改めた（§2.3）。あわせて、provisioning-api 用に、加入者の指定した項目だけを書き換える操作（`SubscriberStore.Patch`。PATCH 用）、作成したかを返すポリシーの書き込み（`PolicyStore.Put`。PUT の 201 / 200 用）、ポリシーの件数（`PolicyStore.Count`。`/status` 用）を追加した。
- 「既に存在する」エラーは、判別できるセンチネルエラー（`ErrSubscriberExists` 等）にした（メッセージは従来と同じ）。
- Admin TUI の動作は変えていない（既存のテストと、simwifi での Admin TUI の操作で確認した）。
- `pkg/masterdata` / `pkg/validation` は `pkg/model` に依存する。E-03 の依存ルール（pkg 内の相互依存禁止）に、`pkg/model` への依存だけを許可する例外を加えた（E-03 §10.2）。

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
| `PROVISIONING_API_AUDIT_MAX` | `10000` | - | Valkey の Stream（`audit:prov`）に保存する監査ログの件数の上限（r6。§3.6）。1 以上。0 以下や数字でなければ起動しない |
| `GIN_MODE` | `release` | - | Gin の動作モード（Vector API と同じ） |
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
| `PROVISIONING_API_AUDIT_MAX` | `10000` | そのままコンテナの `PROVISIONING_API_AUDIT_MAX` に渡す（r6） |
| `PROVISIONING_SHARED_NETWORK` | `eapaka-prov` | 同じホストの BFF との共有ネットワークの名前（compose の `networks` の展開にだけ使う。§8.2）。通常は変えない |
| `VALKEY_PASSWORD` | - | コンテナの `REDIS_PASS` に渡す（他のコンポーネントと同じ） |
| `LOG_LEVEL` / `LOG_MASK_IMSI` | `INFO` / `true` | 他のコンポーネントと同じく、コンテナに渡す |

compose では、`PROVISIONING_API_LISTEN_ADDR` は既定値（`:9444`）のまま、`PROVISIONING_API_TLS_CERT` / `PROVISIONING_API_TLS_KEY` はマウントした証明書のパス（`/certs/server.pem` / `/certs/server.key`）に固定する。

### 8.2 Docker Compose

- `provisioning-api` サービスを追加する。証明書の準備が要るため、compose の profile（`provisioning`）に入れ、既定の `docker compose up` では起動しない。
- 公開は `${PROVISIONING_API_BIND:-127.0.0.1}:9444:9444/tcp`。別ノードの BFF から使う場合だけ `.env` の `PROVISIONING_API_BIND` を変える（§8.1.2）。
- 証明書は `deployments/certs/provisioning/` をコンテナの `/certs` に読み取り専用でマウントする。サーバー証明書の SAN には、BFF が接続に使う名前を入れる（同じホストの BFF は `DNS:provisioning-api`、ホストからの確認用に `DNS:localhost`・`IP:127.0.0.1`、別ノードの BFF は VPN のアドレス等）。SAN にない名前で接続すると、BFF（クライアント）側のホスト名の検証で失敗する（curl では `no alternative certificate subject name matches target hostname`）。
- 共有ネットワーク: トップレベルの `networks` に `provisioning-shared`（`name: ${PROVISIONING_SHARED_NETWORK:-eapaka-prov}`）を定義し、provisioning-api だけを `default` とこのネットワークの両方に参加させる。provisioning-api は profile に入っているため、ネットワークも profile `provisioning` で起動したときだけ作られる（profile なしの `docker compose up` では作られない）。BFF 側の compose は、このネットワークを `external: true` で参照する。
  - 本PoC側を先に起動する（ネットワークがない状態で BFF を起動すると、`network eapaka-prov declared as external, but could not be found` で起動できない）。
  - BFF が参加したまま本PoC側を `down` すると、ネットワークは「Resource is still in use」で残る（問題ない）。本PoC側を起動し直すと、BFF は再起動なしで接続できる（2026-10-08 に simwifi で確認）。
- ヘルスチェックはプロセスの確認（`pgrep -f /usr/local/bin/provisioning-api`）とする（`/status` は mTLS が要るため）。Linux のプロセス名（comm）は15文字までに切り詰められ、16文字の `provisioning-api` は `pgrep -x` で一致しないため、`-f` でコマンドラインと照合する。
- `depends_on` は Valkey（healthy）だけ。サーバー証明書・秘密鍵が読めない、`PROVISIONING_API_ADMIN_CLIENTS` が空・不正、Valkey に接続できない場合は、エラーを出して終了する（`restart: always` により再起動を繰り返す）。
- 秘密鍵のファイルはパーミッション 600 でよい（コンテナのプロセスは root で動く）。
- 手順は B-02 を参照。
- ログは他のサービスと同じく fluent-bit に送り、`provisioning-api.log` に出力する。

---

## 9. テスト方針

| レベル | 内容 |
|-------|------|
| 単体（`pkg/validation`、`pkg/masterdata`） | Admin TUI から移した既存のテストに加え、原子的な作成・変更（同じキーの同時作成で片方が 409 になる等）と PATCH 用の変更を miniredis で確認する |
| 単体（provisioning-api） | ハンドラー・service を httptest と miniredis で確認する（各エンドポイントの正常系・異常系、`cause`、`invalidParams`、秘密の値が応答・ログに出ないこと、監査ログの内容）。mTLS は `httptest.NewUnstartedServer` に TLS を設定して、登録済み・未登録・証明書なしの接続を確認する |
| 結合（simwifi 実機） | compose で起動し、curl とクライアント証明書で全エンドポイントを操作する。provisioning-api で登録した加入者・ポリシー・RADIUSクライアントで eapaka_test の認証が通ること、Admin TUI で同じデータが見えること、Admin TUI で登録したデータを API で読めることを確認する |

カバレッジは他のアプリと同じく 80% 以上を目標とする（T-01）。

**実施結果（2026-10-08）:**

- 単体: パッケージごとのカバレッジは audit 100%、auth 98.4%、config 100%、dto 100%、handler 98.6%、server 95.6%、service 97.6%（`main.go` は対象外。他のアプリと同じ）。ケースは T-02 を参照。
- 結合: simwifi で compose（profile `provisioning`）を起動し、curl とクライアント証明書で全エンドポイントを操作した。provisioning-api で登録した加入者・ポリシー・RADIUSクライアントで eapaka_test の EAP-AKA / AKA' が Access-Accept になること、ポリシーの置き換え・共有シークレットの変更が認証に反映されること、PATCH で SQN が巻き戻らないこと、未登録・証明書なしの接続が拒否されること、`PROVISIONING_API_BIND` を Tailscale のアドレスにして別ノード（WSL）から使えること、Admin TUI との相互参照（双方向）、監査ログの内容と、ログに秘密の値が出ないことを確認した。手順と結果は T-03 を参照。

**実施結果（2026-10-09。r6 の監査ログ・セッションの参照）:**

- 単体: パッケージごとのカバレッジは audit 98.4%、auth 98.4%、config 100%、dto 100%、handler 98.7%、server 96.3%、service 97.6%、`pkg/masterdata` 90.0%（`SessionStore` を含む）。Admin TUI の store（SessionStore を `pkg/masterdata` に委ねた後）の既存のテストもそのまま通る。ケースは T-02（r23）を参照。
- 結合: simwifi で、作業ツリーを profile `provisioning` で起動し、`GET /audit-logs`（新しい順、ページ送り、400、ログファイルの `trace_id` との対応、再起動後の保持、`PROVISIONING_API_AUDIT_MAX`=50 での件数の抑制（254件の記録で 62件））と、eapaka_test の認証で作られたセッションの `GET /sessions`（新しい順、`total`、`?imsi=`、400、`/status` の `sessionCount`）、索引を API は掃除せず Admin TUI の Session Search が掃除すること（セッションの読み出しを移した版の Admin TUI の Statistics・Session List・Session Search の表示を含む）を確認した。手順と結果は T-03（r19）の INT-PROV-037〜043 を参照。

---

## 10. 将来拡張

| 項目 | 内容 |
|------|------|
| eapaka-node-provisioner | 加入者を「IMSI＋鍵の置き場所（本PoCの Vector API / aka-only-server）＋認可ポリシー」として扱い、本APIと aka-only-server の管理API を組み合わせて操作する。複数ノードへの操作の失敗時は、補償（作成したものを消す等）で戻す。本APIは、作成の 409、ポリシーの PUT（置き換え）により、やり直しやすい形にしておく |
| 一括操作 | **本APIでは扱わない**（2026-10-09 決定）。CSV のインポート / エクスポートは Admin TUI だけで行う。エクスポートには Ki / OPc が含まれ、秘密の値を API でまとめて返すことになるため |
| 参照系 | セッション（`sess:`）・統計の参照は r6 で実装済み（§3.7、`/status` の `sessionCount`）。セッションの切断（Disconnect / CoA）は扱わない |
| 監査ログの参照 | r6 で実装済み（§3.6。Valkey の Stream `audit:prov` に保存し、`/audit-logs` で返す）。Admin TUI の操作の監査ログも参照できるようにするかは未定（現状はログファイルだけ） |
| 権限 | 管理クライアントごとの読み取り専用等 |
| IPv6 | RADIUSクライアントの IPv6 アドレス（Admin TUI・Auth Server を含めた対応が必要）。r5 で識別子を ID にしたため、API のパスへの影響はない。サブネット単位の登録も同様 |

---

## 11. 実装ステップ

1. 本書と OpenAPI 定義の作成（本書 r1）
2. 共通ライブラリへの移動（§7.2。`pkg/validation`、`pkg/masterdata`。Admin TUI の動作は変えない）… 実装済み（本書 r2）
3. provisioning-api の実装（§4〜§8）、D-01 / D-04 / D-08 / E-03 / T-02 / T-03 / B-02 等の更新、simwifi での結合確認 … 実装済み（本書 r3）
4. 監査ログ・セッションの参照（§3.6・§3.7。API 0.3.0）、セッションの読み出しの `pkg/masterdata` への移動、D-02 / D-04 / D-07 / D-08 / E-03 / B-02 / O-05 / T-02 / T-03 等の更新、simwifi での結合確認 … 実装済み（本書 r6）
5. そのほかの将来拡張（§10）は別途検討

---

## 改版履歴

| 版数 | 日付 | 内容 |
|------|------|------|
| r1 | 2026-10-07 | 初版作成。Admin TUI の加入者・RADIUSクライアント・認可ポリシーの CRUD を REST API として提供する provisioning-api の設計（位置づけ、リソースモデル、aka-only-server の管理API に揃えた作法、秘密の値の読み出しと監査、mTLS 認証、ログ・監査、共通ライブラリへの移動、設定、テスト方針、将来拡張）。拡張案 X-01 を置き換える |
| r2 | 2026-10-07 | 共通ライブラリへの移動（§7.2）の実装の反映: §7.2 を実装済みとし、`internal/model` の `pkg/model` への統合、ポリシーの変更も原子的にしたこと、`SubscriberStore.Patch` / `PolicyStore.Put` / `PolicyStore.Count` の追加、センチネルエラー、E-03 の依存ルールの例外を追記。§2.3 の注記を過去形に、§11 の手順2を実装済みに |
| r3 | 2026-10-08 | provisioning-api の実装の反映: ステータスを実装済みに。§3.2 に IP アドレスの表記の規則（先頭の 0 を不可）、§3.3 に `vlanId` は数字だけ・正規化・`rules` の検証エラーの区分、§4.1 に PATCH の `application/json`・本文の上限・要求の解釈（未知の項目等は `INVALID_MSG_FORMAT`）・トレースIDの形式・ページングの実装（SCAN）・404/405、§4.2 に書き込み前の読み出しと PATCH の応答、§4.3 に `cause` の優先順・`X-Operator-Id`・500 の扱い、§5 に有効期間外の拒否と実装（`RequestClientCert` + `VerifyConnection`、`GetConfigForClient`）、§6.1 をログの表に（`PROV_REQUEST_ERR`、`PROV_CLIENT_REJECTED`、`src_ip` がゲートウェイIPになる注記と O-05 §11.6 への参照）、§6.2 に `trace_id`・`msg`・`details` の形式・ログレベルによらない出力、§6.3 を反映済みに、§7.1 を実装の構成に（`pkg/masterdata` の `ListPage`・`ClientStore.Patch` の追加、Dockerfile・Makefile・CI）、§8.1.1 に `GIN_MODE`、§8.2 にヘルスチェック（`pgrep -f`）・終了条件・鍵のパーミッション、§9 に実施結果、§11 の手順3を実装済みに |
| r4 | 2026-10-08 | 同じホストの BFF（別の compose。web-gui-for-eapaka-radius）から接続するための共有ネットワークの追加: §2.2 の公開範囲を、同じホストの BFF は共有ネットワーク（既定名 `eapaka-prov`）経由で `https://provisioning-api:9444` に接続する形に改め（コンテナからはホストの 127.0.0.1 に届かないため）、注記を追加。§6.1 の `src_ip` の注記に共有ネットワーク経由では BFF のコンテナの IP になることを追記。§8.1.2 に `PROVISIONING_SHARED_NETWORK`、§8.2 に共有ネットワークの定義・起動と停止の順序、サーバー証明書の SAN（`DNS:provisioning-api`）を追加。いずれも simwifi で確認 |
| r5 | 2026-10-08 | RADIUSクライアントにサーバー採番の ID を導入（API 0.2.0。provisioning-api のバージョン 0.2.0）: §3 の識別子を ID に、§3.2 に `id` と、識別子の設計（`client:{IP}` の Hash の `id`、索引 `idx:client:{ID}`、カウンター `seq:client`、Lua による作成・IP の変更・削除、起動時の採番 `EnsureIDs`、`?ip=` による検索、ID に改めた理由）、PATCH での IP の変更（キーの付け替え、409）を追記。§3.4・§4.2 のパスを `/clients/{clientId}` に、§4.3 の `CLIENT_ALREADY_EXISTS` の条件に PATCH を追加。§6.1 に `assigned client ids`、§6.2 に `target_id` と details の `ip`。§10 の IPv6 に注記。simwifi で、変更前の版で登録したクライアントへの採番、IP の変更の認証への反映、Admin TUI の ID 表示を確認 |
| r6 | 2026-10-09 | 監査ログとセッションの参照を追加（API 0.3.0。provisioning-api のバージョン 0.3.0）: §1.1・§1.4 の対象に監査ログの参照とセッションの参照を加え、一括操作（CSV）は Admin TUI だけで行い本APIでは扱わないことにした。§3.6（監査ログ。Valkey の Stream `audit:prov` への保存、上限 `PROVISIONING_API_AUDIT_MAX`、保存の失敗の扱い、aka-only-server に揃えた項目と `action` の命名）と §3.7（セッション。読み取りだけ、新しい順・`limit`・`total`、`?imsi=` と索引を掃除しないこと）を新設。§4.1 に一覧の作法、§4.2 に `GET /audit-logs`・`GET /sessions`、§4.4 に `sessionCount`、§6.1 に `audit_max` と `PROV_AUDIT_STORE_ERR`、§6.2 に Stream への保存、§7.1 に `SessionStore` と `audit` の Stream、§7.2 にセッションの読み出しの `pkg/masterdata` への移動、§8.1 に `PROVISIONING_API_AUDIT_MAX`、§9 に実施結果、§10・§11 を更新 |
