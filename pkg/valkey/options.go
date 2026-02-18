// Package valkey はValkeyクライアントの共通機能を提供する。
package valkey

import (
	"fmt"
	"time"
)

// Options はValkeyクライアントの接続オプション。
type Options struct {
	Addr           string        // 接続先アドレス（host:port形式）
	Password       string        // 認証パスワード
	DB             int           // データベース番号
	ConnectTimeout time.Duration // 接続タイムアウト
	ReadTimeout    time.Duration // 読み取りタイムアウト
	WriteTimeout   time.Duration // 書き込みタイムアウト
	PoolSize       int           // コネクションプールサイズ
	MinIdleConns   int           // 最小アイドルコネクション数
}

// DefaultOptions はデフォルトのOptionsを返す。
// タイムアウト: 接続3秒、読み取り2秒、書き込み2秒
// プール: サイズ10、最小アイドル2
func DefaultOptions() *Options {
	return &Options{
		Addr:           "localhost:6379",
		Password:       "",
		DB:             0,
		ConnectTimeout: 3 * time.Second,
		ReadTimeout:    2 * time.Second,
		WriteTimeout:   2 * time.Second,
		PoolSize:       10,
		MinIdleConns:   2,
	}
}

// TUIOptions はTUIアプリケーション向けのOptionsを返す。
// タイムアウト: 接続5秒、読み取り5秒、書き込み5秒
// プール: サイズ5、最小アイドル1
func TUIOptions() *Options {
	return &Options{
		Addr:           "localhost:6379",
		Password:       "",
		DB:             0,
		ConnectTimeout: 5 * time.Second,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
		PoolSize:       5,
		MinIdleConns:   1,
	}
}

// WithAddr はアドレスを設定する。
func (o *Options) WithAddr(addr string) *Options {
	o.Addr = addr
	return o
}

// WithPassword はパスワードを設定する。
func (o *Options) WithPassword(password string) *Options {
	o.Password = password
	return o
}

// WithDB はデータベース番号を設定する。
func (o *Options) WithDB(db int) *Options {
	o.DB = db
	return o
}

// WithTimeouts はタイムアウトを設定する。
func (o *Options) WithTimeouts(connect, read, write time.Duration) *Options {
	o.ConnectTimeout = connect
	o.ReadTimeout = read
	o.WriteTimeout = write
	return o
}

// WithPool はプール設定を変更する。
func (o *Options) WithPool(poolSize, minIdle int) *Options {
	o.PoolSize = poolSize
	o.MinIdleConns = minIdle
	return o
}

// BuildAddr はホストとポートからアドレス文字列を生成する。
func BuildAddr(host string, port int) string {
	return fmt.Sprintf("%s:%d", host, port)
}
