package session

import (
	"context"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/store"
)

// manager はSessionManagerインターフェースの実装。
type manager struct {
	sessionStore store.SessionStore
}

// NewManager は新しいSessionManagerを生成する。
func NewManager(ss store.SessionStore) SessionManager {
	return &manager{sessionStore: ss}
}

// Exists はセッションの存在を確認する。
func (m *manager) Exists(ctx context.Context, uuid string) (bool, error) {
	return m.sessionStore.Exists(ctx, uuid)
}

// Get はセッション情報を取得する。
func (m *manager) Get(ctx context.Context, uuid string) (*Session, error) {
	raw, err := m.sessionStore.Get(ctx, uuid)
	if err != nil {
		if err == store.ErrKeyNotFound {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	var sess Session
	if err := store.MapToStruct(raw, &sess); err != nil {
		return nil, err
	}
	return &sess, nil
}

// UpdateOnStart はStart受信時のセッション更新を行う。
func (m *manager) UpdateOnStart(ctx context.Context, uuid string, data *SessionStartData) error {
	fields := map[string]any{
		"start_time": data.StartTime,
		"nas_ip":     data.NasIP,
		"acct_id":    data.AcctID,
	}
	if data.ClientIP != "" {
		fields["client_ip"] = data.ClientIP
	}
	return m.sessionStore.UpdateOnStart(ctx, uuid, fields)
}

// UpdateOnInterim はInterim受信時のセッション更新を行う。
func (m *manager) UpdateOnInterim(ctx context.Context, uuid string, data *SessionInterimData) error {
	fields := map[string]any{
		"nas_ip":        data.NasIP,
		"input_octets":  data.InputOctets,
		"output_octets": data.OutputOctets,
	}
	if data.ClientIP != "" {
		fields["client_ip"] = data.ClientIP
	}
	return m.sessionStore.UpdateOnInterim(ctx, uuid, fields)
}

// Delete はセッションを削除する。
func (m *manager) Delete(ctx context.Context, uuid string) error {
	return m.sessionStore.Delete(ctx, uuid)
}

// RemoveUserIndex はユーザーインデックスからセッションを削除する。
func (m *manager) RemoveUserIndex(ctx context.Context, imsi, uuid string) error {
	return m.sessionStore.RemoveUserIndex(ctx, imsi, uuid)
}
