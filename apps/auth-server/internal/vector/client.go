package vector

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/config"
	"github.com/sony/gobreaker"
)

// Client はVector Gatewayクライアントの実装
type Client struct {
	httpClient *resty.Client
	cb         *gobreaker.CircuitBreaker
	baseURL    string
}

// NewClient は新しいVector Gatewayクライアントを生成する。
func NewClient(cfg *config.Config) *Client {
	httpClient := resty.New().
		SetTimeout(config.VectorRequestTimeout).
		SetTransport(nil) // デフォルトTransportを使用

	// DialTimeout相当の設定
	httpClient.SetTimeout(config.VectorRequestTimeout)

	cbSettings := gobreaker.Settings{
		Name:        config.CBName,
		MaxRequests: config.CBMaxRequests,
		Interval:    config.CBInterval,
		Timeout:     config.CBTimeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= uint32(config.CBFailureThreshold)
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			switch to {
			case gobreaker.StateOpen:
				slog.Warn("circuit breaker opened",
					"event_id", "CB_OPEN",
					"cb_name", name,
					"failure_count", 0, // countsは取得不可
				)
			case gobreaker.StateHalfOpen:
				slog.Info("circuit breaker half-open",
					"event_id", "CB_HALF_OPEN",
					"cb_name", name,
				)
			case gobreaker.StateClosed:
				slog.Info("circuit breaker closed",
					"event_id", "CB_CLOSE",
					"cb_name", name,
				)
			}
		},
	}

	return &Client{
		httpClient: httpClient,
		cb:         gobreaker.NewCircuitBreaker(cbSettings),
		baseURL:    strings.TrimRight(cfg.VectorAPIURL, "/"),
	}
}

// GetVector は認証ベクターを取得する。
func (c *Client) GetVector(ctx context.Context, req *VectorRequest) (*VectorResponse, error) {
	traceID, ok := ctx.Value(traceIDKey{}).(string)
	if !ok || traceID == "" {
		return nil, ErrTraceIDMissing
	}

	start := time.Now()

	result, err := c.cb.Execute(func() (any, error) {
		resp, err := c.httpClient.R().
			SetContext(ctx).
			SetHeader(HeaderTraceID, traceID).
			SetHeader(HeaderContentType, ContentTypeJSON).
			SetBody(req).
			Post(c.baseURL + "/api/v1/vector")

		if err != nil {
			return nil, &ConnectionError{Cause: err}
		}

		latencyMs := time.Since(start).Milliseconds()

		statusCode := resp.StatusCode()

		// CB失敗判定対象: 5xx（501除く）、502
		if statusCode >= 500 && statusCode != 501 {
			apiErr := c.parseAPIError(statusCode, resp.Body())
			slog.Error("vector api error",
				"event_id", "VECTOR_API_ERR",
				"error", apiErr.Error(),
				"http_status", statusCode,
				"latency_ms", latencyMs,
			)
			return nil, apiErr
		}

		// CB失敗判定対象外のエラー: 400, 404, 501
		if statusCode != 200 {
			apiErr := c.parseAPIError(statusCode, resp.Body())
			slog.Error("vector api error",
				"event_id", "VECTOR_API_ERR",
				"error", apiErr.Error(),
				"http_status", statusCode,
				"latency_ms", latencyMs,
			)
			// CB対象外エラーはnilを返してCBカウントに含めない
			return apiErr, nil
		}

		slog.Debug("vector api success",
			"latency_ms", latencyMs,
		)

		return resp.Body(), nil
	})

	if err != nil {
		// Circuit BreakerがOpen状態
		if err == gobreaker.ErrOpenState || err == gobreaker.ErrTooManyRequests {
			return nil, ErrCircuitOpen
		}
		// ConnectionErrorまたはAPIError（CB対象）をそのまま返す
		return nil, err
	}

	// CB対象外のAPIErrorの場合
	if apiErr, ok := result.(*APIError); ok {
		return nil, apiErr
	}

	// 正常レスポンスのパース
	body, ok := result.([]byte)
	if !ok {
		return nil, ErrInvalidResponse
	}

	return c.parseResponse(body)
}

// parseResponse はJSONレスポンスをVectorResponseに変換する。
func (c *Client) parseResponse(body []byte) (*VectorResponse, error) {
	var raw vectorResponseJSON
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("%w: json unmarshal: %v", ErrInvalidResponse, err)
	}

	randBytes, err := hex.DecodeString(raw.RAND)
	if err != nil {
		return nil, fmt.Errorf("%w: rand hex decode: %v", ErrInvalidResponse, err)
	}
	autnBytes, err := hex.DecodeString(raw.AUTN)
	if err != nil {
		return nil, fmt.Errorf("%w: autn hex decode: %v", ErrInvalidResponse, err)
	}
	xresBytes, err := hex.DecodeString(raw.XRES)
	if err != nil {
		return nil, fmt.Errorf("%w: xres hex decode: %v", ErrInvalidResponse, err)
	}
	ckBytes, err := hex.DecodeString(raw.CK)
	if err != nil {
		return nil, fmt.Errorf("%w: ck hex decode: %v", ErrInvalidResponse, err)
	}
	ikBytes, err := hex.DecodeString(raw.IK)
	if err != nil {
		return nil, fmt.Errorf("%w: ik hex decode: %v", ErrInvalidResponse, err)
	}

	return &VectorResponse{
		RAND: randBytes,
		AUTN: autnBytes,
		XRES: xresBytes,
		CK:   ckBytes,
		IK:   ikBytes,
	}, nil
}

// parseAPIError はHTTPエラーレスポンスをAPIErrorに変換する。
func (c *Client) parseAPIError(statusCode int, body []byte) *APIError {
	var details ProblemDetails
	if err := json.Unmarshal(body, &details); err == nil && details.Title != "" {
		return &APIError{
			StatusCode: statusCode,
			Message:    details.Title,
			Details:    &details,
		}
	}
	return &APIError{
		StatusCode: statusCode,
		Message:    string(body),
	}
}

// traceIDKey はコンテキストからTrace IDを取得するためのキー型
type traceIDKey struct{}

// WithTraceID はコンテキストにTrace IDを設定する。
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey{}, traceID)
}
