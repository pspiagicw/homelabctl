package main

import (
	"github.com/pspiagicw/homelabctl/daemon"
	"github.com/pspiagicw/homelabctl/version"
)

func main() {
	version := version.Get()

	daemon.Run(version)
}
