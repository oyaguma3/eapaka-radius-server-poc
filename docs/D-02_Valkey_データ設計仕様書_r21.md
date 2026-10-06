# D-02 Valkey データ設計仕様書 (r21)

## 1. 全体方針

- **接続方式:** Valkey コンテナ（`valkey/valkey:9.0`）。コンテナ間は `valkey:6379`（各アプリの環境変数 `REDIS_HOST` / `REDIS_PORT`）、ホスト（Admin TUI）からは `127.0.0.1:6379`（ループバックのみ公開）で接続する
- **認証:** パスワード認証必須（`--requirepass ${VALKEY_PASSWORD}`。各アプリへは `REDIS_PASS` として渡す）
- **データ永続化:** AOF (Append Only File) 有効化（`--appendonly yes`、`--appendfsync everysec`）。メモリ上限 512MB、`--maxmemory-policy noeviction`
- **ネーミング規則:** キーの役割を明確にするため、以下のプレフィックスを厳守する。

| **プレフィックス** | **カテゴリ** | **用途**                          | **Type** | **永続性 / TTL** |
| ------------------ | ------------ | --------------------------------- | -------- | ---------------- |
| **`sub:`**         | Master       | **加入者情報 (Subscriber)**       | Hash     | 永続 |
| **`client:`**      | Config       | **RADIUSクライアント設定**        | Hash     | 永続 |
| **`policy:`**      | Config       | **認可ポリシー (Authorization)**  | Hash     | 永続 |
| **`eap:`**         | State        | **EAP認証コンテキスト** (認証中)  | Hash     | 一時 (60s、更新ごとにリセット) |
| **`sess:`**        | State        | **アクティブセッション** (認証後) | Hash     | 長期 (24h、Acct Start/Interimでリセット) |
| **`acct:seen:`**   | State        | **Accounting重複検出キャッシュ**  | String   | 一時 (24h、書き込みごとにリセット) |
| **`idx:user:`**    | Index        | **ユーザー検索用インデックス**    | Set      | TTLなし（Acct-Stop時にSREM、Admin TUI読み取り時に掃除） |

> **補足:** Admin TUI の `internal/store/keys.go` に `stats:global` 定数が定義されているが、現行実装では使用していない（統計情報は Admin TUI のメモリ上で1分間キャッシュするのみで、Valkey には保存しない）。

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
| `sqn`        | Yes      | シーケンス番号 (SQN)   | Hex 12桁。**Vector APIが認証ごとに更新する**（Lua スクリプトによる比較・置き換え（CAS）。下記）。Admin TUI の加入者編集は SQN を変更したときだけ書き換える（編集開始時の値との比較・置き換え。下記） |
| `created_at` | -        | 作成日時               | RFC3339（UTC）。Admin TUIが作成時に未指定なら現在時刻を設定。更新時は変更しない。Vector APIは参照しない |

> **Hex表記:** Admin TUI は大文字・小文字どちらの16進数も受け付け、**大文字に正規化して**保存する（加入者の作成・編集、CSVインポートとも。`internal/validation` の `NormalizeSubscriberInput`）。Vector API が書き戻す `sqn` は常に12桁の**小文字** hex（`%012x`）となる。このため、Admin TUI の加入者編集では SQN の変更有無を大文字小文字を区別せずに判定する（下記）。

> **SQN更新方式（現行実装）:**
> - Vector API は `HGETALL sub:{IMSI}` で Ki/OPc/AMF/SQN を取得し、新SQNを算出した後、Lua スクリプトで `sqn` を**比較・置き換え（CAS: Compare-And-Swap）**する。ベクターはこの書き換えに成功した後に計算する（書き換えに失敗した場合はベクターを計算しない）
> - 新SQNの算出:
>   - 通常: 現SQN + 32（SQN = SEQ(43bit) || IND(5bit) のうち SEQ を +1、IND は不変）
>   - 再同期: AUTS から取り出した SQN_MS を検証（SQN_MS > SQN_HE かつ差が 2^28 以下）し、SQN_MS + 32
>   - 48bit 上限を超える場合は `SQN_OVERFLOW_ERR`（HTTP 500）
> - `sqn` は12桁でなければ解析エラーとなる
> - 比較・置き換え（`apps/vector-api/internal/store/subscriber.go` の `CompareAndSetSQN`。`EVALSHA` で実行し、スクリプトが未ロードの場合は go-redis が `EVAL` にフォールバックする）:
>
>   ```lua
>   local cur = redis.call('HGET', KEYS[1], 'sqn')
>   if cur == false or cur ~= ARGV[1] then
>     return 0
>   end
>   redis.call('HSET', KEYS[1], 'sqn', ARGV[2])
>   return 1
>   ```
>
>   - `KEYS[1]` は `sub:{IMSI}`、`ARGV[1]` は `HGETALL` で読んだ `sqn` の値（読んだ文字列をそのまま渡す。Admin TUI が大文字で保存した値も正規化せずに比較する）、`ARGV[2]` は新SQN（12桁小文字hex）
>   - `sqn` が読んだ値と一致するときだけ書き換えて 1 を返す。一致しない（他のリクエストが先に書き換えた）ときは書き換えずに 0 を返す
>   - キーまたは `sqn` フィールドが無い（読み出し後に加入者が削除された）ときも 0 を返し、キーを新たに作成しない（やり直しの `HGETALL` で未登録となり HTTP 404）
>   - 比較するのは `sqn` フィールドだけで、Admin TUI が `ki` / `opc` / `amf` だけを変更しても競合とはしない
> - 競合（0）した場合は、1〜10ms のランダムな時間待ってから `HGETALL` からやり直す（最大3回試行）。やり直すたびに `SQN_CONFLICT_RETRY`（WARN）を出力する。3回とも競合した場合、または待っている間にリクエストの期限切れ・キャンセルとなった場合は HTTP 409 Conflict（`SQN_CONFLICT_ERR`）を返し、ベクターは返さない（D-04 §3.4.7）
> - 再同期のやり直しで、読み直した SQN_HE が SQN_MS 以上の場合は、別のリクエスト（同じ再同期の再送など）が先に同期済みとみなし、デルタ検証をせずに通常どおり SQN_HE + 32 とする（端末の SQN_MS より大きいため受け入れられる）
> - 書き換え時の Valkey エラーは HTTP 500（`VALKEY_CONN_ERR`）とし、ベクターは返さない（テストベクターモードも同じ）。アプリ独自のリトライは行わない（go-redis 既定の自動リトライのみ）
> - 書き換え後のベクター計算に失敗した場合は HTTP 500（`CALC_ERR` "Milenage calculation error"）となり、`sqn` は進んだままとなる（SQN が飛ぶだけで、端末は SEQ が増えていれば受け入れるため問題ない）
> - `sub:{IMSI}` が存在しない場合は HTTP 404（`CALC_ERR` "subscriber not found"）を返し、キーは作成しない（テストベクターモードも同じ）
> - これにより、同一IMSIへの並行リクエストでも、発行するベクターの SQN は IMSI ごとに一意で単調に増加する（同値・巻き戻りを起こさない）。SQN が飛ぶ（使われない値が生じる）ことは許容する
> - 経緯: D-11 r9 までは WATCH/MULTI による CAS で設計していたが、r10 で Lua による `sqn` フィールドの比較・置き換えに変更して実装した（本書 r17 までは、単純な HSET による後勝ちの書き戻しと記載していた）。詳細は D-11「Vector API詳細設計書」セクション7.5・13.6 を参照
>
> **Admin TUI からの書き込み（加入者編集）:**
> - Admin TUI の加入者編集（`pkg/masterdata/subscriber.go`。Provisioning API と共通。E-03 §9）は、1つの Lua スクリプト（`updateSubscriberScript`。`EVALSHA` で実行）で、`sub:{IMSI}` の存在チェックと更新をまとめて行う
>
>   ```lua
>   if redis.call('EXISTS', KEYS[1]) == 0 then
>     return 0
>   end
>   if ARGV[4] == '1' then
>     if redis.call('HGET', KEYS[1], 'sqn') ~= ARGV[5] then
>       return -1
>     end
>     redis.call('HSET', KEYS[1], 'ki', ARGV[1], 'opc', ARGV[2], 'amf', ARGV[3], 'sqn', ARGV[6])
>   else
>     redis.call('HSET', KEYS[1], 'ki', ARGV[1], 'opc', ARGV[2], 'amf', ARGV[3])
>   end
>   return 1
>   ```
>
>   - `KEYS[1]` は `sub:{IMSI}`、`ARGV[1]`〜`ARGV[3]` は Ki / OPc / AMF、`ARGV[4]` は `sqn` も更新するか（`1` / `0`）、`ARGV[5]` は編集開始時（編集画面を開いたとき）に読んだ `sqn`、`ARGV[6]` は新しい SQN（大文字に正規化した入力値）
>   - 戻り値: 1 = 更新した、0 = 加入者が存在しない（`ErrSubscriberNotFound`。キーは作成しない）、-1 = `sqn` が編集開始時の値と一致しない（何も更新しない。`ErrSQNChanged`）
>   - `created_at` は変更しない
> - SQN を変更していない場合（大文字小文字の違いだけの場合を含む）は `Update` で **`ki` / `opc` / `amf` だけ**を更新し、`sqn` は書き換えない。編集画面を開いている間に認証で Vector API が進めた `sqn` はそのまま残る
> - SQN を変更した場合は `UpdateWithSQN` で、現在の `sqn` が編集開始時に読んだ値と**一致するときだけ** `ki` / `opc` / `amf` と `sqn` を更新する。一致しない（編集中に認証で `sqn` が進んだ）ときは何も更新せず、Admin TUI はエラーを表示する（D-05 セクション4.2.2）
> - これにより、Admin TUI の保存で `sqn` が巻き戻ることはない。Vector API 側から見ると、Admin TUI が `ki` / `opc` / `amf` だけを更新した場合は `sqn` が変わらないので競合にならず、Admin TUI が `sqn` を変更した場合は競合として検出してやり直す
> - 例外として、Admin TUI の新規作成と CSV インポート（既存キーを上書きする）は、比較せずに `sqn` を入力値・CSV の値で書き込む
> - 経緯: 本書 r18 までの Admin TUI は、SQN を変更していなくても `sqn` をフォームの値（編集開始時の値）で上書きしていた（`EXISTS` の後に `HSET`）ため、編集中に認証が進むと保存時に `sqn` が巻き戻る可能性があった。また `EXISTS` と `HSET` の間に加入者が削除されると、一部のフィールドだけの Hash が作られる可能性があった。r19 でいずれも解消した
>
> **テストベクターモード（`TEST_VECTOR_ENABLED=true`）時:**
> - IMSI が `TEST_VECTOR_IMSI_PREFIX`（既定 `00101`）で始まる場合、Ki/OPc/AMF は Vector API 内蔵の固定値（3GPP TS 35.208 Test Set 1、AMF `b9b9`）を使い、`sub:{IMSI}` の `ki` / `opc` / `amf` は参照しない
> - それ以外（`sub:{IMSI}` の取得、`sqn` の解析・+32・書き戻し、再同期、エラー処理）は通常モードと同じ。**テストベクター対象の IMSI でも `sub:{IMSI}` の登録が必要**で、未登録なら HTTP 404、Valkey エラー・書き戻し失敗は HTTP 500、`sqn` の解析エラーも HTTP 500 になる（既定 SQN へのフォールバックはしない）
> - `ki` / `opc` / `amf` は参照しないが、Admin TUI・投入スクリプトで登録する場合は通常どおり全フィールドを設定する（テスト用加入者は Test Set 1 の値で登録する）
> - r13 以前の実装では、キーが無い場合に既定 SQN `ff9bb4d0b607` から計算して **`sqn` フィールドだけを持つ Hash を作成していた**が、現行実装では作成しない。旧実装で作られた `sqn` だけの `sub:{IMSI}` が残っている場合、テストベクターモードでは登録済みとして扱われる（不要なら DEL で削除する）

### B. RADIUSクライアント設定 (Client Config)

パケット受信時にIPアドレスで照合し、共有秘密鍵を取得するためのデータ。

- **Key:** `client:{IP_ADDRESS}`
- **Type:** `Hash`

| **Field** | **必須** | **説明**       | **備考**                     |
| --------- | -------- | -------------- | ---------------------------- |
| `secret`  | Yes      | **共有秘密鍵** | Auth/Acct Serverが `HGET client:{IP} secret` で取得 |
| `name`    | -        | クライアント名 | Admin TUIの表示・管理用（Auth/Acct Serverは参照しない） |
| `vendor`  | -        | ベンダー名     | Admin TUIの表示・管理用（Auth/Acct Serverは参照しない） |

> ※利用時の優先順位に関する注意:
>
> パケット受信時は、まず `client:{IP}` の `secret` を検索する。
>
> レコードが存在しない場合、または Valkey エラーの場合は、フォールバックとして環境変数 `RADIUS_SECRET` の値を使用する。
>
> どちらも得られない場合はパケットを破棄する（`RADIUS_NO_SECRET` WARN）。

### C. 認可ポリシー (Authorization Policy)

認証成功後(Post-Auth)に参照される、接続許可ルールおよびパラメータ設定。

- **Key:** `policy:{IMSI}`
- **Type:** `Hash`

| **Field** | **必須** | **説明**                  | **備考**              |
| --------- | -------- | ------------------------- | --------------------- |
| `rules`   | Yes      | **認可ルール (JSON配列の文字列)** | 詳細は後述。Admin TUIはルール0件でも `[]` を書き込む |
| `default` | Yes      | デフォルト動作            | `allow` または `deny`（小文字） |

> **キー・フィールドの欠落と不正値の扱い（Auth Server）:**
> - `policy:{IMSI}` が存在しない → **Access-Reject**（`AUTH_POLICY_NOT_FOUND` WARN）。認証成功にはポリシー登録が必須
> - `rules` が欠落または空文字 → ルール0件として扱う（`null` も0件）
> - `rules` の JSON パースに失敗（型不一致を含む） → **Access-Reject**（ログの event_id は `AUTH_POLICY_NOT_FOUND`、error に `policy invalid` を含む）
> - `default` が欠落、または `allow` / `deny` 以外（`ALLOW` 等の大文字や空文字を含む） → `deny` として扱う

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
    "nas_id": "AP-OFFICE-02",
    "allowed_ssids": ["*"]
  },
  {
    "nas_id": "*",
    "allowed_ssids": ["GUEST-WIFI"],
    "vlan_id": "300"
  }
]
```

上記の例は「`AP-OFFICE-01` では `CORP-WIFI` / `GUEST-WIFI` を許可して VLAN 100・Session-Timeout 3600秒を付与」「`AP-OFFICE-02` では全SSIDを許可（VLAN・Session-Timeoutなし）」「それ以外のNASでは `GUEST-WIFI` だけを許可して VLAN 300 を付与」を表す。ルールは配列順に評価されるため、`AP-OFFICE-01` の `GUEST-WIFI` は先に一致する1番目のルール（VLAN 100）が採用される。`nas_id` の `"*"` は任意のNASに一致するので、個別のNASのルールより後ろに置く。SSIDを問わずどのNASからでも許可したい場合は `nas_id` `"*"`・`allowed_ssids` `["*"]` のルールを置くか、`default` を `allow` にする（default による許可では VLAN・Session-Timeout は付与されない）。

| フィールド | 型 | 説明 | 備考 |
|-----------|-----|------|------|
| `nas_id` | string | NAS識別子 | `"*"` 単独は**任意の NAS-Identifier に一致**するワイルドカード（NAS-Identifier が無いリクエストを含む）。部分一致は行わない（`"AP-*"` などは文字どおりの値としか一致しない）。それ以外は RADIUS `NAS-Identifier` 属性と**完全一致**（大文字小文字を区別）。NAS-Identifier が無いリクエストは空文字として比較される。Admin TUIでは必須（1〜253文字の印字可能ASCII） |
| `allowed_ssids` | []string | 許可SSIDリスト | 要素に `"*"` が含まれていれば全SSID（Called-Station-Id が無い場合を含む）に一致。それ以外は大文字小文字を区別しない完全一致。空配列・省略はどのSSIDにも一致しない。Admin TUIでは1件以上必須（各1〜32文字） |
| `vlan_id` | string | VLAN ID | **JSON文字列**で指定（数値で書くとJSONパースエラーとなり Reject）。空文字・省略は未設定。Auth Serverは値を検証しない（Admin TUIは 0〜4094 の数字のみ受け付ける） |
| `session_timeout` | int | セッションタイムアウト秒 | JSON数値。0以下・省略は未設定。Admin TUIは 0〜86400 を受け付ける |

#### 評価ロジック（Auth Server `internal/policy/evaluator.go`）

1. SSIDは `Called-Station-Id` から抽出する。最初の `:` より後ろをSSIDとし（例: `AA-BB-CC-DD-EE-FF:CORP-WIFI` → `CORP-WIFI`）、`:` が無い場合は値全体をSSIDとする
2. `rules` を**配列順に**評価し、`nas_id` と `allowed_ssids` の**両方に一致した最初のルール**を採用する → 許可（`nas_id` は `"*"` なら任意のNASに一致、それ以外は完全一致。`allowed_ssids` は `"*"` なら任意のSSIDに一致、それ以外は大文字小文字を区別しない完全一致）
3. 一致するルールが無い場合は `default` で判定する
   - `allow` → 許可（採用ルールなし）
   - `deny` → **Access-Reject**（`AUTH_POLICY_DENIED` WARN、reason: `no matching rule and default is deny`）
4. 許可時の応答属性（採用ルールがある場合のみ付与）
   - `vlan_id` が空でない → `Tunnel-Type`=13 (VLAN)、`Tunnel-Medium-Type`=6 (IEEE-802)、`Tunnel-Private-Group-Id`=`vlan_id`（いずれもTag 0）
   - `session_timeout` > 0 → `Session-Timeout`
   - default `allow` による許可では、VLAN・Session-Timeout は付与しない

------

## 3. ステートデータ (一時・動的)

認証・課金プロセスの中で自動的に生成・削除されるデータ群です。

### D. EAP認証コンテキスト (EAP Context)

IdentityフェーズからChallenge応答フェーズへ情報を持ち回るための一時データ。

**重要:** ここで使用するUUIDは、**ログのTrace ID**、**RADIUS State属性**、**Vector API連携用ヘッダ(X-Trace-ID)** として統一して利用する。

- **Key:** `eap:{UUID}`
- **UUIDフォーマット:** RFC 4122準拠、ハイフン含む36文字（例: `550e8400-e29b-41d4-a716-446655440000`）
- **生成:** Auth Server が State属性の無い初回の Access-Request（Identity）を受信したときに `github.com/google/uuid` の `uuid.New().String()` で生成し、コンテキストのキー・State値とする。以降の Access-Request（Access-Challengeへの応答）では、UUID形式の State属性の値をそのまま Trace ID として使う（ハンドラー層のログを含め、1回の認証を通して同じ Trace ID になる。D-04 §4.1）。State属性がUUID形式でない場合は新しいUUIDを生成する（対応するコンテキストはないため Reject）
- **Type:** `Hash`
- **TTL:** **60秒**（作成時に設定し、更新（HSET）のたびに EXPIRE で60秒にリセット）
- **削除:** 認証成功時、および Challenge検証失敗・ポリシー拒否・Authentication-Reject/Client-Error受信・再同期上限超過などの失敗時に DEL。それ以外（初回の Vector 取得失敗等）は TTL で消滅

| **Field** | **格納形式** | **説明** |
| --------- | ------ | -------- |
| `imsi`    | String | 認証中のIMSI（フル認証誘導時は永続ID受領まで空文字） |
| `eap_type` | 10進数文字列 | EAP方式 (`23`=EAP-AKA, `50`=EAP-AKA') |
| `stage`   | String | 認証フェーズ（下記の大文字の状態名） |
| `rand`    | Hex（小文字） | Vector Gatewayから取得したRAND (16 bytes) |
| `autn`    | Hex（小文字） | Vector Gatewayから取得したAUTN (16 bytes) |
| `xres`    | Hex（小文字） | Vector Gatewayから取得した期待値レスポンス (4-16 bytes) |
| `k_aut`   | Hex（小文字） | MAC計算・検証用鍵 (EAP-AKA: 16 bytes, EAP-AKA': 32 bytes) |
| `msk`     | Hex（小文字） | Master Session Key (64 bytes) |
| `resync_count` | 10進数文字列 | 再同期試行回数（上限32回） |
| `permanent_id_requested` | `1` / `0` | フル認証誘導済みフラグ |

> 作成時（Create）は構造体の全フィールドを書き込むため、未確定のフィールドも空文字・`0` として存在する。

> **セキュリティ方針（CK/IKの取り扱い）:**
> - Vector Gatewayから受信したCK/IKは、鍵導出処理の一時変数としてのみ使用する
> - 導出後の鍵（K_aut, MSK）のみをEAPコンテキストに保存する
> - **CK/IKはValkeyに永続化しない**（セキュリティ上の理由）

> **eap_typeについて:**
> - RFC 3748で定義されるEAP Type番号
> - EAP-AKA: 23 (RFC 4187)
> - EAP-AKA': 50 (RFC 9048)
> - Identity解析時に先頭文字から決定し、以降の処理で参照

> **stageフィールドの値:**
> Auth Server の `internal/eap/statemachine.go` の `EAPState` 定数（**大文字**）をそのまま保存する。
>
> | 状態名 | 定数 | Valkeyへの書き込み | 説明 |
> |--------|------|------------------|------|
> | `NEW` | `StateNew` | あり（フル認証誘導の作成時） | 初期状態 |
> | `WAITING_IDENTITY` | `StateWaitingIdentity` | あり | AT_PERMANENT_ID_REQ送信済み、永続ID応答待ち |
> | `IDENTITY_RECEIVED` | `StateIdentityReceived` | あり | 永続ID受領済み |
> | `WAITING_VECTOR` | `StateWaitingVector` | なし（状態遷移の検証のみ） | Vector Gateway応答待ち |
> | `CHALLENGE_SENT` | `StateChallengeSent` | あり（再同期後も再びこの値） | Challenge送信済み |
> | `RESYNC_SENT` | `StateResyncSent` | なし | 再同期処理中 |
> | `SUCCESS` | `StateSuccess` | なし（成功時はコンテキストを削除） | 認証成功（終了状態） |
> | `FAILURE` | `StateFailure` | なし（失敗時は削除またはTTL消滅） | 認証失敗（終了状態） |
>
> `pkg/model` にも小文字の `Stage` 定数（`new`, `challenge_sent` 等）が定義されているが、現行のアプリケーションは使用しておらず、Valkey に保存される値ではない。

> **書き込みの流れ:**
> - 永続ID: 作成 {imsi, stage=`IDENTITY_RECEIVED`, eap_type} → ベクター取得・鍵導出後に更新 {stage=`CHALLENGE_SENT`, rand, autn, xres, k_aut, msk, imsi, eap_type}
> - 仮名/再認証ID: 作成 {stage=`NEW`, permanent_id_requested=`1`, eap_type} → 更新 {stage=`WAITING_IDENTITY`} → 永続ID受信で更新 {imsi, stage=`IDENTITY_RECEIVED`, eap_type} → 以降は永続IDと同じ
> - 再同期: stage が `CHALLENGE_SENT` の場合のみ受け付け、更新 {stage=`CHALLENGE_SENT`, rand, autn, xres, k_aut, msk, resync_count+1}

> **autnの保存理由:**
> - EAP-AKA'では、CK'/IK'導出にAUTNが必要（RFC 9048 Section 3.3）
> - Challenge応答検証時にAT_MAC計算で使用する場合がある

> **k_autについて:**
> - EAP-AKA: K_aut (16 bytes) - MKからPRFで導出
> - EAP-AKA': K_aut (32 bytes) - PRF'で導出、HMAC-SHA-256-128に使用

> **resync_countについて:**
> - SQN再同期（AKA-Synchronization-Failure）の試行回数をカウント
> - 受信時に `resync_count` が32以上なら認証失敗（`AUTH_RESYNC_LIMIT`、Access-Reject）。すなわち再同期は32回まで
> - 上限32回はSQN INDフィールド1サイクル分

> **permanent_id_requestedについて:**
> - 仮名ID(2,7)または高速再認証ID(4,8)を受信した場合に `1` をセット
> - AT_PERMANENT_ID_REQを送信してフル認証に誘導したことを示す

### E. アクティブセッション (Active Session)

認証完了から切断まで維持されるセッション情報。

RADIUS属性 Class にこのUUIDが格納される。

- **Key:** `sess:{UUID}`
- **Type:** `Hash`
- **TTL:** **24時間**（Auth Accept時に設定し、Acct Start/Interim受信時に EXPIRE で24時間にリセット。Start/Interimとも `sess:{UUID}` が存在する場合のみ）
- **UUIDフォーマット:** RFC 4122準拠、ハイフン含む36文字（例: `550e8400-e29b-41d4-a716-446655440000`）
- **生成:** Auth Serverが `github.com/google/uuid` の `uuid.New().String()` で生成し、Class属性に格納
- **検証:** Acct Serverが `uuid.Parse()` でClass属性を検証。UUIDとして不正な場合はClass属性なしとして扱う

> **Session UUIDとTrace IDの関係:**
> - Trace ID: EAP認証プロセス中の追跡用（`eap:{UUID}`のキー、数秒〜数十秒の寿命）
> - Session UUID: セッション管理用（`sess:{UUID}`のキー、Class属性、最大24時間の寿命）
> - 両者は独立して生成される別のUUID

| **Field**      | **格納形式** | **説明**           | **書き込みタイミング**   |
| -------------- | ------------ | ------------------ | -------------------- |
| `imsi`         | String       | IMSI               | Auth Accept時 |
| `nas_ip`       | String       | NAS IPアドレス     | Auth Accept時（Access-Requestの送信元IP）。Acct Start/Interim時に Accounting-Request の送信元IPで上書き。radsecproxy 等のプロキシ経由ではプロキシのIPになる |
| `nas_identifier` | String     | NAS-Identifier     | Auth Accept時（ポリシー評価に使った Access-Request の NAS-Identifier。属性がなければ空文字）。Acct Start/Interim時に Accounting-Request の NAS-Identifier があれば上書き（なければ残す） |
| `start_time`   | 10進数（Unix秒） | 接続開始時刻   | Auth Accept時。Acct Start時に上書き |
| `client_ip`    | String       | 端末IP (Framed-IP-Address) | Auth Accept時は空文字。Acct Start/Interim時に Framed-IP-Address があれば上書き |
| `acct_id`      | String       | Acct-Session-Id    | Auth Accept時は空文字。Acct Start時に設定 |
| `input_octets` | 10進数       | 受信通信量（Acct-Input-Octets） | Auth Accept時は `0`。Acct Interim時に上書き |
| `output_octets`| 10進数       | 送信通信量（Acct-Output-Octets） | Auth Accept時は `0`。Acct Interim時に上書き |

> Acct Stop時はセッションを削除するため、Stop の通信量は `sess:{UUID}` には保存しない（`ACCT_STOP` ログに出力する）。

> **`nas_identifier` を持つ理由:** Auth / Acct Server の前段に radsecproxy 等のプロキシを置くと、`nas_ip`（送信元IP）はすべてプロキシのIPになり、NAS を区別できない。NAS-Identifier はプロキシで NAS ごとの値に置き換えられる（D-08 §5.8）ため、NAS の識別に使う。`nas_identifier` は 2026-10-06 の実装修正で追加したフィールドで、それより前に作られたセッションにはない（Admin TUI は `-` と表示する）。

> **Acct Server のセッション処理と不在時の動作:**
> - **Start:** Class属性が無い・不正、または `sess:{UUID}` が存在しない場合は `ACCT_SESSION_NOT_FOUND`（WARN）を出力し、セッションは更新しない（新規作成もしない）
> - **Interim:** Class属性がある場合、`sess:{UUID}` の存在を確認（EXISTS）し、存在すれば HSET + EXPIRE する。存在しない場合（TTL切れ等）は `ACCT_SESSION_NOT_FOUND`（WARN、`class_uuid` 付き）を出力し、セッションは更新しない（新規作成もしない。`imsi` を持たない Hash は作られない）。存在確認でValkeyエラーが発生した場合は `VALKEY_CONN_ERR` を出力し、セッションは更新しない。Class属性が無い場合はセッションを更新せず、ログも出力しない
> - **Stop:** `sess:{UUID}` を取得して `imsi` を得たうえで DEL し、`imsi` が得られた場合のみ `idx:user:{IMSI}` から SREM する。不在のログは出力しない
> - いずれの場合も Accounting-Response は返却する（クライアント再送防止）
> - TTL超過による削除と未作成は区別しない（Start / Interim 時はいずれも `ACCT_SESSION_NOT_FOUND` を出力する。TTL超過専用の event_id はない）

### F. ユーザー検索インデックス (Index)

IMSIから現在のセッションIDを逆引きするためのセット。

- **Key:** `idx:user:{IMSI}`
- **Type:** `Set` (Members: Session UUID)
- **追加:** Auth Server が Access-Accept 時に SADD（`sess:{UUID}` 作成直後。失敗は `SESSION_INDEX_ERR` WARN のみで認証は継続）
- **削除:** Acct Server が Acct-Stop 時に SREM

> **クリーンアップ方針:**
> - `idx:user:{IMSI}` はTTLなしのSetであり、Acct-Stop未達やセッションTTL切れでゴミが残る可能性がある
> - **クリーンアップは読み取り時（Admin TUI）に実施する**
>   - IMSI指定でセッションを取得する際（`SessionStore.GetByIMSI`）、SMEMBERS で得たUUIDごとに `sess:{UUID}` を取得し、存在しないUUIDは `SREM idx:user:{IMSI}` で自動削除
>   - インデックスが空の場合は `sess:*` を SCAN して `imsi` で絞り込むフォールバックを行う
>   - IMSIごとのセッション数（`GetSessionCount`）は SCARD のため、掃除前のゴミを含む場合がある
> - これにより、データ構造を変更せずにゴミを解消できる（PoCスコープの最小変更方針）
> - 詳細はD-07「Admin TUI詳細設計書【後半】」セクション6.10を参照

### G. Accounting重複検出キャッシュ (Duplicate Detection)

Acct Serverが重複パケットおよび順序異常を検出するためのキャッシュ。

- **Key:** `acct:seen:{Acct-Session-Id}`
- **Type:** `String`
- **TTL:** **24時間**（SETのたびに24時間を設定）

| **値** | **意味** |
| ------ | -------- |
| `start` | Acct-Start受信済み |
| `interim:{input}:{output}` | 最新Interim受信済み（Acct-Input-Octets / Acct-Output-Octets の10進値。通信量で重複判定） |
| `stop` | Acct-Stop受信済み |

> **重複・順序異常の検出ロジック（`internal/acct/duplicate.go`）:**
> - **Start:**
>   - 値なし → `start` をセットして処理継続
>   - 値が `stop` → `start` に上書きし `ACCT_SEQUENCE_ERR`（reason: `start_after_stop`）を出力して処理継続（セッションは存在すれば更新。新規作成はしない）
>   - 値が `start` または `interim:*` → `ACCT_DUPLICATE_START` を出力し、以降の処理（セッション更新・`ACCT_START` ログ）をスキップ
> - **Interim:**（1回の GET で直前の値を取得して判定してから SET する。判定前に自身の書き込みで値が変わることはない）
>   - 値が今回と同一の `interim:{input}:{output}` → `ACCT_DUPLICATE_INTERIM`（msg: `duplicate accounting interim`）を出力し、値は変更せず、以降の処理をスキップ
>   - 値なし → `ACCT_SEQUENCE_ERR`（reason: `no_start_received`）を出力し、`interim:{input}:{output}` をセットして処理継続（課金データの欠損を避けるため）
>   - 値が `stop` → `ACCT_SEQUENCE_ERR`（reason: `interim_after_stop`）を出力し、`interim:{input}:{output}` をセットして処理継続
>   - 値が `start` または別値の `interim:*` → 正常。`interim:{input}:{output}` をセットして処理継続
> - **Stop:**
>   - 値が `stop` → ログを出力せず、以降の処理（セッション削除・`ACCT_STOP` ログ）をスキップ
>   - それ以外（値なしを含む） → `stop` をセットして処理継続
> - 重複判定でValkeyエラー（GET / SET の失敗）が発生した場合は `VALKEY_CONN_ERR` を出力して処理を継続する（GET 失敗時は重複・順序異常の判定を行わない）
> - いずれの場合も Accounting-Response は返却する
>
> **設計意図:**
> - Acct-Session-IdはNASが生成する識別子であり、セッションUUID（Auth Server生成）とは独立
> - NASの再起動やネットワーク障害による再送パケットを適切に処理するため、24時間キャッシュを保持
> - 詳細は D-10「Acct Server詳細設計書」セクション5.8を参照

------

## 4. データアクセスフロー (処理ロジック)

方針変更（Shared Secret優先順位、Trace ID統合）を反映した、各サーバーの処理手順です。

### Auth Server (UDP 1812)

1. **受信時 (共通):**
   - 送信元IPで `HGET client:{IP} secret` を実行。
   - **ヒット時:** その `secret` を使用。
   - **未登録またはValkeyエラー:** 環境変数 `RADIUS_SECRET` を使用（未設定なら破棄）。
   - Trace ID（UUID）を決定し、ログの `trace_id` に設定（State属性なしなら生成、UUID形式のState属性があればその値を引き継ぐ）。
2. **Identity 受信時（State属性なし）:**
   - **Identity種別判定:** 先頭文字とrealm有無で認証方式を判別。
     - 永続ID(0,6): 通常フロー継続、`eap_type`を決定
     - 仮名/再認証ID(2,4,7,8): AT_PERMANENT_ID_REQでフル認証誘導
     - 非対応(1,3,5,realmなし): EAP-Failure返却
   - `eap:{TraceID}` を作成（TTL 60秒）。
   - Vector Gateway (`POST /api/v1/vector`) をコールしてベクター取得。
     - Header `X-Trace-ID` に Trace ID を付与。
     - *Note: サーキットブレーカーを実装し、Vector Gateway障害時はエラー応答する。*
   - **鍵導出処理:** Vector Gateway応答（RAND, AUTN, XRES, CK, IK）から K_aut, MSK を導出。
     - EAP-AKA: MK = SHA1(Identity|IK|CK) → PRF で K_encr, K_aut, MSK, EMSK 導出
     - EAP-AKA': CK', IK' = f(CK, IK, AUTN, Network Name) → PRF' で K_aut, MSK 等を導出
   - `eap:{TraceID}` に stage=`CHALLENGE_SENT`, rand, autn, xres, k_aut, msk, imsi, eap_type を保存（TTLリセット）。
   - `Access-Challenge` (State=Trace ID) を返却。
3. **Challenge Response 受信時:**
   - State属性から `eap:{UUID}` を取得（存在しなければ `EAP_CTX_NOT_FOUND` で Reject）。stage が `CHALLENGE_SENT` であることを確認。
   - `k_aut` を使用して `AT_MAC` を検証、`XRES` と `AT_RES` を比較検証。不一致なら Reject（`AUTH_MAC_INVALID` / `AUTH_RES_MISMATCH`）。
   - **【Post-Auth Policy Check】**
     - `HGETALL policy:{IMSI}` を取得・パース。不在・JSON不正なら Reject（`AUTH_POLICY_NOT_FOUND`）。
     - RADIUSリクエスト内の `NAS-Identifier` / `Called-Station-Id`(SSID) とルールを照合（セクション2.C）。
     - **拒否:** `Access-Reject` を返却（`AUTH_POLICY_DENIED`）。
     - **許可:** `Access-Accept` を返却。
       - `sess:{SessionUUID}` を作成（imsi, nas_ip, nas_identifier, start_time ほか。TTL 24時間）し、`SADD idx:user:{IMSI} {SessionUUID}`。
       - `eap:{UUID}` を削除。
       - 採用ルールの `vlan_id` -> `Tunnel-Type` / `Tunnel-Medium-Type` / `Tunnel-Private-Group-Id` AVPへ。
       - 採用ルールの `session_timeout` -> `Session-Timeout` AVPへ。
       - `msk` から MS-MPPE-Recv-Key, MS-MPPE-Send-Key を生成。
       - Class属性に Session UUID をセット。
4. **再同期要求受信時:**
   - `eap:{UUID}` の stage が `CHALLENGE_SENT` であることを確認し、`resync_count` を取得。
   - **上限チェック（32回）:** `resync_count` が32以上なら `Access-Reject`（`AUTH_RESYNC_LIMIT`）。
   - Vector Gateway を再同期情報（RAND, AUTS）付きでコール。
   - 成功時は新しいVectorで鍵を再導出し、`resync_count` を+1して新しいChallengeを送信。

### Acct Server (UDP 1813)

1. **受信時 (共通):**
   - Auth Server同様、Valkey `client:{IP}` -> 環境変数 `RADIUS_SECRET` の順でSecretを特定し検証。
   - Class属性を `uuid.Parse()` で検証し、Session UUID とする。
2. **Acct-Start 受信時:**
   - `acct:seen:{Acct-Session-Id}` で重複・順序異常を判定（セクション3.G）。
   - Class属性なし・不正、または `sess:{UUID}` 不在 → `ACCT_SESSION_NOT_FOUND`（WARN）、セッション更新なしで処理継続。
   - 存在すれば start_time, nas_ip, acct_id, client_ip（Framed-IP-Address があるとき）、nas_identifier（NAS-Identifier があるとき）を保存 (HSET)、TTL延長 (EXPIRE 24h)。
3. **Acct-Interim 受信時:**
   - `acct:seen:{Acct-Session-Id}` で重複・順序異常を判定（セクション3.G）。
   - Class属性があれば `sess:{UUID}` の存在を確認し、存在すれば nas_ip, input_octets, output_octets, client_ip（Framed-IP-Address があるとき）、nas_identifier（NAS-Identifier があるとき）を保存 (HSET)、TTL延長 (EXPIRE 24h)。不在なら `ACCT_SESSION_NOT_FOUND`（WARN）、セッション更新・作成なしで処理継続。
4. **Acct-Stop 受信時:**
   - `acct:seen:{Acct-Session-Id}` を `stop` に更新。
   - `sess:{UUID}` から imsi を取得したうえで削除 (DEL)。
   - `idx:user:{IMSI}` からUUIDを削除 (SREM)。
5. **Accounting-On/Off 受信時:**
   - ログ出力（`ACCT_ON` / `ACCT_OFF`）のみ。Valkeyは操作しない。

※ `idx:user:{IMSI}` への追加 (SADD) は Auth Server が Access-Accept 時に行う。Acct Server は追加しない。

### Admin TUI

1. **管理機能:**
   - `sub:{IMSI}`, `client:{IP}`, `policy:{IMSI}` の CRUD操作（一覧は SCAN + パイプライン HGETALL、一括登録は TxPipeline）。
   - `sub:{IMSI}` の編集は Lua スクリプトで存在チェックと更新をまとめて行い、`sqn` は SQN を変更したときだけ、編集開始時の値との比較・置き換えで書き換える（セクション2.A）。
2. **モニタリング:**
   - セッション一覧: `sess:*` を SCAN して表示。
   - IMSI指定: `idx:user:{IMSI}` から `sess:{UUID}` を取得して表示（セクション3.F のクリーンアップを実施）。

------

## 5. Go 構造体定義

### 5.1 共通モデル（`pkg/model`）

`pkg/model` パッケージで定義される構造体。jsonタグのみを使用し、redisタグは付与しない。現行実装で利用しているのは Admin TUI（`Subscriber` / `RadiusClient` / `Session`）であり、`Policy` / `PolicyRule` / `Stage` / `EAPContext` は定義のみで、アプリケーションからは参照されていない。

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
    NasID          string   `json:"nas_id"`                    // NAS識別子（"*" 単独で任意のNASに一致。それ以外は完全一致）
    AllowedSSIDs   []string `json:"allowed_ssids"`             // 許可SSIDリスト
    VlanID         string   `json:"vlan_id,omitempty"`         // VLAN ID（空文字は未設定）
    SessionTimeout int      `json:"session_timeout,omitempty"` // セッションタイムアウト秒（0は未設定）
}

// --- State Data ---

// Stage はEAP認証のステージを表す型（小文字。Auth Serverは使用しておらず、Valkeyの stage 値とは異なる）
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

type EAPContext struct {
    TraceID              string `json:"trace_id"`               // トレース識別子
    IMSI                 string `json:"imsi"`                   // 加入者IMSI
    EAPType              uint8  `json:"eap_type"`               // 23=EAP-AKA, 50=EAP-AKA'
    Stage                Stage  `json:"stage"`                  // 認証ステージ
    RAND                 string `json:"rand"`                   // Hex
    AUTN                 string `json:"autn"`                   // Hex
    XRES                 string `json:"xres"`                   // Hex
    Kaut                 string `json:"kaut"`                   // Hex（Valkeyのフィールド名は k_aut）
    MSK                  string `json:"msk"`                    // Hex
    ResyncCount          int    `json:"resync_count"`           // 再同期試行回数
    PermanentIDRequested bool   `json:"permanent_id_requested"` // フル認証誘導済みフラグ
}

func NewEAPContext(traceID, imsi string, eapType uint8) *EAPContext

type Session struct {
    UUID          string `json:"uuid"`            // セッション識別子（Valkeyではキー sess:{UUID} の一部）
    IMSI          string `json:"imsi"`            // 加入者IMSI
    NasIP         string `json:"nas_ip"`          // NAS IPアドレス（パケットの送信元IP。プロキシ経由ではプロキシのIP）
    NasIdentifier string `json:"nas_identifier"`  // NAS-Identifier（プロキシ経由でもNASを識別できる）
    ClientIP      string `json:"client_ip"`       // クライアントIPアドレス
    AcctSessionID string `json:"acct_session_id"` // Acct-Session-Id（Valkeyのフィールド名は acct_id）
    StartTime     int64  `json:"start_time"`      // セッション開始時刻（Unix秒）
    InputOctets   int64  `json:"input_octets"`    // 受信バイト数
    OutputOctets  int64  `json:"output_octets"`   // 送信バイト数
}

func NewSession(uuid, imsi, nasIP, clientIP, acctSessionID string, startTime int64) *Session
```

### 5.2 Valkey Hash の読み書きに使う各アプリの構造体

Valkey の Hash と直接対応するのは、各アプリ内で定義した構造体である。

```go
// apps/auth-server/internal/session/context.go — eap:{UUID}
type EAPContext struct {
    IMSI                 string `redis:"imsi"`
    Stage                string `redis:"stage"`    // eap.EAPState の値（大文字）
    EAPType              uint8  `redis:"eap_type"`
    RAND                 string `redis:"rand"`
    AUTN                 string `redis:"autn"`
    XRES                 string `redis:"xres"`
    Kaut                 string `redis:"k_aut"`
    MSK                  string `redis:"msk"`
    ResyncCount          int    `redis:"resync_count"`
    PermanentIDRequested bool   `redis:"permanent_id_requested"`
}

// apps/auth-server/internal/session/session.go — sess:{UUID}
// apps/acct-server/internal/session/types.go も同じフィールド構成
type Session struct {
    IMSI          string `redis:"imsi"`
    NasIP         string `redis:"nas_ip"`
    NasIdentifier string `redis:"nas_identifier"` // ポリシー評価に使ったNAS-Identifier（プロキシ経由でもNASを識別できる）
    StartTime     int64  `redis:"start_time"`
    ClientIP      string `redis:"client_ip"`
    AcctID        string `redis:"acct_id"`
    InputOctets   int64  `redis:"input_octets"`
    OutputOctets  int64  `redis:"output_octets"`
}

// apps/auth-server/internal/store/client.go — client:{IP}（Auth/Acct Serverは secret のみ HGET）
type RadiusClient struct {
    IP     string `redis:"-"`
    Secret string `redis:"secret"`
    Name   string `redis:"name"`
    Vendor string `redis:"vendor"`
}

// apps/auth-server/internal/policy/types.go — policy:{IMSI}（default / rules は store/policy.go で個別に解釈）
type Policy struct {
    Rules   []PolicyRule
    Default string // "allow" or "deny"
}

type PolicyRule struct {
    NasID          string   `json:"nas_id"`
    AllowedSSIDs   []string `json:"allowed_ssids"`
    VlanID         string   `json:"vlan_id,omitempty"`
    SessionTimeout int      `json:"session_timeout,omitempty"`
}

// apps/vector-api/internal/store/subscriber.go — sub:{IMSI}（HGETALL の結果を手動で詰め替え）
type Subscriber struct {
    IMSI string
    Ki   string // Hex 32桁
    OPc  string // Hex 32桁
    AMF  string // Hex 4桁
    SQN  string // Hex 12桁
}
```

> **ストア層変換方式の補足:**
> - Auth Server / Acct Server: `internal/store/convert.go` の `StructToMap` / `MapToStruct` が、上記のアプリ内構造体の `redis` タグをリフレクションで読み、`map[string]any` ⇔ 構造体を変換する（対応型: string, int/int64, uint8, bool）。bool は `1` / `0` として保存される。Acct Server の Start/Interim 更新は、更新対象フィールドだけの map を直接 HSET する
> - Admin TUI: `pkg/model` の構造体を使い、`pkg/masterdata` の関数（`subscriberFromHash`, `clientFromHash`。E-03 §9）と Admin TUI の `internal/store` の関数（`mapToSession` 等）で Hash フィールドと手動で対応付ける。`sub:` / `client:` / `policy:` の作成と変更は、存在確認と書き込みを1つの Lua スクリプトで行う（同じキーを同時に作成しても上書きしない）。`model.Session.AcctSessionID` は Valkey の `acct_id` に対応する。`sub:{IMSI}` の編集（`Update` / `UpdateWithSQN`）は Lua スクリプト（`updateSubscriberScript`）で行う（セクション2.A）
> - Vector API: `sub:{IMSI}` を HGETALL して手動で詰め替え、`sqn` のみ Lua スクリプト（`CompareAndSetSQN`）で比較・置き換えする（セクション2.A）
> - `pkg/model` の構造体には redis タグを付与しない設計とし、Valkey 実装に依存させない

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
| r12 | 2026-10-04 | 実装コードとの突き合わせによる修正: (1) 2.C 認可ポリシー: `nas_id` はワイルドカード不可の完全一致（大文字小文字区別）に訂正し、JSON例の `nas_id` "*" を削除。`allowed_ssids` の `"*"`・大文字小文字無視、評価順序（配列順で最初の一致）、SSID抽出、`default` の欠落・不正値（deny扱い）、ポリシー不在・JSON不正時のReject、`vlan_id`（JSON文字列、Tunnel属性3種）・`session_timeout`（>0で付与）、default allow時は属性なし、を明記 (2) 2.A: SQN更新は CAS ではなく単純な HSET（+32、再同期は SQN_MS+32、12桁小文字hex）であることに訂正し、テストベクターモード時のSQN扱い（`sqn` のみの Hash 作成を含む）を追記。`created_at` の形式を明記 (3) 2.B: name/vendor はサーバー未使用、Valkeyエラー時もフォールバックすることを明記 (4) 3.D: stage は大文字の `EAPState` 値で保存されることに訂正（実際に書かれる値を明記）、格納形式・TTLリセット・削除タイミング・書き込みの流れを追記 (5) 3.E: 各フィールドの書き込みタイミングを Auth Accept 時の作成を含めて訂正、実装に無い `ACCT_SESSION_EXPIRED` を削除し Start/Interim/Stop の不在時動作を実装どおりに記載 (6) 3.F: SADD は Auth Server の Accept 時であることを明記、Admin TUI の SCAN フォールバックを追記 (7) 3.G: 重複検出ロジックを実装どおりに訂正（Interim の `no_start_received` が実質出力されない点、Stop重複時は以降の処理をスキップ） (8) 4: 各サーバーのフローを実装に合わせて修正（Acct Server は idx:user に SADD しない、Accounting-On/Off を追記） (9) 1: 接続・認証・永続化設定を compose に合わせて修正、Type列追加、`stats:global` 未使用を注記 (10) 5: `pkg/model` の利用状況を明記し、Valkey と直接対応する各アプリの redis タグ付き構造体を追記。D-04 r19 の event_id 全面整合に合わせて修正（2.E の不在時動作の注記から実装に存在しない event_id 名 `ACCT_SESSION_EXPIRED` を削除し、TTL超過・未作成とも `ACCT_SESSION_NOT_FOUND` である旨に修正） |
| r13 | 2026-10-04 | acct-server の Interim シーケンス判定修正の反映: 3.E Acct Server のセッション処理で、Interim も `sess:{UUID}` の存在を確認し、不在時は `ACCT_SESSION_NOT_FOUND` を出力してセッションを作成しない（`imsi` を持たない Hash が作られる問題を解消）ことに修正、TTLの記述を補足。3.G 重複検出ロジックの Interim を実装どおりに修正（1回の GET で判定してから SET、値なしは `no_start_received`、`stop` は新設の `interim_after_stop` として `ACCT_SEQUENCE_ERR` を出力し処理継続。「`no_start_received` が出力されない」実装上の制約の記載を削除）。4 Acct Server の Interim フローを更新。D-10 の参照セクション番号を修正（5.6→5.8） |
| r14 | 2026-10-04 | テストベクターモードでも加入者登録を必須にした Vector API の実装修正の反映（2.A）: テストベクターモードで置き換えるのは Ki/OPc/AMF だけで、`sub:{IMSI}` の取得・`sqn` の解析と書き戻し・エラー処理は通常モードと同じ（未登録は404、Valkeyエラー・書き戻し失敗は500、`sqn` 解析エラーも500）に修正。既定 SQN `ff9bb4d0b607` へのフォールバック、`sqn` だけを持つ Hash の作成、`TEST_SQN_PERSIST_ERR` の記述を削除し、旧実装で作られた `sqn` だけの Hash の扱いを注記。SQN更新方式の注記に、書き戻し失敗時・未登録時の扱いがテストベクターモードでも同じである旨を追記 |
| r15 | 2026-10-04 | ポリシーの `nas_id` で `"*"` を任意の NAS に一致させた Auth Server の実装修正の反映（2.C）: `nas_id` の説明を「`"*"` 単独は任意の NAS-Identifier（NAS-Identifier が無い場合を含む）に一致、部分一致は行わない、それ以外は完全一致（大文字小文字区別）」に改め、r12 で記載した「ワイルドカード不可」を削除。JSON 例に `nas_id` `"*"` のルールを追加し、評価順（個別のNASのルールを前に置く）を説明。評価ロジック2に `nas_id` / `allowed_ssids` の一致条件を追記。5 の `PolicyRule.NasID` のコメントを `pkg/model` の更新後のコメントに合わせて修正 |
| r16 | 2026-10-04 | auth-server の trace_id 引き継ぎの実装修正の反映: 3.D の UUID の「生成」を、State属性の無い初回 Access-Request で生成し、以降は UUID 形式の State属性の値をそのまま Trace ID とする（ハンドラー層のログを含め1回の認証で同じ値。UUID 形式でない State は新規 UUID）記述に修正、4章の処理フロー 1. の Trace ID の「生成」を「決定」に修正 |
| r17 | 2026-10-04 | acct-server の重複 Interim の event_id 分離の実装修正の反映（3.G）: 重複・順序異常の検出ロジックで、直前と同一の `interim:{input}:{output}` の Interim（重複）に出力する event_id を `ACCT_DUPLICATE_START` から `ACCT_DUPLICATE_INTERIM` に変更（重複 Start は従来どおり `ACCT_DUPLICATE_START`） |
| r18 | 2026-10-04 | SQN競合制御の実装（Lua による `sqn` の比較・置き換え、競合時のやり直し最大3回、HTTP 409）の反映（2.A、5.2）: `sqn` フィールドの備考と「SQN更新方式（現行実装）」を、単純な HSET による後勝ちから、Lua スクリプト（`CompareAndSetSQN`。EVALSHA）で読んだ値と一致するときだけ書き換える方式に改めた。スクリプト本体、比較は `sqn` フィールドのみ・大文字小文字を正規化しない・キー不在時は作成しないこと、ベクター計算を書き換え成功後に行う順序、競合時のやり直し（1〜10ms待機、最大3回、`SQN_CONFLICT_RETRY`）と上限超過・期限切れ時の 409（`SQN_CONFLICT_ERR`）、再同期のやり直しで同期済みとみなす場合、書き換え後のベクター計算失敗時は SQN が進んだままになること、守る性質（IMSIごとに一意・単調増加）、WATCH/MULTI 設計からの経緯を記載。「WATCH/MULTI による CAS・リトライ・HTTP 409 は実装していない」の記述を削除。残っている制約として Admin TUI の加入者編集が `sqn` を上書きし巻き戻りうること（別PRで対応予定）を追記。5.2 ストア層変換方式の補足の Vector API の書き戻し方法を更新 |
| r19 | 2026-10-04 | Admin TUI の加入者編集による `sqn` の上書き（巻き戻り）を解消した実装修正の反映（2.A、4、5.2）: r18 で「残っている制約」として記載した Admin TUI による `sqn` の上書きを解消済みとし、「Admin TUI からの書き込み（加入者編集）」に改めた。Lua スクリプト（`updateSubscriberScript`）で存在チェックと更新をまとめて行うこと、SQN を変更していないとき（大文字小文字の違いだけを含む）は `ki` / `opc` / `amf` だけを更新し `sqn` を書き換えないこと、変更したときは編集開始時の値と一致する場合だけ更新し一致しなければ何も更新しないこと、加入者が削除されていればキーを作らないこと、新規作成・CSVインポートは例外であることを記載。`sqn` フィールドの備考を補足。Hex表記の「入力どおりに保存する」を「大文字に正規化して保存する」に訂正。4 の Admin TUI のデータアクセスと 5.2 ストア層変換方式の補足に編集時の Lua スクリプトを追記 |
| r20 | 2026-10-06 | `sess:{UUID}` に `nas_identifier`（NAS-Identifier）を追加した実装修正の反映: radsecproxy 等のプロキシ経由では `nas_ip`（送信元IP）がプロキシのIPになり NAS を区別できないため。Auth Server が Accept 時にポリシー評価に使った NAS-Identifier を書き、Acct Server が Start / Interim で NAS-Identifier があれば上書きする（なければ残す）。§2 のフィールド表・処理フロー・§5 の構造体を更新し、`nas_ip` がプロキシ経由ではプロキシのIPになる旨を追記 |
| r21 | 2026-10-07 | Admin TUI の加入者・RADIUSクライアント・認可ポリシーの store と validation を pkg に移した実装修正（Provisioning API（D-13）と共通で使うため。E-03 r11）の反映: §2.A の Admin TUI の加入者編集の実装箇所を `pkg/masterdata/subscriber.go` に、§5.2 のストア層変換方式の補足を `pkg/masterdata` に修正し、作成・変更を Lua スクリプトで原子的に行う旨を追記。Admin TUI の `internal/model` は廃止して `pkg/model` に統合 |
