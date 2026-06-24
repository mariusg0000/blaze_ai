package helpers

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"blazeai/internal/config"
)

// LookupFunc is the signature for binary lookup, injectable for tests.
type LookupFunc func(name string) (string, error)

// Kind classifies a helper as always-shown (Core) or project-conditional (Contextual).
type Kind string

const (
	KindCore       Kind = "core"
	KindContextual Kind = "contextual"
)

// Helper defines one cross-platform host utility.
//
// WHAT:  Static catalog entry for a host utility.
// WHY:   The runtime needs to know which helpers exist, what they do, and when to show them.
// PARAMS: Name — binary name; Description — short summary; Instruction — usage instruction for the prompt;
//
//	Kind — core or contextual; ProjectFiles — ecosystem indicator files for contextual helpers.
type Helper struct {
	Name         string
	Description  string
	Instruction  string
	Kind         Kind
	ProjectFiles []string
}

// Status pairs a Helper with its detected availability.
//
// WHAT:  Result of runtime helper detection.
// PARAMS: Helper — catalog entry; Available — whether the binary was found; Path — resolved binary path if found.
type Status struct {
	Helper
	Available bool
	Path      string
}

// Known is the static catalog of cross-platform helper utilities.
var Known = []Helper{
	{
		Name:        "rg",
		Description: "fast recursive content search",
		Instruction: "Use rg for fast recursive content search instead of broad grep/find pipelines.",
		Kind:        KindCore,
	},
	{
		Name:        "fd",
		Description: "fast file and directory discovery",
		Instruction: "Use fd for fast file and directory discovery.",
		Kind:        KindCore,
	},
	{
		Name:        "jq",
		Description: "JSON inspection and transformation",
		Instruction: "Use jq for JSON inspection and transformation.",
		Kind:        KindCore,
	},
	{
		Name:        "git",
		Description: "repository inspection and version-control operations",
		Instruction: "Use git for repository inspection and version-control operations.",
		Kind:        KindCore,
	},
	{
		Name:        "curl",
		Description: "HTTP/API checks and downloads",
		Instruction: "Use curl for HTTP requests, API checks, and downloads.",
		Kind:        KindCore,
	},
	{
		Name:         "go",
		Description:  "Go toolchain for building and testing",
		Instruction:  "Use go for building, testing, and running Go code.",
		Kind:         KindContextual,
		ProjectFiles: []string{"go.mod"},
	},
	{
		Name:         "node",
		Description:  "JavaScript/TypeScript runtime",
		Instruction:  "Use node for running JavaScript or TypeScript.",
		Kind:         KindContextual,
		ProjectFiles: []string{"package.json"},
	},
	{
		Name:         "npm",
		Description:  "Node package manager",
		Instruction:  "Use npm for managing Node.js packages and running scripts.",
		Kind:         KindContextual,
		ProjectFiles: []string{"package.json"},
	},
	{
		Name:         "pnpm",
		Description:  "fast disk-efficient package manager",
		Instruction:  "Use pnpm as package manager when the project uses it.",
		Kind:         KindContextual,
		ProjectFiles: []string{"package.json"},
	},
	{
		Name:         "yarn",
		Description:  "Node package manager",
		Instruction:  "Use yarn as package manager when the project uses it.",
		Kind:         KindContextual,
		ProjectFiles: []string{"package.json"},
	},
	{
		Name:         "bun",
		Description:  "JavaScript runtime, bundler, and package manager",
		Instruction:  "Use bun as JS runtime and package manager when the project uses it.",
		Kind:         KindContextual,
		ProjectFiles: []string{"package.json"},
	},
	{
		Name:         "cargo",
		Description:  "Rust package manager and build tool",
		Instruction:  "Use cargo for building, testing, and running Rust code.",
		Kind:         KindContextual,
		ProjectFiles: []string{"Cargo.toml"},
	},
	{
		Name:         "rustc",
		Description:  "Rust compiler",
		Instruction:  "Use rustc for compiling Rust code directly.",
		Kind:         KindContextual,
		ProjectFiles: []string{"Cargo.toml"},
	},
	{
		Name:         "docker",
		Description:  "container management",
		Instruction:  "Use docker for container operations and Docker Compose.",
		Kind:         KindContextual,
		ProjectFiles: []string{"Dockerfile", "compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"},
	},
}

// Detect runs lookup against every known helper and returns their statuses.
//
// WHAT:  Detects which helpers are installed on the host.
// WHY:   The prompt builder needs live availability, not config assumptions.
// HOW:   Runs lookup (typically exec.LookPath) for each known helper.
// PARAMS: lookup — binary resolution function.
// RETURNS: []Status — one entry per known helper with availability.
func Detect(lookup LookupFunc) []Status {
	statuses := make([]Status, 0, len(Known))
	for _, h := range Known {
		path, err := lookup(h.Name)
		statuses = append(statuses, Status{
			Helper:    h,
			Available: err == nil,
			Path:      path,
		})
	}
	return statuses
}

// DetectDefault detects helpers using the real OS path lookup.
//
// RETURNS: []Status — helper statuses from the host.
func DetectDefault() []Status {
	return Detect(DefaultLookup)
}

// DefaultLookup is the real OS binary resolution, overridable for tests.
var DefaultLookup LookupFunc = exec.LookPath

// ProjectRelevant reports whether a contextual helper matches the project ecosystem.
// Core helpers are always relevant. Contextual helpers are relevant only if at least
// one ProjectFile exists in workDir.
//
// WHAT:  Checks whether a contextual helper is applicable to the current project.
// WHY:   Contextual helpers like go or node shouldn't appear for unrelated projects.
// PARAMS: workDir — the current working directory; helper — the helper to check.
// RETURNS: bool — true if the helper should be shown for this project.
func ProjectRelevant(workDir string, helper Helper) bool {
	if helper.Kind == KindCore {
		return true
	}
	if len(helper.ProjectFiles) == 0 {
		return true
	}
	for _, pattern := range helper.ProjectFiles {
		path := filepath.Join(workDir, pattern)
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

// IsDeclined reports whether a helper name appears in the declined list.
//
// PARAMS: name — helper name; declined — list of declined helpers.
// RETURNS: bool — true if the helper was declined.
func IsDeclined(name string, declined []string) bool {
	for _, d := range declined {
		if d == name {
			return true
		}
	}
	return false
}

// Available returns the relevant helpers that are currently installed.
//
// WHAT:  Filters helper statuses down to available helpers relevant to the current project.
// WHY:   Prompt rendering needs a stable data set, not a formatted section.
// PARAMS: statuses — live helper detection results; workDir — current work folder.
// RETURNS: []Status — sorted available helper statuses.
func Available(statuses []Status, workDir string) []Status {
	result := make([]Status, 0, len(statuses))
	for _, s := range statuses {
		if s.Available && ProjectRelevant(workDir, s.Helper) {
			result = append(result, s)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// MissingCore returns the core helpers that are not installed and were not declined.
//
// WHAT:  Filters helper statuses down to missing core helpers that should still be shown.
// WHY:   Prompt rendering needs to know which helper names to display in the optional block.
// PARAMS: statuses — live helper detection results; setup — user helper preferences.
// RETURNS: []Status — sorted missing core helper statuses.
func MissingCore(statuses []Status, setup config.HelperSetup) []Status {
	result := make([]Status, 0, len(statuses))
	for _, s := range statuses {
		if !s.Available && s.Kind == KindCore && !IsDeclined(s.Name, setup.Declined) {
			result = append(result, s)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}
