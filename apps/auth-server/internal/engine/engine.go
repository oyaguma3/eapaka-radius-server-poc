package engine

import (
	"context"
	"encoding/hex"
	"errors"
	"log/slog"
	"time"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/config"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/eap"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/eap/aka"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/eap/akaprime"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/logging"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/policy"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/session"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/vector"
	eapaka "github.com/oyaguma3/go-eapaka"
)

// EngineImpl はeap.EAPProcessorの実装
type EngineImpl struct {
	vectorClient vector.VectorClient
	ctxStore     session.ContextStore
	sessStore    session.SessionStore
	policyStore  policy.PolicyStore
	evaluator    policy.Evaluator
	cfg          *config.Config
}

// NewEngine は新しいEAPエンジンを生成する
func NewEngine(
	vc vector.VectorClient,
	cs session.ContextStore,
	ss session.SessionStore,
	ps policy.PolicyStore,
	ev policy.Evaluator,
	cfg *config.Config,
) *EngineImpl {
	return &EngineImpl{
		vectorClient: vc,
		ctxStore:     cs,
		sessStore:    ss,
		policyStore:  ps,
		evaluator:    ev,
		cfg:          cfg,
	}
}

// Process はEAP認証リクエストを処理する
func (e *EngineImpl) Process(ctx context.Context, req *eap.Request) (*eap.Result, error) {
	if len(req.State) == 0 {
		// State無し → 初回Identity処理
		return e.handleIdentity(ctx, req)
	}

	// State有り → TraceID復元、EAPContext取得
	traceID := string(req.State)
	eapCtx, err := e.ctxStore.Get(ctx, traceID)
	if err != nil {
		slog.Warn("EAPコンテキスト取得失敗",
			"event_id", "EAP_CTX_NOT_FOUND",
			"trace_id", traceID,
			"error", err,
		)
		return e.buildReject(0), nil
	}

	return e.handleSubsequent(ctx, req, traceID, eapCtx)
}

// handleIdentity は初回Identity受信を処理する
func (e *EngineImpl) handleIdentity(ctx context.Context, req *eap.Request) (*eap.Result, error) {
	var pkt *eapaka.Packet

	// EAPパケットのType判定（RFC 3748 Identity vs AKA/AKA'）
	eapType := eap.GetEAPType(req.EAPMessage)
	if eapType == eap.EAPTypeIdentity {
		// RFC 3748 EAP-Response/Identity (Type=1)
		// go-eapakaではパース不可のため、識別子のみ抽出して処理を継続
		identifier := eap.GetEAPIdentifier(req.EAPMessage)
		pkt = &eapaka.Packet{
			Code:       eapaka.CodeResponse,
			Identifier: identifier,
		}
		slog.Info("EAP-Response/Identity受信",
			"event_id", "EAP_IDENTITY_RECEIVED",
			"trace_id", req.TraceID,
		)
	} else {
		// EAP-AKA/AKA' (Type=23/50) パケットパース
		var err error
		pkt, err = eap.ParseEAPPacket(req.EAPMessage)
		if err != nil {
			slog.Warn("EAPパケットパース失敗",
				"event_id", "EAP_PARSE_ERR",
				"trace_id", req.TraceID,
				"error", err,
			)
			return e.buildReject(0), nil
		}

		// Subtype=Identity確認
		if pkt.Subtype != eapaka.SubtypeIdentity {
			slog.Warn("初回リクエストがIdentityではない",
				"event_id", "EAP_NOT_IDENTITY",
				"trace_id", req.TraceID,
				"subtype", pkt.Subtype,
			)
			return &eap.Result{Action: eap.ActionDrop}, nil
		}
	}

	// Identity解析
	identity, err := eap.ParseIdentity(req.UserName)
	if err != nil {
		if errors.Is(err, eap.ErrUnsupportedIdentity) {
			slog.Warn("非対応のIdentity種別",
				"event_id", "EAP_UNSUPPORTED_TYPE",
				"trace_id", req.TraceID,
				"user_name", req.UserName,
			)
		} else {
			slog.Warn("Identity解析失敗",
				"event_id", "EAP_IDENTITY_INVALID",
				"trace_id", req.TraceID,
				"user_name", req.UserName,
				"error", err,
			)
		}
		eapFailure, _ := eap.BuildEAPFailure(pkt.Identifier + 1)
		return &eap.Result{
			Action:     eap.ActionReject,
			EAPMessage: eapFailure,
		}, nil
	}

	// フル認証誘導（仮名/再認証ID）
	if identity.RequiresFullAuth() {
		return e.handleFullAuthRedirect(ctx, req, pkt, identity)
	}

	// 永続ID処理
	if identity.IsPermanent() {
		return e.handlePermanentIdentity(ctx, req, pkt, identity)
	}

	// ここには到達しないはず
	return &eap.Result{Action: eap.ActionDrop}, nil
}

// handleFullAuthRedirect はフル認証への誘導処理を行う
func (e *EngineImpl) handleFullAuthRedirect(ctx context.Context, req *eap.Request, pkt *eapaka.Packet, identity *eap.ParsedIdentity) (*eap.Result, error) {
	// EAPContext作成
	eapCtx := &session.EAPContext{
		Stage:                string(eap.StateNew),
		PermanentIDRequested: true,
		EAPType:              identity.EAPType,
	}
	if err := e.ctxStore.Create(ctx, req.TraceID, eapCtx); err != nil {
		slog.Error("EAPコンテキスト作成失敗",
			"event_id", "EAP_CTX_CREATE_ERR",
			"trace_id", req.TraceID,
			"error", err,
		)
		return e.buildReject(pkt.Identifier + 1), nil
	}

	// AKA-Identity Request構築
	identityReq, err := eap.BuildAKAIdentityRequest(pkt.Identifier+1, identity.EAPType)
	if err != nil {
		slog.Error("Identity Request構築失敗",
			"event_id", "EAP_BUILD_ERR",
			"trace_id", req.TraceID,
			"error", err,
		)
		return e.buildReject(pkt.Identifier + 1), nil
	}

	// Stage更新: WAITING_IDENTITY
	if err := e.ctxStore.Update(ctx, req.TraceID, map[string]any{
		"stage": string(eap.StateWaitingIdentity),
	}); err != nil {
		slog.Error("EAPコンテキスト更新失敗",
			"event_id", "EAP_CTX_UPDATE_ERR",
			"trace_id", req.TraceID,
			"error", err,
		)
		return e.buildReject(pkt.Identifier + 1), nil
	}

	return &eap.Result{
		Action:     eap.ActionChallenge,
		EAPMessage: identityReq,
		State:      []byte(req.TraceID),
	}, nil
}

// handlePermanentIdentity は永続ID受信時の処理を行う
func (e *EngineImpl) handlePermanentIdentity(ctx context.Context, req *eap.Request, pkt *eapaka.Packet, identity *eap.ParsedIdentity) (*eap.Result, error) {
	maskedIMSI := e.maskIMSI(identity.IMSI)

	// EAPContext作成
	eapCtx := &session.EAPContext{
		IMSI:    identity.IMSI,
		Stage:   string(eap.StateIdentityReceived),
		EAPType: identity.EAPType,
	}
	if err := e.ctxStore.Create(ctx, req.TraceID, eapCtx); err != nil {
		slog.Error("EAPコンテキスト作成失敗",
			"event_id", "EAP_CTX_CREATE_ERR",
			"trace_id", req.TraceID,
			"imsi", maskedIMSI,
			"error", err,
		)
		return e.buildReject(pkt.Identifier + 1), nil
	}

	// 状態遷移: IDENTITY_RECEIVED → WAITING_VECTOR
	_, err := eap.ValidateTransition(eap.StateIdentityReceived, eap.EventVectorRequest)
	if err != nil {
		slog.Error("状態遷移失敗",
			"event_id", "EAP_STATE_ERR",
			"trace_id", req.TraceID,
			"error", err,
		)
		return e.buildReject(pkt.Identifier + 1), nil
	}

	// Vector Gateway呼び出し + Challenge構築
	return e.requestVectorAndBuildChallenge(ctx, req, pkt.Identifier, identity, req.TraceID)
}

// requestVectorAndBuildChallenge はVector取得→鍵導出→Challenge構築を行う
func (e *EngineImpl) requestVectorAndBuildChallenge(
	ctx context.Context,
	req *eap.Request,
	identifier uint8,
	identity *eap.ParsedIdentity,
	traceID string,
) (*eap.Result, error) {
	maskedIMSI := e.maskIMSI(identity.IMSI)

	// Vector Gateway呼び出し
	vCtx := vector.WithTraceID(ctx, traceID)
	vecResp, err := e.vectorClient.GetVector(vCtx, &vector.VectorRequest{
		IMSI: identity.IMSI,
	})
	if err != nil {
		e.logVectorError(err, traceID, maskedIMSI)
		eapFailure, _ := eap.BuildEAPFailure(identifier + 1)
		return &eap.Result{
			Action:     eap.ActionReject,
			EAPMessage: eapFailure,
		}, nil
	}

	// 鍵導出
	var kAut, msk []byte
	if identity.IsAKAPrime() {
		keys, err := akaprime.DeriveAllKeys(identity.Raw, vecResp.CK, vecResp.IK, vecResp.AUTN, e.cfg.NetworkName)
		if err != nil {
			slog.Error("AKA'鍵導出失敗",
				"event_id", "EAP_KEY_DERIVE_ERR",
				"trace_id", traceID,
				"imsi", maskedIMSI,
				"error", err,
			)
			return e.buildReject(identifier + 1), nil
		}
		kAut = keys.K_aut
		msk = keys.MSK
	} else {
		keys := aka.DeriveKeys(identity.Raw, vecResp.CK, vecResp.IK)
		kAut = keys.K_aut
		msk = keys.MSK
	}

	// EAPContext更新
	updates := map[string]any{
		"stage":    string(eap.StateChallengeSent),
		"rand":     hex.EncodeToString(vecResp.RAND),
		"autn":     hex.EncodeToString(vecResp.AUTN),
		"xres":     hex.EncodeToString(vecResp.XRES),
		"k_aut":    hex.EncodeToString(kAut),
		"msk":      hex.EncodeToString(msk),
		"imsi":     identity.IMSI,
		"eap_type": identity.EAPType,
	}
	if err := e.ctxStore.Update(ctx, traceID, updates); err != nil {
		slog.Error("EAPコンテキスト更新失敗",
			"event_id", "EAP_CTX_UPDATE_ERR",
			"trace_id", traceID,
			"error", err,
		)
		return e.buildReject(identifier + 1), nil
	}

	// Challenge構築
	var challengeMsg []byte
	if identity.IsAKAPrime() {
		challengeMsg, err = akaprime.BuildChallenge(identifier+1, vecResp.RAND, vecResp.AUTN, e.cfg.NetworkName, kAut)
	} else {
		challengeMsg, err = aka.BuildChallenge(identifier+1, vecResp.RAND, vecResp.AUTN, kAut)
	}
	if err != nil {
		slog.Error("Challenge構築失敗",
			"event_id", "EAP_BUILD_ERR",
			"trace_id", traceID,
			"error", err,
		)
		return e.buildReject(identifier + 1), nil
	}

	slog.Info("Challenge送信",
		"event_id", "EAP_CHALLENGE_SENT",
		"trace_id", traceID,
		"imsi", maskedIMSI,
		"eap_type", identity.EAPType,
	)

	return &eap.Result{
		Action:     eap.ActionChallenge,
		EAPMessage: challengeMsg,
		State:      []byte(traceID),
		IMSI:       identity.IMSI,
	}, nil
}

// handleSubsequent はState有りの後続リクエストを処理する
func (e *EngineImpl) handleSubsequent(ctx context.Context, req *eap.Request, traceID string, eapCtx *session.EAPContext) (*eap.Result, error) {
	// EAPパケットパース
	pkt, err := eap.ParseEAPPacket(req.EAPMessage)
	if err != nil {
		slog.Warn("EAPパケットパース失敗",
			"event_id", "EAP_PARSE_ERR",
			"trace_id", traceID,
			"error", err,
		)
		return e.buildReject(0), nil
	}

	// Subtype分岐
	switch pkt.Subtype {
	case eapaka.SubtypeChallenge:
		return e.handleChallengeResponse(ctx, req, traceID, eapCtx, pkt)

	case eapaka.SubtypeSynchronizationFailure:
		return e.handleResync(ctx, req, traceID, eapCtx, pkt)

	case eapaka.SubtypeAuthenticationReject:
		slog.Warn("Authentication-Reject受信",
			"event_id", "EAP_AUTH_REJECT",
			"trace_id", traceID,
			"imsi", e.maskIMSI(eapCtx.IMSI),
		)
		_ = e.ctxStore.Delete(ctx, traceID)
		return e.buildReject(pkt.Identifier + 1), nil

	case eapaka.SubtypeClientError:
		slog.Warn("Client-Error受信",
			"event_id", "EAP_CLIENT_ERROR",
			"trace_id", traceID,
			"imsi", e.maskIMSI(eapCtx.IMSI),
		)
		_ = e.ctxStore.Delete(ctx, traceID)
		return e.buildReject(pkt.Identifier + 1), nil

	case eapaka.SubtypeIdentity:
		// WAITING_IDENTITY状態の場合のみ受け入れ
		if eap.EAPState(eapCtx.Stage) == eap.StateWaitingIdentity {
			return e.handleIdentityResponse(ctx, req, traceID, eapCtx, pkt)
		}
		slog.Warn("不正なタイミングでIdentity受信",
			"event_id", "EAP_UNEXPECTED_IDENTITY",
			"trace_id", traceID,
			"stage", eapCtx.Stage,
		)
		return e.buildReject(pkt.Identifier + 1), nil

	default:
		slog.Warn("未知のSubtype",
			"event_id", "EAP_UNKNOWN_SUBTYPE",
			"trace_id", traceID,
			"subtype", pkt.Subtype,
		)
		return e.buildReject(pkt.Identifier + 1), nil
	}
}

// handleIdentityResponse はWAITING_IDENTITY状態でIdentity応答を処理する
func (e *EngineImpl) handleIdentityResponse(ctx context.Context, req *eap.Request, traceID string, eapCtx *session.EAPContext, pkt *eapaka.Packet) (*eap.Result, error) {
	identity, err := eap.ParseIdentity(req.UserName)
	if err != nil {
		if errors.Is(err, eap.ErrUnsupportedIdentity) {
			slog.Warn("非対応のIdentity種別",
				"event_id", "EAP_UNSUPPORTED_TYPE",
				"trace_id", traceID,
				"user_name", req.UserName,
			)
		} else {
			slog.Warn("Identity解析失敗",
				"event_id", "EAP_IDENTITY_INVALID",
				"trace_id", traceID,
				"user_name", req.UserName,
				"error", err,
			)
		}
		_ = e.ctxStore.Delete(ctx, traceID)
		eapFailure, _ := eap.BuildEAPFailure(pkt.Identifier + 1)
		return &eap.Result{
			Action:     eap.ActionReject,
			EAPMessage: eapFailure,
		}, nil
	}

	if !identity.IsPermanent() {
		slog.Warn("永続ID応答が仮名/再認証ID",
			"event_id", "EAP_IDENTITY_INVALID",
			"trace_id", traceID,
			"user_name", req.UserName,
		)
		_ = e.ctxStore.Delete(ctx, traceID)
		eapFailure, _ := eap.BuildEAPFailure(pkt.Identifier + 1)
		return &eap.Result{
			Action:     eap.ActionReject,
			EAPMessage: eapFailure,
		}, nil
	}

	// 状態遷移: WAITING_IDENTITY → IDENTITY_RECEIVED
	_, err = eap.ValidateTransition(eap.StateWaitingIdentity, eap.EventPermanentIdentity)
	if err != nil {
		slog.Error("状態遷移失敗",
			"event_id", "EAP_STATE_ERR",
			"trace_id", traceID,
			"error", err,
		)
		return e.buildReject(pkt.Identifier + 1), nil
	}

	// Context更新
	if err := e.ctxStore.Update(ctx, traceID, map[string]any{
		"imsi":     identity.IMSI,
		"stage":    string(eap.StateIdentityReceived),
		"eap_type": identity.EAPType,
	}); err != nil {
		slog.Error("EAPコンテキスト更新失敗",
			"event_id", "EAP_CTX_UPDATE_ERR",
			"trace_id", traceID,
			"error", err,
		)
		return e.buildReject(pkt.Identifier + 1), nil
	}

	// Vector取得 + Challenge構築
	return e.requestVectorAndBuildChallenge(ctx, req, pkt.Identifier, identity, traceID)
}

// handleChallengeResponse はChallenge応答を検証して認証結果を返す
func (e *EngineImpl) handleChallengeResponse(ctx context.Context, req *eap.Request, traceID string, eapCtx *session.EAPContext, pkt *eapaka.Packet) (*eap.Result, error) {
	maskedIMSI := e.maskIMSI(eapCtx.IMSI)

	// 状態遷移検証
	if eap.EAPState(eapCtx.Stage) != eap.StateChallengeSent {
		slog.Warn("不正な状態でChallenge応答受信",
			"event_id", "EAP_STATE_ERR",
			"trace_id", traceID,
			"stage", eapCtx.Stage,
		)
		return e.buildReject(pkt.Identifier + 1), nil
	}

	// Valkey保存値の復元
	kAut, err := hex.DecodeString(eapCtx.Kaut)
	if err != nil {
		slog.Error("K_aut復元失敗",
			"event_id", "EAP_CTX_DECODE_ERR",
			"trace_id", traceID,
			"error", err,
		)
		return e.buildReject(pkt.Identifier + 1), nil
	}
	xres, err := hex.DecodeString(eapCtx.XRES)
	if err != nil {
		slog.Error("XRES復元失敗",
			"event_id", "EAP_CTX_DECODE_ERR",
			"trace_id", traceID,
			"error", err,
		)
		return e.buildReject(pkt.Identifier + 1), nil
	}
	mskBytes, err := hex.DecodeString(eapCtx.MSK)
	if err != nil {
		slog.Error("MSK復元失敗",
			"event_id", "EAP_CTX_DECODE_ERR",
			"trace_id", traceID,
			"error", err,
		)
		return e.buildReject(pkt.Identifier + 1), nil
	}

	// Challenge応答検証（AKA/AKA'分岐）
	var verifyErr error
	if eapCtx.EAPType == eapaka.TypeAKAPrime {
		verifyErr = akaprime.VerifyChallengeResponse(pkt, kAut, xres)
	} else {
		verifyErr = aka.VerifyChallengeResponse(pkt, kAut, xres)
	}

	if verifyErr != nil {
		var eventID string
		if errors.Is(verifyErr, eap.ErrMACInvalid) {
			eventID = "AUTH_MAC_INVALID"
		} else if errors.Is(verifyErr, eap.ErrRESMismatch) {
			eventID = "AUTH_RES_MISMATCH"
		} else {
			eventID = "AUTH_VERIFY_FAIL"
		}
		slog.Warn("Challenge応答検証失敗",
			"event_id", eventID,
			"trace_id", traceID,
			"imsi", maskedIMSI,
			"error", verifyErr,
		)
		_ = e.ctxStore.Delete(ctx, traceID)
		eapFailure, _ := eap.BuildEAPFailure(pkt.Identifier + 1)
		return &eap.Result{
			Action:     eap.ActionReject,
			EAPMessage: eapFailure,
		}, nil
	}

	// ポリシー取得
	pol, err := e.policyStore.GetPolicy(ctx, eapCtx.IMSI)
	if err != nil {
		slog.Warn("ポリシー取得失敗",
			"event_id", "AUTH_POLICY_NOT_FOUND",
			"trace_id", traceID,
			"imsi", maskedIMSI,
			"error", err,
		)
		_ = e.ctxStore.Delete(ctx, traceID)
		eapFailure, _ := eap.BuildEAPFailure(pkt.Identifier + 1)
		return &eap.Result{
			Action:     eap.ActionReject,
			EAPMessage: eapFailure,
		}, nil
	}

	// ポリシー評価
	ssid := policy.ExtractSSID(req.CalledStation)
	evalResult := e.evaluator.Evaluate(pol, req.NASIdentifier, ssid)
	if !evalResult.Allowed {
		slog.Warn("ポリシー拒否",
			"event_id", "AUTH_POLICY_DENIED",
			"trace_id", traceID,
			"imsi", maskedIMSI,
			"reason", evalResult.DenyReason,
		)
		_ = e.ctxStore.Delete(ctx, traceID)
		eapFailure, _ := eap.BuildEAPFailure(pkt.Identifier + 1)
		return &eap.Result{
			Action:     eap.ActionReject,
			EAPMessage: eapFailure,
		}, nil
	}

	// セッション作成
	sessionID := session.GenerateSessionID()
	sess := &session.Session{
		IMSI:      eapCtx.IMSI,
		NasIP:     req.SrcIP,
		StartTime: time.Now().Unix(),
	}
	if err := e.sessStore.Create(ctx, sessionID, sess); err != nil {
		slog.Error("セッション作成失敗",
			"event_id", "SESSION_CREATE_ERR",
			"trace_id", traceID,
			"error", err,
		)
		return e.buildReject(pkt.Identifier + 1), nil
	}
	if err := e.sessStore.AddUserIndex(ctx, eapCtx.IMSI, sessionID); err != nil {
		slog.Warn("ユーザーインデックス追加失敗",
			"event_id", "SESSION_INDEX_ERR",
			"trace_id", traceID,
			"error", err,
		)
		// インデックス失敗は致命的ではない
	}

	// EAPContext削除
	_ = e.ctxStore.Delete(ctx, traceID)

	// EAP-Success構築
	eapSuccess, _ := eap.BuildEAPSuccess(pkt.Identifier + 1)

	// VLAN/Timeout取得
	var vlanID string
	var sessionTimeout int
	if evalResult.MatchedRule != nil {
		vlanID = evalResult.MatchedRule.VlanID
		sessionTimeout = evalResult.MatchedRule.SessionTimeout
	}

	slog.Info("認証成功",
		"event_id", "AUTH_SUCCESS",
		"trace_id", traceID,
		"imsi", maskedIMSI,
		"session_id", sessionID,
	)

	return &eap.Result{
		Action:         eap.ActionAccept,
		EAPMessage:     eapSuccess,
		IMSI:           eapCtx.IMSI,
		SessionID:      sessionID,
		MSK:            mskBytes,
		VlanID:         vlanID,
		SessionTimeout: sessionTimeout,
	}, nil
}

// handleResync は再同期失敗応答を処理する
func (e *EngineImpl) handleResync(ctx context.Context, req *eap.Request, traceID string, eapCtx *session.EAPContext, pkt *eapaka.Packet) (*eap.Result, error) {
	maskedIMSI := e.maskIMSI(eapCtx.IMSI)

	// 状態遷移検証
	if eap.EAPState(eapCtx.Stage) != eap.StateChallengeSent {
		slog.Warn("不正な状態で再同期応答受信",
			"event_id", "EAP_STATE_ERR",
			"trace_id", traceID,
			"stage", eapCtx.Stage,
		)
		return e.buildReject(pkt.Identifier + 1), nil
	}

	// 再同期回数チェック
	if eapCtx.ResyncCount >= config.MaxResyncCount {
		slog.Warn("再同期上限超過",
			"event_id", "AUTH_RESYNC_LIMIT",
			"trace_id", traceID,
			"imsi", maskedIMSI,
			"resync_count", eapCtx.ResyncCount,
		)
		_ = e.ctxStore.Delete(ctx, traceID)
		eapFailure, _ := eap.BuildEAPFailure(pkt.Identifier + 1)
		return &eap.Result{
			Action:     eap.ActionReject,
			EAPMessage: eapFailure,
		}, nil
	}

	// AT_AUTS抽出
	atAuts, found := eap.GetAttribute[*eapaka.AtAuts](pkt)
	if !found {
		slog.Warn("AT_AUTSが見つからない",
			"event_id", "EAP_PARSE_ERR",
			"trace_id", traceID,
			"imsi", maskedIMSI,
		)
		return e.buildReject(pkt.Identifier + 1), nil
	}

	// Vector Gateway呼び出し（再同期情報付き）
	vCtx := vector.WithTraceID(ctx, traceID)
	vecResp, err := e.vectorClient.GetVector(vCtx, &vector.VectorRequest{
		IMSI: eapCtx.IMSI,
		ResyncInfo: &vector.ResyncInfo{
			RAND: eapCtx.RAND,
			AUTS: hex.EncodeToString(atAuts.Auts),
		},
	})
	if err != nil {
		e.logVectorError(err, traceID, maskedIMSI)
		_ = e.ctxStore.Delete(ctx, traceID)
		eapFailure, _ := eap.BuildEAPFailure(pkt.Identifier + 1)
		return &eap.Result{
			Action:     eap.ActionReject,
			EAPMessage: eapFailure,
		}, nil
	}

	// Identity情報復元（鍵導出に必要）
	identity := &eap.ParsedIdentity{
		Raw:     req.UserName,
		IMSI:    eapCtx.IMSI,
		EAPType: eapCtx.EAPType,
		Type:    eap.IdentityTypePermanentAKA,
	}
	if eapCtx.EAPType == eapaka.TypeAKAPrime {
		identity.Type = eap.IdentityTypePermanentAKAPrime
	}

	// 新しい鍵導出
	var kAut, msk []byte
	if identity.IsAKAPrime() {
		keys, err := akaprime.DeriveAllKeys(identity.Raw, vecResp.CK, vecResp.IK, vecResp.AUTN, e.cfg.NetworkName)
		if err != nil {
			slog.Error("AKA'鍵導出失敗（再同期）",
				"event_id", "EAP_KEY_DERIVE_ERR",
				"trace_id", traceID,
				"imsi", maskedIMSI,
				"error", err,
			)
			return e.buildReject(pkt.Identifier + 1), nil
		}
		kAut = keys.K_aut
		msk = keys.MSK
	} else {
		keys := aka.DeriveKeys(identity.Raw, vecResp.CK, vecResp.IK)
		kAut = keys.K_aut
		msk = keys.MSK
	}

	// EAPContext更新（新RAND/AUTN/XRES/Kaut/MSK + resync_count++）
	updates := map[string]any{
		"stage":        string(eap.StateChallengeSent),
		"rand":         hex.EncodeToString(vecResp.RAND),
		"autn":         hex.EncodeToString(vecResp.AUTN),
		"xres":         hex.EncodeToString(vecResp.XRES),
		"k_aut":        hex.EncodeToString(kAut),
		"msk":          hex.EncodeToString(msk),
		"resync_count": eapCtx.ResyncCount + 1,
	}
	if err := e.ctxStore.Update(ctx, traceID, updates); err != nil {
		slog.Error("EAPコンテキスト更新失敗（再同期）",
			"event_id", "EAP_CTX_UPDATE_ERR",
			"trace_id", traceID,
			"error", err,
		)
		return e.buildReject(pkt.Identifier + 1), nil
	}

	// 新Challenge構築
	var challengeMsg []byte
	if identity.IsAKAPrime() {
		challengeMsg, err = akaprime.BuildChallenge(pkt.Identifier+1, vecResp.RAND, vecResp.AUTN, e.cfg.NetworkName, kAut)
	} else {
		challengeMsg, err = aka.BuildChallenge(pkt.Identifier+1, vecResp.RAND, vecResp.AUTN, kAut)
	}
	if err != nil {
		slog.Error("Challenge構築失敗（再同期）",
			"event_id", "EAP_BUILD_ERR",
			"trace_id", traceID,
			"error", err,
		)
		return e.buildReject(pkt.Identifier + 1), nil
	}

	slog.Info("再同期Challenge送信",
		"event_id", "EAP_RESYNC_CHALLENGE",
		"trace_id", traceID,
		"imsi", maskedIMSI,
		"resync_count", eapCtx.ResyncCount+1,
	)

	return &eap.Result{
		Action:     eap.ActionChallenge,
		EAPMessage: challengeMsg,
		State:      []byte(traceID),
		IMSI:       eapCtx.IMSI,
	}, nil
}

// logVectorError はVectorエラーをログに記録する
func (e *EngineImpl) logVectorError(err error, traceID, maskedIMSI string) {
	var apiErr *vector.APIError
	if errors.As(err, &apiErr) {
		eventID := "VECTOR_API_ERR"
		if apiErr.IsNotFound() {
			eventID = "VECTOR_IMSI_NOT_FOUND"
		}
		slog.Error("Vector APIエラー",
			"event_id", eventID,
			"trace_id", traceID,
			"imsi", maskedIMSI,
			"http_status", apiErr.StatusCode,
			"error", apiErr.Error(),
		)
		return
	}

	var connErr *vector.ConnectionError
	if errors.As(err, &connErr) {
		slog.Error("Vector接続エラー",
			"event_id", "VECTOR_CONN_ERR",
			"trace_id", traceID,
			"imsi", maskedIMSI,
			"error", connErr.Error(),
		)
		return
	}

	if errors.Is(err, vector.ErrCircuitOpen) {
		slog.Error("Circuit Breaker Open",
			"event_id", "VECTOR_CB_OPEN",
			"trace_id", traceID,
			"imsi", maskedIMSI,
		)
		return
	}

	slog.Error("Vector不明エラー",
		"event_id", "VECTOR_UNKNOWN_ERR",
		"trace_id", traceID,
		"imsi", maskedIMSI,
		"error", err.Error(),
	)
}

// buildReject はEAP-Failure付きReject結果を生成する
func (e *EngineImpl) buildReject(identifier uint8) *eap.Result {
	eapFailure, _ := eap.BuildEAPFailure(identifier)
	return &eap.Result{
		Action:     eap.ActionReject,
		EAPMessage: eapFailure,
	}
}

// maskIMSI はIMSIマスキングのラッパー
func (e *EngineImpl) maskIMSI(imsi string) string {
	return logging.MaskIMSI(imsi, e.cfg.LogMaskIMSI)
}
