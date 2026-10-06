package format

import "testing"

func TestOrDash(t *testing.T) {
	if got := OrDash(""); got != "-" {
		t.Errorf(`OrDash("") = %q, want "-"`, got)
	}
	if got := OrDash("ap-001"); got != "ap-001" {
		t.Errorf(`OrDash("ap-001") = %q, want "ap-001"`, got)
	}
}
