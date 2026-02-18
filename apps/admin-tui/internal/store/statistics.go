package store

import (
	"context"
	"sync"
	"time"
)

// Statistics は統計情報を表す。
type Statistics struct {
	SubscriberCount int64 `json:"subscriber_count"`
	ClientCount     int64 `json:"client_count"`
	PolicyCount     int64 `json:"policy_count"`
	SessionCount    int64 `json:"session_count"`
	UpdatedAt       int64 `json:"updated_at"`
}

// StatisticsStore は統計情報へのアクセスを提供する。
type StatisticsStore struct {
	subscriberStore *SubscriberStore
	clientStore     *ClientStore
	policyStore     *PolicyStore
	sessionStore    *SessionStore

	mu       sync.RWMutex
	cache    *Statistics
	cacheTTL time.Duration
}

// NewStatisticsStore は新しいStatisticsStoreを生成する。
func NewStatisticsStore(
	subscriberStore *SubscriberStore,
	clientStore *ClientStore,
	policyStore *PolicyStore,
	sessionStore *SessionStore,
) *StatisticsStore {
	return &StatisticsStore{
		subscriberStore: subscriberStore,
		clientStore:     clientStore,
		policyStore:     policyStore,
		sessionStore:    sessionStore,
		cacheTTL:        1 * time.Minute,
	}
}

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

// Refresh はキャッシュを更新して最新の統計情報を取得する。
func (s *StatisticsStore) Refresh(ctx context.Context) (*Statistics, error) {
	stats := &Statistics{
		UpdatedAt: time.Now().Unix(),
	}

	// 並列で各カウントを取得
	var wg sync.WaitGroup
	var subscriberErr, clientErr, policyErr, sessionErr error

	wg.Add(4)

	go func() {
		defer wg.Done()
		count, err := s.subscriberStore.Count(ctx)
		if err != nil {
			subscriberErr = err
			return
		}
		stats.SubscriberCount = count
	}()

	go func() {
		defer wg.Done()
		count, err := s.clientStore.Count(ctx)
		if err != nil {
			clientErr = err
			return
		}
		stats.ClientCount = count
	}()

	go func() {
		defer wg.Done()
		var count int64
		iter := s.policyStore.client.Scan(ctx, 0, PrefixPolicy+"*", 100).Iterator()
		for iter.Next(ctx) {
			count++
		}
		if err := iter.Err(); err != nil {
			policyErr = err
			return
		}
		stats.PolicyCount = count
	}()

	go func() {
		defer wg.Done()
		count, err := s.sessionStore.Count(ctx)
		if err != nil {
			sessionErr = err
			return
		}
		stats.SessionCount = count
	}()

	wg.Wait()

	// エラーがあっても部分的な結果を返す（エラーは最初のものを返す）
	if subscriberErr != nil {
		return stats, subscriberErr
	}
	if clientErr != nil {
		return stats, clientErr
	}
	if policyErr != nil {
		return stats, policyErr
	}
	if sessionErr != nil {
		return stats, sessionErr
	}

	// キャッシュ更新
	s.mu.Lock()
	s.cache = stats
	s.mu.Unlock()

	return stats, nil
}

// ClearCache はキャッシュをクリアする。
func (s *StatisticsStore) ClearCache() {
	s.mu.Lock()
	s.cache = nil
	s.mu.Unlock()
}
