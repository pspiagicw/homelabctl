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
	LogLevel  string           `help:"Set logging verbosity (debug, info, warn, error)" default:"info"`
	Config    string           `help:"Path to config file (overrides default resolution)"`
	LogFormat string           `help:"log output format: text or json" default:"text"`
	Version   kong.VersionFlag `help:"Print version information and exit"`
}

func Run(version version.Info) {
	Parse()
	InitLogger()
	config := config.New(Daemon.Config)
	slog.Info("config loaded and validated!")

	slog.Info("starting homelabctld", "version", version.Version, "commit", version.Commit, "buildDate", version.BuildDate)

	o := NewOrchestrator(config)

	ctx := context.Background()
	o.Init()
	o.Start(ctx)
}

func InitLogger() {
	logging.Setup(logging.Config{
		Level:  Daemon.LogLevel,
		Format: Daemon.LogFormat,
	})
	slog.Info("logger initialized!", "level", Daemon.LogLevel, "format", Daemon.LogFormat)
}

func Parse() {
	kong.Parse(&Daemon, kong.Vars{"version": "homelabctld " + version.String()})
}
