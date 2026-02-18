package eap

// EAPState はEAP認証の状態を表す型（D-03セクション2.2準拠）
type EAPState string

// EAP認証状態の定数（8状態）
const (
	StateNew              EAPState = "NEW"               // 初期状態
	StateWaitingIdentity  EAPState = "WAITING_IDENTITY"  // 永続ID応答待ち
	StateIdentityReceived EAPState = "IDENTITY_RECEIVED" // 永続ID受領済み
	StateWaitingVector    EAPState = "WAITING_VECTOR"    // Vector Gateway応答待ち
	StateChallengeSent    EAPState = "CHALLENGE_SENT"    // Challenge送信済み
	StateResyncSent       EAPState = "RESYNC_SENT"       // 再同期処理中
	StateSuccess          EAPState = "SUCCESS"           // 認証成功（終了状態）
	StateFailure          EAPState = "FAILURE"           // 認証失敗（終了状態）
)

// StateEvent はEAP認証の状態遷移イベントを表す型（D-03セクション2.4準拠）
type StateEvent string

// EAP認証イベントの定数（16イベント）
const (
	EventPermanentIdentity   StateEvent = "PERMANENT_IDENTITY"   // 永続ID受信（'0','6'）
	EventPseudonymIdentity   StateEvent = "PSEUDONYM_IDENTITY"   // 仮名/再認証ID受信（'2','4','7','8'）
	EventUnsupportedIdentity StateEvent = "UNSUPPORTED_IDENTITY" // 非対応ID（EAP-SIM: '1','3','5'）
	EventInvalidIdentity     StateEvent = "INVALID_IDENTITY"     // 不正形式
	EventVectorRequest       StateEvent = "VECTOR_REQUEST"       // Vector Gateway呼び出し開始
	EventVectorSuccess       StateEvent = "VECTOR_SUCCESS"       // Vector API成功
	EventVectorError         StateEvent = "VECTOR_ERROR"         // Vector APIエラー
	EventChallengeSent       StateEvent = "CHALLENGE_SENT"       // Challenge送信完了
	EventChallengeOK         StateEvent = "CHALLENGE_OK"         // MAC/RES検証OK + ポリシーOK
	EventChallengeFail       StateEvent = "CHALLENGE_FAIL"       // 検証NG / ポリシーNG
	EventSyncFailure         StateEvent = "SYNC_FAILURE"         // 同期失敗受信（resync < 上限）
	EventResyncLimit         StateEvent = "RESYNC_LIMIT"         // 再同期上限超過
	EventAuthReject          StateEvent = "AUTH_REJECT"          // Authentication-Reject受信
	EventClientError         StateEvent = "CLIENT_ERROR"         // Client-Error受信
	EventResyncSuccess       StateEvent = "RESYNC_SUCCESS"       // 再同期Vector成功
	EventResyncError         StateEvent = "RESYNC_ERROR"         // 再同期Vectorエラー
)

// transitionTable はEAP状態遷移テーブル（D-03セクション2.3準拠）
var transitionTable = map[EAPState]map[StateEvent]EAPState{
	StateNew: {
		EventPermanentIdentity:   StateIdentityReceived,
		EventPseudonymIdentity:   StateWaitingIdentity,
		EventUnsupportedIdentity: StateFailure,
		EventInvalidIdentity:     StateFailure,
	},
	StateWaitingIdentity: {
		EventPermanentIdentity:   StateIdentityReceived,
		EventPseudonymIdentity:   StateFailure, // 永続ID応答拒否
		EventUnsupportedIdentity: StateFailure,
		EventInvalidIdentity:     StateFailure,
		EventClientError:         StateFailure,
	},
	StateIdentityReceived: {
		EventVectorRequest: StateWaitingVector,
	},
	StateWaitingVector: {
		EventVectorSuccess: StateChallengeSent,
		EventVectorError:   StateFailure,
	},
	StateChallengeSent: {
		EventChallengeOK:   StateSuccess,
		EventChallengeFail: StateFailure,
		EventSyncFailure:   StateResyncSent,
		EventResyncLimit:   StateFailure,
		EventAuthReject:    StateFailure,
		EventClientError:   StateFailure,
	},
	StateResyncSent: {
		EventResyncSuccess: StateChallengeSent,
		EventResyncError:   StateFailure,
	},
}

// ValidateTransition は現在の状態とイベントから次の状態を返す。
// 無効な遷移の場合はErrInvalidStateを返す。
func ValidateTransition(current EAPState, event StateEvent) (EAPState, error) {
	// 終了状態からの遷移は不可
	if IsTerminal(current) {
		return "", ErrInvalidState
	}

	events, ok := transitionTable[current]
	if !ok {
		return "", ErrInvalidState
	}

	next, ok := events[event]
	if !ok {
		return "", ErrInvalidState
	}

	return next, nil
}

// IsTerminal は指定された状態が終了状態（SUCCESS/FAILURE）かどうかを判定する。
func IsTerminal(state EAPState) bool {
	return state == StateSuccess || state == StateFailure
}

// validStates は有効なEAPState一覧
var validStates = map[EAPState]struct{}{
	StateNew:              {},
	StateWaitingIdentity:  {},
	StateIdentityReceived: {},
	StateWaitingVector:    {},
	StateChallengeSent:    {},
	StateResyncSent:       {},
	StateSuccess:          {},
	StateFailure:          {},
}

// IsValidState は文字列が有効なEAPStateかどうかを判定する。
func IsValidState(s string) bool {
	_, ok := validStates[EAPState(s)]
	return ok
}
