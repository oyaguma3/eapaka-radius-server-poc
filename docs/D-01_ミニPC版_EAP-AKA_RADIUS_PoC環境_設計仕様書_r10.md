# D-01 ミニPC版 EAP-AKA RADIUS PoC環境 設計仕様書 (r10)

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
|     |       |       |                                                   |
|     |       |       +--- (ID:01 / HTTPS + mTLS) ------------------------+---> [ aka-only-server ] ※任意
|     |       |                                                           |     (外部サーバー。同一ホストでは
|     |       | (ID:00 / HTTP / JSON / Trace ID Propagation)              |      共有ネットワーク aka-av 経由)
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

> **注記（aka-only-server、任意）:** aka-only-server（https://github.com/oyaguma3/aka-only-server ）は、3GPP TS 29.503 Nudm_UEAU GenerateAv ベースの API で AKA 認証ベクターを払い出す外部サーバーであり、本リポジトリの構成要素ではない。Vector Gateway の接続方式ID `01` として、`VECTOR_GATEWAY_PLMN_MAP` で `01` を指定した PLMN の加入者についてだけ利用する（`VECTOR_GATEWAY_AKAONLY_URL` を設定した場合のみ有効）。接続は mTLS（証明書ピン留め）が既定で、平文HTTP は同一ホスト内に限る。同一ホストで動かす場合は `deployments/docker-compose.aka-av.yml` を重ね、vector-gateway を aka-only-server 側の共有ネットワーク（既定名 `aka-av`）に参加させる。詳細は D-12「Vector Gateway 詳細設計書」4.5節・5章を参照。

## 3. Go実装ノード詳細と採用パッケージ

本システムを構成する5つのGoアプリケーションの役割と、使用するライブラリのマッピングです。

### 3.1 実装ノード一覧

| **ノード名**           | **ディレクトリ**       | **役割**                                | **機能要件・処理フロー**                                     |
| ---------------------- | ---------------------- | --------------------------------------- | ------------------------------------------------------------ |
| **1. Auth Server**     | `apps/auth-server`     | **[RADIUS認証 & EAP制御]** 認証の司令塔 | 1. **設定読込:** `envconfig` で環境変数ロード。Shared SecretはValkey優先、環境変数はフォールバックとして使用。`EAP_AKA_PRIME_NETWORK_NAME` 環境変数でEAP-AKA'のネットワーク名を設定（デフォルト: "WLAN"）。 2. **受信:** UDP 1812で受信。 3. **Trace ID:** 受信時にUUID生成。これを`trace_id`ログ、Valkeyキー(`eap:{UUID}`)、RADIUS `State`属性として統一利用。 4. **API連携:** `resty` + `gobreaker` (Circuit Breaker) でVector Gatewayへ計算リクエスト (Header `X-Trace-ID` 付与)。 5. **EAP制御:** `go-eapaka` 利用。 6. **保存:** `go-redis` 利用。 7. **ログ:** `slog` で構造化出力。 |
| **2. Acct Server**     | `apps/acct-server`     | **[RADIUS課金]** 利用実績の記録         | 1. **設定読込:** `envconfig` でロード。SecretはAuth Server同様の優先順位。 2. **受信:** UDP 1813で受信。 3. **検証:** Shared SecretによるMessage-Authenticator検証。 4. **ログ:** 課金ログの構造化出力。 5. **更新:** Valkeyセッション状態更新。 |
| **3. Vector Gateway**  | `apps/vector-gateway`  | **[ルーティング]** ベクター取得先振分け | 1. **設定読込:** `envconfig` でロード。 2. **API提供:** REST API (`POST /api/v1/vector`)。Auth Serverとの互換性維持。`GET /health` エンドポイントでヘルスチェック応答を提供。 3. **ルーティング:** PLMNベースでバックエンド選択。接続方式ID `00`: 内部Vector API（PLMN未一致・passthroughモードの既定先）、`01`: 外部の aka-only-server（`VECTOR_GATEWAY_AKAONLY_URL` 設定時のみ。mTLSまたは平文HTTP、GenerateAv形式との変換を行う）、`02`〜`99`: 将来実装（501）。 4. **Trace ID:** Header `X-Trace-ID` を読み取り、内部APIへ伝搬（aka-only-serverへはベストエフォート）。 5. **ログ:** `slog` で構造化出力。 |
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

Auth ServerからVector Gateway、Vector GatewayからVector API（および aka-only-server）への接続設定。

| 接続 | タイムアウト | 備考 |
|------|------------|------|
| Auth Server → Vector Gateway | 5秒 | 既存のVector API接続設定を継承 |
| Vector Gateway → Vector API | 5秒 | `VECTOR_GATEWAY_INTERNAL_TIMEOUT` で設定 |
| Vector Gateway → aka-only-server（接続方式01、任意） | 5秒 | `VECTOR_GATEWAY_AKAONLY_TIMEOUT` で設定。HTTPS + mTLS（既定）または平文HTTP |

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
  - 接続方式01で別ホストの aka-only-server に接続する場合、vector-gateway からの送信（Outbound、既定 8443/tcp）が必要となる（受信側の許可は不要）。

### シークレット管理 (環境変数とDBの併用)

以下の機密情報は `.env` ファイルで管理し、各アプリへ注入します（コンテナアプリは `envconfig` 経由、Admin TUIは `os.Getenv()` で直接取得）。

- `VALKEY_PASSWORD`: DBアクセス用パスワード（必須）
- `RADIUS_SECRET`: **デフォルト共有シークレット**（フォールバック用）
  - **優先順位:**
    1. Valkey `client:{IP}` 内の `secret` を使用 (本番/登録済みクライアント)
    2. 上記がない場合、環境変数 `RADIUS_SECRET` を使用 (テスト/未登録クライアント)
- aka-only-server 接続用の証明書（接続方式01使用時のみ）: クライアント証明書（秘密鍵を含む `av-client.pem`）と AV 用サーバー証明書（`av-server.pem`）を `deployments/certs/` に置き、vector-gateway の `/certs` に読み取り専用でマウントする。`deployments/certs/` の中身は `.gitignore` で Git 管理外（`.gitkeep` のみコミット）。

## 5. コンテナ構成詳細 (Docker Compose)

| **No.** | **サービス名**      | **公開ポート**       | **役割**                                                     |
| ------- | ------------------- | -------------------- | ------------------------------------------------------------ |
| **1**   | **auth-server**     | 1812/udp             | 認証ロジック。`VECTOR_API_URL` と `RADIUS_SECRET` (Fallback) を環境変数で受ける。 |
| **2**   | **acct-server**     | 1813/udp             | 課金ロジック。`RADIUS_SECRET` (Fallback) を環境変数で受ける。 |
| **3**   | **vector-gateway**  | なし                 | ルーティング。PLMNベースでバックエンド選択（`00`: vector-api、`01`: aka-only-server）。外部公開なし。`GET /health` でヘルスチェック応答。`./certs` を `/certs` に読み取り専用マウント（aka-only-server 接続用証明書）。 |
| **4**   | **vector-api**      | なし                 | 内部API。外部公開なし。`TEST_VECTOR_ENABLED` 環境変数でテストベクターモードを有効化可能。`GET /health` でヘルスチェック応答。 |
| **5**   | **valkey**          | 127.0.0.1:6379       | DB。ホスト(TUI)用にlocalhostのみバインド。`--requirepass` 有効化。 |
| **6**   | **fluent-bit**      | 24224/tcp, 24224/udp | ログ収集。出力先は `/output_logs` 固定。                     |

> **注記（オーバーレイ）:** aka-only-server を同一ホストで動かす場合は、`deployments/docker-compose.aka-av.yml` を重ねて起動する（`docker compose -f docker-compose.yml -f docker-compose.aka-av.yml up -d`）。このオーバーレイは vector-gateway だけを aka-only-server の compose が作る共有ネットワーク（external、名前 `${AKA_SHARED_NETWORK:-aka-av}`）に参加させる。共有ネットワークは aka-only-server 側が作るため、先に aka-only-server を起動しておく。別ホストの aka-only-server に接続する場合はオーバーレイを重ねない。

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
│   ├── docker-compose.aka-av.yml # aka-only-server 同一ホスト接続用オーバーレイ（任意）
│   ├── .env.example              # 環境変数テンプレート
│   ├── certs/                    # aka-only-server 接続用証明書（中身はGit管理外、.gitkeepのみ）
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

aka-only-server を同一ホストで動かす場合に重ねるオーバーレイ `deployments/docker-compose.aka-av.yml`（任意）:

```yaml
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

## 8. Vector Gateway環境変数

Vector Gatewayの動作を制御する環境変数。

| 環境変数 | 必須 | デフォルト | 説明 |
|---------|------|-----------|------|
| `VECTOR_GATEWAY_MODE` | No | `gateway` | 動作モード。`gateway`: PLMNルーティング、`passthrough`: 全て内部APIへ転送 |
| `VECTOR_GATEWAY_INTERNAL_URL` | Yes | - | 内部Vector APIのURL |
| `VECTOR_GATEWAY_PLMN_MAP` | No | 空文字列 | PLMNマッピング（`PLMN:ID,PLMN:ID,...` 形式） |
| `VECTOR_GATEWAY_INTERNAL_TIMEOUT` | No | `5s` | 内部API呼び出しタイムアウト |
| `VECTOR_GATEWAY_AKAONLY_URL` | No | 空文字列 | aka-only-server（接続方式01）のベースURL。例: `https://aka-only-server:8443`。`https://` で mTLS、`http://` で平文HTTP（同一ホスト限定）。空なら `01` は無効（`01` に向けたPLMNは501） |
| `VECTOR_GATEWAY_AKAONLY_CLIENT_CERT` | https時Yes | 空文字列 | クライアント証明書のPEM（コンテナ内パス、例: `/certs/av-client.pem`）。証明書と秘密鍵が1ファイルでよい |
| `VECTOR_GATEWAY_AKAONLY_CLIENT_KEY` | No | 空文字列 | 秘密鍵のPEM。空なら `CLIENT_CERT` と同じファイルから読む |
| `VECTOR_GATEWAY_AKAONLY_SERVER_CERT` | https時Yes | 空文字列 | aka-only-server の AV 用サーバー証明書のPEM（例: `/certs/av-server.pem`）。これだけを信頼する |
| `VECTOR_GATEWAY_AKAONLY_TIMEOUT` | No | `5s` | aka-only-server 呼び出しタイムアウト |
| `AKA_SHARED_NETWORK` | No | `aka-av` | `docker-compose.aka-av.yml` で参加する共有ネットワーク名（compose の変数） |

> **注記:** `VECTOR_GATEWAY_AKAONLY_*` の設定が不正な場合（URL のスキーム不正、https で証明書未指定、証明書ファイルが読めない等）、vector-gateway は起動しない。起動時の検証・ログの詳細は D-12 5.3節を参照。

### PLMNマッピング形式

```
# 形式: "PLMN:ID,PLMN:ID,..."
# PLMN: MCC+MNC結合形式（5-6桁）
# ID: 接続方式ID（2桁）
#   - 00: Vector API（内部。マップに一致しないPLMNもこちら）
#   - 01: aka-only-server（VECTOR_GATEWAY_AKAONLY_URL の設定が必要。未設定なら501）
#   - 02-99: 将来実装（指定すると501）

# 例: PLMN 44010 の加入者を aka-only-server に振り分け
VECTOR_GATEWAY_PLMN_MAP="44010:01"

# 空設定: 全てVector API（ID:00相当）へ
VECTOR_GATEWAY_PLMN_MAP=""
```

> **注記:** 振り分けは PLMN 単位である。`01` に向けた PLMN の加入者は、Ki/OPc を aka-only-server に、認可ポリシーと RADIUS クライアントを従来どおり Admin TUI（PoC の Valkey）に登録する必要がある。

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
| r10 | 2026-10-04 | Vector Gateway 接続方式ID `01`（外部の aka-only-server、mTLS、任意）対応: 2章構成図に aka-only-server を追加し注記追加、3.1 Vector Gateway の処理フロー更新、3.5に aka-only-server への接続設定追加、4章に Outbound 接続と証明書管理を追記、5章に vector-gateway の証明書マウントと docker-compose.aka-av.yml の注記追加、6章リポジトリ構成に docker-compose.aka-av.yml と certs/ を追加、7章 docker-compose.yml の vector-gateway に `VECTOR_GATEWAY_AKAONLY_*` と certs ボリュームを追加しオーバーレイを掲載、8章に `VECTOR_GATEWAY_AKAONLY_*`・`AKA_SHARED_NETWORK` と接続方式ID `01` を追加 |
