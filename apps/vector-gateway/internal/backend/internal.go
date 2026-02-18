package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	internalBackendID   = "00"
	internalBackendName = "Internal Vector API"
	traceIDHeader       = "X-Trace-ID"
)

// InternalBackend は内部Vector APIへのバックエンド。
type InternalBackend struct {
	baseURL string
	client  *http.Client
}

// NewInternalBackend は新しいInternalBackendを生成する。
func NewInternalBackend(baseURL string, timeout time.Duration) *InternalBackend {
	return &InternalBackend{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// GetVector は内部Vector APIからベクターを取得する。
func (b *InternalBackend) GetVector(ctx context.Context, req *VectorRequest) (*VectorResponse, error) {
	// リクエストボディ生成
	body, err := json.Marshal(req)
	if err != nil {
		return nil, &BackendCommunicationError{Err: fmt.Errorf("failed to marshal request: %w", err)}
	}

	// HTTPリクエスト作成
	url := b.baseURL + "/api/v1/vector"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, &BackendCommunicationError{Err: fmt.Errorf("failed to create request: %w", err)}
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// X-Trace-IDヘッダの伝搬
	if traceID, ok := ctx.Value(traceIDContextKey).(string); ok && traceID != "" {
		httpReq.Header.Set(traceIDHeader, traceID)
	}

	// リクエスト送信
	resp, err := b.client.Do(httpReq)
	if err != nil {
		return nil, &BackendCommunicationError{Err: fmt.Errorf("failed to send request: %w", err)}
	}
	defer resp.Body.Close()

	// レスポンスボディ読み込み
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &BackendCommunicationError{Err: fmt.Errorf("failed to read response: %w", err)}
	}

	// ステータスコードに応じたエラー処理
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		// 4xxエラー: そのまま伝搬
		var problem ProblemDetail
		if err := json.Unmarshal(respBody, &problem); err != nil {
			return nil, &BackendCommunicationError{Err: fmt.Errorf("failed to parse error response: %w", err)}
		}
		return nil, &BackendResponseError{
			StatusCode: resp.StatusCode,
			Problem:    &problem,
		}
	}

	if resp.StatusCode >= 500 {
		// 5xxエラー: 通信エラーとして扱う
		return nil, &BackendCommunicationError{
			Err: fmt.Errorf("backend returned status %d", resp.StatusCode),
		}
	}

	// 成功レスポンスのパース
	var vectorResp VectorResponse
	if err := json.Unmarshal(respBody, &vectorResp); err != nil {
		return nil, &BackendCommunicationError{Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	return &vectorResp, nil
}

// ID はバックエンドIDを返す。
func (b *InternalBackend) ID() string {
	return internalBackendID
}

// Name はバックエンド名を返す。
func (b *InternalBackend) Name() string {
	return internalBackendName
}

// traceIDContextKeyType はTraceIDコンテキストキーの型。
type traceIDContextKeyType struct{}

// traceIDContextKey はTraceIDをコンテキストに格納するキー。
var traceIDContextKey = traceIDContextKeyType{}

// ContextWithTraceID はTraceIDをコンテキストに設定する。
func ContextWithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDContextKey, traceID)
}
