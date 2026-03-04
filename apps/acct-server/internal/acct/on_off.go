package acct

import (
	"context"
	"log/slog"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/radius"
)

// ProcessOn はAccounting-On（NAS起動通知）を処理する。
func (p *Processor) ProcessOn(_ context.Context, attrs *radius.AccountingAttributes, srcIP, traceID string) error {
	slog.Info("accounting on",
		"event_id", "ACCT_ON",
		"trace_id", traceID,
		"src_ip", srcIP,
		"nas_ip_address", attrs.NasIPAddress,
		"nas_identifier", attrs.NasIdentifier,
	)
	return nil
}

// ProcessOff はAccounting-Off（NASシャットダウン通知）を処理する。
func (p *Processor) ProcessOff(_ context.Context, attrs *radius.AccountingAttributes, srcIP, traceID string) error {
	slog.Info("accounting off",
		"event_id", "ACCT_OFF",
		"trace_id", traceID,
		"src_ip", srcIP,
		"nas_ip_address", attrs.NasIPAddress,
		"nas_identifier", attrs.NasIdentifier,
	)
	return nil
}
