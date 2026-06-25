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
	return b.injectTemplateVariables(text, nil, "")
}

// injectVariablesForSkill replaces known placeholders in prompt and skill text.
// {SKILL_DIR} is resolved only when a concrete skill directory is provided.
func (b *Builder) injectVariablesForSkill(text, skillDir string) (string, error) {
	return b.injectTemplateVariables(text, nil, skillDir)
}

// injectTemplateVariables replaces built-in and template-specific placeholders in text.
// Unknown placeholders are left as-is per spec, and escaped braces (\{, \}) are restored
// as literal braces after interpolation.
//
// WHAT:  Replaces {VARIABLE_NAME} placeholders with concrete values.
// WHY:   Prompt fragments need both built-in variables and section-specific injected text.
// HOW:   Escaped braces are protected first; then regex finds all {NAME} patterns and resolves
// built-in values before extra values; protected braces are restored at the end.
// PARAMS: text — the raw text containing placeholders; extra — section-specific replacements;
//
//	skillDir — concrete skill directory for {SKILL_DIR}.
//
// RETURNS: string — text with known variables replaced; unknown ones preserved.
func (b *Builder) injectTemplateVariables(text string, extra map[string]string, skillDir string) (string, error) {
	home, err := platform.AppHome()
	if err != nil {
		return "", err
	}
	const (
		leftBraceEscape  = "__BLAZEAI_ESC_LBRACE__"
		rightBraceEscape = "__BLAZEAI_ESC_RBRACE__"
	)
	text = strings.NewReplacer(`\{`, leftBraceEscape, `\}`, rightBraceEscape).Replace(text)
	result := variablePattern.ReplaceAllStringFunc(text, func(match string) string {
		name := match[1 : len(match)-1]
		if extra != nil {
			if value, ok := extra[name]; ok {
				if value == "" {
					return "NULL"
				}
				return value
			}
		}
		switch name {
		case "APP_HOME":
			if home == "" {
				return "NULL"
			}
			return home
		case "WORK_DIR":
			if b.WorkDir == "" {
				return "NULL"
			}
			return b.WorkDir
		case "OS_INFO":
			if b.OSInfo == "" {
				return "NULL"
			}
			return b.OSInfo
		case "SKILL_DIR":
			if skillDir != "" {
				return skillDir
			}
			return "NULL"
		default:
			return match
		}
	})
	result = strings.NewReplacer(leftBraceEscape, `{`, rightBraceEscape, `}`).Replace(result)
	return result, nil
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

// pruneEmptySection removes a section block when its content is empty.
//
// WHAT:  Drops a section and its surrounding blank-line padding when the section has no content.
// WHY:   sysprompt.md owns the labels, but empty sections should not remain visible.
// HOW:   Removes the exact block between two known section markers when needed.
// PARAMS: text — rendered prompt text; startMarker — section start including leading blank lines;
// endMarker — next section start including leading blank lines, or empty for EOF; empty — whether to remove.
// RETURNS: string — prompt with the empty section removed.
func pruneEmptySection(text, startMarker, endMarker string, empty bool) string {
	if !empty {
		return text
	}
	start := strings.Index(text, startMarker)
	if start == -1 {
		return text
	}
	if endMarker == "" {
		return text[:start]
	}
	searchFrom := start + len(startMarker)
	endRel := strings.Index(text[searchFrom:], endMarker)
	if endRel == -1 {
		return text
	}
	end := searchFrom + endRel
	return text[:start] + text[end:]
}

// buildSkillsSection assembles the skills data from discovered skills and the active list.
// Returns empty strings for missing pieces.
//
// WHAT:  Builds the skills data for the runtime prompt.
// WHY:   Section labels live in sysprompt.md while the content is injected dynamically.
// HOW:   Discovers all skills and renders available/active text blocks separately.
// PARAMS: active — the in-memory active skills list for this session.
// RETURNS: string, string — available block and active block; error if discovery fails.
func (b *Builder) buildSkillsSection(active *skills.ActiveList) (string, string, error) {
	discovered, err := skills.DiscoverFromFS(b.BuiltinSkillsFS)
	if err != nil {
		return "", "", fmt.Errorf("skills discovery: %w", err)
	}
	if len(discovered) == 0 {
		return "", "", nil
	}

	available := make([]string, 0, len(discovered))
	for _, name := range skills.SortedNames(discovered) {
		skill := discovered[name]
		description, err := b.injectVariablesForSkill(skill.Description, skill.Dir)
		if err != nil {
			return "", "", err
		}
		available = append(available, fmt.Sprintf("- **%s.md**: %s", name, description))
	}

	activeNames := active.List()
	activeDetails := ""
	if len(activeNames) > 0 {
		var sb strings.Builder
		for _, name := range activeNames {
			skill, ok := discovered[name]
			if !ok {
				continue
			}
			details, err := b.injectVariablesForSkill(skill.Details, skill.Dir)
			if err != nil {
				return "", "", err
			}
			sb.WriteString(fmt.Sprintf("### %s.md\n\n%s\n\n", name, details))
		}
		activeDetails = strings.TrimSpace(sb.String())
	}

	return strings.Join(available, "\n"), activeDetails, nil
}

// buildMemoriesSection assembles the memories data from custom memories and the active list.
// Returns empty strings for missing pieces.
//
// WHAT:  Builds the memories data for the runtime prompt.
// WHY:   Section labels live in sysprompt.md while the content is injected dynamically.
// HOW:   Discovers all custom memories and renders available/active text blocks separately.
// PARAMS: active — the in-memory active memory list for this session.
// RETURNS: string, string — available block and active block; error if discovery fails.
func (b *Builder) buildMemoriesSection(active *memories.ActiveList) (string, string, error) {
	discovered, err := memories.Discover()
	if err != nil {
		return "", "", fmt.Errorf("memories discovery: %w", err)
	}
	if len(discovered) == 0 {
		return "", "", nil
	}

	available := make([]string, 0, len(discovered))
	for _, name := range memories.SortedNames(discovered) {
		memory := discovered[name]
		description, err := b.injectVariables(memory.Description)
		if err != nil {
			return "", "", err
		}
		available = append(available, fmt.Sprintf("- **%s.md**: %s", name, description))
	}

	activeNames := active.List()
	activeDetails := ""
	if len(activeNames) > 0 {
		var sb strings.Builder
		for _, name := range activeNames {
			memory, ok := discovered[name]
			if !ok {
				continue
			}
			details, err := b.injectVariables(memory.Details)
			if err != nil {
				return "", "", err
			}
			sb.WriteString(fmt.Sprintf("### %s.md\n\n%s\n\n", name, details))
		}
		activeDetails = strings.TrimSpace(sb.String())
	}

	return strings.Join(available, "\n"), activeDetails, nil
}

// buildHostHelpersSection assembles the host helpers data from live detection.
//
// WHAT:  Builds the host helper data for the runtime prompt.
// WHY:   Section labels live in sysprompt.md while the content is injected dynamically.
// HOW:   Detects live helpers and renders available/optional text blocks separately.
// PARAMS: statuses — live helper detection results.
// RETURNS: string, string — available block and optional block; empty strings when nothing is useful to display.
func (b *Builder) buildHostHelpersSection(statuses []helpers.Status) (string, string, error) {
	available := helpers.Available(statuses, b.WorkDir)
	missingCore := helpers.MissingCore(statuses, b.HelperSetup)

	if len(available) == 0 && len(missingCore) == 0 {
		return "", "", nil
	}
	if len(available) == 0 && b.HelperSetup.Dismissed {
		return "", "", nil
	}

	optionalSection := ""
	if len(missingCore) > 0 && !b.HelperSetup.Dismissed {
		missingLines := make([]string, 0, len(missingCore))
		for _, s := range missingCore {
			missingLines = append(missingLines, fmt.Sprintf("- **%s**: %s", s.Name, s.Description))
		}
		optionalSection = strings.Join([]string{
			"Some useful cross-platform host utilities are missing:",
			strings.Join(missingLines, "\n"),
			"If one would materially help the current task, explain the benefit and ask the user before installing anything.",
			"For installation guidance, load the `setup_helpers` skill.",
		}, "\n\n")
	}
	if len(available) == 0 {
		if optionalSection == "" {
			return "", "", nil
		}
		return "", strings.TrimSpace(optionalSection), nil
	}

	availableLines := make([]string, 0, len(available))
	for _, s := range available {
		availableLines = append(availableLines, fmt.Sprintf("- **%s**: %s", s.Name, s.Description))
	}
	return strings.Join(availableLines, "\n"), strings.TrimSpace(optionalSection), nil
}

// buildHostHelpersAdvisory returns an advisory message to remind the LLM to verify host helpers
// when helper setup has not been dismissed. Returns an empty string when dismissed is true.
//
// WHAT:  Builds a persistent reminder to verify host helpers until the user dismisses it.
// WHY:   The LLM should proactively suggest helper verification; the reminder disappears
//
//	only when the setup_helpers skill marks it complete.
//
// RETURNS: string — advisory text or empty string when dismissed.
func (b *Builder) buildHostHelpersAdvisory() string {
	if b.HelperSetup.Dismissed {
		return ""
	}
	return "Host environment helpers have not been verified yet. When a task could benefit from faster file search, data processing, or other system tools, suggest to the user that you can check and set them up. Load the `setup_helpers` skill for guidance. Once all helpers are verified or the user declines, this reminder will stop appearing."
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
	// 1. Universal system prompt (required).
	universal, err := readFileRequiredFS(b.PromptsFS, "sysprompt.md", ErrUniversalPromptMissing)
	if err != nil {
		return "", err
	}

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

	// 3. Host helpers (optional).
	lookup := b.HelperLookup
	if lookup == nil {
		lookup = helpers.DefaultLookup
	}
	helperAdvisory := b.buildHostHelpersAdvisory()
	helperStatuses := helpers.Detect(lookup)
	helperAvailable, helperOptional, err := b.buildHostHelpersSection(helperStatuses)
	if err != nil {
		return "", err
	}

	// 4. Skills section (optional).
	skillsAvailable, skillsActive, err := b.buildSkillsSection(activeSkills)
	if err != nil {
		return "", err
	}

	// 5. Memories section (optional).
	memoriesAvailable, memoriesActive, err := b.buildMemoriesSection(activeMemories)
	if err != nil {
		return "", err
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
	}

	rendered, err := b.injectTemplateVariables(universal, map[string]string{
		"OS_PROMPT":              strings.TrimSpace(osPrompt),
		"HOST_HELPERS_ADVISORY":  strings.TrimSpace(helperAdvisory),
		"HOST_HELPERS_AVAILABLE": strings.TrimSpace(helperAvailable),
		"HOST_HELPERS_OPTIONAL":  strings.TrimSpace(helperOptional),
		"SKILLS_AVAILABLE":       strings.TrimSpace(skillsAvailable),
		"SKILLS_ACTIVE":          strings.TrimSpace(skillsActive),
		"MEMORIES_AVAILABLE":     strings.TrimSpace(memoriesAvailable),
		"MEMORIES_ACTIVE":        strings.TrimSpace(memoriesActive),
		"AGENTS_CONTENT":         strings.TrimSpace(agents),
	}, "")
	if err != nil {
		return "", err
	}
	return rendered, nil
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
