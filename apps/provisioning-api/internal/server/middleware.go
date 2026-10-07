package server

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/auth"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/dto"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/handler"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/logging"
)

const traceIDHeader = "X-Trace-ID"

var (
	// traceIDPattern は受け付けるトレースID（印字可能ASCII 1〜64文字）。それ以外はサーバーが採番し直す
	traceIDPattern = regexp.MustCompile(`^[\x21-\x7E]{1,64}$`)
	// operatorPattern は X-Operator-Id の形式（aka-only-server と同じ）
	operatorPattern = regexp.MustCompile(`^[A-Za-z0-9._@-]{1,64}$`)
)

// newTraceID はトレースID（16バイトの乱数の16進表記）を採番する。
func newTraceID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// TraceIDMiddleware は X-Trace-ID ヘッダーのトレースIDを使い、なければ採番する。
// 使ったトレースIDは応答の X-Trace-ID ヘッダーで返す。
func TraceIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader(traceIDHeader)
		if !traceIDPattern.MatchString(traceID) {
			traceID = newTraceID()
		}
		c.Set(handler.TraceIDKey, traceID)
		c.Header(traceIDHeader, traceID)
		c.Next()
	}
}

// MaskPath はパスのうち、7桁を超える数字だけのセグメント（IMSI 等）を logging.Masker でマスクする。
// 15桁でない誤った IMSI もマスクの対象にする。
func MaskPath(path string, masker *logging.Masker) string {
	segments := strings.Split(path, "/")
	for i, s := range segments {
		if len(s) > 7 && isDigits(s) {
			segments[i] = masker.IMSI(s)
		}
	}
	return strings.Join(segments, "/")
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// LoggingMiddleware は処理を終えたリクエストを request completed として記録する（D-13 §6.1）。
// パスの IMSI は LOG_MASK_IMSI に従ってマスクする。
func LoggingMiddleware(log *slog.Logger, masker *logging.Masker) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Set(handler.MaskedPathKey, MaskPath(c.Request.URL.Path, masker))

		c.Next()

		srcIP, _, err := net.SplitHostPort(c.Request.RemoteAddr)
		if err != nil {
			srcIP = c.Request.RemoteAddr
		}
		log.Info("request completed",
			"trace_id", c.GetString(handler.TraceIDKey),
			"method", c.Request.Method,
			"path", c.GetString(handler.MaskedPathKey),
			"http_status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"mgmt_client", c.GetString(handler.MgmtClientKey),
			"src_ip", srcIP,
		)
	}
}

// RecoveryMiddleware はパニックから復旧し、詳細を伏せた 500 を返す。
func RecoveryMiddleware(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("panic recovered",
					"event_id", "PROV_REQUEST_ERR",
					"trace_id", c.GetString(handler.TraceIDKey),
					"method", c.Request.Method,
					"path", c.GetString(handler.MaskedPathKey),
					"error", r,
				)
				handler.WriteProblem(c, dto.NewProblem(http.StatusInternalServerError, dto.CauseSystemFailure, ""))
			}
		}()
		c.Next()
	}
}

// MgmtClientMiddleware は、クライアント証明書に対応する管理クライアントの識別名を記録する。
// 未登録の証明書は TLS ハンドシェイクで拒否しているため、ここでは識別名を引くだけにする。
func MgmtClientMiddleware(clients auth.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(handler.MgmtClientKey, clients.NameFromRequest(c.Request))
		c.Next()
	}
}

// OperatorMiddleware は X-Operator-Id ヘッダーの形式を確認する。ヘッダーは省略できる。
func OperatorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if v := c.GetHeader(handler.OperatorHeader); v != "" && !operatorPattern.MatchString(v) {
			p := dto.NewProblem(http.StatusBadRequest, dto.CauseOptionalIEIncorrect, "")
			p.InvalidParams = []dto.InvalidParam{{Param: handler.OperatorHeader, Reason: "must match " + operatorPattern.String()}}
			handler.WriteProblem(c, p)
			return
		}
		c.Next()
	}
}
