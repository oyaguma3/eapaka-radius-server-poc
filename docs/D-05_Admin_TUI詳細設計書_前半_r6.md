# D-05 Admin TUI 詳細設計書【前半】(r6)

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
| `Ctrl+C` | 強制終了 | 確認なしで即座に終了 |
| `?` | ヘルプ表示 | 現在画面のキーバインド一覧をモーダル表示 |
| `Tab` | 次のフォーカス要素へ移動 | - |
| `Shift+Tab` | 前のフォーカス要素へ移動 | - |

### 3.2 キーバインド（一覧画面共通）

| キー | 動作 |
|------|------|
| `↑` | カーソル上移動 |
| `↓` | カーソル下移動 |
| `Enter` | 選択項目の編集画面へ |
| `n` | 新規登録画面へ |
| `d` | 削除確認ダイアログ表示 |
| `/` | 検索モード（フィルタ入力） |
| `r` | 一覧を再読み込み |
| `←` | 前のページへ |
| `→` | 次のページへ |

### 3.3 キーバインド（フォーム画面共通）

| キー | 動作 |
|------|------|
| `Ctrl+S` | 保存実行 |
| `Esc` | キャンセル（変更破棄確認あり） |

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

### 3.9 テキスト切り詰め仕様

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

```
┌─────────────────────────────────────────────────────────────┐
│  EAP-AKA RADIUS PoC - Admin TUI                    v1.0.0  │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│    [S] Subscriber Management                                │
│    [C] RADIUS Client Management                             │
│    [P] Authorization Policy Management                      │
│    ─────────────────────────────────                        │
│    [I] Import/Export                                        │
│    [O] Monitoring                                           │
│    ─────────────────────────────────                        │
│    [Q] Exit                                                 │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│  Valkey: Connected (127.0.0.1:6379)  │  ? Help             │
└─────────────────────────────────────────────────────────────┘
```

#### 操作

| キー | 動作 |
|------|------|
| `S` | 加入者管理画面へ |
| `C` | RADIUSクライアント管理画面へ |
| `P` | 認可ポリシー管理画面へ |
| `I` | インポート/エクスポート画面へ |
| `O` | モニタリング画面へ（後半で定義） |
| `Q` / `Esc` | 終了確認ダイアログ表示 |

---

### 4.2 加入者管理

#### 4.2.1 加入者一覧 [S1]

##### レイアウト

```
┌─────────────────────────────────────────────────────────────┐
│  Subscriber Management - List                [n]New [?]Help │
├─────────────────────────────────────────────────────────────┤
│  Filter: [________________]              Showing: 125 / 125 │
├─────┬─────────────────┬────────┬────────────────────────────┤
│ No. │ IMSI            │ Policy │ Created                    │
├─────┼─────────────────┼────────┼────────────────────────────┤
│   1 │ 440101234567890 │ Yes    │ 2025-01-15 10:30           │
│ ! 2 │ 440101234567891 │ No     │ 2025-01-15 10:31           │
│   3 │ 440109876543210 │ Yes    │ 2025-01-16 14:20           │
│   : │ :               │ :      │ :                          │
├─────────────────────────────────────────────────────────────┤
│  [Prev] Page 1/3 [Next]                                     │
├─────────────────────────────────────────────────────────────┤
│  ↑↓:Move  Enter:Edit  n:New  d:Delete  /:Filter  Esc:Back  │
└─────────────────────────────────────────────────────────────┘
```

##### 表示項目

| カラム | 内容 | 幅 |
|--------|------|-----|
| No. | 連番 | 5 |
| IMSI | 加入者識別番号 | 17 |
| Policy | ポリシー有無 (`Yes` / `No`) | 8 |
| Created | 作成日時 | 20 |

##### ポリシー未設定加入者の視覚的識別

| 条件 | 表示 |
|------|------|
| ポリシーあり | 通常表示、Policyカラムに `Yes` |
| ポリシーなし | 黄色ハイライト、No.カラムに `!` プレフィックス、Policyカラムに `No` |

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

**注記：** Ki/OPcは一覧では表示しない（編集画面で表示）。SQNは運用中に変化するため一覧から除外。

#### 4.2.2 加入者登録 [S2] / 編集 [S3]

##### レイアウト

```
┌─────────────────────────────────────────────────────────────┐
│  Subscriber Management - Add                                │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│    IMSI *:  [440101234567890___]                            │
│             15 digits                                       │
│                                                             │
│    Ki *:    [0123456789ABCDEF0123456789ABCDEF]              │
│             32 hex characters (128bit)                      │
│                                                             │
│    OPc *:   [FEDCBA9876543210FEDCBA9876543210]              │
│             32 hex characters (128bit)                      │
│                                                             │
│    AMF *:   [8000]                                          │
│             4 hex characters (16bit)                        │
│                                                             │
│    SQN:     [000000000000]                                  │
│             12 hex characters (48bit), auto-managed         │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│  [Ctrl+S] Save    [Esc] Cancel                              │
└─────────────────────────────────────────────────────────────┘
```

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

```
┌─────────────────────────────────────────────────────────────┐
│  Warning: Manual SQN Edit                                   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  1. The displayed value is from when this record was        │
│     loaded. It may have been updated by authentication      │
│     attempts since then.                                    │
│                                                             │
│  2. Changing SQN during operation may cause authentication  │
│     failures due to synchronization mismatch with the SIM.  │
│                                                             │
│  3. Setting an excessively large value may lead to          │
│     sequence number exhaustion (SQN rollover issues).       │
│                                                             │
│  Are you sure you want to modify SQN manually?              │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│  [Edit Anyway]                                  [Cancel]    │
└─────────────────────────────────────────────────────────────┘
```

---

### 4.3 RADIUSクライアント管理

#### 4.3.1 クライアント一覧 [C1]

##### レイアウト

```
┌─────────────────────────────────────────────────────────────┐
│  RADIUS Client Management - List             [n]New [?]Help │
├─────────────────────────────────────────────────────────────┤
│  Filter: [________________]                Showing: 8 / 8   │
├─────┬─────────────────┬──────────────────┬──────────────────┤
│ No. │ IP Address      │ Name             │ Vendor           │
├─────┼─────────────────┼──────────────────┼──────────────────┤
│   1 │ 192.168.1.100   │ AP-Floor1        │ cisco            │
│ > 2 │ 192.168.1.101   │ AP-Floor2-Lon... │ cisco            │
│   3 │ 10.0.0.1        │ TestClient       │                  │
├─────────────────────────────────────────────────────────────┤
│  [Prev] Page 1/1 [Next]                                     │
├─────────────────────────────────────────────────────────────┤
│  ↑↓:Move  Enter:Edit  n:New  d:Delete  /:Filter  Esc:Back  │
└─────────────────────────────────────────────────────────────┘
```

##### 表示項目

| カラム | 内容 | 幅 | 切り詰め |
|--------|------|-----|---------|
| No. | 連番 | 5 | - |
| IP Address | クライアントIPアドレス | 17 | - |
| Name | クライアント名称 | 18 | "..." 付加 |
| Vendor | ベンダー名 | 18 | "..." 付加 |

**注記：** Shared Secretは一覧では表示しない。

#### 4.3.2 クライアント登録 [C2] / 編集 [C3]

##### レイアウト

```
┌─────────────────────────────────────────────────────────────┐
│  RADIUS Client Management - Add                             │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│    IP Address *:  [192.168.1.100____]                       │
│                   IPv4 format                               │
│                                                             │
│    Secret *:      [mysecretkey_______]                      │
│                   1-128 printable ASCII chars               │
│                                                             │
│    Name:          [AP-Floor1_________]                      │
│                   Display name, max 64 chars                │
│                                                             │
│    Vendor:        [cisco_____________]                      │
│                   For VSA, alphanumeric/hyphen, max 32      │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│  [Ctrl+S] Save    [Esc] Cancel                              │
└─────────────────────────────────────────────────────────────┘
```

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

```
┌─────────────────────────────────────────────────────────────┐
│  Authorization Policy Management - List      [n]New [?]Help │
├─────────────────────────────────────────────────────────────┤
│  Filter: [________________]               Showing: 45 / 45  │
├─────┬─────────────────┬──────────┬──────────────────────────┤
│ No. │ IMSI            │ Default  │ Rules                    │
├─────┼─────────────────┼──────────┼──────────────────────────┤
│   1 │ 440101234567890 │ deny     │ 2 rules                  │
│ > 2 │ 440101234567891 │ allow    │ 1 rule                   │
│   3 │ 440109876543210 │ deny     │ 3 rules                  │
├─────────────────────────────────────────────────────────────┤
│  [Prev] Page 1/1 [Next]                                     │
├─────────────────────────────────────────────────────────────┤
│  ↑↓:Move  Enter:Edit  n:New  d:Delete  /:Filter  Esc:Back  │
└─────────────────────────────────────────────────────────────┘
```

#### 4.4.2 ポリシー登録 [P2] / 編集 [P3]

##### レイアウト

```
┌─────────────────────────────────────────────────────────────┐
│  Authorization Policy Management - Edit      IMSI: 44010... │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  IMSI *:     [440101234567890___]                           │
│                                                             │
│  Default *:  ( ) allow  (●) deny                            │
│              Action when no rules match                     │
│                                                             │
│  ┌─ Rules ────────────────────────────────────────────────┐ │
│  │ No. │ SSID         │ Action │ TimeMin │ TimeMax       │ │
│  │─────┼──────────────┼────────┼─────────┼───────────────│ │
│  │ > 1 │ CORP-WIFI    │ allow  │ 09:00   │ 18:00         │ │
│  │   2 │ GUEST-WIFI   │ allow  │         │               │ │
│  │   3 │ *            │ deny   │         │               │ │
│  │                                                        │ │
│  │ [a]Add  [e]Edit  [d]Delete  [↑↓]Move                   │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│  [Ctrl+S] Save    [Esc] Cancel                              │
└─────────────────────────────────────────────────────────────┘
```

##### ルール編集サブダイアログ

```
┌─────────────────────────────────────────┐
│  Edit Rule                              │
├─────────────────────────────────────────┤
│                                         │
│  SSID *:           [CORP-WIFI_____]     │
│                    Required, or *       │
│                                         │
│  Action *:         ( ) allow  (●) deny  │
│                    Required             │
│                                         │
│  Time Min:         [09:00]              │
│                    Optional, HH:MM      │
│                                         │
│  Time Max:         [18:00]              │
│                    Optional, HH:MM      │
│                                         │
├─────────────────────────────────────────┤
│  [Enter] OK    [Esc] Cancel             │
└─────────────────────────────────────────┘
```

##### フィールド定義（ポリシー本体）

| フィールド | 必須 | 初期値 | 編集時の挙動 |
|-----------|------|-------|-------------|
| IMSI | Yes | 空 | 編集時は変更不可 |
| Default | Yes | `deny` | ラジオボタン選択 |
| Rules | Yes | 空配列 | サブリストで管理 |

**注記：** Defaultを "allow" に設定して保存する場合、警告ダイアログを表示（セクション3.5参照）。

##### フィールド定義（ルール）

| フィールド | 必須 | 初期値 | 型 |
|-----------|------|-------|-----|
| SSID | Yes | 空 | String（ワイルドカード`*`可） |
| Action | Yes | `deny` | String（`allow` または `deny`） |
| Time Min | No | 空 | String（HH:MM形式、空で制限なし） |
| Time Max | No | 空 | String（HH:MM形式、空で制限なし） |

**注記：** D-02 Valkeyデータ設計仕様書のPolicyRule構造に準拠する。SSIDにはワイルドカード`*`を指定して全SSID対象とすることができる。Time Min/Time Maxは時間帯制限で、両方空の場合は常時有効。

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
| Rule | SSID | 1-64文字、またはワイルドカード `*` | `SSID is required` |
| Rule | Action | `allow` または `deny` | - |
| Rule | Time Min | 空 または HH:MM形式（00:00-23:59） | `Time Min must be HH:MM format` |
| Rule | Time Max | 空 または HH:MM形式（00:00-23:59） | `Time Max must be HH:MM format` |

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

```
┌─────────────────────────────────────────────────────────────┐
│  Import                                                     │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Data Type:   (●) Subscriber  ( ) Client  ( ) Policy        │
│                                                             │
│  File Path:   [/home/admin/import.csv_______________]       │
│                                                             │
│  Options:                                                   │
│    [x] Overwrite existing data                              │
│    [ ] Dry run (do not actually import)                     │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│  [Enter] Execute    [Esc] Cancel                            │
└─────────────────────────────────────────────────────────────┘
```

### 6.6 エクスポート画面 [I2]

```
┌─────────────────────────────────────────────────────────────┐
│  Export                                                     │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Data Type:   (●) Subscriber  ( ) Client  ( ) Policy        │
│                                                             │
│  Output Path: [/home/admin/subscribers_20250620_143000.csv] │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│  [Enter] Execute    [Esc] Cancel                            │
└─────────────────────────────────────────────────────────────┘
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

```
┌─────────────────────────────────────────────────────────────┐
│  EAP-AKA RADIUS PoC - Admin TUI                            │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ✗ Startup Error                                            │
│                                                             │
│  Failed to connect to Valkey.                               │
│                                                             │
│  - Host: 127.0.0.1:6379                                     │
│  - Error: connection refused                                │
│                                                             │
│  Please check:                                              │
│  1. Docker Compose is running                               │
│  2. VALKEY_PASSWORD environment variable is correct         │
│                                                             │
│  [Enter] Retry    [Esc] Exit                                │
└─────────────────────────────────────────────────────────────┘
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
