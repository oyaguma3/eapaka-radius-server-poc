package usecase

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/config"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/dto"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/milenage"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/store"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/logging"
	"go.uber.org/mock/gomock"
)

// validHexKi は有効なKi（16バイト = 32 hex文字）。
const validHexKi = "00112233445566778899aabbccddeeff"

// validHexOPc は有効なOPc（16バイト = 32 hex文字）。
const validHexOPc = "aabbccddeeff00112233445566778899"

// validHexAMF は有効なAMF（2バイト = 4 hex文字）。
const validHexAMF = "8000"

// validHexSQN は有効なSQN（6バイト = 12 hex文字）。
const validHexSQN = "000000000020"

// testIMSI はテスト用IMSI。
const testIMSI = "001010000000001"

// normalIMSI は通常フロー用IMSI。
const normalIMSI = "440101234567890"

// resyncRequest は再同期情報付きのリクエストを返すヘルパー。
func resyncRequest() *dto.VectorRequest {
	return &dto.VectorRequest{
		IMSI: normalIMSI,
		ResyncInfo: &dto.ResyncInfo{
			RAND: "0102030405060708090a0b0c0d0e0f10",
			AUTS: "0102030405060708090a0b0c0d0e",
		},
	}
}

// dummyVector はテスト用固定ベクターを返すヘルパー。
func dummyVector() *milenage.Vector {
	return &milenage.Vector{
		RAND: make([]byte, 16),
		AUTN: make([]byte, 16),
		XRES: make([]byte, 8),
		CK:   make([]byte, 16),
		IK:   make([]byte, 16),
	}
}

// validSubscriber は有効な加入者情報を返すヘルパー。
func validSubscriber() *store.Subscriber {
	return &store.Subscriber{
		IMSI: normalIMSI,
		Ki:   validHexKi,
		OPc:  validHexOPc,
		AMF:  validHexAMF,
		SQN:  validHexSQN,
	}
}

// setupUseCase はテスト用のVectorUseCaseとモック群をセットアップする。
func setupUseCase(ctrl *gomock.Controller) (
	*VectorUseCase,
	*MockSubscriberRepository,
	*MockMilenageCalculator,
	*MockSQNManager,
	*MockSQNValidator,
	*MockResyncProcessor,
	*MockTestVectorProvider,
) {
	mockRepo := NewMockSubscriberRepository(ctrl)
	mockCalc := NewMockMilenageCalculator(ctrl)
	mockSQNMgr := NewMockSQNManager(ctrl)
	mockSQNVal := NewMockSQNValidator(ctrl)
	mockResync := NewMockResyncProcessor(ctrl)
	mockTestVP := NewMockTestVectorProvider(ctrl)

	cfg := &config.Config{}
	uc := NewVectorUseCase(mockRepo, mockCalc, mockSQNMgr, mockSQNVal, mockResync, mockTestVP, cfg)
	uc.waitBeforeRetry = func(context.Context) error { return nil }

	return uc, mockRepo, mockCalc, mockSQNMgr, mockSQNVal, mockResync, mockTestVP
}

// テスト用の固定暗号パラメータ
var (
	testKi  = []byte{0x46, 0x5b, 0x5c, 0xe8, 0xb1, 0x99, 0xb4, 0x9f, 0xaa, 0x5f, 0x0a, 0x2e, 0xe2, 0x38, 0xa6, 0xbc}
	testOPc = []byte{0xcd, 0x63, 0xcb, 0x71, 0x95, 0x4a, 0x9f, 0x4e, 0x48, 0xa5, 0x99, 0x4e, 0x37, 0xa0, 0x2b, 0xaf}
	testAMF = []byte{0xb9, 0xb9}
)

const testDefaultSQN uint64 = 0xff9bb4d0b607

// --- TestGenerateVector_TestMode ---

func TestGenerateVector_TestMode(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, mockRepo, mockCalc, mockSQNMgr, _, _, mockTestVP := setupUseCase(ctrl)

	// テストモードでValkey上に加入者あり
	mockTestVP.EXPECT().IsTestIMSI(testIMSI).Return(true)
	mockTestVP.EXPECT().GetTestCryptoParams().Return(testKi, testOPc, testAMF)
	mockRepo.EXPECT().Get(gomock.Any(), testIMSI).Return(&store.Subscriber{
		IMSI: testIMSI,
		SQN:  "ff9bb4d0b607",
	}, nil)
	mockSQNMgr.EXPECT().ParseHex("ff9bb4d0b607").Return(testDefaultSQN, nil)
	mockSQNMgr.EXPECT().Increment(testDefaultSQN).Return(testDefaultSQN+0x20, nil)
	mockCalc.EXPECT().GenerateVector(testKi, testOPc, testAMF, testDefaultSQN+0x20).Return(dummyVector(), nil)
	mockSQNMgr.EXPECT().FormatHex(testDefaultSQN + 0x20).Return("ff9bb4d0b627")
	mockRepo.EXPECT().CompareAndSetSQN(gomock.Any(), testIMSI, "ff9bb4d0b607", "ff9bb4d0b627").Return(true, nil)

	req := &dto.VectorRequest{IMSI: testIMSI}
	resp, err := uc.GenerateVector(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestGenerateVector_TestMode_SubscriberNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, mockRepo, _, _, _, _, mockTestVP := setupUseCase(ctrl)

	// テストモードでも加入者登録は必須 → 未登録なら404相当のエラー
	mockTestVP.EXPECT().IsTestIMSI(testIMSI).Return(true)
	mockRepo.EXPECT().Get(gomock.Any(), testIMSI).Return(nil, nil)

	_, err := uc.GenerateVector(context.Background(), &dto.VectorRequest{IMSI: testIMSI})
	if !errors.Is(err, ErrSubscriberNotFound) {
		t.Errorf("expected ErrSubscriberNotFound, got: %v", err)
	}
}

func TestGenerateVector_TestMode_ValkeyError(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, mockRepo, _, _, _, _, mockTestVP := setupUseCase(ctrl)

	// Valkeyエラーは通常モードと同じく接続エラー（500相当）
	mockTestVP.EXPECT().IsTestIMSI(testIMSI).Return(true)
	mockRepo.EXPECT().Get(gomock.Any(), testIMSI).Return(nil, errors.New("connection refused"))

	_, err := uc.GenerateVector(context.Background(), &dto.VectorRequest{IMSI: testIMSI})
	if !errors.Is(err, ErrValkeyConnection) {
		t.Errorf("expected ErrValkeyConnection, got: %v", err)
	}
}

func TestGenerateVector_TestMode_Resync(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, mockRepo, mockCalc, mockSQNMgr, mockSQNVal, mockResync, mockTestVP := setupUseCase(ctrl)

	// テストモードでResyncInfo付きリクエスト
	mockTestVP.EXPECT().IsTestIMSI(testIMSI).Return(true)
	mockTestVP.EXPECT().GetTestCryptoParams().Return(testKi, testOPc, testAMF)
	mockRepo.EXPECT().Get(gomock.Any(), testIMSI).Return(&store.Subscriber{
		IMSI: testIMSI,
		SQN:  "ff9bb4d0b607",
	}, nil)
	mockSQNMgr.EXPECT().ParseHex("ff9bb4d0b607").Return(testDefaultSQN, nil)

	// 再同期フロー
	mockResync.EXPECT().ExtractSQN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(uint64(0x10), nil)
	mockSQNVal.EXPECT().ValidateResyncSQN(uint64(0x10), testDefaultSQN).Return(nil)
	mockSQNVal.EXPECT().ComputeResyncSQN(uint64(0x10)).Return(uint64(0x30), nil)

	mockCalc.EXPECT().GenerateVector(testKi, testOPc, testAMF, uint64(0x30)).Return(dummyVector(), nil)
	mockSQNMgr.EXPECT().FormatHex(uint64(0x30)).Return("000000000030")
	mockRepo.EXPECT().CompareAndSetSQN(gomock.Any(), testIMSI, "ff9bb4d0b607", "000000000030").Return(true, nil)

	req := &dto.VectorRequest{
		IMSI: testIMSI,
		ResyncInfo: &dto.ResyncInfo{
			RAND: "0102030405060708090a0b0c0d0e0f10",
			AUTS: "0102030405060708090a0b0c0d0e",
		},
	}
	resp, err := uc.GenerateVector(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestGenerateVector_TestMode_CalcError(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, mockRepo, mockCalc, mockSQNMgr, _, _, mockTestVP := setupUseCase(ctrl)

	// ベクター生成失敗
	mockTestVP.EXPECT().IsTestIMSI(testIMSI).Return(true)
	mockTestVP.EXPECT().GetTestCryptoParams().Return(testKi, testOPc, testAMF)
	mockRepo.EXPECT().Get(gomock.Any(), testIMSI).Return(&store.Subscriber{IMSI: testIMSI, SQN: "ff9bb4d0b607"}, nil)
	mockSQNMgr.EXPECT().ParseHex("ff9bb4d0b607").Return(testDefaultSQN, nil)
	mockSQNMgr.EXPECT().Increment(testDefaultSQN).Return(testDefaultSQN+0x20, nil)
	mockSQNMgr.EXPECT().FormatHex(testDefaultSQN + 0x20).Return("ff9bb4d0b627")
	mockRepo.EXPECT().CompareAndSetSQN(gomock.Any(), testIMSI, "ff9bb4d0b607", "ff9bb4d0b627").Return(true, nil)
	mockCalc.EXPECT().GenerateVector(testKi, testOPc, testAMF, testDefaultSQN+0x20).
		Return(nil, errors.New("calculation failed"))

	_, err := uc.GenerateVector(context.Background(), &dto.VectorRequest{IMSI: testIMSI})
	if !errors.Is(err, ErrMilenageCalculation) {
		t.Errorf("expected ErrMilenageCalculation, got: %v", err)
	}
}

func TestGenerateVector_TestMode_SQNParseError(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, mockRepo, _, mockSQNMgr, _, _, mockTestVP := setupUseCase(ctrl)

	// SQNパースエラーは通常モードと同じくエラー（フォールバックしない）
	mockTestVP.EXPECT().IsTestIMSI(testIMSI).Return(true)
	mockTestVP.EXPECT().GetTestCryptoParams().Return(testKi, testOPc, testAMF)
	mockRepo.EXPECT().Get(gomock.Any(), testIMSI).Return(&store.Subscriber{IMSI: testIMSI, SQN: "INVALID_HEX"}, nil)
	mockSQNMgr.EXPECT().ParseHex("INVALID_HEX").Return(uint64(0), errors.New("parse error"))

	if _, err := uc.GenerateVector(context.Background(), &dto.VectorRequest{IMSI: testIMSI}); err == nil {
		t.Fatal("expected error for invalid SQN")
	}
}

func TestGenerateVector_TestMode_SQNPersistError(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, mockRepo, _, mockSQNMgr, _, _, mockTestVP := setupUseCase(ctrl)

	// SQN書き戻し失敗は通常モードと同じく接続エラー（ベクターは返さない）
	mockTestVP.EXPECT().IsTestIMSI(testIMSI).Return(true)
	mockTestVP.EXPECT().GetTestCryptoParams().Return(testKi, testOPc, testAMF)
	mockRepo.EXPECT().Get(gomock.Any(), testIMSI).Return(&store.Subscriber{IMSI: testIMSI, SQN: "ff9bb4d0b607"}, nil)
	mockSQNMgr.EXPECT().ParseHex("ff9bb4d0b607").Return(testDefaultSQN, nil)
	mockSQNMgr.EXPECT().Increment(testDefaultSQN).Return(testDefaultSQN+0x20, nil)
	mockSQNMgr.EXPECT().FormatHex(testDefaultSQN + 0x20).Return("ff9bb4d0b627")
	mockRepo.EXPECT().CompareAndSetSQN(gomock.Any(), testIMSI, "ff9bb4d0b607", "ff9bb4d0b627").Return(false, errors.New("persist failed"))

	resp, err := uc.GenerateVector(context.Background(), &dto.VectorRequest{IMSI: testIMSI})
	if !errors.Is(err, ErrValkeyConnection) {
		t.Errorf("expected ErrValkeyConnection, got: %v", err)
	}
	if resp != nil {
		t.Error("expected nil response when SQN persist fails")
	}
}

func TestGenerateVector_TestMode_IncrementOverflow(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, mockRepo, _, mockSQNMgr, _, _, mockTestVP := setupUseCase(ctrl)

	// テストモードでSQN Incrementオーバーフロー
	mockTestVP.EXPECT().IsTestIMSI(testIMSI).Return(true)
	mockTestVP.EXPECT().GetTestCryptoParams().Return(testKi, testOPc, testAMF)
	mockRepo.EXPECT().Get(gomock.Any(), testIMSI).Return(&store.Subscriber{IMSI: testIMSI, SQN: "ffffffffffe0"}, nil)
	mockSQNMgr.EXPECT().ParseHex("ffffffffffe0").Return(uint64(0xffffffffffe0), nil)
	mockSQNMgr.EXPECT().Increment(uint64(0xffffffffffe0)).Return(uint64(0), errors.New("overflow"))

	_, err := uc.GenerateVector(context.Background(), &dto.VectorRequest{IMSI: testIMSI})
	if !errors.Is(err, ErrSQNOverflow) {
		t.Errorf("expected ErrSQNOverflow, got: %v", err)
	}
}

func TestGenerateVector_TestMode_IgnoresSubscriberKeys(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, mockRepo, mockCalc, mockSQNMgr, _, _, mockTestVP := setupUseCase(ctrl)

	// 加入者データの Ki/OPc/AMF は使わず、テスト用の固定値で計算する（SQN は加入者データを使う）
	sub := validSubscriber()
	sub.IMSI = testIMSI
	mockTestVP.EXPECT().IsTestIMSI(testIMSI).Return(true)
	mockTestVP.EXPECT().GetTestCryptoParams().Return(testKi, testOPc, testAMF)
	mockRepo.EXPECT().Get(gomock.Any(), testIMSI).Return(sub, nil)
	mockSQNMgr.EXPECT().ParseHex(sub.SQN).Return(uint64(0x20), nil)
	mockSQNMgr.EXPECT().Increment(uint64(0x20)).Return(uint64(0x40), nil)
	mockCalc.EXPECT().GenerateVector(testKi, testOPc, testAMF, uint64(0x40)).Return(dummyVector(), nil)
	mockSQNMgr.EXPECT().FormatHex(uint64(0x40)).Return("000000000040")
	mockRepo.EXPECT().CompareAndSetSQN(gomock.Any(), testIMSI, validHexSQN, "000000000040").Return(true, nil)

	if _, err := uc.GenerateVector(context.Background(), &dto.VectorRequest{IMSI: testIMSI}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- TestGenerateVector_Success ---

func TestGenerateVector_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, mockRepo, mockCalc, mockSQNMgr, _, _, mockTestVP := setupUseCase(ctrl)

	mockTestVP.EXPECT().IsTestIMSI(normalIMSI).Return(false)
	mockRepo.EXPECT().Get(gomock.Any(), normalIMSI).Return(validSubscriber(), nil)
	mockSQNMgr.EXPECT().ParseHex(validHexSQN).Return(uint64(0x20), nil)
	mockSQNMgr.EXPECT().Increment(uint64(0x20)).Return(uint64(0x40), nil)
	mockCalc.EXPECT().GenerateVector(gomock.Any(), gomock.Any(), gomock.Any(), uint64(0x40)).Return(dummyVector(), nil)
	mockSQNMgr.EXPECT().FormatHex(uint64(0x40)).Return("000000000040")
	mockRepo.EXPECT().CompareAndSetSQN(gomock.Any(), normalIMSI, validHexSQN, "000000000040").Return(true, nil)

	req := &dto.VectorRequest{IMSI: normalIMSI}
	resp, err := uc.GenerateVector(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

// --- TestGenerateVector_SubscriberGetError ---

func TestGenerateVector_SubscriberGetError(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, mockRepo, _, _, _, _, mockTestVP := setupUseCase(ctrl)

	mockTestVP.EXPECT().IsTestIMSI(normalIMSI).Return(false)
	mockRepo.EXPECT().Get(gomock.Any(), normalIMSI).Return(nil, errors.New("connection refused"))

	req := &dto.VectorRequest{IMSI: normalIMSI}
	_, err := uc.GenerateVector(context.Background(), req)

	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrValkeyConnection) {
		t.Errorf("expected ErrValkeyConnection, got: %v", err)
	}
}

// --- TestGenerateVector_SubscriberNotFound ---

func TestGenerateVector_SubscriberNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, mockRepo, _, _, _, _, mockTestVP := setupUseCase(ctrl)

	mockTestVP.EXPECT().IsTestIMSI(normalIMSI).Return(false)
	mockRepo.EXPECT().Get(gomock.Any(), normalIMSI).Return(nil, nil)

	req := &dto.VectorRequest{IMSI: normalIMSI}
	_, err := uc.GenerateVector(context.Background(), req)

	if !errors.Is(err, ErrSubscriberNotFound) {
		t.Errorf("expected ErrSubscriberNotFound, got: %v", err)
	}
}

// --- TestGenerateVector_InvalidKi ---

func TestGenerateVector_InvalidKi(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, mockRepo, _, _, _, _, mockTestVP := setupUseCase(ctrl)

	sub := validSubscriber()
	sub.Ki = "ZZZZ" // 不正なhex

	mockTestVP.EXPECT().IsTestIMSI(normalIMSI).Return(false)
	mockRepo.EXPECT().Get(gomock.Any(), normalIMSI).Return(sub, nil)

	req := &dto.VectorRequest{IMSI: normalIMSI}
	_, err := uc.GenerateVector(context.Background(), req)

	if err == nil {
		t.Fatal("expected error for invalid Ki")
	}
}

// --- TestGenerateVector_InvalidOPc ---

func TestGenerateVector_InvalidOPc(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, mockRepo, _, _, _, _, mockTestVP := setupUseCase(ctrl)

	sub := validSubscriber()
	sub.OPc = "ZZZZ" // 不正なhex

	mockTestVP.EXPECT().IsTestIMSI(normalIMSI).Return(false)
	mockRepo.EXPECT().Get(gomock.Any(), normalIMSI).Return(sub, nil)

	req := &dto.VectorRequest{IMSI: normalIMSI}
	_, err := uc.GenerateVector(context.Background(), req)

	if err == nil {
		t.Fatal("expected error for invalid OPc")
	}
}

// --- TestGenerateVector_InvalidAMF ---

func TestGenerateVector_InvalidAMF(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, mockRepo, _, _, _, _, mockTestVP := setupUseCase(ctrl)

	sub := validSubscriber()
	sub.AMF = "ZZZZ" // 不正なhex

	mockTestVP.EXPECT().IsTestIMSI(normalIMSI).Return(false)
	mockRepo.EXPECT().Get(gomock.Any(), normalIMSI).Return(sub, nil)

	req := &dto.VectorRequest{IMSI: normalIMSI}
	_, err := uc.GenerateVector(context.Background(), req)

	if err == nil {
		t.Fatal("expected error for invalid AMF")
	}
}

// --- TestGenerateVector_InvalidSQN ---

func TestGenerateVector_InvalidSQN(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, mockRepo, _, mockSQNMgr, _, _, mockTestVP := setupUseCase(ctrl)

	mockTestVP.EXPECT().IsTestIMSI(normalIMSI).Return(false)
	mockRepo.EXPECT().Get(gomock.Any(), normalIMSI).Return(validSubscriber(), nil)
	mockSQNMgr.EXPECT().ParseHex(validHexSQN).Return(uint64(0), errors.New("parse error"))

	req := &dto.VectorRequest{IMSI: normalIMSI}
	_, err := uc.GenerateVector(context.Background(), req)

	if err == nil {
		t.Fatal("expected error for invalid SQN")
	}
}

// --- TestGenerateVector_IncrementOverflow ---

func TestGenerateVector_IncrementOverflow(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, mockRepo, _, mockSQNMgr, _, _, mockTestVP := setupUseCase(ctrl)

	mockTestVP.EXPECT().IsTestIMSI(normalIMSI).Return(false)
	mockRepo.EXPECT().Get(gomock.Any(), normalIMSI).Return(validSubscriber(), nil)
	mockSQNMgr.EXPECT().ParseHex(validHexSQN).Return(uint64(0x20), nil)
	mockSQNMgr.EXPECT().Increment(uint64(0x20)).Return(uint64(0), errors.New("overflow"))

	req := &dto.VectorRequest{IMSI: normalIMSI}
	_, err := uc.GenerateVector(context.Background(), req)

	if !errors.Is(err, ErrSQNOverflow) {
		t.Errorf("expected ErrSQNOverflow, got: %v", err)
	}
}

// --- TestGenerateVector_CalculatorError ---

func TestGenerateVector_CalculatorError(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, mockRepo, mockCalc, mockSQNMgr, _, _, mockTestVP := setupUseCase(ctrl)

	mockTestVP.EXPECT().IsTestIMSI(normalIMSI).Return(false)
	mockRepo.EXPECT().Get(gomock.Any(), normalIMSI).Return(validSubscriber(), nil)
	mockSQNMgr.EXPECT().ParseHex(validHexSQN).Return(uint64(0x20), nil)
	mockSQNMgr.EXPECT().Increment(uint64(0x20)).Return(uint64(0x40), nil)
	mockSQNMgr.EXPECT().FormatHex(uint64(0x40)).Return("000000000040")
	mockRepo.EXPECT().CompareAndSetSQN(gomock.Any(), normalIMSI, validHexSQN, "000000000040").Return(true, nil)
	mockCalc.EXPECT().GenerateVector(gomock.Any(), gomock.Any(), gomock.Any(), uint64(0x40)).
		Return(nil, errors.New("calculation failed"))

	req := &dto.VectorRequest{IMSI: normalIMSI}
	_, err := uc.GenerateVector(context.Background(), req)

	if !errors.Is(err, ErrMilenageCalculation) {
		t.Errorf("expected ErrMilenageCalculation, got: %v", err)
	}
}

// --- TestGenerateVector_CompareAndSetSQNError ---

func TestGenerateVector_CompareAndSetSQNError(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, mockRepo, _, mockSQNMgr, _, _, mockTestVP := setupUseCase(ctrl)

	mockTestVP.EXPECT().IsTestIMSI(normalIMSI).Return(false)
	mockRepo.EXPECT().Get(gomock.Any(), normalIMSI).Return(validSubscriber(), nil)
	mockSQNMgr.EXPECT().ParseHex(validHexSQN).Return(uint64(0x20), nil)
	mockSQNMgr.EXPECT().Increment(uint64(0x20)).Return(uint64(0x40), nil)
	mockSQNMgr.EXPECT().FormatHex(uint64(0x40)).Return("000000000040")
	mockRepo.EXPECT().CompareAndSetSQN(gomock.Any(), normalIMSI, validHexSQN, "000000000040").Return(false, errors.New("update failed"))

	req := &dto.VectorRequest{IMSI: normalIMSI}
	_, err := uc.GenerateVector(context.Background(), req)

	if !errors.Is(err, ErrValkeyConnection) {
		t.Errorf("expected ErrValkeyConnection, got: %v", err)
	}
}

// --- TestProcessResync_Success ---

func TestProcessResync_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, _, _, _, mockSQNVal, mockResync, _ := setupUseCase(ctrl)

	// 有効な16バイトRANDと14バイトAUTS
	validRAND := "0102030405060708090a0b0c0d0e0f10"
	validAUTS := "0102030405060708090a0b0c0d0e" // 14バイト

	resyncInfo := &dto.ResyncInfo{
		RAND: validRAND,
		AUTS: validAUTS,
	}

	mockResync.EXPECT().ExtractSQN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(uint64(0x10), nil)
	mockSQNVal.EXPECT().ValidateResyncSQN(uint64(0x10), uint64(0x20)).Return(nil)
	mockSQNVal.EXPECT().ComputeResyncSQN(uint64(0x10)).Return(uint64(0x30), nil)

	ki, _ := milenage.HexDecode(validHexKi)
	opc, _ := milenage.HexDecode(validHexOPc)

	res, err := uc.processResync(ki, opc, resyncInfo, uint64(0x20), false)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.newSQN != uint64(0x30) || res.sqnMS != uint64(0x10) || res.alreadySynced {
		t.Errorf("unexpected result: %+v", res)
	}
}

// --- TestProcessResync_InvalidRAND ---

func TestProcessResync_InvalidRAND(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, _, _, _, _, _, _ := setupUseCase(ctrl)

	resyncInfo := &dto.ResyncInfo{
		RAND: "ZZZZ",
		AUTS: "0102030405060708090a0b0c0d0e",
	}

	ki, _ := milenage.HexDecode(validHexKi)
	opc, _ := milenage.HexDecode(validHexOPc)

	_, err := uc.processResync(ki, opc, resyncInfo, uint64(0x20), false)

	if !errors.Is(err, ErrResyncInvalidFormat) {
		t.Errorf("expected ErrResyncInvalidFormat, got: %v", err)
	}
}

// --- TestProcessResync_InvalidAUTS ---

func TestProcessResync_InvalidAUTS(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, _, _, _, _, _, _ := setupUseCase(ctrl)

	resyncInfo := &dto.ResyncInfo{
		RAND: "0102030405060708090a0b0c0d0e0f10",
		AUTS: "ZZZZ",
	}

	ki, _ := milenage.HexDecode(validHexKi)
	opc, _ := milenage.HexDecode(validHexOPc)

	_, err := uc.processResync(ki, opc, resyncInfo, uint64(0x20), false)

	if !errors.Is(err, ErrResyncInvalidFormat) {
		t.Errorf("expected ErrResyncInvalidFormat, got: %v", err)
	}
}

// --- TestProcessResync_AUTSLengthError ---

func TestProcessResync_AUTSLengthError(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, _, _, _, _, _, _ := setupUseCase(ctrl)

	// 14バイトではなく10バイトのAUTS
	resyncInfo := &dto.ResyncInfo{
		RAND: "0102030405060708090a0b0c0d0e0f10",
		AUTS: "01020304050607080910",
	}

	ki, _ := milenage.HexDecode(validHexKi)
	opc, _ := milenage.HexDecode(validHexOPc)

	_, err := uc.processResync(ki, opc, resyncInfo, uint64(0x20), false)

	if !errors.Is(err, ErrResyncInvalidFormat) {
		t.Errorf("expected ErrResyncInvalidFormat, got: %v", err)
	}
}

// --- TestProcessResync_ExtractSQNFailed ---

func TestProcessResync_ExtractSQNFailed(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, _, _, _, _, mockResync, _ := setupUseCase(ctrl)

	resyncInfo := &dto.ResyncInfo{
		RAND: "0102030405060708090a0b0c0d0e0f10",
		AUTS: "0102030405060708090a0b0c0d0e",
	}

	mockResync.EXPECT().ExtractSQN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(uint64(0), errors.New("MAC verification failed"))

	ki, _ := milenage.HexDecode(validHexKi)
	opc, _ := milenage.HexDecode(validHexOPc)

	_, err := uc.processResync(ki, opc, resyncInfo, uint64(0x20), false)

	if !errors.Is(err, ErrResyncMACFailed) {
		t.Errorf("expected ErrResyncMACFailed, got: %v", err)
	}
}

// --- TestProcessResync_DeltaExceeded ---

func TestProcessResync_DeltaExceeded(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, _, _, _, mockSQNVal, mockResync, _ := setupUseCase(ctrl)

	resyncInfo := &dto.ResyncInfo{
		RAND: "0102030405060708090a0b0c0d0e0f10",
		AUTS: "0102030405060708090a0b0c0d0e",
	}

	mockResync.EXPECT().ExtractSQN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(uint64(0x10), nil)
	mockSQNVal.EXPECT().ValidateResyncSQN(uint64(0x10), uint64(0x20)).Return(errors.New("delta exceeded"))

	ki, _ := milenage.HexDecode(validHexKi)
	opc, _ := milenage.HexDecode(validHexOPc)

	_, err := uc.processResync(ki, opc, resyncInfo, uint64(0x20), false)

	if !errors.Is(err, ErrResyncDeltaExceeded) {
		t.Errorf("expected ErrResyncDeltaExceeded, got: %v", err)
	}
}

// --- TestProcessResync_ComputeResyncOverflow ---

func TestProcessResync_ComputeResyncOverflow(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, _, _, _, mockSQNVal, mockResync, _ := setupUseCase(ctrl)

	resyncInfo := &dto.ResyncInfo{
		RAND: "0102030405060708090a0b0c0d0e0f10",
		AUTS: "0102030405060708090a0b0c0d0e",
	}

	mockResync.EXPECT().ExtractSQN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(uint64(0x10), nil)
	mockSQNVal.EXPECT().ValidateResyncSQN(uint64(0x10), uint64(0x20)).Return(nil)
	mockSQNVal.EXPECT().ComputeResyncSQN(uint64(0x10)).Return(uint64(0), errors.New("overflow"))

	ki, _ := milenage.HexDecode(validHexKi)
	opc, _ := milenage.HexDecode(validHexOPc)

	_, err := uc.processResync(ki, opc, resyncInfo, uint64(0x20), false)

	if !errors.Is(err, ErrSQNOverflow) {
		t.Errorf("expected ErrSQNOverflow, got: %v", err)
	}
}

// --- TestGenerateVector_Resync_Success ---

func TestGenerateVector_Resync_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, mockRepo, mockCalc, mockSQNMgr, mockSQNVal, mockResync, mockTestVP := setupUseCase(ctrl)

	mockTestVP.EXPECT().IsTestIMSI(normalIMSI).Return(false)
	mockRepo.EXPECT().Get(gomock.Any(), normalIMSI).Return(validSubscriber(), nil)
	mockSQNMgr.EXPECT().ParseHex(validHexSQN).Return(uint64(0x20), nil)

	// 再同期フロー
	mockResync.EXPECT().ExtractSQN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(uint64(0x10), nil)
	mockSQNVal.EXPECT().ValidateResyncSQN(uint64(0x10), uint64(0x20)).Return(nil)
	mockSQNVal.EXPECT().ComputeResyncSQN(uint64(0x10)).Return(uint64(0x30), nil)

	mockCalc.EXPECT().GenerateVector(gomock.Any(), gomock.Any(), gomock.Any(), uint64(0x30)).Return(dummyVector(), nil)
	mockSQNMgr.EXPECT().FormatHex(uint64(0x30)).Return("000000000030")
	mockRepo.EXPECT().CompareAndSetSQN(gomock.Any(), normalIMSI, validHexSQN, "000000000030").Return(true, nil)

	req := &dto.VectorRequest{
		IMSI: normalIMSI,
		ResyncInfo: &dto.ResyncInfo{
			RAND: "0102030405060708090a0b0c0d0e0f10",
			AUTS: "0102030405060708090a0b0c0d0e",
		},
	}
	resp, err := uc.GenerateVector(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestGenerateVector_Resync_LogsTraceIDAndMaskedIMSI(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, mockRepo, mockCalc, mockSQNMgr, mockSQNVal, mockResync, mockTestVP := setupUseCase(ctrl)
	uc.cfg.LogMaskIMSI = true

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	mockTestVP.EXPECT().IsTestIMSI(normalIMSI).Return(false)
	mockRepo.EXPECT().Get(gomock.Any(), normalIMSI).Return(validSubscriber(), nil)
	mockSQNMgr.EXPECT().ParseHex(validHexSQN).Return(uint64(0x20), nil)
	mockResync.EXPECT().ExtractSQN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(uint64(0x40), nil)
	mockSQNVal.EXPECT().ValidateResyncSQN(uint64(0x40), uint64(0x20)).Return(nil)
	mockSQNVal.EXPECT().ComputeResyncSQN(uint64(0x40)).Return(uint64(0x60), nil)
	mockSQNMgr.EXPECT().FormatHex(uint64(0x60)).Return("000000000060")
	mockRepo.EXPECT().CompareAndSetSQN(gomock.Any(), normalIMSI, validHexSQN, "000000000060").Return(true, nil)
	mockCalc.EXPECT().GenerateVector(gomock.Any(), gomock.Any(), gomock.Any(), uint64(0x60)).Return(dummyVector(), nil)

	ctx := ContextWithTraceID(context.Background(), "trace-resync")
	if _, err := uc.GenerateVector(ctx, resyncRequest()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logs := buf.String()
	for _, want := range []string{
		`"msg":"SQN resync successful"`,
		`"event_id":"SQN_RESYNC"`,
		`"trace_id":"trace-resync"`,
		`"imsi":"` + logging.MaskIMSI(normalIMSI, true) + `"`,
		`"sqn_old":"000000000020"`,
		`"sqn_ms":"000000000040"`,
		`"sqn_new":"000000000060"`,
	} {
		if !strings.Contains(logs, want) {
			t.Errorf("log does not contain %s: %s", want, logs)
		}
	}
	if strings.Contains(logs, normalIMSI) {
		t.Errorf("log contains unmasked IMSI: %s", logs)
	}
}

func TestGenerateVector_Resync_NoLogWhenCASFails(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, mockRepo, _, mockSQNMgr, mockSQNVal, mockResync, mockTestVP := setupUseCase(ctrl)

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	// SQNの書き換えに失敗したときは、再同期のログを出さない
	mockTestVP.EXPECT().IsTestIMSI(normalIMSI).Return(false)
	mockRepo.EXPECT().Get(gomock.Any(), normalIMSI).Return(validSubscriber(), nil)
	mockSQNMgr.EXPECT().ParseHex(validHexSQN).Return(uint64(0x20), nil)
	mockResync.EXPECT().ExtractSQN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(uint64(0x40), nil)
	mockSQNVal.EXPECT().ValidateResyncSQN(uint64(0x40), uint64(0x20)).Return(nil)
	mockSQNVal.EXPECT().ComputeResyncSQN(uint64(0x40)).Return(uint64(0x60), nil)
	mockSQNMgr.EXPECT().FormatHex(uint64(0x60)).Return("000000000060")
	mockRepo.EXPECT().CompareAndSetSQN(gomock.Any(), normalIMSI, validHexSQN, "000000000060").Return(false, errors.New("connection refused"))

	_, err := uc.GenerateVector(context.Background(), resyncRequest())
	if !errors.Is(err, ErrValkeyConnection) {
		t.Fatalf("expected ErrValkeyConnection, got: %v", err)
	}
	if strings.Contains(buf.String(), "SQN_RESYNC") {
		t.Errorf("SQN_RESYNC must not be logged: %s", buf.String())
	}
}

func TestProcessResync_DeltaExceeded_NoUseCaseLog(t *testing.T) {
	ctrl := gomock.NewController(t)

	uc, _, _, _, mockSQNVal, mockResync, _ := setupUseCase(ctrl)

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	mockResync.EXPECT().ExtractSQN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(uint64(0x10), nil)
	mockSQNVal.EXPECT().ValidateResyncSQN(uint64(0x10), uint64(0x20)).Return(errors.New("delta too large"))

	ki, _ := milenage.HexDecode(validHexKi)
	opc, _ := milenage.HexDecode(validHexOPc)
	resyncInfo := &dto.ResyncInfo{RAND: "0102030405060708090a0b0c0d0e0f10", AUTS: "0102030405060708090a0b0c0d0e"}

	_, err := uc.processResync(ki, opc, resyncInfo, uint64(0x20), false)
	if !errors.Is(err, ErrResyncDeltaExceeded) {
		t.Fatalf("expected ErrResyncDeltaExceeded, got: %v", err)
	}
	// SQN値はエラー文に含め、ログはハンドラーが1行で出す（ユースケースでは出さない）
	if !strings.Contains(err.Error(), "sqn_ms=000000000010") || !strings.Contains(err.Error(), "sqn_he=000000000020") {
		t.Errorf("error does not contain SQN values: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("use case should not log: %s", buf.String())
	}
}

func TestVectorUseCase_IsTestModeAndMaskIMSI(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, _, _, _, _, _, mockTestVP := setupUseCase(ctrl)

	mockTestVP.EXPECT().IsTestIMSI(testIMSI).Return(true)
	if !uc.IsTestMode(testIMSI) {
		t.Error("IsTestMode() = false, want true")
	}
	if (&VectorUseCase{}).IsTestMode(testIMSI) {
		t.Error("IsTestMode() without provider should be false")
	}

	const imsi = "440101234567890"
	if got := (&VectorUseCase{cfg: &config.Config{LogMaskIMSI: false}}).maskIMSI(imsi); got != imsi {
		t.Errorf("maskIMSI(disabled) = %q", got)
	}
	if got := (&VectorUseCase{}).maskIMSI(imsi); got != "440101********0" {
		t.Errorf("maskIMSI(nil cfg) = %q", got)
	}
}
