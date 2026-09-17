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
	LogLevel  string `help:"Set logging verbosity (debug, info, warn, error)" default:"info"`
	Config    string `help:"Path to config file (overrides default resolution)"`
	LogFormat string `help:"log output format: text or json" default:"text"`
}

func Run(version version.Info) {
	Parse()
	InitLogger()
	config := ParseConfig(Daemon.Config)

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
	kong.Parse(&Daemon)
}

func ParseConfig(configPath string) *config.Config {
	config := config.New(Daemon.Config)
	if config == nil {
		logging.Fatal("FATAL: failed to load config!")
	}
	slog.Info("config loaded and validated!")

	return config
}
