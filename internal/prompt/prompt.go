// prompt.go — prompt assembly from disk sources on every LLM call.
// Rebuilds the runtime prompt part in spec order: universal sysprompt, OS sysprompt,
// host helpers, memory.md, skills section, memory-banks, AGENTS.md.
// Replaces {VARIABLE_NAME} placeholders at build time.
// The conversation part (session message history) is appended after the runtime part.
// Layer: prompt construction. Dependencies: internal/skills, internal/memory,
// internal/memorybanks, internal/platform.
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
	"blazeai/internal/memorybanks"
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
	return b.injectVariablesForSkill(text, "")
}

// injectVariablesForSkill replaces known placeholders in prompt and skill text.
// {SKILL_DIR} is resolved only when a concrete skill directory is provided.
func (b *Builder) injectVariablesForSkill(text, skillDir string) (string, error) {
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
		case "SKILL_DIR":
			if skillDir != "" {
				return skillDir
			}
			return match
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
		description, err := b.injectVariablesForSkill(skill.Description, skill.Dir)
		if err != nil {
			return "", err
		}
		sb.WriteString(fmt.Sprintf("- **%s.md**: %s\n", name, description))
	}
	sb.WriteString("\nOnly skills listed under `## Active Skills` are active right now. Do not infer current active skills from older `load_skill` or `unload_skill` tool results in the conversation history. If there is no `## Active Skills` section below, then no skills are currently active.\n")

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
			details, err := b.injectVariablesForSkill(skill.Details, skill.Dir)
			if err != nil {
				return "", err
			}
			sb.WriteString(fmt.Sprintf("### %s.md\n\n%s\n\n", name, details))
		}
	}

	return sb.String(), nil
}

// buildMemoryBanksSection assembles the memory-bank section from custom memory-banks and the active list.
// Returns the available memory-banks block and active memory-banks block, or empty string if none exist.
//
// WHAT:  Builds the two-part memory-bank section for the runtime prompt.
// WHY:   Memory-banks provide on-demand contextual knowledge separate from skills.
// HOW:   Discovers all custom memory-banks, lists available ones sorted, then injects active ones from the list.
// PARAMS: active — the in-memory active memory-bank list for this session.
// RETURNS: string — assembled memory-bank section; error if discovery fails.
func (b *Builder) buildMemoryBanksSection(active *memorybanks.ActiveList) (string, error) {
	discovered, err := memorybanks.Discover()
	if err != nil {
		return "", fmt.Errorf("memory-banks discovery: %w", err)
	}
	if len(discovered) == 0 {
		return "", nil
	}

	var sb strings.Builder

	// Available memory-banks block: [DESCRIPTION] of every discovered memory-bank + file name.
	sb.WriteString("## Available Memory Banks\n\n")
	for _, name := range memorybanks.SortedNames(discovered) {
		bank := discovered[name]
		description, err := b.injectVariables(bank.Description)
		if err != nil {
			return "", err
		}
		sb.WriteString(fmt.Sprintf("- **%s.md**: %s\n", name, description))
	}
	sb.WriteString("\nOnly memory-banks listed under `## Active Memory Banks` are active right now. Do not infer current active memory-banks from older `load_memory_bank` or `unload_memory_bank` tool results in the conversation history. If there is no `## Active Memory Banks` section below, then no memory-banks are currently active.\n")

	// Active memory-banks block: [DETAILS] of every active memory-bank.
	activeNames := active.List()
	if len(activeNames) > 0 {
		sb.WriteString("\n## Active Memory Banks\n\n")
		sb.WriteString("The following memory-banks are loaded and active. Use their full details.\n\n")
		for _, name := range activeNames {
			bank, ok := discovered[name]
			if !ok {
				continue
			}
			details, err := b.injectVariables(bank.Details)
			if err != nil {
				return "", err
			}
			sb.WriteString(fmt.Sprintf("### %s.md\n\n%s\n\n", name, details))
		}
	}

	return sb.String(), nil
}

// BuildRuntimePart assembles the runtime prompt part from all disk sources.
// Order: universal sysprompt → OS sysprompt → host helpers → memory.md → skills section → memory-banks → AGENTS.md.
// Variable injection is applied to every source. Required sources error if missing.
//
// WHAT:  Builds the runtime part of the prompt from disk sources.
// WHY:   The runtime part is rebuilt on every LLM call per spec.
// HOW:   Reads each source in order, injects variables, concatenates with separators.
// PARAMS: activeSkills — active skills list; activeMemoryBanks — active memory-bank list.
// RETURNS: string — assembled runtime part; error if required sources are missing or unreadable.
func (b *Builder) BuildRuntimePart(activeSkills *skills.ActiveList, activeMemoryBanks *memorybanks.ActiveList) (string, error) {
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

	// 4. Memory (optional).
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
	skillsSection, err := b.buildSkillsSection(activeSkills)
	if err != nil {
		return "", err
	}
	if skillsSection != "" {
		parts = append(parts, skillsSection)
	}

	// 7. Memory-banks section (optional).
	memoryBanksSection, err := b.buildMemoryBanksSection(activeMemoryBanks)
	if err != nil {
		return "", err
	}
	if memoryBanksSection != "" {
		parts = append(parts, memoryBanksSection)
	}

	// 8. AGENTS.md from work folder (optional).
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

	return strings.Join(parts, "\n\n---\n\n"), nil
}

// Build assembles the full prompt: runtime part + conversation part from the session.
// The runtime part is rebuilt from disk; the conversation part is the session's message history.
//
// WHAT:  Produces the complete message array to send to the LLM.
// WHY:   Every LLM call needs the full prompt assembled fresh per spec.
// HOW:   Builds runtime part as a system message, appends session messages.
// PARAMS: sess — the current session with message history; activeSkills — active skills list;
// activeMemoryBanks — active memory-bank list.
// RETURNS: []session.Message — full message array ready for the LLM; error if build fails.
func (b *Builder) Build(sess *session.Session, activeSkills *skills.ActiveList, activeMemoryBanks *memorybanks.ActiveList) ([]session.Message, error) {
	runtimePart, err := b.BuildRuntimePart(activeSkills, activeMemoryBanks)
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
