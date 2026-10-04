package backend

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/httputil"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/logging"
)

const (
	akaOnlyBackendID   = "01"
	akaOnlyBackendName = "aka-only-server"

	// akaOnlyHSSAuthType はGenerateAvで要求する認証タイプ。
	// EAP-AKA'でもCK/IKを受け取り、CK'/IK'はauth-serverが導出する。
	akaOnlyHSSAuthType = "EAP_AKA"

	// akaOnlyMaxResponseBytes は応答ボディの読み込み上限。
	akaOnlyMaxResponseBytes = 64 << 10
)

// AKAOnlyOptions はAKAOnlyBackendの生成条件。
type AKAOnlyOptions struct {
	// BaseURL はaka-only-serverのベースURL（末尾の"/"なし）。スキームでmTLSと平文HTTPを切り替える
	BaseURL string
	// ClientCertFile はクライアント証明書のPEMファイル（https時に必須）
	ClientCertFile string
	// ClientKeyFile は秘密鍵のPEMファイル。空ならClientCertFileから読む
	ClientKeyFile string
	// ServerCertFile は信頼するAV用サーバー証明書のPEMファイル（https時に必須）
	ServerCertFile string
	// Timeout は呼び出しのタイムアウト
	Timeout time.Duration
	// MaskIMSI はログ出力時にIMSIをマスクするか
	MaskIMSI bool
}

// AKAOnlyBackend はaka-only-server（Nudm_UEAU GenerateAvベースAPI）へのバックエンド。
type AKAOnlyBackend struct {
	baseURL  string
	client   *http.Client
	maskIMSI bool
}

// generateAVRequest はGenerateAvのリクエストボディ。
type generateAVRequest struct {
	HSSAuthType           string                 `json:"hssAuthType"`
	NumOfRequestedVectors int                    `json:"numOfRequestedVectors"`
	ResynchronizationInfo *resynchronizationInfo `json:"resynchronizationInfo,omitempty"`
}

// resynchronizationInfo はGenerateAvの再同期情報。
type resynchronizationInfo struct {
	RAND string `json:"rand"`
	AUTS string `json:"auts"`
}

// generateAVResponse はGenerateAvの成功レスポンス。
type generateAVResponse struct {
	HSSAuthenticationVectors []hssAuthVector `json:"hssAuthenticationVectors"`
}

// hssAuthVector はEAP-AKAの認証ベクター。
type hssAuthVector struct {
	AVType string `json:"avType"`
	RAND   string `json:"rand"`
	XRES   string `json:"xres"`
	AUTN   string `json:"autn"`
	CK     string `json:"ck"`
	IK     string `json:"ik"`
}

// akaOnlyProblem はaka-only-serverのエラー応答（application/problem+json）。
type akaOnlyProblem struct {
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
	Cause  string `json:"cause"`
}

// NewAKAOnlyBackend は新しいAKAOnlyBackendを生成する。
// httpsの場合は証明書を読み込み、mTLSのクライアントを組み立てる。
func NewAKAOnlyBackend(opts AKAOnlyOptions) (*AKAOnlyBackend, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	switch {
	case strings.HasPrefix(opts.BaseURL, "https://"):
		tlsCfg, err := newAKAOnlyTLSConfig(opts)
		if err != nil {
			return nil, err
		}
		transport.TLSClientConfig = tlsCfg
	case strings.HasPrefix(opts.BaseURL, "http://"):
		// 平文HTTPでは証明書の設定を使わない
	default:
		return nil, fmt.Errorf("aka-only-server URL must start with http:// or https://: %q", opts.BaseURL)
	}

	return &AKAOnlyBackend{
		baseURL: strings.TrimRight(opts.BaseURL, "/"),
		client: &http.Client{
			Timeout:   opts.Timeout,
			Transport: transport,
		},
		maskIMSI: opts.MaskIMSI,
	}, nil
}

// newAKAOnlyTLSConfig はmTLS用のTLS設定を組み立てる。
// 信頼するのはServerCertFileの証明書だけで、システムのCAは使わない。
func newAKAOnlyTLSConfig(opts AKAOnlyOptions) (*tls.Config, error) {
	if opts.ClientCertFile == "" {
		return nil, errors.New("client certificate is required for https")
	}
	if opts.ServerCertFile == "" {
		return nil, errors.New("server certificate is required for https")
	}

	certPEM, err := os.ReadFile(opts.ClientCertFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read client certificate: %w", err)
	}
	keyPEM := certPEM
	if opts.ClientKeyFile != "" {
		keyPEM, err = os.ReadFile(opts.ClientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read client key: %w", err)
		}
	}
	clientCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse client certificate and key: %w", err)
	}

	serverPEM, err := os.ReadFile(opts.ServerCertFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read server certificate: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(serverPEM) {
		return nil, fmt.Errorf("no valid certificate found in server certificate file %q", opts.ServerCertFile)
	}

	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		RootCAs:      roots,
		Certificates: []tls.Certificate{clientCert},
	}, nil
}

// GetVector はaka-only-serverからEAP-AKAのベクターを1つ取得する。
func (b *AKAOnlyBackend) GetVector(ctx context.Context, req *VectorRequest) (*VectorResponse, error) {
	traceID, _ := ctx.Value(traceIDContextKey).(string)
	start := time.Now()

	resp, status, cause, err := b.call(ctx, traceID, req)
	latencyMs := time.Since(start).Milliseconds()

	if err != nil {
		attrs := []any{
			"trace_id", traceID,
			"event_id", "BACKEND_EXTERNAL_ERR",
			"imsi", logging.MaskIMSI(req.IMSI, b.maskIMSI),
			"backend_id", akaOnlyBackendID,
			"http_status", status,
			"latency_ms", latencyMs,
			"cause", cause,
			"error", err.Error(),
		}
		// 4xxは加入者・クライアント起因のためWARN、通信失敗はERROR
		var respErr *BackendResponseError
		if errors.As(err, &respErr) {
			slog.Warn("aka-only-server rejected request", attrs...)
		} else {
			slog.Error("aka-only-server call failed", attrs...)
		}
		return nil, err
	}

	slog.Info("aka-only-server call succeeded",
		"trace_id", traceID,
		"event_id", "BACKEND_EXTERNAL_CALL",
		"imsi", logging.MaskIMSI(req.IMSI, b.maskIMSI),
		"backend_id", akaOnlyBackendID,
		"external_endpoint", b.baseURL,
		"http_status", status,
		"latency_ms", latencyMs,
		"resync", req.ResyncInfo != nil,
	)
	return resp, nil
}

// call はGenerateAvを呼び出し、内部IF形式に変換する。
// ログ出力用にHTTPステータス（応答がない場合は0）とcauseも返す。
func (b *AKAOnlyBackend) call(ctx context.Context, traceID string, req *VectorRequest) (*VectorResponse, int, string, error) {
	avReq := generateAVRequest{
		HSSAuthType:           akaOnlyHSSAuthType,
		NumOfRequestedVectors: 1,
	}
	if req.ResyncInfo != nil {
		avReq.ResynchronizationInfo = &resynchronizationInfo{
			RAND: req.ResyncInfo.RAND,
			AUTS: req.ResyncInfo.AUTS,
		}
	}
	body, err := json.Marshal(avReq)
	if err != nil {
		return nil, 0, "", &BackendCommunicationError{Err: fmt.Errorf("failed to marshal request: %w", err)}
	}

	endpoint := b.baseURL + "/nudm-ueau/v1/imsi-" + url.PathEscape(req.IMSI) + "/hss-security-information/eap-aka/generate-av"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, 0, "", &BackendCommunicationError{Err: fmt.Errorf("failed to create request: %w", err)}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, application/problem+json")
	// X-Trace-IDはベストエフォート（aka-only-serverは参照しない）
	if traceID != "" {
		httpReq.Header.Set(traceIDHeader, traceID)
	}

	resp, err := b.client.Do(httpReq)
	if err != nil {
		// url.ErrorのメッセージはURL（パスにIMSIを含む）を含むため、原因のエラーだけを残す
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return nil, 0, "", &BackendCommunicationError{Err: fmt.Errorf("failed to send request: %w", err)}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, akaOnlyMaxResponseBytes+1))
	if err != nil {
		return nil, resp.StatusCode, "", &BackendCommunicationError{Err: fmt.Errorf("failed to read response: %w", err)}
	}
	if len(respBody) > akaOnlyMaxResponseBytes {
		return nil, resp.StatusCode, "", &BackendCommunicationError{
			Err: fmt.Errorf("response body exceeds %d bytes", akaOnlyMaxResponseBytes),
		}
	}

	switch {
	case resp.StatusCode == http.StatusOK:
		v, err := parseGenerateAVResponse(respBody)
		if err != nil {
			return nil, resp.StatusCode, "", &BackendCommunicationError{Err: err}
		}
		return v, resp.StatusCode, "", nil

	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		var p akaOnlyProblem
		if err := json.Unmarshal(respBody, &p); err != nil || p.Cause == "" {
			// aka-only-serverのProblemDetailsでない4xx（URLの誤り等）は加入者起因と区別できないため通信エラーとする
			return nil, resp.StatusCode, "", &BackendCommunicationError{
				Err: fmt.Errorf("unexpected error response with status %d", resp.StatusCode),
			}
		}
		title := p.Title
		if title == "" {
			title = http.StatusText(resp.StatusCode)
		}
		return nil, resp.StatusCode, p.Cause, &BackendResponseError{
			StatusCode: resp.StatusCode,
			Problem:    httputil.NewProblemDetail(resp.StatusCode, title, p.Detail),
		}

	default:
		// 5xx（501を含む）とその他のステータスは通信エラー
		var p akaOnlyProblem
		_ = json.Unmarshal(respBody, &p)
		return nil, resp.StatusCode, p.Cause, &BackendCommunicationError{
			Err: fmt.Errorf("backend returned status %d", resp.StatusCode),
		}
	}
}

// parseGenerateAVResponse はGenerateAvの成功レスポンスを内部IF形式に変換する。
func parseGenerateAVResponse(body []byte) (*VectorResponse, error) {
	var avResp generateAVResponse
	if err := json.Unmarshal(body, &avResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	if len(avResp.HSSAuthenticationVectors) == 0 {
		return nil, errors.New("response contains no authentication vector")
	}

	av := avResp.HSSAuthenticationVectors[0]
	var missing []string
	for _, f := range []struct{ name, value string }{
		{"rand", av.RAND},
		{"autn", av.AUTN},
		{"xres", av.XRES},
		{"ck", av.CK},
		{"ik", av.IK},
	} {
		if f.value == "" {
			missing = append(missing, f.name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("authentication vector is missing fields: %s", strings.Join(missing, ","))
	}

	return &VectorResponse{
		RAND: av.RAND,
		AUTN: av.AUTN,
		XRES: av.XRES,
		CK:   av.CK,
		IK:   av.IK,
	}, nil
}

// ID はバックエンドIDを返す。
func (b *AKAOnlyBackend) ID() string {
	return akaOnlyBackendID
}

// Name はバックエンド名を返す。
func (b *AKAOnlyBackend) Name() string {
	return akaOnlyBackendName
}

// UseTLS はmTLSで接続するかを返す。
func (b *AKAOnlyBackend) UseTLS() bool {
	return strings.HasPrefix(b.baseURL, "https://")
}
