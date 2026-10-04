# EAP-AKA RADIUS PoC環境 ドキュメント一覧 (r41)

**作成日:** 2025-12-30
**最終更新:** 2026-10-04
**ステータス:** 運用準備フェーズ

---

## 1. 設計ドキュメント

### 1.1 作成済み

| No. | ドキュメント名 | 版数 | 最終更新 | 内容 |
|-----|---------------|------|---------|------|
| D-01 | ミニPC版 EAP-AKA RADIUS PoC環境 設計仕様書 | r12 | 2026-10-04 | システム概要、アーキテクチャ、ノード構成、パッケージマップ、Vector Gateway追加、Valkeyバージョン9.0統一、Fluent Bit統一、環境変数名統一、実装コードとの不整合20件修正（データモデル注記・Valkeyキースキーマ・パッケージマップ・docker-compose.yml完全同期・healthcheck・テストベクターモード）、外部aka-only-server（接続方式01）を構成図・環境変数に追加、テストベクターモードの説明を実装に整合（固定 Ki/OPc/AMF で計算、加入者登録必須）、compose の LOG_LEVEL（auth-server / vector-gateway / vector-api）を実ファイルに同期 |
| D-02 | Valkeyデータ設計仕様書 | r16 | 2026-10-04 | データ構造、キー設計、TTL、Go構造体、CK/IK非保存方針、UUID仕様明記、Acct重複検出キャッシュ、stageフィールド値明記、SQN競合制御（WATCH/MULTI CAS）、idx:userクリーンアップ方針、PolicyRule構造を実装コードに整合（NasID/AllowedSSIDs/VlanID/SessionTimeout）、実装との突き合わせによる修正（SQN競合制御は設計のみで現行未実装、セッション/インデックス/重複検出/EAPコンテキストの実態）、Interimのセッション不在時に作成しない・重複検出のInterim判定（no_start_received / interim_after_stop）を実装に整合、テストベクターモードでも加入者登録必須・既定SQNフォールバック廃止（`sqn` だけの Hash を作らない）を実装に整合、nas_idの`*`単独で任意のNASに一致（部分一致なし、それ以外は完全一致）を実装に整合、Trace ID は初回に生成し以降は State 属性の UUID を引き継ぐ旨を実装に整合 |
| D-03 | Vector-APIインターフェース定義書およびEAP-AKAステートマシン設計書 | r7 | 2026-10-04 | API仕様、8状態定義、Policy評価Post-Authのみ、接続先をVector Gatewayに変更、接続方式01経由時の403応答、Post-Authのルール評価を実装のPolicyRule（nas_id/allowed_ssids、nas_idの`*`は任意のNASに一致）に整合 |
| D-04 | ログ仕様設計書 | r24 | 2026-10-04 | ログフォーマット、event_id定義、Vector Gateway対応、EAP_INVALID_STATE追加、IMSIマスキング（4コンポーネント対応、Admin TUI除外明記）、Acct Server PKT_RECV追加、SQN_CONFLICT_ERR追加、lnavフォーマット全面改訂（bunyan競合回避・timestamp-format削除・file-pattern・sample追加）、ACCT_ON/ACCT_OFF追加、ヘルスチェックログ分離記述追加、BACKEND_EXTERNAL_CALL/BACKEND_EXTERNAL_ERR実装済み化・causeフィールド追加、IMSIマスク漏れ修正の反映（user_name等のマスク、User-Nameマスク規則追加）・lnavフォーマットのvalue整理（code/subtype/eap_type/acct_status_type/session_time追加、retry_count/downtime_ms削除）、ACCT_SEQUENCE_ERRのInterim側msg変更・interim_after_stop追加、ACCT_SESSION_NOT_FOUNDをStart/Interimに拡大、テストベクターモードのTEST_SQN_FALLBACK/TEST_SQN_PARSE_ERR/TEST_SQN_PERSIST_ERRを削除（エラーログは通常モードと同じ）、auth-server の trace_id を認証単位で引き継ぎ（2回目以降の PKT_RECV 等も同一 trace_id）、EAP_ENGINE_ERR 削除、LOG_LEVEL（§4.6新設、DEBUG で vector api success）、Vector API のログ整理（SQN_RESYNC に trace_id・imsi、SQN_RESYNC_DELTA_ERR とテストモードの CALC_OK を1行化・CALC_OK に test_mode、ProblemError 経路のログに error 属性、Vector Gateway / Vector API も ParseLevel で WARNING を WARN） |
| D-05 | Admin TUI詳細設計書【前半】 | r9 | 2026-02-23 | 画面設計、バリデーション、インポート/エクスポート、IMSI表示方針（常に生値）、全マスタデータHash形式統一、実装スクリーンショットとのASCIIレイアウト整合性修正（12画面） |
| D-06 | エラーハンドリング詳細設計書 | r12 | 2026-10-04 | 異常系処理、タイムアウト、リトライ、Circuit Breaker、Vector Gateway追加、SQN競合エラー（409）追加、Vector Gateway経由フロー明記、接続方式01（aka-only-server）のエラー変換、EAP Identity系ログのuser_nameマスク反映、Interimのセッション不在・順序異常の扱いを実装に整合、テストベクターモードのエラー処理を通常モードと同一化、EAPエンジンが error を返さない実装に合わせ EAP_ENGINE_ERR を削除、Vector API の Valkey リトライなし（未使用の GetWithRetry 削除）・SQN_RESYNC_DELTA_ERR の1行化・定義済みエラーのログに error 属性 |
| D-07 | Admin TUI詳細設計書【後半】 | r8 | 2026-10-04 | モニタリング画面、ヘルプダイアログ、IMSI記録方針（監査ログに生値）、idx:userクリーンアップ処理、実装スクリーンショットとのASCIIレイアウト整合性修正（5画面）、event_idを実装に整合 |
| D-08 | インフラ設定・運用設計書 | r16 | 2026-10-04 | Docker Compose設定、Valkey設定、Fluent Bit設定（fluent/fluent-bit:4.2、YAML形式、rewrite_tagによるヘルスチェックログ分離、キャッチオール廃止による重複出力解消）、UFW設定、運用手順、IMSIマスキング環境変数（4コンポーネント限定）、Valkeyバージョン9.0、ヘルスチェック方針（curl -fsS）、テストベクターモード環境変数、B-02スコープ修正（B-01境界整合）、aka-only-server接続（compose環境変数・証明書マウント・共有ネットワークaka-av用オーバーレイ）、テストベクターモードの.env.example説明を実装に整合（加入者登録必須）、compose / .env.example の LOG_LEVEL（auth-server / vector-gateway / vector-api）を実ファイルに同期 |
| D-09 | Auth Server詳細設計書 | r14 | 2026-10-04 | パッケージ構成、RADIUS受信処理、EAP制御フロー、Vector Gateway連携、セッション管理、IMSIマスキング、UUID仕様明記、互換性エイリアス削除、ベースイメージ方針、Vector関連event_id（VECTOR_IMSI_NOT_FOUND等）を実装に整合、user_nameのマスキング（MaskUserName）追加、Acct ServerのInterim時のセッション不在の扱いを修正、認可ポリシー評価（セクション8）を実装のPolicyRule構造とnas_idの`*`（任意のNASに一致）に整合、Trace ID の決定（State 属性の UUID 引き継ぎ）、EAPProcessor の Process(ctx, req) *Result・EAP_ENGINE_ERR 削除、LOG_LEVEL（pkg/logging.ParseLevel）対応 |
| D-10 | Acct Server詳細設計書 | r9 | 2026-10-04 | パッケージ構成、Accounting処理フロー、セッション更新ロジック、重複検出、IMSIマスキング、Status-Server対応、ベースイメージ方針、Accounting-On/Off対応（ProcessOn/ProcessOff、NAS-Identifier処理）、event_idを実装に整合、IMSI抽出不可時のUser-Nameをマスクして出力、Interimのシーケンス判定（CheckInterim、interim_after_stop）とセッション存在確認 |
| D-11 | Vector API詳細設計書 | r9 | 2026-10-04 | パッケージ構成、HTTPサーバー設定、Milenage計算、SQN管理、SQN競合制御（WATCH/MULTI CAS）、エラーハンドリング、ベースイメージ方針、テストベクターモード本番無効化注記、event_idを実装に整合、SQN競合制御（CAS）は設計済み・現行未実装と明記、テストベクターモードを実装に整合（Ki/OPc/AMFのみ固定値、加入者登録必須、既定SQNフォールバックとTEST_SQN_*ログ廃止）、ログ整理（ContextWithTraceID で SQN_RESYNC に trace_id・imsi、ユースケース層のデルタ超過・test vector generated ログ削除、CALC_OK に test_mode、ProblemError 経路の error 属性、IsTestMode 追加）、未使用の GetWithRetry / ErrInvalidIMSI を削除 |
| D-12 | Vector Gateway詳細設計書 | r7 | 2026-10-04 | 外部API連携設計、PLMNルーティング、接続方式管理、トレーサビリティ、IMSIマスキング、ベースイメージ方針（debian:bookworm-slim）、接続方式01（aka-only-server、mTLS/平文HTTP、GenerateAv変換）実装、compose抜粋・環境変数にLOG_LEVELを追加、LOG_LEVEL の変換を pkg/logging.ParseLevel に統一 |

### 1.2 未作成

なし（設計ドキュメント全12件完了）

### 1.3 Go実装ノードと設計ドキュメントの対応

| 実装ノード | ディレクトリ | 関連設計ドキュメント |
|-----------|------------|-------------------|
| **Auth Server** | `apps/auth-server` | D-03（ステートマシン）, D-06（エラー処理）, **D-09**（詳細設計） |
| **Acct Server** | `apps/acct-server` | D-02（データ構造）, D-06（エラー処理）, **D-10**（詳細設計） |
| **Vector API** | `apps/vector-api` | D-02（データ構造）, D-03（API仕様）, **D-11**（詳細設計） |
| **Vector Gateway** | `apps/vector-gateway` | **D-12**（詳細設計書） |
| **Admin TUI** | `apps/admin-tui` | D-02（データ構造）, **D-05**（前半）, **D-07**（後半） |

---

## 2. 開発ドキュメント

### 2.1 作成済み

| No. | ドキュメント名 | 版数 | 最終更新 | 内容 |
|-----|---------------|------|---------|------|
| E-01 | 開発環境セットアップガイド | r5 | 2026-10-04 | Go環境構築、Go Workspace設定、依存パッケージ、ローカル開発手順、デバッグ方法、環境変数名統一（RADIUS_SECRET）、Makefileセクション追加、golangci-lint/CIセクション追加、テストベクターモード環境変数追加、event_idを実装に整合、LOG_LEVEL（開発用.env・実行例・環境変数一覧）追加 |
| E-02 | コーディング規約（簡易版） | r5 | 2026-10-04 | 命名規則、パッケージ構成、エラーハンドリングパターン、構造体タグ（jsonのみ）、ログ出力規約、IMSIマスキングD-04 r17準拠、golangci-lint導入済み反映、ログ出力例のevent_idを実装に整合、Handler層例のEAP_ENGINE_ERRを削除、ログレベルはLOG_LEVEL（pkg/logging.ParseLevel。Auth Server / Vector Gateway / Vector API 共通）で設定 |
| E-03 | 共通ライブラリ(pkg)設計書 | r7 | 2026-10-04 | pkg配置方針、apperr/valkey/logging/model/httputil各パッケージ設計、IMSIマスキングD-04準拠、MaskUserName / Masker.UserName追加、PolicyRuleを実装（NasID/AllowedSSIDs/VlanID/SessionTimeout）に整合、ParseLevel（LOG_LEVEL→slog.Level変換）追加、ParseLevel の利用箇所に Vector Gateway / Vector API を追加 |

### 2.2 未作成

なし（開発ドキュメント全3件完了）

---

## 3. テストドキュメント

### 3.1 作成済み

| No. | ドキュメント名 | 版数 | 最終更新 | 内容 |
|-----|---------------|------|---------|------|
| T-01 | テスト戦略書 | r4 | 2026-10-04 | テストレベル定義、テスト範囲、テスト環境、モック戦略、テストデータ戦略、品質ゲート、テストベクターモード運用注記、.env例の環境変数名（RADIUS_SECRET）を実装に整合、テストベクターモードの動作を実装に整合（加入者登録必須） |
| T-02 | 単体テスト仕様書 | r8 | 2026-10-04 | コンポーネント別テストケース（全1,245件）、モック戦略、テストデータ設計、Vector Gateway接続方式01のテストケース追加、IMSIマスク漏れ修正のテストケース追加、acct-server Interimシーケンス判定修正のテストケース反映、vector-apiテストベクターモードの加入者登録必須化のテストケース反映、auth-serverポリシーnas_idワイルドカード（`*`）のテストケース追加、auth-server trace_id引き継ぎ・LOG_LEVEL対応のテストケース反映（ParseLevel・TraceIDFromState等を追加、EngineErrorを欠番）、vector-apiログ整理のテストケース反映（LogAttributes・LogsTraceIDAndMaskedIMSI等を追加、ErrInvalidIMSIの検証を欠番） |
| T-03 | 結合テスト仕様書 | r11 | 2026-10-04 | コンポーネント間連携テスト、シナリオテスト、テストベクターモード検証、Valkeyデータ整合性検証、Secret体系明確化、SQN再同期手順改訂、IMSI 003専用config追加、障害系PASS条件修正、identityオーバーライドIMSIのSQNリセット運用補足、Dockerイメージ再ビルド注意事項追加、eapaka_testパス参照をsupplement配下に一般化、INT-ACCT-ON-017/INT-ACCT-OFF-018追加、aka-only-server結合シナリオ追加、INT-GW-PLMN-010の未実装IDを02に変更、テストベクターモードでも加入者登録必須（テストIMSI帯でも未登録は404）・事前準備での登録を明記、INT-006 の期待結果に Auth Server の全パケットの PKT_RECV も同一 trace_id であることを追記、INT-006-03 に Vector API の SQN_RESYNC も同一 trace_id であることを追記 |
| T-04 | E2Eテスト仕様書 | r7 | 2026-10-04 | 実機テスト（SIM/AP）3件、擬似E2E（eapaka_test）5件、実機異常系3件の計11シナリオ、SQN管理注意事項追加、Valkey再起動後データ残存確認追加、eapaka_testパス参照をsupplement配下に一般化、aka-only-server接続E2Eシナリオと実施結果（2026-10-04）追加、テストベクターモードのT-03との差分（加入者登録必須）を実装に整合、認可ポリシーのnas_id `*`（任意のNASに一致）を反映 |

### 3.2 未作成

なし（テストドキュメント全4件完了）

---

## 4. 構築・デプロイドキュメント

### 4.1 作成済み

| No. | ドキュメント名 | 版数 | 最終更新 | 内容 |
|-----|---------------|------|---------|------|
| B-01 | ホストOS構築手順書 | r3 | 2026-02-23 | Ubuntu Serverインストール、初期設定、セキュリティ設定、Docker導入、systemdサービス登録 |
| B-02 | アプリケーションデプロイ手順書 | r12 | 2026-10-04 | リポジトリクローン、.env作成、Docker Compose起動、Admin TUI配置、logrotate設定、バックアップスクリプト配置、lnavフォーマット配置・全面改訂、lnavカスタムフォーマット適用失敗トラブルシューティング、aka-only-server接続手順、lnavフォーマットのvalue整理、オプション項目にLOG_LEVEL追加 |

### 4.2 未作成

なし（構築・デプロイドキュメント全2件完了）

---

## 5. 運用ドキュメント

実装完了後に作成予定。この段階では必須前提・方針と概要を定義し、他ドキュメント作成の指針とする。

### 5.1 運用ガイド類

| No. | ドキュメント名 | 作成時期 | 必須前提・方針 | 概要 |
|-----|---------------|---------|---------------|------|
| O-01 | 操作ガイド（user-guide.md） | **完了 (r3)** | D-05, D-07の完成後 | 加入者登録、ポリシー設定、セッション監視の操作手順。ASCIIレイアウト付き。PLMNによる加入者登録先（Admin TUI / aka-only-server）の区別。ポリシーCSVのnas_id `*`（任意のNASに一致）の説明。 |
| O-02 | ポリシー設定ガイド（policy-config-guide.md） | **完了 (r2)** | D-02の認可ポリシー設計確定後 | NAS-ID/SSID設定例、VLAN割り当て例、トラブルシューティング、NAS ID `*`（任意のNASに一致）の設定例と評価順の注意 |
| O-03 | 障害対応手順書 | **完了 (r5)** | D-06, D-08の完成後 | 障害検知方法、切り分け手順、復旧手順、エスカレーションフロー、aka-only-server接続（接続方式01）の切り分け、Acct Serverの順序異常（reason別）・Interimのセッション不在の切り分け、EAP_CTX_NOT_FOUND の trace_id による切り分け手順、LOG_LEVEL=DEBUG による Vector 呼び出し時間の確認、Vector API の VALKEY_CONN_ERR / CALC_ERR の error 属性で原因を確認 |

### 5.2 保守ドキュメント類

| No. | ドキュメント名 | 作成時期 | 必須前提・方針 | 概要 |
|-----|---------------|---------|---------------|------|
| O-04 | バックアップ・リストア手順書 | **完了 (r1)** | D-08でバックアップ方針定義後 | Valkeyデータのバックアップ/リストア（自動・手動）、リストア手順（6ステップ）、設定ファイルのバックアップと復元、トラブルシューティング、運用チェックリスト |
| O-05 | ログ解析ガイド | **完了 (r12)** | D-04の完成後 | lnavの使い方、頻出クエリ集、障害調査パターン。event_idを実装に整合、lnavクエリを実動作に整合（aka_radius_logテーブル）、user_nameマスク反映・SQLカラム（code/subtype/eap_type/acct_status_type/session_time）追加、ACCT_SEQUENCE_ERR（interim_after_stop）・ACCT_SESSION_NOT_FOUND（Interim）の反映、テストベクターモードのTEST_SQN_*を削除、2回目以降の PKT_RECV も同一 trace_id で追跡できる旨に修正、EAP_ENGINE_ERR 削除、LOG_LEVEL=DEBUG の vector api success、Vector API の SQN_RESYNC を trace_id で追跡・SQN_RESYNC_DELTA_ERR の error から SQN 値を取り出すクエリ・CALC_OK の test_mode で抽出するクエリ |

---

## 6. 補足資料

### 6.1 作成済み

| No. | ドキュメント名 | 版数 | 最終更新 | 内容 |
|-----|---------------|------|---------|------|
| S-01 | eapaka_test利用ノウハウ | r3 | 2026-10-04 | eapaka_testの設定・テストケース解説、SQN管理、configとIMSIの使い分け、トラブルシューティング、プロジェクト固有の運用知見。設定ファイル5件・テストケース15件をsupplement配下に格納、aka-only-server相手の再同期確認時のSQN（IND）注意、テストベクターモードでも加入者登録必須（未登録IMSIは404）を反映 |

> **格納場所**: `docs/supplement/eapaka_test/` 配下

---

## 7. ドキュメント依存関係図

```
[設計ドキュメント] ─────────────────────────────────────────────────────┐
    │                                                                   │
    ├─ D-01: ミニPC版設計仕様書 (r12) ✓                                  │
    ├─ D-02: Valkeyデータ設計仕様書 (r16) ✓                              │
    ├─ D-03: Vector-API/ステートマシン設計書 (r7) ✓                     │
    ├─ D-04: ログ仕様設計書 (r24) ✓                                     │
    ├─ D-05: Admin TUI詳細設計書【前半】(r9) ✓                          │
    ├─ D-06: エラーハンドリング詳細設計書 (r12) ✓                       │
    ├─ D-07: Admin TUI詳細設計書【後半】(r8) ✓                          │
    ├─ D-08: インフラ設定・運用設計書 (r16) ✓                            │
    ├─ D-09: Auth Server詳細設計書 (r14) ✓                               │
    ├─ D-10: Acct Server詳細設計書 (r9) ✓                               │
    ├─ D-11: Vector API詳細設計書 (r9) ✓                                │
    └─ D-12: Vector Gateway詳細設計書 (r7) ✓                            │
                    │                                                   │
                    ▼                                                   │
[開発ドキュメント] ─────────────────────────────────────────────────────┤
    │                                                                   │
    ├─ E-01: 開発環境セットアップガイド (r5) ✓                          │
    ├─ E-02: コーディング規約・簡易版 (r5) ✓                            │
    └─ E-03: 共通ライブラリ設計書 (r7) ✓                                │
                    │                                                   │
                    ▼                                                   │
[テストドキュメント] ───────────────────────────────────────────────────┤
    │                                                                   │
    ├─ T-01: テスト戦略書 (r4) ✓                                        │
    ├─ T-02: 単体テスト仕様書 (r8) ✓                                     │
    ├─ T-03: 結合テスト仕様書 (r11) ✓                                   │
    └─ T-04: E2Eテスト仕様書 (r7) ✓                                    │
                    │                                                   │
                    ▼                                                   │
[構築・デプロイドキュメント] ───────────────────────────────────────────┤
    │                                                                   │
    ├─ B-01: ホストOS構築手順書 (r3) ✓                                   │
    └─ B-02: アプリケーションデプロイ手順書 (r12) ✓                        │
                    │                                                   │
                    ▼                                                   │
[運用ドキュメント] ◄────────────────────────────────────────────────────┘
    │
    ├─ O-01: 操作ガイド (r3) ✓
    ├─ O-02: ポリシー設定ガイド (r2)✓
    ├─ O-03: 障害対応手順書 (r5) ✓
    ├─ O-04: バックアップ・リストア手順書 (r1) ✓
    └─ O-05: ログ解析ガイド (r12)✓
```

---

## 8. 推奨作成順序

### フェーズ1: 設計完了

| 順序 | ドキュメントID | ドキュメント名 | ステータス |
|-----|---------------|---------------|-----------|
| 1 | D-07 | Admin TUI詳細設計書【後半】 | **完了 (r1)** |
| 2 | D-12 | Vector Gateway詳細設計書 | **完了 (r7)** |
| 3 | D-08 | インフラ設定・運用設計書 | **完了 (r16)** |
| 4 | D-09 | Auth Server詳細設計書 | **完了 (r14)** |
| 5 | D-11 | Vector API詳細設計書 | **完了 (r9)** |
| 6 | D-10 | Acct Server詳細設計書 | **完了 (r9)** |

### フェーズ2: 開発準備

| 順序 | ドキュメントID | ドキュメント名 | ステータス |
|-----|---------------|---------------|-----------|
| 7 | E-01 | 開発環境セットアップガイド | **完了 (r5)** |
| 8 | E-02 | コーディング規約（簡易版） | **完了 (r5)** |
| 9 | E-03 | 共通ライブラリ(pkg)設計書 | **完了 (r7)** |
| 10 | T-01 | テスト戦略書 | **完了 (r4)** |

### フェーズ3: 開発・テスト

| 順序 | ドキュメントID | ドキュメント名 | ステータス |
|-----|---------------|---------------|-----------|
| 11 | T-02 | 単体テスト仕様書 | **完了 (r8)** |
| 12 | T-03 | 結合テスト仕様書 | **完了 (r11)** |
| 13 | T-04 | E2Eテスト仕様書 | **完了 (r7)** |

### フェーズ4: 構築・デプロイ

| 順序 | ドキュメントID | ドキュメント名 | ステータス |
|-----|---------------|---------------|-----------|
| 14 | B-01 | ホストOS構築手順書 | **完了 (r3)** |
| 15 | B-02 | アプリケーションデプロイ手順書 | **完了 (r12)** |

### フェーズ5: 運用準備（実装完了後）

| 順序 | ドキュメントID | ドキュメント名 | ステータス |
|-----|---------------|---------------|-----------|
| 16 | O-01 | 操作ガイド | **完了 (r3)** |
| 17 | O-02 | ポリシー設定ガイド | **完了 (r2)** |
| 18 | O-03 | 障害対応手順書 | **完了 (r5)** |
| 19 | O-04 | バックアップ・リストア手順書 | **完了 (r1)** |
| 20 | O-05 | ログ解析ガイド | **完了 (r12)** |

---

## 9. 進捗サマリ

| カテゴリ | 総数 | 作成済み | 未作成 | 進捗率 |
|---------|------|---------|-------|-------|
| 設計ドキュメント | 12 | 12 | 0 | 100% |
| 開発ドキュメント | 3 | 3 | 0 | 100% |
| テストドキュメント | 4 | 4 | 0 | 100% |
| 構築・デプロイドキュメント | 2 | 2 | 0 | 100% |
| 運用ドキュメント | 5 | 5 | 0 | 100% |
| 補足資料 | 1 | 1 | 0 | 100% |
| **合計** | **27** | **27** | **0** | **100%** |

---


## 改訂履歴

| 版数 | 日付 | 内容 |
|------|------|------|
| r1 | 2025-12-30 | 初版作成。設計ドキュメント6件作成済み、全23件のドキュメント体系を定義。 |
| r2 | 2026-01-04 | D-07完了反映。Go実装ノード別詳細設計書（D-09〜D-11）を追加。総ドキュメント数23→26件。進捗率26%→27%。 |
| r2 | 2026-01-05 | D-12（Vector Gateway実装レベル検討書）追加。Vector Gatewayのノードマッピング追加。総ドキュメント数26→27件。進捗率27%→30%。 |
| r3 | 2026-01-14 | 各設計ドキュメントの版数更新反映: D-01(r4), D-02(r4), D-03(r3), D-04(r5), D-06(r3)。D-08（インフラ設定・運用設計書）、D-09（Auth Server詳細設計書）作成完了反映。設計ドキュメント進捗率67%→83%。全体進捗率30%→37%。 |
| r4 | 2026-01-18 | IMSIマスキング機能追加に伴う整合性更新: D-04(r6→r7), D-08(r1→r2), D-09(r2→r3), D-12(r1→r2/名称変更)。D-11(r2)を未作成→作成済みに移動。設計ドキュメント進捗率83%→92%、全体進捗率37%→41%。 |
| r5 | 2026-01-20 | 設計フェーズ完了: D-10(r1)作成完了、設計ドキュメント進捗率92%→100%。各ドキュメント版数更新: D-01(r4→r5), D-02(r4→r6), D-04(r7→r8), D-08(r2→r3), D-09(r3→r4)。D-10想定目次セクション削除。全体進捗率41%→44%。 |
| r6 | 2026-01-21 | 各ドキュメント版数更新: D-07(r1→r2), D-10(r1→r2)。D-10 Status-Server対応追加。|
| r7 | 2026-01-25 | 開発ドキュメント完了: E-01(r1), E-02(r1), E-03(r1)作成完了。開発ドキュメント進捗率0%→100%、全体進捗率44%→56%。各設計ドキュメント版数更新: D-02(r6→r7), D-04(r8→r9), D-08(r3→r4), D-09(r4→r5)。ステータスを「開発準備フェーズ」に変更。 |
| r8 | 2026-01-27 | レビューコメント対応（Phase 1-5）完了反映: T-01(r2)作成完了、テストドキュメント進捗率0%→25%、全体進捗率56%→59%。各ドキュメント版数更新: D-01(r5→r6), D-02(r7→r9), D-03(r3→r4), D-04(r9→r11), D-05(r2→r3), D-06(r3→r5), D-07(r2→r3), D-08(r4→r7), D-09(r5→r7), D-10(r2→r3), D-11(r2→r5), D-12(r2→r3), E-01(r1→r2)。主な更新内容: SQN競合制御、インフラ基盤統一、API接続設計統一、IMSIマスキング適用範囲明確化、idx:userクリーンアップ、テストベクターモード運用注記。 |
| r9 | 2026-01-27 | レビュー指摘対応（Phase 6）: D-09/D-10のDockerfileにprocps追加（ヘルスチェック整合性）、D-08にpgrep依存パッケージ明記、D-04にIDX_USER_CLEANUP event_id追加、D-01のdocker-compose.yml buildパス修正。各ドキュメント版数更新: D-01(r6→r7), D-04(r11→r12), D-08(r7→r8), D-09(r7→r8), D-10(r3→r4)。 |
| r10 | 2026-01-27 | D-01のFluent Bit設定ファイル相対パス修正に伴う版数更新（D-01 r8）。依存関係図の版数反映。 |
| r11 | 2026-02-07 | D-05版数更新（r3→r5）：全マスタデータHash形式統一。T-03結合テスト仕様書（r1）作成完了、テストドキュメント進捗率25%→50%、全体進捗率59%→63%。 |
| r12 | 2026-02-07 | テストドキュメント完了: T-02単体テスト仕様書（r1）作成完了（全876テストケース）、T-04 E2Eテスト仕様書（r2）作成完了（実機3件+擬似E2E 5件+実機異常系3件=計11シナリオ）。テストドキュメント進捗率50%→100%、全体進捗率63%→70%。 |
| r13 | 2026-02-07 | 構築・デプロイドキュメント完了: B-01ホストOS構築手順書（r1）、B-02アプリケーションデプロイ手順書（r1）作成完了。D-08版数更新（r8→r9、B-02スコープ修正）。構築・デプロイドキュメント進捗率0%→100%、全体進捗率70%→78%。 |
| r14 | 2026-02-16 | D-01版数更新（r8→r9）: 実装コードとの不整合20件修正（データモデル注記追加、パッケージマップ更新、Valkeyキースキーマ概要追加、docker-compose.yml完全同期、healthcheck・テストベクターモード・healthエンドポイント記載追加）。B-01版数更新（r1→r2）、B-02版数更新（r1→r2）: 関連ドキュメント参照版数をD-01 r9に整合。 |
| r15 | 2026-02-16 | テスト仕様書ファイル名に版数付与: T-02（r1）、T-03（r1→r3）、T-04（r2→r3）。T-03: Secret体系明確化、SQN再同期手順改訂、IMSI 003専用config追加、障害系PASS条件修正。T-04: SQN管理注意事項追加、Valkey再起動後データ残存確認追加。 |
| r16 | 2026-02-18 | 全文書版数を最新に更新 |
| r17 | 2026-02-22 | O-05ログ解析ガイド（r1）作成完了。運用ドキュメント進捗率0%→17%、全体進捗率78%→81%。 |
| r18 | 2026-02-22 | T-03結合テスト仕様書 版数更新（r3→r5）: go-eapaka PRF修正後の再テストで得られた知見を反映（identityオーバーライドIMSIのSQNリセット運用補足、Dockerイメージ再ビルド注意事項追加）。 |
| r19 | 2026-02-23 | lnavフォーマット・Fluent Bit設定修正に伴う版数更新: D-04(r13→r14)、D-08(r10→r11)、B-02(r2→r4)、O-05(r1→r2)。lnavプロパティ名をハイフン区切りに修正、Fluent Bitログルーティング修正（rewrite_tag廃止→parserフィルター＋タグベースルーティング）。 |
| r20 | 2026-02-23 | Fluent Bit 3.0→4.2アップグレード＋YAML移行に伴う版数更新: D-08(r11→r12)、O-05(r2→r3)。Classic config廃止に伴いYAML形式に移行、Distrolessベースイメージ対応、lnavフォーマットapp値定義追加。 |
| r21 | 2026-02-23 | README.md更新に伴う改版: 実装状況セクション新規追加、ドキュメント一覧を全22件カテゴリ別記載に拡充、環境変数説明・テスト規模情報追加、リポジトリ構成にpkg/httputil・deployments/.env.example・lnav_formats追加、Fluent Bit 4.2バージョン明記。B-01(r2→r3): D-08参照版数更新（r9→r12）。B-02(r4→r5): D-04参照版数更新（r13→r14）、D-08参照版数更新（r9→r12）、Fluent Bit設定ファイル参照をYAML形式に修正。 |
| r22 | 2026-02-23 | lnavフォーマット定義appフィールド欠落修正: D-04(r14→r15)、B-02(r5→r6) |
| r23 | 2026-02-23 | Admin TUI ドキュメント改善（実装スクリーンショットとのASCIIレイアウト整合性修正）: D-05(r8→r9、前半12画面)、D-07(r6→r7、後半5画面) |
| r24 | 2026-02-24 | eapaka_test利用ノウハウ補足資料作成: S-01(r1)新規作成、「6. 補足資料」セクション新設。T-03(r5→r6)・T-04(r3→r4)のeapaka_testパス参照をsupplement配下に一般化。進捗サマリ更新（総数27→28、完了22→23、進捗率81%→82%）。 |
| r25 | 2026-02-27 | O-01 操作ガイド(r1)作成完了。運用ドキュメント進捗率17%→33%、全体進捗率82%→86%（完了23→24件）。 |
| r26 | 2026-02-27 | O-02 ポリシー設定ガイド(r1)作成完了。運用ドキュメント進捗率33%→50%、全体進捗率86%→89%（完了24→25件）。 |
| r27 | 2026-02-27 | O-06（顧客説明ガイドライン資料）および「5.3 対外説明資料」セクション除去。総数28→27、未作成3→2、進捗率89%→93%。 |
| r28 | 2026-02-27 | O-03 障害対応手順書(r1)作成完了。障害検知・切り分け・コンポーネント別対応・復旧手順・エスカレーション・チェックリストを体系化。運用ドキュメント進捗率60%→80%、全体進捗率93%→96%（完了25→26件）。 |
| r29 | 2026-02-28 | O-04 バックアップ・リストア手順書(r1)作成完了。Valkeyデータの自動・手動バックアップ、リストア手順（6ステップ）、設定ファイルのバックアップと復元、トラブルシューティング、運用チェックリストを体系化。運用ドキュメント進捗率80%→100%、全体進捗率96%→100%（完了26→27件、全ドキュメント完了）。 |
| r30 | 2026-02-28 | lnavフォーマットtimestamp-format修正に伴う版数更新: D-04(r15→r16)、B-02(r7→r8)、O-05(r3→r4)。Go slog実出力形式（RFC3339Nano+数値TZオフセット）にマッチする`%N%z`パターンを追加。 |
| r31 | 2026-02-28 | lnavフォーマット定義全面改訂に伴う版数更新: D-04(r16→r17)、B-02(r8→r9)、O-05(r4→r5)。bunyan競合回避（フォーマット名変更）、timestamp-format削除、file-pattern追加、sample追加。B-02に§13.7トラブルシューティング新設、O-05に§13運用ノウハウ新設。 |
| r32 | 2026-03-01 | 開発ドキュメント版数更新: E-01(r2→r3)、E-02(r1→r2)、E-03(r2→r3)。実装・現行ドキュメントとの整合（IMSIマスキングD-04 r17準拠、pkg構成更新、golangci-lint/CI反映、Makefile/テストベクターモード追加、構造体タグjsonのみ化）。 |
| r33 | 2026-03-04 | ヘルスチェックログ分離＋全ログ重複解消: D-08(r12→r13)。rewrite_tagフィルタによるヘルスチェックログのアプリ専用ログ除外、match: "*"キャッチオール廃止→match: "healthcheck.*"に変更しapp.*タグの重複出力解消。 |
| r34 | 2026-03-05 | Accounting-On/Off対応＋ヘルスチェックログ分離のドキュメント反映: D-04(r17→r18)、D-10(r5→r6)、T-03(r6→r7)、O-05(r5→r6)。 |
| r35 | 2026-10-04 | Vector Gatewayに接続方式01（aka-only-server）を追加したことに伴う版数更新: D-01(r9→r10)、D-03(r5→r6)、D-04(r18→r19)、D-06(r6→r7)、D-08(r13→r14)、D-12(r4→r5)、T-02(r1→r2)、T-03(r7→r8)、T-04(r4→r5)、B-02(r9→r10)、O-01(r1→r2)、O-03(r1→r2)、S-01(r1→r2)。README.md も更新（アーキテクチャ表・環境変数表）。 あわせて既存記載の実装との不一致を修正: Vector Gateway/Auth Server の event_id（GW_ROUTE/GW_OK/GW_ERR、VECTOR_IMSI_NOT_FOUND 等）、エラー応答の detail 文言、docker-compose.yml 掲載内容（vector-api に TEST_VECTOR_* を渡すよう compose も修正）、T-02 の件数（削除済み logging テストの除去、全1,196件）、lnav クエリ（aka_radius_log テーブル等）。追加の版数更新: D-09(r9→r10)、E-02(r2→r3)、O-05(r6→r7)、T-01(r2→r3)。 さらに、D-04 の event_id を実装と全件突き合わせて整合し、参照文書（D-03/D-06/D-07/D-08/D-09/D-10/D-11/E-01/E-02/O-03/O-05/T-03）に反映。D-02 を実装に整合（nas_id は完全一致、SQN 競合制御は設計のみで現行未実装）。T-03/S-01 をテストベクターモードの実動作に整合（2026-10-04 実機確認）。追加の版数更新: D-02(r11→r12)、D-07(r7→r8)、D-10(r6→r7)、D-11(r6→r7)、E-01(r3→r4)。 |
| r36 | 2026-10-04 | ログのIMSIマスク漏れ修正（Auth ServerのEAP_UNSUPPORTED_TYPE/EAP_IDENTITY_INVALIDのuser_name、Vector APIのTEST_SQN_FALLBACK/TEST_SQN_PERSIST_ERRのimsi、Acct ServerのIMSI抽出不可時のUser-Name。pkg/logging.MaskUserName追加）＋lnavフォーマット修正（value整理・sample差し替え）に伴う版数更新: D-04(r19→r20)、B-02(r10→r11)、O-05(r7→r8)、D-06(r7→r8)、D-09(r10→r11)、D-10(r7→r8)、E-03(r3→r4)、T-02(r2→r3、全1,196件→1,216件)。 |
| r37 | 2026-10-04 | acct-serverのInterimシーケンス判定修正（StartなしのInterimで ACCT_SEQUENCE_ERR（no_start_received）を出力、Stop後のInterimを interim_after_stop として検出、Interimでセッション不在時は sess:{UUID} を作らず ACCT_SESSION_NOT_FOUND を出力）に伴う版数更新: D-10(r8→r9)、D-04(r20→r21)、D-02(r12→r13)、O-03(r2→r3)、O-05(r8→r9)、T-02(r3→r4、全1,216件→1,224件)。README.md も更新（ドキュメント一覧へのリンク、テスト件数）。 あわせて D-06(r8→r9)、D-09(r11→r12) を更新（Interim時のセッション不在・順序異常の記述を実装に整合）。 |
| r38 | 2026-10-04 | Vector APIのテストベクターモードでも加入者登録を必須にした実装修正（Ki/OPc/AMFだけをテスト用固定値に置き換え、加入者の取得・SQN管理・エラー処理は通常モードと同じ。未登録IMSIは404、既定SQN ff9bb4d0b607 へのフォールバックと TEST_SQN_FALLBACK / TEST_SQN_PARSE_ERR / TEST_SQN_PERSIST_ERR、GetDefaultSQN を削除し、未登録IMSIに sqn だけの sub:{IMSI} を作らない）に伴う版数更新: D-11(r7→r8)、D-02(r13→r14)、D-04(r21→r22)、T-01(r3→r4)、T-02(r4→r5、全1,224件→1,220件)、T-03(r8→r9)、T-04(r5→r6)、S-01(r2→r3)。あわせて D-01(r10→r11)、D-06(r9→r10)、D-08(r14→r15)、O-05(r9→r10) を更新（テストベクターモードの記述を実装に整合）。推奨作成順序の D-11 の版数を現行版（r8）に修正。README.md も更新（ドキュメント一覧へのリンク、テスト件数）。 |
| r39 | 2026-10-04 | ポリシーの nas_id で "*" を任意の NAS に一致させた Auth Server の実装修正（"*" 単独は任意の NAS-Identifier（空を含む）に一致、部分一致は行わない、それ以外は従来どおり完全一致。D-02 の当初設計と Admin TUI のバリデーションに合わせたもの）に伴う版数更新: D-02(r14→r15)、O-02(r1→r2)、T-04(r6→r7)、D-09(r12→r13)、T-02(r5→r6、全1,220件→1,228件)。あわせて D-03(r6→r7)、E-03(r4→r5)、O-01(r2→r3) を更新（D-03・D-09 のポリシー評価、E-03 の PolicyRule に残っていた実装に存在しない ssid/action/time_min/time_max の記述を実装の nas_id/allowed_ssids/vlan_id/session_timeout に整合）。README.md も更新（ドキュメント一覧へのリンク、テスト件数）。 |
| r40 | 2026-10-04 | auth-server の trace_id を認証単位で引き継ぐ実装修正（2回目以降の Access-Request は UUID 形式の State 属性を trace_id とし、PKT_RECV 等のハンドラー層のログもエンジンのログ・X-Trace-ID と同じ値になる）、EAPエンジンの Process が error を返さなくなったことによる EAP_ENGINE_ERR の削除、LOG_LEVEL 対応（既定 INFO、pkg/logging.ParseLevel 新設、compose で auth-server / vector-gateway / vector-api に LOG_LEVEL を渡す）に伴う版数更新: D-01(r11→r12)、D-02(r15→r16)、D-04(r22→r23)、D-06(r10→r11)、D-08(r15→r16)、D-09(r13→r14)、D-12(r5→r6)、E-01(r4→r5)、E-02(r3→r4)、E-03(r5→r6)、T-02(r6→r7、全1,228件→1,241件)、T-03(r9→r10)、B-02(r11→r12)、O-03(r3→r4)、O-05(r10→r11)。subdoc（T-02_UT-PKG / T-02_UT-AUTH、T-03_INT-006、T-04_E2E-001）と T-02 の他の subdoc の親リンクも更新。README.md も更新（ドキュメント一覧へのリンク、テスト件数、環境変数表に LOG_LEVEL）。 |
| r41 | 2026-10-04 | vector-api のログ整理と未使用コード削除の実装修正（SQN_RESYNC に trace_id とマスク済み imsi を追加（ハンドラーが usecase.ContextWithTraceID で context に載せる）、SQN_RESYNC_DELTA_ERR とテストモードの CALC_OK を1行に（SQN値はエラー文、CALC_OK に test_mode 属性）、ProblemError 経路のログに error 属性、未使用の GetWithRetry と ErrInvalidIMSI を削除、vector-api / vector-gateway の LOG_LEVEL 変換を pkg/logging.ParseLevel に統一、VectorUseCaseInterface に IsTestMode を追加）に伴う版数更新: D-04(r23→r24)、D-06(r11→r12)、D-11(r8→r9)、D-12(r6→r7)、E-02(r4→r5)、E-03(r6→r7)、T-02(r7→r8、全1,241件→1,245件)、T-03(r10→r11)、O-03(r4→r5)、O-05(r11→r12)。subdoc（T-02_UT-VAPI、T-03_INT-003、T-03_INT-006、T-04_pseudo_e2e_eapaka_test_scenarios、T-04_troubleshooting_guide）と T-02 の他の subdoc の親リンクも更新。README.md も更新（ドキュメント一覧へのリンク、テスト件数）。 |
