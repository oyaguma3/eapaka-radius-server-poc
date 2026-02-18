package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/config"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/dto"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/milenage"
)

// VectorUseCase はベクター生成ユースケースを実装する。
type VectorUseCase struct {
	subscriberStore    SubscriberRepository
	calculator         MilenageCalculator
	sqnManager         SQNManager
	sqnValidator       SQNValidator
	resyncProcessor    ResyncProcessor
	testVectorProvider TestVectorProvider // nilの場合はテストモード無効
	cfg                *config.Config
}

// NewVectorUseCase は新しいVectorUseCaseを生成する。
func NewVectorUseCase(
	subscriberStore SubscriberRepository,
	calculator MilenageCalculator,
	sqnManager SQNManager,
	sqnValidator SQNValidator,
	resyncProcessor ResyncProcessor,
	testVectorProvider TestVectorProvider,
	cfg *config.Config,
) *VectorUseCase {
	return &VectorUseCase{
		subscriberStore:    subscriberStore,
		calculator:         calculator,
		sqnManager:         sqnManager,
		sqnValidator:       sqnValidator,
		resyncProcessor:    resyncProcessor,
		testVectorProvider: testVectorProvider,
		cfg:                cfg,
	}
}

// GenerateVector はベクターを生成する。
func (u *VectorUseCase) GenerateVector(ctx context.Context, req *dto.VectorRequest) (*dto.VectorResponse, error) {
	// 0. テストモード判定（有効な場合）
	if u.testVectorProvider != nil && u.testVectorProvider.IsTestIMSI(req.IMSI) {
		return u.generateTestVector(ctx, req)
	}

	// 1. 加入者情報取得
	sub, err := u.subscriberStore.Get(ctx, req.IMSI)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValkeyConnection, err)
	}
	if sub == nil {
		return nil, ErrSubscriberNotFound
	}

	// 2. 鍵情報をバイト列に変換
	ki, err := milenage.HexDecode(sub.Ki)
	if err != nil {
		return nil, fmt.Errorf("invalid Ki format: %w", err)
	}
	opc, err := milenage.HexDecode(sub.OPc)
	if err != nil {
		return nil, fmt.Errorf("invalid OPc format: %w", err)
	}
	amf, err := milenage.HexDecode(sub.AMF)
	if err != nil {
		return nil, fmt.Errorf("invalid AMF format: %w", err)
	}
	currentSQN, err := u.sqnManager.ParseHex(sub.SQN)
	if err != nil {
		return nil, fmt.Errorf("invalid SQN format: %w", err)
	}

	var newSQN uint64

	// 3. 再同期処理 or 通常処理
	if req.ResyncInfo != nil {
		newSQN, err = u.processResync(ki, opc, req.ResyncInfo, currentSQN)
		if err != nil {
			return nil, err
		}
	} else {
		newSQN, err = u.sqnManager.Increment(currentSQN)
		if err != nil {
			return nil, ErrSQNOverflow
		}
	}

	// 4. ベクター生成
	vector, err := u.calculator.GenerateVector(ki, opc, amf, newSQN)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMilenageCalculation, err)
	}

	// 5. SQN更新
	newSQNHex := u.sqnManager.FormatHex(newSQN)
	if err := u.subscriberStore.UpdateSQN(ctx, req.IMSI, newSQNHex); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValkeyConnection, err)
	}

	// 6. レスポンス変換
	return milenage.VectorToResponse(vector), nil
}

// processResync は再同期処理を行う。
func (u *VectorUseCase) processResync(ki, opc []byte, resyncInfo *dto.ResyncInfo, currentSQN uint64) (uint64, error) {
	// 1. RAND/AUTS をバイト列に変換
	randVal, err := milenage.HexDecode(resyncInfo.RAND)
	if err != nil {
		return 0, fmt.Errorf("%w: invalid RAND format", ErrResyncInvalidFormat)
	}
	auts, err := milenage.HexDecode(resyncInfo.AUTS)
	if err != nil {
		return 0, fmt.Errorf("%w: invalid AUTS format", ErrResyncInvalidFormat)
	}

	// 2. AUTS長検証
	if len(auts) != 14 {
		return 0, ErrResyncInvalidFormat
	}

	// 3. SQN_MS抽出
	sqnMS, err := u.resyncProcessor.ExtractSQN(ki, opc, randVal, auts)
	if err != nil {
		// MAC検証失敗
		return 0, ErrResyncMACFailed
	}

	// 4. デルタ検証
	if err := u.sqnValidator.ValidateResyncSQN(sqnMS, currentSQN); err != nil {
		slog.Warn("SQN delta validation failed",
			"event_id", "SQN_RESYNC_DELTA_ERR",
			"sqn_ms", fmt.Sprintf("%012x", sqnMS),
			"sqn_he", fmt.Sprintf("%012x", currentSQN),
			"error", err.Error(),
		)
		return 0, ErrResyncDeltaExceeded
	}

	// 5. 新SQN計算（SQN_MS + 32）
	newSQN, err := u.sqnValidator.ComputeResyncSQN(sqnMS)
	if err != nil {
		return 0, ErrSQNOverflow
	}

	// 6. SQN再同期成功ログ
	slog.Info("SQN resync successful",
		"event_id", "SQN_RESYNC",
		"sqn_old", fmt.Sprintf("%012x", currentSQN),
		"sqn_ms", fmt.Sprintf("%012x", sqnMS),
		"sqn_new", fmt.Sprintf("%012x", newSQN),
	)

	return newSQN, nil
}

// generateTestVector はテストモード用のベクターを生成する。
// Ki/OPc/AMFは固定値を使用し、SQNはValkey経由でステートフルに管理する。
func (u *VectorUseCase) generateTestVector(ctx context.Context, req *dto.VectorRequest) (*dto.VectorResponse, error) {
	// 1. テスト用暗号パラメータ取得
	ki, opc, amf := u.testVectorProvider.GetTestCryptoParams()

	// 2. ValkeyからSQN取得（失敗時はデフォルトSQNにフォールバック）
	var currentSQN uint64
	sub, err := u.subscriberStore.Get(ctx, req.IMSI)
	if err != nil || sub == nil {
		currentSQN = u.testVectorProvider.GetDefaultSQN()
		slog.Info("test mode: using default SQN (Valkey unavailable or subscriber not found)",
			"event_id", "TEST_SQN_FALLBACK",
			"imsi", req.IMSI,
			"default_sqn", fmt.Sprintf("%012x", currentSQN),
		)
	} else {
		currentSQN, err = u.sqnManager.ParseHex(sub.SQN)
		if err != nil {
			currentSQN = u.testVectorProvider.GetDefaultSQN()
			slog.Warn("test mode: SQN parse failed, using default",
				"event_id", "TEST_SQN_PARSE_ERR",
				"raw_sqn", sub.SQN,
				"error", err.Error(),
			)
		}
	}

	var newSQN uint64

	// 3. 再同期処理 or 通常処理
	if req.ResyncInfo != nil {
		newSQN, err = u.processResync(ki, opc, req.ResyncInfo, currentSQN)
		if err != nil {
			return nil, err
		}
	} else {
		newSQN, err = u.sqnManager.Increment(currentSQN)
		if err != nil {
			return nil, ErrSQNOverflow
		}
	}

	// 4. ベクター生成
	vector, err := u.calculator.GenerateVector(ki, opc, amf, newSQN)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMilenageCalculation, err)
	}

	// 5. ValkeyにSQN書き戻し
	newSQNHex := u.sqnManager.FormatHex(newSQN)
	if err := u.subscriberStore.UpdateSQN(ctx, req.IMSI, newSQNHex); err != nil {
		slog.Warn("test mode: failed to persist SQN to Valkey",
			"event_id", "TEST_SQN_PERSIST_ERR",
			"imsi", req.IMSI,
			"error", err.Error(),
		)
	}

	slog.Info("test vector generated",
		"event_id", "CALC_OK",
		"test_mode", true,
		"sqn", newSQNHex,
	)

	return milenage.VectorToResponse(vector), nil
}
