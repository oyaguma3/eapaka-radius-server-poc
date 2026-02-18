package radius

import (
	"bytes"
	"testing"

	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
)

func TestExtractProxyStates_None(t *testing.T) {
	p := radius.New(radius.CodeAccessRequest, []byte("secret"))

	ps := ExtractProxyStates(p)
	if len(ps.Values) != 0 {
		t.Errorf("ExtractProxyStates returned %d values, want 0", len(ps.Values))
	}
}

func TestExtractProxyStates_Single(t *testing.T) {
	p := radius.New(radius.CodeAccessRequest, []byte("secret"))
	state := []byte("proxy-1")
	_ = rfc2865.ProxyState_Add(p, state)

	ps := ExtractProxyStates(p)
	if len(ps.Values) != 1 {
		t.Fatalf("ExtractProxyStates returned %d values, want 1", len(ps.Values))
	}
	if !bytes.Equal(ps.Values[0], state) {
		t.Errorf("ProxyState[0] = %x, want %x", ps.Values[0], state)
	}
}

func TestExtractProxyStates_Multiple(t *testing.T) {
	p := radius.New(radius.CodeAccessRequest, []byte("secret"))
	states := [][]byte{
		[]byte("proxy-first"),
		[]byte("proxy-second"),
		[]byte("proxy-third"),
	}
	for _, s := range states {
		_ = rfc2865.ProxyState_Add(p, s)
	}

	ps := ExtractProxyStates(p)
	if len(ps.Values) != 3 {
		t.Fatalf("ExtractProxyStates returned %d values, want 3", len(ps.Values))
	}
	for i, expected := range states {
		if !bytes.Equal(ps.Values[i], expected) {
			t.Errorf("ProxyState[%d] = %x, want %x", i, ps.Values[i], expected)
		}
	}
}

func TestApply(t *testing.T) {
	// リクエストからProxy-Stateを抽出
	req := radius.New(radius.CodeAccessRequest, []byte("secret"))
	states := [][]byte{
		[]byte("hop-1"),
		[]byte("hop-2"),
	}
	for _, s := range states {
		_ = rfc2865.ProxyState_Add(req, s)
	}

	ps := ExtractProxyStates(req)

	// 別のパケットに適用
	resp := req.Response(radius.CodeAccessAccept)
	ps.Apply(resp)

	// 応答パケットのProxy-Stateを検証
	respStates, err := rfc2865.ProxyState_Gets(resp)
	if err != nil {
		t.Fatal(err)
	}
	if len(respStates) != 2 {
		t.Fatalf("response has %d ProxyState values, want 2", len(respStates))
	}
	for i, expected := range states {
		if !bytes.Equal(respStates[i], expected) {
			t.Errorf("response ProxyState[%d] = %x, want %x", i, respStates[i], expected)
		}
	}
}

func TestApply_Nil(t *testing.T) {
	resp := radius.New(radius.CodeAccessAccept, []byte("secret"))

	// nilレシーバーでもパニックしないことを検証
	var ps *ProxyStates
	ps.Apply(resp)

	respStates, _ := rfc2865.ProxyState_Gets(resp)
	if len(respStates) != 0 {
		t.Errorf("nil ProxyStates.Apply added %d values", len(respStates))
	}
}
