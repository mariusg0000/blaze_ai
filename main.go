// main.go — BlazeAI application entry point.
// Parses CLI flags and starts the console transport over the agent core.
// Layer: application entry. Direct dependencies: internal/console, internal/runtime.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "blazeai: %s\n", err)
		os.Exit(1)
	}
}

// run bootstraps configuration, session, runtime, and the console transport.
// Returns an error if any required startup step fails. No silent fallbacks.
func run() error {
	// TODO: bootstrap config, session, runtime, console
	return nil
}
