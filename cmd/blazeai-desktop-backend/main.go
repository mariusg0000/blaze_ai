// main.go — Electron desktop backend entrypoint.
// Loads the existing BlazeAI runtime config, resolves embedded prompts/skills,
// and serves the desktop backend protocol over stdin/stdout.
// Layer: desktop backend application entry. Dependencies: internal/assets, config, desktopbackend, platform, skills.
package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"os"

	"blazeai/internal/config"
	"blazeai/internal/desktopbackend"
	"blazeai/internal/platform"
	"blazeai/internal/skills"
)

// run bootstraps the desktop backend and starts serving the stdio protocol.
//
// WHAT:  Main startup sequence for the Electron desktop backend.
// WHY:   Electron needs a non-interactive Go subprocess that owns the runtime core.
// HOW:   Detect OS -> bootstrap app home -> load config -> seed builtin skills -> serve protocol.
// RETURNS: error if any required startup step fails.
func run() error {
	projectFlag := flag.String("project", "", "initial desktop project directory")
	lastFlag := flag.Bool("last-session", false, "resume the last clean desktop session on first message")
	flag.Parse()

	osType, err := platform.Detect()
	if err != nil {
		return fmt.Errorf("cannot detect OS: %w", err)
	}
	if err := platform.Bootstrap(); err != nil {
		return fmt.Errorf("cannot bootstrap app home: %w", err)
	}
	needsFirstRun, err := config.NeedsFirstRun()
	if err != nil {
		return fmt.Errorf("cannot check config: %w", err)
	}
	if needsFirstRun {
		return fmt.Errorf("desktop backend requires an existing runtime config; run the console first-run setup before starting Electron")
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("cannot load config: %w", err)
	}
	promptsFS, err := fs.Sub(embeddedPrompts, "resources/prompts")
	if err != nil {
		return fmt.Errorf("cannot resolve embedded prompts: %w", err)
	}
	builtinSkillsFS, err := fs.Sub(embeddedBuiltinSkills, "resources/skills")
	if err != nil {
		return fmt.Errorf("cannot resolve embedded builtin skills: %w", err)
	}
	home, err := platform.AppHome()
	if err != nil {
		return fmt.Errorf("cannot resolve app home: %w", err)
	}
	if err := skills.SeedBuiltins(builtinSkillsFS, home+"/skills"); err != nil {
		return fmt.Errorf("cannot seed builtin skills: %w", err)
	}
	return desktopbackend.Run(context.Background(), cfg, osType, promptsFS, desktopbackend.BackendOptions{
		InitialWorkDir:  *projectFlag,
		ResumeLastClean: *lastFlag,
	}, os.Stdin, os.Stdout)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
