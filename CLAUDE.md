# EAP-AKA RADIUS PoC プロジェクト

## 概要
EAP-AKA/AKA' RADIUSサーバーのPoC環境の開発。Wi-Fi認証（WPA2/WPA3-Enterprise）向け。
RADIUS認証機能・課金機能と、AKA認証ベクター生成機能と、管理用TUIアプリケーションを持つ。

## 設計面の概要
- ドキュメント「D-01_ミニPC版_EAP-AKA_RADIUS_PoC環境_設計仕様書」を参照。

## リポジトリ構成
- ドキュメント「D-01_ミニPC版_EAP-AKA_RADIUS_PoC環境_設計仕様書」の `6. 開発リポジトリ構成` を参照。

## Go言語の外部パッケージ
- ドキュメント 「D-01_ミニPC版_EAP-AKA_RADIUS_PoC環境_設計仕様書」を参照。
- Go言語で開発するノードの各設計ドキュメントに記載された外部パッケージ依存も参照すること。

## コーディング規約
- ドキュメント E-02「コーディング規約（簡易版）」を参照。

## ドキュメント参照
- ドキュメント一覧は `docs/` 配下の「EAP-AKA_RADIUS_PoC環境_ドキュメント一覧」です。
- 設計・開発などの各ドキュメントは `docs/` 配下にあります。

## テスト戦略
- 本プロジェクトのテスト戦略は、ドキュメント「T-01_テスト戦略書」を参照。
- 実装時はテストファイルを作成し、テストカバレッジ80%以上となるよう検討する。

## 利用可能なテストツール
- delve
- mockgen
- radclient
- eapaka_test
  - GitHub Repository : https://github.com/oyaguma3/eapaka_test

## セッション管理
- マルチフェーズ計画を実装する際は、次のフェーズに移る前に各フェーズのテストを完了させることを優先する。
- セッション制限に近づいている場合は、新しいフェーズを開始するのではなく、現在のフェーズのテストをパスさせることに集中する。

## Go開発ガイドライン
- go.mod の依存関係を変更する前に、変更をリセットする可能性のある git フックやリンターがないか確認する。
- 変更後は `git diff go.mod` で変更が保持されていることを確認する。
- Goパッケージを再構築する際は、実装コードを書く前に `make build` を実行して循環インポートを早期にチェックする（Go Workspace構成のため `go build ./...` は使用不可）。

## ビルド・静的解析
- Go Workspace (`go.work`) 構成のため `./...` は使用不可。Makefileで各モジュールパスを明示指定している。
- 主要Makefileターゲット:
  - `make build` — 全モジュールのビルド
  - `make test` — 全モジュールのテスト実行
  - `make test-cover` — カバレッジ付きテスト
  - `make test-race` — レースコンディション検出テスト（`CGO_ENABLED=1` が必要。WSL環境ではデフォルト無効、GitHub Actions環境では有効）
  - `make fmt` — コードフォーマット
  - `make vet` — go vet 実行
  - `make lint` — golangci-lint 実行
  - `make clean` — ビルド成果物の削除
- golangci-lint は **v2** 形式（`version: "2"` 必須）
  - v2での注意: `gosimple` は staticcheck に統合済み、`typecheck` は linter ではない
  - テストファイル除外は `linters.exclusions.rules` で `path: _test\.go`
- 設定ファイル: `.golangci.yml`, `Makefile`

## CI/CD（GitHub Actions）
- 設定ファイル: `.github/workflows/ci.yml`
- トリガー: `main` / `develop` への push および PR
- パイプライン: Checkout → Setup Go → Build → Vet → Lint → Test(race)
- `golangci/golangci-lint-action` は **v7** を使用（v6は golangci-lint v2非対応）
- Go バージョンは `go.work` の `go` ディレクティブから自動取得

## ブランチ戦略・PRワークフロー
- `main`（安定版） ← `develop`（統合） ← `feature/*`（個別改善）
- feature → develop: `gh pr merge <N> --squash --delete-branch`
- develop → main: Create a merge commit（履歴保持）
- コミットメッセージ prefix: `feat:`, `fix:`, `refactor:`, `test:`, `docs:`, `chore:`

## 共有パッケージ（pkg/）
- `pkg/logging.MaskIMSI` — IMSIマスキング（D-04仕様: 先頭6桁+末尾1桁）
- `pkg/httputil.ProblemDetail` — RFC 7807準拠エラーレスポンス
- 各appで重複実装せず、pkgをimportして利用すること
