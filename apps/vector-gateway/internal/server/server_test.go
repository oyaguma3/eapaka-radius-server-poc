package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-gateway/internal/backend"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-gateway/internal/config"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-gateway/internal/handler"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-gateway/internal/router"
)

// newTestServer は内部Vector APIをbackendURLに向けたServerを生成する。
func newTestServer(t *testing.T, backendURL string) *Server {
	t.Helper()
	cfg := &config.Config{
		InternalURL:     backendURL,
		InternalTimeout: 5 * time.Second,
		ListenAddr:      "127.0.0.1:0",
		GinMode:         gin.TestMode,
		LogMaskIMSI:     true,
		Mode:            "passthrough",
	}
	reg, err := backend.NewRegistry(cfg)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	r := router.NewRouter(map[string]string{}, reg, true)
	return New(cfg, handler.NewVectorHandler(r, cfg))
}

// captureLog はテスト中のslog出力をバッファに取り込む。
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestServer_Health(t *testing.T) {
	s := newTestServer(t, "http://127.0.0.1:1")

	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestServer_VectorWithTraceID(t *testing.T) {
	var gotTraceID string
	vectorAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTraceID = r.Header.Get("X-Trace-ID")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&backend.VectorResponse{RAND: "r"})
	}))
	defer vectorAPI.Close()
	s := newTestServer(t, vectorAPI.URL)
	logBuf := captureLog(t)

	tests := []struct {
		name        string
		traceID     string
		wantTraceID string
	}{
		{"with trace id", "trace-xyz", "trace-xyz"},
		{"without trace id", "", "no-trace-id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/vector", strings.NewReader(`{"imsi":"440101234567890"}`))
			req.Header.Set("Content-Type", "application/json")
			if tt.traceID != "" {
				req.Header.Set("X-Trace-ID", tt.traceID)
			}
			w := httptest.NewRecorder()
			s.engine.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("Status = %d, want %d", w.Code, http.StatusOK)
			}
			if gotTraceID != tt.wantTraceID {
				t.Errorf("X-Trace-ID = %q, want %q", gotTraceID, tt.wantTraceID)
			}
		})
	}
	if !strings.Contains(logBuf.String(), `"msg":"request completed"`) {
		t.Errorf("request log not found: %s", logBuf.String())
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	captureLog(t)
	engine := gin.New()
	engine.Use(TraceIDMiddleware(), RecoveryMiddleware())
	engine.GET("/panic", func(*gin.Context) { panic("boom") })

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestServer_RunAndShutdown(t *testing.T) {
	captureLog(t)
	s := newTestServer(t, "http://127.0.0.1:1")

	errCh := make(chan error, 1)
	go func() { errCh <- s.Run() }()

	// 起動を待たずにShutdownしても、Runはhttp.ErrServerClosedで戻る
	time.Sleep(50 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		t.Errorf("Run() error = %v, want http.ErrServerClosed", err)
	}
}
