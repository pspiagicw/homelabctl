package utils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/linde12/gowol"
	probing "github.com/prometheus-community/pro-bing"
	"github.com/pspiagicw/homelabctl/apierr"
	"github.com/pspiagicw/homelabctl/config"
	"golang.org/x/crypto/ssh"
)

func After(d time.Duration) <-chan time.Time {
	t := time.NewTimer(d)
	return t.C
}

func Wake(mac string) error {
	packet, err := gowol.NewMagicPacket(mac)
	if err != nil {
	}

	if err != nil {
		// Log one for the server and return error for the request
		slog.Error("failed to create magic packet", "error", err)
		return fmt.Errorf("failed to create magic packet; error: %v; %w", err, apierr.ErrUpstream)
	}

	err = packet.Send("255.255.255.255")

	if err != nil {
		slog.Error("failed to send magic packet", "error", err)
		return fmt.Errorf("failed to send magic packet; error: %v; %w", err, apierr.ErrUpstream)
	}

	return nil
}

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

func SSHRun(ctx context.Context, addr string, cmd string, cfg config.SSHConfig) error {
	client, err := NewSSHClient(cfg, addr)

	if err != nil {
		return fmt.Errorf("error creating ssh client: %v", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("error creating new ssh session: %v", err)
	}

	defer session.Close()

	// var b bytes.Buffer
	// var e bytes.Buffer
	// session.Stdout = &b
	// session.Stderr = &e
	slog.Info("ssh connected", "address", addr)

	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("failed to run cmd: %v", err)
	}

	return nil
}

func NewSSHClient(cfg config.SSHConfig, host string) (*ssh.Client, error) {
	authMethod, err := publicKeyAuth(cfg.IdentityFile)
	if err != nil {
		return nil, fmt.Errorf("error loading identity file: %v", err)
	}

	config := &ssh.ClientConfig{
		User: cfg.User,
		Auth: []ssh.AuthMethod{
			authMethod,
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", host+":22", config)
	if err != nil {
		return nil, fmt.Errorf("error dialing ssh connection: %v", err)
	}

	return client, nil
}

func publicKeyAuth(path string) (ssh.AuthMethod, error) {
	keyBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading identity file: %v", err)
	}

	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("error parsing private key: %v", err)
	}

	return ssh.PublicKeys(signer), nil

}
