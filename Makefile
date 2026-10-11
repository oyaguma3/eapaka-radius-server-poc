# EAP-AKA RADIUS PoC — Go Workspace 一括操作 Makefile
#
# Go Workspace (go.work) 環境のため、各モジュールパスを明示的に指定する。

# Go Workspace 内のモジュールパス
MODULES := ./pkg/... \
	./apps/auth-server/... \
	./apps/acct-server/... \
	./apps/vector-gateway/... \
	./apps/vector-api/... \
	./apps/provisioning-api/... \
	./apps/admin-tui/...

# Go Workspace 内のモジュールのディレクトリ（govulncheck はモジュールごとに実行する）
MODULE_DIRS := pkg apps/auth-server apps/acct-server apps/vector-gateway apps/vector-api apps/provisioning-api apps/admin-tui

# govulncheck の版（CI と同じ）
GOVULNCHECK := golang.org/x/vuln/cmd/govulncheck@v1.9.0

.PHONY: build test test-cover test-race fmt vet lint vulncheck clean

## build: 全モジュールをビルド
build:
	go build $(MODULES)

## test: 全モジュールのテスト実行
test:
	go test $(MODULES)

## test-cover: カバレッジ付きテスト実行（cover.out を生成）
test-cover:
	go test -coverprofile=cover.out $(MODULES)
	go tool cover -func=cover.out | tail -1

## test-race: データ競合検出付きテスト実行
test-race:
	go test -race $(MODULES)

## fmt: 全ソースコードのフォーマット
fmt:
	gofmt -w .

## vet: 全モジュールの静的解析（go vet）
vet:
	go vet $(MODULES)

## lint: golangci-lint による静的解析
lint:
	golangci-lint run $(MODULES)

## vulncheck: 依存と標準ライブラリの既知の脆弱性の検査（govulncheck。go.work の直下では動かないのでモジュールごと）
vulncheck:
	@set -e; for d in $(MODULE_DIRS); do echo "== $$d"; (cd $$d && go run $(GOVULNCHECK) ./...); done

## clean: ビルド成果物・カバレッジファイルの削除
clean:
	rm -f cover.out coverage.out
	go clean $(MODULES)
