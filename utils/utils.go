package utils

import (
	"log/slog"
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

func Ping(address string) bool {
	pinger, err := probing.NewPinger(address)
	if err != nil {
		slog.Error("failed to initialize pinger", "addresss", address, "error", err)
		return false
	}

	pinger.Count = 1
	pinger.Timeout = 3 * time.Second

	if err = pinger.Run(); err != nil {
		slog.Error("failed to run ping", "address", address, "error", err)
		return false
	}

	stats := pinger.Statistics()

	return stats.PacketsRecv > 0
}
