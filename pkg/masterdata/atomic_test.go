package masterdata

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
	"github.com/redis/go-redis/v9"
)

// creator はテスト対象の作成操作（n 番目の値で作成する）と、保存された値の取り出しをまとめたもの。
type creator struct {
	name      string
	key       string
	errExists error
	create    func(ctx context.Context, client *redis.Client, n int) error
	stored    func(mr *miniredis.Miniredis) string // 何番目の値で作成されたかを判別できるフィールド
}

func creators() []creator {
	return []creator{
		{
			name: "subscriber", key: SubscriberKey("001010000000001"), errExists: ErrSubscriberExists,
			create: func(ctx context.Context, client *redis.Client, n int) error {
				return NewSubscriberStore(client).Create(ctx, &model.Subscriber{
					IMSI: "001010000000001", Ki: fmt.Sprintf("%032d", n), OPc: "CD63CB71954A9F4E48A5994E37A02BAF", AMF: "8000", SQN: "000000000000",
				})
			},
			stored: func(mr *miniredis.Miniredis) string { return mr.HGet(SubscriberKey("001010000000001"), "ki") },
		},
		{
			name: "client", key: ClientKey("192.168.10.1"), errExists: ErrClientExists,
			create: func(ctx context.Context, client *redis.Client, n int) error {
				return NewClientStore(client).Create(ctx, &model.RadiusClient{
					IP: "192.168.10.1", Secret: fmt.Sprintf("secret-%d", n), Name: "AP-01", Vendor: "generic",
				})
			},
			stored: func(mr *miniredis.Miniredis) string { return mr.HGet(ClientKey("192.168.10.1"), "secret") },
		},
		{
			name: "policy", key: PolicyKey("001010000000001"), errExists: ErrPolicyExists,
			create: func(ctx context.Context, client *redis.Client, n int) error {
				return NewPolicyStore(client).Create(ctx, &model.Policy{
					IMSI: "001010000000001", Default: "deny",
					Rules: []model.PolicyRule{{NasID: fmt.Sprintf("ap-%d", n), AllowedSSIDs: []string{"*"}}},
				})
			},
			stored: func(mr *miniredis.Miniredis) string { return mr.HGet(PolicyKey("001010000000001"), "rules") },
		},
	}
}

// TestCreate_ExistingIsNotOverwritten は、既に存在するキーの作成がエラーになり、既存の値を上書きしないことを確認する。
func TestCreate_ExistingIsNotOverwritten(t *testing.T) {
	for _, c := range creators() {
		t.Run(c.name, func(t *testing.T) {
			mr, client := newTestRedis(t)
			defer client.Close()
			ctx := context.Background()

			if err := c.create(ctx, client, 1); err != nil {
				t.Fatalf("first Create() error = %v", err)
			}
			first := c.stored(mr)

			if err := c.create(ctx, client, 2); !errors.Is(err, c.errExists) {
				t.Fatalf("second Create() error = %v, want %v", err, c.errExists)
			}
			if got := c.stored(mr); got != first {
				t.Errorf("stored value = %q, want %q (must not be overwritten)", got, first)
			}
		})
	}
}

// TestCreate_Concurrent は、同じキーを同時に作成しても成功するのは1つだけで、保存された値がその1つのものであることを確認する。
func TestCreate_Concurrent(t *testing.T) {
	const workers = 20
	for _, c := range creators() {
		t.Run(c.name, func(t *testing.T) {
			mr, client := newTestRedis(t)
			defer client.Close()
			ctx := context.Background()

			var wg sync.WaitGroup
			errs := make([]error, workers)
			for i := range workers {
				wg.Add(1)
				go func() {
					defer wg.Done()
					errs[i] = c.create(ctx, client, i)
				}()
			}
			wg.Wait()

			winner := -1
			for i, err := range errs {
				switch {
				case err == nil:
					if winner >= 0 {
						t.Fatalf("Create() succeeded twice (%d and %d)", winner, i)
					}
					winner = i
				case !errors.Is(err, c.errExists):
					t.Fatalf("Create() #%d error = %v, want %v", i, err, c.errExists)
				}
			}
			if winner < 0 {
				t.Fatal("no Create() succeeded")
			}

			// 勝った作成の値だけが保存されている
			want := ""
			switch c.name {
			case "subscriber":
				want = fmt.Sprintf("%032d", winner)
			case "client":
				want = fmt.Sprintf("secret-%d", winner)
			case "policy":
				want = fmt.Sprintf(`[{"nas_id":"ap-%d","allowed_ssids":["*"]}]`, winner)
			}
			if got := c.stored(mr); got != want {
				t.Errorf("stored value = %q, want %q", got, want)
			}
		})
	}
}

// TestUpdate_MissingDoesNotCreate は、存在しないキーの変更がエラーになり、キーを作らないことを確認する。
func TestUpdate_MissingDoesNotCreate(t *testing.T) {
	amf := "B9B9"
	tests := []struct {
		name    string
		key     string
		wantErr error
		update  func(ctx context.Context, client *redis.Client) error
	}{
		{"client update", ClientKey("192.168.10.1"), ErrClientNotFound, func(ctx context.Context, client *redis.Client) error {
			return NewClientStore(client).Update(ctx, &model.RadiusClient{IP: "192.168.10.1", Secret: "s", Name: "n"})
		}},
		{"policy update", PolicyKey("001010000000001"), ErrPolicyNotFound, func(ctx context.Context, client *redis.Client) error {
			return NewPolicyStore(client).Update(ctx, &model.Policy{IMSI: "001010000000001", Default: "allow"})
		}},
		{"subscriber patch", SubscriberKey("001010000000001"), ErrSubscriberNotFound, func(ctx context.Context, client *redis.Client) error {
			return NewSubscriberStore(client).Patch(ctx, "001010000000001", &SubscriberPatch{AMF: &amf})
		}},
		{"subscriber empty patch", SubscriberKey("001010000000001"), ErrSubscriberNotFound, func(ctx context.Context, client *redis.Client) error {
			return NewSubscriberStore(client).Patch(ctx, "001010000000001", &SubscriberPatch{})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mr, client := newTestRedis(t)
			defer client.Close()

			if err := tt.update(context.Background(), client); !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if mr.Exists(tt.key) {
				t.Errorf("%s was created", tt.key)
			}
		})
	}
}

// TestSubscriberStore_Patch は、指定した項目だけを書き換え、SQN を指定しなければ SQN に触れないことを確認する。
func TestSubscriberStore_Patch(t *testing.T) {
	str := func(s string) *string { return &s }
	base := map[string]string{
		"ki": "465B5CE8B199B49FAA5F0A2EE238A6BC", "opc": "CD63CB71954A9F4E48A5994E37A02BAF",
		"amf": "8000", "sqn": "ff9bb4d0b627", "created_at": "2026-10-07T00:00:00Z",
	}
	tests := []struct {
		name  string
		patch SubscriberPatch
		want  map[string]string // base から変わるフィールド
	}{
		{"amf only keeps sqn", SubscriberPatch{AMF: str("B9B9")}, map[string]string{"amf": "B9B9"}},
		{"keys only keeps sqn", SubscriberPatch{Ki: str("00112233445566778899AABBCCDDEEFF"), OPc: str("FFEEDDCCBBAA99887766554433221100")},
			map[string]string{"ki": "00112233445566778899AABBCCDDEEFF", "opc": "FFEEDDCCBBAA99887766554433221100"}},
		{"sqn is written as is", SubscriberPatch{SQN: str("000000000000")}, map[string]string{"sqn": "000000000000"}},
		{"empty patch changes nothing", SubscriberPatch{}, map[string]string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mr, client := newTestRedis(t)
			defer client.Close()
			key := SubscriberKey("001010000000001")
			for f, v := range base {
				mr.HSet(key, f, v)
			}

			if err := NewSubscriberStore(client).Patch(context.Background(), "001010000000001", &tt.patch); err != nil {
				t.Fatalf("Patch() error = %v", err)
			}
			for f, v := range base {
				want := v
				if w, ok := tt.want[f]; ok {
					want = w
				}
				if got := mr.HGet(key, f); got != want {
					t.Errorf("%s = %q, want %q", f, got, want)
				}
			}
			if tt.patch.IsEmpty() != (len(tt.want) == 0) {
				t.Errorf("IsEmpty() = %v", tt.patch.IsEmpty())
			}
		})
	}
}

// TestPolicyStore_Put は、作成したときだけ created=true を返し、2回目はポリシー全体を置き換えることを確認する。
func TestPolicyStore_Put(t *testing.T) {
	mr, client := newTestRedis(t)
	defer client.Close()
	ps := NewPolicyStore(client)
	ctx := context.Background()

	created, err := ps.Put(ctx, &model.Policy{IMSI: "001010000000001", Default: "deny",
		Rules: []model.PolicyRule{{NasID: "ap-001", AllowedSSIDs: []string{"CORP"}}, {NasID: "*", AllowedSSIDs: []string{"GUEST"}}}})
	if err != nil || !created {
		t.Fatalf("first Put() = %v, %v, want true, nil", created, err)
	}

	created, err = ps.Put(ctx, &model.Policy{IMSI: "001010000000001", Default: "allow", Rules: []model.PolicyRule{}})
	if err != nil || created {
		t.Fatalf("second Put() = %v, %v, want false, nil", created, err)
	}
	key := PolicyKey("001010000000001")
	if got := mr.HGet(key, "default"); got != "allow" {
		t.Errorf("default = %q, want allow", got)
	}
	if got := mr.HGet(key, "rules"); got != "[]" {
		t.Errorf("rules = %q, want []", got)
	}
}

func TestPolicyStore_Count(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()
	ps := NewPolicyStore(client)
	ctx := context.Background()

	if n, err := ps.Count(ctx); err != nil || n != 0 {
		t.Fatalf("Count() = %d, %v, want 0, nil", n, err)
	}
	for _, imsi := range []string{"001010000000001", "001010000000002"} {
		if err := ps.Create(ctx, &model.Policy{IMSI: imsi, Default: "allow"}); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}
	// 加入者のキーは数えない
	if err := NewSubscriberStore(client).Create(ctx, &model.Subscriber{IMSI: "001010000000001"}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if n, err := ps.Count(ctx); err != nil || n != 2 {
		t.Errorf("Count() = %d, %v, want 2, nil", n, err)
	}
}
