// Package handler は Provisioning API の HTTP ハンドラーを提供する。
// ハンドラーは要求の JSON の読み込みと応答の組み立てだけを行い、検証・正規化・監査ログは service 層で行う（D-13 §7.1）。
package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/audit"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/dto"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/service"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/masterdata"
)

// gin.Context に保存する値のキー（server のミドルウェアが設定する）
const (
	// TraceIDKey はトレースID
	TraceIDKey = "trace_id"
	// MgmtClientKey は管理クライアントの識別名
	MgmtClientKey = "mgmt_client"
	// MaskedPathKey はログ用に IMSI をマスクしたパス
	MaskedPathKey = "masked_path"
)

// OperatorHeader は操作者 ID のヘッダー
const OperatorHeader = "X-Operator-Id"

// maxBodyBytes は要求の本文の上限
const maxBodyBytes = 256 << 10

// 要求の Content-Type
const (
	contentTypeJSON       = "application/json"
	contentTypeMergePatch = "application/merge-patch+json"
)

// basePath は Location ヘッダーに使う API のベースパス
const basePath = "/admin/v1"

// Handler は Provisioning API の HTTP ハンドラー。
type Handler struct {
	svc       *service.Service
	log       *slog.Logger
	version   string
	nodeName  string
	startedAt time.Time
}

// New は新しい Handler を生成する。
func New(svc *service.Service, log *slog.Logger, version, nodeName string, startedAt time.Time) *Handler {
	return &Handler{svc: svc, log: log, version: version, nodeName: nodeName, startedAt: startedAt}
}

// actor は監査ログに記録する操作者を返す。
func actor(c *gin.Context) audit.Actor {
	return audit.Actor{
		Operator:   c.GetHeader(OperatorHeader),
		MgmtClient: c.GetString(MgmtClientKey),
		TraceID:    c.GetString(TraceIDKey),
	}
}

// listQuery は一覧のクエリパラメーターを返す。
func listQuery(c *gin.Context) dto.ListQuery {
	return dto.ListQuery{Prefix: c.Query("prefix"), Cursor: c.Query("cursor"), Limit: c.Query("limit")}
}

// WriteProblem はエラー応答を書き込む。
func WriteProblem(c *gin.Context, p *dto.ProblemDetails) {
	c.Header("Content-Type", dto.ContentTypeProblem)
	c.AbortWithStatusJSON(p.Status, p)
}

// fail は service のエラーを HTTP の応答にする。想定外のエラーはログに残し、詳細を伏せた 500 にする。
func (h *Handler) fail(c *gin.Context, err error) {
	var ve *service.ValidationError
	switch {
	case errors.As(err, &ve):
		p := dto.NewProblem(http.StatusBadRequest, ve.Cause(), ve.Detail)
		p.InvalidParams = ve.Params()
		WriteProblem(c, p)
	case errors.Is(err, masterdata.ErrSubscriberNotFound):
		WriteProblem(c, dto.NewProblem(http.StatusNotFound, dto.CauseUserNotFound, "subscriber not found"))
	case errors.Is(err, masterdata.ErrClientNotFound):
		WriteProblem(c, dto.NewProblem(http.StatusNotFound, dto.CauseClientNotFound, "client not found"))
	case errors.Is(err, masterdata.ErrPolicyNotFound):
		WriteProblem(c, dto.NewProblem(http.StatusNotFound, dto.CausePolicyNotFound, "policy not found"))
	case errors.Is(err, masterdata.ErrSubscriberExists):
		WriteProblem(c, dto.NewProblem(http.StatusConflict, dto.CauseSubscriberAlreadyExists, "subscriber already exists"))
	case errors.Is(err, masterdata.ErrClientExists):
		WriteProblem(c, dto.NewProblem(http.StatusConflict, dto.CauseClientAlreadyExists, "client already exists"))
	default:
		h.log.Error("request failed",
			"event_id", "PROV_REQUEST_ERR",
			"trace_id", c.GetString(TraceIDKey),
			"method", c.Request.Method,
			"path", c.GetString(MaskedPathKey),
			"error", err.Error(),
		)
		WriteProblem(c, dto.NewProblem(http.StatusInternalServerError, dto.CauseSystemFailure, ""))
	}
}

// decode は要求の本文を v に読み込む。読めなければエラー応答を書き込んで false を返す。
// Content-Type は application/json（PATCH では application/merge-patch+json も）に限り、未知の項目は拒否する。
func decode(c *gin.Context, v any) bool {
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	allowed := err == nil && (mediaType == contentTypeJSON ||
		(c.Request.Method == http.MethodPatch && mediaType == contentTypeMergePatch))
	if !allowed {
		detail := "Content-Type must be " + contentTypeJSON
		if c.Request.Method == http.MethodPatch {
			detail = "Content-Type must be " + contentTypeMergePatch + " or " + contentTypeJSON
		}
		WriteProblem(c, dto.NewProblem(http.StatusBadRequest, dto.CauseInvalidMsgFormat, detail))
		return false
	}

	dec := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil || dec.Decode(&struct{}{}) != io.EOF {
		WriteProblem(c, dto.NewProblem(http.StatusBadRequest, dto.CauseInvalidMsgFormat,
			"request body is not valid JSON for this operation"))
		return false
	}
	return true
}

// ---- 状態 ----

// GetStatus は GET /status を処理する。
func (h *Handler) GetStatus(c *gin.Context) {
	counts, err := h.svc.Counts(c.Request.Context())
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.Status{
		Version:         h.version,
		NodeName:        h.nodeName,
		StartedAt:       h.startedAt.UTC(),
		SubscriberCount: counts.Subscribers,
		ClientCount:     counts.Clients,
		PolicyCount:     counts.Policies,
	})
}

// ---- 加入者 ----

// ListSubscribers は GET /subscribers を処理する。
func (h *Handler) ListSubscribers(c *gin.Context) {
	page, err := h.svc.ListSubscribers(c.Request.Context(), listQuery(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	out := dto.SubscriberList{Items: make([]dto.Subscriber, len(page.Items)), Total: page.Total, NextCursor: page.NextCursor}
	for i, sub := range page.Items {
		out.Items[i] = dto.NewSubscriber(sub)
	}
	c.JSON(http.StatusOK, out)
}

// CreateSubscriber は POST /subscribers を処理する。
func (h *Handler) CreateSubscriber(c *gin.Context) {
	var req dto.SubscriberCreate
	if !decode(c, &req) {
		return
	}
	sub, err := h.svc.CreateSubscriber(c.Request.Context(), actor(c), req)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.Header("Location", basePath+"/subscribers/"+sub.IMSI)
	c.JSON(http.StatusCreated, dto.NewSubscriber(sub))
}

// GetSubscriber は GET /subscribers/{imsi} を処理する。
func (h *Handler) GetSubscriber(c *gin.Context) {
	sub, err := h.svc.GetSubscriber(c.Request.Context(), c.Param("imsi"))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewSubscriber(sub))
}

// UpdateSubscriber は PATCH /subscribers/{imsi} を処理する。
func (h *Handler) UpdateSubscriber(c *gin.Context) {
	var req dto.SubscriberUpdate
	if !decode(c, &req) {
		return
	}
	sub, err := h.svc.UpdateSubscriber(c.Request.Context(), actor(c), c.Param("imsi"), req)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewSubscriber(sub))
}

// DeleteSubscriber は DELETE /subscribers/{imsi} を処理する。
func (h *Handler) DeleteSubscriber(c *gin.Context) {
	if err := h.svc.DeleteSubscriber(c.Request.Context(), actor(c), c.Param("imsi")); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// GetSubscriberKeys は GET /subscribers/{imsi}/keys を処理する。
func (h *Handler) GetSubscriberKeys(c *gin.Context) {
	sub, err := h.svc.GetSubscriberKeys(c.Request.Context(), actor(c), c.Param("imsi"))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewSubscriberKeys(sub))
}

// ---- RADIUSクライアント ----

// ListClients は GET /clients を処理する。
func (h *Handler) ListClients(c *gin.Context) {
	clients, err := h.svc.ListClients(c.Request.Context())
	if err != nil {
		h.fail(c, err)
		return
	}
	out := dto.ClientList{Items: make([]dto.Client, len(clients))}
	for i, rc := range clients {
		out.Items[i] = dto.NewClient(rc)
	}
	c.JSON(http.StatusOK, out)
}

// CreateClient は POST /clients を処理する。
func (h *Handler) CreateClient(c *gin.Context) {
	var req dto.ClientCreate
	if !decode(c, &req) {
		return
	}
	rc, err := h.svc.CreateClient(c.Request.Context(), actor(c), req)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.Header("Location", basePath+"/clients/"+rc.IP)
	c.JSON(http.StatusCreated, dto.NewClient(rc))
}

// GetClient は GET /clients/{ip} を処理する。
func (h *Handler) GetClient(c *gin.Context) {
	rc, err := h.svc.GetClient(c.Request.Context(), c.Param("ip"))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewClient(rc))
}

// UpdateClient は PATCH /clients/{ip} を処理する。
func (h *Handler) UpdateClient(c *gin.Context) {
	var req dto.ClientUpdate
	if !decode(c, &req) {
		return
	}
	rc, err := h.svc.UpdateClient(c.Request.Context(), actor(c), c.Param("ip"), req)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewClient(rc))
}

// DeleteClient は DELETE /clients/{ip} を処理する。
func (h *Handler) DeleteClient(c *gin.Context) {
	if err := h.svc.DeleteClient(c.Request.Context(), actor(c), c.Param("ip")); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// GetClientSecret は GET /clients/{ip}/secret を処理する。
func (h *Handler) GetClientSecret(c *gin.Context) {
	rc, err := h.svc.GetClientSecret(c.Request.Context(), actor(c), c.Param("ip"))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ClientSecret{Secret: rc.Secret})
}

// ---- 認可ポリシー ----

// ListPolicies は GET /policies を処理する。
func (h *Handler) ListPolicies(c *gin.Context) {
	page, err := h.svc.ListPolicies(c.Request.Context(), listQuery(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	out := dto.PolicyList{Items: make([]dto.Policy, len(page.Items)), Total: page.Total, NextCursor: page.NextCursor}
	for i, p := range page.Items {
		out.Items[i] = dto.NewPolicy(p)
	}
	c.JSON(http.StatusOK, out)
}

// GetPolicy は GET /policies/{imsi} を処理する。
func (h *Handler) GetPolicy(c *gin.Context) {
	p, err := h.svc.GetPolicy(c.Request.Context(), c.Param("imsi"))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewPolicy(p))
}

// PutPolicy は PUT /policies/{imsi} を処理する。作成したら 201、置き換えたら 200 を返す。
func (h *Handler) PutPolicy(c *gin.Context) {
	var req dto.PolicyPut
	if !decode(c, &req) {
		return
	}
	p, created, err := h.svc.PutPolicy(c.Request.Context(), actor(c), c.Param("imsi"), req)
	if err != nil {
		h.fail(c, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
		c.Header("Location", basePath+"/policies/"+p.IMSI)
	}
	c.JSON(status, dto.NewPolicy(p))
}

// DeletePolicy は DELETE /policies/{imsi} を処理する。
func (h *Handler) DeletePolicy(c *gin.Context) {
	if err := h.svc.DeletePolicy(c.Request.Context(), actor(c), c.Param("imsi")); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
