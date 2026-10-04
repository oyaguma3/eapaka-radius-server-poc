# B-03 VPSデプロイ手順書（AWS Lightsail）(r1)

## 1. 概要

### 1.1 目的

本ドキュメントは、本PoC環境を VPS（AWS Lightsail の Ubuntu インスタンス）に、まっさらな状態から構築・デプロイする手順を提供する。

B-01（ホストOS構築）・B-02（アプリケーションデプロイ）は、LAN 内に置くミニPC（物理コンソールあり、AP と同じ LAN）を前提にしている。VPS はインターネットに直接さらされ、物理コンソールがなく、クラウド側のファイアウォールがある。本書は、B-01・B-02 の手順のうち VPS で**変わるところ・追加するところ**を、実施する順に示す。変わらない手順は B-01・B-02 の該当節を参照する。

### 1.2 スコープ

**本書で扱う範囲：**

- Lightsail インスタンスの作成、静的IP、ファイアウォール（IPv4 / IPv6）
- 初回ログイン、スワップ、作業ユーザー（`admin`）の作成
- B-01 / B-02 を VPS で実施するときの差分（SSH、時刻同期、UFW、イメージのビルド、Admin TUI のビルド、ログディレクトリ）
- RADIUS（1812/1813/udp）を AP のグローバルIPに限って公開する手順
- Lightsail の自動スナップショットによるホスト外への退避
- VPS 固有のチェックリストとトラブルシューティング

**本書で扱わない範囲：**

- RADIUS over TLS（RadSec。PoC のスコープ外。D-01 §1）、VPN 経由の構成
- IPv6 で RADIUS を受けること（クライアント登録は IPv4 アドレスのみ。§6）
- 接続方式01（aka-only-server）の接続（B-02 §14 を参照。同一ホストに置く場合も手順は同じ）

### 1.3 検証状況

本書の手順は **机上確認** である（2026-10-04 時点で Lightsail の実機では通していない）。次の点は、検証機（Debian 13、Docker Compose）で実際に確認した。

- `develop` の compose 一式のビルド・起動、テストベクターモードでの認証・再同期、公開ポート（24224 / 6379 が 127.0.0.1 のみ）
- `RADIUS_SECRET` を空にしたときに、登録のない送信元が破棄されること（B-02 §4.2）
- ログディレクトリが 775 だと logrotate がローテーションを飛ばし、755 で通ること（§5.6）
- Go を入れずに golang コンテナで Admin TUI をビルドできること（§5.8）
- バックアップの取得とリストア（O-04 §4.2）

Lightsail・Ubuntu 24.04 に固有の点（コンソールの操作、ファイアウォール、`ubuntu` ユーザー、SSH のソケット起動）は、AWS・Ubuntu の公式情報をもとに記載した。実機で異なる点があれば本書を更新すること。

### 1.4 前提となるソースの版

本書は、インターネット公開に向けた安全面の修正（fluent-bit の 24224 を 127.0.0.1 のみで公開、`RADIUS_SECRET` を任意にして既定は空、起動ログの `radius_secret_fallback`、RADIUS パケットの認証をハンドラーで行う修正）を含む版を前提とする。§5.5 でクローンした後に、§5.5 の確認コマンドで、これらが含まれていることを必ず確認すること。含まれていない古い版をそのまま公開サーバーに置くと、`.env.example` の `RADIUS_SECRET` の例の値がフォールバックのシークレットとして有効になる等、危険である。

### 1.5 関連ドキュメント

| ドキュメント | 参照内容 |
|-------------|---------|
| B-01 ホストOS構築手順書 (r5) | ホストOS設定（本書と差分のない手順） |
| B-02 アプリケーションデプロイ手順書 (r18) | デプロイ手順（本書と差分のない手順） |
| D-01 ミニPC版設計仕様書 (r17) | §4 インターネット越しに RADIUS を受ける場合の注意 |
| D-08 インフラ設定・運用設計書 (r20) | §5.4 Docker の公開ポートと UFW、§5.7 インターネット越しの RADIUS、§8 セキュリティチェックリスト |
| O-01 操作ガイド (r7) | §4.5 登録する IP アドレスと共有シークレット |
| O-03 障害対応手順書 (r12) | 認証できない場合の切り分け（§4.3.4 `RADIUS_AUTH_ERR` 等） |
| O-04 バックアップ・リストア手順書 (r2) | バックアップ・リストア、ホスト外への退避 |
| O-05 ログ解析ガイド (r18) | §8.5 RADIUS パケットの破棄の調査 |

### 1.6 対象読者

AWS アカウントを持ち、Lightsail コンソールと Linux CLI の基本操作ができるインフラ担当者。

---

## 2. 構成と方針

### 2.1 構成

```
                    インターネット
                          │
  AP（NAS） ─ ルーター ──┤ UDP 1812/1813（送信元: AP のグローバルIP）
  管理端末 ───────────────┤ TCP 22（送信元: 管理端末のグローバルIP）
                          ▼
            ┌─ Lightsail ファイアウォール（IPv4 / IPv6）─┐
            │  Lightsail インスタンス（Ubuntu 24.04 LTS）  │
            │   静的IP                                      │
            │   Docker Compose（B-02 と同じ6サービス）       │
            │   公開: 1812/udp, 1813/udp のみ                │
            │   127.0.0.1 のみ: 6379（Valkey）、24224        │
            └───────────────────────────────────────────────┘
```

### 2.2 ミニPC（B-01 / B-02）との違いと方針

| 項目 | ミニPC（B-01 / B-02） | VPS（本書） | 理由 |
|------|----------------------|------------|------|
| OS の用意 | USB からインストール | Lightsail の Ubuntu 24.04 LTS（OS のみ）で作成 | — |
| 作業ユーザー | インストール時に `admin` を作成 | 既定の `ubuntu` でログインし、`admin` を作成して以降は `admin` で作業 | B-01 / B-02 の systemd・logrotate・crontab は `/home/admin` を前提にしているため |
| SSH ポート | 10022 に変更 | **22 のまま**。Lightsail のファイアウォールで送信元を管理端末のIPに限る | Lightsail のブラウザ SSH（復旧手段）は 22 番の規則でだけ許可できる。物理コンソールがないので、締め出されたときの復旧手段を残す |
| 主なファイアウォール | UFW | **Lightsail のファイアウォール**（UFW は任意） | Docker の公開ポートは UFW を素通りする（D-08 §5.4）。Lightsail のファイアウォールはインスタンスの外で効く |
| RADIUS の送信元 | 同じ LAN の AP | AP のグローバル IPv4 アドレス（NAT の内側ならルーターのIP） | O-01 §4.5。クライアント登録は IPv4 のみ |
| `RADIUS_SECRET` | 任意（空を推奨） | **空にする** | 登録のない送信元を受け付けないため（B-02 §4.2） |
| メモリ | 8GB | 2GB 以上を推奨（1GB ならスワップ必須） | イメージのビルド（Go のコンパイル）と Valkey（maxmemory 512MB） |
| Admin TUI | 開発機でビルドして転送 | 開発機でビルドして転送、または VPS 上で golang コンテナを使ってビルド | VPS に Go を入れなくてよい |
| バックアップのホスト外退避 | 環境に応じて | Lightsail の自動スナップショット | O-04 §2.3 |
| 時刻 | systemd-timesyncd | イメージの既定（同期していればそのまま） | クラウドのイメージは chrony 等で同期済みのことがある |

### 2.3 事前に用意するもの

| 項目 | 内容 |
|------|------|
| AWS アカウント | Lightsail を利用できること |
| 管理端末のグローバルIP | SSH を許可する送信元。`curl -s https://checkip.amazonaws.com` 等で確認 |
| AP のグローバルIP | RADIUS を許可する送信元。AP が NAT の内側ならルーターのグローバルIP（固定IPが望ましい）。AP と同じルーターの配下にある端末で `curl -s https://checkip.amazonaws.com` を実行すると確認できる。IPv4 アドレスであること（§6） |
| AP ごとの共有シークレット | 長いランダム値（`openssl rand -base64 24` 等）。AP 側の設定とクライアント登録（O-01 §4）に使う |
| SSH 鍵ペア | 管理端末の鍵を Lightsail に登録するか、Lightsail が作成する既定の鍵をダウンロードする |

---

## 3. Lightsail インスタンスの作成

### 3.1 インスタンスの作成

Lightsail コンソール（`https://lightsail.aws.amazon.com/`）で **Create instance** を選び、次のとおり設定する。

| 項目 | 設定値 | 備考 |
|------|-------|------|
| リージョン / AZ | 例: 東京（ap-northeast-1） | AP からの遅延が小さいところ |
| プラットフォーム | Linux/Unix | — |
| ブループリント | **OS Only** → **Ubuntu 24.04 LTS** | アプリ入りのブループリントは使わない |
| SSH キーペア | 管理端末の公開鍵をアップロード、または既定の鍵を使う | 既定の鍵を使う場合はダウンロードして管理端末に保存（`chmod 600`） |
| ネットワークタイプ | **デュアルスタック**（IPv4 + IPv6） | IPv6 専用プランは公開 IPv4 アドレスがないため使わない |
| プラン | メモリ **2GB 以上** を推奨 | 1GB 以下にする場合は §4.2 のスワップを必ず作る |
| インスタンス名 | 例: `eapaka-radius-poc` | — |

### 3.2 静的IPのアタッチ

インスタンスを停止・起動すると公開IPが変わるため、静的IPをアタッチする（AP の RADIUS サーバーの宛先になる）。

1. Lightsail コンソールでインスタンスを選び、**Networking** タブを開く。
2. **Attach static IP**（または **Networking** 画面の **Create static IP**）で静的IPを作成し、インスタンスにアタッチする。
3. 表示された静的IP（以下 `<VPSのIP>`）を控える。

> **注記:** インスタンスにアタッチしていない静的IPは課金対象になる。インスタンスを削除する場合は静的IPも解放すること。

### 3.3 ファイアウォールの初期設定

新しい Ubuntu インスタンスの IPv4・IPv6 ファイアウォールには、既定で「SSH（TCP 22）」と「HTTP（TCP 80）」がすべての送信元に開いている。2つのファイアウォールは独立しているので、**両方とも**設定する。

**Networking** タブの **IPv4 Firewall** で次のとおりにする。

| 操作 | アプリケーション | プロトコル | ポート | 送信元 |
|------|----------------|----------|-------|-------|
| 編集 | SSH | TCP | 22 | **Restrict to IP address** を選び、管理端末のグローバルIPを指定。**Allow Lightsail browser SSH/RDP** を選ぶ（ブラウザ SSH を復旧手段として残す） |
| 削除 | HTTP | TCP | 80 | — （本PoC では使わない） |

**IPv6 Firewall** で次のとおりにする。

| 操作 | アプリケーション | プロトコル | ポート | 送信元 |
|------|----------------|----------|-------|-------|
| 削除（または管理端末の IPv6 アドレスに制限） | SSH | TCP | 22 | 管理端末から IPv6 で SSH しないなら削除 |
| 削除 | HTTP | TCP | 80 | — |

> **注意:** RADIUS（1812/1813/udp）の規則はまだ追加しない。アプリのデプロイとクライアント登録が終わってから §6 で追加する。

> **注記:** Lightsail のファイアウォールは許可規則だけを持ち、同じポートに複数の規則があると最も緩い規則が適用される。送信元を限った規則と「すべての送信元」の規則が並ばないようにすること。

---

## 4. 初回ログインとホストの準備

### 4.1 初回ログイン

管理端末から、既定ユーザー `ubuntu` でログインする。

```bash
ssh -i ~/.ssh/<秘密鍵ファイル> ubuntu@<VPSのIP>
```

> **注記:** ログインできない場合は、Lightsail コンソールのインスタンス画面の **Connect using SSH**（ブラウザ SSH）で入れる。§3.3 で **Allow Lightsail browser SSH/RDP** を選んでいれば、送信元を限っていてもブラウザ SSH は使える。

OS とアーキテクチャを確認する。

```bash
lsb_release -a     # Ubuntu 24.04 LTS
uname -m           # x86_64
free -h            # メモリ
df -h /            # ディスク
```

### 4.2 スワップの作成

メモリ 2GB 未満のプランでは必須、2GB でも推奨する。初回のイメージビルド（Go のコンパイル）でメモリが不足して失敗するのを防ぐ。

```bash
# 既存のスワップを確認（何も表示されなければスワップなし）
swapon --show

# 2GB のスワップファイルを作成して有効化
sudo fallocate -l 2G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile

# 再起動後も有効にする
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab

# 確認
swapon --show
free -h
```

### 4.3 手動スナップショットの取得（推奨）

SSH や UFW の設定（§5.2・§5.3）を誤ると、ブラウザ SSH を含めて SSH で入れなくなることがある。Lightsail にはシリアルコンソールがなく、その場合の復旧はスナップショットからインスタンスを作り直すことになる（§8.2）。設定を変更する前に、手動スナップショットを取っておく。

1. Lightsail コンソールでインスタンスを選び、**Snapshots** タブを開く。
2. **Create snapshot** で名前（例: `before-ssh-settings`）を付けて取得する。

### 4.4 作業ユーザー `admin` の作成

B-01 / B-02 の systemd ユニット・logrotate・crontab・各手順は `/home/admin` を前提にしている。`admin` ユーザーを作成し、以降の作業は `admin` で行う。

> **注記:** Ubuntu 24.04 の `adduser` では `--gecos` が非推奨になっている場合がある（警告が出るだけで動作する）。警告が出る場合は `--comment ""` を使ってもよい。

```bash
# admin ユーザーを作成（パスワードは sudo で使う。SSH はパスワードでは入れない）
sudo adduser --gecos "" admin
sudo usermod -aG sudo admin

# ubuntu ユーザーの公開鍵を admin にコピー
sudo mkdir -p /home/admin/.ssh
sudo cp /home/ubuntu/.ssh/authorized_keys /home/admin/.ssh/authorized_keys
sudo chown -R admin:admin /home/admin/.ssh
sudo chmod 700 /home/admin/.ssh
sudo chmod 600 /home/admin/.ssh/authorized_keys
```

管理端末の**別ターミナル**から、`admin` で公開鍵認証のログインができることを確認する。確認できるまで `ubuntu` のセッションは切断しない。

```bash
ssh -i ~/.ssh/<秘密鍵ファイル> admin@<VPSのIP>
sudo -v   # admin のパスワードで sudo できること
```

> **注記:** ブラウザ SSH は `ubuntu` ユーザーで接続する。`ubuntu` ユーザーは削除せずに残しておく（復旧手段）。

---

## 5. B-01 / B-02 の実施（VPS での差分）

以降は `admin` で作業する。B-01 §4 から順に実施し、本節に挙げた節は本節の手順に置き換える。挙げていない節は B-01 / B-02 のとおりに実施する。

> **注意（B-01 §4.1 のパッケージ更新）:** Lightsail はブラウザ SSH のために `/etc/ssh/sshd_config` に設定を追記している（`TrustedUserCAKeys` 等。要実機確認）。`apt upgrade` で openssh-server が更新されると、「ローカルで変更された設定ファイルの新しい版がある」という確認画面が出ることがある。その場合は **現在インストールされているローカル版を保持する（keep the local version currently installed）** を選ぶこと。配布元の版に置き換えると、ブラウザ SSH が使えなくなるおそれがある。本書が sshd の設定を `sshd_config` 本体ではなくドロップインファイル（§5.2）で行うのも、この理由による。

### 5.1 B-01 §4.3 NTP同期設定

B-01 §4.2（タイムゾーンを Asia/Tokyo に設定）を実施した後、時刻が同期していることを確認する。

```bash
timedatectl
# System clock synchronized: yes
# NTP service: active
```

- 上のとおりであれば、B-01 §4.3（systemd-timesyncd の有効化）は不要。クラウドのイメージは chrony など別の時刻同期サービスで同期済みのことがあり、その場合に systemd-timesyncd を追加で有効にしない（同期サービスが重複する）。
- 同期していない場合は、動いている時刻同期サービスを確認する（`systemctl status chrony systemd-timesyncd`）。どちらもなければ B-01 §4.3 を実施する。
- タイムゾーンを変更した後は、cron が新しいタイムゾーンで動くように cron を再起動する（`sudo systemctl restart cron`）。バックアップ（B-02 §9.4、毎日 03:00）の時刻に関わる。

### 5.2 B-01 §5.1 SSH設定

**ポートは変更しない（22 のまま）。** §2.2 のとおり、送信元の制限は Lightsail のファイアウォールで行う。公開鍵は §4.4 で配置済みである。

sshd の設定はドロップインファイルで行う（Ubuntu のクラウドイメージは `/etc/ssh/sshd_config.d/` の設定を読み込み、同じ項目は先に読まれた値が使われる）。

```bash
sudo tee /etc/ssh/sshd_config.d/10-eapaka.conf > /dev/null <<'EOF'
PermitRootLogin no
PasswordAuthentication no
PubkeyAuthentication yes
EOF

# 構文チェック
sudo sshd -t

# 設定値の確認（実際に適用される値）
sudo sshd -T | grep -E '^(port|permitrootlogin|passwordauthentication|pubkeyauthentication)'
# port 22
# permitrootlogin no
# passwordauthentication no
# pubkeyauthentication yes

# 反映（ポートを変えないので ssh.socket の再起動は不要）
sudo systemctl restart ssh
```

管理端末の**別ターミナル**から `admin` でログインできることを確認してから、現在のセッションを閉じる。

> **注記（ポートを変える場合）:** どうしてもポートを変える場合は、先に Lightsail のファイアウォールに新しいポートの規則（送信元を限定）を追加する。Ubuntu 24.04 の sshd はソケット起動のため、`sshd_config` の `Port` を変えた後は `sudo systemctl daemon-reload && sudo systemctl restart ssh.socket` が必要（`systemctl restart ssh` だけでは反映されない。B-01 §5.1）。反映後は `sudo ss -lnt | grep -E ':(22|<新しいポート>) '` で実際の待ち受けポートを確認する（`sshd -T` は設定ファイルの値を表示するだけ）。ブラウザ SSH は 22 番の規則でしか許可できないので、ポートを変えるとブラウザ SSH は使えなくなる。

### 5.3 B-01 §5.2 UFW設定（任意）

Lightsail のファイアウォールが主な防御になるため、UFW は任意とする。ホスト上の Docker 以外のプロセスが誤ってポートを開けた場合の備えとして有効にする場合は、**SSH のポートを 22 に読み替えて**実施する。

```bash
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow 22/tcp          # B-01 の 10022/tcp ではなく 22/tcp
sudo ufw show added            # 22/tcp ALLOW を確認してから有効化
sudo ufw enable
```

> **注記:** B-01 §5.2 の 1812/udp・1813/udp の許可規則は、Docker の公開ポートには効かない（Docker が UFW を素通りする。D-08 §5.4）。RADIUS の送信元の制限は §6 の Lightsail のファイアウォールで行う。

### 5.4 B-01 §6〜§8

B-01 §6（Docker 導入）、§7（運用ツール導入）、§8（systemd サービス設定）はそのまま実施する。B-01 §6.2 で `admin` を docker グループに追加した後は、いったんログアウトして入り直す。

B-01 §9（構築後チェックリスト）は、次のとおり読み替える。

| # | 確認項目 | VPS での期待値 |
|---|---------|---------------|
| 3 | NTP同期 | `timedatectl` で `System clock synchronized: yes`（§5.1） |
| 4 | SSHポート | 22 |
| 8〜11 | UFW | UFW を有効にした場合だけ。SSH は 22/tcp |

### 5.5 B-02 §3 リポジトリクローン

```bash
cd ~
git clone https://github.com/oyaguma3/eapaka-radius-server-poc.git eapaka-radius-server-poc
cd eapaka-radius-server-poc
git log --oneline -1
```

クローンした版に、§1.4 の安全面の修正が含まれていることを確認する。

```bash
grep -n '^RADIUS_SECRET' deployments/.env.example            # 何も表示されないこと（既定はコメントアウト）
grep -n '127.0.0.1:24224' deployments/docker-compose.yml     # 2行表示されること
grep -n 'radius_secret_fallback' apps/auth-server/main.go    # 表示されること
grep -n 'InsecureSkipVerify' apps/auth-server/internal/server/server.go   # 表示されること
```

いずれかが期待どおりでなければ、`main` にまだ反映されていない。`develop` を使う場合は `git checkout develop` で切り替えて再確認する（`develop` は統合ブランチであり、`main` への反映後は `main` を使うことを推奨する）。

### 5.6 B-02 §4〜§5 .env とログディレクトリ

B-02 §4 のとおり `.env` を作成する。VPS では次の点を必ず守る。

- `RADIUS_SECRET` は**設定しない（空のまま）**。登録のない送信元からのパケットを受け付けないため（B-02 §4.2）。
- `TEST_VECTOR_ENABLED` は**設定しない（false のまま）**。
- `VALKEY_PASSWORD` は `openssl rand -base64 32` 等で生成した値にする。
- `.env` のパーミッションを 600 にする（B-02 §4.4）。

設定後、次のコマンドで確認する。

```bash
cd ~/eapaka-radius-server-poc/deployments
grep -E '^(RADIUS_SECRET|TEST_VECTOR_ENABLED)=' .env      # 何も表示されないこと
grep -c 'your_secure_password_here' .env                 # 0 であること（例の値のままにしない）
ls -l .env                                               # -rw------- であること
```

B-02 §5 のとおりログ出力ディレクトリを 755 にする（git clone したディレクトリは、Ubuntu の既定の umask（002）で 775 になり、そのままだと logrotate がローテーションを飛ばす）。

```bash
chmod 755 ~/eapaka-radius-server-poc/deployments/logs_on_host
```

### 5.7 B-02 §6.1 イメージビルド

メモリが少ないプランでは、4つのアプリのビルド（Go のコンパイル）が並行して走るとメモリが不足することがある。1つずつビルドする。

```bash
cd ~/eapaka-radius-server-poc/deployments
for s in auth-server acct-server vector-gateway vector-api; do
  docker compose build "$s" || break
done
docker compose pull valkey fluent-bit
```

メモリ 2GB 以上かつスワップありなら、B-02 §6.1 の `docker compose build` でもよい。ビルドが `signal: killed` 等で失敗した場合は §8.3 を参照。

以降、B-02 §6.2〜§6.5（起動、起動確認、ヘルスチェック、systemd 起動テスト）はそのまま実施する。

### 5.8 B-02 §7 Admin TUI の配置

開発機でビルドして転送する場合は B-02 §7.1 のとおり。ただし `scp` の `-P 10022` は付けず（22 番を使う）、宛先を `admin@<VPSのIP>:~/admin-tui` にし、`-i <秘密鍵ファイル>` で鍵を指定する。

開発機を使わずに VPS 上でビルドする場合は、golang コンテナでビルドする（Go をホストに入れる必要はない）。

```bash
cd ~/eapaka-radius-server-poc
docker run --rm --user "$(id -u):$(id -g)" -e HOME=/tmp -e CGO_ENABLED=0 \
  -v "$PWD":/src -w /src golang:1.25-bookworm \
  go build -buildvcs=false -o /src/admin-tui-linux ./apps/admin-tui
mv admin-tui-linux ~/admin-tui
chmod +x ~/admin-tui
```

- `--user` で自分のユーザーとして実行し、出来上がったファイルが自分の所有になるようにする。`-buildvcs=false` は、コンテナ内で git の VCS 情報を取得できずにビルドが失敗するのを避けるためである（root で実行した検証では `error obtaining VCS status` で失敗した）。
- 使い終わった golang イメージは `docker image rm golang:1.25-bookworm` で削除してよい（次のイメージビルドで再び取得される）。

動作確認は B-02 §7.3 のとおりだが、VPS では Valkey のパスワードをコマンドラインに直接書かない（シェルの履歴に残るため）。`.env` を読み込んでから起動する。

```bash
set -a; . ~/eapaka-radius-server-poc/deployments/.env; set +a
~/admin-tui
```

### 5.9 B-02 §7.4 RADIUS クライアントの登録

Admin TUI で、AP の送信元IP（サーバーから見たIP）をクライアント登録する（O-01 §4.2）。

- AP が NAT の内側にある場合は、ルーターのグローバルIPを登録する。同じ NAT の内側の AP は同じ IP として扱われる（O-01 §4.5）。
- 共有シークレットは AP ごとに長いランダム値にし、AP 側と同じ値を設定する。
- AP のグローバルIPは §2.3 の方法で確認する。§6 のファイアウォールを設定した後に認証できず、auth-server に `RADIUS_NO_SECRET` が出る場合は、その `src_ip` が登録すべき IP である（O-01 §4.5）。
- **VPS では Docker ブリッジのゲートウェイIP（例 `172.18.0.1`）をクライアント登録しない。** これは同じホストから 127.0.0.1 宛てに試験する場合（T-03 §2.2）だけに使う登録で、公開サーバーに残すと、ゲートウェイIPに見える経路で届いたパケットがそのシークレットで受け付けられるおそれがある。試験で登録した場合は削除する。

### 5.10 B-02 §8〜§11

B-02 §8（logrotate）、§9（バックアップ）、§10（lnav フォーマット）はそのまま実施する。§9.4 の crontab は `/home/admin` のパスのままでよい。§11（デプロイ後チェックリスト）の #11 は §5.8 の起動方法、#20 は SSH のポートを 22 に読み替える。

運用中の更新（B-02 §12）や接続方式01の証明書の配置（B-02 §14.3）では、次のとおり読み替える。

| B-02 の節 | VPS での読み替え |
|-----------|----------------|
| §12.3 イメージの再ビルド | メモリが少ないプランでは §5.7 のとおり1つずつビルドする |
| §12.4 Admin TUI の再配置 | §5.8 のとおり（開発機から転送するなら `-P 10022` を付けず `-i` で鍵を指定、宛先は `<VPSのIP>`。VPS 上でビルドするなら golang コンテナ） |
| §14.3 証明書の配置 | `scp` の `-P 10022` を付けず、`-i` で鍵を指定し、宛先は `<VPSのIP>` |

---

## 6. RADIUS の公開

クライアント登録（§5.9）が終わったら、Lightsail のファイアウォールに RADIUS の規則を追加する。

**IPv4 Firewall** に次の2つを追加する。

| アプリケーション | プロトコル | ポート | 送信元 |
|----------------|----------|-------|-------|
| Custom | UDP | 1812 | **Restrict to IP address** で AP のグローバルIP（複数なら CIDR や範囲で） |
| Custom | UDP | 1813 | 同上 |

- 送信元を「すべて」にしない。RADIUS は共有シークレットだけで守られており、送信元を絞ればスキャンや総当たりの的になりにくい（D-08 §5.7）。
- **IPv6 Firewall には RADIUS の規則を追加しない。** クライアント登録（Admin TUI）は IPv4 アドレスしか受け付けないため、IPv6 で届いた RADIUS は処理できない。また、IPv6 で公開ポートに届いた通信は、コンテナから見た送信元IPが Docker ブリッジのゲートウェイIPに置き換わる可能性がある（要実機確認）。
- AP の RADIUS サーバーの宛先は `<VPSのIP>`（静的IP）、認証ポート 1812、課金ポート 1813、共有シークレットは §5.9 で登録した値にする。

追加後、AP から端末を接続して認証を確認する（O-03 の各ログで確認できる）。認証できない場合は §8.4 を参照。

---

## 7. ホスト外への退避（自動スナップショット）

B-02 §9 のバックアップは同じディスクに残るため、インスタンスの障害では失われる（O-04 §2.3）。Lightsail の自動スナップショットを有効にする。

1. Lightsail コンソールでインスタンスを選び、**Snapshots** タブを開く。
2. **Automatic snapshots** を有効にし、取得時刻を選ぶ。B-02 §9.4 の Valkey バックアップ（03:00 JST）より後の時刻にすると、その日のバックアップを含められる。Lightsail の取得時刻の表示はタイムゾーンに注意する。
3. 直近7世代が保持される（Lightsail の仕様）。スナップショットの保存容量に応じて課金される。

> **注記:** スナップショットはインスタンス全体（OS、`.env`、Ki / OPc を含む Valkey のデータ、バックアップファイル）を含む。他のアカウントへの共有やコピーは行わないこと。インスタンスを削除してもスナップショットは残り、保存容量の課金が続く。不要になったら削除する。スナップショットから新しいインスタンスを作った場合は、静的IPのアタッチとファイアウォールの設定（§3.2〜§3.3、§6）をやり直す。

---

## 8. 運用・トラブルシューティング

### 8.1 VPS 固有のチェックリスト

B-01 §9・B-02 §11 に加えて確認する。D-08 §8（セキュリティチェックリスト）の「UFW が有効化されている」「不要なポートが閉じている（ufw status）」「SSH ポートが変更されている」は、VPS では本表の #2・#3（Lightsail のファイアウォール）と SSH ポート 22 で読み替える。

`docker compose` を使う確認は `cd ~/eapaka-radius-server-poc/deployments` で実行する。

| # | 確認項目 | 確認方法 | 期待値 |
|---|---------|---------|-------|
| 1 | 静的IP | Lightsail コンソールの Networking タブ | 静的IPがアタッチされている |
| 2 | IPv4 ファイアウォール | 同上 | SSH 22 は管理端末のIPのみ（ブラウザ SSH 許可）、1812/1813 UDP は AP のIPのみ、HTTP 80 なし |
| 3 | IPv6 ファイアウォール | 同上 | 規則がない（SSH・HTTP を削除、RADIUS は追加しない） |
| 4 | 公開ポート（ホスト） | `sudo ss -lntup` | 0.0.0.0 / [::] で待ち受けているのは 1812/udp・1813/udp と SSH（22/tcp）だけ（インターフェース名付きの DHCP クライアント（68/udp、546/udp）と、ループバック（127.0.0.53 の DNS 等）は対象外） |
| 5 | フォールバック無効 | `docker compose logs auth-server acct-server \| grep 起動開始` | `"radius_secret_fallback":false` |
| 6 | テストベクターモード無効 | `docker compose logs vector-api \| grep "starting vector-api"` | `"test_mode":false` |
| 7 | スワップ | `swapon --show` | /swapfile が有効（メモリ 2GB 未満なら必須） |
| 8 | 自動スナップショット | Lightsail コンソールの Snapshots タブ | 有効 |
| 9 | 再起動後の自動起動 | `sudo reboot` の後、`docker compose ps` | 全サービスが起動している |

### 8.2 SSH で入れなくなった場合

- Lightsail コンソールのブラウザ SSH（**Connect using SSH**、`ubuntu` ユーザー）で入り、`/etc/ssh/sshd_config.d/10-eapaka.conf` や `~admin/.ssh/authorized_keys` を確認する。
- 管理端末のグローバルIPが変わった場合は、Lightsail のファイアウォールの SSH 規則の送信元を更新する。
- ブラウザ SSH も使えない場合（SSH 規則で **Allow Lightsail browser SSH/RDP** を選んでいない等）は、規則を編集して許可する。
- ブラウザ SSH も 22 番への SSH なので、sshd が起動しない、UFW で 22 番を閉じた等の場合は、ブラウザ SSH でも入れない。Lightsail にはシリアルコンソールがないため、§4.3 や §7 のスナップショットから新しいインスタンスを作成して復旧する（作成時に起動スクリプトで設定を直すこともできる）。新しいインスタンスには静的IPを付け替え、ファイアウォール（§3.3・§6）を設定し直す。

### 8.3 イメージのビルドが失敗する（メモリ不足）

- `signal: killed` や `exit code: 137` でビルドが止まる場合はメモリ不足である。§4.2 のスワップを作成し、§5.7 のとおり1つずつビルドする。
- `dmesg | grep -i -E "killed process|out of memory"` で OOM Killer の記録を確認できる。

### 8.4 AP から認証できない

1. **パケットが届いているか**: 認証は `docker compose logs auth-server | grep -E "PKT_RECV|RADIUS_NO_SECRET"`、課金は `docker compose logs acct-server | grep -E "ACCT_|RADIUS_AUTH_ERR|RADIUS_NO_SECRET"`（acct-server の `PKT_RECV` は Status-Server のときだけ出る）。何も出なければ、Lightsail の IPv4 ファイアウォール（送信元IP）と AP の宛先（静的IP、ポート）を確認する。
2. **送信元IPが登録と違う**: `RADIUS_NO_SECRET` の `src_ip` が登録したIPと違えば、その IP でクライアント登録をやり直す（NAT のグローバルIPが変わった等）。
3. **シークレットの不一致**: Access-Request は auth-server の `PKT_MA_INVALID`（D-04 §3.1.3）、Accounting-Request は acct-server の `RADIUS_AUTH_ERR`（O-03 §4.3.4）が出る（O-05 §8.5）。AP とクライアント登録の共有シークレットを合わせる。
4. **その先の認証の失敗**: O-03 の切り分け手順に従う。

### 8.5 時刻がずれている

`timedatectl` で同期していない場合は、§5.1 のとおり時刻同期サービスを確認する。Lightsail のインスタンスは外部の NTP サーバー（UDP 123、送信方向）に接続できる。

---

## 改訂履歴

| 版数 | 日付 | 内容 |
|------|------|------|
| r1 | 2026-10-04 | 初版作成（机上確認）。AWS Lightsail の Ubuntu 24.04 LTS に、B-01 / B-02 の手順をもとに構築・デプロイする手順を作成。インスタンスの作成（デュアルスタック、メモリ 2GB 以上推奨）、静的IP、IPv4 / IPv6 ファイアウォール（SSH は 22 のまま管理端末のIPに限りブラウザ SSH を許可、HTTP 80 を削除、RADIUS は AP のグローバルIPに限って後から追加）、スワップ、`admin` ユーザーの作成、B-01 / B-02 の差分（時刻同期の確認、sshd のドロップイン、UFW は任意、ログディレクトリ 755、1つずつのイメージビルド、golang コンテナでの Admin TUI のビルド、`RADIUS_SECRET` を空に）、自動スナップショット、VPS 固有のチェックリストとトラブルシューティングを記載。検証機（Debian 13）で compose 一式・logrotate の権限・Admin TUI のコンテナビルド・バックアップとリストアを確認し、Lightsail 固有の点は AWS / Ubuntu の公式情報をもとに記載。机上確認（別の担当者による通読）の指摘を反映: 前提となるソースの版とクローン後の確認コマンド（§1.4、§5.5）、`.env` の確認コマンド、SSH・UFW の設定前の手動スナップショットと復旧の限界、`apt upgrade` での sshd_config の確認画面、IPv6 では RADIUS を受けない・Docker ゲートウェイIPを登録しない、Admin TUI の起動で `.env` を読み込む、B-02 §12 / §14.3 と D-08 §8 の読み替え |
