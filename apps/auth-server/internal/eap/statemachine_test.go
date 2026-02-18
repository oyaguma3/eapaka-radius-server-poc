package eap

import (
	"testing"
)

// TestValidateTransition_Valid は有効な遷移パターンをテーブル駆動で検証する
func TestValidateTransition_Valid(t *testing.T) {
	tests := []struct {
		name    string
		current EAPState
		event   StateEvent
		want    EAPState
	}{
		// NEW状態からの遷移
		{"NEW->IDENTITY_RECEIVED(永続ID)", StateNew, EventPermanentIdentity, StateIdentityReceived},
		{"NEW->WAITING_IDENTITY(仮名ID)", StateNew, EventPseudonymIdentity, StateWaitingIdentity},
		{"NEW->FAILURE(非対応ID)", StateNew, EventUnsupportedIdentity, StateFailure},
		{"NEW->FAILURE(不正形式)", StateNew, EventInvalidIdentity, StateFailure},

		// WAITING_IDENTITY状態からの遷移
		{"WAITING_IDENTITY->IDENTITY_RECEIVED(永続ID)", StateWaitingIdentity, EventPermanentIdentity, StateIdentityReceived},
		{"WAITING_IDENTITY->FAILURE(仮名ID拒否)", StateWaitingIdentity, EventPseudonymIdentity, StateFailure},
		{"WAITING_IDENTITY->FAILURE(非対応ID)", StateWaitingIdentity, EventUnsupportedIdentity, StateFailure},
		{"WAITING_IDENTITY->FAILURE(不正形式)", StateWaitingIdentity, EventInvalidIdentity, StateFailure},
		{"WAITING_IDENTITY->FAILURE(ClientError)", StateWaitingIdentity, EventClientError, StateFailure},

		// IDENTITY_RECEIVED状態からの遷移
		{"IDENTITY_RECEIVED->WAITING_VECTOR", StateIdentityReceived, EventVectorRequest, StateWaitingVector},

		// WAITING_VECTOR状態からの遷移
		{"WAITING_VECTOR->CHALLENGE_SENT(成功)", StateWaitingVector, EventVectorSuccess, StateChallengeSent},
		{"WAITING_VECTOR->FAILURE(エラー)", StateWaitingVector, EventVectorError, StateFailure},

		// CHALLENGE_SENT状態からの遷移
		{"CHALLENGE_SENT->SUCCESS(検証OK)", StateChallengeSent, EventChallengeOK, StateSuccess},
		{"CHALLENGE_SENT->FAILURE(検証NG)", StateChallengeSent, EventChallengeFail, StateFailure},
		{"CHALLENGE_SENT->RESYNC_SENT(同期失敗)", StateChallengeSent, EventSyncFailure, StateResyncSent},
		{"CHALLENGE_SENT->FAILURE(再同期上限)", StateChallengeSent, EventResyncLimit, StateFailure},
		{"CHALLENGE_SENT->FAILURE(AuthReject)", StateChallengeSent, EventAuthReject, StateFailure},
		{"CHALLENGE_SENT->FAILURE(ClientError)", StateChallengeSent, EventClientError, StateFailure},

		// RESYNC_SENT状態からの遷移
		{"RESYNC_SENT->CHALLENGE_SENT(再同期成功)", StateResyncSent, EventResyncSuccess, StateChallengeSent},
		{"RESYNC_SENT->FAILURE(再同期エラー)", StateResyncSent, EventResyncError, StateFailure},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateTransition(tt.current, tt.event)
			if err != nil {
				t.Fatalf("予期しないエラー: %v", err)
			}
			if got != tt.want {
				t.Errorf("遷移結果 = %q, 期待値 = %q", got, tt.want)
			}
		})
	}
}

// TestValidateTransition_Invalid は無効な遷移パターンを検証する
func TestValidateTransition_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		current EAPState
		event   StateEvent
	}{
		// NEW状態で無効なイベント
		{"NEW+VectorRequest", StateNew, EventVectorRequest},
		{"NEW+ChallengeOK", StateNew, EventChallengeOK},
		{"NEW+ClientError", StateNew, EventClientError},

		// WAITING_IDENTITY状態で無効なイベント
		{"WAITING_IDENTITY+VectorRequest", StateWaitingIdentity, EventVectorRequest},
		{"WAITING_IDENTITY+ChallengeOK", StateWaitingIdentity, EventChallengeOK},

		// IDENTITY_RECEIVED状態で無効なイベント
		{"IDENTITY_RECEIVED+PermanentIdentity", StateIdentityReceived, EventPermanentIdentity},
		{"IDENTITY_RECEIVED+ChallengeOK", StateIdentityReceived, EventChallengeOK},

		// WAITING_VECTOR状態で無効なイベント
		{"WAITING_VECTOR+PermanentIdentity", StateWaitingVector, EventPermanentIdentity},
		{"WAITING_VECTOR+ChallengeOK", StateWaitingVector, EventChallengeOK},

		// CHALLENGE_SENT状態で無効なイベント
		{"CHALLENGE_SENT+PermanentIdentity", StateChallengeSent, EventPermanentIdentity},
		{"CHALLENGE_SENT+VectorRequest", StateChallengeSent, EventVectorRequest},

		// RESYNC_SENT状態で無効なイベント
		{"RESYNC_SENT+PermanentIdentity", StateResyncSent, EventPermanentIdentity},
		{"RESYNC_SENT+ChallengeOK", StateResyncSent, EventChallengeOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateTransition(tt.current, tt.event)
			if err != ErrInvalidState {
				t.Errorf("エラー = %v, 期待値 = %v (遷移先: %q)", err, ErrInvalidState, got)
			}
			if got != "" {
				t.Errorf("遷移先 = %q, 期待値 = 空文字列", got)
			}
		})
	}
}

// TestValidateTransition_TerminalStates は終了状態からの遷移が不可であることを検証する
func TestValidateTransition_TerminalStates(t *testing.T) {
	terminalStates := []EAPState{StateSuccess, StateFailure}
	allEvents := []StateEvent{
		EventPermanentIdentity, EventPseudonymIdentity,
		EventUnsupportedIdentity, EventInvalidIdentity,
		EventVectorRequest, EventVectorSuccess, EventVectorError,
		EventChallengeSent, EventChallengeOK, EventChallengeFail,
		EventSyncFailure, EventResyncLimit, EventAuthReject,
		EventClientError, EventResyncSuccess, EventResyncError,
	}

	for _, state := range terminalStates {
		for _, event := range allEvents {
			t.Run(string(state)+"+"+string(event), func(t *testing.T) {
				got, err := ValidateTransition(state, event)
				if err != ErrInvalidState {
					t.Errorf("エラー = %v, 期待値 = %v", err, ErrInvalidState)
				}
				if got != "" {
					t.Errorf("遷移先 = %q, 期待値 = 空文字列", got)
				}
			})
		}
	}
}

// TestValidateTransition_UnknownState は未定義の状態文字列での遷移を検証する
func TestValidateTransition_UnknownState(t *testing.T) {
	tests := []struct {
		name    string
		current EAPState
		event   StateEvent
	}{
		{"未定義状態+有効イベント", EAPState("UNKNOWN"), EventPermanentIdentity},
		{"空文字列状態+有効イベント", EAPState(""), EventVectorRequest},
		{"不正状態+不正イベント", EAPState("INVALID"), StateEvent("INVALID_EVENT")},
		{"有効状態+未定義イベント", StateNew, StateEvent("NONEXISTENT")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateTransition(tt.current, tt.event)
			if err != ErrInvalidState {
				t.Errorf("エラー = %v, 期待値 = %v", err, ErrInvalidState)
			}
			if got != "" {
				t.Errorf("遷移先 = %q, 期待値 = 空文字列", got)
			}
		})
	}
}

// TestIsTerminal は全8状態の終了状態判定を検証する
func TestIsTerminal(t *testing.T) {
	tests := []struct {
		state EAPState
		want  bool
	}{
		{StateNew, false},
		{StateWaitingIdentity, false},
		{StateIdentityReceived, false},
		{StateWaitingVector, false},
		{StateChallengeSent, false},
		{StateResyncSent, false},
		{StateSuccess, true},
		{StateFailure, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			got := IsTerminal(tt.state)
			if got != tt.want {
				t.Errorf("IsTerminal(%q) = %v, 期待値 = %v", tt.state, got, tt.want)
			}
		})
	}
}

// TestIsValidState は有効な状態文字列と無効なパターンを検証する
func TestIsValidState(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		// 有効な8状態
		{"NEW", true},
		{"WAITING_IDENTITY", true},
		{"IDENTITY_RECEIVED", true},
		{"WAITING_VECTOR", true},
		{"CHALLENGE_SENT", true},
		{"RESYNC_SENT", true},
		{"SUCCESS", true},
		{"FAILURE", true},

		// 無効なパターン
		{"", false},
		{"UNKNOWN", false},
		{"new", false},      // 小文字
		{"New", false},      // 先頭大文字のみ
		{"WAITING", false},  // 部分一致
		{"SUCCESS ", false}, // 末尾スペース
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsValidState(tt.input)
			if got != tt.want {
				t.Errorf("IsValidState(%q) = %v, 期待値 = %v", tt.input, got, tt.want)
			}
		})
	}
}
