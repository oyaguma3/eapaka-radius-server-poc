package backend

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestInternalBackend_GetVector_Success(t *testing.T) {
	// テスト用HTTPサーバー
	expected := &VectorResponse{
		RAND: "0102030405060708090a0b0c0d0e0f10",
		AUTN: "1112131415161718191a1b1c1d1e1f20",
		XRES: "2122232425262728",
		CK:   "3132333435363738393a3b3c3d3e3f40",
		IK:   "4142434445464748494a4b4c4d4e4f50",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// リクエストの検証
		if r.Method != http.MethodPost {
			t.Errorf("Method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/api/v1/vector" {
			t.Errorf("Path = %q, want %q", r.URL.Path, "/api/v1/vector")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want %q", r.Header.Get("Content-Type"), "application/json")
		}

		// リクエストボディの検証
		var req VectorRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.IMSI != "440101234567890" {
			t.Errorf("IMSI = %q, want %q", req.IMSI, "440101234567890")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer srv.Close()

	b := NewInternalBackend(srv.URL, 5*time.Second)

	req := &VectorRequest{IMSI: "440101234567890"}
	resp, err := b.GetVector(context.Background(), req)
	if err != nil {
		t.Fatalf("GetVector() error = %v", err)
	}

	if resp.RAND != expected.RAND {
		t.Errorf("RAND = %q, want %q", resp.RAND, expected.RAND)
	}
	if resp.AUTN != expected.AUTN {
		t.Errorf("AUTN = %q, want %q", resp.AUTN, expected.AUTN)
	}
	if resp.XRES != expected.XRES {
		t.Errorf("XRES = %q, want %q", resp.XRES, expected.XRES)
	}
	if resp.CK != expected.CK {
		t.Errorf("CK = %q, want %q", resp.CK, expected.CK)
	}
	if resp.IK != expected.IK {
		t.Errorf("IK = %q, want %q", resp.IK, expected.IK)
	}
}

func TestInternalBackend_GetVector_TraceIDPropagation(t *testing.T) {
	var receivedTraceID string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedTraceID = r.Header.Get("X-Trace-ID")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&VectorResponse{})
	}))
	defer srv.Close()

	b := NewInternalBackend(srv.URL, 5*time.Second)
	ctx := ContextWithTraceID(context.Background(), "test-trace-123")

	req := &VectorRequest{IMSI: "440101234567890"}
	_, err := b.GetVector(ctx, req)
	if err != nil {
		t.Fatalf("GetVector() error = %v", err)
	}

	if receivedTraceID != "test-trace-123" {
		t.Errorf("X-Trace-ID = %q, want %q", receivedTraceID, "test-trace-123")
	}
}

func TestInternalBackend_GetVector_4xxError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(&ProblemDetail{
			Type:   "about:blank",
			Title:  "Not Found",
			Detail: "subscriber not found",
			Status: 404,
		})
	}))
	defer srv.Close()

	b := NewInternalBackend(srv.URL, 5*time.Second)
	req := &VectorRequest{IMSI: "440101234567890"}
	_, err := b.GetVector(context.Background(), req)

	if err == nil {
		t.Fatal("GetVector() expected error for 404 response")
	}

	var respErr *BackendResponseError
	if !errors.As(err, &respErr) {
		t.Fatalf("expected BackendResponseError, got %T", err)
	}
	if respErr.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want %d", respErr.StatusCode, 404)
	}
	if respErr.Problem.Detail != "subscriber not found" {
		t.Errorf("Detail = %q, want %q", respErr.Problem.Detail, "subscriber not found")
	}
}

func TestInternalBackend_GetVector_5xxError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	b := NewInternalBackend(srv.URL, 5*time.Second)
	req := &VectorRequest{IMSI: "440101234567890"}
	_, err := b.GetVector(context.Background(), req)

	if err == nil {
		t.Fatal("GetVector() expected error for 500 response")
	}

	var commErr *BackendCommunicationError
	if !errors.As(err, &commErr) {
		t.Fatalf("expected BackendCommunicationError, got %T", err)
	}
}

func TestInternalBackend_GetVector_ConnectionError(t *testing.T) {
	// 存在しないURLに接続
	b := NewInternalBackend("http://127.0.0.1:1", 1*time.Second)
	req := &VectorRequest{IMSI: "440101234567890"}
	_, err := b.GetVector(context.Background(), req)

	if err == nil {
		t.Fatal("GetVector() expected error for connection failure")
	}

	var commErr *BackendCommunicationError
	if !errors.As(err, &commErr) {
		t.Fatalf("expected BackendCommunicationError, got %T", err)
	}
}

func TestInternalBackend_GetVector_WithResyncInfo(t *testing.T) {
	var receivedReq VectorRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&VectorResponse{})
	}))
	defer srv.Close()

	b := NewInternalBackend(srv.URL, 5*time.Second)
	req := &VectorRequest{
		IMSI: "440101234567890",
		ResyncInfo: &ResyncInfo{
			RAND: "aabbccdd",
			AUTS: "11223344",
		},
	}
	_, err := b.GetVector(context.Background(), req)
	if err != nil {
		t.Fatalf("GetVector() error = %v", err)
	}

	if receivedReq.ResyncInfo == nil {
		t.Fatal("ResyncInfo not received")
	}
	if receivedReq.ResyncInfo.RAND != "aabbccdd" {
		t.Errorf("ResyncInfo.RAND = %q, want %q", receivedReq.ResyncInfo.RAND, "aabbccdd")
	}
	if receivedReq.ResyncInfo.AUTS != "11223344" {
		t.Errorf("ResyncInfo.AUTS = %q, want %q", receivedReq.ResyncInfo.AUTS, "11223344")
	}
}

func TestInternalBackend_IDAndName(t *testing.T) {
	b := NewInternalBackend("http://localhost:8080", 5*time.Second)

	if b.ID() != "00" {
		t.Errorf("ID() = %q, want %q", b.ID(), "00")
	}
	if b.Name() != "Internal Vector API" {
		t.Errorf("Name() = %q, want %q", b.Name(), "Internal Vector API")
	}
}

func TestContextWithTraceID(t *testing.T) {
	ctx := ContextWithTraceID(context.Background(), "trace-abc")
	got, ok := ctx.Value(traceIDContextKey).(string)
	if !ok {
		t.Fatal("TraceID not found in context")
	}
	if got != "trace-abc" {
		t.Errorf("TraceID = %q, want %q", got, "trace-abc")
	}
}
