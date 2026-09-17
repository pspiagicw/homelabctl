package sentinel

import (
	"context"
	"log/slog"
	"time"
)

func (s *Sentinel) SetState(state SentinelState) {
	slog.Info("sentinel state transition", "from", s.State, "to", state)
	s.State = state
	s.GraceDownTime = 0
	s.GraceUpTime = 0
}
func (s *Sentinel) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(s.cfg.CheckInterval) * time.Second)
	slog.Info("sentinel started!")
	slog.Info("sentinel info", "addresss", s.cfg.Address, "status", s.State)

	for {
		select {
		case <-ctx.Done():
			slog.Info("shutdown requested, shutting down sentinel watcher.")
			return
		case <-ticker.C:
			pingStatus := s.Ping()

			slog.Info("sentinel status", "response", pingStatus)
			slog.Info("sentinel info", "addresss", s.cfg.Address, "status", s.State)

			switch s.State {
			case Normal:
				slog.Info("sentinel online, no action needed.")
				if !pingStatus {
					s.SetState(OutageDetected)
				}
				break
			case OutageDetected:
				slog.Warn("sentinel offline, waiting for outage confirmation", "downtime", s.GraceDownTime, "total", s.cfg.OutageGracePeriod)
				if pingStatus {
					s.SetState(Normal)
					break
				}
				s.GraceDownTime += s.cfg.CheckInterval
				if s.GraceDownTime >= s.cfg.OutageGracePeriod {
					s.SetState(OutageConfirmed)
				}
				break
			case OutageConfirmed:
				slog.Warn("outage confirmed. starting shutdown sequence!")
				// TODO: Check if we need to run this in a goroutine.
				s.OnOutage(ctx)
				if pingStatus {
					slog.Warn("successfull ping, assuming sentinel online")
					s.SetState(Restoring)
				}
				break
			case Restoring:
				slog.Warn("sentinel online, waiting for stable power", "uptime", s.GraceUpTime, "total", s.cfg.RestoreGracePeriod)
				if !pingStatus {
					slog.Warn("ping failed, assuming outage")
					s.SetState(OutageDetected)
				}
				s.GraceUpTime += s.cfg.CheckInterval
				if s.GraceUpTime >= s.cfg.RestoreGracePeriod {
					s.State = Restored
					s.SetState(Restored)
				}
				break
			case Restored:
				slog.Warn("outage restored, starting restore sequence!")
				s.SetState(Normal)
				s.OnRestore(ctx)
				break
			}
		}
	}
}
