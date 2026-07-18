// main.go — BlazeAI application entry point.
// Parses CLI flags, bootstraps app home, loads config or runs first-run setup,
// and starts the console transport by default. Telegram is opt-in via CLI.
// Layer: application entry. Direct dependencies: internal/console,
// internal/runtime, internal/config, internal/session, internal/platform.
package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"syscall"

	"blazeai/internal/config"
	"blazeai/internal/console"
	"blazeai/internal/platform"
	"blazeai/internal/runtime"
	"blazeai/internal/session"
	"blazeai/internal/telegram"
)

type resumeOptions struct {
	continueLastClean bool
	resumeLast        bool
}

func main() {
	// Prevent Ctrl+\ from sending SIGQUIT and killing the process.
	// In raw mode (at the prompt), Ctrl+\ is captured as input byte 0x1C.
	// In cooked mode (during interactive prompts), ignoring SIGQUIT prevents
	// an accidental Ctrl+\ from terminating the session.
	signal.Ignore(syscall.SIGQUIT)

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "blazeai: %s\n", err)
		os.Exit(1)
	}
}

// run bootstraps configuration, session, runtime, and the selected transport.
// Defaults to the console transport when no transport flag is given.
// Returns an error if any required startup step fails. No silent fallbacks.
//
// WHAT:  The main application startup sequence.
// WHY:   Wires all packages together and starts the chosen transport.
// HOW:   Bootstrap app home → detect OS → load/first-run config → transport.
// RETURNS: error if any startup step fails.
func run() error {
	continueFlag := flag.Bool("c", false, "continue last cleanly closed session (console only)")
	resumeFlag := flag.Bool("r", false, "resume most recent session, interrupted or clean (console only)")
	consoleFlag := flag.Bool("console", false, "run terminal REPL transport (default)")
	telegramFlag := flag.String("telegram", "", "run Telegram bridge instance")
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

	promptsFS, builtinSkillsFS, err := prepareBuiltinAssets()
	if err != nil {
		return err
	}

	if *telegramFlag != "" {
		return telegram.Run(context.Background(), cfg, osType, promptsFS, builtinSkillsFS, *telegramFlag)
	}
	_ = consoleFlag
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get working directory: %w", err)
	}
	resume := resumeOptions{continueLastClean: *continueFlag, resumeLast: *resumeFlag}
	sess, err := openSession(workDir, resume)
	if err != nil {
		return err
	}
	if err := runConsole(cfg, sess, osType, promptsFS, builtinSkillsFS, workDir, resume); err != nil {
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

// prepareBuiltinAssets resolves the immutable embedded prompt and builtin skill filesystems.
// WHAT: Returns read-only embedded sub-filesystems for runtime asset access.
// HOW: Resolves both required roots without creating or modifying app-home asset directories.
func prepareBuiltinAssets() (fs.FS, fs.FS, error) {
	embeddedPromptFS, err := fs.Sub(embeddedPrompts, "prompts")
	if err != nil {
		return nil, nil, fmt.Errorf("cannot resolve embedded prompts: %w", err)
	}
	builtinSkillsFS, err := fs.Sub(embeddedBuiltinSkills, "skills")
	if err != nil {
		return nil, nil, fmt.Errorf("cannot resolve embedded builtin skills: %w", err)
	}
	return embeddedPromptFS, builtinSkillsFS, nil
}

// runConsole starts the console transport over a newly created runtime agent.
func runConsole(cfg *config.Config, sess *session.Session, osType platform.OS, promptsFS, builtinSkillsFS fs.FS, workDir string, resume resumeOptions) error {

	// Create agent and console.
	agent, err := runtime.NewAgent(cfg, sess, osType, promptsFS, builtinSkillsFS, workDir, nil, "console")
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
