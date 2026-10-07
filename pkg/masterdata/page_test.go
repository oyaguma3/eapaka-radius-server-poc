package masterdata

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
)

// pageIMSIs はページの加入者の IMSI を並べて返す。
func pageIMSIs(page *SubscriberPage) []string {
	imsis := make([]string, len(page.Items))
	for i, sub := range page.Items {
		imsis[i] = sub.IMSI
	}
	return imsis
}

func TestSubscriberStore_ListPage(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	ss := NewSubscriberStore(client)
	ctx := context.Background()

	// 登録順と IMSI の順を変えておく
	for _, imsi := range []string{"001010000000003", "440100000000001", "001010000000001", "001010000000002"} {
		sub := &model.Subscriber{IMSI: imsi, Ki: "K", OPc: "O", AMF: "8000", SQN: "000000000000"}
		if err := ss.Create(ctx, sub); err != nil {
			t.Fatalf("Create(%s) error = %v", imsi, err)
		}
	}

	tests := []struct {
		name       string
		prefix     string
		cursor     string
		limit      int
		want       []string
		wantTotal  int
		wantCursor string
	}{
		{"全件", "", "", 50, []string{"001010000000001", "001010000000002", "001010000000003", "440100000000001"}, 4, ""},
		{"1ページ目", "", "", 2, []string{"001010000000001", "001010000000002"}, 4, "001010000000002"},
		{"2ページ目", "", "001010000000002", 2, []string{"001010000000003", "440100000000001"}, 4, ""},
		{"ちょうど最後まで", "", "", 4, []string{"001010000000001", "001010000000002", "001010000000003", "440100000000001"}, 4, ""},
		{"前方一致", "00101", "", 50, []string{"001010000000001", "001010000000002", "001010000000003"}, 3, ""},
		{"前方一致の2ページ目", "00101", "001010000000001", 1, []string{"001010000000002"}, 3, "001010000000002"},
		{"存在しない cursor の次から", "", "001010000000002x", 50, []string{"001010000000003", "440100000000001"}, 4, ""},
		{"cursor が最後", "", "440100000000001", 50, []string{}, 4, ""},
		{"一致なし", "999", "", 50, []string{}, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, err := ss.ListPage(ctx, tt.prefix, tt.cursor, tt.limit)
			if err != nil {
				t.Fatalf("ListPage() error = %v", err)
			}
			if got := pageIMSIs(page); !slices.Equal(got, tt.want) {
				t.Errorf("Items = %v, want %v", got, tt.want)
			}
			if page.Total != tt.wantTotal {
				t.Errorf("Total = %d, want %d", page.Total, tt.wantTotal)
			}
			if page.NextCursor != tt.wantCursor {
				t.Errorf("NextCursor = %q, want %q", page.NextCursor, tt.wantCursor)
			}
		})
	}

	// 項目が読めていること
	page, err := ss.ListPage(ctx, "", "", 1)
	if err != nil {
		t.Fatalf("ListPage() error = %v", err)
	}
	if got := page.Items[0]; got.Ki != "K" || got.AMF != "8000" || got.SQN != "000000000000" {
		t.Errorf("Items[0] = %+v", got)
	}
}

func TestSubscriberStore_ListPage_Error(t *testing.T) {
	mr, client := newTestRedis(t)
	defer client.Close()

	mr.SetError("forced error")
	if _, err := NewSubscriberStore(client).ListPage(context.Background(), "", "", 50); err == nil {
		t.Error("ListPage() expected error")
	}
}

func TestPolicyStore_ListPage(t *testing.T) {
	mr, client := newTestRedis(t)
	defer client.Close()

	ps := NewPolicyStore(client)
	ctx := context.Background()

	for i := 3; i >= 1; i-- {
		policy := model.NewPolicy(fmt.Sprintf("00101000000000%d", i), "deny")
		policy.Rules = []model.PolicyRule{{NasID: "*", AllowedSSIDs: []string{"CORP"}}}
		if err := ps.Create(ctx, policy); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}
	// rules を解釈できないポリシーは結果に含めない（Total には数える）
	mr.HSet(PolicyKey("001010000000004"), "default", "allow", "rules", "{broken")

	page, err := ps.ListPage(ctx, "", "", 2)
	if err != nil {
		t.Fatalf("ListPage() error = %v", err)
	}
	if len(page.Items) != 2 || page.Items[0].IMSI != "001010000000001" || page.Items[1].IMSI != "001010000000002" {
		t.Fatalf("Items = %+v", page.Items)
	}
	if page.Total != 4 || page.NextCursor != "001010000000002" {
		t.Errorf("Total = %d, NextCursor = %q", page.Total, page.NextCursor)
	}
	if got := page.Items[0].Rules; len(got) != 1 || got[0].NasID != "*" {
		t.Errorf("Rules = %+v", got)
	}

	page, err = ps.ListPage(ctx, "", page.NextCursor, 2)
	if err != nil {
		t.Fatalf("ListPage() error = %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].IMSI != "001010000000003" || page.NextCursor != "" {
		t.Errorf("2ページ目 = %+v, NextCursor = %q", page.Items, page.NextCursor)
	}

	mr.SetError("forced error")
	if _, err := ps.ListPage(ctx, "", "", 50); err == nil {
		t.Error("ListPage() expected error")
	}
}
