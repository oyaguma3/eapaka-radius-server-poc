package radius

import (
	"layeh.com/radius"
)

// ApplyProxyStates はProxy-State属性を応答パケットに追加する（順序維持）。
func ApplyProxyStates(packet *radius.Packet, states [][]byte) {
	for _, state := range states {
		packet.Add(radius.Type(AttrTypeProxyState), state)
	}
}
