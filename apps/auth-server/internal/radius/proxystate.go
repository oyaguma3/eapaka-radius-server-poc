package radius

import (
	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
)

// ProxyStates はリクエストから抽出されたProxy-State属性値の順序付きリスト。
// RFC 2865に基づき、応答パケットには受信したProxy-State属性を
// 同じ順序で含める必要がある。
type ProxyStates struct {
	Values [][]byte
}

// ExtractProxyStates はRADIUSパケットから全Proxy-State属性を抽出する（順序維持）。
func ExtractProxyStates(p *radius.Packet) *ProxyStates {
	values, _ := rfc2865.ProxyState_Gets(p)
	return &ProxyStates{Values: values}
}

// Apply はProxy-State属性を応答パケットに追加する（抽出時と同じ順序）。
func (ps *ProxyStates) Apply(p *radius.Packet) {
	if ps == nil {
		return
	}
	for _, v := range ps.Values {
		_ = rfc2865.ProxyState_Add(p, v)
	}
}
