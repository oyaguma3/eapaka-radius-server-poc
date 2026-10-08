# D-05 Admin TUI 詳細設計書【前半】(r15)

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

RADIUSクライアントについては、あわせてサーバー採番の ID の索引 `idx:client:{ID}`（String。値は IP）と採番のカウンター `seq:client`（String。`INCR`）を使う（D-02 §2.B）。Admin TUI はこれらを直接読み書きせず、`pkg/masterdata` の `ClientStore` を通して扱う。

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
| `id` | String (10進数) | サーバー採番の ID（1 から始まる連番。再利用しない） |
| `secret` | String | 共有シークレット |
| `name` | String | クライアント名称 |
| `vendor` | String | ベンダー名 |

**Valkeyコマンド例（`pkg/masterdata` の Lua スクリプトで、次の3つを1回の操作として行う）：**
```
INCR "seq:client"                          # → 3
HSET "client:192.168.1.100" "id" "3" "secret" "mysecretkey" "name" "AP-Floor1" "vendor" "cisco"
SET "idx:client:3" "192.168.1.100"
```

**注記：** IPアドレスはキーに含まれるため、Hashフィールドには含めない。Auth Server / Acct Serverは `HGet` コマンドで共有シークレットを読み取る（`id` は使わない）。

**注記（ID）：** ID は作成（Create）時に `seq:client` を `INCR` して採番し、Hash の `id` と索引 `idx:client:{ID}` に書き込む。削除（Delete）ではクライアントの Hash とあわせて索引も消す。削除したクライアントの ID は再利用しない。編集（Update）では ID は変わらない。いずれも `pkg/masterdata` の Lua スクリプトで行い、Admin TUI の画面のコードは ID の採番・索引を扱わない。ID の導入前に登録されたクライアント（`id` がない）には、起動時に ID を採番する（§7.1）。

#### 認可ポリシーのデータ形式

| フィールド | 型 | 説明 |
|-----------|-----|------|
| `default` | String | デフォルトアクション（`allow` または `deny`） |
| `rules` | String (JSON配列) | ポリシールールの配列 |

**Valkeyコマンド例：**
```
HSET "policy:440101234567890" "default" "deny" "rules" '[{"nas_id":"AP-OFFICE-01","allowed_ssids":["CORP-WIFI"],"vlan_id":"100","session_timeout":3600},{"nas_id":"*","allowed_ssids":["GUEST-WIFI"]}]'
```

**注記：** Auth Serverは `HGETALL` コマンドでポリシーを読み取る。`rules` の各ルールは `nas_id` / `allowed_ssids` / `vlan_id`（JSON文字列）/ `session_timeout`（秒。0・省略は未設定）を持つ（D-02 §2.C）。

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
 └─[Q] Exit（終了。確認ダイアログなし）
```

### 2.2 画面遷移図

```
                    ┌─────────────────┐
                    │   起動時処理      │
                    │  (DB接続確認)     │
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
          │                 │                 │
    ┌─────▼─────┐     ┌─────▼─────┐     ┌─────▼─────┐
    │[S1] List  │     │[C1] List  │     │[P1] List  │
    └───┬───────┘     └───┬───────┘     └───┬───────┘
        │ 選択/操作        │ 選択/操作         │ 選択/操作
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
| `Esc` | 前の画面に戻る / キャンセル | メインメニューでは確認なしで終了。フィルタ適用中の一覧ではフィルタ解除 |
| `Ctrl+Q` | アプリケーション終了 | 確認なしで即座に終了 |
| `F1` / `?` | ヘルプ表示 | 現在画面のキーバインド一覧をモーダル表示 |
| `Tab` | 次のフォーカス要素へ移動 | - |
| `Shift+Tab` | 前のフォーカス要素へ移動 | - |

**注記：** `F1` / `?` はアプリケーション全体の InputCapture（`main.go` の `setupGlobalKeyBindings`）で処理する。ただし、フォーカス中の部品が文字入力を受け付ける部品（`tview.InputField` / `tview.TextArea`。`ui.IsTextInput` で判定）の場合は `?` をショートカットとして扱わず、文字としてそのまま入力欄へ渡す（フォームやフィルタ・検索ダイアログの入力欄で `?` を入力できるようにするため）。入力欄にフォーカスがあるときにヘルプを開くには `F1` を使う。

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

**注記：**
- 加入者・クライアント・ポリシーの一覧で `Enter` を押すと、`F3` / `e` と同じく選択項目の編集画面（ポリシーは Policy Details フォーム）を開く。一覧画面は `Enter` で `onSelect` コールバックを呼ぶため、`main.go` で各一覧に `SetOnSelect` を設定して編集画面を開く（`SetOnEdit` と同じ処理）。
- `F6` / `/` のフィルタは、加入者・クライアント・ポリシーの一覧と Session List（D-07 §5）で共通である。ポリシー詳細フォームの `F6`（§4.4.2。フォーム部分からルールリストへのフォーカス移動）は、フォーム側の InputCapture で処理するため一覧の `F6` とは競合しない。

### 3.3 キーバインド（フォーム画面共通）

| キー | 動作 |
|------|------|
| `Tab` / `Shift+Tab` | 次 / 前のフィールド・ボタンへ移動 |
| `Save` ボタン | 保存実行（tview.Form 標準ボタン） |
| `Cancel` ボタン / `Esc` | 保存せずに一覧画面へ戻る（確認ダイアログなし。入力内容は破棄する） |

**注記：** 保存のショートカットキー（`Ctrl+S` 等）は設けない。保存は `Save` ボタンで行う。

### 3.4 確認ダイアログ仕様

破壊的操作や重要な変更時に表示する。いずれも `tview.Modal` によるダイアログで、`ui.NewConfirmDialog`（`Yes` / `No`）または `ui.NewWarningDialog`（`Continue` / `Cancel`。本文の先頭に `⚠ WARNING ⚠`）で生成する。

| 種別（ボーダータイトル） | トリガー | メッセージ | 選択肢 |
|------|---------|-------------|--------|
| **削除確認**（`Confirm Delete`） | 一覧で削除操作（`F4` / `d`） | `Are you sure you want to delete this subscriber?`<br>（空行）<br>`440101234567890`<br>（client / policy も同形式で、対象のIP・IMSIを表示） | `[Yes] [No]` |
| **SQN変更警告**（`SQN Modification Warning`） | 加入者の編集で SQN を変更して保存（大文字小文字の違いだけは変更とみなさない） | §4.2.2 参照 | `[Continue] [Cancel]` |
| **Default allow 警告**（`Default Allow Warning`） | ポリシーの Default を `allow` にして保存 | §3.5 参照 | `[Continue] [Cancel]` |
| **接続エラー**（`Connection Error`） | 起動時の Valkey 接続失敗 | §7.2 参照 | `[Retry] [Exit]` |

**注記：**
- 変更破棄確認・終了確認・上書き確認のダイアログは設けない。フォームの `Esc` / `Cancel` は確認なしで入力内容を破棄して一覧へ戻り、メインメニューの `q` / `Esc` / `(q) Exit` は確認なしで終了する。既存キーで新規登録しようとした場合は、ダイアログを出さずにステータスバーにエラー（`Failed to create: subscriber already exists` 等）を表示する（§5.2）。
- ダイアログ表示中に `Esc` を押すと、tview.Modal の仕様で2つ目のボタン（`No` / `Cancel`）と同じ処理になる（`Connection Error` では `Exit`）。

### 3.5 Default "allow" 設定時の警告ダイアログ

ポリシー保存時にDefaultが "allow" に設定されている場合に表示する。

tview.Modal（`ui.NewWarningDialog`）を使用。ボーダータイトル「Default Allow Warning」（Yellow）。Policy Details の上にオーバーレイ表示。

```
        ┌ Default Allow Warning ───────────────────────────────────┐
        │                                                          │
        │                    ⚠ WARNING ⚠                           │
        │                                                          │
        │  Setting default action to 'allow' means the subscriber  │
        │  will have access even without matching rules.           │
        │                                                          │
        │  Are you sure you want to continue?                      │
        │                                                          │
        │           < Continue >     < Cancel >                    │
        │                                                          │
        └──────────────────────────────────────────────────────────┘
```

`Continue` で保存し、`Cancel`（または `Esc`）で保存せずに Policy Details に戻る。

### 3.6 メッセージ表示

画面下部にステータスバーを配置し、操作結果を表示する。

| 種別 | 表示色 | 表示時間 | 例 |
|------|-------|---------|-----|
| 成功 | 緑 | 5秒 | `✓ Subscriber created: 440101234567890` |
| エラー | 赤 | 5秒 | `✗ Validation error: IMSI: must be 15 digits` |

5秒経過後は既定の表示（` F1:Help | q:Back/Quit | Ctrl+Q:Exit`）に戻る。ステータスバーには警告（黄、`⚠`）・情報（シアン、`ℹ`）の種別も定義されているが、現行の画面では使用していない。

### 3.7 フィルタ機能仕様

| 項目 | 仕様 |
|------|------|
| 起動方法 | `/` または `F6` キー押下でフィルタ入力ダイアログ（`ui.NewInputDialog`。`OK` / `Cancel` ボタン）を表示し、入力欄にフォーカス |
| ダイアログを閉じる | `OK` で適用。`Cancel` ボタンまたは `Esc` でフィルタを変えずに閉じる（`Esc` は `tview.Form.SetCancelFunc` で `Cancel` と同じ処理を呼ぶ） |
| 対象カラム | 加入者一覧: IMSI（入力欄 `IMSI contains:`）、クライアント一覧: ID・IP Address・Name・Vendor（入力欄は `IP/Name/Vendor contains:` のまま）、ポリシー一覧: IMSI（`IMSI contains:`）。Session List は D-07 §5.4。いずれも部分一致 |
| マッチング | 大文字小文字を区別しない（case-insensitive） |
| 動作 | ダイアログの `OK` 押下時に、入力文字列を含む行のみ表示する（入力中の逐次絞り込みはしない）。適用時は1ページ目に戻る |
| クリア | 一覧画面で `Esc` を押すとフィルタ解除、全件表示に戻る（フィルタ未適用時の `Esc` は前の画面に戻る） |
| 件数表示 | ボーダータイトルに `(Filter: "入力値")` と絞り込み後の件数・ページ情報を表示（例: `Subscriber List (Filter: "00101") 1-6 of 6 (Page 1/1)`） |

フィルタはクライアント側（一覧表示時に取得済みのデータ）に対して適用する。一覧表示時に全件を取得しているため、フィルタ時の追加取得はしない。

### 3.8 ページネーション仕様

| 項目 | 仕様 |
|------|------|
| データ取得 | 一覧表示時（および `F5` / `r` の再読み込み時）に `SCAN`（COUNT 100）で全キーを走査して全件を取得し、メモリに保持する |
| 表示 | 1ページあたり50件 |
| ナビゲーション | `PgUp` 前ページ / `PgDn` 次ページ（`←` / `→` はページ切替に使わない） |
| UI形式 | ボーダータイトルに `1-50 of 125 (Page 1/3)` 形式で表示（0件時は `No items`） |
| フィルタ併用時 | 取得済みデータに適用し、絞り込み後の件数でページ分割する |

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
┌ Admin TUI - Main Menu ──────────────────────────────────────┐
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
│ (4) Import/Export                                           │
│     Import or export data as CSV files                      │
│                                                             │
│ (5) Monitoring                                              │
│     View statistics and active sessions                     │
│                                                             │
│ (q) Exit                                                    │
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
| `q` / `Esc` | Admin TUI を終了（確認ダイアログなし。`(q) Exit` の選択も同じ） |

---

### 4.2 加入者管理

#### 4.2.1 加入者一覧 [S1]

##### レイアウト

ボーダータイトルに「Subscriber List」+件数・ページ情報を表示。フィルタ適用時は `(Filter: "...")` を付加。

```
┌ Subscriber List 1-9 of 9 (Page 1/1) ───────────────────────────────────────────────┐
│ IMSI              Ki              OPc              AMF    SQN            Created   │
│  001010000000000  465B5CE8...A6BC CD63CB71...2BAF  B9B9   000000000001   2024-01-01│
│  001010000000001  465B5CE8...A6BC CD63CB71...2BAF  B9B9   000000000021   2024-01-01│
│  001010000000003  465B5CE8...A6BC CD63CB71...2BAF  8000   0000000000c1   2024-01-01│
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

tview.Form を centered() ヘルパーで画面中央にダイアログ表示する（幅60。高さは入力欄の数から `ui.FormHeight` で計算し、入力欄5つで15行。§4.3.2 の注記）。背景に一覧テーブルが透過表示される。新規作成時のタイトルは「Create Subscriber」、編集時は「Edit Subscriber」。

```
              ┌ Create Subscriber ────────────────────────────────┐
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
| SQN | Yes | `000000000000` | 表示・編集可能（警告表示付き）。空欄にすると保存時にエラー。編集時に変更しなければ `sqn` は書き換えない（下記「編集の保存処理」） |

##### SQN手動編集時の警告

編集時に SQN フィールドの値を変更して `Save` を押した際（入力値の検証に通った後）に以下の警告を表示する。`Continue` で保存し、`Cancel`（または `Esc`）で保存せずに Edit Subscriber に戻る。

SQN を変更したかどうかは、正規化（§5.3。大文字化）した入力値と編集開始時（Edit Subscriber を開いたとき）に読んだ SQN を**大文字小文字を区別せずに**比較して判定する（`sqnChanged`。`strings.EqualFold`）。Vector API は `sqn` を小文字 hex で書き戻す（D-02 セクション2.A）ため、認証後の加入者を開いて SQN に触れずに保存しても、大文字化による違いだけでは変更とみなさず、警告を表示しない。

tview.Modal を使用。ボーダータイトル「SQN Modification Warning」（Yellow）。Edit Subscriber ダイアログの上にオーバーレイ表示。

```
        ┌ SQN Modification Warning ────────────────────────────────┐
        │                                                          │
        │                    ⚠WARNING ⚠                           │
        │                                                          │
        │  Modifying the SQN value may cause                       │
        │  authentication failures.                                │
        │                                                          │
        │  Are you sure you want to change the SQN                 │
        │  from                                                    │
        │    000000000000 to 000000000010?                         │
        │                                                          │
        │           < Continue >     < Cancel >                    │
        │                                                          │
        └──────────────────────────────────────────────────────────┘
```

##### 編集の保存処理

SQN の扱いを除き、編集の保存では Ki / OPc / AMF を更新する（IMSI は変更不可、`created_at` は変更しない）。ストア層（`internal/store/subscriber.go`）では、1つの Lua スクリプト（`updateSubscriberScript`）で `sub:{IMSI}` の存在チェックと更新をまとめて行う（D-02 セクション2.A）。

| 条件 | ストア層の処理 | 結果 |
|------|---------------|------|
| SQN を変更していない（大文字小文字の違いだけを含む） | `Update`: `ki` / `opc` / `amf` だけを更新し、`sqn` は書き換えない | 成功。編集画面を開いている間に認証で Vector API が進めた SQN はそのまま残る |
| SQN を変更した（警告で `Continue`） | `UpdateWithSQN`: 現在の `sqn` が編集開始時に読んだ値と一致するときだけ、`ki` / `opc` / `amf` と `sqn`（大文字に正規化した入力値）を更新する | 一致すれば成功。一致しない（編集中に認証で SQN が進んだ）ときは何も更新せず、エラー（`ErrSQNChanged`）とする |
| 加入者が削除されていた | （どちらも）何も更新せず、キーも作成しない | エラー（`ErrSubscriberNotFound`） |

保存に失敗した場合は、ステータスバーに以下のエラーを赤字で表示し（§3.6）、Edit Subscriber のまま一覧には戻らない。監査ログ（§8）は更新に成功した場合だけ記録する。

| 原因 | ステータスバーの表示 |
|------|---------------------|
| 編集中に認証で SQN が変わっていた（SQN を変更した場合のみ） | `Failed to update: SQN was changed by authentication while editing. Reopen the subscriber and try again` |
| 加入者が削除されていた | `Failed to update: subscriber not found` |
| その他（Valkey エラー等） | `Failed to update: <エラー内容>` |

SQN が変わっていたエラーの場合は、Edit Subscriber を閉じて開き直し（最新の SQN が表示される）、改めて SQN を入力して保存する。

**注記：** r11 までの実装では、SQN を変更していなくても `sqn` をフォームの値（編集開始時の値）で上書きしていたため、編集画面を開いている間に認証が進むと保存時に SQN が巻き戻る可能性があった。また SQN の変更判定が大文字小文字を区別していたため、Vector API が小文字で書き戻した SQN の加入者では、SQN に触れなくても警告が表示され、`Continue` すると編集開始時の値で上書きしていた。r12 でいずれも解消した。

---

### 4.3 RADIUSクライアント管理

#### 4.3.1 クライアント一覧 [C1]

##### レイアウト

ボーダータイトルに「RADIUS Client List」+件数・ページ情報を表示。

```
┌ RADIUS Client List 1-6 of 6 (Page 1/1) ──────────────────────────────────┐
│ ID  IP Address       Name             Secret          Vendor             │
│  4  10.0.0.100       E2E-NAS          e2****as        generic            │
│  1  127.0.0.1        Localhost        TE****23        generic            │
│  2  172.19.0.1       DockerGateway    TE****23        generic            │
│  5  192.168.1.10     TestAP-01        te****p1        generic            │
│  6  192.168.10.1     Customer01       TE****23        generic            │
│  3  192.168.30.1     testClient       ab****mn        none               │
└──────────────────────────────────────────────────────────────────────────┘
F1:Help  |  q:Back/Quit  |  Ctrl+Q:Exit
```

行の並び順は従来どおり IP Address の文字列順（`internal/ui/client/list.go` の `Load`。ID の順ではない）。ID は採番した順の番号なので、上の例のように IP の順とは一致しない。

##### 表示項目

| カラム | 内容 | Expansion | 色 | 備考 |
|--------|------|-----------|-----|------|
| ID | サーバー採番の ID（§1.5） | 0 | Gray | 右寄せ。短いので列を広げない（ヘッダーのセルも Expansion 0） |
| IP Address | クライアントIPアドレス | 1 | White | - |
| Name | クライアント名称 | 1 | White | 超過時は "..." で切り詰め |
| Secret | 共有シークレット（マスク表示） | 1 | Gray | 先頭2文字+****+末尾2文字（例: `e2****as`） |
| Vendor | ベンダー名 | 1 | Gray | 空の場合は `-` と表示 |

**注記：** Shared Secretは一覧でマスク表示する（先頭2文字+****+末尾2文字）。

**注記（ID）：** フィルタ（`/` / `F6`。§3.7）は ID・IP Address・Name・Vendor のいずれかに入力文字列を含む行を表示する（部分一致。たとえば `1` は ID 1・12 や IP に `1` を含む行に一致する）。ID の導入前に登録されたクライアントにも起動時に採番する（§7.1）。Admin TUI の起動後に `pkg/masterdata` を通さずに（valkey-cli などで直接）登録したクライアントは `id` を持たないため ID を `0` と表示する（次回の起動時に採番される）。

#### 4.3.2 クライアント登録 [C2] / 編集 [C3]

##### レイアウト

tview.Form を centered() ヘルパーで画面中央にダイアログ表示する（幅60。高さは入力欄の数から `ui.FormHeight` で計算し、入力欄4つで13行）。新規作成時のタイトルは「Create RADIUS Client」、編集時は「Edit RADIUS Client (ID n)」（`n` はそのクライアントの ID。§1.5）。すべての入力欄と Save / Cancel ボタンを常に枠内に表示する。

> **注記（フォームの高さ）:** tview.Form（既定の枠・余白・項目間隔）は、枠2行＋上下の余白2行＋入力欄ごとに2行（欄と空行）＋ボタン1行の高さが要る。`internal/ui/form.go` の `FormHeight(入力欄の数)` でこれを計算し、`main.go` の加入者・RADIUSクライアントのフォームの表示に使う。2026-10-08 まではクライアントの画面の高さを固定値 12 にしていたため（必要な高さは 13）、Save / Cancel ボタンの行が枠の外に出て表示されず、Tab でボタンにフォーカスを移したときだけフォーム内がスクロールして表示されていた。

```
              ┌ Create RADIUS Client ─────────────────────────────┐
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

編集時: タイトル「Edit RADIUS Client (ID n)」（`internal/ui/client/form.go` の `SetupEdit` で ` Edit RADIUS Client (ID %d) ` を設定する。例: ` Edit RADIUS Client (ID 1) `）、IP Addressフィールドは無効化（グレーアウト、編集不可）。

```
              ┌ Edit RADIUS Client (ID 1) ────────────────────────┐
```

**注記（ID）：** ID は入力しない（登録・編集とも入力欄は IP Address / Secret / Name / Vendor の4つのまま）。登録時は保存（Create）のときに ID を採番するため、登録画面のタイトルに ID は出さない（「Create RADIUS Client」のまま）。編集の保存（Update）では ID は変わらない。

##### フィールド定義

| フィールド | 必須 | 初期値 | 編集時の挙動 |
|-----------|------|-------|-------------|
| IP Address | Yes | 空 | 編集時は変更不可（読取専用表示） |
| Secret | Yes | 空 | 表示・編集可能 |
| Name | Yes | 空 | 表示・編集可能 |
| Vendor | No | 空 | 表示・編集可能 |

---

### 4.4 認可ポリシー管理

#### 4.4.1 ポリシー一覧 [P1]

##### レイアウト

ボーダータイトルに「Authorization Policy List」+件数・ページ情報を表示。

```
┌ Authorization Policy List 1-9 of 9 (Page 1/1) ───────────────────────────┐
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
┌ Policy Details ────────────────────────────────────────────────────┐
│                                                                    │
│  IMSI             001010000000004        (disabled)                │
│  Default Action   [deny ▼]                                         │
│                                                                    │
│  < Add Rule >  < Save >  < Cancel >                                │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
┌ Rules ─────────────────────────────────────────────────────────────┐
│ [1] NAS: Customer01                                                │
│ SSIDs: TESTSSID-01, TESTSSID-02 | VLAN: 10 | Timeout: 7200s        │
│ [2] NAS: Customer02                                                │
│ SSIDs: Guest | VLAN: 20 | Timeout: 1800s                           │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
F1:Help  |  q:Back/Quit  |  Ctrl+Q:Exit
```

**備考:** Rules リストは tview.List を使用（メインテキスト: `[N] NAS: {nasID}`、サブテキスト: `SSIDs: ... | VLAN: ... | Timeout: ...s`、サブテキスト色: Green）。

##### ルール編集サブダイアログ

centered(form, width=60, height=15) で Policy Details の上にオーバーレイ表示。ボーダー色は Teal/Cyan。

新規追加時のタイトル: 「Add Rule」、ボタン: OK / Cancel
編集時のタイトル: 「Edit Rule」、ボタン: OK / Delete / Cancel

```
       ┌ Edit Rule ─────────────────────────────────────────────┐
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

| キー / ボタン | 動作 |
|------|------|
| `F6`（フォーム部分で） | ルールリストへフォーカスを移す |
| `Esc` / `Tab`（ルールリストで） | フォーム部分へフォーカスを戻す |
| `Enter`（ルールリストで） | 選択したルールの編集サブダイアログ（Edit Rule）を開く |
| `Add Rule` ボタン | ルール追加サブダイアログ（Add Rule）を開く |
| `Save` ボタン | 保存（Default が `allow` の場合は §3.5 の警告ダイアログを表示） |
| `Cancel` ボタン / `Esc`（フォーム部分で） | 保存せずに一覧へ戻る（確認ダイアログなし） |

**注記：**
- `Tab` キーはtviewのフォーム内ナビゲーション（フィールド間移動）で使用されるため、フォームからルールリストへのフォーカス移動には `F6` キーを使用する（ルールリストからフォームへは `Esc` / `Tab`）。
- 保存のショートカットキー（`Ctrl+S` 等）は設けない。
- ルール編集サブダイアログは `OK` / `Delete`（編集時のみ）/ `Cancel` ボタンで閉じる。サブダイアログでの `Esc` は無効（ダイアログは閉じない）。

##### フィールド定義（ポリシー本体）

| フィールド | 必須 | 初期値 | 編集時の挙動 |
|-----------|------|-------|-------------|
| IMSI | Yes | 空 | 編集時は変更不可 |
| Default | Yes | `deny` | ドロップダウン選択（`deny` / `allow`） |
| Rules | No | 空配列 | サブリストで管理（0件でも保存できる。ルールが0件の場合は Default のみで判定される） |

**注記：** Defaultを "allow" に設定して保存する場合、警告ダイアログを表示（セクション3.5参照）。

##### フィールド定義（ルール）

| フィールド | 必須 | 初期値 | 幅 | 型 |
|-----------|------|-------|-----|-----|
| NAS ID | Yes | 空 | 40 | String（RADIUS NAS-Identifier 属性と比較する値。`*` 単独で任意のNASに一致） |
| Allowed SSIDs | Yes | 空 | 40 | String（カンマ区切り、例: `SSID1,SSID2`。前後の空白は除去し、空要素は無視） |
| VLAN ID | No | 空 | 10 | String（数字） |
| Session Timeout | No | `0` | 10 | String（秒数。数値として解釈できない入力は `0`（未設定）として扱う） |

**注記：** D-02 Valkeyデータ設計仕様書のPolicyRule構造（`nas_id` / `allowed_ssids` / `vlan_id` / `session_timeout`）に準拠する。NAS ID には、AP（NAS）が Access-Request に載せる RADIUS `NAS-Identifier` 属性の値を指定する。Auth Server は `nas_id` を NAS-Identifier と完全一致（大文字小文字を区別）で比較し、NAS IPアドレス（NAS-IP-Address や送信元IP）とは比較しない。`*` 単独は任意のNAS（NAS-Identifier が無いリクエストを含む）に一致するワイルドカードで、部分一致（`AP-*` など）は行わない（D-02 §2.C）。

---

## 5. 入力バリデーション仕様

### 5.1 バリデーションルール一覧

バリデーションは `pkg/validation` パッケージで行う（画面の保存時と CSV インポートで共通。Provisioning API（D-13）とも共通。E-03 §8）。エラーメッセージは `{Field}: {Message}` 形式で、画面ではステータスバーに `Validation error: ` を前置して最初の1件のみ表示する。

| 対象 | フィールド | ルール | エラーメッセージ（`{Field}: {Message}`） |
|------|-----------|--------|-----------------|
| Subscriber | IMSI | 必須。15桁の数字 `^[0-9]{15}$` | `IMSI: required` / `IMSI: must be 15 digits` |
| Subscriber | Ki | 必須。32桁のHex `^[0-9A-Fa-f]{32}$` | `Ki: required` / `Ki: must be 32 hex characters` |
| Subscriber | OPc | 必須。32桁のHex `^[0-9A-Fa-f]{32}$` | `OPc: required` / `OPc: must be 32 hex characters` |
| Subscriber | AMF | 必須。4桁のHex `^[0-9A-Fa-f]{4}$` | `AMF: required` / `AMF: must be 4 hex characters` |
| Subscriber | SQN | 必須。12桁のHex `^[0-9A-Fa-f]{12}$` | `SQN: required` / `SQN: must be 12 hex characters` |
| Client | IP Address | 必須。有効なIPv4（各オクテット0〜255） | `IP: required` / `IP: must be a valid IPv4 address` |
| Client | Secret | 必須。1〜128文字のASCII印字可能文字（空白を除く `^[\x21-\x7E]{1,128}$`） | `Secret: required` / `Secret: must be at most 128 characters` / `Secret: must contain only printable ASCII characters (no spaces)` |
| Client | Name | 必須。1〜64文字の英数字・ハイフン・アンダースコア `^[a-zA-Z0-9_-]{1,64}$` | `Name: required` / `Name: must be at most 64 characters` / `Name: must contain only alphanumeric characters, hyphens, and underscores` |
| Client | Vendor | 任意。0〜64文字の英数字・スペース・ハイフン `^[a-zA-Z0-9 -]{0,64}$` | `Vendor: must be at most 64 characters` / `Vendor: must contain only alphanumeric characters, spaces, and hyphens` |
| Policy | IMSI | （Subscriberと同じ） | `IMSI: IMSI: required` / `IMSI: IMSI: must be 15 digits` |
| Policy | Default | 必須。`allow` または `deny` | `Default: required` / `Default: must be 'allow' or 'deny'` |
| Rule | NAS ID | 必須。1〜253文字の印字可能ASCII（空白を除く `^[\x21-\x7E*]{1,253}$`）。`*` 単独は任意のNASに一致するワイルドカード | `NasID: required` / `NasID: must be at most 253 characters` / `NasID: must contain only printable ASCII characters and wildcards` |
| Rule | Allowed SSIDs | 1件以上必須（カンマ区切り）。各SSIDは1〜32文字（バイト数） | `AllowedSSIDs: at least one SSID required` / `AllowedSSIDs[{i}]: SSID: required`（CSVで空文字列のSSIDを指定した場合） / `AllowedSSIDs[{i}]: SSID: must be at most 32 characters` |
| Rule | VLAN ID | 任意。空 または 0〜4094 の整数 | `VlanID: must be a valid number` / `VlanID: must be non-negative` / `VlanID: must be at most 4094` |
| Rule | Session Timeout | 任意。0〜86400（秒。0は未設定） | `SessionTimeout: must be non-negative` / `SessionTimeout: must be at most 86400 seconds` |

**注記：**
- ルール編集サブダイアログの OK 押下時はルール単体を検証する（メッセージは上表のとおり）。ポリシーの保存時はポリシー全体を検証し、ルールのエラーは `Rules[{i}].NasID: required` のように `Rules[{i}].` を前置する（`{i}` は0始まり）。
- Policy の IMSI のエラーは、加入者用の検証結果を `IMSI` フィールドで包むため `IMSI: IMSI: must be 15 digits` のように表示される。
- 画面の Session Timeout 入力欄は数値として解釈できない値を `0` として扱うため、`SessionTimeout: must be non-negative` は負の数を入力した場合のみ表示される。

### 5.2 バリデーションタイミング

| タイミング | 動作 |
|-----------|------|
| **入力中・フォーカス離脱時** | 検証しない（入力文字種の制限もない） |
| **保存時（Save / ルールの OK）** | 正規化（§5.3）の後、§5.1 の全項目を検証。エラーがあれば保存せず、最初の1件をステータスバーに赤字表示 |
| **登録時（新規作成）** | 同じキー（`sub:{IMSI}` / `client:{IP}` / `policy:{IMSI}`）が既に存在する場合はエラー（`Failed to create: subscriber already exists` 等）。存在確認と書き込みは1つの Lua スクリプトで行うため、同時に作成しても既存の値を上書きしない（`pkg/masterdata`。E-03 §9.3） |
| **更新時（加入者の編集）** | 加入者が削除されていた場合、および SQN を変更して保存したときに編集中に SQN が変わっていた場合はエラー（§4.2.2「編集の保存処理」） |

### 5.3 入力値の正規化

- 保存時に各フィールドの前後の空白を除去する（ポリシーの IMSI・NAS ID・各SSIDを含む）
- Hexフィールド（Ki / OPc / AMF / SQN）は保存時に大文字に正規化してValkeyに保存する（入力中の表示は変換しない）

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
| 既存IMSI | 上書き（スキップ・上書きの選択オプションはない） |
| バリデーションエラー（§5.1。ヘッダー・列数の不正を含む） | 1行でもエラーがあればインポート全体を中断し、エラー一覧（`line {N}: {Field}: {Message}`）を結果エリアに表示する（1件も投入しない。§6.7） |

#### エクスポート動作

- 全加入者を出力
- 出力ファイルパスは画面で指定する（既定値なし。空欄の場合は `Output file path is required`）

### 6.3 RADIUSクライアントCSV仕様

```csv
ip,secret,name,vendor
192.168.1.100,mysecret123,AP-Floor1,cisco
192.168.1.101,anothersecret,AP-Floor2,cisco
```

**注記（ID）：** CSV ではサーバー採番の ID（§1.5）を扱わない。列は `ip,secret,name,vendor` のまま（ID の導入前と同じ形式）で、エクスポートにも ID は出力しない。インポート（`pkg/masterdata` の `ClientStore.BulkCreate`）では、既存の IP のクライアントは ID を引き継いで上書きし、新しい IP のクライアントには ID を採番する（1件ずつ Lua スクリプトで、Hash・索引・カウンターをまとめて書き換える）。このため、エクスポートした CSV を別の環境にインポートすると、ID は元の環境と一致しない場合がある。

### 6.4 認可ポリシーCSV仕様

```csv
imsi,default,rules_json
440101234567890,deny,"[{""nas_id"":""AP-OFFICE-01"",""allowed_ssids"":[""CORP-WIFI""],""vlan_id"":""100"",""session_timeout"":3600},{""nas_id"":""*"",""allowed_ssids"":[""GUEST-WIFI""]}]"
```

**注記：** `rules_json` フィールドはJSON文字列をダブルクォートでエスケープしてCSV格納する。空文字列または `[]` はルールなし。各ルールは §5.1 の Rule の規則で検証する（エラーは `line {N}, rule[{i}]: NasID: required` の形式）。

### 6.5 インポート画面 [I1]

メインメニューの `(4) Import/Export` で Import/Export メニュー（tview.List。ボーダータイトル「Import/Export」、項目 `(1) Import` / `(2) Export` / `(q) Back`）を表示する。メニューで `q` / `Esc` を押すとメインメニューへ戻る。インポート画面・エクスポート画面では、`Cancel` ボタンまたは `Esc` で Import/Export メニューへ戻る（完了後の状態では `Done` ボタンまたは `Esc`）。

FlexRow で上部（Formエリア）と下部（Result表示エリア）に分割。Data Type は tview.DropDown（Subscribers / RADIUS Clients / Policies）。インポート完了後にフォームが状態遷移する。

#### 初期状態（Import Data）

```
┌ Import Data ───────────────────────────────────────────────────────┐
│                                                                    │
│  Data Type    [Subscribers      ▼]                                 │
│  File Path    [/home/admin/import.csv                          ]   │
│                                                                    │
│  < Validate >  < Import >  < Cancel >                              │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
┌ Import Result ─────────────────────────────────────────────────────┐
│                                                                    │
│  (結果表示エリア — スクロール可能)                                      │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
F1:Help  |  q:Back/Quit  |  Ctrl+Q:Exit
```

Validate 実行後は結果エリアのタイトルが「Validation Result」に変化し、`Validation passed!`（緑色）+ `Records to import: N` を表示。

#### 完了後の状態遷移（Import Completed）

インポート完了後、フォームタイトルが「Import Completed」に変わり、ボタンが Done / Import More に切り替わる。結果エリアには `Import completed!`（緑色）+ `Imported: N {type}` を表示。

```
┌ Import Completed ──────────────────────────────────────────────────┐
│                                                                    │
│  < Done >  < Import More >                                         │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
┌ Import Result ─────────────────────────────────────────────────────┐
│                                                                    │
│  Import completed!                                                 │
│  Imported: 15 subscribers                                          │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
F1:Help  |  q:Back/Quit  |  Ctrl+Q:Exit
```

### 6.6 エクスポート画面 [I2]

FlexRow で上部（Formエリア）と下部（Result表示エリア）に分割。Data Type は tview.DropDown。エクスポート完了後にフォームが状態遷移する。

#### 初期状態（Export Data）

```
┌ Export Data ──────────────────────────────────────────────────────┐
│                                                                   │
│  Data Type     [Subscribers      ▼]                               │
│  Output File   [/home/admin/subscriber_export_20260223.txt     ]  │
│                                                                   │
│  < Export >  < Cancel >                                           │
│                                                                   │
└───────────────────────────────────────────────────────────────────┘
┌ Export Result ────────────────────────────────────────────────────┐
│                                                                   │
│  (結果表示エリア)                                                   │
│                                                                   │
└───────────────────────────────────────────────────────────────────┘
F1:Help  |  q:Back/Quit  |  Ctrl+Q:Exit
```

#### 完了後の状態遷移（Export Completed）

エクスポート完了後、フォームタイトルが「Export Completed」に変わり、ボタンが Done / Export More に切り替わる。結果エリアには `Export completed!`（緑色）+ 件数・ファイルパスを表示。画面最下部のステータスバーにも緑色で成功メッセージが表示される。

```
┌ Export Completed ────────────────────────────────────────────────────┐
│                                                                      │
│  < Done >  < Export More >                                           │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
┌ Export Result ───────────────────────────────────────────────────────┐
│                                                                      │
│  Export completed!                                                   │
│  Exported: 24 subscribers                                            │
│  File: /home/admin/subscriber_export_20260223.txt                    │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
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
1. 環境変数読み込み（os.Getenv）
   └─ VALKEY_PASSWORD 未設定時は空のパスワードとして扱う（ここではエラーにしない）

2. Valkey接続確認（127.0.0.1:6379 に接続し PING）、および RADIUSクライアントの ID の採番（`ClientStore.EnsureIDs`）
   └─ 接続失敗 または 採番の失敗 → Connection Error ダイアログ（§7.2）を表示
      ├─ Retry → 再接続（採番も再実行）。成功すればメインメニューへ、失敗すればステータスバーに `Connection failed: ...`
      └─ Exit（または Esc）→ 終了

3. 画面初期化 (tview.Application)

4. メインメニュー表示
```

**注記（RADIUSクライアントの ID の採番）：** 手順2は `main.go` の `connectValkey` で行う。PING とストアの初期化の後に `masterdata.ClientStore.EnsureIDs` を呼び、ID の導入前に登録された（Hash に `id` がない）RADIUSクライアントに ID を採番する（§1.5）。ID を持つクライアントは変えない（索引 `idx:client:{ID}` がなければ作り直し、カウンター `seq:client` が ID より小さければ合わせる）。何度実行しても結果は同じなので、起動のたびに実行する。Provisioning API（D-13）も起動時に同じ処理を行う。`EnsureIDs` が失敗した場合は Valkey の接続失敗と同じ扱いで Connection Error ダイアログを表示する。

### 7.2 エラー時の表示

tview.Modal を使用した独立ダイアログ。ボーダータイトル「Connection Error」（Red）。画面中央に表示。

```
       ┌ Connection Error ────────────────────────────────────┐
       │                                                      │
       │  Failed to connect to Valkey:                        │
       │                                                      │
       │  {実際のエラーメッセージ}                                │
       │                                                      │
       │  Please check:                                       │
       │  - Valkey is running on 127.0.0.1:6379               │
       │  - VALKEY_PASSWORD environment variable is set       │
       │    correctly                                         │
       │                                                      │
       │          < Retry >     < Exit >                      │
       │                                                      │
       └──────────────────────────────────────────────────────┘
```

---

## 8. 監査ログ出力（最低限）

Admin TUIからの操作は、標準出力にJSON形式で記録する。

### 出力フォーマット

```json
{
  "time": "2025-06-20T14:30:00Z",
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

**注記：**
- `admin_user` は現時点では固定値 `"admin"` とする。将来的にユーザー認証機能を追加する場合に拡張。
- `time` は RFC3339（秒精度・UTC）。
- CSVインポート/エクスポートでは `target_key` にCSVファイルパスを、`record_count`（数値）にインポート/エクスポートしたレコード件数を記録する（0件も `0` として出力）。インポート/エクスポートは成功した場合のみ記録する。`record_count` はそれ以外の操作では出力しない。
- モニタリング画面の Session Detail 検索（`search`）は D-07 §10 を参照。各操作で出力するフィールドの一覧は D-04 §3.5 を参照。

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
| r10 | 2026-10-04 | 実装との不一致の修正: §4.4.2 ルールの NAS ID を「NAS IPアドレスまたはNAS ID」から、RADIUS `NAS-Identifier` 属性と完全一致で比較する値（`*` 単独で任意のNASに一致。NAS IPアドレスとは比較しない。D-02 §2.C）に修正。§5.1 バリデーションルールを `internal/validation` の実装に合わせて全面修正（NAS ID 1-64文字→1〜253文字の印字可能ASCII・`*` ワイルドカード、SQN・Client Name を必須に、Name を英数字・ハイフン・アンダースコア、Vendor を0〜64文字の英数字・スペース・ハイフン、Allowed SSIDs の各SSID 1〜32文字、VLAN ID 0〜4094、Session Timeout 0〜86400、エラーメッセージを実際の `{Field}: {Message}` 形式に）。§5.2 バリデーションタイミングを保存時のみ（リアルタイムの文字種制限・フォーカス離脱時の検証はない）に、§5.3 を入力値の正規化（保存時に空白除去・Hexを大文字化）に修正。§4.2.2 SQN・§4.3.2 Name を必須に、§4.4.2 Rules を任意（0件可）、Default をドロップダウン選択に修正。§1.5・§6.4 のポリシールールの例を現行構造（`nas_id` / `allowed_ssids` / `vlan_id` / `session_timeout`）に修正。§6.2 インポート動作を実装（既存IMSIは上書き、エラーが1行でもあれば全体を中断）とエクスポートの出力ファイルパス（既定値なし）に修正。Admin TUI の監査ログに件数を記録する実装修正の反映: §8 に import / export の `record_count`（0件も出力、成功時のみ記録）を追記し、出力例の `time` を秒精度に修正 |
| r11 | 2026-10-04 | Admin TUI のキー配線漏れを修正した実装修正の反映: §3.1 に、`F1` / `?` をグローバルの InputCapture で処理し、入力欄（`tview.InputField` / `tview.TextArea`。`ui.IsTextInput`）にフォーカスがあるときは `?` を文字として入力欄へ渡す旨の注記を追加。§3.2 に、加入者・クライアント・ポリシーの一覧の `Enter` で編集画面（ポリシーは Policy Details）を開くこと（`main.go` で各一覧に `SetOnSelect` を設定）、`F6` / `/` のフィルタは各一覧と Session List で共通で、ポリシー詳細フォームの `F6` とは競合しないことの注記を追加。§3.7 フィルタの起動方法に `F6` を追加し、フィルタ入力ダイアログを `Cancel` ボタンまたは `Esc`（`tview.Form.SetCancelFunc`）で閉じられること、一覧画面の `Esc` でフィルタを解除することを明記。あわせて、キー操作・ダイアログの記述を実装（`main.go`、`internal/ui`）に合わせて修正: §2.1 / §3.1 / §4.1 メインメニューの `q` / `Esc` は確認なしで終了（終了確認ダイアログはない）。§3.3 フォームのキーに `Tab` / `Shift+Tab` と `Cancel` / `Esc`（確認なしで破棄）を追加し、保存のショートカットはないことを明記。§3.4 確認ダイアログを実装にあるもの（`Confirm Delete` の `Yes` / `No`、SQN変更警告・Default allow 警告の `Continue` / `Cancel`、`Connection Error` の `Retry` / `Exit`）に差し替え、変更破棄確認・終了確認・上書き確認はないこと、ダイアログの `Esc` は2つ目のボタンと同じ動作であることを追記。§3.5 Default allow 警告ダイアログを実際の表示（`Default Allow Warning`、`Continue` / `Cancel`）に差し替え。§3.6 ステータスバーの表示時間・例を実装（成功・エラーとも5秒）に修正。§3.7 フィルタの対象カラム（画面ごと）、`OK` 押下で適用（逐次絞り込みはしない）、件数表示（ボーダータイトルの `(Filter: ...)` とページ情報）を修正し、SCANによる追加取得の記述を削除。§3.8 ページネーションのナビゲーションを `←` / `→` から `PgUp` / `PgDn` に、UI形式をボーダータイトルの `1-50 of 125 (Page 1/3)` に、データ取得を一覧表示時の全件取得に修正。§4.2.2 SQN変更警告の表示タイミングを保存時（SQN を変更して `Save`）に修正。§4.4.2 ポリシーフォームのキーから `Ctrl+S` を削除し、`F6`（フォーム→ルールリスト）、ルールリストの `Esc` / `Tab` / `Enter`、各ボタン、ルール編集サブダイアログは `Esc` では閉じないことを記載。§6.5 に Import/Export メニューの操作と、インポート/エクスポート画面の `Cancel` / `Esc` / `Done` を追加。§7.1 初期化シーケンスを実装（VALKEY_PASSWORD 未設定でもエラーにしない、接続失敗時は Connection Error ダイアログで Retry / Exit）に修正 |
| r12 | 2026-10-04 | Admin TUI の加入者編集による SQN の上書き（巻き戻り）を解消した実装修正の反映: §4.2.2 の「SQN手動編集時の警告」に、SQN の変更判定は正規化後の入力値と編集開始時の値を大文字小文字を区別せずに比較すること（Vector API が小文字で書き戻した SQN で誤って警告が出ていた問題の修正）を追記。「編集の保存処理」を新設し、SQN を変更していなければ `sqn` を書き換えない（`Update`）、変更した場合は編集開始時の値と一致するときだけ書き換える（`UpdateWithSQN`）、Lua スクリプトで存在チェックと更新をまとめて行い削除済みの加入者のキーを作らないこと、失敗時のステータスバーのエラー（`Failed to update: SQN was changed by authentication while editing. Reopen the subscriber and try again` 等）と開き直しての再実行、監査ログは成功時のみであることを記載。§3.4 SQN変更警告のトリガー、§4.2.2 フィールド定義の SQN、§5.2 に更新時のエラーを補足 |
| r13 | 2026-10-07 | Admin TUI の加入者・RADIUSクライアント・認可ポリシーの store と validation を pkg に移した実装修正（Provisioning API（D-13）と共通で使うため。E-03 r11）の反映: §5.1 のバリデーションの実装箇所を `pkg/validation` に修正、§5.2 の登録時のエラーに、存在確認と書き込みを1つの操作で行い同時に作成しても上書きしない旨を追記 |
| r14 | 2026-10-08 | Admin TUI の RADIUS クライアントの登録・編集画面で Save / Cancel ボタンが枠外に出て表示されなかった不具合の修正の反映: §4.3.2 のレイアウトの説明（フォーカスが下に移るとスクロールしてボタンが表示される、としていた）を、高さを入力欄の数から `ui.FormHeight` で計算し（入力欄4つで13行）ボタンを常に表示する動作に修正し、高さの計算と経緯の注記を追加。§4.2.2 に加入者のフォームの高さ（同じ関数、入力欄5つで15行）を追記 |
| r15 | 2026-10-08 | RADIUSクライアントにサーバー採番の ID を導入した実装修正（D-02、D-13）の反映: §1.5 の RADIUSクライアントの Hash に `id` を追加し、索引 `idx:client:{ID}`・カウンター `seq:client`、作成時の採番・削除時の索引の削除（`pkg/masterdata` の Lua）、ID を再利用しないことを追記。§3.7・§4.3.1 のクライアント一覧に、先頭の ID 列（右寄せ、灰色、幅は広げない。列は ID / IP Address / Name / Secret / Vendor）とフィルタの対象への ID の追加、並び順は従来どおり IP の文字列順であることを追記。§4.3.2 の編集画面のタイトルを「Edit RADIUS Client (ID n)」に修正し、登録画面のタイトルは変えないこと・ID は入力しないことを追記。§6.3 に CSV では ID を扱わない（列は変えない）こと、インポートは既存のクライアントの ID を引き継ぎ新しいクライアントに採番することを追記。§7.1 に起動時（`connectValkey`）の `ClientStore.EnsureIDs` による ID の導入前のクライアントへの採番（何度実行しても同じ。失敗時は接続エラーと同じ扱い）を追記。2026-10-08 に simwifi で、一覧の ID 表示、Admin TUI で登録したクライアントへの ID 3 の採番、編集画面のタイトル `Edit RADIUS Client (ID 1)` を確認。あわせて、§4.3.1 の Vendor が空のときの表示を実装どおり `-` に訂正（従来から `none` と誤記）。§6.3 のインポートは、従来どおり MULTI / EXEC で全件を1回の操作として書き込む |
