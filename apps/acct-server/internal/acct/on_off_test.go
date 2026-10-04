package acct

import (
	"context"
	"testing"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/radius"
)

func TestProcessOn(t *testing.T) {
	p := &Processor{}
	attrs := &radius.AccountingAttributes{
		AcctStatusType: radius.AcctStatusTypeOn,
		NasIPAddress:   "192.168.1.1",
		NasIdentifier:  "ap-001.example.com",
	}

	p.ProcessOn(context.Background(), attrs, "10.0.0.1", "trace-on-001")
}

func TestProcessOff(t *testing.T) {
	p := &Processor{}
	attrs := &radius.AccountingAttributes{
		AcctStatusType: radius.AcctStatusTypeOff,
		NasIPAddress:   "192.168.1.1",
		NasIdentifier:  "ap-001.example.com",
	}

	p.ProcessOff(context.Background(), attrs, "10.0.0.1", "trace-off-001")
}
