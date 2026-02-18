package dto

import "testing"

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
