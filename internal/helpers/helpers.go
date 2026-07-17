package helpers

import (
	"os/exec"
	"sort"

	"blazeai/internal/config"
)

// LookupFunc is the signature for binary lookup, injectable for tests.
type LookupFunc func(name string) (string, error)

// Helper defines one cross-platform host utility.
//
// WHAT:  Static catalog entry for a host utility.
// WHY:   The runtime needs to know which helpers exist, what they do, and when to show them.
// PARAMS: Name — binary name; Description — short summary; Instruction — usage instruction for the prompt;
type Helper struct {
	Name        string
	Description string
	Instruction string
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
		Description: "fast recursive code and text search",
		Instruction: "Use rg for fast recursive content search instead of broad grep/find pipelines.",
	},
	{
		Name:        "fd",
		Description: "fast file and directory discovery",
		Instruction: "Use fd for fast file and directory discovery.",
	},
	{
		Name:        "jq",
		Description: "query, filter, and transform JSON",
		Instruction: "Use jq for JSON inspection and transformation.",
	},
	{
		Name:        "git",
		Description: "repository inspection and version control",
		Instruction: "Use git for repository inspection and version-control operations.",
	},
	{
		Name:        "xh",
		Description: "HTTP requests with JSON shorthand (Rust, cross-platform curl alternative)",
		Instruction: "Use xh for HTTP and API requests.",
	},
	{
		Name:        "pandoc",
		Description: "convert between document formats (MD, HTML, PDF, DOCX)",
		Instruction: "Use pandoc for converting between document formats (Markdown, HTML, PDF, DOCX, LaTeX).",
	},
	{
		Name:        "sqlite3",
		Description: "query and inspect SQLite databases",
		Instruction: "Use sqlite3 for querying SQLite databases and quick data inspection.",
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
func Available(statuses []Status, _ string) []Status {
	result := make([]Status, 0, len(statuses))
	for _, s := range statuses {
		if s.Available {
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
		if !s.Available && !IsDeclined(s.Name, setup.Declined) {
			result = append(result, s)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}
