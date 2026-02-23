# D-05 Admin TUI 詳細設計書【前半】(r9)

## 1. 概要

### 1.1 目的

本ドキュメントは、EAP-AKA RADIUS PoC環境における管理コンソール「Admin TUI」のデータ操作機能について詳細設計を定義する。

### 1.2 スコープ

**本書【前半】で扱う範囲：**
- 画面構成・遷移
- マスタデータのCRUD操作仕様
- 入力バリデーション
- CSVインポート/エクスポート

**本書【後半】（別途作成）で扱う範囲：**
- モニタリング画面仕様
- セッション一覧・検索機能

### 1.3 IMSI表示方針

**Admin TUIにおけるIMSI表示・記録は常に生値とする。**

| 項目 | 方針 |
|------|------|
| 画面表示 | IMSI全桁を表示（マスキングなし） |
| 監査ログ | IMSI全桁を記録（マスキングなし） |
| 環境変数 | `LOG_MASK_IMSI` は参照しない |

**設計意図:**
- 管理者が加入者を一意に識別できること
- 監査証跡としてIMSIを確実に追跡可能とすること
- Auth/Acct等のネットワークコンポーネントとは異なり、Admin TUIは管理操作に特化しているため、プライバシーよりも運用性を優先

> **注記:** ネットワークコンポーネント（Auth Server, Acct Server, Vector Gateway, Vector API）のログは `LOG_MASK_IMSI` 環境変数でマスキングを制御する。詳細はD-04「ログ仕様設計書」を参照。


### 1.4 動作環境

| 項目 | 仕様 |
|------|------|
| 実行場所 | ホストOS上（コンテナ外） |
| 起動方法 | SSH接続後、ターミナルから直接実行 |
| 依存 | Valkey（127.0.0.1:6379）への接続 |
| 認証 | 環境変数 `VALKEY_PASSWORD` によるDB認証 |
| UIライブラリ | `rivo/tview` |
| 表示言語 | 英語のみ |

### 1.5 管理対象データ

Valkeyデータ設計仕様書に基づく、以下のマスタデータを管理する。

| カテゴリ | Valkeyキー | データ型 | 用途 |
|----------|-----------|---------|------|
| 加入者情報 | `sub:{IMSI}` | Hash | EAP-AKA認証用のSIM鍵情報 |
| RADIUSクライアント | `client:{IP}` | Hash | 接続元NAS/APの共有秘密鍵 |
| 認可ポリシー | `policy:{IMSI}` | Hash | 認証成功後の接続許可ルール |

全マスタデータは **サーバーコンポーネント（Vector API / Auth Server / Acct Server）との互換性を確保するため、Hash形式** で保存する。

#### 加入者データのHash形式

| フィールド | 型 | 説明 |
|-----------|-----|------|
| `ki` | String (32文字Hex) | 秘密鍵 |
| `opc` | String (32文字Hex) | オペレータ定数 |
| `amf` | String (4文字Hex) | 認証管理フィールド |
| `sqn` | String (12文字Hex) | シーケンス番号 |
| `created_at` | String (RFC3339) | 作成日時 |

**Valkeyコマンド例：**
```
HSET "sub:440101234567890" "ki" "0123456789ABCDEF0123456789ABCDEF" "opc" "FEDCBA9876543210FEDCBA9876543210" "amf" "8000" "sqn" "000000000000" "created_at" "2024-01-01T00:00:00Z"
```

**注記：** IMSIはキーに含まれるため、Hashフィールドには含めない。Vector APIは `HGetAll` コマンドで加入者データを読み取る。

#### RADIUSクライアントデータのHash形式

| フィールド | 型 | 説明 |
|-----------|-----|------|
| `secret` | String | 共有シークレット |
| `name` | String | クライアント名称 |
| `vendor` | String | ベンダー名 |

**Valkeyコマンド例：**
```
HSET "client:192.168.1.100" "secret" "mysecretkey" "name" "AP-Floor1" "vendor" "cisco"
```

**注記：** IPアドレスはキーに含まれるため、Hashフィールドには含めない。Auth Server / Acct Serverは `HGet` コマンドで共有シークレットを読み取る。

#### 認可ポリシーのデータ形式

| フィールド | 型 | 説明 |
|-----------|-----|------|
| `default` | String | デフォルトアクション（`allow` または `deny`） |
| `rules` | String (JSON配列) | ポリシールールの配列 |

**Valkeyコマンド例：**
```
HSET "policy:440101234567890" "default" "deny" "rules" '[{"ssid":"CORP-WIFI","action":"allow","time_min":"09:00","time_max":"18:00"},{"ssid":"*","action":"deny","time_min":"","time_max":""}]'
```

**注記：** Auth Serverは `HGETALL` コマンドでポリシーを読み取る。

### 1.6 ドキュメント成果物

実装完了後に以下のドキュメントを作成する。

| ドキュメント | 内容 | 作成時期 |
|-------------|------|---------|
| `README.md` | ビルド・起動方法、環境変数一覧 | 実装完了後 |
| `docs/user-guide.md` | 操作ガイド、画面説明 | 実装完了後 |
| `docs/policy-config-guide.md` | 認可ポリシー設定ガイド（Default allow/denyの違い、ルール適用例、ベストプラクティス） | 実装完了後 |

---

## 2. 画面構成

### 2.1 画面一覧

```
[M] Main Menu
 │
 ├─[S] Subscriber Management（加入者管理）
 │   ├─[S1] Subscriber List（加入者一覧）
 │   ├─[S2] Add Subscriber（加入者登録）
 │   ├─[S3] Edit Subscriber（加入者編集）
 │   └─[S4] Delete Confirmation（削除確認）
 │
 ├─[C] RADIUS Client Management（RADIUSクライアント管理）
 │   ├─[C1] Client List（クライアント一覧）
 │   ├─[C2] Add Client（クライアント登録）
 │   ├─[C3] Edit Client（クライアント編集）
 │   └─[C4] Delete Confirmation（削除確認）
 │
 ├─[P] Authorization Policy Management（認可ポリシー管理）
 │   ├─[P1] Policy List（ポリシー一覧）
 │   ├─[P2] Add Policy（ポリシー登録）
 │   ├─[P3] Edit Policy（ポリシー編集）
 │   └─[P4] Delete Confirmation（削除確認）
 │
 ├─[I] Import/Export（インポート/エクスポート）
 │   ├─[I1] Import（インポート画面）
 │   └─[I2] Export（エクスポート画面）
 │
 ├─[O] Monitoring（モニタリング）（後半で定義）
 │
 └─[Q] Exit Confirmation（終了確認）
```

### 2.2 画面遷移図

```
                    ┌─────────────────┐
                    │   起動時処理     │
                    │ (DB接続確認)    │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
          ┌─────────┤  [M] Main Menu  ├─────────┐
          │         └────────┬────────┘         │
          │                  │                  │
    ┌─────▼─────┐     ┌─────▼─────┐     ┌─────▼─────┐
    │[S] Subscr.│     │[C] Client │     │[P] Policy │
    │ Management│     │ Management│     │ Management│
    └─────┬─────┘     └─────┬─────┘     └─────┬─────┘
          │                  │                  │
    ┌─────▼─────┐     ┌─────▼─────┐     ┌─────▼─────┐
    │[S1] List  │     │[C1] List  │     │[P1] List  │
    └───┬───────┘     └───┬───────┘     └───┬───────┘
        │ 選択/操作       │ 選択/操作       │ 選択/操作
        ▼                 ▼                 ▼
    [S2] Add          [C2] Add          [P2] Add
    [S3] Edit         [C3] Edit         [P3] Edit
    [S4] Delete       [C4] Delete       [P4] Delete
```

---

## 3. 共通仕様

### 3.1 キーバインド（グローバル）

全画面で有効なキーバインド。

| キー | 動作 | 備考 |
|------|------|------|
| `Esc` | 前の画面に戻る / キャンセル | メインメニューでは終了確認表示 |
| `Ctrl+Q` | アプリケーション終了 | 確認なしで即座に終了 |
| `F1` / `?` | ヘルプ表示 | 現在画面のキーバインド一覧をモーダル表示 |
| `Tab` | 次のフォーカス要素へ移動 | - |
| `Shift+Tab` | 前のフォーカス要素へ移動 | - |

### 3.2 キーバインド（一覧画面共通）

| キー | 動作 |
|------|------|
| `↑` / `↓` | カーソル上下移動 |
| `PgUp` / `PgDn` | ページ上下移動 |
| `Enter` | 選択項目の編集画面へ |
| `F2` / `n` | 新規登録画面へ |
| `F3` / `e` | 選択項目の編集画面へ |
| `F4` / `d` | 削除確認ダイアログ表示 |
| `F5` / `r` | 一覧を再読み込み |
| `F6` / `/` | フィルタ入力ダイアログ表示 |

### 3.3 キーバインド（フォーム画面共通）

| キー | 動作 |
|------|------|
| `Save` ボタン | 保存実行（tview.Form 標準ボタン） |
| `Esc` | キャンセル（一覧画面へ戻る） |

### 3.4 確認ダイアログ仕様

破壊的操作や重要な変更時に表示する。

| 種別 | トリガー | メッセージ例 | 選択肢 |
|------|---------|-------------|--------|
| **削除確認** | 削除操作時 | `Delete IMSI: 440101234567890?`<br>`This action cannot be undone.` | `[Delete] [Cancel]` |
| **変更破棄確認** | 編集中にEsc | `Changes have not been saved.`<br>`Discard and go back?` | `[Discard] [Cancel]` |
| **終了確認** | メインメニューでEsc | `Exit Admin TUI?` | `[Exit] [Cancel]` |
| **上書き確認** | 既存キーで登録試行 | `IMSI: 440101234567890 already exists.`<br>`Overwrite?` | `[Overwrite] [Cancel]` |

### 3.5 Default "allow" 設定時の警告ダイアログ

ポリシー保存時にDefaultが "allow" に設定されている場合に表示する。

```
┌─────────────────────────────────────────────────────────┐
│  Warning: Setting Default to "allow"                    │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  When Default is "allow", authentication will succeed   │
│  for any SSID not explicitly listed in rules, as long   │
│  as the subscriber's credentials (Ki/OPc) are valid.    │
│                                                         │
│                                                         │
│  This may allow unintended network access.              │
│                                                         │
│  Are you sure you want to save with Default = "allow"?  │
│                                                         │
├─────────────────────────────────────────────────────────┤
│  [Save Anyway]                              [Cancel]    │
└─────────────────────────────────────────────────────────┘
```

### 3.6 メッセージ表示

画面下部にステータスバーを配置し、操作結果を表示する。

| 種別 | 表示色 | 表示時間 | 例 |
|------|-------|---------|-----|
| 成功 | 緑 | 3秒 | `✓ Subscriber created (IMSI: 440101234567890)` |
| エラー | 赤 | 手動クリアまで | `✗ Failed: Invalid IMSI format` |
| 警告 | 黄 | 5秒 | `⚠ SQN is still at initial value` |
| 情報 | 白 | 3秒 | `ℹ Loaded 5 subscribers` |

### 3.7 フィルタ機能仕様

| 項目 | 仕様 |
|------|------|
| 起動方法 | `/` キー押下でフィルタ入力欄にフォーカス |
| 対象カラム | 一覧表示中の全カラム（部分一致） |
| マッチング | 大文字小文字を区別しない（case-insensitive） |
| 動作 | 入力文字列を含む行のみ表示（リアルタイム絞り込み） |
| クリア | `Esc` でフィルタ解除、全件表示に戻る |
| 件数表示 | `Showing: 25 / 125` 形式で絞り込み件数を表示 |

フィルタはクライアント側（バッファ済みデータ）に対して適用する。フィルタ結果が不足する場合は、追加で `SCAN` を実行してデータを補充する。

### 3.8 ページネーション仕様

| 項目 | 仕様 |
|------|------|
| データ取得 | `SCAN` コマンドで100件ずつ取得 |
| 表示 | 1ページあたり50件 |
| ナビゲーション | `←` 前ページ / `→` 次ページ |
| UI形式 | `[Prev] Page 1/25 [Next]` |
| フィルタ併用時 | 取得済みデータに適用、必要に応じて追加取得 |

### 3.9 ページライフサイクル管理

#### 画面遷移時のページクリーンアップパターン

tviewはtcell上で動作し、tcellは**変更されたセルのみ更新**する差分レンダリングを行う。ページ遷移時に、削除されたページの内容がターミナルバッファに残存し、新ページのウィジェットが描画しない領域が更新されない問題がある。

この問題を回避するため、以下のページ遷移パターンを採用する。

**遷移元ページを破棄する場合（リスト画面→メニュー、フォーム→リスト等）：**

```go
// 1. ページを非表示にする
app.HidePage("current-page")
// 2. ページを削除する
app.RemovePage("current-page")
// 3. 遷移先ページに切り替える
app.SwitchToPage("target-page")
```

**SwitchToPage / RemovePage のSync()呼び出し：**

`SwitchToPage()` および `RemovePage()` は内部で `tcell.Screen.Sync()` を呼び出し、全セルの再描画を強制する。これにより、前画面の残存描画を確実にクリアする。

#### InputCapture内でのQueueUpdateDraw

tview v0.42.0の `QueueUpdateDraw` は内部で `Draw()` を呼び、`Draw()` はmutexロックを取得する。`InputCapture`（イベントハンドラ）処理中は既にmutexがロックされているため、直接呼び出すとデッドロックが発生する。

**対策：** `InputCapture` 内から `QueueUpdateDraw` を呼ぶ場合は、goroutineでラップする。

```go
// InputCapture 内での正しい呼び出しパターン
go func() {
    s.app.QueueUpdateDraw(func() {
        if err := s.Refresh(context.Background()); err != nil {
            // エラー処理
        }
    })
}()
return nil
```

#### Import/Export完了時のページクリーンアップ

Import/Export画面の完了コールバック（`SetOnComplete`）でも、キャンセルコールバック（`SetOnCancel`）と同じく `HidePage` → `RemovePage` → `SwitchToPage` のパターンを適用する。`SwitchToPage` のみでは前画面のTUI要素が残存する。

#### form.Clear(true) 後のInputCapture再登録

`tview.Form.Clear(true)` は `true` パラメータによりInputCaptureもクリアする。Import/Export画面で完了後にフォームを再構成する際は、InputCapture（ESCキーハンドラ等）を再登録し、フォーカスも再設定する必要がある。

```go
// form.Clear(true) 後の再登録パターン
s.form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
    if event.Key() == tcell.KeyEsc {
        s.handleCancel()
        return nil
    }
    return event
})
s.app.SetFocus(s.form)
```

### 3.10 tview Table の Selectable 状態管理

#### 問題: 全セル NotSelectable 時の無限ループ

tview v0.42.0 の `Table` は `SetSelectable(true, false)` で行選択を有効にすると、`Table.Draw()` 内で選択可能なセルを探すループが動作する。**全セルが `NotSelectable` の場合、`selectedRow` が `rowCount` 以上に押し出される。**

この状態で移動キー（↑↓、PgUp/PgDn等）が押されると、`Table.InputHandler()` 内部の `forward()` / `backwards()` 関数が範囲外の `selectedRow` を終了条件（`finalRow`）として受け取る。これらの関数内では `row` が `0` 〜 `rowCount-1` で周回するため、範囲外の `finalRow` に到達できず **無限ループ（CPU 100%、UIフリーズ）** が発生する。

#### 対策: 結果0件時に SetSelectable(false, false) を設定

テーブルに選択可能なデータ行がない場合は `SetSelectable(false, false)` を設定し、データ行がある場合は `SetSelectable(true, false)` に戻す。

```go
// session_list.go / session_detail.go 共通パターン
if len(items) == 0 {
    table.SetSelectable(false, false)
    table.SetCell(1, 0, tview.NewTableCell("No data").
        SetSelectable(false))
} else {
    table.SetSelectable(true, false)
    // データ行を追加...
    table.Select(1, 0)
}
```

**注記:** ヘッダー行（固定行）のセルには常に `SetSelectable(false)` を設定すること。ヘッダーのみでデータ行がない場合に `SetSelectable(true, false)` のままだと上記の無限ループが発生する。

### 3.11 非同期データ取得パターン（goroutine + QueueUpdateDraw）

#### QueueUpdateDraw 内でのネットワーク I/O 回避

tview v0.42.0 の `QueueUpdateDraw` は内部で `QueueUpdate` を呼び、コールバック完了まで呼び出し元 goroutine をブロックする（同期的）。コールバックは tview イベントループ内で実行されるため、**コールバック内でネットワーク I/O（Redis クエリ等）を行うとイベントループがブロックされ、UI が応答不能になる。**

**対策:** ネットワーク I/O を goroutine 内で先に実行し、UI 更新のみ `QueueUpdateDraw` で行う。

```go
// NG: ネットワーク I/O が QueueUpdateDraw 内
go func() {
    s.app.QueueUpdateDraw(func() {
        data, err := store.Fetch(ctx)  // イベントループをブロック
        // UI更新...
    })
}()

// OK: ネットワーク I/O を goroutine 内で先に実行
go func() {
    data, err := store.Fetch(ctx)  // goroutine内で実行
    s.app.QueueUpdateDraw(func() {
        // UI更新のみ
        if err != nil {
            statusBar.ShowError(err.Error())
        } else {
            render(data)
        }
    })
}()
```

**注記:** 既存のマスタ一覧画面（Subscriber List 等）では `QueueUpdateDraw` 内で `Load()` を呼ぶパターンを使用しているが、これはデータ量が少なく Redis 応答が高速なため問題が顕在化していない。Session Detail の検索のように SCAN フォールバックを伴う処理では、上記の分離パターンを採用すること。

### 3.12 テキスト切り詰め仕様

カラム幅を超えるテキストは末尾に "..." を付加して切り詰める。

```go
func truncate(s string, maxLen int) string {
    if len(s) <= maxLen {
        return s
    }
    if maxLen <= 3 {
        return s[:maxLen]
    }
    return s[:maxLen-3] + "..."
}
```

---

## 4. 画面詳細仕様

### 4.1 メインメニュー [M]

#### レイアウト

tview.List（ShowSecondaryText=true）を使用。ボーダータイトルに「Admin TUI - Main Menu」を表示。

```
┌ Admin TUI - Main Menu ─────────────────────────────────────┐
│                                                             │
│ (1) Subscriber Management                                   │
│     Manage subscriber data (IMSI, Ki, OPc, etc.)            │
│                                                             │
│ (2) RADIUS Client Management                                │
│     Manage RADIUS client (NAS) configurations               │
│                                                             │
│ (3) Authorization Policy Management                         │
│     Manage access control policies for subscribers          │
│                                                             │
│ (4) Import/Export                                            │
│     Import or export data as CSV files                      │
│                                                             │
│ (5) Monitoring                                               │
│     View statistics and active sessions                     │
│                                                             │
│ (q) Exit                                                     │
│     Exit the application                                    │
│                                                             │
└─────────────────────────────────────────────────────────────┘
F1:Help  |  q:Back/Quit  |  Ctrl+Q:Exit
```

**備考:** フッターバー（`F1:Help | q:Back/Quit | Ctrl+Q:Exit`）は全画面共通で、ボーダー外の画面最下部に常時表示される。

#### 操作

| キー | 動作 |
|------|------|
| `1` | 加入者管理画面へ |
| `2` | RADIUSクライアント管理画面へ |
| `3` | 認可ポリシー管理画面へ |
| `4` | インポート/エクスポート画面へ |
| `5` | モニタリング画面へ（後半で定義） |
| `q` / `Esc` | 終了確認ダイアログ表示 |

---

### 4.2 加入者管理

#### 4.2.1 加入者一覧 [S1]

##### レイアウト

ボーダータイトルに「Subscriber List」+件数・ページ情報を表示。フィルタ適用時は `(Filter: "...")` を付加。

```
┌ Subscriber List 1-9 of 9 (Page 1/1) ──────────────────────────────────────────┐
│ IMSI              Ki              OPc              AMF    SQN            Created │
│ 001010000000000   465B5CE8...A6BC CD63CB71...2BAF  B9B9   000000000001   2024-01-01│
│ 001010000000001   465B5CE8...A6BC CD63CB71...2BAF  B9B9   000000000021   2024-01-01│
│ 001010000000003   465B5CE8...A6BC CD63CB71...2BAF  8000   0000000000c1   2024-01-01│
│! 001010000000007  465B5CE8...A6BC CD63CB71...2BAF  B9B9   ff9bb4d0b687   2024-01-01│
│! 441991234567890  465B5CE8...A6BC CD63CB71...2BAF  B9B9   FF9BB4D0B607   2024-01-01│
│  :                :               :                :      :              :         │
└────────────────────────────────────────────────────────────────────────────────────┘
F1:Help  |  q:Back/Quit  |  Ctrl+Q:Exit
```

フィルタ適用時のタイトル例: `Subscriber List (Filter: "00101") 1-6 of 6 (Page 1/1)`

##### 表示項目

| カラム | 内容 | Expansion | 色 | 備考 |
|--------|------|-----------|-----|------|
| IMSI | 加入者識別番号 | 1 | White（ポリシー未設定時はYellow/Orange） | 行頭に "!" 表示でポリシー未設定を示す |
| Ki | 認証鍵（マスク表示） | 1 | Gray | 先頭8文字...末尾4文字（例: `465B5CE8...A6BC`） |
| OPc | オペレータ鍵（マスク表示） | 1 | Gray | 先頭8文字...末尾4文字（例: `CD63CB71...2BAF`） |
| AMF | 認証管理フィールド | 1 | White | 4桁Hex |
| SQN | シーケンス番号 | 1 | White | 12桁Hex |
| Created | 作成日（先頭10文字） | 1 | Gray | YYYY-MM-DD形式 |

##### ポリシー未設定加入者の視覚的識別

| 条件 | 表示 |
|------|------|
| ポリシーあり | 通常表示（White） |
| ポリシーなし | 行全体がYellow/Orange表示、IMSIカラムの先頭に `!` プレフィックス |

**実装：**

```go
type SubscriberListItem struct {
    IMSI      string
    HasPolicy bool
    CreatedAt time.Time
}

func getRowStyle(item SubscriberListItem) tcell.Style {
    if !item.HasPolicy {
        return tcell.StyleDefault.Foreground(tcell.ColorYellow)
    }
    return tcell.StyleDefault
}

// ポリシー存在チェックの一括処理（N+1問題回避）
func checkPoliciesExist(imsiList []string) map[string]bool {
    pipe := rdb.Pipeline()
    cmds := make(map[string]*redis.IntCmd)
    for _, imsi := range imsiList {
        cmds[imsi] = pipe.Exists(ctx, "policy:"+imsi)
    }
    pipe.Exec(ctx)
    
    result := make(map[string]bool)
    for imsi, cmd := range cmds {
        result[imsi] = cmd.Val() > 0
    }
    return result
}
```

**注記：** Ki/OPcは一覧でマスク表示する（先頭8文字+末尾4文字を表示し、中間部分を `...` で省略）。AMF/SQNも一覧に表示し、加入者情報の概要を一目で確認可能とする。

#### 4.2.2 加入者登録 [S2] / 編集 [S3]

##### レイアウト

tview.Form を centered() ヘルパーで画面中央にダイアログ表示する。背景に一覧テーブルが透過表示される。新規作成時のタイトルは「Create Subscriber」、編集時は「Edit Subscriber」。

```
              ┌ Create Subscriber ──────────────────────────────┐
              │                                                   │
              │  IMSI    [001010000000008       ]                 │
              │  Ki      [0123456789ABCDEF0123456789ABCDEF     ]  │
              │  OPc     [FEDCBA9876543210FEDCBA9876543210     ]  │
              │  AMF     [8000      ]                             │
              │  SQN     [000000000000   ]                        │
              │                                                   │
              │          < Save >  < Cancel >                     │
              │                                                   │
              └───────────────────────────────────────────────────┘
```

編集時: タイトル「Edit Subscriber」、IMSIフィールドは無効化（グレーアウト、編集不可）。

##### フィールド定義

| フィールド | 必須 | 初期値 | 編集時の挙動 |
|-----------|------|-------|-------------|
| IMSI | Yes | 空 | 編集時は変更不可（読取専用表示） |
| Ki | Yes | 空 | 表示・編集可能 |
| OPc | Yes | 空 | 表示・編集可能 |
| AMF | Yes | `8000` | 表示・編集可能 |
| SQN | No | `000000000000` | 表示・編集可能（警告表示付き） |

##### SQN手動編集時の警告

ユーザーがSQNフィールドを編集しようとした際に以下の警告を表示する。

tview.Modal を使用。ボーダータイトル「SQN Modification Warning」（Yellow）。Edit Subscriber ダイアログの上にオーバーレイ表示。

```
        ┌ SQN Modification Warning ──────────────────────────────┐
        │                                                          │
        │                    ⚠WARNING ⚠                           │
        │                                                          │
        │  Modifying the SQN value may cause                       │
        │  authentication failures.                                │
        │                                                          │
        │  Are you sure you want to change the SQN                 │
        │  from                                                    │
        │    000000000000 to 000000000010?                          │
        │                                                          │
        │           < Continue >     < Cancel >                     │
        │                                                          │
        └──────────────────────────────────────────────────────────┘
```

---

### 4.3 RADIUSクライアント管理

#### 4.3.1 クライアント一覧 [C1]

##### レイアウト

ボーダータイトルに「RADIUS Client List」+件数・ページ情報を表示。

```
┌ RADIUS Client List 1-6 of 6 (Page 1/1) ────────────────────────────────┐
│ IP Address        Name               Secret          Vendor             │
│ 10.0.0.100        E2E-NAS            e2****as        generic            │
│ 127.0.0.1         Localhost           TE****23        generic            │
│ 172.19.0.1        DockerGateway       TE****23        generic            │
│ 192.168.1.10      TestAP-01           te****p1        generic            │
│ 192.168.10.1      Customer01          TE****23        generic            │
│ 192.168.30.1      testClient          ab****mn        none               │
└─────────────────────────────────────────────────────────────────────────┘
F1:Help  |  q:Back/Quit  |  Ctrl+Q:Exit
```

##### 表示項目

| カラム | 内容 | Expansion | 色 | 備考 |
|--------|------|-----------|-----|------|
| IP Address | クライアントIPアドレス | 1 | White | - |
| Name | クライアント名称 | 1 | White | 超過時は "..." で切り詰め |
| Secret | 共有シークレット（マスク表示） | 1 | Gray | 先頭2文字+****+末尾2文字（例: `e2****as`） |
| Vendor | ベンダー名 | 1 | Gray | 空の場合は `none` と表示 |

**注記：** Shared Secretは一覧でマスク表示する（先頭2文字+****+末尾2文字）。

#### 4.3.2 クライアント登録 [C2] / 編集 [C3]

##### レイアウト

tview.Form を centered() ヘルパーで画面中央にダイアログ表示する。新規作成時のタイトルは「Create RADIUS Client」、編集時は「Edit RADIUS Client」。フォーム内のフィールド数が多いため、フォーカスが下部に移動するとフォーム内がスクロールし、上部フィールドが隠れてSave/Cancelボタンが表示される動作となる。

```
              ┌ Create RADIUS Client ───────────────────────────┐
              │                                                   │
              │  IP Address  [255.255.255.255    ]                │
              │  Secret      [ABCDEFGHIJKLMNOPQRSTUVWXYZ       ]  │
              │  Name        [TestClient                       ]  │
              │  Vendor      [unknown                          ]  │
              │                                                   │
              │          < Save >  < Cancel >                     │
              │                                                   │
              └───────────────────────────────────────────────────┘
```

編集時: タイトル「Edit RADIUS Client」、IP Addressフィールドは無効化（グレーアウト、編集不可）。

##### フィールド定義

| フィールド | 必須 | 初期値 | 編集時の挙動 |
|-----------|------|-------|-------------|
| IP Address | Yes | 空 | 編集時は変更不可（読取専用表示） |
| Secret | Yes | 空 | 表示・編集可能 |
| Name | No | 空 | 表示・編集可能 |
| Vendor | No | 空 | 表示・編集可能 |

---

### 4.4 認可ポリシー管理

#### 4.4.1 ポリシー一覧 [P1]

##### レイアウト

ボーダータイトルに「Authorization Policy List」+件数・ページ情報を表示。

```
┌ Authorization Policy List 1-9 of 9 (Page 1/1) ─────────────────────────┐
│ IMSI              Default   Rules                                        │
│ 001010000000000   allow     No rules                                     │
│ 001010000000001   allow     No rules                                     │
│ 001010000000002   deny      No rules                                     │
│ 001010000000003   deny      1 rule                                       │
│ 001010000000004   deny      2 rules                                      │
│ 001010000000005   deny      1 rule                                       │
│  :                :         :                                            │
└──────────────────────────────────────────────────────────────────────────┘
F1:Help  |  q:Back/Quit  |  Ctrl+Q:Exit
```

**表示色:** Default列は `allow` = Yellow/Orange、`deny` = Green。Rules列は `No rules` / `1 rule` / `N rules` / `10+ rules` 形式で表示。

#### 4.4.2 ポリシー登録 [P2] / 編集 [P3]

##### レイアウト

ボーダータイトル「Policy Details」を表示。FlexRow で上部（Formエリア）と下部（Rules List）に分割し、それぞれ独立したボーダー付きBoxとして描画。Default Action は tview.DropDown で `deny` / `allow` を選択。Rules リストのインデックスは1始まり。

```
┌ Policy Details ──────────────────────────────────────────────────┐
│                                                                    │
│  IMSI             001010000000004        (disabled)                 │
│  Default Action   [deny ▼]                                        │
│                                                                    │
│  < Add Rule >  < Save >  < Cancel >                               │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
┌ Rules ─────────────────────────────────────────────────────────────┐
│ [1] NAS: Customer01                                                 │
│ SSIDs: TESTSSID-01, TESTSSID-02 | VLAN: 10 | Timeout: 7200s        │
│ [2] NAS: Customer02                                                 │
│ SSIDs: Guest | VLAN: 20 | Timeout: 1800s                           │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
F1:Help  |  q:Back/Quit  |  Ctrl+Q:Exit
```

**備考:** Rules リストは tview.List を使用（メインテキスト: `[N] NAS: {nasID}`、サブテキスト: `SSIDs: ... | VLAN: ... | Timeout: ...s`、サブテキスト色: Green）。

##### ルール編集サブダイアログ

centered(form, width=60, height=15) で Policy Details の上にオーバーレイ表示。ボーダー色は Teal/Cyan。

新規追加時のタイトル: 「Add Rule」、ボタン: OK / Cancel
編集時のタイトル: 「Edit Rule」、ボタン: OK / Delete / Cancel

```
       ┌ Edit Rule ───────────────────────────────────────────┐
       │                                                        │
       │  NAS ID          [Customer01                        ]  │
       │  Allowed SSIDs   [TESTSSID-01,TESTSSID-02           ]  │
       │  VLAN ID         [10        ]                          │
       │  Session Timeout [7200      ]                          │
       │                                                        │
       │       < OK >  < Delete >  < Cancel >                   │
       │                                                        │
       └────────────────────────────────────────────────────────┘
```

新規追加時の Session Timeout 初期値は `0`。

##### ポリシーフォームのキーバインド

| キー | 動作 |
|------|------|
| `F6` | フォーム部分とルールリスト間のフォーカス切替 |
| `Ctrl+S` | 保存実行 |
| `Esc` | キャンセル |

**注記：** `Tab` キーはtviewのフォーム内ナビゲーション（フィールド間移動）で使用されるため、フォーム/ルールリスト間のフォーカス切替には `F6` キーを使用する。

##### フィールド定義（ポリシー本体）

| フィールド | 必須 | 初期値 | 編集時の挙動 |
|-----------|------|-------|-------------|
| IMSI | Yes | 空 | 編集時は変更不可 |
| Default | Yes | `deny` | ラジオボタン選択 |
| Rules | Yes | 空配列 | サブリストで管理 |

**注記：** Defaultを "allow" に設定して保存する場合、警告ダイアログを表示（セクション3.5参照）。

##### フィールド定義（ルール）

| フィールド | 必須 | 初期値 | 幅 | 型 |
|-----------|------|-------|-----|-----|
| NAS ID | Yes | 空 | 40 | String（NAS IPアドレスまたはNAS ID） |
| Allowed SSIDs | Yes | 空 | 40 | String（カンマ区切り、例: `SSID1,SSID2`） |
| VLAN ID | No | 空 | 10 | String |
| Session Timeout | No | `0` | 10 | String（秒数） |

**注記：** D-02 Valkeyデータ設計仕様書のPolicyRule構造に準拠する。NAS IDにはNAS IPアドレスまたは識別名を指定する。

---

## 5. 入力バリデーション仕様

### 5.1 バリデーションルール一覧

| 対象 | フィールド | ルール | エラーメッセージ |
|------|-----------|--------|-----------------|
| Subscriber | IMSI | 15桁の数字 `/^[0-9]{15}$/` | `IMSI must be 15 digits` |
| Subscriber | Ki | 32桁のHex `/^[0-9A-Fa-f]{32}$/` | `Ki must be 32 hex characters` |
| Subscriber | OPc | 32桁のHex `/^[0-9A-Fa-f]{32}$/` | `OPc must be 32 hex characters` |
| Subscriber | AMF | 4桁のHex `/^[0-9A-Fa-f]{4}$/` | `AMF must be 4 hex characters` |
| Subscriber | SQN | 12桁のHex `/^[0-9A-Fa-f]{12}$/` | `SQN must be 12 hex characters` |
| Client | IP Address | 有効なIPv4 | `Enter a valid IPv4 address` |
| Client | Secret | 1-128文字（ASCII印字可能文字） | `Secret must be 1-128 characters` |
| Client | Name | 0-64文字 | `Name must be 64 characters or less` |
| Client | Vendor | 0-32文字（英数字とハイフン） | `Vendor must be alphanumeric or hyphen` |
| Policy | IMSI | （Subscriberと同じ） | （同上） |
| Policy | Default | `allow` または `deny` | - |
| Rule | NAS ID | 1-64文字 | `NAS ID is required` |
| Rule | Allowed SSIDs | 1文字以上（カンマ区切り） | `Allowed SSIDs is required` |
| Rule | VLAN ID | 空 または 数値文字列 | `VLAN ID must be numeric` |
| Rule | Session Timeout | 空 または 0以上の整数 | `Session Timeout must be a non-negative integer` |

### 5.2 バリデーションタイミング

| タイミング | 動作 |
|-----------|------|
| **リアルタイム** | 入力文字種の制限（Hexフィールドは0-9,A-F,a-fのみ受付） |
| **フォーカス離脱時** | 桁数・形式チェック、エラー時はフィールド下に赤字表示 |
| **保存時** | 全フィールドの再検証、必須チェック、重複チェック |

### 5.3 Hexフィールドの入力補助

- 小文字入力は自動的に大文字に変換して表示
- 保存時は大文字に正規化してValkey保存

---

## 6. インポート/エクスポート仕様

### 6.1 対応形式

CSV形式（UTF-8、カンマ区切り、ヘッダ行あり）

### 6.2 加入者CSV仕様

#### ファイル形式

```csv
imsi,ki,opc,amf,sqn
440101234567890,0123456789ABCDEF0123456789ABCDEF,FEDCBA9876543210FEDCBA9876543210,8000,000000000000
440101234567891,ABCDEF0123456789ABCDEF0123456789,0123456789ABCDEFFEDCBA9876543210,8000,000000000001
```

#### インポート動作

| 条件 | 動作 |
|------|------|
| 新規IMSI | 新規登録 |
| 既存IMSI | スキップ（ログ出力）※オプションで上書きモード選択可 |
| バリデーションエラー | 該当行スキップ、エラーログ出力 |

#### エクスポート動作

- 全加入者を出力
- ファイル名デフォルト: `subscribers_YYYYMMDD_HHMMSS.csv`

### 6.3 RADIUSクライアントCSV仕様

```csv
ip,secret,name,vendor
192.168.1.100,mysecret123,AP-Floor1,cisco
192.168.1.101,anothersecret,AP-Floor2,cisco
```

### 6.4 認可ポリシーCSV仕様

```csv
imsi,default,rules_json
440101234567890,deny,"[{""ssid"":""CORP-WIFI"",""action"":""allow"",""time_min"":""09:00"",""time_max"":""18:00""},{""ssid"":""*"",""action"":""deny"",""time_min"":"""",""time_max"":""""}]"
```

**注記：** `rules_json` フィールドはJSON文字列をダブルクォートでエスケープしてCSV格納する。

### 6.5 インポート画面 [I1]

FlexRow で上部（Formエリア）と下部（Result表示エリア）に分割。Data Type は tview.DropDown（Subscribers / RADIUS Clients / Policies）。インポート完了後にフォームが状態遷移する。

#### 初期状態（Import Data）

```
┌ Import Data ─────────────────────────────────────────────────────┐
│                                                                    │
│  Data Type    [Subscribers      ▼]                                │
│  File Path    [/home/admin/import.csv                          ]  │
│                                                                    │
│  < Validate >  < Import >  < Cancel >                             │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
┌ Import Result ─────────────────────────────────────────────────────┐
│                                                                     │
│  (結果表示エリア — スクロール可能)                                   │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
F1:Help  |  q:Back/Quit  |  Ctrl+Q:Exit
```

Validate 実行後は結果エリアのタイトルが「Validation Result」に変化し、`Validation passed!`（緑色）+ `Records to import: N` を表示。

#### 完了後の状態遷移（Import Completed）

インポート完了後、フォームタイトルが「Import Completed」に変わり、ボタンが Done / Import More に切り替わる。結果エリアには `Import completed!`（緑色）+ `Imported: N {type}` を表示。

```
┌ Import Completed ────────────────────────────────────────────────┐
│                                                                    │
│  < Done >  < Import More >                                        │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
┌ Import Result ─────────────────────────────────────────────────────┐
│                                                                     │
│  Import completed!                                                  │
│  Imported: 15 subscribers                                           │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
F1:Help  |  q:Back/Quit  |  Ctrl+Q:Exit
```

### 6.6 エクスポート画面 [I2]

FlexRow で上部（Formエリア）と下部（Result表示エリア）に分割。Data Type は tview.DropDown。エクスポート完了後にフォームが状態遷移する。

#### 初期状態（Export Data）

```
┌ Export Data ─────────────────────────────────────────────────────┐
│                                                                    │
│  Data Type     [Subscribers      ▼]                               │
│  Output File   [/home/admin/subscriber_export_20260223.txt     ]  │
│                                                                    │
│  < Export >  < Cancel >                                           │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
┌ Export Result ─────────────────────────────────────────────────────┐
│                                                                     │
│  (結果表示エリア)                                                    │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
F1:Help  |  q:Back/Quit  |  Ctrl+Q:Exit
```

#### 完了後の状態遷移（Export Completed）

エクスポート完了後、フォームタイトルが「Export Completed」に変わり、ボタンが Done / Export More に切り替わる。結果エリアには `Export completed!`（緑色）+ 件数・ファイルパスを表示。画面最下部のステータスバーにも緑色で成功メッセージが表示される。

```
┌ Export Completed ────────────────────────────────────────────────┐
│                                                                    │
│  < Done >  < Export More >                                        │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
┌ Export Result ─────────────────────────────────────────────────────┐
│                                                                     │
│  Export completed!                                                  │
│  Exported: 24 subscribers                                           │
│  File: /home/admin/subscriber_export_20260223.txt                   │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
✓ Exported 24 subscribers to /home/admin/subscriber_export_20260223.txt
```

### 6.7 インポートロールバック（2フェーズインポート）

データ整合性を確保するため、2フェーズ方式でインポートを行う。

```
Phase 1: Validation（ドライラン）
  - CSVを全行読み込み
  - 全行のバリデーションを実行
  - エラーがあれば中断、エラー一覧を表示

Phase 2: Commit（実データ投入）
  - バリデーション通過後のみ実行
  - Valkey MULTI/EXEC（トランザクション）で一括投入
  - 途中エラー時は DISCARD でロールバック
```

**実装：**

```go
func importSubscribers(records []SubscriberRecord) error {
    // Phase 1: Validation
    var errors []ValidationError
    for i, rec := range records {
        if err := validate(rec); err != nil {
            errors = append(errors, ValidationError{Line: i+1, Err: err})
        }
    }
    if len(errors) > 0 {
        return &ImportValidationError{Errors: errors}
    }

    // Phase 2: Commit with transaction
    pipe := rdb.TxPipeline()
    for _, rec := range records {
        pipe.HSet(ctx, "sub:"+rec.IMSI, map[string]interface{}{
            "ki":  rec.Ki,
            "opc": rec.OPc,
            "amf": rec.AMF,
            "sqn": rec.SQN,
        })
    }
    _, err := pipe.Exec(ctx)
    if err != nil {
        // Transaction failed - all operations are discarded
        return fmt.Errorf("import failed: %w", err)
    }
    return nil
}
```

**注記：** 上書きモード時に元データを復元したい場合は、インポート前に手動でバックアップを取得する運用とする（自動バックアップはPoC段階では実装しない）。

---

## 7. 起動時処理

### 7.1 初期化シーケンス

```
1. 環境変数読み込み (envconfig)
   └─ VALKEY_PASSWORD 未設定 → エラー終了

2. Valkey接続確認
   └─ 接続失敗 → エラーメッセージ表示して終了

3. 画面初期化 (tview.Application)

4. メインメニュー表示
```

### 7.2 エラー時の表示

tview.Modal を使用した独立ダイアログ。ボーダータイトル「Connection Error」（Red）。画面中央に表示。

```
       ┌ Connection Error ──────────────────────────────────┐
       │                                                      │
       │  Failed to connect to Valkey:                        │
       │                                                      │
       │  {実際のエラーメッセージ}                              │
       │                                                      │
       │  Please check:                                       │
       │  - Valkey is running on 127.0.0.1:6379               │
       │  - VALKEY_PASSWORD environment variable is set       │
       │    correctly                                         │
       │                                                      │
       │          < Retry >     < Exit >                       │
       │                                                      │
       └──────────────────────────────────────────────────────┘
```

---

## 8. 監査ログ出力（最低限）

Admin TUIからの操作は、標準出力にJSON形式で記録する。

### 出力フォーマット

```json
{
  "time": "2025-06-20T14:30:00.000Z",
  "level": "INFO",
  "app": "admin-tui",
  "event_id": "AUDIT_LOG",
  "msg": "subscriber created",
  "operation": "create",
  "target_type": "subscriber",
  "target_key": "sub:440101234567890",
  "admin_user": "admin"
}
```

### 記録対象操作

| 操作 | event_id | operation |
|------|----------|-----------|
| 加入者登録 | `AUDIT_LOG` | `create` |
| 加入者編集 | `AUDIT_LOG` | `update` |
| 加入者削除 | `AUDIT_LOG` | `delete` |
| Client登録/編集/削除 | `AUDIT_LOG` | `create`/`update`/`delete` |
| Policy登録/編集/削除 | `AUDIT_LOG` | `create`/`update`/`delete` |
| CSVインポート | `AUDIT_LOG` | `import` |
| CSVエクスポート | `AUDIT_LOG` | `export` |

**注記：** `admin_user` は現時点では固定値 `"admin"` とする。将来的にユーザー認証機能を追加する場合に拡張。

---

## 9. ポリシーなし加入者の扱い

### 方針

ポリシーが存在しない加入者（`policy:{IMSI}` がValkeyに存在しない）は **一律deny（認証拒否）** として扱う。

### 影響範囲

| コンポーネント | 影響内容 |
|---------------|---------|
| **Auth Server** | `policy:{IMSI}` が存在しない場合、Access-Reject を返却 |
| **Admin TUI** | 加入者一覧でポリシー未設定を視覚的に識別（セクション4.2.1参照） |
| **運用手順** | 加入者登録→ポリシー登録の順序が必須（ドキュメントに明記） |

---

## 10. 未決事項・将来検討課題

| No. | 項目 | 内容 | 判断時期 |
|-----|------|------|---------|
| 1 | 大量データ対応 | 10,000件超の場合のパフォーマンスチューニング | PoC完了後 |
| 2 | インポート前バックアップ | 上書きモード時の自動バックアップ機能 | PoC完了後（現状は手動運用） |
| 3 | ユーザー認証 | admin_userのOS認証連携 | PoC完了後 |
| 4 | カラムソート | カラムヘッダクリックでソート | PoC完了後 |

---

## 改訂履歴

| 版数 | 日付 | 内容 |
|------|------|------|
| r1 | 2025-12-26 | 初版（前半：CRUD・バリデーション・インポート/エクスポート） |
| r2 | 2025-12-27 | レビュー反映：英語UI、ページネーション、フィルタ仕様、ポリシー視覚的識別、SQN警告、Default allow警告、2フェーズインポート、ドキュメント成果物追加 |
| r3 | 2026-01-27 | IMSI表示方針追加: セクション1.3として、Admin TUIはIMSIを常に生値表示/記録する方針を明記。旧1.3以降は再ナンバリング実施 |
| r4 | 2026-02-06 | ポリシーデータ形式の修正: Auth Serverとの互換性確保のため、認可ポリシーをHash形式で保存するよう変更。セクション1.5にデータ形式詳細を追加。PolicyRule.VlanIDをString型に変更 |
| r5 | 2026-02-07 | 全マスタデータのHash形式統一: 加入者データ・RADIUSクライアントデータもサーバーコンポーネントとの互換性確保のためHash形式に変更。セクション1.5の管理対象データテーブルを更新し、各データ型のHash形式フィールド定義を追加 |
| r6 | 2026-02-18 | PolicyRule構造をD-02 r10に準拠して更新: 旧構造（NAS-ID/Allowed SSIDs/VLAN ID/Session Timeout）を新構造（SSID/Action/TimeMin/TimeMax）に変更。ポリシー登録/編集画面、ルール編集サブダイアログ、バリデーションルール、CSVフォーマット例を更新 |
| r7 | 2026-02-21 | 実機検証不具合修正の反映: セクション3.2にF5キー（リフレッシュ）追加、セクション3.9新設（ページライフサイクル管理 — tcell差分レンダリング対策のSync()、InputCapture内QueueUpdateDrawのgoroutineラップ、Import/Export完了時のページクリーンアップ、form.Clear後のInputCapture再登録）、ポリシーフォームのフォーカス切替をTabからF6に変更。旧3.9は3.10に再ナンバリング |
| r8 | 2026-02-22 | Session Detail フリーズ不具合修正の知見反映: セクション3.10新設（tview Table の Selectable 状態管理 — 全セル NotSelectable 時の無限ループ問題と SetSelectable 切替による対策）、セクション3.11新設（非同期データ取得パターン — QueueUpdateDraw 内でのネットワーク I/O 回避）。旧3.10は3.12に再ナンバリング |
| r9 | 2026-02-23 | 実装画面とのレイアウト整合性修正: スクリーンショット検証に基づくASCII図全面更新。§3.1 Ctrl+C→Ctrl+Q、F1/?ヘルプキー追加。§3.2 F2-F6ファンクションキー+代替文字キー追加。§4.1 メインメニューをtview.List形式に更新（ショートカット(1)-(q)括弧表記、ボーダータイトル追加）。§4.2.1 加入者一覧を6カラム+行頭"!"表示に更新（Ki/OPcマスク表示追加）。§4.2.2 フォームタイトルCreate/Edit Subscriber、SQN警告タイトルSQN Modification Warning。§4.3.1 クライアント一覧にSecret列マスク表示追加。§4.3.2 フォームタイトルCreate/Edit RADIUS Client。§4.4.1-4.4.2 ポリシーフォームタイトルPolicy Details、NAS ID/SSIDs/VLAN/Timeoutルール構造。§5.1 バリデーションルール表のRule部分をNAS ID/Allowed SSIDs/VLAN ID/Session Timeoutに更新。§6.5-6.6 インポート/エクスポート画面に状態遷移（Import Data→Import Completed、Export Data→Export Completed）追加。§7.2 起動エラーをConnection Errorモーダルに更新 |
