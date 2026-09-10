package main

import "fmt"

// import "github.com/pspiagicw/homelabctl/pkg/version"

import "github.com/pspiagicw/homelabctl/logging"

func main() {
	fmt.Println("Hello, world!")

	logging.Setup(logging.Config{
		Level:  "debug",
		Format: "text",
	})
	version := version.Get()

}
