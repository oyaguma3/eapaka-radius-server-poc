package audit

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// StreamKey は監査ログを保存する Valkey の Stream のキー（D-02 §2.H、D-13 §6.2）。
const StreamKey = "audit:prov"

// StreamIDPattern は Stream のエントリID の形式（{ミリ秒}-{連番}）。
var StreamIDPattern = regexp.MustCompile(`^[0-9]{1,20}-[0-9]{1,20}$`)

// Store は監査ログを Valkey の Stream に保存し、読み出す（GET /audit-logs。D-13 §4.2）。
// 標準出力の監査ログ（ログファイル）が正本で、Stream は参照用の写しである。
type Store struct {
	client *redis.Client
	// maxLen は保持する件数の上限（おおよそ。超えた分は古いものから捨てる）
	maxLen int64
}

// NewStore は新しい Store を生成する。
func NewStore(client *redis.Client, maxLen int64) *Store {
	return &Store{client: client, maxLen: maxLen}
}

// StoredEntry は Stream に保存した監査ログの1件。
type StoredEntry struct {
	// ID は Stream のエントリID
	ID string
	// Time は記録日時（エントリID のミリ秒から求める）
	Time time.Time
	Actor
	Entry
}

// add は監査ログを1件保存する。
func (s *Store) add(ctx context.Context, actor Actor, e Entry) error {
	return s.client.XAdd(ctx, &redis.XAddArgs{
		Stream: StreamKey,
		MaxLen: s.maxLen,
		Approx: true,
		Values: []any{
			"trace_id", actor.TraceID,
			"operation", string(e.Operation),
			"target_type", string(e.TargetType),
			"target_key", e.TargetKey,
			"target_id", e.TargetID,
			"target_imsi", e.TargetIMSI,
			"admin_user", actor.Operator,
			"mgmt_client", actor.MgmtClient,
			"details", e.Details,
		},
	}).Err()
}

// List は監査ログを新しい順に最大 limit 件返す。before が空でなければ、そのエントリID より古いものを返す。
// さらに古いものがあれば、次に before に渡す値（返した最後のエントリID）を next に入れる。
func (s *Store) List(ctx context.Context, before string, limit int) (entries []StoredEntry, next string, err error) {
	end := "+"
	if before != "" {
		if end, err = previousID(before); err != nil {
			return nil, "", err
		}
		if end == "" {
			// 0-0 より古いものはない
			return []StoredEntry{}, "", nil
		}
	}
	msgs, err := s.client.XRevRangeN(ctx, StreamKey, end, "-", int64(limit)+1).Result()
	if err != nil {
		return nil, "", err
	}
	more := len(msgs) > limit
	if more {
		msgs = msgs[:limit]
	}
	entries = make([]StoredEntry, len(msgs))
	for i, m := range msgs {
		entries[i] = entryFromMessage(m)
	}
	if more {
		next = msgs[len(msgs)-1].ID
	}
	return entries, next, nil
}

// previousID は、エントリID の直前の ID を返す（XREVRANGE の終端に使う。直前がなければ空文字）。
// 排他的な範囲指定（"(" 付き）に頼らず、どの版の Valkey でも同じに動くようにする。
func previousID(id string) (string, error) {
	msPart, seqPart, ok := strings.Cut(id, "-")
	if !ok || !StreamIDPattern.MatchString(id) {
		return "", fmt.Errorf("invalid stream id %q", id)
	}
	ms, err1 := strconv.ParseUint(msPart, 10, 64)
	seq, err2 := strconv.ParseUint(seqPart, 10, 64)
	if err := errors.Join(err1, err2); err != nil {
		return "", fmt.Errorf("invalid stream id %q: %w", id, err)
	}
	switch {
	case seq > 0:
		return fmt.Sprintf("%d-%d", ms, seq-1), nil
	case ms > 0:
		return fmt.Sprintf("%d-%d", ms-1, uint64(math.MaxUint64)), nil
	default:
		return "", nil
	}
}

// entryFromMessage は Stream のエントリを StoredEntry にする。
func entryFromMessage(m redis.XMessage) StoredEntry {
	str := func(k string) string {
		v, _ := m.Values[k].(string)
		return v
	}
	e := StoredEntry{
		ID: m.ID,
		Actor: Actor{
			Operator:   str("admin_user"),
			MgmtClient: str("mgmt_client"),
			TraceID:    str("trace_id"),
		},
		Entry: Entry{
			Operation:  Operation(str("operation")),
			TargetType: TargetType(str("target_type")),
			TargetKey:  str("target_key"),
			TargetID:   str("target_id"),
			TargetIMSI: str("target_imsi"),
			Details:    str("details"),
		},
	}
	if msPart, _, ok := strings.Cut(m.ID, "-"); ok {
		if ms, err := strconv.ParseInt(msPart, 10, 64); err == nil {
			e.Time = time.UnixMilli(ms).UTC()
		}
	}
	return e
}
