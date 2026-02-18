# D-01 ミニPC版 EAP-AKA RADIUS PoC環境 設計仕様書 (r9)

## 1. システム概要

- **目的:** 持ち運び可能なミニPC単体で、EAP-AKA/AKA'認証および課金機能を提供する（RadSec/TLS機能はスコープ外）
- **プラットフォーム:** Ubuntu Server (Host OS) + Docker Compose
- **運用特性:**
  - 電源投入でコンテナ群が自動起動 (`restart: always`)。
  - 管理操作は、SSH経由でホストOS上の **TUI (Text User Interface)** アプリを用いて行う。
  - ネットワーク未接続（オフライン）状態でもローカル動作可能。

## 2. アーキテクチャ構成

外部のAPやRADIUSクライアントは、ミニPCのUDPポートへ直接認証パケットを送信します。内部ではマイクロサービス構成のGo製サーバー群が処理を行います。

```
[ 外部機器 (AP/NAS) ]
      |
      | (Legacy RADIUS UDP: 1812/1813)
      v
+--[ Host OS (Ubuntu Server) : IP xxx.xxx.xxx.xxx ]-----------------------+
|  [ Firewall (UFW) ] ALLOW: 10022(TCP), 1812(UDP), 1813(UDP)             |
|                                                                         |
|  [ Docker Compose Network (Bridge) ]                                    |
|     |                                                                   |
|     +-- [1. auth-server] (Go / Auth Logic) <--- UDP 1812 受信           |
|     |       | (HTTP / JSON / Trace ID Propagation)                      |
|     |       v                                                           |
|     |   [3. vector-gateway] (Go / Router) ◄--- PLMNベースルーティング    |
|     |       | (HTTP / JSON / Trace ID Propagation)                      |
|     |       v                                                           |
|     |   [4. vector-api] (Internal Only) ◄--- 認証ベクター計算            |
|     |                                                                   |
|     +-- [2. acct-server] (Go / Acct Logic) <--- UDP 1813 受信           |
|     |                                                                   |
|     +-- [6. fluent-bit]  (Log Collector) ---> [Host Logs]               |
|     |                                                                   |
|     +-- [5. valkey] (DB / Redis Compatible)                             |
|             ^                                                           |
|             | (TCP 6379: Internal)                                      |
|             +----(TCP 127.0.0.1:6379 / ACL Auth)-----+                  |
|                                                      |                  |
|  [ Host Application ]                                v                  |
|  +-----------------------------------------------------------+          |
|  | [7. admin-tui] (Go + tview)                               |          |
|  |  - SSH接続(Port 10022)した管理者が実行                    |          |
|  |  - Valkeyデータ操作 (IMSI/Ki/OPc等)                       |          |
|  +-----------------------------------------------------------+          |
|                                                                         |
+-------------------------------------------------------------------------+
```

## 3. Go実装ノード詳細と採用パッケージ

本システムを構成する5つのGoアプリケーションの役割と、使用するライブラリのマッピングです。

### 3.1 実装ノード一覧

| **ノード名**           | **ディレクトリ**       | **役割**                                | **機能要件・処理フロー**                                     |
| ---------------------- | ---------------------- | --------------------------------------- | ------------------------------------------------------------ |
| **1. Auth Server**     | `apps/auth-server`     | **[RADIUS認証 & EAP制御]** 認証の司令塔 | 1. **設定読込:** `envconfig` で環境変数ロード。Shared SecretはValkey優先、環境変数はフォールバックとして使用。`EAP_AKA_PRIME_NETWORK_NAME` 環境変数でEAP-AKA'のネットワーク名を設定（デフォルト: "WLAN"）。 2. **受信:** UDP 1812で受信。 3. **Trace ID:** 受信時にUUID生成。これを`trace_id`ログ、Valkeyキー(`eap:{UUID}`)、RADIUS `State`属性として統一利用。 4. **API連携:** `resty` + `gobreaker` (Circuit Breaker) でVector Gatewayへ計算リクエスト (Header `X-Trace-ID` 付与)。 5. **EAP制御:** `go-eapaka` 利用。 6. **保存:** `go-redis` 利用。 7. **ログ:** `slog` で構造化出力。 |
| **2. Acct Server**     | `apps/acct-server`     | **[RADIUS課金]** 利用実績の記録         | 1. **設定読込:** `envconfig` でロード。SecretはAuth Server同様の優先順位。 2. **受信:** UDP 1813で受信。 3. **検証:** Shared SecretによるMessage-Authenticator検証。 4. **ログ:** 課金ログの構造化出力。 5. **更新:** Valkeyセッション状態更新。 |
| **3. Vector Gateway**  | `apps/vector-gateway`  | **[ルーティング]** ベクター取得先振分け | 1. **設定読込:** `envconfig` でロード。 2. **API提供:** REST API (`POST /api/v1/vector`)。Auth Serverとの互換性維持。`GET /health` エンドポイントでヘルスチェック応答を提供。 3. **ルーティング:** PLMNベースでバックエンド選択（PoC: 全て内部Vector APIへ転送）。 4. **Trace ID:** Header `X-Trace-ID` を読み取り、内部APIへ伝搬。 5. **ログ:** `slog` で構造化出力。 |
| **4. Vector API**      | `apps/vector-api`      | **[暗号計算サービス]** AKAベクター生成  | 1. **設定読込:** `envconfig` でロード。`TEST_VECTOR_ENABLED` 環境変数（デフォルト: false）を有効にすると、テストベクターモードが有効化され、特定IMSIプレフィックス（`TEST_VECTOR_IMSI_PREFIX`、デフォルト: "00101"）に対して固定テストベクターを返却する。 2. **API提供:** REST API (`POST /api/v1/vector`)。`GET /health` エンドポイントでヘルスチェック応答を提供。 3. **Trace ID:** Header `X-Trace-ID` を読み取りログコンテキストに設定。 4. **DB取得:** ValkeyからSIM鍵情報取得。 5. **計算:** `milenage` 実行。 6. **更新:** SQNインクリメントとValkey更新。 |
| **5. Admin TUI**       | `apps/admin-tui`       | **[管理コンソール]** データ操作UI       | 1. **設定読込:** `os.Getenv("VALKEY_PASSWORD")` でDB PASS取得。 2. **UI表示:** `tview` + `tcell` 利用。 3. **DB操作:** 加入者CRUD、RADIUSクライアント管理、ポリシー管理、CSV I/O。 4. **監視:** Valkey接続確認およびセッション閲覧。 5. **監査ログ:** 操作履歴の記録。 |

> **データモデル注記:**
> - **RadiusClient** (`client:{IP}`): `ip`, `secret`, `name`, `vendor` の4フィールド構成（`enabled` フィールドは廃止済み、`vendor` フィールドを追加）。
> - **Subscriber** (`sub:{IMSI}`): `imsi`, `ki`, `opc`, `amf`, `sqn`, `created_at` の6フィールド構成（`enabled` フィールドは廃止済み、`created_at` フィールドを追加）。
> - **ポリシー機能:** Policy構造体（`imsi`, `default`, `rules_json`）とPolicyRule構造体（`ssid`, `action`, `time_min`, `time_max`）によるIMSI単位のアクセス制御を提供。詳細はD-09「Auth Server詳細設計書」を参照。

### 3.2 パッケージ利用マップ

| **カテゴリ** | **パッケージ**              | **Auth** | **Acct** | **Gateway** | **Vector** | **TUI** | **Test** | **用途概要**                          |
| ------------ | --------------------------- | -------- | -------- | ----------- | ---------- | ------- | -------- | ------------------------------------- |
| **Core**     | `layeh/radius`              | ◎        | ◎        | -           | -          | -       | -        | RADIUSプロトコル処理                  |
|              | `oyaguma3/go-eapaka`        | ◎        | -        | -           | -          | -       | -        | EAP-AKAパケット処理                   |
|              | `wmnsk/milenage`            | -        | -        | -           | ◎          | -       | -        | AKA認証ベクター計算                   |
|              | `gin-gonic/gin`             | -        | -        | ◎           | ◎          | -       | -        | Web APIサーバー                       |
|              | `rivo/tview`                | -        | -        | -           | -          | ◎       | -        | ターミナルUI構築                      |
|              | `gdamore/tcell/v2`          | -        | -        | -           | -          | ◎       | -        | ターミナル制御（tview依存）           |
|              | `redis/go-redis`            | ◎        | ○        | -           | ◎          | ◎       | -        | Valkey(Redis)クライアント             |
| **Util**     | `kelseyhightower/envconfig` | ◎        | ◎        | ◎           | ◎          | -       | -        | 環境変数の一括ロード・型変換          |
|              | `google/uuid`               | ◎        | ◎        | -           | -          | -       | -        | UUID生成(Auth)・パース検証(Acct)      |
|              | `go-resty/resty`            | ◎        | -        | -           | -          | -       | -        | HTTPクライアント(Auth→GW)             |
|              | `sony/gobreaker`            | ◎        | -        | -           | -          | -       | -        | サーキットブレーカー                  |
| **Std**      | `log/slog`                  | ◎        | ◎        | ◎           | ◎          | -       | -        | 構造化ログ (JSON形式推奨)             |
| **Test**     | `go.uber.org/mock`          | -        | -        | -           | -          | -       | ◎        | モック生成・テスト用                  |
|              | `alicebob/miniredis/v2`     | -        | -        | -           | -          | -       | ◎        | Valkeyインメモリモック（テスト用）    |

### 3.3 Valkeyキースキーマ概要

各コンポーネントが使用するValkeyキーのプレフィックス一覧。詳細なデータ構造・TTL・操作仕様はD-02「Valkey データ設計仕様書」を参照。

| **キープレフィックス** | **用途**                  | **使用コンポーネント**           |
| ---------------------- | ------------------------- | -------------------------------- |
| `sub:{IMSI}`           | 加入者情報（SIM鍵等）    | Auth Server, Vector API, Admin TUI |
| `client:{IP}`          | RADIUSクライアント情報    | Auth Server, Acct Server, Admin TUI |
| `policy:{IMSI}`        | アクセスポリシー          | Auth Server, Admin TUI           |
| `eap:{UUID}`           | EAPセッション状態         | Auth Server                      |
| `acct:{ID}`            | 課金セッション            | Acct Server                      |
| `audit:log`            | 監査ログ（リスト型）      | Admin TUI                        |

### 3.4 Valkey接続管理

各コンポーネントからValkeyへの接続管理方針。

| 項目 | 方針 |
|------|------|
| 接続方式 | go-redis クライアントの接続プール機能を利用 |
| 再接続 | go-redis の自動再接続機能に依存 |
| ヘルスチェック | go-redis の自動ヘルスチェックに依存 |
| 復旧検知 | アプリケーション側でコマンド成功時に復旧をログ出力（`VALKEY_CONN_RESTORED`） |

> **注記：** Valkey接続断時の各コンポーネントの動作詳細は「エラーハンドリング詳細設計書」を参照。

#### 接続タイムアウト設定

| コンポーネント | 接続タイムアウト | コマンドタイムアウト |
|---------------|-----------------|-------------------|
| Auth Server | 3秒 | 2秒 |
| Acct Server | 3秒 | 2秒 |
| Vector Gateway | - | - |
| Vector API | 3秒 | 2秒 |
| Admin TUI | 5秒 | 5秒 |

> **注記：** Vector GatewayはValkeyに直接接続しない（PoC段階）。

### 3.5 Vector Gateway接続設定

Auth ServerからVector Gateway、Vector GatewayからVector APIへの接続設定。

| 接続 | タイムアウト | 備考 |
|------|------------|------|
| Auth Server → Vector Gateway | 5秒 | 既存のVector API接続設定を継承 |
| Vector Gateway → Vector API | 5秒 | `VECTOR_GATEWAY_INTERNAL_TIMEOUT` で設定 |

## 4. ネットワーク・セキュリティ設定

### ホストOS (Ubuntu Server) 設定

- **SSH:**
  - Port: `10022` (デフォルト22から変更)
  - PermitRootLogin: `no` / PasswordAuthentication: `no` (公開鍵認証のみ推奨)
- **Firewall (UFW):**
  - Default: `DENY` (Incoming)
  - Allow:
    - `10022/tcp` (SSH Remote Admin)
    - `1812/udp` (RADIUS Authentication)
    - `1813/udp` (RADIUS Accounting)
  - **Block:** 上記以外全て（DBポート6379やAPIポート8080は外部から不可視）

### シークレット管理 (環境変数とDBの併用)

以下の機密情報は `.env` ファイルで管理し、各アプリへ注入します（コンテナアプリは `envconfig` 経由、Admin TUIは `os.Getenv()` で直接取得）。

- `VALKEY_PASSWORD`: DBアクセス用パスワード（必須）
- `RADIUS_SECRET`: **デフォルト共有シークレット**（フォールバック用）
  - **優先順位:**
    1. Valkey `client:{IP}` 内の `secret` を使用 (本番/登録済みクライアント)
    2. 上記がない場合、環境変数 `RADIUS_SECRET` を使用 (テスト/未登録クライアント)

## 5. コンテナ構成詳細 (Docker Compose)

| **No.** | **サービス名**      | **公開ポート**       | **役割**                                                     |
| ------- | ------------------- | -------------------- | ------------------------------------------------------------ |
| **1**   | **auth-server**     | 1812/udp             | 認証ロジック。`VECTOR_API_URL` と `RADIUS_SECRET` (Fallback) を環境変数で受ける。 |
| **2**   | **acct-server**     | 1813/udp             | 課金ロジック。`RADIUS_SECRET` (Fallback) を環境変数で受ける。 |
| **3**   | **vector-gateway**  | なし                 | ルーティング。PLMNベースでバックエンド選択。外部公開なし。`GET /health` でヘルスチェック応答。 |
| **4**   | **vector-api**      | なし                 | 内部API。外部公開なし。`TEST_VECTOR_ENABLED` 環境変数でテストベクターモードを有効化可能。`GET /health` でヘルスチェック応答。 |
| **5**   | **valkey**          | 127.0.0.1:6379       | DB。ホスト(TUI)用にlocalhostのみバインド。`--requirepass` 有効化。 |
| **6**   | **fluent-bit**      | 24224/tcp, 24224/udp | ログ収集。出力先は `/output_logs` 固定。                     |

## 6. 開発リポジトリ構成 (Monorepo)

```
my-radius-project/
│
├── apps/                         # アプリケーションコード
│   ├── auth-server/              # Dockerfile (Multi-stage) 含む
│   ├── acct-server/              # Dockerfile (Multi-stage) 含む
│   ├── vector-gateway/           # Dockerfile (Multi-stage) 含む
│   ├── vector-api/               # Dockerfile (Multi-stage) 含む
│   └── admin-tui/                # Host実行用 (ビルドしてscpで転送)
│
├── pkg/                          # 共通ライブラリ (Go Modules)
│   ├── model/                    # データモデル定義（Subscriber, RadiusClient, Policy等）
│   ├── valkey/                   # Valkeyクライアントラッパー
│   ├── logging/                  # 構造化ログユーティリティ
│   ├── apperr/                   # アプリケーションエラー型定義
│   └── httputil/                 # HTTPユーティリティ（レスポンスヘルパー等）
│
├── configs/                      # 設定ファイル
│   └── fluent-bit/
│       ├── fluent-bit.conf       # Fluent Bit設定
│       └── parsers.conf          # パーサー設定
│
├── deployments/                  # 運用関連
│   ├── docker-compose.yml        # 構成定義
│   ├── .env.example              # 環境変数テンプレート
│   ├── lnav_formats/             # lnav用ログ定義ファイル
│   │   └── eap_aka_log.json      # ホストの ~/.lnav/formats/ に配置
│   └── logs_on_host/             # ログ保存先 (実行時に生成)
│
├── docs/                         # ドキュメント
│   ├── D-01〜D-12                # 設計仕様書群
│   ├── B-01〜B-02                # セットアップ手引書群
│   ├── E-01〜E-03                # 開発環境・規約ドキュメント群
│   ├── T-01〜T-04                # テスト戦略・仕様書群
│   ├── subdocs/                  # 補助ドキュメント（テスト詳細仕様、実装ガイド等）
│   └── testresults/              # テスト結果保存先
│
└── go.work                       # Go Workspace設定
```

## 7. `docker-compose.yml` (最終定義)

> **注記:** `docker-compose.yml` は `deployments/` ディレクトリに配置されるため、`build:` パスは `../apps/...` となる。D-08「インフラ設定・運用設計書」の記載と整合している。

```yaml
x-logging: &fluent-bit-logging
  driver: fluentd
  options:
    fluentd-address: localhost:24224
    fluentd-async: "true"
    fluentd-buffer-limit: "1048576"
    tag: "{{.Name}}.logs"

x-timezone: &timezone
  TZ: Asia/Tokyo

services:
  # ==========================================================================
  # Logic Layer
  # ==========================================================================
  auth-server:
    build: ../apps/auth-server
    ports:
      - "1812:1812/udp"
    environment:
      <<: *timezone
      REDIS_HOST: valkey
      REDIS_PORT: "6379"
      REDIS_PASS: ${VALKEY_PASSWORD}
      VECTOR_API_URL: http://vector-gateway:8080
      RADIUS_SECRET: ${RADIUS_SECRET}
      LOG_MASK_IMSI: ${LOG_MASK_IMSI:-true}
    depends_on:
      valkey:
        condition: service_healthy
      vector-gateway:
        condition: service_started
    healthcheck:
      test: ["CMD", "pgrep", "-x", "auth-server"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s
    restart: always
    logging: *fluent-bit-logging

  acct-server:
    build: ../apps/acct-server
    ports:
      - "1813:1813/udp"
    environment:
      <<: *timezone
      REDIS_HOST: valkey
      REDIS_PORT: "6379"
      REDIS_PASS: ${VALKEY_PASSWORD}
      RADIUS_SECRET: ${RADIUS_SECRET}
      LOG_MASK_IMSI: ${LOG_MASK_IMSI:-true}
    depends_on:
      valkey:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "pgrep", "-x", "acct-server"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s
    restart: always
    logging: *fluent-bit-logging

  vector-gateway:
    build: ../apps/vector-gateway
    expose:
      - "8080"
    environment:
      <<: *timezone
      VECTOR_GATEWAY_MODE: ${VECTOR_GATEWAY_MODE:-gateway}
      VECTOR_GATEWAY_INTERNAL_URL: http://vector-api:8080
      VECTOR_GATEWAY_PLMN_MAP: ${VECTOR_GATEWAY_PLMN_MAP:-}
      VECTOR_GATEWAY_INTERNAL_TIMEOUT: ${VECTOR_GATEWAY_INTERNAL_TIMEOUT:-5s}
      LOG_MASK_IMSI: ${LOG_MASK_IMSI:-true}
    depends_on:
      vector-api:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "curl", "-fsS", "http://localhost:8080/health"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s
    restart: always
    logging: *fluent-bit-logging

  vector-api:
    build: ../apps/vector-api
    expose:
      - "8080"
    environment:
      <<: *timezone
      REDIS_HOST: valkey
      REDIS_PORT: "6379"
      REDIS_PASS: ${VALKEY_PASSWORD}
      LOG_MASK_IMSI: ${LOG_MASK_IMSI:-true}
    depends_on:
      valkey:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "curl", "-fsS", "http://localhost:8080/health"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s
    restart: always
    logging: *fluent-bit-logging

  # ==========================================================================
  # Data Layer
  # ==========================================================================
  valkey:
    image: valkey/valkey:9.0
    ports:
      - "127.0.0.1:6379:6379"
    volumes:
      - valkey_data:/data
    command: >
      valkey-server
      --appendonly yes
      --appendfsync everysec
      --maxmemory 512mb
      --maxmemory-policy noeviction
      --requirepass ${VALKEY_PASSWORD}
    environment:
      <<: *timezone
    healthcheck:
      test: ["CMD", "valkey-cli", "-a", "${VALKEY_PASSWORD}", "ping"]
      interval: 10s
      timeout: 3s
      retries: 3
      start_period: 5s
    restart: always
    logging: *fluent-bit-logging

  # ==========================================================================
  # Logging Layer
  # ==========================================================================
  fluent-bit:
    image: fluent/fluent-bit:3.0
    ports:
      - "24224:24224"
      - "24224:24224/udp"
    volumes:
      - ../configs/fluent-bit/fluent-bit.conf:/fluent-bit/etc/fluent-bit.conf:ro
      - ../configs/fluent-bit/parsers.conf:/fluent-bit/etc/parsers.conf:ro
      - ./logs_on_host:/output_logs
    environment:
      <<: *timezone
    restart: always

volumes:
  valkey_data:
```

## 8. Vector Gateway環境変数

Vector Gatewayの動作を制御する環境変数。

| 環境変数 | 必須 | デフォルト | 説明 |
|---------|------|-----------|------|
| `VECTOR_GATEWAY_MODE` | No | `gateway` | 動作モード。`gateway`: PLMNルーティング、`passthrough`: 全て内部APIへ転送 |
| `VECTOR_GATEWAY_INTERNAL_URL` | Yes | - | 内部Vector APIのURL |
| `VECTOR_GATEWAY_PLMN_MAP` | No | 空文字列 | PLMNマッピング（`PLMN:ID,PLMN:ID,...` 形式） |
| `VECTOR_GATEWAY_INTERNAL_TIMEOUT` | No | `5s` | 内部API呼び出しタイムアウト |

### PLMNマッピング形式

```
# 形式: "PLMN:ID,PLMN:ID,..."
# PLMN: MCC+MNC結合形式（5-6桁）
# ID: 接続方式ID（2桁）
#   - 00: Vector API（内部）
#   - 01-99: 外部API（将来実装、現時点では501エラー）

# 例: ドコモ→01、ソフトバンク→01に振り分け
VECTOR_GATEWAY_PLMN_MAP="44010:01,44020:01"

# 空設定: 全てVector API（ID:00相当）へ
VECTOR_GATEWAY_PLMN_MAP=""
```

## 9. ホストPC セットアップ手順概要

1. **OSインストール:** Ubuntu Server (LTS) をインストール。
2. **SSH設定変更:** `/etc/ssh/sshd_config` を編集し、Portを `10022` に変更、rootログイン無効化。
3. **FW設定:** `ufw allow 10022/tcp`, `ufw allow 1812/udp`, `ufw allow 1813/udp` を実行後に有効化。
4. **ログ解析ツール導入:** `sudo apt install lnav`。リポジトリ内の `lnav_formats/eap_aka_log.json` を `~/.lnav/formats/` に配置。
5. **Docker導入:** 公式スクリプトでインストールし、ユーザーを `docker` グループに追加。
6. **デプロイ:**
   - リポジトリを `git clone`。
   - `.env` ファイルを作成し、シークレット（DBパスワード等）を設定。
   - `docker compose up -d` で起動。
7. **ツール配置:**
   - 開発機でビルドした `admin-tui` バイナリを `scp -P 10022` で転送。
   - 実行時は `export VALKEY_PASSWORD=...; ./admin-tui` で起動 (`os.Getenv()` で読み込まれる)。

------

## 改訂履歴

| 版数 | 日付 | 内容 |
|------|------|------|
| r1 | - | 初版 |
| r2 | - | パッケージ利用マップ追加、docker-compose.yml最終化 |
| r3 | 2025-12-30 | Valkey接続管理セクション追加（接続タイムアウト設定、再接続方針）、EAP-AKA'対応明記 |
| r4 | 2026-01-05 | Vector Gateway追加：アーキテクチャ図更新、ノード一覧にVector Gateway追加、パッケージ利用マップにGateway列追加、コンテナ構成更新、リポジトリ構成にvector-gateway追加、docker-compose.yml更新、Vector Gateway環境変数セクション追加 |
| r5 | 2026-01-20 | パッケージ利用マップ更新: google/uuidの用途説明をAuth Server（生成）とAcct Server（パース検証）で区別 |
| r6 | 2026-01-26 | インフラ基盤統一: Valkeyバージョン7.2→9.0、Fluent Bitイメージをfluent/fluent-bit:3.0に統一、LOG_OUTPUT_PATH環境変数削除、RADIUS_SECRET_KEY→RADIUS_SECRETに統一、docker-compose.ymlをD-08と整合化（x-logging/x-timezone追加、LOG_MASK_IMSI追加） |
| r7 | 2026-01-27 | docker-compose.yml相対パス修正: セクション7のbuildパスを`./apps/...`→`../apps/...`に修正（D-08との整合性確保） |
| r8 | 2026-01-27 | Fluent Bit設定ファイルの相対パス修正（`./configs/...`→`../configs/...`、deployments基準でD-08と整合） |
| r9 | 2026-02-16 | 実装コードとの不整合20件を修正: データモデル注記追加（RadiusClient/Subscriber/Policy）、パッケージ利用マップ更新（envconfig/resty/tcell/mock/miniredis）、Valkeyキースキーマ概要追加、コンテナ構成詳細更新（healthcheck/fluent-bitポート/テストベクターモード/healthエンドポイント）、docker-compose.ymlを実装ファイルと完全同期、セットアップ手順のenvconfig記述修正 |