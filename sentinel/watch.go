package sentinel

import (
	"context"
	"time"
)

func (s *Sentinel) Run(ctx context.Context, onOutage, onRestore func(context.Context)) error {
	sleepTime := time.Duration(s.cfg.CheckInterval) * time.Second

	s.startMonitor(sleepTime)
}
func (s *Sentinel) startMonitor(sleepTime time.Duration) {
}
