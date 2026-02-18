package session

import "context"

// ContextStore はEAP認証コンテキストのCRUD操作を定義する。
type ContextStore interface {
	Create(ctx context.Context, traceID string, eapCtx *EAPContext) error
	Get(ctx context.Context, traceID string) (*EAPContext, error)
	Update(ctx context.Context, traceID string, updates map[string]any) error
	Delete(ctx context.Context, traceID string) error
	Exists(ctx context.Context, traceID string) (bool, error)
}

// SessionStore はアクティブセッションの操作を定義する。
type SessionStore interface {
	Create(ctx context.Context, sessionID string, sess *Session) error
	Get(ctx context.Context, sessionID string) (*Session, error)
	AddUserIndex(ctx context.Context, imsi string, sessionID string) error
}
