# B-01 ホストOS構築手順書 (r3)

## 1. 概要

### 1.1 目的

本ドキュメントは、ベアメタルのミニPCにUbuntu Server 24.04 LTSをインストールし、Docker Composeでアプリケーションを実行可能な状態にするまでの手順を提供する。

### 1.2 スコープ

**本書で扱う範囲：**

- Ubuntu Server 24.04 LTSのインストール
- ホストOS初期設定（タイムゾーン、NTP同期）
- セキュリティ設定（SSH、UFW）
- Docker Engine / Docker Compose導入
- 運用ツール導入（lnav、jq、git）
- systemdサービス登録（アプリケーション自動起動）

**アプリケーションデプロイ手順書（B-02）で扱う範囲：**

- リポジトリのクローン（`git clone`）
- 環境変数ファイル（`.env`）の作成
- `docker compose up -d` による起動
- Admin TUIバイナリの配置
- logrotate設定ファイルの配置
- バックアップスクリプトの配置
- lnavフォーマットファイルの配置

### 1.3 関連ドキュメント

| ドキュメント | 参照内容 |
|-------------|---------|
| D-01 ミニPC版設計仕様書 (r9) | システム構成、ホストPCセットアップ手順概要 |
| D-08 インフラ設定・運用設計書 (r12) | ホストOS設定内容、セキュリティチェックリスト |
| B-02 アプリケーションデプロイ手順書 | デプロイ手順、運用ファイル配置 |

### 1.4 対象読者

Linux CLIの基本操作（ファイル編集、パッケージ管理、サービス管理）の知識を有するインフラ担当者。

---

## 2. 前提条件

### 2.1 ハードウェア要件

| 項目 | 想定スペック |
|------|-------------|
| CPU | 4コア以上（Intel N100相当） |
| メモリ | 8GB以上 |
| ストレージ | 256GB SSD以上 |
| ネットワーク | 1GbE有線LAN |

### 2.2 必要な資材

| 資材 | 用途 |
|------|------|
| USBインストールメディア | Ubuntu Server 24.04 LTS ISOを書き込み済み |
| キーボード・ディスプレイ | OSインストール時に使用（SSH設定完了後は不要） |
| SSH公開鍵 | 管理端末からの接続用（事前に生成しておくこと） |
| SSHクライアント | 管理端末側に導入済みであること |

### 2.3 ネットワーク要件

| タイミング | インターネット接続 | 備考 |
|-----------|------------------|------|
| 初期セットアップ時 | 必要 | apt パッケージ更新、Docker導入に使用 |
| 運用時 | 不要 | オフライン環境でもローカル動作可能 |

---

## 3. OSインストール

### 3.1 インストールメディア準備

Ubuntu Server 24.04 LTS の ISO イメージを取得し、USBメディアに書き込む。

- ISOイメージは Ubuntu 公式サイトから取得する
- USBメディアへの書き込みには `dd` コマンド、Rufus、balenaEtcher 等を使用する

> **注記:** インストールメディアの作成手順は、使用するOS・ツールにより異なるため本書では詳細を割愛する。

### 3.2 Ubuntu Server 24.04 LTS インストール

USBメディアからブートし、インストーラーの各画面で以下の設定を行う。

| 設定項目 | 設定内容 |
|---------|---------|
| 言語 | English（推奨）または日本語 |
| キーボードレイアウト | 使用するキーボードに合わせて選択 |
| ネットワーク | DHCPまたは固定IPを環境に合わせて設定 |
| ストレージ | ディスク全体を使用（デフォルト設定） |
| ユーザー名 | `admin` |
| サーバー名 | 任意（例: `eapaka-poc`） |
| OpenSSHサーバー | **有効化する**（インストーラーで選択） |

> **注意:** ユーザー名 `admin` は本プロジェクトの各種設定（systemdサービスファイル、ディレクトリパス等）で前提としている。異なるユーザー名を使用する場合、本書および B-02 の関連パスを全て修正する必要がある。

### 3.3 初回ログインと初期確認

インストール完了後、再起動して作成したユーザーでログインし、以下を確認する。

```bash
# カーネルバージョン確認
uname -a
# 出力例: Linux eapaka-poc 6.8.0-xx-generic #xx-Ubuntu SMP ... x86_64 GNU/Linux

# OSバージョン確認
lsb_release -a
# 出力例:
# Distributor ID: Ubuntu
# Description:    Ubuntu 24.04.x LTS
# Release:        24.04
# Codename:       noble
```

---

## 4. 基本設定

### 4.1 パッケージ更新

システムのパッケージを最新の状態に更新する。

```bash
sudo apt update && sudo apt upgrade -y
```

更新完了後、カーネル更新があった場合は再起動する。

```bash
# 再起動が必要かどうか確認
ls /var/run/reboot-required 2>/dev/null && echo "再起動が必要です" || echo "再起動不要"

# 必要な場合は再起動
sudo reboot
```

### 4.2 タイムゾーン設定

タイムゾーンを `Asia/Tokyo` (JST) に設定する（D-08 §5.2）。

```bash
# タイムゾーン設定
sudo timedatectl set-timezone Asia/Tokyo

# 確認
timedatectl status
# 期待される出力（抜粋）:
#                 Time zone: Asia/Tokyo (JST, +0900)
```

### 4.3 NTP同期設定

`systemd-timesyncd` によるNTP同期を有効化する（D-08 §5.3）。

```bash
# NTP同期サービスの有効化・起動
sudo systemctl enable --now systemd-timesyncd

# 同期状態の確認
timedatectl status
# 期待される出力（抜粋）:
# System clock synchronized: yes
#               NTP service: active

# 詳細確認
timedatectl show-timesync --all
# 期待される出力（抜粋）:
# NTPSynchronized=yes
```

#### NTPサーバーの変更（オプション）

デフォルトのNTPサーバーを `ntp.nict.jp`（NICT公開NTPサービス）に変更する場合は、以下のファイルを編集する。

```bash
sudo vi /etc/systemd/timesyncd.conf
```

以下の内容を設定する。

```ini
[Time]
NTP=ntp.nict.jp
FallbackNTP=ntp.ubuntu.com
```

設定変更後、サービスを再起動して反映する。

```bash
sudo systemctl restart systemd-timesyncd

# 変更後の確認
timedatectl show-timesync --all
# ServerName に ntp.nict.jp が表示されること
```

---

## 5. セキュリティ設定

### 5.1 SSH設定

SSHの接続ポート変更、公開鍵認証の強制、rootログイン無効化を行う（D-08 §5.5）。

#### 公開鍵の配置

管理端末の公開鍵を `admin` ユーザーの `authorized_keys` に配置する。

```bash
# .sshディレクトリ作成（存在しない場合）
mkdir -p ~/.ssh
chmod 700 ~/.ssh

# 公開鍵を配置（管理端末から転送、またはここに貼り付け）
vi ~/.ssh/authorized_keys
# 管理端末の公開鍵（例: ssh-ed25519 AAAA... user@client）を記入

# パーミッション設定
chmod 600 ~/.ssh/authorized_keys
```

#### 公開鍵認証での接続確認

> **⚠️ 警告: sshd_config を変更する前に、必ず公開鍵認証での接続が成功することを確認すること。パスワード認証を無効化した後に公開鍵認証が失敗すると、SSH接続不能となり物理コンソールでの復旧が必要になる。**

管理端末の**別ターミナル**から公開鍵認証での接続を確認する。

```bash
# 管理端末から実行（デフォルトポート22で接続）
ssh -i ~/.ssh/<秘密鍵ファイル> admin@<ミニPCのIPアドレス>
```

接続が成功することを確認してから、次のステップに進む。

#### sshd_config の編集

```bash
sudo vi /etc/ssh/sshd_config
```

以下の設定項目を変更する。既存の設定行がある場合はコメントアウトまたは上書きする。

```
Port 10022
PermitRootLogin no
PasswordAuthentication no
PubkeyAuthentication yes
```

#### 設定の反映と確認

```bash
# 設定ファイルの構文チェック
sudo sshd -t
# エラーが出ないことを確認

# SSHサービス再起動
sudo systemctl restart ssh

# 設定値の確認
sudo sshd -T | grep -E '^(port|permitrootlogin|passwordauthentication|pubkeyauthentication)'
# 期待される出力:
# port 10022
# permitrootlogin no
# passwordauthentication no
# pubkeyauthentication yes
```

> **⚠️ 警告: SSHサービス再起動後、現在の接続セッションは維持されるが、必ず別ターミナルで新しいポート（10022）での接続を確認すること。確認が完了するまで、現在のセッションを切断しないこと。**

```bash
# 管理端末の別ターミナルから実行
ssh -p 10022 -i ~/.ssh/<秘密鍵ファイル> admin@<ミニPCのIPアドレス>
```

### 5.2 UFW設定

ファイアウォール（UFW）を設定し、必要なポートのみを許可する（D-08 §5.4）。

```bash
# デフォルトポリシー設定
sudo ufw default deny incoming
sudo ufw default allow outgoing

# SSH（カスタムポート）を許可
sudo ufw allow 10022/tcp

# RADIUS認証ポートを許可
sudo ufw allow 1812/udp

# RADIUS課金ポートを許可
sudo ufw allow 1813/udp
```

> **⚠️ 警告: `ufw enable` を実行する前に、SSH用ポート（10022/tcp）の許可ルールが追加済みであることを必ず確認すること。SSH用ポートを許可せずにファイアウォールを有効化すると、SSH接続が遮断される。**

```bash
# 設定済みルールの事前確認
sudo ufw show added
# 10022/tcp ALLOW が含まれていることを確認

# ファイアウォール有効化
sudo ufw enable
# "Command may disrupt existing ssh connections. Proceed with operation (y|n)?" と表示されたら y を入力

# 状態確認
sudo ufw status verbose
# 期待される出力:
# Status: active
# Logging: on (low)
# Default: deny (incoming), allow (outgoing), disabled (routed)
#
# To                         Action      From
# --                         ------      ----
# 10022/tcp                  ALLOW IN    Anywhere
# 1812/udp                   ALLOW IN    Anywhere
# 1813/udp                   ALLOW IN    Anywhere
# 10022/tcp (v6)             ALLOW IN    Anywhere (v6)
# 1812/udp (v6)              ALLOW IN    Anywhere (v6)
# 1813/udp (v6)              ALLOW IN    Anywhere (v6)
```

---

## 6. Docker導入

### 6.1 Docker Engineインストール

Docker公式aptリポジトリ経由でDocker Engineをインストールする。

```bash
# 前提パッケージのインストール
sudo apt install -y ca-certificates curl

# Docker公式GPGキーの追加
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc

# Dockerリポジトリの追加
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "${VERSION_CODENAME}") stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# パッケージリスト更新
sudo apt update

# Docker Engine および Docker Compose プラグインのインストール
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
```

### 6.2 ユーザーのdockerグループ追加

`admin` ユーザーを `docker` グループに追加し、`sudo` なしで `docker` コマンドを実行可能にする。

```bash
sudo usermod -aG docker admin
```

> **注意:** グループの変更を反映するには、一度ログアウトして再ログインする必要がある。

```bash
# ログアウト
exit

# 再ログイン後、グループ確認
id admin
# 出力に "docker" が含まれていること
```

### 6.3 Docker自動起動設定

```bash
sudo systemctl enable docker
```

### 6.4 動作確認

```bash
# バージョン確認
docker --version
# 出力例: Docker version 29.x.x, build xxxxxxx

docker compose version
# 出力例: Docker Compose version v2.x.x

# テスト実行（sudo不要で実行できることを確認）
docker run hello-world
# "Hello from Docker!" メッセージが表示されること

# テスト用コンテナ・イメージの削除
docker rm $(docker ps -aq) 2>/dev/null
docker rmi hello-world 2>/dev/null
```

---

## 7. 運用ツール導入

### 7.1 lnav

ログ解析ツール `lnav` をインストールする。

```bash
sudo apt install -y lnav

# バージョン確認
lnav -V
```

> **注記:** lnav用のカスタムフォーマットファイル（`eap_aka_log.json`）の配置はB-02で実施する。

### 7.2 その他ユーティリティ

```bash
# jq: JSONデータの整形・フィルタリング
# git: リポジトリクローン（B-02で使用）
sudo apt install -y jq git

# バージョン確認
jq --version
git --version
```

---

## 8. systemdサービス設定

### 8.1 サービスファイル作成

Docker Composeアプリケーションの自動起動用systemdサービスファイルを作成する（D-08 §5.6）。

```bash
sudo vi /etc/systemd/system/eapaka-radius-server-poc.service
```

以下の内容を記述する。

```ini
[Unit]
Description=EAP-AKA RADIUS PoC
Requires=docker.service
After=docker.service

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=/home/admin/eapaka-radius-server-poc/deployments
ExecStart=/usr/bin/docker compose up -d
ExecStop=/usr/bin/docker compose down
User=admin
Group=docker

[Install]
WantedBy=multi-user.target
```

サービスファイル作成後、systemdにリロードさせる。

```bash
sudo systemctl daemon-reload
```

### 8.2 自動起動有効化

```bash
sudo systemctl enable eapaka-radius-server-poc
```

> **⚠️ 注意: この時点では `systemctl start` を実行しないこと。** リポジトリのクローンと `.env` ファイルの作成（B-02）が完了するまで、サービスの起動は行わない。WorkingDirectoryが存在しない状態で起動すると失敗する。

### 8.3 動作確認

```bash
# 自動起動が有効化されていることを確認
systemctl is-enabled eapaka-radius-server-poc
# 期待される出力: enabled
```

---

## 9. 構築後チェックリスト

以下の全項目を確認し、すべて期待値を満たしていることを確認する。

| # | 確認項目 | 確認コマンド | 期待値 |
|---|---------|-------------|-------|
| 1 | OSバージョン | `lsb_release -a` | Ubuntu 24.04 LTS |
| 2 | タイムゾーン | `timedatectl status` | Asia/Tokyo |
| 3 | NTP同期 | `timedatectl show-timesync` | NTPSynchronized=yes |
| 4 | SSHポート | `sudo sshd -T \| grep port` | 10022 |
| 5 | パスワード認証無効 | `sudo sshd -T \| grep passwordauthentication` | no |
| 6 | rootログイン無効 | `sudo sshd -T \| grep permitrootlogin` | no |
| 7 | 公開鍵認証有効 | `sudo sshd -T \| grep pubkeyauthentication` | yes |
| 8 | UFW有効 | `sudo ufw status` | active |
| 9 | UFWルール（SSH） | `sudo ufw status` | 10022/tcp ALLOW |
| 10 | UFWルール（RADIUS認証） | `sudo ufw status` | 1812/udp ALLOW |
| 11 | UFWルール（RADIUS課金） | `sudo ufw status` | 1813/udp ALLOW |
| 12 | Dockerインストール | `docker --version` | バージョン番号が表示 |
| 13 | Docker Compose | `docker compose version` | バージョン番号が表示 |
| 14 | Docker自動起動 | `systemctl is-enabled docker` | enabled |
| 15 | dockerグループ | `id admin` | docker を含む |
| 16 | systemdサービス登録 | `systemctl is-enabled eapaka-radius-server-poc` | enabled |
| 17 | lnavインストール | `lnav -V` | バージョン番号が表示 |
| 18 | jqインストール | `jq --version` | バージョン番号が表示 |
| 19 | gitインストール | `git --version` | バージョン番号が表示 |

> **注記:** すべての確認項目が期待値を満たしたら、B-02「アプリケーションデプロイ手順書」に進む。

---

## 10. トラブルシューティング

### 10.1 SSH接続不可時の対処

| 症状 | 原因の可能性 | 対処法 |
|------|-------------|-------|
| 接続タイムアウト | ポート番号が異なる | `-p 10022` を指定して接続を試みる |
| 接続タイムアウト | UFWでSSHポートが許可されていない | 物理コンソールで `sudo ufw allow 10022/tcp` を実行 |
| Permission denied (publickey) | 公開鍵が正しく配置されていない | 物理コンソールで `~/.ssh/authorized_keys` の内容とパーミッション（600）を確認 |
| Permission denied (publickey) | 秘密鍵と公開鍵が一致しない | 管理端末側の秘密鍵ファイルを確認 |
| Connection refused | SSHサービスが停止している | 物理コンソールで `sudo systemctl start ssh` を実行 |

**物理コンソールでの復旧手順:**

SSH接続が完全に不可能になった場合、ミニPCにキーボード・ディスプレイを接続して直接ログインし、以下の手順で復旧する。

```bash
# 1. SSH設定を確認
sudo cat /etc/ssh/sshd_config | grep -E '^(Port|PermitRootLogin|PasswordAuthentication|PubkeyAuthentication)'

# 2. 一時的にパスワード認証を有効化（復旧用）
sudo sed -i 's/^PasswordAuthentication no/PasswordAuthentication yes/' /etc/ssh/sshd_config
sudo systemctl restart ssh

# 3. パスワード認証で接続し、公開鍵を修正
# 修正完了後、パスワード認証を再度無効化する

# 4. UFWの状態確認
sudo ufw status
# SSHポートが許可されていない場合
sudo ufw allow 10022/tcp
```

### 10.2 Docker導入失敗時の対処

| 症状 | 原因の可能性 | 対処法 |
|------|-------------|-------|
| GPGキーエラー | ネットワーク不安定、プロキシ環境 | GPGキーの取得コマンドを再実行する |
| リポジトリ追加エラー | `/etc/apt/sources.list.d/docker.list` の書式不正 | ファイルを削除して手順6.1をやり直す |
| パッケージインストール失敗 | DNS解決不可 | `/etc/resolv.conf` を確認し、有効なDNSサーバーを設定する |

```bash
# Docker関連の設定をクリーンアップして再実行する場合
sudo rm -f /etc/apt/keyrings/docker.asc
sudo rm -f /etc/apt/sources.list.d/docker.list
sudo apt update

# セクション6.1の手順を最初から再実行する
```

### 10.3 NTP同期不可時の対処

| 症状 | 原因の可能性 | 対処法 |
|------|-------------|-------|
| NTPSynchronized=no | ネットワーク未接続 | インターネット接続を確認する |
| NTPSynchronized=no | NTPサーバーへの疎通不可 | ファイアウォール（外部側）でUDP 123が許可されているか確認 |
| NTPSynchronized=no | timesyncdが停止している | `sudo systemctl restart systemd-timesyncd` を実行 |

```bash
# NTPサービスの詳細状態確認
systemctl status systemd-timesyncd

# 手動でNTP同期を試行
sudo timedatectl set-ntp false
sudo timedatectl set-ntp true

# ネットワーク非接続時のフォールバック（手動で時刻設定）
sudo timedatectl set-time "2026-02-07 12:00:00"
```

> **注記:** 運用時はオフライン環境のため、初期セットアップ時にNTP同期で正確な時刻を取得しておくことが重要である。運用中の時刻ずれが許容範囲を超える場合は、手動で時刻を修正する。

---

## 改訂履歴

| 版数 | 日付 | 内容 |
|------|------|------|
| r1 | 2026-02-07 | 初版作成 |
| r2 | 2026-02-16 | 関連ドキュメント参照版数を更新（D-01 r8→r9、D-08 r8→r9） |
| r3 | 2026-02-23 | 関連ドキュメント参照版数を更新（D-08 r9→r12） |
