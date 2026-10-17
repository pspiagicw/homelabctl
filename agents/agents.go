// Not AI agents, just interfaces for things that work.
package agents

import (
	"context"
	"time"

	"github.com/pspiagicw/homelabctl/config"
)

type Pinger func(address string) bool
type WakeSender func(mac string) error
type Runner func(ctx context.Context, addr, cmd string, cfg config.SSHConfig) error
type After func(d time.Duration) <-chan time.Time
