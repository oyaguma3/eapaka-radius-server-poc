package backend

import (
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-gateway/internal/config"
)

const defaultBackendID = "00"

// Registry はバックエンドの登録管理を行う。
type Registry struct {
	backends  map[string]Backend
	defaultID string
}

// NewRegistry は新しいRegistryを生成する。
// 内部Vector API（ID:00）をデフォルトバックエンドとして登録する。
func NewRegistry(cfg *config.Config) *Registry {
	r := &Registry{
		backends:  make(map[string]Backend),
		defaultID: defaultBackendID,
	}

	// 内部Vector APIバックエンドを登録
	internal := NewInternalBackend(cfg.InternalURL, cfg.InternalTimeout)
	r.backends[internalBackendID] = internal

	return r
}

// Get は指定IDのバックエンドを取得する。
// 未登録のIDの場合はBackendNotImplementedErrorを返す。
func (r *Registry) Get(id string) (Backend, error) {
	b, ok := r.backends[id]
	if !ok {
		return nil, &BackendNotImplementedError{ID: id}
	}
	return b, nil
}

// Default はデフォルトバックエンド（内部Vector API）を返す。
func (r *Registry) Default() Backend {
	return r.backends[r.defaultID]
}
