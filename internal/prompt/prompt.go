// prompt.go — prompt assembly from disk sources on every LLM call.
// Rebuilds the runtime prompt part in spec order: universal sysprompt, OS sysprompt,
// host helpers, skills section, memories, AGENTS.md.
// Replaces {VARIABLE_NAME} placeholders at build time.
// The conversation part (session message history) is appended after the runtime part.
// Layer: prompt construction. Dependencies: internal/skills, internal/memories,
// internal/platform.
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
	"blazeai/internal/memories"
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

// buildMemoriesSection assembles the memories section from custom memories and the active list.
// Returns the available memories block and active memories block, or empty string if none exist.
//
// WHAT:  Builds the two-part memories section for the runtime prompt.
// WHY:   Memories provide on-demand contextual knowledge separate from skills.
// HOW:   Discovers all custom memories, lists available ones sorted, then injects active ones from the list.
// PARAMS: active — the in-memory active memory list for this session.
// RETURNS: string — assembled memories section; error if discovery fails.
func (b *Builder) buildMemoriesSection(active *memories.ActiveList) (string, error) {
	discovered, err := memories.Discover()
	if err != nil {
		return "", fmt.Errorf("memories discovery: %w", err)
	}
	if len(discovered) == 0 {
		return "", nil
	}

	var sb strings.Builder

	// Available memories block: [DESCRIPTION] of every discovered memory + file name.
	sb.WriteString("## Available Memories\n\n")
	for _, name := range memories.SortedNames(discovered) {
		memory := discovered[name]
		description, err := b.injectVariables(memory.Description)
		if err != nil {
			return "", err
		}
		sb.WriteString(fmt.Sprintf("- **%s.md**: %s\n", name, description))
	}
	sb.WriteString("\nOnly memories listed under `## Active Memories` are active right now. Do not infer current active memories from older `load_memory` or `unload_memory` tool results in the conversation history. If there is no `## Active Memories` section below, then no memories are currently active.\n")

	// Active memories block: [DETAILS] of every active memory.
	activeNames := active.List()
	if len(activeNames) > 0 {
		sb.WriteString("\n## Active Memories\n\n")
		sb.WriteString("The following memories are loaded and active. Use their full details.\n\n")
		for _, name := range activeNames {
			memory, ok := discovered[name]
			if !ok {
				continue
			}
			details, err := b.injectVariables(memory.Details)
			if err != nil {
				return "", err
			}
			sb.WriteString(fmt.Sprintf("### %s.md\n\n%s\n\n", name, details))
		}
	}

	return sb.String(), nil
}

// BuildRuntimePart assembles the runtime prompt part from all disk sources.
// Order: universal sysprompt → OS sysprompt → host helpers → skills section → memories → AGENTS.md.
// Variable injection is applied to every source. Required sources error if missing.
//
// WHAT:  Builds the runtime part of the prompt from disk sources.
// WHY:   The runtime part is rebuilt on every LLM call per spec.
// HOW:   Reads each source in order, injects variables, concatenates with separators.
// PARAMS: activeSkills — active skills list; activeMemories — active memory list.
// RETURNS: string — assembled runtime part; error if required sources are missing or unreadable.
func (b *Builder) BuildRuntimePart(activeSkills *skills.ActiveList, activeMemories *memories.ActiveList) (string, error) {
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

	// 4. Skills section (optional).
	skillsSection, err := b.buildSkillsSection(activeSkills)
	if err != nil {
		return "", err
	}
	if skillsSection != "" {
		parts = append(parts, skillsSection)
	}

	// 5. Memories section (optional).
	memoriesSection, err := b.buildMemoriesSection(activeMemories)
	if err != nil {
		return "", err
	}
	if memoriesSection != "" {
		parts = append(parts, memoriesSection)
	}

	// 6. AGENTS.md from work folder (optional).
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
// activeMemories — active memory list.
// RETURNS: []session.Message — full message array ready for the LLM; error if build fails.
func (b *Builder) Build(sess *session.Session, activeSkills *skills.ActiveList, activeMemories *memories.ActiveList) ([]session.Message, error) {
	runtimePart, err := b.BuildRuntimePart(activeSkills, activeMemories)
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
