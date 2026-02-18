package radius

import (
	"encoding/binary"
	"net"
	"testing"

	radiuspkg "layeh.com/radius"
)

func addUint32Attr(p *radiuspkg.Packet, attrType int, val uint32) {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, val)
	p.Add(radiuspkg.Type(attrType), b)
}

func TestExtractAccountingAttributes(t *testing.T) {
	secret := []byte("testing123")
	packet := &radiuspkg.Packet{
		Code:   radiuspkg.CodeAccountingRequest,
		Secret: secret,
	}

	// Acct-Status-Type = 1 (Start)
	addUint32Attr(packet, AttrTypeAcctStatusType, 1)
	// Acct-Session-Id
	packet.Add(radiuspkg.Type(AttrTypeAcctSessionID), []byte("sess-123"))
	// User-Name
	packet.Add(radiuspkg.Type(AttrTypeUserName), []byte("0001010123456789@example.com"))
	// Class (UUID)
	packet.Add(radiuspkg.Type(AttrTypeClass), []byte("550e8400-e29b-41d4-a716-446655440000"))
	// NAS-IP-Address
	packet.Add(radiuspkg.Type(AttrTypeNASIPAddress), radiuspkg.Attribute(net.IPv4(192, 168, 1, 1).To4()))
	// Framed-IP-Address
	packet.Add(radiuspkg.Type(AttrTypeFramedIPAddr), radiuspkg.Attribute(net.IPv4(10, 0, 0, 1).To4()))
	// Acct-Input-Octets
	addUint32Attr(packet, AttrTypeAcctInputOct, 1000)
	// Acct-Output-Octets
	addUint32Attr(packet, AttrTypeAcctOutputOct, 2000)
	// Acct-Session-Time
	addUint32Attr(packet, AttrTypeAcctSessionTime, 300)

	attrs, err := ExtractAccountingAttributes(packet)
	if err != nil {
		t.Fatalf("ExtractAccountingAttributes failed: %v", err)
	}

	if attrs.AcctStatusType != 1 {
		t.Errorf("AcctStatusType = %d, want 1", attrs.AcctStatusType)
	}
	if attrs.AcctSessionID != "sess-123" {
		t.Errorf("AcctSessionID = %q, want %q", attrs.AcctSessionID, "sess-123")
	}
	if attrs.UserName != "0001010123456789@example.com" {
		t.Errorf("UserName = %q, want %q", attrs.UserName, "0001010123456789@example.com")
	}
	if attrs.ClassUUID != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("ClassUUID = %q, want %q", attrs.ClassUUID, "550e8400-e29b-41d4-a716-446655440000")
	}
	if attrs.NasIPAddress != "192.168.1.1" {
		t.Errorf("NasIPAddress = %q, want %q", attrs.NasIPAddress, "192.168.1.1")
	}
	if attrs.FramedIPAddress != "10.0.0.1" {
		t.Errorf("FramedIPAddress = %q, want %q", attrs.FramedIPAddress, "10.0.0.1")
	}
	if attrs.InputOctets != 1000 {
		t.Errorf("InputOctets = %d, want 1000", attrs.InputOctets)
	}
	if attrs.OutputOctets != 2000 {
		t.Errorf("OutputOctets = %d, want 2000", attrs.OutputOctets)
	}
	if attrs.SessionTime != 300 {
		t.Errorf("SessionTime = %d, want 300", attrs.SessionTime)
	}
}

func TestExtractAccountingAttributes_MissingStatusType(t *testing.T) {
	packet := &radiuspkg.Packet{
		Code:   radiuspkg.CodeAccountingRequest,
		Secret: []byte("secret"),
	}
	packet.Add(radiuspkg.Type(AttrTypeAcctSessionID), []byte("sess-123"))

	_, err := ExtractAccountingAttributes(packet)
	if err != ErrMissingStatusType {
		t.Errorf("expected ErrMissingStatusType, got: %v", err)
	}
}

func TestExtractAccountingAttributes_MissingSessionID(t *testing.T) {
	packet := &radiuspkg.Packet{
		Code:   radiuspkg.CodeAccountingRequest,
		Secret: []byte("secret"),
	}
	addUint32Attr(packet, AttrTypeAcctStatusType, 1)

	_, err := ExtractAccountingAttributes(packet)
	if err != ErrMissingSessionID {
		t.Errorf("expected ErrMissingSessionID, got: %v", err)
	}
}

func TestExtractAccountingAttributes_InvalidClassUUID(t *testing.T) {
	packet := &radiuspkg.Packet{
		Code:   radiuspkg.CodeAccountingRequest,
		Secret: []byte("secret"),
	}
	addUint32Attr(packet, AttrTypeAcctStatusType, 1)
	packet.Add(radiuspkg.Type(AttrTypeAcctSessionID), []byte("sess-123"))
	packet.Add(radiuspkg.Type(AttrTypeClass), []byte("not-a-uuid"))

	attrs, err := ExtractAccountingAttributes(packet)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attrs.ClassUUID != "" {
		t.Errorf("ClassUUID = %q, want empty for invalid UUID", attrs.ClassUUID)
	}
}

func TestExtractProxyStates(t *testing.T) {
	packet := &radiuspkg.Packet{
		Code:   radiuspkg.CodeAccountingRequest,
		Secret: []byte("secret"),
	}
	packet.Add(radiuspkg.Type(AttrTypeProxyState), []byte("proxy1"))
	packet.Add(radiuspkg.Type(AttrTypeProxyState), []byte("proxy2"))

	states := extractProxyStatesRaw(packet)
	if len(states) != 2 {
		t.Fatalf("states count = %d, want 2", len(states))
	}
	if string(states[0]) != "proxy1" {
		t.Errorf("states[0] = %q, want %q", states[0], "proxy1")
	}
	if string(states[1]) != "proxy2" {
		t.Errorf("states[1] = %q, want %q", states[1], "proxy2")
	}
}
