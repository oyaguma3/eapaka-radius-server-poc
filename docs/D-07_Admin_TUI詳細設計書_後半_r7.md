# D-07 Admin TUI 詳細設計書【後半】(r7)

## 1. 概要

### 1.1 目的

本ドキュメントは、EAP-AKA RADIUS PoC環境における管理コンソール「Admin TUI」のモニタリング機能について詳細設計を定義する。

### 1.2 スコープ

**本書【後半】で扱う範囲：**
- モニタリング画面構成・遷移
- Statistics Dashboard仕様
- Session List仕様
- Session Detail仕様
- ヘルプダイアログ仕様

**本書【前半】（別ドキュメント）で定義済み：**
- 画面構成・遷移（マスタ管理部分）
- マスタデータのCRUD操作仕様
- 入力バリデーション
- CSVインポート/エクスポート
- 共通仕様（キーバインド、ページネーション、フィルタ、メッセージ表示等）

### 1.3 関連ドキュメント

| ドキュメント | 参照内容 |
|-------------|---------|
| D-05_Admin_TUI詳細設計書_前半_r9 | 共通仕様、キーバインド規約、ページネーション仕様、ページライフサイクル管理、tview Table Selectable状態管理、非同期データ取得パターン |
| D-02_Valkeyデータ設計仕様書 (r10) | `sess:{UUID}`, `idx:user:{IMSI}` のデータ構造 |
| D-06_エラーハンドリング詳細設計書 (r6) | TUIエラー表示仕様 |

### 1.4 PoC対象外機能

以下の機能は本PoCでは実装しない。

| 機能 | 理由 |
|------|------|
| セッション強制終了 | CoA/DM送信が必要となり複雑度が増すため |
| 自動更新（Auto-refresh） | PoC規模では手動更新で十分 |
| 部分一致IMSI検索 | 完全一致で運用可能、全件SCANのコスト回避 |
| NAS-IP / Client-IP フィルタ | PoC規模では不要 |
| 日時範囲指定フィルタ | PoC規模では不要 |
| 履歴セッション表示 | TTL超過で削除されたセッションはログベースで追跡 |

---

## 2. 画面構成

### 2.1 モニタリング画面一覧

前半で定義した画面構成に追加する形で、モニタリング配下の画面を定義する。

```
[M] Main Menu
 │
 ├─ ... (前半で定義済み) ...
 │
 ├─[O] Monitoring（モニタリングメニュー）
 │   ├─[O0] Statistics Dashboard（統計ダッシュボード）
 │   ├─[O1] Session List（セッション一覧）
 │   └─[O2] Session Search（セッション検索）
 │
 └─ ... (前半で定義済み) ...
```

### 2.2 画面遷移図

```
                    ┌─────────────────┐
                    │  [M] Main Menu  │
                    └────────┬────────┘
                             │ (5) キー
                             ▼
                    ┌─────────────────┐
          ┌─────────┤[O] Monitoring   ├─────────┐
          │         │     Menu        │         │
          │         └─────────────────┘         │
          │                                     │
    (1)キー│                              (2)キー│
          ▼                                     ▼
    ┌───────────┐                        ┌───────────┐
    │[O0]       │                        │[O1]       │
    │Statistics │                        │Session    │
    │Dashboard  │                        │List       │
    └───────────┘                        └─────┬─────┘
                                               │
                                         [/]キー（IMSI検索）
                                               ▼
                                        ┌───────────┐
                                        │[O2]       │
                                        │Session    │
                                        │Search     │
                                        └───────────┘
```

**備考:** Session Search [O2] はモニタリングメニューからは直接遷移せず、Session List から `/` キーによるIMSI検索で遷移する。

---

## 3. モニタリングメニュー [O]

### 3.1 レイアウト

tview.List（ShowSecondaryText=true）を使用。ボーダータイトルに「Monitoring」を表示。

```
┌ Monitoring ─────────────────────────────────────────────────┐
│                                                             │
│ (1) Statistics Dashboard                                    │
│     View system statistics and counts                       │
│                                                             │
│ (2) Session List                                            │
│     View active sessions                                    │
│                                                             │
│ (q) Back                                                     │
│     Return to main menu                                     │
│                                                             │
└─────────────────────────────────────────────────────────────┘
F1:Help  |  q:Back/Quit  |  Ctrl+Q:Exit
```

**備考:** Session Search（旧Session Detail）はメニューに含まれない。Session List 画面から `/` キーでIMSI検索して遷移する。

### 3.2 キーバインド

| キー | 動作 |
|------|------|
| `1` | Statistics Dashboard画面へ |
| `2` | Session List画面へ |
| `q` / `Esc` | メインメニューへ戻る |

---

## 4. Statistics Dashboard [O0]

### 4.1 概要

アクティブセッションの統計情報をサマリ表示する画面。1分間隔でキャッシュ更新される統計データを表示する。

### 4.2 レイアウト

tview.TextView を使用。ボーダータイトル「Statistics Dashboard」。テキスト内にセクション見出し・統計値・更新時刻・キーバインドを表示。

```
┌ Statistics Dashboard ────────────────────────────────────────────┐
│                                                                    │
│  System Statistics                                    ← 橙色/黄色 │
│                                                                    │
│    Subscribers:      24                               ← 白色      │
│    RADIUS Clients:    6                                            │
│    Policies:          9                                            │
│    Active Sessions:   9                                            │
│                                                                    │
│  Last updated: 2026-02-23 20:54:11                    ← 灰色      │
│  (Statistics are cached for 1 minute. Press 'r' to force refresh)  │
│                                                                    │
│  Key bindings:                                        ← 橙色/黄色 │
│    r - Refresh statistics                                          │
│    q/Esc - Back to menu                                            │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
F1:Help  |  q:Back/Quit  |  Ctrl+Q:Exit
```

### 4.3 表示項目

| 項目 | 説明 | データソース |
|------|------|-------------|
| Subscribers | 登録加入者数 | `sub:*` パターンのキー数 |
| RADIUS Clients | 登録クライアント数 | `client:*` パターンのキー数 |
| Policies | 登録ポリシー数 | `policy:*` パターンのキー数 |
| Active Sessions | アクティブセッション数 | `sess:*` パターンのキー数 |

### 4.4 通信量表示フォーマット

| 項目 | 単位 | 最大表示 | 桁数（カンマ含む） |
|------|------|---------|------------------|
| Input/Output total | KB | 999,999,999,999 KB | 18桁 |

**変換ロジック：**

```go
func formatTrafficKB(octets int64) string {
    kb := octets / 1024
    const maxKB = 999_999_999_999
    if kb > maxKB {
        kb = maxKB // カウントストップ
    }
    return fmt.Sprintf("%18s KB", formatWithCommas(kb))
}

func formatWithCommas(n int64) string {
    // 3桁ごとにカンマを挿入
    s := strconv.FormatInt(n, 10)
    var result strings.Builder
    for i, c := range s {
        if i > 0 && (len(s)-i)%3 == 0 {
            result.WriteRune(',')
        }
        result.WriteRune(c)
    }
    return result.String()
}
```

### 4.5 キャッシュ仕様

| 項目 | 仕様 |
|------|------|
| 更新間隔 | 1分（60秒） |
| 保持場所 | メモリ内（ゴルーチンで定期更新） |
| 初回表示 | キャッシュ未取得の場合は即時取得して表示 |
| 手動更新 | `r` キーでキャッシュを即時更新 |
| タイムスタンプ | キャッシュ取得時刻を `Updated: YYYY-MM-DD HH:MM:SS` 形式で表示 |

**実装イメージ：**

```go
type StatisticsCache struct {
    mu              sync.RWMutex
    ActiveSessions  int
    InputTotal      int64  // octets
    OutputTotal     int64  // octets
    UniqueNASIPs    int
    UniqueIMSIs     int
    UpdatedAt       time.Time
    initialized     bool
}

func (c *StatisticsCache) StartBackgroundUpdate(ctx context.Context, rdb *redis.Client) {
    // 初回即時取得
    c.Refresh(ctx, rdb)
    
    ticker := time.NewTicker(60 * time.Second)
    go func() {
        for {
            select {
            case <-ctx.Done():
                ticker.Stop()
                return
            case <-ticker.C:
                c.Refresh(ctx, rdb)
            }
        }
    }()
}

func (c *StatisticsCache) Refresh(ctx context.Context, rdb *redis.Client) error {
    // SCAN + HGETALL で全セッション取得し統計計算
    // ...
    c.mu.Lock()
    defer c.mu.Unlock()
    c.UpdatedAt = time.Now()
    c.initialized = true
    return nil
}
```

### 4.6 フォーカス仕様

Statistics Dashboard画面に遷移した際、TextViewにフォーカスを設定する。これにより、画面表示直後からキー操作（`r` キーによるリロード等）が可能となる。

### 4.7 キーバインド

| キー | 動作 |
|------|------|
| `r` | 統計情報を即時再取得・表示更新 |
| `Esc` | モニタリングメニューへ戻る |
| `?` | ヘルプダイアログ表示 |

### 4.8 エラー時の表示

| 状況 | 表示内容 |
|------|---------|
| Valkey接続失敗（初回） | ステータスバーにエラー表示、各数値項目は `---` 表示 |
| Valkey接続失敗（更新時） | ステータスバーにエラー表示、前回取得値を維持 |
| 手動リロード失敗 | ステータスバーにエラー表示（例: `✗ Failed to refresh statistics`） |

**エラー時レイアウト例：**

```
│    Active session count :        ---                        │
│                                                             │
│    Data traffic                                             │
│      Input total        :                      --- KB       │
│      Output total       :                      --- KB       │
│      Input/Output total :                      --- KB       │
```

---

## 5. Session List [O1]

### 5.1 概要

アクティブセッションの一覧を表示する画面。2つのソートモード（start_time降順 / IMSI昇順）を切り替え可能。

### 5.2 ソートモード

| モード | プライマリキー | セカンダリキー | 表示カラム順 | 切替キー |
|--------|--------------|--------------|-------------|---------|
| start_time降順（デフォルト） | start_time DESC | IMSI ASC | No., start_time, IMSI, NAS-IP, Acct-ID | `i` で IMSI昇順へ |
| IMSI昇順 | IMSI ASC | start_time DESC | No., IMSI, start_time, NAS-IP, Acct-ID | `t` で start_time降順へ |

### 5.3 レイアウト

セッション一覧は start_time 降順で表示する。ヘッダーの `Start Time` カラムにソートインジケータ `▼`（緑色）を表示する。

```
┌ Session List 1-9 of 9 (Page 1/1) ─────────────────────────────────────────────┐
│ IMSI              NAS IP        Client IP    Start Time ▼  Duration       Traffic │
│ 001010000000003   172.19.0.1                 02-22 22:50   22h 5m 21s     0B      │
│ 001010000000006   172.19.0.1                 02-22 22:50   22h 5m 58s     0B      │
│ 001010000000003   172.19.0.1                 02-22 22:43   22h 12m 36s    0B      │
│ 001010000000001   172.19.0.1                 02-22 22:43   22h 12m 46s    0B      │
│ :                 :             :             :             :              :       │
└────────────────────────────────────────────────────────────────────────────────────┘
F1:Help  |  q:Back/Quit  |  Ctrl+Q:Exit
```

**表示形式の注記:**
- `Start Time` は短縮形式 `MM-DD HH:MM`（年・秒なし）で表示する
- `Duration` は `XXh XXm XXs` 形式（スペース区切り、秒あり）で表示する。橙色で表示
- `Traffic` はコンパクト表記（`0B`, `1.2KB`, `5.3MB` 等）で表示する。緑色で表示
- `Client IP` はセッションによって空欄の場合がある
- ボーダータイトルに `Session List 1-N of M (Page X/Y)` 形式でページ情報を表示する

### 5.4 フィルタダイアログ

Session List 画面で `/` キーを押下すると、フィルタダイアログが表示される。

```
              ┌ Filter Sessions ──────────────────────┐
              │                                        │
              │  IMSI/IP contains:  [              ]   │
              │                                        │
              │       < OK >  < Cancel >               │
              │                                        │
              └────────────────────────────────────────┘
```

| 項目 | 仕様 |
|------|------|
| タイトル | `Filter Sessions` |
| フィルタ対象 | IMSI、NAS IP、Client IP（部分一致） |
| フィルタ適用時 | ボーダータイトルに `(Filter: "入力値")` を追加表示 |
| クリア | 空文字で OK 押下、またはフィルタ中に再度 `/` → 空文字で OK |

### 5.5 表示項目

| カラム | 内容 | 表示形式 | 備考 |
|--------|------|---------|------|
| IMSI | 加入者識別番号 | 15桁 | - |
| NAS IP | NAS IPアドレス | 可変幅 | - |
| Client IP | クライアントIPアドレス | 可変幅 | 空欄の場合あり |
| Start Time | セッション開始時刻 | `MM-DD HH:MM` | ソートインジケータ `▼`（緑色） |
| Duration | セッション経過時間 | `XXh XXm XXs` | 橙色表示 |
| Traffic | 通信量合計 | `0B`, `1.2KB` 等 | 緑色表示 |

**Start Time形式について：**

短縮形式 `MM-DD HH:MM`（例: `02-22 22:50`）を採用する。年・秒を省略することで、Duration・Traffic カラムを追加しても画面幅に収まる。

### 5.6 テキスト切り詰め

Acct-ID が12文字を超える場合、末尾を切り詰めて `...` を付加する。

```go
func truncateAcctID(s string, maxLen int) string {
    if len(s) <= maxLen {
        return s
    }
    if maxLen <= 3 {
        return s[:maxLen]
    }
    return s[:maxLen-3] + "..."
}
```

### 5.7 ページネーション仕様

前半のマスタ一覧と統一する。

| 項目 | 仕様 |
|------|------|
| 1ページあたり表示件数 | 50件 |
| ナビゲーション | `←` 前ページ / `→` 次ページ |
| UI形式 | `[Prev] Page 1/3 [Next]` |
| 件数表示 | `Showing: 50 / 125` 形式 |

### 5.8 データ取得方式

#### 5.8.1 処理フロー

```
1. SCAN コマンドで sess:* パターンのキーを取得（COUNT 100 で分割取得）
2. 各キーに対して Pipeline で HGETALL を実行（N+1問題回避）
3. 取得した map[string]string を mapToSession でモデル構造体に変換
4. メモリ上で現在のソートモードに従いソート
5. ページ分割して該当ページを表示
```

#### 5.8.2 セッションデータのRedis型

Auth Server / Acct Server はセッションを **Redis Hash型** で保存する（`HSET` コマンド）。Admin-TUI の SessionStore も **Hash型** で読み取る（`HGETALL` コマンド）。

| Redis Hashフィールド | 型 | model.Session フィールド | 備考 |
|---------------------|-----|------------------------|------|
| `imsi` | String | `IMSI` | 加入者識別番号 |
| `nas_ip` | String | `NasIP` | NAS IPアドレス |
| `client_ip` | String | `ClientIP` | クライアントIPアドレス |
| `acct_id` | String | `AcctSessionID` | アカウンティングセッションID |
| `start_time` | String (数値) | `StartTime` (int64) | セッション開始時刻（Unix秒） |
| `input_octets` | String (数値) | `InputOctets` (int64) | 受信バイト数 |
| `output_octets` | String (数値) | `OutputOctets` (int64) | 送信バイト数 |

**注記：** `UUID` はHashフィールドには含まれない。Redis キー `sess:{UUID}` から `sess:` プレフィックスを除去して取得する。

#### 5.8.3 mapToSession ヘルパー関数

`HGETALL` で取得した `map[string]string` から `model.Session` 構造体へ変換するヘルパー関数を使用する。

```go
func mapToSession(uuid string, m map[string]string) (*model.Session, error) {
    session := &model.Session{
        UUID:          uuid,
        IMSI:          m["imsi"],
        NasIP:         m["nas_ip"],
        ClientIP:      m["client_ip"],
        AcctSessionID: m["acct_id"],
    }

    if v, ok := m["start_time"]; ok && v != "" {
        n, err := strconv.ParseInt(v, 10, 64)
        if err != nil {
            return nil, fmt.Errorf("invalid start_time: %w", err)
        }
        session.StartTime = n
    }

    if v, ok := m["input_octets"]; ok && v != "" {
        n, err := strconv.ParseInt(v, 10, 64)
        if err != nil {
            return nil, fmt.Errorf("invalid input_octets: %w", err)
        }
        session.InputOctets = n
    }

    if v, ok := m["output_octets"]; ok && v != "" {
        n, err := strconv.ParseInt(v, 10, 64)
        if err != nil {
            return nil, fmt.Errorf("invalid output_octets: %w", err)
        }
        session.OutputOctets = n
    }

    return session, nil
}
```

#### 5.8.4 セッション一覧取得

```go
func fetchAllSessions(ctx context.Context, rdb *redis.Client) ([]SessionListItem, error) {
    var sessions []SessionListItem
    var cursor uint64

    for {
        keys, nextCursor, err := rdb.Scan(ctx, cursor, "sess:*", 100).Result()
        if err != nil {
            return nil, err
        }

        if len(keys) > 0 {
            pipe := rdb.Pipeline()
            cmds := make(map[string]*redis.MapStringStringCmd)
            for _, key := range keys {
                cmds[key] = pipe.HGetAll(ctx, key)
            }
            pipe.Exec(ctx)

            for key, cmd := range cmds {
                data, err := cmd.Result()
                if err != nil || len(data) == 0 {
                    continue
                }
                uuid := strings.TrimPrefix(key, "sess:")
                session, err := mapToSession(uuid, data)
                if err != nil {
                    continue
                }
                sessions = append(sessions, sessionToListItem(session))
            }
        }

        cursor = nextCursor
        if cursor == 0 {
            break
        }
    }

    return sessions, nil
}

func sortByStartTimeDesc(sessions []SessionListItem) {
    sort.Slice(sessions, func(i, j int) bool {
        if sessions[i].StartTime.Equal(sessions[j].StartTime) {
            return sessions[i].IMSI < sessions[j].IMSI
        }
        return sessions[i].StartTime.After(sessions[j].StartTime)
    })
}

func sortByIMSIAsc(sessions []SessionListItem) {
    sort.Slice(sessions, func(i, j int) bool {
        if sessions[i].IMSI == sessions[j].IMSI {
            return sessions[i].StartTime.After(sessions[j].StartTime)
        }
        return sessions[i].IMSI < sessions[j].IMSI
    })
}
```

### 5.9 キーバインド

| キー | 動作 |
|------|------|
| `PgUp` | 前のページへ |
| `PgDn` | 次のページへ |
| `/` / `F6` | フィルタダイアログ表示 |
| `r` / `F5` | 一覧を再読み込み |
| `Esc` / `q` | モニタリングメニューへ戻る |
| `F1` / `?` | ヘルプダイアログ表示 |

### 5.10 エラー時の表示

| 状況 | 表示内容 |
|------|---------|
| セッション一覧取得失敗 | ステータスバーにエラー表示、一覧は空表示 |
| リロード失敗 | ステータスバーにエラー表示（例: `✗ Failed to load sessions`）、前回取得値を維持 |

---

## 6. Session Search [O2]

### 6.1 概要

IMSI完全一致検索により、特定加入者のセッション詳細を表示する画面。該当IMSIに紐づく全アクティブセッションの一覧とサマリを表示する。

> **注記:** 実装上のページ名は `Session Search` である（設計初期の `Session Detail` から変更）。

### 6.2 画面状態

| 状態 | 表示内容 |
|------|---------|
| 初期状態 | 検索ダイアログ表示、IMSI入力欄にフォーカス |
| 検索後（結果あり） | サマリ + セッションテーブル |
| 検索後（結果なし） | サマリ + 「No sessions found」メッセージ |

#### 検索ダイアログ仕様

| 項目 | 仕様 |
|------|------|
| 表示方式 | `tview.Form` ベースのモーダルダイアログ（50×7） |
| 入力フィールド幅 | 20文字（IMSIは最大15桁のため十分） |
| OKボタン | 非同期検索を開始（セクション6.9.1参照） |
| Cancelボタン | 初回検索前（IMSI未設定）の場合は Session List に戻る。検索済みの場合は Session Detail 画面に留まる |
| 再検索 | `/` キーで検索ダイアログを再表示 |

### 6.3 レイアウト（検索結果表示）

上部にサマリ情報（IMSI、セッション数、再検索案内）、下部に `Sessions` ボーダータイトル付きのセッションテーブルを表示する。

```
┌ Session Search ──────────────────────────────────────────────────────────────────┐
│                                                                                    │
│  IMSI: 001010000000000                                              ← 橙色/黄色  │
│  Sessions found: 5                                                                 │
│                                                                                    │
│  Press '/' to search for another IMSI                                ← 灰色      │
│                                                                                    │
├ Sessions ──────────────────────────────────────────────────────────────────────────┤
│ UUID            NAS IP        Client IP    Start Time    Duration       In/Out      │
│ e499a25b...     172.19.0.1                 02-22 22:40   22h 22m 22s   0B/0B       │
│ e44f5786...     172.19.0.1                 02-22 22:40   22h 22m 16s   0B/0B       │
│ 4673e913...     172.19.0.1                 02-22 22:41   22h 22m 2s    0B/0B       │
│ 57db0767...     172.19.0.1                 02-22 22:41   22h 21m 50s   0B/0B       │
│ 1abef930...     172.19.0.1                 02-22 22:41   22h 21m 41s   0B/0B       │
└──────────────────────────────────────────────────────────────────────────────────────┘
F1:Help  |  q:Back/Quit  |  Ctrl+Q:Exit
```

### 6.4 レイアウト（検索後・結果なし）

```
┌ Session Search ──────────────────────────────────────────────────────────────────┐
│                                                                                    │
│  IMSI: 440101234567890                                              ← 橙色/黄色  │
│  Sessions found: 0                                                                 │
│                                                                                    │
│  Press '/' to search for another IMSI                                ← 灰色      │
│                                                                                    │
├ Sessions ──────────────────────────────────────────────────────────────────────────┤
│                                                                                    │
│  No active sessions found for IMSI: 440101234567890                                │
│                                                                                    │
└──────────────────────────────────────────────────────────────────────────────────────┘
F1:Help  |  q:Back/Quit  |  Ctrl+Q:Exit
```

### 6.5 IMSI検索ダイアログ

Session Search 画面で `/` キーを押下すると、IMSI検索ダイアログが表示される。

| 項目 | 仕様 |
|------|------|
| 表示方式 | `tview.Form` ベースのモーダルダイアログ（50×7） |
| 入力フィールド幅 | 20文字（IMSIは最大15桁のため十分） |
| OKボタン | 非同期検索を開始（セクション6.9.1参照） |
| Cancelボタン | 初回検索前（IMSI未設定）の場合は Session List に戻る。検索済みの場合は Session Search 画面に留まる |
| 再検索 | `/` キーで検索ダイアログを再表示 |

### 6.6 サマリ表示項目

| 項目 | 説明 | 表示色 |
|------|------|--------|
| IMSI | 検索対象IMSI（15桁） | 橙色/黄色 |
| Sessions found | ヒットしたセッション数 | 白色 |
| 再検索案内 | `Press '/' to search for another IMSI` | 灰色 |

### 6.7 セッションテーブル表示項目

テーブルにはボーダータイトル `Sessions` が付く。

| カラム | 内容 | 表示形式 | 備考 |
|--------|------|---------|------|
| UUID | セッションUUID | 先頭8文字 + `...` | 例: `e499a25b...` |
| NAS IP | NAS IPアドレス | 可変幅 | - |
| Client IP | クライアントIPアドレス | 可変幅 | 空欄の場合あり |
| Start Time | セッション開始時刻 | `MM-DD HH:MM` | Session List と同形式 |
| Duration | セッション経過時間 | `XXh XXm XXs` | Session List と同形式 |
| In/Out | 通信量（受信/送信） | `0B/0B` 形式 | スラッシュ区切り |

### 6.8 ページネーション仕様

| 項目 | 仕様 |
|------|------|
| 1ページあたり表示件数 | 10件 |
| ナビゲーション | `←` 前ページ / `→` 次ページ |
| UI形式 | `[Prev] Page 1/1 [Next]` |

### 6.9 データ取得方式

#### 6.9.1 非同期検索パターン

検索ダイアログのOKボタン押下後、ネットワーク I/O を goroutine 内で先に実行し、UI更新のみ `QueueUpdateDraw` で行う（D-05 セクション3.11参照）。

```go
go func() {
    // goroutine内でネットワークI/Oを実行（イベントループ外）
    auditLogger.LogSearch(audit.TargetSession, imsi, 0)
    sessions, err := sessionStore.GetByIMSI(ctx, imsi)
    // UI更新のみQueueUpdateDrawで行う
    app.QueueUpdateDraw(func() {
        if err != nil {
            statusBar.ShowError("Search failed: " + err.Error())
        } else {
            render(sessions)
        }
        app.SetFocus(sessionsList)
    })
}()
```

**注記:** セッションテーブルの描画では、結果0件時に `SetSelectable(false, false)` を設定し、結果がある場合のみ `SetSelectable(true, false)` に戻すこと（D-05 セクション3.10参照）。

#### 6.9.2 処理フロー

```
1. idx:user:{IMSI} から該当セッションUUIDのセット（Set）を SMEMBERS で取得
2. UUIDが取得できた場合:
   a. 各UUIDに対して Pipeline で sess:{UUID} を HGETALL
   b. 【クリーンアップ】存在しないセッション（HGETALL結果が空）のUUIDを収集
   c. 【クリーンアップ】収集したUUIDを SREM idx:user:{IMSI} で削除
3. idx:user インデックスが空の場合（SCANフォールバック）:
   a. SCAN で全 sess:* キーを取得し、Pipeline で HGETALL
   b. 取得したセッションの IMSI フィールドで完全一致フィルタリング
4. 存在するセッションのみを start_time降順でソート
5. サマリ計算（セッション数、通信量合計）
6. ページ分割して表示
```

#### 6.9.3 SCAN フォールバック

`idx:user:{IMSI}` インデックスは auth-server が認証成功時に作成する。以下の場合にインデックスが存在しない可能性がある:

- acct-server 経由のみでセッションが作成された場合
- auth-server が認証フローを完了していない場合
- テスト環境でセッションを手動作成した場合

このため、`GetByIMSI()` は `idx:user` インデックスが空の場合に全セッション SCAN によるフォールバック検索を行う。

```go
func (s *SessionStore) GetByIMSI(ctx context.Context, imsi string) ([]*model.Session, error) {
    // idx:user:{IMSI} からUUID取得を試行
    uuids, err := s.client.SMembers(ctx, indexKey).Result()
    // ...

    // インデックスが空の場合は SCAN フォールバック
    if len(uuids) == 0 {
        return s.getByIMSIScan(ctx, imsi)
    }

    // インデックス経由の通常取得...
}

func (s *SessionStore) getByIMSIScan(ctx context.Context, imsi string) ([]*model.Session, error) {
    allSessions, err := s.List(ctx)  // SCAN + Pipeline で全セッション取得
    // IMSI フィールドで完全一致フィルタリング
    for _, sess := range allSessions {
        if sess.IMSI == imsi {
            sessions = append(sessions, sess)
        }
    }
    return sessions, nil
}
```

**注記:** SCAN フォールバックは全セッションを走査するため、セッション数が多い環境ではパフォーマンスに影響する。PoC規模（数百〜数千件）では問題ないが、大規模環境では `idx:user` インデックスの整備を前提とすること。

> **クリーンアップ処理の設計意図:**
> - `idx:user:{IMSI}` はTTLなしのSetであり、Acct-Stop未達やセッションTTL切れでゴミが残る可能性がある
> - 読み出し時に存在確認を行い、不整合を自動解消することでデータ整合性を維持する
> - 詳細はD-02「Valkeyデータ設計仕様書」を参照

**実装イメージ：**

```go
type SessionDetailItem struct {
    UUID         string
    StartTime    time.Time
    AcctID       string
    InputOctets  int64
    OutputOctets int64
}

type SessionDetailSummary struct {
    IMSI          string
    SessionCount  int
    TotalInput    int64  // octets
    TotalOutput   int64  // octets
    Items         []SessionDetailItem
}

func fetchSessionsByIMSI(ctx context.Context, rdb *redis.Client, imsi string) (*SessionDetailSummary, error) {
    // 1. インデックスからセッションUUID一覧を取得
    uuids, err := rdb.SMembers(ctx, "idx:user:"+imsi).Result()
    if err != nil {
        return nil, err
    }
    
    if len(uuids) == 0 {
        return &SessionDetailSummary{IMSI: imsi, SessionCount: 0}, nil
    }
    
    // 2. 各セッションの詳細を取得
    pipe := rdb.Pipeline()
    cmds := make(map[string]*redis.MapStringStringCmd)
    for _, uuid := range uuids {
        cmds[uuid] = pipe.HGetAll(ctx, "sess:"+uuid)
    }
    pipe.Exec(ctx)
    
    var items []SessionDetailItem
    var totalIn, totalOut int64
    
    for uuid, cmd := range cmds {
        data, err := cmd.Result()
        if err != nil || len(data) == 0 {
            continue
        }
        item := parseSessionDetail(uuid, data)
        items = append(items, item)
        totalIn += item.InputOctets
        totalOut += item.OutputOctets
    }
    
    // 3. start_time降順でソート
    sort.Slice(items, func(i, j int) bool {
        return items[i].StartTime.After(items[j].StartTime)
    })
    
    return &SessionDetailSummary{
        IMSI:         imsi,
        SessionCount: len(items),
        TotalInput:   totalIn,
        TotalOutput:  totalOut,
        Items:        items,
    }, nil
}
```

### 6.10 idx:user クリーンアップ処理

`idx:user:{IMSI}` インデックスに残存するゴミデータ（存在しないセッションへの参照）を読み出し時にクリーンアップする。

#### 6.10.1 処理フロー

```go
func fetchSessionsByIMSIWithCleanup(ctx context.Context, rdb *redis.Client, imsi string) (*SessionDetailSummary, error) {
    indexKey := "idx:user:" + imsi
    
    // 1. インデックスからセッションUUID一覧を取得
    uuids, err := rdb.SMembers(ctx, indexKey).Result()
    if err != nil {
        return nil, err
    }
    
    if len(uuids) == 0 {
        return &SessionDetailSummary{IMSI: imsi, SessionCount: 0}, nil
    }
    
    // 2. Pipeline で各セッションを取得
    pipe := rdb.Pipeline()
    cmds := make(map[string]*redis.MapStringStringCmd, len(uuids))
    for _, uuid := range uuids {
        cmds[uuid] = pipe.HGetAll(ctx, "sess:"+uuid)
    }
    _, err = pipe.Exec(ctx)
    if err != nil {
        return nil, err
    }
    
    // 3. 存在するセッションと存在しないUUIDを分類
    var sessions []SessionDetailItem
    var orphanedUUIDs []string
    
    for uuid, cmd := range cmds {
        data, err := cmd.Result()
        if err != nil {
            continue
        }
        
        if len(data) == 0 {
            // セッションが存在しない（TTL切れ等）→ クリーンアップ対象
            orphanedUUIDs = append(orphanedUUIDs, uuid)
            continue
        }
        
        sessions = append(sessions, parseSession(uuid, data))
    }
    
    // 4. 孤立したUUIDをインデックスから削除（クリーンアップ）
    if len(orphanedUUIDs) > 0 {
        // SREMは可変長引数対応
        args := make([]interface{}, len(orphanedUUIDs))
        for i, uuid := range orphanedUUIDs {
            args[i] = uuid
        }
        if err := rdb.SRem(ctx, indexKey, args...).Err(); err != nil {
            // クリーンアップ失敗はログ出力のみ（表示処理は継続）
            slog.Warn("failed to cleanup orphaned session uuids",
                "event_id", "IDX_USER_CLEANUP_ERR",
                "imsi", imsi,
                "orphaned_count", len(orphanedUUIDs),
                "error", err)
        } else {
            slog.Debug("cleaned up orphaned session uuids",
                "event_id", "IDX_USER_CLEANUP",
                "imsi", imsi,
                "orphaned_count", len(orphanedUUIDs))
        }
    }
    
    // 5. ソート・サマリ計算
    sortByStartTimeDesc(sessions)
    
    return &SessionDetailSummary{
        IMSI:         imsi,
        SessionCount: len(sessions),
        TotalInput:   sumInputOctets(sessions),
        TotalOutput:  sumOutputOctets(sessions),
        Items:        sessions,
    }, nil
}
```

#### 6.10.2 event_id定義

| event_id | レベル | 発生条件 |
|----------|--------|---------|
| `IDX_USER_CLEANUP` | DEBUG | クリーンアップ成功時 |
| `IDX_USER_CLEANUP_ERR` | WARN | クリーンアップ失敗時（表示は継続） |

#### 6.10.3 注意事項

- クリーンアップ処理の失敗は画面表示をブロックしない（ログ出力のみ）
- 大量のゴミデータがある場合、初回表示時にやや時間がかかる可能性がある
- 将来的に定期バッチでのクリーンアップが必要になった場合は、別途検討する

### 6.11 キーバインド

| キー | 動作 |
|------|------|
| `/` | IMSI検索ダイアログを表示（初回・再検索共通） |
| `PgUp` | 前のページへ |
| `PgDn` | 次のページへ |
| `r` / `F5` | 表示中IMSIの情報を再取得（入力欄の値ではなく、最後に検索したIMSI） |
| `Esc` / `q` | モニタリングメニューへ戻る |
| `F1` / `?` | ヘルプダイアログ表示 |

**リロード動作の注意：**

`r` キーによるリロードは、IMSI入力欄の現在値ではなく、最後に検索・表示したIMSIの情報を再取得する。これにより、入力欄を編集中でも現在表示中のセッション情報を更新できる。

### 6.12 入力バリデーション

前半で定義済みのIMSIバリデーションルールを適用する。

| ルール | エラーメッセージ |
|--------|-----------------|
| 15桁の数字 `/^[0-9]{15}$/` | `IMSI must be 15 digits` |

### 6.13 エラー時の表示

| 状況 | 表示内容 |
|------|---------|
| IMSI形式不正 | ステータスバーに `✗ IMSI must be 15 digits` 表示 |
| Valkey接続失敗 | ステータスバーにエラー表示、テーブルは `---` 表示または空 |
| リロード失敗 | ステータスバーにエラー表示、前回取得値を維持 |

---

## 7. ヘルプダイアログ仕様

### 7.1 概要

各画面で `?` キー押下時にモーダル表示するヘルプダイアログ。現在画面で使用可能なキーバインドを簡潔に表示する。

### 7.2 共通仕様

| 項目 | 仕様 |
|------|------|
| 表示方式 | `tview.Flex` ベースの2カラムレイアウト（`tview.Modal` は表示領域の制約があるため不採用） |
| 閉じる方法 | `Enter` キーまたは `Esc` キー |
| タイトル | `Help` |

### 7.3 レイアウト

2カラム構成で、左カラムにNavigation + Global、右カラムにList Actions + Policy Formを表示する。各操作にはファンクションキーと代替文字キー (alt) の両方が用意されている。

```
┌ Help ────────────────────────────────────────────────────────────────────────┐
│                                                                               │
│  Navigation                          List Actions                             │
│  ──────────                          ────────────                             │
│  ↑         Move up                   F2        Create new                     │
│  ↓         Move down                 n         Create new (alt)               │
│  PgUp      Page up                   F3        Edit selected                  │
│  PgDn      Page down                 e         Edit selected (alt)            │
│  Tab       Next field/item           F4        Delete selected                │
│  Shift+Tab Previous field/item       d         Delete selected (alt)          │
│  Enter     Select/Confirm            F5        Refresh list                   │
│  Esc       Back/Cancel               r         Refresh list (alt)             │
│                                      F6        Filter                         │
│  Global                              /         Filter (alt)                   │
│  ──────                                                                       │
│  F1        Show this help            Policy Form                              │
│  ?         Show this help (alt)      ───────────                              │
│  q         Back/Quit                 F6        Toggle Form/Rules focus         │
│  Ctrl+Q    Exit application                                                   │
│                                                                               │
└───────────────────────────────────────────────────────────────────────────────┘
```

**注記：** ヘルプダイアログは全画面共通のキーバインドを一覧する。画面固有の操作は各画面のフッターバーに表示される。

---

## 8. Go構造体定義

実装時に使用する構造体定義。

### 8.1 統計キャッシュ

```go
// pkg/tui/monitoring/statistics.go

type StatisticsCache struct {
    mu              sync.RWMutex
    ActiveSessions  int
    InputTotal      int64  // octets
    OutputTotal     int64  // octets
    UniqueNASIPs    int
    UniqueIMSIs     int
    UpdatedAt       time.Time
    initialized     bool
    lastError       error
}

func (c *StatisticsCache) Get() StatisticsData {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return StatisticsData{
        ActiveSessions: c.ActiveSessions,
        InputTotal:     c.InputTotal,
        OutputTotal:    c.OutputTotal,
        UniqueNASIPs:   c.UniqueNASIPs,
        UniqueIMSIs:    c.UniqueIMSIs,
        UpdatedAt:      c.UpdatedAt,
        HasData:        c.initialized,
        Error:          c.lastError,
    }
}

type StatisticsData struct {
    ActiveSessions int
    InputTotal     int64
    OutputTotal    int64
    UniqueNASIPs   int
    UniqueIMSIs    int
    UpdatedAt      time.Time
    HasData        bool
    Error          error
}
```

### 8.2 セッション一覧

```go
// pkg/tui/monitoring/session_list.go

type SessionListItem struct {
    UUID      string
    IMSI      string
    StartTime time.Time
    NasIP     string
    AcctID    string
}

func (s *SessionListItem) FormatStartTime() string {
    return s.StartTime.Format("2006-01-02 15:04:05")
}

func (s *SessionListItem) FormatAcctID(maxLen int) string {
    if len(s.AcctID) <= maxLen {
        return s.AcctID
    }
    if maxLen <= 3 {
        return s.AcctID[:maxLen]
    }
    return s.AcctID[:maxLen-3] + "..."
}

type SortMode int

const (
    SortByStartTimeDesc SortMode = iota
    SortByIMSIAsc
)
```

### 8.3 セッション詳細

```go
// pkg/tui/monitoring/session_detail.go

type SessionDetailItem struct {
    UUID         string
    StartTime    time.Time
    AcctID       string
    InputOctets  int64
    OutputOctets int64
}

func (s *SessionDetailItem) FormatStartTime() string {
    return s.StartTime.Format("2006-01-02 15:04:05")
}

func (s *SessionDetailItem) FormatAcctID(maxLen int) string {
    if len(s.AcctID) <= maxLen {
        return s.AcctID
    }
    if maxLen <= 3 {
        return s.AcctID[:maxLen]
    }
    return s.AcctID[:maxLen-3] + "..."
}

type SessionDetailSummary struct {
    IMSI          string
    SessionCount  int
    TotalInput    int64  // octets
    TotalOutput   int64  // octets
    Items         []SessionDetailItem
}

const (
    MaxSessionCount     = 999
    MaxTotalTrafficKB   = 999_999_999_999  // サマリ用
    MaxSessionTrafficKB = 999_999_999      // 個別セッション用
)
```

---

## 9. フォーマット関数

通信量表示に使用する共通フォーマット関数。

```go
// pkg/tui/format/traffic.go

package format

import (
    "strconv"
    "strings"
)

// FormatWithCommas は数値を3桁ごとにカンマ区切りで整形する
func FormatWithCommas(n int64) string {
    s := strconv.FormatInt(n, 10)
    if len(s) <= 3 {
        return s
    }
    
    var result strings.Builder
    remainder := len(s) % 3
    if remainder > 0 {
        result.WriteString(s[:remainder])
        if len(s) > remainder {
            result.WriteRune(',')
        }
    }
    for i := remainder; i < len(s); i += 3 {
        if i > remainder {
            result.WriteRune(',')
        }
        result.WriteString(s[i : i+3])
    }
    return result.String()
}

// FormatTrafficKB は通信量をKB単位で整形する（サマリ用、18桁）
func FormatTrafficKB(octets int64) string {
    kb := octets / 1024
    const maxKB int64 = 999_999_999_999
    if kb > maxKB {
        kb = maxKB
    }
    formatted := FormatWithCommas(kb)
    return fmt.Sprintf("%18s KB", formatted)
}

// FormatSessionTrafficKB は通信量をKB単位で整形する（個別セッション用、14桁）
func FormatSessionTrafficKB(octets int64) string {
    kb := octets / 1024
    const maxKB int64 = 999_999_999
    if kb > maxKB {
        kb = maxKB
    }
    formatted := FormatWithCommas(kb)
    return fmt.Sprintf("%14s KB", formatted)
}

// FormatErrorPlaceholder はエラー時のプレースホルダを返す
func FormatErrorPlaceholder(width int) string {
    return fmt.Sprintf("%*s", width, "---")
}
```

---

## 10. 監査ログ出力

前半で定義した監査ログ仕様に準拠し、モニタリング画面での特定操作についてもログを出力する。

### 10.1 記録対象操作

| 操作 | event_id | operation | 備考 |
|------|----------|-----------|------|
| Session Detail検索 | `AUDIT_LOG` | `search` | IMSIによるセッション検索 |

**注記：** 参照系操作（Statistics表示、Session List表示）は監査ログ対象外とする。

### 10.1.1 IMSI記録方針

Admin TUIの監査ログでは、**IMSIを常に生値（マスキングなし）で記録する**。

| 項目 | 方針 |
|------|------|
| `target_imsi` フィールド | IMSI全桁を記録 |
| 環境変数 | `LOG_MASK_IMSI` は参照しない |

**設計意図:**
- 監査ログはセキュリティ追跡・監査証跡として機能するため、識別情報を完全に記録する必要がある
- 管理者の操作対象を明確に特定できることを優先

### 10.2 ログ出力例

```json
{
  "time": "2025-12-25T14:30:00.000Z",
  "level": "INFO",
  "app": "admin-tui",
  "event_id": "AUDIT_LOG",
  "msg": "session search",
  "operation": "search",
  "target_type": "session",
  "target_imsi": "440101234567890",
  "result_count": 3,
  "admin_user": "admin"
}
```
> **注記:** `target_imsi` は `440101********0` ではなく `440101234567890` と全桁が記録される。

---

## 11. 未決事項・将来検討課題

| No. | 項目 | 内容 | 判断時期 |
|-----|------|------|---------|
| 1 | 大量セッション対応 | 10,000件超の場合のパフォーマンスチューニング（Session List全件取得の最適化） | PoC完了後 |
| 2 | セッション強制終了 | Valkey削除のみか、CoA/DM送信まで対応するか | PoC完了後 |
| 3 | 自動更新機能 | Auto-refresh（5秒/10秒/30秒間隔等）の実装 | PoC完了後 |
| 4 | 履歴セッション表示 | TTL超過で削除されたセッションの参照機能 | PoC完了後 |
| 5 | 高度な検索機能 | 部分一致IMSI検索、NAS-IP/Client-IPフィルタ、日時範囲指定 | PoC完了後 |

---

## 改訂履歴

| 版数 | 日付 | 内容 |
|------|------|------|
| r1 | 2026-01-04 | 初版作成（モニタリング画面：Statistics Dashboard、Session List、Session Detail、ヘルプダイアログ） |
| r2 | 2026-01-21 | 関連ドキュメント参照バージョン更新: Valkeyデータ設計仕様書 r3→r6、エラーハンドリング詳細設計書 r2→r3 |
| r3 | 2026-01-27 | IMSI記録方針明確化: セクション10.1.1新設（監査ログにIMSI生値を出力する方針を明記）、idx:userクリーンアップ処理追加（セクション6.10新設）、これに伴い旧 6.10以降の再ナンバリングを実施 |
| r4 | 2026-02-18 | 関連ドキュメント版数更新: D-05 r3→r6、D-02 r9→r10、D-06 r5→r6 |
| r5 | 2026-02-21 | 実機検証不具合修正の反映: セッションデータ読み取りをString型(GET+JSON)からHash型(HGETALL+mapToSession)に修正しAuth/Acctサーバーとのデータ型整合性を確保（セクション5.8を5.8.1-5.8.4に再構成）、Statistics Dashboardフォーカス仕様追加（セクション4.6新設、旧4.6以降再ナンバリング）、Session ListにF5キー追加、ヘルプダイアログを2カラムFlexレイアウトに変更（セクション7.3更新）、関連ドキュメント版数更新 D-05 r6→r7 |
| r6 | 2026-02-22 | Session Detail 不具合修正の反映: 検索ダイアログ仕様追加（セクション6.2拡充 — InputField幅20、Cancel時のSession List戻り動作）、データ取得方式を再構成（セクション6.9を6.9.1-6.9.3に再構成 — 非同期検索パターン、SCANフォールバック追加）、関連ドキュメント版数更新 D-05 r7→r8 |
| r7 | 2026-02-23 | 実装スクリーンショットとの整合性修正: §3.1 モニタリングメニューのボーダータイトル・ショートカット括弧表記追加、§4.2-4.3 Statistics Dashboard レイアウト差替（4カウント項目構成）、§5.3-5.5 Session List レイアウト差替（6カラム構成・Start Time短縮形式・Duration/Traffic追加・Filter Sessionsダイアログ新設）、§6 Session Detail→Session Search改名・レイアウト差替（Sessionsボーダータイトル・UUID/Duration/In-Out形式変更）、§7.3 ヘルプダイアログ全面差替（22項目・F2-F6ファンクションキー+alt文字キー）、関連ドキュメント版数更新 D-05 r8→r9 |
