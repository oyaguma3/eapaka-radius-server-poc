# S-01 eapaka_test 利用ノウハウ (r1)

**版数:** r1
**作成日:** 2026-02-24
**分類:** 補足資料

---

## 1. 概要

### 1.1 ツールの目的・機能

eapaka_test は、RADIUS 経由で EAP-AKA / EAP-AKA' を実行するサーバ自動テスト向け CLI ツールである。

- **GitHub リポジトリ**: https://github.com/oyaguma3/eapaka_test
- EAP-AKA / EAP-AKA' の両方に対応
- outer/inner identity を分離して管理
- `AT_PERMANENT_ID_REQ` に即時応答（ポリシー指定可）
- SQN を永続化して連続実行時の同期を維持
- MPPE キーの presence check と一致検証に対応

### 1.2 本プロジェクトでの利用位置づけ

本プロジェクトでは以下のテストフェーズで eapaka_test を使用する:

| テストフェーズ | ドキュメント | 用途 |
|:-------------|:-----------|:-----|
| 結合テスト | T-03 結合テスト仕様書 | テストベクターモードでの EAP-AKA/AKA' 認証検証（G2〜G5, G7〜G9） |
| E2Eテスト | T-04 E2Eテスト仕様書 | 擬似E2E（実設定モード）での認証・課金フロー検証（E2E-101〜105） |

### 1.3 supplement 内ファイル構成

```
docs/supplement/eapaka_test/
├── S-01_eapaka_test利用ノウハウ_r1.md    # 本ドキュメント
├── configs/                               # 設定ファイル（5件）
│   ├── example.yaml                       # サンプル設定ファイル
│   ├── config_testvector.yaml             # テストベクターモード用（AMF=B9B9）
│   ├── config_testvector_imsi003.yaml     # テストベクターIMSI 003専用（AMF=8000）
│   ├── config_testsim.yaml                # テストSIM用
│   └── config_commercial.yaml             # 商用確認済みテストSIM用
└── testdata/
    └── cases/                             # テストケース（T-03/T-04使用分 15件）
        ├── success_aka_testvector.yaml
        ├── success_aka_prime_testvector.yaml
        ├── perm_id_req_from_pseudonym.yaml
        ├── resync_aka_testvector.yaml
        ├── resync_aka_prime_testvector.yaml
        ├── mismatch_strict_fail.yaml
        ├── reject_imsi_not_found.yaml
        ├── reject_policy_denied_ssid.yaml
        ├── reject_policy_denied_nas.yaml
        ├── reject_plmn_not_implemented.yaml
        ├── policy_default_allow_testvector.yaml
        ├── policy_default_deny_testvector.yaml
        ├── policy_nas_ssid_match_testvector.yaml
        ├── policy_wildcard_ssid_testvector.yaml
        └── policy_not_found_testvector.yaml
```

---

## 2. インストールとビルド

### 2.1 必要環境

- Go 1.25.x（1.25 以上）

### 2.2 ソース取得

```bash
git clone https://github.com/oyaguma3/eapaka_test.git
cd eapaka_test
```

### 2.3 ビルド

```bash
go build -o eapaka_test ./cmd/eapaka_test
```

### 2.4 バイナリ配置

ビルドしたバイナリはリポジトリ外の開発環境に配置する（バイナリはリポジトリに含めない）。

```bash
# 配置例
mkdir -p ~/devtools/eapaka_test
cp eapaka_test ~/devtools/eapaka_test/
```

設定ファイル・テストケースは本 supplement ディレクトリのものを参照可能（セクション5 参照）。

---

## 3. 設定ファイル（config）解説

### 3.1 YAML 構造

eapaka_test の設定ファイルは以下のセクションで構成される:

| セクション | 内容 |
|:---------|:-----|
| `radius` | RADIUSサーバー接続先（アドレス、Secret、タイムアウト、リトライ） |
| `radius_attrs` | RADIUS属性のデフォルト値（NAS-IP-Address、Called-Station-Id等） |
| `eap` | EAP関連設定（方式ミスマッチポリシー、AKA' ネットワーク名等） |
| `identity` | Identity の realm 部分 |
| `sim` | SIMパラメータ（IMSI、Ki、OPc、AMF、SQN初期値） |
| `sqn_store` | SQN永続化設定（モード、ファイルパス） |

### 3.2 config_testvector.yaml

テストベクターモード用の設定ファイル。3GPP TS 35.208 Test Set 1 のパラメータを使用する。

| パラメータ | 値 | 備考 |
|:---------|:----|:-----|
| IMSI | `001010000000000` | テストベクタートリガー対象 |
| Ki | `465B5CE8B199B49FAA5F0A2EE238A6BC` | 3GPP TS 35.208 Test Set 1 |
| OPc | `CD63CB71954A9F4E48A5994E37A02BAF` | 同上 |
| AMF | `B9B9` | テストベクター標準 |
| sqn_initial_hex | `FF9BB4D0B607` | 高初期値 |
| Secret | `TESTSECRET123` | Valkey登録クライアントSecret |

### 3.3 config_testvector_imsi003.yaml

IMSI 003 専用の設定ファイル。AMF が `8000` である点が `config_testvector.yaml` との主な違い。

| パラメータ | config_testvector.yaml | config_testvector_imsi003.yaml |
|:---------|:----------------------|:-------------------------------|
| IMSI | `001010000000000` | `001010000000003` |
| AMF | `B9B9` | `8000` |
| sqn_initial_hex | `FF9BB4D0B607` | `000000000001` |
| SQNストアパス | `/tmp/eapaka_test-sqn-testvector.json` | `/tmp/eapaka_test-sqn-testvector-imsi003.json` |

> **AMF不一致に関する重要な注意**: IMSI 003（サーバー側AMF=8000）に対して `config_testvector.yaml`（AMF=B9B9）を使用すると、eapaka_test が AUTN 検証時に AMF ミスマッチを検出し認証フローが中断する。IMSI 003 を使用するシナリオでは必ず `config_testvector_imsi003.yaml` を使用すること。

### 3.4 その他の設定ファイル

| ファイル | 用途 | IMSI | AMF |
|:--------|:-----|:-----|:----|
| `example.yaml` | サンプル設定（汎用） | `440100123456789` | `8000` |
| `config_testsim.yaml` | テストSIM用（sysmoISIM等） | `999700000165489` | `8000` |
| `config_commercial.yaml` | 商用確認済みテストSIM用 | `999700000165480` | `8000` |

### 3.5 新規 config 作成方法

既存の config をベースにコピーし、以下の項目を変更する:

1. `sim.imsi` — 対象 IMSI
2. `sim.ki` / `sim.opc` — SIM パラメータ（Valkey 登録値と一致させること）
3. `sim.amf` — サーバー側 AMF と一致させること（セクション7 参照）
4. `sim.sqn_initial_hex` — SQN 初期値
5. `sqn_store.path` — SQN ストアファイルパス（IMSI ごとに分離推奨）
6. `radius.secret` — RADIUS 共有秘密（Valkey のクライアント登録 Secret と一致させること）
7. `eap.aka_prime.net_name` / `identity.realm` — PLMN に応じたネットワーク名

---

## 4. テストケースファイル解説

### 4.1 YAML フォーマット

テストケースファイルは以下のフィールドで構成される:

| フィールド | 必須 | 内容 |
|:---------|:----:|:-----|
| `version` | ○ | フォーマットバージョン（現在 `1`） |
| `name` | ○ | テストケース名 |
| `identity` | △ | EAP outer identity（指定時は config の IMSI をオーバーライド） |
| `eap` | △ | EAP 設定のオーバーライド（AKA' ネットワーク名等） |
| `radius` | △ | RADIUS 属性のオーバーライド（Called-Station-Id、NAS-Identifier等） |
| `sqn` | △ | SQN 制御（`reset: true` でリセット、`persist: true` で永続化） |
| `expect` | ○ | 期待結果（`result: accept` or `reject`、MPPE チェック等） |
| `trace` | △ | トレース設定（`level: verbose` で詳細出力） |

### 4.2 T-03 シナリオ ID との対応表

| テストケースファイル | T-03 シナリオ ID | カテゴリ | 期待結果 |
|:------------------|:---------------|:--------|:--------|
| `success_aka_testvector.yaml` | INT-AUTH-AKA-001, INT-006-01, INT-VALKEY-002, INT-FLOW-001 | 正常系 | Accept |
| `success_aka_prime_testvector.yaml` | INT-AUTH-AKA-002, INT-006-02 | 正常系 | Accept |
| `perm_id_req_from_pseudonym.yaml` | INT-AUTH-AKA-003 | 正常系 | Accept |
| `resync_aka_testvector.yaml` | INT-003-01, INT-006-03 | SQN再同期 | Accept |
| `resync_aka_prime_testvector.yaml` | INT-003-02 | SQN再同期 | Accept |
| `mismatch_strict_fail.yaml` | INT-AUTH-AKA-004 | 異常系 | Reject |
| `reject_imsi_not_found.yaml` | INT-AUTH-AKA-006 | 異常系 | Reject |
| `reject_policy_denied_ssid.yaml` | INT-AUTH-AKA-007, INT-POLICY-004 | ポリシー | Reject |
| `reject_policy_denied_nas.yaml` | INT-POLICY-005 | ポリシー | Reject |
| `reject_plmn_not_implemented.yaml` | INT-GW-PLMN-010 | PLMN | Reject |
| `policy_default_allow_testvector.yaml` | INT-POLICY-001 | ポリシー | Accept |
| `policy_default_deny_testvector.yaml` | INT-POLICY-002 | ポリシー | Reject |
| `policy_nas_ssid_match_testvector.yaml` | INT-POLICY-003 | ポリシー | Accept |
| `policy_wildcard_ssid_testvector.yaml` | INT-POLICY-006 | ポリシー | Accept |
| `policy_not_found_testvector.yaml` | INT-POLICY-007 | ポリシー | Reject |

### 4.3 カテゴリ別解説

#### 正常系（3件）

EAP-AKA / AKA' 認証の基本フローを検証する。`identity` フィールドで EAP outer identity を指定し、先頭の数字で方式を区別する:
- `0...`: EAP-AKA
- `6...`: EAP-AKA'
- `2...`: Pseudonym（Permanent ID 要求テスト）

#### 異常系（2件）

認証失敗パターンを検証する:
- `mismatch_strict_fail.yaml`: `method_mismatch_policy: "strict"` でEAP方式ミスマッチ時にRejectを期待
- `reject_imsi_not_found.yaml`: Valkey 未登録 IMSI での認証試行

#### ポリシー（7件）

認可ポリシー評価の各パターンを検証する:
- default action（allow/deny）
- NAS-Identifier / SSID のマッチング
- ワイルドカード SSID（`["*"]`）
- ポリシー未登録

> **注意**: IMSI 003 を使用するポリシーテストケース（`reject_policy_denied_ssid.yaml`、`reject_policy_denied_nas.yaml`、`policy_nas_ssid_match_testvector.yaml`）では `config_testvector_imsi003.yaml` を使用すること。

#### SQN 再同期（2件）

`sqn.reset: true` でクライアント側 SQN をリセットし、SQN 不整合時の再同期フローを検証する。

#### PLMN ルーティング（1件）

未実装バックエンド PLMN へのルーティングで Reject が返却されることを検証する。

---

## 5. 実行方法

### 5.1 基本コマンド

```bash
./eapaka_test -c <config_file> run <test_case_file>
```

### 5.2 終了コード

| コード | 意味 |
|:-----:|:-----|
| 0 | PASS（期待結果と一致） |
| 1 | FAIL（期待結果不一致） |
| 2 | ERROR（設定不備、通信エラー、パース不能など） |

### 5.3 supplement 配下のファイルを使う場合のパス設定例

```bash
# プロジェクトルートからの相対パス
PROJ_ROOT="/path/to/eapaka-radius-server-poc"
CONFIG="$PROJ_ROOT/docs/supplement/eapaka_test/configs/config_testvector.yaml"
CASES="$PROJ_ROOT/docs/supplement/eapaka_test/testdata/cases"

# バイナリは開発環境から（リポジトリに含めない）
EAPAKA_TEST="/path/to/devtools/eapaka_test"

# 実行例
$EAPAKA_TEST/eapaka_test -c $CONFIG run $CASES/success_aka_testvector.yaml
```

### 5.4 環境変数パターン（T-03 結合テスト）

```bash
# テストベクターモード（T-03）
EAPAKA_TEST="/path/to/devtools/eapaka_test"
CONFIG="$PROJ_ROOT/docs/supplement/eapaka_test/configs/config_testvector.yaml"
CONFIG_IMSI003="$PROJ_ROOT/docs/supplement/eapaka_test/configs/config_testvector_imsi003.yaml"
CASES="$PROJ_ROOT/docs/supplement/eapaka_test/testdata/cases"

# EAP-AKA 正常認証
$EAPAKA_TEST/eapaka_test -c $CONFIG run $CASES/success_aka_testvector.yaml

# IMSI 003 使用シナリオ（AMF=8000）
$EAPAKA_TEST/eapaka_test -c $CONFIG_IMSI003 run $CASES/reject_policy_denied_ssid.yaml
```

---

## 6. SQN 管理

### 6.1 SQN 永続化の仕組み

eapaka_test は SQN（Sequence Number）をファイルに永続化し、連続実行時の SQN 同期を維持する。

- **SQN ストアファイル**: config の `sqn_store.path` で指定（例: `/tmp/eapaka_test-sqn-testvector.json`）
- **ストアキー**: `sim.imsi` の値をキーに SQN を管理
- **初期値**: SQN ストアに未登録の IMSI に対しては `sim.sqn_initial_hex` を初期値として使用

### 6.2 クライアント SQN vs サーバー SQN

| 項目 | クライアント（eapaka_test） | サーバー（Valkey） |
|:-----|:------------------------|:-----------------|
| 保存場所 | SQN ストアファイル | `sub:{IMSI}` の `sqn` フィールド |
| 初期値 | `sqn_initial_hex` | Valkey 登録値 |
| インクリメント | 認証成功ごとに +0x20（テストベクターモード） | Vector API の設定に依存 |

> **重要**: クライアント SQN とサーバー SQN が大きく乖離すると、Vector API の「SQN difference exceeds allowed range」チェックに抵触し認証が失敗する。

### 6.3 リセット手順

テストを繰り返すと SQN が蓄積するため、テスト開始前にリセットを推奨する。

```bash
# 1. eapaka_test の SQN ストアファイル削除
rm -f /tmp/eapaka_test-sqn-testvector.json
rm -f /tmp/eapaka_test-sqn-testvector-imsi003.json

# 2. Valkey 側 SQN リセット
# プライマリ IMSI（IMSI 000）: config の IMSI と一致 → 低値でOK
docker compose exec valkey redis-cli -a "$VALKEY_PASSWORD" HSET sub:001010000000000 sqn "000000000001"

# identity オーバーライド IMSI: sqn_initial_hex に合わせる
docker compose exec valkey redis-cli -a "$VALKEY_PASSWORD" HSET sub:001010000000001 sqn "FF9BB4D0B607"
docker compose exec valkey redis-cli -a "$VALKEY_PASSWORD" HSET sub:001010000000002 sqn "FF9BB4D0B607"
docker compose exec valkey redis-cli -a "$VALKEY_PASSWORD" HSET sub:001010000000006 sqn "FF9BB4D0B607"
docker compose exec valkey redis-cli -a "$VALKEY_PASSWORD" HSET sub:001010000000007 sqn "FF9BB4D0B607"

# IMSI 003: config_testvector_imsi003.yaml の sqn_initial_hex に合わせる
docker compose exec valkey redis-cli -a "$VALKEY_PASSWORD" HSET sub:001010000000003 sqn "000000000001"
```

### 6.4 identity オーバーライド時の注意

テストケースの `identity` フィールドで config の IMSI と異なる IMSI を指定した場合:

1. eapaka_test は SQN ストアにその IMSI が未登録であれば `sqn_initial_hex` を初期 SQN として使用する
2. したがって、**サーバー側 SQN も `sqn_initial_hex` と同等の値に設定する必要がある**
3. 低値にリセットすると、クライアント側 SQN との差が大きくなり「SQN difference exceeds allowed range」エラーが発生する

例:
- `config_testvector.yaml` の `sqn_initial_hex` = `FF9BB4D0B607`
- `policy_wildcard_ssid_testvector.yaml` は IMSI `001010000000006` を identity で指定
- → サーバー側 `sub:001010000000006` の SQN を `FF9BB4D0B607` に設定する必要がある

---

## 7. config と IMSI の使い分け

### 7.1 AMF と config 設定の対応

本プロジェクトでは、テストベクターモードで2つの AMF 値を使用する:

| AMF | 対象 IMSI | config ファイル | 用途 |
|:----|:---------|:--------------|:-----|
| `B9B9` | IMSI 000, 001, 002, 006, 007 | `config_testvector.yaml` | テストベクター標準（大半のシナリオ） |
| `8000` | IMSI 003 | `config_testvector_imsi003.yaml` | NAS/SSID ルール付きポリシーテスト |

### 7.2 identity オーバーライドの注意点

eapaka_test のテストケースで `identity` を指定すると、config の `sim.imsi` とは異なる IMSI で認証を行う。このとき:

1. **AMF**: config の `sim.amf` が使用される → IMSI 側の AMF と不一致なら認証中断
2. **SQN**: SQN ストアに未登録なら `sqn_initial_hex` が使用される → サーバー側との同期が必要
3. **Ki/OPc**: config の値が使用される（テストベクターモードではサーバー側が固定値を返すため影響なし）

したがって、異なる AMF の IMSI を使用する場合は IMSI ごとに個別の config ファイルを用意する必要がある。

---

## 8. トラブルシューティング

### 8.1 認証失敗（Access-Reject）

| 原因 | 確認方法 | 対処 |
|:-----|:--------|:-----|
| `TEST_VECTOR_ENABLED` 設定不整合 | `docker compose exec vector-api env \| grep TEST_VECTOR` | `.env` の設定を確認し `docker compose up -d` |
| 加入者データ未登録 | `redis-cli HGETALL sub:{IMSI}` | Valkey にデータ投入 |
| Ki/OPc 不一致 | eapaka_test のパラメータと Valkey 登録値を比較 | 値を統一 |
| ポリシー設定 | `redis-cli HGETALL policy:{IMSI}` | ポリシー修正 |

### 8.2 AMF ミスマッチ

| 症状 | 原因 | 対処 |
|:-----|:-----|:-----|
| eapaka_test が `amf mismatch` で中断 | config の AMF とサーバー側 AMF が不一致 | 対象 IMSI の AMF に合った config を使用（セクション7参照） |

### 8.3 SQN 不整合

| 症状 | 原因 | 対処 |
|:-----|:-----|:-----|
| `SQN difference exceeds allowed range` | クライアント/サーバー間の SQN 乖離 | SQN リセット（セクション6.3） |
| Resync 後も認証失敗 | SQN ストアファイルの IMSI 間干渉 | SQN ストアファイル削除 |

### 8.4 Docker イメージ再ビルドの必要性

外部パッケージ（go-eapaka 等）を更新した場合、`go.mod` の変更だけでは Docker コンテナに反映されない。

```bash
# 依存ライブラリ更新後は必ず再ビルド
docker compose -f deployments/docker-compose.yml build auth-server
docker compose -f deployments/docker-compose.yml up -d auth-server
docker compose -f deployments/docker-compose.yml ps
```

古いイメージのまま実行すると、サーバーとテストツール間で暗号処理（PRF、鍵導出等）の不一致が発生し、AT_MAC 検証失敗や Authentication-Reject となる。

### 8.5 radclient 属性入力エラー

echo パイプ（`echo -e '...' | radclient`）ではシェル環境によって属性名解析エラーが発生する場合がある。ファイルリダイレクト方式を推奨する。

```bash
# 推奨方式
printf 'Message-Authenticator = 0x00\n' > /tmp/status.attrs
radclient -x -r 1 -t 3 127.0.0.1:1812 status TESTSECRET123 < /tmp/status.attrs
```

---

## 9. プロジェクト固有の運用知見

### 9.1 T-03 結合テストでの利用パターン

- **テストベクターモード**（`TEST_VECTOR_ENABLED=true`）で実行
- 大半のシナリオは `config_testvector.yaml` を使用、IMSI 003 のみ `config_testvector_imsi003.yaml` を使用
- テスト実行前にテストデータ（加入者・クライアント・ポリシー）を Valkey に投入する必要がある
- G1〜G7 は AI 自動実行可能、G8（PLMN）は環境変更を伴い、G9（障害系）は人間介在が必要

### 9.2 T-04 擬似 E2E テストでの利用パターン

- **実設定モード**（`TEST_VECTOR_ENABLED=false`）で実行
- Vector API が実際の Milenage 計算を実行し、Valkey の SIM パラメータを参照
- eapaka_test 設定の AMF は Valkey 登録値と一致させること
- テストケースは T-03 と同一ファイルを使用可能

### 9.3 テスト反復時の SQN リセット運用

テストを繰り返すと SQN がインクリメントされ蓄積する。以下のタイミングでリセットを推奨:

1. **テストセッション開始前**: セクション6.3 の手順で全 IMSI の SQN をリセット
2. **テストグループ切替時**: 異なる IMSI を使用するグループへの切替時に SQN ストアファイルを削除
3. **エラー発生時**: 「SQN difference exceeds allowed range」エラーが出たら即座にリセット

> **SQN increment step**: テストベクターモードでは SQN increment step = 0x20（32）。テストを10回繰り返すだけで SQN が 320 進むため、繰り返しテスト時は定期的なリセットが有効。

---

## 改版履歴

| 版数 | 日付 | 内容 |
|:----:|:-----|:-----|
| r1 | 2026-02-24 | 初版作成 |
