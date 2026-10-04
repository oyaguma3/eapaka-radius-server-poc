package backend

import (
	"fmt"

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
// aka-only-serverのURLが設定されている場合はID:01も登録する。
func NewRegistry(cfg *config.Config) (*Registry, error) {
	r := &Registry{
		backends:  make(map[string]Backend),
		defaultID: defaultBackendID,
	}

	// 内部Vector APIバックエンドを登録
	internal := NewInternalBackend(cfg.InternalURL, cfg.InternalTimeout)
	r.backends[internalBackendID] = internal

	// aka-only-serverバックエンドを登録（URL設定時のみ）
	if cfg.AKAOnlyEnabled() {
		akaOnly, err := NewAKAOnlyBackend(AKAOnlyOptions{
			BaseURL:        cfg.AKAOnlyURL,
			ClientCertFile: cfg.AKAOnlyClientCert,
			ClientKeyFile:  cfg.AKAOnlyClientKey,
			ServerCertFile: cfg.AKAOnlyServerCert,
			Timeout:        cfg.AKAOnlyTimeout,
			MaskIMSI:       cfg.LogMaskIMSI,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create aka-only-server backend: %w", err)
		}
		r.backends[akaOnlyBackendID] = akaOnly
	}

	return r, nil
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
