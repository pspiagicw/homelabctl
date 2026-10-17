package utils

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	probing "github.com/prometheus-community/pro-bing"
	"github.com/pspiagicw/homelabctl/apierr"
)

func Ping(address string) bool {
	pinger, err := probing.NewPinger(address)
	if err != nil {
		slog.Error("failed to initialize pinger", "address", address, "error", err)
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

func ToJSON(data any) []byte {
	content, err := json.Marshal(data)
	if err != nil {
		slog.Info("error encoding data to json", "error", err)
	}

	return content
}

func ParseErrCode(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, apierr.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, apierr.ErrConflict):
		return http.StatusConflict
	case errors.Is(err, apierr.ErrUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, apierr.ErrUpstream):
		return http.StatusBadGateway
	}
	return http.StatusInternalServerError
}
