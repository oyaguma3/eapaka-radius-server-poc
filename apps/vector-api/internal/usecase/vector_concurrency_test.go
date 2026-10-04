package usecase

import (
	"context"
	"errors"
	"net"
	"slices"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/config"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/dto"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/milenage"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/sqn"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/store"
	wmilenage "github.com/wmnsk/milenage"
)

// recordingCalculator は実際の計算を行い、ベクターを生成したSQNを記録する。
// ベクターの生成はSQNの書き換えに成功した後だけ行うので、記録されたSQNが発行したSQNになる。
type recordingCalculator struct {
	calc *milenage.Calculator
	mu   sync.Mutex
	sqns []uint64
}

func (r *recordingCalculator) GenerateVector(ki, opc, amf []byte, sqn uint64) (*milenage.Vector, error) {
	r.mu.Lock()
	r.sqns = append(r.sqns, sqn)
	r.mu.Unlock()
	return r.calc.GenerateVector(ki, opc, amf, sqn)
}

// newConcurrencyUseCase は miniredis と実物の store / sqn / milenage で VectorUseCase を組み立て、
// 初期SQNを持つ加入者を登録する。
func newConcurrencyUseCase(t *testing.T, initialSQN string) (*VectorUseCase, *recordingCalculator, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)
	host, port, err := net.SplitHostPort(mr.Addr())
	if err != nil {
		t.Fatalf("SplitHostPort failed: %v", err)
	}
	client, err := store.NewValkeyClient(&config.Config{RedisHost: host, RedisPort: port})
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	mr.HSet("sub:"+normalIMSI,
		"ki", validHexKi,
		"opc", validHexOPc,
		"amf", validHexAMF,
		"sqn", initialSQN,
	)

	calc := &recordingCalculator{calc: milenage.NewCalculator()}
	uc := NewVectorUseCase(
		store.NewSubscriberStore(client),
		calc,
		sqn.NewManager(),
		sqn.NewValidator(),
		milenage.NewResyncProcessor(),
		nil,
		&config.Config{LogMaskIMSI: true},
	)
	return uc, calc, mr
}

// runConcurrently は reqs を同時に GenerateVector に渡し、それぞれの結果のエラーを返す。
func runConcurrently(t *testing.T, uc *VectorUseCase, reqs []*dto.VectorRequest) []error {
	t.Helper()

	errs := make([]error, len(reqs))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, req := range reqs {
		wg.Go(func() {
			<-start
			_, errs[i] = uc.GenerateVector(context.Background(), req)
		})
	}
	close(start)
	wg.Wait()
	return errs
}

// assertUniqueSQNs は発行したSQNがすべて異なることを確かめる。
func assertUniqueSQNs(t *testing.T, sqns []uint64) {
	t.Helper()

	sorted := slices.Sorted(slices.Values(sqns))
	if len(slices.Compact(slices.Clone(sorted))) != len(sorted) {
		t.Errorf("duplicate SQNs issued: %x", sorted)
	}
}

func TestGenerateVector_Concurrent_NormalRequests(t *testing.T) {
	const n = 20
	const initialSQN = 0x20

	uc, calc, mr := newConcurrencyUseCase(t, "000000000020")

	reqs := make([]*dto.VectorRequest, n)
	for i := range reqs {
		reqs[i] = &dto.VectorRequest{IMSI: normalIMSI}
	}
	errs := runConcurrently(t, uc, reqs)

	succeeded := 0
	for _, err := range errs {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrSQNConflict):
			// 競合がやり直しの上限を超えたもの（409）は許容する
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if succeeded == 0 {
		t.Fatal("no request succeeded")
	}
	if len(calc.sqns) != succeeded {
		t.Fatalf("vectors generated %d, succeeded %d", len(calc.sqns), succeeded)
	}

	// 発行したSQNはすべて異なり、初期値から +32 ずつ隙間なく進む
	assertUniqueSQNs(t, calc.sqns)
	sorted := slices.Sorted(slices.Values(calc.sqns))
	for i, got := range sorted {
		if want := uint64(initialSQN + 0x20*(i+1)); got != want {
			t.Fatalf("issued SQNs = %x, want consecutive from %x", sorted, initialSQN+0x20)
		}
	}

	// 最後に保存されたSQNは、最後に発行したSQN
	want := (&sqn.Manager{}).FormatHex(sorted[len(sorted)-1])
	if got := mr.HGet("sub:"+normalIMSI, "sqn"); got != want {
		t.Errorf("stored sqn = %s, want %s", got, want)
	}
}

func TestGenerateVector_Concurrent_NormalAndResync(t *testing.T) {
	const normalRequests = 10
	const resyncRequests = 5
	const sqnMS = 0x1000 // 端末のSQN（ネットワーク側の初期値 0x20 より進んでいる）

	uc, calc, mr := newConcurrencyUseCase(t, "000000000020")

	// 端末が返す AUTS（同じ再同期要求の再送を想定して、同じ RAND / AUTS を使う）
	ki, _ := milenage.HexDecode(validHexKi)
	opc, _ := milenage.HexDecode(validHexOPc)
	randVal, _ := milenage.HexDecode("0102030405060708090a0b0c0d0e0f10")
	auts, err := wmilenage.NewWithOPc(ki, opc, randVal, sqnMS, 0).GenerateAUTS()
	if err != nil {
		t.Fatalf("GenerateAUTS failed: %v", err)
	}
	resyncInfo := &dto.ResyncInfo{RAND: milenage.HexEncode(randVal), AUTS: milenage.HexEncode(auts)}

	var reqs []*dto.VectorRequest
	for range normalRequests {
		reqs = append(reqs, &dto.VectorRequest{IMSI: normalIMSI})
	}
	for range resyncRequests {
		reqs = append(reqs, &dto.VectorRequest{IMSI: normalIMSI, ResyncInfo: resyncInfo})
	}
	errs := runConcurrently(t, uc, reqs)

	for i, err := range errs {
		switch {
		case err == nil, errors.Is(err, ErrSQNConflict):
		case errors.Is(err, ErrResyncDeltaExceeded) && reqs[i].ResyncInfo != nil:
			// 最初の試行で、すでに SQN_MS 以上に進んでいた再同期要求は今までどおり 400
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if len(calc.sqns) == 0 {
		t.Fatal("no request succeeded")
	}

	assertUniqueSQNs(t, calc.sqns)

	// SQNは巻き戻らない: 最後に保存されたSQNは、発行したSQNの最大値
	maxIssued := slices.Max(calc.sqns)
	want := (&sqn.Manager{}).FormatHex(maxIssued)
	if got := mr.HGet("sub:"+normalIMSI, "sqn"); got != want {
		t.Errorf("stored sqn = %s, want %s (max issued)", got, want)
	}
}
