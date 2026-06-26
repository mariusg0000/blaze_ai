// main.go — BlazeAI application entry point.
// Parses CLI flags, bootstraps app home, loads config or runs first-run setup,
// creates or resumes a session, and starts the console transport over the agent core.
// Layer: application entry. Direct dependencies: internal/console, internal/runtime,
// internal/config, internal/session, internal/platform.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"

	"blazeai/internal/config"
	"blazeai/internal/console"
	"blazeai/internal/platform"
	"blazeai/internal/runtime"
	"blazeai/internal/session"
	"blazeai/internal/skills"
)

type resumeOptions struct {
	continueLastClean bool
	resumeLast        bool
}

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
	resumeFlag := flag.Bool("r", false, "resume most recent session (interrupted or clean)")
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
	cfg, err := loadRuntimeConfig()
	if err != nil {
		return err
	}

	// Get work directory (needed for project-based session storage).
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get working directory: %w", err)
	}

	resume := resumeOptions{continueLastClean: *continueFlag, resumeLast: *resumeFlag}
	sess, err := openSession(workDir, resume)
	if err != nil {
		return err
	}

	promptsFS, err := prepareBuiltinAssets()
	if err != nil {
		return err
	}

	if err := runConsole(cfg, sess, osType, promptsFS, workDir, resume); err != nil {
		return err
	}

	return nil
}

// loadRuntimeConfig loads the global config or runs first-run setup.
func loadRuntimeConfig() (*config.Config, error) {
	needs, err := config.NeedsFirstRun()
	if err != nil {
		return nil, fmt.Errorf("cannot check config: %w", err)
	}
	if needs {
		cfg, err := runFirstRun()
		if err != nil {
			return nil, fmt.Errorf("first-run setup failed: %w", err)
		}
		return cfg, nil
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("cannot load config: %w", err)
	}
	return cfg, nil
}

// openSession creates or resumes the session for the current work directory.
func openSession(workDir string, resume resumeOptions) (*session.Session, error) {
	switch {
	case resume.continueLastClean:
		sess, err := session.LastClean(workDir)
		if err != nil {
			return nil, fmt.Errorf("cannot continue session: %w", err)
		}
		fmt.Printf("Resuming session: %s\n", sess.Folder)
		return sess, nil
	case resume.resumeLast:
		sess, err := session.Last(workDir)
		if err != nil {
			return nil, fmt.Errorf("cannot resume session: %w", err)
		}
		fmt.Printf("Resuming session: %s\n", sess.Folder)
		return sess, nil
	default:
		sess, err := session.Create(workDir)
		if err != nil {
			return nil, fmt.Errorf("cannot create session: %w", err)
		}
		return sess, nil
	}
}

// prepareBuiltinAssets resolves prompt templates and seeds builtin skill files into app home.
func prepareBuiltinAssets() (fs.FS, error) {
	promptsFS, err := fs.Sub(embeddedPrompts, "prompts")
	if err != nil {
		return nil, fmt.Errorf("cannot resolve embedded prompts: %w", err)
	}
	templatesFS, err := fs.Sub(embeddedBuiltinSkills, "skills")
	if err != nil {
		return nil, fmt.Errorf("cannot resolve embedded skill templates: %w", err)
	}
	home, err := platform.AppHome()
	if err != nil {
		return nil, fmt.Errorf("cannot resolve app home: %w", err)
	}
	if err := skills.SeedBuiltins(templatesFS, home+"/skills"); err != nil {
		return nil, fmt.Errorf("cannot seed builtin skills: %w", err)
	}
	return promptsFS, nil
}

// runConsole starts the console transport over a newly created runtime agent.
func runConsole(cfg *config.Config, sess *session.Session, osType platform.OS, promptsFS fs.FS, workDir string, resume resumeOptions) error {

	// Create agent and console.
	agent, err := runtime.NewAgent(cfg, sess, osType, promptsFS, workDir, nil)
	if err != nil {
		return fmt.Errorf("cannot create agent: %w", err)
	}

	// On -c or -r resume, rebuild synthetic summary message from summary files.
	if (resume.continueLastClean || resume.resumeLast) && agent.Compactor != nil {
		if err := agent.Compactor.RebuildForResume(sess); err != nil {
			return fmt.Errorf("cannot rebuild summaries for resume: %w", err)
		}
	}

	cons := console.NewConsole(agent)
	agent.Handler = cons
	if err := cons.Run(); err != nil {
		return fmt.Errorf("console error: %w", err)
	}
	return nil
}
