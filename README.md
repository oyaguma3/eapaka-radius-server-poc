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
- **ログ収集:** Fluent Bit
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
│   └── fluent-bit/         # Fluent Bit 設定
├── deployments/
│   └── docker-compose.yml  # Docker Compose 定義
├── docs/                   # 設計・運用ドキュメント
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

# 環境変数を設定 (必要に応じて .env ファイルを作成)
# deployments/ 配下で Docker Compose を起動
cd deployments
docker compose up -d
```

## テスト実行

```bash
# 全モジュールのテストを実行
go test ./...

# カバレッジ付きで実行
go test -cover ./...
```

## ドキュメント

`docs/` 配下に各種設計・運用ドキュメントがあります。

| ドキュメント | 内容 |
|---|---|
| D-01 設計仕様書 | 全体アーキテクチャ・リポジトリ構成 |
| D-02 Valkey データ設計仕様書 | データストア設計 |
| D-03 Vector-API IF 定義書 | API インターフェース・EAP-AKA ステートマシン |
| D-09 Auth Server 詳細設計書 | 認証サーバー詳細設計 |
| D-10 Acct Server 詳細設計書 | 課金サーバー詳細設計 |
| E-01 開発環境セットアップガイド | 開発環境構築手順 |
| T-01 テスト戦略書 | テスト方針・カバレッジ目標 |

詳細は [ドキュメント一覧](docs/EAP-AKA_RADIUS_PoC環境_ドキュメント一覧_r16.md) を参照してください。

## ライセンス

[MIT License](LICENSE)
