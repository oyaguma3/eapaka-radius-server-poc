# eapaka-radius-server-poc

[![Go](https://img.shields.io/badge/Go-1.25.5-00ADD8?logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

## 概要

EAP-AKA/AKA' RADIUS サーバーの PoC (Proof of Concept) 環境です。
Wi-Fi 認証 (WPA2/WPA3-Enterprise) 向けの RADIUS 認証・課金機能、AKA 認証ベクター生成機能、および管理用 TUI アプリケーションを提供します。

## アーキテクチャ

```
                          ┌─────────────────────────────────────────┐
                          │           Docker Compose 環境            │
                          │                                         │
  Wi-Fi AP ──UDP:1812──▶  │  ┌──────────────┐   ┌────────────────┐  │
  (認証)                  │  │ auth-server   │──▶│ vector-gateway │  │
                          │  │ (RADIUS認証   │   │ (ベクター生成  │  │
                          │  │  + EAP制御)   │   │  ルーティング) │  │
                          │  └──────┬───────┘   └───────┬────────┘  │
                          │         │                    │           │
  Wi-Fi AP ──UDP:1813──▶  │  ┌──────┴───────┐   ┌───────┴────────┐  │
  (課金)                  │  │ acct-server   │   │  vector-api    │  │
                          │  │ (RADIUS課金)  │   │ (Milenage計算  │  │
                          │  └──────┬───────┘   │  + SQN管理)    │  │
                          │         │            └───────┬────────┘  │
                          │         │                    │           │
                          │         ▼                    ▼           │
                          │  ┌──────────────────────────────────┐   │
                          │  │            valkey                 │   │
                          │  │         (データストア)             │   │
                          │  └──────────────────────────────────┘   │
                          │                                         │
                          │  ┌──────────────┐   ┌────────────────┐  │
                          │  │  admin-tui    │   │   fluent-bit   │  │
                          │  │ (管理用TUI)   │   │  (ログ収集)    │  │
                          │  └──────────────┘   └────────────────┘  │
                          └─────────────────────────────────────────┘
```

| コンポーネント | 役割 | ポート |
|---|---|---|
| **auth-server** | RADIUS 認証 + EAP-AKA/AKA' ステートマシン制御 | UDP 1812 |
| **acct-server** | RADIUS 課金 (Accounting) | UDP 1813 |
| **vector-gateway** | 認証ベクター生成リクエストのルーティング | HTTP 8080 |
| **vector-api** | Milenage アルゴリズム計算 + SQN 管理 | HTTP 8081 |
| **admin-tui** | 加入者・セッション管理用ターミナル UI | - |
| **valkey** | データストア (加入者情報・セッション等) | 6379 |
| **fluent-bit** | ログ収集・転送 | 24224 |

## 技術スタック

- **言語:** Go 1.25.5
- **データストア:** Valkey 9.0
- **コンテナ:** Docker Compose
- **ログ収集:** Fluent Bit 4.2
- **ロギング:** log/slog (構造化ログ)
- **RADIUS:** [layeh.com/radius](https://pkg.go.dev/layeh.com/radius)
- **EAP-AKA:** [go-eapaka](https://github.com/oyaguma3/go-eapaka)
- **Circuit Breaker:** sony/gobreaker

## リポジトリ構成

```
eapaka-radius-server-poc/
├── apps/
│   ├── auth-server/        # RADIUS 認証サーバー
│   ├── acct-server/        # RADIUS 課金サーバー
│   ├── vector-gateway/     # ベクター生成ルーティング
│   ├── vector-api/         # Milenage 計算 + SQN 管理
│   └── admin-tui/          # 管理用 TUI アプリケーション
├── pkg/                    # 共通ライブラリ
│   ├── apperr/             # エラー定義
│   ├── httputil/           # HTTP ユーティリティ
│   ├── logging/            # ログ設定
│   ├── model/              # 共通モデル
│   └── valkey/             # Valkey クライアント
├── configs/
│   └── fluent-bit/         # Fluent Bit 設定 (YAML)
├── deployments/
│   ├── docker-compose.yml  # Docker Compose 定義
│   ├── .env.example        # 環境変数テンプレート
│   └── lnav_formats/       # lnav ログフォーマット定義
├── docs/                   # 設計・運用ドキュメント (23ファイル)
├── go.work                 # Go Workspace 定義
└── go.work.sum
```

## セットアップ

### 前提条件

- Docker & Docker Compose
- Go 1.25.5 (ローカル開発時)

### 起動手順

```bash
# リポジトリをクローン
git clone https://github.com/oyaguma3/eapaka-radius-server-poc.git
cd eapaka-radius-server-poc

# 環境変数を設定
cd deployments
cp .env.example .env
# .env を編集して各値を設定

# Docker Compose を起動
docker compose up -d
```

### 主要な環境変数

| 変数名 | 必須 | 説明 |
|---|---|---|
| `VALKEY_PASSWORD` | Yes | Valkey 接続パスワード |
| `RADIUS_SECRET` | Yes | RADIUS 共有シークレット |
| `VECTOR_GATEWAY_MODE` | No | 動作モード (`gateway` / `passthrough`) |
| `LOG_MASK_IMSI` | No | IMSI マスキング有効化 (デフォルト: `true`) |
| `TEST_VECTOR_ENABLED` | No | テストベクターモード (デフォルト: `false`、本番では無効のこと) |

詳細は `deployments/.env.example` を参照してください。

## テスト実行

```bash
# Go Workspace ルートから全モジュールのテストを実行
go test ./...

# カバレッジ付きで実行
go test -cover ./...

# 特定のアプリケーションのみ
go test ./apps/auth-server/...
```

テスト規模: 96 テストファイル、876 テストケース (T-02 単体テスト仕様書準拠)

## 実装状況

全コンポーネントの実装が完了しています。

| コンポーネント | ステータス |
|---|---|
| auth-server | 完了 |
| acct-server | 完了 |
| vector-gateway | 完了 |
| vector-api | 完了 |
| admin-tui | 完了 |
| インフラ (Docker Compose / Fluent Bit / Valkey) | 完了 |

## ドキュメント

`docs/` 配下に各種設計・運用ドキュメントがあります（全22件作成済み / 5件未作成）。

### 設計ドキュメント (12件)

| No. | ドキュメント名 | 内容 |
|---|---|---|
| D-01 | 設計仕様書 | 全体アーキテクチャ・リポジトリ構成 |
| D-02 | Valkey データ設計仕様書 | データストア設計・キー設計 |
| D-03 | Vector-API IF 定義書 | API インターフェース・EAP-AKA ステートマシン |
| D-04 | ログ仕様設計書 | ログフォーマット・event_id 定義 |
| D-05 | Admin TUI 詳細設計書【前半】 | 画面設計・バリデーション |
| D-06 | エラーハンドリング詳細設計書 | 異常系処理・Circuit Breaker |
| D-07 | Admin TUI 詳細設計書【後半】 | モニタリング画面・ヘルプ |
| D-08 | インフラ設定・運用設計書 | Docker Compose・Valkey・Fluent Bit 設定 |
| D-09 | Auth Server 詳細設計書 | 認証サーバー詳細設計 |
| D-10 | Acct Server 詳細設計書 | 課金サーバー詳細設計 |
| D-11 | Vector API 詳細設計書 | Milenage 計算・SQN 管理 |
| D-12 | Vector Gateway 詳細設計書 | PLMN ルーティング・外部 API 連携 |

### 開発ドキュメント (3件)

| No. | ドキュメント名 | 内容 |
|---|---|---|
| E-01 | 開発環境セットアップガイド | Go 環境構築・ローカル開発手順 |
| E-02 | コーディング規約（簡易版） | 命名規則・パッケージ構成 |
| E-03 | 共通ライブラリ(pkg)設計書 | pkg 配置方針・各パッケージ設計 |

### テストドキュメント (4件)

| No. | ドキュメント名 | 内容 |
|---|---|---|
| T-01 | テスト戦略書 | テスト方針・カバレッジ目標 |
| T-02 | 単体テスト仕様書 | 全 876 テストケース定義 |
| T-03 | 結合テスト仕様書 | コンポーネント間連携テスト |
| T-04 | E2E テスト仕様書 | 実機テスト・擬似 E2E (計 11 シナリオ) |

### 構築・デプロイドキュメント (2件)

| No. | ドキュメント名 | 内容 |
|---|---|---|
| B-01 | ホストOS構築手順書 | Ubuntu Server インストール・セキュリティ設定 |
| B-02 | アプリケーションデプロイ手順書 | Docker Compose 起動・logrotate・バックアップ |

### 運用ドキュメント (1件作成済み / 5件未作成)

| No. | ドキュメント名 | ステータス |
|---|---|---|
| O-01 | 操作ガイド | 未作成 |
| O-02 | ポリシー設定ガイド | 未作成 |
| O-03 | 障害対応手順書 | 未作成 |
| O-04 | バックアップ・リストア手順書 | 未作成 |
| O-05 | ログ解析ガイド | 完了 (r3) |
| O-06 | 顧客説明ガイドライン資料 | 未作成 |

詳細は [ドキュメント一覧](docs/EAP-AKA_RADIUS_PoC環境_ドキュメント一覧_r23.md) を参照してください。

## ライセンス

[MIT License](LICENSE)
