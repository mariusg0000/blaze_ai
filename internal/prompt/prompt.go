// prompt.go — prompt assembly from disk sources on every LLM call.
// Rebuilds the runtime prompt part in spec order: universal sysprompt, OS sysprompt,
// host helpers, skills section, AGENTS.md.
// Replaces {VARIABLE_NAME} placeholders at build time.
// The conversation part (session message history) is appended after the runtime part.
// Layer: prompt construction. Dependencies: internal/skills, internal/platform.
package prompt

import (
	"errors"
	"fmt"
	"html"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"blazeai/internal/config"
	"blazeai/internal/helpers"
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
//	WorkDir — current work folder for AGENTS.md and project skill discovery;
//	OS — the detected operating system for selecting the OS-specific prompt;
//	OSInfo — human-readable OS description injected as {OS_INFO};
//	HelperSetup — user UX preferences for host helper installation prompts;
//	HelperLookup — binary lookup function for helper detection (injectable for tests).
type Builder struct {
	PromptsFS       fs.FS
	WorkDir         string
	OS              platform.OS
	OSInfo          string
	HelperSetup     config.HelperSetup
	HelperLookup    helpers.LookupFunc
}

// injectVariables replaces known variable placeholders in text with resolved values.
func (b *Builder) injectVariables(text string) (string, error) {
	return b.injectTemplateVariables(text, nil, "")
}

// injectVariablesForSkill replaces known placeholders with optional SKILL_DIR resolution.
func (b *Builder) injectVariablesForSkill(text, skillDir string) (string, error) {
	return b.injectTemplateVariables(text, nil, skillDir)
}

// injectTemplateVariables replaces built-in and template-specific placeholders in text.
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
		case "GLOBAL_SKILLS_DIR":
			return filepath.Join(home, "skills")
		case "PROJECT_SKILLS_DIR":
			projectDir, err := platform.ProjectDir(b.WorkDir)
			if err != nil {
				return "NULL"
			}
			return filepath.Join(projectDir, "skills")
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

// readFileRequiredFS reads a file from an fs.FS. Missing files return the given error.
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

// readFileOptional reads a file from disk. Missing files return empty string.
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

// buildSkillsSection assembles skill data from discovered skills and the active list.
// Discovered skills are rendered as XML for clear boundary handling by the LLM.
// Available skills: <available_skills><skill><name/><description/></skill></available_skills>
// Active skills:   <active_skills><skill><name/><behavior/><data/></skill></active_skills>
// Behavior and data content is wrapped in CDATA to preserve literal text (&&, <, >, etc.).
func (b *Builder) buildSkillsSection(active *skills.ActiveList) (string, string, error) {
	discovered, err := skills.DiscoverAll(b.WorkDir)
	if err != nil {
		return "", "", fmt.Errorf("skills discovery: %w", err)
	}
	if len(discovered) == 0 {
		return "", "", nil
	}

	// Available skills as XML list.
	var avail strings.Builder
	avail.WriteString("<available_skills>\n")
	for _, id := range skills.SortedNames(discovered) {
		skill := discovered[id]
		desc, err := b.injectVariablesForSkill(skill.Description, skill.Dir)
		if err != nil {
			return "", "", err
		}
		displayName := strings.TrimPrefix(id, "global/")
		avail.WriteString("  <skill>\n")
		avail.WriteString(fmt.Sprintf("    <name>%s</name>\n", html.EscapeString(displayName)))
		avail.WriteString(fmt.Sprintf("    <description>%s</description>\n", html.EscapeString(desc)))
		avail.WriteString("  </skill>\n")
	}
	avail.WriteString("</available_skills>")

	// Active skills as XML with CDATA content blocks.
	activeNames := active.List()
	activeContent := ""
	if len(activeNames) > 0 {
		var sb strings.Builder
		sb.WriteString("<active_skills>\n")
		for _, id := range activeNames {
			skill, ok := discovered[id]
			if !ok {
				continue
			}
			name := strings.TrimPrefix(id, "global/")
			sb.WriteString(fmt.Sprintf("  <skill>\n    <name>%s</name>\n", html.EscapeString(name)))

			if skill.Behavior != "" {
				behavior, err := b.injectVariablesForSkill(skill.Behavior, skill.Dir)
				if err != nil {
					return "", "", err
				}
				sb.WriteString("    <behavior><![CDATA[")
				sb.WriteString(behavior)
				sb.WriteString("]]></behavior>\n")
			}

			if skill.Data != "" {
				data, err := b.injectVariablesForSkill(skill.Data, skill.Dir)
				if err != nil {
					return "", "", err
				}
				sb.WriteString("    <data><![CDATA[")
				sb.WriteString(data)
				sb.WriteString("]]></data>\n")
			}

			sb.WriteString("  </skill>\n")
		}
		sb.WriteString("</active_skills>")
		activeContent = sb.String()
	}

	return avail.String(), activeContent, nil
}

// buildHostHelpersSection assembles the host helpers data from live detection.
// Returns XML-wrapped strings for injection into the prompt template.
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
		var sb strings.Builder
		sb.WriteString("<host_helpers_optional>\n")
		sb.WriteString("Some useful cross-platform host utilities are missing:\n")
		for _, s := range missingCore {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", html.EscapeString(s.Name), html.EscapeString(s.Description)))
		}
		sb.WriteString("If one would materially help the current task, explain the benefit and ask the user before installing anything.\n")
		sb.WriteString("For installation guidance, load the `setup_helpers` skill.\n")
		sb.WriteString("</host_helpers_optional>")
		optionalSection = sb.String()
	}
	if len(available) == 0 {
		if optionalSection == "" {
			return "", "", nil
		}
		return "", optionalSection, nil
	}

	var sb strings.Builder
	sb.WriteString("<host_helpers_available>\n")
	for _, s := range available {
		sb.WriteString(fmt.Sprintf("  <helper><name>%s</name><description>%s</description></helper>\n",
			html.EscapeString(s.Name), html.EscapeString(s.Description)))
	}
	sb.WriteString("</host_helpers_available>")
	return sb.String(), optionalSection, nil
}

// buildHostHelpersAdvisory returns an XML-wrapped advisory message about host helper verification.
func (b *Builder) buildHostHelpersAdvisory() string {
	if b.HelperSetup.Dismissed {
		return ""
	}
	msg := "Host environment helpers have not been verified yet. When a task could benefit from faster file search, data processing, or other system tools, suggest to the user that you can check and set them up. Load the `setup_helpers` skill for guidance. Once all helpers are verified or the user declines, this reminder will stop appearing."
	return fmt.Sprintf("<host_helpers_advisory>%s</host_helpers_advisory>", html.EscapeString(msg))
}

// BuildRuntimePart assembles the runtime prompt part from all disk sources.
// Order: universal → OS → helpers → skills → AGENTS.md.
func (b *Builder) BuildRuntimePart(activeSkills *skills.ActiveList) (string, error) {
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

	// 4. Skills section (optional, includes project-scoped).
	skillsAvailable, skillsActive, err := b.buildSkillsSection(activeSkills)
	if err != nil {
		return "", err
	}

	// 5. AGENTS.md from work folder (optional).
	agents, err := readFileOptional(filepath.Join(b.WorkDir, "AGENTS.md"))
	if err != nil {
		return "", err
	}
	if agents != "" {
		agents, err = b.injectVariables(agents)
		if err != nil {
			return "", err
		}
		agents = fmt.Sprintf("<agents_content><![CDATA[\n%s\n]]></agents_content>", agents)
	}

	rendered, err := b.injectTemplateVariables(universal, map[string]string{
		"OS_PROMPT":              strings.TrimSpace(osPrompt),
		"HOST_HELPERS_ADVISORY":  strings.TrimSpace(helperAdvisory),
		"HOST_HELPERS_AVAILABLE": strings.TrimSpace(helperAvailable),
		"HOST_HELPERS_OPTIONAL":  strings.TrimSpace(helperOptional),
		"SKILLS_AVAILABLE":       strings.TrimSpace(skillsAvailable),
		"SKILLS_ACTIVE":          strings.TrimSpace(skillsActive),
		"AGENTS_CONTENT":         strings.TrimSpace(agents),
	}, "")
	if err != nil {
		return "", err
	}
	return rendered, nil
}

// Build assembles the full prompt: runtime part + conversation part from the session.
func (b *Builder) Build(sess *session.Session, activeSkills *skills.ActiveList) ([]session.Message, error) {
	runtimePart, err := b.BuildRuntimePart(activeSkills)
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
