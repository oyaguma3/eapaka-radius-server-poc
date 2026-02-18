package server

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/mocks"
	"go.uber.org/mock/gomock"
)

func TestSecretSource_ValkeyFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCS := mocks.NewMockClientStore(ctrl)
	mockCS.EXPECT().GetClientSecret(gomock.Any(), "192.168.1.100").
		Return("found-secret", nil)

	ss := NewSecretSource(mockCS, "fallback")

	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.100"), Port: 1812}
	secret, err := ss.RADIUSSecret(context.Background(), addr)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if string(secret) != "found-secret" {
		t.Errorf("secret: got %q, want %q", string(secret), "found-secret")
	}
}

func TestSecretSource_Fallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCS := mocks.NewMockClientStore(ctrl)
	mockCS.EXPECT().GetClientSecret(gomock.Any(), "192.168.1.100").
		Return("", nil) // 未登録

	ss := NewSecretSource(mockCS, "fallback-secret")

	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.100"), Port: 1812}
	secret, err := ss.RADIUSSecret(context.Background(), addr)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if string(secret) != "fallback-secret" {
		t.Errorf("secret: got %q, want %q", string(secret), "fallback-secret")
	}
}

func TestSecretSource_NoSecret(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCS := mocks.NewMockClientStore(ctrl)
	mockCS.EXPECT().GetClientSecret(gomock.Any(), "192.168.1.100").
		Return("", nil) // 未登録

	ss := NewSecretSource(mockCS, "") // フォールバックなし

	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.100"), Port: 1812}
	secret, err := ss.RADIUSSecret(context.Background(), addr)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if secret != nil {
		t.Errorf("secret: got %v, want nil", secret)
	}
}

func TestSecretSource_ValkeyError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCS := mocks.NewMockClientStore(ctrl)
	mockCS.EXPECT().GetClientSecret(gomock.Any(), "192.168.1.100").
		Return("", errors.New("valkey unavailable"))

	ss := NewSecretSource(mockCS, "fallback-secret")

	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.100"), Port: 1812}
	secret, err := ss.RADIUSSecret(context.Background(), addr)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if string(secret) != "fallback-secret" {
		t.Errorf("secret: got %q, want %q", string(secret), "fallback-secret")
	}
}

func TestSecretSource_IPExtraction(t *testing.T) {
	tests := []struct {
		name string
		addr net.Addr
		want string
	}{
		{
			"UDPAddr IPv4",
			&net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 1812},
			"10.0.0.1",
		},
		{
			"UDPAddr IPv6",
			&net.UDPAddr{IP: net.ParseIP("::1"), Port: 1812},
			"::1",
		},
		{
			"TCPAddr",
			&net.TCPAddr{IP: net.ParseIP("172.16.0.1"), Port: 1812},
			"172.16.0.1",
		},
		{
			"nil addr",
			nil,
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractIP(tt.addr)
			if got != tt.want {
				t.Errorf("extractIP: got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSecretSource_NilAddr_WithFallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCS := mocks.NewMockClientStore(ctrl)
	// GetClientSecretは呼ばれない

	ss := NewSecretSource(mockCS, "fallback-secret")

	// nilアドレス → IP抽出失敗 → フォールバック
	secret, err := ss.RADIUSSecret(context.Background(), nil)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if string(secret) != "fallback-secret" {
		t.Errorf("secret: got %q, want %q", string(secret), "fallback-secret")
	}
}

func TestSecretSource_NilAddr_NoFallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCS := mocks.NewMockClientStore(ctrl)

	ss := NewSecretSource(mockCS, "")

	// nilアドレス + フォールバックなし → nil
	secret, err := ss.RADIUSSecret(context.Background(), nil)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if secret != nil {
		t.Errorf("secret: got %v, want nil", secret)
	}
}
