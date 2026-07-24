// migration.go — Legacy agent and mode migration helpers.
// Replaces kind: one-shot with type: executor and converts modes.json entries
// into interactive Markdown definitions.
// Layer: agent definitions. Dependencies: internal/config.
package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"blazeai/internal/config"
)

// MigrateLegacyDefinitions replaces the exact front-matter line `kind: one-shot`
// with `type: executor` in every .md file in agentsDir that contains it.
// WHAT: One-time in-place migration of legacy kind syntax to the new type syntax.
// HOW: Reads each file, scans for the exact front-matter `kind: one-shot` line,
//
//	replaces it with `type: executor`, and writes the file back. Files without
//	the legacy line are skipped. Any other `kind:` value is an error.
//
// Propagates all filesystem and read/write errors.
func MigrateLegacyDefinitions(agentsDir string) error {
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("cannot read agents directory %s: %w", agentsDir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			continue
		}
		path := filepath.Join(agentsDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("cannot read %s: %w", path, err)
		}
		content := string(data)
		// Look for the exact legacy kind line in front matter.
		if !strings.HasPrefix(content, "---\n") {
			continue
		}
		endIdx := strings.Index(content[4:], "\n---\n")
		if endIdx < 0 {
			continue
		}
		frontMatter := content[4 : 4+endIdx]
		lines := strings.Split(frontMatter, "\n")
		modified := false
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "kind: one-shot" {
				lines[i] = "type: executor"
				modified = true
			} else if strings.HasPrefix(trimmed, "kind:") && trimmed != "kind: one-shot" {
				return fmt.Errorf("unsupported legacy kind value in %s: %s", path, trimmed)
			}
		}
		if !modified {
			continue
		}
		newFrontMatter := strings.Join(lines, "\n")
		newContent := "---\n" + newFrontMatter + "\n---\n" + content[4+endIdx+5:]
		if err := os.WriteFile(path, []byte(newContent), 0600); err != nil {
			return fmt.Errorf("cannot write migrated %s: %w", path, err)
		}
	}
	return nil
}

// MigrateLegacyModes creates interactive Markdown definitions for legacy modes
// that do not already have a corresponding .md file.
// WHAT: Generates one .md file per legacy mode that is not already present.
// HOW: For each mode, builds a valid front-matter definition with type: interactive,
//
//	model, tools (baseToolNames minus DeniedTools, agent_done, and run_agent),
//	agents from mode.Agents, and directive with newlines converted to spaces.
//	Writes deterministically sorted tool output.
//
// Never overwrites existing files. Rejects unsafe mode names containing path
// separators or equal to "." or "..". Propagates all write errors.
func MigrateLegacyModes(agentsDir string, legacy *config.ModesConfig, baseToolNames []string) error {
	if legacy == nil {
		return nil
	}
	// Pre-sort baseToolNames for deterministic output.
	sortedBase := make([]string, len(baseToolNames))
	copy(sortedBase, baseToolNames)
	sort.Strings(sortedBase)

	// Build denied tool set for fast lookup.
	denied := make(map[string]bool)
	for _, mode := range legacy.Modes {
		for _, dt := range mode.DeniedTools {
			denied[dt] = true
		}
	}

	for _, mode := range legacy.Modes {
		// Reject unsafe mode names.
		if mode.Name == "." || mode.Name == ".." || strings.ContainsAny(mode.Name, "/\\") {
			return fmt.Errorf("unsafe mode name %q: cannot create agent file", mode.Name)
		}
		targetPath := filepath.Join(agentsDir, mode.Name+".md")
		// Never overwrite an existing file.
		if _, err := os.Stat(targetPath); err == nil {
			continue
		}
		// Build tools list: baseToolNames minus DeniedTools minus agent_done and run_agent.
		tools := make([]string, 0, len(sortedBase))
		seenTools := make(map[string]bool, len(sortedBase))
		for _, name := range sortedBase {
			if denied[name] || name == "agent_done" || name == "run_agent" {
				continue
			}
			if seenTools[name] {
				continue
			}
			seenTools[name] = true
			tools = append(tools, name)
		}
		// Convert directive newlines to spaces for single-line front matter.
		directive := strings.ReplaceAll(mode.Directive, "\n", " ")
		directive = strings.TrimSpace(directive)

		// Build agents list.
		agents := make([]string, len(mode.Agents))
		copy(agents, mode.Agents)

		// Build definition content.
		var sb strings.Builder
		sb.WriteString("---\n")
		sb.WriteString("name: " + mode.Name + "\n")
		sb.WriteString("description: Migrated from mode " + mode.Name + "\n")
		sb.WriteString("type: interactive\n")
		sb.WriteString("model: " + mode.Model + "\n")
		if len(tools) > 0 {
			sb.WriteString("tools:\n")
			for _, t := range tools {
				sb.WriteString("  - " + t + "\n")
			}
		}
		if len(agents) > 0 {
			sb.WriteString("agents:\n")
			for _, a := range agents {
				sb.WriteString("  - " + a + "\n")
			}
		}
		if directive != "" {
			sb.WriteString("directive: " + directive + "\n")
		}
		sb.WriteString("---\n")
		sb.WriteString("\n")

		if err := os.MkdirAll(agentsDir, 0755); err != nil {
			return fmt.Errorf("cannot create agents directory %s: %w", agentsDir, err)
		}
		if err := os.WriteFile(targetPath, []byte(sb.String()), 0600); err != nil {
			return fmt.Errorf("cannot write migrated definition %s: %w", targetPath, err)
		}
	}
	return nil
}
