package model

import "testing"

func TestStageConstants(t *testing.T) {
	tests := []struct {
		stage Stage
		want  string
	}{
		{StageNew, "new"},
		{StageWaitingIdentity, "waiting_identity"},
		{StageIdentityReceived, "identity_received"},
		{StageWaitingVector, "waiting_vector"},
		{StageChallengeSent, "challenge_sent"},
		{StageResyncSent, "resync_sent"},
		{StageSuccess, "success"},
		{StageFailure, "failure"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.stage) != tt.want {
				t.Errorf("Stage = %q, want %q", tt.stage, tt.want)
			}
		})
	}
}

func TestNewSession(t *testing.T) {
	sess := NewSession(
		"uuid-12345678",
		"440101234567890",
		"192.168.1.1",
		"10.0.0.100",
		"acct-session-001",
		1704067200,
	)

	if sess.UUID != "uuid-12345678" {
		t.Errorf("UUID = %q, want %q", sess.UUID, "uuid-12345678")
	}
	if sess.IMSI != "440101234567890" {
		t.Errorf("IMSI = %q, want %q", sess.IMSI, "440101234567890")
	}
	if sess.NasIP != "192.168.1.1" {
		t.Errorf("NasIP = %q, want %q", sess.NasIP, "192.168.1.1")
	}
	if sess.ClientIP != "10.0.0.100" {
		t.Errorf("ClientIP = %q, want %q", sess.ClientIP, "10.0.0.100")
	}
	if sess.AcctSessionID != "acct-session-001" {
		t.Errorf("AcctSessionID = %q, want %q", sess.AcctSessionID, "acct-session-001")
	}
	if sess.StartTime != 1704067200 {
		t.Errorf("StartTime = %d, want %d", sess.StartTime, 1704067200)
	}
	if sess.InputOctets != 0 {
		t.Errorf("InputOctets = %d, want %d", sess.InputOctets, 0)
	}
	if sess.OutputOctets != 0 {
		t.Errorf("OutputOctets = %d, want %d", sess.OutputOctets, 0)
	}
}

func TestNewEAPContext(t *testing.T) {
	ctx := NewEAPContext("trace-abc123", "440101234567890", 23)

	if ctx.TraceID != "trace-abc123" {
		t.Errorf("TraceID = %q, want %q", ctx.TraceID, "trace-abc123")
	}
	if ctx.IMSI != "440101234567890" {
		t.Errorf("IMSI = %q, want %q", ctx.IMSI, "440101234567890")
	}
	if ctx.EAPType != 23 {
		t.Errorf("EAPType = %d, want %d", ctx.EAPType, 23)
	}
	if ctx.Stage != StageNew {
		t.Errorf("Stage = %q, want %q", ctx.Stage, StageNew)
	}
	if ctx.RAND != "" {
		t.Errorf("RAND = %q, want empty", ctx.RAND)
	}
	if ctx.AUTN != "" {
		t.Errorf("AUTN = %q, want empty", ctx.AUTN)
	}
	if ctx.XRES != "" {
		t.Errorf("XRES = %q, want empty", ctx.XRES)
	}
	if ctx.Kaut != "" {
		t.Errorf("Kaut = %q, want empty", ctx.Kaut)
	}
	if ctx.MSK != "" {
		t.Errorf("MSK = %q, want empty", ctx.MSK)
	}
	if ctx.ResyncCount != 0 {
		t.Errorf("ResyncCount = %d, want %d", ctx.ResyncCount, 0)
	}
	if ctx.PermanentIDRequested {
		t.Error("PermanentIDRequested = true, want false")
	}
}

func TestEAPContextWithAKAPrime(t *testing.T) {
	// EAP-AKA' (type 50)
	ctx := NewEAPContext("trace-xyz789", "440109876543210", 50)

	if ctx.EAPType != 50 {
		t.Errorf("EAPType = %d, want %d (AKA')", ctx.EAPType, 50)
	}
}
