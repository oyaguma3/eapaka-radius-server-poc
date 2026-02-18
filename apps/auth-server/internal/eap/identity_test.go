package eap

import (
	"errors"
	"testing"

	eapaka "github.com/oyaguma3/go-eapaka"
)

func TestParseIdentity_PermanentAKA(t *testing.T) {
	id, err := ParseIdentity("0001010123456789@realm")
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if id.Type != IdentityTypePermanentAKA {
		t.Errorf("Type: got %d, want %d", id.Type, IdentityTypePermanentAKA)
	}
	if id.IMSI != "001010123456789" {
		t.Errorf("IMSI: got %q, want %q", id.IMSI, "001010123456789")
	}
	if id.Realm != "realm" {
		t.Errorf("Realm: got %q, want %q", id.Realm, "realm")
	}
	if id.EAPType != eapaka.TypeAKA {
		t.Errorf("EAPType: got %d, want %d", id.EAPType, eapaka.TypeAKA)
	}
	if id.Raw != "0001010123456789@realm" {
		t.Errorf("Raw: got %q, want %q", id.Raw, "0001010123456789@realm")
	}
}

func TestParseIdentity_PermanentAKAPrime(t *testing.T) {
	id, err := ParseIdentity("6001010123456789@realm")
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if id.Type != IdentityTypePermanentAKAPrime {
		t.Errorf("Type: got %d, want %d", id.Type, IdentityTypePermanentAKAPrime)
	}
	if id.IMSI != "001010123456789" {
		t.Errorf("IMSI: got %q, want %q", id.IMSI, "001010123456789")
	}
	if id.EAPType != eapaka.TypeAKAPrime {
		t.Errorf("EAPType: got %d, want %d", id.EAPType, eapaka.TypeAKAPrime)
	}
}

func TestParseIdentity_PseudonymAKA(t *testing.T) {
	id, err := ParseIdentity("2pseudonym@realm")
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if id.Type != IdentityTypePseudonymAKA {
		t.Errorf("Type: got %d, want %d", id.Type, IdentityTypePseudonymAKA)
	}
	if id.IMSI != "" {
		t.Errorf("IMSI: got %q, want empty", id.IMSI)
	}
	if id.EAPType != eapaka.TypeAKA {
		t.Errorf("EAPType: got %d, want %d", id.EAPType, eapaka.TypeAKA)
	}
}

func TestParseIdentity_PseudonymAKAPrime(t *testing.T) {
	id, err := ParseIdentity("7pseudonym@realm")
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if id.Type != IdentityTypePseudonymAKAPrime {
		t.Errorf("Type: got %d, want %d", id.Type, IdentityTypePseudonymAKAPrime)
	}
	if id.EAPType != eapaka.TypeAKAPrime {
		t.Errorf("EAPType: got %d, want %d", id.EAPType, eapaka.TypeAKAPrime)
	}
}

func TestParseIdentity_ReauthAKA(t *testing.T) {
	id, err := ParseIdentity("4reauth@realm")
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if id.Type != IdentityTypeReauthAKA {
		t.Errorf("Type: got %d, want %d", id.Type, IdentityTypeReauthAKA)
	}
	if id.EAPType != eapaka.TypeAKA {
		t.Errorf("EAPType: got %d, want %d", id.EAPType, eapaka.TypeAKA)
	}
}

func TestParseIdentity_ReauthAKAPrime(t *testing.T) {
	id, err := ParseIdentity("8reauth@realm")
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if id.Type != IdentityTypeReauthAKAPrime {
		t.Errorf("Type: got %d, want %d", id.Type, IdentityTypeReauthAKAPrime)
	}
	if id.EAPType != eapaka.TypeAKAPrime {
		t.Errorf("EAPType: got %d, want %d", id.EAPType, eapaka.TypeAKAPrime)
	}
}

func TestParseIdentity_SIMPermanent(t *testing.T) {
	_, err := ParseIdentity("1001010123456789@realm")
	if !errors.Is(err, ErrUnsupportedIdentity) {
		t.Errorf("got %v, want ErrUnsupportedIdentity", err)
	}
}

func TestParseIdentity_SIMPseudonym(t *testing.T) {
	_, err := ParseIdentity("3pseudonym@realm")
	if !errors.Is(err, ErrUnsupportedIdentity) {
		t.Errorf("got %v, want ErrUnsupportedIdentity", err)
	}
}

func TestParseIdentity_SIMReauth(t *testing.T) {
	_, err := ParseIdentity("5reauth@realm")
	if !errors.Is(err, ErrUnsupportedIdentity) {
		t.Errorf("got %v, want ErrUnsupportedIdentity", err)
	}
}

func TestParseIdentity_MissingRealm(t *testing.T) {
	_, err := ParseIdentity("0001010123456789")
	if !errors.Is(err, ErrMissingRealm) {
		t.Errorf("got %v, want ErrMissingRealm", err)
	}
}

func TestParseIdentity_EmptyString(t *testing.T) {
	_, err := ParseIdentity("")
	if !errors.Is(err, ErrInvalidIdentity) {
		t.Errorf("got %v, want ErrInvalidIdentity", err)
	}
}

func TestParseIdentity_OnlyAtSign(t *testing.T) {
	_, err := ParseIdentity("@realm")
	if !errors.Is(err, ErrInvalidIdentity) {
		t.Errorf("got %v, want ErrInvalidIdentity", err)
	}
}

func TestParseIdentity_UnknownPrefix(t *testing.T) {
	_, err := ParseIdentity("Xtest@realm")
	if !errors.Is(err, ErrInvalidIdentity) {
		t.Errorf("got %v, want ErrInvalidIdentity", err)
	}
}

func TestRequiresFullAuth_Permanent(t *testing.T) {
	tests := []struct {
		name     string
		identity string
	}{
		{"AKA永続ID", "0001010123456789@realm"},
		{"AKA'永続ID", "6001010123456789@realm"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := ParseIdentity(tt.identity)
			if err != nil {
				t.Fatalf("予期しないエラー: %v", err)
			}
			if id.RequiresFullAuth() {
				t.Error("永続IDはRequiresFullAuth()=falseであるべき")
			}
		})
	}
}

func TestRequiresFullAuth_Pseudonym(t *testing.T) {
	tests := []struct {
		name     string
		identity string
	}{
		{"AKA仮名", "2pseudonym@realm"},
		{"AKA'仮名", "7pseudonym@realm"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := ParseIdentity(tt.identity)
			if err != nil {
				t.Fatalf("予期しないエラー: %v", err)
			}
			if !id.RequiresFullAuth() {
				t.Error("仮名IDはRequiresFullAuth()=trueであるべき")
			}
		})
	}
}

func TestRequiresFullAuth_Reauth(t *testing.T) {
	tests := []struct {
		name     string
		identity string
	}{
		{"AKA再認証", "4reauth@realm"},
		{"AKA'再認証", "8reauth@realm"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := ParseIdentity(tt.identity)
			if err != nil {
				t.Fatalf("予期しないエラー: %v", err)
			}
			if !id.RequiresFullAuth() {
				t.Error("再認証IDはRequiresFullAuth()=trueであるべき")
			}
		})
	}
}

func TestIsPermanent(t *testing.T) {
	tests := []struct {
		name     string
		identity string
		want     bool
	}{
		{"AKA永続ID", "0001010123456789@realm", true},
		{"AKA'永続ID", "6001010123456789@realm", true},
		{"AKA仮名", "2pseudonym@realm", false},
		{"AKA'仮名", "7pseudonym@realm", false},
		{"AKA再認証", "4reauth@realm", false},
		{"AKA'再認証", "8reauth@realm", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := ParseIdentity(tt.identity)
			if err != nil {
				t.Fatalf("予期しないエラー: %v", err)
			}
			if got := id.IsPermanent(); got != tt.want {
				t.Errorf("IsPermanent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsAKAPrime(t *testing.T) {
	tests := []struct {
		name     string
		identity string
		want     bool
	}{
		{"AKA永続ID", "0001010123456789@realm", false},
		{"AKA仮名", "2pseudonym@realm", false},
		{"AKA再認証", "4reauth@realm", false},
		{"AKA'永続ID", "6001010123456789@realm", true},
		{"AKA'仮名", "7pseudonym@realm", true},
		{"AKA'再認証", "8reauth@realm", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := ParseIdentity(tt.identity)
			if err != nil {
				t.Fatalf("予期しないエラー: %v", err)
			}
			if got := id.IsAKAPrime(); got != tt.want {
				t.Errorf("IsAKAPrime() = %v, want %v", got, tt.want)
			}
		})
	}
}
