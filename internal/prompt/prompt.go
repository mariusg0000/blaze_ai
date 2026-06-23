// prompt.go — prompt assembly from disk sources on every LLM call.
// Rebuilds the runtime prompt part in spec order: universal sysprompt, OS sysprompt,
// AGENTS.md, memory.md, skills section. Replaces {VARIABLE_NAME} placeholders at build time.
// The conversation part (session message history) is appended after the runtime part.
// Layer: prompt construction. Dependencies: internal/skills, internal/memory, internal/platform.
package prompt

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"blazeai/internal/config"
	"blazeai/internal/helpers"
	"blazeai/internal/memory"
	"blazeai/internal/platform"
	"blazeai/internal/session"
	"blazeai/internal/skills"
)

// ErrUniversalPromptMissing is returned when the universal system prompt file does not exist.
var ErrUniversalPromptMissing = fmt.Errorf("universal system prompt missing")

// ErrOSPromptMissing is returned when the OS-specific system prompt file does not exist.
var ErrOSPromptMissing = fmt.Errorf("OS system prompt missing")

// variablePattern matches {VARIABLE_NAME} placeholders in prompt and skill text.
var variablePattern = regexp.MustCompile(`\{([A-Z_][A-Z0-9_]*)\}`)

// Builder assembles the full prompt (runtime part + conversation part) from disk sources.
//
// WHAT:  Holds configuration for prompt building and assembles prompts on every LLM call.
// WHY:   The prompt is rebuilt fresh from disk every time per spec — nothing is reused.
// PARAMS: PromptsFS — filesystem containing sysprompt.md and sysprompt.<os>.md;
//
//	BuiltinSkillsFS — filesystem containing builtin skill .md files;
//	WorkDir — current work folder for AGENTS.md resolution;
//	OS — the detected operating system for selecting the OS-specific prompt;
//	OSInfo — human-readable OS description injected as {OS_INFO};
//	HelperSetup — user UX preferences for host helper installation prompts;
//	HelperLookup — binary lookup function for helper detection (injectable for tests).
type Builder struct {
	PromptsFS       fs.FS
	BuiltinSkillsFS fs.FS
	WorkDir         string
	OS              platform.OS
	OSInfo          string
	HelperSetup     config.HelperSetup
	HelperLookup    helpers.LookupFunc
}

// injectVariables replaces known variable placeholders in text with resolved values.
// Unknown placeholders are left as-is per spec.
//
// WHAT:  Replaces {VARIABLE_NAME} placeholders with concrete values.
// WHY:   Prompt and skill files use {APP_HOME} to reference the app home path without hardcoding.
// HOW:   Regex finds all {NAME} patterns; known names are replaced, unknown ones are left untouched.
// PARAMS: text — the raw text containing placeholders.
// RETURNS: string — text with known variables replaced; unknown ones preserved.
func (b *Builder) injectVariables(text string) (string, error) {
	home, err := platform.AppHome()
	if err != nil {
		return "", err
	}
	return variablePattern.ReplaceAllStringFunc(text, func(match string) string {
		name := match[1 : len(match)-1]
		switch name {
		case "APP_HOME":
			return home
		case "WORK_DIR":
			return b.WorkDir
		case "OS_INFO":
			return b.OSInfo
		default:
			return match
		}
	}), nil
}

// readFileRequiredFS reads a file from an fs.FS and returns its content.
// If the file does not exist, returns the given error.
//
// WHAT:  Reads a file from a filesystem, treating missing files as a hard error.
// WHY:   Universal and OS system prompts are required per spec and read from embedded FS.
// PARAMS: fsys — the filesystem; name — the file name; missingErr — error if absent.
// RETURNS: string — file content; error if missing or unreadable.
func readFileRequiredFS(fsys fs.FS, name string, missingErr error) (string, error) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", missingErr
		}
		return "", fmt.Errorf("cannot read %s: %w", name, err)
	}
	return string(data), nil
}

// readFileOptional reads a file from disk. If the file does not exist,
// returns an empty string and no error. Other read errors are returned.
//
// WHAT:  Reads a file from disk, treating missing files as empty (optional source).
// WHY:   AGENTS.md is an optional source on disk, not in the embedded FS.
// PARAMS: path — the file to read.
// RETURNS: string — file content or empty if missing; error on read failure (not missing).
func readFileOptional(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("cannot read %s: %w", path, err)
	}
	return string(data), nil
}

// buildSkillsSection assembles the skills section from discovered skills and the active list.
// Returns the available skills block and active skills block, or empty string if no skills exist.
//
// WHAT:  Builds the two-part skills section for the runtime prompt.
// WHY:   Spec requires available skills ([DESCRIPTION] + file names) then active skills ([DETAILS]).
// HOW:   Discovers all skills, lists available ones sorted, then injects active ones from the list.
// PARAMS: active — the in-memory active skills list for this session.
// RETURNS: string — assembled skills section; error if discovery fails.
func (b *Builder) buildSkillsSection(active *skills.ActiveList) (string, error) {
	discovered, err := skills.DiscoverFromFS(b.BuiltinSkillsFS)
	if err != nil {
		return "", fmt.Errorf("skills discovery: %w", err)
	}
	if len(discovered) == 0 {
		return "", nil
	}

	var sb strings.Builder

	// Available skills block: [DESCRIPTION] of every discovered skill + file name.
	sb.WriteString("## Available Skills\n\n")
	for _, name := range skills.SortedNames(discovered) {
		skill := discovered[name]
		sb.WriteString(fmt.Sprintf("- **%s.md**: %s\n", name, skill.Description))
	}

	// Active skills block: [DETAILS] of every active skill.
	activeNames := active.List()
	if len(activeNames) > 0 {
		sb.WriteString("\n## Active Skills\n\n")
		sb.WriteString("The following skills are loaded and active. Use their full details.\n\n")
		for _, name := range activeNames {
			skill, ok := discovered[name]
			if !ok {
				continue
			}
			sb.WriteString(fmt.Sprintf("### %s.md\n\n%s\n\n", name, skill.Details))
		}
	}

	return sb.String(), nil
}

// BuildRuntimePart assembles the runtime prompt part from all disk sources.
// Order: universal sysprompt → OS sysprompt → host helpers → AGENTS.md → memory.md → skills section.
// Variable injection is applied to every source. Required sources error if missing.
//
// WHAT:  Builds the runtime part of the prompt from disk sources.
// WHY:   The runtime part is rebuilt on every LLM call per spec.
// HOW:   Reads each source in order, injects variables, concatenates with separators.
// PARAMS: active — the in-memory active skills list for this session.
// RETURNS: string — assembled runtime part; error if required sources are missing or unreadable.
func (b *Builder) BuildRuntimePart(active *skills.ActiveList) (string, error) {
	var parts []string

	// 1. Universal system prompt (required).
	universal, err := readFileRequiredFS(b.PromptsFS, "sysprompt.md", ErrUniversalPromptMissing)
	if err != nil {
		return "", err
	}
	universal, err = b.injectVariables(universal)
	if err != nil {
		return "", err
	}
	parts = append(parts, universal)

	// 2. OS-specific system prompt (required).
	osPromptName := fmt.Sprintf("sysprompt.%s.md", b.OS)
	osPrompt, err := readFileRequiredFS(b.PromptsFS, osPromptName, ErrOSPromptMissing)
	if err != nil {
		return "", err
	}
	osPrompt, err = b.injectVariables(osPrompt)
	if err != nil {
		return "", err
	}
	parts = append(parts, osPrompt)

	// 3. Host helpers (optional).
	lookup := b.HelperLookup
	if lookup == nil {
		lookup = helpers.DefaultLookup
	}
	helperStatuses := helpers.Detect(lookup)
	helperSection := helpers.BuildPromptSection(helperStatuses, b.WorkDir, b.HelperSetup)
	if helperSection != "" {
		helperSection, err = b.injectVariables(helperSection)
		if err != nil {
			return "", err
		}
		parts = append(parts, helperSection)
	}

	// 4. AGENTS.md from work folder (optional).
	agents, err := readFileOptional(filepath.Join(b.WorkDir, "AGENTS.md"))
	if err != nil {
		return "", err
	}
	if agents != "" {
		agents, err = b.injectVariables(agents)
		if err != nil {
			return "", err
		}
		parts = append(parts, "## Project Rules (AGENTS.md)\n\nThe following rules are loaded from the AGENTS.md file in the current working directory. Follow them for all work in this project.\n\n"+agents)
	}

	// 5. Memory (optional).
	mem, err := memory.Read()
	if err != nil {
		return "", err
	}
	if mem != "" {
		mem, err = b.injectVariables(mem)
		if err != nil {
			return "", err
		}
		memHeader := "## Memory (memory.md)\n\nThe following is the content of the persistent memory file {APP_HOME}/memory/memory.md. It is injected automatically on every prompt build. Do not use shell to read memory.\n\n"
		memHeader, err = b.injectVariables(memHeader)
		if err != nil {
			return "", err
		}
		parts = append(parts, memHeader+mem)
	}

	// 6. Skills section (optional).
	skillsSection, err := b.buildSkillsSection(active)
	if err != nil {
		return "", err
	}
	if skillsSection != "" {
		skillsSection, err = b.injectVariables(skillsSection)
		if err != nil {
			return "", err
		}
		parts = append(parts, skillsSection)
	}

	return strings.Join(parts, "\n\n---\n\n"), nil
}

// Build assembles the full prompt: runtime part + conversation part from the session.
// The runtime part is rebuilt from disk; the conversation part is the session's message history.
//
// WHAT:  Produces the complete message array to send to the LLM.
// WHY:   Every LLM call needs the full prompt assembled fresh per spec.
// HOW:   Builds runtime part as a system message, appends session messages.
// PARAMS: sess — the current session with message history; active — active skills list.
// RETURNS: []session.Message — full message array ready for the LLM; error if build fails.
func (b *Builder) Build(sess *session.Session, active *skills.ActiveList) ([]session.Message, error) {
	runtimePart, err := b.BuildRuntimePart(active)
	if err != nil {
		return nil, err
	}

	messages := make([]session.Message, 0, len(sess.Messages)+1)
	messages = append(messages, session.Message{
		Role:    "system",
		Content: runtimePart,
	})
	messages = append(messages, sess.Messages...)

	return messages, nil
}
