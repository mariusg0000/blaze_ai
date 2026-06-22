// main.go — BlazeAI application entry point.
// Parses CLI flags, bootstraps app home, loads config or runs first-run setup,
// creates or resumes a session, and starts the console transport over the agent core.
// Layer: application entry. Direct dependencies: internal/console, internal/runtime,
// internal/config, internal/session, internal/platform.
package main

import (
	"flag"
	"fmt"
	"os"

	"blazeai/internal/config"
	"blazeai/internal/console"
	"blazeai/internal/platform"
	"blazeai/internal/runtime"
	"blazeai/internal/session"
	"blazeai/internal/tools"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "blazeai: %s\n", err)
		os.Exit(1)
	}
}

// run bootstraps configuration, session, runtime, and the console transport.
// Returns an error if any required startup step fails. No silent fallbacks.
//
// WHAT:  The main application startup sequence.
// WHY:   Wires all packages together and starts the REPL.
// HOW:   Bootstrap app home → detect OS → load/first-run config → create/resume session → agent → console.
// RETURNS: error if any startup step fails.
func run() error {
	continueFlag := flag.Bool("c", false, "continue last cleanly closed session")
	flag.Parse()

	// Detect OS.
	osType, err := platformOS()
	if err != nil {
		return fmt.Errorf("cannot detect OS: %w", err)
	}

	// Bootstrap app home directories.
	if err := platform.Bootstrap(); err != nil {
		return fmt.Errorf("cannot bootstrap app home: %w", err)
	}

	// Load config or run first-run setup.
	needs, err := config.NeedsFirstRun()
	if err != nil {
		return fmt.Errorf("cannot check config: %w", err)
	}
	var cfg *config.Config
	if needs {
		cfg, err = runFirstRun()
		if err != nil {
			return fmt.Errorf("first-run setup failed: %w", err)
		}
	} else {
		cfg, err = config.Load()
		if err != nil {
			return fmt.Errorf("cannot load config: %w", err)
		}
	}

	// Create or resume session.
	var sess *session.Session
	if *continueFlag {
		sess, err = session.LastClean()
		if err != nil {
			return fmt.Errorf("cannot continue session: %w", err)
		}
		fmt.Printf("Resuming session: %s\n", sess.Folder)
	} else {
		sess, err = session.Create()
		if err != nil {
			return fmt.Errorf("cannot create session: %w", err)
		}
	}

	// Resolve builtin paths.
	promptsDir, builtinSkillsDir := resolveBuiltinPaths()

	// Get work directory.
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get working directory: %w", err)
	}

	// Create agent and console.
	agent, err := runtime.NewAgent(cfg, sess, osType, builtinSkillsDir, promptsDir, workDir, nil)
	if err != nil {
		return fmt.Errorf("cannot create agent: %w", err)
	}

	// Register skill tools that need the active list.
	agent.Tools.Register(tools.NewLoadSkillTool(agent.Active))
	agent.Tools.Register(tools.NewUnloadSkillTool(agent.Active))

	cons := console.NewConsole(agent)
	if err := cons.Run(); err != nil {
		return fmt.Errorf("console error: %w", err)
	}

	return nil
}
