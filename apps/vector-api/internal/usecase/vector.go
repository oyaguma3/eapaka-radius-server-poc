package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/config"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/dto"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/milenage"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/logging"
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
	// waitBeforeRetry はSQNの競合後、やり直す前に待つ（テストで差し替える）
	waitBeforeRetry func(ctx context.Context) error
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
		waitBeforeRetry:    waitRandom,
	}
}

// maxSQNAttempts はSQNの書き換えを試みる最大回数（競合したときのやり直しを含む）。
const maxSQNAttempts = 3

// errSQNChanged は、加入者を読んでからSQNを書き換えるまでの間に、
// 他のリクエストがSQNを書き換えていたことを表す（やり直しの対象）。
var errSQNChanged = errors.New("SQN was changed by another request")

// GenerateVector はベクターを生成する。
// テストモード（TEST_VECTOR_ENABLED=true かつ対象プレフィックスのIMSI）では、
// Ki/OPc/AMF をテスト用の固定値に置き換える。加入者の取得・SQNの管理・エラー処理は通常モードと同じ。
//
// SQNは、読んだ値から変わっていないときだけ書き換える（CAS）。他のリクエストと競合したときは、
// 加入者の読み出しからやり直す。maxSQNAttempts 回とも競合したときは ErrSQNConflict を返す。
func (u *VectorUseCase) GenerateVector(ctx context.Context, req *dto.VectorRequest) (*dto.VectorResponse, error) {
	testMode := u.IsTestMode(req.IMSI)

	for attempt := 1; attempt <= maxSQNAttempts; attempt++ {
		if attempt > 1 {
			if err := u.waitBeforeRetry(ctx); err != nil {
				return nil, fmt.Errorf("%w: retry aborted after %d attempts: %v", ErrSQNConflict, attempt-1, err)
			}
		}

		resp, err := u.generateOnce(ctx, req, testMode, attempt > 1)
		if !errors.Is(err, errSQNChanged) {
			return resp, err
		}

		if attempt < maxSQNAttempts {
			slog.Warn("SQN update conflict, retrying",
				"event_id", "SQN_CONFLICT_RETRY",
				"trace_id", traceIDFromContext(ctx),
				"imsi", u.maskIMSI(req.IMSI),
				"attempt", attempt,
			)
		}
	}

	return nil, fmt.Errorf("%w: conflicted %d times", ErrSQNConflict, maxSQNAttempts)
}

// generateOnce は加入者の取得からSQNの書き換え・ベクター生成までを1回行う。
// 他のリクエストが先にSQNを書き換えていたときは errSQNChanged を返す。
// retried は競合によるやり直しかどうか（再同期の判定に使う）。
func (u *VectorUseCase) generateOnce(ctx context.Context, req *dto.VectorRequest, testMode, retried bool) (*dto.VectorResponse, error) {
	// 1. 加入者情報取得（テストモードでも登録が必要）
	sub, err := u.subscriberStore.Get(ctx, req.IMSI)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValkeyConnection, err)
	}
	if sub == nil {
		return nil, ErrSubscriberNotFound
	}

	// 2. 鍵情報をバイト列に変換（テストモードは固定値）
	var ki, opc, amf []byte
	if testMode {
		ki, opc, amf = u.testVectorProvider.GetTestCryptoParams()
	} else {
		if ki, err = milenage.HexDecode(sub.Ki); err != nil {
			return nil, fmt.Errorf("invalid Ki format: %w", err)
		}
		if opc, err = milenage.HexDecode(sub.OPc); err != nil {
			return nil, fmt.Errorf("invalid OPc format: %w", err)
		}
		if amf, err = milenage.HexDecode(sub.AMF); err != nil {
			return nil, fmt.Errorf("invalid AMF format: %w", err)
		}
	}
	currentSQN, err := u.sqnManager.ParseHex(sub.SQN)
	if err != nil {
		return nil, fmt.Errorf("invalid SQN format: %w", err)
	}

	// 3. 新SQNの計算（再同期 or 通常）
	var newSQN uint64
	var resync *resyncResult
	if req.ResyncInfo != nil {
		resync, err = u.processResync(ki, opc, req.ResyncInfo, currentSQN, retried)
		if err != nil {
			return nil, err
		}
		newSQN = resync.newSQN
	} else {
		newSQN, err = u.sqnManager.Increment(currentSQN)
		if err != nil {
			return nil, ErrSQNOverflow
		}
	}

	// 4. SQN更新（読んだ値から変わっていないときだけ書き換える）
	updated, err := u.subscriberStore.CompareAndSetSQN(ctx, req.IMSI, sub.SQN, u.sqnManager.FormatHex(newSQN))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValkeyConnection, err)
	}
	if !updated {
		return nil, errSQNChanged
	}

	// 5. 再同期のログ（SQNを書き換えた後に1回だけ出す）
	if resync != nil {
		u.logResync(ctx, req.IMSI, currentSQN, resync)
	}

	// 6. ベクター生成（SQNの書き換え後に行う。失敗してもSQNが飛ぶだけで、端末の検証には影響しない）
	vector, err := u.calculator.GenerateVector(ki, opc, amf, newSQN)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMilenageCalculation, err)
	}

	// 7. レスポンス変換
	return milenage.VectorToResponse(vector), nil
}

// resyncResult は再同期で決めた新SQNを表す。
type resyncResult struct {
	sqnMS  uint64
	newSQN uint64
	// alreadySynced は、競合後のやり直しで、別のリクエストがすでにSQN_MS以上まで
	// SQNを進めていたため、通常どおり+32した場合に true
	alreadySynced bool
}

// processResync は再同期処理を行い、新しいSQNを決める。
// retried は競合によるやり直しかどうか。
func (u *VectorUseCase) processResync(ki, opc []byte, resyncInfo *dto.ResyncInfo, currentSQN uint64, retried bool) (*resyncResult, error) {
	// 1. RAND/AUTS をバイト列に変換
	randVal, err := milenage.HexDecode(resyncInfo.RAND)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid RAND format", ErrResyncInvalidFormat)
	}
	auts, err := milenage.HexDecode(resyncInfo.AUTS)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid AUTS format", ErrResyncInvalidFormat)
	}

	// 2. AUTS長検証
	if len(auts) != 14 {
		return nil, ErrResyncInvalidFormat
	}

	// 3. SQN_MS抽出
	sqnMS, err := u.resyncProcessor.ExtractSQN(ki, opc, randVal, auts)
	if err != nil {
		// MAC検証失敗
		return nil, ErrResyncMACFailed
	}

	// 4. 競合後のやり直しで、別のリクエスト（同じ再同期の再送など）がすでにSQN_MS以上まで
	// 進めていた場合は、同期済みとみなして通常どおり+32する（端末のSQN_MSより大きいので受け入れられる）
	if retried && currentSQN >= sqnMS {
		newSQN, err := u.sqnManager.Increment(currentSQN)
		if err != nil {
			return nil, ErrSQNOverflow
		}
		return &resyncResult{sqnMS: sqnMS, newSQN: newSQN, alreadySynced: true}, nil
	}

	// 5. デルタ検証
	if err := u.sqnValidator.ValidateResyncSQN(sqnMS, currentSQN); err != nil {
		// ログはハンドラーが1行で出力する（SQN値はエラー文に含める）
		return nil, fmt.Errorf("%w: sqn_ms=%012x sqn_he=%012x: %v", ErrResyncDeltaExceeded, sqnMS, currentSQN, err)
	}

	// 6. 新SQN計算（SQN_MS + 32）
	newSQN, err := u.sqnValidator.ComputeResyncSQN(sqnMS)
	if err != nil {
		return nil, ErrSQNOverflow
	}

	return &resyncResult{sqnMS: sqnMS, newSQN: newSQN}, nil
}

// logResync はSQN再同期のログを出す。
func (u *VectorUseCase) logResync(ctx context.Context, imsi string, currentSQN uint64, r *resyncResult) {
	msg := "SQN resync successful"
	if r.alreadySynced {
		msg = "SQN resync already applied by another request"
	}
	slog.Info(msg,
		"event_id", "SQN_RESYNC",
		"trace_id", traceIDFromContext(ctx),
		"imsi", u.maskIMSI(imsi),
		"sqn_old", fmt.Sprintf("%012x", currentSQN),
		"sqn_ms", fmt.Sprintf("%012x", r.sqnMS),
		"sqn_new", fmt.Sprintf("%012x", r.newSQN),
	)
}

// waitRandom は競合後のやり直しの前に、1〜10msのランダムな時間だけ待つ。
// 同時に競合したリクエストが同じタイミングでやり直さないようにする。
// 待っている間に ctx が終わったときは ctx のエラーを返す。
func waitRandom(ctx context.Context) error {
	timer := time.NewTimer(time.Millisecond + rand.N(9*time.Millisecond))
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// IsTestMode はIMSIがテストベクターモードの対象かを返す。
func (u *VectorUseCase) IsTestMode(imsi string) bool {
	return u.testVectorProvider != nil && u.testVectorProvider.IsTestIMSI(imsi)
}

// maskIMSI はログ出力用にIMSIをマスキングする。
// 設定が無い場合はマスキングを有効として扱う（LOG_MASK_IMSI の既定値 true に合わせる）。
func (u *VectorUseCase) maskIMSI(imsi string) string {
	enabled := true
	if u.cfg != nil {
		enabled = u.cfg.LogMaskIMSI
	}
	return logging.MaskIMSI(imsi, enabled)
}
