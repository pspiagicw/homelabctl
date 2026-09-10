package utils

import (
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

func Ping(address string) (bool, error) {
	pinger, err := probing.NewPinger(address)
	if err != nil {
		return false, err
	}

	pinger.Count = 1
	pinger.Timeout = 3 * time.Second

	if err = pinger.Run(); err != nil {
		return false, err
	}

	stats := pinger.Statistics()

	return stats.PacketsRecv > 0, nil
}
