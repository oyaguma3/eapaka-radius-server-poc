package session

import (
	"context"
	"regexp"
	"strings"

	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/logging"
)

var imsiPattern = regexp.MustCompile(`^[0-9]{15}$`)

// identifierResolver はIdentifierResolverインターフェースの実装。
type identifierResolver struct {
	sessionManager SessionManager
	maskEnabled    bool
}

// NewIdentifierResolver は新しいIdentifierResolverを生成する。
func NewIdentifierResolver(sm SessionManager, maskEnabled bool) IdentifierResolver {
	return &identifierResolver{
		sessionManager: sm,
		maskEnabled:    maskEnabled,
	}
}

// ResolveIMSI はログ出力用のIMSI/識別子を取得する。
// 優先順位: セッション→User-Name→Class UUID→"unknown"
func (r *identifierResolver) ResolveIMSI(ctx context.Context, sessionUUID, userName, classUUID string) string {
	// 1. セッションからIMSI取得
	if sessionUUID != "" {
		sess, err := r.sessionManager.Get(ctx, sessionUUID)
		if err == nil && sess != nil && sess.IMSI != "" {
			return logging.MaskIMSI(sess.IMSI, r.maskEnabled)
		}
	}

	// 2. User-NameからIMSI抽出
	if userName != "" {
		imsi := extractIMSIFromIdentity(userName)
		if imsi != "" {
			return logging.MaskIMSI(imsi, r.maskEnabled)
		}
		// 3. IMSI抽出失敗、User-Nameをそのまま返却
		return userName
	}

	// 4. Class UUID
	if classUUID != "" {
		return classUUID
	}

	// 5. 取得失敗
	return "unknown"
}

// extractIMSIFromIdentity はEAP Identity形式からIMSIを抽出する。
// 形式: "0<IMSI>@<realm>" または "6<IMSI>@<realm>"
func extractIMSIFromIdentity(identity string) string {
	// @でrealm部分を除去
	atIndex := strings.Index(identity, "@")
	if atIndex > 0 {
		identity = identity[:atIndex]
	}

	// 先頭文字が0または6の場合、IMSI部分を抽出
	if len(identity) >= 16 {
		prefix := identity[0]
		if prefix == '0' || prefix == '6' {
			candidate := identity[1:]
			if len(candidate) == 15 && imsiPattern.MatchString(candidate) {
				return candidate
			}
		}
	}

	// 直接15桁の数字列の場合
	if imsiPattern.MatchString(identity) {
		return identity
	}

	return ""
}
