package usecase

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/dto"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/milenage"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/store"
	"go.uber.org/mock/gomock"
)

// captureLogs はテスト中の slog の出力を JSON で buf に集める。
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	return &buf
}

// subscriberWithSQN は指定したSQNを持つ有効な加入者情報を返す。
func subscriberWithSQN(sqn string) *store.Subscriber {
	sub := validSubscriber()
	sub.SQN = sqn
	return sub
}

// countingWait は呼ばれた回数を数える待ち関数を設定する。
func countingWait(uc *VectorUseCase) *int {
	calls := 0
	uc.waitBeforeRetry = func(context.Context) error {
		calls++
		return nil
	}
	return &calls
}

func TestGenerateVector_ConflictThenSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, mockRepo, mockCalc, mockSQNMgr, _, _, mockTestVP := setupUseCase(ctrl)
	uc.cfg.LogMaskIMSI = true
	waits := countingWait(uc)
	logs := captureLogs(t)

	mockTestVP.EXPECT().IsTestIMSI(normalIMSI).Return(false)
	gomock.InOrder(
		// 1回目: 読んだ後に他のリクエストが 0x40 に進めていた
		mockRepo.EXPECT().Get(gomock.Any(), normalIMSI).Return(subscriberWithSQN("000000000020"), nil),
		mockRepo.EXPECT().CompareAndSetSQN(gomock.Any(), normalIMSI, "000000000020", "000000000040").Return(false, nil),
		// 2回目: 読み直した値から計算し直す
		mockRepo.EXPECT().Get(gomock.Any(), normalIMSI).Return(subscriberWithSQN("000000000040"), nil),
		mockRepo.EXPECT().CompareAndSetSQN(gomock.Any(), normalIMSI, "000000000040", "000000000060").Return(true, nil),
		mockCalc.EXPECT().GenerateVector(gomock.Any(), gomock.Any(), gomock.Any(), uint64(0x60)).Return(dummyVector(), nil),
	)
	mockSQNMgr.EXPECT().ParseHex("000000000020").Return(uint64(0x20), nil)
	mockSQNMgr.EXPECT().ParseHex("000000000040").Return(uint64(0x40), nil)
	mockSQNMgr.EXPECT().Increment(uint64(0x20)).Return(uint64(0x40), nil)
	mockSQNMgr.EXPECT().Increment(uint64(0x40)).Return(uint64(0x60), nil)
	mockSQNMgr.EXPECT().FormatHex(uint64(0x40)).Return("000000000040")
	mockSQNMgr.EXPECT().FormatHex(uint64(0x60)).Return("000000000060")

	ctx := ContextWithTraceID(context.Background(), "trace-conflict")
	resp, err := uc.GenerateVector(ctx, &dto.VectorRequest{IMSI: normalIMSI})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if *waits != 1 {
		t.Errorf("waitBeforeRetry called %d times, want 1", *waits)
	}

	out := logs.String()
	if n := strings.Count(out, `"event_id":"SQN_CONFLICT_RETRY"`); n != 1 {
		t.Errorf("SQN_CONFLICT_RETRY logged %d times, want 1: %s", n, out)
	}
	for _, want := range []string{
		`"level":"WARN"`,
		`"trace_id":"trace-conflict"`,
		`"imsi":"440101********0"`,
		`"attempt":1`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("log does not contain %s: %s", want, out)
		}
	}
}

func TestGenerateVector_ConflictExceeded(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, mockRepo, _, mockSQNMgr, _, _, mockTestVP := setupUseCase(ctrl)
	waits := countingWait(uc)
	logs := captureLogs(t)

	// 3回とも競合する（ベクターは計算しない）
	mockTestVP.EXPECT().IsTestIMSI(normalIMSI).Return(false)
	mockRepo.EXPECT().Get(gomock.Any(), normalIMSI).Return(validSubscriber(), nil).Times(maxSQNAttempts)
	mockSQNMgr.EXPECT().ParseHex(validHexSQN).Return(uint64(0x20), nil).Times(maxSQNAttempts)
	mockSQNMgr.EXPECT().Increment(uint64(0x20)).Return(uint64(0x40), nil).Times(maxSQNAttempts)
	mockSQNMgr.EXPECT().FormatHex(uint64(0x40)).Return("000000000040").Times(maxSQNAttempts)
	mockRepo.EXPECT().CompareAndSetSQN(gomock.Any(), normalIMSI, validHexSQN, "000000000040").
		Return(false, nil).Times(maxSQNAttempts)

	resp, err := uc.GenerateVector(context.Background(), &dto.VectorRequest{IMSI: normalIMSI})
	if !errors.Is(err, ErrSQNConflict) {
		t.Fatalf("expected ErrSQNConflict, got: %v", err)
	}
	if resp != nil {
		t.Error("expected nil response")
	}
	if !strings.Contains(err.Error(), "conflicted 3 times") {
		t.Errorf("error does not contain attempts: %v", err)
	}
	if *waits != maxSQNAttempts-1 {
		t.Errorf("waitBeforeRetry called %d times, want %d", *waits, maxSQNAttempts-1)
	}

	// やり直すときだけ SQN_CONFLICT_RETRY を出す（上限超過のログはハンドラーが出す）
	out := logs.String()
	if n := strings.Count(out, `"event_id":"SQN_CONFLICT_RETRY"`); n != maxSQNAttempts-1 {
		t.Errorf("SQN_CONFLICT_RETRY logged %d times, want %d: %s", n, maxSQNAttempts-1, out)
	}
	if !strings.Contains(out, `"attempt":2`) || strings.Contains(out, `"attempt":3`) {
		t.Errorf("unexpected attempt values: %s", out)
	}
}

func TestGenerateVector_ConflictRetryAborted(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, mockRepo, _, mockSQNMgr, _, _, mockTestVP := setupUseCase(ctrl)
	uc.waitBeforeRetry = func(context.Context) error { return context.DeadlineExceeded }

	// 競合した後、待っている間に期限が切れたら、やり直さずに ErrSQNConflict を返す
	mockTestVP.EXPECT().IsTestIMSI(normalIMSI).Return(false)
	mockRepo.EXPECT().Get(gomock.Any(), normalIMSI).Return(validSubscriber(), nil)
	mockSQNMgr.EXPECT().ParseHex(validHexSQN).Return(uint64(0x20), nil)
	mockSQNMgr.EXPECT().Increment(uint64(0x20)).Return(uint64(0x40), nil)
	mockSQNMgr.EXPECT().FormatHex(uint64(0x40)).Return("000000000040")
	mockRepo.EXPECT().CompareAndSetSQN(gomock.Any(), normalIMSI, validHexSQN, "000000000040").Return(false, nil)

	_, err := uc.GenerateVector(context.Background(), &dto.VectorRequest{IMSI: normalIMSI})
	if !errors.Is(err, ErrSQNConflict) {
		t.Fatalf("expected ErrSQNConflict, got: %v", err)
	}
	if !strings.Contains(err.Error(), "retry aborted after 1 attempts") ||
		!strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
		t.Errorf("error does not contain cause: %v", err)
	}
}

func TestGenerateVector_ConflictThenSubscriberDeleted(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, mockRepo, _, mockSQNMgr, _, _, mockTestVP := setupUseCase(ctrl)

	// 競合の後に加入者が削除されていたら 404 相当のエラー
	mockTestVP.EXPECT().IsTestIMSI(normalIMSI).Return(false)
	gomock.InOrder(
		mockRepo.EXPECT().Get(gomock.Any(), normalIMSI).Return(validSubscriber(), nil),
		mockRepo.EXPECT().CompareAndSetSQN(gomock.Any(), normalIMSI, validHexSQN, "000000000040").Return(false, nil),
		mockRepo.EXPECT().Get(gomock.Any(), normalIMSI).Return(nil, nil),
	)
	mockSQNMgr.EXPECT().ParseHex(validHexSQN).Return(uint64(0x20), nil)
	mockSQNMgr.EXPECT().Increment(uint64(0x20)).Return(uint64(0x40), nil)
	mockSQNMgr.EXPECT().FormatHex(uint64(0x40)).Return("000000000040")

	_, err := uc.GenerateVector(context.Background(), &dto.VectorRequest{IMSI: normalIMSI})
	if !errors.Is(err, ErrSubscriberNotFound) {
		t.Fatalf("expected ErrSubscriberNotFound, got: %v", err)
	}
}

func TestGenerateVector_TestMode_ConflictThenSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, mockRepo, mockCalc, mockSQNMgr, _, _, mockTestVP := setupUseCase(ctrl)

	// テストモードでも同じく読み直してやり直す（鍵はテスト用の固定値）
	mockTestVP.EXPECT().IsTestIMSI(testIMSI).Return(true)
	mockTestVP.EXPECT().GetTestCryptoParams().Return(testKi, testOPc, testAMF).Times(2)
	gomock.InOrder(
		mockRepo.EXPECT().Get(gomock.Any(), testIMSI).Return(&store.Subscriber{IMSI: testIMSI, SQN: "ff9bb4d0b607"}, nil),
		mockRepo.EXPECT().CompareAndSetSQN(gomock.Any(), testIMSI, "ff9bb4d0b607", "ff9bb4d0b627").Return(false, nil),
		mockRepo.EXPECT().Get(gomock.Any(), testIMSI).Return(&store.Subscriber{IMSI: testIMSI, SQN: "ff9bb4d0b627"}, nil),
		mockRepo.EXPECT().CompareAndSetSQN(gomock.Any(), testIMSI, "ff9bb4d0b627", "ff9bb4d0b647").Return(true, nil),
		mockCalc.EXPECT().GenerateVector(testKi, testOPc, testAMF, testDefaultSQN+0x40).Return(dummyVector(), nil),
	)
	mockSQNMgr.EXPECT().ParseHex("ff9bb4d0b607").Return(testDefaultSQN, nil)
	mockSQNMgr.EXPECT().ParseHex("ff9bb4d0b627").Return(testDefaultSQN+0x20, nil)
	mockSQNMgr.EXPECT().Increment(testDefaultSQN).Return(testDefaultSQN+0x20, nil)
	mockSQNMgr.EXPECT().Increment(testDefaultSQN+0x20).Return(testDefaultSQN+0x40, nil)
	mockSQNMgr.EXPECT().FormatHex(testDefaultSQN + 0x20).Return("ff9bb4d0b627")
	mockSQNMgr.EXPECT().FormatHex(testDefaultSQN + 0x40).Return("ff9bb4d0b647")

	if _, err := uc.GenerateVector(context.Background(), &dto.VectorRequest{IMSI: testIMSI}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateVector_Resync_RetryAlreadySynced(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, mockRepo, mockCalc, mockSQNMgr, mockSQNVal, mockResync, mockTestVP := setupUseCase(ctrl)
	logs := captureLogs(t)

	mockTestVP.EXPECT().IsTestIMSI(normalIMSI).Return(false)
	mockResync.EXPECT().ExtractSQN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(uint64(0x40), nil).Times(2)
	gomock.InOrder(
		// 1回目: SQN_MS=0x40 から 0x60 を書こうとしたが、同じ再同期の再送が先に 0x60 を書いていた
		mockRepo.EXPECT().Get(gomock.Any(), normalIMSI).Return(subscriberWithSQN("000000000020"), nil),
		mockSQNVal.EXPECT().ValidateResyncSQN(uint64(0x40), uint64(0x20)).Return(nil),
		mockSQNVal.EXPECT().ComputeResyncSQN(uint64(0x40)).Return(uint64(0x60), nil),
		mockRepo.EXPECT().CompareAndSetSQN(gomock.Any(), normalIMSI, "000000000020", "000000000060").Return(false, nil),
		// 2回目: SQN_HE(0x60) >= SQN_MS(0x40) なので同期済みとみなし、通常どおり +32 する
		mockRepo.EXPECT().Get(gomock.Any(), normalIMSI).Return(subscriberWithSQN("000000000060"), nil),
		mockSQNMgr.EXPECT().Increment(uint64(0x60)).Return(uint64(0x80), nil),
		mockRepo.EXPECT().CompareAndSetSQN(gomock.Any(), normalIMSI, "000000000060", "000000000080").Return(true, nil),
		mockCalc.EXPECT().GenerateVector(gomock.Any(), gomock.Any(), gomock.Any(), uint64(0x80)).Return(dummyVector(), nil),
	)
	mockSQNMgr.EXPECT().ParseHex("000000000020").Return(uint64(0x20), nil)
	mockSQNMgr.EXPECT().ParseHex("000000000060").Return(uint64(0x60), nil)
	mockSQNMgr.EXPECT().FormatHex(uint64(0x60)).Return("000000000060")
	mockSQNMgr.EXPECT().FormatHex(uint64(0x80)).Return("000000000080")

	if _, err := uc.GenerateVector(context.Background(), resyncRequest()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 再同期のログは書き換えに成功した後の1回だけ
	out := logs.String()
	if n := strings.Count(out, `"event_id":"SQN_RESYNC"`); n != 1 {
		t.Errorf("SQN_RESYNC logged %d times, want 1: %s", n, out)
	}
	for _, want := range []string{
		`"msg":"SQN resync already applied by another request"`,
		`"sqn_old":"000000000060"`,
		`"sqn_ms":"000000000040"`,
		`"sqn_new":"000000000080"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("log does not contain %s: %s", want, out)
		}
	}
}

func TestGenerateVector_Resync_RetryNotYetSynced(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, mockRepo, mockCalc, mockSQNMgr, mockSQNVal, mockResync, mockTestVP := setupUseCase(ctrl)
	logs := captureLogs(t)

	mockTestVP.EXPECT().IsTestIMSI(normalIMSI).Return(false)
	mockResync.EXPECT().ExtractSQN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(uint64(0x100), nil).Times(2)
	gomock.InOrder(
		// 1回目: 通常の認証が先に 0x20 → 0x40 に進めていた
		mockRepo.EXPECT().Get(gomock.Any(), normalIMSI).Return(subscriberWithSQN("000000000020"), nil),
		mockSQNVal.EXPECT().ValidateResyncSQN(uint64(0x100), uint64(0x20)).Return(nil),
		mockSQNVal.EXPECT().ComputeResyncSQN(uint64(0x100)).Return(uint64(0x120), nil),
		mockRepo.EXPECT().CompareAndSetSQN(gomock.Any(), normalIMSI, "000000000020", "000000000120").Return(false, nil),
		// 2回目: SQN_HE(0x40) < SQN_MS(0x100) なので、通常の再同期（SQN_MS + 32）
		mockRepo.EXPECT().Get(gomock.Any(), normalIMSI).Return(subscriberWithSQN("000000000040"), nil),
		mockSQNVal.EXPECT().ValidateResyncSQN(uint64(0x100), uint64(0x40)).Return(nil),
		mockSQNVal.EXPECT().ComputeResyncSQN(uint64(0x100)).Return(uint64(0x120), nil),
		mockRepo.EXPECT().CompareAndSetSQN(gomock.Any(), normalIMSI, "000000000040", "000000000120").Return(true, nil),
		mockCalc.EXPECT().GenerateVector(gomock.Any(), gomock.Any(), gomock.Any(), uint64(0x120)).Return(dummyVector(), nil),
	)
	mockSQNMgr.EXPECT().ParseHex("000000000020").Return(uint64(0x20), nil)
	mockSQNMgr.EXPECT().ParseHex("000000000040").Return(uint64(0x40), nil)
	mockSQNMgr.EXPECT().FormatHex(uint64(0x120)).Return("000000000120").Times(2)

	if _, err := uc.GenerateVector(context.Background(), resyncRequest()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := logs.String()
	if n := strings.Count(out, `"event_id":"SQN_RESYNC"`); n != 1 {
		t.Errorf("SQN_RESYNC logged %d times, want 1: %s", n, out)
	}
	if !strings.Contains(out, `"msg":"SQN resync successful"`) || !strings.Contains(out, `"sqn_old":"000000000040"`) {
		t.Errorf("unexpected resync log: %s", out)
	}
}

func TestProcessResync_FirstAttemptKeepsDeltaCheck(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, _, _, _, mockSQNVal, mockResync, _ := setupUseCase(ctrl)

	// 1回目の試行では SQN_HE >= SQN_MS でも同期済みとはみなさず、今までどおり検証する
	mockResync.EXPECT().ExtractSQN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(uint64(0x40), nil)
	mockSQNVal.EXPECT().ValidateResyncSQN(uint64(0x40), uint64(0x60)).Return(errors.New("SQN_MS must be greater than SQN_HE"))

	ki, _ := milenage.HexDecode(validHexKi)
	opc, _ := milenage.HexDecode(validHexOPc)

	_, err := uc.processResync(ki, opc, resyncRequest().ResyncInfo, uint64(0x60), false)
	if !errors.Is(err, ErrResyncDeltaExceeded) {
		t.Fatalf("expected ErrResyncDeltaExceeded, got: %v", err)
	}
}

func TestProcessResync_RetryAlreadySyncedOverflow(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, _, _, mockSQNMgr, _, mockResync, _ := setupUseCase(ctrl)

	mockResync.EXPECT().ExtractSQN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(uint64(0x40), nil)
	mockSQNMgr.EXPECT().Increment(uint64(0xffffffffffe0)).Return(uint64(0), errors.New("overflow"))

	ki, _ := milenage.HexDecode(validHexKi)
	opc, _ := milenage.HexDecode(validHexOPc)

	_, err := uc.processResync(ki, opc, resyncRequest().ResyncInfo, uint64(0xffffffffffe0), true)
	if !errors.Is(err, ErrSQNOverflow) {
		t.Fatalf("expected ErrSQNOverflow, got: %v", err)
	}
}

func TestWaitRandom(t *testing.T) {
	start := time.Now()
	if err := waitRandom(t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed := time.Since(start); elapsed < time.Millisecond {
		t.Errorf("waited %v, want at least 1ms", elapsed)
	}
}

func TestWaitRandom_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if err := waitRandom(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
}

func TestNewVectorUseCase_DefaultWait(t *testing.T) {
	uc := NewVectorUseCase(nil, nil, nil, nil, nil, nil, nil)
	if uc.waitBeforeRetry == nil {
		t.Fatal("waitBeforeRetry must be set")
	}
}
