# EAP-AKA RADIUS PoC環境 ドキュメント一覧 (r53)

**作成日:** 2025-12-30
**最終更新:** 2026-10-04
**ステータス:** 運用準備フェーズ

---

## 1. 設計ドキュメント

### 1.1 作成済み

| No. | ドキュメント名 | 版数 | 最終更新 | 内容 |
|-----|---------------|------|---------|------|
| D-01 | ミニPC版 EAP-AKA RADIUS PoC環境 設計仕様書 | r17 | 2026-10-04 | システム概要、アーキテクチャ、ノード構成、パッケージマップ、Vector Gateway追加、Valkeyバージョン9.0統一、Fluent Bit統一、環境変数名統一、実装コードとの不整合20件修正（データモデル注記・Valkeyキースキーマ・パッケージマップ・docker-compose.yml完全同期・healthcheck・テストベクターモード）、外部aka-only-server（接続方式01）を構成図・環境変数に追加、テストベクターモードの説明を実装に整合（固定 Ki/OPc/AMF で計算、加入者登録必須）、compose の LOG_LEVEL（auth-server / vector-gateway / vector-api）を実ファイルに同期、compose の LOG_LEVEL を acct-server にも渡す（4サーバー）よう実ファイルに同期、Valkeyキースキーマを実装に整合（存在しない audit:log・acct:{ID} を削除し sess:・idx:user:・acct:seen: を追加、監査ログは Admin TUI の標準出力）、vector-gateway のバックエンド向けタイムアウトの既定値を 3s に（auth-server の5秒より短く。§3.5・compose・環境変数表）、ファイアウォールの記述を訂正（Docker の公開ポートは UFW を素通りする）、インターネット越しに RADIUS を受ける場合の注意、fluent-bit の公開を 127.0.0.1 に、RADIUS_SECRET を任意（空を推奨）に、Acct Server の検証方式（Request Authenticator）を訂正 |
| D-02 | Valkeyデータ設計仕様書 | r19 | 2026-10-04 | データ構造、キー設計、TTL、Go構造体、CK/IK非保存方針、UUID仕様明記、Acct重複検出キャッシュ、stageフィールド値明記、SQN競合制御（WATCH/MULTI CAS）、idx:userクリーンアップ方針、PolicyRule構造を実装コードに整合（NasID/AllowedSSIDs/VlanID/SessionTimeout）、実装との突き合わせによる修正（SQN競合制御は設計のみで現行未実装、セッション/インデックス/重複検出/EAPコンテキストの実態）、Interimのセッション不在時に作成しない・重複検出のInterim判定（no_start_received / interim_after_stop）を実装に整合、テストベクターモードでも加入者登録必須・既定SQNフォールバック廃止（`sqn` だけの Hash を作らない）を実装に整合、nas_idの`*`単独で任意のNASに一致（部分一致なし、それ以外は完全一致）を実装に整合、Trace ID は初回に生成し以降は State 属性の UUID を引き継ぐ旨を実装に整合、重複Interimの event_id を ACCT_DUPLICATE_INTERIM に分離、SQN競合制御を実装に整合（Lua による `sqn` の比較・置き換え、競合時のやり直し最大3回・409、Admin TUI による `sqn` 上書きの制約）、Admin TUI の加入者編集による `sqn` の書き込みを実装に整合（SQN を変えたときだけ編集開始時の値と比較して書き換え、Hex は大文字に正規化して保存） |
| D-03 | Vector-APIインターフェース定義書およびEAP-AKAステートマシン設計書 | r8 | 2026-10-04 | API仕様、8状態定義、Policy評価Post-Authのみ、接続先をVector Gatewayに変更、接続方式01経由時の403応答、Post-Authのルール評価を実装のPolicyRule（nas_id/allowed_ssids、nas_idの`*`は任意のNASに一致）に整合、409 Conflict を実装に整合（Lua による SQN の比較・置き換えで3回とも競合した場合、detail は固定文、Circuit Breaker 対象外） |
| D-04 | ログ仕様設計書 | r31 | 2026-10-04 | ログフォーマット、event_id定義、Vector Gateway対応、EAP_INVALID_STATE追加、IMSIマスキング（4コンポーネント対応、Admin TUI除外明記）、Acct Server PKT_RECV追加、SQN_CONFLICT_ERR追加、lnavフォーマット全面改訂（bunyan競合回避・timestamp-format削除・file-pattern・sample追加）、ACCT_ON/ACCT_OFF追加、ヘルスチェックログ分離記述追加、BACKEND_EXTERNAL_CALL/BACKEND_EXTERNAL_ERR実装済み化・causeフィールド追加、IMSIマスク漏れ修正の反映（user_name等のマスク、User-Nameマスク規則追加）・lnavフォーマットのvalue整理（code/subtype/eap_type/acct_status_type/session_time追加、retry_count/downtime_ms削除）、ACCT_SEQUENCE_ERRのInterim側msg変更・interim_after_stop追加、ACCT_SESSION_NOT_FOUNDをStart/Interimに拡大、テストベクターモードのTEST_SQN_FALLBACK/TEST_SQN_PARSE_ERR/TEST_SQN_PERSIST_ERRを削除（エラーログは通常モードと同じ）、auth-server の trace_id を認証単位で引き継ぎ（2回目以降の PKT_RECV 等も同一 trace_id）、EAP_ENGINE_ERR 削除、LOG_LEVEL（§4.6新設、DEBUG で vector api success）、Vector API のログ整理（SQN_RESYNC に trace_id・imsi、SQN_RESYNC_DELTA_ERR とテストモードの CALC_OK を1行化・CALC_OK に test_mode、ProblemError 経路のログに error 属性、Vector Gateway / Vector API も ParseLevel で WARNING を WARN）、Acct Server の重複Interimを ACCT_DUPLICATE_INTERIM に分離・SYS_ERR 削除・LOG_LEVEL 対応（4コンポーネント、起動ログに log_level）、Admin TUI の監査ログに件数（import / export の record_count、search の result_count）を追加し検索IMSIを target_imsi に記録、Vector Gateway の起動時 WARN（バックエンド向けタイムアウトが auth-server の5秒以上）を追加、SQN競合制御の実装で SQN_CONFLICT_RETRY（attempt）/ SQN_CONFLICT_ERR を §3.4.7 に再追加・SQN_RESYNC を書き換え成功後に1回（msg 2種）、lnav フォーマットに attempt、Admin TUI の加入者編集が `sqn` を上書きしなくなったことを §3.4.7 に反映、auth-server / acct-server の起動ログに radius_secret_fallback と RADIUS_SECRET 設定時の WARN、RADIUS_NO_SECRET の条件を明記、RADIUS_LIB_ERR（ライブラリのエラーを JSON で）を追加し、Accounting-Request のシークレット不一致の RADIUS_AUTH_ERR・未知の Code のログを実装に整合 |
| D-05 | Admin TUI詳細設計書【前半】 | r12 | 2026-10-04 | 画面設計、バリデーション、インポート/エクスポート、IMSI表示方針（常に生値）、全マスタデータHash形式統一、実装スクリーンショットとのASCIIレイアウト整合性修正（12画面）、バリデーション規則を実装に整合（NAS ID 1〜253文字の印字可能ASCII・`*` ワイルドカード、NAS ID は NAS-Identifier と比較、SQN・Client Name 必須、エラーメッセージ等）、ポリシールールの例を現行構造に修正、監査ログの import / export に record_count、キー配線漏れ修正の反映（一覧の Enter で編集画面、`?` は入力欄では文字入力、一覧の F6 でフィルタ、フィルタ入力ダイアログを Esc で閉じる）、キー操作・ダイアログを実装に整合（終了確認・変更破棄確認・上書き確認ダイアログはない、Default allow 警告は Continue / Cancel、ページ切替は PgUp / PgDn、ポリシーフォームに Ctrl+S はない、接続失敗時は Connection Error の Retry / Exit）、加入者編集の保存処理（SQN を変えていなければ `sqn` を書かない、変えたときは編集開始時の値と比較し変わっていれば保存せずエラー、SQN 変更判定は大文字小文字を区別しない） |
| D-06 | エラーハンドリング詳細設計書 | r17 | 2026-10-04 | 異常系処理、タイムアウト、リトライ、Circuit Breaker、Vector Gateway追加、SQN競合エラー（409）追加、Vector Gateway経由フロー明記、接続方式01（aka-only-server）のエラー変換、EAP Identity系ログのuser_nameマスク反映、Interimのセッション不在・順序異常の扱いを実装に整合、テストベクターモードのエラー処理を通常モードと同一化、EAPエンジンが error を返さない実装に合わせ EAP_ENGINE_ERR を削除、Vector API の Valkey リトライなし（未使用の GetWithRetry 削除）・SQN_RESYNC_DELTA_ERR の1行化・定義済みエラーのログに error 属性、重複Interimの ACCT_DUPLICATE_INTERIM 分離、Accounting処理が error を返さない実装に合わせ SYS_ERR を削除、Vector Gateway → バックエンドのタイムアウトを 3秒に（auth-server の5秒より短くする理由、バックエンド障害時は VECTOR_API_ERR（502））・起動時 WARN を追加、SQN競合上限超過（409）を実装済みに（待機中の期限切れも 409、409 を 5xx にしない理由）、Valkey 接続断時の動作をフォールバックが空の場合（応答なし）と分けて記載、RADIUS パケットの認証をハンドラーで行う扱い（シークレット不一致は RADIUS_AUTH_ERR、形の壊れたパケットは RADIUS_LIB_ERR） |
| D-07 | Admin TUI詳細設計書【後半】 | r10 | 2026-10-04 | モニタリング画面、ヘルプダイアログ、IMSI記録方針（監査ログに生値）、idx:userクリーンアップ処理、実装スクリーンショットとのASCIIレイアウト整合性修正（5画面）、event_idを実装に整合、Session Detail 検索の監査ログを実装に整合（検索IMSIを target_imsi、結果件数を result_count、検索失敗時は details に理由）、Session List のソートを現行の s キーによる3項目の切り替えに整合（Start Time ▼ → NAS IP ▲ → IMSI ▲、同値は start_time 降順 → UUID 順）、入力ダイアログを Esc で閉じる・ヘルプは F1 / ?、キー操作・画面遷移を実装に整合（Session Search へは Session List の Enter、ページ切替は PgUp / PgDn、Session Search はページ分割・r キーなしで Esc / q は Session List へ、IMSI 入力は検証しない）、Session Search の検索結果は画面側で start_time 降順に並べ替え、Session List のフィルタ（IMSI・NAS IP・Client IP の部分一致）に合わせて PoC 対象外・将来課題を修正、Statistics Dashboard の統計キャッシュ（件数のみ、要求時更新の1分キャッシュ）を実装に整合、Go構造体定義・フォーマット関数・セッション取得のコード片を実装（model.Session、SessionListScreen / SessionDetailScreen、internal/format の BytesShort 等）に整合 |
| D-08 | インフラ設定・運用設計書 | r20 | 2026-10-04 | Docker Compose設定、Valkey設定、Fluent Bit設定（fluent/fluent-bit:4.2、YAML形式、rewrite_tagによるヘルスチェックログ分離、キャッチオール廃止による重複出力解消）、UFW設定、運用手順、IMSIマスキング環境変数（4コンポーネント限定）、Valkeyバージョン9.0、ヘルスチェック方針（curl -fsS）、テストベクターモード環境変数、B-02スコープ修正（B-01境界整合）、aka-only-server接続（compose環境変数・証明書マウント・共有ネットワークaka-av用オーバーレイ）、テストベクターモードの.env.example説明を実装に整合（加入者登録必須）、compose / .env.example の LOG_LEVEL（auth-server / vector-gateway / vector-api）を実ファイルに同期、compose / .env.example の LOG_LEVEL を acct-server にも渡す（4サーバー）よう実ファイルに同期、compose / .env.example の vector-gateway のタイムアウト既定値（3s）と説明を実ファイルに同期、compose / .env.example の写しを更新（fluent-bit を 127.0.0.1 に、RADIUS_SECRET を任意）、Docker と UFW の関係、インターネット越しの RADIUS の注意（§5.7）、セキュリティチェックリストを更新、リストア手順を訂正（docker compose run でボリュームの中身を入れ替え）、バックアップスクリプトの改善、「RDB併用なし」を訂正 |
| D-09 | Auth Server詳細設計書 | r18 | 2026-10-04 | パッケージ構成、RADIUS受信処理、EAP制御フロー、Vector Gateway連携、セッション管理、IMSIマスキング、UUID仕様明記、互換性エイリアス削除、ベースイメージ方針、Vector関連event_id（VECTOR_IMSI_NOT_FOUND等）を実装に整合、user_nameのマスキング（MaskUserName）追加、Acct ServerのInterim時のセッション不在の扱いを修正、認可ポリシー評価（セクション8）を実装のPolicyRule構造とnas_idの`*`（任意のNASに一致）に整合、Trace ID の決定（State 属性の UUID 引き継ぎ）、EAPProcessor の Process(ctx, req) *Result・EAP_ENGINE_ERR 削除、LOG_LEVEL（pkg/logging.ParseLevel）対応、Vector Gateway 側のタイムアウト（3秒）を VectorRequestTimeout（5秒）より短くする理由と定数を合わせる必要の注記、RADIUS_SECRET を任意（空を推奨）に、起動ログの radius_secret_fallback と WARN、PacketServer の設定（InsecureSkipVerify、ErrorLog）とパケット認証をハンドラーで行う方針、main.go の例の NewServer を訂正 |
| D-10 | Acct Server詳細設計書 | r12 | 2026-10-04 | パッケージ構成、Accounting処理フロー、セッション更新ロジック、重複検出、IMSIマスキング、Status-Server対応、ベースイメージ方針、Accounting-On/Off対応（ProcessOn/ProcessOff、NAS-Identifier処理）、event_idを実装に整合、IMSI抽出不可時のUser-Nameをマスクして出力、Interimのシーケンス判定（CheckInterim、interim_after_stop）とセッション存在確認、重複Interimの ACCT_DUPLICATE_INTERIM 分離、AccountingProcessor の戻り値から error を外し SYS_ERR を削除、LOG_LEVEL 対応（環境変数・設定構造体・main.go のロガー初期化）、RADIUS_SECRET を任意（空を推奨）に、起動ログの radius_secret_fallback と WARN、PacketServer の設定（InsecureSkipVerify、ErrorLog）、Accounting-Request の Request Authenticator 検証をハンドラーで行う |
| D-11 | Vector API詳細設計書 | r11 | 2026-10-04 | パッケージ構成、HTTPサーバー設定、Milenage計算、SQN管理、SQN競合制御（WATCH/MULTI CAS）、エラーハンドリング、ベースイメージ方針、テストベクターモード本番無効化注記、event_idを実装に整合、SQN競合制御（CAS）は設計済み・現行未実装と明記、テストベクターモードを実装に整合（Ki/OPc/AMFのみ固定値、加入者登録必須、既定SQNフォールバックとTEST_SQN_*ログ廃止）、ログ整理（ContextWithTraceID で SQN_RESYNC に trace_id・imsi、ユースケース層のデルタ超過・test vector generated ログ削除、CALC_OK に test_mode、ProblemError 経路の error 属性、IsTestMode 追加）、未使用の GetWithRetry / ErrInvalidIMSI を削除、SQN競合制御を実装（方式を WATCH/MULTI から Lua による `sqn` の比較・置き換えに変更、CompareAndSetSQN、最大3回の試行と 1〜10ms の待ち、ErrSQNConflict（409）、再同期のやり直しで同期済みとみなす扱い、SQN の書き換え後にベクター生成）、Admin TUI の `sqn` 上書きの制約を解消済みに（§13.3、§13.6.9） |
| D-12 | Vector Gateway詳細設計書 | r9 | 2026-10-04 | 外部API連携設計、PLMNルーティング、接続方式管理、トレーサビリティ、IMSIマスキング、ベースイメージ方針（debian:bookworm-slim）、接続方式01（aka-only-server、mTLS/平文HTTP、GenerateAv変換）実装、compose抜粋・環境変数にLOG_LEVELを追加、LOG_LEVEL の変換を pkg/logging.ParseLevel に統一、compose の LOG_LEVEL を acct-server にも渡す旨に修正、バックエンド向けタイムアウトの既定値を 3s に変更（auth-server の5秒より短く）、5秒以上なら起動時 WARN |

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
| E-01 | 開発環境セットアップガイド | r8 | 2026-10-04 | Go環境構築、Go Workspace設定、依存パッケージ、ローカル開発手順、デバッグ方法、環境変数名統一（RADIUS_SECRET）、Makefileセクション追加、golangci-lint/CIセクション追加、テストベクターモード環境変数追加、event_idを実装に整合、LOG_LEVEL（開発用.env・実行例・環境変数一覧）追加、LOG_LEVEL の対象に acct-server を追加、VECTOR_GATEWAY_INTERNAL_TIMEOUT の例・既定値を 3s に、24224 の公開範囲（127.0.0.1 のみ）と RADIUS_SECRET（任意）の説明を訂正 |
| E-02 | コーディング規約（簡易版） | r6 | 2026-10-04 | 命名規則、パッケージ構成、エラーハンドリングパターン、構造体タグ（jsonのみ）、ログ出力規約、IMSIマスキングD-04 r17準拠、golangci-lint導入済み反映、ログ出力例のevent_idを実装に整合、Handler層例のEAP_ENGINE_ERRを削除、ログレベルはLOG_LEVEL（pkg/logging.ParseLevel。Auth Server / Vector Gateway / Vector API 共通）で設定、LOG_LEVEL・ParseLevel の対象に Acct Server を追加 |
| E-03 | 共通ライブラリ(pkg)設計書 | r9 | 2026-10-04 | pkg配置方針、apperr/valkey/logging/model/httputil各パッケージ設計、IMSIマスキングD-04準拠、MaskUserName / Masker.UserName追加、PolicyRuleを実装（NasID/AllowedSSIDs/VlanID/SessionTimeout）に整合、ParseLevel（LOG_LEVEL→slog.Level変換）追加、ParseLevel の利用箇所に Vector Gateway / Vector API を追加、ParseLevel の利用箇所に Acct Server を追加、logging.NewRADIUSLibraryLogger（RADIUS ライブラリのログを slog に流す）を追加 |

### 2.2 未作成

なし（開発ドキュメント全3件完了）

---

## 3. テストドキュメント

### 3.1 作成済み

| No. | ドキュメント名 | 版数 | 最終更新 | 内容 |
|-----|---------------|------|---------|------|
| T-01 | テスト戦略書 | r4 | 2026-10-04 | テストレベル定義、テスト範囲、テスト環境、モック戦略、テストデータ戦略、品質ゲート、テストベクターモード運用注記、.env例の環境変数名（RADIUS_SECRET）を実装に整合、テストベクターモードの動作を実装に整合（加入者登録必須） |
| T-02 | 単体テスト仕様書 | r16 | 2026-10-04 | コンポーネント別テストケース（全1,329件）、モック戦略、テストデータ設計、Vector Gateway接続方式01のテストケース追加、IMSIマスク漏れ修正のテストケース追加、acct-server Interimシーケンス判定修正のテストケース反映、vector-apiテストベクターモードの加入者登録必須化のテストケース反映、auth-serverポリシーnas_idワイルドカード（`*`）のテストケース追加、auth-server trace_id引き継ぎ・LOG_LEVEL対応のテストケース反映（ParseLevel・TraceIDFromState等を追加、EngineErrorを欠番）、vector-apiログ整理のテストケース反映（LogAttributes・LogsTraceIDAndMaskedIMSI等を追加、ErrInvalidIMSIの検証を欠番）、acct-server の重複Interim分離・SYS_ERR削除・LOG_LEVEL対応のテストケース反映（TestLoadLogLevel を追加、ProcessorError を欠番）、admin-tui 監査ログの件数・検索IMSI記録修正のテストケース反映（LogExport_ZeroRecords・LogCreate_NoCounts を追加、LogSearch を4件のテーブル駆動に）、vector-gateway のタイムアウト既定値変更・起動時 WARN のテストケース反映（TestWarnBackendConfig_Timeouts を追加）、admin-tui のキー配線漏れ修正のテストケース追加（`internal/ui` 配下に初のテスト。一覧の Enter / F6、IsTextInput、入力ダイアログの Esc、Session List のソート、Session Search の検索結果の並べ替え。新カテゴリ UT-TUI-UI）、SQN競合制御のテストケース追加（store の CompareAndSetSQN、競合時のやり直し・409・再同期、同一IMSIへの並行リクエスト）、Admin TUI の加入者編集（Update / UpdateWithSQN、編集・新規作成画面）のテストケース追加、auth-server / acct-server の起動時 WARN のテストケース追加、PacketServer の UDP テストと RADIUS ライブラリのログのテストケース追加 |
| T-03 | 結合テスト仕様書 | r14 | 2026-10-04 | コンポーネント間連携テスト、シナリオテスト、テストベクターモード検証、Valkeyデータ整合性検証、Secret体系明確化、SQN再同期手順改訂、IMSI 003専用config追加、障害系PASS条件修正、identityオーバーライドIMSIのSQNリセット運用補足、Dockerイメージ再ビルド注意事項追加、eapaka_testパス参照をsupplement配下に一般化、INT-ACCT-ON-017/INT-ACCT-OFF-018追加、aka-only-server結合シナリオ追加、INT-GW-PLMN-010の未実装IDを02に変更、テストベクターモードでも加入者登録必須（テストIMSI帯でも未登録は404）・事前準備での登録を明記、INT-006 の期待結果に Auth Server の全パケットの PKT_RECV も同一 trace_id であることを追記、INT-006-03 に Vector API の SQN_RESYNC も同一 trace_id であることを追記、G6 と INT-005 の ACCT_DUPLICATE_INTERIM、INT-FAULT の Acct Server ベストエフォート動作を現行ハンドラー（SYS_ERR なし）に整合、INT-FAULT-001（Vector API停止）の PASS 条件を VECTOR_API_ERR（502）のみに（vector-gateway のタイムアウト 3s）、G3（SQN再同期）の手順を訂正（サーバー側 SQN を IND=7 の `FF9BB4D0B587` にして確実に再同期を起こす、PASS 条件に再同期ログ） |
| T-04 | E2Eテスト仕様書 | r8 | 2026-10-04 | 実機テスト（SIM/AP）3件、擬似E2E（eapaka_test）5件、実機異常系3件の計11シナリオ、SQN管理注意事項追加、Valkey再起動後データ残存確認追加、eapaka_testパス参照をsupplement配下に一般化、aka-only-server接続E2Eシナリオと実施結果（2026-10-04）追加、テストベクターモードのT-03との差分（加入者登録必須）を実装に整合、認可ポリシーのnas_id `*`（任意のNASに一致）を反映、E2E-002 のログ確認に ACCT_DUPLICATE_INTERIM を追加 |

### 3.2 未作成

なし（テストドキュメント全4件完了）

---

## 4. 構築・デプロイドキュメント

### 4.1 作成済み

| No. | ドキュメント名 | 版数 | 最終更新 | 内容 |
|-----|---------------|------|---------|------|
| B-01 | ホストOS構築手順書 | r5 | 2026-10-04 | Ubuntu Serverインストール、初期設定、セキュリティ設定、Docker導入、systemdサービス登録、UFW は Docker の公開ポートに及ばない注意（送信元の制限はクラウド側ファイアウォールか DOCKER-USER）、Ubuntu 24.04 の SSH ポート変更の反映手順を訂正（ssh.socket、ss での確認） |
| B-02 | アプリケーションデプロイ手順書 | r18 | 2026-10-04 | リポジトリクローン、.env作成、Docker Compose起動、Admin TUI配置、logrotate設定、バックアップスクリプト配置、lnavフォーマット配置・全面改訂、lnavカスタムフォーマット適用失敗トラブルシューティング、aka-only-server接続手順、lnavフォーマットのvalue整理、オプション項目にLOG_LEVEL追加、LOG_LEVEL の対象に acct-server を追加、vector-gateway のタイムアウト既定値（3s）と5秒より短くする旨の注記、lnav フォーマットの写しに attempt を追加、RADIUS_SECRET を任意（空を推奨）に、RADIUS クライアント登録（送信元IP）の節を新設、デプロイ後チェックリストにテストベクターモード・フォールバックの無効と公開ポートの確認を追加、ポート競合確認を TCP/UDP に、バックアップスクリプトの改善（失敗時に空のファイルを残さない、600）、クローン URL、ログディレクトリを 755 に（logrotate）、コンテナ名を訂正、VPS は B-03 を参照 |
| B-03 | VPSデプロイ手順書（AWS Lightsail） | r2 | 2026-10-04 | VPS（AWS Lightsail の Ubuntu 24.04 LTS）に1から構築・デプロイする手順（机上確認）。インスタンス作成、静的IP、IPv4 / IPv6 ファイアウォール（SSH は 22 のまま管理端末のIPに限定、RADIUS は AP のグローバルIPに限定）、スワップ、admin ユーザー、B-01 / B-02 との差分、golang コンテナでの Admin TUI のビルド、自動スナップショット、VPS 固有のチェックリストとトラブルシューティング、admin ユーザーの作成を既存の admin グループに合わせて訂正（--ingroup admin） |

### 4.2 未作成

なし（構築・デプロイドキュメント全3件完了）

---

## 5. 運用ドキュメント

実装完了後に作成予定。この段階では必須前提・方針と概要を定義し、他ドキュメント作成の指針とする。

### 5.1 運用ガイド類

| No. | ドキュメント名 | 作成時期 | 必須前提・方針 | 概要 |
|-----|---------------|---------|---------------|------|
| O-01 | 操作ガイド（user-guide.md） | **完了 (r7)** | D-05, D-07の完成後 | 加入者登録、ポリシー設定、セッション監視の操作手順。ASCIIレイアウト付き。PLMNによる加入者登録先（Admin TUI / aka-only-server）の区別。ポリシーCSVのnas_id `*`（任意のNASに一致）の説明。入力規則を実装に整合（SQN・Client Name 必須、Vendor 0〜64文字、保存時のみ検証・大文字化、NAS ID 1〜253文字、既存キーの新規登録はエラー、インポートは既存キーを上書き）。ダイアログ（削除確認は Yes / No、Default allow 警告は Continue / Cancel、終了確認・変更破棄確認はない）とキー操作（一覧の ← / → は使わない、Session List のソートは s、Session Search へは Enter）を実装に整合。キー配線漏れ修正の反映（ヘルプは F1 / ?（入力欄では ? は文字入力）、一覧の Enter で編集画面、一覧の F6 でフィルタ、Session List のソートは Start Time ▼ / NAS IP ▲ / IMSI ▲、入力ダイアログは Esc でも閉じる）。確認ダイアログの Esc は No / Cancel と同じ、ルール編集ダイアログは Esc では閉じない。、加入者編集の保存時の動作（SQN を変えなければ認証で進んだ SQN を保つ、SQN が変わっていた場合のエラーと対処）、CSV インポートで SQN も上書きされる注意、RADIUS クライアント登録の運用上の注意（登録する送信元IP、NAT・動的IP、共有シークレットの強度、フォールバックを空にした場合） |
| O-02 | ポリシー設定ガイド（policy-config-guide.md） | **完了 (r3)** | D-02の認可ポリシー設計確定後 | NAS-ID/SSID設定例、VLAN割り当て例、トラブルシューティング、NAS ID `*`（任意のNASに一致）の設定例と評価順の注意。ポリシー画面の操作を実装に整合（Ctrl+S はない、Default allow 警告は Continue / Cancel、F6 はフォーム→ルールリスト、ルール追加ダイアログは Add Rule、マウス操作は無効）、バリデーション規則を実装に整合 |
| O-03 | 障害対応手順書 | **完了 (r12)** | D-06, D-08の完成後 | 障害検知方法、切り分け手順、復旧手順、エスカレーションフロー、aka-only-server接続（接続方式01）の切り分け、Acct Serverの順序異常（reason別）・Interimのセッション不在の切り分け、EAP_CTX_NOT_FOUND の trace_id による切り分け手順、LOG_LEVEL=DEBUG による Vector 呼び出し時間の確認、Vector API の VALKEY_CONN_ERR / CALC_ERR の error 属性で原因を確認、重複Interimの ACCT_DUPLICATE_INTERIM 分離と切り分け、SYS_ERR 削除、Vector API停止時は VECTOR_API_ERR（502）になる旨と vector-gateway のタイムアウト設定（起動時 WARN）の切り分け、SQN競合（SQN_CONFLICT_ERR / SQN_CONFLICT_RETRY、409）の対応手順を実装に整合、Admin TUI の加入者編集で SQN が巻き戻らなくなったことを反映、SQN リセット手順にエラー時の再実行を追加、Valkey 接続断時の応答をフォールバックが空の場合に合わせて訂正、バックアップリストア手順・Vector Gateway/API のヘルスチェック・logrotate とサービスの名前を訂正、RADIUS_AUTH_ERR（Accounting のシークレット不一致）・RADIUS_LIB_ERR の対処を追加 |

### 5.2 保守ドキュメント類

| No. | ドキュメント名 | 作成時期 | 必須前提・方針 | 概要 |
|-----|---------------|---------|---------------|------|
| O-04 | バックアップ・リストア手順書 | **完了 (r2)** | D-08でバックアップ方針定義後 | Valkeyデータのバックアップ/リストア（自動・手動）、リストア手順、設定ファイルのバックアップと復元、トラブルシューティング、運用チェックリスト、リストア手順を訂正（5ステップ、docker compose run でボリュームの中身を入れ替え、加入者キー sub:*）、バックアップのパーミッションとホスト外への退避 |
| O-05 | ログ解析ガイド | **完了 (r18)** | D-04の完成後 | lnavの使い方、頻出クエリ集、障害調査パターン。event_idを実装に整合、lnavクエリを実動作に整合（aka_radius_logテーブル）、user_nameマスク反映・SQLカラム（code/subtype/eap_type/acct_status_type/session_time）追加、ACCT_SEQUENCE_ERR（interim_after_stop）・ACCT_SESSION_NOT_FOUND（Interim）の反映、テストベクターモードのTEST_SQN_*を削除、2回目以降の PKT_RECV も同一 trace_id で追跡できる旨に修正、EAP_ENGINE_ERR 削除、LOG_LEVEL=DEBUG の vector api success、Vector API の SQN_RESYNC を trace_id で追跡・SQN_RESYNC_DELTA_ERR の error から SQN 値を取り出すクエリ・CALC_OK の test_mode で抽出するクエリ、ACCT_DUPLICATE_INTERIM の追加と重複検出クエリ（両 event_id）、SYS_ERR 削除、Admin TUI 監査ログの件数（record_count / result_count）と検索IMSI（target_imsi）の反映・jq 例の更新、VECTOR_CONN_ERR の原因特定に vector-gateway のタイムアウト設定（起動時 WARN）の確認を追記、SQN競合制御のログ（SQN_CONFLICT_RETRY / SQN_CONFLICT_ERR、attempt）を実装済みとして記載、Admin TUI の保存で SQN が巻き戻らなくなったことを反映、RADIUS パケットの破棄の調査（RADIUS_AUTH_ERR・RADIUS_LIB_ERR・PKT_UNKNOWN_CODE）を追加 |

---

## 6. 補足資料

### 6.1 作成済み

| No. | ドキュメント名 | 版数 | 最終更新 | 内容 |
|-----|---------------|------|---------|------|
| S-01 | eapaka_test利用ノウハウ | r4 | 2026-10-04 | eapaka_testの設定・テストケース解説、SQN管理、configとIMSIの使い分け、トラブルシューティング、プロジェクト固有の運用知見。設定ファイル5件・テストケース15件をsupplement配下に格納、aka-only-server相手の再同期確認時のSQN（IND）注意、テストベクターモードでも加入者登録必須（未登録IMSIは404）を反映、内部 Vector API 相手の再同期の条件（サーバー側 SQN を `sqn_initial_hex` と同じ IND の少し古い値にする） |

> **格納場所**: `docs/supplement/eapaka_test/` 配下

---

## 7. ドキュメント依存関係図

```
[設計ドキュメント] ─────────────────────────────────────────────────────┐
    │                                                                   │
    ├─ D-01: ミニPC版設計仕様書 (r17) ✓                                  │
    ├─ D-02: Valkeyデータ設計仕様書 (r19) ✓                              │
    ├─ D-03: Vector-API/ステートマシン設計書 (r8) ✓                     │
    ├─ D-04: ログ仕様設計書 (r31) ✓                                     │
    ├─ D-05: Admin TUI詳細設計書【前半】(r12) ✓                         │
    ├─ D-06: エラーハンドリング詳細設計書 (r17) ✓                       │
    ├─ D-07: Admin TUI詳細設計書【後半】(r10) ✓                         │
    ├─ D-08: インフラ設定・運用設計書 (r20) ✓                            │
    ├─ D-09: Auth Server詳細設計書 (r18) ✓                               │
    ├─ D-10: Acct Server詳細設計書 (r12) ✓                              │
    ├─ D-11: Vector API詳細設計書 (r11) ✓                                │
    └─ D-12: Vector Gateway詳細設計書 (r9) ✓                            │
                    │                                                   │
                    ▼                                                   │
[開発ドキュメント] ─────────────────────────────────────────────────────┤
    │                                                                   │
    ├─ E-01: 開発環境セットアップガイド (r8) ✓                          │
    ├─ E-02: コーディング規約・簡易版 (r6) ✓                            │
    └─ E-03: 共通ライブラリ設計書 (r9) ✓                                │
                    │                                                   │
                    ▼                                                   │
[テストドキュメント] ───────────────────────────────────────────────────┤
    │                                                                   │
    ├─ T-01: テスト戦略書 (r4) ✓                                        │
    ├─ T-02: 単体テスト仕様書 (r16) ✓                                    │
    ├─ T-03: 結合テスト仕様書 (r14) ✓                                   │
    └─ T-04: E2Eテスト仕様書 (r8) ✓                                    │
                    │                                                   │
                    ▼                                                   │
[構築・デプロイドキュメント] ───────────────────────────────────────────┤
    │                                                                   │
    ├─ B-01: ホストOS構築手順書 (r5) ✓                                   │
    ├─ B-02: アプリケーションデプロイ手順書 (r18) ✓                        │
    └─ B-03: VPSデプロイ手順書（AWS Lightsail） (r2) ✓                    │
                    │                                                   │
                    ▼                                                   │
[運用ドキュメント] ◄────────────────────────────────────────────────────┘
    │
    ├─ O-01: 操作ガイド (r7) ✓
    ├─ O-02: ポリシー設定ガイド (r3)✓
    ├─ O-03: 障害対応手順書 (r12) ✓
    ├─ O-04: バックアップ・リストア手順書 (r2) ✓
    └─ O-05: ログ解析ガイド (r18)✓
```

---

## 8. 推奨作成順序

### フェーズ1: 設計完了

| 順序 | ドキュメントID | ドキュメント名 | ステータス |
|-----|---------------|---------------|-----------|
| 1 | D-07 | Admin TUI詳細設計書【後半】 | **完了 (r10)** |
| 2 | D-12 | Vector Gateway詳細設計書 | **完了 (r9)** |
| 3 | D-08 | インフラ設定・運用設計書 | **完了 (r20)** |
| 4 | D-09 | Auth Server詳細設計書 | **完了 (r18)** |
| 5 | D-11 | Vector API詳細設計書 | **完了 (r11)** |
| 6 | D-10 | Acct Server詳細設計書 | **完了 (r12)** |

### フェーズ2: 開発準備

| 順序 | ドキュメントID | ドキュメント名 | ステータス |
|-----|---------------|---------------|-----------|
| 7 | E-01 | 開発環境セットアップガイド | **完了 (r8)** |
| 8 | E-02 | コーディング規約（簡易版） | **完了 (r6)** |
| 9 | E-03 | 共通ライブラリ(pkg)設計書 | **完了 (r9)** |
| 10 | T-01 | テスト戦略書 | **完了 (r4)** |

### フェーズ3: 開発・テスト

| 順序 | ドキュメントID | ドキュメント名 | ステータス |
|-----|---------------|---------------|-----------|
| 11 | T-02 | 単体テスト仕様書 | **完了 (r16)** |
| 12 | T-03 | 結合テスト仕様書 | **完了 (r14)** |
| 13 | T-04 | E2Eテスト仕様書 | **完了 (r8)** |

### フェーズ4: 構築・デプロイ

| 順序 | ドキュメントID | ドキュメント名 | ステータス |
|-----|---------------|---------------|-----------|
| 14 | B-01 | ホストOS構築手順書 | **完了 (r5)** |
| 15 | B-02 | アプリケーションデプロイ手順書 | **完了 (r18)** |
| 15a | B-03 | VPSデプロイ手順書（AWS Lightsail） | **完了 (r2)** |

### フェーズ5: 運用準備（実装完了後）

| 順序 | ドキュメントID | ドキュメント名 | ステータス |
|-----|---------------|---------------|-----------|
| 16 | O-01 | 操作ガイド | **完了 (r7)** |
| 17 | O-02 | ポリシー設定ガイド | **完了 (r3)** |
| 18 | O-03 | 障害対応手順書 | **完了 (r12)** |
| 19 | O-04 | バックアップ・リストア手順書 | **完了 (r2)** |
| 20 | O-05 | ログ解析ガイド | **完了 (r18)** |

---

## 9. 進捗サマリ

| カテゴリ | 総数 | 作成済み | 未作成 | 進捗率 |
|---------|------|---------|-------|-------|
| 設計ドキュメント | 12 | 12 | 0 | 100% |
| 開発ドキュメント | 3 | 3 | 0 | 100% |
| テストドキュメント | 4 | 4 | 0 | 100% |
| 構築・デプロイドキュメント | 3 | 3 | 0 | 100% |
| 運用ドキュメント | 5 | 5 | 0 | 100% |
| 補足資料 | 1 | 1 | 0 | 100% |
| **合計** | **28** | **28** | **0** | **100%** |

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
| r42 | 2026-10-04 | acct-server の重複 Interim の event_id 分離・到達しない SYS_ERR の削除・LOG_LEVEL 対応の実装修正（重複 Interim は新設の ACCT_DUPLICATE_INTERIM（重複 Start は従来どおり ACCT_DUPLICATE_START）、AccountingProcessor の ProcessStart / ProcessInterim / ProcessStop / ProcessOn / ProcessOff の戻り値から error を外しハンドラーの SYS_ERR の分岐を削除（常に Accounting-Response を返す）、LOG_LEVEL 環境変数に対応（既定 INFO、pkg/logging.ParseLevel、起動ログに log_level）、compose / .env.example で acct-server にも LOG_LEVEL を渡し4サーバーとも対応）に伴う版数更新: D-01(r12→r13)、D-02(r16→r17)、D-04(r24→r25)、D-06(r12→r13)、D-08(r16→r17)、D-10(r9→r10)、D-12(r7→r8)、E-01(r5→r6)、E-02(r5→r6)、E-03(r7→r8)、T-02(r8→r9、全1,245件のまま（acct-server は1件追加・1件削除）)、T-03(r11→r12)、T-04(r7→r8)、B-02(r12→r13)、O-03(r5→r6)、O-05(r12→r13)。subdoc（T-02_UT-ACCT、T-03_INT-005、T-03_INT-FAULT、T-04_E2E-002）と T-02 の他の subdoc の親リンク、リネームした文書を参照する subdoc のパスも更新。README.md も更新（ドキュメント一覧へのリンク、環境変数表の LOG_LEVEL の対象サービス）。 |
| r43 | 2026-10-04 | Admin TUI の監査ログに件数と検索IMSIを正しく記録する実装修正（import / export に record_count（インポート/エクスポートしたレコード件数。0件も出力）、Session Detail の検索（search）で検索したIMSIを details ではなく target_imsi に記録し、検索の実行後に result_count（結果件数）を記録。検索に失敗した場合は result_count を出さず details に理由。LogSearch に検索エラーの引数を追加）と、D-05 のバリデーション規則を実装に合わせた修正（NAS ID 1-64文字→1〜253文字の印字可能ASCII・`*` ワイルドカード、NAS ID は NAS IPアドレスではなく NAS-Identifier と比較、SQN・Client Name 必須、Vendor 0〜64文字、エラーメッセージ等）に伴う版数更新: D-04(r25→r26)、D-05(r9→r10)、D-07(r8→r9)、T-02(r9→r10)、O-05(r13→r14)。T-02 は全1,250件（ID付与済み1,253件）に更新。推奨作成順序の D-07 の版数（r1 のままだった）を r9 に修正。あわせて、O-01 の入力規則を実装と D-05 r10 に合わせた修正（SQN・Client Name 必須、Vendor 0〜64文字、入力中の文字種制限・自動大文字化はなく保存時に検証・大文字化、ルールの入力規則、既存キーの新規登録はエラー、インポートは既存キーを上書き、エクスポートのファイル名に既定値なし、ステータスバーの表示）と、D-01 §3.3 Valkeyキースキーマを実装（D-02 r17）に合わせた修正（存在しない audit:log・acct:{ID} を削除、sess:・idx:user:・acct:seen: を追加、sub: の使用コンポーネントから Auth Server を削除、監査ログは Admin TUI の標準出力）に伴う版数更新: O-01(r3→r4)、D-01(r13→r14)。O-01 は画面説明も実装に合わせた（削除確認ダイアログは `Confirm Delete` の Yes / No、終了確認・変更破棄確認ダイアログはない、Default allow 警告は Continue / Cancel、VALKEY_ADDR は参照しない、ヘルプは F1 のみ、一覧の Enter・← / →・F6 は使わない、Session List のソートは s キー、Session Search へは Session List の Enter で移る） |
| r44 | 2026-10-04 | vector-gateway のバックエンド向けタイムアウトを auth-server より短くした実装修正（VECTOR_GATEWAY_INTERNAL_TIMEOUT / VECTOR_GATEWAY_AKAONLY_TIMEOUT の既定値を 5s → 3s。auth-server の Vector Gateway 呼び出しタイムアウト（5秒）と同じだと、バックエンド障害時に auth-server のログが VECTOR_API_ERR（502）になるか VECTOR_CONN_ERR になるかが環境次第で変わっていたため。設定値が5秒以上なら起動時に WARN「backend timeout should be shorter than the auth-server timeout; auth-server may time out before receiving 502」を出力）に伴う版数更新: D-01(r14→r15)、D-04(r26→r27)、D-06(r13→r14)、D-08(r17→r18)、D-09(r14→r15)、D-12(r8→r9)、E-01(r6→r7)、T-02(r10→r11)、T-03(r12→r13)、B-02(r13→r14)、O-03(r6→r7)、O-05(r14→r15)。T-02 は全1,254件（ID付与済み1,257件）に更新。T-03 の INT-FAULT-001（Vector API停止）の PASS 条件を VECTOR_API_ERR（http_status=502）のみに変更 |
| r45 | 2026-10-04 | Admin TUI のキー配線漏れを修正した実装修正（加入者・クライアント・ポリシーの一覧で Enter を押すと編集画面（ポリシーは Policy Details）を開く、? でヘルプを開く（入力欄にフォーカスがあるときは文字として入力）、加入者・クライアント・ポリシー・セッションの一覧で F6 でもフィルタを開く、Session List の s キーのソートの向きを項目ごとに固定（Start Time は新しい順 ▼、NAS IP と IMSI は昇順 ▲。同値は開始時刻の新しい順→UUID順。以前は常に降順）、フィルタ・IMSI検索の入力ダイアログを Esc でも閉じられる）に伴う版数更新: O-01(r4→r5)、D-05(r10→r11)、D-07(r9→r10)、T-02(r11→r12)、O-02(r2→r3)。O-01 は r4 で入れた「? ではヘルプを開かない」「一覧の Enter では開かない・F6 は使わない」「ソートはいずれも降順」の記述を修正後の動作に改めた。D-07 は Session List のソート仕様を元設計の2モード（start_time 降順 / IMSI 昇順、i / t キー）から現行の3項目の切り替えに改めた。T-02 は全1,269件（ID付与済み1,272件）に更新（admin-tui に ui パッケージのテスト15件を追加）。あわせて、キー操作・ボタン名・ダイアログの記述を実装（main.go、internal/ui）に合わせて修正: D-05（終了確認・変更破棄確認・上書き確認ダイアログを削除、Default allow 警告を Continue / Cancel に、ポリシーフォームの Ctrl+S を削除、ページ切替を ← / → から PgUp / PgDn に、フィルタ・ステータスバー・起動時の接続エラーの記述を修正）、D-07（Session Search へは Session List の Enter、Session Search の PgUp / PgDn・r / F5 を削除し Esc / q は Session List へ、ページ切替を PgUp / PgDn に、Statistics・Session Search のエラー表示を修正）、O-01（ルール編集ダイアログは Esc で閉じない、確認ダイアログの Esc）、O-02（Ctrl+S を削除、Default allow 警告を Continue / Cancel に、F6 はフォーム→ルールリストの一方向、ルール追加ダイアログのタイトル・初期値、マウス操作は無効、バリデーション規則を実装に整合）。また、Session Search の検索結果を開始時刻の新しい順（同じなら UUID 順）に並べる実装修正（画面側の sortByStartTimeDesc。テスト TestSortByStartTimeDesc を追加）に合わせて D-07 の処理フロー・コード例を修正し、D-07 の PoC 対象外・将来課題の「NAS-IP / Client-IP フィルタ」を Session List のフィルタの実装に合わせて修正、D-07 §4 / §8.1 の統計キャッシュを実装（件数4項目、要求時更新の1分キャッシュ）に合わせて修正。さらに D-07 の設計初期のまま残っていたコード片・型名（§5.6 の Acct-ID 切り詰め、§5.8.4 の fetchAllSessions、§8.2 / §8.3 の SessionListItem・SessionDetailSummary 等、§9 の KB 表記のフォーマット関数）を実装（SessionStore.List、model.Session、SessionListScreen / SessionDetailScreen、internal/format）に差し替え、Session List の Duration の色・Traffic の表記（1.2K 形式）を D-07 / O-01 で実装に合わせた |
| r46 | 2026-10-04 | Vector API の SQN競合制御（CAS）の実装（D-11 r9 までの WATCH/MULTI の設計に代えて、`sub:{IMSI}` の `sqn` が読んだ値と一致するときだけ書き換える Lua スクリプト（`CompareAndSetSQN`）を store に新設し `UpdateSQN` を削除。競合したら 1〜10ms 待って加入者の読み出しからやり直し、最大3回試行して3回とも競合したら 409（`SQN_CONFLICT_ERR`）、やり直すたびに `SQN_CONFLICT_RETRY`（`attempt`）。再同期のやり直しで SQN_HE が SQN_MS 以上なら同期済みとみなして +32。ベクター生成と `SQN_RESYNC` は SQN の書き換えに成功した後。lnav フォーマットに `attempt`）に伴う版数更新: D-02(r17→r18)、D-03(r7→r8)、D-04(r27→r28)、D-06(r14→r15)、D-11(r9→r10)、B-02(r14→r15)、O-03(r7→r8)、O-05(r15→r16)、T-02(r12→r13。テストケース 1,269→1,299件)。Admin TUI の加入者編集が `sqn` を上書きする制約は別途対応予定と各文書に記載 |
| r47 | 2026-10-04 | Admin TUI の加入者編集が SQN を巻き戻す問題の修正（SQN を変えずに保存したときは `sqn` を書き換えない（ki / opc / amf だけを更新）。SQN を変えたときは、編集開始時に読んだ値と一致するときだけ書き換え、編集中に認証で SQN が進んでいたら何も保存せずエラーを表示。存在チェックと更新を1つの Lua スクリプトで行い、途中で削除された加入者の一部フィールドだけの Hash を作らない。SQN の変更判定を大文字小文字を区別しない比較にし、Vector API が小文字で書き戻した SQN で誤って警告が出て古い値で上書きされていた問題も修正）に伴う版数更新: D-02(r18→r19)、D-04(r28→r29)、D-05(r11→r12)、D-11(r10→r11)、O-01(r5→r6)、O-03(r8→r9)、O-05(r16→r17)、T-02(r13→r14。テストケース 1,299→1,314件)。PR #23（r46）で「別PRで対応予定」とした制約を解消済みに改めた |
| r48 | 2026-10-04 | T-03 G3（SQN再同期）の手順の誤りの訂正（サーバー側 SQN を `000000000001`（IND=1）にすると eapaka_test の別の IND スロットと比較されて再同期が起きずに Accept になっていたため、`sqn_initial_hex` `FF9BB4D0B607` と同じ IND=7 の少し古い値 `FF9BB4D0B587` に変更。2026-10-04 に simwifi 実機で、修正前は再同期が起きないこと・修正後は INT-003-01/02 とも再同期が起きて Accept となることを確認）に伴う版数更新: T-03(r13→r14)、S-01(r3→r4) |
| r49 | 2026-10-04 | インターネット公開（VPS 等）に向けた安全面の修正（fluent-bit の 24224 を 127.0.0.1 だけに公開、RADIUS_SECRET（フォールバックの共有シークレット）を任意にして空を推奨（空なら登録のない送信元IPのパケットは破棄）、auth-server / acct-server の起動ログに radius_secret_fallback と設定時の WARN を追加）と、文書の訂正（Docker が公開したポートは UFW を素通りすること、インターネット越しに RADIUS を受ける場合の注意（共有シークレット、送信元IPとクライアント登録、NAT・動的IP、クラウド側ファイアウォール）、デプロイ後・セキュリティのチェックリストにテストベクターモード・フォールバックの無効と公開ポートの確認を追加）に伴う版数更新: B-01(r3→r4)、B-02(r15→r16)、D-01(r15→r16)、D-04(r29→r30)、D-06(r15→r16)、D-08(r18→r19)、D-09(r15→r16)、D-10(r10→r11)、E-01(r7→r8)、O-01(r6→r7)、O-03(r9→r10)、T-02(r14→r15。テストケース 1,314→1,318件) |
| r50 | 2026-10-04 | バックアップ・リストア手順の訂正（2026-10-04 に simwifi 実機で、旧手順が失敗すること・新手順で戻せることを確認。リストアは `docker compose stop valkey` → `docker compose run` で valkey サービスのボリュームの中身をバックアップと入れ替え → `start`。旧手順はボリューム名が実際の `deployments_valkey_data` と異なり、停止しただけのコンテナが参照するボリュームの `volume rm` は失敗していた。バックアップスクリプトは失敗時に 0バイトのファイルを残さず、バックアップを 600 で作る）と、運用手順の誤りの訂正（加入者キー `sub:*`、Vector Gateway/API のヘルスチェックはコンテナ内で curl、logrotate・systemd の名前、D-08 の「RDB併用なし」）に伴う版数更新: B-02(r16→r17)、D-08(r19→r20)、O-03(r10→r11)、O-04(r1→r2) |
| r51 | 2026-10-04 | RADIUS パケットの認証をハンドラーに一本化し、ライブラリのログを JSON にした実装修正（auth-server / acct-server の PacketServer に InsecureSkipVerify: true と ErrorLog（pkg/logging.NewRADIUSLibraryLogger）を設定。シークレットが一致しない Accounting-Request は、ライブラリが素のテキスト「bad secret」を出して捨てていたため RADIUS_AUTH_ERR が出ていなかったが、ハンドラーが送信元IP付きの RADIUS_AUTH_ERR を出して捨てるようになった。未知の Code もハンドラーに届き PKT_UNKNOWN_CODE / RADIUS_UNKNOWN_CODE で記録。形の壊れたパケット等のライブラリのエラーは RADIUS_LIB_ERR（WARN）、シークレットが空の重複ログは DEBUG）に伴う版数更新: D-04(r30→r31)、D-06(r16→r17)、D-09(r16→r17)、D-10(r11→r12)、E-03(r8→r9)、O-03(r11→r12)、O-05(r17→r18)、T-02(r15→r16。テストケース 1,318→1,329件) |
| r52 | 2026-10-04 | VPS 向けのデプロイ手順書 B-03（AWS Lightsail、r1、机上確認）を追加（構築・デプロイドキュメント 2→3件、総数 27→28件）。あわせて B-01 / B-02 の誤りを訂正（Ubuntu 24.04 の SSH はソケット起動のためポート変更は daemon-reload と ssh.socket の再起動で反映し ss で確認、git clone したログディレクトリは 775 で logrotate が処理を飛ばすため 755 に、クローン URL、コンテナ名）、D-01 の Acct Server の検証方式、D-09 の main.go の例の NewServer を訂正: B-01(r4→r5)、B-02(r17→r18)、B-03(r1 新規)、D-01(r16→r17)、D-09(r17→r18) |
| r53 | 2026-10-04 | B-03 §4.4 の admin ユーザーの作成の訂正（Ubuntu には旧来の sudo 用の admin グループが既にあり、`adduser admin` が失敗することを Lightsail の実機で確認。`--ingroup admin` で既存グループを主グループにする）に伴う版数更新: B-03(r1→r2) |
