package client

import (
	"context"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/ui"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/masterdata"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
	"github.com/redis/go-redis/v9"
)

// TestFormScreen_EditTitleShowsID は、編集画面のタイトルにサーバー採番の ID を表示することを確認する。
func TestFormScreen_EditTitleShowsID(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cs := masterdata.NewClientStore(rdb)

	c := &model.RadiusClient{IP: "10.0.0.1", Secret: "s", Name: "AP"}
	if err := cs.Create(context.Background(), c); err != nil {
		t.Fatal(err)
	}

	s := NewFormScreen(ui.NewApp(), cs, nil)
	if err := s.SetupEdit(context.Background(), "10.0.0.1"); err != nil {
		t.Fatalf("SetupEdit() error = %v", err)
	}
	if got := s.GetForm().GetTitle(); !strings.Contains(got, "(ID 1)") {
		t.Errorf("title = %q, want it to contain (ID 1)", got)
	}
}
