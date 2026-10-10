# D-07 Admin TUI 詳細設計書【後半】(r14)

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
| D-05_Admin_TUI詳細設計書_前半_r11 | 共通仕様、キーバインド規約、ページネーション仕様、ページライフサイクル管理、tview Table Selectable状態管理、非同期データ取得パターン |
| D-02_Valkeyデータ設計仕様書 (r24) | `sess:{UUID}`, `idx:user:{IMSI}` のデータ構造 |
| D-06_エラーハンドリング詳細設計書 (r13) | TUIエラー表示仕様 |

### 1.4 PoC対象外機能

以下の機能は本PoCでは実装しない。

| 機能 | 理由 |
|------|------|
| セッション強制終了 | CoA/DM送信が必要となり複雑度が増すため |
| 自動更新（Auto-refresh） | PoC規模では手動更新で十分 |
| Session Search の部分一致IMSI検索 | 完全一致で運用可能、全件SCANのコスト回避（部分一致の絞り込みは Session List のフィルタで行う） |
| 項目を指定した条件検索（NAS-IP・Client-IP ごとの条件指定、AND/OR 等） | PoC規模では不要。Session List のフィルタ（§5.4）で IMSI・NAS IP・Client IP を対象とする部分一致の絞り込みは提供する |
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
                                     [Enter]キー（IMSI検索）
                                               ▼
                                        ┌───────────┐
                                        │[O2]       │
                                        │Session    │
                                        │Search     │
                                        └───────────┘
```

**備考:** Session Search [O2] はモニタリングメニューからは直接遷移せず、Session List でセッション行を選択して `Enter` キーを押すと遷移し、IMSI検索ダイアログが表示される（選択行のIMSIは自動入力しない）。Session List の `/` キーはフィルタ（§5.4）である。Session List にセッションが1件もない場合は `Enter` で遷移しない。

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
│ (q) Back                                                    │
│     Return to main menu                                     │
│                                                             │
└─────────────────────────────────────────────────────────────┘
F1:Help  |  q:Back/Quit  |  Ctrl+Q:Exit
```

**備考:** Session Search（旧Session Detail）はメニューに含まれない。Session List 画面でセッション行を選択して `Enter` キーを押すと遷移する。

### 3.2 キーバインド

| キー | 動作 |
|------|------|
| `1` | Statistics Dashboard画面へ |
| `2` | Session List画面へ |
| `q` / `Esc`（または `(q) Back` を選択） | メインメニューへ戻る |

---

## 4. Statistics Dashboard [O0]

### 4.1 概要

登録データ（加入者・RADIUSクライアント・ポリシー）とアクティブセッションの件数をサマリ表示する画面。統計データは1分間キャッシュし、キャッシュが古い場合は画面表示時に取得し直す（§4.5）。

### 4.2 レイアウト

tview.TextView を使用。ボーダータイトル「Statistics Dashboard」。テキスト内にセクション見出し・統計値・更新時刻・キーバインドを表示。

```
┌ Statistics Dashboard ──────────────────────────────────────────────┐
│                                                                    │
│  System Statistics                                    ← 橙色/黄色   │
│                                                                    │
│    Subscribers:      24                               ← 白色        │
│    RADIUS Clients:    6                                            │
│    Policies:          9                                            │
│    Active Sessions:   9                                            │
│                                                                    │
│  Last updated: 2026-02-23 20:54:11                    ← 灰色        │
│  (Statistics are cached for 1 minute. Press 'r' to force refresh)  │
│                                                                    │
│  Key bindings:                                        ← 橙色/黄色    │
│    r - Refresh statistics                                          │
│    q/Esc - Back to menu                                            │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
F1:Help  |  q:Back/Quit  |  Ctrl+Q:Exit
```

### 4.3 表示項目

| 項目 | 説明 | データソース |
|------|------|-------------|
| Subscribers | 登録加入者数 | `sub:*` パターンのキー数（SCAN で数える） |
| RADIUS Clients | 登録クライアント数 | `client:*` パターンのキー数（同上） |
| Policies | 登録ポリシー数 | `policy:*` パターンのキー数（同上） |
| Active Sessions | アクティブセッション数 | `sess:*` パターンのキー数（同上） |
| Last updated | 統計を取得した時刻 | `YYYY-MM-DD HH:MM:SS`（ローカル時刻） |

通信量の合計（Input/Output total）、NAS IP数・IMSI数などは表示しない。

### 4.4 通信量表示フォーマット

Statistics Dashboard は件数のみを表示し、通信量は表示しない（設計初期の `Input/Output total`（KB 表示、カンマ区切り）は実装していない）。セッションごとの通信量は Session List の `Traffic` 列（§5.5）と Session Search の `In/Out` 列（§6.7）で確認する。

### 4.5 キャッシュ仕様

実装: `apps/admin-tui/internal/store/statistics.go`（`StatisticsStore`）。

| 項目 | 仕様 |
|------|------|
| 有効期間 | 1分（60秒）。取得時刻（`UpdatedAt`、Unix秒）から1分未満ならキャッシュを返す |
| 保持場所 | メモリ内（`StatisticsStore` が保持。Admin TUI のプロセス内で共有） |
| 更新方式 | 要求時に更新する（定期的なバックグラウンド更新はしない）。画面表示時（`Get`）にキャッシュがない・古い場合は4件数を並列（goroutine）で SCAN して取得し直す |
| 手動更新 | `r` キーでキャッシュを破棄（`ClearCache`）してから取得し直す |
| 取得失敗時 | キャッシュは更新しない。画面はエラー表示（§4.8） |
| タイムスタンプ | 取得時刻を `Last updated: YYYY-MM-DD HH:MM:SS` 形式で表示 |

**実装イメージ：**

```go
// Get は統計情報を取得する（1分キャッシュ）。
func (s *StatisticsStore) Get(ctx context.Context) (*Statistics, error) {
    s.mu.RLock()
    if s.cache != nil && time.Now().Unix()-s.cache.UpdatedAt < int64(s.cacheTTL.Seconds()) {
        cached := *s.cache
        s.mu.RUnlock()
        return &cached, nil
    }
    s.mu.RUnlock()
    return s.Refresh(ctx)
}

// Refresh は加入者・クライアント・ポリシー・セッションの件数を並列に数え直し、
// すべて成功した場合のみキャッシュを更新する。
func (s *StatisticsStore) Refresh(ctx context.Context) (*Statistics, error) {
    // SubscriberStore.Count / ClientStore.Count / policy:* の SCAN / SessionStore.Count を
    // goroutine で並列に実行（各 SCAN の COUNT は 100）
    // ...
}
```

### 4.6 フォーカス仕様

Statistics Dashboard画面に遷移した際、TextViewにフォーカスを設定する。これにより、画面表示直後からキー操作（`r` キーによるリロード等）が可能となる。

### 4.7 キーバインド

| キー | 動作 |
|------|------|
| `r` | 統計情報を即時再取得・表示更新 |
| `Esc` / `q` | モニタリングメニューへ戻る |
| `F1` / `?` | ヘルプダイアログ表示（グローバルキー。D-05 §3.1） |

### 4.8 エラー時の表示

| 状況 | 表示内容 |
|------|---------|
| 統計情報の取得失敗（初回表示時） | 画面に `Error loading statistics: {エラー内容}`（赤）を表示し、ステータスバーに `✗ Failed to load: {エラー内容}` |
| 手動リロード（`r`）失敗 | 画面を `Error loading statistics: {エラー内容}` に置き換え（前回取得値は表示しない）、ステータスバーに `✗ Failed to refresh: {エラー内容}` |
| 手動リロード（`r`）成功 | ステータスバーに `✓ Statistics refreshed` |

---

## 5. Session List [O1]

### 5.1 概要

アクティブセッションの一覧を表示する画面。`s` キーでソート項目（Start Time / NAS IP / IMSI）を切り替えられる。

### 5.2 ソート項目

`s` キーを押すたびに、ソート項目が Start Time（デフォルト）→ NAS IP → IMSI → Start Time の順に切り替わる。ソートの向きはソート項目ごとに固定で、利用者が向きだけを反転する操作はない。

| ソート項目 | 向き | 値が同じ行の並び | ヘッダーのインジケータ |
|-----------|------|----------------|---------------------|
| Start Time（デフォルト） | start_time 降順（新しい順） | UUID 昇順 | `Start Time ▼` |
| NAS IP | NAS IP 昇順（文字列として比較） | start_time 降順 → UUID 昇順 | `NAS IP ▲` |
| IMSI | IMSI 昇順 | start_time 降順 → UUID 昇順 | `IMSI ▲` |

**注記：**
- 表示カラムの並び（IMSI, NAS-ID, NAS IP, Client IP, Start Time, Duration, Traffic）はソート項目によらず固定である。NAS-ID はソート項目にしない。
- NAS IP は文字列として比較するため、`172.19.0.10` は `172.19.0.9` より前に並ぶ。
- 値が同じ行は開始時刻の新しい順、さらに同じなら UUID 順に並べ、再読み込み（`r` / `F5`）のたびに表示順が変わらないようにする（§5.8.5）。
- ソート項目は Session List 画面が存在する間（再読み込み・フィルタ適用時、Session Search から戻ったときを含む）保持する。モニタリングメニューから Session List を開き直すと Start Time ▼ に戻る。

### 5.3 レイアウト

セッション一覧は既定で start_time 降順で表示する。アクティブなソート項目のヘッダーにソートインジケータ（降順 `▼` / 昇順 `▲`）を付加する（ヘッダーの文字色は他のカラムと同じ Yellow）。

```
┌ Session List 1-9 of 9 (Page 1/1) ─────────────────────────────────────────────────┐
│ IMSI             NAS-ID  NAS IP       Client IP  Start Time ▼ Duration    Traffic │
│ 001010000000003  ap-001  172.30.0.10  10.0.0.51  02-22 22:50  22h 5m 21s  759.5K  │
│ 001010000000006  ap-002  172.30.0.10             02-22 22:50  22h 5m 58s  0B      │
│ 001010000000003  -       172.19.0.1              02-22 22:43  22h 12m 36s 0B      │
│ 001010000000001  ap-001  172.30.0.10             02-22 22:43  22h 12m 46s 0B      │
│ :                :       :            :          :            :           :       │
└───────────────────────────────────────────────────────────────────────────────────┘
F1:Help  |  q:Back/Quit  |  Ctrl+Q:Exit
```

**表示形式の注記:**
- `Start Time` は短縮形式 `MM-DD HH:MM`（年・秒なし）で表示する
- `Duration` は開始時刻からの経過時間を `format.Elapsed` で `XXh XXm XXs` 形式（スペース区切り、秒あり。1時間未満は `XXm XXs`、1分未満は `XXs`）で表示する。Teal（青緑）で表示
- `Traffic` は入力・出力オクテットの合計を `format.BytesShort` のコンパクト表記（`0B`, `512B`, `1.2K`, `5.3M`, `1.0G` 等。1024 単位、小数1桁）で表示する。緑色で表示
- `Client IP` はセッションによって空欄の場合がある
- `NAS-ID` はセッションの `nas_identifier`（NAS-Identifier）を表示する。値がない（NAS-Identifier のないセッション、または `nas_identifier` を追加する前に作られたセッション）場合は `-`、24文字を超える場合は先頭21文字＋`...` で表示する（`format.OrDash` / `format.Truncate`）。radsecproxy 等のプロキシ経由では `NAS IP` がすべてプロキシのIPになるため、NAS の識別には `NAS-ID` を使う（D-08 §5.8）
- ボーダータイトルに `Session List 1-N of M (Page X/Y)` 形式でページ情報を表示する

### 5.4 フィルタダイアログ

Session List 画面で `/` または `F6` キーを押下すると、フィルタダイアログ（`ui.NewInputDialog`）が表示される。

```
              ┌ Filter Sessions ───────────────────────┐
              │                                        │
              │  IMSI/NAS-ID/IP contains: [         ] │
              │                                        │
              │       < OK >  < Cancel >               │
              │                                        │
              └────────────────────────────────────────┘
```

| 項目 | 仕様 |
|------|------|
| タイトル | `Filter Sessions` |
| フィルタ対象 | IMSI、NAS-ID、NAS IP、Client IP（部分一致）。ラベルは `IMSI/NAS-ID/IP contains:` |
| フィルタ適用時 | ボーダータイトルに `(Filter: "入力値")` を追加表示 |
| ダイアログを閉じる | `OK` で適用。`Cancel` ボタンまたは `Esc` でフィルタを変えずに閉じる（`Esc` は `tview.Form.SetCancelFunc` で `Cancel` と同じ処理を呼ぶ。D-05 §3.7） |
| クリア | 空文字で OK 押下、またはフィルタ適用中に一覧画面で `Esc` |

### 5.5 表示項目

| カラム | 内容 | 表示形式 | 備考 |
|--------|------|---------|------|
| IMSI | 加入者識別番号 | 15桁 | - |
| NAS-ID | NAS-Identifier | 可変幅（最大24文字） | 値がなければ `-` |
| NAS IP | NAS IPアドレス（パケットの送信元IP） | 可変幅 | プロキシ経由ではプロキシのIP |
| Client IP | クライアントIPアドレス | 可変幅 | 空欄の場合あり |
| Start Time | セッション開始時刻 | `MM-DD HH:MM` | 既定のソート項目（ソートインジケータ `▼`。§5.2） |
| Duration | セッション経過時間 | `XXh XXm XXs` | Teal（青緑）表示 |
| Traffic | 通信量合計（入力＋出力） | `0B`, `1.2K`, `5.3M` 等 | 緑色表示 |

**Start Time形式について：**

短縮形式 `MM-DD HH:MM`（例: `02-22 22:50`）を採用する。年・秒を省略することで、Duration・Traffic カラムを追加しても画面幅に収まる。

### 5.6 テキスト切り詰め

Session List の各カラムは切り詰めない（Acct-Session-Id は一覧に表示しない）。Session Search の UUID カラムは、UUID が8文字を超える場合に先頭8文字に `...` を付加して表示する（§6.7）。

```go
// apps/admin-tui/internal/ui/monitoring/session_detail.go（render より抜粋）
uuidDisplay := session.UUID
if len(uuidDisplay) > 8 {
    uuidDisplay = uuidDisplay[:8] + "..."
}
```

### 5.7 ページネーション仕様

前半のマスタ一覧と統一する。

| 項目 | 仕様 |
|------|------|
| 1ページあたり表示件数 | 50件 |
| ナビゲーション | `PgUp` 前ページ / `PgDn` 次ページ（`←` / `→` はページ切替に使わない） |
| UI形式・件数表示 | ボーダータイトルに `Session List 1-50 of 125 (Page 1/3)` 形式で表示（フィルタ適用時は `(Filter: "入力値")` を付加し、絞り込み後の件数で表示） |

### 5.8 データ取得方式

#### 5.8.1 処理フロー

```
1. SCAN コマンドで sess:* パターンのキーを取得（COUNT 100 で分割取得）
2. 各キーに対して Pipeline で HGETALL を実行（N+1問題回避）
3. 取得した map[string]string を `sessionFromHash`（§5.8.3）で `model.Session` に変換（Hash が空、または数値フィールドが不正なセッションはスキップ）
4. メモリ上で現在のソート項目・向きに従いソート（§5.2）
5. ページ分割して該当ページを表示
```

#### 5.8.2 セッションデータのRedis型

Auth Server / Acct Server はセッションを **Redis Hash型** で保存する（`HSET` コマンド）。Admin-TUI の SessionStore も **Hash型** で読み取る（`HGETALL` コマンド）。

| Redis Hashフィールド | 型 | model.Session フィールド | 備考 |
|---------------------|-----|------------------------|------|
| `imsi` | String | `IMSI` | 加入者識別番号 |
| `nas_ip` | String | `NasIP` | NAS IPアドレス |
| `nas_identifier` | String | `NasIdentifier` | NAS-Identifier（ない場合は空文字。D-02 §2） |
| `client_ip` | String | `ClientIP` | クライアントIPアドレス |
| `acct_id` | String | `AcctSessionID` | アカウンティングセッションID |
| `start_time` | String (数値) | `StartTime` (int64) | セッション開始時刻（Unix秒） |
| `input_octets` | String (数値) | `InputOctets` (int64) | 受信バイト数 |
| `output_octets` | String (数値) | `OutputOctets` (int64) | 送信バイト数 |

**注記：** `UUID` はHashフィールドには含まれない。Redis キー `sess:{UUID}` から `sess:` プレフィックスを除去して取得する。

#### 5.8.3 セッションの Hash の変換

`HGETALL` で取得した `map[string]string` から `model.Session`（`pkg/model`）構造体へ変換する（実装: `pkg/masterdata/session.go` の `sessionFromHash`。r13 で Admin TUI の `mapToSession` を Provisioning API と共通の `pkg/masterdata` に移した。D-13 §7.2）。数値のフィールドが空なら 0 とし、数値として解釈できない場合はエラーにする（一覧ではそのセッションを除く）。

```go
func sessionFromHash(uuid string, m map[string]string) (*model.Session, error) {
    sess := &model.Session{
        UUID:          uuid,
        IMSI:          m["imsi"],
        NasIP:         m["nas_ip"],
        NasIdentifier: m["nas_identifier"],
        ClientIP:      m["client_ip"],
        AcctSessionID: m["acct_id"],
    }
    for _, f := range []struct {
        name string
        dst  *int64
    }{
        {"start_time", &sess.StartTime},
        {"input_octets", &sess.InputOctets},
        {"output_octets", &sess.OutputOctets},
    } {
        v := m[f.name]
        if v == "" {
            continue
        }
        n, err := strconv.ParseInt(v, 10, 64)
        if err != nil {
            return nil, fmt.Errorf("invalid %s: %w", f.name, err)
        }
        *f.dst = n
    }
    return sess, nil
}
```

#### 5.8.4 セッション一覧取得

実装: `apps/admin-tui/internal/store/session.go` の `SessionStore.List`。読み出しは `pkg/masterdata.SessionStore.List`（Provisioning API の `GET /sessions` と共通。r13）に任せる。取得結果は `SessionListScreen.Load` で `s.sessions` に保持し、`sortSessions`（§5.8.5）で並べ替えてから描画する。

```go
// apps/admin-tui/internal/store/session.go
func (s *SessionStore) List(ctx context.Context) ([]*model.Session, error) {
    return s.read.List(ctx) // s.read は *masterdata.SessionStore
}

// pkg/masterdata/session.go（抜粋）
// List は全セッションを取得する（SCAN）。値を解釈できないセッションは除く。
func (s *SessionStore) List(ctx context.Context) ([]*model.Session, error) {
    var uuids []string
    iter := s.client.Scan(ctx, 0, PrefixSession+"*", 100).Iterator()
    for iter.Next(ctx) {
        uuids = append(uuids, strings.TrimPrefix(iter.Val(), PrefixSession))
    }
    if err := iter.Err(); err != nil {
        return nil, err
    }
    sessions, _, err := s.getMany(ctx, uuids) // Pipeline で一括 HGETALL
    return sessions, err
}
```

#### 5.8.5 ソート処理

ソート項目の切り替え（`s` キー）は `ToggleSort` で行う。向きはソート項目から決める（Start Time のみ降順）。

```go
// ToggleSort はソート項目を Start Time → NAS IP → IMSI の順に切り替える。
func (s *SessionListScreen) ToggleSort() {
    s.sortField = nextSortField(s.sortField)
    s.sortDesc = s.sortField == SortByStartTime
    s.sortSessions()
    s.render()
}

// nextSortField は次のソート項目を返す（Start Time → NAS IP → IMSI → Start Time）。
func nextSortField(f SortField) SortField {
    switch f {
    case SortByStartTime:
        return SortByNasIP
    case SortByNasIP:
        return SortByIMSI
    default:
        return SortByStartTime
    }
}

// sortSessions は現在のソート項目・向きで並べ替える。
// 値が同じ場合は開始時刻の新しい順に並べ、さらに同じなら UUID 順にして表示順を安定させる。
func (s *SessionListScreen) sortSessions() {
    key := func(sess *model.Session) string {
        switch s.sortField {
        case SortByIMSI:
            return sess.IMSI
        case SortByNasIP:
            return sess.NasIP
        default:
            return ""
        }
    }
    sort.SliceStable(s.sessions, func(i, j int) bool {
        a, b := s.sessions[i], s.sessions[j]
        if s.sortField != SortByStartTime {
            if ka, kb := key(a), key(b); ka != kb {
                if s.sortDesc {
                    return ka > kb
                }
                return ka < kb
            }
        }
        if a.StartTime != b.StartTime {
            if s.sortField == SortByStartTime && !s.sortDesc {
                return a.StartTime < b.StartTime
            }
            return a.StartTime > b.StartTime
        }
        return a.UUID < b.UUID
    })
}
```

`Load`（初回表示・再読み込み）でも取得後に `sortSessions` を呼び、現在のソート項目を保持したまま並べ替える。

### 5.9 キーバインド

| キー | 動作 |
|------|------|
| `PgUp` | 前のページへ |
| `PgDn` | 次のページへ |
| `↑` / `↓` | カーソル（選択行）上下移動 |
| `Enter` | 選択行があれば Session Search 画面へ遷移し、IMSI検索ダイアログを表示（§6。選択行のIMSIは自動入力しない） |
| `s` | ソート項目の切替（Start Time ▼ → NAS IP ▲ → IMSI ▲。§5.2） |
| `/` / `F6` | フィルタダイアログ表示（§5.4） |
| `r` / `F5` | 一覧を再読み込み |
| `Esc` / `q` | モニタリングメニューへ戻る（フィルタ適用中の `Esc` はフィルタ解除） |
| `F1` / `?` | ヘルプダイアログ表示（グローバルキー。D-05 §3.1） |

### 5.10 エラー時の表示

| 状況 | 表示内容 |
|------|---------|
| セッション一覧取得失敗（初回表示時） | ステータスバーに `✗ Failed to load: {エラー内容}`、一覧は空表示 |
| リロード（`r` / `F5`）失敗 | ステータスバーに `✗ Failed to refresh: {エラー内容}`、前回取得値を維持 |
| リロード成功 | ステータスバーに `✓ Refreshed` |

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
| Cancelボタン / `Esc` | 初回検索前（IMSI未設定）の場合は Session List に戻る。検索済みの場合は Session Search 画面に留まる（`Esc` は `Cancel` と同じ処理。D-05 §3.7） |
| 再検索 | `/` キーで検索ダイアログを再表示 |
| 入力値の扱い | 入力値をそのまま検索する（IMSI形式の検証はしない。§6.12） |

### 6.3 レイアウト（検索結果表示）

上部にサマリ情報（IMSI、セッション数、再検索案内）、下部に `Sessions` ボーダータイトル付きのセッションテーブルを表示する。

```
┌ Session Search ────────────────────────────────────────────────────────────────────┐
│                                                                                    │
│  IMSI: 001010000000000                                              ← 橙色/黄色      │
│  Sessions found: 5                                                                 │
│                                                                                    │
│  Press '/' to search for another IMSI                                ← 灰色         │
│                                                                                    │
├ Sessions ──────────────────────────────────────────────────────────────────────────┤
│ UUID          NAS-ID  NAS IP       Client IP  Start Time   Duration     In/Out     │
│ e499a25b...   ap-001  172.30.0.10  10.0.0.51  02-22 22:40  22h 22m 22s  0B/0B      │
│ e44f5786...   ap-001  172.30.0.10             02-22 22:40  22h 22m 16s  0B/0B      │
│ 4673e913...   -       172.19.0.1              02-22 22:41  22h 22m 2s   0B/0B      │
│ 57db0767...   ap-002  172.30.0.10             02-22 22:41  22h 21m 50s  0B/0B      │
│ 1abef930...   ap-001  172.30.0.10             02-22 22:41  22h 21m 41s  0B/0B      │
└────────────────────────────────────────────────────────────────────────────────────┘
F1:Help  |  q:Back/Quit  |  Ctrl+Q:Exit
```

### 6.4 レイアウト（検索後・結果なし）

```
┌ Session Search ────────────────────────────────────────────────────────────────────┐
│                                                                                    │
│  IMSI: 440101234567890                                              ← 橙色/黄色     │
│  Sessions found: 0                                                                 │
│                                                                                    │
│  Press '/' to search for another IMSI                                ← 灰色         │
│                                                                                    │
├ Sessions ──────────────────────────────────────────────────────────────────────────┤
│                                                                                    │
│  No sessions found                                                                 │
│                                                                                    │
└────────────────────────────────────────────────────────────────────────────────────┘
F1:Help  |  q:Back/Quit  |  Ctrl+Q:Exit
```

### 6.5 IMSI検索ダイアログ

Session Search 画面に遷移した直後（§6.2 初期状態）と、Session Search 画面で `/` キーを押下したときに、IMSI検索ダイアログ（ボーダータイトル `Search Sessions by IMSI`、入力欄 `Enter IMSI:`、前回の検索IMSIを初期値として表示）が表示される。

| 項目 | 仕様 |
|------|------|
| 表示方式 | `tview.Form` ベースのモーダルダイアログ（50×7） |
| 入力フィールド幅 | 20文字（IMSIは最大15桁のため十分） |
| OKボタン | 非同期検索を開始（セクション6.9.1参照） |
| Cancelボタン / `Esc` | 初回検索前（IMSI未設定）の場合は Session List に戻る。検索済みの場合は Session Search 画面に留まる（`Esc` は `Cancel` と同じ処理） |
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
| NAS-ID | NAS-Identifier | 可変幅（最大24文字） | Session List と同形式（値がなければ `-`） |
| NAS IP | NAS IPアドレス | 可変幅 | - |
| Client IP | クライアントIPアドレス | 可変幅 | 空欄の場合あり |
| Start Time | セッション開始時刻 | `MM-DD HH:MM` | Session List と同形式 |
| Duration | セッション経過時間 | `XXh XXm XXs` | Session List と同形式 |
| In/Out | 通信量（受信/送信） | `0B/0B` 形式 | スラッシュ区切り |

### 6.8 ページネーション仕様

| 項目 | 仕様 |
|------|------|
| ページ分割 | しない。検索結果の全件を1つのテーブルに表示する |
| スクロール | セッションテーブルにフォーカスがあるとき `↑` / `↓`（`PgUp` / `PgDn` はテーブル内のスクロール） |

### 6.9 データ取得方式

#### 6.9.1 非同期検索パターン

検索ダイアログのOKボタン押下後、ネットワーク I/O を goroutine 内で先に実行し、UI更新のみ `QueueUpdateDraw` で行う（D-05 セクション3.11参照）。

```go
go func() {
    // goroutine内でネットワークI/Oを実行（イベントループ外）
    sessions, err := sessionStore.GetByIMSI(ctx, imsi)
    // 検索の実行後に監査ログを出力（結果件数、または失敗理由を記録。§10）
    auditLogger.LogSearch(audit.TargetSession, imsi, len(sessions), err)
    // UI更新のみQueueUpdateDrawで行う
    app.QueueUpdateDraw(func() {
        if err != nil {
            statusBar.ShowError("Search failed: " + err.Error())
        } else {
            // 開始時刻の新しい順（同じなら UUID 順）に並べてから表示（§6.9.2）
            render(sortByStartTimeDesc(sessions))
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
4. 画面側（`SessionDetailScreen`）で、取得したセッションを start_time 降順（同じなら UUID 昇順）に並べ替える（`sortByStartTimeDesc`。`GetByIMSI` 自体は並べ替えない。`idx:user` の Set から取り出した順は不定のため）
5. サマリ表示（IMSI、セッション数）
6. 全件を1つのテーブルに表示（ページ分割なし。§6.8）
```

#### 6.9.3 SCAN フォールバック

`idx:user:{IMSI}` インデックスは auth-server が認証成功時に作成する。以下の場合にインデックスが存在しない可能性がある:

- acct-server 経由のみでセッションが作成された場合
- auth-server が認証フローを完了していない場合
- テスト環境でセッションを手動作成した場合

このため、IMSI での読み出し（`pkg/masterdata.SessionStore.ListByIMSI`。r13）は `idx:user` インデックスが空の場合に全セッション SCAN によるフォールバック検索を行う。

```go
// pkg/masterdata/session.go（抜粋）
func (s *SessionStore) ListByIMSI(ctx context.Context, imsi string) (sessions []*model.Session, stale []string, err error) {
    uuids, err := s.client.SMembers(ctx, UserIndexKey(imsi)).Result()
    if err != nil {
        return nil, nil, err
    }
    // インデックスが空の場合は SCAN フォールバック（全セッションを走査して IMSI で絞り込む）
    if len(uuids) == 0 {
        all, err := s.List(ctx)
        // ...
        for _, sess := range all {
            if sess.IMSI == imsi {
                sessions = append(sessions, sess)
            }
        }
        return sessions, nil, nil
    }
    // インデックス経由の通常取得。存在しない UUID は stale として返す（消さない）
    return s.getMany(ctx, uuids)
}
```

**注記:** SCAN フォールバックは全セッションを走査するため、セッション数が多い環境ではパフォーマンスに影響する。PoC規模（数百〜数千件）では問題ないが、大規模環境では `idx:user` インデックスの整備を前提とすること。

> **クリーンアップ処理の設計意図:**
> - `idx:user:{IMSI}` はTTLなしのSetであり、Acct-Stop未達やセッションTTL切れでゴミが残る可能性がある
> - 読み出し時に存在確認を行い、不整合を自動解消することでデータ整合性を維持する
> - 詳細はD-02「Valkeyデータ設計仕様書」を参照

**並べ替え（画面側）：**

実装: `apps/admin-tui/internal/ui/monitoring/session_detail.go`。`GetByIMSI` の結果（§6.10.1）をそのまま表示せず、検索後に開始時刻の新しい順に並べ替える。サマリはIMSIとセッション数のみで、通信量の合計は計算しない（§6.6）。

```go
// sortByStartTimeDesc は検索結果を開始時刻の新しい順に並べる（同じなら UUID 順）。
// idx:user の Set から取り出した順は不定なため、表示順をそろえる。
func sortByStartTimeDesc(sessions []*model.Session) []*model.Session {
    sort.SliceStable(sessions, func(i, j int) bool {
        if sessions[i].StartTime != sessions[j].StartTime {
            return sessions[i].StartTime > sessions[j].StartTime
        }
        return sessions[i].UUID < sessions[j].UUID
    })
    return sessions
}
```

### 6.10 idx:user クリーンアップ処理

`idx:user:{IMSI}` インデックスに残存するゴミデータ（存在しないセッションへの参照）を読み出し時にクリーンアップする。

#### 6.10.1 処理フロー

実装: `apps/admin-tui/internal/store/session.go` の `SessionStore.GetByIMSI`。読み出しは `pkg/masterdata.SessionStore.ListByIMSI`（§6.9.3）で行い、存在しないセッションの UUID（stale）だけを受け取って、Admin TUI がインデックスから消す（r13。Provisioning API の `GET /sessions` は同じ読み出しを使うが、消さない）。

```go
// GetByIMSI は指定されたIMSIのセッションリストを取得する（idx:user経由）。
// 存在しないセッションはインデックスからクリーンアップする。
func (s *SessionStore) GetByIMSI(ctx context.Context, imsi string) ([]*model.Session, error) {
    // 1〜2. idx:user:{IMSI} の UUID を Pipeline で読み、存在しない UUID を stale として受け取る
    //       （インデックスが空なら SCAN フォールバック。§6.9.3）
    sessions, stale, err := s.read.ListByIMSI(ctx, imsi)
    if err != nil {
        return nil, err
    }
    // 3. 存在しないセッションをインデックスから削除（UUIDごとに SREM。失敗はログのみで表示は継続）
    indexKey := UserIndexKey(imsi)
    for _, uuid := range stale {
        if err := s.client.SRem(ctx, indexKey, uuid).Err(); err != nil {
            log.Printf("failed to cleanup stale session from index: imsi=%s, uuid=%s, err=%v", imsi, uuid, err)
        }
    }
    return sessions, nil
}
```

#### 6.10.2 ログ出力

クリーンアップ処理は `event_id` 付きの構造化ログ（JSON）を出力しない（D-04 §3.5.1）。

| 状況 | 出力 | 備考 |
|------|------|------|
| クリーンアップ成功 | なし | - |
| クリーンアップ失敗（`SREM` エラー） | Go標準 `log.Printf` によるテキスト1行（標準エラー出力。JSONではない） | 画面表示は継続。UUIDごとに1行。IMSIは生値 |

**ログ出力例（失敗時）:**

```text
2026/01/27 10:30:00 failed to cleanup stale session from index: imsi=440101234567890, uuid=550e8400-e29b-41d4-a716-446655440000, err=...
```

#### 6.10.3 注意事項

- クリーンアップ処理の失敗は画面表示をブロックしない（`log.Printf` によるテキスト出力のみ）
- 大量のゴミデータがある場合、初回表示時にやや時間がかかる可能性がある
- 将来的に定期バッチでのクリーンアップが必要になった場合は、別途検討する

### 6.11 キーバインド

| キー | 動作 |
|------|------|
| `/` | IMSI検索ダイアログを表示（再検索。同じIMSIで再検索すれば最新の情報を再取得できる） |
| `Tab` | サマリ部分とセッションテーブルの間でフォーカスを切替 |
| `Esc` | セッションテーブルではサマリ部分へフォーカスを戻す。サマリ部分では Session List へ戻る |
| `q` | Session List へ戻る |
| `F1` / `?` | ヘルプダイアログ表示（グローバルキー。D-05 §3.1） |

**注記：** 再読み込み専用のキー（`r` / `F5`）とページ切替（`PgUp` / `PgDn` によるページ移動）はない。Session Search から戻る先はモニタリングメニューではなく Session List（ソート・フィルタの状態を保持）である。

### 6.12 入力バリデーション

IMSI検索ダイアログの入力値は検証しない。15桁の数字でない値（空文字を含む）もそのまま検索し、該当がなければ `Sessions found: 0` / `No sessions found` となる。

### 6.13 エラー時の表示

| 状況 | 表示内容 |
|------|---------|
| 検索失敗（Valkey接続失敗等） | ステータスバーに `✗ Search failed: {エラー内容}`、表示中の検索結果は維持 |
| 結果なし | サマリに `Sessions found: 0`、テーブルに `No sessions found` |

---

## 7. ヘルプダイアログ仕様

### 7.1 概要

各画面で `F1` または `?` キー押下時にモーダル表示するヘルプダイアログ。現在画面で使用可能なキーバインドを簡潔に表示する。入力欄（`tview.InputField` / `tview.TextArea`）にフォーカスがあるときの `?` は文字として入力され、ヘルプは開かない（`F1` は開く。D-05 §3.1）。

### 7.2 共通仕様

| 項目 | 仕様 |
|------|------|
| 表示方式 | `tview.Flex` ベースの2カラムレイアウト（`tview.Modal` は表示領域の制約があるため不採用） |
| 閉じる方法 | `Enter` キーまたは `Esc` キー |
| タイトル | `Help` |

### 7.3 レイアウト

2カラム構成で、左カラムにNavigation + Global、右カラムにList Actions + Policy List + Policy Formを表示する（Policy List は r14 で追加。認可ポリシーの一覧の停止・再開。D-05 §4.4.1）。各操作にはファンクションキーと代替文字キー (alt) の両方が用意されている。

```
┌ Help ─────────────────────────────────────────────────────────────────────────┐
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
│  F1        Show this help            Policy List                              │
│  ?         Show this help (alt)      ───────────                              │
│  q         Back/Quit                 F7 / s    Suspend/Resume selected        │
│  Ctrl+Q    Exit application                                                   │
│                                      Policy Form                              │
│                                      ───────────                              │
│                                      F6        Toggle Form/Rules focus        │
│                                                                               │
└───────────────────────────────────────────────────────────────────────────────┘
```

**注記：** ヘルプダイアログは全画面共通のキーバインドを一覧する。画面固有の操作は各画面のフッターバーに表示される。

---

## 8. Go構造体定義

モニタリング画面の実装で使用する主な型（抜粋）。セッションのデータは画面ごとの専用構造体に変換せず、`pkg/model.Session` をそのまま使う。

### 8.1 統計キャッシュ

```go
// apps/admin-tui/internal/store/statistics.go

// Statistics は統計情報を表す。
type Statistics struct {
    SubscriberCount int64 `json:"subscriber_count"`
    ClientCount     int64 `json:"client_count"`
    PolicyCount     int64 `json:"policy_count"`
    SessionCount    int64 `json:"session_count"`
    UpdatedAt       int64 `json:"updated_at"` // Unix秒
}

// StatisticsStore は統計情報へのアクセスを提供する（1分キャッシュ。§4.5）。
type StatisticsStore struct {
    subscriberStore *masterdata.SubscriberStore // pkg/masterdata（E-03 §9）
    clientStore     *masterdata.ClientStore
    policyStore     *masterdata.PolicyStore    // 件数は PolicyStore.Count
    sessionStore    *SessionStore

    mu       sync.RWMutex
    cache    *Statistics
    cacheTTL time.Duration // 1分
}
```

### 8.2 セッション一覧

```go
// pkg/model/session.go

// Session はRADIUSセッション情報を表す（Valkeyキー: sess:{UUID}。§5.8.2）。
type Session struct {
    UUID          string `json:"uuid"`            // セッション識別子（キーから取得）
    IMSI          string `json:"imsi"`            // 加入者IMSI
    NasIP         string `json:"nas_ip"`          // NAS IPアドレス（パケットの送信元IP。プロキシ経由ではプロキシのIP）
    NasIdentifier string `json:"nas_identifier"`  // NAS-Identifier（プロキシ経由でもNASを識別できる）
    ClientIP      string `json:"client_ip"`       // クライアントIPアドレス
    AcctSessionID string `json:"acct_session_id"` // アカウンティングセッションID（Hash の acct_id）
    StartTime     int64  `json:"start_time"`      // セッション開始時刻（Unix秒）
    InputOctets   int64  `json:"input_octets"`    // 受信バイト数
    OutputOctets  int64  `json:"output_octets"`   // 送信バイト数
}
```

```go
// apps/admin-tui/internal/ui/monitoring/session_list.go

// SortField はソート項目を表す（向きは sortDesc。§5.2）。
type SortField int

const (
    SortByIMSI      SortField = iota // IMSI（昇順）
    SortByStartTime                  // 開始時刻（降順。デフォルト）
    SortByNasIP                      // NAS IP（昇順）
)

// SessionListScreen はセッション一覧画面を表す。
type SessionListScreen struct {
    table        *tview.Table
    app          *ui.App
    sessionStore *store.SessionStore
    sessions     []*model.Session
    filter       *ui.Filter     // 対象: IMSI / NAS-ID / NAS IP / Client IP（§5.4）
    pagination   *ui.Pagination // 50件/ページ（§5.7）
    sortField    SortField      // 既定: SortByStartTime
    sortDesc     bool           // 既定: true
    onSelect     func(uuid string) // Enter で Session Search へ
    onBack       func()
}
```

### 8.3 セッション詳細

```go
// apps/admin-tui/internal/ui/monitoring/session_detail.go

// SessionDetailScreen はセッション詳細画面（Session Search）を表す。
type SessionDetailScreen struct {
    flex         *tview.Flex
    textView     *tview.TextView   // サマリ（IMSI、Sessions found、再検索案内）
    sessionsList *tview.Table      // セッションテーブル（ページ分割なし。§6.8）
    app          *ui.App
    sessionStore *store.SessionStore
    auditLogger  *audit.Logger     // search の監査ログ（§10）
    imsi         string            // 最後に検索したIMSI（検索ダイアログの初期値）
    sessions     []*model.Session  // sortByStartTimeDesc で並べ替え済みの検索結果
    onBack       func()            // Session List へ戻る
}
```

サマリは IMSI とセッション数のみで、通信量の合計やセッション数・通信量の上限値（カウントストップ）は設けていない。

---

## 9. フォーマット関数

モニタリング画面の表示に使用するフォーマット関数（`apps/admin-tui/internal/format`）。

| 関数 | 出力例 | 使用箇所 |
|------|-------|---------|
| `DateTime(unixSec)` | `2026-02-23 20:54:11`（ローカル時刻） | Statistics Dashboard の `Last updated` |
| `DateTimeShort(unixSec)` | `02-22 22:50`（ローカル時刻） | Session List / Session Search の `Start Time` |
| `Elapsed(startUnixSec)` | `22h 5m 21s`、`5m 3s`、`42s` | Session List / Session Search の `Duration`（現在時刻との差を `Duration` で整形） |
| `BytesShort(bytes)` | `0B`、`512B`、`1.2K`、`5.3M`、`1.0G`、`2.0T` | Session List の `Traffic`（入力＋出力の合計）、Session Search の `In/Out` |

```go
// apps/admin-tui/internal/format/bytes.go

// BytesShort はバイト数を短い形式にフォーマットする。
// 例: 1024 -> "1.0K", 1048576 -> "1.0M"
func BytesShort(bytes int64) string {
    const (
        _          = iota
        kb float64 = 1 << (10 * iota)
        mb
        gb
        tb
    )

    b := float64(bytes)

    switch {
    case b >= tb:
        return fmt.Sprintf("%.1fT", b/tb)
    case b >= gb:
        return fmt.Sprintf("%.1fG", b/gb)
    case b >= mb:
        return fmt.Sprintf("%.1fM", b/mb)
    case b >= kb:
        return fmt.Sprintf("%.1fK", b/kb)
    default:
        return fmt.Sprintf("%dB", bytes)
    }
}
```

```go
// apps/admin-tui/internal/format/time.go

// Duration は秒数を人間が読みやすい形式にフォーマットする（例: 3661 -> "1h 1m 1s"）。
func Duration(seconds int64) string {
    if seconds < 0 {
        return "-"
    }
    d := time.Duration(seconds) * time.Second
    hours := int(d.Hours())
    minutes := int(d.Minutes()) % 60
    secs := int(d.Seconds()) % 60
    if hours > 0 {
        return fmt.Sprintf("%dh %dm %ds", hours, minutes, secs)
    }
    if minutes > 0 {
        return fmt.Sprintf("%dm %ds", minutes, secs)
    }
    return fmt.Sprintf("%ds", secs)
}

// Elapsed は開始時刻からの経過時間を計算してフォーマットする。
func Elapsed(startUnixSec int64) string {
    elapsed := time.Now().Unix() - startUnixSec
    return Duration(elapsed)
}
```

**注記：** 設計初期の KB 単位・カンマ区切りの通信量表示（`FormatWithCommas`、`FormatTrafficKB`、`FormatSessionTrafficKB`）とエラー時の `---` プレースホルダ（`FormatErrorPlaceholder`）は実装していない。`format` パッケージには、このほか `Bytes`（`1.00 KB` 形式）、`RFC3339`、`DurationShort`、`Truncate` / `TruncateMiddle` / `PadRight` / `PadLeft` があるが、モニタリング画面では使用していない。

---

## 10. 監査ログ出力

前半で定義した監査ログ仕様に準拠し、モニタリング画面での特定操作についてもログを出力する。

### 10.1 記録対象操作

| 操作 | event_id | operation | 備考 |
|------|----------|-----------|------|
| Session Detail検索 | `AUDIT_LOG` | `search` | IMSIによるセッション検索。検索の実行後に出力し、検索したIMSIを `target_imsi`、結果件数を `result_count` に記録する（検索に失敗した場合も記録する） |

**注記：** 参照系操作（Statistics表示、Session List表示）は監査ログ対象外とする。

**記録内容（`search`）：**

| フィールド | 内容 |
|-----------|------|
| `target_type` | `session` |
| `target_key` | `""`（空文字） |
| `target_imsi` | 検索ダイアログに入力したIMSI（入力値をそのまま全桁で記録） |
| `result_count` | 検索結果のセッション数（数値。0件も `0` として出力）。検索に失敗した場合は出力しない |
| `details` | 検索に失敗した場合のみ `search failed: <理由>`（`GetByIMSI` のエラー）。成功時は出力しない |

監査ログは `audit.Logger.LogSearch(targetType, query, resultCount, searchErr)` で出力する。`targetType` が `session` 以外の場合（現状は呼び出し元なし）は、検索語を `details` に記録し `target_imsi` は出力しない（検索に失敗した場合は `{検索語}; search failed: <理由>`）。

### 10.1.1 IMSI記録方針

Admin TUIの監査ログでは、**IMSIを常に生値（マスキングなし）で記録する**。

| 項目 | 方針 |
|------|------|
| `target_imsi` フィールド | IMSI全桁を記録（加入者・ポリシーの作成/更新/削除時、加入者の停止・再開（`suspend` / `resume`。r14。D-05 §4.4.1）、Session Detail検索（`search`）で検索したIMSI） |
| 環境変数 | `LOG_MASK_IMSI` は参照しない |

**設計意図:**
- 監査ログはセキュリティ追跡・監査証跡として機能するため、識別情報を完全に記録する必要がある
- 管理者の操作対象を明確に特定できることを優先

### 10.2 ログ出力例

```json
{
  "time": "2025-12-25T14:30:00Z",
  "level": "INFO",
  "app": "admin-tui",
  "event_id": "AUDIT_LOG",
  "msg": "session searched",
  "operation": "search",
  "target_type": "session",
  "target_key": "",
  "target_imsi": "440101234567890",
  "admin_user": "admin",
  "result_count": 2
}
```

検索に失敗した場合（例: Valkey接続エラー）:

```json
{
  "time": "2025-12-25T14:31:00Z",
  "level": "INFO",
  "app": "admin-tui",
  "event_id": "AUDIT_LOG",
  "msg": "session searched",
  "operation": "search",
  "target_type": "session",
  "target_key": "",
  "target_imsi": "440101234567890",
  "admin_user": "admin",
  "details": "search failed: dial tcp 127.0.0.1:6379: connect: connection refused"
}
```
> **注記:** 検索したIMSIは `target_imsi` に `440101********0` ではなく `440101234567890` と全桁が記録される。`time` は RFC3339（秒精度・UTC）。`result_count` は検索に成功した場合のみ出力する（0件も `0` として出力）。各操作で出力するフィールドは D-04 §3.5 を参照。

---

## 11. 未決事項・将来検討課題

| No. | 項目 | 内容 | 判断時期 |
|-----|------|------|---------|
| 1 | 大量セッション対応 | 10,000件超の場合のパフォーマンスチューニング（Session List全件取得の最適化） | PoC完了後 |
| 2 | セッション強制終了 | Valkey削除のみか、CoA/DM送信まで対応するか | PoC完了後 |
| 3 | 自動更新機能 | Auto-refresh（5秒/10秒/30秒間隔等）の実装 | PoC完了後 |
| 4 | 履歴セッション表示 | TTL超過で削除されたセッションの参照機能 | PoC完了後 |
| 5 | 高度な検索機能 | Session Search の部分一致IMSI検索、項目を指定した条件検索（NAS-IP・Client-IP ごとの条件指定）、日時範囲指定（IMSI・NAS IP・Client IP を対象とする部分一致の絞り込みは Session List のフィルタで実装済み） | PoC完了後 |

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
| r8 | 2026-10-04 | D-04 r19 の event_id 全面整合に合わせて修正: §6.10.1 クリーンアップ処理のコード例を実装（`SessionStore.GetByIMSI`。インデックス空時のSCANフォールバック、UUIDごとのSREM）に合わせて修正し、実装に存在しない event_id `IDX_USER_CLEANUP` / `IDX_USER_CLEANUP_ERR` を削除。§6.10.2 を「event_id定義」から「ログ出力」に改め、成功時はログなし・失敗時は `log.Printf` の非構造化テキスト（標準エラー出力）のみである旨と出力例を記載。§10.1.1/§10.2 の監査ログ（`AUDIT_LOG`）の検索時の記録内容を実装に合わせて修正（検索IMSIは `details` に記録、`target_imsi`・`result_count` なし、msg `session searched`、`time` は秒精度）。§1.3 関連ドキュメントの版数を現行版に更新（D-02 r12、D-06 r7） |
| r9 | 2026-10-04 | Admin TUI の監査ログに検索IMSIと件数を正しく記録する実装修正の反映: §10.1 に Session Detail 検索（`search`）の記録内容の表を追加（検索したIMSIを `details` ではなく `target_imsi` に全桁で記録、検索の実行後に出力し結果件数を `result_count` に記録、検索に失敗した場合は `result_count` を出さず `details` に `search failed: <理由>`。session 以外の検索は検索語を `details` に記録）。§10.1.1 の `details` の行（検索IMSIを `details` に記録、`target_imsi` は出力しない）を削除し `target_imsi` の行に検索IMSIを追記。§10.2 の出力例を `target_imsi` / `result_count` に修正し、検索失敗時の例を追加。§6.9.1 非同期検索パターンのコード例を、検索の実行後に `LogSearch(audit.TargetSession, imsi, len(sessions), err)` を呼ぶ形に修正。§1.3 関連ドキュメントの版数を現行版に更新（D-05 r10、D-02 r17、D-06 r13） |
| r10 | 2026-10-04 | Admin TUI のキー配線漏れを修正した実装修正の反映: §5.1 / §5.2 Session List のソートを、元設計の2モード（start_time 降順 / IMSI 昇順、`i` / `t` キーで切替、表示カラム順が変わる）から現行の `s` キーによる3項目の切り替え（Start Time ▼ → NAS IP ▲ → IMSI ▲。向きは項目ごとに固定、表示カラム順は固定、同値は start_time 降順 → UUID 昇順）に修正。§5.3 / §5.5 ソートインジケータを `▼`（降順）/ `▲`（昇順）に修正（緑色の記述を削除）。§5.8.4 のソート関数例（`sortByStartTimeDesc` / `sortByIMSIAsc`）を削除し、§5.8.5 ソート処理（`ToggleSort` / `nextSortField` / `sortSessions`）を新設。§8.2 の `SortMode` を実装の `SortField`（IMSI / StartTime / NasIP）に修正。§5.4 フィルタダイアログを `F6` でも開けること、`Cancel` ボタンまたは `Esc` で閉じることを追記。§6.2 / §6.5 IMSI検索ダイアログの `Cancel` に `Esc` を追加。§4.7 Statistics の `?` を `F1` / `?`（グローバルキー）に、`Esc` を `Esc` / `q` に修正。§5.9 に `s` キーを追加。§7.1 ヘルプは `F1` / `?` で開き、入力欄では `?` が文字として入力される旨を追記。§1.3 関連ドキュメントの版数を更新（D-05 r11）。あわせて、キー操作・画面遷移の記述を実装（`main.go`、`internal/ui/monitoring`）に合わせて修正: §2.2 / §3.1 Session List から Session Search への遷移キーを `/` から `Enter` に修正（`/` はフィルタ）。§3.2 モニタリングメニューの `(q) Back` を追記。§4.8 Statistics のエラー表示を実装（画面に `Error loading statistics`、ステータスバーに `Failed to load` / `Failed to refresh`。`---` 表示・前回値の維持はない）に修正。§5.7 ページネーションのナビゲーションを `←` / `→` から `PgUp` / `PgDn` に、UI形式をボーダータイトルのページ情報に修正。§5.9 に `↑` / `↓` と `Enter`（Session Search へ遷移）を追加。§5.10 エラー表示のメッセージを実装に修正。§6.2 / §6.5 IMSI検索ダイアログのタイトル・入力欄・表示タイミングと、入力値を検証しないことを追記。§6.4 結果なしの表示を `No sessions found` に修正。§6.8 Session Search はページ分割しない（全件を1テーブル）ことに修正し、§6.9.2 の処理フローも合わせて修正。§6.11 Session Search のキーから実装にない `PgUp` / `PgDn`（ページ切替）・`r` / `F5`（再取得）を削除し、`Tab`・`Esc`（テーブル→サマリ、サマリ→Session List）・`q`（Session List へ）に修正。§6.12 / §6.13 の IMSI 形式検証（`IMSI must be 15 digits`）を削除し、検索失敗時の表示（`Search failed`）に修正。Session Search の検索結果を画面側で開始時刻の新しい順に並べる実装修正（`session_detail.go` に `sortByStartTimeDesc` を追加。`GetByIMSI` は並べ替えない）の反映: §6.9.1 のコード例を検索後に `sortByStartTimeDesc` を適用する形に、§6.9.2 の手順4を画面側での並べ替え（start_time 降順、同値は UUID 昇順）に修正し、§6.9.3 の設計初期の実装イメージ（`SessionDetailSummary`・`fetchSessionsByIMSI`。通信量合計の計算・取得側でのソート）を `sortByStartTimeDesc` に差し替え。§1.4 PoC対象外の「NAS-IP / Client-IP フィルタ」を、Session List のフィルタで IMSI・NAS IP・Client IP の部分一致の絞り込みを提供していることに合わせて「項目を指定した条件検索」に改め、§11 No.5 も同様に修正。§4 Statistics Dashboard を実装（`store/statistics.go`）に合わせて修正: §4.1 概要を件数のサマリに、§4.3 にデータ取得方法（SCAN）と Last updated を追記し、§4.4 通信量は表示しないことを明記、§4.5 キャッシュ仕様を要求時更新の1分キャッシュ（バックグラウンド定期更新なし、`r` で `ClearCache`、失敗時はキャッシュを更新しない）と実装イメージに差し替え、§8.1 を実装の `Statistics` / `StatisticsStore` に差し替え。設計初期のまま残っていたコード片・型名を実装に合わせて修正: §5.3 / §5.5 Duration の表示色を Teal に、Traffic の表記を `format.BytesShort` の `1.2K` / `5.3M` 形式に修正。§5.6 実装にない Acct-ID の切り詰め（`truncateAcctID`）を削除し、Session Search の UUID の短縮表示に差し替え。§5.8.4 `fetchAllSessions`（`SessionListItem`）を実装の `SessionStore.List` に差し替え。§8.2 / §8.3 の設計初期の構造体（`SessionListItem` / `SessionDetailItem` / `SessionDetailSummary`、上限値の定数）を、実装の `pkg/model.Session`、`SessionListScreen`・`SortField`、`SessionDetailScreen` に差し替え。§9 の KB 単位・カンマ区切りのフォーマット関数（`FormatWithCommas` / `FormatTrafficKB` / `FormatSessionTrafficKB` / `FormatErrorPlaceholder`）を、実装の `internal/format`（`DateTime` / `DateTimeShort` / `Elapsed`・`Duration` / `BytesShort`）に差し替え |
| r11 | 2026-10-06 | セッションに NAS-Identifier（`nas_identifier`）を記録した実装修正の反映（radsecproxy 等のプロキシ経由では NAS IP がプロキシのIPになり NAS を区別できないため）: Session List（§5.2 注記、§5.3 レイアウト、§5.5 表示項目）と Session Search（§6.3 レイアウト、§6.7 表示項目）に NAS-ID カラムを追加（IMSI / UUID の次。値がなければ `-`、24文字を超えれば省略）。§5.4 フィルタの対象に NAS-ID を追加しラベルを `IMSI/NAS-ID/IP contains:` に変更。§5.8.2 の Hash フィールド対応、§5.8.3 の mapToSession、構造体定義に `nas_identifier` / `NasIdentifier` を追加。NAS-ID はソート項目にしない。§1.3 の D-02 の版数を r20 に更新 |
| r12 | 2026-10-07 | Admin TUI の加入者・RADIUSクライアント・認可ポリシーの store と validation を pkg に移した実装修正（Provisioning API（D-13）と共通で使うため。E-03 r11）の反映: §8.1 の StatisticsStore の構造体で、加入者・クライアント・ポリシーのストアを `pkg/masterdata` の型に修正（ポリシーの件数は `PolicyStore.Count`） |
| r13 | 2026-10-09 | セッションの読み出しを Provisioning API（D-13 r6 の `GET /sessions`）と共通の `pkg/masterdata.SessionStore` に移した実装修正の反映: §5.8.3（Hash の変換。`sessionFromHash`）、§5.8.4（一覧の取得）、§6.9.3（SCAN フォールバック。`ListByIMSI`）、§6.10.1（インデックスのクリーンアップ。stale を受け取って Admin TUI が SREM する）の説明と抜粋、§5.8.1 の手順の関数名を更新。§1.3 の D-02 の版数を r24 に更新。Admin TUI の動作は変えていない |
| r14 | 2026-10-10 | 加入者の停止・再開（D-05 r16）の反映: §7.3 のヘルプのレイアウトの右カラムに `Policy List`（`F7 / s  Suspend/Resume selected`）を追加。§10.1.1 の `target_imsi` を記録する操作に停止・再開（`suspend` / `resume`）を追加 |
