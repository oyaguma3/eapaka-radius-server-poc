// Package usecase はベクター生成のビジネスロジックを提供する。
package usecase

//go:generate mockgen -source=interfaces.go -destination=mock_interfaces.go -package=usecase

import (
	"context"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/dto"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/milenage"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/store"
)

// MilenageCalculator はMilenage計算のインターフェース。
type MilenageCalculator interface {
	GenerateVector(ki, opc, amf []byte, sqn uint64) (*milenage.Vector, error)
}

// ResyncProcessor は再同期処理のインターフェース。
type ResyncProcessor interface {
	ExtractSQN(ki, opc, rand, auts []byte) (uint64, error)
}

// SQNManager はSQN管理のインターフェース。
type SQNManager interface {
	Increment(currentSQN uint64) (uint64, error)
	FormatHex(sqn uint64) string
	ParseHex(s string) (uint64, error)
}

// SQNValidator はSQN検証のインターフェース。
type SQNValidator interface {
	ValidateResyncSQN(sqnMS, sqnHE uint64) error
	ComputeResyncSQN(sqnMS uint64) (uint64, error)
}

// SubscriberRepository は加入者データアクセスのインターフェース。
type SubscriberRepository interface {
	Get(ctx context.Context, imsi string) (*store.Subscriber, error)
	UpdateSQN(ctx context.Context, imsi string, sqn string) error
}

// TestVectorProvider はテストベクター生成のインターフェース。
type TestVectorProvider interface {
	IsTestIMSI(imsi string) bool
	GetTestVector(imsi string) (*milenage.Vector, error)
	// GetTestCryptoParams はテスト用の暗号パラメータを返す（Ki, OPc, AMF）
	GetTestCryptoParams() (ki, opc, amf []byte)
	// GetDefaultSQN はテストベクターのデフォルトSQN値を返す
	GetDefaultSQN() uint64
}

// VectorUseCaseInterface はベクター生成ユースケースのインターフェース。
type VectorUseCaseInterface interface {
	GenerateVector(ctx context.Context, req *dto.VectorRequest) (*dto.VectorResponse, error)
}
