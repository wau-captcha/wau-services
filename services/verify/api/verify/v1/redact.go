package v1

import "log/slog"

// LogValue keeps the one-time token out of logs, e.g. out of the request a recovered panic is
// logged with.
func (x *SiteVerifyRequest) LogValue() slog.Value {
	return slog.GroupValue(slog.String("token", "[redacted]"))
}
