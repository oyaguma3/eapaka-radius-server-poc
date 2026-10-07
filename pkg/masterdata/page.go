package masterdata

import (
	"context"
	"errors"
	"sort"

	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
	"github.com/redis/go-redis/v9"
)

// SubscriberPage は加入者の一覧の1ページを表す（Provisioning API の一覧用。D-13 §4.1）。
type SubscriberPage struct {
	Items []*model.Subscriber
	// Total は前方一致の条件に一致する加入者の総数。
	Total int
	// NextCursor は次のページの開始位置（このページの最後の IMSI）。次のページがなければ空文字。
	NextCursor string
}

// PolicyPage は認可ポリシーの一覧の1ページを表す（Provisioning API の一覧用。D-13 §4.1）。
type PolicyPage struct {
	Items []*model.Policy
	// Total は前方一致の条件に一致する認可ポリシーの総数。
	Total int
	// NextCursor は次のページの開始位置（このページの最後の IMSI）。次のページがなければ空文字。
	NextCursor string
}

// ListPage は加入者を IMSI の昇順で limit 件まで返す。
// imsiPrefix が空でなければ IMSI の前方一致で絞り込み、cursor が空でなければ cursor より後の IMSI から返す。
func (s *SubscriberStore) ListPage(ctx context.Context, imsiPrefix, cursor string, limit int) (*SubscriberPage, error) {
	p, err := listHashPage(ctx, s.client, PrefixSubscriber, imsiPrefix, cursor, limit)
	if err != nil {
		return nil, err
	}

	page := &SubscriberPage{Items: []*model.Subscriber{}, Total: p.total, NextCursor: p.nextCursor}
	for i, imsi := range p.ids {
		if fields := p.fields[i]; len(fields) > 0 {
			page.Items = append(page.Items, subscriberFromHash(imsi, fields))
		}
	}
	return page, nil
}

// ListPage は認可ポリシーを IMSI の昇順で limit 件まで返す。
// imsiPrefix が空でなければ IMSI の前方一致で絞り込み、cursor が空でなければ cursor より後の IMSI から返す。
// rules を解釈できないポリシーは、List と同じく結果に含めない（Total には数える）。
func (s *PolicyStore) ListPage(ctx context.Context, imsiPrefix, cursor string, limit int) (*PolicyPage, error) {
	p, err := listHashPage(ctx, s.client, PrefixPolicy, imsiPrefix, cursor, limit)
	if err != nil {
		return nil, err
	}

	page := &PolicyPage{Items: []*model.Policy{}, Total: p.total, NextCursor: p.nextCursor}
	for i, imsi := range p.ids {
		fields := p.fields[i]
		if len(fields) == 0 {
			continue
		}
		policy, err := policyFromHash(imsi, fields)
		if err != nil {
			continue
		}
		page.Items = append(page.Items, policy)
	}
	return page, nil
}

// hashPage は listHashPage の結果を表す。ids と fields は同じ順序で対応する。
type hashPage struct {
	ids        []string
	fields     []map[string]string
	total      int
	nextCursor string
}

// listHashPage は keyPrefix+idPrefix で始まるキーを SCAN で集めて識別子の昇順に並べ、
// cursor より後の limit 件の Hash を読む。
// 一覧の途中で削除されたキーは、空の Hash（呼び出し側で読み飛ばす）になる。
func listHashPage(ctx context.Context, client *redis.Client, keyPrefix, idPrefix, cursor string, limit int) (*hashPage, error) {
	var ids []string
	iter := client.Scan(ctx, 0, keyPrefix+idPrefix+"*", 100).Iterator()
	for iter.Next(ctx) {
		ids = append(ids, iter.Val()[len(keyPrefix):])
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	sort.Strings(ids)

	start := 0
	if cursor != "" {
		// cursor より大きい最初の識別子から返す
		start = sort.Search(len(ids), func(i int) bool { return ids[i] > cursor })
	}
	end := min(start+max(limit, 0), len(ids))

	page := &hashPage{ids: ids[start:end], total: len(ids)}
	if end < len(ids) && end > start {
		page.nextCursor = ids[end-1]
	}
	if len(page.ids) == 0 {
		return page, nil
	}

	pipe := client.Pipeline()
	cmds := make([]*redis.MapStringStringCmd, len(page.ids))
	for i, id := range page.ids {
		cmds[i] = pipe.HGetAll(ctx, keyPrefix+id)
	}
	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}
	page.fields = make([]map[string]string, len(cmds))
	for i, cmd := range cmds {
		page.fields[i] = cmd.Val()
	}
	return page, nil
}
