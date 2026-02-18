package engine

import (
	"context"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/config"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/eap"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/mocks"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/policy"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/session"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/vector"
	eapaka "github.com/oyaguma3/go-eapaka"
	"go.uber.org/mock/gomock"
)

// テスト用定数
const (
	testTraceID     = "test-trace-id-123"
	testIMSI        = "001010123456789"
	testNASID       = "test-nas"
	testSSID        = "TestSSID"
	testNetworkName = "WLAN"
)

// テスト用のダミーバイト列（16バイト）
var (
	testRAND = make([]byte, 16)
	testAUTN = make([]byte, 16)
	testXRES = make([]byte, 8)
	testCK   = make([]byte, 16)
	testIK   = make([]byte, 16)
)

func init() {
	// テスト用バイト列を初期化
	for i := range testRAND {
		testRAND[i] = byte(i + 1)
	}
	for i := range testAUTN {
		testAUTN[i] = byte(i + 0x10)
	}
	for i := range testXRES {
		testXRES[i] = byte(i + 0x20)
	}
	for i := range testCK {
		testCK[i] = byte(i + 0x30)
	}
	for i := range testIK {
		testIK[i] = byte(i + 0x40)
	}
}

func newTestConfig() *config.Config {
	return &config.Config{
		NetworkName: testNetworkName,
		LogMaskIMSI: true,
	}
}

// buildIdentityEAPMessage はEAP-Response/Identityパケットを構築する
func buildIdentityEAPMessage(identifier uint8, eapType uint8) []byte {
	pkt := &eapaka.Packet{
		Code:       eapaka.CodeResponse,
		Identifier: identifier,
		Type:       eapType,
		Subtype:    eapaka.SubtypeIdentity,
	}
	data, _ := pkt.Marshal()
	return data
}

// buildChallengeResponseEAPMessage はEAP-Response/AKA-Challengeパケットを構築する
func buildChallengeResponseEAPMessage(identifier uint8, eapType uint8, kAut, xres []byte) []byte {
	pkt := &eapaka.Packet{
		Code:       eapaka.CodeResponse,
		Identifier: identifier,
		Type:       eapType,
		Subtype:    eapaka.SubtypeChallenge,
		Attributes: []eapaka.Attribute{
			&eapaka.AtRes{Res: xres},
			&eapaka.AtMac{MAC: make([]byte, 16)},
		},
	}
	_ = pkt.CalculateAndSetMac(kAut)
	data, _ := pkt.Marshal()
	return data
}

// buildSyncFailureEAPMessage はEAP-Response/SynchronizationFailureパケットを構築する
func buildSyncFailureEAPMessage(identifier uint8, eapType uint8, auts []byte) []byte {
	pkt := &eapaka.Packet{
		Code:       eapaka.CodeResponse,
		Identifier: identifier,
		Type:       eapType,
		Subtype:    eapaka.SubtypeSynchronizationFailure,
		Attributes: []eapaka.Attribute{
			&eapaka.AtAuts{Auts: auts},
		},
	}
	data, _ := pkt.Marshal()
	return data
}

// buildAuthRejectEAPMessage はEAP-Response/AuthenticationRejectパケットを構築する
func buildAuthRejectEAPMessage(identifier uint8, eapType uint8) []byte {
	pkt := &eapaka.Packet{
		Code:       eapaka.CodeResponse,
		Identifier: identifier,
		Type:       eapType,
		Subtype:    eapaka.SubtypeAuthenticationReject,
	}
	data, _ := pkt.Marshal()
	return data
}

// buildClientErrorEAPMessage はEAP-Response/ClientErrorパケットを構築する
func buildClientErrorEAPMessage(identifier uint8, eapType uint8) []byte {
	pkt := &eapaka.Packet{
		Code:       eapaka.CodeResponse,
		Identifier: identifier,
		Type:       eapType,
		Subtype:    eapaka.SubtypeClientError,
		Attributes: []eapaka.Attribute{
			&eapaka.AtClientErrorCode{Code: 0},
		},
	}
	data, _ := pkt.Marshal()
	return data
}

// buildNotificationEAPMessage はEAP-Response/Notificationパケットを構築する（未知Subtype用）
func buildNotificationEAPMessage(identifier uint8, eapType uint8) []byte {
	pkt := &eapaka.Packet{
		Code:       eapaka.CodeResponse,
		Identifier: identifier,
		Type:       eapType,
		Subtype:    eapaka.SubtypeNotification,
	}
	data, _ := pkt.Marshal()
	return data
}

// --- Identity テスト ---

func TestEngine_PermanentAKA_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockVector := mocks.NewMockVectorClient(ctrl)
	mockCtxStore := mocks.NewMockContextStore(ctrl)
	mockSessStore := mocks.NewMockSessionStore(ctrl)
	mockPolicyStore := mocks.NewMockPolicyStore(ctrl)
	mockEvaluator := mocks.NewMockEvaluator(ctrl)
	cfg := newTestConfig()

	eng := NewEngine(mockVector, mockCtxStore, mockSessStore, mockPolicyStore, mockEvaluator, cfg)

	// Identity EAP-AKA
	eapMsg := buildIdentityEAPMessage(1, eapaka.TypeAKA)

	mockCtxStore.EXPECT().Create(gomock.Any(), testTraceID, gomock.Any()).Return(nil)
	mockVector.EXPECT().GetVector(gomock.Any(), &vector.VectorRequest{IMSI: testIMSI}).
		Return(&vector.VectorResponse{
			RAND: testRAND, AUTN: testAUTN, XRES: testXRES, CK: testCK, IK: testIK,
		}, nil)
	mockCtxStore.EXPECT().Update(gomock.Any(), testTraceID, gomock.Any()).Return(nil)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		EAPMessage: eapMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionChallenge {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionChallenge)
	}
	if string(result.State) != testTraceID {
		t.Errorf("State: got %q, want %q", string(result.State), testTraceID)
	}
}

func TestEngine_PermanentAKAPrime_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockVector := mocks.NewMockVectorClient(ctrl)
	mockCtxStore := mocks.NewMockContextStore(ctrl)
	mockSessStore := mocks.NewMockSessionStore(ctrl)
	mockPolicyStore := mocks.NewMockPolicyStore(ctrl)
	mockEvaluator := mocks.NewMockEvaluator(ctrl)
	cfg := newTestConfig()

	eng := NewEngine(mockVector, mockCtxStore, mockSessStore, mockPolicyStore, mockEvaluator, cfg)

	eapMsg := buildIdentityEAPMessage(1, eapaka.TypeAKAPrime)

	mockCtxStore.EXPECT().Create(gomock.Any(), testTraceID, gomock.Any()).Return(nil)
	mockVector.EXPECT().GetVector(gomock.Any(), &vector.VectorRequest{IMSI: testIMSI}).
		Return(&vector.VectorResponse{
			RAND: testRAND, AUTN: testAUTN, XRES: testXRES, CK: testCK, IK: testIK,
		}, nil)
	mockCtxStore.EXPECT().Update(gomock.Any(), testTraceID, gomock.Any()).Return(nil)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "6" + testIMSI + "@realm",
		EAPMessage: eapMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionChallenge {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionChallenge)
	}
}

func TestEngine_Pseudonym_Fallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockVector := mocks.NewMockVectorClient(ctrl)
	mockCtxStore := mocks.NewMockContextStore(ctrl)
	mockSessStore := mocks.NewMockSessionStore(ctrl)
	mockPolicyStore := mocks.NewMockPolicyStore(ctrl)
	mockEvaluator := mocks.NewMockEvaluator(ctrl)
	cfg := newTestConfig()

	eng := NewEngine(mockVector, mockCtxStore, mockSessStore, mockPolicyStore, mockEvaluator, cfg)

	eapMsg := buildIdentityEAPMessage(1, eapaka.TypeAKA)

	mockCtxStore.EXPECT().Create(gomock.Any(), testTraceID, gomock.Any()).Return(nil)
	mockCtxStore.EXPECT().Update(gomock.Any(), testTraceID, gomock.Any()).Return(nil)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "2pseudonym@realm",
		EAPMessage: eapMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionChallenge {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionChallenge)
	}
}

func TestEngine_SIM_Reject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockVector := mocks.NewMockVectorClient(ctrl)
	mockCtxStore := mocks.NewMockContextStore(ctrl)
	mockSessStore := mocks.NewMockSessionStore(ctrl)
	mockPolicyStore := mocks.NewMockPolicyStore(ctrl)
	mockEvaluator := mocks.NewMockEvaluator(ctrl)
	cfg := newTestConfig()

	eng := NewEngine(mockVector, mockCtxStore, mockSessStore, mockPolicyStore, mockEvaluator, cfg)

	eapMsg := buildIdentityEAPMessage(1, eapaka.TypeAKA)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "1001010123456789@realm",
		EAPMessage: eapMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

func TestEngine_InvalidIdentity_Reject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockVector := mocks.NewMockVectorClient(ctrl)
	mockCtxStore := mocks.NewMockContextStore(ctrl)
	mockSessStore := mocks.NewMockSessionStore(ctrl)
	mockPolicyStore := mocks.NewMockPolicyStore(ctrl)
	mockEvaluator := mocks.NewMockEvaluator(ctrl)
	cfg := newTestConfig()

	eng := NewEngine(mockVector, mockCtxStore, mockSessStore, mockPolicyStore, mockEvaluator, cfg)

	eapMsg := buildIdentityEAPMessage(1, eapaka.TypeAKA)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "Xinvalid@realm",
		EAPMessage: eapMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

func TestEngine_VectorError_Reject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockVector := mocks.NewMockVectorClient(ctrl)
	mockCtxStore := mocks.NewMockContextStore(ctrl)
	mockSessStore := mocks.NewMockSessionStore(ctrl)
	mockPolicyStore := mocks.NewMockPolicyStore(ctrl)
	mockEvaluator := mocks.NewMockEvaluator(ctrl)
	cfg := newTestConfig()

	eng := NewEngine(mockVector, mockCtxStore, mockSessStore, mockPolicyStore, mockEvaluator, cfg)

	eapMsg := buildIdentityEAPMessage(1, eapaka.TypeAKA)

	mockCtxStore.EXPECT().Create(gomock.Any(), testTraceID, gomock.Any()).Return(nil)
	mockVector.EXPECT().GetVector(gomock.Any(), gomock.Any()).
		Return(nil, &vector.ConnectionError{Cause: context.DeadlineExceeded})

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		EAPMessage: eapMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

func TestEngine_Vector404_Reject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockVector := mocks.NewMockVectorClient(ctrl)
	mockCtxStore := mocks.NewMockContextStore(ctrl)
	mockSessStore := mocks.NewMockSessionStore(ctrl)
	mockPolicyStore := mocks.NewMockPolicyStore(ctrl)
	mockEvaluator := mocks.NewMockEvaluator(ctrl)
	cfg := newTestConfig()

	eng := NewEngine(mockVector, mockCtxStore, mockSessStore, mockPolicyStore, mockEvaluator, cfg)

	eapMsg := buildIdentityEAPMessage(1, eapaka.TypeAKA)

	mockCtxStore.EXPECT().Create(gomock.Any(), testTraceID, gomock.Any()).Return(nil)
	mockVector.EXPECT().GetVector(gomock.Any(), gomock.Any()).
		Return(nil, &vector.APIError{StatusCode: 404, Message: "not found"})

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		EAPMessage: eapMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

// --- Challenge テスト ---

// newChallengeTestEngine はChallenge応答テスト用のエンジンとモックを生成する
func newChallengeTestEngine(ctrl *gomock.Controller) (
	*EngineImpl,
	*mocks.MockVectorClient,
	*mocks.MockContextStore,
	*mocks.MockSessionStore,
	*mocks.MockPolicyStore,
	*mocks.MockEvaluator,
) {
	mockVector := mocks.NewMockVectorClient(ctrl)
	mockCtxStore := mocks.NewMockContextStore(ctrl)
	mockSessStore := mocks.NewMockSessionStore(ctrl)
	mockPolicyStore := mocks.NewMockPolicyStore(ctrl)
	mockEvaluator := mocks.NewMockEvaluator(ctrl)
	cfg := newTestConfig()
	eng := NewEngine(mockVector, mockCtxStore, mockSessStore, mockPolicyStore, mockEvaluator, cfg)
	return eng, mockVector, mockCtxStore, mockSessStore, mockPolicyStore, mockEvaluator
}

// makeChallengeContext はChallenge応答テスト用のEAPContextを生成する
func makeChallengeContext(eapType uint8, kAut, xres, msk []byte) *session.EAPContext {
	return &session.EAPContext{
		IMSI:    testIMSI,
		Stage:   string(eap.StateChallengeSent),
		EAPType: eapType,
		RAND:    hex.EncodeToString(testRAND),
		AUTN:    hex.EncodeToString(testAUTN),
		XRES:    hex.EncodeToString(xres),
		Kaut:    hex.EncodeToString(kAut),
		MSK:     hex.EncodeToString(msk),
	}
}

func TestEngine_ChallengeSuccess_Accept(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, mockCtxStore, mockSessStore, mockPolicyStore, mockEvaluator := newChallengeTestEngine(ctrl)

	// AKA用鍵導出
	keys := eapaka.DeriveKeysAKA("0"+testIMSI+"@realm", testCK, testIK)
	eapCtx := makeChallengeContext(eapaka.TypeAKA, keys.K_aut, testXRES, keys.MSK)

	// Challenge応答メッセージ構築
	challengeResp := buildChallengeResponseEAPMessage(2, eapaka.TypeAKA, keys.K_aut, testXRES)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)
	mockPolicyStore.EXPECT().GetPolicy(gomock.Any(), testIMSI).
		Return(&policy.Policy{Default: "allow", Rules: []policy.PolicyRule{
			{NasID: testNASID, AllowedSSIDs: []string{"*"}, VlanID: "100", SessionTimeout: 3600},
		}}, nil)
	mockEvaluator.EXPECT().Evaluate(gomock.Any(), testNASID, testSSID).
		Return(&policy.EvaluationResult{
			Allowed:     true,
			MatchedRule: &policy.PolicyRule{VlanID: "100", SessionTimeout: 3600},
		})
	mockSessStore.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	mockSessStore.EXPECT().AddUserIndex(gomock.Any(), testIMSI, gomock.Any()).Return(nil)
	mockCtxStore.EXPECT().Delete(gomock.Any(), testTraceID).Return(nil)

	req := &eap.Request{
		TraceID:       testTraceID,
		SrcIP:         "192.168.1.1",
		NASIdentifier: testNASID,
		CalledStation: "AA-BB-CC-DD-EE-FF:" + testSSID,
		UserName:      "0" + testIMSI + "@realm",
		State:         []byte(testTraceID),
		EAPMessage:    challengeResp,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionAccept {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionAccept)
	}
	if result.VlanID != "100" {
		t.Errorf("VlanID: got %q, want %q", result.VlanID, "100")
	}
	if result.SessionTimeout != 3600 {
		t.Errorf("SessionTimeout: got %d, want %d", result.SessionTimeout, 3600)
	}
	if len(result.MSK) == 0 {
		t.Error("MSKが空")
	}
	if result.SessionID == "" {
		t.Error("SessionIDが空")
	}
}

func TestEngine_MACInvalid_Reject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	keys := eapaka.DeriveKeysAKA("0"+testIMSI+"@realm", testCK, testIK)
	eapCtx := makeChallengeContext(eapaka.TypeAKA, keys.K_aut, testXRES, keys.MSK)

	// 不正なK_autでChallenge応答を構築 → MAC検証失敗
	wrongKAut := make([]byte, 16)
	challengeResp := buildChallengeResponseEAPMessage(2, eapaka.TypeAKA, wrongKAut, testXRES)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)
	mockCtxStore.EXPECT().Delete(gomock.Any(), testTraceID).Return(nil)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		State:      []byte(testTraceID),
		EAPMessage: challengeResp,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

func TestEngine_RESMismatch_Reject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	keys := eapaka.DeriveKeysAKA("0"+testIMSI+"@realm", testCK, testIK)

	// 正しいXRESとは異なる値をContextに保存
	wrongXRES := make([]byte, 8)
	for i := range wrongXRES {
		wrongXRES[i] = 0xFF
	}
	eapCtx := makeChallengeContext(eapaka.TypeAKA, keys.K_aut, wrongXRES, keys.MSK)

	// 正しいK_autでMAC検証は通るが、XRESが不一致
	challengeResp := buildChallengeResponseEAPMessage(2, eapaka.TypeAKA, keys.K_aut, testXRES)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)
	mockCtxStore.EXPECT().Delete(gomock.Any(), testTraceID).Return(nil)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		State:      []byte(testTraceID),
		EAPMessage: challengeResp,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

func TestEngine_PolicyNotFound_Reject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, mockCtxStore, _, mockPolicyStore, _ := newChallengeTestEngine(ctrl)

	keys := eapaka.DeriveKeysAKA("0"+testIMSI+"@realm", testCK, testIK)
	eapCtx := makeChallengeContext(eapaka.TypeAKA, keys.K_aut, testXRES, keys.MSK)
	challengeResp := buildChallengeResponseEAPMessage(2, eapaka.TypeAKA, keys.K_aut, testXRES)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)
	mockPolicyStore.EXPECT().GetPolicy(gomock.Any(), testIMSI).
		Return(nil, policy.ErrPolicyNotFound)
	mockCtxStore.EXPECT().Delete(gomock.Any(), testTraceID).Return(nil)

	req := &eap.Request{
		TraceID:       testTraceID,
		NASIdentifier: testNASID,
		CalledStation: "AA-BB-CC-DD-EE-FF:" + testSSID,
		UserName:      "0" + testIMSI + "@realm",
		State:         []byte(testTraceID),
		EAPMessage:    challengeResp,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

func TestEngine_PolicyDenied_Reject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, mockCtxStore, _, mockPolicyStore, mockEvaluator := newChallengeTestEngine(ctrl)

	keys := eapaka.DeriveKeysAKA("0"+testIMSI+"@realm", testCK, testIK)
	eapCtx := makeChallengeContext(eapaka.TypeAKA, keys.K_aut, testXRES, keys.MSK)
	challengeResp := buildChallengeResponseEAPMessage(2, eapaka.TypeAKA, keys.K_aut, testXRES)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)
	mockPolicyStore.EXPECT().GetPolicy(gomock.Any(), testIMSI).
		Return(&policy.Policy{Default: "deny"}, nil)
	mockEvaluator.EXPECT().Evaluate(gomock.Any(), testNASID, testSSID).
		Return(&policy.EvaluationResult{Allowed: false, DenyReason: "no matching rule"})
	mockCtxStore.EXPECT().Delete(gomock.Any(), testTraceID).Return(nil)

	req := &eap.Request{
		TraceID:       testTraceID,
		NASIdentifier: testNASID,
		CalledStation: "AA-BB-CC-DD-EE-FF:" + testSSID,
		UserName:      "0" + testIMSI + "@realm",
		State:         []byte(testTraceID),
		EAPMessage:    challengeResp,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

func TestEngine_ContextNotFound_Reject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).
		Return(nil, session.ErrContextNotFound)

	challengeResp := buildChallengeResponseEAPMessage(2, eapaka.TypeAKA, testCK, testXRES)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		State:      []byte(testTraceID),
		EAPMessage: challengeResp,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

// --- Resync テスト ---

func TestEngine_SyncFailure_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, mockVector, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	keys := eapaka.DeriveKeysAKA("0"+testIMSI+"@realm", testCK, testIK)
	eapCtx := makeChallengeContext(eapaka.TypeAKA, keys.K_aut, testXRES, keys.MSK)
	eapCtx.ResyncCount = 0

	auts := make([]byte, 14) // AT_AUTS: 14バイト
	syncMsg := buildSyncFailureEAPMessage(2, eapaka.TypeAKA, auts)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)
	mockVector.EXPECT().GetVector(gomock.Any(), gomock.Any()).
		Return(&vector.VectorResponse{
			RAND: testRAND, AUTN: testAUTN, XRES: testXRES, CK: testCK, IK: testIK,
		}, nil)
	mockCtxStore.EXPECT().Update(gomock.Any(), testTraceID, gomock.Any()).Return(nil)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		State:      []byte(testTraceID),
		EAPMessage: syncMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionChallenge {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionChallenge)
	}
}

func TestEngine_SyncFailure_LimitExceeded(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	keys := eapaka.DeriveKeysAKA("0"+testIMSI+"@realm", testCK, testIK)
	eapCtx := makeChallengeContext(eapaka.TypeAKA, keys.K_aut, testXRES, keys.MSK)
	eapCtx.ResyncCount = 32 // 上限に達している

	auts := make([]byte, 14)
	syncMsg := buildSyncFailureEAPMessage(2, eapaka.TypeAKA, auts)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)
	mockCtxStore.EXPECT().Delete(gomock.Any(), testTraceID).Return(nil)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		State:      []byte(testTraceID),
		EAPMessage: syncMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

// --- Other テスト ---

func TestEngine_AuthReject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	eapCtx := &session.EAPContext{
		IMSI:  testIMSI,
		Stage: string(eap.StateChallengeSent),
	}

	authRejectMsg := buildAuthRejectEAPMessage(2, eapaka.TypeAKA)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)
	mockCtxStore.EXPECT().Delete(gomock.Any(), testTraceID).Return(nil)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		State:      []byte(testTraceID),
		EAPMessage: authRejectMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

func TestEngine_ClientError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	eapCtx := &session.EAPContext{
		IMSI:  testIMSI,
		Stage: string(eap.StateChallengeSent),
	}

	clientErrMsg := buildClientErrorEAPMessage(2, eapaka.TypeAKA)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)
	mockCtxStore.EXPECT().Delete(gomock.Any(), testTraceID).Return(nil)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		State:      []byte(testTraceID),
		EAPMessage: clientErrMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

func TestEngine_UnknownSubtype(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	eapCtx := &session.EAPContext{
		IMSI:  testIMSI,
		Stage: string(eap.StateChallengeSent),
	}

	notifMsg := buildNotificationEAPMessage(2, eapaka.TypeAKA)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		State:      []byte(testTraceID),
		EAPMessage: notifMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

func TestEngine_EAPParseFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, _, _, _, _ := newChallengeTestEngine(ctrl)

	// 不正なEAPメッセージ（空バイト列）
	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		EAPMessage: []byte{},
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

// --- IdentityResponse テスト ---

func TestEngine_IdentityResponse_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, mockVector, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	// WAITING_IDENTITY状態のEAPContext
	eapCtx := &session.EAPContext{
		Stage:                string(eap.StateWaitingIdentity),
		PermanentIDRequested: true,
		EAPType:              eapaka.TypeAKA,
	}

	identityMsg := buildIdentityEAPMessage(2, eapaka.TypeAKA)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)
	mockCtxStore.EXPECT().Update(gomock.Any(), testTraceID, gomock.Any()).Return(nil)
	mockVector.EXPECT().GetVector(gomock.Any(), &vector.VectorRequest{IMSI: testIMSI}).
		Return(&vector.VectorResponse{
			RAND: testRAND, AUTN: testAUTN, XRES: testXRES, CK: testCK, IK: testIK,
		}, nil)
	mockCtxStore.EXPECT().Update(gomock.Any(), testTraceID, gomock.Any()).Return(nil)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		State:      []byte(testTraceID),
		EAPMessage: identityMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionChallenge {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionChallenge)
	}
}

func TestEngine_IdentityResponse_NonPermanent_Reject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	eapCtx := &session.EAPContext{
		Stage:                string(eap.StateWaitingIdentity),
		PermanentIDRequested: true,
		EAPType:              eapaka.TypeAKA,
	}

	identityMsg := buildIdentityEAPMessage(2, eapaka.TypeAKA)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)
	mockCtxStore.EXPECT().Delete(gomock.Any(), testTraceID).Return(nil)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "2pseudonym@realm", // 仮名ID → 非永続
		State:      []byte(testTraceID),
		EAPMessage: identityMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

func TestEngine_IdentityResponse_InvalidIdentity_Reject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	eapCtx := &session.EAPContext{
		Stage:                string(eap.StateWaitingIdentity),
		PermanentIDRequested: true,
		EAPType:              eapaka.TypeAKA,
	}

	identityMsg := buildIdentityEAPMessage(2, eapaka.TypeAKA)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)
	mockCtxStore.EXPECT().Delete(gomock.Any(), testTraceID).Return(nil)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "Xinvalid@realm", // 不正なIdentity
		State:      []byte(testTraceID),
		EAPMessage: identityMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

// --- FullAuthRedirect追加テスト ---

func TestEngine_FullAuthRedirect_CtxCreateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	eapMsg := buildIdentityEAPMessage(1, eapaka.TypeAKA)

	mockCtxStore.EXPECT().Create(gomock.Any(), testTraceID, gomock.Any()).
		Return(errors.New("valkey error"))

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "2pseudonym@realm",
		EAPMessage: eapMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

// --- logVectorError追加テスト ---

func TestEngine_CircuitBreakerOpen_Reject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, mockVector, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	eapMsg := buildIdentityEAPMessage(1, eapaka.TypeAKA)

	mockCtxStore.EXPECT().Create(gomock.Any(), testTraceID, gomock.Any()).Return(nil)
	mockVector.EXPECT().GetVector(gomock.Any(), gomock.Any()).
		Return(nil, vector.ErrCircuitOpen)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		EAPMessage: eapMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

func TestEngine_VectorUnknownError_Reject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, mockVector, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	eapMsg := buildIdentityEAPMessage(1, eapaka.TypeAKA)

	mockCtxStore.EXPECT().Create(gomock.Any(), testTraceID, gomock.Any()).Return(nil)
	mockVector.EXPECT().GetVector(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("unknown error"))

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		EAPMessage: eapMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

// --- NotIdentity テスト ---

func TestEngine_Identity_NotIdentitySubtype(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, _, _, _, _ := newChallengeTestEngine(ctrl)

	// 初回リクエストでSubtype=Challenge（Identityではない）
	challengeMsg := buildChallengeResponseEAPMessage(1, eapaka.TypeAKA, testCK, testXRES)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		EAPMessage: challengeMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionDrop {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionDrop)
	}
}

// --- Identity受信が不正なタイミング テスト ---

func TestEngine_Identity_UnexpectedStage_Reject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	// ChallengeSent状態でIdentityを受信（不正）
	eapCtx := &session.EAPContext{
		IMSI:  testIMSI,
		Stage: string(eap.StateChallengeSent),
	}

	identityMsg := buildIdentityEAPMessage(2, eapaka.TypeAKA)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		State:      []byte(testTraceID),
		EAPMessage: identityMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

// --- Resync追加テスト ---

func TestEngine_Resync_VectorError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, mockVector, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	keys := eapaka.DeriveKeysAKA("0"+testIMSI+"@realm", testCK, testIK)
	eapCtx := makeChallengeContext(eapaka.TypeAKA, keys.K_aut, testXRES, keys.MSK)
	eapCtx.ResyncCount = 0

	auts := make([]byte, 14)
	syncMsg := buildSyncFailureEAPMessage(2, eapaka.TypeAKA, auts)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)
	mockVector.EXPECT().GetVector(gomock.Any(), gomock.Any()).
		Return(nil, &vector.APIError{StatusCode: 500, Message: "internal error"})
	mockCtxStore.EXPECT().Delete(gomock.Any(), testTraceID).Return(nil)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		State:      []byte(testTraceID),
		EAPMessage: syncMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

func TestEngine_Resync_WrongStage_Reject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	// WAITING_IDENTITY状態でSyncFailure受信（不正）
	eapCtx := &session.EAPContext{
		IMSI:  testIMSI,
		Stage: string(eap.StateWaitingIdentity),
	}

	auts := make([]byte, 14)
	syncMsg := buildSyncFailureEAPMessage(2, eapaka.TypeAKA, auts)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		State:      []byte(testTraceID),
		EAPMessage: syncMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

// --- PermanentIdentity追加テスト ---

func TestEngine_PermanentIdentity_CtxCreateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	eapMsg := buildIdentityEAPMessage(1, eapaka.TypeAKA)

	mockCtxStore.EXPECT().Create(gomock.Any(), testTraceID, gomock.Any()).
		Return(errors.New("valkey error"))

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		EAPMessage: eapMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

// --- Challenge WrongStage テスト ---

func TestEngine_ChallengeResponse_WrongStage_Reject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	// WAITING_IDENTITY状態でChallenge受信（不正）
	eapCtx := &session.EAPContext{
		IMSI:  testIMSI,
		Stage: string(eap.StateWaitingIdentity),
	}

	challengeResp := buildChallengeResponseEAPMessage(2, eapaka.TypeAKA, testCK, testXRES)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		State:      []byte(testTraceID),
		EAPMessage: challengeResp,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

// --- 新規ヘルパー関数 ---

// buildRFC3748IdentityMessage はRFC 3748 EAP-Response/Identity (Type=1) を構築する
func buildRFC3748IdentityMessage(identifier uint8, identity string) []byte {
	idBytes := []byte(identity)
	length := uint16(5 + len(idBytes))
	buf := make([]byte, length)
	buf[0] = 0x02 // Response
	buf[1] = identifier
	buf[2] = byte(length >> 8)
	buf[3] = byte(length)
	buf[4] = 0x01 // Type=Identity
	copy(buf[5:], idBytes)
	return buf
}

// buildSyncFailureNoAUTSMessage はAT_AUTSなしのSynchronizationFailureパケットを構築する
func buildSyncFailureNoAUTSMessage(identifier uint8, eapType uint8) []byte {
	pkt := &eapaka.Packet{
		Code:       eapaka.CodeResponse,
		Identifier: identifier,
		Type:       eapType,
		Subtype:    eapaka.SubtypeSynchronizationFailure,
	}
	data, _ := pkt.Marshal()
	return data
}

// buildChallengeResponseNoRES はAT_RESなし（AT_MACのみ）のChallenge応答を構築する
func buildChallengeResponseNoRES(identifier uint8, eapType uint8, kAut []byte) []byte {
	pkt := &eapaka.Packet{
		Code:       eapaka.CodeResponse,
		Identifier: identifier,
		Type:       eapType,
		Subtype:    eapaka.SubtypeChallenge,
		Attributes: []eapaka.Attribute{
			&eapaka.AtMac{MAC: make([]byte, 16)},
		},
	}
	_ = pkt.CalculateAndSetMac(kAut)
	data, _ := pkt.Marshal()
	return data
}

// buildAKAPrimeChallengeResponseEAPMessage はEAP-Response/AKA'-Challengeパケットを構築する
func buildAKAPrimeChallengeResponseEAPMessage(identifier uint8, kAut, xres []byte) []byte {
	pkt := &eapaka.Packet{
		Code:       eapaka.CodeResponse,
		Identifier: identifier,
		Type:       eapaka.TypeAKAPrime,
		Subtype:    eapaka.SubtypeChallenge,
		Attributes: []eapaka.Attribute{
			&eapaka.AtRes{Res: xres},
			&eapaka.AtKdf{KDF: eapaka.KDFAKAPrimeWithCKIK},
			&eapaka.AtMac{MAC: make([]byte, 16)},
		},
	}
	_ = pkt.CalculateAndSetMac(kAut)
	data, _ := pkt.Marshal()
	return data
}

// --- A. handleIdentity系 追加テスト ---

// TestEngine_RFC3748Identity_PermanentAKA_Success はRFC 3748 EAP-Response/Identity (Type=1) で永続ID送信時のテスト
func TestEngine_RFC3748Identity_PermanentAKA_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, mockVector, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	// RFC 3748 Identity (Type=1) メッセージ
	eapMsg := buildRFC3748IdentityMessage(1, "0"+testIMSI+"@realm")

	mockCtxStore.EXPECT().Create(gomock.Any(), testTraceID, gomock.Any()).Return(nil)
	mockVector.EXPECT().GetVector(gomock.Any(), &vector.VectorRequest{IMSI: testIMSI}).
		Return(&vector.VectorResponse{
			RAND: testRAND, AUTN: testAUTN, XRES: testXRES, CK: testCK, IK: testIK,
		}, nil)
	mockCtxStore.EXPECT().Update(gomock.Any(), testTraceID, gomock.Any()).Return(nil)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		EAPMessage: eapMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionChallenge {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionChallenge)
	}
	if string(result.State) != testTraceID {
		t.Errorf("State: got %q, want %q", string(result.State), testTraceID)
	}
}

// --- B. handleFullAuthRedirect系 追加テスト ---

// TestEngine_FullAuthRedirect_UpdateError はctxStore.Update失敗時のテスト
func TestEngine_FullAuthRedirect_UpdateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	eapMsg := buildIdentityEAPMessage(1, eapaka.TypeAKA)

	mockCtxStore.EXPECT().Create(gomock.Any(), testTraceID, gomock.Any()).Return(nil)
	mockCtxStore.EXPECT().Update(gomock.Any(), testTraceID, gomock.Any()).
		Return(errors.New("valkey error"))

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "2pseudonym@realm",
		EAPMessage: eapMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

// --- C. requestVectorAndBuildChallenge系 追加テスト ---

// TestEngine_RequestVector_CtxUpdateError はVector成功後のctxStore.Update失敗テスト
func TestEngine_RequestVector_CtxUpdateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, mockVector, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	eapMsg := buildIdentityEAPMessage(1, eapaka.TypeAKA)

	mockCtxStore.EXPECT().Create(gomock.Any(), testTraceID, gomock.Any()).Return(nil)
	mockVector.EXPECT().GetVector(gomock.Any(), &vector.VectorRequest{IMSI: testIMSI}).
		Return(&vector.VectorResponse{
			RAND: testRAND, AUTN: testAUTN, XRES: testXRES, CK: testCK, IK: testIK,
		}, nil)
	mockCtxStore.EXPECT().Update(gomock.Any(), testTraceID, gomock.Any()).
		Return(errors.New("valkey error"))

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		EAPMessage: eapMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

// --- D. handleSubsequent系 追加テスト ---

// TestEngine_Subsequent_ParseError_Reject はState有り時のEAPパース失敗テスト
func TestEngine_Subsequent_ParseError_Reject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	eapCtx := &session.EAPContext{
		IMSI:  testIMSI,
		Stage: string(eap.StateChallengeSent),
	}

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)

	// 不正なEAPメッセージ（短すぎる）
	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		State:      []byte(testTraceID),
		EAPMessage: []byte{0x02, 0x01},
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

// --- E. handleIdentityResponse系 追加テスト ---

// TestEngine_IdentityResponse_UnsupportedIdentity_Reject はSIM Identity（非対応）のテスト
func TestEngine_IdentityResponse_UnsupportedIdentity_Reject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	eapCtx := &session.EAPContext{
		Stage:                string(eap.StateWaitingIdentity),
		PermanentIDRequested: true,
		EAPType:              eapaka.TypeAKA,
	}

	identityMsg := buildIdentityEAPMessage(2, eapaka.TypeAKA)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)
	mockCtxStore.EXPECT().Delete(gomock.Any(), testTraceID).Return(nil)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "1" + testIMSI + "@realm", // SIM Identity (prefix '1')
		State:      []byte(testTraceID),
		EAPMessage: identityMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

// TestEngine_IdentityResponse_CtxUpdateError_Reject はctxStore.Update失敗時のテスト
func TestEngine_IdentityResponse_CtxUpdateError_Reject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	eapCtx := &session.EAPContext{
		Stage:                string(eap.StateWaitingIdentity),
		PermanentIDRequested: true,
		EAPType:              eapaka.TypeAKA,
	}

	identityMsg := buildIdentityEAPMessage(2, eapaka.TypeAKA)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)
	mockCtxStore.EXPECT().Update(gomock.Any(), testTraceID, gomock.Any()).
		Return(errors.New("valkey error"))

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		State:      []byte(testTraceID),
		EAPMessage: identityMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

// --- F. handleChallengeResponse系 追加テスト ---

// TestEngine_ChallengeResponse_KautDecodeError_Reject はKaut不正hex時のテスト
func TestEngine_ChallengeResponse_KautDecodeError_Reject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	eapCtx := &session.EAPContext{
		IMSI:    testIMSI,
		Stage:   string(eap.StateChallengeSent),
		EAPType: eapaka.TypeAKA,
		Kaut:    "ZZZZ", // 不正なhex
		XRES:    hex.EncodeToString(testXRES),
		MSK:     hex.EncodeToString(make([]byte, 64)),
	}

	challengeResp := buildChallengeResponseEAPMessage(2, eapaka.TypeAKA, testCK, testXRES)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		State:      []byte(testTraceID),
		EAPMessage: challengeResp,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

// TestEngine_ChallengeResponse_XRESDecodeError_Reject はXRES不正hex時のテスト
func TestEngine_ChallengeResponse_XRESDecodeError_Reject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	keys := eapaka.DeriveKeysAKA("0"+testIMSI+"@realm", testCK, testIK)
	eapCtx := &session.EAPContext{
		IMSI:    testIMSI,
		Stage:   string(eap.StateChallengeSent),
		EAPType: eapaka.TypeAKA,
		Kaut:    hex.EncodeToString(keys.K_aut),
		XRES:    "ZZZZ", // 不正なhex
		MSK:     hex.EncodeToString(keys.MSK),
	}

	challengeResp := buildChallengeResponseEAPMessage(2, eapaka.TypeAKA, keys.K_aut, testXRES)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		State:      []byte(testTraceID),
		EAPMessage: challengeResp,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

// TestEngine_ChallengeResponse_MSKDecodeError_Reject はMSK不正hex時のテスト
func TestEngine_ChallengeResponse_MSKDecodeError_Reject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	keys := eapaka.DeriveKeysAKA("0"+testIMSI+"@realm", testCK, testIK)
	eapCtx := &session.EAPContext{
		IMSI:    testIMSI,
		Stage:   string(eap.StateChallengeSent),
		EAPType: eapaka.TypeAKA,
		Kaut:    hex.EncodeToString(keys.K_aut),
		XRES:    hex.EncodeToString(testXRES),
		MSK:     "ZZZZ", // 不正なhex
	}

	challengeResp := buildChallengeResponseEAPMessage(2, eapaka.TypeAKA, keys.K_aut, testXRES)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		State:      []byte(testTraceID),
		EAPMessage: challengeResp,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

// TestEngine_ChallengeSuccess_AKAPrime_Accept はAKA' Challenge応答の成功フローテスト
func TestEngine_ChallengeSuccess_AKAPrime_Accept(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, mockCtxStore, mockSessStore, mockPolicyStore, mockEvaluator := newChallengeTestEngine(ctrl)

	// AKA'用鍵導出
	identity := "6" + testIMSI + "@realm"
	ckPrime, ikPrime, err := eapaka.DeriveCKPrimeIKPrime(testCK, testIK, testNetworkName, testAUTN)
	if err != nil {
		t.Fatalf("DeriveCKPrimeIKPrime失敗: %v", err)
	}
	keys := eapaka.DeriveKeysAKAPrime(identity, ckPrime, ikPrime)

	eapCtx := &session.EAPContext{
		IMSI:    testIMSI,
		Stage:   string(eap.StateChallengeSent),
		EAPType: eapaka.TypeAKAPrime,
		RAND:    hex.EncodeToString(testRAND),
		AUTN:    hex.EncodeToString(testAUTN),
		XRES:    hex.EncodeToString(testXRES),
		Kaut:    hex.EncodeToString(keys.K_aut),
		MSK:     hex.EncodeToString(keys.MSK),
	}

	challengeResp := buildAKAPrimeChallengeResponseEAPMessage(2, keys.K_aut, testXRES)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)
	mockPolicyStore.EXPECT().GetPolicy(gomock.Any(), testIMSI).
		Return(&policy.Policy{Default: "allow", Rules: []policy.PolicyRule{
			{NasID: testNASID, AllowedSSIDs: []string{"*"}, VlanID: "200", SessionTimeout: 7200},
		}}, nil)
	mockEvaluator.EXPECT().Evaluate(gomock.Any(), testNASID, testSSID).
		Return(&policy.EvaluationResult{
			Allowed:     true,
			MatchedRule: &policy.PolicyRule{VlanID: "200", SessionTimeout: 7200},
		})
	mockSessStore.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	mockSessStore.EXPECT().AddUserIndex(gomock.Any(), testIMSI, gomock.Any()).Return(nil)
	mockCtxStore.EXPECT().Delete(gomock.Any(), testTraceID).Return(nil)

	req := &eap.Request{
		TraceID:       testTraceID,
		SrcIP:         "192.168.1.1",
		NASIdentifier: testNASID,
		CalledStation: "AA-BB-CC-DD-EE-FF:" + testSSID,
		UserName:      identity,
		State:         []byte(testTraceID),
		EAPMessage:    challengeResp,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionAccept {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionAccept)
	}
	if result.VlanID != "200" {
		t.Errorf("VlanID: got %q, want %q", result.VlanID, "200")
	}
	if len(result.MSK) == 0 {
		t.Error("MSKが空")
	}
}

// TestEngine_ChallengeResponse_OtherVerifyError_Reject はAT_RESなし時のAUTH_VERIFY_FAIL分岐テスト
func TestEngine_ChallengeResponse_OtherVerifyError_Reject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	keys := eapaka.DeriveKeysAKA("0"+testIMSI+"@realm", testCK, testIK)
	eapCtx := makeChallengeContext(eapaka.TypeAKA, keys.K_aut, testXRES, keys.MSK)

	// AT_RESなしのChallenge応答 → ErrRESNotFound → AUTH_VERIFY_FAIL分岐
	challengeResp := buildChallengeResponseNoRES(2, eapaka.TypeAKA, keys.K_aut)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)
	mockCtxStore.EXPECT().Delete(gomock.Any(), testTraceID).Return(nil)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		State:      []byte(testTraceID),
		EAPMessage: challengeResp,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

// TestEngine_ChallengeResponse_SessionCreateError_Reject はセッション作成失敗時のテスト
func TestEngine_ChallengeResponse_SessionCreateError_Reject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, mockCtxStore, mockSessStore, mockPolicyStore, mockEvaluator := newChallengeTestEngine(ctrl)

	keys := eapaka.DeriveKeysAKA("0"+testIMSI+"@realm", testCK, testIK)
	eapCtx := makeChallengeContext(eapaka.TypeAKA, keys.K_aut, testXRES, keys.MSK)
	challengeResp := buildChallengeResponseEAPMessage(2, eapaka.TypeAKA, keys.K_aut, testXRES)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)
	mockPolicyStore.EXPECT().GetPolicy(gomock.Any(), testIMSI).
		Return(&policy.Policy{Default: "allow"}, nil)
	mockEvaluator.EXPECT().Evaluate(gomock.Any(), testNASID, testSSID).
		Return(&policy.EvaluationResult{Allowed: true})
	mockSessStore.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("valkey error"))

	req := &eap.Request{
		TraceID:       testTraceID,
		SrcIP:         "192.168.1.1",
		NASIdentifier: testNASID,
		CalledStation: "AA-BB-CC-DD-EE-FF:" + testSSID,
		UserName:      "0" + testIMSI + "@realm",
		State:         []byte(testTraceID),
		EAPMessage:    challengeResp,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

// TestEngine_ChallengeResponse_AddUserIndexError_Continue はAddUserIndex失敗時にAccept継続するテスト
func TestEngine_ChallengeResponse_AddUserIndexError_Continue(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, mockCtxStore, mockSessStore, mockPolicyStore, mockEvaluator := newChallengeTestEngine(ctrl)

	keys := eapaka.DeriveKeysAKA("0"+testIMSI+"@realm", testCK, testIK)
	eapCtx := makeChallengeContext(eapaka.TypeAKA, keys.K_aut, testXRES, keys.MSK)
	challengeResp := buildChallengeResponseEAPMessage(2, eapaka.TypeAKA, keys.K_aut, testXRES)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)
	mockPolicyStore.EXPECT().GetPolicy(gomock.Any(), testIMSI).
		Return(&policy.Policy{Default: "allow"}, nil)
	mockEvaluator.EXPECT().Evaluate(gomock.Any(), testNASID, testSSID).
		Return(&policy.EvaluationResult{Allowed: true})
	mockSessStore.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	mockSessStore.EXPECT().AddUserIndex(gomock.Any(), testIMSI, gomock.Any()).
		Return(errors.New("index error")) // 非致命的エラー
	mockCtxStore.EXPECT().Delete(gomock.Any(), testTraceID).Return(nil)

	req := &eap.Request{
		TraceID:       testTraceID,
		SrcIP:         "192.168.1.1",
		NASIdentifier: testNASID,
		CalledStation: "AA-BB-CC-DD-EE-FF:" + testSSID,
		UserName:      "0" + testIMSI + "@realm",
		State:         []byte(testTraceID),
		EAPMessage:    challengeResp,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionAccept {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionAccept)
	}
	if result.SessionID == "" {
		t.Error("SessionIDが空")
	}
}

// --- G. handleResync系 追加テスト ---

// TestEngine_Resync_AUTSNotFound_Reject はAT_AUTSなし時のテスト
func TestEngine_Resync_AUTSNotFound_Reject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, _, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	keys := eapaka.DeriveKeysAKA("0"+testIMSI+"@realm", testCK, testIK)
	eapCtx := makeChallengeContext(eapaka.TypeAKA, keys.K_aut, testXRES, keys.MSK)
	eapCtx.ResyncCount = 0

	// AT_AUTSなしのSyncFailureメッセージ
	syncMsg := buildSyncFailureNoAUTSMessage(2, eapaka.TypeAKA)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		State:      []byte(testTraceID),
		EAPMessage: syncMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}

// TestEngine_Resync_AKAPrime_Success はAKA'再同期成功フローのテスト
func TestEngine_Resync_AKAPrime_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, mockVector, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	// AKA'用鍵導出
	identity := "6" + testIMSI + "@realm"
	ckPrime, ikPrime, err := eapaka.DeriveCKPrimeIKPrime(testCK, testIK, testNetworkName, testAUTN)
	if err != nil {
		t.Fatalf("DeriveCKPrimeIKPrime失敗: %v", err)
	}
	keys := eapaka.DeriveKeysAKAPrime(identity, ckPrime, ikPrime)

	eapCtx := &session.EAPContext{
		IMSI:        testIMSI,
		Stage:       string(eap.StateChallengeSent),
		EAPType:     eapaka.TypeAKAPrime,
		RAND:        hex.EncodeToString(testRAND),
		AUTN:        hex.EncodeToString(testAUTN),
		XRES:        hex.EncodeToString(testXRES),
		Kaut:        hex.EncodeToString(keys.K_aut),
		MSK:         hex.EncodeToString(keys.MSK),
		ResyncCount: 0,
	}

	auts := make([]byte, 14)
	syncMsg := buildSyncFailureEAPMessage(2, eapaka.TypeAKAPrime, auts)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)
	mockVector.EXPECT().GetVector(gomock.Any(), gomock.Any()).
		Return(&vector.VectorResponse{
			RAND: testRAND, AUTN: testAUTN, XRES: testXRES, CK: testCK, IK: testIK,
		}, nil)
	mockCtxStore.EXPECT().Update(gomock.Any(), testTraceID, gomock.Any()).Return(nil)

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   identity,
		State:      []byte(testTraceID),
		EAPMessage: syncMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionChallenge {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionChallenge)
	}
	if string(result.State) != testTraceID {
		t.Errorf("State: got %q, want %q", string(result.State), testTraceID)
	}
}

// TestEngine_Resync_CtxUpdateError_Reject は再同期時のctxStore.Update失敗テスト
func TestEngine_Resync_CtxUpdateError_Reject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eng, mockVector, mockCtxStore, _, _, _ := newChallengeTestEngine(ctrl)

	keys := eapaka.DeriveKeysAKA("0"+testIMSI+"@realm", testCK, testIK)
	eapCtx := makeChallengeContext(eapaka.TypeAKA, keys.K_aut, testXRES, keys.MSK)
	eapCtx.ResyncCount = 0

	auts := make([]byte, 14)
	syncMsg := buildSyncFailureEAPMessage(2, eapaka.TypeAKA, auts)

	mockCtxStore.EXPECT().Get(gomock.Any(), testTraceID).Return(eapCtx, nil)
	mockVector.EXPECT().GetVector(gomock.Any(), gomock.Any()).
		Return(&vector.VectorResponse{
			RAND: testRAND, AUTN: testAUTN, XRES: testXRES, CK: testCK, IK: testIK,
		}, nil)
	mockCtxStore.EXPECT().Update(gomock.Any(), testTraceID, gomock.Any()).
		Return(errors.New("valkey error"))

	req := &eap.Request{
		TraceID:    testTraceID,
		UserName:   "0" + testIMSI + "@realm",
		State:      []byte(testTraceID),
		EAPMessage: syncMsg,
	}

	result, err := eng.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.Action != eap.ActionReject {
		t.Errorf("Action: got %v, want %v", result.Action, eap.ActionReject)
	}
}
