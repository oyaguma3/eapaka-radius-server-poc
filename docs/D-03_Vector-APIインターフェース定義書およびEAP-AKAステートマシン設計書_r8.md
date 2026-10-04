# D-03 Vector-APIインターフェース定義書およびEAP-AKAステートマシン設計書 (r8)

---

# 1. 内部APIインターフェース定義書 (Vector Gateway経由)

Auth Server が Vector Gateway 経由で Vector API に対して行う「認証ベクター取得」および「再同期（Resync）」のリクエスト仕様です。

> **注記:** Auth ServerはVector Gatewayに対してリクエストを送信する。Vector GatewayはIMSIのPLMNに応じて接続先を振り分け（PLMNベースルーティング）、内部Vector API（接続方式ID `00`）または外部の aka-only-server（接続方式ID `01`）からベクターを取得して応答を返却する。どちらに振り分けられても本章の内部IF（リクエスト/レスポンス形式）は同一である。詳細はD-12を参照。

### 基本情報

- **Protocol:** HTTP/1.1 (REST)
- **Base URL:** `http://vector-gateway:8080/api/v1`
- **Content-Type:** `application/json`
- **Encoding:** UTF-8

> **接続構成:**
> ```
> Auth Server → Vector Gateway ─┬─ (00) → Vector API → Valkey
>                                └─ (01) → aka-only-server（外部、mTLS/平文HTTP）
> ```
>
> 接続方式 `01` は `VECTOR_GATEWAY_PLMN_MAP` で `01` を指定したPLMNにのみ適用される（未一致PLMNおよびpassthroughモードは `00`）。
> 
> Auth Serverは環境変数 `VECTOR_API_URL` で接続先を指定する。PoC環境では `http://vector-gateway:8080/api/v1/vector` を設定する。

### 1.1 認証ベクター取得 (Generate Vector)

通常認証時、および再同期（Resync）時に使用する単一のエンドポイントです。

- **Full URL:** `http://vector-gateway:8080/api/v1/vector`
- **Method:** `POST`
- **Headers:**
  - `Content-Type`: `application/json`
  - `X-Trace-ID`: **必須** (Auth Serverが生成したUUID)

> **注記:** Vector GatewayはX-Trace-IDを内部Vector APIへ伝搬し、エンドツーエンドのトレーサビリティを確保する。接続方式 `01` の場合もX-Trace-IDをベストエフォートで付与するが、aka-only-server はこれを記録しない（ログ突き合わせはD-04参照）。

#### Request Body

```json
{
  "imsi": "440101234567890",
  
  // 再同期(Auts受信)時のみセットするオブジェクト
  // 通常時は null または省略
  "resync_info": {
    "rand": "f4b3...", // クライアントに送った元のRAND (Hex)
    "auts": "9a2c..."  // クライアントから返ってきたAUTS (Hex)
  }
}
```

#### Response Body (200 OK)

成功時、AKA認証に必要な5つのパラメータを返します。すべてHex文字列です。

```json
{
  "rand": "f4b38a...", // Random Challenge (16 bytes)
  "autn": "2b9e10...", // Authentication Token (16 bytes)
  "xres": "d8a1...",   // Expected Response (4-16 bytes)
  "ck":   "91e3...",   // Cipher Key (16 bytes)
  "ik":   "c42f..."    // Integrity Key (16 bytes)
}
```

#### Error Response

RFC7807 (Problem Details) に準拠した形式で返します。

| HTTPステータス | 説明 | 発生元 |
|---------------|------|--------|
| 400 Bad Request | IMSIのフォーマット不正、またはSQN同期計算に失敗（MAC検証失敗など） | Vector API |
| 400 Bad Request | aka-only-server がリクエスト不正と判定（cause: INVALID_MSG_FORMAT ほか） | Vector Gateway（接続方式01） |
| 403 Forbidden | aka-only-server が認証ベクター払い出しを拒否（cause: AUTHENTICATION_REJECTED。下表参照） | Vector Gateway（接続方式01） |
| 404 Not Found | 指定されたIMSIがValkeyに存在しない | Vector API |
| 404 Not Found | 指定されたIMSIが aka-only-server に登録されていない（cause: USER_NOT_FOUND） | Vector Gateway（接続方式01） |
| 409 Conflict | 同一IMSIへの並行リクエストでSQNの書き換えが競合し、やり直し（最大3回試行）でも書き換えられなかった（D-11参照） | Vector API |
| 500 Internal Server Error | Valkey接続エラー、Milenage計算の予期せぬエラー | Vector API |
| 501 Not Implemented | 未実装の接続方式IDが指定された | Vector Gateway |
| 502 Bad Gateway | Vector GatewayからVector APIへの通信エラー | Vector Gateway |
| 502 Bad Gateway | aka-only-server との通信失敗（タイムアウト・接続失敗・TLSハンドシェイク失敗）、5xx/501応答、不正な応答 | Vector Gateway（接続方式01） |

> **注記:** 
> - 501 Not Implementedは、PLMNマップで未実装（または未設定）の接続方式IDが指定された場合にVector Gatewayが返却する（下記の接続方式 `01` の注記を参照）。
> - 409 Conflictは、Vector APIのSQN競合制御（Lua スクリプトによる `sqn` の比較・置き換え。D-02 §2.A、D-11 §7.5・§13.6）で、3回とも競合した場合（またはやり直し前の待機中にリクエストが期限切れ・キャンセルとなった場合）に返却される。`title` は `Conflict`、`detail` は固定文 `SQN update conflict`（IMSIや原因は含めない）。Vector Gatewayは他の4xxと同様にそのまま中継する。Auth Serverは Circuit Breaker の失敗回数に数えず、Access-Reject とする（端末は認証をやり直す）。5xxではなく409とするのは、1つのIMSIの競合で Circuit Breaker が開き全体の認証が止まるのを避けるためである（D-06 §3.4.3）。r7 までは「Phase 1で追加された WATCH/MULTI CAS」と記載していたが、WATCH/MULTI は設計のみで、実装は Lua による比較・置き換えである。
> - 接続方式 `01`（aka-only-server）は実装済み。`VECTOR_GATEWAY_AKAONLY_URL` が未設定の場合は `01` も未登録となり、`01` を指定したPLMNは501となる。`02`〜`99` は未実装（501）。
> - 接続方式 `01` の 400/403/404 は、aka-only-server の ProblemDetails（title/status/detail/cause）を Vector Gateway が RFC7807 形式（`type: about:blank`、title/status/detail を引き継ぎ）に詰め替えたもの。`cause` は内部IFに含まれず、Vector Gatewayのログにのみ出力される（D-04参照）。

**接続方式01で 403 Forbidden となるケース:**

接続方式 `01` 経由の場合に限り、以下の3ケースで 403 が返る。`detail` で区別する。

| # | ケース | detail |
|---|--------|--------|
| 1 | Vector Gateway のクライアント証明書が、当該加入者の許可クライアントに含まれていない | `client is not allowed for this subscriber` |
| 2 | 再同期（`resync_info` あり）で AUTS 検証に失敗した | `AUTS verification failed` |
| 3 | 平文HTTP接続で、当該加入者の平文HTTP許可フラグが立っていない | `client is not allowed for this subscriber`（#1と同じ） |

> **注記:**
> - 再同期時の AUTS 検証失敗は、内部Vector API（接続方式00）では **400**、接続方式01では **403** となる。
> - Auth Server は 200 以外の 4xx（400/403/404/409 等）を Circuit Breaker 対象外の APIError として扱い、Access-Reject とする。403 は 404/400 と同様に扱われるため、Auth Server 側の変更は不要（ログは 403 → `VECTOR_API_ERR`（http_status=403）、404 → `VECTOR_IMSI_NOT_FOUND`）。
> - 502 は Auth Server の Circuit Breaker の失敗回数に数えられる。Circuit Breaker は接続方式ごとに分かれていないため、`01` 向けの通信失敗が続いて開くと `00` 向けの認証も停止する。Vector Gateway は `01` 失敗時に `00` へのフォールバックを行わない（SQN競合回避のため）。エラー変換の詳細はD-06を参照。

```json
{
  "type": "about:blank",
  "title": "User Not Found",
  "detail": "IMSI does not exist in subscriber DB",
  "status": 404
}
```

------

# 2. EAP-AKA/AKA' ステートマシン詳細設計書 (Auth Server)

各状態で「何を受信したら」「何をして」「どの状態へ遷移するか」を定義します。

## 2.1 PoC対象外機能

本PoCでは以下の機能は実装対象外とする。

| 機能 | 説明 | 受信時の対処 |
|------|------|-------------|
| 仮名認証 | Pseudonym IDによる認証 | フル認証へ誘導（AT_PERMANENT_ID_REQ） |
| 高速再認証 | Fast Re-authentication | フル認証へ誘導（AT_PERMANENT_ID_REQ） |
| EAP-Notification | 通知メッセージ送信 | 使用しない |

## 2.2 状態定義 (State Definition)

Valkeyの `eap:{UUID}` 内の `stage` フィールドで管理します。

| **状態名** | **説明** | **タイムアウト時** |
|-----------|----------|-------------------|
| **NEW** | セッション開始直後。初期状態。 | FAILURE |
| **WAITING_IDENTITY** | 仮名/再認証ID受信後、`AT_PERMANENT_ID_REQ`送信済み。永続ID応答待ち。 | FAILURE |
| **IDENTITY_RECEIVED** | 永続ID（IMSI）受領済み。Vector Gateway呼び出し前。 | FAILURE |
| **WAITING_VECTOR** | Vector Gateway へリクエスト中。HTTPレスポンス待ち。 | FAILURE |
| **CHALLENGE_SENT** | `EAP-Request/AKA-Challenge` 送信済み。クライアント応答待ち。 | FAILURE |
| **RESYNC_SENT** | 再同期処理中。Vector Gatewayへ再同期リクエスト中。 | FAILURE |
| **SUCCESS** | `EAP-Success` 送信済み。認証完了（成功）。 | (終了状態) |
| **FAILURE** | `EAP-Failure` 送信済み。認証完了（失敗）。 | (終了状態) |

> **注記:**
> - 全ての非終了状態において、タイムアウト（EAPコンテキストTTL超過）時は`FAILURE`へ遷移する
> - 高速再認証用の`REAUTH_IN_PROGRESS`状態、通知用の`NOTIFICATION_SENT`状態は本PoCでは使用しない

## 2.3 状態遷移図 (State Transition Diagram)

```
[NEW]
   │
   ├── EAP-Response/Identity受信
   │       │
   │       ├── [永続ID (0,6)] ─────────────────────────────────► [IDENTITY_RECEIVED]
   │       │                                                            │
   │       ├── [仮名/再認証ID (2,4,7,8)] ──► [WAITING_IDENTITY]         │
   │       │                                       │                    │
   │       │                                       │ EAP-Response/      │
   │       │                                       │ AKA-Identity受信   │
   │       │                                       │       │            │
   │       │                                       │       ├── [永続ID] ─┘
   │       │                                       │       │
   │       │                                       │       └── [非対応/不正] ──► [FAILURE]
   │       │                                       │
   │       │                                       └── [Client-Error] ──► [FAILURE]
   │       │
   │       └── [非対応/不正 (1,3,5,realmなし)] ──► [FAILURE]

[IDENTITY_RECEIVED]
   │
   └── Vector Gateway呼び出し ──► [WAITING_VECTOR]
                                        │
                                        ├── [API成功] ──► Challenge送信 ──► [CHALLENGE_SENT]
                                        │
                                        └── [APIエラー] ──► [FAILURE]

[CHALLENGE_SENT]
   │
   ├── EAP-Response/AKA-Challenge受信
   │       │
   │       ├── [MAC/RES検証OK] ──► Post-Auth Policy評価
   │       │                              │
   │       │                              ├── [ルール一致] ──► [SUCCESS]
   │       │                              │
   │       │                              ├── [ルール不一致 + default=allow] ──► [SUCCESS]
   │       │                              │
   │       │                              ├── [ルール不一致 + default=deny] ──► [FAILURE]
   │       │                              │
   │       │                              └── [ポリシー未設定/不正] ──► [FAILURE]
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

## 2.4 イベント処理ロジック (Event Handlers)

### 1. 初期処理 (Entry Point)

- **Current State:** (なし)
- **Trigger:** RADIUS `Access-Request` 受信 (EAP-Messageなし、またはEAP-Start)
- **Action:**
  1. Trace ID（UUID）生成。
  2. `EAP-Request/Identity` を作成。
  3. RADIUS `Access-Challenge` にEAPを包んで返信。
  4. Valkey `eap:{UUID}` 作成 (Stage: `NEW`)。
- **Next State:** `NEW`

### 2. Identity受信 (Receive Identity)

- **Current State:** `NEW` または `WAITING_IDENTITY`
- **Trigger:** RADIUS `Access-Request` (EAP-Response/Identity または EAP-Response/AKA-Identity)
- **Action:**
  1. EAPパケットから Identity を抽出。
  2. Identity種別を判定（先頭文字とrealm有無で分類）。

#### 2a. 永続ID受信時（通常フロー）

- **Condition:** Identity先頭が `0`(EAP-AKA) または `6`(EAP-AKA') かつ realm あり
- **Action:**
  1. IMSIを抽出。
  2. Valkey `eap:{UUID}` に IMSI, EAPType を保存。
  3. **Next State:** `IDENTITY_RECEIVED`
  4. Vector Gateway (`POST /api/v1/vector`) をコール。
     - Request: `{"imsi": "..."}`
     - Header: `X-Trace-ID`
  5. **Next State:** `WAITING_VECTOR`
- **Branch (Vector Gateway応答):**
  - **API成功 (200 OK):**
    1. 取得したVector (RAND, AUTN, XRES, CK, IK) から鍵導出。
    2. 導出した鍵情報 (K_aut, MSK等) を Valkey `eap:{UUID}` に保存。
    3. `EAP-Request/AKA-Challenge` または `EAP-Request/AKA'-Challenge` を作成。
       - AT_RAND, AT_AUTN, AT_MAC を含む（EAP-AKA'の場合はAT_KDF, AT_KDF_INPUTも含む）
    4. RADIUS `Access-Challenge` 返信。
    5. **Next State:** `CHALLENGE_SENT`
  - **APIエラー (404 Not Found):**
    1. ログ出力（`VECTOR_IMSI_NOT_FOUND`）。
    2. `EAP-Failure` を作成。
    3. RADIUS `Access-Reject` 返信。
    4. **Next State:** `FAILURE`
  - **APIエラー (400/403/500/501/502等) / 接続失敗・タイムアウト / Circuit Breaker Open:**
    1. ログ出力（HTTPエラー応答: `VECTOR_API_ERR`、接続失敗・タイムアウト: `VECTOR_CONN_ERR`、Circuit Breaker Open: `VECTOR_CB_OPEN`、その他: `VECTOR_UNKNOWN_ERR`）。
    2. `EAP-Failure` を作成。
    3. RADIUS `Access-Reject` 返信。
    4. **Next State:** `FAILURE`

#### 2b. 仮名/高速再認証ID受信時（フル認証誘導）

- **Condition:** Identity先頭が `2`,`4`(EAP-AKA) または `7`,`8`(EAP-AKA')
- **Current State制約:** `NEW` のみ（`WAITING_IDENTITY` で再度受信した場合は 2c へ）
- **Action:**
  1. Identity種別を判定（フル認証誘導時の専用ログは出力しない。初回の EAP-Response/Identity 受信時の `EAP_IDENTITY_RECEIVED` のみ）。
  2. Valkey `eap:{UUID}` に `permanent_id_requested: true` をセット。
  3. `EAP-Request/AKA-Identity` または `EAP-Request/AKA'-Identity` を作成。
     - `AT_PERMANENT_ID_REQ` を含める（RFC 4187 Section 4.1.4）。
  4. RADIUS `Access-Challenge` 返信。
  5. **Next State:** `WAITING_IDENTITY`

```
[Client]                              [Auth Server]
    |                                      |
    |  EAP-Response/Identity               |
    |  (Identity: 2<pseudonym>@realm)      |
    |------------------------------------->|
    |                                      |  State: NEW → WAITING_IDENTITY
    |  EAP-Request/AKA-Identity            |
    |  (AT_PERMANENT_ID_REQ)               |
    |<-------------------------------------|
    |                                      |
    |  EAP-Response/AKA-Identity           |
    |  (AT_IDENTITY: 0<IMSI>@realm)        |
    |------------------------------------->|
    |                                      |  State: WAITING_IDENTITY → IDENTITY_RECEIVED
    |  (通常のフル認証フローへ継続)          |
```

#### 2c. 非対応/不正ID受信時

- **Condition:** 以下のいずれか
  - Identity先頭が `1`,`3`,`5`(EAP-SIM)
  - realm なし
  - その他不正な形式
  - `WAITING_IDENTITY` 状態で再度仮名/再認証IDを受信（永続ID応答拒否）
- **Action:**
  1. ログ出力（`EAP_UNSUPPORTED_TYPE` または `EAP_IDENTITY_INVALID`）。
  2. `EAP-Failure` を作成。
  3. RADIUS `Access-Reject` 返信。
  4. **Next State:** `FAILURE`

### 3. Challenge応答受信 (Receive Challenge Response)

- **Current State:** `CHALLENGE_SENT`
- **Trigger:** RADIUS `Access-Request` (EAP-Response/AKA-Challenge)
- **Action:**
  1. Valkey `eap:{UUID}` から `K_aut`, `XRES` を取得。
  2. パケット内の `AT_MAC` を検証 (K_autを使用した改ざんチェック)。
  3. パケット内の `AT_RES` と `XRES` を比較。
- **Branch:**
  - **Match (検証成功):**
    1. **【Post-Auth Policy Check】** へ進む（後述）。
  - **Mismatch (検証失敗):**
    1. ログ出力（`AUTH_RES_MISMATCH` または `AUTH_MAC_INVALID`）。
    2. `EAP-Failure` を作成。
    3. RADIUS `Access-Reject` 返信。
    4. **Next State:** `FAILURE`

#### 3a. Post-Auth Policy Check

EAP認証成功後の認可処理。`policy:{IMSI}` を参照し、接続可否を判定する。

**処理フロー:**

1. Valkey `policy:{IMSI}` を取得。
2. **ポリシー取得結果による分岐:**

| 取得結果 | 処理 | ログ | Next State |
|---------|------|------|------------|
| キー不在 | Access-Reject | `AUTH_POLICY_NOT_FOUND` | FAILURE |
| JSONパース失敗 | Access-Reject | `AUTH_POLICY_NOT_FOUND`（専用のevent_idはなく、`error` 属性で区別） | FAILURE |
| 取得成功 | ルール評価へ | - | - |

3. **ルール評価:**
   - RADIUSリクエストから `NAS-Identifier` と `Called-Station-Id`（SSID: 最初の `:` より後ろ、`:` が無ければ全体）を取得。
   - ポリシー内の `rules` 配列を配列順に評価し、`nas_id` と `allowed_ssids` の両方に一致した最初のルールを採用する（採用したルールで許可）。
   - `nas_id`: `"*"` 単独は任意の NAS-Identifier（属性が無い場合を含む）に一致。部分一致は行わない。それ以外は完全一致（大文字小文字を区別）。
   - `allowed_ssids`: 要素に `"*"` があれば全SSIDに一致。それ以外は大文字小文字を区別しない完全一致。
   - 一致するルールが無ければ `default`（`allow` / `deny`）で判定する。詳細は D-02 セクション2.C を参照。

4. **評価結果による分岐:**

| 評価結果 | 処理 | ログ | Next State |
|---------|------|------|------------|
| ルール一致 | Access-Accept（ルールの `vlan_id` / `session_timeout` を付与） | - | SUCCESS |
| ルール不一致 + `default=allow` | Access-Accept（VLAN・Session-Timeoutなし） | - | SUCCESS |
| ルール不一致 + `default=deny` | Access-Reject | `AUTH_POLICY_DENIED` | FAILURE |

5. **Access-Accept時の処理:**
   - Valkey `sess:{UUID}` (セッション) を作成。
   - `EAP-Success` を作成。
   - RADIUS `Access-Accept` 返信。
     - `Class` 属性: Session UUID
     - `MS-MPPE-Recv-Key`, `MS-MPPE-Send-Key`: MSKから導出
     - ルール一致時のみ: `vlan_id` が空でなければ `Tunnel-Type`=13 / `Tunnel-Medium-Type`=6 / `Tunnel-Private-Group-Id`、`session_timeout` > 0 なら `Session-Timeout`
   - **Next State:** `SUCCESS`

6. **Access-Reject時の処理:**
   - `EAP-Failure` を作成。
   - RADIUS `Access-Reject` 返信。
   - **Next State:** `FAILURE`

### 4. 再同期要求受信 (Receive Synchronization Failure)

- **Current State:** `CHALLENGE_SENT`
- **Trigger:** RADIUS `Access-Request` (EAP-Response/AKA-Synchronization-Failure)
  - Contains `AT_AUTS`
- **Action:**
  1. Valkey `eap:{UUID}` から `resync_count` を取得し、インクリメント。
  2. **再同期リトライ上限チェック:**
     - `resync_count > 32` の場合:
       1. ログ出力（`AUTH_RESYNC_LIMIT`）。
       2. `EAP-Failure` 送信。
       3. **Next State:** `FAILURE`
  3. **Next State:** `RESYNC_SENT`
  4. Valkey `eap:{UUID}` から、前回送った `RAND` を取得。
  5. Vector Gateway (`POST /api/v1/vector`) をコール (Resyncモード)。
     - Request: `{"imsi": "...", "resync_info": {"rand": "...", "auts": "..."}}`
     - Header: `X-Trace-ID`
- **Branch (Vector Gateway応答):**
  - **API成功 (Resync成功):**
    1. 新しいVectorから鍵を再導出。
    2. Valkey `eap:{UUID}` を更新（新Vector情報、`resync_count`）。
    3. **新しい** `EAP-Request/AKA-Challenge` を作成して送信。
    4. **Next State:** `CHALLENGE_SENT`
  - **APIエラー (MAC不正など):**
    1. ログ出力（Auth Server: `VECTOR_API_ERR` 等。2a のAPIエラー時と同じ。接続方式00の場合は Vector API 側で `SQN_RESYNC_MAC_ERR` 等も出力される）。
       - AUTS 検証失敗は、接続方式00では 400、接続方式01（aka-only-server）では 403 で返る。
    2. `EAP-Failure` 送信。
    3. **Next State:** `FAILURE`

**再同期リトライ上限について：**
- 上限値: **32回**
- 根拠: SQN の IND (Index) フィールドが5ビット（0-31）であり、1サイクル分の試行を許容
- 32回を超えた場合は、SIM側またはネットワーク側に深刻な問題があると判断し、認証を拒否

### 5. クライアントエラー受信 (Receive Client Error)

- **Current State:** `CHALLENGE_SENT` または `WAITING_IDENTITY`
- **Trigger:** `EAP-Response/AKA-Client-Error`
- **Action:**
  1. ログ出力（`EAP_CLIENT_ERROR`。属性は `trace_id`, `imsi`。`AT_CLIENT_ERROR_CODE` のエラーコードは抽出・記録しない）。
     - エラーコードの詳細は RFC 4187 Section 10.20 を参照。
  2. `EAP-Failure` 送信。
  3. **Next State:** `FAILURE`

### 6. Authentication-Reject受信 (Receive Authentication Reject)

- **Current State:** `CHALLENGE_SENT`
- **Trigger:** `EAP-Response/AKA-Authentication-Reject`
- **Action:**
  1. ログ出力（`EAP_AUTH_REJECT`）。
     - クライアント（SIM）がネットワーク側のAUTNを検証失敗した場合に送信される。
     - 攻撃の可能性もあるため、WARNレベルで記録。
  2. `EAP-Failure` 送信。
  3. **Next State:** `FAILURE`

### 7. 不正な状態遷移 (Invalid State Transition)

- **Current State:** 期待される状態と異なる
- **Trigger:** 現在の状態で受信すべきでないEAPメッセージを受信
- **Action:**
  1. ログ出力（WARN。属性は `trace_id`, `stage`（現在の状態））。
     - `CHALLENGE_SENT` 以外で Challenge 応答／再同期応答（Synchronization-Failure）を受信: `EAP_STATE_ERR`
     - `WAITING_IDENTITY` 以外で AKA-Identity 応答を受信: `EAP_UNEXPECTED_IDENTITY`
     - 未知の Subtype を受信: `EAP_UNKNOWN_SUBTYPE`（属性は `trace_id`, `subtype`）
     - なお、状態遷移表による内部検証の失敗も `EAP_STATE_ERR`（ERROR、属性は `trace_id`, `error`）で出力される。
  2. `EAP-Failure` 送信。
  3. **Next State:** `FAILURE`

------

## 改訂履歴

| 版数 | 日付 | 内容 |
|------|------|------|
| r1 | - | 初版 |
| r2 | 2025-12-30 | PoC対象外機能明記（仮名認証、高速再認証、EAP-Notification）、フル認証誘導フロー追加（AT_PERMANENT_ID_REQ）、再同期リトライ上限（32回）追加、非対応EAP方式の処理追加、Authentication-Reject処理追加、RFC参照箇所修正 |
| r3 | 2026-01-12 | 状態定義の整理: IDENTITY_RECEIVED状態追加、タイムアウト時の遷移先をFAILUREに統一（ABORTED/TIMEOUT廃止）、PoC対象外状態を表から削除。状態遷移図をテキストベースに変更。Policy評価タイミングをPost-Authのみに変更（Pre-Auth Policy Check削除）。WAITING_VECTORおよびRESYNC_SENT状態の使用を明確化。不正な状態遷移時の処理（EAP_INVALID_STATE）を追加。Post-Auth Policy Checkの詳細化: デフォルトポリシー（default=allow/deny）の評価ロジック追加、ポリシー未設定/パースエラー時の処理明記。WAITING_IDENTITY状態からFAILURE遷移を明記（非対応/不正ID受信時、Client-Error受信時）。 |
| r4 | 2026-01-27 | API接続設計統一: セクション1のタイトルを「Vector Gateway経由」に変更、Base URLを`http://vector-gateway:8080/api/v1`に更新、接続構成図追加、Error Responseに409/501/502エラー追加（表形式に変更）、D-12参照追加 |
| r5 | 2026-02-18 | PolicyRule新構造反映: Post-Auth Policy Checkのルール評価をSSID/Action/TimeMin/TimeMax構造に更新、関連ドキュメント版数更新 |
| r6 | 2026-10-04 | 接続方式01（aka-only-server）対応: 接続構成図・注記を更新（内部IFは変更なし）、Error Responseに接続方式01由来の400/403/404/502を追加、403の3ケース（detailで区別）と再同期AUTS検証失敗が403となる点を明記、Auth ServerでのCB対象外扱い・502のCB計上を注記。既存記載の実装との不一致を修正（2a の404時ログを `AUTH_IMSI_NOT_FOUND` → `VECTOR_IMSI_NOT_FOUND`、APIエラー時ログに `VECTOR_CONN_ERR` / `VECTOR_CB_OPEN` / `VECTOR_UNKNOWN_ERR` を追記、再同期APIエラー時のAuth Server側ログを `VECTOR_API_ERR` 等に修正、404応答例の detail を実装の文言に修正）。D-04 r19 の event_id 全面整合に合わせて修正（2b の `EAP_PSEUDONYM_FALLBACK` を削除（フル認証誘導時は専用ログなし）、§7 不正な状態遷移の `EAP_INVALID_STATE` を `EAP_STATE_ERR` / `EAP_UNEXPECTED_IDENTITY` / `EAP_UNKNOWN_SUBTYPE` に修正、3a のJSONパース失敗時ログ `POLICY_PARSE_ERR` を `AUTH_POLICY_NOT_FOUND` に修正、§5 Client-Error受信時にエラーコードを記録しない旨を明記） |
| r7 | 2026-10-04 | ポリシーの `nas_id` で `"*"` を任意の NAS に一致させた Auth Server の実装修正に合わせ、3a Post-Auth Policy Check のルール評価を実装の PolicyRule 構造（`nas_id` / `allowed_ssids` / `vlan_id` / `session_timeout`）に修正（r5 で記載した `ssid` / `action` / `time_min` / `time_max` による評価は実装に存在しないため削除）。`nas_id` は `"*"` 単独で任意の NAS に一致（部分一致なし）、それ以外は完全一致であることを明記。評価結果の分岐表を「ルール一致 → Accept（VLAN・Session-Timeout 付与）／不一致 → default で判定」に整理し、Access-Accept 時の VLAN・Session-Timeout 属性を追記 |
| r8 | 2026-10-04 | SQN競合制御の実装（Lua による `sqn` の比較・置き換え、競合時のやり直し最大3回、HTTP 409）の反映（1.1 Error Response）: 409 Conflict の説明を実装に合わせて修正（3回とも競合・待機中の期限切れ等で返す、`detail` は固定文 `SQN update conflict`、Vector Gateway はそのまま中継、Auth Server は Circuit Breaker 対象外として Access-Reject、5xx にしない理由）。「Phase 1で追加されたSQN競合制御（WATCH/MULTI CAS）」の記述を、Lua による比較・置き換えに修正。接続方式01の注記の Circuit Breaker 対象外の 4xx の例に 409 を追加 |
