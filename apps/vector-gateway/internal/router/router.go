// Package router はPLMNベースのバックエンドルーティングを提供する。
package router

import (
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-gateway/internal/backend"
)

// Router はIMSIからPLMNを抽出し、適切なバックエンドを選択する。
type Router struct {
	plmnMap     map[string]string
	registry    *backend.Registry
	passthrough bool
}

// NewRouter は新しいRouterを生成する。
func NewRouter(plmnMap map[string]string, registry *backend.Registry, passthrough bool) *Router {
	return &Router{
		plmnMap:     plmnMap,
		registry:    registry,
		passthrough: passthrough,
	}
}

// SelectBackend はIMSIからPLMNを抽出し、対応するバックエンドを選択する。
// passthroughモードの場合は常にデフォルト（内部Vector API）を返す。
// PLMNマップにマッチしない場合もデフォルトを返す。
func (r *Router) SelectBackend(imsi string) (backend.Backend, error) {
	// passthroughモード: 常にデフォルト
	if r.passthrough {
		return r.registry.Default(), nil
	}

	// PLMNマップが空の場合はデフォルト
	if len(r.plmnMap) == 0 {
		return r.registry.Default(), nil
	}

	// IMSIからPLMN候補を抽出（6桁優先、次に5桁）
	candidates := extractPLMNs(imsi)
	for _, plmn := range candidates {
		if backendID, ok := r.plmnMap[plmn]; ok {
			b, err := r.registry.Get(backendID)
			if err != nil {
				return nil, err
			}
			return b, nil
		}
	}

	// マッチなし: デフォルト
	return r.registry.Default(), nil
}

// extractPLMNs はIMSIからPLMN候補を抽出する。
// IMSIの先頭6桁候補と5桁候補を返す（6桁優先）。
func extractPLMNs(imsi string) []string {
	var candidates []string
	if len(imsi) >= 6 {
		candidates = append(candidates, imsi[:6])
	}
	if len(imsi) >= 5 {
		candidates = append(candidates, imsi[:5])
	}
	return candidates
}
