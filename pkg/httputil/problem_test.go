package httputil

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestNewProblemDetail(t *testing.T) {
	p := NewProblemDetail(400, "Bad Request", "Invalid IMSI format")

	if p.Type != "about:blank" {
		t.Errorf("Type = %q, want %q", p.Type, "about:blank")
	}
	if p.Title != "Bad Request" {
		t.Errorf("Title = %q, want %q", p.Title, "Bad Request")
	}
	if p.Status != 400 {
		t.Errorf("Status = %d, want %d", p.Status, 400)
	}
	if p.Detail != "Invalid IMSI format" {
		t.Errorf("Detail = %q, want %q", p.Detail, "Invalid IMSI format")
	}
}

func TestBadRequest(t *testing.T) {
	p := BadRequest("missing required field")
	if p.Status != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", p.Status, http.StatusBadRequest)
	}
	if p.Title != "Bad Request" {
		t.Errorf("Title = %q, want %q", p.Title, "Bad Request")
	}
}

func TestNotFound(t *testing.T) {
	p := NotFound("subscriber not found")
	if p.Status != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", p.Status, http.StatusNotFound)
	}
	if p.Title != "Not Found" {
		t.Errorf("Title = %q, want %q", p.Title, "Not Found")
	}
}

func TestInternalServerError(t *testing.T) {
	p := InternalServerError("database connection failed")
	if p.Status != http.StatusInternalServerError {
		t.Errorf("Status = %d, want %d", p.Status, http.StatusInternalServerError)
	}
	if p.Title != "Internal Server Error" {
		t.Errorf("Title = %q, want %q", p.Title, "Internal Server Error")
	}
}

func TestBadGateway(t *testing.T) {
	p := BadGateway("backend service unavailable")
	if p.Status != http.StatusBadGateway {
		t.Errorf("Status = %d, want %d", p.Status, http.StatusBadGateway)
	}
	if p.Title != "Bad Gateway" {
		t.Errorf("Title = %q, want %q", p.Title, "Bad Gateway")
	}
}

func TestNotImplemented(t *testing.T) {
	p := NotImplemented("feature not implemented")
	if p.Status != http.StatusNotImplemented {
		t.Errorf("Status = %d, want %d", p.Status, http.StatusNotImplemented)
	}
	if p.Title != "Not Implemented" {
		t.Errorf("Title = %q, want %q", p.Title, "Not Implemented")
	}
}

func TestServiceUnavailable(t *testing.T) {
	p := ServiceUnavailable("service under maintenance")
	if p.Status != http.StatusServiceUnavailable {
		t.Errorf("Status = %d, want %d", p.Status, http.StatusServiceUnavailable)
	}
	if p.Title != "Service Unavailable" {
		t.Errorf("Title = %q, want %q", p.Title, "Service Unavailable")
	}
}

func TestProblemDetailJSON(t *testing.T) {
	p := BadRequest("test detail")
	data, err := p.JSON()
	if err != nil {
		t.Fatalf("JSON() error = %v", err)
	}

	// JSONをパースして確認
	var parsed ProblemDetail
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if parsed.Type != p.Type {
		t.Errorf("parsed.Type = %q, want %q", parsed.Type, p.Type)
	}
	if parsed.Title != p.Title {
		t.Errorf("parsed.Title = %q, want %q", parsed.Title, p.Title)
	}
	if parsed.Status != p.Status {
		t.Errorf("parsed.Status = %d, want %d", parsed.Status, p.Status)
	}
	if parsed.Detail != p.Detail {
		t.Errorf("parsed.Detail = %q, want %q", parsed.Detail, p.Detail)
	}
}

func TestProblemDetailMustJSON(t *testing.T) {
	p := NotFound("resource not found")
	data := p.MustJSON()
	if len(data) == 0 {
		t.Error("MustJSON() returned empty data")
	}

	// 正しいJSON形式であることを確認
	var parsed ProblemDetail
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("MustJSON() returned invalid JSON: %v", err)
	}
}

func TestProblemDetailJSONOmitsEmptyDetail(t *testing.T) {
	p := NewProblemDetail(500, "Internal Server Error", "")
	data, err := p.JSON()
	if err != nil {
		t.Fatalf("JSON() error = %v", err)
	}

	// detailフィールドが含まれていないことを確認
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if _, exists := raw["detail"]; exists {
		t.Error("JSON should omit empty detail field")
	}
}

func TestContentType(t *testing.T) {
	if ContentType != "application/problem+json" {
		t.Errorf("ContentType = %q, want %q", ContentType, "application/problem+json")
	}
}
