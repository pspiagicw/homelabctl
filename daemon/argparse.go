package daemon

import (
	"context"
	"log/slog"

	"github.com/alecthomas/kong"
	"github.com/pspiagicw/homelabctl/config"
	"github.com/pspiagicw/homelabctl/logging"
	"github.com/pspiagicw/homelabctl/version"
)

type Context struct {
}

var Daemon struct {
	LogLevel string `help:"Set logging verbosity (debug, info, warn, error)" default:"info"`
	Config   string `help:"Path to config file (overrides default resolution)"`
}

func Parse(version version.Info) {
	kong.Parse(&Daemon)

	logging.Setup(logging.Config{
		Level:  Daemon.LogLevel,
		Format: "text",
	})

	slog.Info("Starting homelabctld", "version", version.Version)
	config, err := config.New(Daemon.Config)
	if err != nil {
		slog.Error("error loading config", "error", err)
	}

	o := NewOrchestrator(config)

	ctx := context.Background()
	o.Start(ctx)
}
