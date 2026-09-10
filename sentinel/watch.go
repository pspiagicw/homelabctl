package sentinel

import (
	"context"
	"log/slog"
	"time"
)

func (s *Sentinel) Run(ctx context.Context, onOutage, onRestore func(context.Context)) {
	ticker := time.NewTicker(time.Duration(s.cfg.CheckInterval) * time.Second)
	slog.Info("Starting sentinel monitoring!")
	slog.Info("Sentinel Address", "addresss", s.cfg.Address)
	downTime := 0
	upTime := 0

	for {
		select {
		case <-ctx.Done():
			slog.Info("shutdown requested, shutting down sentinel watcher.")
			return
		case <-ticker.C:
			pingStatus, err := s.Ping()

			slog.Info("Ping status", "response", pingStatus)

			if err != nil {
				slog.Error("error pinging sentinel", "error", err)
			}

			switch s.State {
			case Normal:
				slog.Info("Sentinel online, no action needed.")
				if !pingStatus {
					// Outage started
					s.State = OutageDetected
				}
				break
			case OutageDetected:
				slog.Warn("Sentinel offline, waiting for it to come online", "downtime", downTime, "total", s.cfg.OutageGracePeriod)
				if pingStatus {
					upTime = 0
					downTime = 0
					s.State = Normal
					break
				}
				downTime += s.cfg.CheckInterval
				if downTime > s.cfg.OutageGracePeriod {
					s.State = OutageConfirmed
				}
				break
			case OutageConfirmed:
				slog.Warn("Confirmed outage, shutting down systems.")
				// TODO: Check if we need to run this in a goroutine.
				onOutage(ctx)
				if pingStatus {
					s.State = Restoring
					downTime = 0
					upTime = 0
				}
				break
			case Restoring:
				slog.Warn("Sentinel online; waiting for stable power", "uptime", upTime, "total", s.cfg.RestoreGracePeriod)
				if !pingStatus {
					s.State = OutageDetected
					downTime = 0
					upTime = 0
				}
				upTime += s.cfg.CheckInterval
				if upTime > s.cfg.RestoreGracePeriod {
					s.State = Restored
				}
				break
			case Restored:
				slog.Warn("Outage Restored, bringing systems back to normal.")
				onRestore(ctx)
				s.State = Normal
				downTime = 0
				upTime = 0
				break
			}
		}
	}
}
