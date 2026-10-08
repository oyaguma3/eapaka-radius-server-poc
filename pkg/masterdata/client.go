package masterdata

import (
	"context"
	"errors"
	"strconv"

	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
	"github.com/redis/go-redis/v9"
)

// ErrClientNotFound はRADIUSクライアントが見つからない場合のエラー
var ErrClientNotFound = errors.New("client not found")

// ErrClientExists は同じIPのRADIUSクライアントが既に存在する場合のエラー
var ErrClientExists = errors.New("client already exists")

// RADIUSクライアントは、Auth / Acct Server が送信元IPで引くため client:{IP} に保存する。
// あわせてサーバー採番のID（Hash の id フィールド）を持ち、IDからIPを引く索引 idx:client:{ID} と、
// 採番のカウンター seq:client を使う（D-02、D-13 §3.2）。
// 作成・変更・削除は、これらを Lua スクリプトでまとめて1回の操作で書き換える。

// createClientScript はキーが存在しないときだけRADIUSクライアントを作成し、IDを採番する。
// KEYS[1]=client:{IP}、KEYS[2]=seq:client。ARGV[1]=索引のプレフィックス、ARGV[2]=IP、ARGV[3..]=フィールドと値。
// 戻り値: 採番したID、0=既に存在する（何も書き込まない）
var createClientScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 1 then
  return 0
end
local id = redis.call('INCR', KEYS[2])
redis.call('HSET', KEYS[1], 'id', id, unpack(ARGV, 3))
redis.call('SET', ARGV[1] .. id, ARGV[2])
return id
`)

// putClientScript はRADIUSクライアントを作成または上書きする（CSV インポート用）。既存のIDは引き継ぎ、なければ採番する。
// KEYS・ARGV は createClientScript と同じ。戻り値: ID
var putClientScript = redis.NewScript(`
local id = redis.call('HGET', KEYS[1], 'id')
if not id then
  id = redis.call('INCR', KEYS[2])
  redis.call('SET', ARGV[1] .. id, ARGV[2])
end
redis.call('HSET', KEYS[1], 'id', id, unpack(ARGV, 3))
return tonumber(id)
`)

// patchClientScript は既存のRADIUSクライアントの指定したフィールドを書き換え、IPが変わる場合はキーを付け替える。
// KEYS[1]=client:{変更前のIP}、KEYS[2]=client:{変更後のIP}（変えない場合は同じ）。
// ARGV[1]=索引のプレフィックス、ARGV[2]=変更後のIP、ARGV[3..]=フィールドと値（なくてもよい）。
// 戻り値: 1=変更した、0=存在しない、-1=変更後のIPのクライアントが既に存在する（どちらも何も書き込まない）
var patchClientScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then
  return 0
end
if KEYS[1] ~= KEYS[2] and redis.call('EXISTS', KEYS[2]) == 1 then
  return -1
end
if #ARGV > 2 then
  redis.call('HSET', KEYS[1], unpack(ARGV, 3))
end
if KEYS[1] ~= KEYS[2] then
  redis.call('RENAME', KEYS[1], KEYS[2])
  local id = redis.call('HGET', KEYS[2], 'id')
  if id then
    redis.call('SET', ARGV[1] .. id, ARGV[2])
  end
end
return 1
`)

// deleteClientScript はRADIUSクライアントと、IDの索引を削除する。
// KEYS[1]=client:{IP}。ARGV[1]=索引のプレフィックス、ARGV[2]=IP。
// 戻り値: 1=削除した、0=存在しない
var deleteClientScript = redis.NewScript(`
local id = redis.call('HGET', KEYS[1], 'id')
if redis.call('DEL', KEYS[1]) == 0 then
  return 0
end
if id and redis.call('GET', ARGV[1] .. id) == ARGV[2] then
  redis.call('DEL', ARGV[1] .. id)
end
return 1
`)

// ensureClientIDScript は、IDを持たないRADIUSクライアント（ID の導入前のデータ）にIDを採番する。
// IDを持つ場合は、索引がなければ作り直し、カウンターがIDより小さければ合わせる（バックアップからの復元等に備える）。
// KEYS[1]=client:{IP}、KEYS[2]=seq:client。ARGV[1]=索引のプレフィックス、ARGV[2]=IP。
// 戻り値: 1=採番した、0=採番しなかった
var ensureClientIDScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then
  return 0
end
local id = redis.call('HGET', KEYS[1], 'id')
if id then
  if redis.call('EXISTS', ARGV[1] .. id) == 0 then
    redis.call('SET', ARGV[1] .. id, ARGV[2])
  end
  if tonumber(redis.call('GET', KEYS[2]) or '0') < tonumber(id) then
    redis.call('SET', KEYS[2], id)
  end
  return 0
end
id = redis.call('INCR', KEYS[2])
redis.call('HSET', KEYS[1], 'id', id)
redis.call('SET', ARGV[1] .. id, ARGV[2])
return 1
`)

// ClientStore はRADIUSクライアントデータへのアクセスを提供する。
type ClientStore struct {
	client *redis.Client
}

// NewClientStore は新しいClientStoreを生成する。
func NewClientStore(client *redis.Client) *ClientStore {
	return &ClientStore{client: client}
}

// Get は指定されたIPのRADIUSクライアントを取得する。
// Auth Server/Acct Serverと互換性のあるHash形式で読み取る。
func (s *ClientStore) Get(ctx context.Context, ip string) (*model.RadiusClient, error) {
	key := ClientKey(ip)
	result, err := s.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	// キーが存在しない場合、HGetAllは空mapを返す
	if len(result) == 0 {
		return nil, ErrClientNotFound
	}

	return clientFromHash(ip, result), nil
}

// GetByID は指定されたIDのRADIUSクライアントを取得する。存在しなければ ErrClientNotFound を返す。
func (s *ClientStore) GetByID(ctx context.Context, id int64) (*model.RadiusClient, error) {
	ip, err := s.client.Get(ctx, ClientIndexKey(id)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, ErrClientNotFound
	}
	if err != nil {
		return nil, err
	}
	c, err := s.Get(ctx, ip)
	if err != nil {
		return nil, err
	}
	// 索引が古い（同じIPに別のIDのクライアントがある）場合は、存在しないものとして扱う
	if c.ID != id {
		return nil, ErrClientNotFound
	}
	return c, nil
}

// Create は新しいRADIUSクライアントを作成し、採番したIDを c.ID に設定する。
// Auth Server/Acct Serverと互換性のあるHash形式で保存する。
// 存在確認・採番・書き込みを1回の操作で行い、既に存在すれば何も書き込まずに ErrClientExists を返す。
func (s *ClientStore) Create(ctx context.Context, c *model.RadiusClient) error {
	id, err := s.runClientScript(ctx, createClientScript, c)
	if err != nil {
		return err
	}
	if id == 0 {
		return ErrClientExists
	}
	c.ID = id
	return nil
}

// runClientScript は作成・上書きのスクリプトを実行し、戻り値（ID）を返す。
func (s *ClientStore) runClientScript(ctx context.Context, script *redis.Script, c *model.RadiusClient) (int64, error) {
	args := append([]any{PrefixClientIndex, c.IP}, fieldArgs(clientFields(c))...)
	return script.Run(ctx, s.client, []string{ClientKey(c.IP), KeyClientSeq}, args...).Int64()
}

// Update は既存のRADIUSクライアントを更新する（IDとIPは変えない）。
// 存在確認と書き込みを1回の操作で行い、存在しなければ何も書き込まずに ErrClientNotFound を返す。
func (s *ClientStore) Update(ctx context.Context, c *model.RadiusClient) error {
	updated, err := runHashScript(ctx, s.client, updateHashScript, ClientKey(c.IP), clientFields(c))
	if err != nil {
		return err
	}
	if !updated {
		return ErrClientNotFound
	}
	return nil
}

// ClientPatch はRADIUSクライアントの変更内容を表す。nil の項目は変更しない（JSON Merge Patch 用。D-13 §3.2）。
type ClientPatch struct {
	IP     *string // 変更後のIP（キーを付け替える。IDは変わらない）
	Secret *string
	Name   *string
	Vendor *string
}

// Patch は既存のRADIUSクライアントの、指定した項目だけを書き換える。IP を指定した場合はキーを付け替える。
// 存在確認と書き込みを1回の操作で行い、存在しなければ ErrClientNotFound、
// 変更後のIPのクライアントが既に存在すれば ErrClientExists を返す（どちらも何も書き込まない）。
func (s *ClientStore) Patch(ctx context.Context, ip string, patch *ClientPatch) error {
	fields := map[string]any{}
	for name, v := range map[string]*string{"secret": patch.Secret, "name": patch.Name, "vendor": patch.Vendor} {
		if v != nil {
			fields[name] = *v
		}
	}
	newIP := ip
	if patch.IP != nil {
		newIP = *patch.IP
	}

	args := append([]any{PrefixClientIndex, newIP}, fieldArgs(fields)...)
	n, err := patchClientScript.Run(ctx, s.client, []string{ClientKey(ip), ClientKey(newIP)}, args...).Int()
	if err != nil {
		return err
	}
	switch n {
	case 0:
		return ErrClientNotFound
	case -1:
		return ErrClientExists
	default:
		return nil
	}
}

// clientFields はRADIUSクライアントのHashフィールドを返す（id は含めない）。
func clientFields(c *model.RadiusClient) map[string]any {
	return map[string]any{
		"secret": c.Secret,
		"name":   c.Name,
		"vendor": c.Vendor,
	}
}

// Delete はRADIUSクライアントと、IDの索引を削除する。
func (s *ClientStore) Delete(ctx context.Context, ip string) error {
	n, err := deleteClientScript.Run(ctx, s.client, []string{ClientKey(ip)}, PrefixClientIndex, ip).Int()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrClientNotFound
	}
	return nil
}

// EnsureIDs は、IDを持たないRADIUSクライアント（IDの導入前に登録されたもの）にIDを採番し、採番した件数を返す。
// 何度実行しても結果は同じ（IDを持つクライアントは変えない）。Admin TUI と Provisioning API の起動時に呼ぶ。
func (s *ClientStore) EnsureIDs(ctx context.Context) (int, error) {
	var keys []string
	iter := s.client.Scan(ctx, 0, PrefixClient+"*", 100).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return 0, err
	}

	assigned := 0
	for _, key := range keys {
		ip := key[len(PrefixClient):]
		n, err := ensureClientIDScript.Run(ctx, s.client, []string{key, KeyClientSeq}, PrefixClientIndex, ip).Int()
		if err != nil {
			return assigned, err
		}
		assigned += n
	}
	return assigned, nil
}

// List は全RADIUSクライアントのリストを取得する（SCAN使用）。
func (s *ClientStore) List(ctx context.Context) ([]*model.RadiusClient, error) {
	var clients []*model.RadiusClient
	var keys []string

	// SCANで全キーを取得
	iter := s.client.Scan(ctx, 0, PrefixClient+"*", 100).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return clients, nil
	}

	// Pipelineで一括取得（HGETALL）
	pipe := s.client.Pipeline()
	cmds := make([]*redis.MapStringStringCmd, len(keys))
	for i, key := range keys {
		cmds[i] = pipe.HGetAll(ctx, key)
	}
	_, err := pipe.Exec(ctx)
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}

	for i, cmd := range cmds {
		result, err := cmd.Result()
		if err != nil {
			continue
		}

		// 空の結果はスキップ
		if len(result) == 0 {
			continue
		}

		// キーからIPを抽出
		ip := keys[i][len(PrefixClient):]
		clients = append(clients, clientFromHash(ip, result))
	}

	return clients, nil
}

// Count はRADIUSクライアントの総数を返す。
func (s *ClientStore) Count(ctx context.Context) (int64, error) {
	var count int64

	iter := s.client.Scan(ctx, 0, PrefixClient+"*", 100).Iterator()
	for iter.Next(ctx) {
		count++
	}
	if err := iter.Err(); err != nil {
		return 0, err
	}

	return count, nil
}

// Exists は指定されたIPのRADIUSクライアントが存在するか確認する。
func (s *ClientStore) Exists(ctx context.Context, ip string) (bool, error) {
	key := ClientKey(ip)
	exists, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

// BulkCreate は複数のRADIUSクライアントを作成または上書きする（CSV インポート用）。
// 既存のクライアントはIDを引き継ぎ、新しいクライアントにはIDを採番する（1件ずつ Lua スクリプトで行う）。
func (s *ClientStore) BulkCreate(ctx context.Context, clients []*model.RadiusClient) error {
	if len(clients) == 0 {
		return nil
	}

	pipe := s.client.Pipeline()
	cmds := make([]*redis.Cmd, len(clients))
	for i, c := range clients {
		args := append([]any{PrefixClientIndex, c.IP}, fieldArgs(clientFields(c))...)
		// パイプラインでは EVALSHA が NOSCRIPT のときに EVAL へ切り替えられないため、Eval でスクリプトごと送る
		cmds[i] = putClientScript.Eval(ctx, pipe, []string{ClientKey(c.IP), KeyClientSeq}, args...)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}
	for i, cmd := range cmds {
		id, err := cmd.Int64()
		if err != nil {
			return err
		}
		clients[i].ID = id
	}
	return nil
}

// clientFromHash はHashマップからRadiusClientを構築する。id がない（ID の導入前の）場合は 0 にする。
func clientFromHash(ip string, fields map[string]string) *model.RadiusClient {
	id, _ := strconv.ParseInt(fields["id"], 10, 64)
	return &model.RadiusClient{
		ID:     id,
		IP:     ip,
		Secret: fields["secret"],
		Name:   fields["name"],
		Vendor: fields["vendor"],
	}
}
