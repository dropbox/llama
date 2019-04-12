package main

import (
	"flag"
	"github.com/dropbox/llama"
	"golang.org/x/sys/unix"
	"log"
	"os"
	"os/signal"
)

func main() {
	flag.Parse()

	// Create the collector
	collector := llama.Collector{}

	// Perform setup
	collector.Setup()

	// Let's do this
	collector.Run()

	// Handle signals for stopping, or reloading the config and updating things
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, unix.SIGINT, unix.SIGTERM, unix.SIGHUP)
	for {
		sig := <-sigChan
		switch sig {
		case unix.SIGINT, unix.SIGTERM:
			log.Printf("Received %s, shutting down", sig)
			// TODO(dmar): Add smarter handling here for around stopping things
			return
		case unix.SIGHUP:
			log.Printf("Received %s, reloading and reconfiguring", sig)
			collector.Reload()
		}
	}
}
