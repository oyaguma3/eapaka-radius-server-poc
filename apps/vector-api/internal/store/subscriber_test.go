package store

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/config"
)

const testIMSI = "440101234567890"

// newTestStore は miniredis に接続した SubscriberStore を返す。
func newTestStore(t *testing.T) (*SubscriberStore, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)
	host, port, err := net.SplitHostPort(mr.Addr())
	if err != nil {
		t.Fatalf("SplitHostPort failed: %v", err)
	}

	client, err := NewValkeyClient(&config.Config{RedisHost: host, RedisPort: port})
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	return NewSubscriberStore(client), mr
}

// registerSubscriber は sub:{IMSI} に加入者を登録する。
func registerSubscriber(t *testing.T, mr *miniredis.Miniredis, sqn string) {
	t.Helper()

	mr.HSet("sub:"+testIMSI,
		"ki", "465b5ce8b199b49faa5f0a2ee238a6bc",
		"opc", "cd63cb71954a9f4e48a5994e37a02baf",
		"amf", "8000",
		"sqn", sqn,
		"created_at", "2026-10-04T00:00:00Z",
	)
}

func TestNewValkeyClient_ConnectionError(t *testing.T) {
	mr := miniredis.RunT(t)
	host, port, err := net.SplitHostPort(mr.Addr())
	if err != nil {
		t.Fatalf("SplitHostPort failed: %v", err)
	}
	mr.Close()

	if _, err := NewValkeyClient(&config.Config{RedisHost: host, RedisPort: port}); err == nil {
		t.Fatal("expected connection error")
	}
}

func TestValkeyClient_Client(t *testing.T) {
	s, _ := newTestStore(t)

	if s.client.Client() == nil {
		t.Fatal("Client() returned nil")
	}
}

func TestSubscriberStore_Get(t *testing.T) {
	s, mr := newTestStore(t)
	registerSubscriber(t, mr, "000000000020")

	sub, err := s.Get(t.Context(), testIMSI)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if sub == nil {
		t.Fatal("expected subscriber, got nil")
	}

	want := Subscriber{
		IMSI: testIMSI,
		Ki:   "465b5ce8b199b49faa5f0a2ee238a6bc",
		OPc:  "cd63cb71954a9f4e48a5994e37a02baf",
		AMF:  "8000",
		SQN:  "000000000020",
	}
	if *sub != want {
		t.Errorf("Get = %+v, want %+v", *sub, want)
	}
}

func TestSubscriberStore_Get_NotFound(t *testing.T) {
	s, _ := newTestStore(t)

	sub, err := s.Get(t.Context(), testIMSI)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if sub != nil {
		t.Errorf("expected nil, got %+v", sub)
	}
}

func TestSubscriberStore_Get_ConnectionError(t *testing.T) {
	s, mr := newTestStore(t)
	mr.Close()

	// go-redis のリトライを待たないよう、短い期限を付ける
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	if _, err := s.Get(ctx, testIMSI); err == nil {
		t.Fatal("expected error")
	}
}

func TestSubscriberStore_CompareAndSetSQN(t *testing.T) {
	tests := []struct {
		name    string
		stored  string // 登録済みの sqn（空なら加入者を登録しない）
		oldSQN  string
		newSQN  string
		want    bool
		wantSQN string // 実行後の sqn（空なら加入者が存在しないこと）
	}{
		{
			name:    "一致すれば更新する",
			stored:  "000000000020",
			oldSQN:  "000000000020",
			newSQN:  "000000000040",
			want:    true,
			wantSQN: "000000000040",
		},
		{
			name:    "一致しなければ更新しない",
			stored:  "000000000040",
			oldSQN:  "000000000020",
			newSQN:  "000000000040",
			want:    false,
			wantSQN: "000000000040",
		},
		{
			// Admin TUI は大文字で保存する。読んだ値をそのまま比較する
			name:    "大文字のSQNも読んだ値どおりなら一致する",
			stored:  "00000000002A",
			oldSQN:  "00000000002A",
			newSQN:  "00000000004a",
			want:    true,
			wantSQN: "00000000004a",
		},
		{
			name:    "大文字小文字が違えば一致しない",
			stored:  "00000000002A",
			oldSQN:  "00000000002a",
			newSQN:  "00000000004a",
			want:    false,
			wantSQN: "00000000002A",
		},
		{
			name:   "加入者が存在しなければ更新せずキーも作らない",
			oldSQN: "000000000020",
			newSQN: "000000000040",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, mr := newTestStore(t)
			if tt.stored != "" {
				registerSubscriber(t, mr, tt.stored)
			}

			got, err := s.CompareAndSetSQN(t.Context(), testIMSI, tt.oldSQN, tt.newSQN)
			if err != nil {
				t.Fatalf("CompareAndSetSQN failed: %v", err)
			}
			if got != tt.want {
				t.Errorf("CompareAndSetSQN = %v, want %v", got, tt.want)
			}

			key := "sub:" + testIMSI
			if tt.wantSQN == "" {
				if mr.Exists(key) {
					t.Errorf("key %s must not be created", key)
				}
				return
			}
			if sqn := mr.HGet(key, "sqn"); sqn != tt.wantSQN {
				t.Errorf("sqn = %q, want %q", sqn, tt.wantSQN)
			}
			// sqn 以外のフィールドは変わらない
			if ki := mr.HGet(key, "ki"); ki != "465b5ce8b199b49faa5f0a2ee238a6bc" {
				t.Errorf("ki changed: %q", ki)
			}
			if createdAt := mr.HGet(key, "created_at"); createdAt != "2026-10-04T00:00:00Z" {
				t.Errorf("created_at changed: %q", createdAt)
			}
		})
	}
}

func TestSubscriberStore_CompareAndSetSQN_SQNFieldMissing(t *testing.T) {
	s, mr := newTestStore(t)
	key := "sub:" + testIMSI
	mr.HSet(key, "ki", "465b5ce8b199b49faa5f0a2ee238a6bc")

	got, err := s.CompareAndSetSQN(t.Context(), testIMSI, "", "000000000040")
	if err != nil {
		t.Fatalf("CompareAndSetSQN failed: %v", err)
	}
	if got {
		t.Error("expected false when sqn field is missing")
	}
	if sqn := mr.HGet(key, "sqn"); sqn != "" {
		t.Errorf("sqn must not be set, got %q", sqn)
	}
}

func TestSubscriberStore_CompareAndSetSQN_ConnectionError(t *testing.T) {
	s, mr := newTestStore(t)
	mr.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	if _, err := s.CompareAndSetSQN(ctx, testIMSI, "000000000020", "000000000040"); err == nil {
		t.Fatal("expected error")
	}
}
