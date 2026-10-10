package masterdata

import (
	"context"
	"sort"

	"github.com/redis/go-redis/v9"
)

// createHashScript はキーが存在しないときだけ Hash を作成する。
// 存在確認と書き込みを1回で行うので、同じキーを同時に作成しても後の方は上書きしない（D-13 §2.3）。
// 戻り値: 1=作成した、0=既に存在する（何も書き込まない）
var createHashScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 1 then
  return 0
end
redis.call('HSET', KEYS[1], unpack(ARGV))
return 1
`)

// updateHashScript はキーが存在するときだけ、指定したフィールドを書き換える。
// 存在確認と書き込みを1回で行うので、途中で削除されたキーを一部のフィールドだけで作り直さない。
// 戻り値: 1=更新した、0=存在しない（何も書き込まない）
var updateHashScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then
  return 0
end
redis.call('HSET', KEYS[1], unpack(ARGV))
return 1
`)

// putPolicyScript は認可ポリシーの指定したフィールドを書き込み、書き込む前にキーが存在したかと、書き込んだ後の status を返す。
// 指定しないフィールド（status）は変えないので、置き換えても停止の状態は保たれる。
// 戻り値: {1=作成した（存在しなかった）/ 0=置き換えた（存在した）, status（ない場合は空文字）}
var putPolicyScript = redis.NewScript(`
local existed = redis.call('EXISTS', KEYS[1])
redis.call('HSET', KEYS[1], unpack(ARGV))
local status = redis.call('HGET', KEYS[1], 'status')
if not status then
  status = ''
end
if existed == 1 then
  return {0, status}
end
return {1, status}
`)

// setStatusScript はキーが存在するときだけ status フィールドを書き換え、変更前の値を返す。
// 変更前の値の読み出しと書き込みを1回で行うので、監査ログに残す変更前の状態がずれない。
// 戻り値: 変更前の status（ない場合は空文字）。キーが存在しなければ nil（何も書き込まない）
// 変更前と同じ値（status がなく active にする場合を含む）なら書き込まない。
var setStatusScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then
  return nil
end
local prev = redis.call('HGET', KEYS[1], 'status')
if not prev then
  prev = ''
end
local cur = prev
if cur == '' then
  cur = 'active'
end
if cur ~= ARGV[1] then
  redis.call('HSET', KEYS[1], 'status', ARGV[1])
end
return prev
`)

// fieldArgs はフィールドと値の組をスクリプトの引数（field1, value1, field2, value2, ...）にする。
// 引数の順序を一定にするため、フィールド名の順に並べる。
func fieldArgs(fields map[string]any) []any {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)

	args := make([]any, 0, len(fields)*2)
	for _, name := range names {
		args = append(args, name, fields[name])
	}
	return args
}

// runHashScript はスクリプトを実行し、戻り値が 1 かどうかを返す。
func runHashScript(ctx context.Context, client *redis.Client, script *redis.Script, key string, fields map[string]any) (bool, error) {
	n, err := script.Run(ctx, client, []string{key}, fieldArgs(fields)...).Int()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}
